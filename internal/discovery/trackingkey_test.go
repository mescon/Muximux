package discovery

import (
	"errors"
	"testing"
)

func TestParseTrackingKey(t *testing.T) {
	cases := []struct {
		in      string
		want    TrackingKey
		wantErr bool
	}{
		{"label:sonarr-prod", TrackingKey{Source: KeySourceLabel, Value: "sonarr-prod"}, false},
		{"name:mxtest-foo", TrackingKey{Source: KeySourceName, Value: "mxtest-foo"}, false},
		{"id:deadbeef", TrackingKey{Source: KeySourceID, Value: "deadbeef"}, false},
		// Edge cases the parser must reject:
		{"", TrackingKey{}, true},          // empty input
		{"foo", TrackingKey{}, true},       // missing colon
		{":foo", TrackingKey{}, true},      // empty source
		{"label:", TrackingKey{}, true},    // empty value
		{"magic:foo", TrackingKey{}, true}, // unknown source
		{"Label:foo", TrackingKey{}, true}, // case-sensitive
		{"label:foo:bar", TrackingKey{Source: KeySourceLabel, Value: "foo:bar"}, false}, // multi-colon: only first split matters
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := ParseTrackingKey(c.in)
			if (err != nil) != c.wantErr {
				t.Fatalf("ParseTrackingKey(%q) err=%v wantErr=%v", c.in, err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("ParseTrackingKey(%q) = %+v, want %+v", c.in, got, c.want)
			}
			if c.wantErr && !errors.Is(err, errMalformedTrackingKey) {
				t.Errorf("ParseTrackingKey(%q) returned err %v, want errMalformedTrackingKey", c.in, err)
			}
		})
	}
}

func TestTrackingKey_String_RoundTrips(t *testing.T) {
	for _, raw := range []string{"label:foo", "name:bar", "id:abc123def"} {
		tk, err := ParseTrackingKey(raw)
		if err != nil {
			t.Fatalf("Parse(%q): %v", raw, err)
		}
		if got := tk.String(); got != raw {
			t.Errorf("String() = %q, want %q", got, raw)
		}
	}
}

func TestTrackingKey_MatchContainer(t *testing.T) {
	containers := []ContainerSummary{
		{
			ID:     "abc1234567890",
			Names:  []string{"/sonarr"},
			Labels: map[string]string{LabelDiscoveryID: "sonarr-stable"},
		},
		{
			ID:    "def0987654321",
			Names: []string{"/radarr"},
		},
	}

	cases := []struct {
		key      string
		wantName string // primary name of the matched container, "" if none
	}{
		{"label:sonarr-stable", "sonarr"},
		{"name:radarr", "radarr"},
		{"id:def0987654321", "radarr"},
		{"id:def0987", "radarr"}, // prefix match
		{"id:abc", "sonarr"},
		// Mismatches
		{"label:absent", ""},
		{"name:nonexistent", ""},
		{"id:zzz", ""},
		{"id:def09876543210", ""}, // longer than stored ID
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			tk, err := ParseTrackingKey(c.key)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got := tk.FindContainer(containers)
			if c.wantName == "" {
				if got != nil {
					t.Errorf("FindContainer found %q, expected nil", got.PrimaryName())
				}
				return
			}
			if got == nil || got.PrimaryName() != c.wantName {
				t.Errorf("FindContainer matched %v, want %q", got, c.wantName)
			}
		})
	}
}

func TestTrackingKey_ParseAndMatchSwarmCompose(t *testing.T) {
	c := ContainerSummary{ID: "1", Names: []string{"/p_s.1.abc"}, Labels: map[string]string{LabelSwarmServiceName: "p_s", LabelComposeProject: "p", LabelComposeService: "s"}}
	for _, raw := range []string{"swarm:p_s", "compose:p:s"} {
		k, err := ParseTrackingKey(raw)
		if err != nil || !k.MatchContainer(&c) || k.String() != raw {
			t.Fatalf("%s: err=%v match=%v", raw, err, k.MatchContainer(&c))
		}
	}
	if k, _ := ParseTrackingKey("compose:p:s"); k.Source != KeySourceCompose || k.Value != "p:s" {
		t.Fatalf("parse cuts at the first colon: %+v", k)
	}
	k, _ := ParseTrackingKey("swarm:other")
	if k.MatchContainer(&c) {
		t.Fatal("wrong service matched")
	}
	k, err := ParseTrackingKey("compose:p")
	if err != nil {
		t.Fatal("a compose value without a service part is still a well-formed key (it just never matches)")
	}
	if k.MatchContainer(&c) {
		t.Fatal("compose:p must not match project p, service s")
	}
}

func TestTrackingKey_ReplicasShareKeyFirstMatchWins(t *testing.T) {
	r1 := ContainerSummary{ID: "a", Names: []string{"/s.1.aaa"}, Labels: map[string]string{LabelSwarmServiceName: "s", LabelComposeProject: "p", LabelComposeService: "svc"}}
	r2 := ContainerSummary{ID: "b", Names: []string{"/s.2.bbb"}, Labels: map[string]string{LabelSwarmServiceName: "s", LabelComposeProject: "p", LabelComposeService: "svc"}}
	set := []ContainerSummary{r1, r2}
	for _, raw := range []string{"swarm:s", "compose:p:svc"} {
		k, err := ParseTrackingKey(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := k.FindContainer(set); got == nil || got.ID != "a" {
			t.Fatalf("%s: want first replica, got %+v", raw, got)
		}
	}
}

func TestParseTrackingKey_ComposeEmptyParts(t *testing.T) {
	// These parse (non-empty value) but can never match a real
	// container, since composeKeyValue is "" unless both labels are set.
	c := ContainerSummary{ID: "1", Labels: map[string]string{LabelComposeProject: "p", LabelComposeService: "s"}}
	for _, raw := range []string{"compose:p:", "compose::s"} {
		k, err := ParseTrackingKey(raw)
		if err != nil {
			t.Fatalf("%s: unexpected error %v", raw, err)
		}
		if k.MatchContainer(&c) || k.FindContainer([]ContainerSummary{c}) != nil {
			t.Fatalf("%s must never match", raw)
		}
	}
	if _, err := ParseTrackingKey("compose:"); err == nil {
		t.Fatal("empty value must be rejected")
	}
}
