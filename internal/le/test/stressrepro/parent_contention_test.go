package teststressrepro

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/test/runner"
)

// TestStressPublishesActualSiblingContention executes real shell children.
// Four configured lanes do not make an isolated final batch concurrent. An
// inherited enclosing factor remains present even for one stress invocation.
func TestStressPublishesActualSiblingContention(t *testing.T) {
	for _, test := range []struct {
		name       string
		parent     string
		iterations int
		parallel   int
		want       []string
	}{
		{"isolated", "1", 1, 4, []string{"1"}},
		{"siblings and final isolated batch", "1", 3, 2, []string{"1", "3", "3"}},
		{"enclosing parallel runner", "3", 1, 1, []string{"3"}},
		{"malformed parent keeps floor", "invalid", 1, 1, []string{"1"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := stressTree(t)
			t.Setenv("ZE_BIN", filepath.Join(root, "bin", "ze"))
			t.Setenv(runner.ParallelFactorEnv, test.parent)
			t.Setenv("ZE_TEST_PARALLEL_FACTOR", "99")
			env.ResetCache()
			results := filepath.Join(root, "child factors")
			executable := filepath.Join(root, "factor observer")
			program := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"${ze_test_parallel_factor-unpublished}\" >> " + fixtureShellQuote(results) + "\n"
			if err := os.WriteFile(executable, []byte(program), 0o700); err != nil {
				t.Fatal(err)
			}
			opts := baseOptions()
			opts.Iterations, opts.Parallel = test.iterations, test.parallel
			deps, _ := testDependencies(captureFixtureRunner{executable: executable})
			report, code := run(context.Background(), root, opts, deps)
			if report.Completed != test.iterations {
				t.Fatalf("completed %d, want %d: %#v", report.Completed, test.iterations, report)
			}
			if code != 1 {
				t.Fatalf("successful observer run exit = %d, want 1: %#v", code, report)
			}
			data, err := os.ReadFile(results)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Fields(string(data))
			slices.Sort(got)
			if !slices.Equal(got, test.want) {
				t.Fatalf("executed child factors = %q, want %q", got, test.want)
			}
		})
	}
}

// TestStressContentionPreservesOperatorInvocationTimeout keeps the explicitly
// requested invocation limit separate from runner-owned contention headroom.
func TestStressContentionPreservesOperatorInvocationTimeout(t *testing.T) {
	root := stressTree(t)
	t.Setenv(runner.ParallelFactorEnv, "3")
	env.ResetCache()
	fake := &fakeRunner{}
	deps, _ := testDependencies(fake)
	opts := baseOptions()
	opts.Iterations, opts.Parallel, opts.Timeout = 2, 2, 7
	report, code := run(context.Background(), root, opts, deps)
	if code != 1 {
		t.Fatalf("observer run exit = %d, want 1: %#v", code, report)
	}
	if report.Completed != 2 {
		t.Fatalf("completed = %d, want 2", report.Completed)
	}
	for index, spec := range fake.invocations {
		if spec.timeout != 7*time.Second {
			t.Errorf("invocation %d limit = %s, want 7s", index, spec.timeout)
		}
	}
}
