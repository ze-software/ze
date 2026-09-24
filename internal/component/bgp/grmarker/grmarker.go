// Design: docs/architecture/core-design.md -- GR restart marker for Restarting Speaker detection
// RFC: rfc/short/rfc4724.md
//
// Package grmarker implements RFC 4724 Restarting Speaker detection using a
// GR marker in the managed store. On graceful restart, the engine writes a
// marker with an expiry timestamp. On startup, the engine reads the marker and
// sets R=1 in GR capabilities for connections within the restart window.
package grmarker

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/pkg/zefs"
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

// fBitMask is the Forwarding State bit mask of a per-family Flags octet.
// RFC 4724 Section 3: "The most significant bit is defined as the Forwarding
// State (F) bit".
const fBitMask = 0x80

// grPrefixLen is the length of the Restart Flags and Restart Time pair that
// opens a code-64 capability value, before the first address-family tuple.
const grPrefixLen = 2

// grTupleLen is the wire length of one <AFI, SAFI, Flags for address family>
// tuple: 2 octets of AFI, 1 of SAFI, 1 of flags.
const grTupleLen = 4

// grTupleFlagsOffset is the position of the Flags octet inside one tuple.
const grTupleFlagsOffset = 3

// Write writes a GR restart marker to zefs with the given expiry timestamp.
// RFC 4724 Section 4.1: the Restarting Speaker MUST set the Restart State
// bit in the Graceful Restart Capability of the OPEN message.
func Write(store Store, expiresAt time.Time) error {
	buf := make([]byte, markerLen)
	binary.BigEndian.PutUint64(buf, uint64(expiresAt.Unix()))
	return store.WriteFile(markerKey, buf, 0)
}

// Read reads the GR restart marker from zefs and judges it against now.
// Returns the expiry time and true if the marker is valid (exists and now is
// before its expiry). Returns zero time and false if the marker is missing,
// corrupt, or expired. The caller passes now, so the verdict does not depend on
// how long the store read took.
func Read(store Store, now time.Time) (time.Time, bool) {
	data, err := store.ReadFile(markerKey)
	if err != nil {
		return time.Time{}, false
	}
	if len(data) != markerLen {
		return time.Time{}, false
	}

	ts := int64(binary.BigEndian.Uint64(data))
	expiry := time.Unix(ts, 0)

	if !now.Before(expiry) {
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

// SetFBit returns a copy of the capabilities with the Forwarding State bit
// (F=1) set on every <AFI, SAFI, Flags> tuple of every code-64 capability.
// Non-code-64 capabilities, and values too short to carry a tuple, are
// returned unchanged. The original Value slices are never modified.
//
// RFC 4724 Section 3: "The most significant bit is defined as the Forwarding
// State (F) bit, which can be used to indicate whether the forwarding state
// for routes that were advertised with the given AFI and SAFI has indeed been
// preserved during the previous BGP restart."
//
// The caller decides WHETHER the state was preserved, and it applies this to
// every tuple rather than to a chosen subset. Ze installs the routes of every
// family it carries through one forwarding plane, so that plane keeping its
// routes across the restart is one answer for all of them, and a per-family
// answer would be a distinction Ze cannot produce
// (Peer.getPluginCapabilities, internal/component/bgp/reactor/peer.go).
//
// RFC 4724 Section 4.1: "the 'Forwarding State' bit for an address family in
// the capability can be set only if the forwarding state has indeed been
// preserved for that address family during the restart." So this runs only
// inside the restart window, beside SetRBit, and never on a cold start.
func SetFBit(caps []plugin.InjectedCapability) []plugin.InjectedCapability {
	result := make([]plugin.InjectedCapability, len(caps))
	for i, ic := range caps {
		result[i] = ic
		if ic.Code != grCapCode || len(ic.Value) < grPrefixLen+grTupleLen {
			continue
		}
		// Copy the Value slice so the original is not modified.
		valueCopy := make([]byte, len(ic.Value))
		copy(valueCopy, ic.Value)
		for off := grPrefixLen; off+grTupleLen <= len(valueCopy); off += grTupleLen {
			valueCopy[off+grTupleFlagsOffset] |= fBitMask
		}
		result[i].Value = valueCopy
	}
	return result
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
