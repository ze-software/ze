// Design: docs/guide/route-policy.md -- the IGP export chain over redistributed routes
// Related: rib_compile.go -- CompileRouteChain, which this reads the chain for
// Related: route_chain_path.go -- MustRegisterRouteChainPath, which ExportChainPath feeds

package policy

import (
	"fmt"

	"github.com/ze-software/ze/internal/core/configvalue"
)

// exportDirection is the filter container that holds an export chain.
const exportDirection = "export"

// ExportChainPath is the data path of the filters leaf-list of root's export
// chain, `<root> filter export filters`. The owner registers it with
// MustRegisterRouteChainPath so `ze config validate` checks the names offline.
func ExportChainPath(root string) []string {
	return []string{root, "filter", exportDirection, "filters"}
}

// CompileExportChain compiles the export chain configured at `<root> filter
// export` in rootCfg, the protocol's own subtree, at attachment, against the
// filter instances under policyCfg. It answers present false when the root
// configures no export chain: every redistributed route is then advertised
// unchanged. A chain with no valid final, or naming a filter the attachment
// refuses, is an error naming the fix.
func CompileExportChain(root, attachment string, rootCfg, policyCfg map[string]any) (chain *RouteChain, present bool, err error) {
	filterCfg, ok := rootCfg["filter"].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	exportCfg, ok := filterCfg[exportDirection].(map[string]any)
	if !ok {
		return nil, false, nil
	}
	finalText, _ := exportCfg["final"].(string) // a missing final is refused below
	var final Final
	switch finalText {
	case "accept":
		final = FinalAccept
	case "reject":
		final = FinalReject
	default:
		return nil, false, fmt.Errorf("%s/filter/export/final: missing or invalid final %q; "+
			"every export policy requires a final decision; write export { filters [ NAME ]; final accept; } or final reject",
			root, finalText)
	}
	chain, err = CompileRouteChain(attachment, configvalue.LeafList(exportCfg["filters"]), final, policyCfg)
	if err != nil {
		return nil, false, fmt.Errorf("%s/filter/export: %w", root, err)
	}
	return chain, true, nil
}
