package discovery

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"unicode"

	"github.com/mescon/muximux/v3/internal/config"
)

// Icon types a muximux.app.icon / muximux.group.icon label can select. They
// are the AppIconConfig types of the same name; custom uploads cannot be
// named by a label.
const (
	iconTypeDashboard = "dashboard"
	iconTypeLucide    = "lucide"
	iconTypeURL       = "url"
)

// maxIconURLLen caps a label icon URL. Browsers and proxies reject much
// longer URLs, and it keeps a stray paste out of config.yaml.
const maxIconURLLen = 2048

// lucideNameRe is the shape of a Lucide icon identifier: lowercase letters,
// digits and dashes, starting with a letter or digit. The set of real names
// is fetched over the network by the icons handler, so discovery checks the
// shape only; an unknown name renders as a missing icon, like in the UI.
var lucideNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// IconRef is a parsed icon label. The zero value means "no icon set".
type IconRef struct {
	Type string // dashboard, lucide or url; "" = unset
	Name string // dashboard slug or lucide name
	URL  string // absolute http(s) URL, kept exactly as written
}

// IsSet reports whether the label named an icon.
func (r IconRef) IsSet() bool { return r.Type != "" }

// Value is the icon's name or URL, whichever its type uses.
func (r IconRef) Value() string {
	if r.Type == iconTypeURL {
		return r.URL
	}
	return r.Name
}

// Matches reports whether ic already shows this icon.
func (r IconRef) Matches(ic *config.AppIconConfig) bool {
	if ic.Type != r.Type {
		return false
	}
	if r.Type == iconTypeURL {
		return ic.URL == r.URL
	}
	return ic.Name == r.Name
}

// Apply stores the icon on ic. Styling the operator set on the icon
// (variant, colour, background, invert) is kept; the fields of other icon
// types are cleared.
func (r IconRef) Apply(ic *config.AppIconConfig) {
	ic.Type = r.Type
	ic.Name, ic.File, ic.URL = "", "", ""
	if r.Type == iconTypeURL {
		ic.URL = r.URL
	} else {
		ic.Name = r.Name
	}
}

// Config returns the icon as an AppIconConfig.
func (r IconRef) Config() config.AppIconConfig {
	var ic config.AppIconConfig
	r.Apply(&ic)
	return ic
}

// iconRefOf builds an IconRef from a Suggestion's icon fields; an empty
// type means a dashboard icon, as before icon types existed.
func iconRefOf(value, typ string) IconRef {
	switch typ {
	case iconTypeURL:
		return IconRef{Type: iconTypeURL, URL: value}
	case iconTypeLucide:
		return IconRef{Type: iconTypeLucide, Name: value}
	}
	return IconRef{Type: iconTypeDashboard, Name: value}
}

// ParseIconLabel parses the value of muximux.app.icon or muximux.group.icon.
// Both labels share it, so they accept the same forms:
//
//	https://example.com/i.svg   a URL icon (http or https)
//	url:https://example.com/i   a URL icon, explicit
//	lucide:server               a Lucide icon
//	dashboard:sonarr            a Dashboard Icons slug, explicit
//	sonarr                      a Dashboard Icons slug (the original form)
//
// Prefixes are case-insensitive. Slugs and Lucide names are lowercased; URLs
// are kept as written. A bare value is never read as a Lucide name, since
// names such as "server" exist in both sets. A blank value is unset with no
// error; an unusable one returns the zero IconRef and a reason.
func ParseIconLabel(raw string) (IconRef, string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return IconRef{}, ""
	}
	lower := strings.ToLower(v)
	switch {
	case strings.HasPrefix(lower, "http://"), strings.HasPrefix(lower, "https://"):
		return parseIconURL(v)
	case strings.HasPrefix(lower, "url:"):
		return parseIconURL(strings.TrimSpace(v[len("url:"):]))
	case strings.HasPrefix(lower, "lucide:"):
		name := strings.TrimSpace(lower[len("lucide:"):])
		if !lucideNameRe.MatchString(name) {
			return IconRef{}, fmt.Sprintf("%q is not a Lucide icon name (lowercase letters, digits and dashes, such as shield-check)", v)
		}
		return IconRef{Type: iconTypeLucide, Name: name}, ""
	case strings.HasPrefix(lower, "dashboard:"):
		return parseDashboardSlug(strings.TrimSpace(lower[len("dashboard:"):]), v)
	}
	return parseDashboardSlug(lower, v)
}

// parseDashboardSlug accepts a Dashboard Icons slug. Anything that cannot be
// part of the icon's file name, such as an unknown scheme like "data:", is
// refused rather than stored as a name.
func parseDashboardSlug(slug, orig string) (IconRef, string) {
	if slug == "" || strings.ContainsAny(slug, ":/\\?#") || strings.IndexFunc(slug, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}) >= 0 {
		return IconRef{}, fmt.Sprintf("%q is not a Dashboard Icons name; use dashboard:<name>, lucide:<name> or an http(s) URL", orig)
	}
	return IconRef{Type: iconTypeDashboard, Name: slug}, ""
}

// parseIconURL accepts an absolute http or https URL with a host, no
// credentials and no whitespace, up to maxIconURLLen. Other schemes
// (javascript:, data:, file:) are refused: the browser loads a URL icon
// straight from the address, so the scheme is the safety check.
func parseIconURL(v string) (IconRef, string) {
	bad := func(why string) (IconRef, string) {
		return IconRef{}, fmt.Sprintf("icon URL %q %s", v, why)
	}
	if len(v) > maxIconURLLen {
		return bad(fmt.Sprintf("is longer than %d characters", maxIconURLLen))
	}
	if strings.IndexFunc(v, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return bad("contains whitespace or control characters")
	}
	u, err := url.Parse(v)
	if err != nil || u.Hostname() == "" {
		return bad("is not an absolute http(s) URL")
	}
	if s := strings.ToLower(u.Scheme); s != "http" && s != "https" {
		return bad("must use http or https")
	}
	if u.User != nil {
		return bad("must not contain credentials")
	}
	return IconRef{Type: iconTypeURL, URL: v}, ""
}
