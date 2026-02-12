package cbor

import (
	"errors"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/wire"
)

// Base64 encoding errors.
var (
	ErrBase64InvalidChar    = errors.New("base64: invalid character")
	ErrBase64InvalidPadding = errors.New("base64: invalid padding")
)

// Base64 encoding alphabets.
const (
	base64Std = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	base64URL = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
)

// base64DecodeTable maps ASCII chars to 6-bit values, 0xFF for invalid, 0xFE for padding.
var base64DecodeStd [256]byte
var base64DecodeURL [256]byte

func init() {
	// Initialize all entries to invalid
	for i := range base64DecodeStd {
		base64DecodeStd[i] = 0xFF
		base64DecodeURL[i] = 0xFF
	}

	// Standard alphabet
	for i, c := range base64Std {
		base64DecodeStd[c] = byte(i)
	}
	base64DecodeStd['='] = 0xFE // padding marker

	// URL-safe alphabet
	for i, c := range base64URL {
		base64DecodeURL[c] = byte(i)
	}
	base64DecodeURL['='] = 0xFE // padding marker
}

// Base64Encode encodes data to standard base64 with padding at buf[off:].
// Returns bytes written.
func Base64Encode(buf []byte, off int, data []byte) int {
	return base64EncodeInternal(buf, off, data, base64Std, true)
}

// Base64EncodeURL encodes data to URL-safe base64 with padding at buf[off:].
// Returns bytes written.
func Base64EncodeURL(buf []byte, off int, data []byte) int {
	return base64EncodeInternal(buf, off, data, base64URL, true)
}

// Base64EncodeNoPadding encodes data to standard base64 without padding at buf[off:].
// Returns bytes written.
func Base64EncodeNoPadding(buf []byte, off int, data []byte) int {
	return base64EncodeInternal(buf, off, data, base64Std, false)
}

// Base64EncodeURLNoPadding encodes data to URL-safe base64 without padding.
// Returns bytes written.
func Base64EncodeURLNoPadding(buf []byte, off int, data []byte) int {
	return base64EncodeInternal(buf, off, data, base64URL, false)
}

func base64EncodeInternal(buf []byte, off int, data []byte, alphabet string, pad bool) int {
	if len(data) == 0 {
		return 0
	}

	pos := off
	n := len(data)

	// Process complete 3-byte groups
	for i := 0; i+3 <= n; i += 3 {
		val := uint32(data[i])<<16 | uint32(data[i+1])<<8 | uint32(data[i+2])
		buf[pos] = alphabet[val>>18&0x3F]
		buf[pos+1] = alphabet[val>>12&0x3F]
		buf[pos+2] = alphabet[val>>6&0x3F]
		buf[pos+3] = alphabet[val&0x3F]
		pos += 4
	}

	// Handle remaining bytes
	remain := n % 3
	if remain > 0 {
		var val uint32
		switch remain {
		case 1:
			val = uint32(data[n-1]) << 16
			buf[pos] = alphabet[val>>18&0x3F]
			buf[pos+1] = alphabet[val>>12&0x3F]
			pos += 2
			if pad {
				buf[pos] = '='
				buf[pos+1] = '='
				pos += 2
			}
		case 2:
			val = uint32(data[n-2])<<16 | uint32(data[n-1])<<8
			buf[pos] = alphabet[val>>18&0x3F]
			buf[pos+1] = alphabet[val>>12&0x3F]
			buf[pos+2] = alphabet[val>>6&0x3F]
			pos += 3
			if pad {
				buf[pos] = '='
				pos++
			}
		}
	}

	return pos - off
}

// Base64Decode decodes standard base64 string to binary at buf[off:].
// Returns (bytes_written, error).
func Base64Decode(buf []byte, off int, b64 string) (int, error) {
	return base64DecodeInternal(buf, off, b64, base64DecodeStd)
}

// Base64DecodeURL decodes URL-safe base64 string to binary at buf[off:].
// Returns (bytes_written, error).
func Base64DecodeURL(buf []byte, off int, b64 string) (int, error) {
	return base64DecodeInternal(buf, off, b64, base64DecodeURL)
}

// Base64DecodeNoPadding decodes unpadded base64 string to binary at buf[off:].
// Works with standard alphabet.
// Returns (bytes_written, error).
func Base64DecodeNoPadding(buf []byte, off int, b64 string) (int, error) {
	// Add padding if needed for decoding
	switch len(b64) % 4 {
	case 2:
		b64 += "=="
	case 3:
		b64 += "="
	}
	return base64DecodeInternal(buf, off, b64, base64DecodeStd)
}

// Base64DecodeURLNoPadding decodes unpadded URL-safe base64 to binary.
// Returns (bytes_written, error).
func Base64DecodeURLNoPadding(buf []byte, off int, b64 string) (int, error) {
	switch len(b64) % 4 {
	case 2:
		b64 += "=="
	case 3:
		b64 += "="
	}
	return base64DecodeInternal(buf, off, b64, base64DecodeURL)
}

func base64DecodeInternal(buf []byte, off int, b64 string, table [256]byte) (int, error) {
	if len(b64) == 0 {
		return 0, nil
	}

	// Calculate and validate padding (max 2 allowed)
	padLen := 0
	n := len(b64)
	for i := 1; i <= 4 && i <= n; i++ {
		if b64[n-i] == '=' {
			padLen++
		} else {
			break
		}
	}
	if padLen > 2 {
		return 0, ErrBase64InvalidPadding
	}

	// Re-calculate using simpler logic for actual use
	padLen = 0
	if len(b64) >= 1 && b64[len(b64)-1] == '=' {
		padLen++
		if len(b64) >= 2 && b64[len(b64)-2] == '=' {
			padLen++
		}
	}

	// Validate length (must be multiple of 4 after handling)
	if len(b64)%4 != 0 {
		return 0, ErrBase64InvalidPadding
	}

	pos := off
	n = len(b64)

	// Process all 4-char groups except last (if padded)
	end := n
	if padLen > 0 {
		end = n - 4
	}

	for i := 0; i < end; i += 4 {
		v0 := table[b64[i]]
		v1 := table[b64[i+1]]
		v2 := table[b64[i+2]]
		v3 := table[b64[i+3]]

		if v0 == 0xFF || v1 == 0xFF || v2 == 0xFF || v3 == 0xFF {
			return 0, ErrBase64InvalidChar
		}

		val := uint32(v0)<<18 | uint32(v1)<<12 | uint32(v2)<<6 | uint32(v3)
		buf[pos] = byte(val >> 16)
		buf[pos+1] = byte(val >> 8)
		buf[pos+2] = byte(val)
		pos += 3
	}

	// Handle last group with padding
	if padLen > 0 {
		i := n - 4
		v0 := table[b64[i]]
		v1 := table[b64[i+1]]

		if v0 == 0xFF || v1 == 0xFF {
			return 0, ErrBase64InvalidChar
		}

		if padLen == 2 {
			// Two padding chars: output 1 byte
			val := uint32(v0)<<18 | uint32(v1)<<12
			buf[pos] = byte(val >> 16)
			pos++
		} else {
			// One padding char: output 2 bytes
			v2 := table[b64[i+2]]
			if v2 == 0xFF {
				return 0, ErrBase64InvalidChar
			}
			val := uint32(v0)<<18 | uint32(v1)<<12 | uint32(v2)<<6
			buf[pos] = byte(val >> 16)
			buf[pos+1] = byte(val >> 8)
			pos += 2
		}
	}

	return pos - off, nil
}

// Base64EncodedLen returns the encoded length for data of given length.
// Includes padding.
func Base64EncodedLen(dataLen int) int {
	if dataLen == 0 {
		return 0
	}
	return (dataLen + 2) / 3 * 4
}

// Base64DecodedLen returns the decoded length for a base64 string.
// Accounts for padding.
func Base64DecodedLen(b64 string) int {
	n := len(b64)
	if n == 0 {
		return 0
	}

	padLen := 0
	if n >= 1 && b64[n-1] == '=' {
		padLen++
		if n >= 2 && b64[n-2] == '=' {
			padLen++
		}
	}

	return n/4*3 - padLen
}

// Base64Blob is base64-encodable data.
type Base64Blob []byte

// EncodeBase64 encodes to standard base64 at buf[off:].
// Returns bytes written.
func (b Base64Blob) EncodeBase64(buf []byte, off int) int {
	return Base64Encode(buf, off, b)
}

// EncodeBase64URL encodes to URL-safe base64 at buf[off:].
// Returns bytes written.
func (b Base64Blob) EncodeBase64URL(buf []byte, off int) int {
	return Base64EncodeURL(buf, off, b)
}

// Len returns the raw byte length (not base64-encoded length).
func (b Base64Blob) Len() int { return len(b) }

// WriteTo implements BufWriter by writing raw bytes (not base64).
func (b Base64Blob) WriteTo(buf []byte, off int) int {
	return copy(buf[off:], b)
}

// CheckedWriteTo validates capacity before writing.
func (b Base64Blob) CheckedWriteTo(buf []byte, off int) (int, error) {
	needed := b.Len()
	if len(buf) < off+needed {
		return 0, wire.ErrBufferTooSmall
	}
	return b.WriteTo(buf, off), nil
}
