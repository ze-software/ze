// Design: docs/labs/pppoe-interop.md -- refuse an unsuitable host before source image builds.
// Related: proof.go -- shared host preflight; plan.go -- diagnostic-preserving image cleanup.
package testvpppppoe

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	buildvpp "github.com/ze-software/ze/internal/le/build/vpp"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// Real Docker probe/kernel primitives surround a controlled fresh staging
// callback. Any failed preflight must stop before Build and all proof stages.
func TestMechanismProofPreflightsBeforeBaselineBuild(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure string
		wanted  []string
	}{
		{name: "ready", wanted: []string{"probe", "stage", "kernel", "build", "baseline", "bindings", "metadata"}},
		{name: "Docker refused", failure: "probe", wanted: []string{"probe"}},
		{name: "stage refused", failure: "stage", wanted: []string{"probe", "stage"}},
		{name: "kernel refused", failure: "kernel", wanted: []string{"probe", "stage", "kernel"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls []string
			cause := errors.New("controlled " + test.failure + " refusal")
			root := t.TempDir()
			ze := filepath.Join(root, "ze")
			docker := interoplab.NewFixtureDocker(mechanismProcess(func(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
				arguments := command.Arguments
				if slices.Equal(arguments, []string{"docker", "info"}) {
					calls = append(calls, "probe")
					if test.failure == "probe" {
						return interoplab.ProcessResult{}, cause
					}
					return interoplab.ProcessResult{}, nil
				}
				if slices.Contains(arguments, "{{json .SecurityOptions}}") {
					return interoplab.ProcessResult{Stdout: "[]"}, nil
				}
				if slices.Contains(arguments, "{{.KernelVersion}}") {
					return interoplab.ProcessResult{Stdout: "6.18.0-ze"}, nil
				}
				if slices.Contains(arguments, "kernel-capabilities") {
					calls = append(calls, "kernel")
					if test.failure == "kernel" {
						return interoplab.ProcessResult{ExitCode: 1, Stdout: `{"ready":false,"capabilities":[{"subsystem":"controlled","kernel":"CONFIG_CONTROLLED","state":"absent","reason":"controlled kernel refusal"}]}`}, nil
					}
					return interoplab.ProcessResult{Stdout: `{"ready":true,"capabilities":[{"subsystem":"controlled","kernel":"CONFIG_CONTROLLED","state":"present"}]}`}, nil
				}
				return interoplab.ProcessResult{}, nil
			}))
			stage := func(context.Context, *interoplab.Docker) error {
				calls = append(calls, "stage")
				if test.failure == "stage" {
					return cause
				}
				return os.WriteFile(ze, []byte("fresh staged Ze fixture"), 0o600)
			}
			builder := &proofPreparationBuilder{proofLeaseBuilder: newProofLeaseBuilder(t), calls: &calls}
			report := runProofWithBuilder(t.Context(), root, builder, func(ctx context.Context) error {
				return prepareVPPHost(ctx, docker, stage, ze)
			}, func(image string) proofStages {
				return proofStages{
					baseline: func(context.Context, string) interoplab.SuiteReport {
						calls = append(calls, "baseline")
						return proofLeaseStageResult(image, false)
					},
					bindings: func(context.Context, string, string) interoplab.SuiteReport {
						calls = append(calls, "bindings")
						return proofLeaseStageResult(image, false)
					},
					metadata: func(context.Context, string) interoplab.SuiteReport {
						calls = append(calls, "metadata")
						result := proofLeaseStageResult("sha256:"+strings.Repeat("2", 64), false)
						result.Images[0].Name = buildvpp.MetadataImageName
						return result
					},
				}
			})
			if !slices.Equal(calls, test.wanted) {
				t.Errorf("preflight/build stages %q, want %q", calls, test.wanted)
			}
			if test.failure == "" {
				if report.Code != 0 {
					t.Errorf("ready host refused: %s", report.Text())
				}
			} else {
				if builder.builds != 0 {
					t.Errorf("unsuitable host built %d source images", builder.builds)
				}
				if builder.releases != 0 {
					t.Error("failed preflight released an unacquired image lease")
				}
				if report.Code == 0 {
					t.Error("failed preflight counted as proof success")
				}
				if !strings.Contains(report.Text(), cause.Error()) {
					t.Errorf("proof lost preflight cause: %s", report.Text())
				}
			}
		})
	}
}

// A client lease cleanup failure must retain the earlier staging/kernel/setup
// refusal and every previously captured cleanup error, including on cancellation.
func TestMechanismImageReleasePreservesPrimaryFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	builder := newProofLeaseBuilder(t)
	builder.releaseError = errors.New("client image retention release failed")
	report := interoplab.SuiteReport{Code: 1, SetupError: "runtime kernel refused", CleanupErrors: []string{"peer removal failed"}}
	releaseMechanismImages(ctx, builder, []interoplab.ImageResult{builder.image}, &report)
	if report.SetupError != "runtime kernel refused" {
		t.Errorf("release replaced primary failure: %q", report.SetupError)
	}
	want := []string{"peer removal failed", builder.releaseError.Error()}
	if !slices.Equal(report.CleanupErrors, want) {
		t.Errorf("cleanup errors %q, want %q", report.CleanupErrors, want)
	}
	if report.Code != 1 {
		t.Error("release failure lost the failure verdict")
	}
	if builder.releases != 1 {
		t.Errorf("client image released %d times, want once", builder.releases)
	}
}

type proofPreparationBuilder struct {
	*proofLeaseBuilder
	calls *[]string
}

func (builder *proofPreparationBuilder) Build(ctx context.Context, recipe interoplab.ImageBuild) (interoplab.ImageResult, error) {
	*builder.calls = append(*builder.calls, "build")
	return builder.proofLeaseBuilder.Build(ctx, recipe)
}
