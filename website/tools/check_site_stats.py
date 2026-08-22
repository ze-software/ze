#!/usr/bin/env -S uv run python3
"""Validate repo-stat rendering and source literals in the website."""

import html
import json
import pathlib
import re
import sys
from html.parser import HTMLParser

import sitelib
import sitefacts

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent

TOKEN_RE = re.compile(r"\{\{ze:([a-z0-9-]+)\}\}")
SNAPSHOT_MARKER = "ze-stat-snapshot"
CURRENT_STAT_RE = re.compile(
    r"\b\d[\d,]*\+?\s+"
    r"(unit tests|fuzz targets|end[- ]to[- ]end tests|interop targets|"
    r"RFCs|MUST-level requirements|Go packages|commands|dependencies|scenarios)\b",
    re.IGNORECASE,
)
STAT_ELEMENT_RE = re.compile(
    r"(<(?P<tag>[A-Za-z][A-Za-z0-9:-]*)"
    r"(?=[^>]*\sdata-ze-stat=(?:\"(?P<dkey>[^\"]+)\"|'(?P<skey>[^']+)'))"
    r"[^>]*>)(?P<value>[^<]*)(?P<close></(?P=tag)>)"
)
SOURCE_GLOBS = (
    "blog/posts/*.md",
    "changes/posts/*.md",
    "compare/*.md",
    "use-cases/**/*.md",
    "quality/*.md",
    "faq/*.md",
    "roadmap/*.md",
    "license/*.md",
    "contribute/*.md",
    "docs/docs.md",
    "data/audience.json",
    "data/command-equivalents.json",
    "data/dependencies.json",
    "data/features.json",
    "data/milestones.json",
    "data/nav.json",
    "data/page-links.json",
    "data/whats-new.json",
)


class StatHTMLParser(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.stack = []
        self.spans = []

    def handle_starttag(self, tag, attrs):
        attr = dict(attrs)
        key = attr.get("data-ze-stat")
        if key:
            self.stack.append({"tag": tag, "key": key, "text": []})

    def handle_endtag(self, tag):
        if self.stack and self.stack[-1]["tag"] == tag:
            item = self.stack.pop()
            self.spans.append((item["key"], "".join(item["text"])))

    def handle_data(self, data):
        for item in self.stack:
            item["text"].append(data)


def lookup(facts, key):
    value = facts
    for part in key.split("."):
        if not isinstance(value, dict) or part not in value:
            return None
        value = value[part]
    return value


def normalize(text):
    return " ".join(str(text).split())

def patch_html_stat_markers(text, facts):
    errors = []
    changed = 0

    def repl(match):
        nonlocal changed
        key = match.group("dkey") or match.group("skey")
        expected = lookup(facts, key)
        if expected is None:
            errors.append("unknown data-ze-stat=%s" % key)
            return match.group(0)
        value = html.escape(str(expected), quote=False)
        if normalize(match.group("value")) == normalize(value):
            return match.group(0)
        changed += 1
        return "%s%s%s" % (match.group(1), value, match.group("close"))

    return STAT_ELEMENT_RE.sub(repl, text), errors, changed


def update_html_stat_markers(root=GH_PAGES, facts=None):
    if facts is None:
        facts = sitefacts.load_facts()
    errors = []
    updated = []
    marker_count = 0
    for path in root.rglob("*.html"):
        text = path.read_text(errors="ignore")
        patched, path_errors, changed = patch_html_stat_markers(text, facts)
        if path_errors:
            errors.extend(
                "%s has %s" % (path.relative_to(root), error) for error in path_errors
            )
            continue
        if changed:
            path.write_text(patched)
            updated.append(path)
            marker_count += changed
    return errors, updated, marker_count


def iter_source_files(root=GH_PAGES):
    seen = set()
    for pattern in SOURCE_GLOBS:
        for path in root.glob(pattern):
            if path.is_file() and path not in seen:
                seen.add(path)
                yield path


def check_html_stat_markers(root=GH_PAGES, facts=None):
    if facts is None:
        facts = sitefacts.load_facts()
    errors = []
    for path in root.rglob("*.html"):
        parser = StatHTMLParser()
        parser.feed(path.read_text(errors="ignore"))
        for key, got in parser.spans:
            expected = lookup(facts, key)
            if expected is None:
                errors.append(
                    "%s has unknown data-ze-stat=%s" % (path.relative_to(root), key)
                )
                continue
            if normalize(got) != normalize(expected):
                errors.append(
                    "%s data-ze-stat=%s shows %r but data/site-facts.json says %r"
                    % (path.relative_to(root), key, normalize(got), normalize(expected))
                )
    return errors


def check_source_tokens(root=GH_PAGES):
    valid = set(sitelib.number_token_specs())
    errors = []
    for path in iter_source_files(root):
        rel = path.relative_to(root)
        text = path.read_text(errors="ignore")
        for match in TOKEN_RE.finditer(text):
            if match.group(1) not in valid:
                errors.append(
                    "%s uses unknown site number token {{ze:%s}}" % (rel, match.group(1))
                )
        if SNAPSHOT_MARKER in text:
            continue
        for line_no, line in enumerate(text.splitlines(), 1):
            if "{{ze:" in line:
                continue
            if CURRENT_STAT_RE.search(line):
                errors.append(
                    "%s:%d hardcodes a current repo statistic; use {{ze:<name>}} "
                    "or mark a historical snapshot" % (rel, line_no)
                )
    return errors


def main():
    errors = check_source_tokens() + check_html_stat_markers()
    if errors:
        for error in errors:
            print("error: " + error, file=sys.stderr)
        return 1
    print("site statistic checks passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
