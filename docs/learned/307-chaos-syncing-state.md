# 307 — Chaos Syncing State

## Objective

Add a `PeerSyncing` status to the chaos web dashboard representing the initial route transfer phase between Established and first EOR, making the RIB loading phase visible to operators.

## Decisions

- `PeerSyncing` appended after `PeerReconnecting` (not inserted between existing values) — inserting would shift iota values of PeerDown and PeerReconnecting, breaking integer comparisons and stored state
- Color: `#58a6ff` (existing `--accent` CSS variable, cyan/blue) — distinct from green (up), red (down), yellow (reconnecting), gray (idle); semantically neutral-to-positive
- `PeersUp` increments on EOR (not Established) — a new `PeersSyncing` counter tracks syncing peers; counter invariant: `PeersUp + PeersSyncing + down + reconnecting + idle = PeerCount`
- `EventDisconnected`/`EventError`/`EventReconnecting` handlers must guard for `PeerSyncing` in addition to `PeerUp` — otherwise counters go negative

## Patterns

- All counter transitions that leave a state decrement that state's counter; all that enter increment — both sides of every transition must be handled
- EOR on an already-up peer is a no-op for status (EOR tracking still updates EORSeen/EORCount/SyncDuration)

## Gotchas

- The spec's Implementation Summary section was left empty (to be filled) — the spec was moved to done without completing the audit tables. The design section is complete and correct.

## Files

- `cmd/ze-chaos/web/state.go` — PeerSyncing enum value, PeersSyncing counter, String()/CSSClass() updated
- `cmd/ze-chaos/web/dashboard.go` — ProcessEvent transitions: Established→Syncing, EOR→Up, disconnect/error/reconnect guards for PeerSyncing
- `cmd/ze-chaos/web/viz.go` — statusColor() PeerSyncing case, all-peers totalSyncing counter
- `cmd/ze-chaos/web/render.go` — stats card syncing count, header format
- `cmd/ze-chaos/web/assets/style.css` — .status-syncing with cyan color
