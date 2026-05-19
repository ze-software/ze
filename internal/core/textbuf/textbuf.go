// Design: ai/rules/no-sprintf-alloc.md — zero-allocation text formatting helpers

package textbuf

import (
	"encoding/hex"
	"net/netip"
	"strconv"
	"sync"
	"unsafe"
)

func Uint(v uint64) string {
	var buf [20]byte
	return string(strconv.AppendUint(buf[:0], v, 10))
}

func Uint8(v uint8) string   { return Uint(uint64(v)) }
func Uint16(v uint16) string { return Uint(uint64(v)) }
func Uint32(v uint32) string { return Uint(uint64(v)) }

func Int(v int64) string {
	var buf [20]byte
	return string(strconv.AppendInt(buf[:0], v, 10))
}

func Addr(addr netip.Addr) string {
	var buf [39]byte
	return string(addr.AppendTo(buf[:0]))
}

func Hex(data []byte) string {
	var buf [64]byte
	dst := buf[:0]
	if hex.EncodedLen(len(data)) > len(buf) {
		dst = make([]byte, 0, hex.EncodedLen(len(data)))
	}
	return string(hex.AppendEncode(dst, data))
}

const upperHexDigits = "0123456789ABCDEF"

func HexUpper(data []byte) string {
	var buf [128]byte
	n := len(data) * 2
	var dst []byte
	if n <= len(buf) {
		dst = buf[:n]
	} else {
		dst = make([]byte, n)
	}
	for i, b := range data {
		dst[i*2] = upperHexDigits[b>>4]
		dst[i*2+1] = upperHexDigits[b&0x0f]
	}
	return string(dst)
}

// Buffer is a zero-alloc string builder with a 128-byte inline backing array.
//
// Stack use: var b textbuf.Buffer; b.Reset().Str("x").Uint(1).String().
// Pooled use: b := textbuf.Get(); defer b.Release(); b.Str("x").String().
//
// Both String and Slice freeze the buffer; all writes panic until Reset.
//
// String: inline (<=128B) copies, heap (>128B) is zero-copy and transfers
// the heap slice to the returned string. The buffer reverts to its inline
// array, so the string is safe to hold indefinitely.
//
// Slice: always zero-copy regardless of size. The returned string shares
// the buffer's memory and is only valid until the next Reset or Release.
type Buffer struct {
	arr    [128]byte
	b      []byte
	done   bool
	pooled bool
}

var bufPool = sync.Pool{
	New: func() any { return new(Buffer) },
}

// Get returns a Buffer from the pool. Call Release when done.
func Get() *Buffer {
	b, _ := bufPool.Get().(*Buffer) //nolint:forcetypeassert // pool only holds *Buffer
	b.done = false
	b.pooled = true
	if cap(b.b) > len(b.arr) {
		b.b = b.b[:0]
	} else {
		b.b = b.arr[:0]
	}
	return b
}

// Release returns a pooled Buffer for reuse. No-op on stack buffers or after
// a prior Release. Safe to call after String (the normal pattern).
func (b *Buffer) Release() {
	if !b.pooled {
		return
	}
	b.pooled = false
	b.done = true
	bufPool.Put(b)
}

func (b *Buffer) mustBeWritable() {
	if b.done {
		panic("BUG: textbuf write after String")
	}
}

func (b *Buffer) Reset(size ...int) *Buffer {
	b.done = false
	if len(size) > 0 && size[0] > len(b.arr) {
		if size[0] > cap(b.b) {
			b.b = make([]byte, 0, size[0])
		} else {
			b.b = b.b[:0]
		}
	} else {
		b.b = b.arr[:0]
	}
	return b
}

func (b *Buffer) Grow(n int) *Buffer {
	b.mustBeWritable()
	if n > 0 && cap(b.b)-len(b.b) < n {
		newBuf := make([]byte, len(b.b), len(b.b)+n)
		copy(newBuf, b.b)
		b.b = newBuf
	}
	return b
}

func (b *Buffer) Str(s string) *Buffer {
	b.mustBeWritable()
	b.b = append(b.b, s...)
	return b
}

func (b *Buffer) Byte(c byte) *Buffer {
	b.mustBeWritable()
	b.b = append(b.b, c)
	return b
}

func (b *Buffer) Uint(v uint64) *Buffer {
	b.mustBeWritable()
	b.b = strconv.AppendUint(b.b, v, 10)
	return b
}

func (b *Buffer) Uint8(v uint8) *Buffer   { return b.Uint(uint64(v)) }
func (b *Buffer) Uint16(v uint16) *Buffer { return b.Uint(uint64(v)) }
func (b *Buffer) Uint32(v uint32) *Buffer { return b.Uint(uint64(v)) }

func (b *Buffer) Int(v int64) *Buffer {
	b.mustBeWritable()
	b.b = strconv.AppendInt(b.b, v, 10)
	return b
}

func (b *Buffer) Addr(a netip.Addr) *Buffer {
	b.mustBeWritable()
	b.b = a.AppendTo(b.b)
	return b
}

func (b *Buffer) Hex(data []byte) *Buffer {
	b.mustBeWritable()
	b.b = hex.AppendEncode(b.b, data)
	return b
}

func (b *Buffer) HexUpper(data []byte) *Buffer {
	b.mustBeWritable()
	for _, v := range data {
		b.b = append(b.b, upperHexDigits[v>>4], upperHexDigits[v&0x0f])
	}
	return b
}

func (b *Buffer) Float2(v float64) *Buffer {
	b.mustBeWritable()
	b.b = strconv.AppendFloat(b.b, v, 'f', 2, 64)
	return b
}

func (b *Buffer) Bool(v bool) *Buffer {
	b.mustBeWritable()
	b.b = strconv.AppendBool(b.b, v)
	return b
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mustBeWritable()
	b.b = append(b.b, p...)
	return len(p), nil
}

func (b *Buffer) Len() int { return len(b.b) }

// String freezes the buffer and returns its contents. Inline data (<=128B) is
// copied. Heap data (>128B) is returned zero-copy; the buffer detaches the
// heap slice so the string is safe to hold past Reset or Release.
func (b *Buffer) String() string {
	b.done = true
	if len(b.b) == 0 {
		return ""
	}
	if unsafe.SliceData(b.b) == &b.arr[0] { //nolint:gosec // inline-backed: must copy
		return string(b.b)
	}
	s := unsafe.String(unsafe.SliceData(b.b), len(b.b)) //nolint:gosec // heap-backed: zero-copy, string owns the slice
	b.b = b.arr[:0]
	return s
}

// Slice freezes the buffer and returns its contents zero-copy at any size.
// The returned string shares the buffer's memory and is invalid after Reset
// or Release. Use for short-lived strings consumed before the next build.
func (b *Buffer) Slice() string {
	b.done = true
	return unsafe.String(unsafe.SliceData(b.b), len(b.b)) //nolint:gosec // zero-copy always; caller must not hold past Reset()
}

func AppendUint(dst []byte, v uint64) []byte {
	return strconv.AppendUint(dst, v, 10)
}

func AppendInt(dst []byte, v int64) []byte {
	return strconv.AppendInt(dst, v, 10)
}

func AppendAddr(dst []byte, addr netip.Addr) []byte {
	return addr.AppendTo(dst)
}

func AppendHex(dst, data []byte) []byte {
	return hex.AppendEncode(dst, data)
}

func IntStr(v int64, suffix string) string {
	var b Buffer
	return b.Reset().Int(v).Str(suffix).String()
}

func UintStr(v uint64, suffix string) string {
	var b Buffer
	return b.Reset().Uint(v).Str(suffix).String()
}

func StrInt(prefix string, v int64) string {
	var b Buffer
	return b.Reset().Str(prefix).Int(v).String()
}

func StrUint(prefix string, v uint64) string {
	var b Buffer
	return b.Reset().Str(prefix).Uint(v).String()
}

func StrIntStr(prefix string, v int64, suffix string) string {
	var b Buffer
	return b.Reset().Str(prefix).Int(v).Str(suffix).String()
}

func StrUintStr(prefix string, v uint64, suffix string) string {
	var b Buffer
	return b.Reset().Str(prefix).Uint(v).Str(suffix).String()
}
