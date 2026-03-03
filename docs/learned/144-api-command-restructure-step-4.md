# 144 — API Command Restructure Step 4: BGP Namespace Foundation

## Objective

Create the `bgp` namespace with introspection commands and move plugin configuration handlers (`session sync`, `session api encoding`) under `bgp plugin *`. Establishes the `bgp` prefix that subsequent steps build on.

## Decisions

- Separated `encoding` (json|text — overall message structure) from `format` (hex|base64|parsed|full — wire bytes representation within JSON). Previously conflated under a single `encoding` setting.
- Removed `WireEncodingCBOR` — incompatible with line-delimited protocol; no migration path.
- `format` is ignored when `encoding=text` — cleaner API than rejecting `format` commands in text mode.
- `FormatRaw` (existing) and `FormatHex` (new) coexist temporarily; unification deferred to when output code is updated.

## Patterns

- BGP introspection commands (`bgp command list`, `bgp command complete`) query the live dispatcher registry — no separate metadata store needed.
- Process state fields (`encoding`, `format`) stored as `atomic.Value`; not yet consumed by output code at this step.

## Gotchas

- `FormatRaw = "raw"` already existed in the codebase; new spec specified `FormatHex = "hex"`. Both constants coexist until output code is updated — a future session must unify them.

## Files

- `internal/plugin/bgp.go` — new, BGP namespace handlers
- `internal/plugin/types.go` — removed CBOR, added Format constants
- `internal/plugin/process.go` — added encoding/format fields and accessors
