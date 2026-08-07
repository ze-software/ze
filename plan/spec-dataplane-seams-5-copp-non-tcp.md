# Spec: dataplane-seams-5 -- Control-Plane Policing Beyond TCP (Skeleton)

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | plugin |
| Depends | `plan/spec-dataplane-seams-0-umbrella.md` (finding F-5) |
| Phase | - |
| Deferral shard | `plan/deferrals/dataplane-seams.md` (create on the first deferral) |
| Updated | 2026-08-07 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`translatePolicy` in `internal/plugins/copp/translate.go` hardcodes a TCP
protocol match in both the trusted-source term and the rate-limit term. Control
plane policing therefore covers TCP destination ports and nothing else. DHCP,
IPv6 ND, OSPF, IS-IS, BFD and PPPoE are unpoliced.

**This is the anticipated extension, not a defect.** CoPP was scoped to BGP on
purpose. `plan/learned/1005-cp-survival-2-copp-port179.md` records the design,
and its Consequences section says directly: "Any future control-plane protection
(e.g., OSPF, LDP, SSH) can extend the copp plugin with additional protocol blocks
under `control-plane-protection { ... }`."

### The part learned 1005 did not anticipate

Its examples are all IP protocols, and those are the easy half. The `ze_copp`
table uses `FamilyInet`, chosen so one input chain covers IPv4 and IPv6. An
`inet` table sees only IP traffic.

| Protocol | Reachable from an `inet` input chain | Note |
|----------|--------------------------------------|------|
| BGP | Yes | covered today |
| SSH, LDP | Yes, TCP | needs only a port block |
| OSPF, VRRP, RSVP-TE | Yes, IP protocol number, not a port | the TCP match must become a protocol match, not just a wider port list |
| BFD, IKE, DHCPv4, DHCPv6 | Yes, UDP | needs a UDP term |
| IPv6 ND | Yes, ICMPv6 | rate-limiting ND needs care; over-policing breaks neighbor resolution |
| ARP | **No** | not IP. Needs the `arp` or `netdev` family, or a different mechanism |
| IS-IS | **No** | runs directly on the link layer |
| PPPoE | **No** | its own ethertypes, not IP |

So the spec has two halves, and the second is a design question rather than a
widening: **what polices the non-IP control traffic, given the current table
family cannot see it.** Options to weigh include a second table in the `netdev`
or `arp` family, a tc ingress filter, or accepting that ze does not police them
and saying so in the docs.

### Constraints that bind this

- **The existing BGP protection must not weaken.** `test/firewall/copp-bgp.ci`, `copp-trusted.ci` and `copp-withdraw.ci` must all still pass unchanged.
- **Term order is structurally fixed** (established, then trusted, then limit) and learned 1005 calls wrong ordering the dangerous failure mode. Adding protocol blocks must not make ordering configurable.
- **Default chain policy is accept, not drop**, to avoid lock-out on first apply. Any new protocol block inherits that choice and must not silently make the chain drop.
- **Over-policing is worse than under-policing here.** A rate limit on ND or DHCP that is too tight breaks address resolution for every host on the segment. Defaults need care, and the docs need to say what happens at the limit.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/ddos-mitigation.md` - the deployment doc CoPP was gated on
- [ ] `ai/patterns/config-option.md` - the structural template for the new YANG blocks
- [ ] `ai/rules/config.md` - YANG vs env var, and naming
- [ ] `ai/rules/plugins.md` - removing the plugin must remove all CoPP; no copp spelling in generic packages

### Learned Summaries
- [ ] `plan/learned/1005-cp-survival-2-copp-port179.md` - the design this extends
  → Decision: copp is a system plugin over a firewall extension, because removing the plugin must remove all CoPP.
  → Decision: term order is structurally fixed (established, trusted, limit) because wrong ordering is the dangerous failure mode. Do not make it configurable.
  → Decision: default chain policy is accept, not drop, to avoid lock-out on first apply. Operators opt into drop.
  → Decision: `FamilyInet` was chosen so one chain covers IPv4 and IPv6. That choice is exactly what puts ARP, IS-IS and PPPoE out of reach.
  → Constraint: the plugin mirrors `policyroute` (`RegisterTables` plus `ApplyAll`). Extend that, do not add a new firewall API.
  → Constraint: `firewall.ParseRateSpec` is the shared rate-spec parser. Reuse it.

### Related Specs
- [ ] `plan/spec-dataplane-seams-0-umbrella.md` - the parent, finding F-5
- [ ] `plan/spec-cp-survival-0-umbrella.md` - **in-progress, owns copp.** Read its closure record before starting, and record the split there
- [ ] `plan/spec-dataplane-seams-4-control-packet-rx.md` - the receive side of the same question

**Key insights:** (minimal context to resume after compaction)
- This is the extension learned 1005 anticipated, for IP protocols.
- For ARP, IS-IS and PPPoE it is a new design question, because the `inet` family cannot see them.
- Over-policing ND or DHCP breaks a whole segment. Defaults matter more than coverage here.

## Current Behavior (MANDATORY)

**Source files read:** (verified 2026-08-07)
- [ ] `internal/plugins/copp/translate.go` - `translatePolicy` builds the `ze_copp` table, `FamilyInet`, one base input chain at priority 0. Terms in fixed order: accept established or related, then per-trusted-prefix accept matching TCP plus destination port plus source prefix, then a rate-limit term matching TCP plus destination port plus conn-state new. Chain policy is accept unless the operator selected drop. `portRanges` expands each configured port into a single-value range

**Source files to read before design:**
- [ ] `internal/plugins/copp/model.go`, `config.go`, `register.go`, `doctor.go` - the policy type, its YANG parsing, and the apply path
- [ ] `internal/plugins/copp/yang/ze-copp-conf.yang` - the config surface the new blocks extend
- [ ] `internal/component/firewall/` - which match kinds exist, and whether a non-`inet` family is supported by the nft backend at all

**Behavior to preserve:**
- The BGP protection exactly as it stands, proven by `test/firewall/copp-bgp.ci`, `copp-trusted.ci`, `copp-withdraw.ci`.
- Fixed term order, established then trusted then limit.
- Default chain policy accept.
- The `ze_copp` table-name prefix that marks the table as ze-owned. Without it, reconcile does not recognise it and withdrawal fails.
- `doctor-copp-missing` and its diagnostic code.

**Behavior to change:**
- The protocol matches, so the policy can express protocols other than TCP.
- Whatever the design decides for non-IP control traffic.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config under the control-plane-protection block.

### Transformation Path
1. `config.go` parses the YANG into the policy type.
2. `translatePolicy` converts the policy into a firewall table.
3. `register.go` installs it via `RegisterTables` and `ApplyAll`.
4. The nft backend renders the table, and the kernel enforces the limit expression.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| config ↔ copp plugin | YANG | No |
| copp ↔ firewall component | `RegisterTables` plus `ApplyAll` | No |
| firewall ↔ kernel | nft table in the `inet` family | No |
| (proposed) firewall ↔ kernel for non-IP | a family the current table cannot use | No |

### Integration Points
- `internal/plugins/copp` - the plugin being extended
- `internal/component/firewall` - the match and action vocabulary available
- `internal/core/diagnostic/codes.go` - the existing doctor code

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
| A-1 | The firewall component already has the match kinds needed for IP-protocol and UDP terms | It has protocol, destination-port, source-address, conn-state and limit today | The spec grows to include firewall work | Read the firewall match vocabulary | unvalidated |
| A-2 | The nft backend can render a table in a family other than `inet` | Not established | The non-IP half needs a different mechanism entirely, such as tc ingress | Read the nft backend's family handling | unvalidated |
| A-3 | Rate-limiting ND and DHCP is safe with careful defaults | Not established. Over-policing breaks address resolution for a whole segment | The feature ships a foot-gun | A functional test that floods at just under and just over the limit, checking a legitimate client still resolves | unvalidated |
| A-4 | `spec-cp-survival-0-umbrella` has no live claim on this work | It is in-progress and awaiting closure verification | Two specs edit copp at once | Read that umbrella's closure record first | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A widened match weakens the existing BGP protection | `copp-bgp.ci` or `copp-trusted.ci` changes behavior | Those tests must pass unchanged. If a test needs editing to pass, the change is wrong (`ai/rules/testing.md`) |
| R-2 | An ND or DHCP limit that is too tight breaks a segment in production | Hosts fail to resolve neighbors or obtain leases under normal load | Conservative defaults, a functional test at the boundary, and documentation of what happens at the limit |
| R-3 | Adding protocol blocks makes term ordering configurable by accident | The generated chain's term order varies with config order | Ordering stays structurally fixed. Learned 1005 names wrong ordering as the dangerous failure mode |
| R-4 | The non-IP half expands the spec into a second mechanism with its own lifecycle | The design introduces tc filters or a second table family with separate withdrawal | Split it into its own child spec if it does. The IP half is independently useful |
| R-5 | A new table family is installed but never withdrawn on removal, because the ownership-prefix mechanism does not extend to it | The table survives plugin removal | `ai/rules/plugins.md`: removing the plugin removes all CoPP. Test withdrawal for any new table |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Control-plane traffic is dropped that should not be. An over-tight ND or DHCP limit breaks address resolution for every host on a segment, which is worse than the DDoS it defends against |
| How is it reverted? | Single-commit revert, plus table withdrawal. Verify withdrawal for any newly introduced table family |
| Who else touches this path? | `spec-cp-survival-0-umbrella` (in-progress, owns copp), child 4 (the receive side) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| (fill at design) a control-plane-protection block naming a non-TCP protocol | → | (fill at design) `translatePolicy` | (fill at design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | The existing BGP configuration is applied unchanged | The generated table is equivalent to today's, and all three existing copp functional tests pass without edits |
| AC-2 | A UDP control protocol is configured for policing | Traffic above the limit is policed and traffic below it is not, proven from the entry point |
| AC-3 | An IP-protocol-numbered protocol such as OSPF is configured | It is policed by protocol number, not by port |
| AC-4 | A legitimate client operates normally while a flood is being policed | It still completes its exchange. Over-policing is a failure, not a pass |
| AC-5 | A protocol that the table family cannot see is configured | Config is rejected with a message naming why, or the design's chosen mechanism polices it. Silently accepting config that polices nothing is not acceptable |
| AC-6 | The plugin is removed | Every table it installed is withdrawn, including any in a new family |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures policing for DHCP and floods the server | config → copp → firewall → nft → kernel limit | (fill at design) |
| 2 | Configures policing for a protocol ze cannot reach | config → copp config validation | (fill at design) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (fill at design) | `internal/plugins/copp/translate_test.go` | The generated table for a BGP-only policy is unchanged from today | |
| (fill at design) | `internal/plugins/copp/translate_test.go` | A UDP block and an IP-protocol block generate the expected terms, in the fixed order | |
| (fill at design) | `internal/plugins/copp/config_test.go` | Config naming an unreachable protocol is rejected with a message that names the reason | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| rate | (fill at design; reuse `firewall.ParseRateSpec` semantics) | | | |
| burst | (fill at design) | | | |
| IP protocol number | 0-255 | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `copp-bgp` (existing, must pass unchanged) | `test/firewall/copp-bgp.ci` | The BGP rate limit still applies exactly as before | |
| `copp-trusted` (existing, must pass unchanged) | `test/firewall/copp-trusted.ci` | Trusted-source ordering is unchanged | |
| `copp-withdraw` (existing, must pass unchanged) | `test/firewall/copp-withdraw.ci` | Table withdrawal still works | |
| new: UDP control protocol policed | `test/firewall/*.ci` | A DHCP or BFD flood is policed while a legitimate exchange still completes | |
| new: IP-protocol-numbered protocol policed | `test/firewall/*.ci` | OSPF is policed by protocol number | |
| new: unreachable protocol rejected | `test/firewall/*.ci` | Config naming a protocol the mechanism cannot see is refused with a clear message | |
| new: withdrawal of any new table family | `test/firewall/*.ci` | Removing the plugin leaves no table behind | |

## Files to Modify
- `internal/plugins/copp/translate.go` - the protocol matches
- `internal/plugins/copp/model.go`, `config.go` - the policy type and its parsing
- `internal/plugins/copp/yang/ze-copp-conf.yang` - the new protocol blocks
- `internal/plugins/copp/doctor.go` - the check, if a new mechanism needs one
- `internal/core/diagnostic/codes.go` - a new diagnostic code, if a new mechanism needs one
- `docs/guide/ddos-mitigation.md` - what is policed, what is not, and what happens at the limit
- `docs/features.md` - the CoPP feature row

## Files to Create
- `test/firewall/*.ci` - the new functional tests (names at design)

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/copp/yang/ze-copp-conf.yang` - new protocol blocks |
| YANG validation constraints | Yes | Protocol numbers take a `range`, protocol names an `enumeration`, rate specs the existing type |
| YANG custom validators | Yes | Rejecting a protocol the chosen mechanism cannot see needs a `ze:validate` validator; native constraints cannot express it |
| CLI commands/flags | N-A | Config only; no new verb |
| CLI grammar (keyword before value) | Yes | `ai/rules/cli.md` - the new config blocks follow keyword-before-value |
| Editor autocomplete | Yes | Automatic for the new enumeration leaves; confirm at design |
| Functional test for new RPC/API | Yes | `test/firewall/*.ci`, named above |
| Pipe completeness | N-A | No new CLI output |
| Env var registration | N-A | No env var; this is operator policy and belongs in YANG |
| Doctor check for runtime dependencies | Yes | If a new table family or tc filter is introduced, it is a new runtime dependency and needs an owning-package check plus a code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | Policed and dropped counts per protocol. An operator cannot tune a limit they cannot observe |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` - the CoPP row |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | N-A | No new verb |
| 4 | API/RPC added/changed? | N-A | No API change |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/ddos-mitigation.md` |
| 7 | Wire format changed? | N-A | No wire format change |
| 8 | Plugin SDK/protocol changed? | N-A | No SDK change |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | Policing is local policy, not an RFC obligation. Confirm no ND rate limit contradicts an RFC 4861 requirement at design |
| 10 | Test infrastructure changed? | N-A | Uses existing `.ci` infrastructure |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` - CoPP coverage is a comparable feature |
| 12 | Internal architecture changed? | Yes | Subsystem doc, if a second mechanism is introduced |
| 13 | Route metadata keys added/changed? | N-A | No route payload change |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` or the subsystem telemetry doc |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/features/plugins.md` if the plugin's declared surface changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on `copp/translate.go` and the copp YANG |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/ddos-mitigation.md` shows CoPP config examples |

## Implementation Steps

1. **Phase: coordinate** -- read `spec-cp-survival-0-umbrella`'s closure record and agree the split before touching copp.
2. **Phase: Wiring (MANDATORY FIRST)** -- write the failing functional test that configures a UDP protocol and expects it policed.
3. **Phase: IP protocols** -- generalise the protocol match so UDP and IP-protocol-numbered terms are expressible. Keep term order fixed and the default policy accept. The BGP tests must pass unchanged throughout.
4. **Phase: decide the non-IP question** -- weigh a second table family, a tc ingress filter, or documenting the gap. If the answer is a second mechanism, split it into its own child spec rather than growing this one.
5. **Phase: observability and docs** -- counters, then the guide, saying plainly what is policed and what happens at the limit.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every protocol the config can name is either policed or rejected. None is silently accepted and unpoliced |
| Correctness | Term order is still structurally fixed, and the default chain policy is still accept |
| Feature completeness | Removing the plugin removes every table it installed, in every family (`ai/rules/plugins.md`) |
| Naming | New YANG leaves follow `ai/rules/config.md` naming, and protocol names match how the rest of ze spells them |
| Rule: `ai/rules/testing.md` | The three existing copp tests pass unchanged. A test that needed editing to pass is evidence the change was wrong |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| BGP behavior unchanged | `make ze-test-pkg PKG=./internal/plugins/copp` and the three existing `.ci` tests |
| No protocol silently unpoliced | Config validation rejects what the mechanism cannot see |
| Withdrawal complete | Remove the plugin and check no ze-owned table remains, in any family |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Authorization that could fail open | A rate limit that silently matches nothing is a policy that looks applied and is not. Config validation must reject it (`ai/rules/evidence.md`) |
| Resource exhaustion | This feature defends against exhaustion. An over-tight limit on ND or DHCP causes an outage larger than the attack, so the defaults are the security decision |
| Input validation | Protocol numbers and rate specs come from operator config and take full YANG validation |

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

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|

## Known Limitations
- The non-IP half may correctly close as "ze does not police ARP, IS-IS or PPPoE, and here is why". Documenting a real limit is a valid outcome. Silently accepting config that polices nothing is not.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
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
