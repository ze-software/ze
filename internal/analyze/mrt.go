// Design: (none -- research/analysis tool)
// RFC: rfc/short/rfc6396.md -- MRT record layout; RFC 8050 add-path subtypes
//
// Shared MRT parsing helpers for ze-analyze subcommands.
// Provides constants, file opening, record iteration, and wire format helpers.
// Wire decoding itself is delegated to internal/mrt; this file only adapts it
// to the callback shape the subcommands use.
package analyze

import (
	"compress/bzip2"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

// MRT types (RFC 6396).
const (
	mrtTableDumpV2 = 13
	mrtBGP4MP      = 16
	mrtBGP4MPET    = 17
)

// TABLE_DUMP_V2 subtypes. The RIB subtypes are taken from internal/mrt rather
// than respelled here: a second copy is a second thing to drift, and this file
// already drifted once by omitting the RFC 8050 add-path subtypes (8-12), which
// made forEachRIBEntry's add-path handling unreachable from every subcommand.
const (
	subtypePeerIndexTable = 1
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

// processMRTFile opens filename (or stdin when "-"), reads all MRT records, and
// dispatches to handler. It shares "-" resolution with the ze-analyze choke
// point (mrt.ReadFile / openReader): "-" reads stdin with magic-byte compression
// sniffing, a real path keeps extension-based sniffing.
func processMRTFile(filename string, h mrtHandler) error {
	rc, err := cliio.OpenReader(filename)
	if err != nil {
		return err
	}

	// Stdin has no filename extension: sniff compression by magic bytes so a
	// gzipped/bzip2'd pipe is not misread as raw (R-4).
	if cliio.IsStdin(filename) {
		dc, derr := mrt.SniffDecompress(rc) // dc owns and closes rc
		if derr != nil {
			return derr
		}
		defer dc.Close() //nolint:errcheck // best-effort close on decompressor
		return readMRTRecords(dc, h)
	}

	defer rc.Close() //nolint:errcheck // best-effort close on read-only file
	var r io.Reader
	switch {
	case strings.HasSuffix(filename, ".gz"):
		gz, gerr := gzip.NewReader(rc)
		if gerr != nil {
			return fmt.Errorf("gzip: %w", gerr)
		}
		defer gz.Close() //nolint:errcheck // best-effort close on decompressor
		r = gz
	case strings.HasSuffix(filename, ".bz2"):
		r = bzip2.NewReader(rc)
	default:
		r = rc
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
			// RFC 6396 Section 4.3 RIB subtypes plus the RFC 8050 add-path
			// subtypes (8-12). The add-path ones were missing, so a dump from a
			// collector with add-path enabled produced NO routes at all from
			// count/dump/aspath/attributes/communities -- an empty analysis
			// presented as a complete one, with exit 0.
			case mrt.TDV2RIBIPv4Unicast, mrt.TDV2RIBIPv4Multicast,
				mrt.TDV2RIBIPv6Unicast, mrt.TDV2RIBIPv6Multicast,
				mrt.TDV2RIBGeneric,
				mrt.TDV2RIBIPv4UnicastAP, mrt.TDV2RIBIPv4MulticastAP,
				mrt.TDV2RIBIPv6UnicastAP, mrt.TDV2RIBIPv6MulticastAP,
				mrt.TDV2RIBGenericAP:
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

// errShortRIBRecord marks a TABLE_DUMP_V2 RIB record too short to carry the
// sequence number, prefix length and prefix that RFC 6396 Section 4.3.2
// requires. getRIBPrefix reports that as a nil slice, which on its own is
// indistinguishable from "nothing to do".
var errShortRIBRecord = errors.New("analyze: RIB record too short to carry a prefix")

// damageTag renders a short, stable classification of a decode failure, for the
// one-line-per-record surfaces where the full error would not fit.
//
// It matches on the exported sentinels rather than the message text, so a
// reworded error can never silently reclassify a record
// (ai/rules/cli.md: one stable leading phrase per failure kind).
// The default arm is deliberately not "unknown": every arm names something an
// operator can act on, and "damaged" is the honest fallback.
func damageTag(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, mrt.ErrShortData):
		return "truncated"
	case errors.Is(err, mrt.ErrBadAFI):
		return "unsupported-afi"
	default:
		return "damaged"
	}
}

// bgp4mpFourByteAS reports whether AS_PATH inside a BGP4MP record with this
// subtype uses 4-byte AS numbers.
//
// readMRTRecords delivers OnBGP4MP for TypeBGP4MP and TypeBGP4MPET alike, and
// RFC 6396 gives the two the same per-subtype AS width (mrt.ASPathIsFourByte
// switches on subtype for both), so the subtype decides on its own and the
// callback does not need to carry the record type.
//
// Every AS_PATH consumer in this package MUST reach the width through this
// helper or mrt.ASPathIsFourByte. Hardcoding 4 bytes reads a 2-byte path
// (subtypes 1 and 6) as half as many, twice-as-large, entirely fictitious ASNs
// -- and the byte count alone cannot reveal the mistake.
func bgp4mpFourByteAS(subtype uint16) bool {
	return mrt.ASPathIsFourByte(mrt.TypeBGP4MP, subtype)
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

// extractUpdateAttrs returns the path attributes section from an UPDATE body,
// or nil when the body is malformed.
//
// The field offsets live in internal/mrt so the offline tools and the MRT
// decoder can never disagree about them.
func extractUpdateAttrs(update []byte) []byte {
	attrs, err := mrt.UpdateAttributeBytes(update)
	if err != nil {
		return nil
	}
	return attrs
}

// iterateAttrs calls fn for each attribute in a packed attribute section.
// fn receives flags, type code, and the attribute value bytes.
//
// Attribute decoding itself lives in internal/mrt (mrt.ParseAttributes); this
// adapter only preserves the callback shape the analyze subcommands use.
func iterateAttrs(attrs []byte, fn func(flags, typeCode uint8, value []byte)) {
	for _, a := range mrt.ParseAttributes(attrs) {
		fn(a.Flags, a.Code, a.Value)
	}
}

// countAttrs counts the number of attributes in a packed attribute section.
func countAttrs(attrs []byte) int {
	return len(mrt.ParseAttributes(attrs))
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
//
// Record walking lives in internal/mrt (DecodeRIBRecord / DecodeRIBGenericRecord),
// which unlike the previous hand-rolled walk also honors the RFC 8050 add-path
// subtypes' 4-octet Path Identifier. A malformed record yields no entries
// rather than a partial prefix of them.
// It returns the decode error for a malformed record so the caller can count
// and report it. Silently yielding no entries would make a damaged record
// indistinguishable from an empty one (ai/rules/evidence.md).
func forEachRIBEntry(data []byte, subtype uint16, fn func(peerIndex uint16, attrs []byte)) error {
	var entries []mrt.RIBEntry

	switch subtype {
	case mrt.TDV2RIBGeneric, mrt.TDV2RIBGenericAP:
		rec, err := mrt.DecodeRIBGenericRecord(subtype, data)
		if err != nil {
			return err
		}
		entries = rec.Entries
	default:
		rec, err := mrt.DecodeRIBRecord(subtype, data)
		if err != nil {
			return err
		}
		entries = rec.Entries
	}

	for i := range entries {
		fn(entries[i].PeerIndex, entries[i].Attributes)
	}
	return nil
}

// malformedCounter tallies MRT records that failed to decode, so a subcommand
// can tell the operator the input is damaged instead of silently
// under-reporting.
//
// The noun is "record" rather than "RIB record" because both record kinds reach
// it: RIB records via forEachRIBEntry, and BGP4MP UPDATE records via density's
// NLRI counting. The zero value is ready to use.
type malformedCounter struct {
	records int
}

func (m *malformedCounter) note(err error) {
	if err != nil {
		m.records++
	}
}

// report writes a warning to w when any record failed to decode. Silent when
// the input was clean, so a good run prints no noise.
func (m *malformedCounter) report(w io.Writer) {
	if m.records == 0 {
		return
	}
	var tb textbuf.Buffer
	tb.Str("warning: ").Int(int64(m.records)).Str(" malformed MRT record(s) skipped or partially decoded; results are incomplete\n")
	wf(w, "%s", tb.Slice())
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
	if _, err := fmt.Fprintf(w, format, args...); err != nil { //nolint:errcheck // output
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
	s := strconv.Itoa(int(n))
	result := make([]byte, 0, len(s)+(len(s)-1)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}
