// Design: docs/architecture/config/syntax.md -- semantic validation for offline checks
// Related: doctor.go -- the doctor check that runs this validator

package config

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// ValidateSemantics runs side-effect-free semantic validators on a parsed tree.
// It surfaces MCP, gNMI, plugin, and hub configuration errors as diagnostics
// without starting plugins or applying config. BGP validation is excluded
// because it lives in a separate package that would create a circular import.
func ValidateSemantics(tree *Tree) []diagnostic.Diagnostic {
	var diags []diagnostic.Diagnostic

	if mcpCfg, ok := ExtractMCPConfig(tree); ok {
		if err := mcpCfg.Validate(); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeConfigMCPInvalid,
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
	}

	// gNMI serves Get AND Set, and its interceptors are installed only when a
	// token is set, so a tokenless non-loopback listener is an unauthenticated
	// config-mutation surface. The boot guard refuses it; this reports the same
	// exposure offline so `ze doctor` and `ze config validate` see it too.
	if gnmiCfg, ok := ExtractGNMIConfig(tree); ok {
		if err := gnmiCfg.Validate(); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     diagnostic.CodeConfigGNMIInvalid,
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
	}

	for _, err := range VerifyPluginConfig(tree) {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnostic.CodeConfigPluginVerify,
			Severity: diagnostic.SeverityError,
			Message:  err.Error(),
		})
	}

	if tree.GetContainer("plugin") != nil {
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
