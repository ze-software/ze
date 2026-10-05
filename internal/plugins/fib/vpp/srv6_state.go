// Design: docs/architecture/fib/fib-depth-4-srv6.md -- durable VPP resource ownership.
package fibvpp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/binapi/sr_types"
)

// srv6StateStore is the existing daemon StateKeys raw-key contract. The owner
// MUST keep its lifetime context alive until the backend stops using it.
type srv6StateStore interface {
	ReadKey(string) ([]byte, error)
	WriteKey(string, []byte) error
	RemoveKey(string) error
	ListKeys(string) ([]string, error)
}

const srv6StatePrefix = "meta/fib-vpp/srv6/"

// srv6Policy records exclusive successful creation, not ownership by resemblance.
// A planned policy found live after a crash is ambiguous and MUST NOT be deleted.
type srv6Policy struct {
	BSID      netip.Addr `json:"bsid"`
	SID       netip.Addr `json:"sid"`
	Table     uint32     `json:"table"`
	Confirmed bool       `json:"confirmed"`
	refs      int
}

type srv6RouteKey struct {
	prefix netip.Prefix
	table  uint32
}

// srv6Route records both sides before a steering update. Reconciliation accepts
// only these identities; it never claims an unrelated steering entry.
type srv6Route struct {
	Prefix  netip.Prefix `json:"prefix"`
	Table   uint32       `json:"table"`
	Current netip.Addr   `json:"current"`
	Next    netip.Addr   `json:"next"`
	Pending bool         `json:"pending"`
}

func srv6PolicyStateKey(bsid netip.Addr) string {
	return srv6StatePrefix + "policy/" + bsid.String()
}

func srv6RouteStateKey(key srv6RouteKey) string {
	return srv6PrefixStateKey("route", key)
}

func srv6PrefixStateKey(kind string, key srv6RouteKey) string {
	var buffer [64]byte // uint32 table, slash, and the longest IPv6 prefix.
	identity := strconv.AppendUint(buffer[:0], uint64(key.table), 10)
	identity = append(identity, '/')
	identity = key.prefix.AppendTo(identity)
	digest := sha256.Sum256(identity)
	var encoded [64]byte
	hex.Encode(encoded[:], digest[:])
	return srv6StatePrefix + kind + "/" + string(encoded[:])
}

func (b *govppSRv6Backend) savePolicy(policy *srv6Policy) error {
	data, err := json.Marshal(policy)
	if err != nil {
		return fmt.Errorf("encode SRv6 policy ownership: %w", err)
	}
	if err := b.store.WriteKey(srv6PolicyStateKey(policy.BSID), data); err != nil {
		return fmt.Errorf("persist SRv6 policy ownership: %w", err)
	}
	return nil
}

func (b *govppSRv6Backend) saveRoute(route *srv6Route) error {
	data, err := json.Marshal(route)
	if err != nil {
		return fmt.Errorf("encode SRv6 steering ownership: %w", err)
	}
	key := srv6RouteKey{prefix: route.Prefix, table: route.Table}
	if err := b.store.WriteKey(srv6RouteStateKey(key), data); err != nil {
		return fmt.Errorf("persist SRv6 steering ownership: %w", err)
	}
	return nil
}

// restore MUST finish reconciliation before an API mutation. Dumps are bounded
// by VPP's live resource population; state lists by this plugin's owned keys.
func (b *govppSRv6Backend) restore() error {
	if b.ready {
		return nil
	}
	if b.store == nil {
		return errors.New("SRv6 requires durable daemon state")
	}
	if err := b.loadState(); err != nil {
		return err
	}
	livePolicies, err := b.dumpPolicies()
	if err != nil {
		return err
	}
	liveRoutes, err := b.dumpRoutes()
	if err != nil {
		return err
	}
	// Validate the complete snapshot before deleting or claiming any resource.
	for bsid, policy := range b.policies {
		live := livePolicies[toIP6Address(bsid)]
		if live == nil {
			continue
		}
		if !sameSRv6Policy(policy, live) {
			return fmt.Errorf("SRv6 owned policy %s conflicts with VPP", bsid)
		}
		if !policy.Confirmed {
			return fmt.Errorf("SRv6 policy %s has unconfirmed creation; refusing ownership", bsid)
		}
	}
	for key, route := range b.routes {
		actual := liveRoutes[key]
		if !actual.IsValid() {
			continue
		}
		allowed := actual == route.Current
		if route.Pending {
			allowed = allowed || actual == route.Next
		}
		if !allowed {
			return fmt.Errorf("SRv6 steering %s table %d has foreign BSID %s", key.prefix, key.table, actual)
		}
		if b.policies[actual] == nil {
			return fmt.Errorf("SRv6 steering %s has no durable policy ownership", key.prefix)
		}
		if livePolicies[toIP6Address(actual)] == nil {
			return fmt.Errorf("SRv6 steering %s references missing policy %s", key.prefix, actual)
		}
	}
	for key, bsid := range liveRoutes {
		if b.policies[bsid] != nil {
			if b.routes[key] == nil {
				return fmt.Errorf("SRv6 policy %s has foreign steering reference %s table %d", bsid, key.prefix, key.table)
			}
		}
	}
	for key, route := range b.routes {
		actual := liveRoutes[key]
		if !actual.IsValid() {
			if err := b.store.RemoveKey(srv6RouteStateKey(key)); err != nil {
				return fmt.Errorf("remove absent SRv6 steering state: %w", err)
			}
			delete(b.routes, key)
			continue
		}
		route.Current, route.Next, route.Pending = actual, netip.Addr{}, false
		if err := b.saveRoute(route); err != nil {
			return err
		}
		b.policies[actual].refs++
	}
	for bsid, policy := range b.policies {
		if livePolicies[toIP6Address(bsid)] == nil {
			if err := b.store.RemoveKey(srv6PolicyStateKey(bsid)); err != nil {
				return fmt.Errorf("remove absent SRv6 policy state: %w", err)
			}
			delete(b.policies, bsid)
			continue
		}
		if policy.refs == 0 {
			if err := b.releasePolicy(policy); err != nil {
				return err
			}
			continue
		}
		if policy.Table == b.tableID {
			if previous := b.bySID[policy.SID]; previous != nil {
				return fmt.Errorf("multiple owned SRv6 policies for SID %s", policy.SID)
			}
			b.bySID[policy.SID] = policy
		}
	}
	b.ready = true
	return nil
}

func (b *govppSRv6Backend) loadState() error {
	keys, err := b.store.ListKeys(srv6StatePrefix)
	if err != nil {
		return fmt.Errorf("list SRv6 ownership: %w", err)
	}
	b.policies = make(map[netip.Addr]*srv6Policy)
	b.bySID = make(map[netip.Addr]*srv6Policy)
	b.routes = make(map[srv6RouteKey]*srv6Route)
	b.fallbacks = make(map[srv6RouteKey]*srv6Fallback)
	for _, key := range keys {
		data, err := b.store.ReadKey(key)
		if err != nil {
			return fmt.Errorf("read SRv6 ownership %s: %w", key, err)
		}
		switch {
		case strings.HasPrefix(key, srv6StatePrefix+"policy/"):
			var policy srv6Policy
			if err := json.Unmarshal(data, &policy); err != nil {
				return fmt.Errorf("decode SRv6 policy state: %w", err)
			}
			if !policy.BSID.Is6() {
				return fmt.Errorf("invalid SRv6 binding SID in %s", key)
			}
			if !policy.SID.Is6() {
				return fmt.Errorf("invalid SRv6 service SID in %s", key)
			}
			if policy.BSID == policy.SID {
				return fmt.Errorf("SRv6 binding SID equals service SID in %s", key)
			}
			if srv6PolicyStateKey(policy.BSID) != key {
				return fmt.Errorf("invalid SRv6 policy state %s", key)
			}
			b.policies[policy.BSID] = &policy
		case strings.HasPrefix(key, srv6StatePrefix+"route/"):
			var route srv6Route
			if err := json.Unmarshal(data, &route); err != nil {
				return fmt.Errorf("decode SRv6 steering state: %w", err)
			}
			if !route.Prefix.IsValid() {
				return fmt.Errorf("invalid SRv6 prefix in %s", key)
			}
			if route.Prefix != route.Prefix.Masked() {
				return fmt.Errorf("noncanonical SRv6 prefix in %s", key)
			}
			identity := srv6RouteKey{prefix: route.Prefix, table: route.Table}
			if srv6RouteStateKey(identity) != key {
				return fmt.Errorf("invalid SRv6 steering state %s", key)
			}
			b.routes[identity] = &route
		case strings.HasPrefix(key, srv6StatePrefix+"fallback/"):
			var fallback srv6Fallback
			if err := json.Unmarshal(data, &fallback); err != nil {
				return fmt.Errorf("decode SRv6 fallback state: %w", err)
			}
			if !fallback.Prefix.IsValid() || fallback.Prefix != fallback.Prefix.Masked() {
				return fmt.Errorf("invalid SRv6 fallback prefix in %s", key)
			}
			identity := srv6RouteKey{prefix: fallback.Prefix, table: fallback.Table}
			if srv6PrefixStateKey("fallback", identity) != key {
				return fmt.Errorf("invalid SRv6 fallback state %s", key)
			}
			if !fallback.Confirmed {
				return fmt.Errorf("unconfirmed IP mutation for %s table %d; reconcile ownership before retry", fallback.Prefix, fallback.Table)
			}
			b.fallbacks[identity] = &fallback
		default:
			return fmt.Errorf("unknown SRv6 ownership key %s", key)
		}
	}
	return nil
}

func sameSRv6Policy(owned *srv6Policy, live *sr.SrPoliciesDetails) bool {
	if !live.IsEncap {
		return false
	}
	if live.IsSpray {
		return false
	}
	if live.FibTable != owned.Table {
		return false
	}
	if len(live.SidLists) != 1 {
		return false
	}
	segments := &live.SidLists[0]
	return segments.NumSids == 1 && segments.Weight == 1 && segments.Sids[0] == toIP6Address(owned.SID)
}

func (b *govppSRv6Backend) dumpPolicies() (map[ip_types.IP6Address]*sr.SrPoliciesDetails, error) {
	policies := make(map[ip_types.IP6Address]*sr.SrPoliciesDetails)
	dump := b.ch.SendMultiRequest(&sr.SrPoliciesDump{})
	for {
		policy := &sr.SrPoliciesDetails{}
		last, err := dump.ReceiveReply(policy)
		if err != nil {
			return nil, fmt.Errorf("dump SRv6 policies: %w", err)
		}
		if last {
			return policies, nil
		}
		policies[policy.Bsid] = policy
	}
}

func (b *govppSRv6Backend) dumpRoutes() (map[srv6RouteKey]netip.Addr, error) {
	routes := make(map[srv6RouteKey]netip.Addr)
	dump := b.ch.SendMultiRequest(&sr.SrSteeringPolDump{})
	for {
		var route sr.SrSteeringPolDetails
		last, err := dump.ReceiveReply(&route)
		if err != nil {
			return nil, fmt.Errorf("dump SRv6 steering: %w", err)
		}
		if last {
			return routes, nil
		}
		switch route.TrafficType {
		case sr_types.SR_STEER_API_IPV4, sr_types.SR_STEER_API_IPV6:
		case sr_types.SR_STEER_API_L2:
			// An L2 reference is never ours, but MUST prevent policy cleanup.
			routes[srv6RouteKey{table: uint32(route.SwIfIndex)}] = netip.AddrFrom16(route.Bsid)
			continue
		default:
			// VPP is an open API boundary; an unknown reference is not safe to ignore.
			return nil, fmt.Errorf("unknown VPP SRv6 steering traffic type %d", route.TrafficType)
		}
		var address netip.Addr
		switch route.Prefix.Address.Af {
		case ip_types.ADDRESS_IP4:
			address = netip.AddrFrom4(route.Prefix.Address.Un.GetIP4())
		case ip_types.ADDRESS_IP6:
			address = netip.AddrFrom16(route.Prefix.Address.Un.GetIP6())
		default:
			// Address family is supplied by the VPP API, not a closed Ze enum.
			return nil, fmt.Errorf("unknown VPP address family %d", route.Prefix.Address.Af)
		}
		prefix := netip.PrefixFrom(address, int(route.Prefix.Len))
		if !prefix.IsValid() {
			return nil, errors.New("invalid prefix in VPP SRv6 steering dump")
		}
		key := srv6RouteKey{prefix: prefix.Masked(), table: route.FibTable}
		routes[key] = netip.AddrFrom16(route.Bsid)
	}
}
