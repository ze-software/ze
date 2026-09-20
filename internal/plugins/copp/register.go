// Design: docs/architecture/traffic/cp-survival-2-copp-port179.md -- plugin registration

package copp

import (
	"errors"
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

// errApplyWithoutVerify rejects a config-apply that arrives without the
// config-verify that stages its candidate. Returning nil accepted the
// transaction while silently keeping the PREVIOUS policy, so the daemon would
// report a successful reload over config it never installed.
var errApplyWithoutVerify = errors.New("copp config apply: no verified policy staged (config-apply arrived without config-verify); refusing to report success over the previous policy")

// pendingPolicy is what OnConfigVerify stages for OnConfigApply.
//
// staged and policy answer two different questions, and one pointer cannot
// answer both. staged says a verify ran for this transaction. policy says what
// that verify found, and nil is a real answer: the operator deleted the
// `control-plane-protection` block, and the table has to come out of the
// kernel.
//
// Folding them made nil mean both "nothing staged" and "the section is gone",
// and the apply read the second as the first and withdrew nothing, so copp's
// nftables table stayed in the kernel after the block was deleted
// (plan/journal/component-rebuilt-during-reload.md, 2026-09-09;
// ai/rules/principles.md -- a value that is silently wrong must not be
// reachable).
//
// NOT safe for concurrent use on its own: the caller holds the plugin's mutex
// across stage and take, as the handlers below do.
type pendingPolicy struct {
	policy *coppPolicy
	staged bool
}

// stage records the candidate a config-verify accepted. A nil policy is the
// operator deleting the section, which is a decision the apply must act on.
func (p *pendingPolicy) stage(policy *coppPolicy) {
	p.policy = policy
	p.staged = true
}

// take returns the staged candidate and unstages it, so a second apply behind
// one verify gets staged false and the caller's fail-closed branch.
func (p *pendingPolicy) take() (policy *coppPolicy, staged bool) {
	policy, staged = p.policy, p.staged
	p.clear()
	return policy, staged
}

// clear unstages the candidate without applying it. The rollback path calls it:
// a transaction that verified here and then failed elsewhere never reaches this
// plugin's apply, and a candidate left staged is applied by the NEXT
// transaction that reaches an apply without a verify. That is the state the
// apply's staged check exists to refuse, and a stale candidate defeats it -- a
// rolled-back removal would leave (nil, true) behind, so the next apply would
// withdraw a table nobody asked it to withdraw.
func (p *pendingPolicy) clear() {
	p.policy = nil
	p.staged = false
}

func runCoppPlugin(conn net.Conn) int {
	logger().Debug("copp plugin starting")

	p := sdk.NewWithConn("copp", conn)
	defer func() { _ = p.Close() }()

	var mu sync.Mutex
	var currentPolicy *coppPolicy
	var pending pendingPolicy

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
				pending.stage(&policy)
			} else {
				// The operator deleted the section, or left it with no `bgp` body.
				// Either way there is no policy to install and the table must come
				// out, so the answer is STAGED rather than left unstaged.
				pending.stage(nil)
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
		oldPolicy := currentPolicy
		mu.Unlock()

		j, err := applyStagedPolicy(&pending, &mu, oldPolicy, func(policy *coppPolicy) error {
			return applyCoppPolicy(policy, &mu, &currentPolicy)
		})
		if err != nil {
			return err
		}
		activeJournal = j
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		// A rollback ends the transaction, so the candidate this plugin verified
		// is unstaged rather than left for an apply that will never ask for it
		// (pendingPolicy.clear).
		mu.Lock()
		pending.clear()
		mu.Unlock()

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
	// and would also ignore the operator's flush-on-shutdown choice.
	//
	// Config removal while the daemon runs is the other stop, and it withdraws
	// through OnConfigApply -> applyCoppPolicy(nil, ...). That sentence stood
	// here while it was false: a removal delivers an empty body, which
	// parseCoppConfig reports as found false, and the apply read the nil that
	// produced as "nothing staged" and returned without withdrawing, leaving the
	// table in the kernel (plan/journal/component-rebuilt-during-reload.md,
	// 2026-09-09). pendingPolicy now carries the two facts separately, so the
	// apply can tell a removal from an empty stage and acts on it.
	return 0
}

// applyStagedPolicy is the body of the plugin's OnConfigApply handler: it takes
// the candidate a config-verify staged and installs it, recording the undo the
// transaction needs if a later participant fails.
//
// A nil staged policy is a REMOVAL, and it is applied rather than skipped. That
// distinction is the whole point of this function and of pendingPolicy: the
// handler used to guard on the policy pointer alone, so the operator deleting
// `control-plane-protection` looked exactly like a transaction with nothing to
// do, and copp's nftables table stayed in the kernel
// (plan/journal/component-rebuilt-during-reload.md, 2026-09-09).
//
// install is applyCoppPolicy in production, closed over the plugin's own mutex
// and current-policy slot. It is a parameter so the verify-to-apply seam can be
// proven without a firewall backend and without root; the Linux integration
// test drives the same seam through the kernel.
//
// It is a named function rather than the closure it replaced because a closure
// over four locals cannot be called from a test at all.
func applyStagedPolicy(pending *pendingPolicy, mu *sync.Mutex, old *coppPolicy, install func(*coppPolicy) error) (*sdk.Journal, error) {
	mu.Lock()
	policy, staged := pending.take()
	mu.Unlock()

	if !staged {
		// Fail closed. The reload transaction drives verify and apply over the
		// SAME participant set -- runTxCoordinator builds both from `affected`
		// (internal/component/plugin/server/reload_tx.go) -- so reaching apply
		// with nothing staged is a protocol violation, not a normal state.
		// Returning no error accepted the transaction over the previous policy.
		return nil, errApplyWithoutVerify
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error { return install(policy) },
		func() error { return install(old) },
	)
	if err != nil {
		j.Rollback()
		return nil, err
	}
	return j, nil
}

func applyCoppPolicy(policy *coppPolicy, mu *sync.Mutex, currentPolicy **coppPolicy) error {
	if policy == nil {
		_ = firewall.RegisterTables("copp", nil) // a withdraw registers no name, so it cannot be refused
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
	if err := firewall.RegisterTables("copp", []firewall.Table{table}); err != nil {
		return fmt.Errorf("copp apply: %w", err)
	}
	if err := firewall.ApplyAll(); err != nil {
		_ = firewall.RegisterTables("copp", nil) // a withdraw registers no name, so it cannot be refused
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
