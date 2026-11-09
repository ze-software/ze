// Design: docs/architecture/provisioning/dhcp-server.md -- DHCP server plugin registration

package dhcpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/taskguard"
	dhcpyang "github.com/ze-software/ze/internal/plugins/dhcpserver/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootService = "service"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	// StartAfter: the server binds each listen-interface in its Stage 2
	// configure, and the interface plugin creates the VLANs, bridges and dummies
	// of ze's own `interface {}` config in its own Stage 2 configure, so the
	// server starts in a later tier whenever both run. It is an order, not a
	// dependency: a config with no `interface` section does not load interface.
	reg := registry.Registration{
		Name:                    "dhcpserver",
		Description:             "DHCP server: address assignment for LAN clients (RFC 2131)",
		Features:                "yang",
		YANG:                    dhcpyang.ZeDHCPServerConfYANG,
		ConfigRoots:             []string{configRootService},
		StartAfter:              []string{"interface"},
		InProcessConfigVerifier: verifyDHCPConfig,
		RunEngine:               runDHCPServerPlugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		l := slogutil.Logger(loggerName)
		if l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureMetrics = func(metricsRegistry metrics.Registry) { dhcpGuard.SetMetricsRegistry(metricsRegistry) }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "dhcpserver: registration failed: %v\n", err)
		os.Exit(1)
	}

	// The server binds a socket on every listen-interface, so this plugin owns
	// the readiness check that asks whether each one exists (doctor.go). A
	// refusal is a programmer error in the declaration beside it.
	if err := diagnostic.RegisterDoctorCheck(dhcpDoctorCheck); err != nil {
		fmt.Fprintf(os.Stderr, "dhcpserver: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyDHCPConfig(sections []sdk.ConfigSection) error {
	_, _, err := parseDHCPSections(sections)
	return err
}

func runDHCPServerPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("dhcpserver plugin starting")

	p := sdk.NewWithConn("dhcpserver", conn)
	defer closeLogged(p, log, "plugin conn")

	// Each listen-interface is resolved to its kernel device through
	// iface.ResolveDevice, whose os-name and mac match selector table the
	// interface plugin fills in the daemon's own process. A forked copy holds an
	// empty table, so ResolveDevice would pass every logical name through and
	// the socket would bind whatever device wears that name, with no error. The
	// model is VRRP's guard (internal/plugins/vrrp/register.go, runVRRPEngine).
	if !p.IsInternal() {
		log.Error("dhcpserver: refusing to start as an external plugin process. " +
			"It resolves each listen-interface to its kernel device through the interface plugin's selector table (iface.ResolveDevice), " +
			"which exists only inside the daemon's process, so an external copy would bind its sockets to whatever device wears the logical name.\n" +
			"Fix: remove the external dhcpserver plugin block so the plugin runs as `internal dhcpserver`")
		return 1
	}

	srv := &dhcpServer{log: log}
	tx := sdk.NewConfigTx(serverConfig{}, parseDHCPSections, srv.install)
	tx.Register(p)
	p.OnConfigure(func(sections []sdk.ConfigSection) error { return srv.configureAtBoot(tx, sections) })
	followLearnedResolvers(p, &srv.learned, &srv.publisher, log)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRootService},
		VerifyBudget: 2,
		ApplyBudget:  5,
	}); err != nil {
		log.Error("dhcpserver plugin failed", "error", err)
		srv.stop()
		return 1
	}

	srv.stop()
	log.Info("dhcpserver plugin stopped")
	return 0
}

// followLearnedResolvers keeps learned on the newest system-dns snapshot, so
// each reply's option 6 reads the current learned resolvers. The startup
// subscription and the snapshot request at OnStarted give a plugin that
// starts after the leases the records already held (AC-13). Replies evaluate
// the list when they are built; a fresh snapshot makes publisher publish each
// subnet's selection again for show system dns and ze doctor.
func followLearnedResolvers(p *sdk.Plugin, learned *dnssource.Follower, publisher *dhcpPublisher, log *slog.Logger) {
	p.SetStartupSubscriptionsIn(dnssource.Namespace, []string{dnssource.SnapshotEventType}, nil, "")
	p.OnEvent(func(event string) error {
		fresh, err := learned.Accept(event)
		if err != nil {
			log.Warn("dhcpserver: learned resolvers not applied; option 6 keeps the previous ones", "error", err)
			return fmt.Errorf("dhcpserver: learned resolvers not applied: %w", err)
		}
		if fresh {
			publisher.refresh()
		}
		return nil
	})
	p.OnStarted(func(ctx context.Context) error {
		if _, err := p.EmitEvent(ctx, dnssource.Namespace, dnssource.SnapshotRequestEventType, "", "", ""); err != nil {
			log.Warn("dhcpserver: learned resolvers not requested; they apply at the next lease change", "error", err)
		}
		// The emit outlives this callback, so it does not take its
		// cancellation.
		emitCtx := context.WithoutCancel(ctx)
		publisher.setSender(func(body string) {
			if _, err := p.EmitEvent(emitCtx, dnssource.Namespace, dnssource.PublicationEventType, "", "", body); err != nil {
				log.Warn("dhcpserver: option 6 selections not published; show system dns shows the previous ones", "error", err)
			}
		})
		return nil
	})
}

type closer interface {
	Close() error
}

func closeLogged(c closer, log *slog.Logger, what string) {
	if err := c.Close(); err != nil {
		log.Debug("dhcpserver: close failed", "what", what, "error", err)
	}
}

// dhcpListen is the per-interface listener factory (a var so tests can stub the
// bind without privileges or a real interface).
var dhcpListen = listenDHCP

// dhcpServe starts the request loop on a bound listener. It is a variable so a
// test can see which device each listener is served on; nothing else assigns it.
var dhcpServe = serveMulti

// dhcpResolveDevice answers the kernel device a configured listen-interface
// names. It is a variable so a test can stand in a selector that matches no
// device; nothing else assigns it.
var dhcpResolveDevice = iface.ResolveDevice

// errListenSelector marks a listen-interface whose os-name or mac match
// selector answered no single device, so the reload refusal names the selector
// rather than the listen-interface leaf.
var errListenSelector = errors.New("the interface's os-name or mac match selector names no single present device")

// dhcpListener is one bound listener and the interface it serves, which a
// refused reload binds again. iface is the name the operator configured;
// device is the kernel device iface.ResolveDevice answered for it, the one the
// socket is bound to and a reply's neighbor entry is written on.
type dhcpListener struct {
	iface  string
	device string
	conn   *net.UDPConn
}

// listenInterface binds the listener for the configured interface name. The
// name is a logical interface, which an os-name or mac match selector binds to
// another kernel device, so it is resolved before SO_BINDTODEVICE sees it, the
// way VRRP resolves each group's parent (internal/plugins/vrrp/groups.go,
// parentDevice). A resolution that fails is refused and never falls back to the
// logical name: that name may belong to another device.
func listenInterface(name string) (dhcpListener, error) {
	device, err := dhcpResolveDevice(name)
	if err != nil {
		return dhcpListener{}, fmt.Errorf("%w: %w", errListenSelector, err)
	}
	// An empty device would be an unbound socket listening on every interface.
	if device == "" {
		return dhcpListener{}, fmt.Errorf("%w: interface %q resolved to no device name", errListenSelector, name)
	}
	conn, err := dhcpListen(device)
	if err != nil {
		return dhcpListener{}, err
	}
	return dhcpListener{iface: name, device: device, conn: conn}, nil
}

// bindListeners binds a DHCP listener on every interface of cfg and starts
// serving handlers on each. A disabled cfg binds nothing.
//
// strict is the reload mode: it binds every interface or none, and when one
// cannot be bound it closes the listeners already bound and returns an error
// naming the interface and the fix, so a reload never reports a server that
// serves part of its interfaces, or none, and the transaction rolls back to
// the config that served. Boot is not strict: it has no previous config to
// keep, and a configure error at boot fails the plugin and aborts the whole
// daemon start, so a failed interface is logged and the others serve. A
// device ze's own `interface {}` config creates exists by then, because the
// server starts after the interface plugin (StartAfter in init); a device
// that appears later is served only after the next reload.
func bindListeners(cfg serverConfig, handlers []*dhcpHandler, log *slog.Logger, strict bool) ([]dhcpListener, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	var listeners []dhcpListener
	for _, name := range cfg.ListenInterfaces {
		ln, err := listenInterface(name)
		if err != nil && !strict {
			log.Error("dhcpserver: listen failed", "interface", name, "error", err)
			continue
		}
		if err != nil {
			for _, bound := range listeners {
				closeLogged(bound.conn, log, "udp listener")
			}
			if errors.Is(err, errListenSelector) {
				return nil, fmt.Errorf("dhcpserver: cannot listen on interface %q: %w. "+
					"The DHCP server does not take this config.\n"+
					"Fix: make the os-name or mac match selector of interface %q name exactly one present device, "+
					"or remove %q from service dhcp-server listen-interface", name, err, name, name)
			}
			return nil, fmt.Errorf("dhcpserver: cannot listen on interface %q: %w. "+
				"The DHCP server does not take this config.\n"+
				"Fix: name an interface that exists in service dhcp-server listen-interface, or remove %q from it", name, err, name)
		}
		listeners = append(listeners, ln)
	}
	if len(listeners) == 0 {
		log.Error("dhcpserver: no interfaces bound; server not serving", "interfaces", cfg.ListenInterfaces)
		return nil, nil
	}
	for _, ln := range listeners {
		go dhcpServe(ln.conn, ln.device, handlers, log)
	}
	log.Info("dhcpserver: started",
		"shared-networks", len(cfg.SharedNetworks),
		"interfaces", cfg.ListenInterfaces,
		"listeners", len(listeners))
	return listeners, nil
}

// serveMulti answers the requests conn receives on the kernel device named device,
// which is the interface a reply to a client with no address is delivered on.
func serveMulti(conn *net.UDPConn, device string, handlers []*dhcpHandler, log *slog.Logger) {
	server := &requestServer{device: device, handlers: handlers, log: log}
	buf := make([]byte, 1500)
	for serveRequests(conn, server, buf, log) {
		// The pass ended on a contained panic; the next one reads on.
	}
}

// dhcpDeviceAddrs answers the IPv4 addresses the kernel device holds, primary
// addresses first (readDeviceAddrs). It is read for every request: for one no
// relay forwarded it names the segment, and for every request it names the
// server identifier (segment.serverID), so an address added to or removed from the listen interface
// is seen by the next request, with no reload and no cached copy to go stale.
// It is a variable so a test can stand in a device's addresses; nothing else
// assigns it.
var dhcpDeviceAddrs = readDeviceAddrs

// unservedWarnInterval is the least time between two warnings of one listener
// that a request found no segment to serve it, so a client retransmitting, or
// a flood, writes one line a minute rather than one per packet.
const unservedWarnInterval = time.Minute

// requestServer answers the requests one listener receives on device. It is
// owned by the listener's one request task, which serveMulti runs, and is not
// safe for concurrent use.
type requestServer struct {
	device   string
	handlers []*dhcpHandler
	log      *slog.Logger
	// warned is when the last unserved warning was written (unservedWarnInterval).
	warned time.Time
	// warnedWithheld is when the last warning that a segment withheld a reply
	// was written (segment.withheld), limited the same way.
	warnedWithheld time.Time
}

// answer returns the reply to pkt, or nil when the server stays silent.
// resolved is false when pkt was dropped before a segment was chosen
// (resolve). It logs no exchange: serveRequests does, after the reply.
func (r *requestServer) answer(pkt []byte) (reply []byte, resolved bool) {
	seg, ok := r.resolve(pkt)
	if !ok {
		return nil, false
	}
	return seg.serve(pkt), true
}

// resolve returns the segment that serves pkt. resolved is false when pkt was
// dropped before a segment was chosen: it is malformed, the receiving device's
// addresses could not be read, or no configured subnet holds the relay's
// address or any address of the receiving device. resolve has then logged
// why, when the reason is one an operator fixes.
func (r *requestServer) resolve(pkt []byte) (seg segment, resolved bool) {
	if !validRequest(pkt) {
		return segment{}, false
	}
	giaddr := netip.AddrFrom4([4]byte(pkt[24:28]))
	device, err := dhcpDeviceAddrs(r.device)
	if err != nil {
		r.warnUnserved("dhcpserver: no reply, the listen interface's addresses could not be read. "+
			"The server serves the subnets the interface is addressed in and identifies itself "+
			"with one of its addresses, so it cannot answer.",
			"interface", r.device, "error", err)
		return segment{}, false
	}
	seg, ok := resolveSegment(r.handlers, giaddr, device)
	if ok {
		seg.warn = r.warnWithheld
		return seg, true
	}
	if giaddr.IsUnspecified() {
		r.warnUnserved("dhcpserver: no reply, the listen interface holds no address in any configured subnet. "+
			"The server answers a client that no relay forwarded only from the subnets of the interface's own segment.\n"+
			"Fix: add an address in a configured subnet to the interface, "+
			"or remove the interface from service dhcp-server listen-interface",
			"interface", r.device, "addresses", device)
		return segment{}, false
	}
	r.warnUnserved("dhcpserver: no reply to a relayed request, no configured subnet holds the relay's address (giaddr).\n"+
		"Fix: add a subnet that holds the relay's address to a shared-network of service dhcp-server",
		"interface", r.device, "giaddr", giaddr.String())
	return segment{}, false
}

// warnUnserved writes a warning that no segment serves a request, at most
// once per unservedWarnInterval.
func (r *requestServer) warnUnserved(msg string, attrs ...any) {
	warnLimited(r.log, &r.warned, msg, attrs...)
}

// warnWithheld writes a warning that a segment withheld a reply
// (segment.withheld), at most once per unservedWarnInterval.
func (r *requestServer) warnWithheld(msg string, attrs ...any) {
	warnLimited(r.log, &r.warnedWithheld, msg, attrs...)
}

// warnLimited writes msg at WARN unless the last warning recorded in last is
// younger than unservedWarnInterval.
func warnLimited(log *slog.Logger, last *time.Time, msg string, attrs ...any) {
	now := time.Now()
	if !last.IsZero() && now.Sub(*last) < unservedWarnInterval {
		return
	}
	*last = now
	log.Warn(msg, attrs...)
}

// serveRequests is the request task: it answers requests until the socket
// closes, answering false, or until a panic ends the pass, answering true. The
// request being handled then gets no reply and its client retransmits. The pool
// and lease tables hold their mutexes only under defer, so the unwind leaves
// them free.
func serveRequests(conn *net.UDPConn, server *requestServer, buf []byte, log *slog.Logger) (again bool) {
	var source *net.UDPAddr // the sender of the request being handled
	describe := func() taskguard.Context {
		return taskguard.Context{Unit: "request from " + source.String()}
	}
	// RFC 2131 Section 4.1: "DHCP clients are responsible for all message
	// retransmission." A request left unanswered is sent again by its client.
	defer requestTask.Contain(describe, func(*taskguard.Panic) { again = true })

	for {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			return false
		}
		source = from
		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		seg, resolved := server.resolve(pkt)
		if !resolved {
			continue
		}
		resp := seg.serve(pkt)
		logExchange(log, pkt, resp, seg.serverIDs())
		if resp == nil {
			continue
		}

		// RFC 2131 Section 4.1
		dst := replyDestination(server.device, pkt, resp, log)
		if _, writeErr := conn.WriteToUDP(resp, dst); writeErr != nil {
			log.Debug("dhcpserver: write failed", "dst", dst.String(), "error", writeErr)
		}
	}
}

// logExchange surfaces each DHCP request/reply at info so a provisioning
// operator can watch address assignment and PXE bootfile selection live.
// Errors and unanswered control packets stay quieter (debug/info).
func logExchange(log *slog.Logger, req, resp []byte, serverIPs []netip.Addr) {
	if len(req) < minPacketLen {
		return
	}
	reqType := parseMsgType(req)
	mac := extractMAC(req)

	if resp == nil {
		switch reqType {
		case msgRequest:
			// A REQUEST goes unanswered only when it is not addressed to us:
			// either the client selected a different DHCP server (its option 54
			// server-id is not ours) or its requested address is outside our
			// subnet. Surfacing the server-id makes a competing DHCP server on
			// the provisioning segment diagnosable from this log alone -- a
			// booted kernel's ip=dhcp racing a second server is the classic
			// cause of an install that PXE-boots but then cannot reach us.
			sid := parseOptionAddr(req, optServerID)
			if sid.IsValid() && !slices.Contains(serverIPs, sid) {
				reqIP := parseOptionAddr(req, optRequestedIP)
				log.Info("dhcpserver: no reply to REQUEST (client selected another DHCP server)",
					"mac", mac.String(),
					"selected-server-id", sid.String(),
					"our-server-ids", serverIPs,
					"requested-ip", reqIP.String())
			} else {
				log.Info("dhcpserver: no reply to REQUEST (not for our subnet)",
					"mac", mac.String())
			}
		case msgDiscover:
			log.Info("dhcpserver: no reply to DISCOVER (no free address)",
				"mac", mac.String())
		default:
			log.Debug("dhcpserver: received", "request", msgTypeName(reqType), "mac", mac.String())
		}
		return
	}

	respType := parseMsgType(resp)
	attrs := []any{"request", msgTypeName(reqType), "reply", msgTypeName(respType), "mac", mac.String()}
	if respType != msgNak {
		yiaddr := netip.AddrFrom4([4]byte(resp[16:20]))
		attrs = append(attrs, "ip", yiaddr.String())
	}
	if bf := parseOptionString(resp, optBootfileName); bf != "" {
		attrs = append(attrs, "bootfile", bf)
	}
	log.Info("dhcpserver: lease", attrs...)
}

// neighborInstall writes the neighbor entry a hardware unicast needs (a var so
// tests can observe the call and model a refusal without CAP_NET_ADMIN).
var neighborInstall = installNeighbor

// neighborRead reads the entry an install would replace (a var so tests can
// model a held entry without a kernel table).
var neighborRead = readNeighbor

// neighborEntry is what neighborRead found for one address. held is true for
// a PERMANENT or NOARP entry, which ze never replaces (neighborHeld); held
// false means no entry, or one ARP learned or ze wrote, which ze may replace.
// hardware is set only when held is true.
type neighborEntry struct {
	held     bool
	hardware net.HardwareAddr
}

// replyDestination answers where resp goes, and first installs the neighbor
// entry a reply to a client with no address needs. Without it the kernel ARPs
// for yiaddr, which the client does not hold yet, the neighbor ends FAILED and
// the reply never leaves.
//
// A held entry for yiaddr (PERMANENT or NOARP) is never replaced: when it names
// chaddr the unicast already reaches the client and nothing is written; when
// it names another host a unicast would reach that host, so the reply is
// broadcast. A table that cannot be read is broadcast too, because the write
// could then replace an entry nobody could see.
func replyDestination(device string, req, resp []byte, log *slog.Logger) *net.UDPAddr {
	dst := responseAddr(req, resp)
	if !toClientHardware(req, resp) {
		return dst
	}
	yiaddr := netip.AddrFrom4([4]byte(resp[16:20]))
	chaddr := extractMAC(req)
	entry, err := neighborRead(device, yiaddr)
	if err != nil {
		log.Info("dhcpserver: reply broadcast, the neighbor table could not be read",
			"interface", device, "ip", yiaddr.String(), "mac", chaddr.String(), "error", err)
		return broadcastReply()
	}
	if entry.held {
		if bytes.Equal(entry.hardware, chaddr) {
			return dst
		}
		log.Info("dhcpserver: reply broadcast, a permanent or noarp neighbor entry names another hardware address",
			"interface", device, "ip", yiaddr.String(), "mac", chaddr.String(), "held-mac", entry.hardware.String())
		return broadcastReply()
	}
	// RFC 2131 Section 4.1: "If the BROADCAST bit is cleared to 0, the message
	// SHOULD be sent as an IP unicast to the IP address specified in the
	// 'yiaddr' field and the link-layer address specified in the 'chaddr'
	// field."
	// The kernel refuses a chaddr whose length is not the interface's
	// link-layer address length, and the reply is then broadcast.
	err = neighborInstall(device, yiaddr, chaddr)
	if err == nil {
		return dst
	}
	log.Info("dhcpserver: reply broadcast, the client's hardware address could not be installed",
		"interface", device, "ip", yiaddr.String(), "mac", chaddr.String(), "error", err)
	return broadcastReply()
}

// broadcastReply is the destination of a reply that cannot be unicast to the
// client's hardware address.
//
// RFC 2131 Section 4.1: "If unicasting is not possible, the message MAY be
// sent as an IP broadcast using an IP broadcast address (preferably
// 0xffffffff) as the IP destination address and the link-layer broadcast
// address as the link-layer destination address."
func broadcastReply() *net.UDPAddr {
	return &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
}

// toClientHardware reports whether resp goes to the client's hardware address
// and yiaddr: a DHCPOFFER or DHCPACK answering a request with giaddr, ciaddr
// and the BROADCAST bit all zero, which offers a non-zero yiaddr. It is the
// one statement of that condition, read by responseAddr for the IP destination
// and by replyDestination for the neighbor entry the unicast needs.
func toClientHardware(req, resp []byte) bool {
	giaddr := netip.AddrFrom4([4]byte(req[24:28]))
	if !giaddr.IsUnspecified() {
		return false
	}
	ciaddr := netip.AddrFrom4([4]byte(req[12:16]))
	if !ciaddr.IsUnspecified() {
		return false
	}
	flags := uint16(req[10])<<8 | uint16(req[11])
	if flags&0x8000 != 0 {
		return false
	}
	if parseMsgType(resp) == msgNak {
		return false
	}
	yiaddr := netip.AddrFrom4([4]byte(resp[16:20]))
	return !yiaddr.IsUnspecified()
}

// responseAddr answers the IP destination of resp under the RFC 2131 Section
// 4.1 delivery rules.
func responseAddr(req, resp []byte) *net.UDPAddr {
	// RFC 2131 Section 4.1: "If the 'giaddr' field in a DHCP message from a
	// client is non-zero, the server sends any return messages to the 'DHCP
	// server' port on the BOOTP relay agent whose address appears in
	// 'giaddr'."
	giaddr := netip.AddrFrom4([4]byte(req[24:28]))
	if !giaddr.IsUnspecified() {
		return &net.UDPAddr{IP: giaddr.AsSlice(), Port: 67}
	}

	// RFC 2131 Section 4.1: "In all cases, when 'giaddr' is zero, the server
	// broadcasts any DHCPNAK messages to 0xffffffff."
	if parseMsgType(resp) == msgNak {
		return &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	}

	// RFC 2131 Section 4.1: "If the 'giaddr' field is zero and the 'ciaddr'
	// field is nonzero, then the server unicasts DHCPOFFER and DHCPACK
	// messages to the address in 'ciaddr'."
	ciaddr := netip.AddrFrom4([4]byte(req[12:16]))
	if !ciaddr.IsUnspecified() {
		return &net.UDPAddr{IP: ciaddr.AsSlice(), Port: 68}
	}

	// RFC 2131 Section 4.1: "If the broadcast bit is not set and 'giaddr' is
	// zero and 'ciaddr' is zero, then the server unicasts DHCPOFFER and DHCPACK
	// messages to the client's hardware address and 'yiaddr' address."
	if toClientHardware(req, resp) {
		yiaddr := netip.AddrFrom4([4]byte(resp[16:20]))
		return &net.UDPAddr{IP: yiaddr.AsSlice(), Port: 68}
	}

	// RFC 2131 Section 4.1: "If 'giaddr' is zero and 'ciaddr' is zero, and the
	// broadcast bit is set, then the server broadcasts DHCPOFFER and DHCPACK
	// messages to 0xffffffff."
	return &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
}
