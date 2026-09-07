package config

import (
	"fmt"
	"strings"
)

// DirectiveValue holds one or more ordered values for a directive. A
// directive that appears once in a block has len(Values) == 1; a repeated
// directive (e.g. multiple IdentityFile lines) has one entry per
// occurrence, in file order.
type DirectiveValue struct {
	Values []string
}

// One builds a single-valued DirectiveValue.
func One(v string) DirectiveValue { return DirectiveValue{Values: []string{v}} }

// Many builds a multi-valued DirectiveValue, preserving the given order.
func Many(vs []string) DirectiveValue { return DirectiveValue{Values: vs} }

// First returns the first value, or "" if there are none — used where only
// one representative value makes sense (e.g. a reachability check), matching
// ssh's own first-occurrence-wins precedence for accidentally-repeated
// directives.
func (d DirectiveValue) First() string {
	if len(d.Values) == 0 {
		return ""
	}
	return d.Values[0]
}

// OrderedDirective is a (key, value) pair used when adding a host or
// setting directives on an existing one. A slice of these (rather than a
// map) keeps insertion order deterministic, which matters for a
// byte-reproducible file when several brand-new directives are added in one
// call.
type OrderedDirective struct {
	Key   string
	Value DirectiveValue
}

// ListAliases returns every Host block's alias, in file order, excluding the
// literal "*" wildcard block (which holds global defaults, not a
// user-addressable host).
func (c *Config) ListAliases() []string {
	var aliases []string
	for _, n := range c.Nodes {
		block, ok := n.(*HostBlock)
		if !ok {
			continue
		}
		if strings.TrimSpace(block.Alias) == "*" {
			continue
		}
		aliases = append(aliases, block.Alias)
	}
	return aliases
}

// findHost returns the HostBlock with an exact (case-sensitive) alias match,
// or nil if there is none.
func (c *Config) findHost(alias string) *HostBlock {
	for _, n := range c.Nodes {
		if block, ok := n.(*HostBlock); ok && block.Alias == alias {
			return block
		}
	}
	return nil
}

// HostParams returns every directive in the named host's block, keyed by
// lowercased directive name, in file order. ok is false if no block matches
// alias exactly.
func (c *Config) HostParams(alias string) (params map[string]DirectiveValue, ok bool) {
	block := c.findHost(alias)
	if block == nil {
		return nil, false
	}
	values := map[string][]string{}
	var keyOrder []string
	for _, bl := range block.Lines {
		d, ok := bl.(*Directive)
		if !ok {
			continue
		}
		lk := strings.ToLower(d.Key)
		if _, seen := values[lk]; !seen {
			keyOrder = append(keyOrder, lk)
		}
		values[lk] = append(values[lk], d.Value)
	}
	params = make(map[string]DirectiveValue, len(keyOrder))
	for _, lk := range keyOrder {
		params[lk] = DirectiveValue{Values: values[lk]}
	}
	return params, true
}

// AddHost appends a brand new Host block at the very end of the file (after
// any trailing "Host *" block), matching current behavior: this app never
// inserts before an existing block, so a Host * placed earlier in the file
// can still shadow a newly-added host under ssh's first-match-wins
// semantics — replicated here deliberately, not "fixed".
func (c *Config) AddHost(alias string, directives []OrderedDirective) error {
	if c.findHost(alias) != nil {
		return fmt.Errorf("host %q already exists", alias)
	}
	block := &HostBlock{
		HostLineRaw: "Host " + alias,
		Alias:       alias,
		Patterns:    strings.Fields(alias),
	}
	for _, d := range directives {
		for _, v := range d.Value.Values {
			block.Lines = append(block.Lines, &Directive{
				Key:   d.Key,
				Value: v,
				Raw:   c.Indent + d.Key + " " + v,
			})
		}
	}
	if len(c.Nodes) > 0 {
		c.Nodes = append(c.Nodes, RawLine{Text: ""})
	}
	c.Nodes = append(c.Nodes, block)
	return nil
}

// UpdateHost sets each directive in set to its given ordered value list and
// removes every directive whose key (case-insensitively) appears in
// removeKeys, on the named host's block. Every other line in the block —
// and every other block in the file — is left untouched.
func (c *Config) UpdateHost(alias string, set []OrderedDirective, removeKeys []string) error {
	block := c.findHost(alias)
	if block == nil {
		return fmt.Errorf("host %q not found", alias)
	}
	for _, d := range set {
		applySet(block, c.Indent, d.Key, d.Value.Values)
	}
	if len(removeKeys) > 0 {
		block.Lines = filterOutKeys(block.Lines, removeKeys)
	}
	return nil
}

// applySet overwrites block's existing Directive lines for key in place
// (position i -> values[i]), deletes any excess existing lines beyond
// len(values), and appends any extra values immediately after the last
// existing line for that key (or at block end if the key is brand new).
func applySet(block *HostBlock, indent, key string, values []string) {
	var matches []int
	for i, bl := range block.Lines {
		if d, ok := bl.(*Directive); ok && strings.EqualFold(d.Key, key) {
			matches = append(matches, i)
		}
	}
	overlap := len(matches)
	if len(values) < overlap {
		overlap = len(values)
	}
	for i := 0; i < overlap; i++ {
		d := block.Lines[matches[i]].(*Directive)
		lineIndent := leadingWhitespace(d.Raw)
		d.Value = values[i]
		d.Raw = lineIndent + d.Key + " " + values[i]
		if d.Comment != "" {
			d.Raw += " " + d.Comment
		}
	}

	switch {
	case len(matches) > len(values):
		// Delete the excess matched lines, highest index first so earlier
		// indices stay valid.
		toDelete := matches[len(values):]
		for i := len(toDelete) - 1; i >= 0; i-- {
			idx := toDelete[i]
			block.Lines = append(block.Lines[:idx], block.Lines[idx+1:]...)
		}
	case len(values) > len(matches):
		extra := values[len(matches):]
		newLines := make([]BlockLine, 0, len(extra))
		for _, v := range extra {
			newLines = append(newLines, &Directive{Key: key, Value: v, Raw: indent + key + " " + v})
		}
		if len(matches) > 0 {
			insertAt := matches[len(matches)-1] + 1
			tail := append([]BlockLine{}, block.Lines[insertAt:]...)
			block.Lines = append(block.Lines[:insertAt], append(newLines, tail...)...)
		} else {
			block.Lines = append(block.Lines, newLines...)
		}
	}
}

// filterOutKeys returns lines with every Directive whose key
// case-insensitively matches one of keys removed; all other lines
// (including comments/blanks) pass through unchanged.
func filterOutKeys(lines []BlockLine, keys []string) []BlockLine {
	out := make([]BlockLine, 0, len(lines))
	for _, bl := range lines {
		if d, ok := bl.(*Directive); ok {
			removed := false
			for _, k := range keys {
				if strings.EqualFold(d.Key, k) {
					removed = true
					break
				}
			}
			if removed {
				continue
			}
		}
		out = append(out, bl)
	}
	return out
}

// RemoveHost deletes the named host's entire block. If doing so leaves two
// adjacent blank lines where the block used to be, one is collapsed away.
// Comments are never auto-deleted, even ones that look orphaned afterward —
// losing a user's comment is worse than an occasional extra blank line.
func (c *Config) RemoveHost(alias string) error {
	idx := -1
	for i, n := range c.Nodes {
		if block, ok := n.(*HostBlock); ok && block.Alias == alias {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("host %q not found", alias)
	}
	c.Nodes = append(c.Nodes[:idx], c.Nodes[idx+1:]...)

	if idx > 0 && idx < len(c.Nodes) {
		before, isBlankBefore := c.Nodes[idx-1].(RawLine)
		after, isBlankAfter := c.Nodes[idx].(RawLine)
		if isBlankBefore && isBlankAfter && strings.TrimSpace(before.Text) == "" && strings.TrimSpace(after.Text) == "" {
			c.Nodes = append(c.Nodes[:idx], c.Nodes[idx+1:]...)
		}
	}
	return nil
}

// RenameHost changes the alias of an existing host in place: only the
// argument portion of its "Host ..." line changes (any trailing comment on
// that line is preserved verbatim); nothing else about the block's content
// or position moves.
func (c *Config) RenameHost(oldAlias, newAlias string) error {
	block := c.findHost(oldAlias)
	if block == nil {
		return fmt.Errorf("host %q not found", oldAlias)
	}
	if newAlias != oldAlias && c.findHost(newAlias) != nil {
		return fmt.Errorf("host %q already exists", newAlias)
	}
	indent := leadingWhitespace(block.HostLineRaw)
	block.HostLineRaw = indent + "Host " + newAlias
	block.Alias = newAlias
	block.Patterns = strings.Fields(newAlias)
	return nil
}
