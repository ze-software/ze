#!/usr/bin/env -S uv run --with markdown python3
"""Regenerate the entire gh-pages site in one command.

Usage:
    tools/build.py                  # everything
    tools/build.py --only docs,nav  # just the listed steps

Steps (default order, also the --only vocabulary):
    docs      main/docs/*.md -> docs/**/index.html      (data/nav.json MANIFEST in render-docs.py)
    blog      blog/posts/*.md -> blog/**/index.html      (tools/render-blog.py)
    activity  git history -> activity/index.html         (tools/render-activity.py)
    compare   compare/comparison.md -> compare/index.html (tools/render-doc.py)
    features  data/features.json -> features/index.html  (tools/render-features.py)
    cli       `ze help command --json` -> cli/index.html  (tools/render-cli-catalog.py)
    deps      ../main/go.mod -> dependencies/index.html    (tools/render-dependencies.py)
    config    ../main/internal/**/register.go + YANG -> config-reference/index.html
              (tools/extract-plugin-registry.py, tools/extract-yang-config-tree.py,
              tools/render-config-reference.py) -- extract-yang-config-tree.py runs
              `ze yang tree --json --config` against ../main/bin/ze (same
              bin/ze requirement as the "cli" step) so each group can show
              a readable, command-line-shaped config tree instead of only
              raw YANG source
    contribute contribute/contribute.md -> contribute/index.html (tools/render-doc.py)
    talks     data/talks.json -> talks/index.html          (tools/render-talks.py)
    index     data/audience.json -> index.html            (tools/render-index.py)
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
    "contribute",
    "talks",
    "index",
    "nav",
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


def step_contribute():
    render_doc = load_module("render-doc")
    render_doc.render(
        GH_PAGES / "contribute" / "contribute.md",
        GH_PAGES / "contribute" / "index.html",
        "../",
        CONTRIBUTE_DESC,
    )
    return 0


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
    "contribute": step_contribute,
    "talks": step_talks,
    "index": step_index,
    "llms": step_llms,
    "nav": step_nav,
}


def check_feature_count_drift():
    """The Features nav-dropdown description repeats a card count that
    tools/render-features.py computes fresh every run; data/nav.json can't
    compute anything, so warn instead of shipping a stale number silently --
    this is exactly the class of bug that motivated this data-driven build
    (the "41 features" copy went stale when AS112 moved to experimental)."""
    import json

    features = json.loads((GH_PAGES / "data" / "features.json").read_text())
    core = next(s for s in features["sections"] if s["id"] == "core")
    experimental = next(s for s in features["sections"] if s["id"] == "experimental")
    live_count = len(core["cards"]) + len(experimental["cards"])

    nav = json.loads((GH_PAGES / "data" / "nav.json").read_text())
    project = next(d for d in nav["dropdowns"] if d["label"] == "Project")
    features_item = next(
        e for e in project["columns"][0] if e.get("title") == "Features"
    )
    if str(live_count) not in features_item["desc"]:
        print(
            "warning: data/nav.json Features dropdown says %r but "
            "data/features.json has %d shipped+experimental cards -- update "
            "the desc text in data/nav.json" % (features_item["desc"], live_count),
            file=sys.stderr,
        )


def check_cli_count_drift():
    """Same drift class as check_feature_count_drift, for the CLI Reference
    nav item's command count -- data/cli-commands.json is regenerated fresh
    from the binary every `cli` step run, data/nav.json's copy is not."""
    import json

    cli_data_path = GH_PAGES / "data" / "cli-commands.json"
    if not cli_data_path.exists():
        return
    commands = json.loads(cli_data_path.read_text())
    live_count = len(commands)

    nav = json.loads((GH_PAGES / "data" / "nav.json").read_text())
    project = next(d for d in nav["dropdowns"] if d["label"] == "Project")
    cli_item = next(
        (e for e in project["columns"][0] if e.get("title") == "CLI Reference"), None
    )
    if cli_item and str(live_count) not in cli_item["desc"]:
        print(
            "warning: data/nav.json CLI Reference dropdown says %r but "
            "data/cli-commands.json has %d commands -- update the desc text "
            "in data/nav.json" % (cli_item["desc"], live_count),
            file=sys.stderr,
        )


def check_config_reference_drift():
    """Same drift class again, for the Configuration Reference nav item's
    plugin/group counts -- data/plugin-registry.json is regenerated fresh
    from ../main/internal/**/register.go every `config` step run, but the
    grouping logic (tools/render-config-reference.py's group_key()) has to
    also run to know the group count, so this only checks the plugin count
    (the number before "plugins across")."""
    import json

    registry_path = GH_PAGES / "data" / "plugin-registry.json"
    if not registry_path.exists():
        return
    plugins = json.loads(registry_path.read_text())
    live_count = sum(
        1 for p in plugins if not p["source_dir"].startswith("internal/test/")
    )

    nav = json.loads((GH_PAGES / "data" / "nav.json").read_text())
    project = next(d for d in nav["dropdowns"] if d["label"] == "Project")
    config_item = next(
        (
            e
            for e in project["columns"][0]
            if e.get("title") == "Configuration Reference"
        ),
        None,
    )
    if config_item and str(live_count) not in config_item["desc"]:
        print(
            "warning: data/nav.json Configuration Reference dropdown says %r "
            "but data/plugin-registry.json has %d non-test plugins -- update "
            "the desc text in data/nav.json" % (config_item["desc"], live_count),
            file=sys.stderr,
        )


def check_homepage_proof_drift():
    """The homepage proof-strip (render-index.py PROOF_STATS) states floors
    like "17,300+ unit tests" -- hand-set because computing them exactly
    means walking ../main (test funcs, fuzz targets, .ci files, interop
    Dockerfiles), which render-index.py doesn't do on every run. Warn if the
    live count in ../main ever drops below (or, for the exact interop-target
    count, diverges from) the stated number -- that turns a "+" floor into
    an outright overstatement, the same drift class as the other checks."""
    import re

    render_index = load_module("render-index")
    stats = render_index.PROOF_STATS

    main_repo = (GH_PAGES.parent / "main").resolve()
    if not main_repo.exists():
        return
    skip_dirs = {"vendor", ".claude", ".git"}
    test_func_re = re.compile(r"^func Test[A-Za-z0-9_]+\(", re.MULTILINE)
    fuzz_func_re = re.compile(r"^func Fuzz[A-Za-z0-9_]+\(", re.MULTILINE)

    def under_skip_dir(path):
        return bool(skip_dirs & set(path.relative_to(main_repo).parts))

    unit_tests = 0
    fuzz_targets = 0
    for path in main_repo.rglob("*_test.go"):
        if under_skip_dir(path):
            continue
        text = path.read_text(errors="ignore")
        unit_tests += len(test_func_re.findall(text))
        fuzz_targets += len(fuzz_func_re.findall(text))

    test_dir = main_repo / "test"
    e2e_tests = sum(
        1
        for path in (test_dir.rglob("*.ci") if test_dir.exists() else [])
        if not under_skip_dir(path)
    )

    interop_dir = main_repo / "test" / "interop"
    interop_targets = 1 + sum(  # +1 for FRR, pulled via FRR_IMAGE env var
        1
        for path in (interop_dir.glob("Dockerfile.*") if interop_dir.exists() else [])
        if path.name != "Dockerfile.ze"
    )

    def floor_int(s):
        return int(s.rstrip("+").replace(",", ""))

    for key, live, label in [
        ("unit_tests", unit_tests, "unit test functions"),
        ("e2e_tests", e2e_tests, "end-to-end .ci files"),
        ("fuzz_targets", fuzz_targets, "fuzz targets"),
    ]:
        stated = floor_int(stats[key])
        if stated > live:
            print(
                "warning: render-index.py PROOF_STATS[%r] claims %r but "
                "../main currently has %d %s -- lower the stated floor"
                % (key, stats[key], live, label),
                file=sys.stderr,
            )

    if floor_int(stats["interop_targets"]) != interop_targets:
        print(
            "warning: render-index.py PROOF_STATS['interop_targets'] claims "
            "%r but ../main/test/interop currently has %d target Dockerfiles "
            "+ FRR -- update the stated number"
            % (stats["interop_targets"], interop_targets),
            file=sys.stderr,
        )


def check_llms_md_siblings():
    """llms.txt links every nav.json entry to a sibling index.md alongside
    its index.html. Every render-*.py that produces one of these pages
    writes that sibling itself, but nothing forces it to -- warn instead of
    shipping a page llms.txt claims exists but a fetch would 404 on."""
    import json

    nav = json.loads((GH_PAGES / "data" / "nav.json").read_text())
    hrefs = [link["href"] for link in nav["trailing_links"]]
    for dropdown in nav["dropdowns"]:
        for column in dropdown["columns"]:
            hrefs.extend(e["href"] for e in column if "href" in e)

    for href in hrefs:
        md_path = GH_PAGES / href / "index.md"
        if not md_path.exists():
            print(
                "warning: llms.txt links %s to an index.md that doesn't exist "
                "(%s) -- run the step that generates it" % (href, md_path),
                file=sys.stderr,
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
            print(
                "warning: performance/index.html %s row says %r but "
                "../main/test/perf/results/ze.json's last run (%s) says %r "
                "-- update the status-row text" % (label, got, run_date, want),
                file=sys.stderr,
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

    check_feature_count_drift()
    check_cli_count_drift()
    check_config_reference_drift()
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

    if failures:
        print("\nfailed step(s): %s" % ", ".join(failures), file=sys.stderr)
        return 1
    print("\nbuild complete.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
