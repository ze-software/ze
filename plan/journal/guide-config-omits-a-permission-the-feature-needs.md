# A guide's config omits a permission the feature needs

The example parses and the daemon runs, but a feature the page describes
cannot act, because the config never grants the permission it is checked
against at runtime. The refusal is a WARN line, not a config error.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-21 | spec-exabgp-6-api-compatibility | `docs/guide/route-reflection.md` examples, peer-up replay (`rr/rr.go` `replayForPeer`, `adj_rib_in/rib.go` relay-stored-route) | Example peers attach adj-rib-in receive-only, so the peer-up replay (sent as bgp-adj-rib-in) is refused and a route received before that peer came up never reaches it | open: grant adj-rib-in `send [ update ]` in the examples, or authorize the replay as rr/rs; read `rib_claims.go` and the send check |
