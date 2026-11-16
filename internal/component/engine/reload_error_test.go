// Design: docs/architecture/plugin/component-boundaries.md -- ordered subsystem reload recovery
// Related: engine_test.go -- shared subsystem and dependency stubs

package engine_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/engine"
	"github.com/ze-software/ze/pkg/ze"
)

// TestReloadVisitsEverySubsystemAfterErrors drives the real Engine with failures
// at each registration position and together, so rollback cannot strand a later
// subsystem and the caller retains every failure's identity.
func TestReloadVisitsEverySubsystemAfterErrors(t *testing.T) {
	causes := [3]error{
		errors.New("alpha reload refused"),
		errors.New("bravo reload refused"),
		errors.New("charlie reload refused"),
	}
	for _, tc := range []struct {
		name     string
		failures [3]error
	}{
		{name: "first", failures: [3]error{causes[0], nil, nil}},
		{name: "middle", failures: [3]error{nil, causes[1], nil}},
		{name: "last", failures: [3]error{nil, nil, causes[2]}},
		{name: "multiple distinct causes", failures: causes},
		{name: "all succeed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recorder{}
			// The nonempty wrapper makes provider identity observable even
			// though the shared testConfig implementation has zero size.
			config := &struct{ ze.ConfigProvider }{stubConfig()}
			eng := engine.NewEngine(stubBus(), config, stubPlugins())
			var calls []reloadErrorCall
			names := []string{"alpha", "bravo", "charlie"}
			for index, name := range names {
				sub := &reloadErrorSubsystem{
					stubSubsystem: newStubSubsystem(name, rec),
					failure:       tc.failures[index],
					calls:         &calls,
				}
				if err := eng.RegisterSubsystem(sub); err != nil {
					t.Fatalf("RegisterSubsystem(%s): %v", name, err)
				}
			}
			ctx := t.Context()
			if err := eng.Start(ctx); err != nil {
				t.Fatalf("Start: %v", err)
			}
			t.Cleanup(func() {
				if err := eng.Stop(context.Background()); err != nil {
					t.Errorf("Stop: %v", err)
				}
			})

			err := eng.Reload(ctx)
			var order []string
			for _, call := range calls {
				order = append(order, call.name)
				if call.context != ctx {
					t.Errorf("%s received a different reload context", call.name)
				}
				if call.config != config {
					t.Errorf("%s received a different config provider", call.name)
				}
			}
			if !slices.Equal(order, names) {
				t.Errorf("reload visits = %v, want each registration once in order %v", order, names)
			}
			wantError := false
			for index, cause := range causes {
				wantCause := tc.failures[index] != nil
				if wantCause {
					wantError = true
				}
				if gotCause := errors.Is(err, cause); gotCause != wantCause {
					t.Errorf("errors.Is(Reload, %v) = %t, want %t; error = %v", cause, gotCause, wantCause, err)
				}
			}
			if gotError := err != nil; gotError != wantError {
				t.Errorf("Reload error = %v, want error present %t", err, wantError)
			}
		})
	}
}

type reloadErrorCall struct {
	name    string
	context context.Context
	config  ze.ConfigProvider
}

// reloadErrorSubsystem borrows the existing lifecycle stub and records only the
// synchronous Reload call under test; it starts no worker or owned resource.
type reloadErrorSubsystem struct {
	*stubSubsystem
	failure error
	calls   *[]reloadErrorCall
}

func (s *reloadErrorSubsystem) Reload(ctx context.Context, config ze.ConfigProvider) error {
	*s.calls = append(*s.calls, reloadErrorCall{name: s.name, context: ctx, config: config})
	return s.failure
}
