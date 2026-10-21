// Design: docs/architecture/ospf/README.md -- Database Description exchange
// Related: dd.go -- handleExchangeDDLocked, the Exchange-state slave branch.
//
// VALIDATES: RFC 2328 Section 10.6 and 10.8 on the slave in Exchange state: the master's DD
// whose sequence number is one more than the slave's is processed in sequence, and the
// slave answers it with a Database Description of its own carrying that sequence number.
// PREVENTS: a slave that refuses the next DD in sequence (the exchange never completes)
// or accepts it in silence (the master retransmits until the adjacency fails).
package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// RFC requirement: RFC2328-10.6-2 positive -- as slave in Exchange state, the master's DD carrying the next sequence number (100 after 99) is processed in sequence: it is accepted with no SeqNumberMismatch, the adjacency stays in Exchange, and the slave's DD sequence number becomes 100 (handleExchangeDDLocked, dd.go).
// RFC requirement: RFC2328-10.6-3 positive -- as slave in Exchange state, the master's accepted next-in-sequence DD (100) draws exactly one Database Description in reply, carrying sequence 100 with the MS and I bits clear (handleExchangeDDLocked, dd.go).
func TestRFC2328SlaveAcceptsAndAnswersNextDDInExchange(t *testing.T) {
	// Goal: drive the Exchange-state slave branch with the next DD in sequence. Method: the
	// master's Init DD at 99 moves the slave to Exchange; the DD at 100 is then fed and the
	// slave's state, stored sequence and transmitted reply are checked.
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if reason := tbl.HandleDBDesc(cfg.Name, peer, masterDD(99, packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster)); reason != "" {
		t.Fatalf("master's first DD: %s", reason)
	}
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	if n.Master {
		t.Fatal("this router is master, want slave (peer Router ID is higher)")
	}
	if n.State != stateExchange {
		t.Fatalf("state after the master's Init DD = %s, want Exchange", n.State)
	}
	sentBefore := len(sender.sent)

	if reason := tbl.HandleDBDesc(cfg.Name, peer, masterDD(100, packet.DDFlagMore|packet.DDFlagMaster)); reason != "" {
		t.Fatalf("next-in-sequence DD 100 refused with %q, want it processed", reason)
	}
	n, _ = tbl.lookupLocked(cfg.Name, peer)
	if n.State != stateExchange {
		t.Fatalf("state after DD 100 = %s, want Exchange (the master set the M bit)", n.State)
	}
	if n.DDSequence != 100 {
		t.Fatalf("slave DD sequence = %d, want 100, the master's", n.DDSequence)
	}
	if got := len(sender.sent) - sentBefore; got != 1 {
		t.Fatalf("packets sent for DD 100 = %d, want exactly one reply", got)
	}
	reply := lastDD(t, sender)
	if reply.DDSequence != 100 {
		t.Fatalf("slave reply sequence = %d, want 100", reply.DDSequence)
	}
	if reply.Flags&packet.DDFlagMaster != 0 {
		t.Fatalf("slave reply flags %#x carry MS, want it clear", reply.Flags)
	}
	if reply.Flags&packet.DDFlagInit != 0 {
		t.Fatalf("slave reply flags %#x carry I, want it clear in Exchange", reply.Flags)
	}
}
