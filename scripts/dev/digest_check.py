#!/usr/bin/env python3
"""Validate the `file:line` anchors in ai/digests/*.md against the live tree.

The subsystem flow digests under `ai/digests/` are hand-maintained (not
generated), so their `file:line` anchors rot silently when code moves. This
checker makes the checkable part mechanical: every backtick anchor of the form
`path.go:123` (or `path.go:123-140`, or a bare `path.go`) must resolve to a real
file, and any line number must be inside that file. A digest that references a
deleted file or an out-of-range line fails the check -- the same guarantee the
per-file `// Design:` targets already get, applied to the digests.

Anchors are written subsystem-relative (a digest about the BGP reactor writes
`peer.go:754`, not the full `internal/component/bgp/reactor/peer.go:754`), so
each digest declares its subtree(s) with a machine-readable header:

    <!-- digest-base: internal/component/bgp internal/core/bgp -->

A bare or partial anchor is resolved by searching those subtrees for a file
whose path ends with the anchor (unique match required); a full repo-relative
anchor (starting `internal/`, `docs/`, `pkg/`, ...) is resolved against the repo
root directly. Ambiguous anchors fail closed and ask you to qualify the path.

Usage:
    python3 scripts/dev/digest_check.py            # validate, exit 1 on any error
    python3 scripts/dev/digest_check.py --check    # alias, for make-target symmetry
    python3 scripts/dev/digest_check.py --json      # machine-readable report
    python3 scripts/dev/digest_check.py --list      # print every resolved anchor
"""

import json
import os
import re
import sys
from pathlib import Path


DIGEST_DIR = ("ai", "digests")
SKIP_FILES = {"README.md"}
SKIP_WALK = {"vendor", "tmp", "testdata", "node_modules", ".git"}

# Top-level dirs that mark a token as an already repo-relative path (resolved
# against the repo root, not a digest base).
TOP_DIRS = {
    "internal",
    "pkg",
    "cmd",
    "scripts",
    "ai",
    "plan",
    "docs",
    "rfc",
    "mk",
    "test",
    "hooks",
    "yang",
    ".claude",
    ".codex",
    ".agents",
}

# Code-ish extensions an anchor may point at. Excludes bare prose in backticks
# (`EventDispatcher`, `s.mu.RLock()`, `ze.bgp.openwait`) which carry no slash,
# no extension, or a trailing paren.
EXTS = "go|py|md|yang|sh|mk|ya?ml|json|txt|proto|c|h|tmpl|html"

BASE_RE = re.compile(r"<!--\s*digest-base:\s*(.+?)\s*-->")
BACKTICK_RE = re.compile(r"`([^`]+)`")
# A line spec is one line (`12`), a range (`12-20`), or a comma list of either
# (`12,20`, `12-14,49`). The comma form is common in the digests to cite several
# lines of one file; without it those cites would be silently unvalidated.
ANCHOR_RE = re.compile(
    r"^(?P<path>[A-Za-z0-9_][\w./-]*\.(?:" + EXTS + r"))"
    r"(?::(?P<lines>\d+(?:-\d+)?(?:,\d+(?:-\d+)?)*))?$"
)


def digest_files(root: Path) -> list[Path]:
    d = root.joinpath(*DIGEST_DIR)
    if not d.is_dir():
        return []
    return sorted(p for p in d.glob("*.md") if p.name not in SKIP_FILES)


def parse_bases(text: str) -> list[str]:
    bases: list[str] = []
    for m in BASE_RE.finditer(text):
        for tok in re.split(r"[,\s]+", m.group(1).strip()):
            if tok and tok not in bases:
                bases.append(tok)
    return bases


def anchors_in(text: str) -> list[tuple[str, int | None, int | None]]:
    """Every backtick token that looks like a code anchor: (path, start, end).

    A comma list (`file.go:12,20-24`) expands to one anchor per element, so every
    cited line is validated instead of the whole token being dropped."""
    out = []
    for m in BACKTICK_RE.finditer(text):
        am = ANCHOR_RE.match(m.group(1).strip())
        if not am:
            continue
        path = am.group("path")
        lines = am.group("lines")
        if not lines:
            out.append((path, None, None))
            continue
        for piece in lines.split(","):
            if "-" in piece:
                a, b = piece.split("-", 1)
                out.append((path, int(a), int(b)))
            else:
                out.append((path, int(piece), None))
    return out


_base_index_cache: dict[str, list[str]] = {}


def base_index(root: Path, base: str) -> list[str]:
    """Repo-relative paths of every file under a digest base (cached).

    Cache is keyed by absolute base dir so tests using multiple temp roots that
    reuse the same relative base name do not poison each other."""
    basedir = root / base
    key = str(basedir)
    if key in _base_index_cache:
        return _base_index_cache[key]
    paths: list[str] = []
    if basedir.is_dir():
        for dirpath, dirnames, filenames in os.walk(basedir):
            dirnames[:] = [
                d for d in dirnames if d not in SKIP_WALK and not d.startswith(".")
            ]
            for f in filenames:
                paths.append(str(Path(dirpath, f).relative_to(root)))
    _base_index_cache[key] = paths
    return paths


def is_repo_relative(path: str) -> bool:
    head = path.split("/", 1)[0]
    return head in TOP_DIRS


def matches_in_base(root: Path, base: str, path: str) -> list[str]:
    """Files under one base an anchor path could mean. A base+path exact hit is
    unambiguous; otherwise fall back to suffix (has slash) or basename match."""
    index = base_index(root, base)
    direct = f"{base}/{path}"
    if direct in index:
        return [direct]
    tail = "/" + path
    has_slash = "/" in path
    basename = path.rsplit("/", 1)[-1]
    return [
        rel
        for rel in index
        if (rel.endswith(tail) if has_slash else rel.rsplit("/", 1)[-1] == basename)
    ]


def resolve(root: Path, bases: list[str], path: str) -> list[str]:
    """Repo-relative file(s) an anchor path resolves to.

    A full repo-relative anchor short-circuits. Otherwise the match must be
    unique across ALL declared bases (deduplicated, so overlapping bases that
    reach the same file are fine). A bare `command.go` that exists under two
    different bases resolves to more than one file and is reported as ambiguous
    -- it fails closed, not to whichever base happens to be listed first, which
    could silently validate the anchor against the wrong same-named file. Qualify
    such an anchor with enough path (or a full repo-relative path) to make it
    unique."""
    if is_repo_relative(path):
        return [path]
    hits: list[str] = []
    for base in bases:
        for hit in matches_in_base(root, base, path):
            if hit not in hits:
                hits.append(hit)
    return hits


def line_count(root: Path, rel: str) -> int:
    try:
        with open(root / rel, "rb") as fh:
            return sum(1 for _ in fh)
    except OSError:
        return -1


def check_digest(root: Path, path: Path) -> tuple[list[dict], list[dict]]:
    """Return (errors, resolved) for one digest."""
    text = path.read_text(encoding="utf-8", errors="replace")
    rel_digest = str(path.relative_to(root))
    bases = parse_bases(text)
    anchors = anchors_in(text)
    errors: list[dict] = []
    resolved: list[dict] = []

    # A base header is required only for the strict case: a subsystem-relative
    # anchor that carries a line number. Bare no-line mentions are informal.
    needs_base = any(
        not is_repo_relative(p) and start is not None for p, start, _ in anchors
    )
    if needs_base and not bases:
        errors.append(
            {
                "digest": rel_digest,
                "anchor": "(header)",
                "problem": "no `<!-- digest-base: <subtree> -->` header, so "
                "subsystem-relative `file:line` anchors cannot be resolved",
            }
        )
        return errors, resolved

    for base in bases:
        if not (root / base).is_dir():
            errors.append(
                {
                    "digest": rel_digest,
                    "anchor": f"digest-base: {base}",
                    "problem": f"declared base subtree `{base}` does not exist",
                }
            )

    for pth, start, end in anchors:
        anchor = pth + (f":{start}" + (f"-{end}" if end else "") if start else "")
        hits = resolve(root, bases, pth)

        if start is None:
            # No line number. Full repo-relative paths (See-also links, doc
            # cross-refs) must exist. Bare basename mentions (`register.go`,
            # `176-topic.md`) are informal shorthand -- validate only when they
            # resolve to exactly one file, never fail on 0 or ambiguous.
            if is_repo_relative(pth):
                if hits and (root / hits[0]).is_file():
                    resolved.append(
                        {"digest": rel_digest, "anchor": anchor, "file": hits[0]}
                    )
                else:
                    errors.append(
                        {
                            "digest": rel_digest,
                            "anchor": anchor,
                            "problem": "linked file does not exist",
                        }
                    )
            elif len(hits) == 1 and (root / hits[0]).is_file():
                resolved.append(
                    {"digest": rel_digest, "anchor": anchor, "file": hits[0]}
                )
            continue

        # Has a line number: strict.
        if not hits:
            errors.append(
                {
                    "digest": rel_digest,
                    "anchor": anchor,
                    "problem": f"file not found under {bases or ['repo root']}",
                }
            )
            continue
        if len(hits) > 1:
            errors.append(
                {
                    "digest": rel_digest,
                    "anchor": anchor,
                    "problem": "ambiguous -- matches "
                    + ", ".join(hits)
                    + "; qualify the path",
                }
            )
            continue
        rel = hits[0]
        if not (root / rel).is_file():
            errors.append(
                {
                    "digest": rel_digest,
                    "anchor": anchor,
                    "problem": f"resolved to `{rel}` which does not exist",
                }
            )
            continue
        n = line_count(root, rel)
        hi = end if end is not None else start
        if end is not None and end < start:
            errors.append(
                {
                    "digest": rel_digest,
                    "anchor": anchor,
                    "problem": f"reversed line range {start}-{end}",
                }
            )
            continue
        if start < 1 or hi > n:
            errors.append(
                {
                    "digest": rel_digest,
                    "anchor": anchor,
                    "problem": f"line {hi} out of range (`{rel}` has {n} lines)",
                }
            )
            continue
        resolved.append({"digest": rel_digest, "anchor": anchor, "file": rel})
    return errors, resolved


def main() -> int:
    root = Path(__file__).resolve().parents[2]
    as_json = "--json" in sys.argv
    listing = "--list" in sys.argv

    digests = digest_files(root)
    all_errors: list[dict] = []
    all_resolved: list[dict] = []
    for d in digests:
        errs, res = check_digest(root, d)
        all_errors += errs
        all_resolved += res

    if as_json:
        print(
            json.dumps(
                {
                    "digests": len(digests),
                    "anchors": len(all_resolved) + len(all_errors),
                    "errors": all_errors,
                },
                indent=2,
            )
        )
        return 1 if all_errors else 0

    if listing:
        for r in all_resolved:
            print(f"{r['digest']}: {r['anchor']} -> {r['file']}")

    if all_errors:
        print(
            f"digest anchor check FAILED: {len(all_errors)} bad anchor(s) in "
            f"{len({e['digest'] for e in all_errors})} digest(s)",
            file=sys.stderr,
        )
        for e in all_errors:
            print(
                f"  {e['digest']}: `{e['anchor']}` -- {e['problem']}", file=sys.stderr
            )
        print(
            "Fix the anchor or run the file to find the moved line, then update "
            "the digest (these are hand-maintained: ai/digests/README.md).",
            file=sys.stderr,
        )
        return 1

    print(
        f"checked {len(all_resolved)} anchors across {len(digests)} digests, all resolve"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
