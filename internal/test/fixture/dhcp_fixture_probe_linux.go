// Design: docs/architecture/provisioning/dhcp-server.md -- the DHCP server these fixtures exchange with
// Related: register_dhcp_probe_linux.go -- the names test/dhcp/*.ci use
// Related: ../dhcpprobe/probe_linux.go -- the link, the frames and the steps

//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"syscall"

	"github.com/ze-software/ze/internal/test/dhcpprobe"
)

// dhcpLinkDriver creates the veth pair a DHCP .ci serves on, before the
// daemon starts: the server binds its listen interface at boot, and a device
// that appears later is served only after a reload. Arguments: the server
// end's name, then its addresses, the primary first. The probe end is the
// name with a "p" suffix.
func dhcpLinkDriver(_ context.Context, args []string) error {
	if len(args) < 1 {
		return errors.New("dhcp/link takes the link name, then its addresses: dhcp/link <name> [<cidr>...]")
	}
	addrs := make([]netip.Prefix, 0, len(args)-1)
	for _, arg := range args[1:] {
		prefix, err := netip.ParsePrefix(arg)
		if err != nil {
			return fmt.Errorf("dhcp/link: %w", err)
		}
		addrs = append(addrs, prefix)
	}
	if err := dhcpprobe.CreateLink(args[0], addrs); err != nil {
		return err
	}
	fmt.Println("dhcp link " + args[0] + " ready")
	return nil
}

// dhcpProbeDriver waits for the daemon, performs the steps against its
// listener on the named link and prints one line per exchange. A reply that
// differs from what a step names is an error, so the verdict does not rest
// on the .ci expect lines alone. It then stops the daemon and removes the
// link, whatever the steps answered.
func dhcpProbeDriver(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("dhcp/probe takes the link name, then the steps: dhcp/probe <name> <step> [<step> ...]")
	}
	steps := make([]dhcpprobe.Step, 0, len(args)-1)
	for _, arg := range args[1:] {
		step, err := dhcpprobe.ParseStep(arg)
		if err != nil {
			return err
		}
		steps = append(steps, step)
	}
	pid, err := waitDaemon(ctx, WaitAttempts(40, netfilterPollDelay, 200))
	if err != nil {
		return err
	}
	probe, err := dhcpprobe.Open(args[0])
	if err != nil {
		return errors.Join(err, signalProcess(pid, syscall.SIGTERM))
	}
	reload := func() error {
		data, err := os.ReadFile(fileConfig2Conf)
		if err != nil {
			return fmt.Errorf("dhcp/probe reload: %w", err)
		}
		if err := os.WriteFile(fileBGPConf, data, 0o600); err != nil {
			return fmt.Errorf("dhcp/probe reload: %w", err)
		}
		return signalProcess(pid, syscall.SIGHUP)
	}
	runErr := probe.Run(steps, reload, func(line string) { fmt.Println(line) })
	closeErr := probe.Close()
	stopErr := signalProcess(pid, syscall.SIGTERM)
	waitDead(ctx, pid)
	linkErr := dhcpprobe.DeleteLink(args[0])
	return errors.Join(runErr, closeErr, stopErr, linkErr)
}
