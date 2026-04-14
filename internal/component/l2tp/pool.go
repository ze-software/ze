// Design: docs/architecture/wire/l2tp.md — buffer pool for L2TP control messages
// Related: header.go — control message assembly using pooled buffers
// Related: hidden.go — hidden-AVP scratch region using pooled buffers

package l2tp

import "sync"

// BufSize is the fixed allocation size for pooled L2TP buffers. The value
// matches the UDP/Ethernet MTU; L2TP control messages are well below this
// (typical SCCRQ is around 100-200 octets, worst case under 1024).
const BufSize = 1500

var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, BufSize)
		return &b
	},
}

// GetBuf returns a pooled 1500-byte buffer. The buffer contents are
// undefined; callers MUST treat it as uninitialized.
//
// Caller MUST call PutBuf with the same pointer when done. Buffers held
// past the call chain of a single outbound message create cross-message
// aliasing bugs.
func GetBuf() *[]byte {
	b, _ := bufPool.Get().(*[]byte)
	return b
}

// PutBuf returns a buffer to the pool. The pointer MUST originate from
// GetBuf; passing a pointer to a different allocation is a bug.
func PutBuf(b *[]byte) {
	if b == nil || len(*b) != BufSize {
		return
	}
	bufPool.Put(b)
}
