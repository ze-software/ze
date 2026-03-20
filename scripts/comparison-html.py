#!/usr/bin/env python3
"""Generate colored HTML comparison page from docs/comparison.md.

Usage: scripts/comparison-html.py [input.md] [output.html]
Defaults: docs/comparison.md -> docs/comparison.html
"""

import re
import sys

CSS = """\
:root {
  --bg: #1a1b26; --surface: #24283b; --text: #c0caf5; --muted: #565f89;
  --border: #3b4261; --green: #9ece6a; --green-bg: #1a2e1a;
  --red: #f7768e; --red-bg: #2d1a1e; --yellow: #e0af68; --yellow-bg: #2d2a1a;
  --blue: #7aa2f7; --blue-bg: #1a1e2d; --purple: #bb9af7; --accent: #7dcfff;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: "Inter",-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  background: var(--bg); color: var(--text); line-height: 1.6; padding: 2rem;
  max-width: 1400px; margin: 0 auto; }
h1 { font-size: 2rem; font-weight: 700; margin-bottom: .25rem; color: var(--accent); }
.subtitle { color: var(--muted); font-size: .9rem; margin-bottom: 2rem; }
h2 { font-size: 1.3rem; font-weight: 600; margin: 2.5rem 0 .75rem 0;
  color: var(--purple); border-bottom: 1px solid var(--border); padding-bottom: .4rem; }
p.note { color: var(--muted); font-size: .85rem; margin-bottom: 1rem; font-style: italic; }
.table-wrap { overflow-x: auto; margin-bottom: 1.5rem; border-radius: 8px;
  border: 1px solid var(--border); }
table { width: 100%; border-collapse: collapse; font-size: .85rem; }
th { background: var(--surface); color: var(--accent); font-weight: 600;
  text-align: center; padding: .6rem .8rem; border-bottom: 2px solid var(--border);
  position: sticky; top: 0; white-space: nowrap; }
th:first-child { text-align: left; color: var(--text); min-width: 220px; }
th.ze { color: var(--green); background: #1e2235;
  border-left: 2px solid var(--accent); border-right: 2px solid var(--accent); }
td { padding: .45rem .8rem; border-bottom: 1px solid var(--border);
  text-align: center; white-space: nowrap; }
td:first-child { text-align: left; color: var(--text); font-weight: 500; }
tr:hover { background: rgba(125,207,255,.04); }
td.yes { color: var(--green); background: var(--green-bg); font-weight: 600; }
td.no { color: var(--red); background: var(--red-bg); }
td.partial { color: var(--yellow); background: var(--yellow-bg); }
td.api { color: var(--blue); background: var(--blue-bg); font-weight: 600; }
td.ze-col { border-left: 2px solid var(--accent); border-right: 2px solid var(--accent); }
.positioning { margin-top: 2.5rem; }
.impl-card { background: var(--surface); border: 1px solid var(--border);
  border-radius: 8px; padding: 1rem 1.2rem; margin-bottom: .75rem; }
.impl-card.ze-card { border-color: var(--accent); border-width: 2px; }
.impl-card strong { color: var(--accent); font-size: 1rem; }
.impl-card.ze-card strong { color: var(--green); }
.impl-card p { color: var(--text); font-size: .85rem; margin-top: .3rem; }
.legend { display: flex; gap: 1.5rem; flex-wrap: wrap; margin-bottom: 1.5rem; font-size: .8rem; }
.legend-item { display: flex; align-items: center; gap: .4rem; }
.legend-swatch { width: 14px; height: 14px; border-radius: 3px; display: inline-block; }
@media (max-width: 768px) { body { padding: 1rem; } h1 { font-size: 1.5rem; }
  table { font-size: .75rem; } td, th { padding: .35rem .5rem; } }
"""

LEGEND = """\
<div class="legend">
  <div class="legend-item"><span class="legend-swatch" style="background:var(--green-bg);border:1px solid var(--green)"></span> <span style="color:var(--green)">Yes</span></div>
  <div class="legend-item"><span class="legend-swatch" style="background:var(--red-bg);border:1px solid var(--red)"></span> <span style="color:var(--red)">No</span></div>
  <div class="legend-item"><span class="legend-swatch" style="background:var(--yellow-bg);border:1px solid var(--yellow)"></span> <span style="color:var(--yellow)">Partial</span></div>
  <div class="legend-item"><span class="legend-swatch" style="background:var(--blue-bg);border:1px solid var(--blue)"></span> <span style="color:var(--blue)">API</span> <span style="color:var(--muted)">(programmatic)</span></div>
</div>"""


def classify(text):
    v = text.strip().lower()
    if v == "yes":
        return "yes"
    if v == "no":
        return "no"
    if v == "api":
        return "api"
    if v in ("partial",) or any(k in v for k in ("decode", "partial", "only")) or v.startswith("rx "):
        return "partial"
    return "plain"


def inline_md(s):
    """Convert inline markdown: **bold**, [text](url), strip leading >."""
    s = re.sub(r"\*\*(.+?)\*\*", r"<strong>\1</strong>", s)
    s = re.sub(r"\[([^\]]+)\]\(([^)]+)\)", r'<a href="\2">\1</a>', s)
    return s


def bold(s):
    return inline_md(s)


def split_row(line):
    cells = line.strip().strip("|").split("|")
    return [c.strip() for c in cells]


def generate(input_path, output_path):
    with open(input_path) as f:
        lines = f.readlines()

    out = []
    w = out.append

    w("<!DOCTYPE html>")
    w('<html lang="en">')
    w("<head>")
    w('<meta charset="utf-8">')
    w('<meta name="viewport" content="width=device-width, initial-scale=1">')
    w("<title>Ze \u2014 BGP Implementation Comparison</title>")
    w(f"<style>\n{CSS}</style>")
    w("</head>")
    w("<body>")
    w("<h1>Ze \u2014 BGP Implementation Comparison</h1>")
    w('<div class="subtitle">Open-source BGP daemon feature matrix</div>')
    w(LEGEND)

    in_table = False
    in_positioning = False
    ze_col = -1
    note_lines = []
    para = ""
    i = 0

    def flush_notes():
        nonlocal note_lines
        if note_lines:
            w(f'<p class="note">{" ".join(note_lines)}</p>')
            note_lines = []

    def close_table():
        nonlocal in_table
        if in_table:
            w("</table></div>")
            in_table = False

    def flush_para():
        nonlocal para
        if para:
            is_ze = para.startswith("**Ze**")
            cls = "impl-card ze-card" if is_ze else "impl-card"
            w(f'<div class="{cls}"><p>{bold(para)}</p></div>')
            para = ""

    while i < len(lines):
        line = lines[i].rstrip("\n")
        trimmed = line.strip()
        i += 1

        # Skip title lines.
        if trimmed == "# BGP Implementation Comparison":
            continue
        if trimmed.startswith("A feature comparison") or trimmed.startswith("Last updated"):
            continue

        # Blockquotes: collect > lines into a styled box.
        if trimmed.startswith(">"):
            bq_lines = [trimmed.lstrip("> ").strip()]
            while i < len(lines):
                nxt = lines[i].rstrip("\n").strip()
                if nxt.startswith(">"):
                    bq_lines.append(nxt.lstrip("> ").strip())
                    i += 1
                else:
                    break
            text = inline_md(" ".join(bq_lines))
            w(f'<div class="impl-card"><p>{text}</p></div>')
            continue

        # Section headers.
        if trimmed.startswith("## "):
            close_table()
            flush_notes()
            flush_para()
            title = trimmed[3:]
            if title == "Positioning":
                in_positioning = True
                w(f'<div class="positioning"><h2>{title}</h2>')
            else:
                in_positioning = False
                w(f"<h2>{title}</h2>")
            continue

        # Table separator.
        if trimmed.startswith("|") and "---" in trimmed:
            continue

        # Table rows.
        if trimmed.startswith("|"):
            cells = split_row(trimmed)

            if not in_table:
                flush_notes()
                in_table = True
                ze_col = -1
                for j, h in enumerate(cells):
                    if h.lower() == "ze":
                        ze_col = j

                row = "<tr>"
                for j, h in enumerate(cells):
                    cls = ' class="ze"' if j == ze_col else ""
                    row += f"<th{cls}>{h}</th>"
                row += "</tr>"
                w('<div class="table-wrap"><table>')
                w(row)
                continue

            # Data row.
            row = "<tr>"
            for j, cell in enumerate(cells):
                if j == 0:
                    row += f"<td>{cell}</td>"
                    continue
                cls = classify(cell)
                is_ze = j == ze_col
                classes = []
                if cls != "plain":
                    classes.append(cls)
                if is_ze:
                    classes.append("ze-col")
                if classes:
                    row += f'<td class="{" ".join(classes)}">{cell}</td>'
                else:
                    row += f"<td>{cell}</td>"
            row += "</tr>"
            w(row)
            continue

        # Empty line.
        if not trimmed:
            close_table()
            flush_para()
            continue

        # Positioning text -- accumulate paragraph.
        if in_positioning and not trimmed.startswith("|"):
            if para:
                para += " " + trimmed
            else:
                para = trimmed
            continue

        # Non-table prose (notes before tables).
        if not in_table and not in_positioning:
            note_lines.append(trimmed)
            continue

    close_table()
    flush_para()
    flush_notes()
    if in_positioning:
        w("</div>")

    w("</body>")
    w("</html>")

    with open(output_path, "w") as f:
        f.write("\n".join(out) + "\n")

    print(f"wrote {output_path}")


if __name__ == "__main__":
    input_path = sys.argv[1] if len(sys.argv) > 1 else "docs/comparison.md"
    output_path = sys.argv[2] if len(sys.argv) > 2 else "docs/comparison.html"
    generate(input_path, output_path)
