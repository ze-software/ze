// Design: docs/architecture/testing/interop.md -- what an integration gate needs
// Detail: actions.go -- the dispatch every row here is turned into
//
// gates.go defines integration, interop, stress, and live gates with their
// exact native callbacks or external toolchain arguments.
// Each gate needs infrastructure that is not present on every machine.
// The requirements include Docker, a network namespace, CAP_NET_ADMIN, root,
// QEMU, and internet access. For that reason, `./le verify current`
// does not include these evidence actions.
//
// The native QEMU area owns kernel staging, guards, boots, and its complete
// guest suite catalog. internal/le/test/qemu/alltests_test.go derives its target set
// from that catalog.
// Kernel staging and the host build share the same native definitions, so the
// guest proofs cannot boot an artifact produced by a different configuration.
//
// INTEROP_SCENARIO, IPSEC_INTEROP_SCENARIO and RADIUS_INTEROP_SCENARIO remain
// environment selectors.
// The three native interop callbacks read them before scenario discovery, so
// `./le test integration interop`, `./le test integration interop-ipsec` and
// `./le test integration interop-radius` select one exact scenario without a helper argv.
//
// Nine neighboring actions are not in this table. internal/le/test/deployment owns
// seven deployment actions, while internal/le/verify/evidence and internal/le/test/qemu own
// one each.
// The L2TP peer proof already uses this gate-family ownership.
// A duplicate row would make the parity census report drift.
//
// Step 10b of spec-le-is-a-ze-binary moved the ze-qemu- gate after its driver was ported.
// leaction derives verbs by removing only `ze-<area>-`.
// Keeping the gate here would expose its complete name as the verb.
package testintegration

import (
	"context"
	"slices"

	"github.com/ze-software/ze/internal/core/env"
	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// Area is the word this command is typed as.
const Area = "test integration"

// goTool is the toolchain command the kernel and live actions run.
const goTool = "go"

// envString is the env registry's word for a string-valued entry.
const envString = "string"

// Action defines one integration action: its native verb, purpose, and native
// callback or external toolchain command. Exactly one of Native, NativeArgs and
// Argv is set.
type Action struct {
	Verb   string
	Why    string
	Native func(context.Context, string) (any, int)
	Argv   func() []string
	// NativeArgs is a native callback for a verb with a keyword grammar,
	// Parameters. It counts as Native for "exactly one runner".
	NativeArgs func(context.Context, string, leaction.Arguments) (any, int)
	Parameters []leaction.Parameter
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
		Key:         "stress.prefixes",
		Type:        "int",
		Default:     "0",
		Description: "smoke runs only: replaces every Ze stress round's prefix count; 0 keeps the registry's counts",
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
		Key:         "radius.interop.scenario",
		Type:        envString,
		Default:     "",
		Description: "one scenario under test/interop-radius/; empty runs every scenario",
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
		{
			Verb:   "interop-radius",
			Native: runRADIUSInterop,
			Why: "RADIUS admin-login interop against a real FreeRADIUS server, every" +
				" scenario under test/interop-radius/scenarios/. Needs Docker and no" +
				" kernel module. RADIUS_INTEROP_SCENARIO=<name> runs one scenario",
		},
		{
			Verb:   "interop-radius-subscriber",
			Native: runRADIUSSubscriberInterop,
			Why: "subscriber RADIUS interop: FreeRADIUS accounting and radclient CoA/DM" +
				" against ze's L2TP LNS with an xl2tpd + pppd LAC, every scenario under" +
				" test/interop-radius-subscriber/scenarios/. Needs Docker, NET_ADMIN and" +
				" the host's /dev/ppp. RADIUS_SUBSCRIBER_INTEROP_SCENARIO=<name> runs one scenario",
		},
		{
			Verb:   "interop-rsvpte",
			Native: runRSVPTEInterop,
			Why: "RSVP-TE interop between Ze and freeRouter in ingress, transit, relay and egress roles, every" +
				" scenario under test/interop-rsvpte/scenarios/. Needs Docker, privileged" +
				" containers and host MPLS routing. RSVPTE_INTEROP_SCENARIO=<name> runs one scenario",
		},
		{
			Verb:       cleanupVerb,
			NativeArgs: runLabCleanup,
			Parameters: []leaction.Parameter{{Keyword: "dry", Kind: leaction.KindSwitch, Help: "list what would be removed and remove nothing"}},
			Why: "remove the Docker containers, networks and run-owned image tags a killed lab run left" +
				" behind: only resources whose ze.lab owner labels prove the owner gone, never an unlabeled" +
				" one, a volume or a cache tag. `dry` lists what would go and what stays, and why",
		},
		{
			Verb:   dockerPeersVerb,
			Native: runDockerPeers,
			Why: "every package test tagged `docker` (a real peer container started through interoplab," +
				" derived from the tree), with ZE_REQUIRE_DOCKER=1 so an absent Docker fails. Needs Docker" +
				" and no Ze kernel",
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
			Verb:   liveRPKIVerb,
			Native: runLiveRPKI,
			Why: "the RPKI validator against a real stayrtr cache started through labtest, with" +
				" ZE_REQUIRE_DOCKER=1 so an absent Docker or an unpullable image fails. Needs Docker" +
				" and internet access",
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
			Why: "the kernel FIB backend: what a route looks like once netlink has it, and" +
				" which route the kernel selects for a destination. Needs CAP_NET_ADMIN",
		},
		{
			Verb: "firewall",
			Argv: goTest("120s", "./internal/plugins/firewall/nft/...", "./internal/plugins/copp/..."),
			Why: "the nft firewall backend against a real nftables ruleset, and the CoPP" +
				" chain policing a SYN flood through it. Needs CAP_NET_ADMIN",
		},
		{
			Verb: "traffic",
			Argv: goTest("120s", "./internal/plugins/traffic/netlink/..."),
			Why: "the traffic-control netlink backend: qdisc and filter programming." +
				" Needs CAP_NET_ADMIN",
		},
		{
			Verb: "gtsm",
			// The reactor package carries its whole unit suite beside the two
			// integration tests, and under -race on a GitHub runner that suite
			// alone runs 240 to 265 s (evidence-nightly verify shards), so a
			// 120 s bound killed the binary every night before it could report.
			Argv: goTest("600s", "./internal/core/network/...", "./internal/component/bgp/reactor/...",
				"./internal/component/gtsm/..."),
			Why: "BGP GTSM and TTL-security, which live in a socket option only a Linux" +
				" kernel can answer for, and the route metric and nftables filter of a" +
				" GTSM peer's related ICMP messages, whose proofs need CAP_NET_ADMIN",
		},
		{
			Verb: "as112",
			Argv: goTest("60s", "./internal/plugins/as112/..."),
			Why: "the AS112 plugin serving DNS on privileged port 53. Needs" +
				" CAP_NET_BIND_SERVICE or root",
		},
	}
}
