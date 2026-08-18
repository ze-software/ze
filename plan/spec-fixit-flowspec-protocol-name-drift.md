# Spec: fixit-flowspec-protocol-name-drift

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | plugin |
| Depends | - |
| Phase | - |
| Deferral shard | - |
| Handoff | - |
| Updated | 2026-08-18 |

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
| A-1 | A lowering error aborts the entire reconcile before `Flush`, so no owner's tables reach the kernel | `Apply` and `applyTable` in `internal/plugins/firewall/nft/backend_linux.go` return the error before the single `Flush` | The blast radius is one rule, not the whole ruleset, and the severity of this spec drops | A unit test that applies one good table plus one unlowerable table and asserts the good table is absent from the fake conn's flushed set | unvalidated |
| A-2 | Every protocol number ze must enforce has a canonical name in `ianaProtocolNumbers` | `internal/component/firewall/yang/ze-firewall-conf.yang` `protocol-name` enumerates exactly those ten | A FlowSpec rule for a legal protocol outside the ten is still unenforceable, and the residual case needs the numeric path in `ProtocolNumber` after all | Decide the residual case explicitly in Phase 2; the AC-4 rejection test pins whichever answer is chosen | unvalidated |
| A-3 | No consumer of `MatchProtocol` accepts a numeric spelling today | `lowerProtoMatch`, `classifyMaskMatch`, `buildDNATMapping`, `applyMatch` all resolve through a name table | A numeric-accepting fix would be simpler than the name fix | Grep every `MatchProtocol` consumer during Phase 1 and record the list in the spec | unvalidated |
| A-4 | The nft `Warn` line plus the apply-duration error bucket are the only operator signal for this failure today | `applyRules` in `internal/plugins/flowspec-firewall/engine.go`, `observeApply` in the firewall metrics file | The observability AC is already met and can be dropped | Read the firewall audit path; if `AuditTables` reports drift for a never-applied ruleset, note it and narrow AC-5 | unvalidated |

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
| `TestComponentToMatchSCTPName` | `internal/plugins/flowspec-firewall/translate_test.go` | protocol 132 becomes the canonical name, not "132" | |
| `TestComponentToMatchEveryCanonicalNumber` | `internal/plugins/flowspec-firewall/translate_test.go` | all ten canonical numbers round-trip to their names | |
| `TestComponentToMatchRejectsUnnamedProtocol` | `internal/plugins/flowspec-firewall/translate_test.go` | replaces `TestProtoNameUnknown`: an unnamed number is refused, never rendered as digits | |
| `TestComponentToMatchMultipleProtocolValues` | `internal/plugins/flowspec-firewall/translate_test.go` | every listed value is enforced or the rule is refused | |
| `TestLowerProtoMatchAcceptsEveryCanonicalName` | `internal/plugins/firewall/nft/lower_linux_test.go` | every name in the canonical table lowers | |
| `TestApplyRulesRejectsUntranslatableRuleAndKeepsOthers` | `internal/plugins/flowspec-firewall/blast_radius_test.go` | validates A-1: a good table still reaches the fake conn's flush when a sibling term is unlowerable | |
| `TestPolicyRouteProtocolRejectsUnknownName` | `internal/plugins/policyroute/translate_test.go` | a non-canonical protocol is refused before lowering | |
| `TestProtocolTableIsSingleSource` | `internal/component/firewall/protocol_test.go` | the flowspec and VPP name sets are derived from `ianaProtocolNumbers`, not independent literals | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| FlowSpec type 3 protocol value | 0-255 | 255 | N/A (0 is legal on the wire) | N/A (the field is one octet) |
| Canonical protocol numbers | 1-132 | 132 (sctp) | 0 (no canonical name) | 133 (no canonical name) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `flowspec-fw-protocol-sctp` | `test/plugin/flowspec-fw-protocol-sctp.ci` | a peer announces a FlowSpec rule matching SCTP and the kernel ruleset carries the matching rule | |
| `flowspec-fw-untranslatable-keeps-others` | `test/plugin/flowspec-fw-untranslatable-keeps-others.ci` | an untranslatable FlowSpec rule does not stop the local firewall terms reaching the kernel | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `bgp-flowspec-sctp-frr` | `test/interop/scenarios/` | FRR | a real peer announcing a FlowSpec SCTP rule leads to an installed kernel rule on ze; reverting the translator change leaves the rule absent | |

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
| Prometheus counters/metrics | Yes | a counter for rules refused at translation, so a silently unenforceable FlowSpec announcement is visible; name and labels decided in Phase 3 |
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
   - Tests: `test/plugin/flowspec-fw-protocol-sctp.ci`, `test/plugin/flowspec-fw-untranslatable-keeps-others.ci`, the FRR interop scenario
   - Files: the two `.ci` files, `test/interop/scenarios/`
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
| Functional proof | `make ze-functional-plugin-test` runs both new `.ci` files green |

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
