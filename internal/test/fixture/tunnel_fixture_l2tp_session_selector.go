// Design: docs/architecture/l2tp/subscriber-session-model.md -- a readable
// session selector resolves once to one captured original: a second match is
// an ambiguity refusal, and a Birth that no longer matches selects nothing.
// Related: register_l2tp_session_selector_scope.go -- registers this peer and its RADIUS mock.
// Related: tunnel_fixture_l2tp_ppp.go -- tunnelL2TPOpenProxyTunnel, openProxyCall
// and connectProxyCall, the LAC calls this peer places.
// Related: tunnel_fixture_l2tp_session_clear.go -- startSessionClearLink,
// sessionClearCLI, readAccountingRecords and jsonObjectsWith, which this peer reuses.
// Related: test/draft/l2tp/session-selector-scope.ci -- the only caller.

package fixture

import (
	"cmp"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
)

// selectorHeldCalls is how many calls the first tunnel holds in wait-connect
// while the second tunnel draws session IDs against them. Ze draws each
// session ID at random from the 65535 non-zero values, unique only within its
// tunnel (allocateSessionID, internal/component/l2tp/session.go), so each draw
// on the second tunnel equals one of the held IDs with probability about
// 255/65535. The default per-tunnel max-sessions is 1024, above this count.
const selectorHeldCalls = 256

// selectorDrawsMax bounds the second tunnel's draws. With 255 eligible held
// IDs the chance that this many draws all miss is (1-255/65535)^4096, about
// 1e-7, and the expected number of draws is about 257.
const selectorDrawsMax = 4096

// selectorFirstLACSID is the LAC's Assigned Session ID of its first search
// call. Every call of the search takes the next one, so no two calls on one
// tunnel share it and ze's duplicate-remote-SID check never drops an ICRQ.
const selectorFirstLACSID = 1000

// selectorWaitShare is the share of the test's budget one wait may take, in
// percent. The peer waits about ten times, so a stall ends with this peer's
// message naming what never came rather than the runner's kill.
const selectorWaitShare = 8

// selectorControlWait bounds one control exchange of the search: a ZLB that
// acknowledges a CDN. Ze answers within one reactor turn; this is the stall
// bound, not a pacing delay.
const selectorControlWait = 10 * time.Second

// tunnelL2TPSessionSelectorScope is the LAC peer of
// test/draft/l2tp/session-selector-scope.ci. Goal: through the operator's
// `clear l2tp session id` over SSH, a unique bare session ID clears exactly
// its session; a bare ID two live sessions share is refused as ambiguous with
// no effect, naming the qualified clear of each; the qualified form clears
// exactly the one it names; an ended session's expected-id, typed with the
// coordinates of the live session that still holds its session ID, is refused
// and that live session survives.
//
// Method: one PPP session on its own tunnel is the unique one. Two more
// tunnels then hold calls until ze's own allocator has given both the same
// session ID (openSelectorCollision), and both become PPP sessions. Each step
// reads `show l2tp sessions`, the links and the RADIUS server once after the
// command answered, because a captured teardown removes the row, publishes
// the session-down and sends the CDN before the command answers.
//
// Arguments: <l2tp-port>. The CLI takes the daemon's SSH address and
// credentials from the environment the runner gave this process.
func tunnelL2TPSessionSelectorScope(ctx context.Context, args []string) error {
	port, err := tunnelArgPort(args, 0)
	if err != nil {
		return err
	}
	unique, err := tunnelL2TPOpenProxySession(ctx, port, 1500, 1500)
	if err != nil {
		return err
	}
	defer unique.conn.Close() //nolint:errcheck // fixture teardown
	if err := unique.negotiateIPCP(ctx); err != nil {
		return err
	}
	uniqueLink := startSessionClearLink(ctx, unique)
	defer uniqueLink.stop()

	first, second, err := openSelectorCollision(ctx, port, unique.zeSID)
	if err != nil {
		return err
	}
	defer first.conn.Close()  //nolint:errcheck // fixture teardown
	defer second.conn.Close() //nolint:errcheck // fixture teardown
	first.connectProxyCall(1500, 1500)
	if err := first.negotiateIPCP(ctx); err != nil {
		return err
	}
	firstLink := startSessionClearLink(ctx, first)
	defer firstLink.stop()
	second.connectProxyCall(1500, 1500)
	if err := second.negotiateIPCP(ctx); err != nil {
		return err
	}
	secondLink := startSessionClearLink(ctx, second)
	defer secondLink.stop()

	scope := selectorScope{
		links: []*sessionClearLink{uniqueLink, firstLink, secondLink},
		ended: map[string]bool{},
	}
	if err := scope.awaitAdmitted(ctx, unique, first, second); err != nil {
		return err
	}
	return scope.run(ctx, uniqueLink, firstLink, secondLink)
}

// openSelectorCollision opens two tunnels and leaves each holding one call in
// wait-connect, the two calls under the same ze session ID. The first tunnel
// places selectorHeldCalls calls; the second places one call at a time and
// hangs up each whose ID no held call has, until one has. The first tunnel
// then hangs up every held call but that one. excluded is the unique
// session's ID, which a collision must not take, so the bare form of it stays
// unique. The caller owns both sockets and MUST close them; an error closes
// them here.
func openSelectorCollision(ctx context.Context, port int, excluded uint16) (*tunnelAccountingPeer, *tunnelAccountingPeer, error) {
	first, err := tunnelL2TPOpenProxyTunnel(ctx, port)
	if err != nil {
		return nil, nil, err
	}
	second, err := tunnelL2TPOpenProxyTunnel(ctx, port)
	if err != nil {
		_ = first.conn.Close() //nolint:errcheck // failed fixture setup
		return nil, nil, err
	}
	if err := searchSelectorCollision(ctx, first, second, excluded); err != nil {
		_ = first.conn.Close()  //nolint:errcheck // failed fixture setup
		_ = second.conn.Close() //nolint:errcheck // failed fixture setup
		return nil, nil, err
	}
	return first, second, nil
}

// searchSelectorCollision is the search openSelectorCollision describes. It
// returns with first.zeSID equal to second.zeSID, and both calls'
// ICRPs acknowledged so ze retransmits nothing while PPP starts elsewhere.
func searchSelectorCollision(ctx context.Context, first, second *tunnelAccountingPeer, excluded uint16) error {
	draws := newSelectorDraws(excluded)
	for index := range selectorHeldCalls {
		if err := ctx.Err(); err != nil {
			return err
		}
		lacSID := uint16(selectorFirstLACSID + index)
		if err := first.openProxyCall(lacSID); err != nil {
			return fmt.Errorf("held call %d of %d on tunnel %d: %w", index+1, selectorHeldCalls, first.localTID, err)
		}
		if err := draws.hold(first.zeSID, lacSID); err != nil {
			return err
		}
	}
	selectorAcknowledge(first)
	found := false
	drawn := 0
	for drawn < selectorDrawsMax {
		if err := ctx.Err(); err != nil {
			return err
		}
		lacSID := uint16(selectorFirstLACSID + drawn)
		drawn++
		if err := second.openProxyCall(lacSID); err != nil {
			return fmt.Errorf("draw %d on tunnel %d: %w", drawn, second.localTID, err)
		}
		if draws.collides(second.zeSID) {
			found = true
			break
		}
		if err := selectorHangUp(second, second.zeSID, lacSID); err != nil {
			return err
		}
	}
	if !found {
		return fmt.Errorf("%d session IDs drawn on tunnel %d matched none of the %d held on tunnel %d",
			drawn, second.localTID, selectorHeldCalls, first.localTID)
	}
	selectorAcknowledge(second)
	shared := second.zeSID
	for _, call := range draws.hangUps(shared) {
		if err := selectorHangUp(first, call.zeSID, call.lacSID); err != nil {
			return err
		}
	}
	first.zeSID = shared
	fmt.Println("OK: ze gave tunnels " + sessionIDText(first.localTID) + " and " + sessionIDText(second.localTID) +
		" the same session ID " + sessionIDText(shared) + " after " + sessionIDText(uint16(drawn)) + " draws")
	return nil
}

// selectorCall is one call the first tunnel holds: ze's session ID and the
// LAC's Assigned Session ID its CDN names.
type selectorCall struct {
	zeSID  uint16
	lacSID uint16
}

// selectorDraws is what the collision search knows: the calls the first
// tunnel holds, by ze's session ID, and the one ID no collision may take.
type selectorDraws struct {
	held     map[uint16]uint16
	excluded uint16
}

func newSelectorDraws(excluded uint16) selectorDraws {
	return selectorDraws{held: make(map[uint16]uint16, selectorHeldCalls), excluded: excluded}
}

// hold records a call ze answered on the first tunnel. Ze's allocator never
// gives two live calls of one tunnel the same ID, so a repeat is refused as
// the defect it would be, and so is the reserved zero.
func (draws selectorDraws) hold(zeSID, lacSID uint16) error {
	if zeSID == 0 {
		return errors.New("ze answered a call with session ID 0, which RFC 2661 reserves")
	}
	if _, taken := draws.held[zeSID]; taken {
		return errors.New("ze gave two live calls of one tunnel session ID " + sessionIDText(zeSID))
	}
	draws.held[zeSID] = lacSID
	return nil
}

// collides answers whether an ID drawn on the second tunnel equals a held
// call's. The excluded ID never counts: the unique session holds it.
func (draws selectorDraws) collides(zeSID uint16) bool {
	if zeSID == draws.excluded {
		return false
	}
	_, held := draws.held[zeSID]
	return held
}

// hangUps answers every held call but keep, in ascending ze session ID, so a
// failure names the same call on every run of the same draws.
func (draws selectorDraws) hangUps(keep uint16) []selectorCall {
	calls := make([]selectorCall, 0, len(draws.held))
	for zeSID, lacSID := range draws.held {
		if zeSID == keep {
			continue
		}
		calls = append(calls, selectorCall{zeSID: zeSID, lacSID: lacSID})
	}
	slices.SortFunc(calls, func(a, b selectorCall) int { return cmp.Compare(a.zeSID, b.zeSID) })
	return calls
}

// selectorAcknowledge sends the ZLB that acknowledges every control message
// the peer has read, so ze stops retransmitting a call's ICRP while the peer
// works on another tunnel. A lost ZLB only brings a retransmission, which the
// peer's next read acknowledges in turn.
func selectorAcknowledge(peer *tunnelAccountingPeer) {
	_, _ = peer.conn.WriteToUDP(tunnelL2TPControl(peer.localTID, 0, peer.ns, peer.nr, nil), peer.target)
}

// selectorHangUp sends the LAC's CDN for one call and waits for ze's ZLB that
// acknowledges it. Ze removes the call before it acknowledges, so the next
// call's draw is made without it, and the peer never has more than one
// unacknowledged message in ze's receive window.
func selectorHangUp(peer *tunnelAccountingPeer, zeSID, lacSID uint16) error {
	peer.sendControl(tunnelL2TPHangUpCDN(lacSID), zeSID)
	want := peer.ns
	deadline := time.Now().Add(selectorControlWait)
	for time.Now().Before(deadline) {
		packet, err := peer.readPacket(sessionClearDelay)
		if err != nil {
			continue
		}
		if selectorCarriesBody(packet) {
			if seq := binary.BigEndian.Uint16(packet[8:10]); seq == peer.nr {
				peer.nr = seq + 1
			}
		}
		if selectorAcknowledges(packet, want) {
			return nil
		}
	}
	return errors.New("ze never acknowledged the LAC's CDN for session " + sessionIDText(zeSID) +
		" on tunnel " + sessionIDText(peer.localTID))
}

// tunnelL2TPHangUpCDN is the body of the LAC's Call-Disconnect-Notify for the
// call it assigned lacSID, Result Code 3, administrative.
func tunnelL2TPHangUpCDN(lacSID uint16) []byte {
	// RFC 2661 Section 6.12: "The following AVPs MUST be present in the
	// CDN:" Message Type, Result Code and Assigned Session ID.
	result := append(tunnelL2TPU16(slowAccountingCDNAdministrative), tunnelL2TPU16(0)...)
	cdn := append(tunnelL2TPAVP(true, 0, tunnelL2TPU16(tunnelL2TPMessageTypeCDN)), tunnelL2TPAVP(true, 1, result)...)
	return append(cdn, tunnelL2TPAVP(true, tunnelL2TPAVPAssignedSessionID, tunnelL2TPU16(lacSID))...)
}

// selectorCarriesBody answers whether datagram is a control message with a
// body, the kind that consumes a sequence number and is owed an
// acknowledgment. A ZLB carries the header alone.
func selectorCarriesBody(datagram []byte) bool {
	if !selectorControl(datagram) {
		return false
	}
	return binary.BigEndian.Uint16(datagram[2:4]) > 12
}

// selectorAcknowledges answers whether datagram is a control message whose Nr
// is want, which acknowledges every message the peer sent below want.
func selectorAcknowledges(datagram []byte, want uint16) bool {
	if !selectorControl(datagram) {
		return false
	}
	return binary.BigEndian.Uint16(datagram[10:12]) == want
}

// selectorControl answers whether datagram is at least a control header: the
// T bit set, and the twelve octets that carry Length, IDs, Ns and Nr.
//
//	 0                   1                   2                   3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|T|L|x|x|S|x|O|P|x|x|x|x|  Ver  |          Length               |  0-3
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|           Tunnel ID           |           Session ID          |  4-7
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//	|             Ns                |             Nr                |  8-11
//	+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
//
// RFC 2661 Section 3.1: "If the Length (L) bit is 1, the Length field is
// present. This bit MUST be set to 1 for control messages." and "The S bit
// MUST be set to 1 for control messages." So a control datagram of twelve
// octets or more holds every field read here.
func selectorControl(datagram []byte) bool {
	if len(datagram) < 12 {
		return false
	}
	return binary.BigEndian.Uint16(datagram[0:2])&0x8000 != 0
}

// selectorRow is one row of `show l2tp sessions | json`: the coordinates the
// qualified clear types, and the address that ties the row to its RADIUS
// accounting.
type selectorRow struct {
	sid      uint16
	tid      uint16
	listener string
	birth    string
	address  string
}

// selectorRows answers every session row of a `show l2tp sessions | json`
// answer. A row without a numeric session or tunnel ID, a birth or a listener
// is refused: the qualified clear could not name it.
func selectorRows(answer any) ([]selectorRow, error) {
	objects := jsonObjectsWith(answer, "local-sid")
	rows := make([]selectorRow, 0, len(objects))
	for _, object := range objects {
		sid, sidOK := selectorID(object["local-sid"])
		if !sidOK {
			return nil, errors.New("a session row whose local-sid is not a session ID: " + describeJSON(object))
		}
		tid, tidOK := selectorID(object["tunnel-local-tid"])
		if !tidOK {
			return nil, errors.New("a session row whose tunnel-local-tid is not a tunnel ID: " + describeJSON(object))
		}
		birth, _ := object["birth"].(string)
		if birth == "" {
			return nil, errors.New("a session row with no birth, which expected-id needs: " + describeJSON(object))
		}
		listener, _ := object["listener"].(string)
		if listener == "" {
			return nil, errors.New("a session row with no listener, which the qualified clear needs: " + describeJSON(object))
		}
		address, _ := object["assigned-addr"].(string)
		rows = append(rows, selectorRow{sid: sid, tid: tid, listener: listener, birth: birth, address: address})
	}
	return rows, nil
}

// selectorID answers a JSON number that is a non-zero 16-bit ID.
func selectorID(value any) (uint16, bool) {
	number, isNumber := value.(float64)
	if !isNumber {
		return 0, false
	}
	if number < 1 {
		return 0, false
	}
	if number > 65535 {
		return 0, false
	}
	if number != float64(uint16(number)) {
		return 0, false
	}
	return uint16(number), true
}

// selectorFind answers the one row with sid on tunnel tid.
func selectorFind(rows []selectorRow, sid, tid uint16) (selectorRow, error) {
	var found []selectorRow
	for _, row := range rows {
		if row.sid != sid {
			continue
		}
		if row.tid != tid {
			continue
		}
		found = append(found, row)
	}
	if len(found) != 1 {
		return selectorRow{}, fmt.Errorf("%d rows hold session %d on tunnel %d, want 1", len(found), sid, tid)
	}
	return found[0], nil
}

// selectorSameRows refuses got unless it holds exactly the rows of want, in
// any order. The birth is compared with every coordinate, so a replacement
// that took a survivor's numbers is not the survivor.
func selectorSameRows(got, want []selectorRow) error {
	if len(got) != len(want) {
		return fmt.Errorf("%d session rows, want %d", len(got), len(want))
	}
	for _, row := range want {
		if !slices.Contains(got, row) {
			return fmt.Errorf("no row is session %d on tunnel %d with birth %s and address %s",
				row.sid, row.tid, row.birth, row.address)
		}
	}
	return nil
}

// selectorCandidate is the qualified clear of a row, in the words ze's
// ambiguity refusal lists it (sessionCandidateCommand,
// internal/component/l2tp/cmd/session_read.go) and the operator types it.
func selectorCandidate(row selectorRow) string {
	return "clear l2tp session id " + sessionIDText(row.sid) + " tunnel-id " + sessionIDText(row.tid) +
		" listener " + row.listener + " expected-id " + row.birth
}

// selectorBareClear is the clear by session ID alone.
func selectorBareClear(sid uint16) string {
	return "clear l2tp session id " + sessionIDText(sid)
}

// selectorJudgeAmbiguity refuses a refusal that does not say the bare ID is
// ambiguous, or that does not list the qualified clear of every row sharing
// it (sessionTeardownAmbiguity, internal/component/l2tp/cmd/session_teardown.go).
func selectorJudgeAmbiguity(output string, sid uint16, rows []selectorRow) error {
	problem := "L2TP session ID " + sessionIDText(sid) + " is ambiguous"
	if !strings.Contains(output, problem) {
		return fmt.Errorf("the refusal does not say %q: %q", problem, awaitAnswerTail(output))
	}
	for _, row := range rows {
		if !strings.Contains(output, selectorCandidate(row)) {
			return fmt.Errorf("the refusal does not name the qualified clear %q: %q", selectorCandidate(row), awaitAnswerTail(output))
		}
	}
	return nil
}

// selectorJudgeStale refuses a refusal that does not say the session was not
// cleared because no session matches the selector (sessionTeardownNotFound):
// a Birth that does not match the row at those coordinates selects nothing.
func selectorJudgeStale(output string, sid uint16) error {
	problem := "L2TP session " + sessionIDText(sid) + " was not cleared"
	if !strings.Contains(output, problem) {
		return fmt.Errorf("the refusal does not say %q: %q", problem, awaitAnswerTail(output))
	}
	const why = "No L2TP session matches this selector"
	if !strings.Contains(output, why) {
		return fmt.Errorf("the refusal does not say %q: %q", why, awaitAnswerTail(output))
	}
	return nil
}

// selectorTeardownAnswered answers whether a `clear l2tp session ... | json`
// answer is the confirmed teardown of session sid.
func selectorTeardownAnswered(answer any, sid uint16) bool {
	for _, row := range jsonObjectsWith(answer, "status") {
		if row["action"] != "session-teardown" {
			continue
		}
		if row["session-id"] != float64(sid) {
			continue
		}
		if row["status"] == "sent" {
			return true
		}
	}
	return false
}

// selectorStarts answers, for each Framed-IP-Address, the Acct-Session-Id of
// its Accounting-Start. Each session of this run has its own address, so the
// address is what ties a row of `show l2tp sessions` to its accounting. A
// Start without either attribute, and a second Start for one address or one
// Acct-Session-Id, is refused.
func selectorStarts(records []accountingRecord) (map[string]string, error) {
	starts := make(map[string]string, len(records))
	ids := make(map[string]bool, len(records))
	for _, record := range records {
		if record.status != accountingStatusStart {
			continue
		}
		address, _ := tunnelRadiusValue(record.line, "Framed-IP-Address")
		if address == "" {
			return nil, errors.New("an Accounting-Start without Framed-IP-Address: " + record.line)
		}
		if record.sessionID == "" {
			// RFC 2866 Section 5.5: "The start and stop records for a given
			// session MUST have the same Acct-Session-Id."
			return nil, errors.New("an Accounting-Start without Acct-Session-Id: " + record.line)
		}
		if _, seen := starts[address]; seen {
			return nil, errors.New("a second Accounting-Start for address " + address + ": " + record.line)
		}
		if ids[record.sessionID] {
			return nil, errors.New("a second Accounting-Start for Acct-Session-Id " + record.sessionID + ": " + record.line)
		}
		starts[address] = record.sessionID
		ids[record.sessionID] = true
	}
	return starts, nil
}

// selectorJudgeAccounting refuses the records unless every one belongs to a
// started session, each ended address's session holds exactly one Stop with
// Acct-Terminate-Cause 6 (Admin Reset), and each live one holds none.
func selectorJudgeAccounting(records []accountingRecord, starts map[string]string, ended map[string]bool) error {
	owners := make(map[string]string, len(starts))
	for address, id := range starts {
		owners[id] = address
	}
	stops := make(map[string]int, len(starts))
	for _, record := range records {
		address, started := owners[record.sessionID]
		if !started {
			return errors.New("a record of an Acct-Session-Id no Start of this run carries: " + record.line)
		}
		if record.status != accountingStatusStop {
			continue
		}
		if record.cause != tunnelTerminateCauseAdminReset {
			return fmt.Errorf("the Stop for %s reports Acct-Terminate-Cause %q, not %s (Admin Reset): %s",
				address, record.cause, tunnelTerminateCauseAdminReset, record.line)
		}
		stops[address]++
	}
	for address := range starts {
		want := 0
		if ended[address] {
			want = 1
		}
		if stops[address] != want {
			return fmt.Errorf("the RADIUS server holds %d Accounting-Stop records for the session at %s, want %d",
				stops[address], address, want)
		}
	}
	return nil
}

// selectorScope is the state the five clears share: the links of the sessions
// still live, each session's Acct-Session-Id by address, the addresses whose
// session has ended, and the three rows as first listed.
type selectorScope struct {
	links  []*sessionClearLink
	starts map[string]string
	ended  map[string]bool
	unique selectorRow
	first  selectorRow
	second selectorRow
}

// awaitAdmitted waits until `show l2tp sessions` lists the three sessions with
// their addresses, and the RADIUS server holds an Accounting-Start for each.
// These are the positive controls: the same command and the same log are read
// after every clear.
func (scope *selectorScope) awaitAdmitted(ctx context.Context, unique, first, second *tunnelAccountingPeer) error {
	err := scope.await(ctx, "three sessions with addresses in `"+sessionClearShowSessions+"`", func() error {
		answer, err := sessionClearCLI(ctx, sessionClearShowSessions)
		if err != nil {
			return err
		}
		rows, err := selectorRows(answer)
		if err != nil {
			return err
		}
		if len(rows) != 3 {
			return errors.New("an answer without exactly three sessions: " + describeJSON(answer))
		}
		for _, row := range rows {
			if row.address == "" {
				return errors.New("a session without its address yet: " + describeJSON(answer))
			}
		}
		if scope.unique, err = selectorFind(rows, unique.zeSID, unique.localTID); err != nil {
			return err
		}
		if scope.first, err = selectorFind(rows, first.zeSID, first.localTID); err != nil {
			return err
		}
		scope.second, err = selectorFind(rows, second.zeSID, second.localTID)
		return err
	})
	if err != nil {
		return err
	}
	if scope.first.birth == scope.second.birth {
		return errors.New("the two sessions sharing session ID " + sessionIDText(scope.first.sid) + " carry one birth " + scope.first.birth)
	}
	fmt.Println("OK: show l2tp sessions lists session " + sessionIDText(scope.first.sid) + " on tunnels " +
		sessionIDText(scope.first.tid) + " and " + sessionIDText(scope.second.tid) + " with two births, and session " +
		sessionIDText(scope.unique.sid) + " alone")
	err = scope.await(ctx, "an Accounting-Start for each of the three sessions", func() error {
		records, err := readAccountingRecords()
		if err != nil {
			return err
		}
		starts, err := selectorStarts(records)
		if err != nil {
			return err
		}
		for _, row := range []selectorRow{scope.unique, scope.first, scope.second} {
			if _, started := starts[row.address]; !started {
				return errors.New("no Accounting-Start for " + row.address)
			}
		}
		scope.starts = starts
		return nil
	})
	if err != nil {
		return err
	}
	fmt.Println("OK: the RADIUS server holds an Accounting-Start for each of the three sessions")
	return nil
}

// run types the five clears in order. Each judges the command's answer, then
// the state the answer left: the rows, the survivors' links, and accounting.
func (scope *selectorScope) run(ctx context.Context, uniqueLink, firstLink, secondLink *sessionClearLink) error {
	shared := scope.first.sid
	live := []selectorRow{scope.first, scope.second}

	if err := scope.clearOne(ctx, uniqueLink, selectorBareClear(scope.unique.sid), scope.unique, live); err != nil {
		return err
	}
	fmt.Println("OK: the bare unique session ID " + sessionIDText(scope.unique.sid) + " cleared only its session")

	output, err := selectorRefusedCLI(ctx, selectorBareClear(shared))
	if err != nil {
		return err
	}
	if err := selectorJudgeAmbiguity(output, shared, live); err != nil {
		return err
	}
	if err := scope.check(ctx, live); err != nil {
		return err
	}
	fmt.Println("OK: the bare shared session ID " + sessionIDText(shared) +
		" was refused as ambiguous, naming both qualified clears, and both sessions remain")

	live = []selectorRow{scope.first}
	if err := scope.clearOne(ctx, secondLink, selectorCandidate(scope.second), scope.second, live); err != nil {
		return err
	}
	fmt.Println("OK: the qualified clear ended only session " + sessionIDText(shared) + " on tunnel " + sessionIDText(scope.second.tid))

	stale := scope.first
	stale.birth = scope.second.birth
	output, err = selectorRefusedCLI(ctx, selectorCandidate(stale))
	if err != nil {
		return err
	}
	if err := selectorJudgeStale(output, shared); err != nil {
		return err
	}
	if err := scope.check(ctx, live); err != nil {
		return err
	}
	fmt.Println("OK: the ended session's expected-id was refused at the live session's coordinates, which remains")

	if err := scope.clearOne(ctx, firstLink, selectorBareClear(shared), scope.first, nil); err != nil {
		return err
	}
	fmt.Println("OK: the bare session ID " + sessionIDText(shared) + ", unique once its sibling ended, cleared the last session")
	fmt.Println("OK: the RADIUS server holds one Stop with Acct-Terminate-Cause 6 for each cleared session and none for a live one")
	return nil
}

// clearOne types command, which MUST clear target, and checks what it left:
// the target's CDN on its own link, then live as the whole session list.
func (scope *selectorScope) clearOne(ctx context.Context, link *sessionClearLink, command string, target selectorRow, live []selectorRow) error {
	link.clearing.Store(true)
	scope.links = slices.DeleteFunc(scope.links, func(member *sessionClearLink) bool { return member == link })
	answer, err := sessionClearCLI(ctx, command+" | json")
	if err != nil {
		return err
	}
	if !selectorTeardownAnswered(answer, target.sid) {
		return fmt.Errorf("`ze cli -c %s` answered %s, not a session-teardown of session %d with status sent", command, describeJSON(answer), target.sid)
	}
	fmt.Println("OK: " + command + " answered status sent for session " + sessionIDText(target.sid))
	scope.ended[target.address] = true
	if err := link.awaitCDN(ctx, target.sid); err != nil {
		return err
	}
	return scope.check(ctx, live)
}

// check reads, once, the state a command left: no survivor's link broken (ze
// sends a survivor nothing a link refuses: a CDN, a StopCCN, an LCP
// Terminate), `show l2tp sessions` listing exactly live, and the accounting
// selectorJudgeAccounting requires of the sessions ended so far.
func (scope *selectorScope) check(ctx context.Context, live []selectorRow) error {
	for _, link := range scope.links {
		if err := link.failure(); err != nil {
			return err
		}
	}
	answer, err := sessionClearCLI(ctx, sessionClearShowSessions)
	if err != nil {
		return err
	}
	rows, err := selectorRows(answer)
	if err != nil {
		return err
	}
	if err := selectorSameRows(rows, live); err != nil {
		return fmt.Errorf("`ze cli -c %s`: %w: %s", sessionClearShowSessions, err, describeJSON(answer))
	}
	records, err := readAccountingRecords()
	if err != nil {
		return err
	}
	return selectorJudgeAccounting(records, scope.starts, scope.ended)
}

// await polls probe every sessionClearDelay within selectorWaitShare of the
// budget, as sessionClearLink.await does for one link, and ends early with the
// error of any link that broke.
func (scope *selectorScope) await(ctx context.Context, what string, probe func() error) error {
	var broken error
	last := errors.New("no attempt finished")
	attempts := WaitAttempts(selectorWaitShare, sessionClearDelay, 120)
	held := Poll(ctx, attempts, sessionClearDelay, func() bool {
		for _, link := range scope.links {
			if err := link.failure(); err != nil {
				broken = err
				return true
			}
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

// selectorRefusedCLI runs one `ze cli -c` command that MUST be refused, and
// answers what it printed on both streams for the caller to judge. A command
// that exits zero was accepted, which is the failure.
func selectorRefusedCLI(ctx context.Context, command string) (string, error) {
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", command)
	if err != nil {
		return "", fmt.Errorf("`ze cli -c %s` did not start: %w", command, err)
	}
	if code == 0 {
		return "", fmt.Errorf("`ze cli -c %s` was accepted, and it MUST be refused; stdout: %q", command, awaitAnswerTail(stdout))
	}
	return stdout + "\n" + stderr, nil
}
