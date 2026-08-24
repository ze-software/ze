#!/usr/bin/env -S uv run --with markdown python3
"""Render Markdown into a site-shell-wrapped HTML page.

Usage:
    tools/render-doc.py <source.md> [<dest/index.html>] [--root ../]

Front matter may provide ``destination`` and the page presentation metadata,
which makes the source self-contained and leaves this as the single renderer
for ordinary Markdown pages. Explicit command-line values remain available to
the existing batch builders and take precedence over front matter.
"""

import argparse
import html
import json
import pathlib
import posixpath
import re
import shutil
import sys

import markdown

import sitelib
import terminal_demos


def first_h1(md_text):
    match = re.search(r"^#\s+(.+)$", md_text, re.MULTILINE)
    return match.group(1).strip() if match else "Ze"


_IMG_REF_RE = re.compile(
    r"!\[[^\]]*\]\(\s*([^)\s]+)"  # ![alt](path ...)
    r"|<img\b[^>]*?\bsrc=[\"']([^\"']+)[\"']"  # <img ... src="path">
)


def copy_referenced_images(source, dest, md_text):
    """Copy the local images a page references from beside its Markdown source
    to beside its published HTML, preserving the relative path.

    Doc pages live one directory deeper once published (foo.md ->
    foo/index.html), so an ``img/x.png`` reference resolves against the source
    dir on one side and the page's own dir on the other; copying keeps the
    relative link valid. Remote (http/data) and rooted/parent paths are left
    alone -- only same-tree relative assets are published."""
    copied = 0
    for match in _IMG_REF_RE.finditer(md_text):
        ref = match.group(1) or match.group(2)
        if not ref or "://" in ref or ref.startswith(("/", "#", "data:", "..")):
            continue
        src_img = (source.parent / ref).resolve()
        if not src_img.is_file():
            sitelib.warn(
                "%s references image %r that does not exist at %s"
                % (source.name, ref, src_img)
            )
            continue
        dst_img = dest.parent / ref
        dst_img.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(src_img, dst_img)
        copied += 1
    return copied


FRONT_MATTER_RE = re.compile(r"\A---[ \t]*\r?\n(.*?)\r?\n---[ \t]*(?:\r?\n|$)", re.S)
PAGE_CATEGORIES = {
    "operate",
    "routing",
    "automate",
    "observe",
    "secure",
    "services",
    "platform",
}
TRUE_VALUES = {"1", "true", "yes", "on"}
FALSE_VALUES = {"0", "false", "no", "off"}


def parse_front_matter(md_text):
    """Return scalar page metadata and the Markdown body."""
    match = FRONT_MATTER_RE.match(md_text)
    if not match:
        return {}, md_text

    metadata = {}
    for line_number, line in enumerate(match.group(1).splitlines(), start=2):
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue
        if ":" not in line:
            raise ValueError("front matter line %d must be `key: value`" % line_number)
        key, _, value = line.partition(":")
        key = key.strip().lower()
        value = value.strip()
        if not key:
            raise ValueError("front matter line %d has an empty key" % line_number)
        if key in metadata:
            raise ValueError("duplicate front matter key: %s" % key)
        if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
            value = value[1:-1]
        metadata[key] = value
    return metadata, md_text[match.end() :]


def metadata_bool(metadata, key, default):
    if key not in metadata:
        return default
    value = metadata[key].lower()
    if value in TRUE_VALUES:
        return True
    if value in FALSE_VALUES:
        return False
    raise ValueError(
        "front matter `%s` must be one of: true, false, yes, no, on, off, 1, 0" % key
    )


def page_destination(value):
    """Resolve a front-matter destination relative to the site root."""
    rel = pathlib.PurePosixPath(value.strip().lstrip("/"))
    if ".." in rel.parts:
        raise ValueError("front matter `destination` must stay inside the site root")
    if rel.suffix != ".html":
        rel = rel / "index.html"
    if rel.name != "index.html":
        raise ValueError(
            "front matter `destination` must end in index.html or a directory"
        )
    return pathlib.Path(__file__).resolve().parent.parent / pathlib.Path(*rel.parts)


def page_root(dest):
    site_root = pathlib.Path(__file__).resolve().parent.parent
    try:
        rel = dest.resolve().relative_to(site_root.resolve())
    except ValueError as exc:
        raise ValueError("destination must stay inside the gh-pages site root") from exc
    if rel.name != "index.html":
        raise ValueError("destination must end in index.html")
    return "../" * (len(rel.parts) - 1)


FIRST_H1_P_RE = re.compile(
    r"\A((?:<!--.*?-->\s*)*)<h1([^>]*)>(.*?)</h1>\n"
    r"((?:<!--.*?-->\s*)*)<p>(.*?)</p>",
    re.S,
)
FIRST_H1_RE = re.compile(r"\A((?:<!--.*?-->\s*)*)<h1([^>]*)>(.*?)</h1>", re.S)


def default_journey_label(dest, doc_rel=None):
    key = sitelib.page_key_for_path(dest).strip("/")
    if doc_rel:
        doc_area = doc_rel.split("/", 1)[0].removesuffix(".md")
        return {
            "architecture": "Architecture",
            "features": "Feature",
            "guide": "Guide",
            "performance": "Performance",
            "research": "Research",
        }.get(doc_area, "Documentation")
    if key.startswith("compare/"):
        return "Compare"
    if key.startswith("contribute/"):
        return "Community"
    if key.startswith("docs/"):
        parts = key.split("/")
        area = parts[1] if len(parts) > 1 else ""
        return {
            "architecture": "Architecture",
            "features": "Feature",
            "guide": "Guide",
            "performance": "Performance",
            "research": "Research",
        }.get(area, "Documentation")
    if key.startswith("quality/"):
        return "Quality"
    return {
        "code-of-conduct": "Community",
        "faq": "FAQ",
        "license": "License",
        "roadmap": "Release path",
        "security": "Security",
    }.get(key, key.replace("-", " ").title() if key else "Ze")


def wrap_journey_hero(body_html, label):
    def repl(match):
        leading_comments, h1_attrs, title, comments, intro = match.groups()
        hero = sitelib.page_hero(
            title,
            intro,
            label,
            h1_attrs=h1_attrs,
            title_html=True,
            lead_html=True,
            indent="",
        )
        return (leading_comments or "") + (comments or "") + hero

    wrapped, count = FIRST_H1_P_RE.subn(repl, body_html, count=1)
    if count:
        return wrapped

    def h1_only_repl(match):
        leading_comments, h1_attrs, title = match.groups()
        hero = sitelib.page_hero(
            title,
            None,
            label,
            h1_attrs=h1_attrs,
            title_html=True,
            indent="",
        )
        return (leading_comments or "") + hero

    return FIRST_H1_RE.sub(h1_only_repl, body_html, count=1)


def _toc_label(text):
    return re.sub(r"<[^>]+>", "", text or "").strip()


def _toc_items(tokens):
    items = []
    for token in tokens or []:
        children = _toc_items(token.get("children", []))
        if token.get("level", 0) < 2:
            items.extend(children)
            continue
        label = _toc_label(token.get("name", ""))
        anchor = token.get("id", "")
        if not label or not anchor:
            items.extend(children)
            continue
        child_html = ""
        if children:
            child_html = "\n<ol>\n%s\n</ol>" % "\n".join(children)
        items.append(
            '<li><a href="#%s">%s</a>%s</li>'
            % (html.escape(anchor, quote=True), html.escape(label), child_html)
        )
    return items


def render_doc_toc(tokens):
    items = _toc_items(tokens)
    if not items:
        return ""
    return (
        '<nav class="doc-toc" aria-labelledby="doc-toc-title">'
        '<h2 id="doc-toc-title">On this page</h2>'
        "<ol>\n%s\n</ol></nav>" % "\n".join(items)
    )


def insert_doc_toc(body_html, toc_html):
    if not toc_html:
        return body_html
    marker = "</div>"
    pos = body_html.find(marker)
    if pos == -1:
        return toc_html + "\n" + body_html
    end = pos + len(marker)
    return body_html[:end] + "\n" + toc_html + body_html[end:]


TD_RE = re.compile(r"<td([^>]*)>((?:(?!</td>).)*)</td>", re.S)
TAG_RE = re.compile(r"<[^>]+>")

# --- Evidence-cell citation layout -------------------------------------------
# Comparison tables jam source citations (file:line refs) inline in the prose,
# which is hard to read. This pass lifts them out of each evidence cell onto
# their own lines beneath the prose, one citation group per line, keeping them
# as <code> so site.js still linkifies them to upstream source.

CODE_SPLIT_RE = re.compile(r"(<code>.*?</code>)", re.S)
CODE_INNER_RE = re.compile(r"^<code>(.*)</code>$", re.S)
REPO_PREFIX_RE = re.compile(r"^(?:ze|freeRtr|vyos-1x)/")
CONT_RE = re.compile(r"^:\d+(?:-\d+)?(?:,:?\d+(?:-\d+)?)*$")
LINE_REF_RE = re.compile(r":\d")
CODE_EXTS = (
    "go|py|java|yang|sh|json|mk|proto|opam|md|txt|conf|tst|ftr|csv|sfdsk|"
    "yml|yaml|service|xml|in|i|j2|beg|end|dsk|gns|def|rng"
)
BARE_FILE_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_.@-]*\.(?:%s)$" % CODE_EXTS)
# text sitting between citations in a group: pure separators or a short
# connector word ("CLI", "render", "import", "help", ...), no sentence/paren
# boundary.
SEP_RE = re.compile(r"^[\s,;]*(?:[A-Za-z][A-Za-z-]{0,9}\s*){0,2}[\s,;]*$")


def _classify_code(inner):
    """Classify a <code> span's text: FULLCITE (starts a citation line),
    JOINER (continuation / sibling file, joins the open line), or INLINE."""
    t = inner.strip()
    if not t or " " in t or "|" in t or "<" in t or "(" in t or ")" in t:
        return "INLINE"
    core = REPO_PREFIX_RE.sub("", t)
    if CONT_RE.match(core):
        return "JOINER"
    if "/" not in core and BARE_FILE_RE.match(core):
        return "JOINER"
    if LINE_REF_RE.search(core):
        return "FULLCITE"
    if "/" in core and "." in core:
        return "FULLCITE"
    if core in ("Makefile", "go.mod"):
        return "FULLCITE"
    if core.endswith("/") and "/" in core[:-1]:
        return "FULLCITE"
    return "INLINE"


def _is_separator(text):
    return (
        "." not in text
        and "(" not in text
        and ")" not in text
        and bool(SEP_RE.match(text))
    )


PROSE_CLEANUPS = [
    (re.compile(r"\s*[,:;]\s*([.;,)])"), r"\1"),
    (re.compile(r"\(\s*\)"), ""),
    (re.compile(r"\(\s*[,;:]\s*"), "("),
    (re.compile(r"[,;:]\s*\)"), ")"),
    (re.compile(r"\s+([.,;:)])"), r"\1"),
    (re.compile(r"\(\s+"), "("),
    (re.compile(r"\s{2,}"), " "),
    (re.compile(r"(?:\.\s*){2,}"), ". "),
    (re.compile(r"\s+,"), ","),
    (re.compile(r",\s*,"), ","),
]


def _clean_prose(p):
    for rx, rep in PROSE_CLEANUPS:
        p = rx.sub(rep, p)
    p = re.sub(r"^[\s,.;:]+", "", p)
    p = re.sub(r"[\s:;,]+$", "", p).strip()
    if p and p[-1] not in ".!?:":
        p += "."
    return p


def _strip_repo_prefix(codeseg):
    """Strip a leading ze/ | freeRtr/ | vyos-1x/ repo prefix from a citation
    <code> span so every ref reads uniformly and site.js can linkify it (its
    source map keys off the bare repo-relative path)."""
    m = CODE_INNER_RE.match(codeseg)
    if not m:
        return codeseg
    stripped = REPO_PREFIX_RE.sub("", m.group(1))
    return codeseg if stripped == m.group(1) else "<code>%s</code>" % stripped


def _relayout_cell(inner):
    parts = CODE_SPLIT_RE.split(inner)
    prose, groups, cur = [], [], None
    for seg in parts:
        m = CODE_INNER_RE.match(seg)
        if m:
            kind = _classify_code(html.unescape(m.group(1)))
            if kind == "FULLCITE":
                cur = [seg]
                groups.append(cur)
            elif kind == "JOINER" and cur is not None:
                cur.append(seg)
            else:
                cur = None
                prose.append(seg)
        else:
            if cur is not None and _is_separator(seg):
                continue
            cur = None
            prose.append(seg)
    if not groups:
        return None
    prose_html = _clean_prose("".join(prose))
    refs = "".join(
        '<span class="ev-ref">%s</span>' % ", ".join(_strip_repo_prefix(s) for s in g)
        for g in groups
    )
    lead = (prose_html + " ") if prose_html else ""
    return '%s<span class="ev-src">%s</span>' % (lead, refs)


def relayout_evidence_cells(body_html):
    def repl(m):
        attrs, inner = m.group(1), m.group(2)
        new_inner = _relayout_cell(inner)
        return m.group(0) if new_inner is None else "<td%s>%s</td>" % (attrs, new_inner)

    return TD_RE.sub(repl, body_html)


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
MD_LINK_RE = re.compile(r"\[([^\]]*)\]\(([^)\s]+)\)")


def markdown_base_url(dest):
    """Canonical directory URL used to resolve links extracted from HTML."""
    key = sitelib.page_key_for_path(dest).strip("/")
    return sitelib.SITE_BASE + (key + "/" if key else "")


def rewrite_doc_links_markdown(md_text, doc_rel, manifest, dest_rel_dir):
    """Markdown-flavored twin of rewrite_doc_links: rewrites [text](target.md)
    links in the *source* markdown (not the rendered HTML) so the published
    index.md sibling points at another page's index.md when that page is
    also published, or falls back to a GitHub blob link otherwise -- same
    resolution rule, just emitting Markdown link syntax instead of an href
    attribute."""
    source_dir = posixpath.dirname(doc_rel)

    def repl(m):
        label, target = m.group(1), m.group(2)
        if target.startswith(("http://", "https://", "mailto:", "#")):
            return m.group(0)
        path_part, sep, fragment = target.partition("#")
        if not path_part:
            return m.group(0)
        if path_part.endswith("/"):
            target_doc_rel = posixpath.normpath(posixpath.join(source_dir, path_part))
            new_target = (
                "https://github.com/ze-software/ze/tree/main/docs/%s" % target_doc_rel
            )
            return "[%s](%s)" % (label, new_target)
        if not path_part.endswith(".md"):
            return m.group(0)
        target_doc_rel = posixpath.normpath(posixpath.join(source_dir, path_part))
        if target_doc_rel in manifest:
            target_dest_dir = manifest[target_doc_rel]
            rel = posixpath.relpath(target_dest_dir, dest_rel_dir)
            new_target = rel + "/index.md" + (("#" + fragment) if sep else "")
            return "[%s](%s)" % (label, new_target)
        new_target = (
            "https://github.com/ze-software/ze/blob/main/docs/%s" % target_doc_rel
        )
        if sep:
            new_target += "#" + fragment
        return "[%s](%s)" % (label, new_target)

    return MD_LINK_RE.sub(repl, md_text)


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
    source,
    dest,
    root,
    desc=None,
    manifest=None,
    doc_rel=None,
    dest_rel_dir=None,
    cat=None,
    journey_label=None,
):
    source_text = source.read_text()
    metadata, md_text = parse_front_matter(source_text)
    # Resolve {{ze:...}} number tokens from site-facts twice: marked spans for
    # HTML verification, plain text for the Markdown mirror.
    md_text_plain = sitelib.substitute_number_tokens(md_text)
    md_text_html = sitelib.substitute_number_tokens(md_text, html_spans=True)
    title = metadata.get("title") or first_h1(md_text_plain)
    description = desc or metadata.get("description") or "Ze documentation."
    category = cat or metadata.get("category")
    if category and category not in PAGE_CATEGORIES:
        raise ValueError(
            "front matter `category` must be one of: %s"
            % ", ".join(sorted(PAGE_CATEGORIES))
        )
    table_columns = metadata_bool(metadata, "table-columns", True)

    md = markdown.Markdown(extensions=["tables", "fenced_code", "sane_lists", "toc"])
    body_html = md.convert(md_text_html)
    toc_html = render_doc_toc(md.toc_tokens)
    md_out = md_text_plain
    if manifest is not None:
        body_html = rewrite_doc_links(body_html, doc_rel, manifest, dest_rel_dir)
        md_out = rewrite_doc_links_markdown(md_text_plain, doc_rel, manifest, dest_rel_dir)
    if sitelib.contains_block_html(md_text_html):
        md_out = sitelib.html_to_markdown(
            body_html,
            base_url=markdown_base_url(dest),
        )
    body_html, md_out, demo_head = terminal_demos.expand(
        body_html, md_out, root, doc_rel
    )
    body_html = relayout_evidence_cells(body_html)
    body_html = colorcode_cells(body_html)
    body_html = sitelib.patch_external_link_targets(body_html)
    body_html = wrap_journey_hero(
        body_html,
        journey_label
        or metadata.get("journey")
        or default_journey_label(dest, doc_rel),
    )
    body_html = insert_doc_toc(body_html, toc_html)
    section_class = (
        "md-content reveal cat-%s" % category if category else "md-content reveal"
    )
    table_attr = "" if table_columns else ' data-table-columns="off"'
    full_title = "%s - Ze" % title
    head = sitelib.page_head(
        full_title,
        description,
        root,
        og_title=full_title,
        og_desc=description,
        extra_head=demo_head,
        page_key=sitelib.page_key_for_path(dest),
    )
    head += '            <section class="%s"%s>\n' % (
        section_class,
        table_attr,
    )
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(
        head + body_html + "\n            </section>\n" + sitelib.page_foot(root)
    )
    sitelib.write_markdown_sibling(dest, md_out)
    copy_referenced_images(source, dest, md_text)
    print("rendered %s -> %s (+ index.md)" % (source, dest))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("source", type=pathlib.Path)
    parser.add_argument(
        "dest",
        type=pathlib.Path,
        nargs="?",
        help="output index.html; may be supplied by `destination` front matter",
    )
    parser.add_argument(
        "--root",
        help="relative path back to site root; derived from the destination by default",
    )
    parser.add_argument("--desc", help="meta description; overrides front matter")
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
        choices=sorted(PAGE_CATEGORIES),
        help="topic category, colors the h1 per the site's color convention "
        "(same seven hues as the Features category legend)",
    )
    parser.add_argument(
        "--journey-label",
        help="override the top-right label for the shared clay page hero",
    )
    args = parser.parse_args()

    if not args.source.exists():
        print("error: source not found: %s" % args.source, file=sys.stderr)
        return 1

    try:
        metadata, _body = parse_front_matter(args.source.read_text())
        dest = args.dest
        if dest is None:
            destination = metadata.get("destination")
            if not destination:
                parser.error(
                    "dest is required unless front matter defines `destination`"
                )
            dest = page_destination(destination)
        root = args.root or page_root(dest)
        manifest = json.loads(args.manifest.read_text()) if args.manifest else None
        render(
            args.source,
            dest,
            root,
            args.desc,
            manifest=manifest,
            doc_rel=args.doc_rel,
            dest_rel_dir=args.dest_rel_dir,
            cat=args.cat,
            journey_label=args.journey_label,
        )
    except (OSError, ValueError) as exc:
        print("error: %s: %s" % (args.source, exc), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
