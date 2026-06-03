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
- [785](plan/learned/785-rfc7606-validation-cache.md) -- RFC 7606 validation cache: rejected, validation too cheap (67-138 ns) to benefit from caching despite 77-97% hit rate; cache lookup cost rivals validation cost

## Plugin System

Registration, SDK, event flow, lifecycle, hook integration.

- [253](plan/learned/253-nlri-plugin-extraction.md) -- NLRI codec extraction to plugins
- [256](plan/learned/256-plugin-lifecycle-mgmt.md) -- Plugin lifecycle management patterns
- [300](plan/learned/300-plugin-service-pattern.md) -- Plugin service pattern (SDK callbacks)
- [301](plan/learned/301-plugin-sdk-interface.md) -- SDK public interface design
- [303](plan/learned/303-plugin-api-dispatch.md) -- Plugin API dispatch via text commands
- [325](plan/learned/325-plugin-rib-families.md) -- Plugin RIB family registration
- [757](plan/learned/757-typed-route-result.md) -- Typed RouteResult replaces map[string]any in update-route, eliminating int/float64 transport divergence
- [781](plan/learned/781-remove-private-as.md) -- Remove-private-as: plugin intent + reactor wire rewrite; Set+Prepend composition
- [806](plan/learned/806-install-1-dhcp-pxe.md) -- DHCP PXE extension: additive options in existing buildReply, server-wide pxeConfig, 1500-byte reply buffer, siaddr+option 66 dual-set for PXE ROM compat
- [807](plan/learned/807-install-2-tftpserver.md) -- TFTP server plugin: RFC 1350 read-only, ephemeral port per transfer, channel semaphore for concurrency, SO_BINDTODEVICE on Linux, 32MB block-number limit acceptable for bootloaders
- [811](plan/learned/811-install-3-image-server.md) -- imageserver plugin: own HTTP listener, http.ServeFile for images/boot/zefs, path traversal via flat filename validation, zefs built at configure time with ze init key patterns
- [812](plan/learned/812-install-5-bootstrap-mode.md) -- bootstrap mode: EmitBootstrapConfig separate from EmitConfig, ethernet-only DHCP, SSH from zefs creds, falls through on failure, inserted between template and web-only in startup switch
- [813](plan/learned/813-install-6-installer-initrd.md) -- installer initrd: busybox-based shell init, wget+dd stream to first non-removable disk (sysfs removable check), blockdev --rereadpt for partition re-read, zefs injection to /perm/ze/, kernel-level DHCP via ip=dhcp cmdline
- [815](plan/learned/815-install-7a-namespace.md) -- namespace migration: cmd/ze/appliance/ moved to cmd/ze/install/appliance/, deprecated alias prints warning, no root registration at new location
- [816](plan/learned/816-install-7b-vendor-builder.md) -- vendor builder: gokBuildFn replaces gokBinary shell-out, go run -mod=vendor ./cmd/ze-gok, GOMODCACHE isolation, gokSizeArg uses strconv.AppendInt
- [817](plan/learned/817-install-7c-vendor-updater.md) -- vendor updater: local copy of gokrazy/updater (stdlib-only), authTransport for all requests, StreamTo+Switch+Reboot sequence, --testboot/--no-reboot flags
- [818](plan/learned/818-flow-export-1-counter-export.md) -- flow export counter closure: IPFIX octetTotalCount/packetTotalCount (not Delta), single MaxDatagramSize, per-datagram metric counting, template timestamp on success only, show flow-export YANG wiring
- [819](plan/learned/819-flow-export-2-flow-records.md) -- flow export spec-2 integration: FlowSample/ConntrackFlow neutral types + factory-registered flow encoders (import-graph constraint), platform-independent workers delegating to _linux netlink, BestChange typed-handle enrichment (next-hop only), deadlock-safe Stop ordering
- [820](plan/learned/820-flow-export-0-umbrella.md) -- flow export umbrella: single-collection/multiple-consumers via iface callback, registration over imports, buffer-first, in-process SDK component; spec-1 complete, spec-2 CI-gated
- [821](plan/learned/821-plugin-internal-keyword.md) -- Plugin internal keyword: explicit `plugin internal <name> { use <builtin> }` config, cross-list uniqueness, internal plugins do not use external encoder/respawn/timeout leaves
- [828](plan/learned/828-codec-callback-passthrough.md) -- NLRI decode single-marshal: DecodeNLRIHex returns any, registry marshals once; RunCLIDecode callers share function so need own marshal
- [830](plan/learned/830-typed-inter-plugin-dispatch.md) -- Typed exact-command inter-plugin dispatch: `DispatchCommandArgs` over rebuilt strings, `CommandArgsAuthorizer` over canonical fallback, command/args boundary pinned by tests

## Configuration

YANG schema, migration, config reload, editor, environment variables.

- [008](plan/learned/008-config-migration-system.md) -- Heuristic version detection, 3-version migration chain
- [065](plan/learned/065-spec-remove-version-numbers.md) -- No version numbers in config (YANG-transformable)
- [166](plan/learned/166-yang-only-schema.md) -- YANG as sole schema source of truth
- [175](plan/learned/175-config-editor-validation.md) -- Config editor validation pipeline
- [184](plan/learned/184-exabgp-to-yang-migration.md) -- ExaBGP syntax to YANG migration
- [226](plan/learned/226-config-reload-6-remove-bgpconfig.md) -- BGPConfig removal, map[string]any
- [232](plan/learned/232-editor-tree-canonical.md) -- Editor tree canonical representation
- [716](plan/learned/716-iface-2-urpf.md) -- rpf-check enum over raw sysctl integer; three-state nil/disable/value pattern; sysctl profile ordering issue
- [725](plan/learned/725-spec-cpe-3-dhcp-ranges.md) -- YANG container-to-list migration, composite pool with per-segment bitmaps, format detection for backward compat
- [743](plan/learned/743-config-schema-stamp.md) -- Schema stamp as comment line (not YANG leaf), emitted at persistence site only, prep for downgrade recovery
- [746](plan/learned/746-cpe-4-firewall-global-options.md) -- Firewall global-options: keyword-to-sysctl mapping via EventBus default layer; inverted semantics for ignore-type sysctls
- [758](plan/learned/758-config-graph.md) -- Config dependency graph for agent impact analysis: derived from validation code paths, 7 edge kinds, plugin registry integration
- [759](plan/learned/759-archive-pruning.md) -- Archive commit-revisions pruning: stable prefix from dual-timestamp diff, mtime-oldest-first, file:// only, uint16 max-keep

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
- [738](plan/learned/738-cli-grammar.md) -- CLI grammar: closed keywords before free-form values (YANG sub-containers consume dispatch tokens, compatibility only after release)
- [730](plan/learned/730-diag-capture-interface.md) -- AF_PACKET live capture (mdlayher/packet + go-pcap BPF, portable/linux file split, Ethernet link type for raw sockets)
- [755](plan/learned/755-ze-doctor.md) -- Offline system readiness checks: diagnostic code taxonomy, error/warning severity, platform-split checks, shared resolve package
- [788](plan/learned/788-doctor-improvements.md) -- Doctor schema-driven listener inventory, RegisterListenerDefault pattern, show doctor provider, dependency inventory guardrail
- [837](plan/learned/837-doctor-check-registry.md) -- Doctor check registry: explicit phase/order/component metadata, plugin binary check first migration, registry code metadata consistency through diagnostic.Lookup
- [838](plan/learned/838-doctor-check-ownership.md) -- Doctor check ownership: runtime dependency check registration, check function, and unit test live in the owning plugin/component/backend/command package; `cmd/ze/doctor` owns only runner coverage and checks with no narrower owner
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
- [316](plan/learned/316-outbound-rib-initialization.md) -- Outbound RIB initialization sequence
- [395](plan/learned/395-local-rib-architecture.md) -- Local RIB architecture, index design
- [402](plan/learned/402-bgp-route-selection.md) -- Best-path selection algorithm
- [717](plan/learned/717-rib-2-multicast.md) -- Multicast RPF via generic LPM on sharded Loc-RIB; query-all-shards pattern; wiring gap caught by review
- [772](plan/learned/772-nh-cascade-wiring.md) -- NH cascade wiring: async OnChange to avoid shard deadlock, LPM race fix, ECMP-aware cascade
- [774](plan/learned/774-fib-depth-2-ecmp.md) -- ECMP is two mechanisms: bgp-rib multipath (within BGP) vs sysrib ecmpCollect (cross-protocol); locrib path means sysrib only sees one "bgp" entry
- [776](plan/learned/776-srv6-prefix-sid.md) -- SRv6 via lazy OtherAttrs extraction; SID resolvability reuses NH resolver; transposition at emission time; Reserved-byte trap in RFC 9252 wire format
- [783](plan/learned/783-rib-peer-lock-split.md) -- RIB peerMu lock split: narrow r.mu to peer-keyed maps only, push RLock into helpers, three-phase UPDATE handler for concurrent peer processing
- [784](plan/learned/784-rib-rs-fastpath.md) -- locrib ForwardHandle: interface-based zero-copy wire bytes on Change for state-tracker consumers; two-trigger model (receive-path for forwarders, OnChange for state trackers) stays
- [789](plan/learned/789-adjribout-compact-storage.md) -- ribOut compact storage: 16 B entry + pool handle replaces 385 B *Route per peer; engine OutgoingRIB is dead code (zero production callers); refcounted source tracking
- [823](plan/learned/823-rib-show-bounded-dump.md) -- show bgp rib lock-scope reduction: sources snapshot peerMu-protected refs then iterate via PeerRIB's own lock; lazy outboundSource buffers per-peer; bestPipeline releases peerMu before terminal drain
- [824](plan/learned/824-rib-feed-replay-batch.md) -- Replay batching: stateful cursor protocol with delta encoding reduces reconnect replay from O(routes) to O(attr-sets) UpdateRoute calls; RegisterProcessCleanup hook pattern for cross-package cleanup without import cycles

## Protocol/RFC

Graceful restart, route refresh, capability negotiation, session management.

- [007](plan/learned/007-family-negotiation.md) -- Four family modes (enable/disable/require/ignore)
- [033](plan/learned/033-spec-eor-handling.md) -- End-of-RIB handling (RFC 4724)
- [254](plan/learned/254-rfc7606-enforcement.md) -- RFC 7606 treat-as-withdraw enforcement
- [369](plan/learned/369-bgp-graceful-restart-design.md) -- Graceful restart state machine design
- [375](plan/learned/375-ebgp-route-refresh-design.md) -- Route refresh design (RFC 2918/7313)
- [574](plan/learned/574-bgp-4-bmp.md) -- BMP receiver + sender (RFC 7854), config-as-strings, synthetic OPENs
- [647](plan/learned/647-bmp-5-sender-compliance.md) -- BMP sender compliance: real OPENs, Route Mirroring, ribout dedup

## Observability

Metrics, telemetry, Prometheus exporters, third-party format compatibility.

- [653](plan/learned/653-netdata-os-collectors.md) -- Netdata-compatible OS collector framework, 138 metrics, counter-wrap protection, per-collector config via YANG, verify names against source not summaries
- [736](plan/learned/736-iface-rate.md) -- Interface rate tracker: raw backend stats (not baseline-adjusted), 12 GaugeVec, stale label cleanup, ticker+stop-channel lifecycle

## Security/Auth

Authentication, authorization, appliance bootstrap, and credential boundaries.

- [831](plan/learned/831-appliance-auth-hardening.md) -- Appliance auth hardening: local admin uses `meta/auth/local/*`, config-file users share web/API/SSH auth loading, mutation paths enforce RBAC across web CLI, REST, and gRPC

## Agent Workflow

Agent rules, self-improvement, discovery paths, and development-time inventories.

- [832](plan/learned/832-self-improving-discovery.md) -- Self-improving discovery: feature/tool/check/test changes must update docs, rules, indexes, and verification paths; `ze-verify-wiring-docs` backs changed-file discovery gates
- [833](plan/learned/833-commit-helper-tooling.md) -- Commit helper tooling: `scripts/dev/commit_helper.py` owns session reuse, message files, executable user-run scripts, ignored-path rejection, `git commit -F`, and learned-summary gating for workflow/tooling/rule changes
- [835](plan/learned/835-commit-request-fast-path.md) -- Commit requests are a fast path: create the user-run helper script, skip late completeness reviews, and treat `verify-status.sh check` FRESH as a hard no-rerun signal for `make ze-verify`

## Testing

Test patterns, infrastructure, chaos testing.

- [274](plan/learned/274-spec-test-diagnostics.md) -- Test diagnostic improvements
- [258](plan/learned/258-bgp-chaos-families.md) -- Chaos family fuzzing
- [265](plan/learned/265-bgp-chaos-selftest.md) -- Chaos self-test patterns
- [608](plan/learned/608-concurrent-test-patterns.md) -- Concurrent-test flake patterns (locked-write/unlocked-read, subscribe-before-broadcast, gate-handler, barrier FIFO, cleanup-drains-work)
- [723](plan/learned/723-chaos-actions-v2.md) -- Parameterized chaos actions: string-map params over typed unions, opt-in scheduling, per-instance weights
- [787](plan/learned/787-chaos-inprocess-scheduling.md) -- In-process chaos: feed vc.Now() to existing schedulers, reconnectDialer factory for stochastic reconnection, tick-channel pattern
- [797](plan/learned/797-interop-gap-coverage.md) -- Interop gap coverage: 5 scenarios (RR, policy, RPKI, BMP, max-prefix) with FRR/BIRD/GoBGP peers, concurrent Docker subnet retry, parse coverage for IXP/large-scale/RPKI/redistribution
- [800](plan/learned/800-bgp-chaos-integration.md) -- Chaos integration tests: fork mode default, run() testability, port auto-allocation
- [802](plan/learned/802-chaos-multi-target.md) -- Multi-target chaos: FRR/BIRD config gen, temp-file fork, single-port dialing, BIRD channel mapping limits
- [842](plan/learned/842-scoped-verify-committed-gap.md) -- Scoped verify committed gap: ze-verify-changed tested only the working-tree diff, so a regression committed before verifying was skipped on the clean tree; changed set now adds packages committed since the last green verify (`scripts/dev/changed-pkgs.sh`, baseline from `tmp/ze-verify.status`)
- [843](plan/learned/843-verify-debugging-protocol.md) -- Verify failure-routing protocol: tested `verify_run.go` owns ze-verify, writes compact text+JSON failure index, groups by stage boundary first (never across), native `VERIFY FAILURE GROUP: {json}` manifests over text parsing, `ZE_VERIFY_MODE=1` env-gated rendering; the house convention for machine-readable test output

## Build/Deployment

Build system, Docker, CI, toolchain upgrades.

- [753](plan/learned/753-docker-go126.md) -- Docker support: two-stage build to scratch, CGO_ENABLED=0, Go 1.26 upgrade across all project references
- [754](plan/learned/754-makefile-split.md) -- Makefile split into mk/ includes: tiered help, component test groups, contributor testing docs

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
