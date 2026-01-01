package api

import (
	"fmt"
	"net/netip"
	"strings"
)

// Selector represents a peer selection pattern.
// Supports: specific IP, all (*), or exclude (!IP).
type Selector struct {
	All     bool       // Match all peers
	IP      netip.Addr // Specific peer IP (if All=false and Exclude is invalid)
	Exclude netip.Addr // Exclude this peer (if All=false and IP is invalid)
}

// ParseSelector parses a peer selector string.
//
// Syntax:
//   - "*" - all peers
//   - "<ip>" - specific peer
//   - "!<ip>" - all peers except this IP
//
// Invalid:
//   - "!*" - cannot exclude all
//   - "!" - empty exclude
//   - "" - empty selector
func ParseSelector(s string) (*Selector, error) {
	s = strings.TrimSpace(s)

	if s == "" {
		return nil, fmt.Errorf("empty selector")
	}

	if s == "*" {
		return &Selector{All: true}, nil
	}

	if s == "!*" {
		return nil, fmt.Errorf("invalid selector: cannot exclude all peers")
	}

	if strings.HasPrefix(s, "!") {
		rest := strings.TrimSpace(s[1:])
		if rest == "" {
			return nil, fmt.Errorf("invalid selector: empty exclude")
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

// Matches returns true if the selector matches the given peer address.
func (sel *Selector) Matches(peer netip.Addr) bool {
	if sel.All {
		return true
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
	if sel.IP.IsValid() {
		return sel.IP.String()
	}
	if sel.Exclude.IsValid() {
		return "!" + sel.Exclude.String()
	}
	return "<invalid>"
}
