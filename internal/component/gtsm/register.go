// Design: ai/patterns/registration.md -- Doctor Check Registry
// Overview: doctor.go -- the check this file installs, and the seam the reactor fills
//
// This package is called by the BGP reactor rather than discovered through
// the component registry, so it registers its readiness check and its
// standalone data-plane feature declaration directly from init(). A refusal is
// a programmer error in the table beside the call, and the daemon must not
// start short of what it declares.

package gtsm

import (
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configvalue"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// gtsmTTLLeaves lists the BGP connection ttl leaves that program GTSM kernel
// state when nonzero, in deterministic evaluation order.
var gtsmTTLLeaves = [...]string{"max", "set", "min"}

func init() {
	if err := diagnostic.RegisterDoctorCheck(kernelStateDoctorCheck()); err != nil {
		panic("BUG: gtsm: doctor check registration failed: " + err.Error())
	}
	if err := registry.RegisterDataPlaneFeature(registry.DataPlaneFeature{
		Name:       filterOwner,
		ConfigPath: "bgp",
		Why:        "GTSM programs Linux kernel host routes and a kernel nftables drop table",
		Planes:     []string{"kernel"},
		InUseKind:  registry.DataPlaneInUsePredicate,
		InUse:      gtsmDataPlaneInUse,
	}); err != nil {
		panic("BUG: gtsm: data-plane feature registration failed: " + err.Error())
	}
}

// gtsmDataPlaneInUse reports whether any BGP peer or peer group in tree sets a
// nonzero `ttl max`, `ttl set`, or `ttl min` leaf, which is the condition under
// which gtsm.SetPeers writes kernel host routes or the ze_gtsm nftables table.
func gtsmDataPlaneInUse(tree map[string]any) registry.DataPlaneFeatureUse {
	bgpSection := configvalue.Section("bgp", tree)
	if bgpSection == nil {
		return registry.DataPlaneFeatureUse{}
	}
	for _, peer := range configvalue.ListEntries(bgpSection["peer"]) {
		if leaf, ok := nonzeroTTLLeaf(peer.Fields); ok {
			return gtsmFeatureUse("bgp peer ", peer.Key, "", leaf)
		}
	}
	for _, group := range configvalue.ListEntries(bgpSection["group"]) {
		if leaf, ok := nonzeroTTLLeaf(group.Fields); ok {
			return gtsmFeatureUse("bgp group ", group.Key, "", leaf)
		}
		for _, peer := range configvalue.ListEntries(group.Fields["peer"]) {
			if leaf, ok := nonzeroTTLLeaf(peer.Fields); ok {
				return gtsmFeatureUse("bgp group ", group.Key, peer.Key, leaf)
			}
		}
	}
	return registry.DataPlaneFeatureUse{}
}

// nonzeroTTLLeaf reports the first leaf among max, set, min under scope's
// connection/ttl container whose configured value is greater than zero.
func nonzeroTTLLeaf(scope map[string]any) (string, bool) {
	conn, ok := scope["connection"].(map[string]any)
	if !ok {
		return "", false
	}
	ttl, ok := conn["ttl"].(map[string]any)
	if !ok {
		return "", false
	}
	for _, leaf := range gtsmTTLLeaves {
		raw, present := ttl[leaf]
		if !present {
			continue
		}
		value, valid := configvalue.Int(raw)
		if !valid {
			continue
		}
		if value > 0 {
			return leaf, true
		}
	}
	return "", false
}

// gtsmFeatureUse formats the operator config path and delete fix for a nonzero
// BGP ttl leaf.
func gtsmFeatureUse(prefix, owner, member, leaf string) registry.DataPlaneFeatureUse {
	var tb textbuf.Buffer
	tb.Str(prefix).Str(owner)
	if member != "" {
		tb.Str(" peer ").Str(member)
	}
	tb.Str(" connection ttl ").Str(leaf)
	path := tb.String()

	tb.Reset()
	tb.Str("delete ").Str(path)
	return registry.DataPlaneFeatureUse{
		InUse:      true,
		ConfigPath: path,
		SecondFix:  tb.String(),
	}
}
