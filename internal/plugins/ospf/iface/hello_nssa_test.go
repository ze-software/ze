// VALIDATES: spec-ospf-11 RFC 2328 sec 3.6 / RFC 3101 -- a stub interface expects the
// E-bit CLEAR and drops a Hello with the E-bit set; an NSSA interface expects the
// E-bit CLEAR and the N-bit SET and drops a Hello whose N-bit does not match. The
// adjacency never forms on an option mismatch (guide trap #11).
// PREVENTS: regressions where a stub/NSSA adjacency forms across an external-capability
// mismatch, injecting Type 5 reachability into an area that forbids it.
package iface

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func TestOSPFStubEbitMismatch(t *testing.T) {
	cfg := baseConfig(t)
	cfg.AreaType = AreaStub
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")

	// Default helloFor sets the E-bit; a stub interface expects it clear -> dropped.
	h := helloFor(cfg)
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != DropReasonOptionsE {
		t.Fatalf("stub E-bit-set Hello reason = %q, want %s", got, DropReasonOptionsE)
	}
	if _, ok := ifc.neighbors[peer]; ok {
		t.Fatal("a stub adjacency formed across an E-bit mismatch")
	}

	// A Hello with the E-bit clear matches the stub interface.
	h = helloFor(cfg)
	h.Options = 0
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != "" {
		t.Fatalf("stub E-bit-clear Hello reason = %q, want accepted", got)
	}
}

func TestOSPFNSSANbitMismatch(t *testing.T) {
	cfg := baseConfig(t)
	cfg.AreaType = AreaNSSA
	ifc := New(cfg, &fakeSender{}, NopMetrics())
	peer := rid(t, "10.0.0.2")

	// NSSA expects E clear + N set; a matching Hello is accepted.
	// RFC requirement: RFC3101-2.1-1 positive -- a Hello whose N-bit and E-bit both match the
	// NSSA area type (N set, E clear) passes the option-agreement check and is accepted.
	// RFC requirement: RFC3101-2.1-2 positive -- adjacency is allowed when the received Hello's
	// N-bit agrees with the NSSA interface (N set).
	// RFC requirement: RFC3101-x-1 positive -- an NSSA Hello with the N-bit set and the E-bit
	// clear is accepted, encoding the "N set => E clear" option pairing.
	h := helloFor(cfg)
	h.Options = types.OptionNP
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != "" {
		t.Fatalf("nssa matching Hello reason = %q, want accepted", got)
	}

	// N-bit clear (E clear) -> N mismatch.
	// RFC requirement: RFC3101-2.1-2 negative -- an N-bit-clear Hello on an NSSA interface
	// disagrees on the N-bit and is dropped (DropReasonOptionsN), so no adjacency forms.
	h = helloFor(cfg)
	h.Options = 0
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != DropReasonOptionsN {
		t.Fatalf("nssa N-bit-clear Hello reason = %q, want %s", got, DropReasonOptionsN)
	}

	// E-bit set -> E mismatch (still rejected before the N check matters).
	// RFC requirement: RFC3101-2.1-1 negative -- a Hello whose E-bit does not match the NSSA
	// area type (E set) fails the option-agreement check and is dropped before adjacency.
	// RFC requirement: RFC3101-x-1 negative -- an NSSA Hello with the E-bit set (violating
	// "N set => E clear") is rejected (DropReasonOptionsE).
	h = helloFor(cfg)
	h.Options = types.OptionE
	if got := ifc.ReceiveHello(peer, h, time.Now()); got != DropReasonOptionsE {
		t.Fatalf("nssa E-bit-set Hello reason = %q, want %s", got, DropReasonOptionsE)
	}
}
