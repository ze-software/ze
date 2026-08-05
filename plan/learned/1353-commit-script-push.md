# 1353 - Pushing from the commit script, and the injection it exposed

**Date:** 2026-08-05
**Scope:** tooling, agent workflow, rules

## What Changed

Pushing was absolutely forbidden. Thomas, who wrote that ban, amended it on
2026-08-05: a push is allowed from a helper-generated commit script, on his
order only. `scripts/dev/commit_helper.py` gained `--push AUTHORISATION`.

**The amendment did not weaken the hook, and needed no hook change at all.**
`check_destructive_git` (`.claude/hooks/pretool-bash.py`) substring-matches the
command an agent TYPES; it never inspects a script's contents. `git commit` is
in that same block list and commit scripts have always run as
`bash tmp/commit-*.sh`. A push line inside a generated script therefore rides a
mechanism that already existed. Typing the bare command is still refused.

## Why The Ban Existed, And Why The Exception Is Safe

The ban's purpose was never "pushing is dangerous". Sessions share one git
index, so a loose add / commit / push sequence carries another session's staged
work to the remote. One script that bundles add, remove, commit and push leaves
no window where that can happen.

**A rule outlives the hazard that produced it.** This one had become absolute
prose about pushing when the real subject was staging races, so the sanctioned
fix looked like a violation. When a rule's stated reason no longer matches its
text, the text is what agents obey.

## The Vulnerability This Exposed (the real lesson)

Adding a push line to the generated script turned a latent shell injection into
a bypass of a safety guard. **The injection predates this work.** Caller text
reached the script unescaped for as long as `--lesson-not-needed` existed.

An adversarial reviewer reproduced it end to end through the real CLI: a
`--lesson-not-needed` value carrying a newline and a forged `# ze-commit-push:`
marker, with no `--push` anywhere, followed by an `--append` onto that script.

Two producers combined. `render_block` wrote caller text unescaped at the start
of a line, so a newline ended the comment. `split_push_section` took the FIRST
line starting with the marker as the recorded authorisation and truncated the
body there. Result: a real push line, `push=AUTHORISED (FORGED...)` printed as
though the owner had ordered it, and the first commit block silently deleted
from the script.

Two further routes surfaced while fixing it: a FILENAME carrying the marker, and
the `--script` path landing unquoted inside an `echo` where `$(...)` executed.

| Lesson | Detail |
|--------|--------|
| **A new feature does not have to introduce a vulnerability to expose one** | The injection was reachable for months. Routing a push through the same renderer is what made anyone look |
| **Per-flag escaping is the shape of this bug** | `push_authorisation` had already solved newline safety for its own value. Nothing else used it, so three other values stayed raw. One neutraliser every comment producer goes through is the fix; a fourth per-flag guard would have been the same defect again |
| **A marker is not an authorisation; its POSITION is** | `split_push_section` now honours the marker only as the script's final section, and `marker_line` refuses a payload that would spell it. Any caller text can spell a marker |
| **Ask a reviewer to reach the bad outcome, not to check the code** | "Find the path by which an unauthorised push reaches a remote" reproduced the forgery. "Does this look right?" would have passed it |

## Derive A Guard From The Thing It Defends Against

Round 2 found `comment_safe` neutralising `[\x00-\x1f\x7f]` while every reader
splits scripts with `str.splitlines()`, which ALSO breaks on U+0085, U+2028 and
U+2029. No forgery resulted, but a legitimate value holding one of those wrote a
script that later `--append` calls rejected as hand-edited.

The fix was not a wider character class. `comment_safe` now joins on
`str.splitlines()` itself, so "one line" means what the splitter means by it:

    " ".join(CONTROL_CHARS_RE.sub(" ", text).splitlines()).strip()

**A hand-kept list of line breaks must be kept in sync with the splitter. A
flattening derived from the splitter cannot fall behind it.** The same mismatch
existed in `rel_path` and was closed the same way. Invariant, stated and tested:
the result is unchanged by flattening it again, verified over 62,142 enumerated
inputs and 300,000 fuzz strings.

## Layered Guards Need Per-Layer Tests

Four independent layers flattened caller text. Reverting any ONE left the whole
suite green, because a sibling covered it. No exploit today; the risk is a later
"this flattening is redundant" refactor that stays green while removing the only
layer that mattered for some path.

**Test each layer at its own boundary, not through the artifact.** The suite now
calls `lesson_comment`, `render_block` and `push_authorisation` directly, and
each mutation reddens exactly one test.

## What Is Convention, Not A Gate

Nothing mechanically verifies that the owner ordered a push. `--push` takes a
recorded reason of at least 12 characters, and any 12 characters pass. The rule
says so plainly rather than implying a gate: the only mechanical gate is the
hook's refusal of the bare command.

An honest statement of a limit is worth more than a check that looks like one.

## Delegation Limit Worth Knowing

Both rule-editing agents were flagged by the harness for instruction poisoning:
they recorded an "owner amendment" whose authorisation existed only in the main
thread's conversation. **A subagent cannot see the operator's messages**, so a
rule change that records an owner decision can never carry its own evidence. The
main thread must verify that authorisation, because it is the only context that
holds it.

## Files

- `scripts/dev/commit_helper.py` - `--push`, `push_authorisation`, `render_push`, `split_push_section`, `comment_safe`, `marker_line`, `comment_line`, `rel_path`, `read_script_text`, `replaced_push_authorisation`
- `scripts/dev/commit_helper_test.py` - 134 tests, each new one mutation-proved
- `ai/INSTRUCTIONS.md` - the always-loaded rule, rewritten from absolute ban to sanctioned path
- `ai/rules/git-safety.md` - `## Pushing (2026-08-05, owner amendment)`, with history and failure modes under a `### Why...` heading the digest drops

## Known Limitations

- No mechanical check that a push was ordered. Convention plus a recorded string.
- `.claude/hooks/pretool-bash.py` blocks any Bash command whose TEXT contains the
  literal command, so an agent cannot grep for it in a file, including in the
  rule that governs it. Surprising, and it will surprise the next auditor.
- `--replace` drops a previously authorised push. Fail-safe, and now reported on
  stderr rather than silent.
