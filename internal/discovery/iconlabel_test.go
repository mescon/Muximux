package discovery

import (
	"context"
	"strings"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// dashIcon is a Dashboard Icons IconRef, for tests.
func dashIcon(slug string) IconRef { return IconRef{Type: "dashboard", Name: slug} }

func TestParseIconLabel(t *testing.T) {
	long := "https://example.com/" + strings.Repeat("a", 2048)
	cases := []struct {
		name, in string
		want     IconRef
		invalid  bool
	}{
		{"blank", "  ", IconRef{}, false},
		{"bare slug", "sonarr", dashIcon("sonarr"), false},
		{"bare slug trimmed and lowercased", " Traefik-Proxy\t", dashIcon("traefik-proxy"), false},
		{"bare server stays dashboard", "server", dashIcon("server"), false},
		{"dashboard prefix", "dashboard:Sonarr", dashIcon("sonarr"), false},
		{"dashboard prefix any case", "DASHBOARD: plex", dashIcon("plex"), false},
		{"lucide", "lucide:shield-check", IconRef{Type: "lucide", Name: "shield-check"}, false},
		{"lucide prefix and name case", "Lucide:Server", IconRef{Type: "lucide", Name: "server"}, false},
		{"lucide digits", "lucide:grid-3x3", IconRef{Type: "lucide", Name: "grid-3x3"}, false},
		{"https url", "https://example.com/Icons/Media.SVG", IconRef{Type: "url", URL: "https://example.com/Icons/Media.SVG"}, false},
		{"http url refused", " http://example.com:8080/i.png ", IconRef{}, true},
		{"url keeps query case", "https://example.com/i?Size=L", IconRef{Type: "url", URL: "https://example.com/i?Size=L"}, false},
		{"scheme case", "HTTPS://example.com/i.svg", IconRef{Type: "url", URL: "HTTPS://example.com/i.svg"}, false},
		{"url prefix", "url:https://example.com/i.svg", IconRef{Type: "url", URL: "https://example.com/i.svg"}, false},
		{"url prefix any case", "URL: https://example.com/i.svg", IconRef{Type: "url", URL: "https://example.com/i.svg"}, false},

		{"lucide empty", "lucide:", IconRef{}, true},
		{"lucide underscore", "lucide:shield_check", IconRef{}, true},
		{"lucide space", "lucide:shield check", IconRef{}, true},
		{"lucide leading dash", "lucide:-x", IconRef{}, true},
		{"lucide path", "lucide:../x", IconRef{}, true},
		{"lucide too long", "lucide:" + strings.Repeat("a", 65), IconRef{}, true},
		{"dashboard empty", "dashboard:", IconRef{}, true},
		{"bare unknown scheme", "foo:bar", IconRef{}, true},
		{"bare path", "a/b", IconRef{}, true},
		{"bare inner space", "my icon", IconRef{}, true},
		{"javascript url", "javascript:alert(1)", IconRef{}, true},
		{"javascript via url prefix", "url:javascript:alert(1)", IconRef{}, true},
		{"data url", "data:image/svg+xml;base64,PHN2Zz4=", IconRef{}, true},
		{"data via url prefix", "url:data:image/png;base64,AA", IconRef{}, true},
		{"file url", "url:file:///etc/passwd", IconRef{}, true},
		{"ftp url", "url:ftp://example.com/i.png", IconRef{}, true},
		{"relative url", "url:/icons/x.svg", IconRef{}, true},
		{"url no host", "https://", IconRef{}, true},
		{"url empty", "url:", IconRef{}, true},
		{"url credentials", "https://user:pw@example.com/i.svg", IconRef{}, true},
		{"url whitespace", "https://example.com/a b.svg", IconRef{}, true},
		{"url control char", "https://example.com/a\x00b.svg", IconRef{}, true},
		{"url too long", long, IconRef{}, true},
		{"url unparsable", "https://exa mple.com", IconRef{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, why := ParseIconLabel(tc.in)
			if got != tc.want {
				t.Errorf("ParseIconLabel(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
			if (why != "") != tc.invalid {
				t.Errorf("ParseIconLabel(%q) reason = %q, invalid want %v", tc.in, why, tc.invalid)
			}
		})
	}
}

func TestIconRef_ApplyMatchesConfig(t *testing.T) {
	var zero IconRef
	if zero.IsSet() || zero.Value() != "" {
		t.Error("zero IconRef reports set")
	}
	for _, ref := range []IconRef{dashIcon("sonarr"), {Type: "lucide", Name: "server"}, {Type: "url", URL: "https://example.com/i.svg"}} {
		ic := config.AppIconConfig{Type: "custom", File: "x.png", Name: "old", URL: "https://old.example.com/i", Color: "#123", Variant: "png", Invert: true}
		if ref.Matches(&ic) {
			t.Errorf("%+v matches a custom icon", ref)
		}
		ref.Apply(&ic)
		if !ref.Matches(&ic) || !labelIconSynced(&ic, ref) {
			t.Errorf("%+v not in sync after Apply: %+v", ref, ic)
		}
		if ic.File != "" || ic.Color != "#123" || ic.Variant != "png" || !ic.Invert {
			t.Errorf("Apply cleared styling or kept the file: %+v", ic)
		}
		if ref.Type == "url" && (ic.Name != "" || ic.URL != ref.URL) {
			t.Errorf("url icon = %+v", ic)
		}
		if ref.Type != "url" && (ic.URL != "" || ic.Name != ref.Name) {
			t.Errorf("named icon = %+v", ic)
		}
		if ref.Config().Type != ref.Type || ref.Value() == "" {
			t.Errorf("Config/Value of %+v", ref)
		}
	}
	// A stale URL left on a named icon is not "in sync": it gets cleaned.
	if labelIconSynced(&config.AppIconConfig{Type: "lucide", Name: "tv", URL: "https://x.example.com/i"}, IconRef{Type: "lucide", Name: "tv"}) {
		t.Error("stale url counted as synced")
	}
}

func TestIconRefOf(t *testing.T) {
	if got := iconRefOf("sonarr", ""); got != dashIcon("sonarr") {
		t.Errorf("default = %+v", got)
	}
	if got := iconRefOf("tv", "lucide"); got != (IconRef{Type: "lucide", Name: "tv"}) {
		t.Errorf("lucide = %+v", got)
	}
	if got := iconRefOf("https://example.com/i.svg", "url"); got != (IconRef{Type: "url", URL: "https://example.com/i.svg"}) {
		t.Errorf("url = %+v", got)
	}
}

func TestParseAppLabels_Icon(t *testing.T) {
	got := ParseAppLabels(map[string]string{LabelAppIcon: "lucide:database"})
	if got.Icon != (IconRef{Type: "lucide", Name: "database"}) || got.IconInvalid != "" {
		t.Errorf("lucide label = %+v", got)
	}
	bad := ParseAppLabels(map[string]string{LabelAppIcon: "url:javascript:alert(1)"})
	if bad.Icon.IsSet() || bad.IconInvalid == "" {
		t.Errorf("javascript icon accepted: %+v", bad)
	}
}

func TestSuggestion_IconLabels(t *testing.T) {
	for _, tc := range []struct{ label, wantIcon, wantType string }{
		{"lucide:server", "server", "lucide"},
		{"https://example.com/i.svg", "https://example.com/i.svg", "url"},
		{"traefik", "traefik", "dashboard"},
	} {
		c := labeledSonarr()
		c.Labels[LabelAppIcon] = tc.label
		s := suggestForContainer(&c, config.StrategyContainerIP, "", "")
		if s.Icon != tc.wantIcon || s.IconType != tc.wantType {
			t.Errorf("%q: icon %q type %q, want %q %q", tc.label, s.Icon, s.IconType, tc.wantIcon, tc.wantType)
		}
	}

	// An invalid label is ignored with a note; the catalog icon still applies.
	c := labeledSonarr()
	c.Labels[LabelAppIcon] = "data:image/png;base64,AA"
	s := suggestForContainer(&c, config.StrategyContainerIP, "", "")
	if !hasNoteContaining(s.Notes, "muximux.app.icon ignored") {
		t.Errorf("notes = %v, want an ignored-icon note", s.Notes)
	}
	if strings.Contains(s.Icon, "data:") {
		t.Errorf("invalid icon leaked into the suggestion: %q", s.Icon)
	}
}

func TestBuildDesired_IconTypes(t *testing.T) {
	for _, tc := range []struct {
		sug  Suggestion
		want config.AppIconConfig
	}{
		{Suggestion{Icon: "sonarr"}, config.AppIconConfig{Type: "dashboard", Name: "sonarr"}},
		{Suggestion{Icon: "server", IconType: "lucide"}, config.AppIconConfig{Type: "lucide", Name: "server"}},
		{Suggestion{Icon: "https://example.com/i.svg", IconType: "url"}, config.AppIconConfig{Type: "url", URL: "https://example.com/i.svg"}},
	} {
		sug := tc.sug
		sug.Name, sug.URL = "App", "http://10.0.0.5:80"
		if got := BuildDesired(&sug, "ep").App.Icon; got != tc.want {
			t.Errorf("icon = %+v, want %+v", got, tc.want)
		}
	}
}

func TestParseGroupLabels_IconForms(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want IconRef
	}{
		{"plex", dashIcon("plex")},
		{"dashboard:Plex", dashIcon("plex")},
		{"lucide:folder-open", IconRef{Type: "lucide", Name: "folder-open"}},
		{"https://example.com/icons/media.svg", IconRef{Type: "url", URL: "https://example.com/icons/media.svg"}},
	} {
		got := ParseGroupLabels(map[string]string{LabelGroupIcon: tc.in})
		if got.Icon != tc.want || len(got.Invalid) != 0 {
			t.Errorf("%q = %+v", tc.in, got)
		}
	}
	bad := ParseGroupLabels(map[string]string{LabelGroupIcon: "javascript:alert(1)", LabelGroupColor: "#fff"})
	if bad.Icon.IsSet() || len(bad.Invalid) != 1 || !strings.Contains(bad.Invalid[0], LabelGroupIcon) || bad.Color != "#fff" {
		t.Errorf("invalid group icon = %+v", bad)
	}
}

func TestTick_GroupIconLabelForms(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	f.cfg.Groups[0].DockerManaged = true
	f.cfg.Groups[0].Icon = config.AppIconConfig{Type: "custom", File: "m.png", Background: "#000"}

	f.label(LabelGroupIcon, "https://example.com/icons/media.svg")
	f.p.tick(context.Background())
	if ic := findGroup(f.cfg, "Media").Icon; ic.Type != "url" || ic.URL != "https://example.com/icons/media.svg" || ic.File != "" || ic.Name != "" || ic.Background != "#000" {
		t.Fatalf("url group icon = %+v", ic)
	}
	f.label(LabelGroupIcon, "lucide:clapperboard")
	f.p.tick(context.Background())
	if ic := findGroup(f.cfg, "Media").Icon; ic.Type != "lucide" || ic.Name != "clapperboard" || ic.URL != "" {
		t.Fatalf("lucide group icon = %+v", ic)
	}
	saves := f.saves
	f.p.tick(context.Background())
	if f.saves != saves {
		t.Errorf("steady tick saved %d times", f.saves-saves)
	}

	// Invalid value: keeps the icon, still reported, discovery goes on.
	f.label(LabelGroupIcon, "data:image/png;base64,AA")
	f.p.tick(context.Background())
	if ic := findGroup(f.cfg, "Media").Icon; ic.Type != "lucide" || ic.Name != "clapperboard" {
		t.Errorf("invalid label changed the icon: %+v", ic)
	}
	if f.p.groupLabelInvalid[lsKey] == "" {
		t.Errorf("invalid group icon not recorded: %v", f.p.groupLabelInvalid)
	}
}

func TestLabelSync_AppIconForms(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportOff)
	for _, tc := range []struct {
		label string
		want  config.AppIconConfig
	}{
		{"https://example.com/sonarr.png", config.AppIconConfig{Type: "url", URL: "https://example.com/sonarr.png", Color: "#123456"}},
		{"lucide:server", config.AppIconConfig{Type: "lucide", Name: "server", Color: "#123456"}},
		{"url:https://example.com/s.svg", config.AppIconConfig{Type: "url", URL: "https://example.com/s.svg", Color: "#123456"}},
		{"dashboard:sonarr", config.AppIconConfig{Type: "dashboard", Name: "sonarr", Color: "#123456"}},
	} {
		f.label(LabelAppIcon, tc.label)
		f.p.tick(context.Background())
		if got := findAppByKey(f.cfg, lsKey).Icon; got != tc.want {
			t.Errorf("%q: icon = %+v, want %+v", tc.label, got, tc.want)
		}
	}

	// Invalid: keeps the stored icon, one WARN state per transition.
	saves := f.saves
	f.label(LabelAppIcon, "javascript:alert(1)")
	f.p.tick(context.Background())
	if got := findAppByKey(f.cfg, lsKey).Icon; got.Type != "dashboard" || got.Name != "sonarr" {
		t.Errorf("invalid label changed the icon: %+v", got)
	}
	if f.saves != saves {
		t.Errorf("invalid icon caused a save")
	}
	if !strings.Contains(f.p.labelHeld[lsKey], "icon label") {
		t.Errorf("held = %q, want the icon reason", f.p.labelHeld[lsKey])
	}
	f.label(LabelAppIcon, "lucide:server")
	f.p.tick(context.Background())
	if _, held := f.p.labelHeld[lsKey]; held {
		t.Error("hold not cleared once the label is valid")
	}
}
