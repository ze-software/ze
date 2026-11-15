# Spec: firewall-prefix-normalize

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | Existing accepted firewall prefixes produce incorrect config diffs and non-idempotent reconciliation; installed match semantics are not claimed broken. |
| Depends | - |
| Phase | 4/4 - independent review clean; closure commits pending |
| Updated | 2026-11-15 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `.claude/rules/planning.md`
3. `internal/component/firewall/config.go` - `parseAddressMatch`
4. `internal/component/firewall/validate.go` - `validateMatch` (and the acknowledged gap comment)
5. `internal/component/firewall/model.go` - `MatchSourceAddress` / `MatchDestinationAddress`

## Task

A firewall term with `source-address 10.10.10.1/30` is accepted even though its
effective match is the network `10.10.10.0/30`. Refuse a literal source or
destination IPv4 or IPv6 prefix whose address has host bits set. Name the
configured table, chain, term, leaf and value, explain the effective network,
and show the corrected configuration statement. Do not silently change the input.

Owner decision, 2026-11-15: Thomas answered "B" to the choice "Reject it: show
the correct prefix and require the operator to enter it." This selects refusal
over the original normalization proposal; the existing filename remains the
work item's identity.

One-off admission, 2026-11-15: "Allow a one-off exception for these three
fixes" authorizes watchdog, firewall prefixes and BMP listener binding to start
above the current WIP limit. The standing cap of 12 is unchanged for other work.
The owner subsequently approved the full presented designs with "go" on
2026-11-15. Implementation follows this approved refusal design.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` - the canonical architecture reference: the design principles all new code follows
- [ ] `docs/architecture/config/syntax.md` - config idempotency expectations.
  → Constraint: the resolved config model should be canonical so re-reading it produces no spurious diff.
- [ ] `ai/rules/config.md` - firewall address matches are operator config.
  → Decision: the owner selected refusal, with the corrected network prefix in the error.

**Key insights:**
- Both dataplane backends already mask (nft `maskedAddr`/`prefixMask`; vpp `.Masked()`), so this is about the *model*, not the installed rule.
- `netip.Prefix.Masked()` supplies the comparison and the suggested network prefix.
- The current parser accepts CIDR literals and `@set` references only. The old range, bare-address and negation acceptance criteria were stale; this work keeps those forms refused.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/firewall/config.go` - `parseAddressMatch` calls `netip.ParsePrefix(v)` and stores the result verbatim in `MatchSourceAddress{Prefix}` / `MatchDestinationAddress{Prefix}` (config.go); no `.Masked()`, no host-bits check.
- [ ] `internal/component/firewall/validate.go` - `validateMatch` has no literal-prefix canonicality guard; the earlier gap comment is no longer present.
- [ ] `internal/component/firewall/model.go` - `MatchSourceAddress` and `MatchDestinationAddress` hold a `netip.Prefix`.
- [ ] `internal/plugins/firewall/nft/lower_linux.go` - `lowerAddrMatch` masks the compared address through `maskedAddr`; the installed match already covers the network.
- [ ] `internal/plugins/firewall/vpp/translate.go` - `prefixToVPP` uses the masked address.
- [ ] `internal/component/firewall/engine.go` - `VerifyConfig`, `parseConfigSections`, `OnConfigure` and `OnConfigVerify` all reach `ParseFirewallConfig` before backend apply.
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `from-block` describes the two supported forms, CIDR and `@set`.

**Behavior to preserve:**
- The installed dataplane rule stays correct (backends keep masking; no regression there).
- Canonical IPv4 and IPv6 prefixes, including host routes and default routes, are accepted unchanged.
- Named-set references keep their existing resolution and type checks.
- Bare addresses, address ranges and `!prefix` remain refused; none is supported by this parser.

**Behavior to change:**
- A noncanonical literal fails parsing before it can become accepted configuration.

## Existing Mechanisms

| Mechanism | Where | Current behavior | This spec |
|-----------|-------|------------------|-----------|
| One literal-address parser | `config.go`, `parseAddressMatch` | Parses a CIDR or returns a named-set match | Extends with one canonicality check after successful CIDR parsing |
| Contextual parse errors | `ParseFirewallConfig`, `parseTable`, `parseChain`, `parseTerm`, `parseFromBlock` | Preserve table, chain, term and source/destination context when wrapping errors | Reuses these wrappers; no private tree walker |
| Offline verifier registration | `register.go`, `InProcessConfigVerifier: VerifyConfig` | Runs the firewall parser for `ze config validate` | Reuses the same rejection |
| Startup and transaction callbacks | `engine.go`, `OnConfigure`, `OnConfigVerify` | Parse before storing pending state or applying tables | Reuses the same rejection before backend changes |
| Operator errors | `internal/core/opererr/opererr.go`, `New` | Carries problem, explanation and exact command or configuration statement | Reuses it for the corrected leaf statement, with the fix on the last line |

## Data Flow (MANDATORY)

### Entry Point
- Config: firewall term `from source-address` and `from destination-address`, at offline validation, daemon startup and configuration verify.

### Transformation Path
1. Operator sets a rule address (possibly with host bits).
2. `parseAddressMatch` parses it via `netip.ParsePrefix`.
3. Compare the parsed prefix with its masked form. If different, return an operator error showing the effective network and the corrected source-address or destination-address statement.
4. The existing error wrappers identify the table, chain, term and leaf. The caller refuses validation, startup or the pending transaction.
5. A canonical prefix follows the existing model and backend path unchanged. An `@set` reference returns through the existing branch before literal parsing.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config ↔ firewall model | `parseAddressMatch` refuses host bits | Source read; full package PASS and changed outcomes counted |
| Offline and live config ↔ parser | `VerifyConfig`, startup and transaction verify | Source read; named real-CLI and own-system live cases PASS |
| Model ↔ dataplane | Existing canonical matches and backend masking | Preserved; no backend change |

### Changed-wire census

No protocol message, event, field name or serialized shape changes. The changed
contract is acceptance of two firewall configuration leaves and their error text.
Source searches for `parseAddressMatch`, `ParseFirewallConfig` and
`MatchSourceAddress`/`MatchDestinationAddress` constructors found the config
parser and separate direct model producers in policyroute, CoPP, GTSM, DDoS,
anomaly, FlowSpec, VRRP and IKE. Those direct producers do not call this parser
and keep their present behavior. No shared model-wide guard is introduced.
The known producer control is `parseFromBlock`; the known reader control is
`parseConfigSections`. Go and YANG carry this change; `.ci` files are its
operator-facing inputs. Backend lowering and non-Go protocol consumers are unchanged.

### Integration Points
- `parseAddressMatch` (`config.go`) - the only changed producer.
- `VerifyConfig` and the engine callbacks - existing callers, covered without changing their signatures.

### Architectural Verification
- [ ] No bypassed layers (refusal at the existing parse point)
- [ ] No unintended coupling (backends unchanged; they still mask harmlessly)
- [ ] No duplicated functionality (one canonicality check at parse, no new validator or backend guard)
- [ ] Registration over hardcoding — firewall match parsing/validation stays in the firewall component; no central change.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Refusal is the intended operator behavior | Owner's answer "B", 2026-11-15 | Design would need revision | Recorded owner decision above | confirmed |
| A-2 | Every literal in these two config leaves reaches `parseAddressMatch` | `parseFromBlock` calls it once for each leaf | A bypass would still accept host bits | Named parser/verifier tests; both real-CLI parse drafts; selected own-kernel reload draft | confirmed |
| A-3 | Existing accepted firewall fixtures use canonical prefixes | Full firewall package and selected existing parse cases pass; repository corpus reports no firewall-prefix refusal, with every failed carrier checked for masking | An affected fixture needs its intended network stated explicitly, with judge approval where required | Scoped cases plus source-aware corpus triage below; overall Darwin corpus remains RED | confirmed for affected firewall inputs |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A previously accepted config now fails | Host-bit prefix at validation or restart | Error names the original and corrected prefix; operator chooses the corrected input |
| R-2 | A new guard rejects supported inputs | Canonical or named-set control goes red | Explicit positive cases for both families, /0, /32, /128 and named sets |
| R-3 | A parser error loses the repair text through wrappers | Final fix line changes or disappears | Assert the full contextual error and its final corrected statement |

## Wiring Test (MANDATORY)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate` with `source-address 10.10.10.1/30` | → | `VerifyConfig` → `parseAddressMatch` refuses | `test/parse/firewall-prefix-host-bits-rejected.ci` |
| Live configuration reload containing a host-bit prefix | → | `OnConfigVerify` → `parseConfigSections` → `parseAddressMatch` refuses | `test/reload/firewall-prefix-host-bits-keeps-active.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted survivors and why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Either address leaf contains IPv4 host bits, including `10.10.10.1/30` | Parser, offline validation and startup refuse; error identifies the leaf and original value, explains the network match and gives the corrected statement with `10.10.10.0/30` | None: the owner selected explicit refusal |
| AC-2 | Either address leaf contains IPv6 host bits, including `2001:db8:10:10::1/126` | Same refusal, with corrected prefix `2001:db8:10:10::/126` | None, for the same reason |
| AC-3 | Canonical IPv4/IPv6 networks, default routes `0.0.0.0/0` and `::/0`, or host routes `/32` and `/128` | Accepted unchanged; match type and prefix are preserved | All canonical prefixes remain useful inputs; host routes name one address |
| AC-4 | Bare address, address range, negated prefix, malformed literal or invalid prefix length | Remains refused | None: these forms are not supported by the current literal parser |
| AC-5 | A valid `@set` source/destination reference | Parsing and existing set resolution/type validation are unchanged | Named sets are a supported alternative to a literal prefix |
| AC-6 | A live valid config is replaced by one containing a host-bit source or destination prefix | Verify refuses before apply; active persisted config and installed rules remain the previous valid ones; corrected canonical config can subsequently commit | Previous active state remains because the transaction failed |
| AC-7 | The refusal is wrapped by table, chain, term and from-block context | Final error carries the configured names, original value, effective network and final corrected leaf statement; `errors.As` can recover the operator error | Existing context wrappers remain; the repair text must survive them |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Validates a term with host bits, then enters the suggested prefix | Offline validator refuses the original and accepts the canonical form | `firewall-prefix-host-bits-rejected.ci`, `firewall-prefix-canonical-accepted.ci` |
| 2 | Reloads a running firewall with an invalid prefix, then corrects it | Verify refuses, active state stays unchanged, corrected reload applies | `firewall-prefix-host-bits-keeps-active.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseAddressMatchRejectsHostBits` | `internal/component/firewall/config_prefix_test.go` | Both families and both leaf directions; AC-1, AC-2 | PASS |
| `TestParseAddressMatchPreservesCanonical` | `internal/component/firewall/config_prefix_test.go` | AC-3, unchanged types and values | PASS |
| `TestParseAddressMatchPreservesNamedSet` | `internal/component/firewall/config_prefix_test.go` | AC-5, unchanged set field and name | PASS |
| `TestParseAddressMatchKeepsUnsupportedFormsRejected` | `internal/component/firewall/config_prefix_test.go` | AC-4 | PASS |
| `TestParseFirewallConfigPrefixErrorContext` | `internal/component/firewall/config_prefix_test.go` | AC-7, operator error type and final statement | PASS |
| `TestVerifyConfigRejectsPrefixHostBits` | `internal/component/firewall/config_prefix_test.go` | Full verifier registration input refuses both leaf directions before backend apply | PASS |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length v4 | 0-32 | 32; canonical /0 and host-bit /0 are both covered | -1 | 33 |
| prefix length v6 | 0-128 | 128; canonical /0 and host-bit /0 are both covered | -1 | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-prefix-host-bits-rejected` | `test/parse/firewall-prefix-host-bits-rejected.ci` | AC-1, AC-2, offline registration and correction text | PASS; promoted |
| `firewall-prefix-canonical-accepted` | `test/parse/firewall-prefix-canonical-accepted.ci` | AC-3, AC-5, positive offline path | PASS; promoted |
| `firewall-prefix-host-bits-keeps-active` | `test/reload/firewall-prefix-host-bits-keeps-active.ci` | AC-6, refused transaction, unchanged persisted and installed state, successful corrected reload | PASS; Ze own-system kernel, zero skips |

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A - config input refusal, no wire protocol changes | - | - | offline and live transaction tests | - |

### Future (if deferring any tests)
- None planned.

## Files to Modify
- `internal/component/firewall/config.go` - refuse host bits in `parseAddressMatch`; reuse existing error context wrappers
- `internal/component/firewall/config_prefix_test.go` - the named focused unit cases, kept apart from other sessions' tests
- `internal/component/firewall/yang/ze-firewall-conf.yang` - document canonical-network requirement and correction behavior in both address leaves
- `docs/guide/firewall.md` - literal input and refusal examples
- `docs/architecture/core-design.md` - the parser's input contract beside the firewall model

`validate.go`, model types and backend lowerers stay unchanged: the parser owns
these configured leaves, while direct model producers retain their existing contracts.

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| Functional test for changed behavior | Yes | The three named parse/reload cases |
| YANG validation constraints | No new validator | Existing registered parser/verifier covers both leaves and all acceptance paths; descriptions change |
| RPC/API or environment additions | No | Existing config leaves and error response carry the refusal |
| Doctor/metrics additions | No | No new runtime dependency or runtime fault class |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config acceptance changed? | Yes | `docs/guide/firewall.md`, canonical prefix and refusal examples |
| 12 | Internal contract changed? | Yes | `docs/architecture/core-design.md`, firewall parsing paragraph |
| 17 | Existing examples for this area? | Yes | Check examples for the two leaf names against the actual parser; repair affected examples only |

## Files to Create
- The three named `.ci` cases, drafted first under `test/draft/parse/` and `test/draft/reload/` through `ze-test`, then promoted only after the main thread's proof runs.
- `internal/test/fixture/firewall_prefix_reload.go` - dedicated observer for the live sequence, with no shared fixture edits.
- `website/changes/fragments/firewall-prefix-normalize.md` - operator-facing closure news.

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 3. Wiring phase | Wiring Test table |
| 4. Implement (TDD) | Implementation Phases below |

### Implementation Phases
1. **Wiring:** draft the offline refusal case and the parser/verifier unit cases; main thread records their expected failure against the current parser.
2. **Refusal:** add the single canonicality guard and typed operator error. Update the two leaf descriptions and the affected design/guide paragraphs with the behavior change.
3. **Live proof:** draft the Linux reload case. Prove rejection keeps both active configuration and installed rules unchanged; then prove the corrected candidate commits. Main thread runs and stress-proves drafts before promotion.
4. **Review and closure:** independent review and spec audit; scoped unit and named functional runs only; two-commit closure. No `./le verify` command is an agent obligation.

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N implemented with file:line |
| Correctness | Both leaves and families refuse host bits; canonical and named-set controls still pass |
| Data flow | One refusal at the existing parser, reached by offline/start/live paths |
| Scope | No new bare-address/range/negation feature and no broad change to direct model producers |
| Registration over hardcoding | change confined to firewall component |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Prefix refusal and positive controls | Scoped firewall package tests, with the named unit tests observed |
| Offline wiring | Both named parse cases |
| Active state preservation | Named Linux reload case, with active stored config and installed rule assertions |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | Confirmed: malformed prefixes still return the `netip.ParsePrefix` error before the new guard |
| Correctness | Confirmed: the guard returns no match; startup and transaction callbacks return its error before applying or changing pending state |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Four selected reload invocations could share one guest kernel safely | The reload suite declares serial guest-root execution, and each copy owns the same nft table name | Invocation 3 of the first selected stress failed after another copy completed; `selectedStressPhase`, `flushOnStop` and nft `shouldDeleteTable` expose the collision | Preserve assertions; run selected guest-root stress serially under CPU burners. This is test admission, not a parser behavior change |
| A paragraph block's final anchors were enough | Each factual paragraph needs its own immediate source anchor | Independent documentation review | Added the parser anchor after the first new guide paragraph; no behavior change |

## Design Insights
<!-- LIVE -->

## Implementation Summary
### What Was Implemented
The parser compares each literal prefix with its masked network and returns a
typed operator error on host bits. It reports the input, effective network and
corrected leaf statement. The existing named-set branch and canonical-prefix
result types are unchanged. Both YANG descriptions and the guide/design
paragraphs now describe refusal.

Main-thread red observed on 2026-11-15:
`tmp/session/2026-11-15-125434/scratch/job-unit-pkg-249b86b7.log` shows
`TestParseAddressMatchRejectsHostBits`, `TestParseFirewallConfigPrefixErrorContext`
and `TestVerifyConfigRejectsPrefixHostBits` failing because host-bit prefixes
were accepted. The main thread's selection included the other parser controls,
but misspelled the full-parser positive control as
`ParseFirewallConfigPreservesCanonicalPrefix`; the actual
`TestParseFirewallConfigCanonicalPrefix` was not selected in that red run and
was covered by the subsequent full firewall package green below.

The dedicated `internal/test/fixture/firewall_prefix_reload.go` observer now
implements `firewall/prefix-host-bits-keeps-active`, with no shared fixture
edits. It checks the running firewall map, persisted active pointer/config bytes
and actual kernel table before and after a rejected reload, then verifies the
corrected reload advances the active version and installs its prefix.

Observed greens on 2026-11-15:

| Scope | Result | Evidence |
|-------|--------|----------|
| Full firewall package, including all seven new unit tests | PASS | `tmp/session/2026-11-15-125434/scratch/job-unit-pkg-721fa93f.log` contains `ok github.com/ze-software/ze/internal/component/firewall 3.199s`; other packages in that same batch are outside this verdict |
| Both real-CLI parse drafts, selector `firewall-prefix-` | PASS 2/2 | Main-thread observed `firewall-prefix-host-bits-rejected` and `firewall-prefix-canonical-accepted` |
| Selected own-system Linux reload draft | PASS 1/1 | Main-thread `test qemu ci test test/draft/reload/firewall-prefix-host-bits-keeps-active.ci` observed the named case pass |
| Each offline parse draft under stress | PASS 80/80 each | Main-thread observed all 80 successful invocations of both named cases; adapter exit 1 means no failure reproduced |
| Selected own-system Linux reload draft, serial under CPU burners | PASS 80/80 | Main reported completed=80, executed=80, exit 0; `tmp/stress-repro/reload-firewall-prefix-host-bits-keeps-active-135c4b66c9c331ba1adfeabe66d7aecc5c263f8c11a871ff93afd031c71e38ae-20261115-144321-473309996.log` contains 80 successful invocation records |

The three cases were promoted unchanged to their planned `test/parse/` and
`test/reload/` paths after 80-invocation stress and producer-cut red/restored-green
proof. Their draft copies no longer exist. The repository config compatibility
result for the affected firewall inputs is recorded below; the overall corpus
is RED. Final promoted parse checks and the equivalent typed-error assertion's
unit check subsequently passed; independent review is recorded below. The
operator-facing news fragment is prepared at
`website/changes/fragments/firewall-prefix-normalize.md`; no closure is claimed.

A-3 corpus compatibility, main-thread execution on 2026-11-15:
`tmp/session/2026-11-15-125434/scratch/job-unit-pkg-435698e6.log` records
3294 Ze configs and 242 migrated ExaBGP configs, 51 declared refusals checked,
658 exit-refused skips, 29 load sequences (71 steps), and 80 legacy fixtures
skipped. The Darwin corpus is **RED**, with 31 findings: 27 interface backend
diagnostics, two RFC 8212 policy diagnostics in the DDOS orphan fixture, and
two Linux-only XFRM fragment parse failures. None reports a firewall prefix
refusal. Skipped populations are not claimed validated.

These findings do not mask an affected prefix rejection. In
`cmd_validate.go`, `runValidation` only returns before plugin verification for
parse errors; other diagnostics accumulate. In `plugin_verify.go`,
`verifyTree` calls `VerifyPluginConfigMapTransition` first, whose registration
loop appends a verifier error and continues, including firewall's registered
`VerifyConfig`. None of the 27 interface-failing carriers or the DDOS
`running.conf` contains a firewall root. The two unparsed XFRM sequences were
checked at their producer: `tapeSequences` uses `showcase-lab/ze.conf` as base;
`ipsec/demo.tape` merges VPN fragments before `4-xfrm.conf`, while
`showcase/demo.tape` also merges BGP/RPKI/IRR/BFD/OSPF fragments beforehand.
Every base and preceding fragment was inspected, and none contains firewall
config; `4-xfrm.conf` itself contains only interface and static roots.
`judgeSequence`/`load` therefore stop before validation of an unaffected
configuration. A-3 is confirmed for the affected firewall population, not as
a whole-corpus pass. The earlier reviewer prefix census has no retained
artifact and is not presented as a second execution receipt.

Functional discrimination, main-thread observations on 2026-11-15:

| Producer cut | Actual failure observed | Restoration state |
|--------------|-------------------------|-------------------|
| Only the host-bit guard disabled with an explicit mutation marker | Offline refusal draft failed: expected exit 1, got exit 0 and `configuration valid` | Exact original parser bytes restored |
| Same guard cut, rebuilt own-kernel guest artifacts | Live draft failed because the refused candidate changed the persisted active pointer from `20261115-150307.299` to `20261115-150307.658` | Guest stopped; exact original parser bytes restored |

Restored `config.go` SHA-256 is
`dcfeeb69c9ac90c9c61094d7fd8fc4d7e3d905a5002e1505fc9ef25563a1a522`.
The actual source has the `prefix != network` guard and no mutation marker.
Main then observed restored-source parse drafts PASS 2/2, selected existing
parse controls PASS 2/2 (`firewall-nft-accepts-basic` and
`firewall-arp-ip-header-match-rejected`), and the selected own-kernel live
reload case PASS 1/1 in 3.2s. The guest stopped after that run. The source-cut
red/restored-green proof is complete; two invalid runner-selection attempts
produced no evidence and are not counted as tests. The promoted parse cases
still need their final main-thread execution.

Final own-kernel compatibility checks, reported by the supervising parent from
peer main session `01a1df4b` on 2026-11-15: the promoted
`firewall-prefix-host-bits-keeps-active` case passed 1/1 in 4.7s with zero
skips, `firewall-match-in-set-addr` passed 1/1 in 4.8s, and `firewall-reload`
passed 1/1 in 2.8s. Each used Ze `7.3.0-ze`; every check, command and lab
returned exit 0 and all VMs stopped. These are actual main-thread receipts,
not child executions. A subsequent lint-only modernization replaced the
contextual unit test's `errors.As` assertion with equivalent
`errors.AsType[*opererr.Error]`; the error context and exact final repair-line
assertions are unchanged. Only that unit assertion and scoped lint need their
own final checks; the unchanged functional assertions do not require another
kernel or stress run. Final Linux lint then requested a dedicated name for
the fixture chain: `firewallPrefixInputChain = "input"` now supplies the
identical path element, without coupling it to an unrelated payload field.

Implementation phase handoff, 2026-11-15: the supervising parent reports all
functional targets passed. Production guard, supported-input behavior and
functional assertions are frozen; only equivalent typed-error assertion and
semantic constant naming changes followed those proofs. Independent closure
owns the final scoped unit/lint recheck, Review Gate and required commits.
Status remains `in-progress`: this spec did not declare `Handoff | verify`,
and the implementing agent does not mark closure complete. Per-spec state
carries exact paths, earlier run receipts, retained evidence and cleanup
conditions. The agent releases its claim for the supervising closure thread.

### Bugs Found/Fixed
- Literal prefixes with host bits are now refused at `parseAddressMatch`;
  `TestParseAddressMatchRejectsHostBits` covers both directions and families.
- Independent closure found a missing paragraph-local source anchor in the
  guide. The closure added the existing parser anchor immediately after that
  paragraph. This is a documentation NOTE, with no behavior or assertion change.

### Documentation Updates
- `2e68f799f4` updates both address-leaf descriptions in
  `yang/ze-firewall-conf.yang`, the literal-prefix paragraph in
  `docs/guide/firewall.md`, and the parser contract in
  `docs/architecture/core-design.md`. The guide and design anchors name
  `config.go` `parseAddressMatch` and `engine.go` `VerifyConfig`,
  `parseConfigSections`, and `runEngine`.
- Closure adds the guide's paragraph-local parser anchor through the single
  hunk in `firewall-guide-provenance.patch`. Main's final `./le doc check all`
  returned exit 1 with four anchors in other IRR pages; none names this work's
  guide, architecture paragraph or YANG descriptions. The complete output is
  retained in `job-firewall-closure-doc-check-f119ecea.log` under the main
  session's scratch. This result supersedes the earlier truncated receipt.

### Deviations from Plan
- A seventh unit control, `TestParseFirewallConfigCanonicalPrefix`, verifies the
  full parser accepts the valid contextual fixture. No planned assertion was removed.
- Selected guest-root reload stress uses serial admission under CPU burners,
  matching the existing suite contract; the failed parallel experiment is not
  counted as accepted proof.

### Risks and observations
- R-1 remains intentional: a previously accepted host-bit literal is now refused;
  the error supplies its effective network and exact replacement statement.
- R-2 and R-3 are covered by canonical, named-set and wrapped-error controls.
- The full Darwin config corpus remains RED on unrelated interface, host and
  DDOS findings. Final scoped lint has no finding in the owned parser, unit test
  or observer, but is RED on other fixture files. Neither is a whole-tree pass.
- This changes only config acceptance, not RFC-level protocol behavior or RFC
  tags; no RFC stem or discrimination record changes. Interop is not applicable.
- Light checks remain RED on three foreign IRR numeric-parse findings, six
  foreign test-weakening findings, and four foreign IRR documentation anchors.
  The exact findings and owners' retained output are in the per-spec state.
  No such finding names this change's code, tests or documentation.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (independent closure)
Scope declared before review: the complete eleven-file implementation in
`2e68f799f4` and `2c5fc3d689`, including all seven reviewable Go, YANG and CI
paths. Lenses: logic and wiring; security and boundary cases; Go style and
simplicity. Historical owned documentation hunks are reviewed independently of
the current foreign IRR edits. No product change is authorized by this review.

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Guide paragraph lacked its immediate parser source anchor | `docs/guide/firewall.md`, literal prefixes | Added one source anchor; no code change |

### Fixes applied
- Documentation provenance only; no BLOCKER or ISSUE found and no production or test edit.

### Final clean result
All three lenses found **0 BLOCKER, 0 ISSUE** over the full implementation.
The guard fails closed with no match; its input is exercised through the real
offline CLI and live transaction. Existing canonical and named-set paths remain
reachable. The observer fences reload completion before comparing running,
persisted and installed state. All changed Go files received the full style
pass: no reachable panic, no new goroutine, bounded polling and fixed tree walk,
closed read-only store, meaningful constants and no new lifecycle API.

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/firewall-prefix-normalize-01a1df4b-40c0-7350-819b-b5f74e6b5a86.md` |
| `./le spec review check` | OK: 7 code files, clean, hashes match |
| Rounds | 1; documentation NOTE does not reopen a round |
| Reviewer lenses used | Logic/wiring; security/boundaries; Go style/simplicity |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| none | - | No BLOCKER or ISSUE | Complete historical implementation | No code fix required |

### Final status
- [ ] `/ze-review` final run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and passing test
- [ ] Wiring Test table complete
- [ ] `/ze-review` gate clean
- [ ] Scoped package tests and the named functional cases pass; no whole-tree verification is owed here
- [ ] Feature code integrated (`internal/*`)
- [ ] Documentation Update Checklist answered

### Quality Gates (SHOULD pass)
- [ ] Implementation Audit complete

### TDD
- [ ] Tests written
- [ ] Tests FAIL (recorded above)
- [ ] Tests PASS (recorded above)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Refuse host-bit literals without changing input | Done | `config.go`, `parseAddressMatch` | IPv4/IPv6; both leaves |
| Preserve context and corrected statement | Done | `config.go`, `ParseFirewallConfig`, `parseTable`, `parseChain`, `parseTerm`, `parseFromBlock` | `%w` keeps typed error and final repair line |
| Preserve active state and permit corrected reload | Done | `engine.go`, `runEngine`; `firewall_prefix_reload.go`, `firewallPrefixReload` | Live kernel/config/persistence proof |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestParseAddressMatchRejectsHostBits`, `TestVerifyConfigRejectsPrefixHostBits`, refusal CLI case | Both IPv4 leaves |
| AC-2 | Done | Same tests and refusal CLI case | Both IPv6 leaves |
| AC-3 | Done | `TestParseAddressMatchPreservesCanonical`, `TestParseFirewallConfigCanonicalPrefix`, positive CLI case | Defaults and host routes preserved |
| AC-4 | Done | `TestParseAddressMatchKeepsUnsupportedFormsRejected` | Empty, malformed, bare, range, negation and invalid lengths |
| AC-5 | Done | `TestParseAddressMatchPreservesNamedSet`, positive CLI and existing kernel named-set control | Existing resolution unchanged |
| AC-6 | Done | `firewallPrefixReload`, promoted reload case | Refusal preserves running/persisted/kernel; corrected reload advances active pointer |
| AC-7 | Done | `TestParseFirewallConfigPrefixErrorContext` | Wrapped `opererr.Error`, configured names and final statement |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestParseAddressMatchRejectsHostBits` | Done | `config_prefix_test.go` | Matrix rejects both directions/families |
| `TestParseAddressMatchPreservesCanonical` | Done | `config_prefix_test.go` | Exact typed prefix equality |
| `TestParseAddressMatchPreservesNamedSet` | Done | `config_prefix_test.go` | Exact set name and field |
| `TestParseAddressMatchKeepsUnsupportedFormsRejected` | Done | `config_prefix_test.go` | Existing refusal controls |
| `TestParseFirewallConfigPrefixErrorContext` | Done | `config_prefix_test.go` | Typed error and exact final line |
| `TestParseFirewallConfigCanonicalPrefix` | Done | `config_prefix_test.go` | Additional full-parser positive control |
| `TestVerifyConfigRejectsPrefixHostBits` | Done | `config_prefix_test.go` | Registered verifier entry |
| `firewall-prefix-host-bits-rejected` | Done | `test/parse/firewall-prefix-host-bits-rejected.ci` | Promoted CLI case PASS |
| `firewall-prefix-canonical-accepted` | Done | `test/parse/firewall-prefix-canonical-accepted.ci` | Promoted CLI case PASS |
| `firewall-prefix-host-bits-keeps-active` | Done | `test/reload/firewall-prefix-host-bits-keeps-active.ci` | Promoted own-system kernel case PASS |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/component/firewall/config.go` | Done | `2e68f799f4` |
| `internal/component/firewall/config_prefix_test.go` | Done | Both implementation commits |
| `internal/component/firewall/yang/ze-firewall-conf.yang` | Done | `2e68f799f4` |
| `docs/guide/firewall.md` | Done | `2e68f799f4`; closure anchor hunk |
| `docs/architecture/core-design.md` | Done | `2e68f799f4` |
| `internal/test/fixture/firewall_prefix_reload.go` | Done | `2c5fc3d689` |
| `test/parse/firewall-prefix-host-bits-rejected.ci` | Done | `2c5fc3d689` |
| `test/parse/firewall-prefix-canonical-accepted.ci` | Done | `2c5fc3d689` |
| `test/reload/firewall-prefix-host-bits-keeps-active.ci` | Done | `2c5fc3d689` |
| `website/changes/fragments/firewall-prefix-normalize.md` | Done | `2c5fc3d689`; six headings and Firewall topic |
| This spec | Done | Implementation evidence and independent closure record |

### Audit Summary
- Total items: 31 (3 Task requirements, 7 ACs, 10 tests, 11 files).
- Done: 31.
- Partial: 0.
- Skipped: 0.
- Changed: one additional positive unit control; no planned item removed.

## Goal Validation (BLOCKING)
| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Refuse host bits without silently changing input | Functional | Promoted negative CLI case PASS for both leaves/families; positive CLI case PASS. Guard cut made negative validation return exit 0; restoration returned PASS |
| Name configured context, effective network and corrected statement | Functional and unit | CLI case asserts rendered corrections; full-package PASS includes wrapped-error test with exact final line and `errors.AsType` |
| Preserve active state on refused reload and accept correction | Functional | Promoted live case PASS 1/1, zero skips on Ze 7.3.0-ze; observer compares all state surfaces and checks corrected install. Guard cut changed active pointer and failed this same observer |

## Work Not Done
| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None | All agreed Task and AC items implemented and demonstrated | No destination required |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/test/fixture/firewall_prefix_reload.go` | Yes | `git ls-files` printed this exact tracked path |
| `test/parse/firewall-prefix-host-bits-rejected.ci` | Yes | `git ls-files` printed this exact tracked path |
| `test/parse/firewall-prefix-canonical-accepted.ci` | Yes | `git ls-files` printed this exact tracked path |
| `test/reload/firewall-prefix-host-bits-keeps-active.ci` | Yes | `git ls-files` printed this exact tracked path |
| `website/changes/fragments/firewall-prefix-normalize.md` | Yes | `git ls-files` printed this exact tracked path |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | IPv4 refusal | Read `parseFromBlock` calls for both leaves and `parseAddressMatch` non-masked comparison; full firewall package `ok .../firewall 2.228s` |
| AC-2 | IPv6 refusal | Same shared comparison; both IPv6 correction assertions in actual negative CLI file |
| AC-3 | Canonical values preserved | Parser returns original typed prefix when equal to `Masked`; exact equality controls present; positive CLI selection PASS |
| AC-4 | Unsupported forms refused | `ParsePrefix` error returned before comparison; unit matrix includes both prefix-length boundaries |
| AC-5 | Named sets preserved | Existing `@` branch returns before literal parse; positive CLI case and existing kernel set case PASS |
| AC-6 | Failed transaction leaves active state | Read `runEngine` before pending mutation and `liveStagesReload` generation fence; promoted own-system reload PASS 1/1 |
| AC-7 | Context and repair survive wrappers | Read `%w` chain and `opererr.Error.Error`; full-package PASS includes exact final repair assertion |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate` | `test/parse/firewall-prefix-host-bits-rejected.ci` | Four real invocations assert exit 1 and corrected source/destination statements; registered `VerifyConfig` reaches parser |
| Valid config correction | `test/parse/firewall-prefix-canonical-accepted.ci` | Real CLI returns exit 0 and `configuration valid`, including named sets/defaults/host routes |
| Live SIGHUP transaction | `test/reload/firewall-prefix-host-bits-keeps-active.ci` | Actual external observer invokes reload, waits for processed generation, compares three state surfaces and proves correction commits |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | Recorded owner answer B and approved refusal design |
| A-2 | confirmed | `parseFromBlock` routes both leaves through one guard; offline/startup/live paths read at producer; entry-point cases passed |
| A-3 | confirmed for affected firewall inputs | Corpus refusal/verifier loop triage above, full firewall package and existing parse/kernel controls; overall Darwin corpus RED and skipped inputs not claimed validated |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| User guide and config acceptance: Yes | Guide literal-prefix paragraph and both YANG descriptions match `parseAddressMatch`; source anchors verified; one missing paragraph-local anchor added | Yes |
| Architecture contract: Yes | `2e68f799f4` core-design hunk matches parser, `VerifyConfig`, `parseConfigSections`, `runEngine` | Yes |
| Existing examples: Yes, revalidated | Parser-backed corpus triage and existing positive CLI/kernel controls; no affected firewall refusal; corpus skips remain excluded | Yes, scoped |
| Feature level/scope and comparison: No | Existing `features/firewall.md` declares address matching; no new feature, backend or supported match type added | No declaration change needed |
| CLI/API/RPC/plugin SDK/wire/RFC/route metadata/metrics: No | Complete diff adds no command, schema path, RPC, event, SDK type, packet encoding or counter; existing source anchors outside literal input contract remain valid | No corresponding update needed |
| Test infrastructure/agent discovery/doctor: No | Observer registers in existing fixture registry and uses existing kernel/store/RPC dependencies; no runtime dependency or harness API added | No corresponding update needed |
| Final `./le doc check all` | Main retained exit 1: four foreign IRR anchors (`resolve.md` Open, `irr-filtering.md` clearResolverCache/readmitRetryInterval, `plugins.md` ConfigReads); all other stages pass | Owned documentation verified; no whole-tree green claimed |
