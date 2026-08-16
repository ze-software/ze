#!/usr/bin/env python3
"""Audit a diff for deleted or weakened tests, and report them.

Backs the test-relaxation pass of /ze-review and /ze-review-deep. Where the
PreToolUse hook (c_test_weakening) guards a single edit as it happens, this runs
over a whole branch or working-tree diff. It reads the accepted weakening rows
from each commit in the resolved range and reports only unexplained weakenings.

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

Exit 0 = a comparison ran and found no unexplained test deletion or weakening.
Exit 1 = unexplained findings reported (review them).
Exit 2 = audit could not run: detection logic not importable, or the base is
         unusable (nonexistent, unrelated, or an empty range auditing nothing).
"""

import importlib.util
import os
import subprocess
import sys
import tempfile
import textwrap
from typing import NamedTuple

RED = "\033[31m"
GREEN = "\033[32m"
BOLD = "\033[1m"
RESET = "\033[0m"


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

_weakened_module = None


def load_weakened_module():
    """Import the shared weakened-row parser and matcher by path."""
    global _weakened_module
    if _weakened_module is None:
        path = os.path.join(os.path.dirname(__file__), "check_weakened_tests.py")
        spec = importlib.util.spec_from_file_location("ze_check_weakened_tests", path)
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        _weakened_module = module
    return _weakened_module


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


def changed_test_files(old_revision, cwd, new_revision=None):
    """Return changed test files between two revisions or a revision and worktree.

    The rows are (status, old_path, new_path). A failed diff returns err, not
    []: an empty row list must only ever mean the comparison genuinely contains
    no test-file changes.
    """
    args = ["diff", "--name-status", "-M", old_revision]
    if new_revision is not None:
        args.append(new_revision)
    args.append("--")
    out = git(args, cwd)
    target = new_revision or "worktree"
    if out.returncode != 0:
        return None, (
            f"git diff {old_revision[:12]}..{target[:12]} failed, so nothing "
            f"was compared:\n  {out.stderr.strip()}"
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


def accepted_rows(commit, cwd, weakened):
    """Return accepted rows from one commit's test/weakened.md, or an error."""
    source = f"{commit}:test/weakened.md"
    tree = git(["ls-tree", "--name-only", commit, "--", "test/weakened.md"], cwd)
    if tree.returncode != 0:
        return None, (
            f"git ls-tree {commit[:12]} failed, so accepted rows cannot be read:\n"
            f"  {tree.stderr.strip()}"
        )
    if not tree.stdout.strip():
        return [], None
    version = git(["show", source], cwd)
    if version.returncode != 0:
        return None, (
            f"git show {source} failed, so accepted rows cannot be read:\n"
            f"  {version.stderr.strip()}"
        )
    rows, problems = weakened.parse_weakened_file(version.stdout)
    if problems:
        return None, (
            f"cannot read accepted rows from commit {commit[:12]}:\n  "
            + "\n  ".join(problems)
        )
    return rows, None


def read_worktree(path, cwd):
    """Return a worktree file's text and a named error when it cannot be read."""
    full = os.path.join(cwd, path)
    try:
        with open(full, encoding="utf-8", errors="replace") as fh:
            return fh.read(), None
    except OSError as exc:
        return None, f"cannot read worktree file {path}: {exc}"


def read_revision(revision, path, cwd):
    """Return one committed file version, failing closed on every git error."""
    source = f"{revision}:{path}"
    out = git(["show", source], cwd)
    if out.returncode != 0:
        return None, (
            f"git show {source} failed, so the test change cannot be examined:\n"
            f"  {out.stderr.strip()}"
        )
    return out.stdout, None


class Audit(NamedTuple):
    anchor: str  # commit the worktree was diffed against
    findings: list  # (kind, path, details:list[str])
    examined: int  # test files actually inspected; the verdict's evidence
    err: str  # why no comparison was possible, or "" when one ran


def audit_changes(
    old_revision,
    new_revision,
    cwd,
    detector,
    rfc_detector,
    weakened,
    accepted,
):
    """Audit one committed diff, or HEAD against the worktree."""
    rows, err = changed_test_files(old_revision, cwd, new_revision)
    if err:
        return [], 0, err

    findings = []
    # Added files are counted but never inspected: a brand-new test cannot be a
    # weakening of anything. The verdict reports what it actually looked at.
    examined = sum(1 for status, _, _ in rows if status != "A")
    for status, old_p, new_p in rows:
        if status == "A":
            continue

        path = old_p if status == "D" else new_p
        old, err = read_revision(old_revision, old_p, cwd)
        if err:
            return [], 0, err
        if status == "D":
            new = ""
        elif new_revision is None:
            new, err = read_worktree(new_p, cwd)
            if err:
                return [], 0, err
        else:
            new, err = read_revision(new_revision, new_p, cwd)
            if err:
                return [], 0, err

        # Keep both detector halves. The hook blocks the first and advises on
        # the second. A branch reviewer needs both to judge the complete range.
        blocking, advisory = detector(old, new, path) if detector else ([], [])
        file_details = list(blocking) + list(advisory)
        units = weakened.weakened_units(path, old, new, detector)
        if not units and file_details:
            stem = os.path.splitext(os.path.basename(path))[0]
            units = [(stem, file_details)]

        unmatched = []
        package = os.path.basename(os.path.dirname(path))
        for name, details in units:
            weak = weakened.Weakened(path, package, name, details)
            if any(weakened.row_matches(row.name, weak) for row in accepted):
                continue
            unmatched.append((name, details))

        # An RFC-tagged test is the proof behind a public compliance claim.
        # Its separate approval mechanism remains mandatory even when a weakened
        # row accepts a structural change.
        rfc_tags = rfc_detector(old, new, path) if rfc_detector else None
        rfc_details = []
        if rfc_tags:
            rfc_details = [
                "RFC-TAGGED test changed without an approval token: "
                + ", ".join(rfc_tags),
                "  the user must approve this; see rfc-test-change-approved:",
            ]

        kind = "DELETED" if status == "D" else "WEAKENED"
        label = ["renamed file"] if status.startswith("R") else []
        for index, (name, details) in enumerate(unmatched):
            extra = rfc_details if index == 0 else []
            findings.append(
                (kind, path, label + [f"test: {name}"] + list(details) + extra)
            )
        if not unmatched and rfc_details:
            findings.append(("WEAKENED", path, label + rfc_details))

    return findings, examined, ""


def run_audit(base, cwd, detector, rfc_detector=None):
    anchor, err = resolve_anchor(base, cwd)
    if err:
        return Audit("", [], 0, err)

    commits = git(["rev-list", "--reverse", f"{anchor}..HEAD"], cwd)
    if commits.returncode != 0:
        return Audit(
            anchor,
            [],
            0,
            f"git rev-list for {anchor[:12]}..HEAD failed, so the branch "
            f"history cannot be examined:\n  {commits.stderr.strip()}",
        )

    weakened = load_weakened_module()
    findings = []
    examined = 0
    for commit in commits.stdout.splitlines():
        accepted, err = accepted_rows(commit, cwd, weakened)
        if err:
            return Audit(anchor, [], 0, err)
        commit_findings, commit_examined, err = audit_changes(
            f"{commit}^",
            commit,
            cwd,
            detector,
            rfc_detector,
            weakened,
            accepted,
        )
        if err:
            return Audit(anchor, [], 0, err)
        findings.extend(commit_findings)
        examined += commit_examined

    # Rows committed at HEAD explain only HEAD's own diff. They cannot accept a
    # later uncommitted weakening of the same named unit.
    worktree_findings, worktree_examined, err = audit_changes(
        "HEAD", None, cwd, detector, rfc_detector, weakened, []
    )
    if err:
        return Audit(anchor, [], 0, err)
    findings.extend(worktree_findings)
    examined += worktree_examined

    return Audit(anchor, findings, examined, "")


def report(audit, base):
    anchor, findings = audit.anchor, audit.findings
    if not findings:
        # The verdict states what it is based on. A pass that cannot show its
        # range and its file count is indistinguishable from a pass that
        # compared nothing, which is the bug this tool had.
        print(
            f"{GREEN}Test-relaxation audit: clean (no unexplained test weakening).{RESET}\n"
            f"  base {base}, range {anchor[:12]}..worktree, "
            f"{audit.examined} changed test file(s) examined."
        )
        return 0
    print(
        f"{BOLD}Test-relaxation audit{RESET} (base {base}, range {anchor[:12]}..worktree)\n"
    )
    color = {"DELETED": RED, "WEAKENED": RED}
    for kind, path, details in findings:
        print(f"  {color.get(kind, '')}[{kind}]{RESET} {path}")
        for d in details:
            if not d:
                continue
            # Detector details can run to a paragraph. Wrap them so the
            # reviewer can read the complete reason without terminal overflow.
            wrapped = textwrap.wrap(d, width=92) or [d]
            print(f"      - {wrapped[0]}")
            for more in wrapped[1:]:
                print(f"        {more}")
    deleted = sum(1 for k, _, _ in findings if k == "DELETED")
    weakened = sum(1 for k, _, _ in findings if k == "WEAKENED")
    print(
        f"\n{len(findings)} unexplained finding(s): {deleted} deleted, "
        f"{weakened} weakened."
    )
    print(
        "  Each is a candidate finding for the review: confirm that the code was\n"
        "  fixed instead of the test. An accepted weakening needs a matching row\n"
        "  in test/weakened.md history."
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
    base_sha = git(["rev-parse", "HEAD"], work).stdout.strip()

    # Weaken A, accept B with a committed row, delete C, and leave keep untouched.
    write(
        "pkg/a_test.go",
        'package a\nfunc TestA(t *testing.T){ t.Skip("x"); require.Equal(t,1,f()) }\n',
    )
    write(
        "pkg/b_test.go", "package a\nfunc TestB(t *testing.T){ t.Skip() }\n"
    )
    write(
        "test/weakened.md",
        "| Test | Reason |\n"
        "|------|--------|\n"
        "| TestB | feature removed in spec-x |\n",
    )
    os.remove(os.path.join(work, "pkg/c_test.go"))
    commit("weaken tests")

    audit = run_audit(base_sha, work, detector, load_rfc_detector(repo_root))
    if audit.err:
        print(f"SELFTEST FAIL: audit could not run: {audit.err}")
        return 2
    got = {path: kind for kind, path, _ in audit.findings}
    expect = {
        "pkg/a_test.go": "WEAKENED",
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
