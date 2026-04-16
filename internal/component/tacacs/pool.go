// Design: .claude/rules/design-principles.md -- "No make where pools exist"
// Overview: client.go -- TACACS+ TCP client and wire protocol
// Related: packet.go -- packet header and body encoding (pool consumer)

package tacacs

import "sync"

// A TACACS+ packet is at most 12 (header) + 65535 (body, uint16 ceiling in
// RFC 8907 §4.1) = 65547 bytes. Every wire read and write path uses
// buffers of this size. A `sync.Pool` is the right structure here because
// the TACACS+ client is consumed by three concurrent goroutines (SSH
// auth callback, dispatcher authorization, accounting worker) and
// sync.Pool's per-P local cache removes Get/Put contention that a
// single-channel pool would introduce.
//
// Sizing rationale: TACACS+ runs at human pace -- one exchange per SSH
// login plus one per dispatched CLI command. Peak concurrency on a
// busy router is bounded by the SSH max-sessions cap plus the single
// accountant worker. Seeding with poolBufs = 16 covers realistic peak
// without over-committing memory (16 * 65 547 B ≈ 1 MB). Under that load
// the pool's New func is practically never invoked, so the runtime
// `make` inside New is not a regular failure point; it is the last-resort
// fallback if bursts exceed the seed AND the GC has flushed the pool's
// victim cache in the same window.
const (
	poolBufSize = hdrLen + maxBodyLen // 12 + 65535 = 65547
	poolBufs    = 16
)

// bufPool wraps a sync.Pool pre-seeded with poolBufs full-size buffers.
// Get returns a []byte of exactly poolBufSize; Put re-slices to full cap
// so callers cannot return a shortened sub-slice. Buffers in the pool
// may be garbage-collected between GC cycles (sync.Pool semantics); the
// New func re-creates them on demand.
type bufPool struct {
	p sync.Pool
}

// newBufPool constructs a sync.Pool-backed buffer pool and primes it
// with `n` buffers so the first `n` Gets hit the cache and do NOT invoke
// New. `n` should be sized for peak concurrent wire activity.
func newBufPool(n, size int) *bufPool {
	bp := &bufPool{
		p: sync.Pool{
			New: func() any {
				// Return a pointer to a slice so sync.Pool avoids the
				// interface-escape allocation on every Put.
				b := make([]byte, size)
				return &b
			},
		},
	}
	for range n {
		b := make([]byte, size)
		bp.p.Put(&b)
	}
	return bp
}

// Get returns a full-capacity buffer from the pool. Callers slice it to
// the needed length and MUST Put the original full-cap slice back.
func (p *bufPool) Get() []byte {
	ptr, ok := p.p.Get().(*[]byte)
	if !ok {
		// Unreachable: every value Put into p.p is *[]byte, and New
		// returns the same shape. Fall back to a fresh allocation so a
		// hypothetical programmer error cannot crash the client.
		b := make([]byte, poolBufSize)
		return b
	}
	return *ptr
}

// Put returns a buffer to the pool. Re-slices to full capacity so the
// next Get caller sees `len == cap`. Silently drops buffers of the
// wrong capacity (caller bug, not worth panicking on in production).
func (p *bufPool) Put(b []byte) {
	if cap(b) != poolBufSize {
		return
	}
	b = b[:cap(b)]
	p.p.Put(&b)
}
