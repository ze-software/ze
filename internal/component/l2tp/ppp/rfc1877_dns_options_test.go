// VALIDATES: RFC 1877 IPCP DNS-option negotiation on the LNS side -- an acceptable
// Configure-Request is Acked with its option Data echoed verbatim, and an unsupported
// option is Configure-Rejected with only the offending option echoed.
// PREVENTS: an Ack that rewrites the peer's option data, or a Reject that drops or
// mangles the offending option (or wrongly echoes a recognized one).
package ppp

import (
	"bytes"
	"testing"
)

// RFC requirement: RFC1877-x-3 positive -- when the peer's IPCP Configure-Request
// carries acceptable values, Ze's Configure-Ack echoes the option Data verbatim
// (producer sendNCPConfigureAck writes req.Data unchanged,
// internal/component/l2tp/ppp/ncp.go:567).
// RFC requirement: RFC1877-x-3 negative -- Ze does not Ack an UNacceptable request:
// an IP-Address that differs from the assigned peer address draws a Configure-Nak,
// so the verbatim-echo Ack is confined to acceptable requests.
func TestRFC1877ConfigureAckEchoesAcceptable(t *testing.T) {
	t.Run("acceptable request is Acked verbatim", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()

		cr := td.readPeerNCPPacket(t, ProtoIPCP)
		if cr.Code != LCPConfigureRequest {
			t.Fatalf("got code %d, want Configure-Request", cr.Code)
		}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)

		// Acceptable: the assigned peer address (10.0.0.2) plus Ze's configured DNS.
		acceptable := []byte{
			IPCPOptIPAddress, 6, 10, 0, 0, 2,
			IPCPOptPrimaryDNS, 6, 1, 1, 1, 1,
			IPCPOptSecondaryDNS, 6, 8, 8, 8, 8,
		}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x30, acceptable)

		ack := readIPCPUntil(t, td, LCPConfigureAck)
		if !bytes.Equal(ack.Data, acceptable) {
			t.Errorf("Configure-Ack Data = % x, want the request echoed verbatim % x", ack.Data, acceptable)
		}
	})

	t.Run("unacceptable request draws a Nak, not an Ack", func(t *testing.T) {
		td := newNCPTestDriverCfg(t, &StartSession{DisableIPv6CP: true})
		defer td.cleanup()

		cr := td.readPeerNCPPacket(t, ProtoIPCP)
		if cr.Code != LCPConfigureRequest {
			t.Fatalf("got code %d, want Configure-Request", cr.Code)
		}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureAck, cr.Identifier, cr.Data)

		// IP-Address 9.9.9.9 does not match the assigned peer address 10.0.0.2.
		unacceptable := []byte{IPCPOptIPAddress, 6, 9, 9, 9, 9}
		td.writePeerNCPPacket(t, ProtoIPCP, LCPConfigureRequest, 0x31, unacceptable)

		nak := readIPCPUntil(t, td, LCPConfigureNak)
		if nak.Code != LCPConfigureNak {
			t.Fatalf("got code %d, want Configure-Nak for the unacceptable IP-Address", nak.Code)
		}
	})
}

// RFC requirement: RFC1877-x-4 positive -- a Configure-Reject echoes the offending
// unsupported option verbatim (RFC 1877 / RFC 1661 sec 5.4; producer buildNakOrReject
// -> copyUnknownOptions, internal/component/l2tp/ppp/ncp.go:590,619).
// RFC requirement: RFC1877-x-4 negative -- only the unsupported option is echoed, NOT
// the recognized ones: copyUnknownOptions copies the unknown type and skips the known
// IP-Address option.
func TestRFC1877ConfigureRejectEchoesUnsupportedOnly(t *testing.T) {
	// A known IP-Address (type 3) option followed by an unknown type-99 option.
	unknownOpt := []byte{99, 4, 0xDE, 0xAD}
	src := append([]byte{IPCPOptIPAddress, 6, 10, 0, 0, 2}, unknownOpt...)

	buf := make([]byte, 32)
	n := copyUnknownOptions(src, isKnownIPCPOption, buf, 0)
	got := buf[:n]

	if !bytes.Equal(got, unknownOpt) {
		t.Errorf("Reject Data = % x, want only the unsupported option echoed verbatim % x", got, unknownOpt)
	}
	if bytes.Contains(got, []byte{IPCPOptIPAddress, 6}) {
		t.Errorf("Reject Data = % x must not echo the recognized IP-Address option", got)
	}
}
