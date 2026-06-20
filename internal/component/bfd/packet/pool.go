// Design: rfc/short/rfc5880.md -- BFD packet size bounds
// Design: .claude/rules/buffer-first.md -- pool sized to RFC max
// Related: control.go -- WriteTo writes into a pool buffer
// Related: auth.go -- ParseAuth indexes into a pool buffer
//
// Pooled byte buffers sized for the largest BFD Control packet.
//
// RFC 5880 caps a single Control packet at the mandatory section (24
// bytes) plus the longest defined authentication section (28 bytes for
// SHA1 family). PoolBufSize is the next power-of-two above that cap so
// the slice base alignment is friendly to the network stack and to
// fuzz harnesses.
//
// The pool stores `*[]byte` so the same pointer round-trips through
// Acquire/Release without escaping a fresh slice header to the heap on
// every release. Callers acquire a Buf, write into Buf.Data(), and pass
// the Buf back to Release. This is the sync.Pool-friendly pattern
// described in the Go runtime docs.
package packet

import "sync"

// PoolBufSize is the fixed capacity of every pool buffer. Sized to fit
// any BFD Control packet defined by RFC 5880, with headroom for future
// TLVs.
const PoolBufSize = 64

// Buf is a pooled byte slice. Callers write into Buf.Data() and pass
// the same Buf value back to Release. Buf is deliberately a one-field
// struct around *[]byte so the pool stores a stable pointer and
// sync.Pool.Put does not escape a fresh slice header per release.
type Buf struct {
	bp *[]byte
}

// Data returns the full-capacity byte slice backing the Buf. The slice
// is NOT zeroed between uses; callers must overwrite every byte they
// rely on.
func (b Buf) Data() []byte { return *b.bp }

// bufPool holds reusable *[]byte values. New runs exactly once per
// added pool slot and is the only place make() runs on the encode/
// decode hot path.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, PoolBufSize)
		return &b
	},
}

// Acquire returns a pool Buf. Caller MUST call Release exactly once.
// Reusing the underlying slice after Release is a use-after-free bug.
func Acquire() Buf {
	bp, ok := bufPool.Get().(*[]byte)
	if !ok || bp == nil {
		// Pool's New always produces a *[]byte, so this is
		// unreachable. The fallback avoids a panic if a future
		// refactor corrupts the pool.
		b := make([]byte, PoolBufSize)
		return Buf{bp: &b}
	}
	return Buf{bp: bp}
}

// Release returns a Buf to the pool. Caller MUST NOT use Buf.Data()
// after calling Release.
func Release(b Buf) {
	if b.bp == nil {
		return
	}
	if cap(*b.bp) != PoolBufSize {
		return
	}
	bufPool.Put(b.bp)
}
