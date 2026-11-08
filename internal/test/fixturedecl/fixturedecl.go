// Design: docs/functional-tests.md -- a fixture that stops the daemon it runs inside
// Related: internal/test/fixture/fixture.go -- Register records the declaration here
// Related: internal/test/runner/self_stop_fixture.go -- recordStartsSelfStoppingFixture reads it
//
// Package fixturedecl holds what a compiled fixture declares about itself that
// the functional runner must know before the fixture runs.
//
// The runner cannot import internal/test/fixture: that package imports the
// runner. So the declaration lives in this leaf, which both import. Register in
// internal/test/fixture writes it from each fixture's init(), and the runner
// reads it in the same process, because `le` links both packages. A process that
// links the runner and no fixture holds no declaration, and answers that no
// fixture stops the daemon, which is true of that process: it can start none.

package fixturedecl

import (
	"path/filepath"
	"sync"
)

// The words that start a compiled fixture: `le test fixture <name> [args...]`.
const (
	commandWord   = "le"
	commandTest   = "test"
	commandAction = "fixture"
)

// Command is how a command line starts a compiled fixture, followed by its name.
const Command = commandWord + " " + commandTest + " " + commandAction

var (
	stoppersMu sync.RWMutex
	stoppers   = make(map[string]struct{})
)

// DeclareStopsDaemon records that the named fixture ends the daemon it runs
// inside, by dispatching `request shutdown` itself as fixture.Observe does.
// Safe for concurrent use. A duplicate or empty name is a defect.
func DeclareStopsDaemon(fixture string) {
	if fixture == "" {
		panic("BUG: fixturedecl.DeclareStopsDaemon: empty fixture name")
	}
	stoppersMu.Lock()
	defer stoppersMu.Unlock()
	if _, exists := stoppers[fixture]; exists {
		panic("BUG: fixturedecl.DeclareStopsDaemon: duplicate fixture " + fixture)
	}
	stoppers[fixture] = struct{}{}
}

// StopsDaemon answers whether the named fixture declared that it ends the daemon
// it runs inside. Safe for concurrent use.
func StopsDaemon(fixture string) bool {
	stoppersMu.RLock()
	defer stoppersMu.RUnlock()
	_, stops := stoppers[fixture]
	return stops
}

// FixtureOf answers the fixture name a split command line starts, and whether it
// starts one. The launcher word is compared by its base name, so a command the
// runner resolved to bin/le still names its fixture.
func FixtureOf(argv []string) (string, bool) {
	if len(argv) < 4 {
		return "", false
	}
	if filepath.Base(argv[0]) != commandWord {
		return "", false
	}
	if argv[1] != commandTest {
		return "", false
	}
	if argv[2] != commandAction {
		return "", false
	}
	return argv[3], true
}
