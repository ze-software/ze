// Design: docs/labs/pppoe-interop.md -- the patched VPP lab image has one owner.
// VALIDATES: the native build action reaches the registered image and keeps its
// immutable identity while releasing only its temporary retention tag.
package buildvpp

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/interoplab"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

func TestVPPBuildActionRegisters(t *testing.T) {
	if !command.HasLocal(leroot.CommandPath(area)) {
		t.Fatal("the VPP build action is not registered")
	}
	if _, code := Answer([]string{"unexpected"}); code == 0 {
		t.Fatal("an unknown build action was accepted")
	}
}

func TestVPPBuildUsesRegisteredImage(t *testing.T) {
	root := t.TempDir()
	builder := &recordingBuilder{}
	report, err := buildImage(t.Context(), root, proofBaseline, builder)
	if err != nil {
		t.Fatal(err)
	}
	if builder.build.Context != filepath.Join(root, "third_party", "vpp-patches") {
		t.Fatalf("build sends unrelated checkout inputs: %#v", builder.build)
	}
	if builder.build.Tag != ImageName {
		t.Fatalf("build tag = %q, want registered image %q", builder.build.Tag, ImageName)
	}
	if report.Image.Reference != "sha256:fixture" {
		t.Fatalf("immutable image identity lost: %#v", report)
	}
	if builder.releases != 1 {
		t.Fatalf("released image %d times, want one", builder.releases)
	}
}

// A failed Docker build is an error; no unbuilt image becomes lab evidence.
func TestVPPBuildFailureIsReported(t *testing.T) {
	want := errors.New("source archive checksum mismatch")
	builder := &recordingBuilder{buildError: want}
	_, err := buildImage(t.Context(), "/checkout", proofBaseline, builder)
	if !errors.Is(err, want) {
		t.Fatalf("build failure = %v, want %v", err, want)
	}
	if builder.releases != 0 {
		t.Fatal("failed build released an image it never retained")
	}
}

// Cleanup failure must reach the caller even when the image built correctly.
func TestVPPBuildRetentionFailureIsReported(t *testing.T) {
	want := errors.New("Docker retention tag cannot be removed")
	builder := &recordingBuilder{releaseError: want}
	_, err := buildImage(t.Context(), "/checkout", proofBaseline, builder)
	if !errors.Is(err, want) {
		t.Fatalf("cleanup failure = %v, want %v", err, want)
	}
}

// A canceled caller must reach Docker unchanged and must not produce a built
// image or release a retention tag it never acquired.
func TestVPPBuildCancellationReachesBuilder(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	builder := &recordingBuilder{}
	_, err := buildImage(ctx, t.TempDir(), proofBaseline, builder)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled build returned %v, want context.Canceled", err)
	}
	if builder.buildContext == nil {
		t.Fatal("the builder never received the caller's context")
	}
	if !errors.Is(builder.buildContext.Err(), context.Canceled) {
		t.Fatal("the caller's cancellation did not reach the builder")
	}
	if builder.releases != 0 {
		t.Fatal("an unbuilt image acquired retention cleanup")
	}
}

// Cancellation can race a successful export. The owned tag must still be
// released with a context whose cancellation cannot interrupt cleanup.
func TestVPPBuildRetentionCleanupSurvivesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	builder := &recordingBuilder{cancelOnBuild: cancel}
	_, err := buildImage(ctx, t.TempDir(), proofBaseline, builder)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(ctx.Err(), context.Canceled) {
		t.Fatal("the fixture never canceled the export's caller")
	}
	if builder.releases != 1 {
		t.Fatalf("cleanup ran %d times, want one", builder.releases)
	}
	if builder.releaseContext == nil {
		t.Fatal("retention cleanup never received a context")
	}
	if err := builder.releaseContext.Err(); err != nil {
		t.Fatalf("retention cleanup inherited cancellation: %v", err)
	}
	if builder.releaseContext.Done() != nil {
		t.Fatal("retention cleanup can still be canceled")
	}
}

type recordingBuilder struct {
	build          interoplab.ImageBuild
	buildError     error
	releaseError   error
	releases       int
	buildContext   context.Context
	releaseContext context.Context
	cancelOnBuild  context.CancelFunc
}

func (b *recordingBuilder) Build(ctx context.Context, build interoplab.ImageBuild) (interoplab.ImageResult, error) {
	b.build = build
	b.buildContext = ctx
	if err := ctx.Err(); err != nil {
		return interoplab.ImageResult{}, err
	}
	if b.buildError != nil {
		return interoplab.ImageResult{}, b.buildError
	}
	if b.cancelOnBuild != nil {
		b.cancelOnBuild()
	}
	return interoplab.ImageResult{Name: build.Name, Tag: build.Tag, Reference: "sha256:fixture", Built: true}, nil
}

func (b *recordingBuilder) ReleaseImage(ctx context.Context, _ interoplab.ImageResult) error {
	b.releases++
	b.releaseContext = ctx
	return b.releaseError
}
