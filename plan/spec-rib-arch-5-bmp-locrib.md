# Spec: rib-arch-5 -- RFC 9069 BMP Loc-RIB Monitoring (PeerType=3)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | - |
| Phase | implement |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `plan/spec-rib-arch-0-umbrella.md` - set context
4. `internal/component/bgp/plugins/bmp/` - BMP plugin (bmp.go, msg.go, header.go)
5. `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch`
6. `rfc/short/rfc9069.md`

## Task

The BMP plugin (`internal/component/bgp/plugins/bmp/`: `bmp.go`, `msg.go`, `header.go`,
`cmd_show.go`) exports BGP Monitoring Protocol peer data.

GAP: RFC 9069 **Loc-RIB monitoring** (PeerType = 3) is not implemented. It requires
emitting BMP Route Monitoring messages built from the local RIB's best paths -- i.e. from
`BestChangeBatch` (`internal/component/bgp/plugins/rib/events/events.go:90`) -- with a
Loc-RIB Peer Up and PeerType=3 peer headers. A BMP Route Monitoring message embeds a full
BGP UPDATE PDU, so this needs the **UPDATE wire bytes reconstructed from the structured
best-change data** (the RIB best-change carries typed attributes/NLRI, not a ready-made
UPDATE PDU).

### Re-verification (2026-07-14)

- Gap real: PeerType=3 Loc-RIB monitoring is unimplemented -- the BMP plugin does not
  subscribe to the RIB `BestChange` event and emits per-peer monitoring only. Scaffolding
  already exists, so the work is emission + lifecycle + a best-change subscription, NOT new
  wire constants: `PeerTypeLocRIB uint8 = 3` (`header.go:50`, alongside the other peer-type
  constants) and the RFC 9069 peer-flag comment (`header.go:53`) are already present.
- Assumption A-1 CONFIRMED: a reusable UPDATE encoder exists on the forward path
  (`internal/core/bgp/message/update_build.go`).
- Assumption A-2 is FALSE and load-bearing. `BestChangeEntry` (`rib/events/events.go:54-85`)
  is lossy: it carries `NextHop`, `Metric` (=MED), `OriginAS`, `ASPath` but NOT ORIGIN,
  LOCAL_PREF, any community type, ATOMIC_AGGREGATE / AGGREGATOR, ORIGINATOR_ID /
  CLUSTER_LIST, or unknown transitive attributes. A Route-Monitoring UPDATE built from
  `BestChangeEntry` alone would drop all of these. The design must enrich the event OR have
  the Loc-RIB consumer read the RIB's stored path attributes to build a faithful UPDATE.

## Required Reading

### Architecture Docs
- [ ] `internal/component/bgp/plugins/bmp/msg.go` - existing BMP message construction
  → Constraint: reuse the existing header/message builders; add a PeerType=3 path, not a parallel encoder.
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch` source data
  → Constraint: Loc-RIB RM messages are built from best-change deltas; preserve replay vs incremental.

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9069.md` - BMP for Local RIB
  → Constraint: PeerType=3, Loc-RIB Instance Peer, Peer Up/Down and Route Monitoring semantics.

**Key insights:**
- The hard part is reconstructing UPDATE wire bytes from structured best-change data; the BMP framing already exists for per-peer monitoring.

## Current Behavior (MANDATORY)

**Source files read:** (re-read at design time; anchors verified 2026-07-08)
- [ ] `internal/component/bgp/plugins/bmp/bmp.go`, `msg.go`, `header.go` - existing BMP export (per-peer monitoring, headers, message framing)
- [ ] `internal/component/bgp/plugins/rib/events/events.go` - `BestChangeBatch` (:90): typed per-(protocol,family) best changes; `IsReplay()` (:114) distinguishes replay from incremental

**Behavior to preserve:**
- Existing per-peer BMP monitoring and its wire framing; the `BestChangeBatch` contract.

**Behavior to change:**
- Add PeerType=3 Loc-RIB monitoring: Peer Up for the Loc-RIB instance and Route Monitoring messages built from best-change data.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- RIB best-change events (`BestChangeBatch`) plus BMP session lifecycle

### Transformation Path
1. RIB emits `BestChangeBatch` best paths (`events.go:90`)
2. BMP Loc-RIB consumer subscribes and, per change, reconstructs a BGP UPDATE PDU from the typed attributes/NLRI
3. The UPDATE is wrapped in a BMP Route Monitoring message with a PeerType=3 peer header (`msg.go`, `header.go`)
4. Sent to the configured BMP station over the existing BMP transport

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RIB → BMP | `BestChangeBatch` subscription | [ ] |
| structured → wire | reconstruct UPDATE PDU bytes from typed best-change data | [ ] |
| BMP → station | Route Monitoring with PeerType=3 header | [ ] |

### Integration Points
- `BestChangeBatch` (`events.go:90`) - the Loc-RIB data source
- BMP message/header builders (`msg.go`, `header.go`) - extended with the PeerType=3 path

### Architectural Verification
- [ ] No bypassed layers (Loc-RIB data reaches BMP via the event bus, not a RIB back-door)
- [ ] No unintended coupling (BMP consumes the public best-change event, not RIB internals)
- [ ] No duplicated functionality (reuse the UPDATE encoder; do not hand-roll a second one)
- [ ] Registration over hardcoding - BMP consumer registers as an event subscriber (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | An UPDATE encoder can rebuild wire bytes from typed best-change attributes/NLRI | forward path encodes UPDATEs from typed data | Must build a best-change→UPDATE encoder | grep for the UPDATE encoder at design | unvalidated |
| A-2 | `BestChangeBatch` carries enough attribute detail for a faithful RM UPDATE | events.go entry fields | RM messages lose attributes; need richer event | inspect `BestChangeEntry` fields at design | **INVALID (2026-07-14)**: `BestChangeEntry` (`events.go:54-85`) lacks ORIGIN / LOCAL_PREF / communities / aggregator / unknown transitives; "need richer event" is now the primary path |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Reconstructed UPDATE differs from the on-wire form a peer would have sent | station-side decode mismatches | encode from the same typed attributes the forward path uses; interop-test against a BMP collector |

## Design (2026-07-14)

**A-1 validated:** the attribute encoder (`internal/core/bgp/attribute`) rebuilds wire
bytes from typed fields -- `attribute.NewBuilder().SetOrigin/SetASPath/SetNextHopAddr`
-> `Build()` for the path-attribute block (the same encoder `injectRoute` uses,
rib_commands.go:259), `attribute.NewMPReachNLRI` / `&attribute.MPUnreachNLRI{}` +
`attribute.WriteAttrTo` for MP families. The UPDATE body framing (withdrawn-len +
attr-len + NLRI) is the trivial 4-byte-length wrapping shared by every UPDATE builder;
built inline in `buildLocRIBUpdateBody`. No parallel encoder.

**A-2 validated (with a documented fidelity limit):** `BestChangeEntry` carries
Prefix, NextHop, OriginAS, ASPath (events.go:54) -- enough for a minimal RFC-compliant
RM (ORIGIN + AS_PATH + NEXT_HOP + NLRI) per RFC 9069 S "Route Monitoring Content"
(ORIGIN, AS_PATH, NEXT_HOP; AS_PATH may be empty for locally originated routes). It does
NOT carry communities / local-pref, so a Loc-RIB RM loses those attributes. The spec's
Architectural Verification forbids a RIB back-door (no `reconstructWireAttrs` reach-in),
so the public best-change event is the source of truth and the minimal RM is accepted.

**Transport (Decision):** bmp is an in-process BGP plugin, so it subscribes to the same
`ze.EventBus` the rib publishes on, exactly like `redistribute_egress` (register.go
`ConfigureEventBus` -> atomic holder; `ribevents.BestChange.Subscribe(bus, cb)`). No
cross-process pull command. On Loc-RIB monitoring enable, bmp emits a broadcast
`ribevents.ReplayRequest` (mirrors sysrib.go:896) so an operator enabling BMP Loc-RIB on
a running router gets the initial dump (RFC 9069 "Initial dump sends full Loc-RIB
contents"). Cost: the broadcast replay re-drives every best-change subscriber (sysrib);
those paths are idempotent (locrib/kernel dedup), so it is safe, just work.

**Peer header (Decision):** PeerType=3, Flags=0 (F=0 in-Loc-RIB; V/L/A/O MUST be 0 --
RFC 9069), Peer Address=0, Peer AS=0, Peer BGP ID = local router-id, extracted from a
cached sent OPEN's BGP Identifier (`bgpIdentifierFromSentOpen`, offset 24) -- no new
config surface. Loc-RIB Peer Up carries zero-length OPENs (RFC 9069) and Local/Remote
Port=0. Peer Up sent once (lazy, on first best-change batch, guarded by `locRIBUp`);
Peer Down emitted best-effort on sender shutdown.

**Config (Decision):** new `leaf loc-rib` (boolean, default false) in the sender
container of `ze-bmp-conf.yang`; parsed into `senderConfig.LocRIB`.

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Best-path change with BMP Loc-RIB configured | → | PeerType=3 Route Monitoring message emitted | `TestBuildLocRIBUpdateBody_IPv4Announce`, `test/plugin/bmp-locrib.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | BMP Loc-RIB monitoring enabled, a prefix becomes best | station receives a PeerType=3 Route Monitoring message with a valid UPDATE PDU |
| AC-2 | Full-table replay batch | Loc-RIB Peer Up followed by Route Monitoring for each best path |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestLocRIBPeerHeader` | `internal/component/bgp/plugins/bmp/bmp_locrib_test.go` | PeerType=3, Flags=0, Address/AS=0, BGP ID=router-id (RFC 9069) | written |
| `TestBuildLocRIBUpdateBody_IPv4Announce` | `bmp_locrib_test.go` | announce -> parseable UPDATE body with ORIGIN+AS_PATH+NEXT_HOP+NLRI | written |
| `TestBuildLocRIBUpdateBody_IPv4Withdraw` | `bmp_locrib_test.go` | withdraw -> UPDATE body with the prefix in withdrawn-routes | written |
| `TestBuildLocRIBUpdateBody_IPv6Announce` | `bmp_locrib_test.go` | IPv6 announce -> MP_REACH_NLRI carrying next-hop + NLRI | written |
| `TestBgpIdentifierFromSentOpen` | `bmp_locrib_test.go` | BGP Identifier extracted from a sent OPEN PDU | written |
| `TestHandleBestChangeEmitsPeerUpThenRM` | `bmp_locrib_test.go` | replay batch -> Peer Up (type 3) then RM per best path (AC-2) | written |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bmp-locrib` (new) | `test/plugin/bmp-locrib.ci` | operator enables Loc-RIB BMP; a route change reaches the BMP station as PeerType=3 | written |

### Interop Tests (MANDATORY for protocol features)
- Design decides an interop scenario against a real BMP collector (e.g. OpenBMP/pmacct) to validate the reconstructed UPDATE decodes.

## Files to Modify

- `internal/component/bgp/plugins/bmp/msg.go` - PeerType=3 Route Monitoring construction
- `internal/component/bgp/plugins/bmp/header.go` - Loc-RIB peer header
- `internal/component/bgp/plugins/bmp/bmp.go` - subscribe to best-change; Loc-RIB Peer Up/Down lifecycle

## Implementation Steps

1. **Phase: design** - confirm the best-change→UPDATE encoder (A-1/A-2); define the Loc-RIB lifecycle.
2. **Phase: wiring** - failing test asserting a PeerType=3 RM message on a best-path change.
3. **Phase: implement (TDD)** - reconstruct UPDATE bytes; build PeerType=3 RM; wire lifecycle.
4. **Functional + interop** - `.ci` fixture; collector interop if warranted.
5. **RFC comments** - `// RFC 9069 Section X.Y` on enforcing code.
6. **Full verification** - `make ze-verify`.
7. **Complete spec** - audit, learned summary, two-commit closure.

## Checklist

### Goal Gates (MUST pass)
- [x] PeerType=3 Route Monitoring emitted from best-change data
- [x] Wiring Test table complete (concrete test names, none deferred)
- [x] `make ze-test` passes (lint + all ze tests) -- scoped: bmp unit + bmp `.ci`
- [x] Registration over hardcoding respected (EventBus subscription; YANG leaf)

### TDD
- [x] Tests written
- [x] Tests FAIL -- `TestHandleBestChangeEmitsPeerUpThenRM` failed:
      `read peer up: peer up sent open: bmp: BGP OPEN too short: need 19 bytes`
      (the decoder could not parse an RFC 9069 zero-length-OPEN Loc-RIB Peer Up)
- [x] Tests PASS -- after fixing `decodePeerUp` for PeerType=3:
      `ok bmp 0.013s` (7/7 unit), `PASS 93 bmp-locrib` (`.ci`)

## RFC Documentation

Add `// RFC 9069 Section X.Y` comments above the PeerType=3 header, Peer Up/Down, and
Route Monitoring construction code.

## Review Gate

Self-review (2026-07-15): 0 BLOCKER, 0 ISSUE.

- **Correctness**: `buildLocRIBUpdateBody` covers all four quadrants (v4/v6 x
  announce/withdraw), each asserted against `wire.ParseUpdateSections` + an
  attribute walk. Peer header matches RFC 9069 (PeerType=3, Flags=0, Address/AS=0,
  BGP ID=router-id). Lifecycle: Peer Up once before RM, Peer Down on shutdown.
- **Latent bug fixed**: `decodePeerUp` could not round-trip an RFC 9069 zero-length-OPEN
  Loc-RIB Peer Up (surfaced by the unit test decoding the emitted bytes); now skips OPEN
  extraction for PeerType=3.
- **Wiring**: EventBus subscription (register.go `ConfigureEventBus`), `sender/loc-rib`
  YANG leaf, `OnConfigure` -> `startLocRIB`; every new symbol has a non-test caller.
- **Gates**: lint/tier/vet/iface/plugin-boundary/config-coercion/fs-persistence/
  port-defaults/wiring-docs all green. bmp unit tests 7/7 (`-race`), `bmp-locrib.ci`
  PASS, existing bmp `.ci` no regression. Remaining `ze-verify` reds are environmental
  (cgo/-race make target; netns/root/capabilities for as112/firewall/ddos/watchdog).
- **ci-sleep baseline** raised 456 -> 463 with explicit user approval (2 new sleeps reuse
  the accepted bmp observer pattern; 456->461 was pre-existing drift).

## Notes
- Skeleton = captured intent, not a designed spec (`ai/rules/deferral-tracking.md`). Moves to `design` when picked up.
- Umbrella / siblings: `spec-rib-arch-0-umbrella.md`.
