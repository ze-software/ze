// Design: docs/architecture/mrt.md -- passive MRT-to-BGP server

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

const serveUsage = `ze-analyze serve -- serve MRT file contents over BGP

Listens for incoming BGP connections and sends all matching UPDATEs from
the MRT file to each connecting peer. Useful for IXP traffic replay and
router behavior testing.

Usage:
  ze-analyze serve [options] <input.mrt...>

Options:
  --listen <addr:port>  Listen address (default: :179)
  --local-as <asn>      Local ASN for OPEN message (required)
  --router-id <ip>      BGP router ID (default: 1.0.0.1)
  --hold-time <secs>    Hold time in OPEN (default: 90)
  --per-peer            Send only records matching connecting peer's ASN
`

func runServe(args []string) int {
	var (
		listen   = ":179"
		localAS  uint32
		routerID = net.IP{1, 0, 0, 1}
		holdTime = uint16(90)
		perPeer  bool
	)
	positional := make([]string, 0, 4)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--listen":
			i++
			if i >= len(args) {
				os.Stderr.WriteString(serveUsage) //nolint:errcheck // usage
				return 1
			}
			listen = args[i]
		case "--local-as": //nolint:goconst // CLI flag name
			i++
			if i >= len(args) {
				os.Stderr.WriteString(serveUsage) //nolint:errcheck // usage
				return 1
			}
			v, err := strconv.ParseUint(args[i], 10, 32)
			if err != nil {
				os.Stderr.WriteString("serve: invalid --local-as\n") //nolint:errcheck // error
				return 1
			}
			localAS = uint32(v) //nolint:gosec // validated range
		case "--router-id": //nolint:goconst // CLI flag name
			i++
			if i >= len(args) {
				os.Stderr.WriteString(serveUsage) //nolint:errcheck // usage
				return 1
			}
			routerID = net.ParseIP(args[i])
			if routerID == nil {
				os.Stderr.WriteString("serve: invalid --router-id\n") //nolint:errcheck // error
				return 1
			}
		case "--hold-time":
			i++
			if i >= len(args) {
				os.Stderr.WriteString(serveUsage) //nolint:errcheck // usage
				return 1
			}
			v, err := strconv.ParseUint(args[i], 10, 16)
			if err != nil {
				os.Stderr.WriteString("serve: invalid --hold-time\n") //nolint:errcheck // error
				return 1
			}
			holdTime = uint16(v) //nolint:gosec // validated range
		case "--per-peer":
			perPeer = true
		default:
			positional = append(positional, args[i])
		}
	}

	if localAS == 0 || len(positional) == 0 {
		os.Stderr.WriteString(serveUsage) //nolint:errcheck // usage
		return 1
	}

	lc := net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", listen)
	if err != nil {
		b := textbuf.Get()
		b.Str("serve: listen: ").Str(err.Error()).Byte('\n')
		b.StdErr() //nolint:errcheck // error
		b.Release()
		return 1
	}
	defer func() { _ = ln.Close() }()

	b := textbuf.Get()
	b.Str("serve: listening on ").Str(listen).Str(" (AS ").Uint32(localAS).Str(")\n")
	b.StdErr() //nolint:errcheck // info
	b.Release()

	for {
		conn, err := ln.Accept()
		if err != nil {
			b := textbuf.Get()
			b.Str("serve: accept: ").Str(err.Error()).Byte('\n')
			b.StdErr() //nolint:errcheck // error
			b.Release()
			return 1
		}
		go handleServeConn(conn, positional, localAS, routerID, holdTime, perPeer)
	}
}

func handleServeConn(conn net.Conn, files []string, localAS uint32, routerID net.IP, holdTime uint16, perPeer bool) {
	defer func() { _ = conn.Close() }()

	remote := conn.RemoteAddr().String()
	b := textbuf.Get()
	b.Str("serve: peer connected: ").Str(remote).Byte('\n')
	b.StdErr() //nolint:errcheck // info
	b.Release()

	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

	peerAS, err := serveBGPOpen(conn, localAS, routerID, holdTime)
	if err != nil {
		b := textbuf.Get()
		b.Str("serve: open failed from ").Str(remote).Str(": ").Str(err.Error()).Byte('\n')
		b.StdErr() //nolint:errcheck // error
		b.Release()
		return
	}

	_ = conn.SetDeadline(time.Time{})

	b = textbuf.Get()
	b.Str("serve: session up with ").Str(remote).Str(" (AS ").Uint32(peerAS).Str(")\n")
	b.StdErr() //nolint:errcheck // info
	b.Release()

	var sent uint64
	for _, f := range files {
		n, ferr := serveFile(conn, f, peerAS, perPeer)
		sent += n
		if ferr != nil {
			break
		}
	}

	// End-of-RIB: IPv4 unicast (empty UPDATE) + IPv6 unicast (MP_UNREACH_NLRI AFI2/SAFI1)
	_ = bgpWrite(conn, 2, nil)
	_ = bgpWrite(conn, 2, ipv6EOR())

	b = textbuf.Get()
	b.Str("serve: sent ").Uint(sent).Str(" updates to ").Str(remote).Byte('\n')
	b.StdErr() //nolint:errcheck // info
	b.Release()

	serveKeepaliveLoop(conn)
}

func serveBGPOpen(conn net.Conn, localAS uint32, routerID net.IP, holdTime uint16) (uint32, error) {
	// Read peer's OPEN
	msgType, body, err := bgpReadMsg(conn)
	if err != nil {
		return 0, err
	}
	if msgType != 1 || len(body) < 10 {
		return 0, errBGPProtocol
	}
	peerAS := uint32(binary.BigEndian.Uint16(body[1:3]))
	optLen := int(body[9])
	if 10+optLen <= len(body) {
		peerAS = bgpExtractAS4(body[10:10+optLen], peerAS)
	}

	// Send our OPEN + KEEPALIVE
	openBody := bgpBuildOpen(localAS, holdTime, routerID)
	if err := bgpWrite(conn, 1, openBody); err != nil {
		return 0, err
	}
	if err := bgpWrite(conn, 4, nil); err != nil {
		return 0, err
	}

	// Read peer's KEEPALIVE
	kaType, _, err := bgpReadMsg(conn)
	if err != nil {
		return 0, err
	}
	if kaType != 4 {
		return 0, errBGPProtocol
	}

	return peerAS, nil
}

func serveFile(conn net.Conn, filename string, peerAS uint32, perPeer bool) (uint64, error) {
	var count uint64
	var peerASNByIndex []uint32

	handler := &mrt.Handler{
		OnPeerIndex: func(_ mrt.Header, pit *mrt.PeerIndexTable) error {
			peerASNByIndex = make([]uint32, len(pit.Peers))
			for i, p := range pit.Peers {
				peerASNByIndex[i] = p.ASN
			}
			return nil
		},
		OnMessage: func(_ mrt.Header, _ uint32, m *mrt.MessageRecord) error {
			if len(m.BGPMessage) < 19 {
				return nil
			}
			if m.BGPMessage[18] != 2 {
				return nil
			}
			if perPeer && m.PeerAS != peerAS {
				return nil
			}
			updateBody := m.BGPMessage[19:]
			if err := bgpWrite(conn, 2, updateBody); err != nil {
				return err
			}
			count++
			return nil
		},
		OnRIB: func(h mrt.Header, r *mrt.RIBRecord) error {
			trailingNLRI := ribSubtypeHasTrailingNLRI(h.Subtype)
			for i := range r.Entries {
				entry := &r.Entries[i]
				if perPeer && !servePeerMatch(entry.PeerIndex, peerASNByIndex, peerAS) {
					continue
				}
				updateBody := buildUpdateBody(r.PrefixLength, r.Prefix, entry, trailingNLRI)
				if err := bgpWrite(conn, 2, updateBody); err != nil {
					return err
				}
				count++
			}
			return nil
		},
	}
	err := mrt.ReadFile(filename, handler)
	return count, err
}

// ipv6EOR builds an IPv6 unicast End-of-RIB UPDATE body:
// withdrawn=0, attrs=MP_UNREACH_NLRI(AFI=2, SAFI=1, empty withdrawn).
func ipv6EOR() []byte {
	// MP_UNREACH_NLRI: flags=0x80 (optional, non-transitive), code=15, len=3, AFI=2, SAFI=1
	mpUnreach := []byte{0x80, 15, 3, 0, 2, 1}
	body := make([]byte, 2+2+len(mpUnreach))
	// withdrawn routes length = 0 (bytes 0-1 already zero)
	binary.BigEndian.PutUint16(body[2:4], uint16(len(mpUnreach)))
	copy(body[4:], mpUnreach)
	return body
}

func servePeerMatch(peerIndex uint16, peerASNs []uint32, targetAS uint32) bool {
	if int(peerIndex) >= len(peerASNs) {
		return false
	}
	return peerASNs[peerIndex] == targetAS
}

// ribSubtypeHasTrailingNLRI returns true for IPv4 unicast subtypes where
// NLRI goes in the trailing field. All other subtypes (IPv6, multicast,
// generic, add-path IPv6) use MP_REACH_NLRI inside attributes.
func ribSubtypeHasTrailingNLRI(subtype uint16) bool {
	return subtype == mrt.TDV2RIBIPv4Unicast || subtype == mrt.TDV2RIBIPv4UnicastAP
}

func buildUpdateBody(prefixLen uint8, prefix []byte, entry *mrt.RIBEntry, trailingNLRI bool) []byte {
	attrLen := len(entry.Attributes)

	nlriBytes := 0
	if trailingNLRI {
		nlriBytes = 1 + len(prefix)
	}

	body := make([]byte, 2+2+attrLen+nlriBytes)
	off := 0
	binary.BigEndian.PutUint16(body[off:], 0) // withdrawn routes length
	off += 2
	binary.BigEndian.PutUint16(body[off:], uint16(attrLen)) //nolint:gosec // bounded by MRT record size
	off += 2
	copy(body[off:], entry.Attributes)
	off += attrLen
	if trailingNLRI {
		body[off] = prefixLen
		off++
		copy(body[off:], prefix)
	}
	return body
}

func serveKeepaliveLoop(conn net.Conn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	buf := make([]byte, 4096)
	for {
		if err := bgpWrite(conn, 4, nil); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		if _, err := conn.Read(buf); err != nil {
			return
		}
	}
}
