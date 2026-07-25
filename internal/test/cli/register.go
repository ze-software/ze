// Design: docs/architecture/testing/ci-format.md -- ze-test root handler registration

package cli

import (
	"github.com/ze-software/ze/internal/test/mock/cymru"
	"github.com/ze-software/ze/internal/test/mock/irr"
	"github.com/ze-software/ze/internal/test/mock/peeringdb"
	radiusmock "github.com/ze-software/ze/internal/test/mock/radius"
	"github.com/ze-software/ze/internal/test/mock/rpki"
	"github.com/ze-software/ze/internal/test/mock/rtr"
	"github.com/ze-software/ze/internal/test/mock/tacacs"
)

func init() {
	// CI test runners
	registerCIRoot("appliance", "appliance", "appliance", "Run appliance CLI functional tests (.ci files in test/appliance/).\nCovers ze appliance build/iso/list/help surfaces and serial login (offline; gok-dependent steps model tool absence).", 0)
	registerCIRoot("firewall", "firewall", "firewall", "Run firewall functional tests (.ci files in test/firewall/).\nCovers component reactor wiring: boot-time parse -> validate -> Apply.", 0)
	registerCIRoot("flow-export", "flow-export", "flow-export", "Run flow-export functional tests (.ci files in test/flow-export/).\nCovers sFlow v5, NetFlow v9, and IPFIX counter export over UDP, the show flow export handler, packet-sampling wiring, and reload-time reconfiguration.", 0)
	registerCIRoot("install", "install", "install", "Run install provisioning functional tests (.ci files in test/install/).\nTests ze install CLI, config validation, and provisioning server setup.", 0)
	registerCIRoot("ipsec", "ipsec", "IPsec", "Run IPsec/IKEv2 functional tests (.ci files in test/ipsec/).\nCovers vpn ipsec show/monitor/clear surfaces and SA lifecycle scenarios.", 0)
	registerCIRoot("isis", "isis", "isis", "Run IS-IS functional tests (.ci files in test/isis/).\nCovers single-daemon boot: config parse -> YANG -> NET/system-id validation -> component startup, and rejection of an invalid NET.", 0)
	registerCIRoot("isis-wire", "isis-wire", "isis-wire", "Run IS-IS wire-level functional tests (.ci files in test/isis-wire/).\nCovers offline PDU decode: ze isis decode parses a hex IS-IS PDU to JSON.", 0)
	registerCIRoot("ospf-wire", "ospf-wire", "ospf-wire", "Run OSPFv2 wire-level functional tests (.ci files in test/ospf-wire/).\nCovers offline packet decode: ze ospf decode parses hex OSPFv2 packets to JSON.", 0)
	registerCIRoot("ospf", "ospf", "OSPF", "Run OSPF functional tests (.ci files in test/ospf/).\nCovers config parse -> YANG -> router-id/area validation, OSPF doctor diagnostics, and later daemon runtime scenarios.", 0)
	registerCIRoot("ospfv3", "ospfv3", "OSPFv3", "Run OSPFv3 (IPv6 family) functional tests (.ci files in test/ospfv3/).\nCovers RFC 5340 raw-socket and RFC 4552 IPsec config/doctor diagnostics.", 0)
	registerCIRoot("l2tp", "l2tp", "l2tp", "Run L2TPv2 functional tests (.ci files in test/l2tp/).\nCovers listener binding, control-connection handshake, challenge/response, hello keepalive, tie-breaker, and teardown.", 0)
	registerCIRoot("l2tp-wire", "l2tp-wire", "l2tp-wire", "Run L2TP wire-level functional tests (.ci files in test/l2tp-wire/).\nCovers control message decode (SCCRQ) and truncated packet handling.", 0)
	registerCIRoot("ldp", "ldp", "LDP", "Run LDP functional tests (.ci files in test/ldp/).\nCovers single-daemon boot: config parse -> YANG -> engine startup -> show ldp neighbor/binding.", 0)
	registerCIRoot("managed", "managed", "managed", "Run managed config functional tests (.ci files in test/managed/).\nTests fleet management: hub config, per-client auth, managed boot, config change.", 1)
	registerCIRoot("policy", "policy", "policy routing", "Run policy routing functional tests (.ci files in test/policy/).\nCovers boot-time apply, table/next-hop actions, tcp-flags, tcp-mss, and reload.", 0)
	registerCIRoot("rsvpte", "rsvpte", "RSVP-TE", "Run RSVP-TE functional tests (.ci files in test/rsvpte/).\nCovers single-daemon boot: config parse -> YANG -> engine startup -> show rsvp-te session/interface/tunnel/fast-reroute (incl. RFC 4090 fast-reroute config + bypass).", 0)
	registerCIRoot("runner", "runner", "runner", "Run test-runner primitive functional tests (.ci files in test/runner/).\nCovers the .ci orchestration grammar itself: naming a background process and stopping it mid-test (cmd=background:name=, cmd=stop).", 0)
	registerCIRoot("static", "static", "static", "Run static route functional tests (.ci files in test/static/).\nCovers boot-time apply, reload add/remove, and show output.", 0)
	registerCIRoot("traffic", "traffic", "traffic", "Run traffic-control functional tests (.ci files in test/traffic/).\nCovers component reactor wiring: boot-time apply and reload-time reapply.", 0)
	registerCIRoot("ui", "ui", "UI", "Run UI functional tests (.ci files in test/ui/).\nTests config completion, editor CLI, and other UI-facing features.", 0)
	registerCIRoot("vrrp", "vrrp", "VRRP", "Run VRRP functional tests (.ci files in test/vrrp/).\nCovers the vrrp YANG augment under interface units, the plugin's cross-leaf verifier (vrid, priority, per-version interval encodings, accept-mode, IPv6 first-address link-local, duplicate vrid/address, VPP backend rejection), and the show/doctor surfaces.", 0)

	// Engine-step executor: spawned BY test daemons as an external plugin
	// (plugin { external engine-steps { run "ze-test engine-steps ./engine-steps.json" } })
	// to drive .ci command=/stream=/expect=output|event|stream directives.
	registerRoot("engine-steps", cmdEngineSteps, "Execute .ci engine-step directives as an external plugin (spawned by test daemons, not run directly)")

	// Big test runners
	registerRoot("bgp", cmdBgp, "Run BGP functional tests (encoding, plugin, decoding, parsing)")
	registerRoot("editor", cmdEditor, "Run editor functional tests (.et files)")
	registerRoot("exabgp", cmdExabgp, "Run predecessor encoding tests")
	registerRoot("vpp", cmdVpp, "Run VPP stub-backed functional tests (test/vpp/*.ci)")
	registerRoot("web", cmdWeb, "Run web browser functional tests (.wb files)")

	// Mock servers
	registerRoot("cymru", cymru.Run, "Deterministic Cymru DNS mock server (ASN to TXT responses)")
	registerRoot("irr", irr.Run, "Deterministic IRR whois mock server (AS-SET expansion, prefix lookup)")
	registerRoot("peeringdb", peeringdb.Run, "Deterministic PeeringDB mock server (ASN-derived prefix counts)")
	registerRoot("rpki", rpki.Run, "Deterministic RPKI mock server (IP modulo for validation state)")
	registerRoot("radius-mock", radiusmock.Run, "Mock RADIUS server (RFC 2865) for AAA admin-auth testing")
	registerRoot("rtr-mock", rtr.Run, "Mock RTR cache server (explicit VRPs for RPKI testing)")
	registerRoot("syslog", cmdSyslog, "Run syslog server for testing")
	registerRoot("tacacs-mock", tacacs.Run, "Mock TACACS+ server (RFC 8907) for AAA testing")

	// Test tools
	registerRoot("l2tp-scale", cmdL2tpScale, "L2TP scale test: LAC simulator + mock RADIUS")
	registerRoot("mcp", cmdMcp, "MCP client (send commands to daemon via MCP endpoint)")
	registerRoot("peer", cmdPeer, "BGP test peer (sink/echo/check modes)")
	registerRoot("plugin-external", cmdPluginExternal, "Run a registered engine plugin's RunEngine externally (TLS connect-back) -- proves IsInternal()-guarded refuse/warn behavior; not a production plugin launcher")
	registerRoot("text-plugin", cmdTextPlugin, "Run minimal text-mode plugin (for .ci tests)")
}
