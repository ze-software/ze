package flowspecfirewall

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/family"
)

func TestComponentToMatches(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	// Type 1: Destination Prefix 10.0.0.0/24
	dest := flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24"))
	matches, err := componentToMatch(dest, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.0.0.0/24")}, matches[0])

	// Type 2: Source Prefix 192.168.1.0/24
	src := flowspec.NewFlowSourcePrefixComponent(netip.MustParsePrefix("192.168.1.0/24"))
	matches, err = componentToMatch(src, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("192.168.1.0/24")}, matches[0])

	// Type 3: Protocol TCP (6)
	proto := flowspec.NewFlowIPProtocolComponent(6)
	matches, err = componentToMatch(proto, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchProtocol{Protocol: "tcp"}, matches[0])

	// Type 5: Destination Port 80
	dport := flowspec.NewFlowDestPortComponent(80)
	matches, err = componentToMatch(dport, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}}, matches[0])

	// Type 6: Source Port 443
	sport := flowspec.NewFlowSourcePortComponent(443)
	matches, err = componentToMatch(sport, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchSourcePort{Ranges: []firewall.PortRange{{Lo: 443, Hi: 443}}}, matches[0])

	// Type 7: ICMP Type 8 (echo request)
	icmpType := flowspec.NewFlowICMPTypeComponent(8)
	matches, err = componentToMatch(icmpType, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchICMPType{Type: 8}, matches[0])

	// Type 9: TCP Flags SYN (0x02)
	tcpFlags := flowspec.NewFlowTCPFlagsComponent(0x02)
	matches, err = componentToMatch(tcpFlags, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchTCPFlags{Flags: 0x02, Mask: 0x02}, matches[0])

	// Type 11: DSCP 46
	dscp := flowspec.NewFlowDSCPComponent(46)
	matches, err = componentToMatch(dscp, fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchDSCP{Value: 46}, matches[0])
}

func TestActionToFirewall(t *testing.T) {
	// Discard
	actions := actionToFirewall(flowAction{discard: true})
	require.Len(t, actions, 1)
	assert.Equal(t, firewall.Drop{}, actions[0])

	// Rate limit
	actions = actionToFirewall(flowAction{rateLimit: 8000})
	require.Len(t, actions, 2)
	limit, ok := actions[0].(firewall.Limit)
	require.True(t, ok)
	assert.Equal(t, uint64(8000), limit.Rate)
	assert.Equal(t, firewall.Accept{}, actions[1])

	// DSCP marking
	actions = actionToFirewall(flowAction{markDSCP: 46, hasMark: true})
	require.Len(t, actions, 2)
	assert.Equal(t, firewall.SetDSCP{Value: 46}, actions[0])
	assert.Equal(t, firewall.Accept{}, actions[1])

	// No action
	actions = actionToFirewall(flowAction{})
	assert.Empty(t, actions)

	// Combined rate-limit + DSCP mark
	actions = actionToFirewall(flowAction{rateLimit: 1000, hasMark: true, markDSCP: 46})
	require.Len(t, actions, 3)
	_, isLimit := actions[0].(firewall.Limit)
	assert.True(t, isLimit)
	assert.Equal(t, firewall.SetDSCP{Value: 46}, actions[1])
	assert.Equal(t, firewall.Accept{}, actions[2])
}

func TestUnsupportedComponentRejected(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	// Type 10: Packet Length - unsupported
	pktLen := flowspec.NewFlowPacketLengthComponent(128)
	_, err := componentToMatch(pktLen, fam)
	assert.ErrorIs(t, err, errUnsupportedComponent)

	// Type 12: Fragment - unsupported
	frag := flowspec.NewFlowFragmentComponent(flowspec.FlowFragDontFragment)
	_, err = componentToMatch(frag, fam)
	assert.ErrorIs(t, err, errUnsupportedComponent)

	// Type 13: Flow Label - unsupported
	flowLabel := flowspec.NewFlowFlowLabelComponent(100)
	_, err = componentToMatch(flowLabel, fam)
	assert.ErrorIs(t, err, errUnsupportedComponent)
}

func TestNoActionSkipped(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	fs := flowspec.NewFlowSpec(fam)
	fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))

	_, err := translateFlowSpec(fs, flowAction{}, "test-key")
	assert.ErrorIs(t, err, errNoAction)
}

func TestBuildTerm(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	fs := flowspec.NewFlowSpec(fam)
	fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))

	act := flowAction{discard: true}
	terms, err := translateFlowSpec(fs, act, "key1")
	require.NoError(t, err)
	require.Len(t, terms, 1)
	assert.NotEmpty(t, terms[0].Name)
	require.Len(t, terms[0].Matches, 1)
	require.Len(t, terms[0].Actions, 1)
	assert.Equal(t, firewall.Drop{}, terms[0].Actions[0])
}

func TestTermNaming(t *testing.T) {
	n1 := termName("key1")
	n2 := termName("key2")
	n1b := termName("key1")

	assert.NotEqual(t, n1, n2)
	assert.Equal(t, n1, n1b)
	assert.NotEmpty(t, n1)
}

func TestParseExtendedCommunities(t *testing.T) {
	// Discard
	act := parseExtendedCommunities([]string{"rate-limit:0"})
	assert.True(t, act.discard)

	// Rate limit
	act = parseExtendedCommunities([]string{"rate-limit:8000"})
	assert.False(t, act.discard)
	assert.Equal(t, uint32(8000), act.rateLimit)

	// DSCP marking
	act = parseExtendedCommunities([]string{"mark:46"})
	assert.True(t, act.hasMark)
	assert.Equal(t, uint8(46), act.markDSCP)

	// No relevant communities
	act = parseExtendedCommunities([]string{"target:65000:100"})
	assert.False(t, act.discard)
	assert.Equal(t, uint32(0), act.rateLimit)
	assert.False(t, act.hasMark)

	// Empty
	act = parseExtendedCommunities(nil)
	assert.False(t, act.discard)

	// Overflow: value > MaxUint32 clamps to MaxUint32
	act = parseExtendedCommunities([]string{"rate-limit:99999999999"})
	assert.Equal(t, uint32(0xFFFFFFFF), act.rateLimit)
}

func TestParseNLRIJSON(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	nlri := []byte(`{"destination":[["10.0.0.0/24"]],"protocol":[["=6"]],"destination-port":[["=80"]]}`)
	fs, err := parseNLRIJSON(fam, nlri)
	require.NoError(t, err)
	require.Len(t, fs.Components(), 3)

	// Unsupported component rejects the rule
	nlriBad := []byte(`{"destination":[["10.0.0.0/24"]],"packet-length":[["=128"]]}`)
	_, err = parseNLRIJSON(fam, nlriBad)
	assert.ErrorIs(t, err, errUnsupportedComponent)

	// Empty NLRI is valid (no components)
	nlriEmpty := []byte(`{}`)
	fs, err = parseNLRIJSON(fam, nlriEmpty)
	require.NoError(t, err)
	assert.Empty(t, fs.Components())

	// Malformed JSON
	_, err = parseNLRIJSON(fam, []byte(`not json`))
	assert.Error(t, err)
}

func TestPortAnySplitsTerms(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	fs := flowspec.NewFlowSpec(fam)
	fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(flowspec.NewFlowPortComponent(80))

	act := flowAction{discard: true}
	terms, err := translateFlowSpec(fs, act, "port-any-key")
	require.NoError(t, err)
	require.Len(t, terms, 2, "Type 4 Port should produce two terms (src OR dst)")

	// One term has MatchSourcePort, the other MatchDestinationPort
	hasSrc, hasDst := false, false
	for _, term := range terms {
		for _, m := range term.Matches {
			switch m.(type) {
			case firewall.MatchSourcePort:
				hasSrc = true
			case firewall.MatchDestinationPort:
				hasDst = true
			}
		}
	}
	assert.True(t, hasSrc, "one term should match source port")
	assert.True(t, hasDst, "one term should match destination port")

	// Both split terms must preserve the destination prefix match
	for _, term := range terms {
		hasDstAddr := false
		for _, m := range term.Matches {
			if _, ok := m.(firewall.MatchDestinationAddress); ok {
				hasDstAddr = true
			}
		}
		assert.True(t, hasDstAddr, "split term %q must preserve MatchDestinationAddress", term.Name)
	}
}

func TestParseNLRIJSONIPv6(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}

	nlri := []byte(`{"destination-ipv6":[["2001:db8::/32"]],"next-header":[["=6"]]}`)
	fs, err := parseNLRIJSON(fam, nlri)
	require.NoError(t, err)
	require.Len(t, fs.Components(), 2)
}

func TestRangeOperatorRejected(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	// Range operator in destination-port
	nlri := []byte(`{"destination":[["10.0.0.0/24"]],"destination-port":[[">=1024"]]}`)
	_, err := parseNLRIJSON(fam, nlri)
	assert.ErrorIs(t, err, errUnsupportedOperator)

	// Range operator in protocol
	nlri = []byte(`{"protocol":[[">6"]]}`)
	_, err = parseNLRIJSON(fam, nlri)
	assert.ErrorIs(t, err, errUnsupportedOperator)
}

func TestProtoNameUnknown(t *testing.T) {
	assert.Equal(t, "99", protoName(99))
}
