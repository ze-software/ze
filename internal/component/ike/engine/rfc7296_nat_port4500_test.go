// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- NAT detection and port 4500 framing
// Related: nat_detect_test.go -- the NAT_DETECTION fixtures; rfc3948_keepalive_need_test.go -- the raw peer socket.
package engine

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// natInitiatorVerdict runs an initiator's IKE_SA_INIT response handling over a
// response that carries the two NAT_DETECTION notifies with the given hashes, and
// returns the SA it leaves.
func natInitiatorVerdict(t *testing.T, srcHash, dstHash []byte) *SA {
	t.Helper()
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
	sa := &SA{
		PeerCfg:      iniPeer,
		InitiatorSPI: ndSPIi,
		ResponderSPI: ndSPIr,
		State:        StateSAInitSent,
	}
	table := NewSATable()
	table.Insert(sa)
	raw := ndSAInitResponse(srcHash, dstHash)
	handleSAInitResponse(sa, parseMsg(t, raw), raw, table, nil, nil, slogutil.DiscardLogger())
	return sa
}

// RFC requirement: RFC7296-2.23-1 positive -- "The IKE initiator MUST check the
// NAT_DETECTION_SOURCE_IP or NAT_DETECTION_DESTINATION_IP payloads if present"
// (rfc/full/rfc7296.txt, Section 2.23). The initiator reads each payload the
// IKE_SA_INIT response carries and compares it with the hash over the address it
// expects: a SOURCE_IP mismatch alone, and a DESTINATION_IP mismatch alone, each
// records the NAT and floats the SA to port 4500.
//
// VALIDATES: handleSAInitResponse checks both NAT_DETECTION payloads, each on its own.
// PREVENTS: an initiator that ignores either payload and keeps talking on port 500
// through a NAT that the peer's payload revealed.
func TestRFC7296InitiatorChecksEachNATDetectionPayload(t *testing.T) {
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
	cleanSource, cleanTarget := ndHash(iniPeer.RemoteAddress), ndHash(iniPeer.LocalAddress)
	cases := []struct {
		name            string
		source, target  []byte
		behind, peerNAT bool
	}{
		{name: "SOURCE_IP mismatch", source: ndTranslatedHash(), target: cleanTarget, peerNAT: true},
		{name: "DESTINATION_IP mismatch", source: cleanSource, target: ndTranslatedHash(), behind: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sa := natInitiatorVerdict(t, c.source, c.target)
			if !sa.NATDetected {
				t.Fatal("the payload revealed a NAT, but the initiator recorded none")
			}
			if sa.BehindNAT != c.behind || sa.PeerBehindNAT != c.peerNAT {
				t.Fatalf("BehindNAT=%v PeerBehindNAT=%v, want %v %v", sa.BehindNAT, sa.PeerBehindNAT, c.behind, c.peerNAT)
			}
			if sa.localPort != transport.NATTPort {
				t.Fatalf("local port = %d after a detected NAT, want %d", sa.localPort, transport.NATTPort)
			}
		})
	}
}

// RFC requirement: RFC7296-2.23-1 negative -- "The IKE initiator MUST check the
// NAT_DETECTION_SOURCE_IP or NAT_DETECTION_DESTINATION_IP payloads if present"
// (rfc/full/rfc7296.txt, Section 2.23). A check compares the hash: the presence of
// the payloads is not itself a NAT. A response whose two hashes match the addresses
// the initiator expects records no NAT and leaves the SA off port 4500.
//
// VALIDATES: the NAT verdict comes from comparing the payload contents.
// PREVENTS: an initiator that treats any NAT_DETECTION payload as a NAT without
// checking it, which floats every NAT-T capable peer to port 4500 and encapsulates.
func TestRFC7296InitiatorCheckFindsNoNATOnMatchingPayloads(t *testing.T) {
	iniPeer, _ := responderTestPeers(ipsec.AuthPreSharedSecret, "k")
	sa := natInitiatorVerdict(t, ndHash(iniPeer.RemoteAddress), ndHash(iniPeer.LocalAddress))
	if sa.NATDetected || sa.BehindNAT || sa.PeerBehindNAT {
		t.Fatalf("matching payloads recorded a NAT: detected=%v behind=%v peer=%v",
			sa.NATDetected, sa.BehindNAT, sa.PeerBehindNAT)
	}
	if sa.localPort == transport.NATTPort {
		t.Fatal("matching payloads floated the SA to port 4500")
	}
}

// natPortRequest encodes an IKE request the tests below send through sendRaw.
func natPortRequest(t *testing.T) []byte {
	t.Helper()
	msg := ndSAInitRequest(ndHash("10.0.0.1"), ndHash("10.0.0.2"))
	buf := make([]byte, msg.Len())
	out := buf[:msg.WriteTo(buf, 0)]
	// Octet 19 is the IKE header's Flags field (RFC 7296 Section 3.1).
	if out[19]&wire.FlagResponse != 0 {
		t.Fatal("fixture is a response; sendRaw would take the reply path")
	}
	return out
}

// natPortReceive reads the one datagram the raw peer socket receives.
func natPortReceive(t *testing.T, f *mbFixture, peer *net.UDPConn) []byte {
	t.Helper()
	if err := peer.SetReadDeadline(time.Now().Add(rtxArrive)); err != nil {
		t.Fatalf("set read deadline: %v", err)
	}
	buf := make([]byte, 2048)
	n, err := peer.Read(buf)
	if err != nil {
		t.Fatalf("peer read (local port %d): %v", f.local.localPort, err)
	}
	return buf[:n]
}

// RFC requirement: RFC7296-2.23-3 positive -- "To tunnel IKE packets over UDP port 4500,
// the IKE header has four octets of zeros prepended and the result immediately follows
// the UDP header." (rfc/full/rfc7296.txt, Section 2.23). An SA floated to port 4500
// sends an IKE message whose UDP payload is four zero octets followed by the IKE
// message, byte for byte.
//
// VALIDATES: sendRaw, the path every post-establishment sender shares, frames IKE on
// port 4500 with the non-ESP marker and nothing else before the IKE header.
// PREVENTS: an IKE message on port 4500 that the peer reads as ESP (its first four
// octets would be the IKE SPI, which the peer takes for an ESP SPI).
func TestRFC7296IKEOnPort4500CarriesTheZeroMarker(t *testing.T) {
	peer := keepaliveNeedPeer(t)
	f := keepaliveNeedSA(t, peer, true, false)
	if f.local.localPort != transport.NATTPort {
		t.Fatalf("fixture SA is on port %d, want %d", f.local.localPort, transport.NATTPort)
	}
	msg := natPortRequest(t)
	sendRaw(f.local, f.myTr, msg, slogutil.DiscardLogger())
	got := natPortReceive(t, f, peer)
	want := append([]byte{0, 0, 0, 0}, msg...)
	if !bytes.Equal(got, want) {
		t.Fatalf("UDP payload on port 4500 = % x..., want four zero octets then the IKE message % x...",
			got[:min(len(got), 12)], msg[:8])
	}
}

// RFC requirement: RFC7296-2.23-3 negative -- "To tunnel IKE packets over UDP port 4500,
// the IKE header has four octets of zeros prepended and the result immediately follows
// the UDP header." (rfc/full/rfc7296.txt, Section 2.23). The zeros belong to port 4500
// only: an SA on port 500 sends the IKE message with nothing prepended, so the IKE
// header immediately follows the UDP header, as RFC 7296 Section 3 frames it there.
//
// VALIDATES: the marker is conditional on port 4500.
// PREVENTS: a marker prepended on port 500, where the peer reads the four zeros as
// the first octets of the initiator SPI and drops the message.
func TestRFC7296IKEOnPort500CarriesNoMarker(t *testing.T) {
	peer := keepaliveNeedPeer(t)
	ini, resp, _ := establishPSK(t)
	f := mbOwner(t, resp, ini, false)
	endpoint, ok := peer.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer socket address is not *net.UDPAddr")
	}
	f.local.peerEndpoint = endpoint
	if f.local.localPort == transport.NATTPort {
		t.Fatal("fixture SA is on port 4500, want the IKE port")
	}
	msg := natPortRequest(t)
	sendRaw(f.local, f.myTr, msg, slogutil.DiscardLogger())
	if got := natPortReceive(t, f, peer); !bytes.Equal(got, msg) {
		t.Fatalf("UDP payload on port 500 = % x..., want the IKE message % x... unchanged",
			got[:min(len(got), 12)], msg[:8])
	}
}
