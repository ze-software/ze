# Handover 49: RFC proof debt, re-taking the stale records

Spec: `spec-rfc-proof-debt-outside-filter-wire` (closed; originally ready,
unclaimed: the WIP cap refused the claim, and the owner rule forbids raising
it; the work was done unclaimed under the owner's order to continue).

## The rule this work runs under

`ai/META-DECISIONS.md`, "Test changes": a proof record goes stale only when its
tagged test or its claim changes. A change to the code it tests, the broken
function included, leaves it standing. Implemented in 6db7cb8964 (verifier and
commit gate). Re-takes use the record's recorded route.

## Update 2026-11-20

Landed since: 8d6bd63526 re-takes 18 refused records with judge-approved
hand-written breaks (RFC8955-4.2-3, RFC5176-2.3-6, RFC5176-3.1-1,
RFC8277-2.4-1, RFC5575-4-8, RFC8955-4-4, RFC7606-5.1-1 two entries,
RFC7606-5.1-3) and moves TestRFC8955NextHopIgnoredForFlowSpec's receive
through flowValidationHandle. 903f715033 fixes the RIB receive-path lock
self-deadlock found on the way; 16b61406ba fixes the rr replay flake (a
fixture race) and the FlowSpec receive hang in the test helpers.

Still owed, in order:
1. The 9 RFC4861 records (unit TestRFC4861SubscriberSolicitationReaderIgnoresReservedAndOptions,
   producer now `internal/component/l2tp/ppp/ra_carrier.go::rsReaderLoop`) and
   RFC4659-3.2.1.1-1: Linux-only, a guest run with `kernel <vmlinuz>`.
   Commands in `tmp/session/2026-11-19-115101/scratch/refused/commands.tsv`.
2. RFC7606-5.1-1 negative, `TestForwardLeadingMPAttributeKeepsZeroCopy`: its
   recorded break still names line 79, now `}`; the report at
   `scratch/mutant/RFC7606-5.1-1-negative-1.json` needs the line the
   MP_REACH arm's `if otherSeen` now sits on (81), then a judge read.
3. AC-29: the SHIFTED verdicts left, mostly rfc4301/rfc7296 (IKE session's).
4. AC-30: the STALE verdicts, one independent `/ze-rfc-audit` per stem.
5. AC-32: RFC 4271 5-3/5-4 negatives; RFC 8955 4-3 and RFC 9252 3.3-2.

## State at handover (2026-11-19, `./le rfc check` at 6db7cb8964)

443 violations: 197 stale records (all unit-changed), 62 SHIFTED audit
verdicts, 173 STALE audit verdicts, the rest other rows.

| Item | State | Next action |
|------|-------|-------------|
| AC-29 SHIFTED verdicts | 211 resealed in 40 stems (commit "rfc: reseal 211 SHIFTED audit verdicts in 40 stems"). 62 remain, mostly rfc4301 and rfc7296, skipped because another session holds uncommitted IKE evidence there | `./le rfc reseal stem <stem>... source head` for the stems `./le rfc check` names, once rfc4301/rfc7296 have no uncommitted edits from another session |
| AC-28 revert records: outcome | Batch finished. 109 of 141 re-taken and committed (two commits, "rfc: re-take 31 stale revert proof records" and "rfc: re-take the rest of the stale revert proof records"), plus the two functional records RFC7606-5.1-2 and RFC8092-3-1 re-run with their citations. 30 refused, listed in `tmp/session/2026-11-19-115101/scratch/revert-refused.tsv` with logs in `retake-results.tsv`: (a) 9 whose producer file `internal/component/l2tp/ppp/ra_linux.go` is gone: name the function that now sends router advertisements and re-take against it; (b) 4 RFC8277-2.4-1 whose changed tests no longer execute the recorded producer (`walkVPNNLRIs`, `wrapNLRI`): choose the producer each test now exercises; (c) RFC5176-3.1-1 `TestRFC5176ProxyStateContentIsNeverRead` stays GREEN with `handleDisconnect` halted: a finding on that test (weakened, or it now checks another path), to fix here with `ze-test-judge` if the test changes; (d) 6 FlowSpec units (RFC5575-4-8, RFC8955-4-4, RFC8955-4.2-3) did not finish inside 15 minutes under a whole-function halt: re-take on a one-line mutant; (e) Linux-only and capability-gated units (RFC4659-3.2.1.1-1, the RFC4861 reader set): need a guest run with `kernel <vmlinuz>` | Work (a) to (e) |
| AC-28 revert records (141, rfc4301 excluded) | A background batch was started: `tmp/session/2026-11-19-115101/scratch/retake-revert.sh` over `revert-retake.tsv`, writing one line per record (exit, id, polarity, unit, log) to `retake-results.tsv`. It runs `./le rfc record ... route revert producer <recorded producer>` one at a time | Read `retake-results.tsv`. Commit the rewritten `rfc/discrimination/<stem>/*.json` per stem through `./le commit create` (`no-test` reason: written by `./le rfc record`, which ran the unit). For each non-zero exit, read its log: a unit that runs in the Linux guest or an interop scenario needs that environment; a green run under the break is a finding on that test (fixed in this spec, `ze-test-judge` where a test changes). Re-run only what failed for a reason outside the record. If the batch did not finish, re-run the script over the lines missing from the results |
| AC-28 mutant records: outcome | 50 re-taken and committed ("rfc: re-take 50 stale mutant proof records from hand-written breaks"), judged first. Three more recorded but NOT committed, awaiting a judge read of their hand-written break: RFC7606-5.1-1 (two TestForwardMovesTrailingMPAttributeFirst* entries) and RFC7606-5.1-3 (`.ci`); their files are modified in the tree. Not re-taken: RFC5176-2.3-6 `TestRFC5176CoANAKMakesNoPartialChange` stays GREEN under its recorded break (a finding on the test), and RFC7606-5.1-1 `TestForwardLeadingMPAttributeKeepsZeroCopy` went red without naming the unit. At the time of writing another session's malformed `rfc/short/rfc2181.md` (a {not-applicable} annotation at line 78) made a hook refuse every command naming an `rfc/` path; wait for it, never edit it | Judge the three RFC 7606 entries, commit them, then look at the two findings |
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
