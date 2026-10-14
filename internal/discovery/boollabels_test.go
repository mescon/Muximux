package discovery

import (
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
		return f(&l), l.BoolInvalid
	}
}

func gatewayBoolGetter(f func(*GatewayLabels) *bool) func(map[string]string) (*bool, []string) {
	return func(labels map[string]string) (*bool, []string) {
		l := ParseGatewayLabels(labels)
		return f(&l), l.Invalid
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
	if len(l.BoolInvalid) != 2 || !strings.HasPrefix(l.BoolInvalid[0], LabelAppEnabled) || !strings.HasPrefix(l.BoolInvalid[1], LabelAppProxy) {
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
	bad := ContainerSummary{ID: "1", Names: []string{"/a"}, Labels: map[string]string{LabelDiscoveryID: "a", LabelAppEnabled: "tru"}}
	gw := ContainerSummary{ID: "2", Names: []string{"/b"}, Labels: map[string]string{LabelDiscoveryID: "b", LabelGatewayStreaming: "nope"}}
	ok := ContainerSummary{ID: "3", Names: []string{"/c"}, Labels: map[string]string{LabelDiscoveryID: "c", LabelAppEnabled: "true"}}
	p := &Poller{}

	p.warnInvalidBoolLabels([]ContainerSummary{bad, gw, ok})
	if len(p.boolLabelInvalid) != 2 || !strings.Contains(p.boolLabelInvalid["label:a"], LabelAppEnabled) ||
		!strings.Contains(p.boolLabelInvalid["label:b"], LabelGatewayStreaming) {
		t.Fatalf("boolLabelInvalid = %v", p.boolLabelInvalid)
	}
	// Same state again: still recorded, no new transition.
	before := p.boolLabelInvalid["label:a"]
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
