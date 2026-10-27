// Design: docs/plugin-development/metrics.md -- community rewrite operator signal
// Related: metrics.go -- the counters these tests read

package filter_community

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/metrics"
)

// installCommunityMetrics binds a real Prometheus registry to the package and
// returns it. The real registry rather than a fake, because these tests read the
// exposed series: name, label value and value.
func installCommunityMetrics(t *testing.T) *metrics.PrometheusRegistry {
	t.Helper()
	reg := metrics.NewPrometheusRegistry()
	setMetricsRegistry(reg)
	t.Cleanup(func() { communityMetricsPtr.Store(nil) })
	return reg
}

// seriesValue scrapes reg and returns the value of the series whose exposition
// line starts with series, the metric name plus its label set exactly as
// Prometheus writes it. A series the scrape does not hold fails the test.
func seriesValue(t *testing.T, reg *metrics.PrometheusRegistry, series string) float64 {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/metrics", http.NoBody))
	require.Equal(t, http.StatusOK, rec.Code)
	body, err := io.ReadAll(rec.Body)
	require.NoError(t, err)

	for line := range strings.SplitSeq(string(body), "\n") {
		value, ok := strings.CutPrefix(line, series+" ")
		if !ok {
			continue
		}
		v, err := strconv.ParseFloat(value, 64)
		require.NoError(t, err)
		return v
	}
	require.Failf(t, "series not exposed", "%s", series)
	return 0
}

const (
	seriesScrubStandard  = metricScrubbed + `{width="standard"}`
	seriesScrubLarge     = metricScrubbed + `{width="large"}`
	seriesRelationTag    = metricRelationTag
	seriesGuardNoExport  = metricBlackholeTag + `{community="no-export"}`
	seriesGuardNoAdverti = metricBlackholeTag + `{community="no-advertise"}`
)

// TestScrubCountsEachRemovedValueByWidth scrubs one route carrying two own-GA
// standard values, one own-GA large value and one foreign value of each width,
// then one route carrying only foreign values.
//
// VALIDATES: ze_filter_community_value_scrubbed_total counts every value the
// scrub removes, under the width it was removed from, and nothing for a route
// it leaves alone.
// PREVENTS: a counter stepped per route instead of per value, or stepped where
// the values are listed rather than where the rebuild removed them.
func TestScrubCountsEachRemovedValueByWidth(t *testing.T) {
	reg := installCommunityMetrics(t)
	foreignStandard := uint32(64511)<<16 | 99
	foreignLarge := [3]uint32{64511, 1, 1}

	attrs := append(buildOriginAttr(), buildCommunityAttr(scrubLocalAS<<16|1, scrubLocalAS<<16|2, foreignStandard)...)
	attrs = append(attrs, buildLargeCommunityAttr([3]uint32{scrubLocalAS, 7, 7}, foreignLarge)...)
	require.NotNil(t, scrubOwnGACommunities(buildPayload(attrs), scrubLocalAS, nil, 0, 0))

	assert.Equal(t, 2.0, seriesValue(t, reg, seriesScrubStandard))
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesScrubLarge))

	attrs = append(buildOriginAttr(), buildCommunityAttr(foreignStandard)...)
	attrs = append(attrs, buildLargeCommunityAttr(foreignLarge)...)
	assert.Nil(t, scrubOwnGACommunities(buildPayload(attrs), scrubLocalAS, nil, 0, 0))

	assert.Equal(t, 2.0, seriesValue(t, reg, seriesScrubStandard), "a route with no own-GA value counts nothing")
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesScrubLarge), "a route with no own-GA value counts nothing")
}

// TestRelationTagCountedWhenWritten runs the registered relation filter for a
// provider, whose role maps to a parameter, and for a route-server client,
// whose role writes no tag.
//
// VALIDATES: ze_filter_community_relation_tag_written_total counts a route the
// filter tagged and not one it left alone.
// PREVENTS: the counter stepped for every route the enabled filter reads.
func TestRelationTagCountedWhenWritten(t *testing.T) {
	reg := installCommunityMetrics(t)
	withPeerConfig(t, relationOn(nil))

	_, modified := relationIngressFilter(relPeerInfo(relPeerAS), buildPayload(buildOriginAttr()),
		map[string]any{"src-peer-role": roleProvider})
	require.NotNil(t, modified)
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesRelationTag))

	_, modified = relationIngressFilter(relPeerInfo(relPeerAS), buildPayload(buildOriginAttr()),
		map[string]any{"src-peer-role": roleRSClient})
	require.Nil(t, modified)
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesRelationTag), "an untagged route counts nothing")
}

// TestBlackholeGuardCountedByCommunity runs the guard over a BLACKHOLE route
// with each guard community, over a route without BLACKHOLE, and over a
// BLACKHOLE route that already carries the community.
//
// VALIDATES: ze_filter_community_blackhole_guard_added_total counts a route the
// guard added a community to, under the community added, and nothing when the
// guard changes nothing.
// PREVENTS: the counter stepped for every route the enabled guard reads, or
// under the wrong community.
func TestBlackholeGuardCountedByCommunity(t *testing.T) {
	reg := installCommunityMetrics(t)
	blackhole := uint32(attribute.CommunityBlackhole)

	require.NotNil(t, blackholePropagationGuard(blackholePayload(blackhole), blackholeLocalAS, blackholeGuardNoExport))
	require.NotNil(t, blackholePropagationGuard(blackholePayload(blackhole), blackholeLocalAS, blackholeGuardNoAdvertise))
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesGuardNoExport))
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesGuardNoAdverti))

	require.Nil(t, blackholePropagationGuard(blackholePayload(uint32(64511)<<16|1), blackholeLocalAS, blackholeGuardNoExport))
	require.Nil(t, blackholePropagationGuard(
		blackholePayload(blackhole, uint32(attribute.CommunityNoExport)), blackholeLocalAS, blackholeGuardNoExport))
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesGuardNoExport), "a guard that adds nothing counts nothing")
	assert.Equal(t, 1.0, seriesValue(t, reg, seriesGuardNoAdverti), "a guard that adds nothing counts nothing")
}

// TestCommunityMetricsWiredThroughTheFilter proves the registry reaches this
// package the way the daemon delivers it: the reactor calls
// filterapi.SetMetricsRegistry, which runs every registered filter's
// ConfigureMetrics.
//
// VALIDATES: the bgp-filter-community registration declares ConfigureMetrics.
// PREVENTS: counters that tests bind by hand and the daemon never binds.
func TestCommunityMetricsWiredThroughTheFilter(t *testing.T) {
	t.Cleanup(func() { communityMetricsPtr.Store(nil) })
	reg := metrics.NewPrometheusRegistry()
	filterapi.SetMetricsRegistry(reg)

	assert.Zero(t, seriesValue(t, reg, seriesScrubStandard), "every series is present at zero from startup")
	assert.Zero(t, seriesValue(t, reg, seriesRelationTag))
	assert.Zero(t, seriesValue(t, reg, seriesGuardNoAdverti))
}
