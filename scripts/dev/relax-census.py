#!/usr/bin/env python3
"""Count every `test-relax:` token in the test suite, and hold the count down.

A `test-relax:` token buys an edit a pass from `c_test_weakening`
(.claude/hooks/pretool-writeedit.py). It is self-service by design: the agent
writes its own justification. The only thing that ever made that safe was the
expectation that a human would read them.

Nothing counted them until 2026-08-10. By then the working tree held 755 across
468 test files (751 across 466 at HEAD). At that volume nobody can read them, so
nobody does, so writing one costs nothing, so more get written. This restores the
cost. The ceiling can be raised, and raising it is a line in a commit that a
reviewer sees.

WHAT IT COUNTS, AND WHY IT IS HEAD. The census reads the content git HOLDS, not
the working tree. Several sessions share this checkout, so a working-tree count
moves under whoever reads it: measured over one hour on 2026-08-10 it went
751 -> 752 -> 755 on edits by three sessions that had never touched this gate.
A gate that reds on another session's half-finished work is a gate that gets
switched off (ai/rules/repo-maintenance.md, "the two changed-file checks").
HEAD is stable, and it blames the commit that actually raised the count.

A token you are ABOUT to add is therefore not caught here. It is caught where a
changed-file check belongs: `scripts/dev/audit-test-relaxation.py`, run over your
diff by /ze-review step 0. `--worktree` shows what you are about to land.

Usage:
    python3 scripts/dev/relax-census.py             # count HEAD, check the ceiling
    python3 scripts/dev/relax-census.py --worktree  # count the working tree instead
    python3 scripts/dev/relax-census.py --list      # every token with its reason
    python3 scripts/dev/relax-census.py --by-area   # counts per top-level area
    python3 scripts/dev/relax-census.py --lower     # rewrite the ceiling DOWN to
                                                    # the current HEAD count
    python3 scripts/dev/relax-census.py --selftest  # internal tests

Exit 0 = at or under the ceiling. Exit 1 = over it, or the ceiling was raised
without a reason. Exit 2 = the census could not run or could not read the whole
corpus. It never reports a pass on a partial read: "clean" must mean "I counted
everything and it is under", never "I counted what I could open"
(ai/rules/evidence.md).

`--lower` never raises. Raising the ceiling is an edit to test/relax-ceiling.txt
plus a `raised-for:` line saying which test lost what, in the same commit.
"""

import argparse
import collections
import importlib.util
import os
import shutil
import subprocess
import sys
import tempfile

RED = "\033[31m"
YELLOW = "\033[33m"
GREEN = "\033[32m"
BOLD = "\033[1m"
RESET = "\033[0m"

CEILING_PATH = "test/relax-ceiling.txt"
RAISED_FOR = "raised-for:"


def repo_root():
    out = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True
    )
    return out.stdout.strip() if out.returncode == 0 else ""


def load_audit(root):
    """Import the audit's token reader so the census counts the SAME thing it does.

    Re-spelling the token pattern here is how two counts of one fact start
    disagreeing, and the disagreement always surfaces as an argument about which
    number is real.
    """
    path = os.path.join(root, "scripts", "dev", "audit-test-relaxation.py")
    if not os.path.isfile(path):
        return None
    spec = importlib.util.spec_from_file_location("ze_relax_audit", path)
    mod = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(mod)
    except Exception:
        return None
    return mod


def git(root, *args):
    return subprocess.run(["git", *args], cwd=root, capture_output=True, text=True)


def tracked_test_files(root, is_test_path, worktree=False):
    """(paths, err) for every tracked test file, vendor excluded.

    The population MUST come from the same place the content does. Listing the
    INDEX (`git ls-files`) and then reading HEAD made every staged-but-uncommitted
    NEW test file "unreadable", so the gate exited 2 for a session that had merely
    run `git add` -- and in a checkout this many sessions share, that is somebody
    else's `git add`, on a run whose author touched nothing.

    `-z` because `git ls-files` quotes and escapes a path with a non-ASCII or
    unusual byte unless asked not to. A quoted path fails `is_test_path`, so it
    was dropped from the population entirely: the census failed OPEN on it, which
    is the one outcome the module contract forbids.
    """
    if worktree:
        out = git(root, "ls-files", "-z", "--", "*_test.go", "*.ci", "*.et")
    else:
        out = git(root, "ls-tree", "-r", "-z", "--name-only", "HEAD")
    if out.returncode != 0:
        return None, f"git listing failed: {out.stderr.strip()}"
    paths = [
        p
        for p in out.stdout.split("\0")
        if p and not p.startswith("vendor/") and is_test_path(p)
    ]
    return paths, ""


def census(root, audit, worktree=False):
    """(rows, err). A row is (path, reason), path-sorted.

    An unreadable file is an ERROR, never a skip. Skipping it silently was the
    zero-value trap this repo has been bitten by before: `chmod 000` on one
    tracked test, or a dangling symlink, or a sparse checkout, and the gate
    reported a clean pass over a corpus it had not read.
    """
    paths, err = tracked_test_files(root, audit.is_test_path, worktree=worktree)
    if err:
        return None, err
    rows, unreadable = [], []
    for p in sorted(paths):
        if worktree:
            try:
                with open(
                    os.path.join(root, p), encoding="utf-8", errors="replace"
                ) as fh:
                    text = fh.read()
            except OSError as exc:
                unreadable.append(f"{p}: {exc.strerror}")
                continue
        else:
            out = git(root, "show", f"HEAD:{p}")
            if out.returncode != 0:
                unreadable.append(f"{p}: not readable at HEAD")
                continue
            text = out.stdout
        if "test-relax:" not in text:  # cheap reject before the line walk
            continue
        for reason in audit.relax_reasons(text, p):
            rows.append((p, reason))
    if unreadable:
        listed = "\n    ".join(unreadable[:10])
        more = (
            f"\n    ... and {len(unreadable) - 10} more" if len(unreadable) > 10 else ""
        )
        return None, (
            f"{len(unreadable)} tracked test file(s) could not be read, so the "
            f"count is incomplete\n  and no verdict can be given:\n    {listed}{more}"
        )
    return rows, ""


def _ceiling_from(body, where):
    """(limit, err) for one ceiling file's text. First bare integer line wins."""
    for line in body.split("\n"):
        line = line.split("#", 1)[0].strip()
        if line.isdigit():
            return int(line), ""
    return None, f"{where} carries no bare integer."


def read_ceiling(root):
    """(limit, err). The file carries one bare integer; every other line is prose."""
    try:
        with open(os.path.join(root, CEILING_PATH), encoding="utf-8") as fh:
            body = fh.read()
    except OSError:
        return None, (
            f"{CEILING_PATH} is missing, so there is no ceiling to hold the count "
            f"against.\n  Nothing was checked; restore the file rather than "
            f"deleting the gate."
        )
    return _ceiling_from(body, CEILING_PATH)


def _raised_for_lines(body, reason_key):
    """The SET of `raised-for:` justifications in one ceiling file, by reason key.

    A set, not a multiset: counted, writing an existing reason a SECOND time was a
    difference, so duplicating the line justified any raise. A copy-paste is not a
    reason.

    Keyed by the HOOK's `_reason_key`, not by a whitespace normalizer of our own.
    Whitespace-only normalization left recasing one letter, adding a full stop, or
    inserting a zero-width space buying any raise -- the same class of cosmetic edit
    the hook already refuses for a `test-relax:` justification. Two halves of one
    gate disagreeing about what makes two sentences the same is how one of them
    becomes the way through.
    """
    return {
        reason_key(line.split(RAISED_FOR, 1)[1])
        for line in body.split("\n")
        if RAISED_FOR in line and reason_key(line.split(RAISED_FOR, 1)[1])
    }


def ceiling_raise(root, reason_key):
    """(delta, justified, err) for a ceiling raised since HEAD.

    Nothing else watches the ceiling itself. The design rests on "a reviewer sees
    the line", which is precisely the property the repo's other ratchets refuse
    to rely on (`check_extraction_ratchet` demands a `resign-reason`). So a raise
    must carry a `raised-for:` line, in the same edit.

    The justification must be NEW, not merely present. Asking only whether the
    file contains the marker meant the FIRST legitimate raise justified every
    later one for good: the ratchet would have destroyed itself the first time
    anybody used it correctly, and the file's own instructions tell them to.
    """
    out = git(root, "show", f"HEAD:{CEILING_PATH}")
    if out.returncode != 0:
        return 0, True, ""  # new file: nothing to compare against yet
    head_limit, err = _ceiling_from(out.stdout, f"HEAD:{CEILING_PATH}")
    if err:
        return 0, True, ""
    now_limit, err = read_ceiling(root)
    if err:
        return 0, True, err
    if now_limit <= head_limit:
        return now_limit - head_limit, True, ""
    try:
        with open(os.path.join(root, CEILING_PATH), encoding="utf-8") as fh:
            now_body = fh.read()
    except OSError:
        return now_limit - head_limit, False, ""
    fresh = _raised_for_lines(now_body, reason_key) - _raised_for_lines(
        out.stdout, reason_key
    )
    return now_limit - head_limit, bool(fresh), ""


def write_ceiling(root, new_limit):
    """Rewrite the recorded ceiling. Callers own the never-raise rule (see main)."""
    full = os.path.join(root, CEILING_PATH)
    with open(full, encoding="utf-8") as fh:
        lines = fh.read().split("\n")
    for i, line in enumerate(lines):
        if line.split("#", 1)[0].strip().isdigit():
            lines[i] = str(new_limit)
            break
    with open(full, "w", encoding="utf-8") as fh:
        fh.write("\n".join(lines))


def area_of(path):
    parts = path.split("/")
    return "/".join(parts[:3]) if len(parts) >= 3 else path


def main(argv):
    ap = argparse.ArgumentParser(add_help=True)
    ap.add_argument("--worktree", action="store_true", help="count the working tree")
    ap.add_argument("--list", action="store_true", help="print every token + reason")
    ap.add_argument("--by-area", action="store_true", help="counts per area")
    ap.add_argument("--lower", action="store_true", help="lower the ceiling to now")
    ap.add_argument("--selftest", action="store_true")
    args = ap.parse_args(argv[1:])

    if args.selftest:
        return selftest()

    root = repo_root()
    if not root:
        print(f"{RED}relax-census: not a git tree.{RESET}", file=sys.stderr)
        return 2
    audit = load_audit(root)
    if audit is None:
        print(
            f"{RED}relax-census: could not import "
            f"scripts/dev/audit-test-relaxation.py{RESET}",
            file=sys.stderr,
        )
        return 2

    # The hook owns what makes two justifications the same. Reached through the
    # audit's loader so this file never re-spells it.
    hook = audit.load_hook_module(root)
    reason_key = getattr(hook, "_reason_key", None) if hook else None
    if reason_key is None:
        print(
            f"{RED}relax-census: could not load _reason_key from the hook, so a\n"
            f"  ceiling raise could not be judged.{RESET}",
            file=sys.stderr,
        )
        return 2

    rows, err = census(root, audit, worktree=args.worktree)
    if err:
        print(f"{RED}relax-census: {err}{RESET}", file=sys.stderr)
        return 2
    limit, err = read_ceiling(root)
    if err:
        print(f"{RED}relax-census: {err}{RESET}", file=sys.stderr)
        return 2

    total = len(rows)
    files = len({p for p, _ in rows})
    basis = "worktree" if args.worktree else "HEAD"

    # A zero over a corpus known to hold hundreds is a broken reader, not an empty
    # backlog. Congratulating on it is the failure mode this whole gate exists to
    # answer for, one layer up (ai/rules/evidence.md).
    if total == 0 and limit > 0:
        print(
            f"{RED}{BOLD}relax-census: counted ZERO tokens against a ceiling of "
            f"{limit}.{RESET}\n"
            f"  A corpus that held {limit} cannot drop to nothing without a commit "
            f"that says so.\n"
            f"  Treating this as a pass would report a clean bill of health for a "
            f"reader that stopped\n"
            f"  reading. Lower the ceiling deliberately if the drain is real.",
            file=sys.stderr,
        )
        return 2

    if args.list:
        for p, reason in rows:
            print(f"{p}: {reason}")
    if args.by_area:
        for area, n in collections.Counter(area_of(p) for p, _ in rows).most_common():
            print(f"{n:6d}  {area}")

    delta, justified, err = ceiling_raise(root, reason_key)
    if err:
        print(f"{RED}relax-census: {err}{RESET}", file=sys.stderr)
        return 2

    if args.lower:
        # "Lower" is measured against HEAD, not against whatever the working copy
        # currently says. Measuring it against the local number let a hand-raised
        # ceiling be rewritten to a value still ABOVE HEAD's and reported as a
        # lowering -- an unjustified raise, laundered by the tool whose docstring
        # promises it never raises.
        head_limit = limit - delta
        if total >= head_limit and delta > 0:
            print(
                f"{RED}{BOLD}refusing: this would leave the ceiling ABOVE "
                f"HEAD's.{RESET}\n"
                f"  HEAD {head_limit}, local {limit}, count {total} at {basis}. "
                f"Lowering to {total} is a raise of\n"
                f"  {total - head_limit} against what is committed. Raise it "
                f"deliberately with a `{RAISED_FOR}` line instead.",
                file=sys.stderr,
            )
            return 1
        if total < limit:
            write_ceiling(root, total)
            print(f"{GREEN}ceiling lowered {limit} -> {total}{RESET}  ({CEILING_PATH})")
            return 0
        print(f"nothing to lower: {total} token(s) at {basis}, ceiling {limit}")
        return 0 if total <= limit else 1

    if delta > 0 and not justified:
        print(
            f"{RED}{BOLD}test-relax census: the ceiling was RAISED by {delta} with no "
            f"reason.{RESET}\n"
            f"  {CEILING_PATH} must carry a `{RAISED_FOR} <which test lost what>` line "
            f"in the same commit.\n"
            f"  A raise is the whole cost this gate imposes. Absorbing one silently "
            f"removes it.",
            file=sys.stderr,
        )
        return 1

    if total > limit:
        print(
            f"{RED}{BOLD}test-relax census: OVER THE CEILING{RESET}\n"
            f"  {total} token(s) in {files} file(s) at {basis}; the ceiling is "
            f"{limit}.\n"
            f"  Every token is a test that stopped proving something, justified by "
            f"the agent that\n"
            f"  weakened it. Fix the CODE and drop the token, or raise the ceiling in "
            f"{CEILING_PATH}\n"
            f"  with a `{RAISED_FOR}` line, in this same commit, so the increase is "
            f"reviewed.\n"
            f"  Read them: scripts/dev/relax-census.py --list",
            file=sys.stderr,
        )
        return 1

    color = GREEN if limit - total else YELLOW
    print(
        f"{color}test-relax census: {total} token(s) in {files} file(s) at {basis}, "
        f"ceiling {limit}.{RESET}"
    )
    # What you are about to land, when it differs from what git holds. Advisory:
    # it never decides the exit code, for the reason the module docstring gives.
    if not args.worktree:
        wt_rows, wt_err = census(root, audit, worktree=True)
        if not wt_err and len(wt_rows) != total:
            sign = "+" if len(wt_rows) > total else ""
            print(
                f"  working tree: {len(wt_rows)} ({sign}{len(wt_rows) - total} "
                f"uncommitted, this checkout is shared)"
            )
    return 0


def selftest():
    """Drive census(), read_ceiling() and ceiling_raise() over a throwaway tree."""
    work = tempfile.mkdtemp(prefix="ze-relax-census-")
    try:
        return _selftest_body(work)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def _selftest_body(work):
    root = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
    audit = load_audit(root)
    if audit is None:
        print("SELFTEST FAIL: audit not importable")
        return 2
    hook = audit.load_hook_module(root)
    _sk = getattr(hook, "_reason_key", None) if hook else None
    if _sk is None:
        print("SELFTEST FAIL: _reason_key not importable from the hook")
        return 2

    def write(rel, content):
        full = os.path.join(work, rel)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w", encoding="utf-8") as fh:
            fh.write(content)

    git(work, "init", "-q")
    write("pkg/a_test.go", "// test-relax: symbol deleted\n// with its only caller\n")
    write("test/plugin/b.ci", "# // test-relax: expectation moved upstream\n")
    write("test/plugin/clean.ci", "expect=out:text=one\n")
    write("vendor/v_test.go", "// test-relax: not ours to count\n")
    # A `.ci` outside test/ is not a test to `is_test_path`, so it must not count.
    write("docs/example.ci", "# // test-relax: documentation sample\n")
    write(CEILING_PATH, "# prose\n2\n")
    git(work, "add", "-A")
    git(
        work,
        "-c",
        "user.email=t@t",
        "-c",
        "user.name=t",
        "-c",
        "commit.gpgsign=false",
        "commit",
        "-q",
        "-m",
        "b",
    )

    rows, err = census(work, audit)
    if err:
        print(f"SELFTEST FAIL: census: {err}")
        return 2
    got = sorted(p for p, _ in rows)
    if got != ["pkg/a_test.go", "test/plugin/b.ci"]:
        print(f"SELFTEST FAIL: counted {got}")
        return 1
    # The reason is the WHOLE justification, not its first line.
    reason = next(r for p, r in rows if p.endswith("a_test.go"))
    if "only caller" not in reason:
        print(f"SELFTEST FAIL: reason truncated: {reason!r}")
        return 1
    limit, err = read_ceiling(work)
    if err or limit != 2:
        print(f"SELFTEST FAIL: ceiling {limit!r} {err}")
        return 1
    # An unreadable tracked test must refuse, never shrink the count silently.
    os.remove(os.path.join(work, "pkg/a_test.go"))
    os.symlink("nowhere", os.path.join(work, "pkg/a_test.go"))
    _, err = census(work, audit, worktree=True)
    if not err:
        print("SELFTEST FAIL: an unreadable tracked test did not refuse")
        return 1
    # A raise without a reason is caught; with one, it passes.
    write(CEILING_PATH, "# prose\n9\n")
    delta, justified, _ = ceiling_raise(work, _sk)
    if delta != 7 or justified:
        print(f"SELFTEST FAIL: unjustified raise delta={delta} justified={justified}")
        return 1
    write(CEILING_PATH, f"# prose\n# {RAISED_FOR} TestFoo lost its only assertion\n9\n")
    delta, justified, _ = ceiling_raise(work, _sk)
    if delta != 7 or not justified:
        print(f"SELFTEST FAIL: justified raise delta={delta} justified={justified}")
        return 1
    write(CEILING_PATH, "# prose\n9\n")
    write_ceiling(work, 1)
    if read_ceiling(work)[0] != 1:
        print("SELFTEST FAIL: ceiling not lowered")
        return 1
    print("SELFTEST PASS")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
