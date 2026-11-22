// Design: docs/architecture/api/commands.md -- exact decoded command path prefixes.
// Related: invocation_scan.go -- the shared operational word grammar.

package command

import (
	"slices"
	"strings"
)

// invocationMatchesPathPrefix compares registered path segments with complete
// decoded invocation words. Matching is case-sensitive; a metadata reader that
// owns case folding MUST apply it before calling. An empty path never matches.
func invocationMatchesPathPrefix(words []string, path string) bool {
	segments := strings.Fields(path)
	if len(segments) == 0 {
		return false
	}
	if len(words) < len(segments) {
		return false
	}
	return slices.Equal(words[:len(segments)], segments)
}

// invocationReachesCommandPath reports whether words reach the command
// registered at path, the question a per-command declaration lookup asks
// (commandRegistry.lookupPath). A command whose model declares a value inside
// its path, such as the peer selector of `show bgp peer <selector> rib`, is
// typed with that value between two path keywords, so the positional prefix
// match never reaches its declaration. Such a path is matched the way the
// dispatcher matches the command itself (MatchInvocationPath), over the
// definitions the model declares for it (argDefSource), so a selector or
// positional value is skipped exactly where dispatch binds one, on every route.
//
// A path the model declares no definition for, a process with no source, and
// a model that could not answer keep the positional match alone: those words
// carry no value the model places inside the path. Words the dispatcher
// cannot bind to the path do not reach it here either.
//
// The source is called with the caller's registry lock held, so it MUST NOT
// call the declaration registries; the YANG source reads its own map.
func invocationReachesCommandPath(words []string, path string) bool {
	if invocationMatchesPathPrefix(words, path) {
		return true
	}
	segments := strings.Fields(path)
	if len(segments) < 2 {
		return false
	}
	// A value inside the path makes the typed line longer than the path, and
	// no value comes before its first keyword: both are cheap refusals ahead
	// of the model lookup, which most registered paths would otherwise pay.
	if len(words) <= len(segments) {
		return false
	}
	if words[0] != segments[0] {
		return false
	}
	if !segmentsAppearInOrder(words, segments) {
		return false
	}
	if argDefSource == nil {
		return false
	}
	defs, err := argDefSource(ModelPath(path))
	if err != nil {
		return false
	}
	if len(defs) == 0 {
		return false
	}
	_, _, matched := MatchInvocationPath(words, path, defs)
	return matched
}

// segmentsAppearInOrder reports whether every keyword in segments appears in
// words in the same order, matching MatchInvocationPath's necessary condition
// without consulting the YANG argument source.
func segmentsAppearInOrder(words, segments []string) bool {
	matched := 0
	for _, word := range words {
		if word == segments[matched] {
			matched++
			if matched == len(segments) {
				return true
			}
		}
	}
	return false
}
