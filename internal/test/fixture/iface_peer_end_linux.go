//go:build linux

// Design: docs/functional-tests.md -- reading a link's peer end inside the namespace a daemon answer names
// Related: register_iface_peer_end_linux.go -- the registration of this fixture
// Related: daemon_await_answer_fixture.go -- the barrier a .ci runs before it

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// ifacePeerEnd prints the state of one link inside the network namespace of a
// process whose id a daemon command answers, so a .ci asserts on the peer end
// of an owned link, which the host namespace cannot show.
//
// Arguments: <pid-key> <command> <link>. It runs `ze cli -c <command>` once,
// with the client environment the runner gave this process, and reads the
// first run of digits after <pid-key> in the answer, which matches the JSON
// and the table renderings alike. Run it after a barrier that saw the answer
// (daemon/await-answer). It then reads <link> through a netlink socket opened
// in that process's namespace and prints one line per fact: `name`, `state`,
// `mtu`, `mac`, one `address` per address and one `default via` per default
// route. A missing key, a missing process or a missing link is an error, never
// an empty answer.
func ifacePeerEnd(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: iface/peer-end <pid-key> <command> <link>")
	}
	key, command, linkName := args[0], args[1], args[2]
	code, stdout, stderr, err := runCaptured(ctx, os.Environ(), "", "ze", "cli", "-c", command)
	if err != nil {
		return fmt.Errorf("iface/peer-end: run `ze cli -c %s`: %w", command, err)
	}
	if code != 0 {
		return fmt.Errorf("iface/peer-end: `ze cli -c %s` exited %d: %s", command, code, strings.TrimSpace(stderr))
	}
	match := regexp.MustCompile(regexp.QuoteMeta(key) + `\W+(\d+)`).FindStringSubmatch(stdout)
	if match == nil {
		return fmt.Errorf("iface/peer-end: the answer of `%s` carries no %s: %s", command, key, strings.TrimSpace(stdout))
	}
	pid, err := strconv.Atoi(match[1])
	if err != nil {
		return fmt.Errorf("iface/peer-end: %s %q is not a process id: %w", key, match[1], err)
	}
	namespace, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("iface/peer-end: open the network namespace of process %d: %w", pid, err)
	}
	defer namespace.Close() //nolint:errcheck // a read-only handle
	handle, err := netlink.NewHandleAt(namespace, unix.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("iface/peer-end: open a netlink socket in the namespace of process %d: %w", pid, err)
	}
	defer handle.Close()
	return printPeerEnd(handle, pid, linkName)
}

// printPeerEnd prints the facts of linkName as handle's namespace holds them.
func printPeerEnd(handle *netlink.Handle, pid int, linkName string) error {
	link, err := handle.LinkByName(linkName)
	if err != nil {
		return fmt.Errorf("iface/peer-end: no %s in the namespace of process %d: %w", linkName, pid, err)
	}
	attrs := link.Attrs()
	state := linkStateDown
	if attrs.Flags&net.FlagUp != 0 {
		state = "up"
	}
	lines := []string{"name " + attrs.Name, "state " + state, "mtu " + strconv.Itoa(attrs.MTU), "mac " + attrs.HardwareAddr.String()}
	addresses, err := handle.AddrList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("iface/peer-end: list the addresses of %s: %w", linkName, err)
	}
	for _, address := range addresses {
		lines = append(lines, "address "+address.IPNet.String())
	}
	routes, err := handle.RouteList(link, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("iface/peer-end: list the routes of %s: %w", linkName, err)
	}
	for i := range routes {
		route := &routes[i]
		if route.Gw == nil {
			continue
		}
		if route.Dst != nil && !route.Dst.IP.IsUnspecified() {
			continue
		}
		lines = append(lines, "default via "+route.Gw.String())
	}
	_, err = os.Stdout.WriteString(strings.Join(lines, "\n") + "\n")
	return err
}
