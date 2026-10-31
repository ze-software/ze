package redistribute

import (
	"context"
	"net/netip"
	"testing"

	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/core/redistevents"

	"github.com/stretchr/testify/require"
)

// TestRedistributeEBGPClass proves that `import ebgp` and `import ibgp` move
// only the best paths of their own class (spec-route-policy-0-umbrella AC-15).
// Method: one bgp batch carries an eBGP, an iBGP and an unclassified best path;
// for each rule the test reads which prefixes the ospf consumer received. Before
// the producer stamped the class, every batch named only `bgp`, so both class
// rules moved nothing.
func TestRedistributeEBGPClass(t *testing.T) {
	const (
		prefixEBGP  = "10.1.0.0/24"
		prefixIBGP  = "10.2.0.0/24"
		prefixLocal = "10.3.0.0/24"
	)
	cases := []struct {
		source string
		want   []string
	}{
		{source: redistevents.SourceNameEBGP, want: []string{prefixEBGP}},
		{source: redistevents.SourceNameIBGP, want: []string{prefixIBGP}},
		{source: "bgp", want: []string{prefixEBGP, prefixIBGP, prefixLocal}},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			resetState(t)
			bgpID := redistevents.RegisterProtocol("bgp")
			for _, name := range []string{"bgp", redistevents.SourceNameEBGP, redistevents.SourceNameIBGP} {
				require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: name, Protocol: "bgp"}))
			}
			configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
				{Source: tc.source, Destination: "ospf"},
			}))
			consumer := &stubConsumer{name: "ospf"}
			require.NoError(t, configredist.RegisterConsumer(consumer))

			batch := &redistevents.RouteChangeBatch{
				Protocol: bgpID,
				AFI:      afiIPv4,
				SAFI:     safiUnicst,
				Entries: []redistevents.RouteChangeEntry{
					{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixEBGP), BGPClass: redistevents.BGPClassExternal},
					{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixIBGP), BGPClass: redistevents.BGPClassInternal},
					{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixLocal)},
				},
			}
			handleBatch(context.Background(), skipIDs(), batch)

			var got []string
			for _, inj := range consumer.snapshotInjected() {
				got = append(got, inj.entry.Prefix)
			}
			require.Equal(t, tc.want, got, "import %s moved the wrong best paths", tc.source)
		})
	}
}

// TestReplayEBGPClass proves the replay path judges a BGP best path under its
// class the way the incremental path does (spec-route-policy-0-umbrella AC-15).
// Method: an `import ebgp` rule feeds ospf, ospf registers and is owed a
// replay, and the bgp producer answers with an eBGP, an iBGP and an
// unclassified best path under that replayID. Only the eBGP prefix may reach
// the consumer; a replay that judged the batch under its protocol name `bgp`
// would move none of them, and one that ignored the class would move all three.
func TestReplayEBGPClass(t *testing.T) {
	const prefixEBGP = "10.1.0.0/24"
	resetState(t)
	bgpID := redistevents.RegisterProtocol("bgp")
	for _, name := range []string{"bgp", redistevents.SourceNameEBGP, redistevents.SourceNameIBGP} {
		require.NoError(t, configredist.RegisterSource(configredist.RouteSource{Name: name, Protocol: "bgp"}))
	}
	configredist.SetGlobal(configredist.NewEvaluator([]configredist.ImportRule{
		{Source: redistevents.SourceNameEBGP, Destination: "ospf"},
	}))
	consumer := &stubConsumer{name: "ospf"}
	require.NoError(t, configredist.RegisterConsumer(consumer))

	coord := newReplayCoordinator()
	setReplayCoordinator(coord)
	t.Cleanup(func() { setReplayCoordinator(nil) })
	replayID, fired := coord.onConsumerRegistered(newTestBus(), "ospf")
	require.True(t, fired, "a consumer an import rule feeds is owed a replay")

	batch := &redistevents.RouteChangeBatch{
		Protocol: bgpID,
		AFI:      afiIPv4,
		SAFI:     safiUnicst,
		ReplayID: replayID,
		Entries: []redistevents.RouteChangeEntry{
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix(prefixEBGP), BGPClass: redistevents.BGPClassExternal},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.2.0.0/24"), BGPClass: redistevents.BGPClassInternal},
			{Action: redistevents.ActionAdd, Prefix: netip.MustParsePrefix("10.3.0.0/24")},
		},
	}
	handleBatch(context.Background(), skipIDs(), batch)

	var got []string
	for _, inj := range consumer.snapshotInjected() {
		got = append(got, inj.entry.Prefix)
	}
	require.Equal(t, []string{prefixEBGP}, got, "the replay moved the wrong best paths")
}
