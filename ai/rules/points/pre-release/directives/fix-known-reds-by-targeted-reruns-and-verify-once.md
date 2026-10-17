---
kind: directive
level: MUST
stage:
rationale: ai/rationale/known-reds-first.md
---
**When a run leaves reds, you MUST write them into one list of known reds, and you MUST NOT start another full verify while a listed red is still open.** A full verify here is any run wider than the failure: lint over the tree, `./le test unit all`, a whole functional suite, `./le verify`.
**Each fix MUST be proven by rerunning only the test that failed: the one Go test with `-run` over its package, or the one `.ci` selection, alone and under the load that broke it.** The package or the suite around it is the next step out, and only after the test itself passes. The recipes are `docs/contributing/running-commands.md` for a Go test and `docs/functional-tests.md`, "Run a single test", for a `.ci`.
**When every listed red passes its targeted rerun, the full verify MUST run once, on a quiet machine with no other agent running tests beside it.** The reds it leaves become the next list.
