package discovery

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// boolLabelCase reads one boolean label out of a parsed label set.
type boolLabelCase struct {
	label string
	get   func(labels map[string]string) (val *bool, invalid []string)
}

func appBoolGetter(f func(*AppLabels) *bool) func(map[string]string) (*bool, []string) {
	return func(labels map[string]string) (*bool, []string) {
		l := ParseAppLabels(labels)
		return f(&l), invalidReasons(l.BoolInvalid)
	}
}

func gatewayBoolGetter(f func(*GatewayLabels) *bool) func(map[string]string) (*bool, []string) {
	return func(labels map[string]string) (*bool, []string) {
		l := ParseGatewayLabels(labels)
		return f(&l), invalidReasons(l.Invalid)
	}
}

func allBoolLabels() []boolLabelCase {
	return []boolLabelCase{
		{LabelAppEnabled, appBoolGetter(func(l *AppLabels) *bool { return l.Enabled })},
		{LabelAppDefault, appBoolGetter(func(l *AppLabels) *bool { return l.Default })},
		{LabelAppProxy, appBoolGetter(func(l *AppLabels) *bool { return l.Proxy })},
		{LabelAppProxySkipTLSVerify, appBoolGetter(func(l *AppLabels) *bool { return l.ProxySkipTLSVerify })},
		{LabelAppAllowNotifications, appBoolGetter(func(l *AppLabels) *bool { return l.AllowNotifications })},
		{LabelAppHealthCheck, appBoolGetter(func(l *AppLabels) *bool { return l.HealthCheck })},
		{LabelAppHTTPActionConfirm, appBoolGetter(func(l *AppLabels) *bool { return l.HTTPActionConfirm })},
		{LabelAppHTTPActionShowToast, appBoolGetter(func(l *AppLabels) *bool { return l.HTTPActionShowToast })},
		{LabelGatewayStreaming, gatewayBoolGetter(func(l *GatewayLabels) *bool { return l.Streaming })},
		{LabelGatewayStripFrameBlockers, gatewayBoolGetter(func(l *GatewayLabels) *bool { return l.StripFrameBlockers })},
		{LabelGatewayForwardedHeaders, gatewayBoolGetter(func(l *GatewayLabels) *bool { return l.ForwardedHeaders })},
		{LabelGatewaySkipTLSVerify, gatewayBoolGetter(func(l *GatewayLabels) *bool { return l.SkipTLSVerify })},
		{LabelGatewayRequireAuth, gatewayBoolGetter(func(l *GatewayLabels) *bool { return l.RequireAuth })},
	}
}

func TestBoolLabels_Strict(t *testing.T) {
	for _, bl := range allBoolLabels() {
		t.Run(bl.label, func(t *testing.T) {
			for _, v := range []string{"true", "TRUE", " 1 ", "yes", "On"} {
				got, inv := bl.get(map[string]string{bl.label: v})
				if got == nil || !*got || len(inv) != 0 {
					t.Errorf("%q = %v, %v; want true", v, got, inv)
				}
			}
			for _, v := range []string{"false", "FALSE", " 0 ", "no", "Off"} {
				got, inv := bl.get(map[string]string{bl.label: v})
				if got == nil || *got || len(inv) != 0 {
					t.Errorf("%q = %v, %v; want false", v, got, inv)
				}
			}
			// A typo is ignored as if unset (never an opt-out) and reported.
			for _, v := range []string{"tru", "flase", "2", "enabled"} {
				got, inv := bl.get(map[string]string{bl.label: v})
				if got != nil {
					t.Errorf("%q set the label to %v, want unset", v, *got)
				}
				if len(inv) != 1 || !strings.Contains(inv[0], bl.label) || !strings.Contains(inv[0], v) {
					t.Errorf("%q invalid = %v", v, inv)
				}
			}
			// Empty or blank is unset, not an error.
			for _, v := range []string{"", "   "} {
				got, inv := bl.get(map[string]string{bl.label: v})
				if got != nil || len(inv) != 0 {
					t.Errorf("%q = %v, %v; want unset and no note", v, got, inv)
				}
			}
		})
	}
}

func TestBoolLabels_InvalidIsSortedAndTypedLabelsUnaffected(t *testing.T) {
	l := ParseAppLabels(map[string]string{
		LabelAppProxy:   "maybe",
		LabelAppEnabled: "tru",
		LabelAppPort:    "8080",
	})
	if len(l.BoolInvalid) != 2 || l.BoolInvalid[0].Name != LabelAppEnabled || l.BoolInvalid[1].Name != LabelAppProxy {
		t.Errorf("BoolInvalid = %v", l.BoolInvalid)
	}
	if l.Port != 8080 || l.Enabled != nil || l.Proxy != nil {
		t.Errorf("parsed = %+v", l)
	}
}

func TestSuggest_InvalidBoolLabelNotesAndDoesNotOptOut(t *testing.T) {
	c := ContainerSummary{
		ID: "abc", Names: []string{"/app"}, Image: "x/app:1", State: "running",
		Ports: []ContainerPort{{PrivatePort: 80, Type: "tcp"}},
		Labels: map[string]string{
			LabelAppEnabled:         "tru",
			LabelAppGatewayDomain:   "app.example.com",
			LabelGatewayRequireAuth: "ture",
		},
	}
	s := suggestForContainer(&c, config.StrategyContainerIP, "", "dash.example.com")
	if s.LabelEnabled != nil {
		t.Errorf("LabelEnabled = %v, want nil (typo must not opt out)", *s.LabelEnabled)
	}
	notes := strings.Join(s.Notes, "\n")
	for _, want := range []string{"Boolean label ignored: " + LabelAppEnabled, "Boolean label ignored: " + LabelGatewayRequireAuth} {
		if !strings.Contains(notes, want) {
			t.Errorf("missing note %q in %q", want, notes)
		}
	}
}

func TestWarnInvalidBoolLabels_OncePerTransition(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	warns := func() int { return strings.Count(buf.String(), "Docker boolean label ignored: invalid value") }
	bad := ContainerSummary{ID: "1", Names: []string{"/a"}, Labels: map[string]string{LabelDiscoveryID: "a", LabelAppEnabled: "tru"}}
	gw := ContainerSummary{ID: "2", Names: []string{"/b"}, Labels: map[string]string{LabelDiscoveryID: "b", LabelGatewayStreaming: "nope"}}
	ok := ContainerSummary{ID: "3", Names: []string{"/c"}, Labels: map[string]string{LabelDiscoveryID: "c", LabelAppEnabled: "true"}}
	p := &Poller{}

	p.warnInvalidBoolLabels([]ContainerSummary{bad, gw, ok})
	if len(p.boolLabelInvalid) != 2 || !strings.Contains(p.boolLabelInvalid["label:a"], LabelAppEnabled) ||
		!strings.Contains(p.boolLabelInvalid["label:b"], LabelGatewayStreaming) {
		t.Fatalf("boolLabelInvalid = %v", p.boolLabelInvalid)
	}
	if warns() != 2 {
		t.Fatalf("warnings = %d, want 2 (one per container)", warns())
	}
	// Same state again: still recorded, no new transition, no new WARN.
	before := p.boolLabelInvalid["label:a"]
	p.warnInvalidBoolLabels([]ContainerSummary{bad, gw, ok})
	if warns() != 2 {
		t.Errorf("warnings after an unchanged tick = %d, want 2", warns())
	}
	// The label changes to another bad value: a new transition warns again.
	bad2 := ContainerSummary{ID: "1", Names: []string{"/a"}, Labels: map[string]string{LabelDiscoveryID: "a", LabelAppEnabled: "flase"}}
	p.warnInvalidBoolLabels([]ContainerSummary{bad2, gw, ok})
	if warns() != 3 {
		t.Errorf("warnings after a changed label = %d, want 3", warns())
	}
	p.warnInvalidBoolLabels([]ContainerSummary{bad, gw, ok})
	if p.boolLabelInvalid["label:a"] != before {
		t.Errorf("state changed: %v", p.boolLabelInvalid)
	}
	// Fixed labels are forgotten.
	p.warnInvalidBoolLabels([]ContainerSummary{ok})
	if len(p.boolLabelInvalid) != 0 {
		t.Errorf("boolLabelInvalid = %v, want empty", p.boolLabelInvalid)
	}
}

func TestSuggest_InvalidGatewayBoolWithoutDomainStillNoted(t *testing.T) {
	c := ContainerSummary{
		ID: "abc", Names: []string{"/app"}, Image: "x/app:1", State: "running",
		Ports:  []ContainerPort{{PrivatePort: 80, Type: "tcp"}},
		Labels: map[string]string{LabelAppName: "A", LabelGatewayRequireAuth: "ture"},
	}
	s := suggestForContainer(&c, config.StrategyContainerIP, "", "")
	if !strings.Contains(strings.Join(s.Notes, "\n"), "Boolean label ignored: "+LabelGatewayRequireAuth) {
		t.Errorf("notes = %v", s.Notes)
	}
}

// A misspelt boolean keeps the stored value on update and sync, and a new
// import uses the default.
func TestReconcile_InvalidBoolKeepsStoredValue(t *testing.T) {
	typo := func(extra map[string]string) *Suggestion {
		labels := map[string]string{
			LabelAppEnabled:       "true",
			LabelAppName:          "App",
			LabelAppGatewayDomain: "app.example.com",
		}
		for k, v := range extra {
			labels[k] = v
		}
		c := ContainerSummary{
			ID: "abc", Names: []string{"/app"}, Image: "x/app:1", State: "running",
			Ports:  []ContainerPort{{PrivatePort: 80, Type: "tcp"}},
			Labels: labels,
		}
		s := suggestForContainer(&c, config.StrategyContainerIP, "", "dash.example.com")
		return &s
	}
	bad := map[string]string{
		LabelGatewayRequireAuth:     "ture",
		LabelGatewayStreaming:       "ture",
		LabelAppDefault:             "ture",
		LabelAppProxy:               "ture",
		LabelAppAllowNotifications:  "ture",
		LabelAppProxySkipTLSVerify:  "ture",
		LabelAppHTTPActionConfirm:   "ture",
		LabelAppHTTPActionShowToast: "ture",
	}
	sug := typo(bad)
	d := BuildDesired(sug, "ep")
	if d.Site == nil || d.Site.RequireAuth {
		t.Fatalf("desired site = %+v", d.Site)
	}

	// A new import gets the defaults.
	for _, mode := range []config.AutoImportMode{config.AutoImportAdd, config.AutoImportUpdate, config.AutoImportSync} {
		plan := Reconcile(&ReconcileInput{Mode: mode, Desired: []Desired{d}})
		if len(plan.Add) != 1 || plan.Add[0].Site.RequireAuth || plan.Add[0].App.Default {
			t.Fatalf("%s: add = %+v", mode, plan.Add)
		}
	}

	// An existing app and site with every flag on keeps them.
	on := true
	good := BuildDesired(typo(map[string]string{}), "ep")
	app := good.App
	app.Default, app.Proxy, app.AllowNotifications = true, true, true
	app.ProxySkipTLSVerify, app.HTTPActionConfirm, app.HTTPActionShowToast = &on, true, &on
	site := *good.Site
	site.RequireAuth, site.Streaming = true, true

	for _, mode := range []config.AutoImportMode{config.AutoImportUpdate, config.AutoImportSync} {
		plan := Reconcile(&ReconcileInput{
			Mode: mode, Desired: []Desired{d},
			Current: []config.AppConfig{app}, CurrentSites: []config.GatewaySite{site},
		})
		// Nothing else differs, so a typo alone causes no update.
		if len(plan.Update) != 0 {
			t.Errorf("%s: typo alone caused an update: %+v", mode, plan.Update)
		}
	}

	// With another label change the update proceeds but keeps the values.
	d2 := BuildDesired(typo(mergeMaps(bad, map[string]string{LabelAppName: "Renamed"})), "ep")
	for _, mode := range []config.AutoImportMode{config.AutoImportUpdate, config.AutoImportSync} {
		plan := Reconcile(&ReconcileInput{
			Mode: mode, Desired: []Desired{d2},
			Current: []config.AppConfig{app}, CurrentSites: []config.GatewaySite{site},
		})
		if len(plan.Update) != 1 {
			t.Fatalf("%s: updates = %d", mode, len(plan.Update))
		}
		u := plan.Update[0]
		merged := mergeManagedFields(&app, &u.App)
		if merged.Name != "Renamed" || !merged.Default || !merged.AllowNotifications || !merged.HTTPActionConfirm ||
			merged.ProxySkipTLSVerify == nil || !*merged.ProxySkipTLSVerify || merged.HTTPActionShowToast == nil || !*merged.HTTPActionShowToast {
			t.Errorf("%s: merged app = %+v", mode, merged)
		}
		if u.Site == nil || !u.Site.RequireAuth || !u.Site.Streaming {
			t.Errorf("%s: site = %+v, want require_auth and streaming kept", mode, u.Site)
		}
	}
	// The label really was a typo: a valid false does flip the site public.
	flip := BuildDesired(typo(map[string]string{LabelGatewayRequireAuth: "false"}), "ep")
	plan := Reconcile(&ReconcileInput{Mode: config.AutoImportSync, Desired: []Desired{flip},
		Current: []config.AppConfig{app}, CurrentSites: []config.GatewaySite{site}})
	if len(plan.Update) != 1 || plan.Update[0].Site.RequireAuth {
		t.Errorf("explicit false did not apply: %+v", plan.Update)
	}
}

func mergeMaps(a, b map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
