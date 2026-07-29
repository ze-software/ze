#!/usr/bin/env python3
"""Generate ai/PACKAGE-MAP.md: one line per Go package, saying what it does.

Answers "what does what" without opening every package. The responsibility text
is derived, in priority order, from:
  1. the package's `// Package <name> ...` doc comment (first sentence), else
  2. the plugin registry `Description:` in the package's register.go, else
  3. `TODO` -- which turns the map into a doc-coverage worklist.

Nothing is hand-maintained: the map is regenerated from the tree, and a stale
map fails `make ze-discovery-index-check`. The registry `Name:` is surfaced so a
directory can be mapped to the name a plugin registers under (and back).

Usage:
    python3 scripts/dev/package_map.py          # regenerate ai/PACKAGE-MAP.md
    python3 scripts/dev/package_map.py --check   # exit 1 if the map is stale
    python3 scripts/dev/package_map.py --root DIR   # run against another tree

`--root` exists so commit_helper.py can point this generator at a materialized
commit view (HEAD plus a commit's own files) instead of the working tree. In
write mode it writes into that tree, not the repo.
"""

import os
import re
import sys
from pathlib import Path

from discovery_sources import root_from_argv


ROOTS = ("internal", "pkg", "cmd")
SKIP_DIRS = {"vendor", "tmp", "testdata", "node_modules", ".git"}
HEADER_LINES = 40
MAX_SUMMARY = 200

PKG_RE = re.compile(r"^//\s*Package\s+\S+\s+(.*)$")
COMMENT_RE = re.compile(r"^//\s?(.*)$")
# Struct-literal fields in a register.go Registration{...}. One-line string values.
DESC_RE = re.compile(r'Description:\s*"((?:[^"\\]|\\.)*)"')
NAME_RE = re.compile(r'\bName:\s*"((?:[^"\\]|\\.)*)"')


def head(path: Path, n: int) -> list[str]:
    try:
        with open(path, encoding="utf-8", errors="replace") as fh:
            out = []
            for i, line in enumerate(fh):
                if i >= n:
                    break
                out.append(line)
            return out
    except OSError:
        return []


def first_sentence(text: str) -> str:
    text = " ".join(text.split())
    dot = text.find(". ")
    if dot != -1:
        text = text[:dot]
    elif text.endswith("."):
        text = text[:-1]
    if len(text) > MAX_SUMMARY:
        cut = text.rfind(" ", 0, MAX_SUMMARY - 1)
        text = text[: cut if cut > 0 else MAX_SUMMARY - 1].rstrip() + "..."
    return text.strip()


def package_doc(path: Path) -> str:
    """First sentence of the `// Package ...` comment, joined across // lines."""
    lines = head(path, HEADER_LINES)
    for i, line in enumerate(lines):
        m = PKG_RE.match(line.strip())
        if not m:
            continue
        parts = [m.group(1).strip()]
        for cont in lines[i + 1 :]:
            stripped = cont.strip()
            cm = COMMENT_RE.match(stripped)
            if not cm:
                break
            parts.append(cm.group(1).strip())
            if "." in cm.group(1):
                break
        return first_sentence(" ".join(p for p in parts if p))
    return ""


def registration(reg: Path) -> tuple[str, str]:
    """Return (registered name, Description) from a register.go, best effort."""
    try:
        text = reg.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return "", ""
    nm = NAME_RE.search(text)
    dm = DESC_RE.search(text)
    return (nm.group(1) if nm else "", dm.group(1) if dm else "")


def area_of(rel: str) -> str:
    if rel.startswith("internal/component/bgp/plugins/"):
        return "internal/component/bgp/plugins"
    if rel.startswith("internal/component/bgp/"):
        return "internal/component/bgp"
    parts = rel.split("/")
    return "/".join(parts[:2]) if len(parts) >= 2 else rel


def build(root: Path) -> dict[str, tuple[str, str]]:
    """Return rel_package_path -> (responsibility, registered_name)."""
    packages: dict[str, tuple[str, str]] = {}
    for top in ROOTS:
        base = root / top
        if not base.is_dir():
            continue
        for dirpath, dirnames, filenames in os.walk(base):
            dirnames[:] = sorted(
                d for d in dirnames if d not in SKIP_DIRS and not d.startswith(".")
            )
            gofiles = [
                f for f in filenames if f.endswith(".go") and not f.endswith("_test.go")
            ]
            if not gofiles:
                continue
            # Pure `//go:embed` packages (e.g. a plugin's yang/ schema dir) do
            # nothing an agent needs a one-line description for. Skip them so the
            # TODO column is a real doc-coverage worklist, not schema noise.
            if set(gofiles) <= {"embed.go"}:
                continue
            here = Path(dirpath)
            rel = str(here.relative_to(root))

            ordered = (["doc.go"] if "doc.go" in gofiles else []) + sorted(
                f for f in gofiles if f != "doc.go"
            )
            doc = ""
            for fname in ordered:
                doc = package_doc(here / fname)
                if doc:
                    break

            name, desc = ("", "")
            if "register.go" in gofiles:
                name, desc = registration(here / "register.go")

            responsibility = doc or desc or "TODO"
            packages[rel] = (responsibility, name)
    return packages


def render(packages: dict[str, tuple[str, str]]) -> str:
    todo = sum(1 for resp, _ in packages.values() if resp == "TODO")
    lines = [
        "# Package Map",
        "",
        "<!-- GENERATED by scripts/dev/package_map.py -- do not edit -->",
        "<!-- Regenerate: make ze-discovery-index -->",
        "",
        "One line per Go package under `internal/`, `pkg/`, `cmd/`. Responsibility",
        "comes from the `// Package` doc comment, else the plugin registry",
        "`Description`, else `TODO` (a package that still needs a doc comment).",
        "`Registered` is the name the package registers under, where it has a",
        "register.go. Design docs per file: `ai/DOCS-TO-CODE.md`.",
        "",
        f"Total: {len(packages)} packages, {len(packages) - todo} described, {todo} TODO",
        "",
    ]
    current = None
    for rel in sorted(packages):
        area = area_of(rel)
        if area != current:
            current = area
            lines += [
                "",
                f"## `{area}/`",
                "",
                "| Package | Responsibility | Registered |",
                "|---------|----------------|------------|",
            ]
        responsibility, name = packages[rel]
        safe = responsibility.replace("|", "\\|")
        lines.append(f"| `{rel}` | {safe} | {name} |")
    lines.append("")
    return "\n".join(lines)


def main() -> int:
    root = root_from_argv(__file__)
    output_file = root / "ai" / "PACKAGE-MAP.md"
    check_mode = "--check" in sys.argv

    if not (root / "ai").is_dir():
        print(f"error: {root / 'ai'} not found", file=sys.stderr)
        return 1

    packages = build(root)
    content = render(packages)

    if check_mode:
        current = output_file.read_text(encoding="utf-8") if output_file.exists() else ""
        if current != content:
            print(
                f"WARNING: {output_file.relative_to(root)} is stale -- "
                "run: make ze-discovery-index",
                file=sys.stderr,
            )
            return 1
        print(f"checked {len(packages)} packages, ai/PACKAGE-MAP.md up to date")
        return 0

    output_file.write_text(content, encoding="utf-8")
    print(f"wrote {output_file} ({len(packages)} packages)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
