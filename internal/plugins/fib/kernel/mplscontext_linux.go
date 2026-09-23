//go:build linux

// Design: docs/architecture/mpls/mpls-kernel.md -- mark-selected MPLS push contexts.
// Related: mplscontext.go -- forwarding-owner contract and reserved mark namespace.
package fibkernel

import (
	"errors"
	"fmt"
	"net/netip"
	"sync"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

const mplsContextPriority = 1
const mplsGuardPriority = 2

type mplsRuleKey struct {
	tableID uint32
	family  int
}

// mplsContextState owns only objects this backend successfully created. A failed
// delete keeps its object here so a later withdrawal or close can retry it.
// mu serializes context operations with close, including a late bus delivery.
// Namespace guards survive owner close because producers can stop concurrently.
type mplsContextState struct {
	mu        sync.Mutex
	closed    bool
	routes    map[mplsPushKey]*netlink.Route
	selectors map[mplsRuleKey]*netlink.Rule
	guards    map[int]*netlink.Rule
}

func (n *netlinkBackend) mplsContextCount() int {
	n.contexts.mu.Lock()
	defer n.contexts.mu.Unlock()
	return len(n.contexts.routes)
}

func (n *netlinkBackend) addMPLSContext(r RichRoute) error {
	n.contexts.mu.Lock()
	defer n.contexts.mu.Unlock()
	if n.contexts.closed {
		return errors.New("mpls context: backend closed")
	}
	if r.TableID&mplsContextMask != mplsContextMark {
		return fmt.Errorf("mpls context: table %#x is outside the bypass namespace", r.TableID)
	}
	if !r.Prefix.IsValid() || !r.NextHop.IsValid() {
		return errors.New("mpls context: valid prefix and next hop required")
	}
	if err := validateMPLSLabels(r.Labels); err != nil {
		return err
	}
	r.Prefix = r.Prefix.Masked()
	route, err := n.buildMPLSContextRoute(r)
	if err != nil {
		return err
	}
	if err := n.ensureMPLSGuard(route.Family); err != nil {
		return err
	}
	key := mplsPushKey{tableID: r.TableID, fec: r.Prefix}
	if err := n.checkMPLSContextTable(key, route.Family); err != nil {
		return err
	}
	previous := n.contexts.routes[key]
	ensureLabelSpace()
	if previous == nil {
		err = n.handle.RouteAdd(route)
	} else {
		err = n.handle.RouteReplace(route)
	}
	if err != nil {
		return fmt.Errorf("mpls context table %d route install: %w", r.TableID, err)
	}
	if n.contexts.routes == nil {
		n.contexts.routes = make(map[mplsPushKey]*netlink.Route)
	}
	n.contexts.routes[key] = route

	// The selector is installed last. A failure here MUST undo this route;
	// Apply cannot acknowledge a context that a marked packet cannot select.
	err = n.ensureMPLSSelector(mplsRuleKey{tableID: r.TableID, family: route.Family})
	if err == nil {
		err = n.checkMPLSContextForwarding(key, route)
	}
	if err == nil {
		return nil
	}
	if previous != nil {
		if rollbackErr := n.handle.RouteReplace(previous); rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("mpls context restore previous route: %w", rollbackErr))
		}
		n.contexts.routes[key] = previous
		return err
	}
	return errors.Join(err, n.removeMPLSContext(key))
}

// buildMPLSContextRoute resolves the immediate neighbor through the ordinary
// FIB, then pins the private route to that device. A private table has no copied
// connected routes; RTNH_F_ONLINK makes its gateway check independent of them.
func (n *netlinkBackend) buildMPLSContextRoute(r RichRoute) (*netlink.Route, error) {
	route, err := buildRichRoute(r)
	if err != nil {
		return nil, err
	}
	index, err := n.resolveMPLSNextHop(r.NextHop)
	if err != nil {
		return nil, err
	}
	route.LinkIndex = index
	route.Flags |= unix.RTNH_F_ONLINK
	// Explicit priority avoids IPv6's kernel-selected default metric.
	route.Priority = 1
	route.Family = netlink.FAMILY_V4
	if r.Prefix.Addr().Is6() {
		route.Family = netlink.FAMILY_V6
	}
	return route, nil
}

// checkMPLSContextTable rejects all foreign routes in the private table, including
// more-specific routes that would take precedence over this FEC. It also checks
// retained ownership before any Replace can overwrite an external replacement.
func (n *netlinkBackend) checkMPLSContextTable(key mplsPushKey, family int) error {
	routes, err := n.handle.RouteListFiltered(family, &netlink.Route{Table: int(key.tableID)}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}
	for i := range routes {
		current := &routes[i]
		if current.Dst == nil {
			return fmt.Errorf("mpls context table %d has a foreign default route: %w", key.tableID, unix.EEXIST)
		}
		prefix, err := netip.ParsePrefix(current.Dst.String())
		if err != nil {
			return err
		}
		owned := n.contexts.routes[mplsPushKey{tableID: key.tableID, fec: prefix.Masked()}]
		if owned == nil || !sameMPLSRoute(current, owned) {
			return fmt.Errorf("mpls context table %d has a foreign route: %w", key.tableID, unix.EEXIST)
		}
	}
	return nil
}

// sameMPLSRoute compares a fresh kernel dump with our installation. Link-state
// flags are transient and MUST NOT prevent withdrawal after an interface fails.
func sameMPLSRoute(current, owned *netlink.Route) bool {
	current.Flags &^= unix.RTNH_F_DEAD | unix.RTNH_F_LINKDOWN
	return current.MTU == owned.MTU && current.MTULock == owned.MTULock && owned.Equal(*current)
}

func mplsMarkRule(family, priority int, mark, mask uint32) *netlink.Rule {
	rule := netlink.NewRule()
	rule.Family = family
	rule.Priority = priority
	rule.Mark = mark
	rule.Mask = &mask
	rule.Protocol = rtprotZE
	return rule
}

// sameMPLSRule compares the selectors and ownership returned by RuleList. The
// netlink binding does not decode the rule action; deletes also supply the action
// originally installed, so an action changed by another writer cannot be deleted.
func sameMPLSRule(current, owned *netlink.Rule) bool {
	if current.Mask == nil || owned.Mask == nil {
		return false
	}
	return current.Family == owned.Family && current.Priority == owned.Priority &&
		current.Table == owned.Table && current.Mark == owned.Mark && *current.Mask == *owned.Mask &&
		current.Protocol == owned.Protocol && current.Src == nil && current.Dst == nil &&
		current.Tos == 0 && current.TunID == 0 && current.Goto == -1 && current.Flow == -1 &&
		current.IifName == "" && current.OifName == "" && current.SuppressIfgroup == -1 &&
		current.SuppressPrefixlen == -1 && !current.Invert && current.Dport == nil &&
		current.Sport == nil && current.IPProto == 0 && current.UIDRange == nil
}

func mplsRuleMatchesMark(rule *netlink.Rule, mark uint32) bool {
	mask := uint32(0)
	if rule.Mask != nil {
		mask = *rule.Mask
	} else if rule.Mark != 0 {
		mask = ^uint32(0)
	}
	matches := mark&mask == rule.Mark&mask
	if rule.Invert {
		return !matches
	}
	return matches
}

func (n *netlinkBackend) ensureMPLSGuard(family int) error {
	guard := mplsMarkRule(family, mplsGuardPriority, mplsContextMark, mplsContextMask)
	guard.Type = unix.FR_ACT_UNREACHABLE
	rules, err := n.handle.RuleList(family)
	if err != nil {
		return err
	}
	for i := range rules {
		rule := &rules[i]
		if sameMPLSRule(rule, guard) {
			continue
		}
		if rule.Priority != mplsGuardPriority {
			continue
		}
		mask := uint32(0)
		if rule.Mask != nil {
			mask = *rule.Mask
		}
		if rule.Invert || (rule.Mark^mplsContextMark)&mask&mplsContextMask == 0 {
			return fmt.Errorf("mpls context guard conflicts with existing rule: %w", unix.EEXIST)
		}
	}
	// RuleList omits the action. An exclusive add either installs the intended
	// unreachable action or confirms that exact rule already exists; matching
	// selectors alone must not turn an old no-op rule into a claimed guard.
	if err := n.handle.RuleAdd(guard); err != nil && !errors.Is(err, unix.EEXIST) {
		return fmt.Errorf("mpls context guard install: %w", err)
	}
	if n.contexts.guards == nil {
		n.contexts.guards = make(map[int]*netlink.Rule)
	}
	n.contexts.guards[family] = guard
	return nil
}

func (n *netlinkBackend) ensureMPLSSelector(key mplsRuleKey) error {
	owned := n.contexts.selectors[key]
	rules, err := n.handle.RuleList(key.family)
	if err != nil {
		return err
	}
	found := false
	for i := range rules {
		rule := &rules[i]
		if owned != nil && sameMPLSRule(rule, owned) {
			found = true
			continue
		}
		if guard := n.contexts.guards[key.family]; guard != nil && sameMPLSRule(rule, guard) {
			continue
		}
		// Local delivery remains ahead of private forwarding. Only the kernel's
		// built-in local-table rule is exempt from the precedence conflict check.
		if rule.Priority == 0 && rule.Table == unix.RT_TABLE_LOCAL && rule.Protocol == unix.RTPROT_KERNEL {
			continue
		}
		if rule.Table == int(key.tableID) {
			return fmt.Errorf("mpls context table %d has a foreign selector: %w", key.tableID, unix.EEXIST)
		}
		if !mplsRuleMatchesMark(rule, key.tableID) {
			continue
		}
		if rule.Priority <= mplsGuardPriority || rule.Mark != 0 {
			return fmt.Errorf("mpls context mark %#x conflicts with existing rule: %w", key.tableID, unix.EEXIST)
		}
	}
	if found {
		return nil
	}
	rule := mplsMarkRule(key.family, mplsContextPriority, key.tableID, ^uint32(0))
	rule.Table = int(key.tableID)
	rule.Type = unix.FR_ACT_TO_TBL
	if err := n.handle.RuleAdd(rule); err != nil {
		return fmt.Errorf("mpls context selector install: %w", err)
	}
	if n.contexts.selectors == nil {
		n.contexts.selectors = make(map[mplsRuleKey]*netlink.Rule)
	}
	n.contexts.selectors[key] = rule
	return nil
}

func (n *netlinkBackend) checkMPLSContextForwarding(key mplsPushKey, expected *netlink.Route) error {
	routes, err := n.handle.RouteGetWithOptions(key.fec.Addr().AsSlice(), &netlink.RouteGetOptions{
		Mark: key.tableID, FIBMatch: true,
	})
	if err != nil {
		return fmt.Errorf("mpls context forwarding lookup: %w", err)
	}
	if len(routes) != 1 || !sameMPLSRoute(&routes[0], expected) {
		return fmt.Errorf("mpls context table %d does not select the installed push: %w", key.tableID, unix.EEXIST)
	}
	return nil
}

func (n *netlinkBackend) delMPLSContext(fec netip.Prefix, tableID uint32) error {
	n.contexts.mu.Lock()
	defer n.contexts.mu.Unlock()
	if n.contexts.closed {
		return errors.New("mpls context: backend closed")
	}
	return n.removeMPLSContext(mplsPushKey{tableID: tableID, fec: fec.Masked()})
}

func (n *netlinkBackend) removeMPLSContext(key mplsPushKey) error {
	family := netlink.FAMILY_V4
	if key.fec.Addr().Is6() {
		family = netlink.FAMILY_V6
	}
	if route := n.contexts.routes[key]; route != nil {
		current, err := n.handle.RouteListFiltered(family,
			&netlink.Route{Table: int(key.tableID), Dst: route.Dst},
			netlink.RT_FILTER_TABLE|netlink.RT_FILTER_DST)
		if err != nil {
			return err
		}
		for i := range current {
			if !sameMPLSRoute(&current[i], route) {
				return fmt.Errorf("mpls context route was replaced by another writer: %w", unix.EEXIST)
			}
		}
		if err := n.handle.RouteDel(route); err != nil && !errors.Is(err, unix.ESRCH) {
			return fmt.Errorf("mpls context route remove: %w", err)
		}
		delete(n.contexts.routes, key)
	}
	for other := range n.contexts.routes {
		if other.tableID == key.tableID && other.fec.Addr().Is6() == key.fec.Addr().Is6() {
			return nil
		}
	}
	ruleKey := mplsRuleKey{tableID: key.tableID, family: family}
	if rule := n.contexts.selectors[ruleKey]; rule != nil {
		if err := n.handle.RuleDel(rule); err != nil && !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("mpls context selector remove: %w", err)
		}
		delete(n.contexts.selectors, ruleKey)
	}
	return nil
}

// closeMPLSContexts runs with contexts.mu held. The fixed namespace guards
// outlive every owner so a sender that already resolved a private route cannot
// fall through to ordinary IP routing during concurrent plugin shutdown.
func (n *netlinkBackend) closeMPLSContexts() error {
	var result error
	for key := range n.contexts.routes {
		result = errors.Join(result, n.removeMPLSContext(key))
	}
	for key, rule := range n.contexts.selectors {
		if err := n.handle.RuleDel(rule); err != nil && !errors.Is(err, unix.ENOENT) {
			result = errors.Join(result, fmt.Errorf("mpls context selector cleanup: %w", err))
			continue
		}
		delete(n.contexts.selectors, key)
	}
	return result
}
