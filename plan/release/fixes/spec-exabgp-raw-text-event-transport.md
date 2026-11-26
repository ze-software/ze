# Spec: ExaBGP Raw-Text Event Transport

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Priority | P2 |
| Release reason | Q-E7 benchmark on Apple M4 Max (2026-11-25) measured ~1.7 us/route/script CPU and ~72-94 MiB private RAM per `ze exabgp plugin` endpoint because every BGP UPDATE is decoded into `FormatFull` JSON (~1.5-2 KiB), JSON-string-escaped inside `deliver-batch` (via `map[string]any` + `json.Marshal` on `MuxConn`), TLS-encrypted N times, unmarshaled into `map[string]any` in every wrapper, re-parsed off wire hex anyway, and marshaled back via reflection. |
| RFC gaps | - |
| Depends | `plan/release/fixes/spec-bus-observer-delivery-decoupling.md` |
| Phase | 4/4 |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Eliminate redundant `FormatFull` JSON formatting in `ze`, per-batch `map[string]any` marshaling on `MuxConn`, inbound `json.Unmarshal` reflection, and hot-path outbound `map[string]any` reflection inside `ze exabgp plugin` by implementing `plugin.FormatBase64 = "base64"` as a positional counted raw-in-text UPDATE event format following Ze's existing Answer Protocol positional counted wire grammar (zero key names, 3-byte kind word closed by space, decimal counted numbers, and `<byte-len>:<bytes>` counted fields with `0:` when absent), opting `ze exabgp plugin` into `base64` via its existing Stage 5 `ready` `SetEventFormat` call, routing `MuxConn` batch delivery through pooled `rpc.WriteBatchFrame`, and rendering hot IPv4/IPv6 unicast UPDATEs directly from BGP wire bytes into a `[]byte` buffer while reusing the wire-backed `Event` renderer for non-unicast families and error cases.

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/plugin-infra.md`, `ai/doors/bgp-engine.md`, `ai/doors/system-plugins.md`, `ai/doors/core.md`, `ai/doors/config-yang.md`
  → Constraint: `internal/exabgp/bridge` is always-on unless a file carries `//go:build ze_bgp` (as `bridge_wire_json.go` already does); always-on code MUST NOT import gated `internal/component/bgp/*` packages (`./le arch tier check`).
  → Constraint: `pkg/plugin/rpc` is public wire API; `MuxConn.CallBatchRPC` must preserve the exact `#<id> ze-plugin-callback:deliver-batch {"events":[...]}` wire contract and `MaxMessageSize` batch splitting (`Conn.CallBatchRPC`).
  → Constraint: Any wire event format addition preserves default `encoding json` / `format parsed` / `format full` / `format raw` / `format hex` behavior for third-party plugins.
- [ ] `docs/architecture/exabgp-bridge.md`
  → Decision: ExaBGP wrapper endpoints request `base64` UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer and falls back to the wire-backed `Event` renderer for non-unicast families and error cases (2026-11-25).
  → Constraint: ExaBGP 4.2/5.0/6.0 JSON (`marshalExabgpJSON` spacing: `{ "key": value }`, `[ a, b ]`) and text (`Event.AppendText`) output must stay byte-identical across all families, `--form` (`parsed`, `packets`, `consolidate`), `bgp:negotiated` grants, dynamic `peer-as auto` = 0, EOR markers, and Q-E4 reconnect/missing-`negotiated` handling.
- [ ] `docs/architecture/api/ipc_protocol.md` and `docs/architecture/api/wire-format.md`
  → Constraint: Ze's high-performance positional wire grammar (Answer Protocol) uses a 3-byte kind token closed by space (single `uint32` load), zero key names on the wire, fixed positional field counts, decimal counted numbers closed by space, and `<byte-len>:<bytes>` counted fields (`0:` when absent) so readers advance by integer arithmetic without scanning payload bytes.
  → Constraint: Lines are terminated by exactly one `\n` (never `\r\n`).
- [ ] `docs/architecture/api/process-protocol.md`
  → Constraint: Engine-to-plugin event delivery is strictly lossless (zero-drop owner ruling, 2026-11-25) and runs over newline-framed `MuxConn` `deliver-batch` frames; quote-free ASCII event lines avoid JSON string escaping overhead inside `deliver-batch`.
  → Constraint: Hot-path event formatters follow the stack-local scratch discipline (`var scratch [4096]byte` with single conversion at the cache edge, never a struct field or `sync.Pool`).
- [ ] `docs/architecture/api/json-format.md`, `docs/architecture/api/text-format.md`, `docs/architecture/api/text-parser.md`, `docs/architecture/api/text-coverage.md`
  → Constraint: Existing `format=raw` / `format=hex` (`peer <ip> <dir> update raw <hex>`) and native parsed text (`peer <ip> remote as <asn> ...`) grammars must stay unchanged for existing callers/tests.
  → Constraint: `format=full` attaches `"raw":{"message":"<hex>"}` on OPEN/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH; `format=base64` must attach the same `"raw":{"message":"<hex>"}` on those four non-UPDATE kinds so `--form packets|consolidate` keeps working.

### RFC Summaries
- [ ] `rfc/short/rfc4271.md` - BGP-4 UPDATE structure, 19-octet header reconstruction for `--form packets|consolidate`, and withdraw+announce precedence.
  → Constraint: Reconstructed packet header is 16-byte all-ones marker + 2-byte total length (`19 + len(body)`) + 1-byte type (`2` for UPDATE).
- [ ] `rfc/short/rfc4724.md` - End-of-RIB marker detection (empty IPv4 UPDATE body or empty `MP_UNREACH_NLRI`).
  → Constraint: `base64` carries the raw UPDATE body bytes so `WireUpdate.IsEOR()` detects both IPv4 unicast and MP EOR markers without special daemon-side JSON framing.
- [ ] `rfc/short/rfc7911.md` - ADD-PATH 4-octet Path Identifier per family and per direction.
  → Constraint: ADD-PATH is not self-describing in BGP UPDATE wire bytes; `base64` MUST carry per-family ADD-PATH flags for the UPDATE's direction (`EncodingContext.AddPathFor(fam)`).
- [ ] `rfc/short/rfc8654.md` - Extended BGP message size up to 65,535 octets (body up to 65,516 octets).
  → Constraint: `base64` decoder MUST accept valid UPDATE bodies up to 65,516 octets (spilling cleanly past stack scratch) and reject bodies above 65,516 octets.

**Key insights:**
- **Stage 5 negotiation already exists:** `sdk.Plugin.SetEventFormat` (`pkg/plugin/sdk/sdk_callbacks.go:1034`) already sends `ReadyInput.Format` to `Server.registerReceiveDeclarations` (`internal/component/plugin/server/bus_catalog.go:189-200`), and `ze exabgp plugin` already calls it at `internal/plugins/exabgp/main_sdk.go:133, 138, 143` — zero operator config changes needed.
- **Positional counted wire line matches Ze's Answer Protocol:** `upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>\n` uses zero key names, a 4-byte kind load (`"upd "`), fused decimal number parsing (~3.2 ns each), bitmask flags for direction / `has-asn4` / `asn4` / `has-remote-as` / IPv4-unicast ADD-PATH / IPv6-unicast ADD-PATH, `0:` for empty counted fields, and byte-counted base64 slicing with zero quotes/backslashes.
- **`MuxConn` currently bypasses `rpc.WriteBatchFrame`:** `internal/component/plugin/ipc/rpc.go:131-142` routes `pc.mux != nil` (every external TLS plugin) through `[]json.RawMessage` + `map[string]any{"events": rawEvents}` + `json.Marshal` instead of the pooled `rpc.WriteBatchFrame`. Adding `MuxConn.CallBatchRPC` fixes that for all external plugins.
- **Bounded `EncodingContext` cache required:** Decoding raw UPDATEs inside `ze exabgp plugin` must reuse a bounded `(asn4, addPathSet)` -> `bgpctx.ContextID` cache rather than calling `bgpctx.Registry.Register` per UPDATE.
- **Compiled NLRI decoders are already linked:** `ze exabgp plugin` runs the `ze` binary (`plugin/all`), so all 10 compiled `internal/component/bgp/plugins/nlri/*` decoders register `InProcessNLRIDecoder` in `internal/component/plugin/registry` at `init()`.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `pkg/plugin/sdk/sdk_callbacks.go` (`SetEventFormat`, `SetEncoding`) - populates Stage 5 `ReadyInput.Format` and `ReadyInput.Encoding`.
- [ ] `pkg/plugin/rpc/message.go` (`appendCountedNumber`, `appendCountedText`, `cutCountedNumber`, `cutCountedText`, `answerWordOf`) - implements Ze's zero-key-name positional counted line primitives.
- [ ] `pkg/plugin/rpc/batch.go` (`WriteBatchFrame`) and `pkg/plugin/rpc/mux.go` (`MuxConn.CallRPC`) - `WriteBatchFrame` formats `deliver-batch` into a pooled buffer, but `MuxConn` has no `CallBatchRPC` method.
- [ ] `internal/component/plugin/ipc/rpc.go` (`PluginConn.CallBatchRPC`) - bypasses `WriteBatchFrame` when `pc.mux != nil`, allocating `[]json.RawMessage` + `map[string]any` + `json.Marshal` per batch.
- [ ] `internal/component/plugin/server/bus_catalog.go` (`registerReceiveDeclarations`) and `process/process.go` (`SetFormat`, `FormatCacheKey`) - applies `ready.Format` unconditionally and precomputes `format+encoding` cache key.
- [ ] `internal/component/plugin/types.go` - declares `FormatHex`, `FormatBase64`, `FormatParsed`, `FormatRaw`, `FormatFull`, `FormatSummary`.
- [ ] `internal/plugins/meta/cmd/plugin_config.go` (`handleBgpPluginFormat`) - accepts runtime `bgp plugin format <hex|base64|parsed|full>`.
- [ ] `internal/component/bgp/server/events.go` (`onMessageReceived`, `onMessageBatchReceived`, `onMessageSent`, `formatMessageForSubscription`) - formats UPDATEs via `format.AppendMessage`/`AppendSentMessage` (cached once per `FormatCacheKey`) and non-UPDATE wire messages via `encoder.Message`.
- [ ] `internal/component/bgp/format/text_update.go` (`appendMessageTyped`, `appendFromFilterResult`, `appendRawFromResult`, `appendFullFromResult`, `appendRawSectionsJSON`, `appendEORUpdateJSON`) - `FormatFull` parses every attribute + NLRI into JSON AND appends hex `raw` sections.
- [ ] `internal/component/bgp/format/json_message.go` (`JSONEncoder.Message`) - attaches `"raw":{"message":"<hex>"}` on OPEN/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH only when `fmtMode == plugin.FormatFull`.
- [ ] `internal/plugins/exabgp/main_sdk.go`, `bridgerun/forms.go` (`EventFormat`), `bridgerun/fleet.go` (`Fleet.Broadcast`) - `EventFormat` requests `FormatFull` whenever `bgp:update` or `--form packets|consolidate` is active; `Fleet.Broadcast` calls `json.Unmarshal` into `map[string]any` per event.
- [ ] `internal/exabgp/bridge/bridge_event.go`, `bridge_event_json.go`, `bridge_event_packets.go`, `bridge_event_wire.go`, `bridge_event_text.go`, `bridge_event_text_nlri.go`, `bridge_event_text_community.go`, `bridge_negotiated.go`, `bridge_wire_json.go` (`//go:build ze_bgp`) - `exabgpEnvelope` calls `os.Hostname`/`os.Getpid`/`os.Getppid` per event, `exabgpJSON` builds `map[string]any` and calls `marshalExabgpJSON`, `overlayWire` re-decodes `AttributesWire` off hex, and `bridge_wire_json.go` already imports `wireu.WireUpdate` under `//go:build ze_bgp`.

**Behavior to preserve:**
- Exact ExaBGP 4.2, 5.0, and 6.0 JSON (including `marshalExabgpJSON` spacing and alphabetical key order) and text output on script stdin across all address families, `--form` (`parsed`, `packets`, `consolidate`), `bgp:negotiated` grants, dynamic `peer-as auto` = 0, EOR markers, unsplit/undecoded NLRI warning logs, and Q-E4 reconnect/missing-`negotiated` handling.
- Existing `encoding json` + `format parsed`/`full`/`raw`/`hex`/`summary` and `encoding text` + `format parsed`/`raw`/`hex` outputs for all existing plugins and tests.
- Standalone `ze exabgp plugin` stdin/stdout runner and `bridge.ReadEvent` JSON path.
- Non-`ze_bgp` build compilation (`//go:build !ze_bgp` stub keeps `EventFormat` on `FormatFull`).
- Exact `deliver-batch` JSON RPC wire framing and `MaxMessageSize` batch splitting on `MuxConn`.

**Behavior to change:**
- Implement `plugin.FormatBase64 = "base64"` (the existing 4-value `zt:wire-format` enum value already accepted by `bgp plugin format`) as positional counted UPDATE lines beside JSON control events.
- Format UPDATEs with `FormatBase64` in `internal/component/bgp/format/raw_b64.go` using Ze's positional counted wire line (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>\n`), bypassing `bgpfilter.ApplyToUpdate` and JSON formatting completely; attach `raw.message` on non-UPDATE wire messages in `JSONEncoder.Message` for `FormatBase64`.
- Add `MuxConn.CallBatchRPC` using pooled `rpc.WriteBatchFrame` (with `MaxMessageSize` splitting) and route `PluginConn.CallBatchRPC` through it for external TLS plugins.
- Opt `ze exabgp plugin` into `FormatBase64` in `bridgerun.EventFormat` (when `bridge.SupportsRawWire()` is true), parse `upd ...` lines without `json.Unmarshal` in `Fleet.Broadcast`, cache process envelope metadata and pre-serialized `negotiated` JSON bytes per session, render hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from `WireUpdate` + `AttributesWire`, and fall back to the wire-backed `Event` renderer for non-unicast families and split/decode error cases.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Stage 5 `ready` format negotiation | `pkg/plugin/sdk/sdk_callbacks.go`, `SetEventFormat`; `internal/component/plugin/server/bus_catalog.go`, `registerReceiveDeclarations`; `internal/core/ipc/yang/ze-plugin-engine.yang`, `rpc ready` | Lets any plugin declare its wire event format at Stage 5 without operator config; `ze exabgp plugin` already calls `SetEventFormat` | Extends: `ze exabgp plugin` requests `plugin.FormatBase64` through this existing Stage 5 mechanism |
| Positional counted wire grammar | `pkg/plugin/rpc/message.go`, `appendCountedNumber`, `appendCountedText`, `cutCountedNumber`, `cutCountedText`, `answerWordOf`; `docs/architecture/api/ipc_protocol.md`, "Answer Protocol" | Encodes/parses fixed-position 3-byte kind + decimal numbers + `<byte-len>:<bytes>` fields with zero key names and `0:` for absent fields | Extends: applies this exact positional counted wire grammar to the `base64` UPDATE line (`upd ...`) so parsing takes ~15 ns with zero keyword scanning |
| Pooled `deliver-batch` frame writer | `pkg/plugin/rpc/batch.go`, `WriteBatchFrame`; `pkg/plugin/rpc/conn.go`, `Conn.CallBatchRPC` | Builds `#<id> ze-plugin-callback:deliver-batch {"events":[...]}` in a pooled buffer with `MaxMessageSize` batch splitting on direct `Conn`, while `MuxConn` lacks it and falls back to `map[string]any` + `json.Marshal` | Extends: adds `MuxConn.CallBatchRPC` using pooled `WriteBatchFrame` and wires `PluginConn.CallBatchRPC` to it for all external TLS plugins |
| Raw text UPDATE formatter | `internal/component/bgp/format/text_update.go`, `appendMessageTyped`, `appendRawFromResult` | Emits `peer <ip> <dir> update raw <hex>` (text) or JSON with hex `raw.update`, skipping `FilterResult` attribute/NLRI parsing | Extends: adds a sibling `FormatBase64` fast path at the top of `appendMessageTyped` (`appendRawB64Update`) without changing `FormatRaw`/`FormatHex` output |
| Non-UPDATE raw byte attachment | `internal/component/bgp/format/json_message.go`, `JSONEncoder.Message` | Attaches `"raw":{"message":"<hex>"}` to OPEN/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH when `fmtMode == plugin.FormatFull` | Extends: attaches `"raw":{"message":"<hex>"}` for `plugin.FormatBase64` as well so `--form packets\|consolidate` keeps working |
| ExaBGP wire attribute & update decoder | `internal/exabgp/bridge/bridge_event_wire.go`, `overlayWire`; `internal/exabgp/bridge/bridge_wire_json.go`, `WireUpdateToExabgpJSON` (`//go:build ze_bgp`) | Already parses BGP UPDATE wire bytes via `wireu.WireUpdate` and `attribute.AttributesWire` under `//go:build ze_bgp` | Extends: parses `base64` `upd` lines directly into `WireUpdate` + `AttributesWire`, rendering hot IPv4/IPv6 unicast UPDATEs straight into a `[]byte` buffer and falling back to wire-backed `Event` for non-unicast families and errors |
| Compiled NLRI plugin decoder registry | `internal/component/plugin/registry/registry.go`, `InProcessNLRIDecoder`; `internal/component/bgp/plugins/nlri/*/register.go` | Registers in-process NLRI decoders for all 10 non-unicast families at `init()` inside every `ze` binary | Extends: `ze exabgp plugin` (running the `ze` binary) reuses the registered decoders on the non-unicast fallback path |
| `FormatBase64` (`"base64"`) | `internal/component/plugin/types.go`, `FormatBase64`; `internal/component/config/yang/modules/ze-types.yang`, `typedef wire-format`; `docs/architecture/api/ipc_protocol.md`, "Plugin Configuration" | Existing 4-value `wire-format` enum value (`hex`, `base64`, `parsed`, `full`), previously unimplemented in `text_update.go` | Implements: merges raw-in-text base64 UPDATE delivery into the existing `base64` (`plugin.FormatBase64`) format per Thomas's 2026-11-26 decision so `ze-types.yang` stays at 4 working values with no dead enum |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | Ze is a platform, and other people's code runs against it | the door every change reads | "A change under `pkg/` is a change to a published contract. A type, a field, a callback name and the bytes on the wire all count. The SDK stays generic and carries no plugin-specific code" | agrees | `pkg/plugin/rpc` adds only generic `MuxConn.CallBatchRPC` preserving identical `deliver-batch` wire bytes; `pkg/plugin/sdk` is untouched; third-party plugin defaults and ExaBGP script output stay byte-identical |
| `docs/architecture/api/process-protocol.md` | Overview | `// Design:` of `types.go`, `ipc/rpc.go`; anchored to `ze-plugin-engine.yang`, `mux.go`; modified | "Ze plugins communicate with the engine via newline-framed YANG RPCs over a single bidirectional connection. All messages use the wire format `#<id> <verb> [<json>]\n`." | agrees | `base64` UPDATE lines travel inside existing newline-framed `deliver-batch` frames with no wire-framing change and zero-drop delivery preserved |
| `docs/architecture/api/architecture.md` | BGP Plugin Configuration | anchored to `types.go`, `mux.go`, `ipc/rpc.go`; modified | "Both parsed AND wire bytes" | departs | Updates the `bgp plugin format base64` row to state positional counted base64 UPDATE lines (`upd ...`) beside JSON control events with `raw.message`, keeping the 4-value set (`hex`, `base64`, `parsed`, `full`) |
| `docs/architecture/api/commands.md` | BGP Plugin Configuration | `// Design:` of `plugin_config.go`; anchored to `types.go`, `ze-bgp-api.yang`, `ipc/rpc.go`; modified | "bgp plugin format full       # Both parsed AND wire bytes" | departs | Updates `bgp plugin format base64` to describe positional counted base64 UPDATE lines beside JSON control events, keeping the 4-value command block |
| `docs/architecture/api/ipc_protocol.md` | BGP Namespace | `// Design:` of `mux.go`; anchored to `types.go`, `ze-bgp-api.yang`, `ze-plugin-engine.yang`, `ipc/rpc.go`; modified | "`encoding json` → format applies" | departs | Implements `bgp plugin format base64` using Ze's Answer Protocol positional counted `upd` wire grammar for UPDATEs while non-UPDATE control events stay JSON with `raw.message`, and routes `MuxConn.CallBatchRPC` through pooled `WriteBatchFrame` |
| `docs/architecture/api/json-format.md` | With Raw Wire Bytes (format=full) | `// Design:` of `text_update.go`, `json_message.go`; anchored to `types.go`, `bridge_event.go`; modified | "a subscriber of any other format is written no `raw` member." | departs | `format=base64` also attaches `"raw":{"message":"<hex>"}` on OPEN/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH and moves ExaBGP bridge UPDATE subscriptions from `format=full` to `format=base64` |
| `docs/architecture/cli/command-verbs.md` | The verb classes | anchored to `types.go`, `ze-types.yang` | - | silent | Covers CLI verb classification (`show`/`request`/`delete`/`update`); this spec adds no CLI verb or command |
| `docs/architecture/cli/error-surface.md` | The operator error | anchored to `types.go` | - | silent | Covers interactive CLI/TUI two-row error rendering and `opererr` responses; this spec adds no operator error surface |
| `docs/architecture/config/transaction-protocol.md` | 1. Design Principles | anchored to `types.go`, `ipc/rpc.go` | - | silent | Covers config verify/apply/rollback transactions; this spec changes no config transaction behavior |
| `docs/architecture/core-design.md` | JSON Events (Engine → Plugin) | `// Design:` of `bridge_event.go`; anchored to `types.go`; modified | "When a process binding requests JSON output, the engine sends formatted BGP event payloads via `deliver-event` or `deliver-batch`:" | departs | Under `format=base64` (keeping `encoding=json` for low-frequency control/session events), UPDATEs are delivered via `deliver-batch` as quote-free positional counted `upd` lines rather than JSON objects; updates Section 6 in Phase 4 |
| `docs/architecture/encoding-context.md` | Overview | anchored to `types.go` | "2. **EncodingContext** - Tracks "how to encode" (ASN4, ADD-PATH, ExtNH per family)" | agrees | `base64` reads `ASN4` and per-family `AddPathFor(fam)` from `msg.SourceEncodingContext()` and reuses a bounded `(asn4, addPathSet)` → `ContextID` cache in `bridge` without registering per UPDATE |
| `docs/architecture/plugin-manager-wiring.md` | Solution: Two-Phase Startup | anchored to `types.go` | - | silent | Covers `PluginManager` process spawning vs `Server` handshake ownership; this spec does not change startup wiring or phase order |
| `docs/architecture/web-interface.md` | Component Structure | anchored to `types.go` | - | silent | Covers HTMX web server and editor routes; this spec touches no web handler or template |
| `ai/doors/plugin-infra.md` | Boundaries | door covering `types.go`, `mux.go`, `ipc/rpc.go`, `mux_batch_test.go` | "A compiled-in and an external plugin differ only in how they reach the bus." | agrees | `FormatBase64` works uniformly via Stage 5 `ready.format` on any transport, and `MuxConn.CallBatchRPC` brings external TLS batching to parity with `Conn.CallBatchRPC`'s pooled `WriteBatchFrame` |
| `docs/architecture/config/yang-config-design.md` | Type Library: `ze-types.yang` | anchored to `ze-types.yang`, `ze-bgp-api.yang` | "Defines reusable types and groupings imported by other modules. Contains no tree nodes" | agrees | Updates `enum base64` help in the existing 4-value `zt:wire-format` typedef in `ze-types.yang` without adding any duplicate enum or config tree nodes |
| `ai/doors/config-yang.md` | Boundaries | door covering `ze-types.yang` | "The generated YANG glue is not hand-edited." | agrees | Runs `./le yang glue update` after editing `ze-types.yang`, `ze-bgp-api.yang`, and `ze-command-meta-cmd.yang`, verified by `./le yang glue check` |
| `docs/architecture/api/update-syntax.md` | Wire Encoding Control | anchored to `ze-bgp-api.yang`; modified | "# Set wire bytes format" | departs | Updates `bgp plugin format base64` in the 4-value block (`hex`, `base64`, `parsed`, `full`) to document positional counted UPDATE lines beside JSON control events |
| `docs/architecture/api/wire-format.md` | Record Answer | anchored to `ze-bgp-api.yang` | "Two field shapes carry every value. A NUMBER is decimal digits closed by a space or by the end of the line, and it never carries a colon. A TEXT is decimal digits, a colon, then that many BYTES." | agrees | The `base64` `upd` line reuses these exact two positional field shapes (`counted number` + `<byte-len>:<bytes>`, with `0:` for empty) and `\n`-only line termination |
| `ai/doors/system-plugins.md` | Boundaries | door covering `ze-command-meta-cmd.yang`, `bridgerun/*`, `wrapper_test.go`, `internal/exabgp/bridge/*` | "Always-on code never imports a gated plugin." | agrees | All `internal/component/bgp/*` imports in `internal/exabgp/bridge` stay isolated in `//go:build ze_bgp` files (`bridge_raw_b64.go`, `bridge_raw_render.go`, `bridge_raw_render_emit.go`), paired with `//go:build !ze_bgp` stub |
| `docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md` | Child SA and the dataplane abstraction | anchored to `mux.go` | - | silent | Covers IKEv2 Child SA dataplane programming; unrelated to BGP event transport or `MuxConn.CallBatchRPC` |
| `docs/architecture/api/text-coverage.md` | Encoding Coverage | anchored to `text_update.go` | "writes parsed text plus raw text line" | agrees | Preserves `Text raw`, `Text parsed`, `Text full`, `JSON raw`, `JSON parsed`, and `JSON full` behaviors untouched; `FormatBase64` is an opt-in format |
| `docs/architecture/api/text-parser.md` | Original native routes in TEXT | anchored to `text_update.go`; modified | "With no attribute filter, the actual received TEXT path requests all attributes" | departs | `appendMessageTyped` short-circuits `content.Format == plugin.FormatBase64` ahead of `ApplyToUpdate` to emit the positional counted `upd` wire line instead of parsed TEXT; all non-`base64` TEXT paths and `bgp-rs`'s `textparse.Scanner` are unchanged |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `text_update.go` | - | silent | Covers route-server forwarding rails and cache eviction ordering; this spec only adds an opt-in event formatter beside `appendMessageTyped` |
| `docs/architecture/buffer-architecture.md` | Executive Summary | anchored to `text_update.go` | "One representation (bytes). Everything else is views/iterators." | agrees | `appendRawB64Update` encodes wire bytes directly without intermediate parsed structs, and `RenderRawUpdate` walks `WireUpdate`/`AttributesWire` directly into a `[]byte` buffer |
| `ai/doors/bgp-engine.md` | Boundaries | door covering `ze-bgp-api.yang`, `event_update_receipt.go`, `text_update.go`, `json_message.go`, `raw_b64.go`, `raw_b64_test.go` | "`WriteTo(buf, off) int` into a pooled buffer" | agrees | `appendRawB64Update` appends directly into the caller's stack-local `[4096]byte` scratch buffer without intermediate struct/slice allocations |
| `docs/architecture/exabgp-bridge.md` | The bridge translates in two directions | `// Design:` of `forms.go`, `forms_test.go`, `fleet.go`, `wrapper_test.go`, `bridge_negotiated.go`; anchored to `bridge_event.go`; modified | "Inside `ze`, the BGP event dispatcher formats each event once per distinct format across all matching external endpoints" | departs | `ze exabgp plugin` now requests `base64` positional counted UPDATE lines (instead of `full` bus JSON + `json.Unmarshal`) and renders hot unicast UPDATEs directly into a `[]byte` buffer; updates `exabgp-bridge.md` (including lines 107–113 and 845–855) in Phase 4 |
| `ai/doors/le-tooling.md` | Where it sits | door covering `test/plugin/exabgp-base64.ci` | "Functional `.ci` files live under `test/<suite>/` and run through `./le test <suite>` on the functional runner" | agrees | New end-to-end proof lands at `test/plugin/exabgp-base64.ci` and runs through `./le test plugin exabgp-base64` |
| `docs/exabgp/exabgp-differences.md` | Three UPDATE members come from the wire bytes | modified | "The AS_PATH is read with the session's AS number width" | departs | Live `ze exabgp plugin` endpoints now subscribe at `FormatBase64` (positional counted wire UPDATEs) in `ze_bgp` builds instead of `FormatFull`, while keeping identical ExaBGP output and `WireUpdateToExabgpJSON` fixture checks; updates `exabgp-differences.md` in Phase 4 |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-23 | "Every engine operation names the message it carries or `none` with a reason, and a plugin identity is authorized by a profile compiled from its filter" | agrees | Adds no new engine operation and changes no bus participant authorization or filter compilation |
| `docs/architecture/api/process-protocol.md` | Design decisions | decision of 2026-11-25 | "Engine-to-plugin event delivery never drops events on queue overflow; endpoints expect every subscribed network event losslessly" | agrees | `base64` and pooled `MuxConn.CallBatchRPC` change only wire representation and batch serialization, keeping engine-to-plugin delivery strictly lossless |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` rendering or config display |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | `ze exabgp plugin` keeps its exact Stage 1 bus registrations unchanged; only its Stage 5 `ready.format` changes |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not alter `send > to` semantics or filter evaluation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not alter `receive > from` semantics or peer scoping |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | `bridgerun.EventFormat` reads the compiled receive allow-list identically to choose `FormatBase64` vs `FormatParsed` |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not modify bus filter compilation or commit conflict checks |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Adds no bus arbitration or exclusive-role rules |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not alter `show config running \| implied` |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Adds no CLI command or presentation flag; `bgp plugin format base64` is an existing IPC session setter |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not touch pipe overrides or answer shapes |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not modify any pipe operation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not touch VPP or kernel dataplane compatibility checks |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not touch dataplane mismatch verification |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Does not touch firewall or VPP verifiers |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Does not touch dataplane selector naming |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Does not touch BGP egress next-hop rewriting or export chains |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not touch policy modify or export chain references |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not touch `policychain.Chain` or filter insertions |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch export-chain next-hop folding or raw override provenance |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch import/export next-hop validation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch route origination next-hop preservation |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch implied-filter publication |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-23 | "An ExaBGP script is an ordinary bus endpoint that runs the wrapper; the bridge config section and the internal bridge mode do not exist" | agrees | Keeps every ExaBGP script as an ordinary external bus endpoint running `ze exabgp plugin`; adds no bridge config section or internal bridge mode |
| `docs/architecture/exabgp-bridge.md` | Design decisions | decision of 2026-11-26 | "ExaBGP wrapper endpoints request `base64` (`plugin.FormatBase64`) UPDATE delivery at Stage 5 `ready` using Ze's positional counted wire grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`, zero key names, `0:` when absent) over pooled `WriteBatchFrame` `MuxConn` batching while keeping low-frequency control/session events as JSON; `ze exabgp plugin` renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes and falls back to the wire-backed `Event` renderer for non-unicast families and error cases" | agrees | Directly implements this owner ruling (amended 2026-11-26 to merge into `base64`) |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Inbound/outbound BGP `RawMessage` (carrying `RawBytes`, `WireUpdate`, `Direction`, `SourceEncodingContext()`, and `PeerInfo`) in `internal/component/bgp/server/events.go` (`onMessageReceived`, `onMessageBatchReceived`, `onMessageSent`).

### Transformation Path
1. At startup (`internal/plugins/exabgp/main_sdk.go`), `ze exabgp plugin` calls `bridgerun.EventFormat(filter, opts.Forms)`, which returns `plugin.FormatBase64` whenever `bridge.SupportsRawWire()` is true and `bgp:update` is allowed (or `forms.NeedsBytes()` with `bgp:update`), and passes it via `p.SetEventFormat(...)` in Stage 5 `ready`.
2. `plugin/server` (`registerReceiveDeclarations`) stores `FormatBase64` on `process.Process`, updating `FormatCacheKey()` (`"base64+json"`).
3. On each received or sent BGP UPDATE, `events.go` checks `fmtCache` once per distinct `FormatCacheKey()` and calls `format.AppendMessage` / `format.AppendSentMessage` with a stack-local `[4096]byte` scratch buffer.
4. `appendMessageTyped` short-circuits `content.Format == plugin.FormatBase64` before `isEORJSONUpdate` and `bgpfilter.ApplyToUpdate`, calling `appendRawB64Update` (`internal/component/bgp/format/raw_b64.go`) to write one fixed-position, quote-free ASCII line: `upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>\n`, where `<flags>` packs bit 0 (`sent`), bit 1 (`has-asn4`), bit 2 (`asn4`), bit 3 (`has-remote-as`), bit 4 (`ipv4/unicast` ADD-PATH), and bit 5 (`ipv6/unicast` ADD-PATH).
5. `process/delivery.go` batches events and calls `PluginConn.CallBatchRPC`, which invokes `MuxConn.CallBatchRPC` → pooled `rpc.WriteBatchFrame` (with `MaxMessageSize` splitting) over TLS — zero `map[string]any` batch marshaling and zero `\"` string escaping on the quote-free `upd` line.
6. `bridgerun.Fleet.Broadcast` checks whether the event starts with `"upd "` (single 4-byte prefix check):
   - JSON events (`{...}` — `state`, `negotiated`, `open`, `notification`, `keepalive`, `refresh`, `connected`, `fsm`, `signal`) take the existing `json.Unmarshal` + `bridge.ReadEvent` path unchanged, and `Sessions.Note` additionally pre-serializes each peer's `negotiated` ExaBGP JSON bytes once per session.
   - `upd ` lines take `bridge.ReadRawB64Update`: single-pass positional number/counted-text parse (zero key names, zero token slice) + `base64.StdEncoding.Decode` into a stack `[4096]byte` scratch (spilling cleanly to heap up to 65,516 bytes for RFC 8654 jumbo UPDATEs), then `Sessions.NoteRaw` (Q-E4 reconnect check on `conn-id`, cached `negotiated` bytes, bounded `(asn4, addPathSet)` -> `bgpctx.ContextID` lookup).
7. Inside `bridge.RenderRawUpdate` (`//go:build ze_bgp`):
   - **Hot path (IPv4/IPv6 unicast & EOR & `--form packets|consolidate`):** when `WireUpdate` contains only body IPv4/unicast and/or MP IPv4/IPv6 unicast (or EOR), parses cleanly without NLRI split errors, and carries only direct-supported attributes (all standard attributes plus unknown `attribute-0xCC-0xFF`), appends ExaBGP v4/v6 JSON (exact spacing and alphabetical key order) or ExaBGP text directly into a `[]byte` buffer — zero `map[string]any`, zero `json.Marshal`, cached `hostname`/`pid`/`ppid`, and pre-serialized `negotiated` bytes.
   - **Fallback path (non-unicast families & error/unsplit cases):** builds a wire-backed `Event` using `format.AppendMessage` (`EncodingJSON`, `FormatParsed`) + `registry.InProcessNLRIDecoder` + `Packet` bytes and renders through existing `Event.Lines`, preserving 100% parity across all 10 non-unicast families (`evpn`, `flowspec`, `vpls`, `vpn`, `labeled`, `rtc`, `bgp-ls`, `mup`, `mvpn`, `sr-policy`) and `unsplitOperation`/`undecodedNLRI` warnings.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `ze` daemon ↔ `ze exabgp plugin` subprocess | Newline-framed `MuxConn` `deliver-batch` via pooled `rpc.WriteBatchFrame` carrying quote-free positional counted `upd` lines for UPDATEs and JSON for low-frequency control/session events | Yes (code read) |
| Always-on `internal/exabgp/bridge` ↔ gated `internal/component/bgp/*` | `//go:build ze_bgp` files (`bridge_raw_b64.go`, `bridge_raw_render.go`) matching `bridge_wire_json.go`, paired with `//go:build !ze_bgp` stub (`bridge_raw_b64_stub.go`) | Yes (code read) |
| `pkg/plugin/rpc` public API ↔ `internal/component/plugin/ipc` | New exported `MuxConn.CallBatchRPC` matching `Conn.CallBatchRPC` signature and `MaxMessageSize` split semantics | Yes (code read) |

### Changed-Wire Census

**Search evidence:** Searched `internal/`, `pkg/`, `cmd/`, and `test/` across `.go`, `.ci`, `.conf`, `.py`, `.sh`, and `.yang` files for `FormatFull`, `FormatRaw`, `FormatHex`, `FormatBase64`, `SetEventFormat`, `EventFormat`, `bgp plugin format`, `update raw`, `CallBatchRPC`, and `WriteBatchFrame`. Positive controls: `internal/component/bgp/format/text_update.go:410-416` (`appendFromFilterResult`), `internal/component/bgp/format/json_message.go:51` (`JSONEncoder.Message`), `internal/plugins/exabgp/bridgerun/forms.go:134` (`EventFormat`), `internal/plugins/exabgp/bridgerun/fleet.go:215` (`Fleet.Broadcast`), `internal/component/plugin/ipc/rpc.go:131` (`PluginConn.CallBatchRPC`), `pkg/plugin/rpc/batch.go:33` (`WriteBatchFrame`).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| `base64` UPDATE event line (`upd ...`) | writer | `internal/component/bgp/format/raw_b64.go`, `appendRawB64Update` (dispatched from `text_update.go`, `appendMessageTyped`) | New format token `plugin.FormatBase64 = "base64"`; short-circuits before `bgpfilter.ApplyToUpdate` | New writer; existing `FormatParsed`/`FormatFull`/`FormatRaw`/`FormatHex`/`FormatSummary` writers preserved untouched |
| `base64` UPDATE event line (`upd ...`) | reader | `internal/exabgp/bridge/bridge_raw_b64.go`, `ReadRawB64Update` (called from `internal/plugins/exabgp/bridgerun/fleet.go`, `Fleet.Broadcast`) | Dispatched on `"upd "` prefix before `json.Unmarshal` | New reader; JSON `{...}` events continue through existing `ReadEvent` untouched |
| `base64` UPDATE event line (`upd ...`) | reader (third-party plugins opting in) | Out-of-tree plugins setting `ready.format = "base64"` or `bgp plugin format base64` | Documented in `docs/architecture/api/ipc_protocol.md` and `docs/architecture/api/json-format.md`; `updateEnvelopeCache.wrap` (`internal/component/bgp/server/event_update_receipt.go:57`) is extended to JSON-quote `output` when `subscriber.Encoding() == plugin.EncodingText \|\| subscriber.Format() == plugin.FormatBase64` so `SetEnvelope(true)` never drops `base64` UPDATEs | Opt-in contract documented beside Answer Protocol positional counted grammar; `event_update_receipt.go` updated in Phase 2 |
| Non-UPDATE JSON `raw.message` under `base64` | writer | `internal/component/bgp/format/json_message.go`, `JSONEncoder.Message` | Line 51 currently checks `fmtMode == plugin.FormatFull` only | Extend guard to `fmtMode == plugin.FormatFull \|\| fmtMode == plugin.FormatBase64`; unchanged for all other formats |
| Non-UPDATE JSON `raw.message` under `base64` | reader | `internal/exabgp/bridge/bridge_event_packets.go`, `readPacket` | Reads `payload["raw"]["message"]` on OPEN/NOTIFICATION/KEEPALIVE/REFRESH | Preserved unchanged; receives identical `"raw":{"message":"<hex>"}` |
| Stage 5 `ready.format` / `bgp plugin format` | writer | `internal/plugins/exabgp/bridgerun/forms.go`, `EventFormat`; `internal/plugins/exabgp/main_sdk.go`, `runSDKWithPlugin` | Calls `p.SetEventFormat(...)` in Stage 2/5 | Migrate to `plugin.FormatBase64` when `bridge.SupportsRawWire()` and `bgp:update` is allowed; preserve `FormatFull` for `!ze_bgp` and non-update packet-only filters, `FormatParsed` otherwise |
| Stage 5 `ready.format` / `bgp plugin format` | reader | `internal/component/plugin/server/bus_catalog.go`, `registerReceiveDeclarations`; `internal/plugins/meta/cmd/plugin_config.go`, `handleBgpPluginFormat`; `internal/component/config/yang/modules/ze-types.yang`, `typedef wire-format`; `internal/component/bgp/yang/ze-bgp-api.yang`, `rpc plugin-format`; `internal/plugins/meta/yang/ze-command-meta-cmd.yang`, `container format` | `registerReceiveDeclarations`, `handleBgpPluginFormat`, and `ze-types.yang` `typedef wire-format` already accept the closed 4-value set (`hex`, `base64`, `parsed`, `full`) | Update `base64` descriptions/help in `ze-types.yang`, `ze-bgp-api.yang`, and `ze-command-meta-cmd.yang`; verify `./le yang glue check`; preserve all existing values |
| `deliver-batch` RPC frame on `MuxConn` | writer | `pkg/plugin/rpc/mux.go`, `MuxConn.CallBatchRPC`; `internal/component/plugin/ipc/rpc.go`, `PluginConn.CallBatchRPC` | `PluginConn.CallBatchRPC` bypassed `WriteBatchFrame` on `mux != nil` | Migrate `mux != nil` path to pooled `WriteBatchFrame` with `MaxMessageSize` splitting; wire bytes remain identical `#<id> ze-plugin-callback:deliver-batch {"events":[...]}\n` |
| `deliver-batch` RPC frame on `MuxConn` | reader | `pkg/plugin/rpc/batch.go`, `ParseBatchEvents`; `pkg/plugin/sdk/sdk.go` batch handler; third-party external plugins | Reads `#<id> ze-plugin-callback:deliver-batch {"events":[...]}` | Preserved unchanged — byte-identical RPC wire framing |
| ExaBGP script stdin lines (4.2/5.0/6.0 JSON & text) | writer | `internal/exabgp/bridge/bridge_raw_render.go`, `RenderRawUpdate` + existing `Event.Lines` | Writes ExaBGP JSON (`{ "key": value }`) or text lines to script stdin | Preserved byte-identical contract proven by differential test across full ExaBGP fixture corpus |
| ExaBGP script stdin lines (4.2/5.0/6.0 JSON & text) | reader | Operator ExaBGP scripts (Python/shell/Go) and `test/exabgp-compat/`, `test/plugin/exabgp-*.ci` fixtures | Outside-repo scripts and repo Python/shell test scripts reading ExaBGP JSON/text | Preserved unchanged — zero script modifications |

### Integration Points
- `internal/component/plugin/types.go` (`FormatBase64 = "base64"` documentation updated)
- `internal/component/config/yang/modules/ze-types.yang` (`typedef wire-format` `enum base64` help), `internal/component/bgp/yang/ze-bgp-api.yang` (`rpc plugin-format`), `internal/plugins/meta/yang/ze-command-meta-cmd.yang` (`container format`)
- `internal/component/bgp/server/event_update_receipt.go` (`updateEnvelopeCache.wrap` JSON-quotes `base64` payloads when `SetEnvelope(true)` is enabled)
- `pkg/plugin/rpc/mux.go` (`MuxConn.CallBatchRPC`) and `internal/component/plugin/ipc/rpc.go` (`PluginConn.CallBatchRPC`)
- `internal/component/bgp/format/text_update.go` (`appendMessageTyped` short-circuit) and `internal/component/bgp/format/raw_b64.go` (`appendRawB64Update`)
- `internal/component/bgp/format/json_message.go` (`JSONEncoder.Message` attaches `raw.message` for `FormatBase64`)
- `internal/plugins/exabgp/bridgerun/forms.go` (`EventFormat`) and `fleet.go` (`Fleet.Broadcast`)
- `internal/exabgp/bridge/bridge_raw_b64.go`, `bridge_raw_b64_stub.go`, `bridge_raw_render.go`, `bridge_negotiated.go` (cached pre-serialized `negotiated` bytes + `NoteRaw`), `bridge_event.go` (cached `hostname`/`pid`/`ppid`)

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | Events still flow through `events.go` → `format.AppendMessage` → `process.Deliver` → `PluginConn.CallBatchRPC` → `MuxConn` → `sdk.Plugin.OnEvent` → `Fleet.Broadcast` |
| No unintended coupling (components stay isolated) | Yes | `internal/component/plugin` stays BGP-free (only adds opaque string constant `FormatBase64`); `internal/exabgp/bridge` isolates all `internal/component/bgp/*` imports in `//go:build ze_bgp` files matching `bridge_wire_json.go` (`./le arch tier check`) |
| No duplicated functionality (extends existing, does not recreate) | Yes | Extends Stage 5 `SetEventFormat`, Answer Protocol positional counted grammar (`counted number` + `<n>:<bytes>`), pooled `WriteBatchFrame`, `wireu.WireUpdate`, `attribute.AttributesWire`, and `registry.InProcessNLRIDecoder` |
| Zero-copy preserved where applicable (refs, not copies) | Yes | `appendRawB64Update` encodes directly from `msg.RawBytes` into stack scratch `[4096]byte`; `ReadRawB64Update` slices counted text directly out of the event string and decodes base64 into stack scratch `[4096]byte`; `RenderRawUpdate` appends directly into a `[]byte` buffer |
| Registration over hardcoding, outbound | Yes | Non-unicast family decoding uses `internal/component/plugin/registry.InProcessNLRIDecoder` registered by each `bgp-nlri-*` plugin; no new switch over NLRI plugins added to core/shared packages |
| Registration over hardcoding, inbound | Yes | Searched all switches/tables listing event format names (`plugin/types.go`, `plugin_config.go`, `text_update.go`, `json_message.go`, `bridgerun/forms.go`, `ze-plugin-engine.yang`); `FormatBase64` is a protocol wire-format constant rather than a pluggable feature registry |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | All ExaBGP UPDATE fields (all 10+ families, attributes, EOR, unsplit/undecoded NLRI warnings, `--form packets\|consolidate`) can be reconstructed identically from raw BGP UPDATE wire bytes + positional `upd` session scalars (`flags`, `local-as`, `remote-as`, `conn-id`, `peer`, `local`, `router-id`, `extra-addpath`) | `internal/exabgp/bridge/bridge_event.go`, `bridge_event_wire.go`, `bridge_wire_json.go`; grep verified `route-meta` is never read in `internal/exabgp/bridge/` | A field depends on daemon-only state not in wire bytes or `upd` scalars | Differential test comparing `FormatFull` JSON vs `FormatBase64` across full ExaBGP fixture corpus | confirmed (`TestRawB64Differential_MatchesFormatFull` 100% byte-identical) |
| A-2 | Keeping low-frequency non-UPDATE events (`state`, `negotiated`, `open`, `notification`, `keepalive`, `refresh`, `connected`, `fsm`, `signal`, <10/session) as JSON while delivering `bgp:update` in positional counted `base64` captures virtually 100% of Q-E7 speedup | Owner decision (2026-11-25, `docs/architecture/exabgp-bridge.md`) and Q-E7 profile | Non-UPDATE storm workload remains JSON-bound | Confirmed by owner at Design gate | confirmed |
| A-3 | `ze exabgp plugin` runs in a binary compiled with `ze_bgp` and `plugin/all` (registering all 10 compiled `InProcessNLRIDecoder`s); non-`ze_bgp` builds cleanly compile with `SupportsRawWire() == false` | `cmd/ze/main.go`, `feature-gates.txt`, `internal/component/bgp/plugins/nlri/*/register.go` | Non-`ze_bgp` build fails to compile or loses ExaBGP support | `//go:build !ze_bgp` stub + `go build` without `ze_bgp` check | confirmed (`./le arch tier check` + untagged `go test` PASS) |
| A-4 | Adding `MuxConn.CallBatchRPC` using pooled `rpc.WriteBatchFrame` preserves exact `deliver-batch` framing and `MaxMessageSize` batch splitting for all external plugins | `pkg/plugin/rpc/batch.go`, `pkg/plugin/rpc/conn.go:586-685`, `pkg/plugin/rpc/mux.go:270-310` | Oversized batch exceeds `MaxMessageSize` or corrupts pending response correlation on concurrent `MuxConn` calls | Unit tests on `MuxConn.CallBatchRPC` (normal batch, oversized split, concurrent `CallRPC` + `CallBatchRPC`, write error) | confirmed (`TestMuxConn_CallBatchRPC_PooledAndSplit` `-race` PASS) |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Subtle ExaBGP 4.2/5.0/6.0 formatting drift between JSON-read path and direct wire-to-buffer renderer (e.g. withdraw-with-NEXT_HOP, empty `AS_PATH`, `AS4_AGGREGATOR`, FlowSpec/extended-community spellings, unknown attribute `0xCC-0xFF` flags, JSON key order/spacing) | Differential unit test or `./le test functional exabgp` diff | Strict whitelist on the direct `[]byte` fast path (falling back to wire-backed `Event` renderer whenever an attribute or family is outside the proven direct set), plus differential test asserting byte-identical output (after `DropVolatile`) across every ExaBGP fixture |
| R-2 | Outbound rendering still allocates excessively if hot unicast UPDATEs unexpectedly hit the fallback path | `testing.AllocsPerRun` regression or Q-E7 20-script Mac measurement | Explicit alloc-bound unit test on hot IPv4/IPv6 unicast JSON & text paths verifying direct path is taken and zero `map[string]any` / `json.Marshal` occurs, plus Mac Q-E7 benchmark |
| R-3 | Out-of-tree foreign external NLRI decoder plugin (not compiled into `ze`) cannot be called via `registry.InProcessNLRIDecoder` inside `ze exabgp plugin` | `undecodedNLRI` warning on a custom foreign family | Documented Known Limitation; all 10 shipped NLRI families are compiled into `ze` (`plugin/all`) |
| R-4 | Exhausting `bgpctx.Registry` by calling `bgpctx.Registry.Register` on every received `base64` UPDATE | `register session context` error after thousands of UPDATEs | Bounded mutex-guarded cache in `internal/exabgp/bridge` keyed by `(asn4 bool, addPathKey string)` reusing registered `ContextID`s across all UPDATEs; unit test verifies `ContextID` reuse over 10,000 UPDATEs |
| R-5 | Malformed/truncated `upd` line or oversized RFC 8654 jumbo UPDATE (> 4,096 bytes) causes panic, truncation, or forged script line | Fuzz / malformed input unit test failure | Strict bounds check before every `<n>:<bytes>` slice, exact trailing-byte rejection, RFC 8654 body bound (`<= 65,516` bytes), warning log on refusal, and stack `[4096]byte` scratch that spills cleanly to heap up to 65,516 bytes |
| R-6 | Concurrent `MuxConn.CallBatchRPC` write interleaving or leaked `pending` entry on split-batch error | `go test -race ./pkg/plugin/rpc/...` | Hold `m.conn`'s write path (`writeAppended` / `WriteRaw` under `conn`'s write mutex) and clean up `m.pending.Delete(idStr)` on every error/cancel path |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | ExaBGP scripts receive subtly misformatted JSON/text lines, or external TLS plugins fail to receive batched events if `MuxConn.CallBatchRPC` regresses |
| How is it reverted? | Single commit revert; external third-party plugin default (`encoding json` + `format parsed`) is unchanged |
| Who else touches this path? | `pkg/plugin/rpc`, `internal/component/plugin/ipc`, `internal/component/bgp/format`, `internal/plugins/exabgp`, `internal/exabgp/bridge` (Spec 1 `spec-bus-observer-delivery-decoupling.md` modifies `process/delivery.go` and does not conflict with `raw_b64.go` or `bridge`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze exabgp plugin` Stage 5 `ready` (`main_sdk.go` → `bridgerun.EventFormat`) | → | `Server.registerReceiveDeclarations` sets `FormatBase64` → `events.go` calls `format.AppendMessage` → `appendRawB64Update` → `MuxConn.CallBatchRPC` → `Fleet.Broadcast` → `ReadRawB64Update` → `RenderRawUpdate` | `TestExabgpRawB64EndToEnd` (`internal/plugins/exabgp/wrapper_test.go`) and `test/plugin/exabgp-base64.ci` |
| `bgp plugin format base64` (`plugin_config.go`) | → | `handleBgpPluginFormat` sets `Process.SetFormat(plugin.FormatBase64)` | `TestHandleBgpPluginFormatBase64` (`internal/plugins/meta/cmd/plugin_config_test.go`) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | `format.AppendMessage` / `AppendSentMessage` called with `plugin.FormatBase64` on received/sent UPDATE (IPv4/IPv6 unicast, ADD-PATH unicast & non-unicast, EOR, dynamic peer without remote AS, absent local IP/router-id/conn-id, and malformed UPDATE body), and `updateEnvelopeCache.wrap` called with `FormatBase64` + `SetEnvelope(true)` | Emits one `\n`-terminated quote-free positional counted line `upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>\n` with exact flag bits, `0:` for absent counted fields, exact base64 of `msg.RawBytes`, and zero calls to `bgpfilter.ApplyToUpdate`; when `SetEnvelope(true)` is enabled, `updateEnvelopeCache.wrap` JSON-quotes the `upd` line as `payload` rather than failing `json.Marshal` or dropping the UPDATE | Existing `FormatRaw`, `FormatHex`, `FormatParsed`, `FormatFull`, and `FormatSummary` outputs remain unchanged for all existing callers and tests |
| AC-2 | `JSONEncoder.Message` called with `plugin.FormatBase64` on OPEN, NOTIFICATION, KEEPALIVE, or ROUTE-REFRESH | Emits the existing JSON event with `"raw":{"message":"<hex>"}` attached, identical to `FormatFull` | Other formats (`FormatParsed`, `FormatRaw`, `FormatHex`) preserve their existing `raw.message` omission |
| AC-3 | `PluginConn.CallBatchRPC` invoked on a `MuxConn`-backed external TLS plugin with ordinary batches and batches exceeding `MaxMessageSize` | Writes `#<id> ze-plugin-callback:deliver-batch {"events":[...]}\n` through `MuxConn.CallBatchRPC` using pooled `rpc.WriteBatchFrame`, splitting batches that exceed `MaxMessageSize`, and delivers every event in FIFO order without `map[string]any` batch marshaling | DirectBridge in-process plugins continue taking their in-process callback path |
| AC-4 | `ze exabgp plugin` starts in a `ze_bgp` build with `bgp:update` allowed (or unscoped filter) | `bridgerun.EventFormat` selects `plugin.FormatBase64` and sends it in Stage 5 `ready`; non-`ze_bgp` builds select `plugin.FormatFull`; update-denied non-packet configs keep `plugin.FormatParsed`; `bgp plugin format base64` sets `FormatBase64` | Standalone stdin/stdout mode and `bridge.ReadEvent` JSON path remain intact for tests and hand-run scripts |
| AC-5 | `Fleet.Broadcast` receives `base64` `upd` lines across IPv4/IPv6 unicast (received & sent), ADD-PATH, EOR, empty AS_PATH, withdraw-with-NEXT_HOP, all standard attributes, unknown attributes (`attribute-0xCC-0xFF`), all 4 community types, `--form parsed\|packets\|consolidate`, v4/v6 JSON, and ExaBGP text | Renders output byte-identical (after `DropVolatile`) to the legacy `FormatFull` JSON → `ReadEvent` → `Event.Lines` path | Volatile wall-clock envelope field (`time`) differs per call; `DropVolatile` matches upstream ExaBGP comparison rules |
| AC-6 | `Fleet.Broadcast` receives `base64` `upd` lines for all 10 compiled non-unicast families (`evpn`, `flowspec`, `vpls`, `vpn`, `labeled`, `rtc`, `bgp-ls`, `mup`, `mvpn`, `sr-policy`) and unsplit/undecoded NLRI error cases | Decodes via wire-backed `Event` + `registry.InProcessNLRIDecoder` and emits byte-identical ExaBGP JSON/text output and identical `unsplitOperation`/`undecodedNLRI` warnings | Out-of-tree foreign external NLRI decoder plugins not compiled into `ze` are out of scope (none exist in Ze) |
| AC-7 | Hot IPv4/IPv6 unicast `base64` UPDATE rendered via `RenderRawUpdate` (both ExaBGP JSON and ExaBGP text, with and without cached `negotiated` bytes) | Writes byte-identical ExaBGP JSON and text lines directly into the `[]byte` buffer and reuses cached `ContextID` across 10,000 UPDATEs; executes without `json.Unmarshal`, without `map[string]any` / `json.Marshal`, and without per-UPDATE `bgpctx.Registry.Register` calls | Fallback path for non-unicast families and split/decode error cases still uses the wire-backed `Event` `map[string]any` path |
| AC-8 | Session reconnect (`conn-id` changes) without `bgp:state`, or `bgp:negotiated` excluded from `receive > allow` (Q-E4) | `Sessions.NoteRaw` invalidates stale cached negotiated block and pre-serialized JSON bytes on `conn-id` mismatch and decodes AS_PATH / ADD-PATH from the `upd` line's `flags` and `extra-addpath` | Synthetic inputs with `conn-id == 0` preserve backward-compatible non-invalidating behavior |
| AC-9 | Malformed `upd` line (wrong kind, bad number, unclosed/out-of-bounds `<n>:`, trailing garbage, invalid base64, or decoded body > 65,516 bytes) or valid RFC 8654 jumbo UPDATE (4,097..65,516 bytes) | Malformed lines are refused with a warning log and produce zero script output; valid jumbo UPDATEs up to 65,516 bytes decode and render completely without truncation | None — every malformed line is rejected and every valid RFC 8654 UPDATE renders completely |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs a migrated ExaBGP script (`ze exabgp plugin --encoder json\|text`) receiving live IPv4/IPv6 unicast, ADD-PATH, EOR, and non-unicast UPDATEs plus `--form packets\|consolidate` | Stage 5 `ready` requests `base64` → `events.go` formats positional `upd` line → `MuxConn.CallBatchRPC` sends pooled `WriteBatchFrame` → `Fleet.Broadcast` parses `upd` → direct buffer render (unicast) or wire-backed `Event` (non-unicast) → byte-identical ExaBGP lines on script stdin | `test/plugin/exabgp-base64.ci` + `./le test functional exabgp` |
| 2 | Runs 1, 5, and 20 `ze exabgp plugin` endpoints on a full-table feed (Mac Q-E7) | `ze` formats `upd` line once (`fmtCache`) without `FormatFull` JSON, batches via pooled `WriteBatchFrame`, each wrapper decodes wire directly into `[]byte` ExaBGP JSON without `json.Unmarshal` or `map[string]any` | Mac Q-E7 re-measurement after Spec 1 + Spec 2 implementation |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAppendRawB64Update_FieldsAndFlags` | `internal/component/bgp/format/raw_b64_test.go` | AC-1: positional `upd` encoding for received/sent, `asn4`, dynamic peer (`has-remote-as=0`), absent `local`/`router-id`/`conn-id` (`0:`), IPv4/IPv6 unicast ADD-PATH flag bits, non-unicast `extra-addpath`, EOR, malformed body, zero quotes/backslashes | pass |
| `TestUpdateEnvelopeWrap_RawB64QuotesPayload` | `internal/component/bgp/server/event_update_receipt_test.go` | AC-1: `updateEnvelopeCache.wrap` JSON-quotes `base64` `upd` line as string `payload` when `SetEnvelope(true)` is enabled (zero dropped UPDATEs) | pass |
| `TestJSONEncoderMessage_RawB64AttachesRaw` | `internal/component/bgp/format/json_message_test.go` | AC-2: OPEN, NOTIFICATION, KEEPALIVE, ROUTE-REFRESH carry `"raw":{"message":"<hex>"}` under `FormatBase64` | pass |
| `TestMuxConn_CallBatchRPC_PooledAndSplit` | `pkg/plugin/rpc/mux_batch_test.go` | AC-3: `MuxConn.CallBatchRPC` writes identical `deliver-batch` frames via `WriteBatchFrame`, splits oversized batches at `MaxMessageSize`, and stays race-free beside concurrent `CallRPC` | pass |
| `TestHandleBgpPluginFormatBase64` | `internal/plugins/meta/cmd/plugin_config_test.go` | AC-4: `bgp plugin format base64` sets `plugin.FormatBase64` | pass |
| `TestEventFormat_SelectsRawB64` | `internal/plugins/exabgp/bridgerun/forms_test.go` | AC-4: `EventFormat` selects `FormatBase64` when `SupportsRawWire()` and `bgp:update` allowed, `FormatParsed` when update denied and no packet form, `FormatFull` for non-update packet-only filters | pass |
| `TestReadRawB64Update_RoundTripAndMalformed` | `internal/exabgp/bridge/bridge_raw_b64_test.go` | AC-1, AC-9: round-trip parse of every field, jumbo RFC 8654 UPDATE (65,516 bytes), and rejection of bad kind, bad count, out-of-bounds slice, trailing bytes, bad base64, and > 65,516 bytes (plus `FuzzReadRawB64Update`) | pass |
| `TestRawB64Differential_MatchesFormatFull` | `internal/exabgp/bridge/bridge_raw_render_test.go` | AC-5, AC-6: differential byte-identical comparison (after `DropVolatile`) of `FormatFull` JSON vs `FormatBase64` across v4 JSON, v6 JSON, ExaBGP text, `--form parsed\|packets\|consolidate`, IPv4/IPv6 unicast, ADD-PATH, EOR, empty AS_PATH, withdraw-with-NEXT_HOP, all 4 community types, unknown attributes, and all 10 non-unicast families + unsplit/undecoded error logs | pass |
| `TestRawB64DirectRender_HotPathNoMapAndContextReuse` | `internal/exabgp/bridge/bridge_raw_render_test.go` | AC-7: proves direct unicast fast path executes without `map[string]any`/`json.Marshal`, keeps allocations bounded, and reuses cached `bgpctx.ContextID` over 10,000 UPDATEs | pass |
| `TestSessionsNoteRaw_ReconnectAndNoNegotiated` | `internal/exabgp/bridge/bridge_raw_b64_test.go` | AC-8: Q-E4 reconnect invalidation on `conn-id` change without `bgp:state`, pre-serialized `negotiated` bytes, and `asn4`/`addpath` decode when `bgp:negotiated` is excluded | pass |
| `TestExabgpRawB64EndToEnd` | `internal/plugins/exabgp/wrapper_raw_b64_test.go` | Wiring: full `ze` event dispatch → `FormatBase64` → `Fleet.Broadcast` → script stdin verification (`//go:build ze_bgp`) | pass |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `upd` decoded UPDATE body bytes (RFC 8654) | `0..65516` | `65516` | N-A (unsigned length; `0` valid empty body) | `65517` (rejected with warning) |
| `upd` `<flags>` bitmask | `0..63` (6 defined bits) | `63` | negative / non-digit (rejected) | unknown high bits `>= 64` (rejected with warning) |
| `upd` `<local-as>` / `<remote-as>` | `0..4294967295` (`uint32`) | `4294967295` | negative / non-digit (rejected) | `4294967296` (rejected with warning) |
| `upd` `<n>:<bytes>` counted text length | `0..len(rest)` | `len(rest)` | negative / missing `:` (rejected) | `len(rest)+1` (rejected before slicing) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `exabgp-base64` | `test/plugin/exabgp-base64.ci` | Real `ze` daemon with `ze exabgp plugin` endpoints (`--encoder json` and `--encoder text`, including `--form receive-consolidate` and ADD-PATH) receiving live BGP UPDATEs, EOR, OPEN, and state transitions via `base64` and asserting exact ExaBGP output | pass (80/80 stress-repro + live pass) |

### Interop Tests (Scope: protocol)
N-A — `Scope` is `plugin`; no BGP peer wire encoding changes (only internal daemon-to-plugin IPC event transport). Existing `./le test functional exabgp` suite exercises the full ExaBGP compatibility corpus end-to-end.

## Files to Modify
- `internal/component/plugin/types.go` - document `FormatBase64 = "base64"` as positional counted base64 UPDATE lines beside JSON control events
- `internal/component/config/yang/modules/ze-types.yang` - update `enum base64` help in `typedef wire-format`
- `internal/component/bgp/yang/ze-bgp-api.yang` - document `base64` positional UPDATE lines in `rpc plugin-format` description
- `internal/plugins/meta/yang/ze-command-meta-cmd.yang` - document `base64` positional UPDATE lines in `container format` description
- `internal/component/bgp/server/event_update_receipt.go` - JSON-quote `FormatBase64` `upd` lines in `updateEnvelopeCache.wrap` when `SetEnvelope(true)` is set
- `pkg/plugin/rpc/conn.go` - add internal `writeBatchWithDeadline` helper reusing pooled `WriteBatchFrame` under `c.writeMu`
- `pkg/plugin/rpc/mux.go` - add `MuxConn.CallBatchRPC` using pooled `WriteBatchFrame` with `MaxMessageSize` splitting
- `internal/component/plugin/ipc/rpc.go` - route `PluginConn.CallBatchRPC` through `pc.mux.CallBatchRPC` when `pc.mux != nil`
- `internal/component/bgp/format/text_update.go` - dispatch `content.Format == plugin.FormatBase64` to `appendRawB64Update` at the top of `appendMessageTyped`
- `internal/component/bgp/format/json_message.go` - attach `"raw":{"message":"<hex>"}` when `fmtMode == plugin.FormatBase64`
- `internal/component/bgp/format/json_message_test.go` - add `TestJSONEncoderMessage_RawB64AttachesRaw`
- `internal/plugins/exabgp/bridgerun/forms.go` - select `plugin.FormatBase64` when `bridge.SupportsRawWire()` and `bgp:update` is allowed
- `internal/plugins/exabgp/bridgerun/forms_test.go` - update `EventFormat` expectations for `FormatBase64` under `ze_bgp` (preserving `FormatFull` for `!ze_bgp` and non-update packet-only filters)
- `internal/plugins/exabgp/bridgerun/fleet.go` - dispatch `"upd "` events to `bridge.ReadRawB64Update` + `bridge.RenderRawUpdate` in `Fleet.Broadcast`
- `internal/plugins/exabgp/wrapper_test.go` - update `TestWrapperScopeFromFilter` assertion for `FormatBase64`
- `internal/exabgp/bridge/bridge_negotiated.go` - cache pre-serialized ExaBGP-spaced `negotiated` JSON bytes per peer and add `Sessions.NoteRaw`
- `internal/exabgp/bridge/bridge_event.go` - cache process `hostname`, `pid`, `ppid` once for envelope rendering and always clone `attrObj` in `normalizeOutgoingAttributes`
- `docs/architecture/api/architecture.md` - update `bgp plugin format base64` row in BGP Plugin Configuration table
- `docs/architecture/api/commands.md` - update `bgp plugin format base64` line in BGP Plugin Configuration
- `docs/architecture/api/ipc_protocol.md` - document positional counted `base64` UPDATE event line, `bgp plugin format base64`, and `MuxConn.CallBatchRPC`
- `docs/architecture/api/json-format.md` - document `format=base64` behavior (positional UPDATE line + `raw.message` on non-UPDATE JSON) and updated ExaBGP bridge subscription format
- `docs/architecture/api/process-protocol.md` - document `base64` format option on Stage 5 `ready` and enveloped `base64` quoting
- `docs/architecture/api/text-parser.md` - document opt-in `format=base64` short-circuit ahead of `ApplyToUpdate` beside `appendMessageTyped` source anchor
- `docs/architecture/api/update-syntax.md` - update `bgp plugin format base64` line in Wire Encoding Control
- `docs/architecture/core-design.md` - note opt-in `base64` positional counted UPDATE delivery beside JSON events in Section 6 (`JSON Events (Engine → Plugin)`)
- `docs/architecture/exabgp-bridge.md` - document `base64` transport, direct unicast buffer renderer, and updated `EventFormat` behavior
- `docs/exabgp/exabgp-differences.md` - update "Three UPDATE members come from the wire bytes" to reflect `FormatBase64` for live wrapper UPDATEs

## Files to Create
- `internal/plugins/meta/cmd/plugin_config_test.go` - unit test `TestHandleBgpPluginFormatBase64`
- `internal/component/bgp/format/raw_b64.go` - `appendRawB64Update` positional counted `upd` formatter
- `internal/component/bgp/format/raw_b64_test.go` - unit and boundary tests for `appendRawB64Update`
- `internal/component/bgp/server/event_update_receipt_test.go` - unit test `TestUpdateEnvelopeWrap_RawB64QuotesPayload`
- `pkg/plugin/rpc/mux_batch_test.go` - unit and race tests for `MuxConn.CallBatchRPC`
- `internal/exabgp/bridge/bridge_raw_b64.go` (`//go:build ze_bgp`) - `SupportsRawWire`, `ReadRawB64Update`, bounded `EncodingContext` cache, wire-backed `Event` fallback
- `internal/exabgp/bridge/bridge_raw_b64_stub.go` (`//go:build !ze_bgp`) - non-`ze_bgp` stub (`SupportsRawWire() == false`)
- `internal/exabgp/bridge/bridge_raw_b64_test.go` - parser round-trip, boundary, malformed input, fuzz target, and `Sessions.NoteRaw` tests
- `internal/exabgp/bridge/bridge_raw_render.go` (`//go:build ze_bgp`) - direct unicast plan builder and attribute/NLRI validation
- `internal/exabgp/bridge/bridge_raw_render_emit.go` (`//go:build ze_bgp`) - direct `[]byte` ExaBGP v4/v6 JSON & text buffer emitters for hot IPv4/IPv6 unicast UPDATEs + EOR + `--form packets|consolidate`
- `internal/exabgp/bridge/bridge_raw_render_test.go` (`//go:build ze_bgp`) - differential byte-identity test across full ExaBGP corpus + hot-path alloc/context-reuse tests
- `internal/plugins/exabgp/wrapper_raw_b64_test.go` (`//go:build ze_bgp`) - wiring test `TestExabgpRawB64EndToEnd`
- `test/plugin/exabgp-base64.ci` - end-to-end functional test proving `base64` delivery with real `ze` + `ze exabgp plugin`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/yang/modules/ze-types.yang` (`typedef wire-format` `enum base64`), `internal/component/bgp/yang/ze-bgp-api.yang` (`rpc plugin-format`), `internal/plugins/meta/yang/ze-command-meta-cmd.yang` (`container format`), verified by `./le yang glue check` |
| YANG validation constraints | Yes | `internal/component/config/yang/modules/ze-types.yang` retains the closed 4-value `typedef wire-format` (`hex`, `base64`, `parsed`, `full`), matching `handleBgpPluginFormat` (`internal/plugins/meta/cmd/plugin_config.go`) |
| YANG custom validators | N-A | No new config leaf or `ze:validate` hook |
| CLI commands/flags | N-A | No new CLI command or flag (`ze exabgp plugin` opts in automatically at Stage 5) |
| CLI grammar (keyword before value) | N-A | Existing `bgp plugin format <value>` grammar unchanged; only adds `base64` value |
| Editor autocomplete | N-A | No new config editor node |
| Functional test for new RPC/API | Yes | `test/plugin/exabgp-base64.ci` |
| Pipe completeness | N-A | Event transport only; no CLI output command added |
| Env var registration | N-A | No new `ze.*` env var |
| Doctor check for runtime dependencies | N-A | Adds no file path, socket, listen port, kernel module, binary, or certificate |
| Doctor code per problem class | N-A | Malformed `base64` lines occur only inside `ze exabgp plugin` subprocess (logged via `exabgp-bridge` `Warn`, matching existing `invalid JSON event` warning in `fleet.go:218`, which carries no daemon doctor code) |
| Prometheus counters/metrics | N-A | Transport-format optimization only; delivery queue depth and backpressure metrics belong to Spec 1 (`spec-bus-observer-delivery-decoupling.md`) |
| BGP family surface (new SAFI / capability / attribute) | N-A | Adds no new SAFI, capability, or attribute; preserves all existing families |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Internal daemon-to-wrapper IPC optimization; user-facing ExaBGP feature scope unchanged |
| 2 | Config syntax changed? | No | Zero config changes; `ze exabgp plugin` negotiates `base64` automatically at Stage 5 |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` (add `base64` to `bgp plugin format` table) |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/ipc_protocol.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md`, `docs/architecture/api/update-syntax.md` (`bgp plugin format base64`, positional `upd` line, `MuxConn.CallBatchRPC`) |
| 5 | Plugin added/changed? | No | No new plugin; `ze exabgp plugin` usage in `docs/guide/plugins.md` unchanged |
| 6 | Has a user guide page? | No | Operator-transparent transport optimization |
| 7 | Wire format changed? | No | BGP peer wire format under `docs/architecture/wire/*.md` is unchanged |
| 8 | Plugin SDK/protocol changed? | Yes | `docs/architecture/api/process-protocol.md` (Stage 5 `ready.format` `base64`) and `docs/architecture/api/json-format.md` (`format=base64` behavior) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC conformance row changes |
| 10 | Test infrastructure changed? | No | Uses existing `.ci` runner |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` unaffected until post-implementation Q-E7 benchmark update |
| 12 | Internal architecture changed? | Yes | `docs/architecture/exabgp-bridge.md`, `docs/architecture/core-design.md`, and `docs/exabgp/exabgp-differences.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata keys added or changed |
| 14 | Prometheus counters added/changed? | No | No metrics added or changed |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry entry changed |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived via `./le spec citation anchors`: modified docs (`docs/architecture/api/architecture.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/api/json-format.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/core-design.md`, `docs/architecture/exabgp-bridge.md`, `docs/exabgp/exabgp-differences.md`, `docs/guide/command-reference.md`) updated in Phase 4; remaining advisory anchor pages unaffected because existing symbols, JSON defaults, and non-`base64` formats remain intact |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/api/ipc_protocol.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/architecture.md`, `docs/architecture/api/update-syntax.md`, `docs/guide/command-reference.md`, and `docs/architecture/api/json-format.md` |

### Discovery (Mechanical Checklist — `ai/rules/repo-maintenance.md`)
- **Where an agent looks first:** `ai/INDEX.md` already maps `ExaBGP` → `docs/architecture/exabgp-bridge.md`, `IPC, wire format, muxconn` → `docs/architecture/api/ipc_protocol.md`, `process-protocol.md`, and `JSON, event format` → `docs/architecture/api/json-format.md`.
- **Rule preventing regression:** `TestRawB64Differential_MatchesFormatFull` guarantees byte-identical output between legacy `FormatFull` JSON and `FormatBase64` across all ExaBGP fixtures; `TestRawB64DirectRender_HotPathNoMapAndContextReuse` guards hot-path allocations and `bgpctx.Registry` reuse; `./le arch tier check` enforces `//go:build ze_bgp` isolation.
- **Registry/inventory preventing drift:** `plugin.FormatBase64` is defined once in `internal/component/plugin/types.go` and referenced by symbol across `text_update.go`, `json_message.go`, `plugin_config.go`, and `bridgerun/forms.go`.
- **Verification proving it:** Scoped `go test -race -tags ze_bgp` on `./pkg/plugin/rpc/...`, `./internal/component/plugin/ipc/...`, `./internal/component/bgp/format/...`, `./internal/plugins/meta/cmd/...`, `./internal/plugins/exabgp/...`, `./internal/exabgp/bridge/...`, `./le arch tier check`, `./le doc check all`, `./le test functional exabgp`, and `test/plugin/exabgp-base64.ci`.

## Design presentation

### What changes

`ze exabgp plugin` automatically requests a new `base64` event format at Stage 5 startup with zero operator configuration changes. On BGP UPDATEs (received and sent), `ze` skips attribute/NLRI parsing and JSON construction and instead emits one fixed-position, zero-key-name, quote-free ASCII line following Ze's Answer Protocol counted grammar (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`), batched through pooled `rpc.WriteBatchFrame` on `MuxConn`. Low-frequency control/session events (`state`, `negotiated`, `open`, `notification`, `keepalive`, `refresh`, `connected`, `fsm`, `signal`) stay on the existing JSON path unchanged (with `raw.message` still attached for `--form packets|consolidate`). Inside `ze exabgp plugin`, hot IPv4/IPv6 unicast UPDATEs decode directly from the wire bytes and serialize ExaBGP 4.2/5.0/6.0 JSON or text straight into a `[]byte` buffer with zero `json.Unmarshal` and zero `map[string]any` reflection, while non-unicast families and split/decode error cases reuse the wire-backed `Event` renderer for 100% parity.

### Before and after

1. **Daemon-to-wrapper UPDATE wire payload (`ze` → `ze exabgp plugin` inside `deliver-batch`):**
   - **Before (`format=full`):** ~1,500–2,000 bytes of escaped JSON containing both parsed `update.attr`/`update.nlri` AND hex `raw.attributes`/`raw.nlri`/`raw.withdrawn`/`raw.update`, marshaled via `map[string]any` on `MuxConn`, then unmarshaled into `map[string]any` and re-parsed off hex inside every wrapper process.
   - **After (`format=base64`):** one quote-free positional counted line — `upd 15 65000 65001 42 8:10.0.0.1 8:10.0.0.2 7:1.1.1.1 0: 48:<48 base64 bytes>` — written via pooled `rpc.WriteBatchFrame` with zero `\"` escaping, ~3–4× smaller on the wire, parsed by fused integer loops and counted byte slicing in ~15 ns.
2. **Operator configuration and ExaBGP script stdin output:**
   - **Before:** operator configures `bus { endpoint <name> { external { run "ze exabgp plugin ..."; } } }` and script receives ExaBGP 4.2/5.0/6.0 JSON or text lines.
   - **After:** exact same operator configuration and byte-identical ExaBGP 4.2/5.0/6.0 JSON or text lines on script stdin, with lower daemon CPU, lower TLS bandwidth, and lower per-wrapper CPU/GC pressure on multi-script full-table feeds.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Custom binary `MuxConn` framing | Breaks newline-framed `MuxConn` / `deliver-batch` line protocol for third-party tooling and adds framing complexity for minimal gain over quote-free positional base64 inside pooled `WriteBatchFrame` |
| Keyword-value raw text line (`base64 update received peer ... local ...`) | Puts ~10 key names on the wire per UPDATE and forces keyword scanning/dispatch instead of Ze's faster Answer Protocol positional counted grammar |
| Separate 5th `raw-b64` enum beside unimplemented `base64` | Leaves a dead/unimplemented `base64` enum beside a near-synonym `raw-b64` in `ze-types.yang` and `bgp plugin format`; Thomas chose Option 1 (2026-11-26) to merge into `base64` |
| Transport-only optimization (keeping outbound `map[string]any` + `marshalExabgpJSON`) | Still leaves per-UPDATE Go map allocation and `json.Marshal` reflection inside each of the 20 wrapper processes |
| Raw-in-text for all 5 BGP wire message types (`open`, `notification`, `keepalive`, `refresh`, `update`) | Non-UPDATEs are <10 events per session lifetime (~0% of Q-E7 CPU) and would require three extra wire decoders in `bridge` |

### Documents departed from

1. **`docs/architecture/core-design.md` (`JSON Events (Engine → Plugin)`):**
   - **Before:** "When a process binding requests JSON output, the engine sends formatted BGP event payloads via `deliver-event` or `deliver-batch`:" (all events as JSON objects).
   - **After:** Clarifies that while default `encoding=json` emits JSON objects for every event, an opt-in `format=base64` subscriber receives BGP UPDATEs as quote-free positional counted `upd` lines inside `deliver-batch` while keeping low-frequency control/session events as JSON objects.
2. **`docs/architecture/api/json-format.md` (`With Raw Wire Bytes (format=full)`):**
   - **Before:** "a subscriber of any other format is written no `raw` member." and "The ExaBGP bridge subscribes `full` whenever a script receives an UPDATE or a packet form".
   - **After:** Documents that `format=base64` also attaches `"raw":{"message":"<hex>"}` on OPEN/NOTIFICATION/KEEPALIVE/ROUTE-REFRESH while sending UPDATEs as positional counted `upd` lines, and that `ze exabgp plugin` subscribes `base64` in `ze_bgp` builds (keeping `full` for non-`ze_bgp` builds and non-update packet-only filters).
3. **`docs/architecture/exabgp-bridge.md` (`The bridge translates in two directions` and `--form` section):**
   - **Before:** States the BGP dispatcher formats Ze's bus JSON, each wrapper runs `json.Unmarshal`, and the wrapper requests `full` event format whenever its filter admits UPDATEs.
   - **After:** Documents that `ze exabgp plugin` requests `base64` positional counted `upd` lines for UPDATEs (with JSON kept for low-frequency control/session events), renders hot IPv4/IPv6 unicast UPDATEs directly into a `[]byte` buffer from wire bytes, and falls back to the wire-backed `Event` renderer for non-unicast families and error cases.
4. **`docs/exabgp/exabgp-differences.md` (`Three UPDATE members come from the wire bytes`):**
   - **Before:** "The bridge subscribes at `FormatFull` whenever a script receives UPDATEs, in any form, `parsed` included".
   - **After:** Documents that live `ze exabgp plugin` endpoints subscribe at `FormatBase64` (positional counted wire UPDATEs) in `ze_bgp` builds so wire bytes arrive directly without daemon-side `FormatFull` JSON.
5. **`docs/architecture/api/ipc_protocol.md` (`BGP Namespace` / `Plugin Configuration`), `docs/architecture/api/architecture.md` (`BGP Plugin Configuration`), `docs/architecture/api/commands.md` (`BGP Plugin Configuration`), and `docs/architecture/api/update-syntax.md` (`Wire Encoding Control`):**
   - **Before:** List `bgp plugin format` as a 4-value JSON-only set (`hex`, `base64`, `parsed`, `full`) with `base64` described as JSON wire bytes.
   - **After:** Keep the 4-value `bgp plugin format` set (`hex`, `base64`, `parsed`, `full`, matching `zt:wire-format` in `ze-types.yang`), define `base64` as positional counted `upd` wire lines for UPDATEs beside JSON control/session events carrying `raw.message`, and document `MuxConn.CallBatchRPC` pooled batching in `ipc_protocol.md`.
6. **`docs/architecture/api/text-parser.md` (`Original native routes in TEXT`):**
   - **Before:** States unconditionally that with no attribute filter, the received TEXT path requests all attributes through `AttributeFilter.ApplyToUpdate` and `AttributesWire.All` before rendering.
   - **After:** Notes that opt-in `format=base64` short-circuits `appendMessageTyped` ahead of `ApplyToUpdate` to emit the positional counted `upd` wire line instead of parsed TEXT, leaving all non-`base64` TEXT paths unchanged.

### Choices for Thomas

| # | Choice | Options | Decision |
|---|--------|---------|----------|
| C-1 | Scope of raw-in-text line | (a) UPDATE-only, keeping <10 control/session events per session as JSON; (b) all 5 BGP wire message types | (a) UPDATE-only raw-in-text; keep control/session events as JSON (2026-11-25) |
| C-2 | Format token and wire grammar | (a) Merge into existing `base64` (`plugin.FormatBase64`) using positional counted `upd` grammar + `MuxConn` pooled `WriteBatchFrame`; (b) separate 5th `raw-b64` token; (c) keyword-value line | (a) Merge into `base64` (`plugin.FormatBase64`) with positional counted `upd` grammar + `MuxConn` pooled `WriteBatchFrame` (owner amended 2026-11-26) |
| C-3 | Outbound ExaBGP rendering inside `ze exabgp plugin` | (a) Direct `[]byte` buffer writer for hot IPv4/IPv6 unicast UPDATEs + wire-backed `Event` fallback for non-unicast families; (b) keep `map[string]any` + `json.Marshal` for all families | (a) Direct `[]byte` buffer writer for hot IPv4/IPv6 unicast + existing renderer fallback for other families (2026-11-25) |

## Implementation Steps

1. **Phase 1: Wiring (MANDATORY FIRST)** -- document `FormatBase64`, update `enum base64` in `ze-types.yang` and YANG RPC docs (`ze-bgp-api.yang`, `ze-command-meta-cmd.yang`) + `./le yang glue check`, verify `bgp plugin format base64`, wire `appendMessageTyped` dispatch, `JSONEncoder.Message`, `MuxConn.CallBatchRPC` + `PluginConn.CallBatchRPC`, `bridgerun.EventFormat`, and `Fleet.Broadcast` `"upd "` dispatch; write failing wiring tests (`TestExabgpRawB64EndToEnd`, `TestHandleBgpPluginFormatRawB64`).
   - Tests: `TestExabgpRawB64EndToEnd`, `TestHandleBgpPluginFormatBase64`
   - Files: `internal/component/plugin/types.go`, `internal/component/config/yang/modules/ze-types.yang`, `internal/component/bgp/yang/ze-bgp-api.yang`, `internal/plugins/meta/yang/ze-command-meta-cmd.yang`, `internal/core/ipc/yang/ze-plugin-engine.yang`, `internal/plugins/meta/cmd/plugin_config.go`, `pkg/plugin/rpc/mux.go`, `internal/component/plugin/ipc/rpc.go`, `internal/component/bgp/format/text_update.go`, `internal/component/bgp/format/json_message.go`, `internal/plugins/exabgp/bridgerun/forms.go`, `internal/plugins/exabgp/bridgerun/fleet.go`, `internal/exabgp/bridge/bridge_raw_b64.go`, `internal/exabgp/bridge/bridge_raw_b64_stub.go`
   - Verify: entry points compile and wire through; `./le yang glue check` passes; wiring test fails while stub returns unimplemented.
2. **Phase 2: Daemon Formatter, Envelope Quoting & Pooled `MuxConn` Batching** -- implement `appendRawB64Update` in `internal/component/bgp/format/raw_b64.go`, `JSONEncoder.Message` `raw.message` attachment for `FormatBase64`, `updateEnvelopeCache.wrap` JSON-quoting for `FormatBase64` in `internal/component/bgp/server/event_update_receipt.go`, and `MuxConn.CallBatchRPC` using pooled `rpc.WriteBatchFrame` with `MaxMessageSize` batch splitting.
   - Tests: `TestAppendRawB64Update_FieldsAndFlags`, `TestUpdateEnvelopeWrap_RawB64QuotesPayload`, `TestJSONEncoderMessage_RawB64AttachesRaw`, `TestMuxConn_CallBatchRPC_PooledAndSplit`
   - Files: `internal/component/bgp/format/raw_b64.go`, `internal/component/bgp/format/raw_b64_test.go`, `internal/component/bgp/server/event_update_receipt.go`, `internal/component/bgp/server/event_update_receipt_test.go`, `internal/component/bgp/format/json_message.go`, `internal/component/bgp/format/json_message_test.go`, `pkg/plugin/rpc/mux.go`, `pkg/plugin/rpc/mux_batch_test.go`, `internal/component/plugin/ipc/rpc.go`
   - Verify: unit and `-race` tests pass; zero quotes/backslashes in `upd` output; `SetEnvelope(true)` + `FormatBase64` delivers quoted payload without drops; `MuxConn.CallBatchRPC` splits cleanly at `MaxMessageSize`.
3. **Phase 3: Bridge Parser, Session Cache & Fallback Wire Decoder** -- implement `ReadRawB64Update`, bounded `(asn4, addPathKey)` → `bgpctx.ContextID` cache, cached `hostname`/`pid`/`ppid`, pre-serialized `negotiated` ExaBGP JSON bytes + `Sessions.NoteRaw` in `bridge_negotiated.go`, update `forms_test.go` and `wrapper_test.go` for `FormatBase64`, and wire-backed `Event` fallback for all 10 non-unicast families and split/decode errors.
   - Tests: `TestReadRawB64Update_RoundTripAndMalformed`, `TestSessionsNoteRaw_ReconnectAndNoNegotiated`, `TestEventFormat_SelectsRawB64`, `TestWrapperScopeFromFilter`
   - Files: `internal/exabgp/bridge/bridge_raw_b64.go`, `internal/exabgp/bridge/bridge_raw_b64_stub.go`, `internal/exabgp/bridge/bridge_raw_b64_test.go`, `internal/exabgp/bridge/bridge_negotiated.go`, `internal/exabgp/bridge/bridge_event.go`, `internal/plugins/exabgp/bridgerun/forms_test.go`, `internal/plugins/exabgp/wrapper_test.go`
   - Verify: all malformed/boundary/Q-E4/jumbo-UPDATE tests pass; `TestExabgpRawB64EndToEnd` passes via fallback path.
4. **Phase 4: Direct Hot-Path Unicast Buffer Renderer, Differential Proof & Docs** -- implement `bridge_raw_render.go` (`RenderRawUpdate` direct `[]byte` ExaBGP v4/v6 JSON & text writer for IPv4/IPv6 unicast, ADD-PATH, EOR, and `--form packets|consolidate`), prove byte-identical output across the full ExaBGP fixture corpus, verify bounded hot-path allocations and `ContextID` reuse, add functional test `test/plugin/exabgp-base64.ci`, and update all departed/affected docs.
   - Tests: `TestRawB64Differential_MatchesFormatFull`, `TestRawB64DirectRender_HotPathNoMapAndContextReuse`, `test/plugin/exabgp-base64.ci`, `./le test functional exabgp`
   - Files: `internal/exabgp/bridge/bridge_raw_render.go`, `internal/exabgp/bridge/bridge_raw_render_test.go`, `test/plugin/exabgp-base64.ci`, `docs/architecture/api/architecture.md`, `docs/architecture/api/commands.md`, `docs/architecture/api/ipc_protocol.md`, `docs/architecture/api/json-format.md`, `docs/architecture/api/process-protocol.md`, `docs/architecture/api/update-syntax.md`, `docs/architecture/core-design.md`, `docs/architecture/exabgp-bridge.md`, `docs/exabgp/exabgp-differences.md`, `docs/guide/command-reference.md`
   - Verify: differential suite 100% byte-identical, `./le arch tier check`, `./le yang glue check`, `./le doc check all`, and `./le test functional exabgp` pass.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-9 has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Exact ExaBGP JSON spacing (`{ "key": value }`, `[ a, b ]`), alphabetical key order, withdraw-with-NEXT_HOP attribute retention, empty `AS_PATH` omission, dynamic `peer-as auto` = 0, and RFC 7911 per-family ADD-PATH flags match legacy `FormatFull` byte for byte |
| Naming | `plugin.FormatBase64 = "base64"` referenced by constant across all sites; `upd ` 3-byte kind word closed by space |
| Data flow | `internal/component/plugin` stays BGP-free; `internal/exabgp/bridge` keeps all `internal/component/bgp/*` imports behind `//go:build ze_bgp` (`./le arch tier check`) |
| Operator errors and doctor codes | Malformed `upd` lines log actionable warnings naming peer/reason; `handleBgpPluginFormat` error message lists `base64` among valid formats |
| Rule: performance (`ai/rules/performance.md`) | Stack-local `[4096]byte` scratch on both `appendRawB64Update` and `ReadRawB64Update`; zero `json.Unmarshal` and zero `map[string]any` on hot unicast path; bounded `bgpctx.Registry` cache |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| `FormatBase64` formatter & non-UPDATE `raw.message` | `go test -race -tags ze_bgp ./internal/component/bgp/format/...` |
| Pooled `MuxConn.CallBatchRPC` | `go test -race ./pkg/plugin/rpc/... ./internal/component/plugin/ipc/...` |
| Positional `upd` parser + direct unicast buffer renderer + differential parity | `go test -race -tags ze_bgp ./internal/exabgp/bridge/... ./internal/plugins/exabgp/... ./internal/plugins/meta/cmd/...` |
| Non-`ze_bgp` build hygiene & tier gate | `./le arch tier check` and `go test ./internal/exabgp/bridge/...` (untagged) |
| End-to-end functional proof | `./le test plugin exabgp-base64` and `./le test functional exabgp` |
| Architecture docs & anchors | `./le doc check all` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Counted text `<n>:<bytes>` checks `size <= uint64(len(rest))` BEFORE slicing (never panics on attacker-supplied count); decoded UPDATE body capped at 65,516 bytes (RFC 8654); unknown `<flags>` bits and trailing bytes refused |
| Log/line injection (CWE-116) | ExaBGP text renderer preserves `appendOneline` sanitization for every peer-chosen string (never allows embedded `\n` to forge a script event) |
| Resource exhaustion | Bounded `bgpctx.ContextID` cache prevents global registry exhaustion; `MuxConn.CallBatchRPC` preserves `MaxMessageSize` frame cap and drops oversized buffers from `batchBufPool` |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Positional counted `upd` line (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>`) | Keyword-value raw line; custom binary `MuxConn` frame | Follows Ze's Answer Protocol counted grammar (zero key names, single `uint32` kind load, ~3.2 ns/number fused parse, byte-counted slicing without payload scanning) while staying inside newline-framed `deliver-batch` |
| Merge into existing `plugin.FormatBase64 = "base64"` token | Adding a 5th `raw-b64` enum beside unimplemented `base64` | Keeps `zt:wire-format` at 4 working values (`hex`, `base64`, `parsed`, `full`) with zero dead enum (owner decision 2026-11-26) |
| UPDATE-only raw-in-text; rare control/session events stay JSON | Raw-in-text for all 5 BGP wire message types | Non-UPDATEs are <10 events per session lifetime (~0% of Q-E7 CPU); avoids duplicating OPEN/NOTIFICATION/REFRESH decoders |
| Direct `[]byte` buffer renderer for hot IPv4/IPv6 unicast + wire-backed `Event` fallback for non-unicast families | Outbound `map[string]any` + `json.Marshal` for all families; duplicating all 10 exotic family renderers | Eliminates per-wrapper GC/reflection on full-table feeds while preserving 100% parity across all 10 complex families with minimal surface |
| Add `MuxConn.CallBatchRPC` using pooled `rpc.WriteBatchFrame` | Leaving `MuxConn` on `map[string]any` + `json.Marshal` | Removes redundant per-batch reflection inside `ze` for all external TLS plugins |

## Known Limitations
- Out-of-tree foreign external NLRI decoder plugins (separate binaries not compiled into `ze`) are not accessible through `registry.InProcessNLRIDecoder` inside `ze exabgp plugin`; all 10 shipped NLRI families are compiled into `ze` via `plugin/all`.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-9 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: plugin scope, no BGP wire change)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)

---

## Implementation Summary

### What Was Implemented
- Implemented `plugin.FormatBase64 = "base64"` as Ze's positional counted UPDATE wire format (`upd <flags> <local-as> <remote-as> <conn-id> <n>:<peer> <n>:<local> <n>:<router-id> <n>:<extra-addpath> <n>:<b64-body>\n`) in `internal/component/bgp/format/raw_b64.go` (`appendRawB64Update`), short-circuiting `appendMessageTyped` (`internal/component/bgp/format/text_update.go`) ahead of `bgpfilter.ApplyToUpdate`.
- Extended `JSONEncoder.Message` (`internal/component/bgp/format/json_message.go`, `attachesRawMessage`) to attach `"raw":{"message":"<hex>"}` on OPEN, NOTIFICATION, KEEPALIVE, and ROUTE-REFRESH for `FormatBase64` identically to `FormatFull`.
- Extended `updateEnvelopeCache.wrap` (`internal/component/bgp/server/event_update_receipt.go`, `needsQuotedEnvelopePayload`) to JSON-quote `FormatBase64` UPDATE lines when `SetEnvelope(true)` is enabled.
- Added pooled `MuxConn.CallBatchRPC` (`pkg/plugin/rpc/mux.go`) with upfront single-event `MaxMessageSize` refusal and sequential FIFO `MaxMessageSize` batch splitting via `Conn.writeBatchWithDeadline` (`pkg/plugin/rpc/conn.go`, armed with `armWriteCancellation`) and wired `PluginConn.CallBatchRPC` (`internal/component/plugin/ipc/rpc.go`) to it.
- Updated `bridgerun.EventFormat` (`internal/plugins/exabgp/bridgerun/forms.go`) to select `plugin.FormatBase64` when `bridge.SupportsRawWire()` is true and `bgp:update` is allowed, and dispatched `"upd "` events in `Fleet.Broadcast` (`internal/plugins/exabgp/bridgerun/fleet.go`, `broadcastRawUpdate`).
- Implemented `ReadRawB64Update`, bounded `rawContextCache` (`resolveRawContextID`), and `Sessions.RenderRawUpdate` + `renderRawUpdateFallback` (`internal/exabgp/bridge/bridge_raw_b64.go`, `//go:build ze_bgp`), non-`ze_bgp` stub (`bridge_raw_b64_stub.go`), zero-map direct `[]byte` ExaBGP v4/v6 JSON & text unicast renderer (`bridge_raw_render.go`, `bridge_raw_render_emit.go`), pre-serialized `negotiated` JSON caching + `Sessions.noteRaw` Q-E4 reconnect invalidation (`bridge_negotiated.go`), and `sync.Once`-cached `hostname`/`pid`/`ppid` (`bridge_event.go`).
- Updated 3 YANG modules (`ze-types.yang`, `ze-bgp-api.yang`, `ze-command-meta-cmd.yang`) and 10 documentation pages, added unit/differential/fuzz/wiring tests and live functional test `test/plugin/exabgp-base64.ci`, and added news fragment `website/changes/fragments/exabgp-raw-text-event-transport.md`.

### Bugs Found/Fixed
- `internal/exabgp/bridge/bridge_event.go` (`normalizeOutgoingAttributes`): always clone `attrObj` so callers (`appendUpdateText` → `overlayWire`) cannot mutate `e.Data["attr"]` in place across sibling renders; covered by `TestRawB64Differential_MatchesFormatFull`.
- Companion standalone bugfix landed separately in commit `ba5601efcc` (`internal/component/plugin/server/{server.go,delivery_graph.go,participant_filter_test.go}`): `deliveryGraphMu` lost-update race uncovered while running 80/80 `stress-repro` on `test/plugin/exabgp-base64.ci`.
- Closure Run 1 fix: unexported 7 package-internal methods (`Sessions.noteRaw` in `bridge_negotiated.go` and `RawUpdate.{isSent,hasASN4,asn4,hasRemoteAS,addPathIPv4Unicast,addPathIPv6Unicast}` in `bridge_raw_b64.go` / `bridge_raw_b64_stub.go`) flagged by `./le repo check` as having no cross-package non-test caller; renamed `TestHandleBgpPluginFormatRawB64` → `TestHandleBgpPluginFormatBase64` (`internal/plugins/meta/cmd/plugin_config_test.go`) to match the spec TDD plan.

### Documentation Updates
- `docs/architecture/api/architecture.md`: updated `bgp plugin format base64` row (`### BGP Plugin Configuration`).
- `docs/architecture/api/commands.md`: updated `bgp plugin format base64` entry (`### BGP Plugin Configuration`).
- `docs/architecture/api/ipc_protocol.md`: documented `MuxConn.CallBatchRPC` pooled `WriteBatchFrame` batching (`pkg/plugin/rpc/mux.go -- MuxConn.CallBatchRPC`, `pkg/plugin/rpc/batch.go -- WriteBatchFrame`, `internal/component/plugin/ipc/rpc.go -- PluginConn.CallBatchRPC`) and positional counted `base64` `upd` grammar (`internal/component/bgp/format/raw_b64.go -- appendRawB64Update`).
- `docs/architecture/api/json-format.md`: documented `format=base64` UPDATE lines and `raw.message` on non-UPDATE JSON (`internal/component/bgp/format/raw_b64.go -- appendRawB64Update`, `internal/component/bgp/format/json_message.go -- JSONEncoder.Message`).
- `docs/architecture/api/process-protocol.md`: documented `base64` on Stage 5 `ReadyInput.format` / `SubscribeEventsInput.format` and JSON-quoting under `EventEnvelope`.
- `docs/architecture/api/text-parser.md`: documented opt-in `format=base64` short-circuit ahead of `ApplyToUpdate`.
- `docs/architecture/api/update-syntax.md`: updated `bgp plugin format base64` in `## Wire Encoding Control`.
- `docs/architecture/core-design.md`: noted opt-in `format=base64` positional `upd` UPDATE delivery in Section 6 (`### JSON Events (Engine → Plugin)`).
- `docs/architecture/exabgp-bridge.md`: updated Stage 5 `FormatBase64` selection, `ReadRawB64Update`, and `Sessions.RenderRawUpdate` (`internal/exabgp/bridge/bridge_raw_b64.go -- ReadRawB64Update, Sessions.RenderRawUpdate, SupportsRawWire`, `internal/plugins/exabgp/bridgerun/forms.go -- EventFormat`, `internal/plugins/exabgp/bridgerun/fleet.go -- Fleet.Broadcast`), and restated line 1322 decision `Source` as bare stem `spec-exabgp-raw-text-event-transport`.
- `docs/exabgp/exabgp-differences.md`: updated "Three UPDATE members come from the wire bytes" (`internal/exabgp/bridge/bridge_raw_b64.go -- ReadRawB64Update, Sessions.RenderRawUpdate`).
- `docs/guide/command-reference.md`: already lists `bgp plugin format <hex|base64|parsed|full>` at line 4843 (verified unchanged after Option 1 merge into `base64`).
- `./le doc check all`: 0 broken source anchors in all 10 edited docs and 0 broken YANG help summaries in our 3 edited YANG modules (1 pre-commit `clicatalog` live-vs-HEAD mismatch on `ze-command-meta-cmd.yang` clears once Commit A lands and `le` rebuilds from HEAD; other reported anchors/char-cap are pre-existing in unrelated files).

### Deviations from Plan
- **Option 1 merge (owner decision 2026-11-26):** Merged the planned 5th `raw-b64` enum into existing `plugin.FormatBase64 = "base64"`, keeping `zt:wire-format` at 4 values; `ze-plugin-engine.yang`, `internal/plugins/meta/cmd/plugin_config.go`, and `docs/guide/command-reference.md` required no diff against HEAD.
- **File split for `ze-go-style.md` 1,000-line threshold:** Split the direct unicast renderer into `internal/exabgp/bridge/bridge_raw_render.go` (611 lines, plan builder & validators) and `internal/exabgp/bridge/bridge_raw_render_emit.go` (876 lines, JSON & text emitters), and placed `TestExabgpRawB64EndToEnd` in `internal/plugins/exabgp/wrapper_raw_b64_test.go` (`//go:build ze_bgp`) so untagged `ze_exabgp` builds never import gated `internal/component/bgp/*`.
- **Unexported package-internal methods:** Unexported `Sessions.noteRaw` and `RawUpdate.{isSent,hasASN4,asn4,hasRemoteAS,addPathIPv4Unicast,addPathIPv6Unicast}` (keeping `Direction()`, `SupportsRawWire()`, `ReadRawB64Update()`, and `Sessions.RenderRawUpdate()` exported) to satisfy `./le repo check` unwired-export enforcement.

## Mistake Log

| Kind | What happened | What was true instead | How discovered | Action |
|------|---------------|----------------------|----------------|--------|
| approach | Exported `Sessions.NoteRaw` and 6 `RawUpdate` flag accessors in `internal/exabgp/bridge` even though only same-package code called them | `./le repo check` flags exported symbols in `internal/` packages with no cross-package non-test caller | Closure Run 1 (`./le repo check`) | Unexported `noteRaw` and the 6 flag accessors across `bridge_negotiated.go`, `bridge_raw_b64.go`, `bridge_raw_b64_stub.go`, `bridge_raw_render.go`, `bridge_raw_render_emit.go`, `bridge_raw_b64_test.go` |
| approach | Option 1 rename left `TestHandleBgpPluginFormatRawB64` in `plugin_config_test.go` while the spec TDD plan named it `TestHandleBgpPluginFormatBase64` | Test name should match the merged `base64` format and TDD plan | Closure Run 1 (`/ze-review-spec`) | Renamed to `TestHandleBgpPluginFormatBase64` in `internal/plugins/meta/cmd/plugin_config_test.go` |

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Positional counted `base64` UPDATE line (`upd ...`) | Done | `internal/component/bgp/format/raw_b64.go:45` (`appendRawB64Update`), `text_update.go:162` | Bypasses `bgpfilter.ApplyToUpdate` and JSON formatting |
| Non-UPDATE JSON `raw.message` under `base64` | Done | `internal/component/bgp/format/json_message.go:52` (`attachesRawMessage`) | Matches `FormatFull` for OPEN/NOTIFICATION/KEEPALIVE/REFRESH |
| Enveloped `base64` payload JSON-quoting | Done | `internal/component/bgp/server/event_update_receipt.go:57` (`needsQuotedEnvelopePayload`) | Quotes `upd` line under `SetEnvelope(true)` |
| Pooled `MuxConn.CallBatchRPC` with `MaxMessageSize` splitting | Done | `pkg/plugin/rpc/mux.go:313` (`MuxConn.CallBatchRPC`), `conn.go:662`, `ipc/rpc.go:134` | Preserves `#<id> ze-plugin-callback:deliver-batch {"events":[...]}\n` |
| Stage 5 `base64` opt-in in `ze exabgp plugin` | Done | `internal/plugins/exabgp/bridgerun/forms.go:138` (`EventFormat`), `fleet.go:218` (`broadcastRawUpdate`) | Automatic in `ze_bgp` builds; `FormatFull` in `!ze_bgp` |
| Positional `upd` parser + bounded `ContextID` cache + fallback | Done | `internal/exabgp/bridge/bridge_raw_b64.go:171,379,503` (`ReadRawB64Update`, `resolveRawContextID`, `RenderRawUpdate`) | RFC 8654 65,516B bound, stack scratch, 1,024-entry cache |
| Direct `[]byte` unicast JSON/text renderer + cached envelope/negotiated | Done | `internal/exabgp/bridge/bridge_raw_render.go:132`, `bridge_raw_render_emit.go`, `bridge_negotiated.go:158`, `bridge_event.go:422` | Zero `json.Unmarshal`, zero `map[string]any` on hot IPv4/IPv6 unicast |

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Done | `TestAppendRawB64Update_FieldsAndFlags`, `TestUpdateEnvelopeWrap_RawB64QuotesPayload` | `appendRawB64Update` (`raw_b64.go:45`), `needsQuotedEnvelopePayload` (`event_update_receipt.go:88`) |
| AC-2 | Done | `TestJSONEncoderMessage_RawB64AttachesRaw` | `attachesRawMessage` (`json_message.go:59`) |
| AC-3 | Done | `TestMuxConn_CallBatchRPC_PooledAndSplit` | `MuxConn.CallBatchRPC` (`mux.go:313`), `PluginConn.CallBatchRPC` (`ipc/rpc.go:131`) |
| AC-4 | Done | `TestEventFormat_SelectsRawB64`, `TestHandleBgpPluginFormatBase64`, `TestWrapperScopeFromFilter` | `EventFormat` (`forms.go:138`), `SupportsRawWire` (`bridge_raw_b64.go:157`, `bridge_raw_b64_stub.go:83`) |
| AC-5 | Done | `TestRawB64Differential_MatchesFormatFull`, `test/plugin/exabgp-base64.ci` | Byte-identical v4/v6 JSON & text across forms, grants, directions, ADD-PATH, EOR, attributes |
| AC-6 | Done | `TestRawB64Differential_MatchesFormatFull` | All 10 compiled non-unicast families + unsplit/undecoded error cases via `renderRawUpdateFallback` |
| AC-7 | Done | `TestRawB64DirectRender_HotPathNoMapAndContextReuse` | `renderRawUpdateDirect` (`<= 4` allocs/run), `resolveRawContextID` reuse across 10,000 UPDATEs |
| AC-8 | Done | `TestSessionsNoteRaw_ReconnectAndNoNegotiated` | `Sessions.noteRaw` / `noteRawJSON` reconnect invalidation on non-zero `conn-id` mismatch |
| AC-9 | Done | `TestReadRawB64Update_RoundTripAndMalformed`, `FuzzReadRawB64Update` | 29 malformed boundary rejections + 65,516B jumbo pass / 65,517B rejection |

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAppendRawB64Update_FieldsAndFlags` | Done | `internal/component/bgp/format/raw_b64_test.go:129` | Exact name |
| `TestUpdateEnvelopeWrap_RawB64QuotesPayload` | Done | `internal/component/bgp/server/event_update_receipt_test.go:32` | Exact name |
| `TestJSONEncoderMessage_RawB64AttachesRaw` | Done | `internal/component/bgp/format/json_message_test.go:61` | Exact name |
| `TestMuxConn_CallBatchRPC_PooledAndSplit` | Done | `pkg/plugin/rpc/mux_batch_test.go:56` | Exact name |
| `TestHandleBgpPluginFormatBase64` | Done | `internal/plugins/meta/cmd/plugin_config_test.go:25` | Renamed from `RawB64` in closure Run 1 |
| `TestEventFormat_SelectsRawB64` | Done | `internal/plugins/exabgp/bridgerun/forms_test.go:138` | Exact name |
| `TestReadRawB64Update_RoundTripAndMalformed` | Done | `internal/exabgp/bridge/bridge_raw_b64_test.go:56` | Exact name (+ `FuzzReadRawB64Update` at line 362) |
| `TestRawB64Differential_MatchesFormatFull` | Done | `internal/exabgp/bridge/bridge_raw_render_test.go:259` | Exact name |
| `TestRawB64DirectRender_HotPathNoMapAndContextReuse` | Done | `internal/exabgp/bridge/bridge_raw_render_test.go:549` | Exact name |
| `TestSessionsNoteRaw_ReconnectAndNoNegotiated` | Done | `internal/exabgp/bridge/bridge_raw_b64_test.go:193` | Exact name |
| `TestExabgpRawB64EndToEnd` | Done | `internal/plugins/exabgp/wrapper_raw_b64_test.go:44` | Exact name |
| `exabgp-base64` | Done | `test/plugin/exabgp-base64.ci` | Promoted functional test |

### Files from Plan
| File | Status | Notes |
|------|--------|-------|
| All 14 modified Go files | Done | Verified via `git status --porcelain` |
| All 12 created Go files | Done | Verified via `ls -l` (`bridge_raw_render.go` split with `bridge_raw_render_emit.go`) |
| All 3 modified YANG files | Done | `ze-types.yang`, `ze-bgp-api.yang`, `ze-command-meta-cmd.yang` |
| All 10 modified docs | Done | All 9 `departs` pages + `process-protocol.md` updated |
| `test/plugin/exabgp-base64.ci` | Done | Promoted live `.ci` functional test |
| `website/changes/fragments/exabgp-raw-text-event-transport.md` | Done | Closure news fragment |
| `plan/release/fixes/spec-bus-observer-delivery-decoupling.md` | Done | Citer cleanup to bare stem `spec-exabgp-raw-text-event-transport` |

### Audit Summary
- **Total items:** 28 (7 Task requirements + 9 ACs + 12 TDD tests)
- **Done:** 28
- **Partial:** 0
- **Skipped:** 0
- **Changed:** 3 (recorded in Deviations: Option 1 merge into `base64`, `bridge_raw_render_emit.go` file split, unexported internal `bridge` methods)

## Goal Validation (BLOCKING)

| Goal (from Task) | Evidence Type | Concrete Evidence |
|------------------|---------------|-------------------|
| Implement `plugin.FormatBase64` positional counted `upd` UPDATE format bypassing `ApplyToUpdate`/JSON while keeping control events JSON with `raw.message` | Unit + functional tests | `TestAppendRawB64Update_FieldsAndFlags` (`raw_b64_test.go`), `TestUpdateEnvelopeWrap_RawB64QuotesPayload` (`event_update_receipt_test.go`), `TestJSONEncoderMessage_RawB64AttachesRaw` (`json_message_test.go`), `test/plugin/exabgp-base64.ci` (13/13 VERIFY STEPs) |
| Eliminate `map[string]any` batch marshaling on `MuxConn` via pooled `rpc.WriteBatchFrame` | Unit + `-race` test | `TestMuxConn_CallBatchRPC_PooledAndSplit` (`mux_batch_test.go`) verifying exact `WriteBatchFrame` equality, `MaxMessageSize` FIFO splitting, and concurrent `CallRPC`/`CallBatchRPC` safety |
| Opt `ze exabgp plugin` into `base64` at Stage 5 without operator config change | Unit + wiring + functional tests | `TestEventFormat_SelectsRawB64`, `TestExabgpRawB64EndToEnd`, `test/plugin/exabgp-base64.ci` (80/80 `stress-repro` + live run) |
| Render hot IPv4/IPv6 unicast UPDATEs directly into `[]byte` without `json.Unmarshal` / `map[string]any`, reusing wire-backed `Event` for non-unicast and errors with byte-identical output | Differential + alloc + fuzz tests | `TestRawB64Differential_MatchesFormatFull`, `TestRawB64DirectRender_HotPathNoMapAndContextReuse` (`<= 4` allocs/run, `ContextID` reuse over 10,000 UPDATEs), `TestReadRawB64Update_RoundTripAndMalformed`, `FuzzReadRawB64Update` (46,485 execs) |

## Work Not Done

| What was not done | Why | The spec that now owns it |
|-------------------|-----|---------------------------|
| None — all in-scope items implemented | All 9 ACs, unit/differential/fuzz tests, functional test, YANG, and docs are complete | N-A |

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | `tmp/review/exabgp-raw-text-event-transport-dead6d00-82cf-49de-a3e9-b034538797de.md` (43 files, `verdict=clean`) |
| `./le spec review check` | `review_gate: OK (31 code files, clean, hashes match tmp/review/exabgp-raw-text-event-transport-dead6d00-82cf-49de-a3e9-b034538797de.md)` |
| Rounds | 4 (Run 1: 2 ISSUEs fixed; Run 2: 2 compile-out BLOCKERs + 2 NOTEs fixed; Run 3: CLEAN; Run 4: CLEAN after commit-gate doctor-code + decision-row Source fixes) |
| Reviewer lenses used | logic+wiring, security+edge-cases, performance+style, compile-out/`!ze_bgp`, doctor-coverage, doc-consistency, design-documents, spec-completeness (`/ze-review` + `/ze-review-spec`) |

### Findings fixed
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
| 1 | ISSUE | `./le repo check` flagged 9 exported symbols with no cross-package non-test caller (`Sessions.NoteRaw` and `RawUpdate.{IsSent,HasASN4,AddPathIPv4Unicast,AddPathIPv6Unicast}` in both build-tagged files, plus `ASN4`/`HasRemoteAS`) | `internal/exabgp/bridge/bridge_negotiated.go:158`, `bridge_raw_b64.go:104-134`, `bridge_raw_b64_stub.go:52-79` | Unexported `noteRaw` and the 6 flag accessors (`isSent`, `hasASN4`, `asn4`, `hasRemoteAS`, `addPathIPv4Unicast`, `addPathIPv6Unicast`) and updated internal callers + white-box tests; re-ran `./le repo check` (0 issues on spec symbols) and `./le go style check` (OK) |
| 2 | ISSUE | `/ze-review-spec` test name mismatch: `TestHandleBgpPluginFormatRawB64` did not match TDD plan `TestHandleBgpPluginFormatBase64` after Option 1 merge | `internal/plugins/meta/cmd/plugin_config_test.go:25` | Renamed test function and comment to `TestHandleBgpPluginFormatBase64`; `./le go style check` OK |
| 3 | BLOCKER | Unexporting `RawUpdate` flag methods left 5 unused `rawB64Flag*` constants and 5 unused methods in `!ze_bgp` compile-out builds (`bridge_raw_b64_stub.go`, flagged by Run 2 reviewer and `./le go lint run` `compile-out`) | `internal/exabgp/bridge/bridge_raw_b64_stub.go` | Removed unused constants and methods from `bridge_raw_b64_stub.go`, keeping only `rawB64FlagSent` and `isSent` used by `Direction()` |
| 4 | BLOCKER | Unexporting `Sessions.noteRaw` left `noteRaw`, `reconnectedRawSession`, and `unexchangedNegotiatedJSON*` unused in `!ze_bgp` compile-out builds (`bridge_negotiated.go`, flagged by Run 2 reviewer and `./le go lint run` `compile-out`) | `internal/exabgp/bridge/bridge_negotiated.go` | Moved `reconnectedRawSession`, `Sessions.noteRaw`, and `unexchangedNegotiatedJSON*` into `//go:build ze_bgp` `bridge_raw_b64.go` |
| 5 | NOTE | Stale `Sessions.NoteRaw` comment spellings in raw bridge tests + Go-internal jargon (`map[string]any`, `MaxMessageSize`) in news fragment | `bridge_raw_b64_test.go:188`, `bridge_raw_render_test.go:4`, `website/changes/fragments/exabgp-raw-text-event-transport.md` | Updated comments to `Sessions.noteRaw` and rewrote news fragment in operator-facing terms |
| 6 | BLOCKER | `./le arch doctor-coverage check` flagged `f.log.Warn("exabgp-bridge: invalid base64 update event", ...)` lacking a registered doctor diagnostic code; `./le doc consistency` flagged `docs/architecture/exabgp-bridge.md` row 2 `Source` bare stem not resolving to a committed file | `internal/plugins/exabgp/bridgerun/fleet.go:261`, `docs/architecture/exabgp-bridge.md:1322` | Added `internal/plugins/exabgp/bridgerun/register_doctor_codes.go` registering `codeDoctorInvalidBase64Update` via `diagnostic.Register` + `diagnostic.RegisterEventCode`, passed `"code", codeDoctorInvalidBase64Update` at `fleet.go:261`, and pointed `docs/architecture/exabgp-bridge.md:1322` `Source` to `website/changes/fragments/exabgp-raw-text-event-transport.md` |

### Run 1
| Check | Result | Notes |
|-------|--------|-------|
| `/ze-review` | 0 BLOCKER, 2 ISSUE (fixed) | Style pass run across all 26 Go files (`./le go style check`: 26/26 OK); 0 peer-reachable panics; 0 weakened-test findings in spec files; Finding #1 fixed and `./le repo check` cleared of all 9 spec issues |
| `/ze-review-spec` | 0 BLOCKER, 1 ISSUE (fixed) | All 9 ACs, 28 design pages, 26 decision rows, 9 departed docs, and functional test verified; Finding #2 (`TestHandleBgpPluginFormatBase64`) fixed |

### Run 2
| Check | Result | Notes |
|-------|--------|-------|
| Independent Run 2 Reviewer (`ba3937f0`) + `./le go lint run` | 2 BLOCKER (fixed), 0 ISSUE, 2 NOTE (fixed) | Caught `!ze_bgp` `compile-out` `unused` symbols in `bridge_raw_b64_stub.go` and `bridge_negotiated.go` after Run 1 unexport; fixed Findings #3-#5 and re-verified with `-race -tags ze_bgp`, untagged `!ze_bgp` tests, `./le go style check`, and `./le go lint run` across default, integration, and `compile-out` flavors (0 findings in any spec file) |

### Run 3
| Check | Result | Notes |
|-------|--------|-------|
| Independent Run 3 Reviewer (`1b648ecc`) | **CLEAN** (0 BLOCKER, 0 ISSUE, 2 NOTE) | Verified `!ze_bgp` stub hygiene, `bridge_raw_b64.go`/`bridge_negotiated.go` build-tag split, `ze-go-style.md` 7 questions (0 peer-reachable panics), and operator news fragment compliance; 2 non-blocking header-comment NOTEs (`Related: bridge_negotiated.go`) noted |

### Run 4
| Check | Result | Notes |
|-------|--------|-------|
| Independent Run 4 Reviewer (`10e4e6a9`) | **CLEAN** (0 BLOCKER, 0 ISSUE, 1 NOTE) | Verified `bridgerun/register_doctor_codes.go`, `fleet.go:261` doctor code wiring, `docs/architecture/exabgp-bridge.md:1322` decision `Source`, `ze-go-style.md`, `./le arch tier check`, `-race -tags "ze_bgp ze_exabgp"` + untagged tests, and `./le go lint run`; 1 non-blocking reciprocal header `WARN` NOTE noted |

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|
| `internal/component/bgp/format/raw_b64.go`, `raw_b64_test.go` | Yes | `ls -l`: 6385B (`raw_b64.go`), 13158B (`raw_b64_test.go`) |
| `internal/component/bgp/server/event_update_receipt_test.go`, `pkg/plugin/rpc/mux_batch_test.go`, `internal/plugins/meta/cmd/plugin_config_test.go` | Yes | `ls -l`: 4257B, 10655B, 2353B |
| `internal/exabgp/bridge/bridge_raw_b64.go`, `bridge_raw_b64_stub.go`, `bridge_raw_b64_test.go`, `bridge_raw_render.go`, `bridge_raw_render_emit.go`, `bridge_raw_render_test.go` | Yes | `ls -l`: 18872B, 2841B, 16167B, 16929B, 25616B, 27737B |
| `internal/plugins/exabgp/wrapper_raw_b64_test.go`, `test/plugin/exabgp-base64.ci`, `website/changes/fragments/exabgp-raw-text-event-transport.md` | Yes | `ls -l`: 7068B, 13305B, 1881B |

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|
| AC-1 | `appendRawB64Update` positional `upd` line + `needsQuotedEnvelopePayload` quoting | `raw_b64.go:45`, `text_update.go:162`, `event_update_receipt.go:88`; tests `TestAppendRawB64Update_FieldsAndFlags`, `TestUpdateEnvelopeWrap_RawB64QuotesPayload` |
| AC-2 | Non-UPDATE `raw.message` under `FormatBase64` | `json_message.go:52,59` (`attachesRawMessage`); test `TestJSONEncoderMessage_RawB64AttachesRaw` |
| AC-3 | Pooled `MuxConn.CallBatchRPC` with `MaxMessageSize` splitting | `mux.go:313`, `conn.go:662`, `ipc/rpc.go:134`; test `TestMuxConn_CallBatchRPC_PooledAndSplit` |
| AC-4 | Stage 5 `FormatBase64` opt-in & `bgp plugin format base64` | `forms.go:138`, `plugin_config.go:61`; tests `TestEventFormat_SelectsRawB64`, `TestHandleBgpPluginFormatBase64`, `TestWrapperScopeFromFilter` |
| AC-5, AC-6 | Byte-identical ExaBGP v4/v6 JSON & text across unicast, all 10 non-unicast families, and error cases | `bridge_raw_render.go:132`, `bridge_raw_render_emit.go`, `bridge_raw_b64.go:520`; test `TestRawB64Differential_MatchesFormatFull` |
| AC-7 | Direct `[]byte` renderer without `map[string]any` and bounded `ContextID` reuse | `bridge_raw_render.go:132`, `bridge_raw_b64.go:379` (`rawContextCacheMax = 1024`); test `TestRawB64DirectRender_HotPathNoMapAndContextReuse` |
| AC-8 | Q-E4 reconnect invalidation on non-zero `conn-id` change + no-`negotiated` decoding | `bridge_raw_b64.go:641,648` (`reconnectedRawSession`, `Sessions.noteRaw`), `bridge_raw_render.go:103` (`noteRawJSON`); test `TestSessionsNoteRaw_ReconnectAndNoNegotiated` |
| AC-9 | Malformed `upd` rejection + RFC 8654 65,516B jumbo support | `bridge_raw_b64.go:55,171,335` (`rawUpdateBodyMax = 65535 - 19`, `ReadRawB64Update`, `decodeRawB64Body`); tests `TestReadRawB64Update_RoundTripAndMalformed`, `FuzzReadRawB64Update` |

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|
| `ze exabgp plugin` Stage 5 `ready` (`FormatBase64`) → `events.go` → `appendRawB64Update` → `MuxConn.CallBatchRPC` → `Fleet.Broadcast` → `ReadRawB64Update` → `RenderRawUpdate` | `test/plugin/exabgp-base64.ci` | Read lines 1-350: starts real `ze` daemon + peer1 + 3 external `ze exabgp plugin` endpoints (`--encoder json`, `--encoder text`, `--encoder json --form receive-consolidate`), sends IPv4 ADD-PATH, IPv6 MP_REACH_NLRI, and IPv4 EOR, verifies exact JSON spacing, `as-path`, `path-information`, `eor`, `header`/`body` hex, OPEN `raw.message`, and zero malformed warnings |
| `bgp plugin format base64` (`handleBgpPluginFormat`) | `internal/plugins/meta/cmd/plugin_config_test.go` (`TestHandleBgpPluginFormatBase64`) | Read lines 1-68: verifies `base64`/`hex`/`parsed`/`full` acceptance and missing/unknown error mentioning `base64` |

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|
| A-1 | confirmed | `TestRawB64Differential_MatchesFormatFull` (`bridge_raw_render_test.go:259`) proves 100% byte-identical output across unicast, all 10 non-unicast families, EOR, forms, and error paths |
| A-2 | confirmed | Owner ruling (2026-11-25/2026-11-26) in `docs/architecture/exabgp-bridge.md` `## Design decisions`; non-UPDATE control events stay JSON with `raw.message` (`TestJSONEncoderMessage_RawB64AttachesRaw`) |
| A-3 | confirmed | `//go:build ze_bgp` files isolated beside `//go:build !ze_bgp` stub (`bridge_raw_b64_stub.go`, `SupportsRawWire() == false`); verified by `./le arch tier check` and untagged `go test` |
| A-4 | confirmed | `TestMuxConn_CallBatchRPC_PooledAndSplit` (`mux_batch_test.go:56`) proves exact `WriteBatchFrame` wire equality, `MaxMessageSize` FIFO splitting, and `-race` concurrency with `CallRPC` |

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|
| All 9 departed pages (`architecture.md`, `commands.md`, `ipc_protocol.md`, `json-format.md`, `core-design.md`, `update-syntax.md`, `text-parser.md`, `exabgp-bridge.md`, `exabgp-differences.md`) + `process-protocol.md` | Verified via `git diff HEAD -- docs/...` and `./le spec design check` (`ready to present`); 0 broken source anchors in `./le doc check all` for all 10 pages | Yes |
| CLI command reference (`docs/guide/command-reference.md:4843`) | Line 4843 already lists `bgp plugin format <hex\|base64\|parsed\|full>` | Yes |
| Citer clearance (`docs/architecture/exabgp-bridge.md:1322`, `plan/release/fixes/spec-bus-observer-delivery-decoupling.md:197,214,427`) | `git grep -n "exabgp-raw-text-event-transport"` confirms 0 remaining `plan/release/fixes/...md` path citations; all restated as bare stem `spec-exabgp-raw-text-event-transport` | Yes |
| Categories marked No (config syntax, RFC compliance, metrics, doctor checks, plugin inventory) | No new config root/leaf, no BGP peer wire change, no new metric/doctor code/plugin registration (`git status` scoped to 41 spec files + citer + news fragment) | Yes |
