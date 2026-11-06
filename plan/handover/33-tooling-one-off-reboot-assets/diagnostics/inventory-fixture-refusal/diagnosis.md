# Generic template exposure masks assembly proofs

Source diagnosis only, November 6, 2026. No evaluator, fixture, or documentation
source was changed. No Go tool, test, formatter, build, or runtime operation ran
in this investigation. MAIN's cohort remains the execution evidence.

## Five-part diagnosis

1. **Symptom.** Five `TestCommittedAssembly...` roots fail their positive
   prerequisite before exercising the intended header discriminator. Four
   families report `reflected value parameter has no retained argument`;
   `TestCommittedAssemblyGeneratedParameterScopeCannotFallBack` reports
   `reflected value generic argument count is unresolved`. These are evaluator
   capability refusals, not signing failures or observations of malformed
   assembly. The live cohort log is
   `job-inventory-committed-fixture-cohort-20261106-136049b1.log`.
2. **Root cause.** `externalentries_effects.go` `externalEffects.staticExports`
   handles every exported `ast.Typ` through `typeMethods` with a bare identifier.
   This conflates an exported generic declaration with an instantiated concrete
   value type. `externaleffecttypes.go` `externalTypeWalk.named` deliberately
   binds a generic declaration's absent actuals as ambient parameters. Its
   reflection-enabled struct-field branch then calls `reflectedValue` for
   `Field T`. `externalreflectionroots.go` `interfaceRoot` correctly refuses an
   unproved concrete parameter, and that local reflection unknown becomes a
   permanent ambient refusal in `expose`/`promoteReflection`. A generic alias
   whose whole representation is its parameter instead reaches the root's
   argument-count refusal. The wrong operation is selected at static type
   exposure; neither concrete refusal should be globally suppressed.
3. **Owning layer.** The Git-only external-effect analyzer's static declaration
   exposure and type capability domain. Header derivation, Git capture, and
   test expectations are not the owning layer. The check/input/data question is
   answered by incomplete classification of the analyzer's input: a certified
   type template is being submitted to a concrete-descriptor operation. The
   fixtures are valid and deliberately require unused generic declarations to
   leave the report intact.
4. **Candidate fixes.** `[workaround]` Remove/rename the generic fixture types,
   skip generic exports, disable all template reflection, suppress missing
   argument errors globally, or classify the positive refusal as the requested
   header failure. `[source]` Give certified exported generic declarations an
   explicit symbolic-template exposure operation. Its known descriptor edges
   and constraints still contribute authority, while its exact formal objects
   denote arbitrary arguments supplied under the already approved ambient
   capability boundary. Keep concrete descriptor operations unchanged.
5. **Why reject the workaround.** It either weakens the authored header proof,
   misses registration authority revealed by fixed types inside a template,
   or permits genuinely unresolved concrete descriptors. Existing lexical
   refusal and shadow-identity controls must continue to pass unchanged.

## Source trace and the five affected families

`asmHeaderFixture` in `committed_assembly_headers_fixture_test.go` adds a
`syscall` boundary and makes `headerprobe` a retained import. Both registration
sinks reach that opaque origin, so `externalSource.entrySet` chooses the cycle
certificate. In that route, a retained package that does not itself reach the
origin remains a static entry, including this internal header-probe package.
`staticRoots` therefore calls `staticExports` on it.

`asmHeaderLZ4Go` and `asmHeaderShapesGo`, in
`committed_assembly_headers_positive_test.go`, both contain:

```go
type Box[T any] struct { Field T }
```

The `Box` declaration is methodless and need not be instantiated for the failing
positive prerequisite. It is unrelated to whether `go_asm.h` admits or collides
with the independently authored generated names. The same declarations explain:

- `TestCommittedAssemblyCanonicalHeaderBindingsRefuse/repeated-nonempty/go_asm.h`
- `TestCommittedAssemblyGeneratedHeaderBindingsRefuse` lz4/resolved-shapes cases
- `TestCommittedAssemblyGeneratedHeadersPreserveFullReport` lz4/alias-shape cases
- `TestCommittedAssemblyHeaderEvidenceRetainsSnapshot` all four cases

The fifth root, `TestCommittedAssemblyGeneratedParameterScopeCannotFallBack`,
has a stronger explicit prerequisite. It first publishes unused templates such
as `type Box[T any] = T`, a direct generic struct, and concrete uses of the
direct struct. Only its subsequent alias of `Box[struct { Field int }]` is
allowed to cause the named Alias membership refusal/collision. The bare Box
export currently fails first, masking that later discriminator. That failure
has the same type-template/concrete-type cause, with a different first message.

## Required source repair boundary

Add a separate operation at the `staticExports` generic `ast.Typ` boundary.
Do not change `interfaceRoot` or ordinary `collect` so that every absent binding
becomes harmless. `TestExternalReflectionDescriptorLexicalOwner` explicitly
requires an unresolved function-lexical parameter in `func(Leaf)` to remain
unknown and never borrow a same-spelled package type's methods.

The template operation needs an explicit identity-bearing symbolic parameter
state, created only from that retained declaration's exact `TypeParams` and
defining package/file. A missing concrete argument remains a different state.
Substitution through aliases, fields, nested generic arguments, and constraints
must preserve those identities and their scopes. The existing 128-edge bounds,
source-order refusals, and separately owned output unions still apply. A bare
generic name is admitted as a template only at this certified entry operation;
it must not become a general exemption for invalid concrete expressions.

For an externally supplied symbolic argument, the approved ordinary-reference
trust boundary limits its retained registration authority to the existing
ambient capability pool. Recompute through the existing fixed point rather
than caching an answer. Traverse all fixed descriptor branches and constraints
normally. This implication needs independent review before implementation;
these source notes do not themselves prove a proposed collector sound.

In particular, neither of these shortcuts is acceptable:

```go
type Carrier[T any] struct { Hidden func(privateType) }
type Bounded[T interface { Inspect(privateType) }] struct { Value T }
```

Skipping a whole generic export or disabling its entire reflection walk would
drop fixed descriptor authority from `privateType`. A private type's exported
method may perform a registration even when the generic body has no method.
The symbolic formal itself grants no fabricated retained reference, but known
descriptor edges remain observable. A constraint's interface is a descriptor
edge here, not an actual nil-interface value to prune.

Existing generic receiver/bodyless-method unsupported refusals remain unchanged.
This correctness repair does not implement either pending inventory performance
route, alter depth 128, add a library whitelist, or expand Git-only inputs.

## Minimal discriminating work for MAIN

Before a repair, add one small committed-source positive using the existing
assembly fixture with only a methodless exported `Box[T any] struct { Field T }`
and the ordinary numeric body; no generated-header include is needed to reach
the wrong static exposure. Its refusal is predicted by the source trace, not
yet independently executed. Add the bare generic-alias variant separately.

The same additive test package must also include a fixed descriptor and a
constraint descriptor whose private type method performs plugin/YANG
registration. Those cases must still refuse, so a blanket template skip cannot
pass. Include nested symbolic substitution and same-spelled parameter/package
identity controls. Keep the existing unbound concrete reflection test unchanged.

MAIN can first run the existing fast failed subcase alone, under its standard
admitted all-feature/race command:

```text
-run '^TestCommittedAssemblyGeneratedHeadersPreserveFullReport/lz4-shape$'
```

After a separately reviewed source fix, rerun that exact subcase and the new
discriminators, then the five failed roots and the affected reflection controls.
Run `TestExternalReflectionDescriptorLexicalOwner` unchanged. The full committed
fixture cohort remains owed, including any members not executed before the
current package deadline. Do not repeat passed tests solely for a timestamp.

The named assembly failure must then occur only after its positive prerequisite
passes. A generic capability refusal is not an acceptable substitute for the
Alias or header-collision cause. Documentation to update with the actual repair
is `docs/architecture/plugin/feature-gates.md`, static entries and reflection
authority; no such update is claimed by this source-only diagnosis.
