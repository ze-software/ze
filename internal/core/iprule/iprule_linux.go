// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management
// Related: iprule.go -- the rule types and table ranges

package iprule

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// newRule MUST start from netlink.NewRule(), never a bare &netlink.Rule{}.
// Several fields use -1 as "unset" and NewRule seeds them (rule.go:50-60:
// SuppressIfgroup, SuppressPrefixlen, Priority, Goto, Flow). The encoder emits
// an attribute for each one that is >= 0 (rule_linux.go:116, :132, :137, :149),
// so a zero-valued literal puts FRA_FLOW, FRA_GOTO and both suppress attributes
// on the wire. FRA_GOTO on a rule whose action is not FR_ACT_GOTO is rejected,
// and the kernel returns EINVAL -- surfacing as
// "ip rule add (mark 0x50000 table 100): invalid argument", which took the whole
// policy-routes plugin down at startup and timed out test/policy 2-5.
//
// The Table bound is not redundant with config validation: netlink.Rule.Table
// is a Go int, and a value that does not survive that conversion is dropped by
// the encoder rather than rejected, so this refuses to build a rule that would
// silently select the wrong table.
//
// tableInt owns that conversion because whether it can truncate at all depends
// on the target's int width. It is defined per architecture (table_linux_amd64.go,
// table_linux_arm64.go, table_linux_generic.go) so the shipped 64-bit targets keep
// the full kernel-legal table range and only a build that could truncate pays a
// bound.
func newRule(priority int, table uint32, family int) (*netlink.Rule, error) {
	tableID, err := tableInt(table)
	if err != nil {
		return nil, err
	}
	rule := netlink.NewRule()
	rule.Priority = priority
	rule.Table = tableID
	rule.Family = family
	return rule, nil
}

// Rule encodes the fwmark rule. A fwmark rule is IPv4: the policy routing plugin
// marks IPv4 traffic only.
func (m Mark) Rule() (*netlink.Rule, error) {
	rule, err := newRule(m.Priority, m.Table, unix.AF_INET)
	if err != nil {
		return nil, err
	}
	mask := m.Mask
	rule.Mark = m.Mark
	rule.Mask = &mask
	return rule, nil
}

// Rule encodes the source-prefix rule in the prefix's family. An invalid prefix and a
// 4-in-6 prefix are refused: the first names no source, and the second would match
// in neither family the way its writer meant.
func (s Source) Rule() (*netlink.Rule, error) {
	if !s.From.IsValid() {
		return nil, fmt.Errorf("ip rule from an invalid prefix (table %d)", s.Table)
	}
	addr := s.From.Addr()
	if addr.Is4In6() {
		return nil, fmt.Errorf("ip rule from %s: a 4-in-6 prefix names neither family; write it as IPv4", s.From)
	}
	family := unix.AF_INET6
	if addr.Is4() {
		family = unix.AF_INET
	}
	rule, err := newRule(s.Priority, s.Table, family)
	if err != nil {
		return nil, err
	}
	from := s.From.Masked()
	rule.Src = prefixIPNet(from)
	return rule, nil
}

// prefixIPNet converts a masked prefix to the net.IPNet the netlink encoder takes,
// in the prefix's own family width.
func prefixIPNet(p netip.Prefix) *net.IPNet {
	bits := p.Addr().BitLen()
	return &net.IPNet{IP: p.Addr().AsSlice(), Mask: net.CIDRMask(p.Bits(), bits)}
}

// Add installs the source rule. A rule the kernel already holds answers nil: the
// state Add asked for is in place, as after a restart that left Ze's rule behind.
func (s Source) Add() error {
	rule, err := s.Rule()
	if err != nil {
		return err
	}
	if err := netlink.RuleAdd(rule); err != nil && !errors.Is(err, unix.EEXIST) {
		return fmt.Errorf("ip rule add from %s lookup %d: %w", s.From, s.Table, err)
	}
	return nil
}

// Delete removes the source rule. A rule the kernel does not hold answers nil: the
// state Delete asked for is in place.
func (s Source) Delete() error {
	rule, err := s.Rule()
	if err != nil {
		return err
	}
	if err := netlink.RuleDel(rule); err != nil && !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("ip rule del from %s lookup %d: %w", s.From, s.Table, err)
	}
	return nil
}

// Replace installs the route, replacing one with the same table and destination.
func (r Route) Replace() error {
	route, err := r.netlinkRoute()
	if err != nil {
		return err
	}
	if err := netlink.RouteReplace(route); err != nil {
		return fmt.Errorf("ip route replace %s table %d: %w", r.Dst, r.Table, err)
	}
	return nil
}

// Delete removes the route. One the kernel does not hold answers nil.
func (r Route) Delete() error {
	route, err := r.netlinkRoute()
	if err != nil {
		return err
	}
	if err := netlink.RouteDel(route); err != nil && !errors.Is(err, unix.ESRCH) && !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("ip route del %s table %d: %w", r.Dst, r.Table, err)
	}
	return nil
}

// netlinkRoute encodes the route. The table goes through the same bounded conversion
// a rule's does (tableInt), and a destination that is not a valid prefix is refused.
func (r Route) netlinkRoute() (*netlink.Route, error) {
	if !r.Dst.IsValid() {
		return nil, fmt.Errorf("route in table %d has no valid destination", r.Table)
	}
	tableID, err := tableInt(r.Table)
	if err != nil {
		return nil, err
	}
	family := netlink.FAMILY_V6
	if r.Dst.Addr().Is4() {
		family = netlink.FAMILY_V4
	}
	route := &netlink.Route{
		Dst:      prefixIPNet(r.Dst.Masked()),
		Table:    tableID,
		Family:   family,
		Protocol: netlink.RouteProtocol(r.Protocol),
	}
	if r.Unreachable {
		route.Type = unix.RTN_UNREACHABLE
		return route, nil
	}
	if r.LinkIndex <= 0 {
		return nil, fmt.Errorf("route %s in table %d names no device", r.Dst, r.Table)
	}
	route.LinkIndex = r.LinkIndex
	route.Scope = netlink.SCOPE_LINK
	return route, nil
}
