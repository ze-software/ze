// Design: (none -- research/analysis tool)
//
// Shared MRT parsing helpers for ze-analyse subcommands.
// Provides constants, file opening, record iteration, and wire format helpers.
package main

import (
	"compress/bzip2"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
)

// MRT types (RFC 6396).
const (
	mrtTableDumpV2 = 13
	mrtBGP4MP      = 16
	mrtBGP4MPET    = 17
)

// TABLE_DUMP_V2 subtypes.
const (
	subtypePeerIndexTable   = 1
	subtypeRIBIPv4Unicast   = 2
	subtypeRIBIPv4Multicast = 3
	subtypeRIBIPv6Unicast   = 4
	subtypeRIBIPv6Multicast = 5
	subtypeRIBGeneric       = 6
)

// BGP4MP subtypes.
const (
	subtypeBGP4MPMessage         = 1
	subtypeBGP4MPMessageAS4      = 4
	subtypeBGP4MPMessageLocal    = 6
	subtypeBGP4MPMessageAS4Local = 7
)

// BGP path attribute type codes.
const (
	attrOrigin          = 1
	attrASPath          = 2
	attrNextHop         = 3
	attrMED             = 4
	attrLocalPref       = 5
	attrAtomicAggregate = 6
	attrAggregator      = 7
	attrCommunity       = 8
	attrOriginatorID    = 9
	attrClusterList     = 10
	attrMPReachNLRI     = 14
	attrMPUnreachNLRI   = 15
	attrExtCommunity    = 16
	attrAS4Path         = 17
	attrAS4Aggregator   = 18
	attrLargeCommunity  = 32
	attrOTC             = 35 // RFC 9234 Only to Customer.
)

// mrtPeerInfo holds peer info from PEER_INDEX_TABLE.
type mrtPeerInfo struct {
	Index  uint16
	IP     net.IP
	BGPID  net.IP
	ASN    uint32
	IsIPv6 bool
	IsAS4  bool
}

// mrtHandler receives MRT records by category.
type mrtHandler struct {
	OnPeerIndex func(data []byte)
	OnRIB       func(data []byte, subtype uint16)
	OnBGP4MP    func(data []byte, subtype uint16, ts uint32)
}

// processMRTFile opens a file, reads all MRT records, and dispatches to handler.
func processMRTFile(filename string, h mrtHandler) error {
	f, err := os.Open(filename) //nolint:gosec // CLI tool intentionally opens user-provided files
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // best-effort close on read-only file

	var r io.Reader
	switch {
	case strings.HasSuffix(filename, ".gz"):
		gz, err := gzip.NewReader(f)
		if err != nil {
			return fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close() //nolint:errcheck // best-effort close on decompressor
		r = gz
	case strings.HasSuffix(filename, ".bz2"):
		r = bzip2.NewReader(f)
	default:
		r = f
	}

	return readMRTRecords(r, h)
}

// readMRTRecords reads all MRT records from a reader and dispatches to handler.
func readMRTRecords(r io.Reader, h mrtHandler) error {
	header := make([]byte, 12)
	for {
		_, err := io.ReadFull(r, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading header: %w", err)
		}

		ts := binary.BigEndian.Uint32(header[0:4])
		mrtType := binary.BigEndian.Uint16(header[4:6])
		subtype := binary.BigEndian.Uint16(header[6:8])
		length := binary.BigEndian.Uint32(header[8:12])

		// Cap record size to prevent OOM from malicious/corrupt MRT files.
		// Legitimate MRT records are well under 1 MB (BGP max is 65535 bytes).
		const maxMRTRecord = 16 * 1024 * 1024 // 16 MB hard cap
		if length > maxMRTRecord {
			return fmt.Errorf("MRT record length %d exceeds %d byte cap; file may be malformed", length, maxMRTRecord)
		}
		data := make([]byte, length)
		_, err = io.ReadFull(r, data)
		if err != nil {
			return fmt.Errorf("reading data: %w", err)
		}

		switch mrtType {
		case mrtTableDumpV2:
			switch subtype {
			case subtypePeerIndexTable:
				if h.OnPeerIndex != nil {
					h.OnPeerIndex(data)
				}
			case subtypeRIBIPv4Unicast, subtypeRIBIPv4Multicast,
				subtypeRIBIPv6Unicast, subtypeRIBIPv6Multicast,
				subtypeRIBGeneric:
				if h.OnRIB != nil {
					h.OnRIB(data, subtype)
				}
			}
		case mrtBGP4MP, mrtBGP4MPET:
			if h.OnBGP4MP != nil {
				offset := 0
				if mrtType == mrtBGP4MPET {
					if len(data) < 4 {
						continue
					}
					offset = 4 // skip microseconds
				}
				h.OnBGP4MP(data[offset:], subtype, ts)
			}
		}
	}
	return nil
}

// extractBGP4MPUpdate extracts the UPDATE body and peer ASN from a BGP4MP record.
// Returns nil body if the record is not an UPDATE message.
func extractBGP4MPUpdate(subtype uint16, data []byte) (body []byte, peerASN uint32) {
	var asSize int
	switch subtype {
	case subtypeBGP4MPMessage, subtypeBGP4MPMessageLocal:
		asSize = 2
	case subtypeBGP4MPMessageAS4, subtypeBGP4MPMessageAS4Local:
		asSize = 4
	default:
		return nil, 0
	}

	minLen := asSize*2 + 4
	if len(data) < minLen {
		return nil, 0
	}

	// Extract peer ASN.
	if asSize == 4 {
		peerASN = binary.BigEndian.Uint32(data[0:4])
	} else {
		peerASN = uint32(binary.BigEndian.Uint16(data[0:2]))
	}

	afi := binary.BigEndian.Uint16(data[asSize*2+2 : asSize*2+4])
	offset := minLen

	// peer_ip + local_ip.
	ipSize := 4
	if afi == 2 {
		ipSize = 16
	}
	offset += ipSize * 2

	if offset+19 > len(data) {
		return nil, 0
	}

	// BGP message: marker(16) + length(2) + type(1) + body.
	offset += 16
	msgLen := binary.BigEndian.Uint16(data[offset : offset+2])
	msgType := data[offset+2]
	offset += 3

	if msgType != 2 { // Only UPDATE.
		return nil, 0
	}

	bodyLen := int(msgLen) - 19
	if bodyLen < 4 || offset+bodyLen > len(data) {
		return nil, 0
	}

	return data[offset : offset+bodyLen], peerASN
}

// extractUpdateAttrs returns the path attributes section from an UPDATE body.
func extractUpdateAttrs(update []byte) []byte {
	if len(update) < 4 {
		return nil
	}

	wdLen := binary.BigEndian.Uint16(update[0:2])
	offset := 2 + int(wdLen)
	if offset+2 > len(update) {
		return nil
	}

	attrLen := binary.BigEndian.Uint16(update[offset : offset+2])
	offset += 2
	if offset+int(attrLen) > len(update) {
		return nil
	}

	return update[offset : offset+int(attrLen)]
}

// iterateAttrs calls fn for each attribute in a packed attribute section.
// fn receives flags, type code, and the attribute value bytes.
func iterateAttrs(attrs []byte, fn func(flags, typeCode uint8, value []byte)) {
	off := 0
	for off < len(attrs) {
		if off+2 > len(attrs) {
			break
		}
		flags := attrs[off]
		typeCode := attrs[off+1]
		off += 2

		var attrLen int
		if flags&0x10 != 0 { // extended length
			if off+2 > len(attrs) {
				break
			}
			attrLen = int(binary.BigEndian.Uint16(attrs[off : off+2]))
			off += 2
		} else {
			if off >= len(attrs) {
				break
			}
			attrLen = int(attrs[off])
			off++
		}

		if off+attrLen > len(attrs) {
			break
		}

		fn(flags, typeCode, attrs[off:off+attrLen])
		off += attrLen
	}
}

// countAttrs counts the number of attributes in a packed attribute section.
func countAttrs(attrs []byte) int {
	count := 0
	iterateAttrs(attrs, func(_, _ uint8, _ []byte) {
		count++
	})
	return count
}

// countPackedPrefixes counts prefix entries in a packed NLRI field.
// Format: repeated [prefix_len(1) + prefix_bytes(ceil(prefix_len/8))].
func countPackedPrefixes(data []byte) int {
	count := 0
	off := 0
	for off < len(data) {
		prefixLen := int(data[off])
		off++
		off += (prefixLen + 7) / 8
		if off > len(data) {
			break
		}
		count++
	}
	return count
}

// parsePeerIndexTable parses a TABLE_DUMP_V2 PEER_INDEX_TABLE record.
func parsePeerIndexTable(data []byte) map[uint16]*mrtPeerInfo {
	peers := make(map[uint16]*mrtPeerInfo)
	if len(data) < 6 {
		return peers
	}

	// Collector BGP ID (4 bytes).
	offset := 4

	// View Name Length (2 bytes) + View Name.
	if offset+2 > len(data) {
		return peers
	}
	viewNameLen := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2 + int(viewNameLen)

	// Peer Count (2 bytes).
	if offset+2 > len(data) {
		return peers
	}
	peerCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	for i := range peerCount {
		if offset+1 > len(data) {
			break
		}

		peerType := data[offset]
		offset++

		isIPv6 := (peerType & 0x01) != 0
		isAS4 := (peerType & 0x02) != 0

		// Peer BGP ID (4 bytes).
		if offset+4 > len(data) {
			break
		}
		bgpID := make(net.IP, 4)
		copy(bgpID, data[offset:offset+4])
		offset += 4

		// Peer IP Address (4 or 16 bytes).
		ipLen := 4
		if isIPv6 {
			ipLen = 16
		}
		if offset+ipLen > len(data) {
			break
		}
		peerIP := make(net.IP, ipLen)
		copy(peerIP, data[offset:offset+ipLen])
		offset += ipLen

		// Peer AS (2 or 4 bytes).
		var asn uint32
		if isAS4 {
			if offset+4 > len(data) {
				break
			}
			asn = binary.BigEndian.Uint32(data[offset : offset+4])
			offset += 4
		} else {
			if offset+2 > len(data) {
				break
			}
			asn = uint32(binary.BigEndian.Uint16(data[offset : offset+2]))
			offset += 2
		}

		peers[i] = &mrtPeerInfo{
			Index:  i,
			IP:     peerIP,
			BGPID:  bgpID,
			ASN:    asn,
			IsIPv6: isIPv6,
			IsAS4:  isAS4,
		}
	}

	return peers
}

// forEachRIBEntry calls fn for each RIB entry in a TABLE_DUMP_V2 RIB record.
// fn receives peer_index and the packed attributes.
func forEachRIBEntry(data []byte, subtype uint16, fn func(peerIndex uint16, attrs []byte)) {
	if len(data) < 4 {
		return
	}

	offset := 4 // skip sequence number

	// RIB_GENERIC has AFI/SAFI before prefix.
	if subtype == subtypeRIBGeneric {
		if offset+3 > len(data) {
			return
		}
		offset += 3 // AFI (2) + SAFI (1)
	}

	// Skip prefix.
	if offset >= len(data) {
		return
	}
	prefixLen := int(data[offset])
	offset++
	offset += (prefixLen + 7) / 8

	// Entry count.
	if offset+2 > len(data) {
		return
	}
	entryCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	for range entryCount {
		if offset+8 > len(data) {
			break
		}

		peerIndex := binary.BigEndian.Uint16(data[offset : offset+2])
		// skip originated_time (4 bytes)
		attrLen := int(binary.BigEndian.Uint16(data[offset+6 : offset+8]))
		offset += 8

		if offset+attrLen > len(data) {
			break
		}

		fn(peerIndex, data[offset:offset+attrLen])
		offset += attrLen
	}
}

// getRIBPrefix extracts the prefix bytes and length from a RIB record.
// Returns nlri (prefix_len + prefix_bytes) for building UPDATE messages.
func getRIBPrefix(data []byte) []byte {
	if len(data) < 5 {
		return nil
	}
	prefixLen := data[4]
	prefixBytes := (int(prefixLen) + 7) / 8
	if 5+prefixBytes > len(data) {
		return nil
	}
	nlri := make([]byte, 1+prefixBytes)
	nlri[0] = prefixLen
	copy(nlri[1:], data[5:5+prefixBytes])
	return nlri
}

// buildUpdate constructs a BGP UPDATE body from components.
func buildUpdate(withdrawn, attrs, nlri []byte) []byte {
	wdLen := len(withdrawn)
	attrLen := len(attrs)

	update := make([]byte, 2+wdLen+2+attrLen+len(nlri))

	binary.BigEndian.PutUint16(update[0:2], uint16(wdLen)) //nolint:gosec // wdLen < 4096
	copy(update[2:], withdrawn)

	binary.BigEndian.PutUint16(update[2+wdLen:], uint16(attrLen)) //nolint:gosec // attrLen < 4096
	copy(update[4+wdLen:], attrs)
	copy(update[4+wdLen+attrLen:], nlri)

	return update
}

// formatBytes formats a byte count for human display.
func formatBytes(b uint64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// wf writes formatted output to a writer, discarding errors.
// Used for human-readable summary output where write failures are not actionable.
func wf(w io.Writer, format string, args ...any) {
	if _, err := fmt.Fprintf(w, format, args...); err != nil {
		return
	}
}

// isAllDigits returns true if s is non-empty and contains only ASCII digits.
func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// formatNumber formats a number with comma separators.
func formatNumber(n uint64) string {
	s := fmt.Sprintf("%d", n)
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
