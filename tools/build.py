#!/usr/bin/env -S uv run --with markdown python3
"""Regenerate the entire gh-pages site in one command.

Usage:
    tools/build.py                  # everything
    tools/build.py --only docs,nav  # just the listed steps

Steps (default order, also the --only vocabulary):
    docs      main/docs/*.md -> docs/**/index.html      (data/nav.json MANIFEST in render-docs.py)
    blog      blog/posts/*.md (editorial articles) -> blog/**/index.html
              (tools/render-blog.py) -- empty until articles are added
    changes   changes/posts/*.md (weekly updates) -> changes/<week>/index.html
              full write-up + changes/index.html terse index + changes/feed.xml
              (tools/render-changes.py)
    activity  git history -> activity/index.html         (tools/render-activity.py)
    compare   compare/comparison.md -> compare/index.html (tools/render-doc.py)
    features  data/features.json -> features/index.html  (tools/render-features.py)
    cli       `ze help command --json` -> cli/index.html  (tools/render-cli-catalog.py)
    deps      ../main/go.mod -> dependencies/index.html    (tools/render-dependencies.py)
    config    live YANG + ../main/internal/**/register.go -> config-reference/index.html
              (tools/extract-yang-config-tree.py, tools/extract-plugin-registry.py,
              tools/render-config-reference.py) -- extract-yang-config-tree.py runs
              `ze yang tree --json --config` against ../main/bin/ze (same
              bin/ze requirement as the "cli" step) to get the whole config
              tree; the plugin registry only supplies which plugin owns each
              config path. The page embeds the tree as JSON and browses it
              level by level (breadcrumb + a table of each node's children),
              the same presentation at every depth, not a per-plugin list
    plugins   ../main/internal/**/register.go + local PLUGIN.md front matter
              -> data/plugin-registry.json -> docs/features/plugins/index.html
              searchable runtime plugin catalog, generated from live registry
              extraction rather than a hand-authored website list
    contribute contribute/contribute.md -> contribute/index.html (tools/render-doc.py)
    talks     data/talks.json -> talks/index.html          (tools/render-talks.py)
    index     data/audience.json -> index.html            (tools/render-index.py)
    timeline  data/milestones.json -> milestones/index.html (tools/render-timeline.py)
    nav       patch <div class="nav-links"> and <footer> in the remaining
              hand-authored pages (zeledon, labs/*, style-guide,
              performance) so they stay in sync with data/nav.json /
              tools/sitelib.py without a full rewrite -- also (re)writes
              each one's index.md sibling from its own <main> content
              (sitelib.extract_main + sitelib.html_to_markdown), since these
              pages have no Markdown source of their own to publish as-is
    llms      data/nav.json + live counts -> llms.txt      (tools/render-llms-txt.py)
              -- always runs; there is no way to regenerate the site without
              also regenerating llms.txt, so it can never silently go stale.
              Runs after "nav" so every page llms.txt links to already has
              its index.md sibling on disk.

Every published page (generated or hand-authored) gets an index.md sibling
next to its index.html -- docs/blog/compare/contribute publish their real
Markdown source (link-rewritten to sibling .md paths); features/cli/deps/
config-reference/activity/talks render Markdown straight from the same data
the HTML comes from; labs/style-guide/performance/zeledon (no source of
either kind) get it via the "nav" step's HTML->Markdown extraction. llms.txt
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
import sitelib  # noqa: E402

STEPS = [
    "docs",
    "blog",
    "activity",
    "compare",
    "features",
    "cli",
    "deps",
    "config",
    "plugins",
    "facts",
    "contribute",
    "contribguide",
    "talks",
    "index",
    "docshub",
    "faq",
    "roadmap",
    "license",
    "coc",
    "security",
    "changes",
    "timeline",
    "nav",
    "search",
    "seo",
    "llms",
]

NAV_PATCH_TARGETS = [
    ("zeledon/index.html", "../"),
    ("style-guide/index.html", "../"),
    ("performance/index.html", "../"),
    ("labs/index.html", "../"),
    ("labs/appliance-install/index.html", "../../"),
    ("labs/bgp-interop/index.html", "../../"),
    ("labs/ipsec-interop/index.html", "../../"),
    ("labs/l2tp-interop/index.html", "../../"),
    ("labs/looking-glass-graph/index.html", "../../"),
    ("labs/pppoe-interop/index.html", "../../"),
    ("labs/vlan-qos/index.html", "../../"),
    ("labs/vpp-dataplane/index.html", "../../"),
]

COMPARE_DESC = (
    "How Ze compares to mature BGP daemon implementations, honestly, "
    "including where it's still behind."
)

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
    "The work between here and a first release of Ze you can trust in production."
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


def step_docs():
    render_docs = load_module("render-docs")
    return render_docs.main()


def step_blog():
    render_blog = load_module("render-blog")
    return render_blog.main()


def step_activity():
    render_activity = load_module("render-activity")
    return render_activity.main()


def step_compare():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "compare" / "comparison.md",
        GH_PAGES / "compare" / "index.html",
        "../",
        COMPARE_DESC,
        cat="routing",
    )
    return 0


def step_features():
    render_features = load_module("render-features")
    return render_features.main()


def step_cli():
    render_cli_catalog = load_module("render-cli-catalog")
    return render_cli_catalog.main()


def step_deps():
    render_dependencies = load_module("render-dependencies")
    return render_dependencies.main()


def step_config():
    extract_plugin_registry = load_module("extract-plugin-registry")
    rc = extract_plugin_registry.main()
    if rc:
        return rc
    extract_yang_config_tree = load_module("extract-yang-config-tree")
    rc = extract_yang_config_tree.main()
    if rc:
        return rc
    render_config_reference = load_module("render-config-reference")
    return render_config_reference.main()

def step_plugins():
    extract_plugin_registry = load_module("extract-plugin-registry")
    rc = extract_plugin_registry.main()
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
    for rel, root in NAV_PATCH_TARGETS:
        path = GH_PAGES / rel
        text = sitelib.patch_navblock(path.read_text(), root)
        text = sitelib.patch_footer(text, root)
        text = sitelib.patch_asset_versions(text)
        path.write_text(text)
        base_url = sitelib.SITE_BASE + str(pathlib.PurePosixPath(rel).parent) + "/"
        md_text = sitelib.html_to_markdown(
            sitelib.extract_main(text), base_url=base_url
        )
        sitelib.write_markdown_sibling(path, md_text)
        print("patched nav+footer, wrote index.md -> %s" % rel)
    return 0


STEP_FUNCS = {
    "docs": step_docs,
    "blog": step_blog,
    "activity": step_activity,
    "compare": step_compare,
    "features": step_features,
    "cli": step_cli,
    "deps": step_deps,
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
    "search": step_search,
    "seo": step_seo,
}


# The Features / CLI Reference / Dependencies / Configuration Reference nav
# counts used to be hardcoded in data/nav.json and guarded by three
# check_*_drift() warnings here. They are now %(name)s placeholders in
# data/nav.json substituted from sitelib.live_counts() at render time, so
# they are always live and cannot drift -- the checks are gone with them.


def check_homepage_proof_drift():
    """Homepage proof-strip counts now come from data/site-facts.json.

    The facts step computes them from ../main before index rendering, so the
    old floor drift check is obsolete. Keep this hook as a named no-op because
    main() still runs the shared drift hooks before selected build steps.
    """
    return


def check_llms_md_siblings():
    """llms.txt links every nav.json entry to a sibling index.md alongside
    its index.html. Every render-*.py that produces one of these pages
    writes that sibling itself, but nothing forces it to -- warn instead of
    shipping a page llms.txt claims exists but a fetch would 404 on."""
    import json

    nav = json.loads((GH_PAGES / "data" / "nav.json").read_text())
    hrefs = [link["href"] for link in nav["trailing_links"]]
    for dropdown in nav["dropdowns"]:
        # Dynamic dropdowns (e.g. Blog) have no static "columns" -- their
        # entries are generated at render time, so there is nothing to check.
        for column in dropdown.get("columns", []):
            hrefs.extend(e["href"] for e in column if "href" in e)

    for href in hrefs:
        md_path = GH_PAGES / href / "index.md"
        if not md_path.exists():
            sitelib.warn(
                "llms.txt links %s to an index.md that doesn't exist "
                "(%s) -- run the step that generates it" % (href, md_path)
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

    check_homepage_proof_drift()
    check_performance_stat_drift()
    check_llms_md_siblings()

    failures = []
    for step in steps:
        print("=== %s ===" % step)
        try:
            rc = STEP_FUNCS[step]()
        except Exception as exc:
            print("error: step %s raised %r" % (step, exc), file=sys.stderr)
            failures.append(step)
            continue
        if rc:
            failures.append(step)

    if "llms" not in steps:
        # Runs even when --only excludes it: llms.txt must never go stale
        # relative to whatever this invocation just changed.
        print("=== llms (always runs) ===")
        try:
            rc = step_llms()
        except Exception as exc:
            print("error: step llms raised %r" % exc, file=sys.stderr)
            failures.append("llms")
        else:
            if rc:
                failures.append("llms")

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
