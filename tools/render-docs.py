#!/usr/bin/env -S uv run --with markdown python3
"""Batch-render main repo docs/*.md files into site-shell-wrapped pages.

Usage:
    tools/render-docs.py

Reads MANIFEST below, renders each ../main/docs/<path>.md into
docs/<path>/index.html (mirroring the source path, e.g.
docs/features/configuration.md -> docs/features/configuration/index.html),
computing the relative --root depth automatically. Re-run this whenever a
doc in MANIFEST changes upstream -- same workflow as render-doc.py for a
single file, just for all of them at once.

Add a new doc to MANIFEST (path relative to the main repo's docs/, no
extension needed logic -- just the .md path) and re-run to publish it.
"""

import json
import pathlib
import subprocess
import sys
import tempfile

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent
MAIN_DOCS = (GH_PAGES.parent / "main" / "docs").resolve()

# path relative to the main repo's docs/, mapped to a topic category (or
# None for umbrella/mixed-topic docs that should keep the neutral heading
# color). Categories reuse the Features page's seven hues so a reader
# learns one color convention across the whole site.
MANIFEST = {
    "architecture.md": None,
    "architecture/testing/interop.md": "routing",
    "architecture/testing/l2tp-interop.md": "services",
    "architecture/testing/pppoe-interop.md": "services",
    "features.md": None,
    "features/ai-first.md": "automate",
    "features/api-commands.md": "automate",
    "features/bgp-protocol.md": "routing",
    "features/cli-commands.md": "operate",
    "features/configuration.md": "operate",
    "features/dns-resolver.md": "services",
    "features/exabgp-compatibility.md": "automate",
    "features/fleet-management.md": "automate",
    "features/interfaces.md": "services",
    "features/interoperability-testing.md": "observe",
    "features/introspection.md": "observe",
    "features/looking-glass.md": "operate",
    "features/mcp-integration.md": "automate",
    "features/plugins.md": "automate",
    "features/web-interface.md": "operate",
    "guide/as112.md": "services",
    "guide/audit.md": "secure",
    "guide/benchmarking.md": "observe",
    "guide/firewall.md": "services",
    "guide/flow-export.md": "observe",
    "guide/isis.md": "routing",
    "guide/l2tp.md": "services",
    "guide/ospf.md": "routing",
    "guide/monitoring.md": "observe",
    "guide/mrt-analysis.md": "observe",
    "guide/policy-routing.md": "services",
    "guide/pppoe.md": "services",
    "guide/production-diagnostics.md": "observe",
    "guide/quickstart.md": "operate",
    "guide/static-routes.md": "routing",
    "guide/tacacs.md": "secure",
    "guide/vpp.md": "services",
    "guide/ze-install.md": "platform",
    "performance.md": "observe",
    "research/vpp-deployment-reference.md": "services",
}


def stem_for(doc_path):
    return doc_path[:-3]  # strip ".md"


def dest_rel_dir_for(doc_path):
    return "docs/%s" % stem_for(doc_path)


def dest_for(doc_path):
    return GH_PAGES / (dest_rel_dir_for(doc_path)) / "index.html"


def root_for(dest):
    depth = len(dest.relative_to(GH_PAGES).parts) - 1  # exclude index.html itself
    return "../" * depth


def main():
    render_doc = HERE / "render-doc.py"
    link_manifest = {doc_path: dest_rel_dir_for(doc_path) for doc_path in MANIFEST}

    with tempfile.NamedTemporaryFile("w", suffix=".json", delete=False, dir=HERE) as f:
        json.dump(link_manifest, f)
        manifest_path = pathlib.Path(f.name)

    try:
        failures = []
        for doc_path in MANIFEST:
            source = MAIN_DOCS / doc_path
            if not source.exists():
                failures.append("missing source: %s" % source)
                continue
            dest = dest_for(doc_path)
            root = root_for(dest)
            cmd = [
                sys.executable,
                str(render_doc),
                str(source),
                str(dest),
                "--root",
                root,
                "--desc",
                "Ze documentation: %s" % doc_path,
                "--manifest",
                str(manifest_path),
                "--doc-rel",
                doc_path,
                "--dest-rel-dir",
                dest_rel_dir_for(doc_path),
            ]
            cat = MANIFEST[doc_path]
            if cat:
                cmd += ["--cat", cat]
            result = subprocess.run(cmd, capture_output=True, text=True)
            if result.returncode != 0:
                failures.append("%s: %s" % (doc_path, result.stderr.strip()))
            else:
                print(result.stdout.strip())
    finally:
        manifest_path.unlink(missing_ok=True)

    if failures:
        print("\n%d failure(s):" % len(failures), file=sys.stderr)
        for f in failures:
            print("  %s" % f, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
