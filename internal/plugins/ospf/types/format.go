// Design: docs/architecture/ospf/ospf-1-types.md -- shared dotted-quad format and parse helpers
// Related: routerid.go -- fixed-width Router ID parsing
// Related: areaid.go -- Area ID dotted and integer parsing
// Related: linkstateid.go -- Link State ID parsing

package types

import "errors"

const dottedQuadLen = 15

var (
	// ErrWrongLength reports a fixed-width wire value with the wrong byte count.
	ErrWrongLength = errors.New("ospf types: wrong length")
	// ErrMalformed reports a printable value that is not in the required shape.
	ErrMalformed = errors.New("ospf types: malformed value")
	// ErrOutOfRange reports a syntactically valid value outside the protocol range.
	ErrOutOfRange = errors.New("ospf types: value out of range")
	// ErrReserved reports a value that the RFC reserves and originators must not use.
	ErrReserved = errors.New("ospf types: reserved value")
)

type fixed4 interface {
	~[4]byte
}

func parseFixed4[T fixed4](s string) (T, error) {
	v, err := parseDottedQuad(s)
	if err != nil {
		var zero T
		return zero, err
	}
	return T(v), nil
}

func fixed4FromBytes[T fixed4](b []byte) (T, error) {
	if len(b) != 4 {
		var zero T
		return zero, ErrWrongLength
	}
	var v [4]byte
	copy(v[:], b)
	return T(v), nil
}

func fixed4Bytes[T fixed4](id T) []byte {
	v := [4]byte(id)
	out := make([]byte, len(v))
	copy(out, v[:])
	return out
}

func fixed4WriteTo[T fixed4](id T, buf []byte, off int) int {
	v := [4]byte(id)
	return copy(buf[off:], v[:])
}

func fixed4AppendTo[T fixed4](id T, dst []byte) []byte {
	return appendDottedQuad(dst, [4]byte(id))
}

func fixed4String[T fixed4](id T) string {
	var scratch [dottedQuadLen]byte
	return string(fixed4AppendTo(id, scratch[:0]))
}

func appendDottedQuad(dst []byte, v [4]byte) []byte {
	dst = appendDecimalByte(dst, v[0])
	dst = append(dst, '.')
	dst = appendDecimalByte(dst, v[1])
	dst = append(dst, '.')
	dst = appendDecimalByte(dst, v[2])
	dst = append(dst, '.')
	dst = appendDecimalByte(dst, v[3])
	return dst
}

func appendDecimalByte(dst []byte, v byte) []byte {
	if v >= 100 {
		dst = append(dst, '0'+v/100)
		v %= 100
		return append(dst, '0'+v/10, '0'+v%10)
	}
	if v >= 10 {
		return append(dst, '0'+v/10, '0'+v%10)
	}
	return append(dst, '0'+v)
}

func parseDottedQuad(s string) ([4]byte, error) {
	var out [4]byte
	if s == "" {
		return [4]byte{}, ErrMalformed
	}
	part := 0
	digits := 0
	parts := 0
	leadingZero := false
	for i := range len(s) {
		c := s[i]
		if c == '.' {
			if digits == 0 || parts >= 3 {
				return [4]byte{}, ErrMalformed
			}
			out[parts] = byte(part)
			parts++
			part = 0
			digits = 0
			leadingZero = false
			continue
		}
		if c < '0' || c > '9' {
			return [4]byte{}, ErrMalformed
		}
		if digits == 0 {
			leadingZero = c == '0'
		} else if leadingZero {
			return [4]byte{}, ErrMalformed
		}
		digits++
		if digits > 3 {
			return [4]byte{}, ErrMalformed
		}
		part = part*10 + int(c-'0')
		if part > 255 {
			return [4]byte{}, ErrOutOfRange
		}
	}
	if digits == 0 || parts != 3 {
		return [4]byte{}, ErrMalformed
	}
	out[parts] = byte(part)
	return out, nil
}

func parseUint32Decimal(s string) (uint32, error) {
	if s == "" {
		return 0, ErrMalformed
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, ErrMalformed
	}
	var v uint64
	for i := range len(s) {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, ErrMalformed
		}
		v = v*10 + uint64(c-'0')
		if v > 1<<32-1 {
			return 0, ErrOutOfRange
		}
	}
	return uint32(v), nil
}

func writeUint16(buf []byte, off int, v uint16) int {
	buf[off] = byte(v >> 8)
	buf[off+1] = byte(v)
	return 2
}

func writeUint32(buf []byte, off int, v uint32) int {
	buf[off] = byte(v >> 24)
	buf[off+1] = byte(v >> 16)
	buf[off+2] = byte(v >> 8)
	buf[off+3] = byte(v)
	return 4
}

func compare4(a, b [4]byte) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
