# Independent generic-template design review

**Verdict: REWORK before implementation.** The diagnosis identifies the wrong boundary, but the proposed symbolic-state and ambient-closure argument need the conditions below. This is a source-only review; no evaluator edits, Go tools, tests, builds, formatters, or runtime operations ran.

## Established source facts

1. `externalentries_effects.go` `staticExports` sends each exported `ast.Typ` to `typeMethods` as a bare name. `externaleffecttypes.go` `named` binds absent actuals, then its struct-field reflection reaches `externalreflectionroots.go` `interfaceRoot`. The positive header fixtures contain valid unused generic declarations, so their prerequisite refusal is not their intended header discriminator.
2. The concrete refusal is correct at its existing boundary. `external_reflection_domain_test.go` `TestExternalReflectionDescriptorLexicalOwner` requires an unresolved function-lexical formal to remain unknown. Neither changing nil-argument handling globally nor treating every bare generic name as a template is acceptable.
3. A separate certified template operation is compatible with the approved ordinary-reference trust contract. An uninstantiated type declaration is not a concrete runtime descriptor. Analyze its fixed type dependencies conservatively without claiming otherwise. This requires no new unsafe/linkname exception or library whitelist.
4. `externalValue` stores callable, location, origin and reflection authority; `amb` is not a complete catalog of types. `expose`/`promoteReflection` close exposed authority, and `solve` repeats those operations. Merely copying `amb`, or suppressing a missing formal because `amb` currently looks empty, does not prove all actual type arguments were exposed.

## Concrete proof gap

`externalexpressions.go` `value` handles `IndexExpr` by evaluating both operands as values. A bare type argument with an `ast.Typ` object reaches `externalbindings.go` `readObject`; `indexDefinitions` adds type facts for `ValueSpec` and `Field`, not `TypeSpec`. Thus `reflect.TypeFor[hidden]()` appears able to expose the `reflect` origin and a read dependency without exposing `hidden`'s descriptor. `externalcalls.go` takes its opaque-origin branch before retained generic-callable refusal. This is a source-derived counterexample candidate, not an executed false-clean claim.

MAIN must discriminate that path using the real `reflect.TypeFor` API and an unexported retained type whose exported method registers a plugin or YANG item. The methodless control must publish; the registration-bearing variant must refuse for registration authority, not an unrelated prerequisite. The caller must be excluded from static entry exposure by its actual origin relationship, so the test cannot pass by exposing the type through another route.

The selected Go 1.27.3 tree's `src/reflect/type.go` `TypeFor` explicitly returns the type represented by its argument and calls `abi.TypeFor[T]()`. This API-premise read is review evidence only; no host library source becomes an inventory input.

## Minimum safe implementation boundary

1. Create symbolic formals only for the exact retained exported `TypeSpec` being exposed by a certified static entry. Preserve declaration/package/file identity and distinguish this state from absent concrete actuals. Ordinary calls, ordinary reflection and unbound lexical formals retain their existing rules.
2. Carry those identities through actual substitutions, generic aliases, fields and nested instantiations. Keep concrete actuals concrete: a previously visited `Box[T]` template must not hide a later `Box[privateRegistrar]` descriptor. No equality based on spelling or erased argument lists is sufficient.
3. Traverse every fixed descriptor branch and every constraint dependency in defining scope. Preserve methods/origins from `func(privateRegistrar)`, nested generic arguments and constraint method signatures. Constraint `~`/union nodes must either have a justified descriptor traversal or explicitly refuse; they cannot disappear. A constraint is not an actual nil-interface value to prune.
4. State and establish the no-new-authority lemma for externally supplied formals: source-chosen concrete type arguments and all type authority actually transferred to opaque code must already enter the existing closure. Close the `TypeFor` candidate above, or retain refusal where that premise is not proved. Do not add an assumption that external code cannot use ordinary generic instantiation or reflection.
5. Keep generic receiver/bodyless-method refusals, the 128-edge and 128-round bounds, argument preflight/refusal ordering, exact active-path identity, and fresh output ownership. The correctness repair must not implement a pending completed-subgraph cache or the alternative performance design.

## Required discrimination before acceptance

MAIN needs bare methodless struct and alias positives; fixed-field and constraint private-registrar negatives; nested symbolic-to-concrete substitution; same-spelled type/formal and function-lexical shadow controls; the opaque actual-type-argument control above; and existing generic receiver/bodyless/depth refusals unchanged. Keep the five authored assembly prerequisites and their later named header/Alias discriminators intact. Additive tests precede the producer repair; rerun the first exact failure, then all five roots and reached reflection controls. The complete committed fixture population remains a separate obligation.

An actual patch satisfying these conditions needs independent review. The source-only proposal is not an accepted implementation, a complete inventory report, or a reason to clear the pending performance decision.
