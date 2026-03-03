# 155 — Chaos UX 3: Event Toast Notifications

## Objective

Add brief toast notifications (slide-in, 5s auto-dismiss via CSS animation) for chaos events (disconnect, reconnect, error, chaos executed) in the ze-chaos dashboard.

## Decisions

- Toast queue bounded at 5 on `DashboardState` — oldest dropped when limit exceeded. Server-side bound prevents DOM accumulation without requiring JS cleanup.
- Toasts accumulated in `ProcessEvent()` (write lock), flushed in `broadcastDirty()` (write lock to consume, no lock needed to broadcast) — follows existing dirty flag pattern.
- CSS-only auto-dismiss: slide-in (0.3s) + fade-out starting at 4.5s. No JS timers. Once the animation ends the element is visually gone; server-side DOM cleanup is not needed.
- `hx-swap-oob="beforeend:#toast-container"` appends each toast to the container — enables multiple toasts per broadcast tick without replacing existing ones.

## Patterns

- `toastForEvent()` pure function mapping event type → ToastEntry with color class. Keeps mapping table in one place rather than scattered switch cases.

## Gotchas

None.

## Files

- `cmd/ze-chaos/web/state.go` — `ToastEntry`, `QueueToast()`, `ConsumePendingToasts()`
- `cmd/ze-chaos/web/dashboard.go` — `ProcessEvent` queuing + `broadcastDirty` flushing
- `cmd/ze-chaos/web/render.go` — `renderToast()`, `toastForEvent()`, toast container in writeLayout
- `cmd/ze-chaos/web/assets/style.css` — toast card styles, slide-in + fade-out keyframes
