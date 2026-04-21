# Spec: Remote RIB Subscription Client

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-04-11 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/bgp/plugins/rib/` - current RIB + `rib inject` path
4. `internal/component/bgp/reactor/` - how routes enter the engine
5. `ai/patterns/plugin.md` - plugin shape

## Task

Operators of route collectors, public looking glasses, and research platforms
want to mirror another router's RIB into a local ze instance without running a
real BGP session to the source. Today, ze can receive routes only from a real
BGP peer (or from the `rib inject` CLI, which is operator-driven and not
continuous).

This spec covers an **opt-in subscription client**: ze opens a connection to a
remote RIB source that speaks a documented streaming protocol, receives a live
feed of prefix + attribute updates, and injects them into the local RIB as if
they came from a synthetic peer. The synthetic peer is first-class for RIB,
filter, and policy purposes; it simply has no outbound BGP session.

Out of scope:
- Designing a new wire protocol. This spec mandates using an existing,
  documented streaming RIB format (gRPC-based). The specific choice is a
  design decision inside the spec.
- Acting as a server. This spec is client-only.
- Replacing the on-demand `rib inject` CLI.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - engine + plugin model
- [ ] `docs/architecture/meta/README.md` - route metadata keys (the synthetic peer must set sensible ones)
- [ ] `ai/patterns/plugin.md`, `ai/patterns/config-option.md`
- [ ] `ai/rules/design-principles.md`
  → Constraint: lazy over eager; the subscription path must not allocate new
  structs per prefix if the wire format can be iterated lazily.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc4271.md` - base BGP wire format (the remote feed claims BGP semantics)
- [ ] `rfc/short/rfc4760.md` - multiprotocol families

**Key insights:**
- The subscription client is a new inbound route source. It must funnel into
  the same RIB entry point as BGP peer updates to guarantee identical policy
  treatment.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/plugins/rib/` - RIB plugin, `rib inject` handler
- [ ] `internal/component/bgp/reactor/` - peer lifecycle, UpdateSource interface
- [ ] `internal/component/bgp/plugins/` - plugin registration pattern
- [ ] `internal/yang/modules/ze-bgp-conf.yang` - where to put the new config node

**Behavior to preserve:**
- Existing BGP peers and `rib inject` CLI unchanged.
- The RIB's single entry point for new routes remains the reactor dispatch;
  the new plugin goes through that path.

**Behavior to change:**
- Add a new `remote-rib` plugin that opens a subscription to a configured
  remote service, receives a streaming feed, and turns each update into a
  route event dispatched to the RIB via the same path as a real peer.

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- Config enables `remote-rib` with a connection URL, family list, optional
  auth credentials, and a local synthetic peer name.

### Transformation Path
1. Plugin startup - open streaming client to the configured endpoint
2. For each inbound update in the stream: decode to a prefix + path tuple
3. Materialize a route event using the synthetic-peer identity
4. Dispatch through the same reactor path used by a real peer
5. Withdraws and resyncs map to the equivalent events

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Network -> plugin | streaming gRPC client | [ ] |
| Plugin -> reactor | same dispatch as a peer, new source type `remote-rib` | [ ] |
| Plugin -> bus | connection state events on `remote-rib/*` | [ ] |

### Integration Points
- Reactor's existing "route-from-peer" path. The synthetic peer has a stable
  ID so the RIB can group/withdraw its routes.
- `rib inject` handler for the per-route injection primitive (if reusable).
- Web / CLI: show synthetic peer in `ze bgp peers` with a clear tag.

### Architectural Verification
- [ ] Routes from the synthetic peer hit the same filter chain as real peers
- [ ] Withdraw-all on disconnect is automatic
- [ ] Zero-copy preserved where applicable
- [ ] No parallel RIB path

## Wiring Test (MANDATORY - NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Config enables `remote-rib` | → | plugin opens client | `TestRemoteRIBStartsFromConfig` |
| Remote stream emits an update | → | reactor dispatch path | `TestRemoteRIBUpdateReachesRIB` |
| Remote stream emits a withdraw | → | RIB withdraw | `TestRemoteRIBWithdrawRemovesRoute` |
| Remote stream disconnects | → | plugin withdraws all routes from synthetic peer | `TestRemoteRIBDisconnectDrainsRoutes` |
| CLI `ze bgp peers` | → | synthetic peer visible with source tag | `test/plugin/remote-rib-peer-visible.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Config enables `remote-rib` with URL + family list | Plugin opens a streaming client; connection state visible in `ze bgp peers` |
| AC-2 | Invalid URL | Commit-time validation rejects the config with a helpful error |
| AC-3 | Stream yields an IPv4 unicast update | Prefix + attributes appear in the RIB under the synthetic peer |
| AC-4 | Stream yields an IPv6 unicast update | Same, under `ipv6/unicast` |
| AC-5 | Stream yields a withdraw | Prefix removed from the synthetic peer's adj-rib-in |
| AC-6 | Stream disconnects | Synthetic peer is marked down; all its routes withdrawn from RIB |
| AC-7 | Filter chain drops a prefix | Prefix is not installed in the RIB, same as a real peer |
| AC-8 | Reload with `remote-rib` disabled | Plugin stops cleanly, all routes withdrawn |
| AC-9 | Remote sends unknown families | Unknown families are ignored with a counter increment; no panic |
| AC-10 | Reconnect after disconnect | Plugin retries with backoff; on success, reinjects routes |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestRemoteRIBDecodeUpdate` | `internal/component/bgp/plugins/remote_rib/decode_test.go` | Streaming message -> prefix + path | |
| `TestRemoteRIBSyntheticPeer` | `internal/component/bgp/plugins/remote_rib/peer_test.go` | Synthetic peer ID stable across reconnects | |
| `TestRemoteRIBReconnectBackoff` | `internal/component/bgp/plugins/remote_rib/reconnect_test.go` | Backoff respects max cap | |
| `TestRemoteRIBStartsFromConfig` | `internal/component/bgp/plugins/remote_rib/plugin_test.go` | Config triggers client | |

### Boundary Tests
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Reconnect backoff | 1s..300s | 300s | 0s | 301s |
| Max subscribed families | 1..N | N | 0 | N+1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-remote-rib-ingest` | `test/plugin/remote-rib-ingest.ci` | Operator configures client, synthetic peer ingests a synthetic stream | |
| `test-remote-rib-disconnect` | `test/plugin/remote-rib-disconnect.ci` | Disconnect withdraws all routes | |

### Future (if deferring any tests)
- Interop against a real public RIB streaming server - requires network
  fixture.

## Files to Modify
- `internal/yang/modules/ze-bgp-conf.yang` - new `remote-rib` container
- `internal/component/bgp/plugins/all.go` - import registrar
- `docs/features.md` - new "Remote RIB client" bullet
- `docs/comparison.md` - add row

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | Yes | `internal/yang/modules/ze-bgp-conf.yang` |
| CLI commands/flags | Yes | expose synthetic peer state in `ze bgp peers` |
| Editor autocomplete | Yes (auto) | - |
| Functional test for new API | Yes | `test/plugin/remote-rib-*.ci` |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` |
| 3 | CLI command added/changed? | Yes (display only) | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/remote-rib.md` |
| 7 | Wire format changed? | No | - |
| 8 | Plugin SDK/protocol changed? | No | - |
| 9 | RFC behavior implemented? | No | - |
| 10 | Test infrastructure changed? | Yes (synthetic stream server) | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` |

## Files to Create
- `internal/component/bgp/plugins/remote_rib/doc.go`
- `internal/component/bgp/plugins/remote_rib/register.go`
- `internal/component/bgp/plugins/remote_rib/plugin.go`
- `internal/component/bgp/plugins/remote_rib/client.go`
- `internal/component/bgp/plugins/remote_rib/decode.go`
- `internal/component/bgp/plugins/remote_rib/peer.go`
- `internal/component/bgp/plugins/remote_rib/reconnect.go`
- `internal/component/bgp/plugins/remote_rib/*_test.go`
- `docs/guide/remote-rib.md`
- `test/plugin/remote-rib-ingest.ci`
- `test/plugin/remote-rib-disconnect.ci`

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files, tests |
| 3. Implement (TDD) | Phases |
| 4. Full verification | `make ze-verify` |
| 5. Critical review | Checklist |
| 6. Fix issues | - |
| 9. Deliverables review | Checklist |
| 10. Security review | Checklist |
| 12. Present summary | Executive Summary |

### Implementation Phases

1. **Phase: Protocol choice** - pick the streaming format (gRPC-based). Document
   the choice and interop goal in the spec before writing code.
2. **Phase: Synthetic peer shape** - design how the plugin presents as a peer to
   the reactor without owning a real BGP session.
3. **Phase: Decoder** - map streaming messages into ze's existing prefix + path
   types. Reuse `internal/component/bgp/message` where possible.
4. **Phase: Client lifecycle** - connect, backoff, disconnect drains.
5. **Phase: Config + YANG** - new container with URL, families, credentials.
6. **Phase: Functional tests** - synthetic server fixture in `test/`.
7. **Phase: Docs** - user guide, features, comparison.
8. **Full verification** - `make ze-verify`.
9. **Complete spec** - audit + learned summary.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Each AC has file:line |
| Correctness | Withdraw on disconnect drains RIB deterministically |
| Naming | Plugin named `remote-rib`, config `remote-rib` |
| Data flow | Single reactor dispatch path shared with BGP peers |
| Rule: no-layering | No parallel RIB ingestion path |
| Rule: lazy-over-eager | Stream decode uses iterators, no per-prefix structs buffered |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| Plugin registered | grep `Register` |
| Synthetic peer visible | CLI output from functional test |
| Ingest + withdraw verified | functional test logs |
| Reconnect backoff | unit test output |
| User guide exists | `ls docs/guide/remote-rib.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Input validation | Reject non-matching families; reject malformed stream messages |
| Resource exhaustion | Bound queue depth; shed load if RIB is slow |
| Auth | TLS + credentials; document what is required |
| Replay protection | Stream sequence numbers if protocol supports them |
| Error leakage | Errors name fields, not internal addresses |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Malformed message | Skip, increment counter, keep stream |
| Stream disconnect | Drain + backoff |
| 3 fix attempts fail | STOP. Report. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

## Design Insights

## RFC Documentation

Not protocol RFC work. Document any RFC constraints the streaming format
implies (family codes, path-id location).

## Implementation Summary

### What Was Implemented
- (fill during /implement)

### Bugs Found/Fixed
- (fill during /implement)

### Documentation Updates
- (fill during /implement)

### Deviations from Plan
- (fill during /implement)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:**
- **Skipped:**
- **Changed:**

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-10 all demonstrated
- [ ] Wiring Test table complete
- [ ] `make ze-test` passes
- [ ] Plugin integrated end-to-end
- [ ] Architecture docs updated

### Design
- [ ] No premature abstraction
- [ ] No speculative features
- [ ] Single responsibility
- [ ] Explicit > implicit
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional tests for end-to-end behavior

### Completion
- [ ] Critical Review passes
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Learned summary written to `plan/learned/NNN-remote-rib-client.md`
- [ ] Summary included in commit
