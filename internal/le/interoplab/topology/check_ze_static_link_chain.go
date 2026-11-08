// Design: docs/architecture/testing/interop.md -- the topology suite.
// Related: register_ze_static_link_chain.go -- the declaration this checks.
package topology

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// checkStaticLinkChain proves the core links and several Ze nodes: every Ze
// node answers `show interface brief` by its own name and lists each link
// device the declaration gives it; A pings C's address on the far link,
// which only B's forwarding can carry; and outside, on eth0 alone, received
// link a-b's frames only with the link's 802.1Q tag (linkABSeparate).
//
// A ping from outside to a link address proves nothing either way: Linux
// answers ARP for any address the host holds on any interface
// (arp_ignore 0), so ze-b answers 10.181.1.3 from its own eth0 MAC whether or
// not link a-b is separate. Every assertion reads state, never a delay.
func checkStaticLinkChain(ctx context.Context, check *interoplab.CheckContext) error {
	for _, name := range []string{nodeA, nodeB, nodeC} {
		if err := waitInterfaceBrief(ctx, check, name); err != nil {
			return err
		}
	}
	near, err := check.Member(nodeA, "a-b")
	if err != nil {
		return err
	}
	// The flush makes ze-a ARP again for ze-b, a broadcast the bridge floods
	// to outside's port, so outside's capture holds a tagged frame of link
	// a-b and its verdict is not vacuous.
	if _, err := check.Lab.Exec(ctx, nodeA, []string{"ip", "neigh", "flush", "dev", near.Device}, nil); err != nil {
		return fmt.Errorf("flush ze-a's neighbours on %s: %w", near.Device, err)
	}
	far, err := check.Member(nodeC, "b-c")
	if err != nil {
		return err
	}
	if err := waitPing(ctx, check, nodeA, far.IPv4.String()); err != nil {
		return fmt.Errorf("ze-a cannot reach ze-c's %s through ze-b: %w", far.IPv4, err)
	}
	return linkABSeparate(ctx, check)
}

// linkABSeparate waits until outside's capture holds a tagged frame of link
// a-b, which proves outside's port receives the link's broadcasts, then
// refuses any frame of the link's prefix that arrived without the tag: that
// frame left the VLAN onto eth0's own segment.
func linkABSeparate(ctx context.Context, check *interoplab.CheckContext) error {
	reading, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 20 * time.Second, Interval: time.Second,
		Description: "outside's capture holding a tagged frame of link a-b",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, "outside", []string{"tcpdump", "-e", "-n", "-r", outsideCapture}, nil)
	}, func(answer string) bool { return strings.Contains(answer, "802.1Q") })
	if err != nil {
		return fmt.Errorf("outside's capture never held a tagged frame of link a-b, so it cannot judge the link (last reading %q): %w", reading, err)
	}
	if leaked := untaggedFrames(reading); leaked != "" {
		return fmt.Errorf("outside, on eth0 alone, received link a-b's frames without the link's tag, so link a-b is not separate from the lab network:\n%s", leaked)
	}
	return nil
}

// waitInterfaceBrief waits until the Ze node name answers `show interface
// brief` with every link device the declaration gives it.
func waitInterfaceBrief(ctx context.Context, check *interoplab.CheckContext, name string) error {
	devices := make([]string, 0, len(check.Memberships[name]))
	for _, membership := range check.Memberships[name] {
		devices = append(devices, membership.Device)
	}
	if len(devices) == 0 {
		return fmt.Errorf("%s joins no link, so its interface table proves nothing about the links", name)
	}
	answer, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second,
		Description: name + " show interface brief listing its link devices",
	}, func(probe context.Context) (string, error) {
		return check.Lab.Query(probe, name, interoplab.ZeCLICommand("show interface brief"), nil)
	}, func(answer string) bool {
		return answerListsDevices(answer, devices)
	})
	if err != nil {
		return fmt.Errorf("%s never listed %v in show interface brief (last answer %q): %w", name, devices, answer, err)
	}
	return nil
}

// answerListsDevices reports whether answer names every device as a word.
func answerListsDevices(answer string, devices []string) bool {
	words := strings.Fields(answer)
	for _, device := range devices {
		if !slices.Contains(words, device) {
			return false
		}
	}
	return true
}

// waitPing waits until peer's ping of address succeeds.
func waitPing(ctx context.Context, check *interoplab.CheckContext, peer, address string) error {
	_, _, err := interoplab.Wait(ctx, interoplab.WaitOptions{
		Timeout: 60 * time.Second, Interval: time.Second,
		Description: peer + " ping " + address,
	}, func(probe context.Context) (interoplab.CommandResult, error) {
		return check.Lab.Exec(probe, peer, []string{"ping", "-c", "2", "-W", "2", address}, nil)
	}, func(result interoplab.CommandResult) bool {
		return result.ExitCode == 0
	})
	return err
}
