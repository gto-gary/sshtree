package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// directiveItem adapts a plain directive name to bubbles/list's Item
// interface (title-only, no description — a compact one-line-per-item
// list, since there are ~49 of these to scroll through).
type directiveItem string

func (d directiveItem) FilterValue() string { return string(d) }
func (d directiveItem) Title() string       { return string(d) }
func (d directiveItem) Description() string { return "" }

// directivePickerDoneMsg carries the picker's result back to the row that
// opened it.
type directivePickerDoneMsg struct {
	ok    bool
	value string
}

// directivePickerModel is a searchable list of commonDirectives, pushed
// when the user opens the picker on a row's key field. Picking one fills
// that row's key field — the field stays a normal free-text input the rest
// of the time, so this is a convenience shortcut, not a constraint (unlike
// Textual's Select widget, which can only ever hold one of its predefined
// options plus an explicit "Custom..." escape hatch).
type directivePickerModel struct {
	list list.Model
}

func newDirectivePicker(width, height int) *directivePickerModel {
	items := make([]list.Item, len(commonDirectives))
	for i, d := range commonDirectives {
		items[i] = directiveItem(d)
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	delegate.SetSpacing(0)

	if width <= 0 {
		width = 40
	}
	if height <= 0 {
		height = 20
	}
	innerWidth := dialogWidth(width) - 6 // dialog padding/border
	innerHeight := height*2/3 - 6        // leave room for the surrounding dialog chrome
	if innerHeight < 5 {
		innerHeight = 5
	}

	l := list.New(items, delegate, innerWidth, innerHeight)
	l.Title = "Choose a directive (or just type a custom one in the row)"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)

	return &directivePickerModel{list: l}
}

func (p *directivePickerModel) Update(msg tea.Msg) (*directivePickerModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			if !p.list.SettingFilter() {
				return p, func() tea.Msg { return directivePickerDoneMsg{ok: false} }
			}
		case "enter":
			if !p.list.SettingFilter() {
				if item, ok := p.list.SelectedItem().(directiveItem); ok {
					value := string(item)
					return p, func() tea.Msg { return directivePickerDoneMsg{ok: true, value: value} }
				}
			}
		}
	}
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p *directivePickerModel) View(width, height int) string {
	return renderDialog(width, height, p.list.View())
}
