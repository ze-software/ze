// Design: docs/architecture/dns/as112.md -- show as112 status command

package as112

import (
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// handleShowAS112 answers `show as112`. It reads the published state
// snapshot (the same atomic state the query path uses) so the reported
// status never drifts from what the server is actually serving. The
// (always-nil) error return is mandated by the pluginserver.RPCRegistration.Handler
// signature.
//
//nolint:unparam // handler signature fixed by pluginserver.RPCRegistration
func handleShowAS112(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	st := loadState()
	if st == nil {
		data := plugin.Map{"enabled": false}
		addRegistryStatus(data)
		return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
	}

	data := plugin.Map{
		"enabled":        st.cfg.Enabled,
		"address-family": st.cfg.AddressFamily,
		"hostname":       st.cfg.Hostname,
		"facility":       st.cfg.Facility,
		"location":       st.cfg.Location,
		"allow-from":     len(st.cfg.AllowFrom),
		"zones":          len(servedZones()),
		"soa-serial":     st.serial,
	}
	addRegistryStatus(data)

	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}

// addRegistryStatus surfaces the outcome of the last registry-triggered
// address reconcile (RegisterOwnedAddresses's async trigger,
// iface/config_apply.go's reconcileOnRegistryChange), which is otherwise
// fire-and-forget and only ever reaches a log line -- an operator has no
// other way to see a stuck address-registration failure without tailing
// logs. Reflects the whole iface registry, not just as112's own
// registration (as112 is normally its only consumer); called from both
// handleShowAS112 branches, including before as112 has ever been
// configured, since a registry failure from another consumer is visible
// regardless of as112's own enabled state.
func addRegistryStatus(data plugin.Map) {
	if ok, at, errMsg := iface.RegistryReconcileStatus(); !ok {
		data["address-registry-ok"] = false
		data["address-registry-error"] = errMsg
		data["address-registry-error-at"] = at.UTC().Format(time.RFC3339)
	} else {
		data["address-registry-ok"] = true
	}
}
