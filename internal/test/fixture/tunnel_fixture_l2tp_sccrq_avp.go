package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"time"
)

// tunnelL2TPSCCRQOmitting builds an SCCRQ carrying every AVP RFC 2661 Section
// 6.1 makes mandatory except omit, which is left out. Attribute numbers are
// written out of the RFC's own AVP table (Section 4.4): 0 Message Type, 2
// Protocol Version, 3 Framing Capabilities, 4 Bearer Capabilities, 7 Host
// Name, 9 Assigned Tunnel ID, 10 Receive Window Size. omit is 0xffff for a
// complete message.
func tunnelL2TPSCCRQOmitting(peerTID uint16, hostname string, omit uint16) []byte {
	body := make([]byte, 0, 128)
	add := func(attribute uint16, value []byte) {
		if attribute == omit {
			return
		}
		body = append(body, tunnelL2TPAVP(true, attribute, value)...)
	}
	add(0, tunnelL2TPU16(1))
	add(2, []byte{1, 0})
	add(3, tunnelL2TPU32(3))
	add(4, tunnelL2TPU32(0))
	add(7, []byte(hostname))
	add(9, tunnelL2TPU16(peerTID))
	add(10, tunnelL2TPU16(8))
	return tunnelL2TPControl(0, 0, 0, 0, body)
}

// tunnelL2TPResultCode unpacks the Result Code AVP (attribute 1) of a StopCCN
// into its three RFC 2661 Section 4.4.2 fields: the 2-octet Result Code, the
// optional 2-octet Error Code, and the optional Error Message that follows it.
func tunnelL2TPResultCode(avps map[uint16][]byte) (uint16, uint16, string, error) {
	value := avps[1]
	if len(value) < 4 {
		return 0, 0, "", fmt.Errorf("result code AVP is %d octets, want at least 4", len(value))
	}
	return binary.BigEndian.Uint16(value[0:2]), binary.BigEndian.Uint16(value[2:4]), string(value[4:]), nil
}

// tunnelL2TPMandatoryAVP drives the RFC 2661 Section 6.1 mandatory-AVP
// obligation against a running ze: an SCCRQ that omits Framing Capabilities is
// answered with a StopCCN naming that AVP, and a complete SCCRQ still gets an
// SCCRP from the same daemon. Every field is unpacked here with
// encoding/binary against values written out of the RFC, never with ze's own
// parser.
func tunnelL2TPMandatoryAVP(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	conn, target, err := tunnelL2TPDial(port)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture teardown

	failures := make([]string, 0)
	check := func(ok bool, message string) {
		if !ok {
			failures = append(failures, message)
			fmt.Fprintln(os.Stderr, "FAIL:", message)
			return
		}
		fmt.Println("PASS:", message)
	}

	// Attribute 3 is Framing Capabilities, which RFC 2661 Section 6.1 makes
	// mandatory in an SCCRQ. Before this test ze checked no such presence and
	// brought the tunnel up with a framing mask of 0.
	bad, _, err := tunnelL2TPExchange(ctx, conn, target,
		tunnelL2TPSCCRQOmitting(0x0404, "py-no-framing", 3), 40, 250*time.Millisecond)
	if err != nil {
		return errors.New("no reply to an SCCRQ missing the Framing Capabilities AVP")
	}
	if len(bad) < 12 {
		return fmt.Errorf("reply is %d octets, want at least 12", len(bad))
	}
	check(int(binary.BigEndian.Uint16(bad[2:4])) == len(bad), "StopCCN Length field matches datagram")
	check(binary.BigEndian.Uint16(bad[4:6]) == 0, "StopCCN header Tunnel ID is 0")
	check(binary.BigEndian.Uint16(bad[8:10]) == 0, "StopCCN Ns is 0")
	check(binary.BigEndian.Uint16(bad[10:12]) == 1, "StopCCN Nr acknowledges the SCCRQ")
	avps, err := tunnelL2TPParseAVPs(bad)
	if err != nil {
		return err
	}
	check(tunnelL2TPMessageType(avps) == 4, "reply is StopCCN")
	result, errorCode, message, err := tunnelL2TPResultCode(avps)
	if err != nil {
		return err
	}
	check(result == 2, "Result Code is 2")
	check(errorCode == 3, "Error Code is 3")
	check(message == "SCCRQ missing Framing Capabilities AVP", "Error Message names the missing AVP")

	// The positive polarity on the same daemon and the same socket: a complete
	// SCCRQ is accepted, so the refusal above cannot have come from a listener
	// that refuses everything.
	good, _, err := tunnelL2TPExchange(ctx, conn, target,
		tunnelL2TPSCCRQOmitting(0x0505, "py-complete", 0xffff), 40, 250*time.Millisecond)
	if err != nil {
		return errors.New("no reply to a complete SCCRQ")
	}
	goodAVPs, err := tunnelL2TPParseAVPs(good)
	if err != nil {
		return err
	}
	check(binary.BigEndian.Uint16(good[4:6]) == 0x0505, "SCCRP addressed with the peer's Assigned Tunnel ID")
	check(tunnelL2TPMessageType(goodAVPs) == 2, "a complete SCCRQ gets SCCRP")
	check(len(goodAVPs[3]) == 4, "SCCRP carries the Framing Capabilities AVP Section 6.2 makes mandatory")

	if len(failures) != 0 {
		return fmt.Errorf("%d mandatory-AVP checks failed", len(failures))
	}
	fmt.Println("OK: all RFC 2661 SCCRQ mandatory AVP checks passed")
	return nil
}
