// Design: docs/plugin-development/metrics.md -- reject-asn operator signal
package filter_path_asn

import (
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/metrics"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// installMetrics binds a real Prometheus registry to the plugin and returns it.
//
// The real registry rather than a fake, because this test is about the exposed
// series: their name, their label names, their label values and how many of them
// there are. A fake would answer whatever the plugin asked it for and could not
// see cardinality at all.
func installMetrics(t *testing.T) *metrics.PrometheusRegistry {
	t.Helper()
	reg := metrics.NewPrometheusRegistry()
	setMetricsRegistry(reg)
	t.Cleanup(func() { filterMetricsPtr.Store(nil) })
	return reg
}

// scrapeRejects returns the exposition lines of the reject counter.
func scrapeRejects(t *testing.T, reg *metrics.PrometheusRegistry) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	var lines []string
	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.HasPrefix(line, metricRejects+"{") {
			lines = append(lines, line)
		}
	}
	return lines
}

// TestSlotsAlignWithPositions holds the one thing metrics.go asserts without
// saying it in code: the first four counter slots ARE the position constants, so
// rejectSlot(hit.at) needs no mapping table.
//
// VALIDATES: each primitive position converts to the slot that carries its own
// label, and the labels come from the position enum rather than a second copy.
// PREVENTS: a position inserted into config.go's enum silently shifting every
// reject onto a neighboring slot's series, which no counting test would notice
// because each series would still increment.
func TestSlotsAlignWithPositions(t *testing.T) {
	for _, p := range []position{positionUnspecified, positionDirect, positionTransit, positionOrigin} {
		slot := rejectSlot(p)
		require.Less(t, slot, slotCount, "position %s has no counter slot", p)
		assert.Equal(t, p.String(), slotPositionLabels[slot],
			"the slot for position %s must carry that position's own word", p)
		assert.Equal(t, reasonLabelListedASN, slotReasonLabels[slot],
			"a position match is always a listed ASN")
	}

	assert.Equal(t, positionLabelRegex, slotPositionLabels[slotPattern])
	assert.Equal(t, reasonLabelUnknownList, slotReasonLabels[slotUnknownList])
	assert.Equal(t, reasonLabelUnconfigured, slotReasonLabels[slotUnconfigured])
}

// TestRejectIncrementsCounter drives every reject the filter can produce from
// its real entry point and reads the exposed counter back.
//
// VALIDATES: AC-26. Each reject increments ze_filter_path_asn_rejects_total
// under the direction, position and reason that decided it, and an accepted
// route increments nothing.
// PREVENTS: a dropped route that no operator can count. The reject log sits at
// Info and says nothing about rate, so without this counter a filter silently
// eating a peer's routes is visible only to somebody reading the log.
func TestRejectIncrementsCounter(t *testing.T) {
	reg := installMetrics(t)
	configureFrom(t, `        reject-asn NO-TRANSIT {
            indirect [ 3356 ]
            regex [ "^65001 174 " ]
        }`)

	cases := []struct {
		name   string
		in     sdk.FilterUpdateInput
		action sdk.FilterAction
		series string
	}{{
		name: "transit_match_on_import",
		in: sdk.FilterUpdateInput{
			Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
			Update: updateFor(sequence(65001, 3356, 65002)),
		},
		action: sdk.FilterReject,
		series: `direction="import",position="transit",reason="listed-asn"`,
	}, {
		name: "origin_match_on_import",
		in: sdk.FilterUpdateInput{
			Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
			Update: updateFor(sequence(65001, 3356)),
		},
		action: sdk.FilterReject,
		series: `direction="import",position="origin",reason="listed-asn"`,
	}, {
		// On export the input carries the DESTINATION peer, so no token is a
		// neighbor and the leading 3356 is judged transit.
		name: "transit_match_on_export",
		in: sdk.FilterUpdateInput{
			Filter: "NO-TRANSIT", Direction: "export", Peer: "10.0.0.2", PeerAS: 3356,
			Update: updateFor(sequence(3356, 65002)),
		},
		action: sdk.FilterReject,
		series: `direction="export",position="transit",reason="listed-asn"`,
	}, {
		name: "pattern_match_on_import",
		in: sdk.FilterUpdateInput{
			Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
			Update: updateFor(sequence(65001, 174, 65002)),
		},
		action: sdk.FilterReject,
		series: `direction="import",position="regex",reason="listed-asn"`,
	}, {
		name: "unknown_list_on_import",
		in: sdk.FilterUpdateInput{
			Filter: "NOT-CONFIGURED", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
			Update: updateFor(sequence(65001, 65002)),
		},
		action: sdk.FilterReject,
		series: `direction="import",position="unspecified",reason="unknown-list"`,
	}, {
		name: "clean_path_counts_nothing",
		in: sdk.FilterUpdateInput{
			Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
			Update: updateFor(sequence(65001, 65002)),
		},
		action: sdk.FilterAccept,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := scrapeRejects(t, reg)

			out := handleFilterUpdate(&tc.in)
			require.Equal(t, tc.action, out.Action, "the decision this case is counting")

			if tc.series == "" {
				assert.Equal(t, before, scrapeRejects(t, reg),
					"an accepted route must leave every reject series where it was")
				return
			}
			assert.Contains(t, scrapeRejects(t, reg), metricRejects+"{"+tc.series+"} 1",
				"the reject must be counted under the labels that decided it")
		})
	}

	// The fail-closed reject before any delivery. It runs last because it drops
	// the configured lists.
	t.Run("before_configure_on_import", func(t *testing.T) {
		held := listsByName.Load()
		t.Cleanup(func() { listsByName.Store(held) })
		listsByName.Store(nil)

		out := handleFilterUpdate(&sdk.FilterUpdateInput{
			Filter: "NO-TRANSIT", Direction: "import", Peer: "10.0.0.1", PeerAS: 65001,
			Update: updateFor(sequence(65001, 65002)),
		})
		require.Equal(t, sdk.FilterReject, out.Action)
		assert.Contains(t, scrapeRejects(t, reg),
			metricRejects+`{direction="import",position="unspecified",reason="unconfigured"} 1`)
	})
}

// TestRejectLabelsAreBounded holds the cardinality rule the metrics page states:
// a label value is compile-time constant, so the series count does not grow with
// the config, the peers or the ASNs.
//
// VALIDATES: AC-26's bounded half. Every series exists from startup, every label
// value comes from the closed vocabulary, and no ASN, list name or peer address
// appears in a label.
// PREVENTS: a later edit adding a list name or a peer to the labels, which would
// make one series per configured list per peer and cost the operator's
// time-series database rather than saying anything new.
func TestRejectLabelsAreBounded(t *testing.T) {
	reg := installMetrics(t)
	lines := scrapeRejects(t, reg)

	require.Len(t, lines, int(flowCount)*int(slotCount),
		"every direction times every slot, all created at 0 before any traffic")

	series := regexp.MustCompile(`^` + regexp.QuoteMeta(metricRejects) +
		`\{direction="(import|export)",position="(direct|transit|origin|nth|regex|unspecified)",` +
		`reason="(listed-asn|unknown-list|unconfigured)"\} 0$`)
	for _, line := range lines {
		assert.Regexp(t, series, line, "a label value outside the closed vocabulary")
	}
}
