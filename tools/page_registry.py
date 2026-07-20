"""Central page lists for gh-pages build scripts.

The registry owns the small hand-maintained sets that decide which Markdown
sources are published by generic renderers and which hand-authored pages get
navigation patches.  Paths are stored as site-relative destinations only; use
page_root_for_dest() to derive the relative path back to the site root.
"""

import pathlib
from dataclasses import dataclass

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent


@dataclass(frozen=True)
class MarkdownPage:
    source: str
    dest: str
    desc: str
    cat: str | None = None


# path relative to the main repo's docs/, mapped to a topic category (or
# None for umbrella/mixed-topic docs that should keep the neutral heading
# color). Categories reuse the Features page's seven hues so a reader
# learns one color convention across the whole site.
DOCS_MANIFEST = {
    "architecture.md": None,
    "architecture/testing/interop.md": "routing",
    "features.md": None,
    "features/ai-first.md": "automate",
    "features/api-commands.md": "automate",
    "features/bgp-protocol.md": "routing",
    "features/rfc-status.md": "routing",
    "features/cli-commands.md": "operate",
    "features/configuration.md": "operate",
    "features/dns-resolver.md": "services",
    "features/exabgp-compatibility.md": "automate",
    "features/fleet-management.md": "automate",
    "features/formatting.md": "operate",
    "features/interfaces.md": "services",
    "features/interoperability-testing.md": "observe",
    "features/introspection.md": "observe",
    "features/looking-glass.md": "operate",
    "features/mcp-integration.md": "automate",
    "features/web-interface.md": "operate",
    "guide/anomaly.md": "secure",
    "guide/as112.md": "services",
    "guide/authentication.md": "secure",
    "guide/authorization.md": "secure",
    "guide/audit.md": "secure",
    "guide/benchmarking.md": "observe",
    "guide/bfd.md": "routing",
    "guide/bmp.md": "routing",
    "guide/command-reference.md": "operate",
    "guide/config-editor.md": "operate",
    "guide/ddos-mitigation.md": "secure",
    "guide/flowspec-protected-router.md": "secure",
    "guide/flowspec-route-reflector.md": "routing",
    "guide/firewall.md": "services",
    "guide/flow-export.md": "observe",
    "guide/isis.md": "routing",
    "guide/ipsec.md": "secure",
    "guide/irr-filtering.md": "routing",
    "guide/l2tp.md": "services",
    "guide/ospf.md": "routing",
    "guide/monitoring.md": "observe",
    "guide/mrt-analysis.md": "observe",
    "guide/policy-routing.md": "services",
    "guide/plugins.md": "automate",
    "guide/pppoe.md": "services",
    "guide/production-diagnostics.md": "observe",
    "guide/health-checks.md": "observe",
    "guide/quickstart.md": "operate",
    "guide/radius.md": "secure",
    "guide/route-injection.md": "routing",
    "guide/rpki.md": "routing",
    "guide/static-routes.md": "routing",
    "guide/looking-glass-howto.md": "operate",
    "guide/tacacs.md": "secure",
    "guide/operator-access-rbac.md": "secure",
    "guide/traffic-usage.md": "observe",
    "guide/terminal-demonstrations.md": "observe",
    "guide/vrrp.md": "routing",
    "guide/vpp.md": "services",
    "guide/appliance.md": "platform",
    "guide/ze-install.md": "platform",
    "guide/ubuntu-build-install.md": "platform",
    "performance.md": "observe",
    "research/vpp-deployment-reference.md": "services",
}

NAV_PATCH_TARGETS = [
    "zeledon/index.html",
    "style-guide/index.html",
    "performance/index.html",
    "labs/index.html",
    "labs/appliance-install/index.html",
    "labs/bgp-interop/index.html",
    "labs/ipsec-interop/index.html",
    "labs/l2tp-interop/index.html",
    "labs/looking-glass-graph/index.html",
    "labs/pppoe-interop/index.html",
    "labs/vlan-qos/index.html",
    "labs/vpp-dataplane/index.html",
]

USAGE_DESC = (
    "Deployment examples for using Ze in a real network, with adjacent router "
    "configs and the lab evidence behind each shape."
)

USAGE_PAGES = [
    MarkdownPage("index.md", "usage/index.html", USAGE_DESC),
    MarkdownPage(
        "as112/index.md",
        "usage/as112/index.html",
        (
            "Use Ze as an AS112 anycast DNS node inside a network, with peer "
            "configs for FRR, BIRD, VyOS, Junos, and Cisco IOS XR."
        ),
        "services",
    ),
    MarkdownPage(
        "exabgp-migration/index.md",
        "usage/exabgp-migration/index.html",
        "Convert an ExaBGP config and run existing process scripts with Ze.",
        "automate",
    ),
    MarkdownPage(
        "bgp-performance/index.md",
        "usage/bgp-performance/index.html",
        "Route-server performance tests with ze-perf sender, receiver, and JSON reports.",
        "automate",
    ),
]

LAB_DETAIL_PAGES = [
    MarkdownPage(
        "labs/l2tp-interop.md",
        "labs/l2tp-interop/architecture/index.html",
        "Peer-isolated Docker lab details for full L2TP PPP/NCP/kernel dataplane evidence.",
        "services",
    ),
    MarkdownPage(
        "labs/pppoe-interop.md",
        "labs/pppoe-interop/architecture/index.html",
        "Peer-isolated Docker lab details for Ze PPPoE client interop with accel-ppp.",
        "services",
    ),
]

COMPARE_DESC = (
    "Comparison hub for Ze against BGP daemons and router network operating systems."
)

COMPARE_PAGES = [
    MarkdownPage("comparison.md", "compare/index.html", COMPARE_DESC, "routing"),
    MarkdownPage(
        "bgp.md",
        "compare/bgp/index.html",
        "How Ze compares to mature BGP daemon implementations, including where it is still behind.",
        "routing",
    ),
    MarkdownPage(
        "nos.md",
        "compare/nos/index.html",
        "How Ze compares to VyOS and freeRtr as full router operating systems.",
        "platform",
    ),
]

QUALITY_DESC = (
    "How Ze proves code quality with unit tests, functional scenarios, QEMU, "
    "fuzzing, gomu mutation testing, and release evidence."
)

QUALITY_PAGES = [
    MarkdownPage("quality.md", "quality/index.html", QUALITY_DESC, "observe"),
    MarkdownPage(
        "functional-ci.md",
        "quality/functional-ci/index.html",
        "How to write and run Ze functional .ci tests.",
        "observe",
    ),
    MarkdownPage(
        "browser-editor.md",
        "quality/browser-editor/index.html",
        "How to write and run Ze browser .wb tests and editor .et tests.",
        "observe",
    ),
    MarkdownPage(
        "unit-fuzz-mutation.md",
        "quality/unit-fuzz-mutation/index.html",
        "How to write and run Ze unit tests, fuzz targets, and gomu mutation checks.",
        "observe",
    ),
    MarkdownPage(
        "qemu-interop-release.md",
        "quality/qemu-interop-release/index.html",
        "How to run Ze QEMU, interop, deployment, performance, and release evidence.",
        "observe",
    ),
    MarkdownPage(
        "verify-debugging.md",
        "quality/verify-debugging/index.html",
        "How Ze verify, failure routing, traces, and debug logging work.",
        "observe",
    ),
]


def doc_stem(doc_path):
    return doc_path[:-3]  # strip ".md"


def docs_dest_rel_dir_for(doc_path):
    return "docs/%s" % doc_stem(doc_path)


def docs_dest_rel_for(doc_path):
    return "%s/index.html" % docs_dest_rel_dir_for(doc_path)


def docs_link_manifest():
    return {doc_path: docs_dest_rel_dir_for(doc_path) for doc_path in DOCS_MANIFEST}


def page_root_for_dest(dest_rel):
    rel = pathlib.PurePosixPath(str(dest_rel))
    if rel.name != "index.html":
        raise ValueError("site page destinations must end in index.html: %s" % dest_rel)
    return "../" * (len(rel.parts) - 1)
