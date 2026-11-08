package fixture

import (
	"testing"

	"github.com/ze-software/ze/internal/test/fixturedecl"
)

// TestLiveStagesDeclaresItStopsTheDaemon proves the live-stages driver tells the
// runner it ends its daemon.
//
// VALIDATES: liveStagesDriver runs on Observe, which dispatches `request
//
//	shutdown`, and its registration declares StopsDaemon, which is what
//	the runner reads (recordStartsSelfStoppingFixture).
//
// PREVENTS: a peerless .ci driven by reload/live-stages being signaled about a
//
//	second after start, before its script ran.
func TestLiveStagesDeclaresItStopsTheDaemon(t *testing.T) {
	if !fixturedecl.StopsDaemon("reload/live-stages") {
		t.Fatal("reload/live-stages runs on Observe, which stops the daemon, but its registration does not declare StopsDaemon")
	}
}
