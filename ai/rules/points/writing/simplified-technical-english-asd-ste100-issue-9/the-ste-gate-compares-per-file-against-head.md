---
kind: directive
level: MUST
stage:
---
- **HEAD is the baseline, and the comparison is per file.** A document nobody touched can never fail the gate, so legacy prose stays until someone rewrites it. The sentence you just wrote is what goes red.
- **There is no baseline file, and nothing to re-bless.** Rewriting a number cannot silence this gate, so the one way to green is to fix the prose (`ai/rules/completion.md`).
- **The gate is at commit time, not in `./le doc-check verify`.** Several sessions share this checkout. A tree-wide prose gate reports a sibling session's in-flight sentences, and a gate that reddens for a colleague's typing gets switched off.
- **`./le ste check` still reads the whole working tree**, so it can name a file another session is editing. Read the path before you read the habit.
- **The checker holds our own word lists, not the ASD dictionary.** It cannot see every violation, so the six habits stay a review checklist as well as a gate. Report a violation as an ISSUE against its habit number.
- **When the tool is wrong, fix the tool and add the case to `internal/le/ste/ste_test.go`.** A checker that flags `setup`, an RFC 2119 MUST, or a code span gets switched off, and then it protects nothing.
- **Escape hatch for a document that MUST quote non-STE text at length:** `<!-- ste: ignore-file <reason> -->`, or `<!-- ste: ignore -->` above one line. The reason is mandatory.
- **Surfaces the tool reads:** Markdown in `docs/`, `ai/`, the durable half of `plan/`, and the repository root. Prose comments in `.go`. The `description` strings in `.yang`. Piped text on stdin. It never reads `rfc/`, which stays verbatim.
- **A document that is DELETED when the work closes is out of scope, and editing its prose is banned work (owner directive, 2026-08-10).** A spec `git rm`s itself in commit B, and a deferral or known-failure shard goes when its rows resolve, so a sentence rewritten there is read once by the session that wrote it. `plan/spec-*.md`, `plan/deferrals/` and `plan/known-failures/` are excluded in `internal/le/ste/ste.go`. `plan/journal/`, `plan/learned/` and `plan/TEMPLATE.md` stay in: they outlive every spec and are read by sessions that were not there.
