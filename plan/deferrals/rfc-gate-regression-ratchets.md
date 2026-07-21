# Deferrals: rfc-gate-regression-ratchets

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-20 | spec-rfc-gate-regression-ratchets (G4) | **Arm the SHA freshness ratchet for the other 164 enrolled RFCs** by recording `rfc/audit/<rfc>.json` verdicts via `/ze-rfc-audit`. `check_audit_freshness` (`scripts/dev/rfc_requirements.py:934`) skips any requirement with no recorded verdict, so the only RFC whose tagged tests are fingerprinted is rfc7606 (`rfc/audit/rfc7606.json`, the sole audit file against 165 enrolled RFCs). Until then, a tagged test WEAKENED IN PLACE is caught only by `c_test_weakening` (`.claude/hooks/pretool-writeedit.py`) and `scripts/dev/audit-test-relaxation.py`, not by the gate | Ruled by Thomas 2026-07-20: skip it. Two reasons, in order. The weakening case it would cover is already handled at better altitude by the edit-time guard and the relaxation auditor, both of which apply to every test rather than to audited RFCs only; and mass-generating 164 audit files would record a verdict for an audit nobody performed, which is precisely the declare-instead-of-prove failure this whole programme exists to remove. The three ratchets shipped in this spec cover deletion, downgrade and retirement, which are the regressions the gate can honestly see | `plan/spec-followup-rfc-enrollment.md` (already owns `rfc/enrolled.txt` and the rollup; the audit backlog belongs with it). Close this row only when verdicts exist AND were genuinely read, never by generating the files | deferred |

