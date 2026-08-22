# Spec: fixit-flowspec-protocol-name-drift

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 5/5 |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-22 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

A FlowSpec NLRI carrying an IP protocol outside the five values `protoName` knows
(1, 6, 17, 47, 58) is translated into a `MatchProtocol` holding the protocol
NUMBER as decimal text. The nft backend resolves that string through
`firewall.ProtocolNumber`, which knows names only, so lowering fails with
`unknown protocol "132"`.

The failure does not stop at the offending rule. `applyTable` returns the error
to `Apply` before `Flush`, so one unlowerable FlowSpec term aborts the whole
firewall reconcile: the tables of every other owner (the firewall engine, copp,
policy routes, ddos-local) are not applied either, and the kernel keeps the
previous ruleset. A peer can therefore freeze this router's firewall
reconciliation by announcing one legal FlowSpec route for SCTP.

Two defects share the root cause and are in scope:

1. `protoName` is a private, incomplete copy of a table the repository declares
   canonical, and its default branch invents a spelling no consumer accepts.
   `ianaProtocolNumbers` already knows five names `protoName` does not: `sctp`,
   `esp`, `ah`, `ospf`, `vrrp`.
2. `componentToMatch` reads `vals[0]` and discards every further value, so a
   FlowSpec component listing several protocols silently enforces only the first.

A third instance of the same class is in scope because it reaches the same
lowering failure from config: the policy-route `protocol` leaf is typed `string`
with no validator, so an operator can commit a protocol spelling the backend
will refuse hours later at reconcile time.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/wire/nlri-flowspec.md` - FlowSpec NLRI component encoding, what type 3 carries
  → Constraint: a type 3 component is a numeric operator list over an 8-bit protocol field; every value 0-255 is legal on the wire, so the translator cannot assume a small set
- [ ] `docs/architecture/policyroute/policy-routing.md` - how policy routes reach the same firewall lowering
  → Constraint: policy routes emit `firewall.Term` values through the same registry, so a match type they can express must be one the backends accept
- [ ] `ai/rules/architecture.md` - grep before proposing, extend rather than duplicate
  → Decision: the fix extends `firewall.ProtocolNumber`'s table as the single source, and deletes the private copy rather than completing it in place

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc8955.md` - FlowSpec type 3 (IP Protocol) component
  → Constraint: [RFC8955-4.2.2.3-1] SHOULD encode type 3 values as a single octet; the value space is the full 8-bit IP protocol range, so refusing 132 is an enforcement gap in ze, not a wire-format question
- [ ] `rfc/short/rfc8956.md` - IPv6 FlowSpec, same component semantics
  → Constraint: the IPv6 dialect reuses the numeric component encoding, so the fix must be family-independent

**Key insights:**
- `internal/component/firewall/protocol.go` states in its own comment that the table is the single source of truth and that backends must not keep private copies. Three private copies exist today.
- The consequence of a lowering error is repo-wide, not rule-local: no partial apply, no kernel change, all owners affected.
- `TestProtoNameUnknown` currently asserts the defective behavior and must change with the fix.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/protocol.go` - declares `ianaProtocolNumbers` (10 names) and `ProtocolNumber(string) (uint8, bool)`; the comment names it the single source of truth for every backend
- [ ] `internal/component/firewall/model.go` - `MatchProtocol` carries one `Protocol string`, documented as an L4 protocol NAME
- [ ] `internal/plugins/flowspec-firewall/translate.go` - `protoName` maps 1, 6, 17, 47, 58 and returns decimal digits otherwise; `componentToMatch` builds `MatchProtocol` from `vals[0]` only
- [ ] `internal/plugins/flowspec-firewall/engine.go` - `handleUpdate`, `handleFlowSpecAdd`, `applyRules`; an apply failure produces one `Warn` log line naming flowspec
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerProtoMatch` rejects any string `ProtocolNumber` does not know
- [ ] `internal/plugins/firewall/nft/backend_linux.go` - `Apply` builds every table then flushes once; `applyTable` returns the lowering error before `Flush` runs
- [ ] `internal/component/firewall/registry.go` - `ApplyAll` merges every owner's tables under one lock and hands them to the backend as one set
- [ ] `internal/plugins/firewall/vpp/translate.go` - a second private protocol table (`protoNumbers`) with the same 10 names
- [ ] `internal/plugins/ddos/local/match.go` - a third private table (4 entries) whose unknown branch emits no match rather than a bad string
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `protocol-name` is an enumeration of exactly the 10 canonical names
- [ ] `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - `leaf protocol` is `type string` with no constraint and no validator
- [ ] `internal/plugins/flowspec-firewall/translate_test.go` - `TestProtoNameUnknown` asserts `protoName(99)` returns "99", pinning the defect

**Behavior to preserve:**
- `MatchProtocol` keeps carrying a name, not a number: the YANG enum, the VPP backend's name-keyed table, the CLI `show` renderer and the web firewall page all read it as a name.
- `ApplyAll` keeps merging every owner's tables into one atomic flush; the fix must not make FlowSpec apply separately.
- A FlowSpec component that names no value keeps producing no match, as `componentToMatch` does today.
- The existing plugin `Warn` line on apply failure stays; the fix adds signal, it does not replace it.

**Behavior to change:**
- A FlowSpec protocol number that maps to a canonical name is translated to that name (all ten, not five).
- A FlowSpec protocol number with no canonical name is refused at translation, with the rule rejected and named, instead of producing a string the backend cannot lower.
- A FlowSpec component listing several protocol values produces a match per value, or is refused; it no longer silently enforces the first alone.
- The policy-route `protocol` leaf is refused at commit when it is not a canonical name.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A FlowSpec UPDATE arrives from a BGP peer (AFI 1, SAFI 133 or 134) and is delivered to the flowspec-firewall plugin as parsed JSON over the plugin socket.
- Format at entry: JSON object per NLRI, component keys to operator/value lists.

### Transformation Path
1. Reactor parses the UPDATE and emits per-family JSON; the plugin receives it in `handleUpdate` in `internal/plugins/flowspec-firewall/engine.go`
2. `handleFlowSpecAdd` calls `parseNLRIJSON` in `internal/plugins/flowspec-firewall/translate.go`, producing `flowspec.FlowComponent` values
3. `translateFlowSpec` walks the components; `componentToMatch` turns type 3 into `MatchProtocol` via `protoName` -- the defect is here
4. `applyRules` registers the resulting tables with the firewall registry under the owner name `flowspec`
5. `ApplyAll` in `internal/component/firewall/registry.go` merges every owner's tables and calls the active backend
6. `Apply` in `internal/plugins/firewall/nft/backend_linux.go` stages tables, chains and rules; `lowerTerm` and `lowerMatch` reach `lowerProtoMatch` in `internal/plugins/firewall/nft/lower_linux.go`
7. On success one `Flush` commits every owner's ruleset; on the lowering error `Apply` returns first and nothing reaches the kernel

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Reactor ↔ flowspec-firewall plugin | parsed UPDATE JSON over the plugin socket | No |
| Plugin ↔ firewall registry | `RegisterTables` with owner `flowspec` | No |
| Registry ↔ nft backend | `[]firewall.Table` handed to `Apply` under `reconcileMu` | No |
| Backend ↔ kernel | one netlink `Flush` per reconcile, all owners together | No |

### Integration Points
- `firewall.ProtocolNumber` - the canonical table the fix extends and that every backend already calls
- `firewall.MatchProtocol` - the match type whose contract (a name) the fix keeps
- `internal/plugins/policyroute/config.go` - the second producer of `MatchProtocol` from operator config

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A lowering error aborts the entire reconcile before `Flush`, so no owner's tables reach the kernel | `Apply` and `applyTable` in `internal/plugins/firewall/nft/backend_linux.go` return the error before the single `Flush` | The blast radius is one rule, not the whole ruleset, and the severity of this spec drops | A unit test that applies one good table plus one unlowerable table and asserts the good table is absent from the fake conn's flushed set | confirmed |
| A-2 | Every protocol number ze must enforce has a canonical name in `ianaProtocolNumbers` | `internal/component/firewall/yang/ze-firewall-conf.yang` `protocol-name` enumerates exactly those ten | A FlowSpec rule for a legal protocol outside the ten is still unenforceable, and the residual case needs the numeric path in `ProtocolNumber` after all | Decide the residual case explicitly in Phase 2; the AC-4 rejection test pins whichever answer is chosen | confirmed |
| A-3 | No consumer of `MatchProtocol` accepts a numeric spelling today | `lowerProtoMatch`, `classifyMaskMatch`, `buildDNATMapping`, `applyMatch` all resolve through a name table | A numeric-accepting fix would be simpler than the name fix | Grep every `MatchProtocol` consumer during Phase 1 and record the list in the spec | confirmed |
| A-4 | The nft `Warn` line plus the apply-duration error bucket are the only operator signal for this failure today | `applyRules` in `internal/plugins/flowspec-firewall/engine.go`, `observeApply` in the firewall metrics file | The observability AC is already met and can be dropped | Read the firewall audit path; if `AuditTables` reports drift for a never-applied ruleset, note it and narrow AC-5 | confirmed |

**A-1 confirmed.** `Apply` in `internal/plugins/firewall/nft/backend_linux.go` loops `applyTable`
and returns the first error before it calls `conn.Flush`. `applyChain` returns a term error, and
`lowerProtoMatch` in `internal/plugins/firewall/nft/lower_linux.go` refuses a name outside the
canonical table. Proven by running `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers` against
a `protocolMatches` mutated back to rendering digits: `ApplyAll` returned `unknown protocol "253"`
and no owner's table reached the flush.

**A-2 confirmed, and the residual case is decided: REFUSE.** A protocol number with no canonical
name is refused at translation by `protocolMatches` in
`internal/plugins/flowspec-firewall/translate.go`, counted as `unknown-protocol` on
`ze_flowspec_rules_refused_total` and logged. `ProtocolNumber` gains no numeric path, so
`MatchProtocol` never carries digits.

**A-3 confirmed. Every non-test file naming `MatchProtocol`, and what it does with the name.**
Four resolve it through `firewall.ProtocolNumber`: `lowerProtoMatch` in
`internal/plugins/firewall/nft/lower_linux.go`, `classifyMaskMatch` in
`internal/plugins/firewall/vpp/classify_linux.go`, `buildDNATMapping` in
`internal/plugins/firewall/vpp/nat_linux.go`, and `applyMatch` in
`internal/plugins/firewall/vpp/translate.go`. Two produce the name from `firewall.ProtocolName`:
`protocolMatches` in `internal/plugins/flowspec-firewall/translate.go` and `buildDropTerm` in
`internal/plugins/ddos/local/match.go`. Three produce it from validated config:
`internal/component/firewall/config.go`, `internal/plugins/policyroute/translate.go` and
`internal/plugins/copp/translate.go`. Two render it: `internal/component/firewall/cmd/show.go` and
`internal/component/web/page_firewall.go`. `internal/plugins/firewall/vpp/verify.go` type-switches
without resolving. No consumer accepts a numeric spelling.

**A-4 confirmed, and the observability AC is NOT already met, so it stays.** `AuditTables` in
`internal/component/firewall/audit.go` compares `LastApplied` against the kernel, so a reconcile
that never ran leaves the two agreeing on the previous ruleset and reports no drift. The signal is
the one `internal/plugins/flowspec-firewall/metrics.go` adds: `ze_flowspec_rules_refused_total` by
reason, plus the log line.

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Rejecting an untranslatable FlowSpec rule turns a silent enforcement gap into a visible one, and an operator who was unknowingly unprotected now sees a refused rule | The new log line appears on a router that was previously quiet | This is the intended outcome; the rule was never enforced, and the message names the protocol number so the operator can act |
| R-2 | Emitting one match per protocol value changes the term count for a multi-value FlowSpec component, which may surprise a counter-reading operator | Term counts change in `show firewall` output after upgrade | Document in the release note; the alternative (silently enforcing one of several protocols) is the defect |
| R-3 | Tightening the policy-route YANG leaf rejects a config that commits today | A previously valid config fails to load after upgrade | Only spellings the backend already refuses become invalid; a config that commits and works is unaffected. Name this in the doc update |
| R-4 | Deleting the VPP and ddos-local private tables changes VPP behavior for a protocol whose VPP enum value differs from the IANA number | VPP classify tests fail | Keep the VPP name-to-enum mapping; only the name set is shared. The IANA number is not the VPP enum and the fix must not conflate them |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | The firewall reconcile path. A regression here leaves the kernel ruleset stale for every owner, which is the defect this spec fixes, so the fix must be proven by a test that asserts the good table still lands |
| How is it reverted? | Single commit revert; no config migration, no on-disk state. The policy-route YANG tightening is the only operator-visible surface and it only refuses what already failed later |
| Who else touches this path? | `plan/spec-fixit-firewall-concurrency-deadlock.md` (the same `reconcileMu`), `plan/spec-ddos-direction-allowlist-deferred-flowspec-withdraw.md` (the same plugin's withdraw path) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| FlowSpec NLRI with protocol 132 arrives from a peer | → | `componentToMatch` in `internal/plugins/flowspec-firewall/translate.go` | `TestComponentToMatchSCTPName` |
| The resulting term is lowered by the nft backend | → | `lowerProtoMatch` in `internal/plugins/firewall/nft/lower_linux.go` | `TestLowerProtoMatchAcceptsEveryCanonicalName` |
| A peer announces a FlowSpec rule ze cannot translate | → | `applyRules` in `internal/plugins/flowspec-firewall/engine.go` | `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers` |
| An operator commits a policy route with a non-canonical protocol | → | the policy-route protocol leaf validator | `TestPolicyRouteProtocolRejectsUnknownName` |
| A peer announces a FlowSpec SCTP rule against a running daemon | → | the whole chain to the kernel ruleset | `test/plugin/flowspec-fw-protocol-sctp.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A FlowSpec component naming IP protocol 132 (also 50, 51, 89, 112) | The rule translates to a protocol match carrying the canonical name and lowers to a kernel rule without error |
| AC-2 | A FlowSpec component naming a protocol number with no canonical name | The rule is refused at translation, the log line names the protocol number and the rule key, and no `MatchProtocol` carrying digits is ever produced |
| AC-3 | A FlowSpec component listing several protocol values | Every listed value is enforced, or the rule is refused; the first value is never enforced alone and silently |
| AC-4 | A FlowSpec rule that cannot be translated arrives while other owners have valid tables | Every other owner's ruleset is applied to the kernel; the untranslatable rule alone is absent |
| AC-5 | A policy route configured with a protocol spelling outside the canonical names | The commit is refused with an error naming the leaf and the accepted values |
| AC-6 | The repository is grepped for protocol-name tables | Exactly one table maps protocol names to IANA numbers; the flowspec, VPP and ddos-local copies are gone or derived from it |
| AC-7 | Any canonical name is passed to every backend that resolves a protocol | Each resolves it without error; no backend knows a name the others do not |

### Evidence, with the mutation that reddens each test (2026-08-22)

A test is evidence only if it fails when the behavior it covers is broken, so every row
below names the production edit that reddened it. Each edit was applied, run, and reverted;
the reverts are byte-identical (`shasum` before and after).

| AC | Producer | Test, green | Mutation, red |
|----|----------|-------------|---------------|
| AC-1 | `protocolMatches` (`internal/plugins/flowspec-firewall/translate.go`) | `bgp-flowspec-sctp-gobgp` interop: GoBGP originates `match destination 10.99.5.0/24 protocol ==sctp then discard`, ze's kernel ruleset carries `ip daddr 10.99.5.0/24 ... meta l4proto 132 drop` | the pre-fix five-name private table: `FAIL: ze installed no flowspec table`, ze logging `firewallnft: table "flowspec": chain "flowspec-fwd": term "fs-2b910eed8e5c4c60": unknown protocol "132"` |
| AC-2 | `protocolMatches` refuses through `firewall.ProtocolName`; `refusalReason` (`internal/plugins/flowspec-firewall/metrics.go`) maps it to `unknown-protocol` | `TestComponentToMatchRejectsUnnamedProtocol`, `TestComponentToMatchEveryWireValue` (all 256 values), `TestUnknownProtocolRouteIsCountedAndNamed` | rendering an unnamed number as digits: `Expected error with "flowspec: IP protocol has no canonical firewall name" in chain but got nil`. Deleting the `errUnknownProtocol` case from `refusalReason`: `the refusal must reach the counter: expected 1, actual 0` |
| AC-3 | `protocolMatches` walks every value; `translateFlowSpec` expands one term per protocol | `TestComponentToMatchMultipleProtocolValues`, `TestTranslateFlowSpecMultipleProtocolsBecomeSeparateTerms` | iterating `vals[:1]`, the pre-fix `vals[0]` read: `expected [tcp udp sctp], actual [tcp]` |
| AC-4 | `handleFlowSpecAdd` (`internal/plugins/flowspec-firewall/engine.go`) drops the route instead of registering it | `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers`, `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | the pre-fix table: the `.ci` times out, the enforceable route that arrives SECOND never reaches the kernel |
| AC-5 | `parsePolicyMatch` (`internal/plugins/policyroute/config.go`) resolves through `firewall.ProtocolNumber`; the leaf is `type fw:protocol-name` (`internal/plugins/policyroute/yang/ze-policyroute-conf.yang`) | `TestPolicyRouteProtocolRejectsUnknownName`, `TestPolicyRouteProtocolAcceptsEveryCanonicalName`, `TestPolicyRouteEmptyProtocolStaysOptional` | removing the `ProtocolNumber` guard: all five spellings (`TCP`, `132`, `igmp`, `bogus`, `tcp6`) are accepted |

**The observability row A-4 came out against is now closed.** `refusedReasonUnknownProtocol`
had no producer any test drove: `refusalReason` mapped to it and nothing asserted the mapping,
and no test proved the series ever reached an exposition. `TestUnknownProtocolRouteIsCountedAndNamed`
covers the first and `test/plugin/flowspec-metrics-registered.ci` the second. The `.ci` is the
one that can deny the wiring: every Go test of this series calls `bindMetrics` directly, so all
of them stay green while the daemon publishes nothing. Deleting `ConfigureMetrics: bindMetrics`
from `register.go` reddens it while the refusal WARN still appears in the log, which is exactly
the hole it exists for.

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Receives a FlowSpec rule matching SCTP from an upstream scrubbing provider | wire → flowspec plugin → translate → firewall registry → nft → kernel | `test/plugin/flowspec-fw-protocol-sctp.ci` |
| 2 | Receives a FlowSpec rule ze cannot enforce, while local firewall terms are configured | wire → translate (refused) → registry → nft applies the rest | `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers` |
| 3 | Configures a policy route with a mistyped protocol | config tree → YANG validation → refused at commit | `TestPolicyRouteProtocolRejectsUnknownName` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestComponentToMatchSCTPName` | `internal/plugins/flowspec-firewall/translate_test.go` | protocol 132 becomes the canonical name, not "132" | green |
| `TestComponentToMatchEveryCanonicalNumber` | `internal/plugins/flowspec-firewall/translate_test.go` | all ten canonical numbers round-trip to their names | green |
| `TestComponentToMatchRejectsUnnamedProtocol` | `internal/plugins/flowspec-firewall/translate_test.go` | replaces `TestProtoNameUnknown`: an unnamed number is refused, never rendered as digits | green |
| `TestComponentToMatchMultipleProtocolValues` | `internal/plugins/flowspec-firewall/translate_test.go` | every listed value is enforced or the rule is refused | green |
| `TestTranslateFlowSpecMultipleProtocolsBecomeSeparateTerms` | `internal/plugins/flowspec-firewall/translate_test.go` | a two-protocol component yields two terms, so the OR semantics survive term construction | green |
| `TestUnknownProtocolRouteIsCountedAndNamed` | `internal/plugins/flowspec-firewall/value_completeness_test.go` | the refusal moves `ze_flowspec_rules_refused_total` with reason `unknown-protocol`, and the WARN names the protocol number and the rule key | green |
| `TestLowerProtoMatchAcceptsEveryCanonicalName` | `internal/plugins/firewall/nft/lower_linux_test.go` | every name in the canonical table lowers | green (linux) |
| `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers` | `internal/plugins/flowspec-firewall/blast_radius_test.go` | validates A-1: a good table still reaches the fake conn's flush when a sibling term is unlowerable | green |
| `TestPolicyRouteProtocolRejectsUnknownName` | `internal/plugins/policyroute/config_test.go` | a non-canonical protocol is refused before lowering | green |
| `TestPolicyRouteProtocolAcceptsEveryCanonicalName` | `internal/plugins/policyroute/config_test.go` | the validator is not a second, narrower protocol table | green |
| `TestProtocolTableIsSingleSource` | `internal/component/firewall/protocol_test.go` | the flowspec and VPP name sets are derived from `ianaProtocolNumbers`, not independent literals | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| FlowSpec type 3 protocol value | 0-255 | 255 | N/A (0 is legal on the wire) | N/A (the field is one octet) |
| Canonical protocol numbers | 1-132 | 132 (sctp) | 0 (no canonical name) | 133 (no canonical name) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `flowspec-fw-protocol-sctp` | `test/plugin/flowspec-fw-protocol-sctp.ci` | a peer announces a FlowSpec rule matching SCTP and the kernel ruleset carries the matching rule | green (QEMU) |
| `flowspec-fw-untranslatable-keeps-others` | `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | an untranslatable FlowSpec rule does not stop the local firewall terms reaching the kernel | green (QEMU) |
| `flowspec-metrics-registered` | `test/plugin/flowspec-metrics-registered.ci` | an operator scraping Prometheus sees `ze_flowspec_rules_refused_total{reason="unknown-protocol"}` after a peer announces a route ze cannot enforce | green |

Both kernel tests carry `option=needs-linux:caps=net-admin` and are named in
`ZE_NETNS_PLUGIN_TESTS` (`mk/test-integration.mk`). They SKIP in a plain
`make ze-functional-plugin-test` on a host without the capability, so they were
run through `make ze-qemu-debug` here. `flowspec-metrics-registered` needs no
kernel: the route is refused at translation, so no table is ever registered.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-flowspec-sctp-gobgp` | `test/interop/scenarios/bgp-flowspec-sctp-gobgp/` | GoBGP | a real peer announcing a FlowSpec SCTP rule leads to an installed kernel rule on ze; reverting the translator change leaves the rule absent | green |

**The peer is GoBGP, not the FRR this row first named.** FRR 10.3.1 RECEIVES FlowSpec and
cannot originate it: `address-family ipv4 flowspec` offers `neighbor ... activate` and policy
attachment and no route-origination command, and `router bgp` names flowspec only inside
`bgp default`. Both command sets were listed in the scenario runner's own FRR image
(`quay.io/frrouting/frr:10.3.1`) on 2026-08-22. `bgp-flowspec-frr` and `bgp-flowspec-gobgp`
cover the SEND direction and name no protocol outside tcp, so this scenario is the first to
cover the direction AC-1 is about.

The scenario asserts the KERNEL, which is where the answer exists. `test/interop/Dockerfile.ze`
gains `nftables` for that read: `show firewall` renders the registry's own record, and the nft
readback skips every table outside the `ze_` ownership prefix
(`internal/plugins/firewall/nft/readback_linux.go`).

## Files to Modify
- `internal/plugins/flowspec-firewall/translate.go` - derive protocol names from the canonical table; refuse an unnamed number; stop discarding values after the first
- `internal/plugins/flowspec-firewall/engine.go` - carry the translation refusal into a log line that names the rule and the protocol
- `internal/component/firewall/protocol.go` - expose whatever the other packages need (name for number) so no private copy is required
- `internal/plugins/firewall/vpp/translate.go` - drop the private name table, keep only the name-to-VPP-enum mapping
- `internal/plugins/ddos/local/match.go` - drop the private name table
- `internal/plugins/policyroute/yang/ze-policyroute-conf.yang` - type the protocol leaf against the canonical name set
- `internal/plugins/policyroute/config.go` - reject a non-canonical protocol at parse time
- `internal/plugins/flowspec-firewall/translate_test.go` - `TestProtoNameUnknown` asserts the defect and must be replaced
- `docs/guide/firewall.md` - document the accepted protocol names
- `docs/guide/policy-routing.md` - same list, and the tightened leaf

## Files to Create
- `test/plugin/flowspec-fw-protocol-sctp.ci` - functional proof for the canonical-name path
- `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` - functional proof for the blast-radius fix
- `test/plugin/flowspec-metrics-registered.ci` - the refusal counter reaches the Prometheus exposition of a running daemon
- `test/interop/scenarios/bgp-flowspec-sctp-gobgp/` - `gobgp.toml`, `ze.conf` and `check.py`: a real peer announces the SCTP rule and ze installs it
- `test/interop/Dockerfile.ze` - add `nftables`, so a scenario can read what reached the kernel

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`: the protocol leaf is retyped, no new node |
| YANG validation constraints | Yes | the leaf takes the canonical name set as its native constraint rather than a free string |
| YANG custom validators | N-A | a native enumeration covers it; no `ze:validate` needed |
| CLI commands/flags | N-A | no new command; the existing `show firewall` renders the match unchanged |
| CLI grammar (keyword before value) | N-A | no grammar change |
| Editor autocomplete | Yes | automatic once the leaf carries an enumeration instead of a string |
| Functional test for new RPC/API | Yes | `test/plugin/flowspec-fw-protocol-sctp.ci` |
| Pipe completeness | N-A | no new command output |
| Env var registration | N-A | no `environment/` leaf |
| Doctor check for runtime dependencies | N-A | no new file path, socket, port, module or binary; the failure is a rule-content error, not a dependency |
| Prometheus counters/metrics | Yes | `ze_flowspec_rules_refused_total{reason}` in `internal/plugins/flowspec-firewall/metrics.go`, bound through `Registration.ConfigureMetrics` in `register.go`. The reason set is closed on purpose: the data that produced the refusal comes from a peer. Documented in `docs/guide/firewall.md`, exposition proven by `test/plugin/flowspec-metrics-registered.ci` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no new family, capability or attribute; FlowSpec 133/134 already registered |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | behavior fix; `docs/features.md` firewall row already claims protocol matching |
| 2 | Config syntax changed? | Yes | `docs/guide/policy-routing.md` protocol row must list the accepted names |
| 3 | CLI command added/changed? | No | no command surface changes |
| 4 | API/RPC added/changed? | No | no API change |
| 5 | Plugin added/changed? | Yes | `docs/guide/flowspec-protected-router.md`: state which protocols are enforceable and what happens to a rule that is not |
| 6 | Has a user guide page? | Yes | `docs/guide/firewall.md` protocol match row |
| 7 | Wire format changed? | No | decoding is unchanged; only enforcement changes |
| 8 | Plugin SDK/protocol changed? | No | no SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | No | no MUST row changes; RFC 8955 type 3 decoding already conforms, the gap is local enforcement |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` if the FlowSpec injection fixture gains a capability |
| 11 | Affects daemon comparison? | No | no comparison claim changes |
| 12 | Internal architecture changed? | Yes | `docs/architecture/policyroute/policy-routing.md` if the protocol leaf typing is described there |
| 12a | Design docs declared by the changed files | No | Three are unaffected and stay as they are. `docs/architecture/core-design.md` describes the firewall data model, and `MatchProtocol` keeps carrying a name. `docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md` states no protocol table, so deleting the ddos-local copy changes nothing it claims. `docs/architecture/firewall/fw-6-firewall-vpp.md` names the ACL fields VPP covers, not the name set, so the VPP name-to-enum mapping it describes is unchanged |
| 13 | Route metadata keys added/changed? | No | no metadata key |
| 14 | Prometheus counters added/changed? | Yes | the new refusal counter belongs in the firewall telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no registration change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/guide/firewall.md` anchors `nft/lower_linux.go`; `docs/features.md` anchors `firewall/model.go` and `config.go`; `docs/architecture/policyroute/policy-routing.md` anchors `policyroute/translate.go` |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | verify the protocol examples in `docs/guide/firewall.md`, `docs/guide/policy-routing.md` and the FlowSpec API syntax pages against the tightened set |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the chain and the blast radius before changing behavior
   - Tests: `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers`, `TestLowerProtoMatchAcceptsEveryCanonicalName`
   - Files: `internal/plugins/flowspec-firewall/blast_radius_test.go`, `internal/plugins/firewall/nft/lower_linux_test.go`
   - Verify: the blast-radius test fails today, validating A-1 or refuting it before any fix is designed around it
2. **Phase: one protocol table** -- make the canonical table serve every producer
   - Tests: `TestProtocolTableIsSingleSource`, `TestComponentToMatchEveryCanonicalNumber`
   - Files: `internal/component/firewall/protocol.go`, `internal/plugins/flowspec-firewall/translate.go`, `internal/plugins/firewall/vpp/translate.go`, `internal/plugins/ddos/local/match.go`
   - Verify: no package holds an independent name literal; every canonical name lowers in every backend
3. **Phase: refuse what cannot be enforced** -- unnamed numbers and multi-value components
   - Tests: `TestComponentToMatchRejectsUnnamedProtocol`, `TestComponentToMatchMultipleProtocolValues`, replacing `TestProtoNameUnknown`
   - Files: `internal/plugins/flowspec-firewall/translate.go`, `internal/plugins/flowspec-firewall/engine.go`
   - Verify: a refused rule is named in the log and counted; no digits reach `MatchProtocol`
4. **Phase: close the config-side instance** -- policy-route protocol leaf
   - Tests: `TestPolicyRouteProtocolRejectsUnknownName`
   - Files: `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`, `internal/plugins/policyroute/config.go`
   - Verify: the commit is refused with a message naming the accepted values
5. **Phase: functional and interop proof**
   - Tests: `test/plugin/flowspec-fw-protocol-sctp.ci`, `test/plugin/flowspec-fw-untranslatable-keeps-others.ci`, `test/plugin/flowspec-metrics-registered.ci`, the GoBGP interop scenario
   - Files: the three `.ci` files, `test/interop/scenarios/bgp-flowspec-sctp-gobgp/`, `test/interop/Dockerfile.ze`
   - Verify: reverting the Phase 2 change makes the SCTP test fail, so the test is not vacuous

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file plus symbol |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A refused rule names the protocol number and the rule key; a multi-value component enforces every value or none |
| Naming | The canonical names are spelled identically in YANG, the table, the docs and the CLI renderer |
| Data flow | Translation refuses; lowering never sees a spelling it cannot resolve |
| Rule: `ai/rules/evidence.md` | The blast-radius claim (A-1) is proven by a test, not by reading |
| Registration over hardcoding | The protocol table stays a single shared source; no backend regains a private copy |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| One protocol name table | grep the repository for protocol-name literals; only `internal/component/firewall/protocol.go` holds them |
| No digits in `MatchProtocol` | `TestComponentToMatchRejectsUnnamedProtocol` passes and `TestProtoNameUnknown` is gone |
| Blast radius closed | `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers` passes |
| Functional proof | `make ze-qemu-debug RUN='ze-test bgp plugin -p 1 flowspec-fw-protocol-sctp flowspec-fw-untranslatable-keeps-others'` is green; both carry `option=needs-linux:caps=net-admin` and SKIP in a plain `make ze-functional-plugin-test`. `flowspec-metrics-registered` runs there and is green |
| Interop proof | `make ze-interop-test INTEROP_SCENARIO=bgp-flowspec-sctp-gobgp` is green, and red against the pre-fix translator |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The protocol value comes from a remote peer; every value 0-255 must produce either an enforced rule or a clean refusal, never a crash and never a stalled reconcile |
| Resource exhaustion | A multi-value component must not let one NLRI expand into an unbounded term count; cap it or refuse it |
| Fail closed | A rule ze cannot enforce must not be reported as applied; the operator signal is part of the fix |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A-1 refuted by the Phase 1 test | Re-scope: the defect is rule-local, keep phases 2-4, drop the blast-radius AC |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Keep `MatchProtocol` carrying a name | Make `ProtocolNumber` parse decimal strings; change the field to a `uint8` | The name is the contract every consumer already implements: the YANG enum, the VPP enum mapping, the CLI and web renderers. A numeric spelling would create two spellings of one match and would still leave VPP refusing it |
| Refuse an unenforceable FlowSpec rule loudly | Skip the match silently, as ddos-local does | A FlowSpec rule exists to drop or rate-limit traffic; enforcing it without the protocol condition is wider than the peer asked for, and skipping it silently leaves the operator believing they are protected |
| Fix the policy-route leaf in the same spec | A separate spec for the config surface | It is the same defect reached from a second entry point, and `ai/rules/completion.md` puts the sibling path with the same defect in scope |

## Known Limitations
- FlowSpec rules for protocols with no canonical name stay unenforceable until the residual case in A-2 is decided; the difference is that they are now refused loudly instead of breaking the reconcile.
- The functional coverage depends on FlowSpec injection from a test peer, which `test/plugin/flowspec-fw-add.ci` records as missing today. If that injection cannot be built in this spec, the interop scenario carries the end-to-end proof instead and the `.ci` files cover translation plus registry only.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented

The production fix shipped across four commits on 2026-08-18 and 2026-08-19,
before the closure agent ran, and the spec's Status was never advanced.

| Commit | Subject |
|--------|---------|
| `7499e66b4` | fix(flowspec): refuse a rule it cannot translate, never widen it |
| `c848301fd` | test(flowspec): spell two compound test files as the repo does |
| `f0b246e7a` | fix(flowspec): enforce every value, or refuse the rule |
| `161162baf` | fix(flowspec): let the firewall bridge see the events the daemon sends |

- `internal/component/firewall/protocol.go` gained `ProtocolName`, the derived
  reverse map `protocolNames` built by `buildProtocolNames`, and `ProtocolNames`.
  One table, both directions, no second literal.
- `protoName` is gone. `protocolMatches` (`internal/plugins/flowspec-firewall/translate.go`)
  resolves every value through `firewall.ProtocolName`, refuses a number with no
  canonical name with `errUnknownProtocol`, refuses a value above 255, drops
  repeats, and returns one `MatchProtocol` per listed protocol.
- `translateFlowSpec` holds the protocol matches back and expands one term per
  protocol, so a multi-value component enforces every value instead of the first.
- `handleFlowSpecAdd` (`internal/plugins/flowspec-firewall/engine.go`) drops the
  route on any translation error, counts it through `refusalReason` and
  `countRuleRefused`, and writes a WARN naming the peer, the rule key and the error.
- The VPP and ddos-local private tables are gone: `applyMatch`
  (`internal/plugins/firewall/vpp/translate.go`) resolves through
  `firewall.ProtocolNumber` and `buildDropTerm` (`internal/plugins/ddos/local/match.go`)
  through `firewall.ProtocolName`.
- `parsePolicyMatch` (`internal/plugins/policyroute/config.go`) refuses a protocol
  outside the canonical set and names the accepted values from `ProtocolNames`;
  the `protocol` leaf in `internal/plugins/policyroute/yang/ze-policyroute-conf.yang`
  is typed `fw:protocol-name`.

The closure phase added the remaining proof, all of it test-side, with no
production Go file changed:

- `TestProtocolTableIsSingleSource` (`internal/component/firewall/protocol_test.go`):
  reads every non-test Go file under `internal`, `pkg` and `cmd` that names
  `MatchProtocol` and fails when one spells a canonical name within three lines of
  that protocol's IANA number. The scope is derived from the vocabulary, not listed.
- `TestUnknownProtocolRouteIsCountedAndNamed`
  (`internal/plugins/flowspec-firewall/value_completeness_test.go`).
- `test/plugin/flowspec-metrics-registered.ci`: the refusal series reaches the
  Prometheus exposition of a running daemon.
- `test/plugin/flowspec-fw-protocol-sctp.ci` and
  `test/plugin/flowspec-fw-untranslatable-keeps-others.ci`: driver predicates and
  assertions corrected, plus a refusal-counter barrier and a telemetry block.
- `test/interop/scenarios/bgp-flowspec-sctp-gobgp/`: a real peer originates the SCTP
  rule and ze installs it in the kernel. `test/interop/Dockerfile.ze` gains `nftables`
  so the scenario can read what reached the kernel.

### Bugs Found/Fixed

- **Both kernel `.ci` files were vacuous.** Each driver polled for the literal
  `sctp` in `nft list ruleset`, while `lowerProtoMatch` programs a raw `expr.Cmp`
  over `MetaKeyL4PROTO` and nft prints `meta l4proto 132`. The predicate never
  matched the surface it was written against. Fixed: each driver matches what nft
  prints, and each `expect` now asserts one full rule line carrying every condition
  the route announced, so a DROPPED match reddens it as well as a missing one.
  Journal row in `plan/journal/green-that-could-not-have-been-red.md`.
- **`flowspec-fw-untranslatable-keeps-others` was flaky at 1 run in 5.** It killed
  the daemon as soon as the kernel carried the sctp rule and then asserted a WARN
  nothing had ordered against that kill. Fixed with a barrier on the
  `ze_flowspec_rules_refused_total` series with reason `unknown-protocol`, which the
  refusal itself publishes. 8/8 green after.
- **`refusedReasonUnknownProtocol` had no producer any test drove.** Covered by
  `TestUnknownProtocolRouteIsCountedAndNamed` and, for the exposition half,
  `test/plugin/flowspec-metrics-registered.ci`.
- **`plan/journal/gate-fires-outside-its-population.md` was malformed**: a row sat
  between the table header and its separator, so the file held no readable table.
  Repaired during closure; another session's commit `dcde27486` absorbed the repair.

### Documentation Updates

Shipped in `7499e66b4`, verified against source during closure:

- `docs/guide/firewall.md` documents `ze_flowspec_rules_refused_total` with its five
  reason values, which match the constants in
  `internal/plugins/flowspec-firewall/metrics.go`, and lists `sctp` in its protocol
  table.
- `docs/guide/policy-routing.md` lists the ten accepted protocol names, matching
  `ianaProtocolNumbers`, and states the commit-time refusal.
- `docs/guide/flowspec-protected-router.md` states that a rule ze cannot enforce is
  refused and counted rather than installed.
- `make ze-doc-wiring-check` PASSED on 2026-08-22, with `ze-doc-verify`,
  `ze-doc-index-check`, `ze-digest-check` and `ze-spec-citation-check` included.

### Deviations from Plan

| Planned | Actual | Why |
|---------|--------|-----|
| Interop scenario `bgp-flowspec-sctp-frr` | `bgp-flowspec-sctp-gobgp` | FRR 10.3.1 cannot ORIGINATE FlowSpec. Verified independently at closure by reading the command strings compiled into the pinned image's own `bgpd` (`quay.io/frrouting/frr:10.3.1`): every flowspec command is a `show`, a `clear`, a `debug`, `address-family ipv4 [flowspec]`, `bgp default ipv4-flowspec`, or an `activate ... by default`, and the only NLRI entry point is `bgp_nlri_parse_flowspec`. No route-origination command exists under the flowspec address family. GoBGP originates it with `gobgp global rib -a ipv4-flowspec add match ... then discard` |
| Learned summary at `plan/learned/NNN-<name>.md` | Journal rows | `plan/learned/NNN-*.md` stopped being a destination in `2cff2050a`; the journal replaced it |
| `TestPolicyRouteProtocolRejectsUnknownName` in `translate_test.go` | `internal/plugins/policyroute/config_test.go` | The validator lives in `parsePolicyMatch` in `config.go`, so its test sits beside it |

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | The spec's interop row named FRR as the peer that would announce the SCTP rule | FRR 10.3.1 receives FlowSpec and cannot originate it | Listing the flowspec command set compiled into the pinned FRR image | Scenario written against GoBGP; recorded as a Deviation |
| approach | Two `.ci` files asserted the string `sctp` in the kernel ruleset | nft prints `meta l4proto 132`, so the predicate could never match | The drivers timed out on their own predicate while the fix worked | Drivers and assertions corrected; journal row written |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| A protocol number outside the five old names must not produce a `MatchProtocol` carrying digits | Done | `protocolMatches`, `internal/plugins/flowspec-firewall/translate.go` | Resolves through `firewall.ProtocolName` and refuses when it answers false |
| One unlowerable FlowSpec term must not abort the whole reconcile | Done | `handleFlowSpecAdd`, `internal/plugins/flowspec-firewall/engine.go` | The route is dropped at translation, so no such term reaches `Apply` |
| The private protocol table must be deleted, not completed in place | Done | `buildProtocolNames`, `internal/component/firewall/protocol.go` | `protocolNames` is derived from `ianaProtocolNumbers` |
| `componentToMatch` must stop discarding values after the first | Done | `protocolMatches` plus the per-protocol term expansion in `translateFlowSpec` | |
| The policy-route `protocol` leaf must be refused at commit | Done | `parsePolicyMatch`, `internal/plugins/policyroute/config.go` | Leaf typed `fw:protocol-name` |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestComponentToMatchSCTPName`, `TestComponentToMatchEveryCanonicalNumber`, `TestLowerProtoMatchAcceptsEveryCanonicalName`, interop `bgp-flowspec-sctp-gobgp` | |
| AC-2 | Done | `TestComponentToMatchRejectsUnnamedProtocol`, `TestComponentToMatchEveryWireValue`, `TestUnknownProtocolRouteIsCountedAndNamed`, `test/plugin/flowspec-metrics-registered.ci` | |
| AC-3 | Done | `TestComponentToMatchMultipleProtocolValues`, `TestTranslateFlowSpecMultipleProtocolsBecomeSeparateTerms` | |
| AC-4 | Done | `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers`, `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | |
| AC-5 | Done | `TestPolicyRouteProtocolRejectsUnknownName`, `TestPolicyRouteProtocolAcceptsEveryCanonicalName` | |
| AC-6 | Done | `TestProtocolTableIsSingleSource` | Re-measured at closure with the go test cache defeated; see Goal Validation |
| AC-7 | Done | One `ProtocolNames` loop per consumer: nft, VPP, ddos-local, flowspec, YANG | |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| The eleven unit tests | Done | as listed in the TDD Test Plan | `make ze-unit-pkg-test` over the five packages: ok |
| `flowspec-fw-protocol-sctp` | Done | `test/plugin/flowspec-fw-protocol-sctp.ci` | needs-linux, caps=net-admin; runs under `ze-netns-plugin-test` and QEMU |
| `flowspec-fw-untranslatable-keeps-others` | Done | `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | same |
| `flowspec-metrics-registered` | Done | `test/plugin/flowspec-metrics-registered.ci` | Run at closure: PASS in 508ms |
| `bgp-flowspec-sctp-gobgp` | Done | `test/interop/scenarios/bgp-flowspec-sctp-gobgp/` | |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| Every file in "Files to Modify" | Done | All landed in `7499e66b4`, `f0b246e7a` or `161162baf` |
| Every file in "Files to Create" | Done | `ls` evidence in Pre-Commit Verification |
| `docs/guide/firewall.md`, `docs/guide/policy-routing.md` | Done | `7499e66b4` |

### Audit Summary
- **Total items:** 20
- **Done:** 20
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A FlowSpec rule for any protocol with a canonical name is enforced, not refused | interop | `bgp-flowspec-sctp-gobgp`: GoBGP originates `match destination 10.99.5.0/24 protocol ==sctp then discard` and ze's kernel ruleset carries the matching rule. Red against the pre-fix five-name table: `FAIL: ze installed no flowspec table`, with ze logging `unknown protocol "132"` |
| A peer cannot freeze this router's firewall reconciliation with one legal FlowSpec route | functional | `test/plugin/flowspec-fw-untranslatable-keeps-others.ci`: the locally configured `ze_fslocal` table and the enforceable route that arrives SECOND both reach the kernel while a route naming protocol 253 is refused. Unit companion `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers` |
| A rule ze cannot enforce is visible to the operator rather than silent | functional | `test/plugin/flowspec-metrics-registered.ci`, run at closure: PASS. Discrimination re-measured at closure by deleting `ConfigureMetrics: bindMetrics` from `internal/plugins/flowspec-firewall/register.go` and rebuilding both binaries: FAIL, with the driver reporting the `unknown-protocol` series absent from a 21935-byte scrape while the refusal WARN still appeared. `register.go` restored byte-identical |
| Exactly one protocol-name table exists in the repository | unit (structural) | `TestProtocolTableIsSingleSource`. Re-measured at closure with the go test cache DEFEATED, because the test reads other packages' source as DATA and the package is cacheable (a second unmutated run reported `ok (cached)`). Baseline `go test -count=1`: ok, 0.585s. With a five-name private table appended to `internal/plugins/flowspec-firewall/translate.go`, `go test -count=1`: FAIL, naming `gre` and 47, `icmp` and 1, `icmpv6` and 58, `tcp` and 6, `udp` and 17. `translate.go` restored byte-identical, shasum `535e08ee1968cedb128e71e424da8b0034dd137c`, `git status` clean |
| An operator cannot commit a policy route the backend will refuse hours later | unit | `TestPolicyRouteProtocolRejectsUnknownName`; red with the `firewall.ProtocolNumber` guard removed, all five spellings accepted |

## Deferrals Resolved

The spec declares no deferral shard, and `plan/deferrals/` holds none for this
stem, so there is no shard to `git rm`. The two findings this work walked into
are homed below.

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| none: the spec declares no shard | n/a | `ls plan/deferrals/` matches no `flowspec` stem |
| The flowspec kernel table is named `flowspec`, outside the `ze_` ownership prefix | deferred | Homed at `spec-fixit-flowspec-table-never-swept`, Status `ready`. Journal row in `plan/journal/gate-excludes-part-of-its-population.md`, committed by another session in `feaa2bf7e`. It blocks no AC here: the announced rule is installed and does enforce |
| The fourth private protocol table, `protoName` in `internal/plugins/trafficusage/metrics.go` | deferred | Journal row in `plan/journal/helper-bypassed-by-an-open-coded-copy.md`. Outside AC-6, which names the flowspec, VPP and ddos-local copies; taking it renames three live Prometheus label values, which is its own change |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-flowspec-protocol-name-drift-ebf5d9b3-b158-40df-bba0-32a51591883e.md`, 16 files, verdict clean |
| `review_gate.py check` | clean |
| Rounds | 2 |
| Reviewer lenses used | wiring plus functional-test coverage; removed-behaviour and test-rewrite audit; logic and guard correctness; security and allocation; docs and gate hygiene |

The review ran inline in the closure agent, which did not author the diff. Its
automated pre-checks: `make ze-repository-check` all checks passed;
`python3 scripts/dev/audit-test-relaxation.py` reported 8 findings, of which 6
are another session's RFC-tagged IKE test files and are not this closure's, and
2 are the `.ci` files below.

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `audit-test-relaxation.py` reports both `.ci` files WEAKENED for a removed `reject=`, and `test/weakened.md` carried no row, so `weakened_problems` in `scripts/dev/commit_helper.py` would refuse commit A | `test/plugin/flowspec-fw-protocol-sctp.ci`, `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | Re-derived at the producer rather than taken on trust, and the removals are correct. With the defect present `lowerProtoMatch` (`internal/plugins/firewall/nft/lower_linux.go`) refuses `MatchProtocol{"132"}` and `Apply` (`internal/plugins/firewall/nft/backend_linux.go`) returns that error before its single `conn.Flush`, so no rule of ANY spelling reaches the kernel and the reject could not fire. The removed patterns also carry literal double quotes, which nft never writes around an l4proto value. What replaces them is one full rule line, strictly stronger: it implies the old protocol assertion and additionally reddens when a match is DROPPED. Two rows added to `test/weakened.md`; `make ze-test-weakened-check` green |
| 2 | ISSUE | Commit B removes the spec while `spec-fixit-flowspec-table-never-swept` cites it by path, and `scripts/dev/spec-citation-check.py` globs the WORKING TREE, so `make ze-spec-citation-check` would go red after closure | `plan/.citation-baseline` | The citer is untracked and belongs to another spec, so the stem was baselined rather than the citer edited. `make ze-spec-citation-check` PASSED |
| 3 | NOTE | A 2026-08-22 row in `plan/journal/gate-fires-outside-its-population.md` sat between the table header and its separator, so the file held no readable table | `plan/journal/gate-fires-outside-its-population.md` | Separator moved above the row. `journal_row_cells` (`scripts/dev/journal.py`) now returns five cells for every row of every journal file this closure carries, with zero `MALFORMED`. Another session's commit `dcde27486` absorbed the repair |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `test/plugin/flowspec-fw-protocol-sctp.ci` | Yes | `ls -1`: 4.4K, 2026-08-22 12:28 |
| `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | Yes | `ls -1`: 7.2K, 2026-08-22 12:37 |
| `test/plugin/flowspec-metrics-registered.ci` | Yes | `ls -1`: 5.4K, 2026-08-22 12:17 |
| `test/interop/scenarios/bgp-flowspec-sctp-gobgp/gobgp.toml` | Yes | `ls -1`: 354 bytes |
| `test/interop/scenarios/bgp-flowspec-sctp-gobgp/ze.conf` | Yes | `ls -1`: 437 bytes |
| `test/interop/scenarios/bgp-flowspec-sctp-gobgp/check.py` | Yes | `ls -1`: 3.8K, mode 755 |
| `test/interop/Dockerfile.ze` | Yes | `ls -1`: 2.0K, holding `RUN apk add --no-cache tini python3 nftables` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | Protocol 132 becomes the canonical name and lowers | `protocolMatches` calls `firewall.ProtocolName`; `lowerProtoMatch` resolves through `firewall.ProtocolNumber`. `make ze-unit-pkg-test PKG="./internal/component/firewall/... ./internal/plugins/flowspec-firewall ./internal/plugins/policyroute ./internal/plugins/ddos/local ./internal/plugins/firewall/vpp"`: ok, every package |
| AC-2 | An unnamed number is refused, counted and logged | `protocolMatches` returns `errUnknownProtocol`; `refusalReason` maps it to `unknown-protocol`; `handleFlowSpecAdd` counts and WARNs. `test/plugin/flowspec-metrics-registered.ci` run at closure: PASS in 508ms |
| AC-3 | Every listed value is enforced | `protocolMatches` loops all values with a `seen` map that drops repeats; `translateFlowSpec` emits one term per held-back protocol match |
| AC-4 | Other owners' rulesets still reach the kernel | `handleFlowSpecAdd` returns before `b.rules.add`, so the untranslatable term never reaches `Apply`, which then reaches `conn.Flush` for the rest |
| AC-5 | A non-canonical policy-route protocol is refused at commit | `parsePolicyMatch` refuses and names `strings.Join(firewall.ProtocolNames(), ", ")`; the leaf is `type fw:protocol-name` |
| AC-6 | Exactly one protocol-name table exists | `grep -rn "firewall.ProtocolNumber\|firewall.ProtocolName\|ProtocolNames()" internal/ --include="*.go"` outside tests returns 13 hits, all CALLS except the three definitions in `internal/component/firewall/protocol.go`. `TestProtocolTableIsSingleSource` green at `-count=1`, red against a re-introduced private table |
| AC-7 | Every backend resolves every canonical name | The same grep: `nft/lower_linux.go`, `vpp/translate.go`, `vpp/classify_linux.go`, `vpp/nat_linux.go`, `policyroute/config.go`, `ddos/local/match.go` and `flowspec-firewall/translate.go` all resolve through the canonical table and none holds a name literal |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| FlowSpec NLRI with protocol 132 arrives from a peer | `test/plugin/flowspec-fw-protocol-sctp.ci` | Yes: the peer block sends a hand-built MP_REACH whose type 3 component carries 132, and the driver reads `nft list ruleset` |
| The resulting term is lowered by the nft backend | same | Yes: the expect asserts one full kernel rule line |
| A peer announces a rule ze cannot translate | `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | Yes: protocol 253 refused, `ze_fslocal` still lands, refusal WARN and counter both asserted |
| An operator commits a policy route with a non-canonical protocol | `TestPolicyRouteProtocolRejectsUnknownName` | Yes: unit, driven at the parser |
| A peer announces a FlowSpec SCTP rule against a running daemon | `test/plugin/flowspec-fw-protocol-sctp.ci` plus `bgp-flowspec-sctp-gobgp` | Yes: the interop scenario is auto-discovered by `run.py` (directory listing plus `check.py`), its `gobgp.toml` starts the GoBGP container at `GOBGP_IP`, and the ze container carries `caps=["NET_ADMIN"]` |
| The refusal reaches an operator scraping Prometheus | `test/plugin/flowspec-metrics-registered.ci` | Yes: run at closure, PASS; FAIL with `ConfigureMetrics` unwired |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `Apply` (`internal/plugins/firewall/nft/backend_linux.go`) returns the first `applyTable` error before it calls `conn.Flush`; read at the producer during closure |
| A-2 | confirmed | The YANG `protocol-name` enumeration lists exactly the ten names in `ianaProtocolNumbers`; the residual case is REFUSE, implemented in `protocolMatches` |
| A-3 | confirmed | The grep under AC-6 above: no consumer of `MatchProtocol` accepts a numeric spelling |
| A-4 | confirmed | `AuditTables` compares `LastApplied` against the kernel, so a reconcile that never ran reports no drift. The observability AC stayed in scope and is closed by the refusal counter and its `.ci` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| `docs/guide/firewall.md` names the five reason values of `ze_flowspec_rules_refused_total` | The five `refusedReason*` constants in `internal/plugins/flowspec-firewall/metrics.go` | Yes |
| `docs/guide/policy-routing.md` lists the accepted protocol names | `ianaProtocolNumbers` in `internal/component/firewall/protocol.go` | Yes |
| `docs/guide/flowspec-protected-router.md` states a rule ze cannot enforce is refused and counted | `handleFlowSpecAdd` in `internal/plugins/flowspec-firewall/engine.go` | Yes |
| Item 12, `docs/architecture/policyroute/policy-routing.md` | `grep -n protocol` returns one hit, in a sentence naming the match generically and stating no type | No update needed |
| Item 10, `docs/functional-tests.md` | The two kernel `.ci` already carried `option=needs-linux:caps=net-admin` from `7499e66b4`; `flowspec-metrics-registered` needs no capability | No update needed |
| Doctor check for a runtime dependency | `nftables` is added to a TEST container only; ze programs the kernel over netlink and gains no new dependency | No check owed |
| Gate results | `make ze-doc-wiring-check` exit 0, with wiring, doc-verify, doc-index, digest, docker-exec and spec-citation all PASSED; `make ze-repository-check` all checks passed | Yes |

## Core Insight

A number rendered as text is not a name, and the moment one crosses a boundary
whose contract is a name, the failure surfaces at the far end of the pipeline
rather than where it was made. The repair that holds is not a wider table: it is
one table with both directions derived from it, plus a producer that REFUSES
rather than inventing a spelling. The refusal then has to be visible, because the
alternative to a stalled reconcile is a peer that believes ze filters traffic ze
does not.
