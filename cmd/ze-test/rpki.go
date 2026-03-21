// Design: docs/architecture/testing/ci-format.md -- deterministic RPKI mock server
//
// ze-test rpki is a deterministic RTR (RFC 8210) cache server for functional tests.
// It auto-generates VRPs based on IP prefix modulo, so validation state is predictable:
//
//	first octet % 3 == 0 → VRP with ASN 65001, maxLen /32 → Valid (for routes from AS 65001)
//	first octet % 3 == 1 → VRP with ASN 65099, maxLen /32 → Invalid (wrong ASN for AS 65001)
//	first octet % 3 == 2 → no VRP                         → NotFound
//
// Usage:
//
//	ze-test rpki --port 3323
//	ze-test rpki --port 3323 --valid-asn 65001 --invalid-asn 65099
package main

import (
	"context"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
)

// RTR PDU types (RFC 8210 Section 5).
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

func rpkiCmd() int {
	fs := flag.NewFlagSet("ze-test rpki", flag.ExitOnError)

	var (
		port       int
		validASN   uint
		invalidASN uint
		serial     uint
	)

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

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}

	srv := &rpkiServer{
		validASN:   uint32(validASN),   //nolint:gosec // CLI flag range
		invalidASN: uint32(invalidASN), //nolint:gosec // CLI flag range
		serial:     uint32(serial),     //nolint:gosec // CLI flag range
	}

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "ze-test rpki: listening on port %s (valid-asn=%d, invalid-asn=%d)\n",
		portStr, srv.validASN, srv.invalidASN)

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

		// Cap remaining bytes to prevent unbounded allocation from malformed PDU length.
		// Largest valid RTR PDU is well under 4096 bytes.
		remaining := int(pduLen) - rpkiPDUHeaderLen
		if remaining > 4096 {
			return // Malformed PDU, close connection.
		}
		if remaining > 0 {
			discard := make([]byte, remaining)
			if _, err := io.ReadFull(conn, discard); err != nil {
				return
			}
		}

		switch pduType {
		case 2, 1: // 2 = Reset Query, 1 = Serial Query (RFC 8210)
			if err := s.sendResponse(conn); err != nil {
				return
			}
		}
	}
}

func (s *rpkiServer) sendResponse(conn net.Conn) error {
	sessionID := uint16(1)

	// Cache Response PDU.
	cr := make([]byte, rpkiPDUHeaderLen)
	cr[0] = rpkiRTRVersion1
	cr[1] = rpkiPDUCacheResp
	binary.BigEndian.PutUint16(cr[2:4], sessionID)
	binary.BigEndian.PutUint32(cr[4:8], rpkiPDUHeaderLen)
	if _, err := conn.Write(cr); err != nil {
		return err
	}

	// Generate VRPs for all /8 prefixes based on first octet modulo 3.
	for octet := range 256 {
		mod := octet % 3
		if mod == 2 {
			continue // NotFound: no VRP for this /8.
		}

		asn := s.validASN
		if mod == 1 {
			asn = s.invalidASN
		}

		if err := s.sendIPv4PrefixPDU(conn, byte(octet), asn); err != nil {
			return err
		}
	}

	// End of Data PDU (version 1: 24 bytes).
	eod := make([]byte, 24)
	eod[0] = rpkiRTRVersion1
	eod[1] = rpkiPDUEndOfData
	binary.BigEndian.PutUint16(eod[2:4], sessionID)
	binary.BigEndian.PutUint32(eod[4:8], 24)
	binary.BigEndian.PutUint32(eod[8:12], s.serial)
	binary.BigEndian.PutUint32(eod[12:16], 3600) // refresh interval
	binary.BigEndian.PutUint32(eod[16:20], 600)  // retry interval
	binary.BigEndian.PutUint32(eod[20:24], 7200) // expire interval
	_, err := conn.Write(eod)
	return err
}

// sendIPv4PrefixPDU sends an IPv4 Prefix PDU for octet.0.0.0/8 with maxLen=32.
func (s *rpkiServer) sendIPv4PrefixPDU(conn net.Conn, octet byte, asn uint32) error {
	pdu := make([]byte, 20)
	pdu[0] = rpkiRTRVersion1
	pdu[1] = rpkiPDUIPv4Prefix
	binary.BigEndian.PutUint32(pdu[4:8], 20) // length
	pdu[8] = 1                               // flags: announce
	pdu[9] = 8                               // prefix length: /8
	pdu[10] = 32                             // max length: /32
	pdu[12] = octet                          // IP: octet.0.0.0
	binary.BigEndian.PutUint32(pdu[16:20], asn)
	_, err := conn.Write(pdu)
	return err
}
