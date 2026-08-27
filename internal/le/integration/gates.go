// Design: docs/architecture/testing/interop.md -- what an integration gate needs
// Detail: actions.go -- the dispatch every row here is turned into
//
// gates.go defines integration, interop, stress, and live gates with their
// exact native callbacks or external toolchain arguments.
// Each gate needs infrastructure that is not present on every machine.
// The requirements include Docker, a network namespace, CAP_NET_ADMIN, root, QEMU, and internet access.
// For that reason, `make ze-precommit-verify` does not include these gates.
//
// All QEMU kernel logic remains in Make.
// Thirteen targets boot ze's runtime kernel through $(ze-qemu-kernel-guard).
// That guard compares tmp/kernel/vmlinuz with the architecture-and-config cache entry from ze-host.
// scripts/evidence/qemu_kernel_wiring_test.go derives its target set from those recipes.
// The guard, its `: ze-host-build` prerequisite, the shared cross-build definition, and their comments form one unit.
// Moving only some of that unit previously made its reasoning stale, so none of it moved here.
//
// INTEROP_SCENARIO and IPSEC_INTEROP_SCENARIO remain environment selectors.
// The two native interop callbacks read them before scenario discovery, so the
// Make and le entry points select the same exact scenario without a script argv.
//
// Nine neighboring gates are not in this table; each remains owned elsewhere.
// internal/le/deployment owns seven ze-deployment- gates.
// internal/le/evidence owns one ze-evidence- gate, and internal/le/qemu owns one ze-qemu- gate.
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

// Area is the word this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const Area = "integration"

// goTool is the toolchain command the nine kernel and live test gates run.
const goTool = "go"

// envString is the env registry's word for a string-valued entry.
const envString = "string"

// Gate defines an integration gate: its Make target, purpose, and native
// callback or external toolchain command. Exactly one of Native and Argv is set.
type Gate struct {
	Name   string
	Why    string
	Native func(context.Context, string) (any, int)
	Argv   func() []string
}

// NeedsCgo reports whether this gate's external command needs CGO_ENABLED=1.
// Native callbacks own their process environment and never request it here.
func (g Gate) NeedsCgo() bool {
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

// Table answers every gate this area declares, in the order the listing prints
// them.
func Table() []Gate {
	return []Gate{
		// ── Interop ──────────────────────────────────────────────────────
		{
			Name:   "ze-interop-test",
			Native: runGeneralInterop,
			Why: "BGP interop against the FRR, BIRD and GoBGP containers, every scenario" +
				" under test/interop/scenarios/. Needs Docker. INTEROP_SCENARIO=<name>" +
				" runs one of them",
		},
		{
			Name:   "ze-interop-ipsec-test",
			Native: runIPsecInterop,
			Why: "IKEv2/IPsec interop against strongSwan. Needs Docker and privileged" +
				" containers. IPSEC_INTEROP_SCENARIO=<name> runs one scenario",
		},
		// ── Stress ───────────────────────────────────────────────────────
		{
			Name:   "ze-stress-bird-test",
			Native: runStressBirdGate,
			Why: "the BIRD baseline the ze bulk-IPv4 stress numbers are read against." +
				" Needs root, bird2 and network namespaces",
		},
		{
			Name: "ze-stress-web-test",
			Argv: fixed(goTool, "test", "-tags", "ze_core stress", "-race", "-count=1",
				"-timeout", "300s", "./internal/component/web/",
				"-run", "TestWebConcurrentEditStress", "-v"),
			Why: "50 or more concurrent editor sessions against the web UI," +
				" race-instrumented. Evidence tier: the `stress` build tag keeps it out" +
				" of ze-precommit-verify (R-6)",
		},
		{
			Name: "ze-stress-fleet-test",
			Argv: fixed(goTool, "test", "-tags", "ze_core fleetperf", "-count=1",
				"-timeout", "300s", "./cmd/ze/hub/",
				"-run", "TestFleetManyClientsPerf", "-v"),
			Why: "128 managed clients against a real hub listener. Evidence tier: the" +
				" `fleetperf` build tag keeps it out of ze-precommit-verify (R-6)",
		},
		// ── Live ─────────────────────────────────────────────────────────
		{
			Name: "ze-live-rpki-test",
			Argv: fixed(goTool, "test", "-v", "-tags", "live", "-timeout", "180s", "-count=1",
				"./internal/component/bgp/plugins/rpki/...", "-run", "TestLive"),
			Why: "the RPKI validator against a real cache. Needs Docker and internet access",
		},
		// ── Integration (network namespace) ───────────────────────────────
		{
			Name: "ze-integration-iface-test",
			Argv: goTest("120s", "./internal/component/iface/..."),
			Why: "the iface component against a real kernel: netlink link, address and" +
				" route programming. Needs CAP_NET_ADMIN",
		},
		{
			Name: "ze-integration-fib-test",
			Argv: goTest("120s", "./internal/plugins/fib/kernel/..."),
			Why: "the kernel FIB backend: what a route looks like once netlink has it." +
				" Needs CAP_NET_ADMIN",
		},
		{
			Name: "ze-integration-firewall-test",
			Argv: goTest("120s", "./internal/plugins/firewall/nft/..."),
			Why:  "the nft firewall backend against a real nftables ruleset. Needs CAP_NET_ADMIN",
		},
		{
			Name: "ze-integration-traffic-test",
			Argv: goTest("120s", "./internal/plugins/traffic/netlink/..."),
			Why: "the traffic-control netlink backend: qdisc and filter programming." +
				" Needs CAP_NET_ADMIN",
		},
		{
			Name: "ze-integration-gtsm-test",
			Argv: goTest("120s", "./internal/core/network/...", "./internal/component/bgp/reactor/..."),
			Why: "BGP GTSM and TTL-security, which live in a socket option only a Linux" +
				" kernel can answer for",
		},
		{
			Name: "ze-integration-as112-test",
			Argv: goTest("60s", "./internal/plugins/as112/..."),
			Why: "the AS112 plugin serving DNS on privileged port 53. Needs" +
				" CAP_NET_BIND_SERVICE or root",
		},
	}
}
