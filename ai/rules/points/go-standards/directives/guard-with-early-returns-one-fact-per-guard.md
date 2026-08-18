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
- **Every non-exempt `.go` file MUST carry a `// Design:` header, and that is now checked at COMMIT time as well as at write time.** `go_design_ref_problems` (`scripts/dev/commit_helper.py`) refuses a commit whose `.go` files lack it. It exists because `c_require_design_ref` reaches the Write and Edit tools only, so a file written from Bash met no gate; a commit's changed-file set is a fact, and it does not matter which tool produced the file. Exempt: `_test.go`, `_gen.go`, `register.go`, `embed.go`, `doc.go`, `vendor/`, and a generated file saying `Code generated` or `DO NOT EDIT` in its first 500 bytes.
- **This obliges; `docs/contributing/ze-style.md` explains.** That page's "Control flow a reader can simulate" carries the reasoning. When the two disagree, this file wins.
- **The one-fact rule was guidance-only until 2026-08-18 and that cost real code.** It lived on a page reachable through three triggers a session could miss entirely, so `timedOut` (`internal/component/ike/engine/dpd.go`) shipped opening on a two-fact condition and `ignoredNames` (`scripts/vendor/check_web.go`) decided its error case on three. Both were written by a session that never opened the guide, because `c_pre_write_go` (`.claude/hooks/pretool-writeedit.py`) fires only for the Write and Edit tools and every one of those edits went through a Bash heredoc.
