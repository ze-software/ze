# Spec: bgp-chaos (Master Design Document)

**This is the master architecture document. It is NOT an active implementation spec.**
**Implementation is split into 9 sub-specs, worked sequentially:**

| # | Spec | Status | Focus |
|---|------|--------|-------|
| 1 | `spec-bgp-chaos-session.md` | Done (255) | Single-peer BGP session |
| 2 | `spec-bgp-chaos-validation.md` | Done (256) | Multi-peer validation |
| 3 | `spec-bgp-chaos-chaos.md` | Done (257) | Chaos event injection |
| 4 | `spec-bgp-chaos-families.md` | Done (258) | Multi-family support |
| 5 | `spec-bgp-chaos-reporting.md` | Done | Dashboard, JSON log, Prometheus |
| 6 | `spec-bgp-chaos-eventlog.md` | Skeleton | Replayable event log |
| 7 | `spec-bgp-chaos-properties.md` | Skeleton | RFC property assertions |
| 8 | `spec-bgp-chaos-shrink.md` | Skeleton | Test case minimization |
| 9 | `spec-bgp-chaos-inprocess.md` | Skeleton | In-process mode (DST bridge) |

**Phases 1-5:** Core chaos tool (external, black-box TCP testing)
**Phases 6-9:** DST bridge (toward deterministic simulation, see `deterministic-simulation-analysis.md`)

**Dependencies:** Phase 9 requires Ze clock + network abstractions (implemented separately)

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/core-design.md` - system architecture
4. `internal/plugins/bgp/reactor/reactor.go` - reactor internals
5. `internal/plugins/bgp/message/update_build.go` - UPDATE building API
6. `internal/test/peer/peer.go` - existing test peer (reference, not reused)

## Task

Build `ze-bgp-chaos`, a long-lived chaos monkey tool for testing Ze's BGP route server (route reflector) route propagation behavior.

The tool:
- Simulates 3-50 configurable BGP peers in a single Go binary
- Uses a seed for deterministic, reproducible scenario generation
- Generates predictable UPDATEs (announce + withdraw) across multiple address families
- Validates that the route reflector correctly propagates routes between peers
- Injects chaos events (disconnects, hold-timer expiry, malformed messages, etc.)
- Generates the matching Ze config so both sides are aligned
- Runs as a long-lived process for continuous testing (chaos monkey) or short CI runs

**Primary goal:** Find route propagation bugs that only surface under realistic multi-peer churn with disconnections.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - system architecture, zero-copy forwarding, UPDATE structure
  → Decision: Engine forwards wire bytes via msg-id cache; plugins (RR) decide forwarding
  → Constraint: Route reflector uses `cache <msg-id> forward !<source-peer>` command syntax
- [ ] `docs/architecture/wire/messages.md` - BGP message wire format
  → Constraint: IPv4/unicast uses inline NLRI; all other families use MP_REACH_NLRI
- [ ] `docs/architecture/wire/nlri.md` - NLRI types per family
  → Constraint: Each family has distinct NLRI encoding (prefix, labeled, VPN RD+prefix, EVPN type, FlowSpec rules)

### Source Code (key files)
- [ ] `internal/plugins/bgp/reactor/reactor.go` - reactor orchestration
- [ ] `internal/plugins/bgp/reactor/peersettings.go` - PeerSettings struct (config per peer)
- [ ] `internal/plugins/bgp/message/update_build.go` - UpdateBuilder API for constructing UPDATEs
- [ ] `internal/plugins/bgp/message/open.go` - OPEN message packing
- [ ] `internal/plugins/bgp/capability/encoding.go` - EncodingCaps (families, ADD-PATH, ASN4)
- [ ] `internal/test/peer/peer.go` - existing test peer (reference for BGP handshake logic)

**Key insights:**
- UpdateBuilder (NewUpdateBuilder) constructs valid UPDATE messages with proper attribute/NLRI placement
- OPEN messages built via message.PackTo(&Open{...}, capabilityBytes)
- Reactor listens on per-peer LocalAddress; peers can be passive or active
- RR plugin forwards all UPDATEs to all compatible established peers (family match)
- Config uses JUNOS-like syntax: `bgp { peer <addr> { ... } }`

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp-rr/server.go` - Route reflector plugin: receives UPDATE events, forwards via `cache <id> forward !<source>` to all other peers that support the family
- [ ] `internal/plugins/bgp/reactor/reactor.go` - Manages peer lifecycle, message dispatching, recent UPDATE cache
- [ ] `internal/test/peer/peer.go` - Test peer: sink/echo/check modes, single-peer-at-a-time

**Behavior to preserve:**
- Ze config syntax unchanged
- RR plugin behavior unchanged (this tool tests it, doesn't modify it)
- No modifications to any existing Ze code

**Behavior to change:**
- None - this is a NEW standalone tool

## Data Flow (MANDATORY)

### Entry Point
- `ze-bgp-chaos` binary starts, generates scenario from seed
- Writes Ze config to `--config-out` path (or prints to stdout)
- User starts Ze with that config: `ze <generated-config.conf>`
- Tool opens TCP connections to Ze (and listens for Ze's outbound connections)

### Transformation Path

```
Seed + CLI flags
    ↓
ScenarioGenerator: produces PeerProfile[] with families, routes, chaos schedule
    ↓
ConfigGenerator: writes Ze config matching the scenario
    ↓
PeerSimulators (goroutine per peer): each runs independent BGP session
    ↓
    ├─ OPEN exchange (capabilities per peer profile)
    ├─ Initial route burst (announce phase)
    ├─ Steady-state churn (announce/withdraw at configurable rate)
    ├─ Chaos events (disconnect, hold-timer, malformed, collision)
    └─ Receive + validate forwarded routes from RR
    ↓
ValidationEngine: compares received routes against expected propagation model
    ↓
Reporter: live dashboard + JSON log + summary + optional Prometheus
```

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Tool ↔ Ze Engine | TCP BGP sessions (wire bytes) | [ ] |
| Ze Engine ↔ RR Plugin | JSON events over Unix sockets (internal to Ze) | N/A (black-box) |

### Integration Points
- Uses Ze's `message` package to build valid OPEN/UPDATE/KEEPALIVE/NOTIFICATION
- Uses Ze's `capability` package for family/capability definitions
- Uses Ze's `nlri` package for NLRI construction per family
- Uses Ze's `attribute` package for path attribute construction
- Does NOT import reactor, RR plugin, or any engine-internal code

### Architectural Verification
- [ ] No bypassed layers (tool is a pure external BGP peer)
- [ ] No unintended coupling (only imports wire-format packages)
- [ ] No duplicated functionality (reuses Ze's encoding, doesn't reimplement)
- [ ] Tool is black-box: tests Ze through its BGP TCP interface only

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze-bgp-chaos --seed 42 --peers 4 --duration 30s` | Runs 4 peers for 30s, prints seed, exits with summary |
| AC-2 | Same seed produces same scenario | Route sets, chaos events, peer profiles are identical |
| AC-3 | Peer A announces route R (ipv4/unicast) | All other established peers supporting ipv4/unicast receive R from RR |
| AC-4 | Peer A withdraws route R | All other peers receive withdrawal |
| AC-5 | Peer A disconnects (TCP close) | All other peers receive withdrawals for A's routes |
| AC-6 | Peer A reconnects after disconnect | A receives all routes from other established peers |
| AC-7 | Disconnect during initial route burst | Partial routes withdrawn, remaining announced on reconnect |
| AC-8 | iBGP peer and eBGP peer coexist | Both receive appropriate routes from RR |
| AC-9 | Peers with different family sets | Peer only receives routes for families it negotiated |
| AC-10 | Hold-timer expiry chaos event | Ze detects hold-timer expiry, tears down session |
| AC-11 | `--families ipv4/unicast,ipv6/unicast` | Only those families used in scenario |
| AC-12 | `--exclude-families l2vpn/evpn` | EVPN excluded from all peer profiles |
| AC-13 | `--config-out ze-chaos.conf` | Valid Ze config written that matches the scenario |
| AC-14 | `--chaos-rate 0` | No chaos events, pure route propagation test |
| AC-15 | `--chaos-rate 1.0` | Maximum chaos, every interval triggers an event |
| AC-16 | Live stdout shows per-peer state | Peer address, state, routes sent/received, errors |
| AC-17 | Exit summary reports convergence stats | Routes announced/received/missing, avg convergence time, errors found |
| AC-18 | Multi-family peers (v4+v6+flow) | Routes generated and validated for each family independently |
| AC-19 | Connection collision (both sides connect) | Tool handles gracefully, one connection wins |

## Design

### Binary Location

`cmd/ze-bgp-chaos/main.go` — standalone binary, built alongside `ze`, `ze-peer`, `ze-test`

### CLI Interface

```
ze-bgp-chaos - Chaos monkey for Ze BGP route server testing

Usage:
  ze-bgp-chaos [options]

Scenario:
  --seed <uint64>            Deterministic seed (default: random, always printed)
  --peers <N>                Number of simulated peers (default: 4, max: 50)
  --ibgp-ratio <float>       Fraction of peers that are iBGP (default: 0.3)

Routes:
  --routes <N>               Base routes per peer (default: 100)
  --heavy-peers <N>          Peers sending many routes (default: 1)
  --heavy-routes <N>         Routes for heavy peers (default: 2000)
  --churn-rate <N/s>         Route changes per second per peer in steady state (default: 5)

Families:
  --families <list>          Only these families (comma-sep, default: all)
  --exclude-families <list>  Exclude these families (comma-sep)

Chaos:
  --chaos-rate <float>       Probability of chaos per interval (default: 0.1, range 0.0-1.0)
  --chaos-interval <dur>     Time between chaos checks (default: 10s)

Network:
  --port <N>                 Base BGP port for Ze to listen on (default: 1790)
  --listen-base <N>          Base port for tool to listen on (default: 1890)
  --local-addr <addr>        Local address (default: 127.0.0.1)

Output:
  --config-out <path>        Write Ze config here (default: stdout before start)
  --log <path>               JSON event log file
  --metrics <addr:port>      Prometheus metrics endpoint
  --quiet                    Only errors and summary
  --verbose                  Extra debug output

Control:
  --duration <dur>           Max runtime (default: 0 = run forever until Ctrl-C)
  --warmup <dur>             Time before chaos starts (default: 5s, allow initial convergence)
```

### Supported Address Families

| Family | NLRI Generation Strategy |
|--------|--------------------------|
| ipv4/unicast | Random /24 prefixes from 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16 |
| ipv6/unicast | Random /48 prefixes from 2001:db8::/32 (documentation range) |
| ipv4/vpn | RD (type 0, peer-derived) + random IPv4 /24 |
| ipv6/vpn | RD (type 0, peer-derived) + random IPv6 /48 |
| l2vpn/evpn | Type-2 MAC/IP routes with generated MACs and IPs |
| ipv4/flow | Source/destination prefix match rules |
| ipv6/flow | Source/destination IPv6 prefix match rules |

Each peer gets a unique slice of address space (partitioned by seed) so routes are distinguishable by origin.

### Component Architecture

```
ze-bgp-chaos
├── main.go                    CLI entry point, flag parsing
├── scenario/
│   ├── generator.go           Seed → PeerProfile[] + ChaosSchedule
│   ├── profile.go             PeerProfile: ASN, families, route count, caps
│   ├── routes.go              Route generation per family (deterministic from seed)
│   └── config.go              Generate Ze config from scenario
├── peer/
│   ├── simulator.go           Per-peer goroutine: BGP session + route lifecycle
│   ├── session.go             TCP + BGP handshake (OPEN/KEEPALIVE)
│   ├── sender.go              Route announcement/withdrawal scheduling
│   └── receiver.go            Incoming message parsing + validation dispatch
├── chaos/
│   ├── scheduler.go           Chaos event scheduling from seed + rate config
│   ├── events.go              Event types: disconnect, hold-expiry, malformed, collision
│   └── executor.go            Execute chaos event on a specific peer
├── validation/
│   ├── model.go               Expected state: what each peer should have received
│   ├── tracker.go             Actual state: what each peer has received
│   ├── checker.go             Compare expected vs actual, report discrepancies
│   └── convergence.go         Track announcement → receipt latency
├── report/
│   ├── dashboard.go           Live terminal output (per-peer status table)
│   ├── jsonlog.go             Structured JSON event log
│   ├── summary.go             Exit summary statistics
│   └── metrics.go             Prometheus metrics endpoint (optional)
└── orchestrator.go            Coordinates all components, handles shutdown
```

### Peer Profile Generation (from seed)

The seed determines everything. Given the same seed + CLI flags, the scenario is identical.

**Per-peer decisions (deterministic from seed):**

| Decision | How |
|----------|-----|
| iBGP vs eBGP | `ibgp-ratio` fraction, peer index determines which |
| ASN | iBGP: all share Ze's local-as; eBGP: unique ASN per peer (65001+index) |
| Router ID | Derived: 10.peer_index.0.1 |
| Families | Random subset of available families (at least 1, weighted toward unicast) |
| Route count | Base `--routes` count, `--heavy-peers` peers get `--heavy-routes` |
| Capabilities | ASN4 always; ADD-PATH, extended-message, GR randomly per peer |
| Passive/Active | ~50% passive (Ze connects out), ~50% active (tool connects in) |

**Route generation:**

Each peer owns a unique prefix block. For peer index `i` with seed `s`:
- IPv4: `10.{i}.{hash(s,i,seq)}.0/24` — up to 256 routes per /16 block per peer
- IPv6: `2001:db8:{i}:{hash(s,i,seq)}::/48` — unique per peer
- VPN: RD `0:{i}:1` + IPv4 from peer's block
- EVPN: MAC `02:{i}:xx:xx:xx:xx` + IP from peer's block
- FlowSpec: match source from peer's block

### Chaos Events

| Event Type | What Happens | Frequency Weight |
|------------|-------------|------------------|
| TCP disconnect | Close TCP socket abruptly (no NOTIFICATION) | 25% |
| NOTIFICATION | Send CEASE/Admin Reset, close cleanly | 15% |
| Hold-timer expiry | Stop sending KEEPALIVEs, wait for Ze to notice | 15% |
| Partial withdrawal | Withdraw 10-50% of announced routes (stay connected) | 15% |
| Full withdrawal | Withdraw all routes (stay connected) | 5% |
| Reconnect storm | Disconnect + reconnect rapidly 3 times | 5% |
| Connection collision | Open second TCP connection while first is active | 5% |
| Malformed UPDATE | Send UPDATE with invalid attribute (RFC 7606 testing) | 5% |
| Disconnect during burst | Disconnect mid-way through initial route announcement | 5% |
| Config reload signal | Send SIGHUP to Ze process (if PID known) | 5% |

Weights are relative — actual selection uses seed-based RNG weighted by these values.
`--chaos-rate` controls how often events fire; weights control which type.

### Validation Model

**Core invariant:** After convergence, every established peer P should have received exactly the routes from all other established peers Q where Q's route family is in P's negotiated families.

```
Expected[peer_P] = Union(
    for each other peer Q where Q.established and Q != P:
        Q.announced_routes.filter(family in P.families)
)
```

**Validation runs continuously:**
1. After each announce/withdraw, update Expected model
2. After each received UPDATE, update Actual model
3. Periodically compare Expected vs Actual
4. Report discrepancies with convergence deadline (allow time for RR forwarding)

**Convergence tracking:**
- Record timestamp of each announcement
- Record timestamp when forwarded route arrives at each expected peer
- Report: min/avg/max/p99 convergence latency
- Flag: routes that never arrive within deadline (default 5s)

**Disconnect handling:**
- When peer disconnects, Expected model removes all its routes from other peers
- When peer reconnects, Expected model adds routes from all other peers to it
- Allows convergence window after each state change

### Ze Config Generation

The tool generates a complete Ze config matching the scenario:

```
# Generated by ze-bgp-chaos (seed: 1234567890)
# Peers: 4 (1 iBGP, 3 eBGP)

plugin {
    internal bgp-rr {
        run "ze.bgp-rr";
        encoder json;
    }
}

bgp {
    router-id 10.255.0.1;
    local-as 65000;

    # Peer 0: eBGP, active (Ze connects out to tool's listener)
    peer 127.0.0.1 {
        router-id 10.255.0.1;
        local-address 127.0.0.1;
        local-as 65000;
        peer-as 65001;
        # passive false; (default: Ze will actively connect)
        family {
            ipv4/unicast;
            ipv6/unicast;
        }
        capability {
            graceful-restart disable;
        }
        process bgp-rr {
            send { update; }
        }
    }

    # Peer 1: iBGP, passive (tool connects to Ze)
    peer 127.0.0.2 {
        router-id 10.255.0.1;
        local-address 127.0.0.2;
        local-as 65000;
        peer-as 65000;
        passive true;
        family {
            ipv4/unicast;
            ipv4/flow;
        }
        capability {
            graceful-restart disable;
        }
        process bgp-rr {
            send { update; }
        }
    }

    # ... more peers
}
```

**Address assignment:** Each peer gets a unique loopback address (127.0.0.{2+index}) to allow Ze to distinguish them. Active peers have Ze connect to the tool's listening port on that address. Passive peers have the tool connect to Ze's listening port.

### Live Dashboard (stdout)

```
ze-bgp-chaos | seed: 1234567890 | uptime: 2m35s | chaos: 3 events

PEER       TYPE   STATE         SENT   RECV   MISS   EXTRA  CONVG(avg)  ERRORS
127.0.0.2  eBGP   established    102    298      0       0     12ms         0
127.0.0.3  iBGP   established    100     85      2       0     15ms         0
127.0.0.4  eBGP   reconnecting     0      0      -       -        -         0
127.0.0.5  eBGP   established   2001    296      0       0     18ms         0

CHAOS LOG (last 5):
  2m30s  [127.0.0.4] TCP disconnect (abrupt)
  2m10s  [127.0.0.3] Partial withdrawal (30 routes)
  1m45s  [127.0.0.2] Hold-timer recovery
  1m20s  [127.0.0.5] Reconnect storm (3x)
  0m50s  [127.0.0.3] NOTIFICATION cease/admin-reset

VALIDATION: 2 pending convergence checks | 0 failures so far
```

Updates in-place using terminal escape codes. Falls back to line-based output if not a TTY.

### Exit Summary

```
ze-bgp-chaos run complete
  Seed:       1234567890
  Duration:   5m00s
  Peers:      4 (1 iBGP, 3 eBGP)
  Families:   ipv4/unicast, ipv6/unicast, ipv4/flow

Route Statistics:
  Total announced:    12,450
  Total withdrawn:     3,200
  Total forwarded:    28,750 (expected: 28,750)

Convergence:
  Min:    2ms
  Avg:   14ms
  P95:   45ms
  P99:  120ms
  Max:  450ms

Chaos Events:
  TCP disconnects:     5
  NOTIFICATIONs:       3
  Hold-timer expiry:   2
  Partial withdrawals: 4
  Reconnect storms:    1
  Connection collisions: 1
  Total:              16

Validation:
  Route mismatches:    0
  Missing routes:      0
  Extra routes:        0
  Convergence timeouts: 0

Result: PASS
```

Exit code: 0 = pass, 1 = validation failure found, 2 = runtime error

### Concurrency Model

```
main goroutine
    ├── Orchestrator (coordinates lifecycle)
    │   ├── PeerSimulator goroutine × N (one per peer)
    │   │   ├── TCP connection manager
    │   │   ├── BGP message reader (goroutine)
    │   │   ├── BGP message writer (goroutine)
    │   │   └── Route scheduler (goroutine)
    │   ├── ChaosScheduler goroutine (picks events, dispatches to peers)
    │   ├── ValidationEngine goroutine (periodic expected vs actual comparison)
    │   └── Reporter (synchronous in event loop — dashboard, JSON log, metrics)
    └── Signal handler (SIGINT/SIGTERM → graceful shutdown)
```

All cross-goroutine communication via channels. No shared mutable state.

### Graceful Shutdown

On SIGINT/SIGTERM or `--duration` expiry:
1. Stop chaos scheduler
2. Stop route churn
3. Wait for final convergence (up to 5s)
4. Run final validation pass
5. Print summary
6. Close all TCP connections (with NOTIFICATION cease)
7. Exit

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestScenarioGenDeterministic` | `scenario/generator_test.go` | Same seed → identical profiles, routes, chaos schedule | |
| `TestScenarioGenPeerCount` | `scenario/generator_test.go` | --peers flag respected, iBGP/eBGP ratio correct | |
| `TestRouteGenIPv4` | `scenario/routes_test.go` | IPv4 /24 prefixes are unique per peer, within expected range | |
| `TestRouteGenIPv6` | `scenario/routes_test.go` | IPv6 /48 prefixes are unique per peer | |
| `TestRouteGenVPN` | `scenario/routes_test.go` | VPN routes have correct RD + prefix per peer | |
| `TestRouteGenEVPN` | `scenario/routes_test.go` | EVPN Type-2 routes have unique MACs per peer | |
| `TestRouteGenFlowSpec` | `scenario/routes_test.go` | FlowSpec rules are valid per family | |
| `TestRouteGenNoOverlap` | `scenario/routes_test.go` | Two peers never generate the same prefix | |
| `TestConfigGenValid` | `scenario/config_test.go` | Generated config passes `ze bgp validate` | |
| `TestConfigGenMatchesScenario` | `scenario/config_test.go` | Config has correct peer count, ASNs, families | |
| `TestChaosScheduleDeterministic` | `chaos/scheduler_test.go` | Same seed → same event sequence | |
| `TestChaosRateZero` | `chaos/scheduler_test.go` | Rate 0.0 produces no events | |
| `TestChaosRateOne` | `chaos/scheduler_test.go` | Rate 1.0 produces event every interval | |
| `TestValidationModelAnnounce` | `validation/model_test.go` | Announce from A → expected at B, C, D (matching families) | |
| `TestValidationModelWithdraw` | `validation/model_test.go` | Withdraw from A → removed from B, C, D expected | |
| `TestValidationModelDisconnect` | `validation/model_test.go` | Disconnect A → all A's routes removed from others' expected | |
| `TestValidationModelReconnect` | `validation/model_test.go` | Reconnect A → A gets all routes from established peers | |
| `TestValidationModelFamilyFilter` | `validation/model_test.go` | Peer not supporting family F doesn't expect F routes | |
| `TestConvergenceTracking` | `validation/convergence_test.go` | Latency recorded correctly, timeout detection works | |
| `TestFamilyFilterInclude` | `scenario/generator_test.go` | --families flag limits families | |
| `TestFamilyFilterExclude` | `scenario/generator_test.go` | --exclude-families removes families | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| peers | 1-50 | 50 | 0 | 51 |
| routes | 1-100000 | 100000 | 0 | N/A |
| chaos-rate | 0.0-1.0 | 1.0 | -0.1 | 1.1 |
| ibgp-ratio | 0.0-1.0 | 1.0 | -0.1 | 1.1 |
| duration | 0 (unlimited) or >0 | 0 | N/A | N/A |
| port | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `chaos-basic` | `test/chaos/basic.ci` | 4 peers, seed 42, 10s, chaos-rate 0 → all routes propagated correctly | |
| `chaos-disconnect` | `test/chaos/disconnect.ci` | 3 peers, force 1 disconnect, verify withdrawal + replay | |
| `chaos-multi-family` | `test/chaos/multi-family.ci` | 3 peers with different family sets, verify correct filtering | |

## Files to Create

- `cmd/ze-bgp-chaos/main.go` - CLI entry point and flag parsing
- `cmd/ze-bgp-chaos/orchestrator.go` - Lifecycle coordination
- `cmd/ze-bgp-chaos/scenario/generator.go` - Seed-based scenario generation
- `cmd/ze-bgp-chaos/scenario/profile.go` - PeerProfile type
- `cmd/ze-bgp-chaos/scenario/routes.go` - Per-family route generation
- `cmd/ze-bgp-chaos/scenario/config.go` - Ze config generation
- `cmd/ze-bgp-chaos/peer/simulator.go` - Per-peer goroutine
- `cmd/ze-bgp-chaos/peer/session.go` - BGP session (OPEN/KEEPALIVE)
- `cmd/ze-bgp-chaos/peer/sender.go` - Route scheduling and sending
- `cmd/ze-bgp-chaos/peer/receiver.go` - Incoming message handling
- `cmd/ze-bgp-chaos/chaos/scheduler.go` - Chaos event scheduling
- `cmd/ze-bgp-chaos/chaos/events.go` - Chaos event types
- `cmd/ze-bgp-chaos/chaos/executor.go` - Event execution
- `cmd/ze-bgp-chaos/validation/model.go` - Expected route state model
- `cmd/ze-bgp-chaos/validation/tracker.go` - Actual received route state
- `cmd/ze-bgp-chaos/validation/checker.go` - Expected vs actual comparison
- `cmd/ze-bgp-chaos/validation/convergence.go` - Latency tracking
- `cmd/ze-bgp-chaos/report/dashboard.go` - Live terminal output (ANSI TTY + line fallback)
- `cmd/ze-bgp-chaos/report/jsonlog.go` - NDJSON event logging
- `cmd/ze-bgp-chaos/report/summary.go` - Exit summary (iBGP/eBGP counts)
- `cmd/ze-bgp-chaos/report/metrics.go` - Prometheus endpoint (per-instance registry)
- `cmd/ze-bgp-chaos/report/reporter.go` - Reporter struct (synchronous event multiplexer)
- `cmd/ze-bgp-chaos/peer/event_string.go` - EventType.String() method
- Test files for all packages above

## Files to Modify

- `Makefile` - Add `ze-bgp-chaos` build target

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A - standalone tool |
| RPC count in architecture docs | No | N/A |
| CLI commands/flags | No | N/A - separate binary |
| CLI usage/help text | No | N/A |
| API commands doc | No | N/A |
| Plugin SDK docs | No | N/A |
| Editor autocomplete | No | N/A |
| Functional test for new RPC/API | No | N/A - tool has its own tests |
| Makefile build target | Yes | `Makefile` |

## Implementation Steps

This is a large tool. Implementation should be phased:

### Phase 1: Foundation (MVP)
1. CLI skeleton with flag parsing and seed generation
2. Scenario generator: PeerProfile with ipv4/unicast only
3. Ze config generator
4. Single-peer BGP session (connect, OPEN, KEEPALIVE)
5. Route sender: announce N ipv4/unicast routes
6. Basic validation: track sent routes
7. Dashboard: simple line-based output

### Phase 2: Multi-Peer + Validation
1. Multi-peer orchestrator (goroutine per peer)
2. Route receiver: parse incoming UPDATEs from RR
3. Validation model: expected vs actual route state
4. Convergence tracking
5. Withdrawal support
6. Exit summary

### Phase 3: Chaos
1. Chaos scheduler with seed-based event selection
2. TCP disconnect + reconnect
3. NOTIFICATION + reconnect
4. Hold-timer expiry
5. Partial/full withdrawal
6. Disconnect during initial burst

### Phase 4: Multi-Family
1. IPv6/unicast route generation
2. VPN route generation
3. EVPN route generation
4. FlowSpec route generation
5. Per-peer family assignment
6. Family-aware validation

### Phase 5: Reporting (Done)

**As-designed:** Combined advanced chaos actions + reporting polish.
**As-built:** Connection collision, reconnect storm, and malformed UPDATE were implemented in Phase 3. iBGP/eBGP was done in Phase 1. Phase 5 focused purely on reporting:

1. EventType.String() — kebab-case names for all 10 event types
2. Reporter struct — synchronous event multiplexer with Consumer interface
3. Live dashboard — raw ANSI escape codes (not bubbletea), TTY detection, line-based fallback
4. NDJSON event log — `--event-file` flag, json.Encoder, kebab-case keys
5. Prometheus metrics — `--metrics` flag, per-instance registry, 6 metrics (4 counters + 1 gauge + 1 withdrawal counter)
6. Enhanced summary — IBGPCount/EBGPCount conditional display
7. orchestratorConfig struct — consolidated 12 parameters into single config

**Not implemented from original Phase 5 design:** Config reload (SIGHUP) — deferred to future work.

Each phase ends with a Self-Critical Review.

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Single Go binary, multiple peers | | | |
| Seed-based deterministic generation | | | |
| Predictable UPDATEs (announce + withdraw) | | | |
| Route propagation validation | | | |
| Chaos events (disconnect, etc.) | | | |
| Long-lived + CI/CD modes | | | |
| Multi-family support | | | |
| Varied OPEN capabilities per peer | | | |
| iBGP + eBGP mix | | | |
| Configurable chaos rate | | | |
| Generate Ze config | | | |
| Live dashboard output | | | |
| JSON log + summary + Prometheus | | | |
| Print seed, accept as CLI arg | | | |
| Max runtime option | | | |
| Family include/exclude CLI | | | |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | | | |
| AC-2 | | | |
| AC-3 | | | |
| AC-4 | | | |
| AC-5 | | | |
| AC-6 | | | |
| AC-7 | | | |
| AC-8 | | | |
| AC-9 | | | |
| AC-10 | | | |
| AC-11 | | | |
| AC-12 | | | |
| AC-13 | | | |
| AC-14 | | | |
| AC-15 | | | |
| AC-16 | | | |
| AC-17 | | | |
| AC-18 | | | |
| AC-19 | | | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| All unit tests | | | |
| All functional tests | | | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All files listed in "Files to Create" | | |

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Checklist

### Goal Gates (MUST pass)
- [ ] Acceptance criteria AC-1..AC-19 all demonstrated
- [ ] Tests pass (`make test`)
- [ ] No regressions (`make functional`)
- [ ] Binary builds and runs

### Quality Gates (SHOULD pass)
- [ ] `make lint` passes
- [ ] Architecture docs updated with learnings
- [ ] Implementation Audit fully completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
- [ ] Boundary tests cover all numeric inputs
- [ ] Functional tests verify end-to-end behavior

### Documentation
- [ ] Required docs read
- [ ] RFC references added to code where applicable

### Completion
- [ ] All Partial/Skipped items have user approval
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-bgp-chaos.md`
- [ ] All files committed together
