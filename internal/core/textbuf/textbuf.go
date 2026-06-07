// Design: ai/rules/no-sprintf-alloc.md — zero-allocation text formatting helpers

package textbuf

import (
	"encoding/hex"
	"net/netip"
	"strconv"
	"sync"
	"unicode/utf8"
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
// String does not freeze the buffer: writes after String are safe because
// inline data is copied and heap data is detached. Slice freezes the buffer;
// writes after Slice panic until Reset.
//
// String: inline (<=128B) copies, heap (>128B) is zero-copy and transfers
// the heap slice to the returned string. The buffer reverts to its inline
// array, so the string is safe to hold indefinitely.
//
// Slice: always zero-copy regardless of size. The returned string shares
// the buffer's memory and is only valid until the next Reset or Release.
//
// Implements io.Writer, io.StringWriter, and io.ByteWriter.
type Buffer struct {
	arr    [128]byte
	b      []byte
	done   bool
	pooled bool
	color  bool
}

// SetColor enables or disables ANSI color output for this buffer.
func (b *Buffer) SetColor(enabled bool) *Buffer {
	b.color = enabled
	return b
}

var bufPool = sync.Pool{
	New: func() any { return new(Buffer) },
}

// Get returns a Buffer from the pool. Call Release when done.
func Get() *Buffer {
	b, _ := bufPool.Get().(*Buffer) //nolint:forcetypeassert // pool only holds *Buffer
	b.done = false
	b.pooled = true
	b.color = false // reset rendering mode so a prior SetColor(true) does not leak across pool reuse
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

func (b *Buffer) Repeat(s string, n int) *Buffer {
	b.mustBeWritable()
	for range n {
		b.b = append(b.b, s...)
	}
	return b
}

func (b *Buffer) PadRight(s string, width int) *Buffer {
	b.mustBeWritable()
	b.b = append(b.b, s...)
	for range width - utf8.RuneCountInString(s) {
		b.b = append(b.b, ' ')
	}
	return b
}

func (b *Buffer) Float(v float64, prec int) *Buffer {
	b.mustBeWritable()
	b.b = strconv.AppendFloat(b.b, v, 'f', prec, 64)
	return b
}

func (b *Buffer) Float2(v float64) *Buffer {
	return b.Float(v, 2)
}

func (b *Buffer) Bool(v bool) *Buffer {
	b.mustBeWritable()
	b.b = strconv.AppendBool(b.b, v)
	return b
}

func (b *Buffer) Prefix(p netip.Prefix) *Buffer {
	b.mustBeWritable()
	b.b = p.AppendTo(b.b)
	return b
}

func (b *Buffer) Quoted(s string) *Buffer {
	b.mustBeWritable()
	b.b = strconv.AppendQuote(b.b, s)
	return b
}

func (b *Buffer) Err(err error) *Buffer {
	b.mustBeWritable()
	if err != nil {
		b.b = append(b.b, err.Error()...)
	}
	return b
}

func (b *Buffer) MAC(mac []byte) *Buffer {
	b.mustBeWritable()
	if len(mac) < 6 {
		return b
	}
	const digits = "0123456789abcdef"
	for i := range 6 {
		if i > 0 {
			b.b = append(b.b, ':')
		}
		b.b = append(b.b, digits[mac[i]>>4], digits[mac[i]&0x0f])
	}
	return b
}

func (b *Buffer) Join(items []string, sep string) *Buffer {
	b.mustBeWritable()
	for i, s := range items {
		if i > 0 {
			b.b = append(b.b, sep...)
		}
		b.b = append(b.b, s...)
	}
	return b
}

func (b *Buffer) PadLeft(s string, width int) *Buffer {
	b.mustBeWritable()
	for range width - utf8.RuneCountInString(s) {
		b.b = append(b.b, ' ')
	}
	b.b = append(b.b, s...)
	return b
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mustBeWritable()
	b.b = append(b.b, p...)
	return len(p), nil
}

func (b *Buffer) WriteString(s string) (int, error) {
	b.mustBeWritable()
	b.b = append(b.b, s...)
	return len(s), nil
}

func (b *Buffer) WriteByte(c byte) error {
	b.mustBeWritable()
	b.b = append(b.b, c)
	return nil
}

func (b *Buffer) WriteRune(r rune) (int, error) {
	b.mustBeWritable()
	if r < utf8.RuneSelf {
		b.b = append(b.b, byte(r))
		return 1, nil
	}
	l := len(b.b)
	b.b = utf8.AppendRune(b.b, r)
	return len(b.b) - l, nil
}

func (b *Buffer) Len() int { return len(b.b) }

// String returns the buffer contents. Inline data (<=128B) is copied; heap
// data (>128B) is returned zero-copy with the heap slice detached. Unlike
// Slice, String does NOT freeze the buffer: subsequent writes are safe
// because inline data was copied and heap data was detached.
func (b *Buffer) String() string {
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
// or Release. Unlike String, Slice freezes the buffer: writes after Slice
// would corrupt the returned view, so they panic.
func (b *Buffer) Slice() string {
	b.done = true
	return unsafe.String(unsafe.SliceData(b.b), len(b.b)) //nolint:gosec // zero-copy always; caller must not hold past Reset()
}

// Color is an ANSI escape sequence for terminal output.
type Color = string

const (
	ColorReset        Color = "\033[0m"
	ColorDim          Color = "\033[2m"
	ColorBoldRed      Color = "\033[1;31m"
	ColorBrightGreen  Color = "\033[92m"
	ColorBrightYellow Color = "\033[93m"
	ColorBrightCyan   Color = "\033[96m"
	ColorBoldCyan     Color = "\033[1;96m"
	ColorBoldMagenta  Color = "\033[1;95m"
)

type colors struct {
	Reset        Color
	Dim          Color
	BoldRed      Color
	BrightGreen  Color
	BrightYellow Color
	BrightCyan   Color
	BoldCyan     Color
	BoldMagenta  Color
}

// C holds all ANSI color constants. Assign locally for short access:
//
//	c := textbuf.C
//	tb.Colored(c.BoldCyan).Str("hello").Colored(c.Reset)
var C = colors{
	Reset:        ColorReset,
	Dim:          ColorDim,
	BoldRed:      ColorBoldRed,
	BrightGreen:  ColorBrightGreen,
	BrightYellow: ColorBrightYellow,
	BrightCyan:   ColorBrightCyan,
	BoldCyan:     ColorBoldCyan,
	BoldMagenta:  ColorBoldMagenta,
}

// Colored appends the ANSI sequence for c if color is enabled on this buffer.
// No-op otherwise. Use ColorReset to terminate a colored span.
func (b *Buffer) Colored(c Color) *Buffer {
	if b.color {
		b.Str(c)
	}
	return b
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

func AppendPrefix(dst []byte, p netip.Prefix) []byte {
	return p.AppendTo(dst)
}

func AppendMAC(dst, mac []byte) []byte {
	if len(mac) < 6 {
		return dst
	}
	const digits = "0123456789abcdef"
	for i := range 6 {
		if i > 0 {
			dst = append(dst, ':')
		}
		dst = append(dst, digits[mac[i]>>4], digits[mac[i]&0x0f])
	}
	return dst
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

func Prefix(p netip.Prefix) string {
	var buf [43]byte
	return string(p.AppendTo(buf[:0]))
}

func MAC(mac []byte) string {
	if len(mac) < 6 {
		return ""
	}
	const digits = "0123456789abcdef"
	var buf [17]byte
	for i := range 6 {
		if i > 0 {
			buf[i*3-1] = ':'
		}
		buf[i*3] = digits[mac[i]>>4]
		buf[i*3+1] = digits[mac[i]&0x0f]
	}
	return string(buf[:])
}

func HostPort(host string, port uint16) string {
	var b Buffer
	return b.Reset().Str(host).Byte(':').Uint(uint64(port)).String()
}

func Join(items []string, sep string) string {
	if len(items) == 0 {
		return ""
	}
	if len(items) == 1 {
		return items[0]
	}
	n := (len(items) - 1) * len(sep)
	for _, s := range items {
		n += len(s)
	}
	var b Buffer
	b.Reset(n)
	b.Str(items[0])
	for _, s := range items[1:] {
		b.Str(sep).Str(s)
	}
	return b.String()
}
