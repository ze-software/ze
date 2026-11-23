package golint

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/le/job"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// deterministicListCapture answers a go-list package keyed by the flavor's
// GOOS/GOARCH/-tags (as real `go list` behaves, independent of call order).
func deterministicListCapture(root string, tracked []string) func(context.Context, []string, string, []string) commandResult {
	var mu sync.Mutex
	keyIndex := map[string]int{}
	for idx, f := range append(basePasses(), flavorMatrix([]string{"ze_a", "ze_b"})...) {
		key := flavorCaptureKey(f.GOOS, f.GOARCH, effectiveFixtureTags(f))
		keyIndex[key] = idx
	}
	return func(_ context.Context, argv []string, _ string, environment []string) commandResult {
		switch argv[0] {
		case listProgram:
			var goos, goarch, tags string
			for _, kv := range environment {
				if v, ok := strings.CutPrefix(kv, "GOOS="); ok {
					goos = v
				} else if v, ok := strings.CutPrefix(kv, "GOARCH="); ok {
					goarch = v
				}
			}
			for i := 0; i+1 < len(argv); i++ {
				if argv[i] == "-tags" {
					tags = argv[i+1]
					break
				}
			}
			mu.Lock()
			idx := keyIndex[flavorCaptureKey(goos, goarch, tags)]
			mu.Unlock()
			path := filepath.Join(root, "pkg", "p"+twoDigits(idx), "file.go")
			return commandResult{stdout: []byte(fmt.Sprintf(`{"Dir":%q,"Name":"fixture","ImportPath":%q,"GoFiles":["file.go"]}`+"\n", filepath.Dir(path), "lintfixture/pkg/p"+twoDigits(idx)))}
		case trackedProgram:
			return commandResult{stdout: append([]byte(strings.Join(tracked, "\x00")), 0)}
		default:
			return commandResult{code: 127, err: fmt.Errorf("unexpected capture command %v", argv)}
		}
	}
}

func flavorCaptureKey(goos, goarch, tags string) string {
	return goos + "|" + goarch + "|" + tags
}

func effectiveFixtureTags(f Flavor) string {
	dropped := map[string]bool{}
	for _, t := range f.Without {
		dropped[t] = true
	}
	var tags []string
	for _, t := range []string{"ze_core", "ze_a", "ze_b"} {
		if !dropped[t] {
			tags = append(tags, t)
		}
	}
	tags = append(tags, f.Tags...)
	return strings.Join(tags, " ")
}

// VALIDATES: parallel go-list planning (listWorkers > 1) produces the exact
// same LintPlan (passes, package scopes, commands, skipped flags, coverage) as
// sequential planning, and still refuses input drift before running any linter.
func TestParallelPlanMatchesSequentialAndRefusesDrift(t *testing.T) {
	t.Parallel()

	root, tracked := lintFixture(t)
	capture := deterministicListCapture(root, tracked)

	seqOps, _ := fixtureOps(root, tracked)
	seqOps.capture = capture
	seqRunner, err := newRunner(t.Context(), root, fixtureChain(root), seqOps)
	if err != nil {
		t.Fatalf("newRunner sequential: %v", err)
	}
	seqPlan, err := seqRunner.Plan()
	if err != nil {
		t.Fatalf("sequential Plan: %v", err)
	}

	parOps, _ := fixtureOps(root, tracked)
	parOps.capture = capture
	parOps.listWorkers = 4
	parRunner, err := newRunner(t.Context(), root, fixtureChain(root), parOps)
	if err != nil {
		t.Fatalf("newRunner parallel: %v", err)
	}
	parPlan, err := parRunner.Plan()
	if err != nil {
		t.Fatalf("parallel Plan: %v", err)
	}

	if len(parPlan.Passes) != len(seqPlan.Passes) {
		t.Fatalf("parallel pass count = %d; want %d", len(parPlan.Passes), len(seqPlan.Passes))
	}
	for i := range seqPlan.Passes {
		got, want := parPlan.Passes[i], seqPlan.Passes[i]
		if got.Name != want.Name || got.GOOS != want.GOOS || got.GOARCH != want.GOARCH ||
			got.Skipped != want.Skipped ||
			!reflect.DeepEqual(got.TagsAdded, want.TagsAdded) ||
			!reflect.DeepEqual(got.TagsDropped, want.TagsDropped) ||
			!reflect.DeepEqual(got.Packages, want.Packages) ||
			!reflect.DeepEqual(got.Command, want.Command) {
			t.Fatalf("pass %d (%s) differs:\n got: %+v\nwant: %+v", i, want.Name, got, want)
		}
	}
	if !reflect.DeepEqual(parPlan.Coverage, seqPlan.Coverage) {
		t.Fatalf("parallel coverage differs: got %#v, want %#v", parPlan.Coverage, seqPlan.Coverage)
	}

	// Input drift mid-planning must still refuse with cannotPlan and start no linter.
	driftOps, _ := fixtureOps(root, tracked)
	var mu sync.Mutex
	var drifted atomic.Bool
	generation := "before"
	driftOps.inputChanges = func(root string, start job.InputSnapshot, reaches func(string) bool) ([]job.InputChange, error) {
		mu.Lock()
		defer mu.Unlock()
		if drifted.Load() {
			generation = "after"
		}
		return generationChanges(&generation, "pkg/p00/file.go")(root, start, reaches)
	}
	var queries atomic.Int64
	driftOps.capture = func(ctx context.Context, argv []string, dir string, environment []string) commandResult {
		res := capture(ctx, argv, dir, environment)
		if argv[0] == listProgram && queries.Add(1) == 3 {
			drifted.Store(true)
		}
		return res
	}
	driftOps.stream = func(context.Context, []string, string, []string, io.Writer) (int, error) {
		t.Fatal("input drift started a lint child under parallel planning")
		return 0, nil
	}
	driftOps.listWorkers = 4
	driftRunner, err := newRunner(t.Context(), root, fixtureChain(root), driftOps)
	if err != nil {
		t.Fatalf("newRunner drift: %v", err)
	}
	answer, code := runRunner(driftRunner, leaction.Arguments{"scope": {"./pkg/p00"}})
	report, ok := answer.(Report)
	if !ok {
		t.Fatalf("answer = %T, want Report", answer)
	}
	if code != cannotPlan || report.Code != cannotPlan || len(report.Passes) != 0 {
		t.Fatalf("drift result = %#v, code %d; want cannotPlan with 0 passes", report, code)
	}
}
