#!/usr/bin/env python3
"""Rewrite the repository's Go module path, and every path derived from it.

A module path is not a single string. It is the `module` line in go.mod, every
quoted import in every .go file, the `local-prefixes` goimports uses to group
those imports, package paths in build configuration (gokrazy config.json, the
Makefile's `go list` filters), the `replace` targets of the satellite modules
under gokrazy/ and examples/, and -- the one a text rewrite alone cannot reach --
DIRECTORY names that mirror the module path on disk (gokrazy's builddir).

What it does:

  1. exact-string rewrite of the old module path in every tracked text file
  2. rewrite of the same path spelled as separate quoted segments, which is how
     Go code names the on-disk mirror: filepath.Join(.., "codeberg.org",
     "thomas-mangin", "ze"). A literal-only rename leaves these compiling and
     failing at run time against a directory that no longer exists
  3. rename of every tracked directory whose path embeds the old module path
     as a run of path segments (gokrazy/ze/builddir/<module>/)
  4. re-sort of Go import groups with `goimports -format-only -local <new>`,
     because the module-local group is ordered by the module path and the rename
     moves it (`go build` tolerates mis-sorted groups, golangci-lint does not)
  5. re-stamp of the rfc/audit/*.json verdict fingerprints the rename staled.
     Those hash the whole enclosing test file, so rewriting one import line
     invalidates a verdict about assertions nothing touched. A verdict is
     re-sealed ONLY where the requirement text is unchanged and every file it
     names differs from HEAD by nothing but this rename; anything else is
     refused, reported, and left stale for a human to re-read

What it REFUSES, by design:

  - Generated protobuf files (*.pb.go). The embedded rawDesc is the serialized
    FileDescriptorProto: the go_package option is a LENGTH-PREFIXED field, so a
    plain string substitution of a different-length path silently corrupts the
    descriptor while still compiling. These are listed as "regenerate" and the
    .proto source (the canonical input) is rewritten normally, so re-running the
    generator produces the correct bytes. See `make ze-proto-gen`.

  - Occurrences that are NOT module paths but merely contain the same characters
    -- an absolute checkout path such as /Users/<you>/Code/<module>/. These live
    in EXCLUDE_FILES, and every occurrence skipped that way is reported, never
    silently dropped.

Dry-run by DEFAULT: prints the full plan and changes nothing. Pass --apply.

This tool performs NO git operations (`git add`/`git rm`/`git mv` are forbidden
from tooling -- ai/rules/git-safety.md). After --apply the moved directory shows
up as a delete plus an untracked add; the commit script lists both.

Usage:
    python3 scripts/dev/rename_module_path.py --to github.com/ze-software/ze
    python3 scripts/dev/rename_module_path.py --to github.com/ze-software/ze --apply
"""

from __future__ import annotations

import argparse
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

# Trees a module rename must not touch.
#   vendor/             third-party source; carries its own module paths
#   .claude/worktrees/  other sessions' checkouts, on their own branches
#   tmp/, bin/          scratch and build output, regenerated
#
# gokrazy/modcache/ is deliberately NOT here even though it is a module cache of
# third-party trees: the shim go.mod at its root declares a module inside OUR
# namespace and has to be renamed with the rest. Nothing else under it can match,
# because only the exact old module path is rewritten.
#
# rfc/audit/ holds fingerprints and the written history of why each verdict was
# trusted. Neither is source: the fingerprints are updated by reseal_rfc_audits()
# below, and the notes name the module paths of PAST renames, which a textual
# rewrite would silently rewrite into a false account of what happened.
SKIP_PREFIXES = (
    "vendor/",
    ".claude/worktrees/",
    "tmp/",
    "bin/",
    "rfc/audit/",
)

# Files whose occurrences of the old module path are not module paths.
# Every skipped occurrence is reported, so this is a routing decision, not a
# silent exemption.
EXCLUDE_FILES = {
    # Permission globs holding this machine's absolute checkout path
    # (/Users/<you>/Code/<old-module>/), which the rename does not move.
    ".claude/settings.local.json",
}

# Generated files a plain substitution would corrupt: the protobuf rawDesc
# encodes go_package with a varint length prefix.
REGENERATE_SUFFIXES = (".pb.go",)

MODULE_RE = re.compile(r"^module\s+(\S+)\s*$", re.MULTILINE)
# A Go module path: host/owner/name, at least two segments, no scheme.
MODULE_PATH_RE = re.compile(r"^[a-z0-9.\-]+\.[a-z]{2,}(/[\w.\-~]+)+$")
BINARY_SNIFF = 8000


def segment_re(module: str) -> re.Pattern[str]:
    """Match the module path spelled as separate quoted path segments.

    `filepath.Join(root, "gokrazy", "ze", "builddir", "codeberg.org",
    "thomas-mangin", "ze")` is the same path as the literal string, and a rename
    that only substitutes the literal leaves these behind -- compiling, passing
    review, and failing at run time against a directory that no longer exists.
    """
    parts = module.split("/")
    return re.compile(
        r'"' + r'"(\s*,\s*)"'.join(re.escape(p) for p in parts) + r'"',
    )


def rewrite(text: str, old: str, new: str) -> tuple[str, int]:
    """Apply every spelling of the rename. Returns (new text, occurrences)."""
    count = text.count(old)
    text = text.replace(old, new)

    new_parts = new.split("/")
    if len(old.split("/")) == len(new_parts):
        # Same depth, so the segments map one to one and the separators
        # (whitespace, line breaks) of the original call are preserved.
        def sub(m: re.Match[str]) -> str:
            seps = m.groups()
            out = '"' + new_parts[0] + '"'
            for sep, part in zip(seps, new_parts[1:]):
                out += sep + '"' + part + '"'
            return out

        text, n = segment_re(old).subn(sub, text)
        count += n
    return text, count


def repo_root() -> Path:
    return Path(__file__).resolve().parents[2]


def current_module(root: Path) -> str:
    """The `module` path declared by the repository's root go.mod."""
    m = MODULE_RE.search((root / "go.mod").read_text(encoding="utf-8"))
    if not m:
        raise SystemExit("error: no `module` line in go.mod")
    return m.group(1)


def tracked_files(root: Path) -> list[str]:
    """Every git-tracked path, repo-relative, in sorted order."""
    out = subprocess.run(
        ["git", "ls-files", "-z"],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    ).stdout
    return sorted(p for p in out.split("\0") if p)


def skipped(rel: str) -> str | None:
    """Why this path is out of scope, or None when it is in scope."""
    for prefix in SKIP_PREFIXES:
        if rel.startswith(prefix):
            return prefix.rstrip("/")
    if rel in EXCLUDE_FILES:
        return "not-a-module-path"
    return None


def read_text(path: Path) -> str | None:
    """File contents, or None when the file is binary or unreadable as UTF-8."""
    try:
        raw = path.read_bytes()
    except OSError:
        return None
    if b"\0" in raw[:BINARY_SNIFF]:
        return None
    try:
        return raw.decode("utf-8")
    except UnicodeDecodeError:
        return None


def plan_edits(root: Path, files: list[str], old: str, new: str) -> dict[str, list]:
    """Split every occurrence of `old` into edits, regenerations and skips."""
    edits: list[tuple[str, int]] = []
    regenerate: list[tuple[str, int]] = []
    skips: list[tuple[str, int, str]] = []
    for rel in files:
        path = root / rel
        if not path.is_file() or path.is_symlink():
            continue
        text = read_text(path)
        if text is None:
            continue
        _, count = rewrite(text, old, new)
        if count == 0:
            continue
        why = skipped(rel)
        if why is not None:
            # Counted, so an excluded file that stops being excluded is visible
            # rather than invisible.
            skips.append((rel, count, why))
        elif rel.endswith(REGENERATE_SUFFIXES):
            regenerate.append((rel, count))
        else:
            edits.append((rel, count))
    return {"edits": edits, "regenerate": regenerate, "skips": skips}


def plan_moves(
    root: Path, files: list[str], old: str, new: str
) -> list[tuple[str, str]]:
    """Directories whose own path embeds the old module path as path segments.

    Derived from tracked files rather than a filesystem walk, so an untracked
    scratch copy of the same layout under tmp/ is never moved.
    """
    needle = "/" + old + "/"
    moves: set[tuple[str, str]] = set()
    for rel in files:
        if skipped(rel) is not None:
            continue
        marked = "/" + rel
        idx = marked.find(needle)
        if idx < 0:
            continue
        # Move the shallowest directory that spells out the whole module path.
        src = marked[1 : idx + len(needle) - 1]
        dst = src[: len(src) - len(old)] + new
        moves.add((src, dst))
    return sorted(moves)


def apply_edits(root: Path, edits: list[tuple[str, int]], old: str, new: str) -> int:
    changed = 0
    for rel, _ in edits:
        path = root / rel
        text = read_text(path)
        if text is None:
            continue
        updated, count = rewrite(text, old, new)
        if count == 0:
            continue
        path.write_text(updated, encoding="utf-8")
        changed += 1
    return changed


def apply_moves(root: Path, moves: list[tuple[str, str]]) -> int:
    moved = 0
    for src, dst in moves:
        src_path, dst_path = root / src, root / dst
        if not src_path.is_dir():
            continue
        if dst_path.exists():
            raise SystemExit(f"error: destination already exists: {dst}")
        dst_path.parent.mkdir(parents=True, exist_ok=True)
        shutil.move(str(src_path), str(dst_path))
        moved += 1
        # Leave no empty husk of the old layout behind (codeberg.org/thomas-mangin/).
        parent = src_path.parent
        while parent != root and parent.is_dir() and not any(parent.iterdir()):
            parent.rmdir()
            parent = parent.parent
    return moved


def _head_blob(root: Path, rel: str) -> str | None:
    """The file's committed content at HEAD, or None when it is not there."""
    out = subprocess.run(
        ["git", "show", f"HEAD:{rel}"],
        cwd=root,
        capture_output=True,
        text=True,
    )
    return out.stdout if out.returncode == 0 else None


def rename_only_since_head(root: Path, rel: str, old: str, new: str) -> bool:
    """True when the ONLY difference from HEAD is this rename.

    Compared under rfc_requirements' own normalisation (blank lines and
    indentation dropped), so a re-sort that does not move a line of code reads
    as unchanged. Anything else -- a moved import, an edited assertion -- fails,
    and every caller treats that as a refusal.
    """
    rr = _rfc_requirements(root)
    head = _head_blob(root, rel)
    current = read_text(root / rel)
    if rr is None or head is None or current is None:
        return False
    return rr._normalize(head).replace(old, new) == rr._normalize(current)


def _rfc_requirements(root: Path):
    """The repo's RFC coverage tool, imported as a library (None when absent).

    Its private `_normalize` and the `test_sha`/`tagged_unit_shas` pair are used
    rather than reimplemented: a second copy of the fingerprint rule that drifted
    would re-seal verdicts against a hash the gate does not compute.
    """
    sys.path.insert(0, str(root / "scripts" / "dev"))
    try:
        import rfc_requirements  # noqa: PLC0415

        return rfc_requirements
    except ImportError:
        return None


def reseal_rfc_audits(root: Path, old: str, new: str) -> tuple[list[str], list[str]]:
    """Re-fingerprint rfc/audit/*.json verdicts this rename staled, and only those.

    `make ze-rfc-check` fails a recorded audit verdict once the requirement text
    or ANY tagged test file stops being byte-identical to what was judged
    (`check_audit_freshness`). The fingerprint hashes the whole enclosing file,
    so rewriting an import line stales a verdict about assertions the rename
    never touched.

    THE RE-STAMP LOOP ITSELF NOW LIVES IN THE GATE (`rfc_requirements.reseal_audits`).
    This function used to own a second copy of it, which is the hazard the docstring
    below already named: a second copy of the fingerprint rule that drifted would
    re-seal verdicts against a hash the gate does not compute. Since
    plan/spec-rfcgate-3-audit-teeth.md the gate resolves freshness into four states and
    re-stamps exactly the mechanical one, so there is one rule and this is a caller of
    it (AC-22).

    What stays here is the rename-SPECIFIC extra proof: `rename_only_since_head` is
    passed in as the `prove` predicate, so a verdict is re-sealed only when the gate
    calls the change mechanical AND every file it names differs from HEAD by nothing
    but this rename. Two independent proofs, and this one can only ever refuse more.

    Returns (resealed, refused).
    """
    rr = _rfc_requirements(root)
    if rr is None:
        return [], ["rfc_requirements.py not importable"]
    note = (
        f"Mechanical re-stamp by scripts/dev/rename_module_path.py: the module "
        f"path moved from {old} to {new}. The whole-file fingerprint stales a verdict "
        f"about assertions the rename never touched. Every verdict re-stamped below was "
        f"re-sealed ONLY after the gate itself judged the change mechanical (the tagged "
        f"unit and the requirement text byte-identical to what was judged) AND after "
        f"proving, per file, that its normalised content at HEAD with {old} replaced by "
        f"{new} is identical to its content now. A verdict failing either proof was "
        f"refused and left stale. The proof is the code in "
        f"rfc_requirements.reseal_audits() and rename_only_since_head(), not this note."
    )
    try:
        return rr.reseal_audits(
            prove=lambda rel: rename_only_since_head(root, rel, old, new),
            note=note,
        )
    except Exception as exc:  # the gate itself will report the real error
        return [], [f"cannot re-seal RFC audits: {exc}"]


def go_targets(
    root: Path, edits: list[tuple[str, int]], moves: list[tuple[str, str]]
) -> list[str]:
    """Rewritten .go files, at their post-move paths, minus generated all*.go.

    The generated composition roots are excluded: the generator owns their exact
    byte format and goimports would fight it. `make generate` refreshes them.
    """
    out = []
    for rel, _ in edits:
        if not rel.endswith(".go"):
            continue
        for src, dst in moves:
            if rel.startswith(src + "/"):
                rel = dst + rel[len(src) :]
                break
        name = os.path.basename(rel)
        if name == "all.go" or (name.startswith("all_") and name.endswith(".go")):
            continue
        out.append(rel)
    return out


def run_goimports(root: Path, module: str, targets: list[str]) -> None:
    """Re-sort import groups: the module-local group is keyed by the module path.

    `-format-only` keeps this a pure re-sort. Plain `-w` would also add and
    remove imports, and goimports resolves them for ONE build context, so it
    strips imports that only a build-tagged file uses.
    """
    if not targets:
        return
    gi = shutil.which("goimports")
    if not gi:
        print(
            "  goimports NOT found -- import groups are now mis-sorted; run:\n"
            f"    goimports -format-only -w -local {module} ./internal ./cmd ./pkg ./test ./scripts"
        )
        return
    print(
        f"  running: goimports -format-only -w -local {module} ({len(targets)} file(s))"
    )
    # Chunked: the argument list is thousands of paths long.
    for i in range(0, len(targets), 400):
        subprocess.run(
            [gi, "-format-only", "-w", "-local", module, *targets[i : i + 400]],
            cwd=root,
            check=False,
        )


def residual_report(root: Path, old: str) -> list[tuple[str, int]]:
    """Remaining references to the old module's HOST, for human classification.

    These are hosting URLs, historical prose in plan/learned, and third-party
    files that legitimately name the same host. A module rename does not decide
    what happens to them, so it reports them instead of guessing.
    """
    host = old.split("/", 1)[0]
    hits: list[tuple[str, int]] = []
    for rel in tracked_files(root):
        if rel.startswith(SKIP_PREFIXES):
            continue
        path = root / rel
        if not path.is_file() or path.is_symlink():
            continue
        text = read_text(path)
        if text and host in text:
            hits.append((rel, text.count(host)))
    return hits


def show(title: str, rows: list, limit: int) -> None:
    print(f"\n{title} ({len(rows)})")
    for row in rows[:limit]:
        print("  " + "  ".join(str(c) for c in row))
    if len(rows) > limit:
        print(f"  ... and {len(rows) - limit} more")


def main(argv: list[str] | None = None) -> int:
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--to", required=True, metavar="MODULE", help="the new module path")
    ap.add_argument(
        "--from",
        dest="old",
        metavar="MODULE",
        help="the old module path (default: the `module` line in go.mod)",
    )
    ap.add_argument(
        "--apply", action="store_true", help="perform the rename (default: dry run)"
    )
    ap.add_argument("--repo", help="repository root (default: this script's repo)")
    ap.add_argument("--limit", type=int, default=15, help="rows printed per section")
    ap.add_argument(
        "--no-goimports", action="store_true", help="skip the import re-sort"
    )
    ap.add_argument(
        "--no-reseal",
        action="store_true",
        help="skip the rfc/audit fingerprint re-stamp",
    )
    args = ap.parse_args(argv)

    root = Path(args.repo).resolve() if args.repo else repo_root()
    old = args.old or current_module(root)
    new = args.to

    for label, value in (("--from", old), ("--to", new)):
        if not MODULE_PATH_RE.match(value):
            print(
                f"error: {label} {value!r} is not a module path (host/owner/name)",
                file=sys.stderr,
            )
            return 2
    if old == new:
        print(f"error: --from and --to are both {old}", file=sys.stderr)
        return 2

    files = tracked_files(root)
    plan = plan_edits(root, files, old, new)
    # git still reports the pre-move path until the rename is committed, so drop
    # the ones already done: a second run should report the work left, not repeat
    # a move it already made.
    moves = [m for m in plan_moves(root, files, old, new) if (root / m[0]).is_dir()]

    print(f"rename {old}")
    print(f"    -> {new}")
    occurrences = sum(c for _, c in plan["edits"])
    print(
        f"\n{occurrences} occurrence(s) in {len(plan['edits'])} file(s), {len(moves)} directory move(s)"
    )
    show("rewrite", plan["edits"], args.limit)
    if moves:
        show("move", [(s, "->", d) for s, d in moves], args.limit)
    if plan["regenerate"]:
        show(
            "REGENERATE (length-prefixed, not rewritten)",
            plan["regenerate"],
            args.limit,
        )
    if plan["skips"]:
        show("skipped (reported, not rewritten)", plan["skips"], args.limit)

    if not args.apply:
        print("\ndry run -- nothing changed. Re-run with --apply.")
        return 0

    print("\napplying")
    changed = apply_edits(root, plan["edits"], old, new)
    print(f"  rewrote {changed} file(s)")
    moved = apply_moves(root, moves)
    print(f"  moved {moved} director(y|ies)")
    if not args.no_goimports:
        run_goimports(root, new, go_targets(root, plan["edits"], moves))

    left = plan_edits(root, tracked_files(root), old, new)["edits"]
    if left:
        show("STILL CONTAIN THE OLD MODULE PATH", left, args.limit)
        return 1

    if not args.no_reseal:
        resealed, refused = reseal_rfc_audits(root, old, new)
        print(f"  re-sealed {len(resealed)} RFC audit verdict(s)")
        if refused:
            show(
                "RFC audit verdicts REFUSED (left stale -- re-read them: /ze-rfc-audit)",
                [(r,) for r in refused],
                args.limit,
            )

    if plan["regenerate"]:
        print("\nregenerate these, then verify (a plain rewrite would corrupt them):")
        for rel, _ in plan["regenerate"]:
            print(f"  {rel}")
        print("  make ze-proto-gen")

    hits = residual_report(root, old)
    if hits:
        show(
            f"residual {old.split('/', 1)[0]} references (hosting URLs, history) -- classify by hand",
            hits,
            args.limit,
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
