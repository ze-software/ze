//go:build ze_core

package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	cli "github.com/ze-software/ze/internal/component/cli/client"
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/helpfmt"
)

func TestCollectCommands(t *testing.T) {
	entries := collectCommands()
	require.NotEmpty(t, entries, "collectCommands must return commands")

	for _, e := range entries {
		assert.NotEmpty(t, e.Path, "every entry must have a path")
		assert.NotEmpty(t, e.Mode, "every entry must have a mode")
		assert.NotEmpty(t, e.Description, "command %q must have a description", e.Path)
	}

	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		assert.False(t, seen[e.Path], "duplicate path %q", e.Path)
		seen[e.Path] = true
	}

	for i := 1; i < len(entries); i++ {
		assert.True(t, entries[i-1].Path <= entries[i].Path,
			"entries must be sorted: %q > %q", entries[i-1].Path, entries[i].Path)
	}
}

func TestFilterCommands(t *testing.T) {
	entries := []commandEntry{
		{Path: "show bgp peers", Description: "List all BGP peer sessions", Mode: "read-only"},
		{Path: "set bgp peer", Description: "Configure a BGP peer", Mode: "daemon"},
		{Path: "show interface", Description: "List OS network interfaces", Mode: "read-only"},
	}

	filtered := filterCommands(entries, "bgp")
	assert.Len(t, filtered, 2)
	for _, e := range filtered {
		assert.Contains(t, e.Path+e.Description, "bgp")
	}

	filtered = filterCommands(entries, "BGP")
	assert.Len(t, filtered, 2, "filter must be case-insensitive")

	filtered = filterCommands(entries, "interface")
	assert.Len(t, filtered, 1)
	assert.Equal(t, "show interface", filtered[0].Path)

	filtered = filterCommands(entries, "nonexistent")
	assert.Empty(t, filtered)
}

func TestExtractCommandFilter(t *testing.T) {
	assert.Equal(t, "bgp", extractCommandFilter([]string{"bgp", "--json"}))
	assert.Equal(t, "", extractCommandFilter([]string{"--json"}))
	assert.Equal(t, "show", extractCommandFilter([]string{"show"}))
	assert.Equal(t, "", extractCommandFilter(nil))
}

// VALIDATES: the published command catalog spells an argument placeholder as
// <id>, not as a unicode escape.
// PREVENTS: the state the gh-pages catalog reached on 2026-08-29, where every
// angle bracket in a usage line was published as a six-character escape,
// because encoding/json escapes HTML by default and this encoder had never
// said otherwise. The bytes decode to the same string, so no parser complains
// and nothing goes red. A person reading the reference is the one who pays.
func TestCommandJSONDoesNotEscapeAngleBrackets(t *testing.T) {
	entries := []commandEntry{{
		Path:        "request cache expire",
		Description: "Remove a cached message immediately.\nUsage: request cache expire <id>.",
		Mode:        "daemon",
	}}

	var out bytes.Buffer
	require.Equal(t, 0, printCommandJSON(&out, entries))

	assert.Contains(t, out.String(), "<id>",
		"the catalog must publish the placeholder as written")
	assert.NotContains(t, strings.ToLower(out.String()), "u003c",
		"an angle bracket must not reach the catalog as a unicode escape")

	var round []commandEntry
	require.NoError(t, json.Unmarshal(out.Bytes(), &round))
	require.Len(t, round, 1)
	assert.Equal(t, entries[0].Description, round[0].Description,
		"the answer must still parse back to the description it was given")
}

// mandatoryTail returns the usage line with every bracketed group removed, so
// what is left is the part of the invocation an operator cannot omit.
//
// The brackets do not nest: writeUsageToken (internal/component/command/usage.go)
// writes one pair around one group and its values.
func mandatoryTail(usage string) string {
	var kept strings.Builder
	depth := 0
	for _, r := range usage {
		switch r {
		case '[':
			depth++
		case ']':
			depth--
		default:
			if depth == 0 {
				kept.WriteRune(r)
			}
		}
	}
	return kept.String()
}

// VALIDATES: AC-1 and AC-12 -- every catalog entry that carries a wire method
// publishes a non-empty usage string and an ordered grammar, rendering the
// grammar reproduces the usage byte for byte, and `args` is unchanged.
// PREVENTS: an agent parsing angle and square brackets out of a rendered string,
// and the two projections drifting apart.
func TestHelpCommandJSONPublishesUsage(t *testing.T) {
	entries := collectCommands()
	require.NotEmpty(t, entries)

	withWireMethod := 0
	for _, e := range entries {
		if e.WireMethod == "" {
			continue
		}
		withWireMethod++
		assert.NotEmpty(t, e.Usage, "%s publishes no usage", e.Path)
		assert.NotEmpty(t, e.Grammar, "%s publishes no grammar", e.Path)
		assert.Equal(t, e.Usage, command.UsageLine(e.Grammar),
			"%s: the grammar and the usage string disagree", e.Path)
		// The path keywords open the grammar, in order. A value belongs to the
		// container that declares it, so it sits between two path keywords
		// rather than after all of them, and the rendered line is therefore not
		// a prefix match on the flat path.
		//
		// A keyword AFTER the path is a mandatory group, which is the one shape
		// that flattens into a keyword and its values rather than into a group
		// token (`ze:modifier "required"`, internal/component/command/usage.go,
		// appendGroupTokens). `debug ip ospf inject opaque scope <link|area|as>
		// id <opaque-id>` is that case. Every optional group keeps its own
		// token kind, so it can never reach this list.
		keywords := make([]string, 0, len(e.Grammar))
		for _, token := range e.Grammar {
			if token.Kind == command.UsageKeyword {
				keywords = append(keywords, token.Text)
			}
		}
		path := strings.Fields(e.Path)
		require.GreaterOrEqual(t, len(keywords), len(path),
			"%s: the grammar states fewer keywords than the command path", e.Path)
		assert.Equal(t, path, keywords[:len(path)],
			"%s: the grammar does not open with the command path", e.Path)
		for _, extra := range keywords[len(path):] {
			assert.Contains(t, mandatoryTail(e.Usage), extra,
				"%s: the keyword %q is not in the mandatory part of %q", e.Path, extra, e.Usage)
		}
	}
	assert.Positive(t, withWireMethod, "no entry carries a wire method")

	// A local command runs no wire method and has no node, so it states no
	// invocation form. The type dictionary is untouched either way.
	for _, e := range entries {
		if e.WireMethod == "" {
			assert.Empty(t, e.Grammar, "%s is local and publishes a grammar", e.Path)
		}
	}
}

// VALIDATES: the published grammar survives the JSON round trip, kind names
// included.
// PREVENTS: a catalog reader that parses the answer back seeing a kind it
// cannot name.
func TestHelpCommandJSONGrammarRoundTrips(t *testing.T) {
	entries := []commandEntry{{
		Path:       "show system sockets",
		Mode:       "daemon",
		WireMethod: "ze-show:system-sockets",
		Grammar: []command.UsageToken{
			{Text: "show", Kind: command.UsageKeyword},
			{Text: "port", Kind: command.UsageOption},
		},
		Usage: "show [port <port>]",
	}}

	var out bytes.Buffer
	require.Equal(t, 0, printCommandJSON(&out, entries))
	assert.Contains(t, out.String(), `"kind": "option"`,
		"the catalog must publish the kind as a word")

	var round []commandEntry
	require.NoError(t, json.Unmarshal(out.Bytes(), &round))
	require.Len(t, round, 1)
	assert.Equal(t, entries[0].Grammar, round[0].Grammar)
}

// TestCommandCatalogCarriesSummaryAndHelp reads the published catalog and
// asserts the two declared help texts arrive under two distinct kebab-case
// keys, each holding what its node declares.
//
// VALIDATES: AC-6 -- `ze help command --json` carries the summary and the long
// help separately.
// PREVENTS: an agent parsing one string for two answers, and the table row
// cutting a summary at the first newline.
func TestCommandCatalogCarriesSummaryAndHelp(t *testing.T) {
	entries := []commandEntry{{
		Path:        "show bgp rib",
		Mode:        "read-only",
		Description: "Show the BGP RIB.",
		LongHelp:    "The RIB answers per family.\nAdd a prefix to narrow it.",
	}, {
		// An unconverted node, whose one description still holds both halves.
		// The renderers must print what they are given rather than cut it: a
		// cut hides the defect the shape gate exists to name.
		Path:        "show bgp summary",
		Mode:        "read-only",
		Description: "Show one row per session.\nThe row carries state, ASN and uptime.",
	}}

	encoded, err := json.Marshal(entries)
	require.NoError(t, err)

	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Len(t, decoded, 2)

	assert.Equal(t, "Show the BGP RIB.", decoded[0]["description"])
	assert.Equal(t, "The RIB answers per family.\nAdd a prefix to narrow it.", decoded[0]["long-help"])
	assert.NotContains(t, decoded[0], "help",
		"`help` names the SUMMARY on the plugin boundary; the long form must not take that spelling")

	assert.NotContains(t, decoded[1], "long-help", "an undeclared long help publishes no key")

	// The table row prints the declared summary whole, with no newline cut.
	var buf bytes.Buffer
	printCommandTable(helpfmt.NewRenderWriter(&buf), entries)
	assert.Contains(t, buf.String(), "Show the BGP RIB.")
	assert.Contains(t, buf.String(), "The row carries state, ASN and uptime.",
		"the table row cut the description at its first newline")

	// The verbose page prints the summary and then the long explanation.
	buf.Reset()
	printCommandVerbose(helpfmt.NewRenderWriter(&buf), entries)
	verbose := buf.String()
	assert.Contains(t, verbose, "  Show the BGP RIB.\n")
	assert.Contains(t, verbose, "  The RIB answers per family.\n")
	assert.Contains(t, verbose, "  Add a prefix to narrow it.\n")
}

// TestCommandCatalogDerivesNeitherHelpTextFromTheOther walks the REAL tree.
// Each published key is byte-identical to the field its node declares. A
// hand-built entry proves the JSON shape. This proves the producer feeds it.
//
// It says nothing about whether a summary is short. That is the content the
// shape gate covers (AC-14), over the whole tree rather than the catalog.
func TestCommandCatalogDerivesNeitherHelpTextFromTheOther(t *testing.T) {
	tree := cli.YANGCommandTree()
	require.NotNil(t, tree, "the YANG command tree is empty, so this test proves nothing")

	checked := 0
	for _, e := range collectCommands() {
		node := findNode(tree, e.Path)
		if node == nil {
			continue
		}
		checked++
		assert.Equal(t, node.Description, e.Description,
			"command %q publishes a summary the node does not declare", e.Path)
		assert.Equal(t, node.Help, e.LongHelp,
			"command %q publishes a long help the node does not declare", e.Path)
	}
	require.Greater(t, checked, 100, "too few YANG-backed commands were compared")
}
