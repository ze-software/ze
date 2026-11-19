# Handover 49: RFC proof debt, re-taking the stale records

Spec: `plan/release/fixes/spec-rfc-proof-debt-outside-filter-wire.md` (ready,
unclaimed: the WIP cap refused the claim, and the owner rule forbids raising
it; the work was done unclaimed under the owner's order to continue).

## The rule this work runs under

`ai/META-DECISIONS.md`, "Test changes": a proof record goes stale only when its
tagged test or its claim changes. A change to the code it tests, the broken
function included, leaves it standing. Implemented in 6db7cb8964 (verifier and
commit gate). Re-takes use the record's recorded route.

## State at handover (2026-11-19, `./le rfc check` at 6db7cb8964)

443 violations: 197 stale records (all unit-changed), 62 SHIFTED audit
verdicts, 173 STALE audit verdicts, the rest other rows.

| Item | State | Next action |
|------|-------|-------------|
| AC-29 SHIFTED verdicts | 211 resealed in 40 stems (commit "rfc: reseal 211 SHIFTED audit verdicts in 40 stems"). 62 remain, mostly rfc4301 and rfc7296, skipped because another session holds uncommitted IKE evidence there | `./le rfc reseal stem <stem>... source head` for the stems `./le rfc check` names, once rfc4301/rfc7296 have no uncommitted edits from another session |
| AC-28 revert records (141, rfc4301 excluded) | A background batch was started: `tmp/session/2026-11-19-115101/scratch/retake-revert.sh` over `revert-retake.tsv`, writing one line per record (exit, id, polarity, unit, log) to `retake-results.tsv`. It runs `./le rfc record ... route revert producer <recorded producer>` one at a time | Read `retake-results.tsv`. Commit the rewritten `rfc/discrimination/<stem>/*.json` per stem through `./le commit create` (`no-test` reason: written by `./le rfc record`, which ran the unit). For each non-zero exit, read its log: a unit that runs in the Linux guest or an interop scenario needs that environment; a green run under the break is a finding on that test (fixed in this spec, `ze-test-judge` where a test changes). Re-run only what failed for a reason outside the record. If the batch did not finish, re-run the script over the lines missing from the results |
| AC-28 mutant records (55, rfc4301 excluded) | Prepared under `tmp/session/2026-11-19-115101/scratch/mutant/`: `commands.tsv` (40, the recorded original still sits once in the producer), `commands-context-resolved.tsv` (13, the original now occurs more than once; the line was chosen from the recording commit's code, each report's note gives the evidence), `proposed-new-break.tsv` (2, the original text is gone: RFC7084-4.2-10 positive and RFC8277-3.1-1 positive, a new break over today's spelling under `proposed/`) | Run the 53 ready commands one at a time after the revert batch ends (one CPU job at a time). The gates doc requires an independent `ze-test-judge` to read the `break` text of every hand-written entry before commit ("An independent judge reads the `break` text and decides whether it is the violation the claim names"): for the 53 it confirms the break equals the old record's apart from the line; the 2 proposals need judging before they are recorded. The commit body says the breaks were written by hand. The `producer` argument in the commands is ignored on the mutant route (`mutantBreak` takes the enclosing function) and harmless |
| AC-30 STALE verdicts (173) | Not started | One independent `/ze-rfc-audit` agent per stem, stamped with `./le rfc audit-stamp stem <stem> from <pending> mode rejudge`, after that stem's records |
| AC-32 unowned rows | Not started | RFC 4271 5-3/5-4 negative tests (owe judge verdicts), RFC 8955 4-3 and RFC 9252 3.3-2 records |
| AC-4 to AC-6 | Wait for `spec-rfc2545-next-hop-truncated-to-sixteen-bytes` | - |

## Scratch to clean when done

`tmp/session/2026-11-19-115101/scratch/`: `retake-revert.sh`,
`revert-stale.tsv`, `revert-retake.tsv`, `retake-results.tsv`,
`mutant-stale.tsv`, `mutant/`. Purpose: AC-28 re-takes. Remove once the
re-takes are committed.

## Other open items from the same session

- Nothing pushed. `main` carries this session's commits since the CI shard fix
  (c8d92da487) awaiting the owner's push order.
- `producer-sha` is still written and validated in every record as provenance;
  no verdict reads it. Dropping it means rewriting about 2,700 record files: an
  owner decision, not taken.
