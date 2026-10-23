| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-22 | spec-bgp-explicit-final-policy (verification) | BGP hold/send-hold timer callbacks | `TestRFC9687Event29ReleasesTheLivePeersRIB` hung for 6m21s: the timer held `Session.mu`, then the FSM event callback reached `Session.Conn` and tried its read lock. Both timer callbacks had this lock pair. | open: remove the redundant outer lock pairs; `FSM.Event` already serializes transitions. Source corrected; focused execution and independent review pending. |
