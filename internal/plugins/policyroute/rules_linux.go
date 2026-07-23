// Design: plan/learned/684-policy-routing.md -- netlink ip rule and route management

package policyroute

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/core/rtproto"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type linuxRuleManager struct {
	handle *netlink.Handle
}

func newRuleManager() (*linuxRuleManager, error) {
	h, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		return nil, fmt.Errorf("policyroute: netlink handle: %w", err)
	}
	return &linuxRuleManager{handle: h}, nil
}

func (rm *linuxRuleManager) close() {
	if rm.handle != nil {
		rm.handle.Close()
	}
}

// newIPRule MUST start from netlink.NewRule(), never a bare &netlink.Rule{}.
// Several fields use -1 as "unset" and NewRule seeds them (rule.go:50-60:
// SuppressIfgroup, SuppressPrefixlen, Priority, Goto, Flow). The encoder emits
// an attribute for each one that is >= 0 (rule_linux.go:116, :132, :137, :149),
// so a zero-valued literal puts FRA_FLOW, FRA_GOTO and both suppress attributes
// on the wire. FRA_GOTO on a rule whose action is not FR_ACT_GOTO is rejected,
// and the kernel returns EINVAL -- surfacing as
// "ip rule add (mark 0x50000 table 100): invalid argument", which took the whole
// policy-routes plugin down at startup and timed out test/policy 2-5.
func newIPRule(r ipRuleSpec) *netlink.Rule {
	mask := r.Mask
	rule := netlink.NewRule()
	rule.Priority = r.Priority
	rule.Table = int(r.Table)
	rule.Mark = r.Mark
	rule.Mask = &mask
	rule.Family = unix.AF_INET
	return rule
}

func (rm *linuxRuleManager) applyIPRules(rules []ipRuleSpec) error {
	for _, r := range rules {
		if err := rm.handle.RuleAdd(newIPRule(r)); err != nil {
			return fmt.Errorf("ip rule add (mark 0x%x table %d): %w", r.Mark, r.Table, err)
		}
	}
	return nil
}

func (rm *linuxRuleManager) removeIPRules(rules []ipRuleSpec) {
	for _, r := range rules {
		_ = rm.handle.RuleDel(newIPRule(r))
	}
}

func (rm *linuxRuleManager) applyAutoRoutes(routes []autoRouteSpec) error {
	for _, r := range routes {
		gw := r.NextHop.As4()
		route := &netlink.Route{
			Gw:       gw[:],
			Table:    int(r.Table),
			Protocol: rtproto.PolicyRoute,
		}
		if err := rm.handle.RouteAdd(route); err != nil {
			return fmt.Errorf("route add (table %d via %s): %w", r.Table, r.NextHop, err)
		}
	}
	return nil
}

func (rm *linuxRuleManager) removeAutoRoutes(routes []autoRouteSpec) {
	for _, r := range routes {
		gw := r.NextHop.As4()
		route := &netlink.Route{
			Gw:       gw[:],
			Table:    int(r.Table),
			Protocol: rtproto.PolicyRoute,
		}
		_ = rm.handle.RouteDel(route)
	}
}

func (rm *linuxRuleManager) applyAll(result *translationResult) error {
	if err := rm.applyAutoRoutes(result.AutoRoutes); err != nil {
		return err
	}
	return rm.applyIPRules(result.IPRules)
}

func (rm *linuxRuleManager) removeAll(result *translationResult) {
	rm.removeIPRules(result.IPRules)
	rm.removeAutoRoutes(result.AutoRoutes)
}
