#!/usr/bin/env -S python3 -S
"""Single consolidated PostToolUse check for Write/Edit.

Replaces 9 of the 10 PostToolUse Write/Edit shell hooks. Parses the payload once
and runs every check in-process (reading the post-edit file from disk, as the
originals did), including the two file-mutating formatters:

    auto-lint        gofmt/goimports -w, then one golangci-lint --new-from-rev pass
    auto-py-format   ruff format -w + ruff check (advisory)
    + 7 advisory checks (file-size, deferral, rfc, test-docs, fuzz, vague, boundary)

NOT folded: validate-spec.sh stays a standalone hook. It has a latent set -e
crash (greps the wiring table for Unicode `→` while real specs use ASCII `->`,
so an unguarded `grep -v` pipeline aborts the script at exit 1). Folding it would
mean either replicating that crash or silently turning a non-blocking gate into a
blocking one. Left as-is; see ai/rules/hook-mapping.md.

Exit codes: 0 allow/advisory, 1 warning, 2 block. Most severe wins. Fails OPEN
(exit 0) on an unexpected internal error.
"""

import os
import re
import shutil
import subprocess
import sys

YELLOW = "\033[33m"
RED = "\033[31m"
DIM = "\033[2m"
BOLD = "\033[1m"
RESET = "\033[0m"


def read_file(path):
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            return fh.read()
    except Exception:
        return None


def _go(ctx, skip_test=True):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit") or not fp.endswith(".go"):
        return False
    if skip_test and fp.endswith("_test.go"):
        return False
    return os.path.isfile(fp)


def grep_count(text, pattern, flags=0):
    return len(re.findall(pattern, text, flags))


# --------------------------------------------------------------------------- #
# Mutating / heavy checks (run first, matching the original hook order)
# --------------------------------------------------------------------------- #


def c_auto_lint(ctx):
    """auto_linter.sh: gofmt/goimports -w, then one golangci-lint --new-from-rev pass."""
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit") or not fp.endswith(".go"):
        return None
    if not os.path.isfile(fp) or "/scripts/" in fp:
        return None
    root = os.path.dirname(fp)
    while root != "/" and not os.path.isfile(os.path.join(root, "go.mod")):
        root = os.path.dirname(root)
    if not os.path.isfile(os.path.join(root, "go.mod")):
        return None
    if shutil.which("gofmt"):
        try:
            subprocess.run(["gofmt", "-w", fp], capture_output=True, timeout=30)
        except Exception:
            pass
    if shutil.which("goimports"):
        try:
            subprocess.run(
                [
                    "goimports",
                    "-local",
                    "codeberg.org/thomas-mangin/ze",
                    "-format-only",
                    "-w",
                    fp,
                ],
                capture_output=True,
                timeout=30,
            )
        except Exception:
            pass
    if shutil.which("golangci-lint"):
        rel = fp[len(root) + 1 :] if fp.startswith(root + "/") else fp
        pkg_dir = os.path.dirname(rel)
        try:
            res = subprocess.run(
                [
                    "golangci-lint",
                    "run",
                    "--new-from-rev=HEAD",
                    "--timeout=30s",
                    f"./{pkg_dir}/...",
                ],
                cwd=root,
                capture_output=True,
                text=True,
                timeout=60,
            )
            output = res.stdout + res.stderr
        except Exception:
            output = ""
        if output and "no issues" not in output and not output.startswith("0 issues"):
            issue_count = sum(1 for l in output.split("\n") if ":" in l)
            if issue_count > 0:
                head = [l for l in output.split("\n") if l][:3]
                return (
                    2,
                    f"{YELLOW}⚠ lint: {issue_count} issues{RESET}\n"
                    + "\n".join(f"  {DIM}{l}{RESET}" for l in head),
                )
    return None


def c_auto_py_format(ctx):
    """auto_py_format.sh: ruff format -w + advisory ruff check. Always exit 0."""
    fp = ctx["fp"]
    if (
        ctx["tool"] not in ("Write", "Edit")
        or not fp.endswith(".py")
        or not os.path.isfile(fp)
    ):
        return None
    if any(s in fp for s in ("/vendor/", "/third_party/", "/tmp/")):
        return None
    if not shutil.which("ruff"):
        return (
            0,
            "⚠ ruff not found; run 'make ze-setup' to install the Python formatter",
        )
    try:
        subprocess.run(
            ["ruff", "format", "--quiet", fp], capture_output=True, timeout=30
        )
    except Exception:
        pass
    try:
        res = subprocess.run(
            ["ruff", "check", "--quiet", fp], capture_output=True, text=True, timeout=30
        )
        out = res.stdout + res.stderr
    except Exception:
        out = ""
    if out:
        n = sum(1 for l in out.split("\n") if re.match(r"^[^:]+:[0-9]+:", l))
        if n > 0:
            head = [l for l in out.split("\n") if l][:3]
            return (
                0,
                f"{YELLOW}⚠ ruff: {n} issues{RESET}\n"
                + "\n".join(f"  {DIM}{l}{RESET}" for l in head),
            )
    return None


# --------------------------------------------------------------------------- #
# Cheap advisory checks
# --------------------------------------------------------------------------- #


def c_file_size(ctx):
    if not _go(ctx):
        return None
    txt = read_file(ctx["fp"])
    if txt is None:
        return None
    n = txt.count("\n")
    base = os.path.basename(ctx["fp"])
    if n > 1000:
        return (1, f"{RED}{BOLD}⚠️  File too large: {base} ({n} lines > 1000){RESET}")
    if n > 600:
        return (1, f"{YELLOW}⚠️  File growing: {base} ({n} lines > 600){RESET}")
    return None


_DEFERRAL = [
    "deferred to",
    "deferred for",
    "defer to",
    "out of scope",
    "future work",
    "future spec",
    "handle later",
    "address later",
    "skip for now",
    "skipping for now",
    "postpone",
    "not yet implemented",
    "not yet wired",
]


def c_warn_deferral(ctx):
    fp = ctx["fp"]
    if ctx["tool"] not in ("Write", "Edit") or not fp.endswith(".md"):
        return None
    if re.search(r"plan/deferrals(\.md$|/)", fp):
        return None
    if re.search(r"\.claude/(memory|plan)/", fp) or "tmp/session/" in fp:
        return None
    if "plan/learned/" in fp:
        return None
    content = (
        ctx["ti"].get("content")
        if ctx["tool"] == "Write"
        else ctx["ti"].get("new_string")
    )
    content = content or ""
    if not content:
        return None
    for p in _DEFERRAL:
        if re.search(re.escape(p), content, re.IGNORECASE):
            return (
                1,
                f"{YELLOW}{BOLD}  Deferral language detected in {os.path.basename(fp)}{RESET}\n"
                f"  {YELLOW}Pattern: '{p}'{RESET}\n"
                f"  {YELLOW}Record in the source's plan/deferrals/<source>.md shard if this is deferred work.{RESET}",
            )
    return None


def c_require_rfc(ctx):
    fp = ctx["fp"]
    if not _go(ctx, skip_test=False):
        return None
    base = os.path.basename(fp)
    if (
        re.search(r"_test\.go$", base)
        or re.search(r"_gen\.go$", base)
        or base in ("register.go", "embed.go", "doc.go")
    ):
        return None
    txt = read_file(fp)
    if txt is None:
        return None
    if "// RFC:" in "\n".join(txt.split("\n")[:10]):
        return None
    if grep_count(txt, r"RFC [0-9]{4}|rfc[0-9]{4}") >= 2:
        return (
            0,
            f"{YELLOW}⚠ {base} references RFCs but has no // RFC: rfc/short/rfcNNNN.md header{RESET}",
        )
    return None


def c_require_test_docs(ctx):
    fp = ctx["fp"]
    if (
        ctx["tool"] not in ("Write", "Edit")
        or not fp.endswith("_test.go")
        or not os.path.isfile(fp)
    ):
        return None
    txt = read_file(fp)
    if txt is None:
        return None
    if (
        grep_count(txt, r"^func Test[A-Z]", re.MULTILINE) > 0
        and "VALIDATES:" not in txt
        and "PREVENTS:" not in txt
    ):
        return (
            0,
            f"{YELLOW}⚠️  Test file without documentation: {os.path.basename(fp)} "
            f"(add VALIDATES:/PREVENTS: comments){RESET}",
        )
    return None


def c_require_fuzz(ctx):
    fp = ctx["fp"]
    if not _go(ctx):
        return None
    if not any(s in fp for s in ("/message/", "/nlri/", "/attribute/", "/capability/")):
        return None
    txt = read_file(fp)
    if txt is None:
        return None
    if (
        grep_count(txt, r"^func Parse[A-Z]", re.MULTILINE) == 0
        and grep_count(txt, r"^func \([^)]+\) Parse", re.MULTILINE) == 0
    ):
        return None
    has_fuzz = False
    t = read_file(fp[:-3] + "_test.go")
    if t and grep_count(t, r"^func Fuzz[A-Z]", re.MULTILINE) > 0:
        has_fuzz = True
    if not has_fuzz:
        try:
            for f in os.listdir(os.path.dirname(fp)):
                if f.endswith("_test.go"):
                    d = read_file(os.path.join(os.path.dirname(fp), f))
                    if d and grep_count(d, r"^func Fuzz[A-Z]", re.MULTILINE) > 0:
                        has_fuzz = True
                        break
        except Exception:
            pass
    if not has_fuzz:
        return (
            0,
            f"{YELLOW}⚠️  Wire format parsing without fuzz tests: {os.path.basename(fp)}{RESET}",
        )
    return None


def c_vague_names(ctx):
    if not _go(ctx):
        return None
    txt = read_file(ctx["fp"])
    if txt is None:
        return None
    pat = r"(^|[^A-Za-z0-9_])(Data|Info|Result|Item|Thing|Temp|Tmp|Val|Obj)[ \t]+[A-Za-z0-9_]+[ \t]*="
    if [l for l in txt.split("\n") if re.search(pat, l)][:3]:
        return (
            0,
            f"{YELLOW}⚠️  Vague variable names detected in {os.path.basename(ctx['fp'])}{RESET}",
        )
    return None


def c_boundary_tests(ctx):
    fp = ctx["fp"]
    if not _go(ctx):
        return None
    txt = read_file(fp)
    if txt is None:
        return None
    patterns = [
        r"if .* > [0-9]",
        r"if .* < [0-9]",
        r"if .* >= [0-9]",
        r"if .* <= [0-9]",
        r"if .* > 0x",
        r"if .* < 0x",
        r"return .*Invalid.*Range",
        r"return .*OutOfBounds",
        r"return .*Exceeds",
    ]
    if not any(re.search(p, txt) for p in patterns):
        return None
    test_file = re.sub(r"\.go.*$", "", fp) + "_test.go"
    t = read_file(test_file)
    if t is None:
        return (
            0,
            f"{YELLOW}⚠️  Numeric validation but no test file: {os.path.basename(test_file)}{RESET}",
        )
    if not re.search(
        r"boundary|invalid.*above|invalid.*below|max.*valid|min.*valid",
        t,
        re.IGNORECASE,
    ):
        return (
            0,
            f"{YELLOW}⚠️  Numeric validation but no boundary tests in {os.path.basename(test_file)}{RESET}",
        )
    return None


# Order mirrors the original PostToolUse hook array.
CHECKS = (
    c_auto_lint,
    c_auto_py_format,
    c_file_size,
    c_warn_deferral,
    c_require_rfc,
    c_require_test_docs,
    c_require_fuzz,
    c_vague_names,
    c_boundary_tests,
)


def main():
    try:
        import json

        payload = json.load(sys.stdin)
    except Exception:
        return 0
    if payload.get("tool_name") not in ("Write", "Edit"):
        return 0
    ti = payload.get("tool_input") or {}
    ctx = {"tool": payload["tool_name"], "ti": ti, "fp": ti.get("file_path") or ""}
    worst = 0
    messages = []
    for check in CHECKS:
        try:
            r = check(ctx)
        except Exception:
            import traceback

            sys.stderr.write(
                f"[posttool-writeedit] {check.__name__} errored (failing open):\n"
            )
            traceback.print_exc()
            continue
        if r is None:
            continue
        code, msg = r
        messages.append(msg)
        worst = max(worst, code)
    if messages:
        sys.stderr.write("\n".join(messages) + "\n")
    return worst


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception:
        import traceback

        traceback.print_exc()
        sys.exit(0)
