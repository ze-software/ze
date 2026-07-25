package flowexport

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
)

func TestIfDirectionFor(t *testing.T) {
	cases := map[string]uint32{
		"full":    IfDirectionFullDuplex,
		"half":    IfDirectionHalfDuplex,
		"":        IfDirectionUnknown,
		"unknown": IfDirectionUnknown,
	}
	for in, want := range cases {
		if got := ifDirectionFor(in); got != want {
			t.Errorf("ifDirectionFor(%q) = %d, want %d", in, got, want)
		}
	}
}

// TestInterfaceCountersFromExtended verifies the extended sFlow if_counters
// fields: rx-multicast maps to IfInMulticastPkts, the sysfs Mbit/s speed scales
// to bit/s, and the duplex string maps to ifDirection. Guards the regression
// where these fields were left zero.
func TestInterfaceCountersFromExtended(t *testing.T) {
	info := &iface.InterfaceInfo{
		Name:    "eth0",
		Index:   3,
		Type:    "device",
		State:   "up",
		Promisc: true,
		Stats: &iface.InterfaceStats{
			RxBytes:     1000,
			RxPackets:   10,
			RxMulticast: 4,
			TxBytes:     2000,
			TxPackets:   20,
		},
	}

	ic := interfaceCountersFrom(info, 10000 /* Mbit/s */, "full")

	if ic.IfSpeed != 10_000_000_000 {
		t.Errorf("IfSpeed = %d, want 10000000000 (10 Gbit/s in bit/s)", ic.IfSpeed)
	}
	if ic.IfDirection != IfDirectionFullDuplex {
		t.Errorf("IfDirection = %d, want %d", ic.IfDirection, IfDirectionFullDuplex)
	}
	if ic.IfInMulticastPkts != 4 {
		t.Errorf("IfInMulticastPkts = %d, want 4", ic.IfInMulticastPkts)
	}
	if ic.IfType != 6 {
		t.Errorf("IfType = %d, want 6 (ethernetCsmacd)", ic.IfType)
	}
	if ic.IfStatus != IfStatusAdminUp|IfStatusOperUp {
		t.Errorf("IfStatus = %d, want %d (up)", ic.IfStatus, IfStatusAdminUp|IfStatusOperUp)
	}
	if ic.IfPromiscuousMode != 1 {
		t.Errorf("IfPromiscuousMode = %d, want 1", ic.IfPromiscuousMode)
	}
}

// TestInterfaceCountersFromUnknownSpeed verifies a virtual / down link whose
// sysfs speed and duplex are unknown leaves ifSpeed and ifDirection zero rather
// than reporting a bogus rate.
func TestInterfaceCountersFromUnknownSpeed(t *testing.T) {
	info := &iface.InterfaceInfo{
		Name:  "veth0",
		Index: 9,
		Type:  "veth",
		State: "down",
		Stats: &iface.InterfaceStats{},
	}
	ic := interfaceCountersFrom(info, 0 /* unknown speed */, "" /* unknown duplex */)
	if ic.IfSpeed != 0 {
		t.Errorf("IfSpeed = %d, want 0 for unknown speed", ic.IfSpeed)
	}
	if ic.IfDirection != IfDirectionUnknown {
		t.Errorf("IfDirection = %d, want %d (unknown)", ic.IfDirection, IfDirectionUnknown)
	}
	if ic.IfStatus != IfStatusAdminUp {
		t.Errorf("IfStatus = %d, want %d (admin up, oper down)", ic.IfStatus, IfStatusAdminUp)
	}
}

// VALIDATES: runEngine refuses to start (nonzero exit) when conn is not a
// same-process (DirectBridge-carrying) connection -- iface.SubscribeCollectNotify
// (register.go) registers a callback into iface's package-level subscriber
// list as a plain Go function call, which only reaches the engine's real
// rate tracker when this plugin shares process memory with it. It is
// flow-export's only counter data source, unconditional, no fallback, so an
// external flow-export would silently never export a single datagram, with
// no error anywhere. A plain net.Pipe() end matches exactly what an external
// plugin's non-bridged conn looks like from the SDK's perspective (see
// sdk.Plugin.IsInternal).
//
// The elapsed-time assertion matters: without the guard, runEngine falls
// through to p.Run(ctx, ...), which also eventually returns a nonzero exit
// against a non-responsive plain net.Pipe() end -- but only after the SDK's
// handshake/registration protocol times out (tens of seconds), not because
// external mode was detected. A refuse-immediately guard must return well
// under that timeout.
// PREVENTS: flow-export configured `plugin { external flow-export { ... } }`
// silently accepting every config commit while never exporting any counters.
func TestRunEngine_RefusesExternalProcess(t *testing.T) {
	pluginEnd, engineEnd := net.Pipe()
	t.Cleanup(func() {
		pluginEnd.Close() //nolint:errcheck // test cleanup
		engineEnd.Close() //nolint:errcheck // test cleanup
	})

	start := time.Now()
	code := runEngine(pluginEnd)
	elapsed := time.Since(start)

	if code != 1 {
		t.Fatalf("runEngine(external conn) = %d, want 1", code)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("runEngine(external conn) took %s, want an immediate refusal (< 2s) -- suggests it fell through to p.Run()'s handshake timeout instead of refusing at the IsInternal() guard", elapsed)
	}
}

// RFC requirement: SFLOW-V5-x-15 positive -- if_counters are cumulative since boot: interfaceCountersFrom copies the raw kernel counters straight through (register.go:349-357) with no per-poll differencing. The 64-bit octet counters are carried without truncation, and calling the (stateless) converter twice on the same input yields identical values -- a differencing implementation would report zero on the second identical call.
func TestInterfaceCountersFromCumulative(t *testing.T) {
	info := &iface.InterfaceInfo{
		Name:  "eth0",
		Index: 2,
		Type:  "device",
		State: "up",
		Stats: &iface.InterfaceStats{
			RxBytes:   5_000_000_000, // > 2^32: must survive as a 64-bit cumulative value
			RxPackets: 4_000_000,
			TxBytes:   9_000_000_000,
			TxPackets: 7_000_000,
		},
	}

	ic := interfaceCountersFrom(info, 1000, "full")

	// Raw cumulative copy, no baseline subtraction and no 32-bit truncation.
	if ic.IfInOctets != 5_000_000_000 {
		t.Errorf("IfInOctets = %d, want 5000000000 (raw cumulative, untruncated)", ic.IfInOctets)
	}
	if ic.IfOutOctets != 9_000_000_000 {
		t.Errorf("IfOutOctets = %d, want 9000000000 (raw cumulative, untruncated)", ic.IfOutOctets)
	}
	if ic.IfInUcastPkts != 4_000_000 {
		t.Errorf("IfInUcastPkts = %d, want 4000000 (raw cumulative)", ic.IfInUcastPkts)
	}
	if ic.IfOutUcastPkts != 7_000_000 {
		t.Errorf("IfOutUcastPkts = %d, want 7000000 (raw cumulative)", ic.IfOutUcastPkts)
	}

	// No differencing: a second conversion of the same counters is identical, not a delta.
	ic2 := interfaceCountersFrom(info, 1000, "full")
	if ic2.IfInOctets != ic.IfInOctets || ic2.IfOutOctets != ic.IfOutOctets {
		t.Errorf("second conversion differs (in %d->%d, out %d->%d); counters must stay cumulative, not be differenced",
			ic.IfInOctets, ic2.IfInOctets, ic.IfOutOctets, ic2.IfOutOctets)
	}
}

func TestIfTypeFor(t *testing.T) {
	cases := map[string]uint32{
		"device":    6,
		"bridge":    209,
		"vlan":      135,
		"veth":      53,
		"dummy":     53,
		"gre":       131,
		"wireguard": 131,
		"sit":       131,
		"":          1,
		"weirdkind": 1,
	}
	for in, want := range cases {
		if got := ifTypeFor(in); got != want {
			t.Errorf("ifTypeFor(%q) = %d, want %d", in, got, want)
		}
	}
}
