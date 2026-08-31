package flowspecfirewall

import (
	"net/netip"
	"strconv"
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
	require.NoError(t, fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24"))))

	_, err := translateFlowSpec(fs, flowAction{}, "test-key")
	assert.ErrorIs(t, err, errNoAction)
}

func TestBuildTerm(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	fs := flowspec.NewFlowSpec(fam)
	require.NoError(t, fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24"))))

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

	nlri := []byte(`{"destination-ipv4":[["10.0.0.0/24"]],"protocol":[["=6"]],"destination-port":[["=80"]]}`)
	fs, err := parseNLRIJSON(fam, nlri)
	require.NoError(t, err)
	require.Len(t, fs.Components(), 3)

	// Unsupported component rejects the rule
	nlriBad := []byte(`{"destination-ipv4":[["10.0.0.0/24"]],"packet-length":[["=128"]]}`)
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
	require.NoError(t, fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24"))))
	require.NoError(t, fs.AddComponent(flowspec.NewFlowPortComponent(80)))

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
	nlri := []byte(`{"destination-ipv4":[["10.0.0.0/24"]],"destination-port":[[">=1024"]]}`)
	_, err := parseNLRIJSON(fam, nlri)
	assert.ErrorIs(t, err, errUnsupportedOperator)

	// Range operator in protocol
	nlri = []byte(`{"protocol":[[">6"]]}`)
	_, err = parseNLRIJSON(fam, nlri)
	assert.ErrorIs(t, err, errUnsupportedOperator)
}

// TestComponentToMatchRejectsUnnamedProtocol replaces TestProtoNameUnknown,
// which asserted that protocol 99 translated to the string "99".
//
// VALIDATES: a type 3 value with no canonical name is refused at translation.
// PREVENTS: a MatchProtocol carrying decimal digits. No backend resolves a
// number, so the rule failed in the nft backend instead, and that failure
// returns from Apply before Flush -- one such rule from one peer stopped every
// other owner's ruleset from reaching the kernel.
func TestComponentToMatchRejectsUnnamedProtocol(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	for _, proto := range []uint8{0, 2, 99, 133, 255} {
		comp := flowspec.NewFlowIPProtocolComponent(proto)
		matches, err := componentToMatch(comp, fam)
		require.ErrorIs(t, err, errUnknownProtocol, "protocol %d must be refused", proto)
		assert.Contains(t, err.Error(), strconv.Itoa(int(proto)), "the error must name the protocol number")
		assert.Empty(t, matches, "a refused protocol produces no match")
	}
}

// TestComponentToMatchSCTPName is the wiring case: protocol 132 arrives from a
// peer and must become the canonical name the backends resolve.
//
// VALIDATES: type 3 value 132 becomes MatchProtocol{"sctp"}.
// PREVENTS: the five-name private table returning, which knew 1, 6, 17, 47 and
// 58 and rendered every other value as digits.
func TestComponentToMatchSCTPName(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	matches, err := componentToMatch(flowspec.NewFlowIPProtocolComponent(132), fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchProtocol{Protocol: "sctp"}, matches[0])
}

// TestComponentToMatchEveryCanonicalNumber walks the whole canonical table
// rather than a sample, so a name added to the table without a translator
// change is caught here and not by an operator.
//
// VALIDATES: every number firewall.ProtocolNumber knows translates to its name.
// PREVENTS: the translator knowing a subset of the names the backends accept.
func TestComponentToMatchEveryCanonicalNumber(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	for _, name := range firewall.ProtocolNames() {
		num, ok := firewall.ProtocolNumber(name)
		require.True(t, ok)
		matches, err := componentToMatch(flowspec.NewFlowIPProtocolComponent(num), fam)
		require.NoError(t, err, "protocol %d (%s)", num, name)
		require.Len(t, matches, 1)
		assert.Equal(t, firewall.MatchProtocol{Protocol: name}, matches[0])
	}
}

// TestComponentToMatchMultipleProtocolValues covers a type 3 component that
// lists several values. RFC 8955 Section 4.2.2 gives a numeric operator list OR
// semantics, and a firewall Term ANDs its matches, so one value per match is
// the only faithful rendering.
//
// VALIDATES: every listed value produces a match, repeats collapse, and the
// list is refused as a whole when one value has no canonical name.
// PREVENTS: the translator reading vals[0] and discarding the rest, which
// enforced the first protocol alone and said nothing.
func TestComponentToMatchMultipleProtocolValues(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	matches, err := componentToMatch(flowspec.NewFlowIPProtocolComponent(6, 17, 132), fam)
	require.NoError(t, err)
	assert.Equal(t, []firewall.Match{
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchProtocol{Protocol: "udp"},
		firewall.MatchProtocol{Protocol: "sctp"},
	}, matches)

	// A repeat cannot inflate the term count: the bound is the table size.
	matches, err = componentToMatch(flowspec.NewFlowIPProtocolComponent(6, 6, 6, 17), fam)
	require.NoError(t, err)
	assert.Equal(t, []firewall.Match{
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchProtocol{Protocol: "udp"},
	}, matches)

	// One unnamed value refuses the component; no partial enforcement.
	_, err = componentToMatch(flowspec.NewFlowIPProtocolComponent(6, 99), fam)
	require.ErrorIs(t, err, errUnknownProtocol)
}

// TestTranslateFlowSpecMultipleProtocolsBecomeSeparateTerms proves the OR
// semantics survive term construction: matches inside one Term are ANDed, so a
// single Term holding tcp AND udp would match nothing at all.
//
// VALIDATES: a two-protocol component yields two terms with distinct names,
// each carrying one protocol and the shared matches.
// PREVENTS: an unenforceable term that silently drops the peer's second
// protocol, or two terms colliding on one name.
func TestTranslateFlowSpecMultipleProtocolsBecomeSeparateTerms(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	fs := flowspec.NewFlowSpec(fam)
	require.NoError(t, fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24"))))
	require.NoError(t, fs.AddComponent(flowspec.NewFlowIPProtocolComponent(6, 132)))

	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "multi-proto-key")
	require.NoError(t, err)
	require.Len(t, terms, 2)
	assert.NotEqual(t, terms[0].Name, terms[1].Name, "each term needs its own name")

	got := make([]string, 0, 2)
	for _, term := range terms {
		var proto string
		var hasPrefix bool
		for _, m := range term.Matches {
			switch v := m.(type) {
			case firewall.MatchProtocol:
				require.Empty(t, proto, "a term must carry at most one protocol match")
				proto = v.Protocol
			case firewall.MatchDestinationAddress:
				hasPrefix = true
			}
		}
		assert.True(t, hasPrefix, "every term keeps the shared destination match")
		got = append(got, proto)
	}
	assert.Equal(t, []string{"tcp", "sctp"}, got)
}

// TestTranslateFlowSpecSingleProtocolKeepsOneTerm pins the common case against
// the term-splitting change.
//
// VALIDATES: one protocol value still yields exactly one term.
// PREVENTS: every existing FlowSpec rule gaining a term and a new name.
func TestTranslateFlowSpecSingleProtocolKeepsOneTerm(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	fs := flowspec.NewFlowSpec(fam)
	require.NoError(t, fs.AddComponent(flowspec.NewFlowIPProtocolComponent(132)))

	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "single-proto-key")
	require.NoError(t, err)
	require.Len(t, terms, 1)
	assert.Equal(t, termName("single-proto-key"), terms[0].Name)
	assert.Contains(t, terms[0].Matches, firewall.MatchProtocol{Protocol: "sctp"})
}

// TestComponentToMatchEveryWireValue sweeps the whole one-octet value space
// rather than a sample of it, because the value comes from a peer.
//
// VALIDATES: every value 0-255 a peer can put in a type 3 component produces
// either exactly one canonical protocol match or a clean refusal naming the
// number. No value panics, and none produces a match carrying digits.
// PREVENTS: a value between the sampled ones behaving differently. RFC 8955
// Section 4.2.2.3 makes the field one octet, so this IS the input space, and a
// peer chooses which of the 256 to send.
func TestComponentToMatchEveryWireValue(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	canonical := 0
	for v := range 256 {
		proto := uint8(v)
		matches, err := componentToMatch(flowspec.NewFlowIPProtocolComponent(proto), fam)

		name, known := firewall.ProtocolName(proto)
		if !known {
			require.ErrorIs(t, err, errUnknownProtocol, "protocol %d must be refused", proto)
			assert.Contains(t, err.Error(), strconv.Itoa(v), "the refusal must name the protocol number")
			assert.Empty(t, matches, "a refused protocol produces no match")
			continue
		}
		canonical++
		require.NoError(t, err, "protocol %d (%s)", proto, name)
		require.Len(t, matches, 1)
		assert.Equal(t, firewall.MatchProtocol{Protocol: name}, matches[0])
	}
	assert.Len(t, firewall.ProtocolNames(), canonical,
		"the sweep must have accepted exactly the canonical table and nothing else")
}

// TestTrafficRatePacketsInstallsAPacketDimension pins the unit the peer asked
// for. RFC 8955 Section 7.2 defines traffic-rate-packets in PACKETS per second
// and Section 7.1 defines traffic-rate in BYTES per second; AppendDecoded
// renders the first as "rate-limit:<n>:packets" and the second as
// "rate-limit:<n>", so the suffix is the only thing telling them apart.
//
// Before this test, actionToFirewall always built RateDimensionBytes, so a peer
// asking to police 1000 pkt/s had a 1000 byte/s limit installed instead --
// roughly three orders of magnitude tighter than requested, on traffic the
// operator asked to be policed rather than dropped.
func TestTrafficRatePacketsInstallsAPacketDimension(t *testing.T) {
	// actionToFirewall appends a trailing Accept, so find the Limit rather than
	// pinning a position: a later action gaining a sibling is not this test's
	// subject and must not redden it.
	limitIn := func(t *testing.T, actions []firewall.Action) firewall.Limit {
		t.Helper()
		for _, a := range actions {
			if lim, ok := a.(firewall.Limit); ok {
				return lim
			}
		}
		t.Fatalf("no firewall.Limit among %d action(s): %#v", len(actions), actions)
		return firewall.Limit{}
	}

	lim := limitIn(t, actionToFirewall(parseExtendedCommunities([]string{"rate-limit:1000:packets"})))
	if lim.Dimension != firewall.RateDimensionPackets {
		t.Errorf("Dimension = %d, want RateDimensionPackets (%d): RFC 8955 Section 7.2 "+
			"counts packets, and installing bytes polices ~1000x tighter than asked",
			lim.Dimension, firewall.RateDimensionPackets)
	}
	if lim.Rate != 1000 {
		t.Errorf("Rate = %d, want 1000", lim.Rate)
	}

	// The bytes form must keep its own dimension: the suffix decides, and a
	// blanket switch to packets would be the same defect pointing the other way.
	limBytes := limitIn(t, actionToFirewall(parseExtendedCommunities([]string{"rate-limit:8000"})))
	if limBytes.Dimension != firewall.RateDimensionBytes {
		t.Errorf("Dimension = %d for the suffix-less form, want RateDimensionBytes (%d)",
			limBytes.Dimension, firewall.RateDimensionBytes)
	}
}
