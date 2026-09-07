package ui

import (
	"fmt"
	"strings"
)

// portColWidth is a fixed width for the Port column (mirrors host_list.py's
// rjust(5) treatment of port, rather than growing with content like the
// other columns).
const portColWidth = 5

// leafDisplay holds the precomputed display fields for one host row.
type leafDisplay struct {
	Alias    string
	Hostname string
	User     string
	Port     string
	HasExtra bool
	Status   string // "unknown" | "up" | "down"
}

type colWidths struct {
	Alias, Hostname, User int
}

// computeColWidths sizes each flexible column to the longer of its header
// label or any value present, matching _compute_col_widths.
func computeColWidths(leaves []leafDisplay) colWidths {
	w := colWidths{Alias: len("Alias"), Hostname: len("Hostname"), User: len("User")}
	for _, l := range leaves {
		if n := len(l.Alias); n > w.Alias {
			w.Alias = n
		}
		if n := len(l.Hostname); n > w.Hostname {
			w.Hostname = n
		}
		if n := len(l.User); n > w.User {
			w.User = n
		}
	}
	return w
}

// treeLeafIndent is the assumed prefix width (guide lines + status icon +
// space) before a typical one-level-deep leaf's Alias column starts, used
// to roughly line up the column header above it. Mirrors host_list.py's own
// _TREE_LEAF_INDENT = 10 empirical constant and its documented limitation:
// this is only exact for a leaf nested exactly one level deep (the common
// case), not for every possible nesting depth.
const treeLeafIndent = 10

func columnHeaderLine(w colWidths) string {
	return strings.Repeat(" ", treeLeafIndent) + fmt.Sprintf("%-*s  %-*s  %*s  %-*s  Extra",
		w.Alias, "Alias", w.Hostname, "Hostname", portColWidth, "Port", w.User, "User")
}

// columnAliasField renders just the (padded) Alias column, unstyled — the
// visually primary field per row.
func columnAliasField(l leafDisplay, w colWidths) string {
	return fmt.Sprintf("%-*s", w.Alias, sanitizeForDisplay(l.Alias))
}

// columnSecondaryFields renders the Hostname/Port/User/Extra columns
// (padded), unstyled — the "detail" fields after Alias, which the caller
// dims via secondaryColumnStyle in the normal (non-cursor) row-rendering
// path. All config-derived values are sanitized first: they originate from
// ~/.ssh/config, which could be shared/untrusted, and could otherwise
// contain literal ANSI escape sequences that corrupt terminal rendering.
func columnSecondaryFields(l leafDisplay, w colWidths) string {
	extra := "No"
	if l.HasExtra {
		extra = "Yes"
	}
	return fmt.Sprintf("  %-*s  %*s  %-*s  %s",
		w.Hostname, sanitizeForDisplay(l.Hostname),
		portColWidth, sanitizeForDisplay(l.Port),
		w.User, sanitizeForDisplay(l.User),
		extra)
}

// rowColumns renders all columns as one PLAIN (unstyled) string. Used only
// where the caller applies a single uniform style to the whole row
// afterward — the cursor-highlighted row — since embedding per-field ANSI
// codes here would break that outer styling (nested lipgloss Render() calls
// each emit their own reset; see model.go's row-rendering loop for the
// full explanation). Elsewhere, use columnAliasField/columnSecondaryFields
// directly so Alias and the detail columns can be styled differently.
func rowColumns(l leafDisplay, w colWidths) string {
	return columnAliasField(l, w) + columnSecondaryFields(l, w)
}

func statusIcon(status string) string {
	if status == "up" || status == "down" {
		return "●"
	}
	return "○"
}

// sanitizeForDisplay strips C0 control characters and DEL from a
// config-derived value before it's ever written to the terminal, so a
// hostname/user/etc. containing a literal ESC (or similar) can't inject
// terminal escape sequences.
func sanitizeForDisplay(s string) string {
	var b []rune
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b = append(b, r)
	}
	return string(b)
}
