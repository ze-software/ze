package fleetui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCountByState counts the test estate by display state.
//
// VALIDATES: every state is listed in legend order, a state no device is in
// included, and the counts add up to the estate.
// PREVENTS: a donut whose legend changes shape when a state empties.
func TestCountByState(t *testing.T) {
	assert.Equal(t, []StateCount{
		{DisplayStateHealthy, 2},
		{DisplayStateDegraded, 1},
		{DisplayStateDown, 1},
		{DisplayStateOffline, 1},
		{DisplayStateDiverged, 1},
		{DisplayStateUnknown, 1},
	}, CountByState(testViews()))

	healthyOnly := CountByState(testViews()[:2])
	assert.Equal(t, StateCount{DisplayStateHealthy, 2}, healthyOnly[0])
	assert.Len(t, healthyOnly, 6)
	assert.Equal(t, StateCount{DisplayStateDown, 0}, healthyOnly[2])
}

// TestCountByStateEqualsItsFilter counts each display state and applies the
// filter its segment links to.
//
// VALIDATES: every count equals the number of rows the state filter keeps.
// PREVENTS: a card whose figure and whose link disagree.
func TestCountByStateEqualsItsFilter(t *testing.T) {
	for _, count := range CountByState(testViews()) {
		kept := filterNames(t, SetFilterTerm("", "state", count.State.Key()))

		assert.Len(t, kept, count.Count, count.State.String())
	}
}

// TestCountByConfig counts the test estate by configuration state.
//
// VALIDATES: every configuration state is listed in legend order with its
// count.
// PREVENTS: a device with a rejected configuration counted as in sync.
func TestCountByConfig(t *testing.T) {
	assert.Equal(t, []ConfigCount{
		{ConfigStateInSync, 3},
		{ConfigStatePending, 1},
		{ConfigStateRejected, 1},
		{ConfigStateDiverged, 1},
		{ConfigStateNeverFetched, 1},
	}, CountByConfig(testViews()))
}

// TestCountByVersion counts the test estate by software version.
//
// VALIDATES: versions are named newest first, and the devices that reported
// no version are counted on their own.
// PREVENTS: a device with no inventory counted under a version.
func TestCountByVersion(t *testing.T) {
	assert.Equal(t, []VersionCount{
		{Bucket: VersionBucketNamed, Versions: []string{"1.4.2"}, Count: 3},
		{Bucket: VersionBucketNamed, Versions: []string{"1.4.1"}, Count: 2},
		{Bucket: VersionBucketNamed, Versions: []string{"1.3.9"}, Count: 1},
		{Bucket: VersionBucketUnreported, Count: 1},
	}, CountByVersion(testViews()))
}

// TestCountByVersionGroupsTheOlderOnes counts an estate that runs six
// versions.
//
// VALIDATES: the newest four versions are named, compared as numbers so that
// 1.10.0 is newer than 1.9.0, and the others are counted together as older.
// PREVENTS: a Software card with one segment per version ever deployed.
func TestCountByVersionGroupsTheOlderOnes(t *testing.T) {
	versions := []string{"1.9.0", "1.10.0", "1.2.0", "1.10.1", "1.3.0", "1.2.0", "1.1.0"}

	devices := make([]Device, 0, len(versions))
	for index, version := range versions {
		devices = append(devices, testConnected("device-"+string(rune('a'+index)), version, 1, 1))
	}

	assert.Equal(t, []VersionCount{
		{Bucket: VersionBucketNamed, Versions: []string{"1.10.1"}, Count: 1},
		{Bucket: VersionBucketNamed, Versions: []string{"1.10.0"}, Count: 1},
		{Bucket: VersionBucketNamed, Versions: []string{"1.9.0"}, Count: 1},
		{Bucket: VersionBucketNamed, Versions: []string{"1.3.0"}, Count: 1},
		{Bucket: VersionBucketOlder, Versions: []string{"1.2.0", "1.1.0"}, Count: 3},
	}, CountByVersion(DeriveDevices(devices, nil, nil, testNow)))
}

// TestCompareVersions orders pairs of versions.
//
// VALIDATES: parts that are numbers compare as numbers, a longer version
// with the same start is newer, and equal versions are equal.
// PREVENTS: 1.4.10 sorted below 1.4.9.
func TestCompareVersions(t *testing.T) {
	assert.Negative(t, compareVersions("1.4.9", "1.4.10"))
	assert.Positive(t, compareVersions("2.0.0", "1.99.99"))
	assert.Negative(t, compareVersions("1.4", "1.4.1"))
	assert.Zero(t, compareVersions("1.4.2", "1.4.2"))
	assert.Negative(t, compareVersions("1.4.2-rc1", "1.4.2-rc2"))
}

// TestEstateFigures reads the figures of the Estate card over the test
// estate.
//
// VALIDATES: the devices, the connected devices, the distinct sites, the
// established peers over the configured ones and the open alarms.
// PREVENTS: a device with no site counted as a site of its own.
func TestEstateFigures(t *testing.T) {
	assert.Equal(t, EstateFigures{
		Devices: 7, Connected: 6, Sites: 2, PeersEstablished: 10, PeersConfigured: 12, AlarmsOpen: 3,
	}, Figures(testViews(), testAlarms(), "site"))
}

// TestNeedsAttention reads the lines of the Needs attention card over the
// test estate and one paused rollout.
//
// VALIDATES: one line per condition that holds, each with its count and the
// filter of its link: a paused rollout, a diverged device, a rejected
// configuration, a device offline for more than a day, a device more than
// one version behind.
// PREVENTS: a line that counts devices its link does not list.
func TestNeedsAttention(t *testing.T) {
	rollouts := []Rollout{
		{Name: "edge-refresh", State: RolloutStatePaused},
		{Name: "core-refresh", State: RolloutStateComplete},
	}

	lines, err := NeedsAttention(testViews(), rollouts)
	require.NoError(t, err)

	assert.Equal(t, []AttentionLine{
		{Kind: AttentionRolloutPaused, Count: 1, Filter: "state:paused"},
		{Kind: AttentionDeviceDiverged, Count: 1, Filter: "state:diverged"},
		{Kind: AttentionConfigRejected, Count: 1, Filter: "config:rejected"},
		{Kind: AttentionOfflineLong, Count: 1, Filter: "state:offline last-seen:>24h"},
		{Kind: AttentionSoftwareBehind, Count: 1, Filter: "version:1.3.9"},
	}, lines)
}

// TestNeedsAttentionCountsEqualTheirFilters applies the filter of each device
// line to the estate.
//
// VALIDATES: the count of a line is the number of rows its filter keeps.
// PREVENTS: a Needs attention line that leads to a table of another length.
func TestNeedsAttentionCountsEqualTheirFilters(t *testing.T) {
	lines, err := NeedsAttention(testViews(), nil)
	require.NoError(t, err)
	require.Len(t, lines, 4)

	for _, line := range lines {
		assert.Len(t, filterNames(t, line.Filter), line.Count, line.Kind.String())
	}
}

// TestNeedsAttentionIsEmptyForAHealthyEstate reads the card over devices
// that need nothing.
//
// VALIDATES: a condition that holds for nothing has no line.
// PREVENTS: a card listing zero counts as things to do.
func TestNeedsAttentionIsEmptyForAHealthyEstate(t *testing.T) {
	lines, err := NeedsAttention(testViews()[:2], nil)
	require.NoError(t, err)

	assert.Empty(t, lines)
}

// TestOfflineLongBoundary derives a device offline for just under and just
// over a day.
//
// VALIDATES: a device offline for less than a day does not need attention,
// and one offline for longer does.
// PREVENTS: every device that drops its connection listed at once.
func TestOfflineLongBoundary(t *testing.T) {
	offlineFor := func(ago time.Duration) []DeviceView {
		device := testConnected("edge", "1.4.2", 1, 1)
		device.Connection = Connection{EverConnected: true, LastSeen: testNow.Add(-ago)}

		return DeriveDevices([]Device{device}, nil, nil, testNow)
	}

	recent, err := NeedsAttention(offlineFor(OfflineLongAfter-time.Minute), nil)
	require.NoError(t, err)
	assert.Empty(t, recent)

	long, err := NeedsAttention(offlineFor(OfflineLongAfter+time.Minute), nil)
	require.NoError(t, err)
	require.Len(t, long, 1)
	assert.Equal(t, AttentionOfflineLong, long[0].Kind)
}

// TestBusiest ranks the test estate by a figure of the last sample.
//
// VALIDATES: the devices come highest first, at most the limit, and a device
// that reported no sample is not ranked.
// PREVENTS: a device with no sample ranked as idle.
func TestBusiest(t *testing.T) {
	byCPU, err := Busiest(testViews(), MetricCPU, 3)
	require.NoError(t, err)
	assert.Equal(t, []Ranked{{coreLon01, 95}, {edgeLon01, 71}, {cpe0113, 30}}, byCPU)

	byMemory, err := Busiest(testViews(), MetricMemory, 10)
	require.NoError(t, err)
	assert.Len(t, byMemory, 5, "the two devices with no sample are not ranked")
	assert.Equal(t, Ranked{edgeLon02, 91}, byMemory[0])
}

// TestBusiestRefusesAMetricNoSampleHolds ranks by a metric that is a series
// of the estate.
//
// VALIDATES: the refusal names the metric and lists the metrics a ranking
// takes.
// PREVENTS: a ranking of zeros shown as a ranking.
func TestBusiestRefusesAMetricNoSampleHolds(t *testing.T) {
	_, err := Busiest(testViews(), MetricDevicesConnected, 5)

	refusal := requireRefusal(t, err, ErrRankRefused)
	assert.Contains(t, refusal.Problem(), `"devices-connected"`)
	assert.Equal(t, "rank by one of: cpu, memory, storage, load, traffic-in, traffic-out, peers-established", refusal.Fix())
}

// TestTopAlarms orders the open alarms of the test estate.
//
// VALIDATES: the most severe come first, the oldest first among equals, and
// the answer holds at most the limit.
// PREVENTS: a warning shown above a critical alarm.
func TestTopAlarms(t *testing.T) {
	ids := func(alarms []Alarm) []string {
		found := make([]string, 0, len(alarms))
		for index := range alarms {
			found = append(found, alarms[index].ID)
		}

		return found
	}

	assert.Equal(t, []string{"a3", "a1", "a2"}, ids(TopAlarms(testAlarms(), 8)))
	assert.Equal(t, []string{"a3", "a1"}, ids(TopAlarms(testAlarms(), 2)))
	assert.Empty(t, TopAlarms(testAlarms(), 0))
}

// TestScopeAlarmsAndTasks scopes the alarms and the tasks of the estate to
// the devices of one site.
//
// VALIDATES: a group sees the alarms about its devices and the tasks that
// target at least one of them.
// PREVENTS: a group page listing the alarms of the whole estate.
func TestScopeAlarmsAndTasks(t *testing.T) {
	ams, err := GroupDevices(testViews(), []GroupLevel{{Key: "site", Value: "ams"}})
	require.NoError(t, err)

	alarms := ScopeAlarms(testAlarms(), ams)
	require.Len(t, alarms, 1)
	assert.Equal(t, "a3", alarms[0].ID)
	assert.Empty(t, ScopeTasks(testTasks(), ams))

	lon, err := GroupDevices(testViews(), []GroupLevel{{Key: "site", Value: "lon"}})
	require.NoError(t, err)
	assert.Len(t, ScopeAlarms(testAlarms(), lon), 2)
	assert.Len(t, ScopeTasks(testTasks(), lon), 2)
}

// TestLabelsInUse lists the labels of the test estate.
//
// VALIDATES: every key in use comes in text order, with its values in text
// order and the count of devices carrying each.
// PREVENTS: an assisted label menu offering a value no device carries.
func TestLabelsInUse(t *testing.T) {
	assert.Equal(t, []string{"role", "site"}, LabelKeys(testViews()))

	assert.Equal(t, []LabelUsage{
		{Key: "role", Values: []LabelValueCount{{"core", 2}, {"cpe", 2}, {"edge", 2}, {"route-server", 1}}},
		{Key: "site", Values: []LabelValueCount{{"ams", 2}, {"lon", 3}}},
	}, LabelsInUse(testViews()))
}

// minuteSeries builds one point per minute from start, with the given
// values. A negative value leaves the minute out, which is a gap.
func minuteSeries(start time.Time, values ...float64) []Point {
	points := make([]Point, 0, len(values))
	for index, value := range values {
		if value < 0 {
			continue
		}
		points = append(points, Point{At: start.Add(time.Duration(index) * time.Minute), Value: value})
	}

	return points
}

// TestReduceKeepsAverageAndMaximum reduces twelve samples to four points.
//
// VALIDATES: each bucket starts where its period starts and holds the count,
// the average and the maximum of its samples.
// PREVENTS: a traffic spike averaged away by the reduction.
func TestReduceKeepsAverageAndMaximum(t *testing.T) {
	start := testNow.Add(-12 * time.Minute)
	points := minuteSeries(start, 1, 2, 3, 10, 20, 90, 5, 5, 5, 0, 0, 3)

	buckets, err := Reduce(points, TimeSpan{From: start, To: testNow}, time.Minute, 4)
	require.NoError(t, err)

	assert.Equal(t, []Bucket{
		{At: start, Samples: 3, Average: 2, Maximum: 3},
		{At: start.Add(3 * time.Minute), Samples: 3, Average: 40, Maximum: 90},
		{At: start.Add(6 * time.Minute), Samples: 3, Average: 5, Maximum: 5},
		{At: start.Add(9 * time.Minute), Samples: 3, Average: 1, Maximum: 3},
	}, buckets)
}

// TestReducePreservesGaps reduces a series with a period that holds no
// sample.
//
// VALIDATES: a period with no sample is a bucket marked as a gap, between
// the buckets on either side, and a partly filled period averages what it
// holds.
// PREVENTS: a chart that joins the points on either side of an outage with a
// line.
func TestReducePreservesGaps(t *testing.T) {
	start := testNow.Add(-8 * time.Minute)
	points := minuteSeries(start, 4, 6, -1, -1, -1, 8, 2, 2)

	buckets, err := Reduce(points, TimeSpan{From: start, To: testNow}, time.Minute, 4)
	require.NoError(t, err)
	require.Len(t, buckets, 4)

	assert.False(t, buckets[0].Gap())
	assert.InDelta(t, 5, buckets[0].Average, 0)
	assert.True(t, buckets[1].Gap(), "minutes two and three hold no sample")
	assert.Equal(t, start.Add(2*time.Minute), buckets[1].At)
	assert.False(t, buckets[2].Gap())
	assert.Equal(t, 1, buckets[2].Samples, "minute four is missing, minute five is not")
	assert.InDelta(t, 8, buckets[2].Average, 0)
	assert.False(t, buckets[3].Gap())
}

// TestReduceNeverExceedsItsBudget reduces series of several lengths to
// several widths.
//
// VALIDATES: the answer never holds more points than the budget, and a
// series already within the budget keeps one bucket per sample.
// PREVENTS: a chart handed more points than it has pixels.
func TestReduceNeverExceedsItsBudget(t *testing.T) {
	week := TimeSpan{From: testNow.Add(-7 * 24 * time.Hour), To: testNow}

	for _, budget := range []int{1, 7, 300, 1000, ReduceBudgetMax} {
		buckets, err := Reduce(nil, week, time.Minute, budget)
		require.NoError(t, err)

		assert.LessOrEqual(t, len(buckets), budget, "budget %d", budget)
		assert.NotEmpty(t, buckets, "budget %d", budget)
	}

	hour := TimeSpan{From: testNow.Add(-time.Hour), To: testNow}
	buckets, err := Reduce(nil, hour, time.Minute, 300)
	require.NoError(t, err)
	assert.Len(t, buckets, 60, "a bucket is never shorter than the sample interval")
}

// TestReduceHandlesNegativeValues reduces samples below zero.
//
// VALIDATES: the maximum of a bucket of negative values is the largest of
// them, not zero.
// PREVENTS: a maximum that starts from the zero value of the bucket.
func TestReduceHandlesNegativeValues(t *testing.T) {
	start := testNow.Add(-2 * time.Minute)
	points := []Point{{At: start, Value: -5}, {At: start.Add(time.Minute), Value: -3}}

	buckets, err := Reduce(points, TimeSpan{From: start, To: testNow}, time.Minute, 1)
	require.NoError(t, err)
	require.Len(t, buckets, 1)

	assert.InDelta(t, -3, buckets[0].Maximum, 0)
	assert.InDelta(t, -4, buckets[0].Average, 0)
}

// TestReduceLeavesOutPointsOutsideTheSpan reduces a series that starts
// before the period and ends after it.
//
// VALIDATES: a point before the start and a point at or after the end are
// left out.
// PREVENTS: a sample of another period counted in the first or last bucket.
func TestReduceLeavesOutPointsOutsideTheSpan(t *testing.T) {
	start := testNow.Add(-2 * time.Minute)
	points := []Point{
		{At: start.Add(-time.Second), Value: 100},
		{At: start, Value: 1},
		{At: testNow.Add(-time.Second), Value: 3},
		{At: testNow, Value: 100},
	}

	buckets, err := Reduce(points, TimeSpan{From: start, To: testNow}, time.Minute, 1)
	require.NoError(t, err)
	require.Len(t, buckets, 1)

	assert.Equal(t, 2, buckets[0].Samples)
	assert.InDelta(t, 3, buckets[0].Maximum, 0)
}

// TestReduceRefusals asks for reductions that cannot be computed.
//
// VALIDATES: a budget below one or above the limit, an interval that is not
// positive and a period that does not end after it starts are each refused.
// PREVENTS: a division by zero, and an allocation a query parameter sizes.
func TestReduceRefusals(t *testing.T) {
	span := TimeSpan{From: testNow.Add(-time.Hour), To: testNow}

	_, err := Reduce(nil, span, time.Minute, 0)
	requireRefusal(t, err, ErrReduceRefused)

	_, err = Reduce(nil, span, time.Minute, ReduceBudgetMax+1)
	requireRefusal(t, err, ErrReduceRefused)

	_, err = Reduce(nil, span, 0, 100)
	requireRefusal(t, err, ErrReduceRefused)

	_, err = Reduce(nil, TimeSpan{From: testNow, To: testNow}, time.Minute, 100)
	requireRefusal(t, err, ErrReduceRefused)
}
