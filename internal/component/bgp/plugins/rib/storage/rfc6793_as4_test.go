// RFC: rfc/short/rfc6793.md — receiving AS4_PATH / AS4_AGGREGATOR from an OLD speaker
//
// Drives ParseAttributes (attrparse.go), the ingest path that turns a received
// UPDATE's attribute bytes into the interned RIB entry.

package storage

var (
	// AS_PATH from an OLD speaker: AS_SEQUENCE [65001, AS_TRANS] in two octets.
	// Flags 0x40, type 2, length 6.
	rfc6793WireASPathWithASTrans = []byte{
		0x40, 0x02, 0x06,
		0x02, 0x02, 0xFD, 0xE9, 0x5B, 0xA0,
	}

	// AS4_PATH alongside it: AS_SEQUENCE [65001, 4200000001] in four octets.
	// Flags 0xC0 (optional transitive), type 17, length 10.
	rfc6793WireAS4Path = []byte{
		0xC0, 0x11, 0x0A,
		0x02, 0x02, 0x00, 0x00, 0xFD, 0xE9, 0xFA, 0x56, 0xEA, 0x01,
	}

	// AGGREGATOR from an OLD speaker: two-octet AS_TRANS + 10.0.0.1.
	rfc6793WireAggregatorASTrans = []byte{
		0xC0, 0x07, 0x06,
		0x5B, 0xA0, 0x0A, 0x00, 0x00, 0x01,
	}

	// AS4_AGGREGATOR: four-octet 4200000001 + 10.0.0.1.
	rfc6793WireAS4Aggregator = []byte{
		0xC0, 0x12, 0x08,
		0xFA, 0x56, 0xEA, 0x01, 0x0A, 0x00, 0x00, 0x01,
	}
)
