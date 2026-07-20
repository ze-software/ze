#!/usr/bin/env python3
"""Generate ai/CODE-TO-DOCS.md: reverse index from source anchors in docs/.

Scans markdown files under docs/ (skipping gitignored research output) for
<!-- source: path -- description --> anchors and inverts the mapping to produce
a code-path -> doc-files index.

Output is grouped by package directory for fast lookup when editing code.

Usage:
    python3 scripts/dev/code_to_docs.py          # generate index
    python3 scripts/dev/code_to_docs.py --check  # also report stale references
"""

import re
import subprocess
import sys
from collections import defaultdict
from pathlib import Path


ANCHOR_RE = re.compile(r"<!--\s*source:\s*(.+?)\s*-->")

PATH_PREFIX = (
    "Makefile",
    "go.mod",
    "internal/",
    "cmd/",
    "pkg/",
    "test/",
    "scripts/",
    "rfc/",
    "mk/",
)
DESC_SEP = re.compile(r"\s+(?:--|-|\u2014)\s+")


def extract_paths(content: str) -> list[str]:
    """Extract code paths from a source anchor's full content.

    Handles two formats:
      - Semicolon-separated: path1 -- desc1; path2 -- desc2
      - Comma-separated with relative paths: dir/file1.go, file2.go, dir2/file3.go
    """
    segments = content.split(";")
    paths = []
    for seg in segments:
        seg = seg.strip()
        if not seg:
            continue
        # Strip description after accepted source-anchor separators.
        seg_path = DESC_SEP.split(seg, maxsplit=1)[0].strip()

        # Handle comma-separated paths within a segment
        # e.g. "internal/component/cmd/show/ipsec.go, ipsec_monitor.go"
        parts = [p.strip() for p in seg_path.split(",")]
        last_dir = ""
        for part in parts:
            if not part:
                continue
            if any(part.startswith(p) for p in PATH_PREFIX):
                paths.append(part)
                # Track directory for relative paths that follow
                if "/" in part:
                    last_dir = "/".join(part.split("/")[:-1])
            elif last_dir and not part.startswith("/"):
                # Relative to last full path's directory
                paths.append(f"{last_dir}/{part}")

    return paths


def check_path_exists(root: Path, code_path: str) -> bool:
    """Check if a code path exists (file or directory)."""
    target = root / code_path
    return target.exists()


def package_dir(path: str) -> str:
    """Return the package directory (parent) for a file path."""
    parts = path.split("/")
    if path.endswith("/"):
        return path.rstrip("/")
    if len(parts) > 1:
        return "/".join(parts[:-1])
    return path


def filter_gitignored(root: Path, paths: list[Path]) -> list[Path]:
    """Drop paths that git ignores, preserving order.

    ai/CODE-TO-DOCS.md is a tracked, committed index, but docs/ also holds
    gitignored research output (docs/research/comparison/, see .gitignore) that
    is present only on machines carrying the local research files. Indexing it
    makes the generated file non-reproducible -- e.g. 1439 code paths on a host
    that has the research docs vs 1438 on a clean checkout. Filtering through
    `git check-ignore` keeps the output identical on every checkout.

    Falls back to the unfiltered list when git is unavailable or the tree is not
    a repository, so the generator still runs outside a git checkout.
    """
    if not paths:
        return paths
    rels = [str(p.relative_to(root)) for p in paths]
    try:
        proc = subprocess.run(
            ["git", "-C", str(root), "check-ignore", "--stdin"],
            input="\n".join(rels),
            capture_output=True,
            text=True,
        )
    except OSError:
        return paths
    # git check-ignore exits 0 when some paths are ignored, 1 when none are, and
    # 128 on error (e.g. not a git repository). Only 0/1 are trustworthy.
    if proc.returncode not in (0, 1):
        return paths
    ignored = {line for line in proc.stdout.splitlines() if line}
    return [path for path, rel in zip(paths, rels) if rel not in ignored]


def main():
    root = Path(__file__).resolve().parents[2]
    docs_dir = root / "docs"
    output_file = root / "ai" / "CODE-TO-DOCS.md"
    check_mode = "--check" in sys.argv

    if not docs_dir.is_dir():
        print(f"error: {docs_dir} not found", file=sys.stderr)
        sys.exit(1)

    # code_path -> set of (doc_file, line_number)
    index: dict[str, set[tuple[str, int]]] = defaultdict(set)

    for md_file in filter_gitignored(root, sorted(docs_dir.rglob("*.md"))):
        rel_doc = str(md_file.relative_to(root))
        with open(md_file, encoding="utf-8", errors="replace") as f:
            for line_no, line in enumerate(f, 1):
                for match in ANCHOR_RE.finditer(line):
                    paths = extract_paths(match.group(1))
                    for path in paths:
                        index[path].add((rel_doc, line_no))

    # Check for stale references
    stale: list[tuple[str, str, int]] = []  # (code_path, doc_file, line)
    if check_mode:
        for code_path, refs in sorted(index.items()):
            if not check_path_exists(root, code_path):
                for doc_file, line_no in sorted(refs):
                    stale.append((code_path, doc_file, line_no))

    # Group by package directory
    pkg_index: dict[str, dict[str, set[str]]] = defaultdict(lambda: defaultdict(set))
    for code_path, doc_refs in index.items():
        pkg = package_dir(code_path)
        for doc_file, _ in doc_refs:
            pkg_index[pkg][code_path].add(doc_file)

    # Generate output
    lines = [
        "# Code to Documentation Index",
        "",
        "<!-- GENERATED by scripts/dev/code_to_docs.py -- do not edit -->",
        "<!-- Regenerate: make ze-doc-index -->",
        "",
        f"Total: {len(index)} code paths referenced from docs/",
        "",
    ]

    # NOTE: the stale-reference table is deliberately NOT part of `content`.
    # `stale` is only populated when check_mode is true (see above), so this
    # section could never appear in a file written by generate mode -- grep the
    # committed ai/CODE-TO-DOCS.md and there is none. Including it here would
    # make check mode's `content` differ from generate mode's for the same tree
    # whenever any anchor is broken, so the freshness comparison below would
    # report "stale -- run: make ze-doc-index" and STILL fail after you ran it:
    # an unfixable loop on a commit-blocking gate, with the useful "MISSING:"
    # report at the end of this function rendered unreachable. Broken anchors
    # are reported to stdout instead, which is what documentation-testing.md
    # documents. Keep `content` identical in both modes.

    for pkg in sorted(pkg_index.keys()):
        files = pkg_index[pkg]
        all_docs = set()
        for doc_set in files.values():
            all_docs.update(doc_set)

        lines.append(f"## `{pkg}/`")
        lines.append("")

        if len(files) <= 3:
            for code_path in sorted(files.keys()):
                docs = sorted(files[code_path])
                lines.append(f"- `{code_path}` -> {', '.join(f'`{d}`' for d in docs)}")
        else:
            lines.append(
                f"Files: {len(files)} | Docs: {', '.join(f'`{d}`' for d in sorted(all_docs))}"
            )
            lines.append("")
            lines.append("| File | Docs |")
            lines.append("|------|------|")
            for code_path in sorted(files.keys()):
                docs = sorted(files[code_path])
                fname = code_path.split("/")[-1]
                lines.append(f"| `{fname}` | {', '.join(f'`{d}`' for d in docs)} |")

        lines.append("")

    content = "\n".join(lines)

    n_stale = len(stale)
    if check_mode:
        print(f"checked {len(index)} code paths, {len(pkg_index)} packages")
    else:
        output_file.write_text(content, encoding="utf-8")
        print(
            f"wrote {output_file} ({len(index)} code paths, {len(pkg_index)} packages)"
        )
    if check_mode:
        # Freshness of the GENERATED FILE itself. This used to be missing: check
        # mode built `content` and then validated only that the anchor paths
        # exist, never comparing against output_file -- so a stale
        # ai/CODE-TO-DOCS.md reported "all references valid" and exit 0. It had
        # silently drifted by 24 code paths (1439 recorded vs 1463 live) before
        # anyone noticed, because the one target that would have caught it
        # (ze-regen-check, via `git diff`) has no callers.
        # A guard that cannot fail is not a guard (ai/rules/fail-closed-guards.md).
        try:
            current = output_file.read_text(encoding="utf-8")
        except OSError as exc:
            print(
                f"ERROR: cannot read {output_file}: {exc} -- run: make ze-doc-index",
                file=sys.stderr,
            )
            sys.exit(1)
        if current != content:
            print(
                f"ERROR: {output_file} is stale -- run: make ze-doc-index",
                file=sys.stderr,
            )
            sys.exit(1)
        # Content is fresh. Do NOT print "up to date" yet: broken anchors below
        # still exit 1, and an "up to date" line one line above a non-zero exit
        # misreads as success. Announce freshness only on the all-clear path.
        if n_stale:
            print(
                f"{output_file} content is fresh, but {n_stale} stale references "
                "(code path no longer exists)"
            )
            # Group by code path for compact output
            by_path: dict[str, list[str]] = defaultdict(list)
            for code_path, doc_file, line_no in stale:
                by_path[code_path].append(f"{doc_file}:{line_no}")
            for path in sorted(by_path):
                refs = by_path[path]
                print(f"  MISSING: {path}")
                for ref in refs[:3]:
                    print(f"           <- {ref}")
                if len(refs) > 3:
                    print(f"           ... and {len(refs) - 3} more")
            sys.exit(1)
        print(f"{output_file} up to date")
        print("all references valid")


if __name__ == "__main__":
    main()
