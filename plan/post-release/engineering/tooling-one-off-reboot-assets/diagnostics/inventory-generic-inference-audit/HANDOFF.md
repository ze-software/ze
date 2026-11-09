# Inferred generic interface probe

SOURCE ONLY. No formatter, test, Go tool, compiler, runtime operation or evaluator
edit ran. `before/` is the exact source read after the explicit type-argument
repair; `after/` adds imports and two roots only. `overlay.json` replaces that
one existing test file in the main checkout. Obtain independent actual-delta
review, then MAIN formats and hashes the selected source before execution.

First confirm the real Go inference/ordinary-reflection premise:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label inventory-inferred-interface-runtime-premise-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/inventory-generic-inference-audit/overlay.json \
  -run '^TestExternalInferredInterfaceRuntimeAuthority$' -count=1 -timeout=3m -v \
  ./internal/le/repo/inventory
```

This root requires a nil runtime interface and actual invocation of the hidden
type's exported method through its inferred descriptor. It touches only a local
counter; it does not call a product registry. Its PASS is the language premise,
not the analyzer verdict.

Then isolate the committed plugin discriminator:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label inventory-inferred-interface-plugin-baseline-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -overlay /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-11-06-080923/scratch/inventory-generic-inference-audit/overlay.json \
  -run '^TestCommittedExternalInferredInterfaceAuthority$/^inferred-call$/^plugin$' \
  -count=1 -timeout=10m -v ./internal/le/repo/inventory
```

The intended RED is a complete published report after the inert prerequisite
passed and only the hidden method body changed to registration. All other
failure classes remain precondition failures, not that evidence. The strict
shared/local refusal helpers already present in canonical source are reused.

Finally classify all six cases with the same command and exact root selector:

```text
-run '^TestCommittedExternalInferredInterfaceAuthority$'
```

The nonescape controls must stay complete even with an effectful hidden method.
Retain the runtime premise, baseline log, exact source identities and resulting
case population before any opaque-call transfer repair. No implementation is
part of this artifact. Do not run from an archive or source-tree copy.
