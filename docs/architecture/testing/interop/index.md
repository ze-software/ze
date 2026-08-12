# Interoperability Testing

Ze validates protocol correctness against production BGP daemons in two complementary ways:
live session interop tests (Docker containers running real daemons) and byte-level wire format
validation against ExaBGP (Ze's predecessor, a BGP implementation in Python).

For BGP terminology used in this document, see [docs/features.md](../../../features/index.md).

## Tested Daemons

| Daemon | Version | Image | Query Method | What It Validates |
|--------|---------|-------|--------------|-------------------|
| FRR | 10.3.1 | `quay.io/frrouting/frr:10.3.1` | vtysh | eBGP, iBGP, route exchange, GR, communities, MD5, route server |
| BIRD | 2.x (Alpine 3.21) | Alpine build | birdc | eBGP, route exchange, triangle topologies |
| GoBGP | 3.31.0 | Go builder | gobgp CLI | eBGP, route injection and verification |
| StayRTR | 0.6.4 | Go builder | HTTP `/rpki.json` export | RTR (RFC 8210) as the CACHE, so Ze is the client of an implementation that is not its own. Origin validation answers (RFC 6811) against VRPs a third party encoded |
| ExaBGP | main (API 6.0.0) | Python harness | Wire byte comparison | Byte-for-byte encoding across all address families |
<!-- source: test/interop/interop.py -- FRR, BIRD, GoBGP, Ze helpers -->
<!-- source: test/interop/run.py -- scenario orchestrator -->

## Prerequisites

| Requirement | Used By | Notes |
|-------------|---------|-------|
| Docker | Interop tests | Containers for FRR, BIRD, GoBGP, Ze |
| Python 3 | Interop tests | Orchestrator and scenario assertions |
| `uv` | ExaBGP compat | Auto-installs `psutil` and `paramiko` dependencies |
| ~1.5 GB disk | Interop tests | Docker images (Go builder, FRR, Alpine) |

The interop test network uses `172.30.0.0/24`. MD5 authentication scenarios require
`NET_ADMIN` capability (granted automatically by the orchestrator).

## Live Interop Tests (`test/interop/`)

Each scenario runs Ze and one or more peer daemons in Docker containers on a shared
network (`172.30.0.0/24`), establishes real BGP sessions, and asserts correct behavior
via each daemon's native CLI.

### How It Works

A Python orchestrator (`run.py`) iterates over scenario directories in `test/interop/scenarios/`.
For each scenario, `interop.py` manages the container lifecycle:

1. Create a Docker network (`172.30.0.0/24`)
2. Start Ze (always) and peer daemons (conditionally, based on which config files exist in the scenario directory)
3. Wait for all containers to become healthy
4. Import and run the scenario's `check.py`
5. Tear down containers and network
<!-- source: test/interop/interop.py -- container lifecycle, network creation -->

Daemons start conditionally: FRR if `frr.conf` exists, BIRD if `bird.conf` exists,
GoBGP if `gobgp.toml` exists. This means each scenario only runs the daemons it needs.

### Container Addresses

| Daemon | IP | Container |
|--------|----|-----------|
| Ze | 172.30.0.2 | `ze-iop-ze-<pid>` |
| FRR | 172.30.0.3 | `ze-iop-frr-<pid>` |
| BIRD | 172.30.0.4 | `ze-iop-bird-<pid>` |
| GoBGP | 172.30.0.5 | `ze-iop-gobgp-<pid>` |
| Raw injector | 172.30.0.9 | `ze-iop-inject-<pid>` |
| Python speaker | 172.30.0.10 | `ze-iop-speaker-<pid>` |
| Python speaker (2nd) | 172.30.0.11 | `ze-iop-speaker2-<pid>` |
| StayRTR | 172.30.0.12 | `ze-iop-stayrtr-<pid>` |

Container names include the runner PID as suffix, so concurrent runs do not conflict.
<!-- source: test/interop/interop.py -- container naming, IP addresses -->

### Scenario Structure

Each scenario is a directory under `test/interop/scenarios/`:

```
scenarios/01-ebgp-ipv4-frr/
  ze.conf        # Ze configuration (required)
  frr.conf       # FRR configuration (starts FRR container)
  check.py       # Python assertions (required)
```

The `check.py` file defines a `check()` function that uses daemon helper classes
(`FRR`, `BIRD`, `GoBGP`, `Ze`) from `interop.py` to query sessions, routes, and
attributes via each daemon's native CLI.

### Optional sidecars

A scenario directory may also carry files that start extra containers before Ze,
alongside `rpki-server` and `bmp-collector.py`:

| File | Sidecar | Purpose |
|------|---------|---------|
| `inject.msg` | `ze-test peer` (raw injector, 172.30.0.9) | Drive Ze with wire bytes no conforming daemon would emit -- e.g. an UPDATE mixing Withdrawn Routes with NLRI (RFC 7606 Section 5.1), which every receiver must accept but no sender may produce. Ze dials it (`accept false` in `ze.conf`), so the injector runs `ze-test peer` in check mode against the `inject.msg` expect/action script. An optional `inject-args` file adds flags (`--asn` is important, or the peer adopts Ze's ASN). Because the injector and Ze start before the peer daemons, a route the injector announces is stored in Ze before FRR connects, so it is delivered by Ze's replay-on-peer-up path -- useful for testing the re-encode/replay rail specifically. |
| `speaker-args` (and optional `speaker2-args`) | Minimal Python speaker (172.30.0.10; second at 172.30.0.11) | Dial Ze with an INDEPENDENT strict peer that applies one per-test check. The fixed engine (`test/interop/speaker/engine.py`) establishes, loads a plugin named in `speaker-args` (`--test /speaker/plugins/<name>.py`), inspects every UPDATE, and prints a verdict `check.py` reads via `docker logs`. Unlike `ze-test peer`, which asserts only the bytes it was told to expect, a speaker plugin runs its own validator -- e.g. RFC 7606 Section 3(g) duplicate attributes -- so it catches wire output Ze's own lenient validator waves through. Started after Ze (like the daemons), so it exercises the replay rail; keep it connected with `--stop-after-updates 0` when the check bytes arrive on Ze's delta-replay rather than the first initial-sync UPDATE. A `speaker2-args` file starts a second instance at a distinct IP/router-id (scenario 49 proves two engines establish without colliding). See `plan/spec-bgp-plugin-speaker.md`. |
| `vrps.json` | StayRTR (172.30.0.12:8282) | Serve RPKI VRPs from a real third-party cache, so Ze is the RTR client of an implementation that is not its own. `rpki-server` above runs `ze-test rpki`, which is Ze's encoder answering Ze's decoder, and that pair agrees with itself whatever it does. The file is the rpki-client / Routinator JSON export format: `roas` of prefix, `maxLength` and `asn`. StayRTR serves the same set back on `http://172.30.0.12:9847/rpki.json`. A `check.py` reads that export to learn what the CACHE meant, rather than what the scenario intended. A cache-side decode fault fails OPEN, because every prefix then reads NotFound and `not-found accept` accepts it. So assert the per-prefix validation ANSWER, never the session. Worked example: `58-rtr-stayrtr`. |

<!-- source: test/interop/interop.py -- inject.msg sidecar startup, INJECT_CONTAINER -->
<!-- source: test/interop/interop.py -- speaker-args sidecar startup, SPEAKER_CONTAINER -->
<!-- source: test/interop/interop.py -- vrps.json sidecar startup, STAYRTR_CONTAINER -->
<!-- source: test/interop/scenarios/47-rfc7606-relay-shape-frr/ -- injector worked example -->
<!-- source: test/interop/scenarios/48-rfc7606-speaker-dup-attr/ -- speaker worked example -->
<!-- source: test/interop/scenarios/58-rtr-stayrtr/ -- StayRTR worked example -->

### Prove a scenario discriminates

An interop scenario is evidence only if it goes RED when the behaviour it tests is
broken. Before relying on a new scenario, revert the fix, rebuild the `ze-interop`
image (`docker build -f test/interop/Dockerfile.ze -t ze-interop .`), and confirm the
scenario fails; then restore and confirm it passes. A scenario that passes either way
(common when the peer must accept both the old and new wire form) proves acceptance,
not correctness -- see `ai/rules/interop-and-goal-validation.md` "Prove the test
discriminates".

### Daemon Helpers

`interop.py` provides helper classes for querying each daemon:

Methods follow a naming convention:

| Prefix | Behavior | Example |
|--------|----------|---------|
| `wait_` | Poll until condition is true, raise on timeout | `wait_session`, `wait_route` |
| `check_` | Assert condition, raise immediately if false | `check_route`, `check_route_community` |
| `has_` | Return bool, no exception | `has_route` |

All classes (`FRR`, `BIRD`, `GoBGP`, `Ze`) are defined in `interop.py`. Each wraps the
daemon's native CLI (`vtysh`, `birdc`, `gobgp`, `ze`) via `docker exec`. Start with an
existing scenario (e.g., `01-ebgp-ipv4-frr/check.py`) as a template.

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
every `ze.conf` (`_render_scenario_dir`), so no scenario carries the boilerplate
and none can forget it. A scenario that forgot it would fail its assertions for a
reason unrelated to what it tests. `test/ipsec-interop/lab.py` does the same thing
for the IPsec lab.

**A Ze helper never converts a failed query into a plausible number.**
`Ze.rib_count` raises when the command fails or answers without a `routes-in`
field, because 0 is a legitimate RIB size and a failed query is not
(`ai/rules/evidence.md`). It returned 0 on failure until 2026-08-07 and three
separate faults hid behind that one number for three days
(`plan/spec-fixit-test-harness-fail-open-guards.md`, guard 3). Write new Ze
helpers the same way.

All session waiters poll with a configurable timeout (default 90s, override via `SESSION_TIMEOUT` env var).
The harness passes that value into the Ze container as `SESSION_TIMEOUT`, so a process
plugin can size its own barriers against the harness budget instead of repeating a
constant (`docker_run(..., env=)`).
<!-- source: test/interop/interop.py -- wait_session, wait_route, check_route -->

### A process plugin that fails

A scenario can drive Ze from a process plugin (`test/scripts/ze_api.py`). When such a
plugin calls `runtime_fail`, it writes the `ZE-OBSERVER-FAIL` sentinel to its stderr
and stops Ze. The `.ci` runner rejects that sentinel in `validateLogging`; this lab
has no such reject, so the plugin's failure would otherwise reach `check.py` as a
route that never arrived, or as `containers not healthy` 30 seconds later.

`raise_if_observer_failed(when)` reads the sentinel from the LAST 2000 lines of Ze's
log and raises with the plugin's own message. The tail is the right end to read:
`runtime_fail` requests shutdown immediately after writing the sentinel, so Ze stops
within a few lines of it. A scenario whose Ze writes more than 2000 lines after the
sentinel must raise the `lines` bound.

An unreadable log raises as well, with the docker error rather than the plugin's
message. "I could not look" is not "the plugin is fine".

An unreadable log is NOT a plugin verdict, and the two are worded differently. The
sentinel case names the plugin as the cause. The unreadable case says a plugin failure
cannot be ruled out. `observer_failure_note` returns the finished line so one writer
states the claim: a caller that added its own prefix asserted the plugin as the cause
for every scenario that failed before Ze's container existed.

Three call sites, and the first covers every failure the other two miss:

| Site | Fires when |
|------|-----------|
| `run.py`, the scenario's `except BaseException` handler | on every scenario failure other than a Ctrl-C. An interrupt is counted and ends the loop, whether it arrives before this point or during the read itself, so the run still prints its summary. Uses `observer_failure_note`, which returns text instead of raising, because a second exception there would replace the failure being reported |
| `wait_containers_healthy` | only when the plugin already stopped Ze before the health loop gave up. A race, measured as such on `11-addpath-frr` |
| `check.py`, before the first wait and again when a wait fails | when the scenario knows which of its own waits the plugin can outrun |
<!-- source: test/interop/interop.py -- observer_fail_line, raise_if_observer_failed, observer_failure_note -->
<!-- source: test/interop/run.py -- main -->
<!-- source: test/interop/scenarios/55-wire-edit-api-origin-bird/check.py -- worked example -->

### Scenario Inventory

The suite has grown to over 100 scenario directories in `test/interop/scenarios/`. The table
below lists the core BGP scenarios (01-37); beyond these, the suite also covers route
reflection, policy import/export, RPKI origin validation, BMP monitoring, PATHS-LIMIT,
max-prefix cease, GTSM, AS112, and full IS-IS (auth, convergence, dual-stack, LAN DIS,
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
make ze-interop-test                                  # all scenarios
make ze-interop-test INTEROP_SCENARIO=01-ebgp-ipv4-frr  # single scenario
VERBOSE=1 make ze-interop-test                         # debug output
NO_BUILD=1 make ze-interop-test                        # skip image rebuilds
FRR_IMAGE=quay.io/frrouting/frr:10.3 make ze-interop-test  # override FRR version
```

Interop tests require Docker and are not part of `make ze-verify` (which runs without
Docker). They are available as a separate target for protocol validation.

The first run builds Docker images (takes a few minutes). Subsequent runs with `NO_BUILD=1`
skip rebuilds. The full suite takes roughly 5-10 minutes depending on session establishment
times.

### Debugging Failures

On failure, the orchestrator automatically dumps the last 20 lines of container logs.
For more detail:

- `VERBOSE=1` enables debug output (polling status, container commands, raw CLI output)
- `SESSION_TIMEOUT=120` increases the session establishment timeout (default 90s)
- Single-scenario runs isolate the problem: `make ze-interop-test INTEROP_SCENARIO=13-graceful-restart-frr`

### Writing a New Scenario

1. Create `test/interop/scenarios/NN-description/`
2. Write `ze.conf` (required) and peer configs (`frr.conf`, `bird.conf`, `gobgp.toml`) as needed
3. Write `check.py` with a `check()` function that imports helpers from `interop`
4. Run `make ze-interop-test INTEROP_SCENARIO=NN-description`

Example `check.py`:

```python
import sys, os
# Make the interop module (two directories up) importable.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from interop import FRR, ZE_IP

def check():
    frr = FRR()
    frr.wait_session(ZE_IP)
    frr.check_route("10.0.0.0/24")
```

For Ze's configuration syntax, see [docs/architecture/config/syntax.md](https://github.com/ze-software/ze/blob/main/docs/architecture/config/syntax.md).
Copy an existing scenario's `ze.conf` as a starting point.

## ExaBGP Wire Compatibility (`test/exabgp-compat/`)

A separate test suite validates that Ze's wire encoding produces identical bytes to
ExaBGP (main branch, JSON API 6.0.0). Rather than establishing live BGP sessions, it
compares encoded output byte-for-byte using a Python harness.

### What It Tests

The harness runs Ze with ExaBGP-derived configurations and compares the wire bytes Ze
produces against known-good ExaBGP output. 42 test cases are defined as `.ci` files in
`test/exabgp-compat/encoding/`. These `.ci` files use a format specific to the ExaBGP
compat harness (`option=file:`, `option=serial`, `1:cmd:`, `1:raw:`, `1:json:` lines),
not the [standard `.ci` format](../ci-format/index.md) used by Ze's functional tests.
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
make ze-exabgp-test   # runs via uv with psutil dependency
```

ExaBGP compatibility is part of `make ze-verify` (the pre-commit gate).
<!-- source: test/exabgp-compat/encoding/ -- .ci test files for wire compatibility -->

## Test Hierarchy

| Target | Includes Interop? | Includes ExaBGP? | Requires Docker? |
|--------|-------------------|-------------------|-------------------|
| `make ze-verify` | No | Yes | No |
| `make ze-test` | No | Yes | No |
| `make ze-interop-test` | Yes | No | Yes |
| `make ze-exabgp-test` | No | Yes | No |

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
| GoBGP 3.31 | Deduplicates Multiprotocol capabilities by AFI. When two families share the same AFI (e.g., ipv4-unicast + l3vpn-ipv4-unicast, both AFI=1), GoBGP keeps only one. | 29-vpn-gobgp | None from Ze side. Ze's OPEN is correct per RFC 4760. Families with different AFIs (e.g., ipv4-unicast + l2vpn-evpn) work fine. |
| BIRD 2.15 | Enforces next-hop reachability for IPv6 routes. On IPv4-only Docker networks, IPv6 next-hops are unreachable and BIRD rejects routes as invalid (RFC 7606 treat-as-withdraw). | 25-ipv6-ebgp-bird | Add `multihop;` to BIRD config to disable the directly-connected next-hop check. |

<!-- source: test/interop/scenarios/29-vpn-gobgp -- GoBGP same-AFI dedup -->
<!-- source: test/interop/scenarios/25-ipv6-ebgp-bird -- BIRD next-hop reachability -->

## Related Documents

- [`.ci` test format](../ci-format/index.md) -- Ze's standard functional test file format
- [Functional test system](https://github.com/ze-software/ze/blob/main/docs/functional-tests.md) -- complete guide to the functional test system
- [BGP implementation comparison](https://github.com/ze-software/ze/blob/main/docs/comparison.md) -- feature matrix comparing Ze with FRR, BIRD, GoBGP, ExaBGP, and others
- [ExaBGP comparison report](https://github.com/ze-software/ze/blob/main/docs/exabgp/exabgp-comparison-report.md) -- detailed implementation differences between Ze and ExaBGP
