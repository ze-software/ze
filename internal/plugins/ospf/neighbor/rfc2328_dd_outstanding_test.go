// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md -- Database Description exchange
// RFC: rfc/short/rfc2328.md -- Section 10.1 (one Database Description outstanding).

package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// masterInExchange brings this router (10.0.0.3, master over 10.0.0.2) into
// Exchange with a Database summary list longer than one DD can carry, so the
// exchange needs a second DD. It returns the table, the sender, the peer, the
// slave's ExStart reply, and the first Exchange DD the master sent (the one
// now outstanding).
type masterExchange struct {
	tbl         *Table
	cfg         InterfaceConfig
	sender      *fakeSender
	peer        types.RouterID
	slaveReply  packet.DBDesc
	outstanding packet.DBDesc
}

func masterInExchange(t *testing.T) masterExchange {
	t.Helper()
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	cfg.RouterID = rid(t, "10.0.0.3")
	cfg.RetransmitInterval = 5
	tbl.ConfigureInterface(cfg)
	db := fakeLSDB{}
	for i := range ddHeaderCapacity(cfg.InterfaceMTU) + 3 {
		h := testHeaderIndex(t, i+1)
		db[h.Key()] = packet.LSA{Header: h}
	}
	tbl.SetLSDB(db)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	if n.State != stateExStart || !n.Master {
		t.Fatalf("setup: state %s master %v, want ExStart as master", n.State, n.Master)
	}
	initial := lastDD(t, sender)
	slaveReply := packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, DDSequence: initial.DDSequence}
	if reason := tbl.HandleDBDesc(cfg.Name, peer, slaveReply); reason != "" {
		t.Fatalf("setup: slave ExStart reply refused: %s", reason)
	}
	if n.State != stateExchange {
		t.Fatalf("setup: state %s, want Exchange", n.State)
	}
	outstanding := lastDD(t, sender)
	if outstanding.DDSequence != initial.DDSequence+1 || outstanding.Flags&packet.DDFlagMore == 0 {
		t.Fatalf("setup: first Exchange DD = %+v, want sequence %d with More set", outstanding, initial.DDSequence+1)
	}
	return masterExchange{tbl: tbl, cfg: cfg, sender: sender, peer: peer, slaveReply: slaveReply, outstanding: outstanding}
}

// sentDDSequences lists the sequence number of every DD sent from index from.
func sentDDSequences(t *testing.T, sender *fakeSender, from int) []uint32 {
	t.Helper()
	var out []uint32
	for i := from; i < len(sender.sent); i++ {
		if p := sentPacket(t, sender, i); p.DBDesc != nil {
			out = append(out, p.DBDesc.DDSequence)
		}
	}
	return out
}

// VALIDATES: the master keeps exactly one DD outstanding. While the slave has
// not acknowledged DD n, every packet the master sends is DD n again (same
// sequence, same flags, same headers), across three RxmtInterval expiries; the
// slave's DD carrying sequence n releases exactly one new DD, n+1, holding the
// rest of the summary list.
//
// RFC requirement: RFC2328-10.1-1 positive -- with DD n outstanding and the summary list not exhausted, three retransmit expiries each resend DD n unchanged (sameDD) and no DD n+1 is sent; the slave's DD acknowledging n releases exactly one DD, n+1, with the remaining headers.
func TestRFC2328MasterHoldsOneOutstandingDD(t *testing.T) {
	m := masterInExchange(t)
	tbl, cfg, sender, peer, outstanding := m.tbl, m.cfg, m.sender, m.peer, m.outstanding
	mark := len(sender.sent)
	for i := 1; i <= 3; i++ {
		now := time.Unix(1, 0).Add(time.Duration(i*int(cfg.RetransmitInterval)+1) * time.Second)
		if got := tbl.Retransmit(now); got != 1 {
			t.Fatalf("retransmit %d sent %d packets, want 1", i, got)
		}
		if resent := lastDD(t, sender); !sameDD(resent, outstanding) {
			t.Fatalf("retransmit %d = %+v, want the outstanding DD %+v", i, resent, outstanding)
		}
	}
	if seqs := sentDDSequences(t, sender, mark); len(seqs) != 3 {
		t.Fatalf("DDs sent while unacknowledged = %v, want three copies of %d", seqs, outstanding.DDSequence)
	}
	ack := packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, DDSequence: outstanding.DDSequence}
	mark = len(sender.sent)
	if reason := tbl.HandleDBDesc(cfg.Name, peer, ack); reason != "" {
		t.Fatalf("slave acknowledgement refused: %s", reason)
	}
	seqs := sentDDSequences(t, sender, mark)
	if len(seqs) != 1 || seqs[0] != outstanding.DDSequence+1 {
		t.Fatalf("DDs sent after the acknowledgement = %v, want exactly one with sequence %d", seqs, outstanding.DDSequence+1)
	}
	next := lastDD(t, sender)
	if len(next.Headers) != 3 {
		t.Fatalf("second DD carries %d headers, want the remaining 3", len(next.Headers))
	}
}

// VALIDATES: nothing but the acknowledgement of the outstanding DD releases
// the next one. A repeat of the slave's previous DD is dropped with nothing
// sent, and a slave DD with a sequence other than the outstanding one restarts
// the exchange with an Init DD. In neither case does DD n+1 leave the master.
//
// RFC requirement: RFC2328-10.1-1 negative -- with DD n outstanding, a duplicate of the slave's previous DD sends nothing, and a slave DD carrying sequence n+2 is refused with SeqNumberMismatch and answered by an Init DD; no DD with sequence n+1 is ever sent.
func TestRFC2328MasterSendsNoSecondDDWithoutAck(t *testing.T) {
	m := masterInExchange(t)
	tbl, cfg, sender, peer, slaveReply, outstanding := m.tbl, m.cfg, m.sender, m.peer, m.slaveReply, m.outstanding
	mark := len(sender.sent)
	if reason := tbl.HandleDBDesc(cfg.Name, peer, slaveReply); reason != "duplicate-drop" {
		t.Fatalf("repeated slave DD reason = %q, want duplicate-drop", reason)
	}
	if len(sender.sent) != mark {
		t.Fatalf("repeated slave DD made the master send %d packets, want none", len(sender.sent)-mark)
	}
	wrong := packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, DDSequence: outstanding.DDSequence + 2}
	if reason := tbl.HandleDBDesc(cfg.Name, peer, wrong); reason != reasonSeqNumberMismatch {
		t.Fatalf("slave DD with sequence %d reason = %q, want %q", wrong.DDSequence, reason, reasonSeqNumberMismatch)
	}
	for i := mark; i < len(sender.sent); i++ {
		p := sentPacket(t, sender, i)
		if p.DBDesc == nil {
			continue
		}
		if p.DBDesc.DDSequence == outstanding.DDSequence+1 && p.DBDesc.Flags&packet.DDFlagInit == 0 {
			t.Fatalf("master sent DD %d although DD %d was never acknowledged", outstanding.DDSequence+1, outstanding.DDSequence)
		}
	}
	if restart := lastDD(t, sender); restart.Flags&packet.DDFlagInit == 0 {
		t.Fatalf("after the mismatch the master sent %+v, want an Init DD restarting the exchange", restart)
	}
}
