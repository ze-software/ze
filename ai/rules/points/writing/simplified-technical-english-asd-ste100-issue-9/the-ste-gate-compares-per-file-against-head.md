---
kind: directive
level: MUST
stage:
---
- **HEAD is the baseline, and the comparison is per file.** A document nobody touched can never fail the gate, so legacy prose stays until someone rewrites it. The sentence you just wrote is what goes red.
- **There is no baseline file, and nothing to re-bless.** Rewriting a number cannot silence this gate, so the one way to green is to fix the prose (`ai/rules/completion.md`).
- **The gate is at commit time, not in `ze-doc-test`.** Several sessions share this checkout. A tree-wide prose gate reports a sibling session's in-flight sentences, and a gate that reddens for a colleague's typing gets switched off.
- **`make ze-ste-check` still reads the whole working tree**, so it can name a file another session is editing. Read the path before you read the habit.
- **The checker holds our own word lists, not the ASD dictionary.** It cannot see every violation, so the six habits stay a review checklist as well as a gate. Report a violation as an ISSUE against its habit number.
- **When the tool is wrong, fix the tool and add the case to `scripts/dev/ste_check_test.py`.** A checker that flags `setup`, an RFC 2119 MUST, or a code span gets switched off, and then it protects nothing.
- **Escape hatch for a document that must quote non-STE text at length:** `<!-- ste: ignore-file <reason> -->`, or `<!-- ste: ignore -->` above one line. The reason is mandatory.
- **Surfaces the tool reads:** Markdown in `docs/`, `ai/`, `plan/`, and the repository root. Prose comments in `.go`. The `description` strings in `.yang`. Piped text on stdin. It never reads `rfc/`, which stays verbatim.
