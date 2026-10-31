// Design: docs/guide/redistribution.md -- the IS-IS export chain over redistributed routes
// Related: redistribute/export.go -- SetExportChain and the consumer's judgment
// Related: register.go -- parses the chain at verify and installs it at configure and apply

package isis

import (
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/internal/component/policy"
	isisredistribute "github.com/ze-software/ze/internal/plugins/isis/redistribute"
)

// exportAttachment is the filter attachment of `isis filter export`.
const exportAttachment = "isis export"

// exportChainConfig is the compiled `isis filter export` chain of one config.
// A nil chain means the config sets no export chain.
type exportChainConfig struct {
	chain *policy.RouteChain
}

// install makes c the chain the redistribution consumer judges routes with.
func (c exportChainConfig) install() {
	if c.chain == nil {
		isisredistribute.SetExportChain(nil)
		return
	}
	isisredistribute.SetExportChain(c.chain)
}

// parseExportChain compiles the export chain from the isis section against
// the filter instances of the policy section. A config with no isis section,
// or no export container, compiles to no chain.
func parseExportChain(sections []configSection) (exportChainConfig, error) {
	var isisCfg, policyCfg map[string]any
	for _, s := range sections {
		if s.Root != configRoot && s.Root != policy.RootName {
			continue
		}
		if s.Data == "" {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(s.Data), &wrapper); err != nil {
			return exportChainConfig{}, fmt.Errorf("isis: the delivered %s section is not a JSON object: %w", s.Root, err)
		}
		// An absent or empty root holds no export chain and no filter instance.
		tree, _ := wrapper[s.Root].(map[string]any)
		if s.Root == configRoot {
			isisCfg = tree
			continue
		}
		policyCfg = tree
	}
	chain, present, err := policy.CompileExportChain(configRoot, exportAttachment, isisCfg, policyCfg)
	if err != nil {
		return exportChainConfig{}, err
	}
	if !present {
		return exportChainConfig{}, nil
	}
	return exportChainConfig{chain: chain}, nil
}
