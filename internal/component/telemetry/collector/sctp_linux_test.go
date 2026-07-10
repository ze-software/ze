// VALIDATES: the sctp collector parses /proc/net/sctp/snmp and emits the
// absolute CurrEstab gauge plus per-second packet rates across two collects
// (path injected via the c.path seam).
// PREVENTS: a broken key lookup or the absolute-vs-rate handling of CurrEstab on
// the sctp.snmp metric.

//go:build linux

package collector

import (
	"testing"
	"time"
)

func TestSCTPCollect(t *testing.T) {
	const first = "SctpCurrEstab 5\nSctpInSCTPPacks 100\nSctpOutSCTPPacks 50\n"
	// deltas: InSCTPPacks 200, OutSCTPPacks 40; CurrEstab absolute 7.
	const second = "SctpCurrEstab 7\nSctpInSCTPPacks 300\nSctpOutSCTPPacks 90\n"

	dir, path := tmpFile(t, "snmp", first)
	c := newSCTPCollector(time.Second)
	c.path = path
	reg := newRecordingRegistry()
	c.Init(reg, "netdata")
	if err := c.Collect(); err != nil {
		t.Fatalf("first Collect: %v", err)
	}
	writeProcFile(t, dir, "snmp", second)
	if err := c.Collect(); err != nil {
		t.Fatalf("second Collect: %v", err)
	}

	const name = "netdata_sctp_snmp_packets_persec_average"
	if got := reg.gauge(t, name, "sctp.snmp", "SctpCurrEstab", "sctp"); got != 7 {
		t.Errorf("CurrEstab = %v, want 7", got)
	}
	if got := reg.gauge(t, name, "sctp.snmp", "SctpInSCTPPacks", "sctp"); got != 200 {
		t.Errorf("InSCTPPacks = %v, want 200", got)
	}
	if got := reg.gauge(t, name, "sctp.snmp", "SctpOutSCTPPacks", "sctp"); got != 40 {
		t.Errorf("OutSCTPPacks = %v, want 40", got)
	}
}
