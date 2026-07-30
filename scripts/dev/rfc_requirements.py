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
    python3 scripts/dev/rfc_requirements.py --reseal       # re-stamp SHIFTED audit verdicts
    python3 scripts/dev/rfc_requirements.py --selftest     # run rfc_requirements_test.py

--reseal is the ONLY mode that writes under rfc/audit/. --check is read-only and --write
touches the ledger alone, so every change to a hand-authored evidence file is either a human
editing it or one greppable command (spec-rfcgate-3-audit-teeth.md, A-7).

Exit 0 = a comparison ran and found nothing wrong.
Exit 2 = violations found, or the gate could not run (unparseable input, nothing
         enrolled, enrolled RFC with no summary). "Clean" must mean "I compared things
         and found nothing", never "I compared nothing" (ai/rules/fail-closed-guards.md).
"""

import calendar
import datetime
import hashlib
import importlib.util
import io
import json
import math
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
import tokenize
from typing import Dict, List, NamedTuple, Optional, Sequence, Set, Tuple

_HERE = os.path.dirname(os.path.abspath(__file__))

# "The tagged unit" has exactly ONE definition, shared with the edit-time guard
# (.claude/hooks/pretool-writeedit.py). Loaded BY PATH rather than by `import`, because this
# module is itself loaded by path in three places -- rfc_requirements_test.py, the Go gate
# wrappers, and rename_module_path.py's library import -- and in none of them is scripts/dev
# guaranteed to be on sys.path.
_scope_spec = importlib.util.spec_from_file_location(
    "rfc_tagged_scope", os.path.join(_HERE, "rfc_tagged_scope.py")
)
rfc_tagged_scope = importlib.util.module_from_spec(_scope_spec)
_scope_spec.loader.exec_module(rfc_tagged_scope)
PROJECT_DIR = os.path.abspath(os.path.join(_HERE, "..", ".."))

SUMMARY_DIR = os.path.join(PROJECT_DIR, "rfc", "short")
ENROLLED_FILE = os.path.join(PROJECT_DIR, "rfc", "enrolled.txt")
# The declared remainder: every summary that is NOT enrolled says here why not. Without it
# "not enrolled" is an absence, and an absence cannot be told apart from a decision
# (plan/spec-rfcgate-4-ledger.md D3).
NOT_ENROLLED_FILE = os.path.join(PROJECT_DIR, "rfc", "not-enrolled.txt")
STATUS_FILE = os.path.join(PROJECT_DIR, "docs", "features", "rfc-status.md")
LEDGER_FILE = os.path.join(PROJECT_DIR, "ai", "RFC-REQUIREMENTS.md")
AUDIT_DIR = os.path.join(PROJECT_DIR, "rfc", "audit")

# Repo-relative forms, for the batch HEAD reader (_git_cat_blobs takes paths, not abspaths).
ENROLLED_REL = os.path.join("rfc", "enrolled.txt")
NOT_ENROLLED_REL = os.path.join("rfc", "not-enrolled.txt")
STATUS_REL = os.path.join("docs", "features", "rfc-status.md")

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

# Why a summary is not enrolled. A closed set, exactly as ANNOTATION_KINDS is: a free-text
# reason alone lets "not enrolled" read as "settled", and only `non-normative` is a claim
# about conformance. `backlog` and `blocked` are DEBT and render as debt.
DISPOSITION_KINDS = frozenset({"non-normative", "backlog", "blocked"})
# The one kind that says the DOCUMENT imposes nothing, and therefore the only one that may
# stand under a public support claim (check_unproven_support).
DISPOSITION_NON_NORMATIVE = "non-normative"

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
_PY_TAG_RE = re.compile(r"^#\s*RFC requirement:\s*(?P<rest>.*)$")
_TERMINATOR_RE = re.compile(r"terminator=(?P<name>[A-Za-z0-9_]+)")

# Every tag, in every carrier, contains this literal. It is the cheap pre-filter that
# tells "this file certainly holds no tag" from "this file might", so the expensive answer
# is only computed where it can change the verdict: scan_tree tests it before handing a
# file to a reader, and _read_git_baseline_tags hands it to `git grep -F`. Both call sites
# read the constant; neither re-spells the string (ai/rules/derive-not-hardcode.md).
TAG_MARKER = "RFC requirement:"

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


def scan_python_tags(src: str, path: str) -> List[Tag]:
    """Find `# RFC requirement: <ID> <polarity>` in an interop scenario's check.py.

    Tokenized, never regex-scanned. A scenario check is full of quoted protocol text --
    prefixes, vtysh output, JSON bodies -- and a `#` inside a string or a docstring is not
    a comment. Only the Python tokenizer can tell the two apart, which is the same
    principle scan_ci_tags applies with `terminator=`: a comment is only a comment where
    the real parser says so.

    `terminator=` semantics are deliberately NOT inherited. That construct models the .ci
    runner's tmpfs blocks (internal/test/runner/parsing.go:264-268), whose bodies are raw
    file content. A check.py has no such construct -- interop.py:1468-1488 hands the whole
    file to importlib -- so every line is Python and skipping "block bodies" would drop
    real comments.

    Requires the comment to be the first thing on its line, mirroring _GO_TAG_RE's `^\\s*//`
    and _CI_TAG_RE's post-strip `^#`. A trailing comment is not a tag in any carrier.

    Fails closed on a file the tokenizer cannot read (AC-9). That is exactly the condition
    under which comment extraction is untrustworthy, so reporting "no tags" would be a
    zero that looks like an answer (ai/rules/fail-closed-guards.md).
    """
    out: List[Tag] = []
    try:
        toks = list(tokenize.generate_tokens(io.StringIO(src).readline))
    except (tokenize.TokenError, IndentationError, SyntaxError) as exc:
        raise ParseError(
            f"{path}: cannot tokenize as Python ({exc}); a file whose comments cannot be "
            f"read cannot be reported as carrying no RFC requirement tags"
        ) from exc
    for tok in toks:
        if tok.type != tokenize.COMMENT:
            continue
        if tok.line[: tok.start[1]].strip():
            continue  # trailing comment, not a line-start tag
        m = _PY_TAG_RE.match(tok.string.strip())
        if not m:
            continue
        line = tok.start[0]
        t = _parse_tag_rest(m.group("rest"), f"{path}:{line}")
        out.append(t._replace(file=path, line=line))
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


# --------------------------------------------------------------------------
# Carrier table (plan/spec-rfcgate-2-evidence.md)
# --------------------------------------------------------------------------
# Evidence has TWO independent axes, and conflating them is how "we have interop
# coverage" becomes true and worthless at once:
#
#   kind -- which layer the test exercises (a unit table test proves the algorithm; a
#           .ci proves the daemon exposes it; an interop scenario proves a foreign peer
#           accepts it).
#   tier -- whether anything EXECUTES it. A tag in a suite no pipeline runs is not weaker
#           evidence, it is the absence of evidence wearing evidence's clothes.
#
# This table is the ONE place either axis is spelled. scan_tree, the HEAD baseline filter,
# the tolerant baseline scanner, the ledger render and the evidence ratchet all read it
# (ai/rules/derive-not-hardcode.md). The two extension filters used to be independent
# literal `endswith` chains in scan_tree and _git_baseline_tag_polarities; extending one
# and not the other desynchronizes the ratchet baseline and manufactures phantom polarity
# losses, silently and in the green direction.
TIER_VERIFY = "verify"  # runs in a ze-verify stage, i.e. on every push
TIER_NIGHTLY = "nightly"  # runs in a scheduled advisory workflow
TIER_UNRUN = "unrun"  # nothing runs it automatically: a tag here is refused

# Functional tests under development live here. Gitignored, and SKIPPED (not refused) by
# every repo-wide scanner: a draft must be able neither to claim evidence nor to redden
# someone else's run. Same exclusion `real_ci_files` applies in the ci-sleep ratchet
# (scripts/dev/verify_wiring_docs.py:215); the registry of scanners that owe it is
# internal/test/runner/draft_dir_test.go's TestDraftDirIsInvisibleToRepoGates.
DRAFT_PREFIX = "test/draft/"

# `ze-functional-test` names the suites it runs in ONE place, and a suite name is also the
# test/<suite>/ directory the runner walks (internal/test/runner/draft_dir.go:40). Reading
# that line is what keeps a `.ci`'s tier tied to whether anything executes it, instead of
# to its extension (ai/rules/derive-not-hardcode.md).
FUNCTIONAL_MK = os.path.join(PROJECT_DIR, "mk", "test-functional.mk")
_ALL_SUITES_RE = re.compile(r'all_suites="(?P<names>[^"]*)"')
EDITOR_SUITE = "editor"


def functional_suites(path: str = FUNCTIONAL_MK) -> Tuple[str, ...]:
    """The suites `make ze-functional-test` runs, read from its own recipe.

    Fails closed. An unreadable or unrecognizable recipe means we do not know what runs,
    and a gate that answers "everything runs" in that state is the exact zero-that-looks-
    like-an-answer this module refuses elsewhere (ai/rules/fail-closed-guards.md).
    """
    try:
        with open(path, encoding="utf-8") as fh:
            src = fh.read()
    except OSError as exc:
        raise ParseError(
            f"{path}: cannot read the functional-test recipe, so the set of suites that "
            f"run inside ze-verify is unknown: {exc}"
        ) from exc
    found = _ALL_SUITES_RE.findall(src)
    if not found:
        raise ParseError(
            f'{path}: no `all_suites="..."` assignment found. That line is where '
            f"ze-functional-test declares which suites it runs, and the .ci/.et evidence "
            f"tier is derived from it; without it no tier can be justified"
        )
    if len(found) > 1:
        # Taking the first match is a fail-OPEN, and the quiet kind. A second recipe in
        # this file (a `ze-functional-list`, a `-quick` subset) would decide the tier of
        # every `.ci` in the repo without touching ze-functional-test, upgrading suites
        # nothing runs to merge-gate evidence. Two answers is not an answer: refuse and
        # make a human say which line is the definition (ai/rules/fail-closed-guards.md).
        raise ParseError(
            f'{path}: {len(found)} `all_suites="..."` assignments found, so which one '
            f"ze-functional-test runs is ambiguous. The .ci/.et evidence tier is derived "
            f"from that line, and picking one of several would grant merge-gate tier on a "
            f"guess. Declare the suite list exactly once"
        )
    names = tuple(n for n in found[0].split() if n)
    if not names:
        raise ParseError(
            f'{path}: `all_suites=""` is empty; no suite runs in ze-verify'
        )
    return names


def _suite_carriers(
    kind: str,
    suffix: str,
    reader: str,
    runner: str,
    stage: str,
    suites: Sequence[str],
) -> Tuple["Carrier", ...]:
    """One verify-tier row per suite, so the prefix carries the execution claim.

    A single `prefix=""` row is what let a `.ci` claim merge-gate tier by extension alone;
    per-suite prefixes make the claim checkable against the recipe that produces them.
    """
    return tuple(
        Carrier(
            f"{kind}-{s}",
            kind,
            TIER_VERIFY,
            f"test/{s}/",
            suffix,
            reader,
            runner,
            f"{stage}, {s} suite)",
            True,
        )
        for s in suites
    )


class Carrier(NamedTuple):
    """One recognized evidence carrier: its shape, its reader, and what executes it."""

    name: str  # stable identity, used in errors
    kind: str  # evidence kind published in the ledger
    tier: str  # execution tier, one of TIER_*
    prefix: str  # repo-relative path prefix ("" matches anywhere under TEST_ROOTS)
    suffix: str  # path suffix selecting this carrier
    reader: str  # which scanner parses this shape: "go" | "ci" | "python"
    runner: str  # the make target that executes it
    pipeline: str  # where that target runs, named in errors and in the ledger legend
    # True for a row _suite_carriers generated from a recipe's suite list, rather than one
    # declared literally. The HEAD table swaps exactly these out (_build_head_carriers).
    derived: bool = False

    @property
    def label(self) -> str:
        """`kind/tier`, the cell the ledger prints beside every test link."""
        return f"{self.kind}/{self.tier}"


# Ordered: the first entry whose prefix AND suffix match wins, so the specific scenario
# trees are declared before the unclassified catch-all.
CARRIERS: Tuple[Carrier, ...] = (
    Carrier(
        "unit",
        "unit",
        TIER_VERIFY,
        "",
        "_test.go",
        "go",
        "make ze-unit-test",
        "ze-verify (unit stage)",
    ),
    # ONE verify-tier row per suite ze-functional-test actually names, derived from its
    # recipe. A single `prefix=""` row credited ANY .ci anywhere under internal/, pkg/ or
    # test/ as merge-gate evidence, which made three silent evasions possible: move a
    # tagged .ci out of a run suite (test/traffic/), into the gitignored incubator
    # (test/draft/), or into a tree whose sibling check.py the SAME table refuses as unrun
    # (test/ipsec-interop/). ~59 .ci live in suites the recipe does not list.
    *_suite_carriers(
        "functional",
        ".ci",
        "ci",
        "make ze-functional-test",
        "ze-verify (functional stage",
        functional_suites(),
    ),
    # .et is the cheapest verify-tier NON-unit carrier available, and it costs one row:
    # 163 of 164 .et files use terminator= blocks, so it is .ci semantics exactly, and
    # mk/test-functional.mk lists `editor` in ze-functional-test, which is in both
    # stagesForMode branches. Supported and currently unbound: no editor-visible RFC
    # obligation exists to bind, and manufacturing one to exercise the plumbing would be
    # a test written for the gate (settled 2026-07-29, spec Key Design Decisions). Only
    # test/editor/ is walked for .et, so only that suite earns the row.
    *_suite_carriers(
        "editor",
        ".et",
        "ci",
        "make ze-editor-test",
        "ze-verify (functional stage",
        [s for s in functional_suites() if s == EDITOR_SUITE],
    ),
    # test/exabgp-compat is NOT one of ze-functional-test's suites: it has its own stage,
    # `ze-exabgp-test`, which stagesForMode lists in BOTH verify modes. A separate target
    # is a separate fact, so it is a declared row rather than a second suite list.
    Carrier(
        "functional-exabgp",
        "functional",
        TIER_VERIFY,
        "test/exabgp-compat/",
        ".ci",
        "ci",
        "make ze-exabgp-test",
        "ze-verify (exabgp stage)",
    ),
    # Catch-alls for the two extensions. Reached by a .ci/.et under a suite no ze-verify
    # stage runs, under internal/ or pkg/ (TEST_ROOTS includes both and no suite walks
    # either), or under an interop tree beside a check.py this table already refuses.
    Carrier(
        "functional-unrun",
        "functional",
        TIER_UNRUN,
        "",
        ".ci",
        "ci",
        "no ze-verify stage walks this directory",
        "no automated caller; ze-functional-test runs "
        + ", ".join(functional_suites()),
    ),
    Carrier(
        "editor-unrun",
        "editor",
        TIER_UNRUN,
        "",
        ".et",
        "ci",
        "no ze-verify stage walks this directory",
        "no automated caller; only test/" + EDITOR_SUITE + "/ is walked for .et",
    ),
    Carrier(
        "interop-bgp",
        "interop",
        TIER_NIGHTLY,
        "test/interop/scenarios/",
        "/check.py",
        "python",
        "make ze-interop-test",
        ".github/workflows/evidence-nightly.yml (advisory)",
    ),
    # The other three interop trees have runners but NO automated caller, so a tag in
    # them would be evidence nothing executes. Declared rather than omitted: an omitted
    # tree falls to the catch-all with a vaguer message, and naming the runner is what
    # makes the error actionable. Wiring them into CI is tracked in the deferral shard.
    Carrier(
        "interop-ipsec",
        "interop",
        TIER_UNRUN,
        "test/ipsec-interop/",
        "/check.py",
        "python",
        "make ze-ipsec-interop-test",
        "no automated caller",
    ),
    Carrier(
        "interop-l2tp",
        "interop",
        TIER_UNRUN,
        "test/l2tp-interop/",
        "/check.py",
        "python",
        "the L2TP interop runner",
        "no automated caller",
    ),
    Carrier(
        "interop-pppoe",
        "interop",
        TIER_UNRUN,
        "test/pppoe-interop/",
        "/check.py",
        "python",
        "make ze-deployment-pppoe-accel-docker-test",
        "no automated caller",
    ),
    # Catch-all. test/stress/scenarios/ and test/l2tp-scale/ also hold check.py files, and
    # any future tree will too. Refusing a tag there by DEFAULT is the fail-closed shape:
    # a carrier whose pipeline nobody has declared is exactly the case where silence would
    # be indistinguishable from proof (ai/rules/fail-closed-guards.md).
    Carrier(
        "scenario-check",
        "unknown",
        TIER_UNRUN,
        "",
        "/check.py",
        "python",
        "no declared runner",
        "no automated caller",
    ),
)

_READERS = {
    "go": scan_go_tags,
    "ci": scan_ci_tags,
    "python": scan_python_tags,
}


def carrier_for(rel: str) -> Optional[Carrier]:
    """The carrier a repo-relative path belongs to, or None when the shape carries no tags.

    The single predicate behind BOTH extension filters. `rel` is always slash-separated:
    every caller normalizes before asking, so this never has to know about os.sep.

    test/draft/ yields None -- SKIPPED, never refused. The incubator is gitignored and
    invisible to every repo-wide gate (ai/rules/testing.md; test/draft/README.md), so a
    draft must be able neither to claim evidence nor to fail this gate for a session that
    has nothing to do with it. Refusing it would do the latter.
    """
    if rel.startswith(DRAFT_PREFIX):
        return None
    return _lookup(rel, CARRIERS)


def _lookup(rel: str, carriers: Sequence["Carrier"]) -> Optional["Carrier"]:
    """First carrier whose prefix AND suffix match. Shared by the tree and HEAD tables."""
    for c in carriers:
        if rel.startswith(c.prefix) and rel.endswith(c.suffix):
            return c
    return None


def _refuse_unrun(carrier: Carrier, tag: Tag) -> ParseError:
    """The message an `unrun` carrier's tag gets. A refusal, not a marker: a ledger note
    would decorate evidence that never executes, and a decorated absence still reads as
    presence."""
    return ParseError(
        f"{tag.file}:{tag.line}: RFC requirement tag for {tag.rid} sits in carrier "
        f"'{carrier.name}', which nothing executes automatically "
        f"(runner: {carrier.runner}; pipeline: {carrier.pipeline}). "
        f"A tag is only evidence if something runs the test, so this "
        f"one is refused rather than counted. Fix it by adding that suite to an automated "
        f"pipeline (the BGP interop tree's own advisory job in "
        f".github/workflows/evidence-nightly.yml is the pattern) and giving its carrier a "
        f"real tier in CARRIERS -- or bind the requirement to a .ci instead, which runs "
        f"inside ze-verify on every push"
    )


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
                rel = os.path.relpath(path, root).replace(os.sep, "/")
                carrier = carrier_for(rel)
                if carrier is None:
                    continue
                try:
                    with open(path, encoding="utf-8", errors="replace") as fh:
                        src = fh.read()
                except OSError as exc:
                    raise ParseError(f"{rel}: cannot read: {exc}") from exc
                # THE pre-filter TAG_MARKER exists for. Every tag in every carrier contains
                # this literal, so a file without it certainly holds none and the expensive
                # answer -- a per-line regex, or a full Python tokenize for a check.py --
                # never has to be computed. 4383 files match a carrier here; 373 hold the
                # marker. Skipping is safe precisely BECAUSE the readers only ever report a
                # tag whose line contains it, so no reachable verdict can change.
                if TAG_MARKER not in src:
                    continue
                found = _READERS[carrier.reader](src, rel)
                if found and carrier.tier == TIER_UNRUN:
                    raise _refuse_unrun(carrier, found[0])
                tags.extend(found)
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


class Disposition(NamedTuple):
    """One `rfc/not-enrolled.txt` row: why this summary is not enrolled."""

    kind: str
    reason: str


def parse_dispositions(text: str) -> Dict[str, Disposition]:
    """Read `rfc/not-enrolled.txt` into {stem: Disposition}.

    Same comment and blank-line tolerance as `parse_enrolled`, and the same first-token
    stem, so one reader serves both files. Everything after that is REJECTED rather than
    skipped: a malformed line here would silently un-declare a summary, and an
    un-declared summary is exactly the absence this file exists to abolish
    (`ai/rules/fail-closed-guards.md`). A typo must cost a red gate, not a quiet hole.
    """
    out: Dict[str, Disposition] = {}
    for n, raw in enumerate(text.split("\n"), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        fields = line.split(None, 2)
        stem = fields[0]
        if stem in out:
            raise ParseError(
                f"rfc/not-enrolled.txt:{n}: duplicate stem {stem!r}. One row per summary: "
                f"two rows can carry two different kinds and nothing decides between them"
            )
        if len(fields) < 2:
            raise ParseError(
                f"rfc/not-enrolled.txt:{n}: {line!r} has no kind. Each row is "
                f"'<stem> <kind> <reason>' with kind one of "
                f"{sorted(DISPOSITION_KINDS)}"
            )
        kind = fields[1]
        if kind not in DISPOSITION_KINDS:
            raise ParseError(
                f"rfc/not-enrolled.txt:{n}: kind {kind!r} for {stem} is not one of "
                f"{sorted(DISPOSITION_KINDS)}. Use 'non-normative' when the DOCUMENT "
                f"imposes nothing, 'backlog' when the extraction is owed, 'blocked' when "
                f"something outside the summary prevents enrolment"
            )
        reason = fields[2].strip() if len(fields) > 2 else ""
        if not reason:
            raise ParseError(
                f"rfc/not-enrolled.txt:{n}: {stem} is declared {kind} with no reason. A "
                f"bare kind is an absence with a label on it: say what makes it true"
            )
        out[stem] = Disposition(kind=kind, reason=reason)
    return out


def load_dispositions() -> Dict[str, Disposition]:
    """The declared remainder, or empty when the file does not exist yet.

    An absent file is NOT an error here: `check_summary_disposition` reports every summary
    that is neither enrolled nor declared, so an absent file surfaces as one violation per
    un-enrolled summary -- which names the actual problem -- rather than as one message
    about a missing path.
    """
    if not os.path.exists(NOT_ENROLLED_FILE):
        return {}
    with open(NOT_ENROLLED_FILE, encoding="utf-8") as fh:
        return parse_dispositions(fh.read())


def _git_baseline_dispositions() -> Set[str]:
    """The stems declared at HEAD, or the empty set when git could not answer.

    The empty set, and unlike its two `Optional` siblings that is the SAFE answer here:
    this baseline has one consumer, the discharge ratchet (AC-8), which reads it as
    `baseline - current`. An empty baseline therefore accuses nobody. The polarity is what
    decides the return type, not a preference -- `_git_baseline_enrolment` documents the
    same rule from the other side.

    A HEAD blob that does not parse also yields the empty set. The working tree's copy is
    parsed strictly by `load_dispositions`, so a malformed CURRENT file already reds the
    gate; a malformed HEAD blob is history nobody can use, and guessing at its content
    would accuse a stem of a discharge that may never have happened.
    """
    blobs = _git_cat_blobs([NOT_ENROLLED_REL])
    if NOT_ENROLLED_REL not in blobs:
        return set()
    try:
        return set(parse_dispositions(blobs[NOT_ENROLLED_REL]))
    except ParseError:
        return set()


def _git_baseline_status_rows() -> Optional[Dict[str, Dict[str, str]]]:
    """The status rows committed at HEAD, or None when git could not answer.

    Read through `_git_cat_blobs` rather than a per-file `git show`: that reader's docstring
    makes the batch interface a condition of the check being kept, and one more fork per
    `make ze-verify` is exactly the cost it was written to avoid.

    None, not {}: the completeness ratchet asks "did this stem HAVE a row at HEAD", and an
    empty answer would say "no" for all 157 of them -- which turns a deleted-row check into
    a wall of false accusations and teaches people to bypass the gate
    (`_git_baseline_summary_stems` records the same trap).
    """
    blobs = _git_cat_blobs([STATUS_REL])
    if STATUS_REL not in blobs:
        return None
    return parse_status_ledger(blobs[STATUS_REL])


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


_BASELINE_TAGS_CACHE: Optional[List[Tag]] = None


def reset_baseline_cache() -> None:
    """Drop the memoized HEAD baseline. For tests that swap the git layer underneath it."""
    global _BASELINE_TAGS_CACHE, _HEAD_CARRIERS_CACHE
    _BASELINE_TAGS_CACHE = None
    _HEAD_CARRIERS_CACHE = None


_HEAD_CARRIERS_CACHE: Optional[Tuple[Carrier, ...]] = None


def _head_carriers() -> Tuple[Carrier, ...]:
    """The carrier table as of git HEAD, for labelling the HEAD baseline.

    The evidence ratchet compares what each requirement proved at HEAD with what it proves
    now. Labelling BOTH sides with today's table makes a tier DOWNGRADE symmetric and
    therefore invisible: drop a suite from ze-functional-test and every tag in it is
    relabelled on both sides at once, so nothing reds even though evidence that used to
    run on the merge path no longer does. Reading HEAD's own recipe is what makes that a
    loss instead of a wash.

    Falls back to today's table when HEAD cannot be read (no git, shallow checkout, a fresh
    repo with no commit). That is the same degradation `_git_baseline_tags` already takes,
    and it is stated rather than silent: with no baseline there is nothing to compare, so
    the ratchet reports nothing either way.
    """
    global _HEAD_CARRIERS_CACHE
    if _HEAD_CARRIERS_CACHE is not None:
        return _HEAD_CARRIERS_CACHE
    _HEAD_CARRIERS_CACHE = _build_head_carriers()
    return _HEAD_CARRIERS_CACHE


def _build_head_carriers() -> Tuple[Carrier, ...]:
    """The uncached read. HEAD's suite list swapped into today's table shape."""
    try:
        proc = subprocess.run(
            ["git", "show", f"HEAD:{os.path.relpath(FUNCTIONAL_MK, PROJECT_DIR)}"],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return CARRIERS
    if proc.returncode != 0:
        return CARRIERS
    m = _ALL_SUITES_RE.search(proc.stdout)
    if not m:
        return CARRIERS
    head_suites = tuple(n for n in m.group("names").split() if n)
    if not head_suites:
        return CARRIERS
    if head_suites == functional_suites():
        return CARRIERS
    # Only the derived rows differ; every declared row is identical by construction. The
    # shapes (suffix, reader, runner, stage text) are READ OFF today's derived rows rather
    # than re-spelled, so CARRIERS stays the one place an extension is written down.
    out: List[Carrier] = [c for c in CARRIERS if not c.derived]
    shapes: Dict[str, Carrier] = {}
    for c in CARRIERS:
        if c.derived:
            shapes.setdefault(c.kind, c)
    rebuilt: List[Carrier] = []
    for kind, shape in shapes.items():
        # Only test/editor/ is walked for .et, so the editor shape covers exactly the
        # suite of that name; every other shape covers each suite directory.
        covered = [s for s in head_suites if kind != EDITOR_SUITE or s == EDITOR_SUITE]
        stage = shape.pipeline.rsplit(",", 1)[0]
        rebuilt.extend(
            shape._replace(
                name=f"{kind}-{s}",
                prefix=f"test/{s}/",
                pipeline=f"{stage}, {s} suite)",
            )
            for s in covered
        )
    # Derived rows first: they are the specific prefixes, and the catch-alls must stay last.
    return tuple(rebuilt + out)


def _head_carrier_for(rel: str) -> Optional[Carrier]:
    """carrier_for, evaluated against HEAD's table. Same draft exclusion."""
    if rel.startswith(DRAFT_PREFIX):
        return None
    return _lookup(rel, _head_carriers())


def _git_baseline_tags() -> List[Tag]:
    """Every RFC requirement tag present at git HEAD, as Tags.

    Memoized for the process. Two ratchets now read this baseline -- polarity and evidence
    kind -- and each uncached call is a `git grep` plus a batch read of every tagged blob,
    measured at ~0.6s on this tree. Paying that twice took `ze-rfc-check` from 2.6s to
    3.2s, and this module already treats gate runtime as a condition of the gate being
    kept rather than skipped (see _git_cat_blobs). HEAD does not change inside one run, so
    the second answer cannot legitimately differ from the first.

    The baseline is re-parsed with the SAME scanners the working tree goes through, never
    with a regex over `git grep` output. A .ci `terminator=` block is raw file content that
    can contain a line looking exactly like a tag (scan_ci_tags:510), so a regex baseline
    would invent tags that were never there and then report their "loss".

    Only files git already told us contain the marker are read, so the cost tracks the
    number of tagged files, not the size of the repository.
    """
    global _BASELINE_TAGS_CACHE
    if _BASELINE_TAGS_CACHE is not None:
        return _BASELINE_TAGS_CACHE
    _BASELINE_TAGS_CACHE = _read_git_baseline_tags()
    return _BASELINE_TAGS_CACHE


def _read_git_baseline_tags() -> List[Tag]:
    """The uncached read. Split out so the memo above is one readable branch."""
    try:
        listing = subprocess.run(
            ["git", "grep", "-l", "-z", "-F", TAG_MARKER, "HEAD", "--"]
            + list(TEST_ROOTS),
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return []
    # git grep exits 1 when nothing matched, which is a real answer ("no tags at HEAD");
    # any other non-zero is a failure and yields no baseline.
    if listing.returncode not in (0, 1):
        return []
    paths = []
    for entry in listing.stdout.split("\0"):
        # NOT stripped: `-z` emits the path verbatim, and stripping would silently rename
        # a path with leading or trailing spaces into one that does not exist.
        # Honest note: no test pins this, because it is not observable here. A path with a
        # leading or trailing space matches no carrier with or without the strip, so both
        # spellings return the same empty baseline. It is kept as correctness by
        # construction, not as a behaviour someone verified.
        if not entry.startswith("HEAD:"):
            continue
        rel = entry[len("HEAD:") :]
        # THE SECOND EXTENSION FILTER. It reads the same carrier_for the tree scan reads,
        # which is the whole point: these were two independent literal `endswith` chains,
        # and widening one alone leaves the baseline blind to a carrier the tree now sees.
        # The ratchet then compares two differently-shaped sets and reports losses that
        # never happened -- or, worse, misses ones that did.
        carrier = _head_carrier_for(rel)
        if carrier is None or carrier.tier == TIER_UNRUN:
            # An unrun carrier cannot contribute a baseline: the tree scan REFUSES its
            # tags outright, so crediting one at HEAD would make the ratchet demand
            # evidence the tree is forbidden to supply.
            #
            # Judged against HEAD's table, not today's: a suite dropped from
            # ze-functional-test in THIS change was runnable at HEAD, so its tags were
            # real evidence then. Filtering them out with today's table would erase the
            # very baseline the downgrade has to be measured against.
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
        return []

    out: List[Tag] = []
    for rel, blob in _git_cat_blobs(paths).items():
        out.extend(_scan_tags_tolerant(blob, rel))
    return out


def _git_baseline_tag_polarities() -> Dict[str, Set[str]]:
    """Which polarities proved each requirement at HEAD: {rid: {"positive", "negative"}}.

    A derivation over _git_baseline_tags, which does the git work. Kept as its own name
    because check_coverage_ratchet's contract is polarity sets and nothing else, and
    because the two ratchets must not accidentally share a shape: proof-of-polarity and
    proof-of-evidence-strength answer different questions about the same tags.
    """
    out: Dict[str, Set[str]] = {}
    for t in _git_baseline_tags():
        out.setdefault(t.rid, set()).add(t.polarity)
    return out


def nonunit_evidence(tags: Sequence[Tag], lookup=None) -> Dict[str, Set[str]]:
    """{rid: {"functional/verify", "interop/nightly", ...}} -- unit evidence excluded.

    Keyed by the carrier LABEL, which carries the tier, so verify-tier and nightly-tier
    evidence are different members of the set rather than the same one. That is what makes
    swapping a `.ci` binding for an interop binding a LOSS instead of a wash (AC-14): the
    two are not interchangeable, because only one of them runs before a merge.

    `lookup` selects WHICH carrier table answers. The tree side uses today's; the HEAD
    baseline uses HEAD's (_head_carrier_for), because labelling both sides with today's
    table makes a tier downgrade symmetric and therefore invisible.
    """
    if lookup is None:
        lookup = carrier_for
    out: Dict[str, Set[str]] = {}
    for t in tags:
        c = lookup(t.file)
        if c is None or c.kind == "unit":
            continue
        out.setdefault(t.rid, set()).add(c.label)
    return out


def _git_baseline_evidence() -> Dict[str, Set[str]]:
    """The non-unit evidence each requirement carried at HEAD, labelled with HEAD's tiers."""
    return nonunit_evidence(_git_baseline_tags(), lookup=_head_carrier_for)


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

    The non-Go guard is load-bearing, not defensive decoration: the Go fallback matches
    `// RFC requirement:` (_GO_TAG_RE, :148), and a .ci records shell and config content
    where such a line can appear without being a tag at all. Running the Go fallback over a
    .ci would invent exactly the phantom tags this function exists to avoid, and the
    ratchet would then report the "loss" of a tag that never existed. A check.py is the
    same case one step further: a `#` inside a docstring is not a comment, so a per-line
    rescan of an untokenizable file would invent tags from quoted protocol text (R-6).

    Reader chosen from CARRIERS, never from a literal suffix here -- this was the module's
    THIRD spelling of the extension list (A-3).
    """
    carrier = carrier_for(rel)
    if carrier is None:
        return []
    try:
        return _READERS[carrier.reader](blob, rel)
    except ParseError:
        pass
    if carrier.reader != "go":
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


def check_evidence_ratchet(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    baseline_evidence: Dict[str, Set[str]],
    baseline_enrolled: Set[str],
) -> List[str]:
    """Non-unit evidence is monotonic, and each TIER ratchets independently.

    check_coverage_ratchet already makes proof monotonic, but it compares polarity sets and
    is blind to what KIND of test supplied them: replacing a `.ci` binding with a unit tag
    of the same polarity is invisible to it. That is the exact regression this gate exists
    to stop, because a unit test proves the algorithm and only a functional or interop test
    proves the daemon or a peer (`ai/rules/integration-completeness.md`).

    Keyed by carrier LABEL (`kind/tier`), so the two counters R-1 asks for fall out of one
    comparison rather than being maintained as two: converting a verify-tier `.ci` binding
    into a nightly-tier interop binding LOSES `functional/verify` and is refused, even
    though the requirement still has "some" non-unit evidence and the total is unchanged
    (AC-14). Nightly evidence is additive, never a substitute.

    Deliberately no annotation escape, for the same reason check_coverage_ratchet has none:
    `{gap}` is the move being blocked, not a way through it. The honest exits are to
    restore the test or retire the requirement id.

    Scoped like its sibling to RFCs enrolled on BOTH sides, so an RFC enrolled in this very
    change is judged by evaluate()'s ordinary rules instead of being accused of losing
    evidence it never had.
    """
    current = nonunit_evidence(tags)
    errs: List[str] = []
    seen: Set[str] = set()
    for req in requirements:
        if req.rfc not in enrolled or req.rfc not in baseline_enrolled:
            continue
        was = baseline_evidence.get(req.rid)
        if not was:
            continue
        lost = was - current.get(req.rid, set())
        if not lost or req.rid in seen:
            continue
        seen.add(req.rid)
        kept = current.get(req.rid, set())
        still = ", ".join(sorted(kept)) if kept else "nothing but unit tests"
        where = f"{req.source}:{req.line}" if req.source else req.rid
        errs.append(
            f"{where}: {req.rid} has lost its {'/'.join(sorted(lost))} evidence -- the "
            f"test(s) of that kind that proved it at HEAD are gone, leaving {still}. "
            f"Non-unit evidence is monotonic and each tier ratchets on its own: a unit "
            f"test proves the algorithm, a functional test proves the daemon exposes the "
            f"behavior, an interop test proves a peer accepts it, and a nightly-tier "
            f"binding never substitutes for a verify-tier one. Restore the test, or retire "
            f"the requirement id if the obligation itself is gone. No annotation satisfies "
            f"this"
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


def row_discloses_a_gap(row: Dict[str, str]) -> bool:
    """Does this docs/features/rfc-status.md row admit the RFC is not fully met?

    One definition, two consumers: a `{gap}` ANNOTATION (check_status_agreement) and a `wrong`
    or `unimplemented` VERDICT (check_audit_disclosure) say the same thing about the same RFC,
    so a verdict must not be able to hide behind a row an annotation could not.

    Under a clean 'Supported' claim, ONLY an explicit non-empty gap note in the Remaining
    column discloses. An empty/whitespace/neutral Remaining does NOT -- that was the
    fail-open: `_NO_GAP_RE.search("")` is None, so a blank Remaining read as "disclosed" and a
    {gap} MUST hid behind clean support (`ai/rules/fail-closed-guards.md`: absence of a claim is
    not a disclosure). A non-'Supported' status (Partial, Not supported, ...) itself discloses
    that the RFC is not fully met, so the row is not advertising clean support.
    """
    remaining = row["remaining"].strip()
    if not row["status"].startswith("Supported"):
        return True
    return bool(remaining) and not _NO_GAP_RE.search(remaining)


def check_status_agreement(
    requirements: Sequence[Requirement],
    rows: Dict[str, Dict[str, str]],
    enrolled: Set[str],
) -> List[str]:
    """A known unmet MUST must not hide behind a clean 'Supported' row.

    Two ledgers that can disagree will disagree. docs/features/rfc-status.md is the
    public claim; a {gap} annotation is a private admission. They must match.

    Enrolment no longer exempts a stem from the DISCLOSURE half. It used to gate the whole
    check, so a `{gap}` on an un-enrolled summary was exempt even when its RFC carried a
    public 'Supported' row -- the private admission and the public claim contradicting each
    other with nothing to notice (plan/spec-rfcgate-4-ledger.md AC-15). Enrolment now gates
    only the MISSING-ROW branch: an un-enrolled, un-rowed RFC makes no public claim to
    contradict, and demanding a row for it would force rows for reference-only summaries
    (AC-16).
    """
    errs: List[str] = []
    for req in requirements:
        ann = req.annotation
        if not ann or ann.kind != "gap":
            continue
        row = rows.get(req.rfc)
        if row is None:
            if req.rfc not in enrolled:
                continue
            errs.append(
                f"{req.rid} is annotated {{gap}} but {req.rfc} has no row in "
                f"docs/features/rfc-status.md; the public ledger must disclose it"
            )
            continue
        status = row["status"]
        remaining = row["remaining"]
        if not row_discloses_a_gap(row):
            errs.append(
                f"{req.rid} is annotated {{gap: {ann.reason[:50]}}} but "
                f"docs/features/rfc-status.md says {req.rfc} is '{status}' with "
                f"'{remaining[:40]}'. A known unmet MUST cannot be advertised as clean "
                f"support -- update the row's Status/Remaining"
            )
    return errs


# --------------------------------------------------------------------------
# The public ledger's edges (plan/spec-rfcgate-4-ledger.md)
# --------------------------------------------------------------------------
# check_status_agreement above reaches for a row only when a {gap} annotation exists, so
# four whole classes of defect sit outside it: an enrolled RFC with no row at all, a summary
# that is neither enrolled nor declared, a public support claim over an empty checklist, and
# a hand-written gap count that disagrees with the summary. Each is guarded below.
#
# Two of the four are HEAD ratchets rather than hard requirements, for the reason
# check_new_summaries records: the existing backlog (32 rowless enrolments) predates the
# guard, and failing on it would block every unrelated commit until 32 product judgements
# were made. Derived from git, never from a checked-in allowlist
# (ai/rules/derive-not-hardcode.md).

# A Status cell that is not a support claim. Exactly two values, and the test is EXACT
# rather than a prefix: "Supported" must never be read as "Unsupported" reversed, and
# "Unsupported on Linux" is a claim about a platform, not an absence of one.
NON_CLAIM_STATUSES = frozenset({"Unsupported", "Future"})


def status_is_a_support_claim(status: str) -> bool:
    """Does this Status cell claim Ze supports the RFC?

    Anything that is not literally `Unsupported` or `Future` is a claim, INCLUDING an empty
    cell. An empty Status is the fail-open case: `row["status"]` is present, so an `ok`-style
    test passes, and a blank cell under an empty checklist would then read as "no claim
    made" when the row's own existence on a page titled "RFC support" is the claim
    (`ai/rules/fail-closed-guards.md`, the zero-value trap). `Experimental` is a claim too:
    it says the code exists, which is precisely what an empty checklist cannot support.
    """
    return status.strip() not in NON_CLAIM_STATUSES


# Phrases that turn a disposition from a statement about the DOCUMENT into a judgement about
# what Ze owes. `non-normative` may only say the RFC imposes nothing; the moment it says Ze
# does not need it, it has laundered an unextracted obligation into a decision (R-4), and
# ai/rules/rfc-compliance.md reserves that judgement to the owner.
#
# SIX SPELLINGS, and that is all this pattern is. It is the named, specific signal -- the
# phrasings that laundering actually reaches for -- and it was never the rule on its own: a
# blacklist accepts every wording nobody thought of, and seven rephrasings of the identical
# laundering walked through it ("Ze is not required to do any of this", "No obligation falls on
# us here", "This RFC is irrelevant for our implementation", and four more). The rule AC-14
# states is carried by non_normative_reason_cites_the_document below, which is POSITIVE and
# therefore fails closed on the wordings this list cannot enumerate
# (ai/rules/fail-closed-guards.md).
_NON_APPLICABILITY_RE = re.compile(
    r"\b(?:not\s+applicable\s+to\s+ze"
    r"|does\s+not\s+apply\s+to\s+ze"
    r"|ze\s+(?:does\s+not|doesn't|never)\b"
    r"|ze\s+has\s+no\b"
    r"|we\s+(?:do\s+not|don't)\s+(?:implement|support|need)"
    r"|out\s+of\s+scope\s+for\s+ze"
    r"|no(?:t)?\s+relevant\s+to\s+ze)",
    re.IGNORECASE,
)

# What a `non-normative` reason must CITE: a property of the document itself. Two arms, because
# the two kinds of evidence differ in whether capitalisation carries the meaning.
#
# The category arm is case-insensitive. An IETF category (Informational, Experimental, Historic,
# Best Current Practice) and the RFC 2119 / RFC 8174 / BCP 14 key-words machinery are named
# things, and a reason that names one is talking about the text.
_DOCUMENT_CATEGORY_RE = re.compile(
    r"\bRFC\s*2119\b"
    r"|\bRFC\s*8174\b"
    r"|\bBCP\s*14\b"
    r"|\bkey[-\s]?words?\b"
    r"|\bInformational\b"
    r"|\bExperimental\b"
    r"|\bHistoric(?:al)?\b"
    r"|\bBest\s+Current\s+Practice\b",
    re.IGNORECASE,
)


def non_normative_reason_cites_the_document(reason: str) -> bool:
    """Does this `non-normative` reason cite a property of the DOCUMENT?

    The positive form of AC-14, and the half that actually guarantees something. A reason may
    cite an IETF category, the presence or absence of the RFC 2119 key-words machinery, or the
    result of a capitalised-keyword scan. A reason that cites none of those says nothing a
    reader could check, and "the document imposes no obligation on any speaker" is exactly the
    claim that must not be assertable by fiat.

    The keyword arm reuses _SITE_KEYWORD_RE -- the same capitalised set the site inventory is
    derived from, so "the keywords the corpus is read in" has one spelling
    (ai/rules/derive-not-hardcode.md). It is CASE SENSITIVE on purpose: the arm exists for a
    claim about the register the document is written in, and a lowercase "must" in ordinary
    prose ("this must be out of scope") is not that claim.

    What this does NOT establish, stated rather than implied: the gate checks that the reason
    CITES a document property, never that the citation is TRUE. Nothing here reads
    rfc/full/<stem>.txt to confirm the category or re-run the scan. It converts an unfalsifiable
    assertion into a checkable one and names who checks it -- a reviewer, at the row -- which is
    strictly more than a phrase blacklist did, and strictly less than proof.
    """
    return bool(_DOCUMENT_CATEGORY_RE.search(reason) or _SITE_KEYWORD_RE.search(reason))


def check_summary_disposition(
    stems: Set[str],
    enrolled: Set[str],
    dispositions: Dict[str, Disposition],
    baseline_dispositions: Set[str],
) -> List[str]:
    """Every summary is enrolled or carries a declared disposition, and a disposition is
    discharged only by enrolment.

    Un-enrolment is the one state the gate could not previously read anything into. Nine
    summaries sat outside every check with no recorded reason, and nothing distinguished
    "the RFC imposes nothing" from "nobody extracted it" from "we do not even have the
    text" (plan/spec-rfcgate-4-ledger.md D3). This makes the remainder a decision.

    Unlike the two ratchets beside it this is a HARD requirement, not a HEAD comparison:
    every summary in the tree is covered from the moment the file lands, because the file
    lands seeded with every un-enrolled stem. There is no backlog to grandfather.

    A `non-normative` reason is judged twice: it must not judge what Ze owes
    (_NON_APPLICABILITY_RE, six named phrasings) and it MUST cite a property of the document
    (non_normative_reason_cites_the_document, the positive requirement that fails closed). The
    second is the one that carries AC-14; read its docstring for what it proves and what it does
    not.

    The discharge is enrolment, and the DELETION of the summary. Both branches used to fire on a
    stem leaving the tree -- keep the row and the stale-disposition branch fires, delete both and
    the left-without-enrolling branch fires -- so no summary could ever be removed. AC-8 exists
    to stop an EXISTING summary returning to the undeclared state, and the first branch here can
    only accuse a stem in `stems`, so a summary that is gone cannot be in that state. The AC-8
    branch is therefore scoped to stems that still exist rather than to a departed-stem
    subtraction: it also covers a row that named no summary at HEAD either, where deleting the
    row is the FIX the stale-disposition branch asks for.
    """
    errs: List[str] = []
    for stem in sorted(stems - enrolled - set(dispositions)):
        errs.append(
            f"rfc/short/{stem}.md is in neither rfc/enrolled.txt nor "
            f"rfc/not-enrolled.txt. Every summary is enrolled or declared: an un-enrolled "
            f"summary with no recorded reason cannot be told apart from one nobody has got "
            f"to yet. Enrol it, or declare it with a kind from "
            f"{sorted(DISPOSITION_KINDS)}"
        )
    for stem in sorted(enrolled & set(dispositions)):
        errs.append(
            f"{stem} is in BOTH rfc/enrolled.txt and rfc/not-enrolled.txt. The two files "
            f"partition the summaries; a stem in both is a contradiction, and resolving it "
            f"by precedence would let one file quietly overrule the other. Remove the "
            f"rfc/not-enrolled.txt row -- enrolment is the discharge"
        )
    for stem in sorted(set(dispositions) - stems):
        errs.append(
            f"rfc/not-enrolled.txt declares {stem}, but rfc/short/{stem}.md does not "
            f"exist. A disposition for a summary nobody wrote records a decision about "
            f"nothing, and it hides the fact that the row is stale"
        )
    for stem in sorted(set(dispositions) & stems):
        disp = dispositions[stem]
        if disp.kind != DISPOSITION_NON_NORMATIVE:
            continue
        if _NON_APPLICABILITY_RE.search(disp.reason):
            # The specific message wins when both halves fire: a reason can cite a category AND
            # launder ("Informational, and ze has no resolver"), and the laundering is what the
            # author has to fix.
            errs.append(
                f"rfc/not-enrolled.txt: {stem} is declared non-normative with a reason "
                f"that judges what ZE owes rather than what the DOCUMENT states: "
                f"{disp.reason[:80]!r}. 'non-normative' means the RFC imposes no MUST-level "
                f"obligation on any speaker. Whether an obligation applies to Ze is a "
                f"conformance judgement (ai/rules/rfc-compliance.md reserves it to the "
                f"owner) -- record 'backlog' or 'blocked' instead"
            )
        elif not non_normative_reason_cites_the_document(disp.reason):
            errs.append(
                f"rfc/not-enrolled.txt: {stem} is declared non-normative with a reason that "
                f"cites nothing about the DOCUMENT: {disp.reason[:80]!r}. 'non-normative' is "
                f"the one kind that makes a claim about conformance, so its reason must rest "
                f"on something a reviewer can check in the text: the RFC's IETF category "
                f"(Informational, Experimental, Historic, Best Current Practice), the presence "
                f"or absence of the RFC 2119 / RFC 8174 / BCP 14 key-words machinery, or the "
                f"result of a capitalised MUST/SHALL/REQUIRED scan over the source. A reason "
                f"that cites none of those cannot be checked or contradicted -- record "
                f"'backlog' or 'blocked' instead"
            )
    # `baseline - current`, so an unreadable baseline is the empty set and accuses nobody.
    #
    # `& stems` is the third discharge: a summary DELETED from the tree. Without it the two
    # branches above and this one closed every exit, so a declared stem could never leave
    # rfc/short/ at all. AC-8 guards an existing summary against returning to the undeclared
    # state, which is unreachable once the summary is gone.
    for stem in sorted((baseline_dispositions & stems) - set(dispositions) - enrolled):
        errs.append(
            f"rfc/short/{stem}.md is still in the tree, but {stem} left "
            f"rfc/not-enrolled.txt without entering rfc/enrolled.txt. A disposition over a "
            f"LIVE summary is discharged by ENROLMENT and by nothing else: deleting the row "
            f"returns the summary to the undeclared state the file exists to abolish. To "
            f"retire the RFC instead, delete rfc/short/{stem}.md in the same commit"
        )
    return errs


def check_status_completeness(
    enrolled: Set[str],
    rows: Dict[str, str],
    baseline_rows: Optional[Dict[str, Dict[str, str]]],
    newly_enrolled: Optional[Set[str]],
    baseline_enrolled: Set[str],
) -> List[str]:
    """A newly enrolled RFC brings a public status row, and a row does not vanish while its
    RFC stays enrolled.

    32 enrolled RFCs have no row today and the gate is green only by coincidence: nothing
    structural stops one of them from acquiring a `{gap}`, and the moment it does
    check_status_agreement fires the missing-row branch with no warning that the row was
    owed all along (plan/spec-rfcgate-4-ledger.md D2). Those 32 are GRANDFATHERED, exactly
    as check_new_summaries grandfathers the pre-HEAD summary backlog: writing them is 32
    product judgements, several of them deliberate non-implementations, and reddening the
    build on them would get the check removed rather than obeyed. The backlog stays visible
    because `ai/RFC-REQUIREMENTS.md` renders it.

    Both halves judge NOTHING on a degraded baseline. `newly_enrolled` is None when the
    enrolment baseline could not be read, and `baseline_rows` is None when the status blob
    could not be read; in each case the corresponding half is skipped rather than evaluated
    against an empty set, because `current - baseline` over an empty baseline accuses every
    enrolled RFC at once (`_git_baseline_summary_stems` records what that does to a gate).
    """
    errs: List[str] = []
    for stem in sorted(newly_enrolled or ()):
        if stem in rows:
            continue
        errs.append(
            f"{stem} is newly enrolled but has no row in docs/features/rfc-status.md. "
            f"Enrolling gates its MUST-level requirements, so the public ledger must "
            f"disclose the RFC: add a row with a Status, an Implemented coverage note and "
            f"a Remaining note. RFCs enrolled before this ratchet existed are "
            f"grandfathered and unaffected"
        )
    if baseline_rows is None:
        return errs
    for stem in sorted(enrolled & baseline_enrolled & set(baseline_rows)):
        if stem in rows:
            continue
        errs.append(
            f"{stem} had a row in docs/features/rfc-status.md at HEAD and does not now, "
            f"while it stays enrolled. Deleting a row retires a public claim without "
            f"retiring the obligation behind it, and it is the one edit that can make "
            f"check_status_agreement's missing-row branch fire on unrelated work later. "
            f"Restore the row, or correct it in place"
        )
    return errs


def check_unproven_support(
    requirements: Sequence[Requirement],
    rows: Dict[str, Dict[str, str]],
    stems: Set[str],
    dispositions: Dict[str, Disposition],
    signed: Dict[str, "Extraction"],
    derived: Dict[str, str],
) -> List[str]:
    """A public support claim may not rest on an empty checklist.

    Four summaries declared ZERO MUST-level requirements while the public page claimed
    support for them, and check_status_agreement could see none of it: it reaches for a row
    only when a `{gap}` annotation exists, and a summary with no gated requirement has no
    annotation. Both ledgers agreed because both were empty
    (plan/spec-rfcgate-4-ledger.md D1). Agreement on nothing is not conformance.

    Two escapes, and both are EVIDENCE rather than assertion:

    - a `non-normative` disposition in rfc/not-enrolled.txt, whose reason must say what
      makes the DOCUMENT impose nothing (check_summary_disposition rejects one that judges
      what Ze owes instead); or
    - a VALID `manual-walk` extraction sign-off carrying a register-reason, over a source the
      derivation does NOT grade `rfc2119`. That is the form Owner Ruling OR-A settled for RFC
      3765, an Informational document with no RFC 2119 section and no capitalised keyword
      anywhere: zero is a property of the text, and a gate that reddened on it would be
      reddening on honest work (ai/rules/rfc-compliance.md).

    OR-A requires the escape to establish that zero gated requirements is a property of the
    DOCUMENT, and three of its four facts are about the artifact rather than the text: the walk
    performed (every site and section classified, or the artifact is not in `signed`), the
    register declared, and the reason recorded. The declared register constrains nothing on its
    own -- `manual-walk` is the WEAKEST grade in _REGISTER_STRENGTH and _evaluate_extraction
    refuses only a claim STRONGER than derived, so any stem in the corpus may declare it. A
    source whose own sentences quote capitalised MUST, a summary declaring zero, a `Supported`
    row and a manual-walk artifact excluding every site as `not-a-requirement` therefore walked
    straight through, which is the reverse of what the escape was for.

    The fourth fact is the DERIVED grade (`derived_registers`), and it is the only one read off
    the text: `rfc2119` means the document's own sentences quote capitalised MUST/SHALL, so it
    plainly imposes obligations and no register-reason can make zero a property of it. The line
    is drawn there rather than at "any derived site", because `prose` is exactly the register
    OR-A's own case derives -- rfc3765 has one lowercase modal and zero capitalised keywords --
    and a walk may legitimately classify indicative prose away.

    What the fourth fact does NOT establish, stated rather than implied: a `prose` grade is not
    proof the classifications were right. It bounds the escape to documents whose obligations
    are not written in the register the corpus is normally read in; judging the exclusions
    themselves is /ze-rfc-audit's job, and ai/rules/rfc-compliance.md:53 voided every
    annotation as authority.

    Scoped to stems that HAVE a summary. 19 rows on the page name an RFC with no
    `rfc/short/*.md` at all; those are a different defect (an unsummarised public claim)
    and firing here would bury this one under them. The limit is stated in the message.
    """
    errs: List[str] = []
    gated: Dict[str, int] = {}
    for req in requirements:
        if req.gated:
            gated[req.rfc] = gated.get(req.rfc, 0) + 1
    for stem in sorted(set(rows) & stems):
        if gated.get(stem, 0):
            continue
        if not status_is_a_support_claim(rows[stem]["status"]):
            continue
        disp = dispositions.get(stem)
        if disp is not None and disp.kind == DISPOSITION_NON_NORMATIVE:
            continue
        status = rows[stem]["status"].strip() or "(blank)"
        art = signed.get(stem)
        if (
            art is not None
            and art.register == REGISTER_MANUAL_WALK
            and art.register_reason
        ):
            # The escape is REACHED. Whether it is EARNED is the derived grade's answer, and a
            # refusal here gets its own message: telling an author who already wrote a
            # manual-walk sign-off to write one is a dead end
            # (ai/rules/error-messages.md leg 3, a remediation must be TRUE).
            grade = derived.get(stem)
            if grade is not None and grade != REGISTER_RFC2119:
                continue
            says = (
                repr(grade)
                if grade
                else "no register at all (its text could not be read)"
            )
            errs.append(
                f"{art.path} signs {stem} under {REGISTER_MANUAL_WALK!r} with a "
                f"register-reason, but the source derives {says}"
                f", so nothing establishes that zero MUST-level requirements is a property "
                f"of the DOCUMENT -- and docs/features/rfc-status.md claims {stem} is "
                f"'{status}'. A source graded {REGISTER_RFC2119!r} quotes capitalised "
                f"MUST/SHALL in its own sentences: extract those obligations "
                f"(/ze-rfc {stem}). {REGISTER_MANUAL_WALK!r} is the weakest grade and any "
                f"stem may declare it, so the declared register cannot carry this claim"
            )
            continue
        errs.append(
            f"docs/features/rfc-status.md claims {stem} is '{status}', but "
            f"rfc/short/{stem}.md declares no MUST-level requirement, so the claim rests "
            f"on an empty checklist and nothing can contradict it. Extract the RFC's "
            f"obligations (/ze-rfc {stem}); or, if the document genuinely imposes none, "
            f"record the evidence -- a non-normative disposition in rfc/not-enrolled.txt, "
            f"or a manual-walk extraction sign-off whose register-reason says why zero is "
            f"a property of the text, over a source the derivation does not grade "
            f"{REGISTER_RFC2119!r}. Rows naming an RFC with no summary at all are outside "
            f"this check"
        )
    return errs


# Spelled cardinals One..Ninety-nine. Built rather than listed so the compound forms cannot
# drift from the units they are made of.
_SPELLED_UNITS = (
    "one two three four five six seven eight nine ten eleven twelve thirteen fourteen "
    "fifteen sixteen seventeen eighteen nineteen"
).split()
_SPELLED_TENS = (
    "twenty",
    "thirty",
    "forty",
    "fifty",
    "sixty",
    "seventy",
    "eighty",
    "ninety",
)
SPELLED_NUMBERS: Dict[str, int] = {w: i for i, w in enumerate(_SPELLED_UNITS, 1)}
for _i, _ten in enumerate(_SPELLED_TENS):
    SPELLED_NUMBERS[_ten] = 20 + 10 * _i
    for _j, _unit in enumerate(_SPELLED_UNITS[:9], 1):
        SPELLED_NUMBERS[f"{_ten}-{_unit}"] = 20 + 10 * _i + _j

# LONGEST FIRST, so "twenty-five" wins over "five" and is read as 25. A units-first
# alternation matches the tail of every compound and turns "Twenty-five MUSTs" into 5 --
# a false mismatch on an honest row, which is the way a check gets deleted rather than
# obeyed. `\b` on the left additionally stops "-five" being reached at all.
_SPELLED_ALT = "|".join(sorted(SPELLED_NUMBERS, key=len, reverse=True))

# IMMEDIATE adjacency: the number, whitespace, then MUST or SHALL. Nothing between.
#
# Not a tolerance window, and this is the load-bearing decision of the check. The page uses
# a SECOND convention: a spelled number immediately before MUST is the {gap} count, while a
# spelled number NEAR MUST is often the {not-applicable} count ("Sixty-four further MUSTs
# bind PE roles ze does not...", where 64 is the not-applicable count against 15 gaps).
# Measured on the committed page: strict adjacency matches 60 rows and agrees with all 60.
# A window matches the same 60 but would, on any looser boundary, read those four
# not-applicable counts as gap counts and red four honest rows on day one.
#
# The keyword may be pluralised. Every one of the 60 rows on the page writes "Nine MUSTs",
# not "Nine MUST", and `\b(?:MUST|SHALL)\b` matches neither: the boundary between `T` and `s`
# is not a word boundary, so the whole check silently judged nothing. `s?` before the `\b` is
# what makes the 60 rows reachable at all.
_GAP_COUNT_RE = re.compile(
    r"\b(?P<num>" + _SPELLED_ALT + r")\s+(?:MUST|SHALL)s?\b", re.IGNORECASE
)


def spelled_gap_count(remaining: str) -> Optional[int]:
    """The gap count a Remaining cell spells out, or None when it spells none.

    None is not zero. A cell with no spelled count is not claiming zero gaps, it is making
    no numeric claim at all, and judging it against a real count would invent a
    disagreement (`Zero MUSTs` is likewise no claim about a gap and does not parse: `zero`
    is absent from SPELLED_NUMBERS).
    """
    m = _GAP_COUNT_RE.search(remaining)
    if m is None:
        return None
    return SPELLED_NUMBERS[m.group("num").lower()]


def check_gap_count_agreement(
    requirements: Sequence[Requirement], rows: Dict[str, Dict[str, str]]
) -> List[str]:
    """A spelled MUST-gap count in a Remaining cell must equal the real count.

    60 rows carry one and all 60 are correct, by discipline alone with nothing enforcing it
    (plan/spec-rfcgate-4-ledger.md D6). This is the only fact on the page a machine can
    own: Area is an editorial label, Implemented coverage is source-anchored prose, and
    Status is a product judgement. A COUNT is a fact about how many annotations exist -- it
    is never a claim that their classifications are right, which matters because
    ai/rules/rfc-compliance.md:53 voided every annotation as authority. Deriving the
    Remaining TEXT from those classifications would launder a void judgement into generated
    prose; cross-checking their number does not.

    Coverage limit, stated here because the gate's own message states it too: SEVEN rows carry a
    number this check does not judge, against the 60 it does. Four spell it in DIGITS (rfc7166
    "17 MUST", rfc7311 "2 MUST", rfc9012 "51 MUST", rfc9830 "20 MUST"), and three put words
    between a spelled number and the keyword (rfc5575 "Four Section 6 validation MUSTs", rfc9085
    and rfc9514 "origination/encode MUSTs"). This check reads a spelled number sitting
    immediately before MUST/SHALL and nothing else. Widening it means first normalising the
    page's two counting conventions, which is editorial work on the page rather than gate work.

    `Sixty-four` is NOT one of the seven, and the earlier note here said it was: it parses to 64
    perfectly well (SPELLED_NUMBERS covers One..Ninety-nine), it is skipped by ADJACENCY because
    rfc7432 writes "Sixty-four further MUSTs", and that row is judged anyway on its first match,
    "Fifteen MUST gaps". Two rows sit in both populations for the same reason -- rfc9012 and
    rfc9830 each carry an unjudged digit count AND an unjudged separated count -- and they are
    counted once, under digits. Re-measured 2026-07-30 against the committed page.
    """
    errs: List[str] = []
    gaps: Dict[str, int] = {}
    for req in requirements:
        if req.annotation and req.annotation.kind == "gap":
            gaps[req.rfc] = gaps.get(req.rfc, 0) + 1
    for stem in sorted(rows):
        claimed = spelled_gap_count(rows[stem]["remaining"])
        if claimed is None:
            continue
        real = gaps.get(stem, 0)
        if claimed == real:
            continue
        errs.append(
            f"docs/features/rfc-status.md says {stem} has {claimed} MUST-level gap(s), but "
            f"rfc/short/{stem}.md carries {real} {{gap}} annotation(s). One of the two is "
            f"wrong. Only a spelled number sitting immediately before MUST or SHALL is "
            f"read as a gap count; a digit count, or a number further from the keyword, is "
            f"outside this check"
        )
    return errs


# --------------------------------------------------------------------------
# Fingerprints (drive /ze-rfc-audit staleness)
# --------------------------------------------------------------------------
def _normalize(src: str) -> str:
    lines = [line.strip() for line in src.split("\n")]
    return "\n".join(line for line in lines if line)


# The width of every fingerprint this module records, in hex characters. ONE constant, consumed
# by both producers below and by the schema check that validates a recorded value, so the shape
# the gate accepts is derived from the shape it emits and the two cannot drift
# (`ai/rules/derive-not-hardcode.md`).
SHA_HEX_LEN = 16
# Matched with `fullmatch`, not `match`. Python's `$` also matches immediately BEFORE a final
# newline, so `match` accepts a 17-character `"a"*16 + "\n"` -- an "invalid above" value waved
# through by the very check meant to catch it.
#
# `\Z`, not `$`, for the same reason one notch further out. An anchored `search` is equivalent to
# `match`, NOT to `fullmatch`, so with `$` the pattern re-opened that exact hole for a
# `search`-based caller: `re.search` accepted `"a"*16 + "\n"`, and a comment here used to claim
# the anchors made it "safe" for one. `\Z` matches only at the very end of the string, so `search`,
# `match` and `fullmatch` now all reject the trailing-newline value and the claim is true rather
# than merely written down. `fullmatch` behaviour is identical under either anchor (it consumes the
# whole string by definition), so the two `assertRegex` call sites in the test module -- whose
# inputs have already cleared `_sha_value` -- are unaffected.
_SHA_RE = re.compile(r"^[0-9a-f]{" + str(SHA_HEX_LEN) + r"}\Z")


def requirement_sha(text: str) -> str:
    return hashlib.sha256(_normalize(text).encode("utf-8")).hexdigest()[:SHA_HEX_LEN]


def test_sha(src: str) -> str:
    return hashlib.sha256(_normalize(src).encode("utf-8")).hexdigest()[:SHA_HEX_LEN]


def recorded_map(verdict: Dict, key: str) -> Dict[str, str]:
    """One RECORDED fingerprint map, with absent and empty as the same state.

    `_sha_map` established that reading at LOAD time ("absent reads as empty"). This is the same
    normalisation at COMPARISON time, and the two have to agree. They did not: `_verdict_claims`
    tests the map for falsiness, so absent and `{}` are one state to the schema, while the
    freshness rule compared the raw `verdict.get("tests")` against a computed `{}` -- and
    `None == {}` is False.

    The cost of that gap was a permanently red gate on the one state OR-1 created. A
    `not-applicable` verdict cites no test, `ai/skills/ze-rfc-audit.md` tells the author to omit
    the field, and the omitted spelling then read STALE_UNIT forever with a message that was
    false in all three of its clauses: no tagged test was ever gone (OR-1 FORBIDS citing one),
    it is not a line shift, and re-running `/ze-rfc-audit` reproduces the identical record.
    `--reseal` refused it too, so nothing cleared it (`ai/rules/fail-closed-guards.md`, the
    zero-value trap: a present-but-empty value must not diverge from an absent one;
    `ai/rules/error-messages.md` leg 3: a remediation must be TRUE, not merely present).

    A wrong TYPE reads as empty here rather than raising: `_validate_verdict` already refused it
    at load time, and this is on the LEDGER RENDER path where raising would take down a report
    that must stay readable (`audit_freshness` makes the same trade for an unresolvable key).
    """
    val = verdict.get(key)
    return val if isinstance(val, dict) else {}


# --------------------------------------------------------------------------
# The audit record's schema (plan/spec-rfcgate-3-audit-teeth.md)
# --------------------------------------------------------------------------
# The verdict vocabulary of ai/skills/ze-rfc-audit.md, as a closed enum. Before this it was
# prose in a skill file that no code read: the freshness rule compared the requirement sha and
# the tests map and never looked at the value, so `weak` and `wrong` -- which that skill calls
# "the valuable outputs" -- were treated exactly like `enforced`. The one mechanism designed to
# surface a bad test wrote its findings into a field nothing read, and the vocabulary had
# already drifted unnoticed (`implemented` on RFC7606-5.1-2).
VERDICT_ENFORCED = "enforced"
VERDICT_WEAK = "weak"
VERDICT_WRONG = "wrong"
VERDICT_UNIMPLEMENTED = "unimplemented"
# Owner ruling OR-1 (Thomas, 2026-07-29). A requirement with genuinely no reachable code path
# had NO legal state: `enforced` with an empty `tests` map is refused (a claim of proof with no
# cited test), and the `code`-map remedy is only open to `unimplemented`. RFC7606-8-1 -- "a
# document that specifies a new BGP attribute MUST provide specifics regarding what constitutes
# an error" -- binds the AUTHORS of future specifications, so there is nothing in Ze to prove or
# to break. Abusing `enforced` for it was the honest reading of a schema that had no honest
# state. NOT a loophole in AC-5: it costs a mandatory `no_code_path` reason AND an agreeing
# {not-applicable} annotation, i.e. two committed facts where `enforced` needs one word.
VERDICT_NOT_APPLICABLE = "not-applicable"

AUDIT_VERDICTS = frozenset(
    {
        VERDICT_ENFORCED,
        VERDICT_WEAK,
        VERDICT_WRONG,
        VERDICT_UNIMPLEMENTED,
        VERDICT_NOT_APPLICABLE,
    }
)

# `enforced` is the ONLY verdict that means proven (ai/skills/ze-rfc-audit.md). Everything
# else is subtracted from the published proven count and named with its reason (AC-24).
UNPROVEN_VERDICTS = frozenset(AUDIT_VERDICTS - {VERDICT_ENFORCED})

# A finding ABOUT A TEST, as opposed to a statement about the code (`unimplemented`) or about
# reachability (`not-applicable`). These are the two the findings ratchet protects from quiet
# deletion and from a silent upgrade to `enforced` (AC-11, AC-12): a finding that can be
# deleted is a finding that will be.
FINDING_VERDICTS = frozenset({VERDICT_WEAK, VERDICT_WRONG})

_AUDIT_FILE_KEYS = frozenset(
    {"rfc", "audited", "requirements", "reaudit_note", "reaudit_history"}
)
_VERDICT_KEYS = frozenset(
    {
        "verdict",
        "note",
        "requirement_sha",
        "tests",
        "units",
        "code",
        "upgrade_reason",
        "no_code_path",
    }
)
# Only these two carry a fingerprint MAP whose keys become filesystem reads.
_FINGERPRINT_MAPS = ("tests", "units", "code")

# A fingerprint key is `<repo-relative path>:<line>`. Validated because a verdict is
# agent-authored input that a build gate turns into an open(): it is not a trusted path
# source (Security Review, Path handling).
_FINGERPRINT_KEY_RE = re.compile(r"^(?P<file>[^:\0]+):(?P<line>\d+)$")


def _fingerprint_key(key: str, where: str) -> Tuple[str, int]:
    """Split `file:line`, refusing anything that could read outside the tree."""
    m = _FINGERPRINT_KEY_RE.match(key)
    if not m:
        raise ParseError(
            f"{where}: fingerprint key {key!r} is not '<repo-relative-path>:<line>'"
        )
    rel = m.group("file")
    if rel.startswith("/") or rel.startswith("~") or ".." in rel.split("/"):
        raise ParseError(
            f"{where}: fingerprint key {key!r} names a path outside the repository. A "
            f"verdict is authored input, not a trusted path source"
        )
    return rel, int(m.group("line"))


def _sha_value(sha: object, where: str) -> str:
    """One recorded fingerprint, checked for SHAPE and not merely for non-emptiness.

    A recorded sha is agent-authored input, and the previous check accepted any non-empty string.
    A malformed one is not inert but it is not silent either: it compares unequal to the computed
    value and resolves to STALE_UNIT, which degrades toward MORE checking, so this was never an
    unsound-green defect (`ai/rules/fail-closed-guards.md`). It is the wrong DIAGNOSIS, though --
    it sends a reader to re-audit a requirement whose judgement never moved, and the remediation
    STALE prints ("re-run /ze-rfc-audit") does not name the actual fault, which leaves leg 3 of
    `ai/rules/error-messages.md` present but untrue.

    Length alone is not the check. A value truncated and then padded to 16 characters has the
    right length and is still not a fingerprint, so the charset is validated too -- and that is
    the case the boundary tests pin alongside 15 and 17, because it is the one a pure length
    check would wave through.
    """
    if isinstance(sha, str) and _SHA_RE.fullmatch(sha):
        return sha
    if isinstance(sha, str):
        got = f"a {len(sha)}-character string"
    else:
        got = f"a {type(sha).__name__}"
    raise ParseError(
        f"{where}: {sha!r} is not a fingerprint. Expected exactly {SHA_HEX_LEN} lowercase hex "
        f"characters, as produced by requirement_sha()/test_sha(); got {got}. Replace the "
        f"recorded value with the computed one -- a hand-edited fingerprint is never valid, and "
        f"`make ze-rfc-reseal` cannot repair it because it loads through this same check"
    )


def _sha_map(verdict: Dict, key: str, where: str) -> Dict[str, str]:
    """One validated fingerprint map. Absent reads as empty; a wrong TYPE never does.

    The zero-value trap this closes: `verdict.get("tests")` returning a string, a list, or a
    map of maps used to flow straight into an equality comparison, where any of them compares
    unequal to the computed shas and reported as STALE -- a real defect wearing the costume of
    a routine re-read (`ai/rules/fail-closed-guards.md`).
    """
    val = verdict.get(key)
    if val is None:
        return {}
    if not isinstance(val, dict):
        raise ParseError(
            f"{where}: {key!r} must be an object, got {type(val).__name__}"
        )
    out: Dict[str, str] = {}
    for k, v in val.items():
        if not isinstance(k, str):
            raise ParseError(f"{where}: {key!r} has a non-string key {k!r}")
        _fingerprint_key(k, f"{where}: {key}")
        out[k] = _sha_value(v, f"{where}: {key}[{k!r}]")
    return out


def _validate_verdict(rfc: str, rid: str, verdict: object, where: str) -> None:
    """One verdict's STRUCTURE. Raises ParseError, which every driver turns into exit 2.

    Structure only: the cross-referential rules (does the rid exist, do the tags cover both
    polarities, does the annotation agree) need the requirements and live in
    `check_audit_schema`. Splitting them that way follows child 1's precedent -- a malformed
    artifact is a ParseError, an unsatisfied CLAIM is a reported violation -- and it matters
    because a ParseError aborts the whole run while a violation lets the other 900 be seen.
    """
    if not isinstance(verdict, dict):
        raise ParseError(
            f"{where}: {rid} must be an object, got {type(verdict).__name__}"
        )
    _reject_unknown_keys(verdict, _VERDICT_KEYS, f"{where}: {rid}")
    value = verdict.get("verdict")
    if value not in AUDIT_VERDICTS:
        raise ParseError(
            f"{where}: {rid} has verdict {value!r}, which is not one of "
            f"{sorted(AUDIT_VERDICTS)}. The vocabulary is closed (ai/skills/ze-rfc-audit.md): "
            f"a fifth word is drift, and drift in this field is a compliance claim nobody can "
            f"read"
        )
    _str_field(verdict, "note", f"{where}: {rid}")
    # The boundary row names all three recorded fingerprints (`requirement_sha`, test sha, unit
    # sha), so the shape check covers the scalar too and not only the maps. `_str_field` runs
    # first, so a wrong TYPE keeps its own message; the shape check then reads the RAW value, NOT
    # `_str_field`'s return, because that return is `.strip()`ed while the record keeps what was
    # written and `verdict_freshness` compares THAT. Checking the stripped form passed a
    # `"<16 hex>\n"` value here and then let it read STALE_REQUIREMENT forever -- validating
    # something other than what the consumer reads is how a fail-open hides inside a guard.
    _str_field(verdict, "requirement_sha", f"{where}: {rid}")
    _sha_value(verdict.get("requirement_sha"), f"{where}: {rid}: 'requirement_sha'")
    for key in _FINGERPRINT_MAPS:
        _sha_map(verdict, key, f"{where}: {rid}")
    if verdict.get("upgrade_reason") is not None:
        _str_field(verdict, "upgrade_reason", f"{where}: {rid}", required=False)
    # `no_code_path` means exactly one thing, so it may only appear where it means it. A field
    # that sits unread on the other four verdicts is a field an author can believe they filled
    # in (ai/rules/fail-closed-guards.md: a guard that cannot deny must at least say something).
    if "no_code_path" in verdict:
        if value != VERDICT_NOT_APPLICABLE:
            raise ParseError(
                f"{where}: {rid} carries 'no_code_path' with verdict {value!r}. That field "
                f"states why no reachable code path exists and is only meaningful on "
                f"{VERDICT_NOT_APPLICABLE!r}"
            )
        # The one string field that was never type-checked. `_verdict_claims` reads it as
        # `str(verdict.get("no_code_path") or "").strip()`, so `123`, `["a"]`, `{"k": "v"}` and
        # `true` all satisfied OR-1's MANDATORY prose reason -- while its siblings `note`,
        # `requirement_sha` and `upgrade_reason` go through `_str_field` and refused exactly
        # that. `required=False` on purpose: absent-or-empty stays the REPORTED violation
        # `_verdict_claims` owns (which names what to write), and only a wrong TYPE raises here.
        _str_field(verdict, "no_code_path", f"{where}: {rid}", required=False)


def audit_stems() -> Set[str]:
    """Every stem with an rfc/audit/<stem>.json.

    The direction `check_audit_freshness` never walked. It iterates REQUIREMENTS and asks each
    for its verdict, so an audit file for a stem that is not enrolled, or has no summary at
    all, was read by nothing and reported by nothing -- a whole file of judgements that looked
    like evidence and was not even loaded (AC-4).
    """
    if not os.path.isdir(AUDIT_DIR):
        return set()
    return {n[: -len(".json")] for n in os.listdir(AUDIT_DIR) if n.endswith(".json")}


def load_audit(rfc: str) -> Dict[str, Dict]:
    """Read and VALIDATE rfc/audit/<rfc>.json: /ze-rfc-audit's per-requirement verdicts.

    Before this the body was a bare `json.load` returning `data.get("requirements", {})`: no
    field-presence check, no enum check, no type check, and every other top-level key silently
    discarded. The vocabulary had already drifted and nothing noticed, because nothing looked.
    """
    path = os.path.join(AUDIT_DIR, rfc + ".json")
    rel = f"rfc/audit/{rfc}.json"
    if not os.path.exists(path):
        return {}
    try:
        with open(path, encoding="utf-8") as fh:
            data = json.load(fh)
    except (OSError, ValueError) as exc:
        raise ParseError(f"{rel}: cannot read: {exc}") from exc
    if not isinstance(data, dict):
        raise ParseError(f"{rel}: expected a JSON object, got {type(data).__name__}")
    _reject_unknown_keys(data, _AUDIT_FILE_KEYS, rel)
    stem = _str_field(data, "rfc", rel)
    if stem != rfc:
        raise ParseError(
            f"{rel}: 'rfc' is {stem!r} but the filename says {rfc!r}. The record names the "
            f"RFC it judges; the two can never drift apart"
        )
    _str_field(data, "audited", rel)
    history = data.get("reaudit_history")
    if history is not None and (
        not isinstance(history, list) or not all(isinstance(h, str) for h in history)
    ):
        raise ParseError(f"{rel}: 'reaudit_history' must be a list of strings")
    reqs = data.get("requirements")
    if not isinstance(reqs, dict):
        raise ParseError(f"{rel}: 'requirements' must be an object")
    for rid, verdict in reqs.items():
        _validate_verdict(rfc, rid, verdict, rel)
    return reqs


def load_audits(enrolled: Set[str]) -> Dict[str, Dict[str, Dict]]:
    """Every enrolled RFC's verdicts, validated once and shared by every audit check.

    One load point, so the validating parse cannot be reached by one consumer and bypassed by
    another, and so a 166-RFC run pays for each file once instead of once per check.
    """
    return {rfc: load_audit(rfc) for rfc in sorted(enrolled)}


def check_audit_files(enrolled: Set[str], stems: Set[str]) -> List[str]:
    """An audit file must judge an RFC that is enrolled and has a summary (AC-4)."""
    errs: List[str] = []
    for stem in sorted(audit_stems()):
        rel = f"rfc/audit/{stem}.json"
        if stem not in stems:
            errs.append(
                f"{rel}: there is no rfc/short/{stem}.md, so this file records judgements "
                f"about requirements that do not exist. Delete it, or write the summary"
            )
        elif stem not in enrolled:
            errs.append(
                f"{rel}: {stem} is not in rfc/enrolled.txt, so nothing reads these verdicts "
                f"-- an audit file for an un-enrolled RFC is evidence the gate never loads. "
                f"Enrol {stem}, or delete the file"
            )
    return errs


def check_audit_schema(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
) -> List[str]:
    """The cross-referential half of the record's schema: claims that need the requirements.

    `load_audit` proves a verdict is well FORMED. This proves it is not self-contradictory:
    `enforced` with no test cited, `enforced` over a single polarity, `unimplemented` with no
    producing code named, `not-applicable` that the summary does not agree with, or a verdict
    for a requirement that is not there at all.
    """
    if audits is None:
        audits = load_audits(enrolled)
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)
    known: Dict[str, Requirement] = {}
    for req in requirements:
        known.setdefault(req.rid, req)

    errs: List[str] = []
    for rfc in sorted(audits):
        rel = f"rfc/audit/{rfc}.json"
        for rid in sorted(audits[rfc]):
            verdict = audits[rfc][rid]
            req = known.get(rid)
            if req is None or req.rfc != rfc:
                errs.append(
                    f"{rel}: {rid} is not a requirement of {rfc}. A verdict may not describe "
                    f"a requirement that is not there: either the id was renumbered (which "
                    f"the id rules forbid) or the checklist line was deleted under it"
                )
                continue
            errs.extend(
                _verdict_claims(rel, rid, verdict, req, by_rid.get(rid, []), tags)
            )
    return errs


def _verdict_claims(
    rel: str,
    rid: str,
    verdict: Dict,
    req: Requirement,
    found: Sequence[Tag],
    tags: Sequence[Tag],
) -> List[str]:
    """Everything one verdict claims about its requirement, checked against the tree."""
    errs: List[str] = []
    value = verdict["verdict"]
    ann = req.annotation
    polarities = {t.polarity for t in found}

    if value == VERDICT_ENFORCED:
        # AC-5. "Proven" with no cited test is not a weaker claim, it is a contradiction: the
        # verdict's whole content is "the tests would fail if the code stopped complying".
        if not verdict.get("tests"):
            errs.append(
                f"{rel}: {rid} is {VERDICT_ENFORCED!r} with an empty 'tests' map. "
                f"{VERDICT_ENFORCED!r} means the tests would fail if the code stopped "
                f"complying, so it must cite at least one. If no reachable code path could "
                f"satisfy or violate the requirement, the honest verdict is "
                f"{VERDICT_NOT_APPLICABLE!r} with a 'no_code_path' reason and an agreeing "
                f"{{not-applicable}} annotation"
            )
        # AC-6. A negative-only test passes if the code rejects everything and a positive-only
        # one passes if it accepts everything, so a single polarity cannot support "proven".
        if not (ann and ann.kind == "single-polarity"):
            missing = sorted(POLARITIES - polarities)
            if missing:
                errs.append(
                    f"{rel}: {rid} is {VERDICT_ENFORCED!r} but has no "
                    f"{'/'.join(missing)} test (only "
                    f"{'/'.join(sorted(polarities)) or 'none'}). One polarity cannot prove a "
                    f"requirement: add the missing test, or annotate the summary line "
                    f"{{single-polarity: <polarity>; why}}"
                )

    if value == VERDICT_UNIMPLEMENTED:
        # AC-7. An empty `tests` map is legitimate here -- the claim is about CODE, not about a
        # test -- which is exactly what made these verdicts permanently fresh: their freshness
        # test reduced to `{} == {}`. The `code` map is what makes the claim falsifiable again,
        # firing when the gap might have closed.
        if not verdict.get("code"):
            errs.append(
                f"{rel}: {rid} is {VERDICT_UNIMPLEMENTED!r} with an empty 'code' map. A claim "
                f"that the CODE does not comply must name the producing code, or it is "
                f"unfalsifiable: with neither tests nor code fingerprinted, the verdict can "
                f"never go stale and no one is ever asked to look again"
            )
        if not (ann and ann.kind in ("gap", "not-applicable")):
            errs.append(
                f"{rel}: {rid} is {VERDICT_UNIMPLEMENTED!r} but "
                f"{req.source}:{req.line} carries no {{gap}} or {{not-applicable}} "
                f"annotation. The record and the checklist must agree: a divergence Ze knows "
                f"about must be declared where a reader of the summary will meet it"
            )

    if value == VERDICT_NOT_APPLICABLE:
        # Owner ruling OR-1: three facts, not one word. Two of them are committed in different
        # files, so the state cannot be reached by editing the audit record alone.
        if verdict.get("tests"):
            errs.append(
                f"{rel}: {rid} is {VERDICT_NOT_APPLICABLE!r} but cites tests "
                f"({', '.join(sorted(verdict['tests']))}). If a test can exercise it, a "
                f"reachable code path exists and the verdict is a judgement about that test"
            )
        if not str(verdict.get("no_code_path") or "").strip():
            errs.append(
                f"{rel}: {rid} is {VERDICT_NOT_APPLICABLE!r} with no 'no_code_path' reason. "
                f"State WHY no reachable code path could satisfy or violate this requirement: "
                f"a verdict whose only content is its own name is exactly the unfalsifiable "
                f"entry this schema rejects"
            )
        if not (ann and ann.kind == "not-applicable"):
            kind = f"{{{ann.kind}}}" if ann else "no annotation"
            errs.append(
                f"{rel}: {rid} is {VERDICT_NOT_APPLICABLE!r} but {req.source}:{req.line} "
                f"carries {kind}. Two committed facts must agree, so this verdict is legal "
                f"only over a {{not-applicable}} checklist line -- the audit record cannot "
                f"reclassify a requirement on its own"
            )
    return errs


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


def _read_source(rel: str, root: str, cache: Dict[str, str]) -> str:
    """One tracked file's text, read at most once per run."""
    if rel not in cache:
        try:
            with open(
                os.path.join(root, rel), encoding="utf-8", errors="replace"
            ) as fh:
                cache[rel] = fh.read()
        except OSError:
            cache[rel] = ""
    return cache[rel]


def unit_shas(
    keys: Sequence[str], root: str = PROJECT_DIR, where: str = "audit"
) -> Dict[str, str]:
    """Fingerprint the enclosing UNIT of each `file:line`, keyed by that same string.

    The unit is one top-level Go function (its doc comment through its closing brace) or the
    whole file, decided by `rfc_tagged_scope` -- the SAME definition the edit-time guard
    widens an Edit to, so the two can never disagree about which text an obligation covers.

    This is the fix for the false-stale class. `tagged_unit_shas` hashes the whole enclosing
    FILE, so a verdict went stale on any edit anywhere in a tagged file and on any line shift:
    six of a pending sixteen commits to the one existing audit file were mechanical re-stamps
    where nothing about what a test asserts had changed, and not one of them changed a verdict.
    At fleet scale that trains the reflex -- re-stamp when the gate goes red -- and the reflex
    is what fails, not the reading.

    An EMPTY unit is an error, never a hash input (spec R-2). Hashing "" would give every
    unreadable file the same fingerprint, so a deleted test would read as "unchanged" -- a
    false FRESH, the one catastrophic outcome. `tagged_unit_shas` above stores "" for an
    unreadable file, which is safe only because it compares unequal to what was recorded; here
    the same value would be a legitimate-looking answer (`ai/rules/fail-closed-guards.md`).
    """
    out: Dict[str, str] = {}
    cache: Dict[str, str] = {}
    for key in keys:
        rel, line = _fingerprint_key(key, where)
        content = _read_source(rel, root, cache)
        if not content:
            raise ParseError(
                f"{where}: {key} names {rel}, which is missing or empty. There is no unit to "
                f"fingerprint, and hashing nothing would make every missing file look "
                f"identical and therefore unchanged"
            )
        text, _kind = rfc_tagged_scope.unit_at(rel, content, line)
        if not text:
            raise ParseError(
                f"{where}: {key} resolved to an empty unit in {rel}; refusing to fingerprint "
                f"an empty extraction"
            )
        out[key] = test_sha(text)
    return out


def unit_scopes(keys: Sequence[str], root: str = PROJECT_DIR) -> Dict[str, str]:
    """How each `file:line` resolved: `func` or `file`. For error messages only."""
    out: Dict[str, str] = {}
    cache: Dict[str, str] = {}
    for key in keys:
        try:
            rel, line = _fingerprint_key(key, "audit")
        except ParseError:
            continue
        content = _read_source(rel, root, cache)
        if not content:
            continue
        out[key] = rfc_tagged_scope.unit_at(rel, content, line)[1]
    return out


def tag_keys(tags: Sequence[Tag]) -> List[str]:
    """The `file:line` keys a requirement's tags fingerprint under."""
    return [f"{t.file}:{t.line}" for t in tags]


# The four freshness states, mutually exclusive and total. Replacing the boolean is the whole
# point: `fresh` and `stale` collapsed a mechanical re-stamp (nothing about the assertions
# moved) into the same signal as a real judgement change, and the only remedy either offered
# was a human re-read.
FRESH = "fresh"
SHIFTED = (
    "shifted"  # units identical, the enclosing file moved: mechanically re-sealable
)
STALE_UNIT = "stale-unit"  # the tagged unit itself, or cited producer code, changed
STALE_REQUIREMENT = "stale-requirement"  # the RFC obligation's own text changed


def _unit_identity(fingerprints: Dict[str, str]) -> Dict[Tuple[str, str], int]:
    """A unit map as a multiset of (file, unit-sha) -- the identity a LINE SHIFT preserves.

    Comparing the maps key-by-key would defeat the whole point: a `file:line` key changes when a
    nine-line header is prepended, so every verdict in the file would compare unequal and report
    STALE even though the recorded shas are identical. That IS the false-stale class this spec
    exists to remove, and the first implementation reproduced it exactly.

    A COUNT, not a set. Two tags inside one function collapse to one (file, sha) pair, so a set
    would call deleting one of them "unchanged" -- a false FRESH. The count changes, so it does
    not.
    """
    out: Dict[Tuple[str, str], int] = {}
    for key, sha in fingerprints.items():
        rel = key.rsplit(":", 1)[0]
        out[(rel, sha)] = out.get((rel, sha), 0) + 1
    return out


def verdict_freshness(
    verdict: Dict,
    req_sha: str,
    test_shas: Dict[str, str],
    unit_fingerprints: Optional[Dict[str, str]] = None,
    code_fingerprints: Optional[Dict[str, str]] = None,
) -> Tuple[str, List[str]]:
    """Which of the four states a verdict is in, and which keys moved.

    THE freshness rule -- there is no second spelling of it. A `verdict_is_fresh` helper carried
    the pre-`units` file-level rule alongside this function and claimed, in its own docstring,
    that the transitional branch below delegated to it. It did not: the branch was written
    inline, the two spellings then diverged (the helper never consulted the `code` map, so a
    record whose cited producer had moved read fresh there and STALE_UNIT here), and ten
    assertions across six test methods went on giving green assurance about a function the gate
    never called (`TestTransitionalFileLevelRule`'s four cases, `test_verdict_fresh_and_stale`,
    and `test_the_transitional_rule_normalises_it_too`; counted, not estimated). Deleted
    rather than wired up, because the inline version is the stronger of the two
    (`ai/rules/no-layering.md`: delete the old spelling, do not keep it beside the new one).

    Biased to over-trigger, as it has been since the boolean: a false 'stale' costs a re-read, a
    false 'fresh' ships a test that no longer enforces its requirement.

    Order matters and is the same bias: the requirement's own text is checked first (re-reading
    the RFC invalidates every judgement under it), then the units, then the producing code, and
    only then the file-level shift -- so a real judgement change can never be reported as the
    cheap mechanical case.

    A verdict with no `units` (recorded before unit fingerprints existed) takes exactly the old
    file-level rule, applied inline below, so pre-existing records keep behaving as they did and
    the migration is a backfill rather than a re-judgement (AC-20).
    """
    if verdict.get("requirement_sha") != req_sha:
        return STALE_REQUIREMENT, []

    # Every recorded map is read through `recorded_map`, so an ABSENT key and an empty one are
    # one state here exactly as they already were to `_verdict_claims`. Reading the raw dict on
    # the line below used to make the documented way of authoring a `not-applicable` verdict
    # permanently STALE_UNIT with an untrue remedy.
    tests_recorded = recorded_map(verdict, "tests")
    code_recorded = recorded_map(verdict, "code")
    units_recorded = recorded_map(verdict, "units")

    if code_recorded:
        current_code = code_fingerprints or {}
        if _unit_identity(code_recorded) != _unit_identity(current_code):
            moved = sorted(
                k
                for k in set(code_recorded) | set(current_code)
                if code_recorded.get(k) != current_code.get(k)
            )
            return STALE_UNIT, moved or sorted(set(code_recorded) ^ set(current_code))

    # THE transitional branch (AC-20): the whole pre-`units` file-level rule, and the only
    # spelling of it. Every assertion about that rule drives this line -- see
    # `TestTransitionalFileLevelRule` in the test module, which was re-pointed here when the
    # `verdict_is_fresh` duplicate it used to drive was deleted.
    if not units_recorded:
        return (FRESH, []) if tests_recorded == test_shas else (STALE_UNIT, [])

    current_units = unit_fingerprints or {}
    if _unit_identity(units_recorded) != _unit_identity(current_units):
        moved = sorted(
            k
            for k in set(units_recorded) | set(current_units)
            if units_recorded.get(k) != current_units.get(k)
        )
        return STALE_UNIT, moved or sorted(set(units_recorded) ^ set(current_units))
    if tests_recorded != test_shas:
        shifted = sorted(
            k
            for k in set(tests_recorded) | set(test_shas)
            if tests_recorded.get(k) != test_shas.get(k)
        )
        return SHIFTED, shifted
    return FRESH, []


def audit_freshness(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
) -> Dict[str, Tuple[str, List[str]]]:
    """{rid: (state, moved keys)} for every requirement carrying a verdict.

    Derived once and shared by the freshness check, the ledger's proven count and `--reseal`,
    so the three can never disagree about which verdicts are current.
    """
    if audits is None:
        audits = load_audits(enrolled)
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)

    out: Dict[str, Tuple[str, List[str]]] = {}
    for req in requirements:
        if req.rfc not in enrolled:
            continue
        verdict = audits.get(req.rfc, {}).get(req.rid)
        if not verdict:
            continue
        keys = tag_keys(by_rid.get(req.rid, []))
        # An unresolvable fingerprint degrades to STALE_UNIT rather than propagating. A file the
        # gate cannot read is NOT "unchanged": naming the keys it could not resolve sends the
        # verdict for a re-read, which is more checking, never less
        # (`ai/rules/fail-closed-guards.md`). Raising here instead would take the LEDGER RENDER
        # down with it -- a report is not a gate, and a cited producer that has been deleted
        # must still be reportable rather than crashing every consumer of the ledger.
        try:
            units = unit_shas(keys, where=f"rfc/audit/{req.rfc}.json: {req.rid} tests")
            code = unit_shas(
                list(verdict.get("code") or {}),
                where=f"rfc/audit/{req.rfc}.json: {req.rid} code",
            )
        except ParseError:
            out[req.rid] = (
                STALE_UNIT,
                sorted(set(keys) | set(verdict.get("code") or {})),
            )
            continue
        out[req.rid] = verdict_freshness(
            verdict,
            requirement_sha(req.text),
            tagged_unit_shas(by_rid.get(req.rid, [])),
            units,
            code,
        )
    return out


def check_audit_freshness(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
) -> List[str]:
    """A recorded verdict must still describe what it judged.

    This is the hinge between the mechanical half and the semantic half. The gate can prove
    a LINK exists; only a reader can say the test still enforces the requirement's letter
    and spirit. The fingerprint turns "someone should re-read this" into a signal that
    fires exactly when it can have gone wrong.

    A MISSING verdict is not an error: the audit is sampled, the gate is total. But a
    verdict that no longer matches what it judged is worse than none -- it is a stale
    assurance -- so that fails.

    Four states now, and each names the ONE command that clears it. `shifted` is the case this
    gate used to spend a human on: the tagged unit is byte-identical and only the file around
    it moved, so nothing was re-judged and nothing needs re-reading.
    """
    if audits is None:
        audits = load_audits(enrolled)
    states = audit_freshness(requirements, tags, enrolled, audits)
    scopes: Dict[str, str] = {}
    errs: List[str] = []
    for req in requirements:
        state, moved = states.get(req.rid, (FRESH, []))
        if state == FRESH:
            continue
        where = f"{req.source}:{req.line}"
        if state == SHIFTED:
            errs.append(
                f"{where}: {req.rid} has a SHIFTED audit verdict -- the tagged unit is "
                f"byte-identical and only the file around it moved "
                f"({', '.join(moved) or 'a line shift'}), so nothing was re-judged. Re-stamp "
                f"it mechanically: make ze-rfc-reseal"
            )
            continue
        if state == STALE_REQUIREMENT:
            errs.append(
                f"{where}: {req.rid} has a STALE audit verdict -- the REQUIREMENT TEXT changed "
                f"since it was judged, so every judgement under it is void. Re-run: "
                f"/ze-rfc-audit {req.rfc}"
            )
            continue
        if not scopes:
            scopes = unit_scopes([k for _, ks in states.values() for k in ks])
        detail = ", ".join(f"{k} ({scopes.get(k, 'file')}-scoped)" for k in moved)
        errs.append(
            f"{where}: {req.rid} has a STALE audit verdict -- what it judged changed: "
            f"{detail or 'a tagged test is gone'}. This is NOT a line shift and "
            f"make ze-rfc-reseal will refuse it. Re-run: /ze-rfc-audit {req.rfc}"
        )
    return errs


# --------------------------------------------------------------------------
# Mechanical re-seal: remove the human step from the no-judgement class
# --------------------------------------------------------------------------
_RESEAL_NOTE = (
    "Mechanical re-stamp by `make ze-rfc-reseal`. Every verdict re-stamped below was in the "
    "SHIFTED state: its recorded unit fingerprints -- the enclosing top-level function of each "
    "tagged test -- were byte-identical to the tree, and only the file around them had moved (a "
    "line shift, a sibling test, an import rewrite). Nothing about what any of these tests "
    "asserts changed, so no verdict was re-judged and only the `tests` map was rewritten. A "
    "verdict whose unit, cited producer code, or requirement text moved was REFUSED and left "
    "stale for a human. The proof is the code in verdict_freshness(), not this note."
)


def reseal_audits(
    prove=None, note: Optional[str] = None
) -> Tuple[List[str], List[str]]:
    """Re-stamp the verdicts whose tagged units did not change, and only those.

    The ONE definition of the mechanical re-stamp. `scripts/dev/rename_module_path.py` used to
    own a second loop, and `reseal_rfc_audits`'s own docstring names the hazard: a second copy of
    the fingerprint rule that drifted would re-seal verdicts against a hash the gate does not
    compute.

    `prove` is an OPTIONAL extra per-file predicate the caller may impose (the rename tool passes
    `rename_only_since_head`, proving the file differs from HEAD by nothing but the rename). It
    can only ever make the re-seal stricter. It is also what unlocks the TRANSITIONAL case: a
    verdict with no recorded `units` cannot be shown to have an unchanged unit, so it is
    re-sealable only when the caller supplies an independent proof -- which is exactly the
    capability the rename tool had before this became shared, kept rather than lost.

    Returns (resealed, refused).
    """
    enrolled, reqs, _, tags, _ = _collect_for_check()
    audits = load_audits(enrolled)
    states = audit_freshness(reqs, tags, enrolled, audits)
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)

    resealed: List[str] = []
    refused: List[str] = []
    proven: Dict[str, bool] = {}
    for rfc in sorted(enrolled):
        verdicts = audits.get(rfc) or {}
        if not verdicts:
            continue
        touched: List[str] = []
        for req in reqs:
            if req.rfc != rfc or req.rid not in verdicts:
                continue
            verdict = verdicts[req.rid]
            state, _moved = states.get(req.rid, (FRESH, []))
            if state == FRESH:
                continue
            transitional = state == STALE_UNIT and not verdict.get("units")
            if state != SHIFTED and not (transitional and prove is not None):
                refused.append(f"{rfc} {req.rid}: {state}, a human must re-read it")
                continue
            fresh = tagged_unit_shas(by_rid.get(req.rid, []))
            if prove is not None:
                files = {
                    key.rsplit(":", 1)[0]
                    for key in list(verdict.get("tests") or {}) + list(fresh)
                }
                for rel in sorted(files):
                    if rel not in proven:
                        proven[rel] = bool(prove(rel))
                unproven = sorted(f for f in files if not proven[f])
                if unproven:
                    refused.append(
                        f"{rfc} {req.rid}: more than the caller's proof changed in "
                        + ", ".join(unproven)
                    )
                    continue
            verdict["tests"] = fresh
            resealed.append(f"{rfc} {req.rid}")
            touched.append(req.rid)
        if touched:
            _write_audit(rfc, audits[rfc], note or _RESEAL_NOTE)
    return resealed, refused


def _write_audit(rfc: str, verdicts: Dict[str, Dict], note: str) -> None:
    """Rewrite one audit file, preserving the previous re-stamp note into history.

    Staged beside the target then `os.replace`d, copying `run_extract_skeleton`: a refusal or a
    kill must leave the reviewer's existing evidence file untouched
    (`ai/rules/never-destroy-work.md`). Re-read through the validating parser BEFORE it lands, so
    a defect in this writer can never commit a record that later makes every `--check` exit
    "cannot run".

    That re-read is of the STAGED BYTES, not of the in-memory dict. The comment claimed the bytes
    and the code validated the dict, which is not the same guarantee: `--check` reads the file, so
    only the file can prove what `--check` will see, and a JSON round trip is exactly where a
    writer defect would hide (`json.dump` silently stringifies a non-string key, for one). Both
    are checked now -- the dict first, since it still refuses things the round trip would launder.
    """
    path = os.path.join(AUDIT_DIR, rfc + ".json")
    with open(path, encoding="utf-8") as fh:
        data = json.load(fh)
    data["requirements"] = verdicts
    # An earlier re-stamp's reasoning is evidence about the same verdicts. Overwriting it would
    # delete the record of why they were trusted then.
    previous = data.get("reaudit_note")
    if previous:
        data.setdefault("reaudit_history", []).append(previous)
    data["reaudit_note"] = note

    _sweep_stale_staging(AUDIT_DIR)
    staging = tempfile.mkdtemp(prefix=_STAGING_PREFIX, dir=AUDIT_DIR)
    try:
        staged = os.path.join(staging, rfc + ".json")
        with open(staged, "w", encoding="utf-8") as fh:
            json.dump(data, fh, indent=2, sort_keys=True)
            fh.write("\n")
        where = f"rfc/audit/{rfc}.json (staged)"
        for rid, verdict in data["requirements"].items():
            _validate_verdict(rfc, rid, verdict, where)
        with open(staged, encoding="utf-8") as fh:
            reread = json.load(fh)
        for rid, verdict in reread.get("requirements", {}).items():
            _validate_verdict(rfc, rid, verdict, where)
        os.replace(staged, path)
    finally:
        shutil.rmtree(staging, ignore_errors=True)


def run_reseal() -> int:
    """`make ze-rfc-reseal`: the ONLY thing that writes rfc/audit/ without a human editing it.

    Deliberately not folded into `--check` (a check that writes cannot be trusted to report) nor
    into `--write` (`ze-rfc-index` runs routinely, for reasons that have nothing to do with an
    audit, so re-sealing there would automate the blind re-stamp reflex this exists to remove).
    Owner ruling 2026-07-29, spec A-7.
    """
    try:
        resealed, refused = reseal_audits()
    except (ParseError, OSError) as exc:
        print(f"{RED}{BOLD}rfc-requirements: cannot run{RESET}: {exc}")
        return 2
    for line in refused:
        print(f"{YELLOW}refused{RESET} {line}")
    if resealed:
        for line in resealed:
            print(f"{GREEN}re-stamped{RESET} {line}")
        print(
            f"{GREEN}re-sealed{RESET} {len(resealed)} shifted verdict(s); "
            f"{len(refused)} refused. The ledger now needs: make ze-rfc-index"
        )
    else:
        print(
            f"{GREEN}nothing to re-seal{RESET}: no verdict is in the 'shifted' state "
            f"({len(refused)} refused)"
        )
    return 0


# --------------------------------------------------------------------------
# The verdict value becomes load-bearing
# --------------------------------------------------------------------------
def check_audit_disclosure(
    requirements: Sequence[Requirement],
    rows: Dict[str, Dict[str, str]],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
) -> List[str]:
    """A verdict saying the requirement is not met must not hide under a clean support claim.

    `check_status_agreement` already refuses to let a `{gap}` ANNOTATION sit behind a
    'Supported' row with nothing in Remaining. A `wrong` or `unimplemented` VERDICT says the
    same thing -- the requirement is not proven, or not met -- so it must not be weaker.

    The red falls on the PUBLIC CLAIM, which is the right place: it costs the auditor nothing to
    report the finding, and the finding ratchet means reverting the verdict to clear the build
    is itself a failure, so the only exit is the docs edit (R-4).

    `weak` is deliberately NOT here. It says the TEST cannot fail on non-compliance, not that
    the code is wrong, so demanding a public gap note for it would publish a claim the audit
    does not support -- and would make honesty the expensive path, which is the incentive
    inversion this whole spec exists to remove (AC-10).
    """
    if audits is None:
        audits = load_audits(enrolled)
    errs: List[str] = []
    seen: Set[str] = set()
    for req in requirements:
        if req.rfc not in enrolled or req.rid in seen:
            continue
        verdict = audits.get(req.rfc, {}).get(req.rid)
        if not verdict or verdict["verdict"] not in (
            VERDICT_WRONG,
            VERDICT_UNIMPLEMENTED,
        ):
            continue
        seen.add(req.rid)
        row = rows.get(req.rfc)
        if row is None:
            errs.append(
                f"rfc/audit/{req.rfc}.json: {req.rid} is {verdict['verdict']!r} but {req.rfc} "
                f"has no row in docs/features/rfc-status.md; the public ledger must disclose it"
            )
            continue
        if not row_discloses_a_gap(row):
            errs.append(
                f"rfc/audit/{req.rfc}.json: {req.rid} is {verdict['verdict']!r} but "
                f"docs/features/rfc-status.md says {req.rfc} is '{row['status']}' with "
                f"'{row['remaining'][:40]}'. An audited requirement that is not met cannot be "
                f"advertised as clean support -- update the row's Status/Remaining. Reverting "
                f"the verdict is not an exit: the findings ratchet refuses that too"
            )
    return errs


def _git_baseline_audits() -> Optional[Dict[str, Dict[str, Dict]]]:
    """Every committed verdict at HEAD, or None when git could not answer.

    None, NOT an empty map: this is consumed as `baseline - current`, so an empty baseline
    would accuse nobody, but the DISTINCTION still has to survive because `check_audit_findings`
    reports on what the baseline SAID. A fresh clone with no history must accuse nothing
    (R-7, the polarity discipline of `_git_baseline_summary_stems`).

    Read tolerantly per file: a HEAD blob that no longer satisfies today's schema contributes no
    baseline instead of aborting the run. The commit that FIXES a malformed record is exactly the
    one where the tree parses and HEAD does not, and failing there would make the fix
    unlandable.
    """
    try:
        res = subprocess.run(
            ["git", "ls-tree", "-r", "-z", "--name-only", "HEAD", "rfc/audit"],
            cwd=PROJECT_DIR,
            capture_output=True,
            text=True,
            check=False,
        )
    except OSError:
        return None
    if res.returncode != 0:
        return None
    paths = [
        p.strip()
        for p in res.stdout.split("\0")
        if p.strip().endswith(".json") and "/" in p.strip()
    ]
    out: Dict[str, Dict[str, Dict]] = {}
    for rel, blob in _git_cat_blobs(paths).items():
        stem = os.path.basename(rel)[: -len(".json")]
        try:
            data = json.loads(blob)
            verdicts = data["requirements"]
            if not isinstance(verdicts, dict):
                continue
        except (ValueError, KeyError, TypeError):
            continue
        out[stem] = {
            rid: v
            for rid, v in verdicts.items()
            if isinstance(v, dict) and v.get("verdict") in AUDIT_VERDICTS
        }
    return out


def check_audit_findings(
    requirements: Sequence[Requirement],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
    baseline: Optional[Dict[str, Dict[str, Dict]]] = None,
) -> List[str]:
    """A finding is PERMANENT: it cannot be deleted, and it cannot quietly become proof.

    `check_retired_requirements` exists because deleting the checklist line was the cheapest
    route from red to green. The same shape applies one level up: once the verdict value has
    consequences, deleting or upgrading the verdict becomes the cheapest route, so this is not
    decoration -- it is what keeps the rest honest.

    The cost falls on ERASURE, never on reporting: recording `weak` is free and green (AC-10),
    removing one is red, and upgrading one to `enforced` with nothing changed is red.
    """
    if audits is None:
        audits = load_audits(enrolled)
    if baseline is None:
        baseline = _git_baseline_audits()
    if baseline is None:
        return []  # "could not look" is not "nothing was there"

    errs: List[str] = []
    seen: Set[str] = set()
    for req in requirements:
        if req.rfc not in enrolled or req.rid in seen:
            continue
        was = baseline.get(req.rfc, {}).get(req.rid)
        if not was or was.get("verdict") not in FINDING_VERDICTS:
            continue
        seen.add(req.rid)
        now = audits.get(req.rfc, {}).get(req.rid)
        if not now:
            errs.append(
                f"rfc/audit/{req.rfc}.json: the {was['verdict']!r} finding on {req.rid} was "
                f"DELETED. A finding is resolved by fixing the test or retiring the "
                f"requirement, never by removing the record of it -- deletion is the cheapest "
                f"route from red to green and is the one this ratchet exists to close"
            )
            continue
        if now["verdict"] != VERDICT_ENFORCED:
            continue
        # AC-12. An upgrade is a claim that something changed. The recorded UNITS are the
        # machine-checkable half of that claim; `upgrade_reason` is the auditable escape for the
        # cases they cannot see (a helper outside the tagged function, a re-read of the RFC).
        if str(now.get("upgrade_reason") or "").strip():
            continue
        if (now.get("units") or {}) != (was.get("units") or {}):
            continue
        errs.append(
            f"rfc/audit/{req.rfc}.json: {req.rid} went from {was['verdict']!r} to "
            f"{VERDICT_ENFORCED!r} while every tagged unit stayed byte-identical. A finding "
            f"cannot become proof with nothing changed: fix the test (which moves its unit "
            f"fingerprint), or record an 'upgrade_reason' saying what you re-read and why the "
            f"earlier judgement was wrong"
        )
    return errs


def check_audit_verdict_ratchet(
    requirements: Sequence[Requirement],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
    baseline: Optional[Dict[str, Dict[str, Dict]]] = None,
    baseline_enrolled: Optional[Set[str]] = None,
) -> List[str]:
    """Audit coverage is monotonic PER REQUIREMENT ID -- never per percentage.

    A ratio ratchet would fail the build for adding a tagged test, because the denominator
    grows; that punishes coverage work, which is the one behaviour this programme most needs
    (R-6, AC-19). The SET of audited ids has no such perverse case, and the percentage stays a
    published figure that nothing gates.

    Deliberately stricter than "a FRESH verdict may not vanish": presence is what is ratcheted,
    so a verdict that has gone stale must be RE-JUDGED, not deleted. Freshness is exactly the
    state in which deletion is most tempting and least honest.
    """
    if audits is None:
        audits = load_audits(enrolled)
    if baseline is None:
        baseline = _git_baseline_audits()
    if baseline is None:
        return []
    errs: List[str] = []
    seen: Set[str] = set()
    for req in requirements:
        if req.rfc not in enrolled or req.rid in seen:
            continue
        if baseline_enrolled is not None and req.rfc not in baseline_enrolled:
            continue
        if req.rid not in baseline.get(req.rfc, {}):
            continue
        if req.rid in audits.get(req.rfc, {}):
            continue
        seen.add(req.rid)
        errs.append(
            f"rfc/audit/{req.rfc}.json: {req.rid} carried a verdict at HEAD and carries none "
            f"now. Audit coverage is monotonic per requirement id: a judgement that was made "
            f"cannot be un-made by deleting it. Re-judge it (/ze-rfc-audit {req.rfc}) or "
            f"re-stamp it (make ze-rfc-reseal) -- removal is not an option"
        )
    return errs


# An identifier a note can cite: 5+ characters, so English words like "the" and "each" do not
# accidentally satisfy the check while a real symbol name does. Measured over the existing
# corpus: 0 of 49 non-empty-`tests` verdicts fail this against their tagged UNIT text.
_NOTE_IDENT_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]{4,}")


def check_audit_note(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
) -> List[str]:
    """An `enforced` note must name something that actually occurs in the tagged unit.

    No gate can prove a human read the RFC; everything here is a proxy. This is the cheapest
    honest one: the note is the auditor's account of WHY the test would fail on non-compliance,
    and an account of a test one has read almost always names something in it. A lazy writer
    fails it and an honest one passes without noticing (R-1).

    ONE matching token is enough, deliberately, so a prose note is not punished for being prose
    (R-3). When a tagged test is renamed this goes red -- accepted: a renamed test IS a reason
    to re-read, and the message names the tokens checked and the files searched so the fix is
    obvious rather than a guess.
    """
    if audits is None:
        audits = load_audits(enrolled)
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)

    errs: List[str] = []
    seen: Set[str] = set()
    cache: Dict[str, str] = {}
    for req in requirements:
        if req.rfc not in enrolled or req.rid in seen:
            continue
        verdict = audits.get(req.rfc, {}).get(req.rid)
        if not verdict or verdict["verdict"] != VERDICT_ENFORCED:
            continue
        found = by_rid.get(req.rid, [])
        if not found:
            continue  # AC-5/AC-6 already report this; do not pile a second error on it
        seen.add(req.rid)
        blob = ""
        for t in found:
            content = _read_source(t.file, PROJECT_DIR, cache)
            if content:
                blob += rfc_tagged_scope.unit_at(t.file, content, t.line)[0]
        tokens = _NOTE_IDENT_RE.findall(verdict["note"])
        if any(tok in blob for tok in tokens):
            continue
        files = ", ".join(sorted({t.file for t in found}))
        errs.append(
            f"rfc/audit/{req.rfc}.json: {req.rid} is {VERDICT_ENFORCED!r} but its note names "
            f"nothing that occurs in the tagged unit(s). Searched {files}; tokens checked: "
            f"{', '.join(tokens[:12]) or '(none)'}. A note that cannot be tied to the test it "
            f"judges is not evidence that the test was read -- name the assertion, the helper, "
            f"or the constant the judgement turns on"
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


# The same four words in LOWERCASE only, deliberately not case-insensitive. It is the
# evidence a pre-RFC-2119 document offers instead of capitalised keywords: RFC 1035 (1987)
# has 0 uppercase MUST and 23 lowercase `must`, and reading the uppercase count alone
# declares the DNS wire format free of obligations. Evidence in the rendered verdict, never
# a gate input -- whether a lowercase clause binds is a human judgement.
_SRC_PROSE_KEYWORD_RE = re.compile(r"\b(must not|must|shall not|shall)\b")


def source_prose_keyword_count(stem: str) -> Optional[int]:
    """Count LOWERCASE must/shall in the RFC's own text, or None if we don't have it.

    Read alongside `source_keyword_count`, not instead of it. A zero uppercase count with a
    large lowercase count is the pre-2119 signature, and the pair is what tells it apart
    from a genuinely non-normative document (both of which show 0 uppercase).
    """
    text = source_text(stem)
    if text is None:
        return None
    return len(_SRC_PROSE_KEYWORD_RE.findall(text))


def unconverted_summaries(captured: Set[str]) -> List[Dict[str, object]]:
    """Summaries that declare no MUST-level requirement, with the source keyword count.

    A summary listing zero obligations is either a genuinely non-normative reference or a
    capture failure. The difference is visible only against the source text: rfc5303,
    rfc5304 and rfc5310 have 23, 13 and 12 MUST-level keywords in rfc/full/ and captured
    ZERO. Reporting these is the point -- an absent summary is indistinguishable from a
    compliant one, which is how a standards claim rots.

    `captured` must mean "captured a GATED requirement". Its caller passed every parsed
    requirement at any level, so ONE advisory row bought a summary immunity from this table:
    a summary with four SHOULDs and zero MUSTs counted as captured and never appeared here,
    which is exactly the shape this table exists to expose. Seven advisory-only summaries
    were hidden that way (plan/spec-rfcgate-4-ledger.md D5).
    """
    out: List[Dict[str, object]] = []
    for stem in sorted(summary_stems()):
        if stem in captured:
            continue
        out.append(
            {
                "stem": stem,
                "src": source_keyword_count(stem),
                "prose": source_prose_keyword_count(stem),
            }
        )
    return out


def evidence_label(rel: str) -> str:
    """The `kind/tier` cell printed beside a test link, derived from CARRIERS.

    A path with no carrier can only arrive from a synthetic tag (nothing in the tree can
    produce one), and it is labelled visibly wrong rather than plausibly right: an
    unrecognized carrier must never render as though something proved something.
    """
    c = carrier_for(rel)
    return c.label if c is not None else "unknown/unrun"


def evidence_tier(rel: str) -> str:
    c = carrier_for(rel)
    return c.tier if c is not None else TIER_UNRUN


def is_nightly_only(found: Sequence[Tag]) -> bool:
    """A requirement HAS evidence, and none of it runs inside ze-verify.

    The distinction R-1 exists for: a nightly-advisory scenario and a merge-gate unit test
    are both "a tag", and flattening them into one 'proven' cell is how a claim nothing
    blocks on gets read as a claim every merge enforces.
    """
    return bool(found) and all(evidence_tier(t.file) != TIER_VERIFY for t in found)


def tag_kind_counts(tags: Sequence[Tag]) -> Dict[str, int]:
    """Tag totals per `kind/tier` label: {"unit/verify": 2571, "functional/verify": 4}."""
    out: Dict[str, int] = {}
    for t in tags:
        label = evidence_label(t.file)
        out[label] = out.get(label, 0) + 1
    return out


def _evidence_phrase(counts: Dict[str, int]) -> str:
    """Every executable label, INCLUDING the zeros.

    A label omitted when it is zero reads as "not applicable here" rather than "we have
    none of this", which is the whole point of publishing the split. Same reasoning as
    _register_phrase and as check_enrolment's refusal to report clean while enforcing
    nothing.
    """
    known = list(dict.fromkeys(c.label for c in CARRIERS if c.tier != TIER_UNRUN))
    parts = [f"{label} {counts.get(label, 0)}" for label in known]
    parts.extend(f"{k} {counts[k]}" for k in sorted(counts) if k not in known)
    return ", ".join(parts)


class RFCCoverage(NamedTuple):
    rfc: str
    gated: int
    both: int
    one: int
    annotated: int
    missing: int  # gated requirements with no tag and no annotation
    # Gated requirements whose evidence exists but runs in NO ze-verify stage. Its own
    # column, never folded into `both`/`one`: those two are the merge-gate view, and a
    # nightly-only requirement is not merge-gate-proven (AC-11). Defaulted so the
    # positional construction in rfc_coverage stays readable at the call site.
    nightly_only: int = 0

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
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)

    by_rfc: Dict[str, List[Requirement]] = {}
    for r in requirements:
        by_rfc.setdefault(r.rfc, []).append(r)

    out: List[RFCCoverage] = []
    for rfc, reqs in by_rfc.items():
        gated = [r for r in reqs if r.gated]
        if not gated:
            continue
        both = one = ann = missing = nightly = 0
        for r in gated:
            found = by_rid.get(r.rid, [])
            pol = {t.polarity for t in found}
            if is_nightly_only(found):
                nightly += 1
            if r.annotation:
                ann += 1
            elif pol == POLARITIES:
                both += 1
            elif pol:
                one += 1
            else:
                missing += 1
        out.append(RFCCoverage(rfc, len(gated), both, one, ann, missing, nightly))
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
    nightly_total = sum(c.nightly_only for c in cov)
    out.append(
        f"**Nightly-only** ({nightly_total} requirement(s)) counts what is proven ONLY by "
        f"evidence no `ze-verify` stage runs -- today, interop scenarios, which are "
        f"scheduled and advisory. **Both** and **One polarity** are the polarity view: "
        f"they answer which polarities exist, not which pipeline runs them, so a "
        f"nightly-only requirement is counted there too. **Nightly-only** is the tier view "
        f"over the same rows -- an overlapping subset marker naming which of them no "
        f"merge-gate stage proves, never a total to sum with the others."
    )
    out.append("")
    out.append(
        "| RFC | Gated | Both | One polarity | Annotated | No test | Outstanding | "
        "Nightly-only | State |"
    )
    out.append("|---|---|---|---|---|---|---|---|---|")
    for c in cov:
        state = (
            "**enrolled**"
            if c.rfc in enrolled
            else ("enrollable" if c.outstanding == 0 else "backlog")
        )
        out.append(
            f"| `{c.rfc}` | {c.gated} | {c.both} | {c.one} | {c.annotated} | "
            f"{c.missing} | {c.outstanding} | {c.nightly_only} | {state} |"
        )
    out.append("")
    return out


class AuditCoverage(NamedTuple):
    """One RFC's audit row. Every field derived; nothing here is authored anywhere.

    TWO partitions, over two different populations, because one denominator cannot carry both
    questions honestly:

    * `auditable = audited + unaudited` -- the REQUIREMENT view. How much of this RFC an audit
      could be performed on, and how much of that carries a verdict.
    * `verdicts = proven + findings` -- the RECORD view, total over every verdict recorded for
      the RFC, and `findings` is exactly the number of worklist rows it contributes.

    They are not the same population, and conflating them is what this row got wrong: `proven`
    and `findings` counted only the both-polarity subset, so a verdict on a requirement whose
    polarity coverage is ANNOTATED was counted in no column at all -- and if it was a fresh
    `enforced` it was in no worklist row either. Five of the tree's 52 verdicts were invisible.
    """

    rfc: str
    auditable: int  # gated, enrolled, polarity coverage complete: what can be audited
    audited: int  # ...carrying a verdict of any value
    proven: int  # every recorded verdict that is `enforced` AND fresh
    findings: int  # every recorded verdict that is NOT proven (AC-24); == worklist rows
    verdicts: int = 0  # every recorded verdict, i.e. proven + findings

    @property
    def unaudited(self) -> int:
        return self.auditable - self.audited


def polarity_covered(req: Requirement, found: Sequence[Tag]) -> bool:
    """Whether a requirement's polarity coverage is COMPLETE, by the rule the schema uses.

    `_verdict_claims` exempts a `{single-polarity}` requirement from the both-polarity demand:
    the annotation IS the missing polarity's justification, and it is what makes an `enforced`
    verdict legal over one test. `audit_coverage` derived the same fact from tags alone and never
    read `req.annotation`, so the two disagreed about what complete coverage is -- and every
    annotated requirement fell outside `Auditable` while the schema was happy to judge it.
    """
    polarities = {t.polarity for t in found}
    if polarities == POLARITIES:
        return True
    ann = req.annotation
    return bool(polarities) and ann is not None and ann.kind == "single-polarity"


def audit_coverage(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    audits: Optional[Dict[str, Dict[str, Dict]]] = None,
    states: Optional[Dict[str, Tuple[str, List[str]]]] = None,
) -> Tuple[List[AuditCoverage], List[Tuple[str, str, str]]]:
    """Per-RFC audit coverage, and the worklist of every requirement that is not proven.

    AC-24 lives here, and this is deliberately NOT the polarity rollup. `_render_rollup`'s
    **Both** column answers "which polarities exist"; subtracting an audit verdict from it would
    contradict that doctrine outright, and would also break the partition
    `scripts/dev/testing_health.py` asserts (`gated - both` must equal the annotation split, or
    it raises rather than publishing a non-partition as one). So `proven` is a SEPARATE count in
    a separate section: a requirement with both polarities and a `weak` verdict is counted in
    **Both** (it has both polarities -- true) and NOT in `proven` (it is not proven -- also
    true), and the worklist names the verdict as the reason. The ledger can never show one
    requirement as proven and weak at once, which is what AC-24 asks for.

    `proven` requires the verdict to be FRESH as well as `enforced`: a stale verdict describes a
    test that has since changed, so publishing it as proof is the stale assurance this whole
    machinery exists to stop. The gate fails on a stale verdict anyway, so the two agree.

    `proven + findings` is TOTAL over the recorded verdicts, and `findings` is exactly the number
    of worklist rows the RFC contributes (`AuditCoverage` documents the two partitions). Both
    counts used to be gated on the requirement having a tagged pair, which is a different
    population from the record, so the two published numbers described different things while
    reading as one: 44 proven + 3 unproven out of 52 recorded, with five verdicts in neither.
    """
    if audits is None:
        audits = load_audits(enrolled)
    if states is None:
        states = audit_freshness(requirements, tags, enrolled, audits)
    by_rid: Dict[str, List[Tag]] = {}
    for t in tags:
        by_rid.setdefault(t.rid, []).append(t)

    rows: List[AuditCoverage] = []
    worklist: List[Tuple[str, str, str]] = []
    for rfc in sorted(enrolled):
        auditable = audited = proven = findings = verdicts = 0
        seen: Set[str] = set()
        recorded = audits.get(rfc, {})
        for req in requirements:
            if req.rfc != rfc or req.rid in seen:
                continue
            seen.add(req.rid)
            verdict = recorded.get(req.rid)
            # The requirement view keeps child 2's doctrine (gated, and polarity coverage
            # complete), now reading the same coverage rule the schema reads.
            if req.gated and polarity_covered(req, by_rid.get(req.rid, [])):
                auditable += 1
                if verdict:
                    audited += 1
            if not verdict:
                continue
            # The record view is TOTAL over recorded verdicts, and deliberately not gated on
            # either flag above: a verdict is schema-legal on any requirement of the RFC
            # (`check_audit_schema` only demands the rid exist), so counting the both-polarity
            # subset left real judgements in no column and, when fresh and `enforced`, in no
            # worklist row -- the gate then published "everything I hold is proven" over a
            # record that said otherwise. A verdict whose rid matches NO requirement is counted
            # here by neither: `check_audit_schema` owns that as a violation, which is why this
            # walk can be driven from the requirements and still be total.
            verdicts += 1
            value = verdict["verdict"]
            state = states.get(req.rid, (FRESH, []))[0]
            if value == VERDICT_ENFORCED and state == FRESH:
                proven += 1
            else:
                findings += 1
                reason = value if state == FRESH else f"{value} ({state})"
                worklist.append((rfc, req.rid, reason))
        rows.append(AuditCoverage(rfc, auditable, audited, proven, findings, verdicts))
    # Sorted, not append-ordered: the ledger's byte content is compared by check_ledger_fresh, and
    # `requirements` arrives in whatever order the summaries were walked in, so an append-ordered
    # worklist would report a fresh ledger as stale on another machine.
    return rows, sorted(worklist)


def _render_audit_coverage(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
) -> List[str]:
    """The semantic half of the gate's coverage, published rather than gated.

    Without this the audit exists and is invisible: a low-tens count of auditable requirements
    carries a verdict, all of them on one RFC out of 166, and nothing said so anywhere a reader
    would meet it. Publishing per-RFC is also what makes SAMPLING possible -- the only real check
    on whether a verdict was written by someone who read something -- which no gate can perform.
    The figures are deliberately not quoted here: every one of them is rendered a few lines below
    from the live walk, and a docstring copy of them is a number that rots (this one said "44 of
    974" until the coverage rule started reading annotations and both halves moved).

    The COLUMN COUNT here is load-bearing. `scripts/dev/testing_health.py:95` pins the polarity
    rollup with a nine-cell regex (`RFC_ROW`) and matches it against every line of this file, so
    a table whose rows had the same shape would be silently folded into that tool's proof-density
    figure. `TestAuditTableCannotBeMistakenForTheRollup` holds this apart.
    """
    rows, worklist = audit_coverage(requirements, tags, enrolled)
    auditable = sum(r.auditable for r in rows)
    audited = sum(r.audited for r in rows)
    proven = sum(r.proven for r in rows)
    findings = sum(r.findings for r in rows)
    verdicts = sum(r.verdicts for r in rows)
    pct = (100.0 * audited / auditable) if auditable else 0.0

    out: List[str] = []
    out.append("## Audit coverage")
    out.append("")
    out.append(
        f"{audited} of {auditable} auditable requirement(s) carry a `/ze-rfc-audit` verdict "
        f"({pct:.2f}%), across {sum(1 for r in rows if r.audited)} of {len(rows)} enrolled "
        f"RFC(s). **Auditable** = gated, enrolled, and polarity coverage complete: a pair of "
        f"tests, or one test over a `{{single-polarity}}` line saying why the other cannot "
        f"exist. Until then there is nothing for an auditor to judge."
    )
    out.append("")
    out.append(
        f"**Proven** ({proven}) is the count that means what the badge implies: a verdict of "
        f"`{VERDICT_ENFORCED}` -- the tests would fail if the code stopped complying -- that is "
        f"still fresh. It is NOT the **Both** column of the rollup above: that one answers which "
        f"polarities exist, and a requirement can have both and still be judged `{VERDICT_WEAK}`. "
        f"Every one of the {findings} verdict(s) that is audited but not proven is named "
        f"below with its verdict, so no requirement can read as proven and weak at once."
    )
    out.append("")
    out.append(
        f"The remaining {auditable - audited} carry no verdict at all. That is not a violation: "
        f"the audit is sampled and the gate is total, so a missing verdict never fails "
        f"`make ze-rfc-check`. It is published because an unmeasured semantic half is "
        f"indistinguishable from a clean one."
    )
    out.append("")
    out.append(
        f"Two partitions over two populations, because one denominator cannot carry both "
        f"questions. **Requirements:** `Auditable` ({auditable}) = `Audited` ({audited}) + "
        f"`Unaudited` ({auditable - audited}). **Records:** all {verdicts} recorded verdict(s) = "
        f"`Proven` ({proven}) + `Not proven` ({findings}), and the worklist below names every one "
        f"of those {len(worklist)}. A verdict can sit on a requirement that is not auditable -- "
        f"an annotated `{{gap}}` or `{{not-applicable}}` line carries no tagged test -- so the "
        f"record totals are the wider of the two and are never a subset of `Audited`."
    )
    out.append("")
    out.append("| RFC | Auditable | Audited | Proven | Not proven | Unaudited |")
    out.append("|---|---|---|---|---|---|")
    for r in sorted(rows, key=lambda r: (-r.audited, r.rfc)):
        if not r.auditable and not r.audited:
            continue
        out.append(
            f"| `{r.rfc}` | {r.auditable} | {r.audited} | {r.proven} | {r.findings} | "
            f"{r.unaudited} |"
        )
    out.append("")
    out.append("### Audited but not proven")
    out.append("")
    if not worklist:
        out.append(
            "None: every recorded verdict is a fresh `enforced`. On a corpus this size that is "
            "worth reading as a warning as much as a result -- `/ze-rfc-audit` says `weak` and "
            "`wrong` are its valuable outputs, and a run that returns all `enforced` has "
            "probably not read anything."
        )
    else:
        out.append(
            "One row per requirement whose verdict is anything other than a fresh "
            f"`{VERDICT_ENFORCED}`. A blur is not a worklist: each is named so it can be picked "
            "up individually."
        )
        out.append("")
        out.append("| Requirement | Verdict | Meaning |")
        out.append("|---|---|---|")
        for rfc, rid, reason in worklist:
            out.append(f"| `{rid}` | `{reason}` | {_verdict_meaning(reason)} |")
    out.append("")
    return out


_UNPROVEN_MEANING = {
    VERDICT_WEAK: "tagged and green, but cannot fail on non-compliance",
    VERDICT_WRONG: "the test asserts something the RFC does not say",
    VERDICT_UNIMPLEMENTED: "the tests are fine; the CODE does not comply",
    VERDICT_NOT_APPLICABLE: "no reachable code path could satisfy or violate it",
}


# A NON-fresh row's meaning is its STATE, not its verdict word: the judgement itself may be
# perfectly good with only its fingerprints out of date. One line per state, because rendering
# them all as "the verdict no longer describes what it judged" told a `shifted` reader the exact
# opposite of the truth -- `shifted` means the tagged unit IS byte-identical -- and sent them to
# re-read an RFC when one mechanical command clears it.
_STATE_MEANING = {
    SHIFTED: (
        "the tagged unit is byte-identical and only the file around it moved; nothing was "
        "re-judged, so re-stamp it with `make ze-rfc-reseal`"
    ),
    STALE_UNIT: (
        "what it judged changed -- the tagged unit itself, or the producing code it cites; it "
        "must be re-judged with `/ze-rfc-audit` before it counts as anything"
    ),
    STALE_REQUIREMENT: (
        "the requirement's own text changed since it was judged, so every judgement under it is "
        "void; re-run `/ze-rfc-audit`"
    ),
}


def _verdict_meaning(reason: str) -> str:
    """One line per verdict, so the worklist explains itself to a reader who has not read the
    skill. Derived from the reason string the coverage walk produced, never a second table.

    Fails closed on a vocabulary that grows: a verdict added to `UNPROVEN_VERDICTS`, or a state
    added to the four, without a published meaning SAYS so in the ledger rather than rendering an
    empty cell or a wrong one, because an unexplained verdict in a worklist is a row a reader
    silently skips (`ai/rules/fail-closed-guards.md` -- a guard that cannot answer must speak).
    """
    value, _, rest = reason.partition(" ")
    if rest:
        state = rest.strip().strip("()")
        return _STATE_MEANING.get(
            state,
            f"recorded `{value}` in the unpublished freshness state `{state}` -- add it to "
            f"_STATE_MEANING",
        )
    if value not in UNPROVEN_VERDICTS:
        return "outside the recorded vocabulary"
    return _UNPROVEN_MEANING.get(
        value, "no published meaning for this verdict -- add one to _UNPROVEN_MEANING"
    )


def _render_evidence_legend() -> List[str]:
    """What each `kind/tier` cell means, derived from CARRIERS rather than restated.

    Without this the ledger prints a vocabulary it never defines, and a reader has to open
    the scanner to learn whether `interop/nightly` is stronger or weaker than
    `functional/verify` (ai/rules/derive-not-hardcode.md).
    """
    out: List[str] = []
    out.append("## Evidence kinds")
    out.append("")
    out.append(
        "Every test link below carries a `kind/tier` cell. **kind** is the layer the test "
        "exercises; **tier** is whether anything executes it. A unit test proves the "
        "algorithm, a `.ci` proves the daemon exposes the behavior to a user, an interop "
        "scenario proves a foreign peer accepts it -- and a tier of `nightly` means the "
        "proof does not run on the merge path."
    )
    out.append("")
    out.append("| Cell | Carrier | Executed by | Pipeline |")
    out.append("|---|---|---|---|")
    # Collapsed by (label, suffix, runner). The .ci and .et carriers are one row PER SUITE
    # now -- that is what ties a tier to something that runs -- but printing 20-odd rows
    # differing only in a suite name is an inventory, not a legend. The suites are listed
    # in the collapsed row's Pipeline cell instead, still derived, still complete.
    groups: Dict[Tuple[str, str, str], List[Carrier]] = {}
    order: List[Tuple[str, str, str]] = []
    for c in CARRIERS:
        if c.tier == TIER_UNRUN:
            continue
        key = (c.label, c.suffix, c.runner)
        if key not in groups:
            groups[key] = []
            order.append(key)
        groups[key].append(c)
    for key in order:
        rows = groups[key]
        label, suffix, runner = key
        if len(rows) == 1 and not rows[0].derived:
            pipeline = rows[0].pipeline
        else:
            suites = ", ".join(f"`{c.prefix.split('/')[1]}`" for c in rows)
            stage = rows[0].pipeline.rsplit(",", 1)[0] + ")"
            pipeline = f"{stage} -- suites: {suites}"
        out.append(f"| `{label}` | `*{suffix}` | `{runner}` | {pipeline} |")
    out.append("")
    # A catch-all row has no prefix to name, so describe it by shape instead. No inner
    # backticks: the caller wraps each entry in backticks, and nesting them renders as
    # literal characters.
    unrun = sorted(
        {
            c.prefix or f"any *{c.suffix} the table above does not cover"
            for c in CARRIERS
            if c.tier == TIER_UNRUN
        }
    )
    out.append(
        "A tag in a carrier nothing executes is REFUSED by `make ze-rfc-check`, not listed "
        "here with a caveat. These have no automated caller: "
        + ", ".join(f"`{r}`" for r in unrun)
        + ". A tag in one of them would be an absence of evidence wearing evidence's "
        "clothes, so the scanner denies it and names the fix "
        "(`ai/rules/fail-closed-guards.md`)."
    )
    out.append("")
    return out


def _render_status_backlog(
    enrolled: Set[str],
    rows: Dict[str, Dict[str, str]],
    dispositions: Dict[str, Disposition],
) -> List[str]:
    """The two backlogs the status ratchets grandfather, rendered rather than listed.

    `check_status_completeness` lets 32 rowless enrolments through and
    `check_summary_disposition` lets a declared summary through, so both would otherwise be
    invisible until someone re-ran the census by hand. Deriving them here keeps them
    countable in review without a second hand-maintained list, which is the shape
    `ai/rules/derive-not-hardcode.md` requires and the shape `unconverted_summaries` already
    uses. Sorted, because `check_ledger_fresh` compares bytes.
    """
    out: List[str] = []
    missing = sorted(enrolled - set(rows))
    out.append("## Enrolled without a public status row")
    out.append("")
    out.append(
        f"{len(missing)} enrolled RFC(s) have no row in `docs/features/rfc-status.md`. "
        "Every MUST-level requirement they declare is gated, so the obligation is enforced "
        "while the public page says nothing about the RFC at all. `check_status_completeness` "
        "grandfathers the ones that predate it and refuses any NEW enrolment without a row, "
        "so this number can only shrink. It is derived from `enrolled - rows`, never listed."
    )
    out.append("")
    if missing:
        out.append("| RFC | Gated requirements enforced with no public row |")
        out.append("|---|---|")
        for stem in missing:
            out.append(f"| `{stem}` | yes |")
    else:
        out.append("None: every enrolled RFC has a row.")
    out.append("")

    out.append("## Declared not enrolled")
    out.append("")
    out.append(
        "`rfc/not-enrolled.txt` records why each un-enrolled summary is not enrolled, so the "
        "remainder is a decision rather than an absence. Only `non-normative` is a claim "
        "about the document; `backlog` and `blocked` are **DEBT** and are listed as such. A "
        "disposition is discharged by enrolment and by nothing else."
    )
    out.append("")
    if dispositions:
        out.append("| Summary | Kind | Debt? | Reason |")
        out.append("|---|---|---|---|")
        for stem in sorted(dispositions):
            disp = dispositions[stem]
            debt = "no" if disp.kind == DISPOSITION_NON_NORMATIVE else "**DEBT**"
            out.append(f"| `{stem}` | {disp.kind} | {debt} | {disp.reason} |")
    else:
        out.append("None: every summary is enrolled.")
    out.append("")
    return out


def render_ledger(
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    rows: Optional[Dict[str, Dict[str, str]]] = None,
    dispositions: Optional[Dict[str, Disposition]] = None,
) -> str:
    """The generated `ai/RFC-REQUIREMENTS.md` body.

    `rows` and `dispositions` are passed in by `run_check`, which has already read both, so
    one run parses `docs/features/rfc-status.md` exactly once and every consumer shares that
    parse. They default to reading from disk for `run_write` and `run_check_fresh`, whose
    whole job is one render: the bytes are identical either way because both paths read the
    same two files, which is what keeps `check_ledger_fresh`'s byte comparison meaningful.
    """
    if rows is None:
        with open(STATUS_FILE, encoding="utf-8") as fh:
            rows = parse_status_ledger(fh.read())
    if dispositions is None:
        dispositions = load_dispositions()

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
    out.extend(_render_evidence_legend())
    out.extend(_render_rollup(by_rfc, by_rid, enrolled))
    out.extend(_render_audit_coverage(requirements, tags, enrolled))
    out.extend(render_extraction_table(requirements, enrolled))

    # The per-row audit marker, so a reader scanning ONE requirement sees the verdict without
    # reconstructing it from the coverage section. Same shape as the nightly-only marker below.
    verdicts_by_rid: Dict[str, str] = {}
    audits = load_audits(enrolled)
    states = audit_freshness(requirements, tags, enrolled, audits)
    for r in requirements:
        v = audits.get(r.rfc, {}).get(r.rid)
        if not v:
            continue
        state = states.get(r.rid, (FRESH, []))[0]
        verdicts_by_rid[r.rid] = (
            v["verdict"] if state == FRESH else f"{v['verdict']}, {state}"
        )

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
                f"`{t.file}:{t.line}` ({evidence_label(t.file)})"
                for t in found
                if t.polarity == "positive"
            )
            neg = ", ".join(
                f"`{t.file}:{t.line}` ({evidence_label(t.file)})"
                for t in found
                if t.polarity == "negative"
            )
            marks: List[str] = []
            # AC-11: the marker is on the ROW, so a reader scanning one requirement sees
            # the weakness without reconstructing it from the per-link tiers.
            if is_nightly_only(found):
                marks.append("**nightly-only**")
            audited = verdicts_by_rid.get(r.rid)
            if audited and audited != VERDICT_ENFORCED:
                # An `enforced` verdict is unmarked on purpose: it says the row's tests do what
                # the row already claims. Every OTHER value contradicts the row, and a
                # contradiction has to be visible where the claim is made (AC-24).
                marks.append(f"**audit: {audited}**")
            if r.annotation:
                marks.append(f"{{{r.annotation.kind}}} {r.annotation.reason}")
            note = " ".join(marks)
            out.append(
                f"| `{r.rid}` | {r.level} | {r.section} | {pos or '--'} | "
                f"{neg or '--'} | {note} |"
            )
        out.append("")

    out.extend(_render_status_backlog(enrolled, rows, dispositions))

    # GATED, not "any requirement". The caller used to pass every parsed requirement at any
    # level, so a summary with one SHOULD row counted as captured and never appeared below --
    # seven advisory-only summaries hid behind that (D5). The table's own purpose is to name
    # summaries that captured no OBLIGATION.
    stale = unconverted_summaries({r.rfc for r in requirements if r.gated})
    if stale:
        out.append("## Summaries declaring no MUST-level requirement")
        out.append("")
        out.append(
            "These summaries gate nothing. That is correct for a genuinely non-normative "
            "reference, and a capture failure for anything else. The uppercase column counts "
            "MUST/MUST NOT/SHALL/SHALL NOT in the RFC's own text; the lowercase column counts "
            "the same four words in indicative prose, which is the only form a pre-RFC-2119 "
            "document has. A non-zero uppercase count with nothing captured means the summary "
            "needs re-authoring (`/ze-rfc <stem>`) before its RFC can ever be enrolled. A "
            "summary appears here whenever it captured no MUST-level row, even if it captured "
            "SHOULD or MAY rows."
        )
        out.append("")
        out.append("| Summary | Uppercase | Lowercase | Verdict |")
        out.append("|---|---|---|---|")
        for row in stale:
            src = row["src"]
            prose = row["prose"]
            prosetxt = "?" if prose is None else str(prose)
            if src is None:
                verdict = "no source text under `rfc/full/` -- cannot judge"
                srctxt = "?"
            elif src == 0 and prose:
                # The pre-2119 case, and the one the old verdict got wrong. RFC 1035 (1987)
                # shows 0 uppercase and 23 lowercase `must`, and "consistent: source declares
                # none" read a normative wire specification as non-normative (R-7).
                verdict = (
                    f"**UNDECIDED**: 0 uppercase keywords but {prose} lowercase -- a "
                    "pre-RFC-2119 document states obligations in indicative prose, so a "
                    "zero uppercase count is not evidence of a non-normative source. "
                    "Needs a human reading, not a keyword count"
                )
                srctxt = "0"
            elif src == 0:
                verdict = (
                    "consistent: source declares none in either register (0 uppercase, "
                    "0 lowercase)"
                )
                srctxt = "0"
            else:
                verdict = "**RE-AUTHOR**: source is normative, summary captured nothing"
                srctxt = str(src)
            out.append(f"| `{row['stem']}` | {srctxt} | {prosetxt} | {verdict} |")
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
#
# Named rather than spelled at each site: check_unproven_support reads the strongest grade by
# name to refuse OR-A's escape over a source that quotes capitalised keywords, and a second
# spelling of "rfc2119" is a second place for that condition to drift from the derivation that
# produces it (ai/rules/derive-not-hardcode.md).
REGISTER_RFC2119 = "rfc2119"
REGISTER_PROSE = "prose"
REGISTER_MANUAL_WALK = "manual-walk"
REGISTERS = (REGISTER_RFC2119, REGISTER_PROSE, REGISTER_MANUAL_WALK)
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
        return REGISTER_RFC2119
    if prose_sites:
        return REGISTER_PROSE
    return REGISTER_MANUAL_WALK


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
    if register == REGISTER_RFC2119:
        sites = keyword
    else:
        prose = _sites_for(stripped, _SITE_PROSE_RE)
        register = derive_register(len(keyword), len(prose), gated)
        sites = prose if register == REGISTER_PROSE else []

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


def derived_registers(
    signed: Dict[str, "Extraction"], requirements: Sequence[Requirement]
) -> Dict[str, str]:
    """{stem: the register the SOURCE derives} for each stem holding a valid sign-off.

    check_unproven_support needs one fact the artifact cannot supply about itself: what register
    the document is actually written in. `manual-walk` is the WEAKEST grade and
    _evaluate_extraction refuses only a claim STRONGER than derived, so every stem in the corpus
    may declare `manual-walk` -- which made the register-reason escape reachable by any RFC,
    including ones whose own sentences quote capitalised MUST.

    Scoped to `signed`, and free. evaluate_extractions already derived the inventory of exactly
    these stems at exactly this gated count, and derive_inventory is memoised on
    (stem, gated, source sha, path), so this re-reads answers rather than recomputing them.

    A stem with no source text is ABSENT from the result, never defaulted. None is not a
    register: "I could not look" and "the source states nothing" must not render alike
    (ai/rules/fail-closed-guards.md, the zero-value trap), and the consumer refuses the escape
    on an absent grade. Every VALID sign-off has derivable source in practice --
    evaluate_extractions drops one that does not -- so the absent case is a degraded tree, which
    is precisely when a permissive default would be wrong.
    """
    gated = gated_counts(requirements)
    out: Dict[str, str] = {}
    for stem in signed:
        inv = derive_inventory(stem, gated.get(stem, 0))
        if inv is not None:
            out[stem] = inv.register
    return out


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
        # `verdict_freshness` records: a false stale costs a re-read, a false fresh
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
    requirements: Sequence[Requirement],
    tags: Sequence[Tag],
    enrolled: Set[str],
    rows: Optional[Dict[str, Dict[str, str]]] = None,
    dispositions: Optional[Dict[str, Disposition]] = None,
) -> List[str]:
    """The generated ledger must match what its sources render to right now.

    `ai/RFC-REQUIREMENTS.md` is derived from the summaries and the `RFC requirement:`
    tags; a test can be re-tagged, moved, or deleted without touching the ledger, and
    then the committed ledger lies about which tests enforce which requirement. This is
    the same staleness `docs_to_code.py --check` guards for `ai/DOCS-TO-CODE.md`
    (`ai/rules/derive-not-hardcode.md`). It runs inside `ze-rfc-check`, which is in both
    verify branches, so a stale ledger fails the build rather than rotting silently.

    `rows` and `dispositions` are forwarded to `render_ledger` so `run_check`'s single parse
    of `docs/features/rfc-status.md` reaches this call too. Absent, the render reads them
    itself; the bytes are the same, so a caller that omits them pays one extra parse and
    never gets a different verdict.
    """
    body = render_ledger(requirements, tags, enrolled, rows, dispositions) + "\n"
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

    Returns EVERY parse error, and separately the per-stem map of the same failures.
    check_new_summaries needs the map keyed by stem so it can name a new summary that does
    not parse.

    Enrolment no longer filters what is reported. It used to: the comment said the id
    migration was per-RFC and un-enrolled summaries had not been converted, so their parse
    failures were expected. The migration is complete -- zero of the 175 summaries fail to
    parse -- so the filter suppressed nothing and only shielded a FUTURE un-enrolled summary
    from a parse error it should report (plan/spec-rfcgate-4-ledger.md D4,
    ai/rules/stale-comments.md, ai/rules/fail-closed-guards.md). A summary the gate cannot
    read is a summary whose obligations nobody can see, enrolled or not.
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

        # The public page, read ONCE per run and shared by every consumer below: the
        # {gap}-disclosure check, the audit-disclosure check, the completeness ratchet, the
        # unproven-support guard, the gap-count cross-check and the ledger render. It used to
        # be opened late, immediately before its single consumer; five more consumers would
        # have meant five more parses of the same 157 rows.
        with open(STATUS_FILE, encoding="utf-8") as fh:
            rows = parse_status_ledger(fh.read())
        baseline_rows = _git_baseline_status_rows()
        # Strictly parsed: a malformed line reaches the handler below as a clean exit 2, and
        # must never be skipped into a silently undeclared summary.
        dispositions = load_dispositions()
        baseline_dispositions = _git_baseline_dispositions()

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
            # And one level further down: proof can hold its polarity while quietly
            # dropping from a running functional test to a unit table test, or from a
            # verify-tier binding to a nightly-tier one. Both are downgrades the polarity
            # comparison above cannot see.
            errs.extend(
                check_evidence_ratchet(
                    reqs,
                    tags,
                    enrolled,
                    _git_baseline_evidence(),
                    base_enrolled,
                )
            )
        # Every parse error, enrolled or not. _collect_for_check's docstring records why the
        # enrolment filter that used to sit here is gone.
        errs.extend(parse_errs)

        # IDs are allocated once and never reused. Compare the current id set against the
        # committed baseline so "delete 5.3-4, add a different 5.3-4" is caught even though
        # the two are textually indistinguishable (AC-2). Reuse is a bug regardless of
        # enrolment, so this runs over every parsed requirement.
        errs.extend(check_id_allocation(reqs, baseline_ids))

        errs.extend(evaluate(reqs, tags, enrolled))

        errs.extend(check_status_agreement(reqs, rows, enrolled))

        # The public ledger's edges (plan/spec-rfcgate-4-ledger.md). Four guards over the
        # same four inputs check_status_agreement already reads, each closing a case it
        # cannot reach: a summary that is neither enrolled nor declared, an enrolled RFC with
        # no public row, a support claim over an empty checklist, and a hand-written gap
        # count that disagrees with the summary.
        errs.extend(
            check_summary_disposition(
                stems, enrolled, dispositions, baseline_dispositions
            )
        )
        errs.extend(
            check_status_completeness(
                enrolled, rows, baseline_rows, newly_enrolled, base_enrolled
            )
        )
        # The derived grade of every signed stem, read off the SOURCE text. It is the one fact
        # behind OR-A's escape that the artifact cannot assert about itself, and the memoised
        # derivation means the walks were already paid for by signed_extractions above.
        errs.extend(
            check_unproven_support(
                reqs,
                rows,
                stems,
                dispositions,
                signed,
                derived_registers(signed, reqs),
            )
        )
        errs.extend(check_gap_count_agreement(reqs, rows))

        # The audit half. Loaded ONCE through the validating parse and shared by every check
        # below, so the schema cannot be reached by one consumer and bypassed by another
        # (plan/spec-rfcgate-3-audit-teeth.md). A malformed record raises ParseError and lands
        # on the handler below as a clean exit 2, never a traceback.
        audits = load_audits(enrolled)
        baseline_audits = _git_baseline_audits()
        errs.extend(check_audit_files(enrolled, stems))
        errs.extend(check_audit_schema(reqs, tags, enrolled, audits))
        errs.extend(check_audit_freshness(reqs, tags, enrolled, audits))
        errs.extend(check_audit_disclosure(reqs, rows, enrolled, audits))
        errs.extend(check_audit_note(reqs, tags, enrolled, audits))
        errs.extend(check_audit_findings(reqs, enrolled, audits, baseline_audits))
        errs.extend(
            check_audit_verdict_ratchet(
                reqs, enrolled, audits, baseline_audits, base_enrolled
            )
        )

        # Extraction sign-off: bounds what the summary MISSED, which every check above is
        # blind to -- they all judge the requirements a summary LISTS
        # (plan/spec-rfcgate-1-extraction.md). Inside the same try, so a malformed artifact
        # or an unparseable budget exits 2 through the handler below rather than as a
        # traceback (AC-18).
        errs.extend(check_extraction_signoff(reqs))
        errs.extend(check_extraction_ratchet())
        errs.extend(check_drain_floor(enrolled, signed))

        errs.extend(check_ledger_fresh(reqs, tags, enrolled, rows, dispositions))
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
    # Never a bare non-unit total. Verify-tier and nightly-tier evidence ratchet
    # independently (AC-14), so reporting them summed on one line would hand a reader the
    # exact conflation the separate counters exist to prevent (R-1).
    print(
        f"{GREEN}evidence{RESET}: {_evidence_phrase(tag_kind_counts(tags))} "
        f"(unit evidence proves the algorithm; only a running non-unit test proves the "
        f"daemon or a peer)."
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
    # Same reason the extraction bound is printed: the semantic half's coverage is stated OUT
    # LOUD on every clean run, including when it is small. A gate that reports "OK" while its
    # judgement half covers one RFC in 166 is telling a reader something it has not measured.
    # The WORKLIST is the unproven count, never a re-derived subset. This line summed
    # `r.findings` and threw the worklist away, and when `findings` counted only the
    # both-polarity subset the two disagreed: it printed "0 audited-but-not-proven, of 44
    # verdict(s)" over a record holding 52 verdicts, two `unimplemented` gaps and one
    # `not-applicable` among them. The ledger reconciled that in prose; the one line an
    # operator reads did not, which is the reporting surface AC-24 exists to make honest.
    audit_rows, audit_worklist = audit_coverage(reqs, tags, enrolled)
    audit_total = sum(r.auditable for r in audit_rows)
    audit_done = sum(r.audited for r in audit_rows)
    audit_verdicts = sum(r.verdicts for r in audit_rows)
    print(
        f"{GREEN}audit{RESET}: {sum(r.proven for r in audit_rows)} proven, "
        f"{len(audit_worklist)} audited-but-not-proven, of {audit_verdicts} "
        f"verdict(s); {audit_done} of {audit_total} auditable requirement(s) audited "
        f"({(100.0 * audit_done / audit_total) if audit_total else 0.0:.2f}%); a missing "
        f"verdict is legal (the audit is sampled, the gate is total)."
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
    if "--reseal" in args:
        return run_reseal()
    if "--check" in args:
        return run_check()
    print(__doc__)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv))
