// Design: docs/labs/pppoe-interop.md -- one baseline lease spans the complete proof.
// Related: proof.go -- retained baseline image; interoplab/docker.go -- Build and ReleaseImage.
package testvpppppoe

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	buildvpp "github.com/ze-software/ze/internal/le/build/vpp"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// Replace the shared tag before the baseline Suite returns. The dependent peer
// must still start from that exact image, and its cleanup must precede release.
func TestMechanismProofRetainsBaselineAcrossStages(t *testing.T) {
	for _, test := range []struct {
		name    string
		failure string
		cancel  bool
		wanted  []string
	}{
		{name: "complete", wanted: []string{"baseline", "bindings", "metadata", "release"}},
		{name: "baseline failure", failure: "baseline", wanted: []string{"baseline", "release"}},
		{name: "binding failure", failure: "bindings", wanted: []string{"baseline", "bindings", "release"}},
		{name: "metadata failure", failure: "metadata", wanted: []string{"baseline", "bindings", "metadata", "release"}},
		{name: "binding cancellation", failure: "bindings", cancel: true, wanted: []string{"baseline", "bindings", "release"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			builder := newProofLeaseBuilder(t)
			root := t.TempDir()
			report := runProofWithBuilder(ctx, root, builder, proofLeasePreparation, func(image string) proofStages {
				return proofStages{
					baseline: func(ctx context.Context, _ string) interoplab.SuiteReport {
						// An empty image follows the old Run/Suite contract: build and
						// release before returning. A supplied ID owns no second lease.
						if image == "" {
							built, err := builder.Build(ctx, interoplab.ImageBuild{Name: buildvpp.ImageName})
							if err != nil {
								return setupFailure(err)
							}
							image = built.Reference
							defer func() {
								if err := builder.ReleaseImage(context.WithoutCancel(ctx), built); err != nil {
									t.Errorf("baseline release: %v", err)
								}
							}()
						}
						builder.stages = append(builder.stages, "baseline")
						builder.shared = "sha256:" + strings.Repeat("9", 64)
						return proofLeaseStageResult(image, test.failure == "baseline")
					},
					bindings: func(_ context.Context, _ string, reference string) interoplab.SuiteReport {
						builder.stages = append(builder.stages, "bindings")
						if reference != builder.image.Reference {
							t.Errorf("binding image %s, want %s", reference, builder.image.Reference)
						}
						if !builder.available(reference) {
							t.Error("baseline image was removed before the dependent binding peer started")
						}
						if test.cancel {
							cancel()
						}
						// Returning represents Suite completing dependent-peer removal.
						return proofLeaseStageResult(reference, test.failure == "bindings")
					},
					metadata: func(context.Context, string) interoplab.SuiteReport {
						builder.stages = append(builder.stages, "metadata")
						if !builder.available(image) {
							t.Error("baseline lease ended before the complete proof finished")
						}
						result := proofLeaseStageResult("sha256:"+strings.Repeat("2", 64), test.failure == "metadata")
						result.Images[0].Name = buildvpp.MetadataImageName
						return result
					},
				}
			})
			if !slices.Equal(builder.stages, test.wanted) {
				t.Errorf("lease stages %q, want %q", builder.stages, test.wanted)
			}
			if builder.builds != 1 {
				t.Errorf("baseline built %d times, want one", builder.builds)
			}
			if builder.releases != 1 {
				t.Errorf("baseline released %d times, want one", builder.releases)
			}
			if builder.available(builder.image.Reference) {
				t.Error("completed proof left its baseline image retained after the shared tag moved")
			}
			if test.failure == "" {
				if report.Code != 0 {
					t.Errorf("successful proof failed: %s", report.Text())
				}
			} else if report.Code == 0 {
				t.Error("failed stage counted as a successful proof")
			}
			if builder.recipe.Dockerfile != filepath.Join(root, "third_party", "vpp-patches", "Dockerfile") {
				t.Errorf("baseline did not use the registered source recipe: %#v", builder.recipe)
			}
		})
	}
}

// Build failure acquires no lease; a successful build whose release fails still
// reports its completed stage results and the exact cleanup failure.
func TestMechanismProofReportsBaselineLeaseFailures(t *testing.T) {
	for _, failure := range []string{"build", "release", "identity"} {
		t.Run(failure, func(t *testing.T) {
			builder := newProofLeaseBuilder(t)
			cause := errors.New("owned baseline " + failure + " failed")
			switch failure {
			case "build":
				builder.buildError = cause
			case "release":
				builder.releaseError = cause
			case "identity":
				builder.image.Reference = "mutable:baseline"
			}
			report := runProofWithBuilder(t.Context(), t.TempDir(), builder, proofLeasePreparation, func(image string) proofStages {
				return proofStages{
					baseline: func(context.Context, string) interoplab.SuiteReport {
						builder.stages = append(builder.stages, "baseline")
						return proofLeaseStageResult(image, false)
					},
					bindings: func(context.Context, string, string) interoplab.SuiteReport {
						builder.stages = append(builder.stages, "bindings")
						return proofLeaseStageResult(image, false)
					},
					metadata: func(context.Context, string) interoplab.SuiteReport {
						builder.stages = append(builder.stages, "metadata")
						result := proofLeaseStageResult("sha256:"+strings.Repeat("2", 64), false)
						result.Images[0].Name = buildvpp.MetadataImageName
						return result
					},
				}
			})
			if report.Code == 0 {
				t.Fatal("a baseline lease failure counted as a successful proof")
			}
			if failure == "build" {
				if builder.releases != 0 {
					t.Error("failed build released a lease it did not acquire")
				}
				if len(builder.stages) != 0 {
					t.Errorf("proof stages ran after build failure: %q", builder.stages)
				}
			} else if builder.releases != 1 {
				t.Errorf("successful build released %d times, want one", builder.releases)
			}
			if failure == "identity" {
				if slices.Contains(builder.stages, "baseline") {
					t.Error("baseline ran without an immutable built image")
				}
			} else if !strings.Contains(report.Text(), cause.Error()) {
				t.Errorf("proof output lost lease failure: %s", report.Text())
			}
		})
	}
}

func proofLeaseStageResult(image string, failed bool) interoplab.SuiteReport {
	result := interoplab.SuiteReport{Images: []interoplab.ImageResult{{Name: buildvpp.ImageName, Reference: image}}}
	if failed {
		result.Code = 1
		result.SetupError = "controlled stage failure"
	}
	return result
}

func proofLeasePreparation(context.Context) error { return nil }

// proofLeaseBuilder models Docker's two tags, not image-ID persistence: moving
// the shared tag leaves only the successful Build's owned retention lease.
type proofLeaseBuilder struct {
	t                        *testing.T
	image                    interoplab.ImageResult
	recipe                   interoplab.ImageBuild
	shared                   string
	retained                 bool
	builds, releases         int
	stages                   []string
	buildError, releaseError error
}

func newProofLeaseBuilder(t *testing.T) *proofLeaseBuilder {
	return &proofLeaseBuilder{t: t, image: interoplab.ImageResult{Name: buildvpp.ImageName, Reference: "sha256:" + strings.Repeat("1", 64), Built: true}}
}

func (builder *proofLeaseBuilder) Build(_ context.Context, recipe interoplab.ImageBuild) (interoplab.ImageResult, error) {
	builder.builds++
	builder.recipe = recipe
	if builder.buildError != nil {
		return interoplab.ImageResult{}, builder.buildError
	}
	builder.shared = builder.image.Reference
	builder.retained = true
	return builder.image, nil
}

func (builder *proofLeaseBuilder) ReleaseImage(ctx context.Context, image interoplab.ImageResult) error {
	builder.t.Helper()
	builder.releases++
	builder.stages = append(builder.stages, "release")
	if image.Reference != builder.image.Reference {
		builder.t.Errorf("released foreign image %s", image.Reference)
	}
	if err := ctx.Err(); err != nil {
		builder.t.Errorf("baseline release inherited cancellation: %v", err)
	}
	deadline, bounded := ctx.Deadline()
	if !bounded {
		builder.t.Error("baseline release has no cleanup deadline")
	} else if time.Until(deadline) > controlCommandBudget {
		builder.t.Error("baseline release exceeds the command cleanup budget")
	}
	builder.retained = false
	return builder.releaseError
}

func (builder *proofLeaseBuilder) available(image string) bool {
	if image == builder.shared {
		return true
	}
	if image != builder.image.Reference {
		return false
	}
	return builder.retained
}
