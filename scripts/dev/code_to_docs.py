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


def extract_anchor_segments(content: str) -> list[tuple[list[str], str]]:
    """Split a source anchor's content into (code paths, description) segments.

    Handles two formats:
      - Semicolon-separated: path1 -- desc1; path2 -- desc2
      - Comma-separated with relative paths: dir/file1.go, file2.go, dir2/file3.go

    The description is what follows the `--` separator. It is a claim about the
    segment's paths, so it is kept beside them: check_anchor_symbols verifies
    the symbols it names. One description covers every path of its segment.
    """
    segments: list[tuple[list[str], str]] = []
    for seg in content.split(";"):
        seg = seg.strip()
        if not seg:
            continue
        # Split the description off at an accepted source-anchor separator.
        parts = DESC_SEP.split(seg, maxsplit=1)
        seg_path = parts[0].strip()
        description = parts[1].strip() if len(parts) > 1 else ""

        # Handle comma-separated paths within a segment
        # e.g. "internal/component/cmd/show/ipsec.go, ipsec_monitor.go"
        paths: list[str] = []
        last_dir = ""
        for part in (p.strip() for p in seg_path.split(",")):
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
        if paths:
            segments.append((paths, description))
    return segments


def extract_paths(content: str) -> list[str]:
    """Extract code paths from a source anchor's full content."""
    return [path for paths, _ in extract_anchor_segments(content) for path in paths]


def check_path_exists(root: Path, code_path: str) -> bool:
    """Check if a code path exists (file or directory)."""
    target = root / code_path
    return target.exists()


# A declaration claim: an identifier, or a dotted chain of them, and nothing
# else. Everything else a description can hold -- a phrase, a hyphenated binary
# name, a range such as StateIdle..StateEstablished -- describes the file
# rather than naming a declaration in it, and is never checked.
SYMBOL_CLAIM_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)*$")
LINE_SUFFIX_RE = re.compile(r":\d+$")

# Severity rule 1. A single lowercase word -- no capital, no `_`, no `.` -- is
# an English noun in a prose list, not an identifier. Measured over the whole
# tree: of 372 claims that resolved against no declaration, 105 are this shape,
# and every one sampled was prose ("link, address, and route control").
# The separators are what keep the rule narrow: `sa_count` and `ze.storage.blob`
# carry no capital either, and both name something a doc really claims, so
# neither is prose. The cost is priced and accepted: an all-lowercase Go
# declaration (`run`, `main`) can no longer be claimed by an anchor.
PROSE_WORD_RE = re.compile(r"^[a-z][a-z0-9]*$")

# gofmt puts every top-level declaration at column 0 and closes it at column 0,
# so indentation alone separates a declaration from a function body.
GO_FUNC_DECL_RE = re.compile(
    r"^func\s+(?:\(\s*\w*\s*\*?(?P<recv>[A-Za-z_]\w*)(?:\[[^\]]*\])?\s*\)\s*)?"
    r"(?P<name>[A-Za-z_]\w*)"
)
GO_TYPE_BODY_RE = re.compile(
    r"^type\s+(?P<name>[A-Za-z_]\w*)(?:\[[^\]]*\])?\s+(?:struct|interface)\s*\{\s*$"
)
GO_TOP_DECL_RE = re.compile(
    r"^(?:type|var|const)\s+(?P<names>[A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)"
)
GO_GROUP_OPEN_RE = re.compile(r"^(?:type|var|const)\s*\(\s*$")
GO_MEMBER_RE = re.compile(r"^(?P<names>[A-Za-z_]\w*(?:\s*,\s*[A-Za-z_]\w*)*)")


def anchor_symbol_tokens(description: str) -> list[str]:
    """The declaration claims in an anchor description, in order.

    The description is a comma-separated list. A token is a claim when it names
    a declaration and only that; a token holding a space, a hyphen, or any
    other non-identifier text is prose about the file and is dropped. A single
    lowercase word is dropped for the same reason (PROSE_WORD_RE).
    """
    claims: list[str] = []
    for raw in description.split(","):
        token = raw.strip()
        if token.endswith("()"):
            token = token[:-2].rstrip()
        if not token or not SYMBOL_CLAIM_RE.match(token):
            continue
        if PROSE_WORD_RE.match(token):
            continue
        claims.append(token)
    return claims


def go_declarations(text: str) -> tuple[set[str], set[str]]:
    """Names one Go file declares, as (simple names, Recv.Member names).

    A text scan, not a type check: it has no build context, so a declaration
    behind `//go:build linux` is found on any host. It reads declarations only
    -- funcs, methods, types, vars, consts, struct fields, interface methods --
    and never a function body.
    """
    names: set[str] = set()
    dotted: set[str] = set()
    owner: str | None = None  # the type whose body we are inside
    in_group = False
    for line in text.splitlines():
        # A blank line separates members inside a declaration body; it neither
        # opens nor closes one. Testing column 0 first would read "" as a
        # top-level line and drop every member after the first blank one.
        if not line.strip():
            continue
        if line[:1] not in (" ", "\t"):
            owner, in_group = None, False
            match = GO_FUNC_DECL_RE.match(line)
            if match:
                names.add(match.group("name"))
                if match.group("recv"):
                    dotted.add(f"{match.group('recv')}.{match.group('name')}")
                continue
            match = GO_TYPE_BODY_RE.match(line)
            if match:
                names.add(match.group("name"))
                owner = match.group("name")
                continue
            if GO_GROUP_OPEN_RE.match(line):
                in_group = True
                continue
            match = GO_TOP_DECL_RE.match(line)
            if match:
                names.update(n.strip() for n in match.group("names").split(","))
            continue
        if owner is None and not in_group:
            continue
        member = line.strip()
        if not member or member.startswith("//"):
            continue
        match = GO_MEMBER_RE.match(member)
        if not match:
            continue
        for name in (n.strip() for n in match.group("names").split(",")):
            names.add(name)
            if owner is not None:
                dotted.add(f"{owner}.{name}")
    return names, dotted


def claim_is_declared(claim: str, names: set[str], dotted: set[str]) -> bool:
    """True when the anchored files declare what the claim names."""
    if "." not in claim:
        return claim in names
    # A dotted claim resolves when the files declare the member itself, or
    # declare the member's name on its own. It does NOT resolve on the prefix:
    # a package-qualified call such as events.Register names another package's
    # declaration, which these files do not hold.
    return claim in dotted or claim.rsplit(".", 1)[1] in names


def claim_appears_as_word(claim: str, texts: list[str]) -> bool:
    """True when one anchored file's text holds the claim as a whole word.

    Severity rule 2. A reader who follows the anchor and searches for the name
    finds it, so the anchor is not lying even though the file declares nothing
    by that name: a call, a member reached through a receiver, a string key, a
    local, a comment. `\\b` treats `.` as a boundary, so `Run` is found inside
    `p.Run()` and `events.Register` only inside that exact dotted text.
    """
    return any(
        re.search(r"\b" + re.escape(claim) + r"\b", text) is not None for text in texts
    )


def check_anchor_symbols(
    root: Path,
    anchors: list[tuple[str, int, list[str], str]],
) -> list[str]:
    """Verify that every symbol a source anchor names is declared where it points.

    `anchors` holds one (doc file, line number, code paths, description) entry
    per anchor segment, collected by the walk in main() so nothing walks docs/
    a second time. A path that does not exist is left to the stale-reference
    check, which already owns it.

    A claim the files do not declare is reported only when their text does not
    hold it either (claim_appears_as_word). That is severity rule 2: it demotes
    the 230 findings of the call, receiver-member, string-key and comment
    shapes, which are accurate enough for the reader the anchor sends.

    Every finding is returned. What to do with them is the caller's decision:
    main() prints them as CLAIM: lines and exits 1.
    """
    problems: list[str] = []
    # path -> (declared simple names, declared Recv.Member names, file text),
    # or None when the file cannot be read. The text is cached beside the
    # declarations so rule 2 costs no second read.
    declarations: dict[str, tuple[set[str], set[str], str] | None] = {}
    for doc_file, line_no, paths, description in anchors:
        claims = anchor_symbol_tokens(description)
        if not claims:
            continue
        go_paths = [
            clean
            for clean in (LINE_SUFFIX_RE.sub("", path) for path in paths)
            if clean.endswith(".go") and (root / clean).is_file()
        ]
        if not go_paths:
            continue
        names: set[str] = set()
        dotted: set[str] = set()
        texts: list[str] = []
        unreadable: list[str] = []
        for path in go_paths:
            if path not in declarations:
                try:
                    text = (root / path).read_text(encoding="utf-8")
                    declarations[path] = (*go_declarations(text), text)
                except (OSError, UnicodeDecodeError):
                    declarations[path] = None
            decls = declarations[path]
            if decls is None:
                unreadable.append(path)
                continue
            names |= decls[0]
            dotted |= decls[1]
            texts.append(decls[2])
        # Fail closed: an anchored file we cannot read is a finding, never a
        # pass. Its symbols are unverifiable, which is what this check is for.
        for path in unreadable:
            problems.append(
                f"{doc_file}:{line_no}: source anchor {path}: cannot read the "
                "anchored file, so its symbols are unverifiable"
            )
        where = ", ".join(go_paths)
        for claim in claims:
            if claim_is_declared(claim, names, dotted):
                continue
            # An unreadable file makes an unresolved claim UNKNOWN, not absent.
            # Saying "not declared" here would be the false claim this check
            # exists to catch; the unreadable finding above already stands.
            if unreadable:
                continue
            # Rule 2: the file does not DECLARE it, but its text names it, so
            # the reader the anchor sends there finds it. Reported at no
            # severity, which is what the tree can carry (spec AC-3).
            if claim_appears_as_word(claim, texts):
                continue
            problems.append(
                f"{doc_file}:{line_no}: source anchor {where} names "
                f"'{claim}', which is not declared there"
            )
    return problems


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
    # (doc_file, line_number, code paths, description) per anchor segment: the
    # symbol check reads this rather than walking docs/ again.
    anchors: list[tuple[str, int, list[str], str]] = []

    for md_file in filter_gitignored(root, sorted(docs_dir.rglob("*.md"))):
        rel_doc = str(md_file.relative_to(root))
        with open(md_file, encoding="utf-8", errors="replace") as f:
            for line_no, line in enumerate(f, 1):
                for match in ANCHOR_RE.finditer(line):
                    for paths, description in extract_anchor_segments(match.group(1)):
                        anchors.append((rel_doc, line_no, paths, description))
                        for path in paths:
                            index[path].add((rel_doc, line_no))

    # Check for stale references
    stale: list[tuple[str, str, int]] = []  # (code_path, doc_file, line)
    unproven_claims: list[str] = []
    if check_mode:
        for code_path, refs in sorted(index.items()):
            if not check_path_exists(root, code_path):
                for doc_file, line_no in sorted(refs):
                    stale.append((code_path, doc_file, line_no))
        # The tree was measured (372 unresolved claims over 4779), the two
        # severity rules cut that to 122, and all 122 were repaired. A finding
        # here fails the gate, at the bottom of this function. It is REPORTED,
        # never added to `content` -- see the NOTE below.
        unproven_claims = check_anchor_symbols(root, anchors)

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
        "<!-- Regenerate: make ze-doc-index-update -->",
        "",
    ]

    # NOTE: the stale-reference table is deliberately NOT part of `content`.
    # `stale` is only populated when check_mode is true (see above), so this
    # section could never appear in a file written by generate mode -- grep the
    # committed ai/CODE-TO-DOCS.md and there is none. Including it here would
    # make check mode's `content` differ from generate mode's for the same tree
    # whenever any anchor is broken, so the freshness comparison below would
    # report "stale -- run: make ze-doc-index-update" and STILL fail after you ran it:
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
        # Check mode validates the ANCHORS, never the freshness of output_file.
        #
        # It used to do both. The freshness half was added 2026-07-20 after
        # ai/CODE-TO-DOCS.md silently drifted by 24 code paths while check mode
        # reported "all references valid": the file was COMMITTED, so a stale
        # copy was authoritative and wrong, and nothing compared it to the tree.
        # Untracking the file (.gitignore) removes that failure mode at its
        # source rather than guarding it -- git no longer holds a copy that can
        # be stale, and `make ze-doc-index-update` rewrites the working-tree one
        # from scratch. Re-adding the comparison would only make this check fail
        # on a clone where the derived file has not been generated yet, and it
        # exited BEFORE the two validations below, so it would mask them.
        #
        # What stays is what a generated file cannot answer for itself: a
        # `<!-- source: -->` anchor pointing at a path that no longer exists,
        # and one naming a symbol its file does not declare.
        if n_stale:
            print(f"{n_stale} stale references (code path no longer exists)")
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
        # A symbol an anchor names but its file does not declare. Reported on
        # the same terms as a stale path: a claim nobody can verify is the same
        # defect as a pointer nobody can follow.
        if unproven_claims:
            print(f"{len(unproven_claims)} anchor symbol(s) not declared where named")
            for problem in unproven_claims:
                print(f"  CLAIM: {problem}")
            sys.exit(1)
        print("all references valid")


if __name__ == "__main__":
    main()
