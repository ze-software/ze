// Design: docs/architecture/testing/interop.md -- DNS-named remotes against strongSwan.
// Related: dns.go -- the lab name server these checkers start.
// Related: register_dns.go -- registers both checkers.
package ipsec

import (
	"context"
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/le/interoplab"
)

const (
	// fqdnRemoteName is the name both scenarios' ze.conf dial and dns.conf serves.
	fqdnRemoteName = "vpn.example.net"

	// zeCycleFailure is the WARN line runInitiator (internal/component/ike/engine/fsm.go)
	// writes when a connect cycle cannot start; resolveCyclePair's error, which the line
	// carries, says the name "does not resolve".
	zeCycleFailure      = "ike: connect cycle cannot start"
	zeCycleFailureCause = "does not resolve"

	// charon's message log (strongSwan 6.1.0 src/libcharon/encoding/message.c, the
	// "parsed %s" and "generating %s" lines over get_string) lists each notify by its
	// short name; NAT_DETECTION_SOURCE_IP is NATD_S_IP and NAT_DETECTION_DESTINATION_IP
	// is NATD_D_IP.
	swanParsedInitRequest     = "parsed IKE_SA_INIT request 0 ["
	swanGeneratedInitResponse = "generating IKE_SA_INIT response 0 ["
	swanNATDSource            = "N(NATD_S_IP)"
	swanNATDDestination       = "N(NATD_D_IP)"

	// strongSwan 6.1.0 src/libcharon/sa/ike_sa.c set_condition logs these when the NAT
	// detection hashes it received do not match the addresses it observed
	// (ike_natd.c process_payloads sets COND_NAT_HERE on a destination mismatch and
	// COND_NAT_THERE on a source mismatch). Every one of them contains swanBehindNAT.
	swanBehindNAT       = "behind NAT"
	swanLocalBehindNAT  = "local host is behind NAT"
	swanRemoteBehindNAT = "remote host is behind NAT"
)

// checkFQDNRemoteDynamicLocal proves a Ze initiator that names its remote by DNS and
// configures no local address at all (no `interface`, no `local-address`: the
// route-chosen source) against strongSwan, booted while DNS is down.
//
// Method, in order:
//
//  1. DNS down. Ze's connect cycle fails naming the peer's name and the resolver error,
//     and fails again (the retry), while charon parses no IKE_SA_INIT (AC-13).
//  2. The lab name server starts. The next cycle establishes (AC-14).
//  3. The IKE_SA_INIT charon parsed carries both NAT_DETECTION payloads (AC-10). charon
//     answers with its own only when it received both (ike_natd.c build_r: "if
//     (this->src_seen && this->dst_seen)"), so its response carrying them is the peer's
//     confirmation that it read them. charon logs no NAT condition, so Ze's source hash
//     matched 172.28.0.2:500 and its destination hash the address the name resolved to
//     (AC-6); Ze's own verdict reports no NAT.
//  4. The request reached charon from 172.28.0.2 port 500, the route's source, and the
//     established IKE SA sits on Ze's port 4500: no float at IKE_SA_INIT, and MOBIKE,
//     which strongSwan offers by default, moved IKE_AUTH to 4500 (AC-12, RFC 4555).
//  5. The Child SA runs between 172.28.0.2 and 172.28.0.3, and traffic crosses it.
//
// Red before spec-home-cpe-4: Ze built the NAT_DETECTION payloads only when the
// configured remote-address parsed as an IP, so step 3 fails on the parsed request.
func checkFQDNRemoteDynamicLocal(ctx context.Context, lab *scenarioLab) error {
	if err := lab.requireUnresolvedRetries(ctx); err != nil {
		return err
	}
	if err := lab.startDNS(ctx); err != nil {
		return err
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := requireNATDetectionExchanged(logs); err != nil {
		return err
	}
	if strings.Contains(logs, swanBehindNAT) {
		return interoplab.WithSearchedOutput(fmt.Errorf("charon recorded a NAT on a path that has none, "+
			"so Ze's NAT_DETECTION hashes were not over 172.28.0.2:500 and the address %s resolved to", fqdnRemoteName), logs)
	}
	if err := lab.requireNoNATVerdict(ctx); err != nil {
		return err
	}
	if err := requireContains(logs, swanReceivedLine(zeIP, "500", swanIP, "500"),
		"charon received no datagram from Ze's route-chosen source 172.28.0.2 port 500"); err != nil {
		return err
	}
	sas, err := lab.listSAs(ctx)
	if err != nil {
		return err
	}
	if err := requireContains(sas, swanEndpoint(zeIP, "4500"),
		"strongSwan's IKE SA is not on Ze's 172.28.0.2 port 4500, so MOBIKE did not float IKE_AUTH: "+sas); err != nil {
		return err
	}
	if _, err := lab.inboundXFRMState(ctx, zePeer, swanIP, zeIP); err != nil {
		return err
	}
	if _, err := lab.inboundXFRMState(ctx, swanPeer, zeIP, swanIP); err != nil {
		return err
	}
	return lab.verifyTunnelTraffic(ctx, "traffic did not flow through the tunnel Ze dialed by name")
}

// checkFQDNRemoteRealNAT proves Ze's NAT detection against a remote named by DNS across
// the lab's real NAT box (AC-11): the name resolves to strongSwan's public address, and
// the box rewrites both addresses of every datagram.
//
// Ze's NAT_DETECTION_DESTINATION_IP is hashed over the public address the name resolved
// to, and charon sees itself at its real one, so charon records itself behind a NAT;
// Ze's source hash names 172.28.0.2 and charon sees Ze at 172.28.0.6, so charon records
// Ze behind one too. Ze checks charon's payloads against the resolved address and records
// the NAT, the IKE SA floats to port 4500, and the Child SA encapsulates ESP in UDP.
//
// Red before spec-home-cpe-4: with a name in remote-address Ze sent no NAT_DETECTION
// payload and checked none, so charon logs no NAT and Ze's verdict reports none.
func checkFQDNRemoteRealNAT(ctx context.Context, lab *scenarioLab) error {
	if err := lab.startDNS(ctx); err != nil {
		return err
	}
	if err := establish(ctx, lab); err != nil {
		return err
	}
	logs, err := lab.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if err := requireNATDetectionExchanged(logs); err != nil {
		return err
	}
	for _, line := range []string{swanLocalBehindNAT, swanRemoteBehindNAT} {
		if err := requireContains(logs, line, "charon did not log "+line+
			", so Ze's NAT_DETECTION hashes did not reveal the translation"); err != nil {
			return err
		}
	}
	if err := lab.assertNATVerdict(ctx); err != nil {
		return err
	}
	sas, err := lab.listSAs(ctx)
	if err != nil {
		return err
	}
	if err := requireContains(sas, swanEndpoint(zePublicIP, "4500"),
		"strongSwan's IKE SA is not on Ze's public 172.28.0.6 port 4500, so the SA did not float: "+sas); err != nil {
		return err
	}
	if err := lab.assertZeSelectors(ctx, natTunnelZeInner+"/32", natTunnelSwanInner+"/32"); err != nil {
		return err
	}
	state, err := lab.inboundXFRMState(ctx, zePeer, swanPublicIP, zeIP)
	if err != nil {
		return err
	}
	if err := requireContains(state, "encap type espinudp",
		"Ze's inbound Child SA does not encapsulate ESP in UDP across the NAT: "+state); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, zePeer, natTunnelZeInner, natTunnelSwanInner); err != nil {
		return err
	}
	if err := lab.natInnerAddress(ctx, swanPeer, natTunnelSwanInner, natTunnelZeInner); err != nil {
		return err
	}
	return lab.verifyESPDirectionsToward(ctx,
		"traffic did not cross the Child SA Ze dialed by name through the NAT",
		pingProbe{peer: zePeer, target: natTunnelSwanInner, source: natTunnelZeInner}, natESPDirections)
}

// requireUnresolvedRetries is the DNS-down phase: Ze logs a failed cycle naming the name
// and the resolver error, logs a second one after it (the backoff retried), and charon
// has parsed no IKE_SA_INIT, so nothing was sent while the name could not resolve.
func (l *scenarioLab) requireUnresolvedRetries(ctx context.Context) error {
	if err := l.waitLog(ctx, zePeer, zeCycleFailure, l.timeout); err != nil {
		return err
	}
	mark, err := l.logMark(ctx, zePeer, zeCycleFailure)
	if err != nil {
		return err
	}
	if err := l.waitLogAfter(ctx, zePeer, zeCycleFailure, mark, l.timeout); err != nil {
		return fmt.Errorf("ze did not retry the connect cycle while %s could not resolve: %w", fqdnRemoteName, err)
	}
	zeLogs, err := l.logs(ctx, zePeer)
	if err != nil {
		return err
	}
	if !lineCarriesAll(zeLogs, zeCycleFailure, fqdnRemoteName, zeCycleFailureCause) {
		return interoplab.WithSearchedOutput(fmt.Errorf("ze's failed-cycle line does not name %s and say it %s",
			fqdnRemoteName, zeCycleFailureCause), zeLogs)
	}
	swanLogs, err := l.logs(ctx, swanPeer)
	if err != nil {
		return err
	}
	if strings.Contains(swanLogs, swanParsedInitRequest) {
		return interoplab.WithSearchedOutput(fmt.Errorf("charon parsed an IKE_SA_INIT while %s could not resolve",
			fqdnRemoteName), swanLogs)
	}
	return nil
}

// requireNoNATVerdict refuses unless every IKE SA Ze reports carries nat-detected false.
// An absent field is not false: `show vpn ipsec sa` declares it on every record.
func (l *scenarioLab) requireNoNATVerdict(ctx context.Context) error {
	records, answer, err := l.zeIKESAs(ctx)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("show vpn ipsec sa reports no IKE SA; answer: %s", answer)
	}
	for _, record := range records {
		detected, ok := record["nat-detected"].(bool)
		if !ok {
			return fmt.Errorf("show vpn ipsec sa carries no boolean nat-detected; answer: %s", answer)
		}
		if detected {
			return fmt.Errorf("ze recorded a NAT on a path that has none; answer: %s", answer)
		}
	}
	return nil
}

// requireNATDetectionExchanged refuses unless the IKE_SA_INIT request charon parsed and
// the response it generated each carry both NAT_DETECTION payloads.
func requireNATDetectionExchanged(logs string) error {
	for _, exchange := range []string{swanParsedInitRequest, swanGeneratedInitResponse} {
		if !lineCarriesAll(logs, exchange, swanNATDSource, swanNATDDestination) {
			return interoplab.WithSearchedOutput(fmt.Errorf("charon's %q line lacks %s or %s: "+
				"Ze sent no NAT detection to the address it resolved", exchange, swanNATDSource, swanNATDDestination), logs)
		}
	}
	return nil
}

// lineCarriesAll reports whether one line of logs carries every needle.
func lineCarriesAll(logs string, needles ...string) bool {
	for line := range strings.SplitSeq(logs, "\n") {
		if carriesAll(line, needles) {
			return true
		}
	}
	return false
}

func carriesAll(line string, needles []string) bool {
	for _, needle := range needles {
		if !strings.Contains(line, needle) {
			return false
		}
	}
	return true
}

// swanReceivedLine is charon's "received packet: from <a>[<p>] to <b>[<q>]" line
// (src/libcharon/network/receiver.c).
func swanReceivedLine(source, sourcePort, target, targetPort string) string {
	return "received packet: from " + source + "[" + sourcePort + "] to " + target + "[" + targetPort + "]"
}

// swanEndpoint is the "@ <host>[<port>]" half of swanctl --list-sas's local and remote lines
// (src/swanctl/commands/list_sas.c: "  remote '%s' @ %s[%s]").
func swanEndpoint(host, port string) string {
	return "@ " + host + "[" + port + "]"
}
