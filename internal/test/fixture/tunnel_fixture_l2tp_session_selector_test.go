// Design: docs/functional-tests.md -- fixture decisions have unit controls.
// Related: tunnel_fixture_l2tp_session_selector.go -- the decisions under test;
// the daemon path is test/l2tp/session-selector-scope.ci.
package fixture

import (
	"encoding/binary"
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

// TestSelectorDrawsCollision proves the search's decision: a draw collides
// only with a held ID, never with the unique session's excluded ID, and every
// held call but the kept one is hung up, in ascending order.
func TestSelectorDrawsCollision(t *testing.T) {
	draws := newSelectorDraws(7)
	for _, call := range []selectorCall{{zeSID: 30, lacSID: 1000}, {zeSID: 7, lacSID: 1001}, {zeSID: 12, lacSID: 1002}} {
		if err := draws.hold(call.zeSID, call.lacSID); err != nil {
			t.Fatalf("hold %d: %v", call.zeSID, err)
		}
	}
	if !draws.collides(12) {
		t.Fatal("a draw equal to a held ID must collide")
	}
	if draws.collides(7) {
		t.Fatal("a draw equal to the excluded ID must not collide, even when held")
	}
	if draws.collides(13) {
		t.Fatal("a draw no held call has must not collide")
	}
	got := draws.hangUps(12)
	want := []selectorCall{{zeSID: 7, lacSID: 1001}, {zeSID: 30, lacSID: 1000}}
	if len(got) != len(want) {
		t.Fatalf("hangUps(12) = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("hangUps(12) = %v, want %v", got, want)
		}
	}
}

// TestSelectorDrawsRefuseImpossibleIDs proves hold refuses what ze's
// allocator must never answer: the reserved zero, and an ID one tunnel
// already holds.
func TestSelectorDrawsRefuseImpossibleIDs(t *testing.T) {
	draws := newSelectorDraws(1)
	if err := draws.hold(0, 1000); err == nil {
		t.Fatal("session ID 0 must be refused")
	}
	if err := draws.hold(5, 1000); err != nil {
		t.Fatalf("hold 5: %v", err)
	}
	if err := draws.hold(5, 1001); err == nil {
		t.Fatal("a second live call with one session ID must be refused")
	}
}

// TestSelectorAcknowledges proves the CDN wait reads ze's ZLB by its Nr, and
// tells a control message with a body from a ZLB and from a data message.
func TestSelectorAcknowledges(t *testing.T) {
	zlb := tunnelL2TPControl(4, 0, 9, 21, nil)
	if !selectorAcknowledges(zlb, 21) {
		t.Fatal("a ZLB with Nr 21 must acknowledge 21")
	}
	if selectorAcknowledges(zlb, 22) {
		t.Fatal("a ZLB with Nr 21 must not acknowledge 22")
	}
	if selectorCarriesBody(zlb) {
		t.Fatal("a ZLB carries no body")
	}
	hello := tunnelL2TPControl(4, 0, 9, 21, tunnelL2TPAVP(true, 0, tunnelL2TPU16(6)))
	if !selectorCarriesBody(hello) {
		t.Fatal("a HELLO carries a body")
	}
	data := make([]byte, 12)
	binary.BigEndian.PutUint16(data[0:2], 0x0002)
	binary.BigEndian.PutUint16(data[10:12], 21)
	if selectorAcknowledges(data, 21) {
		t.Fatal("a data message acknowledges nothing")
	}
	if selectorAcknowledges(zlb[:11], 21) {
		t.Fatal("a datagram shorter than a control header acknowledges nothing")
	}
}

// TestTunnelL2TPHangUpCDN proves the LAC's CDN carries the three AVPs RFC
// 2661 Section 6.12 requires, naming the LAC's own session.
func TestTunnelL2TPHangUpCDN(t *testing.T) {
	avps, err := tunnelL2TPParseAVPs(tunnelL2TPControl(4, 30, 1, 1, tunnelL2TPHangUpCDN(1042)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if tunnelL2TPMessageType(avps) != tunnelL2TPMessageTypeCDN {
		t.Fatalf("message type %d, want CDN", tunnelL2TPMessageType(avps))
	}
	if len(avps[1]) < 2 {
		t.Fatalf("result code AVP %x is shorter than a Result Code", avps[1])
	}
	if binary.BigEndian.Uint16(avps[1][:2]) != slowAccountingCDNAdministrative {
		t.Fatalf("result code AVP %x, want 3", avps[1])
	}
	assigned := avps[tunnelL2TPAVPAssignedSessionID]
	if len(assigned) != 2 {
		t.Fatalf("assigned session ID AVP %x is not two octets", assigned)
	}
	if binary.BigEndian.Uint16(assigned) != 1042 {
		t.Fatalf("assigned session ID AVP %x, want 1042", assigned)
	}
}

// selectorTestAnswer decodes a `show l2tp sessions | json` answer as the CLI
// prints it.
func selectorTestAnswer(t *testing.T, text string) any {
	t.Helper()
	var answer any
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	return answer
}

// TestSelectorRows proves the rows carry every coordinate the qualified clear
// types, and that a row missing one is refused rather than skipped.
func TestSelectorRows(t *testing.T) {
	answer := selectorTestAnswer(t, `{"sessions":[`+
		`{"local-sid":4660,"tunnel-local-tid":2,"listener":"127.0.0.1:1701","birth":"l2tp-a","assigned-addr":"10.99.8.11"},`+
		`{"local-sid":4660,"tunnel-local-tid":3,"listener":"127.0.0.1:1701","birth":"l2tp-b","assigned-addr":""}]}`)
	rows, err := selectorRows(answer)
	if err != nil {
		t.Fatalf("selectorRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("%d rows, want 2", len(rows))
	}
	row, err := selectorFind(rows, 4660, 3)
	if err != nil {
		t.Fatalf("selectorFind: %v", err)
	}
	want := selectorRow{sid: 4660, tid: 3, listener: "127.0.0.1:1701", birth: "l2tp-b"}
	if row != want {
		t.Fatalf("row %+v, want %+v", row, want)
	}
	if _, err := selectorFind(rows, 4660, 9); err == nil {
		t.Fatal("a tunnel holding no such session must be refused")
	}
	for _, broken := range []string{
		`[{"local-sid":4660,"tunnel-local-tid":2,"listener":"127.0.0.1:1701"}]`,
		`[{"local-sid":4660,"tunnel-local-tid":2,"birth":"l2tp-a"}]`,
		`[{"local-sid":0,"tunnel-local-tid":2,"listener":"127.0.0.1:1701","birth":"l2tp-a"}]`,
		`[{"local-sid":70000,"tunnel-local-tid":2,"listener":"127.0.0.1:1701","birth":"l2tp-a"}]`,
		`[{"local-sid":4660,"tunnel-local-tid":"2","listener":"127.0.0.1:1701","birth":"l2tp-a"}]`,
	} {
		if _, err := selectorRows(selectorTestAnswer(t, broken)); err == nil {
			t.Fatalf("selectorRows accepted %s", broken)
		}
	}
}

// TestSelectorSameRows proves a survivor is matched by its birth as well as
// its numbers: a row with the same coordinates and another birth is not it.
func TestSelectorSameRows(t *testing.T) {
	first := selectorRow{sid: 9, tid: 2, listener: "127.0.0.1:1701", birth: "l2tp-a", address: "10.99.8.11"}
	second := selectorRow{sid: 9, tid: 3, listener: "127.0.0.1:1701", birth: "l2tp-b", address: "10.99.8.12"}
	if err := selectorSameRows([]selectorRow{second, first}, []selectorRow{first, second}); err != nil {
		t.Fatalf("same rows in another order: %v", err)
	}
	replacement := first
	replacement.birth = "l2tp-c"
	if err := selectorSameRows([]selectorRow{replacement}, []selectorRow{first}); err == nil {
		t.Fatal("a replacement with the survivor's numbers must not pass as the survivor")
	}
	if err := selectorSameRows([]selectorRow{first, second}, []selectorRow{first}); err == nil {
		t.Fatal("an extra row must be refused")
	}
	if err := selectorSameRows(nil, nil); err != nil {
		t.Fatalf("no rows when none are live: %v", err)
	}
}

// TestSelectorJudgeAmbiguity proves the ambiguity refusal is judged on the
// problem line and on every qualified clear, in the words
// sessionCandidateCommand (internal/component/l2tp/cmd/session_read.go) writes.
func TestSelectorJudgeAmbiguity(t *testing.T) {
	first := selectorRow{sid: 9, tid: 2, listener: "127.0.0.1:1701", birth: "l2tp-a"}
	second := selectorRow{sid: 9, tid: 3, listener: "127.0.0.1:1701", birth: "l2tp-b"}
	if got, want := selectorCandidate(first), "clear l2tp session id 9 tunnel-id 2 listener 127.0.0.1:1701 expected-id l2tp-a"; got != want {
		t.Fatalf("candidate %q, want %q", got, want)
	}
	refusal := "L2TP session ID 9 is ambiguous\nQualify the current original with one of: " +
		selectorCandidate(first) + "; " + selectorCandidate(second) + "\nfix: show l2tp sessions\n"
	if err := selectorJudgeAmbiguity(refusal, 9, []selectorRow{first, second}); err != nil {
		t.Fatalf("a complete refusal: %v", err)
	}
	if err := selectorJudgeAmbiguity(strings.Replace(refusal, "tunnel-id 3", "tunnel-id 4", 1), 9, []selectorRow{first, second}); err == nil {
		t.Fatal("a refusal missing one qualified clear must be refused")
	}
	if err := selectorJudgeAmbiguity(strings.Replace(refusal, "is ambiguous", "was not cleared", 1), 9, []selectorRow{first, second}); err == nil {
		t.Fatal("a refusal that does not say ambiguous must be refused")
	}
}

// TestSelectorJudgeStale proves the stale refusal is the not-found answer,
// and that the ambiguity answer does not pass for it.
func TestSelectorJudgeStale(t *testing.T) {
	refusal := "L2TP session 9 was not cleared\nNo L2TP session matches this selector\nfix: show l2tp sessions\n"
	if err := selectorJudgeStale(refusal, 9); err != nil {
		t.Fatalf("the not-found refusal: %v", err)
	}
	if err := selectorJudgeStale("L2TP session ID 9 is ambiguous\n", 9); err == nil {
		t.Fatal("an ambiguity refusal must not pass for a stale one")
	}
	if err := selectorJudgeStale(strings.Replace(refusal, "session 9 ", "session 8 ", 1), 9); err == nil {
		t.Fatal("a refusal naming another session must be refused")
	}
}

// TestSelectorTeardownAnswered proves only the confirmed teardown of the named
// session counts as the clear's answer.
func TestSelectorTeardownAnswered(t *testing.T) {
	answer := selectorTestAnswer(t, `{"action":"session-teardown","session-id":9,"status":"sent"}`)
	if !selectorTeardownAnswered(answer, 9) {
		t.Fatal("the teardown of session 9 must count")
	}
	if selectorTeardownAnswered(answer, 8) {
		t.Fatal("the teardown of session 9 must not count for session 8")
	}
	other := selectorTestAnswer(t, `{"action":"session-teardown-all","session-id":9,"status":"sent"}`)
	if selectorTeardownAnswered(other, 9) {
		t.Fatal("another action must not count")
	}
}

// selectorTestRecord is one Accounting-Request line as tunnelRadiusDescribe
// writes it.
func selectorTestRecord(status, id, address, cause string) accountingRecord {
	line := "RADIUS-RX Accounting-Request Acct-Status-Type=" + status + " Acct-Session-Id=" + id
	if address != "" {
		line += " Framed-IP-Address=" + address
	}
	if cause != "" {
		line += " Acct-Terminate-Cause=" + cause
	}
	return accountingRecord{status: status, sessionID: id, cause: cause, line: line}
}

// TestSelectorAccounting proves each ended session owes exactly one Stop with
// Admin Reset, each live one none, and a record of no started session is
// refused.
func TestSelectorAccounting(t *testing.T) {
	records := []accountingRecord{
		selectorTestRecord(accountingStatusStart, "a1", "10.99.8.10", ""),
		selectorTestRecord(accountingStatusStart, "b2", "10.99.8.11", ""),
		selectorTestRecord(accountingStatusStop, "a1", "10.99.8.10", tunnelTerminateCauseAdminReset),
	}
	starts, err := selectorStarts(records)
	if err != nil {
		t.Fatalf("selectorStarts: %v", err)
	}
	if starts["10.99.8.10"] != "a1" {
		t.Fatalf("starts %v, want a1 at 10.99.8.10", starts)
	}
	if starts["10.99.8.11"] != "b2" {
		t.Fatalf("starts %v, want b2 at 10.99.8.11", starts)
	}
	if err := selectorJudgeAccounting(records, starts, map[string]bool{"10.99.8.10": true}); err != nil {
		t.Fatalf("one Stop for the ended session: %v", err)
	}
	if err := selectorJudgeAccounting(records, starts, map[string]bool{}); err == nil {
		t.Fatal("a Stop for a live session must be refused")
	}
	if err := selectorJudgeAccounting(records, starts, map[string]bool{"10.99.8.10": true, "10.99.8.11": true}); err == nil {
		t.Fatal("an ended session without its Stop must be refused")
	}
	twice := slices.Concat(records, []accountingRecord{selectorTestRecord(accountingStatusStop, "a1", "10.99.8.10", tunnelTerminateCauseAdminReset)})
	if err := selectorJudgeAccounting(twice, starts, map[string]bool{"10.99.8.10": true}); err == nil {
		t.Fatal("a second Stop must be refused")
	}
	stranger := slices.Concat(records, []accountingRecord{selectorTestRecord(accountingStatusStop, "z9", "", tunnelTerminateCauseAdminReset)})
	if err := selectorJudgeAccounting(stranger, starts, map[string]bool{"10.99.8.10": true}); err == nil {
		t.Fatal("a record of no started session must be refused")
	}
	lostCarrier := []accountingRecord{records[0], records[1], selectorTestRecord(accountingStatusStop, "a1", "10.99.8.10", tunnelTerminateCauseLostCarrier)}
	if err := selectorJudgeAccounting(lostCarrier, starts, map[string]bool{"10.99.8.10": true}); err == nil {
		t.Fatal("a Stop that is not Admin Reset must be refused")
	}
	if _, err := selectorStarts(slices.Concat(records, []accountingRecord{selectorTestRecord(accountingStatusStart, "c3", "10.99.8.10", "")})); err == nil {
		t.Fatal("a second Start for one address must be refused")
	}
	if _, err := selectorStarts([]accountingRecord{selectorTestRecord(accountingStatusStart, "a1", "", "")}); err == nil {
		t.Fatal("a Start without Framed-IP-Address must be refused")
	}
}
