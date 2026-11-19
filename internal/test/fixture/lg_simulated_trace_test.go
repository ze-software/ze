package fixture

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chaosscenario "github.com/ze-software/ze/internal/chaos/scenario"
)

// lgSimulatedTraceSample is a trace graph in the text form
// renderTraceGraphText writes (internal/component/lg/graph_trace.go). Its
// lines are those TestTraceGraphTextIsOrdered expects of that function, for
// the same four routers, so this reader is tested on the producer's own shape.
const lgSimulatedTraceSample = "mode trace\n" +
	"node router 198.51.100.1 exit\n" +
	"node router 198.51.100.2 exit\n" +
	"node router 198.51.100.3\n" +
	"node router 198.51.100.4\n" +
	"node AS64500\n" +
	"node AS65101 name=Transit A\n" +
	"edge 198.51.100.4 -> 198.51.100.3\n" +
	"edge 198.51.100.1 -> AS65101\n" +
	"edge 198.51.100.2 -> AS65102\n" +
	"trace ipv4/unicast 198.18.0.0/24 198.51.100.1 routers=198.51.100.1 end=exit-own-address" +
	" next-hop=198.51.100.1 as-path=65101,64500" +
	" says: leaves the AS at 198.51.100.1, which announces its own address as next hop\n" +
	"trace ipv4/unicast 198.18.0.0/24 198.51.100.2 routers=198.51.100.2 end=exit-external-address" +
	" next-hop=203.0.113.9 as-path=65102,64500" +
	" says: leaves the AS at 198.51.100.2 toward 203.0.113.9, the address of no router of the AS\n" +
	"trace ipv4/unicast 198.18.0.0/24 198.51.100.4 routers=198.51.100.4,198.51.100.3,198.51.100.1" +
	" end=exit-own-address next-hop=198.51.100.1 as-path=65101,64500" +
	" says: leaves the AS at 198.51.100.1, which announces its own address as next hop\n"

func lgSimulatedTraceAddrs(addresses ...string) []netip.Addr {
	out := make([]netip.Addr, 0, len(addresses))
	for _, address := range addresses {
		out = append(out, netip.MustParseAddr(address))
	}
	return out
}

// lgSimulatedTraceModel is the model's walk the sample's last line answers:
// the reflector's client, the reflector, the exit.
func lgSimulatedTraceModel() chaosscenario.Trace {
	return chaosscenario.Trace{
		Routers: lgSimulatedTraceAddrs("198.51.100.4", "198.51.100.3", "198.51.100.1"),
		End:     chaosscenario.TraceEndExitOwnAddress,
		NextHop: netip.MustParseAddr("198.51.100.1"),
		ASPath:  []uint32{65101, 64500},
	}
}

// VALIDATES: the reader takes each `trace` line by its fields (the routers in
// order, the end, the next hop, the AS path) and the routers marked exit, and
// the comparison accepts the model's walk across three routers.
// PREVENTS: a comparison of whole lines, which a reworded sentence would break.
func TestLGSimulatedTraceReadsTheFields(t *testing.T) {
	prefix := netip.MustParsePrefix("198.18.0.0/24")
	reworded := strings.ReplaceAll(lgSimulatedTraceSample, "leaves the AS at", "goes out of the AS by")

	for _, text := range []string{lgSimulatedTraceSample, reworded} {
		read, err := lgSimulatedReadTrace(text, prefix)
		require.NoError(t, err)
		assert.Equal(t, []string{"198.51.100.1", "198.51.100.2"}, read.exits)
		require.Len(t, read.traces, 3)

		want, err := lgSimulatedTraceWant(lgSimulatedTraceModel())
		require.NoError(t, err)
		assert.Empty(t, lgSimulatedTraceProblem(read.traces["198.51.100.4"], want))
	}
}

// VALIDATES: each field that differs from the model's walk is named: a router
// missing from the chain, another end, another next hop, another AS path.
// PREVENTS: a comparison that passes a trace the model did not compute.
func TestLGSimulatedTraceProblemNamesTheDifference(t *testing.T) {
	read, err := lgSimulatedReadTrace(lgSimulatedTraceSample, netip.MustParsePrefix("198.18.0.0/24"))
	require.NoError(t, err)
	got := read.traces["198.51.100.4"]

	cases := []struct {
		name   string
		change func(*chaosscenario.Trace)
		says   string
	}{
		{"the chain skips the reflector", func(trace *chaosscenario.Trace) {
			trace.Routers = lgSimulatedTraceAddrs("198.51.100.4", "198.51.100.1")
		}, "crosses the routers"},
		{"another end", func(trace *chaosscenario.Trace) {
			trace.End = chaosscenario.TraceEndExitExternalAddress
		}, "ends as"},
		{"another next hop", func(trace *chaosscenario.Trace) {
			trace.NextHop = netip.MustParseAddr("203.0.113.9")
		}, "leaves by the next hop"},
		{"another AS path", func(trace *chaosscenario.Trace) {
			trace.ASPath = []uint32{65102, 64500}
		}, "carries the AS path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := lgSimulatedTraceModel()
			tc.change(&changed)
			want, err := lgSimulatedTraceWant(changed)
			require.NoError(t, err)
			assert.Contains(t, lgSimulatedTraceProblem(got, want), tc.says)
		})
	}
}

// VALIDATES: every end the model can state has its `end=` word, a trace with
// no end is refused, and a prefix that originates inside the AS expects no AS
// path on its line.
// PREVENTS: an end the model gains later being compared as an empty word.
func TestLGSimulatedTraceEndNames(t *testing.T) {
	names := map[chaosscenario.TraceEnd]string{
		chaosscenario.TraceEndExitOwnAddress:      "exit-own-address",
		chaosscenario.TraceEndExitExternalAddress: "exit-external-address",
		chaosscenario.TraceEndOriginInside:        "origin-inside",
	}
	for end, name := range names {
		got, err := lgSimulatedTraceEndName(end)
		require.NoError(t, err)
		assert.Equal(t, name, got)
	}
	_, err := lgSimulatedTraceEndName(chaosscenario.TraceEndUnspecified)
	require.Error(t, err)
	_, err = lgSimulatedTraceEndName(chaosscenario.TraceEnd(200))
	require.Error(t, err)

	inside, err := lgSimulatedTraceWant(chaosscenario.Trace{
		Routers: lgSimulatedTraceAddrs("198.51.100.2"),
		End:     chaosscenario.TraceEndOriginInside,
		NextHop: netip.MustParseAddr("198.51.100.2"),
	})
	require.NoError(t, err)
	assert.Empty(t, inside.asPath)
	assert.Equal(t, "origin-inside", inside.end)
}

// VALIDATES: a text that is not a trace graph, a trace of another prefix and a
// trace line with no stated end are each refused.
// PREVENTS: an error page or another prefix's graph read as an empty answer.
func TestLGSimulatedTraceRefusesWhatIsNotTheGraph(t *testing.T) {
	prefix := netip.MustParsePrefix("198.18.0.0/24")

	_, err := lgSimulatedReadTrace("No trace: the query answered no route for this prefix. Peers: 11, routers of the AS: 5, routes answered: 0.\n", prefix)
	require.ErrorContains(t, err, "does not start with")

	_, err = lgSimulatedReadTrace(lgSimulatedTraceSample, netip.MustParsePrefix("198.18.1.0/24"))
	require.ErrorContains(t, err, "holds a trace of 198.18.0.0/24")

	silent := strings.ReplaceAll(lgSimulatedTraceSample, " says: ", " ")
	_, err = lgSimulatedReadTrace(silent, prefix)
	require.ErrorContains(t, err, "states no end")
}
