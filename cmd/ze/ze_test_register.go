// Design: docs/architecture/testing/ci-format.md -- ze-test subcommand registration

//go:build ze_test

package main

import (
	"fmt"
	"os"
	"sort"

	"codeberg.org/thomas-mangin/ze/internal/component/command/registry"
	zeversion "codeberg.org/thomas-mangin/ze/internal/core/version"
)

type zeTestSubcommand struct {
	desc    string
	handler func([]string) int
}

var testSubcommands = map[string]zeTestSubcommand{}

func init() {
	zeTestPopulateAll()
	binarySetup = zeTestSetup
}

func zeTestSetup(args []string) ([]string, int) {
	registry.MustRegisterRootHandler("test", zeTestRootHandler, registry.Meta{
		Description: "Functional test runners, mock servers, and tools",
		Mode:        "offline",
		Section:     registry.SectionTest,
	})
	return args, 0
}

func zeTestRootHandler(_ *registry.RuntimeContext, args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		zeTestUsage()
		return 0
	}
	if args[0] == "--version" || args[0] == "-V" {
		fmt.Println(zeversion.Short())
		return 0
	}
	sub, ok := testSubcommands[args[0]]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown test command: %s\n", args[0])
		zeTestUsage()
		return 1
	}
	return sub.handler(args[1:])
}

func zeTestPopulateAll() {
	r := zeTestAdd

	// CI test runners
	r("bgp", "Run BGP functional tests (encoding, plugin, decoding, parsing)", zeTestBgpCmd)
	r("editor", "Run editor functional tests (.et files)", zeTestEditorCmd)
	r("exabgp", "Run predecessor encoding tests", zeTestExabgpCmd)
	r("firewall", "Run firewall functional tests (test/firewall/*.ci)", zeTestFirewallCmd)
	r("flow-export", "Run flow-export functional tests (test/flow-export/*.ci)", zeTestFlowExportCmd)
	r("install", "Run install provisioning functional tests (test/install/*.ci)", zeTestInstallCmd)
	r("l2tp", "Run L2TPv2 functional tests (listener, tunnel FSM, handshake)", zeTestL2tpCmd)
	r("l2tp-wire", "Run L2TP wire-level functional tests (test/l2tp-wire/*.ci)", zeTestL2tpWireCmd)
	r("managed", "Run managed config tests (hub, auth, fleet)", zeTestManagedCmd)
	r("policy", "Run policy routing functional tests (test/policy/*.ci)", zeTestPolicyCmd)
	r("static", "Run static route functional tests (test/static/*.ci)", zeTestStaticCmd)
	r("traffic", "Run traffic-control functional tests (test/traffic/*.ci)", zeTestTrafficCmd)
	r("ui", "Run UI functional tests (completion, CLI)", zeTestUICmd)
	r("vpp", "Run VPP stub-backed functional tests (test/vpp/*.ci)", zeTestVppCmd)
	r("web", "Run web browser functional tests (.wb files)", zeTestWebCmd)

	// Mock servers
	r("cymru", "Deterministic Cymru DNS mock server (ASN to TXT responses)", zeTestCymruCmd)
	r("irr", "Deterministic IRR whois mock server (AS-SET expansion, prefix lookup)", zeTestIrrCmd)
	r("peeringdb", "Deterministic PeeringDB mock server (ASN-derived prefix counts)", zeTestPeeringdbCmd)
	r("rpki", "Deterministic RPKI mock server (IP modulo for validation state)", zeTestRpkiCmd)
	r("rtr-mock", "Mock RTR cache server (explicit VRPs for RPKI testing)", zeTestRtrMockCmd)
	r("syslog", "Run syslog server for testing", zeTestSyslogCmd)
	r("tacacs-mock", "Mock TACACS+ server (RFC 8907) for AAA testing", zeTestTacacsMockCmd)

	// Test tools
	r("l2tp-scale", "L2TP scale test: LAC simulator + mock RADIUS", zeTestL2tpScaleCmd)
	r("mcp", "MCP client (send commands to daemon via MCP endpoint)", zeTestMcpCmd)
	r("peer", "BGP test peer (sink/echo/check modes)", zeTestPeerCmd)
	r("text-plugin", "Run minimal text-mode plugin (for .ci tests)", zeTestTextPluginCmd)
}

func zeTestAdd(name, desc string, fn func([]string) int) {
	testSubcommands[name] = zeTestSubcommand{desc: desc, handler: fn}
}

func zeTestUsage() {
	names := make([]string, 0, len(testSubcommands))
	for name := range testSubcommands {
		names = append(names, name)
	}
	sort.Strings(names)

	width := 0
	for _, name := range names {
		if len(name) > width {
			width = len(name)
		}
	}

	fmt.Fprintf(os.Stderr, "Usage: ze-test <command> [options]\n\nCommands:\n")
	for _, name := range names {
		fmt.Fprintf(os.Stderr, "  %-*s  %s\n", width, name, testSubcommands[name].desc)
	}
	fmt.Fprintf(os.Stderr, "\nRun 'ze-test <command> --help' for command-specific help.\n")
}
