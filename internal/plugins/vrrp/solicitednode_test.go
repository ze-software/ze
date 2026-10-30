// Design: docs/architecture/vrrp/vrrp-macvlan-vmac-dataplane.md -- the Active router's macvlan hears its addresses' solicitations before it holds them
// Related: instance.go -- doInstallVIPs and doRemoveVIPs, the executor under test
// Related: instance_test.go -- fakeDeps, which records the membership calls

package vrrp

import (
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/vrrp/fsm"
	"github.com/ze-software/ze/internal/plugins/vrrp/packet"
	"github.com/ze-software/ze/internal/plugins/vrrp/transport"
	"github.com/ze-software/ze/internal/test/sim"
)

// fmtAddrs joins addresses with commas, for a recorded call a test compares.
func fmtAddrs(addrs []netip.Addr) string {
	parts := make([]string, 0, len(addrs))
	for _, a := range addrs {
		parts = append(parts, a.String())
	}
	return strings.Join(parts, ",")
}

// TestInstanceJoinsSolicitedNodeBeforeRegisteringAddresses checks the order
// and the lifetime of the Solicited-Node memberships the executor takes.
//
// Method: an IPv6 instance over the recording fakes starts as a Backup, is
// promoted by its own master-down timer, demoted by a higher-priority
// advertisement, promoted again and shut down. A Backup holds no membership;
// each promotion joins every virtual address on the virtual-MAC device before
// the addresses are registered, because the iface reconcile installs them only
// later; each demotion and the shutdown leave after the addresses are
// deregistered.
func TestInstanceJoinsSolicitedNodeBeforeRegisteringAddresses(t *testing.T) {
	f := &fakeDeps{}
	spec := testSpecV6()
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	key := transport.InstanceKey{Interface: spec.Interface, VRID: spec.VRID, Family: packet.V6}
	in := newInstance(spec, key, "vrrp6dev", clk, f.deps())

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	if got := f.solicited; len(got) != 0 {
		t.Fatalf("a Backup took Solicited-Node memberships %v, want none", got)
	}

	promoteToActive(t, in, clk)
	in.dispatch(fsm.AdvertReceived{Priority: 254, SrcIP: netip.MustParseAddr("fe80::99"), IntervalMs: 1000, VIPCount: 2})
	if in.machine.State() != fsm.StateBackup {
		t.Fatalf("state = %v after a higher-priority advertisement, want Backup", in.machine.State())
	}
	promoteToActive(t, in, clk)
	in.dispatch(fsm.Shutdown{})

	join := "join:vrrp6dev:fe80::1,2001:db8::1"
	want := []string{
		join + ":after-0-installs",
		"leave:after-1-removes",
		join + ":after-1-installs",
		"leave:after-2-removes",
	}
	if !slices.Equal(f.solicited, want) {
		t.Fatalf("Solicited-Node membership calls = %v, want %v", f.solicited, want)
	}
}

// TestActiveInstanceRejoinsSolicitedNodeWhenItsAddressesChange checks that an
// Active router whose virtual addresses are reconfigured joins the new set's
// groups before it registers the new addresses, and replaces its membership
// rather than leaving it first.
//
// Method: an IPv6 instance over the recording fakes is promoted by its own
// master-down timer, then reconfigured through reconfigure, the path a commit
// takes, with one virtual address swapped. masterConfigUpdated (fsm/fsm.go)
// answers a changed address set with InstallVIPs, so the executor joins again.
// A leave between the two joins would open a window in which the group that
// both sets share (fe80::1) is off the filter, so the record MUST show the
// second join with no leave before it; the shutdown then leaves once.
func TestActiveInstanceRejoinsSolicitedNodeWhenItsAddressesChange(t *testing.T) {
	f := &fakeDeps{}
	spec := testSpecV6()
	clk := sim.NewFakeClock(time.Unix(0, 0).UTC())
	key := transport.InstanceKey{Interface: spec.Interface, VRID: spec.VRID, Family: packet.V6}
	in := newInstance(spec, key, "vrrp6dev", clk, f.deps())

	in.dispatch(fsm.Startup{Config: in.fsmConfig()})
	promoteToActive(t, in, clk)

	changed := spec
	changed.VIPs = []netip.Addr{netip.MustParseAddr("fe80::1"), netip.MustParseAddr("2001:db8::2")}
	in.reconfigure(changed)
	if in.machine.State() != fsm.StateMaster {
		t.Fatalf("state = %v after the reconfiguration, want Master", in.machine.State())
	}
	in.dispatch(fsm.Shutdown{})

	want := []string{
		"join:vrrp6dev:fe80::1,2001:db8::1:after-0-installs",
		"join:vrrp6dev:fe80::1,2001:db8::2:after-1-installs",
		"leave:after-1-removes",
	}
	if !slices.Equal(f.solicited, want) {
		t.Fatalf("Solicited-Node membership calls = %v, want %v", f.solicited, want)
	}
}
