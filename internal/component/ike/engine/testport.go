// Design: docs/architecture/ike/ipsec-10-cli-diag.md -- IKE engine addressing seam
// RFC: rfc/short/rfc3948.md -- UDP encapsulation of ESP uses port 4500 (Section 2.1)
//
// This file holds the runtime-only env vars the IKE test infrastructure sets.
// Production deployments set none of them.
//
// ze.test.ike.port mirrors ze.test.bgp.port
// (internal/component/bgp/config/loader_create.go): it reroutes both the IKE
// listen socket and peer dial addresses so two unprivileged local daemons can
// negotiate IKEv2 on a high port. Production keeps the RFC 3947/4306 well-known
// UDP 500.

package engine

import (
	"fmt"
	"net"
	"strconv"

	"github.com/ze-software/ze/internal/component/ike/transport"
	coreenv "github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	envKeyIKEPort         = "ze.test.ike.port"
	envKeyIKEDataplane    = "ze.test.ike.dataplane"
	envKeyIKERekeyTSLocal = "ze.test.ike.rekey.ts.local"
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

var _ = coreenv.MustRegister(coreenv.EnvEntry{
	Key:         envKeyIKERekeyTSLocal,
	Type:        "string",
	Default:     "",
	Description: "Child SA rekey local traffic selector override (test infrastructure)",
	Private:     true,
})

// ikeRekeyTSLocalFn reads the override; swapped in unit tests.
var ikeRekeyTSLocalFn = func() string { return coreenv.Get(envKeyIKERekeyTSLocal) }

// narrowedRekeyPairs replaces the local half of a Child SA rekey proposal with the prefix
// ze.test.ike.rekey.ts.local names. It returns a nil slice and a nil error when the key is
// unset, which is every production run.
//
// It exists because one case of RFC 7296 Section 2.9.2 is otherwise unreachable between two
// Ze daemons, and it stays unreachable after spec-fixit-ipsec-peer-reload-ignored. A rekey
// proposal comes from sa.PeerCfg (proposeChildTSPayloads, rekey.go), which startPeerSession
// copies when the session starts and nothing writes again. An operator who narrows a peer's
// traffic selectors now RESTARTS that peer (peerConfigChanged, reconcile.go), which is what
// Section 2.9.2 asks for, so the narrowed selectors travel on a fresh IKE_SA_INIT and the
// old scope is gone before anything proposes against it. A live SA therefore still proposes
// selectors that cover the scope it installed. Section 2.9.2 names the opposite case, where
// "the policy was changed in a way such that the currently used SA is against the policy",
// and this key is still the only thing that produces a Ze peer in it.
// test/ipsec/ipsec-child-rekey-xfrm-narrowing.ci is the caller that sets it.
//
// It is an env var rather than a YANG leaf because it states nothing an operator would
// want: it makes a daemon propose a scope its own policy does not hold.
func narrowedRekeyPairs(pairs []tsPair) ([]tsPair, error) {
	value := ikeRekeyTSLocalFn()
	if value == "" {
		return nil, nil
	}
	_, prefix, err := net.ParseCIDR(value)
	if err != nil {
		return nil, fmt.Errorf("%s=%q: %w", envKeyIKERekeyTSLocal, value, err)
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("%s=%q: this peer proposes no traffic selector to narrow",
			envKeyIKERekeyTSLocal, value)
	}
	out := make([]tsPair, 0, len(pairs))
	for _, pair := range pairs {
		pair.I.Net = prefix
		out = append(out, pair)
	}
	return out, nil
}

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

// nattAddr renders host:port for the NAT-T listen socket.
//
// RFC 3948 Section 2.1 fixes that port at 4500 and no configuration may move it,
// so a production deployment always gets transport.NATTPort.
//
// Under the ze.test.ike.port override it takes the port AFTER the IKE one, for
// the reason the override exists at all: a well-known port is one host-wide
// resource, and the .ci suite runs several two-daemon IKE pairs at once. Every
// daemon bound 0.0.0.0:4500, one won it, and the rest logged "address already in
// use" (linux CI run 31225029268 shows both daemons of one test losing).
//
// ikeTestPort+1 is free by construction. The runner gives each test two ports
// (internal/test/runner/record_parse.go), $PORT for BGP over TCP and $PORT2 for
// IKE over UDP, and the .ci files set this override to $PORT2. The next number
// is the FOLLOWING test's $PORT, which that test binds as TCP, so no other test
// holds it as UDP.
func nattAddr(host string) string {
	port := uint16(transport.NATTPort)
	if v := ikeTestPortFn(); v != "" {
		// 65534 is the bound, not 65535: the port used is p+1, and a wrap to 0
		// would ask the kernel for an ephemeral port that no peer can address.
		if p, err := strconv.ParseUint(v, 10, 16); err == nil && p > 0 && p < 65535 {
			port = uint16(p) + 1
		}
	}
	var tb textbuf.Buffer
	return tb.HostPortN(host, port).String()
}
