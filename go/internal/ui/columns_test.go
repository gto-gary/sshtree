package ui

import "testing"

func TestComputeColWidthsGrowsToLongestValue(t *testing.T) {
	leaves := []leafDisplay{
		{Alias: "web", Hostname: "web.example.com", User: "deploy"},
		{Alias: "a-much-longer-alias", Hostname: "x", User: "u"},
	}
	w := computeColWidths(leaves)
	if w.Alias != len("a-much-longer-alias") {
		t.Errorf("Alias width = %d, want %d", w.Alias, len("a-much-longer-alias"))
	}
	if w.Hostname != len("web.example.com") {
		t.Errorf("Hostname width = %d, want %d", w.Hostname, len("web.example.com"))
	}
}

func TestComputeColWidthsNeverShrinksBelowHeader(t *testing.T) {
	w := computeColWidths(nil)
	if w.Alias != len("Alias") || w.Hostname != len("Hostname") || w.User != len("User") {
		t.Errorf("widths with no rows = %+v, want header-sized minimums", w)
	}
}

func TestSanitizeForDisplayStripsControlCharacters(t *testing.T) {
	malicious := "web\x1b[31mRED\x1b[0m"
	got := sanitizeForDisplay(malicious)
	want := "web[31mRED[0m"
	if got != want {
		t.Errorf("sanitizeForDisplay(%q) = %q, want %q", malicious, got, want)
	}
}

func TestSanitizeForDisplayLeavesNormalTextAlone(t *testing.T) {
	s := "web.example.com"
	if got := sanitizeForDisplay(s); got != s {
		t.Errorf("sanitizeForDisplay(%q) = %q, want unchanged", s, got)
	}
}

func TestStatusIcon(t *testing.T) {
	if statusIcon("unknown") != "○" {
		t.Error(`expected "○" for unknown status`)
	}
	if statusIcon("up") != "●" || statusIcon("down") != "●" {
		t.Error(`expected "●" for up/down status`)
	}
}
