# 350 — GR Receiving Speaker (RFC 4724)

## Objective

Implement Graceful Restart Receiving Speaker procedures (RFC 4724 Section 4.2) as a bgp-gr plugin, with engine-side generic enhancements for EOR detection, event formatting, and inter-plugin coordination.

## Decisions

- GR logic lives in bgp-gr plugin, not the engine — user explicitly corrected initial approach
- State machine uses string-based family names (not `family.Family`) since plugin operates on formatted JSON events
- `afiSAFIToFamily` delegates to `family.Family.String()` — single source of truth eliminates impedance mismatch
- Inter-plugin coordination via `DispatchCommand("rib retain-routes <peer>")` / `release-routes`
- `time.AfterFunc` for restart timers (simpler than clock interface for plugin context)
- Nil SDK guard in `dispatchRIBCommand` for unit testability without full SDK wiring

## Patterns

- Plugin event subscription: `SetStartupSubscriptions(events, peers, format)` with `OnEvent` callback
- JSON envelope parsing: `envelope["bgp"].(map[string]any)` → `message.type` dispatch
- GR capability extraction: find `code:64` in OPEN capabilities, hex-decode wire bytes via `decodeGR`
- State machine lifecycle: `onSessionDown` → `onSessionReestablished` → `onEORReceived` → complete
- Engine event ordering: OPEN arrives before state-up (FSM processes OPEN in OpenConfirm, transitions to Established)
- Dependency-ordered delivery: `sortByReverseDependencyTier()` ensures bgp-gr processes before bgp-rib

## Gotchas

- `afiSAFIToFamily` hand-maintained map had SAFI 4 as "mpls" but `family.SAFI.String()` returns "mpls-label" — caused silent EOR mismatch for MPLS-labeled families
- L2VPN VPLS is AFI=25/SAFI=65, NOT SAFI=72 (which is `SAFIBGPLinkStateVPN`) — easy to confuse in the IANA registry
- `family.Family.String()` lacked a special case for `{AFIBGPLS, SAFIBGPLinkStateVPN}` → produced "bgp-ls/72" instead of "bgp-ls/bgp-ls-vpn", breaking round-trip with `ParseFamily`
- Five SAFIs (mvpn, vpls, mup, rtc, flow-vpn) were missing from the hand-maintained map — all eliminated by delegating to `family.Family.String()`
- Auto-linter hook blocks edits where types appear unused — must add type definitions AND their callers in the same edit

## Files

- `internal/component/bgp/plugins/bgp-gr/gr.go` — event handling, capability extraction, inter-plugin dispatch
- `internal/component/bgp/plugins/bgp-gr/gr_state.go` — RFC 4724 state machine (timers, stale tracking)
- `internal/component/bgp/plugins/bgp-gr/gr_state_test.go` — 15 state machine tests
- `internal/component/bgp/plugins/bgp-gr/gr_event_test.go` — 10 event handling tests
- `internal/component/bgp/nlri/nlri.go` — added SAFIBGPLinkStateVPN to SAFI.String() and Family.String()
- `internal/component/bgp/server/events.go` — EOR detection, sequential state delivery, dependency ordering
- `internal/component/bgp/server/event_dispatcher.go` — OnEORReceived dispatch
- `internal/component/bgp/format/text.go` — FormatEOR function
- `internal/component/bgp/wireu/wire_update.go` — IsEOR() method
- `internal/component/bgp/plugins/bgp-rib/rib_commands.go` — retain-routes, release-routes handlers
- `internal/component/bgp/plugins/bgp-gr/register.go` — Dependencies field
