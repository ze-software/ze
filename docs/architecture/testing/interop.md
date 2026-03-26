# Interoperability Testing

Ze validates protocol correctness against production BGP daemons in two complementary ways:
live session interop tests (Docker containers running real daemons) and byte-level wire format
validation against ExaBGP (Ze's predecessor, a BGP implementation in Python).

For BGP terminology used in this document, see [docs/features.md](../../features.md).

## Tested Daemons

| Daemon | Version | Image | Query Method | What It Validates |
|--------|---------|-------|--------------|-------------------|
| FRR | 10.3.1 | `quay.io/frrouting/frr:10.3.1` | vtysh | eBGP, iBGP, route exchange, GR, communities, MD5, route server |
| BIRD | 2.x (Alpine 3.21) | Alpine build | birdc | eBGP, route exchange, triangle topologies |
| GoBGP | 3.31.0 | Go builder | gobgp CLI | eBGP, route injection and verification |
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

All session waiters poll with a configurable timeout (default 90s, override via `SESSION_TIMEOUT` env var).
<!-- source: test/interop/interop.py -- wait_session, wait_route, check_route -->

### Scenario Inventory

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

For Ze's configuration syntax, see [docs/architecture/config/syntax.md](../config/syntax.md).
Copy an existing scenario's `ze.conf` as a starting point.

## ExaBGP Wire Compatibility (`test/exabgp-compat/`)

A separate test suite validates that Ze's wire encoding produces identical bytes to
ExaBGP (main branch, JSON API 6.0.0). Rather than establishing live BGP sessions, it
compares encoded output byte-for-byte using a Python harness.

### What It Tests

The harness runs Ze with ExaBGP-derived configurations and compares the wire bytes Ze
produces against known-good ExaBGP output. 38 test cases are defined as `.ci` files in
`test/exabgp-compat/encoding/`. These `.ci` files use a format specific to the ExaBGP
compat harness (`option=file:`, `1:cmd:`, `1:raw:`, `1:json:` lines), not the
[standard `.ci` format](ci-format.md) used by Ze's functional tests.

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
capabilities (4-byte ASN, ADD-PATH, GR, route refresh), communities, MD5 auth, and
route server behavior. ExaBGP compat covers wire encoding for all supported address
families.

Not yet covered by interop tests:

- Long-Lived Graceful Restart with live peers (LLGR not yet implemented)
- BFD (no BFD protocol support in Ze)

## Related Documents

- [`.ci` test format](ci-format.md) -- Ze's standard functional test file format
- [Functional test system](../../functional-tests.md) -- complete guide to the functional test system
- [BGP implementation comparison](../../comparison.md) -- feature matrix comparing Ze with FRR, BIRD, GoBGP, ExaBGP, and others
- [ExaBGP comparison report](../../exabgp/exabgp-comparison-report.md) -- detailed implementation differences between Ze and ExaBGP
