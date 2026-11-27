// Design: docs/architecture/diagnostics/production-diagnostics.md -- always-on Go flight recorder with bounded snapshot ring

// Package flightrec wraps Go's runtime/trace.FlightRecorder with a bounded
// in-memory snapshot ring and cooldown-throttled automatic capture.
package flightrec

import (
	"bytes"
	"errors"
	"runtime"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultMinAge       = 10 * time.Second
	defaultMaxBytes     = 4 << 20
	maxSnapshots        = 4
	maxSnapshotBytes    = 4 << 20
	defaultAutoCooldown = 30 * time.Second
)

// ErrNotEnabled is returned when a snapshot is requested while the flight
// recorder is stopped.
var ErrNotEnabled = errors.New("flight recorder is not enabled")

// StatusInfo reports the current flight-recorder state and ring configuration.
type StatusInfo struct {
	Enabled           bool
	MinAge            time.Duration
	MaxBytes          uint64
	SnapshotCount     int
	MaxSnapshots      int
	AutoCaptureErrors uint64
}

// SnapshotMeta describes one captured flight-recorder trace snapshot without
// carrying its raw trace bytes.
type SnapshotMeta struct {
	ID         string
	CapturedAt time.Time
	Trigger    string
	Detail     string
	Bytes      int
}

// Snapshot pairs immutable snapshot metadata with the captured Go execution
// trace bytes.
type Snapshot struct {
	Meta SnapshotMeta
	Data []byte
}

// recorder holds the process-global flight recorder and bounded snapshot ring.
// Safe for concurrent use.
type recorder struct {
	mu                sync.Mutex
	fr                *trace.FlightRecorder
	snapshots         []Snapshot
	seq               uint64
	lastAuto          time.Time
	cooldown          time.Duration
	autoCaptureErrors uint64
}

// limitedBuffer captures at most maxBytes into buf and discards excess bytes so
// trace.FlightRecorder.WriteTo finishes cleanly within a fixed memory budget.
// NOT safe for concurrent use.
type limitedBuffer struct {
	buf      bytes.Buffer
	maxBytes int
	dropped  int
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	remaining := b.maxBytes - b.buf.Len()
	if remaining <= 0 {
		b.dropped += len(p)
		return len(p), nil
	}
	if len(p) <= remaining {
		return b.buf.Write(p)
	}
	_, err := b.buf.Write(p[:remaining])
	if err != nil {
		return 0, err
	}
	b.dropped += len(p) - remaining
	return len(p), nil
}

var (
	global           = recorder{cooldown: defaultAutoCooldown}
	blockProfileRate atomic.Int64
)

// SetBlockProfileRate configures Go's runtime block profile rate and remembers
// the active rate so show system profile can report sampling-rate metadata for
// block profiles (runtime.SetBlockProfileRate returns no previous value).
//
// Safe for concurrent use.
func SetBlockProfileRate(rate int) {
	if rate < 0 {
		rate = 0
	}
	runtime.SetBlockProfileRate(rate)
	blockProfileRate.Store(int64(rate))
}

// BlockProfileRate returns the block profile sampling rate last configured
// through SetBlockProfileRate (0 when disabled or not configured).
//
// Safe for concurrent use.
func BlockProfileRate() int {
	return int(blockProfileRate.Load())
}

// Start activates the process-global flight recorder. Repeated calls while the
// recorder is active are idempotent. Callers MUST pair a successful Start with
// Stop.
//
// Safe for concurrent use.
func Start() error {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.fr != nil {
		if global.fr.Enabled() {
			return nil
		}
	}
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{
		MinAge:   defaultMinAge,
		MaxBytes: defaultMaxBytes,
	})
	if err := fr.Start(); err != nil {
		global.fr = nil
		return err
	}
	global.fr = fr
	return nil
}

// Stop deactivates the process-global flight recorder. Callers MUST pair every
// successful Start with Stop.
//
// Safe for concurrent use.
func Stop() {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.fr == nil {
		return
	}
	global.fr.Stop()
	global.fr = nil
}

// Status returns the current flight-recorder status and snapshot count.
//
// Safe for concurrent use.
func Status() StatusInfo {
	global.mu.Lock()
	defer global.mu.Unlock()

	enabled := false
	if global.fr != nil {
		enabled = global.fr.Enabled()
	}
	return StatusInfo{
		Enabled:           enabled,
		MinAge:            defaultMinAge,
		MaxBytes:          defaultMaxBytes,
		SnapshotCount:     len(global.snapshots),
		MaxSnapshots:      maxSnapshots,
		AutoCaptureErrors: global.autoCaptureErrors,
	}
}

// Snapshots returns retained snapshot metadata ordered newest-first.
//
// Safe for concurrent use.
func Snapshots() []SnapshotMeta {
	global.mu.Lock()
	defer global.mu.Unlock()

	out := make([]SnapshotMeta, len(global.snapshots))
	for i := range global.snapshots {
		out[i] = global.snapshots[len(global.snapshots)-1-i].Meta
	}
	return out
}

// SnapshotByID looks up one retained snapshot by its identifier and returns an
// isolated copy of its trace bytes.
//
// Safe for concurrent use.
func SnapshotByID(id string) (Snapshot, bool) {
	global.mu.Lock()
	defer global.mu.Unlock()

	for i := range global.snapshots {
		if global.snapshots[i].Meta.ID == id {
			return Snapshot{
				Meta: global.snapshots[i].Meta,
				Data: bytes.Clone(global.snapshots[i].Data),
			}, true
		}
	}
	return Snapshot{}, false
}

// CaptureAuto captures a snapshot when the recorder is enabled and outside the
// automatic capture cooldown window.
//
// Safe for concurrent use.
func CaptureAuto(trigger, detail string) bool {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.fr == nil {
		return false
	}
	if !global.fr.Enabled() {
		return false
	}
	now := time.Now()
	if !global.lastAuto.IsZero() {
		if now.Sub(global.lastAuto) < global.cooldown {
			return false
		}
	}
	global.lastAuto = now
	if _, err := captureLocked(trigger, detail, now); err != nil {
		global.autoCaptureErrors++
		return false
	}
	return true
}

// CaptureNow captures a snapshot immediately, bypassing the automatic cooldown.
//
// Safe for concurrent use.
func CaptureNow(trigger, detail string) (SnapshotMeta, error) {
	global.mu.Lock()
	defer global.mu.Unlock()

	return captureLocked(trigger, detail, time.Now())
}

// ResetForTest stops the recorder and clears all retained snapshots and
// cooldown state between tests.
//
// Safe for concurrent use.
func ResetForTest() {
	global.mu.Lock()
	defer global.mu.Unlock()

	if global.fr != nil {
		global.fr.Stop()
		global.fr = nil
	}
	global.snapshots = nil
	global.seq = 0
	global.lastAuto = time.Time{}
	global.cooldown = defaultAutoCooldown
	global.autoCaptureErrors = 0
}

// formatSnapshotID formats a monotonic trace snapshot identifier ("trace-<seq>")
// on the stack using stdlib only so flightrec stays a pure leaf package.
func formatSnapshotID(seq uint64) string {
	var buf [32]byte
	n := copy(buf[:], "trace-")
	var digits [20]byte
	pos := len(digits)
	for {
		pos--
		digits[pos] = byte('0' + seq%10)
		seq /= 10
		if seq == 0 {
			break
		}
	}
	n += copy(buf[n:], digits[pos:])
	return string(buf[:n])
}

// captureLocked writes the flight recorder's current window into a capped
// buffer and appends the resulting snapshot to the circular ring. Callers MUST
// hold global.mu.
func captureLocked(trigger, detail string, now time.Time) (SnapshotMeta, error) {
	if global.fr == nil {
		return SnapshotMeta{}, ErrNotEnabled
	}
	if !global.fr.Enabled() {
		return SnapshotMeta{}, ErrNotEnabled
	}
	var buf limitedBuffer
	buf.maxBytes = maxSnapshotBytes
	if _, err := global.fr.WriteTo(&buf); err != nil {
		return SnapshotMeta{}, err
	}
	data := bytes.Clone(buf.buf.Bytes())
	global.seq++
	meta := SnapshotMeta{
		ID:         formatSnapshotID(global.seq),
		CapturedAt: now.UTC(),
		Trigger:    trigger,
		Detail:     detail,
		Bytes:      len(data),
	}
	snap := Snapshot{Meta: meta, Data: data}
	if len(global.snapshots) >= maxSnapshots {
		copy(global.snapshots, global.snapshots[1:])
		global.snapshots[len(global.snapshots)-1] = snap
	} else {
		global.snapshots = append(global.snapshots, snap)
	}
	return meta, nil
}
