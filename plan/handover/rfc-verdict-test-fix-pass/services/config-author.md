# services child, config + config/yang (stem rfc7950), author handoff

Status: PARTIAL, needs a continuation (budget reached at about 90 calls). No verdict stamped, nothing committed.

## Verdicts (15) and child rows

| id | resolution | what now proves each clause | records written | expected verdict | notes |
|----|-----------|------------------------------|-----------------|------------------|-------|
| RFC7950-7.3.4-1 | defect + tests | new `TestRFC7950DerivedTypeInheritsOrReplacesTheDefault` (+: inherit 5, leaf new default 7, typedef new default 8), `TestRFC7950DerivedTypeMustReplaceAnInvalidatedDefault` (-: leaf and typedef narrowing to 6..10 with no new default refused); old pair still tagged | neg `validateTypedefDefault`, pos `yangToLeaf`; old pair re-recorded on `validateLeafDefaults` | enforced | D-8 fixed in `yang_schema.go`: leaves did not inherit a typedef default (read `entry.Default` only), and a leaf narrowing + giving a new default was falsely refused. Failing test observed red before the fix |
| RFC7950-7.6.4-2 | defect + tests | new `TestRFC7950DefaultNotAnIfFeatureEnum` (- default naming if-feature enum refused, + other enum loads) + old pair | neg+pos `validateDefaultNotIfFeature`; old pair re-recorded | enforced | D-8 fixed: no if-feature check existed. Observed red before fix |
| RFC7950-7.6.3-1 | tests | new `TestRFC7950LeafTypeMustNameAnExistingType` (- `type nosuch`, + local and imported typedef) | neg+pos revert `loader.go::Resolve` | enforced | |
| RFC7950-9.6.4.2-1 | blocked (unlisted) | new `TestRFC7950EnumValueWithinInt32` (-2147483649 refused / -2147483648 loads) closes the lower-bound clause | neg+pos revert `Resolve` | stays weak | last sentence: goyang ACCEPTS a restricted enum with a changed value. Untagged RED test `TestRFC7950EnumRestrictionKeepsTheBaseValue`. Same obligation as gap row 9.6.4-2 in `spec-config-yang-loader-structural-checks` |
| RFC7950-7.19-1 (wrong) | blocked (unlisted) | untagged RED test `TestRFC7950ExtensionUsageSubstatementsAreYANGStatements` (usage `m:e "x" { bogus "y"; }` and `{ description; }` are ACCEPTED by goyang) | none | stays wrong | needs a Ze-side walk of extension-usage substatements: design choice, belongs to `spec-config-yang-loader-structural-checks` |
| RFC7950-9.4.4-1 | blocked (unlisted) | untagged RED test `TestRFC7950LengthPartsDisjointAndAscending` (`1..5 \| 3..8` and `10..20 \| 1..5` ACCEPTED) | none | stays weak | same spec as above |
| RFC7950-9.12-1 | tests | new `TestRFC7950UnionAcceptsEveryMemberType` (+ "2001:db8::1" and "dynamic", later members) + old negative | pos revert `validator.go::validateUnion` | enforced | |
| RFC7950-9.1-1 | partial defect + test | new `TestRFC7950IntegerSignedLexicalForms` (+ "+17" for uint8/int8) | pos revert `validateUnsigned` | stays weak | D-8 fixed: `validateUnsigned` refused "+17" (ParseUint takes no sign); red before fix NOT separately observed (Go semantics). Still open: hex/octal default forms (§9.2.1, a module default "0x10" is refused by ParseUint base 10, and "017" must read octal only in a module default), bits/binary/empty/identityref fall to `default: return nil`. Next agent: decide row split or implement |
| RFC7950-9.3.2-1 (wrong) | unresolved, recommend row retire | none | none | stays wrong | the sentence defines the canonical form, which §9.1 binds only to XML output ("When a server sends XML-encoded data, it MUST use the canonical form"); site 9.1:2 is already excluded as XML. Recommend: retire under D-2 with tags moved to a §9.3.1 lexical row (none exists; needs a new row) and site 9.3.2:1 excluded. Needs the main thread's call |
| RFC7950-8.3.1-1 | unresolved, recommend row retire | none | none | stays weak | reply clause ("invalid-value" in `<rpc-error>`) binds a NETCONF server; siblings 8.3.1:1/:6/:7 already excluded feature-out-of-scope. Value-constraint clause is also in gap row 8.1-1. Recommend retire + exclude site 8.3.1:2 + move tags; destination is a gap row, so the main thread decides |
| RFC7950-7.6.1-1 | not started | | | weak | case-node branch and absent-ancestor negative of `ApplyDefaults` |
| RFC7950-7.6.5-1 | not started | | | weak | case-node branch, absent-ancestor branch; `TestValidator_MandatoryField` type check conditional. Its two records show stale in `./le rfc discriminate stem rfc7950` (validator_yang_test.go lines 338, 935) |
| RFC7950-8.1-2, 8.3-1, 8.3.3-1 | not started | | | weak | need the editor commit path (`internal/component/cli/editor_commit.go`) and daemon reload (`cmd/ze/hub/main_reload.go`); cli is outside this child's packages: main thread decides which agent owns that test file |

Child rows (split/narrowing), all added to `rfc/short/rfc7950.md` with sites remapped in `rfc/extraction/rfc7950.json`:

| new row | from | state |
|---------|------|-------|
| RFC7950-7.3.2-1 | 7.3-1 dropped (site 7.3.2:1) | tagged both polarities, `TestRFC7950TypedefTypeMustBePresent`, records written |
| RFC7950-7.9.2-2 | 7.9.2-1 dropped (site 7.9.2:3) | tagged both, `TestRFC7950CaseIdentifierUniqueWithinAChoice`, records written |
| RFC7950-9.4.4-2 | 9.2.4-1 dropped (site 9.4.4:3) | tagged both, `TestRFC7950LengthRestrictionMustNotWiden` (goyang refuses widening), records written |
| RFC7950-7.2.2-1 | 5.1-1 dropped | {gap} loader-structural-checks (probe: goyang accepts a foreign submodule include) |
| RFC7950-7.21.5-2 | 7.21.5-1 dropped (sites :3, :4) | {gap} when-unique-choice |
| RFC7950-9.1-2 | 9.1-1 split (site 9.1:3) | {gap} identityref (identityref, instance-identifier absent) |
| RFC7950-11-2, 11-3 | 11-1 split (sites 11:7, 11:8) | {gap} authoring-checks |
| RFC7950-9.9-1 | narrowing | nothing to do: second reader "covered-by", HEAD row quotes all four sentences |

Not verified by me: that 7.21.5:3 and :4 are one contiguous span (row-quote gate), the ID allocation gate, and that no row needs a correction paragraph (splits only, no demotion, no retirement). Observation: site 7.9.2:2 ("Schema node identifiers ... MUST always explicitly include case node identifiers") also maps to 7.9.2-1, whose quote lacks it; the narrowing audit did not flag it.

## Owed gates (not run by me)

- `./le rfc check` (row quotes, extraction, ID allocation, coverage ratchet, producer-changed records: `validateUnsigned` and `validateLeafDefaults` changed, so any record in any stem naming them re-records).
- `./le go lint run` (the post-write lint timed out); `yang_schema.go` is now 1141 lines (hook warning).
- `./le test unit all`: leaves now inherit typedef defaults through goyang `DefaultValues`, which can change applied defaults in any package that builds the schema.
- `internal/component/config/yang` is RED by design: three untagged tests above.

## Files changed

- internal/component/config/yang_schema.go
- internal/component/config/yang/validator.go
- internal/component/config/yang/loader_rfc7950_clauses_test.go (new)
- internal/component/config/yang/validator_integer_rfc7950_test.go (new)
- internal/component/config/yang_schema_defaults_rfc7950_clauses_test.go (new)
- internal/component/config/validator_union_rfc7950_test.go (new)
- docs/architecture/config/yang-config-design.md
- rfc/short/rfc7950.md
- rfc/extraction/rfc7950.json
- rfc/discrimination/rfc7950.json (written by discriminate-record)
- approvals recorded: `./le rfc approve unit yang.TestRFC7950{EnumValueBoundsAndRestriction,ExtensionUsageLoadsYANGSubstatements,ExtensionUsageRefusesANonYANGSubstatement,LeafTypeMustNameAnExistingType,LengthPartsMustBeDisjointAndAscending}` (my own session-new units, renamed before commit)
