package bridge

import (
	"errors"
	"strings"
	"testing"
)

// TestNeighborTeardownCarriesTheNotification holds the ExaBGP teardown grammar
// the bridge carries to ze's notify.
//
// GOAL: every `neighbor <ip> teardown [<code>] [<subcode>] [<text>]` form the
// pinned ExaBGP accepts reaches `request peer <ip> notify <code> <subcode>
// [hex <octets>]`, so the script's NOTIFICATION goes on the wire with its
// code, its subcode and the Data field the pin builds from its text.
// METHOD: translate each line and compare the whole command string, because the
// octets are positional and the text keeps its case; a dropped or lowercased
// word is invisible in a substring match. The forms are the pin's
// teardown_notification (reactor/api/command/neighbor.py): no number is Cease,
// Administrative Shutdown; one number is a Cease subcode; two are a code and a
// subcode; the rest, shlex-split and joined with one space, is the text.
//
// VALIDATES: the command, the selector travelling with it, and no UPDATE claimed.
// PREVENTS: a code other than Cease refused or dropped, and a teardown with text
// or with no number refused, which a script written for ExaBGP 6 meets as an
// error where ExaBGP closes the session.
func TestNeighborTeardownCarriesTheNotification(t *testing.T) {
	cases := []struct {
		name     string
		selector string
		rest     string
		want     string
	}{
		{"api-teardown", "127.0.0.1", "teardown 4", "request peer 127.0.0.1 notify 6 4"},
		{"unspecific", "10.0.0.1", "teardown 0", "request peer 10.0.0.1 notify 6 0"},
		{"widest subcode", "10.0.0.1", "teardown 255", "request peer 10.0.0.1 notify 6 255"},
		{"every peer", bridgeEveryPeer, "teardown 6", "request peer * notify 6 6"},
		{"upper case verb", "10.0.0.1", "TEARDOWN 2", "request peer 10.0.0.1 notify 6 2"},
		{"no number is administrative shutdown", "10.0.0.1", "teardown", "request peer 10.0.0.1 notify 6 2"},
		{"code and subcode", "10.0.0.1", "teardown 6 4", "request peer 10.0.0.1 notify 6 4"},
		{"subcode and text", "10.0.0.1", "teardown 2 Back Soon", "request peer 10.0.0.1 notify 6 2 hex 09" + "4261636b20536f6f6e"},
		{"code, subcode and text", "10.0.0.1", "teardown 6 2 going away", "request peer 10.0.0.1 notify 6 2 hex 0a" + "676f696e672061776179"},
		{"quoted text starting with a digit", "10.0.0.1", "teardown 2 '42 is   it'", "request peer 10.0.0.1 notify 6 2 hex 0a" + "34322069732020206974"},
		{"third number is text", "10.0.0.1", "teardown 6 2 1", "request peer 10.0.0.1 notify 6 2 hex 0131"},
		{"code other than cease", "10.0.0.1", "teardown 3 1", "request peer 10.0.0.1 notify 3 1"},
		{"code other than cease with text", "10.0.0.1", "teardown 1 2 bad length", "request peer 10.0.0.1 notify 1 2 hex 626164206c656e677468"},
		{"communication of 128 octets is whole", "10.0.0.1", "teardown 2 " + strings.Repeat("a", 128), "request peer 10.0.0.1 notify 6 2 hex 80" + strings.Repeat("61", 128)},
		{"communication cut to the pin's 128 octets", "10.0.0.1", "teardown 4 " + strings.Repeat("a", 130), "request peer 10.0.0.1 notify 6 4 hex 80" + strings.Repeat("61", 128)},
		{"communication cut back to a whole character", "10.0.0.1", "teardown 2 a" + strings.Repeat("é", 64), "request peer 10.0.0.1 notify 6 2 hex 7f61" + strings.Repeat("c3a9", 63)},
		{"communication keeps UTF-8", "10.0.0.1", "teardown 2 café", "request peer 10.0.0.1 notify 6 2 hex 05636166c3a9"},
		{"Data outside a communication is not cut at 128", "10.0.0.1", "teardown 6 3 " + strings.Repeat("a", 200), "request peer 10.0.0.1 notify 6 3 hex " + strings.Repeat("61", 200)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, ok, err := convertNeighborControl(tc.selector, tc.rest)
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

// TestNeighborTeardownRefusals pins the forms the pin refuses, with its text.
//
// GOAL: a teardown that names no octet value or holds an unclosed quote is
// refused where the script wrote it.
// METHOD: drive each form and compare the whole error text, because the pin's
// texts are what a script written against ExaBGP 6 matches.
//
// VALIDATES: the pin's three refusals word for word.
// PREVENTS: a value no octet holds sent on to ze as text.
func TestNeighborTeardownRefusals(t *testing.T) {
	cases := []struct {
		rest string
		want string
	}{
		{"teardown abc", "expected a code or a subcode, got 'abc'"},
		{"teardown -1", "expected a code or a subcode, got '-1'"},
		{"teardown 256", "'256' is not a value from 0 to 255"},
		{"teardown 6 300", "'300' is not a value from 0 to 255"},
		{"teardown 2 'open", "No closing quotation"},
	}
	for _, tc := range cases {
		t.Run(tc.rest, func(t *testing.T) {
			translation, ok, err := convertNeighborControl("127.0.0.1", tc.rest)
			if !ok {
				t.Fatalf("the line names teardown, so it is ours to refuse, got ok=false")
			}
			if err == nil {
				t.Fatalf("want error %q, got command %q", tc.want, onlyCommand(translation))
			}
			if err.Error() != tc.want {
				t.Errorf("error = %q, want %q", err.Error(), tc.want)
			}
			if onlyCommand(translation) != "" {
				t.Errorf("a refused line carries no command, got %q", onlyCommand(translation))
			}
		})
	}
}

// TestNeighborTeardownWritesThePinsOctets pins the Data field the bridge
// builds for a text, octet for octet against the pin's Notify._wire_text.
//
// GOAL: the peer receives the octets ExaBGP 6 sends for the same line.
// METHOD: each expected hex string is what the pin's own encoding produces,
// computed with Python: `detail.encode('ascii', 'backslashreplace')` under a
// pair that is not Cease 2 or 4, a length octet and UTF-8 cut to 128 under
// Cease 2 and 4, and every Data field cut to 4075 octets. Compare the whole
// command, because the octets are positional.
//
// VALIDATES: teardownWireData, backslashReplace and teardownCommunication.
// PREVENTS: ze sending UTF-8 where ExaBGP sends `caf\xe9`, a `"` or a backslash
// lost to ze's command tokenizer, and a Data field longer than the pin's.
func TestNeighborTeardownWritesThePinsOctets(t *testing.T) {
	cases := []struct {
		name string
		rest string
		want string
	}{
		{"latin-1 character as \\x", "teardown 6 3 café", "notify 6 3 hex 6361665c786539"},
		{"BMP character as \\u", "teardown 1 2 €uro", "notify 1 2 hex 5c753230616375726f"},
		{"astral character as \\U", "teardown 1 2 x😀", "notify 1 2 hex 785c553030303166363030"},
		{"double quote kept", `teardown 3 1 'say "hi"'`, "notify 3 1 hex 7361792022686922"},
		{"backslash kept", `teardown 3 1 'a\b'`, "notify 3 1 hex 615c62"},
		{"Data cut to 4075 octets", "teardown 3 1 " + strings.Repeat("a", 4100), "notify 3 1 hex " + strings.Repeat("61", 4075)},
		{"escapes cut to 4075 octets", "teardown 3 1 " + strings.Repeat("é", 2000), "notify 3 1 hex " + strings.Repeat("5c786539", 1018) + "5c7865"},
		{"communication cut to 128 octets", "teardown 6 2 " + strings.Repeat("€", 50), "notify 6 2 hex 7e" + strings.Repeat("e282ac", 42)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, ok, err := convertNeighborControl("127.0.0.1", tc.rest)
			if !ok {
				t.Fatalf("teardown is a neighbor lifecycle command, got ok=false")
			}
			if err != nil {
				t.Fatalf("ze performs this teardown, got error: %v", err)
			}
			want := "request peer 127.0.0.1 " + tc.want
			if onlyCommand(translation) != want {
				t.Errorf("command = %q, want %q", onlyCommand(translation), want)
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
// process attaches the bridge plugin that runs it, and the created address is
// the selector.
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
			"create bgp peer 127.0.0.1 asn 1 local-as 1 local-address 127.0.0.1 router-id 1.2.3.4 accept false attach exabgp-bridge",
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
			"two api processes attach the one bridge plugin once",
			"create neighbor 10.0.0.2 local-address 10.0.0.1 local-as 65001 peer-as 65002 api proc1 api proc2",
			"create bgp peer 10.0.0.2 asn 65002 local-as 65001 local-address 10.0.0.1 accept false attach exabgp-bridge",
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
			translation, ok, err := convertNeighborControl(bridgeEveryPeer, tc.rest)
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

	translation, _, err := convertNeighborControl(bridgeEveryPeer, line)
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
			translation, ok, err := convertNeighborControl(bridgeEveryPeer, tc.rest)
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
	translation, ok, err := convertNeighborControl(bridgeEveryPeer, "delete neighbor 127.0.0.1")
	if !ok {
		t.Fatal("delete names a neighbor lifecycle command")
	}
	if err != nil {
		t.Fatalf("error = %v, want a translation", err)
	}
	if got := onlyCommand(translation); got != "delete bgp peer 127.0.0.1" {
		t.Errorf("command = %q, want %q", got, "delete bgp peer 127.0.0.1")
	}

	_, _, err = convertNeighborControl(bridgeEveryPeer, "delete neighbor 127.0.0.1 local-as 1")
	if !errors.Is(err, errNeighborDeleteFilter) {
		t.Fatalf("error = %v, want errNeighborDeleteFilter", err)
	}

	_, _, err = convertNeighborControl(bridgeEveryPeer, "delete neighbor")
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

	translation, ok, err := convertNeighborControl("127.0.0.1", rest)
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
			translation, ok, err := convertNeighborControl("127.0.0.1", rest)
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

// TestNeighborDisableEnable checks the ExaBGP 6.0 administrative hold
// commands (reactor/api/command/neighbor.py, disable and enable): disable's
// words, read as shlex.split reads them, become the ze command's message, and
// enable carries nothing.
//
// VALIDATES: AC-41, the bridge half.
// PREVENTS: `neighbor <ip> disable "back soon"` refused as unknown, or the
// quotes reaching the Shutdown Communication on the wire.
func TestNeighborDisableEnable(t *testing.T) {
	cases := []struct {
		name string
		rest string
		want string
	}{
		{"quoted message", `disable "back soon"`, "request peer 127.0.0.1 disable back soon"},
		{"single quotes", `disable 'it''s' later`, "request peer 127.0.0.1 disable its later"},
		{"escaped quote", `disable "say \"hi\""`, `request peer 127.0.0.1 disable say "hi"`},
		{"no message", "disable", "request peer 127.0.0.1 disable"},
		{"enable", "enable", "request peer 127.0.0.1 enable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, ok, err := convertNeighborControl("127.0.0.1", tc.rest)
			if !ok || err != nil {
				t.Fatalf("ok=%v err=%v, want a translation", ok, err)
			}
			if onlyCommand(translation) != tc.want {
				t.Errorf("command = %q, want %q", onlyCommand(translation), tc.want)
			}
			if translation.Selector != "127.0.0.1" {
				t.Errorf("selector = %q", translation.Selector)
			}
		})
	}
}

// TestNeighborDisableEnableRefusals checks the two refusals ExaBGP answers:
// an unclosed quote in disable, and an argument to enable.
func TestNeighborDisableEnableRefusals(t *testing.T) {
	for rest, want := range map[string]string{
		`disable "back soon`: "No closing quotation",
		"enable now":         "enable takes no argument",
	} {
		_, ok, err := convertNeighborControl("127.0.0.1", rest)
		if !ok || err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: ok=%v err=%v, want a refusal containing %q", rest, ok, err, want)
		}
	}
}
