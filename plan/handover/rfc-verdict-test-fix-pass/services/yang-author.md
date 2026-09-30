# services child, config/yang (stem rfc7950) continuation author + bcp195 fix

Continues `config-author.md`. Nothing stamped, nothing committed. Rows are appended per id as each finishes.

## Small fix

- `internal/test/tlsprobe/bcp195.go`: the static-ECDH comment now quotes RFC 9325 §4.1 verbatim ("Similarly, implementations SHOULD NOT negotiate non-ephemeral Elliptic Curve DH key agreement."). Comment above a const block, no function body changed; no discrimination record names a tlsprobe producer (grep of rfc/discrimination), so nothing to re-record.

## Verdicts

| id | resolution | what now proves each clause | records written | expected verdict | notes |
|----|-----------|------------------------------|-----------------|------------------|-------|
| RFC7950-7.6.4-2 | defect + tests | new `TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef` (- union member enum x / bits "b1 b2" / typedef enum x, each with if-feature, refused; + y / "b1" / y load) beside the old `TestRFC7950DefaultNotAnIfFeatureEnum` and the type/mandatory pair | neg+pos new unit, neg+pos old unit re-recorded, all on `yang_schema.go::validateDefaultNotIfFeature` (producer changed) | enforced | D-8: union member was never walked, a union default naming an if-feature enum was ACCEPTED; red observed (union_member subtest) before the fix. Fix: `unionMemberHolding` picks the first member that accepts the default (RFC 7950 §9.12 order), the walk continues into it |
| RFC7950-9.3.2-1 (wrong) | row: retired (R5) + new row RFC7950-9.3.1-1 | tags moved (D-15 approvals recorded) to RFC7950-9.3.1-1 [MUST, D-3 format row; the MUST is §9.1's]: `TestRFC7950Decimal64Accepted` (+: "+"/"-"/no sign, digits alone, digits.period.digits) and `TestRFC7950Decimal64Refused` (-: ".5", "1.", bare sign, "1.2.3", "abc", "") | neg+pos on `yang/validator.go::validateDecimal64` for 9.3.1-1 | 9.3.1-1 enforced; 9.3.2-1 audit entry to drop (judge, P-2) | `Retired 2026-09-30` paragraph in rfc/corrections/rfc7950.md; site 9.3.2:1 now excluded feature-out-of-scope (canonical form binds XML only, §9.1 site 9.1:2); section 9.3.1 lists `unsourced-ids: [RFC7950-9.3.1-1]`. The two old 9.3.2-1 records remain in rfc/discrimination/rfc7950.json (tool-written; not hand-deleted) |
| RFC7950-8.3.1-1 | row: retired (R5/R2, binds a NETCONF server) | 7 tags moved (D-15 approvals recorded) to {gap} row RFC7950-8.1-1 ("All leaf data values MUST match the type constraints ... range, length, pattern"): `TestValidateTree_RangeViolation` (-), `TestValidateTree_PatternViolation` (-), `TestValidateTree_LengthViolation` (-), `TestValidator_ValidatePattern` (+/-), `TestValidator_HoldTimeRange` (+/-) in internal/component/config/validator_yang_test.go; claims no longer say "invalid-value response" | 7 records on 8.1-1: range tests on `yang/validator.go::checkYangRange`, pattern/length on `yang/validator.go::validateString` | 8.3.1-1 audit entry to drop (judge); 8.1-1 stays {gap} (when/unique/choice) | `Retired 2026-09-30` paragraph; site 8.3.1:2 excluded feature-out-of-scope (NETCONF reason, same as 8.3.1:1/:6/:7). Watch: whether the gate accepts +/- tags on a {gap} row (R5 directs it): it does NOT, see below |
| RFC7950-7.6.1-1 | unresolved (not started; analysis only) | | | weak | case-node clause needs choice/case structure: `yang_schema.go::flattenChoiceCases` drops it, and `plan/pre-release/spec-config-yang-when-unique-choice.md` owns choice semantics. Row is one contiguous §7.6.1 span with the case bullet in the MIDDLE, so R3 means a three-way split (prefix bullet / case bullet {gap} naming that spec / "Otherwise ... ancestor exists" + "in use" tail), each piece verbatim. The tail also carries "when"/"if-feature" not-in-use, which walkTree does not evaluate (same spec). Absent-ancestor negative for `ApplyDefaults` still owed |
| RFC7950-7.6.5-1 | unresolved (not started) | | | weak | same shape as 7.6.1-1 (case bullet in the middle, same blocking spec); plus `TestValidator_MandatoryField` asserts the error type only when errors.AsType succeeds (make it require) |
| RFC7950-9.1-1 | unresolved (not started) | | | weak (now STALE: decimal64 units' tag lines changed) | hex/octal integer forms (§9.2.1) in module defaults, and bits/binary/empty/identityref lexical checks, per config-author.md |
| RFC7950-8.1-2, 8.3-1, 8.3.3-1 | unresolved (not started) | | | weak | need the editor commit path (internal/component/cli/editor_commit.go) and daemon reload (cmd/ze/hub/main_reload.go); cli is outside this child: main thread assigns |
| RFC7950-7.19-1, 9.4.4-1, 9.6.4.2-1 | blocked (R4) | | | stay | R4 moves them to `plan/pre-release/spec-config-yang-loader-structural-checks.md` ACs, but NEITHER that spec's Task table (it has 9.6.4-2 only) NOR the services child's Blocked-by table lists 7.19-1, 9.4.4-1, 9.6.4.2-1 yet: the P-3 edit is owed by the main thread. The untracked red `internal/component/config/yang/loader_rfc7950_structural_red_test.go` (3 untagged tests, red by design) belongs to that spec: left in place |

## `./le rfc check` after this work (rc=2; no rfc9325 line)

Owed before a commit:
- `RFC7950-8.1-1 is annotated {gap} but IS tested`: the R5 tag move onto a {gap} row is refused. Fix (R3): split 8.1-1 into (a) the prefix up to "The following properties are true in all data trees:", (b) a new row RFC7950-8.1-3 quoting "o All leaf data values MUST match the type constraints for the leaf, including those defined in the type's "range", "length", and "pattern" properties." carrying the 7 moved tags (retag 8.1-1 to 8.1-3, re-record the 7), (c) a new row 8.1-4 with the rest from "o All key leafs MUST be present" {gap: when-unique-choice}; remap the 8.1 extraction sites; add 8.1-3/8.1-4 to that spec's Task table. Check the 8.1 high-water mark first (8.1-2 exists). HALF-DONE state: the 7 tags and records sit on 8.1-1 now.
- Audit entries for retired `RFC7950-8.3.1-1` and `RFC7950-9.3.2-1` must go (judge, P-2 audit-stamp).
- Two orphan discrimination records `RFC7950-9.3.2-1` (+/-) in rfc/discrimination/rfc7950.json are refused; drop them.
- `rfc/extraction/rfc7950.json: exclusions rose from 35 to 37 with no 'resign-reason'` (sites 9.3.2:1, 8.3.1:2): needs a resign-reason, a reviewer who is not me, and a signed-off bump.
- STALE 7.6.4-2 (new unit) and 9.1-1 (tag lines edited): re-judge. SHIFTED 9.6-1, 9.12-1, 7.6.5-1, 7.3.4-1: reseal (judge, global lock).

Gates run: go test -race on config (ok), config/yang (FAIL only the 3 deliberate red tests above), tlsprobe (no test files); golangci-lint on the three packages through ./le job run: 0 issues; gofmt clean. Owed: `./le test unit all` (the union default walk now calls ValidateType per member), `./le verify worktree`.

## Files changed

- internal/test/tlsprobe/bcp195.go
- internal/component/config/yang_schema.go (validateDefaultNotIfFeature union walk, new unionMemberHolding)
- internal/component/config/yang_schema_defaults_rfc7950_clauses_test.go (new TestRFC7950DefaultNotAnIfFeatureMemberBitOrTypedef)
- internal/component/config/yang/validator_decimal64_rfc7950_test.go (tags 9.3.2-1 to 9.3.1-1)
- internal/component/config/validator_yang_test.go (7 tags 8.3.1-1 to 8.1-1)
- docs/architecture/config/yang-config-design.md
- rfc/short/rfc7950.md, rfc/extraction/rfc7950.json, rfc/corrections/rfc7950.md, rfc/discrimination/rfc7950.json
- approvals: yang.TestRFC7950Decimal64Accepted/Refused; config.TestValidateTree_{Range,Pattern,Length}Violation, config.TestValidator_ValidatePattern, config.TestValidator_HoldTimeRange
- scratch: children/services/yang-rec.sh (record helper) and yang-*.log

# Continuation 2 (2026-09-30)

| id | resolution | what now proves each clause | records written | expected verdict | notes |
|----|-----------|------------------------------|-----------------|------------------|-------|
| RFC7950-8.1-1 | row: R3 split finished | 8.1-1 keeps the prefix (config/state/notification/RPC bullets, sites 8.1:1-5) {gap}; NEW 8.1-3 "All leaf data values MUST match the type constraints ..." (site 8.1:6) carries the 7 tags (retagged from 8.1-1: TestValidateTree_{Range,Pattern,Length}Violation -, TestValidator_ValidatePattern +/-, TestValidator_HoldTimeRange +/-); NEW 8.1-4 "All key leafs ... mandatory ..." (sites 8.1:7-10,12,13) {gap} | 7 records on 8.1-3 (log yang-rec813.log); orphan records of 8.1-1 (7) and 9.3.2-1 (2) dropped from rfc/discrimination/rfc7950.json under the stem lock | 8.1-3 enforced (new verdict owed); 8.1-1/8.1-4 gap | Correction 2026-09-30 paragraph in rfc/corrections/rfc7950.md; Support remaining now "Fifty-four"; spec-config-yang-when-unique-choice.md Task table: 8.1-1 row trimmed to its prefix, 8.1-4 row added. Comment in TestValidateTree_ (untagged, line 274) now names 8.1-3 |
| RFC7950-7.6.1-1 | row: R3 split (4 rows) + new test | 7.6.1-1 keeps lead-in + "no such ancestor" branch, its 2 tags, site 7.6.1:1; NEW 7.6.1-2 case-node branch {gap: flattenChoiceCases}; NEW 7.6.1-3 "Otherwise ... if the ancestor node exists": new `TestRFC7950LeafDefaultFollowsItsAncestor` (+ existing presence container / list entry get default; - absent ones not created) in schema_defaults_rfc7950_test.go; NEW 7.6.1-4 "in use" paragraph {gap: no when, no if-feature} | +/- on 7.6.1-3, producer schema_defaults.go::applyContainerDefault (break = panic, so the negative's red is a panic: judge may want a targeted break) | 7.6.1-1 enforced (re-judge; row narrowed), 7.6.1-3 enforced | Correction paragraph; sites 7.6.1:2/3/4 remapped; 7.6.1-2/-4 added to spec-config-yang-when-unique-choice.md Task table. Package tests ok (log yang-761.log) |
| RFC7950-7.6.5-1 | defect found, STOPPED (design choice; failing test written, untagged) | nothing new tagged | none | stays weak | Three defects against §7.6.5 at the producer: (1) walkTree/validateContainerEntry check mandatory only on containers PRESENT in the data, so a mandatory leaf under absent non-presence containers is never required: ze-bgp-conf.yang bgp/session/asn/local (bgp, session, asn all non-presence) is not reported when bgp carries router-id and no session. Red: NEW untagged `TestRFC7950MandatoryUnderAbsentNonPresenceContainer` (internal/component/config/validator_mandatory_rfc7950_red_test.go, FAIL "got []", log yang-765.log). (2) validate_sections.go SectionValidationError.Blocking grades ErrTypeMissing as a warning, so LoadConfig accepts a config missing a mandatory leaf (also defeats 8.1-2 "running MUST always be valid"). (3) ValidateCustomSections skips an absent top-level section, so its mandatory descendants are never required. Fixing (1)+(2) makes every bgp config without session/asn/local invalid: needs an owner call (mark top containers `presence`, or enforce). Also: TestValidator_MandatoryField subtest "missing_mandatory_as_in_local" validates path "bgp/local", which no longer exists in the schema (asn moved to bgp/session/asn/local), so it passes on the findSchemaNode error; its `errors.AsType` guard hides that: fix (require the type, correct the path) once the owner rules. R3 split planned: 7.6.5-1 prefix + no-ancestor branch (sites 7.6.5:1), 7.6.5-2 case branch {gap: flattenChoiceCases, when-unique-choice spec}, 7.6.5-3 "Otherwise ... ancestor node exists" (+ list entry with the leaf accepted / absent entry not required; - entry without it refused) |
| RFC7950-9.1-1 | unresolved (not started, budget) | | | weak (STALE) | §9.2.1 hex/octal integer defaults in a module ("0x10" refused by ParseUint base 10): a D-8 defect in validateUnsigned/validateSigned for MODULE defaults only (instance data stays decimal); bits/binary/empty/identityref are absent types (Support text lists them): R3-split those clauses to {gap} is not possible (one sentence), so judge the row on the integer forms after the fix |
| RFC7950-8.1-2, 8.3-1, 8.3.3-1 | unresolved (not started) | | | weak | need the cli editor commit path (internal/component/cli/editor_commit.go) and hub reload (cmd/ze/hub/main_reload.go); 8.1-2 also depends on the 7.6.5-1 owner call (missing mandatory is a warning at LoadConfig) |

## `./le rfc check` after Continuation 2 (rc=2), rfc7950 lines are all judge-owned
- audit entries for retired 8.3.1-1, 9.3.2-1 (audit-stamp rejudge, P-2)
- SHIFTED 9.6-1, 9.12-1, 7.6.5-1: reseal. STALE 7.6.1-1 (row text narrowed), 7.6.4-2, 9.1-1: re-judge. New verdicts owed: 8.1-3, 7.6.1-3 (and gap rows 7.6.1-2/-4, 8.1-4)
- rfc/extraction/rfc7950.json exclusions 35 -> 37 (sites 9.3.2:1, 8.3.1:2): resign-reason + non-author reviewer + signed-off bump
- Support remaining gap count fixed (Fifty-six)

## Files changed in Continuation 2
- internal/component/config/validator_yang_test.go (7 tags 8.1-1 -> 8.1-3; untagged comment at line 274)
- internal/component/config/schema_defaults_rfc7950_test.go (new TestRFC7950LeafDefaultFollowsItsAncestor)
- internal/component/config/validator_mandatory_rfc7950_red_test.go (NEW, untagged, red by design)
- rfc/short/rfc7950.md, rfc/extraction/rfc7950.json, rfc/corrections/rfc7950.md, rfc/discrimination/rfc7950.json (7 records 8.1-3, 2 records 7.6.1-3; 9 orphan records of 8.1-1/9.3.2-1 removed)
- plan/pre-release/spec-config-yang-when-unique-choice.md (Task table: 8.1-1 trimmed, 7.6.1-2, 7.6.1-4, 8.1-4 added)
- Gates owed (main thread): go test -race ./internal/component/config/... (only the new red + the 3 structural reds expected), ./le test unit all, ./le verify worktree

# Continuation 3 (2026-09-30), stopped by OWNER RULING (YANG is third-party; do not close RFC 7950 gaps)

| id | resolution | notes |
|----|-----------|-------|
| RFC7950-7.6.5-1 | stopped, no edit | Discovery only. Scan of every conf module (plugin/all loaded): the only mandatory leaves with no presence/list/case ancestor are ze-bgp-conf `bgp/router-id` and `bgp/session/asn/local`. Finding for the main thread: 121 functional .ci configs have a `bgp {` block with no router-id (e.g. test/parse/flowspec-action-encoding.ci, test/ui/ze-stripped-no-bgp.ci), and many set session/asn/local only per peer, which the YANG's own `ze:required` inheritance design (peer/group override of the global) allows. Enforcing §7.6.5 on bgp would refuse them; the schema's `mandatory true` on the two inherited globals and the ze:required design disagree. Also: `bgp` is not in validatedSections, so LoadConfig never walks it; the editor (cli/validator.go) and SectionValidationError.Blocking grade a missing mandatory as a warning. |
| RFC7950-9.1-1, 7.6.1-1 tag, 8.1-2, 8.3-1, 8.3.3-1 | stopped, not started | per owner ruling |

## Files changed in Continuation 3
- internal/component/config/zz_mandatory_scan_test.go (NEW, untracked, temporary discovery test `TestZZScanMandatory`, package config_test, blank-imports plugin/all; compiles and passes, asserts nothing). Throwaway: the main thread should delete it (I was told not to revert).
- No other repo file changed. The red untagged internal/component/config/validator_mandatory_rfc7950_red_test.go from Continuation 2 is untouched and still red.

# Ruling 4 (2026-09-30)

OWNER RULING 4 applied: Meta `mixed`, no validator or YANG-module code. Nothing stamped, nothing committed.

| id | resolution | notes |
|----|-----------|-------|
| RFC7950-7.6.1-1 | tagged existing unit | `TestApplyDefaults_NonPresenceContainer` (schema_defaults_test.go) tagged positive (non-presence ancestor absent -> container created, defaults filled); record written on `schema_defaults.go::applyContainerDefault`, revert route, red observed (log yang-rec761np.log). Expected: enforced on re-judge (STALE) |
| RFC7950-7.6.5-1 | R3 split + stays weak | gate refuses `{gap}` on a tagged row (checkCoverageRatchet / stale-annotation). Row keeps lead-in + no-ancestor sentence and its 3 tags (one sentence, cannot split further; that sentence is itself unmet for absent non-presence ancestors). NEW 7.6.5-2 (case branch) {gap}, NEW 7.6.5-3 ("Otherwise ... ancestor node exists") {gap}. Sites 7.6.5:2/3 remapped. In AC-2 of the when-unique-choice spec |
| RFC7950-8.3.3-1 | R3 split | keeps sentence 1 (final contents obey all constraints) + its 2 tags + site 8.3.3:1; expected enforced on re-judge (STALE). NEW 8.3.3-2 (running/startup enforced at end of edit-config/copy-config) {gap}, site 8.3.3:2 remapped |
| RFC7950-8.1-2, 8.3-1, 9.1-1 | stay weak, no `{gap}` possible | tagged at HEAD, single sentence each: a `{gap}` is refused and a split would be a fragment. Moved to AC-2 of plan/pre-release/spec-config-yang-when-unique-choice.md and the services Blocked-by table |

Other edits: Meta Implementation `mixed` with reason naming openconfig/goyang (vendored) via internal/component/config/yang/loader.go, and Ze's data-tree checks (yang/validator.go, schema_defaults.go, validate_sections.go). readImplementation requires a named component (passes: `internal/...` paths); `mixed` stays counted (implementationCounts), so the gated population is unchanged; render_ledger (rfc-status.md) does not read the field, the site ledger (rfccompliance "Who implements each document") does. Support remaining now "Fifty-nine" (56 + 3 new gaps) and discloses the absent-ancestor mandatory gap and the untested editor commit. Two `Correction 2026-09-30` paragraphs. Journal row in plan/journal/declared-format-contradicts-payload.md (schema mandatory vs group/peer inheritance design).

Owed by the judge / main thread: re-judge STALE 7.6.1-1, 7.6.5-1, 8.3.3-1; `./le rfc index-update` (docs/features/rfc-status.md row for RFC 7950 mirrors Support remaining/coverage); reseal SHIFTED; the pre-existing extraction resign-reason (35 -> 37) from Continuation 1. Delete the throwaway internal/component/config/zz_mandatory_scan_test.go (Continuation 3). validator_mandatory_rfc7950_red_test.go left untracked and untouched.

## Files changed in Ruling 4
- rfc/short/rfc7950.md (Meta Implementation + reason; Support remaining; 7.6.5-1 split into -1/-2/-3; 8.3.3-1 split into -1/-2)
- rfc/corrections/rfc7950.md (two Correction paragraphs)
- rfc/extraction/rfc7950.json (sites 7.6.5:2, 7.6.5:3, 8.3.3:2 remapped; exclusion count unchanged)
- rfc/discrimination/rfc7950.json (new 7.6.1-1 positive record, TestApplyDefaults_NonPresenceContainer)
- internal/component/config/schema_defaults_test.go (tag line only)
- plan/pre-release/spec-config-yang-when-unique-choice.md (Task rows 7.6.5-2, 7.6.5-3, 8.3.3-2; AC-2)
- plan/pre-release/spec-rfc-verdict-fix-services.md (Blocked by: 7 rows)
- plan/journal/declared-format-contradicts-payload.md (one row)
