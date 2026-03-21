# 406 -- LLGR State Machine

## Objective

Extend the GR state machine to support the LLGR period: GR-to-LLGR transition on restart-time expiry, per-family LLST timers, session re-establishment during LLGR, and skip-GR path (restart-time=0 with LLST>0).

## Decisions

- LLGR state machine is in `gr_state.go` (474 lines total, shared with GR state) -- same file because GR and LLGR are a single lifecycle
- Callback-based event flow: state manager fires `onLLGREnter`, `onLLGRFamilyExpired`, `onLLGREntryDone`, `onLLGRComplete` after releasing the lock -- prevents deadlock when callbacks dispatch rib commands
- Ownership guard pattern: `handleLLSTExpired()` uses an `owner` parameter to detect stale callbacks from a previous GR/LLGR cycle (consecutive restart handling)
- `grPeerState` extended with `inLLGR` flag, `llgrFamilies` map (per-family LLST timers), `llgrCap` storage

## Patterns

- Timer interaction is serial: GR first, then LLGR. `enterLLGRLocked()` starts per-family LLST timers after GR timer expires
- Skip-GR: if restart-time=0 and LLGR negotiated, `onSessionDown` enters LLGR immediately (lines 140-147 of gr_state.go)
- Per-family independence: each family has its own LLST timer; expiry of one does not affect others
- Session re-establishment during LLGR: `onSessionReestablished()` validates both GR and LLGR caps in new OPEN, checks F-bits, updates LLGR cap
- Pending actions pattern: `enterLLGRLocked()` collects actions (delete NO_LLGR, attach LLGR_STALE, mark stale level 2) and returns them -- caller dispatches after lock release

## Gotchas

- LLGR callbacks (`onLLGREnter` etc.) in `gr.go` dispatch inter-plugin commands like `rib attach-community` and `rib delete-with-community` -- these are generic RIB commands, not LLGR-specific ones (spec designed `rib enter-llgr` but implementation uses composable building blocks instead)
- `rib mark-stale` accepts optional `level` parameter for LLGR (stale level 2) -- more general than a separate `rib depreference-stale` command
- Consecutive restart guard is critical: if a peer bounces again during LLGR, old LLST timer callbacks must be invalidated
- On LLGR entry done, `rib clear out !peer` triggers readvertisement of stale routes with LLGR_STALE community

## Files

- `internal/component/bgp/plugins/gr/gr_state.go` -- LLGR state machine (enterLLGRLocked, handleLLSTExpired, onSessionReestablished)
- `internal/component/bgp/plugins/gr/gr.go` -- LLGR callbacks (onLLGREnter, onLLGRFamilyExpired, onLLGREntryDone, onLLGRComplete)
- `internal/component/bgp/plugins/gr/gr_event_test.go` -- TestHandleEventOpenLLGR, TestHandleEventOpenLLGR_NoGR
