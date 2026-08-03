# 1127 -- rib-arch-2: Binary `[]byte` Raw Carrier for the Filter IPC

## Context

The runtime filter-update IPC (`ze-plugin-callback:filter-update`) carried the raw UPDATE body
as a HEX `string` (`FilterUpdateInput.Raw` / `FilterUpdateOutput.Raw`, `pkg/plugin/rpc/types.go`)
with hand-rolled `textbuf.StringHexUpper` / `hex.DecodeString` on both ends -- 2x wire expansion
plus explicit encode/decode. rib-arch-2's original goal ("replace the text `Update` carrier with
length-prefixed binary") was scoped DOWN after research (learned in the same spec's Design
Finding): the transport is newline-delimited JSON (no true binary frames), and the text `Update`
is a deliberate format-once/parse-many optimization whose removal is a 9-plugin rewrite for
ambiguous perf.

## Decisions

- **Option A (user-approved slice): binarize the `.Raw` carrier only.** Change `Raw string` (hex)
  -> `Raw []byte`. `encoding/json` base64-encodes a `[]byte` field automatically (newline-safe,
  ~33% expansion vs hex's 2x), the same idiom the codebase already uses for
  `InjectWireRouteInput.UpdateBody`. The text `Update` path is untouched.
- **Pass the payload directly, no copy.** `policyFilterFunc` sets `input.Raw = rawPayload`
  instead of hex-encoding. Safe: every transport (DirectBridge / mux / socket) `json.Marshal`s
  the input before returning, so the plugin always gets a copy -- no aliasing with the reused
  forward-path buffer.
- **`decodeFilterRawOverride` loses the hex step.** It now takes `[]byte` and keeps only the
  4-byte-minimum guard (a valid UPDATE body is withdrawn-len(2) + attr-len(2)).
  `PolicyResponse.Raw` / `PolicyChainResult.Raw` became `[]byte` to match.
- **Two `raw=true` plugins updated in lockstep** (Ze carries no compat burden,
  `ai/rules/go-standards.md`): `filter_family` reads `in.Raw` bytes / returns `out.Raw` bytes;
  `filter_remove_private_as` calls the pre-existing `hasPrivateAS4PathPayload([]byte)` directly
  (the `hasPrivateAS4Path(string)` hex wrapper is deleted).

## Consequences

- The raw carrier is base64-in-JSON, not hex-in-JSON, and the primary raw path (no opt-in hex).
- Behaviour is unchanged: the SAME bytes, a cheaper encoding. Guarded by 7 filter `.ci`
  regression tests (family strip/suppress/teardown; private-AS strip export/import/replace),
  all green, plus the unit tests migrated to the `[]byte` form.
- The text `Update` path removal (all 9 `filter_*` plugins) stays a documented, larger follow-up.

## Gotchas

- Changing a struct field type ripples through EVERY hex call site: `hex.EncodeToString` /
  `StringHexUpper` on the write side and `hex.DecodeString` on the read side all become
  typecheck errors, which is the useful driver -- fix each to the bytes form and drop the now
  unused `encoding/hex` / `textbuf` imports.
- A test that hex-decoded `out.Raw` (`body, err := hex.DecodeString(out.Raw)`) loses its
  decode-error assertion once `Raw` is `[]byte` -- annotate with `// test-relax:` (there is no
  decode step to fail).

## Files

- `pkg/plugin/rpc/types.go` -- `FilterUpdateInput.Raw` / `FilterUpdateOutput.Raw` -> `[]byte`
- `internal/component/bgp/reactor/filter_chain.go` -- encode direct; `decodeFilterRawOverride([]byte)`;
  `PolicyResponse.Raw` / `PolicyChainResult.Raw` -> `[]byte`
- `internal/component/bgp/plugins/filter_family/handler.go` -- `in.Raw`/`out.Raw` bytes
- `internal/component/bgp/plugins/filter_remove_private_as/{filter_remove_private_as,private_as}.go`
  -- `hasPrivateAS4PathPayload(in.Raw)`, hex wrapper removed
- tests: `filter_chain_test.go`, `filter_family/*_test.go`, `filter_remove_private_as/*_test.go`
