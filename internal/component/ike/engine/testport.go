// Design: plan/learned/745-ipsec-10-cli-diag.md -- IKE engine addressing seam
//
// ze.test.ike.port is a runtime-only env var for the test infrastructure,
// mirroring ze.test.bgp.port (internal/component/bgp/config/loader_create.go):
// it reroutes both the IKE listen socket and peer dial addresses so two
// unprivileged local daemons can negotiate IKEv2 on a high port. Production
// deployments never set it and keep the RFC 3947/4306 well-known UDP 500.

package engine

import (
	"strconv"

	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	envKeyIKEPort      = "ze.test.ike.port"
	envKeyIKEDataplane = "ze.test.ike.dataplane"
)

var _ = coreenv.MustRegister(coreenv.EnvEntry{
	Key:         envKeyIKEPort,
	Type:        "int",
	Default:     "",
	Description: "IKE listen/dial port override (test infrastructure)",
	Private:     true,
})

var _ = coreenv.MustRegister(coreenv.EnvEntry{
	Key:         envKeyIKEDataplane,
	Type:        "string",
	Default:     "",
	Description: "IKE dataplane backend override, e.g. noop (test infrastructure)",
	Private:     true,
})

// ikeDataplaneFn reads the backend override; swapped in unit tests.
var ikeDataplaneFn = func() string { return coreenv.Get(envKeyIKEDataplane) }

// ikeDataplaneName picks the dataplane backend: production always uses xfrm;
// unprivileged control-plane tests select noop via ze.test.ike.dataplane
// (EPERM on real xfrm stays fatal by design -- engine/child.go).
func ikeDataplaneName() string {
	if v := ikeDataplaneFn(); v != "" {
		return v
	}
	return "xfrm"
}

// ikeTestPortFn reads the override; swapped in unit tests.
var ikeTestPortFn = func() string { return coreenv.Get(envKeyIKEPort) }

// ikeListenHost picks the listen bind host. Production behavior is unchanged:
// the interface-resolved address when configured, else wildcard. Only under
// the ze.test.ike.port override does a peer local-address change the bind
// host -- that is what lets two unprivileged local daemons share one port
// knob by binding their own loopback addresses (127.0.0.1 vs 127.0.0.2).
func ikeListenHost(interfaceHost, peerLocalAddress string) string {
	if interfaceHost != "" {
		return interfaceHost
	}
	if ikeTestPortFn() != "" && peerLocalAddress != "" {
		return peerLocalAddress
	}
	return "0.0.0.0"
}

// ikeAddr renders host:port for IKE listen and dial addresses, honoring the
// ze.test.ike.port override and falling back to the well-known port 500 when
// the override is absent or not a valid port.
func ikeAddr(host string) string {
	port := uint16(500)
	if v := ikeTestPortFn(); v != "" {
		if p, err := strconv.ParseUint(v, 10, 16); err == nil && p > 0 {
			port = uint16(p)
		}
	}
	var tb textbuf.Buffer
	return tb.HostPortN(host, port).String()
}
