// Design: plan/learned/1108-ddos-detect-enhancements.md -- baseline persistence across restart.
// Restoring the rolling baseline on startup skips the BaselineWindow re-warm (and
// the StartupGrace blind window) that an in-memory-only baseline suffers after every
// restart, update, or reconfigure. The snapshot is a versioned JSON blob persisted
// in the shared zefs store (ai/rules/zefs-persistence.md) via internal/core/statestore,
// not a loose file, so appliance state lives inside the managed database.zefs.

package detect

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// baselineStateVersion guards the on-disk format; bump it on any incompatible
// change to persistedBaseline so an old blob is rejected rather than misread.
const baselineStateVersion = 1

// baselineSaveInterval is how often (in ticks, ~1s each) the detector persists its
// baselines while running, as a belt-and-braces against a hard crash between the
// save-on-Stop points. 300 ticks ~= 5 minutes.
const baselineSaveInterval = 300

// persistedBaseline is the stored snapshot of both rolling baselines.
type persistedBaseline struct {
	Version int           `json:"version"`
	Pps     baselineState `json:"pps"`
	Bps     baselineState `json:"bps"`
}

// saveBaselines writes the PPS+BPS snapshots into the shared zefs store under the
// ddos-detect baseline key. Best-effort: a no-op when no store is registered.
// The caller logs failures at debug and never treats them as fatal.
func saveBaselines(pps, bps baselineState) error {
	blob := persistedBaseline{Version: baselineStateVersion, Pps: pps, Bps: bps}
	data, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	_, err = statestore.Put(zefs.KeyDDoSDetectBaseline.Pattern, data)
	return err
}

// loadBaselines reads a persisted snapshot from the shared zefs store. It returns
// ok=false when the store or key is absent, malformed, or a version mismatch -- the
// per-series sample and sanity guards live in baseline.restore, applied next.
func loadBaselines() (persistedBaseline, bool) {
	data, ok := statestore.Get(zefs.KeyDDoSDetectBaseline.Pattern)
	if !ok {
		return persistedBaseline{}, false
	}
	var blob persistedBaseline
	if err := json.Unmarshal(data, &blob); err != nil {
		return persistedBaseline{}, false
	}
	if blob.Version != baselineStateVersion {
		return persistedBaseline{}, false
	}
	return blob, true
}
