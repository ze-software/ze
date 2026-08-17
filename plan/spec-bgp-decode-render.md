# Spec: bgp-decode-render -- one ordered description tree for decoded BGP messages

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | cli |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/bgp-decode-render.md` (create on the first deferral) |
| Handoff | - |
| Updated | 2026-08-15 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Ze has no Go renderer for a decoded BGP message. The wireshark-style view that
operators ask for exists only in Python, in the exabgp-compat test harness
(`print_payload` and `format_hex_block`, `test/exabgp-compat/bin/bgp`). It never
reaches a user of the daemon or of the `ze` binary.

Ze does have a decoder. `decodeHexPacket` (`internal/component/bgp/cli/decode.go`)
turns hex into a Ze-format JSON envelope or into human text, and it already
resolves plugin-registered capability and NLRI codecs. Three surfaces reach it:
`ze bgp decode`, `show bgp decode`, and the web tool page through
`pluginreg.SetPacketDecoder`.

Its human output is the weak half. `decode_human.go` writes flat two-space lines
from a `map[string]any`, so field order is whatever Go map iteration produced,
there is no hex dump, and no two renderers in the tree agree. Five formatters
render BGP messages today: the Python harness, `decode_human.go`,
`format/text_human.go`, `monitor/format.go`, and `(*decode.DecodedMessage).String`.

Nothing renders decoded messages for LIVE traffic at all. An operator who wants
to see what a peer is sending has no route to it short of a packet capture.

This spec builds the missing piece once: an ordered description tree, one tree
renderer, one hex dump, and a runtime tap that emits the same rendering through
the logging stack ze already has.

The canonical OPEN fixture for AC-1 is:

```
ffffffffffffffffffffffffffffffff001d0104fde8005a0102030400
```

It is a complete BGP OPEN message: 16-octet marker, length 29, type 1, version
4, ASN 65000, hold time 90 seconds, BGP identifier 1.2.3.4, and no optional
parameters.

## What this spec owes

| Piece | Note |
|-------|------|
| An ordered node type | `map[string]any` carries no field order, so it cannot feed a tree. The node type is what makes the renderer possible |
| A tree renderer and a hex dump | Box-drawing children plus the offset, hex and ASCII-gutter block. Neither exists in Go |
| `decode_human.go` retargeted, its flat formatters deleted | `ai/rules/no-layering.md`: delete X, then implement Y. Two renderers side by side is the failure this spec removes |
| A runtime tap | The decoded tree for live traffic, reached by the EXISTING `request log level` operation, adding nothing to the command surface |

## Non-goals

`format/text_human.go` and `monitor/format.go` serve the shipped plugin IPC
format and the monitor TUI. Retargeting them changes contracts other code
depends on, and belongs to its own spec. The Python harness stays as it is: it
cannot call Go, so its tree and this one can drift. That is accepted, recorded
under Known Limitations.

The pcap half of the original request is `plan/spec-bgp-pcap-decode.md`.

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress. -->

### Architecture Docs

- [ ] `ai/rules/simplicity.md` - whether the "interface every BGP component implements" that prompted this work is justified
  → Decision: it is NOT. An interface with one implementation is refused, and the wire interfaces (`message.Message`, `attribute.Attribute`, `capability.Capability`) are `WriteTo`-shaped hot-path contracts over pooled, lazily-parsed data. A mandatory `Describe()` would put an allocating method on the forwarding path and force every NLRI codec plugin to implement rendering it never uses.
  → Constraint: the renderer takes an ordered node; who builds the node is the caller's business. An optional probed interface arrives at the SECOND implementor, following the existing `nlri.JSONAppender` precedent, not in this spec.

- [ ] `ai/rules/architecture.md` - tier placement for a new package under `internal/core/`
  → Constraint: `internal/core/describe` must import nothing from `internal/component/` or `internal/plugins/`. `core_direction_gate` in `scripts/dev/dep_audit.py` enforces it against `scripts/dev/core_import_baseline.txt`, which can only shrink. The node type must therefore be protocol-neutral, with `decode_human.go` doing all BGP-specific mapping.
  → Decision: no row is needed in `scripts/dev/tier_non_engine_categories.txt`. That manifest covers only `internal/component/` and `internal/plugins/` paths.

- [ ] `ai/rules/performance.md` - allocation rules for a helper added to `internal/core/textbuf`
  → Constraint: `textbuf` is used on hot paths (wire encoders, `slogutil.getLogEnv`), so the hex-dump helper must be allocation-free and must not use `fmt`. The existing `HexUpper`, `PadRight` and `Byte` methods cover the shape.
  → Decision: `decode_human.go` itself is NOT hot and already uses `fmt.Fprintf` heavily, so the CLI-side mapping carries no new allocation constraint.

- [ ] `ai/rules/cli.md` - output contracts for anything an operator reads
  → Constraint: a row's state is a field or a column, never a character glued to a value. The tree's box-drawing characters are structure, not state markers on values, so they are legitimate; no value may gain a sigil.
  → Decision: no new command is added, so no CLI grammar gate applies and `make ze-cli-grammar-check` needs no new run.

- [ ] `docs/guide/logging.md` - the operator-facing logging surface this spec extends
  → Constraint: the subsystem table carries a `source:` anchor pointing at `subsystemDescriptions`, consumed by `scripts/dev/code_to_docs.py`, so a new subsystem must appear in both.
  → Decision: this page currently prints `bgp log set ...`, which is not a command. It is fixed here because this spec edits the page anyway. Recorded in `plan/journal/documentation-shows-config-the-parser-refuses.md`.

**Key insights:** (minimal context to resume after compaction)

- The tree the user asked for exists only in Python. This is new Go work, not exposure of an existing renderer.
- The decoder already exists and already resolves plugin codecs. Only its human output changes.
- The runtime operation already exists: `request log level <logger> <level>`, non-persistent. No new command.
- Terminal versus syslog is already solved by `environment/log/{backend,destination}`. Emitting through slog gets both for free.

## Current Behavior (MANDATORY)

**Source files read:** (verified at the producer, 2026-08-15)

- [ ] `internal/component/bgp/cli/decode.go` - `cmdDecode` parses flags and one hex argument. `decodeHexPacket` normalises hex, detects the 16-byte marker, dispatches per message type, and returns either a Ze JSON envelope or human text. `detectMessageType` reads the type byte; `hasValidMarker` decides whether a header is present
- [ ] `internal/component/bgp/cli/decode_human.go` - `formatOpenHuman`, `formatCapabilityHuman`, `formatUpdateHuman`, `formatAttributesHuman`, `formatNLRIHuman` and siblings write flat two-space lines into a `textbuf.Buffer` from a `map[string]any`, using repeated type assertions
- [ ] `internal/component/bgp/cli/register.go` - registers root `bgp`, local `show bgp decode` and `show bgp encode`, and publishes `decodeHexPacket` through `pluginreg.SetPacketDecoder`
- [ ] `test/exabgp-compat/bin/bgp` - `print_payload` builds the tree inline and calls `format_hex_block`; the only implementation of the target look, and it is Python
- [ ] `internal/component/bgp/reactor/reactor_notify.go` - `notifyMessageReceiver` is the single point both directions pass through; iterates `r.msgObservers` unconditionally; feeds `r.capture` and `r.rawCapture`; reads `rawBytes[0]` and `rawBytes[1]` as the NOTIFICATION error code and subcode
- [ ] `internal/component/bgp/reactor/reactor.go` - `MessageObserver` interface, `addMessageObserver`, exported `AddMessageCallback`; the observer runs synchronously on the session read goroutine and must not block
- [ ] `internal/component/bgp/reactor/session_read.go` - passes `body` with `hdr.Type` as a separate argument, which is why the observer never sees a header
- [ ] `internal/core/slogutil/slogutil.go` - `SetLevel` loads the subsystem's `*slog.LevelVar` from the in-memory `levelRegistry` and returns `unknown subsystem` when absent; `subsystemDescriptions` is a plain map whose only reader is `Subsystems()`; `LazyLogger` registers into `levelRegistry` only inside its `once.Do`
- [ ] `internal/core/slogutil/filter.go` - `filterHandler.Enabled` delegates to the base handler, which compares the `*slog.LevelVar`; this is the fast path
- [ ] `internal/core/slogutil/syslog.go` - `newSyslogHandler`, selected by `environment/log/{backend,destination}`
- [ ] `internal/plugins/log/yang/ze-log-cmd.yang` - registers `request log level <logger> <level>` and `show log levels`
- [ ] `internal/core/textbuf/textbuf.go` - pooled `Get`/`Release`, and `Str`, `Byte`, `HexUpper`, `PadLeft`, `PadRight`, `Repeat` on the buffer

**Behavior to preserve:**

- The Ze JSON envelope from `decodeHexPacket` is a shipped contract. `--json` output must not change by one byte.
- `ze bgp decode`, `show bgp decode` and the web tool page keep working through the same entry points and the same seam.
- `test/ui/web-tool-decode.ci` asserts `contains=origin` against the human render; the literal lowercase token must survive.
- KEEPALIVE renders the literal string `KEEPALIVE`.
- Every observer already registered on the reactor keeps receiving the body it receives today.

**Behavior to change:**

- Human output becomes an ordered tree with a hex dump, replacing the flat lines.
- A new `bgp.wire` subsystem emits the same rendering for live traffic when its level is at debug.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point

Three, all existing:

- `ze bgp decode <hex>` and `show bgp decode <hex>`: a hex string on the command line.
- The web tool page: a POST body reaching `decodeHexPacket` through `pluginreg.GetPacketDecoder`.
- Live BGP traffic: a message arriving at or leaving the reactor.

### Transformation Path

1. Offline: hex text, normalised and decoded to bytes by `decodeHexPacket`.
2. Offline: bytes to `map[string]any` by the per-type decoders, unchanged.
3. New: `map[string]any` to an ordered node tree, in the retargeted `decode_human.go`.
4. New: node tree to text by the renderer in `internal/core/describe`.
5. New: raw bytes to a hex-dump block by the helper in `internal/core/textbuf`.
6. Runtime: a BGP message reaches `notifyMessageReceiver`, which iterates observers. The new observer checks the level, reconstructs the 19-byte header, and runs stages 2 to 5.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI to decoder | one hex string argument, existing | No |
| Web handler to decoder | `pluginreg` packet-decoder seam, existing | No |
| Reactor to observer | `MessageObserver.OnBGPMessage`, synchronous on the session read goroutine | No |
| Observer to logger | `slog` record under subsystem `bgp.wire` | No |
| Logger to sink | existing `environment/log/{backend,destination}`, stderr or syslog | No |

### Integration Points

- `decodeHexPacket` keeps its signature and its two output modes; only the human branch changes.
- `Reactor.AddMessageCallback` is the registration point for the new observer; `internal/plugins/mrt` is the working precedent.
- `subsystemDescriptions` gains one row for `bgp.wire`.

### Architectural Verification

| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | No | |
| No unintended coupling | No | |
| No duplicated functionality | No | |
| Zero-copy preserved where applicable | No | |
| Registration over hardcoding | No | |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The 19-byte header is byte-exactly reconstructible at the tap from data already in scope. | RFC 4271 requires the marker to be all ones; length is 19 plus body length; type is the `msgType` argument to the observer. | The tap's hex dump shows bytes that never crossed the wire, and the feature misleads rather than informs. | A test comparing a reconstructed header against the bytes `teeCapture` receives for the same message. | unvalidated |
| A-2 | No `.ci` other than `test/ui/web-tool-decode.ci` asserts on human decode output. | All 39 files in `test/decode/*.ci` pass `--json` and assert `expect=json:json=`; a grep for files lacking `--json` returned none. | The retarget breaks functional tests not accounted for, and the change grows past its estimate. | Re-run the grep at implementation time, then `make ze-functional-decode-test` and `make ze-functional-ui-test`. | unvalidated |
| A-3 | An `Enabled()` guard keeps the disabled cost to one interface call plus a level compare, with no allocation. | `filterHandler.Enabled` delegates to the base handler's `*slog.LevelVar` compare; the precedent is `observeForwardHandles` in the rib plugin. | The tap costs allocations on the forwarding path for every operator who never turns it on. | An allocation test asserting zero allocations per message with the subsystem above debug. | unvalidated |
| A-4 | `map[string]any` from the existing decoders carries every field the tree must show, in recoverable form. | `decode_human.go` renders from it today and reaches capabilities, attributes and NLRI. | Some field is only reachable from the wire bytes, and the node mapping needs decoder changes this spec did not budget. | Build the node tree for one OPEN and one UPDATE fixture and diff the field set against the Python harness output. | unvalidated |
| A-5 | Field order can be fixed in the mapping layer without touching the decoders. | Ordering is a rendering concern; the mapping code chooses the order it appends nodes. | The decoders must change to preserve order, widening blast radius into the shipped JSON path. | Write the mapping for OPEN and confirm the order is chosen there, not inherited. | unvalidated |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | `SetLevel` answers `unknown subsystem: bgp.wire` because nothing has called `Logger("bgp.wire")` yet, so the operator cannot turn the feature on until the first BGP message. | `request log level bgp.wire debug` fails on a freshly started daemon; `show log levels` omits the row. | Touch the logger at reactor construction so registration happens before any operator command. AC-6 covers it. |
| R-2 | The subsystem resolves to `disabled` from the environment, so `Logger` returns the discard handler and never registers, and `SetLevel` fails permanently. | The row is missing from `show log levels` even after traffic. | Name the failure in the spec and test the default-environment path. Do not ship a subsystem whose only enablement route can be locked out. |
| R-3 | Rendering a tree per message on a full-table peer overwhelms the log sink, and syslog worst of all. | Log volume or daemon CPU rises sharply when the level is set on a busy peer. | The level is per subsystem and non-persistent, and the operator opts in. Documented as a diagnostic aid, matching how `capture` is documented. Per-peer narrowing is not part of this spec and is listed under Known Limitations. |
| R-4 | The observer blocks the session read goroutine while formatting, slowing the read path when enabled. | Session read latency or hold-timer expiry under load with the level at debug. | Keep the render allocation-bounded and behind the `Enabled()` guard. Measure with the level at debug in the functional test. |
| R-5 | Retargeting changes human output in a way `test/ui/web-tool-decode.ci` catches late, after the mapping is written. | That `.ci` fails at the end of the work rather than the start. | Preserve the `origin` literal by design, and run `make ze-functional-ui-test` in the same phase as the retarget, not at the end. |
| R-6 | `spec-improve-3-event-replay` is in-progress and touches the same reactor capture area, so the two collide. | A merge conflict in `reactor_notify.go` or `raw_capture.go`. | That spec explicitly calls `BGPRawCaptureRing` "adjacent, not reusable" for its JSONL path, and this spec adds an observer rather than touching the ring. Check its status before starting. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Human decode output, which is read by operators and by one `.ci`. The JSON envelope, the plugin IPC format and the wire path are untouched. When the runtime tap is wrong it costs session-read latency on daemons where an operator enabled it. |
| How is it reverted? | Single commit revert. Nothing persists, no config migration, nothing reaches a peer. |
| Who else touches this path? | `spec-improve-3-event-replay` (in-progress) in the reactor capture area; `spec-interop-wire-capture` (skeleton) will want to consume a decoder from the interop harness. |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `ze bgp decode --open <hex>` | → | the node mapping in `decode_human.go` and the renderer in `internal/core/describe` | `TestDecodeOpenRendersOrderedTree` |
| A POST to the web tool page | → | `decodeHexPacket` through `pluginreg.GetPacketDecoder` | `test/ui/web-tool-decode.ci` |
| `request log level bgp.wire debug` then a BGP message arrives | → | the new reactor message observer | `test-bgp-wire-decode-log.ci` |
| `show log levels` on a freshly started daemon | → | subsystem registration at reactor construction | `TestBGPWireSubsystemRegisteredBeforeFirstMessage` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze bgp decode --open ffffffffffffffffffffffffffffffff001d0104fde8005a0102030400` | Output carries the message type with its byte count, one indented child line per field using box-drawing prefixes with the last child distinguished, and a hex-dump block of offset, hex bytes and an ASCII gutter |
| AC-2 | The same input decoded twice in one process and across two processes | Output is byte-identical every time; no field order varies |
| AC-3 | `ze bgp decode --json` for every fixture in `test/decode/` | Output is byte-identical to the output before this change |
| AC-4 | The web tool page decodes an UPDATE carrying an ORIGIN attribute | The rendered text contains the literal lowercase token `origin`, and `test/ui/web-tool-decode.ci` passes unchanged |
| AC-5 | `ze bgp decode --keepalive <hex>` | Output is the literal `KEEPALIVE` |
| AC-6 | `request log level bgp.wire debug` on a daemon that has received no BGP message | Succeeds, and `show log levels` lists `bgp.wire` with a description |
| AC-7 | `bgp.wire` at debug, a peer sends an UPDATE and ze sends one | Both directions emit the decoded tree through the logger, each identifying its peer and its direction |
| AC-8 | `bgp.wire` above debug, 1000 messages pass through the reactor | No tree is rendered and the observer allocates zero bytes per message |
| AC-9 | `bgp.wire` at debug and `environment/log/backend` set to syslog | The same rendering reaches the syslog sink |
| AC-10 | A message whose body was truncated or malformed such that the decoder errors | The tap emits the hex dump and a decode error, never a partial tree presented as complete, and never a panic on the session read goroutine |
| AC-11 | The reconstructed 19-byte header for a given message | Byte-identical to the header those bytes carried on the wire, compared against what `teeCapture` receives |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | pastes a hex BGP message into `ze bgp decode` and reads it like wireshark | argv → `decodeHexPacket` → per-type decoder → node mapping → tree renderer + hex dump → stdout | `TestDecodeOpenRendersOrderedTree` |
| 2 | opens the web tool page and decodes a message | POST → `pluginreg.GetPacketDecoder` → same chain → HTML | `test/ui/web-tool-decode.ci` |
| 3 | turns on decoded logging on a live daemon and watches messages in the terminal | `request log level bgp.wire debug` → `SetLevel` → observer `Enabled()` passes → same chain → stderr | `test-bgp-wire-decode-log.ci` |
| 4 | points the daemon's log backend at syslog and collects decoded BGP centrally | same chain → `newSyslogHandler` | `test-bgp-wire-decode-log.ci` with the syslog backend |
| 5 | turns the level back down and pays nothing for it | `request log level bgp.wire info` → observer `Enabled()` fails → return | `TestWireObserverZeroAllocWhenDisabled` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestTreeRendersChildPrefixes` | `internal/core/describe/describe_test.go` | box-drawing prefixes, last-child distinction, nesting depth | |
| `TestTreeEmptyAndSingleChild` | `internal/core/describe/describe_test.go` | a node with no children and one with exactly one child both render correctly | |
| `TestTreeDeterministicOrder` | `internal/core/describe/describe_test.go` | node order is preserved exactly as appended | |
| `TestHexDumpBlock` | `internal/core/textbuf/textbuf_test.go` | offset column, hex grouping, ASCII gutter, non-printable substitution | |
| `TestHexDumpZeroAlloc` | `internal/core/textbuf/textbuf_test.go` | the helper allocates nothing beyond the caller's buffer | |
| `TestDecodeOpenRendersOrderedTree` | `internal/component/bgp/cli/decode_test.go` | AC-1 and AC-2 for OPEN | |
| `TestDecodeUpdateRendersOrderedTree` | `internal/component/bgp/cli/decode_test.go` | AC-1 and AC-2 for UPDATE, including attributes and NLRI | |
| `TestDecodeJSONUnchanged` | `internal/component/bgp/cli/decode_test.go` | AC-3, golden comparison against pre-change JSON | |
| `TestDecodeKeepaliveLiteral` | `internal/component/bgp/cli/decode_test.go` | AC-5 | |
| `TestBGPWireSubsystemRegisteredBeforeFirstMessage` | `internal/component/bgp/reactor/wire_observer_test.go` | AC-6 and R-1 | |
| `TestWireObserverZeroAllocWhenDisabled` | `internal/component/bgp/reactor/wire_observer_test.go` | AC-8 | |
| `TestWireObserverReconstructsHeader` | `internal/component/bgp/reactor/wire_observer_test.go` | AC-11 and A-1 | |
| `TestWireObserverDecodeErrorDoesNotPanic` | `internal/component/bgp/reactor/wire_observer_test.go` | AC-10 | |

### Boundary Tests (numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| BGP message length in the reconstructed header | 19-4096 | 4096 | 18 (shorter than a header) | 4097 (past the ring slot, spec-bgp-pcap-decode territory) |
| Hex-dump row width | 16 bytes | 16 | N/A | N/A |
| Bytes in the final hex-dump row | 1-16 | 16 | 0 (no trailing empty row) | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-decode-open-tree.ci` | `test/decode/` | an operator decodes an OPEN and sees the tree with its hex dump | |
| `test-decode-update-tree.ci` | `test/decode/` | an operator decodes an UPDATE and sees attributes and NLRI in a stable order | |
| `web-tool-decode.ci` (existing, must keep passing) | `test/ui/` | the web tool page still renders a decode containing `origin` | |
| `test-bgp-wire-decode-log.ci` | `test/plugin/` | an operator raises `bgp.wire` to debug on a live session and sees decoded messages in both directions | |

#### Peer-block conventions for the new `.ci` (read before authoring)

Two guards landed in the shared checkout on 2026-08-15 and were still
UNCOMMITTED when this spec was written, so `git log` will not explain them. Both
bite a newly authored peer block. Confirm they are in the tree before assuming
either applies.

| Guard | Producer | What to do when authoring |
|-------|----------|---------------------------|
| RFC 4271 Section 5.1.3: a route is withheld from a peer when the NEXT_HOP is that peer's own address | `egressNextHopIsPeerOwn` and `originatedNextHopIsPeerOwn`, `internal/component/bgp/reactor/forward_next_hop.go` | Give `connection > remote > ip` and `connection > local > ip` DIFFERENT addresses. The old suite convention used one address for both, which makes `next-hop self` resolve to the peer's own address, and the route is withheld. Use 127.0.0.1 to 127.0.0.5 on this host |
| A directive inside a `stdin=<name>:terminator=` block that no consumer claims fails at parse time, naming file, line and reason | `internal/test/runner/peer_contract.go` | Every line in a peer block must be a directive a consumer claims. Such a line used to be dropped silently, so a typo now fails loudly rather than passing vacuously |

The daemon-log signature of the first is `withholding originated route: its next
hop is this peer's own address`. If it appears, it is that guard, not this spec.

### Interop Tests (Scope: protocol)

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N/A | | | This spec changes no wire-visible behavior. It only renders bytes ze already sends and receives. `ai/rules/interop-and-goal-validation.md` requires interop only where wire behavior changes | N/A |

## Files to Modify

- `internal/component/bgp/cli/decode_human.go` - build ordered nodes instead of flat lines; DELETE the flat formatters rather than leaving them beside the new path
- `internal/component/bgp/cli/decode.go` - route the human branch through the renderer; keep the JSON branch untouched
- `internal/component/bgp/cli/decode_test.go` - update the human-output assertions, add the tree and determinism tests
- `internal/core/textbuf/textbuf.go` - add the hex-dump helper
- `internal/core/slogutil/slogutil.go` - add the `bgp.wire` row to `subsystemDescriptions`
- `internal/component/bgp/reactor/reactor.go` - register the wire observer and touch the logger at construction so the subsystem exists before the first message
- `docs/guide/logging.md` - add the `bgp.wire` row, and fix the four `bgp log set` / `bgp log levels` invocations that are not commands
- `docs/guide/command-reference.md` - the decode command's output shape changed
- `ai/INDEX.md` - the BGP wire decoding row, so the renderer is discoverable

## Files to Create

- `internal/core/describe/describe.go` - the ordered node type and the tree renderer
- `internal/core/describe/describe_test.go` - unit tests
- `internal/component/bgp/reactor/wire_observer.go` - the message observer that renders to `bgp.wire`
- `internal/component/bgp/reactor/wire_observer_test.go` - unit tests
- `test/decode/test-decode-open-tree.ci` - functional test
- `test/decode/test-decode-update-tree.ci` - functional test
- `test/plugin/test-bgp-wire-decode-log.ci` - functional test for the runtime tap

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | No new command and no new config leaf. The operation is the existing `request log level` |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | N-A | No new command or flag; `ze bgp decode` keeps its flag set |
| CLI grammar (keyword before value) | N-A | No new command, so `make ze-cli-grammar-check` gains no new surface |
| Editor autocomplete | N-A | No new leaf or command |
| Functional test for new RPC/API | Yes | `test/decode/test-decode-open-tree.ci`, `test/decode/test-decode-update-tree.ci`, `test/plugin/test-bgp-wire-decode-log.ci` |
| Pipe completeness | N-A | `ze bgp decode` is offline `cmd/ze` tooling and does not route through `ApplyPipes` today. This spec changes rendering only and does not alter that surface |
| Env var registration | N-A | No new `environment/` leaf. `bgp.wire` is reached through the existing log level machinery |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, kernel module, binary or certificate. The tap writes through the logger that already exists |
| Prometheus counters/metrics | N-A | Rendering is diagnostic output, not observable state worth a counter. A counter for suppressed renders was considered and rejected as machinery nobody asked for |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family, capability or attribute. The spec renders what is already decoded |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` (decoded BGP message rendering, offline and live) |
| 2 | Config syntax changed? | No | No YANG leaf added or changed |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, because the output shape of `ze bgp decode` and `show bgp decode` changes |
| 4 | API/RPC added/changed? | No | The packet-decoder seam keeps its signature |
| 5 | Plugin added/changed? | No | No plugin gains or loses a surface |
| 6 | Has a user guide page? | Yes | `docs/guide/logging.md`, for the `bgp.wire` subsystem and the corrected invocations |
| 7 | Wire format changed? | No | Nothing this spec touches reaches the wire |
| 8 | Plugin SDK/protocol changed? | No | `format/text_human.go` and the IPC format are not part of this spec |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Rendering proves no RFC obligation. The header reconstruction RELIES on RFC 4271's all-ones marker but enforces nothing |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`, for the three new `.ci` tests |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md`, since exabgp's decode output is the reference this matches |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` or a subsystem doc, for the new `internal/core/describe` package and the observer |
| 13 | Route metadata keys added/changed? | No | No metadata key touched |
| 14 | Prometheus counters added/changed? | No | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registration inventory changes; the observer is an internal reactor registration |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Grep `docs/` for anchors on `slogutil.go`, `decode_human.go`, `decode.go`, `reactor.go` and repoint or correct each |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/logging.md` carries four invocations that are not commands; verify every remaining example on that page against the YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- prove every entry point reaches a stub
   - Tests: `TestDecodeOpenRendersOrderedTree`, `TestBGPWireSubsystemRegisteredBeforeFirstMessage`, `test-bgp-wire-decode-log.ci`
   - Files: `internal/core/describe/describe.go` (stub), `internal/component/bgp/reactor/wire_observer.go` (stub registered on the reactor), `internal/core/slogutil/slogutil.go` (subsystem row)
   - Verify: the observer is registered, the subsystem answers `request log level bgp.wire debug` on a cold daemon, and the tests fail because the renderer is a stub
2. **Phase: the node type and the renderer** -- ordered nodes, tree rendering, hex dump
   - Tests: the `internal/core/describe` and `internal/core/textbuf` unit tests
   - Files: `internal/core/describe/describe.go`, `internal/core/textbuf/textbuf.go`
   - Verify: unit tests pass, `make ze-tier-check` accepts the new core package, the hex-dump helper allocates nothing
3. **Phase: retarget the offline decoder** -- map `map[string]any` to nodes, delete the flat formatters
   - Tests: the `decode_test.go` tree, determinism and JSON-unchanged tests, then `make ze-functional-decode-test` and `make ze-functional-ui-test`
   - Files: `internal/component/bgp/cli/decode_human.go`, `internal/component/bgp/cli/decode.go`
   - Verify: AC-1 to AC-5. Run `make ze-functional-ui-test` in THIS phase, not at the end, per R-5
4. **Phase: the runtime tap** -- header reconstruction, level guard, both directions
   - Tests: the `wire_observer_test.go` tests, then `test-bgp-wire-decode-log.ci`
   - Files: `internal/component/bgp/reactor/wire_observer.go`
   - Verify: AC-6 to AC-11, including the zero-allocation assertion and the syslog backend
5. **Phase: documentation and discovery** -- every row of the Documentation checklist
   - Files: the docs listed under Files to Modify, plus `ai/INDEX.md`
   - Verify: `make ze-doc-verify`, `make ze-doc-wiring-check`

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:symbol |
| Feature completeness | All five user stories have a working path; the syslog story is not assumed from the stderr story |
| No layering | The flat formatters in `decode_human.go` are DELETED, not left unreachable beside the new path |
| Correctness | The reconstructed header is byte-exact, not merely plausible; the decode-error path emits the hex dump rather than a partial tree |
| Determinism | No `map` iteration reaches the rendering order anywhere in the chain |
| Performance | The `Enabled()` guard precedes every argument evaluation at the log site; the hex-dump helper uses no `fmt` |
| Rule: `ai/rules/simplicity.md` | No `Describe()` was added to the wire interfaces; the renderer takes a node and nothing more |
| Rule: `ai/rules/architecture.md` | `internal/core/describe` imports nothing from component or plugins |
| Rule: `ai/rules/evidence.md` | The observer fails closed: a decode error is reported, never silently skipped |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The ordered node type and renderer | `make ze-unit-pkg-test PKG=./internal/core/describe` |
| The hex-dump helper | `make ze-unit-pkg-test PKG=./internal/core/textbuf` |
| Flat formatters gone | `grep -n 'formatOpenHuman\|formatAttributesHuman' internal/component/bgp/cli/` returns only the new node-building forms |
| JSON unchanged | `make ze-functional-decode-test` |
| The web tool page still works | `make ze-functional-ui-test` |
| The runtime tap | `make ze-functional-plugin-test` for `test-bgp-wire-decode-log.ci` |
| Core tier respected | `make ze-tier-check` |
| Docs and discovery | `make ze-doc-verify`, `make ze-doc-wiring-check` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | The offline decoder takes operator-supplied hex. A truncated or malformed message must not index past the buffer while building nodes or the hex dump |
| Resource exhaustion | An attacker-controlled peer cannot raise the log level, but once an operator has, a high-rate peer drives rendering. Confirm the render is bounded per message and the guard is checked first |
| Information disclosure | The tap renders full message bytes into the log, and on syslog those leave the host. BGP messages carry routing data, not local secrets: TCP-MD5 keys never appear on the wire. State this in the `docs/guide/logging.md` row so an operator knows what they are exporting, matching how the `capture` container documents the same trade |
| Error leakage | A decode error must name the failure without echoing unbounded attacker-controlled bytes into the error string |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| `make ze-tier-check` rejects `internal/core/describe` | The node type reached for a component or plugin import. Remove it; mapping belongs in `decode_human.go` |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

<!-- LIVE: write immediately when you learn something. -->

- The output that prompted this work was never ze's. It came from `print_payload` in the Python exabgp-compat harness. A feature request framed as "expose what we already print" was in fact "build what only the test harness has".
- The generalisation the request reached for was real but pointed the wrong way: not across BGP components, but across protocols. Four offline hex decoders exist (bgp, isis, ospf, l2tp), each with its own output shape, and `isHexString` is copy-pasted verbatim between two of them.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| An ordered node type plus one renderer | A `Describe()` method on `message.Message`, `attribute.Attribute` and `capability.Capability`, as originally proposed | Those are `WriteTo`-shaped hot-path contracts over pooled, lazily-parsed data. A mandatory method would put allocation on the forwarding path and force every NLRI codec plugin to implement rendering. It also cannot reach the existing decoder, which works from bytes to `map[string]any` and never constructs those types |
| No `Describer` interface yet | Adding the optional probed interface now | `ai/rules/simplicity.md` refuses an interface with one implementation. The precedent for adding it later is `nlri.JSONAppender`, an optional interface probed by the JSON formatter |
| `internal/core/describe`, protocol-neutral | `internal/core/bgp/describe`, BGP-specific | Four protocols have offline decoders with divergent output. A neutral node type is what lets the other three converge later without a second renderer |
| Reconstruct the 19-byte header at the tap | Widen `MessageCallback` with a wire parameter; move the capture beside `teeCapture` | Reconstruction is byte-exact from data already in scope (all-ones marker per RFC 4271, length from the body, type from the argument) and touches no call site. Widening the callback changes six-plus sites and one caller has no wire bytes to give. Owner decision, 2026-08-15 |
| Reuse `request log level` | A new `debug bgp decode` command; a per-peer YANG config leaf | The operation already exists and is already non-persistent, which is what the owner asked for. `ai/rules/cli.md` also reserves the `debug` verb for perturbing protocol state and says verbose logging is not that. Emitting through slog additionally makes terminal and syslog fall out of `environment/log/{backend,destination}` at no cost |
| A `MessageObserver`, not a direct call in `notifyMessageReceiver` | Calling the renderer inline beside the existing capture branches | The observer seam exists, is iterated already, and has a working precedent in `internal/plugins/mrt`. An inline branch would add a per-feature condition to a shared reactor function, which `ai/rules/plugins.md` calls out |

## Known Limitations

- The Python harness (`test/exabgp-compat/bin/bgp`) keeps its own tree implementation and can drift from the Go one. It cannot call Go, and pinning the two with a shared fixture was judged more machinery than the drift is worth. Revisit if they diverge visibly.
- The runtime tap is per subsystem, not per peer. On a daemon with one busy peer and one interesting peer, an operator sees both. A per-peer narrowing is a separate spec, and the existing `capture` container is the precedent for how it would look.
- `format/text_human.go`, `monitor/format.go` and `internal/test/decode/decode.go` keep their own renderings. This spec reduces five renderers to four; converging the rest needs their contracts examined one at a time.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `make ze-precommit-verify` passes. It is the pre-commit gate (`ai/rules/git-safety.md`)
- [ ] Feature code integrated (`internal/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `scripts/dev/review_gate.py`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)

## Review Gate

<!-- Filled by /ze-close via /ze-review. Do not delete this section. -->

| Run | Date | Blockers | Issues | Result |
|-----|------|----------|--------|--------|
| | | | | |
