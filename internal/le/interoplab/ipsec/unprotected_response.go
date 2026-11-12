// Design: docs/architecture/testing/interop.md -- IPsec injected datagrams: forged IKE_AUTH responses at Ze as initiator, against strongSwan.
// Related: unprotected_eap.go -- unprotForgedAuthHex and unprotDropCount, which this checker reuses.
// Related: suspicious.go -- injectOutOfSA, the raw datagram route.
// Related: checkers.go -- the scenario table that names checkInitiatorSurvivesUnprotectedAuthResponse.
// RFC: rfc/short/rfc7296.md -- RFC7296-2.4-3.
package ipsec

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// Ze's initiator retransmit schedule, copied from internal/component/ike/engine/fsm.go
// (retransmitBase, retransmitMax, maxRetransmissions and retransmitBackoff). The engine
// keeps them unexported and le never imports product internals for a constant, so
// TestZeRetransmitScheduleMatchesEngine reads fsm.go and refuses a copy that drifted.
const (
	zeRetransmitBase     = 500 * time.Millisecond
	zeRetransmitMax      = 60 * time.Second
	zeMaxRetransmissions = 7
)

// The nftables table that holds strongSwan's IKE_AUTH responses back from Ze, on the
// output hook of strongSwan's own container. iptables cannot match the exchange type:
// its one match for an arbitrary octet is xt_u32, which Ze's kernel does not build (see
// dfCounterTable), while an nftables raw payload expression is part of nf_tables.
const (
	authHoldCommand = "nft"
	authHoldFamily  = "inet"
	authHoldTable   = "ze_auth_hold"
	authHoldHook    = "output"
	// authHoldExchangeType is the Exchange Type octet of a datagram from UDP 4500, as
	// an nftables raw payload offset in bits from the UDP header: 8 octets of UDP
	// header, 4 of non-ESP marker (RFC 3948 Section 2.2), then octet 18 of the RFC 7296
	// Section 3.1 header. Octet 30 is bit 240.
	authHoldExchangeType = "@th,240,8"
	// ikeExchangeIKEAuth is Exchange Type IKE_AUTH (RFC 7296 Section 3.1).
	ikeExchangeIKEAuth = "35"
	// wordDelete is the verb ip-xfrm(8) and nft(8) both take to remove an object, and
	// xfrmWordDir the ip-xfrm(8) keyword naming a policy's direction.
	wordDelete  = "delete"
	xfrmWordDir = "dir"
	// wordSport, wordList and wordCounter are the nft(8) and ip-xfrm(8) words for a
	// source port, a listing and a counter.
	wordSport   = "sport"
	wordList    = "list"
	wordCounter = "counter"
)

// authRespForgedMessageID is the Message ID of the forged responses: Ze's IKE_AUTH
// request is its second request, after IKE_SA_INIT at 0.
const authRespForgedMessageID = 1

// authRespSwanRetransmit is the line charon logs when a request arrives again at the
// Message ID it last answered and it resends the response it kept: process_message,
// the ALREADY_DONE arm of is_retransmit (src/libcharon/sa/ikev2/task_manager_v2.c,
// strongSwan 6.1.0).
const authRespSwanRetransmit = "received retransmit of request with ID 1, retransmitting response"

// authRespSwanIKE matches one IKE SA line of `swanctl --list-sas`: "ze: #3,
// ESTABLISHED, IKEv2, <spi-i>_i <spi-r>_r*". strongSwan is the responder here, so it
// lists its SA as ESTABLISHED as soon as it has sent its IKE_AUTH response, held or not.
var authRespSwanIKE = regexp.MustCompile(`(?m)^ze: #[0-9]+, [A-Z_]+, IKEv2, ([0-9a-fA-F]{16})_i\*? ([0-9a-fA-F]{16})_r\*?`)

// ikeSPIPair is the initiator and responder SPI of one IKE SA, as hex.
type ikeSPIPair struct {
	initiator string
	responder string
}

// checkInitiatorSurvivesUnprotectedAuthResponse proves RFC 7296 Section 2.4 with Ze as
// the initiator, against strongSwan as the responder: "Since IKE is designed to operate
// in spite of DoS attacks from the network, an endpoint MUST NOT conclude that the other
// endpoint has failed based on any routing information (e.g., ICMP messages) or IKE
// messages that arrive without cryptographic protection (e.g., Notify messages
// complaining about unknown SPIs)."
//
// Method: the PSK tunnel establishes once, unmolested, as the control. strongSwan's
// IKE_AUTH responses to Ze are then held in nftables and its IKE SA terminated, so Ze
// reconnects: IKE_SA_INIT passes, strongSwan authenticates Ze and answers into the hold,
// and Ze retransmits its request, which charon logs (authRespSwanRetransmit). Forged
// IKE_AUTH responses then go to Ze from strongSwan's address and port, under the live
// SPI pair at Message ID 1, each carrying an Encrypted payload that fails its integrity
// check. Ze's drop counter must rise by exactly the number sent. Once the hold is
// lifted, Ze's next retransmission draws charon's kept response, and Ze must establish
// the SA whose initiator SPI the forgeries named and pass traffic over it.
//
// The hold excludes the forgeries by their UDP length, because rawprobe's raw socket
// passes the same output hook as charon's socket. A genuine IKE_AUTH response carries
// IDr, AUTH, SA and both TS payloads, far more than the forgery's 64 octets of filler.
// A bypass policy keeps the forgeries out of the Child SA charon already installed
// (authInjectBypassCommand).
//
// Red: a break that ends the handshake on a forgery (openProtected answering the
// decrypt failure as an error, so handleAuthResponse marks the SA dead) leaves the
// count at one, because the later forgeries name an SA Ze no longer holds, so
// assertion 5 fails; were it skipped, the SA Ze establishes after the hold lifts
// would carry a new initiator SPI, and assertion 7 would fail.
//
// Each assertion is numbered, so a red run names which one failed and a
// discrimination record can cite it.
//
// RFC requirement: RFC7296-2.4-3 negative -- with Ze initiating to strongSwan
// 6.1.0, twenty forged IKE_AUTH responses sent to Ze under the live SPI pair at
// Message ID 1, each with an Encrypted payload that fails its integrity check,
// raise Ze's handshake-unauthenticated count by exactly twenty, and the IKE SA Ze
// then establishes carries the initiator SPI the forgeries named and passes traffic.
func checkInitiatorSurvivesUnprotectedAuthResponse(ctx context.Context, lab *scenarioLab) error {
	const (
		// The grouped form is what `interopScenarioOf` (`internal/le/rfc`) reads to
		// learn which scenario a tagged checker drives.
		name = "ipsec-auth-corrupt-response-strongswan"
	)
	fail := func(assertion int, cause error) error {
		return fmt.Errorf("scenario %s assertion %d: %w", name, assertion, cause)
	}

	// Assertion 1. The control exchange establishes, and charon lists one IKE SA.
	if err := establish(ctx, lab); err != nil {
		return fail(1, err)
	}
	listing, err := lab.listSAs(ctx)
	if err != nil {
		return fail(1, err)
	}
	control := swanIKESPIPairs(listing)
	if len(control) != 1 {
		return fail(1, interoplab.WithSearchedOutput(fmt.Errorf("want one IKE SA after the control exchange, swanctl lists %d", len(control)), listing))
	}
	// Assertion 2. The baselines read, the hold and the bypass install, and charon
	// terminates the control SA.
	dropsBefore, err := lab.unprotDropCount(ctx)
	if err != nil {
		return fail(2, err)
	}
	retransmitsBefore, err := lab.logMark(ctx, swanPeer, authRespSwanRetransmit)
	if err != nil {
		return fail(2, err)
	}
	if err := lab.holdAuthResponses(ctx); err != nil {
		return fail(2, err)
	}
	defer lab.execQuiet(context.Background(), swanPeer, authLiftCommand()...)
	if output, err := lab.exec(ctx, swanPeer, authInjectBypassCommand(wordAdd, "priority", "1")...); err != nil {
		return fail(2, fmt.Errorf("add strongSwan's bypass policy for the forgeries: %w: %s", err, output))
	}
	defer lab.execQuiet(context.Background(), swanPeer, authInjectBypassCommand(wordDelete)...)
	// Ze sends its new IKE_AUTH after this instant, so the retransmission that follows
	// the lift exists only while the lift lands before this instant plus the offset of
	// Ze's last retransmission.
	liftBy := time.Now().Add(zeLastAuthRetransmitOffset())
	if _, err := lab.exec(ctx, swanPeer, cmdSwanctl, "--terminate", "--ike", swanConnection); err != nil {
		return fail(2, err)
	}
	// Assertion 3. charon receives Ze's retransmitted IKE_AUTH: its answer was held.
	if err := lab.waitAuthBudget(ctx, liftBy, "charon receiving Ze's retransmitted IKE_AUTH, answered into the hold",
		func(probe context.Context) (int, error) { return lab.logMark(probe, swanPeer, authRespSwanRetransmit) },
		func(current int) bool { return current > retransmitsBefore }); err != nil {
		return fail(3, err)
	}
	// Assertion 4. charon lists the held handshake's SPI pair, and the forgeries go out.
	listing, err = lab.listSAs(ctx)
	if err != nil {
		return fail(4, err)
	}
	held, err := heldSPIPair(listing, control[0])
	if err != nil {
		return fail(4, err)
	}
	forged, err := unprotForgedAuthHex(held.initiator, held.responder,
		unprotFlagsResponderResponse, authRespForgedMessageID, unprotInnerIDr)
	if err != nil {
		return fail(4, err)
	}
	// The evidence a red carries, so one failing run says where the forgeries went.
	evidenceBefore := lab.authInjectEvidence(ctx, "before the injection")
	var sent textbuf.Buffer
	sent.Str("=== injection: rawprobe -p udp -s ").Str(outOfSAPort).Str(" -d ").Str(outOfSAPort).
		Str(" -x <forged> ").Str(zeIP).Str(", run by docker exec in the ").Str(swanPeer).
		Str(" container (its network namespace), source ").Str(swanIP).Str("\n").
		Str("forged under held SPIs ").Str(held.initiator).Str("_i ").Str(held.responder).
		Str("_r, UDP length ").Int(unprotForgedAuthUDPOctets).
		Str(", first 32 octets ").Str(authFirstOctetsHex(forged, 32)).Str("\n")
	for attempt := range unprotForgeries {
		output, err := lab.injectOutOfSAReport(ctx, forged)
		if err != nil {
			return fail(4, err)
		}
		if attempt == 0 {
			sent.Str("rawprobe answered: ").Str(strings.TrimSpace(output)).Str("\n")
		}
	}
	sent.Str("rawprobe sent ").Int(unprotForgeries).Str(" datagrams without error\n")
	// Assertion 5. Ze counts each forgery as an unauthenticated drop, exactly once.
	lastDrops := dropsBefore
	err = lab.waitAuthBudget(ctx, liftBy, "Ze counting each forged IKE_AUTH response as an unauthenticated drop",
		lab.unprotDropCount, func(current int) bool {
			lastDrops = current
			return current-dropsBefore == unprotForgeries
		})
	if err != nil {
		evidenceAfter := lab.authInjectEvidence(context.Background(), "after the counter wait")
		return fail(5, fmt.Errorf("%s rose by %d, want %d: %w\n%s%s%s", unprotDropMetric, lastDrops-dropsBefore, unprotForgeries, err,
			sent.String(), evidenceBefore, evidenceAfter))
	}

	// Assertion 6. The bypass is removed and the hold lifts inside Ze's retransmit window.
	if output, err := lab.exec(ctx, swanPeer, authInjectBypassCommand(wordDelete)...); err != nil {
		return fail(6, fmt.Errorf("remove strongSwan's bypass policy for the forgeries: %w: %s", err, output))
	}
	if !time.Now().Before(liftBy) {
		return fail(6, fmt.Errorf("the forgeries took Ze past its last IKE_AUTH retransmission, %s after the request, before the hold could lift",
			zeLastAuthRetransmitOffset()))
	}
	if output, err := lab.exec(ctx, swanPeer, authLiftCommand()...); err != nil {
		return fail(6, fmt.Errorf("lift the IKE_AUTH hold on strongSwan: %w: %s", err, output))
	}
	// Assertion 7. Ze establishes the IKE SA whose initiator SPI the forgeries named.
	if err := lab.waitZeInitiatorSPI(ctx, held.initiator); err != nil {
		return fail(7, err)
	}
	// Assertion 8. The Child SA installs.
	if err := lab.waitChild(ctx, ""); err != nil {
		return fail(8, err)
	}
	// Assertion 9. Traffic passes over the tunnel.
	if err := lab.verifyTunnelTraffic(ctx, "no traffic after an IKE_AUTH exchange that survived forged responses"); err != nil {
		return fail(9, err)
	}
	return nil
}

// authEvidenceProbe is one read-only command whose answer a red carries.
type authEvidenceProbe struct {
	peer string
	argv []string
}

// authEvidenceProbes are the reads that place a lost forgery: strongSwan's XFRM
// policies and states and its nftables ruleset with counters (the forgeries' counter
// rule shows whether they crossed its output hook); Ze's XFRM and UDP counters, its
// sockets, its XFRM tables, every ze_ipsec series and its IKE SAs. Neither image
// carries tcpdump, so the counters stand in for a capture.
var authEvidenceProbes = []authEvidenceProbe{
	{swanPeer, []string{"ip", xfrmCommand, ipWordPolicy}},
	{swanPeer, []string{"ip", xfrmCommand, ipWordState}},
	{swanPeer, []string{authHoldCommand, "-a", wordList, "ruleset"}},
	{zePeer, []string{"cat", "/proc/net/xfrm_stat", "/proc/net/snmp"}},
	{zePeer, []string{"ss", "-uanp"}},
	{zePeer, []string{"ip", xfrmCommand, ipWordPolicy}},
	{zePeer, []string{"ip", xfrmCommand, ipWordState}},
	{zePeer, []string{"python3", "-c",
		"import urllib.request\n" +
			"for line in urllib.request.urlopen('http://127.0.0.1:9273/metrics', timeout=3).read().decode().splitlines():\n" +
			"    if line.startswith('ze_ipsec'): print(line)"}},
}

// authInjectEvidence reads every authEvidenceProbe and Ze's IKE SAs, and answers them
// as one labeled block. A probe that fails is recorded with its error, never skipped,
// because a missing answer is part of the evidence.
func (l *scenarioLab) authInjectEvidence(ctx context.Context, label string) string {
	var evidence textbuf.Buffer
	evidence.Str("=== evidence ").Str(label).Str(" ===\n")
	for _, probe := range authEvidenceProbes {
		answer, err := l.exec(ctx, probe.peer, probe.argv...)
		writeAuthEvidence(&evidence, probe.peer, strings.Join(probe.argv, " "), answer, err)
	}
	const zeSAs = "show vpn ipsec sa | json"
	answer, err := l.zeCLI(ctx, zeSAs)
	writeAuthEvidence(&evidence, zePeer, zeSAs, answer, err)
	return evidence.String()
}

// writeAuthEvidence appends one probe's answer under its peer and command, and the
// error it met when it met one.
func writeAuthEvidence(evidence *textbuf.Buffer, peer, command, answer string, err error) {
	evidence.Str("--- ").Str(peer).Str(": ").Str(command).Str("\n")
	if err != nil {
		evidence.Str("(failed: ").Str(err.Error()).Str(")\n")
	}
	evidence.Str(strings.TrimSpace(answer)).Str("\n")
}

// authFirstOctetsHex answers the first octets of a 0x-prefixed hex payload, as hex.
func authFirstOctetsHex(payloadHex string, octets int) string {
	digits := strings.TrimPrefix(payloadHex, "0x")
	return digits[:min(len(digits), 2*octets)]
}

// holdAuthResponses installs the nftables table that drops strongSwan's IKE_AUTH
// datagrams to Ze from UDP 4500, except one of the forgeries' UDP length. A counting
// rule ahead of the drop, with no verdict, counts the forgeries as they cross the
// output hook, so a red can tell a forgery that left from one that never did.
func (l *scenarioLab) holdAuthResponses(ctx context.Context) error {
	steps := [][]string{
		{authHoldCommand, wordAdd, ipWordTable, authHoldFamily, authHoldTable},
		{authHoldCommand, wordAdd, "chain", authHoldFamily, authHoldTable, authHoldHook,
			"{", "type", "filter", "hook", authHoldHook, "priority", "0", ";", ipWordPolicy, "accept", ";", "}"},
		{authHoldCommand, wordAdd, wordRule, authHoldFamily, authHoldTable, authHoldHook,
			"ip", "daddr", zeIP, protocolUDP, wordSport, outOfSAPort,
			protocolUDP, "length", strconv.Itoa(unprotForgedAuthUDPOctets),
			wordCounter, "comment", `"forgeries"`},
		{authHoldCommand, wordAdd, wordRule, authHoldFamily, authHoldTable, authHoldHook,
			"ip", "daddr", zeIP, protocolUDP, wordSport, outOfSAPort,
			authHoldExchangeType, "==", ikeExchangeIKEAuth,
			protocolUDP, "length", "!=", strconv.Itoa(unprotForgedAuthUDPOctets),
			wordCounter, "drop"},
	}
	for _, step := range steps {
		if output, err := l.exec(ctx, swanPeer, step...); err != nil {
			return fmt.Errorf("hold strongSwan's IKE_AUTH responses (%s): %w: %s", strings.Join(step, " "), err, output)
		}
	}
	return nil
}

// authInjectBypassCommand adds or deletes, on strongSwan, an outbound XFRM policy with
// no template for UDP from strongSwan to Ze, so the forgeries leave in the clear.
// Without it they do not reach Ze's socket: charon installs its Child SA (TS 0.0.0.0/0
// both ways) when it answers Ze's IKE_AUTH, rawprobe's raw socket takes the XFRM output
// path, and the forgeries leave as ESP under an SA Ze never installed, because Ze never
// received that answer, so Ze's kernel drops them (XfrmInNoStates) before any IKE code
// sees them. charon's own IKE socket carries a per-socket bypass and needs none.
//
// The selector names no port, because the flow XFRM looks up for rawprobe carries
// none. rawprobe opens SOCK_RAW without IP_HDRINCL, and raw_sendmsg (Linux
// net/ipv4/raw.c) builds its flow with flowi4_init_output(..., sk->sk_protocol, ...,
// daddr, saddr, 0, 0, ...): protocol UDP and both ports 0, and raw_probe_proto_opt
// fills header fields for ICMP alone. A selector with sport and dport 4500 never
// matched it (the second run: the policy installed, XfrmInNoStates 0 -> 20).
//
// While it is installed, any UDP from strongSwan to Ze leaves in the clear. In that
// window strongSwan sends Ze only IKE, which charon's socket bypass already keeps out
// of the Child SA, and ESP rides no UDP because no NAT is present. The policy is
// deleted before the hold lifts, so the tunnel the scenario then verifies is the one
// charon installed. Priority 1 wins over charon's policy, since a lower value takes
// precedence. extra follows the selector: the priority on add, nothing on delete.
func authInjectBypassCommand(verb string, extra ...string) []string {
	command := []string{"ip", xfrmCommand, ipWordPolicy, verb,
		ipWordSrc, swanIP + "/32", xfrmHeaderTarget, zeIP + "/32",
		"proto", protocolUDP, xfrmWordDir, "out"}
	return append(command, extra...)
}

// authLiftCommand deletes the table holdAuthResponses installed, and its rule with it.
func authLiftCommand() []string {
	return []string{authHoldCommand, wordDelete, ipWordTable, authHoldFamily, authHoldTable}
}

// waitAuthBudget waits for ready, but no later than liftBy: a step that has not
// finished by then leaves Ze no retransmission after the lift, and the scenario could
// then fail for its own timing rather than for Ze. The refusal names the bound.
func (l *scenarioLab) waitAuthBudget(ctx context.Context, liftBy time.Time, description string,
	probe func(context.Context) (int, error), ready func(int) bool) error {
	remaining := time.Until(liftBy)
	if remaining <= 0 {
		return fmt.Errorf("%s: no time left before Ze's last IKE_AUTH retransmission, %s after it sent the request",
			description, zeLastAuthRetransmitOffset())
	}
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: remaining, Interval: time.Second, Description: description,
	}, probe, ready)
	if err != nil {
		return fmt.Errorf("within Ze's retransmit window (last retransmission %s after the request): %w",
			zeLastAuthRetransmitOffset(), err)
	}
	return nil
}

// waitZeInitiatorSPI waits until Ze reports its IKE SA with strongSwan established as
// initiator, and refuses one whose initiator SPI is not want: that is a new handshake,
// started because the forgeries ended the one they named. It waits as long as Ze keeps
// retransmitting, until it gives up the handshake.
func (l *scenarioLab) waitZeInitiatorSPI(ctx context.Context, want string) error {
	got, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: zeAuthGiveUpOffset(), Interval: time.Second,
		Description: "Ze establishing its IKE SA once the IKE_AUTH hold lifted",
	}, func(probe context.Context) (zeEstablishedSA, error) {
		answer, err := l.zeCLI(probe, "show vpn ipsec sa | json")
		if err != nil {
			return zeEstablishedSA{}, err
		}
		return zeEstablishedInitiatorSA(answer)
	}, func(sa zeEstablishedSA) bool { return sa.established })
	if err != nil {
		return err
	}
	if !strings.EqualFold(got.initiatorSPI, want) {
		return fmt.Errorf("ze established IKE SA %s, not %s whose handshake the forgeries targeted: the forgeries ended it",
			got.initiatorSPI, want)
	}
	return nil
}

// zeEstablishedSA is one reading of Ze's IKE SA with strongSwan: whether one is
// established, and its initiator SPI when it is.
type zeEstablishedSA struct {
	established  bool
	initiatorSPI string
}

// zeEstablishedInitiatorSA reads Ze's established IKE SA with strongSwan out of `show
// vpn ipsec sa | json`. Two established SAs are an error, because the checker could
// not tell which one the forgeries targeted, and so is one where Ze is the responder.
func zeEstablishedInitiatorSA(answer string) (zeEstablishedSA, error) {
	var records []mobikeZeRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(answer)), &records); err != nil {
		return zeEstablishedSA{}, fmt.Errorf("show vpn ipsec sa | json is not a list of IKE SAs: %w; answer: %s", err, answer)
	}
	var reading zeEstablishedSA
	for i := range records {
		record := &records[i]
		if record.Peer != swanConfigPeer {
			continue
		}
		if record.State != zeStateEstablished {
			continue
		}
		if !record.Initiator {
			return zeEstablishedSA{}, fmt.Errorf("ze lists its established IKE SA with strongSwan as responder, want initiator: %s", answer)
		}
		if reading.established {
			return zeEstablishedSA{}, fmt.Errorf("ze lists two established IKE SAs with strongSwan: %s", answer)
		}
		reading = zeEstablishedSA{established: true, initiatorSPI: record.InitiatorSPI}
	}
	return reading, nil
}

// swanIKESPIPairs reads every IKE SA's SPI pair out of `swanctl --list-sas`.
func swanIKESPIPairs(listing string) []ikeSPIPair {
	matches := authRespSwanIKE.FindAllStringSubmatch(listing, -1)
	pairs := make([]ikeSPIPair, 0, len(matches))
	for _, match := range matches {
		pairs = append(pairs, ikeSPIPair{initiator: match[1], responder: match[2]})
	}
	return pairs
}

// heldSPIPair picks the IKE SA of the held handshake: the one listed SA whose initiator
// SPI is not the control's. The control SA may still be listed while charon deletes it.
func heldSPIPair(listing string, control ikeSPIPair) (ikeSPIPair, error) {
	var held []ikeSPIPair
	for _, pair := range swanIKESPIPairs(listing) {
		if strings.EqualFold(pair.initiator, control.initiator) {
			continue
		}
		held = append(held, pair)
	}
	if len(held) != 1 {
		return ikeSPIPair{}, interoplab.WithSearchedOutput(
			fmt.Errorf("want one IKE SA besides the control's (initiator SPI %s), swanctl lists %d", control.initiator, len(held)), listing)
	}
	return held[0], nil
}

// zeRetransmitBackoff is the delay before Ze's retransmission attempt+1, as
// retransmitBackoff in fsm.go computes it: retransmitBase doubled per attempt, capped.
func zeRetransmitBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return zeRetransmitBase
	}
	shift := min(attempt-1, 7)
	return min(zeRetransmitBase*time.Duration(1<<uint(shift)), zeRetransmitMax)
}

// zeLastAuthRetransmitOffset is when Ze sends its last IKE_AUTH retransmission,
// measured from the request. runInitiator first waits retransmitBase, and before
// retransmission n+1 waits retransmitBackoff(n), for maxRetransmissions sends: 0.5 s,
// then 0.5, 1, 2, 4, 8 and 16 more, so the seventh leaves 32 s after the request.
func zeLastAuthRetransmitOffset() time.Duration {
	offset := zeRetransmitBase
	for attempt := 1; attempt < zeMaxRetransmissions; attempt++ {
		offset += zeRetransmitBackoff(attempt)
	}
	return offset
}

// zeAuthGiveUpOffset is when Ze abandons the handshake, measured from the request:
// one backoff after its last retransmission, 64 s.
func zeAuthGiveUpOffset() time.Duration {
	return zeLastAuthRetransmitOffset() + zeRetransmitBackoff(zeMaxRetransmissions)
}
