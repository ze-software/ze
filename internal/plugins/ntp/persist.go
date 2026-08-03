// Design: docs/features/interfaces.md -- NTP time persistence.
// The last-known time persists in the shared zefs store (ai/rules/architecture.md)
// via internal/core/statestore, not a loose file, so it lives inside the managed
// database.zefs alongside other appliance state on the writable /perm partition.

package ntp

import (
	"fmt"
	"time"

	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/pkg/zefs"
)

// saveTime persists t as RFC3339 text into the shared zefs store under the NTP
// last-time key. Best-effort: statestore is a no-op when no store is registered,
// so an absent store returns nil. The caller logs failures at debug and never
// treats them as fatal.
func saveTime(t time.Time) error {
	buf, err := t.MarshalText()
	if err != nil {
		return fmt.Errorf("ntp persist: marshal: %w", err)
	}
	if _, err := statestore.Put(zefs.KeyNTPLastTime.Pattern, buf); err != nil {
		return fmt.Errorf("ntp persist: store write: %w", err)
	}
	return nil
}

// loadTime reads a previously saved time from the shared zefs store. Returns an
// error when no store is registered, the key is absent, the blob is corrupt, or
// the saved year is out of range.
func loadTime() (time.Time, error) {
	buf, ok := statestore.Get(zefs.KeyNTPLastTime.Pattern)
	if !ok {
		return time.Time{}, fmt.Errorf("ntp persist: no saved time")
	}
	var t time.Time
	if err := t.UnmarshalText(buf); err != nil {
		return time.Time{}, fmt.Errorf("ntp persist: parse: %w", err)
	}
	// Reject absurd saved times.
	if t.Year() < 2020 || t.Year() > 2100 {
		return time.Time{}, fmt.Errorf("ntp persist: saved time out of range: %v", t)
	}
	return t, nil
}
