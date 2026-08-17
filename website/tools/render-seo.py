#!/usr/bin/env -S uv run python3
"""Render sitemap.xml and robots.txt for the published site."""

import html
import pathlib

import page_registry
import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
SITEMAP = GH_PAGES / "sitemap.xml"
ROBOTS = GH_PAGES / "robots.txt"
SKIP_TOP = {
    ".git",
    ".ruff_cache",
    "assets",
    "data",
    "tmp",
    "tools",
}
REDIRECT_PAGE_DIRS = {pathlib.Path(source) for source in page_registry.url_redirects()}


def include_page(path):
    rel = path.relative_to(GH_PAGES)
    if rel.parent in REDIRECT_PAGE_DIRS:
        return False
    if rel.parts[0] in SKIP_TOP:
        return False
    return path.name == "index.html"


def page_url(path):
    rel = path.relative_to(GH_PAGES)
    if rel == pathlib.Path("index.html"):
        return sitelib.SITE_BASE
    parent = rel.parent.as_posix()
    return sitelib.SITE_BASE + parent.rstrip("/") + "/"


def sitemap_xml(urls):
    lines = [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ]
    for url in urls:
        lines.append("  <url>")
        lines.append("    <loc>%s</loc>" % html.escape(url, quote=False))
        lines.append("  </url>")
    lines.append("</urlset>")
    return "\n".join(lines) + "\n"


def robots_txt():
    return "".join(
        [
            "User-agent: *\n",
            "Allow: /\n",
            "\n",
            "Sitemap: %ssitemap.xml\n" % sitelib.SITE_BASE,
        ]
    )


def main():
    urls = sorted(
        {page_url(path) for path in GH_PAGES.rglob("*.html") if include_page(path)}
    )
    SITEMAP.write_text(sitemap_xml(urls))
    ROBOTS.write_text(robots_txt())
    print("rendered sitemap.xml (%d URLs) and robots.txt" % len(urls))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
