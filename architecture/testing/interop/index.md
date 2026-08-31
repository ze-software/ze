# Interoperability Testing

Ze validates protocol correctness against production BGP daemons in two complementary ways:
live session interop tests (Docker containers running real daemons) and byte-level wire format
validation against ExaBGP (Ze's predecessor, a BGP implementation in Python).

For BGP terminology used in this document, see [docs/features.md](../../../reference/feature-status/index.md).

`ai/rules/interop-and-goal-validation.md` states when an interop test is owed.
This page is the infrastructure it is owed against.

## The suites, one per protocol area

| Protocol area | Peer implementation | Scenario directory | Native action |
|---------------|---------------------|--------------------|---------------|
| BGP (session, capability, NLRI, community, policy) | Docker: FRR, BIRD, GoBGP, StayRTR | `test/interop/scenarios/` | `./le integration interop` |
| IPsec (IKEv2, EAP, MOBIKE) | Docker: strongSwan | `test/interop-ipsec/` | `./le integration interop-ipsec` |
| L2TP | Docker | `test/interop-l2tp/` | `./le deployment l2tp-test`, and `./le deployment l2tp-ppp-test` for the full PPP and NCP path |
| PPPoE (Ze as client) | Docker: accel-ppp | `test/interop-pppoe/` | `./le deployment docker-pppoe-accel-test` |

<!-- source: internal/le/integration/gates.go -- interop and interop-ipsec verbs -->
<!-- source: internal/le/deployment/actions.go -- l2tp-test, l2tp-ppp-test, docker-pppoe-accel-test verbs -->

Every suite discovers its scenarios the same way. `Discover`
(`internal/le/interoplab/discover.go`) reads the scenario directory, keeps the
subdirectories, sorts the names lexically, and joins each one against the
owning checker registry. A scenario with no checker, or a nil checker, is an
ERROR rather than a skipped test, so a fixture and its registry cannot silently
disagree. Nothing depends on the order: each scenario gets its own setup, check
and teardown.

A scenario directory carries only declarative inputs its runner reads: `ze.conf`
plus the peer configuration and argument files that topology needs. Assertions
never live there. They are typed Go checkers under `internal/le/interoplab/`:
BGP builds its registry from `scenarioOperations` in
`internal/le/interoplab/bgp/checkers.go`, and IPsec declares `scenarioCheckers`
in `internal/le/interoplab/ipsec/checkers.go`. A checker waits for readiness,
asserts the protocol behaviour, verifies stability where the scenario needs it,
and returns an error on failure.

A scenario directory is NAMED and carries no numeric prefix. The name is the
scenario's identity: `Discover` matches it exactly, `./le integration` takes it
as a scenario selector, and specs, journal rows and code comments cite it.

## Tested Daemons

| Daemon | Version | Image | Query Method | What It Validates |
|--------|---------|-------|--------------|-------------------|
| FRR | 10.3.1 | `quay.io/frrouting/frr:10.3.1` | vtysh | eBGP, iBGP, route exchange, GR, communities, MD5, route server |
| BIRD | 2.x (Alpine 3.21) | Alpine build | birdc | eBGP, route exchange, triangle topologies |
| GoBGP | 3.31.0 | Go builder | gobgp CLI | eBGP, route injection and verification |
| StayRTR | 0.6.4 | Go builder | HTTP `/rpki.json` export | RTR (RFC 8210) as the CACHE, so Ze is the client of an implementation that is not its own. Origin validation answers (RFC 6811) against VRPs a third party encoded |
| ExaBGP | API 6.0.0 contract fixtures | Compiled Go wire server | Wire byte comparison | Byte-for-byte encoding across all address families |
<!-- source: internal/le/interoplab/bgp/prepare.go -- peer helpers and scenario preparation -->
<!-- source: internal/le/interoplab/bgp/run.go -- scenario orchestrator -->

## Prerequisites

| Requirement | Used By | Notes |
|-------------|---------|-------|
| Docker | Interop tests | Containers for FRR, BIRD, GoBGP, Ze |
| ~1.5 GB disk | Interop tests | Docker images (Go builder, FRR, Alpine) |

The interop test network uses `172.30.0.0/24`. MD5 authentication scenarios require
`NET_ADMIN` capability (granted automatically by the orchestrator).

## Live Interop Tests (`test/interop/`)

Each scenario runs Ze and one or more peer daemons in Docker containers on a shared
network (`172.30.0.0/24`), establishes real BGP sessions, and asserts correct behavior
via each daemon's native CLI.

### How It Works

The native `internal/le/interoplab/bgp` package discovers scenario directories in
`test/interop/scenarios/`. For each scenario, the shared `interoplab.Suite`
engine:

1. Creates an isolated Docker network.
2. Starts Ze and the peer daemons declared by the scenario's config files.
3. Waits for every declared readiness probe.
4. Runs the scenario's typed checker from the package-local BGP registry.
5. Tears down every container and network, including after setup or checker failure.
<!-- source: internal/le/interoplab/lab.go -- suite lifecycle -->

Daemons start conditionally: FRR if `frr.conf` exists, BIRD if `bird.conf` exists,
GoBGP if `gobgp.toml` exists. This means each scenario only runs the daemons it needs.

**A scenario directory is NAMED, never numbered** (owner directive, 2026-08-24).
Run order is lexical by scenario directory name and no scenario depends on it.
The rule and its reasoning live in `ai/rules/interop-and-goal-validation.md`,
which is always-on, so a spec author planning a scenario meets it without
opening this page.

### Container Addresses

| Daemon | IP | Container |
|--------|----|-----------|
| Ze | 172.30.0.2 | `ze-iop-ze-<pid>` |
| FRR | 172.30.0.3 | `ze-iop-frr-<pid>` |
| BIRD | 172.30.0.4 | `ze-iop-bird-<pid>` |
| GoBGP | 172.30.0.5 | `ze-iop-gobgp-<pid>` |
| Raw injector | 172.30.0.9 | `ze-iop-inject-<pid>` |
| Compiled strict speaker | 172.30.0.10 | `ze-iop-speaker-<pid>` |
| Compiled strict speaker (2nd) | 172.30.0.11 | `ze-iop-speaker2-<pid>` |
| StayRTR | 172.30.0.12 | `ze-iop-stayrtr-<pid>` |

Container names include the runner PID as suffix, so concurrent runs do not conflict.
<!-- source: internal/le/interoplab/bgp/prepare.go -- container naming, IP addresses -->

### Scenario Structure

Each scenario is a directory under `test/interop/scenarios/`:

```
scenarios/bgp-ebgp-ipv4-frr/
  ze.conf        # Ze configuration (required)
  frr.conf       # FRR configuration (starts FRR container)
```

Every directory has one typed checker in `internal/le/interoplab/bgp`. The
catalogue uses explicit operations for ordinary session, route, adjacency, log,
and negative assertions, plus bespoke checkers for scenarios whose control flow
cannot be represented as an ordered operation list.

A bespoke checker is written in two halves. The body in `check_rfc.go` does the
lab I/O and numbers each assertion, so a failure names the assertion that found
it. The pure predicate in `check_rfc_predicate.go` takes the text or the decoded
JSON a peer daemon produced, holds no lab handle, and decides. The split is what
lets `TestBespokeCheckerBranches` drive both polarities of every predicate in
seconds with no container.
<!-- source: internal/le/interoplab/bgp/check_rfc_predicate.go -- pure predicates -->
<!-- source: internal/le/interoplab/bgp/check_rfc.go -- tagged checker bodies -->

A body MUST NOT call `checkScenario` (`check_engine.go`). That function answers
`scenario %s has no typed assertions` for every name absent from
`scenarioOperations`, and a bespoke name is absent from that table by design, so
the call is an error that always fires and every line under it is unreachable.

### Optional sidecars

A scenario directory may also carry files that start extra containers before Ze:

| File | Sidecar | Purpose |
|------|---------|---------|
| `inject.msg` | `ze-test peer` (raw injector, 172.30.0.9) | Drive Ze with wire bytes no conforming daemon would emit. An optional `inject-args` file adds flags. Because the injector and Ze start before the peer daemons, an early route exercises Ze's replay-on-peer-up path. |
| `speaker-args` (and optional `speaker2-args`) | `ze-test interop-bgp speaker` (172.30.0.10; second at 172.30.0.11) | Dial Ze with an independent strict peer. The compiled speaker negotiates the requested families and ADD-PATH mode, frames BGP itself, applies the named native oracle, and writes a structured verdict to container logs. It catches wire output that Ze's own lenient decoder could accept. |
| `vrps.json` | StayRTR (172.30.0.12:8282) | Serve RPKI VRPs from a real third-party cache, so Ze is the RTR client of an implementation that is not its own. The typed checker asserts each per-prefix validation answer, not merely the RTR session. |

BMP scenarios start `ze-test interop-bgp bmp-collector`. Announcement and
observer process plugins use `ze-test interop-bgp process <scenario> <plugin>`.
These personalities are compiled into `ze-test`; no interpreter or source mount
is present in the Ze image.
<!-- source: internal/le/interoplab/bgp/prepare.go -- sidecar startup -->
<!-- source: internal/le/interoplab/bgp/helper.go -- compiled process and BMP helpers -->
<!-- source: internal/le/interoplab/bgp/speaker.go -- compiled strict speaker -->
<!-- source: test/interop/scenarios/bgp-rfc7606-relay-shape-frr/ -- injector worked example -->
<!-- source: test/interop/scenarios/bgp-rfc7606-speaker-dup-attr/ -- speaker worked example -->
<!-- source: test/interop/scenarios/rtr-stayrtr/ -- StayRTR worked example -->

### Prove a scenario discriminates

An interop scenario is evidence only if it goes RED when the behaviour it tests is
broken. Before you rely on a new scenario, revert the fix, run the scenario, and
confirm that it fails. Then restore the fix and confirm that it passes.

**Let the harness build. Do NOT `docker build -t ze-interop` by hand and then run
with `NO_BUILD=1`.** A tag is shared by every run on the host. A build in another
session rebinds that tag between yours and your container start. Your mutation run
then measures a daemon you did not build, and that inverted a proof twice in one
review on 2026-08-05. `Docker.Build` reads the image ID from `docker build -q`,
and the suite pins every container of the run to that immutable ID.
Quote that line beside the result, because it names the binary the run measured.

A scenario that passes either way (common when the peer must accept both the old
and the new wire form) proves acceptance, not correctness. Say which one it
proves in the spec's Goal Validation, and move the discrimination to a unit or
mutation test that CAN fail.

A scenario added to ALREADY-WORKING code never had a red phase, so its
discrimination is unproven until you force one. That is not TDD's red-then-green:
a regression test and a scenario for existing behaviour both start green.

Four traps make a scenario pass whatever the code does. Check each by its tell
before you call the scenario evidence:

| Vacuity trap | Why it passes anyway | The tell |
|--------------|----------------------|----------|
| A scenario for a sender-side wire change whose receiver is obliged to accept any form (RFC 7606 Section 5.1: receivers accept any field combination) | A conforming peer accepts the old and the new wire equally | Reverting the sender change leaves the peer's routing table identical |
| A test asserting the ABSENCE of something (no log line, no allocation, no route) | Deleting the mechanism leaves the same absence | Ask what would still be absent if the code were removed |
| A test whose fixture is at an extreme (all fields set, maximum value) | An off-by-one or a partial break still handles the extreme | Boundary the fixture: test one below and one above |
| A test whose data reaches the peer by a DIFFERENT path than the one changed | The unchanged path still delivers | Trace which code path actually produces the asserted bytes |

### Typed checker operations

`checkers.go` is the complete scenario catalogue. Each operation identifies the
peer, exact command, required or forbidden evidence, proof for negative
assertions, and a bound. `check_engine.go` executes those operations through the
shared `CheckerLab` interface. FRR, BIRD, GoBGP, Ze, speaker logs, and kernel
state are all queried through explicit typed branches.

An absent value never proves a negative assertion by itself. The operation must
also name positive evidence that the query mechanism ran. Failed and empty
queries remain errors rather than becoming plausible empty protocol state.

Scenarios with non-linear behavior register a bespoke checker in
`specialCheckers` (`check_special.go`). Each one owes a named subtest in
`TestBespokeCheckerBranches` that drives its predicate in BOTH polarities: the
true case, and a false case written against one stated wrong reading, such as
two tokens matched across two log lines, a peer-originated event passing as a
received one, or an absence with no proof that the query ran.
<!-- source: internal/le/interoplab/bgp/bgp_test.go -- TestBespokeCheckerBranches -->

### Querying Ze

`Ze.cli(command)` is the only way to ask the Ze daemon anything. It runs
`ze cli -c <command> --user ... --format json` inside the container.

Two properties of that line are load-bearing and neither is obvious.

`ze cli -c` rather than the verb form `ze show bgp rib status`: `--user` and
`--format` are flags of `ze cli`, and the verb form has no slot for either.

The daemon starts an SSH listener only when its config asks for one
(`infraSetup`, `cmd/ze/hub/infra_setup.go`), and `ze cli` reaches the daemon over
SSH. No scenario `ze.conf` asks. The harness appends `ZE_CLI_CONFIG` -- the
listener plus the account it authenticates against -- to the RENDERED copy of
every `ze.conf` (`renderScenario`), so no scenario carries the boilerplate and
none can forget it. The native IPsec plan appends the same blocks in
`renderZeConfig` (`internal/le/interoplab/ipsec/ipsec.go`).

**A Ze helper never converts a failed query into a plausible number.**
`Ze.rib_count` raises when the command fails or answers without a `routes-in`
field, because 0 is a legitimate RIB size and a failed query is not
(`ai/rules/evidence.md`). It returned 0 on failure until 2026-08-07 and three
separate faults hid behind that one number for three days
(`spec-fixit-test-harness-fail-open-guards`, guard 3). Write new Ze
helpers the same way.

All session waiters use explicit bounds (default 90 seconds, override via
`SESSION_TIMEOUT`). The harness passes that value into the Ze container so a
compiled process helper can size its barriers against the same budget.
<!-- source: internal/le/interoplab/wait.go -- bounded wait -->
<!-- source: internal/le/interoplab/bgp/prepare.go -- container environment -->

### A compiled process helper that fails

Scenario process personalities use the Go plugin SDK. A helper failure writes
the `ZE-OBSERVER-FAIL` sentinel to stderr and requests daemon shutdown. The
checker failure path reads the last 2,000 Ze log lines and appends that measured
cause when the sentinel is present.

An unreadable log is not a plugin verdict. `checkerFailure` retains the original
scenario assertion when the log read fails, so a Docker diagnostic cannot
replace the protocol failure that triggered it.

<!-- source: internal/le/interoplab/bgp/helper.go -- runtimeFailure -->
<!-- source: internal/le/interoplab/bgp/check_engine.go -- checkerFailure -->
<!-- source: internal/le/interoplab/bgp/bgp_test.go -- TestCheckerFailureKeepsPrimaryCauseWhenLogsFail -->

### Scenario Inventory

The suite has grown to over 100 scenario directories in `test/interop/scenarios/`. The table
below lists the core BGP scenarios (01-37); beyond these, the suite also covers route
reflection, policy import/export, RPKI origin validation, BMP monitoring, PATHS-LIMIT,
max-prefix cease, GTSM, AS112, ADD-PATH re-advertisement (`bgp-addpath-readvertise-collision-frr`
proves a receiver keeps two paths whose sources both chose one Path Identifier, and
`bgp-addpath-rail-agreement-speaker` proves the live forward and the peer-up replay emit the same
bytes for one path), and full IS-IS (auth, convergence, dual-stack, LAN DIS,
P2P, redistribution) and OSPFv2/OSPFv3 (auth, BFD, TE, LFA/TI-LFA, graceful restart,
segment routing, opaque LSAs, stub/NSSA, virtual links, and more) interop families.

| # | Scenario | Daemons | What It Tests |
|---|----------|---------|---------------|
| 01 | ebgp-ipv4-frr | Ze, FRR | Basic eBGP session establishment |
| 02 | ebgp-ipv4-bird | Ze, BIRD | Basic eBGP session with BIRD |
| 03 | ibgp-frr | Ze, FRR | iBGP session (same AS) |
| 04 | 4byte-asn-frr | Ze, FRR | 4-byte ASN negotiation (RFC 6793) |
| 05 | routes-from-frr | Ze, FRR | Ze receives routes originated by FRR |
| 06 | routes-from-bird | Ze, BIRD | Ze receives routes originated by BIRD |
| 07 | routes-to-frr | Ze, FRR | FRR receives routes originated by Ze |
| 08 | triangle | Ze, FRR, BIRD | Three-way topology, multi-peer stability |
| 09 | route-withdrawal-frr | Ze, FRR | Route withdrawal propagation |
| 10 | ipv6-ebgp-frr | Ze, FRR | IPv6 eBGP session and route exchange |
| 11 | addpath-frr | Ze, FRR | ADD-PATH capability (RFC 7911) |
| 12 | route-refresh-frr | Ze, FRR | Route Refresh (RFC 2918) |
| 13 | graceful-restart-frr | Ze, FRR | Graceful Restart negotiation (RFC 4724) |
| 14 | route-server-frr | Ze, FRR, BIRD | Route server: forwards without inserting own ASN |
| 15 | community-frr | Ze, FRR | Standard community propagation |
| 16 | extended-community-frr | Ze, FRR | Extended community propagation |
| 17 | md5-auth-frr | Ze, FRR | TCP MD5 authentication (RFC 2385) |
| 18 | ebgp-gobgp | Ze, GoBGP | eBGP session with GoBGP |
| 19 | routes-gobgp | Ze, GoBGP | Route exchange with GoBGP |
| 20 | role-frr | Ze, FRR | RFC 9234 Role capability negotiation |
| 21 | role-gobgp | Ze, GoBGP | RFC 9234 Role capability negotiation |
| 22 | evpn-frr | Ze, FRR | EVPN Type-2 route exchange |
| 23 | vpn-frr | Ze, FRR | VPN (L3VPN) route exchange |
| 24 | flowspec-frr | Ze, FRR | FlowSpec rule exchange |
| 25 | ipv6-ebgp-bird | Ze, BIRD | IPv6 eBGP route exchange |
| 26 | ipv6-ebgp-gobgp | Ze, GoBGP | IPv6 eBGP route exchange |
| 27 | multihop-ebgp-frr | Ze, FRR | Multi-hop eBGP with outgoing-ttl |
| 28 | evpn-gobgp | Ze, GoBGP | EVPN Type-2 route exchange |
| 29 | vpn-gobgp | Ze, GoBGP | VPN (L3VPN) route exchange |
| 30 | flowspec-gobgp | Ze, GoBGP | FlowSpec rule exchange |
| 31 | multihop-ebgp-bird | Ze, BIRD | Multi-hop eBGP with outgoing-ttl |
| 32 | multihop-ebgp-gobgp | Ze, GoBGP | Multi-hop eBGP with outgoing-ttl |
| 33 | bfd-frr | Ze, FRR | BFD opt-in and BFD-triggered BGP teardown |
| 34 | ecmp-frr | Ze, FRR, GoBGP | FRR ECMP selection for the same prefix from Ze and GoBGP |
| 35 | srv6-frr | Ze, FRR | SRv6 VPNv6 route exchange and Prefix-SID handling |
| 36 | remove-private-as-frr | Ze, FRR, GoBGP | remove-private-as export policy to FRR |
| 37 | remove-private-as-as4path-frr | Ze, FRR, BIRD | remove-private-as handling for AS4_PATH private ASNs |
<!-- source: test/interop/scenarios/ -- scenario directories -->

### Running

```bash
./le integration interop
INTEROP_SCENARIO=bgp-ebgp-ipv4-frr ./le integration interop
VERBOSE=1 ./le integration interop
NO_BUILD=1 ./le integration interop
FRR_IMAGE=quay.io/frrouting/frr:10.3 ./le integration interop
```

Interop tests require Docker and are not part of the offline precommit gate.
They are a separate protocol-validation action.

The first run builds Docker images (takes a few minutes). Subsequent runs with `NO_BUILD=1`
skip rebuilds. The full suite takes roughly 5-10 minutes depending on session establishment
times.

### Debugging Failures

On failure, the orchestrator automatically dumps the last 20 lines of container logs.
For more detail:

- `VERBOSE=1` enables debug output (polling status, container commands, raw CLI output)
- `SESSION_TIMEOUT=120` increases the session establishment timeout (default 90s)
- Single-scenario runs isolate the problem: `INTEROP_SCENARIO=bgp-graceful-restart-frr ./le integration interop`

### Writing a New Scenario

1. Create a descriptively named directory under `test/interop/scenarios/`.
2. Add `ze.conf` and whichever peer configs the scenario needs.
3. Add the scenario's ordered assertions to `scenarioOperations` and
   `scenarioExtras`, or register a bespoke checker in `specialCheckers` when the
   control flow is non-linear.
4. For a bespoke checker, put each decision in a pure predicate in
   `check_rfc_predicate.go` and add its both-polarity subtest to
   `TestBespokeCheckerBranches`.
5. Run `INTEROP_SCENARIO=<name> ./le integration interop`.

`TestCheckerPopulationMatchesProducer` compares every scenario directory with
the package-local registry. `TestEveryCheckerFailsClosedWithoutPeerEvidence`
rejects a checker that can pass without reading a peer. Each negative assertion
must carry positive proof that its query mechanism ran.
<!-- source: internal/le/interoplab/bgp/checkers.go -- checkers -->

#### A scenario that reads the RIB attaches the RIB plugin

`plugin { internal rib { use bgp-rib; } }` loads the plugin. It does NOT feed it.
A peer delivers an event to a process only where both halves agree, and the
peer's half is its attach block (`Server.PeerScopedProcs`,
`internal/component/plugin/server/delivery_graph.go`). A peer with no
`attach process rib` block grants nothing, so the plugin sees no peer at all and
every RIB question answers empty:

```
	attach process rib {
		receive [ update state refresh ];
	}
```

The tell is `"peers": 0` from `show bgp rib status` while `show bgp peer list`
reports both sessions Established. Ze also logs it at startup: *"the plugin
declared events and no peer attaches it"*.

#### Editing a config means recreating the container, not restarting it

Ze persists its configuration, so `docker restart` on a scenario container runs
the peers of the FIRST boot and ignores the edited file the mount now carries.
`docker exec ... cat /etc/ze/bgp.conf` shows the new text while `show bgp peer
list` shows the old peers, which reads as a config that had no effect. Remove
the container and start a new one instead. Every restart-based config
experiment in a hand-built lab is void, and the harness is unaffected because it
creates each container once.

For Ze's configuration syntax, see [docs/architecture/config/syntax.md](https://github.com/ze-software/ze/blob/main/docs/architecture/config/syntax.md).
Copy an existing scenario's `ze.conf` as a starting point.

## ExaBGP Wire Compatibility (`test/exabgp-compat/`)

A separate test suite validates that Ze's wire encoding matches the reviewed
ExaBGP API 6.0.0 contract fixtures. The compiled Go server negotiates each BGP
session and compares every received frame byte-for-byte.

### What It Tests

The harness migrates each ExaBGP-derived configuration, runs Ze, and compares
its wire bytes with the known-good fixture. The 42 `.ci` cases in
`test/exabgp-compat/encoding/` use `option=file:`, `option=serial`, `1:cmd:`,
`1:raw:`, and `1:json:` records rather than the standard `.ci` format.
`option=serial` marks process-driven fixtures that must not overlap other ExaBGP
harness instances; the runner executes those after the parallel batch.

Coverage includes:

| Category | Examples |
|----------|----------|
| Address families | IPv4/IPv6 unicast, VPN, FlowSpec, FlowSpec VPN, EVPN, VPLS, MPLS labeled, MUP, MVPN |
| Path attributes | ORIGIN, AS_PATH, NEXT_HOP, MED, LOCAL_PREF, communities (standard, extended, large), AGGREGATOR, ORIGINATOR_ID, PREFIX_SID, SRv6 |
| Capabilities | 4-byte ASN, ADD-PATH, link-local next-hop, software version, hostname |
| Edge cases | Generic/unknown attributes, self-referencing routes, group limits, IPv4+IPv6 mixed configs, deferred announcement (watchdog) |

### Running

```bash
./le functional exabgp-test
```

ExaBGP compatibility is part of the offline precommit gate.
<!-- source: test/exabgp-compat/encoding/ -- .ci test files for wire compatibility -->

## Test Hierarchy

| Workflow | Includes Interop? | Includes ExaBGP? | Requires Docker? |
|----------|-------------------|-------------------|-------------------|
| Offline precommit gate | No | Yes | No |
| Standard functional sweep | No | Yes | No |
| `./le integration interop` | Yes | No | Yes |
| `./le functional exabgp-test` | No | Yes | No |

Interop tests are intentionally separate from the pre-commit gate because they require
Docker and take longer to run. ExaBGP wire compatibility tests run as part of the
standard verification suite.

## Current Scope

Interop scenarios cover core BGP: session establishment, route exchange, withdrawal,
capabilities (4-byte ASN, ADD-PATH, GR, route refresh, PATHS-LIMIT), communities, MD5
auth, route server behavior, route reflection, policy import/export, RPKI origin
validation, BMP monitoring, BFD failover, ECMP, SRv6 VPNv6, remove-private-as export
policy, GTSM, AS112, and non-unicast address families (EVPN, VPN, FlowSpec). The suite
also includes full IS-IS and OSPFv2/OSPFv3 interop families (adjacency, flooding, SPF,
dual-stack, authentication, TE, LFA/TI-LFA, graceful restart, and segment routing).
ExaBGP compat covers wire encoding for all supported address families.
<!-- source: test/interop/scenarios/ -- scenario directories -->

Not yet covered by interop tests:

- Long-Lived Graceful Restart with live peers

## Known Vendor Limitations

| Vendor | Limitation | Affected Scenario | Workaround |
|--------|-----------|-------------------|------------|
| GoBGP 3.31 | Deduplicates Multiprotocol capabilities by AFI. When two families share the same AFI (e.g., ipv4-unicast + l3vpn-ipv4-unicast, both AFI=1), GoBGP keeps only one. | bgp-vpn-gobgp | None from Ze side. Ze's OPEN is correct per RFC 4760. Families with different AFIs (e.g., ipv4-unicast + l2vpn-evpn) work fine. |
| BIRD 2.15 | Enforces next-hop reachability for IPv6 routes. On IPv4-only Docker networks, IPv6 next-hops are unreachable and BIRD rejects routes as invalid (RFC 7606 treat-as-withdraw). | bgp-ipv6-ebgp-bird | Add `multihop;` to BIRD config to disable the directly-connected next-hop check. |

<!-- source: test/interop/scenarios/bgp-vpn-gobgp -- GoBGP same-AFI dedup -->
<!-- source: test/interop/scenarios/bgp-ipv6-ebgp-bird -- BIRD next-hop reachability -->

## Related Documents

- [`.ci` test format](../ci-format/index.md) -- Ze's standard functional test file format
- [Functional test system](https://github.com/ze-software/ze/blob/main/docs/functional-tests.md) -- complete guide to the functional test system
- [BGP implementation comparison](https://github.com/ze-software/ze/blob/main/docs/comparison.md) -- feature matrix comparing Ze with FRR, BIRD, GoBGP, ExaBGP, and others
- [ExaBGP comparison report](https://github.com/ze-software/ze/blob/main/docs/exabgp/exabgp-comparison-report.md) -- detailed implementation differences between Ze and ExaBGP
