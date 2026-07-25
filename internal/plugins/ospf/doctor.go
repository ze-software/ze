// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- OSPF config-sanity doctor checks.
// Related: config.go -- parseOSPFConfig the check reuses to resolve router-id + areas.
// Related: transport -- the SEPARATE raw-socket check (ospf-3 owns doctor-ospf-raw-socket).
// RFC: rfc/short/rfc5880.md, rfc/short/rfc5881.md -- BFD (the bfd-plugin-absent informational check).
//
// This file owns ONLY the two OSPF config-sanity doctor codes (spec-ospf-13 AC-14):
// a configured OSPF block with no derivable router-id, and an enabled interface bound to
// an area not declared under `areas`. The CAP_NET_RAW raw-socket check + its
// doctor-ospf-raw-socket code are owned by ospf-3; this file MUST NOT re-register them.

package ospf

import (
	"encoding/json"

	"github.com/ze-software/ze/internal/component/bfd/api"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

const (
	// codeOSPFRouterIDMissing fires when `ospf {}` is configured but no router-id is set
	// and none can be derived from an interface IPv4 address (the engine cannot originate
	// LSAs without a router-id).
	codeOSPFRouterIDMissing = "doctor-ospf-router-id-missing"
	// codeOSPFInterfaceAreaUnbound fires when an OSPF interface references an area that is
	// not declared under `areas` (the interface forms no adjacency).
	codeOSPFInterfaceAreaUnbound = "doctor-ospf-interface-area-unbound"
	// codeOSPFBFDPluginAbsent fires (informational) when BFD is enabled on an OSPF interface
	// but the BFD plugin is not loaded in this process (api.GetService is nil): OSPF then runs
	// on the Hello/Dead timers alone. RFC 5880 / RFC 5881.
	codeOSPFBFDPluginAbsent = "doctor-ospf-bfd-plugin-absent"
	// codeOSPFGracefulRestartNVS fires when OSPF Graceful Restart's restarter is enabled but
	// the non-volatile restart-fact store cannot be opened (RFC 3623 sec 2.1). Without it a
	// planned restart cannot persist its grace deadline, so non-stop forwarding is defeated.
	codeOSPFGracefulRestartNVS = "doctor-ospf-graceful-restart-nvs"
)

// checkOSPFGracefulRestartNVS is the registered doctor check for the Graceful Restart NVS
// runtime dependency (spec-ospf-ext-9). It fires only when the restarter is enabled and the
// ZeFS blob store the restart fact persists to cannot be opened.
func checkOSPFGracefulRestartNVS(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
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
		return nil
	}
	return grNVSDiagnostics(cfg, grStoreOpenable())
}

// grStoreOpenable reports whether the GR non-volatile restart-fact store can be opened.
func grStoreOpenable() bool {
	store, ok := openGRStore()
	if !ok {
		return false
	}
	if err := store.Close(); err != nil {
		return false
	}
	return true
}

// grNVSDiagnostics is the pure decision for the Graceful Restart NVS readiness check: it warns
// only when the restarter is enabled and the non-volatile store is not openable.
func grNVSDiagnostics(cfg ospfConfig, storeOpenable bool) []diagnostic.Diagnostic {
	if !cfg.GracefulRestart.restarterEnabled() || storeOpenable {
		return nil
	}
	return []diagnostic.Diagnostic{{
		Code:     codeOSPFGracefulRestartNVS,
		Severity: diagnostic.SeverityWarning,
		Message:  "ospf graceful-restart restarter is enabled but the non-volatile restart-fact store cannot be opened; a planned restart cannot persist its grace deadline and would reconverge normally",
	}}
}

// bfdEnabledInterfaceCount returns how many interfaces (across both address families) opt
// into BFD, used by the informational BFD-plugin-absent doctor check.
func bfdEnabledInterfaceCount(cfg ospfConfig) int {
	count := 0
	for _, ic := range cfg.Interfaces {
		if ic.BFD.Enabled {
			count++
		}
	}
	if cfg.V6 != nil {
		count += bfdEnabledInterfaceCount(*cfg.V6)
	}
	return count
}

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
	// Informational: BFD enabled on an interface but the BFD plugin is not loaded in this
	// process (api.GetService nil). OSPF still forms adjacencies on the Hello/Dead timers;
	// only sub-second BFD failure detection is unavailable until the bfd plugin runs.
	if n := bfdEnabledInterfaceCount(cfg); n > 0 && api.GetService() == nil {
		var tb textbuf.Buffer
		tb.Str("bfd is enabled on ").Int(int64(n)).Str(" ospf interface(s) but the BFD plugin is not loaded; ospf runs on the hello/dead timers")
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeOSPFBFDPluginAbsent,
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
