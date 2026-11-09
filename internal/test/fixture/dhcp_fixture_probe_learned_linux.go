// Design: docs/architecture/provisioning/dhcp-server.md -- Functional tests: the probe and its WAN lease
// Related: register_dhcp_probe_learned_linux.go -- the name test/dhcp/*.ci use
// Related: dhcp_fixture_probe_linux.go -- dhcpProbeDriver, the probe this extends with a WAN lease
// Related: ../dhcpprobe/step.go -- the exchange steps it hands on

//go:build linux

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/test/dhcpprobe"
)

// The bounds of the waits this driver performs. untilAttempts times
// untilDelay, plus the second each attempt can wait for its reply, stays
// inside the timeout the .ci files using it set (120s).
const (
	learnedLeaseAttempts = 160
	learnedLeaseDelay    = 250 * time.Millisecond
	untilAttempts        = 40
	untilDelay           = 500 * time.Millisecond
)

// dhcpProbeLearnedDriver is dhcp/probe for a server whose option 6 follows a
// learned resolver: Ze's own DHCP client leases on a WAN veth pair the
// daemon creates, from Ze's own DHCP server on the pair's other end, and the
// steps then watch the LAN replies follow that lease. Arguments: the LAN
// link dhcp/link made, then the steps. It adds three verbs to the probe's:
//
//	wan-up,<server-end>,<cidr>,<client-end>
//	    gives the daemon's server end its subnet address, brings it up, and
//	    waits until the DHCP client on client-end holds an address in it.
//	reload,<file>
//	    copies file over ze-bgp.conf and sends SIGHUP.
//	until,x,<label>,...
//	    sends the exchange, once a step, until the reply carries every field
//	    it names: the state it waits for follows an event (a lease, a
//	    reload) the daemon applies on its own schedule.
//
// Every other step is the probe's own. It then stops the daemon and removes
// the LAN link, whatever the steps answered.
func dhcpProbeLearnedDriver(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return errors.New("dhcp/probe-learned takes the LAN link name, then the steps: dhcp/probe-learned <name> <step>...")
	}
	steps := make([]learnedStep, 0, len(args)-1)
	for _, arg := range args[1:] {
		step, err := parseLearnedStep(arg)
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
	runErr := runLearnedSteps(ctx, probe, pid, steps)
	closeErr := probe.Close()
	stopErr := signalProcess(pid, syscall.SIGTERM)
	waitDead(ctx, pid)
	linkErr := dhcpprobe.DeleteLink(args[0])
	return errors.Join(runErr, closeErr, stopErr, linkErr)
}

// learnedVerb is what one dhcp/probe-learned argument asks for.
type learnedVerb uint8

const (
	learnedUnspecified learnedVerb = iota
	learnedWANUp
	learnedReload
	learnedUntil
	learnedProbe
)

// learnedStep is one parsed argument. Only the fields of its verb are set.
type learnedStep struct {
	verb learnedVerb
	// wan-up
	serverEnd, clientEnd string
	serverAddress        netip.Prefix
	// reload
	file string
	// until and probe
	probe dhcpprobe.Step
}

func parseLearnedStep(arg string) (learnedStep, error) {
	verb, rest, _ := strings.Cut(arg, ",")
	switch verb {
	case "wan-up":
		fields := strings.Split(rest, ",")
		if len(fields) != 3 {
			return learnedStep{}, errors.New("dhcp/probe-learned: wan-up takes wan-up,<server-end>,<cidr>,<client-end>")
		}
		prefix, err := netip.ParsePrefix(fields[1])
		if err != nil {
			return learnedStep{}, fmt.Errorf("dhcp/probe-learned: wan-up: %w", err)
		}
		return learnedStep{verb: learnedWANUp, serverEnd: fields[0], serverAddress: prefix, clientEnd: fields[2]}, nil
	case "reload":
		if rest == "" {
			// The probe's own reload: config2.conf.
			break
		}
		return learnedStep{verb: learnedReload, file: rest}, nil
	case "until":
		step, err := dhcpprobe.ParseStep(rest)
		if err != nil {
			return learnedStep{}, err
		}
		if step.Kind != dhcpprobe.StepExchange {
			return learnedStep{}, errors.New("dhcp/probe-learned: until takes an exchange: until,x,<label>,...")
		}
		if step.Exchange.Silent {
			return learnedStep{}, errors.New("dhcp/probe-learned: until waits for a reply; an expected silence is the probe's own x step")
		}
		// One transmission per attempt: the poll is the retransmission.
		step.Exchange.Attempts = 1
		return learnedStep{verb: learnedUntil, probe: step}, nil
	}
	step, err := dhcpprobe.ParseStep(arg)
	if err != nil {
		return learnedStep{}, err
	}
	return learnedStep{verb: learnedProbe, probe: step}, nil
}

// runLearnedSteps performs steps in order against the daemon pid and stops
// at the first that fails.
func runLearnedSteps(ctx context.Context, probe *dhcpprobe.Probe, pid int, steps []learnedStep) error {
	reload := func(file string) error {
		data, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("dhcp/probe-learned reload: %w", err)
		}
		if err := os.WriteFile(fileBGPConf, data, 0o600); err != nil {
			return fmt.Errorf("dhcp/probe-learned reload: %w", err)
		}
		return signalProcess(pid, syscall.SIGHUP)
	}
	probeReload := func() error { return reload(fileConfig2Conf) }
	emit := func(line string) { fmt.Println(line) }
	for i := range steps {
		step := &steps[i]
		var err error
		switch step.verb {
		case learnedWANUp:
			err = learnedWANLease(ctx, step)
		case learnedReload:
			err = reload(step.file)
		case learnedUntil:
			err = learnedUntilReply(ctx, probe, step.probe)
		case learnedProbe:
			err = probe.Run([]dhcpprobe.Step{step.probe}, probeReload, emit)
		case learnedUnspecified:
			err = errors.New("BUG: dhcp/probe-learned: a step with no verb reached the run")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// learnedWANLease addresses the daemon's DHCP server end of the WAN pair,
// brings it up, and waits until the DHCP client on the other end holds an
// address inside that subnet: the lease whose option 6 the LAN replies are
// to follow.
func learnedWANLease(ctx context.Context, step *learnedStep) error {
	var link netlink.Link
	if !Poll(ctx, learnedLeaseAttempts, learnedLeaseDelay, func() bool {
		found, err := netlink.LinkByName(step.serverEnd)
		if err != nil {
			return false
		}
		link = found
		return true
	}) {
		return fmt.Errorf("dhcp/probe-learned: the daemon never created %s", step.serverEnd)
	}
	address, err := netlink.ParseAddr(step.serverAddress.String())
	if err != nil {
		return fmt.Errorf("dhcp/probe-learned: parse %s: %w", step.serverAddress, err)
	}
	if err := netlink.AddrReplace(link, address); err != nil {
		return fmt.Errorf("dhcp/probe-learned: add %s to %s: %w", step.serverAddress, step.serverEnd, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("dhcp/probe-learned: bring %s up: %w", step.serverEnd, err)
	}
	subnet := step.serverAddress.Masked()
	var leased netip.Addr
	if !Poll(ctx, learnedLeaseAttempts, learnedLeaseDelay, func() bool {
		client, err := netlink.LinkByName(step.clientEnd)
		if err != nil {
			return false
		}
		addrs, err := netlink.AddrList(client, unix.AF_INET)
		if err != nil {
			return false
		}
		for i := range addrs {
			addr, ok := netip.AddrFromSlice(addrs[i].IP.To4())
			if ok && subnet.Contains(addr) {
				leased = addr
				return true
			}
		}
		return false
	}) {
		return fmt.Errorf("dhcp/probe-learned: the DHCP client on %s never leased an address in %s", step.clientEnd, subnet)
	}
	fmt.Println("dhcp wan " + step.clientEnd + " leased " + leased.String())
	return nil
}

// learnedUntilReply sends the exchange until its reply carries every field
// it names, then prints that reply's line. When the poll runs out it prints
// the last reply's line and answers the exchange's own verdict on it, so the
// failure names the field that never arrived.
func learnedUntilReply(ctx context.Context, probe *dhcpprobe.Probe, step dhcpprobe.Step) error {
	var lastLine string
	var lastErr error
	noReload := func() error { return errors.New("BUG: dhcp/probe-learned: an exchange asked for a reload") }
	if Poll(ctx, untilAttempts, untilDelay, func() bool {
		lastErr = probe.Run([]dhcpprobe.Step{step}, noReload, func(line string) { lastLine = line })
		return lastErr == nil
	}) {
		fmt.Println(lastLine)
		return nil
	}
	if lastLine != "" {
		fmt.Println(lastLine)
	}
	if lastErr == nil {
		return fmt.Errorf("dhcp/probe-learned: exchange %s was never sent before the test's context ended", step.Exchange.Label)
	}
	return fmt.Errorf("dhcp/probe-learned: after %d attempts: %w", untilAttempts, lastErr)
}
