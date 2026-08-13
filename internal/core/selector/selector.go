// Design: docs/architecture/core-design.md — peer selector
//
// Package selector provides typed peer selection patterns for ze.
// Parse at CLI/RPC boundaries, pass the typed Selector internally,
// convert back to string only when crossing to external plugins.
//
// Syntax:
//   - "*" - all peers
//   - "<ip>" - specific peer
//   - "!<selector>" - all peers except those matching <selector>
//   - "<ip>,<ip>,..." - multiple specific peers
//   - "as<N>" - all peers with ASN N
//   - "<glob>" - glob pattern (e.g. "192.168.*.*")
//   - "<name>" - peer name
package selector

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/stringsx"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Kind identifies the selector variant. Zero is invalid.
type Kind uint8

const (
	KindAll   Kind = iota + 1 // match all peers
	KindAddr                  // single IP
	KindAddrs                 // comma-separated IP list
	KindName                  // peer name
	KindASN                   // as<N>
	KindGlob                  // IP glob pattern (e.g. 192.168.*.*)
)

var kindNames = [...]string{
	KindAll:   "all",
	KindAddr:  "addr",
	KindAddrs: "addrs",
	KindName:  "name",
	KindASN:   "asn",
	KindGlob:  "glob",
}

func (k Kind) String() string {
	if int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

var (
	errEmptySelector                        = errors.New("empty selector")
	errInvalidSelectorCannotExcludeAllPeers = errors.New("invalid selector: cannot exclude all peers")
	errInvalidSelectorNegationWithMultiIp   = errors.New("invalid selector: negation with multi-IP not supported")
	errInvalidSelectorEmptyExclude          = errors.New("invalid selector: empty exclude")
	errInvalidSelectorEmptyIpInList         = errors.New("invalid selector: empty IP in list")
)

// Selector represents a typed peer selection pattern.
// Exclusion is a flag that can negate any kind.
type Selector struct {
	kind    Kind
	exclude bool                    // negate the match ("!" prefix)
	ip      netip.Addr              // KindAddr (both include and exclude)
	ips     []netip.Addr            // KindAddrs
	ipSet   map[netip.Addr]struct{} // KindAddrs O(1) lookup when len(ips) > 16
	name    string                  // KindName, KindGlob
	asn     uint32                  // KindASN
}

// Constructors — prefer these over Parse for in-process code.

func All() *Selector                      { return &Selector{kind: KindAll} }
func Addr(ip netip.Addr) *Selector        { return &Selector{kind: KindAddr, ip: ip} }
func ExcludeAddr(ip netip.Addr) *Selector { return &Selector{kind: KindAddr, ip: ip, exclude: true} }

// Addrs selects exactly the listed peers. It is the constructor a caller reaches
// for after narrowing a wider selector to a subset it resolved itself, which
// Parse can only express by re-joining the addresses into a string and reading
// them back.
func Addrs(ips []netip.Addr) *Selector { return multiAddr(ips) }

func multiAddr(ips []netip.Addr) *Selector {
	sel := &Selector{kind: KindAddrs, ips: ips}
	if len(ips) > 16 {
		sel.ipSet = make(map[netip.Addr]struct{}, len(ips))
		for _, ip := range ips {
			sel.ipSet[ip] = struct{}{}
		}
	}
	return sel
}

func PeerName(name string) *Selector    { return &Selector{kind: KindName, name: name} }
func excludeName(name string) *Selector { return &Selector{kind: KindName, name: name, exclude: true} }
func ASN(asn uint32) *Selector          { return &Selector{kind: KindASN, asn: asn} }
func Glob(pattern string) *Selector     { return &Selector{kind: KindGlob, name: pattern} }

// Accessors

func (s *Selector) SelectorKind() Kind  { return s.kind }
func (s *Selector) IsExclude() bool     { return s.exclude }
func (s *Selector) IP() netip.Addr      { return s.ip }
func (s *Selector) IPs() []netip.Addr   { return s.ips }
func (s *Selector) NameValue() string   { return s.name }
func (s *Selector) ASNValue() uint32    { return s.asn }
func (s *Selector) globPattern() string { return s.name }

// ParseDefault parses a selector string, treating empty/"*" as All.
// On parse error, falls back to PeerName (fail-closed: no accidental match-all).
func ParseDefault(s string) *Selector {
	if s == "" || s == "*" {
		return All()
	}
	sel, err := Parse(s)
	if err != nil {
		return PeerName(s)
	}
	return sel
}

// MatchesPeerKey returns true if the selector matches a peer identified by key.
// The key may be an IP address string or a non-IP name (e.g. BMP "router1:peer1").
// For KindASN, returns false (reactor-level resolution needed).
func (sel *Selector) MatchesPeerKey(peerKey string) bool {
	switch sel.kind {
	case KindAll:
		return true
	case KindName:
		match := peerKey == sel.name
		if sel.exclude {
			return !match
		}
		return match
	case KindAddr:
		addr, err := netip.ParseAddr(peerKey)
		if err != nil {
			match := peerKey == sel.ip.String()
			if sel.exclude {
				return !match
			}
			return match
		}
		return sel.Matches(addr)
	case KindAddrs:
		addr, err := netip.ParseAddr(peerKey)
		if err != nil {
			return false
		}
		return sel.Matches(addr)
	case KindGlob:
		match := matchIPGlob(sel.name, peerKey)
		if sel.exclude {
			return !match
		}
		return match
	default:
		return false
	}
}

// Parse parses a peer selector string into a typed Selector.
// This is the boundary parser — call it at CLI/RPC entry points.
//
// Detection order:
//  1. "*" → KindAll
//  2. "!*" → error
//  3. "!" prefix → parse remainder, set exclude flag
//  4. comma-separated → KindAddrs (negation rejected)
//  5. valid IP → KindAddr
//  6. "as<N>" (case-insensitive) → KindASN
//  7. contains "*" → KindGlob
//  8. non-empty → KindName
func Parse(s string) (*Selector, error) {
	s = strings.TrimSpace(s)

	if s == "" {
		return nil, errEmptySelector
	}

	if s == "*" {
		return All(), nil
	}

	if s == "!*" {
		return nil, errInvalidSelectorCannotExcludeAllPeers
	}

	if strings.HasPrefix(s, "!") {
		rest := strings.TrimSpace(s[1:])
		if rest == "" {
			return nil, errInvalidSelectorEmptyExclude
		}
		if strings.Contains(rest, ",") {
			return nil, errInvalidSelectorNegationWithMultiIp
		}
		inner, err := parsePositive(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude selector %q: %w", rest, err)
		}
		inner.exclude = true
		return inner, nil
	}

	if strings.Contains(s, ",") {
		return parseMultiIP(s)
	}

	return parsePositive(s)
}

// parsePositive parses a non-negated, non-comma selector.
func parsePositive(s string) (*Selector, error) {
	if ip, err := netip.ParseAddr(s); err == nil {
		return Addr(ip), nil
	}

	if ap, err := netip.ParseAddrPort(s); err == nil {
		return Addr(ap.Addr()), nil
	}

	if asn, ok := parseASNSelector(s); ok {
		return ASN(asn), nil
	}

	if strings.ContainsRune(s, '*') {
		return Glob(s), nil
	}

	return PeerName(s), nil
}

func parseASNSelector(s string) (uint32, bool) {
	if len(s) <= 2 {
		return 0, false
	}
	if s[0] != 'a' && s[0] != 'A' {
		return 0, false
	}
	if s[1] != 's' && s[1] != 'S' {
		return 0, false
	}
	n, err := strconv.ParseUint(s[2:], 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(n), true
}

func parseMultiIP(s string) (*Selector, error) {
	parts, count := stringsx.SplitCount(s, ",")
	ips := make([]netip.Addr, 0, count)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, errInvalidSelectorEmptyIpInList
		}
		ip, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("invalid IP %q in list: %w", part, err)
		}
		ips = append(ips, ip)
	}

	return multiAddr(ips), nil
}

// Matches returns true if the selector matches the given peer address.
// For KindName and KindASN, this method returns false because
// IP-only matching is insufficient -- use SelectorKind() to dispatch at the reactor level.
// Exclusion is handled: KindAddr with exclude returns true for non-matching addresses.
func (sel *Selector) Matches(peer netip.Addr) bool {
	var match bool
	switch sel.kind {
	case KindAll:
		match = true
	case KindAddr:
		match = sel.ip == peer
	case KindAddrs:
		if sel.ipSet != nil {
			_, match = sel.ipSet[peer]
		} else {
			match = slices.Contains(sel.ips, peer)
		}
	case KindGlob:
		match = matchIPGlob(sel.name, peer.String())
	case KindName, KindASN:
		match = false
	default:
		match = false
	}
	if sel.exclude {
		return !match
	}
	return match
}

// matchIPGlob checks if an IP address string matches a glob pattern.
// For IPv4, each octet can be "*" to match any value 0-255.
// Examples: "192.168.*.*", "10.*.0.1".
func matchIPGlob(pattern, ip string) bool {
	if pattern == "*" || pattern == "" {
		return true
	}
	if strings.Contains(pattern, ".") && strings.Contains(ip, ".") {
		patParts := strings.Split(pattern, ".")
		ipParts := strings.Split(ip, ".")
		if len(patParts) != 4 || len(ipParts) != 4 {
			return false
		}
		for i := range 4 {
			if patParts[i] != "*" && patParts[i] != ipParts[i] {
				return false
			}
		}
		return true
	}
	return pattern == ip
}

// String returns the canonical string representation.
func (sel *Selector) String() string {
	var base string
	switch sel.kind {
	case KindAll:
		base = "*"
	case KindAddr:
		base = sel.ip.String()
	case KindAddrs:
		strs := make([]string, len(sel.ips))
		for i, ip := range sel.ips {
			strs[i] = ip.String()
		}
		base = textbuf.Join(strs, ",")
	case KindName:
		base = sel.name
	case KindASN:
		var tb textbuf.Buffer
		base = tb.Str("as").Uint32(sel.asn).String()
	case KindGlob:
		base = sel.name
	default:
		return "<invalid>"
	}
	if sel.exclude {
		var tb textbuf.Buffer
		return tb.Byte('!').Str(base).String()
	}
	return base
}
