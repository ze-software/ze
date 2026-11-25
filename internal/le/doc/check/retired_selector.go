// Design: docs/architecture/core-design.md -- native documentation verifier actions
// Related: retired.go -- shared historical record matcher (retiredRecords) and owned-row skipper
// Related: repository.go -- the tracked-file walk shared by links and retired sweeps
//
// The retired selector value sweep finds every whole-word occurrence of a
// retired data-plane selector value (netlink, nft, tc -> kernel) in every file
// git tracks, refusing any occurrence not covered by a declared file class
// (F-1 to F-4), occurrence class (O-1 to O-7), or line exception. The retired
// values and their successor kernel are declared once here. It exits 1 on any
// finding, 2 on an unreadable file or manifest, and runs as a stage of the
// full verify (internal/le/verify/engine/stages.go).

package doccheck

import (
	"bytes"
	"go/scanner"
	"go/token"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// The three retired data-plane selector values and their single successor,
// declared once in the sweep (AC-20 of spec-dataplane-compatibility-guard).
const (
	selectorValNetlink         = "netlink"
	selectorValNFT             = "nft"
	selectorValTC              = "tc"
	retiredSelectorReplacement = "kernel"
)

var retiredSelectorValues = [...]string{
	selectorValNetlink,
	selectorValNFT,
	selectorValTC,
}

// retiredSelectorNotZePrefixes declares file class F-2: vendored code, copied
// RFCs and references, and patches to Linux or vendored modules whose words Ze
// does not write.
var retiredSelectorNotZePrefixes = [...]string{
	"vendor/",
	"third_party/",
	"rfc/full/",
	"reference/",
	"gokrazy/kernel/patches/",
	"internal/le/vendorpatch/patches/",
}

// retiredSelectorSweepFiles and retiredSelectorSweepDir declare file class
// F-3: the sweep itself, its test, its fixture tree, and its design spec.
var retiredSelectorSweepFiles = [...]string{
	"internal/le/doc/check/retired_selector.go",
	"internal/le/doc/check/retired_selector_test.go",
	"plan/release/fixes/spec-dataplane-compatibility-guard.md",
}

const retiredSelectorSweepDir = "internal/le/doc/check/testdata/retired_selector/"

// retiredSelectorProofFiles declares file class F-4: the AC-19 tests that write
// an old selector value on purpose to prove it is refused.
var retiredSelectorProofFiles = [...]string{
	"internal/component/config/dataplane_retired_values_test.go",
	"test/parse/dataplane-refuses-old-backend-names.ci",
	"test/plugin/dataplane-old-backend-names-refuse-start.ci",
}

// retiredSelectorNetlinkWords, retiredSelectorNFTWords and
// retiredSelectorTCWords are the closed next-word lists of occurrence class
// O-5 (and O-7 for nft and tc). Singular and plural forms are derived from
// these lists once at package initialization.
const retiredKindCommand = "command"

var retiredSelectorNetlinkWords = [...]string{
	"socket", "message", "error", "call", "api", "family", "attribute", "attr",
	"field", "library", "handle", "request", "dump", "subscription",
	"multicast", "monitor", "monitoring", "event", "receive", "round",
	"round-trip", "reply", "write", "read", "operation", "protocol", "link",
	"route", "address", "neighbor", "neighbour", //nolint:misspell // UK spelling is matched intentionally in mechanism prose
	"notification", "update",
	"deadline", "timeout", "failure",
}

var retiredSelectorNFTWords = [...]string{
	"table", "chain", "rule", "ruleset", "set", "expression", retiredKindCommand,
	"program", "binary", "tool", "list", "map", "element", "counter",
	"flowtable", "verdict", "syntax", "output",
}

var retiredSelectorTCWords = [...]string{
	"qdisc", "class", "filter", "action", "mirred", "ingress", "egress",
	"sample", "clsact", "matchall", "police", "u32", "flower", "bpf", "chain",
	"monitor", retiredKindCommand, "program", "binary", "tool",
}

// retiredSelectorNetlinkPrefixes is the closed preceding-word list of
// occurrence class O-6.
var retiredSelectorNetlinkPrefixes = map[string]bool{
	"via":     true,
	"through": true,
	"over":    true,
	"using":   true,
	"generic": true,
	"raw":     true,
	"rt":      true,
}

// mechanismWordsByValue maps each retired selector value to the lower-case
// singular and plural forms derived from its O-5 list.
var mechanismWordsByValue = map[string]map[string]bool{
	selectorValNetlink: buildMechanismWordSet(retiredSelectorNetlinkWords[:]),
	selectorValNFT:     buildMechanismWordSet(retiredSelectorNFTWords[:]),
	selectorValTC:      buildMechanismWordSet(retiredSelectorTCWords[:]),
}

// retiredSelectorException is one line-scoped exception for an occurrence that
// names a kernel mechanism, socket, DNS flag, acronym, or command rather than a
// data-plane selector value, in a shape no occurrence class covers.
type retiredSelectorException struct {
	file  string
	old   string
	lines []string
	why   string
}

const (
	selectorDoctorDepWhy        = "doctor dependency on the Linux netlink socket, not a selector value"
	selectorMonitorNetlinkWhy   = "the monitor system netlink command name or wire method, not a selector value"
	selectorDmesgNetlinkWhy     = "the dmesg filter for kernel netlink messages, not a selector value"
	selectorNftBinaryWhy        = "the Linux nftables nft(8) binary or CLI command, not a selector value"
	selectorTCBinaryWhy         = "the Linux iproute2 tc(8) binary or package name, not a selector value"
	selectorHWTCOffloadWhy      = "the Linux ethtool hw-tc-offload feature and YANG leaf name, not a selector value"
	selectorN1DecisionWhy       = "the verbatim N-1 design decision row naming the retired selector values"
	selectorStrongSwanPluginWhy = "strongSwan's upstream kernel-netlink plugin name, not a selector value"
	selectorVendorPatchWhy      = "the vendored vishvananda/netlink XFRM patch or Go library name, not a selector value"
	selectorIfaceNetlinkNameWhy = "the iface-netlink plugin directory or catalog name, not a selector value"
	selectorExclusiveGroupWhy   = "the functional test exclusive group name, not a selector value"
	selectorDNSFlagTCWhy        = "the DNS header truncated (TC) flag name, not a selector value"
	selectorPolicyRouteCodeWhy  = "the policyroute-netlink doctor check or diagnostic code, not a selector value"
	selectorKernelConfigWhy     = "Linux kernel Kconfig netlink subsystem comment, not a selector value"
	selectorSwaggerAssetWhy     = "vendored third-party Swagger UI minified asset token, not a selector value"
	selectorSiteFixtureWhy      = "frozen published-site renderer fixture text, not a selector value"
	selectorStorageTCWhy        = "the tc storage-consumer restart fixture arm or output, not a selector value"
	selectorKernelProbeWhy      = "the kernel-capabilities JSON subsystem name for netlink, not a selector value"
	selectorHistoricalSpecWhy   = "historical spec slug, not a selector value"
	selectorGoPackageNetlinkWhy = "Go package or symbol fixture for the netlink package, not a selector value"
)

const (
	selectorCoreDesignFile            = "docs/architecture/core-design.md"
	selectorQemuIntegrationFile       = "docs/architecture/testing/qemu-integration.md"
	selectorFleetRegistrySpecFile     = "plan/release/features/spec-fleet-1-device-registry.md"
	selectorBGPNextHopSpecFile        = "plan/release/fixes/spec-bgp-next-hop-auto-rewrites-for-ebgp.md"
	selectorExplicitPluginSpecFile    = "plan/release/fixes/spec-explicit-plugin-and-attach-config.md"
	selectorDoctorForwardingSpecFile  = "plan/release/validation/spec-doctor-coverage-10-forwarding.md"
	selectorFibDummySpecFile          = "plan/release/features/spec-fib-dummy.md"
	selectorFibLabelSpecFile          = "plan/release/fixes/spec-fib-kernel-label-ownership.md"
	selectorFleetTemplatesSpecFile    = "plan/post-release/features/spec-fleet-2-config-templates.md"
	selectorFleetAuditSpecFile        = "plan/post-release/features/spec-fleet-3-audit-trail.md"
	selectorFleetRolloutSpecFile      = "plan/post-release/features/spec-fleet-5-staged-rollout.md"
	selectorFleetRelaySpecFile        = "plan/post-release/features/spec-fleet-9-device-command-relay.md"
	selectorFleetTabsSpecFile         = "plan/post-release/features/spec-fleet-ui-4-device-tabs-and-console.md"
	selectorFleetDriftSpecFile        = "plan/release/features/spec-fleet-10-drift-hash-chain.md"
	selectorFleetHealthSpecFile       = "plan/release/features/spec-fleet-4-inventory-health.md"
	selectorFleetMetricsSpecFile      = "plan/release/features/spec-fleet-8-metric-history.md"
	selectorFleetAlarmsSpecFile       = "plan/release/features/spec-fleet-ui-5-alarm-list.md"
	selectorPublishedYANGTreeFile     = "internal/le/site/testdata/published-yang-config-tree.json"
	selectorExclusiveGroupFirewallIRR = "option=exclusive:group=firewall-irr-nft"
	selectorExclusiveGroupFlowspec    = "option=exclusive:group=flowspec-nft"
	selectorN1DecisionLineCoreDesign  = "| Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`"
	selectorN1DecisionLineFleetSpec   = "| `docs/architecture/core-design.md` | Design decisions | Decision of 2026-11-23 | \"Every data-plane selector names the plane it programs"
	selectorN1DecisionLineBGPSpec     = "| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 on `core-design.md` | \"Every data-plane selector names the plane it programs"
	selectorN1DecisionLinePluginSpec  = "| `docs/architecture/core-design.md` | Design decisions | decision row on `docs/architecture/core-design.md` | \"Every data-plane selector names the plane it programs"
	selectorN1DecisionQuote           = "\"Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`"
)

// retiredSelectorExceptions declares the line exceptions of AC-20. Each entry
// covers only the exact trimmed line text it names in its own file.
var retiredSelectorExceptions = [...]retiredSelectorException{
	{
		file:  "internal/component/gtsm/doctor.go",
		old:   selectorValNetlink,
		lines: []string{`Dependencies: []string{"config-tree", "netlink"},`},
		why:   selectorDoctorDepWhy,
	},
	{
		file:  "internal/plugins/ospf/register.go",
		old:   selectorValNetlink,
		lines: []string{`Dependencies: []string{dependencyConfigTree, "netlink"},`},
		why:   selectorDoctorDepWhy,
	},
	{
		file: "internal/plugins/policyroute/doctor_linux.go",
		old:  selectorValNetlink,
		lines: []string{
			`Dependencies: []string{"netlink"},`,
			`Name:         "policyroute-netlink",`,
		},
		why: "policyroute-netlink doctor check name and netlink socket dependency, not a selector value",
	},
	{
		file:  "internal/test/fixture/storage_consumer_restart.go",
		old:   selectorValTC,
		lines: []string{`return storageConsumerRestartArm(ctx, "tc", storageTCRestart)`},
		why:   selectorStorageTCWhy,
	},
	{
		file: "internal/test/dnsquery/query.go",
		old:  selectorValTC,
		lines: []string{
			`var flagNames = [...]string{"qr", "aa", "tc", "rd", "ra", "ad", "cd"}`,
			"flags <the ones set, of qr aa tc rd ra ad cd>",
		},
		why: selectorDNSFlagTCWhy,
	},
	{
		file:  "internal/le/site/plugins.go",
		old:   selectorValTC,
		lines: []string{`"sr": "SR", "tc": "TC", "te": "TE", "vpp": "VPP", "yang": "YANG",`},
		why:   "the plugin catalog acronym map entry for TC, not a selector value",
	},
	{
		file: "internal/component/iface/cmd/monitor_netlink.go",
		old:  selectorValNetlink,
		lines: []string{
			`pluginserver.RegisterStreamingHandler("monitor system netlink", pluginserver.OneShotStreamingOnly, streamNetlinkMonitor)`,
			`tb.Str("unknown netlink group (valid: ").Join(netlinkGroups, ", ").Byte(')')`,
			`WireMethod: "ze-iface:monitor-system-netlink",`,
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file: "internal/component/iface/yang/ze-iface-monitor-cmd.yang",
		old:  selectorValNetlink,
		lines: []string{
			`description "The monitor system netlink command tree, owned by the iface component.";`,
			`revision 2026-06-06 { description "Relocated monitor system netlink out of the central monitor schema (plugin self-containment)."; }`,
			"container netlink {",
			`ze:command "ze-iface:monitor-system-netlink";`,
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file: "internal/component/ike/engine/dynamic_wan.go",
		old:  selectorValNetlink,
		lines: []string{
			`"fix", "none if the next line reports the watch running; if this repeats, look for the netlink error ze logged before it, and for kernel messages with: dmesg | grep -i netlink")`,
			`"fix", "none if the watch starts on the next try; if this repeats, look for kernel messages with: dmesg | grep -i netlink")`,
		},
		why: selectorDmesgNetlinkWhy,
	},
	{
		file:  ".github/workflows/qemu-nightly.yml",
		old:   selectorValNFT,
		lines: []string{"# The two namespace steps below need only setcap, getcap, nft and ip, which"},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "docs/architecture/api/commands.md",
		old:  selectorValNetlink,
		lines: []string{
			"| Monitor | monitor bgp (TUI dashboard), monitor event (live event streaming), monitor system netlink (kernel events) |",
			"| `OneShotStreamingOnly` | Not offered: the stream is a sequence of events, so no single answer holds its data. Typed in full, the command is refused with `StreamingOnlyError`, which names `ze <command>` and SSH, the places it streams | `monitor bgp`, `monitor event`, `monitor system netlink`, `monitor vpn ipsec` |",
			"monitor system netlink                   # All netlink groups (route + link + address)",
			"monitor system netlink all               # Explicit all (same as no argument)",
			"Wire method: `ze-iface:monitor-system-netlink`. Linux only; returns \"not available on this platform\" on other OSes.",
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file: "docs/architecture/appliance/docker-host.md",
		old:  selectorValNFT,
		lines: []string{
			"a static musl sudo, legacy iptables, iproute2's `ip`, nftables' `nft`, libcap's",
			"replaces busybox's `ip` alias; `nft`, `setcap` and `getcap` are also on",
			"kernel carries the legacy tables it uses. The image's `nft` is for functional",
			"`ip`, `nft`, `setcap` and `getcap` as root-owned executable files. A separate",
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  selectorCoreDesignFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionLineCoreDesign},
		why:   selectorN1DecisionWhy,
	},
	{
		file: selectorCoreDesignFile,
		old:  selectorValNFT,
		lines: []string{
			selectorN1DecisionLineCoreDesign,
			`| The VPP firewall verifier, not a YANG annotation, decides whether ` + "`connection-state`" + ` and ` + "`mark-set`" + ` are accepted on VPP. The ` + "`ze:backend`" + ` annotation of ` + "`flow-offload`" + ` is deleted with theirs; ` + "`flow-offload`" + ` stays refused on VPP because VPP has no flowtable and the ` + "`flowtable`" + ` list keeps its ` + "`ze:backend \"nft\"`" + ` annotation. | A ` + "`ze:backend`" + ` annotation that refuses ` + "`connection-state`" + ` or ` + "`mark-set`" + ` on VPP, or announcing any of ` + "`connection-state`" + `, ` + "`flow-offload`" + ` and ` + "`mark-set`" + ` as supported on VPP before a test on real VPP proves it. | plan/release/fixes/spec-dataplane-compatibility-guard.md | 2026-11-23 |`,
		},
		why: selectorN1DecisionWhy,
	},
	{
		file:  selectorCoreDesignFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionLineCoreDesign},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  "docs/architecture/firewall/table-ownership-and-shutdown-flush.md",
		old:   selectorValNFT,
		lines: []string{"**nft 1.1.1 renders a MASKED source-address match raw.** `ip saddr"},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "docs/architecture/iface/netlink-monitor.md",
		old:   selectorValNetlink,
		lines: []string{"`monitor system netlink` streams kernel route, link and address changes. A"},
		why:   selectorMonitorNetlinkWhy,
	},
	{
		file: "docs/architecture/iface/offload.md",
		old:  selectorValTC,
		lines: []string{
			"Per-interface ethtool offloads (GRO, GSO, SG, TSO, LRO, hw-tc-offload) and",
			"| hw-tc-offload | the generic `ETHTOOL_SFEATURES` API (string-set lookup, then bit manipulation) |",
			"hw-tc-offload has no legacy ioctl, so it pays for the variable-length",
		},
		why: selectorHWTCOffloadWhy,
	},
	{
		file:  "docs/architecture/ike/ipsec-7-ikev2-engine.md",
		old:   selectorValNetlink,
		lines: []string{"an `RTM_GETROUTE` request, as strongSwan's kernel-netlink `get_source_addr` does,"},
		why:   selectorStrongSwanPluginWhy,
	},
	{
		file:  "docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md",
		old:   selectorValNetlink,
		lines: []string{"<!-- source: internal/le/vendorpatch/patches/ -- netlink-xfrm-*.patch, XFRM state and policy decoding corrections -->"},
		why:   selectorVendorPatchWhy,
	},
	{
		file: "docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md",
		old:  selectorValNFT,
		lines: []string{
			"| `moving`, `FirewallMovingSince` ten seconds ago or longer | the ruleset has kept changing since at least the time it names, so bare ESP takes the complete path at its rate bound, at most 5000 datagrams per second; no restart is needed: the fast path comes back on once the commits pause long enough for one read, and that read finds no rule that can drop ESP; run the fix to see which tool keeps committing, then make that tool batch its changes, or stop it (fail2ban, for example) | `sudo nft monitor` |",
			"| An iptables compat expression (`xt`, which iptables-nft writes for an extension) | `xt compat` |",
			"an nftables `bridge` table is judged, and ebtables-nft writes one.",
			"Ze's, and the fix is the `nft insert` of an ESP accept at the head of that base",
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "docs/architecture/linux-limitations.md",
		old:   selectorValNetlink,
		lines: []string{"<!-- source: internal/le/vendorpatch/patches/ -- netlink-xfrm-*.patch, XFRM state and policy decoding corrections -->"},
		why:   selectorVendorPatchWhy,
	},
	{
		file: "docs/architecture/plugin/plugin-relationships.md",
		old:  selectorValNetlink,
		lines: []string{
			"### iface-netlink",
			"OS kernel -> iface-netlink (monitor)",
		},
		why: selectorIfaceNetlinkNameWhy,
	},
	{
		file:  "docs/architecture/testing/interop.md",
		old:   selectorValNFT,
		lines: []string{"withdraws the FlowSpec itself. Empty-ruleset checks require nft's JSON response,"},
		why:   selectorNftBinaryWhy,
	},
	{
		file: selectorQemuIntegrationFile,
		old:  selectorValNFT,
		lines: []string{
			"to a tool the image lacks (the image ships `ip`, `nft`, `setcap`, `getcap`, sudo,",
			"| `//go:build integration && linux` | Test needs a host grant (`/dev/ppp`), a binary `./le setup` does not install (`nft`), an explicitly supplied `ze` executable, or a privilege only the initial user namespace holds (`SO_RCVBUFFORCE`, an nftables payload write, BPF memlock) | `l2tp/bridge_integration_linux_test.go` |",
			"| Firewall table | nftables in netns | `nft add table ip ze_test` in an isolated namespace |",
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "docs/architecture/traffic/tc-original-qdisc-restore.md",
		old:   selectorValTC,
		lines: []string{"adapter does, and needs no `tc` binary: the guest's busybox `tc` cannot replace"},
		why:   selectorTCBinaryWhy,
	},
	{
		file:  "docs/contributing/testing.md",
		old:   selectorValNFT,
		lines: []string{"binary `./le setup` does not install (`nft`), needs a host grant (`/dev/ppp`,"},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "docs/contributing/writing-style.md",
		old:   selectorValNetlink,
		lines: []string{"| Technical nouns of this domain: `peer`, `prefix`, `NLRI`, `netlink`, `teardown` | A domain noun is precise. Keep it, and keep one spelling |"},
		why:   "writing-style table example of a technical domain noun, not a selector value",
	},
	{
		file: "docs/functional-tests.md",
		old:  selectorValNFT,
		lines: []string{
			"The firewall-irr cluster (`option=exclusive:group=firewall-irr-nft`) contends on",
			"The FlowSpec cluster (`option=exclusive:group=flowspec-nft`) is the ratchet's",
		},
		why: selectorExclusiveGroupWhy,
	},
	{
		file:  "docs/functional-tests.md",
		old:   selectorValTC,
		lines: []string{"| `flags <bits>` | the header bits set, in wire order, from `qr aa tc rd ra ad cd` |"},
		why:   selectorDNSFlagTCWhy,
	},
	{
		file:  "docs/guide/command-reference.md",
		old:   selectorValNetlink,
		lines: []string{"monitor system netlink [route|link|address|all]"},
		why:   selectorMonitorNetlinkWhy,
	},
	{
		file:  "docs/guide/configuration.md",
		old:   selectorValTC,
		lines: []string{"| `hw-tc-offload` | kernel ioctl | Hardware TC offload: NIC executes TC rules in hardware |"},
		why:   selectorHWTCOffloadWhy,
	},
	{
		file:  "docs/guide/configuration.md",
		old:   selectorValNFT,
		lines: []string{"`nft delete table ip6 ze_iface_rpf` command to remove a table a privileged run"},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "docs/guide/flowspec-protected-router.md",
		old:   selectorValNFT,
		lines: []string{"sudo nft delete table inet flowspec"},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "docs/guide/ipsec.md",
		old:  selectorValNFT,
		lines: []string{
			"`sudo nft 'insert rule inet \"filter\" \"input\" meta l4proto esp accept'`. When",
			"insert that rule with nft. A table created with the owner flag, which",
			"`protocol esp` and for a rule an operator added with `nft` alike, because Ze",
			"is `sudo nft monitor`, which shows each commit as it lands, so you can see which",
			"in nftables, or load it with iptables-nft, whose rules are nftables rules",
			"which Ze reads, or load it with ebtables-nft.",
			"- Ze learns of a commit another program makes (an `nft` command, firewalld)",
		},
		why: selectorNftBinaryWhy,
	},
	{
		file: "docs/guide/plugins.md",
		old:  selectorValNetlink,
		lines: []string{
			"| `iface-netlink` | Kernel backend for iface: manage, monitor, bridge, sysctl, mirror | -- (registered as iface backend) |",
			"<!-- source: internal/plugins/iface/netlink/register.go -- iface-netlink backend registration -->",
			"`backend` leaf, default `kernel`). The `iface-netlink` backend handles all Linux",
		},
		why: selectorIfaceNetlinkNameWhy,
	},
	{
		file:  "docs/guide/policy-routing.md",
		old:   selectorValNFT,
		lines: []string{"one. An element added to the firewall table's set with `nft` by hand is not"},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "docs/guide/production-diagnostics.md",
		old:  selectorValNetlink,
		lines: []string{
			"| Route/link/addr changes | `monitor system netlink all` |",
			"monitor system netlink all",
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file:  "docs/guide/vpp.md",
		old:   selectorValNetlink,
		lines: []string{"│                         (netlink)      (GoVPP)           │"},
		why:   "ASCII architecture diagram labeling the Linux netlink kernel interface beside GoVPP",
	},
	{
		file:  "docs/integrations/flowtriq-api.md",
		old:   selectorValNFT,
		lines: []string{"`iptables`, `ip6tables`, `ipset`, `sysctl`, `nft`, `ufw`, `firewall-cmd`,"},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "docs/integrations/flowtriq-api.md",
		old:   selectorValTC,
		lines: []string{"`tc`, `ip route`, `fail2ban-client`, `nginx`, `apache2ctl`."},
		why:   selectorTCBinaryWhy,
	},
	{
		file: "docs/research/qos-map.md",
		old:  selectorValTC,
		lines: []string{
			"Arista uses CoS for the PCP field and traffic-class (TC) for internal priority. Both are 0-7. The two maps are called cos-tc (ingress) and tc-cos (egress).",
			"The port must trust CoS for the cos-tc map to take effect. Sub-interfaces inherit from the parent.",
		},
		why: "Arista EOS cos-tc and tc-cos QoS map names, not a selector value",
	},
	{
		file:  "features/core-diagnostics.md",
		old:   selectorValNetlink,
		lines: []string{"`monitor system netlink` streams kernel route/link/address change events as JSON (replaces `ip monitor`)."},
		why:   selectorMonitorNetlinkWhy,
	},
	{
		file:  "features/interfaces.md",
		old:   selectorValTC,
		lines: []string{"per-interface offload and steering (GRO, GSO, SG, TSO, LRO, hw-tc-offload via kernel ethtool ioctl;"},
		why:   selectorHWTCOffloadWhy,
	},
	{
		file:  "gokrazy/kernel/kernel.config",
		old:   selectorValNetlink,
		lines: []string{"# Netlink and xfrm userspace integration."},
		why:   selectorKernelConfigWhy,
	},
	{
		file:  "gokrazy/kernel/kernel.require",
		old:   selectorValNetlink,
		lines: []string{"#   CONFIG_NFT_COMPAT   xtables compatibility. ze speaks netlink directly and"},
		why:   selectorKernelConfigWhy,
	},
	{
		file: "gokrazy/kernel/runtime.config",
		old:  selectorValNetlink,
		lines: []string{
			"# L2TP_NETLINK was removed in kernel 7.0 (netlink is always built with L2TP).",
			"# Ze IPsec dataplane. kernel.config carries CONFIG_XFRM_USER, which is the netlink",
			"# Conntrack netlink, for flow-export's conntrack reader and the DDoS",
		},
		why: selectorKernelConfigWhy,
	},
	{
		file:  "internal/appliance/dockerhost/guest_linux.go",
		old:   selectorValNFT,
		lines: []string{`"/usr/sbin/nft":    shippedDir + "/nft",`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/appliance/dockerhost/guest_tools_linux_test.go",
		old:   selectorValNFT,
		lines: []string{`"/usr/sbin/nft":    "nft",`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/component/api/rest/assets/swagger-ui-bundle.js",
		old:   selectorValTC,
		lines: []string{`!function webpackUniversalModuleDefinition(s,o){"object"==typeof exports&&"object"==typeof module`},
		why:   selectorSwaggerAssetWhy,
	},
	{
		file:  "internal/component/api/rest/assets/swagger-ui.css",
		old:   selectorValTC,
		lines: []string{".swagger-ui{color:#3b4151;container-name:swagger-ui;container-type:inline-size;/*! normalize.css v7.0.0 | MIT License | github.com/necolas/normalize.css */"},
		why:   selectorSwaggerAssetWhy,
	},
	{
		file:  "internal/component/bgp/reactor/link_scope_neighbors.go",
		old:   selectorValNetlink,
		lines: []string{`return "none if the watch starts again, which ze logs; if this repeats, look for the netlink error ze logged just before this line, and for kernel messages with: dmesg | grep -i -e netlink -e neigh"`},
		why:   selectorDmesgNetlinkWhy,
	},
	{
		file:  "internal/component/bgp/reactor/reactor_address_watch.go",
		old:   selectorValNetlink,
		lines: []string{`return "none if the watch starts again, which ze logs; if this repeats, look for the netlink error ze logged just before this line, and for kernel messages with: dmesg | grep -i netlink"`},
		why:   selectorDmesgNetlinkWhy,
	},
	{
		file:  "internal/component/cmd/monitor/yang/self_containment_test.go",
		old:   selectorValNetlink,
		lines: []string{"`\"ze-iface:monitor-system-netlink\"`: \"netlink monitor -> internal/component/iface/yang\","},
		why:   selectorMonitorNetlinkWhy,
	},
	{
		file: "internal/component/cmd/monitor/yang/ze-cli-monitor-cmd.yang",
		old:  selectorValNetlink,
		lines: []string{
			`revision 2026-06-06 { description "Relocated event monitor (-> command component) and system-netlink (-> iface component). Bare anchor only."; }`,
			"// NOTE: `monitor system netlink` is owned by the iface component.",
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file: "internal/component/doctor/doctor_test.go",
		old:  selectorValNetlink,
		lines: []string{
			`"doctor-policyroute-netlink",`,
			`"netlink/policyroute":     "doctor-policyroute-netlink",`,
		},
		why: selectorPolicyRouteCodeWhy,
	},
	{
		file:  "internal/component/firewall/engine.go",
		old:   selectorValNFT,
		lines: []string{`"error", err, "fix", "nft list tables, then nft delete table <family> <ze_name>")`},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "internal/component/iface/cmd/monitor_netlink_test.go",
		old:  selectorValNetlink,
		lines: []string{
			`t.Fatal("monitor system netlink not registered as streaming handler")`,
			`h, _ := streamingLookup(t, "monitor system netlink")`,
			`if !pluginserver.IsStreamingCommand("monitor system netlink") {`,
			`t.Error("monitor system netlink should be detected as streaming command")`,
			`if r.WireMethod == "ze-iface:monitor-system-netlink" {`,
			`t.Error("ze-iface:monitor-system-netlink handler must not be nil")`,
			`t.Fatal("ze-iface:monitor-system-netlink not registered via pluginserver.RegisterRPCs")`,
			`if err.Error() != "unknown netlink group (valid: route, link, address, all)" {`,
			"// netlink` reaches the group the streaming handler subscribes to. The plugin",
			"// fixture monitor-system-netlink proved this through the one-shot echo",
			`{"monitor system netlink", netlinkGroupAll},`,
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file:  "internal/component/iface/config.go",
		old:   selectorValTC,
		lines: []string{`o.HWTCOffload = parseBool("hw-tc-offload")`},
		why:   selectorHWTCOffloadWhy,
	},
	{
		file:  "internal/component/iface/config_test.go",
		old:   selectorValTC,
		lines: []string{`"hw-tc-offload": "true",`},
		why:   selectorHWTCOffloadWhy,
	},
	{
		file: "internal/component/iface/offload_linux.go",
		old:  selectorValTC,
		lines: []string{
			`if err := setEthtoolFeatureByName(fd, ifaceName, "hw-tc-offload", *cfg.HWTCOffload); err != nil {`,
			`log.Warn("iface offload: hw-tc-offload", "iface", ifaceName, "err", err)`,
			"// feature. Used for hw-tc-offload which has no legacy per-feature ioctl.",
		},
		why: selectorHWTCOffloadWhy,
	},
	{
		file:  "internal/component/iface/resolve_test.go",
		old:   selectorValNetlink,
		lines: []string{`if strings.Contains(pkg, "netlink") || strings.Contains(pkg, "vishvananda") || pkg == "net" {`},
		why:   selectorVendorPatchWhy,
	},
	{
		file: "internal/component/iface/yang/show_cmd_schema_test.go",
		old:  selectorValNetlink,
		lines: []string{
			"// self-containment invariant for `monitor system netlink`: the central",
			"`ze:command \"ze-iface:monitor-system-netlink\"`,",
			`"container netlink",`,
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file: "internal/component/iface/yang/ze-iface-conf.yang",
		old:  selectorValTC,
		lines: []string{
			"leaf hw-tc-offload {",
			`tc-offload on|off'.";`,
		},
		why: selectorHWTCOffloadWhy,
	},
	{
		file:  "internal/component/ike/dataplane/espform_fastpath_gate_linux.go",
		old:   selectorValNFT,
		lines: []string{`cmd.Str("sudo nft 'insert rule ").Str(site.Family).Str(" \"").Str(site.Table).Str("\" \"").Str(site.Chain).`},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "internal/component/ike/dataplane/espform_fastpath_gate_linux_test.go",
		old:  selectorValNFT,
		lines: []string{
			"want:   `sudo nft 'insert rule inet \"filter\" \"input\" meta l4proto esp accept'`,",
			"want:   `sudo nft 'insert rule ip \"my $(rm -rf) table\" \"in;put *\" meta l4proto esp accept'`,",
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "internal/component/ike/dataplane/espform_state.go",
		old:   selectorValNFT,
		lines: []string{`const ESPFormFixMoving = "sudo nft monitor"`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/component/ike/dataplane/policy_final.go",
		old:   selectorValNetlink,
		lines: []string{"// strongSwan reaches the same kernel state by another route: its kernel-netlink plugin"},
		why:   selectorStrongSwanPluginWhy,
	},
	{
		file: "internal/component/ike/engine/doctor_esp_form_fast_path_test.go",
		old:  selectorValNFT,
		lines: []string{
			"acceptFix := `sudo nft 'insert rule inet \"filter\" \"input\" meta l4proto esp accept'`",
			`fix:   "sudo nft monitor",`,
			`lacks: []string{"sudo nft", rewritten},`,
			`lacks: []string{"set firewall", "sudo nft", "accept-esp"},`,
			`lacks: []string{"set firewall", "sudo nft"},`,
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "internal/component/ike/engine/mobike_route_linux.go",
		old:   selectorValNetlink,
		lines: []string{"// It asks the routing table itself (RTM_GETROUTE), as strongSwan's kernel-netlink"},
		why:   selectorStrongSwanPluginWhy,
	},
	{
		file:  "internal/component/plugin/all/testdata/wire-methods.snapshot",
		old:   selectorValNetlink,
		lines: []string{"ze-iface:monitor-system-netlink"},
		why:   selectorMonitorNetlinkWhy,
	},
	{
		file:  "internal/core/slogutil/slogutil_test.go",
		old:   selectorValTC,
		lines: []string{`"tab and equals":       "a=b\tc=d",`},
		why:   "tab escape \\t followed by c=d in a log-quoting test string, not a selector value",
	},
	{
		file: "internal/exabgp/migration/migrate_flow_traffic_class_test.go",
		old:  selectorValTC,
		lines: []string{
			"route tc {",
			`for _, word := range []string{"flow route tc", tc.match, strings.TrimPrefix(tc.want, "traffic-class "), "RFC 8955 Section 4.2.2.11"} {`,
		},
		why: "ExaBGP FlowSpec tc (traffic-class) syntax (RFC 8955), not a selector value",
	},
	{
		file:  "internal/le/doc/check/retired.go",
		old:   selectorValNFT,
		lines: []string{"lines: []string{\"| Firewall table | nftables in netns | `nft add table ip ze_test` in an isolated namespace |\"},"},
		why:   "retired-command exception quoting the nft add table line of qemu-integration.md",
	},
	{
		file: "internal/le/doc/check/retired_test.go",
		old:  selectorValNFT,
		lines: []string{
			"\"docs/architecture/testing/qemu-integration.md\": \"| Firewall table | nftables in netns | `nft add table ip ze_test` in an isolated namespace |\\n\",",
			`bothBody     = "./le test-unit all\nbackend nft\n"`,
			"`  \"status\": \"./le test-unit all and backend nft\",`,",
		},
		why: "retired-command and manifest-pin test fixtures proving pinned snapshots are skipped",
	},
	{
		file:  "internal/le/interoplab/bgp/check_dns_resolver_predicate.go",
		old:   selectorValTC,
		lines: []string{`if !slices.Contains(got.flags, "tc") {`},
		why:   selectorDNSFlagTCWhy,
	},
	{
		file:  "internal/le/interoplab/bgp/check_dns_resolver_predicate_test.go",
		old:   selectorValTC,
		lines: []string{`tc, err := parseKdig(strings.Replace(kdigReply, "qr rd ra", "qr tc rd ra", 1))`},
		why:   selectorDNSFlagTCWhy,
	},
	{
		file:  "internal/le/interoplab/bgp/names.go",
		old:   selectorValNFT,
		lines: []string{`cmdNft           = "nft"`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/le/interoplab/bgp/prepare_link_local_test.go",
		old:   selectorValNFT,
		lines: []string{`if strings.Contains(scripts[peerFRR], "nft") {`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/le/interoplab/ipsec/dftreatment.go",
		old:   selectorValNFT,
		lines: []string{`dfCounterCommand = "nft"`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/le/interoplab/ipsec/unprotected_response.go",
		old:   selectorValNFT,
		lines: []string{`authHoldCommand = "nft"`},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "internal/le/interoplab/kernelcheck_unknown_proof_test.go",
		old:  selectorValNetlink,
		lines: []string{
			"answer := `{\"ready\":false,\"capabilities\":[{\"subsystem\":\"xfrm-interface\",\"kernel\":\"CONFIG_XFRM_INTERFACE\",\"state\":\"unknown\",\"reason\":\"netlink probe: operation not permitted\"}]}`",
			`for _, want := range []string{"could not be checked", release, "xfrm-interface", "CONFIG_XFRM_INTERFACE", "unknown", "netlink probe: operation not permitted", "/ze doctor --json kernel-capabilities"} {`,
		},
		why: selectorKernelProbeWhy,
	},
	{
		file:  "internal/le/interoplab/rsvpte/rsvpte_test.go",
		old:   selectorValTC,
		lines: []string{"const labeledSample = `07:08:21.000000 MPLS (label 16, tc 0, [S], ttl 63)"},
		why:   "tcpdump MPLS Traffic Class (tc) output field, not a selector value",
	},
	{
		file: "internal/le/rfc/discriminate_kernel_test.go",
		old:  selectorValNetlink,
		lines: []string{
			`files[kernelFixtureNetlink] = "package netlink\n\ntype netlinkBackend struct{}\n\n" +`,
			`{"a unit importing netlink with no exclusion", kernelImportingFiles(), kernelEscape(), ProofEscapeUnfounded},`,
			`{"a unit importing netlink with its exclusion", kernelImportingFiles(), excluded, ProofVerified},`,
		},
		why: selectorGoPackageNetlinkWhy,
	},
	{
		file:  "internal/le/setup/docker/host/static.go",
		old:   selectorValNFT,
		lines: []string{`"getcap", "ip", "nft", "setcap",`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/le/setup/docker/host/static_test.go",
		old:   selectorValNFT,
		lines: []string{`for _, name := range []string{"ip", "nft", "setcap", "getcap"} {`},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "internal/le/setup/vendor_real_publication_test.go",
		old:  selectorValNetlink,
		lines: []string{
			`nativeVendorUpstream = "package netlink\nconst Upstream = 42\n"`,
			`nativeVendorPatched  = "package netlink\nconst Upstream = 43\n"`,
			`body := fixtureStamp + "diff --git a/" + target + " b/" + target + "\n--- a/" + target + "\n+++ b/" + target + "\n@@ -1,2 +1,2 @@\n package netlink\n-const Upstream = 42\n+const Upstream = 43\n"`,
		},
		why: selectorGoPackageNetlinkWhy,
	},
	{
		file:  "internal/le/setup/vendor_stale_preservation_test.go",
		old:   selectorValNetlink,
		lines: []string{`write(filepath.Join(upstream, "upstream.go"), "package netlink\nconst Upstream = 42\n")`},
		why:   selectorGoPackageNetlinkWhy,
	},
	{
		file:  "internal/le/site/testdata/published-command-paths.txt",
		old:   selectorValNetlink,
		lines: []string{"monitor system netlink"},
		why:   selectorMonitorNetlinkWhy,
	},
	{
		file: "internal/le/site/testdata/published-configuration.md",
		old:  selectorValTC,
		lines: []string{
			"- **hw-tc-offload** `boolean`: Hardware Traffic Control Offload: run TC filter rules in the NIC.",
			"The feature lets the NIC run TC flower and u32 filter rules in hardware, so a matched flow does not reach kernel processing. It needs NIC firmware support, which is common on mlx5, bnxt and nfp. Ze applies it through the kernel ioctl ETHTOOL_SFEATURES. It is equivalent to 'ethtool -K <dev> hw- tc-offload on|off'.",
		},
		why: selectorHWTCOffloadWhy,
	},
	{
		file: "internal/le/site/testdata/published-dependencies-body.html",
		old:  selectorValNetlink,
		lines: []string{
			`<input id="dep-search" type="search" placeholder="Filter dependencies (e.g. netlink, ssh, prometheus)..." aria-label="Filter dependencies" />`,
			`<tr><td><code class="concept-software">github.com/vishvananda/netns</code></td><td><code>v0.0.5</code></td><td><code>Apache-2.0</code></td><td>Vishvananda Ishaya, Docker, Inc.</td><td>Linux network-namespace handles, used alongside netlink for namespace-scoped route watching.</td></tr>`,
		},
		why: selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-dependencies.json",
		old:   selectorValNetlink,
		lines: []string{`"why": "Linux network-namespace handles, used alongside netlink for namespace-scoped route watching.",`},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-dependencies.md",
		old:   selectorValNetlink,
		lines: []string{"| `github.com/vishvananda/netns` | `v0.0.5` | `Apache-2.0` | Vishvananda Ishaya, Docker, Inc. | Linux network-namespace handles, used alongside netlink for namespace-scoped route watching. |"},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-features-body.html",
		old:   selectorValNetlink,
		lines: []string{`<span class="chip">Netlink</span>`},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-features.json",
		old:   selectorValNetlink,
		lines: []string{`"text": "Netlink",`},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-features.md",
		old:   selectorValNetlink,
		lines: []string{"*services / Experimental* -- `Netlink` `DHCP`"},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-milestones-body.html",
		old:   selectorValTC,
		lines: []string{"<p>nftables and tc-netlink backends sharing one YANG data model, with <code>show firewall</code> and <code>show traffic-control</code> commands.</p>"},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-milestones-body.html",
		old:   selectorValNetlink,
		lines: []string{"<p>nftables and tc-netlink backends sharing one YANG data model, with <code>show firewall</code> and <code>show traffic-control</code> commands.</p>"},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-milestones.json",
		old:   selectorValTC,
		lines: []string{"\"blurb\": \"nftables and tc-netlink backends sharing one YANG data model, with `show firewall` and `show traffic-control` commands.\""},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-milestones.json",
		old:   selectorValNetlink,
		lines: []string{"\"blurb\": \"nftables and tc-netlink backends sharing one YANG data model, with `show firewall` and `show traffic-control` commands.\""},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-milestones.md",
		old:   selectorValTC,
		lines: []string{"nftables and tc-netlink backends sharing one YANG data model, with `show firewall` and `show traffic-control` commands."},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-milestones.md",
		old:   selectorValNetlink,
		lines: []string{"nftables and tc-netlink backends sharing one YANG data model, with `show firewall` and `show traffic-control` commands."},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-plugin-registry.json",
		old:   selectorValTC,
		lines: []string{`"description": "Traffic control (tc) qdisc, class, and filter management",`},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-plugin-registry.json",
		old:   selectorValNFT,
		lines: []string{`"description": "DDoS local responder: on-host nft drop on attack detection",`},
		why:   selectorSiteFixtureWhy,
	},
	{
		file: "internal/le/site/testdata/published-plugins-index-body.html",
		old:  selectorValNFT,
		lines: []string{
			`<article class="card plugin-card tone-teal" id="plugin-ddos-local"`,
			`<p class="plugin-desc">DDoS local responder: on-host nft drop on attack detection</p>`,
		},
		why: selectorSiteFixtureWhy,
	},
	{
		file: "internal/le/site/testdata/published-plugins-index-body.html",
		old:  selectorValTC,
		lines: []string{
			`<article class="card plugin-card tone-sky" id="plugin-traffic"`,
			`<p class="plugin-desc">Traffic control (tc) qdisc, class, and filter management</p>`,
		},
		why: selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-plugins-index.md",
		old:   selectorValNFT,
		lines: []string{"| [`ddos-local`](ddos-local/index.md) | DDoS local responder: on-host nft drop on attack detection | `ddos/local` | `firewall` | `internal/plugins/ddos/local` |"},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  "internal/le/site/testdata/published-plugins-index.md",
		old:   selectorValTC,
		lines: []string{"| [`traffic`](traffic/index.md) | Traffic control (tc) qdisc, class, and filter management | `traffic/control` | None | `internal/component/traffic` |"},
		why:   selectorSiteFixtureWhy,
	},
	{
		file:  selectorPublishedYANGTreeFile,
		old:   selectorValTC,
		lines: []string{`"name": "hw-tc-offload",`},
		why:   selectorHWTCOffloadWhy,
	},
	{
		file: selectorPublishedYANGTreeFile,
		old:  selectorValNetlink,
		lines: []string{
			`"description": "The container holds one unit's IPv4 state. Ze installs the addresses\nthrough netlink. Ze turns each other leaf into a net.ipv4.conf.<iface>\nsysctl write, which it emits on the event bus for the sysctl plugin to\nperform.",`,
			`"description": "The container holds one unit's IPv6 state. Ze installs the addresses\nthrough netlink. Ze turns autoconf, accept-ra and forwarding into\nnet.ipv6.conf.<iface> sysctl writes, which it emits on the event bus for\nthe sysctl plugin to perform.",`,
		},
		why: `JSON-escaped \nthrough netlink mechanism phrase from ze-iface-conf.yang (O-6 covered in YANG), not a selector value`,
	},
	{
		file:  "dependency-ledger.txt",
		old:   selectorValNetlink,
		lines: []string{"golang.org/x/sys/unix\tinternal/core/iprule\tnetlink and route constants for the shared ip rule encoder (moved from policyroute)"},
		why:   "owner-approved dependency ledger row naming Linux netlink/route unix constants, not a selector value",
	},
	{
		file:  "internal/le/test/qemu/netns_linux.go",
		old:   selectorValNFT,
		lines: []string{`if err := requireGuestCommands("setcap", "getcap", "nft"); err != nil {`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/le/test/qemu/one_off_proof_test.go",
		old:   selectorValNFT,
		lines: []string{"nft() { :; }"},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "internal/le/test/qemu/run.go",
		old:  selectorValNFT,
		lines: []string{
			`"{ nft add table inet ztrack 2>/dev/null || true; }",`,
			`"{ nft 'add chain inet ztrack out { type filter hook output priority -150 ; policy accept ; }' 2>/dev/null || true; }",`,
			`"{ nft add rule inet ztrack out ct state new,established,related counter 2>/dev/null || true; }",`,
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "internal/plugins/iface/netlink/address_watch_linux.go",
		old:   selectorValNetlink,
		lines: []string{`"fix", "none if the watch starts again; if this repeats, raise net.core.rmem_max for an ENOBUFS error, and look for kernel messages with: dmesg | grep -i netlink")`},
		why:   selectorDmesgNetlinkWhy,
	},
	{
		file: "internal/plugins/iface/netlink/monitor_subscribe_test.go",
		old:  selectorValNetlink,
		lines: []string{
			"plain := regexp.MustCompile(`netlink\\.` + family + `Subscribe\\(`)",
			`t.Errorf("netlink.%sSubscribe( is called without options, so that socket keeps the kernel default receive buffer and the kernel drops what it cannot queue; use %sSubscribeWithOptions with ReceiveBufferSize", family, family)`,
			"withOptions := regexp.MustCompile(`netlink\\.` + family + `SubscribeWithOptions\\(`)",
			`t.Errorf("no netlink.%sSubscribeWithOptions( call found; this test reads the source and the shape it reads has changed", family)`,
		},
		why: selectorVendorPatchWhy,
	},
	{
		file:  "internal/plugins/iface/netlink/native_ppp_linux.go",
		old:   selectorValNetlink,
		lines: []string{`file := os.NewFile(uintptr(socket), "ze-original-ppp-netlink")`},
		why:   "OS file descriptor name for an AF_NETLINK socket, not a selector value",
	},
	{
		file:  "internal/plugins/iface/netlink/neighbor_watch_linux.go",
		old:   selectorValNetlink,
		lines: []string{`"fix", "none if the watch starts again; if this repeats, raise net.core.rmem_max for an ENOBUFS error, and look for kernel messages with: dmesg | grep -i -e netlink -e neigh")`},
		why:   selectorDmesgNetlinkWhy,
	},
	{
		file:  "internal/plugins/iface/netlink/rpf_linux.go",
		old:   selectorValNFT,
		lines: []string{"\"if a privileged run left it, remove it with `nft delete table ip6 \"+rpfTableName+\"`\","},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "internal/plugins/iface/netlink/vlanqoslab_integration_linux_test.go",
		old:  selectorValNFT,
		lines: []string{
			`exec.Command("nft", "delete", "table", family, table).Run() //nolint:errcheck,noctx // best-effort cleanup, runs after t.Context() is canceled`,
			`if _, err := exec.LookPath("nft"); err != nil {`,
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "internal/plugins/policyroute/doctor_linux_test.go",
		old:   selectorValNetlink,
		lines: []string{"// VALIDATES: doctor-policyroute-netlink at warning severity, carrying the"},
		why:   selectorPolicyRouteCodeWhy,
	},
	{
		file: "internal/plugins/policyroute/register_doctor_codes.go",
		old:  selectorValNetlink,
		lines: []string{
			"// doctor-policyroute-netlink` answers.",
			`const codePolicyRouteNetlink = "doctor-policyroute-netlink"`,
			`Examples:    []string{"ze doctor --json", "ze show diagnostic doctor-policyroute-netlink"},`,
		},
		why: selectorPolicyRouteCodeWhy,
	},
	{
		file:  "internal/test/dnsquery/query_test.go",
		old:   selectorValTC,
		lines: []string{`"flags qr aa tc rd ra ad cd\n",`},
		why:   selectorDNSFlagTCWhy,
	},
	{
		file: "internal/test/fixture/netfilter_fixture_firewall.go",
		old:  selectorValNFT,
		lines: []string{
			`commandIgnore(ctx, "nft", "delete", "table", "inet", "surfprotect")`,
			`if _, err := netfilterCommandOutput(ctx, "nft", args...); err != nil {`,
			`defer commandIgnore(ctx, "nft", "delete", "table", "inet", "surfprotect")`,
			`defer commandIgnore(context.Background(), "nft", "delete", "table", "inet", table)`,
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "internal/test/fixture/plugin_fixture_07_flowspec.go",
		old:   selectorValNFT,
		lines: []string{`if output, err := exec.CommandContext(ctx, "nft", args...).CombinedOutput(); err != nil { //nolint:gosec // the fixture chooses the program and its arguments`},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "internal/test/fixture/plugin_fixture_07_irr_refusal_test.go",
		old:   selectorValNFT,
		lines: []string{`require.NoError(t, os.WriteFile(filepath.Join(bin, "nft"), []byte(script), 0o700))`},
		why:   selectorNftBinaryWhy,
	},
	{
		file: "internal/test/fixture/plugin_fixture_10.go",
		old:  selectorValNetlink,
		lines: []string{
			`"plugin/monitor-system-netlink":      {pluginName: "monitor-system-netlink-test", scenario: fixture10MonitorSystemNetlink},`,
			`for _, command := range []string{"monitor system netlink", "monitor system netlink route"} {`,
			`bogus := fixture10Call(ctx, plugin, "monitor system netlink bogus")`,
			`fmt.Fprintln(os.Stderr, "OK: monitor system netlink all/route refused, bogus group refused")`,
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file:  "internal/test/fixture/ui_fixture_le_evidence_vpp_answers.go",
		old:   selectorValNetlink,
		lines: []string{`fmt.Println("{\"ready\":true,\"capabilities\":[{\"subsystem\":\"netlink\",\"kernel\":\"CONFIG_NET\",\"state\":\"present\"}]}")`},
		why:   selectorKernelProbeWhy,
	},
	{
		file: "internal/test/runner/exclusive_group_test.go",
		old:  selectorValNFT,
		lines: []string{
			`{"plugin", "firewall-irr-*.ci", "option=needs-linux:caps=net-admin", "option=exclusive:group=firewall-irr-nft", "the node-wide nftables ruleset they all program", 12},`,
			`{"plugin", "flowspec-fw-*.ci", "option=needs-linux:caps=net-admin", "option=exclusive:group=flowspec-nft", "the one ze_flowspec table they each program in the node-wide nftables ruleset", 4},`,
			"{\"firewall\", \"firewall-legacy-table-*.ci\", \"option=needs-linux:caps=net-admin\", \"option=exclusive:group=flowspec-nft\", \"the `table inet flowspec` it seeds, which any concurrent ze daemon sweeps\", 1},",
		},
		why: selectorExclusiveGroupWhy,
	},
	{
		file:  "internal/test/runner/work_dir_test.go",
		old:   selectorValTC,
		lines: []string{`for _, binName := range []string{"ze", "gobgp", "bird", "tc", "curl"} {`},
		why:   selectorTCBinaryWhy,
	},
	{
		file:  "internal/test/toolstub/stub.go",
		old:   selectorValNetlink,
		lines: []string{"kernelProbeAnswer = `{\"ready\":true,\"capabilities\":[{\"subsystem\":\"netlink\",\"kernel\":\"CONFIG_NET\",\"state\":\"present\"}]}`"},
		why:   selectorKernelProbeWhy,
	},
	{
		file: "plan/post-release/engineering/spec-netlink-xfrm-open-upstream-prs.md",
		old:  selectorValNetlink,
		lines: []string{
			"# Spec: netlink-xfrm-open-upstream-prs",
			"| Depends | - (patch series prepared by `spec-netlink-xfrm-upstream-series`, landed in `b43120b371`, `c9ce6ba81d`, and `8afd1ff560`) |",
			"Homed from `spec-netlink-xfrm-upstream-series` (Phase 7 / AC-11) at its closure on 2026-11-23: opening upstream pull requests is outward-facing and runs only on Thomas's explicit order.",
		},
		why: selectorVendorPatchWhy,
	},
	{
		file: "plan/post-release/engineering/spec-validated-construction-and-state-types.md",
		old:  selectorValNetlink,
		lines: []string{
			"| `pluginserver.StreamingHandler` | `plugin/server/handler.go` | 5 (`monitor vpn ipsec`, `monitor event`, `monitor traffic stat`, `monitor interface rate`, `monitor system netlink`) | R8 SSH streaming (`cmd/ze/hub/service_ssh.go`, `cmd/ze/hub/api.go`) | Yes, since 62842be4c7: `GetStreamingHandlerForCommand` (`plugin/server/handler.go`) judges the words after the streaming prefix with `ValidateModelArgs` and returns the value with the handler |",
			"| `pluginserver.StreamingHandler` | `(ctx, *Server, io.Writer, username, args []string) error` | 5 handlers (`monitor vpn ipsec`, `monitor event`, `monitor traffic stat`, `monitor interface rate`, `monitor system netlink`) | `iface/cmd`, `trafficstat/cmd`, `ike/cmd`, `bgp/plugins/cmd/monitor`, `plugin/server` (declaration and registry), `cmd/ze/hub` (R8), `internal/component/ssh`, `internal/component/cli`, `internal/test/runner` | R8 | Step 1 |",
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file:  selectorFleetRegistrySpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionLineFleetSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRegistrySpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionLineFleetSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRegistrySpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionLineFleetSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorBGPNextHopSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionLineBGPSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorBGPNextHopSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionLineBGPSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorBGPNextHopSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionLineBGPSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorExplicitPluginSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionLinePluginSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorExplicitPluginSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionLinePluginSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorExplicitPluginSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionLinePluginSpec},
		why:   selectorN1DecisionWhy,
	},
	{
		file: selectorFibDummySpecFile,
		old:  selectorValNetlink,
		lines: []string{
			selectorN1DecisionQuote,
			"| A-7 | The guard spec lands the rename `netlink` to `kernel` without changing the meaning of an empty `DataPlane` |",
		},
		why: selectorN1DecisionWhy,
	},
	{
		file:  selectorFibDummySpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFibDummySpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFibLabelSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFibLabelSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFibLabelSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetTemplatesSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetTemplatesSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetTemplatesSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetAuditSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetAuditSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetAuditSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRolloutSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRolloutSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRolloutSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRelaySpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRelaySpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetRelaySpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetTabsSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetTabsSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetTabsSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetDriftSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetDriftSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetDriftSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetHealthSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetHealthSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetHealthSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetMetricsSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetMetricsSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetMetricsSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetAlarmsSpecFile,
		old:   selectorValNetlink,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetAlarmsSpecFile,
		old:   selectorValNFT,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  selectorFleetAlarmsSpecFile,
		old:   selectorValTC,
		lines: []string{selectorN1DecisionQuote},
		why:   selectorN1DecisionWhy,
	},
	{
		file:  "plan/release/fixes/spec-iface-ethernet-speed-duplex.md",
		old:   selectorValTC,
		lines: []string{"| Offload ethtool path | `offload_linux.go` `applyOffloads`, `ethtoolIoctl`; `docs/architecture/iface/offload.md` | Sets GRO, GSO, TSO, LRO, SG and hw-tc-offload by `SIOCETHTOOL` on the resolved device; an absent leaf makes no call | Extends: link settings use the same socket pattern and the same per-entry loop, with the tri-state rule. Does not copy its warn-only failure handling |"},
		why:   selectorHWTCOffloadWhy,
	},
	{
		file: "plan/release/fixes/spec-iface-offload-failure-surfaced.md",
		old:  selectorValTC,
		lines: []string{
			"`interface ethernet <name> offload { gro; gso; sg; tso; lro; hw-tc-offload; rps;",
			"`hw-tc-offload` on a veth or a virtio NIC), or any ethtool or sysfs error, leaves",
		},
		why: selectorHWTCOffloadWhy,
	},
	{
		file: selectorDoctorForwardingSpecFile,
		old:  selectorValNetlink,
		lines: []string{
			"- every existing check and code of the area: `iface-ethernet`, `iface-macvlan`, `iface-ra-forwarding`, `sysctl-procfs`, the five VPP checks, `kernel-nexthop`, the MPLS capabilities, `policyroute-netlink`, `firewall-domain-group-data`, `firewall-irr-data-freshness`",
			"| Existing area checks | `iface-ethernet`, `iface-macvlan`, `iface-ra-forwarding`, `sysctl-procfs`, `vpp-api-socket`, `vpp-hugepages`, `vpp-cpu-isolation`, `vpp-dpdk`, `kernel-nexthop`, `policyroute-netlink`, `firewall-domain-group-data`, `firewall-irr-data-freshness`, MPLS capabilities | config plus host state per check | extends: `iface-ethernet` walks every configured kind (AC-46); `vpp-dpdk` gains the left-bound case (AC-92); `firewall-irr-data-freshness` declares its third code (AC-73) |",
			"| AC-74 | fib-kernel's route netlink handle could not open (`failedBackend`) | `doctor-fib-kernel-netlink` (error, runtime); a `post-config` probe opens a route netlink socket when fib-kernel is the writer and reports the same code locally | none |",
			"| AC-87 | Lines whose class is already covered: MPLS capability sites, `policyroute-netlink` | each line carries `doctor-mpls-transit-mtu-unenforced`, `doctor-mpls-transit-mtu-unknown`, `doctor-mpls-unavailable`, `doctor-mpls-unknown` or `doctor-policyroute-netlink` | none |",
		},
		why: selectorPolicyRouteCodeWhy,
	},
	{
		file: selectorDoctorForwardingSpecFile,
		old:  selectorValTC,
		lines: []string{
			"| AC-18 | An offload, RPS or RFS setting is not in effect (ethtool feature, `hw-tc-offload`, `rps_cpus`, `rps_flow_cnt`, global `rps_sock_flow_entries`) | `doctor-iface-offload-not-applied` (warning, local) naming device, setting, configured and kernel value | a NIC that does not offer the feature reads as not supported, not as not applied |",
			"| AC-70 | A persisted traffic control original-qdisc snapshot does not match the box (boot id, name, ifindex, MAC) or the store is unreadable | `doctor-traffic-tc-snapshot-mismatch` (error, local) naming interface and the mismatched field; fix names the command that clears the snapshot | none |",
		},
		why: "hw-tc-offload ethtool feature and doctor-traffic-tc-snapshot-mismatch diagnostic code, not selector values",
	},
	{
		file: selectorDoctorForwardingSpecFile,
		old:  selectorValNFT,
		lines: []string{
			"| AC-65 | `ze_` nft tables are in the kernel while no daemon runs and `flush-on-shutdown` is true | `doctor-firewall-tables-left-after-stop` (warning, local, only when no daemon answers) naming the tables; fix names `nft delete table` for each | R-3: with a daemon running the tables are its own |",
			"| 1 | runs `show doctor` after `nft flush ruleset` | `HandleDoctor` -> firewall runtime check -> `AuditTables` | `test/firewall/show-doctor-firewall-drift.ci` |",
		},
		why: selectorNftBinaryWhy,
	},
	{
		file:  "plan/release/validation/spec-qemu-nightly-guests-run-ze-system.md",
		old:   selectorValNFT,
		lines: []string{"- [ ] `internal/appliance/dockerhost/guest_linux.go` - `shippedLinks`: sudo, sshd, ip, nft, setcap, getcap, docker, iptables; busybox applets otherwise"},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "plan/release/validation/spec-release-audit-1-surface-inventory.md",
		old:   selectorValNetlink,
		lines: []string{"| Monitor commands | `internal/component/bgp/plugins/cmd/monitor` | BGP events, ping, traceroute, interface-rate, netlink, vpn-ipsec | `test/plugin/monitor-*` | resilience/security, config/CLI | Known pipe-completeness risks for monitor log paths |"},
		why:   selectorMonitorNetlinkWhy,
	},
	{
		file:  "test/firewall/firewall-legacy-table-removed-with-no-config.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFlowspec},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file: "test/interop-pppoe/Dockerfile.client",
		old:  selectorValTC,
		lines: []string{
			"# iproute2-tc: isolate the peer LCP termination path by dropping client PADT.",
			`RUN apk add --no-cache iproute2 iproute2-tc iputils tcpdump tcpreplay $(cat /.ppp-rundeps) \`,
		},
		why: selectorTCBinaryWhy,
	},
	{
		file:  "test/interop-radius-subscriber/Dockerfile.ze",
		old:   selectorValTC,
		lines: []string{"RUN apk add --no-cache tini iproute2 iproute2-tc ppp tcpdump"},
		why:   selectorTCBinaryWhy,
	},
	{
		file:  "test/parse/iface-offload-ethernet.ci",
		old:   selectorValTC,
		lines: []string{"hw-tc-offload false;"},
		why:   selectorHWTCOffloadWhy,
	},
	{
		file:  "test/plugin/firewall-irr-cold-cache-recovers.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-commit.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-empty-answer-keeps-last-good.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-iface-commit.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-iface-no-blackhole.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-iface-reject.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-refresh.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-reject.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-show.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-table-term-commit.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-table-term-uncached-reject.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/firewall-irr-update.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/flowspec-fw-legacy-table-removed.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFlowspec},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/flowspec-fw-protocol-sctp.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFlowspec},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/flowspec-fw-untranslatable-keeps-others.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFlowspec},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/flowspec-fw-withdraw-removes-table.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFlowspec},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/irr-restart-keeps-firewall-sets.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/irr-shared-dataset.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file:  "test/plugin/irr-update-then-firewall-commit.ci",
		old:   selectorValNFT,
		lines: []string{selectorExclusiveGroupFirewallIRR},
		why:   selectorExclusiveGroupWhy,
	},
	{
		file: "test/plugin/monitor-system-netlink.ci",
		old:  selectorValNetlink,
		lines: []string{
			"# Test: `monitor system netlink` (and monitor system netlink route) dispatched from a plugin is refused",
			"# is refused with an error naming `ze monitor system netlink`, the terminal command that streams it",
			"# Native observer exercises monitor system netlink and its group validation.",
			"endpoint monitor-system-netlink-test {",
		},
		why: selectorMonitorNetlinkWhy,
	},
	{
		file:  "test/plugin/plugin-eof-no-spin.ci",
		old:   selectorValNetlink,
		lines: []string{"# VALIDATES: spec-netlink-ci-harness Fix C / AC-5 -- API.read_line() must set"},
		why:   selectorHistoricalSpecWhy,
	},
	{
		file:  "test/reload/background-ze-readiness.ci",
		old:   selectorValNetlink,
		lines: []string{"# VALIDATES: spec-netlink-ci-harness AC-1 (Fix A) -- native-driver suites"},
		why:   selectorHistoricalSpecWhy,
	},
	{
		file:  "test/traffic/storage-tc-restart.ci",
		old:   selectorValTC,
		lines: []string{"expect=stderr:contains=OK: tc consumer restored operational state from selected tree without config-dir pin"},
		why:   selectorStorageTCWhy,
	},
	{
		file:  "test/ui/doctor-procfs-netlink.ci",
		old:   selectorValNetlink,
		lines: []string{"expect=stdout:contains=doctor-policyroute-netlink"},
		why:   selectorPolicyRouteCodeWhy,
	},
	{
		file:  "website/assets/ze-components-dark.svg",
		old:   selectorValTC,
		lines: []string{`<text x="117.5" y="29" text-anchor="middle" class="surface-copy">nftables · tc</text>`},
		why:   "SVG architecture diagram label naming Linux nftables and tc mechanisms",
	},
	{
		file:  "website/assets/ze-components.svg",
		old:   selectorValTC,
		lines: []string{`<text x="117.5" y="29" text-anchor="middle" class="surface-copy">nftables · tc</text>`},
		why:   "SVG architecture diagram label naming Linux nftables and tc mechanisms",
	},
	{
		file: "website/compare/nos.md",
		old:  selectorValNFT,
		lines: []string{
			"VyOS is primarily orchestration over Linux and daemons such as FRR, nft, strongSwan, VPP, etc.",
			"Conf mode scripts call systemctl, nft, ip, podman, etc.",
		},
		why: selectorNftBinaryWhy,
	},
	{
		file: "website/data/wiki.json",
		old:  selectorValNetlink,
		lines: []string{
			`"title": "iface-netlink",`,
			`"slug": "iface-netlink",`,
		},
		why: selectorIfaceNetlinkNameWhy,
	},
	{
		file:  "website/quality/qemu-interop-release.md",
		old:   selectorValNFT,
		lines: []string{"The QEMU guest uses Ze's kernel and system image. It provides Linux features that Docker Desktop on macOS cannot provide reliably. During a debug run, you can inspect `ip`, `nft`, `dmesg`, temporary files, process state, and generated logs inside the guest."},
		why:   selectorNftBinaryWhy,
	},
	{
		file:  "website/quality/verify-debugging.md",
		old:   selectorValNFT,
		lines: []string{"<tr><td>Linux diagnosis</td><td><code>./le test qemu run command '...' keep-alive</code>, then inspect <code>ip</code>, <code>nft</code>, <code>dmesg</code>, and temp files.</td></tr>"},
		why:   selectorNftBinaryWhy,
	},
}

// RetiredSelectorMatch is one tracked occurrence of a retired selector value.
type RetiredSelectorMatch struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Old         string `json:"old"`
	Replacement string `json:"replacement"`
}

// retiredSelectorReport is the answer of `le doc check retired-selector-values`.
type retiredSelectorReport struct {
	Matches    []RetiredSelectorMatch `json:"matches"`
	Count      int                    `json:"count"`
	Unreadable []string               `json:"unreadable,omitempty"`
}

// Text renders every finding naming its file, line, old value and successor,
// any unreadable file, and the finding count.
func (r retiredSelectorReport) Text() string {
	var out textbuf.Buffer
	for _, match := range r.Matches {
		out.Str(match.File).Byte(':').Int(int64(match.Line)).
			Str(": retired selector value ").Str(match.Old).
			Str(" (use ").Str(match.Replacement).Str(")\n")
	}
	for _, rel := range r.Unreadable {
		out.Str("unreadable: ").Str(rel).Byte('\n')
	}
	out.Int(int64(r.Count)).Str(" tracked occurrence(s) name a retired selector value\n")
	return out.String()
}

// byteSpan is a half-open byte range [start, end) inside a file.
type byteSpan struct {
	start int
	end   int
}

// buildMechanismWordSet expands a closed O-5 word list to include each word's
// lower-case singular and plural forms.
func buildMechanismWordSet(words []string) map[string]bool {
	set := make(map[string]bool, len(words)*2)
	for _, word := range words {
		lower := strings.ToLower(word)
		set[lower] = true
		set[pluralOf(lower)] = true
	}
	return set
}

// pluralOf returns the English plural form of a lower-case mechanism word.
func pluralOf(word string) string {
	if len(word) >= 2 && strings.HasSuffix(word, "y") && !isASCIIVowel(word[len(word)-2]) {
		return word[:len(word)-1] + "ies"
	}
	if strings.HasSuffix(word, "s") {
		return word + "es"
	}
	if strings.HasSuffix(word, "x") {
		return word + "es"
	}
	return word + "s"
}

func isASCIIVowel(b byte) bool {
	switch b {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}

func isASCIIAlpha(b byte) bool {
	if b >= 'a' && b <= 'z' {
		return true
	}
	return b >= 'A' && b <= 'Z'
}

func isASCIIAlphaNum(b byte) bool {
	if isASCIIAlpha(b) {
		return true
	}
	return b >= '0' && b <= '9'
}

func isHorizSpace(b byte) bool {
	if b == ' ' {
		return true
	}
	return b == '\t'
}

func isDoubleOrBackQuote(b byte) bool {
	if b == '"' {
		return true
	}
	return b == '`'
}

func isSlashOrDot(b byte) bool {
	if b == '/' {
		return true
	}
	return b == '.'
}

func isSlashOrHyphen(b byte) bool {
	if b == '/' {
		return true
	}
	return b == '-'
}

func isPathTokenByte(b byte) bool {
	if isWordByte(b) {
		return true
	}
	if b == '-' {
		return true
	}
	return isSlashOrDot(b)
}

func isOptionWordByte(b byte) bool {
	if isWordByte(b) {
		return true
	}
	return b == '-'
}

// retiredSelectorFileExcluded returns the file-class exclusion predicate
// applying F-1 through F-4 in order.
func retiredSelectorFileExcluded(recordMatch func(string) bool) func(string) bool {
	return func(rel string) bool {
		// F-1: history records shared with retired.go.
		if recordMatch(rel) {
			return true
		}
		// F-2: not Ze's text.
		for _, prefix := range &retiredSelectorNotZePrefixes {
			if strings.HasPrefix(rel, prefix) {
				return true
			}
		}
		// F-3: the sweep itself, its tests, its fixture tree, and its spec.
		if slices.Contains(retiredSelectorSweepFiles[:], rel) {
			return true
		}
		if strings.HasPrefix(rel, retiredSelectorSweepDir) {
			return true
		}
		// F-4: the AC-19 refusal proof tests.
		return slices.Contains(retiredSelectorProofFiles[:], rel)
	}
}

// sweepRetiredSelectors walks every tracked file under root outside file
// classes F-1 to F-4 and returns every whole-word occurrence of a retired
// selector value not covered by occurrence classes O-1 to O-7 or line
// exceptions.
func sweepRetiredSelectors(root string) (retiredSelectorReport, error) {
	recordMatch, err := newRetiredRecordMatcher(root)
	if err != nil {
		return retiredSelectorReport{}, err
	}
	files, err := trackedFiles(root)
	if err != nil {
		return retiredSelectorReport{}, err
	}

	report := retiredSelectorReport{Matches: []RetiredSelectorMatch{}}
	unreadable, err := walkTracked(root, files, retiredSelectorFileExcluded(recordMatch), func(rel string, raw []byte) error {
		if slices.Contains(raw, 0) {
			return nil
		}
		scanRetiredSelectorFile(rel, raw, &report.Matches)
		return nil
	})
	if err != nil {
		return retiredSelectorReport{}, err
	}
	for _, file := range unreadable {
		var tb textbuf.Buffer
		report.Unreadable = append(report.Unreadable, tb.Str(file.rel).Str(": ").Err(file.err).String())
	}
	report.Count = len(report.Matches)
	return report, nil
}

// runRetiredSelectorCheck is `le doc check retired-selector-values` (AC-20).
func runRetiredSelectorCheck(root string) (any, int) {
	report, err := sweepRetiredSelectors(root)
	if err != nil {
		return errorReport{Error: err.Error()}, 2
	}
	if len(report.Unreadable) != 0 {
		return report, 2
	}
	if len(report.Matches) != 0 {
		return report, 1
	}
	return report, 0
}

// scanRetiredSelectorFile scans one tracked non-binary file and appends every
// uncovered retired selector occurrence to matches.
func scanRetiredSelectorFile(rel string, raw []byte, matches *[]RetiredSelectorMatch) {
	isGo := strings.HasSuffix(rel, ".go")
	var textSpans []byteSpan
	if isGo {
		textSpans = goTextSpans(rel, raw)
	}
	spanCursor := 0
	ownsRows := retiredOwnsRows(rel)
	inOwnedRows := false

	text := string(raw)
	lineNo := 0
	for lineStart := 0; lineStart <= len(text); {
		end := strings.IndexByte(text[lineStart:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += lineStart
		}
		lineNo++
		line := text[lineStart:end]
		nextStart := end + 1

		if ownsRows {
			inOwnedRows = retiredOwnedRowsAfter(inOwnedRows, line)
			if inOwnedRows {
				lineStart = nextStart
				continue
			}
		}

		for pos := 0; pos < len(line); {
			if !isWordByte(line[pos]) {
				pos++
				continue
			}
			wordStart := pos
			for pos < len(line) && isWordByte(line[pos]) {
				pos++
			}
			old, matched := matchRetiredSelectorValue(line[wordStart:pos])
			if !matched {
				continue
			}
			// O-1: Go code outside comments and string/rune literals.
			if isGo {
				absStart := lineStart + wordStart
				absEnd := lineStart + pos
				if !offsetInTextSpans(textSpans, &spanCursor, absStart, absEnd) {
					continue
				}
			}
			if coveredByOccurrenceClass(rel, old, line, wordStart, pos) {
				continue
			}
			if retiredSelectorExempt(rel, old, line) {
				continue
			}
			*matches = append(*matches, RetiredSelectorMatch{
				File:        rel,
				Line:        lineNo,
				Old:         old,
				Replacement: retiredSelectorReplacement,
			})
		}
		lineStart = nextStart
	}
}

// matchRetiredSelectorValue reports whether word folds to one of the declared
// retired selector values, returning its canonical lower-case spelling.
func matchRetiredSelectorValue(word string) (string, bool) {
	for _, value := range retiredSelectorValues {
		if len(word) == len(value) && strings.EqualFold(word, value) {
			return value, true
		}
	}
	return "", false
}

// goTextSpans returns sorted half-open byte ranges of every comment, string
// literal and rune literal in a Go file, as go/scanner tokenizes it.
func goTextSpans(rel string, raw []byte) []byteSpan {
	fset := token.NewFileSet()
	file := fset.AddFile(rel, fset.Base(), len(raw))
	var s scanner.Scanner
	s.Init(file, raw, nil, scanner.ScanComments)
	var spans []byteSpan
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		// Only comments, string literals and rune literals carry non-code text for O-1.
		//exhaustive:ignore
		switch tok {
		case token.COMMENT, token.STRING, token.CHAR:
			offset := file.Offset(pos)
			spans = append(spans, byteSpan{
				start: offset,
				end:   goTokenEnd(raw, offset, tok, lit),
			})
		}
	}
	return spans
}

// goTokenEnd computes the raw byte end offset of a comment, string, or rune
// token, avoiding CR-stripping drift on raw strings or comments.
func goTokenEnd(raw []byte, offset int, tok token.Token, lit string) int {
	// Only line comments, block comments and raw strings need raw-byte scanning.
	//exhaustive:ignore
	switch tok {
	case token.COMMENT:
		if offset+1 < len(raw) && raw[offset+1] == '/' {
			if nl := bytes.IndexByte(raw[offset:], '\n'); nl >= 0 {
				return offset + nl
			}
			return len(raw)
		}
		if offset+1 < len(raw) && raw[offset+1] == '*' {
			if closeIdx := bytes.Index(raw[offset+2:], []byte("*/")); closeIdx >= 0 {
				return offset + 2 + closeIdx + 2
			}
			return len(raw)
		}
	case token.STRING:
		if offset < len(raw) && raw[offset] == '`' {
			if closeIdx := bytes.IndexByte(raw[offset+1:], '`'); closeIdx >= 0 {
				return offset + 1 + closeIdx + 1
			}
			return len(raw)
		}
	}
	return min(offset+len(lit), len(raw))
}

// offsetInTextSpans reports whether [start, end) lies inside any sorted span,
// advancing cursor monotonically across the file.
func offsetInTextSpans(spans []byteSpan, cursor *int, start, end int) bool {
	for *cursor < len(spans) && spans[*cursor].end <= start {
		*cursor++
	}
	if *cursor >= len(spans) {
		return false
	}
	span := spans[*cursor]
	return span.start <= start && end <= span.end
}

// coveredByOccurrenceClass evaluates occurrence classes O-2 through O-7 in the
// order declared by AC-20.
func coveredByOccurrenceClass(rel, old, line string, start, end int) bool {
	// O-2: joined name.
	if coveredByJoinedName(line, start, end) {
		return true
	}
	// O-3: all capitals.
	if coveredByAllCaps(line, start, end) {
		return true
	}
	// O-4: traffic YANG module prefix.
	if coveredByYANGPrefix(rel, line, start, end) {
		return true
	}
	// O-5: kernel mechanism by next word.
	if coveredByNextMechanismWord(old, line, end) {
		return true
	}
	// O-6: netlink after a mechanism word.
	if coveredByNetlinkPrefix(old, line, start, end) {
		return true
	}
	// O-7: a program's argument vector.
	return coveredByProgramArgv(line, start, end)
}

// isSelectorPartner reports whether word is vpp, kernel, or any retired
// selector value, folded to lower case.
func isSelectorPartner(word string) bool {
	if strings.EqualFold(word, "vpp") {
		return true
	}
	if strings.EqualFold(word, retiredSelectorReplacement) {
		return true
	}
	_, retired := matchRetiredSelectorValue(word)
	return retired
}

// hasRefusedJoinPartner reports whether line[start:end] is joined on either
// side by '/' or '-' to a selector partner (vpp, kernel, or a retired value).
func hasRefusedJoinPartner(line string, start, end int) bool {
	if start >= 2 && isSlashOrHyphen(line[start-1]) && isWordByte(line[start-2]) {
		pStart := start - 2
		for pStart > 0 && isWordByte(line[pStart-1]) {
			pStart--
		}
		if isSelectorPartner(line[pStart : start-1]) {
			return true
		}
	}
	if end+1 < len(line) && isSlashOrHyphen(line[end]) && isWordByte(line[end+1]) {
		pEnd := end + 1
		for pEnd < len(line) && isWordByte(line[pEnd]) {
			pEnd++
		}
		if isSelectorPartner(line[end+1 : pEnd]) {
			return true
		}
	}
	return false
}

// coveredByJoinedName implements O-2: joined to [A-Za-z0-9_] by '/' or '.' on
// either side, or by '-' inside a file or path token (holding '/' or '.'
// followed by an ASCII letter), unless joined by '/' or '-' to a selector
// partner.
func coveredByJoinedName(line string, start, end int) bool {
	if hasRefusedJoinPartner(line, start, end) {
		return false
	}
	if start >= 2 && isSlashOrDot(line[start-1]) && isWordByte(line[start-2]) {
		return true
	}
	if end+1 < len(line) && isSlashOrDot(line[end]) && isWordByte(line[end+1]) {
		return true
	}
	leftHyphen := start >= 2 && line[start-1] == '-' && isWordByte(line[start-2])
	rightHyphen := end+1 < len(line) && line[end] == '-' && isWordByte(line[end+1])
	if !leftHyphen && !rightHyphen {
		return false
	}
	tStart := start
	for tStart > 0 && isPathTokenByte(line[tStart-1]) {
		tStart--
	}
	tEnd := end
	for tEnd < len(line) && isPathTokenByte(line[tEnd]) {
		tEnd++
	}
	return isPathOrFileToken(line[tStart:tEnd])
}

// isPathOrFileToken reports whether token contains '/' or '.' followed by an
// ASCII letter.
func isPathOrFileToken(token string) bool {
	if strings.ContainsRune(token, '/') {
		return true
	}
	for i := 0; i+1 < len(token); i++ {
		if token[i] == '.' && isASCIIAlpha(token[i+1]) {
			return true
		}
	}
	return false
}

// coveredByAllCaps implements O-3: all ASCII uppercase letters, except when
// followed by backend/backends (any case) or joined by '/' or '-' to a partner
// O-2 refuses.
func coveredByAllCaps(line string, start, end int) bool {
	for i := start; i < end; i++ {
		if line[i] < 'A' {
			return false
		}
		if line[i] > 'Z' {
			return false
		}
	}
	if hasRefusedJoinPartner(line, start, end) {
		return false
	}
	pos := end
	for pos < len(line) && isAllCapsGapByte(line[pos]) {
		pos++
	}
	if pos >= len(line) {
		return true
	}
	if !isWordByte(line[pos]) {
		return true
	}
	wEnd := pos
	for wEnd < len(line) && isWordByte(line[wEnd]) {
		wEnd++
	}
	next := line[pos:wEnd]
	if strings.EqualFold(next, "backend") {
		return false
	}
	if strings.EqualFold(next, "backends") {
		return false
	}
	return true
}

func isAllCapsGapByte(b byte) bool {
	if isHorizSpace(b) {
		return true
	}
	if isDoubleOrBackQuote(b) {
		return true
	}
	return b == '\''
}

// coveredByYANGPrefix implements O-4: lowercase tc directly followed by ':'
// and an ASCII letter in any file, or `prefix tc;` after line start, '{', or
// ';' in a .yang file.
func coveredByYANGPrefix(rel, line string, start, end int) bool {
	if line[start:end] != selectorValTC {
		return false
	}
	if end+1 < len(line) && line[end] == ':' && isASCIIAlpha(line[end+1]) {
		return true
	}
	if !strings.HasSuffix(rel, ".yang") {
		return false
	}
	if end >= len(line) {
		return false
	}
	if line[end] != ';' {
		return false
	}
	pEnd := start
	for pEnd > 0 && isHorizSpace(line[pEnd-1]) {
		pEnd--
	}
	if pEnd == start {
		return false
	}
	const prefixKeyword = "prefix"
	if pEnd < len(prefixKeyword) {
		return false
	}
	if line[pEnd-len(prefixKeyword):pEnd] != prefixKeyword {
		return false
	}
	before := pEnd - len(prefixKeyword)
	for before > 0 && isHorizSpace(line[before-1]) {
		before--
	}
	if before == 0 {
		return true
	}
	if line[before-1] == '{' {
		return true
	}
	return line[before-1] == ';'
}

// coveredByNextMechanismWord implements O-5: followed by one space and a
// singular/plural word of the closed list for old, or an option word starting
// with '-' for nft and tc.
func coveredByNextMechanismWord(old, line string, end int) bool {
	if end+1 >= len(line) {
		return false
	}
	if line[end] != ' ' {
		return false
	}
	wStart := end + 1
	if line[wStart] == '-' {
		wEnd := wStart
		for wEnd < len(line) && isOptionWordByte(line[wEnd]) {
			wEnd++
		}
		return matchesMechanismWord(old, line[wStart:wEnd])
	}
	if !isASCIIAlphaNum(line[wStart]) {
		return false
	}
	wEnd := wStart
	for wEnd < len(line) {
		if isWordByte(line[wEnd]) {
			wEnd++
			continue
		}
		if line[wEnd] == '-' && wEnd+1 < len(line) && isASCIIAlphaNum(line[wEnd+1]) {
			wEnd++
			continue
		}
		break
	}
	return matchesMechanismWord(old, line[wStart:wEnd])
}

// matchesMechanismWord reports whether candidate is in O-5's closed list
// (singular or plural) for old, or is a CLI option word for nft/tc.
func matchesMechanismWord(old, candidate string) bool {
	if candidate == "" {
		return false
	}
	if candidate[0] == '-' {
		if old == selectorValNetlink {
			return false
		}
		trimmed := strings.TrimLeft(candidate, "-")
		if trimmed == "" {
			return false
		}
		return isASCIIAlphaNum(trimmed[0])
	}
	words := mechanismWordsByValue[old]
	return words[strings.ToLower(candidate)]
}

// coveredByNetlinkPrefix implements O-6: netlink (any case) preceded by one
// mechanism prefix word and one space, except when followed by '/', a plane
// conjunction ("or/and vpp/kernel"), or when either of the next two words
// (punctuation skipped) starts with backend, plane, or default.
func coveredByNetlinkPrefix(old, line string, start, end int) bool {
	if old != selectorValNetlink {
		return false
	}
	if start < 2 {
		return false
	}
	if line[start-1] != ' ' {
		return false
	}
	if !isWordByte(line[start-2]) {
		return false
	}
	pStart := start - 2
	for pStart > 0 && isWordByte(line[pStart-1]) {
		pStart--
	}
	if !retiredSelectorNetlinkPrefixes[strings.ToLower(line[pStart:start-1])] {
		return false
	}
	if end < len(line) && line[end] == '/' {
		return false
	}
	if followedByPlaneConjunction(line, end) {
		return false
	}
	if nextTwoWordsNameSelector(line, end) {
		return false
	}
	return true
}

// followedByPlaneConjunction reports whether line[end:] starts with space +
// ("or" | "and") + space + optional quote/backtick + ("vpp" | "kernel").
func followedByPlaneConjunction(line string, end int) bool {
	if end+1 >= len(line) {
		return false
	}
	if line[end] != ' ' {
		return false
	}
	pos := end + 1
	w1Start := pos
	for pos < len(line) && isWordByte(line[pos]) {
		pos++
	}
	w1 := line[w1Start:pos]
	if !strings.EqualFold(w1, "or") && !strings.EqualFold(w1, "and") {
		return false
	}
	if pos+1 >= len(line) {
		return false
	}
	if line[pos] != ' ' {
		return false
	}
	pos++
	if isDoubleOrBackQuote(line[pos]) {
		pos++
	}
	w2Start := pos
	for pos < len(line) && isWordByte(line[pos]) {
		pos++
	}
	w2 := line[w2Start:pos]
	if strings.EqualFold(w2, "vpp") {
		return true
	}
	return strings.EqualFold(w2, retiredSelectorReplacement)
}

// nextTwoWordsNameSelector reports whether either of the next two word tokens
// after end (skipping non-word punctuation) starts with backend, plane, or
// default in any case.
func nextTwoWordsNameSelector(line string, end int) bool {
	pos := end
	for range 2 {
		for pos < len(line) && !isWordByte(line[pos]) {
			pos++
		}
		if pos >= len(line) {
			return false
		}
		wStart := pos
		for pos < len(line) && isWordByte(line[pos]) {
			pos++
		}
		word := strings.ToLower(line[wStart:pos])
		if strings.HasPrefix(word, "backend") {
			return true
		}
		if strings.HasPrefix(word, "plane") {
			return true
		}
		if strings.HasPrefix(word, "default") {
			return true
		}
	}
	return false
}

// coveredByProgramArgv implements O-7: an exact double- or back-quoted "nft" or
// "tc" literal followed by a comma and a quoted literal matching O-5's list for
// that program.
func coveredByProgramArgv(line string, start, end int) bool {
	word := line[start:end]
	if word != selectorValNFT && word != selectorValTC {
		return false
	}
	if start < 1 {
		return false
	}
	if end >= len(line) {
		return false
	}
	quote := line[start-1]
	if !isDoubleOrBackQuote(quote) {
		return false
	}
	if line[end] != quote {
		return false
	}
	pos := end + 1
	for pos < len(line) && isHorizSpace(line[pos]) {
		pos++
	}
	if pos >= len(line) {
		return false
	}
	if line[pos] != ',' {
		return false
	}
	pos++
	for pos < len(line) && isHorizSpace(line[pos]) {
		pos++
	}
	if pos >= len(line) {
		return false
	}
	argQuote := line[pos]
	if !isDoubleOrBackQuote(argQuote) {
		return false
	}
	pos++
	argStart := pos
	for pos < len(line) && line[pos] != argQuote {
		pos++
	}
	if pos >= len(line) {
		return false
	}
	return matchesMechanismWord(word, line[argStart:pos])
}

// retiredSelectorExempt reports whether a declared line exception covers old on
// line of rel.
func retiredSelectorExempt(rel, old, line string) bool {
	for _, exception := range &retiredSelectorExceptions {
		if exception.file != rel {
			continue
		}
		if exception.old != old {
			continue
		}
		for _, covered := range exception.lines {
			if strings.Contains(line, covered) {
				return true
			}
		}
	}
	return false
}
