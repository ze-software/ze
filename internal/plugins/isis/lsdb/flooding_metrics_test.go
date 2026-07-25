// Design: plan/spec-isis-7-flooding.md -- flooding/SNP Prometheus metrics (owner isis-7).
//
// VALIDATES: this spec registers EXACTLY the umbrella canonical rows it owns
// (ze_isis_lsps_received_total, ze_isis_lsps_transmitted_total,
// ze_isis_csnp_sent_total, ze_isis_csnp_received_total, ze_isis_psnp_sent_total,
// ze_isis_psnp_received_total, ze_isis_srm_resends_total,
// ze_isis_lsps_dropped_total{level,reason}); each labeled by level (the dropped
// one also by reason); and the flooding/SNP paths increment them.
// PREVENTS: bare isis_* names or registering another owner's series.

package lsdb

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/isis/packet"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// TestISISFloodingMetricsRegisterExactSeries drives every flooding/SNP path that
// owns a counter so each labeled series appears in the scrape, and asserts there
// are no bare isis_* names. A CounterVec exposes nothing until a label combo is
// observed, so all eight rows must be exercised here.
func TestISISFloodingMetricsRegisterExactSeries(t *testing.T) {
	reg := metrics.NewPrometheusRegistry()
	d := New(nil)
	const cIn, cOut CircuitID = 1, 2
	rec := &recordingTx{}
	f := NewFlooder(d, rec.tx, staticCircuits(
		l1l2Circuit("in", cIn),
		l1l2Circuit("out", cOut),
	))
	f.SetMetrics(reg)

	// 1) lsps_received_total + lsps_transmitted_total + srm_resends_total: receive a
	//    newer LSP (bumps received, arms SRM on cOut), then flood ticks on a LAN
	//    circuit transmit it (bumps transmitted). The FIRST tick is the normal flood
	//    (no resend); the SECOND tick, with SRM still set (unacknowledged), is the
	//    re-send that bumps srm_resends (ISO/IEC 10589 clause 7.3.15.1: only the
	//    unacknowledged retransmissions are counted, not the first send).
	id := lspID(1, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 5, 1000, nil)
	f.ReceiveLSP(cIn, false, lsp, raw)
	f.FloodTick() // LAN send: transmitted++ (first send, not a resend)
	f.FloodTick() // LAN re-send (SRM still set): transmitted++ and srm_resends++

	// 2) lsps_dropped_total{level,reason}: a lower-seq LSP is dropped (reason=older).
	older, olderRaw := buildLSP(t, packet.PDUTypeL1LSP, id, 2, 1000, nil)
	f.ReceiveLSP(cIn, false, older, olderRaw)

	// 3) csnp_received_total + psnp build path. Receive a CSNP (bumps csnp_received).
	csnp := buildMetricsCSNP(t, id)
	f.ReceiveCSNP(cIn, csnp)

	// 4) csnp_sent_total: send a CSNP on a circuit.
	f.SendCSNP(l1l2Circuit("in", cIn), Level1, types.NewSourceID(testSys(1), 0))

	// 5) psnp_received_total: receive a PSNP.
	psnp := buildMetricsPSNP(t, id)
	f.ReceivePSNP(cIn, psnp)

	// 6) psnp_sent_total: arm SSN on a held LSP so a PSNP is built and sent.
	d.SetSSN(Level1, id, cIn)
	f.SendPSNP(l1l2Circuit("in", cIn), Level1, types.NewSourceID(testSys(1), 0))

	out := scrape(t, reg)
	for _, want := range []string{
		"ze_isis_lsps_received_total",
		"ze_isis_lsps_transmitted_total",
		"ze_isis_csnp_sent_total",
		"ze_isis_csnp_received_total",
		"ze_isis_psnp_sent_total",
		"ze_isis_psnp_received_total",
		"ze_isis_srm_resends_total",
		"ze_isis_lsps_dropped_total",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("metric %q not exposed", want)
		}
	}
	// The dropped series carries both a level and a reason label.
	if !strings.Contains(out, `ze_isis_lsps_dropped_total{level="l1",reason="older"}`) {
		t.Errorf("dropped series missing level+reason labels:\n%s", out)
	}
	if !strings.Contains(out, `level="l1"`) {
		t.Errorf("expected level=\"l1\" label in output:\n%s", out)
	}
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, "isis_") {
			t.Errorf("bare isis_* metric name: %q", line)
		}
	}
}

// TestISISFloodingSetMetricsRace exercises the data race between SetMetrics
// (re-binding the flooding/SNP metric handles) and the hot flooding/SNP paths
// that read those handles (ReceiveLSP, FloodTick, ReceiveCSNP, ReceivePSNP).
// The handles are held behind an atomic pointer (Flooder.mp): SetMetrics swaps
// the whole set with one atomic store; the read paths load it with one atomic
// load, so a reader observes either the old or the new set in full, never a torn
// interface value. Run under `go test -race` this fails if the handles are read
// as plain fields and passes with the atomic pointer. Defense-in-depth: in
// production SetMetrics runs once before any circuit starts, but the field stays
// race-free regardless of future call sites (finding D5).
func TestISISFloodingSetMetricsRace(t *testing.T) {
	d := New(nil)
	const cIn CircuitID = 1
	rec := &recordingTx{}
	f := NewFlooder(d, rec.tx, staticCircuits(l1l2Circuit("in", cIn), l1l2Circuit("out", 2)))

	id := lspID(1, 0)
	lsp, raw := buildLSP(t, packet.PDUTypeL1LSP, id, 5, 1000, nil)
	csnp := buildMetricsCSNP(t, id)
	psnp := buildMetricsPSNP(t, id)

	const rounds = 200
	done := make(chan struct{})

	// Writer: repeatedly rebind the metric handles (the SetMetrics path).
	go func() {
		for range rounds {
			reg := metrics.NewPrometheusRegistry()
			f.SetMetrics(reg)
		}
		close(done)
	}()

	// Readers: drive every hot path that loads a metric handle, concurrently with
	// the rebinds. ReceiveLSP (lspsRecv), FloodTick (lspsTx/srmResend), ReceiveCSNP
	// (csnpRecv), ReceivePSNP (psnpRecv).
	for range rounds {
		f.ReceiveLSP(cIn, false, lsp, raw)
		f.FloodTick()
		f.ReceiveCSNP(cIn, csnp)
		f.ReceivePSNP(cIn, psnp)
	}
	<-done
}

// buildMetricsCSNP builds an L1 CSNP listing one entry for id (used to bump the
// csnp_received counter).
func buildMetricsCSNP(t *testing.T, id types.LSPID) *packet.CSNP {
	t.Helper()
	c := packet.CSNP{
		PDUType:    packet.PDUTypeL1CSNP,
		SourceID:   types.NewSourceID(testSys(2), 0),
		StartLSPID: minLSPID(),
		EndLSPID:   maxLSPID(),
		TLVs: []packet.TLV{lspEntriesTLV([]packet.LSPEntry{
			{LSPID: id, SequenceNumber: 5, RemainingLifetime: 1000},
		})},
	}
	buf := make([]byte, c.EncodedLen())
	n := c.WriteTo(buf, 0)
	out, err := packet.DecodePDU(buf[:n])
	if err != nil {
		t.Fatalf("buildMetricsCSNP: %v", err)
	}
	return out.CSNP
}

// buildMetricsPSNP builds an L1 PSNP listing one entry for id (used to bump the
// psnp_received counter).
func buildMetricsPSNP(t *testing.T, id types.LSPID) *packet.PSNP {
	t.Helper()
	p := packet.PSNP{
		PDUType:  packet.PDUTypeL1PSNP,
		SourceID: types.NewSourceID(testSys(2), 0),
		TLVs: []packet.TLV{lspEntriesTLV([]packet.LSPEntry{
			{LSPID: id, SequenceNumber: 5, RemainingLifetime: 1000},
		})},
	}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	out, err := packet.DecodePDU(buf[:n])
	if err != nil {
		t.Fatalf("buildMetricsPSNP: %v", err)
	}
	return out.PSNP
}
