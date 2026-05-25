// Design: docs/architecture/core-design.md — peer selector
//
// Package selector provides peer selection patterns for ze.
// The selector syntax is used throughout ze for targeting peers.
//
// Syntax:
//   - "*" - all peers
//   - "<ip>" - specific peer
//   - "!<ip>" - all peers except this IP
//   - "<ip>,<ip>,..." - multiple specific peers
package selector

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/core/stringsx"
)

var (
	errEmptySelector                        = errors.New("empty selector")
	errInvalidSelectorCannotExcludeAllPeers = errors.New("invalid selector: cannot exclude all peers")
	errInvalidSelectorNegationWithMultiIp   = errors.New("invalid selector: negation with multi-IP not supported")
	errInvalidSelectorEmptyExclude          = errors.New("invalid selector: empty exclude")
	errInvalidSelectorEmptyIpInList         = errors.New("invalid selector: empty IP in list")
)

// Selector represents a peer selection pattern.
// Supports: specific IP, all (*), exclude (!IP), or multiple IPs (ip,ip,ip).
type Selector struct {
	All     bool                    // Match all peers
	IP      netip.Addr              // Specific peer IP (if All=false and Exclude/IPs are empty)
	Exclude netip.Addr              // Exclude this peer (if All=false and IP/IPs are empty)
	IPs     []netip.Addr            // Multiple specific peers (if All=false and IP/Exclude are empty)
	ipSet   map[netip.Addr]struct{} // O(1) lookup for multi-IP; built at parse time when len(IPs) > 16
}

// Parse parses a peer selector string.
//
// Syntax:
//   - "*" - all peers
//   - "<ip>" - specific peer
//   - "!<ip>" - all peers except this IP
//   - "<ip>,<ip>,..." - multiple specific peers
//
// Invalid:
//   - "!*" - cannot exclude all
//   - "!" - empty exclude
//   - "" - empty selector
//   - "!<ip>,<ip>" - negation with multi-IP not supported
func Parse(s string) (*Selector, error) {
	s = strings.TrimSpace(s)

	if s == "" {
		return nil, errEmptySelector
	}

	if s == "*" {
		return &Selector{All: true}, nil
	}

	if s == "!*" {
		return nil, errInvalidSelectorCannotExcludeAllPeers
	}

	// Check for multi-IP (comma-separated) before checking negation
	if strings.Contains(s, ",") {
		// Negation with multi-IP not supported
		if strings.HasPrefix(s, "!") {
			return nil, errInvalidSelectorNegationWithMultiIp
		}
		return parseMultiIP(s)
	}

	if strings.HasPrefix(s, "!") {
		rest := strings.TrimSpace(s[1:])
		if rest == "" {
			return nil, errInvalidSelectorEmptyExclude
		}
		ip, err := netip.ParseAddr(rest)
		if err != nil {
			return nil, fmt.Errorf("invalid exclude IP %q: %w", rest, err)
		}
		return &Selector{Exclude: ip}, nil
	}

	ip, err := netip.ParseAddr(s)
	if err != nil {
		return nil, fmt.Errorf("invalid peer IP %q: %w", s, err)
	}
	return &Selector{IP: ip}, nil
}

// parseMultiIP parses comma-separated IPs.
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

	sel := &Selector{IPs: ips}
	if len(ips) > 16 {
		sel.ipSet = make(map[netip.Addr]struct{}, len(ips))
		for _, ip := range ips {
			sel.ipSet[ip] = struct{}{}
		}
	}
	return sel, nil
}

// Matches returns true if the selector matches the given peer address.
func (sel *Selector) Matches(peer netip.Addr) bool {
	if sel.All {
		return true
	}

	if len(sel.IPs) > 0 {
		if sel.ipSet != nil {
			_, ok := sel.ipSet[peer]
			return ok
		}
		return slices.Contains(sel.IPs, peer)
	}

	if sel.IP.IsValid() {
		return sel.IP == peer
	}

	if sel.Exclude.IsValid() {
		return sel.Exclude != peer
	}

	return false
}

// String returns a string representation of the selector.
func (sel *Selector) String() string {
	if sel.All {
		return "*"
	}
	if len(sel.IPs) > 0 {
		strs := make([]string, len(sel.IPs))
		for i, ip := range sel.IPs {
			strs[i] = ip.String()
		}
		return strings.Join(strs, ",")
	}
	if sel.IP.IsValid() {
		return sel.IP.String()
	}
	if sel.Exclude.IsValid() {
		return "!" + sel.Exclude.String()
	}
	return "<invalid>"
}
