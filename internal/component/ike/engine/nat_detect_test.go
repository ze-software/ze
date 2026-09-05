package engine

import (
	"net"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ndSPIs are the SPIs every fixture in this file hashes over. The value does not matter;
// only that both ends compute over the same pair.
var (
	ndSPIi = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	ndSPIr = [8]byte{8, 7, 6, 5, 4, 3, 2, 1}
)

// ndHash returns the NAT_DETECTION hash a conforming peer computes over addr, which is
// what a NAT-FREE path produces.
func ndHash(addr string) []byte {
	return transport.NATDetectionHash(ndSPIi, ndSPIr, net.ParseIP(addr), transport.IKEPort)
}

// ndTranslatedHash returns the hash a peer computes over its own PRE-NAT address, which
// is what arrives when a NAT translated the datagram: it cannot match the address the
// receiver observes. RFC 7296 Section 2.23 is that comparison.
func ndTranslatedHash() []byte {
	return ndHash("192.0.2.77")
}

// ndSAInitRequest builds the IKE_SA_INIT REQUEST payload set a responder reads: the two
// NAT_DETECTION notifies and nothing else. detectResponderNAT walks the payload list, so
// the rest of the exchange is irrelevant to what it decides.
func ndSAInitRequest(srcHash, dstHash []byte) *wire.Message {
	return &wire.Message{
		Header: wire.Header{
			InitiatorSPI: ndSPIi,
			ResponderSPI: ndSPIr,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
		},
		Payloads: []wire.PayloadEntry{
			{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNATDetectionSourceIP, NotificationData: srcHash}},
			{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNATDetectionDestIP, NotificationData: dstHash}},
		},
	}
}

// ndSAInitResponse encodes the IKE_SA_INIT RESPONSE an initiator reads. It carries the
// two NAT_DETECTION notifies alone, so handleSAInitResponse runs its notify loop and then
// fails the completeness gate. The notify loop is the producer under test and it runs
// first, which is the same order the production exchange uses.
func ndSAInitResponse(srcHash, dstHash []byte) []byte {
	msg := wire.Message{
		Header: wire.Header{
			InitiatorSPI: ndSPIi,
			ResponderSPI: ndSPIr,
			MajorVersion: 2,
			ExchangeType: wire.ExchangeIKESAInit,
			Flags:        wire.FlagResponse,
		},
		Payloads: []wire.PayloadEntry{
			{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNATDetectionSourceIP, NotificationData: srcHash}},
			{Payload: &wire.PayloadNotify{NotifyMsgType: wire.NotifyNATDetectionDestIP, NotificationData: dstHash}},
		},
	}
	buf := make([]byte, msg.Len())
	return buf[:msg.WriteTo(buf, 0)]
}

// ndVerdict is the three-field NAT verdict an exchange leaves on the SA.
type ndVerdict struct {
	detected      bool
	behindNAT     bool
	peerBehindNAT bool
}

func ndVerdictOf(sa *SA) ndVerdict {
	return ndVerdict{detected: sa.NATDetected, behindNAT: sa.BehindNAT, peerBehindNAT: sa.PeerBehindNAT}
}

// TestNATDetectionRecordsWhichSideIsBehindTheNAT drives all four NAT_DETECTION branches,
// two on each role, and asserts the verdict each one leaves.
//
// VALIDATES: a SOURCE_IP mismatch records that the PEER is behind a NAT, a
// DESTINATION_IP mismatch records that THIS node is, the two are independent, and a
// matching pair of hashes records neither.
// PREVENTS: the defect this test exists for -- NATDetected answers "is there a NAT" and
// both branches set it, so the substitution rules of RFC 7296 Section 2.23.1, which are
// written per side, had no field to read. It also prevents the half fix R-4 names, where
// one role learns which side and the other does not: one direction then establishes and
// the other answers TS_UNACCEPTABLE.
func TestNATDetectionRecordsWhichSideIsBehindTheNAT(t *testing.T) {
	log := slogutil.DiscardLogger()
	_, respPeer := responderTestPeers(ipsec.AuthPreSharedSecret, "k")

	// The responder's own address is 10.0.0.2 and it sees the initiator at 10.0.0.1.
	cleanSrc := ndHash(respPeer.RemoteAddress)
	cleanDst := ndHash(respPeer.LocalAddress)

	responderCases := []struct {
		name             string
		srcHash, dstHash []byte
		want             ndVerdict
	}{{
		name:    "peer behind a NAT",
		srcHash: ndTranslatedHash(), dstHash: cleanDst,
		want: ndVerdict{detected: true, peerBehindNAT: true},
	}, {
		name:    "this node behind a NAT",
		srcHash: cleanSrc, dstHash: ndTranslatedHash(),
		want: ndVerdict{detected: true, behindNAT: true},
	}, {
		name:    "both sides behind a NAT",
		srcHash: ndTranslatedHash(), dstHash: ndTranslatedHash(),
		want: ndVerdict{detected: true, behindNAT: true, peerBehindNAT: true},
	}, {
		name:    "no NAT on the path",
		srcHash: cleanSrc, dstHash: cleanDst,
		want: ndVerdict{},
	}}

	for _, c := range responderCases {
		t.Run("responder/"+c.name, func(t *testing.T) {
			sa := &SA{PeerCfg: respPeer}
			detectResponderNAT(sa, ndSAInitRequest(c.srcHash, c.dstHash))
			if got := ndVerdictOf(sa); got != c.want {
				t.Errorf("responder verdict = %+v, want %+v", got, c.want)
			}
		})
	}

	// The initiator's own address is 10.0.0.1 and it dials the responder at 10.0.0.2.
	// The hashes are the mirror of the responder's: SOURCE_IP covers the peer, which is
	// the responder here, and DESTINATION_IP covers this node.
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
	iniClean := struct{ src, dst []byte }{ndHash(iniPeer.RemoteAddress), ndHash(iniPeer.LocalAddress)}

	initiatorCases := []struct {
		name             string
		srcHash, dstHash []byte
		want             ndVerdict
	}{{
		name:    "peer behind a NAT",
		srcHash: ndTranslatedHash(), dstHash: iniClean.dst,
		want: ndVerdict{detected: true, peerBehindNAT: true},
	}, {
		name:    "this node behind a NAT",
		srcHash: iniClean.src, dstHash: ndTranslatedHash(),
		want: ndVerdict{detected: true, behindNAT: true},
	}, {
		name:    "both sides behind a NAT",
		srcHash: ndTranslatedHash(), dstHash: ndTranslatedHash(),
		want: ndVerdict{detected: true, behindNAT: true, peerBehindNAT: true},
	}, {
		name:    "no NAT on the path",
		srcHash: iniClean.src, dstHash: iniClean.dst,
		want: ndVerdict{},
	}}

	for _, c := range initiatorCases {
		t.Run("initiator/"+c.name, func(t *testing.T) {
			sa := &SA{
				PeerCfg:      iniPeer,
				InitiatorSPI: ndSPIi,
				ResponderSPI: ndSPIr,
				State:        StateSAInitSent,
			}
			table := NewSATable()
			table.Insert(sa)
			raw := ndSAInitResponse(c.srcHash, c.dstHash)
			handleSAInitResponse(sa, parseMsg(t, raw), raw, table, nil, nil, log)
			if got := ndVerdictOf(sa); got != c.want {
				t.Errorf("initiator verdict = %+v, want %+v", got, c.want)
			}
		})
	}
}
