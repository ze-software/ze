package fixture

import (
	"encoding/binary"
)

const (
	tunnelPPPoEEtherType = 0x8863
	tunnelPPPoEPADI      = 0x09
	tunnelPPPoEPADO      = 0x07
	tunnelPPPoEPADR      = 0x19
	tunnelPPPoEPADS      = 0x65
	tunnelPPPoEService   = 0x0101
	tunnelPPPoEACName    = 0x0102
	tunnelPPPoEHostUniq  = 0x0103
	tunnelPPPoEACCookie  = 0x0104
)

func tunnelPPPoETag(attribute uint16, value []byte) []byte {
	result := make([]byte, 4+len(value))
	binary.BigEndian.PutUint16(result[:2], attribute)
	binary.BigEndian.PutUint16(result[2:4], uint16(len(value)))
	copy(result[4:], value)
	return result
}

func tunnelPPPoEPacket(code byte, cookie, hostUniq []byte) []byte {
	tags := tunnelPPPoETag(tunnelPPPoEService, nil)
	tags = append(tags, tunnelPPPoETag(tunnelPPPoEHostUniq, hostUniq)...)
	if cookie != nil {
		tags = append(tags, tunnelPPPoETag(tunnelPPPoEACCookie, cookie)...)
	}
	result := make([]byte, 6+len(tags))
	result[0], result[1] = 0x11, code
	binary.BigEndian.PutUint16(result[4:6], uint16(len(tags)))
	copy(result[6:], tags)
	return result
}

func tunnelPPPoEParseTags(payload []byte) map[uint16][]byte {
	result := make(map[uint16][]byte)
	for offset := 0; offset+4 <= len(payload); {
		attribute := binary.BigEndian.Uint16(payload[offset : offset+2])
		length := int(binary.BigEndian.Uint16(payload[offset+2 : offset+4]))
		if attribute == 0 || offset+4+length > len(payload) {
			break
		}
		result[attribute] = append([]byte(nil), payload[offset+4:offset+4+length]...)
		offset += 4 + length
	}
	return result
}
