# Type-argument repair: independent source review

CLEAN within this repair's scope. No formatter, compiler, test, runtime probe,
or automated gate ran. MAIN's restored and reached regression runs remain owed.

- `externalexpressions.go` `externalEffects.value` wires the existing
  `IndexExpr` argument edge to `externaltypearguments.go` `indexArgument` after
  evaluating the base. Field clearing and element-selector obligations remain.
- `typeArgument` requires retained, lexical, built-in, or literal type syntax.
  It reuses `callableImport`, whose lexical-object check prevents a local value
  from borrowing a same-spelled import. Ordinary index operands and opaque
  selectors of unavailable kind keep their prior value flow; nested retained
  arguments still traverse the same index edge. Ambiguous retained bindings
  remain explicit uncertainty.
- `indexArgument` stores a fresh descriptor summary only in `reflection`.
  Its `valueRoot=false` handles an interface TYPE without applying the absent
  descriptor rule for a nil interface VALUE. No callable owner, writable
  location, backing reference, or field is granted by the new summary.
- `externalcalls.go` `externalEffects.call` joins that summary at an opaque
  call, and `externaleffects.go` `expose` promotes it there. Ordinary callee
  dispatch still reads only `callables`. An unexposed instantiation is not
  promoted. Reflection joins copy their input maps, and each new walk owns
  its output and active path.
- Entrance refusals delegate to the original value path. The classifier and
  descriptor walk remain bounded by 128; no cache, method census, fixed-point
  bound, retained generic invocation, or top-level `IndexListExpr` rule changed.
- The promoted probe preserves the original root and adds the reviewed
  effect-free/registration-bearing and nonescape controls. Its strict refusal
  helper requires the escaped-registration cause and authored source in both
  consumers. The feature-gates paragraph and source anchors match the change.

This verdict does not close the separate generic-template, ranged-field,
actual-report performance, or historical-evidence obligations.
