# 299 — Text Event Format

## Objective

Eliminate JSON serialization overhead on the event delivery hot path: fix FormatHex/FormatRaw switch fall-through bug, allow plugins to opt into text encoding for events, and convert bgp-rr to text parsing (reducing ~6 `json.Unmarshal` calls per UPDATE to `strings.Fields` lookups).

## Decisions

- Text formatters already existed for ALL event types (`formatFilterResultText`, `FormatOpen`, `FormatNotification`, `FormatKeepalive`, `FormatRouteRefresh`, `FormatStateChange`) — no new formatting code needed, only wiring into event delivery
- `Process.Format()` default changed from FormatHex to FormatParsed — the FormatHex default was a latent bug compensated by switch fall-through (FormatHex fell through to FormatParsed anyway); changing the constant to match historical behavior was the correct fix
- Pre-format cache keys include format+encoding (not format alone) — two processes with same format but different encoding must get distinct cached outputs
- `onPeerNegotiated` kept JSON-only — infrequent event, no plugin opts into text for it
- External plugins default to JSON encoding (`Process.Encoding()` returns "json") — text is opt-in via `SetEncoding("text")`

## Patterns

- `bgp-rr` opts in: `p.SetEncoding("text")` before `Run()`, then `dispatchText` replaces `dispatch`, all JSON parsing functions replaced with `strings.Fields`-based equivalents
- `forwardCtx.bgpPayload` → `forwardCtx.textPayload` (raw text line(s) instead of inner JSON object)
- `SubscribeEventsInput.Encoding` field carries plugin's encoding preference to engine; `registerSubscriptions` applies it per-process

## Gotchas

- FormatHex/FormatRaw mismatch: `FormatHex = "hex"` and `FormatRaw = "raw"` are separate constants. Adding FormatHex to the switch case was correct, BUT required also fixing `Process.Format()` default from FormatHex to FormatParsed — otherwise ALL default-format processes would get raw hex output instead of parsed JSON. Discovered when `check.ci` failed after the switch fix.
- Lesson: when fixing a switch fall-through, check what callers rely on the old (broken) behavior — the fall-through was load-bearing for the wrong default

## Files

- `internal/plugins/bgp/format/text.go` — FormatHex case added in `formatFromFilterResult`
- `internal/plugins/bgp/server/events.go` — encoding-aware event delivery, per-process state events
- `internal/plugin/process.go` — default format changed to FormatParsed
- `internal/plugins/bgp-rr/server.go` — text parsing, `dispatchText`, `forwardCtx.textPayload`
- `internal/plugin/server_dispatch.go` — Encoding in `registerSubscriptions`
- `pkg/plugin/rpc/types.go` — Encoding field in `SubscribeEventsInput`
- `pkg/plugin/sdk/sdk.go` — `SetEncoding()` method
