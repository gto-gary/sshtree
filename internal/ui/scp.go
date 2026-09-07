package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/gto_gary/sshtui/internal/actions"
)

// scpDoneMsg carries the scp form's result back to the root model.
type scpDoneMsg struct {
	ok     bool
	action actions.Scp
}

// The focus chain follows the form's visual layout, matching scp.py's tab
// order: local path, then the Browse... button right next to it (same
// visual row), then remote path, then the bottom Upload/Download/Cancel
// button row.
const scpFieldCount = 6

const (
	scpFocusLocal = iota
	scpFocusBrowse
	scpFocusRemote
	scpFocusUpload
	scpFocusDownload
	scpFocusCancel
)

// scpModel configures an upload or download of one file/folder against a
// host, producing an actions.Scp on success. Actually running scp is the
// exec-flow milestone's job — this screen only builds the action.
type scpModel struct {
	alias string

	local  textinput.Model
	remote textinput.Model
	focus  int // one of the scpFocus* constants

	filePicker *filePickerModel

	errMsg string
}

func newScp(alias string) *scpModel {
	local := textinput.New()
	local.Placeholder = "local path"
	remote := textinput.New()
	remote.Placeholder = "remote path"
	local.Focus()

	return &scpModel{alias: alias, local: local, remote: remote}
}

func (s *scpModel) fieldCount() int { return scpFieldCount }

// buttonAt reports whether i is one of the button focus positions (as
// opposed to the local/remote text fields) — the position doubles as the
// button's identity, since each button occupies exactly one focus slot.
func (s *scpModel) buttonAt(i int) (btn int, ok bool) {
	switch i {
	case scpFocusBrowse, scpFocusUpload, scpFocusDownload, scpFocusCancel:
		return i, true
	}
	return 0, false
}

func (s *scpModel) fieldAt(i int) *textinput.Model {
	switch i {
	case scpFocusLocal:
		return &s.local
	case scpFocusRemote:
		return &s.remote
	}
	return nil // one of the button slots
}

func (s *scpModel) moveFocus(delta int) {
	if cur := s.fieldAt(s.focus); cur != nil {
		cur.Blur()
	}
	n := s.fieldCount()
	s.focus = ((s.focus+delta)%n + n) % n
	if next := s.fieldAt(s.focus); next != nil {
		next.Focus()
	}
}

func (s *scpModel) openFilePicker() (*scpModel, tea.Cmd) {
	s.filePicker = newFilePicker(s.startDirForPicker())
	return s, s.filePicker.Init()
}

// expandLocalPath expands a leading "~" or "~/", mirroring
// actions.expandUser (duplicated locally since that's an unexported detail
// of the actions package's print-only protocol, not something worth
// exporting just for this).
func expandLocalPath(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

// startDirForPicker mirrors scp.py's _start_dir_for_picker: if the local
// field already holds a directory, start there; if it holds a file, start
// at its parent; otherwise fall back to the home directory.
func (s *scpModel) startDirForPicker() string {
	if local := strings.TrimSpace(s.local.Value()); local != "" {
		expanded := expandLocalPath(local)
		if info, err := os.Stat(expanded); err == nil {
			if info.IsDir() {
				return expanded
			}
			return filepath.Dir(expanded)
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

func (s *scpModel) trySave(upload bool) (*scpModel, tea.Cmd) {
	local := strings.TrimSpace(s.local.Value())
	remote := strings.TrimSpace(s.remote.Value())
	if local == "" {
		s.errMsg = "local path is required"
		return s, nil
	}
	if remote == "" {
		s.errMsg = "remote path is required"
		return s, nil
	}
	// Only uploads can be checked: the remote path's existence can't be
	// verified without an active connection, and for uploads catching a
	// local typo here is strictly better than scp failing later.
	if upload {
		if _, err := os.Stat(expandLocalPath(local)); err != nil {
			s.errMsg = fmt.Sprintf("local path %q does not exist", local)
			return s, nil
		}
	}

	result := scpDoneMsg{ok: true, action: actions.Scp{
		Alias:      s.alias,
		LocalPath:  local,
		RemotePath: remote,
		Upload:     upload,
	}}
	return s, func() tea.Msg { return result }
}

func (s *scpModel) Update(msg tea.Msg) (*scpModel, tea.Cmd) {
	if fpDone, ok := msg.(filePickerDoneMsg); ok {
		s.filePicker = nil
		if fpDone.ok {
			s.local.SetValue(fpDone.path)
		}
		return s, nil
	}

	if s.filePicker != nil {
		var cmd tea.Cmd
		s.filePicker, cmd = s.filePicker.Update(msg)
		return s, cmd
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			return s, func() tea.Msg { return scpDoneMsg{ok: false} }
		case "tab", "down":
			s.moveFocus(1)
			return s, nil
		case "shift+tab", "up":
			s.moveFocus(-1)
			return s, nil
		case "ctrl+f":
			return s.openFilePicker()
		case "ctrl+u":
			return s.trySave(true)
		case "ctrl+g":
			return s.trySave(false)
		case "enter", " ":
			if btn, ok := s.buttonAt(s.focus); ok {
				switch btn {
				case scpFocusBrowse:
					return s.openFilePicker()
				case scpFocusUpload:
					return s.trySave(true)
				case scpFocusDownload:
					return s.trySave(false)
				case scpFocusCancel:
					return s, func() tea.Msg { return scpDoneMsg{ok: false} }
				}
				return s, nil
			}
		}
	}

	cur := s.fieldAt(s.focus)
	if cur == nil {
		return s, nil
	}
	var cmd tea.Cmd
	*cur, cmd = cur.Update(msg)
	return s, cmd
}

func (s *scpModel) View(width, height int) string {
	if s.filePicker != nil {
		return s.filePicker.View(width, height)
	}

	focusedButton, hasButtonFocus := s.buttonAt(s.focus)

	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", bannerStyle.Render("Copy file: "+s.alias))
	fmt.Fprintf(&b, "Local path:  %s  %s\n", s.local.View(),
		renderButton("Browse...", bannerStyle, hasButtonFocus && focusedButton == scpFocusBrowse))
	fmt.Fprintf(&b, "Remote path: %s\n", s.remote.View())

	if s.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("error: " + s.errMsg))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderButton("Upload →", upStyle.Bold(true), hasButtonFocus && focusedButton == scpFocusUpload))
	b.WriteString("  ")
	b.WriteString(renderButton("← Download", bannerStyle.Bold(true), hasButtonFocus && focusedButton == scpFocusDownload))
	b.WriteString("  ")
	b.WriteString(renderButton("Cancel", errorStyle.Bold(true), hasButtonFocus && focusedButton == scpFocusCancel))

	b.WriteString("\n\n")
	b.WriteString(renderKeyHints("tab/↓, shift+tab/↑", "move", "enter/space", "activate", "esc", "cancel"))
	return renderDialog(width, height, b.String())
}
