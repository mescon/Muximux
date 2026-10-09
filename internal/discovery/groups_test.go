package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/mescon/muximux/v3/internal/config"
)

// Groups an auto-imported app names are created in the same save (#500).

func groupNames(cfg *config.Config) []string {
	out := make([]string, 0, len(cfg.Groups))
	for i := range cfg.Groups {
		out = append(out, cfg.Groups[i].Name)
	}
	return out
}

func TestTick_AutoImportCreatesMissingGroup(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportUpdate)
	f.cfg.Apps = nil
	f.label(LabelAppGroup, "Downloads")
	f.p.tick(context.Background())

	a := findAppByKey(f.cfg, lsKey)
	if a == nil || a.Group != "Downloads" {
		t.Fatalf("app not added to its group: %+v", a)
	}
	want := config.NewAutoGroup("Downloads", 2)
	if len(f.cfg.Groups) != 3 || f.cfg.Groups[2] != want {
		t.Fatalf("groups = %+v, want Downloads appended", f.cfg.Groups)
	}
	if f.saves != 1 {
		t.Errorf("saves = %d, want 1 (app and group in one save)", f.saves)
	}
	if err := f.cfg.Validate(); err != nil {
		t.Fatalf("Validate() = %v", err)
	}
	saveAndLoad(t, f.cfg)

	// Idempotent: the next tick finds the group and writes nothing.
	f.p.tick(context.Background())
	if f.saves != 1 || len(f.cfg.Groups) != 3 {
		t.Errorf("second tick saves=%d groups=%v", f.saves, groupNames(f.cfg))
	}
}

func TestTick_AutoImportGroupMatchedBySlugNoFlap(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportUpdate)
	f.cfg.Apps = nil
	f.label(LabelAppGroup, "media")
	f.p.tick(context.Background())

	if a := findAppByKey(f.cfg, lsKey); a == nil || a.Group != "Media" {
		t.Fatalf("app = %+v, want group Media", a)
	}
	if len(f.cfg.Groups) != 2 {
		t.Errorf("near-duplicate group created: %v", groupNames(f.cfg))
	}
	saves := f.saves
	f.p.tick(context.Background())
	if f.saves != saves {
		t.Errorf("canonical group flapped: %d more saves", f.saves-saves)
	}
}

func TestTick_AutoImportUpdateCreatesGroupAndRollsBack(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportSync)
	f.cfg.Apps = nil
	f.p.tick(context.Background()) // adds the app in the catalog group
	if a := findAppByKey(f.cfg, lsKey); a == nil || a.Group != "Media" {
		t.Fatalf("app = %+v", a)
	}

	f.label(LabelAppGroup, "Tools")
	f.fail = errors.New("disk full")
	f.p.tick(context.Background())
	if a := findAppByKey(f.cfg, lsKey); a.Group != "Media" {
		t.Errorf("app group not rolled back: %q", a.Group)
	}
	if len(f.cfg.Groups) != 2 {
		t.Errorf("group creation not rolled back: %v", groupNames(f.cfg))
	}

	f.fail = nil
	f.p.tick(context.Background())
	if a := findAppByKey(f.cfg, lsKey); a.Group != "Tools" {
		t.Errorf("update did not apply: %q", a.Group)
	}
	if got := groupNames(f.cfg); len(got) != 3 || got[2] != "Tools" {
		t.Errorf("groups = %v", got)
	}
}

// One tick that both adds an app and re-syncs a tracked one into the same
// new group (spelled differently) creates the group once.
func TestTick_SameNewGroupFromTwoPathsCreatedOnce(t *testing.T) {
	f := newLabelSyncFixture(t, config.AutoImportAdd)
	f.label(LabelAppGroup, "infra-tools")
	other := labeledSonarr()
	other.ID = "auto-radarr"
	other.Names = []string{"/radarr"}
	other.Labels = map[string]string{LabelDiscoveryID: "radarr-auto", LabelAppGroup: "Infra Tools"}
	f.set = append(f.set, other)
	f.p.tick(context.Background())

	if got := groupNames(f.cfg); len(got) != 3 {
		t.Fatalf("groups = %v, want one new group", got)
	}
	created := f.cfg.Groups[2].Name
	for i := range f.cfg.Apps {
		if f.cfg.Apps[i].Group != created {
			t.Errorf("app %s group = %q, want %q", f.cfg.Apps[i].Name, f.cfg.Apps[i].Group, created)
		}
	}
	if f.saves != 1 {
		t.Errorf("saves = %d", f.saves)
	}
	saves := f.saves
	f.p.tick(context.Background())
	if f.saves != saves {
		t.Errorf("tick after create saved again")
	}
}
