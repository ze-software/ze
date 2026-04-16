// Design: .claude/rules/design-principles.md -- "Pool strategy by goroutine shape"
// Overview: doc.go -- package overview and shape decision criteria

package bufpool

import "sync"

// Pool is a fixed-size byte-slice pool backed by sync.Pool. All buffers
// are exactly `size` bytes; Get always returns a full-capacity slice.
// Safe for concurrent use.
type Pool struct {
	size int
	name string
	p    sync.Pool
}

// New constructs a Pool pre-seeded with peakN buffers of size bytes.
// name is a debug label (future: exported to metrics).
//
// Callers MUST call Put when done with a buffer returned by Get. The
// pool uses *[]byte internally to avoid the interface-escape allocation
// on every Put.
func New(peakN, size int, name string) *Pool {
	bp := &Pool{
		size: size,
		name: name,
		p: sync.Pool{
			New: func() any {
				b := make([]byte, size)
				return &b
			},
		},
	}
	for range peakN {
		b := make([]byte, size)
		bp.p.Put(&b)
	}
	return bp
}

// Get returns a full-capacity buffer from the pool. Callers slice it to
// the needed length and MUST Put the original full-cap slice back when
// done.
func (p *Pool) Get() []byte {
	ptr, ok := p.p.Get().(*[]byte)
	if !ok {
		// Unreachable: every value Put into p.p is *[]byte, and New
		// returns the same shape. Fall back to a fresh allocation so a
		// programmer error cannot crash the consumer.
		b := make([]byte, p.size)
		return b
	}
	return *ptr
}

// Put returns a buffer to the pool. Re-slices to full capacity so the
// next Get caller sees len == cap. Silently drops buffers of the wrong
// capacity (caller bug, not worth panicking on in production).
func (p *Pool) Put(b []byte) {
	if cap(b) != p.size {
		return
	}
	b = b[:cap(b)]
	p.p.Put(&b)
}

// Size reports the fixed buffer size managed by this pool.
func (p *Pool) Size() int { return p.size }

// Name reports the pool's debug label.
func (p *Pool) Name() string { return p.name }
