// Design: plan/learned/819-flow-export-2-flow-records.md -- tc sample action setup/teardown

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
		QdiscAttrs: netlink.QdiscAttrs{
			LinkIndex: linkIndex,
			Handle:    netlink.MakeHandle(0xffff, 0),
			Parent:    netlink.HANDLE_CLSACT,
		},
	}
	if err := netlink.QdiscAdd(qdisc); err != nil {
		if !errors.Is(err, unix.EEXIST) {
			return fmt.Errorf("sampling: add clsact qdisc on %q: %w", ifaceName, err)
		}
	}

	action := netlink.NewSampleAction()
	action.Rate = rate
	action.Group = group
	action.TruncSize = truncSize

	filter := &netlink.MatchAll{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: linkIndex,
			Parent:    netlink.HANDLE_MIN_INGRESS,
			Priority:  SampleFilterPriority,
			Protocol:  unix.ETH_P_ALL,
		},
		Actions: []netlink.Action{action},
	}
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

// RemoveSampling removes the sample filter (priority 100) from the named
// interface. The clsact qdisc is left in place because mirror filters
// at priority 1 may still be active.
func RemoveSampling(ifaceName string) error {
	linkIndex, err := resolveIfaceIndex(ifaceName)
	if err != nil {
		return err
	}

	filter := &netlink.MatchAll{
		FilterAttrs: netlink.FilterAttrs{
			LinkIndex: linkIndex,
			Parent:    netlink.HANDLE_MIN_INGRESS,
			Priority:  SampleFilterPriority,
			Protocol:  unix.ETH_P_ALL,
		},
	}
	if err := netlink.FilterDel(filter); err != nil {
		if !errors.Is(err, unix.ENOENT) {
			return fmt.Errorf("sampling: remove sample filter on %q: %w", ifaceName, err)
		}
	}

	return nil
}
