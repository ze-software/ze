// Design: (none — parked code)
//
// Package reader provides zero-copy parsing utilities for network wire formats.
package reader

import (
	"encoding/binary"
	"io"
)

// Reader wraps a byte slice for zero-copy parsing of network data.
// All multi-byte reads use big-endian (network byte order).
type Reader struct {
	data []byte
	pos  int
}

// NewReader creates a new reader wrapping the given data.
// The reader does not copy the data; it references the original slice.
func NewReader(data []byte) *Reader {
	return &Reader{data: data, pos: 0}
}

// ReadByte reads and returns a single byte.
// Returns io.EOF if no bytes remain.
func (r *Reader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	v := r.data[r.pos]
	r.pos++
	return v, nil
}

// ReadUint16 reads a big-endian uint16.
// Returns io.EOF if fewer than 2 bytes remain.
func (r *Reader) ReadUint16() (uint16, error) {
	if r.pos+2 > len(r.data) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v, nil
}

// ReadUint32 reads a big-endian uint32.
// Returns io.EOF if fewer than 4 bytes remain.
func (r *Reader) ReadUint32() (uint32, error) {
	if r.pos+4 > len(r.data) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, nil
}

// ReadUint64 reads a big-endian uint64.
// Returns io.EOF if fewer than 8 bytes remain.
func (r *Reader) ReadUint64() (uint64, error) {
	if r.pos+8 > len(r.data) {
		return 0, io.EOF
	}
	v := binary.BigEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return v, nil
}

// ReadBytes reads n bytes and returns a slice.
// The returned slice references the original buffer data (zero-copy).
// Returns io.EOF if fewer than n bytes remain.
func (r *Reader) ReadBytes(n int) ([]byte, error) {
	if r.pos+n > len(r.data) {
		return nil, io.EOF
	}
	v := r.data[r.pos : r.pos+n]
	r.pos += n
	return v, nil
}

// Skip advances the position by n bytes without returning data.
// Returns io.EOF if fewer than n bytes remain.
func (r *Reader) Skip(n int) error {
	if r.pos+n > len(r.data) {
		return io.EOF
	}
	r.pos += n
	return nil
}

// Peek returns the next byte without advancing the position.
// Returns io.EOF if no bytes remain.
func (r *Reader) Peek() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	return r.data[r.pos], nil
}

// Slice reads n bytes and returns them as a new Reader.
// Useful for parsing nested structures with length prefixes.
// Returns io.EOF if fewer than n bytes remain.
func (r *Reader) Slice(n int) (*Reader, error) {
	data, err := r.ReadBytes(n)
	if err != nil {
		return nil, err
	}
	return NewReader(data), nil
}

// Remaining returns the number of unread bytes.
func (r *Reader) Remaining() int {
	return len(r.data) - r.pos
}

// RemainingBytes returns the unread portion of the buffer.
// The returned slice references the original buffer data (zero-copy).
func (r *Reader) RemainingBytes() []byte {
	return r.data[r.pos:]
}

// Offset returns the current read position.
func (r *Reader) Offset() int {
	return r.pos
}

// Len returns the total buffer length.
func (r *Reader) Len() int {
	return len(r.data)
}
