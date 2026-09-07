package ui

import (
	"testing"

	"gitlab.com/gto_gary/sshtui/internal/config"
)

func params(pairs ...string) map[string]config.DirectiveValue {
	m := map[string]config.DirectiveValue{}
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i]] = config.One(pairs[i+1])
	}
	return m
}

func TestMatchesFilterEmpty(t *testing.T) {
	if !MatchesFilter("", "web", params(), "unknown") {
		t.Error("empty filter should match everything")
	}
}

func TestMatchesFilterExtra(t *testing.T) {
	withExtra := params("hostname", "web.example.com", "proxyjump", "bastion")
	withoutExtra := params("hostname", "web.example.com", "user", "deploy")

	cases := []struct {
		filter string
		params map[string]config.DirectiveValue
		want   bool
	}{
		{"extra:yes", withExtra, true},
		{"extra:y", withExtra, true},
		{"extra:true", withExtra, true},
		{"extra:no", withExtra, false},
		{"extra:no", withoutExtra, true},
		{"extra:yes", withoutExtra, false},
		{"extra:bogus", withExtra, false},
	}
	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			if got := MatchesFilter(tc.filter, "web", tc.params, "unknown"); got != tc.want {
				t.Errorf("MatchesFilter(%q) = %v, want %v", tc.filter, got, tc.want)
			}
		})
	}
}

func TestMatchesFilterStatus(t *testing.T) {
	cases := []struct {
		filter string
		status string
		want   bool
	}{
		{"status:up", "up", true},
		{"status:reachable", "up", true},
		{"status:down", "down", true},
		{"status:unreachable", "down", true},
		{"status:unknown", "unknown", true},
		{"status:checking", "unknown", true},
		{"status:up", "down", false},
		{"status:bogus", "up", false},
	}
	for _, tc := range cases {
		t.Run(tc.filter+"/"+tc.status, func(t *testing.T) {
			if got := MatchesFilter(tc.filter, "web", params(), tc.status); got != tc.want {
				t.Errorf("MatchesFilter(%q, status=%q) = %v, want %v", tc.filter, tc.status, got, tc.want)
			}
		})
	}
}

func TestMatchesFilterPlainText(t *testing.T) {
	p := params("hostname", "web.example.com", "user", "deploy")

	cases := []struct {
		filter string
		want   bool
	}{
		{"web", true},         // matches alias
		{"example.com", true}, // matches hostname
		{"deploy", true},      // matches user
		{"22", true},          // matches default port when unset
		{"nomatch", false},
	}
	for _, tc := range cases {
		t.Run(tc.filter, func(t *testing.T) {
			if got := MatchesFilter(tc.filter, "web", p, "unknown"); got != tc.want {
				t.Errorf("MatchesFilter(%q) = %v, want %v", tc.filter, got, tc.want)
			}
		})
	}
}

func TestMatchesFilterPlainTextUsesActualPort(t *testing.T) {
	p := params("port", "2222")
	// "22" is a substring of "2222", so it still matches — this is plain
	// substring matching, not exact-port matching, same as the Python
	// original.
	if !MatchesFilter("2222", "web", p, "unknown") {
		t.Error(`filter "2222" should match a host whose actual port is 2222`)
	}
	if MatchesFilter("3333", "web", p, "unknown") {
		t.Error(`filter "3333" should not match a host whose actual port is 2222`)
	}
}
