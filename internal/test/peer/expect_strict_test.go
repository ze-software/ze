package peer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseExpectRefusesUnknownOptionKey drives ze-peer's own option parser
// with a key no arm reads, a key that belongs to a sibling value, and a value
// that does not parse.
//
// VALIDATES: each option ze-peer claims refuses a key it does not read, per
// option and per open=/update= value, and send-route refuses a number or an
// address that does not parse; the error names the line, the directive, the key
// and the accepted keys.
// PREVENTS: option=update:value=send-route:origin-as=6500x sending AS 0, and
// option=open:value=router-id:code=5 ignoring code= in silence. Both used to
// parse and leave the test asserting against a session it never configured.
func TestParseExpectRefusesUnknownOptionKey(t *testing.T) {
	tests := []struct {
		line   string
		wantIn []string
	}{
		{"option=bind:value=ipv6:bogus=1", []string{"option=bind", `"bogus"`, "accepts value"}},
		{"option=asn:value=65000:bogus=1", []string{"option=asn", `"bogus"`, "accepts peer, value"}},
		{"option=linger:value=true:bogus=1", []string{"option=linger", `"bogus"`}},
		{"option=await-file:path=x:bogus=1", []string{"option=await-file", `"bogus"`, "accepts path"}},
		{"option=tcp_connections:value=2:bogus=1", []string{"option=tcp_connections", `"bogus"`}},
		{"option=open:value=router-id:id=1.2.3.4:code=5", []string{"option=open:value=router-id", `"code"`, "accepts id, value"}},
		{"option=open:value=send-unknown-message:bogus=1", []string{"option=open:value=send-unknown-message", `"bogus"`, "accepts value"}},
		{"option=update:value=send-default-route:prefix=1", []string{"option=update:value=send-default-route", `"prefix"`}},
		{"option=update:value=send-route:prefix=10.0.0.0/24:bogus=1", []string{"option=update:value=send-route", `"bogus"`}},
		{"option=update:value=send-bulk:prefix=10.0.0.0/24:count=1:next-hop=1.1.1.1:origin-as=1:bogus=1", []string{"option=update:value=send-bulk", `"bogus"`}},
		{"option=update:value=send-route:prefix=10.0.0.0/24:origin-as=6500x", []string{"origin-as", `"6500x"`}},
		{"option=update:value=send-route:prefix=10.0.0.0/24:as-path=1,x", []string{"as-path", `"x"`}},
		{"option=update:value=send-route:prefix=10.0.0.0/24:originator-id=1.2.3", []string{"originator-id", `"1.2.3"`}},
		{"option=update:value=send-route:prefix=10.0.0.0/24:originator-id=2001:db8::1", []string{"originator-id"}},
		{"option=update:value=send-route:prefix=10.0.0.0/24:cluster-list=1.1.1.1,bad", []string{"cluster-list", `"bad"`}},
		{"option=update:value=send-route:prefix=10.0.0.0/24:label=x", []string{"label", `"x"`}},
		{"option=linger:value=true:value=false", []string{`"value"`, "twice"}},
	}
	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			_, _, err := ParseExpect(strings.NewReader(tt.line + "\n"))
			require.Error(t, err, "le test peer must refuse the line, not drop the key")
			for _, want := range tt.wantIn {
				assert.Contains(t, err.Error(), want)
			}
			assert.Contains(t, err.Error(), "line 1:")

			// ClaimLine is the runner's view of the same parser, so it must
			// refuse the same line rather than report it claimed.
			_, claimErr := ClaimLine(tt.line)
			require.Error(t, claimErr)
		})
	}
}

// TestParseExpectStrictKeepsEveryKey is the accepting polarity: each value
// carrying every key its arm reads still parses.
func TestParseExpectStrictKeepsEveryKey(t *testing.T) {
	block := strings.Join([]string{
		"option=asn:value=65001:peer=10.0.0.1",
		"option=update:value=send-route:prefix=10.0.0.0/24:origin-as=65001:next-hop=10.0.0.1:as-set=true:" +
			"as-path=1,2:originator-id=1.1.1.1:cluster-list=2.2.2.2,3.3.3.3:label=16",
		"option=open:value=add-capability:code=200:hex=00",
		"option=open:value=router-id:id=1.2.3.4",
	}, "\n") + "\n"
	_, config, err := ParseExpect(strings.NewReader(block))
	require.NoError(t, err)
	require.Len(t, config.SendRoutes, 1)
	route := config.SendRoutes[0]
	assert.Equal(t, uint32(65001), route.OriginAS)
	assert.Equal(t, []uint32{1, 2}, route.ASPath)
	assert.Equal(t, uint32(0x01010101), route.OriginatorID)
	assert.Equal(t, []uint32{0x02020202, 0x03030303}, route.ClusterList)
	assert.Equal(t, []uint32{16}, route.Labels)
}

// TestPeerOptionArmsEveryEntryParses proves every option and open=/update=
// value ze-peer accepts carries a parser.
//
// VALIDATES: no peerOptionArms or peerValueArms entry has a nil parse.
// PREVENTS: an option added to the table that ze-peer claims and then ignores.
func TestPeerOptionArmsEveryEntryParses(t *testing.T) {
	for name, arm := range peerOptionArms {
		assert.NotNil(t, arm.parse, "option=%s has no parser", name)
	}
	for optType, values := range peerValueArms {
		for value, arm := range values {
			assert.NotNil(t, arm.parse, "option=%s:value=%s has no parser", optType, value)
		}
	}
}
