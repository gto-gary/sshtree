package ui

import (
	"strings"

	"github.com/gto-gary/sshtree/internal/config"
)

// coreFields are the directives given dedicated, always-shown UI treatment;
// everything else counts as "extra" for the extra:yes/no filter.
var coreFields = map[string]bool{"hostname": true, "user": true, "port": true}

// flattenValue joins a possibly multi-valued directive into one display
// string, matching config.py's flatten_value: a single value round-trips as
// itself, multiple values join with ", ", and a missing directive is "".
func flattenValue(dv config.DirectiveValue) string {
	return strings.Join(dv.Values, ", ")
}

func hasExtraDirective(params map[string]config.DirectiveValue) bool {
	for k := range params {
		if !coreFields[k] {
			return true
		}
	}
	return false
}

// MatchesFilter reports whether a host matches an already-lowercased,
// trimmed search filter, mirroring host_list.py's _matches_filter:
//
//   - "extra:yes|y|true" / "extra:no|n|false" — whether the host has any
//     directive beyond Hostname/User/Port.
//   - "status:up|reachable" / "status:down|unreachable" /
//     "status:unknown|checking" — reachability state.
//   - anything else — a plain substring match against
//     "alias hostname user port" (port defaulting to "22" if unset).
//
// An empty filter matches everything.
func MatchesFilter(filterText, alias string, params map[string]config.DirectiveValue, status string) bool {
	if filterText == "" {
		return true
	}

	if rest, ok := strings.CutPrefix(filterText, "extra:"); ok {
		hasExtra := hasExtraDirective(params)
		switch rest {
		case "yes", "y", "true":
			return hasExtra
		case "no", "n", "false":
			return !hasExtra
		default:
			return false
		}
	}

	if rest, ok := strings.CutPrefix(filterText, "status:"); ok {
		switch rest {
		case "down", "unreachable":
			return status == "down"
		case "up", "reachable":
			return status == "up"
		case "unknown", "checking":
			return status == "unknown"
		default:
			return false
		}
	}

	port := flattenValue(params["port"])
	if port == "" {
		port = "22"
	}
	haystack := strings.ToLower(strings.Join([]string{
		alias,
		flattenValue(params["hostname"]),
		flattenValue(params["user"]),
		port,
	}, " "))
	return strings.Contains(haystack, filterText)
}
