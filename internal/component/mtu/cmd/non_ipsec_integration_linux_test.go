//go:build linux

// Design: docs/architecture/diagnostics/path-mtu.md -- non-IPsec tunnel sizing against a real Linux kernel
//
// VALIDATES: AC-1, AC-3, AC-5: real Linux kernel GRE (keyed) and VXLAN links
// created in the sender namespace over a 1400-octet clamped three-namespace
// path are discovered through the real kernel iface backend (`listTunnelSnapshot`),
// probed over real ICMP DF sockets (`openLiveProber`), and sized through
// `handleShowMTU` with exact ceilings (1372, 1350), recommendations (1340,
// 1318), `oversized` verdicts, circuit-clamped underlay advice, and exact
// `set interface tunnel <name> mtu <rec>` commands.
// PREVENTS: mismatches between rtnetlink tunnel readback, dormant kernel
// fallback filtering, ICMP path measurement, and non-IPsec overhead math.

package cmd

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ipsecinventory"
	_ "github.com/ze-software/ze/internal/plugins/iface/netlink"
	"github.com/ze-software/ze/internal/test/userns"
)

// TestShowMTUNonIPsecClampedPathIntegration drives handleShowMTU against real
// kernel GRE (keyed, MTU 1472) and VXLAN (MTU 1450) links over the 1400-octet
// clamped three-namespace path inside an unprivileged user namespace.
func TestShowMTUNonIPsecClampedPathIntegration(t *testing.T) {
	if !userns.Enter(t) {
		return
	}
	withClampedPath(t, func(p *clampedPath) {
		// Reference address on the far side of the 1400-octet clamp.
		refAddr := netip.MustParseAddr("10.99.2.9")
		addAddr(t, p.farHandle, "fr0", refAddr, 32)

		if err := iface.EnsureBackend(); err != nil {
			t.Fatalf("EnsureBackend: %v", err)
		}
		backend := iface.GetBackend()

		if err := backend.CreateTunnel(iface.TunnelSpec{
			Kind:          iface.TunnelKindGRE,
			Name:          "mtugre1",
			LocalAddress:  senderAddr4.String(),
			RemoteAddress: farAddr4.String(),
			Key:           42,
			KeySet:        true,
			TTL:           64,
			TTLSet:        true,
		}); err != nil {
			t.Fatalf("create keyed GRE tunnel: %v", err)
		}
		if err := backend.AddAddress("mtugre1", "10.200.0.1/30"); err != nil {
			t.Fatalf("add address to mtugre1: %v", err)
		}
		if err := backend.SetMTU("mtugre1", 1472); err != nil {
			t.Fatalf("set mtugre1 MTU: %v", err)
		}
		if err := backend.SetAdminUp("mtugre1"); err != nil {
			t.Fatalf("bring mtugre1 up: %v", err)
		}

		if err := backend.CreateTunnel(iface.TunnelSpec{
			Kind:          iface.TunnelKindVxlan,
			Name:          "mtuvx10",
			LocalAddress:  senderAddr4.String(),
			RemoteAddress: farAddr4.String(),
			VNI:           100,
			VNISet:        true,
		}); err != nil {
			t.Fatalf("create VXLAN tunnel: %v", err)
		}
		if err := backend.SetMTU("mtuvx10", 1450); err != nil {
			t.Fatalf("set mtuvx10 MTU: %v", err)
		}
		if err := backend.SetAdminUp("mtuvx10"); err != nil {
			t.Fatalf("bring mtuvx10 up: %v", err)
		}

		// Use real listTunnelSnapshot, real openLiveProber, real routeInterfaceOf,
		// real iface.GetLink, and real KernelPathMTU. Only supply an empty IPsec
		// inventory and the test reference address.
		previous := liveDeps
		liveDeps.tunnels = func() ([]ipsecinventory.Tunnel, error) { return nil, nil }
		liveDeps.nameServers = func() ([]string, bool) { return []string{refAddr.String()}, true }
		t.Cleanup(func() { liveDeps = previous })

		doc := showMTU(t)
		if doc[fieldStatus] != runStatusOK.String() {
			t.Fatalf("status = %v, want ok; notes = %v", doc[fieldStatus], doc[fieldNotes])
		}
		if doc[fieldVerdict] != runVerdictActionNeeded.String() {
			t.Errorf("verdict = %v, want action-needed", doc[fieldVerdict])
		}

		rows := rowsOf(t, doc, fieldTunnels)
		if len(rows) != 2 {
			t.Fatalf("got %d tunnel rows (%v), want 2 (mtugre1 and mtuvx10; dormant kernel fallbacks must be filtered)", len(rows), rows)
		}

		greRow := rowByInterface(t, rows, "mtugre1")
		if greRow[fieldKind] != "gre" || number(t, greRow, fieldOverhead) != 28 {
			t.Errorf("mtugre1 kind/overhead = %v/%v, want gre/28", greRow[fieldKind], greRow[fieldOverhead])
		}
		if number(t, greRow, fieldPathMTU) != pathClampMTU || number(t, greRow, fieldCeiling) != 1372 || number(t, greRow, fieldRecommended) != 1340 {
			t.Errorf("mtugre1 sizing = %v, want path 1400 ceiling 1372 rec 1340", greRow)
		}
		if number(t, greRow, fieldMSS) != 1300 {
			t.Errorf("mtugre1 mss = %v, want 1300", greRow[fieldMSS])
		}
		if greRow[fieldVerdict] != tunnelVerdictOversized.String() || number(t, greRow, fieldOctets) != 100 {
			t.Errorf("mtugre1 verdict/octets = %v/%v, want oversized/100", greRow[fieldVerdict], greRow[fieldOctets])
		}

		vxRow := rowByInterface(t, rows, "mtuvx10")
		if vxRow[fieldKind] != "vxlan" || number(t, vxRow, fieldOverhead) != 50 {
			t.Errorf("mtuvx10 kind/overhead = %v/%v, want vxlan/50", vxRow[fieldKind], vxRow[fieldOverhead])
		}
		if number(t, vxRow, fieldPathMTU) != pathClampMTU || number(t, vxRow, fieldCeiling) != 1350 || number(t, vxRow, fieldRecommended) != 1318 {
			t.Errorf("mtuvx10 sizing = %v, want path 1400 ceiling 1350 rec 1318", vxRow)
		}
		if _, hasMSS := vxRow[fieldMSS]; hasMSS {
			t.Errorf("mtuvx10 L2 tunnel must omit mss, got %v", vxRow[fieldMSS])
		}
		if vxRow[fieldVerdict] != tunnelVerdictOversized.String() || number(t, vxRow, fieldOctets) != 100 {
			t.Errorf("mtuvx10 verdict/octets = %v/%v, want oversized/100", vxRow[fieldVerdict], vxRow[fieldOctets])
		}

		underlay, ok := doc[fieldUnderlay].(map[string]any)
		if !ok || underlay[fieldAdvice] != underlayCircuitClamped.String() {
			t.Errorf("underlay = %v, want advice=circuit-clamped", underlay)
		}

		commands := strings.Join(stringsOf(t, doc, fieldCommands), "\n")
		for _, want := range []string{
			"set interface tunnel mtugre1 mtu 1340",
			"set interface tunnel mtuvx10 mtu 1318",
			"set interface veth sr0 mtu 1400",
		} {
			if !strings.Contains(commands, want) {
				t.Errorf("commands lack %q:\n%s", want, commands)
			}
		}
	})
}
