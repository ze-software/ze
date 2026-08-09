// Design: docs/architecture/dns/geodns.md -- show geodns status command

package geodns

import (
	"net"
	"strconv"

	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// handleShowGeoDNS answers `show geodns`. It reads the published resolver
// snapshot (the same atomic state the query path uses) so the reported status
// never drifts from what the server is actually serving. The (always-nil) error
// return is mandated by the pluginserver.RPCRegistration.Handler signature.
//
//nolint:unparam // handler signature fixed by pluginserver.RPCRegistration
func handleShowGeoDNS(_ *pluginserver.CommandContext, _ []string) (*plugin.Response, error) {
	st := loadState()
	if st == nil {
		return &plugin.Response{Status: plugin.StatusDone, Data: plugin.Map{"enabled": false}}, nil
	}

	listeners := make([]string, len(st.cfg.Listeners))
	for i, l := range st.cfg.Listeners {
		listeners[i] = net.JoinHostPort(l.IP.String(), strconv.Itoa(int(l.Port)))
	}

	data := plugin.Map{
		"enabled":          st.cfg.Enabled,
		"listeners":        listeners,
		"client-ip-source": st.cfg.ClientIPSource,
		"zones":            st.cfg.Zones,
		"nameserver-count": len(st.cfg.Nameservers),
		"host-sets":        len(st.cfg.HostSets),
		"sources":          len(st.cfg.Sources),
		"soa-serial":       st.serial,
	}
	return &plugin.Response{Status: plugin.StatusDone, Data: data}, nil
}
