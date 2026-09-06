// Design: docs/architecture/config/exabgp-syntax.md -- ExaBGP api process selection
// Overview: migrate.go -- neighbor and process migration

package migration

import (
	"fmt"
	"regexp"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// processesMatchField is the api block leaf that selects processes by pattern
// rather than by name (src/exabgp/configuration/neighbor/api.py, ParseAPI).
const processesMatchField = "processes-match"

// matchProcessNames answers the declared ExaBGP process names that the patterns
// of one api block select.
//
// ExaBGP matches with `re.match`, which anchors the pattern at the START of the
// name and leaves the end free (src/exabgp/configuration/configuration.py,
// _link). So `^add` and `add` both select `add-remove`, `remove` selects
// nothing, and the match is case-sensitive. `\A(?:...)` is that anchoring
// written in Go's syntax, and the group keeps an alternation inside the anchor.
//
// A pattern set that selects nothing is refused, which is what ExaBGP does with
// it ("Any process match regex ... for neighbor ...", configuration.py
// validate). Migrating it instead would write a peer with no attached process,
// and every route the operator's script then announces is refused at dispatch
// with "this peer does not attach that process with the send permission it
// needs" (internal/component/bgp/reactor/send_permission.go). The config would
// look migrated and the session would carry nothing.
func matchProcessNames(apiName string, patterns []string, declared []ExternalProcess) ([]string, error) {
	anchored := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		var tb textbuf.Buffer
		expression, err := regexp.Compile(tb.Str(`\A(?:`).Str(pattern).Str(`)`).String())
		if err != nil {
			return nil, fmt.Errorf("api %s: processes-match %q is not a regular expression: %w", apiName, pattern, err)
		}
		anchored = append(anchored, expression)
	}

	var matched []string
	for _, process := range declared {
		for _, expression := range anchored {
			if expression.MatchString(process.Name) {
				matched = append(matched, process.Name)
				break
			}
		}
	}

	if len(matched) == 0 {
		return nil, fmt.Errorf("api %s: processes-match %v selects none of the processes this config runs %v",
			apiName, patterns, declaredProcessNames(declared))
	}
	return matched, nil
}

// declaredProcessNames answers the names alone, for an error message that shows
// the operator what the patterns were matched against.
func declaredProcessNames(declared []ExternalProcess) []string {
	names := make([]string, 0, len(declared))
	for _, process := range declared {
		names = append(names, process.Name)
	}
	return names
}
