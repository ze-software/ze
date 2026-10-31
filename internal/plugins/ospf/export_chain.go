// Design: docs/guide/redistribution.md -- the OSPF export chain over redistributed routes
// Related: redistribute/export.go -- SetExportChain and the consumer's judgment
// Related: register.go -- parses the chain at verify and installs it at configure and apply

package ospf

import (
	"encoding/json"
	"fmt"

	"github.com/ze-software/ze/internal/component/policy"
	ospfredistribute "github.com/ze-software/ze/internal/plugins/ospf/redistribute"
)

// exportAttachment is the filter attachment of `ospf filter export`.
const exportAttachment = "ospf export"

// exportChainConfig is the compiled `ospf filter export` chain of one config.
// A nil chain means the config sets no export chain.
type exportChainConfig struct {
	chain *policy.RouteChain
}

// install makes c the chain the redistribution consumer judges routes with.
func (c exportChainConfig) install() {
	if c.chain == nil {
		ospfredistribute.SetExportChain(nil)
		return
	}
	ospfredistribute.SetExportChain(c.chain)
}

// parseExportChain compiles the export chain from the ospf section against the
// filter instances of the policy section. A config with no ospf section, or no
// export container, compiles to no chain.
func parseExportChain(sections []configSection) (exportChainConfig, error) {
	var ospfCfg, policyCfg map[string]any
	for _, s := range sections {
		if s.Root != Namespace && s.Root != policy.RootName {
			continue
		}
		if s.Data == "" {
			continue
		}
		var wrapper map[string]any
		if err := json.Unmarshal([]byte(s.Data), &wrapper); err != nil {
			return exportChainConfig{}, fmt.Errorf("ospf: the delivered %s section is not a JSON object: %w", s.Root, err)
		}
		// An absent or empty root holds no export chain and no filter instance.
		tree, _ := wrapper[s.Root].(map[string]any)
		if s.Root == Namespace {
			ospfCfg = tree
			continue
		}
		policyCfg = tree
	}
	chain, present, err := policy.CompileExportChain(Namespace, exportAttachment, ospfCfg, policyCfg)
	if err != nil {
		return exportChainConfig{}, err
	}
	if !present {
		return exportChainConfig{}, nil
	}
	return exportChainConfig{chain: chain}, nil
}
