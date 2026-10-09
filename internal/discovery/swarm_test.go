package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

func TestHasSwarmTasks(t *testing.T) {
	if hasSwarmTasks([]ContainerSummary{{Labels: map[string]string{"x": "y"}}}) {
		t.Fatal("no swarm label")
	}
	if !hasSwarmTasks([]ContainerSummary{{}, {Labels: map[string]string{LabelSwarmServiceID: "s"}}}) {
		t.Fatal("swarm label present")
	}
}

func TestMergeSwarmServices(t *testing.T) {
	task := ContainerSummary{ID: "t1", Names: []string{"/stack_whoami.1.abc"}, Labels: map[string]string{LabelSwarmServiceID: "svc1", LabelSwarmServiceName: "stack_whoami", "muximux.app.port": "8080"},
		Ports: []ContainerPort{{PrivatePort: 80, Type: "tcp"}}}
	other := ContainerSummary{ID: "t2", Names: []string{"/plain"}}
	cs := []ContainerSummary{task, other}
	mergeSwarmServices(cs, []ServiceSummary{{ID: "svc1", Name: "stack_whoami",
		Labels:          map[string]string{"muximux.app.port": "9999", "muximux.app.name": "Whoami"},
		ContainerLabels: map[string]string{"muximux.app.icon": "whoami"},
		Ports:           []ContainerPort{{PrivatePort: 80, PublicPort: 18081, Type: "tcp"}, {PrivatePort: 443, PublicPort: 18443, Type: "tcp"}}}})
	got := cs[0]
	if got.Labels["muximux.app.port"] != "8080" || got.Labels["muximux.app.name"] != "Whoami" || got.Labels["muximux.app.icon"] != "whoami" {
		t.Fatalf("container labels must win, service labels fill gaps: %+v", got.Labels)
	}
	if len(got.Ports) != 2 || got.Ports[0].PublicPort != 18081 || got.Ports[1].PrivatePort != 443 {
		t.Fatalf("ports = %+v", got.Ports)
	}
	if len(cs[1].Ports) != 0 || len(cs[1].Labels) != 0 {
		t.Fatalf("non-swarm container touched: %+v", cs[1])
	}
}

func TestCollapseDuplicateKeys(t *testing.T) { // ruling 8: any duplicate key
	in := []Suggestion{
		{Key: "swarm:a", ContainerName: "a.2.x"}, {Key: "swarm:a", ContainerName: "a.1.y"},
		{Key: "name:b", ContainerName: "b"},
		{Key: "compose:p:web", ContainerName: "p-web-3"}, {Key: "compose:p:web", ContainerName: "p-web-1"}, {Key: "compose:p:web", ContainerName: "p-web-2"},
		{Key: "swarm:c", ContainerName: "c.1.z"},
	}
	out := collapseDuplicateKeys(in)
	if len(out) != 4 {
		t.Fatalf("got %d suggestions: %+v", len(out), out)
	}
	byKey := map[string]*Suggestion{}
	for i := range out {
		byKey[out[i].Key] = &out[i]
	}
	if s := byKey["swarm:a"]; s.ContainerName != "a.1.y" || len(s.Notes) != 1 || !strings.Contains(s.Notes[0], "2 containers share this key") {
		t.Fatalf("swarm:a = %+v", s)
	}
	if s := byKey["compose:p:web"]; s.ContainerName != "p-web-1" || !strings.Contains(s.Notes[0], "3 containers") {
		t.Fatalf("compose = %+v", s)
	}
	if len(byKey["name:b"].Notes)+len(byKey["swarm:c"].Notes) != 0 {
		t.Fatal("unique keys must not get a note")
	}
}

func TestCollapseDuplicateKeys_PrefersEligible(t *testing.T) {
	in := []Suggestion{
		{Key: "swarm:a", ContainerName: "a.1.x", AutoImportSkip: &AutoImportSkip{Code: SkipNoPort}},
		{Key: "swarm:a", ContainerName: "a.2.y"},
		{Key: "swarm:a", ContainerName: "a.3.z"},
	}
	out := collapseDuplicateKeys(in)
	if len(out) != 1 || out[0].ContainerName != "a.2.y" || out[0].AutoImportSkip != nil {
		t.Fatalf("eligible replica must win, lowest name breaking ties: %+v", out)
	}
	if len(out[0].Notes) != 1 || !strings.Contains(out[0].Notes[0], "3 containers share this key") {
		t.Fatalf("notes = %v", out[0].Notes)
	}
}

func TestMergeSwarmServices_NilLabelsAndNoPorts(t *testing.T) {
	cs := []ContainerSummary{
		{ID: "nil", Names: []string{"/nil"}}, // nil Labels: never a task, even against an empty service ID
		{ID: "t", Names: []string{"/s.1.x"}, Labels: map[string]string{LabelSwarmServiceID: "s1"}, Ports: []ContainerPort{{PrivatePort: 80, Type: "tcp"}}},
	}
	mergeSwarmServices(cs, []ServiceSummary{
		{ID: "", Labels: map[string]string{"muximux.app.name": "Ghost"}},
		{ID: "s1", Labels: map[string]string{"muximux.app.name": "S"}}, // no ports
	})
	if cs[0].Labels != nil || len(cs[0].Ports) != 0 {
		t.Fatalf("nil-label container touched: %+v", cs[0])
	}
	if cs[1].Labels["muximux.app.name"] != "S" || len(cs[1].Ports) != 1 || cs[1].Ports[0].PublicPort != 0 {
		t.Fatalf("service without ports must keep task ports: %+v", cs[1])
	}
}

func swarmDaemon(t *testing.T, set *[]ContainerSummary, services string, servicesStatus int) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/containers/json", func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(*set) })
	mux.HandleFunc("/v1.41/services", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(servicesStatus)
		_, _ = w.Write([]byte(services))
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	t.Cleanup(cleanup)
	return socket
}

const whoamiService = `[{"ID":"svc1","Spec":{"Name":"stack_whoami","Labels":{"muximux.app.name":"Whoami"}},
  "Endpoint":{"Ports":[{"Protocol":"tcp","TargetPort":80,"PublishedPort":18081}]}}]`

func whoamiTask(name string) ContainerSummary {
	return ContainerSummary{ID: "id-" + name, Names: []string{"/" + name}, Image: "traefik/whoami",
		Labels: map[string]string{LabelSwarmServiceID: "svc1", LabelSwarmServiceName: "stack_whoami"}}
}

func TestScan_SwarmServicePortsAndLabels(t *testing.T) {
	set := []ContainerSummary{whoamiTask("stack_whoami.2.bbbbbbbbbbbbbbbbbbbbbbbbb"), whoamiTask("stack_whoami.1.aaaaaaaaaaaaaaaaaaaaaaaaa")}
	socket := swarmDaemon(t, &set, whoamiService, http.StatusOK)
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket, NetworkStrategy: config.StrategyHostPort, HostIP: "10.0.0.1"})
	res := svc.Scan(context.Background(), "")
	if res.Error != "" || len(res.Suggestions) != 1 {
		t.Fatalf("res = %+v", res)
	}
	s := &res.Suggestions[0]
	if s.Key != "swarm:stack_whoami" || s.Name != "Whoami" || s.URL != "http://10.0.0.1:18081" || !s.Labeled || s.AutoImportSkip != nil ||
		s.ContainerName != "stack_whoami.1.aaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("suggestion = %+v", s)
	}
}

func TestScan_SwarmWorkerDegrades(t *testing.T) { // Review Focus 5
	net := ContainerNetworks{Networks: map[string]ContainerNetwork{"stack_net": {IPAddress: "10.0.1.5"}}}
	set := []ContainerSummary{
		{ID: "a", Names: []string{"/a_web.1.aaaaaaaaaaaaaaaaaaaaaaaaa"}, Image: "acme/a", NetworkSettings: net,
			Labels: map[string]string{LabelSwarmServiceID: "sa", LabelSwarmServiceName: "a_web", "muximux.app.port": "80"},
			Ports:  []ContainerPort{{PrivatePort: 80, Type: "tcp"}}},
		{ID: "b", Names: []string{"/b_web.1.bbbbbbbbbbbbbbbbbbbbbbbbb"}, Image: "acme/b", NetworkSettings: net,
			Labels: map[string]string{LabelSwarmServiceID: "sb", LabelSwarmServiceName: "b_web", "muximux.app.name": "B"}},
	}
	socket := swarmDaemon(t, &set, `{"message":"This node is not a swarm manager."}`, http.StatusServiceUnavailable)
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket,
		NetworkStrategy: config.StrategyContainerDNS, NetworkFilter: "stack_net"})
	for round := 0; round < 2; round++ {
		res := svc.Scan(context.Background(), "")
		if len(res.Suggestions) != 2 {
			t.Fatalf("round %d: %+v", round, res)
		}
		byKey := map[string]*Suggestion{}
		for i := range res.Suggestions {
			byKey[res.Suggestions[i].Key] = &res.Suggestions[i]
			if !slices.Contains(res.Suggestions[i].Notes, noteSwarmWorker) {
				t.Fatalf("round %d: missing worker note on %+v", round, res.Suggestions[i])
			}
		}
		if a := byKey["swarm:a_web"]; a.URL != "http://a_web:80" || a.AutoImportSkip != nil {
			t.Fatalf("a = %+v", a)
		}
		if b := byKey["swarm:b_web"]; b.AutoImportSkip == nil || b.AutoImportSkip.Code != SkipNoPort {
			t.Fatalf("b = %+v", b)
		}
	}
	svc.mu.RLock()
	degraded := svc.swarmDegraded
	svc.mu.RUnlock()
	if degraded != noteSwarmWorker {
		t.Fatalf("swarmDegraded = %q (the WARN is guarded by this transition)", degraded)
	}
	svc.Reconfigure(&config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket, NetworkStrategy: config.StrategyContainerDNS, NetworkFilter: "stack_net"})
	svc.mu.RLock()
	degraded = svc.swarmDegraded
	svc.mu.RUnlock()
	if degraded != "" {
		t.Fatal("Reconfigure must reset swarmDegraded")
	}
}

func TestScan_SwarmServicesForbiddenNotes(t *testing.T) { // ruling 12
	set := []ContainerSummary{whoamiTask("stack_whoami.1.aaaaaaaaaaaaaaaaaaaaaaaaa")}
	set[0].Labels["muximux.app.port"] = "80"
	set[0].Ports = []ContainerPort{{PrivatePort: 80, PublicPort: 18081, Type: "tcp"}}
	socket := swarmDaemon(t, &set, `{"message":"forbidden"}`, http.StatusForbidden)
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket, NetworkStrategy: config.StrategyHostPort, HostIP: "10.0.0.1"})
	res := svc.Scan(context.Background(), "")
	if len(res.Suggestions) != 1 || !slices.Contains(res.Suggestions[0].Notes, noteSwarmForbidden) {
		t.Fatalf("res = %+v", res)
	}
}

func TestScan_NoSwarmTasksSkipsServices(t *testing.T) {
	set := []ContainerSummary{{ID: "p", Names: []string{"/plain"}, Image: "acme/p", Labels: map[string]string{"muximux.app.port": "80"},
		Ports: []ContainerPort{{PrivatePort: 80, PublicPort: 8080, Type: "tcp"}}}}
	socket := swarmDaemon(t, &set, `{"message":"must not be called"}`, http.StatusInternalServerError)
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket, NetworkStrategy: config.StrategyHostPort, HostIP: "10.0.0.1"})
	res := svc.Scan(context.Background(), "")
	// The port-source note is expected; no swarm note is.
	if len(res.Suggestions) != 1 || slices.Contains(res.Suggestions[0].Notes, noteSwarmWorker) ||
		slices.Contains(res.Suggestions[0].Notes, noteSwarmForbidden) {
		t.Fatalf("res = %+v", res)
	}
}

// TestEnrichSwarm_TransitionsAndOtherErrors covers recovery (degraded ->
// readable clears the state) and a non-classified error (no note, state kept).
func TestEnrichSwarm_TransitionsAndOtherErrors(t *testing.T) {
	set := []ContainerSummary{whoamiTask("stack_whoami.1.aaaaaaaaaaaaaaaaaaaaaaaaa")}
	status, body := http.StatusServiceUnavailable, `{"message":"This node is not a swarm manager."}`
	var mu sync.Mutex
	mux := http.NewServeMux()
	mux.HandleFunc("/_ping", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("/v1.41/services", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	socket, cleanup := fakeDockerOverUnix(t, mux)
	defer cleanup()
	svc := NewService(&config.DiscoveryDockerConfig{Enabled: true, Endpoint: "unix://" + socket})
	client := svc.currentClient()
	state := func() string {
		svc.mu.RLock()
		defer svc.mu.RUnlock()
		return svc.swarmDegraded
	}
	set2 := func(st int, b string) {
		mu.Lock()
		status, body = st, b
		mu.Unlock()
	}
	cs := append([]ContainerSummary(nil), set...)
	if note := svc.enrichSwarm(context.Background(), client, cs); note != noteSwarmWorker || state() != noteSwarmWorker {
		t.Fatalf("worker: note=%q state=%q", note, state())
	}
	set2(http.StatusInternalServerError, `{"message":"boom"}`)
	if note := svc.enrichSwarm(context.Background(), client, cs); note != "" || state() != noteSwarmWorker {
		t.Fatalf("other error: note=%q state=%q", note, state())
	}
	set2(http.StatusOK, whoamiService)
	if note := svc.enrichSwarm(context.Background(), client, cs); note != "" || state() != "" || cs[0].Labels["muximux.app.name"] != "Whoami" {
		t.Fatalf("recovered: note=%q state=%q labels=%v", note, state(), cs[0].Labels)
	}
}
