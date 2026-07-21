#!/usr/bin/env python3
"""Build the published stylesheet from assets/css source files.

assets/site.css is the GitHub Pages asset every page links. The editable source
lives under assets/css/: 10-base.css carries the legacy bulk stylesheet, while
smaller later files hold extracted tokens, shared components, and responsive
rules. New CSS should move into those smaller files instead of growing the bulk
file.

The published file is minified (rcssmin): comments and whitespace are stripped
so the byte the browser downloads is as small as the source allows. The source
files under assets/css/ stay human-readable; only this generated output is
minified, the same split the JS build uses (assets/js/ -> assets/site.js).
"""

import pathlib
import re

import rcssmin

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
SOURCE = GH_PAGES / "assets" / "css" / "site.css"
DEST = GH_PAGES / "assets" / "site.css"
IMPORT_RE = re.compile(r'^\s*@import\s+url\(["\']?([^"\')]+)["\']?\);\s*$', re.MULTILINE)


def expand(path, seen=None):
    if seen is None:
        seen = set()
    path = path.resolve()
    if path in seen:
        raise ValueError("recursive CSS import: %s" % path)
    seen.add(path)
    text = path.read_text()
    parts = []
    pos = 0
    for match in IMPORT_RE.finditer(text):
        parts.append(text[pos : match.start()])
        child = (path.parent / match.group(1)).resolve()
        if not child.is_relative_to(GH_PAGES):
            raise ValueError("CSS import leaves gh-pages: %s" % child)
        parts.append(expand(child, seen.copy()))
        pos = match.end()
    parts.append(text[pos:])
    body = "".join(parts).strip()
    rel = path.relative_to(GH_PAGES)
    return "/* %s */\n%s\n" % (rel, body)


def main():
    css = expand(SOURCE).strip() + "\n"
    minified = rcssmin.cssmin(css)
    DEST.write_text(minified)
    print(
        "rendered %s -> %s (%d bytes, minified from %d)"
        % (SOURCE, DEST, len(minified.encode()), len(css.encode()))
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
