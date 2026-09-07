// Command sshtui is a terminal UI for browsing and editing hosts in
// ~/.ssh/config.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"gitlab.com/gto_gary/sshtui/internal/config"
	"gitlab.com/gto_gary/sshtui/internal/ui"
)

func main() {
	printOnly := flag.Bool("print-only", false,
		"print the chosen action as lines instead of running it directly, "+
			"for the shell integration to run itself afterward (see --shell-init)")
	shellInit := flag.String("shell-init", "",
		`print shell integration code for the given shell ("zsh" or "bash") and exit; `+
			`add eval "$(sshtui --shell-init zsh)" to your shell rc file`)
	configPath := flag.String("config", defaultConfigPath(), "path to the ssh config file to browse/edit")
	flag.Parse()

	if *shellInit != "" {
		script, err := shellInitScript(*shellInit)
		if err != nil {
			fmt.Fprintf(os.Stderr, "sshtui: %v\n", err)
			os.Exit(1)
		}
		fmt.Print(script)
		return
	}

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "sshtui: could not determine home directory; pass --config explicitly")
		os.Exit(1)
	}

	cfg, err := config.Parse(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshtui: %v\n", err)
		os.Exit(1)
	}

	// Render to and read from the controlling terminal directly, not
	// stdin/stdout: --print-only is meant to be run as `output=$(sshtui
	// --print-only)`, which redirects stdout to a pipe for the shell to
	// capture — without this, the TUI's own rendered frames would end up
	// mixed into that captured output instead of just the plain
	// print-only-protocol lines printed after Run() returns.
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithInputTTY(), tea.WithMouseCellMotion()}
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer tty.Close()
		opts = append(opts, tea.WithOutput(tty))

		// lipgloss's default renderer detects color support by inspecting
		// os.Stdout specifically — when that's redirected (the exact case
		// above), it concludes there's no color support. Point the default
		// renderer at the real tty instead so color detection reflects the
		// actual terminal, not the redirected stdout.
		lipgloss.SetDefaultRenderer(lipgloss.NewRenderer(tty))
	}
	// Must run after the SetDefaultRenderer call above (if any) and before
	// the first View(): see InitStyles' doc comment for why this can't just
	// be ordinary package-level var initializers in internal/ui.
	ui.InitStyles()

	m := ui.New(cfg, *configPath)

	// tea.Program.Run() blocks until the app exits and the terminal is
	// fully restored, then returns the final model — only then is it safe
	// to exec ssh/sftp/scp into the same terminal.
	finalModel, err := tea.NewProgram(m, opts...).Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshtui: %v\n", err)
		os.Exit(1)
	}

	result, ok := finalModel.(*ui.Model)
	if !ok {
		return
	}
	action := result.ChosenAction()
	if action == nil {
		return
	}

	if *printOnly {
		for _, line := range action.PrintOnlyLines() {
			fmt.Println(line)
		}
		return
	}

	dispatch(action)
}

func defaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "config")
}
