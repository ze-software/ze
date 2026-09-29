// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- advertisement transmit path
// Related: instance.go -- the advertisement parameters handed to the transport
// Related: transport/linklocal_first_test.go -- the same order read from the sent frame
package vrrp

import (
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
)

// VALIDATES: an IPv6 Active router hands the transport its addresses with the
// link-local first, in the order the group config gave them.
//
// TestAdvertParamsIPv6LeadWithLinkLocal checks the instance half of the
// transmit order. The config refuses an IPv6 group whose first virtual address
// is not link-local, and TestSendAdvertIPv6LeadsWithLinkLocal proves the
// transport keeps the order it is given, so together they put the link-local
// first on the wire.
//
// Method: an IPv6 address owner (fe80::1 then 2001:db8::1) becomes Active at
// startup and prepares its advertisement. Every parameter set it hands the
// transport must list fe80::1 first and 2001:db8::1 second.
//
// RFC requirement: RFC5798-5.2.9-1 positive -- every advertisement an IPv6 Master prepares lists the link-local fe80::1 first and the global address after it, in the configured order, so the transmitted advertisement leads with the link-local (instance.go, the AdvertParams handed to updateAdvert).
func TestAdvertParamsIPv6LeadWithLinkLocal(t *testing.T) {
	spec := testSpecV6()
	spec.IsOwner = true
	in, _, _ := newTestInstance(t, spec)
	var mu sync.Mutex
	var prepared [][]netip.Addr
	in.deps.updateAdvert = func(_ transport.InstanceKey, p transport.AdvertParams) error {
		mu.Lock()
		defer mu.Unlock()
		prepared = append(prepared, slices.Clone(p.VIPs))
		return nil
	}
	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("owner state = %v, want Master", in.machine.State())
	}

	mu.Lock()
	defer mu.Unlock()
	if len(prepared) == 0 {
		t.Fatal("the Master prepared no advertisement")
	}
	want := []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::1")}
	for i, vips := range prepared {
		if !slices.Equal(vips, want) {
			t.Fatalf("advertisement %d lists %v, want %v with the link-local first", i, vips, want)
		}
	}
}
