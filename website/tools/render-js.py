#!/usr/bin/env -S uv run --with rjsmin python3
"""Build the published script from the assets/js source file.

assets/site.js is the GitHub Pages asset every page links. The editable source
lives at assets/js/site.js; this step minifies it (rjsmin) into the published
assets/site.js so the browser downloads the smallest form while the source stays
human-readable. This mirrors the CSS build (assets/css/ -> assets/site.css).

rjsmin is a conservative whitespace/comment minifier: it does not rename symbols
or reorder code, so the behaviour of the published script is identical to the
source.
"""

import pathlib

import rjsmin

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
SOURCE = GH_PAGES / "assets" / "js" / "site.js"
DEST = GH_PAGES / "assets" / "site.js"


def main():
    js = SOURCE.read_text()
    minified = rjsmin.jsmin(js)
    if not minified.endswith("\n"):
        minified += "\n"
    DEST.write_text(minified)
    print(
        "rendered %s -> %s (%d bytes, minified from %d)"
        % (SOURCE, DEST, len(minified.encode()), len(js.encode()))
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
