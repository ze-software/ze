// Design: docs/architecture/core-design.md -- GR restart marker for Restarting Speaker detection
//
// Package grmarker implements RFC 4724 Restarting Speaker detection using a
// GR marker in zefs. On graceful restart, the engine writes a marker with an
// expiry timestamp. On startup, the engine reads the marker and sets R=1 in
// GR capabilities for connections within the restart window.
package grmarker

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin"
	"codeberg.org/thomas-mangin/ze/pkg/zefs"
)

// Store is the minimal interface for reading/writing the GR marker.
// Satisfied by both *zefs.BlobStore and storage.Storage.
type Store interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	Remove(name string) error
}

// markerKey is the zefs key for the GR restart marker.
// Treat as const -- var only because Go requires const values to be compile-time literals.
var markerKey = zefs.KeyGRMarker.Pattern

// grCapCode is the BGP capability code for Graceful Restart (RFC 4724).
const grCapCode = 64

// markerLen is the length of the marker value (8-byte big-endian int64 UNIX seconds).
const markerLen = 8

// rBitMask is the Restart State bit mask for byte 0 of the GR capability value.
// RFC 4724 Section 3: bit 7 of byte 0 (MSB of Restart Flags nibble).
const rBitMask = 0x80

// Write writes a GR restart marker to zefs with the given expiry timestamp.
// RFC 4724 Section 4.1: the Restarting Speaker MUST set the Restart State
// bit in the Graceful Restart Capability of the OPEN message.
func Write(store Store, expiresAt time.Time) error {
	buf := make([]byte, markerLen)
	binary.BigEndian.PutUint64(buf, uint64(expiresAt.Unix()))
	return store.WriteFile(markerKey, buf, 0)
}

// Read reads the GR restart marker from zefs.
// Returns the expiry time and true if the marker is valid (exists and not expired).
// Returns zero time and false if the marker is missing, corrupt, or expired.
func Read(store Store) (time.Time, bool) {
	data, err := store.ReadFile(markerKey)
	if err != nil {
		return time.Time{}, false
	}
	if len(data) != markerLen {
		return time.Time{}, false
	}

	ts := int64(binary.BigEndian.Uint64(data))
	expiry := time.Unix(ts, 0)

	if !time.Now().Before(expiry) {
		return time.Time{}, false
	}

	return expiry, true
}

// Remove removes the GR restart marker from zefs.
// Safe to call when no marker exists.
func Remove(store Store) error {
	err := store.Remove(markerKey)
	if isNotExist(err) {
		return nil
	}
	return err
}

// isNotExist checks if an error indicates a missing file.
// Uses errors.Is to match regardless of wrapping (PathError or bare ErrNotExist).
func isNotExist(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

// MaxRestartTime returns the maximum restart-time (in seconds) across all
// code-64 (Graceful Restart) capabilities in the given slice.
// RFC 4724 Section 3: restart-time is bits 4-15 of the first 2 bytes.
func MaxRestartTime(caps []plugin.InjectedCapability) int {
	maxRT := 0
	for _, ic := range caps {
		if ic.Code != grCapCode || len(ic.Value) < 2 {
			continue
		}
		// Restart-time: lower nibble of byte 0 (4 bits) + all of byte 1 (8 bits) = 12 bits.
		rt := (int(ic.Value[0]) & 0x0F) << 8
		rt |= int(ic.Value[1])
		if rt > maxRT {
			maxRT = rt
		}
	}
	return maxRT
}

// SetRBit returns a copy of the capabilities with the Restart State bit (R=1)
// set on all code-64 capabilities that have at least 2 bytes of Value.
// Non-code-64 capabilities and short values are returned unchanged.
// The original Value slices are never modified.
// RFC 4724 Section 3: R-bit is bit 7 of byte 0 (0x80 mask).
func SetRBit(caps []plugin.InjectedCapability) []plugin.InjectedCapability {
	result := make([]plugin.InjectedCapability, len(caps))
	for i, ic := range caps {
		if ic.Code == grCapCode && len(ic.Value) >= 2 {
			// Copy the Value slice so the original is not modified.
			valueCopy := make([]byte, len(ic.Value))
			copy(valueCopy, ic.Value)
			valueCopy[0] |= rBitMask
			result[i] = plugin.InjectedCapability{
				Code:     ic.Code,
				Value:    valueCopy,
				Plugin:   ic.Plugin,
				PeerAddr: ic.PeerAddr,
			}
		} else {
			result[i] = ic
		}
	}
	return result
}
