package config

import (
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// envRef remembers a scalar that was written as a ${VAR} reference in the
// file, so Save can write the reference back instead of the secret it
// expanded to. path is a list of mapping keys; sequence items are
// "[name=X]" when the item has a name/username/domain key, else "[i]".
// posPath is the same location with every sequence item named by its
// index, used when the named path no longer resolves (the item was
// renamed) or resolves to a different item (duplicate names).
type envRef struct {
	path     []string
	posPath  []string
	raw      string
	expanded string
}

var wholeEnvRef = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

// recordEnvRefs walks the raw, unexpanded YAML and records every scalar
// containing a ${...} reference.
func recordEnvRefs(raw []byte) ([]envRef, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, err
	}
	var refs []envRef
	var walk func(n *yaml.Node, path, posPath []string)
	walk = func(n *yaml.Node, path, posPath []string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, path, posPath)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				k := n.Content[i].Value
				walk(n.Content[i+1], appendSeg(path, k), appendSeg(posPath, k))
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				walk(c, appendSeg(path, seqKey(c, i, true)), appendSeg(posPath, indexSeg(i)))
			}
		case yaml.ScalarNode:
			if strings.Contains(n.Value, "${") {
				expanded, _ := expandBracedEnv(n.Value)
				refs = append(refs, envRef{path: path, posPath: posPath, raw: n.Value, expanded: expanded})
			}
		}
	}
	walk(&doc, nil, nil)
	return refs, nil
}

// appendSeg returns a copy of path with seg appended.
func appendSeg(path []string, seg string) []string {
	return append(append([]string(nil), path...), seg)
}

// indexSeg is the positional path segment for sequence item i.
func indexSeg(i int) string {
	return "[" + strconv.Itoa(i) + "]"
}

// seqKey names a sequence item by its identifying field so a reordered
// list still matches.
func seqKey(item *yaml.Node, i int, expand bool) string {
	if item.Kind == yaml.MappingNode {
		for j := 0; j+1 < len(item.Content); j += 2 {
			switch item.Content[j].Value {
			case "name", "username", "domain":
				v := item.Content[j+1].Value
				if expand {
					// The encoded node at save time holds expanded values.
					v, _ = expandBracedEnv(v)
				}
				return "[" + item.Content[j].Value + "=" + v + "]"
			}
		}
	}
	return indexSeg(i)
}

// findPath returns the scalar node at path under root, or nil. A
// sequence segment matches an item by its seqKey, or by position when the
// segment is a positional "[i]".
func findPath(root *yaml.Node, path []string) *yaml.Node {
	n := root
	if n.Kind == yaml.DocumentNode && len(n.Content) == 1 {
		n = n.Content[0]
	}
	for _, seg := range path {
		switch n.Kind {
		case yaml.MappingNode:
			var next *yaml.Node
			for i := 0; i+1 < len(n.Content); i += 2 {
				if n.Content[i].Value == seg {
					next = n.Content[i+1]
					break
				}
			}
			if next == nil {
				return nil
			}
			n = next
		case yaml.SequenceNode:
			var next *yaml.Node
			for i, c := range n.Content {
				if seqKey(c, i, false) == seg || indexSeg(i) == seg {
					next = c
					break
				}
			}
			if next == nil {
				return nil
			}
			n = next
		default:
			return nil
		}
	}
	if n.Kind != yaml.ScalarNode {
		return nil
	}
	return n
}

// marshalWithEnvRefs marshals c and writes recorded ${VAR} references
// back wherever the value is still what the reference expanded to.
func (c *Config) marshalWithEnvRefs() ([]byte, error) {
	if len(c.envRefs) == 0 {
		return yaml.Marshal(c)
	}
	var root yaml.Node
	if err := root.Encode(c); err != nil {
		return nil, err
	}
	// Resolve every node before rewriting any, since restoring a reference
	// in an item's name would break the name-based matching of its siblings.
	nodes := make([]*yaml.Node, len(c.envRefs))
	for i := range c.envRefs {
		nodes[i] = c.envRefs[i].resolve(&root)
	}
	for i, n := range nodes {
		if n != nil {
			n.Value = c.envRefs[i].raw
			n.Style = 0
			n.Tag = ""
		}
	}
	return yaml.Marshal(&root)
}

// resolve finds the node a recorded reference should be written back to:
// the item named in path first, then the item at the same position (a
// renamed item, or the second of two items sharing a name). Either way the
// reference is restored only over a value identical to what it expanded
// to, so the fallback cannot attach a reference to a different secret.
func (r *envRef) resolve(root *yaml.Node) *yaml.Node {
	if n := findPath(root, r.path); n != nil && n.Value == r.expanded {
		return n
	}
	if n := findPath(root, r.posPath); n != nil && n.Value == r.expanded {
		return n
	}
	return nil
}

// EnvRefVar reports the variable a field's whole value comes from, for a
// field written as exactly "${VAR}" in the loaded file.
func (c *Config) EnvRefVar(path ...string) (string, bool) {
	for _, r := range c.envRefs {
		if strings.Join(r.path, "\x00") != strings.Join(path, "\x00") {
			continue
		}
		if m := wholeEnvRef.FindStringSubmatch(r.raw); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// OIDCClientSecretInPlaintext reports whether OIDC is enabled with a client
// secret written literally in the loaded file. A secret written as exactly
// "${VAR}" is recorded as a reference at load (the field itself already
// holds the expanded value), and an unresolved "${VAR}" is left as-is, so
// neither counts as plaintext.
func (c *Config) OIDCClientSecretInPlaintext() bool {
	o := &c.Auth.OIDC
	if !o.Enabled || o.ClientSecret == "" {
		return false
	}
	if _, ok := c.EnvRefVar("auth", "oidc", "client_secret"); ok {
		return false
	}
	return !IsBracedEnvRef(o.ClientSecret)
}
