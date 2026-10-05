// Design: docs/architecture/fib/fib-depth-4-srv6.md -- VPP SRv6 SR steer programming
// Related: srv6.go -- addSRv6Steer, the producer this test drives

package fibvpp

// RFC naming: untagged -- this API model is not evidence of VPP packet forwarding.

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"
	"go.fd.io/govpp/binapi/sr_types"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/bgp/routeaction"
)

// srModelChannel models the policy dependency, replacement and table-specific
// steering in VPP sr_steering_policy. It deliberately rejects unknown APIs and
// reply mismatches, so adding an unmodeled lifecycle cannot silently pass.
// Not safe for concurrent use.
type srModelChannel struct {
	policies  map[ip_types.IP6Address]sr.SrPolicyAdd
	steers    []sr.SrSteeringAddDel
	requests  []string
	fail      string
	failAfter string
	collide   bool
}

var _ api.Channel = (*srModelChannel)(nil)

type srModelRequest struct {
	ch  *srModelChannel
	msg api.Message
}

func (r *srModelRequest) ReceiveReply(reply api.Message) (err error) {
	name := r.msg.GetMessageName()
	r.ch.requests = append(r.ch.requests, name)
	defer func() {
		if r.ch.failAfter == name && err == nil {
			r.ch.failAfter = ""
			err = fmt.Errorf("lost successful %s reply", name)
		}
	}()
	if r.ch.fail == name {
		r.ch.fail = ""
		return fmt.Errorf("injected %s failure", name)
	}
	switch request := r.msg.(type) {
	case *sr.SrPolicyAdd:
		out, ok := reply.(*sr.SrPolicyAddReply)
		if !ok {
			return fmt.Errorf("policy add reply mismatch: %T", reply)
		}
		if r.ch.collide {
			r.ch.collide = false
			// Another writer claimed this BSID before the exclusive API add.
			r.ch.policies[request.BsidAddr] = *request
		}
		if _, exists := r.ch.policies[request.BsidAddr]; exists {
			out.Retval = -12
			return nil
		}
		r.ch.policies[request.BsidAddr] = *request
	case *sr.SrPolicyDel:
		out, ok := reply.(*sr.SrPolicyDelReply)
		if !ok {
			return fmt.Errorf("policy delete reply mismatch: %T", reply)
		}
		if _, exists := r.ch.policies[request.BsidAddr]; !exists {
			out.Retval = -1
			return nil
		}
		for i := range r.ch.steers {
			if r.ch.steers[i].BsidAddr == request.BsidAddr {
				return errors.New("policy deleted while steering still references it")
			}
		}
		delete(r.ch.policies, request.BsidAddr)
	case *sr.SrSteeringAddDel:
		out, ok := reply.(*sr.SrSteeringAddDelReply)
		if !ok {
			return fmt.Errorf("steering reply mismatch: %T", reply)
		}
		switch request.TrafficType {
		case sr_types.SR_STEER_API_IPV4, sr_types.SR_STEER_API_IPV6:
		case sr_types.SR_STEER_API_L2:
			return errors.New("model does not accept L2 steering mutations")
		default:
			out.Retval = -1
			return nil
		}
		if !request.IsDel {
			policy, known := r.ch.policies[request.BsidAddr]
			if !known {
				out.Retval = -2
				return nil
			}
			if !policy.IsEncap && request.TrafficType == sr_types.SR_STEER_API_IPV4 {
				out.Retval = -5
				return nil
			}
		}
		for i := range r.ch.steers {
			current := &r.ch.steers[i]
			if current.Prefix != request.Prefix {
				continue
			}
			if current.TableID != request.TableID {
				continue
			}
			if current.TrafficType != request.TrafficType {
				continue
			}
			if request.IsDel {
				r.ch.steers = append(r.ch.steers[:i], r.ch.steers[i+1:]...)
			} else {
				*current = *request
			}
			return nil
		}
		if request.IsDel {
			out.Retval = -4
			return nil
		}
		r.ch.steers = append(r.ch.steers, *request)
	default:
		return fmt.Errorf("unmodeled SR request %T", r.msg)
	}
	return nil
}

func (c *srModelChannel) SendRequest(msg api.Message) api.RequestCtx {
	return &srModelRequest{ch: c, msg: msg}
}

func (c *srModelChannel) SendMultiRequest(msg api.Message) api.MultiRequestCtx {
	dump := &srModelDump{ch: c, msg: msg}
	for _, policy := range c.policies {
		dump.policies = append(dump.policies, policy)
	}
	return dump
}

type srModelDump struct {
	ch       *srModelChannel
	msg      api.Message
	index    int
	policies []sr.SrPolicyAdd
}

func (r *srModelDump) ReceiveReply(reply api.Message) (bool, error) {
	switch r.msg.(type) {
	case *sr.SrPoliciesDump:
		out, ok := reply.(*sr.SrPoliciesDetails)
		if !ok {
			return false, fmt.Errorf("policy dump reply mismatch: %T", reply)
		}
		if r.index == len(r.policies) {
			return true, nil
		}
		policy := &r.policies[r.index]
		*out = sr.SrPoliciesDetails{
			Bsid: policy.BsidAddr, IsEncap: policy.IsEncap,
			IsSpray: policy.IsSpray, FibTable: policy.FibTable,
			NumSidLists: 1, SidLists: []sr.Srv6SidList{policy.Sids},
		}
		r.index++
		return false, nil
	case *sr.SrSteeringPolDump:
		out, ok := reply.(*sr.SrSteeringPolDetails)
		if !ok {
			return false, fmt.Errorf("steering dump reply mismatch: %T", reply)
		}
		if r.index == len(r.ch.steers) {
			return true, nil
		}
		steer := &r.ch.steers[r.index]
		*out = sr.SrSteeringPolDetails{
			Bsid: steer.BsidAddr, FibTable: steer.TableID,
			Prefix: steer.Prefix, TrafficType: steer.TrafficType,
		}
		r.index++
		return false, nil
	default:
		return false, fmt.Errorf("unmodeled SR dump %T", r.msg)
	}
}

func (c *srModelChannel) SubscribeNotification(chan api.Message, api.Message) (api.SubscriptionCtx, error) {
	return nil, errors.New("srModelChannel models no notifications")
}

func (c *srModelChannel) SetReplyTimeout(time.Duration)          {}
func (c *srModelChannel) CheckCompatiblity(...api.Message) error { return nil }
func (c *srModelChannel) Close()                                 {}

// VALIDATES: the VPP backend programs an SRv6 service route so that VPP
// encapsulates it in IPv6 toward the received Service SID: an SR policy in
// encapsulation mode whose segment list is exactly the SID, and a steering
// entry for the prefix that names that policy's BSID.
// PREVENTS: addSRv6Steer sending only sr_steering_add_del with the remote
// Service SID as the BSID. VPP refuses that with retval -2 because no local SR
// policy carries the BSID, so the route is never installed and an ingress PE
// on VPP does not perform the IPv6 encapsulation RFC 9252 Section 5 requires.
// The model proves API state, not RFC interoperability or packet forwarding.
func TestRFC9252VPPServiceRouteEncapsulatesTowardTheSID(t *testing.T) {
	channel := &srModelChannel{policies: map[ip_types.IP6Address]sr.SrPolicyAdd{}}
	backend := newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	sid := netip.MustParseAddr("2001:db8:1:2:abcd::")

	if err := backend.addSRv6Steer(prefix, sid, 0); err != nil {
		t.Fatalf("addSRv6Steer(%v, %v) = %v, want the route steered into an IPv6 encapsulation", prefix, sid, err)
	}
	if len(channel.steers) != 1 {
		t.Fatalf("steering entries = %d, want 1", len(channel.steers))
	}
	policy, ok := channel.policies[channel.steers[0].BsidAddr]
	if !ok {
		t.Fatalf("steering BSID %v names no SR policy", channel.steers[0].BsidAddr)
	}
	if policy.BsidAddr == toIP6Address(sid) {
		t.Error("received Service SID was reused as a local binding SID")
	}
	if !policy.IsEncap {
		t.Errorf("SR policy IsEncap = false, want the encapsulation behavior")
	}
	if policy.Sids.NumSids != 1 || policy.Sids.Sids[0] != toIP6Address(sid) {
		t.Errorf("SR policy segment list = %d %v, want exactly [%v]", policy.Sids.NumSids, policy.Sids.Sids[0], sid)
	}
}

// newSRv6TestStore uses the actual durable framed-tree implementation; a fake
// channel is never confused with fake persistence or dataplane evidence.
func newSRv6TestStore(t *testing.T) storage.Storage {
	t.Helper()
	store, err := storage.Create(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Error(err)
		}
	})
	return store
}

// TestSRv6ConsumerLifecycle drives the real event consumer, including multiple
// tenant tables, shared policies, durable restart, replacement and withdrawal.
func TestSRv6ConsumerLifecycle(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	store := newSRv6TestStore(t)
	fib := newFibVPP(&mockBackend{})
	fib.srv6TableID = 7
	fib.srv6Backend = newGovppSRv6Backend(channel, 7, store)
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	v6 := netip.MustParsePrefix("2001:db8:10::/64")
	sid := netip.MustParseAddr("2001:db8:1::a")
	replacement := netip.MustParseAddr("2001:db8:2::b")
	changes := []incomingChange{
		{Action: routeaction.Add, Prefix: prefix, SRv6SID: sid, TableID: 10},
		{Action: routeaction.Add, Prefix: prefix, SRv6SID: sid, TableID: 20},
		{Action: routeaction.Add, Prefix: v6, SRv6SID: sid, TableID: 20},
	}
	fib.processEvent(&incomingBatch{Changes: changes})
	if len(channel.policies) != 1 || len(channel.steers) != 3 {
		t.Fatalf("shared state: policies=%d steering=%d", len(channel.policies), len(channel.steers))
	}
	for _, policy := range channel.policies {
		if policy.FibTable != 7 {
			t.Errorf("outer lookup table=%d, want configured underlay 7", policy.FibTable)
		}
	}
	if channel.steers[2].TrafficType != sr_types.SR_STEER_API_IPV6 {
		t.Error("IPv6 prefix did not use IPv6 steering")
	}
	// A fresh consumer/backend stands in for process restart against live VPP.
	fib = newFibVPP(&mockBackend{})
	fib.srv6TableID = 7
	fib.srv6Backend = newGovppSRv6Backend(channel, 7, store)
	requests := len(channel.requests)
	fib.processEvent(&incomingBatch{Changes: changes})
	if len(channel.requests) != requests {
		t.Error("restart replay duplicated an existing policy or steering entry")
	}
	fib.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Update, Prefix: prefix, SRv6SID: replacement, TableID: 10},
	}})
	if len(channel.policies) != 2 || len(channel.steers) != 3 {
		t.Fatal("replacement removed a shared policy or another table's steering")
	}
	fib.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Withdraw, Prefix: prefix, TableID: 20},
		{Action: routeaction.Withdraw, Prefix: v6}, // Sysrib omits table on withdrawal.
	}})
	if len(channel.policies) != 1 || len(channel.steers) != 1 {
		t.Fatal("last shared reference did not release the old policy")
	}
	fib.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Withdraw, Prefix: prefix, TableID: 10},
	}})
	keys, err := store.ListKeys(srv6StatePrefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(channel.policies) != 0 || len(channel.steers) != 0 || len(keys) != 0 {
		t.Fatalf("withdrawal leaked resources: policies=%d steers=%d keys=%v", len(channel.policies), len(channel.steers), keys)
	}
}

// TestSRv6ReplacementFailures checks that failures at each API boundary preserve
// the previous forwarding policy and release any unreferenced replacement.
func TestSRv6ReplacementFailures(t *testing.T) {
	for _, message := range []string{"sr_policy_add", "sr_steering_add_del", "sr_policy_del"} {
		t.Run(message, func(t *testing.T) {
			channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			backend := newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
			prefix := netip.MustParsePrefix("192.0.2.0/24")
			sid := netip.MustParseAddr("2001:db8:1::a")
			next := netip.MustParseAddr("2001:db8:2::b")
			if err := backend.addSRv6Steer(prefix, sid, 0); err != nil {
				t.Fatal(err)
			}
			channel.fail = message
			if err := backend.addSRv6Steer(prefix, next, 0); err == nil {
				t.Fatal("injected API failure was silently accepted")
			}
			if len(channel.steers) != 1 || len(channel.policies) != 1 {
				t.Fatalf("failed replacement destroyed forwarding or leaked policy: %d/%d", len(channel.steers), len(channel.policies))
			}
			want := sid
			if message == "sr_policy_del" {
				want = next // Steering committed; cleanup must not roll it back.
			}
			policy := channel.policies[channel.steers[0].BsidAddr]
			if policy.Sids.Sids[0] != toIP6Address(want) {
				t.Fatalf("forwarding SID=%v want %v", policy.Sids.Sids[0], want)
			}
			if err := backend.addSRv6Steer(prefix, next, 0); err != nil {
				t.Fatal(err)
			}
			if err := backend.delSRv6Steer(prefix, 0); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestSRv6VPPRecreation distinguishes restart with retained VPP state from a
// VPP restart: absent resources are recreated from the real consumer replay.
func TestSRv6VPPRecreation(t *testing.T) {
	store := newSRv6TestStore(t)
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	fib := newFibVPP(&mockBackend{})
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	batch := &incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Add, Prefix: netip.MustParsePrefix("192.0.2.0/24"),
		SRv6SID: netip.MustParseAddr("2001:db8:1::a"),
	}}}
	fib.processEvent(batch)
	channel = &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	fib = newFibVPP(&mockBackend{})
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, store)
	fib.processEvent(batch)
	if len(channel.policies) != 1 || len(channel.steers) != 1 {
		t.Fatal("VPP restart replay did not recreate policy and steering")
	}
	fib.flushRoutes()
	if len(channel.policies) != 0 || len(channel.steers) != 0 {
		t.Fatal("flush did not release recreated resources")
	}
}

// TestSRv6LostReplacementReply proves reconciliation keeps the newly committed
// policy instead of deleting it as a rollback after an ambiguous timeout.
func TestSRv6LostReplacementReply(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	backend := newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	sid := netip.MustParseAddr("2001:db8:1::a")
	next := netip.MustParseAddr("2001:db8:2::b")
	if err := backend.addSRv6Steer(prefix, sid, 0); err != nil {
		t.Fatal(err)
	}
	channel.failAfter = "sr_steering_add_del"
	if err := backend.addSRv6Steer(prefix, next, 0); err == nil {
		t.Fatal("lost reply was hidden")
	}
	if len(channel.policies) != 1 || len(channel.steers) != 1 {
		t.Fatal("ambiguous replacement destroyed forwarding or leaked old policy")
	}
	policy := channel.policies[channel.steers[0].BsidAddr]
	if policy.Sids.Sids[0] != toIP6Address(next) {
		t.Fatal("lost reply rolled back successful forwarding")
	}
}

// TestSRv6PlannedOwnershipRefusesLive tests the crash window explicitly:
// planned+absent is discarded, while planned+live is not ownership proof.
func TestSRv6PlannedOwnershipRefusesLive(t *testing.T) {
	for _, live := range []bool{false, true} {
		t.Run(fmt.Sprint(live), func(t *testing.T) {
			channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			store := newSRv6TestStore(t)
			backend := newGovppSRv6Backend(channel, 0, store)
			planned := &srv6Policy{
				BSID: netip.MustParseAddr("fd01::1234"), SID: netip.MustParseAddr("2001:db8:1::a"),
			}
			if err := backend.savePolicy(planned); err != nil {
				t.Fatal(err)
			}
			if live {
				policy := sr.SrPolicyAdd{
					BsidAddr: toIP6Address(planned.BSID), IsEncap: true, Weight: 1,
					Sids: sr.Srv6SidList{NumSids: 1, Weight: 1},
				}
				policy.Sids.Sids[0] = toIP6Address(planned.SID)
				channel.policies[policy.BsidAddr] = policy
			}
			err := backend.restore()
			if live {
				if err == nil {
					t.Fatal("planned live policy was silently adopted")
				}
				if !strings.Contains(err.Error(), planned.BSID.String()) {
					t.Fatalf("ambiguous ownership error omitted the binding SID: %v", err)
				}
				if len(channel.policies) != 1 || len(channel.requests) != 0 {
					t.Fatal("ambiguous live policy was mutated")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				keys, err := store.ListKeys(srv6StatePrefix)
				if err != nil || len(keys) != 0 {
					t.Fatalf("absent planned ownership not removed: %v %v", keys, err)
				}
			}
		})
	}
}

// TestSRv6ForeignStateIsPreserved covers foreign steering, externally changed
// policy contents and an extra foreign reference to an otherwise owned policy.
func TestSRv6ForeignStateIsPreserved(t *testing.T) {
	for _, conflict := range []string{"steering", "policy", "reference"} {
		t.Run(conflict, func(t *testing.T) {
			channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			store := newSRv6TestStore(t)
			backend := newGovppSRv6Backend(channel, 0, store)
			prefix := netip.MustParsePrefix("192.0.2.0/24")
			sid := netip.MustParseAddr("2001:db8:1::a")
			if err := backend.addSRv6Steer(prefix, sid, 0); err != nil {
				t.Fatal(err)
			}
			switch conflict {
			case "steering":
				channel.steers[0].BsidAddr = toIP6Address(netip.MustParseAddr("fd02::99"))
			case "policy":
				bsid := channel.steers[0].BsidAddr
				policy := channel.policies[bsid]
				policy.IsEncap = false
				channel.policies[bsid] = policy
			case "reference":
				foreign := channel.steers[0]
				foreign.TableID = 99
				channel.steers = append(channel.steers, foreign)
			}
			before := len(channel.requests)
			restarted := newGovppSRv6Backend(channel, 0, store)
			if err := restarted.delSRv6Steer(prefix, 0); err == nil {
				t.Fatal("conflicting ownership was accepted")
			}
			if len(channel.requests) != before {
				t.Fatal("ownership conflict mutated VPP")
			}
		})
	}
}

type failingSRv6Store struct {
	srv6StateStore
}

func (s failingSRv6Store) WriteKey(string, []byte) error {
	return errors.New("injected durable write failure")
}

// TestSRv6PersistenceRequired refuses unavailable/corrupt state and durable
// write failure before any API mutation.
func TestSRv6PersistenceRequired(t *testing.T) {
	for _, mode := range []string{"unavailable", "corrupt", "write-failed"} {
		t.Run(mode, func(t *testing.T) {
			channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
			var store srv6StateStore
			switch mode {
			case "unavailable":
			case "corrupt":
				store = newSRv6TestStore(t)
				if err := store.WriteKey(srv6StatePrefix+"policy/fd01::1", []byte("{bad")); err != nil {
					t.Fatal(err)
				}
			case "write-failed":
				store = failingSRv6Store{newSRv6TestStore(t)}
			}
			backend := newGovppSRv6Backend(channel, 0, store)
			err := backend.addSRv6Steer(netip.MustParsePrefix("192.0.2.0/24"), netip.MustParseAddr("2001:db8:1::a"), 0)
			if err == nil || len(channel.requests) != 0 {
				t.Fatalf("unusable persistence permitted mutation: error=%v requests=%v", err, channel.requests)
			}
		})
	}
}

// TestSRv6ModelRejectsMissingPolicy pins the original defect independently of
// the producer; the model must not become a request-forwarding echo.
func TestSRv6ModelRejectsMissingPolicy(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	request := &sr.SrSteeringAddDel{
		BsidAddr:    toIP6Address(netip.MustParseAddr("2001:db8::1")),
		Prefix:      toVPPPrefix(netip.MustParsePrefix("192.0.2.0/24")),
		TrafficType: sr_types.SR_STEER_API_IPV4,
	}
	reply := &sr.SrSteeringAddDelReply{}
	if err := channel.SendRequest(request).ReceiveReply(reply); err != nil {
		t.Fatal(err)
	}
	if reply.Retval != -2 || len(channel.steers) != 0 {
		t.Fatal("model accepted steering without its policy")
	}
	if err := channel.SendRequest(request).ReceiveReply(&sr.SrPolicyAddReply{}); err == nil {
		t.Fatal("model silently ignored a reply mismatch")
	}
	if err := channel.SendRequest(&sr.SrPoliciesDump{}).ReceiveReply(reply); err == nil {
		t.Fatal("model silently accepted an unmodeled single-request API")
	}
}

// TestSRv6RollbackFailureIsReported keeps both the failed steering operation and
// a failed cleanup visible, while preserving the old route.
func TestSRv6RollbackFailureIsReported(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	backend := newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	if err := backend.addSRv6Steer(prefix, netip.MustParseAddr("2001:db8:1::a"), 0); err != nil {
		t.Fatal(err)
	}
	channel.fail = "sr_steering_add_del"
	channel.failAfter = "sr_policy_del"
	err := backend.addSRv6Steer(prefix, netip.MustParseAddr("2001:db8:2::b"), 0)
	if err == nil {
		t.Fatal("failed operation and cleanup were hidden")
	}
	if !strings.Contains(err.Error(), "sr_steering_add_del") || !strings.Contains(err.Error(), "sr_policy_del") {
		t.Fatalf("lost operation or rollback error: %v", err)
	}
	if len(channel.steers) != 1 {
		t.Fatal("rollback failure destroyed the old steering")
	}
	if err := backend.restore(); err != nil {
		t.Fatal(err)
	}
	if len(channel.policies) != 1 {
		t.Fatal("reconciliation did not remove absent cleanup ownership")
	}
}

// TestSRv6RejectsInvalidSID ensures invalid input is an error rather than an
// As16 panic or a VPP policy silently encapsulating toward an invalid address.
func TestSRv6RejectsInvalidSID(t *testing.T) {
	for _, sid := range []netip.Addr{
		{}, netip.MustParseAddr("192.0.2.1"), netip.MustParseAddr("::ffff:192.0.2.1"),
		netip.MustParseAddr("::"), netip.MustParseAddr("ff02::1"), netip.MustParseAddr("fe80::1%eth0"),
	} {
		channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
		backend := newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
		if err := backend.addSRv6Steer(netip.MustParsePrefix("192.0.2.0/24"), sid, 0); err == nil {
			t.Fatalf("invalid SID %v accepted", sid)
		}
		if len(channel.requests) != 0 {
			t.Fatal("invalid SID reached VPP")
		}
	}
}

// TestSRv6ConsumerUnknownActionAndAmbiguousTable preserves enum no-op behavior
// and refuses a zero-table withdrawal when more than one table owns the prefix.
func TestSRv6ConsumerUnknownActionAndAmbiguousTable(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	fib := newFibVPP(&mockBackend{})
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	sid := netip.MustParseAddr("2001:db8:1::a")
	fib.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Add, Prefix: prefix, SRv6SID: sid, TableID: 10},
		{Action: routeaction.Add, Prefix: prefix, SRv6SID: sid, TableID: 20},
	}})
	requests := len(channel.requests)
	fib.processEvent(&incomingBatch{Changes: []incomingChange{
		{Action: routeaction.Unspecified, Prefix: prefix, SRv6SID: sid, TableID: 10},
		{Action: routeaction.Action(255), Prefix: prefix, SRv6SID: sid, TableID: 10},
		{Action: routeaction.Withdraw, Prefix: prefix},
	}})
	if len(channel.requests) != requests || len(channel.steers) != 2 {
		t.Fatal("no-op action or ambiguous withdrawal changed forwarding")
	}
}

// TestSRv6ForeignPrefixCannotBeClaimed protects steering installed by another
// writer even when Ze has no previous durable ownership for that prefix.
func TestSRv6ForeignPrefixCannotBeClaimed(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	foreign := toIP6Address(netip.MustParseAddr("fd02::1234"))
	channel.steers = append(channel.steers, sr.SrSteeringAddDel{
		Prefix: toVPPPrefix(prefix), BsidAddr: foreign, TrafficType: sr_types.SR_STEER_API_IPV4,
	})
	backend := newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	if err := backend.addSRv6Steer(prefix, netip.MustParseAddr("2001:db8:1::a"), 0); err == nil {
		t.Fatal("foreign steering was overwritten")
	}
	if len(channel.requests) != 0 || channel.steers[0].BsidAddr != foreign {
		t.Fatal("failed claim mutated foreign resources")
	}
}

// TestSRv6CollisionDoesNotGrantOwnership proves an add refusal cannot authorize
// deletion or adoption of an identical policy created by another writer.
func TestSRv6CollisionDoesNotGrantOwnership(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd), collide: true}
	store := newSRv6TestStore(t)
	backend := newGovppSRv6Backend(channel, 0, store)
	err := backend.addSRv6Steer(netip.MustParsePrefix("192.0.2.0/24"), netip.MustParseAddr("2001:db8:1::a"), 0)
	if err == nil {
		t.Fatal("exclusive policy collision was hidden")
	}
	keys, listErr := store.ListKeys(srv6StatePrefix)
	if listErr != nil {
		t.Fatal(listErr)
	}
	if len(channel.policies) != 1 || len(channel.steers) != 0 || len(keys) != 0 {
		t.Fatalf("collision claimed foreign state: policies=%d steers=%d keys=%v", len(channel.policies), len(channel.steers), keys)
	}
}

// TestSRv6IPTransitions prevents a previously hidden plain route from becoming
// visible on withdrawal, and preserves SR steering when the IP replacement fails.
func TestSRv6IPTransitions(t *testing.T) {
	channel := &srModelChannel{policies: make(map[ip_types.IP6Address]sr.SrPolicyAdd)}
	ip := &mockBackend{}
	fib := newFibVPP(ip)
	fib.srv6Backend = newGovppSRv6Backend(channel, 0, newSRv6TestStore(t))
	prefix := netip.MustParsePrefix("192.0.2.0/24")
	nextHop := netip.MustParseAddr("198.51.100.1")
	fib.processEvent(&incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Add, Prefix: prefix, NextHop: nextHop,
	}}})
	fib.processEvent(&incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Update, Prefix: prefix, NextHop: nextHop,
		SRv6SID: netip.MustParseAddr("2001:db8:1::a"),
	}}})
	if len(ip.richDels) != 1 || len(fib.installed) != 0 || len(channel.steers) != 1 {
		t.Fatal("IP-to-SRv6 replacement retained stale plain forwarding")
	}
	ip.err = errors.New("injected IP replacement failure")
	fib.processEvent(&incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Update, Prefix: prefix, NextHop: nextHop,
	}}})
	if len(channel.steers) != 1 || len(channel.policies) != 1 {
		t.Fatal("failed IP replacement removed SR forwarding")
	}
	ip.err = nil
	fib.processEvent(&incomingBatch{Changes: []incomingChange{{
		Action: routeaction.Update, Prefix: prefix, NextHop: nextHop,
	}}})
	if len(channel.steers) != 0 || len(channel.policies) != 0 || len(ip.richReplaces) != 1 {
		t.Fatal("SRv6-to-IP replacement failed to switch forwarding")
	}
}
