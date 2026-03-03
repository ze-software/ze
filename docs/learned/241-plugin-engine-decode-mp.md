# 241 — Plugin Engine Decode MP

## Objective

Add three more decode RPCs to the plugin→engine interface: `decode-mp-reach`, `decode-mp-unreach`, `decode-update`.

## Decisions

- Stateless decode RPCs need a registered context even without a peer session — lazily registered via `sync.Once` with ASN4=true as default.
- `NewWireUpdate(body, 0)` fails: context ID 0 is not registered; stateless context must be pre-registered.
- `formatDecodeUpdateJSON()` created separately from `formatFilterResultJSON()` — latter requires peer metadata unavailable in stateless decode.
- Functional test pattern: Python plugin calls `_call_engine()` on Socket A directly; output validated via `expect=stderr:contains=`.

## Patterns

- Stateless decode context: register once at init via `sync.Once`, reuse for all stateless decode RPCs.
- Python functional test for engine RPCs: plugin sends RPC on Socket A, prints result to stderr, `.ci` validates with `expect=stderr:contains=`.

## Gotchas

- Context ID 0 is not a valid registered context — always register a named context, even for stateless operations.
- Stateless decode defaults ASN4=true (safest assumption for unknown peer state).

## Files

- `internal/plugins/bgp/handler/` — decode-mp-reach, decode-mp-unreach, decode-update handlers
- `test/plugin/` — Python functional tests for codec RPCs
