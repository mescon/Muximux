package config

import "testing"

func TestEnsureGroup(t *testing.T) {
	base := []GroupConfig{{Name: "Media", Order: 0}, {Name: "Infra Tools", Order: 1}}

	for _, name := range []string{"", "   "} {
		out, g, created := EnsureGroup(base, name)
		if g != "" || created || len(out) != 2 {
			t.Errorf("EnsureGroup(%q) = %q,%v len %d", name, g, created, len(out))
		}
	}
	for in, want := range map[string]string{"Media": "Media", "media ": "Media", "MEDIA": "Media", "infra-tools": "Infra Tools", " infra_tools": "Infra Tools"} {
		out, g, created := EnsureGroup(base, in)
		if g != want || created || len(out) != 2 {
			t.Errorf("EnsureGroup(%q) = %q,%v len %d; want %q", in, g, created, len(out), want)
		}
	}

	out, g, created := EnsureGroup(base, "  Downloads ")
	if !created || g != "Downloads" || len(out) != 3 {
		t.Fatalf("create = %q,%v len %d", g, created, len(out))
	}
	want := GroupConfig{Name: "Downloads", Icon: AppIconConfig{Type: "lucide", Name: "folder"}, Order: 2, Expanded: true, DockerManaged: true}
	if out[2] != want {
		t.Errorf("new group = %+v, want %+v", out[2], want)
	}
	// Idempotent.
	out2, g2, created2 := EnsureGroup(out, "downloads")
	if created2 || g2 != "Downloads" || len(out2) != 3 {
		t.Errorf("second ensure = %q,%v len %d", g2, created2, len(out2))
	}

	// Sparse orders: the new group still sorts after every existing one.
	sparse := []GroupConfig{{Name: "A", Order: 10}, {Name: "B", Order: 3}}
	out3, _, _ := EnsureGroup(sparse, "C")
	if out3[2].Order != 11 {
		t.Errorf("order = %d, want 11", out3[2].Order)
	}

	// A name with no slug (no ASCII letters or digits) matches exactly only.
	out4, g4, created4 := EnsureGroup([]GroupConfig{{Name: "メディア"}}, "メディア")
	if created4 || g4 != "メディア" || len(out4) != 1 {
		t.Errorf("exact non-ASCII = %q,%v", g4, created4)
	}
	if _, g, c := EnsureGroup([]GroupConfig{{Name: "メディア"}}, " メディア "); c || g != "メディア" {
		t.Errorf("trimmed exact non-ASCII = %q,%v", g, c)
	}
	if _, _, c := EnsureGroup([]GroupConfig{{Name: "メディア"}}, "!!!"); !c {
		t.Error("slugless unknown name should be created")
	}
}

func TestGroupStyleEdited(t *testing.T) {
	base := GroupConfig{Name: "G", Icon: AppIconConfig{Type: "dashboard", Name: "plex"}, Color: "#111", Order: 1, DockerManaged: true}
	same := base
	same.Name, same.Expanded, same.DockerManaged = "Renamed", true, false
	if GroupStyleEdited(&base, &same) {
		t.Error("name, expanded and marker counted as a style edit")
	}
	for _, edit := range []func(g *GroupConfig){
		func(g *GroupConfig) { g.Icon.Background = "#fff" },
		func(g *GroupConfig) { g.Color = "#222" },
		func(g *GroupConfig) { g.Order = 2 },
	} {
		g := base
		edit(&g)
		if !GroupStyleEdited(&base, &g) {
			t.Errorf("edit %+v not detected", g)
		}
	}
}
