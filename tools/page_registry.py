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
    "guide/netlab.md": "observe",
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

# Public URLs follow the site's information architecture, never the source
# repository layout. Exact entries handle pages whose public role differs from
# their source family; prefix entries cover coherent content collections. There
# is deliberately no "docs/<source>" fallback: a new source family must make an
# explicit public-URL decision before it can be published.
DOCS_DEST_EXACT = {
    "architecture.md": "architecture",
    "architecture/config/deprecated-options.md": "reference/deprecations",
    "contributing/testing.md": "contribute/testing",
    "features.md": "reference/feature-status",
    "features/configuration.md": "features/bgp-configuration",
    "features/rfc-status.md": "reference/rfcs",
    "glossary.md": "reference/glossary",
    "guide/developer-setup.md": "contribute/developer-setup",
    "guide/configuration.md": "guides/configuration-model",
    "guide/health-checks.md": "guides/system-readiness",
    "guide/healthcheck.md": "guides/bgp-healthcheck",
    "guide/looking-glass-howto.md": "guides/public-looking-glass",
    "guide/netlab.md": "labs/netlab",
    "guide/terminal-demonstrations.md": "demos/terminal",
    "history.md": "project/history",
    "performance.md": "performance/bgp",
    "plugin-development.md": "developers/plugins",
    "research/vpp-deployment-reference.md": "architecture/vpp-deployment",
    "why-ze.md": "project/why-ze",
}

DOCS_DEST_PREFIXES = (
    ("architecture/", "architecture/"),
    ("contributing/", "contribute/"),
    ("features/", "features/"),
    ("guide/", "guides/"),
    ("plugin-development/", "developers/plugins/"),
)

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

HUB_PAGES = [
    MarkdownPage(
        "guides/index.md",
        "guides/index.html",
        "Task-oriented installation, configuration, operation, and troubleshooting guides.",
        "operate",
    ),
    MarkdownPage(
        "reference/index.md",
        "reference/index.html",
        "Generated and lookup-oriented command, configuration, plugin, standards, and feature data.",
    ),
    MarkdownPage(
        "project/index.md",
        "project/index.html",
        "Ze roadmap, milestones, changes, history, talks, and contribution paths.",
    ),
    MarkdownPage(
        "developers/index.md",
        "developers/index.html",
        "Plugin SDK documentation and contributor development workflows.",
        "automate",
    ),
    MarkdownPage(
        "demos/index.md",
        "demos/index.html",
        "Recorded workflows, runnable labs, netlab topologies, and deployment examples.",
        "observe",
    ),
]

USE_CASE_DESC = (
    "Deployment examples for using Ze in a real network, with adjacent router "
    "configs and the lab evidence behind each setup."
)

USE_CASE_PAGES = [
    MarkdownPage("index.md", "use-cases/index.html", USE_CASE_DESC),
    MarkdownPage(
        "as112/index.md",
        "use-cases/as112/index.html",
        (
            "Use Ze as an AS112 anycast DNS node inside a network, with peer "
            "configs for FRR, BIRD, VyOS, Junos, and Cisco IOS XR."
        ),
        "services",
    ),
    MarkdownPage(
        "exabgp-migration/index.md",
        "use-cases/exabgp-migration/index.html",
        "Convert an ExaBGP config and run existing process scripts with Ze.",
        "automate",
    ),
    MarkdownPage(
        "bgp-performance/index.md",
        "use-cases/bgp-performance/index.html",
        "Route-server performance tests with ze-perf sender, receiver, and JSON reports.",
        "automate",
    ),
    MarkdownPage(
        "route-server/index.md",
        "use-cases/route-server/index.html",
        "Deploy Ze as an IXP route server with member policy, validation, and checks.",
        "routing",
    ),
    MarkdownPage(
        "transit-edge-rpki/index.md",
        "use-cases/transit-edge-rpki/index.html",
        "Build a dual-transit Internet edge with RPKI origin validation and tested failover.",
        "routing",
    ),
    MarkdownPage(
        "flowspec-injection/index.md",
        "use-cases/flowspec-injection/index.html",
        "Inject and withdraw FlowSpec rules through an authorised workflow with visible state and logs.",
        "secure",
    ),
    MarkdownPage(
        "chaos-tested-peering/index.md",
        "use-cases/chaos-tested-peering/index.html",
        "Check BGP convergence and recovery with deterministic chaos scenarios.",
        "observe",
    ),
    MarkdownPage(
        "as-path-topology/index.md",
        "use-cases/as-path-topology/index.html",
        "Use the Looking Glass AS-path graph to investigate routing visibility and changes.",
        "observe",
    ),
]

LAB_DETAIL_PAGES = [
    MarkdownPage(
        "labs/l2tp-interop.md",
        "labs/l2tp-interop/architecture/index.html",
        "Peer-isolated Docker lab details for L2TP, PPP, NCP, and kernel dataplane checks.",
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
    "Comparisons between Ze, BGP daemons, and router network operating systems."
)

COMPARE_PAGES = [
    MarkdownPage("comparison.md", "compare/index.html", COMPARE_DESC, "routing"),
    MarkdownPage(
        "bgp.md",
        "compare/bgp/index.html",
        "How Ze compares to mature BGP daemons, including current gaps.",
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
    "How Ze checks code quality with unit tests, functional scenarios, QEMU, "
    "fuzzing, gomu mutation testing, and release checks."
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
        "How to run Ze QEMU tests, interop labs, deployment checks, performance tests, and release checks.",
        "observe",
    ),
    MarkdownPage(
        "verify-debugging.md",
        "quality/verify-debugging/index.html",
        "How ze verify, failure routing, traces, and debug logging work.",
        "observe",
    ),
]


def doc_stem(doc_path):
    return doc_path[:-3]  # strip ".md"


def docs_dest_rel_dir_for(doc_path):
    if doc_path in DOCS_DEST_EXACT:
        return DOCS_DEST_EXACT[doc_path]
    stem = doc_stem(doc_path)
    for source_prefix, public_prefix in DOCS_DEST_PREFIXES:
        if stem.startswith(source_prefix):
            return public_prefix + stem.removeprefix(source_prefix)
    raise KeyError("no public destination for %s" % doc_path)


def docs_dest_rel_for(doc_path):
    return "%s/index.html" % docs_dest_rel_dir_for(doc_path)


def docs_link_manifest():
    return {doc_path: docs_dest_rel_dir_for(doc_path) for doc_path in DOCS_MANIFEST}


LEGACY_DOCS_DEST_OVERRIDES = {
    "architecture/config/deprecated-options.md": "reference/deprecations",
    "contributing/testing.md": "contribute/testing",
    "features/rfc-status.md": "reference/rfcs",
    "glossary.md": "reference/glossary",
    "history.md": "project/history",
}


def legacy_docs_dest_rel_dir_for(doc_path):
    return LEGACY_DOCS_DEST_OVERRIDES.get(doc_path, "docs/%s" % doc_stem(doc_path))


def url_redirects():
    """Legacy site-relative directory -> canonical site-relative directory."""
    redirects = {}
    for doc_path in DOCS_MANIFEST:
        new = docs_dest_rel_dir_for(doc_path)
        old_routes = (
            "docs/%s" % doc_stem(doc_path),
            legacy_docs_dest_rel_dir_for(doc_path),
        )
        for old in old_routes:
            if old != new:
                redirects[old] = new

    for page in USE_CASE_PAGES:
        new = page.dest.removesuffix("/index.html")
        old = "usage" if new == "use-cases" else new.replace("use-cases/", "usage/", 1)
        redirects[old] = new

    redirects.update(
        {
            "cli": "reference/cli",
            "command-equivalents": "reference/command-equivalents",
            "config-reference": "reference/configuration",
            "dependencies": "reference/dependencies",
            "docs/architecture/testing/l2tp-interop": "labs/l2tp-interop/architecture",
            "docs/architecture/testing/pppoe-interop": "labs/pppoe-interop/architecture",
            "docs/features/plugins": "reference/plugins",
            "docs/guide/exabgp-migration": "use-cases/exabgp-migration",
            "guides/configuration": "guides/configuration-model",
            "presentations/linx-2026-06": "talks/linx-2026-06",
            "presentations/netmcr-2026-04": "talks/netmcr-2026-04",
            "why-ze": "project/why-ze",
        }
    )
    return redirects


def file_redirects():
    return {
        "presentations/linx-2026-06/index-inlined.html": (
            "talks/linx-2026-06/index-inlined.html"
        ),
        "presentations/netmcr-2026-04/index-inlined.html": (
            "talks/netmcr-2026-04/index-inlined.html"
        ),
    }


def rewrite_legacy_public_urls(text, site_base, redirects=None):
    base = site_base.rstrip("/") + "/"
    routes = url_redirects() if redirects is None else redirects
    for source, target in routes.items():
        text = text.replace(base + source + "/", base + target + "/")
    for source, target in file_redirects().items():
        text = text.replace(base + source, base + target)
    return text


def is_frozen_talk_path(path):
    parts = pathlib.PurePosixPath(str(path)).parts
    return len(parts) > 1 and parts[0] == "talks" and parts[1] != "index.html"


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
