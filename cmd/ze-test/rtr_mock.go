// Design: docs/architecture/testing/ci-format.md -- mock RTR cache server for RPKI testing
//
// ze-test rtr-mock is a lightweight RTR (RFC 8210) cache server for functional tests.
// It listens on TCP, accepts Reset Query or Serial Query PDUs, and responds
// with Cache Response + configured VRPs + End of Data.
//
// Usage:
//
//	ze-test rtr-mock --port 3323 --vrp 10.0.0.0/8,24,65001 --vrp 192.168.0.0/16,24,65002
package main

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
)

// RTR PDU types (RFC 8210 Section 5).
const (
	rtrMockPDUSerialQuery = 1
	rtrMockPDUResetQuery  = 2
	rtrMockPDUCacheResp   = 3
	rtrMockPDUIPv4Prefix  = 4
	rtrMockPDUIPv6Prefix  = 6
	rtrMockPDUEndOfData   = 7

	rtrMockVersion1  = 1
	rtrMockHeaderLen = 8
)

// vrpFlag describes a VRP entry from command-line flags.
type vrpFlag struct {
	prefix    net.IPNet
	maxLength uint8
	asn       uint32
}

type vrpList []vrpFlag

func (v *vrpList) String() string { return fmt.Sprintf("%d VRPs", len(*v)) }

func (v *vrpList) Set(s string) error {
	// Format: prefix,maxlen,asn (e.g. "10.0.0.0/8,24,65001")
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

func rtrMockCmd() int {
	var (
		port   int
		serial uint32
		vrps   vrpList
	)

	fs := flag.NewFlagSet("ze-test rtr-mock", flag.ExitOnError)
	fs.IntVar(&port, "port", 0, "TCP listen port (0 = auto)")
	fs.Var(&vrps, "vrp", "VRP entry: prefix,maxlen,asn (repeatable)")
	serialFlag := fs.Uint("serial", 1, "initial serial number")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ze-test rtr-mock [flags]\n\nMock RTR cache server for RPKI testing.\n\nFlags:\n")
		fs.PrintDefaults()
	}

	if err := fs.Parse(os.Args[1:]); err != nil {
		return 1
	}
	serial = uint32(*serialFlag) //nolint:gosec // serial number fits uint32

	lc := &net.ListenConfig{}
	ln, err := lc.Listen(context.Background(), "tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: listen: %v\n", err)
		return 1
	}
	defer func() { _ = ln.Close() }()

	// Print the actual port for test infrastructure to discover.
	_, portStr, _ := net.SplitHostPort(ln.Addr().String())
	fmt.Fprintf(os.Stderr, "ze-test rtr-mock: listening on port %s with %d VRPs\n", portStr, len(vrps))

	for {
		conn, err := ln.Accept()
		if err != nil {
			return 0 // Listener closed
		}
		go rtrMockHandleConn(conn, vrps, serial)
	}
}

func rtrMockHandleConn(conn net.Conn, vrps vrpList, serial uint32) {
	defer func() { _ = conn.Close() }()

	header := make([]byte, rtrMockHeaderLen)
	for {
		if _, err := io.ReadFull(conn, header); err != nil {
			return // Connection closed
		}

		pduType := header[1]
		pduLen := binary.BigEndian.Uint32(header[4:8])

		// Read remaining bytes if any.
		// Cap to prevent unbounded allocation from malformed PDU length.
		remaining := int(pduLen) - rtrMockHeaderLen
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
		case rtrMockPDUResetQuery, rtrMockPDUSerialQuery:
			if err := rtrMockSendResponse(conn, vrps, serial); err != nil {
				return
			}
		default:
			// Ignore unknown PDUs
		}
	}
}

func rtrMockSendResponse(conn net.Conn, vrps vrpList, serial uint32) error {
	sessionID := uint16(1)

	// Cache Response PDU
	cr := make([]byte, rtrMockHeaderLen)
	cr[0] = rtrMockVersion1
	cr[1] = rtrMockPDUCacheResp
	binary.BigEndian.PutUint16(cr[2:4], sessionID)
	binary.BigEndian.PutUint32(cr[4:8], rtrMockHeaderLen)
	if _, err := conn.Write(cr); err != nil {
		return err
	}

	// Send each VRP as IPv4 or IPv6 Prefix PDU
	for _, vrp := range vrps {
		if err := rtrMockSendPrefixPDU(conn, vrp); err != nil {
			return err
		}
	}

	// End of Data PDU (version 1: 24 bytes)
	eod := make([]byte, 24)
	eod[0] = rtrMockVersion1
	eod[1] = rtrMockPDUEndOfData
	binary.BigEndian.PutUint16(eod[2:4], sessionID)
	binary.BigEndian.PutUint32(eod[4:8], 24)
	binary.BigEndian.PutUint32(eod[8:12], serial)
	binary.BigEndian.PutUint32(eod[12:16], 3600) // refresh interval
	binary.BigEndian.PutUint32(eod[16:20], 600)  // retry interval
	binary.BigEndian.PutUint32(eod[20:24], 7200) // expire interval
	if _, err := conn.Write(eod); err != nil {
		return err
	}

	return nil
}

func rtrMockSendPrefixPDU(conn net.Conn, vrp vrpFlag) error {
	ip4 := vrp.prefix.IP.To4()
	if ip4 != nil {
		// IPv4 Prefix PDU: 20 bytes
		pdu := make([]byte, 20)
		pdu[0] = rtrMockVersion1
		pdu[1] = rtrMockPDUIPv4Prefix
		// bytes 2-3: zero (reserved)
		binary.BigEndian.PutUint32(pdu[4:8], 20) // length
		pdu[8] = 1                               // flags: announce
		prefixLen, _ := vrp.prefix.Mask.Size()
		pdu[9] = byte(prefixLen) //nolint:gosec // prefixLen 0-32
		pdu[10] = vrp.maxLength
		// pdu[11] = 0 // zero
		copy(pdu[12:16], ip4)
		binary.BigEndian.PutUint32(pdu[16:20], vrp.asn)
		_, err := conn.Write(pdu)
		return err
	}

	// IPv6 Prefix PDU: 32 bytes
	ip6 := vrp.prefix.IP.To16()
	pdu := make([]byte, 32)
	pdu[0] = rtrMockVersion1
	pdu[1] = rtrMockPDUIPv6Prefix
	binary.BigEndian.PutUint32(pdu[4:8], 32) // length
	pdu[8] = 1                               // flags: announce
	prefixLen, _ := vrp.prefix.Mask.Size()
	pdu[9] = byte(prefixLen) //nolint:gosec // prefixLen 0-128
	pdu[10] = vrp.maxLength
	copy(pdu[12:28], ip6)
	binary.BigEndian.PutUint32(pdu[28:32], vrp.asn)
	_, err := conn.Write(pdu)
	return err
}
