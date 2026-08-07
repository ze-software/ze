// Design: docs/architecture/config/syntax.md -- semantic validation for offline checks

package config

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
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
				Code:     "config-mcp-invalid",
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
	} else if names := MCPServersMissingPort(tree); len(names) > 0 {
		// Reported OUTSIDE the ok gate on purpose: ExtractMCPConfig returns
		// ok=false for exactly this config, so the Validate call above is skipped
		// and the operator would get a listener that never starts and no message
		// at all (ai/rules/evidence.md: a guard that neither denies nor speaks
		// does not exist). MCPServersMissingPort answers only the missing-port
		// cause, never the absent or disabled block that share that ok=false.
		var tb textbuf.Buffer
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "config-mcp-invalid",
			Severity: diagnostic.SeverityError,
			Message:  tb.Str("environment.mcp: server ").Join(names, ", ").Str(MCPMissingPortAdvice).String(),
			Path:     "environment/mcp/server",
		})
	}

	// gNMI serves Get AND Set, and its interceptors are installed only when a
	// token is set, so a tokenless non-loopback listener is an unauthenticated
	// config-mutation surface. The boot guard refuses it; this reports the same
	// exposure offline so `ze doctor` and `ze config validate` see it too.
	if gnmiCfg, ok := ExtractGNMIConfig(tree); ok {
		if err := gnmiCfg.Validate(); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "config-gnmi-invalid",
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
	}

	for _, err := range VerifyPluginConfig(tree) {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     "config-plugin-verify",
			Severity: diagnostic.SeverityError,
			Message:  err.Error(),
		})
	}

	if tree.GetContainer("plugin") != nil {
		if _, err := ExtractHubConfig(tree); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     "config-hub-invalid",
				Severity: diagnostic.SeverityError,
				Message:  err.Error(),
			})
		}
	}

	return diags
}
