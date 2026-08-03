package geodns

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	geodnsyang "github.com/ze-software/ze/internal/plugins/geodns/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRootService = "service"

var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	reg := registry.Registration{
		Name:                    "geodns",
		Description:             "GeoDNS server: DNS answers selected by client source IP (RFC 1035, RFC 7871 client-subnet)",
		Features:                "yang",
		YANG:                    geodnsyang.ZeGeodnsConfYANG,
		ConfigRoots:             []string{configRootService},
		InProcessConfigVerifier: verifyGeoDNSConfig,
		RunEngine:               runGeoDNSPlugin,
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
	reg.DoctorChecks = []registry.DoctorCheckDef{
		{
			Name:         "geodns-listen-capability",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        722,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-geodns-port-unavailable"},
			Check:        checkGeoDNSListenCapability,
		},
		{
			Name:         "geodns-tls-cert",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        725,
			Dependencies: []string{"fib-kernel"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-tls-missing", "doctor-tls-expired", "doctor-tls-invalid", "doctor-tls-reference"},
			Check:        checkGeoDNSTLSCert,
		},
	}

	pluginserver.RegisterRPCs(pluginserver.RPCRegistration{
		WireMethod: "ze-show:geodns",
		Handler:    handleShowGeoDNS,
	})

	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "geodns: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// verifyGeoDNSConfig is the offline verifier: it parses and validates the
// committed config without binding or applying anything, so a bad config aborts
// the commit. It shares parseConfig with the engine's apply path.
func verifyGeoDNSConfig(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != configRootService {
			continue
		}
		if _, err := parseConfig(s.Data); err != nil {
			return fmt.Errorf("geodns: %w", err)
		}
	}
	return nil
}

// runGeoDNSPlugin is the engine entry point. On each committed config it parses
// and validates, computes the SOA serial for the generation, publishes the
// resolver snapshot, and reconciles the UDP+TCP listeners (rebinding only when
// the endpoint set changes; host-data changes are picked up by the handler).
func runGeoDNSPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("geodns plugin starting")

	p := sdk.NewWithConn("geodns", conn)
	defer func() { _ = p.Close() }()

	mgr := newServerManager(log)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != configRootService {
				continue
			}
			cfg, err := parseConfig(s.Data)
			if err != nil {
				gmetrics().reloadTotal.With("error").Inc()
				return fmt.Errorf("geodns: %w", err)
			}
			var prevSerial uint32
			if prev := loadState(); prev != nil {
				prevSerial = prev.serial
			}
			st := buildState(cfg)
			st.serial = computeSerial(cfg.SOA, prevSerial, time.Now())
			storeState(st)
			if aerr := mgr.apply(cfg); aerr != nil {
				log.Error("geodns: listener setup failed", "error", aerr)
			}
			gmetrics().reloadTotal.With("success").Inc()
			log.Info("geodns: config applied",
				"enabled", cfg.Enabled,
				"listeners", len(cfg.Listeners),
				"zones", len(cfg.Zones),
				"host-sets", len(cfg.HostSets),
				"sources", len(cfg.Sources))
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
		log.Error("geodns plugin failed", "error", err)
		mgr.stopAll()
		return 1
	}

	mgr.stopAll()
	log.Info("geodns plugin stopped")
	return 0
}
