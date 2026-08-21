package rpki

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orderedKeys returns the keys of a JSON object in the order the writer wrote
// them. encoding/json into a map loses that order, and the order is what the
// `| summary` expansion has to name, so the test reads the bytes instead.
func orderedKeys(t *testing.T, data any) []string {
	t.Helper()

	raw, ok := data.(json.RawMessage)
	require.True(t, ok, "the handler answers with a JSON payload")

	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	token, err := decoder.Token()
	require.NoError(t, err)
	require.Equal(t, json.Delim('{'), token, "the payload is a JSON object")

	keys := make([]string, 0, 8)
	for decoder.More() {
		token, err := decoder.Token()
		require.NoError(t, err)
		key, isKey := token.(string)
		require.True(t, isKey, "an object key is a string")
		keys = append(keys, key)

		var value json.RawMessage
		require.NoError(t, decoder.Decode(&value))
	}
	return keys
}

// twoServerPlugin builds a plugin with data in every counter the aggregate half
// reports, and two cache servers so the rows are a population rather than one
// row. A payload whose counters are all zero cannot tell a selection that kept a
// field from one that dropped it.
func twoServerPlugin() *rPKIPlugin {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))
	rp.cache.Add(makeVRP("2001:db8::/32", 48, 65002))
	rp.aspaCache.Set(65001, []uint32{65000})
	rp.aspaEnabled.Store(true)

	rp.sessions = append(rp.sessions,
		newRTRSession("192.0.2.1", 323, 100, "", rp.cache, rp.aspaCache, rp.stopCh),
		newRTRSession("192.0.2.2", 324, 50, "", rp.cache, rp.aspaCache, rp.stopCh),
	)
	return rp
}

// VALIDATES: the payload obligation of a pipe alias. `show bgp rpki` carries the
// aggregate fields and the cache server rows as SIBLINGS at one level, so a
// selection over the sibling keys can answer either half.
// PREVENTS: a bare command whose aggregate half is absent, where `| summary`
// has nothing to select and leaves the record whole.
func TestOverviewCarriesAggregatesBesideCacheRows(t *testing.T) {
	rp := twoServerPlugin()

	status, data, err := rp.overviewCommand()
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.EqualValues(t, 2, m["vrp-count"], "vrp-count is the SUM of the two family counts")
	assert.Equal(t, true, m["validation-enabled"])
	assert.EqualValues(t, 2, m["sessions-total"])
	assert.EqualValues(t, 0, m["sessions-established"])
	assert.EqualValues(t, 0, m["sessions-synced"])
	assert.Equal(t, true, m["aspa-enabled"])
	assert.EqualValues(t, 1, m["aspa-records"])

	rows := jsonArray(t, m, "cache-servers")
	require.Len(t, rows, 2, "one row for each configured cache server")
	first := jsonObject(t, rows[0])
	assert.Equal(t, "192.0.2.1", first["address"])
	assert.EqualValues(t, 323, first["port"])
	assert.Equal(t, false, first["synced"])
	assert.Contains(t, first, "state")
	assert.Contains(t, first, "version")
}

// VALIDATES: AC-12 and AC-13 together. `show bgp rpki summary` answers exactly
// the fields the bare command carries, with the same values, so `| summary`
// over the bare command reproduces the subcommand rather than approximating it.
// PREVENTS: the two answers drifting apart, which leaves `show bgp rpki |
// summary` reporting a different set of counters from `show bgp rpki summary`
// with nothing saying so.
func TestOverviewAggregatesMatchSummaryCommand(t *testing.T) {
	rp := twoServerPlugin()

	_, overview, err := rp.overviewCommand()
	require.NoError(t, err)
	_, summary, err := rp.summaryCommand()
	require.NoError(t, err)

	whole := parseJSON(t, overview)
	half := parseJSON(t, summary)
	require.NotEmpty(t, half)

	for name, value := range half {
		assert.Equal(t, value, whole[name], "field %s", name)
	}
	assert.NotContains(t, half, "cache-servers", "the summary half carries no rows")
}

// VALIDATES: the `| summary` expansion names every field `show bgp rpki summary`
// reports, in the order it reports them. A pipe alias selects and re-sequences
// and computes nothing, so a field the expansion does not name is a field the
// alias silently drops.
// PREVENTS: a counter added to the summary payload and not to the alias, where
// `show bgp rpki | summary` answers fewer fields than `show bgp rpki summary`.
func TestSummaryAliasExpansionNamesEverySummaryField(t *testing.T) {
	rp := twoServerPlugin()

	_, summary, err := rp.summaryCommand()
	require.NoError(t, err)

	selected := strings.Fields(summaryAliasExpansion)
	require.Greater(t, len(selected), 1, "the expansion names an operator and its fields")
	assert.Equal(t, "display", selected[0], "selection is what an alias may do")
	assert.Equal(t, orderedKeys(t, summary), selected[1:])
}

// VALIDATES: the aggregate half writes the field names the alias expansion is
// built from, so the one authored list is the one the payload carries.
// PREVENTS: a rename in the writer that leaves the expansion naming a key the
// payload no longer holds, which `| display` answers with an empty record.
func TestSummaryFieldNamesMatchTheWrittenPayload(t *testing.T) {
	rp := twoServerPlugin()

	_, summary, err := rp.summaryCommand()
	require.NoError(t, err)
	assert.Equal(t, summaryFieldNames, orderedKeys(t, summary))
}
