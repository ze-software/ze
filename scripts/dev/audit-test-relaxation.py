#!/usr/bin/env python3
"""Audit a diff for deleted or weakened tests, and report them.

Backs the test-relaxation pass of /ze-review and /ze-review-deep. Where the
PreToolUse hook (c_test_weakening) guards a single edit as it happens, this runs
over a whole branch/working-tree diff so weakening that slipped past the hook
(a relax token, an out-of-band commit, an expected-value tweak the hook cannot
see) is still surfaced for human review.

The weakening detection is IMPORTED from .claude/hooks/pretool-writeedit.py
(_test_weakening_errs) so the audit and the hook can never drift apart.

Usage:
    python3 scripts/dev/audit-test-relaxation.py             # uncommitted vs HEAD
    python3 scripts/dev/audit-test-relaxation.py origin/main # committed work + worktree
    python3 scripts/dev/audit-test-relaxation.py --selftest  # internal tests

This repo commits DIRECTLY TO MAIN, so `main` is normally the same commit as HEAD
and `... .py main` would audit an empty commit range. That used to print a clean
verdict; it is now refused (exit 2). Use origin/main to audit work that is
committed but not yet pushed. On a feature branch, a base of `main` works as it
always did.

A base is only usable if it gives a real comparison. "Clean" must mean "I
compared things and found nothing", never "I compared nothing"
(ai/rules/evidence.md).

Exit 0 = a comparison ran and found no test deletion/weakening/relaxation.
Exit 1 = findings reported (review them).
Exit 2 = audit could not run: detection logic not importable, or the base is
         unusable (nonexistent, unrelated, or an empty range auditing nothing).
"""

import collections
import importlib.util
import os
import re
import subprocess
import sys
import tempfile
import textwrap
from typing import NamedTuple

RED = "\033[31m"
YELLOW = "\033[33m"
GREEN = "\033[32m"
BOLD = "\033[1m"
RESET = "\033[0m"

_RELAX_LINE = re.compile(r"//[ \t]*test-relax:[ \t]*(\S.*)?$")
# The `.ci` and `.et` carriers this audit examines comment with `#`, so a relaxation
# written in their own syntax was invisible here and its reason went unreported. One
# alternation rather than a union of two patterns: `# // test-relax:` matches at the
# `//` only (the `#` branch requires the token immediately after it), so a line is
# never counted twice.
#
# Only the two extensions `is_test_path` can yield. A `.py` arm would be unreachable
# here, whatever the hook's own carrier list says.
_RELAX_LINE_ANY = re.compile(r"(?://|#)[ \t]*test-relax:[ \t]*(\S.*)?$")
# Named apart from the hook's `_HASH_COMMENT_CARRIERS` on purpose: this audit imports
# the hook's DETECTORS but not its carrier list, and the two lists differ (the hook adds
# `.py`, reachable there through `_carries_rfc_tag`). One name with two contents, in
# files that already share code, is a trap for whoever greps it next.
_AUDITED_HASH_CARRIERS = (".ci", ".et")

_CONT_HASH = re.compile(r"^[ \t]*#[ \t]?(.*)$")
_CONT_SLASH = re.compile(r"^[ \t]*//[ \t]?(.*)$")
# A justification longer than this is quoted in full nowhere a reviewer will read it.
# The cap truncates the QUOTE; it never changes which findings are reported.
_REASON_MAX_LINES = 12


def relax_reasons(text, path):
    """Every relaxation justification in `text`, in file order, for `path`'s syntax.

    A justification is its token line PLUS the comment lines under it. Capturing
    only the first line was not a cosmetic limit: measured over the 751 tokens at
    HEAD on 2026-08-10, 62% ran past one line, the median was 3 and the longest 21.
    So `report()` showed a reviewer a fragment -- often one with no verb in it --
    and asked them to confirm the relaxation was justified.

    The walk stops at the first line that is not a comment, at the next token, and
    at `_REASON_MAX_LINES`. It can absorb an unrelated comment paragraph that
    happens to sit directly under a token. That trade is deliberate: this text is
    read by a human deciding whether a relaxation is honest, and too much context
    costs them a second, where too little cost them the decision.
    """
    hashed = path.endswith(_AUDITED_HASH_CARRIERS)
    token = _RELAX_LINE_ANY if hashed else _RELAX_LINE
    cont = _CONT_HASH if hashed else _CONT_SLASH
    lines = text.split("\n")
    out = []
    for i, line in enumerate(lines):
        m = token.search(line)
        if not m:
            continue
        parts = [(m.group(1) or "").strip()]
        for nxt in lines[i + 1 : i + _REASON_MAX_LINES]:
            if token.search(nxt):
                break
            c = cont.match(nxt)
            if not c or not c.group(1).strip():
                break
            parts.append(c.group(1).strip())
        out.append(" ".join(p for p in parts if p))
    return out


def git(args, cwd):
    return subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)


def load_hook_module(repo_root):
    """Import the canonical hook so its detection logic is SHARED, never reimplemented."""
    hook = os.path.join(repo_root, ".claude", "hooks", "pretool-writeedit.py")
    if not os.path.isfile(hook):
        return None
    spec = importlib.util.spec_from_file_location("ze_pretool_we", hook)
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)
    except Exception:
        return None
    return mod


def load_detector(repo_root):
    """Import _test_weakening_errs from the canonical hook so logic stays shared."""
    mod = load_hook_module(repo_root)
    return getattr(mod, "_test_weakening_errs", None) if mod else None


def load_rfc_detector(repo_root):
    """Import _rfc_tagged_change_err from the canonical hook -- the SAME detector object.

    The hook sees one edit as it happens; this sees a whole branch, so it catches an
    RFC-tagged test changed out-of-band with NO approval token: an edit made before the
    hook existed, or one made while the hook was disabled. On those the shared detector
    still fires.

    It does NOT provide a second opinion on a forged token. Because this audit imports the
    hook's exact detector, the `rfc-test-change-approved:` token that silences the hook
    silences this audit too -- the detector returns None the moment the new content carries
    one (see _rfc_tagged_change_err). A self-written token therefore defeats BOTH gates.
    The only backstop against one is `grep -rn 'rfc-test-change-approved:'` plus human
    review of each hit, which the hook's own block message already instructs. Do not read
    this audit as catching a token an agent wrote for itself; it cannot
    (ai/rules/evidence.md, ai/rules/evidence.md).
    """
    mod = load_hook_module(repo_root)
    return getattr(mod, "_rfc_tagged_change_err", None) if mod else None


def is_test_path(p):
    if p.endswith("_test.go"):
        return True
    if (p.endswith(".ci") or p.endswith(".et")) and (
        p.startswith("test/") or "/test/" in p
    ):
        return True
    return False


def _suggest_remote_base(base, cwd, head_sha):
    """Suggest origin/<base> when it exists and WOULD give a real comparison.

    A suggestion, never a substitution: silently retargeting the audit at a base
    the caller did not name would swap one wrong answer for another, and the
    caller still would not know what was compared.
    """
    if "/" in base:
        return None
    remote = f"origin/{base}"
    if git(["rev-parse", "--verify", f"{remote}^{{commit}}"], cwd).returncode != 0:
        return None
    mb = git(["merge-base", remote, "HEAD"], cwd)
    if mb.returncode != 0 or mb.stdout.strip() == head_sha:
        return None
    ahead = git(["rev-list", "--count", f"{remote}..HEAD"], cwd).stdout.strip()
    return remote, ahead


def resolve_anchor(base, cwd):
    """Resolve the commit to diff the worktree against.

    Returns (anchor, err). A non-None err means there is no honest comparison to
    make and the audit MUST NOT run: every path here used to fall through to an
    empty finding list, which report() then printed as a clean bill of health.
    The empty list is the zero-value trap -- it cannot distinguish "compared and
    found nothing" from "compared nothing" -- so the miss is made explicit here,
    at the only layer that knows it happened (ai/rules/evidence.md).
    """
    head = git(["rev-parse", "--verify", "HEAD^{commit}"], cwd)
    if head.returncode != 0:
        return None, "HEAD does not resolve to a commit (empty repository?)."
    head_sha = head.stdout.strip()

    # The default. "Diff the worktree against HEAD" is a real comparison, so an
    # anchor of HEAD is legitimate here and only here.
    if base == "HEAD":
        return head_sha, None

    if git(["rev-parse", "--verify", f"{base}^{{commit}}"], cwd).returncode != 0:
        return None, (
            f"base {base!r} does not resolve to a commit (typo, or a ref that was "
            f"never fetched).\n"
            f"  Nothing was compared, so no verdict can be given."
        )

    mb = git(["merge-base", base, "HEAD"], cwd)
    if mb.returncode != 0 or not mb.stdout.strip():
        return None, (
            f"base {base!r} shares no common ancestor with HEAD, so there is no "
            f"range to audit."
        )
    anchor = mb.stdout.strip()

    if anchor == head_sha:
        msg = (
            f"base {base!r} resolves to the same commit as HEAD ({head_sha[:12]}), so "
            f"the range {base}..HEAD holds no commits.\n"
            f"  Auditing it would compare nothing and report a pass. This repo commits "
            f"directly to main, so a local\n"
            f"  branch name is almost never the base you want. Try:"
        )
        hint = _suggest_remote_base(base, cwd, head_sha)
        cmds = []
        if hint:
            remote, ahead = hint
            cmds.append(
                (
                    remote,
                    f"{ahead} commit(s) here are not on {remote}, plus the worktree",
                )
            )
        cmds.append(("HEAD", "audit uncommitted changes only"))
        width = max(len(c) for c, _ in cmds)
        for cmd, why in cmds:
            msg += f"\n    audit-test-relaxation.py {cmd:<{width}}  # {why}"
        return None, msg

    return anchor, None


def changed_test_files(anchor, cwd):
    """Return (rows, err) of (status, old_path, new_path) for changed test files.

    A failed diff returns err, not []: an empty row list must only ever mean the
    range genuinely contains no test-file changes.
    """
    out = git(["diff", "--name-status", "-M", anchor, "--"], cwd)
    if out.returncode != 0:
        return None, (
            f"git diff against {anchor[:12]} failed, so nothing was compared:\n"
            f"  {out.stderr.strip()}"
        )
    rows = []
    for line in out.stdout.splitlines():
        parts = line.split("\t")
        if len(parts) < 2:
            continue
        status = parts[0]
        if status.startswith("R") and len(parts) >= 3:
            old_p, new_p = parts[1], parts[2]
        else:
            old_p = new_p = parts[1]
        if is_test_path(old_p) or is_test_path(new_p):
            rows.append((status, old_p, new_p))
    return rows, None


def read_worktree(path, cwd):
    full = os.path.join(cwd, path)
    try:
        with open(full, encoding="utf-8", errors="replace") as fh:
            return fh.read()
    except OSError:
        return None


class Audit(NamedTuple):
    anchor: str  # commit the worktree was diffed against
    findings: list  # (kind, path, details:list[str])
    examined: int  # test files actually inspected; the verdict's evidence
    err: str  # why no comparison was possible, or "" when one ran


def run_audit(base, cwd, detector, rfc_detector=None):
    anchor, err = resolve_anchor(base, cwd)
    if err:
        return Audit("", [], 0, err)
    rows, err = changed_test_files(anchor, cwd)
    if err:
        return Audit(anchor, [], 0, err)
    findings = []  # (kind, path, details:list[str])
    # Added files are counted but never inspected: a brand-new test cannot be a
    # weakening of anything. The verdict reports what it actually looked at.
    examined = sum(1 for s, _, _ in rows if s != "A")
    for status, old_p, new_p in rows:
        if status == "A":
            continue
        if status == "D":
            findings.append(("DELETED", old_p, []))
            continue
        old = git(["show", f"{anchor}:{old_p}"], cwd).stdout
        new = read_worktree(new_p, cwd)
        if new is None:
            new = git(["show", f"HEAD:{new_p}"], cwd).stdout
        details = detector(old, new, new_p) if detector else []
        # An RFC-tagged test is the proof behind a public compliance claim
        # (docs/features/rfc-status.md), so ANY behavior change to one is reportable --
        # not only the count-based weakening the heuristic above can see. Swapping an
        # expected value keeps every count identical and would otherwise pass silently.
        rfc_tags = rfc_detector(old, new, new_p) if rfc_detector else None
        if rfc_tags:
            details = details + [
                "RFC-TAGGED test changed without an approval token: "
                + ", ".join(rfc_tags),
                "  the user must approve this; see rfc-test-change-approved:",
            ]
        # A multiset difference, not a positional slice. The slice assumed an added
        # token lands LAST in file order, so a token inserted at the top of a file
        # that already had two made the audit quote the wrong reason back -- it
        # named an old relaxation and stayed silent about the new one. It also went
        # entirely blind when an edit deleted one token and added another, because
        # the count was unchanged and the slice was therefore empty. Both are pinned
        # by audit_relaxation_test.py: test_a_token_inserted_at_the_top_is_the_one
        # _reported and test_deleting_one_token_and_adding_another_is_not_silent.
        old_list = relax_reasons(old, new_p)
        new_list = relax_reasons(new, new_p)
        added_tokens = list(
            (collections.Counter(new_list) - collections.Counter(old_list)).elements()
        )
        # What DISAPPEARED matters as much as what arrived, and a count comparison
        # cannot tell one story from the other. Judging on the count alone called a
        # token drain ("deleted 3 stale, wrote 1 real") a REWORD, which is false, and
        # scored it at the BLOCKER tier -- penalising exactly the cleanup the ceiling
        # gate exists to encourage. So state facts instead of inferring intent: the
        # severity turns on whether this change ALSO disturbed justifications that
        # were already there, and both lists are printed either way.
        removed_tokens = list(
            (collections.Counter(old_list) - collections.Counter(new_list)).elements()
        )
        label = "RENAMED " if status.startswith("R") else ""
        quoted = [f"reason: {t.strip() or '(empty!)'}" for t in added_tokens]
        if removed_tokens:
            quoted += [
                f"a justification that was already here is GONE: "
                f"{t.strip() or '(empty!)'}"
                for t in removed_tokens
            ]
        if added_tokens and not (details and removed_tokens):
            findings.append(
                ("RELAXED", new_p, [label.strip()] * bool(label) + details + quoted)
            )
        elif details or added_tokens:
            # Weakened AND an existing justification disturbed. Two different changes
            # wear that shape -- a reword laundering a relaxation, and an honest drain
            # that retires a stale token while writing a real one -- and nothing here
            # can tell them apart. So it reports both lists and escalates, leaving the
            # judgement to the reader who can see which it is.
            findings.append(
                ("WEAKENED", new_p, [label.strip()] * bool(label) + details + quoted)
            )
    return Audit(anchor, findings, examined, "")


def report(audit, base):
    anchor, findings = audit.anchor, audit.findings
    if not findings:
        # The verdict states what it is based on. A pass that cannot show its
        # range and its file count is indistinguishable from a pass that
        # compared nothing, which is the bug this tool had.
        print(
            f"{GREEN}Test-relaxation audit: clean (no tests deleted or weakened).{RESET}\n"
            f"  base {base}, range {anchor[:12]}..worktree, "
            f"{audit.examined} changed test file(s) examined."
        )
        return 0
    print(
        f"{BOLD}Test-relaxation audit{RESET} (base {base}, range {anchor[:12]}..worktree)\n"
    )
    color = {"DELETED": RED, "WEAKENED": RED, "RELAXED": YELLOW}
    for kind, path, details in findings:
        print(f"  {color.get(kind, '')}[{kind}]{RESET} {path}")
        for d in details:
            if not d:
                continue
            # A reason is now the whole justification, not its first line, so it can
            # run to a paragraph. Wrapped, because an unwrapped paragraph in a
            # terminal is a reason nobody reads, which is how this corpus grew.
            wrapped = textwrap.wrap(d, width=92) or [d]
            print(f"      - {wrapped[0]}")
            for more in wrapped[1:]:
                print(f"        {more}")
    deleted = sum(1 for k, _, _ in findings if k == "DELETED")
    weakened = sum(1 for k, _, _ in findings if k == "WEAKENED")
    relaxed = sum(1 for k, _, _ in findings if k == "RELAXED")
    print(
        f"\n{len(findings)} finding(s): {deleted} deleted, {weakened} weakened "
        f"(undocumented), {relaxed} relaxed (documented; verify the reason)."
    )
    print(
        "  Each is a candidate finding for the review: confirm the CODE was fixed,\n"
        "  not the test. A documented relaxation is only valid for removed features\n"
        "  or replaced coverage."
    )
    return 1


def selftest():
    work = tempfile.mkdtemp(prefix="ze-relax-audit-")
    # commit.gpgsign=false keeps this throwaway repo hermetic: without it a
    # developer whose global config sets commit.gpgsign=true has the baseline
    # commit fail to sign (no tty for the passphrase), HEAD never resolves, and
    # the selftest reports "HEAD does not resolve to a commit". The sibling test
    # fixture (audit_relaxation_test.py) already disables signing the same way.
    env_name = [
        "-c",
        "user.email=t@t",
        "-c",
        "user.name=t",
        "-c",
        "commit.gpgsign=false",
    ]
    git(["init", "-q"], work)
    repo_root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    detector = load_detector(repo_root)
    if detector is None:
        print("SELFTEST FAIL: detector not importable")
        return 2

    def commit(msg):
        git(["add", "-A"], work)
        git([*env_name, "commit", "-q", "-m", msg], work)

    def write(path, content):
        full = os.path.join(work, path)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w") as fh:
            fh.write(content)

    # baseline commit with three healthy tests
    write(
        "pkg/a_test.go",
        "package a\nfunc TestA(t *testing.T){ require.Equal(t,1,f()); require.NoError(t,err) }\n",
    )
    write(
        "pkg/b_test.go", "package a\nfunc TestB(t *testing.T){ require.True(t, g()) }\n"
    )
    write(
        "pkg/c_test.go",
        "package a\nfunc TestC(t *testing.T){ require.Equal(t,2,h()) }\n",
    )
    write(
        "pkg/keep_test.go",
        "package a\nfunc TestKeep(t *testing.T){ require.Equal(t,3,k()) }\n",
    )
    commit("baseline")

    # weaken A (skip + drop assertion), relax B (documented), delete C, leave keep untouched
    write(
        "pkg/a_test.go",
        'package a\nfunc TestA(t *testing.T){ t.Skip("x"); require.Equal(t,1,f()) }\n',
    )
    write(
        "pkg/b_test.go",
        "package a\n// test-relax: feature removed in spec-x\nfunc TestB(t *testing.T){ t.Skip() }\n",
    )
    os.remove(os.path.join(work, "pkg/c_test.go"))

    audit = run_audit("HEAD", work, detector, load_rfc_detector(repo_root))
    if audit.err:
        print(f"SELFTEST FAIL: audit could not run: {audit.err}")
        return 2
    got = {path: kind for kind, path, _ in audit.findings}
    expect = {
        "pkg/a_test.go": "WEAKENED",
        "pkg/b_test.go": "RELAXED",
        "pkg/c_test.go": "DELETED",
    }
    ok = got == expect
    print(("SELFTEST PASS" if ok else "SELFTEST FAIL"))
    if not ok:
        print(f"  expected={expect}")
        print(f"  got     ={got}")
    return 0 if ok else 1


def main(argv):
    if "--selftest" in argv:
        return selftest()
    base = next((a for a in argv[1:] if not a.startswith("-")), "HEAD")
    cwd = (
        git(["rev-parse", "--show-toplevel"], os.getcwd()).stdout.strip() or os.getcwd()
    )
    detector = load_detector(cwd)
    if detector is None:
        print(
            f"{RED}Audit could not load detection logic from .claude/hooks/pretool-writeedit.py{RESET}",
            file=sys.stderr,
        )
        return 2
    audit = run_audit(base, cwd, detector, load_rfc_detector(cwd))
    if audit.err:
        # Exit 2 = "audit could not run", which is exactly what a base with no
        # honest comparison is. Refusing beats guessing at the caller's intent.
        print(
            f"{RED}{BOLD}Test-relaxation audit: CANNOT RUN.{RESET}\n  {audit.err}",
            file=sys.stderr,
        )
        return 2
    return report(audit, base)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
