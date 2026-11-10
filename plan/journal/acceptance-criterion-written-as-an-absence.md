# acceptance-criterion-written-as-an-absence

An acceptance criterion says that something is gone, for example "a grep for
the retired name returns nothing". A removal leaves correct survivors that must
spell the removed string: a fixture proving the name is gone, a revision log, a
release note, another product's command, a record of the change. No tree can
meet the criterion, and the wrong hits hide among the correct ones.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-21 | spec-cli-show-bgp-is-the-command | AC-9, a grep for the retired `show bgp summary` | The criterion allowed hits only in git history and the spec. The tree held five kinds of correct survivor, and six open specs naming the retired command were noticed only in review | fixed: 9059161574 repaired the six specs; fcb9fbe117 refuses an absence-only criterion; `TestCriteriaNamePositiveBehavior` red "want a refusal containing AC-1 states only an absence" (tooling-one-off-evidence.md, "Unit red walks of landed fixes") |
