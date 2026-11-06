# Inferred interface descriptor authority

Source-only audit, November 6, 2026. No test, compiler, formatter, Go tool or
runtime operation ran. The current evaluator was not edited. The proposed
runtime witness and committed discriminator are unexecuted.

## Symptom and source classification

The suspected symptom is a complete report after an opaque generic call receives
a typed-nil private interface whose method signature reveals a private type with
an exported registration method. This is not an observed false completion yet.

`externalcalls.go` `externalEffects.call` evaluates arguments with `value`, then
the opaque-origin branch exposes those values plus the callable and ambient
union. There is no separate transfer of an inferred compile-time argument type.
For a declared private interface variable, `externalbindings.go` `readObject`
joins `typeMethods` of its declared type. The ordinary type walker returns an
empty capability value for `ast.InterfaceType`; a method declaration in the
interface has no retained implementation. `externalreflection.go` `reflectedValue`
also returns an empty summary when `interfaceRoot` proves an interface value.
There is therefore no authority from the private interface's signature here.

The explicit-argument repair in `externalexpressions.go` reaches `indexArgument`
only for `ast.IndexExpr`. An implicit call such as `cmp.Or(value)` contains no
such expression for the private interface. `expose` sees no descriptor summary
to promote. Authored import-cycle witnesses and private declarations can exclude
the hidden method from independent static entries, isolating this path.

The check's data is incomplete: runtime value authority and potentially inferred
static type authority are different inputs. Ordinary nil-interface suppression
is correct as a value rule. It is not proof that an opaque generic body cannot
obtain its inferred type descriptor.

## Primary Go premises

The selected Go tree is
`/home/thomas/.cache/ze/stdpatch/aeca112d3dcc206bdff51822fea40919a276d63cd2a1e68a0e4e2bc230d56615/goroot`.
Its `src/cmp/cmp.go` declares `Or[T comparable](vals ...T) T`. Its
`src/reflect/type.go` documents that `TypeFor[T]` returns T's descriptor even
though `TypeOf` of a nil interface returns nil. Interface `Method` supplies the
method signature without a receiver, so `Out(0)` can reveal the private result
type. A reflected new value of that result type can expose its exported method.

These primary source reads establish the proposed API shape. MAIN must execute
the runtime witness to confirm the full inference and invocation premise. They
are research inputs only, never inputs to the Git-only inventory analyzer.

The actual `cmp.Or` implementation does not use reflection or call the hidden
method. This probe makes no upstream behavior claim. The approved analyzer model
allows arbitrary ordinary-Go behavior in an uninspected generic implementation;
an unknown implementation with the same generic calling convention could use
`reflect.TypeFor[T]` and invoke the method. Its signature and body are outside
the retained Git input, so library spelling cannot exempt it.

## Owning layer and candidate repairs

The owner is opaque-call capability transfer, adjacent to argument exposure in
`externalcalls.go`, rather than ordinary callee dispatch or interface runtime
value reflection.

- `[workaround]` Treat nil interfaces as universally inert, exempt `cmp.Or`, or
  suppress the new negative. That loses an ordinary-Go type-descriptor path.
- `[source]` Model the static authority potentially supplied by generic
  inference at an opaque call separately from the argument's runtime value.
  Prove the exact declared type, substitutions and origin boundary, and keep
  descriptor methods out of ordinary callable ownership. If that proof cannot
  be made, retain uncertainty rather than publishing completeness.

Do not blanket-change `reflectedValue`. Existing direct nil-interface solver
positives remain valid as value-only statements. Known retained calls that ignore
their arguments and local uncalled instantiations still transfer no authority.
The current generic walk may already conservatively include argument methods in
ordinary value summaries; this audit does not claim that every phantom generic
interface exposure is presently a complete-report positive. Standard reflection
also does not automatically establish a public API for recovering arbitrary
phantom type arguments. A future repair must not assume either premise.

## Probe contracts

The private overlay adds two roots without changing any original test:

1. `TestExternalInferredInterfaceRuntimeAuthority` calls a generic helper with
   `cmp.Or` of a typed-nil private interface. The helper names no private type:
   it follows T's interface method result descriptor and invokes its exported
   method. A local integer counter proves actual invocation. There is no unsafe
   operation, mutable package state or product registry mutation.
2. `TestCommittedExternalInferredInterfaceAuthority` uses plugin and YANG cases.
   It first proves the complete useful report with an inert hidden method, then
   changes only that method to `install(0)`. The inferred opaque-call case must
   produce the precise escaped-registration cause and retained source attribution
   in both consumers. Two nonescape controls must remain complete: a local
   uncalled explicit instantiation, and a known retained nongeneric ignore call.

Both sinks receive authored `cmp` import-cycle witnesses. The caller reaches
that origin and its hidden type and interface remain private. A namespace,
compile, signing or inert-prerequisite failure is not the proposed semantic RED.

The overlay is not a repair, accepted proof, or scope expansion. MAIN owns all
execution and any subsequent implementation decision after actual results.
