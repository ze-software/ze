// Design: docs/architecture/wire/l2tp.md -- receiving hidden AVPs (RFC 2661 Section 4.3)
// Related: hidden.go -- revealHiddenAVPs, the reveal under test
// Related: avp.go -- skipHiddenAVP, the M-bit rule for an AVP left hidden
// Related: tunnel_fsm.go -- handleMessage, which reveals before dispatch
// Related: reactor.go -- the SCCRQ pre-parse, which reveals before parseSCCRQ

package l2tp

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// hiddenTestSecret is the tunnel secret the test LAC hides AVPs under.
const hiddenTestSecret = "example-secret"

// hiddenTestRV and hiddenTestRV2 are two Random Vectors of the recommended
// 16 octets.
var (
	hiddenTestRV  = []byte("0123456789abcdef")
	hiddenTestRV2 = []byte("fedcba9876543210")
)

// hiddenTestCallingNumber is the Calling Number the test LAC hides.
const hiddenTestCallingNumber = "5550100"

// writeHiddenTestAVP writes an AVP with H=1 whose value is hidden under the
// test secret and rv, the way a LAC hides it. padding octets follow the value
// in the Hidden AVP Subformat, and Original Length names the value alone.
func writeHiddenTestAVP(t *testing.T, buf []byte, off int, mandatory bool, attrType AVPType, rv, value []byte, padding int) int {
	t.Helper()
	sub := append([]byte(nil), value...)
	for range padding {
		sub = append(sub, 0x5a)
	}
	ct := HiddenEncrypt(buf[off+AVPHeaderLen:], attrType, []byte(hiddenTestSecret), rv, sub)
	if padding > 0 {
		// HiddenEncrypt writes Original Length = len(sub). In a single MD5
		// block c = p XOR b1, so XOR-ing the old and the wanted length into
		// the first two ciphertext octets rewrites the hidden length without
		// the keystream. A second block would chain on the changed c(1).
		require.LessOrEqual(t, len(ct), hiddenBlockSize, "the length rewrite holds for one block only")
		lengthMask := uint16(len(sub)) ^ uint16(len(value))
		binary.BigEndian.PutUint16(ct, binary.BigEndian.Uint16(ct)^lengthMask)
	}
	flags := FlagHidden
	if mandatory {
		flags |= FlagMandatory
	}
	WriteAVPHeader(buf, off, flags, 0, attrType, AVPHeaderLen+len(ct))
	return AVPHeaderLen + len(ct)
}

// writeRandomVectorTestAVP writes a Random Vector AVP (M=1, H=0) carrying rv.
func writeRandomVectorTestAVP(buf []byte, off int, rv []byte) int {
	return WriteAVPBytes(buf, off, true, 0, AVPRandomVector, rv)
}

// processTestICRQ delivers body to tun as the first control message off the
// wire (Ns 0, Nr 0) and returns what the tunnel sends back.
func processTestICRQ(t *testing.T, tun *L2TPTunnel, defaults TunnelDefaults, body []byte) []sendRequest {
	t.Helper()
	pkt := make([]byte, ControlHeaderLen+len(body))
	WriteControlHeader(pkt, 0, uint16(len(pkt)), tun.localTID, 0, 0, 0)
	copy(pkt[ControlHeaderLen:], body)
	hdr, err := ParseMessageHeader(pkt)
	require.NoError(t, err)
	return tun.Process(hdr, pkt[hdr.PayloadOff:hdr.Length], time.Now(), defaults, nil)
}

// sentMessageTypes returns the Message Type of every non-ZLB control message
// in out, in order.
func sentMessageTypes(t *testing.T, out []sendRequest) []MessageType {
	t.Helper()
	var types []MessageType
	for _, o := range out {
		hdr, err := ParseMessageHeader(o.bytes)
		require.NoError(t, err)
		body := o.bytes[hdr.PayloadOff:hdr.Length]
		if len(body) == 0 {
			continue
		}
		types = append(types, extractMsgType(body))
	}
	return types
}

// onlyTestSession returns the one session tun holds.
func onlyTestSession(t *testing.T, tun *L2TPTunnel) *L2TPSession {
	t.Helper()
	require.Len(t, tun.sessions, 1, "the ICRQ created one session")
	for _, s := range tun.sessions {
		return s
	}
	return nil
}

// TestHiddenAVPsRevealedOnICRQ sends an ICRQ off the wire whose Assigned
// Session ID (M=1) and Calling Number (M=1, padded) are hidden under one
// Random Vector, to a tunnel whose secret is the LAC's, and reads the session
// the ICRQ created and the reply on the wire.
//
// RFC requirement: RFC2661-4.3-2 positive -- two hidden AVPs that follow one Random Vector AVP in an ICRQ are both revealed with it: the session holds the hidden Assigned Session ID and the hidden Calling Number (whose Hidden AVP Subformat carries 5 padding octets), and the tunnel answers ICRP.
func TestHiddenAVPsRevealedOnICRQ(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	defaults := TunnelDefaults{SharedSecret: hiddenTestSecret}

	var buf [256]byte
	off := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgICRQ))
	off += writeRandomVectorTestAVP(buf[:], off, hiddenTestRV)
	off += writeHiddenTestAVP(t, buf[:], off, true, AVPAssignedSessionID, hiddenTestRV, u16Bytes(500), 0)
	off += WriteAVPUint32(buf[:], off, true, AVPCallSerialNumber, 1001)
	off += writeHiddenTestAVP(t, buf[:], off, true, AVPCallingNumber, hiddenTestRV, []byte(hiddenTestCallingNumber), 5)

	out := processTestICRQ(t, tun, defaults, buf[:off])

	require.Equal(t, []MessageType{MsgICRP}, sentMessageTypes(t, out), "the ICRQ is answered with ICRP")
	sess := onlyTestSession(t, tun)
	require.EqualValues(t, 500, sess.remoteSID, "the hidden Assigned Session ID was revealed")
	require.Equal(t, hiddenTestCallingNumber, sess.callingNumber,
		"the hidden Calling Number was revealed and its padding dropped")
}

// TestHiddenAVPUsesClosestRandomVector hides the Assigned Session ID under one
// Random Vector and the Calling Number under a second one that follows it.
//
// RFC 2661 Section 4.4.1: "More than one Random Vector AVP may appear in a
// message, in which case a hidden AVP uses the Random Vector AVP most closely
// preceding it." A reveal that kept the first vector would read the Calling
// Number with the wrong keystream.
func TestHiddenAVPUsesClosestRandomVector(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	defaults := TunnelDefaults{SharedSecret: hiddenTestSecret}

	var buf [256]byte
	off := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgICRQ))
	off += writeRandomVectorTestAVP(buf[:], off, hiddenTestRV)
	off += writeHiddenTestAVP(t, buf[:], off, true, AVPAssignedSessionID, hiddenTestRV, u16Bytes(500), 0)
	off += WriteAVPUint32(buf[:], off, true, AVPCallSerialNumber, 1001)
	off += writeRandomVectorTestAVP(buf[:], off, hiddenTestRV2)
	off += writeHiddenTestAVP(t, buf[:], off, true, AVPCallingNumber, hiddenTestRV2, []byte(hiddenTestCallingNumber), 0)

	out := processTestICRQ(t, tun, defaults, buf[:off])

	require.Equal(t, []MessageType{MsgICRP}, sentMessageTypes(t, out))
	sess := onlyTestSession(t, tun)
	require.EqualValues(t, 500, sess.remoteSID)
	require.Equal(t, hiddenTestCallingNumber, sess.callingNumber,
		"the Calling Number is revealed with the second Random Vector, the one closest before it")
}

// TestHiddenAVPNotRevealedRefusesICRQ sends the ICRQ of
// TestHiddenAVPsRevealedOnICRQ in each shape Ze cannot reveal a hidden
// mandatory AVP of, and reads that the session is refused with a CDN while
// the tunnel stays up. RFC 2661 Section 7.1: "if a malformed AVP is received
// with the M-bit set, the session or tunnel should be terminated with a
// proper Result or Error Code sent."
//
// RFC requirement: RFC2661-4.3-1 negative -- an ICRQ whose hidden mandatory Calling Number comes before the message's only Random Vector AVP is refused: the tunnel answers CDN and holds no session, though its shared secret is the LAC's.
func TestHiddenAVPNotRevealedRefusesICRQ(t *testing.T) {
	cases := []struct {
		name   string
		secret string
		build  func(t *testing.T, buf []byte, off int) int
	}{
		{
			name:   "random vector after the hidden AVP",
			secret: hiddenTestSecret,
			build: func(t *testing.T, buf []byte, off int) int {
				off += writeHiddenTestAVP(t, buf, off, true, AVPCallingNumber, hiddenTestRV, []byte(hiddenTestCallingNumber), 0)
				return off + writeRandomVectorTestAVP(buf, off, hiddenTestRV)
			},
		},
		{
			name:   "no shared secret configured",
			secret: "",
			build: func(t *testing.T, buf []byte, off int) int {
				off += writeRandomVectorTestAVP(buf, off, hiddenTestRV)
				return off + writeHiddenTestAVP(t, buf, off, true, AVPCallingNumber, hiddenTestRV, []byte(hiddenTestCallingNumber), 0)
			},
		},
		{
			name:   "random vector hidden",
			secret: hiddenTestSecret,
			build: func(t *testing.T, buf []byte, off int) int {
				// RFC 2661 Section 4.4.1, Random Vector: "This AVP MUST NOT
				// be hidden (the H-bit MUST be 0)." A hidden one is no vector.
				// Its M bit is clear here so that the refusal can only come
				// from the Calling Number it fails to reveal.
				off += writeHiddenTestAVP(t, buf, off, false, AVPRandomVector, hiddenTestRV2, hiddenTestRV, 0)
				return off + writeHiddenTestAVP(t, buf, off, true, AVPCallingNumber, hiddenTestRV, []byte(hiddenTestCallingNumber), 0)
			},
		},
		{
			name:   "original length past the value",
			secret: hiddenTestSecret,
			build: func(t *testing.T, buf []byte, off int) int {
				off += writeRandomVectorTestAVP(buf, off, hiddenTestRV)
				n := writeHiddenTestAVP(t, buf, off, true, AVPCallingNumber, hiddenTestRV, []byte(hiddenTestCallingNumber), 0)
				// Flip the high bit of the hidden Original Length: 7 becomes
				// 32775, more octets than the AVP holds.
				buf[off+AVPHeaderLen] ^= 0x80
				return off + n
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tun := newEstablishedTunnel(t, 0)
			defaults := TunnelDefaults{SharedSecret: tc.secret}

			var buf [256]byte
			off := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgICRQ))
			off += WriteAVPUint16(buf[:], off, true, AVPAssignedSessionID, 500)
			off += WriteAVPUint32(buf[:], off, true, AVPCallSerialNumber, 1001)
			off = tc.build(t, buf[:], off)

			out := processTestICRQ(t, tun, defaults, buf[:off])

			require.Equal(t, []MessageType{MsgCDN}, sentMessageTypes(t, out),
				"a mandatory AVP Ze cannot reveal refuses the session with CDN")
			require.Empty(t, tun.sessions, "the refused ICRQ creates no session")
			require.Equal(t, L2TPTunnelEstablished, tun.state, "a session message never clears the tunnel")
		})
	}
}

// TestHiddenOptionalAVPNotRevealedIsIgnored sends an ICRQ whose Calling
// Number is hidden with M=0 and no Random Vector, and reads that the session
// is created without it. RFC 2661 Section 7.1: "If the M-bit is not set, the
// AVP should be ignored (with the exception of logging a local error message)
// and the message accepted."
func TestHiddenOptionalAVPNotRevealedIsIgnored(t *testing.T) {
	tun := newEstablishedTunnel(t, 0)
	defaults := TunnelDefaults{SharedSecret: hiddenTestSecret}

	var buf [256]byte
	off := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgICRQ))
	off += WriteAVPUint16(buf[:], off, true, AVPAssignedSessionID, 500)
	off += WriteAVPUint32(buf[:], off, true, AVPCallSerialNumber, 1001)
	off += writeHiddenTestAVP(t, buf[:], off, false, AVPCallingNumber, hiddenTestRV, []byte(hiddenTestCallingNumber), 0)

	out := processTestICRQ(t, tun, defaults, buf[:off])

	require.Equal(t, []MessageType{MsgICRP}, sentMessageTypes(t, out), "the ICRQ is accepted")
	sess := onlyTestSession(t, tun)
	require.Empty(t, sess.callingNumber, "the unrevealed Calling Number is ignored, never read as plaintext")
}

// buildSCCRQWithHiddenTunnelID returns an SCCRQ whose Assigned Tunnel ID
// (M=1) is hidden under the test secret after a Random Vector AVP, with every
// other AVP RFC 2661 Section 6.1 makes mandatory in clear. With
// hiddenVendorName a hidden Vendor Name (M=0) follows it, so the message
// carries two hidden AVPs.
func buildSCCRQWithHiddenTunnelID(t *testing.T, peerTID uint16, hiddenVendorName bool) []byte {
	t.Helper()
	var buf [256]byte
	off := WriteAVPUint16(buf[:], 0, true, AVPMessageType, uint16(MsgSCCRQ))
	off += WriteAVPBytes(buf[:], off, true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	off += WriteAVPBytes(buf[:], off, true, 0, AVPFramingCapabilities, []byte{0x00, 0x00, 0x00, 0x03})
	off += WriteAVPString(buf[:], off, true, AVPHostName, "hidden-lac")
	off += writeRandomVectorTestAVP(buf[:], off, hiddenTestRV)
	off += writeHiddenTestAVP(t, buf[:], off, true, AVPAssignedTunnelID, hiddenTestRV, u16Bytes(peerTID), 0)
	if hiddenVendorName {
		off += writeHiddenTestAVP(t, buf[:], off, false, AVPVendorName, hiddenTestRV, []byte("example-vendor"), 0)
	}

	pkt := make([]byte, ControlHeaderLen+off)
	WriteControlHeader(pkt, 0, uint16(len(pkt)), 0, 0, 0, 0)
	copy(pkt[ControlHeaderLen:], buf[:off])
	return pkt
}

// TestSCCRQHiddenTunnelIDOffTheWire sends an SCCRQ whose Assigned Tunnel ID is
// hidden to a running reactor over UDP, once with the LAC's secret configured
// and once with none, and reads the reply datagram.
//
// The Assigned Tunnel ID is what the reply's header carries (RFC 2661 Section
// 4.4.3: "The L2TP peer MUST place this value in the Tunnel ID header field of
// all control and data messages that it subsequently transmits over the
// associated tunnel"), so an SCCRP addressed to it proves the reveal.
func TestSCCRQHiddenTunnelIDOffTheWire(t *testing.T) {
	const peerTID uint16 = 4242

	t.Run("revealed with the shared secret", func(t *testing.T) {
		clk := newTestClock(time.Unix(1_700_000_000, 0))
		ln, r, _, stop := buildLogReactorWithClock(t, clk, 60*time.Second, hiddenTestSecret)
		defer stop()
		client := newClient(t, ln)
		defer client.Close()

		client.Send(t, buildSCCRQWithHiddenTunnelID(t, peerTID, false))

		reply, got := recvControl(t, client, replyWait)
		require.True(t, got, "the SCCRQ is answered")
		hdr, err := ParseMessageHeader(reply)
		require.NoError(t, err)
		require.Equal(t, MsgSCCRP, extractMsgType(reply[hdr.PayloadOff:hdr.Length]))
		require.Equal(t, peerTID, hdr.TunnelID, "the SCCRP goes to the hidden Assigned Tunnel ID")
		require.Equal(t, 1, r.TunnelCount())
	})

	t.Run("refused with no shared secret", func(t *testing.T) {
		clk := newTestClock(time.Unix(1_700_000_000, 0))
		ln, r, _, stop := buildLogReactorWithClock(t, clk, 60*time.Second, "")
		defer stop()
		client := newClient(t, ln)
		defer client.Close()

		client.Send(t, buildSCCRQWithHiddenTunnelID(t, peerTID, false))

		reply, got := recvControl(t, client, replyWait)
		require.True(t, got, "a mandatory AVP Ze cannot reveal is answered, not dropped in silence")
		hdr, err := ParseMessageHeader(reply)
		require.NoError(t, err)
		info, perr := parseStopCCN(reply[hdr.PayloadOff:hdr.Length])
		require.NoError(t, perr)
		require.EqualValues(t, resultProtocolError, info.Result, "Result Code 2")
		require.EqualValues(t, errorValueOutOfRange, info.Error, "Error Code 3")
		require.Equal(t, errSCCRQHiddenNotRevealed.Detail, info.Message)
		require.Equal(t, 0, r.TunnelCount(), "the refusal allocates no tunnel")
	})
}

// hiddenLeftLogLine is the message of the line a message with AVPs left
// hidden earns (hiddenLeft.log).
const hiddenLeftLogLine = "hidden AVP left hidden"

// awaitSCCRP reads replies until an SCCRP arrives. The reactor handles one
// client's datagrams in order, so an SCCRP to a fence sent last proves every
// datagram before it was handled.
func awaitSCCRP(t *testing.T, client *testClient) {
	t.Helper()
	for range 4 {
		reply, got := recvControl(t, client, replyWait)
		require.True(t, got, "the fence SCCRQ is answered")
		hdr, err := ParseMessageHeader(reply)
		require.NoError(t, err)
		if extractMsgType(reply[hdr.PayloadOff:hdr.Length]) == MsgSCCRP {
			return
		}
	}
	t.Fatal("no SCCRP answered the fence SCCRQ")
}

// TestSCCRQHiddenNotRevealedLogBounded sends a burst of unauthenticated
// SCCRQs, each carrying two AVPs Ze cannot reveal (no secret configured),
// from one source within one stopCCNLimitInterval, then a plain SCCRQ as a
// fence, and counts the warning lines. It then moves the clock past the
// interval and sends one more.
//
// VALIDATES: one line per message, never one per hidden AVP (the line says
// count=2), and at most one line per source slot per interval on the
// TunnelID=0 path; the bound lifts once the interval passes.
// PREVENTS: a spoofed SCCRQ flood writing one log line per hidden AVP per
// datagram.
func TestSCCRQHiddenNotRevealedLogBounded(t *testing.T) {
	const burst = 20
	clk := newTestClock(time.Unix(1_700_000_000, 0))
	ln, _, logs, stop := buildLogReactorWithClock(t, clk, 60*time.Second, "")
	defer stop()
	client := newClient(t, ln)
	defer client.Close()

	for i := range burst {
		client.Send(t, buildSCCRQWithHiddenTunnelID(t, uint16(5000+i), true))
	}
	client.Send(t, buildSCCRQWithVendorAVP(t, false, 4300, "fence-peer"))
	awaitSCCRP(t, client)

	require.Equal(t, 1, strings.Count(logs.String(), hiddenLeftLogLine),
		"a burst from one source within one interval logs one line")
	require.Contains(t, logs.String(), "count=2", "the line counts both hidden AVPs of its message")

	clk.add(stopCCNLimitInterval)
	client.Send(t, buildSCCRQWithHiddenTunnelID(t, 6000, true))
	client.Send(t, buildSCCRQWithVendorAVP(t, false, 4301, "fence-peer-2"))
	awaitSCCRP(t, client)

	require.Equal(t, 2, strings.Count(logs.String(), hiddenLeftLogLine),
		"once the interval has passed the next such SCCRQ is logged again")
}
