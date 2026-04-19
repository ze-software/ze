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

// PortRange is a single port (Lo==Hi) or contiguous range (Lo<Hi).
type PortRange struct {
	Lo uint16
	Hi uint16
}

// MatchSourcePort matches packets by source port. Ranges holds one or more
// single-port or range entries; len>1 lowers to an nftables anonymous set.
type MatchSourcePort struct {
	Ranges []PortRange
}

// MatchDestinationPort matches packets by destination port. Ranges holds one
// or more single-port or range entries; len>1 lowers to an nftables anonymous
// set so `destination port 22,80,443` and `destination port 5060-5061,16384-32767`
// both express the operator's intent exactly.
type MatchDestinationPort struct {
	Ranges []PortRange
}

// MatchProtocol matches packets by L4 protocol name.
type MatchProtocol struct{ Protocol string }

// MatchInputInterface matches packets by input interface name.
// A trailing `*` in the config (e.g. `l2tp*`) sets Wildcard=true and
// strips the `*` from Name, producing a prefix match against the first
// len(Name) bytes of the kernel's 16-byte IFNAMSIZ-padded name rather
// than the full exact compare.
type MatchInputInterface struct {
	Name     string
	Wildcard bool
}

// MatchOutputInterface matches packets by output interface name.
// See MatchInputInterface for Wildcard semantics.
type MatchOutputInterface struct {
	Name     string
	Wildcard bool
}

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

// MatchICMPType matches packets by ICMPv4 type byte. Values follow
// IANA assignments (echo-request=8, echo-reply=0, etc.).
type MatchICMPType struct{ Type uint8 }

// MatchICMPv6Type matches packets by ICMPv6 type byte (echo-request=128,
// neighbor-solicit=135, etc.).
type MatchICMPv6Type struct{ Type uint8 }

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

// Interface compliance: every Match type implements Match. Keep this list
// exhaustive with the struct definitions above -- a markerless type compiles
// but silently fails the `m.(Match)` assertion at runtime.
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
func (MatchInSet) matchMarker()              {}
func (MatchICMPType) matchMarker()           {}
func (MatchICMPv6Type) matchMarker()         {}

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

// SNAT applies source NAT. A zero AddressEnd means single-address NAT;
// a non-zero AddressEnd programs a source-address range via the NAT
// expression's RegAddrMax register.
type SNAT struct {
	Address    netip.Addr
	AddressEnd netip.Addr
	Port       uint16
	PortEnd    uint16
	Flags      uint32
}

// DNAT applies destination NAT. AddressEnd works the same as SNAT.AddressEnd.
type DNAT struct {
	Address    netip.Addr
	AddressEnd netip.Addr
	Port       uint16
	PortEnd    uint16
	Flags      uint32
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

// Notrack disables connection tracking for the packet.
type Notrack struct{}

// FlowOffload offloads the connection to a flowtable for hardware acceleration.
type FlowOffload struct{ FlowtableName string }

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
//
// Level, Group, and Snaplen are pointers so the operator can distinguish
// "not set" (kernel default applies) from "explicitly zero" (emerg level
// for Level, group 0 for Group, no truncation for Snaplen). A bare
// `uint32` would collapse both cases to zero and silently remap
// `level 0` (emerg) to the kernel default (warning).
type Log struct {
	Prefix  string
	Level   *uint32
	Group   *uint16
	Snaplen *uint32
}

// RateDimension distinguishes packet-per-unit from byte-per-unit rate
// limits. Zero (unspecified) is rejected at lowering so a Limit built
// outside the parser surfaces the missing dimension instead of silently
// programming a packet rate.
type RateDimension uint8

const (
	rateDimensionUnspecified RateDimension = iota
	RateDimensionPackets
	RateDimensionBytes
)

// Limit applies a rate limit. Rate is expressed in the unit pair
// (Dimension, Unit): packets/second, bytes/second, mbytes/minute, etc.
// For byte rates the caller has already scaled the numeric prefix
// (kbytes=1024, mbytes=1024^2, gbytes=1024^3) so Rate is a plain
// bytes-per-unit value when Dimension==RateDimensionBytes.
//
// Callers MUST set Dimension to either RateDimensionPackets or
// RateDimensionBytes. parseRateSpec does this for all
// operator-originated Limits; programmatic callers (tests, direct
// constructors) must set it explicitly. The zero value
// (rateDimensionUnspecified) is rejected at lowering so a silent
// "default to packets" cannot hide a missing assignment.
type Limit struct {
	Rate      uint64
	Unit      string // "second", "minute", "hour", "day"
	Dimension RateDimension
	Over      bool
	Burst     uint32
}

// Interface compliance: every Action type implements Action. Keep this list
// exhaustive with the struct definitions above -- a markerless type compiles
// but silently fails the `a.(Action)` assertion at runtime.
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
func (Notrack) actionMarker()     {}
func (FlowOffload) actionMarker() {}
func (SetMark) actionMarker()     {}
func (SetConnMark) actionMarker() {}
func (SetDSCP) actionMarker()     {}
func (Counter) actionMarker()     {}
func (Log) actionMarker()         {}
func (Limit) actionMarker()       {}

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

// String returns the nft-native name for the set type (ipv4_addr,
// inet_service, etc.) so verify-time error messages render the same
// token operators wrote in the config rather than a bare integer.
func (s SetType) String() string {
	switch s {
	case SetTypeIPv4:
		return "ipv4_addr"
	case SetTypeIPv6:
		return "ipv6_addr"
	case SetTypeEther:
		return "ether_addr"
	case SetTypeInetService:
		return "inet_service"
	case SetTypeMark:
		return "mark"
	case SetTypeIfname:
		return "ifname"
	}
	return fmt.Sprintf("unknown(%d)", uint8(s))
}

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

// ChainPriorityMin and ChainPriorityMax bound the base-chain priority
// leaf. The kernel accepts any int32 and silently clamps outside its
// internal reserved ranges; surfacing [-400, 400] at verify gives
// operators a clear diagnostic for a common typo (e.g. 500 vs 50).
// Well-known nftables priorities (raw -300, mangle -150, filter 0,
// security 50, nat-srcpost 100, nat-dstpre -100) all fit comfortably.
const (
	ChainPriorityMin int32 = -400
	ChainPriorityMax int32 = 400
)

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
		if c.Priority < ChainPriorityMin || c.Priority > ChainPriorityMax {
			return fmt.Errorf("firewall: base chain %q: priority %d out of range %d..%d",
				c.Name, c.Priority, ChainPriorityMin, ChainPriorityMax)
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

// Validate checks that the set has a valid name. Type 0 is rejected so
// an uninitialised Set cannot slip through OnConfigVerify.
func (s Set) Validate() error {
	if err := ValidateName(s.Name); err != nil {
		return fmt.Errorf("firewall: set: %w", err)
	}
	if s.Type == 0 {
		return fmt.Errorf("firewall: set %q: type required", s.Name)
	}
	return nil
}

// Flowtable represents an nftables flowtable for hardware offload.
type Flowtable struct {
	Name     string
	Hook     ChainHook
	Priority int32
	Devices  []string
}

// Validate checks that the flowtable has a valid name and hook.
func (f Flowtable) Validate() error {
	if err := ValidateName(f.Name); err != nil {
		return fmt.Errorf("firewall: flowtable: %w", err)
	}
	if !f.Hook.Valid() {
		return fmt.Errorf("firewall: flowtable %q: hook required", f.Name)
	}
	return nil
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
