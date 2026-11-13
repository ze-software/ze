// Design: docs/architecture/l2tp/subscriber-session-model.md -- the order of
// one L2TP session's notifications: an operator clear of a live session
// leaves no subscriber, no open accounting record and no session behind
// (spec-single-owner-lifecycle-5-session-end-locals AC-8, AC-9).
// Related: register_l2tp_session_clear.go -- registers this peer and its RADIUS mock.
// Related: tunnel_fixture_l2tp_ppp.go -- tunnelL2TPOpenProxySession and
// tunnelAccountingPeer, the LAC half of the session this peer drives.
// Related: tunnel_fixture_l2tp_radius.go -- tunnelRadiusAccounting and
// tunnelRadiusRXLog, what the RADIUS server decoded.
// Related: daemon_await_answer_fixture.go -- the `ze cli` barrier this
// peer's waits follow, and awaitAnswerTail.
// Related: test/l2tp/session-clear-leaves-no-subscriber.ci -- the only caller.

package fixture

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// sessionClearDelay spaces the attempts of every wait here, and bounds each
// read of the link. One CLI attempt is a whole `ze cli` process and an SSH
// session, as in daemonAwaitAnswer, so a shorter delay only loads the daemon.
const sessionClearDelay = 250 * time.Millisecond

// sessionClearWaitShare is the share of the test's budget one wait may take,
// in percent. The peer waits five times, so the five together stay under the
// budget and a stall ends with this peer's message naming what never came.
const sessionClearWaitShare = 15

// tunnelTerminateCauseAdminReset is RFC 2866 Section 5.10 value 6, "Admin
// Reset", described there as "Administrator reset the port or session." Ze
// reports it for an operator clear (Subsystem.TeardownSession), which is how
// this peer's session ends.
const tunnelTerminateCauseAdminReset = "6"

// tunnelL2TPMessageTypeCDN is the RFC 2661 Section 6.12 Call-Disconnect-Notify.
const tunnelL2TPMessageTypeCDN = 14

// tunnelL2TPAVPAssignedSessionID is the RFC 2661 Assigned Session ID AVP.
const tunnelL2TPAVPAssignedSessionID = 14

// tunnelL2TPSessionClear is the LAC peer of
// test/l2tp/session-clear-leaves-no-subscriber.ci. It brings one PPP session
// up to its IPCP address, waits until the daemon reports that session in every
// view the clear must empty, has the operator clear it over the CLI, and then
// reads each view again with no wait: the clear runs every reader of the
// session-down before it answers (teardownSessionByID delivers before it sends
// the CDN and returns), so the state the answer leaves is the end state.
//
// Arguments: <l2tp-port>. The CLI takes the daemon's SSH address and
// credentials from the environment the runner gave this process.
func tunnelL2TPSessionClear(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
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
		return fmt.Errorf("IPCP settled on %s, which is not an IPv4 address", peer.negotiatedAddr)
	}

	link := startSessionClearLink(ctx, peer)
	defer link.stop()

	acctSessionID, err := link.awaitAccountingStart(ctx)
	if err != nil {
		return err
	}
	if err := link.awaitListedSession(ctx, peer.zeSID, address); err != nil {
		return err
	}
	if err := link.awaitActiveSubscriber(ctx, address); err != nil {
		return err
	}
	link.clearing.Store(true)
	if err := clearL2TPSession(ctx, peer.zeSID); err != nil {
		return err
	}
	if err := checkClearedAccounting(acctSessionID); err != nil {
		return err
	}
	if err := checkNoL2TPSession(ctx); err != nil {
		return err
	}
	if err := checkNoSubscriber(ctx); err != nil {
		return err
	}
	return link.awaitCDN(ctx, peer.zeSID)
}

// sessionClearLink keeps one session's link answered while the driver talks
// to the daemon over the CLI: LCP echo, CHAP and IPCP, and the CDN the clear
// brings. startSessionClearLink starts its goroutine, and the driver MUST call
// stop before it closes the peer's socket; stop returns once the goroutine
// has. From start to stop the goroutine alone touches the peer, its socket
// and its sequence numbers. The driver reads only clearing, which it alone
// writes, and the outcomes serve publishes by closing broken or cdnSeen,
// after it wrote err or cdnSID.
type sessionClearLink struct {
	peer *tunnelAccountingPeer
	// clearing says the operator's clear is under way, so ze's teardown
	// messages and its last CDN are expected rather than a failure.
	clearing atomic.Bool
	cancel   context.CancelFunc
	done     chan struct{}
	// broken is closed once err holds the protocol failure that ended serve.
	broken chan struct{}
	err    error
	// cdnSeen is closed once cdnSID holds the Assigned Session ID of the
	// first CDN ze sent after the clear.
	cdnSeen chan struct{}
	cdnSID  uint16
}

func startSessionClearLink(ctx context.Context, peer *tunnelAccountingPeer) *sessionClearLink {
	linkCtx, cancel := context.WithCancel(ctx)
	link := &sessionClearLink{
		peer:    peer,
		cancel:  cancel,
		done:    make(chan struct{}),
		broken:  make(chan struct{}),
		cdnSeen: make(chan struct{}),
	}
	go link.serve(linkCtx)
	return link
}

// stop ends serve and waits for it. MUST be called before the peer's socket
// is closed.
func (link *sessionClearLink) stop() {
	link.cancel()
	<-link.done
}

// serve answers the link until ctx ends or the session breaks. Its loop is
// bounded by ctx: every read ends within sessionClearDelay.
func (link *sessionClearLink) serve(ctx context.Context) {
	defer close(link.done)
	cdnRecorded := false
	for ctx.Err() == nil {
		packet, err := link.peer.readPacket(sessionClearDelay)
		if err != nil {
			// A read deadline passing is the normal answer of an idle link.
			continue
		}
		if link.clearing.Load() {
			// The clear tears the session down: what ze sends from here on
			// is its teardown, and nothing is owed to keep the link alive.
			link.peer.silent = true
		}
		avps, isCDN := tunnelL2TPCDN(packet)
		if !isCDN {
			if err := link.peer.handleAccountingPacket(packet); err != nil {
				link.fail(err)
				return
			}
			continue
		}
		link.acknowledge(packet)
		if !link.clearing.Load() {
			link.fail(errors.New("ze sent a CDN before the operator cleared the session"))
			return
		}
		if cdnRecorded {
			continue
		}
		cdnRecorded = true
		if err := link.recordCDN(avps); err != nil {
			link.fail(err)
			return
		}
	}
}

// tunnelL2TPCDN answers the AVPs of packet when it is a control message
// carrying a Call-Disconnect-Notify, and false for anything else.
func tunnelL2TPCDN(packet []byte) (map[uint16][]byte, bool) {
	if len(packet) < 12 {
		return nil, false
	}
	if binary.BigEndian.Uint16(packet[:2])&0x8000 == 0 {
		return nil, false
	}
	avps, err := tunnelL2TPParseAVPs(packet)
	if err != nil {
		// Not a well-formed control message, so not a CDN: the accounting
		// peer's own handler judges it.
		return nil, false
	}
	if tunnelL2TPMessageType(avps) != tunnelL2TPMessageTypeCDN {
		return nil, false
	}
	return avps, true
}

// acknowledge advances Nr past the control message in packet and sends the
// ZLB that acknowledges it, so ze stops retransmitting it.
func (link *sessionClearLink) acknowledge(packet []byte) {
	peer := link.peer
	if ns := binary.BigEndian.Uint16(packet[8:10]); ns == peer.nr {
		peer.nr = ns + 1
	}
	// A lost ZLB only makes ze retransmit the CDN, which serve acknowledges
	// again.
	_, _ = peer.conn.WriteToUDP(tunnelL2TPControl(peer.localTID, 0, peer.ns, peer.nr, nil), peer.target)
}

// recordCDN publishes the session the CDN named.
func (link *sessionClearLink) recordCDN(avps map[uint16][]byte) error {
	sid := avps[tunnelL2TPAVPAssignedSessionID]
	if len(sid) != 2 {
		// RFC 2661 Section 6.12: "The following AVPs MUST be present in the
		// CDN:" Message Type, Result Code and Assigned Session ID.
		return fmt.Errorf("ze's CDN carries an Assigned Session ID of %d octets, want 2", len(sid))
	}
	link.cdnSID = binary.BigEndian.Uint16(sid)
	close(link.cdnSeen)
	return nil
}

func (link *sessionClearLink) fail(err error) {
	link.err = err
	close(link.broken)
}

// failure answers the error that ended serve, or nil while the link holds.
func (link *sessionClearLink) failure() error {
	select {
	case <-link.broken:
		return link.err
	default:
		return nil
	}
}

// The Acct-Status-Type values tunnelRadiusDescribe writes for RFC 2866
// Section 5.1 values 1 and 2.
const (
	accountingStatusStart = "Start"
	accountingStatusStop  = "Stop"
)

// await polls probe every sessionClearDelay within its share of the test's
// budget. probe answers nil once the state holds, and otherwise an error
// saying what it saw instead: a failed CLI run is "not yet", as in
// daemonAwaitAnswer. The link breaking ends the wait with its own error, and a
// state that never came ends it with the last attempt's.
func (link *sessionClearLink) await(ctx context.Context, what string, probe func() error) error {
	var broken error
	last := errors.New("no attempt finished")
	attempts := WaitAttempts(sessionClearWaitShare, sessionClearDelay, 120)
	held := Poll(ctx, attempts, sessionClearDelay, func() bool {
		if err := link.failure(); err != nil {
			broken = err
			return true
		}
		if err := probe(); err != nil {
			last = err
			return false
		}
		return true
	})
	if broken != nil {
		return broken
	}
	if !held {
		return fmt.Errorf("%s never came; the last attempt saw: %w", what, last)
	}
	return nil
}

// awaitAccountingStart waits for the Accounting-Start of the session at the
// RADIUS server and answers its Acct-Session-Id. Accounting starts on its own
// goroutine after the address is published, so the Start being at the
// server, before the clear is issued, is this test's precondition: it takes
// the Start-after-Stop race out of what is asserted.
func (link *sessionClearLink) awaitAccountingStart(ctx context.Context) (string, error) {
	var start accountingRecord
	err := link.await(ctx, "an Accounting-Start at the RADIUS server", func() error {
		records, err := readAccountingRecords()
		if err != nil {
			return err
		}
		for _, record := range records {
			if record.status == accountingStatusStart {
				start = record
				return nil
			}
		}
		return fmt.Errorf("%d accounting records, none a Start", len(records))
	})
	if err != nil {
		return "", err
	}
	if start.sessionID == "" {
		// RFC 2866 Section 5.5: "The start and stop records for a given
		// session MUST have the same Acct-Session-Id."
		return "", fmt.Errorf("the Accounting-Start carries no Acct-Session-Id: %s", start.line)
	}
	var line textbuf.Buffer
	fmt.Println(line.Str("OK: Accounting-Start with Acct-Session-Id ").Str(start.sessionID).
		Str(" reached the RADIUS server before the clear").String())
	return start.sessionID, nil
}

// The two commands whose answers the clear must empty.
const (
	sessionClearShowSessions    = "show l2tp sessions | json"
	sessionClearShowSubscribers = "show subscriber | json"
)

// awaitListedSession waits until `show l2tp sessions` lists the session with
// its address. It is the positive control for checkNoL2TPSession: the same
// command, in the same run, did list the session.
func (link *sessionClearLink) awaitListedSession(ctx context.Context, sid uint16, address netip.Addr) error {
	what := "session " + sessionIDText(sid) + " with address " + address.String()
	err := link.awaitRow(ctx, what, sessionClearShowSessions, "local-sid", func(row map[string]any) bool {
		return row["local-sid"] == float64(sid) && row["assigned-addr"] == address.String()
	})
	if err != nil {
		return err
	}
	fmt.Println("OK: show l2tp sessions lists " + what)
	return nil
}

// awaitActiveSubscriber waits until `show subscriber` lists an active L2TP
// subscriber holding the address. Active means the session-up reached the
// registry, so both the address and the session-up of this session were
// delivered before the clear. It is the positive control for
// checkNoSubscriber.
func (link *sessionClearLink) awaitActiveSubscriber(ctx context.Context, address netip.Addr) error {
	what := "an active l2tp subscriber with address " + address.String()
	err := link.awaitRow(ctx, what, sessionClearShowSubscribers, "access-type", func(row map[string]any) bool {
		return row["access-type"] == "l2tp" && row["state"] == "active" && row["ipv4"] == address.String()
	})
	if err != nil {
		return err
	}
	fmt.Println("OK: show subscriber lists " + what)
	return nil
}

// awaitRow polls command until one object holding key in its answer matches.
// A CLI run that fails is "not yet", as in daemonAwaitAnswer, and the message
// for a state that never came quotes the last answer or failure.
func (link *sessionClearLink) awaitRow(ctx context.Context, what, command, key string, match func(map[string]any) bool) error {
	return link.await(ctx, what+" in `"+command+"`", func() error {
		answer, err := sessionClearCLI(ctx, command)
		if err != nil {
			return err
		}
		if slices.ContainsFunc(jsonObjectsWith(answer, key), match) {
			return nil
		}
		return errors.New("an answer without it: " + describeJSON(answer))
	})
}

// clearL2TPSession is the operator's clear of one session, over the CLI.
func clearL2TPSession(ctx context.Context, sid uint16) error {
	command := "clear l2tp session id " + sessionIDText(sid) + " | json"
	answer, err := sessionClearCLI(ctx, command)
	if err != nil {
		return err
	}
	for _, row := range jsonObjectsWith(answer, "status") {
		if row["action"] == "session-teardown" && row["session-id"] == float64(sid) && row["status"] == "sent" {
			fmt.Println("OK: clear l2tp session id " + sessionIDText(sid) + " answered status sent")
			return nil
		}
	}
	return fmt.Errorf("`ze cli -c %s` answered %s, not a session-teardown of session %d with status sent", command, describeJSON(answer), sid)
}

// checkClearedAccounting reads, once and right after the clear answered,
// every accounting record the RADIUS server received. The session must hold
// exactly one Start and one Stop under the one Acct-Session-Id, and no record
// of any other session may exist: this run carries one session, so a second
// Acct-Session-Id is an accounting session a late event started after the
// teardown. Interim-Updates are not counted; the config asks for none.
//
// The order of the two records on the wire is not asserted. The Start is sent
// from the accounting task's goroutine and the Stop at once on teardown, so
// for a teardown right after the address the Stop can reach the server first
// (plan/journal/late-write-lands-on-the-successor.md, 2026-11-12, owned by
// plan/release/fixes/spec-radius-acct-start-after-stop.md). Here the Start was
// seen at the server before the clear was issued, so the race cannot occur.
func checkClearedAccounting(acctSessionID string) error {
	records, err := readAccountingRecords()
	if err != nil {
		return err
	}
	starts, stops := 0, 0
	var stop accountingRecord
	for _, record := range records {
		if record.sessionID != acctSessionID {
			return fmt.Errorf("the RADIUS server holds a record of Acct-Session-Id %q beside the session's %q, and this run carries one session: %s",
				record.sessionID, acctSessionID, record.line)
		}
		if record.status == accountingStatusStart {
			starts++
		}
		if record.status == accountingStatusStop {
			stops++
			stop = record
		}
	}
	if starts != 1 {
		return fmt.Errorf("the RADIUS server holds %d Accounting-Start records for Acct-Session-Id %q after the clear, want 1", starts, acctSessionID)
	}
	if stops != 1 {
		return fmt.Errorf("the RADIUS server holds %d Accounting-Stop records for Acct-Session-Id %q once the clear answered, want 1", stops, acctSessionID)
	}
	if stop.cause != tunnelTerminateCauseAdminReset {
		return fmt.Errorf("the Accounting-Stop reports Acct-Terminate-Cause %q, not %s (Admin Reset) for an operator clear: %s",
			stop.cause, tunnelTerminateCauseAdminReset, stop.line)
	}
	fmt.Println("OK: the RADIUS server holds one Accounting-Start and one Accounting-Stop for Acct-Session-Id " +
		acctSessionID + ", the Stop with Acct-Terminate-Cause " + tunnelTerminateCauseAdminReset)
	return nil
}

// checkNoL2TPSession reads `show l2tp sessions` once, after the clear
// answered, and refuses any session it lists.
func checkNoL2TPSession(ctx context.Context) error {
	const command = sessionClearShowSessions
	answer, err := sessionClearCLI(ctx, command)
	if err != nil {
		return err
	}
	if rows := jsonObjectsWith(answer, "local-sid"); len(rows) != 0 {
		return fmt.Errorf("`ze cli -c %s` still lists %d session(s) after the clear: %s", command, len(rows), describeJSON(answer))
	}
	fmt.Println("OK: show l2tp sessions lists no session after the clear")
	return nil
}

// checkNoSubscriber reads `show subscriber` once, after the clear answered,
// and refuses any L2TP subscriber, and a summary whose total is not zero.
func checkNoSubscriber(ctx context.Context) error {
	const command = sessionClearShowSubscribers
	answer, err := sessionClearCLI(ctx, command)
	if err != nil {
		return err
	}
	if rows := jsonObjectsWith(answer, "access-type"); len(rows) != 0 {
		return fmt.Errorf("`ze cli -c %s` still lists %d subscriber(s) after the clear: %s", command, len(rows), describeJSON(answer))
	}
	summaries := jsonObjectsWith(answer, "total")
	if len(summaries) != 1 {
		return fmt.Errorf("`ze cli -c %s` answered %d summaries holding a total, want 1: %s", command, len(summaries), describeJSON(answer))
	}
	if summaries[0]["total"] != float64(0) {
		return fmt.Errorf("`ze cli -c %s` reports a total of %v subscribers after the clear, want 0: %s", command, summaries[0]["total"], describeJSON(answer))
	}
	fmt.Println("OK: show subscriber lists no subscriber after the clear, total 0")
	return nil
}

// awaitCDN waits for the CDN the clear owes the LAC, and refuses one that
// names another session.
func (link *sessionClearLink) awaitCDN(ctx context.Context, sid uint16) error {
	timer := time.NewTimer(WaitBudget(sessionClearWaitShare, 30*time.Second))
	defer timer.Stop()

	select {
	case <-link.cdnSeen:
	case <-link.broken:
		return link.err
	case <-timer.C:
		return fmt.Errorf("ze sent no CDN for session %d after the clear answered", sid)
	case <-ctx.Done():
		return ctx.Err()
	}
	if link.cdnSID != sid {
		return fmt.Errorf("ze's CDN after the clear names session %d, not the cleared session %d", link.cdnSID, sid)
	}
	fmt.Println("OK: ze sent a CDN for session " + sessionIDText(sid))
	return nil
}

// accountingRecord is one Accounting-Request line of tunnelRadiusRXLog.
type accountingRecord struct {
	status    string
	sessionID string
	cause     string
	line      string
}

// readAccountingRecords answers every complete Accounting-Request line the
// RADIUS mock wrote. A line the mock is still writing has no newline yet, so
// the text after the last newline is left for the next read.
func readAccountingRecords() ([]accountingRecord, error) {
	contents, err := os.ReadFile(tunnelRadiusRXLog)
	if err != nil {
		return nil, fmt.Errorf("read what the RADIUS server received: %w", err)
	}
	lines := strings.Split(string(contents), "\n")
	records := make([]accountingRecord, 0, len(lines))
	for _, line := range lines[:len(lines)-1] {
		if !strings.HasPrefix(line, "RADIUS-RX Accounting-Request ") {
			continue
		}
		status, _ := tunnelRadiusValue(line, "Acct-Status-Type")
		sessionID, _ := tunnelRadiusValue(line, "Acct-Session-Id")
		cause, _ := tunnelRadiusValue(line, "Acct-Terminate-Cause")
		records = append(records, accountingRecord{status: status, sessionID: sessionID, cause: cause, line: line})
	}
	return records, nil
}

// sessionClearCLI runs one `ze cli -c` command under the environment the
// runner gave this process, which names the daemon's SSH address and
// credentials, and decodes the JSON its `| json` pipe prints. A run that does
// not start, a nonzero exit and an answer that is not JSON are each an error
// quoting what the command printed.
func sessionClearCLI(ctx context.Context, command string) (any, error) {
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", command)
	if err != nil {
		return nil, fmt.Errorf("`ze cli -c %s` did not start: %w", command, err)
	}
	if code != 0 {
		return nil, fmt.Errorf("`ze cli -c %s` exited %d; stdout: %q; stderr: %q", command, code, awaitAnswerTail(stdout), awaitAnswerTail(stderr))
	}
	var answer any
	if err := json.Unmarshal([]byte(stdout), &answer); err != nil {
		return nil, fmt.Errorf("`ze cli -c %s` printed no JSON document (%w); stdout: %q", command, err, awaitAnswerTail(stdout))
	}
	return answer, nil
}

// jsonObjectsWith answers every JSON object in value that holds key, at any
// depth. The recursion is bounded by the nesting of one CLI answer, which a
// Ze command handler builds, so no peer chooses the depth. The answer is
// walked rather than decoded into a fixed shape so that a list printed bare
// and a list the CLI wraps read the same; the positive controls prove the
// walk finds the rows the command prints.
func jsonObjectsWith(value any, key string) []map[string]any {
	var found []map[string]any
	switch typed := value.(type) {
	case map[string]any:
		if _, held := typed[key]; held {
			found = append(found, typed)
		}
		for _, member := range typed {
			found = append(found, jsonObjectsWith(member, key)...)
		}
	case []any:
		for _, element := range typed {
			found = append(found, jsonObjectsWith(element, key)...)
		}
	}
	return found
}

// describeJSON renders a decoded answer for an error message, cut to the
// tail awaitAnswerTail keeps.
func describeJSON(answer any) string {
	encoded, err := json.Marshal(answer)
	if err != nil {
		return "an answer that does not encode again: " + err.Error()
	}
	return awaitAnswerTail(string(encoded))
}

// sessionIDText renders a session ID for a command or a message.
func sessionIDText(value uint16) string {
	var tb textbuf.Buffer
	return tb.Uint16(value).String()
}
