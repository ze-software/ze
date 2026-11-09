// Design: docs/features/interfaces.md -- Router Advertisement send loop (Linux)
// Related: ifacera.go -- the counters this loop feeds
//
// The RFC 4861 timing helpers this loop calls are internal/core/ndp, shared
// with the PPP subscriber sender.

//go:build linux

package ifacera

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
	"github.com/ze-software/ze/internal/core/routewatch"
)

// allRoutersGroup is the all-routers multicast address a router joins so it
// receives Router Solicitations (RFC 4861 Section 6.1.1).
const allRoutersGroup = "ff02::2"

// allNodesGroup is where unsolicited and solicited advertisements go
// (RFC 4861 Section 6.2.6).
const allNodesGroup = "ff02::1"

// rsReadBufferSize bounds one Router Solicitation read. A solicitation is a
// short message and only its arrival matters, so a longer one is truncated
// rather than buffered.
const rsReadBufferSize = 256

// Sender advertises on one interface unit. One goroutine owns the socket and
// every timer, so a stopped Sender holds no socket and no multicast group join.
type Sender struct {
	spec iface.RASenderSpec
	log  *slog.Logger
	// updates carries a replacement spec to the send loop. Depth 1, and the
	// latest wins: Update drains a spec the loop has not read yet.
	updates chan iface.RASenderSpec
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
}

// NewSender starts a Router Advertisement sender for spec. It returns an error
// when the interface cannot be resolved or the raw socket cannot be opened. The
// interface component logs that and treats the unit as not advertising, rather
// than failing the whole config apply.
func NewSender(spec iface.RASenderSpec, log *slog.Logger) (*Sender, error) {
	binding, err := iface.Resolve(spec.Interface)
	if err != nil {
		return nil, fmt.Errorf("iface-ra: resolve %q: %w", spec.Interface, err)
	}

	conn, pc, err := openRASocket(binding, log)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Sender{
		spec:    spec,
		log:     log,
		updates: make(chan iface.RASenderSpec, 1),
		cancel:  cancel,
		done:    make(chan struct{}),
	}

	solicitations := make(chan struct{}, 1)
	go readSolicitations(ctx, pc, solicitations)
	go s.run(ctx, conn, pc, binding, solicitations)
	return s, nil
}

// Update hands the send loop a new spec without stopping it, so a lease renew
// never sends Stop's zero Router Lifetime. The interface component is the one
// caller, under its RA lock, so after the drain the slot is free and the send
// never blocks, even once the loop has ended.
func (s *Sender) Update(spec iface.RASenderSpec) {
	select {
	case <-s.updates:
	default:
	}
	select {
	case s.updates <- spec:
	default:
		s.log.Warn("iface-ra: advertisement update dropped, the update slot was taken", "iface", s.spec.Interface)
	}
}

// Stop ends the sender and waits for its goroutine to finish, which happens
// after the final zero-lifetime advertisements leave the socket.
func (s *Sender) Stop() {
	s.once.Do(func() {
		s.cancel()
		<-s.done
	})
}

// openRASocket opens the raw ICMPv6 socket, binds it to the resolved device,
// sets the hop limit RFC 4861 requires, joins the all-routers group, and
// filters everything except Router Solicitations.
func openRASocket(binding iface.Binding, log *slog.Logger) (net.PacketConn, *ipv6.PacketConn, error) {
	conn, err := (&net.ListenConfig{}).ListenPacket(context.Background(), "ip6:ipv6-icmp", "::")
	if err != nil {
		return nil, nil, fmt.Errorf("iface-ra: open ICMPv6 socket: %w", err)
	}
	pc := ipv6.NewPacketConn(conn)

	fail := func(err error) (net.PacketConn, *ipv6.PacketConn, error) {
		if cerr := conn.Close(); cerr != nil {
			log.Warn("iface-ra: socket close failed", "error", cerr)
		}
		return nil, nil, err
	}

	if err := bindToDevice(conn, binding.OsName); err != nil {
		return fail(fmt.Errorf("iface-ra: bind to %q: %w", binding.OsName, err))
	}

	// RFC 4861 Section 4.2: an advertisement is sent with Hop Limit 255, and
	// Section 6.1.2 makes a receiver drop anything else. Setting this is not
	// optional: the kernel default multicast hop limit is 1, so an
	// advertisement sent without it is discarded by every conforming host.
	if err := pc.SetMulticastHopLimit(ndp.MessageHopLimit); err != nil {
		return fail(fmt.Errorf("iface-ra: set multicast hop limit: %w", err))
	}
	if err := pc.SetHopLimit(ndp.MessageHopLimit); err != nil {
		return fail(fmt.Errorf("iface-ra: set hop limit: %w", err))
	}

	// The multicast join and the send both address the device by index, which
	// the resolver already read. Building the struct here keeps every
	// interface-name lookup inside iface.Resolve.
	dev := &net.Interface{Index: binding.Ifindex, Name: binding.OsName}

	// RFC 4861 Section 6.1.1: a router joins the all-routers multicast address
	// so it receives Router Solicitations.
	if err := pc.JoinGroup(dev, &net.IPAddr{IP: net.ParseIP(allRoutersGroup)}); err != nil {
		return fail(fmt.Errorf("iface-ra: join %s on %s: %w", allRoutersGroup, binding.OsName, err))
	}

	var filter ipv6.ICMPFilter
	filter.SetAll(true)
	filter.Accept(ipv6.ICMPTypeRouterSolicitation)
	if err := pc.SetICMPFilter(&filter); err != nil {
		// The filter is an optimization. The read loop uses only the arrival
		// of a message, never its content, so a socket without the filter
		// still behaves correctly, at a higher cost.
		log.Warn("iface-ra: ICMP6_FILTER not set, reading every ICMPv6 message",
			"iface", binding.OsName, "error", err)
	}
	return conn, pc, nil
}

// bindToDevice ties the socket to one device, so an advertisement built for
// this unit cannot leave through another interface.
func bindToDevice(conn net.PacketConn, device string) error {
	ipConn, ok := conn.(*net.IPConn)
	if !ok {
		return errors.New("not an IP connection")
	}
	rawConn, err := ipConn.SyscallConn()
	if err != nil {
		return err
	}
	var setErr error
	if err := rawConn.Control(func(fd uintptr) {
		setErr = unix.SetsockoptString(int(fd), unix.SOL_SOCKET, unix.SO_BINDTODEVICE, device)
	}); err != nil {
		return err
	}
	return setErr
}

// readSolicitations reports each Router Solicitation on ch. The channel holds
// one entry: while an answer is pending the send loop drains it, so a flood of
// solicitations costs one answer rather than one send each. The loop uses only
// the fact that a message arrived, never its content, so a malformed or
// oversized solicitation cannot drive anything.
func readSolicitations(ctx context.Context, pc *ipv6.PacketConn, ch chan<- struct{}) {
	buf := make([]byte, rsReadBufferSize)
	for {
		if ctx.Err() != nil {
			return
		}
		if _, _, _, err := pc.ReadFrom(buf); err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		select {
		case ch <- struct{}{}:
		case <-ctx.Done():
			return
		}
	}
}

// senderState is the mutable state of one send loop, held in one struct so the
// closures in run share it rather than capturing five separate variables.
type senderState struct {
	sent             int
	lastSent         time.Time
	scheduled        time.Time
	linkDown         bool
	solicitedPending bool
}

// run owns the socket and every timer for one sender.
func (s *Sender) run(ctx context.Context, conn net.PacketConn, pc *ipv6.PacketConn, binding iface.Binding, solicitations <-chan struct{}) {
	defer close(s.done)
	defer func() {
		if err := conn.Close(); err != nil {
			s.log.Warn("iface-ra: socket close failed", "iface", s.spec.Interface, "error", err)
		}
	}()

	dst := &net.UDPAddr{IP: net.ParseIP(allNodesGroup), Zone: binding.OsName}
	control := &ipv6.ControlMessage{IfIndex: binding.Ifindex, HopLimit: ndp.MessageHopLimit}

	adv := &advertiser{spec: s.spec}
	// RFC 4861 Section 4.6.1: the advertisement carries the sending
	// interface's link-layer address, which saves each receiver a neighbor
	// solicitation. The resolver already read it, so no second lookup happens.
	sourceMAC := parseMAC(binding.OperMAC)

	// The buffer grows when an update adds options: control-plane rate, at
	// most once per larger advertisement.
	var buf []byte
	//nolint:gosec // the seed drives timer jitter, never a security decision
	random := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(binding.Ifindex)))

	links, unsubscribe := iface.Subscribe(s.spec.Interface)
	defer unsubscribe()

	// A CE-role unit follows the WAN default route (RFC 7084 G-4). The watch
	// starts with the first spec that names a WAN and lasts the sender's life.
	defaults := newWANDefaults()
	wans := newWANResolver(s.spec.Interface, s.log)
	routeChanges := make(chan struct{}, 1)
	var unwatch func()
	defer func() {
		if unwatch != nil {
			unwatch()
		}
	}()
	watchWAN := func() {
		if unwatch != nil {
			return
		}
		if len(adv.spec.WANs) == 0 {
			return
		}
		unwatch = s.watchDefaultRoutes(defaults, routeChanges)
	}
	watchWAN()
	build := func() ndp.RAConfig {
		cfg := adv.build(time.Now(), wans.holdsDefault(adv.spec.WANs, defaults))
		cfg.SourceLinkLayerAddress = sourceMAC
		return cfg
	}

	state := &senderState{scheduled: time.Now()}
	timer := time.NewTimer(0)
	defer timer.Stop()

	send := func(cfg ndp.RAConfig, solicited bool) {
		if need := ndp.RALen(cfg); need > len(buf) {
			buf = make([]byte, need)
		}
		n := ndp.BuildRA(buf, 0, cfg)
		if n == 0 {
			// The next advertisement carries the live state alone, so a
			// withdrawal that cannot be encoded never silences the unit.
			s.log.Error("iface-ra: advertisement could not be encoded, nothing sent; its withdrawals are dropped",
				"iface", s.spec.Interface, "need", ndp.RALen(cfg), "have", len(buf),
				"prefixes", len(cfg.Prefixes), "rdnss", len(cfg.RDNSS), "rdnss-withdrawn", len(cfg.RDNSSWithdrawn))
			s.logDroppedWithdrawals(adv.abandon())
			return
		}
		if _, err := pc.WriteTo(buf[:n], control, dst); err != nil {
			s.log.Debug("iface-ra: send failed", "iface", s.spec.Interface, "error", err)
			return
		}
		adv.sent(cfg)
		state.sent++
		state.lastSent = time.Now()
		incSent(s.spec.Interface)
		if solicited {
			incSolicited(s.spec.Interface)
		}
	}

	// rearm moves the next advertisement to at. Since Go 1.23 a timer channel
	// delivers nothing after Stop, so Reset needs no drain.
	rearm := func(at time.Time) {
		state.scheduled = at
		timer.Stop()
		timer.Reset(time.Until(at))
	}

	// hurry brings the next advertisement forward when the content changed in
	// a way hosts must hear at once (advertiser.urgent), within the rate limit
	// sendFinal also keeps (ndp.CeaseWait).
	hurry := func() {
		if state.linkDown {
			return
		}
		now := time.Now()
		if !adv.urgent(now, wans.holdsDefault(adv.spec.WANs, defaults)) {
			return
		}
		at := now.Add(ndp.CeaseWait(state.lastSent, now))
		if at.Before(state.scheduled) {
			rearm(at)
		}
	}

	// The loop runs until ctx ends: one iteration per event, timer or update.
	for {
		select {
		case <-ctx.Done():
			s.sendFinal(send, build(), state.linkDown, state.lastSent)
			return

		case spec := <-s.updates:
			s.logDroppedWithdrawals(adv.update(spec))
			watchWAN()
			hurry()

		case <-routeChanges:
			hurry()

		case ev := <-links:
			s.onLinkEvent(ev, state, rearm)

		case <-solicitations:
			if state.linkDown {
				continue
			}
			// RFC 4861 Section 6.2.6: "If a single advertisement is sent
			// in response to multiple solicitations, the delay is
			// relative to the first solicitation." An answer is already
			// scheduled, so this solicitation gets that one and draws no
			// second delay of its own.
			if state.solicitedPending {
				continue
			}
			at := ndp.SolicitedSendTime(state.lastSent, time.Now(), ndp.SolicitedDelay(random))
			// RFC 4861 Section 6.2.6: when the answer would land after the
			// advertisement already scheduled, that one answers the
			// solicitation and no extra message is sent.
			if at.Before(state.scheduled) {
				state.solicitedPending = true
				rearm(at)
			}

		case <-timer.C:
			if state.linkDown {
				// Nothing leaves a down link. The next up event rearms the
				// timer and restarts the initial burst.
				continue
			}
			solicited := state.solicitedPending
			state.solicitedPending = false
			send(build(), solicited)
			rearm(time.Now().Add(ndp.UnsolicitedInterval(adv.spec.MinimumInterval, adv.spec.MaximumInterval, state.sent, random)))
			// Withdrawals beyond what one advertisement carries go in the
			// next, at the rate limit rather than the periodic schedule.
			hurry()
		}
	}
}

// logDroppedWithdrawals logs, at WARN, each delegated /64 and each resolver
// whose withdrawal the advertiser dropped: hosts on the link keep it until the
// lifetime last advertised for it runs out.
func (s *Sender) logDroppedWithdrawals(dropped droppedWithdrawals) {
	for _, d := range dropped.delegated {
		s.log.Warn("iface-ra: delegated prefix withdrawal dropped, hosts keep addresses from it until its advertised valid lifetime ends",
			"iface", s.spec.Interface, "prefix", d.Address.Masked(), "queued-max", retiringDelegatedMax)
	}
	for _, addr := range dropped.rdnss {
		s.log.Warn("iface-ra: resolver withdrawal dropped, hosts keep it until its advertised RDNSS lifetime ends",
			"iface", s.spec.Interface, "address", addr, "queued-max", retiringRDNSSMax)
	}
}

// onLinkEvent pauses the sender while the link is down and restarts the initial
// burst when it comes back.
func (s *Sender) onLinkEvent(ev iface.LinkEvent, state *senderState, rearm func(time.Time)) {
	switch ev.Kind {
	case iface.LinkDown:
		state.linkDown = true
		s.log.Info("iface-ra: link down, advertisements paused", "iface", s.spec.Interface)
	case iface.LinkUp, iface.LinkAppeared:
		if !state.linkDown {
			return
		}
		state.linkDown = false
		// RFC 4861 Section 6.2.4: an interface that becomes an advertising
		// interface starts a new initial burst, so hosts find the router
		// quickly after a flap.
		state.sent = 0
		state.lastSent = time.Time{}
		s.log.Info("iface-ra: link up, advertisements resumed", "iface", s.spec.Interface)
		rearm(time.Now())
	default:
		// Link events are open inputs; an unknown kind must not stop the sender.
		s.log.Warn("iface-ra: unhandled link event",
			"iface", s.spec.Interface, "kind", string(ev.Kind))
	}
}

// sendFinal sends the advertisement that retires this router.
//
// RFC 4861 Section 6.2.5: an interface that stops advertising "SHOULD transmit
// one or more (but not more than MAX_FINAL_RTR_ADVERTISEMENTS) final multicast
// Router Advertisements ... with a Router Lifetime field of zero", so hosts
// drop it from their default router list at once instead of waiting the
// lifetime out. Ze sends ONE, immediately. Nothing is sent while the link is
// down, because nothing would leave the interface.
//
// One rather than three, decided 2026-09-05. The three that were sent before
// bought nothing: they left in one scheduler tick, so a receiver could not tell
// them apart and a link that dropped one dropped all three. radvd
// (stop_adverts), FRR zebra (rtadv_stop_ra) and BIRD (radv_iface_shutdown) each
// send exactly one, so one is also what every implementation a Ze link meets
// will send.
//
// The wait is ndp.CeaseWait, the same function the PPP subscriber sender calls
// through raSchedule.ceaseWait. Section 6.2.6 rate limits consecutive multicast
// advertisements to one every MIN_DELAY_BETWEEN_RAS, and whether that reaches a
// final advertisement is arguable, so Ze takes the reading that cannot be wrong
// in BOTH senders rather than one reading in each. It is zero in steady state
// and at most MIN_DELAY_BETWEEN_RAS when the interface stops just after it
// advertised. Sleeping here is bounded and holds only this sender's shutdown.
func (s *Sender) sendFinal(send func(ndp.RAConfig, bool), advertisement ndp.RAConfig, linkDown bool, lastSent time.Time) {
	if linkDown {
		return
	}
	if wait := ndp.CeaseWait(lastSent, time.Now()); wait > 0 {
		time.Sleep(wait)
	}
	send(zeroLifetime(advertisement), false)
}

// zeroLifetime returns the advertisement with a Router Lifetime of zero. RFC
// 4861 Section 4.2 scopes that field to the router's usefulness as a default
// router, so every other field stays as configured and hosts read one
// consistent message.
func zeroLifetime(cfg ndp.RAConfig) ndp.RAConfig {
	final := cfg
	final.RouterLifetime = 0
	return final
}

// parseMAC turns the resolver's textual hardware address into the octets the
// Source Link-layer Address option carries. No address, or an unparsable one,
// yields nil and the option is left out, which RFC 4861 Section 4.2 allows.
// An address that is not six octets is dropped by the encoder for the same
// reason: it does not fill one 8-octet option unit.
func parseMAC(mac string) []byte {
	if mac == "" {
		return nil
	}
	hw, err := net.ParseMAC(mac)
	if err != nil {
		return nil
	}
	return hw
}

// watchDefaultRoutes follows the main table's IPv6 default routes through the
// shared route watcher, signals each change on changed, and returns the
// function that ends the watch. The watcher lists existing routes only when
// it subscribes, and the kernel or fib-kernel plugin may have started it long
// before, so the routes present now are read once from the kernel.
func (s *Sender) watchDefaultRoutes(defaults *wanDefaults, changed chan<- struct{}) func() {
	notify := func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}
	w := routewatch.Global()
	unregister := w.Register(func(ev routewatch.RouteEvent) {
		if defaults.observe(ev) {
			notify()
		}
	})
	// Start runs once per process; a later call is a no-op.
	w.Start(func(err error) {
		s.log.Warn("iface-ra: route watcher error, the WAN default route state may be stale",
			"iface", s.spec.Interface, "error", err)
	})
	existing, err := w.ListRoutes(netip.PrefixFrom(netip.IPv6Unspecified(), 0))
	if err != nil {
		// Unknown is read as no default route: the Router Lifetime stays 0,
		// the answer G-4 gives, until a route event says otherwise.
		s.log.Warn("iface-ra: existing IPv6 default routes not read, Router Lifetime stays 0 until a default route event",
			"iface", s.spec.Interface, "error", err)
	}
	defaults.seed(existing)
	notify()
	return unregister
}

// wanResolver answers whether a default route leaves through the device of a
// WAN the unit names, and says so at WARN when it cannot look. A WAN that does
// not resolve holds no route, and the unit then advertises Router Lifetime 0
// (RFC 7084 G-4); that is right while a PPP session is down, and wrong when
// the name is mistyped or the backend cannot read the device, so the operator
// is told which it is rather than left with a LAN that has no default router.
// One WARN is logged when a WAN starts failing, or fails with another error,
// and one INFO when it resolves again, so the periodic sends do not flood the
// log. Bounded by the WANs the unit names. Not safe for concurrent use; the
// send loop is its only caller.
type wanResolver struct {
	lan string
	log *slog.Logger
	// failing holds the error last reported for each WAN that does not resolve.
	failing map[string]string
}

func newWANResolver(lan string, log *slog.Logger) *wanResolver {
	return &wanResolver{lan: lan, log: log, failing: make(map[string]string)}
}

// holdsDefault reports whether a default route leaves through the device of
// any WAN in wans.
func (r *wanResolver) holdsDefault(wans []string, defaults *wanDefaults) bool {
	held := false
	for _, name := range wans {
		binding, err := iface.Resolve(name)
		if err != nil {
			r.failed(name, err)
			continue
		}
		r.resolved(name)
		if defaults.holds(binding.Ifindex) {
			held = true
		}
	}
	return held
}

// failed reports a WAN that did not resolve, once for each distinct error.
func (r *wanResolver) failed(name string, err error) {
	reason := err.Error()
	if r.failing[name] == reason {
		return
	}
	r.failing[name] = reason
	r.log.Warn("iface-ra: WAN interface not resolved, so this LAN advertises Router Lifetime 0 (RFC 7084 G-4) until it resolves;"+
		" check the WAN session and its name with `show interface`",
		"iface", r.lan, "wan", name, "error", err)
}

// resolved records that a WAN resolves, and says so when it had failed.
func (r *wanResolver) resolved(name string) {
	if _, was := r.failing[name]; !was {
		return
	}
	delete(r.failing, name)
	r.log.Info("iface-ra: WAN interface resolved, its default route counts again",
		"iface", r.lan, "wan", name)
}
