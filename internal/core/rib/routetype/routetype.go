// Design: docs/architecture/api/process-protocol.md -- the FIB forwarding action
// Related: ../locrib/candidate.go -- carries it on a Loc-RIB Path
// Related: ../../bgp/ribevents/ribevents.go -- carries it on a BGP best-path change

// Package routetype holds the forwarding action a FIB entry takes. It is the
// one definition of that vocabulary.
//
// It sits in internal/core because three packages on two tiers name it. The
// Loc-RIB Path and the BGP best-path change produce it, and both are core. The
// sysrib event contract republishes it to the FIB plugins, and that is a
// component. A component-tier definition is unreachable from the two core
// producers, and a second definition lets the two drift.
package routetype

// Type identifies the forwarding action for a FIB entry. Values match the
// Linux RTN_ constants so the kernel backend maps them without a table.
type Type uint8

// The forwarding actions. Zero is "unset". A producer that says nothing leaves
// the consumer to apply its own default, which is a unicast route. Unicast is
// therefore 1 rather than 0. That keeps "the producer chose unicast" and "the
// producer said nothing" distinguishable.
const (
	Unicast     Type = 1
	Blackhole   Type = 6
	Unreachable Type = 7
	Prohibit    Type = 8
)

// Discards reports whether t drops the traffic it matches rather than
// forwarding it. It is true for every action that answers a packet with
// something other than a next-hop, which is what RFC 7999 Section 2 asks a
// BLACKHOLE-tagged prefix to do.
func (t Type) Discards() bool {
	return t == Blackhole || t == Unreachable || t == Prohibit
}

// String renders the action for logs and display. An unset or unrecognized
// value renders as "unset" rather than a number, because the number is the
// Linux constant and means nothing to a reader.
func (t Type) String() string {
	switch t {
	case Unicast:
		return "unicast"
	case Blackhole:
		return "blackhole"
	case Unreachable:
		return "unreachable"
	case Prohibit:
		return "prohibit"
	}
	return "unset"
}
