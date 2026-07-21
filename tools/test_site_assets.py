#!/usr/bin/env -S uv run --with markdown --with pytest python3
"""Tests for the asset-URL, canonical, number-token, and page-root helpers in
sitelib / page_registry -- the pure string/path functions that the asset and
number-accuracy work added, which are trivial to test and easy to regress."""

import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import page_registry  # noqa: E402
import sitelib  # noqa: E402


# --- stable asset URLs (no ?v= churn) --------------------------------------


def test_asset_url_has_no_version_query():
    assert sitelib.asset_url("../", "assets/site.css") == "../assets/site.css"
    assert sitelib.asset_url("", "assets/site.js") == "assets/site.js"


def test_patch_asset_versions_strips_legacy_query():
    before = '<link rel="stylesheet" href="../assets/site.css?v=079eb640b0" />'
    after = sitelib.patch_asset_versions(before)
    assert "?v=" not in after
    assert "../assets/site.css" in after


def test_patch_asset_versions_escapes_font_ampersand():
    page = '<link href="%s" rel="stylesheet" />' % sitelib.FONT_CSS_URL
    patched = sitelib.patch_asset_versions(page)
    # the raw & in the font URL must be &amp; inside the href attribute
    assert "&amp;family=Lato" in patched
    assert "?family=Poppins" in patched


# --- self-referential canonical / og:url -----------------------------------


def test_page_canonical_url_shapes():
    base = sitelib.SITE_BASE
    assert sitelib.page_canonical_url("index.html") == base
    assert sitelib.page_canonical_url("docs/guide/x/index.html") == base + "docs/guide/x/"
    # a non-index page (e.g. a future 404.html) keeps its filename
    assert sitelib.page_canonical_url("404.html") == base + "404.html"


def test_patch_canonical_inserts_and_is_idempotent():
    page = '<head>\n        <link rel="stylesheet" href="../assets/site.css" />\n</head>'
    once = sitelib.patch_canonical(page, "docs/x/index.html")
    twice = sitelib.patch_canonical(once, "docs/x/index.html")
    assert once.count('rel="canonical"') == 1
    assert once.count('property="og:url"') == 1
    assert sitelib.SITE_BASE + "docs/x/" in once
    assert once == twice  # no duplicate accumulation across builds


def test_patch_canonical_no_css_link_is_noop():
    page = "<head><title>x</title></head>"
    assert sitelib.patch_canonical(page, "index.html") == page


# --- number tokens (prose counts sourced from site-facts) ------------------


def test_substitute_known_number_token():
    tokens = sitelib.number_tokens()
    if "unit-tests" not in tokens:  # no facts snapshot in this checkout
        return
    out = sitelib.substitute_number_tokens("Backed by {{ze:unit-tests}} tests.")
    assert "{{ze:unit-tests}}" not in out
    assert tokens["unit-tests"] in out


def test_substitute_leaves_non_ze_braces_untouched():
    # Go/Jinja template syntax in a code sample must survive substitution.
    text = "Render {{ .Value }} and {{ block }}."
    assert sitelib.substitute_number_tokens(text) == text


# --- page root depth (works for any page, not only index.html) --------------


def test_page_root_for_dest_depth():
    assert page_registry.page_root_for_dest("index.html") == ""
    assert page_registry.page_root_for_dest("docs/index.html") == "../"
    assert page_registry.page_root_for_dest("docs/guide/x/index.html") == "../../../"
    # used to raise for a non-index.html page; now it just computes depth
    assert page_registry.page_root_for_dest("404.html") == ""
