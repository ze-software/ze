// Design: rfc/short/rfc5880.md -- in-memory transport for engine tests
// Related: socket.go -- Transport interface
//
// Loopback is an in-process Transport pair. Two Loopback transports linked
// via Pair() exchange Outbound -> Inbound through a goroutine-safe channel
// without touching the kernel network stack. The express-loop engine tests
// use this to drive two real Machines through a real codec without
// permission to bind UDP ports.
package transport

import (
	"errors"
	"net/netip"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/api"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bfd/packet"
)

// Loopback is an in-memory Transport. It is paired with a sibling
// Loopback via Pair; Send on either side enqueues a packet on the other
// side's RX channel. Use only in tests; production code uses the UDP
// transports.
type Loopback struct {
	mu      sync.Mutex
	peer    *Loopback
	rx      chan Inbound
	stopped bool
	mode    api.HopMode
	// localAddr is the address this half represents on the wire. When
	// the paired half delivers a packet, it stamps Inbound.From with
	// the *sender's* localAddr so the engine can associate first
	// packets by source address exactly as a real UDP socket does.
	localAddr netip.Addr
}

// ErrLoopbackUnpaired is returned by Send when the peer half is missing.
var ErrLoopbackUnpaired = errors.New("bfd: loopback transport not paired")

// ErrLoopbackStopped is returned by Send and Start after Stop.
var ErrLoopbackStopped = errors.New("bfd: loopback transport stopped")

// ErrLoopbackOverflow is returned by Send when the peer's RX channel is
// full. Mirrors the UDP socket overflow behavior of dropping rather than
// blocking, but surfaces the drop to the caller so a test can fail loudly
// instead of silently dropping packets.
var ErrLoopbackOverflow = errors.New("bfd: loopback peer rx channel full")

// Pair creates two linked Loopback halves. mode is recorded on every
// Inbound for symmetry with the production transports. addrA and addrB
// are the addresses each half represents on the wire; they appear as
// Inbound.From on packets delivered to the peer.
func Pair(mode api.HopMode, addrA, addrB netip.Addr) (*Loopback, *Loopback) {
	a := &Loopback{rx: make(chan Inbound, 32), mode: mode, localAddr: addrA}
	b := &Loopback{rx: make(chan Inbound, 32), mode: mode, localAddr: addrB}
	a.peer = b
	b.peer = a
	return a, b
}

// Start is a no-op for Loopback; the channel is created in Pair.
func (l *Loopback) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return ErrLoopbackStopped
	}
	return nil
}

// Stop closes the local RX channel and detaches the peer link. Subsequent
// Send calls fail. Stop is idempotent.
func (l *Loopback) Stop() error {
	l.mu.Lock()
	if l.stopped {
		l.mu.Unlock()
		return nil
	}
	l.stopped = true
	close(l.rx)
	peer := l.peer
	l.peer = nil
	l.mu.Unlock()
	if peer != nil {
		peer.mu.Lock()
		peer.peer = nil
		peer.mu.Unlock()
	}
	return nil
}

// Send enqueues an Outbound onto the peer's RX channel. The buffer is
// copied so the caller can recycle Outbound.Bytes immediately.
//
// Send returns ErrLoopbackOverflow if the peer's RX channel is full.
// The engine treats this as equivalent to a dropped UDP packet.
func (l *Loopback) Send(out Outbound) error {
	l.mu.Lock()
	peer := l.peer
	stopped := l.stopped
	l.mu.Unlock()
	if stopped {
		return ErrLoopbackStopped
	}
	if peer == nil {
		return ErrLoopbackUnpaired
	}

	// Copy bytes into a packet pool buffer so the sender can recycle
	// its own buffer immediately. The receiver returns the pool buffer
	// via Inbound.Release.
	if len(out.Bytes) > packet.PoolBufSize {
		return ErrLoopbackOverflow
	}
	pb := packet.Acquire()
	n := copy(pb.Data(), out.Bytes)
	bytes := pb.Data()[:n]
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		packet.Release(pb)
	}

	in := Inbound{
		From:      l.localAddr, // the sender's local address becomes the receiver's source
		VRF:       out.VRF,
		Interface: out.Interface,
		Mode:      l.mode,
		TTL:       255, // loopback never decrements TTL
		Bytes:     bytes,
		release:   release,
	}

	peer.mu.Lock()
	defer peer.mu.Unlock()
	if peer.stopped {
		release()
		return ErrLoopbackStopped
	}
	if len(peer.rx) >= cap(peer.rx) {
		release()
		return ErrLoopbackOverflow
	}
	peer.rx <- in
	return nil
}

// RX returns the receive channel.
func (l *Loopback) RX() <-chan Inbound { return l.rx }
