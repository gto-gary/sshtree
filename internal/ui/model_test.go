package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/gto-gary/sshtree/internal/actions"
	"github.com/gto-gary/sshtree/internal/config"
	"github.com/gto-gary/sshtree/internal/reachability"
)

// newTestModel builds a Model over testConfig(t), backed by a throwaway
// config file path under a temp dir so any save performed during a test
// never touches a real file.
func newTestModel(t *testing.T) *Model {
	t.Helper()
	return New(testConfig(t), filepath.Join(t.TempDir(), "config"))
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	return config.ParseString(`Host web
  HostName web.example.com
  User deploy

Host db
  HostName db.example.com

Host grp--a
  HostName a.example.com

Host grp--b
  HostName b.example.com
`)
}

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+n":
		return tea.KeyMsg{Type: tea.KeyCtrlN}
	case "ctrl+d":
		return tea.KeyMsg{Type: tea.KeyCtrlD}
	case "ctrl+f":
		return tea.KeyMsg{Type: tea.KeyCtrlF}
	case "ctrl+u":
		return tea.KeyMsg{Type: tea.KeyCtrlU}
	case "ctrl+g":
		return tea.KeyMsg{Type: tea.KeyCtrlG}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}

// update applies one key and returns the resulting model. It never invokes
// any returned Cmd — most keys (navigation, typing) don't need that, and
// bubbles' textinput returns a cursor-blink Cmd (a blocking ~530ms
// context.WithTimeout under the hood) on nearly every keystroke, which
// would make tests that type multiple characters pathologically slow if
// invoked eagerly.
func update(t *testing.T, m *Model, key string) *Model {
	t.Helper()
	next, _ := m.Update(keyMsg(key))
	got, ok := next.(*Model)
	if !ok {
		t.Fatalf("Update did not return *Model")
	}
	return got
}

// commitKey is for the specific keys expected to produce one of our own
// synchronous child-screen "done" messages (ctrl+s/esc for the edit form,
// y/n for the confirm modal): it invokes the returned Cmd exactly once and,
// if it's a hostEditDoneMsg/confirmDoneMsg, feeds it back into Update a
// single additional time — simulating the one extra Bubble Tea runtime
// round-trip needed to see the mutation take effect. Whatever Cmd THAT
// second call returns (e.g. the reachability-check listener, which blocks
// on real network I/O) is deliberately left uninvoked.
func commitKey(t *testing.T, m *Model, key string) *Model {
	t.Helper()
	next, cmd := m.Update(keyMsg(key))
	got, ok := next.(*Model)
	if !ok {
		t.Fatalf("Update did not return *Model")
	}
	if cmd == nil {
		return got
	}
	switch followUp := cmd().(type) {
	case hostEditDoneMsg, confirmDoneMsg, scpDoneMsg:
		final, _ := got.Update(followUp)
		result, ok := final.(*Model)
		if !ok {
			t.Fatalf("Update did not return *Model")
		}
		return result
	}
	return got
}

// "web" and "db" have no "--" in their alias, so they fall into the "other"
// group (matching AliasSegments' documented behavior); "grp--a"/"grp--b"
// nest under a real "grp" group. Groups sort before/after each other
// alphabetically ("grp" < "other"), and within a group, subgroups (sorted)
// come before leaves (sorted).
func TestNewFlattensAndSortsRows(t *testing.T) {
	m := newTestModel(t)
	if len(m.rows) != 6 { // grp, a, b, other, db, web
		t.Fatalf("got %d rows, want 6: %+v", len(m.rows), m.rows)
	}
	want := []struct {
		kind  rowKind
		label string
	}{
		{rowGroup, "grp"},
		{rowLeaf, "a"},
		{rowLeaf, "b"},
		{rowGroup, "other"},
		{rowLeaf, "db"},
		{rowLeaf, "web"},
	}
	for i, w := range want {
		if m.rows[i].kind != w.kind || m.rows[i].label != w.label {
			t.Errorf("row %d = (kind=%v, label=%q), want (kind=%v, label=%q)",
				i, m.rows[i].kind, m.rows[i].label, w.kind, w.label)
		}
	}
}

func TestCursorNavigationClampsAtEnds(t *testing.T) {
	m := newTestModel(t)
	if m.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", m.cursor)
	}

	m = update(t, m, "up") // already at top, should clamp
	if m.cursor != 0 {
		t.Errorf("cursor after up-at-top = %d, want 0", m.cursor)
	}

	for i := 0; i < 10; i++ {
		m = update(t, m, "down")
	}
	if m.cursor != len(m.rows)-1 {
		t.Errorf("cursor after many downs = %d, want %d (last row)", m.cursor, len(m.rows)-1)
	}
}

func TestToggleGroupCollapsesAndHidesChildren(t *testing.T) {
	m := newTestModel(t)

	// Cursor starts on row 0, the "grp" group (first alphabetically).
	if m.rows[m.cursor].kind != rowGroup || m.rows[m.cursor].label != "grp" {
		t.Fatalf("expected cursor on group %q, got %+v", "grp", m.rows[m.cursor])
	}

	m = update(t, m, "enter")
	if len(m.rows) != 4 { // grp (collapsed), other, db, web — "a"/"b" hidden
		t.Fatalf("after collapse, got %d rows, want 4: %+v", len(m.rows), m.rows)
	}

	m = update(t, m, "enter") // toggle back open
	if len(m.rows) != 6 {
		t.Fatalf("after re-expand, got %d rows, want 6: %+v", len(m.rows), m.rows)
	}
}

func TestFooterIsContextSensitiveToSearchFocus(t *testing.T) {
	m := newTestModel(t)
	if strings.Contains(m.View(), "apply & return to list") {
		t.Error("footer shows search bindings before search is focused")
	}

	m = update(t, m, "/")
	view := m.View()
	if !strings.Contains(view, "apply & return to list") {
		t.Error("footer doesn't show search bindings while search is focused")
	}
	if strings.Contains(view, "quit") {
		t.Error("footer still shows normal-mode bindings while search is focused")
	}

	m = update(t, m, "esc")
	if !strings.Contains(m.View(), "quit") {
		t.Error("footer didn't return to normal-mode bindings after esc")
	}
}

func TestSearchFocusFilterAndClear(t *testing.T) {
	m := newTestModel(t)

	m = update(t, m, "/")
	if !m.searchFocused {
		t.Fatal("expected searchFocused after '/'")
	}

	for _, r := range "web" {
		m = update(t, m, string(r))
	}
	if m.filterInput.Value() != "web" {
		t.Fatalf("filter input value = %q, want %q", m.filterInput.Value(), "web")
	}
	// Only "web" survives the filter, still wrapped in its "other" group
	// (a group is shown whenever it has at least one surviving descendant).
	if len(m.rows) != 2 || m.rows[0].label != "other" || m.rows[1].alias != "web" {
		t.Fatalf("filtered rows = %+v, want [other, web]", m.rows)
	}

	m = update(t, m, "esc")
	if m.searchFocused {
		t.Error("expected searchFocused to be false after esc")
	}
	if m.filterInput.Value() != "" {
		t.Errorf("filter input value after esc = %q, want empty", m.filterInput.Value())
	}
	if len(m.rows) != 6 {
		t.Errorf("rows after clearing filter = %d, want 6 (unfiltered)", len(m.rows))
	}
}

func TestReachabilityMessageUpdatesStatus(t *testing.T) {
	m := newTestModel(t)
	m.reachGeneration = 1
	m.reachChan = make(chan reachability.Result) // never read from directly; Update issues its own listen

	m.Update(reachabilityResultMsg{generation: 1, result: reachability.Result{Alias: "web", Reachable: true}})
	if m.status["web"] != "up" {
		t.Errorf(`status["web"] = %q, want "up"`, m.status["web"])
	}

	m.Update(reachabilityResultMsg{generation: 1, result: reachability.Result{Alias: "db", Reachable: false}})
	if m.status["db"] != "down" {
		t.Errorf(`status["db"] = %q, want "down"`, m.status["db"])
	}
}

func TestStaleReachabilityGenerationIsDropped(t *testing.T) {
	m := newTestModel(t)
	m.reachGeneration = 2
	m.status["web"] = "unknown"

	// A result tagged with an old generation (from a superseded refresh)
	// must not be applied.
	m.Update(reachabilityResultMsg{generation: 1, result: reachability.Result{Alias: "web", Reachable: true}})
	if m.status["web"] != "unknown" {
		t.Errorf(`stale result changed status to %q, want it left as "unknown"`, m.status["web"])
	}
}

func rowIndexForAlias(m *Model, alias string) int {
	for i, r := range m.rows {
		if r.alias == alias {
			return i
		}
	}
	return -1
}

func typeIntoModel(t *testing.T, m *Model, s string) *Model {
	t.Helper()
	for _, r := range s {
		m = update(t, m, string(r))
	}
	return m
}

func TestAddHostEndToEnd(t *testing.T) {
	m := newTestModel(t)
	m = update(t, m, "a")
	if m.hostEdit == nil {
		t.Fatal("expected the add form to open")
	}
	m = typeIntoModel(t, m, "newhost")
	m = update(t, m, "tab")
	m = typeIntoModel(t, m, "newhost.example.com")
	m = commitKey(t, m, "ctrl+s")

	if m.hostEdit != nil {
		t.Fatal("expected the form to close after save")
	}
	params, ok := m.cfg.HostParams("newhost")
	if !ok {
		t.Fatal("expected 'newhost' to exist in the in-memory config")
	}
	if got := flattenValue(params["hostname"]); got != "newhost.example.com" {
		t.Errorf("hostname = %q, want %q", got, "newhost.example.com")
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		t.Fatalf("config file was not written: %v", err)
	}
	if !strings.Contains(string(data), "Host newhost") {
		t.Errorf("config file on disk missing the new host:\n%s", data)
	}
}

func TestEditHostChangesHostname(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")
	if m.cursor < 0 {
		t.Fatal("fixture host 'web' not found in rows")
	}

	m = update(t, m, "e")
	if m.hostEdit == nil {
		t.Fatal("expected the edit form to open")
	}
	m.hostEdit.hostname.SetValue("new.example.com")
	m = commitKey(t, m, "ctrl+s")

	if m.hostEdit != nil {
		t.Fatal("expected the form to close after save")
	}
	params, ok := m.cfg.HostParams("web")
	if !ok {
		t.Fatal("expected 'web' to still exist after editing")
	}
	if got := flattenValue(params["hostname"]); got != "new.example.com" {
		t.Errorf("hostname = %q, want %q", got, "new.example.com")
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		t.Fatalf("config file was not written: %v", err)
	}
	if !strings.Contains(string(data), "new.example.com") {
		t.Errorf("config file on disk not updated:\n%s", data)
	}
}

func TestRenameHostViaEdit(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "e")
	m.hostEdit.alias.SetValue("webrenamed")
	m = commitKey(t, m, "ctrl+s")

	if _, ok := m.cfg.HostParams("web"); ok {
		t.Error("old alias 'web' should no longer exist after rename")
	}
	if _, ok := m.cfg.HostParams("webrenamed"); !ok {
		t.Error("expected renamed host 'webrenamed' to exist")
	}
}

func TestCloneHostCreatesUniqueAlias(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "c")
	if m.hostEdit == nil {
		t.Fatal("expected the clone form to open")
	}
	if got := m.hostEdit.alias.Value(); got != "web-copy" {
		t.Errorf("pre-filled clone alias = %q, want %q", got, "web-copy")
	}
	m = commitKey(t, m, "ctrl+s")

	if _, ok := m.cfg.HostParams("web-copy"); !ok {
		t.Error("expected cloned host 'web-copy' to exist")
	}
	if _, ok := m.cfg.HostParams("web"); !ok {
		t.Error("original host 'web' should still exist after cloning")
	}
}

func TestDeleteHostRequiresConfirm(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "d")
	if m.confirm == nil {
		t.Fatal("expected the confirm modal to open")
	}

	m = commitKey(t, m, "n")
	if m.confirm != nil {
		t.Error("expected the confirm modal to close after declining")
	}
	if _, ok := m.cfg.HostParams("web"); !ok {
		t.Error("host should still exist after declining delete")
	}

	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "d")
	m = commitKey(t, m, "y")
	if _, ok := m.cfg.HostParams("web"); ok {
		t.Error("host should be gone after confirming delete")
	}
}

func TestSaveConfigCreatesBackupOnFirstWriteOnly(t *testing.T) {
	xdgHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgHome)

	configPath := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(configPath, []byte("Host web\n  HostName web.example.com\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Parse(configPath)
	if err != nil {
		t.Fatal(err)
	}
	m := New(cfg, configPath)

	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "e")
	m.hostEdit.hostname.SetValue("a.example.com")
	m = commitKey(t, m, "ctrl+s")

	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "e")
	m.hostEdit.hostname.SetValue("b.example.com")
	m = commitKey(t, m, "ctrl+s")

	backupDir := filepath.Join(xdgHome, "sshtree", "backups")
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("reading backup dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 backup file (only on the first write), got %d", len(entries))
	}
	data, err := os.ReadFile(filepath.Join(backupDir, entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "web.example.com") {
		t.Errorf("backup should hold the ORIGINAL pre-edit content, got:\n%s", data)
	}
}

func TestScpKeyOpensFormAndUploadChoosesScpAction(t *testing.T) {
	realFile := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(realFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "s")
	if m.scp == nil {
		t.Fatal("expected the scp form to open")
	}

	m = typeIntoModel(t, m, realFile)
	m.scp.moveFocus(2)
	m = typeIntoModel(t, m, "/remote/data.txt")
	m = commitKey(t, m, "ctrl+u")

	if m.scp != nil {
		t.Error("expected the scp form to close")
	}
	action, ok := m.ChosenAction().(actions.Scp)
	if !ok {
		t.Fatalf("ChosenAction() = %#v, want actions.Scp", m.ChosenAction())
	}
	if !action.Upload || action.LocalPath != realFile || action.RemotePath != "/remote/data.txt" {
		t.Errorf("action = %+v, unexpected fields", action)
	}
}

func TestEnterOnLeafChoosesConnectAndQuits(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")

	next, cmd := m.Update(keyMsg("enter"))
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected a quit Cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}

	action, ok := m.ChosenAction().(actions.Connect)
	if !ok {
		t.Fatalf("ChosenAction() = %#v, want actions.Connect", m.ChosenAction())
	}
	if action.Alias != "web" || action.RecordTo != "" {
		t.Errorf("action = %+v, want Alias=web RecordTo=empty", action)
	}
}

func TestEnterOnGroupTogglesInsteadOfChoosing(t *testing.T) {
	m := newTestModel(t)
	// Cursor starts on the "grp" group row.
	m = update(t, m, "enter")
	if m.ChosenAction() != nil {
		t.Errorf("expected no chosen action from toggling a group, got %#v", m.ChosenAction())
	}
}

func TestCapitalRChoosesConnectWithRecording(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")

	next, cmd := m.Update(keyMsg("R"))
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected a quit Cmd")
	}
	cmd() // drain it (tea.QuitMsg), matching the enter-connect test above

	action, ok := m.ChosenAction().(actions.Connect)
	if !ok {
		t.Fatalf("ChosenAction() = %#v, want actions.Connect", m.ChosenAction())
	}
	if action.Alias != "web" || action.RecordTo == "" {
		t.Errorf("action = %+v, want Alias=web with a non-empty RecordTo", action)
	}
}

func TestLowercaseFChoosesSftp(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")

	next, cmd := m.Update(keyMsg("f"))
	m = next.(*Model)
	cmd()

	action, ok := m.ChosenAction().(actions.Sftp)
	if !ok {
		t.Fatalf("ChosenAction() = %#v, want actions.Sftp", m.ChosenAction())
	}
	if action.Alias != "web" {
		t.Errorf("action.Alias = %q, want %q", action.Alias, "web")
	}
}

func TestScpCompletionChoosesScpActionAndQuits(t *testing.T) {
	m := newTestModel(t)
	m.cursor = rowIndexForAlias(m, "web")
	m = update(t, m, "s")
	m = typeIntoModel(t, m, "/local/path")
	m.scp.moveFocus(2)
	m = typeIntoModel(t, m, "/remote/path")

	next, cmd := m.Update(keyMsg("ctrl+g")) // download; skips the local-exists check
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected a Cmd producing scpDoneMsg")
	}
	final, cmd2 := m.Update(cmd())
	m = final.(*Model)
	if cmd2 == nil {
		t.Fatal("expected a quit Cmd after scpDoneMsg")
	}
	if _, ok := cmd2().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd2())
	}

	action, ok := m.ChosenAction().(actions.Scp)
	if !ok {
		t.Fatalf("ChosenAction() = %#v, want actions.Scp", m.ChosenAction())
	}
	if action.Alias != "web" || action.Upload {
		t.Errorf("action = %+v, want Alias=web Upload=false", action)
	}
}

func manyHostsConfig(t *testing.T, n int) *config.Config {
	t.Helper()
	var sb strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&sb, "Host host%02d\n  HostName host%02d.example.com\n\n", i, i)
	}
	return config.ParseString(sb.String())
}

func TestFooterStaysPinnedWithManyRows(t *testing.T) {
	m := New(manyHostsConfig(t, 40), filepath.Join(t.TempDir(), "config"))
	m.width, m.height = 100, 20 // a short terminal — far fewer rows fit than the 40 hosts

	view := m.View()
	if !strings.Contains(view, "quit") {
		t.Fatal("footer (containing 'quit') is missing from the view with many rows")
	}

	lines := strings.Split(view, "\n")
	if len(lines) > m.height+2 { // +2 slack for the trailing blank/footer-wrap edge cases
		t.Errorf("rendered %d lines, want roughly <= terminal height %d — row list isn't being clipped", len(lines), m.height)
	}

	// Not all 40 rows can be present if clipping is working.
	if strings.Count(view, ".example.com") >= 40 {
		t.Error("all rows appear to be rendered — the row list isn't being clipped to fit the terminal")
	}
}

func TestScrollingKeepsCursorVisible(t *testing.T) {
	m := New(manyHostsConfig(t, 40), filepath.Join(t.TempDir(), "config"))
	m.width, m.height = 100, 20

	for i := 0; i < len(m.rows)-1; i++ {
		m = update(t, m, "down")
		view := m.View()
		alias := m.rows[m.cursor].alias
		if alias == "" {
			continue // group row; only leaves have a distinctive alias to search for
		}
		if !strings.Contains(view, alias) {
			t.Fatalf("cursor row (alias %q, index %d) isn't visible in the rendered view after scrolling", alias, m.cursor)
		}
	}
}

func TestMouseWheelMovesCursor(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.View() // populate m.rowsStartY

	start := m.cursor
	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = next.(*Model)
	if m.cursor != start+1 {
		t.Errorf("cursor after wheel-down = %d, want %d", m.cursor, start+1)
	}

	next, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	m = next.(*Model)
	if m.cursor != start {
		t.Errorf("cursor after wheel-up = %d, want %d", m.cursor, start)
	}
}

func TestMouseClickSelectsRow(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.View() // populate m.rowsStartY

	// Click the 3rd visible row (index 2): rowsStartY + 2.
	clickY := m.rowsStartY + 2
	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: clickY})
	m = next.(*Model)
	if m.cursor != 2 {
		t.Errorf("cursor after clicking row 2 = %d, want 2", m.cursor)
	}
}

func TestMouseClickAboveOrBelowListIsIgnored(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.View()
	m.cursor = 1

	// Click well above the list (e.g. on the banner).
	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: 0})
	m = next.(*Model)
	if m.cursor != 1 {
		t.Errorf("cursor changed to %d after an out-of-range click, want unchanged 1", m.cursor)
	}

	// Click well below the list.
	next, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: 1000})
	m = next.(*Model)
	if m.cursor != 1 {
		t.Errorf("cursor changed to %d after an out-of-range click, want unchanged 1", m.cursor)
	}
}

func TestMouseClickOnGroupTogglesIt(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.View() // populate m.rowsStartY

	// Row 0 is the "grp" group (expanded by default).
	if m.rows[0].kind != rowGroup {
		t.Fatalf("expected row 0 to be a group, got %+v", m.rows[0])
	}
	before := len(m.rows)

	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: m.rowsStartY})
	m = next.(*Model)
	if len(m.rows) >= before {
		t.Errorf("expected clicking the group to collapse it (fewer rows); got %d rows, had %d", len(m.rows), before)
	}

	next, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: m.rowsStartY})
	m = next.(*Model)
	if len(m.rows) != before {
		t.Errorf("expected clicking the group again to re-expand it back to %d rows, got %d", before, len(m.rows))
	}
}

func TestMouseDoubleClickOnHostConnects(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.View()
	webIdx := rowIndexForAlias(m, "web")
	clickY := m.rowsStartY + webIdx

	// First click: selects, but does not connect.
	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: clickY})
	m = next.(*Model)
	if m.ChosenAction() != nil {
		t.Fatalf("single click should not connect, got %#v", m.ChosenAction())
	}

	// Second click on the same row, immediately after: connects.
	next, cmd := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: clickY})
	m = next.(*Model)
	if cmd == nil {
		t.Fatal("expected a quit Cmd from the double-click")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", cmd())
	}
	action, ok := m.ChosenAction().(actions.Connect)
	if !ok || action.Alias != "web" {
		t.Errorf("ChosenAction() = %#v, want actions.Connect{Alias: web}", m.ChosenAction())
	}
}

func TestMouseSlowSecondClickDoesNotConnect(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.View()
	webIdx := rowIndexForAlias(m, "web")
	clickY := m.rowsStartY + webIdx

	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: clickY})
	m = next.(*Model)
	m.lastClickTime = m.lastClickTime.Add(-time.Second) // simulate a slow second click, well past the threshold

	next, _ = m.Update(tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionPress, Y: clickY})
	m = next.(*Model)
	if m.ChosenAction() != nil {
		t.Errorf("expected two slow clicks not to connect, got %#v", m.ChosenAction())
	}
}

func TestMouseIgnoredWhileSearchFocused(t *testing.T) {
	m := newTestModel(t)
	m.width, m.height = 100, 30
	m.View()
	m = update(t, m, "/")
	start := m.cursor

	next, _ := m.Update(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	m = next.(*Model)
	if m.cursor != start {
		t.Errorf("cursor changed while search was focused: got %d, want unchanged %d", m.cursor, start)
	}
}

func TestQuitReturnsQuitCmd(t *testing.T) {
	m := newTestModel(t)
	_, cmd := m.Update(keyMsg("q"))
	if cmd == nil {
		t.Fatal("expected a non-nil Cmd for quit")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Errorf("expected tea.QuitMsg, got %T", msg)
	}
}
