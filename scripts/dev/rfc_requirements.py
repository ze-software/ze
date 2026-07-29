#!/usr/bin/env python3
"""RFC requirement coverage: bind every RFC MUST to the tests that enforce it.

Spec: plan/spec-rfc-requirement-coverage.md

WHAT THIS ENFORCES
------------------
`rfc/short/*.md` Compliance Checklists are the registry of RFC obligations. Each line
carries a permanent ID (RFC7606-5.3-6 = §5.3, 6th requirement). A test that enforces an
obligation tags itself:

    // RFC requirement: RFC7606-7.1-1 negative -- ORIGIN length 2 is treated as withdraw
    # RFC requirement: RFC7606-7.1-1 negative -- malformed ORIGIN withdraws the route

The link is two-way, but only ONE side is authored. The tag lives in the test, so it
dies with the test; `ai/RFC-REQUIREMENTS.md` renders the reverse direction and is
GENERATED (ai/rules/derive-not-hardcode.md). A hand-written back-link would outlive the
test it names -- exactly the silent rot this gate exists to catch.

For an ENROLLED RFC (rfc/enrolled.txt), every MUST-level requirement must have BOTH a
positive and a negative tag, or carry an annotation explaining why not. A negative-only
test passes if the code rejects everything; a positive-only test passes if it accepts
everything. Only the pair pins behavior to the requirement.

Annotations are not escape hatches. Each needs a reason, and each FAILS once it becomes
stale -- the two-sided ratchet copied from scripts/dev/dep_audit.py:834-879 (fail on a
NEW violation AND on a baseline row that is no longer true), so the exception set can
only shrink.

Usage:
    python3 scripts/dev/rfc_requirements.py --check        # gate (exit 2 on violation)
    python3 scripts/dev/rfc_requirements.py --check-fresh  # ledger staleness only (exit 1)
    python3 scripts/dev/rfc_requirements.py --write        # render ai/RFC-REQUIREMENTS.md
    python3 scripts/dev/rfc_requirements.py --selftest     # run rfc_requirements_test.py

Exit 0 = a comparison ran and found nothing wrong.
Exit 2 = violations found, or the gate could not run (unparseable input, nothing
         enrolled, enrolled RFC with no summary). "Clean" must mean "I compared things
         and found nothing", never "I compared nothing" (ai/rules/fail-closed-guards.md).
"""

import calendar
import datetime
import hashlib
import json
import math
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from typing import Dict, List, NamedTuple, Optional, Sequence, Set, Tuple

_HERE = os.path.dirname(os.path.abspath(__file__))
PROJECT_DIR = os.path.abspath(os.path.join(_HERE, "..", ".."))

SUMMARY_DIR = os.path.join(PROJECT_DIR, "rfc", "short")
ENROLLED_FILE = os.path.join(PROJECT_DIR, "rfc", "enrolled.txt")
STATUS_FILE = os.path.join(PROJECT_DIR, "docs", "features", "rfc-status.md")
LEDGER_FILE = os.path.join(PROJECT_DIR, "ai", "RFC-REQUIREMENTS.md")
AUDIT_DIR = os.path.join(PROJECT_DIR, "rfc", "audit")

TEST_ROOTS = ("internal", "pkg", "test")

RED = "\033[31m"
YELLOW = "\033[33m"
GREEN = "\033[32m"
BOLD = "\033[1m"
RESET = "\033[0m"

# RFC 2119 keywords that create an obligation the gate enforces. SHOULD/MAY are listed
# in the ledger and may be tagged, but never gate (scope decision, spec Task section).
GATED_LEVELS = frozenset({"MUST", "MUST NOT", "SHALL", "SHALL NOT", "REQUIRED"})
ADVISORY_LEVELS = frozenset(
    {"SHOULD", "SHOULD NOT", "MAY", "RECOMMENDED", "NOT RECOMMENDED", "OPTIONAL"}
)
ALL_LEVELS = GATED_LEVELS | ADVISORY_LEVELS

POLARITIES = frozenset({"positive", "negative"})

ANNOTATION_KINDS = frozenset({"not-applicable", "gap", "single-polarity"})

# Longest-first so "MUST NOT" wins over "MUST" and "NOT RECOMMENDED" over "RECOMMENDED".
_LEVEL_ALT = "|".join(re.escape(k) for k in sorted(ALL_LEVELS, key=len, reverse=True))

_CHECKLIST_RE = re.compile(
    r"^-\s*\[(?P<box>[ xX])\]\s*"
    # Sections carry lowercase letters (§3.b, §7.11), so the id must too.
    r"(?:\[(?P<rid>[A-Za-z0-9][A-Za-z0-9.\-]*-\d+)\]\s*)?"
    r"\[(?P<level>" + _LEVEL_ALT + r")\]\s*"
    r"(?P<rest>.*)$"
)

# Tells "this line is trying to be a requirement and is malformed" from "this line is
# prose" and from "this line is an ad-hoc implementation checklist entry".
#
# Some summaries (rfc5340, rfc4552, rfc2966, ...) use ad-hoc category tags -- [FORMAT],
# [IPSEC], [TRANSPORT], [LSA] -- listing implementation TASKS rather than RFC 2119
# obligations. Those are not compliance lines and are not requirements; they are why those
# summaries capture zero MUSTs and need re-authoring. Treat them as prose here and report
# the summary as non-conforming in the ledger, rather than crash on them or -- worse --
# silently drop the whole file.
_FIRST_TAG_RE = re.compile(r"^-\s*\[[ xX]\]\s*\[(?P<tag>[^\]]*)\]")

# Any bracketed RFC 2119 keyword anywhere on the line. Presence of one means "this line is
# a compliance requirement", independent of whether the id parses -- which is what makes
# a malformed id an ERROR instead of a silent skip.
_LEVEL_BRACKET_RE = re.compile(r"\[(?:" + _LEVEL_ALT + r")\]")

# RFC7606-5.3-4 = RFC 7606, section 5.3, 4th requirement of that section.
#
# The section is the anchor because RFCs are IMMUTABLE: section numbers are frozen the day
# the RFC publishes, which makes them the most stable name available.
#
# RETIRED FORM -- never allocate one: a bare per-RFC counter ("<RFC>-R055"). It encodes
# ALLOCATION ORDER, an artifact of our editing history rather than of the RFC. It teaches a
# reader nothing, its neighbours are unrelated requirements, and once a requirement is
# appended (which renumbering rules force) it stops tracking the document at all. The
# parser rejects the form outright so it cannot come back.
#
# Split on the LAST hyphen for the ordinal; everything between the RFC prefix and that
# hyphen is the section, so dotted (5.3), lettered (3.b) and deep (9.1.2.2) sections all
# work. RFC section numbering never produces a literal "5.3-4", so this is unambiguous.
_ID_RE = re.compile(r"^(?P<head>.+)-(?P<ord>\d+)$")

# Three citation styles are in the wild: "(§5.3)" (2072 lines), "(Section 6)" (669), and
# "(S4.1)" (230). Accept all three; NO_SECTION covers a line citing none of its own.
#
# The bare-S form is why RFC 9234's requirements once anchored to "x" despite citing their
# sections perfectly well. `\bS(?=\d)` demands a digit right after, so SHOULD/SHALL cannot
# match, and the S of "AS4" has no word boundary before it.
#
# A cite naming ANOTHER RFC ("(RFC 2328 §A.3.1)" on an RFC 1071 requirement) must NOT be
# anchored: A.3.1 is RFC 2328's section, and hanging an RFC1071 id off it would name the
# wrong document. _CROSS_RFC_SEC_RE finds those so they can be excluded.
_SECTION_RE = re.compile(
    r"(?:§\s*|\bSection\s+|\bS(?=\d))(?P<sec>[0-9A-Za-z]+(?:\.[0-9A-Za-z]+)*)"
)
_CROSS_RFC_SEC_RE = re.compile(
    r"RFC\s*\d+\s*(?:§\s*|\bSection\s+|\bS(?=\d))[0-9A-Za-z.]+"
)

# The last parenthetical on the line: by convention that is where the section is cited.
_TRAILING_PAREN_RE = re.compile(r"\((?P<body>[^()]*)\)[^()]*$")

# Anchor used when a requirement cites no section of its own. Deliberately conspicuous:
# it is a summary defect to fix (the section reference is what lets an implementer find
# the normative text), not a resting state.
NO_SECTION = "x"
_ANNOTATION_RE = re.compile(r"\{(?P<body>[^{}]*)\}\s*$")

_GO_TAG_RE = re.compile(r"^\s*//\s*RFC requirement:\s*(?P<rest>.*)$")
_CI_TAG_RE = re.compile(r"^#\s*RFC requirement:\s*(?P<rest>.*)$")
_TERMINATOR_RE = re.compile(r"terminator=(?P<name>[A-Za-z0-9_]+)")

# Rows the status ledger uses to say "nothing missing here". A `{gap}` requirement whose
# RFC row says this is a contradiction the gate refuses (AC-10).
_NO_GAP_RE = re.compile(
    r"no tracked gap|none claimed|no separate gap|explicitly unsupported", re.I
)


class ParseError(Exception):
    """Input is malformed. Always raised, never swallowed: a silently skipped MUST is a
    false green (ai/rules/fail-closed-guards.md)."""


class Annotation(NamedTuple):
    kind: str
    polarity: Optional[str]
    reason: str


class Requirement(NamedTuple):
    rfc: str
    rid: str
    level: str
    text: str
    section: str
    annotation: Optional[Annotation]
    source: str
    line: int
    # A hand-ticked "- [x]". Recorded, not obeyed: coverage is DERIVED from test tags, so
    # a tick is someone's claim, not evidence. Reported as a violation once the RFC is
    # enrolled -- the tick is exactly the declare-instead-of-prove habit this replaces.
    ticked: bool = False

    @property
    def gated(self) -> bool:
        return self.level in GATED_LEVELS


class Tag(NamedTuple):
    rid: str
    polarity: str
    file: str
    line: int


# --------------------------------------------------------------------------
# Summary parsing
# --------------------------------------------------------------------------
def rfc_prefix(rfc_stem: str) -> str:
    """rfc7606 -> RFC7606; draft-foo-bar -> DRAFT-FOO-BAR."""
    return rfc_stem.upper()


def _parse_annotation(body: str, where: str) -> Annotation:
    if ":" not in body:
        raise ParseError(
            f"{where}: annotation {{{body}}} has no reason. "
            f"Every annotation must justify itself: {{kind: why}}"
        )
    kind, _, rest = body.partition(":")
    kind = kind.strip()
    rest = rest.strip()
    if kind not in ANNOTATION_KINDS:
        raise ParseError(
            f"{where}: unknown annotation kind {kind!r}; "
            f"expected one of {sorted(ANNOTATION_KINDS)}"
        )
    if not rest:
        raise ParseError(
            f"{where}: annotation {{{kind}}} has an empty reason. "
            f"A bare annotation is an escape hatch; say why."
        )
    if kind == "single-polarity":
        polarity, _, why = rest.partition(";")
        polarity = polarity.strip()
        why = why.strip()
        if polarity not in POLARITIES:
            raise ParseError(
                f"{where}: single-polarity needs a polarity from {sorted(POLARITIES)}, "
                f"got {polarity!r}. Format: {{single-polarity: negative; why}}"
            )
        if not why:
            raise ParseError(
                f"{where}: single-polarity needs a reason explaining why the other "
                f"polarity cannot be tested. Format: {{single-polarity: negative; why}}"
            )
        return Annotation(kind=kind, polarity=polarity, reason=why)
    return Annotation(kind=kind, polarity=None, reason=rest)


def extract_section(text: str) -> str:
    """The section an id anchors to: the FIRST section of THIS RFC that the line cites.

    Returns NO_SECTION ("x") when the line cites none of its own.

    Accepts both house styles, "(§5.3)" and "(Section 6)", because the summaries use both
    and neither is wrong. Ignores cites that name another document ("RFC 2328 §A.3.1"):
    anchoring an RFC 1071 requirement to A.3.1 would point at RFC 2328's section numbering
    and mislead every reader who followed it.

    A merged requirement may cite several ("§2, §3.h"); the first wins so the id stays
    single-valued.

    The citation is the TRAILING parenthetical by convention, and that distinction is
    load-bearing: a requirement whose prose happens to mention another section first
    ("... routes via §3.j to session reset ... (§5.3)") must anchor to §5.3, the section
    it is FROM, not §3.j, the section it refers TO. Searching the trailing group first and
    only then the whole line gets that right.
    """
    scrubbed = _CROSS_RFC_SEC_RE.sub("", text)

    tail = _TRAILING_PAREN_RE.search(scrubbed)
    if tail:
        m = _SECTION_RE.search(tail.group("body"))
        if m:
            return m.group("sec").strip()

    m = _SECTION_RE.search(scrubbed)
    if not m:
        return NO_SECTION
    return m.group("sec").strip()


def _validate_id(rid: str, rfc_stem: str, section: str, where: str) -> None:
    """An id must be <PREFIX>-<section>-<ordinal>, and its section must MATCH the section
    the line cites.

    That cross-check is the payoff of anchoring to sections: a sequential counter can drift
    away from the text it names and nothing notices, whereas here an id claiming §5.3 on a
    line citing §7.1 is a contradiction the parser refuses.
    """
    m = _ID_RE.match(rid)
    if not m:
        raise ParseError(
            f"{where}: malformed requirement id {rid!r}; expected "
            f"{rfc_prefix(rfc_stem)}-<section>-<n>, e.g. {rfc_prefix(rfc_stem)}-5.3-4"
        )
    want_head = rfc_prefix(rfc_stem) + "-" + (section or NO_SECTION)
    if m.group("head") != want_head:
        cited = f"§{section}" if section else "no section"
        raise ParseError(
            f"{where}: id {rid!r} disagrees with its section ({cited}); "
            f"expected {want_head}-<n>. The id is anchored to the section it cites, so "
            f"the two can never drift apart"
        )
    if int(m.group("ord")) < 1:
        raise ParseError(f"{where}: id {rid!r} ordinal starts at 1")


def parse_checklist_line(
    line: str, rfc_stem: str, source: str = "", lineno: int = 0
) -> Optional[Requirement]:
    """Parse one Compliance Checklist line. None if the line is not a checklist entry.

    Raises ParseError for a line that is TRYING to be a requirement but is malformed --
    including a legacy line with no ID. Never returns None for those: skipping a MUST is
    how a gate goes green while an obligation is unenforced.
    """
    where = f"{source}:{lineno}" if source else "line"
    m = _CHECKLIST_RE.match(line)
    if not m:
        tm = _FIRST_TAG_RE.match(line)
        if tm:
            # A line CARRYING an RFC 2119 keyword bracket is a compliance line, full stop.
            # If it did not parse, it is malformed and must fail loudly.
            #
            # Deciding this from the FIRST bracket alone was a fail-open: a line like
            #   - [ ] [RFC9234-R012] [MUST] ... (§5)   <- retired counter form
            # has an unrecognised first bracket, so it was dismissed as an ad-hoc category
            # line and silently dropped -- taking a live MUST out of the ledger with it.
            # A silently skipped obligation is precisely what this gate exists to prevent
            # (ai/rules/fail-closed-guards.md).
            if _LEVEL_BRACKET_RE.search(line):
                raise ParseError(
                    f"{where}: checklist line carries an RFC 2119 keyword but does not "
                    f"parse: {line.strip()!r}. Expected: "
                    f"- [ ] [{rfc_prefix(rfc_stem)}-<section>-<n>] [MUST] text (§N). "
                    f"(The old {rfc_prefix(rfc_stem)}-RNNN counter form is retired -- ids "
                    f"are anchored to the section they cite.)"
                )
            # An ad-hoc category tag ([FORMAT], [IPSEC], ...) with no 2119 keyword is an
            # implementation-task checklist, not a compliance line: not a requirement,
            # not an error.
        return None

    ticked = bool(m.group("box").strip())

    rid = m.group("rid")
    if not rid:
        raise ParseError(
            f"{where}: checklist line has no requirement id: {line.strip()!r}. "
            f"Every line needs one so tests can reference it: "
            f"- [ ] [{rfc_prefix(rfc_stem)}-<section>-<n>] [{m.group('level')}] ..."
        )

    rest = m.group("rest").strip()

    annotation = None
    am = _ANNOTATION_RE.search(rest)
    if am:
        annotation = _parse_annotation(am.group("body").strip(), where)
        rest = rest[: am.start()].strip()

    section = extract_section(rest)
    text = rest

    # After the section is known: the id must agree with it.
    _validate_id(rid, rfc_stem, section, where)

    return Requirement(
        rfc=rfc_stem,
        rid=rid,
        level=m.group("level"),
        text=text,
        section=section,
        annotation=annotation,
        source=source,
        line=lineno,
        ticked=ticked,
    )


def parse_summary_text(text: str, rfc_stem: str, source: str = "") -> List[Requirement]:
    """Parse every checklist line in a summary. Raises on duplicate IDs."""
    out: List[Requirement] = []
    seen: Dict[str, int] = {}
    for i, line in enumerate(text.split("\n"), start=1):
        req = parse_checklist_line(line, rfc_stem, source=source, lineno=i)
        if req is None:
            continue
        if req.rid in seen:
            raise ParseError(
                f"{source or rfc_stem}: duplicate requirement id {req.rid} "
                f"(lines {seen[req.rid]} and {i}). IDs are permanent and unique."
            )
        seen[req.rid] = i
        out.append(req)
    return out


def parse_summary_file(path: str) -> List[Requirement]:
    stem = os.path.basename(path)[: -len(".md")]
    with open(path, encoding="utf-8") as fh:
        return parse_summary_text(
            fh.read(), stem, source=os.path.relpath(path, PROJECT_DIR)
        )


def _ord_of(rid: str) -> Optional[int]:
    m = _ID_RE.match(rid)
    return int(m.group("ord")) if m else None


def _head_of(rid: str) -> Optional[str]:
    """The <PREFIX>-<section> part: the scope an ordinal is unique within."""
    m = _ID_RE.match(rid)
    return m.group("head") if m else None


def high_water(ids: Set[str]) -> Dict[str, int]:
    """Highest ordinal ever allocated, PER SECTION.

    Per-section rather than per-RFC is the point of the section anchor: adding a
    requirement to §5.3 only has to clear §5.3's mark, so it lands beside its siblings
    instead of being exiled to the end of the file. Under a per-RFC counter, every insert
    landed out of document order and the number stopped meaning anything.
    """
    out: Dict[str, int] = {}
    for rid in ids:
        head, ordinal = _head_of(rid), _ord_of(rid)
        if head is None or ordinal is None:
            continue
        out[head] = max(out.get(head, 0), ordinal)
    return out


def check_id_allocation(
    requirements: Sequence[Requirement], baseline_ids: Set[str]
) -> List[str]:
    """IDs are allocated once and never reused.

    `baseline_ids` is the ID set from the committed (HEAD) version of the summaries.

    Reuse and text-correction are indistinguishable from content alone -- "R005 now says
    something different" is either a fixed misquote (fine, keep the ID) or a retired
    number silently re-pointed at a new obligation (catastrophic: every test tagged R005
    now claims to enforce something it has never seen).

    So the rule is positional, not textual: BELOW the section's high-water mark you may
    only keep ids that already existed. A new requirement must take an ordinal ABOVE it.
    That makes "delete 5.3-4, add a different 5.3-4" mechanically detectable while leaving
    text edits completely free.
    """
    errs: List[str] = []
    marks = high_water(baseline_ids)
    for req in requirements:
        head, ordinal = _head_of(req.rid), _ord_of(req.rid)
        if head is None or ordinal is None:
            continue
        mark = marks.get(head)
        if mark is None:
            continue
        if ordinal <= mark and req.rid not in baseline_ids:
            errs.append(
                f"{req.source}:{req.line}: {req.rid} reuses a retired id. "
                f"{head} has allocated up to -{mark}; a new requirement must take "
                f"-{mark + 1} or higher. Reusing an id silently re-points every test "
                f"tagged {req.rid} at a different obligation."
            )
    return errs


# --------------------------------------------------------------------------
# Tag scanning
# --------------------------------------------------------------------------
# Trailing punctuation an author legitimately writes around a tag. `godot` requires a Go
# doc comment's last line to end in a period, so a tag placed last becomes
# "RFC7606-2-1 negative." -- rejecting that would make the lint rule and the tag convention
# contradict each other, and the author would have no way to satisfy both.
_TAG_PUNCT = ".,;:"


def _parse_tag_rest(rest: str, where: str) -> Tag:
    parts = rest.split()
    if not parts:
        raise ParseError(f"{where}: empty 'RFC requirement:' tag")
    rid = parts[0].rstrip(_TAG_PUNCT)
    if len(parts) < 2:
        raise ParseError(
            f"{where}: tag for {rid} has no polarity. Polarity is mandatory and never "
            f"inferred: 'RFC requirement: {rid} positive|negative -- note'"
        )
    polarity = parts[1].lower().rstrip(_TAG_PUNCT)
    if polarity not in POLARITIES:
        raise ParseError(
            f"{where}: tag for {rid} has invalid polarity {parts[1]!r}; "
            f"expected one of {sorted(POLARITIES)}"
        )
    return Tag(rid=rid, polarity=polarity, file="", line=0)


def scan_go_tags(src: str, path: str) -> List[Tag]:
    """Find `// RFC requirement: <ID> <polarity>` anywhere in a Go test file.

    Deliberately not limited to doc comments: one function can cover a dozen
    requirements across ~100 table cases (internal/component/bgp/message/rfc7606_test.go
    :1193-1928), so tags must be placeable inline at the case. A function-level-only tag
    would stay green after the single enforcing case was deleted.
    """
    out: List[Tag] = []
    for i, line in enumerate(src.split("\n"), start=1):
        m = _GO_TAG_RE.match(line)
        if not m:
            continue
        t = _parse_tag_rest(m.group("rest"), f"{path}:{i}")
        out.append(t._replace(file=path, line=i))
    return out


def scan_ci_tags(src: str, path: str) -> List[Tag]:
    """Find `# RFC requirement: <ID> <polarity>` in a .ci file.

    Two constraints from the real parser (internal/test/runner/parsing.go:248, plus the
    sibling parsers record_parse.go:170, decoding.go:177, runner.go:78): a comment is
    only a comment at line start after trimming, and content inside a `terminator=` block
    is RAW FILE CONTENT, not .ci syntax -- test/plugin/rfc7606-withdraw.ci:35 embeds a
    Python shebang. Scanning those blocks would invent phantom tags.
    """
    out: List[Tag] = []
    terminator: Optional[str] = None
    for i, line in enumerate(src.split("\n"), start=1):
        if terminator is not None:
            if line.strip() == terminator:
                terminator = None
            continue
        stripped = line.strip()
        m = _CI_TAG_RE.match(stripped)
        if m:
            t = _parse_tag_rest(m.group("rest"), f"{path}:{i}")
            out.append(t._replace(file=path, line=i))
            continue
        if stripped.startswith("#"):
            continue
        tm = _TERMINATOR_RE.search(stripped)
        if tm:
            terminator = tm.group("name")
    return out


def scan_tree(root: str = PROJECT_DIR) -> List[Tag]:
    tags: List[Tag] = []
    for sub in TEST_ROOTS:
        base = os.path.join(root, sub)
        if not os.path.isdir(base):
            continue
        for dirpath, dirnames, filenames in os.walk(base):
            # Sort in place so the walk is deterministic across filesystems: os.walk
            # yields entries in directory order, which varies by machine. The render
            # re-sorts anyway, but a stable scan keeps `tags` order reproducible for
            # every other consumer too.
            dirnames[:] = sorted(
                d for d in dirnames if d not in (".git", "vendor", "testdata")
            )
            for name in sorted(filenames):
                path = os.path.join(dirpath, name)
                rel = os.path.relpath(path, root)
                try:
                    if name.endswith("_test.go"):
                        with open(path, encoding="utf-8", errors="replace") as fh:
                            tags.extend(scan_go_tags(fh.read(), rel))
                    elif name.endswith(".ci"):
                        with open(path, encoding="utf-8", errors="replace") as fh:
                            tags.extend(scan_ci_tags(fh.read(), rel))
                except OSError as exc:
                    raise ParseError(f"{rel}: cannot read: {exc}") from exc
    return tags


# --------------------------------------------------------------------------
# Coverage evaluation
# --------------------------------------------------------------------------
def evaluate(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
) -> List[str]:
    """Return every coverage violation. Empty list = a real comparison found nothing."""
    errs: List[str] = []
    known = {r.rid for r in requirements}

    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        if t.rid not in known:
            # Same bug class as a dangling `// Design:` reference
            # (scripts/dev/check_doc_links.py:206-209).
            errs.append(f"{t.file}:{t.line}: unknown RFC requirement: {t.rid}")
            continue
        by_rid.setdefault(t.rid, []).append(t)

    for req in requirements:
        if req.rfc not in enrolled:
            continue
        found = by_rid.get(req.rid, [])
        polarities = {t.polarity for t in found}
        ann = req.annotation
        where = f"{req.source}:{req.line}" if req.source else req.rid

        if req.ticked:
            errs.append(
                f"{where}: {req.rid} has a ticked checkbox. The box is a template marker, "
                f"not coverage state -- a tick is a claim, and this gate exists because "
                f"claims are what rot. Untick it; coverage comes from the test tags"
            )

        if ann and ann.kind in ("not-applicable", "gap"):
            if found:
                locs = ", ".join(f"{t.file}:{t.line}" for t in found)
                errs.append(
                    f"{where}: {req.rid} is annotated {{{ann.kind}}} but IS tested "
                    f"({locs}); the annotation is stale -- remove it"
                )
            continue

        if not req.gated:
            continue

        if ann and ann.kind == "single-polarity":
            other = "positive" if ann.polarity == "negative" else "negative"
            if other in polarities:
                locs = ", ".join(
                    f"{t.file}:{t.line}" for t in found if t.polarity == other
                )
                errs.append(
                    f"{where}: {req.rid} is annotated {{single-polarity: {ann.polarity}}} "
                    f"but a {other} test exists ({locs}); the annotation is stale -- "
                    f"remove it and cover both polarities"
                )
            if ann.polarity not in polarities:
                errs.append(
                    f"{where}: {req.rid} [{req.level}] has no {ann.polarity} test: "
                    f"{req.text[:70]}"
                )
            continue

        if not found:
            errs.append(
                f"{where}: {req.rid} [{req.level}] has no test and no annotation: "
                f"{req.text[:70]}"
            )
            continue

        for missing in sorted(POLARITIES - polarities):
            errs.append(
                f"{where}: {req.rid} [{req.level}] has no {missing} test "
                f"(only {'/'.join(sorted(polarities))}). A {missing}-less test cannot "
                f"distinguish correct behavior from blanket accept/reject. "
                f"Add one, or annotate {{single-polarity: ...; why}}"
            )
    return errs


# --------------------------------------------------------------------------
# Enrolment ratchet
# --------------------------------------------------------------------------
def check_enrolment(
    current: Set[str],
    baseline: Set[str],
    summaries: Set[str],
    newly_enrolled: Optional[Set[str]] = None,
    signed: Optional[Set[str]] = None,
) -> List[str]:
    """Enrolment grows only, never names an RFC we have no summary for, and -- for a
    stem enrolled since HEAD -- requires a valid extraction sign-off.

    `newly_enrolled` is computed by the CALLER rather than as `current - baseline` here,
    because the two have opposite failure polarities. Every other use of `baseline` in
    this function is `baseline - current`, where _git_baseline_enrolment:698 returning an
    empty set on git failure accuses nobody. `current - baseline` against that same empty
    set would accuse all 166 enrolled RFCs of being new -- a wall of violations no
    developer can act on, which teaches people to bypass the gate
    (_git_baseline_summary_stems:763 documents the same trap). None means "could not
    tell", and the precondition is then not evaluated.
    """
    errs: List[str] = []
    if not current:
        errs.append(
            "nothing is enrolled: rfc/enrolled.txt is empty or missing. The gate refuses "
            "to report clean while enforcing nothing (ai/rules/fail-closed-guards.md)"
        )
    for rfc in sorted(baseline - current):
        errs.append(
            f"{rfc} was un-enrolled. Enrolment is monotonic: an RFC whose MUSTs were "
            f"gated cannot stop being gated. Restore it in rfc/enrolled.txt"
        )
    for rfc in sorted(current - summaries):
        errs.append(
            f"{rfc} is enrolled but rfc/short/{rfc}.md does not exist -- there is no "
            f"requirement list to enforce"
        )
    for rfc in sorted(current):
        if source_keyword_count(rfc) is None:
            errs.append(
                f"{rfc} is enrolled but there is no source text at rfc/full/{rfc}.txt or "
                f"rfc/drafts/{rfc}.txt -- without it the summary is validated only against "
                f"itself, so a requirement the RFC does not contain cannot be caught and a "
                f"requirement it does contain can be missing invisibly. Fetch the source "
                f"(https://www.rfc-editor.org/rfc/{rfc}.txt for an RFC; the datatracker "
                f"archive for a draft) before enrolling"
            )
    for rfc in sorted(newly_enrolled or ()):
        if rfc in (signed or set()):
            continue
        errs.append(
            f"{rfc} is newly enrolled with no valid extraction sign-off at "
            f"rfc/extraction/{rfc}.json. Enrolling gates the requirements the summary "
            f"LISTS; nothing bounds what it MISSED until the source text has been walked "
            f"site by site (ai/rules/rfc-compliance.md, Extraction Completeness). Run: "
            f"make ze-rfc-extract STEM={rfc}, then classify every site and section. "
            f"RFCs enrolled before this gate existed are grandfathered and unaffected"
        )
    return errs


def parse_enrolled(text: str) -> Set[str]:
    out: Set[str] = set()
    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        out.add(line.split()[0])
    return out


def _git_baseline_enrolment() -> Optional[Set[str]]:
    """The RFCs enrolled at HEAD, or None when git could not answer.

    None, not set(), and for the same reason as _git_baseline_summary_stems:791: this
    baseline feeds TWO consumers with OPPOSITE polarities. check_enrolment reads it as
    `baseline - current` (the un-enrolment ratchet), where an empty set accuses nobody and
    is safe; run_check derives `current - baseline` from it for the new-enrolment sign-off
    precondition, where an empty set accuses EVERY enrolled RFC of being new -- 166
    violations no developer can act on, which is exactly the wall that teaches people to
    bypass a gate. Only this reader knows which case it is in, so it must not hand both
    consumers the same answer. Each caller says what it does with the unknown.
    """
    try:
        res = subprocess.run(
            ["git", "show", "HEAD:rfc/enrolled.txt"],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return None
    if res.returncode != 0:
        return None
    return parse_enrolled(res.stdout)


def _git_baseline_ids() -> Set[str]:
    """The set of requirement IDs allocated in the committed (HEAD) rfc/short/*.md.

    Mirrors _git_baseline_enrolment: the reuse ratchet (check_id_allocation) needs the ids
    that already existed at HEAD so it can tell "text edit, keep the id" (fine) from
    "id retired, then a DIFFERENT obligation re-points at it" (catastrophic). On the first
    commit, a missing baseline, or any git failure, return an empty set -- the same graceful
    fallback, meaning simply "no reuse baseline to compare against yet".
    """
    try:
        listing = subprocess.run(
            ["git", "ls-tree", "-r", "-z", "--name-only", "HEAD", "rfc/short"],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return set()
    if listing.returncode != 0:
        return set()
    ids: Set[str] = set()
    for path in listing.stdout.split("\0"):
        path = path.strip()
        if not path.endswith(".md"):
            continue
        stem = os.path.basename(path)[: -len(".md")]
        try:
            show = subprocess.run(
                ["git", "show", "HEAD:" + path],
                cwd=PROJECT_DIR,
                capture_output=True,
                text=True,
                check=False,
            )
        except OSError:
            continue
        if show.returncode != 0:
            continue
        try:
            for req in parse_summary_text(show.stdout, stem, source=path):
                ids.add(req.rid)
        except ParseError:
            # A committed summary that no longer parses contributes no baseline ids for its
            # sections; the ratchet skips reuse-checking there rather than crashing the gate.
            continue
    return ids


def _git_baseline_summary_stems() -> Optional[Set[str]]:
    """The summary stems committed at HEAD, or None when git could not answer.

    Tells a summary that is NEW in this change from one that is part of the existing
    backlog.

    Returns None, NOT an empty set, when git fails. Its two siblings can conflate the two
    because they are consumed as `baseline - current` and a high-water map, where an empty
    baseline accuses nobody. This one is consumed as `stems - baseline_stems`, where an
    empty baseline accuses EVERY summary in the repository of being new. Same word,
    opposite polarity: "I could not look" must not render as "nothing was there"
    (ai/rules/fail-closed-guards.md -- and note that failing CLOSED here would mean a wall
    of false violations no developer can act on, which teaches people to bypass the gate).
    """
    try:
        res = subprocess.run(
            ["git", "ls-tree", "-r", "-z", "--name-only", "HEAD", "rfc/short"],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return None
    if res.returncode != 0:
        return None
    stems: Set[str] = set()
    for path in res.stdout.split("\0"):
        path = path.strip()
        if path.endswith(".md"):
            stems.add(os.path.basename(path)[: -len(".md")])
    return stems


def _git_baseline_tag_polarities() -> Dict[str, Set[str]]:
    """Which polarities proved each requirement at HEAD: {rid: {"positive", "negative"}}.

    The baseline is re-parsed with the SAME scan_go_tags/scan_ci_tags the working tree
    goes through, never with a regex over `git grep` output. A .ci `terminator=` block is
    raw file content that can contain a line looking exactly like a tag
    (scan_ci_tags:510), so a regex baseline would invent tags that were never there and
    then report their "loss".

    Only files git already told us contain the marker are read, so the cost tracks the
    number of tagged files, not the size of the repository.
    """
    try:
        listing = subprocess.run(
            ["git", "grep", "-l", "-z", "-F", "RFC requirement:", "HEAD", "--"]
            + list(TEST_ROOTS),
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return {}
    # git grep exits 1 when nothing matched, which is a real answer ("no tags at HEAD");
    # any other non-zero is a failure and yields no baseline.
    if listing.returncode not in (0, 1):
        return {}
    paths = []
    for entry in listing.stdout.split("\0"):
        # NOT stripped: `-z` emits the path verbatim, and stripping would silently rename
        # a path with leading or trailing spaces into one that does not exist.
        # Honest note: no test pins this, because it is not observable here. A path with a
        # leading or trailing space fails the `_test.go` suffix check with or without the
        # strip, so both spellings return the same empty baseline. It is kept as
        # correctness by construction, not as a behaviour someone verified.
        if not entry.startswith("HEAD:"):
            continue
        rel = entry[len("HEAD:") :]
        if not (rel.endswith("_test.go") or rel.endswith(".ci")):
            continue
        # A newline in a path would split one request into two inside the newline-delimited
        # cat-file protocol and desync every following blob. Excluding it costs a baseline
        # entry; including it would corrupt all of them.
        if "\n" in rel:
            continue
        # Mirror scan_tree's pruning (:551) exactly. Without this the baseline can hold a
        # tag from a directory the tree scanner never visits, and the ratchet then reports
        # "no longer proven" for a test that was never removed.
        if any(part in (".git", "vendor", "testdata") for part in rel.split("/")):
            continue
        paths.append(rel)
    if not paths:
        return {}

    out: Dict[str, Set[str]] = {}
    for rel, blob in _git_cat_blobs(paths).items():
        for t in _scan_tags_tolerant(blob, rel):
            out.setdefault(t.rid, set()).add(t.polarity)
    return out


def _scan_tags_tolerant(blob: str, rel: str) -> List[Tag]:
    """Baseline tags for one HEAD blob, surviving a malformed tag elsewhere in the file.

    The production scanners raise on the FIRST bad tag, discarding the whole file's tags.
    For the tree that is right (fail closed on malformed input). For the BASELINE it is
    backwards: the commit that fixes a malformed tag is exactly when the tree parses and
    HEAD does not, so a whole file would lose its baseline in the one change most likely
    to be touching those tests.

    Go falls back to a per-line scan that keeps the tags that parse -- safe because
    scan_go_tags is itself a plain line loop (:501). A .ci does NOT: its terminator= blocks
    make line position meaningful (:510), and re-implementing that here is precisely the
    phantom-tag hazard the shared scanner exists to avoid. A .ci with a malformed tag
    therefore contributes no baseline, and says so.

    The .ci guard is load-bearing, not defensive decoration: the Go fallback matches
    `// RFC requirement:` (_GO_TAG_RE, :148), and a .ci records shell and config content
    where such a line can appear without being a tag at all. Running the Go fallback over a
    .ci would invent exactly the phantom tags this function exists to avoid, and the
    ratchet would then report the "loss" of a tag that never existed.
    """
    scan = scan_go_tags if rel.endswith("_test.go") else scan_ci_tags
    try:
        return scan(blob, rel)
    except ParseError:
        pass
    if not rel.endswith("_test.go"):
        return []
    out: List[Tag] = []
    for i, line in enumerate(blob.split("\n"), start=1):
        m = _GO_TAG_RE.match(line)
        if not m:
            continue
        try:
            t = _parse_tag_rest(m.group("rest"), f"{rel}:{i}")
        except ParseError:
            continue
        out.append(t._replace(file=rel, line=i))
    return out


def _git_cat_blobs(paths: Sequence[str]) -> Dict[str, str]:
    """Read many HEAD blobs in ONE git process: {path: contents}.

    A `git show` per file is the obvious spelling and costs ~350 forks here. Measured on
    this tree: the gate runs 1.7s at HEAD, 3.4s with per-file `git show`, and 2.2s with this
    batch read -- so the baseline costs +0.5s (~30%) instead of +1.7s (~100%). A gate that
    doubles the time of every `make ze-verify` is a gate people learn to skip, so the batch
    interface is a condition of the check being kept rather than an optimization.

    Missing or unreadable paths are simply absent from the result: a blob we cannot read
    contributes no baseline, exactly like a git failure above.
    """
    stdin = "".join(f"HEAD:{p}\n" for p in paths)
    try:
        res = subprocess.run(
            ["git", "cat-file", "--batch"],
            cwd=PROJECT_DIR,
            input=stdin.encode("utf-8"),
            capture_output=True,
            check=False,
        )
    except OSError:
        return {}
    if res.returncode != 0:
        return {}

    out: Dict[str, str] = {}
    data = res.stdout
    pos = 0
    for rel in paths:
        nl = data.find(b"\n", pos)
        if nl < 0:
            return {}  # truncated stream: what we have is a PARTIAL baseline, worse than none
        header = data[pos:nl].decode("utf-8", "replace").split()
        pos = nl + 1
        # "<object> missing" / "<object> ambiguous": git echoes the request and emits NO
        # body, so there is nothing to skip.
        if len(header) == 2 and header[1] in ("missing", "ambiguous"):
            continue
        # Anything else must be "<sha> <type> <size>" followed by <size> bytes and a LF.
        # A non-blob type (a tree, if a caller ever passes a directory) still HAS a body:
        # skipping the header alone would consume that body as the next header and every
        # following path would be silently dropped. Frame it properly, then ignore it.
        if len(header) != 3:
            return {}
        try:
            size = int(header[2])
        except ValueError:
            return {}  # a header we cannot trust means we no longer know where bodies end
        body = data[pos : pos + size]
        pos += size + 1  # trailing newline after each body
        if header[1] == "blob":
            out[rel] = body.decode("utf-8", "replace")
    return out


def check_coverage_ratchet(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    baseline_polarities: Dict[str, Set[str]],
    baseline_enrolled: Set[str],
) -> List[str]:
    """Proof is monotonic: a requirement that was proven cannot stop being proven.

    check_enrolment already ratchets which RFCs are gated. This ratchets the evidence one
    level down, because evaluate() reads only the working tree and a tree cannot tell
    "never proven" from "stopped being proven" -- the second is a regression and the first
    is the backlog.

    Compares the SET of polarities per requirement id, so a refactor that renames or moves
    a test is invisible here and only genuine loss fires. There is deliberately no
    annotation that satisfies this check: {gap} is precisely the move being blocked. The
    honest exits are to restore the test or to retire the requirement id.

    Scoped to RFCs that were already enrolled at HEAD: an RFC enrolled in THIS change is
    judged by evaluate()'s ordinary rules, where a declared {gap} is a legitimate starting
    position rather than a regression.
    """
    errs: List[str] = []
    current: Dict[str, Set[str]] = {}
    for t in tags:
        current.setdefault(t.rid, set()).add(t.polarity)

    seen: Set[str] = set()
    for req in requirements:
        if req.rfc not in enrolled or req.rfc not in baseline_enrolled:
            continue
        was = baseline_polarities.get(req.rid)
        if not was:
            continue
        lost = was - current.get(req.rid, set())
        if not lost:
            continue
        if req.rid in seen:
            continue  # two summary lines sharing an id are one loss, not two
        seen.add(req.rid)
        where = f"{req.source}:{req.line}" if req.source else req.rid
        errs.append(
            f"{where}: {req.rid} is no longer proven -- the {'/'.join(sorted(lost))} "
            f"test(s) that covered it at HEAD are gone. Coverage is monotonic: evidence "
            f"that existed cannot quietly stop existing. Restore the test, or retire the "
            f"requirement id if the obligation itself is gone. An annotation does not "
            f"substitute for proof that was already there"
        )
    return errs


def check_retired_requirements(
    requirements: Sequence[Requirement],
    enrolled: Set[str],
    baseline_ids: Set[str],
    baseline_enrolled: Set[str],
    stems: Set[str],
    baseline_stems: Optional[Set[str]] = None,
    parse_errors: Optional[Dict[str, str]] = None,
) -> List[str]:
    """A requirement id of an enrolled RFC cannot simply vanish from its summary.

    check_coverage_ratchet iterates the CURRENT requirements, so a requirement that is no
    longer listed is never visited and its lost tests are never noticed. That makes
    deleting the checklist line the cheapest possible route from red to green -- cheaper
    than {gap}, which costs a public disclosure row. The ratchet would then be pressuring
    people toward hiding the obligation instead of declaring it, which is worse than not
    having the ratchet at all.

    The id rule already says ids are permanent and never renumbered or reused
    (ai/skills/ze-rfc.md); this makes the deletion half enforceable. Correcting a misquoted
    requirement means editing the TEXT under the same id, which this permits.
    """
    errs: List[str] = []
    live = {r.rid for r in requirements}
    # An id is attributed by matching against EVERY stem we know of, then judged. Matching
    # only against the judged stems would let an id whose real owner is un-enrolled be
    # attributed to whichever enrolled stem happens to share a prefix, and reported against
    # a summary that never held it (DRAFT-FOO-BAR-3.1-1 blamed on rfc/short/draft-foo.md).
    # Longest prefix first: a draft stem is itself hyphenated, so splitting the id on "-"
    # would name the wrong RFC.
    known = sorted(set(stems) | set(baseline_stems or ()), key=len, reverse=True)
    # Two ways a stem contributes no requirements without anything being retired, both of
    # which would otherwise emit one confident wrong message PER ID (39 for rfc7606),
    # burying the single accurate error that is the real problem. Both are reported
    # elsewhere -- parse errors by run_check, a deleted summary by check_enrolment -- so
    # the gate stays red either way.
    silent = set(parse_errors or {}) | (set(baseline_stems or ()) - set(stems))
    judged = (enrolled & baseline_enrolled) - silent
    for rid in sorted(baseline_ids - live):
        stem = next((s for s in known if rid.startswith(rfc_prefix(s) + "-")), None)
        if stem is None or stem not in judged:
            continue
        errs.append(
            f"{rid} was in rfc/short/{stem}.md at HEAD and is now gone. Requirement ids "
            f"are permanent: deleting the line retires the obligation silently, which is "
            f"exactly the move that makes a compliance claim rot. Restore the line (edit "
            f"its TEXT under the same id if the wording was wrong), and annotate it if it "
            f"is not met"
        )
    return errs


def check_new_summaries(
    stems: Set[str],
    baseline_stems: Optional[Set[str]],
    enrolled: Set[str],
    requirements: Sequence[Requirement],
    parse_errors: Dict[str, str],
) -> List[str]:
    """Adding an RFC summary must add RFC checking.

    A new rfc/short/*.md is un-enrolled by definition, so without this the gate learns
    nothing from it: the obligations are written down and enforced nowhere, which is the
    exact shape of a compliance claim that rots.

    Judges only summaries that are NEW since HEAD. The ones that predate it are the
    existing backlog; failing on those would block every unrelated commit and the rule
    would be removed rather than obeyed.

    An absent baseline (None: git could not answer; empty: no rfc/short at HEAD) judges
    NOTHING. `stems - baseline_stems` against an empty baseline is `stems`, i.e. every
    summary in the repository accused of being new -- a wall of violations naming files
    committed years ago, which no developer can act on and which teaches people to bypass
    the gate. "I could not look" must not render as "nothing was there".
    """
    errs: List[str] = []
    if not baseline_stems:
        return errs
    gated_by_rfc: Dict[str, int] = {}
    for req in requirements:
        if req.gated:
            gated_by_rfc[req.rfc] = gated_by_rfc.get(req.rfc, 0) + 1

    for stem in sorted(stems - baseline_stems):
        if stem in enrolled:
            continue
        problem = parse_errors.get(stem)
        if problem:
            # Un-enrolled parse errors are suppressed elsewhere because the id migration is
            # per-RFC and predates most summaries. A NEW summary has no such excuse, and
            # suppressing it would make the enrolment rule below evadable by shipping a
            # summary that simply does not parse.
            errs.append(
                f"rfc/short/{stem}.md is new and does not parse: {problem}. A new summary "
                f"must parse before it can be enrolled"
            )
            continue
        gated = gated_by_rfc.get(stem, 0)
        if gated:
            errs.append(
                f"rfc/short/{stem}.md is new and declares {gated} gated MUST-level "
                f"requirement(s), but is not in rfc/enrolled.txt -- so none of them is "
                f"checked. Enrol it (see .claude/skills/ze-rfc/SKILL.md), classifying each "
                f"requirement as tested, {{single-polarity}}, {{gap}} or {{not-applicable}}"
            )
            continue
        src = source_keyword_count(stem)
        if src:
            errs.append(
                f"rfc/short/{stem}.md is new and declares NO MUST-level requirement, but "
                f"the source text has {src} MUST-level keyword(s). An absent summary is "
                f"indistinguishable from a compliant one: extract the obligations, or "
                f"record in the summary why the source keywords are not requirements on a "
                f"speaker"
            )
    return errs


# --------------------------------------------------------------------------
# Status ledger cross-check
# --------------------------------------------------------------------------
def parse_status_ledger(text: str) -> Dict[str, Dict[str, str]]:
    """Parse docs/features/rfc-status.md rows into {rfc_stem: {status, remaining}}."""
    rows: Dict[str, Dict[str, str]] = {}
    for line in text.split("\n"):
        if not line.startswith("|"):
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        if len(cells) < 3:
            continue
        m = re.match(r"^RFC\s*(\d+)$", cells[0])
        if m:
            key = "rfc" + m.group(1)
        elif re.match(r"^draft-[\w.-]+$", cells[0]):
            # Internet-Draft summaries enroll under their full stem (there is no RFC
            # number to key on), so their status row leads with the draft name and is
            # keyed by that stem -- matching Requirement.rfc for a draft-stem summary.
            # Without this a {gap} on an enrolled draft could never find its disclosure
            # row and would fail check_status_agreement (ai/rules/fail-closed-guards.md).
            key = cells[0]
        elif re.match(r"^[a-z][a-z0-9]*(-[a-z0-9.]+)+$", cells[0]):
            # Non-RFC, non-draft summaries (e.g. sflow-v5) enroll under their file
            # stem, which is a lowercase hyphenated token. Key the status row by that
            # exact stem so a {gap} on such a summary can find its disclosure row --
            # same fail-closed reasoning as the draft branch above.
            key = cells[0]
        else:
            continue
        rows[key] = {
            "status": cells[2],
            "coverage": cells[3] if len(cells) > 3 else "",
            "remaining": cells[4] if len(cells) > 4 else "",
        }
    return rows


def check_status_agreement(
    requirements: Sequence[Requirement],
    rows: Dict[str, Dict[str, str]],
    enrolled: Set[str],
) -> List[str]:
    """A known unmet MUST must not hide behind a clean 'Supported' row.

    Two ledgers that can disagree will disagree. docs/features/rfc-status.md is the
    public claim; a {gap} annotation is a private admission. They must match.
    """
    errs: List[str] = []
    for req in requirements:
        ann = req.annotation
        if not ann or ann.kind != "gap" or req.rfc not in enrolled:
            continue
        row = rows.get(req.rfc)
        if row is None:
            errs.append(
                f"{req.rid} is annotated {{gap}} but {req.rfc} has no row in "
                f"docs/features/rfc-status.md; the public ledger must disclose it"
            )
            continue
        status = row["status"]
        remaining = row["remaining"]
        remaining_stripped = remaining.strip()
        if status.startswith("Supported"):
            # Under a clean 'Supported' claim, ONLY an explicit non-empty gap note in the
            # Remaining column discloses the gap. An empty/whitespace/neutral Remaining does
            # NOT -- that was the fail-open: `_NO_GAP_RE.search("")` is None, so a blank
            # Remaining read as "disclosed" and a {gap} MUST hid behind clean support
            # (ai/rules/fail-closed-guards.md: absence of a claim is not a disclosure).
            discloses = bool(remaining_stripped) and not _NO_GAP_RE.search(
                remaining_stripped
            )
        else:
            # A non-'Supported' status (Partial, Not supported, ...) itself discloses that
            # the RFC is not fully met, so the row is not advertising clean support.
            discloses = True
        if not discloses:
            errs.append(
                f"{req.rid} is annotated {{gap: {ann.reason[:50]}}} but "
                f"docs/features/rfc-status.md says {req.rfc} is '{status}' with "
                f"'{remaining[:40]}'. A known unmet MUST cannot be advertised as clean "
                f"support -- update the row's Status/Remaining"
            )
    return errs


# --------------------------------------------------------------------------
# Fingerprints (drive /ze-rfc-audit staleness)
# --------------------------------------------------------------------------
def _normalize(src: str) -> str:
    lines = [line.strip() for line in src.split("\n")]
    return "\n".join(line for line in lines if line)


def requirement_sha(text: str) -> str:
    return hashlib.sha256(_normalize(text).encode("utf-8")).hexdigest()[:16]


def test_sha(src: str) -> str:
    return hashlib.sha256(_normalize(src).encode("utf-8")).hexdigest()[:16]


def verdict_is_fresh(verdict: Dict, req_sha: str, test_shas: Dict[str, str]) -> bool:
    """A verdict is only fresh while BOTH the requirement text and every tagged test are
    byte-identical to what was audited.

    Biased to over-trigger: a false 'stale' costs a re-read, a false 'fresh' ships a
    test that no longer enforces its requirement.
    """
    return (
        verdict.get("requirement_sha") == req_sha and verdict.get("tests") == test_shas
    )


def load_audit(rfc: str) -> Dict[str, Dict]:
    """Read rfc/audit/<rfc>.json: /ze-rfc-audit's per-requirement verdicts."""
    path = os.path.join(AUDIT_DIR, rfc + ".json")
    if not os.path.exists(path):
        return {}
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, ValueError) as exc:
        raise ParseError(f"rfc/audit/{rfc}.json: cannot read: {exc}") from exc
    return data.get("requirements", {})


def tagged_unit_shas(tags: Sequence[Tag], root: str = PROJECT_DIR) -> Dict[str, str]:
    """Fingerprint each tagged test, keyed file:line.

    Coarse on purpose: the whole enclosing file, not the function. Over-triggering costs a
    re-read; under-triggering ships a verdict for a test that has since changed.
    """
    out: Dict[str, str] = {}
    cache: Dict[str, str] = {}
    for t in tags:
        if t.file not in cache:
            try:
                with open(
                    os.path.join(root, t.file), encoding="utf-8", errors="replace"
                ) as fh:
                    cache[t.file] = test_sha(fh.read())
            except OSError:
                cache[t.file] = ""
        out[f"{t.file}:{t.line}"] = cache[t.file]
    return out


def check_audit_freshness(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
) -> List[str]:
    """A recorded verdict must still describe what it judged.

    This is the hinge between the mechanical half and the semantic half. The gate can prove
    a LINK exists; only a reader can say the test still enforces the requirement's letter
    and spirit. The fingerprint turns "someone should re-read this" into a signal that
    fires exactly when it can have gone wrong.

    A MISSING verdict is not an error: the audit is sampled, the gate is total. But a
    verdict that no longer matches what it judged is worse than none -- it is a stale
    assurance -- so that fails.
    """
    errs: List[str] = []
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)

    audits: Dict[str, Dict[str, Dict]] = {}
    for rfc in sorted(enrolled):
        audits[rfc] = load_audit(rfc)

    for req in requirements:
        if req.rfc not in enrolled:
            continue
        verdict = audits.get(req.rfc, {}).get(req.rid)
        if not verdict:
            continue
        found = by_rid.get(req.rid, [])
        fresh = verdict_is_fresh(
            verdict, requirement_sha(req.text), tagged_unit_shas(found)
        )
        if not fresh:
            errs.append(
                f"{req.source}:{req.line}: {req.rid} has a STALE audit verdict -- the "
                f"requirement text or a tagged test changed since it was judged, so the "
                f"verdict no longer describes what it judged. Re-run: /ze-rfc-audit {req.rfc}"
            )
    return errs


# --------------------------------------------------------------------------
# Ledger render (the generated half of the two-way reference)
# --------------------------------------------------------------------------
_SRC_KEYWORD_RE = re.compile(r"\b(MUST NOT|MUST|SHALL NOT|SHALL)\b")


def source_keyword_count(stem: str) -> Optional[int]:
    """Count MUST-level keywords in the RFC's own text, or None if we don't have it.

    This is the ground truth the summary is supposed to capture. Comparing it against the
    captured count is what exposes a summary that quietly captured nothing.

    Reads through source_text, which owns the two-location lookup this function used to
    inline. One reader, so a source the keyword count can see and the extraction inventory
    cannot (or the reverse) is impossible by construction. Behaviour is unchanged: None
    for an absent file and None for an unreadable one, exactly as before.
    """
    text = source_text(stem)
    if text is None:
        return None
    return len(_SRC_KEYWORD_RE.findall(text))


def unconverted_summaries(captured: Set[str]) -> List[Dict[str, object]]:
    """Summaries that declare no MUST-level requirement, with the source keyword count.

    A summary listing zero obligations is either a genuinely non-normative reference or a
    capture failure. The difference is visible only against the source text: rfc5303,
    rfc5304 and rfc5310 have 23, 13 and 12 MUST-level keywords in rfc/full/ and captured
    ZERO. Reporting these is the point -- an absent summary is indistinguishable from a
    compliant one, which is how a standards claim rots.
    """
    out: List[Dict[str, object]] = []
    for stem in sorted(summary_stems()):
        if stem in captured:
            continue
        out.append({"stem": stem, "src": source_keyword_count(stem)})
    return out


class RFCCoverage(NamedTuple):
    rfc: str
    gated: int
    both: int
    one: int
    annotated: int
    missing: int  # gated requirements with no tag and no annotation

    @property
    def outstanding(self) -> int:
        """Requirements still owed work before this RFC could be enrolled."""
        return self.one + self.missing


def rfc_coverage(
    requirements: Sequence[Requirement], tags: Sequence[Tag]
) -> List[RFCCoverage]:
    """Per-RFC coverage. This is the backlog, derived rather than maintained.

    A hand-kept TODO list of missing tests would rot the moment someone wrote one and
    forgot the list (ai/rules/derive-not-hardcode.md). Counting the tags is the only
    version that cannot lie.
    """
    by_rid: Dict[str, Set[str]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, set()).add(t.polarity)

    by_rfc: Dict[str, List[Requirement]] = {}
    for r in requirements:
        by_rfc.setdefault(r.rfc, []).append(r)

    out: List[RFCCoverage] = []
    for rfc, reqs in by_rfc.items():
        gated = [r for r in reqs if r.gated]
        if not gated:
            continue
        both = one = ann = missing = 0
        for r in gated:
            pol = by_rid.get(r.rid, set())
            if r.annotation:
                ann += 1
            elif pol == POLARITIES:
                both += 1
            elif pol:
                one += 1
            else:
                missing += 1
        out.append(RFCCoverage(rfc, len(gated), both, one, ann, missing))
    return out


def _render_rollup(
    by_rfc: Dict[str, List[Requirement]],
    by_rid: Dict[str, List[Tag]],
    enrolled: Set[str],
) -> List[str]:
    """The actionable view: what is owed, per RFC, nearest-to-enrollable first.

    Without this the backlog exists but is unreadable -- thousands of rows with a `--` in
    them is an inventory, not a worklist.
    """
    flat_tags = [t for tags in by_rid.values() for t in tags]
    flat_reqs = [r for reqs in by_rfc.values() for r in reqs]
    cov = rfc_coverage(flat_reqs, flat_tags)

    # Enrolled first (regressions matter most), then closest to enrollable, so the next
    # RFC worth finishing is always at the top.
    cov.sort(key=lambda c: (c.rfc not in enrolled, c.outstanding, c.rfc))

    total_out = sum(c.outstanding for c in cov)
    ready = [c for c in cov if c.outstanding == 0 and c.rfc not in enrolled]

    out: List[str] = []
    out.append("## Coverage by RFC")
    out.append("")
    out.append(
        f"{total_out} MUST-level requirement(s) still owe work across {len(cov)} summaries. "
        f"**Outstanding** = has only one polarity, or has no test and no annotation; those "
        f"are the tests that do not exist yet."
    )
    out.append("")
    if ready:
        out.append(
            f"**Enrollable now** ({len(ready)}): every MUST-level requirement is already "
            f"covered or annotated, so adding these to `rfc/enrolled.txt` would gate them "
            f"without any new work: "
            + ", ".join(f"`{c.rfc}`" for c in ready[:12])
            + ("..." if len(ready) > 12 else "")
        )
        out.append("")
    out.append(
        "| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | State |"
    )
    out.append("|---|---|---|---|---|---|---|---|")
    for c in cov:
        state = (
            "**enrolled**"
            if c.rfc in enrolled
            else ("enrollable" if c.outstanding == 0 else "backlog")
        )
        out.append(
            f"| `{c.rfc}` | {c.gated} | {c.both} | {c.one} | {c.annotated} | "
            f"{c.missing} | {c.outstanding} | {state} |"
        )
    out.append("")
    return out


def render_ledger(
    requirements: Sequence[Requirement], tags: Sequence[Tag], enrolled: Set[str]
) -> str:
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)

    by_rfc: Dict[str, List[Requirement]] = {}
    for r in requirements:
        by_rfc.setdefault(r.rfc, []).append(r)

    total = sum(1 for r in requirements if r.gated)
    gated_total = sum(1 for r in requirements if r.gated and r.rfc in enrolled)

    out: List[str] = []
    out.append("# RFC Requirement Ledger")
    out.append("")
    out.append(
        "GENERATED by `make ze-rfc-index` -- do not edit. Requirement text is authored in "
        "`rfc/short/*.md`; the test links are derived from `RFC requirement:` tags in the "
        "tests themselves (`ai/rules/derive-not-hardcode.md`)."
    )
    out.append("")
    out.append(
        f"{len(requirements)} requirements across {len(by_rfc)} summaries. "
        f"{total} are MUST-level; {gated_total} of those are enrolled and gated by "
        f"`make ze-rfc-check`."
    )
    out.append("")
    out.append(
        "An RFC is **enrolled** (`rfc/enrolled.txt`) when every MUST-level requirement it "
        "declares is covered by a positive AND a negative test, or annotated. Un-enrolled "
        "RFCs are listed here but not gated: that remainder is tracked, not hidden "
        "(`ai/rules/testing.md`, Back-Fill New Test Types)."
    )
    out.append("")
    out.extend(_render_rollup(by_rfc, by_rid, enrolled))
    out.extend(render_extraction_table(requirements, enrolled))

    for rfc in sorted(by_rfc):
        reqs = by_rfc[rfc]
        state = "enrolled (gated)" if rfc in enrolled else "not enrolled"
        out.append(f"## {rfc_prefix(rfc)} -- {state}")
        out.append("")
        out.append("| Requirement | Level | § | Positive test | Negative test | Note |")
        out.append("|---|---|---|---|---|---|")
        for r in sorted(reqs, key=lambda r: r.rid):
            # Sort by (file, line) so the ledger is byte-stable regardless of the
            # order scan_tree happened to walk the tree in -- os.walk order is
            # filesystem-dependent, so an unsorted render churns across machines and
            # defeats the freshness gate (AC-20).
            found = sorted(by_rid.get(r.rid, []), key=lambda t: (t.file, t.line))
            pos = ", ".join(
                f"`{t.file}:{t.line}`" for t in found if t.polarity == "positive"
            )
            neg = ", ".join(
                f"`{t.file}:{t.line}`" for t in found if t.polarity == "negative"
            )
            note = ""
            if r.annotation:
                note = f"{{{r.annotation.kind}}} {r.annotation.reason}"
            out.append(
                f"| `{r.rid}` | {r.level} | {r.section} | {pos or '--'} | "
                f"{neg or '--'} | {note} |"
            )
        out.append("")

    stale = unconverted_summaries({r.rfc for r in requirements})
    if stale:
        out.append("## Summaries declaring no MUST-level requirement")
        out.append("")
        out.append(
            "These summaries contribute nothing to the ledger. That is correct for a "
            "genuinely non-normative reference, and a capture failure for anything else. "
            "The source column is the count of MUST/MUST NOT/SHALL/SHALL NOT in the RFC's "
            "own text: a non-zero source count with no captured requirement means the "
            "summary needs re-authoring (`/ze-rfc <stem>`) before its RFC can ever be "
            "enrolled."
        )
        out.append("")
        out.append("| Summary | MUST-level keywords in source | Verdict |")
        out.append("|---|---|---|")
        for row in stale:
            src = row["src"]
            if src is None:
                verdict = "no source text under `rfc/full/` -- cannot judge"
                srctxt = "?"
            elif src == 0:
                verdict = "consistent: source declares none"
                srctxt = "0"
            else:
                verdict = "**RE-AUTHOR**: source is normative, summary captured nothing"
                srctxt = str(src)
            out.append(f"| `{row['stem']}` | {srctxt} | {verdict} |")
        out.append("")
    return "\n".join(out)


# --------------------------------------------------------------------------
# Extraction sign-off (plan/spec-rfcgate-1-extraction.md)
# --------------------------------------------------------------------------
# Every check above judges the requirements a summary LISTS. None of them can see an
# obligation nobody wrote down, so a green gate is bounded by what was extracted
# (ai/rules/rfc-compliance.md, "Extraction Completeness"). This half bounds the MISS: a
# per-RFC sign-off that a machine re-checks against the RFC's own text.
#
# Only DISPOSITIONS are authored. Sites, sections, quotes, the register and every
# published count are derived at check time, so an unclassified site cannot be hidden and
# a hand-typed "seen" count cannot exist (ai/rules/derive-not-hardcode.md).
EXTRACTION_DIR = os.path.join(PROJECT_DIR, "rfc", "extraction")
DRAIN_BUDGET_FILE = os.path.join(PROJECT_DIR, "rfc", "drain-budget.txt")

EXTRACTION_SCHEMA_VERSION = 1

# Strongest first. A sign-off may declare the DERIVED register or a WEAKER one; a stronger
# claim than the source supports is refused (AC-9, AC-32).
REGISTERS = ("rfc2119", "prose", "manual-walk")
_REGISTER_STRENGTH = {name: len(REGISTERS) - i for i, name in enumerate(REGISTERS)}

# Closed sets, validated at parse time exactly as ANNOTATION_KINDS:77 is. Anything outside
# them is a ParseError, never a silently tolerated novel value.
EXCLUSION_KINDS = frozenset(
    {
        "not-a-requirement",
        "binds-another-role",
        "duplicate-of",
        "cross-document",
        "advisory-in-context",
    }
)
SECTION_SKIP_KINDS = frozenset(
    {"front-matter", "references", "iana", "acknowledgements", "appendix-non-normative"}
)

SITE_DISPOSITIONS = frozenset({"mapped", "excluded"})
SECTION_DISPOSITIONS = frozenset({"walked", "skipped"})

# The section a site is attributed to when it precedes the first numbered heading. A site
# must never be DROPPED for living in the preamble: that would be a silent hole in the
# very bound this artifact exists to provide.
FRONT_SECTION = "front"

# Site scans. The capitalised set matches GATED_LEVELS:69 (SHOULD/MAY are listed in the
# ledger and may be tagged, but never gate, so they are outside the inventory too). The
# prose set is the same words case-insensitively: it is a strict superset, which is why an
# RFC with capitalised keywords can never derive an EMPTY prose inventory.
_SITE_KEYWORD_RE = re.compile(r"\b(?:MUST NOT|MUST|SHALL NOT|SHALL|REQUIRED)\b")
_SITE_PROSE_RE = re.compile(
    r"\b(?:must not|must|shall not|shall|required)\b", re.IGNORECASE
)

# The RFC 2119 / RFC 8174 key-words paragraph is not an obligation on a speaker; it is the
# document saying how to read its other sentences. Counting it as a site would give every
# RFC in the corpus one guaranteed site to classify as `not-a-requirement`, which is noise
# that teaches a reviewer to skim.
_BOILERPLATE_RE = re.compile(
    r"key\s+words.{0,600}?(?:interpreted|RFC\s*2119|BCP\s*14)"
    r"|interpreted\s+as\s+described\s+in\s+\[?(?:RFC\s*2119|BCP\s*14)",
    re.IGNORECASE | re.DOTALL,
)

# A heading sits at column 0. The numeric form tolerates a missing dot ("2 Requirements");
# the alpha form (appendices) REQUIRES it, because "A speaker MUST ..." at column 0 would
# otherwise read as appendix A.
#
# This pattern OVER-MATCHES, deliberately and unavoidably. RFCs put column-0 attribute
# tables, packet diagrams and tables of contents in the same text stream: rfc2865:2893
# ("1     Exactly one instance of this attribute MUST be present in packet."),
# rfc2869:1742, rfc1195:2625 and sflow-v5:36 all match it, and no pattern can separate
# "3.1  Route Selection" from a table row numbered 3.1 by shape alone. So the derivation is
# built to survive a false match rather than to prevent one: _section_bodies keeps the
# matched line's own text in the body (a false heading never ERASES its sentence) and
# merges a repeated id into one section (a false heading never emits a duplicate the
# artifact parser would refuse, and never restarts the per-section site numbering).
_SECTION_HEADING_RE = re.compile(
    r"^(?:Appendix\s+)?(?:(?P<num>\d+(?:\.\d+)*)\.?|(?P<alpha>[A-Z](?:\.\d+)*)\.)"
    r"[ \t]+(?P<title>\S.*)$"
)

# Page furniture: "<author> <status> [Page N]", a form feed, then the running header.
# Left in place it would land inside any quote whose sentence crosses a page boundary.
_PAGE_FOOTER_RE = re.compile(r"\[Page\s+\d+\]\s*$")

# Sentence boundary: end punctuation, whitespace, then something that starts a sentence.
# Demanding the follower rules out "e.g. the" and "Fig. 3" without an abbreviation list.
_SENTENCE_SPLIT_RE = re.compile(r"(?<=[.!?])\s+(?=[A-Z\"(\[])")


class Site(NamedTuple):
    id: str  # "<section>:<n>"
    quote: str
    section: str


class SectionEntry(NamedTuple):
    id: str
    sites: int


class Inventory(NamedTuple):
    stem: str
    register: str
    source_path: str
    source_sha: str
    sections: List[SectionEntry]
    sites: List[Site]
    keyword_sites: int  # sites the capitalised scan alone would have found


def source_path(stem: str) -> Optional[str]:
    """The repo-relative path of the RFC's own text, or None.

    The SAME two locations, in the same order, that source_keyword_count:1329 searches.
    One lookup, so a source the keyword count can see and the inventory cannot (or the
    reverse) is impossible by construction.
    """
    for sub in ("full", "drafts"):
        rel = os.path.join("rfc", sub, stem + ".txt")
        if os.path.exists(os.path.join(PROJECT_DIR, rel)):
            return rel
    return None


def source_text(stem: str) -> Optional[str]:
    rel = source_path(stem)
    if rel is None:
        return None
    try:
        with open(
            os.path.join(PROJECT_DIR, rel), encoding="utf-8", errors="replace"
        ) as fh:
            return fh.read()
    except OSError:
        return None


def _strip_page_furniture(text: str) -> str:
    """Remove the whole page break: the blank run before the "[Page N]" footer, the footer,
    the form feed, the running header, and the blank run after it.

    Removing only the three furniture LINES is not enough. RFCs break pages mid-sentence,
    and the blank lines bracketing the break would still read as a paragraph boundary to
    _sentences, truncating the quote at "A speaker MUST do the first" and losing the rest
    of the obligation. Collapsing the entire break rejoins the paragraph as it was written.

    The cost, stated rather than hidden: when a page happens to break BETWEEN paragraphs,
    those two paragraphs are joined. _sentences still splits them correctly at the sentence
    punctuation between them; only a paragraph ending without terminal punctuation (a
    figure line, a list item) merges with its successor. That is a far smaller and more
    deterministic loss than truncating every page-crossing obligation.
    """
    out: List[str] = []
    # None = ordinary text; "header" = inside the break, still owed the running header;
    # "blanks" = header consumed, still swallowing the blank run before the text resumes.
    state: Optional[str] = None
    for raw in text.split("\n"):
        line = raw.replace("\f", "")
        if _PAGE_FOOTER_RE.search(line):
            # Rewind over the blank run that separated this footer from the text.
            while out and not out[-1].strip():
                out.pop()
            state = "header"
            continue
        if state is not None:
            if not line.strip():
                continue  # the form-feed line, and the blanks bracketing the header
            if state == "header":
                # ("RFC 7296   IKEv2bis   October 2014") -- exactly one line.
                state = "blanks"
                continue
            state = None  # first real line of the new page: the text resumes here
        out.append(line)
    return "\n".join(out)


def _section_bodies(text: str) -> List[Tuple[str, str]]:
    """[(section id, body)] in first-appearance order, with a leading FRONT_SECTION entry.

    Every id appears EXACTLY ONCE and every input line lands in exactly one body. Both are
    load-bearing, because _SECTION_HEADING_RE over-matches (see its comment):

    * The heading's own TITLE stays in its section's body. Dropping the matched line drops
      whatever it said, and for a false match that is a live obligation deleted from the
      inventory without a word. A real heading's title ("Requirements") carries no
      normative keyword, so keeping it costs nothing; a real heading that DOES ("5.1
      Attributes that MUST be present") yields one site a reviewer excludes, which is the
      safe direction -- an extra site is noise, a missing one is a false green.

    * A repeated id EXTENDS the section it already opened rather than starting a second
      one. Two entries sharing an id emit a duplicate `sections` row that
      parse_extraction_artifact:2000 refuses -- so the generated skeleton could not be
      re-read -- and each body restarted _sites_for's per-section counter at 1, so both
      produced a site "7:1" and _evaluate_extraction's dict silently kept one of them.
    """
    order: List[str] = [FRONT_SECTION]
    bodies: Dict[str, List[str]] = {FRONT_SECTION: []}
    current = FRONT_SECTION
    for line in text.split("\n"):
        m = _SECTION_HEADING_RE.match(line)
        if not m:
            bodies[current].append(line)
            continue
        current = m.group("num") or m.group("alpha")
        if current not in bodies:
            order.append(current)
            bodies[current] = []
        elif bodies[current]:
            # A blank line, so the resumed run is a fresh paragraph to _sentences rather
            # than a continuation of a sentence written elsewhere in the document.
            bodies[current].append("")
        bodies[current].append(m.group("title"))
        bodies[current].append("")
    return [(sid, "\n".join(bodies[sid])) for sid in order]


def _sentences(body: str) -> List[str]:
    """Every sentence of a section body, paragraph by paragraph, whitespace collapsed.

    Paragraph-at-a-time so a sentence never runs across a blank line, and whitespace
    collapsed so the derived quote is stable no matter how the source wrapped it.
    """
    out: List[str] = []
    for para in re.split(r"\n\s*\n", body):
        flat = " ".join(para.split())
        if not flat:
            continue
        out.extend(s.strip() for s in _SENTENCE_SPLIT_RE.split(flat) if s.strip())
    return out


def _sites_for(text: str, pattern: "re.Pattern[str]") -> List[Site]:
    """Every normative sentence, located as <section>:<n> in document order."""
    out: List[Site] = []
    for sid, body in _section_bodies(text):
        n = 0
        for sentence in _sentences(body):
            if not pattern.search(sentence):
                continue
            if _BOILERPLATE_RE.search(sentence):
                continue
            n += 1
            out.append(Site(id=f"{sid}:{n}", quote=sentence, section=sid))
    return out


def derive_register(keyword_sites: int, prose_sites: int, gated: int) -> str:
    """Which keyword register the SOURCE is written in, and therefore what a sign-off can
    be graded against (umbrella D6).

    Derived from the text, never authored: the RFCs that would most benefit from claiming
    the strong grade are exactly the ones whose source cannot support it.
    """
    if keyword_sites and keyword_sites >= gated:
        return "rfc2119"
    if prose_sites:
        return "prose"
    return "manual-walk"


# Keyed on the SOURCE SHA, never on the stem alone. run_check derives the inventory of
# every signed stem three times (signed_extractions for the shared set,
# check_extraction_signoff for the violations, and the ledger render), at ~8.5ms mean and
# ~90ms worst (rfc2328) per RFC -- so a fully drained 166-RFC corpus would add about 4.2s
# to every --check, and _git_cat_blobs:927 records what happens to a gate that doubles
# verify time: people learn to skip it.
#
# The key is every input the derivation reads: the stem, the declared gated count (it
# picks the register), the RAW source bytes, and the RESOLVED source path. A stem-keyed
# memo would hand a second body the first body's inventory -- silently, and in the
# direction of a green gate -- and the tests patch `source_text` to serve different bodies
# for the same stem.
#
# RAW bytes, NOT `requirement_sha`. That fingerprint runs `_normalize`, which strips every
# line and drops blank ones, and the derivation depends on exactly those two things:
# `_SECTION_HEADING_RE` anchors at `^`, so leading whitespace decides whether a line is a
# heading at all, and `_sentences` splits paragraphs on blank lines. "1. Rules\n\n..." and
# "   1. Rules\n\n..." share one normalized sha (9daecc1f191899eb) and derive different
# section sets, so a normalized key served the indented body the flush body's inventory.
#
# The path is in the key because it is not a function of the bytes: the same text found at
# rfc/full/<stem>.txt and at rfc/drafts/<stem>.txt is two different sources, and
# `source-path` is a field of the artifact that `_evaluate_extraction` compares.
#
# What the key does NOT cover, stated rather than implied: the memo is never cleared, so
# it is only sound while the derivation is a pure function of these four. Production reads
# each file once per run; the whole test suite shares one process, which is where a
# too-narrow key shows up first.
_INVENTORY_MEMO: Dict[Tuple[str, int, str, str], Inventory] = {}


def derive_inventory(stem: str, gated: int) -> Optional[Inventory]:
    """The full derived inventory for one stem, or None when we have no source text.

    None is NOT an empty inventory. An empty inventory says "the source states no
    obligations"; None says "I could not look", and the two must never render alike
    (ai/rules/fail-closed-guards.md, the zero-value trap).
    """
    raw = source_text(stem)
    if raw is None:
        return None
    source_sha = requirement_sha(raw)
    rel = source_path(stem) or ""
    # The memo key fingerprints the RAW bytes; `source_sha` is the artifact's freshness
    # key and is deliberately normalized (reflowing an RFC must not invalidate a
    # sign-off). They answer different questions, so they are two different digests.
    memo_key = (stem, gated, hashlib.sha256(raw.encode("utf-8")).hexdigest(), rel)
    memoized = _INVENTORY_MEMO.get(memo_key)
    if memoized is not None:
        return memoized
    stripped = _strip_page_furniture(raw)

    keyword = _sites_for(stripped, _SITE_KEYWORD_RE)
    register = derive_register(len(keyword), 0, gated)
    if register == "rfc2119":
        sites = keyword
    else:
        prose = _sites_for(stripped, _SITE_PROSE_RE)
        register = derive_register(len(keyword), len(prose), gated)
        sites = prose if register == "prose" else []

    counts: Dict[str, int] = {}
    for s in sites:
        counts[s.section] = counts.get(s.section, 0) + 1
    sections = [
        SectionEntry(id=sid, sites=counts.get(sid, 0))
        for sid, _ in _section_bodies(stripped)
    ]

    # Asserted at the PRODUCER, not left for a downstream dict to swallow. A locator is the
    # only handle a reviewer's decision has on a sentence, so two sentences sharing one is
    # an obligation nobody judges: _evaluate_extraction:2315 builds {site.id: site} and the
    # second simply disappears, and parse_extraction_artifact refuses the artifact outright.
    # _section_bodies makes both impossible by construction; this says so if it ever stops.
    for label, ids in (
        ("site locator", [s.id for s in sites]),
        ("section id", [s.id for s in sections]),
    ):
        seen: Set[str] = set()
        for one in ids:
            if one in seen:
                raise ParseError(
                    f"{rel}: the derivation produced duplicate {label} {one!r}. Every "
                    f"derived {label} is unique or the sign-off cannot address the "
                    f"sentence it names -- see _section_bodies"
                )
            seen.add(one)

    inv = Inventory(
        stem=stem,
        register=register,
        source_path=rel,
        source_sha=source_sha,
        sections=sections,
        sites=sites,
        keyword_sites=len(keyword),
    )
    # Memoised only on the way OUT, so a derivation that raised the duplicate-locator guard
    # above is never cached as an answer.
    _INVENTORY_MEMO[memo_key] = inv
    return inv


class ExtractionSite(NamedTuple):
    id: str
    quote: str
    disposition: Optional[str]
    mapped_to: str
    excluded_kind: str
    reason: str


class ExtractionSection(NamedTuple):
    id: str
    sites: int
    disposition: Optional[str]
    skip_kind: str
    reason: str
    unsourced_ids: List[str]


class Extraction(NamedTuple):
    stem: str
    register: str
    source_path: str
    source_sha: str
    signed_off: str
    reviewer: str
    resign_reason: str
    register_reason: str
    sections: List[ExtractionSection]
    sites: List[ExtractionSite]
    path: str

    @property
    def excluded(self) -> int:
        return sum(1 for s in self.sites if s.disposition == "excluded")

    @property
    def mapped(self) -> int:
        return sum(1 for s in self.sites if s.disposition == "mapped")


_ARTIFACT_KEYS = frozenset(
    {
        "schema-version",
        "stem",
        "register",
        "register-reason",
        "source-path",
        "source-sha",
        "signed-off",
        "reviewer",
        "resign-reason",
        "sections",
        "sites",
    }
)
_SITE_KEYS = frozenset(
    {"id", "quote", "disposition", "mapped-to", "excluded-kind", "reason"}
)
_SECTION_KEYS = frozenset(
    {"id", "sites", "disposition", "skip-kind", "reason", "unsourced-ids"}
)


def _str_field(obj: Dict, key: str, where: str, required: bool = True) -> str:
    """A string field. `required=False` accepts absent-or-empty as "", but still refuses a
    wrong TYPE: "not filled in yet" is a legal state, `42` never is."""
    val = obj.get(key)
    if not required and (val is None or (isinstance(val, str) and not val.strip())):
        return ""
    if not isinstance(val, str) or not val.strip():
        raise ParseError(f"{where}: {key!r} must be a non-empty string, got {val!r}")
    return val.strip()


def _reject_unknown_keys(obj: Dict, allowed: Set[str], where: str) -> None:
    """A typo'd key would otherwise read as an ABSENT field, and every field here is
    authored: an absent authored field must never pass silently."""
    unknown = sorted(set(obj) - set(allowed))
    if unknown:
        raise ParseError(
            f"{where}: unknown key(s) {unknown}; expected one of {sorted(allowed)}"
        )


def parse_extraction_artifact(path: str) -> Extraction:
    """Read and validate one rfc/extraction/<stem>.json.

    Every enum is closed and every authored field is required, so a malformed artifact is
    a ParseError -- which run_check turns into a clean exit 2, never a traceback (AC-18).
    An UNCLASSIFIED disposition is NOT a parse error: it is a legal skeleton state that
    the CHECK refuses (AC-3), which is what makes generation unable to produce a pass.
    """
    rel = os.path.relpath(path, PROJECT_DIR)
    want_stem = os.path.basename(path)[: -len(".json")]
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, ValueError) as exc:
        raise ParseError(f"{rel}: cannot read: {exc}") from exc
    if not isinstance(data, dict):
        raise ParseError(f"{rel}: expected a JSON object, got {type(data).__name__}")
    _reject_unknown_keys(data, _ARTIFACT_KEYS, rel)

    version = data.get("schema-version")
    if version != EXTRACTION_SCHEMA_VERSION:
        raise ParseError(
            f"{rel}: schema-version must be {EXTRACTION_SCHEMA_VERSION}, got {version!r}"
        )

    stem = _str_field(data, "stem", rel)
    if stem != want_stem:
        raise ParseError(
            f"{rel}: stem {stem!r} does not match the filename ({want_stem!r}). The "
            f"artifact names the RFC it signs off; the two can never drift apart"
        )

    register = data.get("register")
    if register not in REGISTERS:
        raise ParseError(
            f"{rel}: register {register!r} is missing, empty or unknown; expected one of "
            f"{list(REGISTERS)}. It does NOT default to the strong grade: that would "
            f"publish the weakest sign-off as the strongest"
        )
    # `signed-off`, `reviewer` and `register-reason` are required to SIGN OFF, not to
    # parse: an unsigned skeleton is a legal intermediate state, and the alternative --
    # having the writer invent a date and a reviewer so its own output parses -- would
    # fabricate a sign-off record for a walk nobody performed, which is R-2's failure mode
    # exactly. _evaluate_extraction reports them, so a skeleton still FAILS the check, and
    # fails it with the message that helps (which sites are unclassified).
    register_reason = _str_field(data, "register-reason", rel, required=False)
    signed_off = _str_field(data, "signed-off", rel, required=False)
    reviewer = _str_field(data, "reviewer", rel, required=False)
    resign_reason = _str_field(data, "resign-reason", rel, required=False)

    sections: List[ExtractionSection] = []
    seen_sections: Set[str] = set()
    raw_sections = data.get("sections")
    if not isinstance(raw_sections, list):
        raise ParseError(f"{rel}: 'sections' must be a list")
    for entry in raw_sections:
        if not isinstance(entry, dict):
            raise ParseError(f"{rel}: each section must be an object, got {entry!r}")
        _reject_unknown_keys(entry, _SECTION_KEYS, rel)
        sid = _str_field(entry, "id", rel)
        if sid in seen_sections:
            raise ParseError(f"{rel}: duplicate section {sid!r}")
        seen_sections.add(sid)
        where = f"{rel}: section {sid}"
        count = entry.get("sites")
        if not isinstance(count, int) or isinstance(count, bool) or count < 0:
            raise ParseError(f"{where}: 'sites' must be a non-negative integer")
        disp = entry.get("disposition")
        if disp is not None and disp not in SECTION_DISPOSITIONS:
            raise ParseError(
                f"{where}: disposition {disp!r} is not one of "
                f"{sorted(SECTION_DISPOSITIONS)} (null means unclassified)"
            )
        skip_kind = ""
        reason = _str_field(entry, "reason", where, required=False)
        if disp == "skipped":
            skip_kind = entry.get("skip-kind") or ""
            if skip_kind not in SECTION_SKIP_KINDS:
                raise ParseError(
                    f"{where}: skipped needs a 'skip-kind' from "
                    f"{sorted(SECTION_SKIP_KINDS)}, got {skip_kind!r}"
                )
            if not reason:
                raise ParseError(f"{where}: skipped needs a non-empty 'reason'")
        unsourced = entry.get("unsourced-ids") or []
        if not isinstance(unsourced, list) or not all(
            isinstance(u, str) and u.strip() for u in unsourced
        ):
            raise ParseError(
                f"{where}: 'unsourced-ids' must be a list of requirement ids"
            )
        sections.append(
            ExtractionSection(
                id=sid,
                sites=count,
                disposition=disp,
                skip_kind=skip_kind,
                reason=reason,
                unsourced_ids=[u.strip() for u in unsourced],
            )
        )

    sites: List[ExtractionSite] = []
    seen_sites: Set[str] = set()
    raw_sites = data.get("sites")
    if not isinstance(raw_sites, list):
        raise ParseError(f"{rel}: 'sites' must be a list")
    for entry in raw_sites:
        if not isinstance(entry, dict):
            raise ParseError(f"{rel}: each site must be an object, got {entry!r}")
        _reject_unknown_keys(entry, _SITE_KEYS, rel)
        sid = _str_field(entry, "id", rel)
        if sid in seen_sites:
            raise ParseError(f"{rel}: duplicate site locator {sid!r}")
        seen_sites.add(sid)
        where = f"{rel}: site {sid}"
        quote = _str_field(entry, "quote", where)
        disp = entry.get("disposition")
        if disp is not None and disp not in SITE_DISPOSITIONS:
            raise ParseError(
                f"{where}: disposition {disp!r} is not one of "
                f"{sorted(SITE_DISPOSITIONS)} (null means unclassified)"
            )
        mapped_to = ""
        excluded_kind = ""
        reason = _str_field(entry, "reason", where, required=False)
        if disp == "mapped":
            mapped_to = _str_field(entry, "mapped-to", where)
        elif disp == "excluded":
            excluded_kind = entry.get("excluded-kind") or ""
            if excluded_kind not in EXCLUSION_KINDS:
                raise ParseError(
                    f"{where}: excluded needs an 'excluded-kind' from "
                    f"{sorted(EXCLUSION_KINDS)}, got {excluded_kind!r}"
                )
            if not reason:
                raise ParseError(
                    f"{where}: excluded needs a non-empty 'reason'. A bare exclusion is "
                    f"an escape hatch; say why this sentence binds nothing"
                )
            if excluded_kind == "duplicate-of":
                # `mapped-to` means "the requirement id this site relates to": for a
                # mapping, the id this site PROVES; for a duplicate, the id already
                # captured elsewhere. Naming it is what makes AC-8 checkable at all -- a
                # duplicate that names nothing cannot be compared against anything, and a
                # chain of such could cover an RFC in which nothing is actually mapped.
                mapped_to = _str_field(entry, "mapped-to", where)
        sites.append(
            ExtractionSite(
                id=sid,
                quote=quote,
                disposition=disp,
                mapped_to=mapped_to,
                excluded_kind=excluded_kind,
                reason=reason,
            )
        )

    return Extraction(
        stem=stem,
        register=register,
        source_path=_str_field(data, "source-path", rel),
        source_sha=_str_field(data, "source-sha", rel),
        signed_off=signed_off,
        reviewer=reviewer,
        resign_reason=resign_reason,
        register_reason=register_reason,
        sections=sections,
        sites=sites,
        path=rel,
    )


def extraction_stems() -> Set[str]:
    if not os.path.isdir(EXTRACTION_DIR):
        return set()
    return {
        n[: -len(".json")] for n in os.listdir(EXTRACTION_DIR) if n.endswith(".json")
    }


def load_extractions() -> Dict[str, Extraction]:
    """Every artifact under rfc/extraction/, parsed. Raises on the first malformed one."""
    out: Dict[str, Extraction] = {}
    for stem in sorted(extraction_stems()):
        out[stem] = parse_extraction_artifact(
            os.path.join(EXTRACTION_DIR, stem + ".json")
        )
    return out


def gated_counts(requirements: Sequence[Requirement]) -> Dict[str, int]:
    out: Dict[str, int] = {}
    for req in requirements:
        if req.gated:
            out[req.rfc] = out.get(req.rfc, 0) + 1
    return out


def _artifact_document(
    inv: Inventory, previous: Optional[Extraction]
) -> Dict[str, object]:
    """The skeleton document: every derived field filled, every disposition UNCLASSIFIED
    unless a previous artifact classified the SAME locator carrying the SAME sentence.

    Matching on locator alone would silently re-point a reviewer's decision at a sentence
    they never read when the source moved under it -- the same hazard check_id_allocation
    (:427) exists to stop for requirement ids.
    """
    prev_sites = {s.id: s for s in (previous.sites if previous else [])}
    prev_sections = {s.id: s for s in (previous.sections if previous else [])}

    sites: List[Dict[str, object]] = []
    for site in inv.sites:
        keep = prev_sites.get(site.id)
        if keep is not None and keep.quote != site.quote:
            keep = None
        entry: Dict[str, object] = {
            "id": site.id,
            "quote": site.quote,
            "disposition": keep.disposition if keep else None,
        }
        if keep and keep.disposition == "mapped":
            entry["mapped-to"] = keep.mapped_to
        if keep and keep.disposition == "excluded":
            entry["excluded-kind"] = keep.excluded_kind
            entry["reason"] = keep.reason
        sites.append(entry)

    sections: List[Dict[str, object]] = []
    for sec in inv.sections:
        keep = prev_sections.get(sec.id)
        entry = {
            "id": sec.id,
            "sites": sec.sites,
            "disposition": keep.disposition if keep else None,
        }
        if keep and keep.disposition == "skipped":
            entry["skip-kind"] = keep.skip_kind
            entry["reason"] = keep.reason
        if keep and keep.unsourced_ids:
            entry["unsourced-ids"] = keep.unsourced_ids
        sections.append(entry)

    doc: Dict[str, object] = {
        "schema-version": EXTRACTION_SCHEMA_VERSION,
        "stem": inv.stem,
        # The DERIVED register unless a previous artifact declared a WEAKER one. Declaring
        # weaker is legal (AC-9) and a refresh must not silently promote it back.
        "register": (
            previous.register
            if previous
            and _REGISTER_STRENGTH.get(previous.register, 0)
            <= _REGISTER_STRENGTH[inv.register]
            else inv.register
        ),
        "source-path": inv.source_path,
        "source-sha": inv.source_sha,
        "signed-off": previous.signed_off if previous else "",
        "reviewer": previous.reviewer if previous else "",
    }
    if previous and previous.register_reason:
        doc["register-reason"] = previous.register_reason
    if previous and previous.resign_reason:
        doc["resign-reason"] = previous.resign_reason
    doc["sections"] = sections
    doc["sites"] = sites
    return doc


# Every source stem in the corpus (179 of them, RFCs and drafts alike) matches this.
# Validated because `stem` reaches os.path.join for BOTH the source lookup and the artifact
# path, and the Makefile passes $(STEM) unquoted: `--extract-skeleton '../full/rfc4271'`
# resolves a real source and would write outside rfc/extraction/.
#
# `\Z`, never `$`: Python's `$` matches BEFORE a trailing newline, so `^...$` under
# re.match accepted "rfc4271\n" and wrote a file literally named "rfc4271\n.json". Not a
# traversal -- that is the one shape `$` lets through -- but a validator for a filename
# that admits a line terminator is not one. `^` is kept beside it so the pattern stays
# anchored at BOTH ends under `.search()` as well as `.match()`.
_STEM_RE = re.compile(r"^[a-z0-9][a-z0-9._-]*\Z")


def _validated_stem(stem: str) -> str:
    if not _STEM_RE.match(stem) or ".." in stem:
        raise ParseError(
            f"stem {stem!r} is not an RFC or draft stem (lowercase letters, digits, "
            f"'.', '-', '_'; no path separator). The stem names the source text and the "
            f"artifact file, so it may never carry a path"
        )
    return stem


# The writer stages beside its target (see run_extract_skeleton) so `os.replace` is an
# atomic same-filesystem rename. The cost is litter: a kill between `mkdtemp` and the
# `finally` leaves a `.staging-*` directory inside TRACKED rfc/extraction/, which the next
# `git add` would commit. Staging somewhere gitignored instead is not the fix -- `tmp/`
# becomes a symlink to $TMPDIR under `make ze-migrate-scratch`, so the rename could cross
# a filesystem and raise EXDEV -- so the litter is swept by the next run.
_STAGING_PREFIX = ".staging-"

# Why the sweep is AGE-GATED rather than unconditional. An unconditional sweep would
# delete a CONCURRENT run's in-flight staging directory, and that run's
# `parse_extraction_artifact` would then raise FileNotFoundError -- an OSError, outside
# run_extract_skeleton's ParseError handler, i.e. a traceback where the shipped code exits
# cleanly. A live staging directory exists for milliseconds; an abandoned one is minutes
# old before anyone runs the writer again, so the gate separates the two with room to
# spare and never trades one defect for a worse one.
_STAGING_STALE_SECONDS = 3600


def _sweep_stale_staging(directory: str) -> None:
    """Remove abandoned `.staging-*` directories left by a killed run. Best-effort: this
    is hygiene, and a sweep that cannot read the directory must never stop a write."""
    cutoff = time.time() - _STAGING_STALE_SECONDS
    try:
        names = os.listdir(directory)
    except OSError:
        return
    for name in names:
        if not name.startswith(_STAGING_PREFIX):
            continue
        candidate = os.path.join(directory, name)
        try:
            if not os.path.isdir(candidate) or os.path.getmtime(candidate) > cutoff:
                continue
        except OSError:
            continue
        shutil.rmtree(candidate, ignore_errors=True)


def run_extract_skeleton(stem: str) -> int:
    stem = _validated_stem(stem)
    inv = derive_inventory(stem, gated_counts(_summary_requirements(stem)).get(stem, 0))
    if inv is None:
        print(
            f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: {stem} has no source text "
            f"at rfc/full/{stem}.txt or rfc/drafts/{stem}.txt. Fetch it "
            f"(https://www.rfc-editor.org/rfc/{stem}.txt) before extracting: with no "
            f"source there is no inventory to derive and no register to sign under"
        )
        return 2

    path = os.path.join(EXTRACTION_DIR, stem + ".json")
    previous = parse_extraction_artifact(path) if os.path.exists(path) else None
    doc = _artifact_document(inv, previous)

    os.makedirs(EXTRACTION_DIR, exist_ok=True)
    # Round-trip through the REAL parser before the file lands, and stage it beside the
    # target so a refusal leaves the reviewer's existing artifact untouched
    # (ai/rules/never-destroy-work.md).
    #
    # Re-validating with a second, hand-written checker would only prove the copy agrees
    # with itself. The one question that matters is whether parse_extraction_artifact will
    # accept this file, and the honest way to answer it is to run it. Without this, a
    # derivation defect made `make ze-rfc-extract STEM=rfc2865` print success over a file
    # that could not be re-read, and one such artifact committed makes every later --check
    # exit "cannot run", hiding every other RFC violation in the repository.
    #
    # Swept before staging, never after: the litter this clears is what a KILLED run left,
    # and a killed run never reaches its own cleanup by definition (_STAGING_PREFIX).
    _sweep_stale_staging(EXTRACTION_DIR)
    staging = tempfile.mkdtemp(prefix=_STAGING_PREFIX, dir=EXTRACTION_DIR)
    try:
        staged = os.path.join(staging, stem + ".json")
        with open(staged, "w", encoding="utf-8") as fh:
            json.dump(doc, fh, indent=2)
            fh.write("\n")
        try:
            parse_extraction_artifact(staged)
        except ParseError as exc:
            reason = str(exc).replace(
                os.path.relpath(staged, PROJECT_DIR), os.path.relpath(path, PROJECT_DIR)
            )
            print(
                f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: the skeleton derived for "
                f"{stem} does not satisfy the artifact schema, so it was NOT written: "
                f"{reason}\n"
                f"This is a defect in the derivation, not in the source text. Nothing was "
                f"changed on disk; fix scripts/dev/rfc_requirements.py and re-run"
            )
            return 2
        os.replace(staged, path)
    finally:
        shutil.rmtree(staging, ignore_errors=True)

    unclassified = sum(1 for s in doc["sites"] if s["disposition"] is None)
    unwalked = sum(1 for s in doc["sections"] if s["disposition"] is None)
    print(
        f"{GREEN}wrote{RESET} {os.path.relpath(path, PROJECT_DIR)}: "
        f"register {doc['register']}, {len(inv.sites)} site(s) in "
        f"{len(inv.sections)} section(s).\n"
        f"{YELLOW}{unclassified} site(s) and {unwalked} section(s) are UNCLASSIFIED{RESET} "
        f"-- `make ze-rfc-check` fails until every one is classified by hand. Generation "
        f"cannot produce a sign-off; only a walk can."
    )
    return 0


def _summary_requirements(stem: str) -> List[Requirement]:
    """One summary's requirements, or none when it is absent or does not parse.

    A parse failure is reported by run_check against the same file, so swallowing it here
    only affects the skeleton's declared-gated count -- which lands the register on the
    WEAKER side (gated 0 makes rfc2119 easier to derive, so re-check governs).
    """
    path = os.path.join(SUMMARY_DIR, stem + ".md")
    if not os.path.exists(path):
        return []
    try:
        return list(parse_summary_file(path))
    except ParseError:
        return []


def _evaluate_extraction(
    art: Extraction, inv: Inventory, reqs: Sequence[Requirement]
) -> List[str]:
    """Judge ONE sign-off against the freshly re-derived inventory.

    Forward (catches a MISSED obligation): every derived site is mapped or excluded, and
    every derived field still matches what the source re-derives.
    Reverse (catches an INVENTED one): every gated requirement of the summary is the
    target of some site, or declared unsourced on some section.
    """
    errs: List[str] = []
    where = art.path

    if not art.signed_off:
        errs.append(
            f"{where}: 'signed-off' is empty. A skeleton is not a sign-off: record the "
            f"date the walk was performed once every site and section is classified"
        )
    if not art.reviewer:
        errs.append(
            f"{where}: 'reviewer' is empty. A sign-off names who performed the walk"
        )
    if art.register == "manual-walk" and not art.register_reason:
        errs.append(
            f"{where}: a manual-walk sign-off needs a 'register-reason' stating why no "
            f"mechanical inventory exists for this source. The gate cannot verify a "
            f"manual walk, so the assertion must at least say what it rests on"
        )

    if art.source_sha != inv.source_sha:
        # One accurate error, not a wall. With the source moved, every site and every
        # section would mismatch too, and the only useful message is this one. Same bias
        # verdict_is_fresh:1231 records: a false stale costs a re-read, a false fresh
        # ships an unbounded summary.
        return errs + [
            f"{where}: source-sha no longer matches {inv.source_path}. The source text "
            f"changed under this sign-off, so the walk no longer bounds what the summary "
            f"missed. Re-run: make ze-rfc-extract STEM={art.stem}, re-classify any site "
            f"that moved, and bump signed-off"
        ]
    if art.source_path != inv.source_path:
        errs.append(
            f"{where}: source-path {art.source_path!r} is not where the source text lives "
            f"({inv.source_path!r})"
        )

    claimed = _REGISTER_STRENGTH[art.register]
    if claimed > _REGISTER_STRENGTH[inv.register]:
        errs.append(
            f"{where}: register {art.register!r} is STRONGER than the source supports "
            f"({inv.register!r}: {inv.keyword_sites} capitalised keyword site(s) against "
            f"{sum(1 for r in reqs if r.gated)} gated requirement(s) declared). The "
            f"register is a property of the source, not a claim the signer may assert. "
            f"Sign under {inv.register!r} or weaker"
        )

    derived_sites = {s.id: s for s in inv.sites}
    art_sites = {s.id: s for s in art.sites}
    for sid in sorted(set(derived_sites) - set(art_sites)):
        errs.append(
            f"{where}: derived site {sid} is absent from the sign-off "
            f"({derived_sites[sid].quote[:80]!r}). Re-run make ze-rfc-extract "
            f"STEM={art.stem} and classify it"
        )
    for sid in sorted(set(art_sites) - set(derived_sites)):
        errs.append(
            f"{where}: site {sid} is not in the derived inventory. Sites are DERIVED from "
            f"the source text; a hand-added locator classifies nothing"
        )

    by_id = {r.rid: r for r in reqs}
    known_ids = set(by_id)
    mapped_targets = {
        s.mapped_to for s in art.sites if s.disposition == "mapped" and s.mapped_to
    }

    for sid in sorted(set(derived_sites) & set(art_sites)):
        site, derived = art_sites[sid], derived_sites[sid]
        if site.quote != derived.quote:
            errs.append(
                f"{where}: site {sid} quote does not match the source. Derived: "
                f"{derived.quote[:90]!r}. Recorded: {site.quote[:90]!r}. The quote is a "
                f"DERIVED field; editing it hides what the reviewer is meant to judge"
            )
        if site.disposition is None:
            errs.append(
                f"{where}: site {sid} is UNCLASSIFIED: {derived.quote[:110]!r}. Every "
                f"derived site is mapped to a requirement id or excluded with a reason"
            )
            continue
        if site.mapped_to and site.mapped_to not in known_ids:
            errs.append(
                f"{where}: site {sid} names {site.mapped_to}, which does not exist in "
                f"rfc/short/{art.stem}.md"
            )
            continue  # every check below reads by_id[site.mapped_to]

        if site.disposition == "mapped":
            # The site's LEVEL against the row's. Both facts were already here and neither
            # was compared: `known_ids` holds every level, so a sentence quoting a
            # capitalised MUST could be mapped to a SHOULD row and reported as captured --
            # while evaluate() never gates a SHOULD, so the obligation was bound by nobody.
            # That is the RFC's own MUST downgraded to advice inside the artifact whose
            # whole purpose is to bound what the summary understated.
            #
            # Only a CAPITALISED keyword triggers it, and the DERIVED quote is what is
            # read, never the recorded one. A `prose` site's lowercase modal asserts
            # nothing about level, and demanding a gated target there would refuse that
            # register's ordinary output.
            req = by_id[site.mapped_to]
            if _SITE_KEYWORD_RE.search(derived.quote) and not req.gated:
                errs.append(
                    f"{where}: site {sid} quotes a MUST-level keyword but maps to "
                    f"{site.mapped_to} [{req.level}], which is advisory and never gates: "
                    f"{derived.quote[:90]!r}. Either the summary row understates the "
                    f"source and its level is wrong, or this site belongs to a different "
                    f"row -- an obligation recorded as captured but proven by nothing is "
                    f"the miss this sign-off exists to make impossible"
                )
        elif (
            site.excluded_kind == "duplicate-of"
            and site.mapped_to not in mapped_targets
        ):
            errs.append(
                f"{where}: site {sid} is excluded duplicate-of {site.mapped_to}, but no "
                f"other site MAPS that id. A chain of duplicates cannot cover an RFC in "
                f"which nothing is actually mapped"
            )

    derived_sections = {s.id: s.sites for s in inv.sections}
    art_sections = {s.id: s for s in art.sections}
    for sid in sorted(set(derived_sections) - set(art_sections)):
        errs.append(f"{where}: derived section {sid} is absent from the sign-off")
    for sid in sorted(set(art_sections) - set(derived_sections)):
        errs.append(f"{where}: section {sid} is not in the derived section list")
    for sid in sorted(set(derived_sections) & set(art_sections)):
        sec = art_sections[sid]
        if sec.sites != derived_sections[sid]:
            errs.append(
                f"{where}: section {sid} records {sec.sites} site(s); the source derives "
                f"{derived_sections[sid]}. The count is a DERIVED field"
            )
        if sec.disposition is None:
            errs.append(
                f"{where}: section {sid} is UNCLASSIFIED. Every section of the source is "
                f"walked, or skipped with a kind and a reason"
            )

    unsourced = {u for s in art.sections for u in s.unsourced_ids}
    for u in sorted(unsourced - known_ids):
        errs.append(
            f"{where}: unsourced-ids names {u}, which does not exist in "
            f"rfc/short/{art.stem}.md"
        )
    backed = mapped_targets | unsourced
    for req in reqs:
        if not req.gated or req.rid in backed:
            continue
        errs.append(
            f"{where}: {req.rid} [{req.level}] is declared by rfc/short/{art.stem}.md but "
            f"no source site maps to it and no section lists it in unsourced-ids: "
            f"{req.text[:70]}. Either it is backed by a site the walk should map, or it "
            f"was read from indicative prose -- say which"
        )
    return errs


def evaluate_extractions(
    requirements: Sequence[Requirement],
) -> Tuple[Dict[str, Extraction], List[str]]:
    """(valid sign-offs by stem, violations).

    Only an artifact with ZERO violations counts as signed. A stale or contradicted
    sign-off must not keep earning drain credit while the basis under it has moved
    (umbrella "How it fails closed").
    """
    by_rfc: Dict[str, List[Requirement]] = {}
    for req in requirements:
        by_rfc.setdefault(req.rfc, []).append(req)

    # run_check evaluates this THREE times (signed_extractions for the shared set,
    # check_extraction_signoff for the violations, and check_ledger_fresh's render), and
    # each evaluation re-derives the inventory of every SIGNED stem. derive_inventory is
    # memoised on (stem, gated, source sha) for exactly this: the repeats are free, and the
    # sha in the key is what lets the tests keep serving different bodies for one stem.
    signed: Dict[str, Extraction] = {}
    errs: List[str] = []
    gated = gated_counts(requirements)
    for stem, art in load_extractions().items():
        inv = derive_inventory(stem, gated.get(stem, 0))
        if inv is None:
            errs.append(
                f"{art.path}: {stem} has no source text at rfc/full/{stem}.txt or "
                f"rfc/drafts/{stem}.txt, so the sign-off cannot be re-derived and the "
                f"bound it claims cannot be re-checked"
            )
            continue
        found = _evaluate_extraction(art, inv, by_rfc.get(stem, []))
        errs.extend(found)
        if not found:
            signed[stem] = art
    return signed, errs


def check_extraction_signoff(requirements: Sequence[Requirement]) -> List[str]:
    return evaluate_extractions(requirements)[1]


def signed_extractions(requirements: Sequence[Requirement]) -> Dict[str, Extraction]:
    return evaluate_extractions(requirements)[0]


class BaselineExtraction(NamedTuple):
    """One artifact as it stands at HEAD: what the ratchets compare the working tree to."""

    excluded: int
    signed_off: str
    resign_reason: str


def _git_baseline_extractions() -> Optional[Dict[str, BaselineExtraction]]:
    """{stem: BaselineExtraction} at HEAD, or None when git could not answer.

    `resign_reason` is carried because a rise in exclusions must be justified by a reason
    written for THAT rise. _artifact_document copies the field forward on every refresh, so
    once set it is permanently non-empty and comparing it against "" proves nothing;
    comparing it against HEAD is what makes "this walk was redone" checkable.

    Reads through `git ls-tree` plus _git_cat_blobs:899, the same batch path
    _git_baseline_ids:714 uses -- one git process for every artifact rather than one per
    file. No second git reader is introduced.

    Returns None on git failure, and the distinction matters less here than it does at
    _git_baseline_summary_stems:763 -- but it is stated rather than assumed. This
    baseline is consumed as `baseline - current`, where an EMPTY baseline accuses nobody,
    so an empty set would also have been safe. None is used anyway so the two cases stay
    distinguishable to a future consumer whose polarity might be the other one.
    """
    try:
        listing = subprocess.run(
            ["git", "ls-tree", "-r", "-z", "--name-only", "HEAD", "rfc/extraction"],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return None
    if listing.returncode != 0:
        return None

    paths = [
        p.strip()
        for p in listing.stdout.split("\0")
        if p.strip().endswith(".json") and "\n" not in p
    ]
    if not paths:
        return {}

    out: Dict[str, BaselineExtraction] = {}
    for rel, blob in _git_cat_blobs(paths).items():
        stem = os.path.basename(rel)[: -len(".json")]
        try:
            data = json.loads(blob)
            sites = data.get("sites") or []
            excluded = sum(
                1
                for s in sites
                if isinstance(s, dict) and s.get("disposition") == "excluded"
            )
            signed_off = data.get("signed-off") or ""
            resign_reason = data.get("resign-reason") or ""
        except (ValueError, AttributeError, TypeError):
            # A committed artifact that no longer parses contributes no baseline for its
            # stem, exactly as _git_baseline_ids:756 skips an unparseable summary. The
            # working-tree copy is judged by check_extraction_signoff either way.
            continue
        out[stem] = BaselineExtraction(
            excluded=excluded,
            signed_off=signed_off if isinstance(signed_off, str) else "",
            resign_reason=resign_reason if isinstance(resign_reason, str) else "",
        )
    return out


def check_extraction_ratchet() -> List[str]:
    """The signed set only grows, and a signed stem's exclusions only shrink.

    Without the first, a sign-off could be deleted the moment it became inconvenient and
    the bound would silently un-bound. Without the second, R-1's failure arrives: every
    unmapped site excluded with a shrug, until the exclusion list is a 1600-slot escape
    hatch. The pressure is directional and the state is published, rather than a numeric
    threshold that rewording would game.
    """
    baseline = _git_baseline_extractions()
    if baseline is None:
        return []  # git could not answer: judge nothing rather than accuse everything

    errs: List[str] = []
    current: Dict[str, Extraction] = {}
    for stem in sorted(extraction_stems()):
        try:
            current[stem] = parse_extraction_artifact(
                os.path.join(EXTRACTION_DIR, stem + ".json")
            )
        except ParseError:
            # Reported by check_extraction_signoff against the same file; the gate is red
            # either way, and one accurate message beats two.
            continue

    for stem in sorted(set(baseline) - set(current)):
        errs.append(
            f"{stem} had an extraction sign-off at HEAD and has none now. Extraction "
            f"sign-off is monotonic: an RFC whose source walk bounded its summary cannot "
            f"stop being bounded. Restore rfc/extraction/{stem}.json"
        )

    for stem in sorted(set(baseline) & set(current)):
        was = baseline[stem]
        art = current[stem]
        if art.excluded <= was.excluded:
            continue
        if not art.resign_reason:
            errs.append(
                f"{art.path}: exclusions rose from {was.excluded} to {art.excluded} with "
                f"no 'resign-reason'. Exclusions are shrink-only: a rise means the walk "
                f"was redone, so record why, name the reviewer, and bump signed-off"
            )
        elif art.signed_off == was.signed_off:
            errs.append(
                f"{art.path}: exclusions rose from {was.excluded} to {art.excluded} with "
                f"a resign-reason but the same signed-off date ({art.signed_off}). A "
                f"re-sign is a new walk; reusing the old date says it did not happen"
            )
        elif art.resign_reason == was.resign_reason:
            # The reason must be written for THIS rise. `_artifact_document` carries the
            # field forward on every refresh, so once set it is permanently non-empty and
            # a mere presence test guards nothing: exclusions could climb indefinitely
            # behind one sentence written years earlier, with only the date to edit.
            errs.append(
                f"{art.path}: exclusions rose from {was.excluded} to {art.excluded}, but "
                f"'resign-reason' is unchanged from the previous sign-off "
                f"({was.resign_reason[:60]!r}). It is carried forward automatically by "
                f"make ze-rfc-extract, so an unchanged reason justifies the EARLIER walk, "
                f"not this one. Say what this walk found that raised the exclusions"
            )
    return errs


def credited(
    signed: Dict[str, Extraction], enrolled: Set[str]
) -> Dict[str, Extraction]:
    """The sign-offs that COUNT: the ones whose stem is enrolled.

    Every published figure and every comparison is derived from this, never from `signed`
    directly, so credit and backlog always describe the same set. They did not: the drain
    floor compared `len(signed)` (every valid artifact) against a backlog of
    `enrolled - signed` (enrolled only), so a sign-off for a stem nobody enrolled raised
    the credit without lowering the backlog. Measured on 8 enrolled with 6 un-enrolled
    sign-offs: floor satisfied, backlog still 8 of 8, envelope publishing
    `signed + backlog > enrolled` -- a figure no set can have. Eleven un-enrolled source
    texts already sit in rfc/full and rfc/drafts.

    Signing BEFORE enrolling stays the normal workflow (AC-1 makes the sign-off a
    precondition of enrolment). Such an artifact is still parsed, still ratcheted, and
    starts counting the moment its stem enrols -- it simply is not credit yet.
    """
    return {stem: art for stem, art in signed.items() if stem in enrolled}


def register_counts(signed: Dict[str, Extraction]) -> Dict[str, int]:
    """{register: count}, with EVERY register present even at zero.

    A register missing from the split reads as "not a thing" rather than as "zero", and
    the split is the counterweight that keeps owner ruling 1's credit half honest.
    """
    counts = {name: 0 for name in REGISTERS}
    for art in signed.values():
        counts[art.register] = counts.get(art.register, 0) + 1
    return counts


def _register_phrase(counts: Dict[str, int]) -> str:
    return ", ".join(f"{name} {counts[name]}" for name in REGISTERS)


def extraction_status(
    requirements: Sequence[Requirement], enrolled: Set[str]
) -> Dict[str, object]:
    """The counts the umbrella's drain quota consumes (umbrella "Where the counter lives").

    Every figure is DERIVED from rfc/extraction/ plus the live summaries. There is no
    second hand-kept list of who has been signed off: that is the rotting registry
    ai/rules/derive-not-hardcode.md forbids, and the 2026-07-20 ruling in
    plan/deferrals/rfc-gate-regression-ratchets.md already refused that artifact shape.
    """
    signed = credited(signed_extractions(requirements), enrolled)
    counts = register_counts(signed)
    unsigned = sorted(enrolled - set(signed))
    return {
        "schema-version": EXTRACTION_SCHEMA_VERSION,
        "enrolled": len(enrolled),
        # Counted INDEPENDENTLY of the split, not as sum(counts.values()). Defining the
        # total as the sum makes AC-22's "the keys sum to the published total" a tautology
        # that no test could ever fail -- mutation-verified: zeroing register_counts left
        # that assertion green. Two derivations that must agree is a real cross-check, and
        # a register dropped from the split now disagrees with the total out loud.
        "signed": len(signed),
        "signed-by-register": counts,
        "backlog": len(unsigned),
        "unsigned": unsigned,
    }


def run_extraction_status() -> int:
    try:
        enrolled, reqs, _, _, _ = _collect_for_check()
        env = extraction_status(reqs, enrolled)
    except (ParseError, OSError) as exc:
        print(f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: {exc}")
        return 2
    print(json.dumps(env, indent=2, sort_keys=True))
    return 0


def render_extraction_table(
    requirements: Sequence[Requirement], enrolled: Set[str]
) -> List[str]:
    """The published backlog: how much of the standards claim is BOUNDED (AC-15, AC-21).

    Derived columns are shown only for a stem that HAS a sign-off. That is both honest and
    affordable. Honest, because a register derived for a stem nobody has walked is not a
    fact this repository has established. Affordable, because deriving the inventory for
    all 166 enrolled RFCs costs 1.9s measured, on top of a 2.6s gate, on EVERY --check
    (check_ledger_fresh re-renders to compare) -- and _git_cat_blobs:899 records what
    happens to a gate that doubles verify time: people learn to skip it.
    """
    signed = credited(signed_extractions(requirements), enrolled)
    counts = register_counts(signed)
    gated = gated_counts(requirements)
    unsigned = sorted(enrolled - set(signed))

    out: List[str] = []
    out.append("## Extraction sign-off")
    out.append("")
    out.append(
        "Every other table here judges the requirements a summary LISTS. None of them can "
        "see an obligation nobody wrote down, so a green gate is bounded by what was "
        "extracted (`ai/rules/rfc-compliance.md`, Extraction Completeness). A sign-off "
        "(`rfc/extraction/<stem>.json`) bounds the MISS: every normative site of the RFC's "
        "own text is mapped to a requirement id or excluded with a reason, and the gate "
        "re-derives the inventory and re-checks the arithmetic on every run."
    )
    out.append("")
    # NEVER a bare total: umbrella D6's publishing half. Reading "N signed off" as
    # "N keyword-verified" is the category error this whole spec set exists to correct.
    out.append(
        f"Signed off by register: {_register_phrase(counts)}. "
        f"Unsigned (grandfathered) backlog: {len(unsigned)} of {len(enrolled)} enrolled. "
        f"Every register counts toward the drain quota; each is published apart so a "
        f"count can never be read as stronger evidence than it is."
    )
    out.append("")
    out.append(
        "`Register` is DERIVED from the source text and refused when an artifact claims a "
        "stronger grade than the derivation supports: `rfc2119` (capitalised keywords, at "
        "least as many sites as the summary declares gated rows), `prose` (lowercase "
        "indicative modals), `manual-walk` (no mechanical inventory exists at all -- an "
        "assertion the gate cannot verify). Derived columns are blank for an unsigned "
        "stem: nobody has walked it, so there is nothing established to publish."
    )
    out.append("")
    out.append(
        "| RFC | Register | Sites | Mapped | Excluded | Exclusion ratio | Gated rows | Signed off |"
    )
    out.append("|---|---|---|---|---|---|---|---|")
    for stem in sorted(signed):
        art = signed[stem]
        total = len(art.sites)
        ratio = f"{art.excluded / total:.2f}" if total else "--"
        out.append(
            f"| `{stem}` | {art.register} | {total} | {art.mapped} | {art.excluded} | "
            f"{ratio} | {gated.get(stem, 0)} | {art.signed_off} ({art.reviewer}) |"
        )
    for stem in unsigned:
        out.append(
            f"| `{stem}` | -- | -- | -- | -- | -- | {gated.get(stem, 0)} | "
            f"UNSIGNED (grandfathered) |"
        )
    out.append("")
    return out


# --------------------------------------------------------------------------
# Drain floor: the COMPARISON. The POLICY it reads is authored in
# rfc/drain-budget.txt and owned by the umbrella, and ultimately by Thomas.
# --------------------------------------------------------------------------
_BUDGET_KEYS = ("start", "rate")


class DrainBudget(NamedTuple):
    start: "datetime.date"
    rate: float


def parse_drain_budget(path: str) -> DrainBudget:
    """Read rfc/drain-budget.txt: a start date and a rate, and NOTHING else.

    The key set is closed, so the file cannot grow a per-stem row, a count, a stem list or
    a register column. The moment it named an RFC it would have become the hand-kept
    registry the 2026-07-29 resolution rejected, and a reviewer should treat such a row as
    a defect rather than an extension.

    No rate, date or cadence is defaulted here. A hardcoded one would be this module
    quietly authoring policy that belongs to the owner.
    """
    rel = os.path.relpath(path, PROJECT_DIR)
    try:
        with open(path, encoding="utf-8") as fh:
            raw = fh.read()
    except OSError as exc:
        raise ParseError(
            f"{rel}: cannot read the drain policy: {exc}. An absent budget does NOT mean "
            f"'nothing owed' -- a zero value must never be a valid-looking answer "
            f"(ai/rules/fail-closed-guards.md). Create it with a 'start' date and a "
            f"'rate' in entries per calendar month"
        ) from exc

    seen: Dict[str, str] = {}
    for i, line in enumerate(raw.split("\n"), start=1):
        line = line.split("#", 1)[0].strip()
        if not line:
            continue
        parts = line.split()
        if len(parts) != 2 or parts[0] not in _BUDGET_KEYS:
            raise ParseError(
                f"{rel}:{i}: expected '<key> <value>' with key in {list(_BUDGET_KEYS)}, "
                f"got {line!r}. This file carries POLICY ONLY: it may never name an RFC, "
                f"hold a count, or list stems"
            )
        if parts[0] in seen:
            raise ParseError(f"{rel}:{i}: {parts[0]!r} is set twice")
        seen[parts[0]] = parts[1]

    missing = [k for k in _BUDGET_KEYS if k not in seen]
    if missing:
        raise ParseError(
            f"{rel}: missing {missing}. Both are required: 'start' is when the drain clock "
            f"begins, 'rate' is entries per calendar month (it ships at 0, and only the "
            f"owner arms it)"
        )

    try:
        year, month, day = (int(p) for p in seen["start"].split("-"))
        start = datetime.date(year, month, day)
    except ValueError as exc:
        raise ParseError(
            f"{rel}: start {seen['start']!r} is not a YYYY-MM-DD date: {exc}"
        ) from exc
    try:
        rate = float(seen["rate"])
    except ValueError as exc:
        raise ParseError(
            f"{rel}: rate {seen['rate']!r} is not a number of entries per calendar month"
        ) from exc
    # float() accepts "nan", "inf" and "-inf", and NaN compares FALSE against every
    # operator -- so `rate < 0` below and `rate > len(enrolled)` in check_drain_floor both
    # miss it, and it reaches math.ceil(nan * months) as an uncaught ValueError raised
    # OUTSIDE run_check's try, i.e. a traceback where AC-18 requires a clean exit 2.
    # Refused where every other malformed value is refused, so no caller has to defend
    # against a number that is not one.
    if not math.isfinite(rate):
        raise ParseError(
            f"{rel}: rate {seen['rate']!r} is not a finite number of entries per calendar "
            f"month. A schedule needs a rate arithmetic can compare against a count"
        )
    if rate < 0:
        raise ParseError(f"{rel}: rate {rate} is negative; a backlog cannot un-drain")
    return DrainBudget(start=start, rate=rate)


def required_floor(
    start: "datetime.date",
    rate: float,
    drainable: int,
    today: Optional["datetime.date"] = None,
) -> int:
    """ceil(rate x whole calendar months since start), capped at the DRAINABLE set size.

    `drainable` is the whole enrolled set, NOT the remaining backlog. Capping at the
    remainder double-counts every sign-off -- once by raising the cumulative signed total,
    once by lowering the cap it is compared against -- and the comparison collapses to
    `signed >= enrolled / 2`. Measured: 166 enrolled at rate 100/month over 12 months
    flipped red-to-green at exactly 83 sign-offs, so NO rate Thomas could arm would ever
    demand more than half the corpus.

    The cap still retires the check without a removal commit (AC-28), by a different route:
    a fully drained corpus has `signed == enrolled`, and the floor can never exceed
    `enrolled`, so the comparison is permanently satisfied from then on.

    The clock is the committed start date and the current date, and nothing else. A rate
    or date overridable by flag or environment variable is a forcing function the caller
    can silence, which is not a forcing function (umbrella "How it fails closed").

    "Whole calendar month" is counted to the start day CLAMPED to the current month's
    length. Comparing the raw day numbers drops a month whenever the target month is
    shorter than the start day -- 2026-03-31 to 2026-04-30 counted as zero months elapsed,
    and the SHIPPED policy starts on the 29th, so it would have lost a month every February
    for as long as the drain ran. The floor decides whether the gate is red, so a dropped
    month is a sign-off nobody is asked for.
    """
    now = today or datetime.date.today()
    months = (now.year - start.year) * 12 + (now.month - start.month)
    anniversary = min(start.day, calendar.monthrange(now.year, now.month)[1])
    if now.day < anniversary:
        months -= 1
    months = max(0, months)
    # ceil, never floor or round: at 0.5/month the FIRST month already owes one sign-off,
    # and a schedule owing nothing for its first period is not a schedule.
    return min(drainable, math.ceil(rate * months))


def check_drain_floor(enrolled: Set[str], signed: Dict[str, Extraction]) -> List[str]:
    """Judge the derived sign-off count against the authored drain policy.

    Ships INERT: with the rate at 0 the floor is 0 and this passes on every tree it will
    see before the owner arms it (umbrella D5). The arming commit is his, not this spec's.
    """
    # Returned as a violation rather than raised, so it composes with the other checks the
    # way check_enrolment:660 does. Raising would abort run_check at the first problem and
    # hide every other violation behind "cannot run". Exit 2 either way (AC-29).
    try:
        budget = parse_drain_budget(DRAIN_BUDGET_FILE)
    except ParseError as exc:
        return [str(exc)]
    # Credit and backlog are read off the SAME set. Counting `len(signed)` here while the
    # backlog counted only enrolled stems let an un-enrolled sign-off satisfy the floor
    # without draining anything -- see credited().
    signed = credited(signed, enrolled)
    counts = register_counts(signed)
    total = len(signed)  # independent of the split; see extraction_status
    backlog = len(enrolled - set(signed))
    if budget.rate > len(enrolled):
        return [
            f"{os.path.relpath(DRAIN_BUDGET_FILE, PROJECT_DIR)}: rate {budget.rate} "
            f"exceeds the whole enrolled set ({len(enrolled)}); no schedule can be met"
        ]
    # Capped at the WHOLE enrolled set, never at the remaining backlog. `total` is
    # cumulative, so a remainder cap makes every sign-off count twice -- raising the total
    # and lowering the bar it is measured against -- and the whole comparison degenerates
    # to `signed >= enrolled / 2`. See required_floor.
    floor = required_floor(budget.start, budget.rate, len(enrolled))
    if total >= floor:
        return []
    return [
        f"{os.path.relpath(DRAIN_BUDGET_FILE, PROJECT_DIR)}: the drain schedule requires "
        f"{floor} extraction sign-off(s) by now (rate {budget.rate}/calendar month since "
        f"{budget.start.isoformat()}, capped at the {len(enrolled)} enrolled RFC(s)), and "
        f"there are {total} ({_register_phrase(counts)}; every register counts, umbrella "
        f"D6), leaving {backlog} unsigned. Walk another RFC: make ze-rfc-extract "
        f"STEM=<stem>, then classify every site"
    ]


# WIRING STUBS -- replaced phase by phase. They exist first so run_check calls them
# and the CLI dispatches to them before any of them can do anything.


# --------------------------------------------------------------------------
# Driver
# --------------------------------------------------------------------------
def load_enrolled() -> Set[str]:
    if not os.path.exists(ENROLLED_FILE):
        return set()
    with open(ENROLLED_FILE, encoding="utf-8") as fh:
        return parse_enrolled(fh.read())


def summary_stems() -> Set[str]:
    if not os.path.isdir(SUMMARY_DIR):
        return set()
    return {n[:-3] for n in os.listdir(SUMMARY_DIR) if n.endswith(".md")}


def check_ledger_fresh(
    requirements: Sequence[Requirement], tags: Sequence[Tag], enrolled: Set[str]
) -> List[str]:
    """The generated ledger must match what its sources render to right now.

    `ai/RFC-REQUIREMENTS.md` is derived from the summaries and the `RFC requirement:`
    tags; a test can be re-tagged, moved, or deleted without touching the ledger, and
    then the committed ledger lies about which tests enforce which requirement. This is
    the same staleness `docs_to_code.py --check` guards for `ai/DOCS-TO-CODE.md`
    (`ai/rules/derive-not-hardcode.md`). It runs inside `ze-rfc-check`, which is in both
    verify branches, so a stale ledger fails the build rather than rotting silently.
    """
    body = render_ledger(requirements, tags, enrolled) + "\n"
    current = ""
    if os.path.exists(LEDGER_FILE):
        with open(LEDGER_FILE, encoding="utf-8") as fh:
            current = fh.read()
    if current != body:
        rel = os.path.relpath(LEDGER_FILE, PROJECT_DIR)
        return [f"{rel} is stale vs its sources -- run: make ze-rfc-index"]
    return []


def _collect_for_check() -> Tuple[
    Set[str], List[Requirement], List[str], List[Tag], Dict[str, str]
]:
    """Parse every summary tolerantly and scan the tree once.

    Shared by run_check and run_check_fresh so both render the ledger from identical
    inputs -- if they diverged, one could call fresh what the other rebuilds.

    Returns the reported parse errors (enrolled summaries only) AND the per-stem map of
    every parse failure. check_new_summaries needs the suppressed ones: a summary that is
    new cannot hide behind the migration amnesty the un-enrolled backlog gets.
    """
    enrolled = load_enrolled()
    stems = summary_stems()
    reqs: List[Requirement] = []
    parse_errs: List[str] = []
    by_stem: Dict[str, str] = {}
    for stem in sorted(stems):
        path = os.path.join(SUMMARY_DIR, stem + ".md")
        try:
            reqs.extend(parse_summary_file(path))
        except ParseError as exc:
            by_stem[stem] = str(exc)
            if stem in enrolled:
                parse_errs.append(str(exc))
    return enrolled, reqs, parse_errs, scan_tree(), by_stem


def run_check() -> int:
    try:
        enrolled, reqs, parse_errs, tags, parse_by_stem = _collect_for_check()
        stems = summary_stems()
        # None means git could not answer. The `baseline - current` consumers below take
        # the empty set (an unknown baseline must accuse nobody); the ONE consumer with the
        # opposite polarity takes the None itself.
        baseline_enrolled = _git_baseline_enrolment()
        base_enrolled = baseline_enrolled if baseline_enrolled is not None else set()
        baseline_ids = _git_baseline_ids()  # read once: it costs a git show per summary

        # Read once and shared by three consumers below. None means git could not answer.
        baseline_stems = _git_baseline_summary_stems()

        errs: List[str] = []
        # The sign-off set is derived once and shared: check_enrolment gates a NEW
        # enrolment on it, and check_drain_floor counts it. Only an artifact with zero
        # violations counts, so a stale sign-off cannot keep earning credit.
        signed = signed_extractions(reqs)
        # `current - baseline` needs a baseline that distinguishes "nothing was enrolled at
        # HEAD" from "git could not answer" -- see check_enrolment's docstring for why the
        # opposite polarity makes the empty set unsafe HERE and safe everywhere else.
        #
        # Gated on the availability of the baseline it is COMPUTED FROM, not on a different
        # git call's. It used to read `baseline_stems is None` -- a `git ls-tree` over
        # rfc/short -- while subtracting a `git show` of rfc/enrolled.txt. Drive the state
        # where the first succeeds and the second fails (a shallow clone, a grafted
        # worktree, an rfc/enrolled.txt new in this commit) and every enrolled RFC is
        # accused of being newly enrolled without a sign-off.
        newly_enrolled = (
            None if baseline_enrolled is None else enrolled - baseline_enrolled
        )
        errs.extend(
            check_enrolment(enrolled, base_enrolled, stems, newly_enrolled, set(signed))
        )

        # Adding a summary must add checking, and evidence that existed must keep
        # existing. Both compare against HEAD: the working tree alone cannot tell a
        # backlog item from a regression (spec-rfc-gate-regression-ratchets.md).
        errs.extend(
            check_new_summaries(stems, baseline_stems, enrolled, reqs, parse_by_stem)
        )
        if enrolled & base_enrolled:
            # Both ratchets are no-ops with nothing enrolled on both sides, and the tag
            # baseline costs a git grep plus a batch read of every tagged blob. Do not pay
            # for an answer that cannot change the verdict.
            errs.extend(
                check_retired_requirements(
                    reqs,
                    enrolled,
                    baseline_ids,
                    base_enrolled,
                    stems,
                    baseline_stems,
                    parse_by_stem,
                )
            )
            errs.extend(
                check_coverage_ratchet(
                    reqs,
                    tags,
                    enrolled,
                    _git_baseline_tag_polarities(),
                    base_enrolled,
                )
            )
        # parse_errs holds only the errors for enrolled summaries; the migration to IDs
        # is per-RFC and un-enrolled summaries have not been converted, so their parse
        # failures are expected and not reported.
        errs.extend(parse_errs)

        # IDs are allocated once and never reused. Compare the current id set against the
        # committed baseline so "delete 5.3-4, add a different 5.3-4" is caught even though
        # the two are textually indistinguishable (AC-2). Reuse is a bug regardless of
        # enrolment, so this runs over every parsed requirement.
        errs.extend(check_id_allocation(reqs, baseline_ids))

        errs.extend(evaluate(reqs, tags, enrolled))

        with open(STATUS_FILE, encoding="utf-8") as fh:
            rows = parse_status_ledger(fh.read())
        errs.extend(check_status_agreement(reqs, rows, enrolled))
        errs.extend(check_audit_freshness(reqs, tags, enrolled))

        # Extraction sign-off: bounds what the summary MISSED, which every check above is
        # blind to -- they all judge the requirements a summary LISTS
        # (plan/spec-rfcgate-1-extraction.md). Inside the same try, so a malformed artifact
        # or an unparseable budget exits 2 through the handler below rather than as a
        # traceback (AC-18).
        errs.extend(check_extraction_signoff(reqs))
        errs.extend(check_extraction_ratchet())
        errs.extend(check_drain_floor(enrolled, signed))

        errs.extend(check_ledger_fresh(reqs, tags, enrolled))
    except (ParseError, OSError) as exc:
        # OSError too: an unreadable rfc/enrolled.txt or a missing docs/features/rfc-status.md
        # must fail closed with a clean exit-2 message, not surface as an uncaught traceback
        # (ai/rules/fail-closed-guards.md). scan_tree/load_audit already wrap their OSErrors
        # in ParseError; this covers the two direct read sites (load_enrolled, STATUS_FILE).
        print(f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: {exc}")
        return 2

    if errs:
        print(f"{RED}{BOLD}rfc-requirements: {len(errs)} violation(s){RESET}\n")
        for e in errs:
            print(f"  {RED}*{RESET} {e}")
        print(
            f"\n{YELLOW}Rules:{RESET} every MUST-level requirement of an enrolled RFC needs\n"
            f"a positive AND a negative test tagged `RFC requirement: <ID> <polarity>`,\n"
            f"or an annotation saying why not. See ai/skills/ze-rfc.md."
        )
        return 2

    gated = sum(1 for r in reqs if r.gated and r.rfc in enrolled)
    signed = credited(signed, enrolled)  # the same set the floor and the ledger publish
    counts = register_counts(signed)
    print(
        f"{GREEN}rfc-requirements OK{RESET}: {gated} gated MUST-level requirement(s) "
        f"across {len(enrolled)} enrolled RFC(s); {len(tags)} test tag(s) resolved."
    )
    # AC-14 / R-6: state the extraction bound OUT LOUD on every run, including when it is
    # zero. A check that is quietly satisfied by nothing reproduces the failure it exists
    # to fix, and check_enrolment:661 already sets the precedent of refusing to report
    # clean while enforcing nothing. Never a bare total: the register split is on the same
    # line (umbrella D6).
    print(
        f"{GREEN}extraction{RESET}: {_register_phrase(counts)} signed off of "
        f"{len(enrolled)} enrolled; {len(enrolled - set(signed))} unsigned "
        f"(grandfathered backlog)."
    )
    return 0


def run_write() -> int:
    try:
        enrolled, reqs, _, tags, _ = _collect_for_check()
        body = render_ledger(reqs, tags, enrolled)
    except (ParseError, OSError) as exc:
        # The render now reads rfc/extraction/*.json, so a malformed artifact reaches this
        # path too. Same clean exit-2 the other three drivers give, never a traceback.
        print(f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: {exc}")
        return 2
    os.makedirs(os.path.dirname(LEDGER_FILE), exist_ok=True)
    with open(LEDGER_FILE, "w", encoding="utf-8") as fh:
        fh.write(body + "\n")
    print(f"{GREEN}wrote{RESET} {os.path.relpath(LEDGER_FILE, PROJECT_DIR)}")
    return 0


def run_check_fresh() -> int:
    """Just the ledger-freshness half of run_check -- what `ze-doc-test` runs so a
    docs-focused pass catches a stale `ai/RFC-REQUIREMENTS.md` without paying for the
    full coverage evaluation. The same check also runs inside `run_check` (ze-rfc-check),
    which is where verify catches it."""
    try:
        enrolled, reqs, _, tags, _ = _collect_for_check()
        errs = check_ledger_fresh(reqs, tags, enrolled)
    except (ParseError, OSError) as exc:
        print(f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: {exc}")
        return 2
    if errs:
        for e in errs:
            print(f"{RED}*{RESET} {e}")
        return 1
    print(f"{GREEN}ai/RFC-REQUIREMENTS.md up to date{RESET}")
    return 0


def run_selftest() -> int:
    res = subprocess.run(
        [sys.executable, os.path.join(_HERE, "rfc_requirements_test.py"), "-q"],
        cwd=PROJECT_DIR,
    )
    if res.returncode == 0:
        print("rfc_requirements selftest OK")
    return res.returncode


def main(argv: Sequence[str]) -> int:
    args = list(argv[1:])
    if "--selftest" in args:
        return run_selftest()
    if "--write" in args:
        return run_write()
    if "--check-fresh" in args:
        return run_check_fresh()
    if "--extraction-status" in args:
        # Always the JSON envelope: it is the only consumer this mode has (the umbrella's
        # drain quota), and a second human-readable shape nobody reads would be a mode to
        # keep in step for nothing. `--json` is accepted and inert so the documented
        # spelling works.
        return run_extraction_status()
    if "--extract-skeleton" in args:
        i = args.index("--extract-skeleton")
        stem = args[i + 1] if i + 1 < len(args) else ""
        if not stem or stem.startswith("-"):
            print(
                f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: --extract-skeleton "
                f"needs a stem, e.g. --extract-skeleton rfc7296 "
                f"(make ze-rfc-extract STEM=rfc7296)"
            )
            return 2
        try:
            return run_extract_skeleton(stem)
        except (ParseError, OSError) as exc:
            print(f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: {exc}")
            return 2
    if "--check" in args:
        return run_check()
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
