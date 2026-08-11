# discarded-error-becomes-destructive

A caller discards an error slot because, under the behaviour of the day, the
worst case was a smaller or emptier output. The operation later gains the power
to DELETE, and the same discard turns a partial read into data loss that reports
success. The discard is not where the defect was introduced. The new power is.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-11 | rfc-ledger-per-rfc-shards | tooling | `run_write` discarded the `parse_errs` slot of `_collect_for_check`. That was safe while the write only rendered one file. Once the write also PRUNED, a summary that failed to parse rendered no rows. Its stem left the rendered set. The prune then deleted that RFC's tracked evidence file and the run exited 0. An absent `rfc/short/` deleted all 177 the same way | `run_write` refuses before any write or delete. It refuses on a parse error, and on a render with no rows. Two tests drive the entry point, and each fails when its guard is removed |
| 2026-08-11 | - | tooling | `run_check_fresh` discarded the same slot, one function from the row above. It deletes nothing, so the loss is a DIAGNOSIS. The unparsed stem renders no rows. The gate then called the index stale, and called that RFC's file an orphan. Both readings are false. The second one invites a reader to hand-delete the evidence file. One tree gave two answers: `ze-rfc-check` named the parse error, `ze-doc-test` named an orphan | Report the errors and exit 2 first. The comparison never runs. The lesson the row above missed: a discarded error slot is a defect at EVERY caller. Fix all of them in one pass. The blast radius differs, so the harm looks absent at the caller that only reports |
