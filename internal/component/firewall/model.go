// Design: docs/architecture/core-design.md -- Firewall data model types
// Related: backend.go -- Backend interface consuming these types

// Package firewall defines the abstract data model for ze-managed nftables
// firewall tables. Types model firewall concepts (MatchSourceAddress, Accept,
// SetMark), not nftables register operations. The nft backend lowers abstract
// types to nftables expressions internally. The VPP backend maps them directly
// to ACL rules and policers.
package firewall

import (
	"fmt"
	"net/netip"
	"regexp"
)

const unknownStr = "unknown"

// nameRe matches valid identifiers: alphanumeric, hyphens, underscores.
var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

// maxNameLen is the nftables kernel limit for table/chain names (NFT_NAME_MAXLEN).
const maxNameLen = 255

// ValidateName checks that a name is a non-empty valid identifier within kernel limits.
func ValidateName(name string) error {
	if name == "" {
		return fmt.Errorf("firewall: name must not be empty")
	}
	if len(name) > maxNameLen {
		return fmt.Errorf("firewall: name %q exceeds maximum length %d", name, maxNameLen)
	}
	if !nameRe.MatchString(name) {
		return fmt.Errorf("firewall: invalid name %q (must be alphanumeric with hyphens/underscores)", name)
	}
	return nil
}

// ValidatePort checks that a port number is in 1-65535.
func ValidatePort(port uint16) error {
	if port == 0 {
		return fmt.Errorf("firewall: port must be 1-65535, got 0")
	}
	return nil
}

// ValidateRate checks that a rate value is at least 1.
func ValidateRate(rate uint64) error {
	if rate == 0 {
		return fmt.Errorf("firewall: rate must be >= 1, got 0")
	}
	return nil
}

// --- Enums ---

// TableFamily identifies the nftables address family for a table.
type TableFamily uint8

const (
	familyUnknown TableFamily = iota
	FamilyInet                // inet (dual-stack IPv4+IPv6)
	FamilyIP                  // ip (IPv4 only)
	FamilyIP6                 // ip6 (IPv6 only)
	FamilyARP                 // arp
	FamilyBridge              // bridge
	FamilyNetdev              // netdev
)

var familyNames = map[TableFamily]string{
	FamilyInet:   "inet",
	FamilyIP:     "ip",
	FamilyIP6:    "ip6",
	FamilyARP:    "arp",
	FamilyBridge: "bridge",
	FamilyNetdev: "netdev",
}

var familyByName = map[string]TableFamily{
	"inet":   FamilyInet,
	"ip":     FamilyIP,
	"ip6":    FamilyIP6,
	"arp":    FamilyARP,
	"bridge": FamilyBridge,
	"netdev": FamilyNetdev,
}

func (f TableFamily) String() string {
	if name, ok := familyNames[f]; ok {
		return name
	}
	return unknownStr
}

func (f TableFamily) Valid() bool {
	_, ok := familyNames[f]
	return ok
}

// ParseTableFamily returns the TableFamily for a name.
// Returns familyUnknown and false if the name is not recognized.
func ParseTableFamily(name string) (TableFamily, bool) {
	f, ok := familyByName[name]
	if !ok {
		return familyUnknown, false
	}
	return f, true
}

// ChainHook identifies the netfilter hook point for a base chain.
type ChainHook uint8

const (
	chainHookUnknown ChainHook = iota
	HookInput                  // input
	HookOutput                 // output
	HookForward                // forward
	HookPrerouting             // prerouting
	HookPostrouting            // postrouting
	HookIngress                // ingress
	HookEgress                 // egress
)

var chainHookNames = map[ChainHook]string{
	HookInput:       "input",
	HookOutput:      "output",
	HookForward:     "forward",
	HookPrerouting:  "prerouting",
	HookPostrouting: "postrouting",
	HookIngress:     "ingress",
	HookEgress:      "egress",
}

var chainHookByName = map[string]ChainHook{
	"input":       HookInput,
	"output":      HookOutput,
	"forward":     HookForward,
	"prerouting":  HookPrerouting,
	"postrouting": HookPostrouting,
	"ingress":     HookIngress,
	"egress":      HookEgress,
}

func (h ChainHook) String() string {
	if name, ok := chainHookNames[h]; ok {
		return name
	}
	return unknownStr
}

func (h ChainHook) Valid() bool {
	_, ok := chainHookNames[h]
	return ok
}

// ParseChainHook returns the ChainHook for a name.
func ParseChainHook(name string) (ChainHook, bool) {
	h, ok := chainHookByName[name]
	if !ok {
		return chainHookUnknown, false
	}
	return h, true
}

// ChainType identifies the chain type (filter, nat, route).
type ChainType uint8

const (
	chainTypeUnknown ChainType = iota
	ChainFilter                // filter
	ChainNAT                   // nat
	ChainRoute                 // route
)

var chainTypeNames = map[ChainType]string{
	ChainFilter: "filter",
	ChainNAT:    "nat",
	ChainRoute:  "route",
}

var chainTypeByName = map[string]ChainType{
	"filter": ChainFilter,
	"nat":    ChainNAT,
	"route":  ChainRoute,
}

func (c ChainType) String() string {
	if name, ok := chainTypeNames[c]; ok {
		return name
	}
	return unknownStr
}

func (c ChainType) Valid() bool {
	_, ok := chainTypeNames[c]
	return ok
}

// ParseChainType returns the ChainType for a name.
func ParseChainType(name string) (ChainType, bool) {
	ct, ok := chainTypeByName[name]
	if !ok {
		return chainTypeUnknown, false
	}
	return ct, true
}

// Policy identifies the default policy for a base chain.
type Policy uint8

const (
	policyUnknown Policy = iota
	PolicyAccept         // accept
	PolicyDrop           // drop
)

var policyNames = map[Policy]string{
	PolicyAccept: "accept",
	PolicyDrop:   "drop",
}

var policyByName = map[string]Policy{
	"accept": PolicyAccept,
	"drop":   PolicyDrop,
}

func (p Policy) String() string {
	if name, ok := policyNames[p]; ok {
		return name
	}
	return unknownStr
}

func (p Policy) Valid() bool {
	_, ok := policyNames[p]
	return ok
}

// ParsePolicy returns the Policy for a name.
func ParsePolicy(name string) (Policy, bool) {
	pol, ok := policyByName[name]
	if !ok {
		return policyUnknown, false
	}
	return pol, true
}

// ConnState is a bitmask for connection tracking states.
type ConnState uint8

const (
	ConnStateNew         ConnState = 1 << iota // new
	ConnStateEstablished                       // established
	ConnStateRelated                           // related
	ConnStateInvalid                           // invalid
)

// --- Match and Action interfaces ---

// Match marks a type as a firewall match expression (from block).
type Match interface {
	matchMarker()
}

// Action marks a type as a firewall action expression (then block).
type Action interface {
	actionMarker()
}

// --- Match types (18) ---

// MatchSourceAddress matches packets by source IP prefix.
type MatchSourceAddress struct{ Prefix netip.Prefix }

// MatchDestinationAddress matches packets by destination IP prefix.
type MatchDestinationAddress struct{ Prefix netip.Prefix }

// MatchSourcePort matches packets by source port or port range.
type MatchSourcePort struct {
	Port    uint16
	PortEnd uint16 // 0 = single port
}

// MatchDestinationPort matches packets by destination port or port range.
type MatchDestinationPort struct {
	Port    uint16
	PortEnd uint16 // 0 = single port
}

// MatchProtocol matches packets by L4 protocol name.
type MatchProtocol struct{ Protocol string }

// MatchInputInterface matches packets by input interface name.
type MatchInputInterface struct{ Name string }

// MatchOutputInterface matches packets by output interface name.
type MatchOutputInterface struct{ Name string }

// MatchConnState matches packets by connection tracking state bitmask.
type MatchConnState struct{ States ConnState }

// MatchConnMark matches packets by connection mark value and mask.
type MatchConnMark struct {
	Value uint32
	Mask  uint32
}

// MatchMark matches packets by packet mark value and mask.
type MatchMark struct {
	Value uint32
	Mask  uint32
}

// MatchDSCP matches packets by DSCP value (0-63).
type MatchDSCP struct{ Value uint8 }

// MatchConnBytes matches packets by cumulative connection byte count.
type MatchConnBytes struct {
	Bytes uint64
	Over  bool // true = over threshold, false = under
}

// MatchConnLimit matches packets by concurrent connection count.
type MatchConnLimit struct {
	Count uint32
	Flags uint32
}

// FibResult selects which FIB lookup result to check.
type FibResult uint8

const (
	FibResultOIF  FibResult = iota // output interface
	FibResultAddr                  // address type
)

// MatchFib matches using a FIB lookup result.
type MatchFib struct {
	Result FibResult
	Flags  uint32
}

// SocketKey selects which socket attribute to check.
type SocketKey uint8

const (
	SocketTransparent SocketKey = iota
	SocketMark
	SocketWildcard
)

// MatchSocket matches based on an associated socket attribute.
type MatchSocket struct {
	Key   SocketKey
	Level uint32
}

// RtKey selects which routing attribute to check.
type RtKey uint8

const (
	RtClassID RtKey = iota
	RtNexthop
	RtMTU
	RtTCPMSS
)

// MatchRt matches based on a routing attribute.
type MatchRt struct{ Key RtKey }

// MatchExtHdr matches an IPv6 extension header.
type MatchExtHdr struct {
	Type   uint8
	Field  uint32
	Offset uint32
}

// SetFieldType identifies which packet field is used for set lookups.
type SetFieldType uint8

const (
	SetFieldSourceAddr SetFieldType = iota
	SetFieldDestAddr
	SetFieldSourcePort
	SetFieldDestPort
)

// MatchInSet matches packets against a named set.
type MatchInSet struct {
	SetName    string
	MatchField SetFieldType
}

// Interface compliance: all 18 match types implement Match.
func (MatchSourceAddress) matchMarker()      {}
func (MatchDestinationAddress) matchMarker() {}
func (MatchSourcePort) matchMarker()         {}
func (MatchDestinationPort) matchMarker()    {}
func (MatchProtocol) matchMarker()           {}
func (MatchInputInterface) matchMarker()     {}
func (MatchOutputInterface) matchMarker()    {}
func (MatchConnState) matchMarker()          {}
func (MatchConnMark) matchMarker()           {}
func (MatchMark) matchMarker()               {}
func (MatchDSCP) matchMarker()               {}
func (MatchConnBytes) matchMarker()          {}
func (MatchConnLimit) matchMarker()          {}
func (MatchFib) matchMarker()                {}
func (MatchSocket) matchMarker()             {}
func (MatchRt) matchMarker()                 {}
func (MatchExtHdr) matchMarker()             {}
func (MatchInSet) matchMarker()              {}

// --- Action types (16) ---

// Accept terminates evaluation and accepts the packet.
type Accept struct{}

// Drop terminates evaluation and drops the packet.
type Drop struct{}

// Reject terminates evaluation and sends a reject response.
type Reject struct {
	Type string // e.g., "icmp", "icmpv6", "tcp-reset"
	Code uint8
}

// Jump transfers to a target chain, returning on completion.
type Jump struct{ Target string }

// Goto transfers to a target chain without return.
type Goto struct{ Target string }

// Return returns from the current chain to the caller.
type Return struct{}

// SNAT applies source NAT.
type SNAT struct {
	Address netip.Addr
	Port    uint16
	PortEnd uint16
	Flags   uint32
}

// DNAT applies destination NAT.
type DNAT struct {
	Address netip.Addr
	Port    uint16
	PortEnd uint16
	Flags   uint32
}

// Masquerade applies source NAT using the outgoing interface address.
type Masquerade struct {
	Port    uint16
	PortEnd uint16
	Flags   uint32
}

// Redirect redirects the packet to a local port.
type Redirect struct {
	Port  uint16
	Flags uint32
}

// Queue sends the packet to a userspace queue.
type Queue struct {
	Num   uint16
	Total uint16
	Flags uint32
}

// Notrack disables connection tracking for the packet.
type Notrack struct{}

// TProxy redirects the packet to a transparent proxy.
type TProxy struct {
	Address netip.Addr
	Port    uint16
}

// Duplicate sends a copy of the packet to another destination.
type Duplicate struct {
	Address netip.Addr
	Device  string
}

// FlowOffload offloads the connection to a flowtable for hardware acceleration.
type FlowOffload struct{ FlowtableName string }

// Synproxy handles TCP SYN proxying.
type Synproxy struct {
	MSS    uint16
	Wscale uint8
	Flags  uint32
}

// --- Modifier types (8, also implement Action) ---

// SetMark sets the packet mark.
type SetMark struct {
	Value uint32
	Mask  uint32
}

// SetConnMark sets the connection mark.
type SetConnMark struct {
	Value uint32
	Mask  uint32
}

// SetDSCP sets the DSCP field.
type SetDSCP struct{ Value uint8 }

// Counter increments a named or anonymous counter.
type Counter struct{ Name string }

// Log logs the packet with a prefix and severity level.
type Log struct {
	Prefix  string
	Level   uint32
	Group   uint16
	Snaplen uint32
}

// Limit applies a rate limit.
type Limit struct {
	Rate  uint64
	Unit  string // "second", "minute", "hour", "day"
	Over  bool
	Burst uint32
}

// Quota applies a byte quota.
type Quota struct {
	Bytes uint64
	Flags uint32
}

// SecMark applies a security mark.
type SecMark struct{ Name string }

// Interface compliance: all 16 action + 8 modifier types implement Action.
func (Accept) actionMarker()      {}
func (Drop) actionMarker()        {}
func (Reject) actionMarker()      {}
func (Jump) actionMarker()        {}
func (Goto) actionMarker()        {}
func (Return) actionMarker()      {}
func (SNAT) actionMarker()        {}
func (DNAT) actionMarker()        {}
func (Masquerade) actionMarker()  {}
func (Redirect) actionMarker()    {}
func (Queue) actionMarker()       {}
func (Notrack) actionMarker()     {}
func (TProxy) actionMarker()      {}
func (Duplicate) actionMarker()   {}
func (FlowOffload) actionMarker() {}
func (Synproxy) actionMarker()    {}
func (SetMark) actionMarker()     {}
func (SetConnMark) actionMarker() {}
func (SetDSCP) actionMarker()     {}
func (Counter) actionMarker()     {}
func (Log) actionMarker()         {}
func (Limit) actionMarker()       {}
func (Quota) actionMarker()       {}
func (SecMark) actionMarker()     {}

// --- Set types ---

// SetType identifies the data type of set elements.
type SetType uint8

const (
	SetTypeIPv4        SetType = iota + 1 // ipv4_addr
	SetTypeIPv6                           // ipv6_addr
	SetTypeEther                          // ether_addr
	SetTypeInetService                    // inet_service (port)
	SetTypeMark                           // mark
	SetTypeIfname                         // ifname
)

// SetFlags are bitmask flags for set behavior.
type SetFlags uint8

const (
	SetFlagInterval SetFlags = 1 << iota // interval ranges
	SetFlagTimeout                       // per-element timeout
	SetFlagConstant                      // immutable after creation
	SetFlagDynamic                       // dynamically populated
)

// SetElement is a single element in a named set.
type SetElement struct {
	Value   string // string representation (IP, port, etc.)
	Timeout uint32 // per-element timeout in seconds (0 = no timeout)
}

// --- Composite types ---

// Table represents an nftables table owned by ze (kernel name: ze_<Name>).
type Table struct {
	Name       string
	Family     TableFamily
	Chains     []Chain
	Sets       []Set
	Flowtables []Flowtable
}

// Validate checks that the table has a valid name and family.
func (t Table) Validate() error {
	if err := ValidateName(t.Name); err != nil {
		return fmt.Errorf("firewall: table: %w", err)
	}
	if !t.Family.Valid() {
		return fmt.Errorf("firewall: table %q: invalid family %d", t.Name, t.Family)
	}
	return nil
}

// Chain represents an nftables chain within a table.
type Chain struct {
	Name     string
	IsBase   bool      // true = base chain (has hook), false = regular chain
	Type     ChainType // base chain only
	Hook     ChainHook // base chain only
	Priority int32     // base chain only
	Policy   Policy    // base chain only
	Terms    []Term
}

// Validate checks chain consistency. Base chains require type, hook, and policy.
func (c Chain) Validate() error {
	if err := ValidateName(c.Name); err != nil {
		return fmt.Errorf("firewall: chain: %w", err)
	}
	if c.IsBase {
		if !c.Type.Valid() {
			return fmt.Errorf("firewall: base chain %q: type required", c.Name)
		}
		if !c.Hook.Valid() {
			return fmt.Errorf("firewall: base chain %q: hook required", c.Name)
		}
		if !c.Policy.Valid() {
			return fmt.Errorf("firewall: base chain %q: policy required", c.Name)
		}
	}
	return nil
}

// Term is a named rule within a chain (Junos-style from/then split).
type Term struct {
	Name    string
	Matches []Match
	Actions []Action
}

// Set represents a named nftables set within a table.
type Set struct {
	Name     string
	Type     SetType
	Flags    SetFlags
	Elements []SetElement
}

// Flowtable represents an nftables flowtable for hardware offload.
type Flowtable struct {
	Name     string
	Hook     ChainHook
	Priority int32
	Devices  []string
}

// ChainCounters holds per-term counter values for a chain.
type ChainCounters struct {
	Chain string
	Terms []TermCounter
}

// TermCounter holds packet and byte counts for a single term.
type TermCounter struct {
	Name    string
	Packets uint64
	Bytes   uint64
}
