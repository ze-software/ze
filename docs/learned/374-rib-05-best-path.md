# 374 — Best-Path Selection (Loc-RIB)

## Objective

Add RFC 4271 Section 9.1.2 best-path selection to the bgp-rib plugin, computed on-demand via CLI commands rather than maintained in real-time.

## Decisions

- **On-demand, not real-time:** Best-path computed at query time via `rib show best` — no persistent Loc-RIB table. Real-time deferred until export policy exists (YAGNI).
- **Direct subscription over dispatch-command:** Subscribed bgp-rib to `"update direction received"` directly instead of routing notifications through bgp-adj-rib-in via DispatchCommand. Simpler, no inter-plugin coupling.
- **Pure functions for comparison:** `ComparePair` and `SelectBest` operate on `Candidate` structs with no pool dependency. Pool handle extraction happens in `extractCandidate` only.
- **Numeric IP comparison:** BGP Identifier and peer address are 32-bit unsigned integers per RFC 4271. String comparison gives wrong results across digit-count boundaries (e.g., "9.x" vs "10.x"). Use `net.ParseIP` + `bytes.Compare`.
- **Pool review: no changes needed.** Analysis confirmed NLRI is already a map key in FamilyRIB (not pooled). Attribute pools are clean.

## Patterns

- **Dead code activation:** `handleReceivedPool` existed but never fired because bgp-rib only subscribed to `"update direction sent"`. Adding the subscription activated existing code.
- **PeerMeta extraction:** Received UPDATE events carry `ASN.Local` and `ASN.Peer` in nested peer format — needed for eBGP/iBGP step 5 comparison.
- **PeerMeta cleanup:** Must delete `peerMeta[peer]` on peer-down (non-GR), same as `ribInPool[peer]`. Otherwise memory leak.

## Gotchas

- **IP string comparison is wrong for BGP:** "9.0.0.1" > "10.0.0.1" lexicographically but numerically 9 < 10. Always use `compareAddrs()` for Router ID / peer address tiebreak.
- `newTestRIBManager()` must initialize `peerMeta: make(map[string]*PeerMeta)` — nil map panic otherwise.
- Protocol test (`protocol_test.go`) hardcodes exact command count and subscription list — must update when adding new commands or subscriptions.
- Pre-existing JSON keys used snake_case (`adj_rib_in`, `routes_in`). Fixed to kebab-case per `json-format.md` convention.

## Files

- `internal/component/bgp/plugins/bgp-rib/bestpath.go` — Candidate struct, SelectBest, ComparePair, compareAddrs, asPathLength, firstASInPath
- `internal/component/bgp/plugins/bgp-rib/bestpath_test.go` — 16 tests covering all RFC steps + numeric IP comparison
- `internal/component/bgp/plugins/bgp-rib/rib.go` — PeerMeta tracking, received subscription, peerMeta cleanup on peer-down
- `internal/component/bgp/plugins/bgp-rib/rib_commands.go` — show best, best status, extractCandidate, gatherCandidates, kebab-case JSON keys
- `internal/component/bgp/plugins/bgp-rib/rib_test.go` — extractCandidate wiring test, updated JSON key assertions
- `internal/component/bgp/plugins/bgp-rib/protocol_test.go` — updated expectations (14 commands, 4 subscriptions)
