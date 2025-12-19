// Package wire provides zero-copy parsing utilities for BGP wire format.
package wire

import (
	"encoding/binary"
	"io"
)

// Buffer wraps a byte slice for zero-copy parsing of network data.
// All multi-byte reads use big-endian (network byte order).
type Buffer struct {
	data []byte
	pos  int
}

// NewBuffer creates a new buffer wrapping the given data.
// The buffer does not copy the data; it references the original slice.
func NewBuffer(data []byte) *Buffer {
	return &Buffer{data: data, pos: 0}
}

// ReadByte reads and returns a single byte.
// Returns io.EOF if no bytes remain.
func (b *Buffer) ReadByte() (byte, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	v := b.data[b.pos]
	b.pos++
	return v, nil
}

// ReadUint16 reads a big-endian uint16.
// Returns io.EOF if fewer than 2 bytes remain.
func (b *Buffer) ReadUint16() (uint16, error) {
	if b.pos+2 > len(b.data) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint16(b.data[b.pos:])
	b.pos += 2
	return v, nil
}

// ReadUint32 reads a big-endian uint32.
// Returns io.EOF if fewer than 4 bytes remain.
func (b *Buffer) ReadUint32() (uint32, error) {
	if b.pos+4 > len(b.data) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint32(b.data[b.pos:])
	b.pos += 4
	return v, nil
}

// ReadUint64 reads a big-endian uint64.
// Returns io.EOF if fewer than 8 bytes remain.
func (b *Buffer) ReadUint64() (uint64, error) {
	if b.pos+8 > len(b.data) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint64(b.data[b.pos:])
	b.pos += 8
	return v, nil
}

// ReadBytes reads n bytes and returns a slice.
// The returned slice references the original buffer data (zero-copy).
// Returns io.EOF if fewer than n bytes remain.
func (b *Buffer) ReadBytes(n int) ([]byte, error) {
	if b.pos+n > len(b.data) {
		return nil, io.EOF
	}
	v := b.data[b.pos : b.pos+n]
	b.pos += n
	return v, nil
}

// Skip advances the position by n bytes without returning data.
// Returns io.EOF if fewer than n bytes remain.
func (b *Buffer) Skip(n int) error {
	if b.pos+n > len(b.data) {
		return io.EOF
	}
	b.pos += n
	return nil
}

// Peek returns the next byte without advancing the position.
// Returns io.EOF if no bytes remain.
func (b *Buffer) Peek() (byte, error) {
	if b.pos >= len(b.data) {
		return 0, io.EOF
	}
	return b.data[b.pos], nil
}

// Slice reads n bytes and returns them as a new Buffer.
// Useful for parsing nested structures with length prefixes.
// Returns io.EOF if fewer than n bytes remain.
func (b *Buffer) Slice(n int) (*Buffer, error) {
	data, err := b.ReadBytes(n)
	if err != nil {
		return nil, err
	}
	return NewBuffer(data), nil
}

// Remaining returns the number of unread bytes.
func (b *Buffer) Remaining() int {
	return len(b.data) - b.pos
}

// RemainingBytes returns the unread portion of the buffer.
// The returned slice references the original buffer data (zero-copy).
func (b *Buffer) RemainingBytes() []byte {
	return b.data[b.pos:]
}

// Offset returns the current read position.
func (b *Buffer) Offset() int {
	return b.pos
}

// Len returns the total buffer length.
func (b *Buffer) Len() int {
	return len(b.data)
}
