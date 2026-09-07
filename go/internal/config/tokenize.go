package config

import "strings"

// tokenizeLine splits a raw ssh_config line into a directive key/value pair,
// if it is one. A blank or comment-only line yields isDirective == false.
//
// Both "Key value" and "Key=value" forms are recognized. Key=value is
// chosen only when '=' appears strictly before the first whitespace in the
// non-comment portion of the line — so a value containing spaces (e.g.
// "ProxyCommand=ssh -W %h:%p bastion") still parses correctly, unlike a
// naive whole-line "no whitespace anywhere" check.
func tokenizeLine(raw string) (key, value string, isDirective bool) {
	code, _ := splitTrailingComment(raw)
	trimmed := strings.TrimSpace(code)
	if trimmed == "" {
		return "", "", false
	}

	ws := strings.IndexAny(trimmed, " \t")
	if eq := strings.IndexByte(trimmed, '='); eq > 0 && (ws == -1 || eq < ws) {
		k := trimmed[:eq]
		if isValidKeyToken(k) {
			return k, strings.TrimSpace(trimmed[eq+1:]), true
		}
	}

	if ws == -1 {
		// A bare key with no value (unusual, but don't drop the line).
		return trimmed, "", true
	}
	key = trimmed[:ws]
	value = strings.TrimSpace(trimmed[ws+1:])
	return key, value, true
}

// isValidKeyToken reports whether s looks like a directive name: starts
// with a letter, followed by letters/digits only (matches the shape of the
// Python regex this replaces).
func isValidKeyToken(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			// ok anywhere
		case r >= '0' && r <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}
