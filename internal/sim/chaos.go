// Design: docs/architecture/chaos-web-dashboard.md — simulation infrastructure

package sim

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"sync"
	"time"
)

// ResolveSeed resolves special seed values.
// Seed -1 generates a time-based seed for non-reproducible chaos runs;
// the resolved seed is returned so callers can log it for reproduction.
// All other values pass through unchanged.
func ResolveSeed(seed int64) int64 {
	if seed == -1 {
		return time.Now().UnixNano()
	}
	return seed
}

// ChaosConfig holds configuration for chaos fault injection.
type ChaosConfig struct {
	// Seed for the PRNG. Seed=0 means disabled (no chaos).
	Seed int64
	// Rate is the probability of fault per operation (0.0-1.0).
	// Values <0 are clamped to 0, >1 are clamped to 1.
	Rate float64
	// Logger for fault injection events. If nil, faults are silent.
	Logger *slog.Logger
}

// chaosRNG is a mutex-protected PRNG for deterministic, thread-safe fault decisions.
type chaosRNG struct {
	mu     sync.Mutex
	rng    *rand.Rand
	rate   float64
	logger *slog.Logger
	// disabled means seed=0 — all calls pass through regardless of rate
	disabled bool
}

func newChaosRNG(cfg ChaosConfig) *chaosRNG {
	rate := cfg.Rate
	if rate < 0 {
		rate = 0
	}
	if rate > 1 {
		rate = 1
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &chaosRNG{
		rng:      rand.New(rand.NewSource(cfg.Seed)), //nolint:gosec // Deterministic PRNG for chaos, not security
		rate:     rate,
		logger:   logger,
		disabled: cfg.Seed == 0,
	}
}

// shouldFault returns true if a fault should be injected this call.
func (r *chaosRNG) shouldFault() bool {
	if r.disabled || r.rate == 0 {
		return false
	}
	r.mu.Lock()
	v := r.rng.Float64()
	r.mu.Unlock()
	return v < r.rate
}

// float64 returns a random float64 in [0, 1) from the PRNG.
func (r *chaosRNG) float64() float64 {
	r.mu.Lock()
	v := r.rng.Float64()
	r.mu.Unlock()
	return v
}

// intn returns a random int in [0, n) from the PRNG.
func (r *chaosRNG) intn(n int) int {
	r.mu.Lock()
	v := r.rng.Intn(n)
	r.mu.Unlock()
	return v
}

// =============================================================================
// ChaosClock
// =============================================================================

// ChaosClock wraps a Clock with seed-driven fault injection.
// Fault types: Now() jitter (±50ms), timer duration jitter (0.8-1.2x),
// Sleep extension (1.0-2.0x).
type ChaosClock struct {
	inner Clock
	rng   *chaosRNG
}

// NewChaosClock creates a ChaosClock wrapping the given inner clock.
// If cfg.Seed is 0, all calls pass through without modification.
func NewChaosClock(inner Clock, cfg ChaosConfig) *ChaosClock {
	return &ChaosClock{
		inner: inner,
		rng:   newChaosRNG(cfg),
	}
}

// effectiveRate returns the clamped rate (for testing boundary behavior).
func (c *ChaosClock) effectiveRate() float64 {
	return c.rng.rate
}

// jitteredDuration returns duration with 0.8-1.2x jitter applied.
// Uses the PRNG to determine the jitter multiplier.
func (c *ChaosClock) jitteredDuration(d time.Duration) time.Duration {
	if !c.rng.shouldFault() {
		return d
	}
	// Jitter multiplier: 0.8 + random * 0.4 → range [0.8, 1.2)
	multiplier := 0.8 + c.rng.float64()*0.4
	jittered := time.Duration(float64(d) * multiplier)
	c.rng.logger.Debug("chaos: clock jitter",
		"original", d,
		"jittered", jittered,
		"multiplier", fmt.Sprintf("%.3f", multiplier),
	)
	return jittered
}

// Now returns the current time, possibly with ±50ms offset.
func (c *ChaosClock) Now() time.Time {
	t := c.inner.Now()
	if !c.rng.shouldFault() {
		return t
	}
	// Offset: -50ms to +50ms
	offsetMs := c.rng.intn(101) - 50
	offset := time.Duration(offsetMs) * time.Millisecond
	c.rng.logger.Debug("chaos: clock now jitter",
		"offset", offset,
	)
	return t.Add(offset)
}

// Sleep pauses for the given duration, possibly extended by 1.0-2.0x.
func (c *ChaosClock) Sleep(d time.Duration) {
	if c.rng.shouldFault() {
		multiplier := 1.0 + c.rng.float64()
		extended := time.Duration(float64(d) * multiplier)
		c.rng.logger.Debug("chaos: sleep extension",
			"original", d,
			"extended", extended,
		)
		c.inner.Sleep(extended)
		return
	}
	c.inner.Sleep(d)
}

// After waits for duration d (possibly jittered) and sends time on channel.
func (c *ChaosClock) After(d time.Duration) <-chan time.Time {
	return c.inner.After(c.jitteredDuration(d))
}

// AfterFunc waits for duration d (possibly jittered) and calls f.
func (c *ChaosClock) AfterFunc(d time.Duration, f func()) Timer {
	return c.inner.AfterFunc(c.jitteredDuration(d), f)
}

// NewTimer creates a timer with duration d (possibly jittered).
func (c *ChaosClock) NewTimer(d time.Duration) Timer {
	return c.inner.NewTimer(c.jitteredDuration(d))
}

// =============================================================================
// ChaosDialer
// =============================================================================

// ChaosDialer wraps a Dialer with seed-driven fault injection.
// Fault types: connection refused, slow connect, connection reset after N bytes.
type ChaosDialer struct {
	inner Dialer
	rng   *chaosRNG
}

// NewChaosDialer creates a ChaosDialer wrapping the given inner dialer.
func NewChaosDialer(inner Dialer, cfg ChaosConfig) *ChaosDialer {
	return &ChaosDialer{
		inner: inner,
		rng:   newChaosRNG(cfg),
	}
}

// shouldFault exposes the fault decision for testing determinism.
func (d *ChaosDialer) shouldFault() bool {
	return d.rng.shouldFault()
}

// DialContext connects to the address, possibly injecting a fault.
// Fault selection (when faulting):
//   - 50% connection refused (immediate error)
//   - 25% slow connect (1-5s delay then proceed)
//   - 25% connection reset (connect succeeds, conn closes after 0-100 bytes written)
func (d *ChaosDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if !d.rng.shouldFault() {
		return d.inner.DialContext(ctx, network, address)
	}

	// Select fault type: 0-3
	// 0,1 = refused (50%), 2 = slow (25%), 3 = reset (25%)
	faultType := d.rng.intn(4)

	if faultType < 2 {
		// Connection refused (50%)
		d.rng.logger.Debug("chaos: dial refused",
			"network", network,
			"address", address,
		)
		return nil, fmt.Errorf("chaos: connection refused to %s", address)
	}

	if faultType == 2 {
		// Slow connect (25%): delay 1-5s then proceed
		delay := time.Duration(1+d.rng.intn(5)) * time.Second
		d.rng.logger.Debug("chaos: dial slow connect",
			"network", network,
			"address", address,
			"delay", delay,
		)
		select {
		case <-time.After(delay):
			return d.inner.DialContext(ctx, network, address)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Connection reset (25%): connect, then close after 0-100 bytes
	conn, err := d.inner.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	threshold := d.rng.intn(101)
	d.rng.logger.Debug("chaos: dial connection reset",
		"network", network,
		"address", address,
		"threshold", threshold,
	)
	return &chaosConn{Conn: conn, threshold: threshold}, nil
}

// chaosConn wraps a net.Conn and closes after a byte threshold is reached.
type chaosConn struct {
	net.Conn
	mu        sync.Mutex
	written   int
	threshold int
	closed    bool
}

// Write writes to the connection, closing it after the byte threshold.
func (c *chaosConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return 0, net.ErrClosed
	}

	c.written += len(p)
	if c.written >= c.threshold {
		c.closed = true
		_ = c.Conn.Close()
		return 0, fmt.Errorf("chaos: connection reset after %d bytes", c.written)
	}

	return c.Conn.Write(p)
}

// Read reads from the connection.
func (c *chaosConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	c.mu.Unlock()
	return c.Conn.Read(p)
}

// Close closes the connection.
func (c *chaosConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return c.Conn.Close()
}

// =============================================================================
// ChaosListenerFactory
// =============================================================================

// ChaosListenerFactory wraps a ListenerFactory with seed-driven fault injection.
// Fault types: bind failure, accept delay.
type ChaosListenerFactory struct {
	inner ListenerFactory
	rng   *chaosRNG
}

// NewChaosListenerFactory creates a ChaosListenerFactory wrapping the given factory.
func NewChaosListenerFactory(inner ListenerFactory, cfg ChaosConfig) *ChaosListenerFactory {
	return &ChaosListenerFactory{
		inner: inner,
		rng:   newChaosRNG(cfg),
	}
}

// Listen creates a listener, possibly injecting a bind failure.
// When not faulting on Listen, may still return a chaosListener that delays Accept.
func (f *ChaosListenerFactory) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	if f.rng.shouldFault() {
		// Bind failure
		f.rng.logger.Debug("chaos: listen bind failure",
			"network", network,
			"address", address,
		)
		return nil, fmt.Errorf("chaos: bind failure on %s %s", network, address)
	}

	ln, err := f.inner.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}

	// Wrap with chaosListener that may delay accepts
	return &chaosListener{Listener: ln, rng: f.rng}, nil
}

// chaosListener wraps a net.Listener with possible accept delays.
type chaosListener struct {
	net.Listener
	rng *chaosRNG
}

// Accept waits for and returns the next connection, possibly with delay.
func (l *chaosListener) Accept() (net.Conn, error) {
	if l.rng.shouldFault() {
		// Accept delay: 1-3s
		delay := time.Duration(1+l.rng.intn(3)) * time.Second
		l.rng.logger.Debug("chaos: accept delay",
			"delay", delay,
		)
		time.Sleep(delay)
	}
	return l.Listener.Accept()
}

// =============================================================================
// Convenience constructors
// =============================================================================

// NewChaosWrappers creates all three chaos wrappers from a single config.
// Returns the wrapped Clock, Dialer, and ListenerFactory.
// If cfg.Seed is 0, all wrappers are pure passthrough.
func NewChaosWrappers(clock Clock, dialer Dialer, lf ListenerFactory, cfg ChaosConfig) (Clock, Dialer, ListenerFactory) {
	return NewChaosClock(clock, cfg),
		NewChaosDialer(dialer, cfg),
		NewChaosListenerFactory(lf, cfg)
}
