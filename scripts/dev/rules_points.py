#!/usr/bin/env python3
"""Split a rendered rule into per-point files, and render those files back.

One instruction becomes one checked-in file whose PATH is its id. The rendered
`ai/rules/<rule>.md` is generated from those files, so an agent reads the same
bytes it reads today.

The split is a pure LINE PARTITION. Every non-blank line of a rendered rule
lands in exactly one point body or in the manifest header, and every line that
lands in neither is a blank separator the renderer puts back. Byte-identity
then holds by construction rather than by luck.

Layout, at a FIXED depth of two: a directory per rule, a directory per `##`
section, a file per point.

    ai/rules/points/<rule>/manifest.md            the header and the whole spine
    ai/rules/points/<rule>/<section>/<slug>.md    one point

A `##` heading is a section DIRECTORY rather than a point file. `###` and `####`
headings stay points inside their section, because they are sub-structure within
a section rather than sections themselves; that is what keeps the depth fixed at
two. Every point id is therefore exactly three components, `<rule>/<section>/<slug>`.

A directory name is a slug, so it cannot carry the heading's capitalisation, its
punctuation or its exact text. The manifest carries those, which is why the
manifest is the rule's full structural SPINE and not only its reading order. It
already emitted `title`, `when`, `severity` and `related` verbatim; a section
heading is the same class of content.

Reading order lives in the manifest, not in a numeric filename prefix, so a
reorder never changes a point's id. The cost of that choice is a point on disk
that no manifest lists, and `render` pays it with a hard error.
"""

from __future__ import annotations

import argparse
import ast
import collections
import difflib
import json
import re
import subprocess
import sys
import tempfile
from dataclasses import dataclass, field
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

import rules_lint  # noqa: E402

# This checkout. A point's `rationale:` names a repo-relative path, so it is
# resolved from here rather than from the caller's working directory.
REPO_ROOT = Path(__file__).resolve().parents[2]

# The frontmatter delimiter. Horizontal rules are absent from the corpus, so a
# body line can never be mistaken for the header terminator: the terminator is
# found before the body starts.
DELIM = "---"

# Point kinds. `heading` and `fence` are structural: they exist so a later gate
# can exclude them from the gated/ungated denominator.
KINDS = ("directive", "table", "note", "heading", "fence")

# RFC 2119 levels, in detection precedence order.
LEVELS = ("MUST NOT", "MUST", "SHOULD", "MAY")

HEADER_KEYS = ("title", "when", "severity", "related")
# `rationale` is a repo-relative path to the record of WHY the point exists: a
# `plan/learned/NNNN-*.md` summary, or an `ai/rationale/*.md` file. It is a
# HEADER field, so it never reaches the body and the rendered rule is
# byte-identical whether a point carries one or not.
#
# `excepted-by` names the point or points that carve an exception out of THIS
# one, comma-separated. A general instruction must carry its own exception, or a
# reader who stops after the general statement is misled, and the repetition
# that prevents that is invisible to every gate: a dedup pass can delete the
# exception and leave the general statement standing alone with nothing going
# red. The key is declared on the GENERAL point because the general point is the
# one that misleads, and a dangling ref fails the gate map, so deleting an
# exception can no longer be silent. It is a HEADER field for the same reason
# `rationale` is: it records a relationship the prose already states, and it
# never reaches the body.
POINT_KEYS = ("kind", "level", "stage", "rationale", "excepted-by")

# `excepted-by` holds one or more point ids: one general instruction can have
# several exceptions, and `performance/banned-patterns` is the measured case
# with two.
EXCEPTED_BY = "excepted-by"

H1 = re.compile(r"^# (\S.*)$")
META_LINE = re.compile(r"^\*\*(When|Severity|Related):\*\* (\S.*)$")
FENCE = re.compile(r"^\s*(`{3,}|~{3,})(.*)$")
HEADING = re.compile(r"^#{1,6}\s")
# A `##` heading opens a SECTION. Deeper headings do not: they are sub-structure
# inside a section, and giving them directories too would make the depth vary
# with how a rule happens to be nested.
SECTION_HEADING = re.compile(r"^##\s+\S")
LIST_ITEM = re.compile(r"^\s*(-|\d+[.)])\s+\S")
LEVEL_RE = {
    lv: re.compile(rf"(?<![A-Za-z]){re.escape(lv)}(?![A-Za-z])") for lv in LEVELS
}

SLUG_MAX = 60
SLUG_SAFE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


class RulePointsError(Exception):
    """A malformed manifest, a malformed point, or a lossy partition."""


@dataclass
class Point:
    """One block of a rule, with the source lines it owns."""

    slug: str
    kind: str
    level: str
    stage: str
    body: list[str]
    start: int  # index of the first source line this point owns
    end: int  # index one past the last source line this point owns
    rationale: str = ""  # repo-relative path to why this instruction exists
    excepted_by: str = ""  # comma-separated ids of the points that except this one


@dataclass
class Section:
    """One `##` section: a directory, its exact heading line, and its points.

    `heading` is the source line VERBATIM. `slug` is the directory name derived
    from it, and a slug cannot carry capitalisation, punctuation or the marker,
    so the heading is what the manifest records and the renderer emits.
    """

    slug: str
    heading: str
    start: int  # index of the heading line in the source
    points: list[Point] = field(default_factory=list)


@dataclass
class Split:
    """The result of splitting one rendered rule."""

    stem: str
    header: dict[str, str]
    header_start: int
    header_end: int
    sections: list[Section] = field(default_factory=list)
    line_count: int = 0

    @property
    def points(self) -> list[Point]:
        """Every point, in reading order, across every section."""
        return [p for section in self.sections for p in section.points]

    @property
    def ids(self) -> list[str]:
        """Every point id this split would write, `<rule>/<section>/<slug>`."""
        return [
            f"{self.stem}/{section.slug}/{point.slug}"
            for section in self.sections
            for point in section.points
        ]


def rule_files(rules_dir: Path) -> list[Path]:
    """The rule corpus: `ai/rules/*.md` minus the generated artifacts.

    The predicate is the one `rule_coverage.load_rules`, `rules_lint.main` and
    `rules_condensed.is_artifact` already agree on: `rules_lint.SKIP` plus an
    all-caps stem. A fourth spelling of "which files are rules" would drift from
    those three, and a `points/` subdirectory stays invisible to all of them
    because every one of these globs is non-recursive.
    """
    return [
        p
        for p in sorted(rules_dir.glob("*.md"))
        if not (p.name in rules_lint.SKIP or p.stem.isupper())
    ]


def _body_lines(text: str, what: str) -> list[str]:
    """Split file text into lines, requiring exactly one trailing newline.

    Strictness is deliberate. A file that ends with no newline, or with a blank
    line, cannot round-trip through a renderer that always emits exactly one
    trailing newline, and the corpus carries neither shape. Guessing here would
    turn a measurable property into a silent normalization.
    """
    if not text.endswith("\n"):
        raise RulePointsError(f"{what}: must end with a newline")
    if text.endswith("\n\n"):
        raise RulePointsError(f"{what}: must not end with a blank line")
    return text.split("\n")[:-1]


def block_ranges(lines: list[str], start: int) -> list[tuple[int, int]]:
    """The maximal runs of non-blank lines from `start`, as half-open ranges.

    A run ends at a blank line, EXCEPT inside a fenced block: the corpus carries
    66 blank lines inside fences, so a walker without fence state would cut a
    fence in half. A fence closes only on the marker character it opened with,
    so a nested fence of the other character does not close it.

    Nothing splits a run. That is what makes the renderer's "join with one blank
    line" correct without a recorded separator (A-1): two blocks are separated by
    a blank line by construction, and content with no blank line between it is
    one block rather than two blocks with a zero-width gap.
    """
    ranges = []
    i = start
    while i < len(lines):
        if not lines[i].strip():
            i += 1
            continue
        block_start = i
        marker = None
        while i < len(lines):
            line = lines[i]
            fence = FENCE.match(line)
            if marker is None:
                if fence:
                    marker = fence.group(1)[0]
                    i += 1
                    continue
                if not line.strip():
                    break
                i += 1
                continue
            i += 1
            if fence and fence.group(1)[0] == marker and not fence.group(2).strip():
                marker = None
        ranges.append((block_start, i))
    return ranges


def classify(lines: list[str]) -> str:
    """The point kind of a block, decided by its first line."""
    first = lines[0]
    if FENCE.match(first):
        return "fence"
    if HEADING.match(first):
        return "heading"
    if first.lstrip().startswith("|"):
        return "table"
    if LIST_ITEM.match(first) or first.startswith("**"):
        return "directive"
    return "note"


def level_of(lines: list[str]) -> str:
    """The strongest RFC 2119 level a block states, or the empty string."""
    text = "\n".join(lines)
    for level in LEVELS:
        if LEVEL_RE[level].search(text):
            return level
    return ""


def slugify(lines: list[str], kind: str) -> str:
    """A path-safe id derived from a block's first meaningful line."""
    source = lines[0]
    if kind == "fence":
        info = FENCE.match(source).group(2).strip()
        inner = next((ln for ln in lines[1:] if ln.strip()), "")
        source = f"{info} {inner}" if info else inner
    text = re.sub(r"<!--.*?-->", " ", source)
    text = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", text)  # links keep their text
    text = text.replace("`", " ").replace("*", " ").replace("_", " ")
    slug = re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")
    if len(slug) > SLUG_MAX:
        slug = slug[:SLUG_MAX]
        cut = slug.rfind("-")
        if cut > 0:
            slug = slug[:cut]
        slug = slug.strip("-")
    return slug or kind


def _unique(slug: str, taken: set[str]) -> str:
    """`slug`, or the first `slug-N` free in `taken`. Registers the result."""
    candidate = slug
    n = 1
    while candidate in taken:
        n += 1
        candidate = f"{slug}-{n}"
    taken.add(candidate)
    return candidate


def _parse_header(lines: list[str], stem: str) -> tuple[dict[str, str], int]:
    """Read the H1 and the metadata block. Return the header and its end index.

    The end index is one past the last metadata line, so the blank line that
    follows is an ordinary block separator rather than header content.
    """
    if not lines:
        raise RulePointsError(f"{stem}: file is empty")
    title = H1.match(lines[0])
    if not title:
        raise RulePointsError(f"{stem}: first line must be '# Title'")
    if len(lines) < 2 or lines[1].strip():
        raise RulePointsError(f"{stem}: one blank line must follow the title")

    header = {"title": title.group(1)}
    idx = 2
    seen = []
    while idx < len(lines):
        meta = META_LINE.match(lines[idx])
        if not meta:
            break
        header[meta.group(1).lower()] = meta.group(2)
        seen.append(meta.group(1))
        idx += 1

    canon = [k for k in rules_lint.CANON_KEYS if k in seen]
    if seen != canon or "When" not in seen or "Severity" not in seen:
        raise RulePointsError(
            f"{stem}: metadata must be When, Severity, then optional Related "
            f"(found {seen or 'nothing'})"
        )
    if idx >= len(lines) or lines[idx].strip():
        raise RulePointsError(f"{stem}: one blank line must follow the metadata")
    return header, idx


def _verify_partition(lines: list[str], split: Split) -> None:
    """Fail closed when the split is not a total partition of the source lines.

    The round trip alone cannot see this: a splitter that drops a line and a
    renderer that puts an equivalent one back would still compare equal. This
    checks the property the design rests on rather than its visible effect.
    """
    owner: list[str | None] = [None] * len(lines)
    claims = [("header", split.header_start, split.header_end)]
    for section in split.sections:
        # A section owns exactly its heading line. The manifest holds that line,
        # so it is claimed here or it would read as owned by nothing.
        claims.append((f"{section.slug}/", section.start, section.start + 1))
        claims += [(p.slug, p.start, p.end) for p in section.points]
    for name, start, end in claims:
        for i in range(start, end):
            if owner[i] is not None:
                raise RulePointsError(
                    f"{split.stem}: line {i + 1} claimed by {owner[i]} and {name}"
                )
            owner[i] = name
    for i, who in enumerate(owner):
        if who is None and lines[i].strip():
            raise RulePointsError(
                f"{split.stem}: line {i + 1} is non-blank and belongs to no point"
            )


def split_rule(text: str, stem: str) -> Split:
    """Partition one rendered rule into a header and an ordered list of points."""
    lines = _body_lines(text, stem)
    header, header_end = _parse_header(lines, stem)

    split = Split(
        stem=stem,
        header=header,
        header_start=0,
        header_end=header_end,
        line_count=len(lines),
    )

    sections_taken: set[str] = set()
    taken: set[str] = set()
    previous_end = header_end
    for start, end in block_ranges(lines, header_end):
        gap = start - previous_end
        if gap != 1:
            raise RulePointsError(
                f"{stem}: line {start + 1} follows {gap} blank lines, not 1; the "
                "renderer joins blocks with exactly one"
            )
        body = lines[start:end]
        previous_end = end

        if SECTION_HEADING.match(body[0]):
            if len(body) != 1:
                raise RulePointsError(
                    f"{stem}: line {start + 1} opens a `##` section and carries "
                    f"{len(body) - 1} more line(s) with no blank line between; a "
                    "section heading names a directory and must stand alone"
                )
            # Point slugs are unique WITHIN a section, because the id carries the
            # section. Section slugs are unique within the rule.
            taken = set()
            split.sections.append(
                Section(
                    slug=_unique(slugify(body, "heading"), sections_taken),
                    heading=body[0],
                    start=start,
                )
            )
            continue

        if not split.sections:
            raise RulePointsError(
                f"{stem}: line {start + 1} comes before the first `##` section; "
                "every point must live in a section directory"
            )
        kind = classify(body)
        split.sections[-1].points.append(
            Point(
                slug=_unique(slugify(body, kind), taken),
                kind=kind,
                level=level_of(body),
                stage="",
                body=body,
                start=start,
                end=end,
            )
        )

    if previous_end != len(lines):
        raise RulePointsError(f"{stem}: file ends with a blank line")
    if not split.sections:
        raise RulePointsError(f"{stem}: carries a header but no `##` section")
    for section in split.sections:
        if not section.points:
            raise RulePointsError(
                f"{stem}: section {section.slug!r} holds no point; an empty "
                "directory carries no instruction and does not survive a clone"
            )

    _verify_partition(lines, split)
    return split


def render_text(header: dict[str, str], sections: list[Section]) -> str:
    """Assemble rule text from a header and ordered sections.

    A section emits its heading line from the MANIFEST, verbatim, then its
    points' bodies. Every block is separated by exactly one blank line, which is
    the property `block_ranges` and `split_rule` between them guarantee (A-1).
    """
    out = [f"# {header['title']}", "", f"**When:** {header['when']}"]
    out.append(f"**Severity:** {header['severity']}")
    if header.get("related"):
        out.append(f"**Related:** {header['related']}")
    for section in sections:
        out += ["", section.heading]
        for point in section.points:
            out.append("")
            out.extend(point.body)
    return "\n".join(out) + "\n"


def _frontmatter(text: str, what: str) -> tuple[dict[str, str], list[str]]:
    """Split a point or manifest file into its header fields and its body.

    Line 1 is the delimiter and the header ends at the next line that is exactly
    the delimiter. Everything after that line is body, verbatim and unparsed, so
    a body whose own first line is the delimiter round-trips (AC-5): the
    terminator is always found before the body begins.
    """
    lines = _body_lines(text, what)
    if not lines or lines[0] != DELIM:
        raise RulePointsError(f"{what}: first line must be '{DELIM}'")
    try:
        end = lines.index(DELIM, 1)
    except ValueError:
        raise RulePointsError(
            f"{what}: header is not terminated by '{DELIM}'"
        ) from None

    fields = {}
    for line in lines[1:end]:
        key, sep, value = line.partition(":")
        if not sep or not key.strip():
            raise RulePointsError(f"{what}: header line is not 'key: value': {line!r}")
        fields[key.strip()] = value[1:] if value.startswith(" ") else value
    return fields, lines[end + 1 :]


def format_point(point: Point) -> str:
    """One point as a file: a frontmatter header, then the body verbatim.

    `kind`, `level` and `stage` are always written, empty value included: the
    split derives all three, so a missing line would mean a lossy write.
    `rationale` and `excepted-by` are written ONLY when they carry a value, the
    way `related` is handled in `format_manifest`. The split cannot derive
    either one -- an author links the record and the exception by hand -- so an
    empty line would say "this point was examined and has none" when the truth
    is "nobody has linked one".
    """
    head = [DELIM]
    for key, value in (
        ("kind", point.kind),
        ("level", point.level),
        ("stage", point.stage),
    ):
        head.append(f"{key}: {value}" if value else f"{key}:")
    if point.rationale:
        head.append(f"rationale: {point.rationale}")
    if point.excepted_by:
        head.append(f"{EXCEPTED_BY}: {point.excepted_by}")
    head.append(DELIM)
    return "\n".join(head + point.body) + "\n"


def parse_point(text: str, slug: str) -> Point:
    """Read one point file. The inverse of `format_point`."""
    fields, body = _frontmatter(text, slug)
    unknown = sorted(set(fields) - set(POINT_KEYS))
    if unknown:
        raise RulePointsError(f"{slug}: unknown header field(s) {unknown}")
    kind = fields.get("kind", "")
    level = fields.get("level", "")
    if kind not in KINDS:
        raise RulePointsError(
            f"{slug}: kind must be one of {list(KINDS)}, got {kind!r}"
        )
    if level and level not in LEVELS:
        raise RulePointsError(
            f"{slug}: level must be empty or one of {list(LEVELS)}, got {level!r}"
        )
    if not body:
        raise RulePointsError(f"{slug}: has an empty body")
    return Point(
        slug=slug,
        kind=kind,
        level=level,
        stage=fields.get("stage", ""),
        body=body,
        start=0,
        end=len(body),
        rationale=fields.get("rationale", "").strip(),
        excepted_by=fields.get(EXCEPTED_BY, "").strip(),
    )


def exception_refs(raw: str) -> list[str]:
    """The point ids an `excepted-by` value names, in the order it names them.

    Comma-separated, because one general instruction can carry several
    exceptions. An empty element is dropped rather than kept as the empty ref:
    a trailing comma is a typo in the separator, not a claim about a point, and
    keeping it would fail the gate with a message naming nothing.
    """
    return [ref.strip() for ref in raw.split(",") if ref.strip()]


# The manifest body is the rule's SPINE, two line shapes and nothing else:
#
#   <section-dir> ## The Exact Heading Line
#     <point-slug>
#
# A section line names the directory, then carries the heading VERBATIM after
# one space. A directory slug can hold no space, so the split is unambiguous and
# the heading is recovered byte-exactly however it is capitalised or punctuated.
# A point line is indented by two spaces, which no slug can start with, so the
# two shapes cannot be confused and anything matching neither is an error.
MANIFEST_SECTION = re.compile(r"^(?P<slug>\S+) (?P<heading>##\s+\S.*)$")
MANIFEST_POINT = re.compile(r"^ {2}(?P<slug>\S.*)$")
POINT_INDENT = "  "


def format_manifest(split: Split) -> str:
    """The manifest: the rule's spine in the header, the whole tree as body."""
    head = [DELIM, f"title: {split.header['title']}", f"when: {split.header['when']}"]
    head.append(f"severity: {split.header['severity']}")
    if split.header.get("related"):
        head.append(f"related: {split.header['related']}")
    head.append(DELIM)
    body: list[str] = []
    for section in split.sections:
        body.append(f"{section.slug} {section.heading}")
        body += [f"{POINT_INDENT}{p.slug}" for p in section.points]
    return "\n".join(head + body) + "\n"


def parse_manifest(
    text: str, stem: str
) -> tuple[dict[str, str], list[tuple[str, str, list[str]]]]:
    """Read a manifest into its header fields and its ordered section tree.

    Each section is `(directory slug, heading line, ordered point slugs)`. A body
    line that is neither shape is an error rather than a skip: a skipped line is
    an instruction that stops being rendered with nothing going red, which is the
    one failure this design exists to prevent (R-4).
    """
    fields, body = _frontmatter(text, f"{stem}/manifest.md")
    unknown = sorted(set(fields) - set(HEADER_KEYS))
    if unknown:
        raise RulePointsError(f"{stem}/manifest.md: unknown field(s) {unknown}")
    for required in ("title", "when", "severity"):
        if not fields.get(required):
            raise RulePointsError(f"{stem}/manifest.md: missing '{required}'")

    sections: list[tuple[str, str, list[str]]] = []
    for number, line in enumerate(body, 1):
        point = MANIFEST_POINT.match(line)
        if point:
            if not sections:
                raise RulePointsError(
                    f"{stem}/manifest.md:{number}: point slug "
                    f"{point.group('slug')!r} comes before any section line; every "
                    "point lives in a section directory"
                )
            sections[-1][2].append(point.group("slug"))
            continue
        section = MANIFEST_SECTION.match(line)
        if not section:
            raise RulePointsError(
                f"{stem}/manifest.md:{number}: {line!r} is neither a section line "
                "('<dir-slug> ## Heading') nor an indented point slug"
            )
        sections.append((section.group("slug"), section.group("heading"), []))

    if not sections:
        raise RulePointsError(f"{stem}/manifest.md: lists no sections")
    return fields, sections


def _refuse_stale(stem: str, where: Path, stale: list[str]) -> None:
    """Report files this split does not produce. Deleting them is never the move.

    Deleting would destroy an author's file on a slug or a section rename, so
    reporting is the only safe answer (`ai/rules/never-destroy-work.md`).
    """
    if stale:
        raise RulePointsError(
            f"{stem}: {where} already holds {sorted(stale)}, which this split "
            "does not produce; remove them by hand or split into a clean directory"
        )


def write_split(split: Split, out_dir: Path) -> None:
    """Write the manifest, then one file per point under `<stem>/<section>/`."""
    rule_dir = out_dir / split.stem
    rule_dir.mkdir(parents=True, exist_ok=True)

    want = {section.slug for section in split.sections}
    _refuse_stale(
        split.stem,
        rule_dir,
        [p.name for p in rule_dir.glob("*.md") if p.name != "manifest.md"]
        + [d.name for d in rule_dir.iterdir() if d.is_dir() and d.name not in want],
    )

    (rule_dir / "manifest.md").write_text(format_manifest(split), encoding="utf-8")
    for section in split.sections:
        section_dir = rule_dir / section.slug
        section_dir.mkdir(exist_ok=True)
        written = {f"{p.slug}.md" for p in section.points}
        _refuse_stale(
            split.stem,
            section_dir,
            [p.name for p in section_dir.glob("*.md") if p.name not in written],
        )
        for point in section.points:
            (section_dir / f"{point.slug}.md").write_text(
                format_point(point), encoding="utf-8"
            )


def _safe_slug(stem: str, slug: str, what: str) -> None:
    """Refuse any slug that is not a bare lowercase path component.

    Security: a manifest names directories and files the renderer opens, so a
    separator, a leading dot or a parent reference would let it read outside its
    own rule directory.
    """
    if not SLUG_SAFE.match(slug):
        raise RulePointsError(
            f"{stem}: {what} slug {slug!r} must be a bare lowercase path "
            "component; a separator, a leading dot or a parent reference is refused"
        )


def render_dir(rule_dir: Path) -> str:
    """Render the rule text from a point directory on disk.

    Fails hard on an unlisted section, an unlisted point, a listed slug with no
    file or directory, a duplicate slug, a point file sitting outside every
    section directory, and a slug that is not a bare path component. Each of
    those is a way for the rendered rule to silently lose an instruction, which
    is the one failure this whole design exists to prevent (R-4). Never render
    partially.
    """
    stem = rule_dir.name
    manifest_path = rule_dir / "manifest.md"
    if not manifest_path.is_file():
        raise RulePointsError(f"{stem}: no manifest at {manifest_path}")
    header, listed = parse_manifest(manifest_path.read_text(encoding="utf-8"), stem)

    loose = sorted(p.name for p in rule_dir.glob("*.md") if p.name != "manifest.md")
    if loose:
        raise RulePointsError(
            f"{stem}: {loose} sit(s) directly in the rule directory; every point "
            "lives in a `##` section directory, so the id is always "
            "<rule>/<section>/<slug>. Move it into its section"
        )

    seen_sections: set[str] = set()
    for slug, _heading, _slugs in listed:
        _safe_slug(stem, slug, "section")
        if slug in seen_sections:
            raise RulePointsError(f"{stem}: duplicate section slug {slug!r}")
        seen_sections.add(slug)

    on_disk = {d.name for d in rule_dir.iterdir() if d.is_dir()}
    unlisted = sorted(on_disk - seen_sections)
    if unlisted:
        raise RulePointsError(
            f"{stem}: section directory/ies {unlisted} exist but the manifest "
            "does not list them; add them to the reading order or delete them"
        )

    sections: list[Section] = []
    for slug, heading, point_slugs in listed:
        section_dir = rule_dir / slug
        if not section_dir.is_dir():
            raise RulePointsError(
                f"{stem}: manifest lists section {slug!r} with no directory on disk"
            )
        seen: set[str] = set()
        for point_slug in point_slugs:
            _safe_slug(stem, point_slug, "point")
            if point_slug in seen:
                raise RulePointsError(
                    f"{stem}/{slug}: duplicate slug {point_slug!r} in the manifest"
                )
            seen.add(point_slug)

        here = {p.stem for p in section_dir.glob("*.md")}
        extra = sorted(here - seen)
        if extra:
            raise RulePointsError(
                f"{stem}/{slug}: point file(s) {extra} exist but the manifest does "
                "not list them; add them to the reading order or delete them"
            )
        missing = [s for s in point_slugs if not (section_dir / f"{s}.md").is_file()]
        if missing:
            raise RulePointsError(
                f"{stem}/{slug}: manifest lists {missing} with no file on disk"
            )
        if not point_slugs:
            raise RulePointsError(
                f"{stem}: section {slug!r} lists no point; a section directory "
                "with nothing in it carries no instruction"
            )
        sections.append(
            Section(
                slug=slug,
                heading=heading,
                start=0,
                points=[
                    parse_point(
                        (section_dir / f"{s}.md").read_text(encoding="utf-8"), s
                    )
                    for s in point_slugs
                ],
            )
        )
    return render_text(header, sections)


def point_dirs(points_dir: Path) -> list[Path]:
    """Every rule's point directory, sorted. A directory is one iff it has a manifest."""
    if not points_dir.is_dir():
        return []
    return sorted(d for d in points_dir.iterdir() if (d / "manifest.md").is_file())


def _drift(stem: str, want: str, have: str) -> str:
    """A drift report for one rule: the first lines of a unified diff."""
    diff = list(
        difflib.unified_diff(
            have.splitlines(),
            want.splitlines(),
            fromfile=f"a/{stem}.md (on disk)",
            tofile=f"b/{stem}.md (rendered)",
            lineterm="",
        )
    )[:24]
    return f"{stem}.md is stale\n" + "\n".join(diff)


def render_all(rules_dir: Path, points_dir: Path, *, check: bool) -> list[str]:
    """Render every point directory to `ai/rules/<stem>.md`. Return failure lines.

    Fails closed in four directions, because each one is a way for an
    instruction to stop being generated without anything going red:
    an absent or empty `points/` tree is an error rather than a vacuous pass; a
    rule file with no point directory is an error, since nothing would render it
    and the edit hook would still refuse edits to it; a point directory named
    for one of the generated artifacts beside the rules is an error, because
    rendering it would overwrite a file a different generator owns; and in
    `--check` mode any byte of drift is a failure rather than a silent rewrite.
    """
    dirs = point_dirs(points_dir)
    if not dirs:
        return [
            f"no rule point directories under {points_dir}; "
            "the render has nothing to read and must not report success"
        ]

    failures = []
    have_points = {d.name for d in dirs}
    for path in rule_files(rules_dir):
        if path.stem not in have_points:
            failures.append(
                f"{path.name}: no point directory at {points_dir / path.stem}; "
                "every rendered rule must be generated from points"
            )

    for rule_dir in dirs:
        stem = rule_dir.name
        # `point_dirs` accepts any directory carrying a manifest, while
        # `rule_files` excludes an all-caps stem and everything in
        # `rules_lint.SKIP`. Without this the two disagree, and the render's
        # target for a `points/CORE/` directory would be `ai/rules/CORE.md` --
        # a file `rules_condensed.py` owns. The read side already protects
        # those names; this is the write side of the same predicate.
        if stem.isupper() or f"{stem}.md" in rules_lint.SKIP:
            failures.append(
                f"{stem}: a point directory may not be named for a generated "
                f"artifact; rendering it would overwrite ai/rules/{stem}.md, "
                "which another generator owns. Rename the directory"
            )
            continue
        target = rules_dir / f"{stem}.md"
        try:
            rendered = render_dir(rule_dir)
        except RulePointsError as err:
            failures.append(f"{stem}: {err}")
            continue
        current = target.read_text(encoding="utf-8") if target.is_file() else None
        if current == rendered:
            continue
        if check:
            if current is None:
                failures.append(f"{stem}.md does not exist but its points do")
            else:
                failures.append(_drift(stem, rendered, current))
            continue
        target.write_text(rendered, encoding="utf-8")
    return failures


def roundtrip(rules_dir: Path, out_dir: Path) -> list[str]:
    """Split then render every rule. Return one failure line per mismatch."""
    failures = []
    for path in rule_files(rules_dir):
        source = path.read_text(encoding="utf-8")
        try:
            split = split_rule(source, path.stem)
            write_split(split, out_dir)
            rendered = render_dir(out_dir / path.stem)
        except (RulePointsError, NotImplementedError) as err:
            failures.append(f"{path.name}: {type(err).__name__}: {err}")
            continue
        if rendered != source:
            diff = "\n".join(
                list(
                    difflib.unified_diff(
                        source.splitlines(),
                        rendered.splitlines(),
                        fromfile=f"a/{path.name}",
                        tofile=f"b/{path.name}",
                        lineterm="",
                    )
                )[:24]
            )
            failures.append(f"{path.name}: round trip is not byte-identical\n{diff}")
    return failures


# --------------------------------------------------------------------------
# Gate map: which point each hook check enforces
# --------------------------------------------------------------------------

# The PreToolUse dispatchers. A check is a top-level function in one of them,
# and a binding comment sits directly above that function.
#
# The roster is DERIVED from `.claude/settings.json`, never typed here. A fixed
# tuple can be SHRUNK with nothing going red: dropping `pretool-bash.py` from it
# retired seven checks from both the binding join and the published-table check,
# and every gate stayed green. settings.json is what makes a dispatcher RUN, so
# it is the only honest source for which ones exist.
#
# A `.sh` PreToolUse command is excluded by construction: it carries no `def`,
# so neither a check nor a binding comment can live in one. `block-until-lsp.sh`
# is the only one today.
SETTINGS = ".claude/settings.json"
HOOK_DIR = ".claude/hooks"
DISPATCHER_GLOB = "pretool-*.py"

# `# ze point: <rule>/<section>/<slug>` on a line of its own, directly above the
# check it binds. The ref is the point's PATH under `ai/rules/points/` without
# the extension, which is its id (D2). It is always three components: the rule,
# the `##` section directory, and the point file.
#
# The payload is captured whole rather than as one bare token, so a typo lands
# in the dangling set instead of matching nothing and vanishing. No point id
# carries a space, so a malformed payload cannot accidentally resolve.
BINDING = re.compile(r"^\s*#\s*ze point:(?P<payload>.*)$")
# `# ze point: none -- <why>` declares that a check enforces no written point.
# The reason is REQUIRED: without it, "nobody bound this yet" and "there is
# nothing to bind" would look the same, and the second would absorb the first.
NO_POINT = re.compile(r"^none\s+--\s+(?P<why>\S.*)$")
EMPTY_REF = "!empty"
TOP_LEVEL_DEF = re.compile(r"^def\s+([A-Za-z_]\w*)\s*\(")

# A binding attributed to nothing that runs. Reported, never dropped: a binding
# the join cannot attribute is the same defect class as one naming no point.
NO_CHECK = "<module>"

# `heading` and `fence` are structural. A heading names a section and a fence
# quotes code or output; neither states an instruction a check could enforce.
# They are excluded from the ungated DENOMINATOR, so the number means "points
# stating something nothing gates" rather than "markdown blocks".
STRUCTURAL_KINDS = ("heading", "fence")


@dataclass(frozen=True)
class Binding:
    """One `# ze point:` comment: where it is, what carries it, what it names."""

    file: str
    line: int
    check: str
    ref: str  # the point id, or "" when the check declares it binds none
    reason: str = ""  # why the check binds no point; empty for a real ref


@dataclass
class GateMap:
    """The join between the bindings in the dispatchers and the points on disk."""

    points: dict[str, str]  # ref -> kind, every point on disk
    bindings: list[Binding]
    gated: dict[str, list[Binding]]  # ref -> the bindings naming it
    ungated: list[str]  # instruction points no binding names
    dangling: list[Binding]  # a binding naming a point that does not exist
    unbound: list[Binding]  # a check declaring `none -- <why>`
    # ref -> the repo-relative path its `rationale:` names. Only the points that
    # declare one appear, so `len(rationales)` is the AC-17 measurement.
    rationales: dict[str, str] = field(default_factory=dict)
    # (ref, path) for a rationale naming a path absent from disk. This is the
    # AC-16 failing set, and it fails for the same reason `dangling` does.
    missing_rationale: list[tuple[str, str]] = field(default_factory=list)
    # ref -> the point ids its `excepted-by` names. Only the points that declare
    # one appear, so `len(excepted)` is the AC-21 measurement.
    excepted: dict[str, list[str]] = field(default_factory=dict)
    # (ref, named ref) for an `excepted-by` naming a point that is not on disk,
    # or naming the declaring point itself. This is the AC-20 failing set.
    missing_excepted_by: list[tuple[str, str]] = field(default_factory=list)

    @property
    def candidates(self) -> list[str]:
        """The ungated denominator: every point that is not structural."""
        return [r for r, k in self.points.items() if k not in STRUCTURAL_KINDS]

    @property
    def gated_bindings(self) -> list[Binding]:
        """Every binding that resolved to a point."""
        return [b for group in self.gated.values() for b in group]


def parse_bindings(text: str, path: str) -> list[Binding]:
    """Every binding comment in one dispatcher, attributed to the check below it.

    A binding binds the NEXT top-level `def`, and only blank lines and other
    comments may sit between the two. Anything else has come between the comment
    and its check, so the binding is attributed to `NO_CHECK` and reported rather
    than silently dropped: a comment that names a point but gates nothing is a
    claim, and claims are what this whole design removes.

    A payload that is neither a real point id nor the `none -- <why>` form is
    kept as the ref it spells, which no point matches, so it fails as dangling.
    Dropping it would let a typo un-gate a check with nothing going red.
    """
    lines = text.split("\n")
    pending: list[tuple[int, str, str]] = []
    out: list[Binding] = []

    def flush(check: str) -> None:
        nonlocal pending
        out.extend(Binding(path, n, check, ref, why) for n, ref, why in pending)
        pending = []

    for i, line in enumerate(lines, 1):
        match = BINDING.match(line)
        if match:
            payload = match.group("payload").strip()
            declared = NO_POINT.match(payload)
            if declared:
                pending.append((i, "", declared.group("why")))
            else:
                pending.append((i, payload or EMPTY_REF, ""))
            continue
        if not pending:
            continue
        found = TOP_LEVEL_DEF.match(line)
        if found:
            flush(found.group(1))
        elif line.strip() and not line.lstrip().startswith("#"):
            flush(NO_CHECK)
    flush(NO_CHECK)
    return out


def points_on_disk(points_dir: Path) -> dict[str, Point]:
    """Every point's `<rule>/<section>/<slug>` id mapped to the point it names.

    Read from the FILES rather than from the manifests, because the id is the
    path and a binding names a path. A malformed point raises instead of being
    skipped: skipping one would drop it from both the gated and the ungated set,
    and a point missing from every set reads as a smaller problem than it is.
    """
    out: dict[str, Point] = {}
    for rule_dir in point_dirs(points_dir):
        for section_dir in sorted(d for d in rule_dir.iterdir() if d.is_dir()):
            for path in sorted(section_dir.glob("*.md")):
                point = parse_point(path.read_text(encoding="utf-8"), path.stem)
                out[f"{rule_dir.name}/{section_dir.name}/{path.stem}"] = point
    return out


def dispatchers(root: Path) -> tuple[list[Path], list[str]]:
    """Every PreToolUse Python dispatcher, and the problems found deriving them.

    Fails closed in both directions, because each one moves a check out of the
    join while the report still says "no dangling bindings":

    - a command registered in settings.json with no file on disk takes every
      binding in that file out of the map, and the map then looks greener;
    - a `pretool-*.py` on disk that no PreToolUse entry runs is a dispatcher
      whose checks never fire, and publishing it would claim a gate that is not
      wired.

    An unreadable settings.json is a failure rather than an empty roster: an
    empty roster reads as "no dispatchers exist", which is never true here.
    """
    problems: list[str] = []
    try:
        data = json.loads((root / SETTINGS).read_text(encoding="utf-8"))
    except (OSError, ValueError) as err:
        return [], [
            f"{SETTINGS}: cannot be read ({err}); the dispatcher roster is unknown"
        ]

    registered: list[str] = []
    for entry in (data.get("hooks") or {}).get("PreToolUse") or []:
        for hook in entry.get("hooks") or []:
            command = str(hook.get("command") or "")
            name = command.rsplit("/", 1)[-1]
            if not name.endswith(".py") or f"{HOOK_DIR}/{name}" not in command:
                continue
            if name not in registered:
                registered.append(name)

    if not registered:
        problems.append(
            f"{SETTINGS}: no PreToolUse entry runs a {HOOK_DIR}/*.py dispatcher; "
            "the gate map would read nothing and must not report success"
        )

    paths: list[Path] = []
    for name in sorted(registered):
        path = root / HOOK_DIR / name
        if not path.is_file():
            problems.append(
                f"{SETTINGS}: PreToolUse runs {HOOK_DIR}/{name}, which does not "
                "exist; every binding in it is out of the join"
            )
            continue
        paths.append(path)

    for path in sorted((root / HOOK_DIR).glob(DISPATCHER_GLOB)):
        if path.name not in registered:
            problems.append(
                f"{HOOK_DIR}/{path.name}: no PreToolUse entry in {SETTINGS} runs "
                "it, so its checks never fire; wire it up or delete it"
            )
    return paths, problems


def head_sources(root: Path, names: list[str]) -> dict[str, str] | None:
    """Each named dispatcher's text at git HEAD, or None when git cannot answer.

    A file absent at HEAD is a dispatcher added by this change and is simply
    left out: it has no baseline yet, which is not the same as having lost one.
    """
    out: dict[str, str] = {}
    for name in names:
        try:
            res = subprocess.run(
                ["git", "show", f"HEAD:{HOOK_DIR}/{name}"],
                cwd=root,
                capture_output=True,
                text=True,
                check=False,
            )
        except OSError:
            return None
        if res.returncode == 0:
            out[name] = res.stdout
    return out


def gated_at_head(sources: dict[str, str]) -> set[str]:
    """The point ids the dispatchers' binding comments named at HEAD."""
    refs: set[str] = set()
    for name, text in sources.items():
        for binding in parse_bindings(text, name):
            if binding.ref and binding.ref != EMPTY_REF and binding.check != NO_CHECK:
                refs.add(binding.ref)
    return refs


def gated_regressions(gm: GateMap, baseline: set[str]) -> list[str]:
    """Points that were gated at HEAD and that no binding names now.

    The gated set is MONOTONIC, following `check_coverage_ratchet` in
    scripts/dev/rfc_requirements.py. Deleting a `# ze point:` comment alone is
    already caught, because `hook_table_problems` then sees the published
    `Enforces` cell disagree with the bindings. Deleting the comment AND the
    backticked stem from that row leaves both sides agreeing on nothing: the
    point moves from gated to ungated and every gate exits 0. That is the
    cheapest route from red to green, and it defeats the whole reason the id is
    a path rather than a file name.

    A point that no longer EXISTS is out of scope. Its instruction left the
    corpus, which is a rule-content diff a reader sees in review, not a gate
    quietly dropped from under text that stayed.
    """
    return sorted(ref for ref in baseline if ref in gm.points and ref not in gm.gated)


def rationale_problems(
    points: dict[str, Point], root: Path
) -> tuple[dict[str, str], list[tuple[str, str]]]:
    """The declared rationale links, and the ones naming no file.

    A rationale is a repo-relative path, so it is resolved against the
    repository root and never against the point's own directory. A path outside
    the tree, or one that traverses out of it, is reported as missing rather
    than resolved: the only records this field may name are inside the repo
    (`plan/learned/`, `ai/rationale/`), and a resolver that reaches outside
    would let a link pass a gate nobody can re-check.
    """
    declared: dict[str, str] = {}
    missing: list[tuple[str, str]] = []
    root = root.resolve()
    for ref, point in points.items():
        if not point.rationale:
            continue
        declared[ref] = point.rationale
        target = (root / point.rationale).resolve()
        inside = target == root or root in target.parents
        if not inside or not target.is_file():
            missing.append((ref, point.rationale))
    return declared, missing


def exception_problems(
    points: dict[str, Point],
) -> tuple[dict[str, list[str]], list[tuple[str, str]]]:
    """The declared exception links, and the ones naming no point.

    A ref is a point id, joined against the points on disk exactly as a
    `# ze point:` binding is. Two shapes fail:

    - a ref no point on disk carries. That is the protection this key buys.
      Deleting an exception point leaves the general point's link naming
      nothing, so a dedup or compression pass can no longer remove a guard and
      keep every gate green.
    - a point naming ITSELF. A point cannot carve an exception out of its own
      statement, and a self-reference would otherwise resolve and read as a
      declared relationship.
    """
    declared: dict[str, list[str]] = {}
    missing: list[tuple[str, str]] = []
    for ref, point in points.items():
        if not point.excepted_by:
            continue
        named = exception_refs(point.excepted_by)
        if not named:
            missing.append((ref, point.excepted_by))
            continue
        declared[ref] = named
        missing += [(ref, t) for t in named if t == ref or t not in points]
    return declared, missing


def gate_map(
    gate_files: list[Path], points_dir: Path, root: Path | None = None
) -> GateMap:
    """Join the dispatchers' binding comments against the points on disk.

    `root` is where a point's `rationale:` path is resolved from. It defaults to
    this checkout, which is what the gate reads in production; a test passes its
    own tree so the fixture is hermetic.
    """
    on_disk = points_on_disk(points_dir)
    rationales, missing_rationale = rationale_problems(
        on_disk, REPO_ROOT if root is None else root
    )
    excepted, missing_excepted_by = exception_problems(on_disk)
    points = {ref: point.kind for ref, point in on_disk.items()}
    bindings: list[Binding] = []
    for path in gate_files:
        bindings += parse_bindings(path.read_text(encoding="utf-8"), path.name)

    gated: dict[str, list[Binding]] = {}
    dangling: list[Binding] = []
    unbound: list[Binding] = []
    for binding in bindings:
        if binding.check == NO_CHECK:
            dangling.append(binding)
        elif not binding.ref:
            unbound.append(binding)
        elif binding.ref in points:
            gated.setdefault(binding.ref, []).append(binding)
        else:
            dangling.append(binding)

    ungated = [
        ref
        for ref, kind in points.items()
        if ref not in gated and kind not in STRUCTURAL_KINDS
    ]
    return GateMap(
        points=points,
        bindings=bindings,
        gated=gated,
        ungated=sorted(ungated),
        dangling=dangling,
        unbound=unbound,
        rationales=rationales,
        missing_rationale=missing_rationale,
        excepted=excepted,
        missing_excepted_by=missing_excepted_by,
    )


def report_gate_map(
    gm: GateMap,
    *,
    list_ungated: bool = False,
    quiet: bool = False,
    regressed: list[str] | None = None,
    baseline: bool = True,
) -> tuple[list[str], int]:
    """The four sets as report lines, plus the exit code.

    Exit code by set (spec AC-10, AC-11, AC-12):

    - gated and ungated are MEASUREMENTS and never fail. An ungated point is a
      rule nothing enforces yet, which is the number this gate exists to publish.
    - dangling FAILS. A check naming a point that does not exist is deterministic,
      is one line to fix, and is the signal that a rule moved out from under its
      gate. That is the whole reason the binding names a path rather than a file.
    - regressed FAILS. A point gated at HEAD and gated by nothing now has lost
      its machine, and `gated_regressions` states why that is not a measurement.
    - missing rationale FAILS (AC-16). A `rationale:` naming a path that is not
      on disk is the same defect class as a dangling binding, one direction out:
      the explanation moved out from under the instruction. Both are one line to
      fix and neither can be found by reading the rule.
    - missing exception FAILS (AC-20). An `excepted-by:` naming a point that is
      not on disk is the same defect class again, and it is the one this key
      exists for: deleting the exception point reds the gate, so a dedup pass
      can no longer take a guard out from under a general instruction with
      everything staying green.
    - exception COVERAGE is a measurement and never fails (AC-21), for the
      reason rationale coverage is: most instructions state no exception, and a
      demand for one would produce invented exceptions.
    - rationale COVERAGE is a measurement and never fails (AC-17), exactly as
      the ungated count is. 1,469 points and 45 `ai/rationale/` files: a red on
      coverage would be a demand to invent 1,400 explanations, and an invented
      one is worse than an absent one.
    - an empty result is never a pass. No points, or no bindings at all, means the
      join read nothing and must say so (`ai/rules/evidence.md`).
    """
    regressed = list(regressed or [])
    if not gm.points:
        return [
            (
                "no points under ai/rules/points/; the gate map read nothing "
                "and must not report success"
            )
        ], 1
    if not gm.bindings:
        return [
            (
                "no `# ze point:` binding in any dispatcher; the gate map read "
                "nothing and must not report success"
            )
        ], 1

    kinds = collections.Counter(gm.points.values())
    structural = sum(kinds[k] for k in STRUCTURAL_KINDS)
    candidates = len(gm.candidates)
    binding_checks = {b.check for b in gm.gated_bindings}

    total_checks = len(binding_checks) + len(gm.unbound)
    header = (
        f"gate map: {len(gm.points)} points, {len(gm.bindings)} bindings, "
        f"{total_checks} checks"
    )
    out = [
        header,
        "",
        f"GATED: {len(gm.gated)} point(s) named by {len(binding_checks)} check(s)",
    ]
    if not quiet:
        for ref in sorted(gm.gated):
            named = ", ".join(sorted({b.check for b in gm.gated[ref]}))
            out.append(f"  {ref}  <- {named}")

    # The dangling entries print in every mode, quiet included: they are the only
    # part of this report that fails, so suppressing them would leave a red gate
    # with nothing on screen saying which line to fix.
    out += ["", f"DANGLING: {len(gm.dangling)}"]
    for binding in gm.dangling:
        why = (
            "names no point on disk"
            if binding.check != NO_CHECK
            else "sits above no check"
        )
        out.append(f"  {binding.file}: {binding.check} -> {binding.ref} ({why})")

    # Printed in every mode for the same reason as dangling, and printed even
    # when there is no baseline: a ratchet that ran against nothing must say so
    # rather than read as a clean pass (`ai/rules/evidence.md`).
    if baseline:
        out += [
            "",
            f"REGRESSED: {len(regressed)} point(s) gated at HEAD, gated by nothing now",
        ]
        out += [f"  {ref}" for ref in regressed]
    else:
        out += ["", "REGRESSED: no HEAD baseline (git could not answer); not ratcheted"]

    out += [
        "",
        f"UNBOUND: {len(gm.unbound)} check(s) declare `none`, each with a reason",
    ]
    if not quiet:
        for binding in gm.unbound:
            out.append(f"  {binding.check}: {binding.reason}")

    # Printed in every mode, for the reason dangling is: it is a failing set, and
    # a red gate with nothing on screen names no line to fix.
    out += ["", f"MISSING RATIONALE: {len(gm.missing_rationale)}"]
    for ref, path in gm.missing_rationale:
        out.append(f"  {ref} -> {path} (no such file)")

    # Printed in every mode for the reason dangling and missing rationale are:
    # it is a failing set, and a red gate with nothing on screen names no line
    # to fix.
    out += ["", f"MISSING EXCEPTION: {len(gm.missing_excepted_by)}"]
    for ref, target in gm.missing_excepted_by:
        why = "a point cannot except itself" if ref == target else "no such point"
        out.append(f"  {ref} -> {target} ({why})")

    ungated_kinds = collections.Counter(gm.points[r] for r in gm.ungated)
    ungated_rules = collections.Counter(r.split("/", 1)[0] for r in gm.ungated)
    denominator = (
        f"  denominator excludes {structural} structural points "
        f"({kinds['heading']} heading, {kinds['fence']} fence)"
    )
    out += [
        "",
        f"UNGATED: {len(gm.ungated)} of {candidates} instruction points",
        denominator,
        "  by kind: " + ", ".join(f"{k} {n}" for k, n in sorted(ungated_kinds.items())),
        "  most ungated: "
        + ", ".join(f"{r} {n}" for r, n in ungated_rules.most_common(5)),
    ]
    if list_ungated:
        out += [f"  {ref}" for ref in gm.ungated]

    # The AC-17 measurement. The denominator is the ungated one: a heading names
    # a section and a fence quotes output, and neither states an instruction that
    # a record could explain.
    instruction_points = set(gm.candidates)
    linked = [r for r in gm.rationales if r in instruction_points]
    out += [
        "",
        f"RATIONALE: {len(linked)} of {candidates} instruction points name a record",
        "  coverage is a measurement and exits 0 whatever the number; "
        "an invented link is worse than an absent one",
    ]

    # The AC-21 measurement, on the same denominator and for the same reason: a
    # heading names a section and a fence quotes output, and neither states a
    # general instruction that an exception could carve into.
    general = [r for r in gm.excepted if r in instruction_points]
    exceptions = {t for refs in gm.excepted.values() for t in refs}
    out += [
        "",
        f"EXCEPTED: {len(general)} of {candidates} instruction points name an "
        f"exception, naming {len(exceptions)} point(s)",
        "  coverage is a measurement and exits 0 whatever the number; most "
        "instructions state no exception",
    ]

    return out, (
        1
        if (gm.dangling or regressed or gm.missing_rationale or gm.missing_excepted_by)
        else 0
    )


# --------------------------------------------------------------------------
# The published claim: the Hook-to-Rule Mapping table
# --------------------------------------------------------------------------

# `ai/rules/repo-maintenance.md` publishes one sub-table per PreToolUse
# dispatcher, and its `Check` and `Enforces` columns restate what the binding
# comments already state. Two copies of one fact drift, and this one had:
# four checks carried no row at all and one row named a function deleted from
# the tree.
#
# The table is NOT generated. A row is not separable: `Triggers on` and
# `What it does` are authored prose, and a generator would have to rewrite two
# cells inside an authored markdown row that carries escaped pipes and trailing
# HTML comments. No other tool in this system looks inside a point body, and the
# byte-identical round trip rests on bodies staying verbatim. A check buys most
# of the same property: the `Check` column cannot disagree with the roster, and
# the `Enforces` column cannot disagree with the bindings AT RULE GRANULARITY.
# It is one level coarser than the bindings themselves, so rebinding a check
# from one point to another point of the SAME rule leaves the cell correct and
# is invisible here. `hook_table_problems` says so where it makes the
# comparison.
DOC_RULE = "repo-maintenance"
TABLE_HEAD = "| Check | Enforces |"
# A `##`-to-`####` heading naming a dispatcher by its file name opens that
# dispatcher's sub-table. The heading is how a table is MATCHED to a dispatcher,
# so no list of table locations is kept. Which dispatchers must have one is a
# separate question, and `dispatchers()` derives that from settings.json.
DISPATCH_HEADING = re.compile(r"^#{2,4}\s.*`([A-Za-z0-9_-]+\.py)`")
# A markdown cell boundary. `\|` inside a cell is content, not a boundary.
CELL = re.compile(r"(?<!\\)\|")
BACKTICKED = re.compile(r"`([^`]+)`")
# A check is a `c_`/`check_` top-level def. Every one of them is also required to
# carry a binding, so a check that somehow carries none still owes a row.
CHECK_DEF = re.compile(r"^def\s+((?:c|check)_[a-z0-9_]+)\s*\(", re.MULTILINE)


def dispatcher_checks(text: str, bindings: list[Binding]) -> set[str]:
    """Every check in one dispatcher, derived from what that dispatcher RUNS.

    Four sources, unioned, because the dispatchers dispatch in two shapes and
    the `c_`/`check_` prefix is demonstrably not universal:

    - the module's `CHECKS` tuple. That IS the dispatch table in
      `pretool-writeedit.py` and `pretool-bash.py`, so a name in it runs.
    - the top-level functions `main()` calls by name, for a module with no
      `CHECKS` tuple. `pretool-agent-skill.py` calls its two gates directly and
      neither carries the prefix. A `_`-prefixed name is excluded there: the
      dispatcher's helpers already declare themselves that way, and the default
      is the other one, so a gate has to be named private ON PURPOSE to escape.
    - any `c_`/`check_` def. One defined but absent from the dispatch table
      still owes a row, and its absence is itself worth seeing.
    - any def a binding names, so a check that declares what it enforces can
      never escape the published table by being shaped unusually.

    Reading only the prefix and the bindings was the hole this closes: a gate
    that is neither prefixed nor bound was invisible, and two of the three
    dispatchers' gates are already unprefixed.
    """
    try:
        tree = ast.parse(text)
    except SyntaxError as err:
        raise RulePointsError(f"cannot be parsed: {err}") from None

    top = {
        node.name
        for node in tree.body
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef))
    }
    dispatched: set[str] = set()
    for node in tree.body:
        if not isinstance(node, ast.Assign):
            continue
        if not any(
            isinstance(target, ast.Name) and target.id == "CHECKS"
            for target in node.targets
        ):
            continue
        if isinstance(node.value, (ast.Tuple, ast.List)):
            dispatched |= {el.id for el in node.value.elts if isinstance(el, ast.Name)}

    if not dispatched:
        for node in tree.body:
            if isinstance(node, ast.FunctionDef) and node.name == "main":
                for call in ast.walk(node):
                    if (
                        isinstance(call, ast.Call)
                        and isinstance(call.func, ast.Name)
                        and call.func.id in top
                        and not call.func.id.startswith("_")
                    ):
                        dispatched.add(call.func.id)

    named = {b.check for b in bindings if b.check != NO_CHECK}
    return dispatched | set(CHECK_DEF.findall(text)) | named


def cells(line: str) -> list[str]:
    """The cells of one markdown table row, in order, stripped."""
    return [c.strip() for c in CELL.split(line.strip())]


def published_rows(doc_text: str) -> dict[str, list[tuple[int, str, str]]]:
    """Each dispatcher file name mapped to its published rows.

    A row is `(line number, check name, the Enforces cell)`. Rows are read from
    the first `Check | Enforces` table under the heading that names the
    dispatcher, and the table ends at the first line that is not a row.
    """
    out: dict[str, list[tuple[int, str, str]]] = {}
    current = ""
    rows: list[tuple[int, str, str]] | None = None
    for number, line in enumerate(doc_text.split("\n"), 1):
        heading = DISPATCH_HEADING.match(line)
        if heading:
            current, rows = heading.group(1), None
            continue
        if rows is not None:
            if not line.startswith("|"):
                rows = None
                continue
            field_list = cells(line)
            if len(field_list) > 2 and not set(field_list[1]) <= {"-", ":"}:
                rows.append((number, field_list[1].strip("`"), field_list[2]))
            continue
        if current and line.startswith(TABLE_HEAD) and current not in out:
            rows = out.setdefault(current, [])
    return out


def rule_stems_named(cell: str, stems: set[str]) -> set[str]:
    """The rule stems a published `Enforces` cell names.

    Only a backticked token that IS `<stem>.md` counts. A path such as
    `.claude/rules/planning.md` names a rule outside this corpus and a check
    binds no point for it, so it must not read as a claim about `planning.md`.
    """
    found = set()
    for token in BACKTICKED.findall(cell):
        if token.endswith(".md") and token[: -len(".md")] in stems:
            found.add(token[: -len(".md")])
    return found


def hook_table_problems(
    gm: GateMap, doc_text: str, sources: dict[str, str]
) -> list[str]:
    """Where the published Hook-to-Rule Mapping disagrees with the bindings.

    Fails closed at every step. A dispatcher with no published table, and a
    published table with no rows, are both problems: the table's job is to name
    every check, and an empty one names none while looking finished.

    The `Enforces` comparison is at RULE granularity, because that is what the
    cell publishes. A check rebound from one point to another point of the SAME
    rule leaves the cell correct and is invisible here; the binding comment
    itself is the finer record, and `gated_regressions` is what ratchets it.
    """
    problems: list[str] = []
    stems = {ref.split("/", 1)[0] for ref in gm.points}
    tables = published_rows(doc_text)

    for name, source in sources.items():
        bindings = [b for b in gm.bindings if b.file == name]
        try:
            roster = dispatcher_checks(source, bindings)
        except RulePointsError as err:
            problems.append(f"{name}: {err}; its checks cannot be enumerated")
            continue
        rows = tables.get(name)
        if not rows:
            problems.append(
                f"{name}: no `{TABLE_HEAD}...` table under a heading naming it; "
                f"{len(roster)} check(s) are published nowhere"
            )
            continue

        seen: dict[str, int] = {}
        for number, check, enforces in rows:
            if check in seen:
                problems.append(
                    f"ai/rules/{DOC_RULE}.md:{number}: `{check}` has a second row "
                    f"(the first is line {seen[check]})"
                )
                continue
            seen[check] = number
            if check not in roster:
                problems.append(
                    f"ai/rules/{DOC_RULE}.md:{number}: row `{check}` names no "
                    f"check in {name}; delete the row or restore the function"
                )
                continue
            want = {
                b.ref.split("/", 1)[0] for b in bindings if b.check == check and b.ref
            }
            have = rule_stems_named(enforces, stems)
            if want != have:
                wanted = ", ".join(f"`{s}.md`" for s in sorted(want)) or "no rule"
                problems.append(
                    f"ai/rules/{DOC_RULE}.md:{number}: `{check}` Enforces names "
                    f"{', '.join(sorted(have)) or 'no rule'}, its bindings say "
                    f"{wanted}"
                )
        for check in sorted(roster - set(seen)):
            problems.append(
                f"{name}: `{check}` has no row in the Hook-to-Rule Mapping table"
            )
    return problems


def main(argv: list[str] | None = None) -> int:
    root = REPO_ROOT
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_split = sub.add_parser("split", help="split rendered rules into point files")
    p_split.add_argument("--out", required=True, help="output directory")

    p_render = sub.add_parser(
        "render", help="render ai/rules/points/ back to ai/rules/<rule>.md"
    )
    p_render.add_argument(
        "dir", nargs="?", help="one rule's point directory; print it to stdout instead"
    )
    p_render.add_argument(
        "--check",
        action="store_true",
        help="compare instead of writing, exit 1 on drift",
    )

    sub.add_parser("roundtrip", help="split then render every rule, compare bytes")

    p_cov = sub.add_parser(
        "coverage", help="which points the hook checks enforce: gated/ungated/dangling"
    )
    p_cov.add_argument(
        "--ungated",
        action="store_true",
        help="list every ungated point, not only the counts",
    )
    p_cov.add_argument(
        "--quiet",
        action="store_true",
        help="counts only, plus every dangling binding",
    )

    args = parser.parse_args(argv)
    rules_dir = root / "ai" / "rules"
    points_dir = rules_dir / "points"

    if args.cmd == "split":
        out = Path(args.out)
        out.mkdir(parents=True, exist_ok=True)
        for path in rule_files(rules_dir):
            write_split(split_rule(path.read_text(encoding="utf-8"), path.stem), out)
        print(f"rules-points: split {len(rule_files(rules_dir))} rules into {out}")
        return 0

    if args.cmd == "render":
        if args.dir:
            sys.stdout.write(render_dir(Path(args.dir)))
            return 0
        failures = render_all(rules_dir, points_dir, check=args.check)
        if failures:
            for line in failures:
                print(f"rules-points: {line}", file=sys.stderr)
            print(
                f"rules-points: {len(failures)} rule(s) are stale; "
                "run `make ze-rules-render`",
                file=sys.stderr,
            )
            return 1
        count = len(point_dirs(points_dir))
        verb = "are fresh" if args.check else "rendered"
        print(f"rules-points: {count} rules {verb}")
        return 0

    if args.cmd == "coverage":
        gate_files, problems = dispatchers(root)
        if problems:
            # A dispatcher that moved, or one that runs nowhere, takes every
            # binding in it out of the join and the map then looks greener.
            for problem in problems:
                print(f"rules-points: {problem}", file=sys.stderr)
            return 1
        gm = gate_map(gate_files, points_dir, root)
        head = head_sources(root, [p.name for p in gate_files])
        regressed = (
            gated_regressions(gm, gated_at_head(head)) if head is not None else []
        )
        lines, code = report_gate_map(
            gm,
            list_ungated=args.ungated,
            quiet=args.quiet,
            regressed=regressed,
            baseline=head is not None,
        )
        for line in lines:
            print(line)
        if code:
            if gm.dangling:
                reason = (
                    f"{len(gm.dangling)} dangling binding(s): each names a point "
                    "that does not exist, or sits above no check"
                )
            elif regressed:
                reason = (
                    f"{len(regressed)} point(s) were gated at HEAD and are gated by "
                    "nothing now; restore the binding, or say which check replaces it"
                )
            elif gm.missing_rationale:
                reason = (
                    f"{len(gm.missing_rationale)} point(s) name a rationale that is "
                    "not on disk; repoint it at where the record moved to, or drop "
                    "the field rather than leave it naming nothing"
                )
            elif gm.missing_excepted_by:
                reason = (
                    f"{len(gm.missing_excepted_by)} point(s) name an `excepted-by` "
                    "point that does not exist; restore the exception, or repoint "
                    "the general instruction at where the exception moved to. Do "
                    "not drop the field while the general statement still needs "
                    "the carve-out a reader would otherwise miss"
                )
            else:
                reason = lines[0]
            print(f"rules-points: {reason}", file=sys.stderr)

        doc = rules_dir / f"{DOC_RULE}.md"
        if not doc.is_file():
            print(f"rules-points: {doc} not found", file=sys.stderr)
            return 1
        sources = {p.name: p.read_text(encoding="utf-8") for p in gate_files}
        problems = hook_table_problems(gm, doc.read_text(encoding="utf-8"), sources)
        print()
        print(f"PUBLISHED: {len(problems)} disagreement(s) with `{doc}`")
        for problem in problems:
            print(f"  {problem}")
        if problems:
            print(
                f"rules-points: the Hook-to-Rule Mapping table in {doc} "
                "disagrees with the binding comments",
                file=sys.stderr,
            )
            code = 1
        return code

    with tempfile.TemporaryDirectory(prefix="ze-rules-points-") as tmp:
        failures = roundtrip(rules_dir, Path(tmp))
    total = len(rule_files(rules_dir))
    if failures:
        for line in failures:
            print(f"rules-points: {line}", file=sys.stderr)
        print(
            f"rules-points: {len(failures)} of {total} rules do not round-trip",
            file=sys.stderr,
        )
        return 1
    print(f"rules-points: all {total} rules round-trip byte-identical")
    return 0


if __name__ == "__main__":
    sys.exit(main())
