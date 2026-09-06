// Design: docs/architecture/core-design.md -- tc ingress policer translation
// Related: translate_linux.go -- root-qdisc translation, which is the egress half
// Related: backend_linux.go -- the Apply and RestoreOriginal call sites

//go:build linux

package trafficnetlink

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/traffic"
)

// The ingress hook at handle ffff: is a SHARED attachment point, and the
// policer is its third owner. The mirror installs mirred filters at priority 1
// (internal/plugins/iface/netlink/mirror_linux.go), flow-export sampling
// installs its sample filter at priority 100
// (internal/plugins/flowexport/sampling, SampleFilterPriority), and this
// backend installs the subscriber upload policer at 200.
//
// 200 is LAST on purpose. The kernel walks the filter chain in priority order
// and stops at the first filter that returns a verdict, so the policer running
// last is the only placement under which neither existing owner's behavior
// changes: a mirror or a sample still sees the packet, and only then does the
// policer decide whether it is within the subscriber's rate.
//
// The qdisc itself is added, never replaced, and teardown removes this
// priority rather than the qdisc: replacing or deleting the qdisc object drops
// every filter of every subsystem on both hooks.
const (
	// policerFilterPriority is the tc filter priority the ingress policer owns.
	policerFilterPriority uint16 = 200
	// policerQdiscHandleMajor is the major number of the ingress-side qdisc
	// handle, ffff:0. The mirror and sampling paths spell the same handle.
	policerQdiscHandleMajor uint16 = 0xffff
)

// policerMTUBytes bounds the largest packet the token bucket can admit: the
// kernel treats anything above it as exceeding, whatever the bucket holds. The
// ingress hook runs AFTER generic receive offload, so a packet arriving here
// can be a coalesced segment far larger than the link MTU. 64 KiB is the
// largest such segment, so this admits every real packet and lets the rate
// alone decide.
const policerMTUBytes uint32 = 64 * 1024

// policerRateBpsMax is the largest rate the kernel can carry. TCA_POLICE_TBF
// holds the rate as a uint32 of BYTES per second, so the bound in bits is that
// value times eight, about 34.359 Gbit/s. A rate above it is refused rather
// than truncated: a truncated rate polices at a wrapped value, which is
// silently wrong in the direction that lets traffic through.
const policerRateBpsMax uint64 = uint64(^uint32(0)) * 8

// errPolicerBurstOverflow names the second uint32 bound. A burst is 100ms of
// the rate, so it can only overflow for a rate the bound above already
// refuses; the check stays because the burst arrives from the caller rather
// than from the rate.
var errPolicerBurstOverflow = errors.New("trafficnetlink: policer burst does not fit the kernel's uint32 byte count")

// ingressClsactQdisc builds the shared clsact qdisc at ffff:0. clsact carries
// both the ingress and the egress hook in one object, which is why the mirror
// path chose it and why this backend must not replace or delete it.
func ingressClsactQdisc(linkIndex int) *netlink.Clsact {
	return &netlink.Clsact{
		LinkIndex: linkIndex,
		Handle:    netlink.MakeHandle(policerQdiscHandleMajor, 0),
		Parent:    netlink.HANDLE_CLSACT,
	}
}

// ingressPolicerFilter translates a ze Policer into the matchall filter that
// carries it. matchall selects every packet, which is what a per-interface rate
// limit means: the interface belongs to one subscriber, so there is nothing on
// it to tell apart.
//
// Exceeding traffic is DROPPED. The ingress hook holds no queue, so delaying a
// packet is not among the options; the choice is to admit it or to drop it.
func ingressPolicerFilter(p traffic.Policer, linkIndex int) (*netlink.MatchAll, error) {
	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("trafficnetlink: ingress policer: %w", err)
	}
	if p.RateBps > policerRateBpsMax {
		return nil, fmt.Errorf("trafficnetlink: ingress policer rate %d bps exceeds the kernel's maximum of %d bps", p.RateBps, policerRateBpsMax)
	}
	if p.BurstBytes > uint64(^uint32(0)) {
		return nil, fmt.Errorf("%w: %d", errPolicerBurstOverflow, p.BurstBytes)
	}

	action := netlink.NewPoliceAction()
	action.Rate = uint32(p.RateBps / 8)
	action.Burst = uint32(p.BurstBytes)
	action.Mtu = policerMTUBytes
	action.ExceedAction = netlink.TC_POLICE_SHOT
	action.NotExceedAction = netlink.TC_POLICE_OK

	return &netlink.MatchAll{
		LinkIndex: linkIndex,
		Parent:    netlink.HANDLE_MIN_INGRESS,
		Priority:  policerFilterPriority,
		Protocol:  unix.ETH_P_ALL,
		Actions:   []netlink.Action{action},
	}, nil
}

// applyIngressPolicer programs the interface's upload rate limit. An interface
// that asks for none leaves the shared hook untouched, because another
// subsystem's filters live there.
func (b *backend) applyIngressPolicer(link netlink.Link, p traffic.Policer) error {
	if !p.Set() {
		return nil
	}
	linkIndex := link.Attrs().Index
	filter, err := ingressPolicerFilter(p, linkIndex)
	if err != nil {
		return err
	}
	// EEXIST means another subsystem created the hook first, and the hook is
	// what this call needs. Adding is the only safe verb: qdiscReplace would
	// drop the mirror and sampling filters attached to the existing object.
	if err := b.ops.qdiscAdd(ingressClsactQdisc(linkIndex)); err != nil && !errors.Is(err, unix.EEXIST) {
		return fmt.Errorf("add clsact qdisc: %w", err)
	}
	// Replace rather than fail on a re-apply: a mid-session rate change applies
	// the same interface again with a new rate, and the old filter must go.
	if err := b.ops.filterAdd(filter); err != nil {
		if !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("add ingress policer filter: %w", err)
		}
		if delErr := b.ops.filterDel(filter); delErr != nil && !isPolicerAbsent(delErr) {
			return fmt.Errorf("replace ingress policer filter: %w", delErr)
		}
		if err := b.ops.filterAdd(filter); err != nil {
			return fmt.Errorf("replace ingress policer filter: %w", err)
		}
	}
	return nil
}

// removeIngressPolicer clears this backend's priority on the ingress hook and
// leaves the qdisc. A teardown cannot know who else attached to it, and the set
// of attached filters can change between reading it and acting on it.
//
// The delete is unconditional rather than gated on remembered state: the
// desired end state is "no policer at this priority", and asking the kernel for
// it is correct whether or not this process installed one.
func (b *backend) removeIngressPolicer(link netlink.Link) error {
	filter := &netlink.MatchAll{
		LinkIndex: link.Attrs().Index,
		Parent:    netlink.HANDLE_MIN_INGRESS,
		Priority:  policerFilterPriority,
		Protocol:  unix.ETH_P_ALL,
	}
	if err := b.ops.filterDel(filter); err != nil && !isPolicerAbsent(err) {
		return fmt.Errorf("remove ingress policer filter: %w", err)
	}
	return nil
}

// isPolicerAbsent reports whether an error means "there was nothing to
// delete", so the desired state is already reached. The kernel answers a
// FilterDel that matches nothing with one of exactly two errnos: EINVAL when
// the link carries no qdisc at handle ffff: at all, and ENOENT when the qdisc
// is there but the hook holds no filter at that priority. Both are tolerated
// and nothing else is, because this is the only error gate on the teardown
// path and a wider one lets a real failure report success.
func isPolicerAbsent(err error) bool {
	return errors.Is(err, unix.ENOENT) || errors.Is(err, unix.EINVAL)
}
