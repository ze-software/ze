package policyroute

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"

	"codeberg.org/thomas-mangin/ze/internal/component/firewall"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	policyrouteschema "codeberg.org/thomas-mangin/ze/internal/plugins/policyroute/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

func init() {
	reg := registry.Registration{
		Name:        "policy-routes",
		Description: "Policy-based routing: nftables packet marking and ip rule table selection",
		Features:    "yang",
		YANG:        policyrouteschema.ZePolicyrouteConfYANG,
		ConfigRoots: []string{"policy"},
		RunEngine:   runPolicyRoutePlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
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
		fmt.Fprintf(os.Stderr, "policy-routes: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runPolicyRoutePlugin(conn net.Conn) int {
	logger().Debug("policy-routes plugin starting")

	p := sdk.NewWithConn("policy-routes", conn)
	defer func() { _ = p.Close() }()

	alloc := newAllocator()

	var mu sync.Mutex
	var currentPolicies []PolicyRoute
	var currentResult *translationResult
	var pendingPolicies []PolicyRoute

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "policy" {
				continue
			}
			policies, err := parsePolicyConfig(section.Data)
			if err != nil {
				return err
			}
			mu.Lock()
			pendingPolicies = policies
			mu.Unlock()
		}
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "policy" {
				continue
			}
			policies, err := parsePolicyConfig(section.Data)
			if err != nil {
				return err
			}
			if err := applyPolicies(alloc, policies, nil, &mu, &currentPolicies, &currentResult); err != nil {
				return err
			}
		}
		return nil
	})

	var activeJournal *sdk.Journal

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		mu.Lock()
		newPolicies := pendingPolicies
		oldPolicies := currentPolicies
		oldResult := currentResult
		pendingPolicies = nil
		mu.Unlock()

		if newPolicies == nil {
			return nil
		}

		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				return applyPolicies(alloc, newPolicies, oldResult, &mu, &currentPolicies, &currentResult)
			},
			func() error {
				return applyPolicies(alloc, oldPolicies, currentResult, &mu, &currentPolicies, &currentResult)
			},
		)
		if err != nil {
			j.Rollback()
			return err
		}

		activeJournal = j
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("policy-routes rollback: %d errors", len(errs))
		}
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "policy show" {
			mu.Lock()
			policies := currentPolicies
			mu.Unlock()
			data, err := formatPolicies(policies)
			if err != nil {
				return "error", "", err
			}
			return "done", data, nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"policy"},
		VerifyBudget: 1,
		ApplyBudget:  2,
		Commands: []sdk.CommandDecl{
			{Name: "policy show"},
		},
	})
	if err != nil {
		logger().Error("policy-routes plugin failed", "error", err)
		return 1
	}

	mu.Lock()
	result := currentResult
	mu.Unlock()
	cleanupOnShutdown(result)

	return 0
}

func applyPolicies(alloc *allocator, policies []PolicyRoute, oldResult *translationResult, mu *sync.Mutex, currentPolicies *[]PolicyRoute, currentResult **translationResult) error {
	alloc.reset()

	result, err := alloc.translate(policies)
	if err != nil {
		return fmt.Errorf("translate: %w", err)
	}

	rm, err := newRuleManager()
	if err != nil {
		return err
	}
	defer rm.close()

	if oldResult != nil {
		rm.removeAll(oldResult)
	}

	firewall.RegisterTables("policy-routes", result.Tables)
	if err := firewall.ApplyAll(); err != nil {
		return fmt.Errorf("nftables apply: %w", err)
	}

	if err := rm.applyAll(result); err != nil {
		rm.removeAll(result)
		firewall.RegisterTables("policy-routes", nil)
		_ = firewall.ApplyAll()
		return err
	}

	mu.Lock()
	*currentPolicies = policies
	*currentResult = result
	mu.Unlock()

	logger().Info("policy routes applied", "count", len(policies))
	return nil
}

func cleanupOnShutdown(result *translationResult) {
	firewall.RegisterTables("policy-routes", nil)
	_ = firewall.ApplyAll()

	if result == nil {
		return
	}
	rm, err := newRuleManager()
	if err != nil {
		logger().Warn("cleanup: failed to create rule manager", "error", err)
		return
	}
	defer rm.close()
	rm.removeAll(result)
}

type showPolicy struct {
	Name       string     `json:"name"`
	Interfaces []string   `json:"interfaces"`
	Rules      []showRule `json:"rules"`
}

type showRule struct {
	Name   string `json:"name"`
	Action string `json:"action"`
}

func formatPolicies(policies []PolicyRoute) (string, error) {
	var out []showPolicy
	for _, p := range policies {
		sp := showPolicy{Name: p.Name}
		for _, iface := range p.Interfaces {
			name := iface.Name
			if iface.Wildcard {
				name += "*"
			}
			sp.Interfaces = append(sp.Interfaces, name)
		}
		for _, r := range p.Rules {
			action := "unknown"
			switch r.Action.Type {
			case ActionAccept:
				action = "accept"
			case ActionDrop:
				action = "drop"
			case ActionTable:
				action = fmt.Sprintf("table %d", r.Action.Table)
			case ActionNextHop:
				action = fmt.Sprintf("next-hop %s", r.Action.NextHop)
			}
			sp.Rules = append(sp.Rules, showRule{Name: r.Name, Action: action})
		}
		out = append(out, sp)
	}
	data, err := json.Marshal(out)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
