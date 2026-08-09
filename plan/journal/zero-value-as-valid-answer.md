| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-07-27 | - | guard | empty `addrs` slice passed per-address loop, bound `0.0.0.0:3443` unauthenticated | carried presence separately from value |
| 2026-07-27 | - | guard | `maxMsgID == 0` read as no cut, replay ran unbounded | used explicit tracked bool |
| 2026-07-27 | - | guard | `PeerAS == 0` for dynamic peers made `internal` always false | used `*uint64` pointer for presence |
