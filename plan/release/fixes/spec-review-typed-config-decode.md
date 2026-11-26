# Spec: Typed config decode and BGP diff consumption (DESIGN-REVIEW finding 3)

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Config verification and application must consume the resolved transaction tree and preserve YANG schema value types, including `uint64`. |
| RFC gaps | - |
| Depends | `spec-unify-config-diff` (closed, learned 1079). Takes piece A (`uint64`) of the 2026-11-14 split of `plan/release/fixes/spec-yang-single-source-of-truth.md`; siblings `plan/release/fixes/spec-validation-walks-every-schema-section.md` and `plan/release/fixes/spec-yang-defaults-applied-once.md` |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-26 |

Recovery after compaction: `ai/rules/planning.md`, `ai/rules/config.md`, `ai/doors/system.md`, `ai/doors/config-yang.md`, `ai/doors/bgp-engine.md`, `ai/doors/api-surfaces.md`, and the source files in `## Current Behavior`.

## Task

Close the remaining config-pipeline defects from DESIGN-REVIEW finding 3 ("The stringly-typed middle") and piece A (`uint64`) of `spec-yang-single-source-of-truth.md`:

1. **YANG `uint64` type coarsening.** `yangTypeToValueType` (`internal/component/config/yang_schema.go`) still maps both `Yuint32` and `Yuint64` to `TypeUint32`, and `ValidateValue` (`internal/component/config/schema.go`) validates `TypeUint32` with a 32-bit unsigned bound. Every `type uint64` config leaf in shipped `-conf` modules — including BGP's `bgp > peer > session > aigp > link-metric` (`range "1..max"`, `internal/component/bgp/yang/ze-bgp-conf.yang`) and eight non-BGP `-conf` leaves (`ddos detect bps-floor`, `ddos flowspec`, `anomaly shape`, `fib p4`, `bgp-ls-export`, `ospf`) — is rejected above `4294967295` even though YANG declares `uint64` (`plan/journal/declared-format-contradicts-payload.md`, 2026-10-13).
2. **String round-trip and ad-hoc parsing on the BGP subtree.** `Tree` (`internal/component/config/tree.go`) stores all scalar leaves as strings and `ToMap` / `ToRunningMap` flatten them to `map[string]any` of strings. `PeersFromTree`, `parsePeerFromTree`, `parsePeerSettings`, `parseCaptureSettings`, `parseTTLSettings`, `parseBFDSettings` (`internal/component/bgp/reactor/config.go`), and `applyAIGPSettings` (`internal/component/bgp/reactor/config_aigp.go`) then hand-parse strings back into typed Go fields through local `mapUint32`, `mapUint16`, `mapUint8`, `mapBool`, `mapString`, and `strconv.ParseUint(fmt.Sprint(raw), 10, 64)` helpers instead of decoding once from the YANG-derived `config.Schema`.
3. **Remaining disk re-read on BGP `VerifyConfig`.** `ApplyConfigDiff` (`internal/component/bgp/reactor/reactor_api.go`) has already moved onto `loadPeersFromConfig` (`internal/component/bgp/reactor/reload_tree.go`), which consumes the handed whole-config tree via `TreeReloadFunc` without touching disk. However, `VerifyConfig` (`reactor_api.go`) still routes through `loadPeersFullOrTree`, which ignores its handed `tree` argument on a file-configured reactor and calls `reloadFunc(configPath)` to re-read and re-parse the file from disk, even though the reload coordinator (`internal/component/plugin/server/reload.go`) already hands `VerifyConfig` the exact `ToRunningMap()` tree that `diskConfigLoaders` parsed with the startup `cliPlugins`.

Goal:
1. Add `TypeUint64` to `config.ValueType`, map `gyang.Yuint64` to `TypeUint64`, validate `uint64` values and numeric ranges at full 64-bit width across all `-conf` modules (closing owned journal row `declared-format-contradicts-payload.md` 2026-10-13), and wire `TypeUint64` as a text input in the web config editor (`internal/component/web/`) so browser number inputs / JS floats never lose precision above `2^53`.
2. Add a read-side, schema-driven typed decode surface in `internal/component/config/tree.go` backed by `*config.Schema` (preserving string-backed `Tree` storage and byte-identical `ToMap`/`ToPluginMap`/`ToRunningMap` output) and migrate BGP peer config decoding (`config.go`, `config_aigp.go`) onto it.
3. Route `VerifyConfig` through `loadPeersFromConfig` when `treeReloadFunc` is installed (falling back to `configBGPBlock` + `GlobalsFromTree`/`PeersFromTree` only in standalone unit tests with no `treeReloadFunc`), eliminating the disk re-read on both production `VerifyConfig` and `ApplyConfigDiff`.

**Explicitly out of scope (owned elsewhere, referenced not duplicated):**
- Command/response envelope and client `json.Valid` guessing: `spec-unify-response-envelope.md` (closed or tracked separately).
- BGP hot-path filter text serialization ordering: `spec-unify-filters.md`.
- Redistribution text `update ...` command injection (`redistribute/consumer.go`): tracked separately.
- Generic reload map-diff deduplication: `spec-unify-config-diff` (closed, learned 1079).
- Non-BGP consumer migration to the schema-driven typed reader: deferred follow-on once this spec establishes the pattern on BGP.
- Standalone `Reload()` (`reactor_api.go`), which takes no tree argument and exists specifically to re-read the config file on standalone SIGHUP, and section-only callers (`PeerDiffCount`, `ReconcilePeersWithJournal`, `configop.ReconcilePeers`), which receive only the `bgp` section JSON without top-level `policy`/`bfd` roots.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - platform contracts, `ConfigProvider` authority, process boundary
  → Decision: config is YANG-modeled and changes flow as verify/apply/rollback transactions; `pkg/` and plugin JSON wire contracts do not change accidentally.
  → Constraint: keep plugin config delivery (`{"bgp":{...}}` JSON) and `Tree.ToPluginMap` string-valued and byte-identical.
- [ ] `ai/doors/config-yang.md` - config component boundaries, string-valued plugin delivery, coercion check
  → Decision: every delivered value arrives as a JSON string and coercion helpers must handle `case string:` (`./le config coercion check`).
  → Constraint: `internal/component/config` holds zero BGP imports; the typed reader lives in `config` as a generic schema-driven reader with no BGP-specific types.
- [ ] `ai/doors/bgp-engine.md` - BGP reactor boundaries and package tiers
  → Decision: the reactor never imports plugin packages directly; config loading stays behind `TreeReloadFunc` / `config.YANGSchema()`.
  → Constraint: preserve `peerSettingsEqual` equivalence so tree-loaded peers produce zero false session restarts.
- [ ] `docs/architecture/config/syntax.md` - Tree lowerings (`ToMap`, `ToPluginMap`, `ToRunningMap`) and `configvalue`
  → Decision: `ToMap`, `ToPluginMap`, and `ToRunningMap` emit string scalar leaves and keep their distinct sidecars (`@list`, `!inactive-members`).
  → Constraint: typed decode is strictly read-side and must not alter any lowering's emitted map shape.
- [ ] `docs/architecture/config/yang-config-design.md` - YANG validation layers, native type constraints, plugin delivery
  → Decision: `ValidateLeafValue` and `ValidateValue` enforce RFC 7950 Section 9 lexical and range bounds for every integer width.
  → Constraint: per `leaf_value.go:113-118` ("Value refusals"), all-digit overflow (`2^64`) on a **ranged** `uint64` leaf (`range "1..max"`) produces a `rangeRefusal` naming the declared range, whereas an **unranged** `uint64` leaf (or `ValidateValue` directly, `-1`, or non-digits) produces `"invalid uint64"`.
- [ ] `ai/doors/api-surfaces.md` - web UI boundaries and YANG schema-driven rendering
  → Decision: the web UI derives every field control from `config.ValueType` (`handler_config_leaf.go`, `fragment.go`) with no hardcoded field lists.
  → Constraint: `TypeUint64` must be wired in `leafInputType`, `buildFieldMeta`, `valueTypeToFieldType`, and `field_input_test.go` as a text input (like `TypeDecimal64`/`TypeASN`, avoiding browser `<input type="number">` / JS float precision loss above `2^53`).
- [ ] `docs/architecture/web-interface.md` and `docs/architecture/web-components.md` - web YANG schema integration and `fieldFor` dispatch
  → Decision: `TestFieldInputRegistryAnswersEachType` fails if `config.ValueType` adds a constant without wiring web dispatch.
  → Constraint: render `TypeUint64` as a text input (`data-type="uint64"` falling through `fieldInputs` to `inputText`), leaving `TypeUint16/32` as number inputs with min/max, and document `TypeUint64` in `web-interface.md` `## YANG Schema Integration`.
- [ ] `docs/architecture/config/apply-ordering.md` - BGP verify/apply reload loaders (`TreeReloadFunc` vs `ReloadFunc`)
  → Decision: `ApplyConfigDiff` already uses `TreeReloadFunc`; `VerifyConfig` currently kept `ReloadFunc` (re-reading the file).
  → Constraint: moving `VerifyConfig` onto `TreeReloadFunc` is a documented departure from `## The decisions` ("Verify applies no dynamic group") and updates that page in the same change.
- [ ] `docs/architecture/core-design.md` - Section 18 (Config Transaction Protocol) and Section 18a (BGP Peer Reload: Swap or Restart)
  → Decision: `peerSettingsEqual` compares whole `PeerSettings` structs, so tree-parsed and disk-parsed settings must match identically.
  → Constraint: no bypassed layers; both `VerifyConfig` and `ApplyConfigDiff` consume the transaction's handed tree.
- [ ] `ai/rules/config.md` - YANG typing, string coercion rule, `configvalue` helpers
  → Constraint: every coercion handles `case string:` and never asserts `v.(bool)` or `v.(float64)` directly.

**Key insights:**
- At HEAD, `ApplyConfigDiff` already consumes the handed `ToRunningMap()` tree via `loadPeersFromConfig` (`reload_tree.go`) -> `createTreeReloadFunc` (`loader_create.go`) -> `reloadPeersFromTree`; only `VerifyConfig` still takes the file-re-read route on the coordinator reload path.
- `ze-bgp-conf.yang` already declares a real BGP-consumed `uint64` leaf (`bgp > peer > session > aigp > link-metric`, `range "1..max"`) alongside eight non-BGP `-conf` `uint64` leaves, all currently blocked above `2^32-1` by `Yuint64 -> TypeUint32`.
- Five existing unit tests (`TestReloadVerifierRetainsSelectedPluginSchema`, `asn_notation_reload_test.go`, `rfc8212_import_relax_test.go`, `dissemination_verify_test.go`, `reactor_dynamic_swap_test.go`) pass an empty or dummy `map[string]any{}` to `VerifyConfig` relying on the old file re-read; they must be updated to pass the loaded candidate tree (and test `cliPlugins` schema retention on `createReloadFunc` / `diskConfigLoaders`) without lowering any assertion.

## Current Behavior (MANDATORY)

**Source files read:** (verified against HEAD on 2026-11-26)
- [ ] `internal/component/config/yang_schema.go` - `yangTypeToValueType` (line 1299) maps `gyang.Yuint32, gyang.Yuint64` together to `TypeUint32` (line 1329-1330), `gyang.Yenum` to `TypeString` (line 1333-1334); `getKeyTypeExtension` (line 579) recognizes `"string"`, `"prefix"`, `"ip"`, `"uint32"`, not `"uint64"`.
- [ ] `internal/component/config/schema.go` - `ValueType` (lines 35-57) has `TypeUint16` and `TypeUint32` with no `TypeUint64`; `ValidateValue` (lines 914-919) parses `TypeUint32` via `strconv.ParseUint(value, 10, 32)`, rejecting every `uint64` leaf value above `4294967295`.
- [ ] `internal/component/config/leaf_value.go` - `validateLeafRestrictions` (line 108) calls `ValidateValue` before range checks (converting all-digit overflow on a ranged leaf into `rangeRefusal`, line 116); `parseNumericRangeValue` (line 290) handles `TypeUint16`, `TypeUint32` (parsed with bitSize 64), `TypeASN`, `TypeInt`, and has no `TypeUint64` case.
- [ ] `internal/component/config/tree.go` - `Tree` stores `values map[string]string` and `multiValues map[string][]string` (lines 28-75); `ToMap`, `ToPluginMap`, and `ToRunningMap` (lines 1632-1737) lower leaves as strings; no schema-driven typed read accessor exists on `Tree` or over a schema-bound `map[string]any`.
- [ ] `internal/component/bgp/reactor/reactor_api.go` - `ApplyConfigDiff` (line 1022) already calls `a.loadPeersFromConfig(tree)` and never reads disk. `VerifyConfig` (line 1000) still calls `configBGPBlock(tree)` and `a.loadPeersFullOrTree(bgpTree)` (line 1071), which ignores `bgpTree` and calls `r.reloadFunc(configPath)` (re-reading disk via `storage.ReadReloadConfig` + `config.LoadConfig`) whenever `reloadFn != nil && configPath != ""`. `Reload` (line 955) is the standalone file-reload path and takes no tree.
- [ ] `internal/component/bgp/reactor/reload_tree.go` - `loadPeersFromConfig` (line 50) calls `r.treeReloadFunc(tree)` (installed by `CreateReactorFromTree` in `internal/component/bgp/config/loader_create.go:149` via `createTreeReloadFunc`, which runs `config.TreeFromPluginMap` -> `reloadPeersFromTree` -> `PruneInactive`, `ResolveBGPTree`, `GlobalsFromTree`, `refuseIncompletePeers`, `buildPeerPopulations`). Its doc comment (lines 33-36) notes that `VerifyConfig` kept the file route.
- [ ] `internal/component/bgp/reactor/config.go` - `PeersFromTree` (line 733), `parsePeerFromTree` (line 82), `parsePeerSettings` (line 155), `parseCaptureSettings` (line 531), `parseTTLSettings` (line 569), and `parseBFDSettings` (line 610) hand-parse scalar leaves out of `map[string]any` using local helpers `mapString`, `mapUint32`, `mapBool`, `mapUint8`, `mapUint16`, `mapMap`, `mapStringJoined`, `flexString` (lines 1024-1120).
- [ ] `internal/component/bgp/reactor/config_aigp.go` - `applyAIGPSettings` (line 13) reads `aigp.link-metric` (`type uint64 { range "1..max"; }` in `ze-bgp-conf.yang:810`) via `strconv.ParseUint(fmt.Sprint(raw), 10, 64)` and `enabled`/`originate` via `mapBool`.
- [ ] `internal/component/config/transaction/types.go` - `DiffSection` (lines 168-173) carries `Root`, `Added`, `Removed`, `Changed` strings; unchanged.
- [ ] `internal/core/configvalue/configvalue.go` - shared string-tolerant helpers `LeafList`, `ListEntries`, `Bool`, `Int`, `Section`.
- [ ] `internal/component/plugin/server/reload.go` - `reloadConfig` (line 437) already passes the whole `newTree` (`ToRunningMap()` produced by `diskConfigLoaders` in `cmd/ze/hub/main_reload.go:67`) to both `s.reactor.VerifyConfig(newTree)` and `s.reactor.ApplyConfigDiff(newTree)`.
- [ ] `internal/component/web/handler_config_leaf.go` - `leafInputType` (lines 66-122) switches on every `config.ValueType` to pick HTML input attributes (`TypeUint16`/`TypeUint32` as `number` with min/max, `TypeDecimal64`/`TypeASN` as `text`).
- [ ] `internal/component/web/fragment.go` - `buildFieldMeta` (lines 494-510) and `valueTypeToFieldType` (lines 526-550) switch on every `config.ValueType` to set HTML metadata and `data-type` strings.
- [ ] `internal/component/web/field_input_test.go` - `producedFieldTypes` (lines 44-72) walks `config.TypeString..config.TypeDecimal64` with a sentinel check that fails `TestFieldInputRegistryAnswersEachType` whenever `config.ValueType` gains a new constant past `TypeDecimal64` or produces a `FieldMeta.Type` absent from `fieldEditorMarkers`.

**Behavior to preserve:**
- Config file syntax and semantics unchanged; existing `.conf` and set-format files parse identically.
- Plugin config delivery format unchanged: `ToMap`, `ToPluginMap`, `ToRunningMap`, `rpc.ConfigSection` JSON (`{"bgp":{...}}` with string leaf values and `@list` sidecars), and the transaction verify/apply/rollback protocol are byte-identical.
- Parsed `PeerSettings` are equivalent to the full-pipeline result: zero new false diffs in `peerSettingsEqual`, no dropped fields (capabilities, static routes, families, dynamic groups, policy chains, BFD profiles, AIGP settings).
- `ze config diff` output and `rpc.ConfigDiffSection` JSON unchanged.
- Standalone `Reload()` (`reactor_api.go:955`) continues to load from `reloadFunc(configPath)` for standalone SIGHUP.
- Test-mode fallback (`treeReloadFunc == nil`) in `VerifyConfig` continues to parse the `bgp` block via `GlobalsFromTree` / `PeersFromTree` so bare-reactor unit tests keep working.
- `ze config validate` and `ze start` follow `leaf_value.go:113-118` ("Value refusals"): on a ranged `uint64` leaf, out-of-range values and all-digit `2^64` overflow produce `rangeRefusal` naming the declared range while `-1` / non-digits produce `"invalid uint64"`; on an unranged `uint64` leaf (or `ValidateValue` directly), `2^64`, `-1`, and non-digits produce `"invalid uint64"`.
- Web config editor keeps `TypeUint16` and `TypeUint32` as `<input type="number">` with min/max.

**Behavior to change:**
- `yangTypeToValueType`, `ValidateValue`, and `parseNumericRangeValue` gain `TypeUint64` so YANG `uint64` leaves accept `0..18446744073709551615` (subject to declared YANG `range`) instead of failing at `2^32-1`.
- Web config editor (`handler_config_leaf.go`, `fragment.go`, `field_input_test.go`) handles `TypeUint64` as a text input (`type="text"`, `data-type="uint64"`, no browser number-input precision loss above `2^53`; server validates exact 64-bit bounds).
- Production `VerifyConfig` consumes the handed whole-config `tree` through `loadPeersFromConfig` (`TreeReloadFunc`), matching `ApplyConfigDiff`, and never calls `reloadFunc(configPath)` to re-read disk.
- BGP peer config decoding (`config.go`, `config_aigp.go`) reads scalar leaves through the new schema-driven typed reader in `internal/component/config/tree.go` backed by `config.YANGSchema()`, replacing ad-hoc `mapUint32`/`mapUint16`/`mapUint8`/`mapBool` string hand-parsers.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Tree reload pipeline (`TreeReloadFunc`) | `internal/component/bgp/reactor/reload_tree.go` `loadPeersFromConfig`; `internal/component/bgp/config/loader_create.go` `createTreeReloadFunc`, `reloadPeersFromTree` | Already landed for `ApplyConfigDiff`, `RebuildRunningPeers`, and `createdPeerFromRunning`: reconstructs a `*config.Tree` from the handed `ToRunningMap()` via `TreeFromPluginMap` and runs `PruneInactive`, `ResolveBGPTree`, `GlobalsFromTree`, `refuseIncompletePeers`, and `buildPeerPopulations` with zero disk I/O | Extends: routes `VerifyConfig` through the same `loadPeersFromConfig` pipeline (AC-1, AC-2) instead of calling `loadPeersFullOrTree` |
| File reload seam (`loadPeersFullOrTree`) | `internal/component/bgp/reactor/reactor_api.go` `loadPeersFullOrTree` | Calls `reloadFunc(configPath)` (re-reading disk) when set, else parses `bgpTree` via `GlobalsFromTree`/`PeersFromTree`; still called by `VerifyConfig`, `peerDiffCount`, `ReconcilePeersWithJournal`, and `configop.ReconcilePeers` | Replaces on `VerifyConfig` with `loadPeersFromConfig` (with `treeReloadFunc == nil` fallback); keeps `Reload()` and section-only callers untouched because those do not receive the whole-config tree |
| Value-type derivation | `internal/component/config/yang_schema.go` `yangTypeToValueType`, `getKeyTypeExtension` | Maps both `Yuint32` and `Yuint64` to `TypeUint32` | Extends: maps `Yuint64` to `TypeUint64` (and `"uint64"` key type) |
| Value validation and numeric ranges | `internal/component/config/schema.go` `ValueType`, `ValidateValue`; `internal/component/config/leaf_value.go` `parseNumericRangeValue`, `validateLeafRestrictions` | `ValidateValue` parses `TypeUint32` with 32-bit bound so `uint64` leaves above `2^32-1` are refused; `parseNumericRangeValue` has no `TypeUint64` arm | Extends: adds `TypeUint64` with 64-bit unsigned parse and range validation (AC-4, AC-6) |
| Ranged `decimal64` validation | `internal/component/config/leaf_value.go` `validateDecimal64Leaf` (`833c08af3f`) | Compares ranged `decimal64` by fraction digits | Does not apply: already fixed |
| Leaf-list native range/length/union/decimal | `internal/component/config/leaf_value.go` `memberLeaf` (`8c83416bfc`) | Carries enums, ranges, lengths, patterns, union, decimal64 onto leaf-list member checks | Extends automatically: `memberLeaf` carries `TypeUint64` once `ValueType` and `ValidateValue` support it |
| Delivered config coercion helpers | `internal/core/configvalue/configvalue.go` `Bool`, `Int`, `LeafList`, `ListEntries`, `Section` | String-tolerant coercion for delivered `map[string]any` values (complies with `./le config coercion check`) | Extends / reuses: the schema-driven typed reader in `config/tree.go` reuses `configvalue.LeafList` / string-tolerant decoding and adds schema-checked typed leaf accessors (`Uint64`, `Uint32`, `Uint16`, `Uint8`, `Bool`, `Addr`, `String`, `Enum`) |
| Tree lowerings (`ToMap`, `ToPluginMap`, `ToRunningMap`) | `internal/component/config/tree.go` `toMap`; `internal/component/config/plugin_map.go` `TreeFromPluginMap` | Lowers `Tree` to string-valued `map[string]any` (with `@list` order on `ToPluginMap` and `!inactive-members` on `ToRunningMap`) and restores `*Tree` from it | Does not apply: AC-5 preserves all three lowerings byte-identically |
| Transaction diff representation | `internal/component/config/transaction/types.go` `DiffSection` | Carries string `Added`, `Removed`, `Changed` per config root | Does not apply: AC-5 preserves transaction diff format |
| Web schema-driven field input dispatch | `internal/component/web/handler_config_leaf.go` `leafInputType`; `internal/component/web/fragment.go` `buildFieldMeta`, `valueTypeToFieldType`; `internal/component/web/field_input_test.go` `producedFieldTypes`, `fieldEditorMarkers` | Switches on every `config.ValueType` (`TypeUint16/32` as `number`, `TypeDecimal64`/`TypeASN` as `text`) and fails `TestFieldInputRegistryAnswersEachType` if a new `ValueType` is unhandled | Extends: adds `config.TypeUint64` as a text input (`type="text"`, `data-type="uint64"`, avoiding HTML number-input / JS float precision loss above `2^53`) and updates `docs/architecture/web-interface.md` `## YANG Schema Integration` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "A config change is a transaction of stream events (verify, apply, rollback) that each plugin answers for its own roots" | agrees | Preserves the stream-event transaction protocol and `ConfigProvider` authority while fixing `VerifyConfig` to consume the transaction's handed tree and preserving `uint64` schema typing. |
| `ai/doors/bgp-engine.md` | Boundaries | the door covering `internal/component/bgp/reactor/reactor_api.go`, `reload_tree.go`, `config.go`, `config_aigp.go` | "Never name a plugin package from the reactor." | agrees | Reactor stays decoupled from plugin packages; it uses `TreeReloadFunc` injected by `bgp/config` and the generic schema reader in `internal/component/config`. |
| `ai/doors/config-yang.md` | Traps | the door covering `internal/component/config/yang_schema.go`, `schema.go`, `leaf_value.go`, `tree.go` | "**Values arrive as strings.** A direct `v.(bool)` or `v.(float64)` assertion silently takes the default." | agrees | Keeps `ToPluginMap` and `ToMap` string-valued, decodes string representations inside the schema-driven typed reader, and stays green under `./le config coercion check`. |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `test/plugin/config-apply-consumes-diff.ci`, `test/plugin/config-uint64-no-truncation.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | Places both new end-to-end functional proofs under `test/plugin/` using the standard `.ci` runner format. |
| `ai/doors/api-surfaces.md` | Boundaries | the door covering `internal/component/web/handler_config_leaf.go`, `fragment.go`, `field_input_test.go` | "**Never copy a fact the YANG model or the registry already holds**" | agrees | Derives `TypeUint64` web field rendering directly from `config.ValueType` in `leafInputType`, `buildFieldMeta`, and `valueTypeToFieldType`, with zero hardcoded leaf lists. |
| `docs/architecture/web-interface.md` | YANG Schema Integration | modified page; the `// Design:` page of `handler_config_leaf.go`; anchored to `schema.go`, `leaf_value.go`, `fragment.go` | "The YANG schema drives the entire UI. No hardcoded field lists." | agrees | Keeps `TypeUint16/32` as number inputs with min/max and adds a `TypeUint64` text-input row (matching `TypeDecimal64`/`TypeASN` to prevent browser `<input type="number">` / JS float precision loss above `2^53`; server validates exact 64-bit bounds), updating the table in the same change. |
| `docs/architecture/web-components.md` | Template Dispatch (fieldFor) | the `// Design:` page of `internal/component/web/fragment.go` | "3. Add case to valueTypeToFieldType() in fragment.go" | agrees | Adds `case config.TypeUint64: return "uint64"` in `valueTypeToFieldType`, lets `fieldInputFor` route it to `inputText`, and adds `"uint64"` to `fieldEditorMarkers` and `producedFieldTypes` in `field_input_test.go`. |
| `docs/architecture/config/apply-ordering.md` | The decisions | modified page; anchored to `internal/component/bgp/reactor/reload_tree.go` | "Only a file-configured reactor also holds the ReloadFunc, which re-reads the file; verify keeps the file." | departs | `VerifyConfig` stops re-reading the file and consumes the handed whole-config tree via `TreeReloadFunc`, matching `ApplyConfigDiff`. `apply-ordering.md` is listed in `Files to Modify` and updated in the same change so the doc reflects that `ReloadFunc` remains only on standalone `Reload()` and section-only callers. |
| `docs/architecture/core-design.md` | 18a. BGP Peer Reload: Swap or Restart | the `// Design:` page of `reactor_api.go`, `config.go`; anchored to `yang_schema.go`, `tree.go`, `reload_tree.go` | "`peerSettingsEqual` (`reactor_api.go`) compares the whole `PeerSettings` struct, so a field nobody classified still counts as a change." | agrees | AC-2 explicitly requires tree-loaded and schema-decoded `PeerSettings` to compare equal under `peerSettingsEqual` with zero false diffs or spurious session restarts. |
| `docs/architecture/config/syntax.md` | What a reader receives | the `// Design:` page of `yang_schema.go`, `schema.go`, `tree.go`; anchored to `config.go`, `leaf_value.go` | "A plain leaf is a STRING at both ends, whatever its YANG type says." | agrees | Preserves string-valued `Tree` storage and `ToMap`/`ToPluginMap`/`ToRunningMap` lowering; typed decode is strictly a read-side schema-driven view over those strings. |
| `docs/architecture/config/yang-config-design.md` | Every delivered value is a JSON string | the `// Design:` page of `leaf_value.go`; anchored to `yang_schema.go`, `schema.go`, `tree.go` | "The plugin config framework hands every YANG leaf value to a plugin's `ParseConfig` as a JSON string" | agrees | Keeps JSON string delivery to plugins unchanged (AC-5) while restoring 64-bit lexical and range validation for `uint64` leaves in `ValidateValue` / `ValidateLeafValue` (Section 4). |
| `docs/architecture/config/transaction-protocol.md` | 4. Stream Events | the `// Design:` page of `reload_tree.go`; anchored to `reactor_api.go` | "Plugins that reuse a tree-based parser reconstruct their delivered section with `config.TreeFromPluginMap`." | agrees | `VerifyConfig` now shares `loadPeersFromConfig` -> `createTreeReloadFunc` -> `config.TreeFromPluginMap` -> `reloadPeersFromTree` with `ApplyConfigDiff`, leaving transaction events untouched. |
| `docs/architecture/api/process-protocol.md` | Config Reload Protocol | anchored to `reactor_api.go`, `reload_tree.go`, `tree.go` | "Config reload uses a two-phase verify/apply pattern:" | agrees | Preserves the two-phase `config-verify` / `config-apply` wire protocol and `ConfigDiffSection` payloads unchanged (AC-5). |
| `docs/architecture/hub-architecture.md` | 3. Hub Validates Against Combined Schema | anchored to `yang_schema.go` | "The Hub validates the config file against the schema of every compiled-in module (see section 2 for external plugins)." | agrees | `diskConfigLoaders` already validates the candidate against the combined schema (with startup `cliPlugins`) before handing `newTree` to `VerifyConfig`, and `TypeUint64` makes `uint64` validation accurate across all compiled-in modules. |
| `docs/architecture/route-selection.md` | AIGP session policy | the `// Design:` page of `internal/component/bgp/reactor/config_aigp.go` | "Only an explicitly written `aigp` configuration activates the declared bus requirement (`bgp-rib` receiving `bgp:update` and `bgp:state`);" | agrees | Migrating `applyAIGPSettings` to the schema-driven typed reader preserves `AIGPConfigured` activation semantics and enables full 64-bit `link-metric` values declared by YANG. |
| `docs/architecture/api/architecture.md` | Message Bus and Participant Wiring | anchored to `reactor_api.go` | - | silent | Anchored to `reactor_api.go` only for `getMatchingPeersSel` and `SendRawMessage`; says nothing about BGP config reload tree loading, `TypeUint64`, or schema-driven leaf decoding. |
| `docs/architecture/api/commands.md` | Overview | anchored to `reactor_api.go`, `config.go` | - | silent | Anchored to `reactor_api.go` and `config.go` for `show policy-chain`, `Peers`, `PeerFSMHistory`, `ReloadRunning`, and `DrainPeerSync`; says nothing about `VerifyConfig` tree loading or YANG `uint64` leaf decoding. |
| `docs/architecture/api/text-format.md` | Current Format | anchored to `reactor_api.go` | - | silent | Anchored to `reactor_api.go` only for `OnPeerEstablished`/`OnPeerClosed` text event formatting; says nothing about config parsing or reload verification. |
| `docs/architecture/behavior/fsm.md` | Ze Implementation Notes | anchored to `reactor_api.go` | - | silent | Anchored to `reactor_api.go` only for `apiStateObserver`; says nothing about config loading, `VerifyConfig`, or schema value types. |
| `docs/architecture/behavior/peer-lifecycle.md` | TL;DR | anchored to `reactor_api.go` | - | silent | Anchored to `reactor_api.go` for `reconcilePeersJournaled`, `establishedPeerInfo`, and `heldInbound`; says nothing about how `VerifyConfig` obtains candidate peers or how YANG scalar leaves are decoded. |
| `docs/architecture/wire/nlri-bgpls.md` | Ze Implementation Notes | anchored to `reactor_api.go` | - | silent | Anchored to `reactor_api.go` only for `establishedPeerInfo` and `connectedLocalEndpoint`; says nothing about config validation or peer config decoding. |
| `docs/architecture/meta/role.md` | Config keying | anchored to `config.go` | - | silent | Anchored to `config.go` only for remote-IP validation on peer keying; says nothing about schema-driven scalar decode, `uint64` width, or `VerifyConfig`. |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `config.go` | - | silent | Anchored to `config.go` only for `IgnoreFamilyMismatch`; says nothing about schema-driven scalar decode, `uint64` width, or reload tree consumption. |
| `docs/architecture/wire/capabilities.md` | 9b. BFD Strict-Mode (Code 74) | anchored to `config.go` | - | silent | Anchored to `config.go` only for emitting capability 74 from the `bfd` block; says nothing about schema-driven scalar decode or reload tree loading. |
| `docs/architecture/config/change-file-structural-ops.md` | The shape | anchored to `tree.go` | - | silent | Anchored to `tree.go` only for `RepositionListEntries` during session change-file replay; says nothing about read-side typed decode, `TypeUint64`, or BGP reload. |
| `docs/architecture/aaa-tacacs.md` | The schema merge defect this work uncovered | anchored to `schema.go` | - | silent | Anchored to `schema.go` only for `Define`/`mergeContainer`/`mergeNode` container merging; says nothing about `ValueType` `TypeUint64`, typed decode, or BGP reload. |
| `docs/architecture/ssh/fixit-bcrypt-hash-credential.md` | Decisions | anchored to `schema.go` | - | silent | Anchored to `schema.go` only for `Schema.LookupTokenPath`; says nothing about `TypeUint64`, typed decode, or BGP reload. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not touch data-plane backend compatibility checks or `fib`/`sysctl` validation; commit-time VPP/kernel mismatch refusal stays intact. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not alter the data-plane mismatch verdict or its enforcement across commit, reload, and `ze config validate`. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Touches no firewall verifier or `ze:backend` annotations; VPP firewall verification rules stay unchanged. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Introduces no data-plane selector or `ze:backend` value and preserves the `kernel`/`vpp` plane vocabulary untouched. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Adds no session-level next-hop rewrite knob and preserves export-chain next-hop compilation through `reloadPeersFromTree`. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not change `policy { modify }` syntax, `policychain.Sets`, or built-in `next-hop:*` chain references. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | `VerifyConfig` uses `reloadPeersFromTree` -> `buildPeerPopulations`, keeping the exact config-compile-time `next-hop:auto` chain compilation and `inactive:next-hop:auto` refusal intact. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch export-chain execution, folding, or `wireOverride` provenance checks. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not alter `policy { modify }` next-hop IPv4/IPv6 support or import-chain refusal at config load. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not modify egress forwarding or static/ExaBGP/API route origination next-hop behavior. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch implied-filter publication or `\| implied` rendering. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Does not add any service-level refusal at daemon start; `VerifyConfig` continues to refuse invalid candidate configs on reload as a whole while running services keep serving. |
| `docs/architecture/core-design.md` | Design decisions | `core-design.md` decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Does not alter kernel/VPP mismatch handling at start, commit, reload, or `ze config validate`. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` serialization or comment placement. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `parsePeerBusFromTree` keeps canonical peer/group bus paths and registered participant defaults unchanged when reading through the typed reader. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not alter bus `send > to` parsing or replacement semantics. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not alter bus `receive > from` parsing or empty-list semantics. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not alter bus allow-list parsing or enforcement. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | `VerifyConfig` continues to run `resolveDisseminationSettings` over the tree-loaded peers and dynamic groups, preserving bus filter compilation and conflict refusals. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Adds no bus arbitration or exclusive-role rules. |
| `docs/architecture/api/architecture.md` | Design decisions | `api/architecture.md` decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not touch bus or implied display commands. |
| `docs/architecture/api/commands.md` | Design decisions | `api/commands.md` decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no CLI command paths or selector subcommands; pipe semantics stay untouched. |
| `docs/architecture/api/commands.md` | Design decisions | `api/commands.md` decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not change any pipe operator or command answer shape. |
| `docs/architecture/api/commands.md` | Design decisions | `api/commands.md` decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not change any pipe operator behavior. |
| `docs/architecture/api/process-protocol.md` | Design decisions | `api/process-protocol.md` decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no engine RPC or message operation and leaves plugin authorization profiles untouched. |
| `docs/architecture/api/process-protocol.md` | Design decisions | `api/process-protocol.md` decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | Does not touch event delivery queues or overflow policy. |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A config load or reload arrives via `config.LoadConfig` (for `ze start`, `ze config validate`, and `diskConfigLoaders` on SIGHUP / CLI commit / web commit).
- On reload, `diskConfigLoaders` (`cmd/ze/hub/main_reload.go`) parses the candidate once into `*config.Tree`, lowers it via `Tree.ToRunningMap()`, and passes that whole-config `map[string]any` to `pluginserver.Server.ReloadConfigWithSelection` -> `s.reactor.VerifyConfig(newTree)` and `s.reactor.ApplyConfigDiff(newTree)`.

### Transformation Path
1. YANG modules load into `*config.Schema` (`yang_schema.go`, `schema.go`). `yangTypeToValueType` maps `gyang.Yuint64` to `TypeUint64` (no longer `TypeUint32`).
2. Parsers and `set` commands validate each scalar and leaf-list member through `ValidateLeafValue` -> `ValidateValue` and `parseNumericRangeValue` (`schema.go`, `leaf_value.go`), accepting `uint64` up to `18446744073709551615` within declared YANG `range`. Per `leaf_value.go:113-118`, a ranged `uint64` leaf refuses out-of-range values and all-digit `>= 18446744073709551616` overflow with `rangeRefusal` naming the declared range (while `-1` and non-digits return `"invalid uint64"`), and an unranged `uint64` leaf (or `ValidateValue` directly) refuses `>= 18446744073709551616`, `-1`, and non-digits with `"invalid uint64"`.
3. Parsed config populates `*config.Tree` (string-backed) and lowers to `ToMap`, `ToPluginMap`, or `ToRunningMap` (`tree.go`) with string leaf values preserved. In the web UI, `leafInputType` (`handler_config_leaf.go`), `buildFieldMeta`, and `valueTypeToFieldType` (`fragment.go`) render `TypeUint64` leaves as text inputs (`type="text"`, `data-type="uint64"`) so browser number inputs cannot round values above `2^53`.
4. On reload, both `VerifyConfig(tree)` and `ApplyConfigDiff(tree)` (`reactor_api.go`, `reload_tree.go`) call `loadPeersFromConfig(tree)` -> `createTreeReloadFunc` -> `config.TreeFromPluginMap` -> `reloadPeersFromTree` (`PruneInactive`, `ResolveBGPTree`, `GlobalsFromTree`, `refuseIncompletePeers`, `buildPeerPopulations`), consuming the handed tree directly with zero disk re-read.
5. `PeersFromTree`, `parsePeerFromTree`, `parsePeerSettings`, `parseCaptureSettings`, `parseTTLSettings`, `parseBFDSettings` (`config.go`), and `applyAIGPSettings` (`config_aigp.go`) decode scalar leaves through the schema-driven typed reader in `config/tree.go` backed by `config.YANGSchema()`, producing typed `PeerSettings` (including 64-bit `AIGPLinkMetric`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG schema ↔ Go `config.Schema` | `yangTypeToValueType` maps `Yuint64 -> TypeUint64`; `ValidateValue` and `parseNumericRangeValue` enforce 64-bit unsigned width | [ ] |
| `config.Tree` / `map[string]any` ↔ BGP reactor config parser | Read-side schema-driven typed reader in `internal/component/config/tree.go` resolves `*LeafNode` types and decodes string leaves into typed Go values | [ ] |
| Reload coordinator ↔ BGP reactor (`VerifyConfig`, `ApplyConfigDiff`) | Both pass the whole-config `ToRunningMap()` through `loadPeersFromConfig` (`TreeReloadFunc`); zero disk re-read | [ ] |
| Engine ↔ plugins (`rpc.ConfigSection`, `DiffSection`) | `ToPluginMap` JSON (`{"bgp":{...}}`, string values, `@list` order) and `transaction.DiffSection` unchanged | [ ] |

### Changed-Wire Census

**Search evidence:** Inspected `internal/component/config/tree.go` (`ToMap`, `ToPluginMap`, `ToRunningMap`), `internal/component/config/transaction/types.go` (`DiffSection`), `internal/component/bgp/reactor/reactor_api.go` (`VerifyConfig`, `ApplyConfigDiff`), `internal/component/bgp/reactor/config.go` (`PeersFromTree`), `internal/component/bgp/reactor/config_aigp.go` (`applyAIGPSettings`), `internal/component/web/handler_config_leaf.go` (`leafInputType`), `internal/component/web/fragment.go` (`buildFieldMeta`, `valueTypeToFieldType`), and `docs/architecture/api/process-protocol.md` (`Config Reload Protocol`).

No wire format changes in this spec. Every external and cross-process contract — BGP wire messages (OPEN/UPDATE/NOTIFICATION), plugin config delivery (`rpc.ConfigSection` JSON via `Tree.ToPluginMap` with string-valued leaves and `@list` sidecars), transaction stream events (`transaction.DiffSection` and `rpc.ConfigDiffSection`), web `/config/set/` form fields (string leaf values submitted identically from HTML `<input type="text">`), and `ze config diff` output — stays byte-identical (verified by AC-5). `TypeUint64` repairs schema validation of an existing YANG type (`uint64`) in-process; the typed reader in `internal/component/config/tree.go` is strictly read-side over the unchanged string-backed `Tree` and `map[string]any`; and routing `VerifyConfig` through `loadPeersFromConfig` changes only an internal in-process loader call inside `internal/component/bgp/reactor`.

### Integration Points
- `config.YANGSchema()` (`internal/component/config/yang_schema.go`) - schema authority passed into the read-side typed reader.
- `ValidateValue` / `ValidateLeafValue` / `parseNumericRangeValue` (`schema.go`, `leaf_value.go`) - 64-bit integer lexical and range checks for every `-conf` module.
- `leafInputType` / `buildFieldMeta` / `valueTypeToFieldType` (`internal/component/web/handler_config_leaf.go`, `fragment.go`) - renders `TypeUint64` as a text input (`type="text"`, `data-type="uint64"`) and keeps `TestFieldInputRegistryAnswersEachType` (`field_input_test.go`) green.
- `loadPeersFromConfig` (`internal/component/bgp/reactor/reload_tree.go`) - shared tree-reload seam called by both `VerifyConfig` and `ApplyConfigDiff`.
- `PeersFromTree` / `parsePeerSettings` / `applyAIGPSettings` (`config.go`, `config_aigp.go`) - BGP consumer of the schema-driven typed reader.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Verify both `VerifyConfig` and `ApplyConfigDiff` consume the handed tree via `loadPeersFromConfig` with zero `reloadFunc(configPath)` call on production reload |
| No unintended coupling (components stay isolated) | No | Verify `internal/component/config` gains zero BGP imports (`./le arch tier check`) and the typed reader takes `*config.Schema` / `ChildProvider` generically |
| No duplicated functionality (extends existing, does not recreate) | No | Verify `VerifyConfig` reuses existing `loadPeersFromConfig` (`TreeReloadFunc`), `TypeUint64` extends `ValueType`/`ValidateValue`/`parseNumericRangeValue`, web reuses `inputText` via `fieldInputFor`, and the typed reader reuses `configvalue.LeafList` |
| Zero-copy preserved where applicable (refs, not copies) | No | Verify typed decode reads existing string-backed maps/`Tree` in place without re-stringifying or cloning subtrees |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | Verify typed decode dispatches strictly on schema `ValueType` (`LeafNode.Type`), with zero per-leaf or BGP-specific switch cases in `internal/component/config` or `internal/component/web` |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | Grep across the repo for `TypeUint32`/`TypeDecimal64` shows every `config.ValueType` switch lives in `internal/component/config` (`schema.go`, `leaf_value.go`, `yang_schema.go`) and `internal/component/web` (`handler_config_leaf.go`, `fragment.go`, `field_input_test.go`); all six are updated to handle `TypeUint64`, and no CLI/command/plugin registry list needs a manual entry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The `ToRunningMap()` tree handed by `pluginserver.Server.reloadConfig` to `VerifyConfig` carries every root (`bgp`, `policy`, `bfd`, `bus`) and inactive leaf-list intent needed by `reloadPeersFromTree`, producing `PeerSettings` equal to `createReloadFunc` | `cmd/ze/hub/main_reload.go:109` (`result.Tree.ToRunningMap()`), `internal/component/bgp/config/loader_create.go:313` (`createTreeReloadFunc` already powers `ApplyConfigDiff`) | `VerifyConfig` would accept/reject differently from `ApplyConfigDiff` or produce false diffs | Unit comparison of `PeerSettings` from `createReloadFunc` vs `createTreeReloadFunc` over a full fixture config (groups, defaults, static routes, policy, BFD, dynamic groups) | unvalidated |
| A-2 | `bgp > peer > session > aigp > link-metric` (`ze-bgp-conf.yang:810`, `type uint64 { range "1..max"; }`) and 8 non-BGP `-conf` `uint64` leaves are currently rejected above `4294967295`, and no consumer relies on silent 32-bit rejection | Verified at HEAD: `yang_schema.go:1329`, `schema.go:915`, `config_aigp.go:33` (`ps.AIGPLinkMetric uint64`), journal row 2026-10-13 | A consumer expects a 32-bit cap not declared in YANG | Boundary tests at `4294967295`, `4294967296`, `18446744073709551615`, `18446744073709551616` plus existing config/BGP test suites | unvalidated |
| A-3 | Plugin config delivery JSON (`ToPluginMap`) and `ze config diff` stay string-valued and byte-identical while BGP internal decode uses the schema-driven typed reader | `docs/architecture/config/syntax.md` ("What a reader receives"), `docs/architecture/config/yang-config-design.md` Section 14, `ai/rules/config.md` | Wire contract break for plugins or `./le config coercion check` failure | Byte-compare emitted plugin JSON and `ze config diff` output before/after; run `./le config coercion check` | unvalidated |
| A-4 | The five existing unit tests that pass an empty/dummy `map[string]any{}` to `VerifyConfig` (`TestReloadVerifierRetainsSelectedPluginSchema`, `asn_notation_reload_test.go`, `rfc8212_import_relax_test.go`, `dissemination_verify_test.go`, `reactor_dynamic_swap_test.go`) relied only on `VerifyConfig`'s old file re-read and retain full coverage when updated to pass the candidate tree (and test `cliPlugins` retention on `createReloadFunc` / `diskConfigLoaders`) | Verified via grep of all `VerifyConfig(` call sites under `internal/component/bgp/` | A production caller invokes `VerifyConfig(map[string]any{})` expecting a disk read | Grep every production `VerifyConfig` caller (`internal/component/plugin/server/reload.go:437` is the sole production caller and passes `newTree`) and have `ze-test-judge` review any updated test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Moving `VerifyConfig` off `reloadFunc` breaks unit tests that passed `map[string]any{}` expecting a candidate file read from `store` | Failures in `TestReloadVerifierRetainsSelectedPluginSchema`, `asn_notation_reload_test.go`, `rfc8212_import_relax_test.go`, `dissemination_verify_test.go`, or `reactor_dynamic_swap_test.go` | Update those tests to pass the loaded candidate tree to `VerifyConfig` and keep `cliPlugins` retention tested on `createReloadFunc` / `Reload()` / `diskConfigLoaders`, with `ze-test-judge` approval |
| R-2 | Exhaustive `ValueType` switches or web field-type tests miss the new `TypeUint64` enum member | Compiler / exhaustive-switch lint, runtime `unknown type`, or `TestFieldInputRegistryAnswersEachType` (`internal/component/web/field_input_test.go`) failure | Wire `TypeUint64` across every `ValueType` switch in both `internal/component/config/` (`schema.go`, `leaf_value.go`, `yang_schema.go`) and `internal/component/web/` (`handler_config_leaf.go`, `fragment.go`, `field_input_test.go`, rendering as text input like `TypeDecimal64`/`TypeASN` to avoid JS float `> 2^53` precision loss), and run scoped `config` + `web` tests |
| R-3 | Schema-driven typed decode rejects or mishandles synthetic `map[string]any` fixtures in existing BGP reactor unit tests (e.g. partial trees or bool/int values injected directly in Go tests) | Existing `internal/component/bgp/reactor/*_test.go` failures | Keep string-tolerant coercion (`case string:` plus Go native fallback only where `configvalue` allows, passing `./le config coercion check`) and preserve `ErrIncompleteConfig` semantics |
| R-4 | Scope creeps into non-BGP consumers or `ReconcilePeersWithJournal` section-only paths | Diff spreads across unrelated plugin packages | Limit typed decode migration to `internal/component/bgp/reactor/config.go` and `config_aigp.go`, and limit tree-reload wiring to `VerifyConfig` (beside already-migrated `ApplyConfigDiff`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Reload verification could falsely reject valid BGP configs or produce false `PeerSettings` diffs causing unnecessary BGP session resets; broken `TypeUint64` validation could reject valid integer config leaves or accept overflowing integers |
| How is it reverted? | Single commit revert; no persisted config format, state store key, or wire protocol changes |
| Who else touches this path? | `internal/component/config` (shared by all config consumers, CLI editor, gNMI, web UI) and `internal/component/bgp/reactor` (reload coordinator, BGP plugin); coordinate with sibling `spec-validation-walks-every-schema-section.md` and `spec-yang-defaults-applied-once.md` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config commit / reload changes a peer's ASN in memory (with disk-reread seam instrumented) | → | `VerifyConfig` and `ApplyConfigDiff` both parse from the handed resolved tree via `loadPeersFromConfig`, never invoking `reloadFunc(configPath)` | `test/plugin/config-apply-consumes-diff.ci` |
| config sets `bgp > peer > session > aigp > link-metric` (and a non-BGP `uint64` leaf such as `ddos detect bps-floor`) to a value `> 2^32-1` | → | `yangTypeToValueType` (`TypeUint64`) -> `ValidateValue` / `parseNumericRangeValue` -> schema-driven typed decode -> `PeerSettings.AIGPLinkMetric` | `test/plugin/config-uint64-no-truncation.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Production config reload on a file-configured reactor (`reloadFunc` and `treeReloadFunc` both installed) | `VerifyConfig` and `ApplyConfigDiff` both load peers, globals, and dynamic groups from the handed whole-config tree via `loadPeersFromConfig`, reconciling the peer change with the `reloadFunc` disk-reread counter remaining `0` | Standalone `Reload()` (`reactor_api.go`) keeps `reloadFunc(configPath)` because standalone SIGHUP receives no tree; section-only callers (`PeerDiffCount`, `ReconcilePeersWithJournal`, `configop.ReconcilePeers`) keep `loadPeersFullOrTree` because their input carries only the `bgp` section without top-level `policy`/`bfd` roots |
| AC-2 | Same BGP configuration (globals, peer groups, templates, YANG defaults, static routes, policy chains, BFD profiles, AIGP, dynamic groups) loaded via `createTreeReloadFunc` vs `createReloadFunc` | Parsed `PeerSettings`, `Globals`, and `DynamicGroupConfig` compare equal under `peerSettingsEqual` and produce zero peer swaps or restarts on a no-op reload | None: every field in `PeerSettings` participates in `peerSettingsEqual` (Section 18a of `core-design.md`), so any divergence would cause a false session restart |
| AC-3 | BGP peer config leaves (remote/local/migration ASN, router-id, timers, local/remote ports, capture settings, TTL, BFD settings, AIGP settings, booleans, descriptions, families) in `PeersFromTree` / `parsePeerSettings` / `applyAIGPSettings` | Values are decoded into typed Go fields through the schema-driven typed reader in `internal/component/config/tree.go` backed by `config.YANGSchema()`, returning identical `PeerSettings` and explicit errors on malformed values while removing `mapUint32`, `mapUint16`, `mapUint8`, `mapBool`, and ad-hoc `strconv.ParseUint` from `config.go` and `config_aigp.go` | Structural container navigation (`mapMap`) and multi-shape `family` / `community` / `bus` / `capability` subtrees keep their specialized parsers because they carry freeform/flex or cross-module structures outside plain scalar leaves; non-BGP consumers are out of scope for this spec |
| AC-4 | BGP leaf `bgp > peer > session > aigp > link-metric` set to `5000000000` (`> 2^32-1`) and to `18446744073709551615` (`2^64-1`) | Config load, `ze config validate`, and daemon apply accept both values and populate `PeerSettings.AIGPLinkMetric` with the exact `uint64` value (`5000000000` and `18446744073709551615`) | None: every value in `1..18446744073709551615` is valid under `ze-bgp-conf.yang` and must survive without truncation |
| AC-5 | Plugin config delivery (`Tree.ToPluginMap` / `rpc.ConfigSection` JSON) and `ze config diff` (`rpc.ConfigDiffSection` JSON) on an unchanged fixture config | Output is byte-identical to pre-change output (all scalar leaves remain JSON strings, `@list` sidecars and `DiffSection` fields preserved) and `./le config coercion check` passes | None: plugin JSON wire format and `ze config diff` output are external contracts (`ai/doors/system.md`, `ai/doors/config-yang.md`) |
| AC-6 | Any YANG `uint64` config leaf across `-conf` modules — both **ranged** (e.g., `bgp > peer > session > aigp > link-metric` and `ddos > detect > bps-floor`, `range "1..max"`) and **unranged** (e.g., `bgp-ls-export` / `fib p4` / `ospf` `type uint64;`, or direct `ValidateValue(TypeUint64, ...)`) — set to `18446744073709551615`, `0`, `18446744073709551616` (`2^64`), and `-1` / non-digits, plus web rendering of `TypeUint64` | `ze config validate` and `ze start` accept `18446744073709551615` on both ranged and unranged leaves (and `0` on unranged leaves); per `leaf_value.go:113-118`, a **ranged** leaf refuses `0` and all-digit overflow `18446744073709551616` with `rangeRefusal` naming the declared range (`1..18446744073709551615`), and refuses `-1` / non-digits with `"invalid uint64"`; an **unranged** leaf (and `ValidateValue`) refuses `18446744073709551616`, `-1`, and non-digits with `"invalid uint64"`; web UI renders `TypeUint64` as `<input type="text">` (`data-type="uint64"`) with `TestFieldInputRegistryAnswersEachType` green; closes owned journal row `declared-format-contradicts-payload.md` (2026-10-13) | None: every `type uint64` leaf across all loaded `-conf` modules maps through `yangTypeToValueType` and must honor full 64-bit unsigned bounds and exact `leaf_value.go` refusal semantics |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | commits or reloads a peer ASN change | config load -> `ToRunningMap()` -> `VerifyConfig(tree)` -> `ApplyConfigDiff(tree)` -> `loadPeersFromConfig` -> typed parse -> reconcile (zero disk re-read) | `test/plugin/config-apply-consumes-diff.ci` |
| 2 | sets `bgp peer ... session aigp link-metric 5000000000` or `ddos detect bps-floor 5000000000` | config parse -> `TypeUint64` validation -> `Tree` -> schema-driven typed decode -> `PeerSettings.AIGPLinkMetric` (`5000000000`) | `test/plugin/config-uint64-no-truncation.ci` |
| 3 | runs `ze config diff` and delivers plugin config sections after edit | `Tree.ToPluginMap()` / `DiffConfigs` -> string-valued JSON and `rpc.ConfigDiffSection` unchanged | existing config-diff and plugin `.ci` suites unchanged |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyConfigAndApplyConfigDiffConsumeTreeWithoutDiskRead` | `internal/component/bgp/reactor/reactor_api_test.go` | Both `VerifyConfig` and `ApplyConfigDiff` invoke `treeReloadFunc` and never call `reloadFunc(configPath)` when both are installed; bare reactor with `treeReloadFunc == nil` keeps fallback on `VerifyConfig` | |
| `TestTreeReloadMatchesFileReloadPeerSettings` | `internal/component/bgp/config/loader_test.go` | `createTreeReloadFunc` and `createReloadFunc` produce `peerSettingsEqual` `PeerSettings`, `Globals`, and `DynamicGroupConfig` for a full fixture (groups, defaults, static routes, policy, BFD, AIGP, dynamic groups) | |
| `TestPeersFromTreeTypedDecode` | `internal/component/bgp/reactor/config_test.go` | Schema-driven typed reader produces identical `PeerSettings` across ASN, router-id, timers, ports, capture, TTL, BFD, AIGP (`uint64`), booleans, and rejects malformed values | |
| `TestTreeTypedReaderSchemaDriven` | `internal/component/config/tree_test.go` | Read-side typed reader decodes `TypeUint64`, `TypeUint32`, `TypeUint16`, `TypeASN`, `TypeBool`, `TypeIPv4`, `TypeIPv6`, `TypeIP`, `TypeString`, enums, and leaf-lists from string-backed `Tree`/`map[string]any` and rejects type/overflow errors | |
| `TestYangUint64NoTruncationAndBounds` | `internal/component/config/yang_schema_test.go` | `yangTypeToValueType` maps `Yuint64 -> TypeUint64`; `ValidateValue` and unranged `ValidateLeafValue` accept `0`, `4294967296`, `18446744073709551615` and refuse `18446744073709551616`, `-1`, non-digits with `"invalid uint64"`; ranged `ValidateLeafValue` (`range "1..max"`) refuses `0` and all-digit `18446744073709551616` with `rangeRefusal` naming `1..18446744073709551615`, and `-1`/non-digits with `"invalid uint64"` | |
| `TestFieldInputRegistryAnswersEachType` | `internal/component/web/field_input_test.go` | `producedFieldTypes` walks through `TypeUint64`, `valueTypeToFieldType(TypeUint64)` answers `"uint64"`, and both `fieldComponent` and `leafInputType` render `TypeUint64` as `<input type="text">` without number-input `min`/`max` truncation | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `uint64` config leaf (unranged) / `ValidateValue(TypeUint64, ...)` | `0..18446744073709551615` | `18446744073709551615` (also test `0`, `4294967296`) | `-1` (refused: `"invalid uint64"`) | `18446744073709551616` (refused: `"invalid uint64"`) |
| `bgp > peer > session > aigp > link-metric` (ranged) | `1..18446744073709551615` | `18446744073709551615` (also test `4294967296`, `5000000000`) | `0` (refused: `rangeRefusal` naming `1..18446744073709551615`); `-1` (`"invalid uint64"`) | `18446744073709551616` (refused: `rangeRefusal` naming `1..18446744073709551615` per `leaf_value.go:116`) |
| `ddos > detect > bps-floor` (ranged non-BGP `uint64`) | `1..18446744073709551615` | `18446744073709551615` (also test `5000000000`) | `0` (refused: `rangeRefusal` naming `1..18446744073709551615`); `-1` (`"invalid uint64"`) | `18446744073709551616` (refused: `rangeRefusal` naming `1..18446744073709551615` per `leaf_value.go:116`) |
| `uint32` leaf (ASN / timers / capture) | `0..4294967295` | `4294967295` | `-1` (refused: `"invalid uint32"`) | `4294967296` (refused as `uint32` overflow / `rangeRefusal` on ranged leaves) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `config-apply-consumes-diff` | `test/plugin/config-apply-consumes-diff.ci` | Config reload updates peer ASN cleanly via resolved tree verify+apply without re-reading disk | |
| `config-uint64-no-truncation` | `test/plugin/config-uint64-no-truncation.ci` | `ze config validate` and daemon start/apply accept 64-bit `aigp link-metric` (`5000000000`, `18446744073709551615`), `ddos detect bps-floor`, and an unranged `uint64` leaf; refuse `0` and all-digit `18446744073709551616` on ranged leaves with `rangeRefusal`, and refuse `18446744073709551616` on an unranged leaf (and `-1` on both) with `"invalid uint64"` | |

### Interop Tests (Scope: config)
Not applicable: this is an internal config-validation and reload-loading change with no wire-format effect. Existing BGP interop scenarios remain unchanged as a regression gate.

## Files to Modify
- `internal/component/config/schema.go` - add `TypeUint64` to `ValueType` (after `TypeDecimal64`), `(ValueType).String()`, and `ValidateValue` (64-bit unsigned parse).
- `internal/component/config/yang_schema.go` - map `gyang.Yuint64` to `TypeUint64` in `yangTypeToValueType` and `"uint64"` in `getKeyTypeExtension`.
- `internal/component/config/leaf_value.go` - add `TypeUint64` arm in `parseNumericRangeValue` and tighten `TypeUint32` to 32-bit unsigned parse.
- `internal/component/config/tree.go` - add read-side schema-driven typed reader (`TypedReader` / `TypedNode`) over `*Tree` and `map[string]any` keyed by `*Schema` / `ChildProvider`; preserve string-backed storage and `ToMap`/`ToPluginMap`/`ToRunningMap`.
- `internal/component/web/handler_config_leaf.go` - add `config.TypeUint64` to the text-input case in `leafInputType` (like `TypeDecimal64`/`TypeASN`, avoiding browser `<input type="number">` / JS float precision loss above `2^53`).
- `internal/component/web/fragment.go` - add `config.TypeUint64` to the no-extra-metadata case in `buildFieldMeta` and map `config.TypeUint64` to `"uint64"` in `valueTypeToFieldType` (falling through `fieldInputs` to `inputText`).
- `internal/component/web/field_input_test.go` - add `"uint64": type="text"` to `fieldEditorMarkers` and extend `producedFieldTypes` walk sentinel to `config.TypeUint64`.
- `internal/component/bgp/reactor/reactor_api.go` - route `VerifyConfig` through `loadPeersFromConfig` when `treeReloadFunc` is set (keeping the `treeReloadFunc == nil` fallback for bare-reactor unit tests and leaving standalone `Reload()` on `reloadFunc`).
- `internal/component/bgp/reactor/reload_tree.go` - share tree-loading helper between `VerifyConfig` and `ApplyConfigDiff` and update doc comment.
- `internal/component/bgp/reactor/config.go` - migrate `PeersFromTree`, `parsePeerFromTree`, `parsePeerSettings`, `parseCaptureSettings`, `parseTTLSettings`, and `parseBFDSettings` scalar leaf reads onto the schema-driven typed reader; remove ad-hoc `mapUint32`, `mapUint16`, `mapUint8`, `mapBool`.
- `internal/component/bgp/reactor/config_aigp.go` - migrate `applyAIGPSettings` (`enabled`, `originate`, `link-metric` as `uint64`, `domain-as`) onto the schema-driven typed reader.
- `docs/architecture/config/apply-ordering.md` - update `## The decisions` ("Verify applies no dynamic group") to state that `VerifyConfig` consumes the handed tree via `TreeReloadFunc` alongside `ApplyConfigDiff`.
- `docs/architecture/web-interface.md` - add a `TypeUint64` text-input row to the `## YANG Schema Integration` rendering table alongside `TypeUint16/32`.

## Files to Create
- `test/plugin/config-apply-consumes-diff.ci` - functional proof that reload verify and apply consume the resolved tree without disk re-read.
- `test/plugin/config-uint64-no-truncation.ci` - functional proof that 64-bit YANG leaves (`aigp link-metric` and non-BGP `uint64` leaf) survive validation and apply, while out-of-range and `2^64` overflow values are refused.

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | No new YANG leaves or RPCs; fixes Go `ValueType` mapping for existing YANG `uint64` leaves (`yang_schema.go`) |
| YANG validation constraints | Yes | `internal/component/config/schema.go`, `leaf_value.go`, `yang_schema.go` enforce existing YANG `uint64` type and `range` constraints at 64-bit width |
| YANG custom validators | No | Native YANG `uint64` and `range` constraints suffice; no `ze:validate` added |
| CLI commands/flags | No | No CLI command or flag added or changed |
| CLI grammar (keyword before value) | N-A | No CLI command added or changed |
| Editor autocomplete | No | Automatic from existing YANG schema; `TypeUint64` leaves keep their existing completion |
| Functional test for new RPC/API | Yes | `test/plugin/config-apply-consumes-diff.ci`, `test/plugin/config-uint64-no-truncation.ci` prove end-to-end config reload and `uint64` validation/apply |
| Pipe completeness | N-A | No CLI command output added or changed |
| Env var registration | No | No env var added or changed |
| Doctor check for runtime dependencies | N-A | No new file path, socket, service, kernel module, port, procfs, netlink, binary, or certificate |
| Doctor code per problem class | No | Uses existing YANG validation / range refusal operator errors (`configyang.RangeRefusal` / `ValidateValue`) |
| Prometheus counters/metrics | No | No new operator-facing metric required; reload verification/apply uses existing `configReloads`/`configReloadErrors` metrics and unit/functional seam assertions |
| BGP family surface (new SAFI / capability / attribute) | N-A | No SAFI, capability, or BGP attribute added or changed |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Bug fix and internal decode cleanup; no feature level changes |
| 2 | Config syntax changed? | No | Config file syntax and set syntax are unchanged; `docs/architecture/config/syntax.md` unaffected |
| 3 | CLI command added/changed? | No | No CLI command added or changed |
| 4 | API/RPC added/changed? | No | No API or RPC added or changed |
| 5 | Plugin added/changed? | No | No plugin added or changed |
| 6 | Has a user guide page? | No | User-facing guides (`docs/guide/configuration.md`, `docs/guide/config-reload.md`, `docs/guide/config-editor.md`, `docs/guide/bgp-policy.md`, `docs/guide/command-reference.md`, `docs/guide/monitoring.md`, `docs/guide/plugins.md`, `docs/guide/policy-routing.md`, `docs/guide/authentication.md`, `docs/guide/web-interface.md`) describe unchanged operator syntax and behavior |
| 7 | Wire format changed? | No | No wire format changes (see Changed-Wire Census) |
| 8 | Plugin SDK/protocol changed? | No | `docs/architecture/api/process-protocol.md` unchanged (string-valued JSON delivery and reload protocol preserved) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No new RFC requirement row; `ValidateValue` already implements RFC 7950 Section 9.2 integer bounds |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` functional runner; `docs/functional-tests.md` unaffected |
| 11 | Affects daemon comparison? | No | Internal config pipeline repair; `docs/comparison.md` unaffected |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/apply-ordering.md` (`## The decisions` updated to document `VerifyConfig` consuming the handed tree via `TreeReloadFunc`); `docs/architecture/web-interface.md` (`## YANG Schema Integration` updated with the `TypeUint64` text-input row); `docs/architecture/core-design.md` unaffected (Section 18/18a already describe transaction reload and `peerSettingsEqual` without claiming a disk re-read) |
| 13 | Route metadata keys added/changed? | No | `docs/architecture/meta/role.md` unaffected |
| 14 | Prometheus counters added/changed? | No | No Prometheus counters added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registered plugin, event, send type, command, or capability changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Checked via `./le spec citation anchors`: update `docs/architecture/config/apply-ordering.md` (`## The decisions`, `loadPeersFromConfig`) and `docs/architecture/web-interface.md` (`## YANG Schema Integration`, `TypeUint64` text input). Unaffected (anchors point to unchanged contracts/symbols): `docs/DESIGN.md`, `docs/architecture/core-design.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md`, `docs/architecture/config/transaction-protocol.md`, `docs/architecture/config/change-file-structural-ops.md`, `docs/architecture/route-selection.md`, `docs/architecture/aaa-tacacs.md`, `docs/architecture/ssh/fixit-bcrypt-hash-credential.md`, `docs/architecture/web-components.md`, `docs/architecture/api/architecture.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/text-format.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/behavior/peer-lifecycle.md`, `docs/architecture/wire/nlri-bgpls.md`, `docs/architecture/meta/role.md`, `docs/architecture/rfc-may-decisions.md`, `docs/architecture/wire/capabilities.md`, `docs/architecture/hub-architecture.md`, `docs/config-migration.md`, `docs/features/bgp-protocol.md`, `docs/functional-tests.md`, `docs/guide/bgp-policy.md`, `docs/guide/command-reference.md`, `docs/guide/config-editor.md`, `docs/guide/config-reload.md`, `docs/guide/configuration.md`, `docs/guide/monitoring.md`, `docs/guide/plugins.md`, `docs/guide/policy-routing.md`, `docs/guide/authentication.md`, `docs/guide/web-interface.md` |
| 17 | Existing docs show config/CLI/API examples for this area? | No | Config syntax and CLI examples stay valid as written |

## Design presentation

### What changes

Configuration validation now honors full 64-bit unsigned integers (`TypeUint64`) across every YANG `-conf` module, so operators can configure `uint64` leaves above `4294967295` — such as `bgp > peer > session > aigp > link-metric` and `ddos > detect > bps-floor` — while respecting `leaf_value.go`'s value-refusal contract (on a ranged `uint64` leaf, out-of-range values and all-digit `2^64` overflow produce `rangeRefusal` naming the declared range while `-1` / non-digits produce `"invalid uint64"`; on an unranged `uint64` leaf or `ValidateValue`, `2^64`, `-1`, and non-digits produce `"invalid uint64"`). The web config editor renders `TypeUint64` leaves as `<input type="text">` (`data-type="uint64"`, matching `TypeDecimal64`/`TypeASN`) so browser number inputs never lose integer precision above `2^53`. On the BGP reload path, `VerifyConfig` joins `ApplyConfigDiff` in consuming the already-parsed whole-config tree handed by the reload coordinator via `TreeReloadFunc` instead of re-reading and re-parsing the config file from disk. Inside BGP peer config parsing, scalar leaves are decoded through one schema-driven typed reader in `internal/component/config/tree.go` backed by `config.YANGSchema()`, removing ad-hoc string hand-parsers while keeping plugin JSON config delivery (`ToPluginMap`) and `ze config diff` byte-identical.

### Before and after

1. **Configuring a 64-bit YANG leaf (`bgp > peer > session > aigp > link-metric`, `ddos > detect > bps-floor`, or an unranged `uint64` leaf):**
   - Before: `ze config validate` and `ze start` refuse `link-metric 5000000000;` or `bps-floor 5000000000;` because `yangTypeToValueType` collapsed `Yuint64` into `TypeUint32` (32-bit `ValidateValue`), and the web UI rendered those leaves as `<input type="number" max="4294967295">`.
   - After: `ze config validate` and `ze start` accept `5000000000` and `18446744073709551615`, populate `PeerSettings.AIGPLinkMetric` with the exact `uint64` value, refuse `0` and all-digit `18446744073709551616` on a ranged `1..max` leaf with `rangeRefusal` naming `1..18446744073709551615` (and `-1` / non-digits with `"invalid uint64"`), refuse `18446744073709551616` and `-1` on an unranged `uint64` leaf with `"invalid uint64"`, and render `TypeUint64` in the web editor as `<input type="text">` (`data-type="uint64"`).
2. **BGP reload verification (`VerifyConfig`):**
   - Before: `pluginserver.Server.reloadConfig` parsed the candidate once into `newTree` and handed it to `VerifyConfig(newTree)`, which ignored `newTree` on a file-configured reactor and called `reloadFunc(configPath)` to re-read and re-parse the file from disk a second time.
   - After: `VerifyConfig(newTree)` uses `loadPeersFromConfig(newTree)` (`TreeReloadFunc`), matching `ApplyConfigDiff(newTree)`, with zero disk re-read.
3. **BGP scalar leaf decoding (`PeersFromTree` / `parsePeerSettings` / `applyAIGPSettings`):**
   - Before: each leaf was hand-parsed from `map[string]any` strings via local `mapUint32`, `mapUint16`, `mapUint8`, `mapBool`, and `strconv.ParseUint(fmt.Sprint(raw), 10, 64)` helpers.
   - After: scalar leaves are read through the shared schema-driven typed reader in `internal/component/config/tree.go` keyed off `config.YANGSchema()`, while `ToPluginMap` JSON delivery and `ze config diff` stay string-valued and byte-identical.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Render `TypeUint64` in the web editor as `<input type="number" min="0" max="18446744073709551615">` | HTML `<input type="number">` uses IEEE-754 double-precision floats in browsers/JS and silently rounds integers above `2^53` (`9007199254740991`); text input (like `TypeDecimal64` and `TypeASN`) preserves exact digits while server-side `ValidateLeafValue` enforces full 64-bit bounds |
| Change `Tree.ToMap` / `Tree.ToPluginMap` to emit native JSON numbers and booleans | Breaks the published plugin config contract (`ai/doors/config-yang.md`, `docs/architecture/config/syntax.md`, `docs/architecture/config/yang-config-design.md` Section 14) and `./le config coercion check` across every in-tree and third-party plugin |
| Keep `VerifyConfig` on the disk re-read (`reloadFunc`) and fix only `TypeUint64` + typed decode | Leaves the redundant disk I/O and full re-parse on every production reload verify even though `reloadConfig` already hands `VerifyConfig` the exact `ToRunningMap()` tree that `ApplyConfigDiff` consumes |
| Migrate every non-BGP config consumer to the typed reader in the same spec | Would turn a targeted P1 fix into a repo-wide refactor touching dozens of unrelated plugins; establishing the generic schema-driven reader in `config/tree.go` and proving it on BGP keeps this spec focused and closeable |
| Route section-only callers (`ReconcilePeersWithJournal`, `PeerDiffCount`, `configop.ReconcilePeers`) through `TreeReloadFunc` by wrapping only `"bgp"` | `buildPeerPopulations` reads top-level `policy` and `bfd` containers from the whole config tree; wrapping `"bgp"` alone would silently drop peer policy chains and BFD checks |

### Documents departed from

- `docs/architecture/config/apply-ordering.md`, section `The decisions`: currently states `"Only a file-configured reactor also holds the ReloadFunc, which re-reads the file; verify keeps the file."` After this work, that sentence is updated to state that `VerifyConfig` consumes the handed whole-config tree via `TreeReloadFunc` alongside `ApplyConfigDiff` (with the `treeReloadFunc == nil` fallback preserved for bare-reactor unit tests), while `ReloadFunc` remains for standalone `Reload()` and section-only callers that do not receive the whole-config tree.

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** — add unit and functional tests proving `VerifyConfig` + `ApplyConfigDiff` consume the handed tree with zero `reloadFunc` invocation and proving `uint64` leaves (`> 2^32-1`) survive validation and BGP decode (tests fail against HEAD).
   - Tests: `TestVerifyConfigAndApplyConfigDiffConsumeTreeWithoutDiskRead`, `TestYangUint64NoTruncationAndBounds`, `test/plugin/config-apply-consumes-diff.ci`, `test/plugin/config-uint64-no-truncation.ci`.
   - Files: `internal/component/bgp/reactor/reactor_api_test.go`, `internal/component/config/yang_schema_test.go`, `test/plugin/config-apply-consumes-diff.ci`, `test/plugin/config-uint64-no-truncation.ci`.
   - Verify: tests fail on HEAD because `VerifyConfig` still calls `reloadFunc` and `yangTypeToValueType` still collapses `Yuint64` to `TypeUint32`.
2. **Phase: `TypeUint64` schema, validation & web rendering** — add `TypeUint64` to `ValueType` (after `TypeDecimal64`), `String()`, `ValidateValue`, `yangTypeToValueType`, `getKeyTypeExtension`, and `parseNumericRangeValue`; wire `TypeUint64` as text input in `internal/component/web/` (`handler_config_leaf.go`, `fragment.go`, `field_input_test.go`) and update `docs/architecture/web-interface.md` `## YANG Schema Integration`; update owned journal row `declared-format-contradicts-payload.md` (2026-10-13) once proven.
   - Tests: `TestYangUint64NoTruncationAndBounds`, `TestFieldInputRegistryAnswersEachType`, `test/plugin/config-uint64-no-truncation.ci`.
   - Files: `internal/component/config/schema.go`, `internal/component/config/yang_schema.go`, `internal/component/config/leaf_value.go`, `internal/component/web/handler_config_leaf.go`, `internal/component/web/fragment.go`, `internal/component/web/field_input_test.go`, `docs/architecture/web-interface.md`.
   - Verify: scoped `internal/component/config` and `internal/component/web` tests and `config-uint64-no-truncation.ci` pass.
3. **Phase: BGP `VerifyConfig` consumes the handed tree** — route `VerifyConfig` through `loadPeersFromConfig` when `treeReloadFunc` is set (keeping `treeReloadFunc == nil` fallback), update `docs/architecture/config/apply-ordering.md`, and update the five unit tests that passed empty dummy maps to `VerifyConfig` (with `ze-test-judge` review).
   - Tests: `TestVerifyConfigAndApplyConfigDiffConsumeTreeWithoutDiskRead`, `TestTreeReloadMatchesFileReloadPeerSettings`, `test/plugin/config-apply-consumes-diff.ci`.
   - Files: `internal/component/bgp/reactor/reactor_api.go`, `internal/component/bgp/reactor/reload_tree.go`, `docs/architecture/config/apply-ordering.md`, plus the five affected unit test files in `internal/component/bgp/{reactor,config}/`.
   - Verify: `config-apply-consumes-diff.ci` and scoped `internal/component/bgp/...` tests pass.
4. **Phase: Schema-driven typed decode for the BGP subtree** — add the read-side schema-driven typed reader in `internal/component/config/tree.go` and migrate `internal/component/bgp/reactor/config.go` and `config_aigp.go` off ad-hoc `mapUint32`/`mapUint16`/`mapUint8`/`mapBool`/`strconv.ParseUint(fmt.Sprint(raw), ...)`.
   - Tests: `TestTreeTypedReaderSchemaDriven`, `TestPeersFromTreeTypedDecode`, `./le config coercion check`, `./le arch tier check`.
   - Files: `internal/component/config/tree.go`, `internal/component/bgp/reactor/config.go`, `internal/component/bgp/reactor/config_aigp.go`.
   - Verify: all unit tests, `.ci` functional tests, and config/arch gates pass; `ToPluginMap` and `ze config diff` remain byte-identical.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has implementation at file:line and a passing test |
| Correctness | Tree-loaded `PeerSettings` equal file-loaded `PeerSettings` under `peerSettingsEqual`; zero false diffs; `uint64` boundary values (`0`, `1`, `4294967295`, `4294967296`, `5000000000`, `18446744073709551615`, `18446744073709551616`) all behave per AC-4 and AC-6 |
| Data flow | Both `VerifyConfig` and `ApplyConfigDiff` consume the handed `ToRunningMap()` tree through `loadPeersFromConfig`; zero disk re-read on production reload |
| Registration over hardcoding | Typed reader in `config/tree.go` dispatches only on schema `ValueType`; zero BGP imports or per-leaf switches in `internal/component/config` |
| Coercion and wire preservation | `./le config coercion check` passes; `ToMap`, `ToPluginMap`, `ToRunningMap`, and `ze config diff` JSON are byte-identical |
| Rule: no-layering | `VerifyConfig` no longer routes through `loadPeersFullOrTree`; `docs/architecture/config/apply-ordering.md` updated in the same change |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No disk re-read in `VerifyConfig` or `ApplyConfigDiff` | `grep -n loadPeersFullOrTree internal/component/bgp/reactor/reactor_api.go` shows `VerifyConfig` and `ApplyConfigDiff` both call `loadPeersFromConfig`; `TestVerifyConfigAndApplyConfigDiffConsumeTreeWithoutDiskRead` and `test/plugin/config-apply-consumes-diff.ci` pass |
| Schema-driven typed decode on BGP subtree | `grep -n "func mapUint\|func mapBool" internal/component/bgp/reactor/config.go` returns empty; `TestTreeTypedReaderSchemaDriven`, `TestPeersFromTreeTypedDecode`, and `./le config coercion check` pass |
| `TypeUint64` full 64-bit validation, decode, and web rendering | `TestYangUint64NoTruncationAndBounds`, `TestFieldInputRegistryAnswersEachType`, and `test/plugin/config-uint64-no-truncation.ci` pass; owned journal row `declared-format-contradicts-payload.md` (2026-10-13) marked fixed |
| Docs updated | `docs/architecture/config/apply-ordering.md` and `docs/architecture/web-interface.md` updated and `./le doc check all` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Typed decode and `TypeUint64` reject negative numbers, non-digits, `>= 2^64` overflow, and out-of-range values with clear errors and zero panics |
| No silent default fallback | Malformed config values return explicit errors rather than silently falling back to zero/false defaults |

### Failure Routing
| Failure | Route To |
|---------|----------|
| False diff after `VerifyConfig` tree consumption | Compare `TreeFromPluginMap` output against `LoadConfig` output in `TestTreeReloadMatchesFileReloadPeerSettings` |
| Unit test fails because it passed `map[string]any{}` to `VerifyConfig` | Update test to pass loaded candidate tree and obtain `ze-test-judge` approval |
| `./le config coercion check` flags a typed-reader helper | Ensure every value coercion handles `case string:` and never asserts `v.(bool)` or `v.(float64)` directly |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask the user. |

## Design Insights
<!-- LIVE: write immediately when you learn something. Route each lesson to its governing surface under ai/rules/planning.md. -->
- 2026-11-26 (verified against HEAD): `ApplyConfigDiff`, `RebuildRunningPeers`, and `createdPeerFromRunning` already migrated onto `loadPeersFromConfig` (`TreeReloadFunc`, `internal/component/bgp/reactor/reload_tree.go`). Only `VerifyConfig` remains on `loadPeersFullOrTree` on the coordinator reload path, and `docs/architecture/config/apply-ordering.md` (`## The decisions`) explicitly recorded `"verify keeps the file"`, making that change a documented `departs` row.
- 2026-11-26 (verified against HEAD): `ze-bgp-conf.yang:810` declares `bgp > peer > session > aigp > link-metric` as `type uint64 { range "1..max"; }`, giving a concrete BGP-consumed `uint64` leaf alongside eight non-BGP `-conf` `uint64` leaves (`ddos detect bps-floor`, etc.).

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `declared-format-contradicts-payload.md` | 2026-10-13 | `yangTypeToValueType`: `uint64` validated as `uint32` |

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Route `VerifyConfig` through existing `loadPeersFromConfig` (`TreeReloadFunc`) alongside `ApplyConfigDiff` | Keep `VerifyConfig` re-reading disk via `reloadFunc`, or wrap `"bgp"` alone for section-only callers | Eliminates redundant disk I/O and re-parse on production reload verify using the already-proven whole-tree pipeline (`ApplyConfigDiff`), without breaking `buildPeerPopulations`'s access to top-level `policy` and `bfd` |
| Keep `Tree` storage and `ToMap`/`ToPluginMap`/`ToRunningMap` string-valued, and add a read-side schema-driven typed reader in `config/tree.go` | Store native Go types in `Tree` or emit JSON numbers/booleans in `ToPluginMap` | Preserves plugin JSON wire contract (`ai/doors/config-yang.md`, AC-5), `ze config diff`, and `./le config coercion check` while removing per-leaf hand parsers in BGP |
| Scope typed decode migration to the BGP subtree first | Repo-wide migration of every config consumer in one spec | Keeps this P1 fix focused and closeable while establishing the reusable schema-driven reader in `internal/component/config/tree.go` |

## Known Limitations
- Broad migration of non-BGP config consumers onto the schema-driven typed reader in `internal/component/config/tree.go` is a post-release follow-on; this spec establishes the reader and migrates the BGP subtree (`config.go`, `config_aigp.go`).
- Standalone `Reload()` (`reactor_api.go`) continues to read from `reloadFunc(configPath)` because standalone SIGHUP has no caller-supplied tree; section-only callers (`PeerDiffCount`, `ReconcilePeersWithJournal`, `configop.ReconcilePeers`) keep `loadPeersFullOrTree` because their section payload omits top-level `policy` and `bfd` roots.
- Command/response envelope re-parsing (`spec-unify-response-envelope.md`) and BGP hot-path text serialization (`spec-unify-filters.md`, redistribution text injection) are owned by their respective specs.

## Review Gate

<!-- BLOCKING (ai/rules/planning.md Review Gate). Filled by /ze-implement's /ze-review gate over the complete diff. -->

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
|   | BLOCKER / ISSUE / NOTE | [what /ze-review reported] | file:line | fixed in `<commit/line>` / deferred (id) / acknowledged |

### Fixes applied
- [short bullet per BLOCKER/ISSUE, naming the file and change]

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Scope, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior, Existing Mechanisms, Design documents, Data Flow (with Changed-Wire Census), and Design presentation sections completed
- [ ] AC-1..AC-6 carry positive observable assertions and Permitted Survivors and Why
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration and Documentation checklists answered Yes/No/N-A with source evidence

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: scoped package tests (`./le test unit pkg path ./internal/component/config/`, `./le test unit pkg path ./internal/component/web/`, `./le test unit pkg path ./internal/component/bgp/reactor/`, `./le test unit pkg path ./internal/component/bgp/config/`), config/arch gates (`./le config coercion check`, `./le arch tier check`, `./le doc check all`), and the functional `.ci` tests (`test/plugin/config-apply-consumes-diff.ci`, `test/plugin/config-uint64-no-truncation.ci`). No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own or a stated non-goal

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (`uint64`, `uint32`)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests N-A (internal config change; existing BGP interop is the regression gate)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `./le spec review record`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; owned journal row `declared-format-contradicts-payload.md` (2026-10-13) marked fixed
- [ ] **Commit A:** code + tests + docs + edited spec + journal row update
- [ ] **Commit B:** `remove plan/release/fixes/spec-review-typed-config-decode.md` only, in the same `./le commit create` script
