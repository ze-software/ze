// Design: docs/architecture/testing/interop.md -- what an integration gate needs
// Detail: actions.go -- the dispatch every row here is turned into
//
// gates.go defines integration, interop, stress, and live gates with their
// exact native callbacks or external toolchain arguments.
// Each gate needs infrastructure that is not present on every machine.
// The requirements include Docker, a network namespace, CAP_NET_ADMIN, root,
// QEMU, and internet access. For that reason, `./le verify current mode full`
// does not include these evidence actions.
//
// The native QEMU area owns kernel staging, guards, boots, and its complete
// guest suite catalog. internal/le/qemu/alltests_test.go derives its target set
// from that catalog.
// Kernel staging and the host build share the same native definitions, so the
// guest proofs cannot boot an artifact produced by a different configuration.
//
// INTEROP_SCENARIO and IPSEC_INTEROP_SCENARIO remain environment selectors.
// The two native interop callbacks read them before scenario discovery, so
// `./le integration interop` and `./le integration interop-ipsec` select one
// exact scenario without a helper argv.
//
// Nine neighboring actions are not in this table. internal/le/deployment owns
// seven deployment actions, while internal/le/evidence and internal/le/qemu own
// one each.
// The L2TP peer proof already uses this gate-family ownership.
// A duplicate row would make the parity census report drift.
//
// Step 10b of spec-le-is-a-ze-binary moved the ze-qemu- gate after its driver was ported.
// leaction derives verbs by removing only `ze-<area>-`.
// Keeping the gate here would expose its complete name as the verb.
package integration

import (
	"context"
	"slices"

	"github.com/ze-software/ze/internal/core/env"
)

// Area is the word this command is typed as.
const Area = "integration"

// goTool is the toolchain command the kernel and live actions run.
const goTool = "go"

// envString is the env registry's word for a string-valued entry.
const envString = "string"

// Action defines one integration action: its native verb, purpose, and native
// callback or external toolchain command. Exactly one of Native and Argv is set.
type Action struct {
	Verb   string
	Why    string
	Native func(context.Context, string) (any, int)
	Argv   func() []string
}

// needsCgo reports whether this action's external command needs CGO_ENABLED=1.
// Native callbacks own their process environment and never request it here.
func (g Action) needsCgo() bool {
	if g.Argv == nil {
		return false
	}
	return slices.Contains(g.Argv(), "-race")
}

var (
	_ = env.MustRegister(env.EnvEntry{
		Key:         "interop.scenario",
		Type:        envString,
		Default:     "",
		Description: "one scenario under test/interop/scenarios/; empty runs every scenario",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "stress.scenario",
		Type:        envString,
		Default:     "",
		Description: "one scenario in the native BGP stress registry; empty runs all five",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "ipsec.interop.scenario",
		Type:        envString,
		Default:     "",
		Description: "one scenario under test/interop-ipsec/; empty runs every scenario",
		Private:     true,
	})
)

// goTest answers a kernel-facing integration suite, argument for argument.
//
// These packages use only `-tags integration`.
// Their `//go:build integration` guards select them, and daemon feature tags do not affect netlink calls.
func goTest(timeout string, packages ...string) func() []string {
	return func() []string {
		argv := make([]string, 0, 8+len(packages))
		argv = append(argv, goTool, "test", "-tags", "integration", "-count=1", "-race", "-timeout", timeout)
		return append(argv, packages...)
	}
}

// fixed answers a command that reads nothing from the environment.
func fixed(argv ...string) func() []string {
	return func() []string { return slices.Clone(argv) }
}

// Table answers every action this area declares, in listing order.
func Table() []Action {
	return []Action{
		// ── Interop ──────────────────────────────────────────────────────
		{
			Verb:   "interop",
			Native: runGeneralInterop,
			Why: "BGP interop against the FRR, BIRD and GoBGP containers, every scenario" +
				" under test/interop/scenarios/. Needs Docker. INTEROP_SCENARIO=<name>" +
				" runs one of them",
		},
		{
			Verb:   "interop-ipsec",
			Native: runIPsecInterop,
			Why: "IKEv2/IPsec interop against strongSwan. Needs Docker and privileged" +
				" containers. IPSEC_INTEROP_SCENARIO=<name> runs one scenario",
		},
		// ── Stress ───────────────────────────────────────────────────────
		{
			Verb:   StressAction,
			Native: runStressGate,
			Why: "the complete native BGP stress registry: bulk IPv4, mixed families," +
				" session flap, BIRD baseline, and 1M-route profiling. Needs root," +
				" network namespaces, iproute2, ethtool, tcpdump, and BIRD." +
				" STRESS_SCENARIO=<name> runs one scenario",
		},
		{
			Verb:   "stress-bird",
			Native: runStressBirdGate,
			Why: "the BIRD baseline the ze bulk-IPv4 stress numbers are read against." +
				" Needs root, bird2 and network namespaces",
		},
		{
			Verb: "stress-web",
			Argv: fixed(goTool, "test", "-tags", "ze_core stress", "-race", "-count=1",
				"-timeout", "300s", "./internal/component/web/",
				"-run", "TestWebConcurrentEditStress", "-v"),
			Why: "50 or more concurrent editor sessions against the web UI," +
				" race-instrumented. Evidence tier: the `stress` build tag keeps it out" +
				" of ze-precommit-verify (R-6)",
		},
		{
			Verb: "stress-fleet",
			Argv: fixed(goTool, "test", "-tags", "ze_core fleetperf", "-count=1",
				"-timeout", "300s", "./cmd/ze/hub/",
				"-run", "TestFleetManyClientsPerf", "-v"),
			Why: "128 managed clients against a real hub listener. Evidence tier: the" +
				" `fleetperf` build tag keeps it out of ze-precommit-verify (R-6)",
		},
		// ── Live ─────────────────────────────────────────────────────────
		{
			Verb: "live-rpki",
			Argv: fixed(goTool, "test", "-v", "-tags", "live", "-timeout", "180s", "-count=1",
				"./internal/component/bgp/plugins/rpki/...", "-run", "TestLive"),
			Why: "the RPKI validator against a real cache. Needs Docker and internet access",
		},
		// ── Integration (network namespace) ───────────────────────────────
		{
			Verb: "iface",
			Argv: goTest("120s", "./internal/component/iface/..."),
			Why: "the iface component against a real kernel: netlink link, address and" +
				" route programming. Needs CAP_NET_ADMIN",
		},
		{
			Verb: "fib",
			Argv: goTest("120s", "./internal/plugins/fib/kernel/..."),
			Why: "the kernel FIB backend: what a route looks like once netlink has it." +
				" Needs CAP_NET_ADMIN",
		},
		{
			Verb: "firewall",
			Argv: goTest("120s", "./internal/plugins/firewall/nft/..."),
			Why:  "the nft firewall backend against a real nftables ruleset. Needs CAP_NET_ADMIN",
		},
		{
			Verb: "traffic",
			Argv: goTest("120s", "./internal/plugins/traffic/netlink/..."),
			Why: "the traffic-control netlink backend: qdisc and filter programming." +
				" Needs CAP_NET_ADMIN",
		},
		{
			Verb: "gtsm",
			Argv: goTest("120s", "./internal/core/network/...", "./internal/component/bgp/reactor/..."),
			Why: "BGP GTSM and TTL-security, which live in a socket option only a Linux" +
				" kernel can answer for",
		},
		{
			Verb: "as112",
			Argv: goTest("60s", "./internal/plugins/as112/..."),
			Why: "the AS112 plugin serving DNS on privileged port 53. Needs" +
				" CAP_NET_BIND_SERVICE or root",
		},
	}
}
