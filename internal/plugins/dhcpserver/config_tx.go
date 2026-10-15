// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runDHCPServerPlugin, which registers the transaction

package dhcpserver

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"reflect"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseDHCPSections reads the service section of one delivery. present is
// false when the delivery carries no service section. It changes no state, so
// it is also the static verifier (verifyDHCPConfig).
func parseDHCPSections(sections []sdk.ConfigSection) (serverConfig, bool, error) {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		cfg, err := parseConfig(s.Data)
		if err != nil {
			return serverConfig{}, true, fmt.Errorf("dhcpserver: %w", err)
		}
		return cfg, true, nil
	}
	return serverConfig{}, false, nil
}

// dhcpServer is the running DHCP server: one handler, with its pool and lease
// table, per configured subnet, and one listener per bound interface. It is
// written only by install, which sdk.ConfigTx serializes, and by stop after
// Run returned. Not safe for concurrent use on its own.
type dhcpServer struct {
	handlers  []*dhcpHandler
	listeners []*net.UDPConn
	log       *slog.Logger
}

// install makes cfg the running config. A subnet whose definition and PXE
// settings are unchanged keeps its handler, so the leases it granted survive
// the reload (owner decision Q3 = d1). Every other handler is stopped, which
// drops its leases, and a new one is built for each new or edited subnet. The
// listeners are rebound over the new handler set, so a packet that arrives
// while they are replaced is lost and the client retransmits it. A listener
// that fails to bind is logged, not returned, as at boot.
func (s *dhcpServer) install(cfg serverConfig) error {
	s.closeListeners()

	previous := s.handlers
	s.handlers = nil
	if cfg.Enabled {
		s.handlers = buildHandlers(cfg, previous)
	}
	kept := 0
	for _, h := range previous {
		if slices.Contains(s.handlers, h) {
			kept++
			continue
		}
		h.leases.stop()
	}
	if !cfg.Enabled {
		s.log.Debug("dhcpserver: disabled in config")
		return nil
	}
	// The counts tell an operator which subnets kept their leases.
	s.log.Info("dhcpserver: config applied", "subnets-kept", kept, "subnets-restarted", len(s.handlers)-kept)
	s.listeners = startListeners(cfg, s.handlers, s.log)
	return nil
}

// buildHandlers returns one handler per subnet of cfg, reusing the handler of
// previous whose subnet and PXE settings deeply equal the new ones.
func buildHandlers(cfg serverConfig, previous []*dhcpHandler) []*dhcpHandler {
	var handlers []*dhcpHandler
	for si := range cfg.SharedNetworks {
		for subi := range cfg.SharedNetworks[si].Subnets {
			sub := &cfg.SharedNetworks[si].Subnets[subi]
			if kept := unchangedHandler(previous, sub, cfg.PXE); kept != nil {
				handlers = append(handlers, kept)
				continue
			}
			handlers = append(handlers, newDHCPHandler(*sub, subnetServerIP(sub), cfg.PXE))
		}
	}
	return handlers
}

// unchangedHandler answers the handler of previous serving exactly sub with
// pxe, or nil when no handler does.
func unchangedHandler(previous []*dhcpHandler, sub *subnetConfig, pxe pxeConfig) *dhcpHandler {
	for _, h := range previous {
		if !reflect.DeepEqual(h.subnet, *sub) {
			continue
		}
		if reflect.DeepEqual(h.pxe, pxe) {
			return h
		}
	}
	return nil
}

// subnetServerIP is the address the server answers a subnet from: its default
// router, else the start of its first range, else the subnet address.
func subnetServerIP(sub *subnetConfig) netip.Addr {
	if sub.DefaultRouter.IsValid() {
		return sub.DefaultRouter
	}
	if len(sub.Ranges) > 0 && sub.Ranges[0].Start.IsValid() {
		return sub.Ranges[0].Start
	}
	return sub.Prefix.Addr()
}

func (s *dhcpServer) closeListeners() {
	for _, ln := range s.listeners {
		closeLogged(ln, s.log, "udp listener")
	}
	s.listeners = nil
}

// stop closes every listener and stops every lease table. It MUST be called
// only after Run returned, when no install can run.
func (s *dhcpServer) stop() {
	s.closeListeners()
	for _, h := range s.handlers {
		h.leases.stop()
	}
	s.handlers = nil
}
