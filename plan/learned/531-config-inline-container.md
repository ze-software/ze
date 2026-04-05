# 531 -- Config Inline Container

## Context

The hierarchical config display (`show`, editor viewport) used braces for every container, even single-child ones like `remote { ip 192.0.2.1 }`. This added visual noise. The user wanted containers with one child displayed inline: `remote ip 192.0.2.1`. Config storage (set format in zefs) was unaffected -- display only.

## Decisions

- Chose automatic brace insertion (ABI) for the parser, same principle as the tokenizer's existing ASI (automatic semicolon insertion). When `parseContainer` expects `{` but finds a known child name, it injects virtual braces and parses one child inline.
- Only leaf children (values/multiValues) trigger serializer inlining, over also inlining container children. This naturally prevents cascading without needing a `parentInlined` flag.
- Hardcoded `maxInlineDepth = 1` as a constant, over making it a config option. Documents the design constraint in code.
- Parser is permissive (accepts both forms), serializer enforces the depth limit. Follows "liberal in what you accept, conservative in what you produce."

## Consequences

- All `show` and editor viewport output now uses inline form for single-leaf containers. Existing `.ci` functional tests and `config fmt` tests needed expected-output updates.
- `mergeAtContext` in `model_load.go` is text-based and relied on `{` to find container blocks. Had to add inline-container expansion to handle merge into inlined containers. This is fragile -- a tree-level merge would be more robust.
- Round-trip is preserved: `Parse(Serialize(tree))` produces identical trees. Verified by `TestInlineContainerRoundTrip`.

## Gotchas

- `mergeAtContext` broke silently because it scans for `{` to locate target containers. When the target was inlined (no braces), the merge content was simply lost. No error returned. This is a class of bug: text-based tree manipulation is fragile when the text format changes.
- Presence containers must be excluded from ABI because they already handle word tokens after the container name as flag/value syntax.

## Files

- `internal/component/config/serialize.go` -- `canInlineContainer`, `serializeContainerInline`, `writeInlineLeaf`, `maxInlineDepth`
- `internal/component/config/parser.go` -- ABI in `parseContainer`
- `internal/component/config/serialize_annotated.go` -- inline in annotated view
- `internal/component/cli/model_load.go` -- `mergeAtContext` inline expansion
