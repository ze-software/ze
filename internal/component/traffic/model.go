// Design: docs/architecture/core-design.md -- Traffic control data model types
// Related: backend.go -- Backend interface consuming these types

// Package traffic defines the data model for ze-managed tc (traffic control)
// qdiscs, classes, and filters. The trafficnetlink backend translates these
// types to vishvananda/netlink tc calls. The trafficvpp backend translates
// them to VPP policer and scheduler APIs.
package traffic

import (
	"errors"
	"fmt"
)

var errTrafficRateMustBe1Got = errors.New("traffic: rate must be >= 1, got 0")

const unknownStr = "unknown"

// ValidateRate checks that a rate value is at least 1 bps.
func ValidateRate(rate uint64) error {
	if rate == 0 {
		return errTrafficRateMustBe1Got
	}
	return nil
}

// validateCeil checks that ceil is >= rate.
func validateCeil(rate, ceil uint64) error {
	if ceil < rate {
		return fmt.Errorf("traffic: ceil (%d) must be >= rate (%d)", ceil, rate)
	}
	return nil
}

// --- Enums ---

// QdiscType identifies the queueing discipline type.
type QdiscType uint8

const (
	qdiscUnknown QdiscType = iota
	QdiscHTB               // htb (Hierarchical Token Bucket)
	QdiscHFSC              // hfsc (Hierarchical Fair Service Curve)
	QdiscFQ                // fq (Fair Queue)
	QdiscFQCodel           // fq_codel (Fair Queue Controlled Delay)
	QdiscSFQ               // sfq (Stochastic Fair Queue)
	QdiscTBF               // tbf (Token Bucket Filter)
	QdiscNetem             // netem (Network Emulator)
	QdiscPrio              // prio (Priority)
	QdiscClsact            // clsact (Classifier Action)
	QdiscIngress           // ingress
)

var qdiscTypeNames = map[QdiscType]string{
	QdiscHTB:     "htb",
	QdiscHFSC:    "hfsc",
	QdiscFQ:      "fq",
	QdiscFQCodel: "fq_codel",
	QdiscSFQ:     "sfq",
	QdiscTBF:     "tbf",
	QdiscNetem:   "netem",
	QdiscPrio:    "prio",
	QdiscClsact:  "clsact",
	QdiscIngress: "ingress",
}

// qdiscTypeByName parses a configured name. It carries no clsact or ingress
// row, because neither is a discipline an operator can ask for: they attach at
// the ingress hook rather than at the root, and the mirror and sampling paths
// own that hook. The two names stay in qdiscTypeNames above, which is the
// other direction: naming a qdisc this backend READS off an interface.
var qdiscTypeByName = map[string]QdiscType{
	"htb":      QdiscHTB,
	"hfsc":     QdiscHFSC,
	"fq":       QdiscFQ,
	"fq_codel": QdiscFQCodel,
	"sfq":      QdiscSFQ,
	"tbf":      QdiscTBF,
	"netem":    QdiscNetem,
	"prio":     QdiscPrio,
}

func (q QdiscType) String() string {
	if name, ok := qdiscTypeNames[q]; ok {
		return name
	}
	return unknownStr
}

func (q QdiscType) Valid() bool {
	_, ok := qdiscTypeNames[q]
	return ok
}

// ParseQdiscType returns the QdiscType for a name.
func ParseQdiscType(name string) (QdiscType, bool) {
	q, ok := qdiscTypeByName[name]
	if !ok {
		return qdiscUnknown, false
	}
	return q, true
}

// FilterType identifies the type of traffic filter for class matching.
type FilterType uint8

const (
	filterUnknown  FilterType = iota
	FilterMark                // fw mark
	FilterDSCP                // dscp value
	FilterProtocol            // protocol
)

var filterTypeNames = map[FilterType]string{
	FilterMark:     "mark",
	FilterDSCP:     "dscp",
	FilterProtocol: "protocol",
}

var filterTypeByName = map[string]FilterType{
	"mark":     FilterMark,
	"dscp":     FilterDSCP,
	"protocol": FilterProtocol,
}

func (f FilterType) String() string {
	if name, ok := filterTypeNames[f]; ok {
		return name
	}
	return unknownStr
}

func (f FilterType) Valid() bool {
	_, ok := filterTypeNames[f]
	return ok
}

// parseFilterType returns the FilterType for a name.
func parseFilterType(name string) (FilterType, bool) {
	ft, ok := filterTypeByName[name]
	if !ok {
		return filterUnknown, false
	}
	return ft, true
}

// --- Composite types ---

// InterfaceQoS holds the complete QoS configuration for one interface.
//
// Qdisc programs the EGRESS direction, which is what leaves the interface. On a
// subscriber interface that is the download direction. Ingress programs the
// other one: traffic arriving on the interface, which is the subscriber's
// upload. The two need different mechanisms because they are different
// problems. Egress owns a queue, so it can shape by delaying a packet. Ingress
// has no queue to delay into, so the only enforcement available there is to
// drop what exceeds the rate, which is what a policer does.
type InterfaceQoS struct {
	Interface string
	Qdisc     Qdisc
	Ingress   Policer
}

// Policer is a token-bucket rate limiter on an interface's ingress hook.
//
// A zero Policer means the interface asks for no ingress enforcement, and Set
// is the name of that distinction. The zero is safe to read that way because
// ValidateRate already refuses a zero rate, so no operator can configure one:
// there is no legitimate zero for a caller to confuse with an absent policer.
type Policer struct {
	// RateBps is the policed rate in bits per second. Traffic above it is
	// dropped.
	RateBps uint64
	// BurstBytes is the depth of the token bucket in bytes. It MUST be large
	// enough to hold one full packet: a bucket shallower than the packet can
	// never fill enough to admit one, so every packet exceeds and the policer
	// drops the whole flow. NewPolicer sizes it; Validate refuses a rate
	// carrying no burst.
	BurstBytes uint64
}

// burstWindowMs is the window a policer's burst absorbs at the policed rate.
// 100ms of traffic rides out a brief spike without letting the long-term rate
// exceed the configured one.
const burstWindowMs = 100

// burstBytesFloor is the smallest burst any policer gets. A standard Ethernet
// MTU is 1500 bytes and this rounds up to 2048 to leave room for VLAN or tunnel
// encapsulation. Below roughly 160 kbit/s the window alone yields less than one
// packet, and a bucket that small drops everything.
const burstBytesFloor = 2048

// bitsPerByte converts the configured bit rate to the byte-denominated bucket
// depth the kernel and VPP both take.
const bitsPerByte = 8

// millisecondsPerSecond scales the burst window into the rate's own unit.
const millisecondsPerSecond = 1000

// PolicerBurstBytes returns the token-bucket depth for a policed rate:
//
//	bytes = rateBps / 8 * (burstWindowMs / 1000)
//
// The floor applies below roughly 160 kbit/s, where the window alone yields
// less than one packet. This is the one declaration of that derivation; the tc
// and VPP backends both consume it, so the two cannot drift.
func PolicerBurstBytes(rateBps uint64) uint64 {
	b := rateBps * burstWindowMs / bitsPerByte / millisecondsPerSecond
	if b < burstBytesFloor {
		return burstBytesFloor
	}
	return b
}

// NewPolicer builds a policer for a rate, sizing the burst with
// PolicerBurstBytes. Callers use this rather than the struct literal so no call
// site can leave the burst at zero, which the kernel accepts and which drops
// every packet.
func NewPolicer(rateBps uint64) Policer {
	return Policer{RateBps: rateBps, BurstBytes: PolicerBurstBytes(rateBps)}
}

// Set reports whether this policer asks for ingress enforcement. An absent
// policer is the zero value, and a configured one always carries a rate.
func (p Policer) Set() bool {
	return p.RateBps > 0
}

// Validate refuses a policer a backend must not program. An absent one is
// valid: it asks for nothing.
func (p Policer) Validate() error {
	if !p.Set() {
		return nil
	}
	if p.BurstBytes < burstBytesFloor {
		return fmt.Errorf("traffic: policer burst (%d bytes) must be >= %d; use NewPolicer to size it", p.BurstBytes, burstBytesFloor)
	}
	return nil
}

// Qdisc represents a queueing discipline with its classes and default class.
type Qdisc struct {
	Type         QdiscType
	DefaultClass string // name of the default class
	Classes      []TrafficClass
}

// TrafficClass represents one class within a classful qdisc.
type TrafficClass struct {
	Name     string
	Rate     uint64 // guaranteed rate in bps
	Ceil     uint64 // maximum rate in bps (>= Rate)
	Priority uint8  // scheduling priority (lower = higher priority)
	Filters  []TrafficFilter
}

// TrafficFilter matches packets to a class.
type TrafficFilter struct {
	Type  FilterType
	Value uint32 // mark value, dscp value, or protocol number
}
