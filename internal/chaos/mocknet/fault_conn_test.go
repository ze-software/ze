package mocknet

import (
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	chaossim "github.com/ze-software/ze/internal/chaos"
)

// TestFaultConnDisabledPassThrough verifies zero-rate config returns the
// original connection untouched with zero wrapper overhead.
//
// VALIDATES: Disabled stream faults add zero overhead.
// PREVENTS: Wrapping connections when no stream fault rate is configured.
func TestFaultConnDisabledPassThrough(t *testing.T) {
	cpm := NewConnPairManager()
	a, b, err := cpm.NewPair()
	require.NoError(t, err)
	defer func() {
		_ = a.Close()
		_ = b.Close()
	}()

	got := NewFaultConn(a, StreamFaultConfig{Seed: 42}, 0)
	assert.Equal(t, a, got)
}

// TestFaultConnFragmentationPreservesByteStream verifies that 100% read and
// write fragmentation splits BGP-sized payloads into small chunks without
// losing, duplicating, or reordering a single byte.
//
// VALIDATES: Wire-level TCP fragmentation exercises partial-read reassembly
// while preserving exact stream semantics.
// PREVENTS: Fragmentation dropping trailing bytes or corrupting offsets.
func TestFaultConnFragmentationPreservesByteStream(t *testing.T) {
	cpm := NewConnPairManager()
	a, b, err := cpm.NewPair()
	require.NoError(t, err)
	defer func() {
		_ = a.Close()
		_ = b.Close()
	}()

	counter := &StreamFaultCounter{}
	writer := NewFaultConn(a, StreamFaultConfig{
		Seed:             42,
		FragmentRate:     1.0,
		MaxFragmentBytes: 7,
		Counter:          counter,
	}, 0)
	reader := NewFaultConn(b, StreamFaultConfig{
		Seed:             42,
		FragmentRate:     1.0,
		MaxFragmentBytes: 7,
		Counter:          counter,
	}, 1)

	payload := bytes.Repeat([]byte("BGP-OPEN-UPDATE-KEEPALIVE-"), 32)
	writeErrCh := make(chan error, 1)
	go func() {
		n, wErr := writer.Write(payload)
		if wErr == nil && n != len(payload) {
			wErr = io.ErrShortWrite
		}
		writeErrCh <- wErr
	}()

	var received bytes.Buffer
	buf := make([]byte, 128)
	readCalls := 0
	for received.Len() < len(payload) {
		n, rErr := reader.Read(buf)
		require.NoError(t, rErr)
		assert.LessOrEqual(t, n, 7, "fragmented read must respect MaxFragmentBytes")
		received.Write(buf[:n])
		readCalls++
	}

	require.NoError(t, <-writeErrCh)
	assert.Equal(t, payload, received.Bytes())
	assert.Greater(t, readCalls, 10)

	stats := counter.Snapshot()
	assert.Positive(t, stats.WriteFragments)
	assert.Positive(t, stats.ReadFragments)
}

// TestFaultConnBlackholeDrivenByVirtualClock verifies that asymmetric
// blackholes block outbound writes until the virtual clock advances past
// BlackholeDuration, and unblock immediately if closed early.
//
// VALIDATES: VirtualClock drives deterministic stream blackholes without real sleeps.
// PREVENTS: Blackhole stalls hanging on real time or leaking goroutines on Close.
func TestFaultConnBlackholeDrivenByVirtualClock(t *testing.T) {
	cpm := NewConnPairManager()
	a, b, err := cpm.NewPair()
	require.NoError(t, err)
	defer func() {
		_ = a.Close()
		_ = b.Close()
	}()

	vc := chaossim.NewVirtualClock(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))
	counter := &StreamFaultCounter{}
	writer := NewFaultConn(a, StreamFaultConfig{
		Seed:              7,
		BlackholeRate:     1.0,
		BlackholeDuration: 5 * time.Second,
		Clock:             vc,
		Counter:           counter,
	}, 0)

	payload := []byte("after-blackhole")
	done := make(chan error, 1)
	go func() {
		_, wErr := writer.Write(payload)
		done <- wErr
	}()

	// Wait until the blackhole counter increments (timer registered on vc).
	require.Eventually(t, func() bool {
		return counter.Snapshot().Blackholes == 1
	}, time.Second, 5*time.Millisecond)

	// Advancing less than BlackholeDuration keeps Write stalled.
	vc.Advance(3 * time.Second)
	select {
	case wErr := <-done:
		t.Fatalf("write completed early before blackhole duration: %v", wErr)
	case <-time.After(20 * time.Millisecond):
	}

	// Advancing past BlackholeDuration releases the write immediately.
	vc.Advance(3 * time.Second)
	require.NoError(t, <-done)

	got := make([]byte, len(payload))
	_, err = io.ReadFull(b, got)
	require.NoError(t, err)
	assert.Equal(t, payload, got)

	// Closing during an active blackhole unblocks immediately with net.ErrClosed.
	done2 := make(chan error, 1)
	go func() {
		_, wErr := writer.Write(payload)
		done2 <- wErr
	}()
	require.Eventually(t, func() bool {
		return counter.Snapshot().Blackholes == 2
	}, time.Second, 5*time.Millisecond)
	require.NoError(t, writer.Close())
	require.ErrorIs(t, <-done2, net.ErrClosed)
}

// TestFaultConnResetAbruptlyCloses verifies that ResetRate=1.0 returns
// ErrStreamReset, closes the connection, and increments the Reset counter.
//
// VALIDATES: Mid-frame resets tear down the stream cleanly.
// PREVENTS: Silent write failures or unclosed sockets on injected reset.
func TestFaultConnResetAbruptlyCloses(t *testing.T) {
	cpm := NewConnPairManager()
	a, b, err := cpm.NewPair()
	require.NoError(t, err)
	defer func() {
		_ = a.Close()
		_ = b.Close()
	}()

	counter := &StreamFaultCounter{}
	writer := NewFaultConn(a, StreamFaultConfig{
		Seed:      99,
		ResetRate: 1.0,
		Counter:   counter,
	}, 0)

	_, wErr := writer.Write([]byte("0123456789abcdef0123456789"))
	require.True(t, errors.Is(wErr, ErrStreamReset), "expected ErrStreamReset, got %v", wErr)
	assert.Equal(t, int64(1), counter.Snapshot().Resets)

	// Peer end must observe EOF (possibly after a truncated prefix).
	data, rErr := io.ReadAll(b)
	require.NoError(t, rErr)
	assert.Less(t, len(data), 26, "reset must truncate or drop the frame")
}
