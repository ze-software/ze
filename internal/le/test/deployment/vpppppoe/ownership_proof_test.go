// Design: docs/labs/pppoe-interop.md -- native ownership proof after exact binding success.
// Related: proof.go -- isolated baseline instance and native CLI verdicts.
package testvpppppoe

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// A native CLI assertion can exit zero. Check the exact complete ownership
// result, its failure evidence, and its bounded invocation independently.
func TestMechanismOwnershipProofRequiresCompletedExactSuccess(t *testing.T) {
	marker := "PPPoE ownership: 55 cases passed"
	transport := errors.New("ownership Docker transport disconnected")
	for _, test := range []struct {
		name, stdout, stderr string
		exit                 int
		cause                error
		proven               bool
	}{
		{name: "all cases", stdout: marker + "\n", proven: true},
		{name: "missing CLI", stdout: "unknown input `test pppoe ownership'\n"},
		{name: "foreign IPv6 assertion", stdout: "foreign IPv6 session exposed an IPv4 table\n"},
		{name: "old cases only", stdout: "PPPoE ownership: 30 cases passed\n"},
		{name: "empty answer"},
		{name: "longer answer", stdout: marker + " unexpectedly\n"},
		{name: "contradictory answer", stdout: "foreign delete succeeded\n" + marker + "\n"},
		{name: "stderr", stdout: marker + "\n", stderr: "ownership cleanup failed\n"},
		{name: "transport failure", stdout: marker + "\n", cause: transport},
		{name: "process failure", stdout: marker + "\n", exit: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				calls++
				want := []string{"docker", "exec", vppPeer, "vppctl", "-s", vppCLIPath, "test", "pppoe", "ownership"}
				if !slices.Equal(command.Arguments, want) {
					t.Fatalf("ownership command=%q, want %q", command.Arguments, want)
				}
				deadline, bounded := ctx.Deadline()
				if !bounded {
					t.Fatal("ownership CLI has no deadline")
				}
				if time.Until(deadline) > controlCommandBudget {
					t.Fatal("ownership CLI exceeds command budget")
				}
				return interoplab.ProcessResult{Stdout: test.stdout, Stderr: test.stderr, ExitCode: test.exit}, test.cause
			}))
			err := checkOwnershipOracle(t.Context(), mechanismProbeLab{docker: docker})
			if calls != 1 {
				t.Fatalf("ownership calls=%d", calls)
			}
			if test.proven {
				if err != nil {
					t.Fatalf("complete native ownership result refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("incomplete or failed ownership result passed")
			}
			if test.cause != nil {
				if !errors.Is(err, test.cause) {
					t.Fatalf("transport cause lost: %v", err)
				}
			}
			for _, evidence := range []string{test.stdout, test.stderr} {
				if evidence == "" {
					continue
				}
				if !strings.Contains(err.Error(), strings.TrimSpace(evidence)) {
					t.Fatalf("ownership evidence %q lost: %v", evidence, err)
				}
			}
		})
	}
}

// The isolated baseline must finish binding first, then observe ownership;
// a binding refusal never reaches the ownership producer.
func TestMechanismBindingProofRequiresOwnershipStage(t *testing.T) {
	for _, failure := range []string{"none", "bindings", "ownership"} {
		t.Run(failure, func(t *testing.T) {
			var stages []string
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				args := command.Arguments
				if slices.Equal(args, []string{"docker", "exec", vppPeer, "cat", "/usr/share/doc/vpp/proof-variant"}) {
					stages = append(stages, "variant")
					return interoplab.ProcessResult{Stdout: "baseline\n"}, nil
				}
				if slices.Equal(args, []string{"docker", "exec", vppPeer, "vppctl", "-s", vppCLIPath, "test", "pppoe", "cp-binding"}) {
					stages = append(stages, "bindings")
					if failure == "bindings" {
						return interoplab.ProcessResult{Stdout: "unmapped deletion was accepted\n"}, nil
					}
					return interoplab.ProcessResult{Stdout: "PPPoE cp binding: 10 cases passed\n"}, nil
				}
				if !slices.Equal(args, []string{"docker", "exec", vppPeer, "vppctl", "-s", vppCLIPath, "test", "pppoe", "ownership"}) {
					t.Fatalf("unexpected command %q", args)
				}
				stages = append(stages, "ownership")
				if failure == "ownership" {
					return interoplab.ProcessResult{Stdout: "foreign IPv6 session exposed an IPv4 table\n"}, nil
				}
				return interoplab.ProcessResult{Stdout: "PPPoE ownership: 55 cases passed\n"}, nil
			}))
			err := checkBindingProof(t.Context(), &interoplab.CheckContext{Lab: mechanismProbeLab{docker: docker}})
			want := []string{"variant", "bindings", "ownership"}
			if failure == "bindings" {
				want = []string{"variant", "bindings"}
			}
			if !slices.Equal(stages, want) {
				t.Fatalf("stages=%v,want %v", stages, want)
			}
			if failure == "none" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatalf("%s producer refusal was ignored", failure)
			}
		})
	}
}
