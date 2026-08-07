#!/usr/bin/env python3
"""PreToolUse Agent|Task: refuse a raw agent when a ze-* skill covers the task.

ai/rules/cli.md: "When a skill covers the task (/ze-rfc, /ze-review,
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

# A `ze point:` comment directly above a gate names the rule point it enforces
# (`<rule-stem>/<slug>` under ai/rules/points/), or `none -- <why>`. Joined by
# `make ze-rules-gate-map`, which fails on a point that does not exist.

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


# ze point: cli/agent-tooling-contract/use-the-skill-instead-of-a-raw-agent
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


# Skills whose work IS review. ai/rules/planning.md puts review on Opus 5,
# and a review done on the implementation model is the author grading their own
# work. The skills gate already knows when a prompt asks for a review, so it is
# the earliest place that can say so -- before the agent runs, rather than when
# the artifact is recorded.
_REVIEW_SKILLS = (
    "/ze-review",
    "/ze-review-spec",
    "/ze-review-deep",
    "/ze-audit",
    "/ze-close",  # it RUNS review_gate.py record, so it is review-phase work
)
# Telling a review apart from work ABOUT a review is a question of the verb, not
# of where a word sits in the line. A line-anchored routing regex got it wrong in
# both directions: it missed "Please follow /ze-review ...", "/ze-review the
# diff" and "You are the /ze-review agent", and it caught "Per /ze-review
# findings, fix the parser bug", which is implementation.
#
# So: a prompt that names a review skill IS review work, unless an
# implementation verb governs it. Fixing what a review found is implementation
# and belongs on the implementation model.
_ANY_SKILL_REF = re.compile(r"(/ze-[a-z0-9-]+)")
_IMPLEMENTATION_VERB = re.compile(
    # "fixes" is the NOUN in "review over the fixes", so only the verb form
    # counts. That one word made a round-2 review prompt read as implementation.
    r"\b(apply|fix|implement|update|edit|rewrite|refactor|rename|migrate|"
    r"add|write|remove|delete|port|wire)\b",
    re.IGNORECASE,
)
# Only the opening of a prompt says what the agent is FOR. A review prompt that
# later says "fix" (as an instruction to the reviewer) must still read as review.
_VERB_WINDOW = 160


def _load_reader():
    root = os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd()
    sys.path.insert(0, os.path.join(root, "scripts", "dev"))
    import running_model as rm

    return rm


def _running_model(transcript: str | None = None) -> str:
    """The session's model via the ONE shared reader, or '' when unreadable.

    The payload's transcript_path is passed through when present. Throwing it
    away and re-resolving made this gate answer differently from the edit gate
    for the same session.
    """
    try:
        return _load_reader().running_model(transcript)
    except Exception:
        return ""


def rm_is_review_tier(model: str) -> bool:
    """Tier test from the shared reader, so the literal lives in ONE place."""
    try:
        return _load_reader().is_review_tier(model)
    except Exception:
        return True  # cannot tell the tier: do not block


def _ack_recorded() -> bool:
    """The operator's recorded decision, the same escape the edit gate uses."""
    root = os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd()
    sid = os.environ.get("CLAUDE_CODE_SESSION_ID", "").strip()
    if not sid:
        return False
    path = os.path.join(root, "tmp", "session", f".model-ack-{sid}")
    try:
        # An EMPTY file is not a recorded decision. Requiring the reason makes
        # the escape an act rather than a touch.
        return len(open(path, encoding="utf-8").read().strip()) >= 10
    except Exception:
        return False


def _is_review_work(prompt: str) -> bool:
    """Will this agent PERFORM a review, or work on the output of one?"""
    if _IMPLEMENTATION_VERB.search(prompt[:_VERB_WINDOW]):
        return False
    names_review = any(
        m.group(1).lower() in _REVIEW_SKILLS for m in _ANY_SKILL_REF.finditer(prompt)
    )
    if names_review:
        return True
    hit = verdict(prompt)
    return bool(hit) and hit[0] in _REVIEW_SKILLS


# ze point: planning/work-phases/run-every-review-on-opus-5
def review_model_refusal(prompt: str, transcript: str | None = None) -> str:
    """Why this review may not run here, or '' when it may."""
    if not _is_review_work(prompt):
        return ""
    if _ack_recorded():
        return ""
    model = _running_model(transcript)
    if not model:
        # Fail-closed guards must deny or SPEAK. This one cannot deny, so it
        # says so (ai/rules/evidence.md). The record gate still checks.
        sys.stderr.write(
            "note: could not determine the running model, so the review-model "
            "boundary is UNCHECKED here (ai/rules/planning.md)\n"
        )
        return ""
    if rm_is_review_tier(model):
        return ""
    return (
        f"\u274c Blocked: review runs on Opus 5, and this session is on {model}\n"
        "  (ai/rules/planning.md).\n"
        "  A subagent inherits the PHASE, not the task shape, so spawning a\n"
        "  reviewer from an implementation session still reviews on the wrong\n"
        "  model. Worse, it is usually the session that wrote the code.\n"
        "  Say so and stop, so the operator can switch or start a review session.\n"
        "  If the operator decides otherwise, their reason goes in\n"
        "  tmp/session/.model-ack-<sid>, the same escape the edit gate uses."
    )


def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except Exception:
        return 0
    if payload.get("tool_name") not in ("Agent", "Task"):
        return 0
    ti = payload.get("tool_input") or {}
    prompt = str(ti.get("prompt") or "")
    refusal = review_model_refusal(prompt, payload.get("transcript_path"))
    if refusal:
        sys.stderr.write(refusal + "\n")
        return 2
    hit = verdict(prompt)
    if hit is None:
        return 0
    skill, matched = hit
    sys.stderr.write(
        "❌ Blocked: a ze-* skill covers this agent's task "
        "(ai/rules/cli.md).\n"
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
