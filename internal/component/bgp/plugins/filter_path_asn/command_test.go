// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
package filter_path_asn

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// showConfig is two lists and three chain references, written the three ways an
// operator can write one: the bare name, the filter-type form and the plugin
// form. All three name the same list, which is what the attachment counts have
// to see through.
const showConfig = `
bgp {
    router-id 1.2.3.4;
    session {
        asn {
            local 65000
        }
    }
    policy {
        reject-asn NO-TRANSIT {
            indirect [ 174 3356 ]
            direct [ 3356 ]
            origin [ 64512 ]
            regex [ "^65001 174 " ]
        }
        reject-asn UNUSED {
            anywhere [ 6939 ]
            nth 2 [ 3491 ]
        }
    }
    peer peer-a {
        connection {
            remote {
                ip 10.0.0.1
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65001
            }
        }
        filter {
            import [ NO-TRANSIT ]
            export [ reject-asn:NO-TRANSIT ]
        }
    }
    peer peer-b {
        connection {
            remote {
                ip 10.0.0.2
            }
            local {
                ip auto
            }
        }
        session {
            asn {
                remote 65002
            }
        }
        filter {
            import [ bgp-filter-path-asn:NO-TRANSIT ]
        }
    }
}
`

// deliverShowConfig hands showConfig to the plugin the way the engine hands a
// config over, and returns nothing: what the plugin now holds is the subject of
// every assertion below.
//
// One document for every test in this file, on purpose. The show commands answer
// about a WHOLE configuration -- which lists exist, and which peers name each
// one -- so a per-test document would make each assertion true about a config no
// operator would write.
func deliverShowConfig(t *testing.T) {
	t.Helper()

	tree, err := config.ParseTreeWithYANG(showConfig, nil)
	require.NoError(t, err, "the config must parse against the registered YANG")

	subtree := config.ExtractConfigSubtree(tree.ToMap(), "bgp")
	require.NotNil(t, subtree)
	data, err := json.Marshal(subtree)
	require.NoError(t, err)

	require.NoError(t, configure([]sdk.ConfigSection{{Root: "bgp", Data: string(data)}}))
}

// answerOf runs one command and decodes its JSON answer.
func answerOf(t *testing.T, command string, args ...string) map[string]any {
	t.Helper()

	status, payload, err := handleCommand(command, args)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)

	raw, ok := payload.(json.RawMessage)
	require.True(t, ok, "every answer is structured data, so every pipe operator can render it")

	var answer map[string]any
	require.NoError(t, json.Unmarshal(raw, &answer), "the answer must be valid JSON")
	return answer
}

// listNamed finds one list record in a `show bgp reject-asn` answer.
func listNamed(t *testing.T, answer map[string]any, name string) map[string]any {
	t.Helper()

	lists, ok := answer["lists"].([]any)
	require.True(t, ok, "the answer holds its rows under lists")
	for _, row := range lists {
		record, ok := row.(map[string]any)
		require.True(t, ok)
		if record["name"] == name {
			return record
		}
	}
	t.Fatalf("no list named %q in the answer", name)
	return nil
}

// positionsOf reads the position set one entry row carries.
func positionsOf(t *testing.T, record map[string]any) []string {
	t.Helper()

	raw, ok := record["positions"].([]any)
	require.True(t, ok, "an entry states its positions as a set")
	names := make([]string, 0, len(raw))
	for _, value := range raw {
		name, ok := value.(string)
		require.True(t, ok)
		names = append(names, name)
	}
	return names
}

// entriesOf indexes one list's entry rows by ASN, as the JSON number they carry.
func entriesOf(t *testing.T, list map[string]any) map[float64]map[string]any {
	t.Helper()

	rows, ok := list["entries"].([]any)
	require.True(t, ok, "a list states its ASNs under entries")
	byASN := make(map[float64]map[string]any, len(rows))
	for _, row := range rows {
		record, ok := row.(map[string]any)
		require.True(t, ok)
		asn, ok := record["asn"].(float64)
		require.True(t, ok, "an entry states its ASN as a number, never as text")
		byASN[asn] = record
	}
	return byASN
}

// TestShowPrintsEffectivePositionPerASN is the operator's view of a list: what
// it holds, where each ASN is refused, and which network that ASN is.
//
// The effective set rather than the blocks, because the effective set is what
// the filter acts on. An operator who wrote 3356 under two keys has one rule,
// and printing the two blocks back would make them derive the union themselves.
//
// VALIDATES: AC-24 (every ASN with its effective position set and its curated
// annotation) and AC-25 (an ASN the curated table does not know prints with an
// empty annotation, never omitted and never guessed).
// PREVENTS: an answer that lists the config back instead of the rule, and an
// unknown ASN silently dropped from the listing, which would tell the operator
// their filter holds fewer ASNs than it does.
func TestShowPrintsEffectivePositionPerASN(t *testing.T) {
	deliverShowConfig(t)

	entries := entriesOf(t, listNamed(t, answerOf(t, cmdShowRejectASN), "NO-TRANSIT"))
	require.Len(t, entries, 3, "every listed ASN appears once, whatever number of blocks named it")

	// 3356 is written under `indirect` and under `direct`, so its effective set is
	// the union of the two (AC-17), printed in path order.
	assert.Equal(t, []string{"direct", "transit", "origin"}, positionsOf(t, entries[3356]),
		"an ASN under two keys prints once, with both")
	assert.Equal(t, "Lumen Technologies", entries[3356]["network"])

	assert.Equal(t, []string{"transit", "origin"}, positionsOf(t, entries[174]),
		"`indirect` is transit plus origin")
	assert.Equal(t, "Cogent Communications (contested)", entries[174]["network"],
		"the dispute the table records reaches the operator reading the list")

	// AC-25. 64512 is a private ASN and no curated table will ever hold it.
	network, present := entries[64512]["network"]
	assert.True(t, present, "the annotation column is written for every ASN, so the column exists")
	assert.Equal(t, "", network, "an ASN the table does not know is annotated with nothing, never a guess")
	assert.Equal(t, []string{"origin"}, positionsOf(t, entries[64512]))
}

// TestShowCountsPeersPerDirection answers the question a list of ASNs cannot: is
// this list attached to anything.
//
// A configured list that no chain names refuses nothing while reading like a
// safety filter, which is the same failure the parse refuses for an empty list.
// The parse cannot see peers, so the count is what surfaces it here.
//
// VALIDATES: AC-24's peer counts, over all three spellings of a chain reference.
// PREVENTS: a count that reads only the bare form, which would report zero for
// an operator who wrote reject-asn:NAME and let them believe the list is inert.
func TestShowCountsPeersPerDirection(t *testing.T) {
	deliverShowConfig(t)

	answer := answerOf(t, cmdShowRejectASN)

	attached := listNamed(t, answer, "NO-TRANSIT")
	assert.Equal(t, float64(2), attached["import-peers"], "peer-a bare, peer-b through the plugin form")
	assert.Equal(t, float64(1), attached["export-peers"], "peer-a through the filter-type form")

	unused := listNamed(t, answer, "UNUSED")
	assert.Equal(t, float64(0), unused["import-peers"], "a list no chain names is attached to nothing")
	assert.Equal(t, float64(0), unused["export-peers"])
	assert.Contains(t, entriesOf(t, unused), float64(6939), "an unattached list is still printed in full")
}

// TestShowByNameAnswersOneList covers the second form of the listing, and the
// answer an operator gets for a name that is not there.
//
// VALIDATES: `show bgp reject-asn name <name>` answers that list alone, and an
// unknown name is an ERROR rather than an empty list.
// PREVENTS: a mistyped name answering an empty entry set, which reads as a list
// that holds no ASN rather than as a list that does not exist
// (ai/rules/principles.md).
func TestShowByNameAnswersOneList(t *testing.T) {
	deliverShowConfig(t)

	answer := answerOf(t, cmdShowRejectASNName, "NO-TRANSIT")
	assert.Equal(t, "NO-TRANSIT", answer["name"])
	assert.Len(t, entriesOf(t, answer), 3)
	assert.Equal(t, []any{"^65001 174 "}, answer["patterns"], "the regex arm is part of the list")

	_, _, err := handleCommand(cmdShowRejectASNName, []string{"NO-SUCH-LIST"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NO-SUCH-LIST")

	_, _, err = handleCommand(cmdShowRejectASNName, nil)
	require.Error(t, err, "the name is the whole question, so no name is a usage error")
}

// TestKnownTransitFreePrintsPasteableBlock is the authoring aid held to its one
// promise: what it prints is what an operator pastes.
//
// Ze ships no ASN set that decides anything, so this command is the ONLY route
// the well-known transit-free ASNs take into a config. A format that stopped
// parsing would break that route silently, and the operator would find out at
// commit. So the printed block goes back through the real config parser here.
//
// VALIDATES: AC-53 (one pasteable `indirect [ ... ];` line plus the sources and the
// curated date as comments) and AC-55 (after the paste the config holds NUMBERS,
// so a later change to the curated table cannot alter it).
// PREVENTS: a rendering change that stops the block being pasteable, and any
// coupling that would let the curated table reach into a running config.
func TestKnownTransitFreePrintsPasteableBlock(t *testing.T) {
	answer := answerOf(t, cmdShowRejectASNTransitFree)

	raw, ok := answer["block"].([]any)
	require.True(t, ok, "the block is an array of config lines")
	lines := make([]string, 0, len(raw))
	for _, value := range raw {
		line, ok := value.(string)
		require.True(t, ok)
		lines = append(lines, line)
	}

	// The provenance, as comments the config parser skips.
	assert.Contains(t, lines[0], curatedDate, "the curated date is the only staleness signal there is")
	for _, source := range curatedSources {
		assert.Contains(t, lines, "# source: "+source)
	}
	assert.Equal(t, 1, countPrefix(lines, positionKeyPasteable+" ["),
		"the ASNs are one line, the way a leaf-list is written")

	// The paste, through the real parser. The block carries its own leaf name, so
	// it goes straight under the list an operator names.
	block := strings.Join(lines, "\n            ")
	lists, err := parseLists(t, "        reject-asn TIER1 {\n            "+block+"\n        }")
	require.NoError(t, err, "the printed block must parse where an operator pastes it")

	list, ok := lists["TIER1"]
	require.True(t, ok)

	pasted := slices.Sorted(maps.Keys(list.positions))
	want := make([]uint32, 0, len(curatedTransitFree))
	for _, network := range curatedTransitFree {
		want = append(want, network.asn)
	}
	slices.Sort(want)
	require.Equal(t, want, pasted, "every curated ASN reaches the config, as a number")

	// AC-55. The pasted config is numbers, so the table can change under it.
	held := curatedTransitFree
	t.Cleanup(func() { curatedTransitFree = held })
	curatedTransitFree = []curatedNetwork{{asn: 64512, name: "invented"}}

	assert.Equal(t, want, slices.Sorted(maps.Keys(list.positions)),
		"a later edit to the curated table cannot alter a config that already holds the numbers")
}

// countPrefix counts the lines that start with prefix.
func countPrefix(lines []string, prefix string) int {
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(line, prefix) {
			count++
		}
	}
	return count
}

// TestKnownTransitFreeJSONShape holds the machine-readable half of the same
// answer, so a script builds the block instead of a person.
//
// VALIDATES: AC-54. The answer carries the curated date, both sources, and one
// record per network with its ASN as a number and its contested flag.
// PREVENTS: the set reaching a script only as the rendered block, which a script
// would have to parse back out of a config line.
func TestKnownTransitFreeJSONShape(t *testing.T) {
	answer := answerOf(t, cmdShowRejectASNTransitFree)

	assert.Equal(t, curatedDate, answer["curated"])
	sources, ok := answer["sources"].([]any)
	require.True(t, ok)
	require.Len(t, sources, len(curatedSources), "no source may be dropped from the answer")

	networks, ok := answer["networks"].([]any)
	require.True(t, ok)
	require.Len(t, networks, len(curatedTransitFree))

	contested := 0
	for i, row := range networks {
		record, ok := row.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, float64(curatedTransitFree[i].asn), record["asn"], "the ASN is a number a script can use")
		assert.Equal(t, curatedTransitFree[i].name, record["name"])
		if record["contested"] == true {
			contested++
		}
	}
	assert.Equal(t, 1, contested, "AS174 is the one entry the sources disagree about")
}

// TestShowPrintsNthPositions proves an ASN written only under an `nth` keyword
// reaches the operator's view of the list.
//
// VALIDATES: AC-24 for the one keyword whose position is a number. The ASN is
// listed, its collapsed positions are named, and an ASN no nth rule names prints
// an empty array rather than omitting the key.
// PREVENTS: an answer built from the position map alone, which would show an
// operator a shorter list than the one they configured and let them believe a
// rule is missing.
func TestShowPrintsNthPositions(t *testing.T) {
	deliverShowConfig(t)

	entries := entriesOf(t, listNamed(t, answerOf(t, cmdShowRejectASN), "UNUSED"))
	require.Len(t, entries, 2, "the nth ASN is listed beside the anywhere ASN")

	assert.Equal(t, []any{float64(2)}, entries[3491]["nth"],
		"AS3491 is written under nth 2 and the answer must say so")
	assert.Empty(t, positionsOf(t, entries[3491]),
		"and it holds no partition position, because nth is not one")

	assert.Equal(t, []any{}, entries[6939]["nth"],
		"an ASN no nth rule names prints an empty array, never a missing key")
}
