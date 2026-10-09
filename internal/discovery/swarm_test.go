package discovery

import (
	"strings"
	"testing"
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
