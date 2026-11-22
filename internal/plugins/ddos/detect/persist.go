// Design: docs/architecture/ddos/ddos-detect-enhancements.md -- baseline persistence across restart.
// Restoring the rolling baseline on startup skips the BaselineWindow re-warm (and
// the StartupGrace blind window) that an in-memory-only baseline suffers after every
// restart, update, or reconfigure. The snapshot is a versioned JSON blob persisted
// in the daemon's managed store (ai/rules/architecture.md) through the plugin SDK
// state RPCs (p.StateGet/p.StatePut), not a loose file or direct statestore access,
// so appliance state lives inside the database/ tree on both internal and forked
// transports.

package detect

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/zefs"
)

// stateCallTimeout bounds one daemon state RPC round trip.
const stateCallTimeout = 5 * time.Second

// stateClient is the daemon-owned state RPC surface (*sdk.Plugin satisfies it).
type stateClient interface {
	StateGet(ctx context.Context, key string) ([]byte, bool, error)
	StatePut(ctx context.Context, key string, data []byte) error
}

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

// saveBaselines writes the PPS+BPS snapshots into the daemon's managed zefs store
// under the ddos-detect baseline key via the SDK state RPC. Best-effort: a no-op
// when no state client is attached or the daemon reports no store registered
// (rpc.StateUnavailable). The caller logs failures at debug and never treats
// them as fatal.
func saveBaselines(ctx context.Context, client stateClient, pps, bps baselineState) error {
	if client == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	blob := persistedBaseline{Version: baselineStateVersion, Pps: pps, Bps: bps}
	data, err := json.Marshal(blob)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, stateCallTimeout)
	defer cancel()
	if err := client.StatePut(callCtx, zefs.KeyDDoSDetectBaseline.Pattern, data); err != nil {
		var stateErr *rpc.StateError
		if errors.As(err, &stateErr) {
			if stateErr.Status == rpc.StateUnavailable {
				return nil
			}
		}
		return err
	}
	return nil
}

// loadBaselines reads a persisted snapshot from the daemon's managed zefs store
// via the SDK state RPC. It returns ok=false when no client is attached, the
// store or key is absent, the read fails, the JSON is malformed, or the version
// mismatches -- the per-series sample and sanity guards live in baseline.restore,
// applied next.
func loadBaselines(ctx context.Context, client stateClient) (persistedBaseline, bool) {
	if client == nil {
		return persistedBaseline{}, false
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, stateCallTimeout)
	defer cancel()
	data, ok, err := client.StateGet(callCtx, zefs.KeyDDoSDetectBaseline.Pattern)
	if err != nil {
		logger().Debug("ddos-detect: baseline load failed", "error", err)
		return persistedBaseline{}, false
	}
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
