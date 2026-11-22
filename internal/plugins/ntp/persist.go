// Design: docs/features/interfaces.md -- NTP time persistence.
// The last-known time persists in the daemon's managed store
// (ai/rules/architecture.md) through the plugin SDK state RPCs
// (p.StateGet/p.StatePut), not a loose file or direct statestore access, so it
// lives inside the database/ tree on both internal and forked transports.

package ntp

import (
	"context"
	"errors"
	"fmt"
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

// saveTime persists t as RFC3339 text into the daemon's managed zefs store
// under the NTP last-time key via the SDK state RPC. Best-effort when no state
// client is attached or the daemon reports no store registered
// (rpc.StateUnavailable). Any other write error is returned; periodic saves
// log it and continue, while final shutdown saves surface it as a refusal.
func saveTime(ctx context.Context, client stateClient, t time.Time) error {
	if client == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	buf, err := t.MarshalText()
	if err != nil {
		return fmt.Errorf("ntp persist: marshal: %w", err)
	}
	callCtx, cancel := context.WithTimeout(ctx, stateCallTimeout)
	defer cancel()
	if err := client.StatePut(callCtx, zefs.KeyNTPLastTime.Pattern, buf); err != nil {
		var stateErr *rpc.StateError
		if errors.As(err, &stateErr) {
			if stateErr.Status == rpc.StateUnavailable {
				return nil
			}
		}
		return fmt.Errorf("ntp persist: store write: %w", err)
	}
	return nil
}

// loadTime reads a previously saved time from the daemon's managed zefs store
// via the SDK state RPC. Returns an error when no client is attached, the
// store is unavailable or unreadable, the key is absent, the blob is corrupt,
// or the saved year is out of range.
func loadTime(ctx context.Context, client stateClient) (time.Time, error) {
	if client == nil {
		return time.Time{}, errors.New("ntp persist: no saved time")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	callCtx, cancel := context.WithTimeout(ctx, stateCallTimeout)
	defer cancel()
	buf, ok, err := client.StateGet(callCtx, zefs.KeyNTPLastTime.Pattern)
	if err != nil {
		return time.Time{}, fmt.Errorf("ntp persist: store read: %w", err)
	}
	if !ok {
		return time.Time{}, errors.New("ntp persist: no saved time")
	}
	var t time.Time
	if err := t.UnmarshalText(buf); err != nil {
		return time.Time{}, fmt.Errorf("ntp persist: parse: %w", err)
	}
	// Reject absurd saved times.
	if t.Year() < 2020 {
		return time.Time{}, fmt.Errorf("ntp persist: saved time out of range: %v", t)
	}
	if t.Year() > 2100 {
		return time.Time{}, fmt.Errorf("ntp persist: saved time out of range: %v", t)
	}
	return t, nil
}
