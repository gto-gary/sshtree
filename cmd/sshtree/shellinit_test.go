package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestShellInitScriptKnownShells(t *testing.T) {
	for _, shell := range []string{"zsh", "bash"} {
		t.Run(shell, func(t *testing.T) {
			script, err := shellInitScript(shell)
			if err != nil {
				t.Fatalf("shellInitScript(%q) returned error: %v", shell, err)
			}
			if !strings.Contains(script, "sshtree() {") {
				t.Errorf("script doesn't define an sshtree() function:\n%s", script)
			}
			if !strings.Contains(script, "command sshtree --print-only") {
				t.Errorf("script doesn't call command sshtree --print-only:\n%s", script)
			}
			if strings.Contains(script, "source /path/to") {
				t.Error("script still references the old file-sourcing setup instead of eval")
			}
		})
	}
}

func TestShellInitScriptUnknownShell(t *testing.T) {
	_, err := shellInitScript("fish")
	if err == nil {
		t.Fatal("expected an error for an unsupported shell")
	}
}

// TestShellInitScriptZshIsSyntacticallyValid feeds the printed zsh script
// through `zsh -n` (parse-only, no execution) to catch a real syntax error
// rather than just checking for expected substrings.
func TestShellInitScriptZshIsSyntacticallyValid(t *testing.T) {
	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not available")
	}
	script, err := shellInitScript("zsh")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("zsh", "-n")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("zsh -n rejected the script: %v\n%s", err, out)
	}
}

// TestShellInitScriptBashIsSyntacticallyValid does the same for bash.
func TestShellInitScriptBashIsSyntacticallyValid(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	script, err := shellInitScript("bash")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-n")
	cmd.Stdin = strings.NewReader(script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("bash -n rejected the script: %v\n%s", err, out)
	}
}
