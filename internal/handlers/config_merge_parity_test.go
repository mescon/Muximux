package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// mergeParityFixture is shared with web/src/lib/configMerge.parity.test.ts:
// the server merge and the client rebase must reach the same result on it.
type mergeParityFixture struct {
	BaseServer ClientConfigUpdate `json:"base_server"`
	BaseSent   ClientConfigUpdate `json:"base_sent"`
	Mine       ClientConfigUpdate `json:"mine"`
	Theirs     ClientConfigUpdate `json:"theirs"`
	Expected   struct {
		Title  string   `json:"title"`
		Apps   []string `json:"apps"`
		Groups []string `json:"groups"`
	} `json:"expected"`
}

func loadMergeParityFixture(t *testing.T) *mergeParityFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "merge_parity_sparse.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f mergeParityFixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	return &f
}

func mergedNames(got *ClientConfigUpdate) (apps, groups []string) {
	apps, groups = []string{}, []string{}
	for i := range got.Apps {
		apps = append(apps, got.Apps[i].Name)
	}
	for i := range got.Groups {
		groups = append(groups, got.Groups[i].Name)
	}
	return apps, groups
}

// S-05: the base Settings sends is normalised like its payload, so an
// untouched app the server removed (auto-import sync) is not saved back.
func TestMergeThreeWay_ParityFixtureDropsServerRemovedSparseApp(t *testing.T) {
	f := loadMergeParityFixture(t)
	got := mustMergeThreeWay(t, &f.BaseSent, &f.Mine, &f.Theirs)
	apps, groups := mergedNames(got)
	if got.Title != f.Expected.Title || !reflect.DeepEqual(apps, f.Expected.Apps) || !reflect.DeepEqual(groups, f.Expected.Groups) {
		t.Errorf("title %q apps %v groups %v, want %q %v %v", got.Title, apps, groups, f.Expected.Title, f.Expected.Apps, f.Expected.Groups)
	}
}

// The raw sparse base is what Settings used to send: the defaults its
// payload carries (scale 1, a group colour) read as edits and resurrect
// the removed items. This pins why the client must normalise the base.
func TestMergeThreeWay_ParityFixtureRawBaseReadsDefaultsAsEdits(t *testing.T) {
	f := loadMergeParityFixture(t)
	got := mustMergeThreeWay(t, &f.BaseServer, &f.Mine, &f.Theirs)
	apps, groups := mergedNames(got)
	if !reflect.DeepEqual(apps, []string{"Plex", "Whoami"}) || !reflect.DeepEqual(groups, []string{"Media", "Lab"}) {
		t.Errorf("apps %v groups %v", apps, groups)
	}
}
