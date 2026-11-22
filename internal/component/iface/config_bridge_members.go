// Design: docs/architecture/iface/logical-name-resolution.md -- a configured-interface name binds through its entry's selector
// Related: config_apply.go -- applyConfigPass, which calls this in Phase 2a and after the Phase 2 VLAN loop
// Related: interface_targets.go -- interfaceTargets, which binds each member name

package iface

import (
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// enslaveBridgeMembers enslaves the members of every enabled bridge in cfg
// whose kind matches units: the members that name an entry when units is
// false, the members that name a VLAN unit when it is true. applyConfigPass
// runs the first pass in Phase 2a, before the per-entry loop sets properties,
// and the second after that loop, because the loop is what creates a VLAN and
// a unit member names the VLAN's device.
//
// Each member binds through targets: an ethernet entry through its selector,
// a VLAN unit through its parent's. An unbound member is skipped with a
// warning, as every unbound setting defers, and a member already in its
// bridge is skipped with no step (portInBridge). A refused step is answered by
// record, whose result the caller returns: stopped is true exactly when it ran.
func enslaveBridgeMembers(cfg *ifaceConfig, targets interfaceTargets, units bool, b Backend,
	journal *sdk.Journal, record func(string, error) ([]error, bool),
) (errs []error, deferred, stopped bool) {
	log := loggerPtr.Load()
	for _, e := range cfg.Bridge {
		if e.Disable {
			continue
		}
		for _, member := range e.Members {
			if _, isUnit := targets.unitNamed(member); isUnit != units {
				continue
			}
			port, bound := targets.device(member)
			if !bound {
				log.Warn("iface config: no present device answers this bridge member's hardware selector, enslaving deferred",
					"bridge", e.Name, "member", member, "code", diagnostic.CodeDoctorIfaceSelectorUnmatched)
				continue
			}
			bridge := e.Name
			// A port already in this bridge gets no step: enslaving it
			// again needs CAP_NET_ADMIN for a change that changes nothing,
			// and the undo would take out of the bridge a port this apply
			// did not put in.
			if portInBridge(b, bridge, port) {
				continue
			}
			if err := applyBackendStep(journal, func() error {
				return b.BridgeAddPort(bridge, port)
			}, func() error {
				return b.BridgeDelPort(port)
			}); err != nil {
				var bPort textbuf.Buffer
				errs, deferred = record(bPort.Str("bridge ").Str(bridge).Str(" add port ").Str(port).String(), err)
				return errs, deferred, true
			}
		}
	}
	return nil, false, false
}
