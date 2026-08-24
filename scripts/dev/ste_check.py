#!/usr/bin/env python3
"""Review the repository's writing against ASD-STE100 Simplified Technical English.

`ai/rules/writing.md` is rule one of this repository: every
word Ze writes is Simplified Technical English (ASD-STE100, Issue 9, 2025-01-15).
The goal is simpler English that a reader with basic English understands the first
time. That rule names six habits to avoid. This tool finds them, so the rule is
reviewable by a machine instead of by memory.

The six habits, and the STE rule behind each one:

    1 synonym-rotation     one concept with several names   Rules 1.3, 1.11, 9.4
    2 hedging              "may", "should", "generally"     Rule 1.1 + dictionary
    3 frozen-verbs         "do the installation of"         Rule 3.7
    4 marketing-adjectives "powerful", "seamless"           Rule 1.1
    5 run-ons              long sentences, semicolons       Rules 4.1, 5.1, 6.3, 6.6, 8.1
    6 phrasal-verbs        "spin up", "figure out"          Rule 9.3

Surfaces
--------
Every writing surface INSIDE the repository:

    markdown             docs/, ai/, the durable half of plan/, and the root *.md
                         files. A document deleted at closure is out: see
                         EXCLUDE_GLOBS
    go-comments          prose comments in *.go, minus the structured markers
    yang-descriptions    `description` strings in *.yang
    stdin                a commit message or a PR body, piped in as "-"

The website and the wiki are outside this repository and outside this tool. Their
copy is marketing, is written in Thomas's voice, and keeps UK English.

What this tool is NOT
---------------------
It is not an STE conformance checker. Full conformance needs part 2 of the
standard, the controlled dictionary of approximately 900 approved words. That
dictionary is copyright (c) ASD 2025 and Ze holds no reproduction right, so it
cannot be embedded here (see the rule's "Getting the standard" section). Every
word list below is OURS: written from the six habits, tuned for Ze's vocabulary,
and deliberately narrow. A narrow list that reviewers trust beats a wide list
that they learn to ignore.

Precision over recall
---------------------
Each list holds only patterns that are wrong in Ze prose in every context we
could construct. Some real STE violations therefore pass. Widen a list when a
review finds a habit the tool missed, and add the case to ste_check_test.py.

Two facts from the dictionary that the lists encode:
  * "usually" is APPROVED. `generally` and `normally` both resolve to USUALLY,
    so "usually" is a fix, never a finding.
  * `may` resolves to CAN and `should` resolves to MUST. Uppercase MUST, SHOULD,
    and MAY are RFC 2119 keywords, are never findings, and are never rewritten.

Legacy prose is handled by a ratchet, not by a flag day. `--check` compares each
file the working tree changed against its OWN HEAD version, and fails when a
habit grew in that file. See the comment above `ratchet()` for why HEAD is the
baseline instead of a committed count file.

Usage:
    python3 scripts/dev/ste_check.py docs/guide/quickstart.md   # review one file
    python3 scripts/dev/ste_check.py                            # the whole repo
    python3 scripts/dev/ste_check.py --changed                  # changed files only
    git log -1 --format=%B | python3 scripts/dev/ste_check.py - # a commit or PR body
    python3 scripts/dev/ste_check.py --check                    # the gate (exit 3 = a habit grew)
    python3 scripts/dev/ste_check.py --json                     # machine-readable
"""

import argparse
import json
import re
import subprocess
import sys
from fnmatch import fnmatch
from pathlib import Path

# --------------------------------------------------------------------------
# What gets reviewed
# --------------------------------------------------------------------------

SURFACE_MARKDOWN = "markdown"
SURFACE_GO = "go-comments"
SURFACE_YANG = "yang-descriptions"
SURFACE_STDIN = "stdin"

SUFFIX_SURFACE = {
    ".md": SURFACE_MARKDOWN,
    ".go": SURFACE_GO,
    ".yang": SURFACE_YANG,
}

DEFAULT_GLOBS = (
    "*.md",
    "docs/**/*.md",
    "ai/**/*.md",
    "plan/**/*.md",
    "internal/**/*.go",
    "cmd/**/*.go",
    "pkg/**/*.go",
    "internal/**/*.yang",
)

# rfc/ holds external normative text that must stay verbatim. The rest is either
# vendored, generated into, or scratch.
#
# ai/rules/points/ is the source `ai/rules/<rule>.md` is rendered from, and every
# point body is a VERBATIM fragment of that rendered file. Reviewing both counts
# each sentence twice: 951 of the 2417 lines the ratchet printed the first time
# the points existed were the same findings the rendered rule already reports.
# The rendered rule stays in scope, so nothing here goes unreviewed.
EXCLUDE_DIRS = (
    "rfc/",
    "tmp/",
    "vendor/",
    "third_party/",
    "gokrazy/",
    ".git/",
    "backups/",
    "ai/rules/points/",
    # A deferral shard and a known-failure shard are deleted when their rows are
    # resolved, like the spec below. See EXCLUDE_GLOBS.
    "plan/deferrals/",
    "plan/known-failures/",
)

# A document that is DELETED when the work closes is not worth an STE edit
# (owner directive, 2026-08-10). A spec lives for the length of one piece of
# work and `git rm`s itself in commit B (`ai/rules/planning.md`, "Spec
# Closure"), so a sentence rewritten there is read once by the session that
# wrote it and then removed. The durable records under `plan/` stay in scope:
# `plan/journal/`, `plan/learned/` and `plan/TEMPLATE.md` outlive every spec
# and are read by sessions that were not there.
EXCLUDE_GLOBS = ("plan/spec-*.md",)

# Generated files carry their producer's prose, so a finding there belongs to the
# generator. Detected by marker, so a new generated document needs no wiring.
#
# "DO NOT EDIT" is deliberately NOT a marker. ai/INSTRUCTIONS.md opens with the
# banner it writes INTO its generated outputs, so that string skipped the one
# document every session reads.
GENERATED_MARKERS = ("GENERATED by", "<!-- generated", "Code generated")

# A document that must quote non-STE text at length opts out with this marker on
# its own line. The reason is mandatory: an opt-out with no reason is a silent
# exemption, and those accumulate.
#
# Anchored to the start of a line, because a document that DOCUMENTS the escape
# hatch must not exempt itself. This rule file names the marker in its own
# Enforcement section, and an unanchored pattern switched the rule file off.
IGNORE_FILE = re.compile(
    r"^\s*(?:<!--|//|#)\s*ste:\s*ignore-file\s+(?P<reason>.+?)\s*(?:-->|$)",
    re.MULTILINE,
)
IGNORE_LINE = re.compile(r"^\s*(?:<!--|//|#)\s*ste:\s*ignore\s*(?:-->|$)")

# --------------------------------------------------------------------------
# Habit 1 -- synonym rotation (STE Rules 1.3, 1.11, 9.4)
# --------------------------------------------------------------------------

# (canonical, rotations). One concept, one name. Every rotation here is a name
# Ze does not use, so the fix is mechanical. Pairs that look like synonyms but
# name DIFFERENT things are deliberately absent: `delete` (config), `clear`
# (counters), and `remove` (route) are three Ze operations, not three words for
# one operation, and flagging them would teach reviewers to ignore this habit.
# The plain word for an action, and the formal words that mean the same. This is
# the standard's own core discipline: when English gives several words for one
# action, keep the shortest common one. Every pair here was read out of the
# controlled dictionary, then filtered for Ze: a domain verb that no plainer word
# replaces is deliberately absent, so `implement`, `negotiate`, `withdraw`, and
# `advertise` are never flagged. Reported under habit 1, because one concept
# keeping one name is the same rule seen from the other side.
PLAIN_WORDS = {
    "initiate": "start",
    "initiates": "starts",
    "commence": "start",
    "commences": "starts",
    "terminate": "stop",
    "terminates": "stops",
    "cease": "stop",
    "ceases": "stops",
    "obtain": "get",
    "obtains": "gets",
    "acquire": "get",
    "acquires": "gets",
    "utilize": "use",
    "utilizes": "uses",
    "utilise": "use",
    "assist": "help",
    "assists": "helps",
    "facilitate": "help",
    "facilitates": "helps",
    "ascertain": "make sure",
    "eliminate": "remove",
    "eliminates": "removes",
    "additional": "more",
    "acceptable": "permitted",
    "endeavor": "try",
    "endeavour": "try",
    "prior to": "before",
    "subsequent to": "after",
    "in order to": "to",
}

# A plain word that is ALSO a protocol identifier. `Cease` is the BGP
# NOTIFICATION error code of RFC 4271 Section 4.5, so `the Cease/MaxPrefixes
# Data field` names a wire field rather than the verb. The capital letter is the
# tell, and a sentence-initial capital is not, so the guard reads what comes
# before the word. This is the same reasoning as the RFC 2119 guard in
# check_hedging: a checker that flags a protocol name teaches its readers to
# ignore it.
PLAIN_WORD_IDENTIFIERS = {"Cease"}

# The end of the preceding sentence, or the start of the unit.
SENTENCE_BOUNDARY_BEFORE = re.compile(r"(?:[.!?:]\s*|^\s*)$")

TERM_SETS = (
    ("peer", ("neighbour", "neighbor")),
    ("plugin", ("plug-in",)),
    ("hostname", ("host name",)),
    ("filename", ("file name",)),
    ("dataplane", ("data plane", "data-plane")),
    ("runtime", ("run-time",)),
    ("startup", ("start-up",)),
    ("nftables", ("nft tables",)),
)

# --------------------------------------------------------------------------
# Habit 2 -- hedging (STE Rule 1.1 and the dictionary)
# --------------------------------------------------------------------------

HEDGES = {
    "may": "CAN for a possibility, MUST for an obligation",
    "might": "CAN, or state the condition",
    "could": "CAN, or state the condition",
    "should": "MUST",
    "probably": "state the fact, or say you do not know",
    "possibly": "state the fact, or say you do not know",
    "perhaps": "state the fact, or say you do not know",
    "arguably": "state the fact, or say you do not know",
    "seemingly": "state what the code does",
    "apparently": "state what the code does",
    "presumably": "state what the code does",
    "generally": "usually (the approved word)",
    "normally": "usually (the approved word)",
    "typically": "usually (the approved word)",
    "essentially": "delete it, then state the fact",
    "basically": "delete it, then state the fact",
    "hopefully": "delete it, then state the fact",
    "ideally": "state the requirement",
    "simply": "delete it",
    "easily": "delete it, or give the number",
    "somewhat": "give the number",
    "relatively": "give the number",
    "fairly": "give the number",
}

HEDGE_PHRASES = {
    "in most cases": "usually",
    "in general": "usually",
    "more or less": "give the number",
    "it seems": "state what the code does",
    "seems to": "state what the code does",
    "appears to": "state what the code does",
    "we believe": "state the fact, or cite the source",
    "we think": "state the fact, or cite the source",
    "tend to": "usually",
    "tends to": "usually",
    "kind of": "delete it",
    "sort of": "delete it",
    "pretty much": "delete it",
}

# --------------------------------------------------------------------------
# Habit 3 -- frozen verbs (STE Rule 3.7)
# --------------------------------------------------------------------------

# noun -> the verb to use instead. Only nouns whose verb is unambiguous.
NOMINALIZATIONS = {
    "installation": "install",
    "validation": "validate",
    "verification": "verify",
    "configuration": "configure",
    "initialization": "initialize",
    "registration": "register",
    "allocation": "allocate",
    "negotiation": "negotiate",
    "activation": "activate",
    "deactivation": "deactivate",
    "cancellation": "cancel",
    "deletion": "delete",
    "creation": "create",
    "execution": "execute",
    "modification": "modify",
    "transmission": "transmit",
    "distribution": "distribute",
    "generation": "generate",
    "calculation": "calculate",
    "notification": "notify",
    "implementation": "implement",
    "replacement": "replace",
    "movement": "move",
    "measurement": "measure",
    "removal": "remove",
    "comparison": "compare",
    "examination": "examine",
    "inspection": "inspect",
    "selection": "select",
    "computation": "compute",
    "evaluation": "evaluate",
    "migration": "migrate",
    "conversion": "convert",
    "insertion": "insert",
    "extraction": "extract",
    "resolution": "resolve",
    "submission": "submit",
}

# "Do a check of the battery" is STE, because "check" is not an approved verb
# (STE Rule 3.7 gives that exact example). So the trigger is never the light verb
# alone: it is a light verb, or a preposition, bound to a nominalization whose
# verb IS available.
_NOUNS = "|".join(sorted(NOMINALIZATIONS))
LIGHT_VERB = re.compile(
    r"\b(do|does|did|doing|perform|performs|performed|performing|make|makes|made"
    r"|making|conduct|conducts|conducted|carry out|carries out|carried out)\s+"
    r"(?:a|an|the|any|its|their)?\s*(" + _NOUNS + r")\b",
    re.IGNORECASE,
)
FROZEN_OF = re.compile(
    r"\b(before|after|during|upon)\s+the\s+(" + _NOUNS + r")\s+of\b",
    re.IGNORECASE,
)

# --------------------------------------------------------------------------
# Habit 4 -- marketing adjectives (STE Rule 1.1)
# --------------------------------------------------------------------------

MARKETING = (
    "powerful",
    "seamless",
    "seamlessly",
    "blazing",
    "blazingly",
    "lightning-fast",
    "effortless",
    "effortlessly",
    "feature-rich",
    "cutting-edge",
    "state-of-the-art",
    "world-class",
    "best-in-class",
    "battle-tested",
    "enterprise-grade",
    "industrial-strength",
    "bulletproof",
    "rock-solid",
    "turnkey",
    "next-generation",
    "revolutionary",
    "game-changing",
    "unparalleled",
    "unmatched",
    "amazing",
    "incredible",
    "stunning",
    "insanely",
    "super-fast",
    "ultra-fast",
    "buttery",
    "silky",
)

# --------------------------------------------------------------------------
# Habit 5 -- run-ons (STE Rules 4.1, 5.1, 6.3, 6.6, 8.1)
# --------------------------------------------------------------------------

MAX_PROCEDURAL = 20  # Rule 5.1: a numbered step is a procedure
MAX_DESCRIPTIVE = 25  # Rule 6.3
MAX_SENTENCES_PER_PARAGRAPH = 6  # Rule 6.6

# --------------------------------------------------------------------------
# Habit 6 -- phrasal verbs (STE Rule 9.3)
# --------------------------------------------------------------------------

# Two-word verbs whose meaning is not the sum of their parts. "set up" is the
# verb and "setup" is the noun, so the spaced form is always the verb.
# `shutdown`, `teardown`, and `backoff` are Ze technical nouns (STE Rules 1.5,
# 1.6) and stay correct as one word. Only the spaced verb forms are findings.
PHRASAL_VERBS = {
    "spin up": "start",
    "spun up": "started",
    "spins up": "starts",
    "kick off": "start",
    "kicks off": "starts",
    "kicked off": "started",
    "figure out": "find, or diagnose",
    "figures out": "finds",
    "figured out": "found",
    "get rid of": "delete, or remove",
    "gets rid of": "deletes",
    "put out": "extinguish",
    "give off": "release",
    "gives off": "releases",
    "come up with": "write, or select",
    "comes up with": "writes",
    "look into": "examine, or diagnose",
    "looks into": "examines",
    "end up": "become, or result in",
    "ends up": "becomes",
    "sort out": "correct, or resolve",
    "work out": "calculate, or resolve",
    "take care of": "do, or manage",
    "takes care of": "does",
    "tear down": "stop, or remove",
    "tears down": "stops",
    "set up": "install, prepare, or configure",
    "sets up": "installs",
}

# Recommendation 6: a Latin abbreviation confuses a reader who does not know it,
# so the English words replace it. Reported under hedging, because both habits
# make the reader supply meaning the writer withheld.
LATIN_ABBREVIATIONS = {
    "e.g.": "for example",
    "i.e.": "that is",
    "etc.": "and so on",
    "viz.": "namely",
    "cf.": "compare",
    "et al.": "and others",
}

# Restricted meanings. An approved word keeps its approved meaning: `above` and
# `below` are physical positions, never limits. Reported under habit 1.
RESTRICTED_MEANINGS = (
    (
        re.compile(r"\babove\s+\d", re.IGNORECASE),
        '"above" for a limit',
        'write "more than"',
    ),
    (
        re.compile(r"\bbelow\s+\d", re.IGNORECASE),
        '"below" for a limit',
        'write "less than"',
    ),
)

# A gerund clause. The `-ing` form is permitted as a technical noun (`routing
# table`) or as a modifier in one (`switching relay`), and nowhere else, so a
# preposition in front of one marks the banned use. Reported under frozen-verbs.
GERUND_CLAUSE = re.compile(
    r"\b(before|after|while|without|when)\s+([a-z]+ing)\b", re.IGNORECASE
)

# Words that end in `-ing` and are not gerunds. The second group of
# GERUND_CLAUSE accepts any lowercase word with that ending, so an indefinite
# pronoun after one of its five prepositions reads as a gerund clause and
# "when nothing is removed" is reported as a frozen verb. `string` is the entry
# that matters most here, because this repository writes about strings and
# "when string parsing fails" is ordinary prose.
NOT_GERUND = frozenset(
    """
    anything everything nothing something
    bring during king ring spring sting string swing thing wing
    ceiling evening morning
    """.split()
)

# A definite article before a noun that carries an alphanumeric identifier. The
# identifier makes the noun proper, so the article is incorrect. Kept to Ze's own
# nouns, because a general `the \w+ \d` pattern would flag "the first 3 bytes".
ARTICLE_BEFORE_ID = re.compile(
    r"\bthe\s+(RFC|AS|ASN|table|port|peer|prefix|section|chapter|figure|step|rule)\s+\d",
    re.IGNORECASE,
)

# argparse exits 2 on a usage error, so the gate needs a code of its own. A
# caller that treated 2 as "habit grew" would report argparse's usage text as a
# prose finding and block the commit.
EXIT_HABIT_GREW = 3

HABITS = {
    1: "synonym-rotation",
    2: "hedging",
    3: "frozen-verbs",
    4: "marketing-adjectives",
    5: "run-ons",
    6: "phrasal-verbs",
}
SLUGS = {name: number for number, name in HABITS.items()}
SURFACES = (SURFACE_MARKDOWN, SURFACE_GO, SURFACE_YANG, SURFACE_STDIN)


class Finding:
    """One habit found at one place."""

    def __init__(self, path, line, surface, habit, detail, fix, excerpt):
        self.path = str(path)
        self.line = line
        self.surface = surface
        self.habit = habit
        self.detail = detail
        self.fix = fix
        self.excerpt = excerpt

    def __str__(self):
        return f"{self.path}:{self.line}: [{self.habit}] {self.detail} -> {self.fix}"

    def as_dict(self):
        return {
            "file": self.path,
            "line": self.line,
            "surface": self.surface,
            "habit": self.habit,
            "habit-number": SLUGS[self.habit],
            "detail": self.detail,
            "fix": self.fix,
            "excerpt": self.excerpt,
        }


class Unit:
    """A run of prose, with the line where it starts.

    A paragraph, one list item, one table cell, one comment block, or one YANG
    description. Table cells are separate units because a wide row is a table,
    not a run-on sentence.
    """

    def __init__(self, text, line, procedural=False, paragraph=True):
        self.text = text
        self.line = line
        self.procedural = procedural
        # STE Rule 6.6 caps the SENTENCES in a paragraph. A table cell, a
        # heading, and a list item are not paragraphs: a reference-table cell
        # holding eight short facts is a table, and capping it would push
        # authors to write fewer, longer sentences, which is the opposite of
        # what this rule wants.
        self.paragraph = paragraph


# --------------------------------------------------------------------------
# Extractor: Markdown
# --------------------------------------------------------------------------

CODE_SPAN = re.compile(r"`[^`]*`")
LINK_TARGET = re.compile(r"\]\([^)]*\)")
AUTOLINK = re.compile(r"<https?://[^>]*>|https?://\S+")
HTML_COMMENT = re.compile(r"<!--.*?-->", re.DOTALL)
HTML_ENTITY = re.compile(r"&(?:[a-zA-Z]+|#\d+);")

HEADING = re.compile(r"^\s{0,3}#{1,6}\s")
FENCE = re.compile(r"^\s*(`{3,}|~{3,})")
TABLE_DIVIDER = re.compile(r"^\s*\|?[\s:|-]+\|[\s:|-]*$")
ORDERED_ITEM = re.compile(r"^\s*\d+[.)]\s+")
BULLET_ITEM = re.compile(r"^\s*[-*+]\s+")
# A bolded field label starts its own unit. Without this, the three lines of a
# rule's **When:** / **Severity:** / **Related:** block join into one 28-word
# "sentence" that has no verb and cannot be split.
BOLD_FIELD = re.compile(r"^\s*\*\*[^*]+:\*\*")
# A run of two or more spaces INSIDE a line is column alignment, and a column is
# not a wrapped sentence. Markdown joins consecutive lines into one paragraph, so
# a commit list, an ASCII table, or a two-column note reads as a single sentence
# that no rewrite can shorten: a 15-line commit list in a handover measured 187
# words. Prose that puts two spaces after a period is NOT this, so the run must
# not follow sentence punctuation. Each aligned line still becomes its own unit,
# so every word stays in the population and only the false join is removed.
LAYOUT_COLUMNS = re.compile(r"[^\s.!?:,;]\s{2,}\S")


def is_layout_row(line):
    """True when the line is column-aligned layout rather than wrapped prose.

    The probe removes a code span, a link, and a comment WITHOUT leaving a
    space, because `scrub` pads its placeholders and that padding reads as a
    column. One vendored README aligns three numbers inside a code span,
    `(-2105.254  300.680  286.185)`, and the prose around it is an ordinary
    wrapped paragraph.
    """
    probe = CODE_SPAN.sub("CODE", line)
    probe = AUTOLINK.sub("URL", probe)
    probe = HTML_COMMENT.sub("", probe)
    return bool(LAYOUT_COLUMNS.search(probe.strip()))


def scrub(text):
    """Remove everything that is data rather than prose."""
    text = CODE_SPAN.sub(" CODE ", text)
    text = HTML_ENTITY.sub(" ", text)  # `&nbsp;` is not a semicolon
    text = AUTOLINK.sub(" URL ", text)
    text = LINK_TARGET.sub("] ", text)  # keep the link label, drop the target
    text = HTML_COMMENT.sub(" ", text)
    return text


def blank_html_comments(text):
    """Blank multi-line HTML comments, preserving line numbers.

    An `ste:` directive survives: it IS an HTML comment, and blanking it here
    erased the very marker that units_markdown reads.
    """

    def blank(match):
        body = match.group(0)
        if "ste:" in body:
            return body
        return "\n" * body.count("\n")

    return HTML_COMMENT.sub(blank, text)


def units_markdown(lines):
    """Split Markdown into reviewable units.

    Skipped: fenced blocks (data), blockquotes (external quotation, kept verbatim
    per the rule), table dividers, and lines after `<!-- ste: ignore -->`.
    """
    out = []
    fence = ""  # the opening delimiter, empty when not inside a block
    paragraph = []
    para_line = 0
    skip_next = False

    def flush():
        nonlocal paragraph, para_line
        if paragraph:
            out.append(Unit(" ".join(paragraph), para_line))
        paragraph = []
        para_line = 0

    for number, raw in enumerate(lines, start=1):
        marker = FENCE.match(raw)
        if marker:
            run = marker.group(1)
            if not fence:
                flush()
                fence = run
                continue
            # Only a run of the SAME character, at least as long, closes it. A
            # ~~~ line inside a ``` block used to close it and expose the code.
            if run[0] == fence[0] and len(run) >= len(fence):
                fence = ""
                continue
        if fence:
            continue
        if IGNORE_LINE.search(raw):
            flush()
            skip_next = True
            continue
        if skip_next:
            skip_next = False
            continue

        line = raw.rstrip("\n")
        if not line.strip():
            flush()
            continue
        if line.lstrip().startswith(">"):  # external quotation
            flush()
            continue
        if TABLE_DIVIDER.match(line):
            flush()
            continue

        text = scrub(line)

        if "|" in line and line.strip().startswith("|"):
            flush()
            for cell in text.split("|"):
                if cell.strip():
                    out.append(Unit(cell.strip(), number, paragraph=False))
            continue
        if HEADING.match(line):
            flush()
            out.append(Unit(HEADING.sub("", text).strip(), number, paragraph=False))
            continue
        if BOLD_FIELD.match(line):
            flush()
            out.append(Unit(text.strip(), number, paragraph=False))
            continue
        if ORDERED_ITEM.match(line):
            flush()
            out.append(
                Unit(ORDERED_ITEM.sub("", text).strip(), number, True, paragraph=False)
            )
            continue
        if BULLET_ITEM.match(line):
            flush()
            out.append(Unit(BULLET_ITEM.sub("", text).strip(), number, paragraph=False))
            continue
        if is_layout_row(line):
            flush()
            out.append(Unit(text.strip(), number, paragraph=False))
            continue

        if not paragraph:
            para_line = number
        paragraph.append(text.strip())

    flush()
    return out


# --------------------------------------------------------------------------
# Extractor: Go comments
# --------------------------------------------------------------------------

# Structured markers are machine-read contracts, not prose. `// Design:`,
# `// Related:` and friends are required by go-standards.md and
# go-standards.md, and their content is paths.
GO_MARKERS = (
    "go:",
    "nolint",
    "Design:",
    "Related:",
    "Detail:",
    "Overview:",
    "RFC:",
    "RFC requirement:",
    "VALIDATES:",
    "PREVENTS:",
    "source:",
    "test-asserts-nothing:",
    "ste:",
    "Code generated",
    "+build",
    "TODO",
    "FIXME",
    "Deprecated:",
)

# A commented-out line of code is code, not prose.
GO_CODE_LIKE = re.compile(
    r":=|==|!=|\{\s*$|\}\s*$|;\s*$|\)\s*\{|^\s*(if|for|func|return)\b"
)


def units_go(lines):
    """Extract prose comment blocks from Go source.

    Consecutive `//` lines join into one unit, so a wrapped sentence is measured
    once rather than per line.
    """
    out = []
    block = []
    block_line = 0
    in_block_comment = False

    def flush():
        nonlocal block, block_line
        if block:
            text = " ".join(block).strip()
            if len(text.split()) > 2:
                out.append(Unit(text, block_line))
        block = []
        block_line = 0

    for number, raw in enumerate(lines, start=1):
        line = raw.strip()

        if in_block_comment:
            if "*/" in line:
                in_block_comment = False
                line = line.split("*/")[0]
            body = line.lstrip("*").strip()
            if body:
                if not block:
                    block_line = number
                block.append(scrub(body))
            else:
                flush()
            continue

        if line.startswith("/*"):
            body = line[2:]
            if "*/" in body:
                body = body.split("*/")[0]
            else:
                in_block_comment = True
            if body.strip():
                block_line = block_line or number
                block.append(scrub(body.strip()))
            continue

        if line.startswith("//"):
            body = line[2:].strip()
            if not body or any(body.startswith(m) for m in GO_MARKERS):
                flush()
                continue
            if GO_CODE_LIKE.search(body):
                flush()
                continue
            if not block:
                block_line = number
            block.append(scrub(body))
            continue

        flush()

    flush()
    return out


# --------------------------------------------------------------------------
# Extractor: YANG descriptions
# --------------------------------------------------------------------------

YANG_DESCRIPTION = re.compile(
    r"\b(?:description|error-message)\s+(?:\"(?P<body>[^\"]*)\"|'(?P<body2>[^']*)')",
    re.DOTALL,
)


def units_yang(text):
    """Extract `description` and `error-message` strings from a YANG module."""
    out = []
    for match in YANG_DESCRIPTION.finditer(text):
        raw = match.group("body")
        if raw is None:
            raw = match.group("body2")
        body = scrub(" ".join(raw.split()))
        if len(body.split()) < 3:
            continue
        line = text.count("\n", 0, match.start()) + 1
        out.append(Unit(body, line))
    return out


# --------------------------------------------------------------------------
# Sentences and STE word count
# --------------------------------------------------------------------------

# Dots that do not end a sentence: abbreviations first, then the dots inside a
# word (4.5, foo.go, Rule 1.1). The abbreviations go first because their own dots
# would otherwise survive and split the sentence.
ABBREVIATIONS = (
    "e.g.",
    "i.e.",
    "etc.",
    "vs.",
    "approx.",
    "Mr.",
    "Ms.",
    "Dr.",
)
# `No.` and `Fig.` abbreviate only in front of the number they label, as in
# "No. 5". Everywhere else the dot is a real full stop, and an unconditional
# hold glues the next sentence onto this one. That inflates a word count and
# reports a run-on that nobody can fix, because the sentence is already two.
#
# This repository decides the shape of the rule. It writes "answered Yes/No."
# and a table cell of "| No. Answer the person who asked" constantly, and it
# numbers almost nothing: 38 occurrences against 1 when this was split out.
# Filed as F-ste-2 in plan/learned/HOOK-FRICTION.md.
NUMBERED_ABBREVIATION = re.compile(r"\b(No|Fig)\.(?=\s*\d)")
INNER_DOT = re.compile(r"(?<=\w)\.(?=\w)")
# Markdown emphasis and closing punctuation sit BETWEEN the full stop and the
# space: "**... dictionary.** It cannot ...". Without them in the pattern, a
# bolded lead-in glues its whole bullet into one 29-word sentence.
SENTENCE_END = re.compile(r"(?<=[.!?])[*_`\"'’”)\]]*\s+")
HELD = "\x00"


def sentences(text):
    """Split a unit into sentences, keeping numbers and file names whole."""
    held = text
    for abbreviation in ABBREVIATIONS:
        held = held.replace(abbreviation, abbreviation.replace(".", HELD))
    held = NUMBERED_ABBREVIATION.sub(lambda m: m.group(1) + HELD, held)
    held = INNER_DOT.sub(HELD, held)
    parts = [p.strip() for p in SENTENCE_END.split(held) if p.strip()]
    return [p.replace(HELD, ".") for p in parts]


PARENTHESES = re.compile(r"\([^)]*\)")
QUOTED = re.compile(r"\"[^\"]*\"|“[^”]*”")
NUMBER_UNIT = re.compile(r"\b\d+(?:\.\d+)?\s+[A-Za-z%]+\b")
WORDLIKE = re.compile(r"[A-Za-z0-9]")


def word_count(sentence):
    """Count words the way STE counts them (Rules 8.5 thru 8.7).

    Parenthesised text, quoted text, a number with its unit, and a hyphenated
    word each count as one word.
    """
    text = PARENTHESES.sub(" PAREN ", sentence)
    text = QUOTED.sub(" QUOTED ", text)
    text = NUMBER_UNIT.sub(" MEASURE ", text)
    return sum(1 for token in text.split() if WORDLIKE.search(token))


# --------------------------------------------------------------------------
# Detectors
# --------------------------------------------------------------------------


def word_re(word):
    """Word-boundary regex for a word or a multi-word phrase."""
    return re.compile(r"(?<![\w-])" + re.escape(word) + r"(?![\w-])")


def any_of(words, flags=0):
    """One regex for a whole word list.

    A regex per word costs about 100 searches for each unit of prose, which is
    100 seconds over this repository. One alternation for each list is 4
    searches, and the matched text says which word was found.
    """
    ordered = sorted(words, key=len, reverse=True)
    body = "|".join(re.escape(w) for w in ordered)
    return re.compile(r"(?<![\w-])(" + body + r")(?![\w-])", flags)


_HEDGE_WORDS_RE = any_of(HEDGES, re.IGNORECASE)
_HEDGE_PHRASE_RE = any_of(HEDGE_PHRASES, re.IGNORECASE)
_MARKETING_RE = any_of(MARKETING, re.IGNORECASE)
_PHRASAL_RE = any_of(PHRASAL_VERBS, re.IGNORECASE)
_PLAIN_RE = any_of(PLAIN_WORDS, re.IGNORECASE)
_TERM_RE = {rot: word_re(rot) for _, rots in TERM_SETS for rot in rots}
_CANON_RE = {canon: word_re(canon) for canon, _ in TERM_SETS}


def add(found, unit, path, surface, habit, detail, fix, excerpt=None):
    found.append(
        Finding(
            path,
            unit.line,
            surface,
            habit,
            detail,
            fix,
            (excerpt if excerpt is not None else unit.text)[:120],
        )
    )


def check_hedging(unit, path, surface, found):
    # Case-insensitive, minus the ALL-CAPS form. MUST, SHOULD, and MAY in capitals
    # are RFC 2119 keywords. "Typically" and "Should" at the head of a sentence are
    # hedges, and a case-sensitive match let every sentence-initial hedge through.
    for match in _HEDGE_WORDS_RE.finditer(unit.text):
        word = match.group(1)
        if word.isupper() and len(word) > 1:
            continue  # RFC 2119 keyword
        add(
            found,
            unit,
            path,
            surface,
            "hedging",
            f'"{word.lower()}"',
            HEDGES[word.lower()],
        )
    for match in _HEDGE_PHRASE_RE.finditer(unit.text):
        phrase = match.group(1).lower()
        add(found, unit, path, surface, "hedging", f'"{phrase}"', HEDGE_PHRASES[phrase])


def check_plain_words(unit, path, surface, found):
    for match in _PLAIN_RE.finditer(unit.text):
        raw = match.group(1)
        if raw in PLAIN_WORD_IDENTIFIERS and not SENTENCE_BOUNDARY_BEFORE.search(
            unit.text[: match.start()]
        ):
            continue  # a protocol identifier, not the verb
        word = " ".join(raw.lower().split())
        add(
            found,
            unit,
            path,
            surface,
            "synonym-rotation",
            f'"{word}"',
            f'use the plain word: "{PLAIN_WORDS[word]}"',
        )


def check_latin(unit, path, surface, found):
    lowered = unit.text.lower()
    for abbreviation, english in LATIN_ABBREVIATIONS.items():
        if abbreviation in lowered:
            add(
                found,
                unit,
                path,
                surface,
                "hedging",
                f'"{abbreviation}"',
                f'write "{english}"',
            )


def check_restricted_meanings(unit, path, surface, found):
    for pattern, detail, fix in RESTRICTED_MEANINGS:
        if pattern.search(unit.text):
            add(found, unit, path, surface, "synonym-rotation", detail, fix)


def check_articles(unit, path, surface, found):
    for match in ARTICLE_BEFORE_ID.finditer(unit.text):
        add(
            found,
            unit,
            path,
            surface,
            "synonym-rotation",
            f'"{" ".join(match.group(0).split())}"',
            'an alphanumeric identifier makes the noun proper: drop "the"',
        )


def check_frozen_verbs(unit, path, surface, found):
    for match in GERUND_CLAUSE.finditer(unit.text):
        verb = match.group(2).lower()
        if verb in NOT_GERUND:
            continue
        add(
            found,
            unit,
            path,
            surface,
            "frozen-verbs",
            f'"{match.group(1).lower()} {verb}"',
            f'name the actor: "{match.group(1).lower()} you <verb>". The -ing form'
            " is permitted only as a technical noun or its modifier",
        )

    for match in LIGHT_VERB.finditer(unit.text):
        verb = NOMINALIZATIONS[match.group(2).lower()]
        add(
            found,
            unit,
            path,
            surface,
            "frozen-verbs",
            f'"{" ".join(match.group(0).split())}"',
            f'use the verb "{verb}"',
        )
    for match in FROZEN_OF.finditer(unit.text):
        verb = NOMINALIZATIONS[match.group(2).lower()]
        add(
            found,
            unit,
            path,
            surface,
            "frozen-verbs",
            f'"{" ".join(match.group(0).split())}"',
            f'use the verb: "{match.group(1)} you {verb} ..."',
        )


def check_marketing(unit, path, surface, found):
    for match in _MARKETING_RE.finditer(unit.text):
        add(
            found,
            unit,
            path,
            surface,
            "marketing-adjectives",
            f'"{match.group(1).lower()}"',
            "give the number, the limit, or the mechanism",
        )


def check_phrasal(unit, path, surface, found):
    for match in _PHRASAL_RE.finditer(unit.text):
        phrase = " ".join(match.group(1).lower().split())
        add(
            found,
            unit,
            path,
            surface,
            "phrasal-verbs",
            f'"{phrase}"',
            f'use one verb: "{PHRASAL_VERBS[phrase]}"',
        )


def check_run_ons(unit, path, surface, found):
    limit = MAX_PROCEDURAL if unit.procedural else MAX_DESCRIPTIVE
    rule = "5.1" if unit.procedural else "6.3"
    parts = sentences(unit.text)
    for sentence in parts:
        # No cheap prefilter here. STE counting usually LOWERS the total (Rules
        # 8.5 thru 8.7), but "Stop()" becomes two tokens once the parenthesis is
        # collapsed, so a whitespace count can undercount. Measuring 42 fewer
        # run-ons is worse than measuring them 3 seconds slower.
        count = word_count(sentence)
        if count > limit:
            add(
                found,
                unit,
                path,
                surface,
                "run-ons",
                f"{count} words (STE Rule {rule} allows {limit})",
                "one topic per sentence, or a vertical list (Rule 4.3)",
                sentence,
            )
        if ";" in sentence:
            add(
                found,
                unit,
                path,
                surface,
                "run-ons",
                "semicolon (STE Rule 8.1)",
                "write two sentences, or a vertical list",
                sentence,
            )
    if unit.paragraph and len(parts) > MAX_SENTENCES_PER_PARAGRAPH:
        add(
            found,
            unit,
            path,
            surface,
            "run-ons",
            f"{len(parts)} sentences in one paragraph (STE Rule 6.6 allows 6)",
            "split the paragraph",
            parts[0],
        )


def check_synonyms(all_units, path, surface, found):
    """One finding per document per rotated concept."""
    text = " ".join(u.text for u in all_units)
    lowered = text.lower()
    for canonical, rotations in TERM_SETS:
        seen = [r for r in rotations if _TERM_RE[r].search(lowered)]
        if not seen:
            continue
        uses_canonical = bool(_CANON_RE[canonical].search(lowered))
        if not (uses_canonical or len(seen) > 1):
            continue
        line = next(
            (
                u.line
                for u in all_units
                for r in seen
                if _TERM_RE[r].search(u.text.lower())
            ),
            1,
        )
        detail = ", ".join(seen)
        if uses_canonical:
            detail += f" beside {canonical}"
        found.append(
            Finding(
                path,
                line,
                surface,
                "synonym-rotation",
                detail,
                f'use "{canonical}" every time',
                "",
            )
        )


def extract(path, text, surface):
    """Return the reviewable units of one document."""
    if surface == SURFACE_YANG:
        return units_yang(text)
    if surface == SURFACE_GO:
        return units_go(text.splitlines())
    return units_markdown(blank_html_comments(text).splitlines())


def review(path, text, surface):
    """Return (findings, skip_reason) for one document."""
    match = IGNORE_FILE.search(text)
    if match:
        return [], match.group("reason")
    head = "\n".join(text.splitlines()[:8])
    if any(marker in head for marker in GENERATED_MARKERS):
        return [], "generated file"

    found = []
    all_units = extract(path, text, surface)
    for unit in all_units:
        check_hedging(unit, path, surface, found)
        check_plain_words(unit, path, surface, found)
        check_latin(unit, path, surface, found)
        check_restricted_meanings(unit, path, surface, found)
        check_articles(unit, path, surface, found)
        check_frozen_verbs(unit, path, surface, found)
        check_marketing(unit, path, surface, found)
        check_phrasal(unit, path, surface, found)
        check_run_ons(unit, path, surface, found)
    if surface != SURFACE_GO:  # per-file term consistency is a document property
        check_synonyms(all_units, path, surface, found)
    found.sort(key=lambda f: (f.line, f.habit))
    return found, None


# --------------------------------------------------------------------------
# File selection
# --------------------------------------------------------------------------


def excluded(rel):
    if any(rel.startswith(d) for d in EXCLUDE_DIRS):
        return True
    return any(fnmatch(rel, pattern) for pattern in EXCLUDE_GLOBS)


def default_files(root):
    out = set()
    for pattern in DEFAULT_GLOBS:
        out.update(root.glob(pattern))
    return sorted(p for p in out if not excluded(p.relative_to(root).as_posix()))


def changed_files(root):
    try:
        raw = subprocess.run(
            ["git", "diff", "--name-only", "HEAD", "--"],
            cwd=root,
            capture_output=True,
            text=True,
            check=True,
        ).stdout
    except (subprocess.CalledProcessError, FileNotFoundError):
        return []
    out = []
    for name in (n.strip() for n in raw.split("\n")):
        if not name or Path(name).suffix not in SUFFIX_SURFACE or excluded(name):
            continue
        path = root / name
        if path.is_file():
            out.append(path)
    return out


# --------------------------------------------------------------------------
# The ratchet: each changed file against its own HEAD version
# --------------------------------------------------------------------------
#
# HEAD is the baseline, and the comparison is PER FILE. A committed baseline file
# was the first design and it was wrong twice over:
#
#   * Unattributable. It failed with "markdown run-ons 24186 -> 24188" and named
#     no file, so the author could not tell which sentence to fix.
#   * Red for someone else's work. Several sessions share this checkout. A
#     sibling session editing internal/component/mcp/*.go moved the global count
#     by +42 while this session touched no Go file at all. A gate that reddens
#     because a colleague is typing gets switched off, and then it guards
#     nothing.
#
# Per-file-vs-HEAD also makes legacy prose free: a file nobody touched cannot
# fail, so there is no 44000-finding baseline to maintain and no `--bless` that
# could ever be used to reach green.


def empty_counts():
    return {s: dict.fromkeys(HABITS.values(), 0) for s in SURFACES}


def git_lines(root, args):
    try:
        done = subprocess.run(
            ["git", "-c", "core.quotePath=false", *args],
            cwd=root,
            capture_output=True,
            text=True,
            check=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return None
    return [line for line in done.stdout.split("\n") if line.strip()]


def head_text(root, rel):
    """The file's content at HEAD, or "" when HEAD has no such file."""
    try:
        done = subprocess.run(
            ["git", "-c", "core.quotePath=false", "show", f"HEAD:{rel}"],
            cwd=root,
            capture_output=True,
            text=True,
            errors="replace",
        )
    except FileNotFoundError:
        return ""
    return done.stdout if done.returncode == 0 else ""


def candidates(root, only=None):
    """(current_path, head_path) for every reviewable file that differs from HEAD.

    head_path follows a rename, so moving a legacy document does not report its
    whole inherited content as new.

    `only` restricts the set to named repo-relative paths. That is the commit-time
    form: several sessions share this checkout, so the working tree is the wrong
    unit of attribution, and the files of ONE commit are the right one.
    """
    if only is not None:
        # A prefix strip, never lstrip: `str.lstrip("./")` removes a character
        # SET, so ".claude/rules/x.md" became "claude/rules/x.md", failed the
        # is_file() test below, and left 20 tracked files silently ungated.
        wanted = {p[2:] if p.startswith("./") else p for p in only}
        renames = {}
        status = git_lines(root, ["diff", "--name-status", "-M", "HEAD", "--"]) or []
        for line in status:
            parts = line.split("\t")
            if parts[0].startswith("R") and len(parts) >= 3:
                renames[parts[2]] = parts[1]
        keep = []
        for name in sorted(wanted):
            if Path(name).suffix not in SUFFIX_SURFACE or excluded(name):
                continue
            if (root / name).is_file():
                keep.append((name, renames.get(name, name)))
        return keep

    out = []
    status = git_lines(root, ["diff", "--name-status", "-M", "HEAD", "--"])
    if status is None:
        return out
    for line in status:
        parts = line.split("\t")
        code = parts[0]
        if code.startswith("D"):  # deleted: nothing left to review
            continue
        if code.startswith("R") and len(parts) >= 3:
            out.append((parts[2], parts[1]))
        else:
            out.append((parts[-1], parts[-1]))
    untracked = git_lines(root, ["ls-files", "--others", "--exclude-standard"]) or []
    out.extend((name, name) for name in untracked)

    keep = []
    for current, before in out:
        if Path(current).suffix not in SUFFIX_SURFACE or excluded(current):
            continue
        if (root / current).is_file():
            keep.append((current, before))
    return sorted(set(keep))


def ratchet(root, limit, only=None):
    """Report every habit that GREW in a file this working tree changed.

    Returns (growth, examined). growth is a list of
    (path, habit, was, now, new_findings). `only` limits it to named paths.
    """
    growth = []
    examined = 0
    for current, before in candidates(root, only):
        surface = SUFFIX_SURFACE[Path(current).suffix]
        try:
            text = (root / current).read_text("utf-8", "replace")
        except OSError:
            continue
        now_found, skip_reason = review(current, text, surface)
        if skip_reason:
            continue
        examined += 1
        old_found, _ = review(before, head_text(root, before), surface)
        now_counts = {}
        old_counts = {}
        for finding in now_found:
            now_counts[finding.habit] = now_counts.get(finding.habit, 0) + 1
        for finding in old_found:
            old_counts[finding.habit] = old_counts.get(finding.habit, 0) + 1
        for habit, now in sorted(now_counts.items(), key=lambda kv: SLUGS[kv[0]]):
            was = old_counts.get(habit, 0)
            if now > was:
                fresh = added(now_found, old_found, habit)[:limit]
                growth.append((current, habit, was, now, fresh))
    return growth, examined


def added(now_found, old_found, habit):
    """The findings of one habit that are new since HEAD.

    Matched on content, not on line number: an edit higher in the file moves
    every line below it. Printing all 34 findings of a file that grew by 2 tells
    the author nothing about which 2 sentences to rewrite.
    """
    seen = {}
    for finding in old_found:
        if finding.habit == habit:
            key = (finding.detail, finding.excerpt)
            seen[key] = seen.get(key, 0) + 1
    fresh = []
    for finding in now_found:
        if finding.habit != habit:
            continue
        key = (finding.detail, finding.excerpt)
        if seen.get(key):
            seen[key] -= 1
            continue
        fresh.append(finding)
    return fresh


def tally(findings):
    counts = empty_counts()
    for finding in findings:
        counts[finding.surface][finding.habit] += 1
    return counts


def total(counts, habit):
    return sum(counts[s][habit] for s in SURFACES)


def report(findings, counts, reviewed, skipped, limit):
    by_habit = {}
    for finding in findings:
        by_habit.setdefault(finding.habit, []).append(finding)
    for number in sorted(HABITS):
        slug = HABITS[number]
        group = by_habit.get(slug)
        if not group:
            continue
        print(f"\nhabit {number}: {slug}  ({len(group)})")
        for finding in group[:limit]:
            print(f"  {finding}")
        if len(group) > limit:
            print(f"  ... {len(group) - limit} more (use --json for all)")

    print(f"\nste_check: {len(findings)} finding(s) in {reviewed} document(s)")
    header = "  habit                    " + "".join(f"{s:>18}" for s in SURFACES)
    print(header)
    for number in sorted(HABITS):
        slug = HABITS[number]
        row = f"  {number} {slug:22}" + "".join(
            f"{counts[s][slug]:>18}" for s in SURFACES
        )
        print(row)
    if skipped:
        print(f"  skipped: {skipped} document(s) (generated, or ste: ignore-file)")
    print("\nRule: ai/rules/writing.md")


def run_gate(root, args):
    """The gate: no habit may grow in a file this working tree changed.

    With paths, only those files are gated. commit_helper.py uses that form to
    gate exactly the files of one commit.
    """
    only = args.paths or None
    growth, examined = ratchet(root, args.max_report, only)

    if args.json:
        print(
            json.dumps(
                {
                    "files-examined": examined,
                    "growth": [
                        {
                            "file": path,
                            "habit": habit,
                            "habit-number": SLUGS[habit],
                            "was": was,
                            "now": now,
                            "findings": [f.as_dict() for f in fresh],
                        }
                        for path, habit, was, now, fresh in growth
                    ],
                },
                indent=2,
            )
        )
        return EXIT_HABIT_GREW if growth else 0

    if growth:
        print(
            "ste_check: FAIL -- an ASD-STE100 habit grew in a file you changed\n",
            file=sys.stderr,
        )
        for path, habit, was, now, fresh in growth:
            print(
                f"  {path}: habit {SLUGS[habit]} {habit}: {was} -> {now} (+{now - was})",
                file=sys.stderr,
            )
            for finding in fresh:
                print(
                    f"      {finding.line}: {finding.detail} -> {finding.fix}",
                    file=sys.stderr,
                )
        print(
            "\nRewrite the prose. HEAD is the baseline, so only your own new text counts."
            "\nWhole-tree report: make ze-ste-review"
            "\nRule: ai/rules/writing.md",
            file=sys.stderr,
        )
        return EXIT_HABIT_GREW

    if not args.quiet:
        print(f"ste_check: OK -- no habit grew in {examined} changed document(s)")
    return 0


def main():
    parser = argparse.ArgumentParser(
        description="Review the repository's writing against ASD-STE100 Issue 9."
    )
    parser.add_argument(
        "paths", nargs="*", help='documents to review, or "-" for stdin'
    )
    parser.add_argument(
        "--check", action="store_true", help="gate: each changed file against HEAD"
    )
    parser.add_argument("--changed", action="store_true", help="changed files only")
    parser.add_argument("--json", action="store_true", help="machine-readable output")
    parser.add_argument("--quiet", action="store_true", help="exit code only")
    parser.add_argument("--max-report", type=int, default=15, help="per-habit lines")
    args = parser.parse_args()

    root = Path(__file__).resolve().parents[2]

    if args.check:
        return run_gate(root, args)

    findings = []
    skipped = 0
    reviewed = 0

    if args.paths == ["-"]:
        text = sys.stdin.read()
        found, _ = review("<stdin>", text, SURFACE_STDIN)
        findings.extend(found)
        reviewed = 1
    else:
        explicit = bool(args.paths)
        if explicit:
            files = [Path(p) if Path(p).is_absolute() else root / p for p in args.paths]
        elif args.changed:
            files = changed_files(root)
        else:
            files = default_files(root)

        for path in files:
            # A discovered file can vanish between the glob and the read: several
            # sessions share this checkout, and a spec closure deletes files.
            # That is not this tool's failure. A path the CALLER named is.
            if not path.is_file():
                if explicit:
                    print(f"ste_check: not a file: {path}", file=sys.stderr)
                    return 1
                continue
            surface = SUFFIX_SURFACE.get(path.suffix)
            if surface is None:
                print(f"ste_check: unsupported suffix: {path}", file=sys.stderr)
                return 1
            rel = (
                path.relative_to(root).as_posix() if path.is_relative_to(root) else path
            )
            try:
                text = path.read_text("utf-8", "replace")
            except OSError:
                if explicit:
                    print(f"ste_check: cannot read: {path}", file=sys.stderr)
                    return 1
                continue
            found, skip_reason = review(rel, text, surface)
            if skip_reason:
                skipped += 1
                continue
            reviewed += 1
            findings.extend(found)

    counts = tally(findings)

    if args.json:
        print(
            json.dumps(
                {
                    "documents-reviewed": reviewed,
                    "documents-skipped": skipped,
                    "counts": counts,
                    "totals": {h: total(counts, h) for h in HABITS.values()},
                    "findings": [f.as_dict() for f in findings],
                },
                indent=2,
            )
        )
        return 0

    if args.quiet:
        return 1 if findings else 0

    report(findings, counts, reviewed, skipped, args.max_report)
    return 0


if __name__ == "__main__":
    sys.exit(main())
