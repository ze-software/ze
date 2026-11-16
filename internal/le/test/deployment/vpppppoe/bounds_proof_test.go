// Design: docs/labs/pppoe-interop.md -- malformed wire frames must change their own counter.
// Related: check.go -- empty session observations enclose the actual receive-path proof.
package testvpppppoe

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// Feed the pinned producer's default table shape, including omitted zero rows
// and independent worker rows, so old text and other nodes cannot prove delivery.
func TestMechanismBoundsCounterReadsPinnedOutput(t *testing.T) {
	for _, test := range []struct {
		name, output string
		count        uint64
		valid        bool
	}{
		{name: "zero rows omitted", output: "Count Node Reason Severity\n", valid: true},
		{name: "exact node", output: "Count Node Reason Severity\n         9            pppoe-input       malformed PPPoE frame length     error\n", count: 9, valid: true},
		{name: "workers summed", output: "Count Node Reason Severity\n2 pppoe-input malformed PPPoE frame length error\n3 pppoe-input malformed PPPoE frame length error\n", count: 5, valid: true},
		{name: "another node", output: "Count Node Reason Severity\n99 pppoe-cp-dispatch malformed PPPoE frame length error\n", valid: true},
		{name: "another reason", output: "Count Node Reason Severity\n99 pppoe-input no such session error\n", valid: true},
		{name: "missing header", output: "9 pppoe-input malformed PPPoE frame length error\n"},
		{name: "partial row", output: "Count Node Reason Severity\n9 pppoe-input malformed PPPoE frame length\n"},
		{name: "nonnumeric count", output: "Count Node Reason Severity\nmany pppoe-input malformed PPPoE frame length error\n"},
		{name: "count overflow", output: "Count Node Reason Severity\n18446744073709551616 pppoe-input malformed PPPoE frame length error\n"},
		{name: "sum overflow", output: "Count Node Reason Severity\n18446744073709551615 pppoe-input malformed PPPoE frame length error\n1 pppoe-input malformed PPPoE frame length error\n"},
		{name: "refusal", output: "unknown input `show errors'\n"},
		{name: "contradictory refusal", output: "Count Node Reason Severity\n2 pppoe-input malformed PPPoE frame length error\nshow errors: refused\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			count, err := boundsMalformedCount(test.output)
			if test.valid {
				if err != nil {
					t.Fatal(err)
				}
				if count != test.count {
					t.Fatalf("count %d, want %d", count, test.count)
				}
				return
			}
			if err == nil {
				t.Fatal("unreadable counter output counted as evidence")
			}
		})
	}
}

// Exercise the real wire checker through Docker's process seam. Its command
// order must surround injection with empty tables and compare fresh counters
// only after the CP TAP barrier; every refusal stops the proof at that point.
func TestMechanismBoundsWireRequiresNewDropsAndEmptySessions(t *testing.T) {
	const empty = "No pppoe sessions configured...\n"
	const header = "Count Node Reason Severity\n"
	const old = header + "7 pppoe-input malformed PPPoE frame length error\n"
	const fresh = header + "9 pppoe-input malformed PPPoE frame length error\n"
	transport := errors.New("Docker disconnected")
	for _, test := range []struct {
		name, beforeSession, afterSession, before, after, stderr string
		cause                                                    error
		exit                                                     int
		proven                                                   bool
	}{
		{name: "fresh drops", beforeSession: empty, afterSession: empty, before: old, after: fresh, proven: true},
		{name: "omitted zero baseline", beforeSession: empty, afterSession: empty, before: header, after: header + "2 pppoe-input malformed PPPoE frame length error\n", proven: true},
		{name: "old matching count", beforeSession: empty, afterSession: empty, before: old, after: old},
		{name: "one drop", beforeSession: empty, afterSession: empty, before: old, after: header + "8 pppoe-input malformed PPPoE frame length error\n"},
		{name: "reset counter", beforeSession: empty, afterSession: empty, before: old, after: header + "2 pppoe-input malformed PPPoE frame length error\n"},
		{name: "another node", beforeSession: empty, afterSession: empty, before: old, after: old + "2 pppoe-cp-dispatch malformed PPPoE frame length error\n"},
		{name: "existing session", beforeSession: "[0] session-id 17\n", afterSession: empty, before: old, after: fresh},
		{name: "injected session", beforeSession: empty, afterSession: "[0] session-id 18\n", before: old, after: fresh},
		{name: "session refusal", beforeSession: "unknown input `show pppoe session'\n", afterSession: empty, before: old, after: fresh},
		{name: "empty session response", afterSession: empty, before: old, after: fresh},
		{name: "contradictory empty table", beforeSession: empty, afterSession: empty + "[0] session-id 18\n", before: old, after: fresh},
		{name: "partial counter", beforeSession: empty, afterSession: empty, before: old, after: header + "9 pppoe-input malformed PPPoE frame length\n"},
		{name: "stderr refusal", beforeSession: empty, afterSession: empty, before: old, after: fresh, stderr: "readback refused\n"},
		{name: "transport refusal", beforeSession: empty, afterSession: empty, before: old, after: fresh, cause: transport},
		{name: "process refusal", beforeSession: empty, afterSession: empty, before: old, after: fresh, exit: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			sessions, counters := 0, 0
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				arguments := command.Arguments
				if slices.Contains(arguments, "session") {
					sessions++
					calls = append(calls, "sessions")
					if sessions == 1 {
						return interoplab.ProcessResult{Stdout: test.beforeSession, Stderr: test.stderr, ExitCode: test.exit}, test.cause
					}
					return interoplab.ProcessResult{Stdout: test.afterSession}, nil
				}
				if slices.Contains(arguments, "errors") {
					counters++
					calls = append(calls, "counters")
					if counters == 1 {
						return interoplab.ProcessResult{Stdout: test.before}, nil
					}
					return interoplab.ProcessResult{Stdout: test.after}, nil
				}
				if slices.Contains(arguments, "bounds") {
					calls = append(calls, "inject")
					return interoplab.ProcessResult{}, nil
				}
				if slices.Contains(arguments, "/evidence/control.error") {
					return interoplab.ProcessResult{ExitCode: 1, Stderr: "No such file or directory"}, nil
				}
				if slices.Contains(arguments, "/evidence/control.state") {
					calls = append(calls, "barrier")
					return interoplab.ProcessResult{Stdout: `{"bounds-barrier":true}`}, nil
				}
				t.Fatalf("wire proof ran an unexpected command: %q", arguments)
				return interoplab.ProcessResult{}, errors.New("unexpected command")
			}))
			err := checkBoundsWire(t.Context(), mechanismProbeLab{docker: docker}, "eth0", "02:00:00:00:00:01")
			if test.proven {
				if err != nil {
					t.Fatal(err)
				}
				wanted := []string{"sessions", "counters", "inject", "barrier", "sessions", "counters"}
				if !slices.Equal(calls, wanted) {
					t.Fatalf("wire proof calls %q, want %q", calls, wanted)
				}
				return
			}
			if err == nil {
				t.Fatal("stale, unrelated or refused observations counted as wire proof")
			}
			if test.cause != nil {
				if !errors.Is(err, test.cause) {
					t.Fatalf("transport refusal lost: %v", err)
				}
			}
			if test.stderr != "" {
				if !strings.Contains(err.Error(), strings.TrimSpace(test.stderr)) {
					t.Fatalf("stderr refusal lost: %v", err)
				}
			}
		})
	}
}

// Drive the actual optional CLI readback through Docker's process seam: a
// successful exit alone cannot substitute for its completed four-path answer.
func TestMechanismReceiveBoundsRequiresCompletedExactSuccess(t *testing.T) {
	const marker = "PPPoE receive bounds: 4 paths passed"
	transport := errors.New("Docker disconnected")
	for _, test := range []struct {
		name, stdout, stderr string
		cause                error
		exit                 int
		proven               bool
	}{
		{name: "completed", stdout: marker + "\n", proven: true},
		{name: "unknown command", stdout: "unknown input `test pppoe receive-bounds'\n"},
		{name: "partial", stdout: "PPPoE receive bounds: 3 paths passed\n"},
		{name: "suffix", stdout: marker + " unexpectedly\n"},
		{name: "contradictory", stdout: "receive path failed\n" + marker + "\n"},
		{name: "empty"},
		{name: "stderr refusal", stdout: marker + "\n", stderr: "receive cleanup failed\n"},
		{name: "transport failure", stdout: marker + "\n", cause: transport},
		{name: "process failure", stdout: marker + "\n", exit: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				calls++
				wanted := []string{"docker", "exec", vppPeer, vppctlCommand, "-s", vppCLIPath, testCommand, "pppoe", "receive-bounds"}
				if !slices.Equal(command.Arguments, wanted) {
					t.Fatalf("receive proof ran %q, want %q", command.Arguments, wanted)
				}
				return interoplab.ProcessResult{Stdout: test.stdout, Stderr: test.stderr, ExitCode: test.exit}, test.cause
			}))
			err := checkReceiveBounds(t.Context(), mechanismProbeLab{docker: docker})
			if calls != 1 {
				t.Fatalf("receive proof ran %d commands, want one completed readback", calls)
			}
			if test.proven {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("incomplete, contradictory or failed receive proof counted as success")
			}
			if test.cause != nil {
				if !errors.Is(err, test.cause) {
					t.Fatalf("transport failure lost: %v", err)
				}
			}
		})
	}
}
