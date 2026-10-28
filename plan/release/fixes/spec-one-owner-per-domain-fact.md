# Spec: one-owner-per-domain-fact

| Field | Value |
|-------|-------|
| Status | design |
| Scope | protocol |
| Priority | P1 |
| Release reason | Shipped BGP, RPKI, IKE, VPP, firewall and web paths answer one domain question differently depending on which copy a path reaches: an RFC 7705 migration session reads as EBGP on two rails, RPKI derives the origin AS by RFC 6811 on one rail only, and four VPP consumers can hang forever. These are defects in implemented capabilities |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-23 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Root cause.** A domain fact Ze relies on (who is an IBGP neighbour, what LOCAL_PREF Ze adds, how an RFC 6811 origin AS is derived, how a labelled withdrawal is framed, which families are CIDR, which IP protocol number has which name, how a govpp channel is bounded, where an env key is registered, how `set` resolves a list key) has no single owner. Each path that needs it writes its own copy: a literal, an open-coded comparison, a private table, a second parser, or a second rail decoding the same bytes. When a guard or a correction is added, it lands on the copy the author was looking at, and the other copies keep the old answer.

**Symptoms (48 open journal rows, seven classes).** `neighborAS` in the RIB and the `filter_community` relation tag compare `PeerASN == LocalASN` and ignore `MigrationAS`. LOCAL_PREF 100 is written as a literal by six producers. `originASFromParsed` takes the last ASN regardless of segment type while `rpkiOriginASFromASPath` follows RFC 6811. Five decoders frame an RFC 8277 withdrawal as an announcement. Only the firewall and traffic VPP backends set a reply deadline. `protoName` is a fourth protocol-name table. The web `set` and the SSH `set` resolve a list key differently, and the session serializer writes a line the parser refuses.

**Why it matters for shipping Ze.** Each copy is a protocol answer Ze gives a peer or an operator. Two answers to one question is a correctness defect on whichever rail holds the stale copy, and it is the shape every later fix repeats: the journal shows guards added to one half of a pair forty-one times.

**Goal.** One owner per fact: every fact gets one exported function, constant or table in the package that owns the domain, every copy is deleted and its callers moved to the owner (no-layering, no wrapper), and a static check refuses a new copy.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/bgp-engine.md`, `ai/doors/bgp-plugins.md`, `ai/doors/system-plugins.md`, `ai/doors/core.md` - boundaries the owners must respect
- [ ] `docs/architecture/route-selection.md` - the IBGP and neighbour-AS rules `neighborAS` implements
  → Constraint: plugins may run out of process, so an owner reachable only by a Go call does not serve an external plugin; plugins consume raw attributes through the shared accessor in `pkg/` or `internal/core`
- [ ] `docs/architecture/config/environment.md` - env registration rule
- [ ] `docs/guide/vpp.md` - VPP connector and channel ownership
- [ ] `ai/rules/principles.md` - "Every fact MUST be declared once"
- [ ] `ai/rules/no-layering.md` - delete the copy, do not wrap it

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc7705.md` - migration AS and the IBGP rule
- [ ] `rfc/short/rfc6811.md` - Route Origin ASN derivation
- [ ] `rfc/short/rfc8277.md` - withdrawal Compatibility field
- [ ] `rfc/short/rfc4271.md` - LOCAL_PREF, simultaneous announce and withdraw
- [ ] `rfc/short/rfc7999.md` - BLACKHOLE readvertisement

**Key insights:**
- The analysis named "JSON rail and raw rail" as a source of copies: a plugin that decodes event JSON reimplements what the raw accessor already answers. The fix makes JSON output-only for BGP semantics.

## Current Behavior (MANDATORY)

**Source files read (producers verified 2026-10-23):**
- [ ] `internal/component/bgp/plugins/rib/bestpath.go` `neighborAS` (line 660): returns `LocalASN` when `PeerASN == LocalASN`, no `MigrationAS` term. Still open.
- [ ] `internal/component/bgp/plugins/rpki/rpki.go` `originASFromParsed` (line 1150) returns the last element of a flat `[]uint32`, which cannot know the final segment type; `rpkiOriginASFromASPath` (line 1377) reads the segment type as RFC 6811 Section 2 requires. Two derivations of one fact on two rails. Still open.
- [ ] LOCAL_PREF 100: `attribute.LocalPref(100)` literal at `internal/component/bgp/rib/commit.go` line 317 and `internal/component/bgp/reactor/peer_rib_routes.go` line 178; `localPrefDefault` constant at `internal/component/bgp/message/update_build.go` line 486; a separate byte array `localPrefDefault` at `internal/component/bgp/reactor/forward_local_pref.go` line 77. Still open.
- [ ] `internal/plugins/trafficusage/metrics.go` `protoName` (line 46) beside `firewall.ProtocolName` (`internal/component/firewall/protocol.go` line 48). Still open.
- [ ] govpp channels: `NewChannel` is called in `internal/plugins/iface/vpp/ifacevpp.go` line 200, `internal/plugins/static/backend_vpp_linux.go` line 36, `internal/plugins/fib/vpp/register.go` line 157 and `internal/component/ike/dataplane/vpp.go` line 81 with no `SetReplyTimeout`; only `internal/plugins/firewall/vpp/timeout_linux.go` and `internal/plugins/traffic/vpp/timeout_linux.go` set one. `vpp.Connector.NewChannel` (`internal/component/vpp/conn.go` line 93) hands out an unbounded channel. Still open.

**Behavior to preserve:**
- Every answer the owner already gives where it is correct today (for example `rpkiOriginASFromASPath`, `firewall.ProtocolName`, the firewall VPP deadline).

**Behavior to change:**
- Every copy answers what the owner answers, because it is deleted and its caller calls the owner.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Received UPDATE bytes, operator commands (`update text`, `set`), config, VPP binary API, env.

### Transformation Path
1. Each fact is declared once in its domain package (`internal/core/bgp/...`, `internal/component/firewall`, `internal/component/vpp`, `internal/core/sysctl`, `internal/core/env`, `internal/component/config`).
2. Engine and plugins read BGP semantics through the raw attribute accessor; event JSON stays an output format and no plugin re-derives semantics from it.
3. The static check finds the shapes each owner declares it owns and refuses them anywhere else.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine and plugin | Raw attribute accessor, not JSON re-decoding | No |
| Plugin and VPP | `vpp.Connector.NewChannel` returns a bounded channel | No |

### Integration Points
- `bestpath.go`, `rpki.go`, `nlrisplit`, `wireu.ParseNLRIs`, `family.Family.LegacyNextHop`, `firewall.ProtocolName`, `vpp.Connector`, `internal/core/sysctl`, `env.Register`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | Callers move to the owner |
| No unintended coupling | No | Owners live in `internal/core` or the domain component, reachable by external plugins through `pkg/` where needed |
| No duplicated functionality | No | This spec deletes the duplicates |
| Zero-copy preserved where applicable | No | Accessors read the wire bytes in place |
| Registration over hardcoding, outbound | No | `fib-kernel` hardcoded dependencies replaced by the backend the config names |
| Registration over hardcoding, inbound | No | Private name tables replaced by the owner's table |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Each owned fact has one correct answer the copies should share | rows name the RFC sentence or the owner function | A copy is deliberately different (a rail that must differ) | per-fact RFC read during implementation | unvalidated |
| A-2 | A go/analysis pass can express each owned shape without matching text | the shapes are calls, field comparisons and typed literals | A shape needs text matching (banned as a population proxy) | prototype the analyzer on `PeerASN == LocalASN` and `LocalPref(100)` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving a hot-path copy to an owner adds a call or allocation | `./le perf` UPDATE throughput | Owner is inlineable and allocation-free; measure before and after |
| R-2 | Fixing the IBGP rule changes best-path on migration sessions | route-selection tests | Owned by RFC 7705; interop test proves the new answer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Best-path, RPKI state or withdrawal handling on the moved rails; a VPP call timing out where it used to hang |
| How is it reverted? | Single commit revert per fact |
| Who else touches this path? | RIB, reactor forward, rpki, vpp plugins, web editor |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| UPDATE from an RFC 7705 migration peer | → | IBGP owner used by `neighborAS` and the relation-tag step | `test/plugin/rib-migration-as-is-ibgp.ci` |
| UPDATE with AS_PATH ending in an AS_SET on the structured RPKI rail | → | RFC 6811 owner | `test/plugin/rpki-origin-as-set-none.ci` |
| MP_UNREACH labelled withdrawal with Compatibility 0x800000 | → | withdrawal framing owner | `test/encode/rfc8277-withdraw-compat.ci` |
| VPP that accepts a request and never answers | → | bounded `vpp.Connector.NewChannel` | `TestEveryVPPChannelHasReplyDeadline` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A new open-coded copy of an owned shape (reproduced: `PeerASN == LocalASN`, `attribute.LocalPref(100)`, `conn.NewAPIChannel` outside `internal/component/vpp`, a procfs sysctl write outside `internal/core/sysctl`, an `env.Register` of a key in a package that its reader does not import) | The static check refuses it, naming the owner to call |
| AC-2 | RFC 7705 migration session | RIB MED step and RFC 8195 relation-tag step both treat it as IBGP |
| AC-3 | Route whose final AS_PATH segment is AS_SET, on either RPKI rail | Origin AS is NONE on both, per RFC 6811 Section 2 |
| AC-4 | Labelled MP_UNREACH withdrawal with Compatibility 0x800000, on the RIB JSON rail, reactor prefix walk, `decodeMPNLRIs`, `nlritype.Retain` and the VPN decoder | Every path withdraws the right prefix; no framing error |
| AC-5 | Ze adds LOCAL_PREF on any rail | One declared value; no literal 100 remains |
| AC-6 | Every govpp channel any plugin obtains | Carries the reply deadline; a silent VPP yields an error, not a hang |
| AC-7 | `update text` EVPN `ethernet-ad`/`ethernet-segment`; CIDR family set; legacy next-hop families; IP protocol names; rp_filter words; mrt direction words; RADIUS rate forms | Each answered by one owner; every rail accepts what config accepts |
| AC-8 | `set <list> <key-leaf> <v>` from SSH, web terminal, CLI bar and session write-through | One resolver; all surfaces store the same tree, and the serialized form parses back |
| AC-9 | OSPF, IS-IS or LDP with `interface backend vpp` | Loads the VPP FIB writer, not `fib-kernel` |
| AC-10 | Each owned pair row (blackhole gate, web rollback, TLS rotation, IKE SK failure, `SendAnnounce` admission, audit success signal, ChangeCount and diff, geodns TTL bound, link-event queue, hub CR handling, firewall ARP literal, vpp ARP/ND verifier, policy-route protocol leaf, simultaneous announce and withdraw) | Both halves call one owner and give one answer |
| AC-11 | Each owned row's representative reproduction (Owned journal rows below) | No longer reproduces; the row is re-pointed to this spec and closed with its test name |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| `TestFactOwnerAnalyzerRefusesCopies` | `internal/le/lint/factowner/analyzer_test.go` | AC-1, one testdata case per owned shape |
| `TestNeighborASHonoursMigrationAS` | `internal/component/bgp/plugins/rib/bestpath_test.go` | AC-2 |
| `TestOriginASBothRailsRFC6811` | `internal/component/bgp/plugins/rpki/rpki_test.go` | AC-3 |
| `TestWithdrawFramingCompatibilityField` | `internal/core/bgp/nlri/nlrisplit/split_test.go` | AC-4 |
| `TestEveryVPPChannelHasReplyDeadline` | `internal/component/vpp/conn_test.go` | AC-6 |
| `TestSetListKeyOneResolver` | `internal/component/config/setpath_test.go` | AC-8 |

### Boundary Tests (numeric inputs)
| Field | Range | Last valid | First invalid |
|-------|-------|------------|---------------|
| geodns SOA `minimum` | 0..2147483647 | 2147483647 | 2147483648 |
| RFC 8277 Compatibility | 0x800000 or label | 0x800000 | N-A (any other value is a label) |

### Functional Tests
| Test | Validates |
|------|-----------|
| `test/plugin/rib-migration-as-is-ibgp.ci` | AC-2 |
| `test/plugin/rpki-origin-as-set-none.ci` | AC-3 |
| `test/encode/rfc8277-withdraw-compat.ci` | AC-4 |
| `test/plugin/evpn-update-text-all-route-types.ci` | AC-7 |

### Interop Tests (Scope: protocol)
| Scenario | Validates |
|----------|-----------|
| `bgp-migration-as-ibgp-frr` (new) | AC-2 against FRR |
| `bgp-vpn-labelled-withdraw-frr` (new) | AC-4 against FRR |

## Files to Modify
- `internal/component/bgp/plugins/rib/bestpath.go`, `internal/component/bgp/plugins/filter_community/` - IBGP owner
- `internal/component/bgp/plugins/rpki/rpki.go` - one origin derivation
- `internal/component/bgp/rib/commit.go`, `internal/component/bgp/reactor/peer_rib_routes.go`, `internal/component/bgp/reactor/forward_local_pref.go`, `internal/component/bgp/message/update_build.go` - one LOCAL_PREF
- `nlrisplit`, `wireu`, `internal/component/bgp/plugins/rib`, reactor `session_prefix`, VPN decoder - one withdrawal framing
- `internal/component/vpp/conn.go` and the four unbounded consumers - one bounded channel
- `internal/plugins/trafficusage/metrics.go`, policy-route, mrt, vrrp, RADIUS, iface `sourceValidationValues` - delete private tables
- `internal/plugins/ospf/register.go`, `internal/plugins/isis/register.go`, `internal/plugins/ldp/register.go` - FIB writer from backend
- `internal/core/env` registrations named in the env rows
- web editor, CLI bar and SSH `set` path resolution

## Files to Create
- `internal/le/lint/factowner/` - the analyzer, wired into the lint the commit path runs

### Documentation Update Checklist (BLOCKING)
- [ ] `docs/architecture/route-selection.md` - IBGP owner and migration AS
- [ ] `docs/guide/vpp.md` - reply deadline on every channel
- [ ] `docs/architecture/config/environment.md` - registration beside the reader
- [ ] `docs/contributing/go-conventions.md` - the fact-owner check
- [ ] `docs/architecture/ospf/ospf-4-component-config.md` - FIB writer chosen from the backend
- [ ] `docs/architecture/plugin/rib-storage-design.md` - CIDR family owner
- [ ] `docs/architecture/pool-architecture.md` - one forward pool release point
- [ ] `docs/architecture/traffic/traffic-usage.md` - protocol names from `firewall.ProtocolName`
- [ ] `docs/research/vpp-deployment-reference.md` - reply deadline on every channel

## Implementation Steps
1. Analyzer with the AC-1 shapes, red on today's tree; its report is the work list.
2. One commit per fact: declare the owner, move every caller, delete the copy, add its test (AC-2 to AC-9).
3. Pair rows: route both halves through the owner (AC-10).
4. Analyzer green on the tree; re-run every owned row's reproduction (AC-11).

## Owned journal rows

The 48 open rows this root cause owns. Each is re-pointed to this spec when it lands.

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-18 | `update text` EVPN keeps its own route-type list |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-17 | CIDR family set declared twice (`storage.IsCIDRFamily` and the filter rail) |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-15 | role `configuredPeerKeys` copies the peer walk, raw vs canonical address |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-13 | VRRP writes procfs sysctls bypassing `internal/core/sysctl` |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-10 | `dockerDefaultHead` AppArmor copy omits the signal rule |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-13 | `filter_community` relation tag open-codes the IBGP rule |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-10-01 | RIB `neighborAS` open-codes the IBGP rule |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-09-04 | LOCAL_PREF 100 written as a literal by six producers |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-09-04 | `peer` selector declared twice for one merged container |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-08-29 | `extractNextHopFromMPReach` second MP_REACH next-hop reader |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-08-22 | `protoName` fourth IP-protocol name table |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-08-10 | forward pool supersede paths open-code `releaseItem` |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-09-06 | RADIUS `parseFilterRate` and CoA `extractRate` accept different forms |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-09-14 | four rails open-code `Family.LegacyNextHop` |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-09-14 | rp_filter word set declared in two packages |
| `helper-bypassed-by-an-open-coded-copy.md` | 2026-09-14 | mrt `direction` parsed with its own literals |
| `plugin-list-hardcoded.md` | 2026-09-07 | OSPF and IS-IS `Dependencies` name `fib-kernel` whatever the backend |
| `plugin-list-hardcoded.md` | 2026-10-20 | LDP `Dependencies` names `fib-kernel` |
| `env-var-double-registration.md` | 2026-04-18 | `ze.config.dir` registered in two files |
| `env-var-double-registration.md` | 2026-09-04 | `ze.plugin.hub.*`, `ze.plugin.ca.pem` registered in the SDK, read elsewhere |
| `same-bytes-decoded-once-per-accessor.md` | 2026-08-14 | `Event.GetPeerGroup` re-unmarshals the peer JSON per accessor |
| `parser-refuses-a-form-its-producer-emits.md` | 2026-10-13 | session `writeThroughSet` writes a list-key line `walkAndSet` refuses |
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-13 | web and CLI bar `set` list-key resolution differs from SSH `ensureListEntry` |
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-11 | IKE half-open initiator arms kill the SA on SK decrypt failure |
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-09 | `Session.SendAnnounce` bypasses `writeUpdateGated` next-hop admission |
| `guard-added-to-one-half-of-a-pair.md` | 2026-08-11 | govpp reply deadline set by the firewall only |
| `guard-added-to-one-half-of-a-pair.md` | 2026-08-23 | IKE VPP dataplane: fifth unbounded govpp consumer |
| `guard-added-to-one-half-of-a-pair.md` | 2026-08-18 | iface resolver `onLinkEvent` keeps the old drop shape |
| `guard-added-to-one-half-of-a-pair.md` | 2026-08-22 | web `ChangeCount` and `/config/diff` body from two producers |
| `guard-added-to-one-half-of-a-pair.md` | 2026-08-31 | RPKI origin AS: structured rail ignores the final segment type |
| `guard-added-to-one-half-of-a-pair.md` | 2026-08-30 | RPKI RFC 6811 origin derivation reached one rail only |
| `guard-added-to-one-half-of-a-pair.md` | 2026-09-27 | VPP firewall verifier refuses ARP and ND matches nft lowers |
| `guard-added-to-one-half-of-a-pair.md` | 2026-09-28 | firewall family arp: literal address match unguarded |
| `green-that-could-not-have-been-red.md` | 2026-09-03 | `system command list` has two producers; a plugin reads the stage-4 copy |
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-11 | web commit audits on output text, discard on a success signal |
| `guard-added-to-one-half-of-a-pair.md` | 2026-10-15 | IKE `newVPPBackend` channel without `SetReplyTimeout` |
| `guard-enumerates-instead-of-subtracting.md` | 2026-10-21 | `mpReachLabeledNLRI` lists the labelled families again beside `nlrisplit` |
| `plugin-list-hardcoded.md` | 2026-10-28 | completion generators hand-write root verb arms beside `Subs` |

## Rows found already fixed

Another agent closes these in the journal; listed here so they are not re-owned.

| Class file | Date | Surface (short) | Evidence (2026-10-23) |
|------------|------|-----------------|-----------------------|
| `env-var-double-registration.md` | 2026-09-06 | `ze.storage.blob` registered only in `cmd/ze`, read by `internal/core/resolve` | No Go file under `internal/`, `cmd/` or `pkg/` contains the string `"ze.storage.blob"`; `resolve.StorageFor` reads `ze.config.dir` |
| `env-key-read-but-never-registered.md` | 2026-09-07 | `ze.storage.blob` read but never registered under feature tags | Same evidence: the key is no longer read |

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Producers read for at least three owned rows (`neighborAS`, `originASFromParsed` and `rpkiOriginASFromASPath`, LOCAL_PREF literals, `protoName`, govpp `NewChannel` sites)

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated; AC-1 refuses a reproduced copy of each owned shape
- [ ] Every owned row's reproduction no longer reproduces
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass: `./le test unit` over the changed packages, `./le test functional plugin`, `./le test functional encode`, `./le test integration` for the two new interop scenarios, and `./le verify current fast`; the full gate runs once after commit A
- [ ] `./le perf` UPDATE throughput pasted before and after the LOCAL_PREF and withdrawal-framing moves
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows re-pointed
- [ ] **Commit B:** `remove` of this spec only
