//go:build linux

// Design: docs/features/dns-resolver.md -- where the router's servers come from
// Related: register_dns_source_router_merge_linux.go -- the registration of the driver below
// Related: dns_source_router_follows_lease_linux.go -- the lease negotiation and the lookup reused here
// Related: dns_source_received_interface_linux.go -- the second user of the resolv.conf reader below

package fixture

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"os"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// mergeResolvConf is the file the daemon writes as resolv.conf, named
// relative to the runner's fresh per-test working directory, so no other test
// reads or writes it and no earlier run leaves it behind. Config validation
// refuses a relative resolv-conf-path (ze:validate "resolv-conf-path"), so
// the .ci names it /proc/self/cwd/<this name>: absolute, and
// inside the daemon's working directory. This fixture is the daemon's plugin
// and runs in that same directory, so it reads the bare name.
const mergeResolvConf = "dns-source-router-merge.resolv.conf"

// mergeConfiguredServer is the .ci's system name-server. The stub serves the
// zone on it; nothing listens on leaseDNSServer, so a query sent there is
// refused and the resolver must pass it to this server.
var mergeConfiguredServer = netip.MustParseAddr("198.51.100.53")

// dnsSourceFileAttempts and dnsSourceFileDelay bound each wait on the
// daemon's resolv.conf at 30 s.
const (
	dnsSourceFileAttempts = 120
	dnsSourceFileDelay    = 250 * time.Millisecond
)

// dnsSourceRouterMerge proves spec-home-cpe-23 AC-17 through the daemon:
// under system dns-source source [ received configured ] mode merge, the
// router lists the IPCP lease's server and then the configured one, and the
// internal resolver reaches the configured server when the lease's server
// does not answer. Before the lease the file must list the configured server
// alone, which also proves the daemon writes the path this fixture reads.
// After the lease it must list both, received first; only then is the lookup
// asked, so its answer comes from a resolver whose first server is dead.
func dnsSourceRouterMerge(ctx context.Context, plugin *sdk.Plugin) error {
	acCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	if err := dnsSourceWaitResolvConf(acCtx, mergeResolvConf, mergeConfiguredServer); err != nil {
		return fmt.Errorf("before the lease: %w", err)
	}

	fd, peer, err := renegotiationOpenAC()
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last read is done; a close error changes no verdict.

	// RFC 2516 Section 5.
	session, err := peer.Accept(acCtx)
	if err != nil {
		return fmt.Errorf("access concentrator: discovery: %w", err)
	}
	if err := leaseNegotiate(acCtx, session); err != nil {
		return fmt.Errorf("access concentrator: negotiation: %w", err)
	}

	if err := dnsSourceWaitResolvConf(acCtx, mergeResolvConf, leaseDNSServer, mergeConfiguredServer); err != nil {
		return fmt.Errorf("after the lease, mode merge: %w", err)
	}

	data, err := fixture06PollObject(acCtx, plugin, leaseLookup, leaseLookupAttempts,
		func(answer map[string]any) bool { return dnsRecordsEqual(answer, leaseStubRecord) })
	if err != nil {
		return fmt.Errorf("the daemon never resolved web.example.test through %s after %s refused it: %w",
			mergeConfiguredServer, leaseDNSServer, err)
	}

	var tb textbuf.Buffer
	tb.Str("OK: resolv.conf lists ").Str(leaseDNSServer.String()).Str(" then ").Str(mergeConfiguredServer.String()).
		Str(", and web.example.test type A -> ").Str(dnsAnswerLine(data)).
		Str(" through the configured server after the lease's server refused\n").StdErr()
	return nil
}

// dnsSourceWaitResolvConf polls the resolv.conf the daemon writes at path
// until its nameserver lines are exactly want, in order. A missing or
// unreadable file is "not yet": the daemon writes it after it starts. The
// error names the last servers read, so a red run shows what the daemon chose.
func dnsSourceWaitResolvConf(ctx context.Context, path string, want ...netip.Addr) error {
	var last []netip.Addr
	var readErr error
	if Poll(ctx, dnsSourceFileAttempts, dnsSourceFileDelay, func() bool {
		last, readErr = dnsSourceReadResolvConf(path)
		return readErr == nil && slices.Equal(last, want)
	}) {
		return nil
	}
	if readErr != nil {
		return fmt.Errorf("%s never listed %v: %w", path, want, readErr)
	}
	return fmt.Errorf("%s never listed %v: it lists %v", path, want, last)
}

// dnsSourceReadResolvConf answers the nameserver addresses of the resolv.conf
// at path, in file order. A nameserver line whose value is not an address is
// an error: the daemon writes only addresses (writeResolvFile).
func dnsSourceReadResolvConf(path string) ([]netip.Addr, error) {
	content, err := os.ReadFile(path) //nolint:gosec // path is the fixture's own constant, relative to the run directory
	if err != nil {
		return nil, err
	}
	var servers []netip.Addr
	lines := bufio.NewScanner(bytes.NewReader(content))
	for lines.Scan() {
		value, ok := strings.CutPrefix(lines.Text(), "nameserver ")
		if !ok {
			continue
		}
		addr, err := netip.ParseAddr(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("%s: nameserver %q: %w", path, value, err)
		}
		servers = append(servers, addr)
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return servers, nil
}
