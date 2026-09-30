// Design: docs/architecture/ospf/ospf-7-lsdb-flooding.md -- flooding procedure (RFC 2328 Section 13.3), shared by OSPFv3.
// Related: lsdb/flooding.go -- floodCopy and RetransmitTick, the outgoing copy's LS age.
// Related: encoder_v6.go -- v6Encoder, the OSPFv3 LS Update encoder the copy is written through.
//
// VALIDATES: RFC 5340 Appendix C.3, InfTransDelay: "LSAs contained in the update packet must
// have their age incremented by this amount before transmission", on the OSPFv3 wire: the LS
// age read back from the encoded OSPFv3 LS Update, for the first flood and a retransmission.
// PREVENTS: an OSPFv3 LSA leaving with its database age, or with an age past MaxAge.
package ospf

import (
	"testing"
	"time"

	ospflsdb "github.com/ze-software/ze/internal/plugins/ospf/lsdb"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
	ospfv3packet "github.com/ze-software/ze/internal/plugins/ospf/v3/packet"
)

// v3FloodAges originates this router's OSPFv3 Link-LSA on eth0, whose InfTransDelay is
// transmitDelay and whose one neighbor is Full, and returns the LS age of each LS Update eth0
// sent, decoded from the OSPFv3 wire: the first flood, then, when retransmit is set, the
// retransmission 5 seconds later.
func v3FloodAges(t *testing.T, transmitDelay uint16, retransmit bool) []uint16 {
	t.Helper()
	now := time.Unix(1000, 0)
	e := &engine{lsdb: ospflsdb.New(func() time.Time { return now })}
	e.lsdb.SetPacketEncoder(v6Encoder{})
	self := types.RouterID{172, 30, 0, 2}
	iface := v6BroadcastInterface(types.BackboneArea, self)
	iface.TransmitDelay = transmitDelay
	iface.RetransmitInterval = 5
	tx := &ospfRawTx{}
	e.lsdb.SetTx(tx.Send)
	e.lsdb.SetTopology(func() []ospflsdb.InterfaceInfo { return []ospflsdb.InterfaceInfo{iface} })
	if _, changed := e.v6OriginateLinkLSA(self, iface); !changed {
		t.Fatal("v6OriginateLinkLSA did not originate")
	}
	if retransmit {
		now = now.Add(5 * time.Second)
		if n := e.lsdb.RetransmitTick(now); n != 1 {
			t.Fatalf("retransmissions after RxmtInterval = %d, want 1", n)
		}
	}
	var ages []uint16
	for i := range tx.sends {
		s := &tx.sends[i]
		if s.iface != "eth0" {
			continue
		}
		p, err := ospfv3packet.DecodePacket(s.raw)
		if err != nil {
			t.Fatalf("send %d is not an OSPFv3 packet: %v", i, err)
		}
		if p.LSUpdate == nil || len(p.LSUpdate.LSAs) != 1 {
			t.Fatalf("send %d is not an LS Update carrying one LSA", i)
		}
		ages = append(ages, uint16(p.LSUpdate.LSAs[0].Header.Age))
	}
	return ages
}

// TestRFC5340FloodIncrementsAgeByInfTransDelay reads the OSPFv3 LS age on the wire for an
// interface with InfTransDelay 7, then for one whose InfTransDelay alone passes MaxAge.
func TestRFC5340FloodIncrementsAgeByInfTransDelay(t *testing.T) {
	// RFC requirement: RFC5340-C.3-2 positive -- the Link-LSA originated at age 0 leaves in the
	// OSPFv3 LS Update with age 7 (InfTransDelay 7), and its retransmission 5 s later with 12
	// (held 5 s, plus InfTransDelay once) (§C.3).
	ages := v3FloodAges(t, 7, true)
	if len(ages) != 2 || ages[0] != 7 || ages[1] != 12 {
		t.Fatalf("OSPFv3 LS ages on the wire = %v, want [7 12] (flood 0+7, retransmit 0+5+7)", ages)
	}
	// RFC requirement: RFC5340-C.3-2 negative -- an InfTransDelay larger than MaxAge does not
	// carry the LS age past MaxAge: the increment stops at MaxAge (RFC 2328 Section 13.3 (5),
	// which RFC 5340 keeps).
	ages = v3FloodAges(t, 65535, false)
	if len(ages) != 1 || ages[0] != types.MaxAge {
		t.Fatalf("OSPFv3 LS ages on the wire = %v, want [%d] (MaxAge)", ages, types.MaxAge)
	}
}
