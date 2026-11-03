// Design: docs/architecture/api/process-protocol.md -- event stream correlation.

package sdk

import (
	"runtime"
	"testing"
	"time"
)

// TestUnionConfiguredBeforeStart exercises scheduling before the first event and
// requires the configured sweeper to deliver the complete timeout payload.
func TestUnionConfiguredBeforeStart(t *testing.T) {
	delivered := make(chan [2]string, 1)
	u := newUnion("update", "rpki", time.Millisecond, func(primary, secondary string) {
		delivered <- [2]string{primary, secondary}
	}, time.Millisecond, defaultMaxPending)
	defer u.Stop()
	runtime.Gosched()
	u.OnEvent("update", "10.0.0.1", 99, "configured-timeout")
	select {
	case got := <-delivered:
		if got != [2]string{"configured-timeout", ""} {
			t.Fatalf("timeout payload = %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("configured sweeper did not deliver the timeout")
	}
}
