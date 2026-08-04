# Learned Summaries Index

Curated index of `plan/learned/` summaries that capture structural decisions, patterns, and gotchas.
Task-completion-only summaries (the majority) are omitted. Full list: `ls plan/learned/`.

Summaries 001 to 400 were retired on 2026-08-01, so this index has no rows for them.
Their surviving knowledge was merged into `plan/learned/DESIGN-HISTORY.md`, which IS
the record for that era rather than an index to one. Each section below names the
DESIGN-HISTORY section that holds its pre-401 history.

Three meta-summaries sit alongside the numbered per-spec summaries. Read the one that
matches your question first — each one points at the specific numbered summaries that
hold the full record, so one file of reading replaces hundreds.

| Question | File |
|----------|------|
| "Why is the code as it is?" | `plan/learned/DESIGN-HISTORY.md` — design evolution by subsystem, abandoned approaches, load-bearing invariants |
| "Am I about to fall into a known trap?" | `plan/learned/RECURRING-PATTERNS.md` — patterns that recurred 5+ times, with avoid-it-by and recover-if-you-hit-it |
| "Why did this hook reject my code?" | `plan/learned/HOOK-FRICTION.md` — every hook false positive with verified workaround |

## Core Architecture

System boundaries, component design, lifecycle patterns, subsystem separation.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "Plugin system: architecture" and "BGP engine: session,
FSM, TCP" (retired 001, 013, 015, 133, 149, 165, 244, 247). The hub separation of
157 is recorded there under 165, which carries the process-per-subsystem decision.

- [760](plan/learned/760-subscriber-session-model.md) -- Unified subscriber session model: shared Session struct across PPPoE/L2TP, handler delegation, event bridge pattern
- [761](plan/learned/761-vpp-fib-query.md) -- How a FIB route lookup reaches VPP through the active backend instead of a netlink bypass
- [762](plan/learned/762-rs-dynamic-peers.md) -- How an IXP route server creates peer groups from prefix ranges at connection time
- [763](plan/learned/763-backend-aware-completion.md) -- Why command completion filters on the configured backend, not on imported components
- [912](plan/learned/912-irr-prefix-store.md) -- Why the shared IRR prefix store is a subpackage, and how it shares state across processes
- [945](plan/learned/945-tiers-1-rule-and-audit.md) -- Why module tier is decided by dependency direction, and what `make ze-tier-check` enforces
- [1089](plan/learned/1089-layout-2-core-import-gate.md) -- How the `internal/core/` import-direction gate works, and why its baseline is pair-granular
- [1092](plan/learned/1092-layout-0-umbrella.md) -- What the layout umbrella made checkable, and what it deliberately left advisory
- [1019](plan/learned/1019-traffic-usage-monitor.md) -- How trafficstat became a refcounted service, and how its import cycle was broken
- [1015](plan/learned/1015-cp-survival-5-detect-5-characterization.md) -- How the DDoS detector characterizes an attack, and why a generation guard needs one lock
- [1108](plan/learned/1108-ddos-detect-enhancements.md) -- How the bandwidth trigger, baseline persistence and confidence grading reached DDoS detect
- [1027](plan/learned/1027-dns-server-harness.md) -- Why the DNS listener moved to `internal/core/dnsserver` at only two consumers
- [1083](plan/learned/1083-unify-startup.md) -- How one 5-stage startup handshake serves both the engine and the hub

## Wire/Encoding

Buffer-first, zero-copy, attribute pools, UPDATE building, NLRI parsing.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding and RIB"
(retired 059, 073, 076, 092, 102, 105, 176, 204).

- [721](plan/learned/721-bgp-2-aspa.md) -- ASPA path verification: RTR v2 per-session version, ROACache O(1) counter, route tracker for re-validation
- [722](plan/learned/722-spec-bgp-4-aspa-policy.md) -- How ASPA policy orders its overrides against origin validation
- [764](plan/learned/764-attr-flags-json.md) -- How attribute flags reach Ze native JSON without a second formatter
- [765](plan/learned/765-gc-pressure-reduction.md) -- Which GC-pressure fixes paid, and which stack arrays are not optimizations
- [821](plan/learned/821-spec-attrpool-shard.md) -- How attrpool sharding works, and why a Handle slot is composite not dense
- [1077](plan/learned/1077-unify-buffer-lifetime.md) -- Why the four buffer-lifetime contracts stay separate while enforcement unifies
- [767](plan/learned/767-tokenizer-no-escape.md) -- Why the command tokenizer treats backslash as an ordinary character
- [770](plan/learned/770-precomputation-review.md) -- Which precomputation proposals were rejected, and why profiling comes first
- [771](plan/learned/771-performance-optimization-campaign.md) -- What the optimization campaign changed, and how much convergence time it cut
- [859](plan/learned/859-perf-hot-alloc-reduction.md) -- How value-type route keys removed the hot-path string maps
- [875](plan/learned/875-filter-delta-parse-once.md) -- Why the filter modify path parses filter text once instead of four times
- [785](plan/learned/785-rfc7606-validation-cache.md) -- Why an RFC 7606 validation cache was rejected despite a high hit rate

## Plugin System

Registration, SDK, event flow, lifecycle, hook integration.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "Plugin system: architecture" (retired 253).

- [905](plan/learned/905-bgp-family-checklist.md) -- What the BGP family integration checklist covers, and which gates enforce it
- [757](plan/learned/757-typed-route-result.md) -- Typed RouteResult replaces map[string]any in update-route, eliminating int/float64 transport divergence
- [781](plan/learned/781-remove-private-as.md) -- Remove-private-as: plugin intent + reactor wire rewrite; Set+Prepend composition
- [806](plan/learned/806-install-1-dhcp-pxe.md) -- How DHCP PXE options were added to the existing reply builder
- [807](plan/learned/807-install-2-tftpserver.md) -- How the TFTP server handles concurrency and per-transfer ports
- [811](plan/learned/811-install-3-image-server.md) -- How the image server serves images without allowing path traversal
- [867](plan/learned/867-yang-rename-ownership.md) -- Why command YANG moved from component to plugin, and what codegen missed
- [1060](plan/learned/1060-vpp-crash-reconciliation.md) -- How the iface backend recovers from a VPP crash by reloading
- [1016](plan/learned/1016-observation-feed.md) -- Why the shared observation feed is a typed in-process feed, not the EventBus
- [977](plan/learned/977-traffic-usage.md) -- How a pure-Go eBPF TCX program is assembled without C or clang
- [980](plan/learned/980-feature-gate-1-lg.md) -- How per-feature compile-out works, and what the looking-glass pilot proved
- [981](plan/learned/981-feature-gate-2-ssh.md) -- Why SSH compile-out needs a dedicated seam instead of the listener registry
- [984](plan/learned/984-feature-gate-3-web.md) -- Why web compile-out required extracting the certificate helpers first
- [983](plan/learned/983-feature-gate-manifest-ssot.md) -- Why `feature-gates.txt` is the single source for the tag-to-package fact
- [986](plan/learned/986-feature-gate-4-gnmi.md) -- Why gNMI compile-out uses a seam, and how one feature owns three blank imports
- [987](plan/learned/987-feature-gate-5-mcp.md) -- How MCP compile-out freed the always-on command lister from an MCP type
- [989](plan/learned/989-feature-gate-6-api.md) -- Why REST and gRPC gate separately, and how one config container splits across modules
- [990](plan/learned/990-feature-gate-7-monitoring.md) -- Why the telemetry seam lives in a core leaf rather than the hub
- [995](plan/learned/995-feature-gate-8-protocols.md) -- How routing protocols gate by blank-import partitioning with no source tags
- [1177](plan/learned/1177-feature-gate-9-vrrp.md) -- Why VRRP is the simplest gated-plugin shape, with one composition root
- [1249](plan/learned/1249-feature-gate-10-bgp.md) -- How BGP was gated by extract-then-gate, and the three traps it exposed
- [1263](plan/learned/1263-feature-gate-12-remaining.md) -- How the last 20 features were gated, and why dependent gates went per-package
- [1000](plan/learned/1000-cli-object-rooting.md) -- Why operational commands root at their owning object, with no `ip` namespace
- [988](plan/learned/988-kernel-build-consolidation.md) -- How the kernel build converged on one driver, and how fragments are shared
- [812](plan/learned/812-install-5-bootstrap-mode.md) -- How bootstrap mode emits its own config and falls through on failure
- [813](plan/learned/813-install-6-installer-initrd.md) -- How the installer initrd finds a disk and streams the image onto it
- [815](plan/learned/815-install-7a-namespace.md) -- Where the appliance command moved to, and what the deprecated alias does
- [816](plan/learned/816-install-7b-vendor-builder.md) -- How the vendored gokrazy builder replaced the `bin/gok` shell-out
- [817](plan/learned/817-install-7c-vendor-updater.md) -- How the vendored updater streams, switches and reboots an appliance
- [982](plan/learned/982-install-11-hw-kernel-profiles.md) -- How kernel profiles are registered, and what Go enforces over a raw `make`
- [818](plan/learned/818-flow-export-1-counter-export.md) -- How IPFIX counter export chooses total counts and sizes its datagrams
- [819](plan/learned/819-flow-export-2-flow-records.md) -- How flow records stay platform-independent, and how enrichment attaches
- [820](plan/learned/820-flow-export-0-umbrella.md) -- Why one collection feeds many flow-export consumers through the iface callback
- [1145](plan/learned/1145-plugin-internal-keyword.md) -- How an internal plugin is declared, and which external leaves it ignores
- [884](plan/learned/884-cos-plugin.md) -- How the CoS registry binds to interfaces, and why registry order matters
- [887](plan/learned/887-cos-vendor-radius.md) -- How vendor CoS and rate attributes map onto the shaper pipeline
- [894](plan/learned/894-show-enricher.md) -- How plugins register show-command enrichers, and how handlers call them
- [895](plan/learned/895-show-enricher-v2.md) -- How an external plugin declares and serves show enrichment
- [828](plan/learned/828-codec-callback-passthrough.md) -- Why NLRI hex decode marshals once in the registry
- [830](plan/learned/830-typed-inter-plugin-dispatch.md) -- Why inter-plugin dispatch passes typed args instead of a rebuilt string
- [858](plan/learned/858-typed-peer-selector.md) -- Why the BGP reactor takes a typed peer selector instead of a string
- [922](plan/learned/922-cross-plugin-switch-audit.md) -- Which cross-plugin switches are correct Go, and which signal a missing method
- [938](plan/learned/938-bug-review-0-umbrella.md) -- How a review-only bug campaign routes findings into fix specs
- [1028](plan/learned/1028-as112-1-iface-address-registry.md) -- How a plugin owns kernel addresses through iface, and why stale tracking is scoped
- [1035](plan/learned/1035-as112-0-umbrella.md) -- Why AS112 host addresses and covering prefixes are different objects
- [1033](plan/learned/1033-as112-2-dns-server.md) -- How the AS112 DNS core matches zones, and why its listener needs a sentinel reset
- [1034](plan/learned/1034-as112-3-bgp-integration.md) -- What the AS112 watchdog probe exposed in the SSH executor and community parser
- [1032](plan/learned/1032-as112-review-hardening.md) -- What two review passes found in AS112, including the internal-only plugin guard
- [1045](plan/learned/1045-plugin-process-boundary.md) -- How the plugin process-boundary rule and its checker came out of AS112

- [1046](plan/learned/1046-traffic-analysis-restructure.md) -- Where neutral traffic analysis ends and a detection verdict begins
- [1048](plan/learned/1048-anomaly-1-detect.md) -- How anomaly detection scores an entity without poisoning its own baseline
- [1049](plan/learned/1049-anomaly-2-shape.md) -- How the anomaly responder arms, reverts and caps its blast radius
- [1055](plan/learned/1055-config-apply-ordering.md) -- How config apply orders its operations, and who owns the decomposition rules
- [1075](plan/learned/1075-unify-rpc-dispatch.md) -- How one registry replaced four plugin-RPC dispatch tables
- [1124](plan/learned/1124-vrrp-first-hop-redundancy.md) -- How VRRP interoperates with keepalived, and why the checksum form is the v3 one

## Configuration

YANG schema, migration, config reload, editor, environment variables.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "Configuration: YANG, parser, editor, reload"
(retired 008, 065, 166, 175, 232). The BGPConfig removal of 226 survives only as the
`map[string]any` step in that section's Current shape pipeline.

- [716](plan/learned/716-iface-2-urpf.md) -- rpf-check enum over raw sysctl integer; three-state nil/disable/value pattern; sysctl profile ordering issue
- [882](plan/learned/882-vlan-qos-map.md) -- How VLAN 802.1p QoS maps are modeled and validated
- [883](plan/learned/883-vlan-qos-lab.md) -- How the VLAN QoS wire-level lab decodes TCI and proves ingress priority
- [725](plan/learned/725-spec-cpe-3-dhcp-ranges.md) -- YANG container-to-list migration, composite pool with per-segment bitmaps, format detection for backward compat
- [743](plan/learned/743-config-schema-stamp.md) -- Schema stamp as comment line (not YANG leaf), emitted at persistence site only, prep for downgrade recovery
- [746](plan/learned/746-cpe-4-firewall-global-options.md) -- How firewall global options map keywords onto sysctls
- [915](plan/learned/915-firewall-irr-iface.md) -- How per-interface IRR source validation is tabled and hooked
- [1005](plan/learned/1005-cp-survival-2-copp-port179.md) -- How CoPP for BGP builds its input chain, and why established comes first
- [1007](plan/learned/1007-cp-survival-3-egress-cs6-sched.md) -- How the DSCP selector is populated per address family
- [758](plan/learned/758-config-graph.md) -- How the config dependency graph is derived, and what its edge kinds are
- [759](plan/learned/759-archive-pruning.md) -- How archive commit revisions are pruned, and by which timestamp
- [860](plan/learned/860-yang-required-generic.md) -- How `ze:required` is enforced across multi-module YANG sections
- [1058](plan/learned/1058-redist-source-registration.md) -- Why registry-backed config validation must populate at `init()`
- [1180](plan/learned/1180-rpki-per-peer-action.md) -- Why a plugin keying by peer identity must read `configjson.PeerRemoteIP`
- [1340](plan/learned/1340-fixit-bgp-per-family-prefix-enforcement.md) -- Why a YANG leaf inside a list container stored as a Go scalar is a defect before it runs, and why a `test/parse/` .ci cannot see it

## CLI/API

Command structure, text format, IPC, RPC dispatch.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "CLI, web, lookings glass, monitoring" and
"Plugin system: architecture" (retired 072, 081, 132, 209, 229, 245). The command
consolidations of 110 and 143 survive under 245: unify by DELETING the engine
builtin and letting the command fall through to the plugin.

- [727](plan/learned/727-diag-core.md) -- What the nine built-in diagnostic commands are, and how they split by platform
- [728](plan/learned/728-diag-netlink-monitor.md) -- Netlink monitor streaming (unified output channel, YANG verb tree placement, register_*.go hook bypass)
- [729](plan/learned/729-diag-traceroute.md) -- ICMP traceroute (ttlSetter interface for IPv4/IPv6 TTL, pure Go over library, argTimeout goconst pattern)
- [901](plan/learned/901-env-autocomplete.md) -- How env autocomplete builds its tree without creating nodes
- [738](plan/learned/738-cli-grammar.md) -- Why a closed keyword must come before a free-form value
- [730](plan/learned/730-diag-capture-interface.md) -- AF_PACKET live capture (mdlayher/packet + go-pcap BPF, portable/linux file split, Ethernet link type for raw sockets)
- [755](plan/learned/755-ze-doctor.md) -- How offline readiness checks are classified by severity and platform
- [788](plan/learned/788-doctor-improvements.md) -- How the doctor listener inventory is derived from the schema
- [837](plan/learned/837-doctor-check-registry.md) -- What metadata a doctor check registers, and how its code stays consistent
- [838](plan/learned/838-doctor-check-ownership.md) -- Where a doctor check, its function and its unit test must live
- [863](plan/learned/863-plugin-doctor-checks.md) -- How an external plugin declares doctor checks, and why there are two paths
- [1021](plan/learned/1021-spec-dev-bootstrap.md) -- How `ze-setup` replaced Makefile platform branching, and what drift it tests
- [868](plan/learned/868-test-web-parallel.md) -- How web tests isolate a daemon and a browser session per test
- [872](plan/learned/872-structural-review-fixes.md) -- Which hand-maintained texts became generators, and what verify-gate soundness means
- [791](plan/learned/791-spec-cli-default-format.md) -- How the default CLI output format is configured and overridden per session
- [795](plan/learned/795-cmd-typed-args.md) -- How YANG leaves become typed command arguments and enum suggestions
- [792](plan/learned/792-platform-detection.md) -- How the runtime platform is classified, and which limits are read
- [794](plan/learned/794-cli-session-transcript.md) -- How the CLI session transcript records commands and output
- [796](plan/learned/796-doctor-platform-coherence.md) -- How doctor checks vary by platform, and how they are named
- [803](plan/learned/803-gnmi.md) -- gNMI component: segment-based paths, ChangeNotifier, time.After(0) trap
- [804](plan/learned/804-gnmi-yang.md) -- gNMI YANG config, show command, Prometheus counters, external commit notify
- [809](plan/learned/809-pol-3-validation.md) -- Why a plain filter name is the default operator form
- [810](plan/learned/810-show-command-pipe-filters.md) -- How a command owns its pipe filters, and how lookup stays BGP-free
- [822](plan/learned/822-pipe-first-last.md) -- How `first` and `last` work on both the generic and server-side paths
- [826](plan/learned/826-ipc-dispatch-data-raw.md) -- Why `Data` became `json.RawMessage`, and what callers must stop asserting
- [827](plan/learned/827-dispatch-response-passthrough.md) -- How SDK handlers return Go values while the SDK marshals once
- [829](plan/learned/829-command-verb-first.md) -- Why the root verb set is small, and how deprecated prefixes dispatch
- [1056](plan/learned/1056-cli-grammar-gate.md) -- How the CLI grammar gate mechanizes its rules, and why a path rename breaks the wire
- [1057](plan/learned/1057-cli-grammar-runtime-audit.md) -- Why a live command-list audit is redundant, and what replaced it
- [814](plan/learned/814-pol-4-explain.md) -- How `show policy test` traces a filter chain without applying it
- [849](plan/learned/849-command-surface-ownership.md) -- How a command surface is owned, and where a dedicated feature module fits
- [850](plan/learned/850-appliance-command-plugin.md) -- Where the appliance command lives now, and why 815 was reversed
- [1262](plan/learned/1262-gokrazy-builddir-tmp.md) -- Why every image build runs from a copy, and why a boot proof must check the kernel

## Web Interface

Web UI, HTMX, templates, looking glass, chaos dashboard.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "CLI, web, lookings glass, monitoring"
(retired 266, 268), including the rejected WebSockets, template-file and custom-JS
dashboard designs.

- [741](plan/learned/741-graceful-listener-migration.md) -- Graceful listener migration on config reload (bind-before-close, cross-service conflict detection)
- [756](plan/learned/756-web-auto-reload.md) -- Web UI auto-reload on commit: late-bound commit hook on EditorManager, moved reloadAfterCommit outside apiCfgOK guard

## RIB/Routing

Route storage, selection, forwarding, communities, path selection.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "BGP engine: wire encoding and RIB"
(retired 173, 275).

- 010 -- RIB config design, storage model. Retired and NOT carried into
  DESIGN-HISTORY; that file's header gives the git-recovery route.
- [717](plan/learned/717-rib-2-multicast.md) -- Multicast RPF via generic LPM on sharded Loc-RIB; query-all-shards pattern; wiring gap caught by review
- [772](plan/learned/772-nh-cascade-wiring.md) -- NH cascade wiring: async OnChange to avoid shard deadlock, LPM race fix, ECMP-aware cascade
- [774](plan/learned/774-fib-depth-2-ecmp.md) -- Why ECMP is two mechanisms, and what sysrib actually sees
- [776](plan/learned/776-srv6-prefix-sid.md) -- How SRv6 SIDs are extracted lazily, and where transposition happens
- [906](plan/learned/906-srv6-review-fixes.md) -- Which SRv6 constants must be shared, and what errata 7652 changes
- [783](plan/learned/783-rib-peer-lock-split.md) -- How the RIB peer lock was split for concurrent peer processing
- [784](plan/learned/784-rib-rs-fastpath.md) -- How a forward handle keeps wire bytes zero-copy for state trackers
- [789](plan/learned/789-adjribout-compact-storage.md) -- How compact ribOut storage replaced a per-peer route pointer
- [823](plan/learned/823-rib-show-bounded-dump.md) -- How `show bgp rib` reduced its lock scope while iterating peers
- [824](plan/learned/824-rib-feed-replay-batch.md) -- How replay batching turns a reconnect into O(attribute sets) calls
- [1062](plan/learned/1062-redistribute-late-join-replay.md) -- How redistribute late-join replays, and why a reconnect `.ci` is a false pass
- [1066](plan/learned/1066-as112-bgp-redistribute.md) -- Why originating an AS needs an `origin-as` primitive, not a verbatim AS_PATH
- [1074](plan/learned/1074-unify-route-events.md) -- Why the two route-change batches are layers, and what the bridge was losing
- [1080](plan/learned/1080-unify-redist-loop-guard.md) -- Why the redistribution loop guard compares names, not protocol IDs

## Protocol/RFC

Graceful restart, route refresh, capability negotiation, session management.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "BGP engine: session, FSM, TCP" and "BGP
engine: wire encoding and RIB" (retired 007, 128, 254).

- 033 -- End-of-RIB tracking (RFC 4724). Retired and deliberately NOT carried into
  DESIGN-HISTORY. The EOR facts that survive are recorded there under 013 (EOR after
  an API commit) and 281 (the mandatory `eor` keyword). For 033 itself, that file's
  header gives the git-recovery route.
- [574](plan/learned/574-bgp-4-bmp.md) -- BMP receiver + sender (RFC 7854), config-as-strings, synthetic OPENs
- [647](plan/learned/647-bmp-5-sender-compliance.md) -- BMP sender compliance: real OPENs, Route Mirroring, ribout dedup
- [879](plan/learned/879-l2tp-priority.md) -- L2TP control message priority: Ns-at-send-time invariant enables queue reordering; kernel P-bit limitation
- [919](plan/learned/919-mpls-kernel.md) -- How labeled routes reach the kernel MPLS FIB, and when to replace not add
- [920](plan/learned/920-mpls-ldp.md) -- How LDP reloads interfaces, and what shape plugin config actually arrives in
- [1110](plan/learned/1110-ddos-direction-allowlist.md) -- Why YANG `ordered-by user` order does not survive delivery to a plugin
- [921](plan/learned/921-mpls-rsvp-te.md) -- How RSVP-TE refreshes soft state, and why FRR cannot be an interop peer
- [925](plan/learned/925-mpls-rsvp-te-fast-reroute.md) -- How RSVP-TE facility backup repairs locally, and why swaps must replace
- [923](plan/learned/923-isis-8-dis-broadcast.md) -- How the IS-IS DIS is elected, and how its pseudo-node ages out
- [957](plan/learned/957-ospf-3-ip-transport.md) -- How the OSPFv2 IPv4 transport splits sockets and sets both TTLs
- [958](plan/learned/958-ospf-4-component-config.md) -- How the OSPFv2 config backbone stays self-contained under its plugin
- [959](plan/learned/959-ospf-5-interface-ism.md) -- How the OSPFv2 interface state machine stays separate from the neighbor one
- [960](plan/learned/960-ospf-6-neighbor-nsm.md) -- What the OSPFv2 neighbor state machine owns, and how packets are chunked
- [961](plan/learned/961-ospf-7-lsdb-flooding.md) -- How the OSPFv2 LSDB floods, retransmits and restarts its sequence
- [962](plan/learned/962-ospf-8-spf-rib.md) -- How OSPFv2 SPF installs equal-cost paths into the Loc-RIB
- [972](plan/learned/972-ospf-af-unify.md) -- How one OSPF engine serves both address families through three seams
- [974](plan/learned/974-ospfv3-4-link-lsa.md) -- How the link-scope LSDB works, and why link LSAs must ride DD and LSReq
- [975](plan/learned/975-ospfv3-5-nssa-redist.md) -- How an OSPFv3 NSSA injects Type-7 and the ABR translates it
- [971](plan/learned/971-ospf-14-must-remediation.md) -- Which OSPFv2 RFC MUST gaps were closed, and how the auth boot count persists
- [970](plan/learned/970-ospfv3-3-ipv6-transport.md) -- How the OSPFv3 IPv6 transport finalizes the checksum and demuxes Instance ID
- [969](plan/learned/969-ospfv3-2-wire.md) -- Why every wire codec needs a golden vector, not only a round-trip
- [968](plan/learned/968-ospfv3-1-types.md) -- Why OSPFv3 leaf types are a separate copy, and how a 16-bit LS Type decodes
- [967](plan/learned/967-ospf-13-cli-diag-interop.md) -- How the OSPF CLI, web view, doctor checks and FRR interop are wired
- [966](plan/learned/966-ospf-12-auth.md) -- Where OSPFv2 signs and verifies, and why Keyed-MD5 is not HMAC
- [965](plan/learned/965-ospf-11-stub-nssa.md) -- How stub and NSSA policy is applied, and when the P-bit is set
- [964](plan/learned/964-ospf-10-as-external-asbr.md) -- How OSPFv2 originates AS-External routes and a conditional default
- [963](plan/learned/963-ospf-9-inter-area-abr.md) -- How the ABR summarizes, and why backbone-only acceptance keeps loops out

### OSPF extension family (ext-1..16, unified AF-neutral engine per [972])
- [1029](plan/learned/1029-ospf-ext-1-opaque-framework.md) -- How the opaque-LSA carrier registers consumers and splits the Link State ID
- [1030](plan/learned/1030-ospf-ext-2-traffic-engineering.md) -- How TE LSAs are carried, and why inter-AS TE is AS-wide
- [1031](plan/learned/1031-ospf-ext-3-router-information.md) -- How Router-Information TLVs register, and why AS-wide is not AS-External
- [1039](plan/learned/1039-ospf-ext-4-extended-link-prefix.md) -- How extended prefix and link LSAs correlate to their Router-LSA link
- [1050](plan/learned/1050-ospf-ext-5-segment-routing.md) -- Why an SR forwarding label uses the next-hop router's SRGB, not the originator's
- [1051](plan/learned/1051-ospf-ext-6-ti-lfa.md) -- Why every reachable TI-LFA adjacency-SID repair is a remote-node one
- [1043](plan/learned/1043-ospf-ext-7-virtual-links.md) -- How a virtual link routes across a transit area, and where the V-bit goes
- [1040](plan/learned/1040-ospf-ext-8-nbma-p2mp.md) -- How NBMA and point-to-multipoint network types originate host routes
- [1041](plan/learned/1041-ospf-ext-10-bfd.md) -- How OSPF subscribes to BFD without importing the component
- [1042](plan/learned/1042-ospf-ext-11-ldp-igp-sync.md) -- How LDP-IGP sync costs out only the transit link, not the stub
- [1044](plan/learned/1044-ospf-ext-9-graceful-restart.md) -- How graceful restart rides the opaque carrier, and why a v6 sentinel is needed
- [1036](plan/learned/1036-ospf-ext-12-multi-instance.md) -- How multi-instance OSPFv2 isolates transports and reads the Instance ID
- [1052](plan/learned/1052-ospf-ext-14-debug-introspection.md) -- How OSPF debug tooling is double-gated, and which denial actually fires
- [1037](plan/learned/1037-ospf-ext-15-multi-af.md) -- How per-family OSPFv3 engines spawn, and why the v4 injector needs a fallback
- [1038](plan/learned/1038-ospf-ext-16-ipsec-auth.md) -- Why OSPFv3 IPsec needs one wildcard SA rather than two transport SAs

## Observability

Metrics, telemetry, Prometheus exporters, third-party format compatibility.

- [653](plan/learned/653-netdata-os-collectors.md) -- What the OS collector framework measures, and why names come from source
- [736](plan/learned/736-iface-rate.md) -- How the interface rate tracker gauges raw stats and cleans stale labels
- [808](plan/learned/808-smart-management.md) -- How SMART disk health is read without smartctl, and how it alerts

## Security/Auth

Authentication, authorization, appliance bootstrap, and credential boundaries.

- [831](plan/learned/831-appliance-auth-hardening.md) -- Where appliance local admin credentials live, and which paths enforce RBAC
- [1159](plan/learned/1159-fixit-cli-credential-resolution.md) -- Why the CLI client is not an identity authority, and who owns prompting

## Agent Workflow

Agent rules, self-improvement, discovery paths, and development-time inventories.

- [832](plan/learned/832-self-improving-discovery.md) -- Which discovery surfaces a feature, tool or check change must update
- [833](plan/learned/833-commit-helper-tooling.md) -- What `commit_helper.py` owns, and which gates it applies at creation
- [835](plan/learned/835-commit-request-fast-path.md) -- Why an explicit commit request is a fast path with no verify rerun
- [876](plan/learned/876-commit-helper-session-collision.md) -- Why a per-session tmp artifact needs a session-unique path component
- [1006](plan/learned/1006-feature-surface-gate.md) -- What the Feature Surface Gate adds to `/ze-spec` and `/ze-review`
- [1086](plan/learned/1086-claude-runs-commit-script.md) -- Why Claude runs the commit script, and what stays banned as a bare call
- [1155](plan/learned/1155-learned-numbers-collide-across-branches.md) -- Why learned numbers collide across branches, and what detects it
- [1306](plan/learned/1306-delegation-reminder-position.md) -- How a rule loses on position, and why the reminder must arrive last
- [1314](plan/learned/1314-rule-heading-inverted-its-directive.md) -- How a rule loses on its own heading, the companion failure to 1306
- [1308](plan/learned/1308-stop-hook-reregistration.md) -- Why registering a hook is half a fix, and what the claim marker must outlive
- [1309](plan/learned/1309-detail-budget.md) -- Detail is a cost the reader pays. Verification is an action, the citation is a choice
- [1310](plan/learned/1310-phase-gates.md) -- Two rules with no gate: use the skill, and implement on the implementation model
- [1093](plan/learned/1093-followup-hooks.md) -- Which three dead agent-guard hooks were enabled, and where they now run

## Testing

Test patterns, infrastructure, chaos testing.

Pre-401 history: `plan/learned/DESIGN-HISTORY.md`, "Testing infrastructure" (retired 265, 274).

- 258 -- Chaos family fuzzing. Retired and NOT carried into DESIGN-HISTORY; that
  file's header gives the git-recovery route.
- [1172](plan/learned/1172-rfc-requirement-coverage.md) -- How RFC MUST requirements bind to tests, and how a green pilot lied
- [1313](plan/learned/1313-rfcgate-1b-rfc7296-pilot.md) -- Guards inert on empty, oracles satisfied by any error, and mutations aimed at a discarded module
- [608](plan/learned/608-concurrent-test-patterns.md) -- Which concurrent-test flake shapes recur, and how each one is fixed
- [881](plan/learned/881-test-flake-under-load.md) -- Why a wall-clock timeout fails under host load, and how a run is labeled
- [723](plan/learned/723-chaos-actions-v2.md) -- Parameterized chaos actions: string-map params over typed unions, opt-in scheduling, per-instance weights
- [787](plan/learned/787-chaos-inprocess-scheduling.md) -- How in-process chaos drives existing schedulers from a virtual clock
- [797](plan/learned/797-interop-gap-coverage.md) -- Which five interop gaps were covered, and against which peer daemons
- [800](plan/learned/800-bgp-chaos-integration.md) -- Chaos integration tests: fork mode default, run() testability, port auto-allocation
- [802](plan/learned/802-chaos-multi-target.md) -- Multi-target chaos: FRR/BIRD config gen, temp-file fork, single-port dialing, BIRD channel mapping limits
- [842](plan/learned/842-scoped-verify-committed-gap.md) -- Why scoped verify missed a committed regression, and how the set widened
- [843](plan/learned/843-verify-debugging-protocol.md) -- How a verify failure index is grouped and emitted for machine reading
- [892](plan/learned/892-spec-validate-command.md) -- What `make ze-validate` checks after verify, and why it greps
- [908](plan/learned/908-test-trace-mode.md) -- How per-step trace output is formatted for humans and machines
- [1100](plan/learned/1100-followup-l2tp-call.md) -- How a `.ci` drives a mutating RPC, and why xl2tpd cannot answer OCRQ
- [1120](plan/learned/1120-payload-predicate-waits.md) -- Which predicate waits replaced sleeps, and why the ratchet baseline drifted
- [1171](plan/learned/1171-fixit-reject-fence-observability-deferred-external-plugin-signals.md) -- How a stderr fence waits on a refusing plugin, and why an observer cannot
- [1101](plan/learned/1101-followup-test-infra.md) -- Which test infrastructure landed, and where the LLGR egress rail is not wired

## Build/Deployment

Build system, Docker, CI, toolchain upgrades.

- [753](plan/learned/753-docker-go126.md) -- Docker support: two-stage build to scratch, CGO_ENABLED=0, Go 1.26 upgrade across all project references
- [754](plan/learned/754-makefile-split.md) -- Makefile split into mk/ includes: tiered help, component test groups, contributor testing docs
- [853](plan/learned/853-build-tag-split.md) -- Why positive build tags replaced the negative stripped tag
- [854](plan/learned/854-install-8-appliance-iso.md) -- How the appliance ISO wraps a raw image, and how it boots under UEFI
- [870](plan/learned/870-kernel-build-convergence.md) -- How the runtime and installer kernels converged on one builder
- [1103](plan/learned/1103-fixit-appliance-evidence-config.md) -- Why appliance readiness needs the SSH banner, not any TCP listener
- [1106](plan/learned/1106-gokrazy-l2tp-evidence-networking.md) -- Why the L2TP evidence harness needs TAP instead of slirp, and a kernel log knob
- [1105](plan/learned/1105-vpp-host-tuning.md) -- How VPP host tuning and hugepage reservation reach the image config

## Gotchas

Reusable lessons extracted from gotchas sections across summaries.

- (001) Freeform config parsing without schema causes data extraction failures; schema-driven (YANG) prevents this
- (008) Preserve insertion order of conditional rules -- they apply sequentially, not by specificity
- (013) EOR semantics extend RFC 4724 beyond graceful restart; document RFC violations when accepting them
- (102) Over-engineered specialized types (e.g., Span) often lose to native types; prefer native until proven insufficient
- (133) Renaming packages to short common nouns causes variable shadowing in callers
- (149) Do not force async protocols for in-process communication; adds complexity for no benefit
- (165) Organizational separation is distinct from protocol redesign; reuse existing infrastructure
- (176) Preserve attribute flag values separately -- do not hardcode reconstruction flags
- (247) Check dependency graphs before large restructurings; circular imports are blocking
- (253) Import cycles in test files reveal over-tight coupling; use external test packages
- (266) SSE debounce in HTTP layer prevents blocking the main event loop
- (275) Concurrent sends racing with channel close require WaitGroup coordination to avoid panic
- (647) Early return in event handler blocks housekeeping (caching, cleanup) that must run regardless of sender state
- (647) KEEPALIVE has nil RawBytes; check RawMessage != nil, not RawBytes != nil, for messages with no body
- (652) Verify "does not exist" claims during child spec RESEARCH; umbrella assumed show interface was missing but it was fully implemented
- (652) subsystem-list was hardcoded to ["bgp"]; always check stub implementations before assuming real data flows
- (708) Boolean three-state offloads avoid VyOS bootstrap activation script; ethtool_sset_info.sset_mask is __u64 not __u32; cap kernel-reported feature count before allocating
- (720) Editor commit-triggered archives run in CLI process, cannot emit daemon events; EventEmitter callback pattern enables future plugin backends
- (731) Plugin-to-infrastructure state exposure via registry.SetNTPSyncProvider; plugins register RPCs in their own init() to avoid import boundary violations
- (733) PKI store: shared certificate infrastructure; config.Tree lists via AddListEntry not GetOrCreateContainer; ECDSA keySize via Curve.Params().BitSize not generic Size()
- (734) IPsec data model: L2TP-pattern tree-walker parser; algorithm enum strings match strongSwan naming for ipsec-4 consumption; DHGroup as uint8 with range validation over iota enum; cross-ref validation via function callbacks for testability
- (735) XFRM interfaces: standalone type not TunnelKind (no endpoints); VTI deliberately excluded; GetXFRMInfo for operational display of managed and unmanaged interfaces; XfrmPolicyList is system-wide, filter by Ifid client-side
- (737) IPsec EAP extension: extend AuthMode enum not separate enum; EAP auth shares x509 cert branch; single pool per remote-access; match parser method to YANG node type (leaf vs list)
- (739) IKEv2 crypto primitives: flat map registry over registration pattern for fixed algorithm set; MODP private keys in [2,p-2]; prf+ capped at 255 iterations; constant-time PKCS#7 unpadding; Go GC prevents secure memory wiping
- (740) IKEv2 engine: plugin registration for config delivery; JSON-to-Tree dual storage for container/list ambiguity; per-peer goroutine lifecycle from PPPoE pattern; constant-time PSK verification; store peer config in PeerSession not SA to avoid reconcile race
- (742) Child SA and dataplane: Dataplane interface with Register/Load/Get following iface.Backend pattern; dp.Get() returns nil if Load() never called (silent no-op trap); IfID must come from config not runtime lookup (plugin subprocess has no iface backend); VPP backend compiles but needs govpp/binapi/ipsec vendored; DPD lastSent must init to now() not zero
- (744) EAP and NAT-T: MD4 removed from Go, must implement from scratch; RFC 2759 magic constants exclude trailing null (RFC C array size is off-by-one); EAP-TLS pipes crypto/tls through async net.Conn transport; DetectNAT must accept []byte not [8]byte for 20-byte SHA-1 hashes; NATDetected flag on SA propagates to child SA UDP encap automatically
- (745) IPsec CLI and diagnostics: show/clear/monitor commands for IKE SAs, Child SAs, proposals; ipsec-status daemon RPC returns structured IKE/Child SA state
- (805) EAP interop: initiator must omit AUTH in first IKE_AUTH to signal EAP willingness; server AUTH verified before EAP processing; eap/peer.go separates initiator dispatch from server-side eap.go; shared test PKI across scenarios 03/04; Alpine strongSwan includes eap-mschapv2 and eap-tls plugins natively
- (748) Self-update: SelfUpdater parallel to UpdateChecker (hub starts one or the other based on auto-apply); FNV-1a spread deterministic per device+version; manifest field named Ver to avoid hook false positive; atomic rename with .prev hard-link backup; history persisted across restart
- (749) AI agent tooling: lower-kebab diagnostic codes over short prefixes; dropped errors/warnings arrays (diagnostics is single source); repair metadata on warnings is intentional for fix-plan; FindListenerConflict returns structured pair without breaking ValidateListenerConflicts callers; block-version-config hook needs string-concatenation workaround in test files
- (753) `scratch` Docker image has no shell; `docker exec` debugging requires a multi-stage override or separate debug image
- (754) Adding a named test group to `mk/test-unit.mk` requires adding the exclusion pattern to `ZE_GROUP_REST`
- (756) Web commit hook runs synchronously in HTTP handler; slow reload blocks the response (acceptable because reload is <100ms)
- (759) Archive pruning prefix computed by diff of two timestamps at different dates; if filename format has no time token, prefix equals full filename
- (768-a) Enum-over-string for text event pipeline: Event.Type stays string (non-BGP types), TypeKind caches the parsed EventKind; FamilyOperation.Action typed as RouteAction; local familyOperation types in format/rs/rr/persist are independent; NLRI index map[string] is Go-idiomatic and not improvable
- (768-b) Doctor health checks: unconditional prefix counting needs familyString early-return guard; AuditTables must guard nil LastApplied; VPP health check must gate on socket existence; health check kernel calls need 1s timeout goroutine; plugin-crash error must come after validation guards; EOR timer needs familyCount=0 guard; pending map needs cap
- (786) Backend-specific health checks belong in their backend plugin packages (RegisterHealthCheck pattern), not cmd/show/; block-init-register hook requires explicit registration, not direct health.Register in init()
- (790) Debug flags: three-tier resolution (global > per-subsystem > default) in zefs `state/debug/` keys; `storage.BlobStoreFrom()` exposes underlying BlobStore from Storage interface; `state/` is the new namespace for runtime state keys
- (797) Interop helpers must not bind peer IPs as default args (subnet chosen at setup time); BMP sidecar must start before Ze (races PeerUp); structured adj-rib-in must preserve legacy IPv4 NEXT_HOP from path attributes; BGP interop runner accepts only one scenario filter argument
- (801) YANG sub-containers do not work for positional-argument CLI commands: dispatcher prefix-match breaks when a `<name>` arg sits between the command and sub-keyword; route sub-commands inside the handler via arg inspection instead of separate wire methods
- (833) Commit script generation should be mechanical: use `scripts/dev/commit_helper.py` so session IDs, message files, ignored-path checks, executable scripts, and learned-summary decisions are enforced before the user runs the script
- (835) When the user asks for a commit, do not re-audit implementation or rerun gates. Use `scripts/dev/commit_helper.py` immediately, and if `scripts/dev/verify-status.sh check` is FRESH, never rerun `make ze-verify` or `make ze-verify-changed`
- (871) Thomas Owner Override lets the repository owner explicitly request commit-script preparation without a fresh verify run. It was added for OpenAI rigidity, not Anthropic. Agents still use `commit_helper.py`, never disable hooks, and report skipped verification plainly
- (874) Leaf-list nodes MUST use the multi-value Tree API in every write/apply path: serializers read multiValues, scalar Set is silently dropped. Session change-tracking is per-(path,member) via MetaEntry.Member; ordered member ops (insert/deactivate/activate) are structural ops so position is exact at commit. SSH session editors now wire SetReloadNotifier (commit = apply + propagate); doReload rewrites resolv.conf
- (890) Env-to-YANG promotion for BGP plugin config: create plugin yang/ module augmenting /bgp:bgp, add ConfigRoots+Features+YANG to registration, OnConfigure handler applies YANG value via SetChanSize, env var renamed to mirror YANG path per config.md
- (917) SR-Policy migration + encoding: InProcessConfigRouteParser on plugin Registration replaces hardcoded family switch cases. PluginRoute carries pre-built NLRI + attribute bytes. fullRawAttribute must strip header for WriteAttrTo; ExaBGP TunnelEncap uses draft sub-TLV types. MPLS S-bit MUST be zero per RFC 9830; both Ze and ExaBGP were non-compliant. Bridge must pass tunnel-encap tokens through, not drop them.
- (1009) Spec selection moved from a shared `tmp/session/selected-spec` file (append/remove discipline, concurrency-fragile, dead commit gate) to per-session claims via `scripts/dev/spec-session.sh` into each session's own `.session-<SID>` marker; auto `ready->in-progress` transition moved into `claim`; helper must not use `set -u` (shared session-id.sh reads an unset env var)
- (1047) Offline-fallback for read-only daemon commands: `registry.RegisterOfflineFallback(path, handler)` serves host-local reads (show host, show crashes) in-process only when the daemon is unreachable, never shadowing the daemon. It is dead code unless `cmdutil.RunCommand` is patched to skip the "unknown command" rejection for fallback-registered paths (RunCommand rejects non-tree commands before cli.Run). Verb-first CLI cleanup: debug set/delete (VyOS-style, not invented enable/disable), event list -> show event list. Moving a YANG ze:command container changes its dispatch key (LoadBuiltins keys on YANG path, command.go), so a "verb-first rename" of a command a plugin sends by its bare path is a wire break -- grep for senders first. command list/help/complete -> show command list/help/complete (its one sender, plugin-cli-debug.ci, updated); plugin encoding/format/ack STAY noun-first (session directives; set plugin collides with the config-tree plugin node)
- (1078) BGP received-UPDATE filtering unified into ONE Stage-ordered pass: the external per-peer `PolicyFilterChain` becomes a single ordered step at new terminal `filterapi.FilterStagePeerChain=300`, which runs AFTER OTC/Annotation (the original spec had the order backwards -- code ran the whole in-process pass, incl OTC, before the external chain). `filterapi` stays a stdlib leaf, gaining only the stage constant + exported `LessOrder` comparator; the reactor binds the step at `startAPIServer`. Egress is order-only at `forwardUpdateCore`; the 3 single-kind egress paths (RS fast path, injected routes, default-originate) are intentionally left separate (unifying them would change behavior). Egress is asymmetric to ingress: egress in-process filters defer into `ModAccumulator` and the policy chain reads the ORIGINAL payload, so ingress/egress use separate step-result types
- (1082) Unified the `{status,data,error}` command-result envelope + dispatcher across every surface: `plugin.Response` wins in place; `api.ExecResult`/`CallerIdentity`/`Executor` and the five surface `CommandDispatcher` types become ALIASES of `plugin.CommandDispatcher`/`plugin.CallerIdentity` -- aliases keep the names so only the ~15 dispatcher CALL sites change, not the ~31 web signatures that merely thread it. Flatten centralized once in `plugin.ResponseJSON` + `CommandDispatcher.JSON`; the API engine returns the typed `*Response` directly (finding-3: drops the marshal-to-string then reparse round trip). New `plugin.Text` renders pre-rendered text verbatim on text surfaces (web BGP-decode) but encodes as a JSON string in the API. GOTCHAS: the envelope is MARSHAL-ONLY (`Data` is the `ResponseData` marker interface, so `json.Unmarshal` into `plugin.Response`/`api.ExecResult` fails when a `data` field is present; no production code unmarshals it). Removing the API re-parse changes REST/gRPC JSON key ORDER + number fidelity (int64 no longer coerced to float64) -- semantically equal and more faithful, NOT byte-identical; do not "restore" it by re-adding the round trip. `serverDispatcher` must NOT thread `context.Background()` as `RequestContext` -- the old adapter left it nil so `CommandContext.Context()` fell back to the shutdown-cancellable SERVER context; thread only a genuine per-request ctx (verify context-fallback claims against `CommandContext.Context()`, not intuition). `validate.py` floods with unwired-export false positives on a handler-heavy refactor (see 1081).
- (1081) Late-join replay unified onto ONE vocabulary in new leaf `internal/core/replay` (`Request{ReplayID}` token + `Broadcast=MaxUint64` sentinel + `IsReplay`); the token-correlated shape wins because broadcast is the special case where the token addresses everyone (a payload-less signal cannot target one consumer). `redistevents.ReplayRequest` becomes a `type = replay.Request` alias so producers/orchestrator churn zero. Two payload-less `RegisterSignal` broadcasts (bgp-rib, system-rib) become `Register[*replay.Request]`; two write-only `Replay bool` fields retire to a token-derived marker via a `type alias`+embedded-pointer `MarshalJSON`/`UnmarshalJSON` that preserves the legacy `replay` bool wire (best-change round-trips through JSON in-tree). `RegisterSignal`/`SignalEvent` now have zero users but stay as a primitive. GOTCHA: touching a widely-imported core leaf makes `changed-pkgs.sh` return ~200 reverse-dep pkgs, so `ze-lint-changed`/`ze-validate` scan that closure and surface PRE-EXISTING debt (a `goconst` already red on main; `validate.py` unwired-export false-positives -- it can't see `RunEngine: RunFoo` func-values, same-package-only use, or map-value type usage) in every file the change edits; budget for it. `json.Marshal` never dispatches a pointer-receiver `MarshalJSON` on nil (returns null), so the marshaler is nil-safe.
- (1094) Web/CLI UX finish: `helpfmt.RenderWriter` is a shared error-capturing render writer with return-less `Str`/`Line` methods (chosen over `fmt.Fprintln(rw,...)`, which errcheck flags on a custom writer -> ~250 nolints); CLI render paths now exit non-zero on a broken pipe. `zeweb.WebRoute{Pattern,Wrap,Build}` + optional `Enabled`/`Portal` registry replaces hardcoded l2tp/isis/ospf/gokrazy routes -- features self-register from `register_*.go`, the hub iterates (http.ServeMux panics on a leftover duplicate). i18n = catalog+English-fallback `Translate` wired as a `t` FuncMap helper (login page only). `.wb` harness gains `option=viewport/locale/auth` + `action=login` (unit-tested via the `installFakeAgentBrowser` command-log seam). GOTCHA: byte-identity of `ze help` must be checked by sorted-set diff (map-iteration non-determinism in `aihelp.Services`); gosec G101/G117 + misspell false-positive on translation keys and the French word "Connexion"; Chrome is env-blocked (`libatk`) so all `.wb` browser proofs are Go-tier
- [1099](plan/learned/1099-iface-resolve-0-umbrella.md) -- What the iface-resolve umbrella delivered, and which gate outlived it
- [1098](plan/learned/1098-followup-vpp-iface.md) -- How the VPP iface backend does tunnels, mirroring, WireGuard and LCP
- [1097](plan/learned/1097-followup-vpp-traffic.md) -- Why VPP `filter dscp` polices by DSCP rather than remarking it
- [1095](plan/learned/1095-followup-subsystem.md) -- How DoT and DoH extend the DNS harness, and which DNSSEC model applies
- (1085) Config leaf-list per-member deactivation moved from an in-band `inactive:` value prefix to an out-of-band `Tree.inactiveMembers` map; `GetSlice`/`GetMultiValues`/`ToMap` are now active-only (deactivated = not in effect), a new `GetMultiValuesState` serves serialize/diff/reactor. The reactor filter chain went `[]string` -> `filterapi.FilterRef{Name,Inactive}` end-to-end; `inactive:` survives ONLY at input (parse normalization), serialize output, and display/API seams (`FilterRefStrings`). GOTCHAS: the spec's premise that per-member deactivation reaches the reactor via `ToMap` was FALSE (it reaches it via `GetMultiValues`/`extractFilterChain`); A-1's "prefix never a valid input" was validated with a grep that missed Go-embedded config seeds AND `docs/guide/redistribution.md`, which documents the inline `[ inactive:X ]` form -- it is a real feature, normalized at the parse boundary (`stripInactiveMemberPrefix`), not an artifact to delete.
- [1104](plan/learned/1104-startup-resilience.md) -- Why an external client must never dial in a constructor or apply callback
- [1102](plan/learned/1102-followup-bgp-feature.md) -- Which BGP follow-up items landed, and why a spy-metric test needs a red run
- [1111](plan/learned/1111-ownership-0-umbrella.md) -- Why the ownership review misread the wiring, and which three remnants were real
- [1112](plan/learned/1112-netlink-ci-harness.md) -- How the netlink `.ci` harness enters a namespace and drops privilege
- [1165](plan/learned/1165-fixit-vpp-lcp-netns-remediation.md) -- How a doctor check told operators to break their dataplane, and why it survived
- [1169](plan/learned/1169-cli-root-namespace-grammar.md) -- Why root commands need their own grammar feeder, and what renaming them collided with
- [1225](plan/learned/1225-rfc7606-relay-shape.md) -- How RFC 7606 5.1 closed on the relay path, and why an absence test proved nothing
