# Verification debt -- commit session 20dfdf8d

Gates that had not run green over these commits when they were made.
One row holds one gate and one reason, and covers every commit this
session made under it. `git log -- <this file>` names those commits.
Clear rows only through `le commit debt-clear` after the named gate exits 0.

| Date | Session | Subject | Gate owed | Reason | Status |
|------|---------|---------|-----------|--------|--------|
| 2026-09-09 | 20dfdf8d | blog: restore personal narratives and original arguments (+7 more) | full native verification (not FRESH-green) | verify-status is not FRESH-green: STALE: last verify failed (exit=1, at 2026-09-05T00:11:54Z) | open |
| 2026-09-10 | 20dfdf8d | fix(ospf): publish deferred auto-cost changes and join origination (+3 more) | full native verification over this commit's Go | no full native verification covers this commit's Go | open |
| 2026-09-10 | 20dfdf8d | fix(telemetry): bind metrics before standalone plugin startup | discovery-index freshness | Only startup ordering inside runYANGConfig changes. No package, import or function declaration changes. | open |
| 2026-09-10 | 20dfdf8d | fix(ddos): sweep stale responses after firewall configuration (+1 more) | full native verification (not FRESH-green) | Owner-authorised pre-release closure reuses current scoped race, Linux7.2 and timing proof; no full green certificate is claimed; unchanged gates must not be rerun. | open |
| 2026-09-10 | 20dfdf8d | fix(ddos): sweep stale responses after firewall configuration (+1 more) | native structural checks (red) | Parent gates remain RED outside DDoS: lint93/104/2/1, scoped22/22 with no DDoS source diagnostics, repository24BGP, audit16concurrent, doc3759/8. Owner forbids unchanged reruns. | open |
| 2026-09-10 | 20dfdf8d | fix(ddos): sweep stale responses after firewall configuration (+1 more) | full native verification over this commit's Go | Owner authorised closure with scoped DDoS proof and inherited discrimination REDs, skipping project-wide validation. | open |
| 2026-09-10 | 20dfdf8d | fix(ddos): sweep stale responses after firewall configuration (+1 more) | discovery-index freshness | Native index-feed gate charges these source edits; runtime registrations and exported APIs are unchanged, and the owner forbids index generation or validation reruns during this closure. Index state remains explicitly unverified. | open |
| 2026-09-10 | 20dfdf8d | docs(plan): close the DDoS timing leaves spec | full native verification (not FRESH-green) | Owner-authorised pre-release closure preserves the supplied DDoS proof and hash-pinned CLEAN review; no full green certificate or unchanged-gate rerun is claimed. | open |
| 2026-09-10 | 20dfdf8d | docs(plan): close the DDoS timing leaves spec | native structural checks (red) | Settled unrelated repository, commit-audit, lint and documentation REDs remain attributed in the preserved spec; the owner forbids rerunning unchanged gates. | open |
| 2026-09-10 | 20dfdf8d | fix(ospf): preserve adjacency through explicit cost reloads | discovery-index freshness | Existing package responsibilities and registrations are unchanged. The change modifies reconciliation, tests and an existing interop scenario, not the package inventory. | open |
