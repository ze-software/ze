// Design: docs/architecture/api/commands.md -- an override keeps the generic's meaning
//
// Related: rib_pipeline.go -- the overrides: matchFilter, firstFilter, lastFilter, countTerminal
// Related: rib_pipeline_best.go -- the same overrides over the best-path rows

package rib

import (
	"encoding/json"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/family"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// genericProbePath is a command path that declares no pipe operation, no
// answer shape and no alias, so every operation typed after it is the
// generic one of the command package's catalog.
const genericProbePath = "rib-override-generic-probe"

// Received-route attributes no other row of the fixture carries. The values
// differ from the sent route's (65000:3:4, 0002fde800000001), so a case that
// names one keeps exactly one row.
var (
	// LARGE_COMMUNITIES = [65010:7:8].
	testWireLargeCommunity = []byte{0xC0, 0x20, 0x0C, 0x00, 0x00, 0xFD, 0xF2, 0x00, 0x00, 0x00, 0x07, 0x00, 0x00, 0x00, 0x08}
	// EXTENDED_COMMUNITIES = [0002fdf300000009].
	testWireExtCommunity = []byte{0xC0, 0x10, 0x08, 0x00, 0x02, 0xFD, 0xF3, 0x00, 0x00, 0x00, 0x09}
)

// overrideCase is one line typed two ways: chain is what an operator types
// after the pipe, which the generic runs over the bare answer, and ops is the
// same operations as the engine hands them to the RIB's handler. rows is the
// number of rows the generic keeps, or the count it answers when count is set.
type overrideCase struct {
	name  string
	chain string
	ops   []sdk.PipeOperation
	rows  int
	count bool
}

// matchWords is the declared `match` operation of a pattern typed as several
// words, which the engine hands over word by word (sdk.PipeOperation.Args).
func matchWords(words ...string) []sdk.PipeOperation {
	return []sdk.PipeOperation{{Name: filterMatch, Args: words}}
}

// TestRIBOverridesKeepGenericMeaning holds each operation `show bgp rib`
// declares under a generic's name (`count`, `first`, `last`, `match`) to the
// generic's meaning (AC-17 of spec-pipe-chain-honored-on-every-command-surface):
// over the same rows, the override answers what the generic answers.
//
// METHOD: one table holds three received routes on two peers and one sent
// route, with every field a row shows. The bare `show bgp rib` answer is taken
// once; each case runs the generic chain over that answer through the command
// package's own formatter, and the RIB's override through the handler
// (compareOverridesWithGeneric). The two documents must be equal, and each
// case states how many rows or what count the generic gave, so no comparison
// is between two empty answers.
//
// The `match` cases name each value a row shows: prefix, peer, family,
// next-hop, origin, AS path, community, large and extended community on a
// sent and on a received route, MED, local preference, and the direction
// (`received`, `sent`). Two more name what only the row holds and no single
// route field does: the flag booleans beside an attribute's value (`true
// false false 100` is the MED's flags then its value), and a text that spans
// two values (`received ipv4/unicast`, the direction then the family, which
// holds on the two rows where no value sits between them and not on the row
// whose extended community does).
//
// MUTATION: make showRow leave rowKeyDirection out of the row the match stage
// searches (build the showRowJSON row from serializeRouteItem and the peer
// alone) and the `received`, `sent`, `RECEIVED` and spanning cases go red: the
// generic keeps three, one, three and two rows, the override none. Make
// matchFilter.matches compare the pattern with item.Prefix, item.Peer and the
// family alone, as a field list would, and every attribute case goes red.
func TestRIBOverridesKeepGenericMeaning(t *testing.T) {
	r := newTestRIBManager(t)
	seedOverrideMeaningRows(t, r)

	compareOverridesWithGeneric(t, r, "show bgp rib", showRowsEnvelopeKey, 4, []overrideCase{
		{"count", "| count", ribPipe("count"), 4, true},
		{"first", "| first 2", ribPipe("first", "2"), 2, false},
		{"last", "| last 2", ribPipe("last", "2"), 2, false},
		{"match then count", "| match 65001 | count", ribPipe("match", "65001", "count"), 2, true},
		{"match direction then count", "| match received | count", ribPipe("match", "received", "count"), 3, true},
		{"prefix", "| match 10.0.1.0/24", ribPipe("match", "10.0.1.0/24"), 1, false},
		{"peer", "| match 192.0.2.3", ribPipe("match", "192.0.2.3"), 1, false},
		{"family", "| match ipv4/unicast", ribPipe("match", "ipv4/unicast"), 4, false},
		{"next-hop", "| match 10.0.0.1", ribPipe("match", "10.0.0.1"), 3, false},
		{"origin igp", "| match igp", ribPipe("match", "igp"), 3, false},
		{"origin egp", "| match egp", ribPipe("match", "egp"), 1, false},
		{"as path", "| match 65001", ribPipe("match", "65001"), 2, false},
		{"community", "| match 65000:100", ribPipe("match", "65000:100"), 1, false},
		{"large community sent", "| match 65000:3:4", ribPipe("match", "65000:3:4"), 1, false},
		{"extended community sent", "| match 0002fde800000001", ribPipe("match", "0002fde800000001"), 1, false},
		{"large community received", "| match 65010:7:8", ribPipe("match", "65010:7:8"), 1, false},
		{"extended community received", "| match 0002fdf300000009", ribPipe("match", "0002fdf300000009"), 1, false},
		{"med and local-pref", "| match 100", ribPipe("match", "100"), 1, false},
		{"local-pref sent", "| match 200", ribPipe("match", "200"), 1, false},
		{"direction received", "| match received", ribPipe("match", "received"), 3, false},
		{"direction sent", "| match sent", ribPipe("match", "sent"), 1, false},
		{"case-insensitive", "| match RECEIVED", ribPipe("match", "RECEIVED"), 3, false},
		{"attribute flags then value", "| match true false false 100", matchWords("true", "false", "false", "100"), 1, false},
		{"text spanning two values", "| match received ipv4/unicast", matchWords("received", "ipv4/unicast"), 2, false},
		{"no row", "| match absent-value", ribPipe("match", "absent-value"), 0, false},
	})
}

// TestRIBBestOverridesKeepGenericMeaning holds the operations `show bgp rib
// best` declares under a generic's name to the generic's meaning, over the
// best-path rows: the command declares the same names as `show bgp rib` and
// answers another row (bestResult: family, prefix, best-peer and the
// attributes under one key, with no direction).
//
// METHOD: the table of TestRIBOverridesKeepGenericMeaning gives three best
// paths, one per received prefix, once their next hop resolves. The bare `show bgp rib best` answer is taken
// once and each case is compared as there (compareOverridesWithGeneric).
//
// The `direction` case is the one that tells the two rows apart: a best path
// is a received route, and its row does not say so, so the generic keeps no
// row for `received` and neither may the override.
//
// MUTATION: make parseBestPipelineOperations set stage.row to showRowJSON, the
// row of the other command, and the `direction` case goes red (the generic
// keeps no row, the override three), and so does `best peer then family`
// (the generic keeps one row, the override none: in that row the family
// stands before the peer).
func TestRIBBestOverridesKeepGenericMeaning(t *testing.T) {
	r := newTestRIBManager(t)
	// A best path is elected among the candidates whose next hop resolves
	// (gatherPrefixCandidatesLocked asks for the forwarding election), so the
	// one next hop every received route of the table carries is given a
	// connected route. Without it no candidate is admitted and the command
	// answers no row. It installs a process-wide cost hook, so this test
	// MUST NOT run in parallel (electionFixtureLocRIB).
	electionFixtureLocRIB(t, r, netip.MustParseAddr("10.0.0.1"))
	seedOverrideMeaningRows(t, r)

	compareOverridesWithGeneric(t, r, "show bgp rib best", bestPathEnvelopeKey, 3, []overrideCase{
		{"count", "| count", ribPipe("count"), 3, true},
		{"first", "| first 2", ribPipe("first", "2"), 2, false},
		{"last", "| last 2", ribPipe("last", "2"), 2, false},
		{"match then count", "| match igp | count", ribPipe("match", "igp", "count"), 3, true},
		{"prefix", "| match 10.0.1.0/24", ribPipe("match", "10.0.1.0/24"), 1, false},
		{"best peer", "| match 192.0.2.3", ribPipe("match", "192.0.2.3"), 1, false},
		{"as path", "| match 65001", ribPipe("match", "65001"), 2, false},
		{"community", "| match 65000:100", ribPipe("match", "65000:100"), 1, false},
		{"large community", "| match 65010:7:8", ribPipe("match", "65010:7:8"), 1, false},
		{"extended community", "| match 0002fdf300000009", ribPipe("match", "0002fdf300000009"), 1, false},
		{"direction", "| match received", ribPipe("match", "received"), 0, false},
		{"attribute flags then value", "| match true false false 100", matchWords("true", "false", "false", "100"), 1, false},
		{"best peer then family", "| match 192.0.2.3 ipv4/unicast", matchWords("192.0.2.3", "ipv4/unicast"), 1, false},
		{"no row", "| match absent-value", ribPipe("match", "absent-value"), 0, false},
	})
}

// compareOverridesWithGeneric runs each case both ways over the rows of
// command, which answers total rows under key, and requires one answer.
//
// The generic side is the chain applied to the bare answer by the command
// package's formatter. The override side is the document the handler answers
// for the declared operations. The chain ends in `| raw` so the formatter
// answers the data alone: every other format adds the `pipe` metadata that
// records the chain for a renderer. That metadata is written by the formatter
// for an override's answer and a generic's alike (partitionPipeChain collects
// it from the whole chain, declared operations included), so it is no part of
// what an override answers, and the handler's document never carries it
// (command.CheckOverrideAnswer judges the document without it).
func compareOverridesWithGeneric(t *testing.T, r *RIBManager, commandPath, key string, total int, cases []overrideCase) {
	t.Helper()
	bare := ribAnswerDocument(t, r, commandPath, nil)
	var answer map[string][]any
	require.NoError(t, json.Unmarshal([]byte(bare), &answer), bare)
	require.Len(t, answer[key], total, "the rows the bare %s answers over the fixture", commandPath)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := applyGenericChain(t, genericProbePath+" "+tc.chain+" | raw", bare)
			got := ribAnswerDocument(t, r, commandPath, tc.ops)

			if tc.count {
				var counted map[string]int
				require.NoError(t, json.Unmarshal([]byte(want), &counted), want)
				require.Len(t, counted, 1, "the generic's count is one object holding the count alone: %s", want)
				require.Equal(t, tc.rows, counted["count"], "the generic's count over the fixture")
			} else {
				var kept map[string][]any
				require.NoError(t, json.Unmarshal([]byte(want), &kept), want)
				require.Len(t, kept, 1, "the generic keeps the rows envelope alone: %s", want)
				require.Len(t, kept[key], tc.rows, "the rows the generic keeps over the fixture")
			}
			assert.JSONEq(t, want, got, "the override of %q by %s answers what the generic answers", tc.chain, commandPath)
		})
	}
}

// seedOverrideMeaningRows stores the table the two meaning tests read: on
// 192.0.2.1 a received route with every attribute and one with origin and
// next-hop alone, on 192.0.2.3 one received route with an AS path, a large
// and an extended community, and to 192.0.2.2 one sent route whose attributes
// differ from every received one.
func seedOverrideMeaningRows(t *testing.T, r *RIBManager) {
	t.Helper()
	fam := family.IPv4Unicast

	first := storage.NewPeerRIB("192.0.2.1")
	first.Insert(fam, concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001,
		testWireCommunity, testWireMED100, testWireLocalPref100), []byte{24, 10, 0, 0})
	first.Insert(fam, concatBytes(testWireOriginIGP, testWireNextHop), []byte{24, 10, 0, 1})
	r.peerMeta[1] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.1")}
	r.bgpPeers[1] = first

	third := storage.NewPeerRIB("192.0.2.3")
	third.Insert(fam, concatBytes(testWireOriginIGP, testWireNextHop, testWireASPath65001,
		testWireLargeCommunity, testWireExtCommunity), []byte{24, 10, 0, 2})
	r.peerMeta[3] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.3")}
	r.bgpPeers[3] = third

	localPref := uint32(200)
	r.peerMeta[2] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.2")}
	r.ribOut[2] = testRibOutFamilyMap(map[family.Family]map[string]*Route{
		fam: {
			"172.16.0.0/24": {
				Family:              fam,
				Prefix:              "172.16.0.0/24",
				NextHop:             "10.0.0.9",
				Origin:              new(OriginEGP),
				ASPath:              []uint32{64512},
				LocalPreference:     &localPref,
				LargeCommunities:    []attribute.LargeCommunity{{GlobalAdmin: 65000, LocalData1: 3, LocalData2: 4}},
				ExtendedCommunities: []attribute.ExtendedCommunity{{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01}},
			},
		},
	})
	t.Cleanup(func() {
		first.Release()
		third.Release()
		for _, families := range r.ribOut {
			for _, entries := range families {
				for _, entry := range entries {
					entry.release()
				}
			}
		}
	})
}

// ribAnswerDocument answers the JSON document commandPath gives for the
// declared operations, a streamed walk collected into its envelope.
func ribAnswerDocument(t *testing.T, r *RIBManager, commandPath string, operations []sdk.PipeOperation) string {
	t.Helper()
	status, payload, err := r.handleCommand(commandPath, "*", nil, operations...)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	if records, streaming := payload.(sdk.Records); streaming {
		return string(mustMarshal(t, drainShowRecords(t, records)))
	}
	return string(mustMarshal(t, payload))
}

// applyGenericChain applies the chain of line to document with the command
// package's formatter, the one every caller's answer passes.
func applyGenericChain(t *testing.T, line, document string) string {
	t.Helper()
	_, format, problem := command.ProcessPipesDefaultFormatResult(line, "json")
	require.Empty(t, problem, "the chain of %q is refused", line)
	applied, failure := format(document)
	require.Empty(t, failure, "the chain of %q fails over the rows", line)
	return applied
}
