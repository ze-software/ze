// Design: docs/guide/l2tp.md -- PPP negotiation: each NCP opens and closes on
// its own for the life of the session, and a session with no NCP open is
// bounded by `ncp timeout` (spec-ppp-ncp-independent-lifetime).
// Detail: docs/architecture/l2tp/subscriber-session-model.md -- the per-family
// release notice that clears the subscriber registry's IPv4 address.
// Related: register_l2tp_ncp_lifetime.go -- registers both peers and their RADIUS mock.
// Related: tunnel_fixture_l2tp_ppp.go -- tunnelAccountingPeer, the LAC half
// both peers drive, and its Terminate-Request, renegotiation and
// Protocol-Reject primitives.
// Related: tunnel_fixture_l2tp_session_clear.go -- sessionClearCLI,
// jsonObjectsWith and describeJSON, the `ze cli` reads used here.
// Related: test/l2tp/ppp-ncp-terminate-after-up.ci,
// test/l2tp/ppp-ncp-protocol-reject-ipcp.ci -- the tests these peers serve.

package fixture

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ncpLifetimeWaitShare is the share of the test's budget, in percent, one
// wait of the terminate peer may take. Its happy path holds nine waits and
// the hold past the no-NCP bound; each finishes in a few seconds, so a stall
// ends with this peer naming what never came rather than the runner's kill.
const ncpLifetimeWaitShare = 10

// ncpLifetimeDelay is how long the link is answered between two reads of a
// state. One read can be a whole `ze cli` process and an SSH session, as in
// sessionClearDelay.
const ncpLifetimeDelay = 250 * time.Millisecond

// ncpLifetimeHoldMargin is how far past the no-NCP bound, measured from ze's
// Terminate-Ack, the terminate peer keeps the session before it reads LCP
// once more. The bound was armed when IPCP left Opened, which is before the
// Terminate-Ack left ze, so the hold ends past the bound's expiry.
const ncpLifetimeHoldMargin = 3 * time.Second

// ncpLifetimeTerminateID is the Identifier of the peer's IPCP
// Terminate-Request, chosen apart from the Configure-Request Identifiers the
// peer counts up from 1 so an echo of the wrong one cannot pass.
const ncpLifetimeTerminateID = 0x5a

// tunnelL2TPNCPTerminateAfterUp is the LAC peer of
// test/l2tp/ppp-ncp-terminate-after-up.ci. It opens IPCP, waits until the
// daemon shows the IPv4 family in the subscriber registry and on the session's
// pppN, then closes IPCP with a Terminate-Request after session-up. It checks
// the Terminate-Ack carries the request's Identifier, that the address leaves
// pppN, and that `show subscriber id <id> detail` stops printing `ipv4` while
// the entry stays active. Then it opens IPCP again with the held address and
// checks both come back. Last it keeps the session past the no-NCP bound, armed
// when IPCP left Opened: an LCP Terminate-Request, a CDN or a StopCCN in that
// span fails the run, and an LCP Echo-Request must still be answered after it.
//
// Arguments: <l2tp-port> <ncp-timeout-seconds>. The second is the `ncp timeout`
// leaf of the test's config, which this peer cannot read. The CLI takes the
// daemon's SSH address and credentials from the environment the runner gave
// this process.
func tunnelL2TPNCPTerminateAfterUp(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	bound, err := ncpLifetimeArgDuration(args, 1, time.Second, "ncp timeout seconds")
	if err != nil {
		return err
	}
	peer, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer peer.conn.Close() //nolint:errcheck // fixture teardown

	if err := peer.negotiateIPCP(ctx); err != nil {
		return err
	}
	address, ok := netip.AddrFromSlice(peer.negotiatedAddr.To4())
	if !ok {
		return errors.New("IPCP settled on " + peer.negotiatedAddr.String() + ", which is not an IPv4 address")
	}
	subscriberID, pppName, err := peer.awaitActiveSubscriber(ctx, address)
	if err != nil {
		return err
	}
	if err := peer.awaitDetailIPv4(ctx, subscriberID, address, true, "before the IPCP Terminate-Request"); err != nil {
		return err
	}
	if err := peer.awaitInterfaceIPv4(ctx, pppName, true, "before the IPCP Terminate-Request"); err != nil {
		return err
	}

	terminatedAt, err := peer.terminateIPCP(ctx)
	if err != nil {
		return err
	}
	if err := peer.awaitInterfaceIPv4(ctx, pppName, false, "after the IPCP Terminate-Request"); err != nil {
		return err
	}
	if err := peer.awaitDetailIPv4(ctx, subscriberID, address, false, "after the IPCP Terminate-Request"); err != nil {
		return err
	}

	if err := peer.renegotiateIPCP(ctx, WaitBudget(ncpLifetimeWaitShare, 15*time.Second)); err != nil {
		return err
	}
	if !peer.negotiatedAddr.Equal(net.IP(address.AsSlice())) {
		return errors.New("IPCP opened again on " + peer.negotiatedAddr.String() + ", not on the held address " + address.String())
	}
	if err := peer.awaitInterfaceIPv4(ctx, pppName, true, "again after IPCP reopened"); err != nil {
		return err
	}
	if err := peer.awaitDetailIPv4(ctx, subscriberID, address, true, "again after IPCP reopened"); err != nil {
		return err
	}
	return peer.holdPastNoNCPBound(ctx, terminatedAt, bound)
}

// terminateIPCP sends the IPCP Terminate-Request and waits for ze's
// Terminate-Ack, which must carry the request's Identifier. It answers when
// the Ack arrived.
func (peer *tunnelAccountingPeer) terminateIPCP(ctx context.Context) (time.Time, error) {
	acks := peer.ipcpTermAcks
	peer.sendIPCPTerminateRequest(ncpLifetimeTerminateID)
	err := peer.serveUntil(ctx, "ze's IPCP Terminate-Ack", WaitBudget(ncpLifetimeWaitShare, 15*time.Second), func() bool {
		return peer.ipcpTermAcks > acks
	})
	if err != nil {
		return time.Time{}, err
	}
	at := time.Now()
	var tb textbuf.Buffer
	if peer.ipcpTermAckID != ncpLifetimeTerminateID {
		// RFC 1661 Section 5.5: "On reception, the Identifier field of the
		// Terminate-Request is copied into the Identifier field of the
		// Terminate-Ack packet."
		return time.Time{}, errors.New(tb.Str("ze's IPCP Terminate-Ack carries Identifier ").Uint8(peer.ipcpTermAckID).
			Str(", not the Terminate-Request's ").Uint8(ncpLifetimeTerminateID).String())
	}
	_ = tb.Str("OK: ze answered the IPCP Terminate-Request with a Terminate-Ack carrying Identifier ").
		Uint8(ncpLifetimeTerminateID).Byte('\n').StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
	return at, nil
}

// holdPastNoNCPBound keeps the session answered until ncpLifetimeHoldMargin
// past the no-NCP bound measured from terminatedAt, then sends an LCP
// Echo-Request and waits for its Echo-Reply. A bound the reopen did not cancel
// ends the session inside the hold with an LCP Terminate-Request or a CDN, and
// the handler fails the run on either.
func (peer *tunnelAccountingPeer) holdPastNoNCPBound(ctx context.Context, terminatedAt time.Time, bound time.Duration) error {
	if err := peer.serveFor(ctx, time.Until(terminatedAt.Add(bound+ncpLifetimeHoldMargin))); err != nil {
		return err
	}
	replies := peer.echoReplies
	identifier := peer.sendLCPEchoRequest()
	err := peer.serveUntil(ctx, "ze's LCP Echo-Reply", WaitBudget(ncpLifetimeWaitShare, 15*time.Second), func() bool {
		return peer.echoReplies > replies && peer.echoReplyID == identifier
	})
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	_ = tb.Str("OK: ze answered an LCP Echo-Request ").Str((bound + ncpLifetimeHoldMargin).String()).
		Str(" after the IPCP Terminate-Ack, past the ").Str(bound.String()).
		Str(" no-NCP bound, with no LCP Terminate-Request and no CDN\n").StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
	return nil
}

// awaitState answers the link between reads of probe until probe answers nil,
// within one wait's share of the budget. A failed probe is "not yet"; the
// link failing ends the wait with its own error, and a state that never came
// ends it naming what and quoting the last attempt.
func (peer *tunnelAccountingPeer) awaitState(ctx context.Context, what string, probe func() error) error {
	deadline := time.Now().Add(WaitBudget(ncpLifetimeWaitShare, 15*time.Second))
	for {
		last := probe()
		if last == nil {
			return nil
		}
		if !time.Now().Before(deadline) {
			return errors.New(what + " never came; the last attempt saw: " + last.Error())
		}
		if err := peer.serveFor(ctx, ncpLifetimeDelay); err != nil {
			return err
		}
	}
}

// awaitActiveSubscriber waits until `show subscriber` lists an active L2TP
// subscriber holding address, and answers its registry ID and its pppN. Active
// means the session-up reached the registry, so the Terminate-Request that
// follows lands after session-up.
func (peer *tunnelAccountingPeer) awaitActiveSubscriber(ctx context.Context, address netip.Addr) (string, string, error) {
	const command = sessionClearShowSubscribers
	var subscriberID, pppName string
	what := "an active l2tp subscriber with address " + address.String() + " in `" + command + "`"
	err := peer.awaitState(ctx, what, func() error {
		answer, err := sessionClearCLI(ctx, command)
		if err != nil {
			return err
		}
		rows := jsonObjectsWith(answer, "access-type")
		index := slices.IndexFunc(rows, func(row map[string]any) bool {
			return row["access-type"] == "l2tp" && row["state"] == "active" && row["ipv4"] == address.String()
		})
		if index < 0 {
			return errors.New("an answer without it: " + describeJSON(answer))
		}
		subscriberID, _ = rows[index]["id"].(string)
		pppName, _ = rows[index]["interface"].(string)
		return nil
	})
	if err != nil {
		return "", "", err
	}
	if subscriberID == "" {
		return "", "", errors.New("`ze cli -c " + command + "` lists the subscriber with no id")
	}
	if pppName == "" {
		return "", "", errors.New("`ze cli -c " + command + "` lists subscriber " + subscriberID + " with no interface")
	}
	var tb textbuf.Buffer
	_ = tb.Str("OK: show subscriber lists an active l2tp subscriber with address ").Addr(address).
		Str(" on ").Str(pppName).Byte('\n').StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
	return subscriberID, pppName, nil
}

// awaitDetailIPv4 waits until `show subscriber id <id> detail` prints the
// subscriber active with `ipv4` set to address (present) or with no `ipv4`
// field at all (absent). The entry staying active while `ipv4` leaves is what
// tells a family release from a session-down. when names the moment, for the
// success line.
func (peer *tunnelAccountingPeer) awaitDetailIPv4(ctx context.Context, subscriberID string, address netip.Addr, present bool, when string) error {
	command := "show subscriber id " + subscriberID + " detail | json"
	what := "`" + command + "` printing ipv4 " + address.String()
	if !present {
		what = "`" + command + "` printing the subscriber active with no ipv4"
	}
	err := peer.awaitState(ctx, what, func() error {
		answer, err := sessionClearCLI(ctx, command)
		if err != nil {
			return err
		}
		rows := jsonObjectsWith(answer, "access-type")
		if len(rows) != 1 {
			return errors.New("an answer holding " + strconv.Itoa(len(rows)) + " subscriber objects, want 1: " + describeJSON(answer))
		}
		row := rows[0]
		if row["id"] != subscriberID || row["state"] != "active" {
			return errors.New("an answer for another subscriber or state: " + describeJSON(answer))
		}
		ipv4, held := row["ipv4"]
		if present && ipv4 == address.String() {
			return nil
		}
		if !present && !held {
			return nil
		}
		return errors.New("an answer that does not: " + describeJSON(answer))
	})
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	if present {
		_ = tb.Str("OK: show subscriber id detail prints ipv4 ").Addr(address).Byte(' ').Str(when).Byte('\n').StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
		return nil
	}
	_ = tb.Str("OK: show subscriber id detail prints no ipv4 ").Str(when).Str(", the subscriber still active\n").StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
	return nil
}

// awaitInterfaceIPv4 waits until the session's pppN holds an IPv4 address
// (present) or none (absent). An interface that cannot be read is never
// taken for one with no address: the attempt fails and the wait goes on, so
// a pppN that vanished, which a session-down would do, fails the absent wait.
func (peer *tunnelAccountingPeer) awaitInterfaceIPv4(ctx context.Context, pppName string, present bool, when string) error {
	what := pppName + " holding an IPv4 address"
	if !present {
		what = pppName + " holding no IPv4 address"
	}
	var held []netip.Addr
	err := peer.awaitState(ctx, what, func() error {
		addresses, err := ncpLifetimeInterfaceIPv4(pppName)
		if err != nil {
			return err
		}
		held = addresses
		if present == (len(addresses) != 0) {
			return nil
		}
		var tb textbuf.Buffer
		return errors.New(tb.Str(pppName).Str(" holds ").Int(int64(len(addresses))).Str(" IPv4 address(es)").String())
	})
	if err != nil {
		return err
	}
	var tb textbuf.Buffer
	if present {
		_ = tb.Str("OK: ").Str(pppName).Str(" holds IPv4 address ").Addr(held[0]).Byte(' ').Str(when).Byte('\n').StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
		return nil
	}
	_ = tb.Str("OK: ").Str(pppName).Str(" holds no IPv4 address ").Str(when).Byte('\n').StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
	return nil
}

// ncpLifetimeInterfaceIPv4 answers the IPv4 addresses the kernel holds on the
// named interface. The peer runs in the daemon's network namespace, so this is
// the pppN ze installs the family on.
func ncpLifetimeInterfaceIPv4(name string) ([]netip.Addr, error) {
	link, err := net.InterfaceByName(name)
	if err != nil {
		return nil, errors.New("read interface " + name + ": " + err.Error())
	}
	addresses, err := link.Addrs()
	if err != nil {
		return nil, errors.New("read the addresses of " + name + ": " + err.Error())
	}
	var held []netip.Addr
	for _, entry := range addresses {
		network, ok := entry.(*net.IPNet)
		if !ok {
			continue
		}
		address, ok := netip.AddrFromSlice(network.IP)
		if ok && address.Unmap().Is4() {
			held = append(held, address.Unmap())
		}
	}
	return held, nil
}

// tunnelL2TPNCPProtocolRejectIPCP is the LAC peer of
// test/l2tp/ppp-ncp-protocol-reject-ipcp.ci: a subscriber that runs no IPv4,
// as pppd `noip` is. It answers ze's first IPCP packet with an LCP
// Protocol-Reject naming 0x8021, fails on any IPCP packet ze sends after it,
// and waits for the CDN that ends the session at the no-NCP bound. That CDN
// must name the session, carry Result Code 2, follow no LCP Terminate-Request
// (the handler fails on one), and come at least one Restart time after the
// Protocol-Reject. A shorter window would have owed ze no IPCP retransmission,
// so the absence of IPCP in it would prove nothing.
//
// Arguments: <l2tp-port> <lcp-restart-timer-milliseconds>. The second is the
// `lcp restart-timer` leaf of the test's config, which this peer cannot read.
func tunnelL2TPNCPProtocolRejectIPCP(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	restart, err := ncpLifetimeArgDuration(args, 1, time.Millisecond, "lcp restart-timer milliseconds")
	if err != nil {
		return err
	}
	peer, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer peer.conn.Close() //nolint:errcheck // fixture teardown

	peer.rejectIPCP = true
	peer.cdnAwaited = true
	err = peer.serveUntil(ctx, "ze's CDN", WaitBudget(70, 60*time.Second), func() bool {
		return peer.cdn != nil
	})
	if err != nil {
		return err
	}
	return peer.judgeNoNCPCDN(time.Now(), restart)
}

// judgeNoNCPCDN judges the CDN that ended a session whose only NCP the peer
// Protocol-Rejected, received at cdnAt.
func (peer *tunnelAccountingPeer) judgeNoNCPCDN(cdnAt time.Time, restart time.Duration) error {
	var tb textbuf.Buffer
	if peer.ipcpRejectedAt.IsZero() {
		return errors.New("ze sent its CDN before any IPCP packet, so no Protocol-Reject was sent and this run proves nothing")
	}
	window := cdnAt.Sub(peer.ipcpRejectedAt).Truncate(time.Millisecond)
	if window < restart {
		return errors.New(tb.Str("ze's CDN came ").Str(window.String()).Str(" after the Protocol-Reject, inside one Restart time of ").
			Str(restart.String()).Str(": ze owed no IPCP retransmission yet, so the absence of one proves nothing").String())
	}
	_ = tb.Str("OK: ze sent no IPCP packet in the ").Str(window.String()).
		Str(" between the Protocol-Reject and its CDN, longer than one Restart time of ").Str(restart.String()).Byte('\n').StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
	tb.Reset()
	result := peer.cdn[1]
	if len(result) < 2 {
		// RFC 2661 Section 6.12: "The following AVPs MUST be present in the
		// CDN:" Message Type, Result Code and Assigned Session ID.
		return errors.New(tb.Str("ze's CDN carries a Result Code of ").Int(int64(len(result))).Str(" octets, want at least 2").String())
	}
	if code := binary.BigEndian.Uint16(result[:2]); code != 2 {
		return errors.New(tb.Str("ze's CDN carries Result Code ").Uint16(code).Str(", not 2, which ze sends for a session PPP failed").String())
	}
	sid := peer.cdn[tunnelL2TPAVPAssignedSessionID]
	if len(sid) != 2 {
		return errors.New(tb.Str("ze's CDN carries an Assigned Session ID of ").Int(int64(len(sid))).Str(" octets, want 2").String())
	}
	if binary.BigEndian.Uint16(sid) != peer.zeSID {
		return errors.New(tb.Str("ze's CDN names session ").Uint16(binary.BigEndian.Uint16(sid)).
			Str(", not this call's session ").Uint16(peer.zeSID).String())
	}
	_ = tb.Str("OK: ze ended session ").Uint16(peer.zeSID).
		Str(" with a CDN carrying Result Code 2, and sent no LCP Terminate-Request\n").StdOut() //nolint:errcheck // progress line; the assertions are the returned errors
	return nil
}

// ncpLifetimeArgDuration reads a positive whole count of unit from args at
// index. name says what the argument is, for the refusal.
func ncpLifetimeArgDuration(args []string, index int, unit time.Duration, name string) (time.Duration, error) {
	if len(args) <= index {
		return 0, errors.New("missing " + name + " argument")
	}
	value, err := strconv.Atoi(args[index])
	if err != nil {
		return 0, errors.New("invalid " + name + " " + strconv.Quote(args[index]) + ": " + err.Error())
	}
	if value < 1 {
		return 0, errors.New("invalid " + name + " " + strconv.Quote(args[index]) + ": want a whole number above 0")
	}
	return time.Duration(value) * unit, nil
}
