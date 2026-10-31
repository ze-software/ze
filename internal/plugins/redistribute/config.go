// Design: docs/guide/redistribution.md -- route redistribution
// Related: register.go -- binds these handlers to the plugin's config stages
// Related: ../../component/config/loader_redistribute.go -- ExtractRedistributeRules, the one rule parser

package redistribute

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	"github.com/ze-software/ze/internal/component/policy"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// configRoot is the config root this plugin owns and is auto-loaded by.
const configRoot = "redistribute"

// chainAttachment is the filter attachment an import rule's chain compiles
// against, registered in register.go with the fields a redistributed route
// carries.
const chainAttachment = "redistribute import"

// rulesFromSections builds the import rules from the delivered `redistribute`
// section with the one parser the config component owns, so this plugin and
// the config doors refuse the same rules in the same words, then compiles each
// rule's chain against the delivered policy root. A delivery with no such
// section, or an empty one (a reload that deleted the root), answers no rules:
// redistribution then moves nothing, which is what the config says.
func rulesFromSections(sections []sdk.ConfigSection) ([]configredist.ImportRule, error) {
	var tree *config.Tree
	var policyCfg map[string]any
	for _, section := range sections {
		if section.Root != configRoot && section.Root != policy.RootName {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(section.Data), &data); err != nil {
			return nil, fmt.Errorf("redistribute: the delivered %s section is not a JSON object: %w", section.Root, err)
		}
		if section.Root == policy.RootName {
			// The section carries the root wrapped in its own path; an absent
			// or empty policy root defines no instance.
			policyCfg, _ = data[policy.RootName].(map[string]any)
			continue
		}
		// The section carries the root wrapped in its own path
		// (config.ExtractConfigSubtree), the shape TreeFromPluginMap reads.
		rebuilt, err := config.TreeFromPluginMap(data)
		if err != nil {
			return nil, fmt.Errorf("redistribute: rebuild the delivered config: %w", err)
		}
		tree = rebuilt
	}
	// refuse-by-default: guards ruleState.apply -- no redistribute section is a config with no rules, and apply installs an evaluator moving nothing
	if tree == nil {
		return nil, nil
	}
	rules, err := config.ExtractRedistributeRules(tree)
	if err != nil {
		return nil, err
	}
	if err := compileRuleChains(rules, policyCfg); err != nil {
		return nil, err
	}
	return rules, nil
}

// compileRuleChains compiles the chain of every rule that names one, with the
// policy library's one compile path, so a redistribute import chain refuses
// the names a Loc-RIB import chain refuses, in the same words.
func compileRuleChains(rules []configredist.ImportRule, policyCfg map[string]any) error {
	for i := range rules {
		rule := &rules[i]
		// refuse-by-default: guards configredist.ImportRule.Accept -- a rule with no final has no chain and moves every route of its source
		if rule.Final == "" {
			continue
		}
		var final policy.Final
		switch rule.Final {
		case "accept":
			final = policy.FinalAccept
		case "reject":
			final = policy.FinalReject
		default:
			return fmt.Errorf("redistribute: import %q under destination %q has an invalid final %q; write final accept or final reject",
				rule.Source, rule.Destination, rule.Final)
		}
		chain, err := policy.CompileRouteChain(chainAttachment, rule.Filters, final, policyCfg)
		if err != nil {
			return fmt.Errorf("redistribute: import %q under destination %q: %w", rule.Source, rule.Destination, err)
		}
		rule.Chain = chain
	}
	return nil
}

// ruleState carries the rules a reload verified until it applies them, and the
// evaluator an apply replaced until the transaction commits or rolls back.
// Safe for concurrent use: the SDK may run the stages on different goroutines.
type ruleState struct {
	mu       sync.Mutex
	pending  []configredist.ImportRule
	verified bool
	previous *configredist.Evaluator
	replaced bool
}

// configure installs the startup rules. A config this plugin cannot turn into
// rules refuses the startup: a missing evaluator disables every rule in the
// file, and the operator would see nothing move with no reason given.
func (s *ruleState) configure(sections []sdk.ConfigSection) error {
	rules, err := rulesFromSections(sections)
	if err != nil {
		return err
	}
	consumerHeld.install(configredist.NewEvaluator(rules))
	return nil
}

// verify parses a reload's candidate and keeps its rules for apply. Each
// transaction starts empty, so an apply never installs rules a verify of an
// earlier, aborted transaction left behind.
func (s *ruleState) verify(sections []sdk.ConfigSection) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending, s.verified = nil, false
	rules, err := rulesFromSections(sections)
	if err != nil {
		return err
	}
	s.pending, s.verified = rules, true
	return nil
}

// apply installs the verified rules, keeps the evaluator it replaced for a
// rollback, and reconciles every consumer with the new rules
// (reconcileConsumers). An apply with no verified candidate changes nothing:
// the transaction did not touch this plugin's root.
func (s *ruleState) apply() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// refuse-by-default: guards ruleState.verify -- no verified candidate means the transaction never delivered this root, so the running rules stand
	if !s.verified {
		return nil
	}
	s.previous, s.replaced = configredist.Global(), true
	ev := configredist.NewEvaluator(s.pending)
	consumerHeld.install(ev)
	s.pending, s.verified = nil, false
	reconcileConsumers(ev)
	return nil
}

// rollback restores the evaluator the last apply replaced, and puts every
// consumer back to what those rules decide: the routes the apply withdrew are
// sent again, and the ones it admitted are withdrawn.
func (s *ruleState) rollback() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.replaced {
		consumerHeld.install(s.previous)
		reconcileConsumers(s.previous)
	}
	s.previous, s.replaced = nil, false
	s.pending, s.verified = nil, false
	return nil
}

// reconcileConsumers makes what each consumer holds agree with ev, the rules
// just installed, without waiting for any source to announce again. A held
// route ev rejects is withdrawn now. Then every registered consumer is
// replayed (replayConsumers): each producer re-sends its current set, which ev
// judges, so a route ev newly accepts is dispatched and a route whose chain now
// sets another tag or metric is sent again with it. The withdraw runs first,
// so the replay never races a withdraw of the route it re-sends.
//
// The config stages carry no context, and the withdraw is the same dispatch an
// accepted Remove makes.
func reconcileConsumers(ev *configredist.Evaluator) {
	withdrawRejectedHeld(context.Background(), ev)
	replayConsumers(getEventBus(), getReplayCoordinator())
}

// committed drops the undo state once the transaction holds.
func (s *ruleState) committed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previous, s.replaced = nil, false
}
