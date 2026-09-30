# Judge brief (Procedure P, judge half)

Repo: /home/thomas/Code/github.com/ze-software/ze/main (shared checkout). You are the INDEPENDENT JUDGE. You wrote none of the tests or code you judge.
READ: ai/skills/ze-rfc-audit.md fully (incl. `mode rejudge`), the parent spec's Method table, the child spec named in your prompt, and the author's handoff file named in your prompt.
FOR EVERY id in the handoff, and every verdict `./le rfc check` reports stale in the touched stems: read row quote, RFC text, EVERY tagged unit (grep "RFC requirement: <ID>" across internal/ cmd/ test/), the producer, and the discrimination records (`./le rfc discriminate id <ID>`). Judge strictly: every clause both polarities or a valid single-polarity, buffers isolated, the negative violates THIS requirement, each cover has an observed-red record. Also sanity-check any code fix at its producer; report a defect rather than fixing it.
STAMP: pending file under `$(./le session scratch ensure)`; `./le rfc audit-stamp stem <stem> from <pending> mode rejudge` for ids with a verdict, default mode for new rows. Honest verdicts only: weak with a note naming what is missing beats an inflated enforced. `upgrade_reason` only where the tool/skill requires it. Mismatched units that only SHIFTED: `./le rfc reseal`. Then `./le rfc index-update`.
CHECK once: `./le rfc check` -- no violation in the touched stems except the 18 known producer-changed records (rfc4271/7611/7705/7947/8907). If Go changed: `./le go lint run` once (0 issues required; fix trivial lint yourself only in files the author changed, then re-record any stale discrimination record).
COMMIT one commit: `./le commit create replace subject "rfc: <child> <package> verdict fixes" body "<counts, defects fixed>" file ...` naming exactly: the author's file list, the rfc/audit/<stem>.json files you stamped or resealed, rfc/discrimination and rfc/corrections/rfc/short/rfc/extraction files the author changed. Never test/weakened/*, never files another agent is working on (check `git status`; if a file you need carries foreign hunks, apply the AGENTS.md carry rule and say so in the body). Run the printed script= with bash. Report refusals verbatim.
REPORT <= 12 lines: per stem counts old->new (enforced/weak/wrong), any verdict still weak and why, defects, commit SHA.

## ADDENDUM 2026-09-29 (binding)
- LEDGER LOCK: many judges run at once and `./le rfc reseal` is corpus-wide. Wrap EVERY ledger-writing command in the lock:
  `flock /home/thomas/Code/github.com/ze-software/ze/main/tmp/session/2026-09-28-869df689-cc8f-4d78-9161-1d7c87434c8e/scratch/children/ledger.lock ./le rfc audit-stamp ...` (same for `./le rfc reseal` and `./le rfc index-update`). Never hold the lock across your reading/judging.
- COMMIT ONLY YOUR STEMS: after a reseal, other stems' audit files may change in the tree; do NOT commit audit/discrimination/short/extraction/corrections files of stems outside your handoff. `./le commit create` names files explicitly; name only yours.
- PARTIAL PACKAGES: judge only the ids the handoff marks resolved (tests/row/defect-fixed). Leave unresolved ids untouched (their verdict stays as is). The commit is still one commit for what was resolved.
- INTENTIONALLY RED untagged defect tests named in the handoff: EXCLUDE them from the commit (they stay in the tree for the defect-fix step); list them in the commit body.
- SUBJECT <= 72 chars.
- If the author used a shell/python edit instead of Edit/Write (handoff says so), open those files and re-check the diff yourself.
- LINT: if you cannot run `./le go lint run` within your limits, say so; the main thread runs it after your commit.

ADDENDUM (2026-09-30, lock scope). `./le rfc discriminate-record` writes only rfc/discrimination/<stem>.json, so take a PER-STEM lock for it: `flock scratch/children/ledger-<stem>.lock ./le rfc discriminate-record ...` (e.g. ledger-rfc4271.lock). Keep the GLOBAL `scratch/children/ledger.lock` for corpus-wide steps only: `./le rfc reseal`, `./le rfc audit-stamp`, `./le rfc index-update`. A long record run under the global lock stalls every other agent.
