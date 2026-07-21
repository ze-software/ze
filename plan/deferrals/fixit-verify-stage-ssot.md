# Deferrals: fixit-verify-stage-ssot

Deferral rows for this source. The aggregate live backlog is folded on
read from `plan/deferrals/` by `/ze-status`; nothing stores it (`ai/rules/deferral-tracking.md`).

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-07-20 | spec-fixit-verify-stage-ssot | skill_sync.sh never removes a mirror whose canonical `ai/skills/<name>.md` was deleted; an orphan makes `--check` fail with a remediation that cannot fix it | separate-subsystem latent bug in the session-start advisory path, not the verify stage list; moot for verify (skill_sync is not wired into ze-regen-check-readonly) | plan/learned/1227-fixit-verify-stage-ssot.md | cancelled |

