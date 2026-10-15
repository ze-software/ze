// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config
// Related: register.go -- runDHCPServerPlugin, which registers the transaction

package dhcpserver

import (
	"bytes"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
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
	// running is the config the handlers and listeners serve, which a failed
	// install binds again.
	running serverConfig
	// boot is true only while the Stage 2 configure installs, whose binds
	// are not strict (bindListeners).
	boot      bool
	handlers  []*dhcpHandler
	listeners []*net.UDPConn
	log       *slog.Logger
}

// install makes cfg the running config, or returns an error and leaves the
// running config serving. A subnet whose address plan (prefix, ranges and
// static mappings) is unchanged keeps its pool and lease table, so the leases
// it granted survive the reload (owner decision Q3 = d1), while its options
// and the PXE settings come from cfg. Every other subnet gets new tables.
//
// The listeners are closed and bound again over the new handler set, because
// a socket bound to an interface's port 67 cannot be bound twice; a packet
// that arrives meanwhile is lost and the client retransmits it. When an
// interface of cfg cannot be bound, the replaced listeners are bound again
// over the replaced handlers and the error is returned, so the transaction
// rolls back. When those cannot be bound either, the error says so: a
// rollback that answers it is reported broken and the plugin is restarted.
func (s *dhcpServer) install(cfg serverConfig) error {
	var handlers []*dhcpHandler
	if cfg.Enabled {
		handlers = buildHandlers(cfg, s.handlers)
	}
	s.closeListeners()
	listeners, err := bindListeners(cfg, handlers, s.log, !s.boot)
	if err != nil {
		stopLeasesNotIn(handlers, s.handlers)
		return s.restore(err)
	}
	stopLeasesNotIn(s.handlers, handlers)
	kept := len(handlers) - len(newLeaseTables(handlers, s.handlers))
	s.running, s.handlers, s.listeners = cfg, handlers, listeners
	if !cfg.Enabled {
		s.log.Debug("dhcpserver: disabled in config")
		return nil
	}
	// The counts tell an operator which subnets kept their leases.
	s.log.Info("dhcpserver: config applied", "subnets-kept", kept, "subnets-restarted", len(handlers)-kept)
	return nil
}

// configureAtBoot runs the Stage 2 configure of tx with binds that are not
// strict, so an interface that cannot be bound at boot is logged and the
// daemon starts with the interfaces that can.
func (s *dhcpServer) configureAtBoot(tx *sdk.ConfigTx[serverConfig], sections []sdk.ConfigSection) error {
	s.boot = true
	defer func() { s.boot = false }()
	return tx.Configure(sections)
}

// restore binds the running config's listeners again after an install whose
// own listeners failed with cause, and answers the error install returns.
func (s *dhcpServer) restore(cause error) error {
	listeners, err := bindListeners(s.running, s.handlers, s.log, true)
	s.listeners = listeners
	if err != nil {
		return fmt.Errorf("%w\nThe previous DHCP config could not be bound again either, so the DHCP server is not serving: %w", cause, err)
	}
	return cause
}

// buildHandlers returns one handler per subnet of cfg, each with cfg's
// options and PXE settings. A subnet whose address plan equals that of a
// handler of previous shares that handler's pool and lease table.
func buildHandlers(cfg serverConfig, previous []*dhcpHandler) []*dhcpHandler {
	var handlers []*dhcpHandler
	for si := range cfg.SharedNetworks {
		for subi := range cfg.SharedNetworks[si].Subnets {
			sub := cfg.SharedNetworks[si].Subnets[subi]
			serverIP := subnetServerIP(&sub)
			if kept := samePlanHandler(previous, &sub); kept != nil {
				handlers = append(handlers, newDHCPHandlerWithTables(sub, serverIP, cfg.PXE, kept.pool, kept.leases))
				continue
			}
			handlers = append(handlers, newDHCPHandler(sub, serverIP, cfg.PXE))
		}
	}
	return handlers
}

// samePlanHandler answers the handler of previous whose subnet has the
// address plan of sub, or nil when none has.
func samePlanHandler(previous []*dhcpHandler, sub *subnetConfig) *dhcpHandler {
	for _, h := range previous {
		if sameAddressPlan(&h.subnet, sub) {
			return h
		}
	}
	return nil
}

// sameAddressPlan reports whether a and b allocate the same addresses: the
// same prefix, the same range bounds and the same static MAC to address
// bindings. A range or mapping name, the lease time and the options do not
// decide which address a client holds, so they are not compared.
func sameAddressPlan(a, b *subnetConfig) bool {
	if a.Prefix != b.Prefix {
		return false
	}
	if !slices.EqualFunc(a.Ranges, b.Ranges, func(x, y addressRange) bool {
		return x.Start == y.Start && x.Stop == y.Stop
	}) {
		return false
	}
	return slices.EqualFunc(a.StaticMappings, b.StaticMappings, func(x, y staticMapping) bool {
		return x.IP == y.IP && bytes.Equal(x.MAC, y.MAC)
	})
}

// newLeaseTables answers the handlers of handlers whose lease table no
// handler of others shares.
func newLeaseTables(handlers, others []*dhcpHandler) []*dhcpHandler {
	var fresh []*dhcpHandler
	for _, h := range handlers {
		if !slices.ContainsFunc(others, func(o *dhcpHandler) bool { return o.leases == h.leases }) {
			fresh = append(fresh, h)
		}
	}
	return fresh
}

// stopLeasesNotIn stops the lease table of every handler of handlers that no
// handler of kept shares, which drops its leases.
func stopLeasesNotIn(handlers, kept []*dhcpHandler) {
	for _, h := range newLeaseTables(handlers, kept) {
		h.leases.stop()
	}
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
