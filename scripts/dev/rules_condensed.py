#!/usr/bin/env python3
"""Generate the two rule-digest artifacts from ONE parse of ai/rules/*.md.

  TRIGGERS.md   one routing line per rule -- path, severity, `**When:**` trigger
  CORE.md       the directives of the rules that must never sit behind a trigger

A third artifact, CONDENSED.md, held every rule's directives in one file. It was
deleted on 2026-08-03: nothing loaded it, and it cost a 5,182-line regeneration
on every rule edit. TRIGGERS.md keeps every rule NAMED in every session, so no
rule became undiscoverable, and the rule's own file is one Read away.

Both come from the same parse. A second source would drift, and a routed
section must read identically to the digest section it replaces.

Extraction relies on the canonical rule format (ai/rules/rule-format.md): a
`**When:**` / `**Severity:**` metadata block, imperatives under directive
sections, and rationale / examples under denylisted headings. It is therefore
deterministic:

  KEEP  the metadata block, and every non-denylisted section's headings, list
        items, table rows, and bold-directive lines (plus the first sentence of
        each prose paragraph -- the rule statement).
  DROP  denylisted sections (Rationale, Why, Examples, ...), fenced code blocks,
        HTML comments, and `Rationale:`/`See:` pointer lines.

## Core membership is DERIVED, never listed

`ai/rules/rule-precedence.md` owns the ladder that ranks rules, and its rungs 1
and 2 (irreversible action; correctness owed to someone outside this repo) are
the rules that apply before a task type is even known. This generator PARSES
that table. Rename a rule there and the core follows; a filename list here would
read the same until the ladder changed underneath it
(`ai/rules/evidence.md`).

Two further members are structural, not editorial:

  - `rule-precedence.md` itself. It is the ladder every conflict is resolved
    against, so it cannot be the one thing behind a trigger.
  - Any rule the router provably cannot route: no trigger, an unknown severity,
    or a trigger with no term to match on. It goes INTO the core rather than
    being dropped from both artifacts (`ai/rules/evidence.md`).

Derived mechanically, never hand-edited. Kept fresh by `make ze-rules-condensed-update`;
`--check` fails when either artifact is stale.

Usage:
    python3 scripts/dev/rules_condensed.py            # regenerate both
    python3 scripts/dev/rules_condensed.py --check    # exit 1 if either is stale
    python3 scripts/dev/rules_condensed.py --payload  # measure the always-on load
"""

import re
import sys
from collections import Counter
from pathlib import Path

# Generated artifacts are skipped by SHAPE, an all-caps stem, the repo's
# convention for INDEX.md / TRIGGERS.md / CORE.md, so adding a third artifact
# needs no edit here and no artifact can be parsed as a rule. CONDENSED.md stays
# listed so a stale copy left in a working tree is never parsed as a rule.
SKIP = {"INDEX.md", "CONDENSED.md"}

SEVERITIES = {"blocking", "advisory"}

# AC-1. A trigger longer than this is cut at a word boundary: the index is a
# routing key, and the path on the same line makes the full clause one read away.
MAX_TRIGGER_LINE = 200

# AC-5. The budget the always-loaded payload must come in under.
TOKEN_BUDGET = 40000

# Bytes per token, used by the payload report.
BYTES_PER_TOKEN = 4

# Terms that carry no routing signal. A trigger is unroutable when nothing but
# these survive, which is the fail-closed case that puts it in the core.
STOPWORDS = frozenset(
    """a an and any are as at be been before being but by can do does for from has have
    if in into is it its more most no not of on once only or other over own same so some
    such than that the their them then there these they this those to under until up upon
    was were what when whenever whether which while who whom why will with within without
    you your yourself each every after during unless prior soon time about above across
    against all also am because both cannot could did doing done down else ever few
    further had he her here hers him his how i just me my nor now off our ours out same
    she should since still sub then thus too very we would""".split()
)


def significant_terms(text):
    """Lowercase content words of a trigger or a task description.

    A compound yields BOTH itself and its parts: `wire-encoding` also emits
    `wire` and `encoding`, because "wire-encoding path" and "wire encoding path"
    are the same phrase and a reader matches them alike. Without the split, a
    hyphen in a trigger silently costs it every match.

    Shared with `scripts/dev/rules_router.py` so the report scores triggers with
    the same tokenizer the core derivation uses. Two tokenizers would let a rule
    be judged routable here and unroutable there.
    """
    terms = set()
    for word in re.findall(r"[a-z0-9][a-z0-9.\-_/]*", (text or "").lower()):
        word = word.strip(".-_/")
        for part in [word] + re.split(r"[-_/.]+", word):
            if len(part) > 2 and part not in STOPWORDS:
                terms.add(part)
    return terms


# A trigger word appearing in more than this many triggers carries no
# routing signal: it is the vocabulary every rule shares.
MAX_TRIGGER_DF = 8

# How many distinctive trigger words a task must contain to surface the rule.
# One is too eager (a single shared word routes everything); three demands more
# overlap than a real task description carries.
MIN_HITS = 2


def distinctive_terms(rules):
    """Per-rule trigger terms, minus the vocabulary the triggers all share.

    "code", "test" and "any" appear in dozens of triggers and separate nothing.
    "gokrazy", "nlri" and "qdisc" separate a great deal.
    """
    df = Counter()
    per_rule = {}
    for rule in rules:
        terms = significant_terms(rule["trigger"])
        per_rule[rule["name"]] = terms
        df.update(terms)
    return {
        name: {t for t in terms if df[t] <= MAX_TRIGGER_DF}
        for name, terms in per_rule.items()
    }


def surfaced_by(rules, task_text, terms=None):
    """The rules a trigger index would surface for one task description."""
    terms = terms if terms is not None else distinctive_terms(rules)
    task_terms = significant_terms(task_text)
    return {r["name"] for r in rules if len(terms[r["name"]] & task_terms) >= MIN_HITS}


def unreachable_blocking(rules, corpus):
    """Blocking rules that NO task in the corpus would surface.

    This is the population routing puts at risk, and it is why the corpus is a
    generator input rather than a report an operator is trusted to act on. A
    hand-copied list of these names would read identically until the next rule
    landed (`ai/rules/evidence.md`).

    `corpus is None` means the caller did not ask for this derivation, which is
    what `rules_router` does on purpose. An EMPTY corpus is a different fact:
    the caller DID ask and the read returned nothing, so seven blocking rules
    leave the core with no signal. Both return the same set, so the empty case
    says so on stderr rather than passing for a real result
    (`ai/rules/evidence.md`).
    """
    if corpus is None:
        return set()
    if not corpus:
        print(
            "warning: the task corpus is empty, so no blocking rule can be shown "
            "unreachable and ai/rules/CORE.md loses that derivation -- check that "
            "plan/spec-*.md is readable",
            file=sys.stderr,
        )
        return set()
    terms = distinctive_terms(rules)
    reached = set()
    for entry in corpus:
        reached |= surfaced_by(rules, entry["text"], terms)
    return {
        r["name"]
        for r in rules
        if r["severity"] == "blocking" and r["name"] not in reached
    }


# A section is dropped whole when its heading's FIRST word is one of these:
# clearly explanation/examples, never directives. Matching the first word (not
# the exact heading) means "Why this matters", "Rationale and background", and
# "Example: the good case" all drop. Ambiguous generic headings (Notes,
# Reference, History, Appendix) are deliberately NOT here -- a rule may put real
# directives under them, and keeping a non-directive section is cheaper than
# silently dropping a directive one.
DENY_FIRST_WORDS = {
    "rationale",
    "why",
    "background",
    "example",
    "examples",
}

WS = re.compile(r"\s+")
H1 = re.compile(r"^#\s+(.+?)\s*$")
HEADING = re.compile(r"^(#{2,6})\s+(.*)$")
META_LINE = re.compile(r"^\*\*(When|Severity|Related):\*\*\s*(.*)$")
LIST_ITEM = re.compile(r"^\s*([-*+]|\d+[.)])\s+\S")
TABLE_ROW = re.compile(r"^\s*\|")
BOLD_DIRECTIVE = re.compile(r"^\*\*")
POINTER_LINE = re.compile(r"^(Rationale|Principle|See|Further reading)\b", re.I)
MAX_PROSE = 220


def norm_heading(text):
    text = re.sub(r"\(.*?\)", "", text)  # drop parentheticals like "(BLOCKING)"
    text = re.sub(r"[^a-z0-9 ]", "", text.lower()).strip()
    return WS.sub(" ", text)


def first_sentence(text, limit=MAX_PROSE):
    text = WS.sub(" ", text).strip()
    m = re.search(r"\.(\s|$)", text)
    if m and m.start() <= limit:
        return text[: m.start() + 1]
    if len(text) > limit:
        cut = text.rfind(" ", 0, limit - 1)
        return text[: cut if cut > 0 else limit - 1].rstrip() + "..."
    return text


def parse(raw_lines):
    """Return (title, meta_lines, body_lines)."""
    idx = 0
    while idx < len(raw_lines) and not raw_lines[idx].strip():
        idx += 1
    title = ""
    if idx < len(raw_lines):
        m = H1.match(raw_lines[idx].strip())
        if m:
            title = m.group(1)
            idx += 1
    while idx < len(raw_lines) and not raw_lines[idx].strip():
        idx += 1
    meta = []
    while idx < len(raw_lines) and raw_lines[idx].strip():
        m = META_LINE.match(raw_lines[idx].strip())
        if not m:
            break
        meta.append((m.group(1), m.group(2).strip()))
        idx += 1
    return title, meta, raw_lines[idx:]


def is_artifact(md):
    """True for a generated artifact (all-caps stem), which is never a rule."""
    return md.name in SKIP or md.stem.isupper()


def load_rules(rules_dir):
    """Every rule, parsed once. The single source all three artifacts read.

    Each entry carries the title, the ordered metadata pairs, the raw body, and
    the two routing fields pulled out of the metadata block for convenience.
    """
    rules = []
    for md in sorted(Path(rules_dir).glob("*.md")):
        if is_artifact(md):
            continue
        raw = md.read_text(encoding="utf-8", errors="replace").splitlines()
        title, meta, body = parse(raw)
        meta_map = dict(meta)
        rules.append(
            {
                "name": md.name,
                "stem": md.stem,
                "path": f"ai/rules/{md.name}",
                "title": title or md.stem,
                "meta": meta,
                "body": body,
                "trigger": meta_map.get("When", "").strip(),
                "severity": meta_map.get("Severity", "").strip().lower(),
            }
        )
    return rules


def trigger_lines(rules, core=frozenset()):
    """One routing row per rule: path, severity, trigger. Never longer than 200.

    A core rule is marked `always-on`, because the index's own header promises
    the reader can tell which bodies are already loaded. Without the marker that
    sentence is a claim the table does not support.

    A rule with no trigger still gets a line. Dropping it would make the one
    thing this index promises -- that every rule stays named -- untrue for
    exactly the rules that are hardest to route (`evidence.md`).
    """
    lines = []
    for rule in rules:
        severity = (
            rule["severity"] if rule["severity"] in SEVERITIES else "unclassified"
        )
        if rule["name"] in core:
            severity += ", always-on"
        trigger = WS.sub(" ", rule["trigger"]).strip() or "(no trigger: always read it)"
        prefix = f"| `{rule['path']}` | {severity} | "
        room = MAX_TRIGGER_LINE - len(prefix) - len(" |")
        if len(trigger) > room:
            cut = trigger.rfind(" ", 0, room - 3)
            trigger = trigger[: cut if cut > 0 else room - 3].rstrip(" ,;:-") + "..."
        lines.append(f"{prefix}{trigger} |")
    return lines


class LadderError(RuntimeError):
    """The precedence ladder could not be read, so the core cannot be derived.

    Raised instead of returning an empty set. An empty ladder and an unreadable
    one produce the same value, and the caller cannot tell them apart, so the
    layer that KNOWS it missed is the only one that can say so
    (`ai/rules/evidence.md`).
    """


def precedence_rung_slugs(rules, rungs=(1, 2)):
    """Rule slugs the ladder in `rule-precedence.md` puts on the named rungs.

    The ladder is a markdown table whose columns are located by their HEADER
    text, not by index, so re-ordering the table cannot silently empty the core.
    A backticked token that names no rule file (`CLAUDE.md`) is skipped: it is
    already loaded, and it is not a rule under `ai/rules/`.

    Header-text lookup is what makes re-ordering safe, and it is exactly what
    makes REWORDING dangerous: rename the column to `Level`, or rewrite the
    ladder as a list, and every row is skipped. So each way the parse can come
    back empty raises `LadderError` naming that cause. Returning the empty set
    dropped `git-safety`, `never-destroy-work`, `rfc-compliance` and
    `interop-and-goal-validation` out of the always-on core in silence.
    """
    source = next((r for r in rules if r["stem"] == "rule-precedence"), None)
    if source is None:
        raise LadderError(
            "ai/rules/rule-precedence.md is absent: the always-on core is derived "
            "from its ladder, so there is nothing left to derive it from"
        )

    known = {r["stem"] for r in rules}
    wanted = "/".join(str(n) for n in rungs)
    slugs = set()
    rung_col = rules_col = None
    saw_header = saw_rung_row = False
    for line in source["body"]:
        if not TABLE_ROW.match(line):
            # A markdown table cannot have gaps, so any non-row line ends it.
            # Carrying the column indexes past the table let a LATER table's
            # rows be read with this one's layout.
            rung_col = rules_col = None
            continue
        cells = [c.strip() for c in line.strip().strip("|").split("|")]
        headers = [c.lower() for c in cells]
        if "rung" in headers and "rules" in headers:
            rung_col, rules_col = headers.index("rung"), headers.index("rules")
            saw_header = True
            continue
        if rung_col is None or max(rung_col, rules_col) >= len(cells):
            continue
        if not cells[rung_col].isdigit() or int(cells[rung_col]) not in rungs:
            continue
        saw_rung_row = True
        for token in re.findall(r"`([^`]+)`", cells[rules_col]):
            stem = token.strip().removesuffix(".md")
            if stem in known:
                slugs.add(f"{stem}.md")

    if not saw_header:
        raise LadderError(
            "no table in ai/rules/rule-precedence.md has both a `Rung` and a "
            "`Rules` column: the columns are found by header text, so renaming "
            "either one, or rewriting the ladder as a list, empties the core"
        )
    if not saw_rung_row:
        raise LadderError(
            f"the ladder in ai/rules/rule-precedence.md has no rung {wanted} row: "
            "those rungs are what the always-on core is derived from"
        )
    if not slugs:
        raise LadderError(
            f"rung {wanted} of the ladder in ai/rules/rule-precedence.md names no "
            "rule under ai/rules/: the core would lose every destructive-action "
            "and outside-facing-correctness guard"
        )
    return slugs, source


def core_members(rules, rungs=(1, 2), corpus=None):
    """The always-on set, derived. Each member carries WHY it is eager.

    Four derivations, no list:

      1. The precedence ladder's rungs 1 and 2, parsed from the ladder table.
      2. The ladder itself, which every conflict is resolved against.
      3. Fail-closed: no trigger, no severity, or no term to match on.
      4. Blocking rules no past task description would surface (needs `corpus`).

    Order follows the rule list, so CORE.md reads in the same order as the
    digest and a reader can diff the two.
    """
    ladder, source = precedence_rung_slugs(rules, rungs)
    unreachable = unreachable_blocking(rules, corpus)
    core = []
    for rule in rules:
        if rule["name"] in ladder:
            reason = f"precedence rung {'/'.join(str(n) for n in rungs)}"
        elif source is not None and rule["name"] == source["name"]:
            reason = "the ladder itself"
        elif not rule["trigger"]:
            reason = "no trigger to route on"
        elif rule["severity"] not in SEVERITIES:
            reason = "no severity to route on"
        elif rule["severity"] == "blocking" and not significant_terms(rule["trigger"]):
            reason = "trigger carries no term to match"
        elif rule["name"] in unreachable:
            reason = "no past task would surface it"
        else:
            continue
        core.append({**rule, "core-reason": reason})
    return core


def estimate_tokens(chars):
    """Tokens for a byte count, using the spec's own 4-bytes-per-token reading."""
    return chars // BYTES_PER_TOKEN


def condense_body(body):
    out = []
    in_fence = False
    dropping = False  # inside a denylisted section
    prose = []
    kept_prose_in_section = False

    def flush_prose():
        nonlocal kept_prose_in_section
        if not prose:
            return
        para = " ".join(prose)
        prose.clear()
        if not dropping and not kept_prose_in_section:
            out.append(first_sentence(para))
            kept_prose_in_section = True

    for line in body:
        s = line.strip()
        if s.startswith("```") or s.startswith("~~~"):
            flush_prose()
            in_fence = not in_fence
            continue
        if in_fence or s.startswith("<!--"):
            continue

        hm = HEADING.match(s)
        if hm:
            flush_prose()
            name = norm_heading(hm.group(2))
            first_word = name.split(" ", 1)[0] if name else ""
            dropping = first_word in DENY_FIRST_WORDS
            kept_prose_in_section = False
            if not dropping:
                out.append(f"{hm.group(1)} {hm.group(2).strip()}")
            continue

        if dropping:
            continue
        if not s:
            flush_prose()
            continue
        if POINTER_LINE.match(s):
            flush_prose()
            continue
        if LIST_ITEM.match(line) or TABLE_ROW.match(line) or BOLD_DIRECTIVE.match(s):
            flush_prose()
            out.append(s)
            continue
        prose.append(s)
    flush_prose()

    cleaned = []
    for ln in out:
        if ln == "" and (not cleaned or cleaned[-1] == ""):
            continue
        cleaned.append(ln)
    while cleaned and cleaned[-1] == "":
        cleaned.pop()
    return cleaned


def rule_block(rule):
    """One rule's condensed section, identical in every artifact that shows it."""
    metastr = " — ".join(f"**{k}:** {v}" for k, v in rule["meta"])
    block = [f"## {rule['title']}", f"`{rule['path']}`"]
    if metastr:
        block.append(metastr)
    block.append("")
    block.extend(condense_body(rule["body"]))
    return "\n".join(block).rstrip()


def build_triggers(rules_dir, corpus=None):
    """TRIGGERS.md: every rule named, in one line each, always loaded."""
    rules = load_rules(rules_dir)
    core = {r["name"] for r in core_members(rules, corpus=corpus)}
    blocking = sum(1 for r in rules if r["severity"] == "blocking")
    header = [
        "# Ze Rules -- Trigger Index",
        "",
        "<!-- GENERATED by scripts/dev/rules_condensed.py -- do not edit -->",
        "<!-- Regenerate: make ze-rules-condensed-update -->",
        "",
        "Every rule under `ai/rules/`, one line each. When a trigger matches the work",
        "in hand, READ that rule's file before acting. A row marked `always-on` is",
        "already loaded in full (`ai/rules/CORE.md`) and needs no read; every other",
        "rule's body is one Read away at the path in its row.",
        "",
        f"Rules: {len(rules)} ({blocking} blocking, {len(rules) - blocking} advisory). "
        f"Always-on: {len(core)}.",
        "",
        "| Rule | Severity | When to read it |",
        "|------|----------|-----------------|",
    ]
    content = "\n".join(header + trigger_lines(rules, core)) + "\n"
    return content, len(rules)


def build_core(rules_dir, corpus=None):
    """CORE.md: the directives that must reach every session unconditionally.

    The header states no corpus SIZE. That number changed on every spec closure,
    which rewrote this generated file from commits that touched no rule and left
    `--check` red for an author with no reason to look at it. `--check` is inside
    `ze-generated-files-check`, and a structural gate is never a known-red
    (`ai/rules/git-safety.md`).

    What survives is membership: `Rules: N of M` and the reason list move only
    when the core itself moves, which is the fact a reader acts on.
    `make ze-rules-router-report` prints the corpus on demand.
    """
    rules = load_rules(rules_dir)
    core = core_members(rules, corpus=corpus)
    reasons = sorted({r["core-reason"] for r in core})
    header = [
        "# Ze Rules -- Always-On Core",
        "",
        "<!-- GENERATED by scripts/dev/rules_condensed.py -- do not edit -->",
        "<!-- Regenerate: make ze-rules-condensed-update -->",
        "",
        "The rules that apply before the shape of a task is known, so they are never",
        "reached through a trigger. Membership is DERIVED, never listed here: the",
        "ladder in `ai/rules/rule-precedence.md` names rungs 1 and 2, and rename a rule",
        "there and this file follows. A rule with no routable trigger lands here too,",
        "rather than being dropped from both artifacts, and so does a blocking rule",
        "that no past task description in `plan/` would surface",
        "(`make ze-rules-router-report` prints that set and the corpus it read).",
        "",
        "Every other rule is named in `ai/rules/TRIGGERS.md`. Read its file when its",
        "trigger matches.",
        "",
        f"Rules: {len(core)} of {len(rules)}. Reasons: {', '.join(reasons)}.",
        "",
    ]
    blocks = [
        f"{rule_block(rule)}\n\n<!-- always-on: {rule['core-reason']} -->"
        for rule in core
    ]
    content = "\n".join(header) + "\n---\n\n" + "\n\n---\n\n".join(blocks) + "\n"
    return content, len(core)


# Every artifact this generator owns: (filename, builder). `--check` walks all of
# them, so adding one here is the only edit a new artifact needs.
ARTIFACTS = (
    ("TRIGGERS.md", build_triggers),
    ("CORE.md", build_core),
)


def load_task_corpus(root):
    """Past task descriptions, the input the core's reachability test needs.

    Imported lazily: `rules_router` imports this module, so a module-level
    import would be circular.
    """
    import rules_router

    return rules_router.load_corpus(root / "plan")


def payload_report(root):
    """What a session would actually load: instructions + index + core."""
    rules_dir = root / "ai" / "rules"
    parts = [
        root / "ai" / "INSTRUCTIONS.md",
        rules_dir / "TRIGGERS.md",
        rules_dir / "CORE.md",
    ]
    sizes = [
        (p, len(p.read_text(encoding="utf-8")) if p.is_file() else 0) for p in parts
    ]
    total = sum(n for _, n in sizes)
    lines = [
        f"  {p.relative_to(root)}: {n} chars ({estimate_tokens(n)} tokens)"
        for p, n in sizes
    ]
    lines.append(f"  TOTAL: {total} chars ({estimate_tokens(total)} tokens)")
    lines.append(
        f"  budget: {TOKEN_BUDGET} tokens -- "
        f"{'MET' if estimate_tokens(total) < TOKEN_BUDGET else 'EXCEEDED'}"
    )
    headroom = 100.0 * (TOKEN_BUDGET - estimate_tokens(total)) / TOKEN_BUDGET
    lines.append(f"  headroom: {headroom:.1f}%")
    return "\n".join(lines), estimate_tokens(total), headroom


def main():
    root = Path(__file__).resolve().parents[2]
    rules_dir = root / "ai" / "rules"
    check_mode = "--check" in sys.argv

    if not rules_dir.is_dir():
        print(f"error: {rules_dir} not found", file=sys.stderr)
        sys.exit(1)

    if "--payload" in sys.argv:
        report, _, _ = payload_report(root)
        print("always-loaded payload:")
        print(report)
        return

    corpus = load_task_corpus(root)
    stale = []
    try:
        built = [(name, *builder(rules_dir, corpus)) for name, builder in ARTIFACTS]
    except LadderError as exc:
        # Refuse rather than emit artifacts whose core silently lost the ladder.
        print(f"error: {exc}", file=sys.stderr)
        sys.exit(1)

    for name, content, n in built:
        target = rules_dir / name
        if check_mode:
            current = target.read_text(encoding="utf-8") if target.exists() else ""
            if current != content:
                stale.append(target.relative_to(root))
            else:
                print(f"checked {n} rules, ai/rules/{name} up to date")
        else:
            target.write_text(content, encoding="utf-8")
            print(f"wrote {target} ({n} rules, {len(content)} chars)")

    if stale:
        for path in stale:
            print(f"WARNING: {path} is stale -- run: make ze-rules-condensed-update")
        sys.exit(1)


if __name__ == "__main__":
    main()
