package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Color palette. Named after the roles they play (matching app.css's
// $primary/$secondary/$panel/$error theme-variable scheme in the Python
// original) rather than the raw ANSI codes themselves, so the whole app's
// look can be re-tuned from one place.
var (
	colorPrimary   = lipgloss.Color("6")   // cyan — dialog borders, titles, banner
	colorSecondary = lipgloss.Color("5")   // magenta — nested sub-panels
	colorError     = lipgloss.Color("1")   // red — validation errors, destructive confirm
	colorUp        = lipgloss.Color("2")   // green — reachable
	colorDown      = lipgloss.Color("1")   // red — unreachable
	colorUnknown   = lipgloss.Color("8")   // grey — checking / not yet known
	colorGroup     = lipgloss.Color("15")  // bright white — group labels (kept high-contrast; magenta read as too dim)
	colorTreeGuide = lipgloss.Color("8")   // grey — the ├──/└──/│ guide-line characters themselves
	colorMuted     = lipgloss.Color("245") // dim grey — de-emphasized secondary text (detail columns, hint descriptions)
	colorBarBg     = lipgloss.Color("236") // solid dark background for the top banner/column-header/footer bars
)

// Every style below is populated by InitStyles, not by var initializers —
// see InitStyles' doc comment for why that distinction actually matters
// here (it's not just tidiness).
var (
	upStyle        lipgloss.Style
	downStyle      lipgloss.Style
	unknownStyle   lipgloss.Style
	bannerStyle    lipgloss.Style
	cursorStyle    lipgloss.Style
	groupStyle     lipgloss.Style
	treeGuideStyle lipgloss.Style
	errorStyle     lipgloss.Style

	// Inline keybinding hints (e.g. a dialog's "tab move · esc cancel"
	// footer line): the key itself picked out in the same accent color used
	// for the bottom bar's keys, description in a dimmer neutral tone.
	hintKeyStyle  lipgloss.Style
	hintDescStyle lipgloss.Style

	// secondaryColumnStyle dims the Hostname/Port/User/Extra columns in the
	// host list so Alias reads as the visually primary field per row.
	secondaryColumnStyle lipgloss.Style

	// dialogStyle boxes a modal screen's content: rounded primary-colored
	// border, padded — matches Python's shared dialog look (HostEditScreen/
	// ConfirmScreen/ScpScreen/FilePickerScreen all get this same treatment).
	dialogStyle lipgloss.Style

	// nestedPanelStyle is the secondary-colored sub-panel used inside a
	// dialog for a scrollable/nested list — the "Other parameters" rows in
	// the edit form, or the directory listing in the file picker.
	nestedPanelStyle lipgloss.Style

	// Footer bar: a full-width strip with one uniform background — the key
	// itself is picked out with color/bold rather than a solid block, so
	// the whole bar reads as one continuous strip instead of colored chips
	// poking out of it.
	footerBarStyle  lipgloss.Style
	footerKeyStyle  lipgloss.Style
	footerDescStyle lipgloss.Style

	// Top banner bar (app name/version + host stats) and the column-header
	// bar (Alias/Hostname/Port/User/Extra): same solid, non-transparent
	// background as the footer, so all three chrome bars read as one
	// consistent treatment. Individual pieces need the background baked
	// into each style (not just wrapped afterward), since nested lipgloss
	// Render() calls each emit their own reset codes.
	barTextStyle       lipgloss.Style
	bannerTitleStyle   lipgloss.Style
	bannerUpStyle      lipgloss.Style
	bannerDownStyle    lipgloss.Style
	bannerUnknownStyle lipgloss.Style
	headerBarStyle     lipgloss.Style

	// searchBarStyle boxes the search input in a full rounded, primary-
	// colored border — matching the Python original's bordered search box.
	searchBarStyle lipgloss.Style
)

// InitStyles (re)builds every style in this package from lipgloss's current
// default renderer. Call it once, after any call to
// lipgloss.SetDefaultRenderer, before the first View().
//
// This can't be done with ordinary "var x = lipgloss.NewStyle()..."
// initializers: lipgloss.Style captures a reference to whatever the default
// renderer *is at construction time*, and Go initializes package-level vars
// before main() runs at all — before main() ever gets a chance to point the
// renderer at the real controlling terminal instead of (possibly redirected)
// os.Stdout. cmd/sshtree/main.go does exactly that when it opens /dev/tty
// (needed so --print-only's `$(sshtree --print-only)` capture doesn't also
// swallow the rendered frames) — without this function, every style here
// would have permanently bound itself to a renderer that saw a non-tty
// os.Stdout and concluded there was no color support, rendering everything
// in plain text with no way to distinguish the cursor-highlighted row (that
// highlight is a color/background style too).
func InitStyles() {
	upStyle = lipgloss.NewStyle().Foreground(colorUp)
	downStyle = lipgloss.NewStyle().Foreground(colorDown)
	unknownStyle = lipgloss.NewStyle().Foreground(colorUnknown)
	bannerStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(colorPrimary)
	groupStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGroup)
	treeGuideStyle = lipgloss.NewStyle().Foreground(colorTreeGuide)
	errorStyle = lipgloss.NewStyle().Foreground(colorError)

	hintKeyStyle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)
	hintDescStyle = lipgloss.NewStyle().Foreground(colorMuted)

	secondaryColumnStyle = lipgloss.NewStyle().Foreground(colorMuted)

	dialogStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(1, 2)

	nestedPanelStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorSecondary).
		Padding(0, 1)

	footerBarStyle = lipgloss.NewStyle().Background(colorBarBg)
	footerKeyStyle = footerBarStyle.Foreground(colorPrimary).Bold(true)
	footerDescStyle = footerBarStyle.Foreground(lipgloss.Color("15"))

	barTextStyle = lipgloss.NewStyle().Background(colorBarBg).Foreground(lipgloss.Color("15"))
	bannerTitleStyle = barTextStyle.Bold(true).Foreground(colorPrimary)
	bannerUpStyle = barTextStyle.Foreground(colorUp)
	bannerDownStyle = barTextStyle.Foreground(colorDown)
	bannerUnknownStyle = barTextStyle.Foreground(colorUnknown)
	headerBarStyle = barTextStyle.Bold(true)

	searchBarStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorPrimary).
		Padding(0, 1)
}

// renderKeyHints joins alternating key/description pairs into one inline
// hint line — e.g. renderKeyHints("esc", "cancel", "enter", "save") — with
// each key colored the same as the bottom bar's keys, for a dialog's own
// "tab move · esc cancel"-style footer text.
func renderKeyHints(pairs ...string) string {
	var parts []string
	for i := 0; i+1 < len(pairs); i += 2 {
		parts = append(parts, hintKeyStyle.Render(pairs[i])+" "+hintDescStyle.Render(pairs[i+1]))
	}
	return strings.Join(parts, hintDescStyle.Render(" · "))
}

// fillBar renders content and pads the remainder of the row with barTextStyle's
// background so it reads as one continuous solid bar rather than styled text
// sitting on the terminal's default (transparent) background.
func fillBar(width int, content string) string {
	if width > 0 {
		if pad := width - lipgloss.Width(content); pad > 0 {
			content += barTextStyle.Render(strings.Repeat(" ", pad))
		}
	}
	return content
}

// footerBinding is one entry in the footer's keybinding bar.
type footerBinding struct{ key, desc string }

// renderFooter lays out bindings as highlighted-key/description chips
// across one or more full-width bars, wrapping to additional lines rather
// than truncating when they don't all fit in one row — a fixed-width cutoff
// would otherwise silently hide bindings (including quit) in a narrower
// terminal. Each line's remainder is padded with the bar's own background
// so it reads as a continuous strip rather than styled text floating on the
// default background.
func renderFooter(width int, bindings []footerBinding) string {
	chips := make([]string, len(bindings))
	chipWidths := make([]int, len(bindings))
	for i, fb := range bindings {
		chips[i] = footerKeyStyle.Render(" "+fb.key+" ") + footerDescStyle.Render(" "+fb.desc+" ")
		chipWidths[i] = lipgloss.Width(chips[i])
	}

	if width <= 0 {
		return strings.Join(chips, "")
	}

	var lines []string
	var cur strings.Builder
	curWidth := 0
	flush := func() {
		if curWidth == 0 {
			return
		}
		line := cur.String()
		if pad := width - curWidth; pad > 0 {
			line += footerBarStyle.Render(strings.Repeat(" ", pad))
		}
		lines = append(lines, line)
		cur.Reset()
		curWidth = 0
	}
	for i, chip := range chips {
		if curWidth > 0 && curWidth+chipWidths[i] > width {
			flush()
		}
		cur.WriteString(chip)
		curWidth += chipWidths[i]
	}
	flush()

	return strings.Join(lines, "\n")
}

// dialogWidth sizes a dialog to roughly 70% of the terminal width, capped
// at 80 columns and floored at 40 — matching app.css's `width: 70%;
// max-width: 80` for the standard dialogs.
func dialogWidth(termWidth int) int {
	w := termWidth * 7 / 10
	if w > 80 {
		w = 80
	}
	if w < 40 {
		w = 40
	}
	return w
}

// renderDialog boxes content in the shared dialog style and centers it in
// the terminal. Falls back to an unplaced (unrolled) box if the terminal
// size isn't known yet (e.g. the very first frame, before the initial
// WindowSizeMsg arrives).
func renderDialog(width, height int, content string) string {
	box := dialogStyle.Width(dialogWidth(width)).Render(content)
	if width <= 0 || height <= 0 {
		return box
	}
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
