// Design: docs/architecture/mrt.md -- MRT BGP session replay

package analyze

import (
	"context"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

const replayUsage = `ze-analyze replay -- replay MRT over a BGP session

Reads BGP4MP MESSAGE records from an MRT file and replays them over a
live BGP session to a remote peer, preserving original inter-message
timing. A BGP OPEN exchange is performed first.

Only UPDATE messages (type 2) are replayed. OPEN, NOTIFICATION, and
KEEPALIVE messages in the MRT file are skipped.

Usage:
  ze-analyze replay <file.mrt[.gz|.bz2]> <peer-ip:port> [options]

Options:
  --local-as <asn>   Local AS number (default: 65000)
  --router-id <ip>   Local router ID (default: 0.0.0.1)
  --speed <factor>   Replay speed multiplier (default: 1.0, 0 = no delay)
`

type replayOpts struct {
	inputFile string
	peerAddr  string
	localAS   uint32
	routerID  net.IP
	speed     float64
}

func parseReplayOpts(args []string) (*replayOpts, bool) {
	opts := &replayOpts{
		localAS:  65000,
		routerID: net.ParseIP("0.0.0.1"),
		speed:    1.0,
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
		case "--speed":
			i++
			if i >= len(args) {
				return nil, false
			}
			v, err := strconv.ParseFloat(args[i], 64)
			if err != nil || v < 0 {
				return nil, false
			}
			opts.speed = v
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

func runReplay(args []string) int {
	opts, ok := parseReplayOpts(args)
	if !ok {
		os.Stderr.WriteString(replayUsage) //nolint:errcheck // usage output
		return 1
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(context.Background(), "tcp", opts.peerAddr)
	if err != nil {
		os.Stderr.WriteString("replay: connect failed: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}
	defer func() { _ = conn.Close() }()

	if err := bgpOpenExchange(conn, opts.localAS, 90, opts.routerID); err != nil {
		os.Stderr.WriteString("replay: session setup: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	var (
		sent   uint64
		prevTS uint32
	)

	handler := &mrt.Handler{
		OnMessage: func(h mrt.Header, _ uint32, m *mrt.MessageRecord) error {
			if len(m.BGPMessage) < 19 || m.BGPMessage[18] != 2 {
				return nil
			}

			if opts.speed > 0 && prevTS > 0 && h.Timestamp > prevTS {
				delta := time.Duration(h.Timestamp-prevTS) * time.Second
				scaled := time.Duration(float64(delta) / opts.speed)
				if scaled > 0 {
					time.Sleep(scaled)
				}
			}
			prevTS = h.Timestamp

			if _, err := conn.Write(m.BGPMessage); err != nil {
				return err
			}
			sent++
			return nil
		},
	}

	if err := mrt.ReadFile(opts.inputFile, handler); err != nil {
		os.Stderr.WriteString("replay: " + err.Error() + "\n") //nolint:errcheck // error output
		return 1
	}

	_ = bgpWrite(conn, 4, nil)
	os.Stderr.WriteString("replay: sent " + textbuf.StringUint(sent) + " UPDATEs\n") //nolint:errcheck // status
	return 0
}
