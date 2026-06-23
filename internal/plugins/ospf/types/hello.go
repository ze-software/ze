// Design: plan/spec-ospf-af-unify.md -- the Hello body is shared via the types leaf as a
// superset of the OSPFv2 and OSPFv3 fields; the version-specific wire encode/decode lives
// in the codec. OSPFv2 uses NetworkMask + IP-address DR/BDR; OSPFv3 uses InterfaceID +
// Router-ID DR/BDR (RFC 5340 sec A.3.2). The engine's neighbor FSM reads the common fields
// and applies AF-aware checks (the Network Mask match is OSPFv2-only).

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
}
