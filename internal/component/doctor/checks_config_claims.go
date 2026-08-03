// Design: docs/architecture/config/yang-config-design.md — config claim model
// Overview: doctor.go — runChecks calls checkConfigClaims
// Related: internal/component/config/claims — the claim semantics and allowlist
//
// Config claim check: report config the daemon stores and delivers to nobody.
//
// Server.reloadConfig selects plugins by Registration.WantsConfigRoots
// (internal/component/plugin/server/reload.go). When nothing matches, it logs
// Info "config reload: no affected plugins, updating config" and calls
// SetConfigTree. The config is accepted and the operator sees no sign that the
// feature it configures is not running. This check is the operator-visible
// version of that log line.
//
// The build-time gate (internal/component/plugin/all/config_claims_test.go)
// judges the SCHEMA. This judges one running config on one build, so it also
// catches a root whose plugin is compiled out or failed to load.

package doctor

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/claims"
	schemacli "github.com/ze-software/ze/internal/component/config/schema/cli"
	pluginregistry "github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	// diagnosticConfigRootUnclaimed marks a configured subtree that no plugin
	// and no hub handler receives.
	diagnosticConfigRootUnclaimed = "doctor-config-root-unclaimed"
	// diagnosticConfigClaimsUnavailable marks the check being unable to run.
	diagnosticConfigClaimsUnavailable = "doctor-config-claims-unavailable"
)

// checkConfigClaims resolves the live claim inventory and judges the config
// tree against it. This is the entry point runChecks calls.
func checkConfigClaims(tree *config.Tree) []diagnostic.Diagnostic {
	cs := claims.FromConfigRoots(pluginregistry.ConfigRootsMap())

	// The hub delivers a config path to a subsystem through
	// SchemaRegistry.FindHandler, so a handler path claims as much as a plugin
	// config root does. A failure to build the registry is reported rather
	// than treated as "no handlers": that would turn a working root into a
	// warning.
	handlers, err := schemacli.ConfigHandlerPaths()
	if err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     diagnosticConfigClaimsUnavailable,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("cannot read the schema handler paths, so config delivery was not checked: ").Err(err).String(),
		}}
	}
	cs = append(cs, claims.FromHubHandlers(handlers)...)

	allow, err := claims.Allowlist()
	if err != nil {
		var tb textbuf.Buffer
		return []diagnostic.Diagnostic{{
			Code:     diagnosticConfigClaimsUnavailable,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("cannot read the config claim allowlist, so config delivery was not checked: ").Err(err).String(),
		}}
	}

	return configClaimDiagnostics(tree, cs, allow)
}

// configClaimDiagnostics compares one config tree against a claim inventory.
//
// It fails closed on an empty claim inventory: with nothing claiming anything,
// every root looks undelivered, and reporting them all would be noise that says
// nothing. The check reports that it could not run instead.
func configClaimDiagnostics(tree *config.Tree, cs []claims.Claim, allow []claims.Allow) []diagnostic.Diagnostic {
	if tree == nil {
		return nil
	}
	if len(cs) == 0 {
		return []diagnostic.Diagnostic{{
			Code:     diagnosticConfigClaimsUnavailable,
			Severity: diagnostic.SeverityWarning,
			Message:  "no plugin declares a config root in this build, so config delivery could not be checked",
		}}
	}

	findings := claims.AuditConfigured(claims.FromConfigTree(tree), cs, allow)

	var tb textbuf.Buffer
	diags := make([]diagnostic.Diagnostic, 0, len(findings))
	for _, f := range findings {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     diagnosticConfigRootUnclaimed,
			Severity: diagnostic.SeverityWarning,
			Message: tb.Reset().Str("config under ").Str(f.Path).
				Str(" is stored but delivered to no plugin and no handler: it has no effect. ").
				Str("Either the owning plugin is not built into this binary or did not load, or its config root is missing.").String(),
		})
	}
	return diags
}
