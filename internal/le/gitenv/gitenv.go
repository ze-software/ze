// Design: docs/architecture/core-design.md -- the environment of a git query a deadline can kill
// Related: ../hookruntime/postwrite.go -- the post-edit lint run, whose revgrep runs git status and git diff

// Package gitenv answers the environment le gives a read-only git query that
// runs under a deadline.
//
// A deadline kills the git child with SIGKILL. `git status` holds
// `.git/index.lock` while it refreshes the index, and `git diff` takes it to
// write the refreshed stat data back, so a child killed inside that window
// leaves an empty lock no process holds, and every session's next git write
// fails until somebody removes it. The settings below make those queries skip
// the optional refresh, so a kill leaves nothing behind.
package gitenv

import (
	"strconv"
	"strings"
)

// optionalLocksOff is the documented switch: "If this Boolean environment
// variable is set to false, Git will complete any requested operation without
// performing any optional sub-operations that require taking a lock. For
// example, this will prevent git status from refreshing the index as a side
// effect." (git(1), GIT_OPTIONAL_LOCKS).
const optionalLocksOff = "GIT_OPTIONAL_LOCKS=0"

const configCountKey = "GIT_CONFIG_COUNT"

// ReadOnly answers base with GIT_OPTIONAL_LOCKS=0 appended, for a `git status`
// or another query whose index refresh git(1) calls optional. It does not
// change what the query prints. base is not modified.
func ReadOnly(base []string) []string {
	out := make([]string, 0, len(base)+1)
	out = append(out, base...)
	return append(out, optionalLocksOff)
}

// ReadOnlyPatch answers ReadOnly(base) plus diff.autoRefreshIndex=false, for a
// caller that also runs `git diff` and reads its patch text. GIT_OPTIONAL_LOCKS
// does not reach `git diff`: measured on git 2.56, a `git diff HEAD` over a
// stat-dirty file still rewrote the index with it set. With autoRefreshIndex
// off, diff takes no lock, and a file whose content is unchanged carries no
// hunk in the patch either way. It MUST NOT be used for `git diff --name-only`
// or `--name-status`: there a stat-only change is then listed as changed.
//
// The setting travels as GIT_CONFIG_KEY_<n>, after any pair base already
// declares, so a caller's own GIT_CONFIG_COUNT settings survive. A count git
// cannot parse is left as it is, and git then refuses the query with its own
// message rather than running it with the caller's settings dropped.
func ReadOnlyPatch(base []string) []string {
	out := ReadOnly(base)
	count := 0
	for _, entry := range base {
		value, found := strings.CutPrefix(entry, configCountKey+"=")
		if !found {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return out
		}
		if parsed < 0 {
			return out
		}
		count = parsed
	}
	index := strconv.Itoa(count)
	return append(out,
		"GIT_CONFIG_KEY_"+index+"=diff.autoRefreshIndex",
		"GIT_CONFIG_VALUE_"+index+"=false",
		configCountKey+"="+strconv.Itoa(count+1),
	)
}
