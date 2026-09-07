//go:build ze_core && ze_bgp

// VALIDATES: AC-3. `ze help command --json` names a command only a PLUGIN
// provides, and reports the shape, the column order and the address fields that
// plugin declares. The path reaches no YANG command tree and no local command
// registry, so before this the catalog answered that the command did not exist.
// PREVENTS: a published catalog that silently omits every purely
// plugin-provided command. It answered 270 commands at ze_core,ze_bgp and named
// no `show bgp rpki` path, while the plugin declared a shape and a column order
// for four of them.
//
// The file is tagged ze_bgp because bgp-rpki and bgp-adj-rib-in are behind that
// gate. The normal unit run carries every feature tag
// (internal/le/gotoolchain, Toolchain.TestTags).

package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHelpCommandNamesAPluginCommand(t *testing.T) {
	var out bytes.Buffer
	require.Equal(t, 0, renderHelpCommand(&out, []string{flagJSON}))

	var entries []commandEntry
	require.NoError(t, json.Unmarshal(out.Bytes(), &entries))

	byPath := make(map[string]commandEntry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}

	// bgp-rpki declares all three in commandDecls()
	// (internal/component/bgp/plugins/rpki/rpki.go). No YANG node carries the
	// path and no builtin RPC answers it, so the entry can only have come from
	// registry.Registration.Commands.
	const path = "show bgp rpki roa"
	entry, found := byPath[path]
	require.True(t, found, "the catalog names no %q", path)
	assert.Equal(t, "tab", entry.AnswerShape)
	assert.Equal(t, [][]string{{"prefix", "max-length", "asn"}}, entry.ColumnOrders)
	assert.Equal(t, []string{"prefix"}, entry.AddressFields)
	assert.NotEmpty(t, entry.Description, "%q publishes no summary", path)
	assert.NotEmpty(t, entry.Operators, "%q publishes no operator", path)

	// A hidden declaration stays out. The daemon keeps it out of
	// VisibleCommandEntries and out of completion, so a catalog an operator
	// reads would be the only surface offering it.
	_, hidden := byPath["request bgp adj-rib-in claim-replay"]
	assert.False(t, hidden, "a hidden plugin command reached the published catalog")

}

// VALIDATES: AC-2. The catalog reports a pipe ALIAS a plugin declares, which
// until now reached the daemon's Stage 1 message alone and so reached no reader
// outside a running daemon. bgp-rpki puts `summary` on `show bgp rpki`
// (internal/component/bgp/plugins/rpki/rpki.go, pipeDecls).
// PREVENTS: the published catalog listing a plugin's commands without the names
// they answer to, which is the second of the two deferral rows this spec exists
// to close.
func TestHelpCommandReportsAPluginDeclaredAlias(t *testing.T) {
	var out bytes.Buffer
	require.Equal(t, 0, renderHelpCommand(&out, []string{flagJSON}))

	var entries []commandEntry
	require.NoError(t, json.Unmarshal(out.Bytes(), &entries))

	byPath := make(map[string]commandEntry, len(entries))
	for _, entry := range entries {
		byPath[entry.Path] = entry
	}

	const path = "show bgp rpki"
	entry, found := byPath[path]
	require.True(t, found, "the catalog names no %q", path)
	require.Len(t, entry.Aliases, 1, "%q publishes %d aliases", path, len(entry.Aliases))
	assert.Equal(t, "summary", entry.Aliases[0].Name)
	assert.NotEmpty(t, entry.Aliases[0].Description, "the alias publishes no summary")
	assert.NotEmpty(t, entry.Aliases[0].Expansion,
		"the alias publishes no expansion, which is the whole of what the name does")

	// The barrier holds. An alias sits on ONE command path, and the plugin's
	// own commands below it answer to no inherited name
	// (internal/component/command/alias.go, aliasBarriers).
	for _, below := range []string{"show bgp rpki roa", "show bgp rpki summary"} {
		child, named := byPath[below]
		require.True(t, named, "the catalog names no %q", below)
		assert.Empty(t, child.Aliases, "%q inherited an alias that sits on %q", below, path)
	}
}
