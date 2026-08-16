#!/usr/bin/env python3
"""Validate that every ai/rules/*.md rule follows the canonical format.

The format (see ai/rules/rule-format.md) is:

    # Title
    **When:** <trigger>
    **Severity:** blocking|advisory
    **Related:** slug, slug          (optional)
    ...body...

Tooling (rules_index.py, rules_condensed.py, and the eager @-import of
TRIGGERS.md / CORE.md) relies on this block being present and machine-readable. This
linter is the durable gate that keeps it true; it runs inside `make ze-doc-test`.

Beyond presence, two checks keep the block HONEST, because a metadata field that
is syntactically fine but semantically wrong is worse than a missing one: it
routes confidently to the wrong place.

  1. `**When:**` must name a SITUATION, not restate the directive. It is the
     routing key an agent matches against the task in hand, so it has to start
     with a temporal opener or a gerund ("when ...", "before ...", "writing ...").
     "All CLI commands MUST follow these patterns" is a directive: it matches
     every task and therefore routes nothing. The same check catches triggers
     copied out of a wrapped body line and cut mid-clause; eight had shipped
     into the digest artifacts, where they were read by every session.

  2. `**Severity:**` must agree with the prose. A rule that declares `advisory`
     while its body says BLOCKING teaches readers that the field is decoration.

A third pass reads the POINT files under ai/rules/points/ rather than the
rendered rules, because that is where an author writes and where `level:` lives.
Every `kind: directive` point MUST state its obligation in RFC 2119 language:
one of the capitalised keywords, and a `level:` naming the strongest keyword the
body states. An instruction whose weight a reader has to infer from tone is an
instruction two readers weigh differently, and the corpus had 509 of them.
Lowercase `must`/`shall`/`should`/`may` in a directive body is refused for the
same reason: it reads as the obligation word while carrying none of its force,
and `ai/rules/writing.md` bans the hedging spelling outright.

The pass is scoped to `kind: directive` on purpose. A `table` is usually a
lookup and a `note` is usually context; forcing MUST into a two-column glossary
would add a word without adding an obligation.

Usage:
    python3 scripts/dev/rules_lint.py           # report violations, exit 1 if any
    python3 scripts/dev/rules_lint.py --quiet    # exit code only
    python3 scripts/dev/rules_lint.py --points   # the RFC 2119 pass only
"""

import re
import sys
from pathlib import Path

# Generated aggregates are not rules. Recognised by SHAPE (an all-caps stem, the
# repo's convention for INDEX.md / CONDENSED.md / TRIGGERS.md / CORE.md) as well
# as by name, so a new artifact is never linted as a malformed rule.
SKIP = {"INDEX.md", "CONDENSED.md"}
SEVERITIES = {"blocking", "advisory"}

# A trigger must open with one of these, or with a gerund (any -ing word not in
# NOT_GERUND). Keep the set small and closed: the value of a uniform routing
# column is that it can be scanned, and every addition is one more shape to read.
OPENERS = (
    "when ",
    "whenever ",
    "while ",
    "before ",
    "after ",
    "during ",
    "if ",
    "unless ",
    "once ",
    "upon ",
    "on ",
    "at ",
    "any time ",
    "every time ",
    "each time ",
    "prior to ",
    "as soon as ",
)

# Words that end in -ing but are not gerunds, so they cannot open a trigger.
NOT_GERUND = {"nothing", "something", "anything", "everything", "string", "thing"}

# Truncation tells: a trigger cut out of a wrapped body line keeps its dangling
# punctuation or a stray bold marker.
TRUNCATED_TAIL = (",", ";", ":", "-", "--")

# ... or it stops on a word no English clause ends on. planning.md shipped
# "...and is enforced by" for months: syntactically a fine metadata line, and
# useless as a trigger.
DANGLING_LAST_WORD = {
    "a",
    "an",
    "and",
    "any",
    "are",
    "as",
    "at",
    "be",
    "been",
    "before",
    "but",
    "by",
    "each",
    "every",
    "for",
    "from",
    "in",
    "into",
    "is",
    "its",
    "not",
    "of",
    "on",
    "or",
    "the",
    "their",
    "then",
    "these",
    "this",
    "those",
    "to",
    "was",
    "were",
    "which",
    "with",
}
# Exact spelling the consumers require: rules_condensed.py:META_LINE and
# rules_index.py match `**When:**` / `**Severity:**` / `**Related:**`
# case-sensitively, so the lint must too -- a lowercase key that "passes" here
# would leak into the digest/INDEX bodies unparsed. Keep this in sync with them.
CANON_KEYS = ("When", "Severity", "Related")

CODE_SPAN = re.compile(r"`[^`]*`")
# A BLOCKING mention that carries this marker belongs to the artifact the line
# describes, not to the rule declaring it. Line-scoped on purpose: a file-scoped
# opt-out would silently cover every later addition to that file.
SEVERITY_NOTE = re.compile(r"<!--\s*severity-note:")

META_LINE = re.compile(r"^\*\*(?P<key>[A-Za-z]+):\*\*\s*(?P<val>.*)$")
H1 = re.compile(r"^#\s+(\S.*)$")
RELATED_SLUG = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


def _strip_markup(text):
    """Drop the markup a trigger may legitimately carry, keeping the words."""
    return text.replace("**", "").replace("`", "").replace("*", "").strip()


def check_trigger(trigger):
    """Return violations for a '**When:**' value that cannot route a task."""
    problems = []
    bare = _strip_markup(trigger)
    if not bare:
        return ["'**When:**' has no text once markup is stripped"]

    # An ODD number of '**' outside code spans means one marker lost its partner,
    # which is what truncating a wrapped bold body line produces. Balanced pairs
    # are legitimate emphasis, and `test/**/*.ci` inside backticks is a glob.
    if CODE_SPAN.sub("", trigger).count("**") % 2:
        problems.append(
            "'**When:**' has an unbalanced '**' -- it was copied out of a wrapped "
            "bold body line, not written as a trigger"
        )
    if bare.endswith(TRUNCATED_TAIL):
        problems.append(
            f"'**When:**' ends with {bare[-1]!r}, so it is a truncated sentence "
            "rather than a complete situation"
        )
    last = bare.rstrip(".").split()[-1].lower() if bare.split() else ""
    if last in DANGLING_LAST_WORD:
        problems.append(
            f"'**When:**' ends on {last!r}, so the situation is cut off mid-clause"
        )

    lowered = bare.lower()
    words = lowered.split()
    first = words[0].strip(",;:.") if words else ""
    is_gerund = first.endswith("ing") and len(first) >= 5 and first not in NOT_GERUND
    if not (is_gerund or lowered.startswith(OPENERS)):
        problems.append(
            "'**When:**' must name a situation, not a directive: start it with a "
            "temporal opener (when/whenever/before/after/while/if/once/during) or "
            f"a gerund (writing/adding/reviewing/...). Got {bare.split()[0]!r}. "
            "See ai/rules/rule-format.md 'The trigger is a routing key'"
        )
    return problems


def check_severity_agrees(severity, title, lines, body_start):
    """Return violations where the declared severity contradicts the prose.

    Table rows are exempt: reference rules such as repo-maintenance.md tabulate OTHER
    rules' severities, and those cells say nothing about this rule's own weight.
    A prose line that describes another artifact's severity can say so with a
    trailing `<!-- severity-note: ... -->`.
    """
    problems = []
    if "BLOCKING" in title:
        problems.append(
            "the title must not say BLOCKING -- '**Severity:** blocking' carries "
            "that, and a title marker cannot be read by tooling"
        )
    if severity != "advisory":
        return problems
    for offset, line in enumerate(lines[body_start:]):
        stripped = line.strip()
        if stripped.startswith(("|", ">")):
            continue  # table row or quoted example, not this rule's own claim
        if "BLOCKING" not in stripped or SEVERITY_NOTE.search(stripped):
            continue
        problems.append(
            f"declares '**Severity:** advisory' but line {body_start + offset + 1} "
            f"says BLOCKING ({stripped[:60]!r}) -- raise the severity, drop the "
            "word, or mark the line <!-- severity-note: whose severity this is -->"
        )
        break
    return problems


def check_rule(path):
    """Return a list of human-readable violation strings for one rule file."""
    problems = []
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()

    # Find the title: first non-blank line, must be an H1.
    idx = 0
    while idx < len(lines) and not lines[idx].strip():
        idx += 1
    title_match = H1.match(lines[idx].strip()) if idx < len(lines) else None
    if not title_match:
        problems.append("first non-blank line must be a single '# Title'")
        return problems
    title = title_match.group(1)
    idx += 1

    # Allow one blank line between title and the metadata block.
    while idx < len(lines) and not lines[idx].strip():
        idx += 1

    # Collect the contiguous metadata block (**Key:** value lines). Keys are
    # case-sensitive: a mis-cased key (e.g. `**when:**`) is a violation, not a
    # silently-accepted alias, because the consumers only parse the exact case.
    meta = {}
    order = []
    while idx < len(lines) and lines[idx].strip():
        m = META_LINE.match(lines[idx].strip())
        if not m:
            break
        key = m.group("key")
        if key not in CANON_KEYS:
            problems.append(
                f"metadata key '**{key}:**' must be one of "
                f"{'/'.join(CANON_KEYS)} (exact case)"
            )
            break
        meta[key] = m.group("val").strip()
        order.append(key)
        idx += 1

    if "When" not in meta:
        problems.append("missing required '**When:** <trigger>' line")
    elif not meta["When"]:
        problems.append("'**When:**' line is empty")
    else:
        problems.extend(check_trigger(meta["When"]))

    if "Severity" not in meta:
        problems.append("missing required '**Severity:** blocking|advisory' line")
    elif meta["Severity"] not in SEVERITIES:
        problems.append(
            f"'**Severity:**' must be one of {sorted(SEVERITIES)}, got "
            f"'{meta['Severity']}'"
        )
    else:
        problems.extend(check_severity_agrees(meta["Severity"], title, lines, idx))

    # Order: When before Severity before Related, when present.
    canon = [k for k in CANON_KEYS if k in order]
    present_in_order = [k for k in order if k in CANON_KEYS]
    if present_in_order != canon:
        problems.append(
            "metadata keys must be ordered When, Severity, Related "
            f"(found {', '.join(present_in_order)})"
        )

    if "Related" in meta and meta["Related"]:
        for slug in [s.strip() for s in meta["Related"].split(",") if s.strip()]:
            if not RELATED_SLUG.match(slug):
                problems.append(
                    f"'**Related:**' entry '{slug}' must be a bare rule slug "
                    "(filename without .md, no path)"
                )

    # Nothing but the metadata block may sit before the first body line: the
    # loop above already stopped at the first non-metadata line, so if the block
    # was empty we would have flagged missing When/Severity. Guard the case
    # where prose precedes the block entirely.
    if "When" not in meta and "Severity" not in meta and not problems:
        problems.append("no metadata block found directly after the title")

    return problems


# RFC 2119 / RFC 8174 keywords, longest first so "MUST NOT" wins over "MUST".
# Keep this tuple and RFC_KEYWORD in sync with the copy in
# .claude/hooks/pretool-writeedit.py (c_rule_point_rfc_language): the hook
# refuses at write time, this pass refuses at gate time, and a keyword accepted
# by one and not the other would make the two disagree about the same file.
RFC_LEVELS = (
    "MUST NOT",
    "SHALL NOT",
    "SHOULD NOT",
    "NOT RECOMMENDED",
    "MUST",
    "SHALL",
    "REQUIRED",
    "SHOULD",
    "RECOMMENDED",
    "MAY",
    "OPTIONAL",
)
RFC_KEYWORD = re.compile(r"\b(?:" + "|".join(RFC_LEVELS) + r")\b")

# The strongest keyword a body states, for the `level:` field. Synonyms collapse
# onto the keyword that names the level, so `level:` has one spelling per level
# and a reader never has to decide whether REQUIRED outranks MUST.
LEVEL_CANON = {
    "MUST": "MUST",
    "SHALL": "MUST",
    "REQUIRED": "MUST",
    "MUST NOT": "MUST NOT",
    "SHALL NOT": "MUST NOT",
    "SHOULD": "SHOULD",
    "RECOMMENDED": "SHOULD",
    "SHOULD NOT": "SHOULD NOT",
    "NOT RECOMMENDED": "SHOULD NOT",
    "MAY": "MAY",
    "OPTIONAL": "MAY",
}
LEVEL_RANK = ("MAY", "SHOULD NOT", "SHOULD", "MUST NOT", "MUST")

# RFC 2119 ranks obligation by STRENGTH, and it does not rank MUST against MUST
# NOT: they are one tier with opposite polarity, and so are SHOULD and SHOULD
# NOT. So `level:` names the strongest TIER a body states, and either polarity
# in that tier is a true answer. Ordering the two would force a point whose
# central clause is a prohibition to declare `MUST` because some other sentence
# in it happened to be positive, which is the prohibition going unrecorded.
LEVEL_TIERS = (("MAY",), ("SHOULD", "SHOULD NOT"), ("MUST", "MUST NOT"))

# Lowercase obligation words inside directives. Quoted Markdown is removed first,
# because a rule that reproduces another artifact does not state that artifact's
# obligations.
LOWER_MODAL = re.compile(r"(?<![\w-])(must|shall|should|may)\b(?![-\w])")
POINT_FRONTMATTER = re.compile(
    r"\A---\n(?P<fm>.*?)\n---\n(?P<body>.*)\Z", re.DOTALL
)
FENCE_OPEN = re.compile(r"^ {0,3}(?P<mark>`{3,}|~{3,})(?P<info>[^\n]*)$")
BLOCKQUOTE = re.compile(r"^[ \t]{0,3}>.*$", re.MULTILINE)


def strip_fences(text):
    """Remove Markdown fenced blocks."""
    output = []
    fence_char = ""
    fence_length = 0
    for line in text.splitlines(keepends=True):
        bare = line.rstrip("\r\n")
        if fence_char:
            candidate = bare.lstrip(" ")
            indent = len(bare) - len(candidate)
            marker = candidate.rstrip(" \t")
            if (
                indent <= 3
                and len(marker) >= fence_length
                and marker == fence_char * len(marker)
            ):
                fence_char = ""
                fence_length = 0
            continue
        opened = FENCE_OPEN.match(bare)
        if opened:
            mark = opened.group("mark")
            if mark[0] != "`" or "`" not in opened.group("info"):
                fence_char = mark[0]
                fence_length = len(mark)
                continue
        output.append(line)
    return "".join(output)


def strip_quoted(body):
    """Drop quoted Markdown from the obligations that a point states."""
    body = strip_fences(body)
    body = BLOCKQUOTE.sub("", body)
    return CODE_SPAN.sub("", body)



def strongest_tier(body):
    """The levels of the strongest tier a body states, or () when it states none.

    A tuple rather than one level: the tier is the answer, and either polarity in
    it is a true `level:`. See LEVEL_TIERS.
    """
    found = {LEVEL_CANON[k] for k in RFC_KEYWORD.findall(strip_quoted(body))}
    for tier in reversed(LEVEL_TIERS):
        hit = tuple(level for level in tier if level in found)
        if hit:
            return hit
    return ()


def check_point(path, text):
    """Return violations for one ai/rules/points/ point file."""
    m = POINT_FRONTMATTER.match(text)
    if not m:
        return ["no '---' frontmatter block: see ai/rules/rule-format.md"]
    fm, body = m.group("fm"), m.group("body")
    kind = re.search(r"^kind:\s*(\S*)", fm, re.M)
    if not (kind and kind.group(1) == "directive"):
        return []

    problems = []
    visible = strip_quoted(body)
    tier = strongest_tier(body)
    if not tier:
        problems.append(
            "a directive MUST state its obligation in RFC 2119 language: no "
            f"capitalised keyword ({', '.join(RFC_LEVELS[:6])}, ...) appears in "
            "the body"
        )
    lower = sorted({w for w in LOWER_MODAL.findall(visible)})
    if lower:
        problems.append(
            f"lowercase obligation word(s) {', '.join(repr(w) for w in lower)}: "
            "capitalise the RFC 2119 keyword, or rewrite the sentence so it "
            "carries no modal (ai/rules/writing.md bans the hedging spelling)"
        )

    declared = re.search(r"^level:[^\S\n]*(.*)$", fm, re.M)
    declared = declared.group(1).strip() if declared else ""
    if declared and declared not in LEVEL_RANK:
        problems.append(f"'level: {declared}' is not one of {', '.join(LEVEL_RANK)}")
    elif tier and declared not in tier:
        problems.append(
            f"'level: {declared or '(empty)'}' disagrees with the body, whose "
            f"strongest obligation is {' or '.join(tier)}"
        )
    return problems


def check_points(points_dir):
    """Return {relative path: [violations]} for every point file under the tree."""
    failures = {}
    n = 0
    for md in sorted(points_dir.rglob("*.md")):
        if md.name == "manifest.md" or md.stem.isupper():
            continue
        n += 1
        problems = check_point(md, md.read_text(encoding="utf-8", errors="replace"))
        if problems:
            failures[str(md.relative_to(points_dir.parents[2]))] = problems
    return n, failures


def main():
    root = Path(__file__).resolve().parents[2]
    rules_dir = root / "ai" / "rules"
    quiet = "--quiet" in sys.argv
    points_only = "--points" in sys.argv

    if not rules_dir.is_dir():
        print(f"error: {rules_dir} not found", file=sys.stderr)
        sys.exit(1)

    failures = {}
    n = 0
    if not points_only:
        for md in sorted(rules_dir.glob("*.md")):
            if md.name in SKIP or md.stem.isupper():
                continue
            n += 1
            problems = check_rule(md)
            if problems:
                failures[md.name] = problems

        if failures:
            if not quiet:
                print(
                    f"rules_lint: {len(failures)}/{n} rule file(s) violate the format\n"
                )
                for name, problems in sorted(failures.items()):
                    print(f"  ai/rules/{name}")
                    for p in problems:
                        print(f"      - {p}")
                print("\nFormat spec: ai/rules/rule-format.md")
            sys.exit(1)

    points_dir = rules_dir / "points"
    points_n, point_failures = (
        check_points(points_dir) if points_dir.is_dir() else (0, {})
    )
    if point_failures:
        if not quiet:
            print(
                f"rules_lint: {len(point_failures)}/{points_n} rule point(s) do not "
                "state their obligation in RFC 2119 language\n"
            )
            for name, problems in sorted(point_failures.items()):
                print(f"  {name}")
                for p in problems:
                    print(f"      - {p}")
            print(
                "\nFormat spec: ai/rules/rule-format.md 'Every directive states a level'"
            )
        sys.exit(1)

    if not quiet:
        if not points_only:
            print(f"rules_lint: {n} rule file(s) conform to ai/rules/rule-format.md")
        print(f"rules_lint: {points_n} rule point(s) state an RFC 2119 level")


if __name__ == "__main__":
    main()
