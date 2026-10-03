package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRecordUseCreatesFile(t *testing.T) {
	store := &Store{Path: filepath.Join(t.TempDir(), "sshtree", "history.json")}

	if err := store.RecordUse("web"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(store.Path)
	if err != nil {
		t.Fatalf("history file not created: %v", err)
	}
	var entries map[string]Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("history file isn't valid JSON: %v", err)
	}
	if entries["web"].Count != 1 {
		t.Errorf("count = %d, want 1", entries["web"].Count)
	}
	if entries["web"].LastUsed == "" {
		t.Error("last_used is empty, want a timestamp")
	}
}

func TestRecordUseIncrementsExisting(t *testing.T) {
	store := &Store{Path: filepath.Join(t.TempDir(), "history.json")}

	for i := 0; i < 3; i++ {
		if err := store.RecordUse("web"); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.RecordUse("db"); err != nil {
		t.Fatal(err)
	}

	entries := store.load()
	if entries["web"].Count != 3 {
		t.Errorf("web count = %d, want 3", entries["web"].Count)
	}
	if entries["db"].Count != 1 {
		t.Errorf("db count = %d, want 1", entries["db"].Count)
	}
}

func TestLoadToleratesMissingFile(t *testing.T) {
	store := &Store{Path: filepath.Join(t.TempDir(), "does-not-exist.json")}
	entries := store.load()
	if len(entries) != 0 {
		t.Errorf("expected empty map for missing file, got %v", entries)
	}
}

func TestLoadToleratesCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := &Store{Path: path}
	entries := store.load()
	if len(entries) != 0 {
		t.Errorf("expected empty map for corrupt file, got %v", entries)
	}

	// A subsequent RecordUse should recover by overwriting the corrupt file.
	if err := store.RecordUse("web"); err != nil {
		t.Fatal(err)
	}
	if got := store.load()["web"].Count; got != 1 {
		t.Errorf("count after recovery = %d, want 1", got)
	}
}

func TestDefaultStoreRespectsXDGConfigHome(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	store, err := DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, "sshtree", "history.json")
	if store.Path != want {
		t.Errorf("Path = %q, want %q", store.Path, want)
	}
}
