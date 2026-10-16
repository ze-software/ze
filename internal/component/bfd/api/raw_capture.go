// Design: docs/architecture/diagnostics/packet-capture.md -- raw capture activation and pcap export
// Related: registry.go -- the in-process Service publication this mirrors

package api

import (
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin"
)

// RawCapture is the BFD engine's raw byte capture, published by the
// bfd plugin for `capture-raw protocol bfd`. The diag command reads it here,
// so neither package imports the other.
type RawCapture interface {
	EnableRawCapture()
	DisableRawCapture()
	BFDRawCaptureSnapshot(limit int) []plugin.BGPRawCaptureEntry
}

// globalRawCapture holds the published provider. nil when the bfd plugin is
// not running, which the reader reports as "BFD capture not available".
var globalRawCapture atomic.Pointer[RawCapture]

// SetRawCaptureProvider publishes p. The bfd plugin calls it in OnStarted,
// beside SetService, and with nil in its shutdown path, so a late reader sees
// no provider rather than one whose engine is tearing down.
//
// Safe for concurrent use.
func SetRawCaptureProvider(p RawCapture) {
	if p == nil {
		globalRawCapture.Store(nil)
		return
	}
	globalRawCapture.Store(&p)
}

// RawCaptureProvider returns the published provider, or nil when the bfd
// plugin has not published one or has cleared it. Callers MUST handle nil.
//
// Safe for concurrent use.
func RawCaptureProvider() RawCapture {
	p := globalRawCapture.Load()
	if p == nil {
		return nil
	}
	return *p
}
