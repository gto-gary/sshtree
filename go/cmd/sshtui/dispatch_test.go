package main

import (
	"os/exec"
	"testing"
)

// TestShellQuoteIsShellSafe is the one security-sensitive test in this
// package: shellQuote is used to embed a config-derived alias (untrusted —
// ~/.ssh/config could be shared/edited by someone else) into a shell
// command string for `script -c`. Feed it back through an actual shell and
// confirm it round-trips as a single literal argument, for a battery of
// aliases containing shell metacharacters that would be dangerous if
// mishandled.
func TestShellQuoteIsShellSafe(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no /bin/sh available to verify against")
	}

	cases := []string{
		"plain-alias",
		"has'quote",
		"'; rm -rf /; echo '",
		"$(whoami)",
		"`whoami`",
		"a b c",
		"a\nb",
		"",
		"'''",
		"back\\slash",
	}
	for _, alias := range cases {
		t.Run(alias, func(t *testing.T) {
			quoted := shellQuote(alias)
			// Ask the shell to print exactly the one argument our quoting
			// produced, using printf with a literal %s so no further
			// shell-side interpretation of the *contents* occurs.
			out, err := exec.Command("sh", "-c", "printf '%s' "+quoted).Output()
			if err != nil {
				t.Fatalf("shell rejected quoted form %q: %v", quoted, err)
			}
			if string(out) != alias {
				t.Errorf("shellQuote(%q) = %q; shell round-trip gave %q, want %q",
					alias, quoted, out, alias)
			}
		})
	}
}
