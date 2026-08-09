// Design: docs/architecture/ospf/ospf-af-unify.md -- the Hello body is shared via the types leaf as a
// superset of the OSPFv2 and OSPFv3 fields; the version-specific wire encode/decode lives
// in the codec. OSPFv2 uses NetworkMask + IP-address DR/BDR; OSPFv3 uses InterfaceID +
// Router-ID DR/BDR (RFC 5340 sec A.3.2). The engine's neighbor FSM reads the common fields
// and applies AF-aware checks (the Network Mask match is OSPFv2-only).
// RFC: rfc/short/rfc5340.md (§A.3.2 Hello), rfc/short/rfc5838.md (§2.4 AF-bit)

package types

// Hello is the OSPF Hello packet body (superset of OSPFv2 RFC 2328 A.3.2 and OSPFv3
// RFC 5340 A.3.2). NetworkMask is OSPFv2-only; InterfaceID is OSPFv3-only; DR/BDR hold an
// IP address (v2) or a Router ID (v6), both 4 octets.
type Hello struct {
	NetworkMask   [4]byte
	InterfaceID   uint32
	HelloInterval uint16
	Options       Options
	Priority      uint8
	DeadInterval  uint32
	DR            [4]byte
	BDR           [4]byte
	Neighbors     []RouterID
	// AFBit carries the OSPFv3 AF-bit (RFC 5838 §2.4), which the neutral 8-bit Options
	// superset cannot represent. OSPFv3-only: the v6 codec sets it from the received
	// 24-bit Options; the OSPFv2 codec leaves it false. The engine's AF-bit adjacency
	// gate (§2.5/§2.6) reads it for non-default address families.
	AFBit bool
}
