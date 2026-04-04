package sysrib

import (
	"context"
	"fmt"
	"net"
	"os"

	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sysribschema "codeberg.org/thomas-mangin/ze/internal/plugins/sysrib/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

func init() {
	reg := registry.Registration{
		Name:        "sysrib",
		Description: "System RIB: selects best route across protocols by admin distance",
		Features:    "yang",
		YANG:        sysribschema.ZeSysribConfYANG,
		ConfigRoots: []string{"sysrib"},
		RunEngine:   runSysRIBPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureBus: func(bus any) {
			if b, ok := bus.(ze.Bus); ok {
				setBus(b)
			}
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "sysrib: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runSysRIBPlugin(conn net.Conn) int {
	logger().Debug("sysrib plugin starting (RPC)")

	p := sdk.NewWithConn("sysrib", conn)
	defer func() { _ = p.Close() }()

	s := newSysRIB()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "sysrib" {
				continue
			}
			dist, err := parseAdminDistanceConfig(section.Data)
			if err != nil {
				logger().Error("admin-distance config parse failed", "error", err)
				return err
			}
			s.mu.Lock()
			s.adminDist = dist
			s.mu.Unlock()
			logger().Info("admin-distance config loaded", "distances", dist)
		}
		return nil
	})

	// pendingDist holds the validated admin-distance map between verify and apply.
	var pendingDist map[string]int

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "sysrib" {
				continue
			}
			dist, err := parseAdminDistanceConfig(section.Data)
			if err != nil {
				return err
			}
			pendingDist = dist
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		dist := pendingDist
		pendingDist = nil
		if dist == nil {
			return nil
		}

		s.mu.Lock()
		s.adminDist = dist
		s.mu.Unlock()

		// Re-evaluate existing routes with new distances.
		changes := s.reapplyAdminDistances()
		for family, ch := range changes {
			if len(ch) > 0 {
				publishChanges(ch, family)
			}
		}

		logger().Info("admin-distance config reloaded", "distances", dist)
		return nil
	})

	p.OnStarted(func(ctx context.Context) error {
		go s.run(ctx)
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "sysrib show" {
			data, err := s.showRIB()
			if err != nil {
				return "error", "", err
			}
			return "done", data, nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"sysrib"},
		Commands: []sdk.CommandDecl{
			{Name: "sysrib show"},
		},
	})
	if err != nil {
		logger().Error("sysrib plugin failed", "error", err)
		return 1
	}

	return 0
}
