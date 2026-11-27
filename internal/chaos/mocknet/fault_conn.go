// Design: docs/architecture/chaos-web-dashboard.md -- seeded stream fault injection for in-process simulation

package mocknet

import (
	"errors"
	"math/rand/v2"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/clock"
)

const (
	// DefaultMaxFragmentBytes matches the 19-byte BGP message header so
	// fragmented reads and writes regularly split headers and payloads mid-frame.
	DefaultMaxFragmentBytes = 19

	// DefaultBlackholeDuration is the default virtual-clock pause injected by an
	// asymmetric stream blackhole before traffic resumes or the session times out.
	DefaultBlackholeDuration = 3 * time.Second
)

// ErrStreamReset is returned when FaultConn injects a mid-stream connection reset.
var ErrStreamReset = errors.New("mocknet: injected stream reset")

// StreamFaultConfig configures deterministic wire-level fault injection on
// in-process connections (inspired by TigerBeetle VOPR's packet simulator).
// Zero value disables all stream faults.
type StreamFaultConfig struct {
	// Seed drives deterministic fault selection across connections.
	Seed uint64

	// FragmentRate is the per-Read / per-Write probability (0.0-1.0) of
	// splitting I/O into small chunks up to MaxFragmentBytes.
	FragmentRate float64

	// MaxFragmentBytes bounds chunk sizes during fragmentation.
	// Defaults to DefaultMaxFragmentBytes (19) when <= 0.
	MaxFragmentBytes int

	// BlackholeRate is the per-Write probability (0.0-1.0) of stalling outbound
	// traffic for BlackholeDuration to simulate asymmetric half-open blackholes.
	BlackholeRate float64

	// BlackholeDuration is the duration of each blackhole stall.
	// Uses Clock when set, otherwise real time. Defaults to DefaultBlackholeDuration.
	BlackholeDuration time.Duration

	// ResetRate is the per-Write probability (0.0-1.0) of writing a truncated
	// prefix and abruptly closing the connection with ErrStreamReset.
	ResetRate float64

	// Clock supplies virtual time for blackhole stalls when non-nil.
	Clock clock.Clock

	// Counter, when non-nil, aggregates fault counts across all connections in a run.
	Counter *StreamFaultCounter

	// Pause, when non-nil and true, suppresses new write-side fault injection
	// (blackholes, resets, write fragmentation) so teardown/settle can converge.
	Pause *atomic.Bool
}

// Enabled reports whether any stream fault probability is positive.
func (c StreamFaultConfig) Enabled() bool {
	return c.FragmentRate > 0 || c.BlackholeRate > 0 || c.ResetRate > 0
}

func (c StreamFaultConfig) normalized() StreamFaultConfig {
	c.FragmentRate = clampUnit(c.FragmentRate)
	c.BlackholeRate = clampUnit(c.BlackholeRate)
	c.ResetRate = clampUnit(c.ResetRate)
	if c.MaxFragmentBytes <= 0 {
		c.MaxFragmentBytes = DefaultMaxFragmentBytes
	}
	if c.BlackholeDuration <= 0 {
		c.BlackholeDuration = DefaultBlackholeDuration
	}
	return c
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// StreamFaultStats is a point-in-time snapshot of injected stream faults.
type StreamFaultStats struct {
	ReadFragments  int64
	WriteFragments int64
	Blackholes     int64
	Resets         int64
}

// Total returns the sum of all injected stream fault events.
func (s StreamFaultStats) Total() int64 {
	return s.ReadFragments + s.WriteFragments + s.Blackholes + s.Resets
}

// StreamFaultCounter tracks injected stream faults safely across goroutines.
type StreamFaultCounter struct {
	readFragments  atomic.Int64
	writeFragments atomic.Int64
	blackholes     atomic.Int64
	resets         atomic.Int64
}

// Snapshot returns a consistent copy of the accumulated fault counts.
func (c *StreamFaultCounter) Snapshot() StreamFaultStats {
	if c == nil {
		return StreamFaultStats{}
	}
	return StreamFaultStats{
		ReadFragments:  c.readFragments.Load(),
		WriteFragments: c.writeFragments.Load(),
		Blackholes:     c.blackholes.Load(),
		Resets:         c.resets.Load(),
	}
}

// FaultConn wraps a net.Conn with deterministic fragmentation, asymmetric
// blackholes, and mid-frame resets. Safe for concurrent Read, Write, and Close.
type FaultConn struct {
	net.Conn
	cfg       StreamFaultConfig
	counter   *StreamFaultCounter
	readMu    sync.Mutex
	readRNG   *rand.Rand
	writeMu   sync.Mutex
	writeRNG  *rand.Rand
	closeOnce sync.Once
	closed    chan struct{}
}

// NewFaultConn wraps conn with deterministic stream fault injection.
// streamID differentiates per-connection PRNG streams under the same run seed.
// When !cfg.Enabled(), conn is returned directly with zero overhead.
func NewFaultConn(conn net.Conn, cfg StreamFaultConfig, streamID uint64) net.Conn {
	cfg = cfg.normalized()
	if !cfg.Enabled() {
		return conn
	}
	base := cfg.Seed ^ ((streamID + 1) * 0x9e3779b97f4a7c15)
	counter := cfg.Counter
	if counter == nil {
		counter = &StreamFaultCounter{}
	}
	return &FaultConn{
		Conn:     conn,
		cfg:      cfg,
		counter:  counter,
		readRNG:  rand.New(rand.NewPCG(base, base^0x517cc1b727220a95)),     //nolint:gosec // Deterministic simulation PRNG.
		writeRNG: rand.New(rand.NewPCG(base^0x1, base^0x2545f4914f6cdd1d)), //nolint:gosec // Deterministic simulation PRNG.
		closed:   make(chan struct{}),
	}
}

// Stats returns the accumulated stream fault counts for this connection's counter.
func (f *FaultConn) Stats() StreamFaultStats {
	return f.counter.Snapshot()
}

// Read reads from the underlying connection, optionally clamping the read
// buffer to a small random fragment to exercise partial-frame reassembly.
func (f *FaultConn) Read(p []byte) (int, error) {
	if len(p) > 1 && f.cfg.FragmentRate > 0 {
		limit := f.nextReadChunk(len(p))
		if limit > 0 && limit < len(p) {
			f.counter.readFragments.Add(1)
			return f.Conn.Read(p[:limit])
		}
	}
	return f.Conn.Read(p)
}

func (f *FaultConn) nextReadChunk(bufLen int) int {
	f.readMu.Lock()
	defer f.readMu.Unlock()
	if f.readRNG.Float64() >= f.cfg.FragmentRate {
		return 0
	}
	upper := min(bufLen-1, f.cfg.MaxFragmentBytes)
	if upper < 1 {
		return 0
	}
	return 1 + f.readRNG.IntN(upper)
}

// Write writes p to the underlying connection, optionally injecting a
// mid-stream reset, an asymmetric blackhole stall, or fragmented writes.
func (f *FaultConn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return f.Conn.Write(p)
	}

	reset, prefixLen, blackhole, fragment := f.decideWriteFault(len(p))

	if reset {
		f.counter.resets.Add(1)
		written := 0
		if prefixLen > 0 {
			written, _ = f.Conn.Write(p[:prefixLen])
		}
		_ = f.Close()
		return written, ErrStreamReset
	}

	if blackhole {
		f.counter.blackholes.Add(1)
		if !f.waitBlackhole() {
			return 0, net.ErrClosed
		}
	}

	if fragment && len(p) > 1 {
		f.counter.writeFragments.Add(1)
		return f.writeFragmented(p)
	}

	return f.Conn.Write(p)
}

func (f *FaultConn) decideWriteFault(dataLen int) (reset bool, prefixLen int, blackhole, fragment bool) {
	if f.cfg.Pause != nil && f.cfg.Pause.Load() {
		return false, 0, false, false
	}

	f.writeMu.Lock()
	defer f.writeMu.Unlock()

	if f.cfg.ResetRate > 0 && f.writeRNG.Float64() < f.cfg.ResetRate {
		prefix := 0
		if dataLen > 1 {
			prefix = f.writeRNG.IntN(dataLen)
		}
		return true, prefix, false, false
	}
	if f.cfg.BlackholeRate > 0 && f.writeRNG.Float64() < f.cfg.BlackholeRate {
		blackhole = true
	}
	if f.cfg.FragmentRate > 0 && f.writeRNG.Float64() < f.cfg.FragmentRate {
		fragment = true
	}
	return false, 0, blackhole, fragment
}

func (f *FaultConn) writeFragmented(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		remaining := len(p) - total
		chunk := remaining
		if remaining > 1 {
			upper := min(remaining, f.cfg.MaxFragmentBytes)
			f.writeMu.Lock()
			chunk = 1 + f.writeRNG.IntN(upper)
			f.writeMu.Unlock()
		}
		n, err := f.Conn.Write(p[total : total+chunk])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// waitBlackhole blocks for cfg.BlackholeDuration (using virtual Clock when set)
// or returns false immediately if the connection is closed first.
func (f *FaultConn) waitBlackhole() bool {
	if f.cfg.Clock != nil {
		timer := f.cfg.Clock.NewTimer(f.cfg.BlackholeDuration)
		defer timer.Stop()
		select {
		case <-timer.C():
			return true
		case <-f.closed:
			return false
		}
	}
	timer := time.NewTimer(f.cfg.BlackholeDuration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-f.closed:
		return false
	}
}

// Close closes the underlying connection and unblocks any stalled blackhole.
func (f *FaultConn) Close() error {
	var err error
	f.closeOnce.Do(func() {
		close(f.closed)
		err = f.Conn.Close()
	})
	return err
}
