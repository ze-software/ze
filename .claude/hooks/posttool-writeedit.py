#!/usr/bin/env -S python3 -S
"""Single consolidated PostToolUse check for Write/Edit.

Replaces 7 cheap advisory shell hooks that each spawned bash + jq after every
edit. Parses the payload once and runs them in-process (reading the post-edit
file from disk, as the originals did).

Kept as separate hooks (expensive, mutating, or complex -- not folded here):
    auto_linter.sh      -- gofmt/goimports/golangci, mutates .go files, blocking
    auto_py_format.sh   -- ruff format, mutates .py files
    validate-spec.sh    -- 300-line spec-markdown validator, plan/spec-*.md only

Exit codes: 0 allow/advisory, 1 warning, 2 block. Most severe wins. Fails OPEN
(exit 0) on an unexpected internal error.
"""

import os
import re
import sys

YELLOW = "\033[33m"
RED = "\033[31m"
BOLD = "\033[1m"
RESET = "\033[0m"


def read_file(path, limit=None):
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            return fh.read() if limit is None else fh.read(limit)
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


# --- check-file-size.sh ---
def c_file_size(ctx):
    if not _go(ctx):
        return None
    txt = read_file(ctx["fp"])
    if txt is None:
        return None
    n = txt.count("\n")  # wc -l counts newlines
    base = os.path.basename(ctx["fp"])
    if n > 1000:
        return (1, f"{RED}{BOLD}⚠️  File too large: {base} ({n} lines > 1000){RESET}")
    if n > 600:
        return (1, f"{YELLOW}⚠️  File growing: {base} ({n} lines > 600){RESET}")
    return None


# --- warn-deferral-in-edit.sh ---
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
    if re.search(r"plan/deferrals\.md$", fp):
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
                f"  {YELLOW}Record in plan/deferrals.md if this is deferred work.{RESET}",
            )
    return None


# --- require-rfc-reference.sh (advisory, always exit 0) ---
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


# --- require-test-docs.sh (advisory) ---
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
    test_count = grep_count(txt, r"^func Test[A-Z]", re.MULTILINE)
    if test_count > 0 and "VALIDATES:" not in txt and "PREVENTS:" not in txt:
        return (
            0,
            f"{YELLOW}⚠️  Test file without documentation: {os.path.basename(fp)} "
            f"(add VALIDATES:/PREVENTS: comments){RESET}",
        )
    return None


# --- require-fuzz-tests.sh (advisory) ---
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
    test_file = fp[:-3] + "_test.go"
    has_fuzz = False
    t = read_file(test_file)
    if t and grep_count(t, r"^func Fuzz[A-Z]", re.MULTILINE) > 0:
        has_fuzz = True
    if not has_fuzz:
        directory = os.path.dirname(fp)
        try:
            for f in os.listdir(directory):
                if f.endswith("_test.go"):
                    d = read_file(os.path.join(directory, f))
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


# --- block-vague-names.sh (advisory) ---
def c_vague_names(ctx):
    if not _go(ctx):
        return None
    txt = read_file(ctx["fp"])
    if txt is None:
        return None
    pat = r"(^|[^A-Za-z0-9_])(Data|Info|Result|Item|Thing|Temp|Tmp|Val|Obj)[ \t]+[A-Za-z0-9_]+[ \t]*="
    hits = [l for l in txt.split("\n") if re.search(pat, l)][:3]
    if hits:
        return (
            0,
            f"{YELLOW}⚠️  Vague variable names detected in {os.path.basename(ctx['fp'])}{RESET}",
        )
    return None


# --- require-boundary-tests.sh (advisory) ---
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
    test_file = re.sub(r"\.go.*$", "", fp) + "_test.go"  # bash %%.go
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


CHECKS = (
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
