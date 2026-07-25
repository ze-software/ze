package detect

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/ddosevent"
)

// TestClassifyDirectionUnknownIsRemote validates AC-12: an unresolved victim
// classifies as remote (the fail-safe), without any backend lookup.
func TestClassifyDirectionUnknownIsRemote(t *testing.T) {
	d := &detector{}
	if got := d.classifyDirection(netip.Prefix{}); got != ddosevent.DirectionRemote {
		t.Errorf("invalid victim: got %q, want remote", got)
	}
}
