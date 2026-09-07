package ui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/gto_gary/sshtui/internal/config"
)

func hostEditDone(t *testing.T, f *hostEditModel, key string) (hostEditDoneMsg, bool) {
	t.Helper()
	next, cmd := f.Update(keyMsg(key))
	*f = *next
	if cmd == nil {
		return hostEditDoneMsg{}, false
	}
	msg := cmd()
	done, ok := msg.(hostEditDoneMsg)
	return done, ok
}

func typeString(f *hostEditModel, s string) {
	for _, r := range s {
		f.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
}

func TestHostEditAddRequiresAlias(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	done, ok := hostEditDone(t, f, "ctrl+s")
	if ok {
		t.Fatalf("expected no done message (validation should block save), got %+v", done)
	}
	if f.errMsg == "" {
		t.Error("expected an error message for missing alias")
	}
}

func TestHostEditAddRejectsDuplicateAlias(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{"web": true})
	typeString(f, "web")
	done, ok := hostEditDone(t, f, "ctrl+s")
	if ok {
		t.Fatalf("expected no done message for a duplicate alias, got %+v", done)
	}
	if f.errMsg == "" {
		t.Error("expected a duplicate-alias error message")
	}
}

func TestHostEditAddSucceedsWithCoreFields(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{"web": true})
	typeString(f, "db")
	f.moveFocus(1) // -> hostname
	typeString(f, "db.example.com")
	f.moveFocus(1) // -> user
	typeString(f, "admin")

	done, ok := hostEditDone(t, f, "ctrl+s")
	if !ok || !done.ok {
		t.Fatalf("expected a successful done message, got ok=%v done=%+v", ok, done)
	}
	if done.alias != "db" {
		t.Errorf("alias = %q, want %q", done.alias, "db")
	}
	if done.editingAlias != "" {
		t.Errorf("editingAlias = %q, want empty (this is an add)", done.editingAlias)
	}
	want := []config.OrderedDirective{
		{Key: "hostname", Value: config.One("db.example.com")},
		{Key: "user", Value: config.One("admin")},
	}
	if !reflect.DeepEqual(done.directives, want) {
		t.Errorf("directives = %+v, want %+v", done.directives, want)
	}
}

func TestHostEditEditAllowsKeepingOwnAlias(t *testing.T) {
	f := newHostEditForEdit("web", map[string]config.DirectiveValue{
		"hostname": config.One("web.example.com"),
	}, map[string]bool{"web": true})

	done, ok := hostEditDone(t, f, "ctrl+s")
	if !ok || !done.ok {
		t.Fatalf("expected success keeping the same alias, got ok=%v done=%+v", ok, done)
	}
	if done.editingAlias != "web" {
		t.Errorf("editingAlias = %q, want %q", done.editingAlias, "web")
	}
}

func TestHostEditEscCancels(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	done, ok := hostEditDone(t, f, "esc")
	if !ok {
		t.Fatal("expected a done message for esc")
	}
	if done.ok {
		t.Error("expected ok=false for a cancelled form")
	}
}

func TestHostEditRemoveButtonRemovesRow(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	typeString(f, "web")

	f.addRow()
	typeString(f, "ProxyJump")
	f.moveFocus(1)
	typeString(f, "bastion")

	if len(f.rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(f.rows))
	}

	// Move onto that row's ✕ remove button (the 3rd of its 3 slots) and
	// activate it with enter, same as clicking it would.
	f.moveFocus(1)
	if rowIdx, ok := f.rowRemoveButtonAt(f.focus); !ok || rowIdx != 0 {
		t.Fatalf("expected focus on row 0's remove button, got focus=%d", f.focus)
	}
	next, _ := f.Update(keyMsg("enter"))
	f = next

	if len(f.rows) != 0 {
		t.Fatalf("expected the row to be removed, got %d rows", len(f.rows))
	}
}

func TestHostEditRowAddRemoveAndMultiValueGrouping(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	typeString(f, "web")

	f.addRow()
	typeString(f, "IdentityFile")
	f.moveFocus(1)
	typeString(f, "~/.ssh/id_a")

	f.addRow()
	typeString(f, "IdentityFile") // same key, different case doesn't matter — lowercased on save
	f.moveFocus(1)
	typeString(f, "~/.ssh/id_b")

	if len(f.rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(f.rows))
	}

	done, ok := hostEditDone(t, f, "ctrl+s")
	if !ok || !done.ok {
		t.Fatalf("expected success, got ok=%v done=%+v", ok, done)
	}
	var identityFile *config.OrderedDirective
	for i := range done.directives {
		if done.directives[i].Key == "identityfile" {
			identityFile = &done.directives[i]
		}
	}
	if identityFile == nil {
		t.Fatal("expected an identityfile directive in the result")
	}
	want := []string{"~/.ssh/id_a", "~/.ssh/id_b"}
	if !reflect.DeepEqual(identityFile.Value.Values, want) {
		t.Errorf("identityfile values = %v, want %v (order preserved)", identityFile.Value.Values, want)
	}

	// Remove the row we just added focus is on (the second IdentityFile row's
	// value field) and confirm it's gone.
	f.removeCurrentRow()
	if len(f.rows) != 1 {
		t.Fatalf("expected 1 row after removal, got %d", len(f.rows))
	}
}

func TestHostEditSkipsEmptyRows(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	typeString(f, "web")
	f.addRow() // left entirely blank

	done, ok := hostEditDone(t, f, "ctrl+s")
	if !ok || !done.ok {
		t.Fatalf("expected success, got ok=%v done=%+v", ok, done)
	}
	if len(done.directives) != 0 {
		t.Errorf("expected no directives (only a blank row was added), got %+v", done.directives)
	}
}

// focusButton tabs through the form until the given button index has focus,
// failing the test if it never does (bounded to avoid an infinite loop on a
// bug).
func focusButton(t *testing.T, f *hostEditModel, want int) {
	t.Helper()
	for i := 0; i < f.fieldCount()+1; i++ {
		if btn, ok := f.buttonAt(f.focus); ok && btn == want {
			return
		}
		f.moveFocus(1)
	}
	t.Fatalf("never reached button %d (focus ended at %d)", want, f.focus)
}

func TestHostEditArrowKeysMoveFocusLikeTab(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	if f.focus != 0 {
		t.Fatalf("expected initial focus 0 (alias), got %d", f.focus)
	}

	next, _ := f.Update(keyMsg("down"))
	f = next
	if f.focus != 1 {
		t.Errorf("focus after down = %d, want 1 (hostname)", f.focus)
	}

	next, _ = f.Update(keyMsg("down"))
	f = next
	if f.focus != 2 {
		t.Errorf("focus after second down = %d, want 2 (user)", f.focus)
	}

	next, _ = f.Update(keyMsg("up"))
	f = next
	if f.focus != 1 {
		t.Errorf("focus after up = %d, want 1 (hostname)", f.focus)
	}
}

func TestHostEditSaveButtonMatchesCtrlS(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	typeString(f, "web")
	focusButton(t, f, buttonSave)

	done, ok := hostEditDone(t, f, "enter")
	if !ok || !done.ok {
		t.Fatalf("expected the Save button to succeed, got ok=%v done=%+v", ok, done)
	}
	if done.alias != "web" {
		t.Errorf("alias = %q, want %q", done.alias, "web")
	}
}

func TestHostEditCancelButtonMatchesEsc(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	focusButton(t, f, buttonCancel)

	done, ok := hostEditDone(t, f, "enter")
	if !ok {
		t.Fatal("expected the Cancel button to produce a done message")
	}
	if done.ok {
		t.Error("expected ok=false from the Cancel button")
	}
}

func TestHostEditAddParameterButtonAddsRow(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	if len(f.rows) != 0 {
		t.Fatalf("expected 0 rows initially, got %d", len(f.rows))
	}
	focusButton(t, f, buttonAddRow)

	next, _ := f.Update(keyMsg("enter"))
	f = next
	if len(f.rows) != 1 {
		t.Fatalf("expected 1 row after activating '+ Add parameter', got %d", len(f.rows))
	}
}

func TestDirectivePickerFillsRowKey(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	f.addRow()
	// Focus is already on the new row's key field after addRow().
	if _, ok := f.keyFieldRowAt(f.focus); !ok {
		t.Fatalf("expected focus on a row key field, got index %d", f.focus)
	}

	next, cmd := f.Update(keyMsg("enter"))
	f = next
	if f.directivePicker == nil {
		t.Fatal("expected the directive picker to open")
	}
	if cmd != nil {
		cmd() // the picker's own Init()-equivalent work, if any; harmless
	}

	// Pick the first item in the (unfiltered) list.
	next2, cmd2 := f.directivePicker.Update(keyMsg("enter"))
	f.directivePicker = next2
	if cmd2 == nil {
		t.Fatal("expected selecting an item to produce a done message")
	}
	doneMsg := cmd2()

	final, _ := f.Update(doneMsg)
	f = final
	if f.directivePicker != nil {
		t.Error("expected the picker to close after a selection")
	}
	if got := f.rows[0].key.Value(); got != commonDirectives[0] {
		t.Errorf("row key = %q, want %q", got, commonDirectives[0])
	}
}

func TestDirectivePickerEscLeavesRowUnchanged(t *testing.T) {
	f := newHostEditForAdd(map[string]bool{})
	f.addRow()
	typeString(f, "MyCustomKey")

	next, _ := f.Update(keyMsg("enter"))
	f = next
	if f.directivePicker == nil {
		t.Fatal("expected the directive picker to open")
	}

	_, cmd := f.directivePicker.Update(keyMsg("esc"))
	doneMsg := cmd()
	final, _ := f.Update(doneMsg)
	f = final

	if f.directivePicker != nil {
		t.Error("expected the picker to close after esc")
	}
	if got := f.rows[0].key.Value(); got != "MyCustomKey" {
		t.Errorf("row key = %q, want unchanged %q", got, "MyCustomKey")
	}
}

func TestConfirmYesNo(t *testing.T) {
	c := newConfirm("delete?")

	_, cmd := c.Update(keyMsg("n"))
	msg := cmd().(confirmDoneMsg)
	if msg.ok {
		t.Error("expected ok=false for 'n'")
	}

	_, cmd = c.Update(keyMsg("y"))
	msg = cmd().(confirmDoneMsg)
	if !msg.ok {
		t.Error("expected ok=true for 'y'")
	}
}
