//go:build linux

// Design: docs/architecture/provisioning/dhcp-server.md -- Functional tests: the probe and its WAN lease
// Related: register_dhcp_server_dns_replay_hold_linux.go -- the name test/pppoe/dhcp-server-dns-replay-hold.ci uses
// Related: dns_resolver_forward_ppp_replay_linux.go -- the resolver's replay scenario this one follows for the DHCP server
// Related: dns_source_router_follows_lease_linux.go -- leaseNegotiate and leaseDNSServer, the IPCP lease reused here
// Related: ../dhcpprobe/first_linux.go -- First, which judges the server's first reply

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/test/dhcpprobe"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// replayHoldLink is the DHCP server's listen interface in
// test/pppoe/dhcp-server-dns-replay-hold.ci, the server end of the veth pair
// the .ci's netns-link creates; the probe's end is replayHoldLink+"p".
const replayHoldLink = "zedrh"

// replayHoldResolvConf is the resolv.conf the daemon writes in its working
// directory, which is the fixture's working directory too.
const replayHoldResolvConf = "dhcp-server-dns-replay-hold.resolv.conf"

// replayHoldExchange is the request whose first reply is judged: option 6
// MUST carry the PPP lease's server alone, and the lease time MUST be the
// subnet's own, which the server shortens while the received class it reads
// is still empty (D-9).
const replayHoldExchange = "x,hold-first,type=discover,mac=02-00-00-00-00-71,expect=offer,dns=192.0.2.53,dns-count=1,lease=3600"

// The DHCPDISCOVER is resent replayHoldInterval apart, so one reaches the
// listener within that time of the moment it binds, long before the startup
// replay it holds for can be applied. replayHoldWindow bounds the whole
// wait, the reload included.
const (
	replayHoldInterval = 5 * time.Millisecond
	replayHoldWindow   = 30 * time.Second
)

// dhcpServerDNSReplayHold proves the DHCP server's half of spec-home-cpe-23b
// AC-13 (D-21) through the daemon. The boot config has no `service` block, so
// the dhcpserver plugin, whose config root is `service`, is not running. The
// scripted access concentrator brings up a PPPoE lease naming
// leaseDNSServer, and only once the daemon's resolv.conf lists that server
// does reloadSource add `service dhcp-server`. The plugin binds its listener
// when it is configured, before the snapshot-request it sends once started
// is answered, so its learned list is still empty while the first requests
// arrive. The probe is already sending a DHCPDISCOVER every
// replayHoldInterval when the SIGHUP goes out, and the first OFFER the server
// gives MUST already carry the lease's server: offerDNS holds every reply
// until the replay is applied (dnssource.Follower.AwaitReplay). Without the
// hold the first OFFER carries no option 6 and the shortened lease.
func dhcpServerDNSReplayHold(ctx context.Context, _ *sdk.Plugin, reloadSource string) error {
	runCtx, cancel := context.WithTimeout(ctx, 100*time.Second)
	defer cancel()

	fd, peer, err := renegotiationOpenAC()
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last read is done; a close error changes no verdict.

	// RFC 2516 Section 5.
	session, err := peer.Accept(runCtx)
	if err != nil {
		return fmt.Errorf("access concentrator: discovery: %w", err)
	}
	if err := leaseNegotiate(runCtx, session); err != nil {
		return fmt.Errorf("access concentrator: negotiation: %w", err)
	}
	if err := replayHoldAwaitRecord(runCtx); err != nil {
		return err
	}

	step, err := dhcpprobe.ParseStep(replayHoldExchange)
	if err != nil {
		return fmt.Errorf("BUG: dhcp-server-dns-replay-hold: %w", err)
	}
	probe, err := dhcpprobe.Open(replayHoldLink)
	if err != nil {
		return err
	}
	defer probe.Close() //nolint:errcheck // The verdict is already taken; a close error changes none.

	if err := replayHoldSignalReload(runCtx, reloadSource); err != nil {
		return fmt.Errorf("reload %s: %w", reloadSource, err)
	}
	emit := func(line string) {
		var tb textbuf.Buffer
		tb.Str(line).Str("\n").StdErr()
	}
	if err := probe.First(&step.Exchange, replayHoldInterval, replayHoldWindow, emit); err != nil {
		return fmt.Errorf("the DHCP server started after the PPPoE lease, its first offer: %w", err)
	}

	var tb textbuf.Buffer
	tb.Str("OK: dhcp-server started after the PPPoE lease -> first offer carries the replayed lease server ").
		Str(leaseDNSServer.String()).Str("\n").StdErr()
	return nil
}

// replayHoldSignalReload copies source over the daemon's config file and
// sends it SIGHUP, and returns at once. liveStagesReload would wait for the
// reload to finish, by which time the server has long applied the replay and
// the probe could no longer reach the replies the hold exists for.
func replayHoldSignalReload(ctx context.Context, source string) error {
	if !waitForFile(ctx, "daemon.pid", liveStagesPollAttempts, liveStagesPollDelay) {
		return errors.New("daemon.pid not found")
	}
	pid, err := readPID("daemon.pid")
	if err != nil {
		return err
	}
	data, err := os.ReadFile(source) //nolint:gosec // the source is a tmpfs file the .ci declares
	if err != nil {
		return err
	}
	if err := os.WriteFile(fileBGPConf, data, 0o600); err != nil {
		return err
	}
	return syscall.Kill(pid, syscall.SIGHUP)
}

// replayHoldAwaitRecord waits until the daemon's resolv.conf lists
// leaseDNSServer. The registry writes that file from its records, so the
// ppp record the replay must carry exists once it does.
func replayHoldAwaitRecord(ctx context.Context) error {
	var listed []netip.Addr
	var readErr error
	if Poll(ctx, dnsSourceFileAttempts, dnsSourceFileDelay, func() bool {
		listed, readErr = dnsSourceReadResolvConf(replayHoldResolvConf)
		return readErr == nil && slices.Contains(listed, leaseDNSServer)
	}) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("the daemon's %s never listed the lease's server %s: %w",
			replayHoldResolvConf, leaseDNSServer, readErr)
	}
	return fmt.Errorf("the daemon's %s lists %v, never the lease's server %s",
		replayHoldResolvConf, listed, leaseDNSServer)
}
