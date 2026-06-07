// Design: docs/architecture/mrt.md -- MRT RIB injection

package analyze

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"strconv"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/internal/mrt"
)

const injectUsage = `ze-analyze inject -- inject MRT routes into a BGP session

Reads TABLE_DUMP_V2 or BGP4MP UPDATE records from an MRT file and sends
them as BGP UPDATE messages to a remote peer over a raw TCP connection.

The tool opens a BGP session (OPEN exchange), then sends each route as
an UPDATE. TABLE_DUMP_V2 records are converted to UPDATE messages with
the stored path attributes. BGP4MP MESSAGE records containing UPDATEs
are forwarded verbatim.

Usage:
  ze-analyze inject <file.mrt[.gz|.bz2]> <peer-ip:port> [options]

Options:
  --local-as <asn>   Local AS number (default: 65000)
  --router-id <ip>   Local router ID (default: 0.0.0.1)
  --hold-time <sec>  Hold time in seconds (default: 90)
`

type injectOpts struct {
	inputFile string
	peerAddr  string
	localAS   uint32
	routerID  net.IP
	holdTime  uint16
}

func parseInjectOpts(args []string) (*injectOpts, bool) {
	opts := &injectOpts{
		localAS:  65000,
		routerID: net.ParseIP("0.0.0.1"),
		holdTime: 90,
	}
	positional := make([]string, 0, 2)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--local-as":
			i++
			if i >= len(args) {
				return nil, false
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return nil, false
			}
			opts.localAS = uint32(v) //nolint:gosec // validated range
		case "--router-id":
			i++
			if i >= len(args) {
				return nil, false
			}
			opts.routerID = net.ParseIP(args[i])
			if opts.routerID == nil {
				return nil, false
			}
		case "--hold-time":
			i++
			if i >= len(args) {
				return nil, false
			}
			v, err := strconv.ParseUint(args[i], 10, 16)
			if err != nil {
				return nil, false
			}
			opts.holdTime = uint16(v) //nolint:gosec // validated range
		default:
			positional = append(positional, args[i])
		}
	}
	if len(positional) != 2 {
		return nil, false
	}
	opts.inputFile = positional[0]
	opts.peerAddr = positional[1]
	return opts, true
}

func runInject(args []string) int {
	opts, ok := parseInjectOpts(args)
	if !ok {
		os.Stderr.WriteString(injectUsage) //nolint:errcheck // usage output
		return 1
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp", opts.peerAddr)
	if err != nil {
		os.Stderr.WriteString("inject: connect failed: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}
	defer func() { _ = conn.Close() }()

	if err := bgpOpenExchange(conn, opts.localAS, opts.holdTime, opts.routerID); err != nil {
		os.Stderr.WriteString("inject: session setup: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	var sent uint64
	handler := &mrt.Handler{
		OnMessage: func(_ mrt.Header, _ uint32, m *mrt.MessageRecord) error {
			if len(m.BGPMessage) < 19 || m.BGPMessage[18] != 2 {
				return nil
			}
			_, err := conn.Write(m.BGPMessage)
			if err != nil {
				return err
			}
			sent++
			return nil
		},
		OnRIB: func(_ mrt.Header, r *mrt.RIBRecord) error {
			for i := range r.Entries {
				update := buildUpdateFromRIB(r.PrefixLength, r.Prefix, &r.Entries[i])
				if _, err := conn.Write(update); err != nil {
					return err
				}
				sent++
			}
			return nil
		},
	}

	if err := mrt.ReadFile(opts.inputFile, handler); err != nil {
		os.Stderr.WriteString("inject: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	_ = writeBGPKeepalive(conn)
	os.Stderr.WriteString("inject: sent " + textbuf.Uint(sent) + " UPDATEs\n") //nolint:errcheck // status
	return 0
}

func bgpOpenExchange(conn net.Conn, localAS uint32, holdTime uint16, routerID net.IP) error {
	if err := writeBGPOpen(conn, localAS, holdTime, routerID); err != nil {
		return err
	}

	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return err
	}
	if n < 19 || buf[18] != 1 {
		return net.UnknownNetworkError("unexpected BGP response")
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}
	return writeBGPKeepalive(conn)
}

func writeBGPOpen(conn net.Conn, localAS uint32, holdTime uint16, routerID net.IP) error {
	id := routerID.To4()
	if id == nil {
		id = []byte{0, 0, 0, 1}
	}
	as2 := uint16(localAS) //nolint:gosec // AS_TRANS fallback below
	if localAS > 65535 {
		as2 = 23456 // AS_TRANS per RFC 6793
	}

	var optParams []byte
	if localAS > 65535 {
		cap4byte := []byte{65, 4, byte(localAS >> 24), byte(localAS >> 16), byte(localAS >> 8), byte(localAS)}
		optParams = append(optParams, 2, byte(len(cap4byte)))
		optParams = append(optParams, cap4byte...)
	}

	openLen := 29 + len(optParams)
	msg := make([]byte, openLen)
	for i := range 16 {
		msg[i] = 0xff
	}
	binary.BigEndian.PutUint16(msg[16:], uint16(openLen)) //nolint:gosec // bounded by optParams
	msg[18] = 1                                           // OPEN
	msg[19] = 4                                           // BGP version
	binary.BigEndian.PutUint16(msg[20:], as2)
	binary.BigEndian.PutUint16(msg[22:], holdTime)
	copy(msg[24:28], id)
	msg[28] = byte(len(optParams))
	copy(msg[29:], optParams)

	_, err := conn.Write(msg)
	return err
}

func writeBGPKeepalive(conn net.Conn) error {
	msg := make([]byte, 19)
	for i := range 16 {
		msg[i] = 0xff
	}
	binary.BigEndian.PutUint16(msg[16:], 19)
	msg[18] = 4 // KEEPALIVE
	_, err := conn.Write(msg)
	return err
}

func buildUpdateFromRIB(prefixLen uint8, prefix []byte, entry *mrt.RIBEntry) []byte {
	nlriBytes := 1 + len(prefix)
	attrLen := len(entry.Attributes)
	totalLen := 19 + 2 + 2 + attrLen + nlriBytes
	msg := make([]byte, totalLen)

	for i := range 16 {
		msg[i] = 0xff
	}
	binary.BigEndian.PutUint16(msg[16:], uint16(totalLen)) //nolint:gosec // bounded by MRT record size
	msg[18] = 2                                            // UPDATE

	off := 19
	binary.BigEndian.PutUint16(msg[off:], 0) // withdrawn routes length
	off += 2
	binary.BigEndian.PutUint16(msg[off:], uint16(attrLen)) //nolint:gosec // bounded by MRT record size
	off += 2
	copy(msg[off:], entry.Attributes)
	off += attrLen
	msg[off] = prefixLen
	off++
	copy(msg[off:], prefix)

	return msg
}
