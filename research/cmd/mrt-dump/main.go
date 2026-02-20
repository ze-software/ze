// Design: (none — research tool)
//
// mrt-dump reads MRT RIB dump files and outputs each route as BGP UPDATE hex
package main

import (
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// MRT types (RFC 6396).
const (
	MRT_TABLE_DUMP_V2 = 13
	MRT_BGP4MP        = 16
	MRT_BGP4MP_ET     = 17
)

// TABLE_DUMP_V2 subtypes.
const (
	PEER_INDEX_TABLE   = 1
	RIB_IPV4_UNICAST   = 2
	RIB_IPV4_MULTICAST = 3
	RIB_IPV6_UNICAST   = 4
	RIB_IPV6_MULTICAST = 5
	RIB_GENERIC        = 6
)

// BGP4MP subtypes.
const (
	BGP4MP_MESSAGE           = 1
	BGP4MP_MESSAGE_AS4       = 4
	BGP4MP_MESSAGE_LOCAL     = 6
	BGP4MP_MESSAGE_AS4_LOCAL = 7
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <file.gz> [file2.gz ...]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Reads MRT RIB dumps and outputs each route as BGP UPDATE hex (one per line)\n")
		os.Exit(1)
	}

	for _, filename := range os.Args[1:] {
		if err := processMRT(filename); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", filename, err)
			os.Exit(1)
		}
	}
}

func processMRT(filename string) error {
	f, err := os.Open(filename) // #nosec G304 -- filename from CLI args
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	header := make([]byte, 12)
	for {
		// MRT common header: timestamp(4) + type(2) + subtype(2) + length(4)
		_, err := io.ReadFull(gz, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading header: %w", err)
		}

		mrtType := binary.BigEndian.Uint16(header[4:6])
		subtype := binary.BigEndian.Uint16(header[6:8])
		length := binary.BigEndian.Uint32(header[8:12])

		data := make([]byte, length)
		_, err = io.ReadFull(gz, data)
		if err != nil {
			return fmt.Errorf("reading data: %w", err)
		}

		switch mrtType {
		case MRT_TABLE_DUMP_V2:
			if err := processTableDumpV2(subtype, data); err != nil {
				return err
			}
		case MRT_BGP4MP, MRT_BGP4MP_ET:
			offset := 0
			if mrtType == MRT_BGP4MP_ET {
				offset = 4 // skip microseconds
			}
			if err := processBGP4MP(subtype, data[offset:]); err != nil {
				return err
			}
		}
	}
	return nil
}

func processTableDumpV2(subtype uint16, data []byte) error {
	switch subtype {
	case PEER_INDEX_TABLE:
		// Skip peer index table - it's metadata
		return nil
	case RIB_IPV4_UNICAST, RIB_IPV4_MULTICAST:
		return processRIBEntry(data, 4) // IPv4
	case RIB_IPV6_UNICAST, RIB_IPV6_MULTICAST:
		return processRIBEntry(data, 6) // IPv6
	case RIB_GENERIC:
		return processRIBGeneric(data)
	}
	return nil
}

func processRIBEntry(data []byte, _ int) error {
	if len(data) < 7 {
		return nil
	}

	// RIB entry: sequence(4) + prefix_len(1) + prefix(var) + entry_count(2) + entries...
	// sequence := binary.BigEndian.Uint32(data[0:4])
	prefixLen := data[4]
	prefixBytes := (int(prefixLen) + 7) / 8

	offset := 5 + prefixBytes
	if offset+2 > len(data) {
		return nil
	}

	// Build NLRI bytes
	nlri := make([]byte, 1+prefixBytes)
	nlri[0] = prefixLen
	copy(nlri[1:], data[5:5+prefixBytes])

	entryCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Process each RIB entry (one per peer)
	for range entryCount {
		if offset+8 > len(data) {
			break
		}

		// RIB entry: peer_index(2) + originated_time(4) + attr_len(2) + attrs(var)
		// peerIndex := binary.BigEndian.Uint16(data[offset : offset+2])
		// originatedTime := binary.BigEndian.Uint32(data[offset+2 : offset+6])
		attrLen := binary.BigEndian.Uint16(data[offset+6 : offset+8])
		offset += 8

		if offset+int(attrLen) > len(data) {
			break
		}

		attrs := data[offset : offset+int(attrLen)]
		offset += int(attrLen)

		// Build UPDATE message body:
		// withdrawn_len(2) + withdrawn(0) + attrs_len(2) + attrs + nlri
		update := buildUpdate(nil, attrs, nlri)
		fmt.Println(hex.EncodeToString(update))
	}

	return nil
}

func processRIBGeneric(data []byte) error {
	if len(data) < 11 {
		return nil
	}

	// RIB_GENERIC: sequence(4) + afi(2) + safi(1) + nlri_len(1) + nlri(var) + entry_count(2) + entries...
	// sequence := binary.BigEndian.Uint32(data[0:4])
	// afi := binary.BigEndian.Uint16(data[4:6])
	// safi := data[6]
	nlriLen := data[7]

	offset := 8 + int(nlriLen)
	if offset+2 > len(data) {
		return nil
	}

	nlri := data[8 : 8+int(nlriLen)]

	entryCount := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	for range entryCount {
		if offset+8 > len(data) {
			break
		}

		attrLen := binary.BigEndian.Uint16(data[offset+6 : offset+8])
		offset += 8

		if offset+int(attrLen) > len(data) {
			break
		}

		attrs := data[offset : offset+int(attrLen)]
		offset += int(attrLen)

		update := buildUpdate(nil, attrs, nlri)
		fmt.Println(hex.EncodeToString(update))
	}

	return nil
}

func processBGP4MP(subtype uint16, data []byte) error { //nolint:unparam
	// BGP4MP contains actual BGP messages - extract UPDATE messages
	var offset int
	var asSize int

	switch subtype {
	case BGP4MP_MESSAGE, BGP4MP_MESSAGE_LOCAL:
		asSize = 2
	case BGP4MP_MESSAGE_AS4, BGP4MP_MESSAGE_AS4_LOCAL:
		asSize = 4
	default:
		return nil // Skip state changes
	}

	if len(data) < asSize*2+4 {
		return nil
	}

	// peer_as + local_as + iface(2) + afi(2)
	offset = asSize*2 + 4
	if offset > len(data) {
		return nil
	}

	afi := binary.BigEndian.Uint16(data[asSize*2+2 : asSize*2+4])

	// peer_ip + local_ip
	ipSize := 4
	if afi == 2 {
		ipSize = 16
	}
	offset += ipSize * 2

	if offset+19 > len(data) {
		return nil
	}

	// BGP message starts here: marker(16) + length(2) + type(1) + body
	// Skip marker
	offset += 16
	msgLen := binary.BigEndian.Uint16(data[offset : offset+2])
	msgType := data[offset+2]
	offset += 3

	// Only process UPDATE messages (type 2)
	if msgType != 2 {
		return nil
	}

	bodyLen := int(msgLen) - 19 // subtract header
	if offset+bodyLen > len(data) {
		return nil
	}

	// Output the UPDATE body as hex
	fmt.Println(hex.EncodeToString(data[offset : offset+bodyLen]))
	return nil
}

func buildUpdate(withdrawn, attrs, nlri []byte) []byte {
	// UPDATE format: withdrawn_len(2) + withdrawn + attrs_len(2) + attrs + nlri
	wdLen := len(withdrawn)
	attrLen := len(attrs)

	update := make([]byte, 2+wdLen+2+attrLen+len(nlri))

	binary.BigEndian.PutUint16(update[0:2], uint16(wdLen)) //nolint:gosec // wdLen < 4096
	copy(update[2:], withdrawn)

	binary.BigEndian.PutUint16(update[2+wdLen:], uint16(attrLen)) //nolint:gosec // attrLen < 4096
	copy(update[4+wdLen:], attrs)
	copy(update[4+wdLen+attrLen:], nlri)

	return update
}
