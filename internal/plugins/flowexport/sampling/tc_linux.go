// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- tc sample action setup/teardown

//go:build linux

package sampling

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/iface"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// resolveIfaceIndex maps a logical interface name to its kernel ifindex via the
// shared iface resolver, honoring the os-name / mac-match selectors so sampling
// attaches to the right device even when the logical name differs from it.
func resolveIfaceIndex(ifaceName string) (int, error) {
	b, err := iface.Resolve(ifaceName)
	if err != nil {
		return 0, fmt.Errorf("sampling: interface %q not found: %w", ifaceName, err)
	}
	return b.Ifindex, nil
}

// SetupSampling installs a tc sample action on the named interface.
// It creates or reuses the clsact qdisc and adds a MatchAll filter
// at priority 100 with SampleAction. Mirror filters at priority 1
// are not affected.
func SetupSampling(ifaceName string, rate, group, truncSize uint32) error {
	linkIndex, err := resolveIfaceIndex(ifaceName)
	if err != nil {
		return err
	}

	qdisc := &netlink.Clsact{
		LinkIndex: linkIndex,
		Handle:    netlink.MakeHandle(0xffff, 0),
		Parent:    netlink.HANDLE_CLSACT,
	}
	if err := netlink.QdiscAdd(qdisc); err != nil {
		if !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("sampling: add clsact qdisc on %q: %w", ifaceName, err)
		}
	}

	filter := buildSampleFilter(linkIndex, rate, group, truncSize)
	if err := netlink.FilterAdd(filter); err != nil {
		if errors.Is(err, unix.EEXIST) {
			_ = netlink.FilterDel(filter)
			if err := netlink.FilterAdd(filter); err != nil {
				return fmt.Errorf("sampling: replace sample filter on %q: %w", ifaceName, err)
			}
		} else {
			return fmt.Errorf("sampling: add sample filter on %q: %w", ifaceName, err)
		}
	}

	return nil
}

// buildSampleFilter is the tc filter Ze installs for one sampler instance:
// what the kernel's act_sample then reads for every packet on the link.
//
// sFlow v5, "Packet Flow Sampling": sampling "must ensure that any packet
// observed at a Data Source has an equal chance of being sampled,
// irrespective of the Packet Flow(s) to which it belongs", so the filter is
// MatchAll on ETH_P_ALL and no packet is classified before the draw.
// "Each packet must only be considered once for sampling, irrespective of
// the number of ports it will be forwarded to", so the filter sits on the
// ingress hook only, where a packet passes once. "Each sFlow sampler
// instance must operate independently of all other instances", so the
// instance's own rate, group and truncation ride in its own action and no
// state is shared between links. "The sampling algorithm must converge so
// that over time the number of packets sampled approaches 1/Nth of the total
// number of packets", and act_sample's per-packet random draw at the rate
// written here is that algorithm.
func buildSampleFilter(linkIndex int, rate, group, truncSize uint32) *netlink.MatchAll {
	action := netlink.NewSampleAction()
	action.Rate = rate
	action.Group = group
	action.TruncSize = truncSize

	return &netlink.MatchAll{
		LinkIndex: linkIndex,
		Parent:    netlink.HANDLE_MIN_INGRESS,
		Priority:  SampleFilterPriority,
		Protocol:  unix.ETH_P_ALL,
		Actions:   []netlink.Action{action},
	}
}

// RemoveSampling removes the sample filter (priority 100) from the named
// interface. The clsact qdisc is left in place because mirror filters
// at priority 1 may still be active.
func RemoveSampling(ifaceName string) error {
	linkIndex, err := resolveIfaceIndex(ifaceName)
	if err != nil {
		return err
	}

	filter := &netlink.MatchAll{
		LinkIndex: linkIndex,
		Parent:    netlink.HANDLE_MIN_INGRESS,
		Priority:  SampleFilterPriority,
		Protocol:  unix.ETH_P_ALL,
	}
	if err := netlink.FilterDel(filter); err != nil {
		if !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("sampling: remove sample filter on %q: %w", ifaceName, err)
		}
	}

	return nil
}
