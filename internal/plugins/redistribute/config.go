// Design: docs/guide/redistribution.md -- route redistribution
// Related: register.go -- binds these handlers to the plugin's config stages
// Related: ../../component/config/loader_redistribute.go -- ExtractRedistributeRules, the one rule parser

package redistribute

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	configredist "github.com/ze-software/ze/internal/component/config/redistribute"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// configRoot is the config root this plugin owns and is auto-loaded by.
const configRoot = "redistribute"

// rulesFromSections builds the import rules from the delivered `redistribute`
// section with the one parser the config component owns, so this plugin and
// the config doors refuse the same rules in the same words. A delivery with no
// such section, or an empty one (a reload that deleted the root), answers no
// rules: redistribution then moves nothing, which is what the config says.
func rulesFromSections(sections []sdk.ConfigSection) ([]configredist.ImportRule, error) {
	for _, section := range sections {
		if section.Root != configRoot {
			continue
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(section.Data), &data); err != nil {
			return nil, fmt.Errorf("redistribute: the delivered config section is not a JSON object: %w", err)
		}
		// The section carries the root wrapped in its own path
		// (config.ExtractConfigSubtree), the shape TreeFromPluginMap reads.
		tree, err := config.TreeFromPluginMap(data)
		if err != nil {
			return nil, fmt.Errorf("redistribute: rebuild the delivered config: %w", err)
		}
		return config.ExtractRedistributeRules(tree)
	}
	return nil, nil
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
	configredist.SetGlobal(configredist.NewEvaluator(rules))
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

// apply installs the verified rules and keeps the evaluator it replaced for a
// rollback. An apply with no verified candidate changes nothing: the
// transaction did not touch this plugin's root.
func (s *ruleState) apply() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// refuse-by-default: guards ruleState.verify -- no verified candidate means the transaction never delivered this root, so the running rules stand
	if !s.verified {
		return nil
	}
	s.previous, s.replaced = configredist.Global(), true
	configredist.SetGlobal(configredist.NewEvaluator(s.pending))
	s.pending, s.verified = nil, false
	return nil
}

// rollback restores the evaluator the last apply replaced.
func (s *ruleState) rollback() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.replaced {
		configredist.SetGlobal(s.previous)
	}
	s.previous, s.replaced = nil, false
	s.pending, s.verified = nil, false
	return nil
}

// committed drops the undo state once the transaction holds.
func (s *ruleState) committed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.previous, s.replaced = nil, false
}
