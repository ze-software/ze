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
    index     data/audience.json -> index.html            (tools/render-index.py)
    nav       patch <div class="nav-links"> in the remaining hand-authored
              pages (zeledon, labs/*, talks, style-guide, performance) so
              they stay in sync with data/nav.json without a full rewrite

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

STEPS = ["docs", "blog", "activity", "compare", "features", "cli", "index", "nav"]

NAV_PATCH_TARGETS = [
    ("zeledon/index.html", "../"),
    ("talks/index.html", "../"),
    ("style-guide/index.html", "../"),
    ("performance/index.html", "../"),
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


def step_index():
    render_index = load_module("render-index")
    return render_index.main()


def step_nav():
    for rel, root in NAV_PATCH_TARGETS:
        path = GH_PAGES / rel
        path.write_text(sitelib.patch_navblock(path.read_text(), root))
        print("patched nav -> %s" % rel)
    return 0


STEP_FUNCS = {
    "docs": step_docs,
    "blog": step_blog,
    "activity": step_activity,
    "compare": step_compare,
    "features": step_features,
    "cli": step_cli,
    "index": step_index,
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

    if failures:
        print("\nfailed step(s): %s" % ", ".join(failures), file=sys.stderr)
        return 1
    print("\nbuild complete.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
