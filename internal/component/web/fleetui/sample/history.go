// Design: docs/architecture/fleet-interface.md -- the generated history of the sample estate
// Overview: source.go -- the sample data source
// Related: estate.go -- the rows and roles the figures are drawn from
// Related: views.go -- the peers and interfaces a sample counts
//
// No sample is stored. A figure is a function of the device, the metric and
// the time, measured from the start of the source, so every start draws the
// same charts: a daily pattern, one traffic spike two days back, and a gap
// wherever the device was offline. The only randomness is a hash of a fixed
// seed, the device's name, the metric and the sample's position, computed
// here: no shared generator, and no state.
//
// Each request generates only what it asks for, and at most historyPointsMax
// points per series: a longer period is answered at a wider interval, as a
// history that thins its older samples does.

package sample

import (
	"cmp"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/web/fleetui"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// The shape of the generated history.
const (
	// historyRetention is how far back the history goes.
	historyRetention = 7 * 24 * time.Hour
	// sampleInterval is the time between two samples of the last hours.
	sampleInterval = 30 * time.Second
	// historyPointsMax bounds the points of one generated series, and so the
	// work of one request: a request over n devices computes at most
	// n times historyPointsMax figures.
	historyPointsMax = 360
	// historySeed is what every hash starts from.
	historySeed = "ze-fleet-sample-1"
	// historyCapBytes is the cap of the sample hub's history.
	historyCapBytes = 256 * mebibyte

	day = 24 * time.Hour
	// dailyPeakAge is how long before the start the daily peak fell.
	dailyPeakAge = 2 * time.Hour
	// spikeAge is how long before the start the traffic spike peaked, and
	// spikeHalfWidth half of how long it lasted.
	spikeAge       = 52 * time.Hour
	spikeHalfWidth = 25 * time.Minute
)

// historyIntervals holds the sample intervals a series is answered at, the
// finest first. A period uses the first that fits it in historyPointsMax
// points, and the last fits the whole retention.
var historyIntervals = []time.Duration{sampleInterval, 5 * time.Minute, 30 * time.Minute}

// memoryBases names the devices whose memory use differs from their role's.
var memoryBases = map[string]float64{
	scriptAlarmDevice: 88,
}

// The constants of the 64-bit FNV-1a hash, and of the mixing step that
// spreads its last bytes over the whole word.
const (
	hashOffset uint64 = 14695981039346656037
	hashPrime  uint64 = 1099511628211
	mixFirst   uint64 = 0xbf58476d1ce4e5b9
	mixSecond  uint64 = 0x94d049bb133111eb
	// unitBits is how many bits of a hash make a fraction.
	unitBits = 53
)

// hashText folds text into sum.
func hashText(sum uint64, text string) uint64 {
	for at := range len(text) {
		sum ^= uint64(text[at])
		sum *= hashPrime
	}

	return sum
}

// hashBytes folds raw into sum.
func hashBytes(sum uint64, raw []byte) uint64 {
	for _, value := range raw {
		sum ^= uint64(value)
		sum *= hashPrime
	}

	return sum
}

// unit answers a fraction in [0, 1) drawn from sum.
func unit(sum uint64) float64 {
	sum ^= sum >> 30
	sum *= mixFirst
	sum ^= sum >> 27
	sum *= mixSecond
	sum ^= sum >> 31

	return float64(sum>>(64-unitBits)) / float64(uint64(1)<<unitBits)
}

// bias answers a fraction in [-1, 1) that belongs to the device and to what
// names the use, the same at every time. It is what makes two devices of one
// role differ.
func bias(name, use string) float64 {
	sum := hashText(hashText(hashText(hashOffset, historySeed), name), use)

	return 2*unit(sum) - 1
}

// jitter answers a fraction in [-1, 1) that belongs to one sample: the
// device, the metric and the sample's position counted from the start.
func jitter(name string, metric fleetui.Metric, offset time.Duration) float64 {
	var scratch [24]byte

	sum := hashText(hashText(hashOffset, historySeed), name)
	sum = hashBytes(sum, []byte{uint8(metric)})
	sum = hashBytes(sum, strconv.AppendInt(scratch[:0], int64(offset/sampleInterval), 10))

	return 2*unit(sum) - 1
}

// daily answers where offset falls in the daily pattern, from 0 in the night
// to 1 at the peak.
func daily(offset time.Duration) float64 {
	phase := 2 * math.Pi * float64(offset+dailyPeakAge) / float64(day)

	return 0.5 + 0.5*math.Cos(phase)
}

// spike answers how much of the traffic spike offset holds, from 0 outside
// it to 1 at its peak.
func spike(offset time.Duration) float64 {
	distance := (offset + spikeAge).Abs()
	if distance >= spikeHalfWidth {
		return 0
	}

	return 1 - float64(distance)/float64(spikeHalfWidth)
}

// figure answers the value of one metric of one device at offset from the
// start, and true. It answers false for a metric that is not a device's.
func figure(row *deviceRow, metric fleetui.Metric, offset time.Duration) (float64, bool) {
	facts := &roleTable[row.role]
	noise := jitter(row.name, metric, offset)

	switch metric {
	case fleetui.MetricCPU:
		return cpuFigure(row, offset, noise), true
	case fleetui.MetricMemory:
		base, special := memoryBases[row.name]
		if !special {
			base = facts.memoryBase + 6*bias(row.name, "memory")
		}

		return clampPercent(base + 3*daily(offset) + 0.8*noise), true
	case fleetui.MetricStorage:
		grown := 0.4 * float64(offset) / float64(day)

		return clampPercent(facts.storageBase + 8*bias(row.name, "storage") + grown), true
	case fleetui.MetricLoad:
		return cpuFigure(row, offset, noise) / 100 * float64(facts.cores), true
	case fleetui.MetricTrafficIn:
		return trafficFigure(row, offset, noise), true
	case fleetui.MetricTrafficOut:
		return 0.62 * trafficFigure(row, offset, noise), true
	case fleetui.MetricPeersEstablished:
		return float64(peersEstablishedAt(row, offset)), true
	case fleetui.MetricDevicesConnected, fleetui.MetricUnspecified:
		return 0, false
	}

	return 0, false
}

// cpuFigure answers the processor use of a device, in percent.
func cpuFigure(row *deviceRow, offset time.Duration, noise float64) float64 {
	base := roleTable[row.role].cpuBase + 5*bias(row.name, "cpu")

	return clampPercent(base + 22*daily(offset) + 25*spike(offset) + 4*noise)
}

// trafficFigure answers the traffic a device receives, in bits per second.
func trafficFigure(row *deviceRow, offset time.Duration, noise float64) float64 {
	peak := roleTable[row.role].trafficPeakBps * (1 + 0.3*bias(row.name, "traffic"))

	return peak * (0.25 + 0.75*daily(offset)) * (1 + 2*spike(offset)) * (1 + 0.06*noise)
}

// clampPercent keeps a percentage between 1 and 99.
func clampPercent(value float64) float64 {
	return min(max(value, 1), 99)
}

// reported reports whether the device sent a sample at offset from the start.
// It did not while it was offline, and the newcomer did not before it first
// connected.
func reported(row *deviceRow, snap *snapshot, offset time.Duration) bool {
	if away, offline := offlineFor[row.name]; offline {
		if offset > -away {
			return false
		}
	}
	if row.name == newcomerName {
		if offset < -newcomerConnectedFor {
			return false
		}
	}
	if outage, held := pastOutages[row.name]; held {
		if offset > -outage.from {
			if offset <= -outage.to {
				return false
			}
		}
	}
	if row.name != scriptDevice {
		return true
	}

	at := snap.start.Add(offset)
	for index := range snap.script.outages {
		if inOutage(&snap.script.outages[index], at) {
			return false
		}
	}

	return true
}

// inOutage reports whether at falls inside the outage.
func inOutage(period *outage, at time.Time) bool {
	if at.Before(period.from) {
		return false
	}
	if period.open {
		return true
	}

	return at.Before(period.to)
}

// floorTo answers the multiple of interval at or below offset.
func floorTo(offset, interval time.Duration) time.Duration {
	steps := offset / interval
	if steps*interval > offset {
		steps--
	}

	return steps * interval
}

// ceilTo answers the multiple of interval at or above offset.
func ceilTo(offset, interval time.Duration) time.Duration {
	steps := offset / interval
	if steps*interval < offset {
		steps++
	}

	return steps * interval
}

// latestOffset answers the offset of the last sample a device reported: the
// last sample position at the snapshot, or the last one before the device
// went offline.
func latestOffset(row *deviceRow, snap *snapshot) time.Duration {
	offset := floorTo(snap.now.Sub(snap.start), sampleInterval)

	if seen, offline := lastSeen(row, snap); offline {
		offset = floorTo(seen.Sub(snap.start), sampleInterval) - sampleInterval
	}

	return offset
}

// latestAt answers the last sample of a device at the snapshot. It is the
// last point of the device's series, with the counts of the device's peers
// and interfaces at that time.
func latestAt(row *deviceRow, snap *snapshot) fleetui.LatestSample {
	facts := &roleTable[row.role]
	offset := latestOffset(row, snap)

	cpu, _ := figure(row, fleetui.MetricCPU, offset)
	memory, _ := figure(row, fleetui.MetricMemory, offset)
	storage, _ := figure(row, fleetui.MetricStorage, offset)
	load, _ := figure(row, fleetui.MetricLoad, offset)
	trafficIn, _ := figure(row, fleetui.MetricTrafficIn, offset)
	trafficOut, _ := figure(row, fleetui.MetricTrafficOut, offset)

	if row.name == scriptAlarmDevice {
		if snap.script.alarmOpen {
			memory = scriptAlarmMemory
		}
	}

	peers := peerCountsAt(row, offset)
	links := interfaceCounts(row, offset)

	return fleetui.LatestSample{
		Reported:            true,
		At:                  snap.start.Add(offset),
		CPUPercent:          cpu,
		MemoryPercent:       memory,
		StoragePercent:      storage,
		Load:                load,
		MemoryUsedBytes:     uint64(memory / 100 * float64(facts.memoryBytes)),
		MemoryTotalBytes:    facts.memoryBytes,
		StorageUsedBytes:    uint64(storage / 100 * float64(facts.storageBytes)),
		StorageTotalBytes:   facts.storageBytes,
		TrafficInBps:        uint64(trafficIn),
		TrafficOutBps:       uint64(trafficOut),
		PeersEstablished:    peers.established,
		PeersDown:           peers.down,
		InterfacesUp:        links.up,
		InterfacesDown:      links.down,
		InterfacesAdminDown: links.adminDown,
	}
}

// seriesGrid is the sample positions of one answer: every multiple of
// interval from first, below end, all offsets from the start.
type seriesGrid struct {
	interval time.Duration
	first    time.Duration
	end      time.Duration
}

// gridFor answers the sample positions inside span, cut to the retention and
// to the snapshot's clock.
func gridFor(snap *snapshot, span fleetui.TimeSpan) seriesGrid {
	newest := snap.now.Sub(snap.start)
	oldest := newest - historyRetention

	from := max(span.From.Sub(snap.start), oldest)
	// A sample taken at the snapshot's own moment is in the answer.
	to := min(span.To.Sub(snap.start), newest+1)

	interval := historyIntervals[len(historyIntervals)-1]
	for _, candidate := range historyIntervals {
		if (to-from)/candidate <= historyPointsMax {
			interval = candidate
			break
		}
	}

	return seriesGrid{interval: interval, first: ceilTo(from, interval), end: to}
}

// seriesRefused is the refusal of a metric a series cannot be made of.
func seriesRefused(metric fleetui.Metric, scope string) error {
	problem := textbuf.New().Str("The history holds no ").Str(metric.String()).Str(" series for ").Str(scope).Str(".").Slice()

	return opererr.New(problem,
		"That metric is not one this kind of chart can draw, so there is no series to answer.",
		"open /fleet/")
}

// deviceSeries answers the samples of one metric of one device inside span.
func deviceSeries(
	snap *snapshot, device string, metric fleetui.Metric, span fleetui.TimeSpan,
) (fleetui.Series, error) {
	row, err := findRow(device)
	if err != nil {
		return fleetui.Series{}, err
	}
	if _, drawn := figure(row, metric, 0); !drawn {
		return fleetui.Series{}, seriesRefused(metric, "one device")
	}

	grid := gridFor(snap, span)
	points := make([]fleetui.Point, 0, historyPointsMax)

	for offset := grid.first; offset < grid.end; offset += grid.interval {
		if len(points) == historyPointsMax {
			break
		}
		if !reported(row, snap, offset) {
			continue
		}

		value, _ := figure(row, metric, offset)
		points = append(points, fleetui.Point{At: snap.start.Add(offset), Value: value})
	}

	return fleetui.Series{Metric: metric, Interval: grid.interval, Points: points}, nil
}

// estateSeries answers one metric summed over the named devices inside span.
// The connected devices metric counts the devices that reported.
func estateSeries(
	snap *snapshot, devices []string, metric fleetui.Metric, span fleetui.TimeSpan,
) (fleetui.Series, error) {
	if metric == fleetui.MetricUnspecified {
		return fleetui.Series{}, seriesRefused(metric, "the estate")
	}

	rows := make([]*deviceRow, 0, len(devices))
	for _, name := range devices {
		row, err := findRow(name)
		if err != nil {
			return fleetui.Series{}, err
		}
		rows = append(rows, row)
	}

	grid := gridFor(snap, span)
	points := make([]fleetui.Point, 0, historyPointsMax)

	for offset := grid.first; offset < grid.end; offset += grid.interval {
		if len(points) == historyPointsMax {
			break
		}

		total, reporting := estateFigure(rows, snap, metric, offset)
		if metric == fleetui.MetricDevicesConnected {
			total = float64(reporting)
		} else if reporting == 0 {
			continue
		}

		points = append(points, fleetui.Point{At: snap.start.Add(offset), Value: total})
	}

	return fleetui.Series{Metric: metric, Interval: grid.interval, Points: points}, nil
}

// estateFigure answers the sum of metric over the rows that reported at
// offset, and how many did.
func estateFigure(
	rows []*deviceRow, snap *snapshot, metric fleetui.Metric, offset time.Duration,
) (total float64, reporting int) {
	for _, row := range rows {
		if !reported(row, snap, offset) {
			continue
		}
		reporting++

		value, _ := figure(row, metric, offset)
		total += value
	}

	return total, reporting
}

// historyUsage answers how much the history of the sample estate would hold.
func historyUsage(snap *snapshot) fleetui.HistoryUsage {
	usage := fleetui.HistoryUsage{
		Retention: historyRetention,
		CapBytes:  historyCapBytes,
		Oldest:    snap.now.Add(-historyRetention),
		Largest:   make([]fleetui.HistoryDeviceUsage, 0, len(deviceRows)),
	}

	for at := range deviceRows {
		row := &deviceRows[at]
		size := roleTable[row.role].historyBytes + 8192*uint64(row.host)

		usage.SizeBytes += size
		usage.Largest = append(usage.Largest, fleetui.HistoryDeviceUsage{Device: row.name, SizeBytes: size})
	}

	slices.SortStableFunc(usage.Largest, func(a, b fleetui.HistoryDeviceUsage) int {
		return cmp.Compare(b.SizeBytes, a.SizeBytes)
	})
	usage.Largest = usage.Largest[:fleetui.HistoryDevicesMax]

	return usage
}
