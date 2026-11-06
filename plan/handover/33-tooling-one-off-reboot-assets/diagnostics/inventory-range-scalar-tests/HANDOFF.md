# Ranged scalar field tests — deferred authority included

Source-only test-first package. No evaluator, canonical test or documentation
source changed. No formatter, Go tool, test, build, script, Docker operation or
other runtime code ran. MAIN must review and execute the artifact.

## Files and preserved inputs

`overlay.json` selects two existing test files in the main checkout:

| File under `internal/le/repo/inventory/` | Private change |
|---|---|
| `committed_external_root_fields_test.go` | Add `strings` import, seven committed roots and private fixture helpers |
| `external_received_field_solver_test.go` | Add `go/ast` import, two selected-source solver roots and a method locator |

`before/` contains exact canonical source bytes. The initial artifact writer
added one final blank line; that draft-only error was corrected before handoff.
Read-only `cmp` then matched both canonical files. All original function bodies
and assertions remain unchanged. `after/` is deliberately unformatted. MAIN
must format only those two selected copies and compute source/overlay hashes.

The package reads the system and le-tooling doors, the Go style guide, the
root-field/formal-provenance contract in `feature-gates.md`, and the diagnosis
and independent REWORK in `../inventory-flowspec-range/`.

The first draft and its independent REFUSE are preserved under `refused-v1/`.
That review found missing deferred-reflection checks. This revision adds the
descriptor-only pair and explicit empty authority-domain assertions below.
It requires a fresh actual-delta review before execution.

## Required tests

- `TestCommittedExternalRangedScalarSiblingIsolation` requires a complete useful
  report when only a builtin scalar leaves a ranged element containing an
  installer callback sibling. The only changed operation then exposes the whole
  element and requires the exact escaped-registration cause from both public
  consumers. Slice, array, returned source, existing `=` target and a fresh
  binding shadowing a callback-bearing outer name retain their own identities.
- `TestCommittedExternalRangedScalarDescriptorIsolation` uses an absent or inert
  sibling callback whose signature names a private type with an exported
  installer-bearing method. The scalar field must not inherit that descriptor
  authority. Only changing exposure to the whole element may reach it, and both
  consumers must report the precise attributed escaped-registration cause.
- `TestCommittedExternalRangedScalarMethodReceiver` calls an unrelated retained
  scalar-returning method through a fresh range binding. It complements the
  unchanged failing canonical-family/support witness; it does not replace it.
- `TestCommittedExternalRangedNamedScalarKeepsMethods` changes only the exported
  method body on a private named scalar. The inert control must pass the same
  field projection before the installer-bearing variant must refuse.
- `TestCommittedExternalElementReferenceFieldObligation` first checks the direct
  known callback/interface/pointer field. Inert direct controls publish a full
  report; installer direct controls must reach the semantic sink. Both range
  and index selections still require their explicit element-field ownership
  proof. A scalar exception cannot certify an empty callback from its type.
- `TestCommittedExternalRangedScalarUnknownTransfer` keeps a known element type
  and direct scalar member, but changes only an earlier whole-element channel
  transfer in a public entry. The channel cause must survive field selection.
- `TestCommittedExternalRangedScalarMetadataBacking` traverses the same scalar
  branch in both halves. A scalar copy remains clean; adding the actual writable
  registered object to the same opaque argument must produce a real metadata
  mutation cause, not an earlier field/provenance refusal.
- `TestExternalRangedScalarReadDependencies` solves production-selected source,
  then inspects the actual retained method result. It requires the exact
  collection read identity, no writable locations, no unrelated callables and no
  ordinary origins or reflected methods/origins/unknown, and no remaining
  selector/unknown obligation. This is the direct dependency check;
  the metadata pair alone would not establish the returned read map.
- `TestExternalRangedScalarFormalProvenance` queries the actual indexed source
  equations. Nonempty and explicit empty retained collections must ground their
  own method receiver. Missing arguments and an additional ungrounded incoming
  caller still refuse. Static type, name or a temporary empty summary is not a
  witness.

Registration negatives reuse the unchanged shared no-partial-report/Plugins
oracle, then require `external registration may execute through an escaped
capability` and an authored caller/helper or correct semantic sink attribution in
both consumers. The generic `registration capability is unknown` prefix does
not satisfy them. Metadata negatives additionally require one of the actual
mutation-producing causes used by `externalEffects.solve`.

## Scope and source validity

No direct-index scalar positive was added. That would be candidate support
requiring a separate source-design decision; the required range repair does not
create that obligation. Existing index callback/interface/reference guard tests
remain strengthened by the new explicit field cases.

Fixtures use ordinary structs, receiver methods, slice/array range declarations,
range assignments, concrete pointers and channel types. They add no generic
invocation, alias-to-type-parameter, iterator support, host-library inspection or
new trust assumption. Opaque operations use the existing authored import-cycle
witnesses. Hidden installer callbacks/types do not become unrelated public
entries. Both semantic variants retain the ordinary full report oracle.

The metadata pair's element is a pointer to a retained concrete object. Its
scalar copy grants no writable location; the negative deliberately exposes the
original pointer after that same read. The selected-source dependency test
separately checks the exact read map and avoids inferring it from a failure on
another path.

## MAIN execution owed

Do not run before independent actual-delta review. Check canonical-before
equality and coordinate the TypeArg implementation writer before compiling.
Use the main checkout only and the established all-feature/race admission recipe.
All root names below are future commands, not recorded outcomes.

First isolate the source-equation boundary without signed Git fixtures:

```text
-run '^TestExternalRangedScalarFormalProvenance$'
```

Then exercise the unrelated committed receiver witness:

```text
-run '^TestCommittedExternalRangedScalarMethodReceiver$/^plugin$'
```

For these selectors, the existing evaluator is expected to fail the positive
range cases. Missing/mixed incoming controls must keep their attributed refusal.
A compile, signing or unrelated namespace failure is not this baseline.

Run the complete new population after classifying those baselines:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label inventory-range-scalar-baseline quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/inventory-range-scalar-tests/overlay.json \
  -run '^(TestCommittedExternal(RangedScalarSiblingIsolation|RangedScalarDescriptorIsolation|RangedScalarMethodReceiver|RangedNamedScalarKeepsMethods|ElementReferenceFieldObligation|RangedScalarUnknownTransfer|RangedScalarMetadataBacking)|TestExternalRangedScalar(ReadDependencies|FormalProvenance))$' \
  -count=1 -timeout=10m -v ./internal/le/repo/inventory
```

Keep `TestCommittedFamilyHelpersRetainCanonicalNames/flowspec-projection`
unchanged and run it alone after the eventual repair. No implementation belongs
to this package. Parent owns any source edits, independent reviews, formatting,
signed fixtures, scoped controls, raw-log retention and commits.

## Do not assume

Source self-review is not a test result. No baseline or restored PASS is claimed.
The new fields/provenance representation is not specified by these tests.
`externalFormalProofs` and method result maps are existing private production
interfaces used for direct solver checks. If a correct refactor moves their
representation, preserve these semantic checks through its real interface;
do not replace them with a check of a constant or of the new test's own data.
