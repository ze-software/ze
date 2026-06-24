# Learned Summaries Index

Curated index of `plan/learned/` summaries that capture structural decisions, patterns, and gotchas.
Task-completion-only summaries (the majority) are omitted. Full list: `ls plan/learned/`.

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

- [001](plan/learned/001-initial-implentation.md) -- Zero-copy ContextID, per-type pools, lazy iterators, goroutine-per-peer
- [013](plan/learned/013-unified-commit-system.md) -- CommitService abstraction, grouping by config not command, implicit EOR
- [015](plan/learned/015-fsm-active-design.md) -- FSM active connect state machine design
- [133](plan/learned/133-internal-migration.md) -- internal/ package restructuring, reactor -> component/bgp
- [149](plan/learned/149-unified-subsystem-protocol.md) -- Unified subsystem protocol, in-process vs async
- [157](plan/learned/157-hub-separation-phases.md) -- Hub separation into 7 phases
- [165](plan/learned/165-reactor-service-separation.md) -- Reactor service separation from protocol
- [244](plan/learned/244-reactor-interface-split.md) -- Reactor interface split for testability
- [247](plan/learned/247-plugin-restructure.md) -- Plugin restructure, circular import resolution
- [760](plan/learned/760-subscriber-session-model.md) -- Unified subscriber session model: shared Session struct across PPPoE/L2TP, handler delegation, event bridge pattern
- [761](plan/learned/761-vpp-fib-query.md) -- RouteLookup added to Backend interface: dispatch through active backend, VPP via IPRouteLookupV2, eliminates netlink bypass
- [762](plan/learned/762-rs-dynamic-peers.md) -- IXP route server: dynamic peer groups from prefix ranges, RS-client transparent AS-path (RFC 7947), community-based selective forwarding, connection-time peer creation bypassing parsePeerFromTree
- [763](plan/learned/763-backend-aware-completion.md) -- ze:backend extended from commit-time validation to completion-time filtering: config completer reads entry.Exts, command tree stores Backend on Node, backends derived from config tree not component imports
- [912](plan/learned/912-irr-prefix-store.md) -- Shared IRR PrefixStore in resolve/irr/store subpackage: per-entry zefs keys (meta/irr/{name}), cross-process sharing via disk not a struct, subpackage breaks the peeringdb import cycle (no interface needed), read-first Open (write lock only to migrate)
- [945](plan/learned/945-tiers-1-rule-and-audit.md) -- Module tiers (core/component/plugin) decided by dependency direction, made auditable: `ai/rules/module-tiers.md` + `dep_audit.py --check` engine gate (`make ze-tier-check`); transitional migration baseline (not an allowlist); gate must reuse the generator's pluginDirs (engine-package granularity excludes nested sub-plugins)

## Wire/Encoding

Buffer-first, zero-copy, attribute pools, UPDATE building, NLRI parsing.

- [059](plan/learned/059-spec-pool-handle-migration.md) -- Pool handle migration from mutex stores
- [073](plan/learned/073-spec-buffer-writer.md) -- BufWriter WriteTo(buf, off) int pattern
- [076](plan/learned/076-spec-wire-update.md) -- WireUpdate lazy parsing design
- [092](plan/learned/092-pack-to-writeto-migration.md) -- Pack() to WriteTo() migration
- [102](plan/learned/102-buffer-first-migration.md) -- Buffer-first migration, Span type abandoned
- [105](plan/learned/105-pathattributes-removal.md) -- PathAttributes struct removal (lazy over eager)
- [176](plan/learned/176-per-attribute-deduplication.md) -- Per-attribute-type pool dedup design
- [204](plan/learned/204-update-shared-parsing.md) -- Shared UPDATE parsing for wire/API
- [721](plan/learned/721-bgp-2-aspa.md) -- ASPA path verification: RTR v2 per-session version, ROACache O(1) counter, route tracker for re-validation
- [722](plan/learned/722-spec-bgp-4-aspa-policy.md) -- ASPA policy enforcement: override ordering (ASPA reject wins over origin accept), re-validation via validateCh, origin policy is hardcoded not configurable
- [764](plan/learned/764-attr-flags-json.md) -- Attribute flags in Ze native JSON: includeFlags parameter on shared formatter, static flags for pool RIB, unwrap at extractRoutes for LG consumers
- [765](plan/learned/765-gc-pressure-reduction.md) -- GC pressure reduction: [256]bool for attr codes, inline FNV-1a, NextHopAddrs inline struct, LargeCommunities dedup fast path, clear() map reuse; stack arrays that escape via closure/interface/return are NOT optimizations
- [821](plan/learned/821-spec-attrpool-shard.md) -- attrpool sharding: split single RWMutex into 16 content-hashed shards, shard id in high 4 bits of the 26-bit Slot field (Handle ABI unchanged), fan-out compaction leaves scheduler untouched, read path 7.3x; h.Slot() is composite shardID<<22|slot not a dense index
- [767](plan/learned/767-tokenizer-no-escape.md) -- Command tokenizer: removed backslash escape handling, backslash is a normal character; no per-byte escape scan on every command
- [770](plan/learned/770-precomputation-review.md) -- Precomputation critical review: 3 of 7 proposals rejected after source validation showed trivially cheap operations; profile before optimizing
- [771](plan/learned/771-performance-optimization-campaign.md) -- Full optimization campaign: Ze 91ms->71ms convergence (22%), BIRD gap from 1.82x to 1.61x; profile-driven, lifecycle-boundary precomputation
- [859](plan/learned/859-perf-hot-alloc-reduction.md) -- Hot-path allocation reduction: value-type struct keys (netip.Prefix, compactRouteKey, withdrawalKey) replace string maps across rib/adj-rib-in/RS; adj-rib-in bypasses bgp.Event; Ze 71ms->62ms, now matches BIRD
- [875](plan/learned/875-filter-delta-parse-once.md) -- Filter modify path parses filter text once and passes maps to the three extractors (was 4 parses per modified UPDATE); golden-corpus equivalence gate for AS-path refactors; modify path -39% time, -29% allocs
- [785](plan/learned/785-rfc7606-validation-cache.md) -- RFC 7606 validation cache: rejected, validation too cheap (67-138 ns) to benefit from caching despite 77-97% hit rate; cache lookup cost rivals validation cost

## Plugin System

Registration, SDK, event flow, lifecycle, hook integration.

- [253](plan/learned/253-nlri-plugin-extraction.md) -- NLRI codec extraction to plugins
- [905](plan/learned/905-bgp-family-checklist.md) -- BGP family integration checklist: 12-section pattern from SR-Policy/PATHS-LIMIT/SRv6 post-mortem; BLOCKING gates in /ze-spec and /ze-implement
- [757](plan/learned/757-typed-route-result.md) -- Typed RouteResult replaces map[string]any in update-route, eliminating int/float64 transport divergence
- [781](plan/learned/781-remove-private-as.md) -- Remove-private-as: plugin intent + reactor wire rewrite; Set+Prepend composition
- [806](plan/learned/806-install-1-dhcp-pxe.md) -- DHCP PXE extension: additive options in existing buildReply, server-wide pxeConfig, 1500-byte reply buffer, siaddr+option 66 dual-set for PXE ROM compat
- [807](plan/learned/807-install-2-tftpserver.md) -- TFTP server plugin: RFC 1350 read-only, ephemeral port per transfer, channel semaphore for concurrency, SO_BINDTODEVICE on Linux, 32MB block-number limit acceptable for bootloaders
- [811](plan/learned/811-install-3-image-server.md) -- imageserver plugin: own HTTP listener, http.ServeFile for images/boot/zefs, path traversal via flat filename validation, zefs built at configure time with ze init key patterns
- [867](plan/learned/867-yang-rename-ownership.md) -- YANG rename (schema/ -> yang/) + command YANG ownership (component/ -> plugins/): folder test, codegen cleanup gaps, acronym map, intrinsic vs removable subsystems
- [977](plan/learned/977-traffic-usage.md) -- Pure-Go eBPF TCX plugin (traffic-usage): asm.Instructions assembled in-process, no C/.o/clang; BPF_PROG_TEST_RUN tests load-bearing; parse-then-account avoids verifier packet-pointer invalidation; StoreImm has no DWord form; go mod vendor needs chmod -R u+w
- [812](plan/learned/812-install-5-bootstrap-mode.md) -- bootstrap mode: EmitBootstrapConfig separate from EmitConfig, ethernet-only DHCP, SSH from zefs creds, falls through on failure, inserted between template and web-only in startup switch
- [813](plan/learned/813-install-6-installer-initrd.md) -- installer initrd: busybox-based shell init, wget+dd stream to first non-removable disk (sysfs removable check), blockdev --rereadpt for partition re-read, zefs injection to /perm/ze/, kernel-level DHCP via ip=dhcp cmdline
- [815](plan/learned/815-install-7a-namespace.md) -- namespace migration: cmd/ze/appliance/ moved to cmd/ze/install/appliance/, deprecated alias prints warning, no root registration at new location
- [816](plan/learned/816-install-7b-vendor-builder.md) -- vendor builder: gokBuildFn replaces gokBinary shell-out, go run -mod=vendor ./cmd/ze-gok, GOMODCACHE isolation, gokSizeArg uses strconv.AppendInt
- [817](plan/learned/817-install-7c-vendor-updater.md) -- vendor updater: local copy of gokrazy/updater (stdlib-only), authTransport for all requests, StreamTo+Switch+Reboot sequence, --testboot/--no-reboot flags
- [818](plan/learned/818-flow-export-1-counter-export.md) -- flow export counter closure: IPFIX octetTotalCount/packetTotalCount (not Delta), single MaxDatagramSize, per-datagram metric counting, template timestamp on success only, show flow-export YANG wiring
- [819](plan/learned/819-flow-export-2-flow-records.md) -- flow export spec-2 integration: FlowSample/ConntrackFlow neutral types + factory-registered flow encoders (import-graph constraint), platform-independent workers delegating to _linux netlink, BestChange typed-handle enrichment (next-hop only), deadlock-safe Stop ordering
- [820](plan/learned/820-flow-export-0-umbrella.md) -- flow export umbrella: single-collection/multiple-consumers via iface callback, registration over imports, buffer-first, in-process SDK component; spec-1 complete, spec-2 CI-gated
- [821](plan/learned/821-plugin-internal-keyword.md) -- Plugin internal keyword: explicit `plugin internal <name> { use <builtin> }` config, cross-list uniqueness, internal plugins do not use external encoder/respawn/timeout leaves
- [884](plan/learned/884-cos-plugin.md) -- CoS plugin: shared registry in core/cos, YANG container-merge over augment for interface binding, InProcessConfigVerifier ordering relies on alphabetical registry.All() ("cos" < "interface")
- [887](plan/learned/887-cos-vendor-radius.md) -- Vendor VSA CoS/rate: hardcoded 5-vendor switch over registry, MikroTik rate as "Nbit" FilterID string for shaper pipeline, Ze "cos:" prefix wins over vendor VSA
- [894](plan/learned/894-show-enricher.md) -- Show enricher registry: core/show leaf package, plugins register enrichers for show commands via init(), handlers call Enrich()/EnrichBrief(), panic recovery, alphabetical ordering
- [895](plan/learned/895-show-enricher-v2.md) -- External enrichment: declare at Stage 1, proxy enricher in server, ze-plugin-callback:enrich-show with 2s timeout, Unregister for cleanup, web service-locator explicit Enrich(), fakeenrich test plugin
- [828](plan/learned/828-codec-callback-passthrough.md) -- NLRI decode single-marshal: DecodeNLRIHex returns any, registry marshals once; RunCLIDecode callers share function so need own marshal
- [830](plan/learned/830-typed-inter-plugin-dispatch.md) -- Typed exact-command inter-plugin dispatch: `DispatchCommandArgs` over rebuilt strings, `CommandArgsAuthorizer` over canonical fallback, command/args boundary pinned by tests
- [858](plan/learned/858-typed-peer-selector.md) -- Typed peer selector: BGPReactor takes `*selector.Selector` not string; SDK `*Sel` variants for DirectBridge; `SoftClearPeer` fixed for name/ASN; GR stays string at args boundary
- [922](plan/learned/922-cross-plugin-switch-audit.md) -- Cross-plugin switch audit: most are correct Go (backend lowering has no virtual-dispatch alt); only producer-owned re-derivation is a smell -> hoist as a method on the producer's type (`RouteAction.Verb()`, `Path.IsEBGP`, `AddPathMode.Label()`); 3 dup-hidden bugs fixed; `ze-validate` undercounts a typed enum reached only via its constants (fixed in validate.py)
- [938](plan/learned/938-bug-review-0-umbrella.md) -- Review-only bug campaign pattern: generated aggregator inventory first, child reports by owner, accepted findings routed to TDD-ready fix specs, closure must include spec audits and fix-spec audit sections before final report claims artifact completion

## Configuration

YANG schema, migration, config reload, editor, environment variables.

- [008](plan/learned/008-config-migration-system.md) -- Heuristic version detection, 3-version migration chain
- [065](plan/learned/065-spec-remove-version-numbers.md) -- No version numbers in config (YANG-transformable)
- [166](plan/learned/166-yang-only-schema.md) -- YANG as sole schema source of truth
- [175](plan/learned/175-config-editor-validation.md) -- Config editor validation pipeline
- [226](plan/learned/226-config-reload-6-remove-bgpconfig.md) -- BGPConfig removal, map[string]any
- [232](plan/learned/232-editor-tree-canonical.md) -- Editor tree canonical representation
- [716](plan/learned/716-iface-2-urpf.md) -- rpf-check enum over raw sysctl integer; three-state nil/disable/value pattern; sysctl profile ordering issue
- [882](plan/learned/882-vlan-qos-map.md) -- VLAN 802.1p QoS maps: VLANSpec struct (TunnelSpec precedent), nil-means-unconfigured, duplicate-canonical-key rejection, defense-in-depth validation at 3 layers
- [883](plan/learned/883-vlan-qos-lab.md) -- VLAN QoS wire-level lab: AF_PACKET TCI decode, single-netns veth with static ARP, nftables meta priority counters for ingress, negative controls as first-class tests
- [725](plan/learned/725-spec-cpe-3-dhcp-ranges.md) -- YANG container-to-list migration, composite pool with per-segment bitmaps, format detection for backward compat
- [743](plan/learned/743-config-schema-stamp.md) -- Schema stamp as comment line (not YANG leaf), emitted at persistence site only, prep for downgrade recovery
- [746](plan/learned/746-cpe-4-firewall-global-options.md) -- Firewall global-options: keyword-to-sysctl mapping via EventBus default layer; inverted semantics for ignore-type sysctls
- [915](plan/learned/915-firewall-irr-iface.md) -- Per-interface IRR source validation: separate ze_irr_iface table, prerouting hook, policy-accept with per-interface drop terms
- [758](plan/learned/758-config-graph.md) -- Config dependency graph for agent impact analysis: derived from validation code paths, 7 edge kinds, plugin registry integration
- [759](plan/learned/759-archive-pruning.md) -- Archive commit-revisions pruning: stable prefix from dual-timestamp diff, mtime-oldest-first, file:// only, uint16 max-keep
- [860](plan/learned/860-yang-required-generic.md) -- Generic ze:required enforcement: anchor-scoped walker, ValidateTreeAllModules for multi-module YANG sections, bare-form migration to mandatory true

## CLI/API

Command structure, text format, IPC, RPC dispatch.

- [072](plan/learned/072-cli-run-merge.md) -- CLI run command consolidation
- [081](plan/learned/081-update-text-parser.md) -- Unified text parser for API commands
- [110](plan/learned/110-consolidate-update-commands.md) -- Update command consolidation
- [132](plan/learned/132-spec-test-cmd-consolidation.md) -- Test command consolidation
- [143](plan/learned/143-api-command-restructure-step-3.md) -- API command restructure (8 steps)
- [209](plan/learned/209-yang-ipc-dispatch.md) -- YANG-driven IPC dispatch
- [229](plan/learned/229-command-context-server-refactor.md) -- CommandContext server refactor
- [245](plan/learned/245-rib-command-unification.md) -- RIB command unification
- [727](plan/learned/727-diag-core.md) -- 9 built-in diagnostic commands (procfs package, build-split patterns, singleflight without x/sync, BFD capture provider interface)
- [728](plan/learned/728-diag-netlink-monitor.md) -- Netlink monitor streaming (unified output channel, YANG verb tree placement, register_*.go hook bypass)
- [729](plan/learned/729-diag-traceroute.md) -- ICMP traceroute (ttlSetter interface for IPv4/IPv6 TTL, pure Go over library, argTimeout goconst pattern)
- [901](plan/learned/901-env-autocomplete.md) -- Env autocomplete (local handlers don't create tree nodes, ensureEnvPath with desc guard, registry-derived shell roots, core envcatalog)
- [738](plan/learned/738-cli-grammar.md) -- CLI grammar: closed keywords before free-form values (YANG sub-containers consume dispatch tokens, compatibility only after release)
- [730](plan/learned/730-diag-capture-interface.md) -- AF_PACKET live capture (mdlayher/packet + go-pcap BPF, portable/linux file split, Ethernet link type for raw sockets)
- [755](plan/learned/755-ze-doctor.md) -- Offline system readiness checks: diagnostic code taxonomy, error/warning severity, platform-split checks, shared resolve package
- [788](plan/learned/788-doctor-improvements.md) -- Doctor schema-driven listener inventory, RegisterListenerDefault pattern, show doctor provider, dependency inventory guardrail
- [837](plan/learned/837-doctor-check-registry.md) -- Doctor check registry: explicit phase/order/component metadata, plugin binary check first migration, registry code metadata consistency through diagnostic.Lookup
- [838](plan/learned/838-doctor-check-ownership.md) -- Doctor check ownership: runtime dependency check registration, check function, and unit test live in the owning plugin/component/backend/command package; `internal/component/doctor` owns only runner coverage and checks with no narrower owner
- [863](plan/learned/863-plugin-doctor-checks.md) -- Plugin doctor check registration: external plugins declare doctor checks in Stage 1, invoked via callback at runtime; two parallel paths (Go registry for offline, plugin callback for runtime)
- [868](plan/learned/868-test-web-parallel.md) -- Parallel web tests: per-test ze daemon + agent-browser session isolation; web suite migrated from bespoke loop to ParallelRunner (two test engines, not three)
- [872](plan/learned/872-structural-review-fixes.md) -- Structural review fix pass: generators/checkers over hand-maintained text (arch map, link checker, drift check); netip.Addr peer keys; ownership moves; verify-gate soundness (mode/skips, reverse deps, fail-not-skip); six parallel subagents on disjoint files
- [791](plan/learned/791-spec-cli-default-format.md) -- Configurable default CLI output format: env.Get in command package, session override via env.Set, intercept placement before isConfigCommand
- [795](plan/learned/795-cmd-typed-args.md) -- YANG-typed command arguments: ArgDefs on Node from YANG leaves, two-phase dispatcher validation, completer auto-generates enum suggestions, mergeYANGEntry second pass for leaf children
- [792](plan/learned/792-platform-detection.md) -- Runtime platform detection: gokrazy/systemd/container/plain-linux/darwin classification, cgroups v1+v2, FD limits, set system file-descriptors
- [794](plan/learned/794-cli-session-transcript.md) -- CLI session transcript: executor wrapping for command+output recording, YANG enumeration, best-effort file writes, cmd/ vs internal/ hook constraints
- [796](plan/learned/796-doctor-platform-coherence.md) -- Platform-aware doctor checks: thread PlatformInfo as local, severity tiers by platform, env overrides for deterministic functional tests, naming convention (thing checked, not analysis type)
- [803](plan/learned/803-gnmi.md) -- gNMI component: segment-based paths, ChangeNotifier, time.After(0) trap
- [804](plan/learned/804-gnmi-yang.md) -- gNMI YANG config, show command, Prometheus counters, external commit notify
- [809](plan/learned/809-pol-3-validation.md) -- Policy plain names: unique filter names as default operator form, show policy chain output shape change (name+canonical), prefixed forms as escape hatch
- [810](plan/learned/810-show-command-pipe-filters.md) -- Command-owned pipe filters: PipeFilter registration, longest-prefix lookup, FoldFilters rewrite, show verb-first grammar, generic pipe code BGP-free
- [822](plan/learned/822-pipe-first-last.md) -- Pipe first/last: dual-path generic+server-side pattern, FoldFilters 3-tuple return with metadata map, pipe metadata dict in JSON output, table/text renderers skip pipe key
- [826](plan/learned/826-ipc-dispatch-data-raw.md) -- DispatchCommandOutput raw JSON: `Data` is `json.RawMessage`, errors move to `Error`, callers must avoid byte-slice substring assertions
- [827](plan/learned/827-dispatch-response-passthrough.md) -- Execute-command response passthrough: SDK handlers return Go values, SDK marshals once, pipeline JSON strings must be wrapped as `json.RawMessage`
- [829](plan/learned/829-command-verb-first.md) -- Verb-first commands: small root verb set, deprecated aliases only for released names, longest-prefix dispatch needs deprecated-prefix lookup
- [814](plan/learned/814-pol-4-explain.md) -- Policy dry-run: show policy test command, TracePolicyFilterChain trace helper, narrow PolicyDryRunner interface, per-filter decision trace, wire diff output
- [849](plan/learned/849-command-surface-ownership.md) -- Command surface ownership: container-merge YANG, owner/cmd handler pattern, dedicated feature modules (ping/traceroute), exported doctor check registry, generic-stay allowlist, substring-marker YANG gotcha
- [850](plan/learned/850-appliance-command-plugin.md) -- Appliance command plugin: reversed 815, moved appliance from cmd/ze/install/appliance/ to internal/appliance/ as self-contained command provider, ze appliance replaces ze install appliance, clean break (supersedes 815)

## Web Interface

Web UI, HTMX, templates, looking glass, chaos dashboard.

- [266](plan/learned/266-chaos-web-foundation.md) -- Chaos web foundation, SSE debounce, OOB swaps
- [268](plan/learned/268-chaos-web-route-matrix.md) -- Route matrix visualization pattern
- [741](plan/learned/741-graceful-listener-migration.md) -- Graceful listener migration on config reload (bind-before-close, cross-service conflict detection)
- [756](plan/learned/756-web-auto-reload.md) -- Web UI auto-reload on commit: late-bound commit hook on EditorManager, moved reloadAfterCommit outside apiCfgOK guard

## RIB/Routing

Route storage, selection, forwarding, communities, path selection.

- [010](plan/learned/010-rib-config-design.md) -- RIB config design, storage model
- [173](plan/learned/173-plugin-rib-pool-storage.md) -- RIB pool storage design
- [275](plan/learned/275-spec-forward-pool.md) -- Forward pool, per-peer worker goroutines
- [717](plan/learned/717-rib-2-multicast.md) -- Multicast RPF via generic LPM on sharded Loc-RIB; query-all-shards pattern; wiring gap caught by review
- [772](plan/learned/772-nh-cascade-wiring.md) -- NH cascade wiring: async OnChange to avoid shard deadlock, LPM race fix, ECMP-aware cascade
- [774](plan/learned/774-fib-depth-2-ecmp.md) -- ECMP is two mechanisms: bgp-rib multipath (within BGP) vs sysrib ecmpCollect (cross-protocol); locrib path means sysrib only sees one "bgp" entry
- [776](plan/learned/776-srv6-prefix-sid.md) -- SRv6 via lazy OtherAttrs extraction; SID resolvability reuses NH resolver; transposition at emission time; Reserved-byte trap in RFC 9252 wire format
- [906](plan/learned/906-srv6-review-fixes.md) -- SRv6 RFC compliance: encoder/extractor must share constants; transposition errata 7652 (high-order bits); short-circuit must cover all emitted fields; store derived state only after all gates pass
- [783](plan/learned/783-rib-peer-lock-split.md) -- RIB peerMu lock split: narrow r.mu to peer-keyed maps only, push RLock into helpers, three-phase UPDATE handler for concurrent peer processing
- [784](plan/learned/784-rib-rs-fastpath.md) -- locrib ForwardHandle: interface-based zero-copy wire bytes on Change for state-tracker consumers; two-trigger model (receive-path for forwarders, OnChange for state trackers) stays
- [789](plan/learned/789-adjribout-compact-storage.md) -- ribOut compact storage: 16 B entry + pool handle replaces 385 B *Route per peer; engine OutgoingRIB is dead code (zero production callers); refcounted source tracking
- [823](plan/learned/823-rib-show-bounded-dump.md) -- show bgp rib lock-scope reduction: sources snapshot peerMu-protected refs then iterate via PeerRIB's own lock; lazy outboundSource buffers per-peer; bestPipeline releases peerMu before terminal drain
- [824](plan/learned/824-rib-feed-replay-batch.md) -- Replay batching: stateful cursor protocol with delta encoding reduces reconnect replay from O(routes) to O(attr-sets) UpdateRoute calls; RegisterProcessCleanup hook pattern for cross-package cleanup without import cycles

## Protocol/RFC

Graceful restart, route refresh, capability negotiation, session management.

- [007](plan/learned/007-family-negotiation.md) -- Four family modes (enable/disable/require/ignore)
- [033](plan/learned/033-spec-eor-tracking.md) -- End-of-RIB handling (RFC 4724)
- [254](plan/learned/254-rfc7606-enforcement.md) -- RFC 7606 treat-as-withdraw enforcement
- [128](plan/learned/128-graceful-restart-plugin.md) -- Graceful restart plugin design
- [574](plan/learned/574-bgp-4-bmp.md) -- BMP receiver + sender (RFC 7854), config-as-strings, synthetic OPENs
- [647](plan/learned/647-bmp-5-sender-compliance.md) -- BMP sender compliance: real OPENs, Route Mirroring, ribout dedup
- [879](plan/learned/879-l2tp-priority.md) -- L2TP control message priority: Ns-at-send-time invariant enables queue reordering; kernel P-bit limitation
- [919](plan/learned/919-mpls-kernel.md) -- Kernel MPLS FIB: labeled BestChange routed through rich-route RTA_ENCAP; push uses RouteAdd (no-clobber) vs RouteReplace for relabel; AF_MPLS swap/pop via separate mpls-fib topic
- [920](plan/learned/920-mpls-ldp.md) -- LDP (RFC 5036): dynamic interface reload via discoveryManager reconcile (stop Hellos → adjacency ages out); **plugin config delivered root-wrapped + string-numbers + keyed-map lists — a parser reading the wrong shape leaves the engine idle and unit tests that bypass the parser miss it**; show-proxy must use PluginCommand+ForwardToPlugin (re-Dispatch recurses to stack overflow)
- [921](plan/learned/921-mpls-rsvp-te.md) -- RSVP-TE (RFC 3209/2205): RESV soft-state refresh at egress/transit; link-failure→PathErr sourced from iface EventDown (no IGP); RRO prepend per §4.4; same config-shape + show-proxy traps as LDP; **AC-12 unsatisfiable — FRR has no rsvpd daemon**
- [925](plan/learned/925-mpls-rsvp-te-fast-reroute.md) -- RSVP-TE Fast Reroute (RFC 4090) facility backup: local repair is a single in-worker FIB reprogram in handleLinkDown (push bypass label over protected label); **AF_MPLS swaps must use RouteReplace not RouteAdd — local repair re-programs an existing in-label, RouteAdd EEXISTs on a live kernel and the fake FIB + isolated-program QEMU test both hide it**; bypass paths configured explicitly (no CSPF), keyed in the same lspTable via a reserved tunnel-id base; **node protection needs RRO label recording — the PLR pushes the NNHOP's recorded label, not the NHOP's**; one-to-one detour split to mpls-9
- [923](plan/learned/923-isis-8-dis-broadcast.md) -- IS-IS DIS election + pseudo-node (ISO/IEC 10589 §8.4.5): pure per-level election (priority desc, MAC desc) + damping in circuit/dis.go; pseudo-node LSP reuses the isis-6 Originator (non-zero pseudonode SourceID, members at metric 0); own-LSP star encoding in levelState; **isis-5 reconcile does NOT re-advertise a live circuit's priority** (engine AC-5 via DIS-loss/join, not reconcile); **an abruptly-departed DIS's pseudo-node ages out over MaxLifetime — purge-before-yielding only applies to a node losing the role while present**
- [957](plan/learned/957-ospf-3-ip-transport.md) -- OSPFv2 raw IPv4 transport: per-interface RX/TX socket split, iface resolver before socket bind, `IP_TTL` plus `IP_MULTICAST_TTL` both set to 1, IP multicast membership via address-bound `IPMreq`, and QEMU peer-netns veth tests for real multicast behavior
- [958](plan/learned/958-ospf-4-component-config.md) -- OSPFv2 config backbone: plugin/YANG/runtime stay self-contained under `internal/plugins/ospf`; `ze config validate` needs a static section allow-list; OSPF depends on `interface` for router-id derivation; dispatcher checks receiving-interface area; ranges are IPv4-only in YANG and Go; link-up/down callbacks keep engine state aligned with raw sockets
- [959](plan/learned/959-ospf-5-interface-ism.md) -- OSPFv2 interface ISM follows the IS-IS per-interface runtime pattern but keeps ISM and NSM separate; Hello DR/BDR fields are interface addresses, not Router IDs; passive and loopback records exist without raw sockets; BackupSeen requires 2-Way; inactivity timers must schedule exact neighbour deadlines
- [960](plan/learned/960-ospf-6-neighbor-nsm.md) -- OSPFv2 Neighbor State Machine: ISM drives Hello-derived events, NSM owns DD/LSReq synchronization only; nil LSDB means no-request until spec-ospf-7 wires `SetLSDB`; DD/LSReq/LSUpdate chunk by OSPF payload budget (`InterfaceMTU - IPv4HeaderLen`), not full MTU; packet-handler tests must dispatch encoded packets through the engine
- [961](plan/learned/961-ospf-7-lsdb-flooding.md) -- OSPFv2 LSDB/flooding: IS-IS-style full regeneration for Router/Network-LSAs; `lsdb` owns retransmit lists and purge retention; neighbor Loading only drains requests after LSDB acceptance; Type 5 cleanup is AS-wide across normal areas; MinLSInterval needs timer retry plus unchanged-body skip; MaxSequenceNumber restart happens when the acknowledged MaxAge purge resets own sequence
- [962](plan/learned/962-ospf-8-spf-rib.md) -- OSPFv2 SPF/RIB: IS-IS-shaped graph/SPF/route/install/computer split without shared protocol code; FIB install is one Loc-RIB Path per equal-cost next-hop, never redistevents; Loc-RIB must emit ECMP membership-only `ChangeUpdate` without a misleading ForwardHandle and sysrib startup replay must carry `PathGroup.ECMPNextHops`; carrier-down keeps config in `running`, keeps the down active interface in topology for area membership, and suppresses its links during origination so stale Network-LSAs flush; `.ci` regexes are unquoted and tmpfs commands do not run from the repo root
- [972](plan/learned/972-ospf-af-unify.md) -- OSPFv3 unified engine follow-up: one `ospf` engine with Transport, Codec, and AFPrefixStrategy seams; `ospfv3/{types,packet,transport}` stay leaf modules consumed by the engine; OSPFv3 next-hop source is neighbor link-local per interface, not Router-LSA link data; Link-LSAs are interface-scoped and must participate in DD/LSReq exchange or Loading never drains; scope-typed LS Types need helper classification; real v6 redist interop requires BGP source init registration plus generic `redistevents` best-path production.
- [974](plan/learned/974-ospfv3-4-link-lsa.md) -- OSPFv3 Link-LSA + DR Intra-Area-Prefix aggregation (spec-ospfv3-4): a new link-local LSDB flooding scope (per-interface store reusing `areaDB`) alongside area/AS; the DR aggregates attached routers' Link-LSA prefixes (excluding NU/LA option bits and link-local addresses) into a Network-referenced Intra-Area-Prefix-LSA; link-scope LSAs MUST flow through DD summaries + LSReq lookup or Loading never drains; v6-only (OSPFv2 never touches the link store)
- [975](plan/learned/975-ospfv3-5-nssa-redist.md) -- OSPFv3 NSSA redistribution (spec-ospfv3-5, RFC 3101): a v6 ASBR inside an NSSA injects Type-7 (0x2007), the elected ABR translates to Type-5; reuses the v4 NSSA policy and `ExternalLSA.WriteTo` (the NSSA-LSA body is byte-identical, differing only in LS Type/scope); P-bit lives in PrefixOptions not header Options; Part B bridges BGP best-path changes into the generic redistribution producer (`bgpredist.EmitBestChange`) so `import bgp` installs real routes; the redist framework still couples to a configured BGP reactor (follow-up)
- [971](plan/learned/971-ospf-14-must-remediation.md) -- OSPFv2 MUST-compliance remediation (spec-ospf-14, audit-then-fix over ospf-1..13): 18 RFC-MUST gaps fixed across auth (RFC 2328 App D/5709/7474), config validation (App C.3), flooding (§13/Table 19), NSSA (RFC 3101). KEY DECISIONS: AC-14 auto-default scoped to **totally-stubby (no-summary) NSSAs only** (`isABR && (NoSummary||DefaultOriginate)`), regular NSSAs stay operator-gated (FRR/Cisco parity); AC-18 boot count = ZeFS blob `KeyOSPFAuthBootCount` read+incremented once in `newEngine`, fallback = **SHA-1(time.UnixNano) truncated to 32 bits** (plain seconds collide on fast restart); P-bit clamp + `HigherRIDType5Exists` suppression enforced at the **LSDB origination boundary** (no caller bypass); AC-1 trailer length is **exact `!= len(wire)`** not `>=`; AC-5 recvSeq reset wired through `nsmAdapter` on NeighborDown/InterfaceDown. GOTCHAS: injected new-diagnostics/lint LAG+go STALE -- trust `go vet`/`go test`/grep; two flooding tests encoded the OLD buggy behavior and had to be updated to RFC-correct (correctness sync, not weakening); `floodExcept` signature change rippled to 6 callers; AC-3/6/7 wired to `ze config validate` via `verifyOSPFConfigSections` (InProcessConfigVerifier) so the `.ci` proves the user surface (cost/transmit-delay 0 caught by YANG range, key-id 256 by `ErrKeyIDTooWide`); **AC-14 end-to-end is Linux-CI/QEMU-only** (ABR needs 2 interfaces in 2 areas; loopback-only daemon harness can't) -- engine unit test proves it, interop variant left as open item. OSPFv3 isolation held (v2/v3 share no code, `ospfv3/types` import-guard). Open item: AC-14 no-summary auto-default interop in `ospf-stub-nssa-frr` (currently only the explicit `default-originate` path).
- [970](plan/learned/970-ospfv3-3-ipv6-transport.md) -- OSPFv3 raw IPv6 transport (spec-ospfv3-3): `internal/plugins/ospfv3/transport/` mirrors the OSPFv2 transport orchestrator (Backend/InterfaceHandle/Transport, iface EventBus lifecycle, rescan) but swaps the IPv4 raw socket for one `golang.org/x/net/ipv6.PacketConn` per interface (PPP-RA pattern). KEY DIVERGENCES: IPv6 raw sockets deliver the upper-layer payload with NO IP-header strip -- `src` from `ReadFrom`, `dst`/`ifindex`/`hopLimit` from the ancillary ControlMessage (`SetControlMessage(FlagDst|FlagInterface|FlagHopLimit)`); the address-bound IPv6 checksum is FINALIZED ON TX by the transport (only it knows the egress source) via `packet.FinalizePacketChecksum`, binding the same link-local as `ControlMessage.Src` so the on-wire source == the pseudo-header source (kills the "passes on veth, fails at a real peer" class). Instance ID demux (RFC 5340 §4.2.1) lives in the TRANSPORT rxLoop (umbrella + named test) reading byte 14 via `packet.PeekInstanceID`, per-interface Instance ID from `EnableInterface(name, instanceID)`. INTEROP LESSON (design review vs FRR source): FRR ospf6d DROPS received multicast OSPFv3 with `hoplim != 1` (RFC 5340 App A.1) -- hop limit 1 is MANDATORY, GTSM-255 is BFD's not OSPF's. `SetChecksum(true,12)` kernel offload rejected (can't zero the checksum for RFC 7166 signed packets). Link-local source lags link-up (IPv6 DAD) -> `ErrNoLinkLocal` + rescan/addr-added retry. QEMU veth round-trip asserts PEER `VerifyPacketChecksum`. Added `packet.FinalizePacketChecksum`+`packet.PeekInstanceID` to the ospfv3-2 codec. Next: ospfv3-4-plugin-config.
- [969](plan/learned/969-ospfv3-2-wire.md) -- OSPFv3 packet+LSA codec (spec-ospfv3-2): `internal/plugins/ospfv3/packet/` is a SEPARATE codec mirroring the OSPFv2 buffer-first conventions (`WriteTo(buf,off) int`, lazy `RawBytes` re-flood, opaque unknown-type passthrough, length-validated decode) but sharing no code; consumes `ospfv3/types`. KEY DIVERGENCE: the OSPFv3 packet checksum is the IPv6 upper-layer checksum over a pseudo-header (src16+dst16+len32+zero24+nextHeader=89), so `PacketChecksum(src,dst,pkt)`/`VerifyPacketChecksum` TAKE the IPv6 addresses (transport supplies them); `Packet.WriteTo` leaves the checksum field zero and transport finalizes (also covers the RFC 7166 auth-trailer case). The LSA Fletcher checksum is byte-identical to OSPFv2 (`lsa[2:length]`, non-zero), re-owned here. BLOCKER LESSON (review): a round-trip test of a wire-WRONG-but-self-consistent encoding passes green -- the first AS-External impl put the E/F/T flags at body offset 6 + an 8-bit Referenced LS Type, but RFC 5340 §A.4.7 puts flags in byte 0 (sharing the Metric word) + a 16-bit Referenced LS Type @6; the spec table AND research digest had the same error so nothing caught it until an independent review vs the RFC. FIX = flags@0, `types.LSType` 16-bit reftype@6, + a hardcoded GOLDEN VECTOR. **Every wire codec needs >=1 golden/hardcoded-bytes vector per structure, not only round-trips (round-trips prove encode==decode, not encode==RFC).** Fletcher-255 treats 0x00==0xff (both 0 mod 255) so a tamper test must avoid those byte values. Next: ospfv3-3-ipv6-transport (raw IPv6 proto-89, QEMU).
- [968](plan/learned/968-ospfv3-1-types.md) -- OSPFv3 leaf types (spec-ospfv3-1, FIRST OSPFv3 child): `internal/plugins/ospfv3/types/` is a SEPARATE copy of the OSPFv2 leaf conventions, NOT a shared package (umbrella guide §15: do NOT unify v2/v3; FRR ships two daemons) -- a `go/parser` import-guard test enforces no ospf/ospfv3-sibling/component/rib imports; OSPFv3 LS Type is 16-bit with embedded flooding scope (U-bit 0x8000, S2/S1 scope 0x6000 shift 13, 13-bit function 0x1fff), so `LSType.Scope()/FunctionCode()/UBit()` decode it and the LSDB never re-derives scope; `LSAKey=(LSType,LinkStateID,AdvertisingRouter)` has NO separate scope field (the type carries it); width fidelity for FRR interop = 24-bit Options + 24-bit Metric + IPv6 prefix byte-len `((PrefixLength+31)/32)*4` with zero-padding validation; GOTCHA `uint32(<negative typed const>)` overflows at COMPILE time (use int32+%d); AreaID parses dotted-quad OR plain int. Next: ospfv3-2-wire (packet+LSA codec)
- [967](plan/learned/967-ospf-13-cli-diag-interop.md) -- OSPFv2 CLI/diag/web/interop (spec-ospf-13): presentation layer over the engine, NO new protocol logic; CLI the IS-IS/LDP way (`ze-show:ospf-*`/`ze-clear:ospf-*` RPC proxies in cmd_show.go + ze-ospf-cmd.yang command tree, proxy `PluginCommand` MUST match engine `CommandDecl` exactly); ONE generic web view (`snapshot_views.go` snapshotHandlers + `sse_snapshot.go` loop + `page_snapshot.go` escaped shell) with IS-IS refactored onto it + OSPF a parallel thin adapter (`//nolint:dupl` on the PACKAGE line -- dupl attributes two-file clones there, function-level nolint does not suppress); doctor reuses `parseOSPFConfig` via `tree.ToMap()`->`{"ospf":{...}}` (same shape as `ExtractConfigSubtree`) so verdict can't diverge from runtime; `ze_ospf_*` namespace asserted via a recording `metrics.Registry`; shared `show > ip` YANG container description must be copied VERBATIM or the command-tree merge warns; config-test JSON needs nested list key + explicit key leaf (`areas:{area:{"0.0.0.0":{area-id:...}}}`); on-startup/on-shutdown max-metric parsed-but-unarmed engine-wide (summary `stub-router.active`=always-only, documented); 6 FRR `ospfd` interop scenarios + `FRROSPF` harness auto-discovered (`ospfd=yes`), Linux-CI-gated like the IS-IS siblings; review gate 0/0 (3 NOTES)
- [966](plan/learned/966-ospf-12-auth.md) -- OSPFv2 authentication (RFC 2328 App D / 5709 / 7474): the ospf-2 codec already excludes the 8-byte auth field from the checksum and zeroes it for AuType 2/3, so auth is just the crypto backend on top; sign at ONE transport chokepoint (`transport.SetSigner` in `SendPacket`) that rewrites the AuType byte + fixes the checksum (recompute for AuType 1 since the AuType byte is in the checksum region, zero for crypto) + appends the digest, instead of threading auth into every encoder; verify at ONE dispatcher chokepoint (`authOK`) after checksum/area, before any handler; Keyed-MD5 is `MD5(packet||key16)` NOT HMAC, while HMAC-SHA uses `hmac.New(H, Ko)` with Ko derived to L octets (Go zero-pads Ko to block B, matching RFC 5709's Ko-XOR-Ipad); a per-chain `extended-sequence` bool selects AuType 3 (64-bit seq trailer + 0x0001 protocol-id key suffix); `$9$` secrets via `secret.Decode` with plaintext fallback; per-(iface,neighbor,key-id) non-decreasing sequence replay; boot-count NVRAM persistence is a documented limitation (intra-session replay still enforced)
- [965](plan/learned/965-ospf-11-stub-nssa.md) -- OSPFv2 stub/NSSA (RFC 3101): extend existing scaffolding (areaTypes, flood filter, OptionNP, Type7=LSTypeNSSA sharing the Type 5 body codec) rather than duplicate; stub Type 3 default injection lives in the SPF summary originator (`applyAreaTypePolicy`, threaded via `SummaryInput.Policies`), not the LSDB; the P-bit rule sets Type 7 P=1 only when the router cannot inject Type 5 directly (`externalScope.canType5` = normal/backbone attachment OR no NSSA attachment) and FA!=0; translator election is local (highest Router ID among NSSA Router-LSA B-bit ABRs) with a stability grace (`translatorEffective` keeps translating for stability-interval after losing); §2.5 preference is a new primary key on the external candidate (Type7-P1 < Type5 < Type7-P0) ahead of trap #7; NSSA reconciliation runs from BOTH reconcile and the 1s tick so it needs its own `nssaMu`; a Type 7 P-bit toggle on an unchanged body must still re-originate (body-compare alone misses it); the translated Type 5 shares the AS-wide key with self-redistributed Type 5 (documented collision); split `dispatcher` out of `instance.go` at the 1000-line cap
- [964](plan/learned/964-ospf-10-as-external-asbr.md) -- OSPFv2 AS-External/ASBR: two never-merged paths (FIB install vs `redistevents`); path type is the primary external key (trap #7 E1>E2); the redistribution source AND consumer share the name `ospf` so self-import auto-rejects; `default-information originate` lives on the engine (reads Loc-RIB via the in-process `locrib.Default()` singleton), `always` unconditional vs conditional-on-a-non-OSPF-RIB-default re-evaluated live by a Loc-RIB `OnChange` watcher (handler runs under the shard write lock -- never re-enter the RIB, defer to a worker); the watcher's second goroutine exposed a latent LSDB race (`installOriginated` mutated the `*Entry` after `install` released `d.mu`) fixed by an `installLocked` that holds the lock across install + Header + markPurged; `hasNonOSPFDefault` excludes OSPF's own source to avoid a self-sustaining loop; a `defaultInfoOriginated` flag stops a withdraw purging a redistribute-owned `0.0.0.0/0`
- [963](plan/learned/963-ospf-9-inter-area-abr.md) -- OSPFv2 inter-area/ABR: inter-area is a producer/consumer pair over the ABR's own §16.1 route table (intra costs are both the Type 3/4 metric and the cost-to-ABR), one `locrib.Path` per prefix, trap #8 backbone-only acceptance is the load-bearing loop-freedom rule; review gate caught a latent spec-8 gap where transit/broadcast-LAN subnets (Network-LSA prefixes, not stub links) were installed/summarized nowhere -- fix emits `VertexNetwork` routes (§16.1(4)), skipping directly-connected LANs on install but including them for summary; LS-ID collision uses increment-until-free not RFC Appendix E (collision-free but FRR wire LS-IDs may differ, ospf-13 interop); verify agent numeric findings against the constant (`LSInfinity = 0x00ff_ffff`, not 65535)

## Observability

Metrics, telemetry, Prometheus exporters, third-party format compatibility.

- [653](plan/learned/653-netdata-os-collectors.md) -- Netdata-compatible OS collector framework, 138 metrics, counter-wrap protection, per-collector config via YANG, verify names against source not summaries
- [736](plan/learned/736-iface-rate.md) -- Interface rate tracker: raw backend stats (not baseline-adjusted), 12 GaugeVec, stale label cleanup, ticker+stop-channel lifecycle
- [808](plan/learned/808-smart-management.md) -- YANG-modeled SMART disk health: core/smart ioctl library (host wraps, no smartctl), storage.Manager ticker+stopCh, three-tier temperature alerting via report bus, self-test scheduling, `show storage smart` RPC via atomic pointer; hub-wired (not a plugin)

## Security/Auth

Authentication, authorization, appliance bootstrap, and credential boundaries.

- [831](plan/learned/831-appliance-auth-hardening.md) -- Appliance auth hardening: local admin uses `meta/auth/local/*`, config-file users share web/API/SSH auth loading, mutation paths enforce RBAC across web CLI, REST, and gRPC

## Agent Workflow

Agent rules, self-improvement, discovery paths, and development-time inventories.

- [832](plan/learned/832-self-improving-discovery.md) -- Self-improving discovery: feature/tool/check/test changes must update docs, rules, indexes, and verification paths; `ze-verify-wiring-docs` backs changed-file discovery gates
- [833](plan/learned/833-commit-helper-tooling.md) -- Commit helper tooling: `scripts/dev/commit_helper.py` owns session reuse, message files, executable user-run scripts, ignored-path rejection, `git commit -F`, and learned-summary gating for workflow/tooling/rule changes
- [835](plan/learned/835-commit-request-fast-path.md) -- Commit requests are a fast path: create the user-run helper script, skip late completeness reviews, and treat `verify-status.sh check` FRESH as a hard no-rerun signal for `make ze-verify`
- [876](plan/learned/876-commit-helper-session-collision.md) -- Per-session tmp/ artifacts must embed a session-unique path component: the shared commit-session-id let a concurrent session's `--replace` overwrite another's prepared commit script; helper now keys the ID file by Claude session fingerprint

## Testing

Test patterns, infrastructure, chaos testing.

- [274](plan/learned/274-spec-test-diagnostics.md) -- Test diagnostic improvements
- [258](plan/learned/258-bgp-chaos-families.md) -- Chaos family fuzzing
- [265](plan/learned/265-bgp-chaos-selftest.md) -- Chaos self-test patterns
- [608](plan/learned/608-concurrent-test-patterns.md) -- Concurrent-test flake patterns (locked-write/unlocked-read, subscribe-before-broadcast, gate-handler, barrier FIFO, cleanup-drains-work)
- [881](plan/learned/881-test-flake-under-load.md) -- Host-load test flakiness: wall-clock timeouts under CPU starvation, near-timeout classification, contended-run labeling, timing baseline pollution prevention
- [723](plan/learned/723-chaos-actions-v2.md) -- Parameterized chaos actions: string-map params over typed unions, opt-in scheduling, per-instance weights
- [787](plan/learned/787-chaos-inprocess-scheduling.md) -- In-process chaos: feed vc.Now() to existing schedulers, reconnectDialer factory for stochastic reconnection, tick-channel pattern
- [797](plan/learned/797-interop-gap-coverage.md) -- Interop gap coverage: 5 scenarios (RR, policy, RPKI, BMP, max-prefix) with FRR/BIRD/GoBGP peers, concurrent Docker subnet retry, parse coverage for IXP/large-scale/RPKI/redistribution
- [800](plan/learned/800-bgp-chaos-integration.md) -- Chaos integration tests: fork mode default, run() testability, port auto-allocation
- [802](plan/learned/802-chaos-multi-target.md) -- Multi-target chaos: FRR/BIRD config gen, temp-file fork, single-port dialing, BIRD channel mapping limits
- [842](plan/learned/842-scoped-verify-committed-gap.md) -- Scoped verify committed gap: ze-verify-changed tested only the working-tree diff, so a regression committed before verifying was skipped on the clean tree; changed set now adds packages committed since the last green verify (`scripts/dev/changed-pkgs.sh`, baseline from `tmp/ze-verify.status`)
- [843](plan/learned/843-verify-debugging-protocol.md) -- Verify failure-routing protocol: tested `verify_run.go` owns ze-verify, writes compact text+JSON failure index, groups by stage boundary first (never across), native `VERIFY FAILURE GROUP: {json}` manifests over text parsing, `ZE_VERIFY_MODE=1` env-gated rendering; the house convention for machine-readable test output
- [892](plan/learned/892-spec-validate-command.md) -- Post-verify validation tool (`make ze-validate`): Python-based doc/spec hygiene checks (stale anchors, line-number anchors, unwired exports, spec AC completeness); grep-based cross-package search over Python rglob for speed
- [908](plan/learned/908-test-trace-mode.md) -- Per-step trace output: leaf `internal/test/trace` package, dual human+machine format (`VERIFY STEP: {json}`), two-tier gate (failure default, `-v` all), incremental `.ci` recording over full collect-all refactor

## Build/Deployment

Build system, Docker, CI, toolchain upgrades.

- [753](plan/learned/753-docker-go126.md) -- Docker support: two-stage build to scratch, CGO_ENABLED=0, Go 1.26 upgrade across all project references
- [754](plan/learned/754-makefile-split.md) -- Makefile split into mk/ includes: tiered help, component test groups, contributor testing docs
- [853](plan/learned/853-build-tag-split.md) -- Positive build tags (ze_distro, ze_appliance, ze_setup) replace negative ze_stripped; no-tag default is minimal; Go _linux.go suffix gotcha
- [854](plan/learned/854-install-8-appliance-iso.md) -- Appliance ISO installer: transport envelope around raw image, ze.source=iso initrd mode, gzip compression, media-id exclusion, UEFI GRUB boot, hard checksum enforcement, power-off not reboot
- [870](plan/learned/870-kernel-build-convergence.md) -- shared kernel builder: runtime and installer converge on `tools/kernel-builder/` with Linux 7.0.11, runtime fragment tracking, Docker-first builder selection, and modcache overlay/restore flow

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
- (890) Env-to-YANG promotion for BGP plugin config: create plugin yang/ module augmenting /bgp:bgp, add ConfigRoots+Features+YANG to registration, OnConfigure handler applies YANG value via SetChanSize, env var renamed to mirror YANG path per config-naming.md
- (917) Generic config route parser registration: InProcessConfigRouteParser on plugin Registration replaces hardcoded family switch cases in extractRoutesFromUpdateBlock. PluginRoute carries pre-built NLRI + attribute bytes. fullRawAttribute must strip header for WriteAttrTo; ExaBGP TunnelEncap uses draft sub-TLV types
