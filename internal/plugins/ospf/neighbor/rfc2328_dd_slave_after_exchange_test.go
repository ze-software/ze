// Design: docs/architecture/ospf/ospf-6-neighbor-nsm.md -- Database Description exchange
// RFC: rfc/short/rfc2328.md -- Section 10.8 (the slave keeps its last DD after Exchange).

package neighbor

import (
	"slices"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// slaveAfterExchange runs a complete exchange as slave (10.0.0.1 under master
// 10.0.0.2) and returns the table, the sender, the peer, the master's final DD
// and the slave's reply to it. When want is Loading, the master's final DD
// lists one LSA the slave's empty database lacks, so the slave requests it and
// stops in Loading; when want is Full, the final DD lists nothing.
type slaveExchange struct {
	tbl         *Table
	cfg         InterfaceConfig
	sender      *fakeSender
	peer        types.RouterID
	masterFinal packet.DBDesc
	slaveFinal  packet.DBDesc
}

func slaveAfterExchange(t *testing.T, want state) slaveExchange {
	t.Helper()
	tbl, cfg := testTable(t, types.NetworkPointToPoint)
	tbl.SetLSDB(fakeLSDB{})
	sender := &fakeSender{}
	tbl.SetSender(sender)
	peer := rid(t, "10.0.0.2")
	_ = tbl.Hello(hello(cfg, peer, true, time.Unix(1, 0)))
	if reason := tbl.HandleDBDesc(cfg.Name, peer, masterDD(99, packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster)); reason != "" {
		t.Fatalf("setup: master's Init DD refused: %s", reason)
	}
	final := masterDD(100, packet.DDFlagMaster)
	if want == stateLoading {
		final.Headers = []packet.LSAHeader{testHeaderIndex(t, 7)}
	}
	if reason := tbl.HandleDBDesc(cfg.Name, peer, final); reason != "" {
		t.Fatalf("setup: master's final DD refused: %s", reason)
	}
	n, _ := tbl.lookupLocked(cfg.Name, peer)
	if n.Master || n.State != want {
		t.Fatalf("setup: master %v state %s, want slave in %s", n.Master, n.State, want)
	}
	var slaveFinal packet.DBDesc
	found := false
	for i := range slices.Backward(sender.sent) {
		if p := sentPacket(t, sender, i); p.DBDesc != nil {
			slaveFinal = *p.DBDesc
			found = true
			break
		}
	}
	if !found || slaveFinal.DDSequence != 100 {
		t.Fatalf("setup: slave's reply to DD 100 = %+v (found %v), want sequence 100", slaveFinal, found)
	}
	return slaveExchange{tbl: tbl, cfg: cfg, sender: sender, peer: peer, masterFinal: final, slaveFinal: slaveFinal}
}

// VALIDATES: after the exchange ends, in Loading and in Full, the slave still
// answers a duplicate of the master's last DD by sending its own last DD again,
// unchanged. The duplicate arrives once at once and once RouterDeadInterval - 1
// seconds later, so a slave that freed its last DD on leaving Exchange, or
// before RouterDeadInterval has passed, has nothing to send and fails.
//
// RFC requirement: RFC2328-10.6-1 positive -- in Loading and in Full the slave answers a duplicate of the master's last DD with exactly one packet, its own last DD (sameDD), both straight after the exchange and RouterDeadInterval - 1 seconds later, and stays in the same state.
func TestRFC2328SlaveResendsLastDDInLoadingAndFull(t *testing.T) {
	for _, want := range []state{stateLoading, stateFull} {
		t.Run(want.String(), func(t *testing.T) {
			s := slaveAfterExchange(t, want)
			for _, after := range []int{0, int(s.cfg.DeadInterval) - 1} {
				s.tbl.now = func() time.Time { return time.Unix(1, 0).Add(time.Duration(after) * time.Second) }
				mark := len(s.sender.sent)
				if reason := s.tbl.HandleDBDesc(s.cfg.Name, s.peer, s.masterFinal); reason != "duplicate-resend" {
					t.Fatalf("duplicate %d s after the exchange: reason %q, want duplicate-resend", after, reason)
				}
				if len(s.sender.sent) != mark+1 {
					t.Fatalf("duplicate %d s after the exchange made the slave send %d packets, want 1", after, len(s.sender.sent)-mark)
				}
				if again := lastDD(t, s.sender); !sameDD(again, s.slaveFinal) {
					t.Fatalf("duplicate %d s after the exchange answered %+v, want the slave's last DD %+v", after, again, s.slaveFinal)
				}
				n, _ := s.tbl.lookupLocked(s.cfg.Name, s.peer)
				if n.State != want {
					t.Fatalf("state after the duplicate = %s, want %s", n.State, want)
				}
			}
		})
	}
}

// VALIDATES: only a duplicate of the master's LAST DD earns the repeat. In
// Loading and in Full, a repeat of the master's earlier Init DD, or a DD with
// a new sequence number, is no duplicate: the slave does not resend its last
// DD, it records SeqNumberMismatch and restarts the exchange with an Init DD.
//
// RFC requirement: RFC2328-10.6-1 negative -- in Loading and in Full a master DD that is not a duplicate of the master's last DD (its earlier Init DD 99, or a new DD 101) is never answered with the slave's last DD: the reason is SeqNumberMismatch, the adjacency returns to ExStart, and the slave's next DD is an Init DD.
func TestRFC2328SlaveRepeatsOnlyForTheLastMasterDD(t *testing.T) {
	notDuplicates := map[string]packet.DBDesc{
		"earlier Init DD 99": masterDD(99, packet.DDFlagInit|packet.DDFlagMore|packet.DDFlagMaster),
		"new DD 101":         masterDD(101, packet.DDFlagMaster),
	}
	for _, want := range []state{stateLoading, stateFull} {
		for name, dd := range notDuplicates {
			t.Run(want.String()+"/"+name, func(t *testing.T) {
				s := slaveAfterExchange(t, want)
				if reason := s.tbl.HandleDBDesc(s.cfg.Name, s.peer, dd); reason != reasonSeqNumberMismatch {
					t.Fatalf("reason = %q, want %q", reason, reasonSeqNumberMismatch)
				}
				n, _ := s.tbl.lookupLocked(s.cfg.Name, s.peer)
				if n.State != stateExStart {
					t.Fatalf("state = %s, want ExStart", n.State)
				}
				if next := lastDD(t, s.sender); sameDD(next, s.slaveFinal) || next.Flags&packet.DDFlagInit == 0 {
					t.Fatalf("slave sent %+v, want an Init DD and never its last DD %+v", next, s.slaveFinal)
				}
			})
		}
	}
}
