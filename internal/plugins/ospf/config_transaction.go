// Design: docs/architecture/ospf/ospf-4-component-config.md -- one transaction owns runtime and export undo.
// Related: register.go -- runOSPFEngine installs this value through sdk.ConfigTx.
package ospf

import "github.com/ze-software/ze/pkg/plugin/sdk"

// ospfTransactionConfig is one parsed configuration, including the export chain
// compiled against its delivered policy section. Its zero describes an idle
// plugin with no export chain; installed values come from the section parser.
type ospfTransactionConfig struct {
	cfg    ospfConfig
	export exportChainConfig
}

// parseOSPFTransactionSections changes no state. A delivered empty OSPF section
// means removal and therefore remains present; no delivered OSPF root means the
// transaction has nothing to install. ConfigReads supplies the matching policy,
// BFD and interface roots whenever a change in any of them reaches OSPF.
func parseOSPFTransactionSections(sections []sdk.ConfigSection) (ospfTransactionConfig, bool, error) {
	present := false
	for _, section := range sections {
		if section.Root == Namespace {
			present = true
			break
		}
	}
	if !present {
		return ospfTransactionConfig{}, false, nil
	}
	delivered := sdkConfigSections(sections)
	cfg, err := parseOSPFConfig(delivered, systemRouterIDSource{})
	if err != nil {
		return ospfTransactionConfig{}, true, err
	}
	if err := validateConfig(cfg); err != nil {
		return ospfTransactionConfig{}, true, err
	}
	if err := checkBFDOptIns(cfg, delivered); err != nil {
		return ospfTransactionConfig{}, true, err
	}
	if err := checkUnnumberedNetworkTypes(cfg, delivered); err != nil {
		return ospfTransactionConfig{}, true, err
	}
	if err := validateSRConfig(delivered); err != nil {
		return ospfTransactionConfig{}, true, err
	}
	export, err := parseExportChain(delivered)
	if err != nil {
		return ospfTransactionConfig{}, true, err
	}
	return ospfTransactionConfig{cfg: cfg, export: export}, true, nil
}
