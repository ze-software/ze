# 541 -- Policy Framework

## Context

Ze had no configurable route filter framework. Filters existed only as in-process Go functions (loop detection, OTC, community) registered at startup and sorted by stage/priority. Users could reference external plugin filters via `redistribution { import [ rpki:validate ] }` with a `<plugin>:<filter>` colon format, but there was no way to define named filter instances with per-peer settings, no way to configure loop detection parameters (allow-own-as, cluster-id), and no way to deactivate a default filter for a specific peer. The goal was a Junos-inspired policy framework with specialized filter types, named instances, user-controlled chain order, and per-peer deactivation.

## Decisions

- Specialized filter plugins over a generic policy language (from/then terms). Each filter type does one thing with its own YANG settings. Composition via the import/export chain achieves the same configs as Junos policy-statements.
- Three config concerns separated: `policy` (filter definitions), `filter` (per-peer chains), `redistribute` (route source/dest selection). Junos mixes source selection with filtering -- ze separates them.
- Each filter type augments `bgp/policy` with a `ze:filter`-marked list, over a flat list with type discriminator. Follows the ze-role/ze-filter-community augment pattern.
- Loop-detection as a facade over in-process `LoopIngress`, over moving it to the text-format policy chain. Zero-copy preserved: settings (allow-own-as, cluster-id) flow through PeerFilterInfo, wire-bytes filter reads them.
- `inactive:` prefix on leaf-list values for deactivation, over a separate `no-import` leaf-list or `active false` leaf. `delete` on a built-in auto-populated filter sets `inactive:` prefix instead of removing. Matches Junos `inactive:` semantics.
- `redistribute` as a top-level core YANG module (`ze-redistribute.yang`), over nesting under bgp. Protocols augment the neutral root -- BGP adds ibgp/ebgp, future OSPF/ISIS add theirs. No cross-protocol YANG imports.
- Callback pattern for redistribute validator (`SetRedistributeSourceCallbacks`), over direct import from config to bgp/redistribute. Keeps config layer protocol-agnostic.
- `ze:hidden` enforced in all serializers (was declared but never checked). `ze:ephemeral` extension added for future runtime-only nodes.
- Filter name validation deferred: external plugins register at stage 1 (after config parse), so parse-time validation cannot see them. Registry and ValidateFilterNames exist and are tested but not wired.

## Consequences

- Users can configure loop detection per-peer: `bgp { policy { loop-detection my-filter { allow-own-as 2; cluster-id 10.0.0.1; } } }` with `peer X { filter { import [ my-filter ]; } }`.
- Default loop-detection auto-populates in every peer's import chain. Deactivatable via `inactive:` prefix.
- The `redistribution { import [ rpki:validate ] }` format is fully removed (no-layering). Replaced by `filter { import [ ... ] }`.
- Future filter types (prefix-filter, as-path-filter, community-tag, etc.) add a YANG augment + Go filter logic -- zero core changes.
- Future protocols (OSPF, ISIS) augment `ze-redistribute.yang` with their sources.
- `ze:hidden` now works: nodes marked hidden are excluded from all three serializers (display, annotated, blame) but still saved to file.

## Gotchas

- Renaming `redistribution` to `filter` in ze-bgp-conf.yang broke the community-filter plugin: both defined `container filter` at the same augment target. goyang silently drops duplicates (RFC 7950 violation). Fix: community plugin augments INTO the existing filter container instead of introducing its own.
- The pre-write hook uses `$PPID` for session identity, which varies across hook subprocesses. Fixed by walking `/proc` to find the Claude CLI process (argv[0] == "claude"). Linux-only; macOS falls back to `$PPID`.
- `asnCount` in LoopIngress was initially uint8 -- overflows on pathological AS_PATHs with 256+ local ASN occurrences. Changed to uint16.
- `RegisterBGPSources()` needs `sync.Once` because `RegisterValidators` is called from multiple sites (YANG schema loading, CLI completer init).

## Files

- `internal/component/config/yang/modules/ze-extensions.yang` -- ze:filter, ze:ephemeral extensions
- `internal/component/config/schema.go` -- Hidden bool on LeafNode, ContainerNode, ListNode
- `internal/component/config/serialize.go`, `serialize_annotated.go`, `serialize_blame.go` -- ze:hidden enforcement
- `internal/component/config/yang_schema.go` -- hasHiddenExtension, set during YANG parsing
- `internal/component/bgp/schema/ze-bgp-conf.yang` -- policy container, filter replaces redistribution
- `internal/component/bgp/config/filter_registry.go` -- BuildFilterRegistry, ValidateFilterNames
- `internal/component/bgp/config/redistribution.go` -- extractFilterChain (was extractRedistributionFilters)
- `internal/component/bgp/config/peers.go` -- applyLoopDetectionConfig, prependDefaultFilters
- `internal/component/bgp/reactor/filter/loop.go` -- allow-own-as count, cluster-id override
- `internal/component/bgp/reactor/filter/schema/ze-loop-detection.yang` -- loop-detection filter type
- `internal/component/bgp/reactor/filter_chain.go` -- inactive: prefix skipping
- `internal/component/bgp/reactor/peersettings.go` -- LoopAllowOwnAS, LoopClusterID
- `internal/component/plugin/registry/registry_bgp_filter.go` -- AllowOwnAS, ClusterID in PeerFilterInfo
- `internal/component/config/redistribute/schema/ze-redistribute.yang` -- core redistribute module
- `internal/component/bgp/redistribute/schema/ze-bgp-redistribute.yang` -- BGP augment (ibgp/ebgp)
- `internal/component/bgp/redistribute/registry.go` -- source registry
- `internal/component/config/validators.go` -- RedistributeSourceValidator with callbacks
- `internal/component/config/tree.go` -- RenameListEntry, CopyListEntry
- `internal/component/cli/editor_commands.go` -- RenameListEntry, resolveListTarget
- `internal/component/cli/model_commands.go` -- cmdRename, cmdCopy
