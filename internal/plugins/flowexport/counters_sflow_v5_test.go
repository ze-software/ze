// sFlow v5 conformance tests for the availability of each if_counters field
// Ze fills from the kernel: a counter Linux does not expose carries the
// max-value sentinel on every poll, while available counters keep their values.

package flowexport

import (
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// pollCounters is one poll of eth0 with the given kernel statistics.
func pollCounters(rxBytes, rxPackets, txBytes, txPackets uint64) InterfaceCounters {
	info := &iface.InterfaceInfo{
		Name: "eth0", Index: 3, Type: "device", State: "up",
		Stats: &iface.InterfaceStats{
			RxBytes: rxBytes, RxPackets: rxPackets, RxMulticast: 1,
			TxBytes: txBytes, TxPackets: txPackets,
		},
	}
	return interfaceCountersFrom(info, 1000, "full")
}

// unavailableFields names the if_counters fields rtnl_link_stats64 never
// carries, read off one poll.
func unavailableFields(ic *InterfaceCounters) map[string]uint32 {
	return map[string]uint32{
		"ifInBroadcastPkts":  ic.IfInBroadcastPkts,
		"ifInUnknownProtos":  ic.IfInUnknownProtos,
		"ifOutMulticastPkts": ic.IfOutMulticastPkts,
		"ifOutBroadcastPkts": ic.IfOutBroadcastPkts,
	}
}

// availableFields names the 32-bit if_counters fields the kernel exposes.
func availableFields(ic *InterfaceCounters) map[string]uint32 {
	return map[string]uint32{
		"ifInUcastPkts":     ic.IfInUcastPkts,
		"ifInMulticastPkts": ic.IfInMulticastPkts,
		"ifInDiscards":      ic.IfInDiscards,
		"ifInErrors":        ic.IfInErrors,
		"ifOutUcastPkts":    ic.IfOutUcastPkts,
		"ifOutDiscards":     ic.IfOutDiscards,
		"ifOutErrors":       ic.IfOutErrors,
	}
}

// RFC requirement: SFLOW-V5-x-16 positive -- the four if_counters fields the kernel does not expose (ifInBroadcastPkts, ifInUnknownProtos, ifOutMulticastPkts, ifOutBroadcastPkts) carry the max value 0xFFFFFFFF, and on two polls with different kernel statistics the same four fields carry it.
// RFC requirement: SFLOW-V5-x-32 positive -- across two polls of one interface with different kernel statistics, every counter unavailable on the first poll is unavailable on the second, so availability is constant for the session.
func TestSFlowV5UnavailableCountersCarrySentinelEveryPoll(t *testing.T) {
	first := pollCounters(1000, 10, 2000, 20)
	second := pollCounters(5000, 50, 9000, 90)
	for name, got := range unavailableFields(&first) {
		if got != CounterUnavailable {
			t.Errorf("%s = %#x on the first poll, want the sentinel %#x", name, got, CounterUnavailable)
		}
	}
	for name, got := range unavailableFields(&second) {
		if got != CounterUnavailable {
			t.Errorf("%s = %#x on the second poll, want the sentinel %#x", name, got, CounterUnavailable)
		}
	}
}

// RFC requirement: SFLOW-V5-x-16 negative -- with zero and small kernel counts, available fields retain those values instead of the unavailable sentinel.
// RFC requirement: SFLOW-V5-x-32 negative -- fields available on the first poll remain available on a later zero-valued poll.
func TestSFlowV5AvailableCountersNeverTurnUnavailable(t *testing.T) {
	first := pollCounters(1000, 10, 2000, 20)
	second := pollCounters(0, 0, 0, 0)
	if first.IfInOctets != 1000 || first.IfOutOctets != 2000 || first.IfInUcastPkts != 10 || first.IfOutUcastPkts != 20 {
		t.Fatalf("first poll = in %d/%d out %d/%d, want the kernel values 1000/10 2000/20", first.IfInOctets, first.IfInUcastPkts, first.IfOutOctets, first.IfOutUcastPkts)
	}
	if second.IfInOctets != 0 || second.IfInUcastPkts != 0 {
		t.Fatalf("second poll = in %d/%d, want the kernel's zero", second.IfInOctets, second.IfInUcastPkts)
	}
	for poll, ic := range map[string]*InterfaceCounters{"first": &first, "second": &second} {
		for name, got := range availableFields(ic) {
			if got == CounterUnavailable {
				t.Errorf("%s reads the unavailable sentinel on the %s poll", name, poll)
			}
		}
	}
}

// TestRawPacketTotalsKeepFullWidth prevents sFlow's 32-bit representation
// and its unavailable sentinel from changing the totals used by IPFIX.
func TestRawPacketTotalsKeepFullWidth(t *testing.T) {
	inPackets := uint64(^uint32(0))
	outPackets := inPackets + 7
	ic := pollCounters(1, inPackets, 2, outPackets)
	if ic.InPackets != inPackets || ic.OutPackets != outPackets {
		t.Fatalf("packet totals = %d/%d, want %d/%d", ic.InPackets, ic.OutPackets, inPackets, outPackets)
	}
	if ic.IfInUcastPkts != ^uint32(0) {
		t.Fatalf("available sFlow counter = %d, want legitimate maximum", ic.IfInUcastPkts)
	}
}
