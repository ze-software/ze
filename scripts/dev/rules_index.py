#!/usr/bin/env python3
"""Generate ai/rules/INDEX.md: a one-line overview of every ai/rules/*.md rule.

Gives agents a complete, greppable map of which rule covers which topic, so they
know when to open a rule in full instead of discovering it by accident. Derived
from each rule file's own heading and summary line -- never hand-edited.

Per rule, the "When to read" summary is taken in priority order from:
  1. an explicit `**When:** ...` line, else
  2. the `**BLOCKING:** ...` (or `**BLOCKING.**`) directive line, else
  3. the first plain prose paragraph after the title.

A rule whose summary cannot be derived must add a `**When:**` line; `--check`
fails until it does, so a new rule cannot land without a discoverable trigger.

Usage:
    python3 scripts/dev/rules_index.py          # regenerate ai/rules/INDEX.md
    python3 scripts/dev/rules_index.py --check   # exit 1 if INDEX.md is stale
"""

import re
import sys
from pathlib import Path


MARKER_STRIP = re.compile(r"^\*\*[A-Za-z]+[:.]?\*\*\s*")
WS = re.compile(r"\s+")
# A bare cross-reference line ("Rationale: ...", "Principle: ...") ends the
# preceding paragraph and is never summary material itself.
POINTER_LINE = re.compile(
    r"^(Rationale|Principle|Structural template)\b", re.IGNORECASE
)
MAX_SUMMARY = 200


def paragraphs(raw_lines):
    """Group lines into paragraphs, dropping headings, code fences, and pointers."""
    paras = []
    cur = []
    in_fence = False

    def flush():
        if cur:
            paras.append(" ".join(cur))
            cur.clear()

    for line in raw_lines:
        stripped = line.strip()
        if stripped.startswith("```"):
            in_fence = not in_fence
            flush()
            continue
        if in_fence:
            continue
        if stripped.startswith("#") or not stripped or POINTER_LINE.match(stripped):
            flush()
            continue
        cur.append(stripped)
    flush()
    return paras


def title_of(raw_lines, fallback):
    for line in raw_lines:
        m = re.match(r"#\s+(.+)", line.strip())
        if m:
            return m.group(1).strip()
    return fallback


def _strip_bold(text):
    """Drop bold markers outside code spans.

    A blanket replace corrupts globs: the trigger of testing.md
    carries `test/**/*.ci`, which rendered as `test//*.ci` in the index until the
    code spans were protected.
    """
    out = []
    for i, part in enumerate(re.split(r"(`[^`]*`)", text)):
        out.append(part if i % 2 else part.replace("**", ""))
    return "".join(out)


def clean(text):
    text = MARKER_STRIP.sub("", text)
    text = _strip_bold(text)
    text = WS.sub(" ", text).strip()
    text = text.replace("|", r"\|")
    if len(text) > MAX_SUMMARY:
        cut = text.rfind(" ", 0, MAX_SUMMARY - 1)
        if cut <= 0:
            cut = MAX_SUMMARY - 1
        text = text[:cut].rstrip() + "..."
    return text


def is_prose(para):
    low = para.lower()
    if para.startswith(("**", "|", ">", "-", "*", "[", "<!--")):
        return False
    if low.startswith(("rationale", "principle", "structural template", "see ")):
        return False
    return True


def meta_of(raw_lines):
    """Return the rule's metadata block as a dict.

    The block is contiguous by construction (ai/rules/rule-format.md), so a
    paragraph-level match would swallow Severity and Related into the When text.
    That is exactly what happened before this function existed: every index row
    read "<trigger> Severity: blocking Related: ...".
    """
    meta = {}
    seen_title = False
    for line in raw_lines:
        stripped = line.strip()
        if not seen_title:
            seen_title = stripped.startswith("# ")
            continue
        if not stripped:
            if meta:
                break  # block ended
            continue
        m = re.match(r"^\*\*(When|Severity|Related):\*\*\s*(.*)$", stripped)
        if not m:
            break
        meta[m.group(1)] = m.group(2).strip()
    return meta


def summarise(raw_lines):
    """Return the When-to-read summary for a rule, or '' if none can be derived.

    `**When:**` (an explicit trigger) wins, then a `**BLOCKING:**` directive, then
    the first plain prose paragraph. A bold heading like `**When to use ...**` is
    not a trigger, so the When branch matches the exact `**When:**` marker only.
    """
    meta = meta_of(raw_lines)
    if meta.get("When"):
        return clean(meta["When"])
    blocking = prose = ""
    for para in paragraphs(raw_lines):
        if not blocking and para.startswith("**BLOCKING"):
            blocking = para
        if not prose and is_prose(para):
            prose = para
    chosen = blocking or prose
    return clean(chosen) if chosen else ""


def build(rules_dir):
    """Return (content, missing) where missing lists rules without a summary."""
    rows = []
    missing = []
    for md in sorted(rules_dir.glob("*.md")):
        # A generated aggregate is not a rule. They are recognised by SHAPE --
        # an all-caps stem, the repo's convention for INDEX.md / TRIGGERS.md /
        # TRIGGERS.md / CORE.md -- so the next artifact needs no edit here. The
        # two-name list this replaces made TRIGGERS.md and CORE.md land in the
        # index as malformed rules on the day they were generated.
        if md.stem.isupper():
            continue
        raw = md.read_text(encoding="utf-8", errors="replace").splitlines()
        title = title_of(raw, md.stem)
        summary = summarise(raw)
        if not summary:
            missing.append(md.name)
            summary = "_(no summary -- add a `**When:**` line)_"
        severity = meta_of(raw).get("Severity", "-")
        rows.append((md.name, title, summary, severity))

    lines = [
        "# Ze Rules Index",
        "",
        "<!-- GENERATED by scripts/dev/rules_index.py -- do not edit -->",
        "<!-- Regenerate: make ze-rules-index -->",
        "",
        "One-line overview of every rule under `ai/rules/`. Read the listed file in",
        "full before acting on a topic it covers.",
        "",
        f"Total: {len(rows)} rules",
        "",
        "| Rule | When to read | Severity | File |",
        "|------|--------------|----------|------|",
    ]
    for name, title, summary, severity in rows:
        lines.append(f"| {title} | {summary} | {severity} | `ai/rules/{name}` |")
    lines.append("")
    return "\n".join(lines), missing


def main():
    root = Path(__file__).resolve().parents[2]
    rules_dir = root / "ai" / "rules"
    output_file = rules_dir / "INDEX.md"
    check_mode = "--check" in sys.argv

    if not rules_dir.is_dir():
        print(f"error: {rules_dir} not found", file=sys.stderr)
        sys.exit(1)

    content, missing = build(rules_dir)
    n_rules = content.count("\n| ") - 1  # rows minus the header separator line

    if check_mode:
        problems = []
        if missing:
            problems.append(
                "rules missing a derivable summary (add a `**When:**` line): "
                + ", ".join(missing)
            )
        current = (
            output_file.read_text(encoding="utf-8") if output_file.exists() else ""
        )
        if current != content:
            problems.append(
                f"{output_file.relative_to(root)} is stale -- run: make ze-rules-index"
            )
        if problems:
            for p in problems:
                print(f"WARNING: {p}")
            sys.exit(1)
        print(f"checked {n_rules} rules, ai/rules/INDEX.md up to date")
    else:
        output_file.write_text(content, encoding="utf-8")
        print(f"wrote {output_file} ({n_rules} rules)")
        if missing:
            print(
                "WARNING: "
                + f"{len(missing)} rule(s) lack a derivable summary -- add a "
                + "`**When:**` line to: "
                + ", ".join(missing)
            )


if __name__ == "__main__":
    main()
