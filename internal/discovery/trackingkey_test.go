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
