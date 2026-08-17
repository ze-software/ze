#!/usr/bin/env python3
"""Independent-review gate: record and check the review artifact that
`ai/rules/planning.md` requires before a spec may be closed.

The artifact `tmp/review/<spec-stem>-<session-id>.md` is written by INDEPENDENT
reviewers (subagents / a fresh session, never the author's own inline reasoning)
and is session-scoped so two agents working the same spec never clobber or ride
each other's review (see session_id()). It pins
the exact content of every code/test file they examined by SHA-256. The commit
gate (scripts/dev/commit_helper.py) refuses a spec-closure commit unless a
matching CLEAN artifact exists, so a review cannot be faked by narrating
"0 issues" into the spec: the artifact must cover the code being committed and
its hashes must still match (any post-review edit invalidates it, forcing a
fresh pass).

Content-hashing (not `git diff`) is deliberate: it captures untracked new files
and deletions, and is independent of staging state.

Usage:
  review_gate.py hash   --files F...                 # print per-file hashes
  review_gate.py record --spec STEM --verdict {clean|findings} --rounds N
                        --files F...
                        [--reviewers TEXT] [--findings-file PATH]
                        [--rounds-reason TEXT]  # required past ROUND_CAP rounds
  review_gate.py check  --spec STEM --files F...      # exit 0 pass / 3 block

Exit codes: 0 pass; 2 usage error; 3 gate BLOCK (missing/stale/dirty review).
"""

from __future__ import annotations

import argparse
import datetime
import hashlib
import importlib.util
import os
import re
import sys
from pathlib import Path

_SID_MODULE_PATH = (
    Path(__file__).resolve().parents[2] / ".claude" / "hooks" / "lib" / "session_id.py"
)
_sid_spec = importlib.util.spec_from_file_location(
    "ze_session_id", str(_SID_MODULE_PATH)
)
_ze_session_id = importlib.util.module_from_spec(_sid_spec)
_sid_spec.loader.exec_module(_ze_session_id)

ARTIFACT_DIR = Path("tmp/review")

# Round 1 reviews the whole diff, round 2 the fixes, round 3 the fixes to those
# (ai/rules/planning.md, "How each review round is scoped and when it ends"). A
# fourth round means the fixes are themselves producing findings, and the loop is
# no longer converging on the product. On 2026-08-09 a test-only change took seven
# passes: the code was clean after pass 1 and every later finding was a false
# statement in the spec's own closure prose, so each round's prose fix gave the
# next round fresh prose to audit.
# The cap is not a ban -- a genuinely defective implementation can need more. It
# costs one sentence naming what the extra round found in the PRODUCT, which is
# the sentence nobody can write when the loop is auditing its own bookkeeping.
ROUND_CAP = 3
HEADER_RE = re.compile(
    r"<!--\s*ze-review\s+spec=(?P<spec>\S+)\s+verdict=(?P<verdict>\S+).*?-->"
)
FILE_LINE_RE = re.compile(r"^\s{2}(?P<hash>[0-9a-f]{64}|DELETED)\s+(?P<path>.+)$")


def session_id() -> str:
    """Per-session component of the review-artifact filename.

    A safe direct id wins. A fork with an absent or invalid direct id delegates
    to the canonical resolver. Human and CI invocations keep the historical
    ``shared`` fallback and never cause the resolver to mint an id.
    """
    sid = _ze_session_id._sid_safe(os.environ.get("CLAUDE_CODE_SESSION_ID"))
    if sid:
        return sid
    if os.environ.get("CLAUDE_CODE_FORK_SUBAGENT"):
        return _ze_session_id.session_id()
    return "shared"


# Files whose correctness a critical review must cover. Prose (.md) and the
# spec/learned records are reviewed by other gates (doc review, the spec's own
# Review Gate). Everything logic-bearing is listed, INCLUDING hand-written build
# and template files (Makefile/.mk/.tmpl/.html) that a suffix-only whitelist once
# missed. A generated .go (e.g. plugin/all/all.go) is over-required rather than
# under (fail-closed): record it in the artifact -- covering a generated file is
# trivial. Note this is a whitelist, so a genuinely-new logic-bearing extension
# must be added here or it escapes review.
CODE_SUFFIXES = (
    ".go",
    ".ci",
    ".et",
    ".py",
    ".sh",
    ".yang",
    ".wb",
    ".mk",
    ".tmpl",
    ".html",
    ".c",
    ".rs",
    ".s",
    ".rego",
    ".tac",
)
CODE_BASENAMES = ("Makefile",)


def file_hash(path: str) -> str:
    p = Path(path)
    if not p.exists():
        return "DELETED"
    return hashlib.sha256(p.read_bytes()).hexdigest()


def spec_stem(spec: str) -> str:
    """Reduce any spelling of a spec reference to its bare stem.

    Accepts all three forms a caller reaches for, because all three occur in the
    wild and two of them used to fail silently:

        plan/spec-<stem>.md   the path ze-close and commit_helper.py carry
        spec-<stem>.md        the filename
        <stem>                the bare stem this tool documents

    Stripping only the "spec-" prefix left a leading "plan/" in place, so
    `--spec plan/spec-X.md` resolved to tmp/review/plan/spec-X-<session>.md, a
    directory that does not exist. The tool then reported BLOCKED with the
    remedy "run an independent review", which is the one thing that could not
    help: the review had been run and its artifact was sitting in tmp/review/
    under the right name. commit_helper.py never hit this because it derives the
    stem itself with _SPEC_STEM_RE; only direct callers did, and they read the
    BLOCKED as a missing review.
    """
    stem = spec.rsplit("/", 1)[-1]
    return stem.removeprefix("spec-").removesuffix(".md")


def artifact_path(spec: str) -> Path:
    return ARTIFACT_DIR / f"{spec_stem(spec)}-{session_id()}.md"


def is_code(path: str) -> bool:
    return path.endswith(CODE_SUFFIXES) or Path(path).name in CODE_BASENAMES


def cmd_hash(files: list[str]) -> int:
    for f in sorted(files):
        print(f"  {file_hash(f)}  {f}")
    return 0


# --------------------------------------------------------------------------- #
# The review model (ai/rules/planning.md).
#
# Review runs on Opus 5. Implementation no longer carries a model requirement
# (ai/rules/planning.md, 2026-08-03), so this is the only edit-adjacent model
# check left. Recording the artifact is the moment a review is
# CLAIMED, so it is the right place to check. The reverse boundary -- editing
# code on the review model -- is gated by nothing: c_model_phase went with the
# Opus 4.8 requirement, and no gate replaced it.


def _running_model() -> str:
    """The session's model via the ONE shared reader, or '' when unreadable."""
    try:
        sys.path.insert(0, str(Path(__file__).resolve().parent))
        import running_model as rm

        return rm.running_model()
    except Exception:
        return ""


def _model_refusal(force: str) -> str:
    """Why this session may not record a review, or '' when it may."""
    model = _running_model()
    if not model:
        # Cannot tell. Say so and allow: a gate that guesses would refuse real
        # reviews on a machine whose transcript it cannot read.
        print(
            "review_gate: WARNING could not determine the running model; "
            "the review-model boundary is UNCHECKED "
            "(ai/rules/planning.md)",
            file=sys.stderr,
        )
        return ""
    try:
        sys.path.insert(0, str(Path(__file__).resolve().parent))
        import running_model as rm

        if rm.is_review_tier(model):
            return ""
    except Exception:
        return ""
    if force:
        print(
            f"review_gate: WARNING recording a review made on {model}, "
            f"not the review model. Operator reason: {force}",
            file=sys.stderr,
        )
        return ""
    return (
        f"review_gate: BLOCKED this session is on {model}. Review runs on "
        "Opus 5 (ai/rules/planning.md).\n"
        "  A review performed on the implementation model is the author "
        "grading their own work,\n"
        "  which is the failure the independent-review rule exists to "
        "prevent (ai/rules/planning.md).\n"
        "  Switch to Opus 5 and re-run the review, or pass --model-override "
        "with the operator's reason."
    )


def cmd_record(args: argparse.Namespace) -> int:
    override = str(getattr(args, "model_override", "") or "")
    refusal = _model_refusal(override)
    if refusal:
        print(refusal, file=sys.stderr)
        return 2
    files = sorted(set(args.files))
    if not files:
        print("review_gate: record needs --files", file=sys.stderr)
        return 2
    verdict = args.verdict.lower()
    if verdict not in ("clean", "findings"):
        print("review_gate: --verdict must be clean|findings", file=sys.stderr)
        return 2
    rounds = int(args.rounds)
    rounds_reason = str(getattr(args, "rounds_reason", "") or "").strip()
    if rounds < 1:
        print(
            "review_gate: --rounds must be at least 1; an artifact claiming zero "
            "passes is a review that never ran",
            file=sys.stderr,
        )
        return 2
    if rounds > ROUND_CAP and not rounds_reason:
        print(
            f"review_gate: {rounds} review rounds needs --rounds-reason "
            f"(the cap is {ROUND_CAP}).\n"
            "  Name the PRODUCT defect a round past the cap found: wrong behavior, "
            "a missing test, an unwired symbol, a guard that fails open.\n"
            "  A false statement in the spec's own closure prose is NOT one. Fix "
            "those in one edit and stop the loop; they ship nothing.\n"
            "  See ai/rules/planning.md, 'How each review round is scoped and when "
            "it ends'.",
            file=sys.stderr,
        )
        return 2
    findings = ""
    if args.findings_file:
        fp = Path(args.findings_file)
        if fp.exists():
            findings = fp.read_text(encoding="utf-8").strip()
    ts = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    ARTIFACT_DIR.mkdir(parents=True, exist_ok=True)
    out = artifact_path(args.spec)
    stem = args.spec.removeprefix("spec-").removesuffix(".md")
    lines = [
        f"<!-- ze-review spec={stem} verdict={verdict} rounds={rounds} reviewers={args.reviewers or 'unspecified'} model={_running_model() or 'unknown'} ts={ts} -->",
        f"# Independent review — {stem}",
        "",
        "files:",
    ]
    for f in files:
        lines.append(f"  {file_hash(f)}  {f}")
    if override:
        lines += ["", f"model-override: {override}"]
    if rounds_reason:
        lines += ["", f"rounds-reason: {rounds_reason}"]
    lines += ["", "## Findings", "", findings or "(none recorded)", ""]
    out.write_text("\n".join(lines), encoding="utf-8")
    print(f"review_gate: wrote {out} ({len(files)} files, verdict={verdict})")
    return 0


def _parse_artifact(spec: str) -> tuple[str, dict[str, str]] | None:
    out = artifact_path(spec)
    if not out.exists():
        return None
    text = out.read_text(encoding="utf-8")
    header = HEADER_RE.search(text)
    if not header:
        return None
    hashes: dict[str, str] = {}
    for line in text.splitlines():
        m = FILE_LINE_RE.match(line)
        if m:
            hashes[m.group("path")] = m.group("hash")
    return header.group("verdict").lower(), hashes


def _report_recorded_model(spec: str) -> None:
    """Say which model produced the artifact when it was not the review model.

    Recording stamps `model=` into the header, and nothing read it back, so a
    review recorded under --model-override or while the reader was blind looked
    identical to a clean one at check time.
    """
    try:
        text = artifact_path(spec).read_text(encoding="utf-8")
    except Exception:
        return
    m = re.search(r"model=(\S+)", text)
    if not m:
        return
    model = m.group(1)
    try:
        sys.path.insert(0, str(Path(__file__).resolve().parent))
        import running_model as rm

        on_review_model = rm.is_review_tier(model)
    except Exception:
        on_review_model = False
    if not on_review_model:
        print(
            f"review_gate: NOTE this artifact was recorded on {model}, not the "
            "review model (ai/rules/planning.md)",
            file=sys.stderr,
        )


def cmd_check(args: argparse.Namespace) -> int:
    _report_recorded_model(args.spec)
    parsed = _parse_artifact(args.spec)
    out = artifact_path(args.spec)
    if parsed is None:
        _block(
            f"no independent-review artifact at {out}",
            "Run an INDEPENDENT critical review (subagents / fresh session, never "
            "your own inline reasoning) and record it with review_gate.py record. "
            "See ai/rules/planning.md.",
        )
        return 3
    verdict, hashes = parsed
    code_files = sorted(f for f in set(args.files) if is_code(f))
    unreviewed = [f for f in code_files if f not in hashes]
    stale = [f for f in code_files if f in hashes and hashes[f] != file_hash(f)]
    if verdict != "clean":
        _block(
            f"review artifact {out} verdict is {verdict!r}, not clean",
            "Fix every BLOCKER/ISSUE, then re-run the independent review to a clean pass.",
        )
        return 3
    if unreviewed:
        _block(
            f"{len(unreviewed)} code file(s) in the commit were not covered by the review",
            "Unreviewed: " + ", ".join(unreviewed) + "\n"
            "  Re-run the independent review over the FULL changeset and re-record.",
        )
        return 3
    if stale:
        _block(
            f"{len(stale)} reviewed file(s) changed AFTER the review (stale review)",
            "Changed since review: " + ", ".join(stale) + "\n"
            "  Every fix is new code that needs a fresh review. Re-review and re-record.",
        )
        return 3
    print(f"review_gate: OK ({len(code_files)} code files, clean, hashes match {out})")
    return 0


def _block(headline: str, detail: str) -> None:
    print(f"review-gate: BLOCKED — {headline}\n  {detail}", file=sys.stderr)


def main(argv: list[str]) -> int:
    ap = argparse.ArgumentParser(description=__doc__)
    sub = ap.add_subparsers(dest="cmd", required=True)

    h = sub.add_parser("hash")
    h.add_argument("--files", nargs="+", required=True)

    r = sub.add_parser("record")
    r.add_argument("--spec", required=True)
    r.add_argument("--verdict", required=True)
    r.add_argument("--files", nargs="+", required=True)
    r.add_argument("--reviewers")
    r.add_argument("--findings-file")
    r.add_argument(
        "--rounds",
        required=True,
        type=int,
        help="how many independent review passes ran. Written into the artifact. "
        f"More than {ROUND_CAP} needs --rounds-reason",
    )
    r.add_argument(
        "--rounds-reason",
        default="",
        help=f"required past round {ROUND_CAP}: the PRODUCT defect a later round "
        "found. A finding in the spec's own closure prose is not one",
    )
    r.add_argument(
        "--model-override",
        default="",
        help="operator reason to record a review made off the review model "
        "(ai/rules/planning.md). Their call, not yours.",
    )

    c = sub.add_parser("check")
    c.add_argument("--spec", required=True)
    # nargs="*": a code-free closure commit still requires a CLEAN artifact to
    # EXIST (an author cannot commit code in earlier commits then close with a
    # bare learned summary and no review on record).
    c.add_argument("--files", nargs="*", default=[])

    args = ap.parse_args(argv)
    if args.cmd == "hash":
        return cmd_hash(args.files)
    if args.cmd == "record":
        return cmd_record(args)
    if args.cmd == "check":
        return cmd_check(args)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
