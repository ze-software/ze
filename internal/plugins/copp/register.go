// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- plugin registration

package copp

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/slogutil"
	coppyang "github.com/ze-software/ze/internal/plugins/copp/yang"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRoot = "control-plane-protection"

func init() {
	reg := registry.Registration{
		Name:                    "copp",
		Description:             "Control-plane policing: rate-limit new TCP connections to BGP listen port",
		Features:                "yang",
		YANG:                    coppyang.ZeCoppConfYANG,
		ConfigRoots:             []string{configRoot},
		Dependencies:            []string{"firewall"},
		InProcessConfigVerifier: verifyCoppConfig,
		RunEngine:               runCoppPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		DoctorChecks: []registry.DoctorCheckDef{{
			Name:         "copp-input-chain",
			Phase:        rpc.DoctorPhasePostConfig,
			Order:        720,
			Dependencies: []string{"firewall"},
			Platforms:    []string{"any"},
			Codes:        []string{"doctor-copp-missing"},
			Check:        checkCoppInputChain,
		}},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "copp: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func verifyCoppConfig(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != configRoot {
			continue
		}
		_, _, err := parseCoppConfig(section.Data)
		if err != nil {
			return err
		}
	}
	return nil
}

func runCoppPlugin(conn net.Conn) int {
	logger().Debug("copp plugin starting")

	p := sdk.NewWithConn("copp", conn)
	defer func() { _ = p.Close() }()

	var mu sync.Mutex
	var currentPolicy *coppPolicy
	var pendingPolicy *coppPolicy

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRoot {
				continue
			}
			policy, found, err := parseCoppConfig(section.Data)
			if err != nil {
				return err
			}
			mu.Lock()
			if found {
				pendingPolicy = &policy
			} else {
				pendingPolicy = nil
			}
			mu.Unlock()
		}
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRoot {
				continue
			}
			policy, found, err := parseCoppConfig(section.Data)
			if err != nil {
				return err
			}
			if found {
				return applyCoppPolicy(&policy, &mu, &currentPolicy)
			}
			return applyCoppPolicy(nil, &mu, &currentPolicy)
		}
		return nil
	})

	var activeJournal *sdk.Journal

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		mu.Lock()
		newPolicy := pendingPolicy
		oldPolicy := currentPolicy
		pendingPolicy = nil
		mu.Unlock()

		if newPolicy == nil {
			return nil
		}

		j := sdk.NewJournal()
		err := j.Record(
			func() error {
				return applyCoppPolicy(newPolicy, &mu, &currentPolicy)
			},
			func() error {
				return applyCoppPolicy(oldPolicy, &mu, &currentPolicy)
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
			return fmt.Errorf("copp rollback: %d errors", len(errs))
		}
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 1,
		ApplyBudget:  2,
	})
	if err != nil {
		logger().Error("copp plugin failed", "error", err)
		return 1
	}

	// No shutdown-time table withdrawal here: clean-shutdown teardown is owned
	// centrally by the firewall engine (firewall.FlushAllTables, gated on the
	// `flush-on-shutdown` option), which holds the shared in-process backend and
	// runs as a single ordered actor. A copp-side withdraw would race that close
	// and would also ignore the operator's flush-on-shutdown choice. Config
	// removal while running still withdraws via OnConfigApply -> applyCoppPolicy(nil).
	return 0
}

func applyCoppPolicy(policy *coppPolicy, mu *sync.Mutex, currentPolicy **coppPolicy) error {
	if policy == nil {
		firewall.RegisterTables("copp", nil)
		if err := firewall.ApplyAll(); err != nil {
			return fmt.Errorf("copp withdraw: %w", err)
		}
		mu.Lock()
		*currentPolicy = nil
		mu.Unlock()
		logger().Info("copp table withdrawn")
		return nil
	}

	table := translatePolicy(*policy)
	firewall.RegisterTables("copp", []firewall.Table{table})
	if err := firewall.ApplyAll(); err != nil {
		firewall.RegisterTables("copp", nil)
		_ = firewall.ApplyAll()
		return fmt.Errorf("copp apply: %w", err)
	}

	mu.Lock()
	*currentPolicy = policy
	mu.Unlock()
	logger().Info("copp table applied",
		"rate", policy.Rate,
		"unit", policy.RateUnit,
		"ports", policy.ProtectedPorts,
	)
	return nil
}
