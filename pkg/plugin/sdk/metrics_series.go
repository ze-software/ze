// Design: docs/plugin-development/metrics.md -- public series retention contract
// Related: ../../../internal/core/metrics/series_custody.go -- canonical contract
package sdk

import "github.com/ze-software/ze/internal/core/metrics"

// SeriesRetainer is the optional vector capability for shared series cleanup.
// Callers MUST retain successfully before updating a label set, and MUST Release
// the returned SeriesLease after their final update. Delete refuses while any
// lease remains. This alias exposes the canonical contract to plugin callers;
// Registry and vector interfaces do not require the optional capability.
type SeriesRetainer = metrics.SeriesRetainer

// SeriesLease retains one holder of the original metric series. Callers MUST
// obtain it from SeriesRetainer before publishing, and MUST Release it after
// their final update. Copies share release state; Release is idempotent and
// safe for concurrent use. A zero lease has no metric effect.
type SeriesLease = metrics.SeriesLease
