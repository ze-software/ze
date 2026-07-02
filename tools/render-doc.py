#!/usr/bin/env -S uv run --with markdown python3
"""Render a markdown file into a site-shell-wrapped HTML page.

Usage:
    tools/render-doc.py <source.md> <dest/index.html> [--root ../]

The destination is wrapped in the same header/nav/footer markup as every
other gh-pages page and linked against assets/site.css. Re-run this after
editing the source markdown to refresh the published page -- same workflow
as presentations/tools/bundle-html.py for presentation content.
"""

import argparse
import json
import pathlib
import posixpath
import re
import sys

import markdown

NAV_LINKS = [
    ("Status", "{root}index.html#status"),
    ("Try", "{root}index.html#try"),
    ("Features", "{root}features/"),
    ("Activity", "{root}activity/"),
    ("Performance", "{root}performance/"),
    ("Labs", "{root}labs/"),
    ("Compare", "{root}compare/"),
    ("Blog", "{root}blog/"),
    ("Talks", "{root}talks/"),
]

HEAD = """<!doctype html>
<html lang="en">
    <head>
        <meta charset="utf-8" />
        <meta name="viewport" content="width=device-width, initial-scale=1" />
        <title>{title} - Ze</title>
        <meta name="description" content="{desc}" />
        <meta property="og:title" content="{title} - Ze" />
        <meta property="og:description" content="{desc}" />
        <meta property="og:type" content="website" />
        <link rel="icon" href="{root}assets/ze.svg" type="image/svg+xml" />
        <link rel="preconnect" href="https://fonts.googleapis.com" />
        <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
        <link
            href="https://fonts.googleapis.com/css2?family=Poppins:wght@300;400;600;700;800&family=Lato:wght@300;400;700&display=swap"
            rel="stylesheet"
        />
        <link rel="stylesheet" href="{root}assets/site.css" />
    </head>
    <body>
        <header class="site-header">
            <nav class="nav" aria-label="Main navigation">
                <a class="brand" href="{root}index.html#top" aria-label="Ze home">
                    <img src="{root}assets/ze.svg" alt="" width="32" height="32" />
                    <span>Ze</span>
                </a>
                <div class="nav-links">
{navlinks}
                    <a
                        href="https://github.com/ze-software/ze"
                        target="_blank"
                        rel="noopener"
                        >GitHub</a
                    >
                </div>
            </nav>
        </header>

        <main id="top">
            <section class="md-content reveal">
"""

FOOT = """            </section>
        </main>

        <script>
            document.addEventListener("DOMContentLoaded", function () {{
                var observer = new IntersectionObserver(
                    function (entries) {{
                        entries.forEach(function (entry) {{
                            if (entry.isIntersecting) {{
                                entry.target.classList.add("visible");
                                observer.unobserve(entry.target);
                            }}
                        }});
                    }},
                    {{ threshold: 0.01 }},
                );
                document.querySelectorAll(".reveal").forEach(function (el) {{
                    observer.observe(el);
                }});
            }});
        </script>

        <footer>
            <div class="footer-inner">
                <span>Ze is AGPLv3 open source.</span>
                <div class="footer-links">
                    <a
                        href="https://github.com/ze-software/ze"
                        target="_blank"
                        rel="noopener"
                        >GitHub</a
                    >
                    <a
                        href="https://codeberg.org/thomas-mangin/ze"
                        target="_blank"
                        rel="noopener"
                        >Codeberg</a
                    >
                    <a
                        href="https://github.com/ze-software/ze/issues"
                        target="_blank"
                        rel="noopener"
                        >Issues</a
                    >
                    <a
                        href="https://discord.gg/3Sx4S2dYQ"
                        target="_blank"
                        rel="noopener"
                        >Discord</a
                    >
                    <a href="{root}style-guide/">Style Guide</a>
                </div>
            </div>
        </footer>
    </body>
</html>
"""


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


def colorcode_cells(body_html):
    """Tag Yes/No/Partial/N-A table cells with classes so site.css can
    color-code them for scanning."""

    def repl(m):
        attrs, inner = m.group(1), m.group(2)
        if "class=" in attrs:
            return m.group(0)
        text = TAG_RE.sub("", inner).strip()
        for pattern, cls in CELL_CLASSES:
            if pattern.match(text):
                return '<td%s class="%s">%s</td>' % (attrs, cls, inner)
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
    navlinks = "\n".join(
        '                    <a href="%s">%s</a>' % (href.format(root=root), label)
        for label, href in NAV_LINKS
    )
    section_class = "md-content reveal cat-%s" % cat if cat else "md-content reveal"
    head = HEAD.format(title=title, desc=desc, root=root, navlinks=navlinks)
    head = head.replace(
        '<section class="md-content reveal">',
        '<section class="%s">' % section_class,
    )
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(head + body_html + "\n" + FOOT.format(root=root))
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
