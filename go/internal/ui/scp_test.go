package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func scpDone(t *testing.T, s *scpModel, key string) (scpDoneMsg, bool) {
	t.Helper()
	next, cmd := s.Update(keyMsg(key))
	*s = *next
	if cmd == nil {
		return scpDoneMsg{}, false
	}
	msg := cmd()
	done, ok := msg.(scpDoneMsg)
	return done, ok
}

func typeIntoScp(s *scpModel, text string) {
	for _, r := range text {
		s.Update(keyMsg(string(r)))
	}
}

func TestScpRequiresLocalAndRemotePaths(t *testing.T) {
	s := newScp("web")
	done, ok := scpDone(t, s, "ctrl+u")
	if ok {
		t.Fatalf("expected no done message with both paths empty, got %+v", done)
	}
	if s.errMsg == "" {
		t.Error("expected an error about the missing local path")
	}

	typeIntoScp(s, "/tmp/somefile")
	done, ok = scpDone(t, s, "ctrl+u")
	if ok {
		t.Fatalf("expected no done message with remote path still empty, got %+v", done)
	}
	if s.errMsg == "" {
		t.Error("expected an error about the missing remote path")
	}
}

func TestScpUploadRequiresLocalFileToExist(t *testing.T) {
	s := newScp("web")
	typeIntoScp(s, "/this/path/almost-certainly/does-not-exist")
	s.moveFocus(2)
	typeIntoScp(s, "/remote/path")

	done, ok := scpDone(t, s, "ctrl+u")
	if ok {
		t.Fatalf("expected upload to be rejected for a nonexistent local file, got %+v", done)
	}
	if s.errMsg == "" {
		t.Error("expected an error about the missing local file")
	}
}

func TestScpUploadSucceedsWithRealFile(t *testing.T) {
	realFile := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(realFile, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newScp("web")
	typeIntoScp(s, realFile)
	s.moveFocus(2)
	typeIntoScp(s, "/remote/data.txt")

	done, ok := scpDone(t, s, "ctrl+u")
	if !ok || !done.ok {
		t.Fatalf("expected a successful upload, got ok=%v done=%+v", ok, done)
	}
	if !done.action.Upload {
		t.Error("expected Upload=true")
	}
	if done.action.Alias != "web" || done.action.LocalPath != realFile || done.action.RemotePath != "/remote/data.txt" {
		t.Errorf("action = %+v, unexpected fields", done.action)
	}
}

func TestScpDownloadDoesNotCheckLocalPathExists(t *testing.T) {
	s := newScp("web")
	typeIntoScp(s, "/this/path/does-not-exist-yet")
	s.moveFocus(2)
	typeIntoScp(s, "/remote/path")

	done, ok := scpDone(t, s, "ctrl+g")
	if !ok || !done.ok {
		t.Fatalf("expected download to succeed without checking local existence, got ok=%v done=%+v", ok, done)
	}
	if done.action.Upload {
		t.Error("expected Upload=false for a download")
	}
}

func focusScpButton(t *testing.T, s *scpModel, want int) {
	t.Helper()
	for i := 0; i < s.fieldCount()+1; i++ {
		if btn, ok := s.buttonAt(s.focus); ok && btn == want {
			return
		}
		s.moveFocus(1)
	}
	t.Fatalf("never reached scp button %d (focus ended at %d)", want, s.focus)
}

func TestScpArrowKeysMoveFocusLikeTab(t *testing.T) {
	s := newScp("web")
	if s.focus != scpFocusLocal {
		t.Fatalf("expected initial focus scpFocusLocal, got %d", s.focus)
	}
	next, _ := s.Update(keyMsg("down"))
	s = next
	if s.focus != scpFocusBrowse {
		t.Errorf("focus after down = %d, want scpFocusBrowse (%d)", s.focus, scpFocusBrowse)
	}
	next, _ = s.Update(keyMsg("down"))
	s = next
	if s.focus != scpFocusRemote {
		t.Errorf("focus after second down = %d, want scpFocusRemote (%d)", s.focus, scpFocusRemote)
	}
	next, _ = s.Update(keyMsg("up"))
	s = next
	if s.focus != scpFocusBrowse {
		t.Errorf("focus after up = %d, want scpFocusBrowse (%d)", s.focus, scpFocusBrowse)
	}
}

func TestScpUploadButtonMatchesCtrlU(t *testing.T) {
	realFile := filepath.Join(t.TempDir(), "data.txt")
	if err := os.WriteFile(realFile, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newScp("web")
	typeIntoScp(s, realFile)
	s.moveFocus(2)
	typeIntoScp(s, "/remote/data.txt")

	focusScpButton(t, s, scpFocusUpload)
	done, ok := scpDone(t, s, "enter")
	if !ok || !done.ok || !done.action.Upload {
		t.Fatalf("expected the Upload button to succeed, got ok=%v done=%+v", ok, done)
	}
}

func TestScpDownloadButtonMatchesCtrlG(t *testing.T) {
	s := newScp("web")
	typeIntoScp(s, "/local/path")
	s.moveFocus(2)
	typeIntoScp(s, "/remote/path")

	focusScpButton(t, s, scpFocusDownload)
	done, ok := scpDone(t, s, "enter")
	if !ok || !done.ok || done.action.Upload {
		t.Fatalf("expected the Download button to succeed, got ok=%v done=%+v", ok, done)
	}
}

func TestScpCancelButtonMatchesEsc(t *testing.T) {
	s := newScp("web")
	focusScpButton(t, s, scpFocusCancel)

	done, ok := scpDone(t, s, "enter")
	if !ok {
		t.Fatal("expected the Cancel button to produce a done message")
	}
	if done.ok {
		t.Error("expected ok=false from the Cancel button")
	}
}

func TestScpBrowseButtonOpensFilePicker(t *testing.T) {
	s := newScp("web")
	focusScpButton(t, s, scpFocusBrowse)

	next, _ := s.Update(keyMsg("enter"))
	s = next
	if s.filePicker == nil {
		t.Fatal("expected the Browse button to open the file picker")
	}
}

func TestScpEscCancels(t *testing.T) {
	s := newScp("web")
	done, ok := scpDone(t, s, "esc")
	if !ok {
		t.Fatal("expected a done message for esc")
	}
	if done.ok {
		t.Error("expected ok=false for a cancelled scp form")
	}
}

func TestScpBrowseOpensFilePickerAndSelectionFillsLocalField(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "pick-me.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	s := newScp("web")
	next, cmd := s.Update(keyMsg("ctrl+f"))
	s = next
	if s.filePicker == nil {
		t.Fatal("expected ctrl+f to open the file picker")
	}
	if cmd != nil {
		cmd() // runs the picker's Init (initial readDir); harmless synchronous work
	}
	s.filePicker.picker.CurrentDirectory = dir

	// Simulate the picker directly reporting a selection, as if the user
	// had navigated to `file` and pressed enter.
	next2, _ := s.Update(filePickerDoneMsg{ok: true, path: file})
	s = next2
	if s.filePicker != nil {
		t.Error("expected the file picker to close after a selection")
	}
	if s.local.Value() != file {
		t.Errorf("local field = %q, want %q", s.local.Value(), file)
	}
}

func TestFilePickerEscCancelsRatherThanNavigatingUp(t *testing.T) {
	fp := newFilePicker(t.TempDir())
	_, cmd := fp.Update(keyMsg("esc"))
	if cmd == nil {
		t.Fatal("expected esc to produce a done message")
	}
	msg, ok := cmd().(filePickerDoneMsg)
	if !ok {
		t.Fatalf("expected filePickerDoneMsg, got %T", msg)
	}
	if msg.ok {
		t.Error("expected ok=false for esc")
	}
}

func TestFilePickerCtrlSSelectsCurrentDirectory(t *testing.T) {
	dir := t.TempDir()
	fp := newFilePicker(dir)
	_, cmd := fp.Update(keyMsg("ctrl+s"))
	if cmd == nil {
		t.Fatal("expected ctrl+s to produce a done message")
	}
	msg := cmd().(filePickerDoneMsg)
	if !msg.ok || msg.path != dir {
		t.Errorf("got %+v, want ok=true path=%q", msg, dir)
	}
}
