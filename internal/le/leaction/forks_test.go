// This package derives the third gate state. These tests keep that derivation
// honest.
//
// internal/le/parity counted a gate as ported when a registered command claimed the
// gate. It never asked whether that command did the work. An area now publishes
// the argv that each action starts. This file pins the two resulting answers:
// which gates are forked and what the listing shows.

package leaction

import (
	"slices"
	"testing"
)

// forkFixture is one area with all three action types. Go can do the work, a
// compiled repository tool can do it, or a script can do it.
func forkFixture() Area {
	return New("proof",
		Action{Gate: "ze-proof-in-go", Why: "the work is this package's", Answer: func() (any, int) { return nil, 0 }},
		Action{
			Gate: "ze-proof-toolchain", Why: "the work is the Go toolchain's",
			Forks:  []string{"go", "test", "-count=1", "./internal/..."},
			Answer: func() (any, int) { return nil, 0 },
		},
		Action{
			Gate: "ze-proof-driver", Why: "the work is a Python driver's",
			Forks:  []string{"python3", "scripts/evidence/effective-vpp.py"},
			Answer: func() (any, int) { return nil, 0 },
		},
		Action{
			Gate: "ze-proof-lab", Why: "the work is a lab runner's",
			Forks:  []string{"sudo", "VERBOSE=", "python3", "test/stress/run.py", "04-bulk"},
			Answer: func() (any, int) { return nil, 0 },
		},
		Action{
			Gate: "ze-proof-shell", Why: "the work is a shell script's",
			Forks:  []string{"bash", "scripts/evidence/effective-verify.sh"},
			Answer: func() (any, int) { return nil, 0 },
		},
	)
}

// TestForkedGatesNamesOnlyTheActionsThatStartAScript tests the derivation. An
// action that runs `go test` does its own work with the toolchain. An action
// that runs python3 or bash over a repository file is a port that remains.
func TestForkedGatesNamesOnlyTheActionsThatStartAScript(t *testing.T) {
	area := forkFixture()

	want := []string{"ze-proof-driver", "ze-proof-lab", "ze-proof-shell"}
	if got := area.ForkedGates(); !slices.Equal(got, want) {
		t.Errorf("ForkedGates answered %v, want %v", got, want)
	}
	if got := len(area.Gates()); got != 5 {
		t.Errorf("Gates answered %d targets, want all 5: a forked gate is still served", got)
	}
}

// TestTheScriptIsFoundWhereverItSitsInTheArgv is the reason every argument is
// read rather than the first one. A driver is named in the second position, a
// lab runner behind sudo and two variable assignments in the fifth.
func TestTheScriptIsFoundWhereverItSitsInTheArgv(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want bool
	}{
		{"nothing to start", nil, false},
		{"the toolchain", []string{"go", "build", "-tags", "ze_le", "./cmd/ze"}, false},
		{"a compiled runner", []string{"timeout", "600", "bin/ze-test", "run"}, false},
		{"a driver in second position", []string{"python3", "scripts/evidence/effective-vpp.py"}, true},
		{"a lab runner in fifth", []string{"sudo", "VERBOSE=", "T=", "python3", "test/stress/run.py"}, true},
		{"a shell script", []string{"bash", "scripts/dev/session-scratch.sh"}, true},
		{"a path that merely mentions one", []string{"go", "test", "./scripts/dev/"}, false},
	}
	for _, tc := range cases {
		if got := forksAScript(tc.argv); got != tc.want {
			t.Errorf("%s: forksAScript(%v) = %v, want %v", tc.name, tc.argv, got, tc.want)
		}
	}
}

// TestTheListingPublishesOnlyTheWorkThatHasNotMoved tests the operator's view.
// The listing shows the script that an action still starts. It shows nothing
// for an action that runs the toolchain. `go test -tags integration ...` is the
// action doing its own work. Listing it would describe that work as unported.
func TestTheListingPublishesOnlyTheWorkThatHasNotMoved(t *testing.T) {
	forks := map[string][]string{}
	for _, row := range forkFixture().Actions().Actions {
		forks[row.Gate] = row.Forks
	}

	if got := forks["ze-proof-driver"]; !slices.Contains(got, "scripts/evidence/effective-vpp.py") {
		t.Errorf("the driver row published %v, and it starts a Python script", got)
	}
	for _, gate := range []string{"ze-proof-in-go", "ze-proof-toolchain"} {
		if got := forks[gate]; len(got) != 0 {
			t.Errorf("%s does its own work and the listing published a fork: %v", gate, got)
		}
	}
}
