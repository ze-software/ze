//go:build integration && linux

// Design: docs/features/interfaces.md AC-6 -- SLAAC address lifecycle
// tracking. ze observes kernel-autoconfigured addresses and classifies them.
//
// This test exercises the classifier against the REAL kernel netlink stack in
// an ephemeral network namespace (requires CAP_NET_ADMIN; skipped otherwise).
// A kernel-timed IPv6 address (finite valid/preferred lifetime, non-permanent)
// is the flag-equivalent of a SLAAC/RA-assigned address -- IFA_F_PERMANENT is
// clear -- so it drives the same `origin=slaac` classification without needing
// a Router Advertisement daemon. A plain (permanent) address must classify
// `static`. Run under `make ze-qemu-integration-test`.

package iface

import (
	"testing"

	"github.com/vishvananda/netlink"
)

func TestSLAACAddressTracked(t *testing.T) {
	withNetNS(t, func() {
		const dev = "slaac0"
		createDummyForTest(t, dev)
		if err := SetAdminUp(dev); err != nil {
			t.Fatalf("SetAdminUp %s: %v", dev, err)
		}

		// A permanent (operator/kernel-configured) address -> origin "static".
		const staticCIDR = "2001:db8:5100::2/64"
		if err := AddAddress(dev, staticCIDR); err != nil {
			t.Fatalf("AddAddress static: %v", err)
		}

		// A kernel-timed (non-permanent) address, the flag-equivalent of a
		// SLAAC/RA lease -> origin "slaac".
		link, err := netlink.LinkByName(dev)
		if err != nil {
			t.Fatalf("LinkByName %s: %v", dev, err)
		}
		slaacAddr, err := netlink.ParseAddr("2001:db8:5100::abcd/64")
		if err != nil {
			t.Fatalf("ParseAddr: %v", err)
		}
		slaacAddr.ValidLft = 3600
		slaacAddr.PreferedLft = 1800
		if err := netlink.AddrReplace(link, slaacAddr); err != nil {
			t.Fatalf("AddrReplace timed: %v", err)
		}

		ifaces, err := ListInterfaces()
		if err != nil {
			t.Fatalf("ListInterfaces: %v", err)
		}
		var info *InterfaceInfo
		for i := range ifaces {
			if ifaces[i].Name == dev || ifaces[i].OsName == dev {
				info = &ifaces[i]
				break
			}
		}
		if info == nil {
			t.Fatalf("interface %s not found in ListInterfaces", dev)
		}

		var sawStatic, sawSlaac bool
		for _, a := range info.Addresses {
			switch a.Address {
			case "2001:db8:5100::2":
				sawStatic = true
				if a.Origin != "static" {
					t.Errorf("permanent address origin = %q, want static", a.Origin)
				}
			case "2001:db8:5100::abcd":
				sawSlaac = true
				if a.Origin != "slaac" {
					t.Errorf("timed address origin = %q, want slaac", a.Origin)
				}
				if a.ValidLifetime == 0 {
					t.Errorf("timed address valid-lifetime = 0, want the RA/lease lifetime")
				}
			}
		}
		if !sawStatic {
			t.Error("static address not tracked in ze interface state")
		}
		if !sawSlaac {
			t.Error("SLAAC (timed) address not tracked in ze interface state")
		}
	})
}
