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
type envRef struct {
	path     []string
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
	var walk func(n *yaml.Node, path []string)
	walk = func(n *yaml.Node, path []string) {
		switch n.Kind {
		case yaml.DocumentNode:
			for _, c := range n.Content {
				walk(c, path)
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				walk(n.Content[i+1], append(append([]string(nil), path...), n.Content[i].Value))
			}
		case yaml.SequenceNode:
			for i, c := range n.Content {
				walk(c, append(append([]string(nil), path...), seqKey(c, i, true)))
			}
		case yaml.ScalarNode:
			if strings.Contains(n.Value, "${") {
				expanded, _ := expandBracedEnv(n.Value)
				refs = append(refs, envRef{path: path, raw: n.Value, expanded: expanded})
			}
		}
	}
	walk(&doc, nil)
	return refs, nil
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
	return "[" + strconv.Itoa(i) + "]"
}

// findPath returns the scalar node at path under root, or nil.
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
				if seqKey(c, i, false) == seg {
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
	for i, r := range c.envRefs {
		if n := findPath(&root, r.path); n != nil && n.Value == r.expanded {
			nodes[i] = n
		}
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
