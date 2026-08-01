// Design: docs/architecture/api/architecture.md -- BGP route filter pipeline
// Related: filterapi.go -- ModAccumulator.Op, whose caller obligation this counts violations of
// Related: metrics_test.go -- label vocabulary and the unwired-is-silent contract

package filterapi

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/metrics"
)

// attrModMetrics holds the attribute-modification contract metrics.
type attrModMetrics struct {
	removeBufferRefused metrics.CounterVec // labels: attribute
	editSpill           metrics.CounterVec // labels: store
}

// attrModMetricsPtr stays nil until the reactor wires a registry, so every
// recorder below guards its use. A build with metrics disabled leaves it nil.
var attrModMetricsPtr atomic.Pointer[attrModMetrics]

// SetMetricsRegistry creates the attribute-modification metrics from the given
// registry. A nil registry is a no-op, leaving the recorders disabled.
//
// Called from the reactor's metrics-enable block, NOT from a plugin's
// ConfigureMetrics callback. The handlers these metrics describe are registered
// at init() through RegisterAttrModHandler and run during the progressive build
// whether or not the owning plugin is running -- bgp-filter-community supplies
// the COMMUNITY handler even for a config with no `community { }` block at all.
// A ConfigureMetrics hook would therefore leave the counter dead in exactly the
// configuration where the contract violation it counts was first found. This is
// the same reasoning the reactor already records for filter.SetMetricsRegistry.
func SetMetricsRegistry(reg metrics.Registry) {
	if reg == nil {
		return
	}
	attrModMetricsPtr.Store(&attrModMetrics{
		removeBufferRefused: reg.CounterVec(
			"ze_bgp_attr_mod_remove_buffer_refused_total",
			"Attribute-modification Remove operations refused because the value buffer was not a whole number of wire values (a caller-contract violation of ModAccumulator.Op).",
			[]string{"attribute"},
		),
		editSpill: reg.CounterVec(
			"ze_bgp_update_edit_spill_total",
			"Per-destination edit sets whose slot, fragment, or arena store exceeded its inline capacity and allocated.",
			[]string{"store"},
		),
	})
}

// RecordEditSpill counts one destination whose edit set outgrew an inline store.
//
// The inline capacities come from a static census of the edit producers, not
// from a traffic histogram, so this counter is how that judgement is checked
// against real traffic rather than re-argued. A spill is correct behavior and is
// never refused; it is only an allocation on a path that is otherwise free.
//
// Attribute count and community-list length are peer-influenceable, so the label
// set is closed at the three stores and a peer cannot mint a time series.
func RecordEditSpill(store string) {
	if m := attrModMetricsPtr.Load(); m != nil {
		m.editSpill.With(store).Inc()
	}
}

// Edit-set store labels. A closed set: these are the only three stores an
// EditSet has.
const (
	EditStoreSlots     = "slot"
	EditStoreFragments = "fragment"
	EditStoreArena     = "arena"
)

// RecordRemoveBufferRefused counts one refused Remove operation for an
// attribute code.
//
// A log line alone is easy to miss in a soak run, and the violation it reports
// is by construction silent at the wire: the offending operation is skipped and
// the route goes out with the attribute unchanged. The counter turns "some
// producer is violating the arity contract" into something you can alert on
// rather than something you must already suspect before grepping for it.
//
// Only the attribute code is a label. The expected width and the actual buffer
// length identify the offending producer and go in the log line instead; as
// labels they would be unbounded in a second dimension for no gain.
func RecordRemoveBufferRefused(code uint8) {
	if m := attrModMetricsPtr.Load(); m != nil {
		m.removeBufferRefused.With(attrLabel(code)).Inc()
	}
}

// attrLabel names the well-known list-valued attribute codes and collapses
// everything else to "other".
//
// A decimal-formatted code would be a correct label too, but it would let an
// unexpected code mint a new time series on a path that is already an error.
// The three below are the only codes with a registered list-valued handler.
func attrLabel(code uint8) string {
	switch code {
	case 8:
		return "community"
	case 16:
		return "extended-community"
	case 32:
		return "large-community"
	default:
		return "other"
	}
}
