// Design: docs/architecture/mrt.md -- export MRT as BMP to a collector

package analyze

import (
	"context"
	"encoding/binary"
	"net"
	"os"

	"github.com/ze-software/ze/internal/core/subdispatch"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

var exportDispatcher = newExportDispatcher()

func newExportDispatcher() *subdispatch.Dispatcher {
	d := subdispatch.New("export", "Export MRT data to network targets")
	d.Register("bmp", runExportBMP, subdispatch.SubMeta{Desc: "Send BGP4MP records as BMP Route Monitoring to a collector"})
	return d
}

func runExport(args []string) int {
	return exportDispatcher.Dispatch(args)
}

const exportBMPUsage = `ze-analyze export bmp -- send MRT BGP4MP records as BMP Route Monitoring

Connects to a BMP collector via TCP and sends each BGP4MP message record
as a BMP v3 Route Monitoring message. TABLE_DUMP and state change records
are skipped.

Usage:
  ze-analyze export bmp --target <host:port> [--peer-ip <ip>] <input.mrt>

Options:
  --target <host:port>  BMP collector address (required)
  --peer-ip <ip>        Only send records from this peer (optional)
`

func runExportBMP(args []string) int {
	var target string
	var peerFilter net.IP
	var inputFile string

	positional := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--target":
			i++
			if i >= len(args) {
				os.Stderr.WriteString(exportBMPUsage) //nolint:errcheck // usage
				return 1
			}
			target = args[i]
		case "--peer-ip":
			i++
			if i >= len(args) {
				os.Stderr.WriteString(exportBMPUsage) //nolint:errcheck // usage
				return 1
			}
			peerFilter = net.ParseIP(args[i])
			if peerFilter == nil {
				os.Stderr.WriteString("export bmp: invalid --peer-ip\n") //nolint:errcheck // error
				return 1
			}
		default:
			positional = append(positional, args[i])
		}
	}

	if target == "" || len(positional) != 1 {
		os.Stderr.WriteString(exportBMPUsage) //nolint:errcheck // usage
		return 1
	}
	inputFile = positional[0]

	var d net.Dialer
	conn, err := d.DialContext(context.Background(), "tcp", target)
	if err != nil {
		b := textbuf.Get()
		b.Str("export bmp: connect: ").Str(err.Error()).Byte('\n')
		os.Stderr.WriteString(b.String()) //nolint:errcheck // error
		b.Release()
		return 1
	}
	defer func() { _ = conn.Close() }()

	var count uint64
	handler := &mrt.Handler{
		OnMessage: func(h mrt.Header, usec uint32, m *mrt.MessageRecord) error {
			if len(m.BGPMessage) == 0 {
				return nil
			}
			if peerFilter != nil && !peerFilter.Equal(net.IP(m.PeerIP)) {
				return nil
			}

			bmpMsg := buildBMPRouteMonitoring(h.Timestamp, usec, m)
			if _, err := conn.Write(bmpMsg); err != nil {
				return err
			}
			count++
			return nil
		},
	}

	if err := mrt.ReadFile(inputFile, handler); err != nil {
		b := textbuf.Get()
		b.Str("export bmp: ").Str(err.Error()).Byte('\n')
		os.Stderr.WriteString(b.String()) //nolint:errcheck // error
		b.Release()
		return 1
	}

	b := textbuf.Get()
	b.Str("export bmp: sent ").Uint(count).Str(" messages to ").Str(target).Byte('\n')
	os.Stderr.WriteString(b.String()) //nolint:errcheck // status
	b.Release()
	return 0
}

// buildBMPRouteMonitoring wraps a BGP message in a BMP v3 Route Monitoring frame.
func buildBMPRouteMonitoring(ts, usec uint32, m *mrt.MessageRecord) []byte {
	totalLen := BMPCommonHdrLen + BMPPeerHdrLen + len(m.BGPMessage)
	buf := make([]byte, totalLen)

	buf[0] = 3                                             // BMP version
	binary.BigEndian.PutUint32(buf[1:5], uint32(totalLen)) //nolint:gosec // bounded
	buf[5] = 0                                             // Route Monitoring

	off := BMPCommonHdrLen
	buf[off] = 0 // peer type: global instance
	var flags uint8
	if m.AFI == mrt.AFIIPv6 {
		flags |= 0x80
	}
	buf[off+1] = flags
	if m.AFI == mrt.AFIIPv6 {
		copy(buf[off+10:off+26], m.PeerIP)
	} else if len(m.PeerIP) == 4 {
		buf[off+20] = 0xff
		buf[off+21] = 0xff
		copy(buf[off+22:off+26], m.PeerIP)
	}
	binary.BigEndian.PutUint32(buf[off+26:off+30], m.PeerAS)
	binary.BigEndian.PutUint32(buf[off+34:off+38], ts)
	binary.BigEndian.PutUint32(buf[off+38:off+42], usec)

	copy(buf[BMPCommonHdrLen+BMPPeerHdrLen:], m.BGPMessage)
	return buf
}
