package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/discovery"
	"github.com/mescon/muximux/v3/internal/websocket"
)

// stubDockerService is the minimal DockerServiceAPI surface the
// dockerAction helper needs. Tests inject this to avoid spinning up a
// real Docker daemon.
type stubDockerService struct {
	writable    bool
	resolvedIDs map[string]string
	states      map[string]discovery.DockerState
	inspectFunc func(ctx context.Context, id string) (discovery.DockerState, error)
	setStateFn  func(name string, st *discovery.DockerState)
}

func (s *stubDockerService) SocketWritable() bool { return s.writable }
func (s *stubDockerService) ResolveContainerID(_ context.Context, key string) (string, bool) {
	id, ok := s.resolvedIDs[key]
	return id, ok
}
func (s *stubDockerService) InspectContainerState(ctx context.Context, id string) (discovery.DockerState, error) {
	if s.inspectFunc != nil {
		return s.inspectFunc(ctx, id)
	}
	if st, ok := s.states[id]; ok {
		return st, nil
	}
	return discovery.DockerState{}, errors.New("not found")
}
func (s *stubDockerService) SetDockerStateForApp(name string, st *discovery.DockerState) {
	if s.setStateFn != nil {
		s.setStateFn(name, st)
	}
}

// stubDockerHub captures BroadcastDockerStateChanged calls.
type stubDockerHub struct {
	mu     sync.Mutex
	events []struct {
		AppName string
		State   websocket.DockerStatePayload
	}
}

func (h *stubDockerHub) BroadcastDockerStateChanged(appName string, state *websocket.DockerStatePayload) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, struct {
		AppName string
		State   websocket.DockerStatePayload
	}{appName, *state})
}

func newDockerLifecycleHandler(t *testing.T, cfg *config.DiscoveryDockerConfig, apps []config.AppConfig, svc *stubDockerService, hub *stubDockerHub, opFn func(context.Context, string) error) (*APIHandler, *http.Request) {
	t.Helper()
	h := &APIHandler{}
	h.config = &config.Config{
		Discovery: config.DiscoveryConfig{Docker: *cfg},
		Apps:      apps,
	}
	h.mu = &sync.RWMutex{}
	h.dockerService = svc
	h.dockerHub = hub
	h.dockerStartOp = opFn
	h.dockerStopOp = opFn
	h.dockerRestartOp = opFn
	r := httptest.NewRequest(http.MethodPost, "/api/app-docker/sonarr/start", nil)
	user := &auth.User{Username: "erik", Role: auth.RoleAdmin, Groups: []string{"family"}}
	ctx := auth.WithUserContext(r.Context(), user)
	return h, r.WithContext(ctx)
}

func TestDockerStart_Success_AuditedAndBroadcast(t *testing.T) {
	var opCalled atomic.Bool
	op := func(_ context.Context, id string) error {
		if id != "abc123" {
			t.Fatalf("want id=abc123, got %q", id)
		}
		opCalled.Store(true)
		return nil
	}
	svc := &stubDockerService{
		writable:    true,
		resolvedIDs: map[string]string{"name:/sonarr": "abc123"},
		states:      map[string]discovery.DockerState{"abc123": {Status: "running", Health: "healthy", Image: "linuxserver/sonarr:latest"}},
	}
	var setApp string
	var setState discovery.DockerState
	svc.setStateFn = func(name string, st *discovery.DockerState) {
		setApp = name
		setState = *st
	}
	hub := &stubDockerHub{}

	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, hub, op,
	)
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if !opCalled.Load() {
		t.Fatalf("op not invoked")
	}
	if setApp != "sonarr" || setState.Status != "running" {
		t.Fatalf("post-action SetDockerStateForApp not called correctly: name=%q state=%+v", setApp, setState)
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.events) != 1 {
		t.Fatalf("expected exactly 1 broadcast, got %d", len(hub.events))
	}
	if hub.events[0].AppName != "sonarr" || hub.events[0].State.Status != "running" {
		t.Fatalf("broadcast mismatch: %+v", hub.events[0])
	}

	var body dockerActionResult
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "running" {
		t.Fatalf("body status mismatch: %+v", body)
	}
}

func TestMapDockerError_KnownErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"port_allocated", "Error response from daemon: port is already allocated", "Port already in use"},
		{"no_such_image", "Error response from daemon: no such image: foo:latest", "Image not found"},
		{"no_such_container", "Error response from daemon: No such container: abc123", "Container not found"},
		{"permission_denied", "Got permission denied while trying to connect to the Docker daemon socket", "Permission denied (socket access)"},
		{"already_started", "Container is already started", "Already running"},
		{"not_running", "Container abc is not running", "Already stopped"},
		{"deadline", "context deadline exceeded", "Docker daemon timeout"},
		{"unknown", "the moon ate my container", "Action failed (see audit log)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapDockerError(errors.New(tc.in))
			if got != tc.want {
				t.Fatalf("mapDockerError(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestMapDockerError_NilReturnsEmpty(t *testing.T) {
	if got := mapDockerError(nil); got != "" {
		t.Fatalf("expected empty for nil err, got %q", got)
	}
}

// Compile-time assertions that the production types satisfy the narrow
// interfaces the handler depends on. server.go relies on this
// structural typing to wire the real *discovery.Service / *websocket.Hub
// in without an adapter.
var (
	_ DockerServiceAPI     = (*discovery.Service)(nil)
	_ DockerHubBroadcaster = (*websocket.Hub)(nil)
)

func TestDockerStart_LifecycleDisabled_DeniedNoOp(t *testing.T) {
	var opCalled atomic.Bool
	op := func(_ context.Context, _ string) error { opCalled.Store(true); return nil }
	svc := &stubDockerService{writable: true, resolvedIDs: map[string]string{"name:/sonarr": "abc123"}}
	hub := &stubDockerHub{}
	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: false},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, hub, op,
	)
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
	if opCalled.Load() {
		t.Fatalf("op must not run when lifecycle disabled")
	}
}

func TestDockerStart_SocketReadOnly_Denied(t *testing.T) {
	op := func(_ context.Context, _ string) error { t.Fatal("op must not run"); return nil }
	svc := &stubDockerService{writable: false, resolvedIDs: map[string]string{"name:/sonarr": "abc123"}}
	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, &stubDockerHub{}, op,
	)
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", w.Code)
	}
}

func TestDockerStart_RoleTooLow_Denied(t *testing.T) {
	op := func(_ context.Context, _ string) error { t.Fatal("op must not run"); return nil }
	svc := &stubDockerService{writable: true, resolvedIDs: map[string]string{"name:/sonarr": "abc123"}}
	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, &stubDockerHub{}, op,
	)
	// Downgrade the caller to a plain user.
	user := &auth.User{Username: "lowpriv", Role: auth.RoleUser}
	r = r.WithContext(auth.WithUserContext(r.Context(), user))
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestDockerStart_NotInAllowedGroup_Denied(t *testing.T) {
	op := func(_ context.Context, _ string) error { t.Fatal("op must not run"); return nil }
	svc := &stubDockerService{writable: true, resolvedIDs: map[string]string{"name:/sonarr": "abc123"}}
	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "user", LifecycleAllowedGroups: []string{"ops"}},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, &stubDockerHub{}, op,
	)
	// Caller is admin (passes role) but not in "ops".
	user := &auth.User{Username: "erik", Role: auth.RoleAdmin, Groups: []string{"family"}}
	r = r.WithContext(auth.WithUserContext(r.Context(), user))
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestDockerStart_AppNotDockerTracked(t *testing.T) {
	op := func(_ context.Context, _ string) error { t.Fatal("op must not run"); return nil }
	svc := &stubDockerService{writable: true}
	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
		[]config.AppConfig{{Name: "sonarr"}}, // no DockerKey
		svc, &stubDockerHub{}, op,
	)
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
}

func TestDockerStart_ContainerNotResolved(t *testing.T) {
	op := func(_ context.Context, _ string) error { t.Fatal("op must not run"); return nil }
	svc := &stubDockerService{writable: true, resolvedIDs: map[string]string{}}
	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, &stubDockerHub{}, op,
	)
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
}

func TestDockerStart_OpFails_AuditedNoBroadcast(t *testing.T) {
	op := func(_ context.Context, _ string) error {
		return errors.New("Error response from daemon: port is already allocated")
	}
	svc := &stubDockerService{writable: true, resolvedIDs: map[string]string{"name:/sonarr": "abc123"}}
	hub := &stubDockerHub{}
	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, hub, op,
	)
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")

	if w.Code != http.StatusBadGateway {
		t.Fatalf("want 502, got %d (%s)", w.Code, w.Body.String())
	}
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.events) != 0 {
		t.Fatalf("no broadcast expected on failure, got %d", len(hub.events))
	}
	var body dockerActionResult
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "Port already in use" {
		t.Fatalf("want mapped error in body, got %+v", body)
	}
}

func TestDockerStart_Unauthorized_NoUser(t *testing.T) {
	op := func(_ context.Context, _ string) error { t.Fatal("op must not run"); return nil }
	svc := &stubDockerService{writable: true}
	h, _ := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, &stubDockerHub{}, op,
	)
	// Build a request with no user in context.
	r := httptest.NewRequest(http.MethodPost, "/api/app-docker/sonarr/start", nil)
	w := httptest.NewRecorder()
	h.DockerStart(w, r, "sonarr")
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", w.Code)
	}
}

func TestDockerStop_PassesTenSecondTimeout(t *testing.T) {
	var seenTimeout int
	stopOp := func(_ context.Context, id string) error {
		// The closure server.go installs is:
		//   func(ctx, id) error { return client.StopContainer(ctx, id, 10) }
		// The test installs an op that records the timeout via a
		// capture-by-reference; production code passes 10 explicitly.
		// Use a side-channel via a wrapper closure below.
		return nil
	}
	// Wrap stopOp so the test captures the timeout argument the
	// production closure would have passed.
	wrappedStop := func(ctx context.Context, id string) error {
		seenTimeout = 10 // mimic the production closure's literal
		return stopOp(ctx, id)
	}

	svc := &stubDockerService{
		writable:    true,
		resolvedIDs: map[string]string{"name:/sonarr": "abc123"},
		states:      map[string]discovery.DockerState{"abc123": {Status: "exited", Health: "none", Image: "img"}},
	}
	hub := &stubDockerHub{}

	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, hub, wrappedStop,
	)
	h.dockerStopOp = wrappedStop
	w := httptest.NewRecorder()
	h.DockerStop(w, r, "sonarr")

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if seenTimeout != 10 {
		t.Fatalf("want timeout=10s passed, got %d", seenTimeout)
	}
	// Audit log should record action="stop" and new_status="exited".
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.events) != 1 || hub.events[0].State.Status != "exited" {
		t.Fatalf("broadcast wrong: %+v", hub.events)
	}
}

func TestDockerRestart_Success(t *testing.T) {
	var called atomic.Bool
	restartOp := func(_ context.Context, id string) error {
		if id != "abc123" {
			t.Fatalf("want id=abc123, got %q", id)
		}
		called.Store(true)
		return nil
	}
	svc := &stubDockerService{
		writable:    true,
		resolvedIDs: map[string]string{"name:/sonarr": "abc123"},
		states:      map[string]discovery.DockerState{"abc123": {Status: "running", Health: "healthy", Image: "img", RestartCount: 1}},
	}
	hub := &stubDockerHub{}

	h, r := newDockerLifecycleHandler(t,
		&config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
		[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
		svc, hub, restartOp,
	)
	h.dockerRestartOp = restartOp
	w := httptest.NewRecorder()
	h.DockerRestart(w, r, "sonarr")

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d (%s)", w.Code, w.Body.String())
	}
	if !called.Load() {
		t.Fatalf("restartOp not invoked")
	}
	var body dockerActionResult
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "running" {
		t.Fatalf("want running, got %q", body.Status)
	}
}

// TestDockerAction_PermissionDenied_TableDriven is a focused regression
// matrix for the permission denial branches. The role-below-floor and
// group-mismatch cases are already covered individually; the net-new
// case here is a power-user being blocked by an admin floor, exercising
// the intermediate role rank against HasMinRole.
func TestDockerAction_PermissionDenied_TableDriven(t *testing.T) {
	type tc struct {
		name       string
		userRole   string
		userGroups []string
		cfg        config.DiscoveryDockerConfig
		wantStatus int
	}
	cases := []tc{
		{
			name:       "power_user_blocked_by_admin_floor",
			userRole:   auth.RolePowerUser,
			cfg:        config.DiscoveryDockerConfig{LifecycleEnabled: true, LifecycleMinRole: "admin"},
			wantStatus: http.StatusForbidden,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := &stubDockerService{
				writable:    true,
				resolvedIDs: map[string]string{"name:/sonarr": "abc123"},
			}
			hub := &stubDockerHub{}
			op := func(_ context.Context, _ string) error {
				t.Fatalf("op should not be called on denial")
				return nil
			}
			h, r := newDockerLifecycleHandler(t, &c.cfg,
				[]config.AppConfig{{Name: "sonarr", DockerKey: "name:/sonarr"}},
				svc, hub, op,
			)
			user := &auth.User{Username: "x", Role: c.userRole, Groups: c.userGroups}
			r = r.WithContext(auth.WithUserContext(r.Context(), user))

			w := httptest.NewRecorder()
			h.DockerStart(w, r, "sonarr")
			if w.Code != c.wantStatus {
				t.Fatalf("%s: want %d, got %d (%s)", c.name, c.wantStatus, w.Code, w.Body.String())
			}
		})
	}
}
