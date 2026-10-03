package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/gto-gary/sshtree/internal/config"
)

// kvRow is one dynamic directive-name/value pair in the edit form. A
// directive that occurs more than once in a host's block (e.g. repeated
// IdentityFile) becomes multiple rows sharing the same key, mirroring
// host_edit.py's KeyValueRow expansion.
type kvRow struct {
	key   textinput.Model
	value textinput.Model
}

func newKVRow(key, value string) kvRow {
	k := textinput.New()
	k.Placeholder = "directive (enter to pick from a list)"
	k.SetValue(key)
	v := textinput.New()
	v.Placeholder = "value"
	v.SetValue(value)
	return kvRow{key: k, value: v}
}

// hostEditDoneMsg carries the form's result back to the root model.
// editingAlias is "" for add/clone (a brand new host); for an in-place edit
// it's the host's alias before this edit, so the root model can tell an add
// from an update (and detect a rename).
type hostEditDoneMsg struct {
	ok           bool
	alias        string
	editingAlias string
	directives   []config.OrderedDirective
}

// The three form-level buttons live at the end of the focus chain, after
// every alias/core-field/row field — matching host_edit.py's actual
// Button widgets ("+ Add parameter", "Save", "Cancel") rather than only
// being reachable via keyboard shortcuts.
const buttonCount = 3

const (
	buttonAddRow = iota
	buttonSave
	buttonCancel
)

// hostEditModel is the add/edit/clone form.
type hostEditModel struct {
	title string

	alias    textinput.Model
	hostname textinput.Model
	user     textinput.Model
	port     textinput.Model
	rows     []kvRow

	focus int // flat field index: 0=alias,1=hostname,2=user,3=port, then 3 slots per row (key, value, ✕ remove), then buttonCount button slots

	editingAlias string          // "" if this is add/clone
	taken        map[string]bool // every currently-taken alias

	errMsg string

	directivePicker *directivePickerModel
	pickerRowIdx    int

	// width/height are cached from the most recent View() call so Update()
	// (which doesn't otherwise receive terminal size) can size a freshly
	// opened directive picker sensibly.
	width, height int
}

func newHostEdit(title, initialAlias, editingAlias string, params map[string]config.DirectiveValue, taken map[string]bool) *hostEditModel {
	f := &hostEditModel{
		title:        title,
		editingAlias: editingAlias,
		taken:        taken,
	}

	f.alias = textinput.New()
	f.alias.Placeholder = "alias"
	f.alias.SetValue(initialAlias)

	f.hostname = textinput.New()
	f.hostname.Placeholder = "Hostname"
	f.hostname.SetValue(flattenValue(params["hostname"]))

	f.user = textinput.New()
	f.user.Placeholder = "User"
	f.user.SetValue(flattenValue(params["user"]))

	f.port = textinput.New()
	f.port.Placeholder = "Port"
	f.port.SetValue(flattenValue(params["port"]))

	var extraKeys []string
	for k := range params {
		if !coreFields[k] {
			extraKeys = append(extraKeys, k)
		}
	}
	sort.Strings(extraKeys)
	for _, k := range extraKeys {
		for _, v := range params[k].Values {
			f.rows = append(f.rows, newKVRow(k, v))
		}
	}

	f.focus = 0
	f.alias.Focus()
	return f
}

func newHostEditForAdd(taken map[string]bool) *hostEditModel {
	return newHostEdit("Add host", "", "", nil, taken)
}

func newHostEditForEdit(alias string, params map[string]config.DirectiveValue, taken map[string]bool) *hostEditModel {
	return newHostEdit(fmt.Sprintf("Edit host: %s", alias), alias, alias, params, taken)
}

func newHostEditForClone(cloneAlias, source string, params map[string]config.DirectiveValue, taken map[string]bool) *hostEditModel {
	return newHostEdit(fmt.Sprintf("Clone host: %s", source), cloneAlias, "", params, taken)
}

// rowSlots is how many focus positions each dynamic row occupies: key
// field, value field, and a ✕ remove button.
const rowSlots = 3

// fieldCount includes the trailing button slots, so Tab cycles through them
// too.
func (f *hostEditModel) fieldCount() int { return 4 + rowSlots*len(f.rows) + buttonCount }

// buttonAt returns which button (buttonAddRow/buttonSave/buttonCancel) i
// refers to, if any.
func (f *hostEditModel) buttonAt(i int) (btn int, ok bool) {
	base := 4 + rowSlots*len(f.rows)
	if i < base || i >= base+buttonCount {
		return 0, false
	}
	return i - base, true
}

// keyFieldRowAt returns which row's key field i refers to, if any (as
// opposed to that row's value field, its remove button, or a non-row
// field entirely).
func (f *hostEditModel) keyFieldRowAt(i int) (rowIdx int, ok bool) {
	if i < 4 {
		return 0, false
	}
	idx := i - 4
	rowIdx = idx / rowSlots
	if rowIdx < 0 || rowIdx >= len(f.rows) || idx%rowSlots != 0 {
		return 0, false
	}
	return rowIdx, true
}

// rowRemoveButtonAt returns which row's ✕ remove button i refers to, if any.
func (f *hostEditModel) rowRemoveButtonAt(i int) (rowIdx int, ok bool) {
	if i < 4 {
		return 0, false
	}
	idx := i - 4
	rowIdx = idx / rowSlots
	if rowIdx < 0 || rowIdx >= len(f.rows) || idx%rowSlots != 2 {
		return 0, false
	}
	return rowIdx, true
}

func (f *hostEditModel) fieldAt(i int) *textinput.Model {
	switch {
	case i == 0:
		return &f.alias
	case i == 1:
		return &f.hostname
	case i == 2:
		return &f.user
	case i == 3:
		return &f.port
	case i >= 4:
		idx := i - 4
		rowIdx := idx / rowSlots
		if rowIdx < 0 || rowIdx >= len(f.rows) {
			return nil // out of range, or one of the trailing button slots
		}
		switch idx % rowSlots {
		case 0:
			return &f.rows[rowIdx].key
		case 1:
			return &f.rows[rowIdx].value
		}
		return nil // the row's ✕ remove button — not a textinput
	}
	return nil
}

func (f *hostEditModel) moveFocus(delta int) {
	if cur := f.fieldAt(f.focus); cur != nil {
		cur.Blur()
	}
	n := f.fieldCount()
	f.focus = ((f.focus+delta)%n + n) % n
	if next := f.fieldAt(f.focus); next != nil {
		next.Focus()
	}
}

func (f *hostEditModel) addRow() {
	if cur := f.fieldAt(f.focus); cur != nil {
		cur.Blur()
	}
	f.rows = append(f.rows, newKVRow("", ""))
	f.focus = 4 + rowSlots*len(f.rows) - rowSlots // the new row's key field
	if next := f.fieldAt(f.focus); next != nil {
		next.Focus()
	}
}

// removeRow deletes rows[rowIdx], moving focus back into range if it was
// past the end afterward.
func (f *hostEditModel) removeRow(rowIdx int) {
	if rowIdx < 0 || rowIdx >= len(f.rows) {
		return
	}
	f.rows = append(f.rows[:rowIdx], f.rows[rowIdx+1:]...)
	n := f.fieldCount()
	if f.focus >= n {
		f.focus = n - 1
	}
	if f.focus < 0 {
		f.focus = 0
	}
	if next := f.fieldAt(f.focus); next != nil {
		next.Focus()
	}
}

// removeCurrentRow is the Ctrl+D shortcut: removes whichever row currently
// holds focus (its key field or value field — both count).
func (f *hostEditModel) removeCurrentRow() {
	if f.focus < 4 {
		return
	}
	f.removeRow((f.focus - 4) / rowSlots)
}

func (f *hostEditModel) Update(msg tea.Msg) (*hostEditModel, tea.Cmd) {
	if done, ok := msg.(directivePickerDoneMsg); ok {
		f.directivePicker = nil
		if done.ok && f.pickerRowIdx >= 0 && f.pickerRowIdx < len(f.rows) {
			f.rows[f.pickerRowIdx].key.SetValue(done.value)
		}
		return f, nil
	}

	if f.directivePicker != nil {
		var cmd tea.Cmd
		f.directivePicker, cmd = f.directivePicker.Update(msg)
		return f, cmd
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			return f, func() tea.Msg { return hostEditDoneMsg{ok: false} }
		case "ctrl+s":
			return f.trySave()
		case "tab", "down":
			f.moveFocus(1)
			return f, nil
		case "shift+tab", "up":
			f.moveFocus(-1)
			return f, nil
		case "ctrl+n":
			f.addRow()
			return f, nil
		case "ctrl+d":
			f.removeCurrentRow()
			return f, nil
		case "enter", " ":
			if btn, ok := f.buttonAt(f.focus); ok {
				switch btn {
				case buttonAddRow:
					f.addRow()
				case buttonSave:
					return f.trySave()
				case buttonCancel:
					return f, func() tea.Msg { return hostEditDoneMsg{ok: false} }
				}
				return f, nil
			}
			if rowIdx, ok := f.rowRemoveButtonAt(f.focus); ok {
				f.removeRow(rowIdx)
				return f, nil
			}
			if rowIdx, ok := f.keyFieldRowAt(f.focus); ok {
				f.pickerRowIdx = rowIdx
				f.directivePicker = newDirectivePicker(f.width, f.height)
				return f, nil
			}
		}
	}
	cur := f.fieldAt(f.focus)
	if cur == nil {
		return f, nil
	}
	var cmd tea.Cmd
	*cur, cmd = cur.Update(msg)
	return f, cmd
}

// buildDirectives assembles the ordered directive list the form currently
// represents: core fields first (only if non-empty), then extra rows
// grouped by (lowercased, trimmed) key in first-occurrence order — rows
// sharing a key become one multi-valued directive, in row order, mirroring
// host_edit.py's _save().
func (f *hostEditModel) buildDirectives() []config.OrderedDirective {
	var out []config.OrderedDirective
	if v := strings.TrimSpace(f.hostname.Value()); v != "" {
		out = append(out, config.OrderedDirective{Key: "hostname", Value: config.One(v)})
	}
	if v := strings.TrimSpace(f.user.Value()); v != "" {
		out = append(out, config.OrderedDirective{Key: "user", Value: config.One(v)})
	}
	if v := strings.TrimSpace(f.port.Value()); v != "" {
		out = append(out, config.OrderedDirective{Key: "port", Value: config.One(v)})
	}

	grouped := map[string][]string{}
	var order []string
	for _, r := range f.rows {
		key := strings.ToLower(strings.TrimSpace(r.key.Value()))
		value := strings.TrimSpace(r.value.Value())
		if key == "" || value == "" {
			continue
		}
		if _, seen := grouped[key]; !seen {
			order = append(order, key)
		}
		grouped[key] = append(grouped[key], value)
	}
	for _, key := range order {
		out = append(out, config.OrderedDirective{Key: key, Value: config.DirectiveValue{Values: grouped[key]}})
	}
	return out
}

func (f *hostEditModel) trySave() (*hostEditModel, tea.Cmd) {
	alias := strings.TrimSpace(f.alias.Value())
	if alias == "" {
		f.errMsg = "alias is required"
		return f, nil
	}
	if alias != f.editingAlias && f.taken[alias] {
		f.errMsg = fmt.Sprintf("alias %q is already in use", alias)
		return f, nil
	}

	result := hostEditDoneMsg{
		ok:           true,
		alias:        alias,
		editingAlias: f.editingAlias,
		directives:   f.buildDirectives(),
	}
	return f, func() tea.Msg { return result }
}

// renderButton draws label as a bracketed pseudo-button, highlighted with
// the same blue accent used for the tree's cursor row when focused, or in
// its role color otherwise (default/save=green/cancel=red — matching
// host_edit.py's actual Button variants) when not.
func renderButton(label string, roleStyle lipgloss.Style, focused bool) string {
	text := "[ " + label + " ]"
	if focused {
		return cursorStyle.Render(text)
	}
	return roleStyle.Render(text)
}

func (f *hostEditModel) View(width, height int) string {
	f.width, f.height = width, height

	if f.directivePicker != nil {
		return f.directivePicker.View(width, height)
	}

	var b strings.Builder
	b.WriteString(bannerStyle.Render(f.title))
	b.WriteString("\n\n")
	fmt.Fprintf(&b, "Alias:    %s\n", f.alias.View())
	fmt.Fprintf(&b, "Hostname: %s\n", f.hostname.View())
	fmt.Fprintf(&b, "User:     %s\n", f.user.View())
	fmt.Fprintf(&b, "Port:     %s\n", f.port.View())

	var rows strings.Builder
	rows.WriteString("Other parameters:\n")
	if len(f.rows) == 0 {
		rows.WriteString("(none yet — use \"+ Add parameter\" below)")
	} else {
		removeFocusRow, hasRemoveFocus := f.rowRemoveButtonAt(f.focus)
		for i, r := range f.rows {
			if i > 0 {
				rows.WriteString("\n")
			}
			removeBtn := renderButton("✕", errorStyle, hasRemoveFocus && removeFocusRow == i)
			fmt.Fprintf(&rows, "%s = %s  %s", r.key.View(), r.value.View(), removeBtn)
		}
	}
	b.WriteString("\n")
	b.WriteString(nestedPanelStyle.Render(rows.String()))
	b.WriteString("\n")

	if f.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("error: " + f.errMsg))
		b.WriteString("\n")
	}

	focusedButton, hasButtonFocus := f.buttonAt(f.focus)
	labels := []string{"+ Add parameter", "Save", "Cancel"}
	roles := []lipgloss.Style{bannerStyle, upStyle.Bold(true), errorStyle.Bold(true)}
	parts := make([]string, buttonCount)
	for i, label := range labels {
		parts[i] = renderButton(label, roles[i], hasButtonFocus && i == focusedButton)
	}

	b.WriteString("\n")
	b.WriteString(strings.Join(parts, "  "))
	b.WriteString("\n\n")
	b.WriteString(renderKeyHints("tab/↓, shift+tab/↑", "move", "enter/space", "activate", "esc", "cancel"))
	return renderDialog(width, height, b.String())
}
