// Design: docs/architecture/testing/interop.md -- NCP lifetime checker evidence readers.
// Related: ncp_lifetime.go -- the readers and assertions under test.
package l2tp

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// captureBuilder writes relay capture rows the way avprelay logs them: normal
// data rows numbered from one, injections and control messages unnumbered.
type captureBuilder struct {
	t       *testing.T
	lines   []string
	ordinal int
}

func (c *captureBuilder) row(direction string, datagram []byte, ordinal int) {
	c.t.Helper()
	row := map[string]any{"direction": direction, "original": hex.EncodeToString(datagram),
		"forwarded": hex.EncodeToString(datagram)}
	if ordinal > 0 {
		row["data_ordinal"] = ordinal
		row["code_reject_armed"] = false
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		c.t.Fatal(err)
	}
	c.lines = append(c.lines, string(encoded))
}

// ppp logs one PPP frame: protocol, then Code, Identifier, Length and data.
func (c *captureBuilder) ppp(direction string, protocol uint16, code, identifier byte, data ...byte) {
	c.t.Helper()
	datagram := []byte{0x40, 0x02, 0, 0, 0x12, 0x34, 0x56, 0x78, 0xff, 0x03, byte(protocol >> 8), byte(protocol),
		code, identifier, 0, 0}
	datagram = append(datagram, data...)
	binary.BigEndian.PutUint16(datagram[2:], uint16(len(datagram)))
	binary.BigEndian.PutUint16(datagram[14:], uint16(4+len(data)))
	ordinal := 0
	if direction != relayInjected {
		c.ordinal++
		ordinal = c.ordinal
	}
	c.row(direction, datagram, ordinal)
}

// control logs one L2TP control message carrying messageType, or a ZLB.
func (c *captureBuilder) control(direction string, messageType uint16, zlb bool) {
	c.t.Helper()
	datagram := []byte{0xc8, 0x02, 0, 12, 0x12, 0x34, 0x56, 0x78, 0, 1, 0, 1}
	if !zlb {
		datagram = append(datagram, 0x80, 8, 0, 0, 0, 0, byte(messageType>>8), byte(messageType))
		binary.BigEndian.PutUint16(datagram[2:], uint16(len(datagram)))
	}
	c.row(direction, datagram, 0)
}

func (c *captureBuilder) text() string { return strings.Join(c.lines, "\n") + "\n" }

func (c *captureBuilder) frames() []pppFrame {
	c.t.Helper()
	frames, err := sessionPPPFrames(c.text())
	if err != nil {
		c.t.Fatal(err)
	}
	return frames
}

// TestSessionPPPFramesRefusesAnIncompleteCapture proves the capture reader
// keeps every frame in order, injections included, and refuses a capture
// whose normal data ordinals do not run from one without a gap, so an
// absence assertion can never read a truncated log as silence.
func TestSessionPPPFramesRefusesAnIncompleteCapture(t *testing.T) {
	capture := &captureBuilder{t: t}
	capture.ppp(relayFromLAC, protocolLCP, 1, 1)
	capture.control(relayFromZe, 0, true)
	capture.ppp(relayInjected, protocolCCP, 1, ccpIdentifier)
	capture.ppp(relayFromZe, protocolLCP, codeProtocolReject, 2, 0x80, 0xfd, 1, ccpIdentifier, 0, 4)
	frames := capture.frames()
	if len(frames) != 3 {
		t.Fatalf("%d frames, want 3", len(frames))
	}
	if frames[1].direction != relayInjected {
		t.Fatalf("second frame from %s, want the injection", frames[1].direction)
	}

	truncated := &captureBuilder{t: t, ordinal: 1}
	truncated.ppp(relayFromZe, protocolLCP, 1, 1)
	if _, err := sessionPPPFrames(truncated.text()); err == nil {
		t.Fatal("accepted a capture whose first data ordinal is 2")
	}
	gap := &captureBuilder{t: t}
	gap.ppp(relayFromZe, protocolLCP, 1, 1)
	gap.ordinal++
	gap.ppp(relayFromLAC, protocolLCP, 2, 1)
	if _, err := sessionPPPFrames(gap.text()); err == nil {
		t.Fatal("accepted a capture with a data ordinal gap")
	}
	unknown := &captureBuilder{t: t}
	unknown.ppp(relayFromZe, protocolLCP, 1, 1)
	unknown.ppp("inject-compressed-lcp", protocolLCP, 9, 1)
	if _, err := sessionPPPFrames(unknown.text()); err == nil {
		t.Fatal("accepted a row with a direction these scenarios never log")
	}
}

// TestZeControlMessagesCountsOnlyZeCDN proves the CDN count reads only Ze's
// control messages of that type, skips a ZLB, and refuses a control message
// it cannot read.
func TestZeControlMessagesCountsOnlyZeCDN(t *testing.T) {
	capture := &captureBuilder{t: t}
	capture.ppp(relayFromZe, protocolLCP, 1, 1)
	capture.control(relayFromZe, 0, true)
	capture.control(relayFromZe, 6, false)
	capture.control(relayFromLAC, cdnMessageType, false)
	count, err := zeCallDisconnects(capture.text())
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("counted %d CDNs with none from ze", count)
	}
	capture.control(relayFromZe, cdnMessageType, false)
	count, err = zeCallDisconnects(capture.text())
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("counted %d CDNs, want 1", count)
	}
	broken := &captureBuilder{t: t}
	broken.row(relayFromZe, []byte{0xc8, 0x02, 0, 20, 0, 1, 0, 2, 0, 1, 0, 1, 0, 8, 0, 0, 0, 0, 0, 14}, 0)
	if _, err := zeCallDisconnects(broken.text()); err == nil {
		t.Fatal("accepted a control message that does not open with its Message Type AVP")
	}
}

// TestNoIPIPCPAfterProtocolRejectIsRefused proves the noip scenario's
// discriminating assertion: an IPCP Configure-Request Ze sends after pppd's
// Protocol-Reject naming 0x8021 is refused, and none after it passes. A
// Protocol-Reject that names another protocol, or precedes every Ze IPCP
// Configure-Request, is not taken for pppd's rejection of Ze's IPCP.
func TestNoIPIPCPAfterProtocolRejectIsRefused(t *testing.T) {
	stopped := &captureBuilder{t: t}
	stopped.ppp(relayFromZe, protocolIPCP, 1, 1, 3, 6, 10, 100, 0, 1)
	stopped.ppp(relayFromLAC, protocolLCP, codeProtocolReject, 7, 0x80, 0x21, 1, 1, 0, 10)
	stopped.ppp(relayFromLAC, protocolIPv6CP, 1, 1)
	stopped.ppp(relayFromZe, protocolLCP, codeProtocolReject, 2, 0x80, 0x57, 1, 1, 0, 4)
	frames := stopped.frames()
	reject, err := ipcpRejectedByPeer(frames)
	if err != nil {
		t.Fatal(err)
	}
	if err := requireNoPacketsFrom(frames, reject+1, relayFromZe, protocolIPCP, "the reject"); err != nil {
		t.Fatal(err)
	}
	if _, err := ipv6cpRejected(frames); err != nil {
		t.Fatal(err)
	}

	stopped.ppp(relayFromZe, protocolIPCP, 1, 2, 3, 6, 10, 100, 0, 1)
	frames = stopped.frames()
	if err := requireNoPacketsFrom(frames, reject+1, relayFromZe, protocolIPCP, "the reject"); err == nil {
		t.Fatal("accepted ze's IPCP Configure-Request after pppd's Protocol-Reject naming 0x8021")
	}

	early := &captureBuilder{t: t}
	early.ppp(relayFromLAC, protocolLCP, codeProtocolReject, 7, 0x80, 0x21, 1, 1, 0, 4)
	early.ppp(relayFromZe, protocolIPCP, 1, 1)
	if _, err := ipcpRejectedByPeer(early.frames()); err == nil {
		t.Fatal("accepted a Protocol-Reject that precedes every ze IPCP Configure-Request")
	}
	other := &captureBuilder{t: t}
	other.ppp(relayFromZe, protocolIPCP, 1, 1)
	other.ppp(relayFromLAC, protocolLCP, codeProtocolReject, 7, 0x80, 0xfd, 1, 1, 0, 4)
	if _, err := ipcpRejectedByPeer(other.frames()); err == nil {
		t.Fatal("took a Protocol-Reject naming CCP for one naming IPCP")
	}
}

// TestIPv6CPParkedAfterZeProtocolReject proves the home-line scenario's
// IPv6CP assertions: pppd must offer IPv6CP, Ze must Protocol-Reject it and
// send no IPv6CP itself, and pppd must send no IPv6CP after the reject.
func TestIPv6CPParkedAfterZeProtocolReject(t *testing.T) {
	parked := &captureBuilder{t: t}
	parked.ppp(relayFromLAC, protocolIPv6CP, 1, 1, 1, 10, 0, 0, 0, 0, 0, 0, 0, 1)
	parked.ppp(relayFromZe, protocolLCP, codeProtocolReject, 3, 0x80, 0x57, 1, 1, 0, 14)
	parked.ppp(relayFromLAC, 0x0021, 0x45, 0)
	if err := requireIPv6CPParked(parked.frames()); err != nil {
		t.Fatal(err)
	}
	parked.ppp(relayFromLAC, protocolIPv6CP, 1, 2)
	if err := requireIPv6CPParked(parked.frames()); err == nil {
		t.Fatal("accepted pppd's IPv6CP after ze's Protocol-Reject naming 0x8057")
	}

	silent := &captureBuilder{t: t}
	silent.ppp(relayFromZe, protocolLCP, codeProtocolReject, 3, 0x80, 0x57, 1, 1, 0, 4)
	if _, err := ipv6cpRejected(silent.frames()); err == nil {
		t.Fatal("accepted a capture in which pppd never offered IPv6CP")
	}
	zeRan := &captureBuilder{t: t}
	zeRan.ppp(relayFromLAC, protocolIPv6CP, 1, 1)
	zeRan.ppp(relayFromZe, protocolIPv6CP, 1, 1)
	zeRan.ppp(relayFromZe, protocolLCP, codeProtocolReject, 3, 0x80, 0x57, 1, 1, 0, 4)
	if _, err := ipv6cpRejected(zeRan.frames()); err == nil {
		t.Fatal("accepted ze sending IPv6CP on an IPv4-only pool")
	}
}

// TestCCPProtocolRejectCarriesTheInjectedPacket proves the CCP assertion
// accepts only Ze's Protocol-Reject naming 0x80fd that carries the injected
// Configure-Request, after the injection, and refuses Ze running CCP itself.
func TestCCPProtocolRejectCarriesTheInjectedPacket(t *testing.T) {
	answered := &captureBuilder{t: t}
	answered.ppp(relayFromLAC, protocolCCP, 1, 1, 26, 4, 0x78, 0)
	answered.ppp(relayFromZe, protocolLCP, codeProtocolReject, 4, 0x80, 0xfd, 1, 1, 0, 8, 26, 4, 0x78, 0)
	if err := ccpProtocolRejected(answered.frames()); err == nil {
		t.Fatal("accepted a capture with no injected CCP Configure-Request")
	}
	answered.ppp(relayInjected, protocolCCP, 1, ccpIdentifier)
	if err := ccpProtocolRejected(answered.frames()); err == nil {
		t.Fatal("took the Protocol-Reject of pppd's own CCP for the injected one's")
	}
	answered.ppp(relayFromZe, protocolLCP, codeProtocolReject, 5, 0x80, 0xfd, 1, ccpIdentifier, 0, 4)
	if err := ccpProtocolRejected(answered.frames()); err != nil {
		t.Fatal(err)
	}
	answered.ppp(relayFromZe, protocolCCP, 2, ccpIdentifier)
	if err := ccpProtocolRejected(answered.frames()); err == nil {
		t.Fatal("accepted ze sending CCP")
	}
}

// TestRedistributeEvidenceReaders proves the redistribute scenario's two
// capture assertions: exactly one injected frame, the IPCP Terminate-Request,
// and no CDN or LCP Terminate-Request once IPCP has reopened.
func TestRedistributeEvidenceReaders(t *testing.T) {
	capture := &captureBuilder{t: t}
	capture.ppp(relayFromZe, protocolIPCP, 1, 1)
	capture.ppp(relayInjected, protocolIPCP, codeTerminateRequest, terminateIdentifier)
	capture.ppp(relayFromZe, protocolIPCP, codeTerminateAck, terminateIdentifier)
	frames := capture.frames()
	if err := injectedTerminateAlone(frames); err != nil {
		t.Fatal(err)
	}
	if err := requireSessionNotEnded(capture.text(), frames); err != nil {
		t.Fatal(err)
	}
	capture.ppp(relayInjected, protocolCCP, 1, ccpIdentifier)
	if err := injectedTerminateAlone(capture.frames()); err == nil {
		t.Fatal("accepted a second injected frame")
	}

	ended := &captureBuilder{t: t}
	ended.ppp(relayFromZe, protocolIPCP, 1, 1)
	ended.control(relayFromZe, cdnMessageType, false)
	if err := requireSessionNotEnded(ended.text(), ended.frames()); err == nil {
		t.Fatal("accepted a capture holding ze's CDN")
	}
	closed := &captureBuilder{t: t}
	closed.ppp(relayFromLAC, protocolLCP, codeTerminateRequest, 9)
	if err := requireSessionNotEnded(closed.text(), closed.frames()); err == nil {
		t.Fatal("accepted a capture holding an LCP Terminate-Request")
	}
}

// TestNCPLifetimeScenarioPreparation proves each of the three scenarios is
// discovered from its directory, prepared with the peers its checker drives,
// in start order, and built with the images those peers need. Without its
// plan case prepareScenario refuses the name, and without its image case
// standardImageBuilds leaves the relay or FRR image unbuilt, so both are
// checked here rather than at the first lab run. The redistribute topology is
// the one that runs the relay and FRR together, so their hosts must differ.
func TestNCPLifetimeScenarioPreparation(t *testing.T) {
	root := l2tpCheckout(t)
	for _, tc := range []struct {
		scenario string
		peers    []string
		frr      bool
	}{
		{scenarioIPv6CPRejectedByLNS, []string{peerZe, peerAVPRelay, peerLAC}, false},
		{scenarioNoIPRejectsIPCP, []string{peerZe, peerAVPRelay, peerLAC}, false},
		{scenarioIPCPTerminateRedist, []string{peerZe, peerAVPRelay, peerFRR, peerLAC}, true},
	} {
		t.Run(tc.scenario, func(t *testing.T) {
			plans, needsKernel, err := plansAt(root, tc.scenario, testEnvironment())
			if err != nil {
				t.Fatal(err)
			}
			if !needsKernel {
				t.Fatal("scenario omitted the kernel preflight")
			}
			if len(plans) != 1 {
				t.Fatalf("selected %d plans, want 1", len(plans))
			}
			prepared, err := plans[0].Prepare(t.Context(), interoplab.PrepareContext{
				Source: plans[0].Source, Network: interoplab.Network{Name: "ncp", IPv4: netip.MustParsePrefix("172.29.0.0/24")},
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(prepared.Peers) != len(tc.peers) {
				t.Fatalf("peers = %d, want %d", len(prepared.Peers), len(tc.peers))
			}
			hosts := make(map[uint8]string, len(prepared.Peers))
			for index, name := range tc.peers {
				peer := prepared.Peers[index]
				if peer.Name != name {
					t.Fatalf("peer %d = %s, want %s", index, peer.Name, name)
				}
				if other, taken := hosts[peer.Host]; taken {
					t.Fatalf("%s and %s share host %d", other, peer.Name, peer.Host)
				}
				hosts[peer.Host] = peer.Name
				if peer.Name == peerAVPRelay && peer.Host != 4 {
					t.Fatalf("relay on host %d, but xl2tpd.conf dials 172.29.0.4", peer.Host)
				}
				if peer.Name == peerFRR && peer.Host != relayFRRHost {
					t.Fatalf("FRR on host %d, but frr.conf and ze.conf place it on %d", peer.Host, relayFRRHost)
				}
			}
			built := make(map[string]bool)
			for _, image := range imageBuilds(root, plans, "frr-under-test") {
				built[image.Name] = true
			}
			if !built[imageAVPRelay] {
				t.Fatal("scenario does not build its relay image")
			}
			if built[imageFRR] != tc.frr {
				t.Fatalf("FRR image built = %v, want %v", built[imageFRR], tc.frr)
			}
		})
	}
}
