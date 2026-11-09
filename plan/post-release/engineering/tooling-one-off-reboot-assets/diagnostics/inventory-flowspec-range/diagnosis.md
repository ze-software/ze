# Ranged scalar fields need an authority proof, not a callback value summary

Source-only diagnosis, November 6, 2026. No evaluator, fixture, or documentation
source was edited. No test, Go tool, formatter, build, or runtime operation ran
in this investigation. MAIN owns the retained execution evidence.

## Five-part diagnosis

1. **Symptom.** The remaining committed-fixture root
   `TestCommittedFamilyHelpersRetainCanonicalNames/flowspec-projection` refuses
   with `ranged element field ownership requires a retained element summary`.
   The sibling returned-slice and direct Family-method cases pass. The failed
   case expects the exact canonical family keys and decode/encode support rows;
   it must not be removed or reduced to a parsing-only check.
2. **Root cause.** `externalexpressions.go` `externalEffects.value` recognizes
   the parser's synthetic second range binding, retains the collection's whole
   capability union and the declared element-type capabilities, then always
   adds a field-value ownership obligation. Calling the retained `String` method
   binds that complete value to its exact `Family` receiver. `projectField` in
   `externalrootfields.go` proves that the receiver's direct `name` field exists
   and is declared builtin `string`, but rejects the carried obligation before
   distinguishing that scalar field from a callback/reference-bearing field.
   The code applies an essential callback-field guard to a scalar capability
   question that can be proved without the stored element's callback summary.
3. **Owning layer.** The external-effect analyzer's root-field projection proof.
   The ordinary immutable metadata reader already supports this finite
   make/range/index/return helper in `familyexpressions.go` `familyProjection`.
   That metadata reader is not the repair site. The input is valid; the guard's
   required facts are too coarse. A static field type does not prove an actual
   callback value, but can prove a closed scalar representation while retaining
   its own method authority and metadata dependencies.
4. **Candidate fixes.** `[workaround]` Whitelist `Family`, `String`, `name`, or
   FlowSpec; remove the range/index ownership marker; or return an empty scalar
   capability without dependencies. `[source]` Add a separate scalar-projection
   proof after exact receiver/member ownership is established. It admits only
   a proved closed scalar representation, constructs that field type's existing
   capability/reflection summary, and retains its enclosing reads and mutation
   dependencies. Callback, interface, and other reference-bearing selections
   still require an actual element-field value summary.
5. **Why reject the workaround.** It can turn a type-derived empty function
   field into a falsely known inert callback, borrow an interface alternative's
   field, hide a named scalar's registration method, or lose the backing-object
   dependency that must invalidate mutable registration metadata.

## Actual data flow

The fixture in `committed_retained_forms_test.go` declares a slice of two
concrete `family.Family` values. Its second range binding `fam` has the retained
static type `Family`; `methods.go` `expression.valueType` already proves this
for immutable slice/array range operands. That proof certifies the type, not
the stored value or a concrete callback field.

`externalEffects.value` retains the aggregate before adding the obligation.
The direct method lookup proves the real `Family.String` declaration and binds
its receiver `f` to that aggregate. Inside `return f.name`, `fieldReceiverType`
finds the receiver's declared `Family` type, and `declaredFieldType` finds the
actual direct member `name string`. The failure is then the unconditional
`receiver.unresolvedFields` branch in `projectField`.

There are two distinct obligations. Static root/member identity must always be
proved. A stored field-value summary is additionally necessary when the selected
field can carry a callable, interface alternative, location, or other reference.
The current code represents only the latter as a string marker, and uses it to
reject both cases.

## Recommended bounded repair

Keep the current known-field projection unchanged. Add a separate operation for
an unresolved collection element's **proved scalar field**, with these gates:

- Prove the exact receiver root and direct field using retained declarations.
  No field-name lookup in an aggregate union can substitute for this step.
- Preserve any actual `receiver.unknown` and any unsupported transfer, such as
  a channel receive. The exception must not consume an arbitrary future string
  placed in `unresolvedFields`; give the collection-element obligation an
  explicit proof category or restrict entry by a proved range/index producer.
- Prove the selected type's closed basic scalar representation. The existing
  `formalShape` classification can supply this fact for retained scalar/alias
  chains. Its depth bound, package binding identity and builtin shadow checks
  remain required. An unresolved generic parameter, interface, pointer, function,
  collection, channel or unsupported external type does not pass this gate.
- Use `typeMethods` for the selected field type rather than inventing an empty
  value. A named scalar can have registration-bearing methods; reflection and
  unsupported method ownership must retain their current consequences.
- Retain any already known field summary and its uncertainty. The scalar proof
  must not erase contradictory or unresolved value facts already observed.
- Preserve enclosing `receiver.reads`. If the selected value is read through
  tracked backing locations, preserve those locations as **read dependencies**,
  not as newly writable references granted by the scalar copy. Metadata mutation
  checks still need the actual backing objects. The aggregate's location set
  can include unrelated child references; do not claim that every such child
  is the selected field's backing location. Reuse proved read provenance, and
  keep an unresolved metadata dependency explicit if that provenance is missing.
- Construct a fresh selected-field result. Do not mutate the original receiver,
  clear all its obligations, or replace the collection's whole capability union.
  Whole-value exposure must still see every callback/location already retained.

The scalar result need not carry unrelated sibling callbacks as its own
callables. Exact scalar-member identity proves that they are not the selected
value. They remain in the original collection/receiver summary and must still
be exposed when that whole value is passed externally. Existing normal scalar
sibling precision must not be replaced by blanket over-aggregation either.

For direct range/index selectors, static root proof and the value obligation
must remain separate. `expression.valueType` already proves the second binding
of an immutable slice/array range. Reuse that exact proof where applicable.
An index needs an independently retained collection/element-type proof; do not
simply remove `fieldReceiverType`'s index refusal. Unsupported forms may keep
their existing explicit refusal. In every admitted form a callback/reference
field still fails the separate value-summary gate.

This operation proves capabilities only. It does not invent a concrete string
or change canonical metadata interpretation. The normal report reader continues
to resolve actual canonical family names through its existing immutable source
rules. This is unrelated to generic-template exposure and to the pending
performance A/B decision; it adds no cache or traversal cutoff change.

## Larger alternative

Preserving complete element-field summaries would also solve this case, but
requires a broader value-domain change: retain collection element summaries
separately from holder fields, keep exact element root identities across
interfaces/assertions, close indexed/aliased writes, and preserve unknown whole
transfers. Merely retaining a flattened `fields` map is unsound: a map key,
child field, or another interface alternative can have the same spelling.
The current callback-transfer tests deliberately refuse until that full proof
exists. A scalar authority proof is the smaller repair for this defect.

## Minimum discriminating tests

1. Keep the exact failed FlowSpec subcase unchanged as the real public report
   witness. Retain its existing RED, then require exact canonical keys, owners,
   and support flags after repair. Run this root alone first.
2. Add a general private type with a scalar field read by a retained method from
   a ranged collection. Give it no family-related names. Require a clean
   capability proof, including any independently proved direct range/index
   selector forms the new code admits.
3. Put a registration callback in a sibling field. Exposing only the proved
   scalar copy stays clean; exposing the whole element/collection still refuses.
   Use fixtures where the private callback/type is not already a separate
   static public entry, so the test actually discriminates this projection.
4. Make the selected scalar a private named scalar with an installer-bearing
   method. Passing that scalar to opaque code must still refuse. The inert
   method control must reach and pass the same projection first.
5. Select actual callback or interface fields from range/index elements. Both
   inert and installer-bearing cases must retain the explicit field-value
   obligation until an actual element summary is implemented. These guards
   must not be inferred only from a scalar positive test.
6. Verify reads and backing-location dependencies in the actual new scalar
   branch, and add a public metadata mutation pair that reaches that branch.
   A failure in an earlier unrelated metadata parser is not this proof. Keep
   the existing ordinary field-metadata public pair unchanged as a control.
7. Keep the channel receive, interface alternative, root namespace and scalar
   sibling controls below unchanged. No source-representation or bound changes
   are needed in these tests.

Existing negative/control roots inspected:

- `TestCommittedExternalRangeTransferRefuses`
- `TestCommittedExternalMixedKnownUnknownCallableRefuses`
- `TestCommittedExternalRangeAssignmentRetainsCallback`
- `TestCommittedExternalFunctionRangeExecutionRefuses`
- `TestCommittedExternalReceivedStructFieldRemainsUnknown` and
  `TestExternalReceivedStructFieldSolver`
- `TestCommittedExternalAssertedOwnerCannotBorrowField`
- `TestCommittedExternalCollectionKeysCannotBorrowMethodOwner`
- `TestCommittedExternalAggregateFieldsCannotBorrowMethodOwner`
- `TestCommittedExternalNestedFieldsKeepScalarSiblingIndependent`
- `TestCommittedExternalFieldMetadataRetainsEnclosingRead`
- `TestCommittedExternalIndirectHelperMetadataWrite`

The current callback-range tests cover whole function elements and assignments;
they do not by themselves establish safety of a newly admitted scalar **field**
fallback. Add the field-specific positive/negative pair rather than declaring
that coverage implicit.

MAIN's first scoped selector remains:

```text
^TestCommittedFamilyHelpersRetainCanonicalNames/flowspec-projection$
```

After independent design/code review, run it and new discriminators under the
standard admitted all-feature/race command, then the affected controls. Update
`docs/architecture/plugin/feature-gates.md` root field ownership prose with the
actual narrow scalar proof. This source diagnosis claims no implementation or
executed restored result.
