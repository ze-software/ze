// Design: docs/architecture/storage-backends.md -- shared configuration key space.
// Package storage owns the live tree and explicit blob artifacts.
package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/statestore"
)

// Storage, WriteGuard, FileMeta and VersionInfo are declared once in the leaf
// tier (internal/core/statestore) so that runtime-state consumers never import
// this component. The aliases keep this package's names as the spelling every
// component-side caller uses.
type (
	Storage     = statestore.Storage
	WriteGuard  = statestore.WriteGuard
	FileMeta    = statestore.FileMeta
	VersionInfo = statestore.VersionInfo
)

// FormatVersionStamp formats a time as YYYYMMDD-HHMMSS.mmm.
func FormatVersionStamp(t time.Time) string {
	return fmt.Sprintf("%s.%03d", t.Format("20060102-150405"), t.Nanosecond()/1e6)
}

// parseVersionStamp parses the canonical version timestamp.
func parseVersionStamp(s string) (time.Time, error) {
	parts := strings.SplitN(s, ".", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid version stamp: %s", s)
	}
	if len(parts[1]) != 3 {
		return time.Time{}, fmt.Errorf("invalid milliseconds: %s", s)
	}
	t, err := time.ParseInLocation("20060102-150405", parts[0], time.Local)
	if err != nil {
		return time.Time{}, err
	}
	ms, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	if ms < 0 {
		return time.Time{}, fmt.Errorf("milliseconds out of range: %d", ms)
	}
	if ms > 999 {
		return time.Time{}, fmt.Errorf("milliseconds out of range: %d", ms)
	}
	return t.Add(time.Duration(ms) * time.Millisecond), nil
}
