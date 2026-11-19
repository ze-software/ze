// Design: docs/architecture/fleet-interface.md -- the counts, the rankings and the series a dashboard shows
// Related: derive.go -- the device views these functions count
// Related: filter.go -- the filter each count links to
// Related: source_history.go -- the series Reduce fits to a chart
//
// A dashboard card is a count of something specific with a link to the table
// that lists exactly those things. So each count here is taken over the same
// device views the table filters, on the same value the filter field reads,
// and the lines of the Needs attention card are counted by running the very
// filter their link carries.

package fleetui

import (
	"cmp"
	"errors"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The bounds of the summaries.
const (
	// VersionsNamedMax is how many software versions the Software card
	// names. The older ones are counted together.
	VersionsNamedMax = 4
	// OfflineLongAfter is how long a device stays offline before it needs
	// attention.
	OfflineLongAfter = 24 * time.Hour
	// ReduceBudgetMax is the most points Reduce is asked for.
	ReduceBudgetMax = 4096
)

// StateCount is how many devices are in one display state.
type StateCount struct {
	State DisplayState
	Count int
}

// CountByState counts the devices of views by display state. Every state is
// in the answer, in the order of DisplayStates, with a count of zero for a
// state no device is in.
func CountByState(views []DeviceView) []StateCount {
	states := DisplayStates()

	counts := make([]StateCount, 0, len(states))
	for _, state := range states {
		count := 0
		for index := range views {
			if views[index].State == state {
				count++
			}
		}
		counts = append(counts, StateCount{State: state, Count: count})
	}

	return counts
}

// ConfigCount is how many devices are in one configuration state.
type ConfigCount struct {
	State ConfigState
	Count int
}

// CountByConfig counts the devices of views by configuration state. Every
// state is in the answer, in the order of ConfigStates.
func CountByConfig(views []DeviceView) []ConfigCount {
	states := ConfigStates()

	counts := make([]ConfigCount, 0, len(states))
	for _, state := range states {
		count := 0
		for index := range views {
			if views[index].Device.Config.State == state {
				count++
			}
		}
		counts = append(counts, ConfigCount{State: state, Count: count})
	}

	return counts
}

// VersionBucket is what one segment of the Software card counts. The zero
// value is not a bucket.
type VersionBucket uint8

const (
	// VersionBucketUnspecified is the invalid zero value.
	VersionBucketUnspecified VersionBucket = iota
	// VersionBucketNamed counts the devices of one version.
	VersionBucketNamed
	// VersionBucketOlder counts the devices of every version older than the
	// named ones.
	VersionBucketOlder
	// VersionBucketUnreported counts the devices that have reported no
	// version.
	VersionBucketUnreported
)

// VersionCount is how many devices one segment of the Software card counts.
type VersionCount struct {
	Bucket VersionBucket
	// Versions holds the one version of a named bucket, every version of
	// the older bucket, and nothing for the unreported bucket. The version
	// filter term of the segment's link lists them.
	Versions []string
	Count    int
}

// CountByVersion counts the devices of views by software version: the
// newest VersionsNamedMax versions each on their own, newest first, then the
// older versions together, then the devices that have reported none. A
// bucket that holds no device is left out.
func CountByVersion(views []DeviceView) []VersionCount {
	perVersion := make(map[string]int)
	unreported := 0
	for index := range views {
		version := deviceVersion(&views[index])
		if version == "" {
			unreported++
			continue
		}
		perVersion[version]++
	}

	versions := slices.SortedFunc(maps.Keys(perVersion), func(a, b string) int { return compareVersions(b, a) })

	named := min(len(versions), VersionsNamedMax)
	counts := make([]VersionCount, 0, named+2)
	for _, version := range versions[:named] {
		counts = append(counts, VersionCount{
			Bucket: VersionBucketNamed, Versions: []string{version}, Count: perVersion[version],
		})
	}

	if older := versions[named:]; len(older) > 0 {
		count := 0
		for _, version := range older {
			count += perVersion[version]
		}
		counts = append(counts, VersionCount{Bucket: VersionBucketOlder, Versions: older, Count: count})
	}
	if unreported > 0 {
		counts = append(counts, VersionCount{Bucket: VersionBucketUnreported, Count: unreported})
	}

	return counts
}

// Versions answers the software versions the devices of views report, each
// once, the newest first.
func Versions(views []DeviceView) []string {
	seen := make(map[string]struct{})
	versions := make([]string, 0, VersionsNamedMax)
	for index := range views {
		version := deviceVersion(&views[index])
		if version == "" {
			continue
		}
		if _, held := seen[version]; held {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	slices.SortFunc(versions, func(a, b string) int { return compareVersions(b, a) })

	return versions
}

// compareVersions orders two software versions: negative when a is the
// older, zero only for the same text. Versions are compared part by part
// between dots (compareVersionPart), so 1.4.10 is newer than 1.4.9, and a
// version that extends another is the newer. The order is total, so every
// sort of versions answers the same order whatever order they came in.
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	for index := range min(len(partsA), len(partsB)) {
		if order := compareVersionPart(partsA[index], partsB[index]); order != 0 {
			return order
		}
	}

	return cmp.Compare(len(partsA), len(partsB))
}

// compareVersionPart orders two parts of a version, as the leading number of
// each and the text after it: a part that opens with no digit is older than
// one that does, leading numbers compare as numbers of any length, a part
// with text after its number is older than the number alone (1.4.2-rc1 is
// older than 1.4.2), and texts compare as text. Parts these leave equal, 01
// and 1, compare as text, so the order is total: each step is a total
// order, and the steps are taken one after another.
func compareVersionPart(a, b string) int {
	numberA, restA := versionPartNumber(a)
	numberB, restB := versionPartNumber(b)

	if order := compareFalseFirst(numberA != "", numberB != ""); order != 0 {
		return order
	}
	if order := cmp.Compare(len(numberA), len(numberB)); order != 0 {
		return order
	}
	if order := strings.Compare(numberA, numberB); order != 0 {
		return order
	}
	if order := compareFalseFirst(restA == "", restB == ""); order != 0 {
		return order
	}
	if order := strings.Compare(restA, restB); order != 0 {
		return order
	}

	return strings.Compare(a, b)
}

// compareFalseFirst orders two truths, false before true.
func compareFalseFirst(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	default:
		return 1
	}
}

// versionPartNumber answers the leading digits of part, their leading zeros
// dropped, and the text after them.
func versionPartNumber(part string) (string, string) {
	end := 0
	for end < len(part) && part[end] >= '0' && part[end] <= '9' {
		end++
	}
	number := strings.TrimLeft(part[:end], "0")
	if number == "" && end > 0 {
		number = "0"
	}

	return number, part[end:]
}

// LabelKeys answers the label keys in use among the devices of views, each
// once, in text order. A filter reads a term whose field is one of them as a
// label term.
func LabelKeys(views []DeviceView) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, LabelsMax)
	for index := range views {
		for _, label := range views[index].Device.Labels {
			if _, held := seen[label.Key]; held {
				continue
			}
			seen[label.Key] = struct{}{}
			keys = append(keys, label.Key)
		}
	}
	slices.Sort(keys)

	return keys
}

// LabelUsage is one label key with the values in use for it.
type LabelUsage struct {
	Key string
	// Values holds every value of the key, in text order.
	Values []LabelValueCount
}

// LabelValueCount is how many devices carry one value of a label key.
type LabelValueCount struct {
	Value string
	Count int
}

// LabelsInUse answers every label key in use among the devices of views, in
// text order, with its values and how many devices carry each. The labels
// administration page and the assisted label menu show it.
func LabelsInUse(views []DeviceView) []LabelUsage {
	perKey := make(map[string]map[string]int)
	for index := range views {
		for _, label := range views[index].Device.Labels {
			if perKey[label.Key] == nil {
				perKey[label.Key] = make(map[string]int)
			}
			perKey[label.Key][label.Value]++
		}
	}

	keys := LabelKeys(views)
	usage := make([]LabelUsage, 0, len(keys))
	for _, key := range keys {
		values := make([]LabelValueCount, 0, len(perKey[key]))
		for value, count := range perKey[key] {
			values = append(values, LabelValueCount{Value: value, Count: count})
		}
		slices.SortFunc(values, func(a, b LabelValueCount) int { return cmp.Compare(a.Value, b.Value) })

		usage = append(usage, LabelUsage{Key: key, Values: values})
	}

	return usage
}

// EstateFigures is the figures of the Estate card, or of the Group card for
// the devices of a group node.
type EstateFigures struct {
	Devices   int
	Connected int
	// Sites counts the distinct values of the site label among the devices.
	Sites int
	// PeersEstablished and PeersConfigured count the BGP peers of the
	// devices that have reported a sample.
	PeersEstablished int
	PeersConfigured  int
	// AlarmsOpen counts the open alarms given.
	AlarmsOpen int
}

// Figures answers the figures of the Estate card over views and the open
// alarms of the same scope. siteKey is the label key whose values are the
// sites.
func Figures(views []DeviceView, alarms []Alarm, siteKey string) EstateFigures {
	figures := EstateFigures{Devices: len(views), AlarmsOpen: len(alarms)}

	sites := make(map[string]struct{})
	for index := range views {
		view := &views[index]

		if view.Device.Connection.Connected {
			figures.Connected++
		}
		if site, labeled := view.Label(siteKey); labeled {
			sites[site] = struct{}{}
		}

		latest := &view.Device.Latest
		if latest.Reported {
			figures.PeersEstablished += latest.PeersEstablished
			figures.PeersConfigured += latest.PeersEstablished + latest.PeersDown + latest.PeersAdminDown
		}
	}
	figures.Sites = len(sites)

	return figures
}

// AttentionKind is one condition that needs an operator. The zero value is
// not a kind.
type AttentionKind uint8

const (
	// AttentionUnspecified is the invalid zero value.
	AttentionUnspecified AttentionKind = iota
	// AttentionRolloutPaused is a rollout that paused and waits.
	AttentionRolloutPaused
	// AttentionDeviceDiverged is a device that holds a locally edited
	// configuration.
	AttentionDeviceDiverged
	// AttentionConfigRejected is a device that refused its configuration.
	AttentionConfigRejected
	// AttentionOfflineLong is a device offline for longer than
	// OfflineLongAfter.
	AttentionOfflineLong
	// AttentionSoftwareBehind is a device more than one software version
	// behind the newest in the estate.
	AttentionSoftwareBehind
)

var attentionWords = []string{
	enumWordUnspecified, "rollout paused", "device diverged", "configuration rejected",
	"device offline for more than a day", "device more than one version behind",
}

// String answers what the condition is, for one thing: a line of the card
// writes the count before it.
func (kind AttentionKind) String() string {
	return enumText(attentionWords, kind)
}

// AttentionLine is one line of the Needs attention card.
type AttentionLine struct {
	Kind  AttentionKind
	Count int
	// Filter is the filter of the line's link: over the devices table for a
	// line about devices, over the rollouts table for the paused rollouts.
	// The count is the number of rows that filter keeps.
	Filter string
}

// NeedsAttention answers the lines of the Needs attention card over views
// and the rollouts of the hub. A condition that holds for nothing has no
// line. Each device line is counted by applying the filter its link carries,
// so the count and the rows behind it cannot differ.
func NeedsAttention(views []DeviceView, rollouts []Rollout) ([]AttentionLine, error) {
	lines := make([]AttentionLine, 0, len(attentionWords))

	paused := 0
	for index := range rollouts {
		if rollouts[index].State == RolloutStatePaused {
			paused++
		}
	}
	if paused > 0 {
		lines = append(lines, AttentionLine{
			Kind:   AttentionRolloutPaused,
			Count:  paused,
			Filter: fieldState + filterFieldSeparator + RolloutStatePaused.Key(),
		})
	}

	deviceLines := []AttentionLine{
		{Kind: AttentionDeviceDiverged, Filter: fieldState + filterFieldSeparator + DisplayStateDiverged.Key()},
		{Kind: AttentionConfigRejected, Filter: fieldConfig + filterFieldSeparator + ConfigStateRejected.Key()},
		{Kind: AttentionOfflineLong, Filter: offlineLongFilter()},
	}
	if behind := versionsBehind(views); len(behind) > 0 {
		deviceLines = append(deviceLines, AttentionLine{
			Kind:   AttentionSoftwareBehind,
			Filter: textbuf.New().Str(fieldVersion).Str(filterFieldSeparator).Join(behind, filterListSeparator).Slice(),
		})
	}

	labelKeys := LabelKeys(views)
	for _, line := range deviceLines {
		filter, err := DeviceFilters().Parse(line.Filter, labelKeys)
		if err != nil {
			return nil, err
		}

		line.Count = len(filter.Apply(views))
		if line.Count > 0 {
			lines = append(lines, line)
		}
	}

	return lines, nil
}

// offlineLongFilter answers the filter that keeps the devices offline for
// longer than OfflineLongAfter.
func offlineLongFilter() string {
	return textbuf.New().
		Str(fieldState).Str(filterFieldSeparator).Str(DisplayStateOffline.Key()).Byte(' ').
		Str(fieldLastSeen).Str(filterFieldSeparator).Byte(filterAbove).
		Int(int64(OfflineLongAfter / time.Hour)).Byte('h').
		Slice()
}

// versionsBehind answers the software versions more than one version behind
// the newest the devices of views report.
func versionsBehind(views []DeviceView) []string {
	versions := Versions(views)
	if len(versions) > 2 {
		return versions[2:]
	}

	return []string{}
}

// TopAlarms answers the most severe alarms, at most limit: the most severe
// first, then the oldest first. It is the default order of an alarms table
// and the content of an alarms card.
func TopAlarms(alarms []Alarm, limit int) []Alarm {
	ordered := slices.Clone(alarms)
	slices.SortStableFunc(ordered, func(a, b Alarm) int {
		if order := cmp.Compare(a.Severity, b.Severity); order != 0 {
			return order
		}

		return a.Since.Compare(b.Since)
	})

	return ordered[:min(max(limit, 0), len(ordered))]
}

// ErrRankRefused means a ranking was asked for a metric no device reports in
// its last sample.
var ErrRankRefused = errors.New("the metric cannot rank devices")

// Ranked is one device of a ranking, with the value it was ranked by.
type Ranked struct {
	Device string
	Value  float64
}

// Busiest answers the devices of views with the highest value of metric in
// their last sample, at most limit, the highest first. A device that has
// reported no sample is not ranked. It refuses a metric the last sample does
// not hold with an error wrapping ErrRankRefused.
func Busiest(views []DeviceView, metric Metric, limit int) ([]Ranked, error) {
	// The metric is judged before any device is read, so an estate where no
	// device has reported yet is refused the same way.
	var none LatestSample
	if _, held := latestFigure(&none, metric); !held {
		return nil, rankRefused(metric)
	}

	ranked := make([]Ranked, 0, len(views))
	for index := range views {
		latest := &views[index].Device.Latest
		if !latest.Reported {
			continue
		}

		value, _ := latestFigure(latest, metric)
		ranked = append(ranked, Ranked{Device: views[index].Device.Name, Value: value})
	}

	slices.SortStableFunc(ranked, func(a, b Ranked) int { return cmp.Compare(b.Value, a.Value) })

	return ranked[:min(max(limit, 0), len(ranked))], nil
}

// latestFigure answers the value of metric in a last sample, or false for a
// metric the last sample does not hold.
func latestFigure(latest *LatestSample, metric Metric) (float64, bool) {
	switch metric {
	case MetricCPU:
		return latest.CPUPercent, true
	case MetricMemory:
		return latest.MemoryPercent, true
	case MetricStorage:
		return latest.StoragePercent, true
	case MetricLoad:
		return latest.Load, true
	case MetricTrafficIn:
		return float64(latest.TrafficInBps), true
	case MetricTrafficOut:
		return float64(latest.TrafficOutBps), true
	case MetricPeersEstablished:
		return float64(latest.PeersEstablished), true
	case MetricUnspecified, MetricDevicesConnected:
		return 0, false
	}

	return 0, false
}

// rankRefused is the refusal for a ranking by a metric the last sample of a
// device does not hold. Its fix lists the metrics latestFigure answers for.
func rankRefused(metric Metric) error {
	var none LatestSample
	ranking := make([]string, 0, len(metricKeys))
	for index := range len(metricKeys) - 1 {
		candidate := Metric(index + 1)
		if _, held := latestFigure(&none, candidate); held {
			ranking = append(ranking, candidate.Key())
		}
	}

	problem := textbuf.New().Str("Devices cannot be ranked by ").Quoted(metric.Key()).Str(".").Slice()
	fix := textbuf.New().Str("rank by one of: ").Join(ranking, ", ").Slice()

	return opererr.Wrap(ErrRankRefused, problem,
		"The last sample of a device does not hold that figure, so no ranking was made.", fix)
}

// ErrReduceRefused means Reduce was asked for something it cannot compute.
var ErrReduceRefused = errors.New("the series cannot be reduced")

// Bucket is one point of a reduced series: what the samples of one period
// say together.
type Bucket struct {
	// At is the start of the period.
	At time.Time
	// Samples counts the samples of the period. Average and Maximum mean
	// nothing while it is zero: see Gap.
	Samples int
	Average float64
	Maximum float64
}

// Gap reports whether the period holds no sample. A chart draws nothing
// there and never joins the points on either side.
func (bucket Bucket) Gap() bool {
	return bucket.Samples == 0
}

// Reduce fits the points of a series to a chart at most budget points wide.
// It cuts span into periods of equal length, each at least interval long and
// a whole number of intervals, and answers one bucket per period, in time
// order, with the average and the maximum of the samples inside it. A period
// with no sample is a gap, so a gap of the series survives the reduction. A
// point outside span is left out.
//
// interval is the time between two samples of the series when none is
// missing. It refuses, with an error wrapping ErrReduceRefused, a budget
// outside 1 to ReduceBudgetMax, an interval that is not positive, and a span
// that does not end after it starts.
func Reduce(points []Point, span TimeSpan, interval time.Duration, budget int) ([]Bucket, error) {
	if budget < 1 {
		return nil, reduceRefused("The chart asked for no point.")
	}
	if budget > ReduceBudgetMax {
		return nil, reduceRefused("The chart asked for more points than a series is reduced to.")
	}
	if interval <= 0 {
		return nil, reduceRefused("The series names no interval between its samples.")
	}

	length := span.To.Sub(span.From)
	if length <= 0 {
		return nil, reduceRefused("The period of the chart does not end after it starts.")
	}

	// The shortest period that fits the budget, rounded up to a whole number
	// of intervals and never shorter than one.
	width := ceilDiv(ceilDiv(length, time.Duration(budget)), interval) * interval
	count := int(ceilDiv(length, width))

	buckets := make([]Bucket, count)
	sums := make([]float64, count)
	for index := range buckets {
		buckets[index].At = span.From.Add(time.Duration(index) * width)
	}

	for _, point := range points {
		if point.At.Before(span.From) {
			continue
		}
		if !point.At.Before(span.To) {
			continue
		}

		slot := int(point.At.Sub(span.From) / width)
		bucket := &buckets[slot]
		if bucket.Gap() {
			bucket.Maximum = point.Value
		}
		bucket.Maximum = max(bucket.Maximum, point.Value)
		bucket.Samples++
		sums[slot] += point.Value
	}

	for index := range buckets {
		if !buckets[index].Gap() {
			buckets[index].Average = sums[index] / float64(buckets[index].Samples)
		}
	}

	return buckets, nil
}

// ceilDiv divides two positive durations and rounds up.
func ceilDiv(a, b time.Duration) time.Duration {
	return (a + b - 1) / b
}

// reduceRefused is the refusal of a reduction that cannot be computed.
func reduceRefused(problem string) error {
	return opererr.Wrap(ErrReduceRefused, problem,
		"The chart cannot be drawn from a series it cannot fit, so it shows no line.",
		"reload the page")
}
