# Spec: peer-local-port-overwritten-by-remote-port

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | config |
| Depends | - |
| Phase | 2/2 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.** Two leaves of `container connection`
(`internal/component/bgp/yang/ze-bgp-conf.yang`) name a TCP port.
`connection/local/port` says "Local bind port", and it sits beside `local/ip`
and `local/accept`, which are the bind address and the inbound switch.
`connection/remote/port` says "Remote connection port", and it sits beside
`remote/ip` and `remote/connect`. A reader takes the first for the port Ze binds
and answers on, and the second for the port Ze dials. This spec covers those two
leaves.

**What Ze does instead.** `parsePeerSettings`
(`internal/component/bgp/reactor/config.go`) writes both into one field,
`PeerSettings.Port` (`internal/component/bgp/reactor/peer_settings.go`), whose
comment calls it "the peer's BGP port". It reads the local leaf first and the
remote leaf second, so the remote value overwrites the local value whenever both
are set. That one field then decides two endpoints. It is the dial target:
session setup in `internal/component/bgp/reactor/session_connection.go` joins
`settings.Address` with `settings.Port`. It is also the listen port:
`peerListenPort` (`internal/component/bgp/reactor/reactor_peers.go`) gives the
peer a dedicated listener at that number whenever it is neither zero nor 179.
Ze binds no local source port to the value at all. So an operator who asks for a
dedicated listener on 1790 and a peer that listens on 179 gets 179 in both
places, and the commit reports nothing. The leaf's own `ze:help` states this
outcome, and the `description` one line above it still says "Local bind port",
so the node disagrees with itself.

**What closing it means.** The implementer builds the behavior, or refuses the
leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses the `vrf` leaf. Building is
obviously right here. Two endpoints need two numbers, the listen side already
has a function of its own in `peerListenPort`, and refusing the leaf takes away
the only way an operator names the port a dedicated listener answers on. The
design has to say what the local port means when it is set alone, because
`peerListenPort` reads the same field that the dial path reads, and today a
peer with a local port dials it. Whichever way it lands, the `description` and
the `ze:help` on both leaves are brought back into agreement in the same work
(`ai/rules/documentation.md`).

## Progress (2026-09-06, resumed and implemented)

The paused session had split the field and updated two tests. The resuming
session enumerated every reader of the merged field with `gopls references` on
`PeerSettings.Port`, found one more test that used `Port` with LISTEN semantics
(`TestMD5PeersForListener`, `reactor_test.go`), corrected it, added the
socket-level wiring test, and recorded the red-then-green walk below.

**The commit carries a second spec's hunks.** Three of the files this change
must touch (`peer_settings.go`, `config.go`, `reactor_peers.go`) also hold
finished work from `plan/immediate/spec-peer-leaves-the-peer-parser-never-reads.md`,
left by an agent this same session stopped mid-work. The port split does not
compile without those files, and the hunks were verified coherent -- the
`manual-eor` leaf reaches `sendInitialRoutes` and `AddDynamicPeer` has a test --
so they are carried rather than reverted. That spec's own Progress section
records exactly which of its hunks landed and which did not.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/config-reference.md` - the peer `connection` container's leaf table
  → Decision: the table gives `remote { port }` and `local { port }` one row each, so the two leaves are documented as two endpoints.
  → Constraint: a page a change makes wrong is repaired in the same work (`ai/rules/documentation.md`).
- [ ] `docs/guide/configuration.md` - the Peer Settings table
  → Decision: the single `port` row named no endpoint and is replaced by two rows.
- [ ] `docs/perf-stale-arp.md` - the perf harness dials INTO Ze at 1790 and 1791
  → Constraint: `test/perf/configs/ze.conf` sets `connection { local { port } }` on both peers, so the local leaf must keep naming the LISTEN port.

### RFC Summaries (Scope: protocol)
- [ ] N-A - Scope is config. The BGP port is RFC 4271 Section 4.1's well-known 179, which `DefaultBGPPort` already carries; no wire behavior changes.

**Key insights:** (minimal context to resume after compaction)
- The peer map is keyed on (remote address, remote port), so the dial port and the key are one value and the LISTEN port must be a second field.
- Ze binds no source port on an outbound connection, and must not: a port already in LISTEN cannot be bound again.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/bgp/reactor/config.go` - `parsePeerSettings` read `connection > local > port` and then `connection > remote > port`, writing both into `PeerSettings.Port`, remote last.
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `PeerSettings.Port` was "the peer's BGP port"; `PeerKey` keys the peer map on (Address, Port); `NewPeerSettings` defaults it to `DefaultBGPPort`.
- [ ] `internal/component/bgp/reactor/reactor_peers.go` - `peerListenPort` gave a peer a listener of its own whenever `Port` was neither 0 nor 179.
- [ ] `internal/component/bgp/reactor/session_connection.go` - `Session.Connect` joins `Address` with `Port` to dial.
- [ ] `internal/component/bgp/reactor/operation.go` - `operationPeerPortExplicit` treated EITHER leaf as "the caller named the dial port".
- [ ] `internal/component/bgp/reactor/reactor.go` - `startMultiListeners`, `md5PeersForListener`, `listenTTLForListener` all select peers by `peerListenPort`.
- [ ] `internal/component/bgp/reactor/reactor_connection.go` - `listenerClaimants` and `listenerPeerKey` decide claim by `peerListenPort` and return `PeerKey`.
- [ ] `internal/component/bgp/config/peers.go` - `applyPortOverride` writes the test harness port into `Port`, which is the DIAL target.
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `connection/local/port` said "Local bind port" and its `ze:help` admitted the remote leaf replaced it.

**Behavior to preserve:** (unless the user explicitly said to change it)
- The dial target stays `connection > remote > port`, defaulting to 179.
- The peer map key stays (remote address, remote port); `parsePeerAddrToKey` parses the same pair from `ip:port`.
- A peer that names no local port keeps sharing the daemon's listener.
- `ze.test.bgp.port` keeps overriding the DIAL port only.

**Behavior to change:** (only what the user asked for)
- `connection > local > port` reaches its own field, `PeerSettings.LocalPort`, and decides the listen port alone.
- `connection > remote > port` no longer moves a listener, and `connection > local > port` no longer moves the dial target or the peer key.
- The parser's out-of-range message names which endpoint carried the bad value.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator configuration arrives as a YANG-modeled config tree, at load and at commit.
- `PeersFromTree` hands one peer map per peer to the reactor's peer parser.

### Transformation Path
1. `internal/component/bgp/yang/ze-bgp-conf.yang` declares `connection/local/port` and `connection/remote/port`, both `zt:port`.
2. `parsePeerSettings` (`internal/component/bgp/reactor/config.go`) writes the local leaf to `PeerSettings.LocalPort` and the remote leaf to `PeerSettings.Port`.
3. `peerListenPort` (`internal/component/bgp/reactor/reactor_peers.go`) reads `LocalPort`, falling back to the daemon's `config.Port` and then to 179.
4. `startMultiListeners` (`internal/component/bgp/reactor/reactor.go`) binds one socket per (local address, listen port) pair.
5. `Session.Connect` (`internal/component/bgp/reactor/session_connection.go`) joins `Address` with `Port` to dial.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ reactor | `parsePeerFromTree` reads the `connection` container | Yes -- `TestParsePeerFromTreeSplitsLocalAndRemotePorts` |
| Reactor ↔ kernel | `startMultiListeners` binds the listen port | Yes -- `TestPeerLocalPortOpensARealListener` reads `ListenAddrs()` |
| Engine ↔ Plugin | none: no plugin reads either port | N-A |

### Integration Points
- `md5PeersForListener` and `listenTTLForListener` (`internal/component/bgp/reactor/reactor.go`) select by `peerListenPort`, so both now key off the local leaf.
- `listenerClaimants` and `listenerPeerKey` (`internal/component/bgp/reactor/reactor_connection.go`) claim by listen port and return the peer key, which stays the remote pair.
- `buildDynamicPeerSettings` (`internal/component/bgp/reactor/reactor_dynamic.go`) resets `Port` to 179 for the map key and inherits `LocalPort` untouched.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The leaf reaches the socket through the parser and `peerListenPort` alone; no second reader of the YANG key exists |
| No unintended coupling (components stay isolated) | Yes | Both fields live in `PeerSettings`; no component outside `internal/component/bgp/reactor` reads either |
| No duplicated functionality (extends existing, does not recreate) | Yes | `peerListenPort` is the one listen-port decision and it stays the one; only the field it reads changed |
| Zero-copy preserved where applicable (refs, not copies) | N-A | Two `uint16` fields, no buffers |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | No registry, switch, or factory is touched; `PeerSettings` is the peer's own settings struct |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every reader of `PeerSettings.Port` that meant LISTEN is inside `internal/component/bgp/reactor` | `gopls references` on the field lists 61 sites, all in that package plus `internal/component/bgp/config/peers.go` and `test/integration/integration_test.go` | A listener elsewhere keeps reading the dial port | `gopls references internal/component/bgp/reactor/peer_settings.go:242:2`, then reading each non-test site | confirmed |
| A-2 | The peer map key belongs to the REMOTE pair, so `PeerKey` keeps reading `Port` | `parsePeerAddrToKey` parses `ip:port` as the remote pair; `listenerPeerKey` returns the key for a claimant | Two peers on one IP distinguished only by local port would collide | Reading `parsePeerAddrToKey`, `PeerKey`, `listenerPeerKey`; `TestListenerPeerKeyRoutesDirectlyForASoleClaimant` asserts the key carries the remote port | confirmed |
| A-3 | Ze must NOT bind the local port as an outbound source port | A port in LISTEN cannot be bound again by a connecting socket; Ze binds no source port today | An operator setting both leaves would get a dial that fails with EADDRINUSE | Reading `Session.Connect` (`session_connection.go`), which passes no local address to the dialer | confirmed |
| A-4 | `test/perf/configs/ze.conf` reads the local leaf as the LISTEN port | `docs/perf-stale-arp.md` records peers dialing into Ze at 1790 and 1791, which are the two `local { port }` values | The perf harness would break | Reading the config and the doc together | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A test used `Port` to mean "listen port" and keeps passing for the wrong reason | A listener test asserting a count | Every `Port` reference was read; `TestMD5PeersForListener` was the one found, and it went RED under the split before it was corrected |
| R-2 | The peer key changes for a peer that named only a local port, so a reload sees a different peer | `reactor_api.go` diffs peers by `PeerKey` | The key is now stable under a local-port change and moves only with the remote port, which is the endpoint the key names |
| R-3 | Another session's work sits in the same files | `git diff HEAD -- <path>` shows hunks this spec did not write | Every file named below was diffed before it was committed; the two files carrying another spec's hunks are reported, not committed |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A peer listens on the wrong port and accepts nothing, or dials the wrong port and never establishes. Both are session-level, not route-level: no route is mis-encoded |
| How is it reverted? | Single commit revert. No config migration: both leaves keep their names, their types and their defaults |
| Who else touches this path? | `plan/immediate/spec-peer-leaves-the-peer-parser-never-reads.md` edits the same parser and the same settings struct |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `connection { local { port } }` in the peer config tree | → | `parsePeerSettings` writes `PeerSettings.LocalPort` | `TestParsePeerFromTreeSplitsLocalAndRemotePorts` |
| `connection { local { port } }` in the peer config tree | → | `Reactor.peerListenPort` returns it, and `startMultiListeners` binds a socket there | `TestPeerLocalPortOpensARealListener` |
| `connection { remote { port } }` in the peer config tree | → | `parsePeerSettings` writes `PeerSettings.Port`, which `Session.Connect` dials | `TestParsePeerFromTreeSplitsLocalAndRemotePorts` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A peer names `local { port 1790 }` and `remote { port 2790 }` | Ze listens on 1790 and dials 2790 |
| AC-2 | A peer names `local { port 1790 }` alone | Ze listens on 1790 and dials 179 |
| AC-3 | A peer names `remote { port 2790 }` alone | Ze dials 2790 and opens no listener of its own |
| AC-4 | A peer names neither leaf | Ze shares the daemon's listener and dials 179 |
| AC-5 | Either leaf carries 0 or 65536 | The commit is refused, and the message names which endpoint carried the value |
| AC-6 | A peer names `local { port }` and carries an MD5 password or GTSM | The key and the TTL are applied to the listener on the LOCAL port, not to the daemon's |
| AC-7 | Both leaves are set and the reactor starts | `ListenAddrs()` holds the local port and does not hold the remote port |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Gives one peer a listener of its own on 1790 while it dials the peer at 179 | config tree -> `parsePeerSettings` -> `LocalPort` -> `peerListenPort` -> `startMultiListeners` -> bound socket | `TestPeerLocalPortOpensARealListener` |
| 2 | Points a peer at a daemon listening on a non-standard port | config tree -> `parsePeerSettings` -> `Port` -> `Session.Connect` | `TestParsePeerFromTreeSplitsLocalAndRemotePorts` |
| 3 | Runs the perf harness, whose two peers each own a listener at 1790 and 1791 | `test/perf/configs/ze.conf` -> parser -> `peerListenPort` -> two sockets | `TestPeerLocalPortOpensARealListener` covers the same chain with ephemeral ports |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParsePeerFromTreeSplitsLocalAndRemotePorts` | `internal/component/bgp/reactor/config_test.go` | AC-1..AC-4: each leaf reaches its own field | pass |
| `TestParsePeerFromTreeRefusesPortOutsideRange` | `internal/component/bgp/reactor/config_test.go` | AC-5: bounds on both endpoints, and the message names the endpoint | pass |
| `TestPeerListenPortReadsTheOperatorsLocalPort` | `internal/component/bgp/reactor/config_test.go` | AC-1: parser to `peerListenPort` | pass |
| `TestPeerListenPort` | `internal/component/bgp/reactor/reactor_peers_test.go` | AC-2..AC-4: fallback order, and that a remote port moves no listener | pass |
| `TestListenerPeerKeyRoutesDirectlyForASoleClaimant` | `internal/component/bgp/reactor/reactor_listener_attribution_test.go` | The claim is decided by the listen port and the key carries the remote port | pass |
| `TestMD5PeersForListener` | `internal/component/bgp/reactor/reactor_test.go` | AC-6: MD5 keys follow the local port | pass |
| `TestListenTTLForListener` | `internal/component/bgp/reactor/reactor_test.go` | AC-6: GTSM TTL follows the local port | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `connection/local/port` | 1-65535 | 65535 | 0 | 65536 |
| `connection/remote/port` | 1-65535 | 65535 | 0 | 65536 |

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `TestPeerLocalPortOpensARealListener` | `internal/component/bgp/reactor/config_test.go` | AC-7: an operator's `local { port }` becomes a bound TCP socket, and the remote port becomes none | pass |

The end-user surface here is the configuration file, not a command: no CLI verb,
RPC, or JSON payload names either port. The proof therefore runs from the config
tree to a real kernel socket rather than through a `.ci` script, which would add
a daemon start and assert less.

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | - | - | Nothing wire-visible changes: no message, attribute, or capability is touched. The change moves which TCP port a socket binds, and the red-then-green walk below proves it at the socket | N-A |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/component/bgp/reactor/peer_settings.go` - `PeerSettings.LocalPort` declared; `Port` documented as the remote endpoint
- `internal/component/bgp/reactor/config.go` - `parsePeerSettings` writes the local leaf to `LocalPort`, and names the endpoint in the range error
- `internal/component/bgp/reactor/reactor_peers.go` - `peerListenPort` reads `LocalPort`
- `internal/component/bgp/reactor/operation.go` - `operationPeerRemotePortExplicit` reads the remote leaf alone
- `internal/component/bgp/reactor/reactor_dynamic.go` - comment: the template's `LocalPort` is what the group listens on
- `internal/component/bgp/yang/ze-bgp-conf.yang` - both leaves' `description` and `ze:help` state the endpoint they name
- `internal/component/bgp/reactor/config_test.go` - three new tests plus the socket-level wiring test
- `internal/component/bgp/reactor/reactor_peers_test.go` - `TestPeerListenPort` covers both ports
- `internal/component/bgp/reactor/reactor_listener_attribution_test.go` - `attrPeer` takes both ports
- `internal/component/bgp/reactor/reactor_test.go` - `TestMD5PeersForListener` and `TestListenTTLForListener` use `LocalPort`
- `docs/config-reference.md` - one row per port leaf
- `docs/guide/configuration.md` - the Peer Settings table replaces its single `port` row with two

## Files to Create
- None. The behavior extends `PeerSettings` and the parser that fills it.

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/bgp/yang/ze-bgp-conf.yang` -- both leaves already existed; their `description` and `ze:help` now name the endpoint each one is |
| YANG validation constraints | Yes | Both leaves are `zt:port`, which carries the 1-65535 range; the parser repeats it for a tree that reached it without the schema |
| YANG custom validators | No | The native `zt:port` range is sufficient |
| CLI commands/flags | No | No command names either port |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | Yes | Automatic: both leaves are typed `zt:port` |
| Functional test for new RPC/API | N-A | No RPC or API added |
| Pipe completeness | N-A | No command output added |
| Env var registration | No | `ze.test.bgp.port` already exists and is unchanged; it overrides the DIAL port |
| Doctor check for runtime dependencies | No | The listen port is bound by `startMultiListeners`, whose failure is already reported at start; no new file, socket, or service is introduced |
| Prometheus counters/metrics | No | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability, or attribute is touched |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The leaves existed and were documented; only one of them did nothing |
| 2 | Config syntax changed? | Yes | `docs/config-reference.md` and `docs/guide/configuration.md` -- one row per port leaf |
| 3 | CLI command added/changed? | No | No command names either port |
| 4 | API/RPC added/changed? | No | `AddDynamicPeer` takes the same tree shape |
| 5 | Plugin added/changed? | No | No plugin reads either port |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md`, Peer Settings |
| 7 | Wire format changed? | No | No message, attribute, or capability is touched |
| 8 | Plugin SDK/protocol changed? | No | `pkg/plugin` and `pkg/ze` are untouched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | The port stays RFC 4271's well-known 179 by default; no requirement's proof changes |
| 10 | Test infrastructure changed? | No | `ze.test.bgp.port` keeps overriding the dial port |
| 11 | Affects daemon comparison? | No | Every daemon compared already offers both endpoints |
| 12 | Internal architecture changed? | No | `docs/architecture/core-design.md` describes peer add/remove/lookup, which is unchanged: the peer key still names the remote pair |
| 13 | Route metadata keys added/changed? | No | No metadata key touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes, and unaffected | DERIVED: `./le spec citation anchors spec plan/immediate/spec-peer-local-port-overwritten-by-remote-port.md`. One document is DECLARED by this spec's code: `docs/architecture/config/apply-ordering.md`, declared by `operation.go`. It describes the ORDER a config operation is applied in and names no port; this change alters which leaf `operationPeerRemotePortExplicit` reads, inside one operation, so the ordering the page describes is unchanged. `docs/architecture/core-design.md`, declared by `reactor_peers.go`, describes peer add/remove/lookup, and the peer key still names the remote pair. The other 25 documents only mention the files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `test/perf/configs/ze.conf` and `docs/perf-stale-arp.md` show `local { port }` used as the LISTEN port, which is the reading this change makes true |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the leaf reaches a socket
   - Tests: `TestParsePeerFromTreeSplitsLocalAndRemotePorts`, `TestPeerListenPortReadsTheOperatorsLocalPort`, `TestPeerLocalPortOpensARealListener`
   - Files: `peer_settings.go` (declare `LocalPort`), `config.go` (write it), `reactor_peers.go` (read it)
   - Verify: with the merged field the three tests fail; with the split they pass
2. **Phase: every other reader of the merged field** -- find what `Port` meant elsewhere
   - Tests: `TestPeerListenPort`, `TestListenerPeerKeyRoutesDirectlyForASoleClaimant`, `TestMD5PeersForListener`, `TestListenTTLForListener`
   - Files: `operation.go`, `reactor_dynamic.go`, and the four test files above
   - Verify: `gopls references` on `PeerSettings.Port` lists every site, and each one is read and classified as dial, key, or listen
3. **Phase: schema and pages** -- the node agrees with itself
   - Files: `ze-bgp-conf.yang`, `docs/config-reference.md`, `docs/guide/configuration.md`
   - Verify: no page still says one leaf replaces the other

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | AC-1..AC-7 each have a named test in the TDD plan |
| Feature completeness | The chain from the leaf to a bound socket has a test that reads `ListenAddrs()` |
| Correctness | Every `PeerSettings.Port` site is classified dial, key, or listen, and only the listen ones moved |
| Naming | `LocalPort` matches `connection > local > port`; the range error names the endpoint |
| Data flow | `peerListenPort` stays the ONE listen-port decision; no second reader of `LocalPort` exists |
| Rule: `ai/rules/principles.md` | The split removes a silently-wrong value: the operator's listen port used to vanish with no log line |
| Rule: `ai/rules/documentation.md` | The YANG `ze:help` no longer says the remote leaf replaces the local one |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| `PeerSettings.LocalPort` exists and is written by the parser | `grep -n 'LocalPort' internal/component/bgp/reactor/config.go internal/component/bgp/reactor/peer_settings.go` |
| `peerListenPort` reads it | `grep -n 'LocalPort' internal/component/bgp/reactor/reactor_peers.go` |
| No reader of the merged field is left meaning "listen" | `gopls references internal/component/bgp/reactor/peer_settings.go:242:2` |
| The tests discriminate | The red-then-green walk recorded under Design Insights |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | Both ports are bounded 1-65535 by `zt:port` and again by the parser, so no listener binds port 0 and takes a port of the kernel's choosing |
| Fail closed | An out-of-range value returns an error rather than a zero port (`ai/rules/principles.md`) |
| Resource exhaustion | A local port gives ONE peer ONE listener; `startMultiListeners` deduplicates by (address, port), so N peers on one port bind one socket |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- **Splitting a field changes what its READERS mean, not only what its writers write.** `gopls references` on `PeerSettings.Port` listed 61 sites. Two production sites meant "listen" (`peerListenPort`, `operationPeerPortExplicit`), one meant "map key" (`PeerKey`), one meant "dial" (`Session.Connect`), and four test sites set `Port` while meaning "listen". A field with ONE writer had four different meanings among its readers.
- **The red-then-green walk, recorded.** With `ps.LocalPort = uint16(v)` reverted to `ps.Port` and `peerListenPort` reverted to read `s.Port`, seven tests went RED: `TestParsePeerFromTreeSplitsLocalAndRemotePorts` (2 cases), `TestPeerListenPortReadsTheOperatorsLocalPort`, `TestPeerLocalPortOpensARealListener`, `TestListenerPeerKeyRoutesDirectlyForASoleClaimant` (2 cases), `TestPeerListenPort` (2 cases), `TestMD5PeersForListener`, `TestListenTTLForListener`. Restoring both lines returned all seven to green.
- **`TestMD5PeersForListener` is why the enumeration was owed.** It set `s4.Port = 1179` to mean "this peer is on another listener", and the split silently moved that peer onto the shared listener. The test caught it, which is what a test that names its PREVENTS clause is for.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| `LocalPort` decides the LISTENER only, and no outbound source port is bound to it | Bind the outbound socket's source port to `LocalPort` | A port already in LISTEN cannot be bound by a connecting socket, so a peer that both listens and dials would fail to connect. The YANG `ze:help` states the limit |
| `PeerKey` keeps reading `Port` | Key on (address, local port, remote port) | The key names the REMOTE endpoint everywhere else: `parsePeerAddrToKey` parses `ip:port` as the remote pair, and `listenerPeerKey` returns the key for a peer a listener claimed. A third component would make the key disagree with its own parser |
| `operationPeerRemotePortExplicit` reads the remote leaf alone | Keep treating either leaf as "the dial port was named" | The daemon's port supplies the DIAL target when the operation names none. A config that names only a listen port has said nothing about the dial target, so it must still get the daemon's port |
| The proof is a socket-level Go test, not a `.ci` | Add a `.ci` functional test | No CLI verb, RPC, or JSON payload names either port, so a `.ci` would start a daemon and assert less than `ListenAddrs()` does |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- Ze binds no source port on an outbound connection, so `connection > local > port` moves the listener alone. This is a deliberate boundary, not missing work: the kernel refuses a bind to a port in LISTEN.
- `connection > local > ip` still selects the bind address for the listener alone, on the same reasoning. Unchanged by this spec.

## RFC Documentation (Scope: protocol)

N-A. Scope is config. No MUST, state transition, timer, or message ordering is
enforced by this change. The default port stays RFC 4271's well-known 179,
declared once as `DefaultBGPPort`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Implementation Summary

### What Was Implemented
- `PeerSettings.LocalPort` (`internal/component/bgp/reactor/peer_settings.go`), written by `parsePeerSettings` (`internal/component/bgp/reactor/config.go`) from `connection > local > port`, and read by `peerListenPort` (`internal/component/bgp/reactor/reactor_peers.go`) as the sole listen-port decision. `Port` keeps the remote endpoint: the dial target `Session.Connect` joins with `Address`, and the port half of the peer map key.
- `operationPeerRemotePortExplicit` (`internal/component/bgp/reactor/operation.go`) reads the remote leaf alone, so a config naming only a listen port still takes the daemon's port as its dial target.
- `listenerPeerKey` (`internal/component/bgp/reactor/reactor_connection.go`) tests the SOCKET's port rather than the claimant's `Port` field, which no longer means listen.
- Both leaves' `description` and `ze:help` in `internal/component/bgp/yang/ze-bgp-conf.yang` name the endpoint they decide; the node no longer contradicts itself.
- `test/plugin/peer-local-port-listener.ci` proves the chain from a configuration FILE to an established session on the operator's port. Added at closure (see Review Gate, finding 1).

### Bugs Found/Fixed
- The defect this spec exists to fix: both leaves wrote one field, remote last, so an operator's listen port was replaced with no error and no log line. Covered by `TestParsePeerFromTreeSplitsLocalAndRemotePorts` and `TestPeerLocalPortOpensARealListener`.
- No new defect was introduced. Two found on the way are recorded below and in `plan/journal/guard-added-to-one-half-of-a-pair.md`.

### Documentation Updates
- `docs/config-reference.md`, peer `connection` table: the merged `remote { port }` / `local { port }` row became one row per leaf, each naming its endpoint.
- `docs/guide/configuration.md`, Peer Settings table: the single `port` row became `remote { port }` and `local { port }`, the second stating that Ze binds no source port on an outbound connection.
- Both edits ride on the implementation commit ce5ed4459, not on closure.
- `./le doc check verify` exits 1 over this checkout with 8611 lines of findings. Grepping that log for every file this spec names (`peer_settings.go`, `reactor_peers.go`, `config.go`, `operation.go`, `ze-bgp-conf.yang`, `docs/config-reference.md`, `docs/guide/configuration.md`) returns nothing: the red is other sessions' work across the BGP command surface and `../gh-pages/`.

### Deviations from Plan
- The Key Design Decisions row "the proof is a socket-level Go test, not a `.ci`" was OVERTURNED at review. Its stated reason, that a `.ci` "would start a daemon and assert less than `ListenAddrs()` does", is wrong: a `.ci` reaches the configuration FILE, which `parsePeerFromTree` over a hand-built map does not, and it carries a session to Established on the operator's port, which `ListenAddrs()` does not. `ai/rules/testing.md` is explicit that a change is not done on unit tests alone. The `.ci` was written, walked red-then-green, and promoted.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | The spec decided a `.ci` would assert less than the socket-level Go test, and recorded that as a design decision | A `.ci` asserts strictly more: it starts at the configuration file and ends at an established session, where the Go test starts at a hand-built `map[string]any` and ends at a bound socket | `/ze-review` step 3, functional test coverage, read against `ai/rules/testing.md` | `test/plugin/peer-local-port-listener.ci` written and promoted |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `connection > local > port` decides the listen port and nothing else | Done | `parsePeerSettings` (`internal/component/bgp/reactor/config.go`), `peerListenPort` (`internal/component/bgp/reactor/reactor_peers.go`) | `LocalPort` has one writer and one reader |
| `connection > remote > port` decides the dial target and the peer key | Done | `Session.Connect` (`internal/component/bgp/reactor/session_connection.go`), `PeerKey` (`internal/component/bgp/reactor/peer_settings.go`) | Unchanged from before the split |
| The two leaves' `description` and `ze:help` agree with the code | Done | `internal/component/bgp/yang/ze-bgp-conf.yang`, the `port` leaf of each of `connection/local` and `connection/remote` | Each says which endpoint it names and that the two are independent |
| The range error names which endpoint carried the bad value | Done | `parsePeerSettings` (`internal/component/bgp/reactor/config.go`) | `local port must be 1-65535` and `remote port must be 1-65535` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestParsePeerFromTreeSplitsLocalAndRemotePorts`, `test/plugin/peer-local-port-listener.ci` | Listen 1790, dial 2790 |
| AC-2 | Done | `TestPeerListenPortReadsTheOperatorsLocalPort`, `TestPeerListenPort` | Local alone: listen on it, dial 179 |
| AC-3 | Done | `TestPeerListenPort` | Remote alone moves no listener |
| AC-4 | Done | `TestPeerListenPort` | Neither leaf: the daemon's listener, dial 179 |
| AC-5 | Done | `TestParsePeerFromTreeRefusesPortOutsideRange` | 0 and 65536 on both endpoints, message names the endpoint |
| AC-6 | Done | `TestMD5PeersForListener`, `TestListenTTLForListener` | MD5 keys and GTSM TTL follow `LocalPort` |
| AC-7 | Done | `TestPeerLocalPortOpensARealListener` | `ListenAddrs()` holds the local port and not the remote one |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestParsePeerFromTreeSplitsLocalAndRemotePorts` | Done | `internal/component/bgp/reactor/config_test.go` | pass |
| `TestParsePeerFromTreeRefusesPortOutsideRange` | Done | `internal/component/bgp/reactor/config_test.go` | pass |
| `TestPeerListenPortReadsTheOperatorsLocalPort` | Done | `internal/component/bgp/reactor/config_test.go` | pass |
| `TestPeerLocalPortOpensARealListener` | Done | `internal/component/bgp/reactor/config_test.go` | pass |
| `TestPeerListenPort` | Done | `internal/component/bgp/reactor/reactor_peers_test.go` | pass |
| `TestListenerPeerKeyRoutesDirectlyForASoleClaimant` | Done | `internal/component/bgp/reactor/reactor_listener_attribution_test.go` | pass |
| `TestMD5PeersForListener` | Done | `internal/component/bgp/reactor/reactor_test.go` | pass |
| `TestListenTTLForListener` | Done | `internal/component/bgp/reactor/reactor_test.go` | pass |
| `peer-local-port-listener` | Changed | `test/plugin/peer-local-port-listener.ci` | Added at closure; the plan recorded "no `.ci`" and that decision was overturned |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/bgp/reactor/peer_settings.go` | Done | `LocalPort` declared, `Port` documented as the remote endpoint |
| `internal/component/bgp/reactor/config.go` | Done | Writes `LocalPort`, names the endpoint in the range error |
| `internal/component/bgp/reactor/reactor_peers.go` | Done | `peerListenPort` reads `LocalPort` |
| `internal/component/bgp/reactor/operation.go` | Done | `operationPeerRemotePortExplicit` |
| `internal/component/bgp/reactor/reactor_dynamic.go` | Done | Comment: the template's `LocalPort` is the group's listen port |
| `internal/component/bgp/yang/ze-bgp-conf.yang` | Done | Both leaves state their endpoint |
| `internal/component/bgp/reactor/reactor_connection.go` | Changed | Not in the plan. `listenerPeerKey` had to test the socket's port instead of the claimant's `Port` field |
| `internal/component/bgp/reactor/config_test.go` | Done | Four tests, the socket-level wiring test among them |
| `internal/component/bgp/reactor/reactor_peers_test.go` | Done | `TestPeerListenPort` covers both ports |
| `internal/component/bgp/reactor/reactor_listener_attribution_test.go` | Done | `attrPeer` takes both ports |
| `internal/component/bgp/reactor/reactor_test.go` | Done | Two setup fields moved to `LocalPort` |
| `docs/config-reference.md` | Done | One row per port leaf |
| `docs/guide/configuration.md` | Done | Two rows replace the single `port` row |
| `test/plugin/peer-local-port-listener.ci` | Changed | Added at closure, not in the plan |
| `internal/test/fixture/plugin_shell_extra_3.go` | Changed | One `Register` line for that test's trigger |

### Audit Summary
- **Total items:** 40
- **Done:** 36
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (recorded in Deviations and in the tables above)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| An operator who names `connection > local > port` gets a listener on that port | functional | `test/plugin/peer-local-port-listener.ci` passes in the live plugin suite (`ze-test bgp plugin --pattern peer-local-port-listener`: `pass 1/1 100.0% 6.9s`). Reverting `ps.LocalPort = uint16(v)` and `peerListenPort`'s read of `LocalPort`, rebuilding the daemon, and re-running gives `TIME 6 peer-local-port-listener` at 30s: nothing binds the port, so the dial never connects |
| The remote leaf keeps deciding the dial target and the peer map key | data correctness | `TestParsePeerFromTreeSplitsLocalAndRemotePorts` asserts each leaf reaches its own field; `TestListenerPeerKeyRoutesDirectlyForASoleClaimant` asserts the key a listener returns carries the REMOTE port |
| Neither leaf silently overwrites the other, which is the defect | data correctness | The red-then-green walk in Design Insights: reverting the two lines reddens seven tests over eleven cases, and restoring them returns all seven to green |
| The YANG node stops contradicting itself | documentation | `internal/component/bgp/yang/ze-bgp-conf.yang`: `local/port` reads "Local listen port" with a `ze:help` naming the listener, `remote/port` reads "Remote connection port" with a `ze:help` naming the dial. Neither says the other replaces it |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | Every AC has product code and a passing test. Two defects met on the way are journal rows rather than work this spec owed | N-A |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/peer-local-port-overwritten-by-remote-port-d64e7b3f-bfdc-4614-8db4-f12043eb77cc.md`, 19 files, verdict=clean |
| `review check` | `review_gate: OK (5 code files, clean, hashes match)` |
| Rounds | 2 |
| Reviewer lenses used | wiring and functional-test coverage; logic, guard audit and removed-behavior; style, simplicity and security |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | No `.ci` or `.et` covered either port leaf, so the change stood on unit tests alone (`ai/rules/testing.md`). The Go test starts at a hand-built `map[string]any`, so no test carried the configuration FILE to the feature | `test/plugin/` held no test naming `connection { local { port } }` | `test/plugin/peer-local-port-listener.ci`, plus one `Register` line for its trigger in `internal/test/fixture/plugin_shell_extra_3.go`. Walked red (TIME, 30s) with the fix reverted and green (6.9s) with it restored |

### Findings recorded, not fixed
| # | Severity | Finding | Location | Disposition |
|---|----------|---------|----------|-------------|
| 2 | NOTE | `peerListenPort` treats an explicit `local { port 179 }` as "the operator named nothing", because the guard still carries `&& s.LocalPort != DefaultBGPPort` from the days the merged field defaulted to 179. `NewPeerSettings` leaves `LocalPort` at zero, so zero alone already means "not named" | `internal/component/bgp/reactor/reactor_peers.go`, `peerListenPort` | No operator can observe it: the only production writer of `reactor.Config.Port` is the `ze.test.bgp.port` override, read by `CreateReactorFromTree` (`internal/component/bgp/config/loader_create.go`), so the fallback returns 179 anyway and the same socket binds. It becomes reachable the day a config leaf sets the daemon's listen port |
| 3 | NOTE | `ze-peer` builds its OPEN by mirroring ze's and overriding only the 2-octet My AS, so `option=asn:value=N` leaves the RFC 6793 AS4 capability holding ze's AS and ze answers NOTIFICATION 2/2 | `generateOpen` (`internal/test/peer/peer.go`) | Harness code, and the third scaffolding repair this session would have made (`ai/rules/pre-release.md`). Row in `plan/journal/guard-added-to-one-half-of-a-pair.md`. It reddens the live gated `test/plugin/peer-port-listener-direct-route.ci`, which is left red |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/peer-local-port-listener.ci` | Yes | `ze-test bgp plugin --list` shows it as test 479 of 771 |
| `internal/component/bgp/reactor/config_test.go` | Yes | `git show --stat ce5ed4459` lists it at +196 |
| `internal/component/bgp/reactor/reactor_peers_dynamic_test.go` | Yes | `git show --stat ce5ed4459` lists it at +144 |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1..AC-5 | The parser splits the two leaves and bounds both | `go test -run 'TestParsePeerFromTreeSplitsLocalAndRemotePorts\|TestParsePeerFromTreeRefusesPortOutsideRange\|TestPeerListenPortReadsTheOperatorsLocalPort\|TestPeerListenPort' ./internal/component/bgp/reactor/`: 30 subtests, `--- PASS` on every one, `ok ... 0.298s` |
| AC-6 | MD5 keys and GTSM TTL follow the local port | Same run: `--- PASS: TestMD5PeersForListener`, `--- PASS: TestListenTTLForListener` |
| AC-7 | `ListenAddrs()` holds the local port and not the remote one | Same run: `--- PASS: TestPeerLocalPortOpensARealListener (0.09s)` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `connection { local { port } }` in a configuration file | `test/plugin/peer-local-port-listener.ci` | Yes. Read the file: the peer's only reachable socket is `$PORT2`, and the harness rewrites every peer's remote port to `$PORT`, so the session can only arrive on the port the local leaf named. Green at 6.9s, TIME at 30s with the fix reverted |
| `connection { local { port } }` in the peer config tree | none (Go test) | Yes. `TestPeerLocalPortOpensARealListener` starts a reactor and reads `ListenAddrs()` |
| `connection { remote { port } }` in the peer config tree | none (Go test) | Yes. `TestParsePeerFromTreeSplitsLocalAndRemotePorts` asserts the remote leaf reaches `Port`, which `Session.Connect` dials |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `./le repository check` reports one issue in the whole tree, `ExtractRemovePrivateASOps` in another session's uncommitted file, so no reader of the merged field is left unwired. `grep -rn LocalPort internal/ --include=*.go` outside tests lists exactly one writer and one reader in the reactor |
| A-2 | confirmed | `listenerPeerKey` returns `only.PeerKey()`, and `PeerKey` reads `Port`. `TestListenerPeerKeyRoutesDirectlyForASoleClaimant` passes |
| A-3 | confirmed | `Session.Connect` (`internal/component/bgp/reactor/session_connection.go`) passes no local address to the dialer, so no source port is bound |
| A-4 | confirmed | `test/perf/configs/ze.conf` sets `local { ip 172.31.0.2; port 1790 }` on one peer and `port 1791` on the other, and neither peer sets a remote port |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| Config syntax (checklist row 2) | `docs/config-reference.md` and `docs/guide/configuration.md` each carry one row per leaf, and their wording matches the `ze:help` in `internal/component/bgp/yang/ze-bgp-conf.yang` | Yes |
| User guide (row 6) | Same, `docs/guide/configuration.md` Peer Settings | Yes |
| Every No row | `./le doc check verify` names no file this spec touched across 8611 lines of findings; `./le spec citation anchors` reported the two declared documents, `docs/architecture/config/apply-ordering.md` and `docs/architecture/core-design.md`, and neither describes a port | Yes |

## Core Insight

A field with one writer can still have four readers that each mean something
different by it. `gopls references` on `PeerSettings.Port` listed 61 sites, and
splitting the field was not the work: classifying every reader as dial, key or
listen was. The tests that had to change were the ones whose SETUP encoded a
meaning, not the ones whose assertions did.
