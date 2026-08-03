# Deferrals: rib-arch-7

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-15 | rib-arch-7 (DONE, learned 1126) | RESIDUAL ONLY: the live multi-peer BGP `.ci` for the LLGR readvertise divergence. The egress-filter wiring into `AnnounceNLRIBatch` IS IMPLEMENTED (`filterapi.Filter.Readvertise` + `sendStaleReadvertise`/`decideStaleReadvertise`, `reactor_api_batch.go`), covered byte-for-byte by `TestStaleReadvertiseWireOutput`. | DELIVERED `test/plugin/llgr-readvertise-multipeer.ci` (route-reflector topology; observer plugin drives `mark-stale` + `clear bgp rib out`; obsn asserts the reflected route then the NO_EXPORT depreference on the wire; 3/3 deterministic ~6s). Root cause of the earlier stall was NOT OPEN establishment but the ze-peer lifecycle: a peer closes its TCP the instant its send-script is exhausted (`internal/test/peer/peer.go` `SequenceEnded`), firing GR before the forward propagates; fixed with the rr-basic EOR-wait pattern + observer-driven (not disconnect-driven) stale transition. | `plan/learned/1126-rib-arch-7-llgr-multipeer-readvertise.md` | resolved |

