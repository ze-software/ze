---
kind: note
level:
stage:
---
Enforced: `check_known_failure_load_excuses` in `scripts/dev/verify_wiring_docs.py`
(`make ze-doc-wiring-check`, inside `make ze-precommit-verify`) fails a CHANGED
`plan/known-failures/` shard containing "under load", "loaded host", "load
average", "load-sensitive", "passes in isolation", "resource contention" or
"contended host". `README.md` and `RESOLVED.md` are exempt: the first states this
policy, the second is a verbatim archive of history and is not edited to satisfy
a present-day gate. The gate checks the excuse, not the existence of a shard:
a red whose mechanism is genuinely unknown still belongs there.
