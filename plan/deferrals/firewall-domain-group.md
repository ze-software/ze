# Deferrals -- spec-firewall-domain-group

Source: `plan/spec-firewall-domain-group.md`. Format: `ai/rules/planning.md`.

Created 2026-09-02 by the spec's own author, at the WRITE gate.

Reference lists in this file are written as bullets, never as tables. Every pipe-delimited
line here is read by the deferral gate as a six-cell row.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-09-02 | spec-firewall-domain-group | The shared sourced-group substrate: disk cache, keeping the last good answer when a source fails, set programming, and the show/update/clear command shape | Owner decision at the WRITE gate: this spec adds DNS as a source and TTL as a schedule, and does not build the substrate a second time. The destination spec owns it and is still `skeleton`, so whichever spec implements first settles the shared shape | `plan/spec-firewall-remote-group.md` | deferred |
| 2026-09-02 | spec-firewall-domain-group | Signature validation for DNSSEC, rather than trusting an upstream validating resolver's SERVFAIL | `dnssecDecision` (`resolver.go`) sets the EDNS0 DO bit and reads the rcode and AD bit; it performs no crypto and always accepts AD=0 from an unsigned zone. Making Ze validate signatures itself is resolver-sized work, recorded as R-5 and in Known Limitations | `plan/spec-fixit-dns-rfc1035-conformance.md` | deferred |
| 2026-09-02 | spec-firewall-domain-group | A registration path for dynamic CLI value completion, so a plugin's configured names complete without hand-wiring | Completion for runtime values is hand-wired in `internal/component/cli/client/main.go`, which sits against `ai/rules/plugins.md` on plugin spelling in central packages. `firewall-irr` carries the same gap and wires no `CompleteFn` either, so this spec follows precedent rather than inventing a registry for one feature | `plan/spec-firewall-remote-group.md` | deferred |
