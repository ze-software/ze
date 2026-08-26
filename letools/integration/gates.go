// Design: docs/architecture/testing/interop.md -- what an integration gate needs
// Detail: actions.go -- the dispatch every row here is turned into
//
// gates.go defines the integration, interop, stress, live, and QEMU gates with their exact arguments.
//
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
// INTEROP_SCENARIO and IPSEC_INTEROP_SCENARIO moved with their targets, from
// $(VAR) to the environment. A variable set on the make command line reaches the
// recipe's environment, so `make ze-interop-test INTEROP_SCENARIO=x` and
// `INTEROP_SCENARIO=x le integration ze-interop-test` build the same argv.
// VERBOSE and SESSION_TIMEOUT reach ze-stress-bird-test the same way, and they
// are passed THROUGH sudo as VAR=value arguments because sudo scrubs the
// environment it is given.
//
// Nine gates from the Python area are not in this table, but each remains owned elsewhere.
// letools/deployment owns seven ze-deployment- gates.
// letools/evidence owns one ze-evidence- gate, and letools/qemu owns one ze-qemu- gate.
// The L2TP peer proof already uses this gate-family ownership.
// A duplicate row would make the parity census report drift.
//
// Step 10b of spec-le-is-a-ze-binary moved the ze-qemu- gate after its driver was ported.
// leaction derives verbs by removing only `ze-<area>-`.
// Keeping the gate here would expose its complete name as the verb.
package integration

import (
	"slices"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// Area is the word this command is typed as, and the prefix leaction removes
// from each gate name to derive its verb.
const Area = "integration"

// python is the interpreter every lab runner and evidence driver here is
// started with, and goTool is the toolchain command the nine test gates run.
const (
	python = "python3"
	goTool = "go"
)

// envString is the env registry's own word for a string-valued entry, named
// because this file declares four of them.
const envString = "string"

// Gate defines an integration gate: its Make target, purpose, and command.
//
// Argv is a function because three gates read variables set on the Make command line.
// A slice built during package initialization would freeze the startup environment.
// The function instead reads the value requested by the caller.
type Gate struct {
	Name string
	Why  string
	Argv func() []string
}

// NeedsCgo reports whether this gate's command needs CGO_ENABLED=1.
//
// NeedsCgo is derived from the command instead of stored separately.
// A suite that stops using the race detector therefore stops requesting cgo.
// The race detector requires cgo.
func (g Gate) NeedsCgo() bool { return slices.Contains(g.Argv(), "-race") }

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
	_ = env.MustRegister(env.EnvEntry{
		Key:         "verbose",
		Type:        envString,
		Default:     "",
		Description: "passed through sudo to the stress runner, which prints more when it is set",
		Private:     true,
	})
	_ = env.MustRegister(env.EnvEntry{
		Key:         "session.timeout",
		Type:        envString,
		Default:     "",
		Description: "passed through sudo to the stress runner, as its per-session budget",
		Private:     true,
	})
)

// scenario answers a lab runner's argv, with the one scenario its variable
// selects.
//
// Empty selects every scenario, which is what an unset make variable did:
// `python3 test/interop/run.py $(INTEROP_SCENARIO)` expanded to the bare command
// and the runner's own default took over.
func scenario(script, key string) func() []string {
	return func() []string {
		if chosen := env.Get(key); chosen != "" {
			return []string{python, script, chosen}
		}
		return []string{python, script}
	}
}

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

// sudoStress answers a stress scenario as root, carrying VERBOSE and
// SESSION_TIMEOUT through sudo.
//
// The two names are passed as sudo's own VAR=value arguments rather than
// exported, because sudo does not forward the environment it is handed. They are
// spelled even when empty, which is what the Make recipe did: `sudo VERBOSE=
// SESSION_TIMEOUT= python3 ...` is the expansion of an unset pair.
func sudoStress(name string) func() []string {
	return func() []string {
		var tb textbuf.Buffer
		verbose := tb.Str("VERBOSE=").Str(env.Get("verbose")).String()
		tb.Reset()
		timeout := tb.Str("SESSION_TIMEOUT=").Str(env.Get("session.timeout")).String()
		return []string{"sudo", verbose, timeout, python, "test/stress/run.py", name}
	}
}

// Table answers every gate this area declares, in the order the listing prints
// them.
func Table() []Gate {
	return []Gate{
		// ── Interop ──────────────────────────────────────────────────────
		{
			Name: "ze-interop-test",
			Argv: scenario("test/interop/run.py", "interop.scenario"),
			Why: "BGP interop against the FRR, BIRD and GoBGP containers, every scenario" +
				" under test/interop/scenarios/. Needs Docker. INTEROP_SCENARIO=<name>" +
				" runs one of them",
		},
		{
			Name: "ze-interop-ipsec-test",
			Argv: scenario("test/interop-ipsec/run.py", "ipsec.interop.scenario"),
			Why: "IKEv2/IPsec interop against strongSwan. Needs Docker and privileged" +
				" containers. IPSEC_INTEROP_SCENARIO=<name> runs one scenario",
		},
		// ── Stress ───────────────────────────────────────────────────────
		{
			Name: "ze-stress-bird-test",
			Argv: sudoStress("04-bulk-ipv4-bird"),
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
