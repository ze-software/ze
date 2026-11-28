# Fleet interface traffic history

The post-release device Interfaces tab will show received and transmitted traffic and packet drops (`rx-octets`, `tx-octets`, `rx-dropped`, `tx-dropped`) for one named interface over time. Fleet-8 supplies the first-release whole-device traffic totals. Fleet UI-4 owns the additional per-interface collection, storage, query and panel. [Device tabs design](../../plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md)

## Design decisions

| Decision | It forbids | Source | Date |
|----------|------------|--------|------|
| For the post-release device Interfaces panel, Thomas chose “Add bounded per-interface history in the post-release design”. | Substituting the device aggregate for an individual interface, limiting collection to the lifetime of an open panel, or making this a first-release dependency | plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md | 2026-11-25 |

## Query and retention

A query names one authorized device, one interface and one time range. The hub retains interface observations independently of page visits. The history ring accounts for every series against a per-device and a hub-wide cap. A missing report, a counter reset or an unsupported device leaves a gap. The chart returns the covered time range and visible gaps even if capacity pressure shortened retention.
