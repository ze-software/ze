package rpki

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestPlugin() *rPKIPlugin {
	return &rPKIPlugin{
		cache:      newROACache(),
		aspaCache:  newASPACache(),
		validateCh: make(chan validationRequest, 64),
		stopCh:     make(chan struct{}),
	}
}

func parseJSON(t *testing.T, data any) map[string]any {
	t.Helper()
	var b []byte
	switch d := data.(type) {
	case string:
		b = []byte(d)
	default:
		var err error
		b, err = json.Marshal(d)
		require.NoError(t, err)
	}
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	return m
}

func jsonArray(t *testing.T, m map[string]any, key string) []any {
	t.Helper()
	arr, ok := m[key].([]any)
	require.True(t, ok, "%s should be an array", key)
	return arr
}

func jsonObject(t *testing.T, v any) map[string]any {
	t.Helper()
	obj, ok := v.(map[string]any)
	require.True(t, ok, "value should be an object")
	return obj
}

func TestStatusCommandEmpty(t *testing.T) {
	rp := newTestPlugin()
	status, data, err := rp.statusCommand()
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.Equal(t, true, m["running"])
	assert.EqualValues(t, 0, m["vrp-count-ipv4"])
	assert.EqualValues(t, 0, m["vrp-count-ipv6"])
	assert.EqualValues(t, 0, m["sessions"])
	assert.Equal(t, false, m["aspa-enabled"])
}

func TestStatusCommandWithSessions(t *testing.T) {
	rp := newTestPlugin()
	sess := newRTRSession("192.0.2.1", 323, 100, "", rp.cache, rp.aspaCache, rp.stopCh)
	rp.sessions = append(rp.sessions, sess)

	status, data, err := rp.statusCommand()
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.EqualValues(t, 1, m["sessions"])

	servers := jsonArray(t, m, "cache-servers")
	require.Len(t, servers, 1)

	srv := jsonObject(t, servers[0])
	assert.Equal(t, "192.0.2.1", srv["address"])
	assert.EqualValues(t, 323, srv["port"])
	assert.Equal(t, "idle", srv["state"])
}

func TestCacheCommandEmpty(t *testing.T) {
	rp := newTestPlugin()
	status, data, err := rp.cacheCommand()
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	servers := jsonArray(t, m, "cache-servers")
	assert.Empty(t, servers)
}

func TestCacheCommandWithSession(t *testing.T) {
	rp := newTestPlugin()
	sess := newRTRSession("198.51.100.1", 8282, 50, "", rp.cache, rp.aspaCache, rp.stopCh)
	rp.sessions = append(rp.sessions, sess)

	status, data, err := rp.cacheCommand()
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	servers := jsonArray(t, m, "cache-servers")
	require.Len(t, servers, 1)

	srv := jsonObject(t, servers[0])
	assert.Equal(t, "198.51.100.1", srv["address"])
	assert.EqualValues(t, 8282, srv["port"])
	assert.EqualValues(t, 50, srv["preference"])
	assert.Equal(t, "idle", srv["state"])
	assert.EqualValues(t, 3600, srv["refresh-interval"])
	assert.EqualValues(t, 600, srv["retry-interval"])
	assert.EqualValues(t, 7200, srv["expire-interval"])
}

func TestRoaCommandCounts(t *testing.T) {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))
	rp.cache.Add(makeVRP("2001:db8::/32", 48, 65003))

	status, data, err := rp.roaCommand(nil)
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.EqualValues(t, 2, m["total-vrps"])
	assert.EqualValues(t, 1, m["ipv4-vrps"])
	assert.EqualValues(t, 1, m["ipv6-vrps"])

	entries := jsonArray(t, m, "entries")
	assert.Len(t, entries, 2)
}

func TestRoaLookupCommand(t *testing.T) {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65099))

	status, data, err := rp.roaCommand([]string{"10.1.0.0/24"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.Equal(t, "10.1.0.0/24", m["prefix"])
	assert.EqualValues(t, 2, m["covering-vrps"])
	assert.Equal(t, true, m["covered"])

	entries := jsonArray(t, m, "entries")
	require.Len(t, entries, 2)
	for _, raw := range entries {
		e := jsonObject(t, raw)
		assert.Equal(t, "10.0.0.0/8", e["prefix"], "entry prefix should be the VRP prefix, not the query")
	}
}

func TestRoaLookupNotCovered(t *testing.T) {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))

	status, data, err := rp.roaCommand([]string{"172.16.0.0/16"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.Equal(t, false, m["covered"])
	assert.EqualValues(t, 0, m["covering-vrps"])
}

func TestRoaInvalidPrefix(t *testing.T) {
	rp := newTestPlugin()
	status, _, err := rp.roaCommand([]string{"not-a-prefix"})
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid prefix")
}

func TestSummaryCommand(t *testing.T) {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))
	rp.aspaCache.Set(65001, []uint32{65000})
	rp.aspaEnabled.Store(true)

	sess := newRTRSession("192.0.2.1", 323, 100, "", rp.cache, rp.aspaCache, rp.stopCh)
	rp.sessions = append(rp.sessions, sess)

	status, data, err := rp.summaryCommand()
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.EqualValues(t, 1, m["vrp-count"])
	assert.Equal(t, true, m["validation-enabled"])
	assert.EqualValues(t, 1, m["sessions-total"])
	assert.EqualValues(t, 0, m["sessions-established"])
	assert.Equal(t, true, m["aspa-enabled"])
	assert.EqualValues(t, 1, m["aspa-records"])
}

func TestValidateCommandValid(t *testing.T) {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))

	status, data, err := rp.validateCommand([]string{"10.1.0.0/24", "65001"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.Equal(t, "10.1.0.0/24", m["prefix"])
	assert.EqualValues(t, 65001, m["origin-asn"])
	assert.Equal(t, "valid", m["state"])
}

func TestValidateCommandInvalid(t *testing.T) {
	rp := newTestPlugin()
	rp.cache.Add(makeVRP("10.0.0.0/8", 24, 65001))

	status, data, err := rp.validateCommand([]string{"10.1.0.0/24", "65099"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.Equal(t, "invalid", m["state"])
}

func TestValidateCommandNotFound(t *testing.T) {
	rp := newTestPlugin()

	status, data, err := rp.validateCommand([]string{"172.16.0.0/16", "65001"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.Equal(t, "not-found", m["state"])
}

func TestValidateCommandBadPrefix(t *testing.T) {
	rp := newTestPlugin()
	status, _, err := rp.validateCommand([]string{"bad", "65001"})
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid prefix")
}

func TestValidateCommandBadASN(t *testing.T) {
	rp := newTestPlugin()
	status, _, err := rp.validateCommand([]string{"10.0.0.0/8", "notanumber"})
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ASN")
}

func TestValidateCommandMissingArgs(t *testing.T) {
	rp := newTestPlugin()
	status, _, err := rp.validateCommand([]string{"10.0.0.0/8"})
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usage:")
}

func TestAspaCommandEmpty(t *testing.T) {
	rp := newTestPlugin()
	status, data, err := rp.aspaCommand(nil)
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.EqualValues(t, 0, m["total-records"])
	assert.Equal(t, false, m["enabled"])
}

func TestAspaCommandWithRecords(t *testing.T) {
	rp := newTestPlugin()
	rp.aspaEnabled.Store(true)
	rp.aspaCache.Set(65001, []uint32{65000, 65002})
	rp.aspaCache.Set(65003, []uint32{65004})

	status, data, err := rp.aspaCommand(nil)
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.EqualValues(t, 2, m["total-records"])
	assert.Equal(t, true, m["enabled"])

	entries := jsonArray(t, m, "entries")
	assert.Len(t, entries, 2)
}

func TestAspaCommandLookupFound(t *testing.T) {
	rp := newTestPlugin()
	rp.aspaCache.Set(65001, []uint32{65000, 65002})

	status, data, err := rp.aspaCommand([]string{"65001"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.EqualValues(t, 65001, m["customer-asn"])
	assert.Equal(t, true, m["found"])

	// The providers sit on the matched row under "entries", where the no-argument
	// branch also writes them, rather than at the top level of the answer.
	entries := jsonArray(t, m, "entries")
	require.Len(t, entries, 1)
	row := jsonObject(t, entries[0])
	assert.EqualValues(t, 65001, row["customer-asn"])
	providers := jsonArray(t, row, "providers")
	assert.Len(t, providers, 2)
}

func TestAspaCommandLookupNotFound(t *testing.T) {
	rp := newTestPlugin()

	status, data, err := rp.aspaCommand([]string{"65999"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)

	m := parseJSON(t, data)
	assert.Equal(t, false, m["found"])
}

func TestAspaCommandBadASN(t *testing.T) {
	rp := newTestPlugin()
	status, _, err := rp.aspaCommand([]string{"notanumber"})
	assert.Equal(t, statusError, status)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ASN")
}

// jsonKeys returns an object's field names in sorted order, so two answers can be
// compared on the names they use rather than on the order a range produces.
func jsonKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// VALIDATES: AC-15 -- "show bgp rpki aspa <customer-asn>" answers a row set under
// "entries", in the same spelling the no-argument branch uses.
// PREVENTS: the lookup branch drifting back to a bare {customer-asn, found,
// providers} object, which no single answer-shape declaration can describe, so an
// operator such as `| count` would answer one thing for the cache dump and
// something else for a lookup of one customer.
//
// TestAspaLookupAnswersRows proves AC-15: "show bgp rpki aspa" answers one shape
// whatever its argument, with its rows under "entries" in both branches. The method
// is to read BOTH branches and compare the row they write for the same customer, so
// the test stays honest if a later change moves the cache-dump branch instead of the
// lookup one. A test that only read the lookup answer would go green on two branches
// that disagree.
//
// The providers are compared as a set: aSPACache stores them in a map, so the order
// each branch writes is a range order rather than a promise (plan/journal/
// output-not-byte-stable.md, row dated 2026-08-24 for this plugin).
func TestAspaLookupAnswersRows(t *testing.T) {
	rp := newTestPlugin()
	rp.aspaEnabled.Store(true)
	rp.aspaCache.Set(65001, []uint32{65000, 65002})
	rp.aspaCache.Set(65003, []uint32{65004})

	_, dumpData, err := rp.aspaCommand(nil)
	require.NoError(t, err)
	var dumpRow map[string]any
	for _, entry := range jsonArray(t, parseJSON(t, dumpData), "entries") {
		row := jsonObject(t, entry)
		if row["customer-asn"] == float64(65001) {
			dumpRow = row
		}
	}
	require.NotNil(t, dumpRow, "the no-argument answer holds no row for customer 65001")

	status, lookupData, err := rp.aspaCommand([]string{"65001"})
	require.NoError(t, err)
	assert.Equal(t, statusDone, status)
	lookup := parseJSON(t, lookupData)
	assert.EqualValues(t, 65001, lookup["customer-asn"])
	assert.Equal(t, true, lookup["found"])

	rows := jsonArray(t, lookup, "entries")
	require.Len(t, rows, 1)
	lookupRow := jsonObject(t, rows[0])
	assert.Equal(t, jsonKeys(dumpRow), jsonKeys(lookupRow),
		"the lookup row and the cache-dump row must spell their fields the same way")
	assert.EqualValues(t, dumpRow["customer-asn"], lookupRow["customer-asn"])
	assert.ElementsMatch(t, jsonArray(t, dumpRow, "providers"), jsonArray(t, lookupRow, "providers"))

	// A customer with no ASPA record answers no rows. "found" is what separates it
	// from an empty cache, which the row count alone cannot say.
	_, missData, err := rp.aspaCommand([]string{"65999"})
	require.NoError(t, err)
	miss := parseJSON(t, missData)
	assert.EqualValues(t, 65999, miss["customer-asn"])
	assert.Equal(t, false, miss["found"])
	assert.Empty(t, jsonArray(t, miss, "entries"))
}

func TestHandleCommandDispatch(t *testing.T) {
	rp := newTestPlugin()

	tests := []struct {
		cmd  string
		args []string
		ok   bool
	}{
		{"show bgp rpki", nil, true},
		{"show bgp rpki status", nil, true},
		{"show bgp rpki cache", nil, true},
		{"show bgp rpki roa", nil, true},
		{"show bgp rpki summary", nil, true},
		{"request bgp rpki validate", []string{"10.0.0.0/8", "65001"}, true},
		{"show bgp rpki aspa", nil, true},
		{"rpki unknown", nil, false},
	}
	for _, tt := range tests {
		status, _, err := rp.handleCommand(tt.cmd, tt.args)
		if tt.ok {
			assert.Equal(t, statusDone, status, "cmd %s", tt.cmd)
			assert.NoError(t, err, "cmd %s", tt.cmd)
		} else {
			assert.Equal(t, statusError, status, "cmd %s", tt.cmd)
			assert.Error(t, err, "cmd %s", tt.cmd)
		}
	}
}
