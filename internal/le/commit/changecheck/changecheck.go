// Design: docs/contributing/committing.md -- the change-set checks `./le commit create` runs
//
// Package changecheck is the seam a change-set check registers into so that
// `./le commit create` runs it over the commit it prepares. The commit code
// names no check: a check package calls Register from its own init(), and
// the composition root (internal/le/register.go) links it. This package is a
// leaf, so a check never imports internal/le/commit to register.
//
// A check judges what the commit ADDS, never what the tree already holds: a
// red never holds a commit, while a refusal of the commit's own additions
// stands (ai/META-DECISIONS.md, "Ze is pre-release").
package changecheck

import (
	"errors"
	"slices"
	"strings"
	"sync"
)

// Side reads one path's text on one side of the commit: present false is a
// side that does not hold the path, and a non-nil error is a read that
// failed, which a check MUST answer as an error rather than an absent file.
type Side func(path string) (text string, present bool, err error)

// Change is the commit being prepared, as a check reads it.
type Change struct {
	// Root is the checkout. A check that needs context outside the carried
	// files (a registration in another file) reads it from the working tree.
	Root string
	// Paths are the root-relative, slash-separated paths the commit carries
	// (added or modified); Removed are the paths it deletes.
	Paths   []string
	Removed []string
	// Before reads the commit's parent: HEAD, or the tree an earlier block of
	// the same commit script leaves.
	Before Side
	// After reads what the commit carries: the staged selection, honoring a
	// `hunks` selection, never the whole working-tree file.
	After Side
}

// Check is one registered change-set check.
type Check struct {
	// Name is the check's command, as an operator types it
	// (`arch operator-error`), so a refusal names what refused.
	Name string
	// Judge answers the refusal text for what change adds, or "" to accept.
	// The text names what, why and the fix (ai/rules/user-facing-errors.md).
	// An error is a run that could not judge, never a refusal.
	Judge func(change Change) (string, error)
}

var (
	mu     sync.Mutex
	checks []Check
)

// Register adds a check. It MUST be called from init(). A check with no name
// or no judge, or a second check of one name, is a Ze defect and panics.
func Register(check Check) {
	if check.Name == "" {
		panic("BUG: changecheck.Register: a check with no name")
	}
	if check.Judge == nil {
		panic("BUG: changecheck.Register: check " + check.Name + " has no Judge")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, registered := range checks {
		if registered.Name == check.Name {
			panic("BUG: changecheck.Register: check " + check.Name + " registered twice")
		}
	}
	checks = append(checks, check)
}

// Registered answers every registered check, sorted by name. Safe for
// concurrent use.
func Registered() []Check {
	mu.Lock()
	defer mu.Unlock()
	answer := slices.Clone(checks)
	slices.SortFunc(answer, func(a, b Check) int { return strings.Compare(a.Name, b.Name) })
	return answer
}

// Refusal is one check's refusal of a change.
type Refusal struct {
	Check string
	Text  string
}

// ErrCouldNotJudge wraps a check that could not judge the change.
var ErrCouldNotJudge = errors.New("a change-set check could not judge the commit")

// Run judges change with every check in checks, in order, and answers the
// refusals. The first check that cannot judge stops the run with an error
// naming it: a check that did not run is not a check that accepted.
func Run(change Change, checks []Check) ([]Refusal, error) {
	var refusals []Refusal
	for _, check := range checks {
		text, err := check.Judge(change)
		if err != nil {
			return nil, errors.Join(ErrCouldNotJudge, errors.New(check.Name+": "+err.Error()))
		}
		if text == "" {
			continue
		}
		refusals = append(refusals, Refusal{Check: check.Name, Text: text})
	}
	return refusals, nil
}

// Snapshot answers the registered checks and a function that restores them,
// for a test that registers a fixture check (t.Cleanup(restore)).
func Snapshot() func() {
	mu.Lock()
	saved := slices.Clone(checks)
	mu.Unlock()
	return func() {
		mu.Lock()
		checks = saved
		mu.Unlock()
	}
}
