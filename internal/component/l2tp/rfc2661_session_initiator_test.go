package l2tp

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/callsink"
)

// --- Session-message encoder / parser round-trips (AC-3, AC-4 codecs) ---

func TestWriteICRQBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeICRQBody produces a body the existing parseICRQ
	// accepts, with the required + optional AVPs surviving.
	buf := make([]byte, 512)
	n := writeICRQBody(buf, 0x0501, 12345, 6, "5551000", "5552000")
	info, err := parseICRQ(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x0501, info.assignedSessionID)
	require.EqualValues(t, 12345, info.callSerialNumber)
	require.EqualValues(t, 6, info.bearerType)
	require.Equal(t, "5551000", info.calledNumber)
	require.Equal(t, "5552000", info.callingNumber)
}

func TestWriteICRQBody_MinimalRoundTrip(t *testing.T) {
	// VALIDATES: omitting optional called/calling numbers still parses.
	buf := make([]byte, 256)
	n := writeICRQBody(buf, 7, 1, 0, "", "")
	info, err := parseICRQ(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 7, info.assignedSessionID)
	require.EqualValues(t, 1, info.callSerialNumber)
}

func TestWriteICCNBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeICCNBody round-trips through parseICCN.
	buf := make([]byte, 256)
	n := writeICCNBody(buf, 64000, 0x1, nil)
	info, err := parseICCN(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 64000, info.txConnectSpeed)
	require.EqualValues(t, 0x1, info.framingType)
}

func TestWriteOCRQBody_RoundTrip(t *testing.T) {
	// VALIDATES: writeOCRQBody round-trips through parseOCRQ with all the
	// RFC 2661 S7.9 required AVPs present.
	buf := make([]byte, 512)
	n := writeOCRQBody(buf, 0x0A0B, 999, 9600, 128000, 1, 0x1, "5559999")
	info, err := parseOCRQ(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x0A0B, info.assignedSessionID)
	require.EqualValues(t, 999, info.callSerialNumber)
	require.EqualValues(t, 9600, info.minimumBPS)
	require.EqualValues(t, 128000, info.maximumBPS)
	require.EqualValues(t, 1, info.bearerType)
	require.EqualValues(t, 0x1, info.framingType)
	require.Equal(t, "5559999", info.calledNumber)
}

func TestParseICRP_RoundTrip(t *testing.T) {
	// VALIDATES: parseICRP reads back a body from the existing writeICRPBody
	// encoder and captures the Assigned Session ID.
	buf := make([]byte, 64)
	n := writeICRPBody(buf, 0x4242)
	info, err := parseICRP(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x4242, info.assignedSessionID)
}

func TestParseICRP_Rejects(t *testing.T) {
	// VALIDATES: parseICRP rejects a missing / zero Assigned Session ID.
	_, err := parseICRP(nil)
	require.Error(t, err)

	buf := make([]byte, 32)
	n := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgICRP))
	_, err = parseICRP(buf[:n])
	require.Error(t, err)

	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgICRP))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, 0)
	_, err = parseICRP(buf[:off])
	require.Error(t, err)
}

func TestParseOCRP_RoundTrip(t *testing.T) {
	// VALIDATES: parseOCRP reads back a body from writeOCRPBody.
	buf := make([]byte, 64)
	n := writeOCRPBody(buf, 0x9001)
	info, err := parseOCRP(buf[:n])
	require.NoError(t, err)
	require.EqualValues(t, 0x9001, info.assignedSessionID)
}

func TestParseOCRP_Rejects(t *testing.T) {
	_, err := parseOCRP(nil)
	require.Error(t, err)

	buf := make([]byte, 32)
	off := WriteAVPUint16(buf, 0, true, AVPMessageType, uint16(MsgOCRP))
	off += WriteAVPUint16(buf, off, true, AVPAssignedSessionID, 0)
	_, err = parseOCRP(buf[:off])
	require.Error(t, err)
}

// testProxyLCP is a captured LCP exchange: the first request received, the
// LAC's Acked request and the subscriber's Acked request (after one Nak).
var testProxyLCP = callsink.ProxyLCP{
	InitialReceived: []byte{0x01, 0x04, 0x05, 0xdc},
	LastSent:        []byte{0x03, 0x05, 0xc2, 0x23, 0x05},
	LastReceived:    []byte{0x01, 0x04, 0x05, 0xd4},
}

// RFC requirement: RFC2661-4.4.5-1 positive -- a CHAP realm call's ICCN carries Proxy Authen Type (AVP 29) with value 2.
// RFC requirement: RFC2661-4.4.5-2 positive -- that ICCN carries Proxy Authen Name (AVP 30) holding the username.
// RFC requirement: RFC2661-4.4.5-3 positive -- that ICCN carries Proxy Authen Challenge (AVP 31) holding the challenge sent.
// RFC requirement: RFC2661-4.4.5-4 positive -- Proxy Authen ID (AVP 32) is written with its most significant octet 0.
// RFC requirement: RFC2661-4.4.5-5 positive -- that ICCN carries Proxy Authen ID (AVP 32) holding the CHAP Identifier.
// RFC requirement: RFC2661-4.4.5-6 positive -- that ICCN carries Proxy Authen Response (AVP 33) holding the response.
// RFC requirement: RFC2661-4.4.5-7 positive -- every proxy AVP 26 to 33 of that ICCN carries M=0.
// TestWriteICCNBodyProxyAVPs pins the byte layout of the proxy AVPs a CHAP
// realm call appends to its ICCN (spec-home-cpe-14 AC-2).
//
// VALIDATES: after the three required AVPs come 26, 27, 28, 29, 30, 31, 32,
// 33 in that order; each header has M=0 and H=0 (flag word 0x0000 | length);
// AVP 29 is 0x0002; AVP 32 is 0x00 then the CHAP Identifier.
// PREVENTS: RFC2661-4.4.5-1 to -6 regressing (an absent AVP), an M bit of 1
// on a proxy AVP, a non-zero high octet in Proxy Authen ID.
func TestWriteICCNBodyProxyAVPs(t *testing.T) {
	auth := callsink.ProxyAuth{Kind: callsink.AuthKindCHAP, Identifier: 0xff,
		Challenge: bytes.Repeat([]byte{0x11}, 16), Response: bytes.Repeat([]byte{0x22}, 16)}
	proxy, reason, fits := newICCNProxy("cpe@example.net", testProxyLCP, auth, "")
	require.True(t, fits)
	require.Empty(t, reason, "CHAP needs no secret")
	buf := make([]byte, BufSize)
	base := writeICCNBody(buf, 64000, 1, nil)
	n := writeICCNBody(buf, 64000, 1, proxy)

	want := []byte{
		0x00, 0x0a, 0x00, 0x00, 0x00, 26, 0x01, 0x04, 0x05, 0xdc,
		0x00, 0x0b, 0x00, 0x00, 0x00, 27, 0x03, 0x05, 0xc2, 0x23, 0x05,
		0x00, 0x0a, 0x00, 0x00, 0x00, 28, 0x01, 0x04, 0x05, 0xd4,
		0x00, 0x08, 0x00, 0x00, 0x00, 29, 0x00, 0x02,
	}
	want = append(want, 0x00, byte(6+len("cpe@example.net")), 0x00, 0x00, 0x00, 30)
	want = append(want, "cpe@example.net"...)
	want = append(want, 0x00, 22, 0x00, 0x00, 0x00, 31)
	want = append(want, auth.Challenge...)
	want = append(want,
		0x00, 0x08, 0x00, 0x00, 0x00, 32, 0x00, 0xff,
		0x00, 22, 0x00, 0x00, 0x00, 33)
	want = append(want, auth.Response...)
	require.Equal(t, want, buf[base:n])

	info, err := parseICCN(buf[:n])
	require.NoError(t, err, "the LNS-side parser still accepts the ICCN")
	require.EqualValues(t, 64000, info.txConnectSpeed)
}

// RFC requirement: RFC2661-4.3-3 positive -- a PAP realm call toward a remote with no shared secret carries no Proxy Authen AVPs (29 to 33), no Random Vector, and not the password.
// TestWriteICCNBodyPAPProxyHiddenOrAbsent covers AC-3 and AC-4.
//
// VALIDATES: a PAP subscriber toward a remote with a shared secret gets AVP 29
// type 3, 30, 32 (the Authenticate-Request Identifier) and a hidden 33, and
// no AVP 31; toward a remote without a secret it gets AVPs 26 to 28 only and
// the reason no-shared-secret.
// PREVENTS: a PAP password leaving the LAC unhidden.
func TestWriteICCNBodyPAPProxyHiddenOrAbsent(t *testing.T) {
	auth := callsink.ProxyAuth{Kind: callsink.AuthKindPAP, Identifier: 3, Response: []byte("s3cret-pass")}

	proxy, reason, fits := newICCNProxy("cpe@example.net", testProxyLCP, auth, "")
	require.True(t, fits)
	require.Equal(t, proxyAuthReasonNoSharedSecret, reason)
	buf := make([]byte, BufSize)
	n := writeICCNBody(buf, 64000, 1, proxy)
	order, _ := avpsOf(t, buf[:n])
	require.Equal(t, []AVPType{AVPMessageType, AVPTxConnectSpeed, AVPFramingType,
		AVPInitialReceivedLCPConfReq, AVPLastSentLCPConfReq, AVPLastReceivedLCPConfReq}, order)
	require.False(t, bytes.Contains(buf[:n], auth.Response), "no password on the wire")

	proxy, reason, fits = newICCNProxy("cpe@example.net", testProxyLCP, auth, "tunnel-secret")
	require.True(t, fits)
	require.Empty(t, reason)
	n = writeICCNBody(buf, 64000, 1, proxy)
	order, avps := avpsOf(t, buf[:n])
	require.Equal(t, []AVPType{AVPMessageType, AVPTxConnectSpeed, AVPFramingType,
		AVPInitialReceivedLCPConfReq, AVPLastSentLCPConfReq, AVPLastReceivedLCPConfReq,
		AVPProxyAuthenType, AVPProxyAuthenName, AVPProxyAuthenID, AVPRandomVector, AVPProxyAuthenResponse}, order)
	require.Equal(t, []byte{0x00, 0x03}, avps[AVPProxyAuthenType].value)
	require.Equal(t, []byte{0x00, 0x03}, avps[AVPProxyAuthenID].value)
	require.False(t, bytes.Contains(buf[:n], auth.Response), "the password travels hidden")
}

// RFC requirement: RFC2661-4.3-1 positive -- in a PAP realm call's ICCN a Random Vector AVP precedes AVP 33, the only AVP with H=1.
// TestWriteICCNBodyHiddenAVP33 checks the hiding of AVP 33 for PAP (AC-3).
//
// VALIDATES: RFC2661-4.3-1 -- a Random Vector AVP (M=1, H=0) precedes AVP 33,
// the only AVP with H=1; RFC2661-4.3-3 -- H is set only with a secret, and
// HiddenDecrypt with that secret and the vector yields the password.
// PREVENTS: an unreadable AVP 33 at the LNS, or a hidden AVP with no vector.
func TestWriteICCNBodyHiddenAVP33(t *testing.T) {
	auth := callsink.ProxyAuth{Kind: callsink.AuthKindPAP, Identifier: 3, Response: []byte("s3cret-pass")}
	secret := []byte("tunnel-secret")
	proxy, _, fits := newICCNProxy("cpe@example.net", testProxyLCP, auth, string(secret))
	require.True(t, fits)
	buf := make([]byte, BufSize)
	n := writeICCNBody(buf, 64000, 1, proxy)

	var rv []byte
	hidden := 0
	it := NewAVPIterator(buf[:n])
	for {
		_, attrType, flags, value, ok := it.Next()
		if !ok {
			break
		}
		if attrType == AVPRandomVector {
			require.NotZero(t, flags&FlagMandatory, "Random Vector has M=1")
			require.Zero(t, flags&FlagHidden, "Random Vector is never hidden")
			require.Len(t, value, randomVectorOctets)
			rv = append([]byte(nil), value...)
			continue
		}
		if flags&FlagHidden == 0 {
			continue
		}
		hidden++
		require.Equal(t, AVPProxyAuthenResponse, attrType, "only AVP 33 is hidden")
		require.Zero(t, flags&FlagMandatory, "AVP 33 has M=0")
		require.NotNil(t, rv, "a Random Vector precedes the hidden AVP")
		plain, err := HiddenDecrypt(make([]byte, len(value)), attrType, secret, rv, value)
		require.NoError(t, err)
		require.Equal(t, auth.Response, plain)
	}
	require.NoError(t, it.Err())
	require.Equal(t, 1, hidden)
}

// TestICCNProxyRefusesOversizedLCP bounds the subscriber-controlled LCP bodies:
// a body that cannot fit one AVP yields no proxy data instead of a panic in
// WriteAVPHeader.
func TestICCNProxyRefusesOversizedLCP(t *testing.T) {
	lcp := callsink.ProxyLCP{InitialReceived: make([]byte, AVPMaxLen-AVPHeaderLen+1)}
	_, _, fits := newICCNProxy("u@r", lcp, callsink.ProxyAuth{Kind: callsink.AuthKindCHAP}, "")
	require.False(t, fits)
	lcp = callsink.ProxyLCP{InitialReceived: make([]byte, 400), LastSent: make([]byte, 400), LastReceived: make([]byte, 400)}
	_, _, fits = newICCNProxy("u@r", lcp, callsink.ProxyAuth{Kind: callsink.AuthKindCHAP}, "")
	require.False(t, fits, "the set stays within one control buffer")
}
