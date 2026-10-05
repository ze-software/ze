// Design: docs/architecture/fib/fib-depth-4-srv6.md -- bounded durable transition ownership.
package fibvpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/netip"
	"slices"
	"strings"
	"testing"

	ipapi "go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/binapi/sr_types"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// srRPCBoundedStore models the production state-list envelope, not an unlimited
// raw Storage handle: opStateList refuses more than rpc.StateListMax keys.
// Other lifecycle tests exercise real framed-tree durability independently.
type srRPCBoundedStore struct {
	files         map[string][]byte
	failRemoveKey string
}

func (s *srRPCBoundedStore) ReadKey(key string) ([]byte, error) {
	data, found := s.files[key]
	if !found {
		return nil, fs.ErrNotExist
	}
	return slices.Clone(data), nil
}

func (s *srRPCBoundedStore) WriteKey(key string, data []byte) error {
	s.files[key] = slices.Clone(data)
	return nil
}

func (s *srRPCBoundedStore) RemoveKey(key string) error {
	if key == s.failRemoveKey {
		return errors.New("state removal failed")
	}
	delete(s.files, key)
	return nil
}

func (s *srRPCBoundedStore) ListKeys(prefix string) ([]string, error) {
	var keys []string
	for key := range s.files {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	if len(keys) > rpc.StateListMax {
		return nil, errors.New("state list exceeds key limit")
	}
	slices.Sort(keys)
	return keys, nil
}

// srIPModelBackend retains ordinary IP forwarding separately from SR steering,
// including table identity, so removing steering can reveal stale IP routes.
type srIPModelBackend struct {
	mockBackend
	routes       map[srv6RouteKey]vppRichRoute
	failDelete   bool
	beforeAdd    func(vppRichRoute)
	rejectAdd    bool
	uncertainAdd bool
}

func (b *srIPModelBackend) addRoute(prefix netip.Prefix, nextHop netip.Addr) error {
	return b.addRichRoute(vppRichRoute{Prefix: prefix, NextHop: nextHop})
}

func (b *srIPModelBackend) replaceRoute(prefix netip.Prefix, nextHop netip.Addr) error {
	return b.addRoute(prefix, nextHop)
}

func (b *srIPModelBackend) addRichRoute(route vppRichRoute) error {
	if b.beforeAdd != nil {
		b.beforeAdd(route)
	}
	if b.rejectAdd {
		return errors.New("injected known ordinary route rejection")
	}
	b.routes[srv6RouteKey{prefix: route.Prefix.Masked(), table: route.TableID}] = route
	if b.uncertainAdd {
		return errVPPMutationUncertain
	}
	return nil
}

func (b *srIPModelBackend) replaceRichRoute(route vppRichRoute) error {
	return b.addRichRoute(route)
}

func (b *srIPModelBackend) delRoute(prefix netip.Prefix) error {
	return b.delRichRoute(prefix, 0)
}

func (b *srIPModelBackend) delRichRoute(prefix netip.Prefix, table uint32) error {
	if b.failDelete {
		return errors.New("injected ordinary route cleanup failure")
	}
	delete(b.routes, srv6RouteKey{prefix: prefix.Masked(), table: table})
	return nil
}

func srCapacityFixture(t *testing.T) (*govppSRv6Backend, *srModelChannel, *srRPCBoundedStore) {
	t.Helper()
	store := &srRPCBoundedStore{files: make(map[string][]byte)}
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	backend := newGovppSRv6Backend(channel, 0, store)
	policy := &srv6Policy{
		BSID: netip.MustParseAddr("fd01::1"), SID: netip.MustParseAddr("2001:db8:1::a"), Confirmed: true,
	}
	if err := backend.savePolicy(policy); err != nil {
		t.Fatal(err)
	}
	live := sr.SrPolicyAdd{BsidAddr: toIP6Address(policy.BSID), IsEncap: true, Weight: 1,
		Sids: sr.Srv6SidList{NumSids: 1, Weight: 1}}
	live.Sids.Sids[0] = toIP6Address(policy.SID)
	channel.policies[live.BsidAddr] = live
	for i := range rpc.StateListMax - 1 {
		address := netip.AddrFrom4([4]byte{10, 0, byte(i >> 8), byte(i)})
		prefix := netip.PrefixFrom(address, 32)
		route := &srv6Route{Prefix: prefix, Current: policy.BSID}
		if err := backend.saveRoute(route); err != nil {
			t.Fatal(err)
		}
		channel.steers = append(channel.steers, sr.SrSteeringAddDel{
			BsidAddr: live.BsidAddr, Prefix: toVPPPrefix(prefix), TrafficType: sr_types.SR_STEER_API_IPV4,
		})
	}
	if err := backend.restore(); err != nil {
		t.Fatal(err)
	}
	return backend, channel, store
}

// TestSRv6RPCStateCapacityAdmission checks the actual RPC key limit before new
// shared routes, new policies, or a replacement can create any state/hardware.
func TestSRv6RPCStateCapacityAdmission(t *testing.T) {
	for _, operation := range []string{"shared-prefix", "new-policy", "replacement"} {
		t.Run(operation, func(t *testing.T) {
			backend, channel, store := srCapacityFixture(t)
			prefix := netip.MustParsePrefix("198.51.100.7/32")
			sid := netip.MustParseAddr("2001:db8:1::a")
			if operation != "shared-prefix" {
				sid = netip.MustParseAddr("2001:db8:2::b")
			}
			if operation == "replacement" {
				prefix = netip.MustParsePrefix("10.0.0.0/32")
			}
			requests := len(channel.requests)
			if err := backend.addSRv6Steer(prefix, sid, 0); err == nil {
				t.Fatal("admitted a mutation without recoverable RPC ownership capacity")
			}
			if len(store.files) != rpc.StateListMax || len(channel.requests) != requests {
				t.Fatal("capacity refusal occurred after state or hardware mutation")
			}
			restarted := newGovppSRv6Backend(channel, 0, store)
			if err := restarted.restore(); err != nil {
				t.Fatalf("capacity made owned state unrecoverable: %v", err)
			}
			if err := restarted.addSRv6Steer(netip.MustParsePrefix("10.0.0.0/32"), netip.MustParseAddr("2001:db8:1::a"), 0); err != nil {
				t.Fatalf("capacity blocked existing-key replay: %v", err)
			}
			if err := restarted.delSRv6Steer(netip.MustParsePrefix("10.0.0.0/32"), 0); err != nil {
				t.Fatalf("capacity blocked a withdrawal: %v", err)
			}
		})
	}
}

// TestSRv6RPCStateCapacityIncludesIPTransition reserves its temporary fallback
// record before programming ordinary forwarding, not after hardware succeeds.
func TestSRv6RPCStateCapacityIncludesIPTransition(t *testing.T) {
	_, channel, store := srCapacityFixture(t)
	ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
	fib := newFibVPP(ip)
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	if err := fib.restoreSRv6(); err != nil {
		t.Fatal(err)
	}
	requests := len(channel.requests)
	fib.processEvent(&incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Update, Prefix: netip.MustParsePrefix("10.0.0.0/32"),
		NextHop: netip.MustParseAddr("192.0.2.1"),
	}}})
	if len(ip.routes) != 0 || len(channel.requests) != requests || len(store.files) != rpc.StateListMax {
		t.Fatal("IP transition consumed unreserved state capacity after hardware mutation")
	}
}

// TestSRv6IPTransitionKeepsTablesDistinct follows SR->IP for the same prefix in
// two tables, then IP->SR and withdrawal in just one table.
func TestSRv6IPTransitionKeepsTablesDistinct(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
	fib := newFibVPP(ip)
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	sid := netip.MustParseAddr("2001:db8:1::a")
	for _, table := range []uint32{10, 20} {
		fib.processEvent(&incomingBatch{Changes: []incomingChange{{Action: routeaction.Add, Prefix: prefix, SRv6SID: sid, TableID: table}}})
		fib.processEvent(&incomingBatch{Changes: []incomingChange{{Action: routeaction.Update, Prefix: prefix, NextHop: netip.MustParseAddr("198.51.100.1"), TableID: table}}})
	}
	fib.processEvent(&incomingBatch{Changes: []incomingChange{{Action: routeaction.Update, Prefix: prefix, SRv6SID: sid, TableID: 10}}})
	fib.processEvent(&incomingBatch{Changes: []incomingChange{{Action: routeaction.Withdraw, Prefix: prefix, TableID: 10}}})
	if _, stale := ip.routes[srv6RouteKey{prefix: prefix, table: 10}]; stale {
		t.Fatal("withdrawal exposed stale ordinary forwarding in table 10")
	}
	if _, preserved := ip.routes[srv6RouteKey{prefix: prefix, table: 20}]; !preserved {
		t.Fatal("table 10 transition removed table 20 forwarding")
	}
}

// TestSRv6InterruptedIPTransitionsRestoreOwnership stops each transition after
// both forwarding forms exist, restarts the consumer, then explicitly withdraws.
func TestSRv6InterruptedIPTransitionsRestoreOwnership(t *testing.T) {
	for _, towardSR := range []bool{false, true} {
		t.Run(fmt.Sprint(towardSR), func(t *testing.T) {
			channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			store := newSRv6TestStore(t)
			ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
			fib := newFibVPP(ip)
			fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
			prefix := netip.MustParsePrefix("192.0.2.0/24")
			change := incomingChange{Action: routeaction.Add, Prefix: prefix, TableID: 10, NextHop: netip.MustParseAddr("198.51.100.1")}
			if !towardSR {
				change.SRv6SID = netip.MustParseAddr("2001:db8:1::a")
			}
			fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
			change.Action = routeaction.Update
			if towardSR {
				change.SRv6SID = netip.MustParseAddr("2001:db8:1::a")
				ip.failDelete = true
			} else {
				change.SRv6SID = netip.Addr{}
				channel.fail = "sr_steering_add_del"
			}
			fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
			if len(ip.routes) != 1 || len(channel.steers) != 1 {
				t.Fatal("fixture did not reach the interrupted transition window")
			}
			ip.failDelete = false
			restarted := newFibVPP(ip)
			restarted.srv6Backend = newGovppSRv6Backend(channel, 0, store)
			restarted.processEvent(&incomingBatch{Changes: []incomingChange{{Action: routeaction.Withdraw, Prefix: prefix, TableID: 10}}})
			if len(ip.routes) != 0 || len(channel.steers) != 0 || len(channel.policies) != 0 {
				t.Fatalf("restart lost transition ownership: ip=%d steering=%d policies=%d", len(ip.routes), len(channel.steers), len(channel.policies))
			}
		})
	}
}

// TestSRv6NewPrefixChecksSharedPolicy rejects a completed foreign modification
// before another prefix can attach to the cached policy; no concurrent race exists.
func TestSRv6NewPrefixChecksSharedPolicy(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	backend := newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	sid := netip.MustParseAddr("2001:db8:1::a")
	if err := backend.addSRv6Steer(netip.MustParsePrefix("192.0.2.0/24"), sid, 0); err != nil {
		t.Fatal(err)
	}
	bsid := channel.steers[0].BsidAddr
	changed := channel.policies[bsid]
	changed.Sids.Sids[0] = toIP6Address(netip.MustParseAddr("2001:db8:ffff::1"))
	channel.policies[bsid] = changed
	requests := len(channel.requests)
	if err := backend.addSRv6Steer(netip.MustParsePrefix("198.51.100.0/24"), sid, 0); err == nil {
		t.Fatal("new prefix attached to a sequentially modified shared policy")
	}
	if len(channel.steers) != 1 || len(channel.requests) != requests {
		t.Fatal("shared-policy conflict was detected only after mutation")
	}
}

// The IP checkpoint, new policy, and new steering MUST fit together; admitting
// one record at a time would mutate durable state before refusing the operation.
func TestSRv6RPCStateCapacityReservesWholeTransition(t *testing.T) {
	backend, channel, store := srCapacityFixture(t)
	for _, prefix := range []string{"10.0.0.0/32", "10.0.0.1/32"} {
		if err := backend.delSRv6Steer(netip.MustParsePrefix(prefix), 0); err != nil {
			t.Fatal(err)
		}
	}
	ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
	fib := newFibVPP(ip)
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	change := incomingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix("198.51.100.0/24"), NextHop: netip.MustParseAddr("192.0.2.1")}
	fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
	requests, records := len(channel.requests), len(store.files)
	change.Action, change.SRv6SID = routeaction.Update, netip.MustParseAddr("2001:db8:2::b")
	fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
	if len(channel.requests) != requests || len(store.files) != records || len(ip.routes) != 1 {
		t.Fatal("compound transition did not reserve all three temporary records before mutation")
	}
}

// Intent precedes the IP API. A known rejection releases it; an applied request
// whose acknowledgement was lost remains ambiguous across process restart.
func TestSRv6FallbackIntentAndUncertainReply(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			store := &srRPCBoundedStore{files: make(map[string][]byte)}
			channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
			fib := newFibVPP(ip)
			fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
			change := incomingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"),
				TableID: 10, NextHop: netip.MustParseAddr("198.51.100.1"), SRv6SID: netip.MustParseAddr("2001:db8:1::a")}
			fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
			key := srv6RouteKey{prefix: change.Prefix, table: change.TableID}
			ip.beforeAdd = func(route vppRichRoute) {
				var intent srv6Fallback
				data, err := store.ReadKey(srv6PrefixStateKey("fallback", key))
				if err != nil {
					t.Fatalf("IP API preceded durable intent: %v", err)
				}
				if err := json.Unmarshal(data, &intent); err != nil {
					t.Fatal(err)
				}
				if intent.Confirmed || intent.Prefix != route.Prefix || intent.Table != route.TableID {
					t.Fatal("IP API did not have an exact unconfirmed ownership intent")
				}
			}
			ip.uncertainAdd, ip.rejectAdd = uncertain, !uncertain
			change.Action, change.SRv6SID = routeaction.Update, netip.Addr{}
			fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
			if len(channel.steers) != 1 {
				t.Fatal("failed IP transition removed previous SR forwarding")
			}
			restarted := newFibVPP(ip)
			restarted.srv6Backend = newGovppSRv6Backend(channel, 0, store)
			if uncertain {
				if err := restarted.restoreSRv6(); err == nil || !strings.Contains(err.Error(), "unconfirmed IP mutation") {
					t.Fatalf("restart adopted an unconfirmed API result: %v", err)
				}
				restarted.processEvent(&incomingBatch{Changes: []incomingChange{{Action: routeaction.Withdraw, Prefix: change.Prefix, TableID: change.TableID}}})
				if len(ip.routes) != 1 || len(channel.steers) != 1 || len(store.files) != 3 {
					t.Fatal("ambiguous restart changed forwarding or discarded intent")
				}
				return
			}
			if len(ip.routes) != 0 || len(store.files) != 2 {
				t.Fatal("known rejection retained an unnecessary fallback intent")
			}
			ip.rejectAdd = false
			restarted.processEvent(&incomingBatch{Changes: []incomingChange{change}})
			clean := newFibVPP(ip)
			clean.srv6Backend = newGovppSRv6Backend(channel, 0, store)
			clean.processEvent(&incomingBatch{Changes: []incomingChange{{Action: routeaction.Withdraw, Prefix: change.Prefix, TableID: change.TableID}}})
			if len(ip.routes) != 0 || len(channel.steers) != 0 || len(store.files) != 0 {
				t.Fatal("confirmed ordinary fallback was not withdrawable after completed transition and restart")
			}
		})
	}
}

func TestVPPOrdinaryMutationReplyClassification(t *testing.T) {
	prefix, nextHop := netip.MustParsePrefix("192.0.2.0/24"), netip.MustParseAddr("198.51.100.1")
	for _, uncertain := range []bool{false, true} {
		t.Run(fmt.Sprint(uncertain), func(t *testing.T) {
			channel := &testChannel{retval: -1}
			if uncertain {
				channel.sendErr = errors.New("reply lost")
			}
			ip := newGovppBackend(channel, 0)
			mpls := newGovppMPLSBackend(channel, 0)
			for _, err := range []error{
				ip.addRoute(prefix, nextHop),
				ip.replaceRichRoute(vppRichRoute{Prefix: prefix, NextHop: nextHop}),
				ip.delRichRoute(prefix, 0),
				mpls.addMPLSRoute(prefix, nextHop, []uint32{100}),
				mpls.delMPLSRoute(prefix, nil),
			} {
				if err == nil || errors.Is(err, errVPPMutationUncertain) != uncertain {
					t.Fatalf("incorrect VPP reply certainty classification: %v", err)
				}
			}
		})
	}
}

func TestSRv6FallbackKeepsOriginalZeroTable(t *testing.T) {
	store := &srRPCBoundedStore{files: make(map[string][]byte)}
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	owner := newGovppSRv6Backend(channel, 0, store)
	if err := owner.restore(); err != nil {
		t.Fatal(err)
	}
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	if err := owner.saveFallback(&srv6Fallback{Prefix: prefix, NextHop: "198.51.100.1", Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	ipChannel := &testChannel{}
	fib := newFibVPP(newGovppBackend(ipChannel, 100))
	fib.srv6Backend, fib.srv6TableID = newGovppSRv6Backend(channel, 100, store), 100
	for _, action := range []routeaction.Action{routeaction.Update, routeaction.Withdraw} {
		fib.processEvent(&incomingBatch{Changes: []incomingChange{{Action: action, Prefix: prefix, NextHop: netip.MustParseAddr("198.51.100.2")}}})
		request, ok := ipChannel.lastRequest.(*ipapi.IPRouteAddDel)
		if !ok || request.Route.TableID != 0 || request.IsAdd != (action == routeaction.Update) {
			t.Fatalf("retained table zero was reinterpreted as configured table 100: %#v", ipChannel.lastRequest)
		}
	}
	if len(store.files) != 0 {
		t.Fatal("withdrawal retained ordinary ownership")
	}
}

func TestSRv6FallbackFlushPreservesSteeringOnIPDeleteFailure(t *testing.T) {
	store := &srRPCBoundedStore{files: make(map[string][]byte)}
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
	fib := newFibVPP(ip)
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	change := incomingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"), NextHop: netip.MustParseAddr("198.51.100.1")}
	fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
	ip.failDelete = true
	change.Action, change.SRv6SID = routeaction.Update, netip.MustParseAddr("2001:db8:1::a")
	fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
	fib.flushRoutes()
	if len(ip.routes) != 1 || len(channel.steers) != 1 || len(store.files) != 3 {
		t.Fatal("flush exposed ordinary forwarding after its cleanup failed")
	}
	ip.failDelete = false
	fib.flushRoutes()
	if len(ip.routes) != 0 || len(channel.steers) != 0 || len(store.files) != 0 {
		t.Fatal("flush lost retained ownership needed for retry")
	}
}

func TestSRv6FallbackFlushRetriesDurableRemoval(t *testing.T) {
	store := &srRPCBoundedStore{files: make(map[string][]byte)}
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	ip := &srIPModelBackend{routes: make(map[srv6RouteKey]vppRichRoute)}
	fib := newFibVPP(ip)
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	change := incomingChange{Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"), NextHop: netip.MustParseAddr("198.51.100.1")}
	fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
	ip.failDelete = true
	change.Action, change.SRv6SID = routeaction.Update, netip.MustParseAddr("2001:db8:1::a")
	fib.processEvent(&incomingBatch{Changes: []incomingChange{change}})
	if len(ip.routes) != 1 || len(channel.steers) != 1 || len(store.files) != 3 {
		t.Fatal("fixture must retain ordinary forwarding and its durable cleanup obligation beneath steering")
	}
	key := srv6RouteKey{prefix: change.Prefix, table: 0}
	fallbackKey := srv6PrefixStateKey("fallback", key)
	store.failRemoveKey = fallbackKey
	ip.failDelete = false
	fib.flushRoutes()
	if _, retained := store.files[fallbackKey]; !retained || len(ip.routes) != 0 {
		t.Fatal("fault must follow successful ordinary deletion and retain its durable cleanup obligation")
	}
	store.failRemoveKey = ""
	fib.flushRoutes()
	if len(ip.routes) != 0 || len(channel.steers) != 0 || len(store.files) != 0 {
		t.Fatal("flush did not retry the failed durable cleanup")
	}
}
