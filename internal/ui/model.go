// Package ui implements the Bubble Tea TUI: the host-list screen (grouped
// tree, search/filter, reachability status, add/edit/clone/delete, scp) and
// the child screens it pushes. Choosing a connect/sftp/scp action quits the
// program; cmd/sshtui reads it back via ChosenAction and does the actual
// exec.
package ui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gto-gary/sshtui/internal/actions"
	"github.com/gto-gary/sshtui/internal/config"
	"github.com/gto-gary/sshtui/internal/history"
	"github.com/gto-gary/sshtui/internal/reachability"
)

type rowKind int

const (
	rowGroup rowKind = iota
	rowLeaf
)

// row is one flattened, currently-visible line in the tree: either a group
// header or a host leaf.
type row struct {
	kind  rowKind
	depth int
	label string // group name, or leaf's display name (last alias segment)
	path  string // full group path — expand-state key (groups only)
	alias string // full alias (leaves only)
	leaf  leafDisplay

	// isLast and ancestorContinues drive the tree guide-line prefix
	// (├── / └── / │): isLast is whether this row is the last child among
	// its immediate siblings; ancestorContinues has one entry per ancestor
	// level (0..depth-1), true if that ancestor still has more siblings
	// below it (so its vertical guide line must keep drawing past this row).
	isLast            bool
	ancestorContinues []bool
}

// Model is the host-list screen's Bubble Tea model.
type Model struct {
	cfg        *config.Config
	configPath string
	backedUp   bool // config.py's "backup once per run before the first write" guard

	filterInput   textinput.Model
	searchFocused bool

	expanded map[string]bool   // group path -> expanded (missing = default expanded)
	status   map[string]string // alias -> "unknown" | "up" | "down"

	rows       []row
	cursor     int
	rowScroll  int // index of the first visible row, kept following cursor in View()
	rowsStartY int // terminal Y (0-indexed) where the row list starts, cached from View() for mouse-click hit-testing

	lastClickIdx  int // row index of the most recent left-click, for double-click detection
	lastClickTime time.Time
	colWidths     colWidths

	reachChan       <-chan reachability.Result
	reachGeneration int
	reachCancel     context.CancelFunc

	// hostEdit/confirm are the two child "screens" that can be pushed over
	// the tree — nil when not shown. Bubble Tea has no direct equivalent of
	// Textual's awaitable push_screen_wait, so a child screen "returns" by
	// emitting its own *DoneMsg, which Update catches and forwards into the
	// mutation logic below, then clears the field back to nil.
	hostEdit           *hostEditModel
	confirm            *confirmModel
	scp                *scpModel
	pendingDeleteAlias string

	statusMessage string

	history      *history.Store
	chosenAction actions.Action // set + tea.Quit'd when the user picks connect/sftp/scp; nil if they just quit

	width, height int
}

// New builds the host-list model for cfg, which is saved back to
// configPath on every successful add/edit/clone/delete.
func New(cfg *config.Config, configPath string) *Model {
	ti := textinput.New()
	ti.Placeholder = "search, or extra:yes / status:down"
	ti.Prompt = "/ "

	historyStore, _ := history.DefaultStore() // best-effort; a broken history file must never block connecting

	m := &Model{
		cfg:         cfg,
		configPath:  configPath,
		filterInput: ti,
		expanded:    map[string]bool{},
		status:      map[string]string{},
		history:     historyStore,
	}
	m.rebuildRows()
	return m
}

// ChosenAction returns what the user picked (connect/record/sftp/scp)
// before quitting, or nil if they quit without choosing anything. Read this
// after tea.Program.Run() returns.
func (m *Model) ChosenAction() actions.Action {
	return m.chosenAction
}

func (m *Model) recordUse(alias string) {
	if m.history == nil {
		return
	}
	_ = m.history.RecordUse(alias) // best-effort; never blocks connecting
}

func (m *Model) Init() tea.Cmd {
	return m.startReachabilityCheck()
}

// rebuildRows recomputes the filtered, flattened, currently-visible rows
// from cfg + the current search filter + status map. Column widths are
// computed from the filtered set only, matching host_list.py's render_tree
// (filter first, then size columns to what's actually shown).
func (m *Model) rebuildRows() {
	aliases := m.cfg.ListAliases()
	filterText := strings.ToLower(strings.TrimSpace(m.filterInput.Value()))

	displays := make(map[string]leafDisplay, len(aliases))
	var kept []string
	var keptLeaves []leafDisplay

	for _, alias := range aliases {
		params, _ := m.cfg.HostParams(alias)
		status := m.status[alias]
		if status == "" {
			status = "unknown"
		}
		ld := leafDisplay{
			Alias:    alias,
			Hostname: flattenValue(params["hostname"]),
			User:     flattenValue(params["user"]),
			Port:     flattenValue(params["port"]),
			HasExtra: hasExtraDirective(params),
			Status:   status,
		}
		if ld.Port == "" {
			ld.Port = "22"
		}
		if MatchesFilter(filterText, alias, params, status) {
			displays[alias] = ld
			kept = append(kept, alias)
			keptLeaves = append(keptLeaves, ld)
		}
	}

	m.colWidths = computeColWidths(keptLeaves)
	tree := BuildGroupTree(kept)
	m.rows = flattenRows(tree, 0, "", m.expanded, displays, nil)

	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// flattenRows walks the group tree in the same order host_list.py's
// _render_group does: child groups (sorted) recursed into first, then
// leaves (sorted), skipping recursion into a group that's been collapsed.
// ancestorContinues carries, for each level above depth, whether that
// ancestor has more siblings after it (so its guide line must keep drawing
// past this subtree) — see the row struct's docs.
func flattenRows(node *GroupNode, depth int, path string, expanded map[string]bool, displays map[string]leafDisplay, ancestorContinues []bool) []row {
	var rows []row

	groupNames := make([]string, 0, len(node.Children))
	for name := range node.Children {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	leafNames := make([]string, 0, len(node.Leaves))
	for name := range node.Leaves {
		leafNames = append(leafNames, name)
	}
	sort.Strings(leafNames)

	total := len(groupNames) + len(leafNames)
	i := 0

	for _, name := range groupNames {
		i++
		isLast := i == total
		childPath := name
		if path != "" {
			childPath = path + "/" + name
		}
		rows = append(rows, row{kind: rowGroup, depth: depth, label: name, path: childPath, isLast: isLast, ancestorContinues: ancestorContinues})
		if isExpanded(expanded, childPath) {
			childContinues := append(append([]bool{}, ancestorContinues...), !isLast)
			rows = append(rows, flattenRows(node.Children[name], depth+1, childPath, expanded, displays, childContinues)...)
		}
	}

	for _, name := range leafNames {
		i++
		isLast := i == total
		alias := node.Leaves[name]
		rows = append(rows, row{kind: rowLeaf, depth: depth, label: name, alias: alias, leaf: displays[alias], isLast: isLast, ancestorContinues: ancestorContinues})
	}

	return rows
}

// treePrefix renders r's guide-line prefix: a "│   " or "    " per ancestor
// level (depending on whether that ancestor still has siblings below), then
// "├── " or "└── " for r's own connection to its parent — the same visual
// language as Python's Tree widget.
func treePrefix(r row) string {
	var b strings.Builder
	for _, cont := range r.ancestorContinues {
		if cont {
			b.WriteString("│   ")
		} else {
			b.WriteString("    ")
		}
	}
	if r.depth > 0 {
		if r.isLast {
			b.WriteString("└── ")
		} else {
			b.WriteString("├── ")
		}
	}
	return b.String()
}

func isExpanded(expanded map[string]bool, path string) bool {
	v, ok := expanded[path]
	if !ok {
		return true
	}
	return v
}

// reachabilityResultMsg/reachabilityDoneMsg carry a generation number so a
// stale run (superseded by a newer refresh) is dropped instead of mutating
// state out from under the current one — the manual equivalent of Textual's
// @work(exclusive=True) worker cancellation.
type reachabilityResultMsg struct {
	generation int
	result     reachability.Result
}

type reachabilityDoneMsg struct {
	generation int
}

func listenForReachability(ch <-chan reachability.Result, gen int) tea.Cmd {
	return func() tea.Msg {
		result, ok := <-ch
		if !ok {
			return reachabilityDoneMsg{generation: gen}
		}
		return reachabilityResultMsg{generation: gen, result: result}
	}
}

// startReachabilityCheck cancels any in-flight check, resets every alias to
// "unknown" (so the UI shows all-checking dots immediately), and kicks off
// a fresh concurrent probe of every host.
func (m *Model) startReachabilityCheck() tea.Cmd {
	if m.reachCancel != nil {
		m.reachCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.reachCancel = cancel
	m.reachGeneration++
	gen := m.reachGeneration

	aliases := m.cfg.ListAliases()
	targets := make([]reachability.Target, 0, len(aliases))
	for _, alias := range aliases {
		params, _ := m.cfg.HostParams(alias)
		host := flattenValue(params["hostname"])
		if host == "" {
			host = alias
		}
		port := reachability.ResolvePort(flattenValue(params["port"]))
		targets = append(targets, reachability.Target{Alias: alias, Host: host, Port: port})
		m.status[alias] = "unknown"
	}
	m.rebuildRows()

	ch := reachability.CheckAll(ctx, targets)
	m.reachChan = ch
	return listenForReachability(ch, gen)
}

// saveConfig backs up the on-disk file once per process (before the first
// write of this run), then writes the current in-memory config back out.
func (m *Model) saveConfig() error {
	if !m.backedUp {
		if err := backupConfig(m.configPath); err != nil {
			return err
		}
		m.backedUp = true
	}
	return os.WriteFile(m.configPath, []byte(m.cfg.Serialize()), 0o644)
}

func backupConfig(path string) error {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configHome = filepath.Join(home, ".config")
	}
	dir := filepath.Join(configHome, "sshtui", "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := time.Now().Format("20060102T150405")
	return os.WriteFile(filepath.Join(dir, "config-"+stamp), data, 0o644)
}

// selectedAlias returns the alias of the currently-selected row, or "" if
// the cursor is on a group (or there's no selection at all).
func (m *Model) selectedAlias() string {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return ""
	}
	if r := m.rows[m.cursor]; r.kind == rowLeaf {
		return r.alias
	}
	return ""
}

func (m *Model) takenAliases() map[string]bool {
	taken := map[string]bool{}
	for _, a := range m.cfg.ListAliases() {
		taken[a] = true
	}
	return taken
}

// uniqueCloneAlias mirrors host_list.py's _unique_clone_alias: try
// "<source>-copy", then "-copy2", "-copy3", ... until one isn't taken.
func (m *Model) uniqueCloneAlias(source string) string {
	taken := m.takenAliases()
	if candidate := source + "-copy"; !taken[candidate] {
		return candidate
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-copy%d", source, i)
		if !taken[candidate] {
			return candidate
		}
	}
}

func equalDirectiveValues(a, b config.DirectiveValue) bool {
	if len(a.Values) != len(b.Values) {
		return false
	}
	for i := range a.Values {
		if a.Values[i] != b.Values[i] {
			return false
		}
	}
	return true
}

// diffDirectives compares a host's pre-edit params against the form's full
// new directive list and returns only what actually changed: brand-new or
// changed-value directives to set (in the form's own order), and keys that
// existed before but are no longer present at all. Mirrors host_list.py's
// edit-time diffing, which avoids rewriting (and thus reformatting) a line
// whose value the user didn't actually change.
func diffDirectives(existing map[string]config.DirectiveValue, newDirectives []config.OrderedDirective) (set []config.OrderedDirective, removedKeys []string) {
	seen := map[string]bool{}
	for _, d := range newDirectives {
		seen[d.Key] = true
		old, existed := existing[d.Key]
		if !existed || !equalDirectiveValues(old, d.Value) {
			set = append(set, d)
		}
	}
	for k := range existing {
		if !seen[k] {
			removedKeys = append(removedKeys, k)
		}
	}
	sort.Strings(removedKeys)
	return set, removedKeys
}

func (m *Model) handleHostEditDone(msg hostEditDoneMsg) (tea.Model, tea.Cmd) {
	m.hostEdit = nil
	if !msg.ok {
		return m, nil
	}

	if msg.editingAlias == "" {
		if err := m.cfg.AddHost(msg.alias, msg.directives); err != nil {
			m.statusMessage = err.Error()
			return m, nil
		}
	} else {
		alias := msg.editingAlias
		existing, _ := m.cfg.HostParams(alias)
		if msg.alias != alias {
			if err := m.cfg.RenameHost(alias, msg.alias); err != nil {
				m.statusMessage = err.Error()
				return m, nil
			}
			alias = msg.alias
		}
		set, removedKeys := diffDirectives(existing, msg.directives)
		if err := m.cfg.UpdateHost(alias, set, removedKeys); err != nil {
			m.statusMessage = err.Error()
			return m, nil
		}
	}

	if err := m.saveConfig(); err != nil {
		m.statusMessage = err.Error()
		return m, nil
	}
	m.rebuildRows()
	return m, m.startReachabilityCheck()
}

func (m *Model) handleScpDone(msg scpDoneMsg) (tea.Model, tea.Cmd) {
	m.scp = nil
	if !msg.ok {
		return m, nil
	}
	m.recordUse(msg.action.Alias)
	m.chosenAction = msg.action
	return m, tea.Quit
}

func (m *Model) handleConfirmDone(msg confirmDoneMsg) (tea.Model, tea.Cmd) {
	m.confirm = nil
	alias := m.pendingDeleteAlias
	m.pendingDeleteAlias = ""
	if !msg.ok || alias == "" {
		return m, nil
	}

	if err := m.cfg.RemoveHost(alias); err != nil {
		m.statusMessage = err.Error()
		return m, nil
	}
	delete(m.status, alias)
	if err := m.saveConfig(); err != nil {
		m.statusMessage = err.Error()
		return m, nil
	}
	m.rebuildRows()
	return m, nil // the removed host needs no reachability recheck
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case reachabilityResultMsg:
		if msg.generation != m.reachGeneration {
			return m, nil // superseded by a newer check; drop
		}
		status := "down"
		if msg.result.Reachable {
			status = "up"
		}
		m.status[msg.result.Alias] = status
		m.rebuildRows()
		return m, listenForReachability(m.reachChan, msg.generation)

	case reachabilityDoneMsg:
		return m, nil

	case hostEditDoneMsg:
		return m.handleHostEditDone(msg)

	case confirmDoneMsg:
		return m.handleConfirmDone(msg)

	case scpDoneMsg:
		return m.handleScpDone(msg)
	}

	if m.hostEdit != nil {
		var cmd tea.Cmd
		m.hostEdit, cmd = m.hostEdit.Update(msg)
		return m, cmd
	}
	if m.confirm != nil {
		var cmd tea.Cmd
		m.confirm, cmd = m.confirm.Update(msg)
		return m, cmd
	}
	if m.scp != nil {
		var cmd tea.Cmd
		m.scp, cmd = m.scp.Update(msg)
		return m, cmd
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		return m.handleKey(km)
	}
	if mm, ok := msg.(tea.MouseMsg); ok && !m.searchFocused {
		return m.handleMouse(mm)
	}
	return m, nil
}

// doubleClickThreshold is the max gap between two left-clicks on the same
// row for them to count as a double-click.
const doubleClickThreshold = 400 * time.Millisecond

// handleMouse supports the host list's mouse interactions: the wheel moves
// the cursor; a single click on a group toggles it (same as enter/space);
// a single click on a host selects it without connecting, so a stray click
// can't accidentally start an ssh session — connecting takes a double-click,
// same as opening an item in a file manager.
func (m *Model) handleMouse(mm tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch mm.Button {
	case tea.MouseButtonWheelUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.MouseButtonWheelDown:
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case tea.MouseButtonLeft:
		if mm.Action != tea.MouseActionPress {
			return m, nil
		}
		idx := m.rowScroll + (mm.Y - m.rowsStartY)
		if idx < 0 || idx >= len(m.rows) {
			return m, nil
		}

		now := time.Now()
		isDoubleClick := idx == m.lastClickIdx && now.Sub(m.lastClickTime) < doubleClickThreshold
		m.lastClickIdx = idx
		m.lastClickTime = now
		m.cursor = idx

		r := m.rows[idx]
		switch r.kind {
		case rowGroup:
			// A single click anywhere on a group row toggles it (not just
			// the ▾/▸ marker specifically) — same as pressing enter/space
			// on it, just via mouse.
			m.expanded[r.path] = !isExpanded(m.expanded, r.path)
			m.rebuildRows()
		case rowLeaf:
			if isDoubleClick {
				m.recordUse(r.alias)
				m.chosenAction = actions.Connect{Alias: r.alias}
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.statusMessage = ""

	if m.searchFocused {
		switch msg.String() {
		case "esc":
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			m.searchFocused = false
			m.rebuildRows()
			return m, nil
		case "enter":
			m.filterInput.Blur()
			m.searchFocused = false
			return m, nil
		}
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		m.rebuildRows()
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "enter", " ":
		if m.cursor >= 0 && m.cursor < len(m.rows) {
			r := m.rows[m.cursor]
			if r.kind == rowGroup {
				m.expanded[r.path] = !isExpanded(m.expanded, r.path)
				m.rebuildRows()
				return m, nil
			}
			m.recordUse(r.alias)
			m.chosenAction = actions.Connect{Alias: r.alias}
			return m, tea.Quit
		}
	case "R":
		if alias := m.selectedAlias(); alias != "" {
			logPath, err := actions.SessionLogPath(alias)
			if err != nil {
				m.statusMessage = err.Error()
				return m, nil
			}
			m.recordUse(alias)
			m.chosenAction = actions.Connect{Alias: alias, RecordTo: logPath}
			return m, tea.Quit
		}
	case "f":
		if alias := m.selectedAlias(); alias != "" {
			m.recordUse(alias)
			m.chosenAction = actions.Sftp{Alias: alias}
			return m, tea.Quit
		}
	case "/":
		m.searchFocused = true
		return m, m.filterInput.Focus()
	case "r":
		return m, m.startReachabilityCheck()
	case "a":
		m.hostEdit = newHostEditForAdd(m.takenAliases())
	case "e":
		if alias := m.selectedAlias(); alias != "" {
			params, _ := m.cfg.HostParams(alias)
			m.hostEdit = newHostEditForEdit(alias, params, m.takenAliases())
		}
	case "c":
		if alias := m.selectedAlias(); alias != "" {
			params, _ := m.cfg.HostParams(alias)
			m.hostEdit = newHostEditForClone(m.uniqueCloneAlias(alias), alias, params, m.takenAliases())
		}
	case "d":
		if alias := m.selectedAlias(); alias != "" {
			m.pendingDeleteAlias = alias
			m.confirm = newConfirm(fmt.Sprintf("Delete host %q?", alias))
		}
	case "s":
		if alias := m.selectedAlias(); alias != "" {
			m.scp = newScp(alias)
		}
	}
	return m, nil
}

func (m *Model) View() string {
	if m.hostEdit != nil {
		return m.hostEdit.View(m.width, m.height)
	}
	if m.confirm != nil {
		return m.confirm.View(m.width, m.height)
	}
	if m.scp != nil {
		return m.scp.View(m.width, m.height)
	}

	// Width() sets the content area; the box's rounded border (2 cols) plus
	// Padding(0,1) (2 cols) add 4 more, so subtract those to keep the
	// overall box exactly m.width wide instead of overflowing it.
	searchContentWidth := m.width - 4
	if searchContentWidth < 10 {
		searchContentWidth = 36 // no size info yet, or a very narrow terminal
	}
	searchBox := searchBarStyle.Width(searchContentWidth).Render(m.filterInput.View())

	bindings := footerBindings
	if m.searchFocused {
		bindings = searchFooterBindings
	}
	footer := renderFooter(m.width, bindings)
	footerLines := strings.Count(footer, "\n") + 1

	statusLines := 0
	if m.statusMessage != "" {
		statusLines = 1
	}

	// Fixed chrome: banner(1) + search box(3: border/content/border) +
	// column header(1) + status message + blank line before footer(1) +
	// footer. Everything else (the row list) gets whatever's left, so the
	// footer always stays pinned at the bottom instead of scrolling off
	// with a long host list.
	const bannerLines, searchBoxLines, headerLines, blankBeforeFooter = 1, 3, 1, 1
	chromeLines := bannerLines + searchBoxLines + headerLines + statusLines + blankBeforeFooter + footerLines

	startIdx, endIdx := 0, len(m.rows)
	if m.height > 0 {
		avail := m.height - chromeLines
		if avail < 1 {
			avail = 1
		}
		if len(m.rows) > avail {
			if m.cursor < m.rowScroll {
				m.rowScroll = m.cursor
			}
			if m.cursor >= m.rowScroll+avail {
				m.rowScroll = m.cursor - avail + 1
			}
			if maxScroll := len(m.rows) - avail; m.rowScroll > maxScroll {
				m.rowScroll = maxScroll
			}
			if m.rowScroll < 0 {
				m.rowScroll = 0
			}
			startIdx = m.rowScroll
			endIdx = startIdx + avail
			if endIdx > len(m.rows) {
				endIdx = len(m.rows)
			}
		} else {
			m.rowScroll = 0
		}
	}

	var b strings.Builder
	b.WriteString(m.renderBanner()) // self-pads to full width when m.width > 0
	b.WriteString("\n")
	b.WriteString(searchBox)
	b.WriteString("\n")
	b.WriteString(fillBar(m.width, headerBarStyle.Render(columnHeaderLine(m.colWidths))))
	b.WriteString("\n")

	m.rowsStartY = strings.Count(b.String(), "\n")

	if len(m.rows) == 0 {
		b.WriteString("  (no hosts match)\n")
	}
	for i := startIdx; i < endIdx; i++ {
		r := m.rows[i]
		prefixText := treePrefix(r)
		var contentText string
		switch r.kind {
		case rowGroup:
			marker := "▾"
			if !isExpanded(m.expanded, r.path) {
				marker = "▸"
			}
			contentText = marker + " " + sanitizeForDisplay(r.label)
		case rowLeaf:
			contentText = statusIcon(r.leaf.Status) + " " + rowColumns(r.leaf, m.colWidths)
		}

		var line string
		if i == m.cursor {
			// A single style applied to the whole plain (unstyled) row text,
			// not per-segment coloring wrapped afterward: nesting several
			// already-rendered lipgloss segments (each ending in its own
			// SGR reset) inside an outer Render() call would cancel the
			// outer highlight partway through, leaving only the first
			// segment (the guide-line prefix) actually highlighted — which
			// is exactly the bug being fixed here.
			line = cursorStyle.Render(prefixText + contentText)
		} else {
			switch r.kind {
			case rowGroup:
				line = treeGuideStyle.Render(prefixText) + groupStyle.Render(contentText)
			case rowLeaf:
				style := unknownStyle
				switch r.leaf.Status {
				case "up":
					style = upStyle
				case "down":
					style = downStyle
				}
				icon := statusIcon(r.leaf.Status)
				line = treeGuideStyle.Render(prefixText) + style.Render(icon) + " " +
					columnAliasField(r.leaf, m.colWidths) +
					secondaryColumnStyle.Render(columnSecondaryFields(r.leaf, m.colWidths))
			}
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	if m.statusMessage != "" {
		b.WriteString(errorStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(footer)

	return b.String()
}

var footerBindings = []footerBinding{
	{"↑/↓", "move"},
	{"enter", "connect"},
	{"R", "record"},
	{"f", "sftp"},
	{"s", "scp"},
	{"a", "add"},
	{"e", "edit"},
	{"c", "clone"},
	{"d", "delete"},
	{"/", "search"},
	{"r", "refresh"},
	{"q", "quit"},
}

var searchFooterBindings = []footerBinding{
	{"enter", "apply & return to list"},
	{"esc", "clear & cancel"},
}

// renderBanner shows the app name/version on the left and live host counts
// on the right (individual counts colored to match their status dots),
// right-aligned within the terminal width — matching the Python original's
// banner-row layout. Every segment (including plain separator text) is
// rendered through a bar-background style, not just padded afterward by the
// caller, so the whole row reads as one solid bar rather than styled text
// with gaps of the terminal's default background showing through.
func (m *Model) renderBanner() string {
	left := bannerTitleStyle.Render("sshtui v" + Version)

	total := len(m.cfg.ListAliases())
	up, down, checking := 0, 0, 0
	for _, alias := range m.cfg.ListAliases() {
		switch m.status[alias] {
		case "up":
			up++
		case "down":
			down++
		default:
			checking++
		}
	}
	right := barTextStyle.Render(fmt.Sprintf("%d hosts · ", total)) +
		bannerUpStyle.Render(fmt.Sprintf("%d", up)) + barTextStyle.Render(" up · ") +
		bannerDownStyle.Render(fmt.Sprintf("%d", down)) + barTextStyle.Render(" down · ") +
		bannerUnknownStyle.Render(fmt.Sprintf("%d", checking)) + barTextStyle.Render(" checking")

	if m.width > 0 {
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
		if gap < 1 {
			gap = 1
		}
		return left + barTextStyle.Render(strings.Repeat(" ", gap)) + right
	}
	return left + "  " + right
}
