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

import hashlib
import json
import os
import re
import subprocess
import sys
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
    current: Set[str], baseline: Set[str], summaries: Set[str]
) -> List[str]:
    """Enrolment grows only, and never names an RFC we have no summary for."""
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
    return errs


def parse_enrolled(text: str) -> Set[str]:
    out: Set[str] = set()
    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        out.add(line.split()[0])
    return out


def _git_baseline_enrolment() -> Set[str]:
    try:
        res = subprocess.run(
            ["git", "show", "HEAD:rfc/enrolled.txt"],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return set()
    if res.returncode != 0:
        return set()
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
    lines = [l.strip() for l in src.split("\n")]
    return "\n".join(l for l in lines if l)


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
    """
    for sub in ("full", "drafts"):
        path = os.path.join(PROJECT_DIR, "rfc", sub, stem + ".txt")
        if os.path.exists(path):
            try:
                with open(path, encoding="utf-8", errors="replace") as fh:
                    return len(_SRC_KEYWORD_RE.findall(fh.read()))
            except OSError:
                return None
    return None


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
        baseline_enrolled = _git_baseline_enrolment()
        baseline_ids = _git_baseline_ids()  # read once: it costs a git show per summary

        errs: List[str] = []
        errs.extend(check_enrolment(enrolled, baseline_enrolled, stems))

        # Adding a summary must add checking, and evidence that existed must keep
        # existing. Both compare against HEAD: the working tree alone cannot tell a
        # backlog item from a regression (spec-rfc-gate-regression-ratchets.md).
        baseline_stems = _git_baseline_summary_stems()
        errs.extend(
            check_new_summaries(stems, baseline_stems, enrolled, reqs, parse_by_stem)
        )
        if enrolled & baseline_enrolled:
            # Both ratchets are no-ops with nothing enrolled on both sides, and the tag
            # baseline costs a git grep plus a batch read of every tagged blob. Do not pay
            # for an answer that cannot change the verdict.
            errs.extend(
                check_retired_requirements(
                    reqs,
                    enrolled,
                    baseline_ids,
                    baseline_enrolled,
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
                    baseline_enrolled,
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
    print(
        f"{GREEN}rfc-requirements OK{RESET}: {gated} gated MUST-level requirement(s) "
        f"across {len(enrolled)} enrolled RFC(s); {len(tags)} test tag(s) resolved."
    )
    return 0


def run_write() -> int:
    enrolled, reqs, _, tags, _ = _collect_for_check()
    body = render_ledger(reqs, tags, enrolled)
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
    if "--check" in args:
        return run_check()
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
