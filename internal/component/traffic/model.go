// Design: docs/architecture/core-design.md -- Traffic control data model types
// Related: backend.go -- Backend interface consuming these types

// Package traffic defines the data model for ze-managed tc (traffic control)
// qdiscs, classes, and filters. The trafficnetlink backend translates these
// types to vishvananda/netlink tc calls. The trafficvpp backend translates
// them to VPP policer and scheduler APIs.
package traffic

import "fmt"

const unknownStr = "unknown"

// ValidateRate checks that a rate value is at least 1 bps.
func ValidateRate(rate uint64) error {
	if rate == 0 {
		return fmt.Errorf("traffic: rate must be >= 1, got 0")
	}
	return nil
}

// ValidateCeil checks that ceil is >= rate.
func ValidateCeil(rate, ceil uint64) error {
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

var qdiscTypeByName = map[string]QdiscType{
	"htb":      QdiscHTB,
	"hfsc":     QdiscHFSC,
	"fq":       QdiscFQ,
	"fq_codel": QdiscFQCodel,
	"sfq":      QdiscSFQ,
	"tbf":      QdiscTBF,
	"netem":    QdiscNetem,
	"prio":     QdiscPrio,
	"clsact":   QdiscClsact,
	"ingress":  QdiscIngress,
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

// ParseFilterType returns the FilterType for a name.
func ParseFilterType(name string) (FilterType, bool) {
	ft, ok := filterTypeByName[name]
	if !ok {
		return filterUnknown, false
	}
	return ft, true
}

// --- Composite types ---

// InterfaceQoS holds the complete QoS configuration for one interface.
type InterfaceQoS struct {
	Interface string
	Qdisc     Qdisc
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
