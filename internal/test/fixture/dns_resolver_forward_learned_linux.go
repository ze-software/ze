//go:build linux

// Design: docs/architecture/dns/lan-resolver.md -- Forwarding: the upstreams the forward dns-source list picks
// Related: register_dns_resolver_forward_learned_linux.go -- the registration of the drivers below
// Related: dns_source_router_follows_ra_rdnss_linux.go -- rdnssScene, the RA RDNSS setup both drivers reuse
// Related: reload_fixture_live_stages.go -- liveStagesReload, the reload the default driver commits through

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"time"

	mdns "github.com/miekg/dns"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/core/textbuf"
	dnsmock "github.com/ze-software/ze/internal/test/mock/dns"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// The name both drivers ask the LAN resolver. The learned stub serves the
// mock's default zone (203.0.113.10 with TTL 0, so no cached answer hides a
// change of upstream) and the configured stub is set to answer 203.0.113.77,
// so the answer names the upstream the resolver forwarded to.
const (
	forwardQueryName = "web.example.test."
	// forwardProbeName is the name dnsResolverForwardDefault asks before any
	// RA. It is not forwardQueryName, so a negative answer from whatever the
	// resolver reaches first (the guest's own resolver, through the host
	// fallback record) is cached against this name alone and cannot hide a
	// later stage's answer. The learned stub answers it like forwardQueryName.
	forwardProbeName = "probe.example.test."
	// forwardQueryTimeout bounds one exchange with the LAN resolver.
	forwardQueryTimeout = 2 * time.Second
)

var (
	forwardLearnedRecord    = netip.MustParseAddr("203.0.113.10")
	forwardConfiguredRecord = netip.MustParseAddr("203.0.113.77")
)

// The devices and addresses of test/plugin/dns-resolver-forward-received.ci:
// the resolver listens on forwardReceivedListener, and its forward server is
// forwardReceivedConfigured.
var (
	forwardReceivedScene = rdnssScene{
		receiver:   "zerfwr0",
		advertiser: "zerfwr1",
		stubLink:   "zerfwrdns0",
		server:     netip.MustParseAddr("2001:db8:53::153"),
	}
	forwardReceivedListener   = netip.MustParseAddrPort("127.0.23.53:53")
	forwardReceivedConfigured = netip.MustParseAddrPort("127.0.23.54:53")
)

// The devices and addresses of test/plugin/dns-resolver-forward-default.ci:
// the resolver listens on forwardDefaultListener, and the config the driver
// reloads adds forwardDefaultConfigured as its forward server.
var (
	forwardDefaultScene = rdnssScene{
		receiver:   "zerfwd0",
		advertiser: "zerfwd1",
		stubLink:   "zerfwddns0",
		server:     netip.MustParseAddr("2001:db8:53::253"),
	}
	forwardDefaultListener   = netip.MustParseAddrPort("127.0.23.63:53")
	forwardDefaultConfigured = netip.MustParseAddrPort("127.0.23.64:53")
)

// dnsResolverForwardReceived proves AC-6 through the daemon: the LAN
// resolver's list `forward dns-source source [ received configured ] mode
// first-usable` forwards to the configured server while nothing is learned,
// to the server an accepted RA's RDNSS option named once one is, and back to
// the configured server when a later RA gives that server Lifetime 0. Every
// step polls the resolver's answer, which only the upstream it forwarded to
// decides.
func dnsResolverForwardReceived(ctx context.Context, _ *sdk.Plugin) error {
	runCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	scene := forwardReceivedScene
	defer netlink.LinkDel(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: scene.stubLink}}) //nolint:errcheck // Cleanup after the verdict; a missing link is the goal.
	learned, err := scene.startStub()
	if err != nil {
		return err
	}
	defer learned.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	configured, err := forwardStartConfiguredStub(forwardReceivedConfigured)
	if err != nil {
		return err
	}
	defer configured.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	if !forwardAwaitAnswer(runCtx, forwardReceivedListener, forwardConfiguredRecord, nil) {
		return fmt.Errorf("the LAN resolver on %s never forwarded to the configured server %s while nothing was learned",
			forwardReceivedListener, forwardReceivedConfigured)
	}

	fd, err := scene.sender(runCtx)
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last send is done; a close error changes no verdict.

	// The daemon writes accept_ra on its end some time after it starts, and
	// the kernel drops an RA it is not yet accepting, so every poll sends one.
	var sendErr error
	advertise := func() { sendErr = scene.send(fd, raRDNSSLifetime) }
	if !forwardAwaitAnswer(runCtx, forwardReceivedListener, forwardLearnedRecord, advertise) {
		return errors.Join(fmt.Errorf("the LAN resolver never forwarded to the RDNSS server %s advertised to %s",
			scene.server, scene.receiver), sendErr)
	}

	// RFC 8106 Section 5.1: "A value of zero means that the RDNSS address
	// MUST no longer be used."
	if err := scene.send(fd, 0); err != nil {
		return err
	}
	if !forwardAwaitAnswer(runCtx, forwardReceivedListener, forwardConfiguredRecord, nil) {
		return fmt.Errorf("the LAN resolver did not fall back to the configured server %s after an RA gave %s Lifetime 0",
			forwardReceivedConfigured, scene.server)
	}

	var tb textbuf.Buffer
	tb.Str("OK: dns-resolver forward -> configured, then the RA RDNSS server, then configured again at Lifetime 0\n").StdErr()
	return nil
}

// dnsResolverForwardDefault proves AC-20 through the daemon: with no forward
// server and no forward dns-source, the LAN resolver forwards to a learned
// server under its built-in default, and once a reload adds a forward server
// it forwards to that server alone. reloadSource is the config the reload
// writes over the daemon's.
func dnsResolverForwardDefault(ctx context.Context, plugin *sdk.Plugin, reloadSource string) error {
	runCtx, cancel := context.WithTimeout(ctx, 75*time.Second)
	defer cancel()

	scene := forwardDefaultScene
	defer netlink.LinkDel(&netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: scene.stubLink}}) //nolint:errcheck // Cleanup after the verdict; a missing link is the goal.
	learned, err := scene.startStub()
	if err != nil {
		return err
	}
	defer learned.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	configured, err := forwardStartConfiguredStub(forwardDefaultConfigured)
	if err != nil {
		return err
	}
	defer configured.Close() //nolint:errcheck // The scenario's verdict is reached; a close error changes none.

	// Before any RA the resolver knows no learned upstream. A resolver that
	// answers the learned stub's record now reaches the stub by some route
	// other than the RA, and the rest of the run could not tell the RA's
	// effect. The probe asks forwardProbeName, which the learned stub serves
	// too, so the probe still sees that route if it exists.
	learned.Set(forwardProbeName, mdns.TypeA, dnsmock.Answer{Addresses: []netip.Addr{forwardLearnedRecord}})
	if got, err := forwardAnswerName(runCtx, forwardDefaultListener, forwardProbeName); err == nil && got == forwardLearnedRecord {
		return fmt.Errorf("the LAN resolver answered %s before any RA named %s: the test cannot tell the RA's effect", got, scene.server)
	}

	fd, err := scene.sender(runCtx)
	if err != nil {
		return err
	}
	defer unix.Close(fd) //nolint:errcheck // The scenario's last send is done; a close error changes no verdict.

	var sendErr error
	advertise := func() { sendErr = scene.send(fd, raRDNSSLifetime) }
	if err := forwardAwaitLearned(runCtx, forwardDefaultListener, learned, advertise); err != nil {
		return errors.Join(fmt.Errorf("under the default list the LAN resolver never forwarded to the RDNSS server %s advertised to %s",
			scene.server, scene.receiver), err, sendErr)
	}

	if err := liveStagesReload(runCtx, plugin, reloadSource); err != nil {
		return fmt.Errorf("reload %s: %w", reloadSource, err)
	}
	if !forwardAwaitAnswer(runCtx, forwardDefaultListener, forwardConfiguredRecord, nil) {
		return fmt.Errorf("after forward server %s was committed the LAN resolver never forwarded to it alone",
			forwardDefaultConfigured)
	}

	var tb textbuf.Buffer
	tb.Str("OK: dns-resolver default forward -> the RA RDNSS server, then the committed forward server alone\n").StdErr()
	return nil
}

// forwardStartConfiguredStub starts the stub that stands for the operator's
// forward server, answering forwardConfiguredRecord. Its address is on the
// loopback, which holds all of 127.0.0.0/8 on Linux.
func forwardStartConfiguredStub(address netip.AddrPort) (*dnsmock.Server, error) {
	stub, err := dnsmock.Start(address.String())
	if err != nil {
		return nil, fmt.Errorf("start the configured stub on %s: %w", address, err)
	}
	stub.Set("web.example.test", mdns.TypeA, dnsmock.Answer{Addresses: []netip.Addr{forwardConfiguredRecord}})
	return stub, nil
}

// forwardAwaitAnswer polls the resolver on listener until it answers want,
// running before ahead of each attempt when it is not nil.
func forwardAwaitAnswer(ctx context.Context, listener netip.AddrPort, want netip.Addr, before func()) bool {
	return Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		if before != nil {
			before()
		}
		got, err := forwardAnswer(ctx, listener)
		return err == nil && got == want
	})
}

// forwardAwaitLearned waits until the resolver on listener forwards to the
// learned stub, then asks forwardQueryName once, which MUST answer
// forwardLearnedRecord. Attempt n runs before when it is not nil, sets
// learned-<n>.example.test. on the stub and asks that name, which nothing
// asked before. Until the learned server is in the resolver's list a query
// can reach another upstream (the host fallback record names the guest's
// own resolver, which answers NXDOMAIN), and the resolver caches that
// negative answer against the name it asked. A fresh name per attempt keeps
// each such answer away from the next attempt, and forwardQueryName is asked
// only once the stub answered, so no negative answer is ever cached for it.
func forwardAwaitLearned(ctx context.Context, listener netip.AddrPort, stub *dnsmock.Server, before func()) error {
	attempt := 0
	reached := Poll(ctx, raRDNSSAttempts, raRDNSSDelay, func() bool {
		if before != nil {
			before()
		}
		attempt++
		name := "learned-" + strconv.Itoa(attempt) + ".example.test."
		stub.Set(name, mdns.TypeA, dnsmock.Answer{Addresses: []netip.Addr{forwardLearnedRecord}})
		got, err := forwardAnswerName(ctx, listener, name)
		return err == nil && got == forwardLearnedRecord
	})
	if !reached {
		return fmt.Errorf("the resolver on %s answered none of %d fresh names from the learned server", listener, attempt)
	}
	got, err := forwardAnswer(ctx, listener)
	if err != nil {
		return fmt.Errorf("the resolver on %s answered fresh names from the learned server, then not %s: %w",
			listener, forwardQueryName, err)
	}
	if got != forwardLearnedRecord {
		return fmt.Errorf("the resolver on %s answered %s %s, not %s from the learned server",
			listener, forwardQueryName, got, forwardLearnedRecord)
	}
	return nil
}

// forwardAnswer asks the resolver on listener for forwardQueryName
// (forwardAnswerName).
func forwardAnswer(ctx context.Context, listener netip.AddrPort) (netip.Addr, error) {
	return forwardAnswerName(ctx, listener, forwardQueryName)
}

// forwardAnswerName asks the resolver on listener for name type A with RD
// set and returns the first A record of a NOERROR reply. Any other reply is
// an error naming its rcode, never a zero address.
func forwardAnswerName(ctx context.Context, listener netip.AddrPort, name string) (netip.Addr, error) {
	query := new(mdns.Msg)
	query.SetQuestion(name, mdns.TypeA)
	query.RecursionDesired = true
	client := &mdns.Client{Net: "udp", Timeout: forwardQueryTimeout}
	reply, _, err := client.ExchangeContext(ctx, query, listener.String())
	if err != nil {
		return netip.Addr{}, err
	}
	if reply.Rcode != mdns.RcodeSuccess {
		return netip.Addr{}, fmt.Errorf("%s answered rcode %s", listener, mdns.RcodeToString[reply.Rcode])
	}
	for _, rr := range reply.Answer {
		record, isA := rr.(*mdns.A)
		if !isA {
			continue
		}
		addr, parsed := netip.AddrFromSlice(record.A.To4())
		if parsed {
			return addr, nil
		}
	}
	return netip.Addr{}, fmt.Errorf("%s answered NOERROR with no A record", listener)
}
