#!/usr/bin/env python3
"""Check that path references in the instruction corpus resolve.

Two checks:
  1. Markdown corpus: backtick path references and markdown links in the
     normative agent-instruction files (ai/, .claude/rules/, plan/ meta docs)
     must point at files/dirs that exist. Historical records (plan/learned
     numbered summaries, plan/handover/) are NOT checked: they describe the
     tree as it was at the time.
  2. Go sources: the target of every `// Design:` comment must exist
     (`(none ...)` placeholders are allowed).

A line containing `doc-links: ignore` is skipped (use for deliberate
references to removed paths, e.g. negative examples).

Usage: scripts/dev/check_doc_links.py [--md-only|--design-only] [-v]
Called by: make ze-doc-links, make ze-regen-check
Exit codes: 0 = all resolve, 1 = broken references found.
"""

import argparse
import glob as globmod
import os
import re
import subprocess
import sys

KNOWN_ROOTS = {
    "ai",
    ".claude",
    ".codex",
    ".agents",
    ".github",
    "scripts",
    "internal",
    "cmd",
    "pkg",
    "test",
    "plan",
    "docs",
    "mk",
    "rfc",
    "tools",
    "etc",
    "examples",
    "api",
    "gokrazy",
    "third_party",
    "parked",
    "vendor",
    # Non-canonical shorthands, deliberately checked so they FAIL:
    # write ai/rules/, .claude/rules/, ai/patterns/ explicitly.
    "rules",
    "patterns",
}
ROOT_FILES = {
    "Makefile",
    "CLAUDE.md",
    "AGENTS.md",
    "README.md",
    "go.mod",
    "go.sum",
    ".gitignore",
    ".golangci.yml",
    "tools.go",
    "LICENSE",
    "SECURITY.md",
    "CONTRIBUTING.md",
}
# Runtime/build artifacts: existence depends on local state, never checked.
SKIP_PREFIXES = ("tmp/", "bin/", "~", "/", "test/tmp/")
# Tokens containing these are templates, not concrete paths.
PLACEHOLDER_MARKERS = ("<", ">", "$", "*", "NNN", "...", "..")

MD_GLOBS = [
    "ai/INSTRUCTIONS.md",
    "ai/INDEX.md",
    "ai/NAVIGATION.md",
    "ai/LEARNED-INDEX.md",
    "ai/rules/*.md",
    "ai/rationale/*.md",
    "ai/patterns/*.md",
    "ai/skills/*.md",
    ".claude/rules/*.md",
    ".claude/README.md",
    ".claude/hooks/README.md",
    "plan/README.md",
    "plan/TEMPLATE.md",
    "plan/learned/RECURRING-PATTERNS.md",
    "plan/learned/HOOK-FRICTION.md",
    # DESIGN-HISTORY.md is deliberately absent: it is a historical
    # narrative and references paths as they were at the time.
]
# Generated indexes are validated by their own --check generators.
MD_EXCLUDE = {"ai/CODE-TO-DOCS.md"}

BACKTICK = re.compile(r"`([^`]+)`")
MD_LINK = re.compile(r"\]\(([^)#][^)]*)\)")
LINE_SUFFIX = re.compile(r":\d+(?:-\d+)?$")
# `file.go:Symbol` / `pkg.Symbol` references: strip the symbol, check the path.
SYMBOL_COLON = re.compile(r":[A-Za-z_][\w.]*$")
SYMBOL_DOT = re.compile(r"\.[A-Z]\w*$")
LEARNED_NUMBER = re.compile(r"^plan/learned/(\d+)$")
DESIGN = re.compile(r"^// Design:\s*(.+)$")


def expand_braces(token: str) -> list[str]:
    m = re.search(r"\{([^{}]+)\}", token)
    if not m or "," not in m.group(1):
        return [token]
    out = []
    for alt in m.group(1).split(","):
        out.extend(expand_braces(token[: m.start()] + alt + token[m.end() :]))
    return out


def candidate_paths(raw: str) -> list[str]:
    """Reduce a backtick span to concrete repo paths to verify ([] = skip)."""
    token = raw.strip().split()[0] if raw.strip() else ""
    token = token.rstrip(".,;:)('\"")
    token = token.split("#")[0]
    token = LINE_SUFFIX.sub("", token)
    token = SYMBOL_COLON.sub("", token)
    if not os.path.exists(token):
        token = SYMBOL_DOT.sub("", token)
    if not token or "/" not in token and token not in ROOT_FILES:
        return []
    if any(m in token for m in PLACEHOLDER_MARKERS):
        return []
    if token.startswith(SKIP_PREFIXES):
        return []
    root = token.split("/")[0]
    if token not in ROOT_FILES and root not in KNOWN_ROOTS:
        return []
    return [
        t for t in expand_braces(token) if not any(m in t for m in PLACEHOLDER_MARKERS)
    ]


def path_resolves(path: str) -> bool:
    m = LEARNED_NUMBER.match(path)
    if m:
        # `plan/learned/734` shorthand for `plan/learned/734-*.md`
        return bool(globmod.glob(f"plan/learned/{m.group(1)}-*.md"))
    if "*" in path or "?" in path or "[" in path:
        return bool(globmod.glob(path))
    return os.path.exists(path.rstrip("/"))


def check_markdown(verbose: bool) -> list[str]:
    errors = []
    files = []
    for pattern in MD_GLOBS:
        files.extend(sorted(globmod.glob(pattern)))
    for md in files:
        if md in MD_EXCLUDE:
            continue
        with open(md, encoding="utf-8") as fh:
            for lineno, line in enumerate(fh, 1):
                if "doc-links: ignore" in line:
                    continue
                tokens = BACKTICK.findall(line) + MD_LINK.findall(line)
                for raw in tokens:
                    if raw.startswith(("http://", "https://", "mailto:")):
                        continue
                    for path in candidate_paths(raw):
                        if not path_resolves(path):
                            errors.append(
                                f"{md}:{lineno}: broken path reference: {path}"
                            )
                        elif verbose:
                            print(f"ok {md}:{lineno}: {path}")
    return errors


def go_files() -> list[str]:
    out = subprocess.run(
        ["git", "ls-files", "internal/", "cmd/", "pkg/", "scripts/", "*.go"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.splitlines()
    return [f for f in out if f.endswith(".go") and not f.endswith("_test.go")]


def check_design_refs(verbose: bool) -> list[str]:
    errors = []
    for go in go_files():
        try:
            with open(go, encoding="utf-8") as fh:
                head = fh.read(4096)
        except OSError:
            continue
        if "Code generated" in head.split("\n", 3)[0]:
            continue
        for lineno, line in enumerate(head.splitlines(), 1):
            m = DESIGN.match(line)
            if not m:
                continue
            target = m.group(1).strip()
            if target.startswith("("):
                break
            target = target.split()[0].rstrip(".,;:")
            # Same-package anchors ("// Design: pipe.go") resolve relative
            # to the referencing file's own directory.
            if "/" not in target and not os.path.exists(target):
                target = os.path.join(os.path.dirname(go), target)
            if not path_resolves(target):
                errors.append(f"{go}:{lineno}: broken Design reference: {target}")
            elif verbose:
                print(f"ok {go}:{lineno}: {target}")
            break
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--md-only", action="store_true")
    parser.add_argument("--design-only", action="store_true")
    parser.add_argument("-v", "--verbose", action="store_true")
    args = parser.parse_args()

    os.chdir(
        subprocess.run(
            ["git", "rev-parse", "--show-toplevel"],
            capture_output=True,
            text=True,
            check=True,
        ).stdout.strip()
    )

    errors = []
    if not args.design_only:
        errors += check_markdown(args.verbose)
    if not args.md_only:
        errors += check_design_refs(args.verbose)

    for err in errors:
        print(err)
    if errors:
        print(
            f"{len(errors)} broken reference(s) -- fix the reference or "
            f"mark the line with doc-links: ignore",
            file=sys.stderr,
        )
        return 1
    print("all corpus path references resolve")
    return 0


if __name__ == "__main__":
    sys.exit(main())
