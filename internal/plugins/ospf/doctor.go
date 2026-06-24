// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- OSPF config-sanity doctor checks.
// Related: config.go -- parseOSPFConfig the check reuses to resolve router-id + areas.
// Related: transport -- the SEPARATE raw-socket check (ospf-3 owns doctor-ospf-raw-socket).
//
// This file owns ONLY the two OSPF config-sanity doctor codes (spec-ospf-13 AC-14):
// a configured OSPF block with no derivable router-id, and an enabled interface bound to
// an area not declared under `areas`. The CAP_NET_RAW raw-socket check + its
// doctor-ospf-raw-socket code are owned by ospf-3; this file MUST NOT re-register them.

package ospf

import (
	"encoding/json"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
	"codeberg.org/thomas-mangin/ze/internal/core/diagnostic"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/types"
)

const (
	// codeOSPFRouterIDMissing fires when `ospf {}` is configured but no router-id is set
	// and none can be derived from an interface IPv4 address (the engine cannot originate
	// LSAs without a router-id).
	codeOSPFRouterIDMissing = "doctor-ospf-router-id-missing"
	// codeOSPFInterfaceAreaUnbound fires when an OSPF interface references an area that is
	// not declared under `areas` (the interface forms no adjacency).
	codeOSPFInterfaceAreaUnbound = "doctor-ospf-interface-area-unbound"
)

// ospfConfigDiagnostics returns the config-sanity diagnostics for a resolved OSPF config
// (router-id resolvable, every interface bound to a declared area). It is a no-op when
// OSPF is not present.
func ospfConfigDiagnostics(cfg ospfConfig) []diagnostic.Diagnostic {
	if !cfg.present {
		return nil
	}
	var diags []diagnostic.Diagnostic
	if cfg.RouterID == (types.RouterID{}) {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeOSPFRouterIDMissing,
			Severity: diagnostic.SeverityWarning,
			Message:  "ospf is configured but has no router-id and none can be derived from an interface IPv4 address",
		})
	}
	areas := cfg.areaSet()
	for _, ic := range cfg.Interfaces {
		if _, ok := areas[ic.AreaID]; ok {
			continue
		}
		var tb textbuf.Buffer
		tb.Str("ospf interface ").Quoted(ic.Name).Str(" is bound to undeclared area ").Str(ic.AreaID.String())
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeOSPFInterfaceAreaUnbound,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.String(),
		})
	}
	return diags
}

// checkOSPFConfigSanity is the registered doctor check (spec-ospf-13 AC-14). It resolves
// the OSPF config the same way the engine does (parseOSPFConfig with the live interface
// router-id source) so the verdict matches runtime, and stays silent when OSPF is not
// configured or the config has a structural error the YANG layer already flagged.
func checkOSPFConfigSanity(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	ospfTree := tree.GetContainer("ospf")
	if ospfTree == nil {
		return nil
	}
	data, err := json.Marshal(map[string]any{"ospf": ospfTree.ToMap()})
	if err != nil {
		return nil
	}
	cfg, err := parseOSPFConfig([]configSection{{Root: "ospf", Data: string(data)}}, systemRouterIDSource{})
	if err != nil {
		return nil // a structural error is the per-leaf YANG validator's job to report.
	}
	return ospfConfigDiagnostics(cfg)
}
