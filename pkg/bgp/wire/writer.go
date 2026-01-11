// Package wire provides zero-allocation buffer writing for BGP messages.
//
// This package implements the Buffer Writer Architecture described in
// .claude/zebgp/wire/BUFFER_WRITER.md for efficient UPDATE message building.
package wire

import "encoding/binary"

// Standard and extended BGP message sizes.
const (
	StandardMaxSize = 4096  // RFC 4271
	ExtendedMaxSize = 65535 // RFC 8654
)

// BufWriter writes directly into a pre-allocated buffer.
// All wire types (attributes, NLRI, messages) implement this interface.
type BufWriter interface {
	// WriteTo writes the wire representation into buf starting at offset.
	// Returns the number of bytes written.
	// Caller guarantees buf has sufficient capacity.
	WriteTo(buf []byte, off int) int
}

// CheckedBufWriter extends BufWriter with capacity validation.
type CheckedBufWriter interface {
	BufWriter
	// CheckedWriteTo validates capacity before writing.
	// Returns (bytesWritten, error). On error, buffer state is undefined.
	CheckedWriteTo(buf []byte, off int) (int, error)
	// Len returns the number of bytes WriteTo will write.
	Len() int
}

// SessionBuffer wraps a fixed buffer for message building.
// Allocated once per session, reused for all messages.
type SessionBuffer struct {
	buf    []byte
	offset int
}

// NewSessionBuffer creates a buffer for message building.
// If extended is true, allocates 65535 bytes (RFC 8654).
// Otherwise allocates 4096 bytes (RFC 4271).
func NewSessionBuffer(extended bool) *SessionBuffer {
	size := StandardMaxSize
	if extended {
		size = ExtendedMaxSize
	}
	return &SessionBuffer{
		buf: make([]byte, size),
	}
}

// Reset clears the buffer for reuse.
// Does not reallocate - just resets offset to 0.
func (sb *SessionBuffer) Reset() {
	sb.offset = 0
}

// Write writes a BufWriter's content to the buffer.
// Returns number of bytes written.
func (sb *SessionBuffer) Write(w BufWriter) int {
	n := w.WriteTo(sb.buf, sb.offset)
	sb.offset += n
	return n
}

// WriteBytes copies data into the buffer.
// Returns number of bytes written.
func (sb *SessionBuffer) WriteBytes(data []byte) int {
	n := copy(sb.buf[sb.offset:], data)
	sb.offset += n
	return n
}

// CheckedWrite validates capacity before copying data.
// Returns ErrBufferTooSmall if data exceeds remaining capacity.
func (sb *SessionBuffer) CheckedWrite(data []byte) (int, error) {
	if len(data) > sb.Remaining() {
		return 0, ErrBufferTooSmall
	}
	n := copy(sb.buf[sb.offset:], data)
	sb.offset += n
	return n, nil
}

// WriteByte writes a single byte to the buffer.
// Always returns nil - signature matches io.ByteWriter.
func (sb *SessionBuffer) WriteByte(b byte) error {
	sb.buf[sb.offset] = b
	sb.offset++
	return nil
}

// WriteUint16 writes a 16-bit value in network byte order (big-endian).
func (sb *SessionBuffer) WriteUint16(v uint16) {
	binary.BigEndian.PutUint16(sb.buf[sb.offset:], v)
	sb.offset += 2
}

// WriteUint32 writes a 32-bit value in network byte order (big-endian).
func (sb *SessionBuffer) WriteUint32(v uint32) {
	binary.BigEndian.PutUint32(sb.buf[sb.offset:], v)
	sb.offset += 4
}

// PutUint16At writes a 16-bit value at a specific offset without moving the current offset.
// Used for filling in length placeholders after writing variable-length data.
func (sb *SessionBuffer) PutUint16At(pos int, v uint16) {
	binary.BigEndian.PutUint16(sb.buf[pos:], v)
}

// Bytes returns the written portion of the buffer.
func (sb *SessionBuffer) Bytes() []byte {
	return sb.buf[:sb.offset]
}

// Len returns the number of bytes written.
func (sb *SessionBuffer) Len() int {
	return sb.offset
}

// Cap returns the buffer capacity.
func (sb *SessionBuffer) Cap() int {
	return len(sb.buf)
}

// Remaining returns the remaining available space.
func (sb *SessionBuffer) Remaining() int {
	return len(sb.buf) - sb.offset
}

// Offset returns the current write position.
func (sb *SessionBuffer) Offset() int {
	return sb.offset
}

// SetOffset sets the write position.
// Used for skipping space for headers or placeholders.
func (sb *SessionBuffer) SetOffset(off int) {
	sb.offset = off
}

// Resize changes the buffer size if needed.
// If extended is true and current size < 65535, reallocates.
// Preserves existing data.
func (sb *SessionBuffer) Resize(extended bool) {
	targetSize := StandardMaxSize
	if extended {
		targetSize = ExtendedMaxSize
	}

	if len(sb.buf) >= targetSize {
		return // Already large enough
	}

	newBuf := make([]byte, targetSize)
	copy(newBuf, sb.buf[:sb.offset])
	sb.buf = newBuf
}

// Buffer returns the underlying buffer for direct access.
// Use with caution - caller must manage offset manually.
func (sb *SessionBuffer) Buffer() []byte {
	return sb.buf
}
