#!/usr/bin/env python3
"""Generate ai/rules/CONDENSED.md: the actionable core of every ai/rules/*.md rule.

Unlike INDEX.md (one line per rule), CONDENSED.md keeps each rule's directives so
it can be imported eagerly into every session (via `@ai/rules/CONDENSED.md` in
ai/INSTRUCTIONS.md) -- a fresh session then sees every rule's directives without
opening 89 files. The full rule file is still one Read away for nuance.

It relies on the canonical rule format (ai/rules/rule-format.md): a `**When:**` /
`**Severity:**` metadata block, imperatives under directive sections, and rationale
/ examples under denylisted headings. Extraction is therefore deterministic:

  KEEP  the metadata block, and every non-denylisted section's headings, list
        items, table rows, and bold-directive lines (plus the first sentence of
        each prose paragraph -- the rule statement).
  DROP  denylisted sections (Rationale, Why, Examples, ...), fenced code blocks,
        HTML comments, and `Rationale:`/`See:` pointer lines.

Derived mechanically -- never hand-edited. Kept fresh by `make ze-rules-condensed`;
`--check` fails when stale.

Usage:
    python3 scripts/dev/rules_condensed.py           # regenerate ai/rules/CONDENSED.md
    python3 scripts/dev/rules_condensed.py --check    # exit 1 if CONDENSED.md is stale
"""

import re
import sys
from pathlib import Path

SKIP = {"INDEX.md", "CONDENSED.md"}

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


def build(rules_dir):
    blocks = []
    for md in sorted(rules_dir.glob("*.md")):
        if md.name in SKIP:
            continue
        raw = md.read_text(encoding="utf-8", errors="replace").splitlines()
        title, meta, body = parse(raw)
        title = title or md.stem
        metastr = " — ".join(f"**{k}:** {v}" for k, v in meta)
        block = [f"## {title}", f"`ai/rules/{md.name}`"]
        if metastr:
            block.append(metastr)
        block.append("")
        block.extend(condense_body(body))
        blocks.append("\n".join(block).rstrip())

    header = [
        "# Ze Rules -- Condensed",
        "",
        "<!-- GENERATED by scripts/dev/rules_condensed.py -- do not edit -->",
        "<!-- Regenerate: make ze-rules-condensed -->",
        "",
        "The actionable core of every rule under `ai/rules/`, condensed for eager",
        "loading into every session. Directives are kept; rationale and examples are",
        "dropped. When a rule governs your current action, open its full file (the",
        "path under each heading) before acting -- this digest maps directives, it is",
        "not a substitute for the rule.",
        "",
        f"Rules: {len(blocks)}",
        "",
    ]
    content = "\n".join(header) + "\n---\n\n" + "\n\n---\n\n".join(blocks) + "\n"
    return content, len(blocks)


def main():
    root = Path(__file__).resolve().parents[2]
    rules_dir = root / "ai" / "rules"
    output_file = rules_dir / "CONDENSED.md"
    check_mode = "--check" in sys.argv

    if not rules_dir.is_dir():
        print(f"error: {rules_dir} not found", file=sys.stderr)
        sys.exit(1)

    content, n = build(rules_dir)

    if check_mode:
        current = (
            output_file.read_text(encoding="utf-8") if output_file.exists() else ""
        )
        if current != content:
            print(
                f"WARNING: {output_file.relative_to(root)} is stale -- "
                "run: make ze-rules-condensed"
            )
            sys.exit(1)
        print(f"checked {n} rules, ai/rules/CONDENSED.md up to date")
    else:
        output_file.write_text(content, encoding="utf-8")
        print(f"wrote {output_file} ({n} rules, {len(content)} chars)")


if __name__ == "__main__":
    main()
