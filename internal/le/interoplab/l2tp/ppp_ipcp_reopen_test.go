// Design: docs/architecture/testing/interop.md -- IPCP withdrawal and reopen after an injected Terminate-Request.
// Related: ppp_ipcp_reopen.go -- the predicates under test.
package l2tp

import "testing"

// TestIPCPReopenEvidence drives ipcpReopened over relay logs built here: a
// Configure-Ack from each side after Ze's Terminate-Ack passes, and each
// missing piece fails, including Configure-Acks that come only before the
// Terminate-Ack, which the startup negotiation leaves in every capture.
func TestIPCPReopenEvidence(t *testing.T) {
	frames := func(t *testing.T, text string) []pppFrame {
		t.Helper()
		parsed, err := relayPPPFrames(text)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	line := func(direction string, protocol uint16, code, id byte) string {
		datagram := pppDatagram(protocol, []byte{code, id, 0, 4})
		return wireLine(t, direction, datagram, datagram)
	}
	terminateAck := line(relayFromZe, protocolIPCP, codeTerminateAck, terminateIdentifier)
	zeAck := line(relayFromZe, protocolIPCP, codeConfigureAck, 7)
	lacAck := line(relayFromLAC, protocolIPCP, codeConfigureAck, 3)

	if err := ipcpReopened(frames(t, terminateAck+zeAck+lacAck)); err != nil {
		t.Fatalf("a Configure-Ack from each side after the Terminate-Ack was refused: %v", err)
	}
	if err := ipcpReopened(frames(t, zeAck+lacAck+terminateAck)); err == nil {
		t.Fatal("Configure-Acks before the Terminate-Ack were read as the reopen")
	}
	if err := ipcpReopened(frames(t, terminateAck+lacAck)); err == nil {
		t.Fatal("a reopen with no Ze Configure-Ack was accepted")
	}
	if err := ipcpReopened(frames(t, terminateAck+zeAck)); err == nil {
		t.Fatal("a reopen with no pppd Configure-Ack was accepted")
	}
	if err := ipcpReopened(frames(t, zeAck+lacAck)); err == nil {
		t.Fatal("Configure-Acks with no Terminate-Ack were read as a reopen")
	}
	otherAck := line(relayFromZe, protocolIPCP, codeTerminateAck, terminateIdentifier+1)
	if err := ipcpReopened(frames(t, otherAck+zeAck+lacAck)); err == nil {
		t.Fatal("a Terminate-Ack with another Identifier anchored the reopen")
	}
	lcpAcks := line(relayFromZe, protocolLCP, codeConfigureAck, 7) + line(relayFromLAC, protocolLCP, codeConfigureAck, 3)
	if err := ipcpReopened(frames(t, terminateAck+lcpAcks)); err == nil {
		t.Fatal("LCP Configure-Acks were read as an IPCP reopen")
	}
}

// TestPPPIPv4PairReadsIPAddrOutput reads the local and peer addresses from
// the `ip -4 -o addr show dev pppN` line Linux prints for a point-to-point
// PPP interface, and answers not held for an interface with no IPv4 address,
// which prints nothing.
func TestPPPIPv4PairReadsIPAddrOutput(t *testing.T) {
	output := "5: ppp0    inet 10.100.0.1 peer 10.100.0.2/32 scope global ppp0\\       valid_lft forever preferred_lft forever\n"
	local, peer, held := pppIPv4Pair(output)
	if !held {
		t.Fatal("an inet line was read as no IPv4 address")
	}
	if local != localPPPAddress {
		t.Fatalf("local address %q, want %s", local, localPPPAddress)
	}
	if peer != peerPPPAddress {
		t.Fatalf("peer address %q, want %s", peer, peerPPPAddress)
	}
	if _, _, held := pppIPv4Pair(""); held {
		t.Fatal("empty output was read as an IPv4 address")
	}
	inet6 := "5: ppp0    inet6 fe80::1 peer fe80::2/128 scope link\\       valid_lft forever preferred_lft forever\n"
	if _, _, held := pppIPv4Pair(inet6); held {
		t.Fatal("an inet6 line was read as an IPv4 address")
	}
}
