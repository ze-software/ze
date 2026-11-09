# Independent ranged-scalar test review

Verdict: **REFUSE this packet as the completed scalar-isolation discriminator**
until the deferred-reflection domain is checked. The additions are otherwise
valid source tests. No formatter, compiler, Go tool, test, runtime operation or
approval command ran in this review.

## Preserved assertions and valid inputs

Both canonical files still match their `before/` snapshots. The diff changes
imports and appends new roots/helpers; every original body and assertion is
unchanged. Neither baseline path has a HEAD blob, so this review uses the exact
provided current-source snapshots, rather than inventing a historical test.
No RFC-tagged unit is changed, and no weakening/RFC approval row is owed.

The authored Go uses ordinary named structs/scalars, explicit composite
elements, slice/array range, existing-target assignment, lexical shadowing,
channel receive and pointers. No invalid generic alias or unsupported generic
invocation is introduced. `externalEffectsSource` supplies the declared opaque
`order` import, and the existing fixture supplies both sink-to-origin cycle
witnesses. The ordinary caller and its private installer types do not become
unrelated external static entries.

`externalEffectsPositive` checks the useful complete report and Plugins result.
Registration negatives retain both no-partial-output checks and require the
specific escaped-registration cause plus an authored call/helper or correct
sink source. The field-obligation and channel guards require their actual
producer categories. The metadata pair requires a mutation-producing cause,
so an earlier field or parser refusal cannot certify that negative.

## Required correction: descriptor-only sibling authority

`externalValue` has two separate authority domains: `callables` and
`reflection`. `externalEffects.expose` promotes the latter through
`promoteReflection`, so an unrelated reflected method is an observable escaped
capability even when `callables` is empty.

`TestExternalRangedScalarReadDependencies` checks ordinary callables, locations,
reference shape, unknown and selector obligation, but never checks
`result.reflection`. Its own `scalarParcel` has an exported `Text` method.
A defective scalar projection that copied `receiver.reflection` into the
builtin-string result could pass every written assertion in this solver root.

The committed sibling pair does not cover that distinction: its sibling is a
`func()` value, with no method-bearing parameter/result type. The named-scalar
test proves that the selected type's methods survive, but not that an
unselected sibling's descriptor authority stays private.

Before accepting the packet:

1. Require the builtin-string solver result to have no reflected methods,
   reflected origins or reflected unknown. Its ordinary origin set should
   likewise be empty in this wholly retained, import-free selected fixture.
2. Add a scalar-only versus whole-element pair with a private named sibling
   type, or callback-signature type, whose exported method can install the
   fixture registration. Keep that sibling's ordinary callback inert/absent
   in this case so the whole-element negative identifies descriptor authority.
   Scalar-only exposure must preserve the useful report; whole-element
   exposure must reach the same precise escaped-registration cause in both
   consumers. Reuse the existing cycle witnesses and private caller scope.

This does not ask for direct-index scalar support, a broader value domain, or
a weaker existing obligation.

## Unit decisions

| Added unit | Decision | Reason |
|---|---|---|
| `TestCommittedExternalRangedScalarSiblingIsolation` | REFUSE as complete isolation coverage | Add the descriptor-only sibling pair above; the current actual-callback pair remains valid and must stay. |
| `TestCommittedExternalRangedScalarMethodReceiver` | APPROVE source | Valid unrelated receiver and useful full-report witness; its baseline must still be attributed by MAIN. |
| `TestCommittedExternalRangedNamedScalarKeepsMethods` | APPROVE source | The inert/body-only installer pair preserves selected named-scalar authority. |
| `TestCommittedExternalElementReferenceFieldObligation` | APPROVE source | Direct controls precede explicit range/index callback, interface and pointer obligations. |
| `TestCommittedExternalRangedScalarUnknownTransfer` | APPROVE source | The same known range/scalar path distinguishes the actual channel transfer. |
| `TestCommittedExternalRangedScalarMetadataBacking` | APPROVE source | Both halves traverse scalar selection; only the negative exposes the actual registered pointer and requires metadata mutation. |
| `TestExternalRangedScalarReadDependencies` | REFUSE pending full authority checks | Add deferred-reflection and ordinary-origin emptiness checks above. |
| `TestExternalRangedScalarFormalProvenance` | APPROVE source | Actual nonempty/empty collection sources are distinct from missing and mixed ungrounded caller equations. It queries the same production source-equation APIs as validation, not current temporary return maps. |

MAIN still owes formatting and execution. A compile, signing, namespace,
metadata-parser or unrelated ownership failure is not the requested scalar
baseline. The solver tests do not replace the unchanged full FlowSpec
canonical-family/support witness. The tests check membership of the actual
collection in the read set; they do not independently prove that every other
aggregate child location is a legitimate backing read. The eventual source
review must retain that distinction, or add a focused sibling-location control
if its implementation proposes such a conversion.
