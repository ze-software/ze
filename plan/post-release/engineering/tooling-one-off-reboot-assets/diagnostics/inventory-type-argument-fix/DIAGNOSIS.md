# Opaque type-argument authority

Status: the approved source repair and regression tests are written. MAIN verification remains owed. No runtime operations occurred in this package.

## Diagnosis

1. Symptom: MAIN's named/plugin probe publishes the complete ordinary report after only `hidden.Trigger` changes to invoke registration. Its inert prerequisite passes.
2. Root cause: `externalexpressions.go` `externalEffects.value` handles `IndexExpr` by joining both operands as values. A named type argument reaches `externalbindings.go` `readObject`, which has no `TypeSpec` definition equation. Its reflected methods disappear.
3. Owning layer: expression capability construction at generic instantiation. `externalValue.reflection` already represents deferred descriptor authority. `expose` and `promoteReflection` already activate it at opaque exposure.
4. `[workaround]`: mark familiar generic library calls inert, expose all private types globally, or suppress the new negative. Each changes the approved authority boundary or hides the missing transfer.
5. `[source]`: identify proved type arguments and retain their descriptor authority on the instantiated abstract value. Keep that authority separate from ordinary callee ownership.

The input is valid Go, and the registration uncertainty expectation is correct. The capability construction omits a required fact.

## Planned change

- Keep `IndexExpr` base evaluation first. Preserve its existing field clearing and selector obligation.
- Classify an index argument as a type only from retained declaration identity, a built-in type, an exact lexical formal, or literal type syntax. Resolve parentheses, pointer types, and nested instantiations through a proved type base.
- Preserve value indexing for variable, constant, function and nonimport field bases. An opaque imported selector whose kind is unavailable keeps its current origin flow. Ambiguous retained binding is explicit uncertainty.
- For a proved type, use `externalReflectionWalk.value` with the exact lexical scope, methods enabled, and `valueRoot=false`. That exposes an interface's descriptor instead of applying nil-interface value suppression.
- Store the result only in `externalValue.reflection`. Do not add descriptor methods to `callables`, locations or fields. Existing exposure activates the summary only when the value reaches opaque execution.
- Keep the 128-edge limit and original child order. Do not change reflection collector identity, query ownership, method census, caches, fixed-point bounds, retained generic invocation, or top-level unsupported `IndexListExpr` handling.

Expected source files: a small `IndexExpr` branch in `externalexpressions.go`, one focused `externaltypearguments.go` helper, the approved additive probe tests, and one contract paragraph in `feature-gates.md`.

## MAIN proof queue

The named/plugin semantic RED is retained in `job-inventory-opaque-type-argument-named-baseline-20261106-706c604e.log`. MAIN also classified all 32 probe cases before releasing the source freeze.

After implementation, MAIN must run the same named/plugin probe first, then both probe roots and the reached prior reflection/callable-ownership controls. MAIN owns formatting, tests and all other runtime operations.

The five generic-template prerequisite failures and the range-field failure remain separate repairs. This plan changes neither the Git-only trust contract nor the pending performance decision.
