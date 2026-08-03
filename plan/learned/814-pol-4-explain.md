# 814 -- pol-4-explain: Command-Line Policy Test and Trace

## Context

Ze had `show policy list` and `show policy chain` for policy introspection, but no way to test what a policy chain would actually do to a specific UPDATE. Operators had to configure a peer, send traffic, and inspect the result. Learned summary 572 explicitly deferred `show policy test` because it needed synthetic route construction and filter-chain dry-run execution.

## Decisions

- Used online daemon command (not offline tool) as the first version, because real filter plugins, config, peer context, and plugin readiness all matter for trustworthy results.
- Added `TracePolicyFilterChain` as a separate trace helper rather than modifying `PolicyFilterChain`, because runtime execution must stay untouched and the trace helper needs per-filter decision recording that the runtime path does not need.
- Used a narrow optional `PolicyDryRunner` interface (type-asserted by the handler) rather than widening `ReactorLifecycle`, because most command handlers do not need dry-run and the interface should not leak into unrelated consumers.
- Accepted full BGP UPDATE hex (with 19-byte header) as input rather than body-only hex, because operators can paste captured messages directly without stripping headers.
- Input validation (hex decode, minimum length, UPDATE type byte) happens before any reactor or plugin call, so bad input never reaches IPC.

## What Worked

- Reusing `AppendUpdateForFilter` for before/after text rendering and `applyFilterDelta` for modification kept the dry-run semantically identical to the runtime path without reimplementation.
- The `resolveFilterOverride` helper matches plain names against the canonical chain by stripping the plugin prefix, aligning with pol-3's plain-name-first UX.
- Structured `PolicyDryRunResult` with `DataMarker` embedding satisfies `ResponseData` and works through pipe operators without special handling.
- **AC-10 (AS4_PATH):** the flat filter text only carries a merged `as-path`; AS4_PATH (RFC 6793) is wire-only. `computeWireChanges` mirrors the runtime egress path (`textDeltaToModOps` + `ExtractRemovePrivateASOps` + `ExtractASPathPrependOps`) and reports the resulting `ModAccumulator` ops as `"<ATTRIBUTE> <verb>"` (e.g. `AS4_PATH suppressed`) in a new `wire-changes` field. This surfaces effects `changed-attrs` (text view) cannot. Unit-tested by `TestComputeWireChangesAS4Path`.

## What to Watch

- **Peer-selector commands must put `peer` in the YANG path.** The dispatcher's `extractPeerSelector` (`command.go`) strips only the selector *value* and leaves the `peer` keyword for the registered command path to absorb (like `show bgp peer <X> detail`). A flat command (`show policy test`) with `peer` arriving as a mid-command arg leaves an orphan `peer` token that fails `validateCommandArgs` against the first enum leaf ("invalid value peer, expected one of: import, export"). Fix: nest the leaves under a `container peer` so the path is `show policy test peer` and the grammar is `show policy test peer PEER export ...`. The same defect existed in `show policy chain` (pol-3) and was fixed identically. Both were latent because the commands had never been run end-to-end.
- **`update HEX`, not `update hex HEX`.** A two-word value (`update` keyword + `hex` sub-keyword + value) breaks `validateCommandArgs`: it consumes `update`+`hex` as keyword-value, orphaning the real hex, which then fails when `filter` is also present. Use a single value after `update`.
- **`.ci` observer scripts: use `runtime_fail()`, parse string `data`, dispatch `daemon shutdown`.** Three pitfalls that all manifest as a generic 20s timeout: (1) `print(FAIL); sys.exit(1)` is a silent no-op -- ze exits 0 on the next `daemon shutdown` and the runner reports success; use `ze_api.runtime_fail()` (emits the `ZE-OBSERVER-FAIL` sentinel + shuts down). (2) The dispatch result's `data` field is a JSON *string*, so `data.get(...)` raises and the observer crashes (mux conn closed) before shutdown -> timeout; guard with `if isinstance(data, str): data = json.loads(data)`. (3) Success path must `dispatch('daemon shutdown')` so the foreground `ze` exits 0. `expect=stdout:contains=` does NOT see observer stdout -- assert via `expect=exit:code=0` + `reject=stderr:pattern=ZE-OBSERVER-FAIL`.
- Functional `.ci` tests follow the **background `ze-peer` + foreground `ze --plugin <embedded> -`** pattern (see `policy-test-configured-export.ci`). Do NOT use `cmd=daemon` -- the runner only accepts `background`/`foreground` (`internal/test/runner/record_parse.go`). `smart-show.ci` is committed with `cmd=daemon` and silently breaks plugin-test discovery (still unfixed -- separate issue).
- `computeChangedAttrs` does line-level text comparison of filter text attributes. If `AppendUpdateForFilter` format changes, the changed-attr detection may need updating.
- ASN4 context defaults to true (matching `APIContextID`). Tests for ASN2 scenarios need explicit `source-asn4 false` (the AS4_PATH test relies on this).

## Follow-ups surfaced by /ze-review-deep

Real observations from the deep review that sit outside the pol-4 dry-run change:

- **SSH logs the full command (`ssh.go`) — FIXED separately.** `show policy test ... update <HEX>` carried a large hex blob into the SSH exec log. Addressed by `truncateForLog`/`maxLoggedCommandLen` in `ssh.go` (matching the existing `truncateProfiles` pattern), committed via `tmp/commit-ssh-logtrunc.sh` — not part of the pol commit (SSH-component hardening). The user confirmed non-backward-compatible changes are acceptable pre-release.
- **Dispatch `Data` double-encoding (system-wide) — GREEN-LIT, deferred to a dedicated change.** `responseToDispatchOutput` JSON-marshals `resp.Data` into `DispatchCommandOutput.Data string` (`pkg/plugin/rpc/types.go`), so every command's result reaches consumers as a JSON string inside JSON. The user OK'd a non-backward-compatible fix (pre-release), removing the only compat blocker. It is still NOT a pol-4 edit: it changes a core RPC type consumed by every surface (all `.ci`, SDK, web/REST/gRPC/MCP) and the error path (`output.Data = resp.Error`) needs redesign, so it must be validated by a full `make ze-verify` — currently blocked by the dirty tree. Do it as its own change on a clean tree (change `Data` to `json.RawMessage`, fix the error path, fix all consumers, full verify).
- **Redundant `parseFilterAttrs` (perf, cold + hot path).** `computeWireChanges` calls `textDeltaToModOps` + `ExtractRemovePrivateASOps` + `ExtractASPathPrependOps`, which each re-parse the modified filter text. Cold path here, but the same three-call sequence runs on the runtime egress hot path (`reactor_api_forward.go`). A shared "parse once, pass the map" refactor of those extractors would help both; out of scope for a CLI dry-run change.

## Files

| File | Role |
|------|------|
| `internal/component/bgp/plugins/cmd/policy/handler.go` | Handler: argument parsing, hex validation, reactor dispatch |
| `internal/component/cmd/show/show_policy_test_cmd_test.go` | Unit tests: parsing, bad hex rejection |
| `internal/component/bgp/reactor/policy_dryrun.go` | Reactor: TracePolicyFilterChain, PolicyDryRun method, helpers |
| `internal/component/bgp/reactor/policy_dryrun_test.go` | Unit tests: trace chain, filter override, changed attrs |
| `internal/component/plugin/types_bgp.go` | Types: PolicyDryRunner, PolicyDryRunResult, PolicyTraceEntry |
| `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` | YANG: `show policy test` command tree |
| `test/plugin/policy-test-*.ci` | Functional tests |
