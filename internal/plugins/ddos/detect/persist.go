// Design: plan/learned/1108-ddos-detect-enhancements.md -- baseline persistence across restart.
// Restoring the rolling baseline on startup skips the BaselineWindow re-warm (and
// the StartupGrace blind window) that an in-memory-only baseline suffers after every
// restart, update, or reconfigure. Pattern copied from the traffic tc-snapshot store
// (internal/plugins/traffic/netlink/snapshot_linux.go): a versioned JSON blob written
// atomically (temp file + rename), rejected on version/sanity mismatch.

package detect

import (
	"encoding/json"
	"os"
	"path/filepath"

	"codeberg.org/thomas-mangin/ze/internal/core/env"
	"codeberg.org/thomas-mangin/ze/internal/core/paths"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

// baselineStateVersion guards the on-disk format; bump it on any incompatible
// change to persistedBaseline so an old file is rejected rather than misread.
const baselineStateVersion = 1

// baselineSaveInterval is how often (in ticks, ~1s each) the detector persists its
// baselines while running, as a belt-and-braces against a hard crash between the
// save-on-Stop points. 300 ticks ~= 5 minutes.
const baselineSaveInterval = 300

// persistedBaseline is the on-disk snapshot of both rolling baselines.
type persistedBaseline struct {
	Version int           `json:"version"`
	Pps     baselineState `json:"pps"`
	Bps     baselineState `json:"bps"`
}

// baselineStatePath returns <config-dir>/state/ddos-detect-baseline.json, resolving
// the config dir from the ze.config.dir env var then the binary-location default
// (same convention as the traffic tc-snapshot and crash-log stores).
func baselineStatePath() string {
	dir := env.Get("ze.config.dir")
	if dir == "" {
		dir = paths.DefaultConfigDir()
	}
	return filepath.Join(dir, "state", "ddos-detect-baseline.json")
}

// saveBaselines writes the PPS+BPS snapshots to path atomically (temp file + rename).
// Best-effort: the caller logs failures at debug and never treats them as fatal.
func saveBaselines(path string, pps, bps baselineState) error {
	blob := persistedBaseline{Version: baselineStateVersion, Pps: pps, Bps: bps}
	data, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	var tb textbuf.Buffer
	tb.Str(path).Str(".tmp")
	tmp := tb.String()
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadBaselines reads a persisted snapshot. It returns ok=false when the file is
// absent, unreadable, malformed, or a version mismatch -- the per-series sample and
// sanity guards live in baseline.restore, which the caller applies next.
func loadBaselines(path string) (persistedBaseline, bool) {
	data, err := os.ReadFile(path) //nolint:gosec // path is the Ze state-dir baseline store, not external input
	if err != nil {
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
