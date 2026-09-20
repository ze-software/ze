// Design: internal/component/l2tp/errors.go -- the SCCRQ rejections and their General Error Codes
// Related: internal/component/l2tp/tunnel_fsm.go -- parseSCCRQ, which raises them
// Related: internal/component/l2tp/reactor.go -- sendUnassociatedStopCCN, which writes them

package l2tp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// vendorCisco is a vendor id ze recognizes no AVP of. RFC 2661 Section 4.1:
// "The Vendor ID field is the SMI Network Management Private Enterprise Code",
// and 9 is Cisco's.
const vendorCisco uint16 = 9

// buildSCCRQWithVendorAVP returns an SCCRQ carrying every AVP RFC 2661 Section
// 6.1 makes mandatory, plus one vendor-specific AVP ze does not recognize. The
// M-bit of that one AVP is the only thing a case changes.
func buildSCCRQWithVendorAVP(t *testing.T, mandatory bool, peerTID uint16, hostName string) []byte {
	t.Helper()
	bodyBuf := GetBuf()
	defer PutBuf(bodyBuf)
	buf := *bodyBuf
	off := 0
	write := func(m bool, vendorID uint16, attr AVPType, value []byte) {
		off += WriteAVPBytes(buf, off, m, vendorID, attr, value)
	}
	write(true, 0, AVPMessageType, u16Bytes(uint16(MsgSCCRQ)))
	write(true, 0, AVPProtocolVersion, []byte{0x01, 0x00})
	write(true, 0, AVPFramingCapabilities, []byte{0x00, 0x00, 0x00, 0x03})
	write(true, 0, AVPBearerCapabilities, []byte{0x00, 0x00, 0x00, 0x00})
	write(true, 0, AVPHostName, []byte(hostName))
	write(true, 0, AVPAssignedTunnelID, u16Bytes(peerTID))
	write(true, 0, AVPReceiveWindowSize, u16Bytes(8))
	write(mandatory, vendorCisco, AVPType(1), []byte{0xde, 0xad})

	total := ControlHeaderLen + off
	pkt := make([]byte, total)
	WriteControlHeader(pkt, 0, uint16(total), 0, 0, 0, 0)
	copy(pkt[ControlHeaderLen:], buf[:off])
	return pkt
}

// VALIDATES: an SCCRQ carrying an unknown vendor-specific AVP with the M-bit
//
//	set is answered on the wire with a StopCCN whose General Error Code is 8,
//	the code RFC 2661 Section 4.4.2 reserves for exactly this cause, and the
//	same SCCRQ with the M-bit clear establishes the tunnel.
//
// PREVENTS: the two failures this pair was written for. parseSCCRQ returned a
//
//	plain error for the M-bit case, so the reactor logged the datagram and
//	dropped it and the peer learned nothing; and the emitter wrote a constant
//	Error Code 3, which names a field out of range rather than an unknown
//	mandatory AVP.
//
// RFC 2661 Section 4.2: "Receipt of an unknown AVP that has the M-bit set is
// catastrophic to the session or tunnel it is associated with", and "if the
// M-bit is not set, the AVP is ignored and the message is accepted". Section
// 4.4.2 gives the code: "8 - Session or tunnel was shutdown due to receipt of
// an unknown AVP with the M-bit set".
func TestSCCRQUnknownVendorAVPAnsweredByMBit(t *testing.T) {
	t.Run("m-bit set is answered with error code 8", func(t *testing.T) {
		clk := newTestClock(time.Unix(1_700_000_000, 0))
		ln, r, _, stop := buildZeroTIDReactor(t, clk)
		defer stop()

		client := newClient(t, ln)
		defer client.Close()

		client.Send(t, buildSCCRQWithVendorAVP(t, true, 4242, "vendor-peer"))

		reply, got := recvControl(t, client, replyWait)
		require.True(t, got, "an unknown mandatory AVP must be answered, not dropped in silence")

		hdr, err := ParseMessageHeader(reply)
		require.NoError(t, err)
		require.True(t, hdr.IsControl)

		info, perr := parseStopCCN(reply[hdr.PayloadOff:hdr.Length])
		require.NoError(t, perr)
		require.EqualValues(t, resultProtocolError, info.Result, "Result Code 2")
		require.EqualValues(t, errorUnknownMandatoryAVP, info.Error,
			"Error Code 8 names an unknown AVP with the M-bit set; 3 would name a field out of range")
		require.Equal(t, errSCCRQUnknownMandatoryVendorAVP.Detail, info.Message)

		require.Equal(t, 0, r.TunnelCount(), "the refusal allocates no tunnel")
	})

	t.Run("m-bit clear is ignored and the tunnel establishes", func(t *testing.T) {
		clk := newTestClock(time.Unix(1_700_000_000, 0))
		ln, r, _, stop := buildZeroTIDReactor(t, clk)
		defer stop()

		client := newClient(t, ln)
		defer client.Close()

		client.Send(t, buildSCCRQWithVendorAVP(t, false, 4243, "vendor-peer"))

		reply, got := recvControl(t, client, replyWait)
		require.True(t, got, "an SCCRQ whose only unknown AVP has the M-bit clear must be accepted")

		hdr, err := ParseMessageHeader(reply)
		require.NoError(t, err)
		require.EqualValues(t, MsgSCCRP, extractMsgType(reply[hdr.PayloadOff:hdr.Length]),
			"an ignorable unknown AVP gets SCCRP, never StopCCN")
		require.Equal(t, 1, r.TunnelCount())
	})
}
