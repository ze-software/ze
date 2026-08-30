// Design: docs/architecture/isis/isis-13-cli-diag-interop.md -- IS-IS config-sanity doctor checks.
// Related: config.go -- applyTree/Config the check reuses to read NET/system-id
// Related: transport/doctor.go -- the SEPARATE raw-socket check (isis-3 owns it)
//
// This file owns ONLY the IS-IS config-sanity doctor CHECK FUNCTION (spec-isis-13):
// doctor-isis-net-missing (the `isis` block is present but carries no `net`) and
// doctor-isis-system-id-mismatch (an explicit `system-id` does not match the
// System ID derivable from the first NET). The CAP_NET_RAW / raw-socket check
// and its doctor-isis-raw-socket code are OWNED by isis-3 (transport/doctor.go)
// and only surfaced by `ze doctor`; this file MUST NOT re-register them
// (ai/rules/repo-maintenance.md, one code one owner). The check gates on the `isis`
// container being present so a BGP-only node sees no spurious IS-IS warning. The
// init() that registers this check lives in register.go (registerISISDoctor).

package isis

import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

const (
	// codeNETMissing fires when `isis {}` is configured with no `net` leaf. IS-IS
	// cannot derive a System ID or originate LSPs without a NET (config.go AC-3).
	codeNETMissing = "doctor-isis-net-missing"
	// codeSystemIDMismatch fires when an explicit `system-id` disagrees with the
	// System ID embedded in the first NET (config.go AC-4/AC-9).
	codeSystemIDMismatch = "doctor-isis-system-id-mismatch"
)

// checkISISConfigSanity validates the IS-IS config block for the two failure
// modes an operator most commonly hits before the engine even starts: a missing
// NET and an inconsistent explicit System ID. It is a no-op when IS-IS is not
// configured (no `isis` container), so a node that does not run IS-IS gets no
// warning (R-4). It reads the config tree through the same applyTree path the
// engine uses so the doctor verdict matches what the engine would resolve.
func checkISISConfigSanity(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	isisTree := tree.GetContainer(configRoot)
	if isisTree == nil {
		return nil // IS-IS not configured: nothing to check.
	}

	cfg := Config{
		Level:              DefaultLevel,
		LSPLifetime:        DefaultLSPLifetime,
		LSPRefreshInterval: DefaultLSPRefreshInterval,
	}
	// applyTree parses leaves the same way OnConfigVerify does. A structural error
	// (a malformed NET/system-id) is the per-leaf YANG validator's job to report;
	// here a parse error means we cannot reason about NET/system-id, so we emit no
	// config-sanity diagnostic (the YANG layer already flagged the bad leaf).
	if err := applyTree(&cfg, isisTree.ToMap()); err != nil {
		return nil
	}

	var diags []diagnostic.Diagnostic
	if len(cfg.NETs) == 0 {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeNETMissing,
			Severity: diagnostic.SeverityError,
			Message:  "IS-IS is configured but no net is set; add at least one net (e.g. 49.0001.0000.0000.0001.00)",
		})
		// With no NET there is no System ID to compare against, so stop here.
		return diags
	}
	if cfg.systemIDFromConfig && cfg.SystemID != cfg.NETs[0].SystemID() {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeSystemIDMismatch,
			Severity: diagnostic.SeverityError,
			Message:  "IS-IS system-id does not match the System ID embedded in the first net; remove system-id or align it with the net",
		})
	}
	return diags
}
