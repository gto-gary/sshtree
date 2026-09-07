// Package actions defines the possible outcomes of the host-list screen:
// what the app should exec into once the TUI has torn down and restored the
// terminal (see cmd/sshtui's run loop for the actual exec dispatch).
package actions

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Action is something the app execs into after the TUI exits cleanly.
type Action interface {
	// PrintOnlyLines renders the action as one field per line, for the
	// `--print-only` shell-integration protocol (see shell/sshtui.zsh /
	// sshtui.bash): the wrapper reads these lines back and re-issues the
	// real command itself, so the user's shell history recalls the actual
	// ssh/sftp/scp invocation instead of "ran sshtui".
	PrintOnlyLines() []string
}

// Connect execs `ssh <alias>`. If RecordTo is non-empty, the session is
// additionally logged via `script` to that path.
type Connect struct {
	Alias    string
	RecordTo string // "" means not recording
}

func (a Connect) PrintOnlyLines() []string {
	if a.RecordTo != "" {
		return []string{"record", a.Alias, a.RecordTo}
	}
	return []string{"connect", a.Alias}
}

// Sftp execs `sftp <alias>`.
type Sftp struct {
	Alias string
}

func (a Sftp) PrintOnlyLines() []string {
	return []string{"sftp", a.Alias}
}

// Scp execs `scp` to upload or download one file/directory against Alias.
type Scp struct {
	Alias      string
	LocalPath  string
	RemotePath string
	Upload     bool // true: local -> remote; false: remote -> local
}

func (a Scp) PrintOnlyLines() []string {
	direction := "download"
	if a.Upload {
		direction = "upload"
	}
	return []string{"scp", direction, a.Alias, ExpandUser(a.LocalPath), a.RemotePath}
}

// SessionLogPath computes where a recorded session for alias would be
// logged: ~/Documents/sshtui/<alias>-<timestamp>.log. Computed fresh each
// call so the timestamp reflects when recording actually starts.
func SessionLogPath(alias string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	stamp := time.Now().Format("20060102T150405")
	return filepath.Join(home, "Documents", "sshtui", alias+"-"+stamp+".log"), nil
}

// ExpandUser expands a leading "~" or "~/" to the user's home directory.
// Falls back to the original path if the home directory can't be resolved,
// matching Path.expanduser()'s "best effort" behavior.
func ExpandUser(path string) string {
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
