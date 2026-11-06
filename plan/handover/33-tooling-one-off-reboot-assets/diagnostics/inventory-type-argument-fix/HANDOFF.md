# Type-argument repair handoff

Source is frozen after self-review. No formatter, Go tool, test, build, runtime operation, or commit ran in this agent.

## Changes

- `externalexpressions.go`: the `IndexExpr` argument uses `indexArgument`. Base evaluation stays first. Existing field clearing and selector obligations remain.
- New `externaltypearguments.go`: proved retained, lexical and built-in types carry only `externalValue.reflection`. Literal types and proved nested type bases use descriptor semantics.
- Ordinary indexes, nonimport field selectors, and opaque selectors of unproved kind keep value flow. Nested retained arguments still reach the same argument transfer.
- `committed_external_type_reference_test.go`: the corrected, reviewed 32-case probe is promoted unchanged. Read-only `cmp` confirms exact equality with MAIN's formatted probe artifact. The original root remains unchanged.
- `feature-gates.md`: one paragraph records deferred type-argument authority and the descriptor/value distinction. Other dirty documentation remains intact.

The new descriptor walk has methods enabled and `valueRoot=false`. It grants no locations, fields or actual callee ownership. Existing exposure activates the summary. Each argument uses a fresh walker and output maps.

The entry guard delegates unavailable, linked and exhausted inputs to `value`. Type-syntax classification follows at most 128 base edges. When that proof exhausts, ordinary traversal preserves the original expression-depth and linkage refusal order.

No method census, receiver cache, completed-descriptor cache, fixed-point bound, retained generic invocation, or top-level `IndexListExpr` rule changed. The generic-template and range-field failures remain open.

## MAIN verification owed

1. Format only the three changed Go files. Compute source identities for all four changed canonical files.
2. Run the unchanged first failing selector on canonical source:

```sh
CLAUDE_CODE_SESSION_ID=01a1a907-3278-7971-98dc-dbc41d4a54a5 CGO_ENABLED=1 \
  bin/le-tooling-resume-01a1a907/le job run \
  label inventory-opaque-type-argument-named-restored-20261106 quiet command \
  go test -race -tags "$tooling_test_tags" -p=1 \
  -run '^TestCommittedExternalOpaqueTypeArgumentAuthority$/^named$/^plugin$' \
  -count=1 -timeout=10m -v ./internal/le/repo/inventory
```

3. Run both promoted roots with that recipe and selector `^(TestCommittedExternalOpaqueTypeArgumentAuthority|TestCommittedExternalOpaqueTypeArgumentDoesNotEscape)$`.
4. Run reached prior `^TestExternal` reflection/value/identity controls and `TestCommittedOpaqueCallableOwnersCannotDisappear`. Preserve existing unsupported generic and lexical-unbound refusals.
5. Inspect ordinary index/selector and type-shadow controls during independent source review. No library spelling proves type identity.

The former private overlay is unnecessary for the promoted probe. If MAIN retains it for the first restoration run, it currently replaces the canonical test with identical bytes.

## Risks and limits

The written fix is uncompiled and untested. MAIN must observe restoration of all 16 false-clean cases and preservation of the ten nonescape cases. The six prior committed fixture failures are separate obligations.

Retained binding ambiguity becomes explicit uncertainty. An opaque selector's unavailable kind is not guessed: its existing origin path and nested argument traversal remain. Its origin certificate still governs external static access.
