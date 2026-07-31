#!/usr/bin/env python3
"""PreToolUse Agent|Task: refuse a raw agent when a ze-* skill covers the task.

ai/rules/agent-tooling.md: "When a skill covers the task (/ze-rfc, /ze-review,
/ze-implement, etc.), use it instead of spawning a raw agent or improvising the
workflow." Nothing enforced that. On 2026-07-31 a session ran three research
fan-outs and two reviews as hand-written prompts, reproducing a worse version of
/ze-explore and /ze-review, and lost every gate those skills carry.

The rule is not "always use a skill". Plenty of agent work has no covering skill.
So this matches on what the prompt ASKS FOR, and only blocks when the ask lands
squarely on a skill that exists. Naming the skill anywhere in the prompt, or
loading it with the Skill tool, satisfies the check.

Exit 2 (block) with the skill named. Exit 0 otherwise. Unparsable input exits 0:
a broken guard must never wedge delegation.
"""

from __future__ import annotations

import json
import os
import re
import sys

# (skill, regex over the prompt) -- ordered, first match wins.
# Each pattern names the ASK, never the subject matter: "review this diff" is a
# review, while "explain how review works" is research about reviews.
SKILL_TRIGGERS = (
    (
        "/ze-review",
        r"\b(review|critique|audit)\b[^.]{0,60}\b(diff|change|commit|branch|"
        r"implementation|code|pr)\b|\bfind (bugs|issues|defects|problems)\b|"
        r"\b(blocker|adversarial(ly)? (review|verify))\b",
    ),
    (
        "/ze-review-spec",
        r"\breview\b[^.]{0,40}\bagainst\b[^.]{0,20}\bspec\b|"
        r"\bacceptance criteri\w+\b[^.]{0,40}\b(met|verified|implemented)\b",
    ),
    (
        "/ze-debug",
        r"\b(debug|diagnose|root[- ]cause)\b[^.]{0,60}\b(failure|failing|red|"
        r"test|gate|hang|crash|flake)\b",
    ),
    (
        "/ze-explore",
        r"\b(survey|explore|investigate|research|map out|find (out |every |all )?"
        r"(where|which|how))\b",
    ),
    (
        "/ze-audit",
        r"\baudit\b[^.]{0,60}\b(spec|requirement|implementation|coverage)\b",
    ),
    (
        "/ze-implement",
        r"\bimplement\b[^.]{0,60}\b(spec|feature|ac-\d)\b",
    ),
    (
        "/ze-hunt",
        r"\bhunt\b[^.]{0,40}\b(bug|class|pattern)\b",
    ),
)

# A prompt that names a REAL skill has been routed deliberately.
#
# The first version matched any `ze-<word>`, which the repo path (`ze-software`)
# and every `make ze-verify` in a prompt satisfied. That switched the gate off
# for almost every real prompt in this repository. Two things fix it: the slash
# is required, and the name must be a skill that exists on disk.
_SKILL_REF = re.compile(r"/(ze-[a-z0-9-]+)")


def _known_skills():
    """Skill names from ai/skills/. Empty set when unreadable, which makes the
    gate stricter, never looser."""
    root = os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd()
    try:
        return {
            name[:-3]
            for name in os.listdir(os.path.join(root, "ai", "skills"))
            if name.startswith("ze-") and name.endswith(".md")
        }
    except Exception:
        return set()


def names_a_skill(prompt: str) -> bool:
    known = _known_skills()
    return any(m.group(1) in known for m in _SKILL_REF.finditer(prompt))


def verdict(prompt: str) -> tuple[str, str] | None:
    """(skill, matched-text) when a skill covers this prompt, else None."""
    if not prompt:
        return None
    if names_a_skill(prompt):
        return None
    low = prompt.lower()
    for skill, pattern in SKILL_TRIGGERS:
        m = re.search(pattern, low)
        if m:
            return (skill, m.group(0).strip())
    return None


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0
    if payload.get("tool_name") not in ("Agent", "Task"):
        return 0
    ti = payload.get("tool_input") or {}
    hit = verdict(str(ti.get("prompt") or ""))
    if hit is None:
        return 0
    skill, matched = hit
    sys.stderr.write(
        "❌ Blocked: a ze-* skill covers this agent's task "
        "(ai/rules/agent-tooling.md).\n"
        f'  The prompt asks for: "{matched}"\n'
        f"  Use {skill} instead of a hand-written prompt. The skill carries the\n"
        "  workflow, the gates and the report format that a raw agent improvises\n"
        "  and usually drops.\n"
        f"  Invoke it with the Skill tool, or name {skill} in the agent prompt so\n"
        "  the agent follows it. Delegation itself needs no permission.\n"
    )
    return 2


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:  # a broken guard must never wedge delegation
        sys.exit(0)
