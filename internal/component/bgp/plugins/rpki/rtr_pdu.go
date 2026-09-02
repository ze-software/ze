// Design: docs/architecture/plugin/rib-storage-design.md — RTR PDU wire format (RFC 8210)
// Overview: rpki.go — plugin entry point consuming RTR data
// Related: rtr_session.go — RTR session using these PDU types
package rpki

import (
	"encoding/binary"
	"fmt"
	"net"
	"slices"
)

// RTR protocol versions.
const (
	rtrVersionMax uint8 = 2 // maximum version we support (v2 for ASPA)
	rtrVersionMin uint8 = 1 // minimum fallback version
)

// PDU type constants (RFC 8210 Section 5, RFC 9582 Section 5.12).
const (
	pduSerialNotify uint8 = 0
	pduSerialQuery  uint8 = 1
	pduResetQuery   uint8 = 2
	pduCacheResp    uint8 = 3
	pduIPv4Prefix   uint8 = 4
	// Type 5 intentionally skipped (not assigned).
	pduIPv6Prefix uint8 = 6
	pduEndOfData  uint8 = 7
	pduCacheReset uint8 = 8
	pduRouterKey  uint8 = 9
	pduErrorRpt   uint8 = 10
	pduASPA       uint8 = 11 // RFC 9582: ASPA PDU (v2 only)
)

// PDU fixed lengths.
const (
	pduHeaderLen      = 8
	pduSerialQueryLen = 12
	pduResetQueryLen  = 8
	pduIPv4PrefixLen  = 20
	pduIPv6PrefixLen  = 32
	pduEndOfDataLen   = 24
	pduASPAFixedLen   = 16 // RFC 9582: header(8) + flags/AFI/zero(4) + customer-AS(4)
	pduASPAMinLen     = 20 // fixed(16) + at least one provider(4)
)

// RTR error codes (RFC 8210 Section 12, RFC 9582 Section 12).
const (
	errNoDataAvail        uint16 = 2
	errUnsupportedVersion uint16 = 4
)

// VRP represents a Validated ROA Payload (prefix + maxLength + origin AS).
type VRP struct {
	Prefix    net.IPNet
	MaxLength uint8
	ASN       uint32
}

// rTRHeader is the common 8-byte PDU header.
type rTRHeader struct {
	Version   uint8
	Type      uint8
	SessionID uint16
	Length    uint32
}

// endOfDataParams holds timing parameters from End of Data PDU.
type endOfDataParams struct {
	SessionID       uint16
	SerialNumber    uint32
	RefreshInterval uint32
	RetryInterval   uint32
	ExpireInterval  uint32
}

// parseHeader reads an 8-byte RTR header from buf.
func parseHeader(buf []byte) (rTRHeader, error) {
	if len(buf) < pduHeaderLen {
		return rTRHeader{}, fmt.Errorf("rtr: header too short: %d bytes", len(buf))
	}
	return rTRHeader{
		Version:   buf[0],
		Type:      buf[1],
		SessionID: binary.BigEndian.Uint16(buf[2:4]),
		Length:    binary.BigEndian.Uint32(buf[4:8]),
	}, nil
}

// writeResetQuery writes a Reset Query PDU into buf at offset off.
// Returns bytes written (always 8).
func writeResetQuery(buf []byte, off int, version uint8) int { //nolint:unparam // off keeps the buffer-first (buf, off) signature every writer here shares
	buf[off] = version
	buf[off+1] = pduResetQuery
	buf[off+2] = 0
	buf[off+3] = 0
	binary.BigEndian.PutUint32(buf[off+4:off+8], pduResetQueryLen)
	return pduResetQueryLen
}

// writeSerialQuery writes a Serial Query PDU into buf at offset off.
// Returns bytes written (always 12).
func writeSerialQuery(buf []byte, off int, version uint8, sessionID uint16, serial uint32) int {
	buf[off] = version
	buf[off+1] = pduSerialQuery
	binary.BigEndian.PutUint16(buf[off+2:off+4], sessionID)
	binary.BigEndian.PutUint32(buf[off+4:off+8], pduSerialQueryLen)
	binary.BigEndian.PutUint32(buf[off+8:off+12], serial)
	return pduSerialQueryLen
}

// parsePrefixPDU parses an IPv4 or IPv6 Prefix PDU.
// ipLen is 4 for IPv4, 16 for IPv6. maxPrefixBits is 32 or 128.
func parsePrefixPDU(buf []byte, minLen, ipLen, maxPrefixBits int) (VRP, bool, error) {
	if len(buf) < minLen {
		return VRP{}, false, fmt.Errorf("rtr: prefix PDU too short: %d < %d", len(buf), minLen)
	}
	flags := buf[8]
	prefixLen := buf[9]
	maxLen := buf[10]

	if int(prefixLen) > maxPrefixBits {
		return VRP{}, false, fmt.Errorf("rtr: prefix length %d > %d", prefixLen, maxPrefixBits)
	}
	if maxLen < prefixLen || int(maxLen) > maxPrefixBits {
		return VRP{}, false, fmt.Errorf("rtr: max length %d invalid (prefix len %d, max %d)", maxLen, prefixLen, maxPrefixBits)
	}

	ip := make(net.IP, ipLen)
	copy(ip, buf[12:12+ipLen])
	asn := binary.BigEndian.Uint32(buf[12+ipLen : 12+ipLen+4])
	announce := flags&1 == 1

	mask := net.CIDRMask(int(prefixLen), maxPrefixBits)
	return VRP{
		Prefix:    net.IPNet{IP: ip.Mask(mask), Mask: mask},
		MaxLength: maxLen,
		ASN:       asn,
	}, announce, nil
}

// parseIPv4Prefix parses an IPv4 Prefix PDU (Type 4) from 20 bytes.
func parseIPv4Prefix(buf []byte) (VRP, bool, error) {
	return parsePrefixPDU(buf, pduIPv4PrefixLen, 4, 32)
}

// parseIPv6Prefix parses an IPv6 Prefix PDU (Type 6) from 32 bytes.
func parseIPv6Prefix(buf []byte) (VRP, bool, error) {
	return parsePrefixPDU(buf, pduIPv6PrefixLen, 16, 128)
}

// parseEndOfData parses an End of Data PDU (Type 7) from 24 bytes.
func parseEndOfData(buf []byte) (endOfDataParams, error) {
	if len(buf) < pduEndOfDataLen {
		return endOfDataParams{}, fmt.Errorf("rtr: End of Data PDU too short: %d", len(buf))
	}
	return endOfDataParams{
		SessionID:       binary.BigEndian.Uint16(buf[2:4]),
		SerialNumber:    binary.BigEndian.Uint32(buf[8:12]),
		RefreshInterval: binary.BigEndian.Uint32(buf[12:16]),
		RetryInterval:   binary.BigEndian.Uint32(buf[16:20]),
		ExpireInterval:  binary.BigEndian.Uint32(buf[20:24]),
	}, nil
}

// isFatalError returns true if the RTR error code is fatal (must drop session).
func isFatalError(code uint16) bool {
	return code != errNoDataAvail && code != errUnsupportedVersion
}

// parseASPAPDU parses an ASPA PDU (Type 11, RFC 9582 Section 5.12).
// Returns the record, whether it's an announce (vs withdraw), and any error.
// A zero CustomerAS in the returned record means "skip this PDU" (unknown AFI).
func parseASPAPDU(buf []byte) (ASPARecord, bool, error) {
	if len(buf) < pduASPAMinLen {
		return ASPARecord{}, false, fmt.Errorf("rtr: ASPA PDU too short: %d < %d", len(buf), pduASPAMinLen)
	}

	flags := buf[8]
	afiFlags := buf[9]
	customerAS := binary.BigEndian.Uint32(buf[12:16])

	announce := flags&1 == 1

	// RFC 9582: router MUST ignore ASPA PDUs with unknown AFI values (>=3).
	if afiFlags >= 3 {
		return ASPARecord{}, false, nil
	}

	// RFC 9582: customer AS 0 and 0xFFFFFFFF are reserved.
	if customerAS == 0 || customerAS == 0xFFFFFFFF {
		return ASPARecord{}, false, fmt.Errorf("rtr: ASPA PDU reserved customer AS: %d", customerAS)
	}

	providerBytes := len(buf) - pduASPAFixedLen
	if providerBytes%4 != 0 {
		return ASPARecord{}, false, fmt.Errorf("rtr: ASPA PDU provider data not aligned: %d bytes", providerBytes)
	}
	provCount := providerBytes / 4

	// RFC 9582 Section 5.12: must have at least one provider.
	if provCount == 0 {
		return ASPARecord{}, false, fmt.Errorf("rtr: ASPA PDU has zero providers")
	}

	providers := make([]uint32, provCount)
	for i := range provCount {
		providers[i] = binary.BigEndian.Uint32(buf[pduASPAFixedLen+i*4 : pduASPAFixedLen+i*4+4])
	}

	// RFC 9582: customer AS MUST NOT appear in its own provider set.
	if slices.Contains(providers, customerAS) {
		return ASPARecord{}, false, fmt.Errorf("rtr: ASPA PDU customer AS %d in own provider set", customerAS)
	}

	// RFC 9582: provider ASNs 0 and 0xFFFFFFFF are reserved.
	for _, p := range providers {
		if p == 0 || p == 0xFFFFFFFF {
			return ASPARecord{}, false, fmt.Errorf("rtr: ASPA PDU reserved provider AS: %d", p)
		}
	}

	// RFC 9582: provider ASNs MUST be sorted ascending (duplicates also rejected).
	for i := 1; i < len(providers); i++ {
		if providers[i] <= providers[i-1] {
			return ASPARecord{}, false, fmt.Errorf("rtr: ASPA PDU providers not sorted ascending (duplicate or out of order at index %d)", i)
		}
	}

	return ASPARecord{
		CustomerAS: customerAS,
		Providers:  providers,
	}, announce, nil
}
