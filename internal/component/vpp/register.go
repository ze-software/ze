// Design: docs/research/vpp-deployment-reference.md -- VPP component registration
// Overview: vpp.go -- VPPManager lifecycle

package vpp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	vppschema "codeberg.org/thomas-mangin/ze/internal/component/vpp/schema"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// defaultVPPBinary is the default path to the VPP executable.
const defaultVPPBinary = "/usr/bin/vpp"

// defaultConfDir is the default directory for VPP configuration files.
const defaultConfDir = "/etc/vpp"

func init() {
	reg := registry.Registration{
		Name:        "vpp",
		Description: "VPP data plane lifecycle management",
		Features:    "yang",
		YANG:        vppschema.ZeVppConfYANG,
		ConfigRoots: []string{"vpp"},
		RunEngine:   runVPPEngine,
		ConfigureEngineLogger: func(loggerName string) {
			SetVPPLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				SetVPPEventBus(e)
			}
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			SetVPPLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "vpp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

// runVPPEngine is the plugin RunEngine entry point.
// It creates the VPPManager and runs it inside the SDK's OnStarted callback
// so the plugin remains live for config reload events.
func runVPPEngine(conn net.Conn) int {
	lg := logger()
	lg.Debug("vpp plugin starting")

	p := sdk.NewWithConn("vpp", conn)
	defer func() { _ = p.Close() }()

	var settings *VPPSettings
	var mgrCancel context.CancelFunc
	var mgrDone chan struct{}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, s := range sections {
			if s.Root != "vpp" {
				continue
			}
			parsed, err := ParseSettings(json.RawMessage(s.Data))
			if err != nil {
				return fmt.Errorf("vpp: parse config: %w", err)
			}
			settings = parsed
		}
		if settings == nil {
			settings = &VPPSettings{Enabled: false}
		}
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		// Start the VPP Manager in a background goroutine.
		// The Manager owns VPP's full lifecycle; the SDK event loop
		// continues to handle config reload callbacks.
		mgrCtx, cancel := context.WithCancel(context.Background())
		mgrCancel = cancel
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

	ctx := context.Background()
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"vpp"},
	}); err != nil {
		lg.Error("vpp plugin failed", "error", err)
		return 1
	}

	// SDK event loop exited (engine shutdown). Stop the Manager and wait for cleanup.
	if mgrCancel != nil {
		mgrCancel()
	}
	if mgrDone != nil {
		<-mgrDone
	}

	return 0
}
