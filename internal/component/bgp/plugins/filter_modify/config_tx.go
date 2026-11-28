// Design: docs/architecture/core-design.md -- route modify filter
// Related: filter_modify.go -- defsByName, the state an install replaces
// Related: config.go -- parseModifyDefsWithBGP, which builds each definition's sets and checks import conflicts

package filter_modify

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/opererr"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// parseModifySections is the shared config parser for Stage 2 configure,
// live reload verify, and offline InProcessConfigVerifier. It decodes the
// policy and bgp sections fail-closed (rejecting invalid JSON, JSON null, and
// non-object root values with structured opererr errors and without adding a
// duplicate plugin prefix), then parses modify definitions and enforces AC-4.
func parseModifySections(sections []sdk.ConfigSection) (map[string]*modifyDef, bool, error) {
	var policyTree, bgpTree map[string]any
	var sawPolicy, sawBGP bool
	for _, section := range sections {
		switch section.Root {
		case policy.RootName:
			tree, err := parseModifySectionObject(section.Data, policy.RootName)
			if err != nil {
				return nil, false, err
			}
			policyTree = tree
			sawPolicy = true
		case configRootBGP:
			tree, err := parseModifySectionObject(section.Data, configRootBGP)
			if err != nil {
				return nil, false, err
			}
			bgpTree = tree
			sawBGP = true
		}
	}
	if !sawPolicy {
		if !sawBGP {
			return nil, false, nil
		}
	}
	defs, err := parseModifyDefsWithBGP(policyTree, bgpTree)
	if err != nil {
		return nil, false, err
	}
	return defs, true, nil
}

// parseModifySectionObject decodes one delivered config section and unwraps
// its root container when present. Invalid JSON, JSON null, and non-object
// root values fail closed rather than looking like an empty configuration.
func parseModifySectionObject(data, root string) (map[string]any, error) {
	const validateFix = "ze config validate <file>"
	why := "bgp-filter-modify cannot verify modify definitions or import as-path-prepend rules without a valid " + root + " object"
	var raw map[string]any
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, opererr.Wrap(err, "the "+root+" config section is not valid JSON", why, validateFix)
	}
	if raw == nil {
		return nil, opererr.New("the "+root+" config section is null instead of a JSON object", why, validateFix)
	}
	if val, exists := raw[root]; exists {
		inner, ok := val.(map[string]any)
		if !ok {
			return nil, opererr.New("the "+root+" config section value is not a JSON object", why, validateFix)
		}
		return inner, nil
	}
	return raw, nil
}

// newModifyConfigTx is the filter's side of the config transaction: the
// modify definitions are its config, and an install stores them with one
// pointer, so a route reads the modifiers of one config and never a mix.
// Before any install none is defined and every route fails (evaluateModify).
func newModifyConfigTx() *sdk.ConfigTx[map[string]*modifyDef] {
	return sdk.NewConfigTx(map[string]*modifyDef(nil), parseModifySections, installModifyDefs)
}

// verifyModifyConfig is the plugin's side-effect-free in-process config
// verifier, run by `ze config validate` and commit without starting the
// plugin. It executes the same parser as Stage 2 configure and reload verify.
func verifyModifyConfig(sections []sdk.ConfigSection) error {
	_, _, err := parseModifySections(sections)
	return err
}

// installModifyDefs makes the definitions every later filter-update reads. It
// cannot fail.
func installModifyDefs(defs map[string]*modifyDef) error {
	defsByName.Store(&defs)
	logger().Debug("configured", "modifiers", len(defs))
	return nil
}
