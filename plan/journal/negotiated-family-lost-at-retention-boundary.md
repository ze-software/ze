| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-04 | - | `onSessionDownDeferred`, `internal/component/bgp/plugins/gr/gr_state.go` | Empty GR families return before LLGR retention. RFC 9494 Section 4.2: "After the session goes down, and before the session is re-established, the stale routes for an AFI/SAFI MUST be retained." | Open, source-traced only. RFC9494-4.2-1/-2 remain weak. Retain LLGR-only families with GR time zero and prove session-down/RIB behavior in separately scoped work. |
