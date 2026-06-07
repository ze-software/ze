// Design: docs/architecture/testing/ci-format.md -- ze-test root handler registration

package cli

import (
	"fmt"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/subdispatch"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/cymru"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/irr"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/peeringdb"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/rpki"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/rtr"
	"codeberg.org/thomas-mangin/ze/internal/test/mock/tacacs"
)

func registerCIRunner(name, testSubdir, description, detail string, parallel int) {
	cfg := CIRunnerConfig{Name: name, TestSubdir: testSubdir, Description: description, Detail: detail, DefaultParallel: parallel}
	Register(name, func(args []string) int { return RunCISubcommand(cfg, args) }, subdispatch.SubMeta{Desc: "Run " + description + " functional tests"})
}

func init() {
	// CI test runners
	registerCIRunner("firewall", "firewall", "firewall", "Run firewall functional tests (.ci files in test/firewall/).\nCovers component reactor wiring: boot-time parse -> validate -> Apply.", 0)
	registerCIRunner("flow-export", "flow-export", "flow-export", "Run flow-export functional tests (.ci files in test/flow-export/).\nCovers sFlow v5, NetFlow v9, and IPFIX counter export over UDP, the show flow-export handler, packet-sampling wiring, and reload-time reconfiguration.", 0)
	registerCIRunner("install", "install", "install", "Run install provisioning functional tests (.ci files in test/install/).\nTests ze install CLI, config validation, and provisioning server setup.", 0)
	registerCIRunner("l2tp", "l2tp", "l2tp", "Run L2TPv2 functional tests (.ci files in test/l2tp/).\nCovers listener binding, control-connection handshake, challenge/response, hello keepalive, tie-breaker, and teardown.", 0)
	registerCIRunner("l2tp-wire", "l2tp-wire", "l2tp-wire", "Run L2TP wire-level functional tests (.ci files in test/l2tp-wire/).\nCovers control message decode (SCCRQ) and truncated packet handling.", 0)
	registerCIRunner("managed", "managed", "managed", "Run managed config functional tests (.ci files in test/managed/).\nTests fleet management: hub config, per-client auth, managed boot, config change.", 1)
	registerCIRunner("policy", "policy", "policy routing", "Run policy routing functional tests (.ci files in test/policy/).\nCovers boot-time apply, table/next-hop actions, tcp-flags, tcp-mss, and reload.", 0)
	registerCIRunner("static", "static", "static", "Run static route functional tests (.ci files in test/static/).\nCovers boot-time apply, reload add/remove, and show output.", 0)
	registerCIRunner("traffic", "traffic", "traffic", "Run traffic-control functional tests (.ci files in test/traffic/).\nCovers component reactor wiring: boot-time apply and reload-time reapply.", 0)
	registerCIRunner("ui", "ui", "UI", "Run UI functional tests (.ci files in test/ui/).\nTests config completion, editor CLI, and other UI-facing features.", 0)

	// Big test runners
	Register("bgp", cmdBgp, subdispatch.SubMeta{Desc: "Run BGP functional tests (encoding, plugin, decoding, parsing)"})
	Register("editor", cmdEditor, subdispatch.SubMeta{Desc: "Run editor functional tests (.et files)"})
	Register("exabgp", cmdExabgp, subdispatch.SubMeta{Desc: "Run predecessor encoding tests"})
	Register("vpp", cmdVpp, subdispatch.SubMeta{Desc: "Run VPP stub-backed functional tests (test/vpp/*.ci)"})
	Register("web", cmdWeb, subdispatch.SubMeta{Desc: "Run web browser functional tests (.wb files)"})

	// Mock servers
	Register("cymru", cymru.Run, subdispatch.SubMeta{Desc: "Deterministic Cymru DNS mock server (ASN to TXT responses)"})
	Register("irr", irr.Run, subdispatch.SubMeta{Desc: "Deterministic IRR whois mock server (AS-SET expansion, prefix lookup)"})
	Register("peeringdb", peeringdb.Run, subdispatch.SubMeta{Desc: "Deterministic PeeringDB mock server (ASN-derived prefix counts)"})
	Register("rpki", rpki.Run, subdispatch.SubMeta{Desc: "Deterministic RPKI mock server (IP modulo for validation state)"})
	Register("rtr-mock", rtr.Run, subdispatch.SubMeta{Desc: "Mock RTR cache server (explicit VRPs for RPKI testing)"})
	Register("syslog", cmdSyslog, subdispatch.SubMeta{Desc: "Run syslog server for testing"})
	Register("tacacs-mock", tacacs.Run, subdispatch.SubMeta{Desc: "Mock TACACS+ server (RFC 8907) for AAA testing"})

	// Test tools
	Register("l2tp-scale", cmdL2tpScale, subdispatch.SubMeta{Desc: "L2TP scale test: LAC simulator + mock RADIUS"})
	Register("mcp", cmdMcp, subdispatch.SubMeta{Desc: "MCP client (send commands to daemon via MCP endpoint)"})
	Register("peer", cmdPeer, subdispatch.SubMeta{Desc: "BGP test peer (sink/echo/check modes)"})
	Register("text-plugin", cmdTextPlugin, subdispatch.SubMeta{Desc: "Run minimal text-mode plugin (for .ci tests)"})

	// Root handler
	registry.MustRegisterRootHandler("test", func(_ *registry.RuntimeContext, args []string) int {
		if len(args) == 1 && (args[0] == "--version" || args[0] == "-V") {
			fmt.Println(zeversion.Short())
			return 0
		}
		return Dispatch(args)
	}, registry.Meta{
		Description: "Functional test runners, mock servers, and tools",
		Mode:        "offline",
		Section:     registry.SectionTest,
		SubsFunc:    Subcommands,
	})
}
