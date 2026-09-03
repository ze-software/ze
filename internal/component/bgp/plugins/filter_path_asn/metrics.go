// Design: docs/plugin-development/metrics.md -- reject-asn operator signal
// Detail: config.go -- the position vocabulary the position label names
// Related: filter_path_asn.go -- handleFilterUpdate, the only caller of recordReject
//
// The one counter this filter publishes.
//
// A dropped route is invisible to an operator otherwise: the reject log line
// sits at Info and says nothing about how often it happens. The counter answers
// "how many, in which direction, and what caught them", and the log line answers
// "which peer and which ASN". The split is deliberate: a peer address in a label
// makes cardinality grow with the session count, so peer identity stays in the
// log (the role plugin's metrics.go states the same rule).
//
// Every label value is a compile-time constant, so the series count is fixed at
// two directions times the slots below, whatever an operator configures.
package filter_path_asn

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// metricRejects counts every route this filter refuses.
//
// The name follows docs/plugin-development/metrics.md
// (ze_{scope}_{subject}_{event}_total) with the scope filter_path_asn, which is
// the package name rather than the registration name with its hyphens stripped:
// the `bgp` prefix is redundant on a BGP filter, and `bgpfilterpathasn` cannot
// be read.
const metricRejects = "ze_filter_path_asn_rejects_total"

// rejectSlot is one reason the filter refused a route, at the granularity the
// counter reports it.
//
// The first five values are numerically EQUAL to the position constants of
// config.go, so a hit's position converts to its slot with no lookup table and
// no mapping that a later position could fall out of. TestSlotsAlignWithPositions
// holds that alignment.
//
// slotUnspecified is the zero and is never a real reject: positionAt answers
// direct, transit or origin for every index and an nth match answers nth, so
// nothing produces it. It is counted rather than dropped because a zero position
// reaching here means positionAt returned the value that is not a place, and a
// series that moves off zero says so (ai/rules/principles.md).
type rejectSlot uint8

const (
	slotUnspecified rejectSlot = rejectSlot(positionUnspecified)
	slotDirect      rejectSlot = rejectSlot(positionDirect)
	slotTransit     rejectSlot = rejectSlot(positionTransit)
	slotOrigin      rejectSlot = rejectSlot(positionOrigin)
	slotNth         rejectSlot = rejectSlot(positionNth)
	// slotPattern is a route a `regex` pattern matched. No position
	// applies to a pattern, so the position label carries the keyword an operator
	// wrote it under.
	slotPattern rejectSlot = slotNth + 1
	// slotUnknownList and slotUnconfigured are the two fail-closed rejects. No
	// ASN and no pattern decided them, so their position label is the word the
	// position enum answers for a value that is not a place.
	slotUnknownList  rejectSlot = slotPattern + 1
	slotUnconfigured rejectSlot = slotUnknownList + 1

	// slotCount is the array length, never a real slot. Each value above states
	// its own predecessor because the first four are pinned to the position
	// constants, which breaks the iota run they would otherwise continue.
	slotCount rejectSlot = slotUnconfigured + 1
)

// Label values. Bounded and compile-time constant.
const (
	positionLabelRegex = "regex"

	reasonLabelListedASN    = "listed-asn"
	reasonLabelUnknownList  = "unknown-list"
	reasonLabelUnconfigured = "unconfigured"
)

// slotPositionLabels names where in the AS_PATH the reject was decided.
// Each position uses the keyword the YANG declares, so the config file, the log
// line and the metric all say the same thing. An nth reject reads position="nth"
// with no index: the index is unbounded operator input and belongs in the log
// line rather than in a label that would grow the series count.
var slotPositionLabels = [slotCount]string{
	slotUnspecified:  positionUnspecified.String(),
	slotDirect:       positionDirect.String(),
	slotTransit:      positionTransit.String(),
	slotOrigin:       positionOrigin.String(),
	slotNth:          positionNth.String(),
	slotPattern:      positionLabelRegex,
	slotUnknownList:  positionUnspecified.String(),
	slotUnconfigured: positionUnspecified.String(),
}

// slotReasonLabels names what the filter was acting on. A pattern match reads
// listed-asn beside position="regex": both arms of a list are the operator's
// listing, and the position label is what separates them.
var slotReasonLabels = [slotCount]string{
	slotUnspecified:  reasonLabelListedASN,
	slotDirect:       reasonLabelListedASN,
	slotTransit:      reasonLabelListedASN,
	slotOrigin:       reasonLabelListedASN,
	slotNth:          reasonLabelListedASN,
	slotPattern:      reasonLabelListedASN,
	slotUnknownList:  reasonLabelUnknownList,
	slotUnconfigured: reasonLabelUnconfigured,
}

// flow is the direction dimension of the counter. It is an index rather than the
// direction string, so the hot path resolves a child with two array lookups.
type flow uint8

const (
	flowImport flow = iota
	flowExport
	flowCount // sentinel: array length, never a real direction
)

// directionExport is the direction label for every call that is not an import.
// senderOf takes the same reading of a direction it does not recognize: what is
// not an import carries the destination peer, so it is an export.
const directionExport = "export"

var flowLabels = [flowCount]string{
	flowImport: directionImport,
	flowExport: directionExport,
}

// flowOf reads the direction the filter input carried.
func flowOf(direction string) flow {
	if direction == directionImport {
		return flowImport
	}
	return flowExport
}

// filterMetrics holds the reject counter's children, one per direction and slot,
// resolved once at build time.
//
// CounterVec.With allocates a []string for its variadic on every call, so
// resolving here is what keeps the reject path allocation-free
// (ai/rules/performance.md). Pre-creating every child also makes each series
// present at 0 from startup, so an alert on a rate does not wait for the series
// to appear.
type filterMetrics struct {
	rejects [flowCount][slotCount]metrics.Counter
}

var filterMetricsPtr atomic.Pointer[filterMetrics]

// buildMetrics registers the metric set against reg.
func buildMetrics(reg metrics.Registry) *filterMetrics {
	rejects := reg.CounterVec(metricRejects,
		"Routes refused by a reject-asn list, by direction, position and reason.",
		[]string{"direction", "position", "reason"})

	m := &filterMetrics{}
	for f := range int(flowCount) {
		for s := range int(slotCount) {
			m.rejects[f][s] = rejects.With(flowLabels[f], slotPositionLabels[s], slotReasonLabels[s])
		}
	}
	return m
}

// setMetricsRegistry publishes metrics backed by the host registry. Called via
// the plugin Registration's ConfigureMetrics before RunEngine.
func setMetricsRegistry(reg metrics.Registry) { filterMetricsPtr.Store(buildMetrics(reg)) }

// fmetrics returns the current metric set, lazily defaulting to a no-op
// registry.
//
// The filter is registered from init() and the engine calls it whether or not
// ConfigureMetrics ever ran: telemetry can be off, and the plugin can run as an
// external process, where no host registry reaches it. Counting must not depend
// on either, so a missing registry is a no-op counter rather than a nil
// dereference on the reject path.
func fmetrics() *filterMetrics {
	if m := filterMetricsPtr.Load(); m != nil {
		return m
	}
	filterMetricsPtr.CompareAndSwap(nil, buildMetrics(metrics.NopRegistry{}))
	return filterMetricsPtr.Load()
}

// recordReject counts one refused route. It is the single counting seam, so
// every reject handleFilterUpdate returns passes through here.
func recordReject(direction string, slot rejectSlot) {
	fmetrics().rejects[flowOf(direction)][slot].Inc()
}
