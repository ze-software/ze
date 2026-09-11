//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"strings"
	"testing"
	"time"

	discovery "github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/core/pcap"
)

var (
	replayClientMAC = [discovery.EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x01}
	replayACMAC     = [discovery.EthALen]byte{0x02, 0x00, 0x00, 0x00, 0x00, 0x02}
)

// buildCapture writes one classic pcap file carrying frames, in order, so a
// test can hand observeDiscoveryFrames exactly the bytes stopDiscoveryCapture
// would have handed it.
func buildCapture(t *testing.T, frames ...[]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := pcap.WriteFileHeader(&buf, discovery.EthMaxLen, pcap.LinkTypeEthernet); err != nil {
		t.Fatalf("write pcap file header: %v", err)
	}
	for _, frame := range frames {
		if err := pcap.WriteRecord(&buf, time.Unix(0, 0), frame, len(frame)); err != nil {
			t.Fatalf("write pcap record: %v", err)
		}
	}
	return buf.Bytes()
}

func padrFrame(t *testing.T, cookie []byte) []byte {
	t.Helper()
	var padoBuf [discovery.EthMaxLen]byte
	padoFrame := discovery.BuildPADO(
		padoBuf[:],
		replayACMAC,
		&discovery.Packet{SrcMAC: replayClientMAC, Code: discovery.CodePADI},
		"ze-bng",
		nil,
		cookie,
	)
	pado, err := discovery.ParseDiscovery(padoFrame)
	if err != nil {
		t.Fatalf("parse synthetic PADO: %v", err)
	}
	var padrBuf [discovery.EthMaxLen]byte
	padr := discovery.BuildPADR(padrBuf[:], replayClientMAC, &pado, "internet", nil)
	if padr == nil {
		t.Fatal("BuildPADR returned nil (truncated)")
	}
	return append([]byte(nil), padr...)
}

func padsFrame(t *testing.T, padr []byte, sid uint16) []byte {
	t.Helper()
	pkt, err := discovery.ParseDiscovery(padr)
	if err != nil {
		t.Fatalf("parse synthetic PADR: %v", err)
	}
	var buf [discovery.EthMaxLen]byte
	pads := discovery.BuildPADS(buf[:], replayACMAC, &pkt, "ze-bng", sid)
	if pads == nil {
		t.Fatal("BuildPADS returned nil (truncated)")
	}
	return append([]byte(nil), pads...)
}

func padsErrorFrame(t *testing.T, padr []byte) []byte {
	t.Helper()
	pkt, err := discovery.ParseDiscovery(padr)
	if err != nil {
		t.Fatalf("parse synthetic PADR: %v", err)
	}
	var buf [discovery.EthMaxLen]byte
	pads := discovery.BuildPADSError(buf[:], replayACMAC, &pkt, "ze-bng", discovery.TagACSystemError)
	if pads == nil {
		t.Fatal("BuildPADSError returned nil (truncated)")
	}
	return append([]byte(nil), pads...)
}

func TestObserveDiscoveryFramesReadsPADRAndPADS(t *testing.T) {
	// VALIDATES: the replay checker's cross-reference of the wire against
	// REST reads the SAME session id BuildPADS actually wrote.
	// PREVENTS: an off-by-endianness or field-offset regression in
	// observeDiscoveryFrames silently reporting the wrong session id.
	padr := padrFrame(t, []byte("cookie-1"))
	pads := padsFrame(t, padr, 0x1234)
	capture := buildCapture(t, padr, pads)

	observed, err := observeDiscoveryFrames(capture)
	if err != nil {
		t.Fatalf("observeDiscoveryFrames: %v", err)
	}
	if observed.padrCount != 1 || observed.padsCount != 1 {
		t.Fatalf("counts = %+v, want 1 PADR and 1 PADS", observed)
	}
	if observed.padsSID != 0x1234 {
		t.Fatalf("padsSID = %#04x, want 0x1234", observed.padsSID)
	}
	if observed.padsHadACSystemError {
		t.Fatal("a normal PADS must not report an AC-System-Error tag")
	}
	if !bytes.Equal(observed.firstPADR, padr) {
		t.Fatal("firstPADR did not preserve the exact captured PADR bytes")
	}
}

func TestObserveDiscoveryFramesReadsTheCapRefusal(t *testing.T) {
	// VALIDATES: the over-cap discriminating assertion -- session id 0x0000
	// alongside an AC-System-Error tag -- is what observeDiscoveryFrames
	// reports for a PADS built by BuildPADSError.
	// PREVENTS: a refusal being misread as an ordinary session (SID nonzero,
	// no error tag), which is exactly what a broken admitPerMACCap produces.
	padr := padrFrame(t, []byte("cookie-2"))
	pads := padsErrorFrame(t, padr)
	capture := buildCapture(t, padr, pads)

	observed, err := observeDiscoveryFrames(capture)
	if err != nil {
		t.Fatalf("observeDiscoveryFrames: %v", err)
	}
	if observed.padsSID != 0 {
		t.Fatalf("padsSID = %#04x, want 0x0000", observed.padsSID)
	}
	if !observed.padsHadACSystemError {
		t.Fatal("a refusal PADS must report the AC-System-Error tag")
	}
}

func TestObserveDiscoveryFramesRejectsNonEthernetCapture(t *testing.T) {
	// VALIDATES: a capture the checker cannot trust is refused explicitly.
	// PREVENTS: silently reading a cooked-header capture as Ethernet and
	// misparsing every offset in it.
	var buf bytes.Buffer
	if err := pcap.WriteFileHeader(&buf, discovery.EthMaxLen, pcap.LinkTypeLinuxSLL); err != nil {
		t.Fatalf("write pcap file header: %v", err)
	}
	if _, err := observeDiscoveryFrames(buf.Bytes()); err == nil || !strings.Contains(err.Error(), "link type") {
		t.Fatalf("non-Ethernet capture error = %v, want a link-type complaint", err)
	}
}

func TestObserveDiscoveryFramesFailsClosedOnACorruptFrame(t *testing.T) {
	// VALIDATES: the capture is trusted to be PPPoE discovery only (the
	// tcpdump filter's job), so a frame that fails to parse is reported as a
	// corrupt capture, matching checkDiscoveryServiceNameTags's rule, rather
	// than silently skipped.
	// PREVENTS: a truncated or unrelated frame being read as "no PADR seen",
	// which would pass the over-cap check's absence assertion for the wrong
	// reason.
	capture := buildCapture(t, []byte{0x00, 0x01, 0x02})
	if _, err := observeDiscoveryFrames(capture); err == nil || !strings.Contains(err.Error(), "did not parse") {
		t.Fatalf("corrupt-frame error = %v, want a parse complaint", err)
	}
}
