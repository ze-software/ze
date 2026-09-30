# An exemption one mechanism grants is still billed by another

A gate offers a way to record that a member of a population does not carry the
evidence the population usually carries. The record is accepted. A second
mechanism of the same gate then counts that member against a budget the
exemption said it does not draw from.

Nothing is wrong on the day the exemption is written, because the budget has
room. The failure arrives later, at the boundary, when a member that DOES carry
the evidence is added and the budget refuses it. The refusal names the new
member, so the reader repairs the new member and never sees the exempted ones
consuming its room.

The shape that survives subtracts the exempted members before the comparison.
An exemption a gate grants and then charges for is two rules with one name.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | - | `DeriveRegister` (`internal/le/rfc/inventory.go`), via `./le rfc check` | The register is `keywordSites >= gated`, and `gated` counts ids an extraction sanctions as `unsourced-ids`, which the same check accepts. rfc8671 sits at the boundary (10 sites, 10 gated, 2 unsourced), so declaring its §5.2 MUST would refuse its rfc2119 sign-off; that site stays excluded feature-out-of-scope | fixed 2026-09-30: `sourcedGatedCounts` subtracts sanctioned unsourced ids before the comparison; a prose sign-off is judged against its own prose sites (`Deriver.InventoryUnder`) |
| 2026-09-30 | spec-rfc-verdict-fix-ike-eap | `DeriveRegister`, rfc4301 | Splitting RFC4301-4.4.2.1-1 into one unsourced row per SAD item raised gated to 91 against 89 keyword sites: the register flipped to prose and the check reported ~70 false derived-site and quote errors | fixed by the same change (tests in `internal/le/rfc/register_unsourced_test.go`); rfc2516 and rfc8707 were labelled prose but walked the keyword set, so their register is relabelled rfc2119 |
