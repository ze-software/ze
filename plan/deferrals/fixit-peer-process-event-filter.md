# Deferrals -- spec-fixit-peer-process-event-filter

One row. The spec made a peer's `attach process` block authoritative for which
events each program receives and which messages it may originate. The per-peer
`content { encoding format }` binding sits in the same block and stays inert,
for a reason that is structural rather than an omission.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-15 | spec-fixit-peer-process-event-filter | Honour the per-peer `content { encoding format }` binding, which resolves through the same method the receive and send lists do and is read by nothing | Format and encoding are per-process last-writer-wins in the subscription producer (`registerSubscriptions`, `internal/component/plugin/server/dispatch.go`), so two peers asking one process for different encodings cannot both be served. Honouring the binding needs that producer to carry a value per peer, which is a different change with its own design question | `plan/spec-fixit-stored-route-relay-hardening.md`, whose Current Behavior records the inert binding at `GetPeerProcessBindings` and the same per-process format producer | resolved |
