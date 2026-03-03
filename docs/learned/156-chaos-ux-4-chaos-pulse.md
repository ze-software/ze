# 156 — Chaos UX 4: Chaos Pulse Animation

## Objective

Add a CSS pulse animation (radial glow, 0.5s, single iteration) to peer grid cells when a chaos event affects that peer, using a transient `ChaosActive` flag on PeerState.

## Decisions

- `ChaosActive` is a transient one-render-cycle flag: set in `ProcessEvent()` (write lock), read by `renderPeerCell()` (read lock), cleared by `broadcastDirty()` after the render pass (write lock). The clearing happens AFTER rendering — if cleared before, the render cycle won't see the flag.
- Four event types trigger the pulse: `EventChaosExecuted`, `EventDisconnected`, `EventError`, `EventReconnecting`. Route events do not trigger it.
- Pulse class is only added to grid cells, never table rows — table rendering is unchanged.
- CSS `animation-iteration-count: 1` ensures the glow plays once per HTMX `outerHTML` swap; swapping re-adds the element, retriggering the animation naturally.

## Patterns

- Transient render flag pattern: set on event → survive ConsumeDirty → used in render → cleared after render. Follow this exact sequence for any future "one-shot visual effect" on grid cells.

## Gotchas

- Clearing must happen AFTER `renderPeerCell()` runs, not before. The ConsumeDirty call clears dirty flags but `ChaosActive` must survive it to reach the render phase — they are cleared in separate passes.

## Files

- `cmd/ze-chaos/web/state.go` — `ChaosActive` bool on `PeerState`
- `cmd/ze-chaos/web/dashboard.go` — set in ProcessEvent, pulse class in renderPeerCell, clear in broadcastDirty
- `cmd/ze-chaos/web/assets/style.css` — `.pulse` class with `chaos-pulse` keyframe animation
