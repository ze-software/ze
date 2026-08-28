// Design: docs/research/vpp-deployment-reference.md -- VPP component registration
// Overview: vpp.go -- VPPManager lifecycle

package vpp

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	vppyang "github.com/ze-software/ze/internal/component/vpp/yang"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

// defaultVPPBinary is the default path to the VPP executable.
const (
	componentVPP     = "vpp"
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
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "vpp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyVPPConfig(sections []sdk.ConfigSection) error {
	for _, s := range sections {
		if s.Root != componentVPP {
			continue
		}
		parsed, err := ParseConfigSection(s.Data)
		if err != nil {
			return err
		}
		if err := parsed.Validate(); err != nil {
			return fmt.Errorf("vpp: config validation: %w", err)
		}
	}
	return nil
}

// runVPPEngine is the plugin RunEngine entry point.
// It creates the VPPManager and runs it inside the SDK's OnStarted callback
// so the plugin remains live for config reload events.
func runVPPEngine(conn net.Conn) int {
	lg := logger()
	lg.Debug("vpp plugin starting")

	p := sdk.NewWithConn(componentVPP, conn)
	defer func() { _ = p.Close() }()

	// Initialize settings to the disabled default so OnStarted can run
	// safely even if OnConfigure never fires or fails to parse. Without this,
	// a failed OnConfigure returns an error, leaves `settings` as nil, and
	// NewVPPManager(nil) in OnStarted deref-panics on settings.APISocket.
	settings := &VPPSettings{Enabled: false}

	// The Manager's context is created here, not inside OnStarted, so every
	// return path cancels it. A p.Run error below returned without canceling
	// it and without waiting for the Manager goroutine.
	mgrCtx, mgrCancel := context.WithCancel(context.Background())
	defer mgrCancel()

	var mgrDone chan struct{}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != componentVPP {
				continue
			}
			parsed, err := ParseConfigSection(s.Data)
			if err != nil {
				return err
			}
			if err := parsed.Validate(); err != nil {
				return fmt.Errorf("vpp: config validation: %w", err)
			}
			settings = parsed
		}
		return nil
	})

	p.OnConfigVerify(verifyVPPConfig)

	p.OnConfigApply(func(sections []sdk.ConfigDiffSection) error {
		lg.Warn("vpp: config reload requires daemon restart to take effect")
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		// Start the VPP Manager in a background goroutine.
		// The Manager owns VPP's full lifecycle; the SDK event loop
		// continues to handle config reload callbacks.
		mgrDone = make(chan struct{})

		mgr := NewVPPManager(settings, defaultConfDir, defaultVPPBinary)
		go func() {
			defer close(mgrDone)
			if err := mgr.Run(mgrCtx); err != nil {
				lg.Error("vpp manager failed", "error", err)
			}
		}()
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	rc := 0
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{componentVPP},
	}); err != nil {
		lg.Error("vpp plugin failed", "error", err)
		rc = 1
	}

	// The SDK event loop exited (engine shutdown, or p.Run failed). Stop the
	// Manager and wait for its goroutine before returning.
	mgrCancel()
	if mgrDone != nil {
		<-mgrDone
	}

	return rc
}
