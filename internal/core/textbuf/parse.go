// Design: docs/architecture/textbuf-string-building.md -- zero-allocation positional counted wire parser on Buffer.
// Related: textbuf.go -- Buffer ownership, freezing, and string building.

package textbuf

import (
	"encoding/binary"
	"errors"
	"math"
	"strconv"
	"unsafe"
)

const (
	// Uint64DigitsMax is the maximum number of decimal digits in a uint64 (18446744073709551615).
	Uint64DigitsMax = 20
	// Word4Width is the fixed byte width of a Word4 wire token (3 ASCII bytes plus trailing space).
	Word4Width = 4
)

var (
	// ErrEmptyNumber reports a numeric field with no decimal digits before its delimiter.
	ErrEmptyNumber = errors.New("textbuf: empty numeric field")
	// ErrInvalidDigit reports a non-decimal byte in a numeric or counted-length field.
	ErrInvalidDigit = errors.New("textbuf: invalid decimal digit")
	// ErrNumberTooWide reports a decimal field exceeding Uint64DigitsMax digits.
	ErrNumberTooWide = errors.New("textbuf: numeric field exceeds 20 decimal digits")
	// ErrNumberOverflow reports a 20-digit decimal value exceeding math.MaxUint64.
	ErrNumberOverflow = errors.New("textbuf: numeric field overflows uint64")
	// ErrMissingSpace reports a missing single-space field separator.
	ErrMissingSpace = errors.New("textbuf: missing space separator")
	// ErrNumberHasColon reports a numeric field terminated by ':' instead of a space or end of buffer.
	ErrNumberHasColon = errors.New("textbuf: numeric field closed by ':'")
	// ErrEmptyCount reports a counted field whose length prefix contains no decimal digits.
	ErrEmptyCount = errors.New("textbuf: counted field length has no digits")
	// ErrMissingColon reports a counted field length not terminated by ':'.
	ErrMissingColon = errors.New("textbuf: counted field length not closed by ':'")
	// ErrCountOutOfBounds reports a counted field whose declared byte count exceeds the remaining buffer.
	ErrCountOutOfBounds = errors.New("textbuf: counted field length exceeds remaining buffer")
	// ErrTrailingBytes reports unexpected bytes remaining after a field expected to end the buffer.
	ErrTrailingBytes = errors.New("textbuf: unexpected trailing bytes")
)

// Word4 is a 4-byte wire verb word packing a 3-byte token and trailing space
// in little-endian byte order for single-instruction switch dispatch.
type Word4 uint32

// MakeWord4 packs a 3-byte token and trailing space into a Word4. Panics if
// len(token) != 3 because wire verb words are compile-time constants.
func MakeWord4(token string) Word4 {
	if len(token) != 3 {
		panic("BUG: textbuf.MakeWord4 requires a 3-byte token")
	}
	return Word4(uint32(token[0]) | uint32(token[1])<<8 | uint32(token[2])<<16 | uint32(' ')<<24)
}

// PeekWord4 freezes the buffer and returns the 4-byte Word4 at the cursor
// without advancing it. Returns (0, false) if fewer than Word4Width bytes remain.
func (b *Buffer) PeekWord4() (Word4, bool) {
	b.done = true
	if len(b.b)-b.pos < Word4Width {
		return 0, false
	}
	return Word4(binary.LittleEndian.Uint32(b.b[b.pos : b.pos+Word4Width])), true
}

// ReadWord4 freezes the buffer, reads the 4-byte Word4 at the cursor, and
// advances the cursor by Word4Width on success.
func (b *Buffer) ReadWord4() (Word4, bool) {
	w, ok := b.PeekWord4()
	if !ok {
		return 0, false
	}
	b.pos += Word4Width
	return w, true
}

// Pos returns the current read cursor byte offset in the buffer.
func (b *Buffer) Pos() int { return b.pos }

// Remaining returns the number of unread bytes remaining in the buffer.
func (b *Buffer) Remaining() int { return len(b.b) - b.pos }

// EOF reports whether the read cursor has reached the end of the buffer.
func (b *Buffer) EOF() bool { return b.pos >= len(b.b) }

// Skip advances the read cursor by n bytes. Returns false without moving the
// cursor if n is negative or exceeds the remaining bytes.
func (b *Buffer) Skip(n int) bool {
	if n < 0 {
		return false
	}
	if n > len(b.b)-b.pos {
		return false
	}
	b.pos += n
	return true
}

// RestBytes freezes the buffer and returns the unread remainder as a 3-index
// byte slice sharing buffer memory. Panics on a read-only string buffer.
func (b *Buffer) RestBytes() []byte {
	b.mustBeMutable()
	b.done = true
	return b.b[b.pos:len(b.b):len(b.b)]
}

// RestString freezes the buffer and returns the unread remainder as a zero-copy
// string sharing buffer memory.
func (b *Buffer) RestString() string {
	b.done = true
	if b.pos >= len(b.b) {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b.b[b.pos:]), len(b.b)-b.pos) //nolint:gosec // zero-copy view into frozen buffer
}

// TrimTrailingLF strips a single trailing '\n' byte from the unread tail of the
// buffer. Safe on read-only buffers because it only shortens the slice header.
func (b *Buffer) TrimTrailingLF() bool {
	if len(b.b) <= b.pos {
		return false
	}
	if b.b[len(b.b)-1] != '\n' {
		return false
	}
	b.b = b.b[:len(b.b)-1]
	return true
}

func (b *Buffer) readUint() (uint64, int, error) {
	b.done = true
	data := b.b
	n := len(data)
	start := b.pos
	var val uint64
	digits := 0
	for i := start; i < n; i++ {
		c := data[i]
		d := c - '0'
		if d <= 9 {
			if digits >= Uint64DigitsMax {
				return 0, start, ErrNumberTooWide
			}
			digit := uint64(d)
			if digits == Uint64DigitsMax-1 {
				if val > (math.MaxUint64-digit)/10 {
					return 0, start, ErrNumberOverflow
				}
			}
			val = val*10 + digit
			digits++
			continue
		}
		if c == ' ' {
			if digits == 0 {
				return 0, start, ErrEmptyNumber
			}
			return val, i, nil
		}
		if c == ':' {
			if digits == 0 {
				return 0, start, ErrEmptyNumber
			}
			return 0, start, ErrNumberHasColon
		}
		return 0, start, ErrInvalidDigit
	}
	if digits == 0 {
		return 0, start, ErrEmptyNumber
	}
	return val, n, nil
}

// ReadUint freezes the buffer and parses a base-10 uint64 at the cursor,
// stopping at a space or end of buffer without consuming the space. Leaves the
// cursor unchanged on error.
func (b *Buffer) ReadUint() (uint64, error) {
	val, end, err := b.readUint()
	if err != nil {
		return 0, err
	}
	b.pos = end
	return val, nil
}

// ReadUintSpace freezes the buffer, parses a base-10 uint64 terminated by a
// single space, and consumes the trailing space. Leaves the cursor unchanged on
// error.
func (b *Buffer) ReadUintSpace() (uint64, error) {
	val, end, err := b.readUint()
	if err != nil {
		return 0, err
	}
	if end >= len(b.b) {
		return 0, ErrMissingSpace
	}
	b.pos = end + 1
	return val, nil
}

// ReadSpace freezes the buffer and consumes a single ASCII space byte at the
// cursor. Leaves the cursor unchanged and returns ErrMissingSpace otherwise.
func (b *Buffer) ReadSpace() error {
	b.done = true
	if b.pos >= len(b.b) {
		return ErrMissingSpace
	}
	if b.b[b.pos] != ' ' {
		return ErrMissingSpace
	}
	b.pos++
	return nil
}

// PeekCountedHeader freezes the buffer and parses a counted field's "<n>:"
// length prefix at the cursor without advancing the cursor or requiring the
// payload bytes to be present. Returns the declared byte count and the header
// byte width (len("<n>:")).
func (b *Buffer) PeekCountedHeader() (uint64, int, error) {
	b.done = true
	data := b.b
	n := len(data)
	var size uint64
	digits := 0
	for i := b.pos; i < n; i++ {
		c := data[i]
		d := c - '0'
		if d <= 9 {
			if digits >= Uint64DigitsMax {
				return 0, 0, ErrNumberTooWide
			}
			digit := uint64(d)
			if digits == Uint64DigitsMax-1 {
				if size > (math.MaxUint64-digit)/10 {
					return 0, 0, ErrNumberOverflow
				}
			}
			size = size*10 + digit
			digits++
			continue
		}
		if c == ':' {
			if digits == 0 {
				return 0, 0, ErrEmptyCount
			}
			return size, digits + 1, nil
		}
		if digits > 0 {
			return 0, 0, ErrMissingColon
		}
		if c == ' ' {
			return 0, 0, ErrEmptyCount
		}
		return 0, 0, ErrInvalidDigit
	}
	if digits == 0 {
		return 0, 0, ErrEmptyCount
	}
	return 0, 0, ErrMissingColon
}

func (b *Buffer) readCountedSpan() (int, int, error) {
	size, headerLen, err := b.PeekCountedHeader()
	if err != nil {
		return 0, 0, err
	}
	start := b.pos + headerLen
	remaining := len(b.b) - start
	if size > uint64(remaining) {
		return 0, 0, ErrCountOutOfBounds
	}
	end := start + int(size)
	b.pos = end
	return start, end, nil
}

func (b *Buffer) stringSpan(start, end int) string {
	if start == end {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b.b[start:end]), end-start) //nolint:gosec // zero-copy view into frozen buffer
}

// ReadCountedBytes freezes the buffer, parses a "<n>:<bytes>" counted field at
// the cursor, and returns a 3-index byte slice sharing buffer memory with
// capacity capped at len so appending cannot overwrite following fields.
// Panics on a read-only string buffer. Leaves the cursor unchanged on error.
func (b *Buffer) ReadCountedBytes() ([]byte, error) {
	b.mustBeMutable()
	start, end, err := b.readCountedSpan()
	if err != nil {
		return nil, err
	}
	return b.b[start:end:end], nil
}

// ReadCountedBytesSpace parses a counted byte field followed by a single space
// separator and consumes the trailing space. Panics on a read-only string
// buffer. Leaves the cursor unchanged on error.
func (b *Buffer) ReadCountedBytesSpace() ([]byte, error) {
	b.mustBeMutable()
	saved := b.pos
	start, end, err := b.readCountedSpan()
	if err != nil {
		return nil, err
	}
	if err := b.ReadSpace(); err != nil {
		b.pos = saved
		return nil, err
	}
	return b.b[start:end:end], nil
}

// ReadCountedBytesEnd parses a counted byte field and requires the cursor to
// reach the end of the buffer. Panics on a read-only string buffer. Leaves the
// cursor unchanged on error.
func (b *Buffer) ReadCountedBytesEnd() ([]byte, error) {
	b.mustBeMutable()
	saved := b.pos
	start, end, err := b.readCountedSpan()
	if err != nil {
		return nil, err
	}
	if !b.EOF() {
		b.pos = saved
		return nil, ErrTrailingBytes
	}
	return b.b[start:end:end], nil
}

// ReadCountedString freezes the buffer, parses a "<n>:<bytes>" counted field at
// the cursor, and returns a zero-copy string view into buffer memory. Leaves
// the cursor unchanged on error.
func (b *Buffer) ReadCountedString() (string, error) {
	start, end, err := b.readCountedSpan()
	if err != nil {
		return "", err
	}
	return b.stringSpan(start, end), nil
}

// ReadCountedStringSpace parses a counted string field followed by a single
// space separator and consumes the trailing space. Leaves the cursor unchanged
// on error.
func (b *Buffer) ReadCountedStringSpace() (string, error) {
	saved := b.pos
	start, end, err := b.readCountedSpan()
	if err != nil {
		return "", err
	}
	if err := b.ReadSpace(); err != nil {
		b.pos = saved
		return "", err
	}
	return b.stringSpan(start, end), nil
}

// ReadCountedStringEnd parses a counted string field and requires the cursor to
// reach the end of the buffer. Leaves the cursor unchanged on error.
func (b *Buffer) ReadCountedStringEnd() (string, error) {
	saved := b.pos
	start, end, err := b.readCountedSpan()
	if err != nil {
		return "", err
	}
	if !b.EOF() {
		b.pos = saved
		return "", ErrTrailingBytes
	}
	return b.stringSpan(start, end), nil
}

// CountedBytes appends val framed as "<len(val)>:<val>" (or "0:" when empty).
func (b *Buffer) CountedBytes(val []byte) *Buffer {
	b.mustBeWritable()
	b.b = CountedBytes(b.b, val)
	return b
}

// CountedStr appends s framed as "<len(s)>:<s>" (or "0:" when empty).
func (b *Buffer) CountedStr(s string) *Buffer {
	b.mustBeWritable()
	b.b = CountedStr(b.b, s)
	return b
}

// CountedBytes appends val framed as "<len(val)>:<val>" onto dst.
func CountedBytes(dst, val []byte) []byte {
	dst = strconv.AppendUint(dst, uint64(len(val)), 10)
	dst = append(dst, ':')
	return append(dst, val...)
}

// CountedStr appends s framed as "<len(s)>:<s>" onto dst.
func CountedStr(dst []byte, s string) []byte {
	dst = strconv.AppendUint(dst, uint64(len(s)), 10)
	dst = append(dst, ':')
	return append(dst, s...)
}
