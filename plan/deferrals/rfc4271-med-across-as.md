# Deferrals: rfc4271-med-across-as

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/planning.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-14 | spec-rfc4271-med-across-as (Known Limitations) | The MED comparison rules of the decision process, beyond `RFC4271-9.1.2.2-2` | This spec governs what leaves ze on the wire. How MED is weighed when choosing a best path among routes from one AS is a different question, in a different function, and changing it moves which route ze installs rather than which attribute it advertises | the spec's own Known Limitations put this outside what it governs, and no obligation is left unproven by it: RFC4271-9.1.2.2-1 and RFC4271-9.1.2.2-3 both carry real dispositions | cancelled |
| 2026-08-14 | spec-rfc4271-med-across-as (phases 4 to 7) | `RFC4271-5.1.4-4`, the configuration-driven removal mechanism, plus `5.1.4-2` and `9.1.2.2-2` which are exempted only while it is absent. Also `med-removal-configured.ci` and the interop scenario | The propagation rule (5.1.4-1) is implemented and proven and stands alone. The removal mechanism is a separate obligation with its own config surface, and the two exemptions become answerable only once it exists. The design and the file and symbol anchors are in the per-spec handoff | done in spec-rfc4271-med-across-as: 5.1.4-4 and 5.1.4-2 carry tagged proof, `test/plugin/med-removal-configured.ci` and `test/interop/scenarios/61-med-remove-configured-gobgp/` exist | done |
