// Design: website/AI.md -- the public URL of a page is an information-architecture decision
// Detail: docs.go renders these rows; redirect.go will publish the routes they moved from.
package site

import "fmt"

// docsManifestRow names one Markdown source under docs/ that the site
// publishes, and the topic category that colors its title.
//
// The category is the only per-page fact the registry carries. A page's title
// comes from its front matter or its first heading, and its description is
// derived, so this is a category map rather than a page table.
type docsManifestRow struct {
	// Source is the path relative to docs/, extension included.
	Source string
	// Category is one of the site's seven topic hues, or empty for an
	// umbrella page that keeps the neutral heading color.
	Category string
}

// docsDestinationPrefix maps a source family to the public family it publishes
// under. The pairs are ORDERED and the first matching prefix wins.
type docsDestinationPrefix struct {
	Source string
	Public string
}

// docsDestination answers the artifact directory one docs source publishes to.
//
// The exact table is consulted first, then the prefixes in their declared
// order. There is deliberately NO docs/<stem> fallback: a source family with no
// entry in either table is refused, so a new family states its public URL
// before it can publish rather than acquiring one nobody reviewed.
func docsDestination(docPath string) (string, error) {
	if destination, named := docsDestinationExact[docPath]; named {
		return destination, nil
	}
	stem, isMarkdown := cutMarkdownSuffix(docPath)
	if !isMarkdown {
		return "", fmt.Errorf("docs source %s is not a Markdown file", docPath)
	}
	for _, prefix := range docsDestinationPrefixes {
		if len(stem) > len(prefix.Source) && stem[:len(prefix.Source)] == prefix.Source {
			return prefix.Public + stem[len(prefix.Source):], nil
		}
	}
	return "", fmt.Errorf("no public destination for docs/%s: add it to the exact table or to a prefix family", docPath)
}

// cutMarkdownSuffix answers a source path without its .md extension.
func cutMarkdownSuffix(docPath string) (string, bool) {
	const suffix = ".md"
	if len(docPath) <= len(suffix) || docPath[len(docPath)-len(suffix):] != suffix {
		return "", false
	}
	return docPath[:len(docPath)-len(suffix)], true
}

// docsLinkManifest answers every published docs source and the directory it
// publishes to, which is what a cross-document link is rewritten against.
func docsLinkManifest() (map[string]string, error) {
	manifest := make(map[string]string, len(docsManifest))
	for _, row := range docsManifest {
		destination, err := docsDestination(row.Source)
		if err != nil {
			return nil, err
		}
		manifest[row.Source] = destination
	}
	return manifest, nil
}

// docsManifest names every Markdown source under docs/ that the site
// publishes, in the order the retired registry declared them.
//
// The order is kept because the redirect table derives from it: the legacy
// route of each row is replaced over the output of the row before it, so a set
// with no order would publish a different redirect map. The retired registry
// named architecture/config/deprecated-options.md twice with one value and
// Python deduped it silently; the row appears once here, at its first position.
var docsManifest = []docsManifestRow{
	{Source: "architecture.md", Category: ""},
	{Source: "architecture/api/commands.md", Category: categoryAutomate},
	{Source: "architecture/config/deprecated-options.md", Category: categoryOperate},
	{Source: "architecture/testing/interop.md", Category: categoryRouting},
	{Source: "features.md", Category: ""},
	{Source: "features/ai-first.md", Category: categoryAutomate},
	{Source: "features/api-commands.md", Category: categoryAutomate},
	{Source: "features/bgp-protocol.md", Category: categoryRouting},
	{Source: "features/rfc-status.md", Category: categoryRouting},
	{Source: "features/cli-commands.md", Category: categoryOperate},
	{Source: "features/configuration.md", Category: categoryOperate},
	{Source: "features/dns-resolver.md", Category: categoryServices},
	{Source: "features/exabgp-compatibility.md", Category: categoryAutomate},
	{Source: "features/fleet-management.md", Category: categoryAutomate},
	{Source: "features/formatting.md", Category: categoryOperate},
	{Source: "features/interfaces.md", Category: categoryServices},
	{Source: "features/interoperability-testing.md", Category: categoryObserve},
	{Source: "features/introspection.md", Category: categoryObserve},
	{Source: "features/looking-glass.md", Category: categoryOperate},
	{Source: "features/mcp-integration.md", Category: categoryAutomate},
	{Source: "features/web-interface.md", Category: categoryOperate},
	{Source: "guide/anomaly.md", Category: categorySecure},
	{Source: "guide/as112.md", Category: categoryServices},
	{Source: "guide/authentication.md", Category: categorySecure},
	{Source: "guide/authorization.md", Category: categorySecure},
	{Source: "guide/audit.md", Category: categorySecure},
	{Source: "guide/benchmarking.md", Category: categoryObserve},
	{Source: "guide/bfd.md", Category: categoryRouting},
	{Source: "guide/bmp.md", Category: categoryRouting},
	{Source: "guide/command-reference.md", Category: categoryOperate},
	{Source: "guide/config-editor.md", Category: categoryOperate},
	{Source: "guide/ddos-mitigation.md", Category: categorySecure},
	{Source: "guide/flowspec-protected-router.md", Category: categorySecure},
	{Source: "guide/flowspec-route-reflector.md", Category: categoryRouting},
	{Source: "guide/firewall.md", Category: categoryServices},
	{Source: "guide/flow-export.md", Category: categoryObserve},
	{Source: "guide/isis.md", Category: categoryRouting},
	{Source: "guide/ipsec.md", Category: categorySecure},
	{Source: "guide/irr-filtering.md", Category: categoryRouting},
	{Source: "guide/l2tp.md", Category: categoryServices},
	{Source: "guide/ospf.md", Category: categoryRouting},
	{Source: "guide/monitoring.md", Category: categoryObserve},
	{Source: "guide/mrt-analysis.md", Category: categoryObserve},
	{Source: "guide/netlab.md", Category: categoryObserve},
	{Source: "guide/policy-routing.md", Category: categoryServices},
	{Source: "guide/plugins.md", Category: categoryAutomate},
	{Source: "guide/pppoe.md", Category: categoryServices},
	{Source: "guide/production-diagnostics.md", Category: categoryObserve},
	{Source: "guide/health-checks.md", Category: categoryObserve},
	{Source: "guide/quickstart.md", Category: categoryOperate},
	{Source: "guide/radius.md", Category: categorySecure},
	{Source: "guide/route-injection.md", Category: categoryRouting},
	{Source: "guide/rpki.md", Category: categoryRouting},
	{Source: "guide/static-routes.md", Category: categoryRouting},
	{Source: "guide/looking-glass-howto.md", Category: categoryOperate},
	{Source: "guide/tacacs.md", Category: categorySecure},
	{Source: "guide/operator-access-rbac.md", Category: categorySecure},
	{Source: "guide/traffic-usage.md", Category: categoryObserve},
	{Source: "guide/terminal-demonstrations.md", Category: categoryObserve},
	{Source: "guide/vrrp.md", Category: categoryRouting},
	{Source: "guide/vpp.md", Category: categoryServices},
	{Source: "guide/appliance.md", Category: categoryPlatform},
	{Source: "guide/ze-install.md", Category: categoryPlatform},
	{Source: "guide/ubuntu-build-install.md", Category: categoryPlatform},
	{Source: "performance.md", Category: categoryObserve},
	{Source: "research/vpp-deployment-reference.md", Category: categoryServices},
	{Source: "architecture/command-ownership.md", Category: categoryAutomate},
	{Source: "architecture/route-selection.md", Category: categoryRouting},
	{Source: "architecture/system-architecture.md", Category: categoryPlatform},
	{Source: "architecture/testing/ci-format.md", Category: categoryObserve},
	{Source: "contributing/documentation-testing.md", Category: categoryObserve},
	{Source: "contributing/rfc-implementation-guide.md", Category: categoryRouting},
	{Source: "contributing/testing.md", Category: categoryObserve},
	{Source: "features/srv6.md", Category: categoryRouting},
	{Source: "glossary.md", Category: ""},
	{Source: "guide/add-path.md", Category: categoryRouting},
	{Source: "guide/api.md", Category: categoryAutomate},
	{Source: "guide/bgp-peering.md", Category: categoryRouting},
	{Source: "guide/bgp-policy.md", Category: categoryRouting},
	{Source: "guide/bgp-resilience.md", Category: categoryRouting},
	{Source: "guide/bgp-role.md", Category: categoryRouting},
	{Source: "guide/chaos-testing.md", Category: categoryObserve},
	{Source: "guide/cli.md", Category: categoryOperate},
	{Source: "guide/config-archive.md", Category: categoryOperate},
	{Source: "guide/configuration.md", Category: categoryOperate},
	{Source: "guide/debugging-tools.md", Category: categoryObserve},
	{Source: "guide/developer-setup.md", Category: categoryPlatform},
	{Source: "guide/environment-variables.md", Category: categoryOperate},
	{Source: "guide/fleet-config.md", Category: categoryAutomate},
	{Source: "guide/gnmi.md", Category: categoryAutomate},
	{Source: "guide/graceful-restart.md", Category: categoryRouting},
	{Source: "guide/healthcheck.md", Category: categoryObserve},
	{Source: "guide/lifecycle.md", Category: categoryOperate},
	{Source: "guide/logging.md", Category: categoryObserve},
	{Source: "guide/looking-glass.md", Category: categoryOperate},
	{Source: "guide/mcp/chaos.md", Category: categoryObserve},
	{Source: "guide/mcp/elicitation.md", Category: categoryAutomate},
	{Source: "guide/mcp/overview.md", Category: categoryAutomate},
	{Source: "guide/mcp/remote-access.md", Category: categorySecure},
	{Source: "guide/mpls.md", Category: categoryRouting},
	{Source: "guide/operations.md", Category: categoryOperate},
	{Source: "guide/redistribution.md", Category: categoryRouting},
	{Source: "guide/rsvp-te.md", Category: categoryRouting},
	{Source: "guide/self-update.md", Category: categoryPlatform},
	{Source: "guide/traffic-control.md", Category: categoryServices},
	{Source: "guide/web-interface.md", Category: categoryOperate},
	{Source: "history.md", Category: ""},
	{Source: "plugin-development.md", Category: categoryAutomate},
	{Source: "plugin-development/commands.md", Category: categoryAutomate},
	{Source: "plugin-development/handlers.md", Category: categoryAutomate},
	{Source: "plugin-development/metrics.md", Category: categoryObserve},
	{Source: "plugin-development/protocol.md", Category: categoryAutomate},
	{Source: "plugin-development/schema.md", Category: categoryAutomate},
	{Source: "plugin-development/testing.md", Category: categoryObserve},
	{Source: "why-ze.md", Category: ""},
}

// docsDestinationExact names the pages whose public role differs from their
// source family, so neither the source path nor a prefix answers their URL.
var docsDestinationExact = map[string]string{
	"architecture.md": "architecture",
	"architecture/config/deprecated-options.md": "reference/deprecations",
	"contributing/testing.md":                   "contribute/testing",
	"features.md":                               "reference/feature-status",
	"features/configuration.md":                 "features/bgp-configuration",
	"features/rfc-status.md":                    "reference/rfcs",
	"glossary.md":                               "reference/glossary",
	"guide/developer-setup.md":                  "contribute/developer-setup",
	"guide/configuration.md":                    "guides/configuration-model",
	"guide/health-checks.md":                    "guides/system-readiness",
	"guide/healthcheck.md":                      "guides/bgp-healthcheck",
	"guide/looking-glass-howto.md":              "guides/public-looking-glass",
	"guide/netlab.md":                           "labs/netlab",
	"guide/terminal-demonstrations.md":          "demos/terminal",
	"history.md":                                "project/history",
	"performance.md":                            "performance/bgp",
	"plugin-development.md":                     "developers/plugins",
	"research/vpp-deployment-reference.md":      "architecture/vpp-deployment",
	"why-ze.md":                                 "project/why-ze",
}

// docsDestinationPrefixes maps a coherent source family to its public family.
// The first match wins, so the order is part of the answer.
var docsDestinationPrefixes = []docsDestinationPrefix{
	{Source: "architecture/", Public: "architecture/"},
	{Source: "contributing/", Public: "contribute/"},
	{Source: sectionFeatures, Public: sectionFeatures},
	{Source: "guide/", Public: "guides/"},
	{Source: "plugin-development/", Public: "developers/plugins/"},
}

// hubPages are the five curated collection landing pages, whose sources live
// in the website tree rather than in docs/.
var hubPages = []sitePage{
	{Source: "website/guides/index.md", Dest: "guides/index.html", Desc: "Task-oriented installation, configuration, operation, and troubleshooting guides.", Category: categoryOperate},
	{Source: "website/reference/index.md", Dest: "reference/index.html", Desc: "Generated and lookup-oriented command, configuration, plugin, standards, and feature data."},
	{Source: "website/project/index.md", Dest: "project/index.html", Desc: "Ze roadmap, milestones, changes, history, talks, and contribution paths."},
	{Source: "website/developers/index.md", Dest: "developers/index.html", Desc: "Plugin SDK documentation and contributor development workflows.", Category: categoryAutomate},
	{Source: "website/demos/index.md", Dest: "demos/index.html", Desc: "Recorded workflows, runnable labs, netlab topologies, and deployment examples.", Category: categoryObserve},
}

// useCasePages are the deployment examples. Each one takes the same eyebrow,
// so the label is stated rather than derived from the destination.
var useCasePages = []sitePage{
	{Source: "website/use-cases/index.md", Dest: "use-cases/index.html", Desc: "Deployment examples for using Ze in a real network, with adjacent router configs and the lab evidence behind each setup.", Journey: journeyUseCase},
	{Source: "website/use-cases/as112/index.md", Dest: "use-cases/as112/index.html", Desc: "Use Ze as an AS112 anycast DNS node inside a network, with peer configs for FRR, BIRD, VyOS, Junos, and Cisco IOS XR.", Category: categoryServices, Journey: journeyUseCase},
	{Source: "website/use-cases/exabgp-migration/index.md", Dest: "use-cases/exabgp-migration/index.html", Desc: "Convert an ExaBGP config and run existing process scripts with Ze.", Category: categoryAutomate, Journey: journeyUseCase},
	{Source: "website/use-cases/bgp-performance/index.md", Dest: "use-cases/bgp-performance/index.html", Desc: "Route-server performance tests with ze-perf sender, receiver, and JSON reports.", Category: categoryAutomate, Journey: journeyUseCase},
	{Source: "website/use-cases/route-server/index.md", Dest: "use-cases/route-server/index.html", Desc: "Deploy Ze as an IXP route server with member policy, validation, and checks.", Category: categoryRouting, Journey: journeyUseCase},
	{Source: "website/use-cases/transit-edge-rpki/index.md", Dest: "use-cases/transit-edge-rpki/index.html", Desc: "Build a dual-transit Internet edge with RPKI origin validation and tested failover.", Category: categoryRouting, Journey: journeyUseCase},
	{Source: "website/use-cases/flowspec-injection/index.md", Dest: "use-cases/flowspec-injection/index.html", Desc: "Inject and withdraw FlowSpec rules through an authorized workflow with visible state and logs.", Category: categorySecure, Journey: journeyUseCase},
	{Source: "website/use-cases/chaos-tested-peering/index.md", Dest: "use-cases/chaos-tested-peering/index.html", Desc: "Check BGP convergence and recovery with deterministic chaos scenarios.", Category: categoryObserve, Journey: journeyUseCase},
	{Source: "website/use-cases/as-path-topology/index.md", Dest: "use-cases/as-path-topology/index.html", Desc: "Use the Looking Glass AS-path graph to investigate routing visibility and changes.", Category: categoryObserve, Journey: journeyUseCase},
}

// labDetailPages publish two docs/ sources that the manifest does not name,
// under the lab whose architecture they describe. They take no link manifest,
// so a cross-document link in them resolves to the code host.
var labDetailPages = []sitePage{
	{Source: "docs/labs/l2tp-interop.md", Dest: "labs/l2tp-interop/architecture/index.html", Desc: "Peer-isolated Docker lab details for L2TP, PPP, NCP, and kernel dataplane checks.", Category: categoryServices, Journey: "Lab details"},
	{Source: "docs/labs/pppoe-interop.md", Dest: "labs/pppoe-interop/architecture/index.html", Desc: "Peer-isolated Docker lab details for Ze PPPoE client interop with accel-ppp.", Category: categoryServices, Journey: "Lab details"},
}

// comparePages are the comparison pages, whose evidence tables carry the
// citation cells relayoutEvidenceCells lifts onto their own lines.
var comparePages = []sitePage{
	{Source: "website/compare/comparison.md", Dest: "compare/index.html", Desc: "Comparisons between Ze, BGP daemons, and router network operating systems.", Category: categoryRouting},
	{Source: "website/compare/bgp.md", Dest: "compare/bgp/index.html", Desc: "How Ze compares to mature BGP daemons, including current gaps.", Category: categoryRouting},
	{Source: "website/compare/nos.md", Dest: "compare/nos/index.html", Desc: "How Ze compares to VyOS and freeRtr as full router operating systems.", Category: categoryPlatform},
}

// qualityPages describe how Ze checks itself.
var qualityPages = []sitePage{
	{Source: "website/quality/quality.md", Dest: "quality/index.html", Desc: "How Ze checks code quality with unit tests, functional scenarios, QEMU, fuzzing, gomu mutation testing, and release checks.", Category: categoryObserve},
	{Source: "website/quality/functional-ci.md", Dest: "quality/functional-ci/index.html", Desc: "How to write and run Ze functional .ci tests.", Category: categoryObserve},
	{Source: "website/quality/browser-editor.md", Dest: "quality/browser-editor/index.html", Desc: "How to write and run Ze browser .wb tests and editor .et tests.", Category: categoryObserve},
	{Source: "website/quality/unit-fuzz-mutation.md", Dest: "quality/unit-fuzz-mutation/index.html", Desc: "How to write and run Ze unit tests, fuzz targets, and gomu mutation checks.", Category: categoryObserve},
	{Source: "website/quality/qemu-interop-release.md", Dest: "quality/qemu-interop-release/index.html", Desc: "How to run Ze QEMU tests, interop labs, deployment checks, performance tests, and release checks.", Category: categoryObserve},
	{Source: "website/quality/verify-debugging.md", Dest: "quality/verify-debugging/index.html", Desc: "How ze verify, failure routing, traces, and debug logging work.", Category: categoryObserve},
}

// oneShotPages are the pages the retired build rendered one call each, rather
// than through a family list. Two of them read the repository's own root
// documents, which is why the source paths here are not all under website/.
var oneShotPages = []sitePage{
	{
		Source:  "website/contribute/contribute.md",
		Dest:    "contribute/index.html",
		Desc:    "How to contribute to Ze: the CLA, how the project is funded, and where to start.",
		Journey: journeyCommunity,
	},
	{
		Source: "website/contribute/guide.md",
		Dest:   "contribute/guide/index.html",
		Desc:   "The practical side of contributing to Ze: how work gets in, how to build, and what a good change looks like.",
	},
	{
		Source: "website/docs/docs.md",
		Dest:   "docs/index.html",
		Desc:   "All Ze documentation, organized by what you are trying to do: learn, do, look up, and understand.",
	},
	{
		Source:  "website/faq/faq.md",
		Dest:    "faq/index.html",
		Desc:    "The questions people ask before they spend time on Ze, answered honestly.",
		Journey: labelFAQ,
	},
	{
		Source:  "website/roadmap/roadmap.md",
		Dest:    "project/roadmap/index.html",
		Desc:    "The work between now and a first release of Ze you can trust in production.",
		Journey: "Release path",
	},
	{
		Source: "website/license/license.md",
		Dest:   "license/index.html",
		Desc:   "Ze is free software under the GNU Affero General Public License v3.",
	},
	{
		Source: "CODE_OF_CONDUCT.md",
		Dest:   "code-of-conduct/index.html",
		Desc:   "The code of conduct for the Ze community.",
	},
	{
		Source:   "SECURITY.md",
		Dest:     "security/index.html",
		Desc:     "How to report a security vulnerability in Ze, what is in scope, and what to expect.",
		Category: categorySecure,
	},
}
