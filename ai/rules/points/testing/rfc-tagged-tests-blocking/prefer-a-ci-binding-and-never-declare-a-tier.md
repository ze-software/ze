---
kind: directive
level:
stage:
---
- **Prefer a `.ci` over an interop binding** when a behavior is reachable from both: a `.ci` runs inside `ze-verify` on every push, interop does not (owner decision, umbrella D3).
- A requirement whose ONLY evidence is nightly-tier is marked `**nightly-only**` on its ledger row and counted in its own rollup column: it is not merge-gate-proven, and the rollup deliberately never sums the two.
- **An interop tier is DERIVED, never declared.** A tree earns `interop/nightly` when a SCHEDULED workflow under `.github/workflows/` names its runner, which `scheduled_workflow_targets()` reads. So adding the job IS the whole fix and `CARRIERS` needs no edit, and deleting the job takes the tier away again rather than leaving a stale claim behind (`ai/rules/evidence.md`).
- **A tag in `test/l2tp-interop/`, `test/pppoe-interop/`, or any other `check.py` tree is REFUSED** with an error naming the file, because no scheduled workflow runs those suites and a tag nothing executes is an absence of evidence rather than weak evidence. The l2tp and pppoe labs need host kernel modules (`l2tp_ppp`, `pppoe`, `/dev/ppp`) that no runner is yet confirmed to provide, so the sequence stays wire, observe one green run, then the tier follows on its own.
- **A QEMU sibling is not that pipeline.** `ze-qemu-l2tp-ppp-test` and `ze-qemu-pppoe-accel-test` run `scripts/evidence/effective-*.py`, never the trees' `check.py`, so they execute no tagged carrier and cannot justify a tier for one.
- **Non-unit evidence is monotonic, per requirement and per tier.** Replacing a `.ci` binding with a unit tag, or with a nightly interop tag, fails `make ze-rfc-check`, and no annotation satisfies it.
- A `check.py` is TOKENIZED, not line-scanned: a `#` inside a docstring or string literal is not a comment and is not a tag, and an untokenizable `check.py` fails the scan closed.
