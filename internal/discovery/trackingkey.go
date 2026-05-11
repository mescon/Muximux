package discovery

import (
	"errors"
	"strings"
)

// TrackingKey identifies a docker container across restarts using the
// "<source>:<value>" prefix vocabulary the discovery feature speaks.
// Three sources, in stability order:
//
//   - label (KeySourceLabel) - reads muximux.discovery.id from the
//     container labels. Set explicitly by the operator and survives
//     docker-compose --force-recreate or swarm reschedule.
//   - name (KeySourceName) - matches the container name. Reliable
//     for hand-managed containers, fragile under compose's numeric
//     suffix on recreate.
//   - id (KeySourceID) - matches the full container ID OR any prefix.
//     Last-resort identifier when neither label nor name is usable;
//     guaranteed to change on every recreate so the tracked entry
//     will need re-linking after most container lifecycle events.
//
// The poller's refresh loop and the re-link probe handler both need
// to resolve tracked entries against the live daemon. Before this
// type existed, each had its own near-identical
// strings.Cut + switch/case copy - a recipe for the two to drift
// when a new key source gets added. The type is the single source
// of truth.
type TrackingKey struct {
	Source KeySource
	Value  string
}

// KeySource enumerates the three valid tracking-key prefixes. Defined
// type so a typo in a comparison or switch case is a compile error.
type KeySource string

const (
	KeySourceLabel KeySource = "label"
	KeySourceName  KeySource = "name"
	KeySourceID    KeySource = "id"
)

// errMalformedTrackingKey is the sentinel returned by ParseTrackingKey
// when the input doesn't contain the source:value separator.
var errMalformedTrackingKey = errors.New("malformed tracking key (no source prefix)")

// ParseTrackingKey splits a "<source>:<value>" string into a typed
// TrackingKey. Rejects:
//
//   - missing colon ("foo")
//   - empty source (":foo")
//   - empty value ("label:")
//   - unknown source ("magic:foo")
//
// The wire format on disk and over the API is unchanged; this is the
// boundary check that lets the rest of the codebase work with a
// well-formed value.
func ParseTrackingKey(raw string) (TrackingKey, error) {
	source, value, ok := strings.Cut(raw, ":")
	if !ok {
		return TrackingKey{}, errMalformedTrackingKey
	}
	if source == "" || value == "" {
		return TrackingKey{}, errMalformedTrackingKey
	}
	src := KeySource(source)
	if src != KeySourceLabel && src != KeySourceName && src != KeySourceID {
		return TrackingKey{}, errMalformedTrackingKey
	}
	return TrackingKey{Source: src, Value: value}, nil
}

// String renders the key back to its wire shape ("<source>:<value>").
// Used by the audit log and the LastSeenAt map key.
func (k TrackingKey) String() string {
	return string(k.Source) + ":" + k.Value
}

// MatchContainer returns true when the given container's identity
// satisfies this tracking key. Resolution priority follows
// KeyForContainer's stability order, but at match-time the rules
// per source are:
//
//   - label: container has the muximux.discovery.id label with this
//     exact value
//   - name:  container's primary name (without leading "/") equals
//     this exact value
//   - id:    container ID equals this value OR begins with it
//     (operator-friendly: pasting a 12-char prefix matches)
func (k TrackingKey) MatchContainer(c *ContainerSummary) bool {
	switch k.Source {
	case KeySourceLabel:
		return c.Labels[LabelDiscoveryID] == k.Value
	case KeySourceName:
		return c.PrimaryName() == k.Value
	case KeySourceID:
		return c.ID == k.Value || strings.HasPrefix(c.ID, k.Value)
	}
	return false
}

// FindContainer scans the slice for the first container that matches
// this tracking key. Returns nil when no match is found.
func (k TrackingKey) FindContainer(containers []ContainerSummary) *ContainerSummary {
	for i := range containers {
		if k.MatchContainer(&containers[i]) {
			return &containers[i]
		}
	}
	return nil
}
