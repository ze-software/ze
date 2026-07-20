#!/usr/bin/env python3
"""One-shot migration: bring every ai/rules/*.md into the canonical format.

See ai/rules/rule-format.md. Inserts the required metadata block after the H1:

    **When:** <trigger>            (kept if already present; else derived)
    **Severity:** blocking|advisory (blocking iff a **BLOCKING** marker exists)
    **Related:** slug, slug         (from existing `Extends:`/`Related:` lines)

and wraps any leading pre-`##` body content under a `## Directives` heading.
Idempotent: a file that already has a **When:**+**Severity:** block is left
untouched. After this runs, `rules_lint.py` is the durable gate.

Known limitation (one-shot, already applied): the `**When:**` and `Extends:`/
`Related:` extraction is single-physical-line. A rule whose original opener
spanned two lines -- a multi-line `Related:` reference list, or a `**BLOCKING**`
sentence continued on the next line -- yields a truncated When or a partial
Related. After running on new files, eyeball the derived `**When:**`/`**Related:**`
and hand-correct; that is what was done for fail-closed-guards / no-sprintf-alloc /
qemu-testing when this migrated the initial corpus.

Usage:
    python3 scripts/dev/rules_reformat.py            # rewrite all rule files
    python3 scripts/dev/rules_reformat.py --dry-run   # list what would change
"""

import re
import sys
from pathlib import Path

SKIP = {"INDEX.md", "CONDENSED.md", "rule-format.md"}

H1 = re.compile(r"^#\s+(.+?)\s*$")
BLOCKING_MARK = re.compile(r"^\*\*BLOCKING[:.]?\*\*\s*", re.I)
BLOCKING_ANY = re.compile(r"\*\*BLOCKING[:.]?\*\*", re.I)
WHEN_MARK = re.compile(r"^\*\*When:\*\*\s*(.*)$")
SEVERITY_MARK = re.compile(r"^\*\*Severity:\*\*")
EXTENDS_LINE = re.compile(r"^(?:Extends|Related):\s*(.*)$", re.I)
SLUGREF = re.compile(r"`?ai/rules/([a-z0-9]+(?:-[a-z0-9]+)*)\.md`?")
WS = re.compile(r"\s+")
WHEN_TRIGGER = re.compile(r"^when\b(.*?)[,:]", re.I)


def first_sentence(text, limit=180):
    text = WS.sub(" ", text).strip()
    # Prefer a sentence end ('.'); accept a ':' break only once the clause before
    # it is substantial, so we don't truncate "the registration pattern: ..." to
    # a bare fragment.
    dot = re.search(r"\.(\s|$)", text)
    colon = re.search(r":(\s|$)", text)
    end = None
    if dot and dot.start() <= limit:
        end = dot.start()
    elif colon and colon.start() >= 40 and colon.start() <= limit:
        end = colon.start()
    if end is not None:
        return text[:end].strip()
    if len(text) > limit:
        cut = text.rfind(" ", 0, limit - 1)
        return text[: cut if cut > 0 else limit - 1].rstrip()
    return text.rstrip(".")


def derive_when(body_lines):
    """Best-effort trigger phrase for a rule that lacks an explicit **When:**."""
    in_fence = False
    blocking_txt = ""
    prose_txt = ""
    for line in body_lines:
        s = line.strip()
        if s.startswith("```") or s.startswith("~~~"):
            in_fence = not in_fence
            continue
        if in_fence or not s or s.startswith(("#", "|", ">", "<!--")):
            continue
        bm = BLOCKING_MARK.match(s)
        if bm and not blocking_txt:
            blocking_txt = s[bm.end() :].strip()
        if not prose_txt and not s.startswith(("**", "-", "*", "[")):
            if not re.match(r"^(Rationale|Principle|Extends|Related|See)\b", s, re.I):
                prose_txt = s
    chosen = blocking_txt or prose_txt
    if not chosen:
        return ""
    # If the sentence opens with a "When ... ," trigger clause, prefer that.
    tm = WHEN_TRIGGER.match(chosen)
    if tm:
        return "when " + tm.group(1).strip()
    return first_sentence(chosen)


def migrate(path, title_fallback):
    raw = path.read_text(encoding="utf-8", errors="replace")
    lines = raw.splitlines()

    idx = 0
    while idx < len(lines) and not lines[idx].strip():
        idx += 1
    m = H1.match(lines[idx].strip()) if idx < len(lines) else None
    if not m:
        return None, "no H1 title"
    title = m.group(1)
    body = lines[idx + 1 :]

    # Already migrated?
    probe = [l for l in body if l.strip()][:4]
    if any(WHEN_MARK.match(l.strip()) for l in probe) and any(
        SEVERITY_MARK.match(l.strip()) for l in probe
    ):
        return None, "already conforms"

    severity = "blocking" if BLOCKING_ANY.search(raw) else "advisory"

    when = ""
    related = []
    kept = []
    i = 0
    while i < len(body):
        line = body[i]
        s = line.strip()
        wm = WHEN_MARK.match(s)
        if wm and not when:
            # A **When:** value may wrap across lines; absorb plain continuation
            # lines until a blank, a new **Key:** line, or a heading.
            parts = [wm.group(1).strip()]
            i += 1
            while i < len(body):
                nxt = body[i].strip()
                if not nxt or nxt.startswith(("**", "#", "|", ">", "-", "<!--")):
                    break
                parts.append(nxt)
                i += 1
            when = WS.sub(" ", " ".join(p for p in parts if p)).strip()
            continue  # consumed into the metadata block
        em = EXTENDS_LINE.match(s)
        if em:
            related += SLUGREF.findall(em.group(1))
            i += 1
            continue  # folded into **Related:**
        # Severity now lives in the metadata block; strip the redundant
        # **BLOCKING:** marker from body lines, keeping the directive text.
        bm = BLOCKING_MARK.match(s)
        if bm:
            kept.append(line[: len(line) - len(s)] + s[bm.end() :])
            i += 1
            continue
        kept.append(line)
        i += 1

    if not when:
        when = derive_when(kept) or f"working on {title.lower()}"

    # de-dup related, drop self-reference
    seen = set()
    related = [r for r in related if not (r in seen or seen.add(r)) and r != path.stem]

    # Strip leading blank lines of the kept body.
    while kept and not kept[0].strip():
        kept.pop(0)

    # Wrap leading pre-`##` content under a `## Directives` heading.
    if kept and not kept[0].lstrip().startswith("##"):
        lead_end = len(kept)
        for i, line in enumerate(kept):
            if line.lstrip().startswith("## "):
                lead_end = i
                break
        lead = kept[:lead_end]
        if any(l.strip() for l in lead):
            kept = ["## Directives", ""] + lead + kept[lead_end:]

    meta = [f"**When:** {when}", f"**Severity:** {severity}"]
    if related:
        meta.append(f"**Related:** {', '.join(related)}")

    out = [f"# {title}", ""] + meta + [""] + kept
    # collapse trailing blanks
    while out and not out[-1].strip():
        out.pop()
    return "\n".join(out) + "\n", "migrated"


def main():
    root = Path(__file__).resolve().parents[2]
    rules_dir = root / "ai" / "rules"
    dry = "--dry-run" in sys.argv

    changed = skipped = 0
    for md in sorted(rules_dir.glob("*.md")):
        if md.name in SKIP:
            continue
        content, status = migrate(md, md.stem)
        if content is None:
            skipped += 1
            continue
        changed += 1
        if dry:
            print(f"WOULD migrate ai/rules/{md.name}")
        else:
            md.write_text(content, encoding="utf-8")
            print(f"migrated ai/rules/{md.name}")

    print(f"\n{changed} migrated, {skipped} already conform / skipped")


if __name__ == "__main__":
    main()
