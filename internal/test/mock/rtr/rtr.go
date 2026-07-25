// Design: docs/architecture/testing/ci-format.md -- mock RTR cache server for RPKI testing

package rtr

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	rtrMockPDUSerialQuery = 1
	rtrMockPDUResetQuery  = 2
	rtrMockPDUCacheResp   = 3
	rtrMockPDUIPv4Prefix  = 4
	rtrMockPDUIPv6Prefix  = 6
	rtrMockPDUEndOfData   = 7
	rtrMockPDUASPA        = 11

	rtrMockVersion2  = 2
	rtrMockHeaderLen = 8
)

type vrpFlag struct {
	prefix    net.IPNet
	maxLength uint8
	asn       uint32
}

type vrpList []vrpFlag

func (v *vrpList) String() string { return textbuf.IntStr(int64(len(*v)), " VRPs") }

func (v *vrpList) Set(s string) error {
	parts := strings.SplitN(s, ",", 3)
	if len(parts) != 3 {
		return fmt.Errorf("expected prefix,maxlen,asn got %q", s)
	}

	_, ipnet, err := net.ParseCIDR(parts[0])
	if err != nil {
		return fmt.Errorf("invalid prefix %q: %w", parts[0], err)
	}

	maxLen, err := strconv.ParseUint(parts[1], 10, 8)
	if err != nil {
		return fmt.Errorf("invalid maxlen %q: %w", parts[1], err)
	}

	asn, err := strconv.ParseUint(parts[2], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid ASN %q: %w", parts[2], err)
	}

	*v = append(*v, vrpFlag{
		prefix:    *ipnet,
		maxLength: uint8(maxLen), //nolint:gosec // range checked by ParseUint
		asn:       uint32(asn),   //nolint:gosec // range checked by ParseUint
	})
	return nil
}

type aspaFlag struct {
	customerAS uint32
	providers  []uint32
}

type aspaList []aspaFlag

func (a *aspaList) String() string { return textbuf.IntStr(int64(len(*a)), " ASPAs") }

func (a *aspaList) Set(s string) error {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("expected customer:provider1,provider2,... got %q", s)
	}

	customer, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid customer AS %q: %w", parts[0], err)
	}

	provStrs, count := stringsx.SplitCount(parts[1], ",")
	providers := make([]uint32, 0, count)
	for _, ps := range provStrs {
		p, err := strconv.ParseUint(strings.TrimSpace(ps), 10, 32)
		if err != nil {
			return fmt.Errorf("invalid provider AS %q: %w", ps, err)
		}
		providers = append(providers, uint32(p)) //nolint:gosec // range checked
	}

	*a = append(*a, aspaFlag{
		customerAS: uint32(customer), //nolint:gosec // range checked
		providers:  providers,
	})
	return nil
}

func Run(args []string) int {
	var (
		port  int
		vrps  vrpList
		aspas aspaList
	)

	fs := flag.NewFlagSet("ze-test rtr-mock", flag.ExitOnError)
	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")
	fs.Var(&vrps, "vrp", "VRP entry: prefix,maxlen,asn (repeatable)")
	fs.Var(&aspas, "aspa", "ASPA record: customer:provider1,provider2,... (repeatable)")
	serialFlag := fs.Uint("serial", 1, "initial serial number")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze-test rtr-mock [flags]\n\nMock RTR cache server for RPKI testing.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}
	serial := uint32(*serialFlag) //nolint:gosec // serial number fits uint32

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "ze-test rtr-mock: listening on port %s with %d VRPs, %d ASPAs\n", portStr, len(vrps), len(aspas))

	for {
		conn, err := ln.Accept()
		if err != nil {
			return 0
		}
		go rtrMockHandleConn(conn, vrps, aspas, serial)
	}
}

func rtrMockHandleConn(conn net.Conn, vrps vrpList, aspas aspaList, serial uint32) {
	defer func() { _ = conn.Close() }()

	header := make([]byte, rtrMockHeaderLen)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}

		clientVersion := header[0]
		pduType := header[1]
		pduLen := binary.BigEndian.Uint32(header[4:8])

		remaining := int(pduLen) - rtrMockHeaderLen
		if remaining > 4096 {
			return
		}
		if remaining > 0 {
			discard := make([]byte, remaining)
			if _, err := io.ReadFull(conn, discard); err != nil {
				return
			}
		}

		switch pduType {
		case rtrMockPDUResetQuery, rtrMockPDUSerialQuery:
			if err := rtrMockSendResponse(conn, vrps, aspas, serial, clientVersion); err != nil {
				return
			}
		default:
		}
	}
}

func rtrMockSendResponse(conn net.Conn, vrps vrpList, aspas aspaList, serial uint32, version uint8) error {
	sessionID := uint16(1)

	cr := make([]byte, rtrMockHeaderLen)
	cr[0] = version
	cr[1] = rtrMockPDUCacheResp
	binary.BigEndian.PutUint16(cr[2:4], sessionID)
	binary.BigEndian.PutUint32(cr[4:8], rtrMockHeaderLen)
	if _, err := conn.Write(cr); err != nil {
		return err
	}

	for _, vrp := range vrps {
		if err := rtrMockSendPrefixPDU(conn, vrp, version); err != nil {
			return err
		}
	}

	if version >= rtrMockVersion2 {
		for _, aspa := range aspas {
			if err := rtrMockSendASPAPDU(conn, aspa); err != nil {
				return err
			}
		}
	}

	eod := make([]byte, 24)
	eod[0] = version
	eod[1] = rtrMockPDUEndOfData
	binary.BigEndian.PutUint16(eod[2:4], sessionID)
	binary.BigEndian.PutUint32(eod[4:8], 24)
	binary.BigEndian.PutUint32(eod[8:12], serial)
	binary.BigEndian.PutUint32(eod[12:16], 3600)
	binary.BigEndian.PutUint32(eod[16:20], 600)
	binary.BigEndian.PutUint32(eod[20:24], 7200)
	if _, err := conn.Write(eod); err != nil {
		return err
	}

	return nil
}

func rtrMockSendPrefixPDU(conn net.Conn, vrp vrpFlag, version uint8) error {
	ip4 := vrp.prefix.IP.To4()
	if ip4 != nil {
		pdu := make([]byte, 20)
		pdu[0] = version
		pdu[1] = rtrMockPDUIPv4Prefix
		binary.BigEndian.PutUint32(pdu[4:8], 20)
		pdu[8] = 1
		prefixLen, _ := vrp.prefix.Mask.Size()
		pdu[9] = byte(prefixLen) //nolint:gosec // prefixLen 0-32
		pdu[10] = vrp.maxLength
		copy(pdu[12:16], ip4)
		binary.BigEndian.PutUint32(pdu[16:20], vrp.asn)
		_, err := conn.Write(pdu)
		return err
	}

	ip6 := vrp.prefix.IP.To16()
	pdu := make([]byte, 32)
	pdu[0] = version
	pdu[1] = rtrMockPDUIPv6Prefix
	binary.BigEndian.PutUint32(pdu[4:8], 32)
	pdu[8] = 1
	prefixLen, _ := vrp.prefix.Mask.Size()
	pdu[9] = byte(prefixLen) //nolint:gosec // prefixLen 0-128
	pdu[10] = vrp.maxLength
	copy(pdu[12:28], ip6)
	binary.BigEndian.PutUint32(pdu[28:32], vrp.asn)
	_, err := conn.Write(pdu)
	return err
}

func rtrMockSendASPAPDU(conn net.Conn, aspa aspaFlag) error {
	pduLen := 16 + 4*len(aspa.providers)
	pdu := make([]byte, pduLen)
	pdu[0] = rtrMockVersion2
	pdu[1] = rtrMockPDUASPA
	binary.BigEndian.PutUint32(pdu[4:8], uint32(pduLen)) //nolint:gosec // bounded by provider count
	pdu[8] = 1
	pdu[9] = 0
	binary.BigEndian.PutUint32(pdu[12:16], aspa.customerAS)
	for i, p := range aspa.providers {
		binary.BigEndian.PutUint32(pdu[16+i*4:16+i*4+4], p)
	}
	_, err := conn.Write(pdu)
	return err
}
