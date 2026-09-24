// Design: docs/architecture/testing/interop.md -- same-transport LCP renegotiation
// Overview: l2tpppp.go -- the native xl2tpd/pppd proof that calls this assertion
package deployment

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// RFC 2661 sections 3.1 and 5.3 put the recipient's IDs in data packets.
// These values come from the live kernel tunnel, including its UDP endpoints.
type l2tpPPPTransport struct {
	Tunnel      uint16 `json:"tunnel"`
	PeerTunnel  uint16 `json:"peer_tunnel"`
	Session     uint16 `json:"session"`
	PeerSession uint16 `json:"peer_session"`
	Local       string `json:"local"`
	Peer        string `json:"peer"`
	LocalPort   uint16 `json:"local_port"`
	PeerPort    uint16 `json:"peer_port"`
}

func readL2TPPPPTransport(ns string) (l2tpPPPTransport, string, error) {
	var identity l2tpPPPTransport
	state, err := readL2TPSnapshot(ns)
	if err != nil {
		return identity, "", err
	}
	listing := state.tunnel + "\n" + state.session
	if strings.Count(state.tunnel, "Tunnel ") != 1 || strings.Count(state.session, "Session ") != 1 {
		return identity, listing, errors.New("LCP restart requires exactly one kernel tunnel and session in " + ns)
	}
	_, err = fmt.Sscanf(state.tunnel, "Tunnel %d, encap UDP\n  From %s to %s\n  Peer tunnel %d\n  UDP source / dest ports: %d/%d",
		&identity.Tunnel, &identity.Local, &identity.Peer, &identity.PeerTunnel, &identity.LocalPort, &identity.PeerPort)
	if err != nil {
		return identity, listing, fmt.Errorf("read live L2TP UDP identity in %s: %w", ns, err)
	}
	var tunnel, peerTunnel uint16
	_, err = fmt.Sscanf(state.session, "Session %d in tunnel %d\n  Peer session %d, tunnel %d",
		&identity.Session, &tunnel, &identity.PeerSession, &peerTunnel)
	if err != nil || tunnel != identity.Tunnel || peerTunnel != identity.PeerTunnel {
		return identity, listing, fmt.Errorf("kernel session does not identify its live tunnel in %s: %s", ns, state.session)
	}
	for _, addr := range []string{identity.Local, identity.Peer} {
		ip, err := netip.ParseAddr(addr)
		if err != nil || !ip.Is4() || ip.IsUnspecified() {
			return identity, listing, fmt.Errorf("LCP restart requires a concrete IPv4 UDP endpoint, got %q", addr)
		}
	}
	if identity.Tunnel == 0 || identity.PeerTunnel == 0 || identity.Session == 0 || identity.PeerSession == 0 || identity.LocalPort == 0 || identity.PeerPort == 0 {
		return identity, listing, errors.New("live L2TP identity contains an unassigned ID or port")
	}
	// This fixture negotiates unsequenced data. Injecting an arbitrary Ns into
	// a sequenced session would disturb xl2tpd's independent sequence space.
	if strings.Contains(state.session, "sequence numbering:") {
		return identity, listing, errors.New("LCP restart injection requires the fixture's unsequenced data session")
	}
	return identity, listing, nil
}

// Keep the original collector intact for failure diagnostics. Each phase reads
// only hits appended after its boundary, including the final route withdrawal.
func l2tpPPPObservation(seen *collector) map[string]int {
	seen.mu.Lock()
	defer seen.mu.Unlock()
	mark := make(map[string]int, len(seen.hits))
	for needle, hits := range seen.hits {
		mark[needle] = len(hits)
	}
	return mark
}

func l2tpPPPSince(seen *collector, mark map[string]int) *collector {
	fresh := newCollector()
	seen.mu.Lock()
	defer seen.mu.Unlock()
	for needle, hits := range seen.hits {
		fresh.hits[needle] = slices.Clone(hits[mark[needle]:])
	}
	return fresh
}

// The raw IPv4 socket supplies the existing xl2tpd UDP source port without
// binding or replacing xl2tpd's UDP socket. Linux supplies the IPv4 header; a
// zero UDP checksum is permitted for IPv4. No control packet or signal is sent.
const l2tpPPPInjectLCP = `import socket, sys
packet = bytes.fromhex(sys.argv[3])
with socket.socket(socket.AF_INET, socket.SOCK_RAW, socket.IPPROTO_UDP) as sock:
    sock.settimeout(3)
    sock.bind((sys.argv[1], 0))
    sent = sock.sendto(packet, (sys.argv[2], 0))
    if sent != len(packet):
        raise RuntimeError("short raw UDP send: %d/%d" % (sent, len(packet)))
print("LCP restart UDP datagram sent: " + packet.hex())
`

// An empty Configure-Request selects the defaults (RFC 1661 section 5.1).
// Receiving it in Opened requires tld,scr,sca (section 4.1): Ze sends its own
// request, which causes the real pppd to restart and negotiate its own options.
// 0xa5 is separate from pppd's initial request; the injected Ack cannot stand
// in for the independently logged pppd request/Ack exchange checked below.
//
// UDP + L2TPv2 data (RFC 2661 Section 3.1) + PPP LCP (RFC 1661 Section 5.1):
//
//	Offset  0          2          4          6
//	        +----------+----------+----------+----------+
//	        | UDP src  | UDP dst  | len: 24  | sum: 0   |
//	     8  +----------+----------+----------+----------+
//	        | 0x4002   | len: 16  | peer TID | peer SID |
//	    16  +----------+----------+----------+----------+
//	        | ff 03    | c0 21    | 01 | a5  | LCP: 4   |
//	    24  +----------+----------+----------+----------+
func l2tpPPPRestartDatagram(peer l2tpPPPTransport) []byte {
	packet := make([]byte, 24)
	binary.BigEndian.PutUint16(packet[0:2], peer.LocalPort)
	binary.BigEndian.PutUint16(packet[2:4], peer.PeerPort)
	binary.BigEndian.PutUint16(packet[4:6], uint16(len(packet)))
	binary.BigEndian.PutUint16(packet[8:10], 0x4002) // data, length present, version 2
	binary.BigEndian.PutUint16(packet[10:12], 16)
	binary.BigEndian.PutUint16(packet[12:14], peer.PeerTunnel)
	binary.BigEndian.PutUint16(packet[14:16], peer.PeerSession)
	copy(packet[16:], []byte{0xff, 0x03, 0xc0, 0x21, 0x01, 0xa5, 0x00, 0x04})
	return packet
}

// Match each side's last request to an identical Ack later in the fresh log.
// This checks the identifier AND options, including IPCP after a Configure-Nak.
func l2tpPPPExchange(log, protocol, direction, replyDirection string) bool {
	prefix := direction + " [" + protocol + " ConfReq "
	start := strings.LastIndex(log, prefix)
	if start < 0 {
		return false
	}
	request := log[start+len(prefix):]
	before, after, ok := strings.Cut(request, "\n")
	if !ok {
		return false
	}
	return strings.Contains(after, replyDirection+" ["+protocol+" ConfAck "+before+"\n")
}

func l2tpPPPRestartProgress(log, authentication string) string {
	for _, protocol := range []string{"LCP", "IPCP"} {
		if !l2tpPPPExchange(log, protocol, "sent", "rcvd") || !l2tpPPPExchange(log, protocol, "rcvd", "sent") {
			return "fresh bidirectional " + protocol + " request/Ack exchange"
		}
	}
	var auth []string
	if authentication == l2tpPPPScenarioCHAP {
		auth = []string{"<auth chap MD5>", "rcvd [CHAP Challenge ", "sent [CHAP Response ", "rcvd [CHAP Success "}
	}
	remaining := log
	for _, needle := range auth {
		at := strings.Index(remaining, needle)
		if at < 0 {
			return "fresh " + needle
		}
		remaining = remaining[at+len(needle):]
	}
	if len(auth) != 0 {
		// RFC 1661 Section 3.5: "Advancement from the Authentication phase to
		// the Network-Layer Protocol phase MUST NOT occur until authentication
		// has completed." pppd logs Ze-originated IPCP as received, not sent.
		if strings.Contains(log[:len(log)-len(remaining)], " [IPCP ") || !strings.Contains(remaining, "sent [IPCP ConfReq ") {
			return "authentication success before fresh IPCP"
		}
	}
	for _, needle := range []string{"local  IP address " + L2TPPPPPeerAddr, "remote IP address " + L2TPPPPLocalAddr} {
		if !strings.Contains(remaining, needle) {
			return "fresh pppd " + needle
		}
	}
	return ""
}

func (l *L2TPPPP) assertLCPRestart(report L2TPPPPReport, seen *collector, ze, dialer *running, work string, zeBase, lacBase pppBaseline) (L2TPPPPReport, bool) {
	fail := func(err error) (L2TPPPPReport, bool) {
		return l.fail(report, seen, "same-transport LCP restart: "+err.Error()), false
	}
	beforeZe, zeListing, err := readL2TPPPPTransport(l.ZeNamespace)
	if err != nil {
		return fail(err)
	}
	beforeLAC, lacListing, err := readL2TPPPPTransport(l.LACNamespace)
	if err != nil {
		return fail(err)
	}
	if beforeZe.Tunnel != beforeLAC.PeerTunnel || beforeZe.PeerTunnel != beforeLAC.Tunnel || beforeZe.Session != beforeLAC.PeerSession || beforeZe.PeerSession != beforeLAC.Session || beforeZe.Local != beforeLAC.Peer || beforeZe.Peer != beforeLAC.Local || beforeZe.LocalPort != beforeLAC.PeerPort || beforeZe.PeerPort != beforeLAC.LocalPort {
		return fail(errors.New("kernel LNS and LAC transport identities are not reciprocal"))
	}
	log, err := os.Open(filepath.Join(work, "pppd.log")) //nolint:gosec // this run's peer log
	if err != nil {
		return fail(err)
	}
	defer log.Close() //nolint:errcheck // read-only evidence
	initial, err := io.ReadAll(log)
	if err != nil {
		return fail(err)
	}
	if len(initial) == 0 || initial[len(initial)-1] != '\n' {
		return fail(errors.New("pppd log has no complete initial observation boundary"))
	}
	authentication := "none"
	if l.Scenario == l2tpPPPScenarioCHAP {
		authentication = l2tpPPPScenarioCHAP
	} else if strings.Contains(string(initial), "<auth ") {
		return fail(errors.New("unexpected authentication in the no-auth native peer input"))
	}
	if missing := l2tpPPPRestartProgress(string(initial), authentication); missing != "" {
		return fail(errors.New("initial peer exchange is incomplete: " + missing))
	}
	mark := l2tpPPPObservation(seen)
	packet := l2tpPPPRestartDatagram(beforeLAC)
	boundary := map[string]any{
		"at": time.Now().UTC(), "pppd_log_offset": len(initial), "ze_observation_counts": mark,
		"ze_transport": beforeZe, "lac_transport": beforeLAC, "ze_kernel": zeListing, "lac_kernel": lacListing,
		"authentication": authentication, "udp_datagram_hex": hex.EncodeToString(packet), "l2tp_datagram_hex": hex.EncodeToString(packet[8:]),
	}
	body, err := json.MarshalIndent(boundary, "", "  ")
	if err != nil {
		return fail(err)
	}
	if err := os.WriteFile(filepath.Join(work, "lcp-restart-boundary.json"), append(body, '\n'), 0o600); err != nil {
		return fail(err)
	}
	writeProgress(l.Progress, "LCP restart boundary: "+string(body))
	out, ok := nsText(l.LACNamespace, "python3", "-c", l2tpPPPInjectLCP, beforeLAC.Local, beforeLAC.Peer, hex.EncodeToString(packet))
	writeProgress(l.Progress, out)
	if !ok {
		return fail(errors.New("raw LCP Configure-Request injection failed: " + out))
	}

	deadline := time.NewTimer(l.NCPWait)
	defer deadline.Stop()
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	var fresh *collector
	var freshLog string
	for {
		fresh = l2tpPPPSince(seen, mark)
		if fatal := fresh.firstSeen(append(slices.Clone(pppFatalLines), pppTeardownLine, pppSessionLine)); fatal != "" {
			return fail(errors.New("transport ended or was replaced: " + fatal))
		}
		if ze.exited() || dialer.exited() {
			return fail(errors.New("ze or xl2tpd exited during renegotiation"))
		}
		info, err := log.Stat()
		if err != nil {
			return fail(err)
		}
		if info.Size() < int64(len(initial)) {
			return fail(errors.New("pppd log was truncated after the observation boundary"))
		}
		body, err := io.ReadAll(io.NewSectionReader(log, int64(len(initial)), info.Size()-int64(len(initial))))
		if err != nil {
			return fail(err)
		}
		freshLog = string(body)
		for _, fatal := range []string{"Using interface ", "Connect: ", "Connection terminated", "Modem hangup", "LCP terminated by peer", "CHAP authentication failed"} {
			if strings.Contains(freshLog, fatal) {
				return fail(errors.New("pppd ended or replaced its link: " + fatal))
			}
		}
		missing := l2tpPPPRestartProgress(freshLog, authentication)
		wanted := []string{pppWithdrawLine, pppIPLine, pppRouteLine, pppUpLine}
		if l.Scenario == l2tpPPPScenarioCHAP {
			wanted = append(wanted, pppCHAPAcceptedLine)
		}
		if missing == "" && fresh.sawAll(wanted) {
			break
		}
		if missing == "" {
			missing = "fresh Ze address assignment, route withdrawal/reinjection and PPP session-up"
		}
		select {
		case <-deadline.C:
			return fail(errors.New("timed out awaiting " + missing))
		case <-poll.C:
		}
	}
	writeProgress(l.Progress, "pppd after LCP restart boundary:\n"+freshLog)
	report, ok = l.assertKernelState(report, fresh, zeBase, lacBase)
	if !ok {
		return fail(errors.New(report.Reason))
	}
	// With both -c and -w, iputils ping fails unless all three replies arrive.
	// Bind each PPP interface so the namespace underlay cannot satisfy the probe.
	for _, probe := range []struct{ ns, iface, address string }{
		{l.LACNamespace, report.LACInterface, L2TPPPPLocalAddr},
		{l.ZeNamespace, report.ZeInterface, L2TPPPPPeerAddr},
	} {
		out, ok := nsText(probe.ns, "ping", "-n", "-I", probe.iface, "-c", "3", "-W", "3", "-w", "10", probe.address)
		writeProgress(l.Progress, probe.ns+" resumed PPP traffic:\n"+out)
		if !ok {
			return fail(errors.New("three-reply PPP delivery failed in " + probe.ns + ": " + out))
		}
	}
	for _, ns := range []string{l.ZeNamespace, l.LACNamespace} {
		after, listing, err := readL2TPPPPTransport(ns)
		writeProgress(l.Progress, ns+" after LCP restart:\n"+listing)
		if err != nil {
			return fail(err)
		}
		want := beforeZe
		if ns == l.LACNamespace {
			want = beforeLAC
		}
		if after != want {
			return fail(fmt.Errorf("%s transport changed from %+v to %+v", ns, want, after))
		}
	}
	if fatal := l2tpPPPSince(seen, mark).firstSeen(append(slices.Clone(pppFatalLines), pppTeardownLine, pppSessionLine)); fatal != "" {
		return fail(errors.New("transport ended or was replaced during traffic: " + fatal))
	}
	return report, true
}

func awaitL2TPPPPWithdrawal(seen *collector, mark map[string]int, ze *running, timeout time.Duration) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	poll := time.NewTicker(pollInterval)
	defer poll.Stop()
	for {
		if l2tpPPPSince(seen, mark).saw(pppWithdrawLine) {
			return nil
		}
		if ze.exited() {
			return errors.New("ze exited before the final subscriber route withdrawal")
		}
		select {
		case <-deadline.C:
			return errors.New("fresh subscriber route withdrawal was not observed during peer-first teardown")
		case <-poll.C:
		}
	}
}
