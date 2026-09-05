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
			assert.Equal(t, tc.want, translation.Command)
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
			assert.Empty(t, translation.Command, "a local action reaches ze's dispatcher never")
		})
	}

	// The neighbor prefix is carried into the selector, so a control command can
	// be aimed the same way a route can.
	translation, err := TranslateLine("neighbor 10.0.0.1 flush adj-rib out")
	require.NoError(t, err)
	assert.Equal(t, "request peer 10.0.0.1 flush", translation.Command)
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
			assert.Contains(t, translation.Command, "ipv4/"+zeName,
				"the command must name the family ze declares, not the one ExaBGP wrote")
		})
	}

	// The row that pays for the mapping existing at all.
	translation, err := TranslateLine("announce ipv4 mcast-vpn source-ad source 10.0.0.1 group 239.0.0.1 rd 65000:1 next-hop 10.0.0.2")
	require.NoError(t, err)
	assert.Contains(t, translation.Command, "ipv4/mvpn")
	assert.NotContains(t, translation.Command, "mcast-vpn", "ze does not know that spelling")
}
