// Command sshtui is a terminal UI for browsing and editing hosts in
// ~/.ssh/config.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"gitlab.com/gto_gary/sshtui/go/internal/config"
	"gitlab.com/gto_gary/sshtui/go/internal/ui"
)

func main() {
	printOnly := flag.Bool("print-only", false,
		"print the chosen action as lines instead of running it directly, "+
			"for a wrapping shell function to run itself afterward (see shell/)")
	configPath := flag.String("config", defaultConfigPath(), "path to the ssh config file to browse/edit")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "sshtui: could not determine home directory; pass --config explicitly")
		os.Exit(1)
	}

	cfg, err := config.Parse(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sshtui: %v\n", err)
		os.Exit(1)
	}

	m := ui.New(cfg, *configPath)

	// Render to and read from the controlling terminal directly, not
	// stdin/stdout: --print-only is meant to be run as `output=$(sshtui
	// --print-only)`, which redirects stdout to a pipe for the shell to
	// capture — without this, the TUI's own rendered frames would end up
	// mixed into that captured output instead of just the plain
	// print-only-protocol lines printed after Run() returns.
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithInputTTY(), tea.WithMouseCellMotion()}
	if tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0); err == nil {
		defer tty.Close()
		opts = append(opts, tea.WithOutput(tty))
	}

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
