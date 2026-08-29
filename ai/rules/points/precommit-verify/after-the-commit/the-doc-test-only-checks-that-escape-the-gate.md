---
kind: note
level:
stage:
---
`./le doc check verify` and `./le repository generated-check` are separate
native actions. `internal/le/docwiring.Verify` owns the ordered documentation
gate, including `internal/le/docvalid` command and drift checks,
`internal/le/doccheck` links, and RFC freshness. `internal/le/repository` owns
generated repository artifacts. Run the documentation action when the changed
surface selects it; never invoke a retired producer directly.
