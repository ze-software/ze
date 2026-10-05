// Design: docs/architecture/fib/fib-depth-4-srv6.md -- VPP SRv6 policy and steering lifecycle.
// Related: srv6_state.go -- durable ownership and reconnect reconciliation.
// Related: fibvpp.go -- processEvent dispatches service routes to this backend.

package fibvpp

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net/netip"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/binapi/sr_types"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
)

// srv6Backend extends fibVPP with SRv6 SR steering operations.
type srv6Backend interface {
	addSRv6Steer(prefix netip.Prefix, sid netip.Addr, tableID uint32) error
	delSRv6Steer(prefix netip.Prefix, tableID uint32) error
}

// processSRv6Change handles a single best-change entry with SRv6 SID.
// Caller MUST hold f.mu.
func (f *fibVPP) processSRv6Change(c *incomingChange) {
	if f.srv6Backend == nil {
		logger().Warn("fib-vpp: SRv6 change but no SRv6 backend configured", "prefix", c.Prefix)
		return
	}
	key, err := f.srv6ChangeKey(c.Prefix, c.TableID, !c.SRv6SID.IsValid())
	if err != nil {
		logger().Error("fib-vpp: SRv6 table selection failed", "error", err)
		return
	}
	defer f.syncSRv6Route(key)
	switch c.Action.Verb() {
	case routeaction.VerbInstall, routeaction.VerbReplace:
		if !c.SRv6SID.IsValid() {
			// Program the replacement before removing SR forwarding. A failed
			// replacement MUST leave the previous steering intact.
			if err := f.replaceSRv6WithIP(c, key); err != nil {
				logger().Error("fib-vpp: SRv6 to IP replacement failed", "error", err)
				return
			}
			if err := f.srv6Backend.delSRv6Steer(key.prefix, key.table); err != nil {
				logger().Error("fib-vpp: SRv6 to IP cleanup failed", "error", err)
				return
			}
			delete(f.srv6Installed, key)
			return
		}
		if err := f.checkpointSRv6Fallback(key, c.SRv6SID); err != nil {
			logger().Error("fib-vpp: IP to SRv6 ownership checkpoint failed", "error", err)
			return
		}
		if err := f.srv6Backend.addSRv6Steer(key.prefix, c.SRv6SID, key.table); err != nil {
			logger().Error("fib-vpp: SRv6 steer failed", "prefix", c.Prefix, "sid", c.SRv6SID, "error", err)
			return
		}
		f.srv6Installed[key] = true
		if err := f.removeSRv6Fallback(key); err != nil {
			logger().Error("fib-vpp: replaced IP cleanup failed", "error", err)
			return
		}
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeInstalls.Inc()
		}
	case routeaction.VerbRemove:
		if err := f.removeSRv6Fallback(key); err != nil {
			logger().Error("fib-vpp: withdrawal IP cleanup failed", "error", err)
			return
		}
		if err := f.srv6Backend.delSRv6Steer(key.prefix, key.table); err != nil {
			logger().Error("fib-vpp: SRv6 steer del failed", "prefix", c.Prefix, "error", err)
			return
		}
		delete(f.srv6Installed, key)
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeRemovals.Inc()
		}
	case routeaction.VerbSkip:
		// Unspecified and unknown actions do not change SRv6 forwarding.
	default:
		panic("BUG: invalid normalized route verb")
	}
}

// replaceSRv6WithIP MUST install ordinary forwarding before steering is
// removed. The existing MPLS backend supports only its configured table.
func (f *fibVPP) replaceSRv6WithIP(c *incomingChange, key srv6RouteKey) error {
	if len(c.Labels) > 0 {
		if f.mplsBackend == nil {
			return errors.New("SRv6 replacement requires an MPLS backend")
		}
		if key.table != f.srv6TableID {
			return errors.New("MPLS replacement requires the configured VPP table")
		}
		old, err := f.beginSRv6Fallback(c, key)
		if err != nil {
			return err
		}
		if err := f.finishSRv6Fallback(key, old, f.mplsBackend.addMPLSRoute(key.prefix, c.NextHop, c.Labels)); err != nil {
			return err
		}
		f.mplsInstalled[key.prefix.String()] = true
		delete(f.installed, key)
		return nil
	}
	old, err := f.beginSRv6Fallback(c, key)
	if err != nil {
		return err
	}
	route := changeToRichRoute(c)
	route.Prefix, route.TableID = key.prefix, key.table
	var mutationErr error
	if backend, ok := f.backend.(*govppBackend); ok {
		mutationErr = backend.addRichRouteInTable(route, key.table)
	} else {
		mutationErr = f.backend.replaceRichRoute(route)
	}
	if err := f.finishSRv6Fallback(key, old, mutationErr); err != nil {
		return err
	}
	f.installed[key] = installedRoute{nextHop: c.NextHop.String()}
	if key.table == f.srv6TableID {
		delete(f.mplsInstalled, key.prefix.String())
	}
	return nil
}

// removeSRv6Fallback removes an older ordinary route hidden by SR steering.
// A withdrawal MUST remove it before removing steering, or stale IP forwarding
// would become visible again after the service route leaves.
func (f *fibVPP) removeSRv6Fallback(key srv6RouteKey) error {
	if _, found := f.installed[key]; found {
		var err error
		if backend, ok := f.backend.(*govppBackend); ok {
			err = backend.delRouteInTable(key.prefix, key.table)
		} else {
			err = f.backend.delRichRoute(key.prefix, key.table)
		}
		if err != nil {
			return err
		}
	}
	if key.table == f.srv6TableID && f.mplsInstalled[key.prefix.String()] {
		if f.mplsBackend == nil {
			return errors.New("SRv6 cleanup requires the previous MPLS backend")
		}
		if err := f.mplsBackend.delMPLSRoute(key.prefix, nil); err != nil {
			return err
		}
	}
	if backend, ok := f.srv6Backend.(*govppSRv6Backend); ok {
		if err := backend.removeFallback(key); err != nil {
			return err
		}
	}
	// Keep cleanup ownership until durable removal succeeds as well.
	delete(f.installed, key)
	if key.table == f.srv6TableID {
		delete(f.mplsInstalled, key.prefix.String())
	}
	return nil
}

func (f *fibVPP) hasSRv6Route(prefix netip.Prefix, table uint32) bool {
	prefix = prefix.Masked()
	if table != 0 {
		key := srv6RouteKey{prefix: prefix, table: table}
		if backend, ok := f.srv6Backend.(*govppSRv6Backend); ok && backend.fallbacks[key] != nil {
			return true
		}
		return f.srv6Installed[key]
	}
	if backend, ok := f.srv6Backend.(*govppSRv6Backend); ok {
		for key := range backend.fallbacks {
			if key.prefix == prefix {
				return true
			}
		}
	}
	for key := range f.srv6Installed {
		if key.prefix == prefix {
			return true
		}
	}
	return false
}

// Zero-table withdrawals from sysrib omit the original table. A unique retained
// prefix recovers it; multiple tables are ambiguous and MUST NOT be guessed.
func (f *fibVPP) srv6ChangeKey(prefix netip.Prefix, table uint32, recoverTable bool) (srv6RouteKey, error) {
	key := srv6RouteKey{prefix: prefix.Masked(), table: table}
	if table != 0 {
		return key, nil
	}
	key.table = f.srv6TableID
	if !recoverTable {
		return key, nil
	}
	found := false
	for owned := range f.srv6Installed {
		if owned.prefix != key.prefix {
			continue
		}
		if found && owned != key {
			return key, fmt.Errorf("SRv6 prefix %s is installed in multiple tables; table is required", prefix)
		}
		key, found = owned, true
	}
	for owned := range f.installed {
		if owned.prefix != key.prefix {
			continue
		}
		if found && owned != key {
			return key, fmt.Errorf("VPP prefix %s is installed in multiple tables; table is required", prefix)
		}
		key, found = owned, true
	}
	if backend, ok := f.srv6Backend.(*govppSRv6Backend); ok {
		for owned := range backend.fallbacks {
			if owned.prefix != key.prefix {
				continue
			}
			if found && owned != key {
				return key, fmt.Errorf("VPP prefix %s is installed in multiple tables; table is required", prefix)
			}
			key, found = owned, true
		}
	}
	return key, nil
}

// restoreSRv6 imports durable-owned steering into the real consumer so a
// post-restart withdrawal without a SID reaches the same policy lifecycle.
func (f *fibVPP) restoreSRv6() error {
	backend, ok := f.srv6Backend.(*govppSRv6Backend)
	if !ok {
		return nil
	}
	if backend.ready {
		return nil
	}
	if err := backend.restore(); err != nil {
		return err
	}
	clear(f.srv6Installed)
	for key := range backend.routes {
		f.srv6Installed[key] = true
	}
	for key, fallback := range backend.fallbacks {
		if fallback.MPLS {
			if key.table != f.srv6TableID {
				backend.ready = false
				return errors.New("retained MPLS fallback requires its original configured VPP table")
			}
			f.mplsInstalled[key.prefix.String()] = true
		} else {
			f.installed[key] = installedRoute{nextHop: fallback.NextHop}
		}
	}
	return nil
}

func (f *fibVPP) syncSRv6Route(key srv6RouteKey) {
	backend, ok := f.srv6Backend.(*govppSRv6Backend)
	if !ok {
		return
	}
	if !backend.ready {
		return
	}
	if backend.routes[key] != nil {
		f.srv6Installed[key] = true
	} else {
		delete(f.srv6Installed, key)
	}
}

// govppSRv6Backend owns policies and steering through durable state records.
// Not safe for concurrent use: callers MUST serialize operations with f.mu.
// A policy MUST outlive every steering reference; delSRv6Steer releases the
// policy only after its last steering entry has been removed.
type govppSRv6Backend struct {
	ch        api.Channel
	tableID   uint32
	store     srv6StateStore
	ready     bool
	policies  map[netip.Addr]*srv6Policy
	bySID     map[netip.Addr]*srv6Policy
	routes    map[srv6RouteKey]*srv6Route
	fallbacks map[srv6RouteKey]*srv6Fallback
}

func newGovppSRv6Backend(ch api.Channel, tableID uint32, store srv6StateStore) *govppSRv6Backend {
	return &govppSRv6Backend{ch: ch, tableID: tableID, store: store}
}

// addSRv6Steer MUST acquire a policy before installing a steering reference.
// RFC 9252 Section 1: "The ingress PE encapsulates the payload in an outer IPv6
// header where the destination address is the SRv6 Service SID provided by the
// egress PE."
func (b *govppSRv6Backend) addSRv6Steer(prefix netip.Prefix, sid netip.Addr, tableID uint32) error {
	if !prefix.IsValid() {
		return errors.New("SRv6 steering requires a valid prefix")
	}
	if !sid.Is6() {
		return errors.New("SRv6 service SID must be IPv6")
	}
	if sid.Is4In6() {
		return errors.New("SRv6 service SID must not be IPv4-mapped")
	}
	if !sid.IsGlobalUnicast() {
		return errors.New("SRv6 service SID must be unicast")
	}
	if sid.Zone() != "" {
		return errors.New("SRv6 service SID must not have a zone")
	}
	if err := b.restore(); err != nil {
		return err
	}
	key := b.routeKey(prefix, tableID)
	previous := b.routes[key]
	if err := b.checkSteeringOwner(key, previous); err != nil {
		return err
	}
	if previous != nil {
		if b.policies[previous.Current].SID == sid && b.policies[previous.Current].Table == b.tableID {
			return nil
		}
	}
	if err := b.reserveState(b.steeringGrowth(key, sid)); err != nil {
		return err
	}
	policy, err := b.acquirePolicy(sid)
	if err != nil {
		return b.recoverError(err)
	}
	next := srv6Route{Prefix: key.prefix, Table: key.table, Next: policy.BSID, Pending: true}
	if previous != nil {
		next.Current = previous.Current
	}
	if err := b.saveRoute(&next); err != nil {
		return b.recoverError(err)
	}
	if err := b.steer(&next, policy.BSID, false); err != nil {
		return b.recoverError(err)
	}
	next.Current, next.Next, next.Pending = policy.BSID, netip.Addr{}, false
	if err := b.saveRoute(&next); err != nil {
		return b.recoverError(err)
	}
	b.routes[key] = &next
	policy.refs++
	if previous != nil {
		old := b.policies[previous.Current]
		old.refs--
		if err := b.releasePolicy(old); err != nil {
			return b.recoverError(err)
		}
	}
	return nil
}

// delSRv6Steer MUST remove steering before releasing the last policy reference.
// Unknown routes are not ours to delete, even if another writer uses the prefix.
func (b *govppSRv6Backend) delSRv6Steer(prefix netip.Prefix, tableID uint32) error {
	if err := b.restore(); err != nil {
		return err
	}
	key := srv6RouteKey{prefix: prefix.Masked(), table: tableID}
	if b.routes[key] == nil {
		key = b.routeKey(prefix, tableID)
	}
	route := b.routes[key]
	if route == nil {
		return nil
	}
	if err := b.checkSteeringOwner(key, route); err != nil {
		return err
	}
	pending := *route
	pending.Pending = true
	if err := b.saveRoute(&pending); err != nil {
		return err
	}
	if err := b.steer(route, netip.Addr{}, true); err != nil {
		return b.recoverError(err)
	}
	if err := b.store.RemoveKey(srv6RouteStateKey(key)); err != nil {
		return b.recoverError(fmt.Errorf("remove SRv6 steering state: %w", err))
	}
	delete(b.routes, key)
	policy := b.policies[route.Current]
	policy.refs--
	if err := b.releasePolicy(policy); err != nil {
		return b.recoverError(err)
	}
	return nil
}

func (b *govppSRv6Backend) routeKey(prefix netip.Prefix, tableID uint32) srv6RouteKey {
	if tableID == 0 {
		tableID = b.tableID
	}
	return srv6RouteKey{prefix: prefix.Masked(), table: tableID}
}

func (b *govppSRv6Backend) steer(route *srv6Route, bsid netip.Addr, remove bool) error {
	req := &sr.SrSteeringAddDel{
		IsDel: remove, TableID: route.Table, Prefix: toVPPPrefix(route.Prefix),
		TrafficType: steerTypeForPrefix(route.Prefix),
	}
	if bsid.IsValid() {
		req.BsidAddr = toIP6Address(bsid)
	}
	reply := &sr.SrSteeringAddDelReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("sr steering: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("sr steering retval=%d", reply.Retval)
	}
	return nil
}

// RFC 9252 Section 1: "The ingress PE encapsulates the payload in an outer IPv6
// header where the destination address is the SRv6 Service SID provided by the
// egress PE."
func (b *govppSRv6Backend) acquirePolicy(sid netip.Addr) (*srv6Policy, error) {
	if policy := b.bySID[sid]; policy != nil {
		live, err := b.dumpPolicies()
		if err != nil {
			return nil, err
		}
		if actual, ok := live[toIP6Address(policy.BSID)]; !ok || !sameSRv6Policy(policy, actual) {
			return nil, fmt.Errorf("owned SRv6 policy %s changed before reuse", policy.BSID)
		}
		return policy, nil
	}
	var address [16]byte
	if _, err := rand.Read(address[:]); err != nil {
		return nil, fmt.Errorf("allocate SRv6 binding SID: %w", err)
	}
	address[0] = 0xfd
	bsid := netip.AddrFrom16(address)
	if bsid == sid {
		return nil, errors.New("SRv6 binding SID collided with service SID")
	}
	if b.policies[bsid] != nil {
		return nil, errors.New("SRv6 binding SID collided with an owned policy")
	}
	policy := &srv6Policy{BSID: bsid, SID: sid, Table: b.tableID}
	// A durable planned record MUST precede the API mutation. Only a successful
	// add followed by durable acknowledgement grants confirmed ownership.
	if err := b.savePolicy(policy); err != nil {
		return nil, err
	}
	req := &sr.SrPolicyAdd{
		BsidAddr: toIP6Address(bsid), Weight: 1, IsEncap: true, FibTable: b.tableID,
		Sids: sr.Srv6SidList{NumSids: 1, Weight: 1},
	}
	req.Sids.Sids[0] = toIP6Address(sid)
	reply := &sr.SrPolicyAddReply{}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return nil, fmt.Errorf("sr policy add: %w", err)
	}
	if reply.Retval != 0 {
		// VPP refused creation; this record MUST NOT authorize later adoption
		// of a colliding foreign policy, even when its contents happen to match.
		err := b.store.RemoveKey(srv6PolicyStateKey(bsid))
		return nil, errors.Join(fmt.Errorf("sr policy add retval=%d", reply.Retval), err)
	}
	policy.Confirmed = true
	if err := b.savePolicy(policy); err != nil {
		return nil, err
	}
	b.policies[bsid], b.bySID[sid] = policy, policy
	return policy, nil
}

func (b *govppSRv6Backend) releasePolicy(policy *srv6Policy) error {
	if policy.refs != 0 {
		return nil
	}
	live, err := b.dumpPolicies()
	if err != nil {
		return err
	}
	if current := live[toIP6Address(policy.BSID)]; current != nil {
		if !sameSRv6Policy(policy, current) {
			return fmt.Errorf("SRv6 policy %s changed before cleanup", policy.BSID)
		}
	}
	routes, err := b.dumpRoutes()
	if err != nil {
		return err
	}
	for _, bsid := range routes {
		if bsid == policy.BSID {
			return fmt.Errorf("SRv6 policy %s still has live steering references", bsid)
		}
	}
	reply := &sr.SrPolicyDelReply{}
	req := &sr.SrPolicyDel{BsidAddr: toIP6Address(policy.BSID)}
	if err := b.ch.SendRequest(req).ReceiveReply(reply); err != nil {
		return fmt.Errorf("sr policy delete: %w", err)
	}
	if reply.Retval != 0 {
		return fmt.Errorf("sr policy delete retval=%d", reply.Retval)
	}
	if err := b.store.RemoveKey(srv6PolicyStateKey(policy.BSID)); err != nil {
		return fmt.Errorf("remove SRv6 policy state: %w", err)
	}
	delete(b.policies, policy.BSID)
	if b.bySID[policy.SID] == policy {
		delete(b.bySID, policy.SID)
	}
	return nil
}

// recoverError resolves ambiguous API outcomes before any cleanup. In
// particular, a lost successful replacement reply MUST NOT delete its policy.
func (b *govppSRv6Backend) recoverError(err error) error {
	b.ready = false
	return errors.Join(err, b.restore())
}

// checkSteeringOwner uses a live dump because the VPP API has no conditional
// steering replace/delete. Other writers MUST serialize their own changes with
// Ze; a concurrent CLI mutation cannot be made atomic by this API.
func (b *govppSRv6Backend) checkSteeringOwner(key srv6RouteKey, owned *srv6Route) error {
	routes, err := b.dumpRoutes()
	if err != nil {
		return err
	}
	actual := routes[key]
	if owned == nil {
		if actual.IsValid() {
			return fmt.Errorf("SRv6 prefix %s table %d is owned by another writer", key.prefix, key.table)
		}
		return nil
	}
	if actual != owned.Current {
		return fmt.Errorf("SRv6 prefix %s table %d changed outside Ze", key.prefix, key.table)
	}
	policies, err := b.dumpPolicies()
	if err != nil {
		return err
	}
	current := policies[toIP6Address(owned.Current)]
	if current == nil {
		return fmt.Errorf("SRv6 policy %s disappeared outside Ze", owned.Current)
	}
	if !sameSRv6Policy(b.policies[owned.Current], current) {
		return fmt.Errorf("SRv6 policy %s changed outside Ze", owned.Current)
	}
	return nil
}

func steerTypeForPrefix(prefix netip.Prefix) sr_types.SrSteer {
	if prefix.Addr().Is6() {
		return sr_types.SR_STEER_API_IPV6
	}
	return sr_types.SR_STEER_API_IPV4
}

func toIP6Address(addr netip.Addr) ip_types.IP6Address {
	a16 := addr.As16()
	var ip6 ip_types.IP6Address
	copy(ip6[:], a16[:])
	return ip6
}
