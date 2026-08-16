---
kind: note
level:
stage:
---
Known gap, recorded rather than papered over. Several checks run under BOTH
`ze-doc-verify` and `ze-generated-files-check`. That overlap is harmless: the runner
continues across stage failures, so one underlying red fails both stages in the
same run, `structural_gate_reds` always sees `ze-generated-files-check`, and the
commit is blocked regardless of what `plan/known-failures/` says about
`ze-doc-verify`. The real gap is the checks that run ONLY under `ze-doc-verify` --
`doc_drift.go`, `commands.go`, `digest_check.py`, and `rfc_requirements.py
--check-fresh` (`mk/inventory.mk`; note the script's `--selftest`/`--check`
invocations DO run as the `ze-rfc-check` stage, so only the `--check-fresh`
ledger-staleness one is doc-test-exclusive). Those are just as deterministic and
structural, and they ARE
parkable, because `ze-doc-verify` is not in the set. Whoever picks this up should
decide whether `ze-doc-verify` belongs in `STRUCTURAL_GATES`; that is where reds
actually escape.
