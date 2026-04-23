package policyroute

import (
	"fmt"

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

func newIPRule(r ipRuleSpec) *netlink.Rule {
	mask := r.Mask
	return &netlink.Rule{
		Priority: r.Priority,
		Table:    int(r.Table),
		Mark:     r.Mark,
		Mask:     &mask,
		Family:   unix.AF_INET,
	}
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
			Protocol: 250, // rtprotZE
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
			Protocol: 250,
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
