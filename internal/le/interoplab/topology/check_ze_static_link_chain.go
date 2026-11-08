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
// which only B's forwarding can carry; and outside, on eth0 alone, cannot
// reach a link address. outside has no default route and routes link a-b's
// prefix onto eth0 (outsidePrestart), so its ping ARPs on the lab network:
// a reply proves a-b leaks onto eth0's segment, and the Docker host's own
// routing cannot answer it. Every assertion reads state, never a delay.
func checkStaticLinkChain(ctx context.Context, check *interoplab.CheckContext) error {
	for _, name := range []string{nodeA, nodeB, nodeC} {
		if err := waitInterfaceBrief(ctx, check, name); err != nil {
			return err
		}
	}
	far, err := check.Member(nodeC, "b-c")
	if err != nil {
		return err
	}
	if err := waitPing(ctx, check, nodeA, far.IPv4.String()); err != nil {
		return fmt.Errorf("ze-a cannot reach ze-c's %s through ze-b: %w", far.IPv4, err)
	}
	near, err := check.Member(nodeB, "a-b")
	if err != nil {
		return err
	}
	answer, err := check.Lab.Exec(ctx, "outside", []string{"ping", "-c", "2", "-W", "2", near.IPv4.String()}, nil)
	if err != nil {
		return fmt.Errorf("outside's ping of %s did not run: %w", near.IPv4, err)
	}
	if answer.ExitCode == 0 {
		return fmt.Errorf("outside, on eth0 alone, reached ze-b's link address %s, so link a-b is not separate from the lab network", near.IPv4)
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
