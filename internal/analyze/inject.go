// Design: docs/architecture/mrt.md -- MRT RIB injection

package analyze

import (
	"context"
	"encoding/binary"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
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
		case "--local-as": //nolint:goconst // CLI flag name
			i++
			if i >= len(args) {
				return nil, false
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				return nil, false
			}
			opts.localAS = uint32(v) //nolint:gosec // validated range
		case "--router-id": //nolint:goconst // CLI flag name
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
		OnRIB: func(h mrt.Header, r *mrt.RIBRecord) error {
			trailing := ribSubtypeHasTrailingNLRI(h.Subtype)
			for i := range r.Entries {
				update := buildUpdateFromRIB(r.PrefixLength, r.Prefix, &r.Entries[i], trailing)
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

	_ = bgpWrite(conn, 4, nil)
	os.Stderr.WriteString("inject: sent " + textbuf.StringUint(sent) + " UPDATEs\n") //nolint:errcheck // status
	return 0
}

func bgpOpenExchange(conn net.Conn, localAS uint32, holdTime uint16, routerID net.IP) error {
	openBody := bgpBuildOpen(localAS, holdTime, routerID)
	if err := bgpWrite(conn, 1, openBody); err != nil {
		return err
	}

	if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return err
	}
	msgType, _, err := bgpReadMsg(conn)
	if err != nil {
		return err
	}
	if msgType != 1 {
		return errBGPProtocol
	}

	if err := bgpWrite(conn, 4, nil); err != nil {
		return err
	}

	// Read peer's KEEPALIVE confirming session establishment
	kaType, _, err := bgpReadMsg(conn)
	if err != nil {
		return err
	}
	if kaType != 4 {
		return errBGPProtocol
	}
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		return err
	}
	return nil
}

func buildUpdateFromRIB(prefixLen uint8, prefix []byte, entry *mrt.RIBEntry, trailingNLRI bool) []byte {
	body := buildUpdateBody(prefixLen, prefix, entry, trailingNLRI)
	totalLen := 19 + len(body)
	msg := make([]byte, totalLen)
	for i := range 16 {
		msg[i] = 0xff
	}
	binary.BigEndian.PutUint16(msg[16:], uint16(totalLen)) //nolint:gosec // bounded by MRT record size
	msg[18] = 2
	copy(msg[19:], body)
	return msg
}
