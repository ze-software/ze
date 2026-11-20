package testdeployment

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"testing"
)

// The cleartext proof's verdicts. These tests feed judgeCleartextFrame the text
// VPP v26.06 prints (show_errors' row layout, the trace node lines traceNodes
// reads) for upstream VPP and for VPP with Ze's patch
// (third_party/vpp-patches-v26.06), so a judge that accepts the upstream
// verdict, or a fate under the wrong rule, goes red here rather than in a real
// run.

// cleartextCounts renders ipsec4-input-feature rows of `show errors`, one per
// reason, in show_errors' "%10lu%=35v%=35s%=10s" layout.
func cleartextCounts(rows ...vppCleartextCount) string {
	var b strings.Builder
	for _, row := range rows {
		b.WriteString("         ")
		b.WriteString(strconv.FormatInt(row.count, 10))
		b.WriteString("            ipsec4-input-feature           ")
		b.WriteString(row.reason)
		b.WriteString("          error\n")
	}
	return b.String()
}

// decryptedProbeFrame answers the proof's frame the inbound SA decrypts.
func decryptedProbeFrame(t *testing.T) vppCleartextProbeFrame {
	t.Helper()
	for _, frame := range vppCleartextProbeFrames {
		if frame.decrypt {
			return frame
		}
	}
	t.Fatal("the proof sends no frame its SA decrypts")
	return vppCleartextProbeFrame{}
}

// The cleartext proof holds VPP to RFC 4301 Section 5.2: the verdict upstream
// VPP gave on 2026-11-20 (cleartext matching a PROTECT entry forwarded, counted
// 'not matched') fails, and only the owed fate under the owed rule passes.
func TestJudgeCleartextFrameHoldsVPPToTheOwedVerdict(t *testing.T) {
	protect := vppCleartextProbeFrames[0]
	forwarded := cleartextTrace("pg-input", "ethernet-input", "ip4-input", "ip4-lookup", "ip4-local")
	dropped := cleartextTrace("pg-input", "ethernet-input", "ip4-input", "ipsec4-input-feature", "error-drop")
	upstream := cleartextCounts(vppCleartextCount{vppIPsecInputReceived, 1}, vppCleartextCount{vppIPsecInputNoMatch, 1})
	patched := cleartextCounts(vppCleartextCount{vppIPsecInputReceived, 1}, vppCleartextCount{vppIPsecInputCleartextProtect, 1})

	if got := judgeCleartextFrame(protect, false, "", dropped, patched); got.Verdict != VPPProofFail {
		t.Fatalf("a frame the generator never sent was judged %v", got.Verdict)
	}
	if got := judgeCleartextFrame(protect, true, "", dropped, ""); got.Verdict != VPPProofFail {
		t.Fatalf("a frame ipsec4-input-feature never counted was judged %v", got.Verdict)
	}
	if got := judgeCleartextFrame(protect, true, "", cleartextTrace("pg-input"), patched); got.Verdict != VPPProofFail {
		t.Fatalf("a frame with no readable fate was judged %v", got.Verdict)
	}

	got := judgeCleartextFrame(protect, true, "", forwarded, upstream)
	if got.Verdict != VPPProofFail {
		t.Fatalf("upstream VPP's verdict (forwarded, 'not matched') was judged %v: %s", got.Verdict, got.Detail)
	}
	if !strings.Contains(got.Detail, "real VPP forwarded cleartext ICMP echo") {
		t.Fatalf("the failure %q does not say VPP forwarded the frame", got.Detail)
	}
	if got := judgeCleartextFrame(protect, true, "", dropped, upstream); got.Verdict != VPPProofFail {
		t.Fatalf("a drop no rule of the SPD counted was judged %v: %s", got.Verdict, got.Detail)
	}

	got = judgeCleartextFrame(protect, true, "", dropped, patched)
	if got.Verdict != VPPProofPass {
		t.Fatalf("the owed verdict was judged %v: %s", got.Verdict, got.Detail)
	}
	for _, want := range []string{"real VPP dropped cleartext ICMP echo", "'IPSec cleartext matched protect' 1", protect.owed} {
		if !strings.Contains(got.Detail, want) {
			t.Fatalf("detail %q lacks %q", got.Detail, want)
		}
	}

	bypass := vppCleartextProbeFrames[2]
	passed := cleartextCounts(vppCleartextCount{vppIPsecInputReceived, 1}, vppCleartextCount{vppIPsecInputBypass, 1})
	if got := judgeCleartextFrame(bypass, true, "", forwarded, passed); got.Verdict != VPPProofPass {
		t.Fatalf("a bypassed frame was judged %v: %s", got.Verdict, got.Detail)
	}
	if got := judgeCleartextFrame(bypass, true, "", dropped, passed); got.Verdict != VPPProofFail {
		t.Fatalf("a dropped BYPASS frame was judged %v", got.Verdict)
	}
}

// The decrypted frame passes ipsec4-input-feature twice, as ESP and as the
// datagram the SA decrypted, and is owed delivery on the decrypt node's mark.
func TestJudgeDecryptedFrameNeedsTheMark(t *testing.T) {
	frame := decryptedProbeFrame(t)
	delivered := cleartextTrace("pg-input", "ethernet-input", "ip4-input", "ipsec4-input-feature", "esp4-decrypt",
		"ip4-input-no-checksum", "ipsec4-input-feature", "ip4-lookup", "ip4-local")
	marked := cleartextCounts(vppCleartextCount{vppIPsecInputReceived, 2}, vppCleartextCount{vppIPsecInputMatch, 1},
		vppCleartextCount{vppIPsecInputDecrypted, 1})
	unmarked := cleartextCounts(vppCleartextCount{vppIPsecInputReceived, 2}, vppCleartextCount{vppIPsecInputMatch, 1},
		vppCleartextCount{vppIPsecInputNoMatch, 1})
	once := cleartextCounts(vppCleartextCount{vppIPsecInputReceived, 1}, vppCleartextCount{vppIPsecInputMatch, 1},
		vppCleartextCount{vppIPsecInputDecrypted, 1})

	if got := judgeCleartextFrame(frame, true, "", delivered, marked); got.Verdict != VPPProofPass {
		t.Fatalf("a decrypted, marked and delivered frame was judged %v: %s", got.Verdict, got.Detail)
	}
	if got := judgeCleartextFrame(frame, true, "", delivered, unmarked); got.Verdict != VPPProofFail {
		t.Fatalf("a delivery without the mark was judged %v", got.Verdict)
	}
	if got := judgeCleartextFrame(frame, true, "", delivered, once); got.Verdict != VPPProofFail {
		t.Fatalf("a frame counted once, so never decrypted, was judged %v", got.Verdict)
	}
}

// The decrypted frame must be ESP the probe's SA accepts: the SPI and tunnel
// source TestVPPRealCleartextProbe installs, and an AES-256-GCM payload under
// its key (RFC 4106: salt then IV as nonce, SPI and sequence number as AAD)
// whose plaintext is RFC 4303's trailer around a valid IPv4 echo.
func TestDecryptedProbePacketIsESPTheProbeSADecrypts(t *testing.T) {
	frame := decryptedProbeFrame(t)
	var packet [vppDecryptedFrameLen]byte
	if err := decryptedProbePacket(packet[:], frame); err != nil {
		t.Fatalf("build: %v", err)
	}
	ip := packet[14:34]
	if got := ipv4HeaderChecksum(ip); got != 0 {
		t.Fatalf("outer IPv4 header does not verify, residual %#x", got)
	}
	if ip[9] != 50 {
		t.Fatalf("outer protocol %d, want ESP 50", ip[9])
	}
	if got := int(binary.BigEndian.Uint16(ip[2:4])); got != vppDecryptedFrameLen-14 {
		t.Fatalf("outer length %d, want %d", got, vppDecryptedFrameLen-14)
	}
	if got := net.IP(ip[12:16]).String(); got != vppDecryptTunnelSource {
		t.Fatalf("outer source %s, want the tunnel's remote end %s", got, vppDecryptTunnelSource)
	}
	esp := packet[34:]
	if got := binary.BigEndian.Uint32(esp[0:4]); got != 0x45450001 {
		t.Fatalf("SPI %#x, want the probe SA's 0x45450001", got)
	}
	if got := binary.BigEndian.Uint32(esp[4:8]); got != 1 {
		t.Fatalf("sequence number %d, want 1", got)
	}

	// The probe SA's key, written out here rather than read from the builder.
	key := make([]byte, 36)
	for i := range key {
		key[i] = byte(0x40 + i)
	}
	block, err := aes.NewCipher(key[:32])
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	nonce := append(append([]byte{}, key[32:]...), esp[8:16]...)
	plaintext, err := aead.Open(nil, nonce, esp[16:], esp[0:8])
	if err != nil {
		t.Fatalf("the SA's key does not open the payload: %v", err)
	}
	n := len(plaintext)
	if plaintext[n-1] != 4 {
		t.Fatalf("Next Header %d, want 4 (IPv4)", plaintext[n-1])
	}
	pad := int(plaintext[n-2])
	for i := range pad {
		if plaintext[n-2-pad+i] != byte(i+1) {
			t.Fatalf("padding byte %d is %d, want %d", i, plaintext[n-2-pad+i], i+1)
		}
	}
	if n%4 != 0 {
		t.Fatalf("plaintext of %d octets does not end the ESP trailer on a 4-octet boundary", n)
	}
	inner := plaintext[:n-2-pad]
	if got := ipv4HeaderChecksum(inner[:20]); got != 0 {
		t.Fatalf("inner IPv4 header does not verify, residual %#x", got)
	}
	if inner[9] != 1 || inner[20] != 8 {
		t.Fatalf("inner protocol %d type %d, want an ICMP echo request", inner[9], inner[20])
	}
	if got := net.IP(inner[12:16]).String(); got != frame.source {
		t.Fatalf("inner source %s, want %s in the PROTECT entry's remote range", got, frame.source)
	}
	if got := int(binary.BigEndian.Uint16(inner[2:4])); got != len(inner) {
		t.Fatalf("inner length %d, want %d", got, len(inner))
	}
}
