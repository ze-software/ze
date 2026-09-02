// Design: docs/architecture/textbuf-string-building.md -- zero-allocation text formatting helpers

package textbuf

import (
	"encoding/hex"
	"net/netip"
	"os"
	"strconv"
	"sync"
	"unicode/utf8"
	"unsafe"
)

func byteIndex(s string, c byte) int {
	for i := range len(s) {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func StringUint(v uint64) string {
	var buf [20]byte
	return string(strconv.AppendUint(buf[:0], v, 10))
}

func StringUint8(v uint8) string   { return StringUint(uint64(v)) }
func StringUint16(v uint16) string { return StringUint(uint64(v)) }
func StringUint32(v uint32) string { return StringUint(uint64(v)) }

func StringInt(v int64) string {
	var buf [20]byte
	return string(strconv.AppendInt(buf[:0], v, 10))
}

func StringAddr(addr netip.Addr) string {
	var buf [39]byte
	return string(addr.AppendTo(buf[:0]))
}

func StringHex(data []byte) string {
	var buf [64]byte
	dst := buf[:0]
	if hex.EncodedLen(len(data)) > len(buf) {
		dst = make([]byte, 0, hex.EncodedLen(len(data)))
	}
	return string(hex.AppendEncode(dst, data))
}

const upperHexDigits = "0123456789ABCDEF"

func StringHexUpper(data []byte) string {
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

// Buffer is a string builder with a 128-byte inline backing array.
//
// Stack residence: Reset() uses noescape (same technique as strings.Builder
// via abi.NoEscape) to break the self-referential b.b = b.arr[:0] from
// escape analysis. var b Buffer stays on the stack for local use.
//
// Choosing an init:
//
//	Local use (0 alloc):  var b Buffer; b.Reset().Str("x").Bytes()
//	Return a string:      b := New(); return b.Str("x").Slice()
//	Hot loop (amortized): b := Get(); defer b.Release()
//	                      for ... { use(b.Reset().Str("x").Slice()) }
//
// String: hands back the contents and EMPTIES the buffer, at every size.
// Does not freeze; writes after String are safe and start from empty.
//
// Slice: always zero-copy. Freezes the buffer; writes panic until Reset.
// Valid only until the next Reset or Release. Do not use with Get()
// unless the string is consumed before Release (pool exhaustion).
//
// Bytes: returns raw []byte sharing buffer memory. For w.Write() or
// string(b.Bytes()) in map/switch (compiler elides alloc).
//
// StdOut / StdErr: print the buffer to a standard stream without extracting
// it. The write consumes the bytes, so no string is built and the buffer is
// not frozen.
//
// Go compiler review gate: noescape mirrors strings.Builder. On every
// Go update, compare against $(go env GOROOT)/src/strings/builder.go.
//
// Implements io.Writer, io.StringWriter, and io.ByteWriter.
//
// NEVER COPY A BUFFER THAT HAS BEEN WRITTEN TO. Pass it as *Buffer, and when
// one lives in a struct, hold that struct by pointer. strings.Builder catches
// this: copyCheck runs on every write and panics on a copy. Buffer has no such
// check, and there is nowhere cheap to put one, because the whole point of the
// inline array is that a fresh Buffer costs nothing. A copy leaves the copy's
// b field pointing into the ORIGINAL's arr, so the two write over each other
// with no panic, no race detector report and no test red.
//
// The shape to watch is a Buffer as a struct FIELD. `[]*node` is safe and
// `[]node` is not, and the day someone changes one to the other, nothing in
// this package objects.
//
// RESET BEFORE THE FIRST WRITE. The inline array is what makes a Buffer cheaper
// than a strings.Builder, and a zero value never reaches it: b.b is nil, the
// first write appends to a nil slice, and the allocator hands back a heap slice
// exactly as strings.Builder would. Only Reset, New and Get point b.b at arr.
// So `var b Buffer` followed by b.Str(...) buys nothing over the type it
// replaced. `var b Buffer; b.Reset()` is the whole fix, and it allocates
// nothing.
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

// New returns a heap-allocated Buffer. Use Slice() to extract the result
// zero-copy; the GC keeps the Buffer alive via the interior pointer in
// the returned string. Prefer Get() in loops where the allocation can be
// amortized.
func New() *Buffer {
	b := new(Buffer)
	b.b = b.inlineSlice()
	return b
}

var bufPool = sync.Pool{
	New: func() any { return new(Buffer) },
}

// Get returns a Buffer from the pool. Call Release when done.
// Use String() to extract (copies), then Release. Slice() before Release
// is only safe when the string is consumed before Release.
func Get() *Buffer {
	b, _ := bufPool.Get().(*Buffer) //nolint:forcetypeassert // pool only holds *Buffer
	b.done = false
	b.pooled = true
	b.color = false // reset rendering mode so a prior SetColor(true) does not leak across pool reuse
	if cap(b.b) > len(b.arr) {
		b.b = b.b[:0]
	} else {
		b.b = b.inlineSlice()
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

// noescape hides a pointer from escape analysis. Same technique as
// strings.Builder (via abi.NoEscape): the uintptr round-trip prevents
// the compiler from seeing that the returned pointer equals the input.
// This lets Reset() set b.b = b.arr[:0] without the self-referential
// slice forcing the Buffer to the heap.
//
//go:nosplit
//go:nocheckptr
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0) //nolint:gosec,govet,staticcheck // escape analysis break, compiles to no-op
}

// inlineSlice returns b.arr[:0:128] without creating a self-referential
// slice visible to escape analysis.
func (b *Buffer) inlineSlice() []byte {
	return unsafe.Slice((*byte)(noescape(unsafe.Pointer(&b.arr[0]))), len(b.arr))[:0] //nolint:gosec // see noescape
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
		b.b = b.inlineSlice()
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

// HostPort appends "host:port" (string port, e.g. from net.SplitHostPort).
// IPv6 hosts (containing ':') are bracketed: "[::1]:80".
func (b *Buffer) HostPort(host, port string) *Buffer {
	if byteIndex(host, ':') >= 0 {
		return b.Byte('[').Str(host).Byte(']').Byte(':').Str(port)
	}
	return b.Str(host).Byte(':').Str(port)
}

// HostPortN appends "host:port" (numeric port).
// IPv6 hosts (containing ':') are bracketed: "[::1]:80".
func (b *Buffer) HostPortN(host string, port uint16) *Buffer {
	if byteIndex(host, ':') >= 0 {
		return b.Byte('[').Str(host).Byte(']').Byte(':').Uint(uint64(port))
	}
	return b.Str(host).Byte(':').Uint(uint64(port))
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

// Bytes returns the buffer contents as a byte slice. The slice shares the
// buffer's memory and is valid until the next Reset, Release, or write that
// triggers a grow. Does not freeze the buffer.
//
// Use for io.Writer output (w.Write(b.Bytes())) or to enable Go's
// compiler optimization for string([]byte) in map lookups and switch
// statements (the compiler elides the allocation when the conversion
// does not escape).
func (b *Buffer) Bytes() []byte { return b.b }

// String returns the buffer contents and EMPTIES the buffer. Unlike Slice,
// String does not freeze it: the next write starts a new string, from empty.
//
// READ IT ONCE, LAST. This is where Buffer differs from strings.Builder, which
// keeps everything and answers it again on a second call:
//
//	s1 := b.String()      // b is now empty
//	b.Str("more")
//	s2 := b.String()      // Buffer: "more".  strings.Builder: "...more"
//
// ONE size rule, and it holds at every size (owner directive, 2026-09-02). A
// five-byte buffer empties exactly as a five-kilobyte one does. Before that
// directive the inline case (<=128B, and only when Reset, New or Get had
// pointed the buffer at its own array) kept its content while the heap case
// detached, so the same code passed on a small fixture and truncated on real
// input. Nothing now depends on how much the buffer holds, on which init it
// got, or on how it happens to be backed. A read-then-write defect fails on
// the first test that runs it, whatever the fixture.
//
// Cost differs where behavior does not: inline data is copied because the array
// is written over in place, heap data is handed to the string zero-copy. That
// is an allocation question, never a semantic one. Use Slice when the string is
// consumed immediately and zero-copy at every size is what you want.
//
// So judge a call site by whether anything writes to the buffer afterwards on
// any path, a loop that reads per iteration and a helper that appends after the
// read included. Converting a strings.Builder to a Buffer is the moment this
// bites, because every other method carries over unchanged.
func (b *Buffer) String() string {
	if len(b.b) == 0 {
		return ""
	}
	// Inline data is copied because the array is written over in place. Heap
	// data is handed to the string, which then owns it. The two branches differ
	// in cost and in nothing else: each leaves the buffer empty.
	var s string
	if unsafe.SliceData(b.b) == &b.arr[0] { //nolint:gosec // inline-backed: must copy
		s = string(b.b)
	} else {
		s = unsafe.String(unsafe.SliceData(b.b), len(b.b)) //nolint:gosec // heap-backed: zero-copy, the string owns the slice
	}
	b.b = b.inlineSlice()
	return s
}

// StdOut writes the buffer contents to standard output.
//
// Nothing is copied and nothing is extracted: os.Stdout.Write consumes the
// bytes before it returns, so neither String nor Slice is needed. The buffer
// is not frozen, so it can be Reset and reused for the next line.
//
// No newline is added. Write one into the buffer when the output needs it.
func (b *Buffer) StdOut() error {
	_, err := os.Stdout.Write(b.b)
	return err
}

// StdErr writes the buffer contents to standard error, on the same terms as
// StdOut: no copy, no freeze, and no newline of its own.
func (b *Buffer) StdErr() error {
	_, err := os.Stderr.Write(b.b)
	return err
}

// Slice freezes the buffer and returns its contents zero-copy at any size.
// The returned string shares the buffer's memory and is invalid after Reset
// or Release. Unlike String, Slice freezes the buffer: writes after Slice
// would corrupt the returned view, so they panic.
//
// Prefer Slice over String when the string is consumed immediately (passed
// to WriteString, Printf, or similar I/O) or when it is the last extraction
// from the buffer before the buffer goes out of scope.
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

func Uint(dst []byte, v uint64) []byte {
	return strconv.AppendUint(dst, v, 10)
}

func Int(dst []byte, v int64) []byte {
	return strconv.AppendInt(dst, v, 10)
}

func Addr(dst []byte, addr netip.Addr) []byte {
	return addr.AppendTo(dst)
}

func Hex(dst, data []byte) []byte {
	return hex.AppendEncode(dst, data)
}

func HexUpper(dst, data []byte) []byte {
	for _, v := range data {
		dst = append(dst, upperHexDigits[v>>4], upperHexDigits[v&0x0f])
	}
	return dst
}

func Prefix(dst []byte, p netip.Prefix) []byte {
	return p.AppendTo(dst)
}

func MAC(dst, mac []byte) []byte {
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

func StringPrefix(p netip.Prefix) string {
	var buf [43]byte
	return string(p.AppendTo(buf[:0]))
}

func StringMAC(mac []byte) string {
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

// HostPort returns "host:port" without net.JoinHostPort or fmt.Sprintf.
func HostPort(host string, port uint16) string {
	var b Buffer
	return b.HostPortN(host, port).String()
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
