package bridge

import (
	"errors"
	"testing"
)

// TestNeighborTeardownCarriesTheCeaseSubcode holds the one ExaBGP neighbor
// lifecycle command ze performs.
//
// GOAL: `neighbor <ip> teardown <code>` reaches ze's own teardown with the
// subcode the script wrote, so api-teardown gets the Cease NOTIFICATION it
// asserts on the wire.
// METHOD: translate the line and compare the whole command string, because the
// subcode is a positional argument and a dropped one is invisible in a substring
// match.
//
// VALIDATES: the command is `request peer <selector> teardown <subcode>`, the
// selector travels with it, and no UPDATE is claimed.
// PREVENTS: a teardown translated to a bare session close, which sends a
// NOTIFICATION with the wrong subcode and passes every test that reads only the
// session state.
func TestNeighborTeardownCarriesTheCeaseSubcode(t *testing.T) {
	cases := []struct {
		name     string
		selector string
		rest     string
		want     string
	}{
		{"api-teardown", "127.0.0.1", "teardown 4", "request peer 127.0.0.1 teardown 4"},
		{"unspecific", "10.0.0.1", "teardown 0", "request peer 10.0.0.1 teardown 0"},
		{"widest subcode", "10.0.0.1", "teardown 255", "request peer 10.0.0.1 teardown 255"},
		{"every peer", bridgeEveryPeer, "teardown 6", "request peer * teardown 6"},
		{"upper case verb", "10.0.0.1", "TEARDOWN 2", "request peer 10.0.0.1 teardown 2"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, ok, err := ConvertNeighborControl(tc.selector, tc.rest)
			if !ok {
				t.Fatalf("teardown is a neighbor lifecycle command, got ok=false for %q", tc.rest)
			}
			if err != nil {
				t.Fatalf("ze performs this teardown, got error: %v", err)
			}
			if onlyCommand(translation) != tc.want {
				t.Errorf("command = %q, want %q", onlyCommand(translation), tc.want)
			}
			if translation.Selector != tc.selector {
				t.Errorf("selector = %q, want %q", translation.Selector, tc.selector)
			}
			if translation.Route {
				t.Error("a teardown puts no UPDATE on a wire, so no flush is owed")
			}
			if translation.Local != LocalNone {
				t.Error("a teardown is dispatched, not answered by the bridge")
			}
		})
	}
}

// TestNeighborTeardownRefusesASubcodeTheWireCannotCarry pins the boundary the
// NOTIFICATION field sets.
//
// GOAL: a subcode ze cannot put in one octet is refused where the script wrote
// it, rather than sent on to a handler as text.
// METHOD: drive each malformed form and require the named error, so the refusal
// is one a caller can recognize instead of a message it can only print.
//
// VALIDATES: a non-numeric, an out-of-range and a missing subcode each answer
// errTeardownSubcode, and none of them produces a command.
// PREVENTS: `teardown` translated with no subcode at all, which ze's handler
// would read as a usage error long after the bridge could name the line.
func TestNeighborTeardownRefusesASubcodeTheWireCannotCarry(t *testing.T) {
	for _, rest := range []string{"teardown", "teardown abc", "teardown 256", "teardown -1", "teardown 4 now"} {
		t.Run(rest, func(t *testing.T) {
			translation, ok, err := ConvertNeighborControl("127.0.0.1", rest)
			if !ok {
				t.Fatalf("the line names teardown, so it is ours to refuse, got ok=false")
			}
			if !errors.Is(err, errTeardownSubcode) {
				t.Fatalf("error = %v, want errTeardownSubcode", err)
			}
			if onlyCommand(translation) != "" {
				t.Errorf("a refused line carries no command, got %q", onlyCommand(translation))
			}
		})
	}
}

// TestNeighborCreateIsRefusedByName holds the answer for a command ze has no
// way to perform.
//
// GOAL: the bridge says ze cannot create a peer at runtime, rather than mapping
// the line to a command that does something else.
// METHOD: translate the api-peer-lifecycle line and require the named error.
//
// VALIDATES: `create neighbor ...` answers errNeighborCreate and no command.
// PREVENTS: an approximate translation, and a silent success that acks the
// script for a session that was never created (ai/rules/principles.md).
func TestNeighborCreateIsRefusedByName(t *testing.T) {
	const line = "create neighbor 127.0.0.1 local-address 127.0.0.1 local-as 1 peer-as 1 router-id 1.2.3.4 api peer-lifecycle"

	translation, ok, err := ConvertNeighborControl(bridgeEveryPeer, line)
	if !ok {
		t.Fatal("create names a neighbor lifecycle command, so the bridge owns the refusal")
	}
	if !errors.Is(err, errNeighborCreate) {
		t.Fatalf("error = %v, want errNeighborCreate", err)
	}
	if onlyCommand(translation) != "" {
		t.Errorf("a refused line carries no command, got %q", onlyCommand(translation))
	}
	if translation.Local != LocalNone {
		t.Error("a refusal is not a local action the bridge answers")
	}
}

// TestNeighborReceiveIsRefusedAsAnEvent keeps the two directions apart.
//
// GOAL: `neighbor <ip> receive update ...` is named as an event ze SENDS, so a
// reader of the refusal looks at the event encoder rather than the translator.
// METHOD: translate the exact line api-check reads on its stdin.
//
// VALIDATES: the line answers errNeighborReceive and no command.
// PREVENTS: the line being read as a command a script sends down, which is the
// misdiagnosis that sends the repair to the wrong half of the bridge.
func TestNeighborReceiveIsRefusedAsAnEvent(t *testing.T) {
	const rest = "receive update announced 0.0.0.0/32 next-hop 127.0.0.1 origin igp local-preference 100"

	translation, ok, err := ConvertNeighborControl("127.0.0.1", rest)
	if !ok {
		t.Fatal("receive names the event vocabulary, so the bridge owns the refusal")
	}
	if !errors.Is(err, errNeighborReceive) {
		t.Fatalf("error = %v, want errNeighborReceive", err)
	}
	if onlyCommand(translation) != "" {
		t.Errorf("a refused line carries no command, got %q", onlyCommand(translation))
	}
}

// TestConvertNeighborControlLeavesEveryOtherLine keeps the entry point from
// claiming lines another converter reads.
//
// GOAL: a route, a control command and the ack words fall through untouched.
// METHOD: drive one line from each of the other converters and require ok=false
// with no error, which is what tells the caller to keep looking.
//
// VALIDATES: ok is false and err is nil for a line that is not a neighbor
// lifecycle command.
// PREVENTS: a `create interface` or an `announce route` swallowed by this
// converter, which would refuse a line the bridge translates today.
func TestConvertNeighborControlLeavesEveryOtherLine(t *testing.T) {
	for _, rest := range []string{
		"",
		"announce route 1.2.3.4/32 next-hop 5.6.7.8",
		"withdraw route 1.2.3.4/32",
		"flush adj-rib out",
		"clear adj-rib in",
		"enable-ack",
		"shutdown",
		"help",
		"create interface dummy name dummy0",
		"receiver update",
	} {
		t.Run(rest, func(t *testing.T) {
			translation, ok, err := ConvertNeighborControl("127.0.0.1", rest)
			if ok {
				t.Errorf("this converter claimed %q, which belongs to another one", rest)
			}
			if err != nil {
				t.Errorf("a line this converter does not read is not an error here, got %v", err)
			}
			if onlyCommand(translation) != "" {
				t.Errorf("an unclaimed line carries no command, got %q", onlyCommand(translation))
			}
		})
	}
}
