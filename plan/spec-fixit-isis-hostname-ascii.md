# Spec: fixit-isis-hostname-ascii

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | protocol |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/fixit-isis-hostname-ascii.md` |
| Updated | 2026-07-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze advertises the IS-IS Dynamic Hostname TLV 137 with no character restriction.
RFC 5301 section 3 says the value is encoded in 7-bit ASCII. The encoder writes a
bare byte conversion of the configured string. A UTF-8 hostname therefore reaches
a peer as 8-bit octets. This is a live wire defect.

`plan/spec-rfcgate-4-ledger.md` re-extracted RFC 5301 on 2026-07-30. The
extraction declares 7 gated MUST-level requirements. Three are not met:

1. `RFC5301-3-7` (7-bit ASCII) is **violated on emit**.
2. `RFC5301-3-9` (the value is a domain name per RFC 2181) is **absent**.
3. `RFC5301-3-10` (IDNA ToASCII / ToUnicode duty of the user-interface) is **absent**.

The debt is declared today at `rfc/not-enrolled.txt`, kind `backlog`. That row
is the reason `rfc5301` is absent from `rfc/enrolled.txt`. The stem cannot enrol
until all three requirements are met. Enrolment is the only way a row leaves
`rfc/not-enrolled.txt`.

**Goal.** Meet all three requirements, then enrol `rfc5301` and keep
`make ze-rfc-check` at exit 0.

**Owner ruling (Thomas, 2026-07-30).** Fix all three now. Reject non-conforming
input at config time. Do not sanitise at emit. He chose this over two narrower
options:

- Fixing only the wire defect and leaving `-3-9` and `-3-10` open.
- Sanitising the value at emit instead of rejecting it at the config boundary.

The accepted reasoning is `ai/rules/protocol.md`. A backend that cannot
deliver the operator's request exactly must fail at verify or commit with a clear
error. It must never silently approximate. Sanitising at emit would put a
different hostname on the wire than the operator configured.

**Accepted cost.** An operator who has a non-ASCII hostname configured today gets
a validation error on their next commit. That is the honest failure.

## RFC 5301 Section 3 IDNA Reading Question (READ THIS, DO NOT ANNOTATE IT AWAY)

`RFC5301-3-10` is conditional. The RFC text is:

> If a user-interface for configuring or displaying this field permits Unicode
> characters, that user-interface is responsible for applying the ToASCII and/or
> ToUnicode algorithm as described in [RFC3490] to achieve the correct format for
> transmission or display.

The antecedent is "permits Unicode characters". AC-1 makes Ze's config surface
**refuse** Unicode. Two readings are then honest.

| Reading | Statement | Consequence for Ze |
|---------|-----------|--------------------|
| A (antecedent false) | The duty attaches only to a UI that permits Unicode. A UI that refuses Unicode never enters the conditional. | Ze owes no ToASCII. The requirement is met by the refusal itself. |
| B (duty to convert) | The sentence assigns the UI responsibility for producing the correct transmission format. A UI that refuses pushes the conversion onto the operator. | Ze owes a ToASCII conversion so an operator can configure a Unicode name. |

**This implementation takes Reading A.** The basis is `ai/rules/protocol.md`.
ToASCII is a silent rewrite of the operator's configured value. A configured
`café.example` would travel as `xn--caf-dma.example` and would read back
differently in `show isis hostname`. That is the approximation the rule bans.
Refusal is the honest failure, and it is what the owner ruled.

**If the implementer disagrees with Reading A, the decision belongs to Thomas.**
`ai/rules/rfc-compliance.md` reserves any answer short of full compliance to the
owner. Ask which way to fix it. Never ask whether it can be skipped.

**Recording either reading as `{gap}` or `{not-applicable}` is forbidden.**
`ai/rules/rfc-compliance.md` voids those as authority for an RFC obligation. The
summary row must state the reading, not annotate the requirement away.

**Reading B has a prerequisite that does not exist yet.** `rfc/full/rfc3490.txt`
is absent from this checkout, and no RFC 3490 summary exists under `rfc/short/`.
Implementing ToASCII without the normative text would breach
`ai/rules/protocol.md`. Fetch the text first if Thomas selects Reading B.

## The Config-Surface Narrowing (explicit, by design)

AC-1 adds a YANG `pattern` to a leaf that today accepts any string of 1 to 255
bytes. The narrowing is deliberate and it is the point of the owner ruling.

| Question | Answer |
|----------|--------|
| What is accepted after this change? | Printable 7-bit ASCII, octets `0x20` to `0x7e`, subject to RFC 2181 label rules in AC-3. |
| What stops validating? | Any hostname carrying an octet outside that set. Control characters, `0x7f`, and every multi-byte UTF-8 character. |
| Who is affected? | An operator whose `isis { hostname ... }` value is not printable 7-bit ASCII. |
| What does that operator see? | A validation error on `ze config validate` and on commit. The config is refused, not silently altered. |
| Does any current test break? | No. A byte-level sweep found no non-ASCII IS-IS hostname anywhere in the repository. See A-1. |

**The error message is part of the deliverable.** `ai/rules/cli.md`
requires the message to name the offending value and the accepted shape. The raw
YANG pattern failure at `internal/component/config/yang/validator.go` renders
`value %q does not match pattern %q`. It names the value but reports a regex as
the remediation. That is not actionable. AC-2 therefore requires a custom
validator message that states:

1. The offending value, quoted.
2. The offending octet or label, and its position.
3. The accepted shape in words, not as a regex.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - how a `ze:validate` custom validator is declared and registered
  → Constraint: a validator name used in YANG must be registered, or the startup integrity check at `internal/component/config/yang/validator_registry.go` fires.
  → Decision: the new check is a registered custom validator, not ad-hoc Go in the plugin's parser.

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc5301.md` - the extracted checklist and the 7 gated MUST rows
  → Constraint: the gated rows are `RFC5301-3-4` through `RFC5301-3-10`. Three of them are unmet and are this spec's subject.
  → Constraint: `rfc/short/rfc5301.md` records why indicative prose was extracted at MUST level. Do not re-litigate that grade.
- [ ] `rfc/short/rfc2181.md` - the normative reference behind `RFC5301-3-9`
  → Decision: RFC 2181 section 11 places only a LENGTH restriction on labels. It forbids an implementation from restricting label characters. `-3-9` is therefore a label-structure rule, not a character-set rule.
  → Constraint: labels are 1 to 63 octets. A full name is at most 255 octets including separators.

**Key insights:** (minimal context to resume after compaction)
- The character-set rule comes from `-3-7`. The label-length rule comes from `-3-9`. They are two requirements with two enforcement points.
- RFC 5301 section 3 also says the value "can be any string operators want to use for the router". An LDH letter-digit-hyphen rule would reject strings the RFC permits. Do not invent one.
- YANG `pattern` is auto-anchored by the implementation. A self-anchored pattern is rejected.

## Current Behavior (MANDATORY)

**Source files read:** (read these BEFORE you write this spec)
- [ ] `internal/plugins/isis/lsdb/encode.go` - `hostnameTLV` converts the name with a bare byte conversion at line 60, then truncates at 255 bytes. No character check.
- [ ] `internal/plugins/isis/yang/ze-isis-conf.yang` - the `hostname` leaf is `type string { length "1..255"; }`. No `pattern`. No `ze:validate`.
- [ ] `internal/plugins/isis/config.go` - `cfg.Hostname` is filled by `configString`, a bare type assertion at `internal/plugins/isis/config.go`. No validation.
- [ ] `internal/plugins/isis/config.go` - `validateConfig` checks NET presence and system-id agreement only. It does not read `Hostname`.
- [ ] `internal/plugins/isis/register.go` - `OnConfigVerify` parses, then calls `validateConfig`.
- [ ] `internal/plugins/isis/lsdb_wiring.go` - `nodeInfo` copies `cfg.Hostname` into `NodeInfo.Hostname`.
- [ ] `internal/plugins/isis/lsdb/origination.go` - `fixedTLVs` calls `hostnameTLV` when the name is not empty.
- [ ] `internal/plugins/isis/packet/tlv_core.go` - the codec comment already claims the value "is a 1..255 byte 7-bit-ASCII hostname". Line 220 says the caller ensures the bound. Nothing produces that guarantee.
- [ ] `internal/plugins/isis/show.go` - `sanitizeHostname` keeps octets `0x20` to `0x7e` on the DISPLAY path only.
- [ ] `internal/component/config/yang/validator.go` - `length` is measured with a byte count, not a rune count.
- [ ] `internal/component/config/yang/validator.go` - `pattern` is enforced here, through `MatchPattern`.
- [ ] `internal/component/config/yang/pattern.go` - a pattern is wrapped as a whole-string anchored non-capturing group.
- [ ] `internal/component/config/yang/validator.go` - `applyCustomValidators` runs a `ze:validate` function and reports its own error text.
- [ ] `internal/component/config/validators_register.go` - `isis-net` and `isis-system-id` are registered centrally. This is the precedent for a third IS-IS validator.

**Behavior to preserve:** (unless the user explicitly said to change it)
- `sanitizeHostname` at `internal/plugins/isis/show.go` keeps octets `0x20` to `0x7e` and drops the rest. `internal/plugins/isis/show_test.go` asserts that exact behavior. The RECEIVE side stays lenient. RFC 5301 section 4 lets a receiver ignore or accept the TLV, and a peer's malformed advertisement must not break the CLI.
- TLV 137 wire framing: type 137, a 1-octet length, and no NUL terminator. `RFC5301-3-4`, `RFC5301-3-5` and `RFC5301-3-8` are already proven in both directions. Do not disturb them.
- An empty hostname omits the TLV, per `internal/plugins/isis/lsdb/origination.go`.
- Every existing hostname fixture keeps validating. The seven config fixtures and five Go test values are listed in A-1.
- The byte-count semantics of `length` at `internal/component/config/yang/validator.go`. It matches the 255-octet TLV cap exactly.

**Behavior to change:** (only what the user asked for)
- The config boundary refuses a hostname that is not printable 7-bit ASCII.
- The config boundary refuses a hostname whose label structure breaks RFC 2181 section 11.
- `rfc5301` moves from `rfc/not-enrolled.txt` to `rfc/enrolled.txt`.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator's `isis { hostname <value> }` statement in a config file, reaching validation through `ze config validate`. The isis section is walked because `internal/component/config/cli/cmd_validate.go` lists it.
- The same statement reaching the running daemon through a commit or a reload, arriving at `OnConfigVerify` at `internal/plugins/isis/register.go`.
- The same leaf reaching the interactive editor through a `set` command, validated by `validatePatterns` at `internal/component/config/schema.go`.

### Transformation Path
1. The config text is parsed into a tree, and the `hostname` leaf becomes a string value in that tree.
2. YANG native validation runs. `internal/component/config/yang/validator.go` applies the byte-length bound. `internal/component/config/yang/validator.go` applies each `pattern`.
3. `applyCustomValidators` at `internal/component/config/yang/validator.go` runs the `ze:validate` function and records its error text verbatim.
4. `parseISISConfig` reads the tree. `internal/plugins/isis/config.go` copies the leaf into `cfg.Hostname`.
5. `validateConfig` at `internal/plugins/isis/config.go` applies the plugin's own required-field policy.
6. `nodeInfo` at `internal/plugins/isis/lsdb_wiring.go` copies `cfg.Hostname` into `NodeInfo.Hostname`.
7. `fixedTLVs` at `internal/plugins/isis/lsdb/origination.go` builds TLV 137 by calling `hostnameTLV`.
8. `hostnameTLV` at `internal/plugins/isis/lsdb/encode.go` produces the value octets.
9. `writeHostnameTLV` at `internal/plugins/isis/packet/tlv_core.go` writes type, length and value into the LSP buffer.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config file → YANG validator | tree map, string leaf value | Yes - `internal/component/config/cli/cmd_validate.go` lists the isis section |
| YANG validator → custom validator | `ze:validate` name lookup in the central registry | Yes - `internal/component/config/yang/validator.go` |
| Config tree → IS-IS plugin | `sdk.ConfigSection` delivered to `OnConfigVerify` | Yes - `internal/plugins/isis/register.go` |
| Plugin config → LSDB originator | `NodeInfo` value struct | Yes - `internal/plugins/isis/lsdb_wiring.go` |
| Originator → wire | `packet.TLV` value octets | Yes - `internal/plugins/isis/lsdb/encode.go` |
| Ze → foreign daemon | TLV 137 inside a flooded LSP | Yes - FRR decodes it, `test/interop/scenarios/isis-p2p-frr/check.py` |

### Integration Points
- `internal/component/config/validators_register.go` - the registration function the new validator joins, beside `isis-net` and `isis-system-id`.
- `internal/component/config/validators.go` - `ISISNETValidator`, the shape the new validator copies.
- `internal/plugins/isis/register.go` - where the plugin registers its own completion functions, if completion is wanted later.
- `rfc/short/rfc5301.md` - the `RFC5301-3-7` checklist row that the new tests bind.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The check sits at the config boundary, the same layer that already rejects a bad NET and a bad system-id |
| No unintended coupling (components stay isolated) | Yes | The validator lives in the central config package because config cannot import isis without a cycle, per `internal/plugins/isis/register.go`. That is the existing arrangement for `isis-net`, not a new coupling |
| No duplicated functionality (extends existing, does not recreate) | Yes | It reuses the `ze:validate` mechanism and the `pattern` mechanism. No new validation framework |
| Zero-copy preserved where applicable (refs, not copies) | Yes | Nothing on the per-UPDATE or per-LSP hot path changes. Validation runs once per config load |
| Registration over hardcoding: new commands, views, families and handlers register, and the core discovers them. No per-feature field, switch case or factory reaches a core or shared package (`ai/rules/plugins.md`) | Yes | The validator is added by a `reg.Register` call in the existing registration list. No switch case, no new field, no factory |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | No existing test or fixture uses a non-ASCII IS-IS hostname, so the narrowing breaks nothing | A byte-level sweep found zero non-ASCII files under `internal/plugins/isis/`, `test/isis`, `test/isis-wire` and every `test/interop/scenarios/isis-*`. The 7 config fixtures are `test/isis/isis-config.ci` (`r1`), `test/isis/isis-show.ci` (`r1-isis`), and `ze.conf` in `isis-p2p-frr` (`ze-p2p`), `isis-lan-dis-frr` (`ze-lan`), `isis-auth-frr` (`ze-auth`), `isis-convergence-frr` (`ze-conv`), `isis-dualstack-frr` (`ze-ds`). Go values are `r1`, `ze-router`, `snap-node`, `router-a`, `host-a`, `peer-host` | The spec ships a test-breaking change and the fixtures need updating first | Re-run the sweep, then `make ze-isis-test` and `make ze-verify` | unvalidated |
| A-2 | YANG `length` counts bytes, so `length "1..255"` already matches the 255-octet TLV cap | `internal/component/config/yang/validator.go` uses a byte count. No rune counting exists in `internal/component/config/` | A 255-rune multi-byte name would pass validation and then be cut mid-character at `internal/plugins/isis/lsdb/encode.go` | Read the line, then a unit test feeding a 256-byte name | unvalidated |
| A-3 | A YANG `pattern` must NOT be self-anchored, and `\p{...}` classes are unavailable | `internal/component/config/yang/pattern.go` wraps the pattern as a whole-string anchored group. `internal/component/config/yang/pattern.go` rejects a bare `^` or `$` outside a character class. `internal/component/config/yang/pattern.go` rejects the `\p` escape | The chosen pattern fails to compile and the leaf silently accepts everything | Author the pattern, then a unit test asserting a rejected value really is rejected | unvalidated |
| A-4 | A `ze:validate` function is reached by `ze config validate`, unlike `OnConfigVerify` | `applyCustomValidators` at `internal/component/config/yang/validator.go` runs inside the validator tree walk. `internal/component/config/cli/cmd_validate.go` includes the isis section | The offline check does not fire and only the daemon rejects the value | A `.ci` asserting a non-zero exit from `ze config validate` | unvalidated |
| A-5 | `pattern` reaches the interactive `set` path, and `length` does not | `internal/component/config/schema.go` applies patterns. The `LeafNode` at `internal/component/config/schema.go` carries no length field | An editor `set` accepts a value the file-based validator refuses | A `.et` or `.ci` case driving `set isis hostname` with a rejected value | unvalidated |
| A-6 | `golang.org/x/net/idna` is available if Reading B is ever chosen | `go.mod:26` requires `golang.org/x/net v0.57.0` as a direct dependency, and `vendor/golang.org/x/net/idna` is vendored and listed at `vendor/modules.txt` | Reading B would need a dependency addition, which needs the user's approval per `ai/rules/go-standards.md` | Already read. Confirm before any Reading B work | unvalidated |
| A-7 | Enrolling `rfc5301` needs no new extraction sign-off, because `rfc/extraction/rfc5301.json` already exists and is signed | `rfc/extraction/rfc5301.json` carries `signed-off: 2026-07-30`, and line 5 carries the `register-reason` | Enrolment fails the newly-enrolled sign-off check at `scripts/dev/rfc_requirements.py` | `make ze-rfc-check` after you move the row | unvalidated |
| A-8 | The files this spec reads are committed by the time work starts | Every artifact this spec cites is UNCOMMITTED today. `rfc/not-enrolled.txt`, `rfc/extraction/rfc5301.json`, `rfc/short/rfc5301.md`, `rfc/enrolled.txt` and `docs/features/rfc-status.md` are the working-tree output of `plan/spec-rfcgate-4-ledger.md`, which is still `in-progress` | The cited line numbers move, and `rfc/not-enrolled.txt` does not exist at HEAD. AC-8 cannot be evaluated until the ledger spec lands | `git log -1 -- rfc/not-enrolled.txt` before Phase 1. If the ledger spec has not landed, set `Depends` to `spec-rfcgate-4-ledger` and wait | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Two enforcement points for one rule drift apart. The `pattern` and the custom validator disagree on the accepted set | A value is accepted by one path and refused by the other | One shared table of accepted and refused values drives both, in a single unit test. The validator is authoritative and also re-checks the character set, so removing the pattern cannot open a hole |
| R-2 | Over-restriction. An LDH rule would reject strings RFC 5301 section 3 explicitly permits, and RFC 2181 section 11 forbids restricting label characters | An operator string like `core_1` or a space-bearing name is refused | Restrict only what the RFC restricts. See the Key Design Decisions table. Do not add a letter-digit-hyphen rule |
| R-3 | The interop scenarios do not actually prove the TLV 137 hostname round-trip. The `has_database_lsp("ze-p2p")` assertion at `test/interop/scenarios/isis-p2p-frr/check.py` sits inside an `if not have_route:` fallback branch, so the happy path never asserts the hostname | Reverting the emit change leaves the interop suite green | Move the hostname assertion out of the fallback, or add a dedicated assertion. Then prove it discriminates by reverting the change, per `ai/rules/interop-and-goal-validation.md` |
| R-4 | An accepted hostname containing a space breaks the interactive `set` grammar, which splits on whitespace | `set isis hostname a b` behaves unexpectedly | A `.ci` or `.et` case pins the behavior. If the grammar genuinely cannot carry a space, raise it as a grammar question rather than narrowing the RFC-derived set silently |
| R-5 | The truncation at `internal/plugins/isis/lsdb/encode.go` becomes unreachable from config once the character set is one byte per character. Left undocumented, a later reader can treat silent truncation as intended behavior | A programmatic `NodeInfo` that bypasses config still truncates silently | Keep the bound as a defensive guard, document it as an invariant violation, and pin with a unit test that config can never reach it |
| R-6 | `RFC5301-3-10` under Reading A looks like a requirement satisfied by doing nothing, which a reviewer can read as an unproven MUST | The reviewer asks what code produces the guarantee | The proof is the refusal itself. AC-5 and AC-6 bind both polarities to observable config-boundary behavior, so the requirement is test-bound, not annotated |
| R-7 | The value is 7-bit ASCII on the emit path but the codec comment at `internal/plugins/isis/packet/tlv_core.go` already asserted that before it was true. A comment is not evidence | Another such claim is found and trusted | Per `ai/rules/evidence.md`, the producing function must make the guarantee. Correct the comment to name the enforcing layer |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A valid config is refused, and the operator cannot start or reload IS-IS. The opposite error is quieter and worse. A non-ASCII hostname keeps reaching peers as 8-bit octets |
| How is it reverted? | A single commit revert. Nothing persists, no state migrates, and no peer holds the value beyond an LSP lifetime |
| Who else touches this path? | Concurrent sessions own `internal/component/mcp/`, `internal/test/`, `test/plugin/mcp-*`, `test/plugin/task-*`, `test/chaos-web/`, `plan/spec-mcp2026-*`, and an ASD-STE100 workstream. None of those overlap this spec's files. `internal/component/config/validators.go` is shared with OSPF and BGP validators, so the edit must be additive |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze config validate <file>` with a non-ASCII `isis { hostname }` | → | YANG `pattern` at `internal/component/config/yang/validator.go`, then the new validator through `applyCustomValidators` at `internal/component/config/yang/validator.go` | `test/isis/isis-hostname-ascii.ci` |
| Daemon commit or reload of the same config | → | `OnConfigVerify` at `internal/plugins/isis/register.go` → `parseISISConfig` → `validateConfig` at `internal/plugins/isis/config.go` | `test/isis/isis-hostname-ascii.ci` |
| Interactive `set isis hostname <value>` | → | `validatePatterns` at `internal/component/config/schema.go` | `test/isis/isis-hostname-ascii.ci` |
| An accepted hostname originated into an LSP | → | `fixedTLVs` at `internal/plugins/isis/lsdb/origination.go` → `hostnameTLV` at `internal/plugins/isis/lsdb/encode.go` | `TestISISHostnameTLVIsPrintableASCII` |
| A validator name used in YANG | → | `reg.Register` in `internal/component/config/validators_register.go` | `TestISISHostnameValidatorRegistered` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `isis { hostname }` set to a value carrying any octet outside `0x20` to `0x7e`, including a multi-byte UTF-8 character, a control character, or `0x7f` | `ze config validate` exits non-zero and the daemon refuses the commit. Binds `RFC5301-3-7 positive` |
| AC-2 | The same rejected value | The error names the offending value quoted, names the offending octet and its position, and states the accepted shape in words. No regex is offered as the remediation |
| AC-3 | `isis { hostname }` set to a value that breaks RFC 2181 section 11. An empty interior label, a label of more than 63 octets, or a total of more than 255 octets | The config is refused, and the error names the offending label and the limit it broke. Binds `RFC5301-3-9 positive` |
| AC-4 | `isis { hostname }` set to a printable 7-bit ASCII name. Every label is 1 to 63 octets and the total is at most 255 octets. Cases include a 63-octet label and a single trailing dot | The config validates and the daemon accepts it. The refusal is specific, not a blanket refusal of the leaf. Binds `RFC5301-3-7 negative` and `RFC5301-3-9 negative` |
| AC-5 | `isis { hostname }` set to a Unicode value such as `café.example` | The config is refused at the config boundary. Ze's configuring user-interface does not permit Unicode characters, so the antecedent of RFC 5301 section 3's IDNA sentence is false. Binds `RFC5301-3-10 positive` |
| AC-6 | An accepted printable 7-bit ASCII hostname originated into an LSP | The TLV 137 value octets are byte-identical to the configured string. No ToASCII, no punycode, and no other rewrite is applied on the emit path. Binds `RFC5301-3-10 negative` |
| AC-7 | Any configuration that the config boundary accepts | The emitted TLV 137 value contains no octet outside `0x20` to `0x7e`, and its length is 1 to 255 octets. The truncation at `internal/plugins/isis/lsdb/encode.go` is unreachable from config |
| AC-8 | `make ze-rfc-check` after the work | Exit 0. `rfc5301` appears in `rfc/enrolled.txt` and is absent from `rfc/not-enrolled.txt`. `rfc/extraction/rfc5301.json` remains a valid sign-off. All 7 gated MUST rows are covered by tagged tests or by a recorded reading, and none by `{gap}` or `{not-applicable}` |
| AC-9 | `docs/features/rfc-status.md` and `rfc/short/rfc5301.md` after the work | The row for RFC 5301 records the enforced constraint, with a source anchor to the producing line. It drops the three unmet-obligation claims that are now false |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `ze config validate` on a config whose IS-IS hostname holds a UTF-8 character | config file → tree → YANG pattern → custom validator → non-zero exit with a named value | `test/isis/isis-hostname-ascii.ci` |
| 2 | Commits a config whose IS-IS hostname is a valid FQDN | config → `OnConfigVerify` → `validateConfig` → `setConfig` → origination → TLV 137 on the wire | `test/isis/isis-hostname-ascii.ci` |
| 3 | Reads `show isis hostname` on a peer running FRR after Ze advertises its name | Ze config → TLV 137 → flooded LSP → FRR LSDB → FRR `show isis database` | `test/interop/scenarios/isis-p2p-frr/check.py` |
| 4 | Types `set isis hostname` with a rejected value in the editor | editor `set` → `validatePatterns` → error surfaced in the editor | `test/isis/isis-hostname-ascii.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestISISHostnameValidatorCharset` | `internal/component/config/validators_isis_test.go` | Every octet outside `0x20` to `0x7e` is refused, and every octet inside it is accepted. Tagged `RFC requirement: RFC5301-3-7 positive` and `negative` | |
| `TestISISHostnameValidatorLabels` | `internal/component/config/validators_isis_test.go` | Empty interior label, a 64-octet label, and a 256-octet total are refused. A 63-octet label, a multi-label FQDN, and a single trailing dot are accepted. Tagged `RFC requirement: RFC5301-3-9 positive` and `negative` | |
| `TestISISHostnameValidatorMessage` | `internal/component/config/validators_isis_test.go` | The error text names the offending value, the offending octet or label with its position, and the accepted shape in words | |
| `TestISISHostnameValidatorRegistered` | `internal/component/config/validators_register_test.go` | The name used by the YANG leaf is present in the central registry, so the startup integrity check at `internal/component/config/yang/validator_registry.go` cannot fire | |
| `TestISISHostnameYANGPatternAgreesWithValidator` | `internal/component/config/validators_isis_test.go` | One shared table of values drives both the YANG pattern and the validator, and they agree on every row. Mitigates R-1 | |
| `TestISISHostnameYANGDeclaresPattern` | `internal/plugins/isis/config_test.go` | The leaf in `yang/ze-isis-conf.yang` declares the pattern and the validator name, read from disk. Mirrors the existing `TestISISConfigBoundaries` at `internal/plugins/isis/config_test.go` | |
| `TestISISHostnameTLVIsPrintableASCII` | `internal/plugins/isis/lsdb/encode_test.go` | For every accepted value, the TLV 137 octets are byte-identical to the input and hold no octet outside `0x20` to `0x7e`. Tagged `RFC requirement: RFC5301-3-10 negative` | |
| `TestISISHostnameTLVTruncationUnreachable` | `internal/plugins/isis/lsdb/encode_test.go` | A config-accepted value never exceeds 255 octets, so the truncation branch is unreachable from config. Pins R-5 | |
| `TestISISHostnameUnicodeRefusedNotConverted` | `internal/component/config/validators_isis_test.go` | A Unicode value is refused, and no ToASCII output appears in the accepted set. Tagged `RFC requirement: RFC5301-3-10 positive` | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hostname total length, octets | 1-255 | 255 | 0 (empty string) | 256 |
| single label length, octets | 1-63 | 63 | 0 (empty interior label) | 64 |
| accepted octet value | 0x20-0x7e | 0x7e | 0x1f | 0x7f |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-hostname-ascii` | `test/isis/isis-hostname-ascii.ci` | A UTF-8 hostname is refused by `ze config validate` with a named value. A control character is refused. An over-long label is refused. A valid FQDN validates and the daemon accepts it. An editor `set` with a rejected value fails | |

Draft the file at `test/draft/isis/isis-hostname-ascii.ci` first, per `ai/rules/testing.md`. Run it with the draft flag, prove it under load with the stress reproducer, then promote it into `test/isis/`.

Mutation-verify the test before you claim it gates, per `ai/rules/testing.md`. Remove the pattern and the validator, confirm the test turns red, then restore both.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-p2p-frr` | `test/interop/scenarios/` | FRR | FRR decodes Ze's TLV 137 and renders the hostname in `show isis database`. The assertion must move out of the `if not have_route:` fallback at `check.py` so the happy path asserts it. Addresses R-3 | |

Prove the interop assertion discriminates. Revert the emit-side guarantee, confirm the scenario turns red, then restore it. A conforming peer accepts both wire forms, so an unreverted-green scenario is a vacuous test.

## Files to Modify
- `internal/plugins/isis/yang/ze-isis-conf.yang` - add the `pattern` and the `ze:validate` name to the `hostname` leaf at line 69. Add a `revision` entry. State the accepted shape in the leaf `description`
- `internal/component/config/validators.go` - add the hostname validator beside `ISISNETValidator` at line 211
- `internal/component/config/validators_register.go` - register the new name in the list at line 10
- `internal/plugins/isis/lsdb/encode.go` - document the enforced invariant on `hostnameTLV` at line 57, and correct the truncation comment so silent truncation does not read as intended behavior
- `internal/plugins/isis/packet/tlv_core.go` - correct the comment at line 215 to name the layer that now enforces the 7-bit ASCII guarantee, per R-7
- `internal/plugins/isis/config_test.go` - add the schema-declaration test
- `internal/component/config/validators_isis_test.go` - add the validator tests
- `internal/component/config/validators_register_test.go` - add the registration test
- `test/interop/scenarios/isis-p2p-frr/check.py` - move the hostname assertion out of the fallback branch
- `rfc/short/rfc5301.md` - bind the three rows and record the Reading A decision for `RFC5301-3-10`
- `rfc/enrolled.txt` - add the `rfc5301` row
- `rfc/not-enrolled.txt` - remove the `rfc5301` row at line 34
- `docs/features/rfc-status.md` - update the row for RFC 5301 at line 139
- `docs/guide/isis.md` - document the `hostname` leaf and its accepted shape
- `ai/RFC-REQUIREMENTS.md` - regenerate with `make ze-rfc-index`

## Files to Create
- `internal/plugins/isis/lsdb/encode_test.go` - the emit-side character and length tests
- `test/draft/isis/isis-hostname-ascii.ci` - the draft functional test, promoted to `test/isis/isis-hostname-ascii.ci` when green

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/isis/yang/ze-isis-conf.yang` - no new leaf, a constraint on an existing one |
| YANG validation constraints | Yes | The `pattern` is the native constraint. The `length "1..255"` bound already exists and is a byte bound |
| YANG custom validators | Yes | Native `pattern` cannot produce an actionable message, and the label-structure rule is clearer in Go. The validator name is registered centrally beside `isis-net`, which is the existing arrangement |
| CLI commands/flags | N-A | No command changes. The leaf is reached through `set` and through the config file |
| CLI grammar (keyword before value) | N-A | No new command grammar. See R-4 for the space-in-value question on the `set` path |
| Editor autocomplete | No | A free-form constrained string has no closed value set. A `CompleteFn` can be added later from the plugin side, following `internal/plugins/isis/register.go` |
| Functional test for new RPC/API | Yes | `test/isis/isis-hostname-ascii.ci` |
| Pipe completeness | N-A | No new output surface. `show isis hostname` is unchanged |
| Env var registration | N-A | No `environment/` leaf is added |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, or binary. The failure is a config-validation error, not a runtime dependency |
| Prometheus counters/metrics | N-A | No new observable state |
| BGP family surface (new SAFI / capability / attribute) | N-A | IS-IS work, no BGP family surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | A constraint on an existing leaf, not a feature. Grep `docs/features.md` for an IS-IS hostname claim to confirm |
| 2 | Config syntax changed? | Yes | `docs/guide/isis.md` - the `hostname` leaf and its accepted shape are undocumented today. Only `docs/guide/isis.md` mentions the show command |
| 3 | CLI command added/changed? | No | No command changes |
| 4 | API/RPC added/changed? | No | No RPC changes |
| 5 | Plugin added/changed? | No | No registration fields change |
| 6 | Has a user guide page? | Yes | `docs/guide/isis.md` |
| 7 | Wire format changed? | No | The framing is unchanged. The value's accepted octets narrow, which the RFC row records |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface changes |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `rfc/short/rfc5301.md`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, and `docs/features/rfc-status.md`, each with a source anchor to the producing line |
| 10 | Test infrastructure changed? | No | Existing suites and runners are reused |
| 11 | Affects daemon comparison? | No | Check `docs/comparison.md` for an IS-IS hostname row and leave it unless the support level changes |
| 12 | Internal architecture changed? | No | No structural change |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | A registered validator name is added, which no inventory doc enumerates. Confirm with `make ze-inventory` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/features/rfc-status.md` anchors `internal/plugins/isis/lsdb/encode.go` and `internal/plugins/isis/show.go`. Both claims must be re-checked after the edit |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify every `hostname` example under `docs/guide/isis.md` still validates against the new pattern |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the validator name and prove the entry points reach it
   - Tests: `TestISISHostnameValidatorRegistered`, and the draft `.ci` asserting a non-zero exit from `ze config validate`
   - Files: `internal/component/config/validators_register.go`, `internal/component/config/validators.go` with a refusing stub, `internal/plugins/isis/yang/ze-isis-conf.yang`, `test/draft/isis/isis-hostname-ascii.ci`
   - Verify: the name resolves in the registry, the startup integrity check stays quiet, and the `.ci` fails because the stub has no real rule yet
2. **Phase: Character set (`RFC5301-3-7`)** -- the pattern and the validator's charset half
   - Tests: `TestISISHostnameValidatorCharset`, `TestISISHostnameYANGDeclaresPattern`, `TestISISHostnameYANGPatternAgreesWithValidator`
   - Files: `internal/plugins/isis/yang/ze-isis-conf.yang`, `internal/component/config/validators.go`, `internal/plugins/isis/config_test.go`
   - Verify: A-3 holds, meaning the pattern compiles unanchored and really refuses. Tests fail, then implement, then pass
3. **Phase: Label structure (`RFC5301-3-9`)** -- the label rules from RFC 2181 section 11
   - Tests: `TestISISHostnameValidatorLabels`, `TestISISHostnameValidatorMessage`
   - Files: `internal/component/config/validators.go`
   - Verify: each boundary row in the Boundary Tests table is covered, and the message meets AC-2
4. **Phase: Emit invariant (`RFC5301-3-10`, AC-6 and AC-7)** -- prove nothing rewrites the value
   - Tests: `TestISISHostnameTLVIsPrintableASCII`, `TestISISHostnameTLVTruncationUnreachable`, `TestISISHostnameUnicodeRefusedNotConverted`
   - Files: `internal/plugins/isis/lsdb/encode_test.go`, `internal/plugins/isis/lsdb/encode.go`, `internal/plugins/isis/packet/tlv_core.go`
   - Verify: the emitted octets equal the configured string byte for byte, and the corrected comments name the enforcing layer
5. **Phase: Functional and interop proof** -- reach the behavior through the user's entry points
   - Tests: `test/isis/isis-hostname-ascii.ci`, `test/interop/scenarios/isis-p2p-frr/check.py`
   - Files: the draft `.ci` promoted into `test/isis/`, and the interop check
   - Verify: mutation-verify both. Remove the enforcement, confirm red, restore, confirm green
6. **Phase: Enrolment and documentation (AC-8, AC-9)** -- discharge the debt
   - Tests: `make ze-rfc-check`, `make ze-doc-test`
   - Files: `rfc/short/rfc5301.md`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `docs/features/rfc-status.md`, `docs/guide/isis.md`, `ai/RFC-REQUIREMENTS.md`
   - Verify: `make ze-rfc-check` exits 0, the row moved, and no gated MUST is recorded as `{gap}` or `{not-applicable}`

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and each of AC-1, AC-3, AC-4, AC-5, AC-6 has a tagged test in both polarities |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | The pattern is not self-anchored, per A-3. The accepted set restricts only what RFC 5301 and RFC 2181 restrict, per R-2. The error message names the offending value and the accepted shape in words |
| Naming | The validator name follows the `isis-net` and `isis-system-id` convention in `internal/component/config/validators_register.go`. The YANG leaf name is unchanged |
| Data flow | Validation happens once at the config boundary. Nothing is added to the per-LSP origination path, and `sanitizeHostname` on the display path is untouched |
| Rule: `ai/rules/protocol.md` | No path silently approximates. No truncation, no sanitising, and no ToASCII rewrite reaches the wire |
| Rule: `ai/rules/rfc-compliance.md` | The three requirements are met, not annotated. No `{gap}` and no `{not-applicable}` appears on any gated MUST row. The Reading A decision is recorded with the RFC text beside it |
| Rule: `ai/rules/evidence.md` | Removing the YANG pattern must not silently open the character-set hole, because the validator re-checks it. A comment asserting the invariant is not accepted as evidence |
| Rule: `ai/rules/interop-and-goal-validation.md` | The interop assertion is outside the fallback branch, and reverting the emit change turns it red |
| Registration over hardcoding | The validator arrives through `reg.Register` in the existing list. No switch case, no per-feature field, and no factory is added to a shared package |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| The YANG leaf declares the pattern and the validator name | `grep -n -A6 "leaf hostname" internal/plugins/isis/yang/ze-isis-conf.yang` |
| The validator is registered | `grep -n "isis-hostname" internal/component/config/validators_register.go` |
| Both polarities are tagged for each of the three requirements | `grep -rn "RFC5301-3-7\|RFC5301-3-9\|RFC5301-3-10" --include=*_test.go internal/` |
| The functional test exists and gates | `make ze-isis-test`, then mutation-verify by removing the pattern and the validator |
| The row moved | `grep -n rfc5301 rfc/enrolled.txt rfc/not-enrolled.txt` |
| The gate is green | `make ze-rfc-check` exits 0 |
| The ledger is fresh | `make ze-rfc-index`, then confirm `ai/RFC-REQUIREMENTS.md` has no pending diff |
| No fixture regressed | `make ze-isis-test`, `make ze-isis-wire-test`, and `make ze-verify` |
| The prose passes the style gate | `make ze-ste-review-changed` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The configured hostname is operator input reaching the wire. The accepted set must be an allowlist of octets, never a denylist, so an unforeseen octet is refused rather than forwarded |
| Untrusted peer input | A received TLV 137 stays untrusted. `sanitizeHostname` at `internal/plugins/isis/show.go` keeps bounding it for display. Do not tighten or remove that filter, and do not let the new stricter rule reject a peer's LSP, which would be a denial-of-service lever |
| Resource exhaustion | The label walk runs once per config load over at most 255 octets. No unbounded loop and no allocation per LSP |
| Error leakage | The message quotes the operator's own value only. Nothing about internal state is disclosed |
| Fail-open risk | A validator that cannot evaluate a value must refuse it. A nil or unregistered validator must not read as acceptance, per `ai/rules/evidence.md` |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| The YANG pattern fails to compile | A-3 is broken. Re-read `internal/component/config/yang/pattern.go` and drop the unsupported construct |
| The startup integrity check fires on the validator name | The name is missing from `internal/component/config/validators_register.go`. Add it |
| `ze config validate` accepts a value the daemon refuses | A-4 or A-5 is broken. Re-read the enforcement sites and move the rule to the layer both paths reach |
| An existing fixture stops validating | A-1 is broken. Re-run the sweep, then decide with the user whether the fixture or the rule is wrong |
| The interop scenario stays green after you revert the emit change | The assertion is vacuous. Move it out of the fallback branch, per R-3 |
| `make ze-rfc-check` reds after the row moves | Read its message. A missing polarity needs a test, never an annotation |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood, return to RESEARCH |
| Lint failure | Fix inline. If architectural, return to DESIGN |
| Functional test fails | Check the AC. A wrong AC returns to DESIGN, a correct AC returns to IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The character-set rule and the label-structure rule are two separate requirements with two separate enforcement points. Collapsing them into one regex is where drift begins. See R-1.
- A YANG `pattern` reaches more entry points than a YANG `length`. Patterns are applied by the validator tree walk and by the schema path used for `set`. The `LeafNode` at `internal/component/config/schema.go` carries no length field, so `length` is enforced only on the validator walk. A constraint expressed as a pattern therefore covers the interactive editor as well.
- A `ze:validate` function runs inside the validator tree walk, so it is reached by `ze config validate`. `OnConfigVerify` is not. A rule that must hold offline cannot live only in the plugin's verify callback.
- Multiple names in one `ze:validate` argument are OR semantics, per `internal/component/config/yang/validator_registry.go`. Use exactly one name here. A joined name would weaken the rule to "any one of these accepts".
- The codec comment at `internal/plugins/isis/packet/tlv_core.go` asserted the 7-bit ASCII property years before any enforcement existed. That is the shape `ai/rules/evidence.md` warns about. A comment that claims a safety property is not evidence the property holds.
- Restricting the character set to one byte per character makes the byte-length bound and the character-length bound identical. The truncation at `internal/plugins/isis/lsdb/encode.go` then becomes unreachable from config, which is why R-5 asks for it to be pinned rather than deleted.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Reject at the config boundary | Sanitise at emit, or truncate at emit | The owner ruled it, and `ai/rules/protocol.md` requires it. Sanitising would put a different name on the wire than the operator configured |
| Accepted set is printable 7-bit ASCII, octets `0x20` to `0x7e` | Strict 7-bit ASCII including control characters, or a letter-digit-hyphen rule | Strict 7-bit ASCII would admit NUL and control characters into a value the RFC calls a domain name. A letter-digit-hyphen rule would reject strings RFC 5301 section 3 explicitly permits, since it says the value can be any string operators want to use. The chosen set also matches the receive-side filter at `internal/plugins/isis/show.go` exactly, which makes emit and display symmetric |
| The narrowing beyond literal `RFC5301-3-7` is recorded, not hidden | Silently refusing control characters as though the RFC demanded it | Refusing control characters is Ze's own decision, not the RFC's. `ai/rules/evidence.md` forbids presenting it as an RFC requirement |
| `RFC5301-3-9` enforces label lengths, not a character set | An LDH or hostname-syntax rule | RFC 2181 section 11 places only a length restriction and says implementations must not restrict which labels can be used. Reading a character rule into it would over-restrict. See R-2 |
| A single trailing dot is accepted | Rejecting any trailing dot, or accepting repeated dots | RFC 2181 section 11 defines the zero-length full name as the root, and the trailing dot is the conventional absolute form. Other empty labels have no meaning |
| `RFC5301-3-10` takes Reading A | Reading B, implementing ToASCII with the vendored IDNA package | ToASCII is a silent rewrite of the operator's value, which `ai/rules/protocol.md` bans. Reading B also needs RFC 3490's text, which this checkout lacks. Recorded as a reading, never as `{gap}` |
| Both a YANG `pattern` and a custom validator | Pattern only, or validator only | Pattern only cannot produce an actionable message, since the failure renders a regex. Validator only would leave the schema silent about the accepted shape. The editor and the readers consult that schema. The validator stays authoritative and re-checks the character set, so the pair cannot fail open |
| The validator lives in the central config package | Inside the IS-IS plugin | `internal/plugins/isis/register.go` records that config cannot import isis without a cycle, which is why `isis-net` and `isis-system-id` already live centrally. This follows the existing arrangement rather than inventing a new one |

## Known Limitations
- Completion for the `hostname` leaf is not added. A constrained free-form string has no closed value set, and the plugin can register a `CompleteFn` later following `internal/plugins/isis/register.go`.
- The receive path stays lenient. A peer advertising a non-ASCII TLV 137 is still accepted and filtered for display only. RFC 5301 section 4 permits that, and tightening it would let a peer break Ze's CLI.
- Reading B for `RFC5301-3-10` is not implemented. If Thomas selects it, RFC 3490's text must be fetched and summarised first, and this spec reopens.
- A hostname carrying a space is accepted by the rule but can be awkward on the interactive `set` path. R-4 tracks it. Anything outstanding after implementation belongs in the deferral shard named in the metadata table.

## RFC Documentation (Scope: protocol)

Add `// RFC NNNN Section X.Y: "<quoted requirement>"` above enforcing code.
MUST document: validation rules, error conditions, state transitions, timer
constraints, message ordering, and every MUST/MUST NOT.

| Enforcing site | Comment must quote |
|----------------|--------------------|
| The new validator in `internal/component/config/validators.go` | RFC 5301 Section 3: "The Value field is encoded in 7-bit ASCII." and RFC 5301 Section 3: "The content of this value is a domain name, see [RFC2181]." |
| The label-length walk in the same validator | RFC 2181 Section 11: "The length of any one label is limited to between 1 and 63 octets. A full domain name is limited to 255 octets (including the separators)." |
| The `pattern` in `internal/plugins/isis/yang/ze-isis-conf.yang` | A `description` naming RFC 5301 Section 3 and stating the accepted shape in words |
| `hostnameTLV` in `internal/plugins/isis/lsdb/encode.go` | The invariant, and the layer that enforces it, so no future reader treats the truncation as intended behavior |
| `writeHostnameTLV` in `internal/plugins/isis/packet/tlv_core.go` | A corrected claim naming the enforcing layer, replacing the present unenforced assertion |

The Reading A decision for `RFC5301-3-10` is recorded in `rfc/short/rfc5301.md`
beside the requirement, with the RFC sentence quoted in full and the alternative
reading named. It is not recorded as an annotation that lowers what Ze owes.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-verify` passes (the pre-commit gate, `ai/rules/git-safety.md`)
- [ ] `make ze-rfc-check` exits 0 with `rfc5301` enrolled
- [ ] `make ze-ste-review-changed` clean for every file this spec touches
- [ ] No gated MUST row for RFC 5301 carries `{gap}` or `{not-applicable}`
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
- [ ] Mutation-verified: the pattern and the validator removed, the functional and interop tests turn red, both restored

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
