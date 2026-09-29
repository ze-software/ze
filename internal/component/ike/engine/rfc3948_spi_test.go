// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the ESP SPIs of a Child SA
// Related: rfc4303_spi_test.go -- scriptedSPIReader; rfc4303_peer_spi_test.go -- peerSPIPreAuth, reencrypt
// VALIDATES: neither ESP SPI of a Child SA is zero: the inbound one Ze generates and the
// outbound one Ze takes from the peer (RFC 3948 Section 2.1).
// PREVENTS: an ESP header that begins with the Non-ESP Marker on port 4500.

package engine

import (
	"crypto/rand"
	"testing"

	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRFC3948GeneratedESPSPIIsNeverZero proves the inbound SPI Ze advertises, which the
// peer then writes into the ESP header of every packet it sends Ze, is never zero.
//
// Goal: RFC 3948 Section 2.1 forbids a zero SPI because four zero octets in that place are
// the Non-ESP Marker that tells an IKE message from an ESP packet on port 4500. Method: the
// system random source is replaced with a script. A non-zero first draw is returned
// unchanged. A script of zero draws followed by one non-zero draw returns that draw, so
// every zero draw was discarded rather than advertised. The reader is restored in Cleanup,
// and no test in this package calls t.Parallel.
//
// RFC 3948 Section 2.1: "The SPI field in the ESP header MUST NOT be a zero value."
//
// RFC requirement: RFC3948-2.1-1 positive -- a non-zero first random draw (0x0a0b0c0d) is the SPI generateESPSPI returns, unchanged.
// RFC requirement: RFC3948-2.1-1 negative -- a random source that draws the zero SPI five times never has it returned: generateESPSPI answers the first non-zero draw (0x00000001) after them.
func TestRFC3948GeneratedESPSPIIsNeverZero(t *testing.T) {
	saved := rand.Reader
	t.Cleanup(func() { rand.Reader = saved })

	rand.Reader = &scriptedSPIReader{words: []uint32{0x0a0b0c0d}}
	spi, err := generateESPSPI()
	if err != nil {
		t.Fatalf("generateESPSPI over a non-zero draw: %v", err)
	}
	if spi != 0x0a0b0c0d {
		t.Fatalf("generateESPSPI = %#08x, want the non-zero draw 0x0a0b0c0d unchanged", spi)
	}

	script := &scriptedSPIReader{words: []uint32{0, 0, 0, 0, 0, 0x00000001}}
	rand.Reader = script
	spi, err = generateESPSPI()
	if err != nil {
		t.Fatalf("generateESPSPI over zero draws: %v", err)
	}
	if spi == 0 {
		t.Fatal("generateESPSPI returned the zero SPI (RFC 3948 Section 2.1)")
	}
	if spi != 0x00000001 {
		t.Fatalf("generateESPSPI = %#08x, want 0x00000001, the first draw after the five zero draws", spi)
	}
	if script.reads != 6 {
		t.Fatalf("generateESPSPI read %d draws, want 6: five zero draws discarded, then the non-zero one", script.reads)
	}
}

// TestRFC3948OutboundESPSPIIsNeverZero proves the outbound SPI, which Ze writes into the
// ESP header of every packet it sends, is never zero.
//
// Goal: the outbound SPI is the one the peer chose in SAr2, so a peer that answered zero
// would have Ze send ESP packets that begin with the Non-ESP Marker. Method: the
// responder's real IKE_AUTH response is re-encrypted with its SAr2 ESP SPI set, once to
// 0x0a0b0c0d and once to 0. The first installs exactly one outbound ESP state on that SPI.
// The second leaves the IKE SA not established, and the first Child SA installs no ESP
// state at all.
//
// RFC 3948 Section 2.1: "The SPI field in the ESP header MUST NOT be a zero value."
//
// RFC requirement: RFC3948-2.1-1 positive -- an SAr2 ESP SPI of 0x0a0b0c0d becomes the one outbound ESP state the dataplane holds.
// RFC requirement: RFC3948-2.1-1 negative -- an SAr2 ESP SPI of 0 leaves the IKE SA not established, initiatorFirstChildSA returns an error, and the dataplane holds no ESP state.
func TestRFC3948OutboundESPSPIIsNeverZero(t *testing.T) {
	const peerSPI = 0x0a0b0c0d
	log := slogutil.DiscardLogger()

	ini, resp, ps := peerSPIPreAuth(t)
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	answer := reencrypt(t, resp, ini, resp.LastSentMsg, peerSPI)
	handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
	if ini.State != StateEstablished {
		t.Fatalf("state = %v, want established for SAr2 SPI %#08x", ini.State, uint32(peerSPI))
	}
	dp := &mockDP{}
	if _, err := initiatorFirstChildSA(ini, ini.PeerCfg, 7, dp, log); err != nil {
		t.Fatalf("initiatorFirstChildSA: %v", err)
	}
	got := outboundSPIs(dp)
	if len(got) != 1 || got[0] != peerSPI {
		t.Fatalf("outbound ESP states = %#08x, want exactly [%#08x]", got, uint32(peerSPI))
	}

	ini, resp, ps = peerSPIPreAuth(t)
	ps.handleAuthRequest(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg, nil, nil, log)
	answer = reencrypt(t, resp, ini, resp.LastSentMsg, 0)
	handleAuthResponse(ini, parseMsg(t, answer), answer, nil, nil, log)
	if ini.State == StateEstablished {
		t.Fatal("the initiator established an IKE SA whose SAr2 carries the zero ESP SPI")
	}
	dp = &mockDP{}
	if child, err := initiatorFirstChildSA(ini, ini.PeerCfg, 7, dp, log); err == nil {
		t.Fatalf("initiatorFirstChildSA accepted the zero SPI and sends on SPI %#08x", child.OutboundSPI)
	}
	if len(dp.sas) != 0 {
		t.Fatalf("the dataplane holds %d ESP states, want 0", len(dp.sas))
	}
}
