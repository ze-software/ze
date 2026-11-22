// Design: docs/contributing/ze-go-style.md -- tests for the commit-time error-wrap gate
//
// VALIDATES: judgeChange is registered in changecheck, refuses newly added
// fmt.Errorf wraps that repeat a segment already carried by the wrapped error,
// and skips untouched legacy wraps and non-repeating wraps.
// PREVENTS: duplicate error prefixes surviving commit until post-push CI.
package archerrorwrap

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

func TestErrorWrapChangeCheckIsRegistered(t *testing.T) {
	t.Parallel()
	registered := changecheck.Registered()
	if !slices.ContainsFunc(registered, func(c changecheck.Check) bool { return c.Name == area+" check" }) {
		t.Fatalf("%q not registered in changecheck: %+v", area+" check", registered)
	}
}

func TestErrorWrapJudgeChangeRefusesAddedAndSkipsLegacy(t *testing.T) {
	t.Parallel()
	legacy := `package demo

import (
	"errors"
	"fmt"
)

func leaf() error {
	return errors.New("parse config: bad token")
}

func oldWrap() error {
	err := leaf()
	return fmt.Errorf("parse config: %w", err)
}
`
	addedClean := legacy + `
func cleanWrap() error {
	err := leaf()
	return fmt.Errorf("reload: %w", err)
}
`
	addedBad := legacy + `
func badWrap() error {
	err := leaf()
	return fmt.Errorf("reload: parse config: %w", err)
}
`

	makeChange := func(before, after string) changecheck.Change {
		return changecheck.Change{
			Paths: []string{"internal/demo/demo.go"},
			Before: func(rel string) (string, bool, error) {
				return before, before != "", nil
			},
			After: func(rel string) (string, bool, error) {
				return after, true, nil
			},
		}
	}

	if got, err := judgeChange(makeChange(legacy, addedClean)); err != nil || got != "" {
		t.Fatalf("clean addition got (%q, %v), want empty", got, err)
	}
	got, err := judgeChange(makeChange(legacy, addedBad))
	if err != nil {
		t.Fatalf("bad addition err = %v", err)
	}
	if !strings.Contains(got, "badWrap") || !strings.Contains(got, "parse config") {
		t.Fatalf("bad addition refusal = %q, want finding naming badWrap and parse config", got)
	}
	if strings.Contains(got, "(oldWrap)") {
		t.Fatalf("bad addition refusal flagged untouched legacy wrap:\n%s", got)
	}
}
