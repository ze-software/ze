// Design: docs/contributing/gh-pages.md -- every fixture must prove readiness.
// Related: scenarios_showcase.go -- startup and readiness boundaries.
package siteterminaldemo

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// PREVENTS: starting an unobservable fixture and reporting an unchecked lab ready.
func TestShowcaseRefusesUnobservableFixture(t *testing.T) {
	state := t.TempDir()
	pidPath := filepath.Join(state, "owned.pid")
	fixture := labFixture{
		name: "unobserved",
		start: func(string, []string) ([]int, error) {
			return nil, nil
		},
	}
	if err := startFixtures(state, pidPath, nil, []labFixture{fixture}); err == nil {
		t.Fatal("fixture without any readiness observation was accepted")
	}
	for _, path := range []string{pidPath, filepath.Join(state, "unobserved.pid")} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unobservable fixture created ownership state %s: %v", path, err)
		}
	}
}

// PREVENTS: an empty expected string accepting a command that proves nothing.
func TestShowcaseRefusesEmptyReadinessTarget(t *testing.T) {
	ready := labReadiness{kind: readinessAnswer, argv: []string{"true"}}
	if err := awaitReadiness("silent", ready); err == nil {
		t.Fatal("successful command with no readiness target was accepted")
	}
}

// PREVENTS: malformed command observations panicking instead of refusing readiness.
func TestShowcaseRefusesMissingReadinessCommand(t *testing.T) {
	ready := labReadiness{kind: readinessAnswer, target: "ready"}
	if err := awaitReadiness("unrunnable", ready); err == nil {
		t.Fatal("readiness target without a command was accepted")
	}
}
