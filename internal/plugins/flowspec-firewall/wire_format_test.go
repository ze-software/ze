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

// realNLRIJSON produces the NLRI JSON exactly as the daemon does. The bridge
// receives the output of FlowSpec.AppendJSON (internal/component/bgp/format,
// appendNLRIJSONValue) over the plugin socket, so a test that hand-writes the
// JSON tests the bridge against a shape nothing produces.
func realNLRIJSON(t *testing.T, fam family.Family, comps ...flowspec.FlowComponent) []byte {
	t.Helper()
	fs := flowspec.NewFlowSpec(fam)
	for _, c := range comps {
		require.NoError(t, fs.AddComponent(c))
	}
	return fs.AppendJSON(nil)
}

// TestParseNLRIJSONReadsWhatTheDaemonWrites is the producer-to-consumer test
// this bridge never had.
//
// VALIDATES: every component the NLRI JSON writer emits survives the bridge's
// parser, protocol names and the prefix offset suffix included.
// PREVENTS: the bridge silently discarding a component it cannot parse. The
// writer emits "10.1.0.0/24/0" and "=tcp"; the parser accepted only
// "10.1.0.0/24" and "=6" and dropped anything else without a word, so a peer's
// narrow "drop tcp to 10.1.0.0/24 port 80" became an unconditional drop of
// port 80 to EVERY address on EVERY protocol.
func TestParseNLRIJSONReadsWhatTheDaemonWrites(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	data := realNLRIJSON(t, fam,
		flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24")),
		flowspec.NewFlowSourcePrefixComponent(netip.MustParsePrefix("192.0.2.0/24")),
		flowspec.NewFlowIPProtocolComponent(6),
		flowspec.NewFlowDestPortComponent(80),
	)

	fs, err := parseNLRIJSON(fam, data)
	require.NoError(t, err, "the bridge must parse what the daemon writes: %s", data)

	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "wire-key")
	require.NoError(t, err)
	require.Len(t, terms, 1)

	assert.ElementsMatch(t, []firewall.Match{
		firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
		firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("192.0.2.0/24")},
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 80, Hi: 80}}},
	}, terms[0].Matches, "no component announced by the peer may go missing")
}

// parseAndTranslate runs the pair of steps handleFlowSpecAdd runs, and returns
// the first refusal. Both take the same branch in the bridge: the route is
// logged, counted and dropped rather than registered.
func parseAndTranslate(fam family.Family, data []byte) error {
	fs, err := parseNLRIJSON(fam, data)
	if err != nil {
		return err
	}
	_, err = translateFlowSpec(fs, flowAction{discard: true}, "proto-key")
	return err
}

// TestParseNLRIJSONResolvesEveryProtocolNameTheWriterEmits walks the writer's
// own name set instead of a sample.
//
// VALIDATES: each protocol the writer spells as a name is either resolved to
// its canonical firewall name or refused; none is silently dropped.
// PREVENTS: a protocol condition disappearing from a drop rule, which widens
// the rule beyond what the peer announced.
func TestParseNLRIJSONResolvesEveryProtocolNameTheWriterEmits(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	for _, num := range []uint8{1, 2, 6, 17, 47, 50, 51, 58, 89, 112, 132, 253} {
		data := realNLRIJSON(t, fam,
			flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24")),
			flowspec.NewFlowIPProtocolComponent(num))

		// handleFlowSpecAdd runs both steps and refuses on either, so the
		// test asks the same question of the pair rather than of one half.
		err := parseAndTranslate(fam, data)

		name, canonical := firewall.ProtocolName(num)
		if canonical {
			require.NoError(t, err, "protocol %d (%s) must translate", num, name)
			continue
		}
		require.ErrorIs(t, err, errUnknownProtocol,
			"protocol %d has no canonical name and must be refused, not dropped", num)
	}
}

// TestParseNLRIJSONRefusesAnUnreadableValue closes the silent-drop class the
// two tests above found.
//
// VALIDATES: a value the parser cannot read refuses the whole NLRI.
// PREVENTS: any future producer change widening a peer's rule by having its
// narrowing component quietly ignored. A rule ze cannot read is a rule ze must
// not enforce a looser version of.
func TestParseNLRIJSONRefusesAnUnreadableValue(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}

	for _, nlri := range []string{
		`{"destination":[["not-a-prefix"]]}`,
		`{"destination":[["10.1.0.0/24/8"]]}`,
		`{"protocol":[["=nonsense"]]}`,
		`{"destination-port":[["=eighty"]]}`,
		`{"icmp-type":[["=echo"]]}`,
	} {
		_, err := parseNLRIJSON(fam, []byte(nlri))
		assert.Error(t, err, "an unreadable NLRI must be refused, not narrowed away: %s", nlri)
	}
}

// TestParseNLRIJSONRefusesFlagNamesItCannotRead pins the fail-closed answer for
// the one component whose writer spells values as names ze cannot yet read.
//
// VALIDATES: a tcp-flags component written as flag names refuses the route.
// PREVENTS: the route being enforced without its flag condition. A FlowSpec
// SYN-flood rule stripped of "syn" drops every TCP packet to the target, which
// is the attack succeeding through the mitigation.
func TestParseNLRIJSONRefusesFlagNamesItCannotRead(t *testing.T) {
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
	data := realNLRIJSON(t, fam,
		flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24")),
		flowspec.NewFlowTCPFlagsComponent(0x02))

	_, err := parseNLRIJSON(fam, data)
	require.Error(t, err, "a component ze cannot read must refuse the route: %s", data)
	assert.NotContains(t, string(data), `"tcp-flags":[["2"]]`,
		"if the writer ever emits digits here, this test is pinning the wrong shape")
}
