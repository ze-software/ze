// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client interface kind
// Related: config.go -- pppoeClientEntry parsed from YANG config

package iface

import (
	"context"
	"errors"
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/config/system"
)

// PPPoE client session states.
type pppoeSessionState uint8

const (
	sessStateDisconnected pppoeSessionState = iota
	sessStateDiscovery
	sessStateLCP
	sessStateAuth
	sessStateNCP
	sessStateUp
)

var errStopped = errors.New("pppoe-client: stopped")

// PPPoEClientConfig holds the configuration for a PPPoE client session.
type PPPoEClientConfig struct {
	Name            string
	SourceInterface string
	Username        string
	AuthSecret      string //nolint:gosec // not a hardcoded credential; parsed from ze:sensitive YANG leaf
	ServiceName     string
	ACName          string
	NoDefaultRoute  bool
	MTU             int
	// RoutePriority is the kernel metric of the default route installed when
	// IPCP completes. It carries defaultLearnedRouteMetric unless the
	// pppoe-client sets route-priority, so a session learned from an access
	// concentrator does not land on the metric an operator's static default
	// occupies.
	RoutePriority int
	// Timing is the LCP, IPv6CP, keepalive and reconnect timing. A reload that changes
	// only Timing keeps the running session: reconcilePPPoEClients stores the
	// new value through setTiming, the next Dial and the next redial wait
	// read it. Readers outside the run goroutine MUST hold mu.
	Timing PPPoEClientTiming
	// IPv6 is the ipv6 container: IPv6CP, the device's IPv6 sysctls and the
	// DHCPv6 client (reconcileDHCP) on the session device.
	IPv6 PPPoEClientIPv6
}

// PPPoEClientStatus is a snapshot of one client's state, the answer of
// `show interface pppoe-client` (PPPoEClientCommand).
type PPPoEClientStatus struct {
	Name            string     `json:"name"`
	State           string     `json:"state"`
	SourceInterface string     `json:"source-interface"`
	PPPInterface    string     `json:"ppp-interface"`
	SessionID       uint16     `json:"session-id"`
	LocalIP         netip.Addr `json:"local-ip"`
	PeerIP          netip.Addr `json:"peer-ip"`
	// Uptime is the session's age to the second, empty while it is not up.
	Uptime string `json:"uptime"`
	// DNSServers are the servers IPCP learned and resolv.conf was given,
	// primary first. It is never nil, so the key is an empty list when the
	// access concentrator supplied none.
	DNSServers []string `json:"dns-servers"`
	// DelegatedPrefixes are the prefixes the client's DHCPv6 lease holds
	// (RFC 9915 IA_PD), with the lifetimes that remain, sorted by prefix. It
	// is never nil, so the key is an empty list when the client requests no
	// prefix or the server delegated none.
	DelegatedPrefixes []DelegatedPrefixStatus `json:"delegated-prefixes"`
}

// PPPoEDialer abstracts the platform-specific PPPoE discovery and kernel
// session setup. Defined here to break the import cycle (pppoe imports
// iface). Production implementation lives in pppoeclient package.
type PPPoEDialer interface {
	Dial(cfg PPPoEClientConfig, stopCh <-chan struct{}, logger *slog.Logger) (PPPoESession, error)
}

// PPPoEResult carries every negotiated interface setting as an immutable value.
// It contains no transport ownership or mutable authentication state.
type PPPoEResult struct {
	// LocalIP and PeerIP are invalid, with both DNS servers, while IPCP is
	// Stopped and IPv6CP holds the session (RFC 7084 WLL-3): the consumer
	// then withdraws the IPv4 route, address pair and DNS lease.
	LocalIP netip.Addr
	PeerIP  netip.Addr
	NegMTU  uint16
	// PrimaryDNS and SecondaryDNS are the RFC 1877 servers IPCP learned. An
	// invalid address means the access concentrator supplied none.
	PrimaryDNS   netip.Addr
	SecondaryDNS netip.Addr
	// LinkLocal is the local IPv6 link-local address IPv6CP negotiated
	// (RFC 5072 Section 5), invalid while IPv6CP is not Opened or when the
	// client configures no ipv6.
	LinkLocal netip.Addr
	// Seq numbers the results one session publishes, from 1 and increasing.
	// The consumer hands it back through PPPoESession.Applied once the
	// result's MTU, addresses, route and DNS are on the device.
	Seq uint64
}

// PPPoESession represents a fully negotiated PPPoE+PPP session.
type PPPoESession struct {
	SessionID uint16
	// UnitNum remains reserved until the owner invokes Cleanup, even after Done closes.
	// The caller may finish configuring the session device, which carries the
	// configured name and not ppp<UnitNum>, while retaining ownership.
	UnitNum int
	// NativeWriter carries this Dial's exact original-unit capability. UnitNum
	// and the configured name are labels, never native lookup authority.
	NativeWriter NativePPPWriter
	LocalIP      netip.Addr
	PeerIP       netip.Addr
	NegMTU       uint16
	// PrimaryDNS and SecondaryDNS are the RFC 1877 servers IPCP learned. An
	// invalid address means the access concentrator supplied none.
	PrimaryDNS   netip.Addr
	SecondaryDNS netip.Addr
	// Changes carries a complete result after every successful renegotiation,
	// including unchanged results. The sole producer replaces the oldest pending
	// value in a one-slot channel. It never closes Changes; Done ends consumption.
	Changes <-chan PPPoEResult
	// Applied MUST be called by the consumer of Changes after it has applied
	// a result (MTU, address pair, default route and DNS lease), with that
	// result's Seq, including a result that changed nothing or whose route
	// install only warned. Pending or explicitly unavailable native outcomes
	// MUST end application before Applied or another native mutation.
	// After an LCP or IPCP renegotiation the session
	// keeps IPv4, and IPv6 when IPv6CP opened, discarded until Applied
	// reports that result, so no packet leaves with the replaced address or
	// MTU (pppd's lcp_up sets the MTU before link_established opens the
	// NCPs). The session ends the wait when it stops. Nil when the dialer
	// does not wait. Safe to call from the consumer while the session runs.
	Applied func(seq uint64)
	// Done is closed when the session ends (echo timeout, LCP terminate).
	// The PPPoEClient selects on this to detect session loss.
	Done <-chan struct{}
	// Cleanup stops the original transport and workers, sends PADT, and
	// retires the exact unit before settling its writer and backend. The caller
	// MUST invoke nonnil Cleanup even when Dial fails, and MUST finish native
	// use first. It reports every cause; ErrNativePPPPending retains custody
	// and forbids another admission until a later call explicitly completes.
	// Repeated calls retry pending cleanup only, never a native mutation.
	Cleanup func() error
}

// PPPoEClient manages a single PPPoE client session lifecycle.
type PPPoEClient struct {
	cfg             PPPoEClientConfig
	native          NativePPPWriter
	registry        *pppoeClientSet      // immutable after publication; same actual map/mutex
	nativeAdmission pppoeNativeAdmission // guarded by admissionMutex
	dialer          PPPoEDialer
	logger          *slog.Logger

	mu      sync.Mutex
	state   pppoeSessionState
	sessID  uint16
	pppUnit int
	localIP netip.Addr
	peerIP  netip.Addr
	// dnsServers are the servers leased to resolvers, primary first.
	dnsServers []string
	upSince    time.Time
	lastErr    error
	attempts   int

	// resolvers is the resolv.conf writer the learned servers go to.
	resolvers *system.Resolvers
	// sessionEnded is called with the client's name when a session ends,
	// beside the IPCP lease's withdrawal, so the RA record learned over the
	// session device leaves with it: runEngine passes the RA RDNSS
	// producer's withdrawPPPoESession. Nil calls nothing, as in a test that
	// drives the client alone. Set before Start and never written after.
	sessionEnded func(name string)

	stopCh          chan struct{}
	done            chan struct{}
	stopOnce        sync.Once
	startOnce       sync.Once
	cleanupBusy     atomic.Bool
	pendingCleanup  func() error // one original session; protected by mu
	cleanupErr      error        // latest actual cleanup result; protected by mu
	cleanupReportAt time.Time    // next bounded history report; protected by mu
}

// NewPPPoEClient creates a PPPoE client from config. Call Start to begin.
func NewPPPoEClient(cfg PPPoEClientConfig, dialer PPPoEDialer, logger *slog.Logger) *PPPoEClient { //nolint:gocritic // hugeParam: the public value signature establishes an owned config snapshot, off the data plane.
	return &PPPoEClient{
		cfg:             cfg,
		dialer:          dialer,
		logger:          logger.With("pppoe-client", cfg.Name, "src-iface", cfg.SourceInterface),
		state:           sessStateDisconnected,
		nativeAdmission: pppoeNativeIdle,
		pppUnit:         -1,
		// The process writer the hub configures, as the DHCP client uses.
		resolvers: system.ProcessResolvers(),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// Start launches the session worker once. Safe for concurrent use with Stop.
// A client stopped before its worker starts never dials and still publishes done.
func (c *PPPoEClient) Start() {
	c.startOnce.Do(func() {
		if c.stopped() {
			close(c.done)
			return
		}
		go c.run()
	})
}

// Stop permanently fences Dial, joins the worker and retries original cleanup.
// Safe for concurrent use. A Pending result retains custody; callers MUST keep
// this client and retry Stop before replacing it. Terminal causes remain reported.
func (c *PPPoEClient) Stop() error {
	c.stopOnce.Do(func() { close(c.stopCh) })
	c.Start() // A stopped client that never started still publishes done.
	<-c.done
	err := c.finishCleanup()
	c.mu.Lock()
	result := errors.Join(c.cleanupErr, err)
	c.mu.Unlock()
	return result
}

// Status returns a snapshot of the client state.
func (c *PPPoEClient) status() PPPoEClientStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	pppIface := ""
	if c.pppUnit >= 0 {
		// The kernel device carries the configured name once the dialer
		// creates it through RTM_NEWLINK kind "ppp" with IFLA_IFNAME. For a
		// client that has never completed Dial (pppUnit still -1) the OS
		// name column stays empty. See docs/architecture/l2tp/cpe-1-pppoe-client.md.
		pppIface = c.cfg.Name
	}

	uptime := ""
	if c.state == sessStateUp && !c.upSince.IsZero() {
		uptime = time.Since(c.upSince).Truncate(time.Second).String()
	}

	return PPPoEClientStatus{
		Name:            c.cfg.Name,
		State:           c.stateString(),
		SourceInterface: c.cfg.SourceInterface,
		PPPInterface:    pppIface,
		LocalIP:         c.localIP,
		PeerIP:          c.peerIP,
		SessionID:       c.sessID,
		Uptime:          uptime,
		DNSServers:      append([]string{}, c.dnsServers...),
		// The lease events are keyed by the client's name, the source a
		// LAN unit's delegated-prefix entry names.
		DelegatedPrefixes: heldDelegations.statusOf(c.cfg.Name, time.Now()),
	}
}

func (c *PPPoEClient) stateString() string {
	switch c.state {
	case sessStateDisconnected:
		return "disconnected"
	case sessStateDiscovery:
		return "discovery"
	case sessStateLCP:
		return "lcp"
	case sessStateAuth:
		return "authenticating"
	case sessStateNCP:
		return "ncp"
	case sessStateUp:
		return "up"
	default:
		panic("BUG: unknown PPPoE client session state")
	}
}

func (c *PPPoEClient) stopped() bool {
	select {
	case <-c.stopCh:
		return true
	default:
		return false
	}
}

// run is the main goroutine loop with reconnection on failure (AC-10).
func (c *PPPoEClient) run() {
	defer close(c.done)

	for {
		if c.stopped() {
			return
		}
		err := c.runSession()
		if errors.Is(err, ErrNativePPPPending) {
			c.logger.Warn("pppoe-client: original native cleanup remains pending", "error", err,
				"code", pppoeCleanupDoctorCode, "fix", pppoeCleanupRecovery(err))
		}
		if err == nil || errors.Is(err, errStopped) {
			return
		}

		if c.stopped() {
			return
		}

		c.mu.Lock()
		c.state = sessStateDisconnected
		c.lastErr = err
		c.attempts++
		delay, delayErr := c.cfg.Timing.reconnectDelay(c.attempts)
		c.mu.Unlock()

		if delayErr != nil {
			c.logger.Error("pppoe-client: the redial wait is invalid, so the client stops",
				"error", delayErr, "fix", "correct the reconnect and keepalive values and commit")
			return
		}

		// The session's end carries its cause (a refused /dev/ppp, a lost
		// carrier, a peer PADT): at Info it vanished at the default level, so a
		// client that can never come up left nothing in the log.
		c.logger.Warn("pppoe-client: session ended, reconnecting",
			"error", err, "delay", delay, "attempt", c.attempts,
			"fix", "see the error for the cause; inspect with ze show interface pppoe-client")

		select {
		case <-c.stopCh:
			return
		case <-time.After(delay):
		}
	}
}

// setTiming stores a reloaded timing for the next session and the next redial
// wait. The running session keeps the LCP, IPv6CP and keepalive values it was dialed with.
func (c *PPPoEClient) setTiming(timing PPPoEClientTiming) {
	c.mu.Lock()
	c.cfg.Timing = timing
	c.mu.Unlock()
}

// runSession executes one complete PPPoE client session: discovery,
// LCP/auth/NCP negotiation, interface setup, then monitors until teardown.
func (c *PPPoEClient) runSession() (result error) {
	if err := c.finishCleanup(); err != nil {
		return err
	}
	c.mu.Lock()
	c.state = sessStateDiscovery
	c.sessID = 0
	c.pppUnit = -1
	c.localIP = netip.Addr{}
	c.peerIP = netip.Addr{}
	c.dnsServers = nil
	// setTiming writes cfg.Timing from the reconcile goroutine.
	dialCfg := c.cfg
	c.mu.Unlock()

	// PPPoE Discovery + LCP + Auth + NCP via the Dialer abstraction.
	if err := c.beginNativeDial(); err != nil {
		return err
	}
	sess, err := c.dialer.Dial(dialCfg, c.stopCh, c.logger)
	c.finishNativeDial(sess.Cleanup != nil, err)
	if sess.Cleanup != nil {
		c.mu.Lock()
		c.pendingCleanup = sess.Cleanup
		c.mu.Unlock()
		defer func() { result = errors.Join(result, c.finishCleanup()) }()
	}
	if err != nil {
		if sess.Cleanup == nil {
			if sess.NativeWriter != nil {
				return errors.Join(err, c.retainMissingCleanup(sess))
			}
		}
		return err
	}
	if sess.Cleanup == nil {
		return c.retainMissingCleanup(sess)
	}
	if sess.NativeWriter == nil {
		return errors.Join(ErrNativePPPUnavailable, errors.New("pppoe-client: negotiated original has no native writer"))
	}
	c.native = sess.NativeWriter
	// The leases end with the session, on every return path below.
	defer c.endSessionLeases()

	// The configured name is diagnostic and selects existing IPv6 sysctls.
	// Native mutations use only the original writer returned by this Dial.
	ifname := c.cfg.Name
	mtuApplied := int(sess.NegMTU)
	if c.cfg.MTU > 0 {
		mtuApplied = min(mtuApplied, c.cfg.MTU)
	}
	pairApplied := [2]netip.Addr{sess.LocalIP, sess.PeerIP}
	// An invalid routeApplied explicitly means no successful route install.
	// It guards removal and keeps an initial failed install retryable.
	var routeApplied netip.Addr
	// An invalid routeRetired means no old cleanup awaits confirmation.
	// AddRoute replaces the fixed prefix/metric slot, so successful installs
	// cannot accumulate routes while this one cleanup remains pending.
	var routeRetired netip.Addr
	// An invalid linkLocalApplied means no IPv6CP link-local is installed.
	var linkLocalApplied netip.Addr

	// AC-7: Set MTU on the ppp interface.
	if c.native != nil {
		if err := c.nativeWrite(func(ctx context.Context) error { return c.native.SetMTU(ctx, mtuApplied) }); err != nil {
			return err
		}
		if err := c.nativeWrite(func(ctx context.Context) error { return c.native.SetAdminUp(ctx) }); err != nil {
			return err
		}
		if c.cfg.IPv6.Enabled {
			// The device is new on every session, so its accept_ra,
			// autoconf and forwarding are written before IPv6CP opens and
			// the access concentrator's Router Advertisements arrive.
			applyPPPoEClientSysctls(ifname, c.cfg.IPv6)
		}
		// AC-5: Apply server-assigned address as P2P link.
		if sess.LocalIP.IsValid() {
			if sess.PeerIP.IsValid() {
				if err := c.addNativePair(sess.LocalIP, sess.PeerIP); err != nil {
					return err
				}
			}
		}
		// AC-8: Install default route unless no-default-route is set.
		if !c.cfg.NoDefaultRoute {
			if sess.PeerIP.IsValid() {
				if err := c.addNativeRoute(sess.PeerIP); err != nil {
					c.warnRouteApply(ifname, sess.PeerIP, err)
					if pppoeNativeRefused(err) {
						return err
					}
				} else {
					routeApplied = sess.PeerIP
				}
			}
		}
	}

	dnsLeased := pppoeDNSServers(sess.PrimaryDNS, sess.SecondaryDNS)
	c.leaseDNS(dnsLeased)

	c.mu.Lock()
	c.sessID = sess.SessionID
	c.pppUnit = sess.UnitNum
	c.localIP = sess.LocalIP
	c.peerIP = sess.PeerIP
	c.state = sessStateUp
	c.upSince = time.Now()
	c.attempts = 0
	c.mu.Unlock()

	c.logger.Info("pppoe-client: session up",
		"session-id", sess.SessionID, "iface", ifname,
		"local-ip", sess.LocalIP, "peer-ip", sess.PeerIP, "dns-servers", dnsLeased)

	// This worker owns all interface writes until stop, session loss or a fatal
	// apply error. Changes holds at most one complete pending result.
	for {
		select {
		case <-c.stopCh:
			return nil
		case <-sess.Done:
			return errors.New("pppoe-client: session lost")
		case next, ok := <-sess.Changes:
			if !ok {
				return errors.New("pppoe-client: session change source closed")
			}
			if c.native != nil {
				mtu := int(next.NegMTU)
				if c.cfg.MTU > 0 {
					mtu = min(mtu, c.cfg.MTU)
				}
				if mtu != mtuApplied {
					if err := c.nativeWrite(func(ctx context.Context) error { return c.native.SetMTU(ctx, mtu) }); err != nil {
						return err
					}
					mtuApplied = mtu
				}
				if next.LocalIP.IsValid() {
					if err := c.applyPPPoEIPv4(ifname, next, &routeApplied, &routeRetired, &pairApplied); err != nil {
						return err
					}
				} else {
					// RFC 7084 Section 4.2 WLL-3: IPCP is down while IPv6CP
					// holds the session, so IPv4 alone is withdrawn.
					if err := c.withdrawPPPoEIPv4(ifname, &routeApplied, &routeRetired, &pairApplied); err != nil {
						return err
					}
				}
				// RFC 5072 Section 5: the link-local follows IPv6CP, and
				// IPv4 is untouched by it (RFC 7084 WLL-3).
				applied, err := c.applyNativeLinkLocal(linkLocalApplied, next.LinkLocal)
				if err != nil {
					return err
				}
				linkLocalApplied = applied
			}
			// A renegotiation that changed only the DNS servers still
			// replaces the lease; an equal list makes no call. A result
			// without IPv4 names no server, so the lease is withdrawn.
			if dns := pppoeDNSServers(next.PrimaryDNS, next.SecondaryDNS); !slices.Equal(dns, dnsLeased) {
				c.leaseDNS(dns)
				dnsLeased = dns
			}
			c.mu.Lock()
			c.localIP = next.LocalIP
			c.peerIP = next.PeerIP
			c.mu.Unlock()
			// The session holds forwarding discarded after a renegotiation
			// until this call: report the result only once it is applied.
			// refuse-by-default: guards PPPoESession -- a dialer that sets no Applied does not wait for one
			if sess.Applied != nil {
				sess.Applied(next.Seq)
			}
		}
	}
}

// withdrawPPPoEIPv4 removes the IPv4 state a session result no longer
// carries: the installed default route, a retired one still awaiting cleanup,
// then the point-to-point address pair. A failed route removal keeps its
// ownership for the next result to retry; a failed address removal ends the
// session as every address error does. Called only from runSession's worker.
//
// RFC 7084 Section 4.2 WLL-3: "If the WAN interface supports PPP
// encapsulation, in a dual-stack environment with IPCP and IPV6CP running
// over one PPP logical channel, the Network Control Protocols (NCPs) MUST be
// treated as independent of each other and start and terminate
// independently."
func (c *PPPoEClient) withdrawPPPoEIPv4(ifname string, routeApplied, routeRetired *netip.Addr, pairApplied *[2]netip.Addr) error {
	// A route is owned only after a successful install, so an owned route
	// is removed even when no-default-route would not install it again.
	if routeApplied.IsValid() {
		if err := c.removeNativeRoute(*routeApplied); err != nil {
			c.warnRouteRemoval(ifname, *routeApplied, err)
			if pppoeNativeRefused(err) {
				return err
			}
		} else {
			*routeApplied = netip.Addr{}
		}
	}
	if routeRetired.IsValid() {
		if err := c.removeNativeRoute(*routeRetired); err != nil {
			c.warnRouteRemoval(ifname, *routeRetired, err)
			if pppoeNativeRefused(err) {
				return err
			}
		} else {
			*routeRetired = netip.Addr{}
		}
	}
	if pairApplied[0].IsValid() {
		if err := c.removeNativePair(pairApplied[0], pairApplied[1]); err != nil {
			return err
		}
		*pairApplied = [2]netip.Addr{}
	}
	return nil
}

// applyPPPoEIPv4 installs the address pair and default gateway of a result
// that carries IPv4, new values before the removal of old ones. Called only
// from runSession's worker.
func (c *PPPoEClient) applyPPPoEIPv4(ifname string, next PPPoEResult, routeApplied, routeRetired *netip.Addr, pairApplied *[2]netip.Addr) error {
	pair := [2]netip.Addr{next.LocalIP, next.PeerIP}
	if pair != *pairApplied {
		if err := c.addNativePair(next.LocalIP, next.PeerIP); err != nil {
			return err
		}
	}
	if !c.cfg.NoDefaultRoute {
		if next.PeerIP != *routeApplied {
			if err := c.addNativeRoute(next.PeerIP); err != nil {
				c.warnRouteApply(ifname, next.PeerIP, err)
				if pppoeNativeRefused(err) {
					return err
				}
			} else {
				// Linux fib_table_insert replaces the prefix/metric slot.
				// Keep one failed cleanup, not a backlog of replaced gateways.
				if !routeRetired.IsValid() {
					*routeRetired = *routeApplied
				}
				*routeApplied = next.PeerIP
				if *routeRetired == *routeApplied {
					// This gateway is desired again. Never remove it later.
					*routeRetired = netip.Addr{}
				}
			}
		}
		if routeRetired.IsValid() {
			if err := c.removeNativeRoute(*routeRetired); err != nil {
				c.warnRouteRemoval(ifname, *routeRetired, err)
				if pppoeNativeRefused(err) {
					return err
				}
			} else {
				*routeRetired = netip.Addr{}
			}
		}
	}
	if pair != *pairApplied {
		// A session that came up without IPv4, or whose IPv4 was withdrawn,
		// holds no pair, and an absent pair has nothing to remove.
		if pairApplied[0].IsValid() {
			if err := c.removeNativePair(pairApplied[0], pairApplied[1]); err != nil {
				return err
			}
		}
		*pairApplied = pair
	}
	return nil
}

// pppoeDNSServers lists the servers IPCP learned for resolv.conf, primary
// first, leaving out an option that named no server.
//
// RFC 1877 Section 1: "Primary and secondary addresses are negotiated
// independently.  They serve identical purposes, except that when both are
// present an attempt SHOULD be made to resolve names using the primary
// address before using the secondary address."
func pppoeDNSServers(primary, secondary netip.Addr) []string {
	var servers []string
	if primary.IsValid() {
		servers = append(servers, primary.String())
	}
	if secondary.IsValid() {
		servers = append(servers, secondary.String())
	}
	return servers
}

// PPPoEResolverSource names the resolv.conf lease of the PPPoE client
// called name. It is keyed by the configured name, which a reconnect keeps,
// never by the ppp unit, which a reconnect changes.
func PPPoEResolverSource(name string) system.ResolverSource {
	return pppoeClientResolverOrigin.Source(name, "")
}

// leaseDNS hands servers to the resolv.conf writer as this client's lease;
// an empty list withdraws it. Static name-servers still win
// (system.Resolvers). A failed write keeps the session: the operator reads
// the warning, and the next change retries.
func (c *PPPoEClient) leaseDNS(servers []string) {
	c.mu.Lock()
	c.dnsServers = servers
	c.mu.Unlock()
	if err := c.resolvers.SetLease(PPPoEResolverSource(c.cfg.Name), servers, system.UntilWithdrawn); err != nil {
		c.logger.Warn("pppoe-client: failed to write the learned DNS servers to resolv.conf; name resolution may use stale servers; inspect with ze show interface pppoe-client",
			"dns-servers", servers, "error", err)
	}
}

// endSessionLeases withdraws the resolver records a session held: the IPCP
// lease, then, through sessionEnded, the RA record learned over the session
// device, which is gone with the session. The DHCPv6 lease is not among
// them: its client keeps it across the device's loss, as a possible move to
// a new link (RFC 9915 Section 18.2.12). Called only from runSession's
// worker, once per session.
func (c *PPPoEClient) endSessionLeases() {
	c.leaseDNS(nil)
	if c.sessionEnded != nil {
		c.sessionEnded(c.cfg.Name)
	}
}

// warnRouteApply keeps a failed route install visible without ending PPP.
func (c *PPPoEClient) warnRouteApply(device string, gateway netip.Addr, err error) {
	c.logger.Warn("pppoe-client: failed to add default route; traffic may fail until a later session result retries it; inspect with ze show interface",
		"route", "0.0.0.0/0", "gateway", gateway, "device", device, "error", err)
}

// warnRouteRemoval reports retained ownership so a later record can retry it.
func (c *PPPoEClient) warnRouteRemoval(device string, gateway netip.Addr, err error) {
	c.logger.Warn("pppoe-client: failed to remove old default route; cleanup will retry without delaying the desired gateway; inspect with ze show interface",
		"route", "0.0.0.0/0", "gateway", gateway, "device", device, "error", err)
}

// pppoeDialerVar holds the production PPPoEDialer, set by the pppoeclient
// package at init time. Nil on non-Linux or when the package is not linked.
var pppoeDialerVar PPPoEDialer

// SetPPPoEDialer registers the production dialer. Called from pppoeclient's
// init() or from the loader that imports the pppoeclient package.
func SetPPPoEDialer(d PPPoEDialer) { pppoeDialerVar = d }

// reconcilePPPoEClients starts clients for new config entries, stops
// clients whose config was removed or disabled. Each client it starts calls
// sessionEnded with its name when a session ends (PPPoEClient.sessionEnded);
// nil calls nothing. activeMu MUST be the same mutex that guards active.
// No registry lock spans Stop, native I/O or sessionEnded callbacks.
func reconcilePPPoEClients(cfg *ifaceConfig, active map[string]*PPPoEClient, activeMu *sync.Mutex, log *slog.Logger, sessionEnded func(name string)) {
	if pppoeDialerVar == nil {
		return
	}

	desired := make(map[string]pppoeClientEntry, len(cfg.PPPoEClient))
	for index := range cfg.PPPoEClient {
		e := &cfg.PPPoEClient[index]
		if !e.Disable {
			desired[e.Name] = *e
		}
	}

	activeMu.Lock()
	snapshot := make(map[string]*PPPoEClient, len(active))
	for name, client := range active {
		snapshot[name] = client
	}
	activeMu.Unlock()
	for name, client := range snapshot {
		entry, wanted := desired[name]
		client.mu.Lock()
		running := client.cfg
		client.mu.Unlock()
		stop := !wanted
		if client.stopped() {
			stop = true
		}
		if pppoeClientConfigChanged(&running, &entry) {
			stop = true
		}
		if stop {
			err := client.Stop()
			if err != nil {
				log.Warn("pppoe-client: original client cleanup failed", "name", name, "error", err,
					"code", pppoeCleanupDoctorCode, "fix", pppoeCleanupRecovery(err))
			}
			if errors.Is(err, ErrNativePPPPending) {
				continue
			}
			activeMu.Lock()
			if active[name] == client {
				delete(active, name)
			}
			activeMu.Unlock()
			continue
		}
		if running.Timing != entry.Timing {
			client.setTiming(entry.Timing)
		}
	}

	// Publish desired clients without allocating native resources. Their real
	// Dial takes a fresh synchronized admission check; transient native work
	// cannot drop a desired config entry or require another config event.
	for name, entry := range desired {
		client := NewPPPoEClient(pppoeClientConfigFrom(&entry), pppoeDialerVar, log)
		client.sessionEnded = sessionEnded
		client.registry = pppoeRegistryFor(active, activeMu)
		activeMu.Lock()
		_, exists := active[name]
		if !exists {
			active[name] = client
		}
		activeMu.Unlock()
		if exists {
			continue
		}
		// Stop can win after publication; Start's terminal fence cannot Dial.
		client.Start()
		log.Info("pppoe-client: started client", "name", name, "src-iface", entry.SourceInterface)
	}
}

// pppoeClientConfigFrom builds the client config one parsed entry declares.
// entry MUST be non-nil and remain unchanged for the call; no pointer is retained.
func pppoeClientConfigFrom(entry *pppoeClientEntry) PPPoEClientConfig {
	return PPPoEClientConfig{
		Name:            entry.Name,
		SourceInterface: entry.SourceInterface,
		Username:        entry.Username,
		AuthSecret:      entry.AuthSecret,
		ServiceName:     entry.ServiceName,
		ACName:          entry.ACName,
		NoDefaultRoute:  entry.NoDefaultRoute,
		MTU:             entry.MTU,
		RoutePriority:   entry.RoutePriority,
		Timing:          entry.Timing,
		IPv6:            entry.IPv6,
	}
}

// pppoeClientConfigChanged reports whether entry needs a restart of the
// running client. It compares every field but Timing, so a field added to
// PPPoEClientConfig restarts the client on change with no edit here. Timing
// alone never restarts: reconcilePPPoEClients hands it to setTiming instead.
// Both inputs MUST be non-nil snapshots; neither is changed or retained.
func pppoeClientConfigChanged(running *PPPoEClientConfig, entry *pppoeClientEntry) bool {
	desired := pppoeClientConfigFrom(entry)
	desired.Timing = running.Timing
	return *running != desired
}
