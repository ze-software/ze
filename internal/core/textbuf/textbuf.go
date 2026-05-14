// Design: ai/rules/no-sprintf-alloc.md — zero-allocation text formatting helpers

package textbuf

import (
	"encoding/hex"
	"net/netip"
	"strconv"
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

// Buffer is a stack-allocated string builder for zero-intermediate-alloc
// formatting. Declare as `var b textbuf.Buffer`, chain methods, call String().
// The 128-byte backing array stays on the stack; only String() allocates.
type Buffer struct {
	arr [128]byte
	b   []byte
}

func (b *Buffer) grow() {
	if b.b == nil {
		b.b = b.arr[:0]
	}
}

func (b *Buffer) Str(s string) *Buffer      { b.grow(); b.b = append(b.b, s...); return b }
func (b *Buffer) Byte(c byte) *Buffer       { b.grow(); b.b = append(b.b, c); return b }
func (b *Buffer) Uint(v uint64) *Buffer     { b.grow(); b.b = strconv.AppendUint(b.b, v, 10); return b }
func (b *Buffer) Uint8(v uint8) *Buffer     { return b.Uint(uint64(v)) }
func (b *Buffer) Uint16(v uint16) *Buffer   { return b.Uint(uint64(v)) }
func (b *Buffer) Uint32(v uint32) *Buffer   { return b.Uint(uint64(v)) }
func (b *Buffer) Int(v int64) *Buffer       { b.grow(); b.b = strconv.AppendInt(b.b, v, 10); return b }
func (b *Buffer) Addr(a netip.Addr) *Buffer { b.grow(); b.b = a.AppendTo(b.b); return b }
func (b *Buffer) Hex(data []byte) *Buffer   { b.grow(); b.b = hex.AppendEncode(b.b, data); return b }
func (b *Buffer) String() string            { return string(b.b) }

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
