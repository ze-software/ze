// Design: docs/architecture/firewall/backend-command-dispatch.md -- VPP health check

package ifacevpp

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/ze-software/ze/internal/core/health"
)

const vppSocketPath = "/run/vpp/api.sock"

// RegisterHealthCheck registers the VPP health check with the default registry.
func RegisterHealthCheck() {
	health.Register("vpp", checkVPPHealth)
}

func checkVPPHealth() (health.Status, string) {
	if _, err := os.Stat(vppSocketPath); err != nil {
		return health.StatusHealthy, ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", vppSocketPath)
	if err != nil {
		return health.StatusDown, "VPP API socket unreachable"
	}
	if closeErr := conn.Close(); closeErr != nil {
		return health.StatusDegraded, "VPP API socket close error"
	}
	return health.StatusHealthy, ""
}
