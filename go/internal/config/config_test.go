package config

import (
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

var updateGolden = flag.Bool("update", false, "update golden files instead of comparing against them")

func fixturePath(name string) string { return filepath.Join("testdata", "fixtures", name) }
func goldenPath(name string) string  { return filepath.Join("testdata", "golden", name) }

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := goldenPath(name)
	if *updateGolden {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v (run with -update to create it)", path, err)
	}
	if got != string(want) {
		t.Errorf("golden mismatch for %s:\n--- got ---\n%s--- want ---\n%s", name, got, string(want))
	}
}

// TestRoundTrip is the primary safety net: every fixture must serialize back
// byte-for-byte identical to its original text, and re-parsing the
// serialized output must yield a structurally identical Config. A bug here
// means untouched content in a real ~/.ssh/config could be silently
// mutated.
func TestRoundTrip(t *testing.T) {
	matches, err := filepath.Glob(filepath.Join("testdata", "fixtures", "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) == 0 {
		t.Fatal("no fixtures found")
	}
	for _, path := range matches {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			original := readFile(t, path)

			cfg1 := ParseString(original)
			out := cfg1.Serialize()
			if out != original {
				t.Errorf("round-trip not byte-identical:\n--- got ---\n%s--- want ---\n%s", out, original)
			}

			cfg2 := ParseString(out)
			if !reflect.DeepEqual(cfg1, cfg2) {
				t.Errorf("parse->serialize->parse is not idempotent for %s", name)
			}
		})
	}
}

func TestParseMissingFile(t *testing.T) {
	cfg, err := Parse(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("Parse of missing file returned error: %v", err)
	}
	if len(cfg.Nodes) != 0 {
		t.Fatalf("expected empty Config, got %d nodes", len(cfg.Nodes))
	}
	if err := cfg.AddHost("new", []OrderedDirective{
		{Key: "HostName", Value: One("new.example.com")},
	}); err != nil {
		t.Fatal(err)
	}
	got := cfg.Serialize()
	want := "Host new\n  HostName new.example.com\n"
	if got != want {
		t.Errorf("AddHost on empty config:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestListAliasesExcludesWildcard(t *testing.T) {
	cfg := ParseString(readFile(t, fixturePath("host_wildcard_defaults.txt")))
	got := cfg.ListAliases()
	want := []string{"web", "db"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListAliases = %v, want %v", got, want)
	}
}

func TestMultiPatternHostLookup(t *testing.T) {
	cfg := ParseString(readFile(t, fixturePath("multi_pattern_host.txt")))
	alias := "foo bar *.example.com"

	aliases := cfg.ListAliases()
	if len(aliases) != 1 || aliases[0] != alias {
		t.Fatalf("ListAliases = %v, want [%q]", aliases, alias)
	}

	params, ok := cfg.HostParams(alias)
	if !ok {
		t.Fatalf("HostParams(%q) not found", alias)
	}
	if params["user"].First() != "shared" {
		t.Errorf("user = %v, want shared", params["user"])
	}
}

func TestEqualsSyntaxTokenizesValueWithSpaces(t *testing.T) {
	cfg := ParseString(readFile(t, fixturePath("equals_syntax.txt")))
	params, ok := cfg.HostParams("bastion")
	if !ok {
		t.Fatal("host bastion not found")
	}
	want := "ssh -W %h:%p relay.example.com"
	if got := params["proxycommand"].First(); got != want {
		t.Errorf("proxycommand = %q, want %q", got, want)
	}
}

// TestOperations exercises AddHost/UpdateHost/RemoveHost/RenameHost against
// each fixture and checks the result against a golden file.
func TestOperations(t *testing.T) {
	type step func(t *testing.T, cfg *Config)

	cases := []struct {
		name    string
		fixture string
		golden  string
		step    step
	}{
		{
			name:    "basic_update_port",
			fixture: "basic.txt",
			golden:  "basic_update_port.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.UpdateHost("web", []OrderedDirective{
					{Key: "Port", Value: One("2200")},
				}, nil); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "comments_and_blanks_remove_staging",
			fixture: "comments_and_blanks.txt",
			golden:  "comments_and_blanks_remove_staging.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.RemoveHost("staging"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "equals_syntax_update_user",
			fixture: "equals_syntax.txt",
			golden:  "equals_syntax_update_user.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.UpdateHost("bastion", []OrderedDirective{
					{Key: "User", Value: One("svc")},
				}, nil); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "multi_identityfile_shrink",
			fixture: "multi_identityfile.txt",
			golden:  "multi_identityfile_shrink.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.UpdateHost("multi", []OrderedDirective{
					{Key: "IdentityFile", Value: Many([]string{"~/.ssh/id_ed25519", "~/.ssh/id_rsa_backup"})},
				}, nil); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "multi_identityfile_grow",
			fixture: "multi_identityfile.txt",
			golden:  "multi_identityfile_grow.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.UpdateHost("multi", []OrderedDirective{
					{Key: "IdentityFile", Value: Many([]string{
						"~/.ssh/id_ed25519", "~/.ssh/id_rsa_legacy", "~/.ssh/id_rsa_backup", "~/.ssh/id_ecdsa_new",
					})},
				}, nil); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "host_wildcard_defaults_add",
			fixture: "host_wildcard_defaults.txt",
			golden:  "host_wildcard_defaults_add.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.AddHost("cache", []OrderedDirective{
					{Key: "HostName", Value: One("cache.example.com")},
					{Key: "User", Value: One("deploy")},
				}); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "mixed_indentation_add_key",
			fixture: "mixed_indentation.txt",
			golden:  "mixed_indentation_add_key.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.UpdateHost("spaced4", []OrderedDirective{
					{Key: "Compression", Value: One("yes")},
				}, nil); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "rename_db_to_database",
			fixture: "basic.txt",
			golden:  "basic_rename_db.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.RenameHost("db", "database"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name:    "remove_keys",
			fixture: "equals_syntax.txt",
			golden:  "equals_syntax_remove_proxycommand.txt",
			step: func(t *testing.T, cfg *Config) {
				if err := cfg.UpdateHost("bastion", nil, []string{"ProxyCommand"}); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := ParseString(readFile(t, fixturePath(tc.fixture)))
			tc.step(t, cfg)
			checkGolden(t, tc.golden, cfg.Serialize())
		})
	}
}
