// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management

package policyroute

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/iprule"
	"github.com/ze-software/ze/internal/core/rtproto"

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

// applyIPRules installs the fwmark rules. The encoding, and why it starts from
// netlink.NewRule(), lives in internal/core/iprule (Mark.Rule).
func (rm *linuxRuleManager) applyIPRules(rules []iprule.Mark) error {
	for _, r := range rules {
		rule, err := r.Rule()
		if err != nil {
			return fmt.Errorf("ip rule add (mark 0x%x table %d): %w", r.Mark, r.Table, err)
		}
		if err := rm.handle.RuleAdd(rule); err != nil {
			return fmt.Errorf("ip rule add (mark 0x%x table %d): %w", r.Mark, r.Table, err)
		}
	}
	return nil
}

func (rm *linuxRuleManager) removeIPRules(rules []iprule.Mark) {
	for _, r := range rules {
		rule, err := r.Rule()
		if err != nil {
			// Cleanup is best-effort, but say so: a rule we could not encode
			// was never installed, so there is nothing to delete.
			logger().Warn("policyroute: skipping ip rule delete", "mark", r.Mark, "table", r.Table, "error", err)
			continue
		}
		_ = rm.handle.RuleDel(rule)
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
