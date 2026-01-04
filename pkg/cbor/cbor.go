// Package cbor provides zero-allocation CBOR encoding/decoding with WriteTo pattern.
//
// RFC 8949 - Concise Binary Object Representation (CBOR)
//
// This package follows the BufWriter pattern from pkg/bgp/wire for
// zero-allocation encoding into pre-allocated buffers.
package cbor

import (
	"encoding/binary"
	"errors"
)

// CBOR major types (3 high bits of initial byte).
const (
	MajorUnsigned = 0 << 5 // Major type 0: unsigned integer
	MajorNegative = 1 << 5 // Major type 1: negative integer
	MajorBytes    = 2 << 5 // Major type 2: byte string
	MajorText     = 3 << 5 // Major type 3: text string (UTF-8)
	MajorArray    = 4 << 5 // Major type 4: array
	MajorMap      = 5 << 5 // Major type 5: map
	MajorTag      = 6 << 5 // Major type 6: semantic tag
	MajorSimple   = 7 << 5 // Major type 7: simple values and floats
)

// Additional info values (5 low bits of initial byte).
const (
	InfoDirect   = 23 // Values 0-23 encoded directly
	InfoOneByte  = 24 // Following 1 byte
	InfoTwoByte  = 25 // Following 2 bytes
	InfoFourByte = 26 // Following 4 bytes
	InfoEight    = 27 // Following 8 bytes
)

// Simple values (major type 7).
const (
	SimpleFalse = 20
	SimpleTrue  = 21
	SimpleNull  = 22
	SimpleUndef = 23
)

// Errors.
var (
	ErrShortBuffer   = errors.New("cbor: buffer too short")
	ErrInvalidType   = errors.New("cbor: invalid type for operation")
	ErrInvalidInfo   = errors.New("cbor: invalid additional info")
	ErrOverflow      = errors.New("cbor: integer overflow")
	ErrInvalidString = errors.New("cbor: invalid string encoding")
)

// Blob is raw CBOR bytes that implements BufWriter.
type Blob []byte

// WriteTo writes the raw CBOR bytes into buf at offset.
// Returns number of bytes written.
func (b Blob) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], b)
}

// Len returns the length of the CBOR blob.
func (b Blob) Len() int {
	return len(b)
}

// encodeHead writes CBOR type header (major type + additional info + value).
// Returns bytes written.
func encodeHead(buf []byte, off int, major byte, val uint64) int {
	if val <= InfoDirect {
		buf[off] = major | byte(val)
		return 1
	}
	if val <= 0xFF {
		buf[off] = major | InfoOneByte
		buf[off+1] = byte(val)
		return 2
	}
	if val <= 0xFFFF {
		buf[off] = major | InfoTwoByte
		binary.BigEndian.PutUint16(buf[off+1:], uint16(val))
		return 3
	}
	if val <= 0xFFFFFFFF {
		buf[off] = major | InfoFourByte
		binary.BigEndian.PutUint32(buf[off+1:], uint32(val))
		return 5
	}
	buf[off] = major | InfoEight
	binary.BigEndian.PutUint64(buf[off+1:], val)
	return 9
}

// EncodeUint encodes an unsigned integer (major type 0).
// Returns bytes written.
func EncodeUint(buf []byte, off int, val uint64) int {
	return encodeHead(buf, off, MajorUnsigned, val)
}

// EncodeInt encodes a signed integer (major type 0 or 1).
// Positive values use type 0, negative use type 1.
// Returns bytes written.
func EncodeInt(buf []byte, off int, val int64) int {
	if val >= 0 {
		return encodeHead(buf, off, MajorUnsigned, uint64(val))
	}
	// CBOR negative: -1 encodes as 0, -2 as 1, etc.
	// #nosec G115 -- val is negative, so -1-val is positive and fits in uint64
	return encodeHead(buf, off, MajorNegative, uint64(-1-val))
}

// EncodeBytes encodes a byte string (major type 2).
// Returns bytes written.
func EncodeBytes(buf []byte, off int, data []byte) int {
	n := encodeHead(buf, off, MajorBytes, uint64(len(data)))
	n += copy(buf[off+n:], data)
	return n
}

// EncodeString encodes a text string (major type 3).
// Returns bytes written.
func EncodeString(buf []byte, off int, s string) int {
	n := encodeHead(buf, off, MajorText, uint64(len(s)))
	n += copy(buf[off+n:], s)
	return n
}

// EncodeArrayHeader encodes an array header (major type 4).
// Caller must encode 'count' elements after this.
// Returns bytes written.
func EncodeArrayHeader(buf []byte, off int, count int) int {
	// #nosec G115 -- count is expected to be non-negative
	return encodeHead(buf, off, MajorArray, uint64(count))
}

// EncodeMapHeader encodes a map header (major type 5).
// Caller must encode 'count' key-value pairs after this.
// Returns bytes written.
func EncodeMapHeader(buf []byte, off int, count int) int {
	// #nosec G115 -- count is expected to be non-negative
	return encodeHead(buf, off, MajorMap, uint64(count))
}

// EncodeTag encodes a semantic tag (major type 6).
// Caller must encode the tagged value after this.
// Returns bytes written.
func EncodeTag(buf []byte, off int, tag uint64) int {
	return encodeHead(buf, off, MajorTag, tag)
}

// EncodeBool encodes a boolean (major type 7, simple values 20/21).
// Returns bytes written.
func EncodeBool(buf []byte, off int, val bool) int {
	if val {
		buf[off] = MajorSimple | SimpleTrue
	} else {
		buf[off] = MajorSimple | SimpleFalse
	}
	return 1
}

// EncodeNull encodes null (major type 7, simple value 22).
// Returns bytes written.
func EncodeNull(buf []byte, off int) int {
	buf[off] = MajorSimple | SimpleNull
	return 1
}

// decodeHead reads CBOR type header and returns (major, value, bytes_consumed, error).
func decodeHead(data []byte, off int) (major byte, val uint64, n int, err error) {
	if off >= len(data) {
		return 0, 0, 0, ErrShortBuffer
	}

	initial := data[off]
	major = initial & 0xE0
	info := initial & 0x1F

	if info <= InfoDirect {
		return major, uint64(info), 1, nil
	}

	switch info {
	case InfoOneByte:
		if off+2 > len(data) {
			return 0, 0, 0, ErrShortBuffer
		}
		return major, uint64(data[off+1]), 2, nil

	case InfoTwoByte:
		if off+3 > len(data) {
			return 0, 0, 0, ErrShortBuffer
		}
		return major, uint64(binary.BigEndian.Uint16(data[off+1:])), 3, nil

	case InfoFourByte:
		if off+5 > len(data) {
			return 0, 0, 0, ErrShortBuffer
		}
		return major, uint64(binary.BigEndian.Uint32(data[off+1:])), 5, nil

	case InfoEight:
		if off+9 > len(data) {
			return 0, 0, 0, ErrShortBuffer
		}
		return major, binary.BigEndian.Uint64(data[off+1:]), 9, nil

	default:
		return 0, 0, 0, ErrInvalidInfo
	}
}

// DecodeUint decodes an unsigned integer (major type 0).
// Returns (value, bytes_consumed, error).
func DecodeUint(data []byte, off int) (uint64, int, error) {
	major, val, n, err := decodeHead(data, off)
	if err != nil {
		return 0, 0, err
	}
	if major != MajorUnsigned {
		return 0, 0, ErrInvalidType
	}
	return val, n, nil
}

// DecodeInt decodes a signed integer (major type 0 or 1).
// Returns (value, bytes_consumed, error).
func DecodeInt(data []byte, off int) (int64, int, error) {
	major, val, n, err := decodeHead(data, off)
	if err != nil {
		return 0, 0, err
	}

	switch major {
	case MajorUnsigned:
		if val > 0x7FFFFFFFFFFFFFFF {
			return 0, 0, ErrOverflow
		}
		return int64(val), n, nil

	case MajorNegative:
		// CBOR negative: value 0 = -1, value 1 = -2, etc.
		if val > 0x7FFFFFFFFFFFFFFF {
			return 0, 0, ErrOverflow
		}
		return -1 - int64(val), n, nil

	default:
		return 0, 0, ErrInvalidType
	}
}

// DecodeBytes decodes a byte string (major type 2).
// Returns (data, bytes_consumed, error).
// Note: Returns a slice into the original buffer (zero-copy).
func DecodeBytes(data []byte, off int) ([]byte, int, error) {
	major, length, n, err := decodeHead(data, off)
	if err != nil {
		return nil, 0, err
	}
	if major != MajorBytes {
		return nil, 0, ErrInvalidType
	}
	// #nosec G115 -- length validated against buffer size
	dataLen := int(length)
	if off+n+dataLen > len(data) {
		return nil, 0, ErrShortBuffer
	}
	return data[off+n : off+n+dataLen], n + dataLen, nil
}

// DecodeString decodes a text string (major type 3).
// Returns (string, bytes_consumed, error).
func DecodeString(data []byte, off int) (string, int, error) {
	major, length, n, err := decodeHead(data, off)
	if err != nil {
		return "", 0, err
	}
	if major != MajorText {
		return "", 0, ErrInvalidType
	}
	// #nosec G115 -- length validated against buffer size
	strLen := int(length)
	if off+n+strLen > len(data) {
		return "", 0, ErrShortBuffer
	}
	return string(data[off+n : off+n+strLen]), n + strLen, nil
}

// DecodeArrayHeader decodes an array header (major type 4).
// Returns (element_count, bytes_consumed, error).
// Caller must decode 'count' elements after this.
func DecodeArrayHeader(data []byte, off int) (int, int, error) {
	major, val, n, err := decodeHead(data, off)
	if err != nil {
		return 0, 0, err
	}
	if major != MajorArray {
		return 0, 0, ErrInvalidType
	}
	// #nosec G115 -- array count fits in int for practical use
	return int(val), n, nil
}

// DecodeMapHeader decodes a map header (major type 5).
// Returns (pair_count, bytes_consumed, error).
// Caller must decode 'count' key-value pairs after this.
func DecodeMapHeader(data []byte, off int) (int, int, error) {
	major, val, n, err := decodeHead(data, off)
	if err != nil {
		return 0, 0, err
	}
	if major != MajorMap {
		return 0, 0, ErrInvalidType
	}
	// #nosec G115 -- map count fits in int for practical use
	return int(val), n, nil
}

// DecodeTag decodes a semantic tag (major type 6).
// Returns (tag_value, bytes_consumed, error).
// Caller must decode the tagged value after this.
func DecodeTag(data []byte, off int) (uint64, int, error) {
	major, val, n, err := decodeHead(data, off)
	if err != nil {
		return 0, 0, err
	}
	if major != MajorTag {
		return 0, 0, ErrInvalidType
	}
	return val, n, nil
}

// DecodeBool decodes a boolean (major type 7, simple values 20/21).
// Returns (value, bytes_consumed, error).
func DecodeBool(data []byte, off int) (bool, int, error) {
	if off >= len(data) {
		return false, 0, ErrShortBuffer
	}
	switch data[off] {
	case MajorSimple | SimpleFalse:
		return false, 1, nil
	case MajorSimple | SimpleTrue:
		return true, 1, nil
	default:
		return false, 0, ErrInvalidType
	}
}

// DecodeNull checks for null (major type 7, simple value 22).
// Returns (bytes_consumed, error). Error if not null.
func DecodeNull(data []byte, off int) (int, error) {
	if off >= len(data) {
		return 0, ErrShortBuffer
	}
	if data[off] != MajorSimple|SimpleNull {
		return 0, ErrInvalidType
	}
	return 1, nil
}

// PeekType returns the major type of the next CBOR item without consuming it.
// Returns (major_type, error).
func PeekType(data []byte, off int) (byte, error) {
	if off >= len(data) {
		return 0, ErrShortBuffer
	}
	return data[off] & 0xE0, nil
}

// Encoder provides a fluent API for building CBOR.
type Encoder struct {
	buf    []byte
	offset int
}

// NewEncoder creates an encoder writing to buf starting at offset.
func NewEncoder(buf []byte, offset int) *Encoder {
	return &Encoder{buf: buf, offset: offset}
}

// Uint encodes an unsigned integer.
func (e *Encoder) Uint(val uint64) *Encoder {
	e.offset += EncodeUint(e.buf, e.offset, val)
	return e
}

// Int encodes a signed integer.
func (e *Encoder) Int(val int64) *Encoder {
	e.offset += EncodeInt(e.buf, e.offset, val)
	return e
}

// Bytes encodes a byte string.
func (e *Encoder) Bytes(data []byte) *Encoder {
	e.offset += EncodeBytes(e.buf, e.offset, data)
	return e
}

// String encodes a text string.
func (e *Encoder) String(s string) *Encoder {
	e.offset += EncodeString(e.buf, e.offset, s)
	return e
}

// ArrayHeader encodes an array header.
func (e *Encoder) ArrayHeader(count int) *Encoder {
	e.offset += EncodeArrayHeader(e.buf, e.offset, count)
	return e
}

// MapHeader encodes a map header.
func (e *Encoder) MapHeader(count int) *Encoder {
	e.offset += EncodeMapHeader(e.buf, e.offset, count)
	return e
}

// Tag encodes a semantic tag.
func (e *Encoder) Tag(tag uint64) *Encoder {
	e.offset += EncodeTag(e.buf, e.offset, tag)
	return e
}

// Bool encodes a boolean.
func (e *Encoder) Bool(val bool) *Encoder {
	e.offset += EncodeBool(e.buf, e.offset, val)
	return e
}

// Null encodes null.
func (e *Encoder) Null() *Encoder {
	e.offset += EncodeNull(e.buf, e.offset)
	return e
}

// Raw writes raw CBOR bytes.
func (e *Encoder) Raw(data []byte) *Encoder {
	e.offset += copy(e.buf[e.offset:], data)
	return e
}

// Len returns bytes written so far.
func (e *Encoder) Len() int {
	return e.offset
}

// Offset returns current write position.
func (e *Encoder) Offset() int {
	return e.offset
}
