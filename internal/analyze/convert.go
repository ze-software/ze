// Design: docs/architecture/mrt.md -- MRT format conversion

package analyze

import (
	"net/netip"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/core/subdispatch"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

var convertDispatcher = newConvertDispatcher()

func newConvertDispatcher() *subdispatch.Dispatcher {
	d := subdispatch.New("convert", "Convert MRT to other formats")
	d.Register("pcap", runConvertPcap, subdispatch.SubMeta{Desc: "Convert BGP4MP to pcap (IPv4 and IPv6, Wireshark-compatible)"})
	d.Register("json", runConvertJSON, subdispatch.SubMeta{Desc: "Dump MRT record headers as JSON"})
	return d
}

func runConvert(args []string) int {
	return convertDispatcher.Dispatch(args)
}

func runConvertPcap(args []string) int {
	if len(args) != 2 {
		os.Stderr.WriteString("usage: ze-analyze convert pcap <input.mrt> <output.pcap>\n") //nolint:errcheck // usage
		return 1
	}

	inputFile := args[0]
	outputFile := args[1]

	out, err := cliio.Create(outputFile) // "-" writes stdout
	if err != nil {
		os.Stderr.WriteString("convert pcap: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}
	defer func() { _ = out.Close() }()

	if err := pcap.WriteFileHeader(out, convertSnapLen, pcap.LinkTypeRaw); err != nil {
		os.Stderr.WriteString("convert pcap: write header: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	var count, unaddressed uint64
	framer := pcap.NewFramer()
	handler := &mrt.Handler{
		OnMessage: func(h mrt.Header, usec uint32, m *mrt.MessageRecord) error {
			if len(m.BGPMessage) == 0 {
				return nil
			}
			// A BGP4MP record runs from the peer to the collector, so the peer
			// is the source. Link type 101 takes the family from the version
			// nibble, which is why an IPv6 record needs no second file.
			flow, ok := convertFlow(m.PeerIP, m.LocalIP)
			if !ok {
				unaddressed++
				return nil
			}
			ts := time.Unix(int64(h.Timestamp), int64(usec)*1000)
			if err := framer.WriteMessage(out, ts, flow, m.BGPMessage, len(m.BGPMessage)); err != nil {
				return err
			}
			count++
			return nil
		},
	}

	if err := mrt.ReadFile(inputFile, handler); err != nil {
		os.Stderr.WriteString("convert pcap: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	os.Stderr.WriteString("convert pcap: wrote " + textbuf.StringUint(count) + " packets\n") //nolint:errcheck // status
	if unaddressed > 0 {
		os.Stderr.WriteString("convert pcap: skipped " + textbuf.StringUint(unaddressed) + " records with no usable peer or local address\n") //nolint:errcheck // status
	}
	return 0
}

func runConvertJSON(args []string) int {
	if len(args) != 1 {
		os.Stderr.WriteString("usage: ze-analyze convert json <input.mrt>\n") //nolint:errcheck // usage
		return 1
	}

	os.Stdout.WriteString("[\n") //nolint:errcheck // JSON output
	first := true

	handler := &mrt.Handler{
		OnHeader: func(h mrt.Header, usec uint32, _ []byte) error {
			if !first {
				os.Stdout.WriteString(",\n") //nolint:errcheck // JSON output
			}
			first = false
			os.Stdout.WriteString(`{"timestamp":` + textbuf.StringUint32(h.Timestamp)) //nolint:errcheck // JSON output
			if usec > 0 {
				os.Stdout.WriteString(`,"microsecond":` + textbuf.StringUint32(usec)) //nolint:errcheck // JSON output
			}
			os.Stdout.WriteString(`,"type":` + textbuf.StringUint16(h.Type))       //nolint:errcheck // JSON output
			os.Stdout.WriteString(`,"subtype":` + textbuf.StringUint16(h.Subtype)) //nolint:errcheck // JSON output
			os.Stdout.WriteString(`,"length":` + textbuf.StringUint32(h.Length))   //nolint:errcheck // JSON output
			os.Stdout.WriteString("}")                                             //nolint:errcheck // JSON output
			return nil
		},
	}

	if err := mrt.ReadFile(args[0], handler); err != nil {
		os.Stderr.WriteString("convert json: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	os.Stdout.WriteString("\n]\n") //nolint:errcheck // JSON output
	return 0
}

// convertSnapLen is the snapshot length the converted file declares. It covers
// the largest message RFC 8654 allows, so no record exceeds it.
const convertSnapLen = 65535

// convertFlow turns an MRT record's two addresses into the direction its
// message traveled. It reports false when either address is missing or is not
// 4 or 16 octets, which is the only record the conversion now drops: an IPv6
// record is converted like any other, because link type 101 carries both
// families.
func convertFlow(peerIP, localIP []byte) (pcap.Flow, bool) {
	peer, ok := netip.AddrFromSlice(peerIP)
	if !ok {
		return pcap.Flow{}, false
	}
	local, ok := netip.AddrFromSlice(localIP)
	if !ok {
		return pcap.Flow{}, false
	}
	peer, local = peer.Unmap(), local.Unmap()
	if peer.Is4() != local.Is4() {
		return pcap.Flow{}, false
	}
	return pcap.Flow{
		SourceAddr: peer,
		TargetAddr: local,
		SourcePort: bgpPort,
		TargetPort: bgpPort,
	}, true
}

// bgpPort is the TCP port a BGP session uses (RFC 4271 Section 3). Both ends of
// a converted flow carry it: an MRT record holds no port, so the conversion
// fabricates the pair rather than inventing an ephemeral one.
const bgpPort = 179
