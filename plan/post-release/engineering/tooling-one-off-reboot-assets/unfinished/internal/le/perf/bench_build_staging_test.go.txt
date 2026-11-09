// Design: docs/architecture/testing/interop.md -- benchmark image inputs.

package perf

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/test/perfrunner"
)

// TestBuildOnlyStagesCurrentZeBeforeImageBuild drives Bench.Run's build-only
// entry. The image-build consumer reads the actual staged file, so an old file
// or a staging call after image construction cannot satisfy the assertion.
func TestBuildOnlyStagesCurrentZeBeforeImageBuild(t *testing.T) {
	for _, duts := range [][]string{{"ze"}, nil} {
		bench, _ := fixtureBench(t)
		path := bench.stagedZe()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("obsolete-input"), 0o600); err != nil {
			t.Fatal(err)
		}
		staged, built := 0, 0
		bench.Stage = func(_ context.Context, root string) error {
			staged++
			if root != bench.Root {
				t.Errorf("staging root %q, want %q", root, bench.Root)
			}
			return os.WriteFile(path, []byte("current-input"), 0o600)
		}
		bench.Kernel = func(context.Context, string) error {
			t.Error("build-only invoked a runtime kernel probe")
			return errors.New("unexpected runtime probe")
		}
		bench.Measure = func(_ context.Context, run suite) int {
			built++
			if run.Steps != (perfrunner.Steps{Build: true}) {
				t.Errorf("image consumer got steps %+v", run.Steps)
			}
			input, err := os.ReadFile(path)
			if err != nil {
				t.Error(err)
				return 1
			}
			if string(input) != "current-input" {
				t.Errorf("image build consumed %q, want the current staged input", input)
				return 1
			}
			return 0
		}
		report, code := bench.Run(t.Context(), perfrunner.Steps{Build: true}, duts)
		if code != 0 || !report.Built {
			t.Fatalf("build-only duts=%v returned code=%d report=%+v", duts, code, report)
		}
		if staged != 1 || built != 1 {
			t.Fatalf("build-only duts=%v staged=%d built=%d, want one staging before one image build", duts, staged, built)
		}
	}
}

// TestBuildOnlyStagingFailureStopsImageBuild requires the preparation error to
// stop the chain before Docker can build an image from missing or stale inputs.
func TestBuildOnlyStagingFailureStopsImageBuild(t *testing.T) {
	bench, _ := fixtureBench(t)
	bench.Stage = func(context.Context, string) error {
		return errors.New("current Ze input cannot be staged")
	}
	bench.Kernel = func(context.Context, string) error {
		t.Error("failed build-only preparation reached the runtime kernel probe")
		return nil
	}
	bench.Measure = func(context.Context, suite) int {
		t.Error("image build ran after input staging failed")
		return 0
	}
	report, code := bench.Run(t.Context(), perfrunner.Steps{Build: true}, []string{"ze"})
	if code == 0 {
		t.Fatal("failed input staging was reported as a successful build")
	}
	if !strings.Contains(report.Error, "current Ze input cannot be staged") {
		t.Fatalf("build refusal lost its input failure: %+v", report)
	}
}
