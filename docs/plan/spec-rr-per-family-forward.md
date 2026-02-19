# Spec: rr-per-family-forward

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `internal/plugins/bgp-rr/server.go` — the RR plugin (channel-based worker from spec 269)
3. `internal/plugins/bgp/reactor/reactor.go:3154-3290` — ForwardUpdate (cache forward engine path)
4. `internal/plugins/bgp/format/text.go` — event JSON formatting
5. `internal/plugins/bgp/server/events.go` — event delivery to plugins

## Task

The bgp-rr plugin currently receives fully decoded UPDATE events and forwards entire cached UPDATEs to all peers that support **any** family. This has two problems:

1. **Wasted decoding** — the engine fully parses NLRI and attributes for every UPDATE event, but the RR plugin only needs the family list to make forwarding decisions. Full decode should only happen on demand.

2. **Protocol-incorrect forwarding** — a peer supporting only ipv4/unicast receives the full cached UPDATE including ipv6/unicast MP_REACH NLRIs it didn't negotiate.

**Four changes needed:**

1. **Lightweight family-only UPDATE events** — new subscription format where the engine sends just the UPDATE id, peer, and family list (no decoded NLRI/attributes). The plugin can request families as integers (default) or as names.

2. **Per-peer decoder goroutine in engine** — UPDATE decoding moves out of the peer read loop into a dedicated per-peer goroutine. This goroutine does partial or full decode depending on what subscribers need.

3. **Per-peer forwarding goroutines in bgp-rr** — pre-started per-peer goroutines that decide: all families match → `cache forward` as-is; partial match → request decode, send per-family route commands.

4. **On-demand UPDATE decode** — plugin can ask the engine to decode a cached UPDATE when it needs the full content (for partial-match peers that need per-family splitting).

**UPDATE family decomposition:**

A single BGP UPDATE can carry up to 3 families (RFC 4760 does not forbid different families in MP_REACH vs MP_UNREACH):

| Component | Family | Wire location |
|-----------|--------|---------------|
| Native NLRI + Withdrawn | ipv4/unicast | UPDATE body directly |
| MP_REACH_NLRI attribute | Any family | Path attribute (AFI/SAFI in first 3 bytes) |
| MP_UNREACH_NLRI attribute | Any family (can differ from MP_REACH) | Path attribute (AFI/SAFI in first 3 bytes) |

Split produces up to 3 sub-UPDATEs:
- One for native ipv4/unicast (NLRI + Withdrawn fields)
- One for MP_REACH announce (if family differs from MP_UNREACH)
- One for MP_UNREACH withdraw (if family differs from MP_REACH)
- If MP_REACH and MP_UNREACH are same family → combined into one sub-UPDATE (2 total)

## Design Decisions

### D-1: `cache forward` stays unchanged

The `cache forward` command sends raw wire bytes to listed peers. No per-family variant needed. The intelligence is in the plugin's forwarding decision, not the engine's forward command.

- Full-family-match peers → `cache N forward peer1,peer2` (zero-copy, as-is)
- Partial-match peers → plugin decodes and sends per-family `update text` commands

### D-2: Lightweight event format

New subscription format (name TBD: `"families"`, `"summary"`, etc.) that delivers:

| Field | Value |
|-------|-------|
| `bgp.message.type` | `"update"` |
| `bgp.message.id` | Cache ID (integer) |
| `bgp.peer.address` | Source peer IP |
| `bgp.peer.asn` | Source peer ASN |
| `bgp.update.families` | Native ipv4/unicast presence (boolean or implicit) |
| `bgp.update.mp-reach` | `[AFI, SAFI]` integer pair, absent if no MP_REACH |
| `bgp.update.mp-unreach` | `[AFI, SAFI]` integer pair, absent if no MP_UNREACH |

No `attr`, no `nlri` — just enough to decide forwarding strategy.

### D-3: Family format preference

Plugin specifies family encoding in subscription:

| Option | Default | Encoding | Example |
|--------|---------|----------|---------|
| `"integer"` | Yes | `[AFI, SAFI]` pair | `[1, 1]` for ipv4/unicast |
| `"name"` | No | String | `"ipv4/unicast"` |

### D-4: Per-peer decoder goroutine (engine-side)

UPDATE decoding moves OUT of the peer TCP read loop:

| Layer | Goroutine | Responsibility |
|-------|-----------|----------------|
| Read loop | Existing per-peer | Read wire bytes from TCP, hand off raw message |
| Decoder | New per-peer | Partial or full decode based on subscriber needs |
| Event delivery | Existing | Send formatted event to subscribed plugins |

For subscribers wanting only families: decoder reads 3 bytes from MP_REACH/MP_UNREACH headers (AFI + SAFI). No NLRI parsing, no attribute extraction.

For subscribers wanting full decode: decoder does complete parsing as today.

### D-5: On-demand decode for partial-match peers

When the RR plugin encounters a partial-match peer, it needs the decoded NLRI to reconstruct per-family UPDATEs. It calls back to the engine:

- Existing `p.DecodeUpdate(ctx, hex, addPath)` can decode a cached UPDATE
- Or a new RPC that decodes a cached UPDATE by ID (avoids re-transmitting raw bytes)

### D-6: Per-peer forwarding goroutines (plugin-side)

| Event | Action |
|-------|--------|
| Peer goes up | Start per-peer goroutine with buffered channel |
| Peer goes down | Close channel, goroutine drains and exits |
| UPDATE arrives | Fan out work items to relevant per-peer channels |

Each per-peer goroutine is sequential (FIFO preserved per peer).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` — Engine/Plugin boundary, cache system
  → Constraint: Plugin communicates via SDK RPCs only
  → Constraint: CacheConsumer plugins must forward or release every cached UPDATE
- [ ] `.claude/rules/plugin-design.md` — SDK callback pattern
  → Constraint: OnEvent must return promptly (synchronous RPC)

### RFC Summaries
- [ ] `rfc/short/rfc4760.md` — Multiprotocol extensions (MP_REACH, MP_UNREACH)
  → Constraint: MP_REACH and MP_UNREACH can be different families in same UPDATE
  → Constraint: AFI (2 bytes) + SAFI (1 byte) at start of each MP attribute

**Key insights:**
- Engine does ZERO per-family filtering on `cache forward` — sends entire UPDATE to all listed peers
- `ExtractRawComponents` in `format/text.go` already breaks UPDATE into per-family raw bytes (used by `format: "full"`)
- MP_REACH family = bytes 0-1 (AFI) + byte 2 (SAFI) of attribute value; same for MP_UNREACH
- Current UPDATE decoding happens inline in the peer read loop — should be moved to a dedicated goroutine
- Plugin already has parsed family names from JSON event, but lightweight events with integer families avoid full decode cost

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/plugins/bgp-rr/server.go` — single forward worker goroutine drains `workCh`; `forwardUpdate` sends one `cache N forward peer1,peer2,...` command for all peers; `selectForwardTargets` includes peer if it supports ANY family
- [ ] `internal/plugins/bgp/reactor/reactor.go:3154-3290` — `ForwardUpdate` sends entire cached UPDATE to all listed peers without family filtering; zero-copy path when contexts match
- [ ] `internal/plugins/bgp/format/text.go:153-236` — `formatFullFromResult` already extracts per-family raw components
- [ ] `internal/plugins/bgp/format/text.go:238-329` — `formatFilterResultJSON` builds parsed UPDATE event JSON
- [ ] `internal/plugins/bgp/server/events.go:27-64` — event delivery with format selection; full decode happens before event delivery
- [ ] `internal/plugin/types.go:267-273` — format constants: `FormatParsed`, `FormatRaw`, `FormatFull`

**Behavior to preserve:**
- `cache N forward peers` command unchanged (zero-copy fast path)
- CacheConsumer protocol: every UPDATE must be forwarded or released
- FIFO ordering per peer
- `selectForwardTargets` source-peer exclusion and down-peer exclusion
- OnEvent must return promptly
- Existing `"parsed"`, `"raw"`, `"full"` format modes unchanged

**Behavior to change:**
- New subscription format for lightweight family-only events
- Family format preference: integer (default) or name
- UPDATE decoding moves from read loop to per-peer decoder goroutine
- Single forward worker → per-peer goroutines in RR plugin
- One cache-forward for all peers → smart grouping by family match

## Data Flow (MANDATORY)

### Entry Point
- UPDATE wire bytes arrive from TCP read loop (unchanged)
- Read loop hands raw message to per-peer decoder goroutine (new)

### Transformation Path
1. Read loop reads BGP message from TCP → hands raw bytes to decoder goroutine
2. Decoder goroutine checks subscriber needs:
   - Family-only subscribers: read 3 bytes from MP_REACH/MP_UNREACH headers
   - Full-decode subscribers: parse attributes, NLRI, build JSON (as today)
3. Engine sends lightweight event to RR plugin: UPDATE id + peer + families
4. RR plugin handleUpdate receives event, extracts family list
5. For each target peer:
   - Full-match → work item to per-peer channel with `cache forward` instruction
   - Partial-match → work item with decode-and-split instruction
6. Per-peer goroutine processes work:
   - Full-match: sends `cache N forward peer` SDK RPC
   - Partial-match: calls engine to decode UPDATE, sends per-family `update text` commands
7. Engine receives commands and sends to peers

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Plugin → Engine | SDK RPC (`UpdateRoute`) — unchanged | [x] |
| Engine → Plugin | Lightweight event JSON with family integers — new | [ ] |
| Read loop → Decoder | Per-peer channel with raw message — new | [ ] |
| handleUpdate → Per-peer goroutine | Per-peer buffered channel — new | [ ] |

### Integration Points
- `sdk.Plugin.UpdateRoute()` — sends cache forward/release commands (unchanged)
- `sdk.Plugin.DecodeUpdate()` — decodes cached UPDATE on demand (existing, reused)
- `format/text.go` — new lightweight format with family integers
- `server/events.go` — event delivery with new format option
- `internal/plugin/types.go` — new format constant

### Architectural Verification
- [ ] No bypassed layers (per-peer channels are internal to their respective layers)
- [ ] No unintended coupling (family integers are read-only event metadata)
- [ ] No duplicated functionality (lightweight format is a subset, not a copy)
- [ ] Zero-copy preserved for full-family-match peers

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | UPDATE with ipv4/unicast only, all peers support it | Single `cache N forward` to all peers (fast path preserved) |
| AC-2 | UPDATE with ipv4/unicast + ipv6/unicast (MP_REACH), peer supports only ipv4 | Peer receives only ipv4/unicast portion via `update text` |
| AC-3 | UPDATE with MP_REACH family X and MP_UNREACH family Y (different), peer supports only X | Peer receives only the announce portion |
| AC-4 | UPDATE with MP_REACH and MP_UNREACH same family, peer supports it | Peer receives combined sub-UPDATE (one command) |
| AC-5 | Peer goes up → per-peer goroutine started; peer goes down → goroutine stopped | No goroutine leak, no send on closed channel |
| AC-6 | Plugin subscribes with `family-format: "integer"` (default) | Event contains `mp-reach: [AFI, SAFI]` as integers |
| AC-7 | Plugin subscribes with `family-format: "name"` | Event contains `mp-reach: "afi/safi"` as string |
| AC-8 | 100 rapid UPDATEs to a partial-match peer | Per-family commands arrive in FIFO order per peer |
| AC-9 | UPDATE decoding does NOT happen in peer read loop | Decoder goroutine handles parsing |
| AC-10 | All existing propagation tests still pass | No regression |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestPerPeerWorker_FullMatchUsesCache` | `propagation_test.go` | Full-match peers get single cache-forward | |
| `TestPerPeerWorker_PartialMatchSplits` | `propagation_test.go` | Partial-match peers get per-family commands | |
| `TestPerPeerWorker_DifferentMPFamilies` | `propagation_test.go` | MP_REACH ≠ MP_UNREACH → 3-way split | |
| `TestPerPeerWorker_SameMPFamily` | `propagation_test.go` | MP_REACH = MP_UNREACH → combined sub-UPDATE | |
| `TestPerPeerWorker_StartStop` | `propagation_test.go` | Goroutine lifecycle on peer up/down | |
| `TestPerPeerWorker_OrderPreserved` | `propagation_test.go` | FIFO ordering within a single peer | |
| `TestLightweightEvent_IntegerFamilies` | `format/text_test.go` | Family integers in lightweight event | |
| `TestLightweightEvent_NameFamilies` | `format/text_test.go` | Family names when requested | |
| `TestLightweightEvent_NativeIPv4Only` | `format/text_test.go` | No mp-reach/mp-unreach for native-only UPDATE | |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| chaos test | `cmd/ze-chaos/` | 4-peer 7-family route reflection with mixed capabilities | deferred — run manually |

## Files to Modify

- `internal/plugins/bgp-rr/server.go` — per-peer goroutines, family-match grouping, lightweight event parsing
- `internal/plugins/bgp-rr/propagation_test.go` — per-peer worker tests
- `internal/plugins/bgp/format/text.go` — new lightweight format with family integers
- `internal/plugins/bgp/server/events.go` — decoder goroutine, format dispatch
- `internal/plugin/types.go` — new format constant, family-format preference
- `pkg/plugin/rpc/types.go` — family-format field in `SubscribeEventsInput`

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | |
| RPC count in architecture docs | No | |
| CLI commands/flags | No | |
| CLI usage/help text | No | |
| API commands doc | No | `cache forward` unchanged |
| Plugin SDK docs | Yes | `.claude/rules/plugin-design.md` — document family-format subscription option |
| Editor autocomplete | No | |
| Functional test for new RPC/API | Yes | Integration tests in propagation_test.go |

## Files to Create

None.

## Implementation Steps

Each step ends with a **Self-Critical Review**.

1. **Add lightweight format to engine** — new format constant (e.g., `FormatFamilies`), reads 3 bytes from MP_REACH/MP_UNREACH headers, builds minimal JSON with family integers or names based on preference
   → **Review:** Does it handle native ipv4/unicast (no MP attributes)?

2. **Add family-format preference to subscription** — extend `SubscribeEventsInput` with `family-format` field, default `"integer"`
   → **Review:** Backwards compatible? Existing subscribers unaffected?

3. **Move UPDATE decoding to per-peer decoder goroutine** — read loop hands raw message to channel, decoder goroutine does partial or full decode based on subscribers
   → **Review:** Does this affect latency? Is the channel bounded?

4. **Implement per-peer forwarding goroutines in RR plugin** — replace single worker with per-peer goroutines, started on peer-up, stopped on peer-down
   → **Review:** Clean lifecycle? No goroutine leaks? FIFO preserved per peer?

5. **Implement family-match grouping** — classify each target peer as full-match or partial-match, full-match → cache forward, partial-match → decode and per-family route commands
   → **Review:** Full-match fast path preserved? On-demand decode works?

6. **Write tests**
   → **Review:** Tests cover all split cases? Integer and name formats?

7. **Run tests** — `go test -race ./internal/plugins/bgp-rr/... -v`
   → **Review:** All tests pass? Race detector clean?

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| MP_REACH and MP_UNREACH must be same family | RFC 4760 allows different families | User correction | Design must handle 3 families |
| Per-family forward command needed in engine | `cache forward` stays as-is, plugin handles splitting | User correction | Simpler engine, smarter plugin |
| Plugin needs full decoded UPDATE to decide forwarding | Only needs family list — lightweight event sufficient | User correction | Major performance improvement |
| Decoding in read loop is fine | Should be in dedicated per-peer goroutine | User requirement | Unblocks read loop |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

- The RR plugin's primary decision (forward or split) depends ONLY on the family list — full UPDATE decoding is wasted for the common case (single-family or full-match).
- `cache forward` is the correct abstraction — it forwards raw wire bytes. Adding per-family intelligence to the forward command would mix concerns. The plugin decides WHAT to forward; the engine handles HOW.
- Per-peer decoder goroutines in the engine enable subscriber-driven decode depth: family-only (3 bytes), full decode, or raw bytes. Different plugins can get different levels of processing for the same UPDATE.
- Family format preference (integer vs name) defaults to integer because integer comparison is cheaper and avoids string allocation on the hot path. Plugins that need human-readable names opt in.

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

## Checklist

### Goal Gates (MUST pass)
- [ ] Acceptance criteria AC-1..AC-10 all demonstrated
- [ ] Tests pass (`make ze-unit-test`)
- [ ] No regressions (`make ze-functional-test`)
- [ ] Feature code integrated into codebase (`internal/*`)
- [ ] Integration completeness: per-family forward proven through real SDK connections

### Quality Gates (SHOULD pass)
- [ ] `make ze-lint` passes
- [ ] Implementation Audit fully completed

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL
- [ ] Implementation complete
- [ ] Tests PASS
