// Design: docs/architecture/testing/ci-format.md -- ze-test root handler registration

package cli

import (
	"codeberg.org/thomas-mangin/ze/internal/test/mock/cymru"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/irr"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/peeringdb"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/rpki"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/rtr"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/tacacs"
)

func init() {
	// CI test runners
	registerCIRoot("firewall", "firewall", "firewall", "Run firewall functional tests (.ci files in test/firewall/).\nCovers component reactor wiring: boot-time parse -> validate -> Apply.", 0)
	registerCIRoot("flow-export", "flow-export", "flow-export", "Run flow-export functional tests (.ci files in test/flow-export/).\nCovers sFlow v5, NetFlow v9, and IPFIX counter export over UDP, the show flow-export handler, packet-sampling wiring, and reload-time reconfiguration.", 0)
	registerCIRoot("install", "install", "install", "Run install provisioning functional tests (.ci files in test/install/).\nTests ze install CLI, config validation, and provisioning server setup.", 0)
	registerCIRoot("l2tp", "l2tp", "l2tp", "Run L2TPv2 functional tests (.ci files in test/l2tp/).\nCovers listener binding, control-connection handshake, challenge/response, hello keepalive, tie-breaker, and teardown.", 0)
	registerCIRoot("l2tp-wire", "l2tp-wire", "l2tp-wire", "Run L2TP wire-level functional tests (.ci files in test/l2tp-wire/).\nCovers control message decode (SCCRQ) and truncated packet handling.", 0)
	registerCIRoot("managed", "managed", "managed", "Run managed config functional tests (.ci files in test/managed/).\nTests fleet management: hub config, per-client auth, managed boot, config change.", 1)
	registerCIRoot("policy", "policy", "policy routing", "Run policy routing functional tests (.ci files in test/policy/).\nCovers boot-time apply, table/next-hop actions, tcp-flags, tcp-mss, and reload.", 0)
	registerCIRoot("static", "static", "static", "Run static route functional tests (.ci files in test/static/).\nCovers boot-time apply, reload add/remove, and show output.", 0)
	registerCIRoot("traffic", "traffic", "traffic", "Run traffic-control functional tests (.ci files in test/traffic/).\nCovers component reactor wiring: boot-time apply and reload-time reapply.", 0)
	registerCIRoot("ui", "ui", "UI", "Run UI functional tests (.ci files in test/ui/).\nTests config completion, editor CLI, and other UI-facing features.", 0)

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
	registerRoot("rtr-mock", rtr.Run, "Mock RTR cache server (explicit VRPs for RPKI testing)")
	registerRoot("syslog", cmdSyslog, "Run syslog server for testing")
	registerRoot("tacacs-mock", tacacs.Run, "Mock TACACS+ server (RFC 8907) for AAA testing")

	// Test tools
	registerRoot("l2tp-scale", cmdL2tpScale, "L2TP scale test: LAC simulator + mock RADIUS")
	registerRoot("mcp", cmdMcp, "MCP client (send commands to daemon via MCP endpoint)")
	registerRoot("peer", cmdPeer, "BGP test peer (sink/echo/check modes)")
	registerRoot("text-plugin", cmdTextPlugin, "Run minimal text-mode plugin (for .ci tests)")
}
