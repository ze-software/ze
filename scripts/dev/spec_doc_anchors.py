#!/usr/bin/env python3
"""Refuse a spec that omits a design document owned by the code it changes.

Two edges tie a source file to a document, and they are not equally strong:

  1. `// Design: <doc> -- topic` in the file's own header. The file DECLARES that
     document as its design. Changing the file's behavior without naming that doc
     is how a design change ships with its design unwritten. Enforced here as an
     ERROR, read from the file itself so the check never depends on an index being
     fresh.
  2. `<!-- source: <path> -->` inside a document. The doc MENTIONS the file.
     `ai/CODE-TO-DOCS.md` inverts these (scripts/dev/code_to_docs.py). A change can
     legitimately leave most of them alone, so these are WARNINGS.

`plan/TEMPLATE.md` row 16 of the Documentation Update Checklist already asks the
question. It was honor-system: the author answers from memory and names the docs
they happen to remember. `spec-streaming-answer-protocol` changes
`pkg/plugin/rpc/message.go` and `pkg/plugin/rpc/mux.go`, both of which declare
`// Design: docs/architecture/api/ipc_protocol.md`. Its first draft answered row 16
"Yes", listed two other documents, and never named `ipc_protocol.md` -- the primary
wire document and the declared design of the two files being changed.

Naming is the whole requirement. The author may list a document under
`## Files to Modify`, or record in the Documentation Update Checklist why it is
unaffected. What is refused is silence, because silence cannot be told apart from
not having looked.

Usage:
    python3 scripts/dev/spec_doc_anchors.py plan/spec-foo.md   # exit 1 on a missing owner
    python3 scripts/dev/spec_doc_anchors.py --json plan/spec-foo.md
"""

import json
import re
import sys
from collections import defaultdict
from pathlib import Path

INDEX_PATH = Path("ai/CODE-TO-DOCS.md")

# A `## `dir/`` heading opens a package section in the index.
SECTION_RE = re.compile(r"^##\s+`([^`]+)`\s*$")
# A `| `name.go` | `docs/a.md`, `docs/b.md` |` row maps one file to its docs.
ROW_RE = re.compile(r"^\|\s*`([^`]*)`\s*\|\s*(.+?)\s*\|\s*$")
DOC_RE = re.compile(r"`([^`]+\.md)`")
# A `- `path` - description` bullet in a Files section names one path.
BULLET_RE = re.compile(r"^\s*-\s+`([^`]+)`")
# The header edge. The separator after the path is an em dash, `--`, `-` or `:`.
DESIGN_RE = re.compile(r"^//\s*Design:\s*(\S+\.md)\b")
# `// Design:` lives in the file header block, not scattered through the body.
HEADER_LINES = 25

# Only paths the two edges can cover are looked up. A doc path or a glob in a
# Files section is not a source file and must not be reported as a missed owner.
CODE_PREFIX = ("internal/", "cmd/", "pkg/", "scripts/", "test/", "mk/", "rfc/")
CODE_SUFFIX = (".go", ".py", ".sh", ".mk", ".yang")


def design_doc(root: Path, path: str) -> str:
    """The document a source file declares as its design, or "" if it declares none.

    Read from the file, never from `ai/DOCS-TO-CODE.md`: the generated index can be
    stale, and a stale index would silently drop the strongest edge this check has.
    A file that does not exist yet (the spec plans to create it) declares nothing,
    which is not a finding -- the spec cannot omit a doc no file has named.
    """
    source = root / path
    if not source.is_file():
        return ""
    try:
        with source.open(encoding="utf-8", errors="replace") as handle:
            for _, line in zip(range(HEADER_LINES), handle):
                found = DESIGN_RE.match(line)
                if found:
                    return found.group(1)
    except OSError:
        return ""
    return ""


def load_index(root: Path) -> dict[str, list[str]]:
    """Map every indexed source path to the docs that anchor it.

    Returns an EMPTY dict only when the index file is absent. A caller must treat
    that as "could not check", never as "nothing to report".
    """
    index_file = root / INDEX_PATH
    if not index_file.is_file():
        return {}

    mapping: dict[str, list[str]] = {}
    section = ""
    for line in index_file.read_text(encoding="utf-8").splitlines():
        heading = SECTION_RE.match(line)
        if heading:
            section = heading.group(1)
            continue
        row = ROW_RE.match(line)
        if not row or not section:
            continue
        name, docs_cell = row.group(1), row.group(2)
        # The generator emits a row with an empty file name for anchors naming
        # the directory rather than a file in it. It maps to no path.
        if not name or name == "File":
            continue
        docs = DOC_RE.findall(docs_cell)
        if docs:
            mapping[section.rstrip("/") + "/" + name] = docs
    return mapping


def spec_source_files(text: str) -> list[str]:
    """Every source path the spec says it will modify or create."""
    files: list[str] = []
    in_section = False
    for line in text.splitlines():
        if line.startswith("## "):
            in_section = line.startswith("## Files to Modify") or line.startswith(
                "## Files to Create"
            )
            continue
        if line.startswith("### "):
            # `### Integration Checklist` sits under Files to Create and lists
            # docs and rules, which are not code.
            in_section = False
            continue
        if not in_section:
            continue
        bullet = BULLET_RE.match(line)
        if not bullet:
            continue
        path = bullet.group(1)
        if path.startswith(CODE_PREFIX) and path.endswith(CODE_SUFFIX):
            files.append(path)
    return files


def audit(text: str, files: list[str], root: Path, index: dict[str, list[str]]):
    """Split omitted documents by edge strength.

    Returns (owners, mentions): both map a doc path to the source files that tie it
    to this spec, and both list only docs the spec never names.
    """
    owners: dict[str, list[str]] = defaultdict(list)
    mentions: dict[str, list[str]] = defaultdict(list)
    for path in files:
        owner = design_doc(root, path)
        if owner and owner not in text:
            owners[owner].append(path)
        for doc in index.get(path, []):
            if doc not in text and doc != owner:
                mentions[doc].append(path)
    return dict(sorted(owners.items())), dict(sorted(mentions.items()))


def report(stream, title: str, found: dict[str, list[str]], edge: str) -> None:
    """Print one finding group. `edge` names WHICH tie is being reported, because
    "declares" and "mentions" carry different obligations and a shared label would
    read as though a warning were a blocking one."""
    print(title, file=stream)
    for doc, sources in found.items():
        print(f"  {doc}", file=stream)
        print(f"      {edge}: {', '.join(sources)}", file=stream)


def main(argv: list[str]) -> int:
    as_json = "--json" in argv
    args = [a for a in argv if not a.startswith("--")]
    if len(args) != 1:
        print(
            "usage: spec_doc_anchors.py [--json] plan/spec-<name>.md", file=sys.stderr
        )
        return 2

    spec = Path(args[0])
    root = Path.cwd()
    if not spec.is_file():
        print(f"spec not found: {spec}", file=sys.stderr)
        return 2

    text = spec.read_text(encoding="utf-8")
    index = load_index(root)
    if not index:
        # "I could not tell what to check" is not "nothing to report".
        print(
            f"{INDEX_PATH} is absent or empty, so no anchor could be derived. "
            "Run: make ze-doc-index-update",
            file=sys.stderr,
        )
        return 2

    files = spec_source_files(text)
    owners, mentions = audit(text, files, root, index)

    if as_json:
        print(
            json.dumps(
                {"files": files, "owners": owners, "mentions": mentions}, indent=2
            )
        )
        return 1 if owners else 0

    if mentions:
        report(
            sys.stderr,
            f"note: {len(mentions)} document(s) mention this spec's code and are not named:",
            mentions,
            "mentioned by",
        )

    if not owners:
        return 0

    report(
        sys.stderr,
        f"\n{spec}: {len(owners)} design document(s) DECLARED by this spec's own code "
        "are never named in it:",
        owners,
        "declared by",
    )
    print(
        "\n  Each file above carries `// Design: <doc>` in its header, naming that\n"
        "  document as its design. Changing the file without naming the doc is how a\n"
        "  design change ships with its design unwritten.\n"
        "  Name each one: list it under `## Files to Modify`, or record in the\n"
        "  Documentation Update Checklist why it is unaffected.",
        file=sys.stderr,
    )
    return 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
