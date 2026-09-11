package nlrisplit

import "encoding/binary"

// keyVPLS implements RFC 4761 Section 3.5 equivalence: RD, VE ID and VE Block
// Offset. Neither VE Block Size nor Label Base participates in path selection.
func keyVPLS(raw, _ []byte, _ bool) ([]byte, error) {
	if len(raw) < 19 || int(binary.BigEndian.Uint16(raw[:2])) != len(raw)-2 {
		return nil, errPrefixKey
	}
	return raw[2:14], nil
}
