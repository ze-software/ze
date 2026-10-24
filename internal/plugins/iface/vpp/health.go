// Design: docs/architecture/firewall/backend-command-dispatch.md -- VPP health check
// Related: register.go -- RegisterHealthCheck called from init()
// Related: doctor_api.go -- checkVPPAPISocket, the offline twin of this check

package ifacevpp

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// vppSocketPath is the YANG default of `vpp { api-socket }`, read by the
// offline doctor check when the config names no socket.
const vppSocketPath = "/run/vpp/api.sock"

// vppHealthFix is the command every down answer of the health row names.
const vppHealthFix = "; run `ze doctor` to see why VPP is not answering"

// activeVPPSocket is the API socket the running vpp component dials, false
// when that component is not running. A var so the unit tests drive both.
var activeVPPSocket = func() (string, bool) {
	c := vppcomp.GetActiveConnector()
	if c == nil {
		return "", false
	}
	return c.Socket(), true
}

// vppHealthApplies reports whether VPP is the loaded interface backend, the
// only state in which the vpp health row has anything to measure.
func vppHealthApplies() bool {
	return iface.ActiveBackendName() == backendVPP
}

// RegisterHealthCheck registers the VPP health check with the default
// registry. The row exists only while VPP is the interface backend: on a host
// that does not run VPP there is nothing to measure, and a healthy row there
// would be an answer nobody measured.
func RegisterHealthCheck() {
	health.RegisterWhen(backendVPP, vppHealthApplies, checkVPPHealth)
}

// checkVPPHealth answers for the VPP data plane while VPP is the interface
// backend. It dials the socket the vpp component dials, so the answer is about
// the configured `vpp { api-socket }`. A missing component, a missing socket
// and a refused dial are each down: no interface or route can be programmed.
func checkVPPHealth() (health.Status, string) {
	socket, running := activeVPPSocket()
	if !running {
		return health.StatusDown, "VPP is the interface backend, but the vpp component is not running, " +
			"so no interface can be programmed" + vppHealthFix
	}
	var tb textbuf.Buffer
	if _, err := os.Stat(socket); errors.Is(err, fs.ErrNotExist) {
		return health.StatusDown, tb.Str("VPP API socket ").Str(socket).
			Str(" does not exist, so VPP is not running and no interface can be programmed").
			Str(vppHealthFix).String()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", socket)
	if err != nil {
		return health.StatusDown, tb.Str("VPP API socket ").Str(socket).
			Str(" refuses connections, so no interface can be programmed (").Err(err).Str(")").
			Str(vppHealthFix).String()
	}
	if closeErr := conn.Close(); closeErr != nil {
		return health.StatusDegraded, tb.Str("VPP API socket ").Str(socket).
			Str(" accepted a connection that then failed to close (").Err(closeErr).Str(")").
			Str(vppHealthFix).String()
	}
	return health.StatusHealthy, ""
}
