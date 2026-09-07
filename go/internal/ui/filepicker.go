package ui

import (
	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
)

// filePickerDoneMsg carries the file picker's result back to whichever
// screen pushed it (currently only scpModel).
type filePickerDoneMsg struct {
	ok   bool
	path string
}

// filePickerModel wraps bubbles/filepicker for picking a local file or
// folder. Unlike Textual's DirectoryTree (which can only browse downward
// from its initial root — the reason file_picker.py had to swap in a whole
// new tree to "jump" elsewhere), bubbles/filepicker can navigate both up
// and down freely from any starting point, so no jump-to-path workaround is
// needed here.
type filePickerModel struct {
	picker filepicker.Model
}

func newFilePicker(startDir string) *filePickerModel {
	fp := filepicker.New()
	fp.CurrentDirectory = startDir
	fp.FileAllowed = true
	fp.DirAllowed = false // Enter always navigates into a directory, never selects it; ctrl+s selects the current folder instead (see Update)
	fp.Height = 15

	return &filePickerModel{picker: fp}
}

func (f *filePickerModel) Init() tea.Cmd {
	return f.picker.Init()
}

func (f *filePickerModel) Update(msg tea.Msg) (*filePickerModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			// Override filepicker's own default (esc is bound to "up one
			// directory" there) — in a modal, esc must mean "cancel",
			// matching every other screen in this app.
			return f, func() tea.Msg { return filePickerDoneMsg{ok: false} }
		case "ctrl+s":
			return f, func() tea.Msg { return filePickerDoneMsg{ok: true, path: f.picker.CurrentDirectory} }
		}
	}

	var cmd tea.Cmd
	f.picker, cmd = f.picker.Update(msg)
	if didSelect, path := f.picker.DidSelectFile(msg); didSelect {
		return f, func() tea.Msg { return filePickerDoneMsg{ok: true, path: path} }
	}
	return f, cmd
}

func (f *filePickerModel) View(width, height int) string {
	content := bannerStyle.Render("Choose a local file or folder") + "\n\n" +
		nestedPanelStyle.Render(f.picker.View()) +
		"\nenter/l open dir or pick file · h/backspace up a dir · ctrl+s use current folder · esc cancel"
	return renderDialog(width, height, content)
}
