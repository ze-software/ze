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
    "architecture/config/deprecated-options.md": "operate",
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
    "architecture/command-ownership.md": "automate",
    "architecture/config/deprecated-options.md": "operate",
    "architecture/route-selection.md": "routing",
    "architecture/system-architecture.md": "platform",
    "architecture/testing/ci-format.md": "observe",
    "contributing/documentation-testing.md": "observe",
    "contributing/rfc-implementation-guide.md": "routing",
    "contributing/testing.md": "observe",
    "features/srv6.md": "routing",
    "glossary.md": None,
    "guide/add-path.md": "routing",
    "guide/api.md": "automate",
    "guide/bgp-peering.md": "routing",
    "guide/bgp-policy.md": "routing",
    "guide/bgp-resilience.md": "routing",
    "guide/bgp-role.md": "routing",
    "guide/chaos-testing.md": "observe",
    "guide/cli.md": "operate",
    "guide/config-archive.md": "operate",
    "guide/configuration.md": "operate",
    "guide/debugging-tools.md": "observe",
    "guide/developer-setup.md": "platform",
    "guide/environment-variables.md": "operate",
    "guide/fleet-config.md": "automate",
    "guide/gnmi.md": "automate",
    "guide/graceful-restart.md": "routing",
    "guide/healthcheck.md": "observe",
    "guide/lifecycle.md": "operate",
    "guide/logging.md": "observe",
    "guide/looking-glass.md": "operate",
    "guide/mcp/chaos.md": "observe",
    "guide/mcp/elicitation.md": "automate",
    "guide/mcp/overview.md": "automate",
    "guide/mcp/remote-access.md": "secure",
    "guide/mpls.md": "routing",
    "guide/operations.md": "operate",
    "guide/redistribution.md": "routing",
    "guide/rsvp-te.md": "routing",
    "guide/self-update.md": "platform",
    "guide/traffic-control.md": "services",
    "guide/web-interface.md": "operate",
    "history.md": None,
    "plugin-development.md": "automate",
    "plugin-development/commands.md": "automate",
    "plugin-development/handlers.md": "automate",
    "plugin-development/metrics.md": "observe",
    "plugin-development/protocol.md": "automate",
    "plugin-development/schema.md": "automate",
    "plugin-development/testing.md": "observe",
    "why-ze.md": None,
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
    MarkdownPage(
        "route-server/index.md",
        "usage/route-server/index.html",
        "Deploy Ze as an IXP route server with member policy, validation, and verification.",
        "routing",
    ),
    MarkdownPage(
        "transit-edge-rpki/index.md",
        "usage/transit-edge-rpki/index.html",
        "Build a dual-transit Internet edge with RPKI origin validation and tested failover.",
        "routing",
    ),
    MarkdownPage(
        "flowspec-injection/index.md",
        "usage/flowspec-injection/index.html",
        "Inject and withdraw FlowSpec rules through an authorised, observable workflow.",
        "secure",
    ),
    MarkdownPage(
        "chaos-tested-peering/index.md",
        "usage/chaos-tested-peering/index.html",
        "Prove BGP convergence and recovery with deterministic chaos scenarios.",
        "observe",
    ),
    MarkdownPage(
        "as-path-topology/index.md",
        "usage/as-path-topology/index.html",
        "Use the Looking Glass AS-path graph to investigate routing visibility and changes.",
        "observe",
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
    """Relative path from a page back to the site root.

    Depth is the number of directory segments above the file, so this works for
    any page, not only ``index.html`` ones: a top-level ``404.html`` gets ``""``
    and ``a/b/index.html`` gets ``../../``. (It used to require an
    ``index.html`` name and raise otherwise, which meant any standalone page
    carrying the shared header -- e.g. a future ``404.html`` -- crashed the nav
    step instead of being patched.)"""
    rel = pathlib.PurePosixPath(str(dest_rel))
    return "../" * (len(rel.parts) - 1)
