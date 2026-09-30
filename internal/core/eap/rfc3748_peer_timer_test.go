// Design: docs/architecture/ike/ipsec-9-ikev2-eap-nat.md -- EAP framework (RFC 3748)
// RFC: rfc/short/rfc3748.md -- Section 4.1: the peer's Response is never retransmitted on a timer
// Related: rfc3748_remaining_clause_test.go -- the "only in reply to a valid Request" clause
//
// VALIDATES: the EAP peer holds no timer and no send path besides Process, and
// a packet that is not a valid Request draws nothing, not even a copy of the
// Response the peer already sent.
// PREVENTS: a peer that retransmits its last Response on its own schedule, or
// that resends it when anything other than a valid Request arrives.

package eap

import (
	"reflect"
	"testing"
	"time"
)

// TestRFC3748PeerNeverRetransmitsOnATimer states the "never retransmitted on a
// timer" clause as properties of the peer the test can read, rather than as an
// absence nobody checks.
//
// Method: the PeerSession type is walked for a time.Timer, a time.Ticker or a
// channel of time.Time, which a retransmission timer needs; the exported method
// set is read for any send path other than Process; then a peer that has
// answered one Request is handed a packet that is not a Request and must answer
// nothing.
//
// RFC requirement: RFC3748-4.1-1 positive -- PeerSession holds no timer or
// timer channel, Process is its only method that returns a Response, and after
// answering a Request the peer answers an EAP-Success it cannot accept with no
// Response at all, so its Response is never sent again without a new Request.
func TestRFC3748PeerNeverRetransmitsOnATimer(t *testing.T) {
	if path := timerField(reflect.TypeFor[PeerSession](), "PeerSession", 0); path != "" {
		t.Fatalf("the EAP peer holds a timer at %s, so it can send without a Request", path)
	}

	packetType := reflect.TypeFor[*Packet]()
	resultType := reflect.TypeFor[PeerResult]()
	for method := range reflect.TypeFor[*PeerSession]().Methods() {
		if method.Name == "Process" {
			continue
		}
		for result := range method.Type.Outs() {
			if result == packetType || result == resultType {
				t.Fatalf("PeerSession.%s returns %v, a send path besides Process", method.Name, result)
			}
		}
	}

	auth, err := NewSession(TypeMSCHAPv2, MethodConfig{Password: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	peer := NewPeerSession(TypeMSCHAPv2, "user", "secret")
	t.Cleanup(peer.Close)

	request := auth.Begin()
	answered := peer.Process(request)
	if answered.Response == nil {
		t.Fatalf("the peer answered the Request with no Response: %v", answered.Err)
	}

	early := &Packet{Code: CodeSuccess, Identifier: request.Identifier}
	again := peer.Process(early)
	if again.Response != nil {
		t.Fatalf("a packet that is not a Request drew Response %+v, a retransmission of %+v", again.Response, answered.Response)
	}
	if !again.Discarded {
		t.Fatalf("the EAP-Success before the method ended was not discarded: %+v", again)
	}
}

// timerField answers the path of the first field reachable from typ that can
// fire on its own schedule, or "" when none can. It descends only into types of
// this package, because a timer the peer owns has to sit in one of its own
// fields, and depth is bounded at eight levels of nesting.
func timerField(typ reflect.Type, path string, depth int) string {
	if depth > 8 {
		return ""
	}
	switch typ {
	case reflect.TypeFor[time.Timer](), reflect.TypeFor[time.Ticker]():
		return path
	}
	switch typ.Kind() { //nolint:exhaustive // only the kinds that can hold a nested type are walked
	case reflect.Pointer, reflect.Slice, reflect.Array:
		return timerField(typ.Elem(), path, depth+1)
	case reflect.Chan:
		if typ.Elem() == reflect.TypeFor[time.Time]() {
			return path
		}
		return timerField(typ.Elem(), path, depth+1)
	case reflect.Struct:
		if typ.PkgPath() != reflect.TypeFor[PeerSession]().PkgPath() {
			return ""
		}
		for field := range typ.Fields() {
			if found := timerField(field.Type, path+"."+field.Name, depth+1); found != "" {
				return found
			}
		}
	}
	return ""
}
