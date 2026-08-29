---
kind: directive
level: MUST
stage:
rationale: plan/journal/gate-excludes-part-of-its-population.md
---
- **Guard with EARLY RETURNS: handle the edge case and return, and never wrap the happy path in an `else`.** A sequence of guard clauses followed by the main logic reads top to bottom; a happy path inside an `else` does not. Applies to `if err != nil { return }`, validation guards and nil checks alike.
- **One fact per guard. A compound condition MUST be split.** `if a || b` and `if a && b && !c` make a reader hold two or three facts at once to decide whether every case is covered, and the answer is usually that one of them was never considered. Write one `if` per question and ask, for each, whether the negative case needs a branch of its own.
- **State the invariant POSITIVELY.** `if index < length` reads directly. `if index >= length` states the failure of the invariant and makes the reader invert it before they can check it.
- **Name a compound test rather than inlining it.** Two exit codes tested inline are a condition; `isCheckIgnoreAnswer(code)` is a sentence. The name is where the reason lives, and a reviewer checks a name against its call site far faster than they re-derive a boolean.
- **Every non-exempt `.go` file MUST carry a `// Design:` header.** The native `./le consistency` action reports missing headers from `checkDesignRefs` in `internal/le/consistency/consistency.go`. Exempt: tests, generated files, registration leaves, and vendor code.
- **This obliges; `docs/contributing/ze-go-style.md` explains.** That page's "Control flow a reader can simulate" carries the reasoning. When the two disagree, this file wins.
- **The one-fact rule was guidance-only until 2026-08-18 and that cost real code.** `timedOut` in `internal/component/ike/engine/dpd.go` shipped with a two-fact condition and `ignoredNames` in `internal/le/vendorweb/check.go` decided its error case on three. The native edit checks do not replace reading this rule.
