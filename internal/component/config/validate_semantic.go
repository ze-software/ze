// Design: docs/architecture/config/syntax.md -- semantic validation for offline checks
// Related: doctor.go -- the doctor check that runs this validator

package config

import (
	"errors"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// ValidateSemantics runs side-effect-free semantic validators on a parsed tree.
// It surfaces MCP, gNMI, plugin, and hub configuration errors as diagnostics
// without starting plugins or applying config. BGP validation is excluded
// because it lives in a separate package that would create a circular import.
func ValidateSemantics(tree *Tree) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	// A non-loopback MCP listener needs authentication and TLS, or the daemon
	// refuses to boot (MCPExposure, the one rule the boot guard applies). Each
	// exposure is reported with its reason and fix, as gNMI's are below.
	if mcpCfg, ok := ExtractMCPConfig(tree); ok {
		if err := mcpCfg.Validate(); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeConfigMCPInvalid,
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
		for _, refusal := range mcpCfg.Refusals() {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeConfigMCPInvalid,
				Severity: diagnostic.SeverityError,
				Message:  refusal.Problem(),
				Help:     refusal.Why(),
				Fix:      refusal.Fix(),
			})
		}
	}

	// A non-loopback gNMI listener needs a token and TLS, or the daemon refuses
	// to boot (GNMIExposure, the one rule the boot guard applies). This reports
	// each exposure offline, one finding per breach, so `ze doctor` and `ze
	// config validate` see what boot would refuse.
	if gnmiCfg, ok := ExtractGNMIConfig(tree); ok {
		for _, refusal := range gnmiCfg.Refusals() {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeConfigGNMIInvalid,
				Severity: diagnostic.SeverityError,
				Message:  refusal.Problem(),
				Help:     refusal.Why(),
				Fix:      refusal.Fix(),
			})
		}
	}

	// A failure relayed from the bgp section's plugin extractor is skipped: it
	// is the bgp tree failing to resolve (ResolveBGPTree), which the bgp peer
	// doctor check reports from the same resolver. That check is registered by
	// the package that registers the bgp extractor, so whenever this copy can
	// exist the other one is reported, and listing both told the operator about
	// one fault twice.
	for _, err := range VerifyPluginConfig(tree) {
		if extractErr, isExtraction := errors.AsType[*PluginExtractionError](err); isExtraction && extractErr.Section == string(ConfigTypeBGP) {
			continue
		}
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeConfigPluginVerify,
			Severity: diagnostic.SeverityError,
			Message:  err.Error(),
		})
	}

	if tree.GetContainer("bus") != nil {
		if _, err := ExtractHubConfig(tree); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeConfigHubInvalid,
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
	}

	return diags
}
