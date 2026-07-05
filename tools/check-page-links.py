#!/usr/bin/env python3
"""Validate external links used by generated site navigation.

Checks data/page-links.json plus generated HTML anchor policy:
  * page-link external URLs are unique by normalized URL.
  * page-link groups do not repeat the same external page.
  * generated external anchors use target="_blank" and rel="noopener".
  * page-link external URLs are reachable, following redirects.

By default the network reachability check is scoped to data/page-links.json,
because command-equivalent source citations include many vendor deep links that
are useful evidence but noisy for every local build. Pass --all-html to check
all unique external hrefs found in generated site HTML as well.
"""

import argparse
import collections
import html.parser
import json
import pathlib
import socket
import ssl
import sys
import urllib.error
import urllib.parse
import urllib.request

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
PAGE_LINKS = GH_PAGES / "data" / "page-links.json"
SITE_BASE = "https://ze-software.net/"
USER_AGENT = "ze-site-link-check/1.0"
TIMEOUT = 12


class AnchorScanner(html.parser.HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.anchors = []

    def handle_starttag(self, tag, attrs):
        if tag != "a":
            return
        data = dict(attrs)
        href = data.get("href", "")
        if is_external_url(href):
            self.anchors.append((href, data.get("target"), data.get("rel")))


def is_external_url(url):
    return url.startswith(("http://", "https://")) and not url.startswith(SITE_BASE)


def normalize_url(url):
    parsed = urllib.parse.urlsplit(url)
    scheme = parsed.scheme.lower()
    netloc = parsed.netloc.lower()
    if scheme == "http" and netloc.endswith(":80"):
        netloc = netloc[:-3]
    if scheme == "https" and netloc.endswith(":443"):
        netloc = netloc[:-4]
    path = parsed.path or "/"
    if path != "/":
        path = path.rstrip("/")
    return urllib.parse.urlunsplit((scheme, netloc, path, parsed.query, ""))


def load_page_links():
    return json.loads(PAGE_LINKS.read_text())


def iter_page_specs(data):
    for kind in ("pages", "patterns"):
        for key, spec in data.get(kind, {}).items() if kind == "pages" else enumerate(data.get(kind, [])):
            yield "%s:%s" % (kind, key), spec


def check_page_link_data(data):
    errors = []
    external = data.get("external", {})
    by_normalized = collections.defaultdict(list)
    for ref, item in external.items():
        url = item.get("url", "")
        if not is_external_url(url) and not url.startswith(SITE_BASE):
            errors.append("external %s has non-http URL %r" % (ref, url))
            continue
        by_normalized[normalize_url(url)].append(ref)
    for url, refs in sorted(by_normalized.items()):
        if len(refs) > 1:
            errors.append("duplicate external URL %s used by %s" % (url, ", ".join(refs)))

    for spec_name, spec in iter_page_specs(data):
        seen = collections.defaultdict(list)
        for group in spec.get("groups", []):
            group_title = group.get("title", "group")
            for link in group.get("links", []):
                if "external" in link:
                    ref = link["external"]
                    if ref not in external:
                        errors.append("%s group %s references unknown external %s" % (spec_name, group_title, ref))
                        continue
                    identity = normalize_url(external[ref]["url"])
                elif is_external_url(link.get("href", "")):
                    identity = normalize_url(link["href"])
                else:
                    continue
                seen[identity].append(group_title)
        for identity, groups in sorted(seen.items()):
            if len(groups) > 1:
                errors.append("%s repeats external page %s in groups %s" % (spec_name, identity, ", ".join(groups)))
    return errors


def iter_html_files():
    for path in GH_PAGES.rglob("*.html"):
        rel = path.relative_to(GH_PAGES)
        if rel.parts and rel.parts[0] == "presentations":
            continue
        yield path


def scan_html_external_links():
    anchors = []
    for path in iter_html_files():
        parser = AnchorScanner()
        parser.feed(path.read_text(errors="ignore"))
        for href, target, rel in parser.anchors:
            anchors.append((path, href, target, rel))
    return anchors


def check_html_anchor_policy(anchors):
    errors = []
    for path, href, target, rel in anchors:
        if target != "_blank":
            errors.append("%s external link %s target is %r, expected _blank" % (path.relative_to(GH_PAGES), href, target))
        rel_tokens = set((rel or "").split())
        if "noopener" not in rel_tokens:
            errors.append("%s external link %s rel lacks noopener" % (path.relative_to(GH_PAGES), href))
    return errors


def request_once(url, method):
    req = urllib.request.Request(url, method=method, headers={"User-Agent": USER_AGENT})
    with urllib.request.urlopen(req, timeout=TIMEOUT, context=ssl.create_default_context()) as resp:
        if method == "GET":
            resp.read(2048)
        return resp.status, resp.geturl()


def check_url(url):
    try:
        return request_once(url, "HEAD")
    except urllib.error.HTTPError as exc:
        if exc.code not in (403, 405, 501):
            return exc.code, exc.geturl()
    except (urllib.error.URLError, TimeoutError, socket.timeout, ssl.SSLError):
        pass
    try:
        return request_once(url, "GET")
    except urllib.error.HTTPError as exc:
        return exc.code, exc.geturl()


def check_reachable(named_urls):
    errors = []
    final_urls = collections.defaultdict(list)
    for name, url in sorted(named_urls.items()):
        try:
            status, final_url = check_url(url)
        except Exception as exc:  # network stack exposes several exception types
            errors.append("%s %s failed: %s" % (name, url, exc))
            continue
        if not (200 <= int(status) < 400):
            errors.append("%s %s returned HTTP %s" % (name, url, status))
            continue
        final_urls[normalize_url(final_url)].append(name)
    for final_url, names in sorted(final_urls.items()):
        page_link_names = [name for name in names if name.startswith("page-links:")]
        if len(page_link_names) > 1:
            errors.append("external entries resolve to the same page %s: %s" % (final_url, ", ".join(page_link_names)))
    return errors


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--all-html", action="store_true", help="also check reachability for every unique generated external href")
    parser.add_argument("--skip-network", action="store_true", help="skip reachability and only validate data and generated anchor policy")
    args = parser.parse_args()

    data = load_page_links()
    errors = []
    errors.extend(check_page_link_data(data))

    anchors = scan_html_external_links()
    errors.extend(check_html_anchor_policy(anchors))

    if not args.skip_network:
        named_urls = {
            "page-links:%s" % ref: item["url"]
            for ref, item in data.get("external", {}).items()
        }
        if args.all_html:
            for href in sorted({href for _path, href, _target, _rel in anchors}):
                named_urls.setdefault("html:%s" % href, href)
        errors.extend(check_reachable(named_urls))

    if errors:
        for err in errors:
            print("error: " + err, file=sys.stderr)
        return 1
    scope = "all generated external hrefs" if args.all_html else "data/page-links.json external URLs"
    print("validated external links: %s, %d generated external anchors" % (scope, len(anchors)))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
