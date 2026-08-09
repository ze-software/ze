// Design: docs/architecture/dns/as112.md -- as112 plugin registration, OnConfigure
// RFC: rfc/short/rfc7534.md Section 2.3, rfc/short/rfc7535.md Section 2 -- the four fixed anycast addresses

package as112

import (
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/dnsserver"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	as112yang "github.com/ze-software/ze/internal/plugins/as112/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// The four fixed AS112 anycast host addresses (RFC 7534 Section 2.3, RFC
// 7535 Section 2) -- Go constants, never operator-typed (spec Task section).
// Host prefix lengths (/32, /128), NOT the /24,/48 covering prefixes BGP
// announces (spec-as112-0 umbrella finding H3; spec-as112-3 owns those).
const (
	anycastV4DirectDelegationAddr = "192.175.48.1"
	anycastV4DNAMERedirectionAddr = "192.31.196.1"
	anycastV6DirectDelegationAddr = "2620:4f:8000::1"
	anycastV6DNAMERedirectionAddr = "2001:4:112::1"

	anycastV4DirectDelegationHost = "192.175.48.1/32"
	anycastV4DNAMERedirectionHost = "192.31.196.1/32"
	anycastV6DirectDelegationHost = "2620:4f:8000::1/128"
	anycastV6DNAMERedirectionHost = "2001:4:112::1/128"

	as112Port = 53

	// as112Owner is the registry owner name spec-as112-1's API keys
	// registrations by.
	as112Owner = "as112"
)

// hostAddresses returns the /32,/128 CIDR strings to register with
// spec-as112-1's iface registry, filtered by address-family (AC-10).
func hostAddresses(family string) []string {
	var addrs []string
	if family != addressFamilyIPv6Only {
		addrs = append(addrs, anycastV4DirectDelegationHost, anycastV4DNAMERedirectionHost)
	}
	if family != addressFamilyIPv4Only {
		addrs = append(addrs, anycastV6DirectDelegationHost, anycastV6DNAMERedirectionHost)
	}
	return addrs
}

// serverEndpoints returns the UDP+TCP bind targets: the fixed anycast
// addresses filtered by family, plus loopback for local diagnostics (spec
// Data Flow step 3), all on port 53.
func serverEndpoints(family string) []dnsserver.Endpoint {
	var addrs []string
	if family != addressFamilyIPv6Only {
		addrs = append(addrs, anycastV4DirectDelegationAddr, anycastV4DNAMERedirectionAddr, "127.0.0.1")
	}
	if family != addressFamilyIPv4Only {
		addrs = append(addrs, anycastV6DirectDelegationAddr, anycastV6DNAMERedirectionAddr, "::1")
	}
	endpoints := make([]dnsserver.Endpoint, 0, len(addrs))
	for _, a := range addrs {
		endpoints = append(endpoints, dnsserver.Endpoint{IP: netip.MustParseAddr(a), Port: as112Port})
	}
	return endpoints
}

// applyAddressRegistration registers or unregisters as112's fixed host
// addresses against spec-as112-1's iface registry based on cfg.Enabled.
// registerFn/unregisterFn are injected (production: iface.RegisterOwnedAddresses/
// iface.UnregisterOwnedAddresses) so this logic is testable without the real
// iface package's global state.
func applyAddressRegistration(cfg as112Config, registerFn func(ifaceName, owner string, addrs []string) error, unregisterFn func(owner string)) error {
	if !cfg.Enabled {
		unregisterFn(as112Owner)
		return nil
	}
	return registerFn("lo", as112Owner, hostAddresses(cfg.AddressFamily))
}

// computeSerial produces the 32-bit SOA serial for a config generation.
// as112 has no operator-configurable serial-mode (unlike geodns): every
// generation uses auto-epoch semantics (R-2: "reuse geodns's already-solved
// serial-mode design", the auto-epoch case specifically, since as112 never
// exposes the other two modes as config).
func computeSerial(prevSerial uint32, now time.Time) uint32 {
	s := uint32(now.Unix())
	if s <= prevSerial {
		s = prevSerial + 1
	}
	return s
}

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())
	// Register the redistribute source at init so `import as112` resolves during
	// `ze config validate`, which imports plugins but does not start engines.
	registerAS112Sources()

	reg := registry.Registration{
		Name:                    "as112",
		Description:             "AS112 anycast DNS node: authoritative sink for misdirected RFC 1918 / link-local reverse-DNS queries (RFC 7534, RFC 7535)",
		Features:                "yang",
		YANG:                    as112yang.ZeAs112ConfYANG,
		ConfigRoots:             []string{configRootService},
		InProcessConfigVerifier: verifyAS112Config,
		RunEngine:               runAS112Plugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		if l := slogutil.Logger(loggerName); l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureMetrics = func(r metrics.Registry) {
		setMetricsRegistry(r)
	}
	// The redistribute producer emits covering-prefix route-change batches on the
	// in-process EventBus; the hub injects the bus here before RunEngine.
	reg.ConfigureEventBus = func(eb ze.EventBus) {
		setEventBus(eb)
	}
	reg.DoctorChecks = []registry.DoctorCheckDef{
		{
			Name:         "as112-listen-capability",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        723,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-as112-port-unavailable"},
			Check:        checkAS112ListenCapability,
		},
		{
			Name:         "as112-tls-cert",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        724,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-tls-missing", "doctor-tls-expired", "doctor-tls-invalid", "doctor-tls-reference"},
			Check:        checkAS112TLSCert,
		},
	}

	// Finding L3: as112's ipv4-anycast/ipv6-anycast schema anchors
	// (yang/ze-as112-conf.yang) are `config false` lists a real operator
	// config commit never populates, so config.CollectListeners alone would
	// never contribute an endpoint for them. Registering one representative
	// address per family here (own package, not the shared
	// listener_defaults.go, per plugin-self-containment) is what actually
	// makes config.CollectListenersWithDefaults fill in a fallback endpoint,
	// so a parse-time cross-service conflict (e.g. another service
	// wildcard-binding :53) is caught. Names are schema-path-derived by
	// config.DiscoverListenerServices, not chosen freely -- verified
	// empirically against the built schema, not hand-traced (see spec
	// Mistake Log).
	config.RegisterListenerDefault("service-as112-ipv4-anycast-listener", anycastV4DirectDelegationAddr, "53")
	config.RegisterListenerDefault("service-as112-ipv6-anycast-listener", anycastV6DirectDelegationAddr, "53")

	// as112 serves DNS through dnsserver.Manager, whose bind
	// (internal/core/dnsserver/manager.go) takes BOTH a udp PacketConn and a tcp
	// Listener for every endpoint. The zt:listener shape says TCP alone, so
	// without this ze doctor probed TCP/53 and passed while UDP/53 -- the
	// transport nearly every resolver uses -- was held by something else.
	config.RegisterListenerProtocols("service-as112-ipv4-anycast-listener", config.ProtocolUDP, config.ProtocolTCP)
	config.RegisterListenerProtocols("service-as112-ipv6-anycast-listener", config.ProtocolUDP, config.ProtocolTCP)

	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-show:as112",
		Handler:    handleShowAS112,
	})
	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-as112:health",
		Handler:    handleAS112Health,
	})

	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "as112: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// verifyAS112Config is the offline verifier: it parses and validates the
// committed config without binding or applying anything.
func verifyAS112Config(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		if _, err := parseConfig(s.Data); err != nil {
			return fmt.Errorf("as112: %w", err)
		}
	}
	return nil
}

// runAS112Plugin is the engine entry point. On each committed config it
// parses and validates, registers/unregisters the fixed anycast addresses
// against spec-as112-1's iface registry, computes the SOA serial for the
// generation, publishes the resolver snapshot, and reconciles the UDP+TCP
// listeners.
func runAS112Plugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("as112 plugin starting")

	p := sdk.NewWithConn("as112", conn)
	defer func() { _ = p.Close() }()

	// applyAddressRegistration below calls iface.RegisterOwnedAddresses as a
	// plain Go function, not through DirectBridge/DispatchCommand -- that
	// only reaches the engine's real iface.addressOwners registry when this
	// plugin shares process memory with it. An external as112 (operator
	// explicitly writes plugin { external as112 { ... } }; nothing in config
	// validation forbids it) would otherwise "succeed" at every config
	// commit while silently never landing its anycast addresses on any real
	// kernel interface. Refuse to start rather than degrade silently.
	if !p.IsInternal() {
		log.Error("as112: refusing to start as an external plugin process -- the address-ownership registry (iface.RegisterOwnedAddresses) is a same-process call and would silently no-op across a process boundary; configure as112 to run internal")
		return 1
	}

	// Redistribute producer: originate the AS112 covering prefixes into BGP while
	// serving (subject to `import as112`). The DNS server's anycast listener
	// transitions notify prod.onServingChanged (runtime serving-state gate, RFC
	// 7534 Section 3.3, including a listener crash); the producer reads the live
	// PER-FAMILY serving state via mgr.servingFor. servingFn is wired BEFORE
	// subscribeReplay so no subscriber can reconcile before it is set. The deferred
	// withdraw retracts the routes on shutdown (AC-9).
	prod := newAS112Producer()
	defer prod.withdraw()

	mgr := newServerManager(log, prod.onServingChanged)
	prod.setServingFn(mgr.servingFor)

	unsubReplay := prod.subscribeReplay()
	defer unsubReplay()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootService {
				continue
			}
			cfg, err := parseConfig(s.Data)
			if err != nil {
				ametrics().reloadTotal.With("error").Inc()
				return fmt.Errorf("as112: %w", err)
			}

			if rerr := applyAddressRegistration(cfg, iface.RegisterOwnedAddresses, iface.UnregisterOwnedAddresses); rerr != nil {
				ametrics().reloadTotal.With("error").Inc()
				return fmt.Errorf("as112: address registration: %w", rerr)
			}

			var prevSerial uint32
			if prev := loadState(); prev != nil {
				prevSerial = prev.serial
			}
			serial := computeSerial(prevSerial, time.Now())
			storeState(buildState(cfg, serial))

			// Apply the config to the producer; the runtime serving state (anycast
			// listener up/down) is driven separately via prod.onServingChanged from
			// the DNS server's listener transitions. `import as112` gates whether the
			// emitted routes reach the RIB, so as112 still never reads bgp config.
			prod.applyConfig(cfg)
			if aerr := mgr.applyConfig(cfg); aerr != nil {
				log.Error("as112: listener setup failed", "error", aerr)
			}
			ametrics().reloadTotal.With("success").Inc()
			log.Info("as112: config applied", "enabled", cfg.Enabled, "address-family", cfg.AddressFamily,
				"dot", cfg.Secure.DoTEnabled, "doh", cfg.Secure.DoHEnabled)
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
		log.Error("as112 plugin failed", "error", err)
		mgr.stopAll()
		return 1
	}

	mgr.stopAll()
	log.Info("as112 plugin stopped")
	return 0
}
