---
kind: directive
level: MUST NOT
stage:
---
**A check that has run MUST NOT be run again to reconfirm what its output already said.** One run, read the whole output, act on it.
**Three re-runs are forbidden by name: a gate that passed, a log you have already read, and a tree unchanged since the last run.** A re-run is earned by one thing only, which is an EDIT to the code under test.
**Re-reading is the same waste as re-running, and it MUST NOT be spent either.** Opening the same failure summary a second time to look for something new is the tell that the session is avoiding the product.
