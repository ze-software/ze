// Design: docs/architecture/rsvpte/mpls-rsvp-te.md -- RSVP Path MTU discovery.
// Related: wire.go -- IntServ SENDER_TSPEC and FLOWSPEC codecs.
// Related: build.go -- PATH carries ADSPEC; RESV returns the receiver's M.
package rsvpte

import (
	"encoding/binary"
	"errors"
)

const (
	serviceGeneral        uint8 = 1
	serviceControlledLoad uint8 = 5
	serviceNull           uint8 = 6
	intservBreak          uint8 = 0x80
	adspecSize                  = 48
)

var errIntserv = errors.New("rsvp: malformed IntServ object")

// encodeAdspec writes the RFC 2210 Section 3.3.2 general fragment and the
// Controlled-Load (Section 3.3.4) or Null Service (RFC 2997 Section 4.2) fragment.
// The caller owns buf. Zero MTU emits no advertisement; -1 means invalid input
// or insufficient space. MTU includes the label stack, per RFC 3209 Section 2.6.
//
// Byte  0: | RSVP Length=48 | Class=13 | C-Type=2 |
// Byte  4: | Version=0, reserved       | Words=10 |
// Byte  8: | Service=1 | flags=0       | Words=8  |
// Byte 12: | Param=4   | flags=0       | Words=1  | IS hops at 16
// Byte 20: | Param=6   | flags=0       | Words=1  | Bandwidth at 24
// Byte 28: | Param=8   | flags=0       | Words=1  | Latency at 32
// Byte 36: | Param=10  | flags=0       | Words=1  | Path MTU at 40
// Byte 44: | Service=5 or 6 | flags=0   | Words=0  |
func encodeAdspec(buf []byte, mtu uint32, service uint8) int {
	if mtu == 0 {
		return 0
	}
	if service != serviceControlledLoad && service != serviceNull {
		return -1
	}
	if len(buf) < adspecSize {
		return -1
	}
	clear(buf[:adspecSize])
	encodeObjectHeader(buf, objectHeader{Length: adspecSize, ClassNum: ClassAdspec, CType: 2})
	binary.BigEndian.PutUint16(buf[6:8], 10)
	buf[8] = serviceGeneral
	binary.BigEndian.PutUint16(buf[10:12], 8)
	for off, id := range [...]uint8{4, 6, 8, 10} {
		start := 12 + off*8
		buf[start] = id
		binary.BigEndian.PutUint16(buf[start+2:start+4], 1)
	}
	binary.BigEndian.PutUint32(buf[16:20], 1)
	// RFC 2215 Sections 3.3 and 3.4 define zero bandwidth and all-ones latency
	// as unknown. MTU has no unknown wire value; omit it at the source instead.
	binary.BigEndian.PutUint32(buf[32:36], ^uint32(0))
	binary.BigEndian.PutUint32(buf[40:44], mtu)
	buf[44] = service
	return adspecSize
}

// intservHeader validates the common header inside an RSVP object body.
func intservHeader(body []byte) error {
	if len(body) < 4 || len(body)%4 != 0 {
		return errIntserv
	}
	// RFC 2210 Appendix A1.1: "The message length is measured in 32-bit
	// words *not including the word containing the header*."
	if int(binary.BigEndian.Uint16(body[2:4]))*4 != len(body)-4 {
		return errIntserv
	}
	if body[0]>>4 != 0 {
		return errIntserv
	}
	return nil
}

// intservBlock bounds one service or parameter header and its declared body.
// Both lengths exclude their own four-byte header (RFC 2210 Appendix A1).
func intservBlock(b []byte) (int, error) {
	if len(b) < 4 {
		return 0, errIntserv
	}
	n := 4 + int(binary.BigEndian.Uint16(b[2:4]))*4
	if n > len(b) {
		return 0, errIntserv
	}
	return n, nil
}

// validateAdspec checks nested lengths before any caller modifies an object.
// Unknown service bodies remain opaque, as RFC 2210 Section 3.3 requires.
func validateAdspec(raw []byte) error {
	if len(raw) < objHdrLen+4 {
		return errIntserv
	}
	if int(binary.BigEndian.Uint16(raw[:2])) != len(raw) || raw[2] != ClassAdspec || raw[3] != 2 {
		return errIntserv
	}
	if err := intservHeader(raw[objHdrLen:]); err != nil {
		return err
	}
	previous := uint8(0)
	for off := 8; off < len(raw); {
		n, err := intservBlock(raw[off:])
		if err != nil {
			return err
		}
		service := raw[off]
		// RFC 2210 Section 3.3: "Data fragments must always appear in an
		// ADSPEC in service_number order."
		if service <= previous || (off == 8 && service != serviceGeneral) {
			return errIntserv
		}
		if service == serviceGeneral || service == serviceControlledLoad || service == serviceNull {
			if err := validateAdspecParameters(raw[off : off+n]); err != nil {
				return err
			}
		}
		previous = service
		off += n
	}
	// RFC 2210 Section 3.3.1: "The message header and the default general
	// parameters fragment are always present."
	if previous == 0 {
		return errIntserv
	}
	return nil
}

// validateAdspecParameters checks the general parameters Ze composes, including
// service-specific overrides. Unknown parameters are retained byte for byte.
func validateAdspecParameters(fragment []byte) error {
	var required uint8
	previous := uint8(0)
	for off := 4; off < len(fragment); {
		n, err := intservBlock(fragment[off:])
		if err != nil {
			return err
		}
		id := fragment[off]
		if id <= previous {
			return errIntserv
		}
		switch id {
		case 4, 6, 8, 10:
			if n != 8 {
				return errIntserv
			}
			required |= 1 << ((id - 4) / 2)
			if id == 10 && binary.BigEndian.Uint32(fragment[off+4:off+8]) == 0 {
				return errIntserv
			}
		}
		previous = id
		off += n
	}
	if fragment[0] == serviceGeneral && required != 15 {
		return errIntserv
	}
	return nil
}

// updateAdspec composes one outgoing hop into a validated, owned ADSPEC. The
// caller MUST copy received bytes before this call and MUST start each refresh
// from the received advertisement, so hop counts do not accumulate on refresh.
// A missing local MTU marks the global break bit without inventing a wire MTU.
func updateAdspec(raw []byte, outgoingMTU uint32) error {
	if err := validateAdspec(raw); err != nil {
		return err
	}
	if outgoingMTU == 0 {
		raw[9] |= intservBreak
	}
	for off := 8; off < len(raw); {
		n := 4 + int(binary.BigEndian.Uint16(raw[off+2:off+4]))*4
		switch raw[off] {
		case serviceGeneral, serviceControlledLoad, serviceNull:
			for parameter := off + 4; parameter < off+n; {
				length := 4 + int(binary.BigEndian.Uint16(raw[parameter+2:parameter+4]))*4
				if length == 8 {
					value := binary.BigEndian.Uint32(raw[parameter+4 : parameter+8])
					switch raw[parameter] {
					case 4:
						value = min(value, uint32(254)) + 1
					case 6:
						value = 0
					case 8:
						value = ^uint32(0)
					case 10:
						// RFC 2215 Section 3.5: "The composition rule is to take
						// the minimum of the network element's MTU and the
						// previously composed value."
						if outgoingMTU != 0 {
							value = min(value, outgoingMTU)
						}
					}
					binary.BigEndian.PutUint32(raw[parameter+4:parameter+8], value)
				}
				parameter += length
			}
		default:
			// RFC 2210 Section 3.3: "In all cases, a network element
			// encountering a per-service data header it does not understand
			// simply sets bit 23 to report that the service is not supported,
			// then skips over the rest of the fragment."
			raw[off+1] |= intservBreak
		}
		off += n
	}
	return nil
}

// adspecPathMTU returns the requested service's composed MTU. Zero means absent
// or unreliable, including a missing service fragment or an INVALID parameter.
func adspecPathMTU(raw []byte, service uint8) uint32 {
	if err := validateAdspec(raw); err != nil {
		return 0
	}
	if service == 0 || service == serviceGeneral {
		service = serviceControlledLoad
	}
	if service != serviceControlledLoad && service != serviceNull {
		return 0
	}
	var mtu uint32
	for off := 8; off < len(raw); {
		n := 4 + int(binary.BigEndian.Uint16(raw[off+2:off+4]))*4
		if raw[off] == serviceGeneral || raw[off] == service {
			if raw[off+1]&intservBreak != 0 {
				return 0
			}
			for parameter := off + 4; parameter < off+n; {
				length := 4 + int(binary.BigEndian.Uint16(raw[parameter+2:parameter+4]))*4
				if raw[parameter] == 10 {
					if raw[parameter+1]&intservBreak != 0 {
						return 0
					}
					value := binary.BigEndian.Uint32(raw[parameter+4 : parameter+8])
					if mtu == 0 {
						mtu = value
					} else {
						mtu = min(mtu, value)
					}
				}
				parameter += length
			}
			if raw[off] == service {
				return mtu
			}
		}
		off += n
	}
	return 0
}

// receivedPathMTU interprets a receiver's FLOWSPEC M only when a usable ADSPEC
// was sent on this path. RFC 2210 Sections 2.3.2 and 3.2.1 permit a receiver to
// request a smaller M, so this is a safe negotiated budget, not a measurement of
// the physical path. SENDER_TSPEC M never passes this boundary.
func receivedPathMTU(fs FlowSpec, advertised bool) uint32 {
	if !advertised {
		return 0
	}
	switch fs.Service {
	case serviceControlledLoad, serviceNull:
		return fs.MaxPacketSize
	}
	return 0
}
