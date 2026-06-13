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
    python3 scripts/dev/audit-test-relaxation.py            # uncommitted vs HEAD
    python3 scripts/dev/audit-test-relaxation.py main       # whole branch vs main
    python3 scripts/dev/audit-test-relaxation.py --selftest # internal tests

Exit 0 = no test deletion/weakening/relaxation found.
Exit 1 = findings reported (review them).
Exit 2 = audit could not run (e.g. detection logic not importable).
"""

import importlib.util
import os
import re
import subprocess
import sys
import tempfile

RED = "\033[31m"
YELLOW = "\033[33m"
GREEN = "\033[32m"
BOLD = "\033[1m"
RESET = "\033[0m"

_RELAX_LINE = re.compile(r"//[ \t]*test-relax:[ \t]*(\S.*)?$", re.MULTILINE)


def git(args, cwd):
    return subprocess.run(["git", *args], cwd=cwd, capture_output=True, text=True)


def load_detector(repo_root):
    """Import _test_weakening_errs from the canonical hook so logic stays shared."""
    hook = os.path.join(repo_root, ".claude", "hooks", "pretool-writeedit.py")
    if not os.path.isfile(hook):
        return None
    spec = importlib.util.spec_from_file_location("ze_pretool_we", hook)
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)
    except Exception:
        return None
    return getattr(mod, "_test_weakening_errs", None)


def is_test_path(p):
    if p.endswith("_test.go"):
        return True
    if (p.endswith(".ci") or p.endswith(".et")) and (
        p.startswith("test/") or "/test/" in p
    ):
        return True
    return False


def changed_test_files(base, cwd):
    """Return list of (status, old_path, new_path) for changed test files in base..worktree."""
    mb = git(["merge-base", base, "HEAD"], cwd)
    anchor = mb.stdout.strip() if mb.returncode == 0 and mb.stdout.strip() else base
    out = git(["diff", "--name-status", "-M", anchor, "--"], cwd)
    if out.returncode != 0:
        return anchor, []
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
    return anchor, rows


def read_worktree(path, cwd):
    full = os.path.join(cwd, path)
    try:
        with open(full, encoding="utf-8", errors="replace") as fh:
            return fh.read()
    except OSError:
        return None


def run_audit(base, cwd, detector):
    anchor, rows = changed_test_files(base, cwd)
    findings = []  # (kind, path, details:list[str])
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
        old_tokens = len(_RELAX_LINE.findall(old))
        new_tokens = _RELAX_LINE.findall(new)
        added_tokens = [t for t in new_tokens[old_tokens:]]
        label = "RENAMED " if status.startswith("R") else ""
        if added_tokens:
            extra = [f"reason: {t.strip() or '(empty!)'}" for t in added_tokens]
            findings.append(
                ("RELAXED", new_p, [label.strip()] * bool(label) + details + extra)
            )
        elif details:
            findings.append(
                ("WEAKENED", new_p, [label.strip()] * bool(label) + details)
            )
    return anchor, findings


def report(anchor, findings, base):
    if not findings:
        print(
            f"{GREEN}Test-relaxation audit: clean (no tests deleted or weakened).{RESET}"
        )
        return 0
    print(
        f"{BOLD}Test-relaxation audit{RESET} (base {base}, range {anchor[:12]}..worktree)\n"
    )
    color = {"DELETED": RED, "WEAKENED": RED, "RELAXED": YELLOW}
    for kind, path, details in findings:
        print(f"  {color.get(kind, '')}[{kind}]{RESET} {path}")
        for d in details:
            if d:
                print(f"      - {d}")
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
    env_name = ["-c", "user.email=t@t", "-c", "user.name=t"]
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

    _, findings = run_audit("HEAD", work, detector)
    got = {path: kind for kind, path, _ in findings}
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
    anchor, findings = run_audit(base, cwd, detector)
    return report(anchor, findings, base)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
