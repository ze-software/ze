// Design: docs/architecture/plugin/component-boundaries.md -- engine shutdown retains every owner failure.
package engine_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/engine"
)

type cleanupSubsystem struct {
	*stubSubsystem
	failure error
}

func (s *cleanupSubsystem) Stop(ctx context.Context) error {
	_ = s.stubSubsystem.Stop(ctx)
	return s.failure
}

type cleanupPlugins struct {
	testPlugins
	recorder *recorder
	failure  error
}

func (p *cleanupPlugins) StopAll(context.Context) error {
	p.recorder.record("stop:plugins")
	return p.failure
}

// TestStopRetainsEveryCleanupCause runs two subsystem owners and the plugin
// manager through actual Engine.Stop. Clean cleanup stays nil; failing cleanup
// keeps all causes, visits every owner in order, and is not repeated.
func TestStopRetainsEveryCleanupCause(t *testing.T) {
	first := errors.New("first subsystem cleanup sentinel")
	second := errors.New("second subsystem cleanup sentinel")
	plugins := errors.New("plugin manager cleanup sentinel")
	for _, test := range []struct {
		name                   string
		first, second, plugins error
	}{
		{name: "clean owners"},
		{name: "first owner", first: first},
		{name: "second owner", second: second},
		{name: "plugin owner", plugins: plugins},
		{name: "all owners", first: first, second: second, plugins: plugins},
	} {
		t.Run(test.name, func(t *testing.T) {
			rec := &recorder{}
			eng := engine.NewEngine(stubBus(), stubConfig(), &cleanupPlugins{recorder: rec, failure: test.plugins})
			for _, input := range []struct {
				name    string
				failure error
			}{{name: "first", failure: test.first}, {name: "second", failure: test.second}} {
				if err := eng.RegisterSubsystem(&cleanupSubsystem{
					stubSubsystem: newStubSubsystem(input.name, rec), failure: input.failure,
				}); err != nil {
					t.Fatal(err)
				}
			}
			if err := eng.Start(t.Context()); err != nil {
				t.Fatal(err)
			}
			failure := eng.Stop(t.Context())
			if test.first == nil && test.second == nil && test.plugins == nil && failure != nil {
				t.Fatalf("clean owners returned failure: %v", failure)
			}
			for _, cause := range []error{test.first, test.second, test.plugins} {
				if cause != nil && !errors.Is(failure, cause) {
					t.Errorf("Stop lost cleanup cause %v: %v", cause, failure)
				}
			}
			if err := eng.Stop(t.Context()); err != nil {
				t.Fatalf("idempotent stop returned failure: %v", err)
			}
			want := []string{"start:first", "start:second", "stop:second", "stop:first", "stop:plugins"}
			if got := rec.get(); !slices.Equal(got, want) {
				t.Errorf("lifecycle calls = %v, want %v", got, want)
			}
		})
	}
}
