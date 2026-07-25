// Design: docs/architecture/mrt.md -- BMP-to-MRT recording server

package analyze

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/subdispatch"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/mrt"
)

var recordDispatcher = newRecordDispatcher()

func newRecordDispatcher() *subdispatch.Dispatcher {
	d := subdispatch.New("record", "Record incoming protocol streams to MRT")
	d.Register("bmp", runRecordBMP, subdispatch.SubMeta{Desc: "Accept BMP connections and write received BGP messages as MRT"})
	return d
}

func runRecord(args []string) int {
	return recordDispatcher.Dispatch(args)
}

const recordBMPUsage = `ze-analyze record bmp -- accept BMP connections, write MRT files

Listens for incoming BMP (RFC 7854) connections and writes received BGP
messages as BGP4MP MRT records. Useful for recording traffic from routers
that speak BMP for later replay or analysis.

Usage:
  ze-analyze record bmp [options] <output.mrt>

Options:
  --listen <addr:port>  Listen address (default: :4321)
`

type syncWriter struct {
	mu sync.Mutex
	w  *mrt.Writer
}

func (sw *syncWriter) Write(data []byte) error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Write(data)
}

func (sw *syncWriter) Close() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.w.Close()
}

func runRecordBMP(args []string) int {
	listen := ":4321"
	var outputFile string

	positional := make([]string, 0, 1)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			i++
			if i >= len(args) {
				os.Stderr.WriteString(recordBMPUsage) //nolint:errcheck // usage
				return 1
			}
			listen = args[i]
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) != 1 {
		os.Stderr.WriteString(recordBMPUsage) //nolint:errcheck // usage
		return 1
	}
	outputFile = positional[0]

	sw := &syncWriter{w: mrt.NewWriter(outputFile)}
	defer func() { _ = sw.Close() }()

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", listen)
	if err != nil {
		b := textbuf.Get()
		b.Str("record bmp: listen: ").Str(err.Error()).Byte('\n')
		os.Stderr.WriteString(b.String()) //nolint:errcheck // error
		b.Release()
		return 1
	}
	defer func() { _ = ln.Close() }()

	b := textbuf.Get()
	b.Str("record bmp: listening on ").Str(listen).Str(", writing to ").Str(outputFile).Byte('\n')
	os.Stderr.WriteString(b.String()) //nolint:errcheck // info
	b.Release()

	for {
		conn, err := ln.Accept()
		if err != nil {
			b := textbuf.Get()
			b.Str("record bmp: accept: ").Str(err.Error()).Byte('\n')
			os.Stderr.WriteString(b.String()) //nolint:errcheck // error
			b.Release()
			return 1
		}
		go handleBMPConn(conn, sw)
	}
}

func handleBMPConn(conn net.Conn, w *syncWriter) {
	defer func() { _ = conn.Close() }()

	remote := conn.RemoteAddr().String()
	b := textbuf.Get()
	b.Str("record bmp: connection from ").Str(remote).Byte('\n')
	os.Stderr.WriteString(b.String()) //nolint:errcheck // info
	b.Release()

	var recorded uint64
	hdr := make([]byte, BMPCommonHdrLen)

	for {
		if err := bgpReadFull(conn, hdr); err != nil {
			if !errors.Is(err, io.EOF) {
				b := textbuf.Get()
				b.Str("record bmp: read error from ").Str(remote).Str(": ").Str(err.Error()).Byte('\n')
				os.Stderr.WriteString(b.String()) //nolint:errcheck // error
				b.Release()
			}
			break
		}

		if hdr[0] != 3 {
			break
		}
		msgLen := binary.BigEndian.Uint32(hdr[1:5])
		msgType := hdr[5]

		if msgLen < BMPCommonHdrLen || msgLen > 65535 {
			break
		}

		body := make([]byte, msgLen-BMPCommonHdrLen)
		if err := bgpReadFull(conn, body); err != nil {
			break
		}

		switch msgType {
		case 0:
			recorded += bmpRouteMonToMRT(body, w)
		case 2:
			recorded += bmpPeerDownToMRT(body, w)
		case 3:
			recorded += bmpPeerUpToMRT(body, w)
		}
	}

	b = textbuf.Get()
	b.Str("record bmp: ").Str(remote).Str(" disconnected, recorded ").Uint(recorded).Str(" messages\n")
	os.Stderr.WriteString(b.String()) //nolint:errcheck // info
	b.Release()
}

// bmpRouteMonToMRT extracts the BGP message from a BMP Route Monitoring body
// (per-peer header + BGP message) and writes it as a BGP4MP_MESSAGE_AS4 MRT record.
func bmpRouteMonToMRT(body []byte, w *syncWriter) uint64 {
	if len(body) < BMPPeerHdrLen {
		return 0
	}

	flags := body[1]
	isV6 := flags&0x80 != 0

	var peerIP []byte
	if isV6 {
		peerIP = body[10:26]
	} else {
		peerIP = body[22:26]
	}

	peerAS := binary.BigEndian.Uint32(body[26:30])
	tsSec := binary.BigEndian.Uint32(body[34:38])
	if tsSec == 0 {
		tsSec = uint32(time.Now().Unix()) //nolint:gosec // timestamp fits uint32 until 2106
	}

	bgpMsg := body[BMPPeerHdrLen:]
	if len(bgpMsg) < 19 {
		return 0
	}

	afi := mrt.AFIIPv4
	if isV6 {
		afi = mrt.AFIIPv6
	}

	localIP := make([]byte, len(peerIP))

	totalLen := mrt.CommonHeaderLen + 4 + 4 + 2 + 2 + len(peerIP) + len(localIP) + len(bgpMsg)
	msgBuf := make([]byte, totalLen)

	off := mrt.CommonHeaderLen
	binary.BigEndian.PutUint32(msgBuf[off:], peerAS)
	off += 4
	binary.BigEndian.PutUint32(msgBuf[off:], 0) // local AS unknown from BMP
	off += 4
	binary.BigEndian.PutUint16(msgBuf[off:], 0) // ifIndex
	off += 2
	binary.BigEndian.PutUint16(msgBuf[off:], afi)
	off += 2
	copy(msgBuf[off:], peerIP)
	off += len(peerIP)
	copy(msgBuf[off:], localIP)
	off += len(localIP)
	copy(msgBuf[off:], bgpMsg)
	off += len(bgpMsg)

	msgLen := uint32(off - mrt.CommonHeaderLen)
	mrt.WriteCommonHeader(msgBuf, 0, tsSec, mrt.TypeBGP4MP, mrt.BGP4MPMessageAS4, msgLen)

	if err := w.Write(msgBuf[:off]); err != nil {
		return 0
	}
	return 1
}

// bmpPeerUpToMRT writes a BGP4MP_STATE_CHANGE_AS4 (Idle -> Established).
func bmpPeerUpToMRT(body []byte, w *syncWriter) uint64 {
	return bmpStateChangeToMRT(body, w, mrt.FSMIdle, mrt.FSMEstablished)
}

// bmpPeerDownToMRT writes a BGP4MP_STATE_CHANGE_AS4 (Established -> Idle).
func bmpPeerDownToMRT(body []byte, w *syncWriter) uint64 {
	return bmpStateChangeToMRT(body, w, mrt.FSMEstablished, mrt.FSMIdle)
}

func bmpStateChangeToMRT(body []byte, w *syncWriter, oldState, newState uint16) uint64 {
	if len(body) < BMPPeerHdrLen {
		return 0
	}

	flags := body[1]
	isV6 := flags&0x80 != 0

	var peerIP []byte
	if isV6 {
		peerIP = body[10:26]
	} else {
		peerIP = body[22:26]
	}

	peerAS := binary.BigEndian.Uint32(body[26:30])
	tsSec := binary.BigEndian.Uint32(body[34:38])
	if tsSec == 0 {
		tsSec = uint32(time.Now().Unix()) //nolint:gosec // timestamp fits uint32 until 2106
	}

	afi := mrt.AFIIPv4
	if isV6 {
		afi = mrt.AFIIPv6
	}

	localIP := make([]byte, len(peerIP))

	// BGP4MP_STATE_CHANGE_AS4: PeerAS(4) + LocalAS(4) + IfIndex(2) + AFI(2) + PeerIP + LocalIP + OldState(2) + NewState(2)
	totalLen := mrt.CommonHeaderLen + 4 + 4 + 2 + 2 + len(peerIP) + len(localIP) + 2 + 2
	msgBuf := make([]byte, totalLen)

	off := mrt.CommonHeaderLen
	binary.BigEndian.PutUint32(msgBuf[off:], peerAS)
	off += 4
	binary.BigEndian.PutUint32(msgBuf[off:], 0)
	off += 4
	binary.BigEndian.PutUint16(msgBuf[off:], 0)
	off += 2
	binary.BigEndian.PutUint16(msgBuf[off:], afi)
	off += 2
	copy(msgBuf[off:], peerIP)
	off += len(peerIP)
	copy(msgBuf[off:], localIP)
	off += len(localIP)
	binary.BigEndian.PutUint16(msgBuf[off:], oldState)
	off += 2
	binary.BigEndian.PutUint16(msgBuf[off:], newState)
	off += 2

	msgLen := uint32(off - mrt.CommonHeaderLen)
	mrt.WriteCommonHeader(msgBuf, 0, tsSec, mrt.TypeBGP4MP, mrt.BGP4MPStateChangeAS4, msgLen)

	if err := w.Write(msgBuf[:off]); err != nil {
		return 0
	}
	return 1
}
