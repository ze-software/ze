// VALIDATES: RFC 2328 Section 10.6 and 10.8 slave-side Database Description
// processing -- the slave answers each master DD with one of its own, repeats its
// last DD when the master's DD is a duplicate, and refuses a DD whose sequence
// number is not the next one.
// PREVENTS: a slave answering a duplicate with a fresh packet (which advances the
// exchange out of step) and a slave accepting an out-of-sequence DD.
package neighbor

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

func masterDD(seq uint32, flags uint8) packet.DBDesc {
	return packet.DBDesc{InterfaceMTU: 1500, Options: types.OptionE, Flags: flags, DDSequence: seq}
}

func lastDD(t *testing.T, sender *fakeSender) packet.DBDesc {
	t.Helper()
	p := sentPacket(t, sender, len(sender.sent)-1)
	if p.DBDesc == nil {
		t.Fatalf("last sent packet is not a Database Description: %+v", p)
	}
	return *p.DBDesc
}

// RFC requirement: RFC2328-10.6-1 positive -- as slave, the router replies to the master's Database Description with one of its own carrying the master's sequence number, and answers an exact duplicate of that master DD by repeating the last DD it sent, byte for byte the same sequence and flags (handleDBDesc sameDD branch and resendLastDDLocked, dd.go).
func TestRFC2328SlaveRepliesAndRepeatsOnDuplicate(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	first := masterDD(99, packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster)
	if reason := tbl.HandleDBDesc(cfg.Name, peer, first); reason != "" {
		t.Fatalf("master's first DD: %s", reason)
	}
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	if n.Master {
		t.Fatal("this router is master, want slave (peer Router ID is higher)")
	}
	reply := lastDD(t, sender)
	if reply.DDSequence != 99 || reply.Flags&packet.DDFlagMaster != 0 {
		t.Fatalf("slave reply = %+v, want the master's sequence 99 with the MS bit clear", reply)
	}
	sentBefore := len(sender.sent)
	if reason := tbl.HandleDBDesc(cfg.Name, peer, first); reason != "duplicate-resend" {
		t.Fatalf("duplicate master DD reason = %q, want duplicate-resend", reason)
	}
	if len(sender.sent) != sentBefore+1 {
		t.Fatalf("packets after the duplicate = %d, want exactly one more", len(sender.sent)-sentBefore)
	}
	repeat := lastDD(t, sender)
	if !sameDD(repeat, reply) {
		t.Fatalf("repeat = %+v, want the last DD sent %+v", repeat, reply)
	}
}

// RFC requirement: RFC2328-10.6-1 negative -- as slave, a master Database Description whose sequence number is not the next expected one is not processed in sequence: it is refused with the SeqNumberMismatch reason, the adjacency falls back to ExStart, and the slave does not repeat its last DD for it (handleExchangeDDLocked, dd.go).
func TestRFC2328SlaveRefusesOutOfSequenceDD(t *testing.T) {
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if reason := tbl.HandleDBDesc(cfg.Name, peer, masterDD(99, packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster)); reason != "" {
		t.Fatalf("master's first DD: %s", reason)
	}
	reply := lastDD(t, sender)
	if reason := tbl.HandleDBDesc(cfg.Name, peer, masterDD(101, packet.DDFlagMore|packet.DDFlagMaster)); reason != reasonSeqNumberMismatch {
		t.Fatalf("out-of-sequence DD reason = %q, want %q", reason, reasonSeqNumberMismatch)
	}
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	if n.State != stateExStart {
		t.Fatalf("state after the out-of-sequence DD = %s, want ExStart", n.State)
	}
	if last := lastDD(t, sender); sameDD(last, reply) || last.Flags&packet.DDFlagInit == 0 {
		t.Fatalf("last packet = %+v, want a fresh Init DD restarting the exchange, never a repeat of %+v", last, reply)
	}
}
