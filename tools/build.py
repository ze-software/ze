#!/usr/bin/env -S uv run --with markdown --with rcssmin --with rjsmin python3
"""Regenerate the entire gh-pages site in one command.

Usage:
    tools/build.py                  # everything
    tools/build.py --only docs,nav  # just the listed steps

Steps (default order, also the --only vocabulary):
    css       assets/css/site.css imports -> assets/site.css, minified
              (tools/render-css.py)
    js        assets/js/site.js -> assets/site.js, minified (tools/render-js.py)
    docs      main/docs/*.md -> docs/** or IA namespace (tools/page_registry.py DOCS_MANIFEST)
    use-cases use-cases/*.md -> use-cases/**/index.html (tools/render-doc.py)
    blog      blog/posts/*.md (editorial articles) -> blog/**/index.html
              (tools/render-blog.py) -- empty until articles are added
    changes   changes/posts/*.md (weekly updates) -> changes/<week>/index.html
              full write-up + changes/index.html terse index + changes/feed.xml
              (tools/render-changes.py)
    activity  git history -> activity/index.html         (tools/render-activity.py)
    compare   compare/*.md -> compare/**/index.html      (tools/render-doc.py)
    features  data/features.json -> features/index.html  (tools/render-features.py)
    cli       `ze help command --json` -> reference/cli/index.html  (tools/render-cli-catalog.py)
    deps      ../main/go.mod -> reference/dependencies/index.html (tools/render-dependencies.py)
    quality   quality/*.md -> quality/**/index.html      (tools/render-doc.py)
    config    live YANG + ../main/internal/**/register.go -> reference/configuration/index.html
              (tools/extract-yang-config-tree.py, tools/extract-plugin-registry.py,
              tools/render-config-reference.py) -- extract-yang-config-tree.py runs
              `ze yang tree --json --config` against ../main/bin/ze (same
              bin/ze requirement as the "cli" step) to get the whole config
              tree; the plugin registry only supplies which plugin owns each
              config path. The page embeds the tree as JSON and browses it
              level by level (breadcrumb + a table of each node's children),
              the same presentation at every depth, not a per-plugin list
    plugins   ../main/internal/**/register.go + local PLUGIN.md front matter
              -> data/plugin-registry.json -> reference/plugins/index.html
              searchable runtime plugin catalog, generated from live registry
              extraction rather than a hand-authored website list
    contribute contribute/contribute.md -> contribute/index.html (tools/render-doc.py)
    talks     data/talks.json -> talks/index.html          (tools/render-talks.py)
    index     data/audience.json -> index.html            (tools/render-index.py)
    timeline  data/milestones.json -> milestones/index.html (tools/render-timeline.py)
    hubs      curated collection landing pages -> */index.html (tools/render-doc.py)
    nav       patch <div class="nav-links"> and <footer> in the remaining
              hand-authored pages (zeledon, labs/*, style-guide,
              performance) so they stay in sync with data/nav.json /
              tools/sitelib.py without a full rewrite -- also (re)writes
              each one's index.md sibling from its own <main> content
              (sitelib.extract_main + sitelib.html_to_markdown), since these
              pages have no Markdown source of their own to publish as-is
    redirects legacy public URLs -> noindex fallback pages (tools/render-redirects.py)
    links     patch generated external links so they use target="_blank" and
              rel="noopener" consistently; always runs after selected steps
    linkcheck validate page-links data and generated external-anchor policy
              without network reachability; always runs after links
    llms      data/nav.json + live counts -> llms.txt      (tools/render-llms-txt.py)
              -- always runs; there is no way to regenerate the site without
              also regenerating llms.txt, so it can never silently go stale.
              Runs after "nav" so every page llms.txt links to already has
              its index.md sibling on disk.

Every published page (generated or hand-authored) gets an index.md sibling
next to its index.html. Plain Markdown sources are published directly, while
sources containing layout HTML are converted from their rendered body so tags
and browser-only controls never leak into the text mirror. Pages built from
structured data render Markdown from that same data; pages with no separate
source use the "nav" step's HTML->Markdown extraction. llms.txt
links every entry to its .md (for an LLM to fetch) alongside the human-
facing HTML page (for when a link needs to be shown to a person) -- see
tools/render-llms-txt.py.

Replaces the old workflow of remembering to run four separate scripts (see
AI.md) -- every page on the site, generated or hand-authored, reads its nav
from the single data/nav.json, so there is nothing left to go stale.
"""

import argparse
import importlib.util
import pathlib
import sys

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent

sys.path.insert(0, str(HERE))
import page_registry  # noqa: E402
import sitelib  # noqa: E402

STEPS = [
    "css",
    "js",
    "docs",
    "use-cases",
    "hubs",
    "labdetails",
    "blog",
    "activity",
    "compare",
    "features",
    "cli",
    "command-equivalents",
    "deps",
    "quality",
    "test-health",
    "rfc-compliance",
    "config",
    "plugins",
    "facts",
    "contribute",
    "contribguide",
    "talks",
    "changes",
    "index",
    "docshub",
    "faq",
    "roadmap",
    "license",
    "coc",
    "security",
    "timeline",
    "nav",
    "redirects",
    "links",
    "linkcheck",
    "search",
    "seo",
    "llms",
]


CONTRIBUTE_DESC = (
    "How to contribute to Ze: the CLA, how the project is funded, and where to start."
)

CONTRIB_GUIDE_DESC = (
    "The practical side of contributing to Ze: how work gets in, how to build, "
    "and what a good change looks like."
)

DOCSHUB_DESC = (
    "All Ze documentation, organised by what you are trying to do: learn, do, "
    "look up, and understand."
)

FAQ_DESC = "The questions people ask before they spend time on Ze, answered honestly."

ROADMAP_DESC = (
    "The work between now and a first release of Ze you can trust in production."
)

LICENSE_DESC = "Ze is free software under the GNU Affero General Public License v3."

COC_DESC = "The code of conduct for the Ze community."

SECURITY_DESC = "How to report a security vulnerability in Ze, what is in scope, and what to expect."

MAIN_REPO = (GH_PAGES.parent / "main").resolve()


def load_module(stem):
    """render-doc.py etc. have hyphenated filenames, so plain `import`
    can't find them -- load by path instead."""
    path = HERE / ("%s.py" % stem)
    spec = importlib.util.spec_from_file_location(stem.replace("-", "_"), path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def step_css():
    render_css = load_module("render-css")
    return render_css.main()


def step_js():
    render_js = load_module("render-js")
    return render_js.main()


def step_docs():
    render_docs = load_module("render-docs")
    return render_docs.main()


def step_use_cases():
    render_doc = load_module("render-doc")
    for page in page_registry.USE_CASE_PAGES:
        render_doc.render(
            GH_PAGES / "use-cases" / page.source,
            GH_PAGES / page.dest,
            page_registry.page_root_for_dest(page.dest),
            page.desc,
            cat=page.cat,
            journey_label="Use case",
        )
    return 0


def step_hubs():
    render_doc = load_module("render-doc")
    for page in page_registry.HUB_PAGES:
        render_doc.render(
            GH_PAGES / page.source,
            GH_PAGES / page.dest,
            page_registry.page_root_for_dest(page.dest),
            page.desc,
            cat=page.cat,
        )
    return 0


def step_labdetails():
    render_doc = load_module("render-doc")
    for page in page_registry.LAB_DETAIL_PAGES:
        render_doc.render(
            MAIN_REPO / "docs" / page.source,
            GH_PAGES / page.dest,
            page_registry.page_root_for_dest(page.dest),
            page.desc,
            cat=page.cat,
            journey_label="Lab details",
        )
    return 0


def step_blog():
    render_blog = load_module("render-blog")
    return render_blog.main()


def step_activity():
    render_activity = load_module("render-activity")
    return render_activity.main()


def step_compare():
    render_doc = load_module("render-doc")
    for page in page_registry.COMPARE_PAGES:
        render_doc.render(
            GH_PAGES / "compare" / page.source,
            GH_PAGES / page.dest,
            page_registry.page_root_for_dest(page.dest),
            page.desc,
            cat=page.cat,
        )
    return 0


def step_features():
    render_features = load_module("render-features")
    return render_features.main()


def step_cli():
    render_cli_catalog = load_module("render-cli-catalog")
    return render_cli_catalog.main()


def step_command_equivalents():
    render_command_equivalents = load_module("render-command-equivalents")
    return render_command_equivalents.main()


def step_deps():
    render_dependencies = load_module("render-dependencies")
    return render_dependencies.main()


def step_quality():
    render_doc = load_module("render-doc")
    for page in page_registry.QUALITY_PAGES:
        render_doc.render(
            GH_PAGES / "quality" / page.source,
            GH_PAGES / page.dest,
            page_registry.page_root_for_dest(page.dest),
            page.desc,
            cat=page.cat,
        )
    return 0


def step_test_health():
    render_test_health = load_module("render-test-health")
    return render_test_health.main()


def step_rfc_compliance():
    render_rfc_compliance = load_module("render-rfc-compliance")
    return render_rfc_compliance.main()


def _extract_plugin_registry_once():
    """Regenerate data/plugin-registry.json from ../main, at most once per
    build. Both step_config and step_plugins consume it; without this guard a
    full build parsed every internal/**/register.go twice."""
    if _extract_plugin_registry_once.done:
        return 0
    extract_plugin_registry = load_module("extract-plugin-registry")
    rc = extract_plugin_registry.main()
    if not rc:
        _extract_plugin_registry_once.done = True
    return rc


_extract_plugin_registry_once.done = False


def step_config():
    rc = _extract_plugin_registry_once()
    if rc:
        return rc
    extract_yang_config_tree = load_module("extract-yang-config-tree")
    rc = extract_yang_config_tree.main()
    if rc:
        return rc
    render_config_reference = load_module("render-config-reference")
    return render_config_reference.main()


def step_plugins():
    rc = _extract_plugin_registry_once()
    if rc:
        return rc
    render_plugin_catalog = load_module("render-plugin-catalog")
    return render_plugin_catalog.main()


def step_facts():
    render_site_facts = load_module("render-site-facts")
    return render_site_facts.main()


def step_contribute():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "contribute" / "contribute.md",
        GH_PAGES / "contribute" / "index.html",
        "../",
        CONTRIBUTE_DESC,
        journey_label="Community",
    )
    return 0


def step_contribguide():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "contribute" / "guide.md",
        GH_PAGES / "contribute" / "guide" / "index.html",
        "../../",
        CONTRIB_GUIDE_DESC,
    )
    return 0


def step_docshub():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "docs" / "docs.md",
        GH_PAGES / "docs" / "index.html",
        "../",
        DOCSHUB_DESC,
    )
    return 0


def step_faq():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "faq" / "faq.md",
        GH_PAGES / "faq" / "index.html",
        "../",
        FAQ_DESC,
        journey_label="FAQ",
    )
    return 0


def step_roadmap():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "roadmap" / "roadmap.md",
        GH_PAGES / "roadmap" / "index.html",
        "../",
        ROADMAP_DESC,
        journey_label="Release path",
    )
    return 0


def step_license():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "license" / "license.md",
        GH_PAGES / "license" / "index.html",
        "../",
        LICENSE_DESC,
    )
    return 0


def step_coc():
    render_doc = load_module("render-doc")
    render_doc.render(
        MAIN_REPO / "CODE_OF_CONDUCT.md",
        GH_PAGES / "code-of-conduct" / "index.html",
        "../",
        COC_DESC,
    )
    return 0


def step_security():
    render_doc = load_module("render-doc")
    render_doc.render(
        MAIN_REPO / "SECURITY.md",
        GH_PAGES / "security" / "index.html",
        "../",
        SECURITY_DESC,
        cat="secure",
    )
    return 0


def step_changes():
    render_changes = load_module("render-changes")
    return render_changes.main()


def step_timeline():
    render_timeline = load_module("render-timeline")
    return render_timeline.main()


def step_search():
    render_search_index = load_module("render-search-index")
    return render_search_index.main()


def step_seo():
    render_seo = load_module("render-seo")
    return render_seo.main()


def step_talks():
    render_talks = load_module("render-talks")
    return render_talks.main()


def step_index():
    render_index = load_module("render-index")
    return render_index.main()


def step_llms():
    render_llms_txt = load_module("render-llms-txt")
    return render_llms_txt.main()


def step_nav():
    """Publish one shared header fragment and stable mounts on every page."""
    sitelib.reset_nav_data_cache()
    header_changed = sitelib.write_shared_header()
    rich_targets = set(page_registry.NAV_PATCH_TARGETS)
    patched = 0
    for path in sorted(GH_PAGES.rglob("*.html")):
        rel = path.relative_to(GH_PAGES)
        if page_registry.is_frozen_talk_path(rel):
            continue
        rel_text = rel.as_posix()
        if path == sitelib.SHARED_HEADER_PATH:
            continue
        text = path.read_text()
        if not sitelib.has_replaceable_header(text):
            continue

        root = page_registry.page_root_for_dest(rel_text)
        updated = sitelib.patch_shared_header(text, root)
        updated = sitelib.patch_theme_bootstrap(updated)
        if rel_text in rich_targets:
            updated = sitelib.patch_page_sidebar(
                updated, root, sitelib.page_key_for_path(rel_text)
            )
            updated = sitelib.patch_footer(updated, root)
        updated = sitelib.patch_asset_versions(updated)
        if updated != text:
            path.write_text(updated)
            patched += 1

        if rel_text in rich_targets:
            base_url = (
                sitelib.SITE_BASE + str(pathlib.PurePosixPath(rel_text).parent) + "/"
            )
            md_text = sitelib.html_to_markdown(
                sitelib.extract_main(updated), base_url=base_url
            )
            sitelib.write_markdown_sibling(path, md_text)

    state = "updated" if header_changed else "unchanged"
    print(
        "shared header %s -> %s; patched mounts -> %d html files"
        % (state, sitelib.SHARED_HEADER_PATH, patched)
    )
    return 0


def step_links():
    html_patched = 0
    markdown_patched = 0
    redirects = page_registry.url_redirects()
    redirect_files = page_registry.file_redirects()
    for path in GH_PAGES.rglob("*.html"):
        rel = path.relative_to(GH_PAGES)
        if page_registry.is_frozen_talk_path(rel):
            continue
        text = path.read_text()
        updated = page_registry.rewrite_legacy_public_urls(
            text, sitelib.SITE_BASE, redirects
        )
        updated = sitelib.patch_external_link_targets(updated)
        updated = sitelib.patch_asset_versions(updated)
        if (
            rel.parent.as_posix() not in redirects
            and rel.as_posix() not in redirect_files
        ):
            updated = sitelib.patch_canonical(updated, rel.as_posix())
        if updated != text:
            path.write_text(updated)
            html_patched += 1

    for path in GH_PAGES.rglob("index.md"):
        rel = path.relative_to(GH_PAGES)
        if page_registry.is_frozen_talk_path(rel):
            continue
        text = path.read_text()
        updated = page_registry.rewrite_legacy_public_urls(
            text, sitelib.SITE_BASE, redirects
        )
        if updated != text:
            path.write_text(updated)
            markdown_patched += 1

    print(
        "patched internal routes, external links, asset versions, canonical "
        "-> %d html files, %d Markdown files" % (html_patched, markdown_patched)
    )
    return 0


def step_linkcheck():
    check_page_links = load_module("check-page-links")
    return check_page_links.main(["--skip-network"])


def step_redirects():
    render_redirects = load_module("render-redirects")
    return render_redirects.main()


STEP_FUNCS = {
    "css": step_css,
    "js": step_js,
    "docs": step_docs,
    "use-cases": step_use_cases,
    "hubs": step_hubs,
    "labdetails": step_labdetails,
    "blog": step_blog,
    "activity": step_activity,
    "compare": step_compare,
    "features": step_features,
    "cli": step_cli,
    "command-equivalents": step_command_equivalents,
    "deps": step_deps,
    "quality": step_quality,
    "test-health": step_test_health,
    "rfc-compliance": step_rfc_compliance,
    "config": step_config,
    "plugins": step_plugins,
    "facts": step_facts,
    "contribute": step_contribute,
    "contribguide": step_contribguide,
    "talks": step_talks,
    "index": step_index,
    "docshub": step_docshub,
    "faq": step_faq,
    "roadmap": step_roadmap,
    "license": step_license,
    "coc": step_coc,
    "security": step_security,
    "changes": step_changes,
    "timeline": step_timeline,
    "llms": step_llms,
    "nav": step_nav,
    "links": step_links,
    "linkcheck": step_linkcheck,
    "search": step_search,
    "seo": step_seo,
    "redirects": step_redirects,
}


# The Features / CLI Reference / Dependencies / Configuration Reference nav
# counts used to be hardcoded in data/nav.json and guarded by three
# check_*_drift() warnings here. They are now %(name)s placeholders in
# data/nav.json substituted from sitelib.live_counts() at render time, so
# they are always live and cannot drift -- the checks are gone with them.


def check_llms_md_siblings():
    """llms.txt links every nav.json entry to a sibling index.md alongside
    its index.html. Every render-*.py that produces one of these pages
    writes that sibling itself, but nothing forces it to -- warn instead of
    shipping a page llms.txt claims exists but a fetch would 404 on."""
    import json

    nav = json.loads((GH_PAGES / "data" / "nav.json").read_text())
    hrefs = [link["href"] for link in nav["trailing_links"]]
    hrefs.extend(link["href"] for link in nav.get("top_links", []))
    for dropdown in nav["dropdowns"]:
        # Dynamic dropdowns (e.g. Blog) have no static "columns" -- their
        # entries are generated at render time, so there is nothing to check.
        for column in dropdown.get("columns", []):
            hrefs.extend(e["href"] for e in column if "href" in e)

    for href in hrefs:
        path = href.split("#", 1)[0]
        if path in ("", "index.html"):
            md_path = GH_PAGES / "index.md"
        else:
            md_path = GH_PAGES / path / "index.md"
        if not md_path.exists():
            sitelib.warn(
                "llms.txt links %s to an index.md that doesn't exist "
                "(%s) -- run the step that generates it" % (href, md_path)
            )


def check_markdown_mirrors():
    """Fail the build when an index.md still contains site-layout HTML."""
    for md_path in GH_PAGES.rglob("index.md"):
        rel = md_path.relative_to(GH_PAGES)
        if page_registry.is_frozen_talk_path(rel):
            continue
        if sitelib.contains_block_html(md_path.read_text()):
            sitelib.warn(
                "%s contains block HTML instead of plain Markdown" % rel.as_posix()
            )


def check_homepage_proof_drift():
    """index.html's proof strip quotes four evidence numbers (unit tests, end
    to end tests, fuzz targets, interop targets). render-index.py generates
    them from data/site-facts.json, but the numbers are also the site's most
    load-bearing trust claim, so this guard re-reads the rendered page and
    fails the build if any of the four no longer matches the facts snapshot --
    catching a render bug or a hand-edit before it ships a stale headline
    number. Same drift class as check_performance_stat_drift."""
    import json
    import re

    facts_path = GH_PAGES / "data" / "site-facts.json"
    page_path = GH_PAGES / "index.html"
    if not facts_path.exists() or not page_path.exists():
        return

    facts = json.loads(facts_path.read_text())
    page = page_path.read_text()

    # label as it appears in the proof strip -> its facts display value
    expected = {
        "unit tests": facts["tests"]["unit_display"],
        "end to end tests": facts["tests"]["e2e_display"],
        "fuzz targets": facts["tests"]["fuzz_display"],
        "interop targets": facts["interop"]["target_display"],
    }

    for label, want in expected.items():
        match = re.search(
            r"<strong\s*>\s*([\d,]+\+?)\s*<span class=\"label\">%s</span>"
            % re.escape(label),
            page,
        )
        got = match.group(1).strip() if match else None
        if got != want:
            sitelib.warn(
                "index.html proof strip shows %r for %r but data/site-facts.json "
                "says %r -- rerun tools/build.py --only index" % (got, label, want)
            )


def check_performance_stat_drift():
    """performance/index.html's status-row (convergence/throughput/withdrawal)
    is hand-copied from ../main/test/perf/results/ze.json's last real
    ze-perf run, not regenerated by any build.py step -- warn if the page
    ever drifts from that file, the same drift class as the other checks,
    so a new perf run doesn't silently leave the page quoting a stale one."""
    import json
    import re

    result_path = (
        GH_PAGES.parent / "main" / "test" / "perf" / "results" / "ze.json"
    ).resolve()
    page_path = GH_PAGES / "performance" / "index.html"
    if not result_path.exists() or not page_path.exists():
        return

    result = json.loads(result_path.read_text())
    page = page_path.read_text()

    run_date = result["timestamp"].split("T")[0]
    expected = {
        "Convergence": "%dms to propagate %s routes (%s run)"
        % (result["convergence-ms"], "{:,}".format(result["routes"]), run_date),
        "Throughput": "%s routes/sec sustained during propagation"
        % "{:,}".format(result["throughput-avg"]),
        "Withdrawal": "%dms from withdrawal sent to receiver idle"
        % result["withdrawal-ms"],
    }

    for label, want in expected.items():
        match = re.search(
            r"<strong>%s</strong>\s*<span>([^<]+)</span>" % re.escape(label), page
        )
        got = match.group(1).strip() if match else None
        if got != want:
            sitelib.warn(
                "performance/index.html %s row says %r but "
                "../main/test/perf/results/ze.json's last run (%s) says %r "
                "-- update the status-row text" % (label, got, run_date, want)
            )


def main():
    parser = argparse.ArgumentParser(
        description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter
    )
    parser.add_argument(
        "--only",
        help="comma-separated subset of: %s (default: all)" % ",".join(STEPS),
    )
    args = parser.parse_args()

    steps = args.only.split(",") if args.only else STEPS
    unknown = set(steps) - set(STEPS)
    if unknown:
        print(
            "error: unknown step(s): %s" % ", ".join(sorted(unknown)), file=sys.stderr
        )
        return 1

    check_performance_stat_drift()

    failures = []

    def run_step(step, always=False):
        label = "%s (always runs)" % step if always else step
        print("=== %s ===" % label)
        try:
            rc = STEP_FUNCS[step]()
        except (Exception, SystemExit) as exc:
            # SystemExit is caught deliberately: a renderer that calls
            # sys.exit() (e.g. render-cli-catalog's production-binary guard)
            # must fail its own step, not tear down the interpreter and skip
            # every downstream step -- including the ALWAYS_RUN guardrails.
            print("error: step %s raised %r" % (step, exc), file=sys.stderr)
            failures.append(step)
            return
        if rc:
            failures.append(step)

    # These guardrails run at the end of every build in this fixed order,
    # whether or not --only names them. Redirects remove obsolete Markdown
    # mirrors before search indexing. Navigation runs before link patching,
    # link validation sees the final anchors, SEO sees canonical routes, and
    # llms runs after every linked page has its index.md sibling. Running these
    # in the main loop as well would invert that order for partial builds, so
    # the loop skips them and runs them once here.
    TAIL = ["redirects", "nav", "links", "linkcheck", "search", "seo", "llms"]

    for step in steps:
        if step in TAIL:
            continue
        run_step(step)

    # facts stays in the loop at its STEPS position for full builds (so content
    # renderers that follow it read fresh counts); a partial build that omits it
    # still refreshes the one public snapshot the menus read.
    if "facts" not in steps:
        run_step("facts", always=True)

    for step in TAIL:
        run_step(step, always=step not in steps)

    check_markdown_mirrors()
    check_llms_md_siblings()
    # Runs after the steps so it validates the freshly rendered homepage (and
    # freshly written site-facts) rather than the pre-build copy -- a full
    # build that corrects the number must not still fail on the stale one, and
    # a partial build that moved the facts without rebuilding index must fail.
    check_homepage_proof_drift()

    # Drift warnings (sitelib.warn) fail the build here, at the very end, so
    # the whole site is still generated but the build goes red until every one
    # is resolved -- a warning nobody must act on gets scrolled past. The
    # messages already printed next to the step that emitted them; this is the
    # consolidated list plus the non-zero exit that forces action.
    drift = sitelib.build_warnings()
    if drift:
        sys.stdout.flush()  # so the summary lands after buffered step output in piped logs
        print(
            "\n%d drift warning(s) must be resolved before this build can "
            "pass (repeated from above):" % len(drift),
            file=sys.stderr,
        )
        for message in drift:
            print("  - " + message, file=sys.stderr)

    if failures or drift:
        if failures:
            print("\nfailed step(s): %s" % ", ".join(failures), file=sys.stderr)
        return 1
    print("\nbuild complete.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
