# Spec: firewall-prefix-normalize

| Field | Value |
|-------|-------|
| Status | in-progress |
| Priority | P1 |
| Release reason | Existing accepted firewall prefixes produce incorrect config diffs and non-idempotent reconciliation; installed match semantics are not claimed broken. |
| Depends | - |
| Phase | 4/4 - independent closure ready |
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
| Config ↔ firewall model | `parseAddressMatch` refuses host bits | Source read; execution owed |
| Offline and live config ↔ parser | `VerifyConfig`, startup and transaction verify | Source read; entry-point tests owed |
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
| `TestParseAddressMatchRejectsHostBits` | `internal/component/firewall/config_prefix_test.go` | Both families and both leaf directions; AC-1, AC-2 | owed |
| `TestParseAddressMatchPreservesCanonical` | `internal/component/firewall/config_prefix_test.go` | AC-3, unchanged types and values | owed |
| `TestParseAddressMatchPreservesNamedSet` | `internal/component/firewall/config_prefix_test.go` | AC-5, unchanged set field and name | owed |
| `TestParseAddressMatchKeepsUnsupportedFormsRejected` | `internal/component/firewall/config_prefix_test.go` | AC-4 | owed |
| `TestParseFirewallConfigPrefixErrorContext` | `internal/component/firewall/config_prefix_test.go` | AC-7, operator error type and final statement | owed |
| `TestVerifyConfigRejectsPrefixHostBits` | `internal/component/firewall/config_prefix_test.go` | Full verifier registration input refuses both leaf directions before backend apply | owed |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| prefix length v4 | 0-32 | 32; canonical /0 and host-bit /0 are both covered | -1 | 33 |
| prefix length v6 | 0-128 | 128; canonical /0 and host-bit /0 are both covered | -1 | 129 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `firewall-prefix-host-bits-rejected` | `test/parse/firewall-prefix-host-bits-rejected.ci` | AC-1, AC-2, offline registration and correction text | owed |
| `firewall-prefix-canonical-accepted` | `test/parse/firewall-prefix-canonical-accepted.ci` | AC-3, AC-5, positive offline path | owed |
| `firewall-prefix-host-bits-keeps-active` | `test/reload/firewall-prefix-host-bits-keeps-active.ci` | AC-6, refused transaction, unchanged persisted and installed state, successful corrected reload | owed; Linux kernel backend required |

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
| Input validation | malformed prefixes still rejected by `ParsePrefix` |
| Correctness | No invalid candidate reaches apply and no silent replacement of operator input occurs |

## Mistake Log
### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Four selected reload invocations could share one guest kernel safely | The reload suite declares serial guest-root execution, and each copy owns the same nft table name | Invocation 3 of the first selected stress failed after another copy completed; `selectedStressPhase`, `flushOnStop` and nft `shouldDeleteTable` expose the collision | Preserve assertions; run selected guest-root stress serially under CPU burners. This is test admission, not a parser behavior change |

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
is RED. Final promoted parse checks, the equivalent typed-error assertion's
unit/lint checks and independent closure review remain outstanding. The
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

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate: -->
<!-- the final review before closure, run AFTER the inline critical/security/doc reviews, over the complete diff. -->
<!-- Every BLOCKER and ISSUE (severity > NOTE) must be fixed, then re-run /ze-review. -->
<!-- Loop until the review returns 0 BLOCKER/0 ISSUE (only NOTEs, or nothing). Paste the final clean run. -->
<!-- NOTE-only findings do not block — record them and proceed. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| - | - | Review not started | - | Owed after implementation |

### Fixes applied
- None; implementation has not started.

### Run 2+ (re-runs until clean)
<!-- Add a new block per re-run. Final run MUST show zero BLOCKER/ISSUE. -->
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

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
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior
