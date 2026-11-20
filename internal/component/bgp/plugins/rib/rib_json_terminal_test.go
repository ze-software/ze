package rib

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestRIBJSONTerminalEqualsBareCommand proves that removing the RIB's `json`
// terminal word loses no route (A-11 and D-7 of
// spec-pipe-chain-honored-on-every-command-surface).
//
// METHOD: each subtest drains the source the removed terminal drained and
// builds the document that terminal built from it: one serializeRouteItem row
// per item, plus its peer and direction, under "routes", in source order. The
// bare command's answer is then compared with it. On `show bgp rib` and
// `show bgp rib protocol` the two documents are equal, so `| json`, which the
// engine applies to the bare answer, renders what the word answered. On
// `show bgp rib best` the terminal answered a second shape, so the comparison
// is the same family, prefix and winning peer per row: the `routes` shape goes,
// the routes stay.
func TestRIBJSONTerminalEqualsBareCommand(t *testing.T) {
	t.Run("show bgp rib", func(t *testing.T) {
		r := newTestRIBManager(t)
		seedShowRows(t, r, 3)
		r.ribOut[1] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
			family.IPv4Unicast: {
				"10.9.0.0/24": {Family: family.IPv4Unicast, Prefix: "10.9.0.0/24"},
			},
		})

		status, payload, err := r.handleCommand("show bgp rib", "*", nil)
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		records, streaming := payload.(sdk.Records)
		require.True(t, streaming, "the bare command answers a walk, got %T", payload)

		source, release := r.newShowSource("*", scopeSentReceived, wholeTableRead{})
		want := removedTerminalDocument(t, source, release)
		require.Len(t, want["routes"], 4, "three received routes and one sent route were stored")
		assert.Equal(t, want, roundTrip(t, drainShowRecords(t, records)))
	})

	t.Run("show bgp rib protocol", func(t *testing.T) {
		r := newTestRIBManager(t)
		attrBytes := concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001)
		peerRIB := storage.NewPeerRIB("router1:peer1")
		peerRIB.Insert(family.IPv4Unicast, attrBytes, []byte{24, 10, 0, 1})
		peerRIB.Insert(family.IPv4Unicast, attrBytes, []byte{24, 10, 0, 2})
		r.ribInPool[bmpProtocolID]["router1:peer1"] = peerRIB

		status, payload, err := r.showProtocolPipeline("bmp", nil)
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		var bare map[string]any
		require.NoError(t, json.Unmarshal(mustMarshal(t, payload), &bare))

		r.peerMu.RLock()
		source := newProtocolInboundSource(r, bmpProtocolID, "", wholeTableRead{})
		r.peerMu.RUnlock()
		want := removedTerminalDocument(t, source, source.release)
		require.Len(t, want["routes"], 2, "two protocol routes were stored")
		assert.Equal(t, want, bare)
	})

	t.Run("show bgp rib best", func(t *testing.T) {
		r := newTestRIBManager(t)
		seedBestPathFixture(t, r)

		status, payload, err := r.handleCommand("show bgp rib best", "*", nil)
		require.NoError(t, err)
		require.Equal(t, statusDone, status)
		records, streaming := payload.(sdk.Records)
		require.True(t, streaming, "the bare command answers a walk, got %T", payload)

		r.peerMu.RLock()
		source := newBestSource(r, "*", nil)
		r.peerMu.RUnlock()
		var want []bestRowIdentity
		for {
			item, ok := source.Next()
			if !ok {
				break
			}
			want = append(want, bestRowIdentity{family: item.Family.String(), prefix: item.Prefix, peer: item.Peer})
		}
		source.release()
		require.Len(t, want, 2, "two prefixes were stored, so two best paths are elected")

		rows, isList := drainShowRecords(t, records)[bestPathEnvelopeKey].([]any)
		require.True(t, isList, "the bare command answers best-path rows")
		got := make([]bestRowIdentity, 0, len(rows))
		for _, raw := range rows {
			row, isRow := raw.(map[string]any)
			require.True(t, isRow)
			famName, _ := row["family"].(string)
			prefix, _ := row["prefix"].(string)
			peer, _ := row["best-peer"].(string)
			got = append(got, bestRowIdentity{family: famName, prefix: prefix, peer: peer})
		}
		assert.Equal(t, want, got)
	})
}

// bestRowIdentity is what a best-path row and a route row of the removed
// terminal both say about one route: which family, which prefix, which peer won.
type bestRowIdentity struct {
	family string
	prefix string
	peer   string
}

// removedTerminalDocument builds, from source, the document the removed `json`
// terminal built, then gives the source back. It is the oracle of
// TestRIBJSONTerminalEqualsBareCommand and nothing else.
func removedTerminalDocument(t *testing.T, source pipelineIterator, release func()) map[string]any {
	t.Helper()
	defer release()
	rows := make([]map[string]any, 0)
	for {
		item, ok := source.Next()
		if !ok {
			break
		}
		row := serializeRouteItem(item)
		row[rowKeyPeer] = item.Peer
		row[rowKeyDirection] = item.Direction.String()
		rows = append(rows, row)
	}
	return roundTrip(t, map[string]any{"routes": rows})
}

// roundTrip answers v as JSON decodes it, so two documents built from different
// Go types compare by what a caller reads.
func roundTrip(t *testing.T, v any) map[string]any {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))
	return document
}
