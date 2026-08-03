#!/usr/bin/env python3
"""Which model is driving this session.

`ai/rules/planning.md` puts planning and review on Opus 5 and
implementation on Opus 4.8. Nothing could check that, because no tool is told
which model it is running under. One thing does know: the session transcript,
which records `message.model` on every assistant turn.

This is the ONE reader of that fact. Two gates use it, and a second copy would
drift from the first:

  * `c_model_phase` in `.claude/hooks/pretool-writeedit.py` -- refuses an
    implementation edit on a planning/review model.
  * `scripts/dev/review_gate.py record` -- refuses to record a review that was
    not performed on the review model.

It answers "" when it cannot tell, and every caller must then stand down and say
so. A gate that guesses a model would be worse than one that admits it cannot
see: it would attribute work to the wrong phase and block the right one.
"""

from __future__ import annotations

import json
import os
import sys

# One oversized tool result can fill a tail, so read enough to clear a few.
TAIL_BYTES = 1_048_576

REVIEW_TIER = ("opus-5",)


def transcript_dir() -> str:
    """Claude's per-project transcript directory for this checkout."""
    root = os.environ.get("CLAUDE_PROJECT_DIR") or os.getcwd()
    # Claude slugifies the checkout path: every "/" AND every "." becomes "-".
    # Missing the dot yields "github.com" where the real directory says
    # "github-com", and the lookup silently finds nothing.
    slug = os.path.abspath(root).replace("/", "-").replace(".", "-")
    return os.path.join(os.path.expanduser("~"), ".claude", "projects", slug)


def transcript_path() -> str:
    """This session's transcript, or '' when it cannot be identified.

    When a session id is exported, its file is the ONLY acceptable answer. If
    that file is missing (a new, resumed, or compacted session) the answer is "".
    Falling through to "most recently written" would hand back a NEIGHBOUR
    session's model: this project directory routinely holds three live
    transcripts, and the mtime winner changes from second to second. A wrong
    model is worse than no model, because it confidently blocks correct work and
    confidently passes an off-model review.

    The mtime fallback survives only for the case it is actually right for: no
    session id at all, which means a single interactive session.
    """
    sid = os.environ.get("CLAUDE_CODE_SESSION_ID", "").strip()
    d = transcript_dir()
    if sid:
        p = os.path.join(d, f"{sid}.jsonl")
        return p if os.path.isfile(p) else ""
    try:
        entries = [os.path.join(d, n) for n in os.listdir(d) if n.endswith(".jsonl")]
    except Exception:
        return ""
    entries = [p for p in entries if os.path.isfile(p)]
    if not entries:
        return ""
    return max(entries, key=os.path.getmtime)


def running_model(path: str | None = None) -> str:
    """Model of the last MAIN-THREAD assistant message, or '' if unreadable.

    Subagent lines carry their own model and are skipped. Taking one would let
    any sonnet or haiku helper answer for the session that spawned it.

    `path=None` means "work it out". `path=""` means "the caller HAD a path and
    it was empty", which is not the same thing and must not fall back to a
    guess: the hook that passes a payload path would otherwise inherit the
    fallback it was written to avoid.
    """
    if path == "":
        return ""
    path = path or transcript_path()
    if not path or not os.path.isfile(path):
        return ""
    try:
        size = os.path.getsize(path)
        with open(path, "rb") as fh:
            if size > TAIL_BYTES:
                fh.seek(size - TAIL_BYTES)
                fh.readline()  # discard the partial line
            lines = fh.read().decode("utf-8", "replace").splitlines()
    except Exception:
        return ""
    for line in reversed(lines):
        if '"model"' not in line:
            continue
        try:
            d = json.loads(line)
        except Exception:
            continue
        if d.get("isSidechain"):
            continue
        model = (d.get("message") or {}).get("model")
        if model:
            return model
    return ""


def is_review_tier(model: str) -> bool:
    return bool(model) and any(m in model for m in REVIEW_TIER)


if __name__ == "__main__":
    m = running_model()
    print(m or "unknown")
    sys.exit(0 if m else 1)
