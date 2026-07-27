// Design: docs/architecture/mrt.md — MRT record filtering
// RFC: rfc/short/rfc6396.md -- per-record-type AS width (Sections 4.2, 4.3.4, 4.4.2, 4.4.3)

package analyze

import (
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/mrt"
)

type filterOpts struct {
	peerIP      net.IP
	peerASN     uint32
	prefix      string
	typeName    string
	after       uint32
	before      uint32
	asPathRe    *regexp.Regexp
	communityRe *regexp.Regexp
	inputFile   string
	outputFile  string
}

const filterUsage = `ze-analyze filter -- filter MRT records to a new file

Reads MRT records and writes only matching ones to the output file.
All matching is exact. Multiple filters are AND-composed.

Usage:
  ze-analyze filter [options] <input.mrt> <output.mrt>

Options:
  --peer-ip <ip>        Filter by peer IP address
  --peer-asn <asn>      Filter by peer ASN
  --prefix <prefix/len> Filter by exact prefix match (RIB records only)
  --type <name>         Filter by MRT type (bgp4mp, table-dump-v2, table-dump)
  --after <timestamp>   Only records after this unix timestamp
  --before <timestamp>  Only records before this unix timestamp
  --as-path <regex>     Filter by AS-path regex (space-separated ASNs)
  --community <regex>   Filter by community regex (matched per community string)
`

func parseFilterOpts(args []string) (*filterOpts, bool) {
	opts := &filterOpts{before: ^uint32(0)}
	positional := make([]string, 0, 2)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--peer-ip":
			i++
			if i >= len(args) {
				return nil, false
			}
			opts.peerIP = net.ParseIP(args[i])
			if opts.peerIP == nil {
				return nil, false
			}
		case "--peer-asn":
			i++
			if i >= len(args) {
				return nil, false
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return nil, false
			}
			opts.peerASN = uint32(v) //nolint:gosec // validated range
		case "--prefix":
			i++
			if i >= len(args) {
				return nil, false
			}
			opts.prefix = args[i]
		case "--type":
			i++
			if i >= len(args) {
				return nil, false
			}
			opts.typeName = args[i]
		case "--after":
			i++
			if i >= len(args) {
				return nil, false
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return nil, false
			}
			opts.after = uint32(v) //nolint:gosec // validated range
		case "--before":
			i++
			if i >= len(args) {
				return nil, false
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return nil, false
			}
			opts.before = uint32(v) //nolint:gosec // validated range
		case "--as-path":
			i++
			if i >= len(args) {
				return nil, false
			}
			re, err := regexp.Compile(args[i])
			if err != nil {
				os.Stderr.WriteString("filter: bad --as-path regex: " + err.Error() + "\n") //nolint:errcheck // error output
				return nil, false
			}
			opts.asPathRe = re
		case "--community":
			i++
			if i >= len(args) {
				return nil, false
			}
			re, err := regexp.Compile(args[i])
			if err != nil {
				os.Stderr.WriteString("filter: bad --community regex: " + err.Error() + "\n") //nolint:errcheck // error output
				return nil, false
			}
			opts.communityRe = re
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 2 {
		return nil, false
	}
	opts.inputFile = positional[0]
	opts.outputFile = positional[1]
	return opts, true
}

func (o *filterOpts) matchType(h mrt.Header) bool {
	if o.typeName == "" {
		return true
	}
	switch strings.ToLower(o.typeName) {
	case "bgp4mp":
		return h.Type == mrt.TypeBGP4MP || h.Type == mrt.TypeBGP4MPET
	case "table-dump-v2":
		return h.Type == mrt.TypeTableDumpV2
	case "table-dump":
		return h.Type == mrt.TypeTableDump
	}
	return false
}

func (o *filterOpts) matchTimestamp(h mrt.Header) bool {
	return h.Timestamp >= o.after && h.Timestamp <= o.before
}

func runFilter(args []string) int {
	opts, ok := parseFilterOpts(args)
	if !ok {
		os.Stderr.WriteString(filterUsage) //nolint:errcheck // usage output
		return 1
	}

	w := mrt.NewWriter(opts.outputFile)
	defer w.Close() //nolint:errcheck // best-effort

	needsDeepFilter := opts.peerIP != nil || opts.peerASN != 0 || opts.prefix != "" ||
		opts.asPathRe != nil || opts.communityRe != nil

	var matched, total uint64
	headerBuf := make([]byte, mrt.CommonHeaderLen)

	var pendingHeader mrt.Header
	var pendingData []byte

	writeRawRecord := func(h mrt.Header, data []byte) error {
		matched++
		mrt.WriteCommonHeader(headerBuf, 0, h.Timestamp, h.Type, h.Subtype, h.Length)
		if err := w.Write(headerBuf); err != nil {
			return err
		}
		return w.Write(data)
	}

	err := mrt.ReadFile(opts.inputFile, &mrt.Handler{
		OnHeader: func(h mrt.Header, _ uint32, data []byte) error {
			total++
			if !opts.matchType(h) || !opts.matchTimestamp(h) {
				pendingData = nil
				return nil
			}

			if !needsDeepFilter {
				return writeRawRecord(h, data)
			}

			pendingHeader = h
			pendingData = data
			return nil
		},
		OnMessage: func(h mrt.Header, _ uint32, m *mrt.MessageRecord) error {
			if pendingData == nil {
				return nil
			}
			if opts.peerIP != nil && !opts.peerIP.Equal(net.IP(m.PeerIP)) {
				pendingData = nil
				return nil
			}
			if opts.peerASN != 0 && m.PeerAS != opts.peerASN {
				pendingData = nil
				return nil
			}
			if opts.prefix != "" {
				pendingData = nil
				return nil
			}
			if opts.asPathRe != nil || opts.communityRe != nil {
				if !matchMessageContent(m, mrt.ASPathIsFourByte(h.Type, h.Subtype), opts) {
					pendingData = nil
					return nil
				}
			}
			err := writeRawRecord(pendingHeader, pendingData)
			pendingData = nil
			return err
		},
		OnRIB: func(h mrt.Header, r *mrt.RIBRecord) error {
			if pendingData == nil {
				return nil
			}
			if opts.prefix != "" {
				afi := mrt.RIBSubtypeAFI(h.Subtype)
				if !prefixMatches(opts.prefix, r.PrefixLength, r.Prefix, afi) {
					pendingData = nil
					return nil
				}
			}
			if opts.asPathRe != nil || opts.communityRe != nil {
				if !matchRIBContent(r, opts) {
					pendingData = nil
					return nil
				}
			}
			err := writeRawRecord(pendingHeader, pendingData)
			pendingData = nil
			return err
		},
		OnTableDump: func(h mrt.Header, t *mrt.TableDumpRecord) error {
			if pendingData == nil {
				return nil
			}
			if opts.peerIP != nil && !opts.peerIP.Equal(net.IP(t.PeerIP)) {
				pendingData = nil
				return nil
			}
			if opts.peerASN != 0 && uint32(t.PeerAS) != opts.peerASN {
				pendingData = nil
				return nil
			}
			if opts.asPathRe != nil || opts.communityRe != nil {
				// TABLE_DUMP (type 12) is 2-byte AS (RFC 6396 Section 4.2).
				// Hardcoding 4-byte here made ParseASPath overrun every record,
				// matchASPath return false for all of them, and the run report
				// "filtered 0/N records" with exit 0: silent total data loss.
				if !matchAttrsContent(t.Attributes, mrt.ASPathIsFourByte(h.Type, h.Subtype), opts) {
					pendingData = nil
					return nil
				}
			}
			err := writeRawRecord(pendingHeader, pendingData)
			pendingData = nil
			return err
		},
	})
	if err != nil {
		wf(os.Stderr, "filter error: %v\n", err)
		return 1
	}

	wf(os.Stderr, "filtered %d/%d records -> %s\n", matched, total, opts.outputFile)
	return 0
}

// prefixMatches checks whether a RIB record's prefix matches the filter string.
// filterPrefix is "addr/len" (e.g. "10.0.0.0/8" or "2001:db8::/32").
func prefixMatches(filterPrefix string, recPrefixLen uint8, recPrefix []byte, afi uint16) bool {
	parts := strings.SplitN(filterPrefix, "/", 2)
	if len(parts) != 2 {
		return false
	}
	filterIP := net.ParseIP(parts[0])
	if filterIP == nil {
		return false
	}
	filterLen, err := strconv.Atoi(parts[1])
	if err != nil || filterLen < 0 {
		return false
	}

	if int(recPrefixLen) != filterLen {
		return false
	}

	var filterBytes []byte
	if afi == mrt.AFIIPv4 {
		filterBytes = filterIP.To4()
	} else {
		filterBytes = filterIP.To16()
	}
	if filterBytes == nil {
		return false
	}

	prefixByteCount := (int(recPrefixLen) + 7) / 8
	if len(recPrefix) < prefixByteCount || len(filterBytes) < prefixByteCount {
		return false
	}
	for i := range prefixByteCount {
		if recPrefix[i] != filterBytes[i] {
			return false
		}
	}
	return true
}
