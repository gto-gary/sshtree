// Package history tracks per-alias usage counts and last-used timestamps.
// It currently drives no display logic (hosts sort alphabetically, not by
// use) — kept only as a record for possible future use, matching the
// Python original's own documented intent.
package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Entry is one alias's recorded usage.
type Entry struct {
	Count    int    `json:"count"`
	LastUsed string `json:"last_used"`
}

// Store reads/writes the history file at Path. Use DefaultStore for the
// real on-disk location, or construct a Store directly (e.g. pointing at a
// temp file) in tests.
type Store struct {
	Path string
}

// DefaultStore returns a Store pointing at
// $XDG_CONFIG_HOME/sshtui/history.json (defaulting to ~/.config if
// XDG_CONFIG_HOME is unset).
func DefaultStore() (*Store, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configHome = filepath.Join(home, ".config")
	}
	return &Store{Path: filepath.Join(configHome, "sshtui", "history.json")}, nil
}

// load reads the history file, tolerating a missing or corrupt file by
// returning an empty map rather than an error — a damaged history file
// should never block the app from starting.
func (s *Store) load() map[string]Entry {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return map[string]Entry{}
	}
	var entries map[string]Entry
	if err := json.Unmarshal(data, &entries); err != nil {
		return map[string]Entry{}
	}
	if entries == nil {
		entries = map[string]Entry{}
	}
	return entries
}

// RecordUse increments alias's use count and sets its last-used timestamp
// to now, creating the history file/directory if needed.
func (s *Store) RecordUse(alias string) error {
	entries := s.load()
	e := entries[alias]
	e.Count++
	e.LastUsed = time.Now().Format("2006-01-02T15:04:05")
	entries[alias] = e

	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o644)
}
