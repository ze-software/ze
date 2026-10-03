// Design: docs/architecture/fib/fib-depth-4-srv6.md -- VPP SRv6 SR steer programming
// Related: srv6.go -- addSRv6Steer, the producer this test drives

package fibvpp

import (
	"errors"
	"net/netip"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/ip_types"
	"go.fd.io/govpp/binapi/sr"
)

// srModelChannel is a fake api.Channel that answers the two SR messages the
// way VPP does. src/vnet/srv6/sr_steering.c sr_steering_policy() looks the
// BSID up in sr_policies_index_hash and returns -2 when no SR policy carries
// it ("The requested SR policy could not be located. Review the BSID/index."),
// so a steering entry can only name a policy an earlier sr_policy_add created.
// Not safe for concurrent use.
type srModelChannel struct {
	policies map[ip_types.IP6Address]sr.SrPolicyAdd
	steers   []sr.SrSteeringAddDel
}

var _ api.Channel = (*srModelChannel)(nil)

type srModelRequest struct {
	ch  *srModelChannel
	msg api.Message
}

func (r *srModelRequest) ReceiveReply(reply api.Message) error {
	switch request := r.msg.(type) {
	case *sr.SrPolicyAdd:
		r.ch.policies[request.BsidAddr] = *request
		if out, ok := reply.(*sr.SrPolicyAddReply); ok {
			out.Retval = 0
		}
	case *sr.SrSteeringAddDel:
		out, ok := reply.(*sr.SrSteeringAddDelReply)
		if !ok {
			return nil
		}
		if _, known := r.ch.policies[request.BsidAddr]; !known {
			out.Retval = -2
			return nil
		}
		r.ch.steers = append(r.ch.steers, *request)
		out.Retval = 0
	}
	return nil
}

func (c *srModelChannel) SendRequest(msg api.Message) api.RequestCtx {
	return &srModelRequest{ch: c, msg: msg}
}

func (c *srModelChannel) SendMultiRequest(api.Message) api.MultiRequestCtx { return nil }

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
// Untagged and red on purpose: the fix is design-sized (a local BSID per SID,
// policy lifetime shared by every prefix that resolves to the SID).
func TestRFC9252VPPServiceRouteEncapsulatesTowardTheSID(t *testing.T) {
	channel := &srModelChannel{policies: map[ip_types.IP6Address]sr.SrPolicyAdd{}}
	backend := newGovppSRv6Backend(channel, 0)
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
	if !policy.IsEncap {
		t.Errorf("SR policy IsEncap = false, want the encapsulation behavior")
	}
	if policy.Sids.NumSids != 1 || policy.Sids.Sids[0] != toIP6Address(sid) {
		t.Errorf("SR policy segment list = %d %v, want exactly [%v]", policy.Sids.NumSids, policy.Sids.Sids[0], sid)
	}
}
