// Design: docs/architecture/core-design.md -- FlowSpec to firewall translation
// Related: translate.go -- valuesToPortRanges, singleValue, componentToMatch

package flowspecfirewall

import (
	"bytes"
	"log/slog"
	"net/netip"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/nlri/flowspec"
	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/metrics"
)

// A component the peer announced carries every value the peer put in it. This
// file holds the tests for the two ways that fails: a value dropped on the way
// to a match, which makes the rule WIDER than announced, and a value list
// truncated to its first entry, which enforces a rule the peer never sent.

func flowFamily() family.Family {
	return family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
}

// TestPortZeroKeepsItsMatch drives the writer's own JSON, as the daemon does.
//
// VALIDATES: a destination-port of 0 reaches the firewall term as a port match.
// PREVENTS: a peer's "drop tcp to 10.1.0.0/24 destination-port =0" installing a
// drop of ALL tcp to that prefix. Port 0 failed the `v > 0` test in
// valuesToPortRanges, the component contributed no match, and the rule was
// enforced without the one condition that narrowed it.
func TestPortZeroKeepsItsMatch(t *testing.T) {
	fam := flowFamily()
	data := realNLRIJSON(t, fam,
		flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.1.0.0/24")),
		flowspec.NewFlowIPProtocolComponent(6),
		flowspec.NewFlowDestPortComponent(0),
	)

	fs, err := parseNLRIJSON(fam, data)
	require.NoError(t, err, "the bridge must parse what the daemon writes: %s", data)

	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "port-zero-key")
	require.NoError(t, err)
	require.Len(t, terms, 1)

	assert.ElementsMatch(t, []firewall.Match{
		firewall.MatchDestinationAddress{Prefix: netip.MustParsePrefix("10.1.0.0/24")},
		firewall.MatchProtocol{Protocol: "tcp"},
		firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 0, Hi: 0}}},
	}, terms[0].Matches, "port 0 is a legal value: the rule keeps its port condition")
}

// TestPortListKeepsEveryValue covers the mixed list, where the loss is partial
// and so invisible in the term count.
//
// VALIDATES: every value of a port component becomes a range in the match.
// PREVENTS: "=0 =80" enforcing port 80 alone, which is a rule the peer never
// announced.
func TestPortListKeepsEveryValue(t *testing.T) {
	fam := flowFamily()

	matches, err := componentToMatch(flowspec.NewFlowDestPortComponent(0, 80), fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchDestinationPort{
		Ranges: []firewall.PortRange{{Lo: 0, Hi: 0}, {Lo: 80, Hi: 80}},
	}, matches[0])

	matches, err = componentToMatch(flowspec.NewFlowSourcePortComponent(0, 443), fam)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, firewall.MatchSourcePort{
		Ranges: []firewall.PortRange{{Lo: 0, Hi: 0}, {Lo: 443, Hi: 443}},
	}, matches[0])
}

// TestPortAnyZeroSplitsTerms covers type 4, which translateFlowSpec reads
// itself rather than through componentToMatch.
//
// VALIDATES: a port-any component holding 0 still produces the source and
// destination terms.
// PREVENTS: the type 4 branch leaving portAnyRanges empty, which collapses the
// two narrow terms into one term that matches every port.
func TestPortAnyZeroSplitsTerms(t *testing.T) {
	fam := flowFamily()
	fs := flowspec.NewFlowSpec(fam)
	fs.AddComponent(flowspec.NewFlowDestPrefixComponent(netip.MustParsePrefix("10.0.0.0/24")))
	fs.AddComponent(flowspec.NewFlowPortComponent(0))

	terms, err := translateFlowSpec(fs, flowAction{discard: true}, "port-any-zero-key")
	require.NoError(t, err)
	require.Len(t, terms, 2, "type 4 gives one source-port term and one destination-port term")

	want := []firewall.PortRange{{Lo: 0, Hi: 0}}
	hasSrc, hasDst := false, false
	for _, term := range terms {
		for _, m := range term.Matches {
			switch v := m.(type) {
			case firewall.MatchSourcePort:
				hasSrc = true
				assert.Equal(t, want, v.Ranges)
			case firewall.MatchDestinationPort:
				hasDst = true
				assert.Equal(t, want, v.Ranges)
			}
		}
	}
	assert.True(t, hasSrc, "one term must match source port 0")
	assert.True(t, hasDst, "one term must match destination port 0")
}

// TestComponentWithNoValueIsRefused covers the component ze cannot read at all.
//
// VALIDATES: a numeric component carrying no readable value refuses the rule.
// PREVENTS: componentToMatch returning (nil, nil), which drops the condition
// and enforces the rest of the rule against traffic the peer never named.
func TestComponentWithNoValueIsRefused(t *testing.T) {
	fam := flowFamily()

	empty := []flowspec.FlowComponent{
		flowspec.NewFlowDestPortComponent(),
		flowspec.NewFlowSourcePortComponent(),
		flowspec.NewFlowICMPTypeComponent(),
		flowspec.NewFlowTCPFlagsComponent(),
		flowspec.NewFlowDSCPComponent(),
	}
	for _, comp := range empty {
		_, err := componentToMatch(comp, fam)
		assert.ErrorIs(t, err, errUnreadableValue, "%s with no value must refuse the rule", comp.Type())
	}

	fs := flowspec.NewFlowSpec(fam)
	fs.AddComponent(flowspec.NewFlowPortComponent())
	_, err := translateFlowSpec(fs, flowAction{discard: true}, "empty-port-any")
	assert.ErrorIs(t, err, errUnreadableValue, "port-any with no value must refuse the rule")
}

// TestAlternativeListRefusesRatherThanTruncates covers the components whose
// firewall match holds one value.
//
// VALIDATES: ICMP type, TCP flags and DSCP refuse a list of alternatives.
// PREVENTS: "icmp-type =3 =8" enforcing type 3 alone. The peer asked for both,
// ze enforced one, and nothing said so.
func TestAlternativeListRefusesRatherThanTruncates(t *testing.T) {
	fam := flowFamily()

	lists := []flowspec.FlowComponent{
		flowspec.NewFlowICMPTypeComponent(3, 8),
		flowspec.NewFlowTCPFlagsComponent(0x02, 0x10),
		flowspec.NewFlowDSCPComponent(46, 10),
	}
	for _, comp := range lists {
		_, err := componentToMatch(comp, fam)
		assert.ErrorIs(t, err, errUnsupportedComponent, "%s listing alternatives must refuse the rule", comp.Type())
	}
}

// TestValueTooLargeForTheMatchIsRefused covers the value that does not fit.
//
// VALIDATES: a DSCP above 63 and a port above 65535 refuse the rule.
// PREVENTS: uint8(vals[0]) truncating a DSCP of 64 to 0, which matches every
// packet marked best-effort, and a wire component with a four-octet port value
// losing its match.
func TestValueTooLargeForTheMatchIsRefused(t *testing.T) {
	fam := flowFamily()

	_, err := componentToMatch(flowspec.NewFlowDSCPComponent(64), fam)
	assert.ErrorIs(t, err, errUnsupportedComponent, "DSCP is six bits: 64 does not fit")

	_, err = componentToMatch(widePortComponent(t, 0x00010000), fam)
	assert.ErrorIs(t, err, errUnreadableValue, "a port above 65535 must refuse the rule")
}

// widePortComponent builds a destination-port component whose single value is
// encoded in four octets, which the constructors cannot express and only the
// wire can carry. The operator octet is end-of-list, equality, length code 2
// (4 octets), and the NLRI is parsed by the daemon's own decoder.
func widePortComponent(t *testing.T, value uint32) flowspec.FlowComponent {
	t.Helper()
	nlri := []byte{
		6, // NLRI length, RFC 8955 Section 4.1
		byte(flowspec.FlowDestPort),
		0x80 | 0x20 | 0x01,
		byte(value >> 24), byte(value >> 16), byte(value >> 8), byte(value),
	}
	fs, err := flowspec.ParseFlowSpec(flowFamily(), nlri)
	require.NoError(t, err)
	require.Len(t, fs.Components(), 1)
	return fs.Components()[0]
}

// TestTCPFlagsListSurvivesTheJSONParser closes the parser end of the same hole.
//
// VALIDATES: parseNLRIJSON carries every tcp-flags value into the component, so
// the refusal above can see the list.
// PREVENTS: the parser keeping vals[0] and dropping the rest, which produced a
// single-value component that translated cleanly into a rule matching one flag
// combination out of the several the peer sent.
func TestTCPFlagsListSurvivesTheJSONParser(t *testing.T) {
	fam := flowFamily()

	fs, err := parseNLRIJSON(fam, []byte(`{"destination":[["10.1.0.0/24"]],"tcp-flags":[["=2","=16"]]}`))
	require.NoError(t, err)

	comp, ok := findComponent(fs, flowspec.FlowTCPFlags)
	require.True(t, ok, "the tcp-flags component must reach the FlowSpec")
	assert.Len(t, extractNumericValues(comp), 2, "both flag values must survive the parser")

	_, err = translateFlowSpec(fs, flowAction{discard: true}, "tcp-flags-key")
	assert.ErrorIs(t, err, errUnsupportedComponent, "a list of flag alternatives refuses the rule")
}

func findComponent(fs *flowspec.FlowSpec, typ flowspec.FlowComponentType) (flowspec.FlowComponent, bool) {
	for _, comp := range fs.Components() {
		if comp.Type() == typ {
			return comp, true
		}
	}
	return nil, false
}

// TestPortAnyRefusedThroughComponentToMatch pins the guard on the type 4 case.
//
// VALIDATES: componentToMatch refuses type 4 rather than answering for the
// destination alone.
// PREVENTS: a future caller that skips translateFlowSpec's expansion silently
// enforcing half of a source-OR-destination rule.
func TestPortAnyRefusedThroughComponentToMatch(t *testing.T) {
	_, err := componentToMatch(flowspec.NewFlowPortComponent(80), flowFamily())
	assert.ErrorIs(t, err, errUnsupportedComponent)
}

// TestRouteWithNoActionIsCountedAndLogged covers the refusal that left no trace.
//
// VALIDATES: a FlowSpec route carrying no traffic action moves
// ze_flowspec_rules_refused_total with reason "no-action" and writes a log line.
// PREVENTS: handleFlowSpecAdd returning early on it. metrics.go promises every
// refusal is visible, refusedReasonNoAction had no producer at all, and the
// operator had no way to see that a peer's route was doing nothing.
func TestRouteWithNoActionIsCountedAndLogged(t *testing.T) {
	reg := newReasonRegistry()
	previous := bridgeMetricsPtr.Load()
	t.Cleanup(func() { bridgeMetricsPtr.Store(previous) })
	bindMetrics(reg)

	var logged bytes.Buffer
	b := newBridge(slog.New(slog.NewTextHandler(&logged, nil)))

	event := `{
		"type": "update",
		"peer": {"address": "10.0.0.1"},
		"extended-communities": ["target:65000:100"],
		"ipv4/flow": [{
			"action": "add",
			"nlri": {"destination": [["10.1.0.0/24"]]}
		}]
	}`
	b.handleUpdate(event, "10.0.0.1")

	assert.Nil(t, b.rules.buildTable(), "a route with no action installs nothing")
	assert.Equal(t, 1, reg.count(refusedReasonNoAction), "the refusal must reach the counter")
	assert.Contains(t, logged.String(), "rule refused", "the refusal must reach the log")
}

// reasonRegistry is a metrics.Registry that tallies CounterVec .Inc() by its
// first label value, which is the refusal reason this bridge records.
type reasonRegistry struct {
	metrics.NopRegistry
	mu     sync.Mutex
	counts map[string]int
}

func newReasonRegistry() *reasonRegistry { return &reasonRegistry{counts: map[string]int{}} }

func (r *reasonRegistry) CounterVec(_, _ string, _ []string) metrics.CounterVec {
	return &reasonVec{r: r}
}

func (r *reasonRegistry) count(reason string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.counts[reason]
}

type reasonVec struct{ r *reasonRegistry }

func (v *reasonVec) With(labels ...string) metrics.Counter {
	reason := ""
	if len(labels) > 0 {
		reason = labels[0]
	}
	return &reasonCounter{r: v.r, reason: reason}
}

func (v *reasonVec) Delete(_ ...string) bool { return false }

type reasonCounter struct {
	r      *reasonRegistry
	reason string
}

func (c *reasonCounter) Inc() { c.Add(1) }

func (c *reasonCounter) Add(f float64) {
	c.r.mu.Lock()
	c.r.counts[c.reason] += int(f)
	c.r.mu.Unlock()
}
