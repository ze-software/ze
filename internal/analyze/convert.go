// Design: docs/architecture/mrt.md -- MRT format conversion

package analyze

import (
	"encoding/binary"
	"io"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/subdispatch"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

var convertDispatcher = newConvertDispatcher()

func newConvertDispatcher() *subdispatch.Dispatcher {
	d := subdispatch.New("convert", "Convert MRT to other formats")
	d.Register("pcap", runConvertPcap, subdispatch.SubMeta{Desc: "Convert BGP4MP to pcap (IPv4, Wireshark-compatible)"})
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

	if err := writePcapGlobalHeader(out); err != nil {
		os.Stderr.WriteString("convert pcap: write header: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	var count, skippedV6 uint64
	handler := &mrt.Handler{
		OnMessage: func(h mrt.Header, usec uint32, m *mrt.MessageRecord) error {
			if len(m.BGPMessage) == 0 {
				return nil
			}
			if m.AFI == mrt.AFIIPv6 {
				skippedV6++
				return nil
			}
			ts := time.Unix(int64(h.Timestamp), int64(usec)*1000)
			if err := writePcapBGPPacket(out, ts, m.PeerIP, m.LocalIP, m.BGPMessage); err != nil {
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
	if skippedV6 > 0 {
		os.Stderr.WriteString("convert pcap: skipped " + textbuf.StringUint(skippedV6) + " IPv6 records (LINKTYPE_IPV4)\n") //nolint:errcheck // status
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

// pcap global header: magic, version 2.4, timezone 0, snaplen 65535, link type raw IPv4 (228).
func writePcapGlobalHeader(f io.Writer) error {
	var hdr [24]byte
	binary.LittleEndian.PutUint32(hdr[0:], 0xa1b2c3d4) // magic
	binary.LittleEndian.PutUint16(hdr[4:], 2)          // version major
	binary.LittleEndian.PutUint16(hdr[6:], 4)          // version minor
	binary.LittleEndian.PutUint32(hdr[8:], 0)          // timezone
	binary.LittleEndian.PutUint32(hdr[12:], 0)         // sigfigs
	binary.LittleEndian.PutUint32(hdr[16:], 65535)     // snaplen
	binary.LittleEndian.PutUint32(hdr[20:], 228)       // LINKTYPE_IPV4 (raw IPv4)
	_, err := f.Write(hdr[:])
	return err
}

// writePcapBGPPacket writes one pcap record: record header + IPv4 + TCP + BGP payload.
func writePcapBGPPacket(f io.Writer, ts time.Time, srcIP, dstIP, bgpMsg []byte) error {
	src4 := ipTo4(srcIP)
	dst4 := ipTo4(dstIP)

	tcpLen := 20 + len(bgpMsg)
	ipLen := 20 + tcpLen
	totalLen := 16 + ipLen // pcap record header + IP packet

	buf := make([]byte, totalLen)

	// Pcap record header (16 bytes).
	binary.LittleEndian.PutUint32(buf[0:], uint32(ts.Unix()))            //nolint:gosec // timestamp fits uint32
	binary.LittleEndian.PutUint32(buf[4:], uint32(ts.Nanosecond()/1000)) //nolint:gosec // microseconds
	binary.LittleEndian.PutUint32(buf[8:], uint32(ipLen))                //nolint:gosec // bounded
	binary.LittleEndian.PutUint32(buf[12:], uint32(ipLen))               //nolint:gosec // bounded

	// IPv4 header (20 bytes, no options).
	off := 16
	buf[off] = 0x45 // version 4, IHL 5
	buf[off+1] = 0
	binary.BigEndian.PutUint16(buf[off+2:], uint16(ipLen)) //nolint:gosec // bounded
	buf[off+8] = 64                                        // TTL
	buf[off+9] = 6                                         // TCP
	copy(buf[off+12:], src4[:])
	copy(buf[off+16:], dst4[:])

	// TCP header (20 bytes, no options).
	off += 20
	binary.BigEndian.PutUint16(buf[off:], 179)   // src port (BGP)
	binary.BigEndian.PutUint16(buf[off+2:], 179) // dst port (BGP)
	buf[off+12] = 0x50                           // data offset = 5 words
	buf[off+13] = 0x18                           // PSH + ACK

	// BGP payload.
	off += 20
	copy(buf[off:], bgpMsg)

	_, err := f.Write(buf)
	return err
}

func ipTo4(ip []byte) [4]byte {
	var out [4]byte
	switch len(ip) {
	case 4:
		copy(out[:], ip)
	case 16:
		copy(out[:], ip[12:16])
	}
	return out
}
