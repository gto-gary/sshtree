package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/gto-gary/sshtui/internal/actions"
)

// dispatch execs into ssh/sftp/scp (or script, for a recorded connect)
// based on the chosen action. It never returns on success: syscall.Exec
// replaces the current process image, matching os.execvp's behavior in the
// Python original — the terminal has already been fully restored by the
// time this runs (tea.Program.Run() only returns after that), so ssh's
// password prompt etc. render into a normal terminal instead of a
// still-alt-screen one.
func dispatch(action actions.Action) {
	switch a := action.(type) {
	case actions.Connect:
		if a.RecordTo != "" {
			recordAndConnect(a)
			return
		}
		execInto("ssh", []string{"ssh", a.Alias})
	case actions.Sftp:
		execInto("sftp", []string{"sftp", a.Alias})
	case actions.Scp:
		local := actions.ExpandUser(a.LocalPath)
		remote := a.Alias + ":" + a.RemotePath
		if a.Upload {
			execInto("scp", []string{"scp", local, remote})
		} else {
			execInto("scp", []string{"scp", remote, local})
		}
	}
}

func recordAndConnect(a actions.Connect) {
	if err := os.MkdirAll(filepath.Dir(a.RecordTo), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "sshtui: %v\n", err)
		os.Exit(1)
	}
	// script's syntax for running a specific command differs by platform:
	// BSD/macOS takes it as trailing argv (no shell involved); util-linux/
	// Linux takes it as a single string via -c that IT shell-execs
	// internally — quote the alias for that one case, since it's the only
	// place in this codebase a value from ~/.ssh/config passes through a
	// shell rather than argv.
	var argv []string
	if runtime.GOOS == "darwin" {
		argv = []string{"script", "-q", a.RecordTo, "ssh", a.Alias}
	} else {
		argv = []string{"script", "-q", "-c", "ssh " + shellQuote(a.Alias), a.RecordTo}
	}
	execInto("script", argv)
}

// shellQuote produces a POSIX-shell-safe single-quoted form of s: close the
// quote, emit a backslash-escaped literal quote, reopen the quote — the
// standard technique for embedding a literal "'" inside a single-quoted
// shell string. This is the one place in the whole app where a
// config-derived value (the alias) passes through a shell instead of argv,
// so it must be exactly right regardless of what characters the alias
// contains.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// execInto replaces the current process with name (resolved via PATH).
// argv[0] should be name itself. Never returns on success.
func execInto(name string, argv []string) {
	path, err := exec.LookPath(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %q not found on PATH\n", name)
		os.Exit(1)
	}
	if err := syscall.Exec(path, argv, os.Environ()); err != nil {
		fmt.Fprintf(os.Stderr, "error: exec %q failed: %v\n", name, err)
		os.Exit(1)
	}
}
