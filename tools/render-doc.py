#!/usr/bin/env -S uv run --with markdown python3
"""Render a markdown file into a site-shell-wrapped HTML page.

Usage:
    tools/render-doc.py <source.md> <dest/index.html> [--root ../]

The destination is wrapped in the same header/nav/footer markup as every
other gh-pages page (via tools/sitelib.py) and linked against
assets/site.css. Re-run this after editing the source markdown to refresh
the published page -- same workflow as presentations/tools/bundle-html.py
for presentation content.
"""

import argparse
import json
import pathlib
import posixpath
import re
import sys

import markdown

import sitelib


def first_h1(md_text):
    match = re.search(r"^#\s+(.+)$", md_text, re.MULTILINE)
    return match.group(1).strip() if match else "Ze"


TD_RE = re.compile(r"<td([^>]*)>((?:(?!</td>).)*)</td>", re.S)
TAG_RE = re.compile(r"<[^>]+>")
CELL_CLASSES = (
    (re.compile(r"^yes\b", re.I), "cell-yes"),
    (re.compile(r"^no\b", re.I), "cell-no"),
    (re.compile(r"^partial\b", re.I), "cell-partial"),
    (re.compile(r"^n/a$", re.I), "cell-na"),
)
CELL_SYMBOLS = {
    "cell-yes": "✓",
    "cell-no": "✕",
    "cell-partial": "∿",
}


def colorcode_cells(body_html):
    """Tag Yes/No/Partial/N-A table cells with classes so site.css can
    color-code them for scanning, and collapse Yes/No/Partial to just their
    symbol -- the color and the icon already carry the meaning, the word
    only adds width."""

    def repl(m):
        attrs, inner = m.group(1), m.group(2)
        if "class=" in attrs:
            return m.group(0)
        text = TAG_RE.sub("", inner).strip()
        for pattern, cls in CELL_CLASSES:
            if pattern.match(text):
                symbol = CELL_SYMBOLS.get(cls)
                content = symbol if symbol else inner
                return '<td%s class="%s">%s</td>' % (attrs, cls, content)
        return m.group(0)

    return TD_RE.sub(repl, body_html)


HREF_RE = re.compile(r'href="([^"]*)"')


def rewrite_doc_links(body_html, doc_rel, manifest, dest_rel_dir):
    """Rewrite relative .md links: local site path if the target is also
    published (per manifest), GitHub fallback otherwise. doc_rel is the
    source's path relative to docs/ (e.g. "features/configuration.md").
    dest_rel_dir is this page's own output directory relative to the site
    root (e.g. "docs/features/configuration"), used to compute the local
    relative path back."""
    source_dir = posixpath.dirname(doc_rel)

    def repl(m):
        href = m.group(1)
        if href.startswith(("http://", "https://", "mailto:", "#")):
            return m.group(0)
        path_part, sep, fragment = href.partition("#")
        if not path_part:
            return m.group(0)
        target_doc_rel = posixpath.normpath(posixpath.join(source_dir, path_part))
        if path_part.endswith("/"):
            new_href = (
                "https://github.com/ze-software/ze/tree/main/docs/%s" % target_doc_rel
            )
            return 'href="%s" target="_blank" rel="noopener"' % new_href
        if target_doc_rel in manifest:
            target_dest_dir = manifest[target_doc_rel]
            rel = posixpath.relpath(target_dest_dir, dest_rel_dir) + "/"
            new_href = rel + (("#" + fragment) if sep else "")
            return 'href="%s"' % new_href
        new_href = (
            "https://github.com/ze-software/ze/blob/main/docs/%s" % target_doc_rel
        )
        if sep:
            new_href += "#" + fragment
        return 'href="%s" target="_blank" rel="noopener"' % new_href

    return HREF_RE.sub(repl, body_html)


def render(
    source, dest, root, desc, manifest=None, doc_rel=None, dest_rel_dir=None, cat=None
):
    md_text = source.read_text()
    title = first_h1(md_text)
    body_html = markdown.markdown(
        md_text, extensions=["tables", "fenced_code", "sane_lists"]
    )
    body_html = colorcode_cells(body_html)
    if manifest is not None:
        body_html = rewrite_doc_links(body_html, doc_rel, manifest, dest_rel_dir)
    section_class = "md-content reveal cat-%s" % cat if cat else "md-content reveal"
    full_title = "%s - Ze" % title
    head = sitelib.page_head(full_title, desc, root, og_title=full_title, og_desc=desc)
    head += '            <section class="%s">\n' % section_class
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(
        head + body_html + "\n            </section>\n" + sitelib.page_foot(root)
    )
    print("rendered %s -> %s" % (source, dest))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=pathlib.Path)
    parser.add_argument("dest", type=pathlib.Path)
    parser.add_argument("--root", default="../", help="relative path back to site root")
    parser.add_argument("--desc", default="Ze documentation.", help="meta description")
    parser.add_argument(
        "--manifest",
        type=pathlib.Path,
        help="JSON map of docs-relative .md path -> published dest dir, "
        "for rewriting cross-doc links (local if published, GitHub otherwise)",
    )
    parser.add_argument(
        "--doc-rel",
        help="source path relative to docs/, e.g. features/configuration.md",
    )
    parser.add_argument(
        "--dest-rel-dir",
        help="this page's own output directory relative to the site root, "
        "e.g. docs/features/configuration",
    )
    parser.add_argument(
        "--cat",
        choices=[
            "operate",
            "routing",
            "automate",
            "observe",
            "secure",
            "services",
            "platform",
        ],
        help="topic category, colors the h1 per the site's color convention "
        "(same seven hues as the Features category legend)",
    )
    args = parser.parse_args()

    if not args.source.exists():
        print("error: source not found: %s" % args.source, file=sys.stderr)
        return 1

    manifest = json.loads(args.manifest.read_text()) if args.manifest else None
    render(
        args.source,
        args.dest,
        args.root,
        args.desc,
        manifest=manifest,
        doc_rel=args.doc_rel,
        dest_rel_dir=args.dest_rel_dir,
        cat=args.cat,
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
