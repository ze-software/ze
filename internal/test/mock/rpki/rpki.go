// Design: docs/architecture/testing/ci-format.md -- deterministic RPKI mock server

package rpki

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
)

const (
	rpkiPDUCacheResp  = 3
	rpkiPDUIPv4Prefix = 4
	rpkiPDUEndOfData  = 7

	rpkiRTRVersion1  = 1
	rpkiPDUHeaderLen = 8
)

type rpkiServer struct {
	validASN   uint32
	invalidASN uint32
	serial     uint32
}

func Run(args []string) int {
	fs := flag.NewFlagSet("ze-test rpki", flag.ExitOnError)

	var (
		bind       string
		port       int
		validASN   uint
		invalidASN uint
		serial     uint
	)

	fs.StringVar(&bind, "bind", "127.0.0.1", "TCP bind address")
	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")
	fs.UintVar(&validASN, "valid-asn", 65001, "ASN for Valid VRPs (octet % 3 == 0)")
	fs.UintVar(&invalidASN, "invalid-asn", 65099, "ASN for Invalid VRPs (octet % 3 == 1)")
	fs.UintVar(&serial, "serial", 1, "initial serial number")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: ze-test rpki [flags]

Deterministic RTR cache server for RPKI functional tests.

VRPs are auto-generated for all /8 prefixes based on the first octet:
  octet %% 3 == 0  →  VRP(prefix/8, maxLen=32, ASN=valid-asn)   → Valid
  octet %% 3 == 1  →  VRP(prefix/8, maxLen=32, ASN=invalid-asn) → Invalid
  octet %% 3 == 2  →  no VRP                                    → NotFound

Examples (with default ASNs, routes from AS 65001):
  9.0.0.0/24   → Valid    (9 %% 3 == 0, VRP ASN matches 65001)
  10.0.0.0/24  → Invalid  (10 %% 3 == 1, VRP ASN 65099 != 65001)
  11.0.0.0/24  → NotFound (11 %% 3 == 2, no VRP)

Flags:
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return 1
	}

	srv := &rpkiServer{
		validASN:   uint32(validASN),   //nolint:gosec // CLI flag range
		invalidASN: uint32(invalidASN), //nolint:gosec // CLI flag range
		serial:     uint32(serial),     //nolint:gosec // CLI flag range
	}

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", net.JoinHostPort(bind, strconv.Itoa(port)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "ze-test rpki: listening on %s:%s (valid-asn=%d, invalid-asn=%d)\n",
		bind, portStr, srv.validASN, srv.invalidASN)

	for {
		conn, err := ln.Accept()
		if err != nil {
			return 0
		}
		go srv.handleConn(conn)
	}
}

func (s *rpkiServer) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()

	header := make([]byte, rpkiPDUHeaderLen)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return
		}

		pduType := header[1]
		pduLen := binary.BigEndian.Uint32(header[4:8])

		remaining := int(pduLen) - rpkiPDUHeaderLen
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
		case 2, 1:
			if err := s.sendResponse(conn); err != nil {
				return
			}
		}
	}
}

func (s *rpkiServer) sendResponse(conn net.Conn) error {
	sessionID := uint16(1)

	cr := make([]byte, rpkiPDUHeaderLen)
	cr[0] = rpkiRTRVersion1
	cr[1] = rpkiPDUCacheResp
	binary.BigEndian.PutUint16(cr[2:4], sessionID)
	binary.BigEndian.PutUint32(cr[4:8], rpkiPDUHeaderLen)
	if _, err := conn.Write(cr); err != nil {
		return err
	}

	for octet := range 256 {
		mod := octet % 3
		if mod == 2 {
			continue
		}

		asn := s.validASN
		if mod == 1 {
			asn = s.invalidASN
		}

		if err := s.sendIPv4PrefixPDU(conn, byte(octet), asn); err != nil {
			return err
		}
	}

	eod := make([]byte, 24)
	eod[0] = rpkiRTRVersion1
	eod[1] = rpkiPDUEndOfData
	binary.BigEndian.PutUint16(eod[2:4], sessionID)
	binary.BigEndian.PutUint32(eod[4:8], 24)
	binary.BigEndian.PutUint32(eod[8:12], s.serial)
	binary.BigEndian.PutUint32(eod[12:16], 3600)
	binary.BigEndian.PutUint32(eod[16:20], 600)
	binary.BigEndian.PutUint32(eod[20:24], 7200)
	_, err := conn.Write(eod)
	return err
}

func (s *rpkiServer) sendIPv4PrefixPDU(conn net.Conn, octet byte, asn uint32) error {
	pdu := make([]byte, 20)
	pdu[0] = rpkiRTRVersion1
	pdu[1] = rpkiPDUIPv4Prefix
	binary.BigEndian.PutUint32(pdu[4:8], 20)
	pdu[8] = 1
	pdu[9] = 8
	pdu[10] = 32
	pdu[12] = octet
	binary.BigEndian.PutUint32(pdu[16:20], asn)
	_, err := conn.Write(pdu)
	return err
}
