# Spec: fixit-isis-hostname-ascii

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | protocol |
| Depends | - |
| Phase | 1/6 |
| Deferral shard | `-` (corrected 2026-08-03: the row named a shard that never existed; not started; the spec already says outstanding work goes here after implementation. Create `plan/deferrals/fixit-isis-hostname-ascii.md` on the first deferral) |
| Updated | 2026-08-10 |

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

AC-1 narrows a leaf that today accepts any string of 1 to 255 bytes. The
narrowing is deliberate and it is the point of the owner ruling. It is carried by
a registered `ze:validate` custom validator and NOT by a YANG `pattern`; D-1
records why.

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
| A-1 | No existing test or fixture uses a non-ASCII IS-IS hostname, so the narrowing breaks nothing | A byte-level sweep found zero non-ASCII files under `internal/plugins/isis/`, `test/isis`, `test/isis-wire` and every `test/interop/scenarios/isis-*`. The 7 config fixtures are `test/isis/isis-config.ci` (`r1`), `test/isis/isis-show.ci` (`r1-isis`), and `ze.conf` in `isis-p2p-frr` (`ze-p2p`), `isis-lan-dis-frr` (`ze-lan`), `isis-auth-frr` (`ze-auth`), `isis-convergence-frr` (`ze-conv`), `isis-dualstack-frr` (`ze-ds`). Go values are `r1`, `ze-router`, `snap-node`, `router-a`, `host-a`, `peer-host` | The spec ships a test-breaking change and the fixtures need updating first | Re-run the sweep, then `make ze-isis-test` and `make ze-verify` | confirmed 2026-08-10: `grep -rlP '[^\x00-\x7f]'` over `internal/plugins/isis/`, `test/isis/`, `test/isis-wire/` and `test/interop/scenarios/isis-*` returned nothing. Two more fixtures exist beyond the seven listed: `isis-purge-reorig-frr` (`ze-purge`) and `isis-show.ci` (`r1-isis`). Every one is printable ASCII |
| A-2 | YANG `length` counts bytes, so `length "1..255"` already matches the 255-octet TLV cap | `internal/component/config/yang/validator.go` uses a byte count. No rune counting exists in `internal/component/config/` | A 255-rune multi-byte name would pass validation and then be cut mid-character at `internal/plugins/isis/lsdb/encode.go` | Read the line, then a unit test feeding a 256-byte name | confirmed 2026-08-10: `validateString` uses `uint64(len(str))`. The SECOND length site does not: `validateLengths` at `internal/component/config/schema.go` counts runes with `utf8.RuneCountInString`, and it runs on the config-file parse path and the editor `set` path. The two disagree for a multi-byte value, which is one more reason the character-set rule cannot rest on `length` |
| A-3 | A YANG `pattern` must NOT be self-anchored, and `\p{...}` classes are unavailable | `internal/component/config/yang/pattern.go` wraps the pattern as a whole-string anchored group. `internal/component/config/yang/pattern.go` rejects a bare `^` or `$` outside a character class. `internal/component/config/yang/pattern.go` rejects the `\p` escape | The chosen pattern fails to compile and the leaf silently accepts everything | Author the pattern, then a unit test asserting a rejected value really is rejected | confirmed 2026-08-10 by reading `rejectUnsupportedPattern` and `patternToGoRegexp` at `internal/component/config/yang/pattern.go`. Now MOOT: the implementation declares no `pattern` on the leaf. See the D-1 design correction below |
| A-4 | A `ze:validate` function is reached by `ze config validate`, unlike `OnConfigVerify` | `applyCustomValidators` at `internal/component/config/yang/validator.go` runs inside the validator tree walk. `internal/component/config/cli/cmd_validate.go` includes the isis section | The offline check does not fire and only the daemon rejects the value | A `.ci` asserting a non-zero exit from `ze config validate` | confirmed 2026-08-10: `./bin/ze config validate` on a config carrying `net 49.0001.00` exits 1 with `isis: isis/net: "49.0001.00" is not a valid IS-IS NET for isis/net: length 4 octets, want 8..20`, which is the custom validator's own text |
| A-5 | `pattern` reaches the interactive `set` path, and `length` does not | `internal/component/config/schema.go` applies patterns. The `LeafNode` at `internal/component/config/schema.go` carries no length field | An editor `set` accepts a value the file-based validator refuses | A `.et` or `.ci` case driving `set isis hostname` with a rejected value | **broken** 2026-08-10, in both halves, and the second half changes the design. `LeafNode` DOES carry `Lengths` (`internal/component/config/schema.go`), and `ValidateLeafValue` applies it. More important: `ValidateLeafValue` also runs on the config-FILE parse path at `internal/component/config/parser.go`, and a pattern failure there aborts the parse before the YANG tree walk ever runs. A `pattern` therefore PREEMPTS the custom validator on every entry point. Measured: `./bin/ze config validate` on `system-id zzzz.0000.0001` renders `invalid value for system-id: value "zzzz.0000.0001" does not match pattern "[0-9a-fA-F]{4}\\.[0-9a-fA-F]{4}\\.[0-9a-fA-F]{4}"` and the `isis-system-id` validator's message never appears. See D-1 |
| A-6 | `golang.org/x/net/idna` is available if Reading B is ever chosen | `go.mod:26` requires `golang.org/x/net v0.57.0` as a direct dependency, and `vendor/golang.org/x/net/idna` is vendored and listed at `vendor/modules.txt` | Reading B would need a dependency addition, which needs the user's approval per `ai/rules/go-standards.md` | Already read. Confirm before any Reading B work | confirmed 2026-08-10: `go.mod` requires `golang.org/x/net v0.57.0` and `vendor/golang.org/x/net/idna` is present. Unused: this implementation takes Reading A |
| A-7 | Enrolling `rfc5301` needs no new extraction sign-off, because `rfc/extraction/rfc5301.json` already exists and is signed | `rfc/extraction/rfc5301.json` carries `signed-off: 2026-07-30`, and line 5 carries the `register-reason` | Enrolment fails the newly-enrolled sign-off check at `scripts/dev/rfc_requirements.py` | `make ze-rfc-check` after you move the row | confirmed 2026-08-10: `rfc/extraction/rfc5301.json` carries `"signed-off": "2026-07-30"` and a `register-reason` for the `prose` grade |
| A-8 | The files this spec reads are committed by the time work starts | Every artifact this spec cites is UNCOMMITTED today. `rfc/not-enrolled.txt`, `rfc/extraction/rfc5301.json`, `rfc/short/rfc5301.md`, `rfc/enrolled.txt` and `docs/features/rfc-status.md` are the working-tree output of `plan/spec-rfcgate-4-ledger.md`, which is still `in-progress` | The cited line numbers move, and `rfc/not-enrolled.txt` does not exist at HEAD. AC-8 cannot be evaluated until the ledger spec lands | `git log -1 -- rfc/not-enrolled.txt` before Phase 1. If the ledger spec has not landed, set `Depends` to `spec-rfcgate-4-ledger` and wait | confirmed 2026-08-10: the ledger landed. `rfc/not-enrolled.txt` is committed at `3be45483e` and `rfc/extraction/rfc5301.json` at `e558c55b2`. `git status --porcelain rfc/ docs/features/rfc-status.md` is clean, and `rfc/not-enrolled.txt` carries a `backlog` row for `rfc5301` |

### Mistake Log: Wrong Assumptions

| ID | What the spec assumed | What is true | What changed |
|----|----------------------|--------------|--------------|
| D-1 | A YANG `pattern` and a custom validator can both guard the leaf, with the validator producing the message the operator reads (Key Design Decisions, "Both a YANG `pattern` and a custom validator") | A `pattern` runs FIRST, at parse, through `ValidateLeafValue` at `internal/component/config/schema.go` reached from `parseLeaf` at `internal/component/config/parser.go`. A parse error aborts the file before `ValidateTreeAllModules` runs, so `applyCustomValidators` never sees a value the pattern refused. The operator reads `value %q does not match pattern %q` and nothing else. Measured against the existing `system-id` leaf, which carries both today | The leaf declares `ze:validate "isis-hostname"` and NO `pattern`. The validator is the single rule, and its message is the one the operator reads. This is the arrangement the sibling `net` leaf already uses. AC-2 is met on the file path, the daemon commit path, the hub API and the web editor, all of which run `ValidateContent` at `internal/component/config/cli/cmd_validate.go` |

### Blockers

| ID | What is blocked | Why | The exact fix |
|----|-----------------|-----|---------------|
| B-1 | ~~Six defects in `internal/component/config/validators_isis_test.go` cannot be repaired~~ **RESOLVED 2026-08-10: Thomas approved the repair and the main thread applied it** | The block was real. `_rfc_tagged_change_err` (`.claude/hooks/pretool-writeedit.py`) refuses every behaviour-bearing edit to a file carrying `RFC requirement:` tags, at WHOLE-FILE scope because the shared tables sit outside every function (`tag_scope`, `scripts/dev/rfc_tagged_scope.py`, returns the file for a hunk it cannot place inside one). The only escape is `// rfc-test-change-approved:`, reserved to the user by `ai/rules/testing.md`. This is `plan/learned/HOOK-FRICTION.md` "the RFC-tagged-test guard blocks the author repairing a file it just wrote", in its TRACKED-file form, where that record's untracked-draft resolution is shut | Applied: the contradictory row removed, the 255-octet assertion re-pointed at a name reaching 255 octets with conforming labels, the `intrange` loop modernised, the zero-width space escaped. Each edit carries its own marker recording what was approved and why |

**The 255-octet coverage was not lost.** `ISISHostnameAccepted` already carried
`strings.Repeat("c.", 127) + "d"`: 128 labels, 255 octets, every label inside RFC
2181 section 11's 1-to-63 range. That row IS the total-length boundary. The
deleted row was a 255-octet SINGLE label, which is not a valid domain name and
which `isisCheckHostnameLabels` is right to refuse. The two rows were never
testing the same thing, and the table asserted the wrong one as accepted.

**One limitation of the guard, worth knowing before trusting a marker.** A
`rfc-test-change-approved:` marker is an assertion in a comment. Nothing in the
tree distinguishes an approval the owner gave from one an author wrote to clear
its own block. The guard raises the cost of an unapproved edit; it cannot prove
consent. An implementing agent flagged exactly that and refused to write the
marker itself, which is the behaviour the guard exists to produce.

The defect is in the fixture, never in the code. `ISISHostnameAccepted` carries
`strings.Repeat("b", 255)`, a 255-octet SINGLE label. RFC 2181 section 11 gives
one label 1 to 63 octets, so that value is not a valid domain name and
`isisCheckHostnameLabels` is right to refuse it. The same table already lists
`strings.Repeat("a", 64)` as REFUSED, so the two rows contradict each other and no
change to the validator can satisfy both. Four edits are owed, and each needs the
approval marker on the lines being written:

1. In `ISISHostnameAccepted`, delete the `strings.Repeat("b", 255)` row. The
   label-legal 255-octet name beside it, `strings.Repeat("c.", 127) + "d"`, already
   covers the total-length boundary.
2. In `TestISISHostnameValidatorLabels`, replace the two `strings.Repeat("b", 255)`
   and `strings.Repeat("b", 256)` boundary lines with `strings.Repeat("c.", 127) + "d"`
   (255, accepted) and `strings.Repeat("c.", 127) + "de"` (256, refused).
3. `for c := 0; c < 0x100; c++` becomes `for c := range 0x100` (`intrange`).
4. The literal zero-width space in `TestISISHostnameUnicodeRefusedNotConverted`
   becomes its backslash-u escape, spelled `\` then `u200b` (`ST1018`).
5. The prose gate wants 3 hedges and 2 run-ons out of the doc comments, and it
   names each one: `make ze-ste-review` on that path.

One growth is NOT a defect and must stay: `internal/plugins/isis/lsdb/encode_test.go`
gains one run-on, and that run-on is RFC 5301 section 3's IDNA sentence quoted
verbatim in an `RFC requirement:` tag. `ai/rules/writing.md` never edits quoted
external text.

Nothing else in the package is red, and no other gate is waiting on it.

**What D-1 costs, stated plainly.** The CLI editor's `set` command validates through
`ValidateLeafValue` only, so it does not run a custom validator. A rejected hostname
typed at `set` is refused at `commit`, not at `set`. The `net` leaf behaves the same
way today. The alternative that would close it is a `Validate` field on `LeafNode`
plus a package-level registry lookup in the parse path, which adds machinery to a
shared package that this defect does not need (`ai/rules/simplicity.md`). It is
reported to the owner rather than taken here.

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

| Entry Point | → | Feature Code | Test | State |
|-------------|---|--------------|------|-------|
| `ze config validate <file>` with a non-ASCII `isis { hostname }` | → | `ISISHostnameValidator` at `internal/component/config/validators.go`, reached through `applyCustomValidators` at `internal/component/config/yang/validator.go` | `test/isis/isis-hostname-ascii.ci` | green |
| Daemon start, SIGHUP reload, or a commit through the API or the web editor | → | one walk, `ValidateCustomSections` at `internal/component/config/validate_sections.go`. `LoadConfig` (`internal/component/config/loader.go`) calls it, which covers the daemon start and the SIGHUP reload. `runValidation` (`internal/component/config/cli/cmd_validate.go`) calls it, which covers `ze config validate`, the hub config API (`cmd/ze/hub/api.go`) and the web editor manager (`cmd/ze/hub/service_web.go`). Every one of them reaches `ISISHostnameValidator` | `test/isis/isis-hostname-startup-refused.ci` drives the daemon path. `TestLoadConfigRefusesAnISISHostnameOutside7BitASCII` and `TestLoadConfigAndConfigValidateAgree` pin the two callers to one verdict on the same bytes | green. Corrected 2026-08-12: this row asserted the daemon path reached the validator while `LoadConfig` validated nothing at all. `spec-fixit-config-validators-bypassed-at-startup` gave the walk its second caller, and the row is true from that commit |
| Interactive `set isis hostname <value>` in the CLI editor | → | `ValidateLeafValue` at `internal/component/config/schema.go`, which applies `Enums`, `Ranges`, `Lengths` and `Patterns` but NO custom validator | refused at `commit`, not at `set`. D-1 records the cost and the alternative | open question for the owner |
| An accepted hostname originated into an LSP | → | `fixedTLVs` at `internal/plugins/isis/lsdb/origination.go` → `hostnameTLV` at `internal/plugins/isis/lsdb/encode.go` | `TestISISHostnameTLVIsPrintableASCII`, `TestISISHostnameTLVFraming` | green |
| A validator name used in YANG | → | `reg.Register` in `internal/component/config/validators_register.go` | `TestISISHostnameValidatorRegistered` | green |

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
| `TestISISHostnameValidatorCharset` | `internal/component/config/validators_isis_test.go` | Every octet outside `0x20` to `0x7e` is refused, and every octet inside it is accepted. Tagged `RFC requirement: RFC5301-3-7 positive` and `negative` | written, RED on its own fixture: see B-1 |
| `TestISISHostnameValidatorLabels` | `internal/component/config/validators_isis_test.go` | Empty interior label, a 64-octet label, and a 256-octet total are refused. A 63-octet label, a multi-label FQDN, and a single trailing dot are accepted. Tagged `RFC requirement: RFC5301-3-9 positive` and `negative` | written, RED on its own fixture: see B-1 |
| `TestISISHostnameValidatorMessage` | `internal/component/config/validators_isis_test.go` | The error text names the offending value, the offending octet or label with its position, and the accepted shape in words | green |
| `TestISISHostnameValidatorRegistered` | `internal/component/config/validators_register_test.go` | The name used by the YANG leaf is present in the central registry, so the startup integrity check at `internal/component/config/yang/validator_registry.go` cannot fire | green, new file |
| ~~`TestISISHostnameYANGPatternAgreesWithValidator`~~ | -- | DROPPED, and R-1 with it: D-1 removed the second enforcement point, so no two rules can disagree. The shared accepted set survives and now drives the validator tests and the emit-side test instead | not written, by design |
| `TestISISHostnameYANGDeclaresValidator` | `internal/plugins/isis/config_test.go` | The leaf in `yang/ze-isis-conf.yang` declares the validator name and the length bound, and declares NO pattern, read from disk. Renamed from `...DeclaresPattern` by D-1 | green |
| `TestISISHostnameTLVIsPrintableASCII` | `internal/plugins/isis/lsdb/encode_test.go` | For every accepted value, the TLV 137 octets are byte-identical to the input and hold no octet outside `0x20` to `0x7e`. Tagged `RFC5301-3-10 negative` and `RFC5301-3-8 negative` | green, new file |
| `TestISISHostnameTLVTruncationUnreachable` | `internal/plugins/isis/lsdb/encode_test.go` | A config-accepted value never exceeds 255 octets, so the truncation branch is unreachable from config. Pins R-5 | green |
| `TestISISHostnameUnicodeRefusedNotConverted` | `internal/component/config/validators_isis_test.go` | A Unicode value is refused, and no ToASCII output appears in the accepted set. Tagged `RFC requirement: RFC5301-3-10 positive` | green |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| hostname total length, octets | 1-255 | 255 | 0 (empty string) | 256 |
| single label length, octets | 1-63 | 63 | 0 (empty interior label) | 64 |
| accepted octet value | 0x20-0x7e | 0x7e | 0x1f | 0x7f |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `isis-hostname-ascii` | `test/isis/isis-hostname-ascii.ci` | A UTF-8 hostname is refused by `ze config validate` with a named value and a named octet. An over-long label, an empty interior label and a 256-octet name are refused. Three accepted shapes validate. The editor `set` case is NOT here: D-1 leaves that path refusing at `commit` rather than at `set` | green, promoted from `test/draft/`, mutation-verified |

Draft the file at `test/draft/isis/isis-hostname-ascii.ci` first, per `ai/rules/testing.md`. Run it with the draft flag, prove it under load with the stress reproducer, then promote it into `test/isis/`.

Mutation-verify the test before you claim it gates, per `ai/rules/testing.md`. Remove the pattern and the validator, confirm the test turns red, then restore both.

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `isis-p2p-frr` | `test/interop/scenarios/` | FRR | FRR decodes Ze's TLV 137 and renders the hostname in `show isis database`. The assertion is now on the happy path, tagged `RFC5301-3-4 positive` and `RFC5301-3-6 positive`. R-3 closed | edited, NOT RUN here (needs Docker, `make ze-interop-test`) |

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

## State at 2026-08-10 (BLOCKER cleared 2026-08-12, see the resolution below)

The fix is implemented, `rfc5301` is enrolled with all 7 gated MUSTs carrying
both polarities, and B-1 is resolved. One BLOCKER stood, and it was not this
spec's to fix.

**BLOCKER: the config boundary is not on the daemon's startup path.**
This spec makes the config boundary the ONLY enforcement point, deliberately, by
owner ruling of 2026-07-30: `hostnameTLV` is unchanged so the operator's octets
reach the wire byte for byte, and a non-conforming value is rejected at config
time rather than sanitised at emit.

But `LoadConfig` (`internal/component/config/loader.go`) parses and returns. It
never calls `ValidateTreeAllModules`, which has exactly ONE non-test caller,
`runValidation` in `internal/component/config/cli/cmd_validate.go`. Both
`runYANGConfig` and the SIGHUP path reach `LoadConfig`. So a hand-edited
`config.conf` plus a restart still puts 8-bit octets into TLV 137, and
`RFC5301-3-7` is unenforced on the path that matters most.

**The Wiring Test row asserting the opposite is FALSE today.** It says a daemon
commit or reload "runs the same tree walk, so it reaches the same validator". It
does not.

**Thomas ruled on 2026-08-10: validate in `LoadConfig`.** That is
`spec-fixit-config-validators-bypassed-at-startup`, which this spec depended on.
It closes 22 validators over 29 YANG bindings at once, not just this one.

### BLOCKER resolution (2026-08-12)

That spec landed in the commit immediately before this one. `LoadConfig`
(`internal/component/config/loader.go`) now calls `refuseInvalidCustomSections`,
which walks `ValidateCustomSections`
(`internal/component/config/validate_sections.go`) over the same section list
`ze config validate` walks, and `isis` is on that list. So the daemon start and
the SIGHUP reload reach `ISISHostnameValidator`, and the Wiring Test row above is
corrected to say what the code does.

Two tests bind the claim rather than assert it.
`TestLoadConfigRefusesAnISISHostnameOutside7BitASCII`
(`internal/component/config/cli/cmd_validate_startup_agreement_test.go`) drives
`LoadConfig` with `café.example` and fails unless the error names `0xc3` and
`7-bit ASCII`. `test/isis/isis-hostname-startup-refused.ci` drives the same value
through `ze -` and fences exit 1 with the same two strings. Both carry
`RFC requirement: RFC5301-3-7 positive`, and `rfc/requirements/rfc5301.md` lists
them under that id.

**Also owed before closure, and its state at closure**

| # | Owed | State |
|---|------|-------|
| 1 | No `## Implementation Audit`, `## Goal Validation` or `## Review Gate` section | done: appended from `plan/TEMPLATE-CLOSURE.md` and filled below |
| 2 | No independent Review Gate artifact | done: recorded, `verdict=clean`, path in the Review Gate table |
| 3 | The R-4 space-in-`set` question and the D-1 editor-path cost need a deferral shard | resolved WITHOUT a shard. Each is separable from this spec's goal and each carries a question only Thomas answers, and `ai/rules/rule-precedence.md` homes that shape in a spec, never in a deferral row. Both are stated in Known Limitations and carried into the Deferrals Resolved table below |
| 4 | Three review NOTEs, none blocking | recorded in the Review Gate findings below. One is fixed, two are recorded and left: see Run 2 |

**B-1 is RESOLVED.** Thomas approved the fixture repair and the main thread
applied it: the contradictory 255-octet single-label row removed, the assertion
re-pointed at a name reaching 255 octets with conforming labels, the loop
modernised, the zero-width space escaped. Four markers record what was approved.
`make ze-test-pkg PKG=./internal/component/config RUN='TestISISHostname'` is ok
and `golangci-lint run ./internal/component/config/...` reports 0 issues.

---

## Implementation Summary

### What Was Implemented
- `internal/component/config/validators.go`: `ISISHostnameValidator`, with
  `isisCheckHostname` (empty, total length, character set) and
  `isisCheckHostnameLabels` (RFC 2181 section 11 label lengths). The four bounds
  are named constants. No letter-digit-hyphen rule: RFC 2181 section 11 forbids
  restricting which characters a label carries.
- `internal/component/config/validators_register.go`: one `reg.Register` line for
  `isis-hostname`, beside `isis-net` and `isis-system-id`.
- `internal/plugins/isis/yang/ze-isis-conf.yang`: the `hostname` leaf declares
  `ze:validate "isis-hostname"` and NO `pattern` (D-1), plus a `revision` entry
  and a `description` stating the accepted shape in words.
- `internal/plugins/isis/lsdb/encode.go` and
  `internal/plugins/isis/packet/tlv_core.go`: the comments now name the layer
  that PRODUCES the 7-bit ASCII guarantee instead of asserting it (R-7), and the
  255-octet bound is documented as a defensive guard rather than as policy (R-5).
- `test/interop/scenarios/isis-p2p-frr/check.py`: the `has_database_lsp("ze-p2p")`
  assertion moved out of the `if not have_route:` fallback onto the happy path
  (R-3), and it carries the `RFC5301-3-4` and `RFC5301-3-6` positive tags.
- `rfc/enrolled.txt`, `rfc/not-enrolled.txt`, `rfc/short/rfc5301.md`,
  `rfc/requirements/rfc5301.md`, `docs/guide/isis.md`.
- Tests: `validators_isis_test.go` (4 tests over two shared tables),
  `validators_register_test.go` (1), `internal/plugins/isis/config_test.go`
  (`TestISISHostnameYANGDeclaresValidator`), `internal/plugins/isis/lsdb/encode_test.go`
  (4), `test/isis/isis-hostname-ascii.ci` (7 steps).

### Bugs Found/Fixed
- **The interop hostname assertion was vacuous (R-3).** It sat inside the
  `if not have_route:` fallback, so a passing run never reached it. Reverting the
  emit path would have left the scenario green. It is on the happy path now.
- **The fixture table contradicted itself (B-1).** `ISISHostnameAccepted` listed a
  255-octet SINGLE label as accepted while the same table listed a 64-octet label
  as refused. No validator can satisfy both, and RFC 2181 section 11 says the
  refusal is correct. Thomas approved the repair on 2026-08-10 and the main thread
  applied it under four `rfc-test-change-approved` markers, one per edit.
- **`RFC5301-3-7` was unenforced on the daemon's own path.** Found by the round-1
  independent review of this spec. It is not fixed here: it is
  `spec-fixit-config-validators-bypassed-at-startup`, which closed in the commit
  before this one. See the BLOCKER resolution above.

### Documentation Updates
- `docs/guide/isis.md`, new section "The dynamic hostname": the accepted shape as
  a table, the refusal with the message `ISISHostnameValidator` really produces,
  the Reading A decision in operator terms, and the receive-side leniency. Three
  source anchors: `validators.go` (`ISISHostnameValidator`), `lsdb/encode.go`
  (`hostnameTLV`), `show.go` (`sanitizeHostname`).
- The `docs/features/rfc-status.md` row for RFC 5301 was written and committed
  earlier; `git diff` over that file is empty, so no further edit is owed.
- `rfc/short/rfc5301.md` gains "Reading of the conditional IDNA sentence": both
  readings stated, Reading A named as the grading basis, and the note that neither
  reading lowers the obligation. No `{gap}` and no `{not-applicable}`.
- `make ze-doc-test` NOT RUN: this session was instructed to run no suite.
  `make ze-rfc-check` WAS run and exits 0.

### Deviations from Plan
- **No YANG `pattern` on the leaf.** The plan called for a pattern beside the
  validator. D-1 measured that a pattern runs at parse and aborts the file before
  the tree walk, so it would preempt the validator and hand the operator a regex.
  `TestISISHostnameYANGPatternAgreesWithValidator` was dropped with it, and R-1
  with both: one rule cannot disagree with itself.
- `docs/features/rfc-status.md` and `ai/RFC-REQUIREMENTS.md` are not in this
  commit. The first landed earlier; the second was replaced by the per-RFC shards,
  so the artifact is `rfc/requirements/rfc5301.md`.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| assumption | A-3 and A-5: a YANG `pattern` and a custom validator can both guard the leaf, with the validator's message reaching the operator | `ValidateLeafValue` applies the pattern on the config-file PARSE path, which aborts before `ValidateTreeAllModules` runs. Measured on the existing `system-id` leaf, which carries both and shows only the regex | measured against a real `ze config validate` run | the leaf carries no `pattern`. Recorded as D-1, and in `plan/journal/earlier-guard-hides-the-better-error.md` |
| approach | The interop scenario was believed to prove the TLV 137 round-trip | The assertion was inside the route fallback, so a passing run never evaluated it | R-3, raised at design time and confirmed by reading the file | assertion moved onto the happy path and tagged |
| escalation | The RFC-tagged-test guard blocked this session repairing fixture data it had just written, on a TRACKED file | The move-aside route that answers this for an untracked draft is shut once git holds the file | B-1, hit while fixing the contradictory table | escalated to Thomas, who approved four specific edits. Recorded in `plan/journal/guard-blocks-its-own-authors-repair.md`. It blocked one more edit at closure: see the Review Gate NOTE below |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| `RFC5301-3-7`: the value is 7-bit ASCII | Done | `isisCheckHostname`, `internal/component/config/validators.go` | refused at the config boundary, never sanitised at emit |
| `RFC5301-3-9`: the content is a domain name per RFC 2181 | Done | `isisCheckHostnameLabels`, same file | lengths only; RFC 2181 section 11 forbids a character rule |
| `RFC5301-3-10`: the IDNA duty | Done, Reading A | the refusal itself; recorded in `rfc/short/rfc5301.md` | not annotated: both polarities are test-bound |
| `rfc5301` enrols and `make ze-rfc-check` stays at exit 0 | Done | `rfc/enrolled.txt`, `rfc/not-enrolled.txt` | `rfc-requirements OK`: 2963 gated MUSTs across 170 enrolled RFCs, 3341 tags resolved |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestISISHostnameValidatorCharset`; `test/isis/isis-hostname-ascii.ci` seq=4 | every octet outside `0x20`..`0x7e` refused, driven as a loop over all 256 |
| AC-2 | Done | `TestISISHostnameValidatorMessage` | value quoted, octet position named, accepted shape in words, no regex |
| AC-3 | Done | `TestISISHostnameValidatorLabels`; `.ci` seq=5, seq=6, seq=7 | 63/64 and 255/256 boundaries both stated explicitly |
| AC-4 | Done | `ISISHostnameAccepted` (11 rows); `.ci` seq=1..3 | `core_1`, a name with spaces, ` ` and `~` all accepted |
| AC-5 | Done | `TestISISHostnameUnicodeRefusedNotConverted` | it also asserts no `xn--` form entered the accepted set |
| AC-6 | Done | `TestISISHostnameTLVIsPrintableASCII` | the emitted octets equal the configured string for every accepted value |
| AC-7 | Done | `TestISISHostnameTLVTruncationUnreachable` | a 255-octet config value survives whole; the 256-octet case is programmatic only |
| AC-8 | Done | `make ze-rfc-check` exit 0 on 2026-08-12 | no `{gap}`, no `{not-applicable}` on any RFC 5301 row |
| AC-9 | Done | the RFC 5301 row of `docs/features/rfc-status.md`, and `rfc/short/rfc5301.md` | the row carries the enforced constraint and a source anchor; the three unmet-obligation claims are gone |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestISISHostnameValidatorCharset` | Done | `internal/component/config/validators_isis_test.go` | tagged `RFC5301-3-7` both polarities |
| `TestISISHostnameValidatorLabels` | Done | same file | tagged `RFC5301-3-9` both polarities |
| `TestISISHostnameValidatorMessage` | Done | same file | |
| `TestISISHostnameValidatorRegistered` | Done | `internal/component/config/validators_register_test.go` | new file; covers all three IS-IS validator names |
| `TestISISHostnameYANGPatternAgreesWithValidator` | Changed | dropped | D-1 removed the second enforcement point, so there is nothing to agree with |
| `TestISISHostnameYANGDeclaresValidator` | Done | `internal/plugins/isis/config_test.go` | reads the YANG from disk and asserts NO pattern |
| `TestISISHostnameTLVIsPrintableASCII` | Done | `internal/plugins/isis/lsdb/encode_test.go` | tagged `RFC5301-3-10` and `RFC5301-3-8` negative |
| `TestISISHostnameTLVTruncationUnreachable` | Done | same file | pins R-5 |
| `TestISISHostnameUnicodeRefusedNotConverted` | Done | `internal/component/config/validators_isis_test.go` | tagged `RFC5301-3-10 positive` |
| `isis-hostname-ascii` | Done | `test/isis/isis-hostname-ascii.ci` | written and promoted; UNRUN in this session, which ran no functional suite |
| `isis-p2p-frr` | Changed | `test/interop/scenarios/isis-p2p-frr/check.py` | assertion moved onto the happy path; NOT RUN here (needs Docker) |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| `internal/plugins/isis/yang/ze-isis-conf.yang` | Changed | `ze:validate` and a revision, but no `pattern` (D-1) |
| `internal/component/config/validators.go` | Done | |
| `internal/component/config/validators_register.go` | Done | |
| `internal/plugins/isis/lsdb/encode.go` | Done | comment only; the code is deliberately unchanged |
| `internal/plugins/isis/packet/tlv_core.go` | Done | comment only |
| `internal/plugins/isis/config_test.go` | Done | |
| `internal/component/config/validators_isis_test.go` | Done | |
| `internal/component/config/validators_register_test.go` | Done | new |
| `test/interop/scenarios/isis-p2p-frr/check.py` | Done | |
| `rfc/short/rfc5301.md`, `rfc/enrolled.txt`, `rfc/not-enrolled.txt` | Done | |
| `docs/features/rfc-status.md` | Done | committed earlier; empty diff now |
| `docs/guide/isis.md` | Done | |
| `ai/RFC-REQUIREMENTS.md` | Changed | superseded by the per-RFC shard `rfc/requirements/rfc5301.md` |
| `internal/plugins/isis/lsdb/encode_test.go` | Done | new |
| `test/draft/isis/isis-hostname-ascii.ci` | Done | drafted, then promoted to `test/isis/` |

### Audit Summary
- **Total items:** 39
- **Done:** 35
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 4 (the dropped pattern test, the YANG pattern, `ai/RFC-REQUIREMENTS.md`, the interop check), each recorded in Deviations

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| A UTF-8 hostname can no longer reach a peer as 8-bit octets in TLV 137 | unit + functional (`.ci` written, UNRUN) | `TestISISHostnameValidatorCharset` drives all 256 octets and refuses every one outside `0x20`..`0x7e`; `TestLoadConfigRefusesAnISISHostnameOutside7BitASCII` proves the DAEMON path refuses it, which is the half that was missing until the previous commit; `test/isis/isis-hostname-ascii.ci` and `test/isis/isis-hostname-startup-refused.ci` drive both entry points |
| The value is REFUSED, never sanitised or converted | unit | `TestISISHostnameTLVIsPrintableASCII` asserts the emitted value equals the configured name for all 11 accepted fixtures, so any rewrite on the emit path turns it red; `TestISISHostnameUnicodeRefusedNotConverted` asserts no `xn--` form is in the accepted set |
| `RFC5301-3-9`: the value is a domain name per RFC 2181 | unit + functional | `TestISISHostnameValidatorLabels` states the 63/64 and 255/256 boundaries; `.ci` seq=5..7 drive them through `ze config validate` |
| `RFC5301-3-10` is met, not annotated away | unit, both polarities | positive: `TestISISHostnameUnicodeRefusedNotConverted`. negative: `TestISISHostnameTLVIsPrintableASCII`. The reading is recorded in `rfc/short/rfc5301.md` with the RFC sentence quoted and the alternative named |
| `rfc5301` enrols and the gate stays green | gate output | `make ze-rfc-check` exit 0, 2026-08-12: `rfc-requirements OK: 2963 gated MUST-level requirement(s) across 170 enrolled RFC(s); 3341 test tag(s) resolved` |
| An independent implementation reads the name off the wire | interop (edited, NOT RUN) | `test/interop/scenarios/isis-p2p-frr/check.py` asserts FRR renders `ze-p2p` in its IS-IS database on the happy path. FRR can only print that after decoding type 137. It needs Docker and did not run in this session |

**Not proven yet, said plainly.** `test/isis/isis-hostname-ascii.ci` and the
`isis-p2p-frr` scenario did not run in any session that closed this spec. The
character-set and label rules rest on unit evidence; the wire round-trip rests on
`encode_test.go`, which originates a real LSP and reads the framed octets back
out of the LSDB entry, not on a foreign peer.

## Deferrals Resolved

| Row (from the deferral shard) | Final Status | Destination or evidence |
|-------------------------------|--------------|-------------------------|
| the Deferral shard field is `-`; no shard was ever created | done | nothing was deferred through the shard machinery |
| R-4: a hostname carrying a space is accepted by the rule and the interactive `set` grammar splits on whitespace | deferred, owner question | not a deferral row. The rule is right (RFC 5301 section 3 admits any string), so narrowing it to suit a grammar would be the wrong fix. It is stated in Known Limitations and owed to Thomas as "which way do I fix the grammar", never as "may I narrow the rule" |
| D-1: the editor `set` path runs `ValidateLeafValue` only, so a refused hostname is caught at `commit` rather than at `set` | deferred, owner question | not a deferral row. `net` and `system-id` behave the same way today, so this is a property of the editor path and not of this leaf. Closing it needs a `Validate` field on `LeafNode` plus a registry lookup in the parse path, which `ai/rules/simplicity.md` refuses to add for one leaf. Stated in Known Limitations and owed to Thomas |
| Reading B for `RFC5301-3-10` | cancelled unless Thomas reopens it | Reading A is implemented and recorded. Reading B needs `rfc/full/rfc3490.txt`, which this checkout does not hold |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/fixit-isis-hostname-ascii-640fa955-f03a-45e8-a58f-4b367f5859e6.md`, `verdict=clean rounds=2` |
| `review_gate.py check` | `review_gate: OK (clean, hashes match)` |
| Rounds | 2. Round 1 (2026-08-10) found the BLOCKER above and three NOTEs. Round 2 (2026-08-12) is the independent pass after the BLOCKER's own spec landed: 0 BLOCKER, 0 ISSUE, 3 NOTE |
| Reviewer lenses used | RFC-text-versus-code, test discrimination and vacuity, comment-versus-producer, guard reachability |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | BLOCKER | The config boundary is the only enforcement point and the daemon's own startup path does not reach it, so `RFC5301-3-7` is unenforced where it matters most | `internal/component/config/loader.go` | not fixable here. Thomas ruled on 2026-08-10 that `LoadConfig` validates, and `spec-fixit-config-validators-bypassed-at-startup` did it. See the BLOCKER resolution section |
| 2 | ISSUE | The interop hostname assertion was inside the route fallback, so a passing run never evaluated it (R-3) | `test/interop/scenarios/isis-p2p-frr/check.py` | moved onto the happy path, with the RFC tags on it and a note stating why `RFC5301-3-7` is NOT provable against a conforming peer |
| 3 | ISSUE | `ISISHostnameAccepted` asserted a 255-octet single label as accepted while the same table refused a 64-octet label (B-1) | `internal/component/config/validators_isis_test.go` | the contradictory row deleted and the boundary re-pointed, under four owner-approved markers |

### Run 2 (independent, 2026-08-12, after the blocking spec landed)

0 BLOCKER, 0 ISSUE, 3 NOTE. The BLOCKER is discharged by the previous commit and
was re-verified at its producer: `LoadConfig`
(`internal/component/config/loader.go`) calls `refuseInvalidCustomSections`,
`validatedSections` (`internal/component/config/validate_sections.go`) carries
`isis`, and two tests drive `café.example` through the daemon path.

| # | Severity | Finding | Location | Disposition |
|---|----------|---------|----------|-------------|
| 4 | NOTE | `isisCheckHostname`'s comment said the character set is checked first, and the TOTAL-LENGTH rule runs before it. A 400-octet UTF-8 name is reported as too long, not as non-ASCII | `internal/component/config/validators.go` | fixed: the comment states the real order and why the length wins. The behaviour is right and is unchanged |
| 5 | NOTE | `hostnameAccepted`'s comment claimed one accepted set drives the config rule and the wire. They are two literals in two packages, hand-copied, and nothing fails when they drift | `internal/plugins/isis/lsdb/encode_test.go` | fixed: the comment now says it is a copy, why it cannot be shared (the original is in a `_test.go` file), and what a drift costs |
| 6 | NOTE | `.ci` seq=7 asserts only `contains=255` where its siblings assert a full phrase, and `bytes.Count(raw, framed) != 1` can meet a coincidental 3-octet run for the `" "` and `"~"` fixtures | `test/isis/isis-hostname-ascii.ci`, `internal/plugins/isis/lsdb/encode_test.go` | RECORDED, NOT FIXED. The strengthening edit to the `.ci` was refused by `_rfc_tagged_change_err` (`.claude/hooks/pretool-writeedit.py`), which cannot tell a stronger assertion from a weaker one and reserves the escape to Thomas. Writing the marker to clear my own block is what `ai/rules/testing.md` forbids, so the edit is owed to him. Both are NOTEs: the assertions are correct today, and neither can fail open |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/plugins/isis/lsdb/encode_test.go` | Yes | `-rw-rw-r-- 8441 Aug 12 02:32` |
| `internal/component/config/validators_register_test.go` | Yes | `-rw-rw-r-- 1140 Aug 10 19:24` |
| `test/isis/isis-hostname-ascii.ci` | Yes | `-rw-rw-r-- 4939 Aug 10 18:41` |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1, AC-2, AC-3, AC-4, AC-5 | the validator refuses and its message is actionable | `ok github.com/ze-software/ze/internal/component/config 71.901s` (2026-08-12), carrying all four `TestISISHostname*` tests |
| AC-6, AC-7 | the emit path preserves the value and its bound is unreachable from config | `ok github.com/ze-software/ze/internal/plugins/isis/lsdb` (2026-08-12) |
| AC-8 | the gate is green with `rfc5301` enrolled | `make ze-rfc-check` exit 0; `grep -c rfc5301 rfc/enrolled.txt` = 1 |
| AC-9 | the public row states the enforced constraint | the RFC 5301 row of `docs/features/rfc-status.md` names `ISISHostnameValidator` and the `ze:validate` binding; `git diff` over the file is empty, so the row is already in git |
| the YANG declaration | the leaf declares the validator and no pattern | `ok github.com/ze-software/ze/internal/plugins/isis 72.835s`, carrying `TestISISHostnameYANGDeclaresValidator`; the `hostname` leaf in `ze-isis-conf.yang` reads `ze:validate "isis-hostname";` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze config validate <file>` with a non-ASCII hostname | `test/isis/isis-hostname-ascii.ci` | Read: seq=4 drives `café.example` and fences `octet 4`, `0xc3`, `7-bit ASCII` and `0x7e` on stdout with exit 1 |
| Daemon start or SIGHUP reload of the same config | `test/isis/isis-hostname-startup-refused.ci` | Read: `exec=ze -` on stdin, exit 1, stderr names `0xc3` and `7-bit ASCII`. The walk it depends on is `LoadConfig` to `refuseInvalidCustomSections` to `ValidateCustomSections`, read at `internal/component/config/loader.go` |
| Interactive `set isis hostname` | none | refused at `commit`, not at `set`. D-1 states the cost; it is an owner question, not a wiring gap |
| An accepted hostname originated into an LSP | `internal/plugins/isis/lsdb/encode_test.go` | Read: `emitHostnameTLV` runs `Originate` and reads TLV 137 out of the LSDB entry's raw bytes, so it exercises the real origination path rather than calling `hostnameTLV` directly |
| A validator name used in YANG | `internal/component/config/validators_register_test.go` | Read: it builds a registry, runs `RegisterValidators`, and fails if any of the three IS-IS names is absent or carries a nil `ValidateFn` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | the byte-level sweep found no non-ASCII hostname anywhere under `internal/plugins/isis/`, `test/isis/`, `test/isis-wire/` or `test/interop/scenarios/isis-*` |
| A-2 | confirmed, with a correction | `validateString` counts bytes; `validateLengths` (`schema.go`) counts runes. The two disagree for a multi-byte value, which is one more reason the character rule cannot rest on `length` |
| A-3 | confirmed, then MOOT | the pattern constraints are real, and the leaf declares no pattern (D-1) |
| A-4 | confirmed | a real `ze config validate` run renders the custom validator's own text |
| A-5 | **broken** | a `pattern` preempts the custom validator on every entry point. Recorded as D-1, with the Deviations entry and the Mistake Log row |
| A-6 | confirmed, unused | `golang.org/x/net/idna` is vendored; Reading A needs none of it |
| A-7 | confirmed | `rfc/extraction/rfc5301.json` carries `signed-off: 2026-07-30` and a `register-reason` |
| A-8 | confirmed | the ledger spec landed; `rfc/not-enrolled.txt` and `rfc/extraction/rfc5301.json` are both committed |
| R-1 | retired | D-1 removed the second enforcement point, so there is no pair to drift |
| R-2 | retired | the rule bounds octets and label lengths only; `core_1` and a name with spaces are in the accepted table |
| R-3 | fixed | the interop assertion is on the happy path |
| R-5 | pinned | `TestISISHostnameTLVTruncationUnreachable` |
| R-6 | retired | `RFC5301-3-10` carries a tagged test in both polarities, so Reading A is observable rather than asserted |
| R-7 | fixed | both codec comments name the enforcing layer |
| R-4 | live, owner question | see Deferrals Resolved |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| the refusal message quoted in `docs/guide/isis.md` | produced by `isisCheckHostname` (`internal/component/config/validators.go`), whose format string carries `octet %d is 0x%02x` and the "space (0x20) to tilde (0x7e)" tail verbatim | Yes |
| "one label 1 to 63 octets, the whole name 1 to 255" | `isisHostnameMaxLabel` and `isisHostnameMaxOctets`, same file | Yes |
| "`core_1` and a name with spaces are both accepted" | both are rows of `ISISHostnameAccepted`, and the package test is green | Yes |
| "a hostname a PEER advertises is accepted whatever its octets" | `sanitizeHostname` (`internal/plugins/isis/show.go`) filters for DISPLAY only; nothing on the receive path refuses an LSP over TLV 137 | Yes |
| the `docs/features/rfc-status.md` row | it names `ISISHostnameValidator` and the `ze:validate` binding, both of which exist at the named paths | Yes |
| `rfc/short/rfc5301.md` carries the reading, not an annotation | the file states both readings and grades against A; `make ze-rfc-check` reports no `{gap}` on any RFC 5301 row | Yes |

## Core Insight

An enforcement point is only as good as the paths that reach it. This spec put
one rule at the config boundary and proved it from `ze config validate`, and the
review's question was not "is the rule right" but "who calls it". The answer was
one caller, and the daemon was not it. A validator with a green test and one
caller looks exactly like a validator with a green test and every caller.
