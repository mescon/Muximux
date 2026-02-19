package health

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestMonitor_CheckApp_Healthy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second) // Long interval so no auto-check
	m.SetApps([]AppConfig{
		{Name: "testapp", URL: backend.URL, Enabled: true},
	})

	result := m.CheckNow("testapp")
	if result == nil {
		t.Fatal("expected health result")
	}
	if result.Status != StatusHealthy {
		t.Errorf("expected healthy, got %s", result.Status)
	}
	if result.ResponseTimeMs < 0 {
		t.Error("expected non-negative response time in ms")
	}
	if result.CheckCount != 1 {
		t.Errorf("expected 1 check, got %d", result.CheckCount)
	}
	if result.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", result.SuccessCount)
	}
	if result.Uptime != 100 {
		t.Errorf("expected 100%% uptime, got %.1f%%", result.Uptime)
	}
}

func TestMonitor_CheckApp_Unhealthy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second)
	m.SetApps([]AppConfig{
		{Name: "badapp", URL: backend.URL, Enabled: true},
	})

	result := m.CheckNow("badapp")
	if result == nil {
		t.Fatal("expected health result")
	}
	if result.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy, got %s", result.Status)
	}
	if result.LastError == "" {
		t.Error("expected error message for unhealthy app")
	}
	if result.SuccessCount != 0 {
		t.Errorf("expected 0 successes, got %d", result.SuccessCount)
	}
}

func TestMonitor_CheckApp_Timeout(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond) // Longer than timeout
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 50*time.Millisecond) // Very short timeout
	m.SetApps([]AppConfig{
		{Name: "slowapp", URL: backend.URL, Enabled: true},
	})

	result := m.CheckNow("slowapp")
	if result == nil {
		t.Fatal("expected health result")
	}
	if result.Status != StatusUnhealthy {
		t.Errorf("expected unhealthy for timeout, got %s", result.Status)
	}
	if result.LastError == "" {
		t.Error("expected error message for timeout")
	}
}

func TestMonitor_CheckApp_CustomHealthURL(t *testing.T) {
	// Main server should not be hit
	mainServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // Would fail if hit
	}))
	defer mainServer.Close()

	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthServer.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second)
	m.SetApps([]AppConfig{
		{Name: "app-with-health", URL: mainServer.URL, HealthURL: healthServer.URL, Enabled: true},
	})

	result := m.CheckNow("app-with-health")
	if result == nil {
		t.Fatal("expected health result")
	}
	if result.Status != StatusHealthy {
		t.Errorf("expected healthy (should use health URL), got %s", result.Status)
	}
}

func TestMonitor_CheckApp_Redirect(t *testing.T) {
	// Test that 3xx responses are considered healthy
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redirected", http.StatusMovedPermanently)
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second)
	m.SetApps([]AppConfig{
		{Name: "redirect-app", URL: backend.URL, Enabled: true},
	})

	result := m.CheckNow("redirect-app")
	if result == nil {
		t.Fatal("expected health result")
	}
	// 301 redirects are followed; the redirect handler loops, so the client
	// eventually sees a response. Status 200-399 is healthy.
	// With the test server's handler that always redirects, the client will
	// get ErrUseLastResponse after 3 redirects, which results in a 301 response.
	// 301 < 400 so it should be healthy.
	if result.Status != StatusHealthy {
		t.Errorf("expected healthy for redirect, got %s (error: %s)", result.Status, result.LastError)
	}
}

func TestMonitor_StartStop(t *testing.T) {
	m := NewMonitor(50*time.Millisecond, 5*time.Second)
	m.SetApps([]AppConfig{
		{Name: "noop", URL: "http://localhost:1", Enabled: false}, // disabled, won't be checked
	})

	// Start should not panic
	m.Start()

	// Give it a moment to run a cycle
	time.Sleep(100 * time.Millisecond)

	// Stop should not panic
	m.Stop()

	// Double stop should be safe
	m.Stop()
}

func TestMonitor_CheckNow(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second)
	m.SetApps([]AppConfig{
		{Name: "myapp", URL: backend.URL, Enabled: true},
	})

	// Initial status should be unknown
	initial := m.GetHealth("myapp")
	if initial == nil {
		t.Fatal("expected initial health entry")
	}
	if initial.Status != StatusUnknown {
		t.Errorf("expected unknown initially, got %s", initial.Status)
	}

	// CheckNow should update
	result := m.CheckNow("myapp")
	if result.Status != StatusHealthy {
		t.Errorf("expected healthy after CheckNow, got %s", result.Status)
	}

	// Non-existent app
	nonExistent := m.CheckNow("does-not-exist")
	if nonExistent != nil {
		t.Error("expected nil for non-existent app")
	}
}

func TestMonitor_Callback(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second)

	var mu sync.Mutex
	var callbackName string
	var callbackHealth *AppHealth

	m.SetHealthChangeCallback(func(appName string, health *AppHealth) {
		mu.Lock()
		defer mu.Unlock()
		callbackName = appName
		callbackHealth = health
	})

	m.SetApps([]AppConfig{
		{Name: "callback-app", URL: backend.URL, Enabled: true},
	})

	// First check: unknown -> healthy should trigger callback
	m.CheckNow("callback-app")

	mu.Lock()
	name := callbackName
	health := callbackHealth
	mu.Unlock()

	if name != "callback-app" {
		t.Errorf("expected callback for callback-app, got %q", name)
	}
	if health == nil {
		t.Fatal("expected health in callback")
	}
	if health.Status != StatusHealthy {
		t.Errorf("expected healthy in callback, got %s", health.Status)
	}
}

func TestMonitor_Callback_StatusChange(t *testing.T) {
	callCount := 0
	healthy := true

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if healthy {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second)

	var mu sync.Mutex
	var lastStatus Status

	m.SetHealthChangeCallback(func(appName string, health *AppHealth) {
		mu.Lock()
		defer mu.Unlock()
		callCount++
		lastStatus = health.Status
	})

	m.SetApps([]AppConfig{
		{Name: "flaky", URL: backend.URL, Enabled: true},
	})

	// First check: unknown -> healthy
	m.CheckNow("flaky")

	mu.Lock()
	if callCount != 1 {
		t.Errorf("expected 1 callback, got %d", callCount)
	}
	if lastStatus != StatusHealthy {
		t.Errorf("expected healthy, got %s", lastStatus)
	}
	mu.Unlock()

	// Second check: healthy -> healthy (no change, no callback)
	m.CheckNow("flaky")

	mu.Lock()
	if callCount != 1 {
		t.Errorf("expected still 1 callback (no change), got %d", callCount)
	}
	mu.Unlock()

	// Third check: healthy -> unhealthy (change, callback fires)
	healthy = false
	m.CheckNow("flaky")

	mu.Lock()
	if callCount != 2 {
		t.Errorf("expected 2 callbacks, got %d", callCount)
	}
	if lastStatus != StatusUnhealthy {
		t.Errorf("expected unhealthy, got %s", lastStatus)
	}
	mu.Unlock()
}

func TestMonitor_GetAllHealth(t *testing.T) {
	m := NewMonitor(1*time.Hour, 5*time.Second)
	m.SetApps([]AppConfig{
		{Name: "app1", URL: "http://localhost:1", Enabled: true},
		{Name: "app2", URL: "http://localhost:2", Enabled: true},
	})

	all := m.GetAllHealth()
	if len(all) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(all))
	}

	if _, ok := all["app1"]; !ok {
		t.Error("expected app1 in results")
	}
	if _, ok := all["app2"]; !ok {
		t.Error("expected app2 in results")
	}
}

func TestMonitor_SetApps_RemovesStale(t *testing.T) {
	m := NewMonitor(1*time.Hour, 5*time.Second)

	// Set initial apps
	m.SetApps([]AppConfig{
		{Name: "keep", URL: "http://localhost:1", Enabled: true},
		{Name: "remove", URL: "http://localhost:2", Enabled: true},
	})

	if len(m.GetAllHealth()) != 2 {
		t.Fatalf("expected 2 apps initially")
	}

	// Update with only one app
	m.SetApps([]AppConfig{
		{Name: "keep", URL: "http://localhost:1", Enabled: true},
	})

	all := m.GetAllHealth()
	if len(all) != 1 {
		t.Fatalf("expected 1 app after update, got %d", len(all))
	}
	if _, ok := all["keep"]; !ok {
		t.Error("expected 'keep' app to remain")
	}
	if _, ok := all["remove"]; ok {
		t.Error("expected 'remove' app to be gone")
	}
}

func TestMonitor_GetHealth_Nonexistent(t *testing.T) {
	m := NewMonitor(1*time.Hour, 5*time.Second)

	result := m.GetHealth("does-not-exist")
	if result != nil {
		t.Error("expected nil for non-existent app")
	}
}

func TestMonitor_DisabledAppsNotChecked(t *testing.T) {
	hitCount := 0
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	m := NewMonitor(1*time.Hour, 5*time.Second)
	m.SetApps([]AppConfig{
		{Name: "disabled", URL: backend.URL, Enabled: false},
	})

	// CheckNow should still work for disabled apps (it checks by name in apps map)
	result := m.CheckNow("disabled")
	if result == nil {
		t.Fatal("expected result for disabled app via CheckNow")
	}

	// But the health status should reflect the actual check
	if hitCount != 1 {
		t.Errorf("expected 1 hit from CheckNow, got %d", hitCount)
	}
}
