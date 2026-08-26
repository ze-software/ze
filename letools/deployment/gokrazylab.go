// Design: docs/architecture/testing/qemu-integration.md -- the lab an appliance proof runs in
// Related: netns.go -- the namespace primitives this lab is built out of
// Related: gokrazykernel.go -- the kernel the appliance in this lab boots
// Related: gokrazyimage.go -- the image this lab carries
// Related: gokrazyl2tp.go -- the proof that builds this lab
// Related: gokrazyl2tpreport.go -- the payload carrying this lab's names
//
// gokrazylab.go builds the network that an appliance proof needs. It creates a
// bridge in the root namespace and a TAP on that bridge for the virtual
// machine's NIC. A veth pair connects the bridge to a namespace that holds the
// peer. A DHCP server gives the appliance a known address.
//
// Two facts force this shape. QEMU user-mode networking does NOT deliver
// inbound UDP to the guest, but an L2TP tunnel starts with an inbound SCCRQ.
// Thus, the appliance needs a real layer-2 segment instead of a port forward.
// The appliance uses DHCP, so a server on that segment must answer it. The
// server reserves one address because the peer needs that address before the
// appliance boots.

package deployment

import (
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// dnsmasqLease is the duration of the appliance's address lease. It is short
// because the appliance holds the address for one proof. If the lease outlives
// the run, it keeps the file alive for the next one to trip over.
const dnsmasqLease = "2m"

// dhcpNetmask is the mask supplied with the reservation. It matches the /24
// that the underlay uses by default. An operator who narrows the prefix narrows
// the addresses rather than the mask. The reservation needs that mask.
const dhcpNetmask = "255.255.255.0"

// appliancePoll is how often the lab re-asks whether the appliance has answered
// on its address. A DHCP exchange plus an interface coming up takes hundreds of
// milliseconds, so a shorter poll would only spin.
const appliancePoll = 500 * time.Millisecond

// gokrazyLab is one run's network: the names it made, the addresses it uses,
// and where it writes what it must clean up.
type gokrazyLab struct {
	// Namespace holds the peer. The bridge and the TAP stay in the root
	// namespace, where QEMU can reach them.
	Namespace string
	Bridge    string
	Tap       string
	HostVeth  string
	PeerVeth  string

	// The underlay. HostIP is the bridge's own address, PeerIP the peer's, and
	// ApplianceIP the reservation the appliance is given.
	HostIP       string
	PeerIP       string
	ApplianceIP  string
	ApplianceMAC string
	Prefix       string

	// Where dnsmasq keeps what it must be stopped and cleaned up by.
	PidFile   string
	LeaseFile string
	LogFile   string

	Progress io.Writer
}

// newGokrazyLab answers the lab one run owns, with every name carrying the
// process id so two runs on one machine do not collide.
func newGokrazyLab(suffix string) *gokrazyLab {
	short := linkSuffix(suffix)
	var tb textbuf.Buffer
	return &gokrazyLab{
		Namespace: tb.Str("ze-gokrazy-lac-").Str(suffix).String(),
		Bridge:    tb.Reset().Str("zebr").Str(short).String(),
		Tap:       tb.Reset().Str("zetap").Str(short).String(),
		HostVeth:  tb.Reset().Str("zgokh").Str(short).String(),
		PeerVeth:  tb.Reset().Str("zgokl").Str(short).String(),
		PidFile:   tb.Reset().Str("/run/ze-l2tp-dnsmasq-").Str(suffix).Str(".pid").String(),
		LeaseFile: tb.Reset().Str("/run/ze-l2tp-dnsmasq-").Str(suffix).Str(".leases").String(),
		LogFile:   tb.Reset().Str("/run/ze-l2tp-dnsmasq-").Str(suffix).Str(".log").String(),
		Progress:  os.Stderr,
	}
}

// setup builds the lab and leaves it able to carry an appliance.
//
// It removes existing objects first. A run that was killed leaves its bridge,
// its TAP, and its namespace behind under names that this run derives the same
// way.
func (g *gokrazyLab) setup() error {
	g.remove()
	if err := ensureNetnsDir(); err != nil {
		return err
	}

	var tb textbuf.Buffer
	steps := []struct {
		what string
		argv []string
	}{
		{tb.Str("create netns ").Str(g.Namespace).String(),
			[]string{"ip", ipNetns, ipAdd, g.Namespace}},
		// The bridge disables spanning tree and sets the forward delay to zero.
		// Otherwise, a port that was just enslaved spends seconds not forwarding.
		// The appliance's first DHCP DISCOVER arrives during that delay.
		{"create bridge",
			[]string{"ip", ipLink, ipAdd, ipName, g.Bridge, ipType, "bridge", "forward_delay", "0"}},
		{"assign bridge underlay address",
			[]string{"ip", ipAddr, ipAdd, g.cidr(g.HostIP), ipDev, g.Bridge}},
		{"bring up bridge", []string{"ip", ipLink, ipSet, g.Bridge, "up"}},
		// QEMU attaches with script=no, so the TAP must exist and be up before
		// the virtual machine starts.
		{"create tap", []string{"ip", "tuntap", ipAdd, ipDev, g.Tap, "mode", "tap"}},
		{"enslave tap to bridge", []string{"ip", ipLink, ipSet, g.Tap, "master", g.Bridge}},
		{"bring up tap", []string{"ip", ipLink, ipSet, g.Tap, "up"}},
		{"create LAC veth pair",
			[]string{"ip", ipLink, ipAdd, g.HostVeth, ipType, "veth", "peer", ipName, g.PeerVeth}},
		{"enslave host veth", []string{"ip", ipLink, ipSet, g.HostVeth, "master", g.Bridge}},
		{"bring up host veth", []string{"ip", ipLink, ipSet, g.HostVeth, "up"}},
		{"move LAC veth", []string{"ip", ipLink, ipSet, g.PeerVeth, ipNetns, g.Namespace}},
	}
	for i, step := range steps {
		if err := hostRequired(g.Progress, step.what, step.argv...); err != nil {
			return err
		}
		// The setup sets the spanning-tree switch immediately after it creates the
		// bridge. A failure is not fatal. A kernel that rejects the switch still
		// forwards, just later.
		if i == 1 {
			hostText("ip", ipLink, ipSet, g.Bridge, ipType, "bridge", "stp_state", "0") //nolint:errcheck // see above
		}
	}

	inside := []struct {
		what string
		argv []string
	}{
		{"bring up LAC loopback", []string{"ip", ipLink, ipSet, "lo", "up"}},
		{"assign LAC underlay address",
			[]string{"ip", ipAddr, ipAdd, g.cidr(g.PeerIP), ipDev, g.PeerVeth}},
		{"bring up LAC veth", []string{"ip", ipLink, ipSet, g.PeerVeth, "up"}},
	}
	for _, step := range inside {
		if err := nsRequired(g.Progress, g.Namespace, step.what, step.argv...); err != nil {
			return err
		}
	}

	g.allowBridgeThroughFirewall()
	if err := g.startDNSMasq(); err != nil {
		return err
	}

	if out, ok := nsText(g.Namespace, "ping", "-c", "1", "-W", "2", g.HostIP); !ok {
		writeProgress(g.Progress, out)
		return errors.New("LAC namespace cannot reach the underlay bridge")
	}
	return nil
}

// cidr answers an underlay address with the run's prefix length on it.
func (g *gokrazyLab) cidr(address string) string {
	var tb textbuf.Buffer
	return tb.Str(address).Byte('/').Str(g.Prefix).String()
}

// remove takes the whole lab down: the peer's processes, the DHCP server, the
// firewall hole, the four links and the namespace.
//
// Nothing here reports a failure. It runs on the way in, where none of it
// exists, and on the way out, where the run's verdict is what the caller reads.
func (g *gokrazyLab) remove() {
	killNamespaceProcesses(g.Namespace, syscall.SIGTERM)
	time.Sleep(settleGrace)
	killNamespaceProcesses(g.Namespace, syscall.SIGKILL)

	g.stopDNSMasq()
	g.clearBridgeFromFirewall()

	// The cleanup removes the TAP and the veths before the bridge they are
	// enslaved to. It removes the namespace last because the namespace holds one
	// end of the veth pair.
	for _, link := range []string{g.Tap, g.HostVeth, g.PeerVeth, g.Bridge} {
		hostText("ip", ipLink, "delete", link) //nolint:errcheck // cleanup
	}
	hostText("ip", ipNetns, "delete", g.Namespace) //nolint:errcheck // cleanup
}

// firewallActive reports whether this machine's firewall is turning traffic
// away. A machine with no ufw at all answers false and is left alone.
func (g *gokrazyLab) firewallActive() bool {
	if !onPath("ufw") {
		return false
	}
	out, ok := hostText("ufw", "status")
	return ok && strings.Contains(out, "Status: active")
}

// allowBridgeThroughFirewall opens the one hole that the appliance's DHCP
// needs.
//
// A default-deny INPUT chain drops the appliance's DISCOVER before dnsmasq can
// see it. The host stops that packet instead of bridging it. Later traffic
// passes between the peer and the appliance. The bridge traffic bypasses the
// filter. Thus, only DHCP needs a firewall rule.
func (g *gokrazyLab) allowBridgeThroughFirewall() {
	if !g.firewallActive() {
		return
	}
	hostText("ufw", "allow", "in", "on", g.Bridge) //nolint:errcheck // a firewall that refuses the rule reports itself when DHCP then fails
}

// clearBridgeFromFirewall takes the hole back out.
//
// It does not first ask whether the firewall is active. A rule added by a run
// that was killed survives an off-on firewall cycle.
func (g *gokrazyLab) clearBridgeFromFirewall() {
	if !onPath("ufw") {
		return
	}
	hostText("ufw", "--force", "delete", "allow", "in", "on", g.Bridge) //nolint:errcheck // cleanup
}

// startDNSMasq starts the DHCP server that supplies the appliance's address.
//
// DNS is off, and the reservation is a single address. This server tells ONE
// appliance ONE address that the peer already knows. The configuration
// deliberately does NOT request a bind to the interface. That option binds the
// socket to the interface's unicast address. The unicast address cannot receive
// the broadcast DISCOVER that the appliance sends.
func (g *gokrazyLab) startDNSMasq() error {
	os.Remove(g.PidFile)   //nolint:errcheck // a file from a previous run, whose absence is the wanted state
	os.Remove(g.LeaseFile) //nolint:errcheck // see above

	return hostRequired(g.Progress, "start dnsmasq for appliance DHCP", dnsmasqArgv(g)...)
}

// dnsmasqArgv answers the DHCP server this lab starts.
//
// A test can read this function's argv without dnsmasq on the machine. The proof
// needs a server reservation and no DNS. Both properties come from the argv,
// not from a running process.
func dnsmasqArgv(g *gokrazyLab) []string {
	var tb textbuf.Buffer
	return []string{
		"dnsmasq",
		tb.Str("--interface=").Str(g.Bridge).String(),
		"--port=0",
		"--no-resolv",
		"--no-hosts",
		tb.Reset().Str("--dhcp-range=").Str(g.ApplianceIP).Byte(',').Str(g.ApplianceIP).
			Byte(',').Str(dhcpNetmask).Byte(',').Str(dnsmasqLease).String(),
		tb.Reset().Str("--dhcp-host=").Str(g.ApplianceMAC).Byte(',').Str(g.ApplianceIP).String(),
		"--log-dhcp",
		tb.Reset().Str("--log-facility=").Str(g.LogFile).String(),
		tb.Reset().Str("--pid-file=").Str(g.PidFile).String(),
		tb.Reset().Str("--dhcp-leasefile=").Str(g.LeaseFile).String(),
	}
}

// stopDNSMasq ends the DHCP server this run started, and removes what it wrote.
//
// The pid file identifies the process that this run started. A process name CAN
// identify dnsmasq from another run on the same machine. If this function stops
// that process, the other run's appliance leaves the network.
func (g *gokrazyLab) stopDNSMasq() {
	body, err := os.ReadFile(g.PidFile) //nolint:gosec // a path this run named
	if err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(body))); err == nil {
			syscall.Kill(pid, syscall.SIGTERM) //nolint:errcheck // a process that has already gone is the wanted state
		}
	}
	os.Remove(g.PidFile)   //nolint:errcheck // cleanup
	os.Remove(g.LeaseFile) //nolint:errcheck // cleanup
}

// awaitAppliance waits until the appliance answers on the address it was
// reserved.
//
// The proof uses a ping instead of the lease file. The peer needs the
// appliance's stack to answer at the reserved address, not only a completed DHCP
// exchange. A lease does not prove that a dial will succeed.
func (g *gokrazyLab) awaitAppliance(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if _, ok := nsText(g.Namespace, "ping", "-c", "1", "-W", "1", g.ApplianceIP); ok {
			return nil
		}
		if !time.Now().Before(deadline) {
			var tb textbuf.Buffer
			return errors.New(tb.Str("appliance did not obtain underlay address ").
				Str(g.ApplianceIP).Str(" (DHCP) in time").String())
		}
		time.Sleep(appliancePoll)
	}
}

// dnsmasqLog answers what the DHCP server wrote, for a run whose appliance
// never appeared. It is the only place the exchange is visible.
func (g *gokrazyLab) dnsmasqLog() string {
	body, err := os.ReadFile(g.LogFile) //nolint:gosec // a path this run named
	if err != nil {
		return ""
	}
	return string(body)
}

// onPath reports whether a command exists to be run.
func onPath(name string) bool { return look(name) == nil }

// lacScratch answers a short directory for the peer's own files.
//
// This directory is deliberately SHORT and outside the checkout. xl2tpd 1.3.18
// truncates a configuration path longer than about ninety characters. The
// checkout's scratch path is already near that limit.
func lacScratch() (string, error) {
	return os.MkdirTemp(os.TempDir(), "zel2tp-")
}
