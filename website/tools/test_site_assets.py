#!/usr/bin/env -S uv run --with markdown --with pytest python3
"""Tests for the asset-URL, canonical, number-token, and page-root helpers in
sitelib / page_registry -- the pure string/path functions that the asset and
number-accuracy work added, which are trivial to test and easy to regress."""

import importlib.util
import json
import pathlib
import pytest
import sys

HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import page_registry  # noqa: E402
import sitefacts  # noqa: E402
import sitelib  # noqa: E402
import check_site_stats  # noqa: E402

SEO_SPEC = importlib.util.spec_from_file_location("render_seo", HERE / "render-seo.py")
render_seo = importlib.util.module_from_spec(SEO_SPEC)
SEO_SPEC.loader.exec_module(render_seo)
REDIRECT_SPEC = importlib.util.spec_from_file_location(
    "render_redirects", HERE / "render-redirects.py"
)
render_redirects = importlib.util.module_from_spec(REDIRECT_SPEC)
REDIRECT_SPEC.loader.exec_module(render_redirects)
LINK_SPEC = importlib.util.spec_from_file_location(
    "check_page_links", HERE / "check-page-links.py"
)
check_page_links = importlib.util.module_from_spec(LINK_SPEC)
LINK_SPEC.loader.exec_module(check_page_links)


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
    assert (
        sitelib.page_canonical_url("docs/guide/x/index.html") == base + "docs/guide/x/"
    )
    # a non-index page (e.g. a future 404.html) keeps its filename
    assert sitelib.page_canonical_url("404.html") == base + "404.html"


def test_patch_canonical_inserts_and_is_idempotent():
    page = (
        '<head>\n        <link rel="stylesheet" href="../assets/site.css" />\n</head>'
    )
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


def test_substitute_known_number_token_as_marked_span():
    tokens = sitelib.number_tokens()
    if "unit-tests" not in tokens:
        return
    out = sitelib.substitute_number_tokens(
        "Backed by {{ze:unit-tests}} tests.", html_spans=True
    )
    assert 'data-ze-stat="tests.unit_display"' in out
    assert tokens["unit-tests"] in out


def test_stat_marker_check_matches_site_facts(tmp_path):
    facts = {"tests": {"unit_display": "23,100+"}}
    old_facts_path = sitefacts.FACTS_PATH
    try:
        sitefacts.FACTS_PATH = tmp_path / "site-facts.json"
        sitefacts.FACTS_PATH.write_text(json.dumps(facts))
        page = tmp_path / "index.html"
        page.write_text('<span data-ze-stat="tests.unit_display">23,100+</span>')
        assert check_site_stats.check_html_stat_markers(tmp_path) == []
        page.write_text('<span data-ze-stat="tests.unit_display">23,000+</span>')
        assert check_site_stats.check_html_stat_markers(tmp_path)
    finally:
        sitefacts.FACTS_PATH = old_facts_path


def test_update_html_stat_markers_rewrites_stale_values(tmp_path):
    facts = {
        "tests": {"unit_display": "23,100+"},
        "rfc": {"gated_must_display": "2,966"},
    }
    page = tmp_path / "index.html"
    page.write_text(
        '<strong><span data-ze-stat="tests.unit_display">old</span></strong>\\n'
        "<span data-ze-stat='rfc.gated_must_display'>2,900</span>\\n"
    )

    errors, pages, markers = check_site_stats.update_html_stat_markers(
        tmp_path, facts
    )

    assert errors == []
    assert pages == [page]
    assert markers == 2
    text = page.read_text()
    assert 'data-ze-stat="tests.unit_display">23,100+</span>' in text
    assert "data-ze-stat='rfc.gated_must_display'>2,966</span>" in text
    assert check_site_stats.check_html_stat_markers(tmp_path, facts) == []


def _check_changes_post_source(tmp_path, text):
    post = tmp_path / "changes" / "posts" / "snapshot.md"
    post.parent.mkdir(parents=True)
    post.write_text(text)
    return check_site_stats.check_source_tokens(tmp_path)


def test_source_stat_snapshot_requires_canonical_true_front_matter(tmp_path):
    errors = _check_changes_post_source(
        tmp_path,
        "---\n"
        "title: Historical statistics\n"
        "ze-stat-snapshot: true\n"
        "---\n"
        "There are 3 MUST-level requirements.\n",
    )

    assert errors == []


@pytest.mark.parametrize(
    "source",
    [
        "---\nze-stat-snapshot: false\n---\nThere are 3 MUST-level requirements.\n",
        "---\nnot-ze-stat-snapshot: true\n---\nThere are 3 MUST-level requirements.\n",
        "---\ntitle: Current statistics\n---\n"
        "ze-stat-snapshot: true\n\nThere are 3 MUST-level requirements.\n",
        "ze-stat-snapshot: true\n\nThere are 3 MUST-level requirements.\n",
        "---\nze-stat-snapshot: true\nThere are 3 MUST-level requirements.\n",
        "---\nze-stat-snapshot: true\nze-stat-snapshot: false\n---\n"
        "There are 3 MUST-level requirements.\n",
    ],
    ids=[
        "false",
        "prefixed-key",
        "body",
        "missing-front-matter",
        "unterminated-front-matter",
        "conflicting-duplicates",
    ],
)
def test_source_stat_snapshot_rejects_noncanonical_markers(tmp_path, source):
    errors = _check_changes_post_source(tmp_path, source)

    assert len(errors) == 1
    assert "hardcodes a current repo statistic" in errors[0]


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


def test_docs_destinations_follow_public_information_architecture():
    expected = {
        "architecture.md": "architecture/index.html",
        "features/configuration.md": "features/bgp-configuration/index.html",
        "features/rfc-status.md": "reference/rfcs/index.html",
        "guide/quickstart.md": "guides/quickstart/index.html",
        "guide/configuration.md": "guides/configuration-model/index.html",
        "guide/netlab.md": "labs/netlab/index.html",
        "guide/terminal-demonstrations.md": "demos/terminal/index.html",
        "performance.md": "performance/bgp/index.html",
        "plugin-development/schema.md": "developers/plugins/schema/index.html",
        "why-ze.md": "project/why-ze/index.html",
    }
    assert {
        source: page_registry.docs_dest_rel_for(source) for source in expected
    } == expected


def test_every_registered_doc_has_one_unique_public_destination():
    destinations = [
        page_registry.docs_dest_rel_dir_for(source)
        for source in page_registry.DOCS_MANIFEST
    ]
    assert len(destinations) == len(set(destinations))
    assert all(not destination.startswith("docs/") for destination in destinations)


def test_only_individual_talk_decks_are_frozen():
    assert page_registry.is_frozen_talk_path("talks/linx-2026-06/index.html")
    assert not page_registry.is_frozen_talk_path("talks/index.html")
    assert not page_registry.is_frozen_talk_path("presentations/tools/bundle-html.py")


def test_legacy_routes_redirect_to_canonical_collections():
    redirects = page_registry.url_redirects()
    assert redirects["docs/guide/quickstart"] == "guides/quickstart"
    assert redirects["docs/features/configuration"] == "features/bgp-configuration"
    assert redirects["docs/contributing/testing"] == "contribute/testing"
    assert redirects["docs/history"] == "project/history"
    assert redirects["usage/route-server"] == "use-cases/route-server"
    assert redirects["presentations/linx-2026-06"] == "talks/linx-2026-06"
    assert redirects["dependencies"] == "reference/dependencies"
    assert redirects["roadmap"] == "project/roadmap"
    assert redirects["changes"] == "project/changes"
    assert redirects["milestones"] == "project/milestones"
    assert redirects["activity"] == "project/activity"
    assert (
        page_registry.file_redirects()["presentations/linx-2026-06/index-inlined.html"]
        == "talks/linx-2026-06/index-inlined.html"
    )
    assert not (set(redirects) & set(redirects.values()))


def test_legacy_absolute_links_are_rewritten_to_canonical_routes():
    text = (
        "https://ze-software.net/docs/guide/quickstart/ "
        "https://ze-software.net/presentations/linx-2026-06/index-inlined.html"
    )
    rewritten = page_registry.rewrite_legacy_public_urls(text, sitelib.SITE_BASE)
    assert rewritten == (
        "https://ze-software.net/guides/quickstart/ "
        "https://ze-software.net/talks/linx-2026-06/index-inlined.html"
    )


def test_local_link_checker_resolves_clean_routes(tmp_path, monkeypatch):
    page = tmp_path / "docs" / "index.html"
    target = tmp_path / "guides" / "quickstart" / "index.html"
    page.parent.mkdir(parents=True)
    target.parent.mkdir(parents=True)
    page.write_text("<html></html>")
    target.write_text("<html></html>")
    monkeypatch.setattr(check_page_links, "GH_PAGES", tmp_path)

    references = [(page, "a", "href", "../guides/quickstart/")]
    errors, checked = check_page_links.check_local_references(references)
    assert checked == 1
    assert errors == []

    broken = [(page, "a", "href", "../guides/missing/")]
    errors, checked = check_page_links.check_local_references(broken)
    assert checked == 1
    assert "points to missing ../guides/missing/" in errors[0]


def test_static_redirects_are_noindex_and_canonical():
    page = render_redirects.redirect_html("guides/quickstart")
    target = sitelib.SITE_BASE + "guides/quickstart/"
    assert '<meta name="robots" content="noindex">' in page
    assert '<link rel="canonical" href="%s">' % target in page
    assert "location.replace(" in page
    assert 'content="0; url=/guides/quickstart/"' in page
    assert 'location.replace("/guides/quickstart/"' in page
    assert target in page


def test_static_file_redirect_does_not_add_a_trailing_slash():
    page = render_redirects.redirect_html("talks/linx-2026-06/index-inlined.html")
    assert 'content="0; url=/talks/linx-2026-06/index-inlined.html"' in page
    assert (
        'href="https://ze-software.net/talks/linx-2026-06/index-inlined.html"' in page
    )


def test_sitemap_uses_clean_canonical_routes_and_excludes_redirects():
    root = HERE.parent
    assert not render_seo.include_page(
        root / "docs" / "guide" / "quickstart" / "index.html"
    )
    assert render_seo.include_page(root / "guides" / "quickstart" / "index.html")
    assert (
        render_seo.page_url(root / "talks" / "linx-2026-06" / "index.html")
        == sitelib.SITE_BASE + "talks/linx-2026-06/"
    )
    assert "Sitemap: %ssitemap.xml" % sitelib.SITE_BASE in render_seo.robots_txt()


# --- footer publication stamp ----------------------------------------------


def _fixed_published(monkeypatch, iso):
    monkeypatch.setattr(sitefacts, "_published_at_cache", None)
    monkeypatch.setenv(sitefacts.PUBLISHED_AT_ENV, iso)


def test_published_display_renders_the_stamp_in_utc(monkeypatch):
    _fixed_published(monkeypatch, "2026-08-17T14:32:05+00:00")
    assert sitefacts.published_display() == "17 August 2026 14:32 UTC"


def test_published_display_converts_an_offset_stamp_to_utc(monkeypatch):
    # The footer prints the word UTC, so an offset stamp must be converted, not
    # relabelled. British Summer Time is +01:00, the offset a local clock gives.
    _fixed_published(monkeypatch, "2026-08-17T00:32:05+01:00")
    assert sitefacts.published_display() == "16 August 2026 23:32 UTC"


def test_published_at_takes_the_build_stamp_over_any_facts_snapshot(monkeypatch):
    # The page renderers run as subprocesses and the `facts` step runs after
    # most of them, so a stamp read from the snapshot gives the pages built
    # before that step the PREVIOUS build's time. The env stamp is the source.
    _fixed_published(monkeypatch, "2026-08-17T14:32:05+00:00")
    monkeypatch.setattr(
        sitefacts, "load_facts", lambda: {"published_at": "1999-01-01T00:00:00+00:00"}
    )
    assert sitefacts.published_at() == "2026-08-17T14:32:05+00:00"


def test_published_at_without_a_build_stamp_reads_the_clock_once(monkeypatch):
    # A renderer run by hand has no env stamp. It must still hand every page it
    # writes one value, not one clock reading per call.
    monkeypatch.setattr(sitefacts, "_published_at_cache", None)
    monkeypatch.delenv(sitefacts.PUBLISHED_AT_ENV, raising=False)
    assert sitefacts.published_at() == sitefacts.published_at()


def test_footer_carries_the_license_line_and_the_stamp(monkeypatch):
    _fixed_published(monkeypatch, "2026-08-17T14:32:05+00:00")
    footer = sitelib.footer_html("../../")
    assert '<a href="../../license/">Ze is AGPLv3 open source.</a>' in footer
    assert (
        '<span class="footer-published">Published 17 August 2026 14:32 UTC</span>'
        in footer
    )


def test_patch_footer_stamps_an_already_authored_page(monkeypatch):
    _fixed_published(monkeypatch, "2026-08-17T14:32:05+00:00")
    before = (
        "<body>\n"
        "        <footer>\n"
        '            <div class="footer-inner">\n'
        '                <div class="footer-bottom">\n'
        '                    <a href="../license/">Ze is AGPLv3 open source.</a>\n'
        "                </div>\n"
        "            </div>\n"
        "        </footer>\n"
        "</body>"
    )
    patched = sitelib.patch_footer(before, "../")
    assert "Published 17 August 2026 14:32 UTC" in patched
    assert patched.count("<footer>") == 1
    assert patched.startswith("<body>\n") and patched.endswith("\n</body>")
    # Re-running the nav step must not stack a second stamp.
    assert sitelib.patch_footer(patched, "../") == patched
