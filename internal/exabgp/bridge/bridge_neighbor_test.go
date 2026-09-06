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

// TestNeighborCreateWritesTheZeCreateCommand holds the translation of the one
// line that brings a BGP peer into being while ze runs.
//
// GOAL: `create neighbor <ip> ...` reaches `create bgp peer <ip> asn <asn> ...`
// with every parameter the script wrote, so api-peer-lifecycle gets the session
// it announces its route on.
// METHOD: translate the api-peer-lifecycle line and compare the whole command
// string, because a dropped parameter is invisible in a substring match.
//
// VALIDATES: peer-as becomes asn, local-address and router-id travel, each `api`
// process becomes an `attach` name, and the created address is the selector.
// PREVENTS: a parameter accepted and dropped, which would build a session with
// the wrong AS or feed no plugin, and ack the script for it
// (ai/rules/principles.md).
func TestNeighborCreateWritesTheZeCreateCommand(t *testing.T) {
	cases := []struct {
		name string
		rest string
		want string
	}{
		{
			"api-peer-lifecycle",
			"create neighbor 127.0.0.1 local-address 127.0.0.1 local-as 1 peer-as 1 router-id 1.2.3.4 api peer-lifecycle",
			"create bgp peer 127.0.0.1 asn 1 local-as 1 local-address 127.0.0.1 router-id 1.2.3.4 accept false attach peer-lifecycle",
		},
		{
			"the smallest line ExaBGP accepts",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 65001 peer-as 65002",
			"create bgp peer 10.0.0.2 asn 65002 local-as 65001 local-address 10.0.0.1 accept false",
		},
		{
			"local-ip is the second spelling of local-address",
			"create neighbor 10.0.0.2 local-ip 10.0.0.1 local-as 65001 peer-as 65002",
			"create bgp peer 10.0.0.2 asn 65002 local-as 65001 local-address 10.0.0.1 accept false",
		},
		{
			"two api processes reach one attach",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 65001 peer-as 65002 api proc1 api proc2",
			"create bgp peer 10.0.0.2 asn 65002 local-as 65001 local-address 10.0.0.1 accept false attach proc1,proc2",
		},
		{
			"the family list changes separator and joiner",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 65001 peer-as 65002 family-allowed ipv4-unicast/ipv6-unicast",
			"create bgp peer 10.0.0.2 asn 65002 local-as 65001 local-address 10.0.0.1 accept false family ipv4/unicast,ipv6/unicast",
		},
		{
			"graceful-restart and group-updates travel",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 65001 peer-as 65002 graceful-restart 120 group-updates false",
			"create bgp peer 10.0.0.2 asn 65002 local-as 65001 local-address 10.0.0.1 accept false graceful-restart 120 group-updates false",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, ok, err := ConvertNeighborControl(bridgeEveryPeer, tc.rest)
			if !ok {
				t.Fatalf("create names a neighbor lifecycle command, got ok=false for %q", tc.rest)
			}
			if err != nil {
				t.Fatalf("error = %v, want a translation", err)
			}
			if got := onlyCommand(translation); got != tc.want {
				t.Errorf("command = %q, want %q", got, tc.want)
			}
			if translation.Route {
				t.Error("a create carries no route, so no flush is owed")
			}
		})
	}
}

// TestNeighborCreateSelectorIsTheCreatedPeer keeps the destination on the
// command the bridge wrote.
//
// GOAL: the line begins with `create`, so splitNeighborSelector reads no
// neighbor and hands the wildcard down. The address in the line is the peer.
// METHOD: translate with the wildcard selector and read Translation.Selector.
//
// VALIDATES: the selector is the created address rather than the wildcard.
// PREVENTS: a later flush or ack addressed to every peer instead of this one.
func TestNeighborCreateSelectorIsTheCreatedPeer(t *testing.T) {
	const line = "create neighbor 192.0.2.7 local-address 192.0.2.1 local-as 1 peer-as 2"

	translation, _, err := ConvertNeighborControl(bridgeEveryPeer, line)
	if err != nil {
		t.Fatalf("error = %v, want a translation", err)
	}
	if translation.Selector != "192.0.2.7" {
		t.Errorf("selector = %q, want the created peer 192.0.2.7", translation.Selector)
	}
}

// TestNeighborCreateRefusesWhatZeCannotHonour holds every parameter the bridge
// will not carry.
//
// GOAL: a parameter ze has no behavior for is refused BY NAME, rather than
// dropped from a command the script is then acked for.
// METHOD: translate one line per refusal and require the named error with no
// command.
//
// VALIDATES: `family-allowed in-open`, an unknown parameter, a duplicate, a
// value-less parameter, a missing required parameter and a missing address each
// answer their own error.
// PREVENTS: a peer built with fewer families, no plugin binding, or the wrong
// AS, which every test that reads only the session state would pass
// (ai/rules/principles.md).
func TestNeighborCreateRefusesWhatZeCannotHonour(t *testing.T) {
	cases := []struct {
		name string
		rest string
		want error
	}{
		{
			"in-open names no ze behavior",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 1 peer-as 2 family-allowed in-open",
			errNeighborFamilyInOpen,
		},
		{
			"a family that is not afi-safi",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 1 peer-as 2 family-allowed ipv4",
			errNeighborFamilyForm,
		},
		{
			"a parameter the bridge does not read",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 1 peer-as 2 md5 secret",
			errNeighborParameter,
		},
		{
			"a parameter with no value",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 1 peer-as",
			errNeighborParameter,
		},
		{
			"the two local-address spellings are one parameter",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-ip 10.0.0.3 local-as 1 peer-as 2",
			errNeighborDuplicate,
		},
		{
			"no peer-as",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 1",
			errNeighborCreateRequired,
		},
		{
			"no local-address",
			"create neighbor 10.0.0.2 local-as 1 peer-as 2",
			errNeighborCreateRequired,
		},
		{
			"no address at all",
			"create neighbor",
			errNeighborCreateAddress,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, ok, err := ConvertNeighborControl(bridgeEveryPeer, tc.rest)
			if !ok {
				t.Fatal("create names a neighbor lifecycle command, so the bridge owns the refusal")
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
			if onlyCommand(translation) != "" {
				t.Errorf("a refused line carries no command, got %q", onlyCommand(translation))
			}
		})
	}
}

// TestNeighborDeleteReachesTheZeDeleteCommand holds the sibling of create.
//
// GOAL: `delete neighbor <ip>` reaches `delete bgp peer <ip>`, and a line
// carrying ExaBGP's filter is refused rather than widened.
// METHOD: translate one line of each shape.
//
// VALIDATES: the plain line becomes the ze command, and a filtered one answers
// errNeighborDeleteFilter.
// PREVENTS: a filtered delete that removes a peer the script did not name.
func TestNeighborDeleteReachesTheZeDeleteCommand(t *testing.T) {
	translation, ok, err := ConvertNeighborControl(bridgeEveryPeer, "delete neighbor 127.0.0.1")
	if !ok {
		t.Fatal("delete names a neighbor lifecycle command")
	}
	if err != nil {
		t.Fatalf("error = %v, want a translation", err)
	}
	if got := onlyCommand(translation); got != "delete bgp peer 127.0.0.1" {
		t.Errorf("command = %q, want %q", got, "delete bgp peer 127.0.0.1")
	}

	_, _, err = ConvertNeighborControl(bridgeEveryPeer, "delete neighbor 127.0.0.1 local-as 1")
	if !errors.Is(err, errNeighborDeleteFilter) {
		t.Fatalf("error = %v, want errNeighborDeleteFilter", err)
	}

	_, _, err = ConvertNeighborControl(bridgeEveryPeer, "delete neighbor")
	if !errors.Is(err, errNeighborDeleteAddress) {
		t.Fatalf("error = %v, want errNeighborDeleteAddress", err)
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
