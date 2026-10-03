// Design: docs/architecture/flowexport/flow-export-1-counter-export.md -- IPFIX Template timing
// Related: export_rfc7011_test.go -- the per-flow Template's send-time Export Time
//
// VALIDATES: the counter Template carries its send second as Export Time, and a
// Template refresh after the wall clock steps backwards never carries an Export
// Time earlier than the Template action before it, on the counter and the flow
// encoder alike.
// PREVENTS: a collector ordering Template actions by Export Time and seeing a
// refresh land before the definition it repeats.

package ipfix

import (
	"testing"
	"testing/synctest"
	"time"
)

// RFC requirement: RFC7011-8.2-1 positive -- the counter Template message carries
// the wall-clock second of its send as Export Time, so a collector can order the
// counter Template action by it.
func TestRFC7011CounterTemplateExportTimeIsSendTime(t *testing.T) {
	s, pc := captureSender(t)
	enc := NewCounterEncoder(1)
	before := uint32(time.Now().Unix())
	if err := enc.EncodeTemplate(s); err != nil {
		t.Fatal(err)
	}
	after := uint32(time.Now().Unix())
	if got := exportTimeOf(readDatagram(t, pc)); got < before || got > after {
		t.Fatalf("counter Template Export Time = %d, want within [%d, %d]", got, before, after)
	}
}

// RFC requirement: RFC7011-8.2-1 negative -- after the wall clock steps back, a
// refreshed Template (counter Template 256, flow Templates 257 and 258) is sent
// with the Export Time of the Template action before it, never an earlier one,
// so the Template actions of each encoder stay in Export Time order.
func TestRFC7011TemplateRefreshAfterClockRollbackKeepsOrder(t *testing.T) {
	s, pc := captureSender(t)
	counter := NewCounterEncoder(1)
	flow := NewFlowEncoder(1)
	if err := counter.EncodeTemplate(s); err != nil {
		t.Fatal(err)
	}
	if err := flow.EncodeFlowTemplate(s); err != nil {
		t.Fatal(err)
	}
	first := make([]uint32, 3)
	for i := range first {
		first[i] = exportTimeOf(readDatagram(t, pc))
	}

	// synctest moves time.Now back to 2000-01-01 inside the bubble. Only the
	// synchronous UDP sends run there; the socket reads stay outside it.
	synctest.Test(t, func(t *testing.T) {
		if now := uint32(time.Now().Unix()); now >= first[0] {
			t.Fatalf("clock did not step back: now=%d first Template=%d", now, first[0])
		}
		if err := counter.EncodeTemplate(s); err != nil {
			t.Fatal(err)
		}
		if err := flow.EncodeFlowTemplate(s); err != nil {
			t.Fatal(err)
		}
	})

	for i, name := range []string{"counter 256", "flow 257", "flow 258"} {
		got := exportTimeOf(readDatagram(t, pc))
		if got < first[i] {
			t.Errorf("%s: refreshed Template Export Time %d is before the earlier Template action at %d", name, got, first[i])
		}
	}
}
