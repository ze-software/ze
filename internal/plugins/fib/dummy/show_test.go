// VALIDATES: honest display of `show fib dummy` and `show fib dummy counters`:
// every route row carries `programmed: "no"`, ordinary unicast routes omit
// `route-type`, discard routes carry their type, and counters carry
// `forwarding: "none"` with kebab-case JSON keys (AC-1, AC-13).
// PREVENTS: held routes being rendered as programmed, null JSON for an empty
// table, or field-name drift from the spec contract.
package fibdummy

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/internal/core/family"
)

func TestRowsSayNotProgrammed(t *testing.T) {
	since := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	f := newDummyFIB(since)

	// Empty table encodes as JSON array [], never null.
	emptyJSON, err := json.Marshal(f.routeRows())
	require.NoError(t, err)
	assert.Equal(t, "[]", string(emptyJSON))

	f.applyBatch(&sysribevents.BestChangeBatch{
		Family: family.IPv4Unicast,
		Changes: []sysribevents.BestChangeEntry{
			{
				Action:    routeaction.Add,
				Prefix:    netip.MustParsePrefix("198.51.100.0/24"),
				NextHop:   netip.MustParseAddr("192.0.2.1"),
				Protocol:  "bgp",
				RouteType: sysribevents.RouteTypeUnicast,
				TableID:   254,
			},
			{
				Action:    routeaction.Add,
				Prefix:    netip.MustParsePrefix("203.0.113.0/24"),
				Protocol:  "static",
				RouteType: sysribevents.RouteTypeBlackhole,
				TableID:   254,
			},
			{
				Action:    routeaction.Add,
				Prefix:    netip.MustParsePrefix("203.0.113.128/25"),
				Protocol:  "static",
				RouteType: sysribevents.RouteTypeUnreachable,
				TableID:   254,
			},
			{
				Action:    routeaction.Add,
				Prefix:    netip.MustParsePrefix("203.0.113.192/26"),
				Protocol:  "static",
				RouteType: sysribevents.RouteTypeProhibit,
				TableID:   254,
			},
			{
				Action:    routeaction.Add,
				Prefix:    netip.MustParsePrefix("192.0.2.0/24"),
				Interface: "eth0",
				Protocol:  "static",
				TableID:   254,
			},
		},
	})

	rows := f.routeRows()
	require.Len(t, rows, 5)

	rawRows, err := json.Marshal(rows)
	require.NoError(t, err)
	var decoded []map[string]any
	require.NoError(t, json.Unmarshal(rawRows, &decoded))
	require.Len(t, decoded, 5)

	for _, row := range decoded {
		assert.Equal(t, "no", row["programmed"], "every row must say programmed=no")
	}

	// Sorted order: 192.0.2.0/24, 198.51.100.0/24, 203.0.113.0/24, 203.0.113.128/25, 203.0.113.192/26.
	// Device-only route: interface present, next-hop and route-type omitted.
	assert.Equal(t, "192.0.2.0/24", decoded[0]["prefix"])
	assert.Equal(t, "eth0", decoded[0]["interface"])
	assert.NotContains(t, decoded[0], "next-hop")
	assert.NotContains(t, decoded[0], "route-type")
	assert.Equal(t, float64(1), decoded[0]["paths"])

	// Ordinary unicast route: next-hop present, route-type and interface omitted.
	assert.Equal(t, "198.51.100.0/24", decoded[1]["prefix"])
	assert.Equal(t, "192.0.2.1", decoded[1]["next-hop"])
	assert.NotContains(t, decoded[1], "route-type")
	assert.NotContains(t, decoded[1], "interface")
	assert.Equal(t, float64(1), decoded[1]["paths"])

	// Discard routes carry route-type and omit next-hop.
	assert.Equal(t, "blackhole", decoded[2]["route-type"])
	assert.NotContains(t, decoded[2], "next-hop")
	assert.Equal(t, "unreachable", decoded[3]["route-type"])
	assert.Equal(t, "prohibit", decoded[4]["route-type"])

	// Counters snapshot carries forwarding: "none" and kebab-case keys.
	rawCounters, err := json.Marshal(f.counters())
	require.NoError(t, err)
	var countersMap map[string]any
	require.NoError(t, json.Unmarshal(rawCounters, &countersMap))
	assert.Equal(t, "none", countersMap["forwarding"])
	assert.Equal(t, float64(5), countersMap["routes-held"])
	assert.Equal(t, float64(5), countersMap["installs"])
	assert.Equal(t, float64(0), countersMap["replaces"])
	assert.Equal(t, float64(0), countersMap["removals"])
	assert.Equal(t, float64(0), countersMap["invalid-changes"])
	assert.Equal(t, float64(0), countersMap["label-entries-refused"])
	assert.Contains(t, countersMap, "since")
}
