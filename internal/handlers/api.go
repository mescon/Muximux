package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/mescon/muximux/v3/internal/auth"
	"github.com/mescon/muximux/v3/internal/config"
	"github.com/mescon/muximux/v3/internal/logging"
)

// APIHandler handles API requests
type APIHandler struct {
	config       *config.Config
	configPath   string
	mu           *sync.RWMutex
	onConfigSave func() // called after config is saved to trigger route rebuilds etc.
	// actionClient relays http_action fires from the dashboard to the
	// configured target URL. Reused across requests so the connection
	// pool amortises across fires. 10s timeout is the lock-in from the
	// spec (Q8): long enough for slow webhooks, short enough that a
	// hung target can't pile up dashboard requests.
	actionClient *http.Client
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(cfg *config.Config, configPath string, mu *sync.RWMutex) *APIHandler {
	return &APIHandler{
		config:       cfg,
		configPath:   configPath,
		mu:           mu,
		actionClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SetOnConfigSave sets a callback invoked after every config save.
func (h *APIHandler) SetOnConfigSave(fn func()) {
	h.onConfigSave = fn
}

func (h *APIHandler) notifyConfigSaved() {
	if h.onConfigSave != nil {
		h.onConfigSave()
	}
}

// Pre-marshaled response for the high-frequency health endpoint
var healthOKResponse = []byte("{\"status\":\"ok\"}\n")

// Health returns server health status
func (h *APIHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(headerContentType, contentTypeJSON)
	w.Write(healthOKResponse)
}

// GetConfig returns the current configuration (sanitized)
func (h *APIHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userRole, userGroups := getUserRoleAndGroups(r)
	sendJSON(w, http.StatusOK, buildClientConfigResponse(h.config, userRole, userGroups))
}

// ExportConfig returns the full configuration as a downloadable YAML file,
// with sensitive auth fields (password hashes, secrets, API keys) stripped.
func (h *APIHandler) ExportConfig(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	cfg := *h.config
	h.mu.RUnlock()

	// Deep-copy slices that will be mutated to avoid writing through the
	// shared backing array into the live config.
	users := make([]config.UserConfig, len(cfg.Auth.Users))
	copy(users, cfg.Auth.Users)
	cfg.Auth.Users = users

	// Strip sensitive auth data
	for i := range cfg.Auth.Users {
		cfg.Auth.Users[i].PasswordHash = ""
	}
	cfg.Auth.APIKeyHash = ""
	cfg.Auth.OIDC.ClientSecret = ""

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "Failed to marshal config", "source", "config", "error", err)
		return
	}

	logging.From(r.Context()).Info("Config exported", "source", "audit")
	filename := fmt.Sprintf("muximux-config-%s.yaml", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Write(data)
}

// ParseImportedConfig accepts a YAML config file via POST, validates it, and
// returns the parsed config as JSON so the frontend can preview before applying.
func (h *APIHandler) ParseImportedConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MB limit
	if err != nil {
		respondError(w, r, http.StatusBadRequest, "Failed to read request body")
		return
	}

	// KnownFields(true) rejects any YAML field not declared on the
	// Config struct. Guards against mass-assignment: if a sensitive
	// field is ever added to Config, an older backup that happens to
	// contain a field of the same name cannot seed it without an
	// explicit code change acknowledging the import path
	// (findings.md M7).
	var cfg config.Config
	dec := yaml.NewDecoder(bytes.NewReader(body))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		respondError(w, r, http.StatusBadRequest, fmt.Sprintf("Invalid YAML: %s", err.Error()), "source", "config", "error", err)
		return
	}

	if len(cfg.Apps) == 0 {
		respondError(w, r, http.StatusBadRequest, "Config must contain at least one app")
		return
	}

	if err := validateImportedConfig(&cfg); err != nil {
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "config")
		return
	}

	// Return as the same sanitized JSON format the frontend expects
	sendJSON(w, http.StatusOK, buildClientConfigResponse(&cfg, "", nil))
}

// validateImportedConfig rejects backups that would leave the running
// instance in an opaque broken state (findings.md M20). Checks:
//   - every app has a Name and URL, and each URL is parseable http(s)
//   - known open_mode values
//   - min_role is either empty or a known role
//   - auth.method is one of the known values
//   - any duration fields parse
//
// Missing fields that Load() would silently default are left alone; the
// point here is to reject structurally invalid inputs, not to force
// every field to be explicit.
func validateImportedConfig(cfg *config.Config) error {
	knownOpenModes := map[string]bool{"": true, "iframe": true, "tab": true, "window": true, "popup": true}
	knownRoles := map[string]bool{"": true, auth.RoleAdmin: true, auth.RolePowerUser: true, auth.RoleUser: true}
	knownAuthMethods := map[string]bool{"": true, "none": true, "builtin": true, "forward_auth": true, "oidc": true}

	for i := range cfg.Apps {
		app := &cfg.Apps[i]
		if app.Name == "" {
			return fmt.Errorf("each app must have a name")
		}
		if app.URL == "" {
			return fmt.Errorf("app %q must have a URL", app.Name)
		}
		if u, err := parseURL(app.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "") {
			return fmt.Errorf("app %q has invalid URL %q", app.Name, app.URL)
		}
		if !knownOpenModes[app.OpenMode] {
			return fmt.Errorf("app %q has unknown open_mode %q", app.Name, app.OpenMode)
		}
		if !knownRoles[app.MinRole] {
			return fmt.Errorf("app %q has unknown min_role %q", app.Name, app.MinRole)
		}
	}

	if !knownAuthMethods[cfg.Auth.Method] {
		return fmt.Errorf("unknown auth.method %q", cfg.Auth.Method)
	}

	if cfg.Auth.SessionMaxAge != "" {
		if _, err := time.ParseDuration(cfg.Auth.SessionMaxAge); err != nil {
			return fmt.Errorf("invalid auth.session_max_age: %w", err)
		}
	}
	if cfg.Server.ProxyTimeout != "" {
		if _, err := time.ParseDuration(cfg.Server.ProxyTimeout); err != nil {
			return fmt.Errorf("invalid server.proxy_timeout: %w", err)
		}
	}

	// Gateway sites use the same rules the YAML loader applies. Without
	// this check, a backup with a malformed gateway block (bad domain,
	// http://path, header smuggling) would pass UI preview and break
	// the next boot when Load() rejects it.
	if err := config.ValidateGatewaySites(cfg.Server.GatewaySites, cfg); err != nil {
		return err
	}

	return nil
}

func parseURL(raw string) (*url.URL, error) {
	return url.Parse(raw)
}

// clientConfigResponse is the sanitized config structure sent to the frontend.
type clientConfigResponse struct {
	Title        string `json:"title"`
	Language     string `json:"language"`
	LogLevel     string `json:"log_level"`
	ProxyTimeout string `json:"proxy_timeout,omitempty"`
	// SessionCookieDomain mirrors server.session_cookie_domain so the
	// Settings UI can pre-warn operators ticking require_auth on a
	// gateway site that the cookie won't reach the gated subdomain.
	// Empty when unset. Not sensitive.
	SessionCookieDomain string                    `json:"session_cookie_domain,omitempty"`
	Navigation          config.NavigationConfig   `json:"navigation"`
	Theme               config.ThemeConfig        `json:"theme"`
	Health              *config.HealthConfig      `json:"health,omitempty"`
	Keybindings         *config.KeybindingsConfig `json:"keybindings,omitempty"`
	Auth                *clientAuthConfig         `json:"auth,omitempty"`
	Groups              []config.GroupConfig      `json:"groups"`
	Apps                []ClientAppConfig         `json:"apps"`
}

// clientAuthConfig is the sanitized auth config sent to the frontend.
// Excludes sensitive fields (users, api_key, etc).
type clientAuthConfig struct {
	Method         string            `json:"method"`
	TrustedProxies []string          `json:"trusted_proxies,omitempty"`
	Headers        map[string]string `json:"headers,omitempty"`
	LogoutURL      string            `json:"logout_url,omitempty"`
}

// buildClientConfigResponse creates a sanitized config response from the server config.
// userRole filters apps by minimum role; empty string means no filtering (e.g. import preview).
// userGroups filters apps by allowed_groups; nil means no group filtering.
func buildClientConfigResponse(cfg *config.Config, userRole string, userGroups []string) clientConfigResponse {
	language := cfg.Server.Language
	if language == "" {
		language = "en"
	}
	resp := clientConfigResponse{
		Title:               cfg.Server.Title,
		Language:            language,
		LogLevel:            cfg.Server.LogLevel,
		ProxyTimeout:        cfg.Server.ProxyTimeout,
		SessionCookieDomain: cfg.Server.SessionCookieDomain,
		Navigation:          cfg.Navigation,
		Theme:               cfg.Theme,
		Health:              &cfg.Health,
		Groups:              cfg.Groups,
		Apps:                sanitizeApps(cfg.Apps, userRole, userGroups, cfg.Server.GatewaySites),
	}
	if len(cfg.Keybindings.Bindings) > 0 {
		resp.Keybindings = &cfg.Keybindings
	}
	if cfg.Auth.Method != "" {
		authCfg := &clientAuthConfig{Method: cfg.Auth.Method}
		if len(cfg.Auth.TrustedProxies) > 0 {
			authCfg.TrustedProxies = cfg.Auth.TrustedProxies
		}
		if len(cfg.Auth.Headers) > 0 {
			authCfg.Headers = cfg.Auth.Headers
		}
		authCfg.LogoutURL = cfg.Auth.LogoutURL
		resp.Auth = authCfg
	}
	return resp
}

// ClientConfigUpdate represents the configuration update from the frontend
type ClientConfigUpdate struct {
	Title        string `json:"title"`
	Language     string `json:"language"`
	LogLevel     string `json:"log_level"`
	ProxyTimeout string `json:"proxy_timeout"`
	// SessionCookieDomain controls the Domain attribute on the session
	// cookie. Required when any gateway site has require_auth=true.
	// Editable from the UI so operators don't have to drop into
	// config.yaml + restart to enable the gateway auth gate.
	SessionCookieDomain string                    `json:"session_cookie_domain"`
	Navigation          config.NavigationConfig   `json:"navigation"`
	Theme               config.ThemeConfig        `json:"theme"`
	Health              *config.HealthConfig      `json:"health,omitempty"`
	Keybindings         *config.KeybindingsConfig `json:"keybindings,omitempty"`
	Groups              []config.GroupConfig      `json:"groups"`
	Apps                []ClientAppConfig         `json:"apps"`
}

// SaveConfig updates and saves the configuration
func (h *APIHandler) SaveConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		respondError(w, r, http.StatusMethodNotAllowed, errMethodNotAllowed)
		return
	}

	var update ClientConfigUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Snapshot every field mergeConfigUpdate mutates so we can restore
	// the in-memory config if the disk Save fails. Without this the
	// live process would see the new shape (next GET returns it) while
	// disk still holds the old one - the same divergence the per-app
	// and per-group CRUD endpoints already guard against via
	// saveOrRollback{Apps,Groups} (codebase review C1-shf).
	//
	// Invariant: rollback works because mergeConfigUpdate *replaces*
	// each field wholesale rather than mutating it in place
	// (cfg.Navigation = update.Navigation, cfg.Apps = newApps,
	// cfg.Keybindings = *update.Keybindings, etc.). Snapshot copies
	// here are shallow struct copies; for fields that contain maps
	// (Keybindings.Bindings, Theme.Colors when present) the snapshot
	// shares the underlying map header, but the merge code never
	// touches those map entries directly so the prior reference still
	// points at the old map contents. If anyone changes
	// mergeConfigUpdate to mutate map entries in place, this rollback
	// silently breaks - clone the relevant map here and update both
	// places together.
	priorTitle := h.config.Server.Title
	priorLanguage := h.config.Server.Language
	priorLogLevel := h.config.Server.LogLevel
	priorProxyTimeout := h.config.Server.ProxyTimeout
	priorSessionCookieDomain := h.config.Server.SessionCookieDomain
	priorNavigation := h.config.Navigation
	priorTheme := h.config.Theme
	priorHealth := h.config.Health
	priorKeybindings := h.config.Keybindings
	priorGroups := h.config.Groups
	priorApps := h.config.Apps

	mergeConfigUpdate(h.config, &update)

	rollback := func() {
		h.config.Server.Title = priorTitle
		h.config.Server.Language = priorLanguage
		h.config.Server.LogLevel = priorLogLevel
		h.config.Server.ProxyTimeout = priorProxyTimeout
		h.config.Server.SessionCookieDomain = priorSessionCookieDomain
		h.config.Navigation = priorNavigation
		h.config.Theme = priorTheme
		h.config.Health = priorHealth
		h.config.Keybindings = priorKeybindings
		h.config.Groups = priorGroups
		h.config.Apps = priorApps
	}

	// Re-run the same invariant checks Load uses at startup, so a bad
	// runtime mutation (e.g. a session_cookie_domain that doesn't cover
	// a configured gateway site) is rejected with a 400 before it can
	// be persisted and break the next boot.
	if err := h.config.Validate(); err != nil {
		rollback()
		logging.From(r.Context()).Warn("SaveConfig rejected by validation",
			"source", "audit",
			"error", err)
		respondError(w, r, http.StatusBadRequest, err.Error(), "source", "config", "error", err)
		return
	}

	// Save to file
	if err := h.config.Save(h.configPath); err != nil {
		rollback()
		logging.Error("SaveConfig failed; in-memory state rolled back",
			"source", "audit",
			"error", err)
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "error", err)
		return
	}

	logging.From(r.Context()).Info("Configuration saved", "source", "audit")
	h.notifyConfigSaved()

	// Apply log level change at runtime
	if h.config.Server.LogLevel != "" {
		logging.SetLevel(logging.Level(h.config.Server.LogLevel))
		logging.From(r.Context()).Info("Log level changed", "source", "config", "level", h.config.Server.LogLevel)
	}

	// Admin role automatically passes every group gate, so no need to
	// thread the admin's actual group list here.
	sendJSON(w, http.StatusOK, buildClientConfigResponse(h.config, auth.RoleAdmin, nil))
}

// mergeConfigUpdate applies a client config update to the server config,
// preserving sensitive fields (auth bypass, access rules, original proxy URLs).
func mergeConfigUpdate(cfg *config.Config, update *ClientConfigUpdate) {
	cfg.Server.Title = update.Title
	cfg.Server.Language = update.Language
	cfg.Server.LogLevel = update.LogLevel
	if update.ProxyTimeout != "" {
		cfg.Server.ProxyTimeout = update.ProxyTimeout
	}
	// SessionCookieDomain is a server-level setting that gates the
	// gateway auth feature; persist explicit edits so the operator
	// can flip it from Settings without restarting first. The cookie
	// manager re-reads it on startup, so a restart is still required
	// before the new scope takes effect on issued cookies.
	cfg.Server.SessionCookieDomain = update.SessionCookieDomain
	cfg.Navigation = update.Navigation
	cfg.Theme = update.Theme
	if update.Health != nil {
		cfg.Health = *update.Health
	}
	cfg.Groups = update.Groups
	if update.Keybindings != nil {
		cfg.Keybindings = *update.Keybindings
	}

	// Build lookup of existing apps by name to preserve sensitive data.
	existingApps := make(map[string]config.AppConfig)
	for i := range cfg.Apps {
		existingApps[cfg.Apps[i].Name] = cfg.Apps[i]
	}

	newApps := make([]config.AppConfig, 0, len(update.Apps))
	for i := range update.Apps {
		app, detachKey := mergeClientApp(&update.Apps[i], existingApps)
		if detachKey != "" {
			// Manual URL edit on a tracked app auto-detaches: the
			// operator took manual control of the URL, so further
			// poller refreshes would clobber that change.
			// Logged at audit so operators can see exactly which apps
			// fell out of auto-management as a result of a SaveConfig
			// round-trip. The frontend should normally surface the
			// edit-lock prompt before this fires.
			logging.Audit("Docker tracking auto-detached on URL change",
				"kind", "app", "name", app.Name,
				"previous_key", detachKey)
		}
		newApps = append(newApps, app)
	}
	cfg.Apps = newApps
}

// clientAppToConfig converts a client app payload to a full AppConfig.
func clientAppToConfig(c *ClientAppConfig) config.AppConfig {
	return config.AppConfig{
		Name:                c.Name,
		URL:                 c.URL,
		HealthURL:           c.HealthURL,
		Icon:                c.Icon,
		Color:               c.Color,
		Group:               c.Group,
		Order:               c.Order,
		Enabled:             c.Enabled,
		Default:             c.Default,
		OpenMode:            c.OpenMode,
		HTTPActionMethod:    c.HTTPActionMethod,
		HTTPActionHeaders:   c.HTTPActionHeaders,
		HTTPActionConfirm:   c.HTTPActionConfirm,
		HTTPActionShowToast: c.HTTPActionShowToast,
		Proxy:               c.Proxy,
		HealthCheck:         c.HealthCheck,
		ProxySkipTLSVerify:  c.ProxySkipTLSVerify,
		ProxyHeaders:        c.ProxyHeaders,
		Scale:               c.Scale,
		Shortcut:            c.Shortcut,
		MinRole:             c.MinRole,
		AllowedGroups:       c.AllowedGroups,
		ForceIconBackground: c.ForceIconBackground,
		Permissions:         c.Permissions,
		AllowNotifications:  c.AllowNotifications,
		DockerKey:           c.DockerKey,
		DockerEndpoint:      c.DockerEndpoint,
		DockerStrategy:      c.DockerStrategy,
		DockerManagedURL:    c.DockerManagedURL,
	}
}

// mergeClientApp converts a client app config back to a full app config,
// preserving sensitive fields from the existing app if it was previously configured.
// The second return is the detach event when the call auto-detaches a
// previously-tracked app due to an explicit URL change (see
// applyDockerTrackingPreservation); "" when no detach happened.
func mergeClientApp(clientApp *ClientAppConfig, existingApps map[string]config.AppConfig) (config.AppConfig, string) {
	app := clientAppToConfig(clientApp)
	var detachReason string

	if existing, ok := existingApps[clientApp.Name]; ok {
		app.AuthBypass = existing.AuthBypass
		app.Access = existing.Access
		detachReason = applyDockerTrackingPreservation(&app, &existing)
	}

	return app, detachReason
}

// applyDockerTrackingPreservation reconciles tracking fields between
// an incoming PUT payload and the previously-stored app, preventing
// silent detach via two safety nets:
//
//  1. **Empty-payload preservation**: when the incoming payload
//     omits DockerKey entirely (clientApp.DockerKey == ""), copy the
//     existing tracking fields back. This stops a buggy frontend or
//     scripted PUT that doesn't echo the read-only tracking fields
//     from accidentally wiping them. The only sanctioned forget path
//     is DELETE /api/discovery/docker/track/{key}.
//
//  2. **URL-change auto-detach**: when the existing app was tracked
//     AND the incoming URL differs from existing.URL, clear all three
//     tracking fields. The operator explicitly took manual control of
//     the URL, so further refresh-poller writes would clobber that
//     change every tick. The plan v4 "Manual URL edit on a docker-
//     tracked app via SaveConfig is rejected or auto-detaches (pick:
//     auto-detach, document)" line motivates this branch. Returns a
//     non-empty reason string so the caller can emit one audit log
//     entry per detached app.
//
// Other tracking fields (Strategy / Endpoint) being changed without
// a URL change still go through, since the frontend's Re-link flow
// is allowed to swap those without detaching.
func applyDockerTrackingPreservation(updated *config.AppConfig, existing *config.AppConfig) string {
	if updated.DockerKey == "" {
		updated.DockerKey = existing.DockerKey
		updated.DockerEndpoint = existing.DockerEndpoint
		updated.DockerStrategy = existing.DockerStrategy
		updated.DockerManagedURL = existing.DockerManagedURL
		return ""
	}
	if existing.DockerKey != "" && existing.URL != "" && updated.URL != existing.URL {
		reason := existing.DockerKey
		updated.DockerKey = ""
		updated.DockerEndpoint = ""
		updated.DockerStrategy = ""
		updated.DockerManagedURL = ""
		return reason
	}
	// Tracking stays; keep DockerManagedURL in sync with the
	// stored URL so a re-link or no-op save doesn't accidentally
	// stale the baseline Load() compares against.
	updated.DockerManagedURL = updated.URL
	return ""
}

// GetApps returns the list of apps
func (h *APIHandler) GetApps(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userRole, userGroups := getUserRoleAndGroups(r)
	sendJSON(w, http.StatusOK, sanitizeApps(h.config.Apps, userRole, userGroups, h.config.Server.GatewaySites))
}

// GetGroups returns the list of groups
func (h *APIHandler) GetGroups(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sendJSON(w, http.StatusOK, h.config.Groups)
}

// GetApp returns a single app by name
func (h *APIHandler) GetApp(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			sendJSON(w, http.StatusOK, sanitizeApp(&h.config.Apps[i]))
			return
		}
	}

	respondError(w, r, http.StatusNotFound, errAppNotFound)
}

// saveOrRollbackApps persists the live config and, on failure, restores
// the apps slice from the supplied snapshot so the in-memory and
// on-disk views never diverge. Caller holds h.mu.
//
// Mirrors the rollback pattern the gateway handler pioneered for its
// own mutations; bringing the existing apps/groups CRUD endpoints
// into the same shape closes a class of "save failed but in-memory
// state was already mutated" bugs that otherwise surface as silently
// persisted writes on the next successful save.
func (h *APIHandler) saveOrRollbackApps(prior []config.AppConfig, op, name string) error {
	if err := h.config.Save(h.configPath); err != nil {
		h.config.Apps = prior
		logging.Error("Apps "+op+" save failed; in-memory state rolled back",
			"source", "audit",
			"app", name,
			"error", err)
		return err
	}
	return nil
}

// saveOrRollbackGroups is the groups-slice counterpart of
// saveOrRollbackApps. Same contract, different field.
func (h *APIHandler) saveOrRollbackGroups(prior []config.GroupConfig, op, name string) error {
	if err := h.config.Save(h.configPath); err != nil {
		h.config.Groups = prior
		logging.Error("Groups "+op+" save failed; in-memory state rolled back",
			"source", "audit",
			"group", name,
			"error", err)
		return err
	}
	return nil
}

// CreateApp creates a new app
func (h *APIHandler) CreateApp(w http.ResponseWriter, r *http.Request) {
	var clientApp ClientAppConfig
	if err := json.NewDecoder(r.Body).Decode(&clientApp); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if clientApp.Name == "" {
		respondError(w, r, http.StatusBadRequest, "App name is required")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if app already exists
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == clientApp.Name {
			respondError(w, r, http.StatusConflict, "App already exists")
			return
		}
	}

	// Create new app config
	newApp := clientAppToConfig(&clientApp)
	newApp.Order = len(h.config.Apps) // Add at end

	priorApps := append([]config.AppConfig(nil), h.config.Apps...)
	h.config.Apps = append(h.config.Apps, newApp)

	// Save config (rollback on disk failure so the in-memory state
	// never diverges from what's on disk).
	if err := h.saveOrRollbackApps(priorApps, "create", newApp.Name); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "app", newApp.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("App created", "source", "audit", "app", newApp.Name)
	h.notifyConfigSaved()
	sendJSON(w, http.StatusCreated, sanitizeApp(&newApp))
}

// UpdateApp updates an existing app
func (h *APIHandler) UpdateApp(w http.ResponseWriter, r *http.Request, name string) {
	var clientApp ClientAppConfig
	if err := json.NewDecoder(r.Body).Decode(&clientApp); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the app
	idx := -1
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	// Update app config, preserving sensitive fields
	existing := h.config.Apps[idx]
	updated := clientAppToConfig(&clientApp)
	updated.AuthBypass = existing.AuthBypass
	updated.Access = existing.Access
	detachKey := applyDockerTrackingPreservation(&updated, &existing)
	if detachKey != "" {
		logging.Audit("Docker tracking auto-detached on URL change",
			"kind", "app", "name", updated.Name,
			"previous_key", detachKey)
	}

	priorApps := append([]config.AppConfig(nil), h.config.Apps...)
	h.config.Apps[idx] = updated

	// Save config (rollback on disk failure).
	if err := h.saveOrRollbackApps(priorApps, "update", clientApp.Name); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "app", clientApp.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("App updated", "source", "audit", "app", clientApp.Name)
	h.notifyConfigSaved()
	sendJSON(w, http.StatusOK, sanitizeApp(&h.config.Apps[idx]))
}

// DeleteApp removes an app
func (h *APIHandler) DeleteApp(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find and remove the app
	idx := -1
	for i := range h.config.Apps {
		if h.config.Apps[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errAppNotFound)
		return
	}

	priorApps := append([]config.AppConfig(nil), h.config.Apps...)
	priorSites := append([]config.GatewaySite(nil), h.config.Server.GatewaySites...)
	h.config.Apps = append(h.config.Apps[:idx], h.config.Apps[idx+1:]...)

	// Clear AppName on any gateway site that referenced this app, so
	// the gateway-sites validator's cross-reference check stays
	// satisfied. Without this cascade, an unrelated config save after
	// the app deletion would fail validation with a dangling app_name
	// error.
	clearedSites := 0
	for i := range h.config.Server.GatewaySites {
		if h.config.Server.GatewaySites[i].AppName == name {
			h.config.Server.GatewaySites[i].AppName = ""
			clearedSites++
		}
	}

	// Save config; on failure roll back both the apps slice and the
	// gateway-sites slice so the cascading clear is undone too.
	if err := h.config.Save(h.configPath); err != nil {
		h.config.Apps = priorApps
		h.config.Server.GatewaySites = priorSites
		logging.Error("Apps delete save failed; in-memory state rolled back",
			"source", "audit",
			"app", name,
			"error", err)
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "app", name, "error", err)
		return
	}

	if clearedSites > 0 {
		logging.From(r.Context()).Info("Cleared dangling gateway-site app_name references",
			"source", "audit",
			"app", name,
			"sites", clearedSites)
	}
	logging.From(r.Context()).Info("App deleted", "source", "audit", "app", name)
	h.notifyConfigSaved()
	w.WriteHeader(http.StatusNoContent)
}

// GetGroup returns a single group by name
func (h *APIHandler) GetGroup(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for i := range h.config.Groups {
		if h.config.Groups[i].Name == name {
			sendJSON(w, http.StatusOK, h.config.Groups[i])
			return
		}
	}

	respondError(w, r, http.StatusNotFound, errGroupNotFound)
}

// CreateGroup creates a new group
func (h *APIHandler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var group config.GroupConfig
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	if group.Name == "" {
		respondError(w, r, http.StatusBadRequest, "Group name is required")
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Check if group already exists
	for i := range h.config.Groups {
		if h.config.Groups[i].Name == group.Name {
			respondError(w, r, http.StatusConflict, "Group already exists")
			return
		}
	}

	group.Order = len(h.config.Groups)
	priorGroups := append([]config.GroupConfig(nil), h.config.Groups...)
	h.config.Groups = append(h.config.Groups, group)

	// Save config (rollback on disk failure).
	if err := h.saveOrRollbackGroups(priorGroups, "create", group.Name); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "group", group.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Group created", "source", "audit", "group", group.Name)
	sendJSON(w, http.StatusCreated, group)
}

// UpdateGroup updates an existing group
func (h *APIHandler) UpdateGroup(w http.ResponseWriter, r *http.Request, name string) {
	var group config.GroupConfig
	if err := json.NewDecoder(r.Body).Decode(&group); err != nil {
		respondError(w, r, http.StatusBadRequest, errInvalidJSON+err.Error())
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Find the group
	idx := -1
	for i := range h.config.Groups {
		if h.config.Groups[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errGroupNotFound)
		return
	}

	priorGroups := append([]config.GroupConfig(nil), h.config.Groups...)
	h.config.Groups[idx] = group

	// Save config (rollback on disk failure).
	if err := h.saveOrRollbackGroups(priorGroups, "update", group.Name); err != nil {
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "group", group.Name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Group updated", "source", "audit", "group", group.Name)
	sendJSON(w, http.StatusOK, group)
}

// DeleteGroup removes a group
func (h *APIHandler) DeleteGroup(w http.ResponseWriter, r *http.Request, name string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Find and remove the group
	idx := -1
	for i := range h.config.Groups {
		if h.config.Groups[i].Name == name {
			idx = i
			break
		}
	}

	if idx == -1 {
		respondError(w, r, http.StatusNotFound, errGroupNotFound)
		return
	}

	priorGroups := append([]config.GroupConfig(nil), h.config.Groups...)
	priorApps := append([]config.AppConfig(nil), h.config.Apps...)
	h.config.Groups = append(h.config.Groups[:idx], h.config.Groups[idx+1:]...)

	for i := range h.config.Apps {
		if h.config.Apps[i].Group == name {
			h.config.Apps[i].Group = ""
		}
	}

	// Save config; on failure roll back both the groups slice and the
	// apps slice so the cascading orphan-clear is undone too.
	if err := h.config.Save(h.configPath); err != nil {
		h.config.Groups = priorGroups
		h.config.Apps = priorApps
		logging.Error("Groups delete save failed; in-memory state rolled back",
			"source", "audit",
			"group", name,
			"error", err)
		respondError(w, r, http.StatusInternalServerError, errFailedSaveConfig, "source", "config", "group", name, "error", err)
		return
	}

	logging.From(r.Context()).Info("Group deleted", "source", "audit", "group", name)
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeApp converts a single app config to client format. It assumes
// an admin-level caller: use sanitizeAppForRole instead when returning
// config to non-admins so embedded URL credentials
// (https://user:token@host/) and per-app injected headers (Authorization
// / X-Api-Key) don't leak to everyone who can read /api/config.
func sanitizeApp(app *config.AppConfig) ClientAppConfig {
	return sanitizeAppForRole(app, true)
}

// sanitizeAppForRole builds a client-facing app config with sensitive
// fields stripped when isAdmin is false. Non-admins see app.URL with
// any userinfo removed and never receive ProxyHeaders.
func sanitizeAppForRole(app *config.AppConfig, isAdmin bool) ClientAppConfig {
	var proxyURL string
	if app.Proxy {
		proxyURL = proxyPathPrefix + Slugify(app.Name) + "/"
	}
	url := app.URL
	var proxyHeaders map[string]string
	if isAdmin {
		proxyHeaders = app.ProxyHeaders
	} else {
		url = stripURLCredentials(url)
	}
	out := ClientAppConfig{
		Name:                app.Name,
		URL:                 url,
		HealthURL:           stripURLCredentialsIf(!isAdmin, app.HealthURL),
		ProxyURL:            proxyURL,
		Icon:                app.Icon,
		Color:               app.Color,
		Group:               app.Group,
		Order:               app.Order,
		Enabled:             app.Enabled,
		Default:             app.Default,
		OpenMode:            app.OpenMode,
		HTTPActionMethod:    app.HTTPActionMethod,
		HTTPActionConfirm:   app.HTTPActionConfirm,
		HTTPActionShowToast: app.HTTPActionShowToast,
		Proxy:               app.Proxy,
		HealthCheck:         app.HealthCheck,
		ProxySkipTLSVerify:  app.ProxySkipTLSVerify,
		ProxyHeaders:        proxyHeaders,
		Scale:               app.Scale,
		Shortcut:            app.Shortcut,
		MinRole:             app.MinRole,
		AllowedGroups:       app.AllowedGroups,
		ForceIconBackground: app.ForceIconBackground,
		Permissions:         app.Permissions,
		AllowNotifications:  app.AllowNotifications,
	}
	// Docker tracking fields go to admins only - they reference a
	// privileged daemon endpoint and a tracking key that's not useful
	// to non-admin users. Admins need them so the App form can show
	// the docker badge + lock-on-edit prompt.
	if isAdmin {
		out.DockerKey = app.DockerKey
		out.DockerEndpoint = app.DockerEndpoint
		out.DockerStrategy = app.DockerStrategy
		out.DockerManagedURL = app.DockerManagedURL
		// http_action headers can carry secrets (bearer tokens), so they
		// go to admins only, mirroring ProxyHeaders above.
		out.HTTPActionHeaders = app.HTTPActionHeaders
	}
	return out
}

// stripURLCredentials removes any userinfo component (user / user:pass)
// from a URL string so non-admin clients cannot read admin-embedded
// credentials out of the app config (findings.md H12). Returns the
// input unchanged if parsing fails, since an unparseable URL cannot
// leak a structured credential.
func stripURLCredentials(raw string) string {
	if raw == "" {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.User == nil {
		return raw
	}
	u.User = nil
	return u.String()
}

func stripURLCredentialsIf(strip bool, raw string) string {
	if !strip {
		return raw
	}
	return stripURLCredentials(raw)
}

// ClientAppConfig is the app config sent to the frontend (no sensitive data)
type ClientAppConfig struct {
	Name                string               `json:"name"`
	URL                 string               `json:"url"` // Original target URL (for editing/config)
	HealthURL           string               `json:"health_url,omitempty"`
	ProxyURL            string               `json:"proxyUrl,omitempty"` // Proxy path for iframe loading (when proxy enabled)
	Icon                config.AppIconConfig `json:"icon"`
	Color               string               `json:"color"`
	Group               string               `json:"group"`
	Order               int                  `json:"order"`
	Enabled             bool                 `json:"enabled"`
	Default             bool                 `json:"default"`
	OpenMode            string               `json:"open_mode"`
	// HTTP action fields. Only meaningful when OpenMode == "http_action".
	// Method/Confirm/ShowToast are non-sensitive and surface to every
	// role (the frontend needs them to decide whether to show the
	// confirmation modal and the result toast). Headers can carry
	// secrets (Authorization bearer tokens) so, like ProxyHeaders, they
	// are populated for admins only in sanitizeAppForRole.
	HTTPActionMethod    string               `json:"http_action_method,omitempty"`
	HTTPActionHeaders   map[string]string    `json:"http_action_headers,omitempty"`
	HTTPActionConfirm   bool                 `json:"http_action_confirm,omitempty"`
	HTTPActionShowToast *bool                `json:"http_action_show_toast,omitempty"`
	Proxy               bool                 `json:"proxy"`
	HealthCheck         *bool                `json:"health_check,omitempty"`          // nil/true = enabled, false = disabled
	ProxySkipTLSVerify  *bool                `json:"proxy_skip_tls_verify,omitempty"` // nil = true (default)
	ProxyHeaders        map[string]string    `json:"proxy_headers,omitempty"`
	Scale               float64              `json:"scale"`
	Shortcut            *int                 `json:"shortcut,omitempty"`
	MinRole             string               `json:"min_role,omitempty"`
	AllowedGroups       []string             `json:"allowed_groups,omitempty"`
	ForceIconBackground bool                 `json:"force_icon_background,omitempty"`
	Permissions         []string             `json:"permissions,omitempty"`
	AllowNotifications  bool                 `json:"allow_notifications,omitempty"`
	// GatewayDomain is set when a gateway site references this app via
	// `app_name`. The frontend uses this to surface a "Hosted by Muximux
	// gateway at <domain>" badge on the App form so the operator knows
	// to keep the App's URL aligned with the gateway domain. This is a
	// hint, not an enforcement: the URL field stays editable and is
	// what actually drives the dashboard iframe.
	GatewayDomain string `json:"gateway_domain,omitempty"`

	// Docker tracking fields. The frontend treats these as read-only
	// echo: it sends back what the server sent. mergeClientApp also
	// preserves them when the incoming payload omits them entirely,
	// so a buggy or scripted PUT cannot accidentally clear tracking
	// (the only sanctioned detach path is DELETE
	// /api/discovery/docker/track/{key}).
	DockerKey      string `json:"docker_key,omitempty"`
	DockerEndpoint string `json:"docker_endpoint,omitempty"`
	DockerStrategy string `json:"docker_strategy,omitempty"`
	// DockerManagedURL is round-tripped through the API so the
	// applyDockerTrackingPreservation logic can keep the field in
	// sync with URL after a successful tracking-preserving save.
	// Frontends do not need to read or display it.
	DockerManagedURL string `json:"docker_managed_url,omitempty"`
}

// sanitizeApps removes sensitive fields and filters by role and group
// membership. userRole is the requesting user's role; empty string
// disables filtering AND is treated as admin-level for compatibility
// with callers that never carried a role (e.g. unauthenticated setup
// previews). userGroups is the requesting user's group memberships
// from OIDC, forward-auth, or built-in user config; matched against
// each app's allowed_groups list when set. gatewaySites is the
// configured gateway-sites list; any site whose AppName matches an
// app surfaces as ClientAppConfig.GatewayDomain so the App form can
// show the "Hosted by Muximux gateway" badge.
func sanitizeApps(apps []config.AppConfig, userRole string, userGroups []string, gatewaySites []config.GatewaySite) []ClientAppConfig {
	isAdmin := userRole == "" || userRole == auth.RoleAdmin
	domains := gatewayDomainsByAppName(gatewaySites)
	result := make([]ClientAppConfig, 0, len(apps))
	for i := range apps {
		// Admins see all apps including disabled ones - they need to
		// manage them in the Settings UI, and any save path that
		// round-trips a sanitised /api/config response would otherwise
		// silently delete every disabled app the operator configured.
		// Non-admins only see enabled apps (disabled = hidden from
		// their menu, matching the nav-bar's own filtering).
		if !isAdmin && !apps[i].Enabled {
			continue
		}
		// Filter by minimum role if a user role is provided.
		if userRole != "" && apps[i].MinRole != "" {
			if !auth.HasMinRole(userRole, apps[i].MinRole) {
				continue
			}
		}
		// Filter by group membership when the app declares an
		// allowed_groups list. Empty list = no group gate. Matching is
		// case-insensitive to mirror the admin-group check elsewhere.
		// Admins bypass the group gate the same way they bypass min_role
		// in HasMinRole, so an operator can still see every app even
		// from a personal account that isn't in any IdP group.
		// userRole == "" disables the group check too so unauth setup
		// previews still see every app, matching the role behaviour.
		if userRole != "" && !isAdmin && len(apps[i].AllowedGroups) > 0 {
			if !userInAnyAllowedGroup(userGroups, apps[i].AllowedGroups) {
				continue
			}
		}
		client := sanitizeAppForRole(&apps[i], isAdmin)
		if d, ok := domains[apps[i].Name]; ok {
			client.GatewayDomain = d
		}
		result = append(result, client)
	}
	return result
}

// gatewayDomainsByAppName builds an app-name → gateway-domain lookup
// from the configured gateway sites. Used to set GatewayDomain on
// each ClientAppConfig in O(1) per app instead of an O(N*M) walk.
// When two sites name the same app (an operator misconfiguration) the
// last one wins; the App form will still render correctly, and the
// gateway-sites table will show both entries.
func gatewayDomainsByAppName(sites []config.GatewaySite) map[string]string {
	if len(sites) == 0 {
		return nil
	}
	out := make(map[string]string, len(sites))
	for i := range sites {
		if sites[i].AppName != "" {
			out[sites[i].AppName] = sites[i].Domain
		}
	}
	return out
}

// userInAnyAllowedGroup returns true when at least one of the user's
// groups matches one of allowed (case-insensitive). Returns false when
// the user has no groups, regardless of allowed contents.
func userInAnyAllowedGroup(userGroups, allowed []string) bool {
	if len(userGroups) == 0 {
		return false
	}
	allowSet := make(map[string]struct{}, len(allowed))
	for _, g := range allowed {
		allowSet[strings.ToLower(strings.TrimSpace(g))] = struct{}{}
	}
	for _, g := range userGroups {
		if _, ok := allowSet[strings.ToLower(strings.TrimSpace(g))]; ok {
			return true
		}
	}
	return false
}

// Slugify converts a name to a URL-safe slug
func Slugify(name string) string {
	// Slugify: lowercase, keep alphanumeric, collapse separators to single dash, trim edges
	result := make([]byte, 0, len(name))
	lastDash := true // start true to suppress leading dash
	for _, c := range name {
		switch {
		case c >= 'A' && c <= 'Z':
			result = append(result, byte(c+32)) // lowercase
			lastDash = false
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			result = append(result, byte(c)) //nolint:gosec // case guard restricts c to ASCII range
			lastDash = false
		case c == ' ', c == '-', c == '_':
			if !lastDash {
				result = append(result, '-')
				lastDash = true
			}
		}
	}
	// Trim trailing dash
	if len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	return string(result)
}

// getUserRoleAndGroups extracts both the role and the group memberships
// from the request context in one lookup. Used by handlers that need to
// run sanitizeApps, where role gates min_role and groups gate
// allowed_groups.
func getUserRoleAndGroups(r *http.Request) (string, []string) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		return "", nil
	}
	return user.Role, user.Groups
}
