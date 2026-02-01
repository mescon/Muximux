package health

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of an app
type Status string

const (
	StatusUnknown  Status = "unknown"
	StatusHealthy  Status = "healthy"
	StatusUnhealthy Status = "unhealthy"
)

// AppHealth holds health information for an app
type AppHealth struct {
	Name         string        `json:"name"`
	Status       Status        `json:"status"`
	ResponseTime time.Duration `json:"response_time_ms"`
	LastCheck    time.Time     `json:"last_check"`
	LastError    string        `json:"last_error,omitempty"`
	Uptime       float64       `json:"uptime_percent"`
	CheckCount   int           `json:"check_count"`
	SuccessCount int           `json:"success_count"`
}

// HealthChangeCallback is called when an app's health status changes
type HealthChangeCallback func(appName string, health *AppHealth)

// Monitor handles health checks for all apps
type Monitor struct {
	apps             map[string]AppConfig
	health           map[string]*AppHealth
	mu               sync.RWMutex
	interval         time.Duration
	timeout          time.Duration
	httpClient       *http.Client
	ctx              context.Context
	cancel           context.CancelFunc
	onHealthChange   HealthChangeCallback
}

// AppConfig holds the configuration for health checking an app
type AppConfig struct {
	Name     string
	URL      string
	HealthURL string // Optional custom health check URL
	Enabled  bool
}

// NewMonitor creates a new health monitor
func NewMonitor(interval, timeout time.Duration) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	return &Monitor{
		apps:     make(map[string]AppConfig),
		health:   make(map[string]*AppHealth),
		interval: interval,
		timeout:  timeout,
		httpClient: &http.Client{
			Timeout: timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Follow redirects but limit to 3
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// SetHealthChangeCallback sets a callback that's invoked when health status changes
func (m *Monitor) SetHealthChangeCallback(cb HealthChangeCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onHealthChange = cb
}

// SetApps updates the list of apps to monitor
func (m *Monitor) SetApps(apps []AppConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear old apps that are no longer in the list
	newApps := make(map[string]AppConfig)
	for _, app := range apps {
		newApps[app.Name] = app
		// Initialize health status if new
		if _, exists := m.health[app.Name]; !exists {
			m.health[app.Name] = &AppHealth{
				Name:   app.Name,
				Status: StatusUnknown,
			}
		}
	}

	// Remove health entries for apps that no longer exist
	for name := range m.health {
		if _, exists := newApps[name]; !exists {
			delete(m.health, name)
		}
	}

	m.apps = newApps
}

// Start begins the health check loop
func (m *Monitor) Start() {
	go m.run()
}

// Stop stops the health check loop
func (m *Monitor) Stop() {
	m.cancel()
}

// run is the main health check loop
func (m *Monitor) run() {
	// Do an initial check immediately
	m.checkAll()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAll()
		}
	}
}

// checkAll checks the health of all apps
func (m *Monitor) checkAll() {
	m.mu.RLock()
	apps := make([]AppConfig, 0, len(m.apps))
	for _, app := range m.apps {
		if app.Enabled {
			apps = append(apps, app)
		}
	}
	m.mu.RUnlock()

	// Check apps concurrently
	var wg sync.WaitGroup
	for _, app := range apps {
		wg.Add(1)
		go func(app AppConfig) {
			defer wg.Done()
			m.checkApp(app)
		}(app)
	}
	wg.Wait()
}

// checkApp checks the health of a single app
func (m *Monitor) checkApp(app AppConfig) {
	url := app.HealthURL
	if url == "" {
		url = app.URL
	}

	start := time.Now()
	req, err := http.NewRequestWithContext(m.ctx, http.MethodGet, url, nil)
	if err != nil {
		m.updateHealth(app.Name, StatusUnhealthy, 0, err.Error())
		return
	}

	resp, err := m.httpClient.Do(req)
	responseTime := time.Since(start)

	if err != nil {
		m.updateHealth(app.Name, StatusUnhealthy, responseTime, err.Error())
		return
	}
	defer resp.Body.Close()

	// Consider 2xx and 3xx as healthy
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		m.updateHealth(app.Name, StatusHealthy, responseTime, "")
	} else {
		m.updateHealth(app.Name, StatusUnhealthy, responseTime, resp.Status)
	}
}

// updateHealth updates the health status for an app
func (m *Monitor) updateHealth(name string, status Status, responseTime time.Duration, errMsg string) {
	m.mu.Lock()

	h, exists := m.health[name]
	if !exists {
		h = &AppHealth{Name: name}
		m.health[name] = h
	}

	// Track if status changed
	previousStatus := h.Status

	h.Status = status
	h.ResponseTime = responseTime
	h.LastCheck = time.Now()
	h.LastError = errMsg
	h.CheckCount++

	if status == StatusHealthy {
		h.SuccessCount++
	}

	// Calculate uptime percentage
	if h.CheckCount > 0 {
		h.Uptime = float64(h.SuccessCount) / float64(h.CheckCount) * 100
	}

	// Get callback and copy of health for notification
	cb := m.onHealthChange
	var healthCopy *AppHealth
	if cb != nil && (previousStatus != status || previousStatus == StatusUnknown) {
		copy := *h
		healthCopy = &copy
	}

	m.mu.Unlock()

	log.Printf("Health check for %s: %s (%.0fms)", name, status, float64(responseTime.Milliseconds()))

	// Notify callback outside of lock (only on status change)
	if healthCopy != nil {
		cb(name, healthCopy)
	}
}

// GetHealth returns the health status of a specific app
func (m *Monitor) GetHealth(name string) *AppHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if h, exists := m.health[name]; exists {
		// Return a copy
		copy := *h
		return &copy
	}
	return nil
}

// GetAllHealth returns the health status of all apps
func (m *Monitor) GetAllHealth() map[string]*AppHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*AppHealth)
	for name, h := range m.health {
		copy := *h
		result[name] = &copy
	}
	return result
}

// CheckNow triggers an immediate health check for a specific app
func (m *Monitor) CheckNow(name string) *AppHealth {
	m.mu.RLock()
	app, exists := m.apps[name]
	m.mu.RUnlock()

	if !exists {
		return nil
	}

	m.checkApp(app)
	return m.GetHealth(name)
}
