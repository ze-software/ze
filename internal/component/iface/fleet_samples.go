// Design: docs/architecture/fleet-config.md — interface-owned fleet metric sample producer
// Related: rate_command.go — plugin command declarations and dispatch
// Related: cmd/register_rate.go — daemon proxy registration forwarding to the iface plugin

package iface

import (
	"cmp"
	"math"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/pkg/fleet"
)

// FleetSourceCommand is the hidden read-only plugin command that returns the
// interface owner's bounded fleet metric sample snapshot.
const FleetSourceCommand = "show fleet source iface"

const (
	linkTypeDevice   = "device"
	vppTypeHardware  = "IF_API_TYPE_HARDWARE"
	ownedAliasPrefix = "ze:owned:"
)

type physicalCounterSample struct {
	name              string
	index             int
	counterGeneration uint64
	rxBytes           uint64
	txBytes           uint64
}

// fleetSampleTracker computes physical interface counts and aggregate physical
// traffic rates across stable physical-interface membership, counter-generation
// continuity, and non-decreasing byte counters. Safe for concurrent use.
type fleetSampleTracker struct {
	mu           sync.Mutex
	ownerTracker *rateTracker
	hasBaseline  bool
	prevAt       time.Time
	prev         []physicalCounterSample
	list         func() ([]InterfaceInfo, error)
	now          func() time.Time
}

var defaultFleetSampler = newFleetSampleTracker(listLiveInterfaces, time.Now)

func newFleetSampleTracker(list func() ([]InterfaceInfo, error), now func() time.Time) *fleetSampleTracker {
	return &fleetSampleTracker{
		list: list,
		now:  now,
	}
}

func listLiveInterfaces() ([]InterfaceInfo, error) {
	b := GetBackend()
	if b == nil {
		return nil, ErrNoBackendLoaded
	}
	return b.ListInterfaces()
}

func (t *fleetSampleTracker) syncOwner(tracker *rateTracker) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.ownerTracker == tracker {
		return
	}
	t.ownerTracker = tracker
	t.hasBaseline = false
	t.prev = nil
	t.prevAt = time.Time{}
}

// Sample reads live interfaces, counts physical interfaces and operationally
// up physical interfaces, and computes aggregate bits-per-second traffic rates
// whenever the physical membership, counter generations, monotonic counters,
// and elapsed time are continuous since the prior tick.
func (t *fleetSampleTracker) Sample() (fleet.SourceSnapshot, error) {
	if t.list == nil {
		return fleet.SourceSnapshot{}, ErrNoBackendLoaded
	}
	infos, err := t.list()
	if err != nil {
		return fleet.SourceSnapshot{}, err
	}

	phys := make([]InterfaceInfo, 0, len(infos))
	upCount := 0
	allHaveStats := true
	for i := range infos {
		info := &infos[i]
		if !isPhysicalInterface(info) {
			continue
		}
		phys = append(phys, *info)
		if info.State == LinkStateUp {
			upCount++
		}
		if info.Stats == nil {
			allHaveStats = false
		}
	}

	readings := make([]fleet.SourceReading, 0, 4)
	readings = append(readings,
		fleet.SourceReading{
			Figure: fleet.FigureInterfacesConfigured,
			Unit:   fleet.SampleUnitCount,
			Value:  float64(len(phys)),
		},
		fleet.SourceReading{
			Figure: fleet.FigureInterfacesUp,
			Unit:   fleet.SampleUnitCount,
			Value:  float64(upCount),
		},
	)

	if len(phys) == 0 {
		t.resetBaseline()
		return fleet.SourceSnapshot{
			Owner:    fleet.SampleOwnerIface,
			Readings: readings,
		}, nil
	}
	if !allHaveStats {
		t.resetBaseline()
		return fleet.SourceSnapshot{
			Owner:    fleet.SampleOwnerIface,
			Readings: readings,
		}, nil
	}

	curr := make([]physicalCounterSample, len(phys))
	for i := range phys {
		curr[i] = physicalCounterSample{
			name:              phys[i].Name,
			index:             phys[i].Index,
			counterGeneration: phys[i].CounterGeneration,
			rxBytes:           phys[i].Stats.RxBytes,
			txBytes:           phys[i].Stats.TxBytes,
		}
	}
	slices.SortFunc(curr, comparePhysicalSample)

	nowFn := t.now
	if nowFn == nil {
		nowFn = time.Now
	}
	now := nowFn()

	rxBPS, txBPS, hasRate := t.advanceBaseline(curr, now)
	if hasRate {
		readings = append(readings,
			fleet.SourceReading{
				Figure: fleet.FigureTrafficIn,
				Unit:   fleet.SampleUnitBPS,
				Value:  rxBPS,
			},
			fleet.SourceReading{
				Figure: fleet.FigureTrafficOut,
				Unit:   fleet.SampleUnitBPS,
				Value:  txBPS,
			},
		)
	}

	return fleet.SourceSnapshot{
		Owner:    fleet.SampleOwnerIface,
		Readings: readings,
	}, nil
}

func (t *fleetSampleTracker) resetBaseline() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.hasBaseline = false
	t.prev = nil
	t.prevAt = time.Time{}
}

func (t *fleetSampleTracker) advanceBaseline(curr []physicalCounterSample, now time.Time) (rxBPS, txBPS float64, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.hasBaseline {
		t.prev = curr
		t.prevAt = now
		t.hasBaseline = true
		return 0, 0, false
	}

	prev := t.prev
	prevAt := t.prevAt
	t.prev = curr
	t.prevAt = now
	t.hasBaseline = true

	elapsed := now.Sub(prevAt).Seconds()
	if elapsed <= 0 {
		return 0, 0, false
	}
	deltaRx, deltaTx, continuous := computePhysicalTrafficDeltas(prev, curr)
	if !continuous {
		return 0, 0, false
	}
	rxBPS = deltaRx * 8.0 / elapsed
	txBPS = deltaTx * 8.0 / elapsed
	if !isFiniteNonNegativeRate(rxBPS) {
		return 0, 0, false
	}
	if !isFiniteNonNegativeRate(txBPS) {
		return 0, 0, false
	}
	return rxBPS, txBPS, true
}

func fleetSampleAnswer(tracker *rateTracker, sampler *fleetSampleTracker) (fleet.SourceSnapshot, error) {
	if tracker == nil {
		return fleet.SourceSnapshot{}, errRateTrackerNotRunning
	}
	if sampler == nil {
		return fleet.SourceSnapshot{}, errRateTrackerNotRunning
	}
	sampler.syncOwner(tracker)
	return sampler.Sample()
}

func isPhysicalInterface(info *InterfaceInfo) bool {
	if info == nil {
		return false
	}
	if info.Name == "" {
		return false
	}
	if info.Name == "lo" {
		return false
	}
	if info.VlanID > 0 {
		return false
	}
	if info.ParentIndex > 0 {
		if info.ParentIndex != info.Index {
			return false
		}
	}
	if strings.HasPrefix(info.Alias, ownedAliasPrefix) {
		return false
	}
	switch info.Type {
	case linkTypeDevice:
		if info.MAC == "" {
			if strings.HasPrefix(info.Name, "ppp") {
				return false
			}
		}
		return true
	case zeTypeEthernet, vppTypeHardware:
		return true
	default:
		return false
	}
}

func comparePhysicalSample(a, b physicalCounterSample) int {
	if c := cmp.Compare(a.name, b.name); c != 0 {
		return c
	}
	return cmp.Compare(a.index, b.index)
}

func computePhysicalTrafficDeltas(prev, curr []physicalCounterSample) (deltaRx, deltaTx float64, ok bool) {
	if len(prev) != len(curr) {
		return 0, 0, false
	}
	for i := range curr {
		if prev[i].name != curr[i].name {
			return 0, 0, false
		}
		if prev[i].index != curr[i].index {
			return 0, 0, false
		}
		if prev[i].counterGeneration != curr[i].counterGeneration {
			return 0, 0, false
		}
		if curr[i].rxBytes < prev[i].rxBytes {
			return 0, 0, false
		}
		if curr[i].txBytes < prev[i].txBytes {
			return 0, 0, false
		}
		deltaRx += float64(curr[i].rxBytes - prev[i].rxBytes)
		deltaTx += float64(curr[i].txBytes - prev[i].txBytes)
	}
	if !isFiniteNonNegativeRate(deltaRx) {
		return 0, 0, false
	}
	if !isFiniteNonNegativeRate(deltaTx) {
		return 0, 0, false
	}
	return deltaRx, deltaTx, true
}

func isFiniteNonNegativeRate(v float64) bool {
	if math.IsNaN(v) {
		return false
	}
	if math.IsInf(v, 0) {
		return false
	}
	return v >= 0
}
