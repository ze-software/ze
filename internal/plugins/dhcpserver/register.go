// Design: docs/architecture/provisioning/dhcp-server.md -- DHCP server plugin registration

package dhcpserver

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"slices"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	dhcpyang "github.com/ze-software/ze/internal/plugins/dhcpserver/yang"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootService = "service"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:                    "dhcpserver",
		Description:             "DHCP server: address assignment for LAN clients (RFC 2131)",
		Features:                "yang",
		YANG:                    dhcpyang.ZeDHCPServerConfYANG,
		ConfigRoots:             []string{configRootService},
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
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "dhcpserver: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyDHCPConfig(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		if _, err := parseConfig(s.Data); err != nil {
			return fmt.Errorf("dhcpserver: %w", err)
		}
	}
	return nil
}

func runDHCPServerPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("dhcpserver plugin starting")

	p := sdk.NewWithConn("dhcpserver", conn)
	defer closeLogged(p, log, "plugin conn")

	var handlers []*dhcpHandler
	var listeners []*net.UDPConn

	stopListeners := func() {
		for _, ln := range listeners {
			closeLogged(ln, log, "udp listener")
		}
		for _, h := range handlers {
			h.leases.stop()
		}
		handlers = nil
		listeners = nil
	}

	startServer := func(cfg serverConfig) {
		stopListeners()

		if !cfg.Enabled {
			log.Debug("dhcpserver: disabled in config")
			return
		}

		for si := range cfg.SharedNetworks {
			for subi := range cfg.SharedNetworks[si].Subnets {
				sub := &cfg.SharedNetworks[si].Subnets[subi]
				serverIP := sub.DefaultRouter
				if !serverIP.IsValid() {
					if len(sub.Ranges) > 0 {
						serverIP = sub.Ranges[0].Start
					}
				}
				if !serverIP.IsValid() {
					serverIP = sub.Prefix.Addr()
				}
				h := newDHCPHandler(*sub, serverIP, cfg.PXE)
				handlers = append(handlers, h)
			}
		}

		listeners = startListeners(cfg, handlers, log)
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootService {
				continue
			}
			cfg, err := parseConfig(s.Data)
			if err != nil {
				return fmt.Errorf("dhcpserver: %w", err)
			}
			startServer(cfg)
			return nil
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRootService},
		VerifyBudget: 2,
		ApplyBudget:  5,
	}); err != nil {
		log.Error("dhcpserver plugin failed", "error", err)
		stopListeners()
		return 1
	}

	stopListeners()
	log.Info("dhcpserver plugin stopped")
	return 0
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

// startListeners binds a DHCP listener on each configured interface and starts
// serving. It returns the bound listeners, or nil when none bound -- so the
// caller never logs a false "started" for a server with zero listeners (the
// same silent-failure class fixed in tftpserver/imageserver).
func startListeners(cfg serverConfig, handlers []*dhcpHandler, log *slog.Logger) []*net.UDPConn {
	var listeners []*net.UDPConn
	for _, iface := range cfg.ListenInterfaces {
		ln, err := dhcpListen(iface)
		if err != nil {
			log.Error("dhcpserver: listen failed", "interface", iface, "error", err)
			continue
		}
		listeners = append(listeners, ln)
		go serveMulti(ln, handlers, log)
	}

	if len(listeners) == 0 {
		log.Error("dhcpserver: no interfaces bound; server not serving",
			"interfaces", cfg.ListenInterfaces)
		return nil
	}

	log.Info("dhcpserver: started",
		"shared-networks", len(cfg.SharedNetworks),
		"interfaces", cfg.ListenInterfaces,
		"listeners", len(listeners))
	return listeners
}

func serveMulti(conn *net.UDPConn, handlers []*dhcpHandler, log *slog.Logger) {
	serverIPs := make([]netip.Addr, 0, len(handlers))
	for _, h := range handlers {
		serverIPs = append(serverIPs, h.serverIP)
	}

	buf := make([]byte, 1500)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])

		var resp []byte
		for _, h := range handlers {
			resp = h.handle(pkt)
			if resp != nil {
				break
			}
		}

		logExchange(log, pkt, resp, serverIPs)

		if resp == nil {
			continue
		}

		dst := responseAddr(pkt, resp)
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

// RFC 2131 Section 4.1: response delivery rules.
func responseAddr(req, resp []byte) *net.UDPAddr {
	giaddr := netip.AddrFrom4([4]byte(req[24:28]))
	if giaddr.IsValid() && !giaddr.IsUnspecified() {
		return &net.UDPAddr{IP: giaddr.AsSlice(), Port: 67}
	}

	ciaddr := netip.AddrFrom4([4]byte(req[12:16]))
	if ciaddr.IsValid() && !ciaddr.IsUnspecified() {
		return &net.UDPAddr{IP: ciaddr.AsSlice(), Port: 68}
	}

	// RFC 2131 Section 4.1: if BROADCAST flag is set, broadcast.
	flags := uint16(req[10])<<8 | uint16(req[11])
	if flags&0x8000 != 0 {
		return &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
	}

	// BROADCAST flag clear: unicast to yiaddr.
	yiaddr := netip.AddrFrom4([4]byte(resp[16:20]))
	if yiaddr.IsValid() && !yiaddr.IsUnspecified() {
		return &net.UDPAddr{IP: yiaddr.AsSlice(), Port: 68}
	}

	return &net.UDPAddr{IP: net.IPv4bcast, Port: 68}
}
