package bridge

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBridgeTranslatesTheControlVocabulary holds the ExaBGP API commands that
// are not routes.
//
// A script drives more than announcements. It clears and flushes the adj-RIB,
// turns the command acknowledgement off and on, drives a watchdog group, and
// asks the daemon to stop. The bridge answered none of these until 2026-09-05,
// so a script stopped at its first control command however well the route half
// worked.
//
// VALIDATES: each form translates to the ze command the live registry declares,
// and the three ack words are answered by the bridge rather than dispatched.
// PREVENTS: a script blocking forever on an ack the bridge never sends.
func TestBridgeTranslatesTheControlVocabulary(t *testing.T) {
	dispatched := []struct {
		line string
		want string
	}{
		{"clear adj-rib out", "clear bgp rib out"},
		{"clear adj-rib in", "clear bgp rib in"},
		{"flush adj-rib out", "request peer * flush"},
		{"announce watchdog dnsr", "request bgp watchdog announce dnsr"},
		{"withdraw watchdog dnsr", "request bgp watchdog withdraw dnsr"},
		{"shutdown", "request shutdown"},
		{"request shutdown", "request shutdown"},
	}
	for _, tc := range dispatched {
		t.Run(tc.line, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			require.NoError(t, err)
			assert.Equal(t, tc.want, onlyCommand(translation))
			assert.Equal(t, LocalNone, translation.Local, "a dispatched command is not a local action")
			assert.False(t, translation.Route, "a control command puts no UPDATE on a wire")
		})
	}

	local := []struct {
		line string
		want LocalAction
	}{
		{"enable-ack", LocalAckEnable},
		{"disable-ack", LocalAckDisableAfter},
		{"silence-ack", LocalAckDisableNow},
	}
	for _, tc := range local {
		t.Run(tc.line, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			require.NoError(t, err)
			assert.Equal(t, tc.want, translation.Local)
			assert.Empty(t, onlyCommand(translation), "a local action reaches ze's dispatcher never")
		})
	}

	// The neighbor prefix is carried into the selector, so a control command can
	// be aimed the same way a route can.
	translation, err := TranslateLine("neighbor 10.0.0.1 flush adj-rib out")
	require.NoError(t, err)
	assert.Equal(t, "request peer 10.0.0.1 flush", onlyCommand(translation))
}

// TestAckModeApplyTimesTheSilence pins the one thing the three ack words differ
// on, which is WHEN the silence starts. api-ack-control and api-silence-ack
// assert exactly this and nothing else.
//
// VALIDATES: disable-ack is acked and silences what follows; silence-ack is not
// acked at all; enable-ack resumes and is acked.
// PREVENTS: a script that turns acks off waiting for one anyway, and a script
// that turns them back on never being answered again.
func TestAckModeApplyTimesTheSilence(t *testing.T) {
	mode := AckMode{enabled: true}

	assert.True(t, mode.apply(LocalAckDisableAfter), "disable-ack is itself acked")
	assert.False(t, mode.apply(LocalNone), "an ordinary command is silent after disable-ack")

	assert.True(t, mode.apply(LocalAckEnable), "enable-ack is acked")
	assert.True(t, mode.apply(LocalNone), "an ordinary command is acked again")

	assert.False(t, mode.apply(LocalAckDisableNow), "silence-ack is not acked itself")
	assert.False(t, mode.apply(LocalNone), "and what follows is silent too")
}

// TestAnswerLocalWritesTheAckDisableStillOwes is the regression for a decision
// made twice. apply sets enabled false for disable-ack while that command still
// owes a done, so a caller that asked apply and then called WriteAck got
// silence: WriteAck re-read the flag apply had just cleared.
func TestAnswerLocalWritesTheAckDisableStillOwes(t *testing.T) {
	var out strings.Builder
	mode := AckMode{enabled: true}

	mode.AnswerLocal(&out, LocalAckDisableAfter)
	assert.Equal(t, "done\n", out.String(), "disable-ack is itself acked")

	out.Reset()
	mode.AnswerLocal(&out, LocalAckDisableNow)
	assert.Empty(t, out.String(), "silence-ack is not")

	out.Reset()
	mode.AnswerLocal(&out, LocalAckEnable)
	assert.Equal(t, "done\n", out.String(), "enable-ack resumes and is acked")
}

// TestBridgeSAFIListIsOneDeclaration holds the family vocabulary to a single
// source.
//
// bridgeFamilyRE's alternation and canonicalExabgpSAFI's mapping were two
// hand-written copies of one vocabulary, and they drifted: mcast-vpn was in
// neither, so every frame of api-mvpn was refused by a translator that names
// the family correctly the moment it reaches the mapping. The alternation is
// built from the map now, so a family the map knows is a family the regexp
// matches, by construction rather than by review.
//
// VALIDATES: every key of bridgeSAFI is matched by bridgeFamilyRE and answers
// its mapped ze name; mcast-vpn answers mvpn.
// PREVENTS: the two halves drifting again, which costs a whole family silently.
func TestBridgeSAFIListIsOneDeclaration(t *testing.T) {
	for exabgpName, zeName := range bridgeSAFI {
		t.Run(exabgpName, func(t *testing.T) {
			line := "neighbor 10.0.0.1 announce ipv4 " + exabgpName + " 10.0.0.0/24 next-hop 1.2.3.4"
			translation, err := TranslateLine(line)
			require.NoError(t, err, "a family the map knows must be a family the regexp matches")
			assert.Contains(t, onlyCommand(translation), "ipv4/"+zeName,
				"the command must name the family ze declares, not the one ExaBGP wrote")
		})
	}

	// The row that pays for the mapping existing at all.
	translation, err := TranslateLine("announce ipv4 mcast-vpn source-ad source 10.0.0.1 group 239.0.0.1 rd 65000:1 next-hop 10.0.0.2")
	require.NoError(t, err)
	assert.Contains(t, onlyCommand(translation), "ipv4/mvpn")
	assert.NotContains(t, onlyCommand(translation), "mcast-vpn", "ze does not know that spelling")
}

// TestBareAddressBecomesAHostRoute pins ExaBGP's own reading of an address
// written with no prefix length. Its `prefix` parser splits on `/` and takes 32
// on failure, or 128 when the address holds a colon
// (src/exabgp/configuration/static/parser.py), and the API reuses that parser.
//
// api-check.run writes `neighbor 127.0.0.1 announce route 1.2.3.4 next-hop
// 5.6.7.8`, and its `.ci` expects 1.2.3.4/32 on the wire. Ze reads a prefix and
// answered `invalid prefix: 1.2.3.4`, so the bridge acked a route that never
// left.
func TestBareAddressBecomesAHostRoute(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "ipv4 host route",
			line: "neighbor 127.0.0.1 announce route 1.2.3.4 next-hop 5.6.7.8",
			want: "send bgp 127.0.0.1 update text nhop 5.6.7.8 nlri ipv4/unicast add 1.2.3.4/32",
		},
		{
			name: "ipv6 host route",
			line: "announce route 2001:db8::1 next-hop 2001:db8::2",
			want: "send bgp * update text nhop 2001:db8::2 nlri ipv6/unicast add 2001:db8::1/128",
		},
		{
			name: "a stated length is left alone",
			line: "announce route 10.0.0.0/24 next-hop 1.1.1.1",
			want: "send bgp * update text nhop 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24",
		},
		{
			name: "the attributes form completes every prefix it names",
			line: "announce attributes next-hop 1.1.1.1 nlri 1.2.3.4 5.6.7.8/32",
			want: "send bgp * update text nhop 1.1.1.1 nlri ipv4/unicast add 1.2.3.4/32 5.6.7.8/32",
		},
		{
			name: "a withdraw completes it too",
			line: "neighbor 127.0.0.1 withdraw route 1.2.3.4 next-hop 5.6.7.8",
			want: "send bgp 127.0.0.1 update text nhop 5.6.7.8 nlri ipv4/unicast del 1.2.3.4/32",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			if err != nil {
				t.Fatalf("TranslateLine(%q): %v", tc.line, err)
			}
			if len(translation.Commands) != 1 {
				t.Fatalf("commands = %v, want one", translation.Commands)
			}
			if translation.Commands[0].Text != tc.want {
				t.Errorf("command = %q, want %q", translation.Commands[0].Text, tc.want)
			}
		})
	}
}

// TestAFieldValueKeepsItsBareAddress is the negative half. A family whose NLRI
// is a FIELD LIST carries bare addresses that are values rather than prefixes,
// so completing one would corrupt the route instead of finishing it.
func TestAFieldValueKeepsItsBareAddress(t *testing.T) {
	line := "announce ipv4 mcast-vpn shared-join rp 10.99.199.1 group 239.251.255.228 " +
		"rd 65000:99999 source-as 65000 next-hop 10.10.6.3"
	translation, err := TranslateLine(line)
	if err != nil {
		t.Fatalf("TranslateLine: %v", err)
	}
	if len(translation.Commands) != 1 {
		t.Fatalf("commands = %v, want one", translation.Commands)
	}
	got := translation.Commands[0].Text
	for _, unwanted := range []string{"10.99.199.1/32", "239.251.255.228/32"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("command %q gave a field value a prefix length", got)
		}
	}
}

// TestFamilyAllowedInOpenReachesNoSession pins the one selector qualifier that
// EXCLUDES: `family-allowed in-open`.
//
// VALIDATES: a line qualified `family-allowed in-open` dispatches nothing and is
// still answered, and every other qualifier value still reaches the address.
// PREVENTS: the route going out. ExaBGP's `in-open` names a session that
// negotiates the families the OPEN carried; ze STATES the families its OPEN
// offers and has no such mode, so no ze session is the one that qualifier
// names (neighborFamilies, bridge_neighbor.go, refuses the same word on
// `create neighbor`). Sending anyway put a prefix on the wire that ExaBGP would
// not have sent, which is what `test/exabgp-compat/api/api-multisession.ci`
// caught: five commands, four expected frames.
func TestFamilyAllowedInOpenReachesNoSession(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		unmatched bool
		want      string
	}{
		{
			name:      "in-open reaches nothing",
			line:      "neighbor 127.0.0.1 local-as 1 family-allowed in-open announce route 9.9.9.9/24 next-hop 101.1.101.1",
			unmatched: true,
		},
		{
			name: "a named family still reaches the address",
			line: "neighbor 127.0.0.1 local-as 1 family-allowed ipv4-unicast announce route 1.2.0.0/24 next-hop 101.1.101.1",
			want: "send bgp 127.0.0.1 update text nhop 101.1.101.1 nlri ipv4/unicast add 1.2.0.0/24",
		},
		{
			name: "the other qualifiers still reach the address",
			line: "neighbor 127.0.0.1 local-as 1 peer-as 1 local-ip 127.0.0.1 router-id 1.2.3.4 announce route 1.3.0.0/24 next-hop 101.1.101.1",
			want: "send bgp 127.0.0.1 update text nhop 101.1.101.1 nlri ipv4/unicast add 1.3.0.0/24",
		},
		{
			name: "one excluded neighbor leaves the other addressed",
			line: "neighbor 127.0.0.1 family-allowed in-open, neighbor 127.0.0.2 announce route 1.4.0.0/24 next-hop 101.1.101.1",
			want: "send bgp 127.0.0.2 update text nhop 101.1.101.1 nlri ipv4/unicast add 1.4.0.0/24",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			translation, err := TranslateLine(tc.line)
			if err != nil {
				t.Fatalf("TranslateLine(%q): %v", tc.line, err)
			}
			if tc.unmatched {
				if !translation.Unmatched {
					t.Fatalf("commands = %v, want the line to reach no session", translation.Commands)
				}
				if len(translation.Commands) != 0 {
					t.Errorf("an unmatched line carries commands: %v", translation.Commands)
				}
				return
			}
			if translation.Unmatched {
				t.Fatalf("the line reached no session, want %q", tc.want)
			}
			if len(translation.Commands) != 1 {
				t.Fatalf("commands = %v, want one", translation.Commands)
			}
			if translation.Commands[0].Text != tc.want {
				t.Errorf("command = %q, want %q", translation.Commands[0].Text, tc.want)
			}
		})
	}
}
