package fixture

import (
	"context"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRunPlugin13RequiresCompletedQuiesceBeforeShutdown drives the actual
// shutdown owner with a successful scenario over the shared TLS engine. The
// owner must complete its final drain and report a refusal before shutdown.
//
// MUTATION: remove runPlugin13's final quiesce and the engine receives shutdown
// without a completed delivery barrier.
func TestRunPlugin13RequiresCompletedQuiesceBeforeShutdown(t *testing.T) {
	testObserverQuiesce(t, func(ctx context.Context, _ []string) error {
		return runPlugin13(ctx, "observer-quiesce-test", nil,
			func(context.Context, *sdk.Plugin) error { return nil }, true)
	}, 0, "request quiesce", "request quiesce: status=warning")
}

// TestPlugin15StartedReportsBarrierFailureBeforeShutdown drives the real
// OnStarted owner through SDK startup and the same refused-drain cases. Its
// scenario error must be reported before that owner requests shutdown.
//
// MUTATION: remove plugin15ObserveStarted's ReportFailure call and rejected
// barriers reach request shutdown without the failure sentinel.
func TestPlugin15StartedReportsBarrierFailureBeforeShutdown(t *testing.T) {
	driver := plugin15ObserveStarted("observer-quiesce-test", &sdk.Registration{}, plugin01Quiesce)
	testObserverQuiesce(t, driver, 0, "request quiesce", "request quiesce: status=warning")
}
