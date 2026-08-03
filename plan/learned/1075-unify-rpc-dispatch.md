# 1075 -- unify-rpc-dispatch

## Context
Each plugin->engine RPC operation existed up to three times, linked only by magic-string
method names (`ze-plugin-engine:*`): a JSON handler (socket path), a Direct handler
(in-process, no socket), and a typed DirectBridge fast-path slot -- spread across two
hand-maintained `switch` tables in `dispatch.go` plus a hand-written `Set*` list in
`wireBridgeDispatch`, with a fourth per-op branch table on the SDK side. Nothing forced
them to agree and coverage had already drifted (subscribe/unsubscribe had no bridge slot;
route-install/remove had no Direct arm; inject-wire-route/batch-validate were bridge-only).
The CORE handlers (`dispatchCommand`, `deliverEvent`, `forwardCached`, Loc-RIB apply) were
already single-source; only the DISPATCH/WIRING tables were triplicated. Goal: one entry
per op from which all three transports derive, so adding an op touches one place and the
paths cannot drift. Behavior-preserving internal refactor.

## Decisions
- **One `engineOp` registry in the SERVER package** (`dispatch_registry.go`), NOT the leaf
  `registry` package the spec's Files-to-Modify named. Chose the server package over the leaf
  because built-in ops bind `*Server` methods, `*process.Process`, and bridge types that the
  leaf registry (by design: no plugin-impl deps) cannot import. Codec RPCs stay in the leaf
  `CollectRPCHandlers`, consulted as the fallback exactly as before. A-2 ("bare codec-func
  shape suffices") was BROKEN: the return shape `(any,error)` suffices but the input needs proc.
- **`handle func(*Server,*process.Process,json.RawMessage)(any,error)` with proc PASSED, not
  captured**, over a per-request closure capturing proc. A method expression `(*Server).opX` is
  a static value -> zero per-request allocation (R-3). typedWire descriptors capture proc but
  only once per process at wiring time (same as the old inline `Set*`).
- **Serve wrappers derive the sent error from ONE source** (`rpcErrMessage` = `RPCCallError.Message`
  else `err.Error()`) over each path formatting its own. This fixed a latent bug where the JSON
  path sent `RPCCallError.Error()` (which prepends `"rpc error: "`) while Direct returned the bare
  error -- emit-event errors differed across transports. Now byte-identical (AC-2).
- **`rpc.Method*` string constants** in `pkg/plugin/rpc/types.go`, shared by SDK + engine, over
  inline literals on both sides -- structurally prevents the SDK's method string drifting from
  the engine's.
- **Closed the coverage gaps as by-products**: route-install/remove gain a Direct arm from
  derivation (AC-5); inject-wire-route/batch-validate gain JSON-codec fallbacks (`InjectWireRouteInput`/
  `BatchValidateInput` + `opInjectWireRoute`/`opBatchValidate`), and the SDK's `InjectWireRoute`
  (was "bridge not available" error) and `BatchValidate` (was hand-rolled stride-6 string) now use
  them (AC-6). Every SDK engine method now follows ONE shape: typed slot if available, else
  `callEngine*` with an `rpc.Method*` constant.

## Consequences
- Adding a plugin->engine op = add one `engineOps` entry (method + handle + optional typedWire).
  JSON, Direct, and bridge wiring all pick it up; `TestPluginRPCRegistryCoversAllPaths` fails if
  the method set or typed-descriptor set drifts.
- The two magic-string switches and the hand-written `Set*` list are GONE, not left alongside the
  registry (`grep 'case "ze-plugin-engine:' dispatch.go` = none). `wireBridgeDispatch` is a loop
  over `typedWire` descriptors; only the generic `SetDispatchRPC` is still set explicitly.
- emit-event's JSON error text dropped a redundant `"rpc error: "` prefix (now matches Direct). No
  test asserted the old string. Any consumer that string-matched the doubled prefix would change.
- batch-validate's SDK fallback no longer routes through the `request bgp adj-rib-in batch-validate`
  command (which applied command authz); it uses the JSON codec -> `GetBatchValidator`, exactly as
  the typed bridge slot already did (the typed path never applied command authz). The command is
  unchanged and still CLI-reachable/tested.

## Gotchas
- **The spec's Wiring-Test cited `TestRPCRegistrationExpectedMethods` as the engine-op drift guard,
  but that test covers `AllBuiltinRPCs()` -- a SEPARATE `ze-system:*`/`ze-plugin:*` `RPCDispatcher`
  mechanism (command.go), NOT `ze-plugin-engine:*` dispatch.** They are two unrelated registries.
  The `ze-plugin-engine:*` ops were never in `AllBuiltinRPCs`. Built the correct guard
  `TestPluginRPCRegistryCoversAllPaths`; the cited test passes unchanged.
- **`RPCCallError.Error()` prepends `"rpc error: "`** (`pkg/plugin/rpc/message.go`). Sending
  `err.Error()` for an RPCCallError double-prefixes on the wire. Send the raw `.Message`.
- **`return nil, nil` from a `(any,error)` handler trips the `nilnil` linter** (unlike a
  `(json.RawMessage,error)` slice return). No-content ops need `//nolint:nilnil // ... success-with-
  no-content` (established repo pattern).
- **Const `+` concatenation is blocked by the string-concat hook even at compile time.** Write method
  constants as full string literals, not `PREFIX + "suffix"`.
- **Deleting a big chunk of `dispatch.go` moves every anchor below it.** Two hand-maintained digest
  anchors (`plugin-transport.md`, `aaa-auth.md`) went out of range; `make ze-doc-test` catches only
  out-of-range, so re-verify moved (still-in-range) anchors by hand. Also regenerate `ai/DOCS-TO-CODE.md`
  (`make ze-discovery-index`) after adding a new source file.
- **Pre-existing, unrelated:** `internal/component/plugin/all` fails (`TestYANGSchemaProviders`,
  wire-methods) because generated `all.go` is stale vs ospf/isis/ldp/rsvpte; `internal/plugins/ospf`
  has a FLAKY data race (`onVirtualLinksResolved` timer goroutine vs `TestVirtualLinkResolutionDrivesRuntime`,
  ~1/3 runs). Neither is touched by this refactor; scoped verification to changed packages.

## Files
- `internal/component/plugin/server/dispatch_registry.go` (new) -- `engineOp`, `engineOps`,
  `engineOpTable`/`lookupEngineOp`, `serveEngineOpJSON`/`serveEngineOpDirect`, `rpcErrMessage`, and the
  `op*` handlers (update-route, dispatch-command, dispatch-command-args, subscribe, unsubscribe,
  emit-event, inject-wire-route, batch-validate).
- `internal/component/plugin/server/dispatch_registry_test.go` (new) -- `TestPluginRPCRegistryCoversAllPaths`,
  `TestWireBridgeDispatchInstallsTypedSlots`, `TestEngineOpJSONAndDirectMatch`.
- `internal/component/plugin/server/dispatch.go` -- switches -> registry lookup; `wireBridgeDispatch`
  -> typedWire loop; 12 old handlers deleted; CORE methods untouched.
- `internal/component/plugin/server/dispatch_cached.go` -- RPC/Direct pairs -> `opForwardCached`/`opReleaseCached`.
- `internal/component/plugin/server/dispatch_route.go` -- RPC handlers -> `opRouteInstall`/`opRouteRemove` (+ Direct arm).
- `pkg/plugin/rpc/types.go` -- `rpc.Method*` constants + `InjectWireRouteInput`/`BatchValidateInput`.
- `pkg/plugin/sdk/sdk_engine.go` -- uniform typed-else-JSON shape; `rpc.Method*` constants; inject/batch JSON fallbacks.
- `docs/architecture/api/process-protocol.md`, `ai/digests/plugin-transport.md`, `ai/digests/aaa-auth.md`,
  `ai/DOCS-TO-CODE.md` -- doc + digest updates.
