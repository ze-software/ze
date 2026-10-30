// Design: docs/architecture/isis/isis-6-lsdb.md -- the metric style of the own LSPs (narrow, transition, wide).
// Related: origination.go -- fragmentTLVs, the reader of the style
// Related: encode.go -- narrowISReachEntryBytes, the narrow IS-reachability entry
//
// RFC: rfc/short/rfc3787.md -- narrow metrics continue while any device lacks wide metrics (sec 5)

package lsdb

import "errors"

// MetricStyle selects which reachability TLVs the node's own LSPs carry. It is
// the instance-wide `metric-style` leaf: narrow emits only the 6-bit metric TLVs
// (2, 128), wide emits only the extended TLVs (22, 135), and transition emits
// both, which is step 2 of the RFC 3787 section 5.1 migration.
type MetricStyle uint8

// Metric style values. The zero value is Unspecified and is never a style:
// SetMetricStyle refuses it.
const (
	MetricStyleUnspecified MetricStyle = iota
	MetricStyleNarrow
	MetricStyleTransition
	MetricStyleWide
)

// ErrMetricStyle reports a metric style that names none of narrow, transition
// or wide.
var ErrMetricStyle = errors.New("isis: metric style is not narrow, transition or wide")

// String renders the style as the YANG enum token. Unspecified renders as
// "unspecified", a word the model does not admit.
func (s MetricStyle) String() string {
	// enumeration: gated by TestMetricStyleVocabularyMatchesModel
	switch s {
	case MetricStyleUnspecified:
		return "unspecified"
	case MetricStyleNarrow:
		return "narrow"
	case MetricStyleTransition:
		return "transition"
	case MetricStyleWide:
		return "wide"
	default:
		return "invalid"
	}
}

// valid reports whether s is one of the three styles an LSP can be built with.
func (s MetricStyle) valid() bool {
	return s.OriginatesNarrow() || s.OriginatesWide()
}

// OriginatesNarrow reports whether the style emits the narrow-metric TLVs: TLV 2
// (IS Reachability) and TLV 128 (IP Internal Reachability).
//
// RFC 3787 Section 5: "If not all devices in the IS-IS domain support wide
// metrics, narrow metrics MUST continue to be used."
func (s MetricStyle) OriginatesNarrow() bool {
	return s == MetricStyleNarrow || s == MetricStyleTransition
}

// OriginatesWide reports whether the style emits the wide-metric TLVs: TLV 22
// (Extended IS Reachability) and TLV 135 (Extended IP Reachability).
func (s MetricStyle) OriginatesWide() bool {
	return s == MetricStyleWide || s == MetricStyleTransition
}
