// Design: docs/labs/pppoe-interop.md -- normal VPP control-binding API regression.
// Related: proof.go -- the isolated single-peer proof and its CLI verdict.
package testvpppppoe

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	buildvpp "github.com/ze-software/ze/internal/le/build/vpp"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// The binding regression must be reachable as its own admitted native action,
// so the ordinary API failure can be observed before repairing its producer.
func TestMechanismBindingsActionRegisters(t *testing.T) {
	for _, action := range actions.Actions().Actions {
		if action.Verb != "bindings" {
			continue
		}
		if !action.Heavy {
			t.Fatal("the Docker binding proof bypasses admission")
		}
		if !action.Writes {
			t.Fatal("the binding proof does not declare its staged artifacts")
		}
		if action.Does != "run" {
			t.Fatalf("bindings performs %q, want run", action.Does)
		}
		if len(action.Parameters) != 0 {
			t.Fatal("the normal binding proof must use the registered baseline image")
		}
		return
	}
	t.Fatal("the native binding regression action is not registered")
}

// Pinned vppctl exits zero for CLI assertions. Only its complete, exact
// success answer proves the normal producer passed all ten API cases.
func TestMechanismBindingProofRequiresCompletedExactSuccess(t *testing.T) {
	marker := "PPPoE cp binding: 10 cases passed"
	transport := errors.New("Docker transport disconnected")
	for _, test := range []struct {
		name, stdout, stderr string
		exit                 int
		cause                error
		proven               bool
	}{
		{name: "all cases passed", stdout: marker + "\n", proven: true},
		{name: "unknown CLI", stdout: "unknown input `test pppoe cp-binding'\n"},
		{name: "unmapped delete accepted", stdout: "test pppoe cp-binding: unmapped deletion was accepted\n"},
		{name: "different assertion", stdout: "mismatched deletion was accepted\n"},
		{name: "empty output"},
		{name: "incomplete case set", stdout: "PPPoE cp binding: 7 cases passed\n"},
		{name: "longer marker", stdout: marker + " unexpectedly\n"},
		{name: "contradictory success", stdout: "unmapped deletion was accepted\n" + marker + "\n"},
		{name: "stderr assertion", stdout: marker + "\n", stderr: "binding cleanup failed\n"},
		{name: "transport failure", stdout: marker + "\n", cause: transport},
		{name: "process failure", stdout: marker + "\n", exit: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(ctx context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				calls++
				wanted := []string{"docker", "exec", vppPeer, "vppctl", "-s", "/run/vpp/cli.sock", "test", "pppoe", "cp-binding"}
				if !slices.Equal(command.Arguments, wanted) {
					t.Fatalf("binding checker ran %q, want %q", command.Arguments, wanted)
				}
				deadline, bounded := ctx.Deadline()
				if !bounded {
					t.Fatal("binding CLI has no deadline")
				}
				if time.Until(deadline) > controlCommandBudget {
					t.Fatal("binding CLI exceeds the control command budget")
				}
				return interoplab.ProcessResult{Stdout: test.stdout, Stderr: test.stderr, ExitCode: test.exit}, test.cause
			}))
			err := checkBindingOracle(t.Context(), mechanismProbeLab{docker: docker})
			if calls != 1 {
				t.Fatalf("binding checker executed %d commands, want one completed CLI call", calls)
			}
			if test.proven {
				if err != nil {
					t.Fatalf("the complete binding regression was refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("an incomplete, contradictory or failed command counted as a pass")
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
					t.Fatalf("binding failure evidence %q lost: %v", evidence, err)
				}
			}
		})
	}
}

// The isolated action must use one normal VPP image and its declared checker;
// a metadata mutant or another image cannot answer the binding regression.
func TestMechanismBindingPlanUsesBaseline(t *testing.T) {
	plan := bindingProofPlan("/scratch/bindings", "fixture")
	if len(plan.Peers) != 1 {
		t.Fatalf("binding proof has %d peers, want one VPP", len(plan.Peers))
	}
	if plan.Peers[0].Image != buildvpp.ImageName {
		t.Fatalf("binding proof uses %s, want baseline %s", plan.Peers[0].Image, buildvpp.ImageName)
	}
	for _, variant := range []string{"baseline", metadataProofVariant, "", "unknown"} {
		t.Run(variant, func(t *testing.T) {
			calls := 0
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				calls++
				if calls == 1 {
					wanted := []string{"docker", "exec", vppPeer, "cat", "/usr/share/doc/vpp/proof-variant"}
					if !slices.Equal(command.Arguments, wanted) {
						t.Fatalf("binding proof did not read its variant: %q", command.Arguments)
					}
					return interoplab.ProcessResult{Stdout: variant + "\n"}, nil
				}
				if calls == 2 {
					wanted := []string{"docker", "exec", vppPeer, "vppctl", "-s", vppCLIPath, "test", "pppoe", "cp-binding"}
					if !slices.Equal(command.Arguments, wanted) {
						t.Fatalf("binding command = %q", command.Arguments)
					}
					return interoplab.ProcessResult{Stdout: "PPPoE cp binding: 10 cases passed\n"}, nil
				}
				wanted := []string{"docker", "exec", vppPeer, "vppctl", "-s", vppCLIPath, "test", "pppoe", "ownership"}
				if !slices.Equal(command.Arguments, wanted) {
					t.Fatalf("ownership command = %q", command.Arguments)
				}
				if calls != 3 {
					t.Fatalf("unexpected command count %d", calls)
				}
				return interoplab.ProcessResult{Stdout: "PPPoE ownership: 55 cases passed\n"}, nil
			}))
			err := plan.Source.Checker(t.Context(), &interoplab.CheckContext{Lab: mechanismProbeLab{docker: docker}})
			if variant == "baseline" {
				if err != nil {
					t.Fatal(err)
				}
				if calls != 3 {
					t.Fatalf("baseline ran %d commands, want variant readback, binding and ownership regressions", calls)
				}
				return
			}
			if err == nil {
				t.Fatalf("binding proof accepted variant %q", variant)
			}
			if calls != 1 {
				t.Fatal("binding proof ran API cases against a non-baseline image")
			}
		})
	}
}

// The full proof must complete the existing wire-first mechanism before the
// optional binding command, then stop before metadata if either baseline
// stage fails. Bindings must receive the exact positive image identity.
func TestMechanismProofOrdersBindingStageAndPreservesIdentity(t *testing.T) {
	baselineImage := "sha256:" + strings.Repeat("1", 64)
	metadataImage := "sha256:" + strings.Repeat("2", 64)
	for _, test := range []struct {
		name, identity            string
		baselineCode, bindingCode int
		wanted                    []string
		code                      int
	}{
		{name: "complete proof", identity: baselineImage, wanted: []string{"baseline", "bindings", "metadata"}},
		{name: "baseline failed", identity: baselineImage, baselineCode: 1, wanted: []string{"baseline"}, code: 1},
		{name: "baseline identity absent", wanted: []string{"baseline"}, code: 1},
		{name: "baseline identity mutable", identity: "ze-vpp-pppoe-lab:latest", wanted: []string{"baseline"}, code: 1},
		{name: "binding failed", identity: baselineImage, bindingCode: 1, wanted: []string{"baseline", "bindings"}, code: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var stages []string
			report := runProofStages(t.Context(), "/checkout", proofStages{
				baseline: func(context.Context, string) interoplab.SuiteReport {
					stages = append(stages, "baseline")
					return interoplab.SuiteReport{Code: test.baselineCode, Images: []interoplab.ImageResult{{Name: buildvpp.ImageName, Reference: test.identity}}}
				},
				bindings: func(_ context.Context, _ string, image string) interoplab.SuiteReport {
					stages = append(stages, "bindings")
					if image != baselineImage {
						t.Fatalf("binding stage received %q, want baseline identity %s", image, baselineImage)
					}
					result := interoplab.SuiteReport{Code: test.bindingCode, Images: []interoplab.ImageResult{{Name: buildvpp.ImageName, Reference: image}}}
					if test.bindingCode != 0 {
						result.SetupError = "unmapped deletion was accepted"
					}
					return result
				},
				metadata: func(context.Context, string) interoplab.SuiteReport {
					stages = append(stages, "metadata")
					return interoplab.SuiteReport{Images: []interoplab.ImageResult{{Name: buildvpp.MetadataImageName, Reference: metadataImage}}}
				},
			})
			if !slices.Equal(stages, test.wanted) {
				t.Fatalf("proof stages %q, want %q", stages, test.wanted)
			}
			if report.Code != test.code {
				t.Fatalf("proof code %d, want %d", report.Code, test.code)
			}
			if slices.Contains(stages, "bindings") {
				if report.Bindings.Image != baselineImage {
					t.Fatalf("binding report lost the baseline image: %#v", report.Bindings)
				}
				if report.Bindings.Result.Code != test.bindingCode {
					t.Fatal("binding report lost the stage's outcome")
				}
				if test.bindingCode != 0 {
					if !strings.Contains(report.Text(), "unmapped deletion was accepted") {
						t.Fatal("proof output lost the binding stage's C assertion")
					}
				}
			} else if len(report.Bindings.Result.NotRun) != 1 {
				t.Fatal("unreached binding stage is not reported as not run")
			}
			if slices.Contains(stages, "metadata") {
				if report.Metadata.Image != metadataImage {
					t.Fatal("metadata report lost its distinct image")
				}
			} else if len(report.Metadata.Result.NotRun) != 1 {
				t.Fatal("unreached metadata stage is not reported as not run")
			}
		})
	}
}
