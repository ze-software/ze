package policyroute

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
	"github.com/ze-software/ze/internal/core/textbuf"
	policyrouteyang "github.com/ze-software/ze/internal/plugins/policyroute/yang"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const configRoot = "policy"

func init() {
	reg := registry.Registration{
		Name:                    "policy-routes",
		Description:             "Policy-based routing: nftables packet marking and ip rule table selection",
		Features:                "yang",
		YANG:                    policyrouteyang.ZePolicyrouteConfYANG,
		ConfigRoots:             []string{configRoot},
		Dependencies:            []string{"firewall"},
		InProcessConfigVerifier: verifyPolicyConfig,
		RunEngine:               runPolicyRoutePlugin,
		Commands:                commandDecls(),
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

func verifyPolicyConfig(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != configRoot {
			continue
		}
		if _, err := parsePolicyConfig(section.Data); err != nil {
			return err
		}
	}
	return nil
}

// errApplyWithoutVerify rejects a config-apply that arrives without the
// config-verify that stages its candidate. Returning nil accepted the
// transaction while silently keeping the PREVIOUS policies, so the daemon
// would report a successful reload over config it never installed.
var errApplyWithoutVerify = errors.New("policy-routes config apply: no verified policies staged (config-apply arrived without config-verify); refusing to report success over the previous policies")

// pendingPolicies is what OnConfigVerify stages for OnConfigApply.
//
// staged and policies answer two different questions, and one slice cannot
// answer both. staged says a verify ran for this transaction. policies says
// what that verify found, and an empty set is a real answer: the operator
// deleted the `policy` block, or the last `policy route` under it, and the
// nftables tables and the ip rules this plugin installed have to come out of
// the kernel.
//
// Folding them made nil mean both "nothing staged" and "there are no policies
// left", and the apply read the second as the first and removed nothing
// (plan/journal/component-rebuilt-during-reload.md;
// ai/rules/principles.md -- a value that is silently wrong must not be
// reachable). parsePolicyConfig returns (nil, nil) both for the empty body a
// reload delivers for a removed root and for a `policy` block with no `route`
// child, so both reached that branch.
//
// NOT safe for concurrent use on its own: the caller holds the plugin's mutex
// across stage and take, as the handlers below do.
type pendingPolicies struct {
	policies []PolicyRoute
	staged   bool
}

// stage records the candidate a config-verify accepted. An empty set is the
// operator removing every policy route, which is a decision the apply must act
// on.
func (p *pendingPolicies) stage(policies []PolicyRoute) {
	p.policies = policies
	p.staged = true
}

// take returns the staged candidate and unstages it, so a second apply behind
// one verify gets staged false and the caller's fail-closed branch.
func (p *pendingPolicies) take() (policies []PolicyRoute, staged bool) {
	policies, staged = p.policies, p.staged
	p.clear()
	return policies, staged
}

// clear unstages the candidate without applying it. The rollback path calls it:
// a transaction that verified here and then failed elsewhere never reaches this
// plugin's apply, and a candidate left staged is applied by the NEXT
// transaction that reaches an apply without a verify. A rolled-back removal
// would leave (nil, true) behind, so that later apply would tear down policy
// routes nobody asked it to remove.
func (p *pendingPolicies) clear() {
	p.policies = nil
	p.staged = false
}

// applyStagedPolicies is the body of the plugin's OnConfigApply handler: it
// takes the candidate a config-verify staged and installs it, recording the
// undo the transaction needs if a later participant fails.
//
// An EMPTY staged set is a removal, and it is applied rather than skipped.
// install with no policies is what takes the nftables tables and the ip rules
// back out (applyPolicies: rm.removeAll over the old result, then a withdraw of
// the table name). The handler used to guard on the slice alone, so deleting
// every policy route looked exactly like a transaction with nothing to do.
//
// install and undo are the journal's two arms. Both are applyPolicies in
// production, closed over the plugin's allocator, mutex and current-state
// slots; undo reads the result the install just recorded, so it cannot be
// derived from install. They are parameters so the verify-to-apply seam can be
// proven without a firewall backend and without root.
func applyStagedPolicies(pending *pendingPolicies, mu *sync.Mutex, install func([]PolicyRoute) error, undo func() error) (*sdk.Journal, error) {
	mu.Lock()
	policies, staged := pending.take()
	mu.Unlock()

	if !staged {
		// Fail closed. The reload transaction drives verify and apply over the
		// SAME participant set -- runTxCoordinator builds both from `affected`
		// (internal/component/plugin/server/reload_tx.go) -- so reaching apply
		// with nothing staged is a protocol violation, not a normal state.
		return nil, errApplyWithoutVerify
	}

	j := sdk.NewJournal()
	err := j.Record(
		func() error { return install(policies) },
		func() error { return undo() },
	)
	if err != nil {
		j.Rollback()
		return nil, err
	}
	return j, nil
}

func runPolicyRoutePlugin(conn net.Conn) int {
	logger().Debug("policy-routes plugin starting")

	p := sdk.NewWithConn("policy-routes", conn)
	defer func() { _ = p.Close() }()

	alloc := newAllocator()

	var mu sync.Mutex
	var currentPolicies []PolicyRoute
	var currentResult *translationResult
	var pending pendingPolicies

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRoot {
				continue
			}
			policies, err := parsePolicyConfig(section.Data)
			if err != nil {
				return err
			}
			mu.Lock()
			pending.stage(policies)
			mu.Unlock()
		}
		return nil
	})

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != configRoot {
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
		oldPolicies := currentPolicies
		oldResult := currentResult
		mu.Unlock()

		j, err := applyStagedPolicies(&pending, &mu,
			func(policies []PolicyRoute) error {
				return applyPolicies(alloc, policies, oldResult, &mu, &currentPolicies, &currentResult)
			},
			func() error {
				return applyPolicies(alloc, oldPolicies, currentResult, &mu, &currentPolicies, &currentResult)
			},
		)
		if err != nil {
			return err
		}
		activeJournal = j
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		// A rollback ends the transaction, so the candidate this plugin verified
		// is unstaged rather than left for an apply that will never ask for it
		// (pendingPolicies.clear).
		mu.Lock()
		pending.clear()
		mu.Unlock()

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

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		if command == "show policy routes" {
			mu.Lock()
			policies := currentPolicies
			mu.Unlock()
			return "done", formatPolicies(policies), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 1,
		ApplyBudget:  2,
		Commands:     commandDecls(),
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

	if err := firewall.RegisterTables("policy-routes", result.Tables); err != nil {
		return err
	}
	if err := firewall.ApplyAll(); err != nil {
		return fmt.Errorf("nftables apply: %w", err)
	}

	if err := rm.applyAll(result); err != nil {
		rm.removeAll(result)
		_ = firewall.RegisterTables("policy-routes", nil) // a withdraw registers no name, so it cannot be refused
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
	_ = firewall.RegisterTables("policy-routes", nil) // a withdraw registers no name, so it cannot be refused
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

func formatPolicies(policies []PolicyRoute) any {
	out := make([]showPolicy, 0, len(policies))
	for _, p := range policies {
		sp := showPolicy{Name: p.Name}
		for _, iface := range p.Interfaces {
			name := iface.Name
			if iface.Wildcard {
				name += "*"
			}
			sp.Interfaces = append(sp.Interfaces, name)
		}
		for i := range p.Rules {
			r := &p.Rules[i]
			action := "unknown"
			switch r.Action.Type {
			case ActionAccept:
				action = "accept"
			case ActionDrop:
				action = "drop"
			case ActionTable:
				var bAct textbuf.Buffer
				action = bAct.Reset().Str("table ").Int(int64(r.Action.Table)).String()
			case ActionNextHop:
				action = "next-hop " + r.Action.NextHop.String()
			}
			sp.Rules = append(sp.Rules, showRule{Name: r.Name, Action: action})
		}
		out = append(out, sp)
	}
	return out
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: "show policy routes"},
	}
}
