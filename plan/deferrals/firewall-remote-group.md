# Deferrals -- spec-firewall-remote-group

Source: `plan/spec-firewall-remote-group.md`. Format: `ai/rules/planning.md`.

Created 2026-08-02 by a session sweeping the shared working tree, NOT by the spec's
author. The spec names two mechanisms it deliberately excludes, and the commit gate reads
that exclusion as a deferral with no shard. The rows below transcribe what the spec itself
says, including the destination it already names. They add no judgement of their own: the
author should correct them if the transcription is wrong.

Reference lists in this file are written as bullets, never as tables. Every pipe-delimited
line here is read by the deferral gate as a six-cell row.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-02 | spec-firewall-remote-group | Packet-triggered set population through nftables `dynset`, and finishing the inert `flags-dynamic` lowering | The spec states this is "a different mechanism for a different purpose" and names the spec that owns it, including the inert lowering | `plan/spec-firewall-dynamic-address-group.md` | deferred |
| 2026-08-02 | spec-firewall-remote-group | FQDN and domain groups | The spec calls this "a third mechanism" and states it is not in scope. Homed 2026-09-02 (owner directive): the destination now exists and owns DNS as a source, the TTL schedule, name-beside-address display, and the change record for audit | `plan/spec-firewall-domain-group.md` | deferred |
