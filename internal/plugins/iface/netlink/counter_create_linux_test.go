// Design: docs/features/interfaces.md -- creation-bound raw counter ownership
// Related: counter_linux.go -- the backend's single ordered counter source

//go:build linux

package ifacenetlink

import (
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
)

// TestCounterCreateCapability checks the actual backend's native creation seam
// through a runtime assertion, so an absent implementation is an assertion RED.
func TestCounterCreateCapability(t *testing.T) {
	backend, err := newNetlinkBackend()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, supported := backend.(interface {
		CreateLinkCounterReader(*nl.NetlinkRequest) (
			netlink.Link, func() (*netlink.LinkStatistics, error), bool, error,
		)
	}); !supported {
		t.Fatal("kernel interface backend does not provide a creation-bound counter reader")
	}
}
