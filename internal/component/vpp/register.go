// Design: docs/research/vpp-deployment-reference.md -- VPP component registration
// Overview: vpp.go -- VPPManager lifecycle

package vpp

import (
	"context"
	"net"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	vppyang "github.com/ze-software/ze/internal/component/vpp/yang"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/report"
	"github.com/ze-software/ze/internal/core/slogutil"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// defaultVPPBinary is the default path to the VPP executable.
const (
	componentVPP = "vpp"
	// dependencyKernel names the doctor check every VPP check runs behind.
	dependencyKernel = "kernel"
	defaultVPPBinary = "/usr/bin/vpp"
	defaultConfDir   = "/etc/vpp"
)

func init() {
	_ = events.RegisterNamespace(vppevents.Namespace,
		vppevents.EventConnected, vppevents.EventDisconnected, vppevents.EventReconnected,
	)

	reg := registry.Registration{
		Name:                    componentVPP,
		Description:             "VPP data plane lifecycle management",
		Features:                "yang",
		YANG:                    vppyang.ZeVPPConfYANG,
		ConfigRoots:             []string{componentVPP},
		InProcessConfigVerifier: verifyVPPConfig,
		RunEngine:               runVPPEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setVPPLogger(slogutil.Logger(loggerName))
		},
		ConfigureMetrics: func(reg metrics.Registry) {
			setVPPMetricsRegistry(reg)
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setVPPEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setVPPLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	// The registration is a compile-time value, so a refusal (a duplicate
	// name, a missing handler) is a Ze defect no config can reach.
	if err := registry.Register(reg); err != nil {
		panic("BUG: vpp: registration failed: " + err.Error())
	}
}

func verifyVPPConfig(sections []sdk.ConfigSection) error {
	parsed, err := ParseConfigSections(sections)
	if err != nil {
		return err
	}
	return parsed.Validate()
}

// runVPPEngine is the plugin RunEngine entry point.
// It binds one configuration transaction and starts its owned manager only
// from OnStarted. That callback waits until the actual API is ready.
func runVPPEngine(conn net.Conn) int {
	lg := logger()
	lg.Debug("vpp plugin starting")

	p := sdk.NewWithConn(componentVPP, conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	runtime := newVPPLifecycle(ctx, runConfiguredVPP)
	newVPPConfigTx(runtime).Register(p)
	p.OnStarted(func(context.Context) error { return runtime.Start() })
	rc := 0
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig: startupConfigRoots(),
		// Candidate startup and restoration each have a 45-second bound.
		// The remaining allowance covers their cancellation and joined stops.
		ApplyBudget: 120,
	}); err != nil {
		lg.Error("vpp plugin failed", "error", err)
		rc = 1
	}

	if err := runtime.Stop(); err != nil {
		report.RaiseWarningFix(processReportSource, codeDoctorVPPProcess, "manager-stop",
			"VPP manager cleanup failed", processReportFix, map[string]any{reportDetailErrorKey: err.Error()})
		lg.Error("vpp manager stopped with an error", "code", codeDoctorVPPProcess,
			"fix", processReportFix, "error", err)
		rc = 1
	}

	return rc
}

func runConfiguredVPP(ctx context.Context, settings *VPPSettings,
	reconnected bool, receipt chan<- error) error {
	manager := NewVPPManager(settings, defaultConfDir, defaultVPPBinary)
	manager.hasRunOnce = reconnected
	manager.startup = receipt
	return manager.Run(ctx)
}
