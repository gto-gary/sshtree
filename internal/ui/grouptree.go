package ui

import "strings"

// GroupNode is one node in the alias-prefix nesting hierarchy: hosts nest by
// "--" in their alias (srv--nas--x -> group srv > subgroup nas > host x).
type GroupNode struct {
	Children map[string]*GroupNode // subgroup name -> subgroup
	Leaves   map[string]string     // display name -> full alias
}

func newGroupNode() *GroupNode {
	return &GroupNode{Children: map[string]*GroupNode{}, Leaves: map[string]string{}}
}

// AliasSegments splits an alias into hierarchical group segments on "--",
// with the last segment being the leaf's own display name:
// "srv--nas--trunas" -> ["srv", "nas", "trunas"] (nests two levels deep).
// Empty parts from a leading/trailing/doubled "--" are dropped (so
// "srv----nas" normalizes to ["srv", "nas"], same as "srv--nas"). An alias
// with no "--" at all, or where dropping empties leaves fewer than 2 parts
// (e.g. "srv--"), falls into a single "other" group with the literal alias
// as its display name rather than showing a blank-looking row.
func AliasSegments(alias string) []string {
	var parts []string
	for _, p := range strings.Split(alias, "--") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) < 2 {
		return []string{"other", alias}
	}
	return parts
}

// BuildGroupTree builds the alias-prefix nesting hierarchy from a flat list
// of aliases, rebuilt fresh from scratch each call (not cached/diffed).
func BuildGroupTree(aliases []string) *GroupNode {
	root := newGroupNode()
	for _, alias := range aliases {
		segments := AliasSegments(alias)
		node := root
		for _, seg := range segments[:len(segments)-1] {
			child, ok := node.Children[seg]
			if !ok {
				child = newGroupNode()
				node.Children[seg] = child
			}
			node = child
		}
		node.Leaves[segments[len(segments)-1]] = alias
	}
	return root
}
