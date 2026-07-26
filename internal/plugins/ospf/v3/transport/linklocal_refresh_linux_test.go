//go:build linux

package transport

import (
	"net"
	"net/netip"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// TestLinkLocalSourceRefreshesWhileTentative proves a handle opened during IPv6
// DAD picks up the address once DAD completes.
//
// VALIDATES: LinkLocalSource re-resolves while the captured source is tentative,
//
//	and latches once a DAD-complete address appears.
//
// PREVENTS:  an interface opened in the ~1s DAD window keeping an address the
//
//	kernel refuses as a packet source for the handle's whole life --
//	every Send failing `sendmsg: invalid argument` with no recovery
//	when DAD finishes. interfaceLinkLocal deliberately falls back to a
//	tentative address so an environment where DAD never completes still
//	forms an adjacency; caching that fallback is what made it permanent.
func TestLinkLocalSourceRefreshesWhileTentative(t *testing.T) {
	var dadComplete atomic.Bool
	var calls atomic.Int32

	withFakeResolver(t,
		func(string) (iface.Binding, error) { return iface.Binding{OsName: "ens3", Ifindex: 7}, nil },
		func(string) ([]iface.AddrInfo, error) {
			calls.Add(1)
			return []iface.AddrInfo{{
				Address:   "fe80::1",
				Family:    "ipv6",
				LinkLocal: true,
				Tentative: !dadComplete.Load(),
			}}, nil
		},
	)

	li := &linuxInterface{
		ifi:                &net.Interface{Index: 7, Name: "ens3"},
		linkLocal:          netipMustParse(t, "fe80::1"),
		linkLocalTentative: true,
	}

	// Still tentative: the source is returned (so a DAD-never-completes
	// environment keeps working) but the handle must not latch it.
	if got := li.LinkLocalSource(); got.String() != "fe80::1" {
		t.Fatalf("LinkLocalSource while tentative = %v, want fe80::1", got)
	}
	if !li.linkLocalTentative {
		t.Fatal("handle latched a tentative address; it must keep re-resolving until DAD completes")
	}

	dadComplete.Store(true)
	if got := li.LinkLocalSource(); got.String() != "fe80::1" {
		t.Fatalf("LinkLocalSource after DAD = %v, want fe80::1", got)
	}
	if li.linkLocalTentative {
		t.Fatal("handle did not latch the DAD-complete address; it would re-resolve on every send forever")
	}

	// Latched: no further resolver calls, so the steady state costs nothing.
	before := calls.Load()
	li.LinkLocalSource()
	if after := calls.Load(); after != before {
		t.Errorf("LinkLocalSource re-resolved after latching (%d -> %d calls)", before, after)
	}
}

func netipMustParse(t *testing.T, s string) netip.Addr {
	t.Helper()
	a, err := netip.ParseAddr(s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	return a
}
