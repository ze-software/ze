// VALIDATES: the Host side of RFC 2516 SESSION_ID rules: a PADO carrying a
// non-zero SESSION_ID is not an offer, and a PADS carrying SESSION_ID 0x0000
// is a refusal, never a session.
// PREVENTS: a client that opens a PPP session on a SESSION_ID the AC never
// generated for it.

package pppoeclient

import (
	"encoding/binary"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe"
)

var (
	testACMAC   = [pppoe.EthALen]byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}
	testHostMAC = [pppoe.EthALen]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55}
)

// serveFrame makes readDiscoveryFrame hand back frame once on interface 1,
// then report no frame.
func serveFrame(t *testing.T, frame []byte) {
	t.Helper()
	orig := readDiscoveryFrame
	served := false
	readDiscoveryFrame = func(_ int, buf []byte) (int, int, error) {
		if served {
			return 0, 0, errFakeNoDiscoveryFrame
		}
		served = true
		return copy(buf, frame), 1, nil
	}
	t.Cleanup(func() { readDiscoveryFrame = orig })
}

// RFC requirement: RFC2516-5.2-6 negative — tryReadPADO accepts a PADO whose SESSION_ID is 0x0000 and refuses the same PADO once its SESSION_ID is any other value.
func TestRFC2516PADOWithNonZeroSessionIDIsNotAnOffer(t *testing.T) {
	hostUniq := [4]byte{1, 2, 3, 4}
	padi := pppoe.Packet{Code: pppoe.CodePADI, SrcMAC: testHostMAC, Tags: []pppoe.Tag{
		{Type: pppoe.TagServiceName}, {Type: pppoe.TagHostUniq, Value: hostUniq[:]},
	}}
	var buf [pppoe.EthMaxLen]byte
	pado := pppoe.BuildPADO(buf[:], testACMAC, &padi, "ze", nil, []byte("cookie"))
	if pado == nil {
		t.Fatal("BuildPADO returned nil")
	}

	serveFrame(t, pado)
	if _, ok := tryReadPADO(0, 1, hostUniq, ""); !ok {
		t.Fatal("a PADO with SESSION_ID 0x0000 was refused")
	}

	binary.BigEndian.PutUint16(pado[pppoe.EthHdrLen+2:], 0x0001)
	serveFrame(t, pado)
	if pkt, ok := tryReadPADO(0, 1, hostUniq, ""); ok {
		t.Fatalf("a PADO with SESSION_ID 0x0001 was accepted as an offer: %+v", pkt)
	}
}

// RFC requirement: RFC2516-5.4-3 negative — tryReadPADS returns the SESSION_ID of a PADS carrying the AC's generated value and returns an error, not a session, for a PADS carrying 0x0000.
func TestRFC2516PADSWithZeroSessionIDIsARefusal(t *testing.T) {
	padr := pppoe.Packet{Code: pppoe.CodePADR, SrcMAC: testHostMAC, Tags: []pppoe.Tag{{Type: pppoe.TagServiceName}}}
	var buf [pppoe.EthMaxLen]byte

	pads := pppoe.BuildPADS(buf[:], testACMAC, &padr, "ze", 0x1234)
	if pads == nil {
		t.Fatal("BuildPADS returned nil")
	}
	serveFrame(t, pads)
	sid, err := tryReadPADS(0, 1, testACMAC)
	if err != nil {
		t.Fatalf("tryReadPADS on a PADS with SESSION_ID 0x1234: %v", err)
	}
	if sid != 0x1234 {
		t.Fatalf("tryReadPADS = 0x%04x, want 0x1234", sid)
	}

	refusal := pppoe.BuildPADSError(buf[:], testACMAC, &padr, "ze", pppoe.TagSvcNameError)
	if refusal == nil {
		t.Fatal("BuildPADSError returned nil")
	}
	serveFrame(t, refusal)
	sid, err = tryReadPADS(0, 1, testACMAC)
	if err == nil {
		t.Fatalf("tryReadPADS on a PADS with SESSION_ID 0x0000 returned 0x%04x with no error", sid)
	}
	if sid != 0 {
		t.Fatalf("tryReadPADS returned SESSION_ID 0x%04x beside the error", sid)
	}
}
