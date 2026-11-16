// Design: docs/labs/pppoe-interop.md -- exact-session receive observations.
// Related: product_ipv6_raw_linux.go -- native socket supplies bounded packets.
package testvpppppoe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"
)

var errProductFamilyObservationBudget = errors.New("family reply observation is unknown: packet budget exhausted before its full receive window")

// productFamilyObserve distinguishes a completed healthy window from an
// interrupted observation. Empty reads mean one completed bounded poll, never
// a packet. Exhaustion and transport failures cannot establish reply absence.
func productFamilyObserve(now func() time.Time, deadline time.Time, read func(time.Duration) ([]byte, error), request []byte, budget int) ([]byte, error) {
	if budget <= 0 {
		return nil, errProductFamilyObservationBudget
	}
	if !now().Before(deadline) {
		return nil, errors.New("family reply observation has no future receive window")
	}
	packets := 0
	for {
		remaining := deadline.Sub(now())
		if remaining <= 0 {
			return nil, nil
		}
		if packets >= budget {
			return nil, errProductFamilyObservationBudget
		}
		wait := min(remaining, 200*time.Millisecond)
		frame, err := read(wait)
		if err != nil {
			return nil, err
		}
		if len(frame) == 0 {
			continue
		}
		packets++
		if productFamilyResponse(frame, request) {
			return bytes.Clone(frame), nil
		}
	}
}

// productFamilyResponse admits only the exact observed request's reverse
// Ethernet/PPPoE tuple and complete, internally consistent IP/ICMP envelope.
// Malformed or unrelated frames remain unaccepted until the caller finishes
// its bounded window or reports an explicitly incomplete observation.
func productFamilyResponse(frame, request []byte) bool {
	if len(request) < 26 || len(frame) < 26 {
		return false
	}
	if !bytes.Equal(frame[:6], request[6:12]) || !bytes.Equal(frame[6:12], request[:6]) || binary.BigEndian.Uint16(frame[12:14]) != 0x8864 || frame[14] != 0x11 || frame[15] != 0 || !bytes.Equal(frame[16:18], request[16:18]) || !bytes.Equal(frame[20:22], request[20:22]) {
		return false
	}
	size := int(binary.BigEndian.Uint16(frame[18:20]))
	requestSize := int(binary.BigEndian.Uint16(request[18:20]))
	if size < 6 || size > len(frame)-20 || requestSize < 6 || requestSize > len(request)-20 {
		return false
	}
	p, q := frame[22:20+size], request[22:20+requestSize]
	protocol := binary.BigEndian.Uint16(request[20:22])
	if protocol == 0x8021 || protocol == 0x8057 {
		return len(p) == 4 && bytes.Equal(p, []byte{6, 0x77, 0, 4})
	}
	var offset, requestOffset int
	kind := byte(0)
	switch protocol {
	case 0x21:
		if len(p) < 20 || len(q) < 20 || p[0]>>4 != 4 || q[0]>>4 != 4 {
			return false
		}
		offset, requestOffset = int(p[0]&15)*4, int(q[0]&15)*4
		if offset < 20 || offset > len(p) || requestOffset < 20 || requestOffset > len(q) || int(binary.BigEndian.Uint16(p[2:4])) != len(p) || productFamilyChecksum(p[:offset]) != 0 || p[9] != 1 || binary.BigEndian.Uint16(p[6:8])&0x3fff != 0 {
			return false
		}
		if !bytes.Equal(p[12:16], q[16:20]) || !bytes.Equal(p[16:20], q[12:16]) {
			return false
		}
	case 0x57:
		offset, requestOffset, kind = 40, 40, 129
		if len(p) < 40 || len(q) < 40 || p[0]>>4 != 6 || q[0]>>4 != 6 || int(binary.BigEndian.Uint16(p[4:6])) != len(p)-40 || p[6] != 58 {
			return false
		}
		if !bytes.Equal(p[8:24], q[24:40]) || !bytes.Equal(p[24:40], q[8:24]) {
			return false
		}
	default:
		return false
	}
	if len(q) < requestOffset+8 || len(p) != offset+len(q)-requestOffset || p[offset] != kind || p[offset+1] != 0 || !bytes.Equal(p[offset+4:], q[requestOffset+4:]) {
		return false
	}
	if protocol == 0x21 {
		return productFamilyChecksum(p[offset:]) == 0
	}
	pseudo := append([]byte(nil), p[8:40]...)
	pseudo = binary.BigEndian.AppendUint32(pseudo, uint32(len(p)-offset))
	pseudo = append(pseudo, 0, 0, 0, 58)
	pseudo = append(pseudo, p[offset:]...)
	return productFamilyChecksum(pseudo) == 0
}

func productFamilyChecksum(data []byte) uint16 {
	var sum uint32
	for len(data) >= 2 {
		sum += uint32(binary.BigEndian.Uint16(data[:2]))
		data = data[2:]
	}
	if len(data) == 1 {
		sum += uint32(data[0]) << 8
	}
	for sum>>16 != 0 {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}
