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
	"sync"
	"time"

	"golang.org/x/net/ipv6"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/ndp"
)

// allRoutersGroup is the all-routers multicast address a router joins so it
// receives Router Solicitations (RFC 4861 Section 6.1.1).
const allRoutersGroup = "ff02::2"

// allNodesGroup is where unsolicited and solicited advertisements go
// (RFC 4861 Section 6.2.6).
const allNodesGroup = "ff02::1"

// advertisementHopLimit is the IPv6 Hop Limit every Neighbor Discovery message
// carries. RFC 4861 Section 4.2 states it, and Section 6.1.2 makes a receiver
// discard an advertisement that arrives with any other value. The kernel
// default for multicast is 1, so the sender sets this on the socket rather
// than trusting the default.
const advertisementHopLimit = 255

// rsReadBufferSize bounds one Router Solicitation read. A solicitation is a
// short message and only its arrival matters, so a longer one is truncated
// rather than buffered.
const rsReadBufferSize = 256

// Sender advertises on one interface unit. One goroutine owns the socket and
// every timer, so a stopped Sender holds no socket and no multicast group join.
type Sender struct {
	spec   iface.RASenderSpec
	log    *slog.Logger
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
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
		spec:   spec,
		log:    log,
		cancel: cancel,
		done:   make(chan struct{}),
	}

	solicitations := make(chan struct{}, 1)
	go readSolicitations(ctx, pc, solicitations)
	go s.run(ctx, conn, pc, binding, solicitations)
	return s, nil
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
	if err := pc.SetMulticastHopLimit(advertisementHopLimit); err != nil {
		return fail(fmt.Errorf("iface-ra: set multicast hop limit: %w", err))
	}
	if err := pc.SetHopLimit(advertisementHopLimit); err != nil {
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
	control := &ipv6.ControlMessage{IfIndex: binding.Ifindex, HopLimit: advertisementHopLimit}

	advertisement := s.spec.Advertisement
	// RFC 4861 Section 4.6.1: the advertisement carries the sending
	// interface's link-layer address, which saves each receiver a neighbor
	// solicitation. The resolver already read it, so no second lookup happens.
	advertisement.SourceLinkLayerAddress = parseMAC(binding.OperMAC)

	buf := make([]byte, ndp.RALen(advertisement))
	//nolint:gosec // the seed drives timer jitter, never a security decision
	random := rand.New(rand.NewPCG(uint64(time.Now().UnixNano()), uint64(binding.Ifindex)))

	links, unsubscribe := iface.Subscribe(s.spec.Interface)
	defer unsubscribe()

	state := &senderState{scheduled: time.Now()}
	timer := time.NewTimer(0)
	defer timer.Stop()

	send := func(cfg ndp.RAConfig, solicited bool) {
		n := ndp.BuildRA(buf, 0, cfg)
		if n == 0 {
			s.log.Error("iface-ra: advertisement did not fit its buffer, nothing sent",
				"iface", s.spec.Interface, "need", ndp.RALen(cfg), "have", len(buf))
			return
		}
		if _, err := pc.WriteTo(buf[:n], control, dst); err != nil {
			s.log.Debug("iface-ra: send failed", "iface", s.spec.Interface, "error", err)
			return
		}
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

	for {
		select {
		case <-ctx.Done():
			s.sendFinal(send, advertisement, state.linkDown, state.lastSent)
			return

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
			send(advertisement, solicited)
			rearm(time.Now().Add(ndp.UnsolicitedInterval(s.spec.MinimumInterval, s.spec.MaximumInterval, state.sent, random)))
		}
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
