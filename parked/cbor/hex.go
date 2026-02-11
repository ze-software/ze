package cbor

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/plugin/bgp/wire"
)

// Hex encoding errors.
var (
	ErrHexOddLength   = errors.New("hex: odd length string")
	ErrHexInvalidChar = errors.New("hex: invalid character")
)

// Hex encoding tables.
const (
	hexLower = "0123456789abcdef"
	hexUpper = "0123456789ABCDEF"
)

// hexDecodeTable maps ASCII chars to nibble values (0-15), 0xFF for invalid.
var hexDecodeTable = [256]byte{
	'0': 0, '1': 1, '2': 2, '3': 3, '4': 4,
	'5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
	'a': 10, 'b': 11, 'c': 12, 'd': 13, 'e': 14, 'f': 15,
	'A': 10, 'B': 11, 'C': 12, 'D': 13, 'E': 14, 'F': 15,
}

func init() {
	// Initialize invalid entries to 0xFF
	for i := range hexDecodeTable {
		if hexDecodeTable[i] == 0 && i != '0' {
			hexDecodeTable[i] = 0xFF
		}
	}
}

// HexEncode encodes binary data to lowercase hex at buf[off:].
// Returns bytes written.
func HexEncode(buf []byte, off int, data []byte) int {
	for i, b := range data {
		buf[off+i*2] = hexLower[b>>4]
		buf[off+i*2+1] = hexLower[b&0x0F]
	}
	return len(data) * 2
}

// HexEncodeUpper encodes binary data to uppercase hex at buf[off:].
// Returns bytes written.
func HexEncodeUpper(buf []byte, off int, data []byte) int {
	for i, b := range data {
		buf[off+i*2] = hexUpper[b>>4]
		buf[off+i*2+1] = hexUpper[b&0x0F]
	}
	return len(data) * 2
}

// HexDecode decodes hex string to binary at buf[off:].
// Returns (bytes_written, error).
func HexDecode(buf []byte, off int, hex string) (int, error) {
	if len(hex)%2 != 0 {
		return 0, ErrHexOddLength
	}

	n := len(hex) / 2
	for i := range n {
		hi := hexDecodeTable[hex[i*2]]
		lo := hexDecodeTable[hex[i*2+1]]
		if hi == 0xFF || lo == 0xFF {
			return 0, ErrHexInvalidChar
		}
		// #nosec G602 -- caller guarantees buf has sufficient capacity
		buf[off+i] = hi<<4 | lo
	}
	return n, nil
}

// HexEncodedLen returns the encoded length for data of given length.
func HexEncodedLen(dataLen int) int {
	return dataLen * 2
}

// HexDecodedLen returns the decoded length for hex string of given length.
func HexDecodedLen(hexLen int) int {
	return hexLen / 2
}

// HexBlob is hex-encoded data that can encode/decode.
type HexBlob []byte

// EncodeHex encodes the blob to hex at buf[off:].
// Returns bytes written.
func (h HexBlob) EncodeHex(buf []byte, off int) int {
	return HexEncode(buf, off, h)
}

// Len returns the raw byte length (not hex-encoded length).
func (h HexBlob) Len() int { return len(h) }

// WriteTo implements BufWriter by writing raw bytes (not hex).
func (h HexBlob) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], h)
}

// CheckedWriteTo validates capacity before writing.
func (h HexBlob) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := h.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return h.WriteTo(buf, off), nil
}
