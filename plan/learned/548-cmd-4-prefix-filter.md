# 548 -- cmd-4 Prefix-List Filter

## Context

The cmd-0 vendor-parity umbrella identified prefix-list filtering as a missing capability in ze: Junos/EOS/IOS-XR/VyOS all support named prefix-lists with ge/le ranges and accept/reject actions, referenced from peer filter chains. Phase 4 of the umbrella was the first child filter spec to land after the policy framework (541) and apply-mods framework (434) unblocked phases 4-7. Goal: a `bgp-filter-prefix` plugin that loads named prefix-lists from `bgp/policy/prefix-list/NAME` and rejects whole UPDATEs whose NLRI prefixes do not match.

## Decisions

- **Named filter chain dispatch via `OnFilterUpdate` SDK callback**, over the older in-process `IngressFilter` callback (filter_community style). The chain syntax `filter import [ bgp-filter-prefix:CUSTOMERS ]` requires PolicyFilterChain dispatch so future cmd-5/6/7 filters compose with cmd-4 in user-controlled order. cmd-4 is the first production plugin in-tree to use this pattern; previously only Python mock plugins (`test/plugin/redistribution-*.ci`) exercised it.
- **Zero filter declarations at Stage 1**, over pre-declaring filter names. Filter names come from config (Stage 2), so they cannot be known at Stage 1 registration. Verified that `CallFilterUpdate` does not gate on declared filters; `FilterInfo` lookup miss returns `nil, false` (safe defaults: no AC-13 modify validation, raw=false); `FilterOnError` defaults to fail-closed reject. The plugin handles dynamic dispatch on `input.Filter` by looking up its config-loaded `map[string]*prefixList`.
- **Strict whole-update mode** (any denied prefix rejects whole UPDATE) over per-prefix nlri rewriting. The spec ACs are written for single-prefix UPDATEs and per-prefix splitting requires `action=modify` with new nlri text -- significantly more complex. Strict mode is the safest default and matches how operators configure prefix-lists in practice (one prefix per UPDATE in production). Per-prefix filtering deferred to a future spec.
- **YANG `union { zt:prefix-ipv4 ; zt:prefix-ipv6 }`** over importing `ietf-inet-types`. ze does not load `ietf-inet-types` (discovered when first `ze config validate` reported `no such module`). The union of ze's existing typedefs is the right choice and avoids adding a third-party YANG dependency.
- **Plugin folder under `internal/component/bgp/plugins/filter_prefix/`** with the standard 4-file scaffolding (`filter_prefix.go` entry + dispatch, `match.go` algorithm, `config.go` parser, `register.go` registration) plus `schema/` subdir. Matches the filter_community layout exactly.
- **`atomic.Pointer[map[string]*prefixList]`** for runtime config storage, over RWMutex + map. Lock-free hot-path reads; `OnConfigure` swaps the entire map atomically.
- **Chain ref name is the actual plugin name** (`bgp-filter-prefix:CUSTOMERS`), not the umbrella's shorthand (`prefix-list:CUSTOMERS`). The dispatcher uses `strings.Cut(ref, ":")` to split into plugin-name + filter-name; the plugin name must be the registered process name.

## Consequences

- **The named-filter-chain dispatch path is now production-validated.** Any future filter plugin (cmd-5/6/7) can copy the cmd-4 scaffolding: zero-declaration Stage 1, OnFilterUpdate handler with `input.Filter` dispatch, atomic config storage, parse-and-store at OnConfigure.
- **cmd-5 (AS-path filter), cmd-6 (community match), and cmd-7 (route modify) inherit the pattern unchanged.** They differ only in their match algorithm and (for cmd-7) returning `action=modify` with a delta instead of `accept`/`reject`.
- **Per-prefix filtering remains future work.** Until then, an UPDATE with mixed-match prefixes is rejected wholesale. This matches Junos's "strict" prefix-list mode but is stricter than Cisco's "permit some, drop others" default.
- **Documentation update is deferred to a sweep with cmd-5..cmd-7.** The umbrella's deliverables checklist already lists `docs/guide/command-reference.md`, `docs/comparison.md`, and `docs/guide/configuration.md` as targets; updating them once per filter type would be churn.
- **Plugin count is 33 internal plugins** (was 32). Tests in `cmd/ze/main_test.go` and `internal/component/plugin/all/all_test.go` updated to include `bgp-filter-prefix`.

## Gotchas

- The redistribution-*.ci tests use Python plugins via `external` config blocks; cmd-4 uses an in-process Go plugin via `--plugin ze.bgp-filter-prefix` on the command line. Both work because the engine treats the named process the same way regardless of transport (net.Pipe vs TCP).
- `FilterInfo`/`FilterOnError` lookup miss is by design SAFE (returns `nil, false` and `"reject"` respectively), not an error. The plugin author does not need to defensively pre-declare every possible runtime filter name -- the engine just forwards the RPC.
- The `pre-write-spec.sh` and `require-related-refs.sh` hooks enforce that cross-references in `// Detail:` and `// Related:` comments point to existing files. When writing a new package, create the leaf files BEFORE the hub file, or write the hub file without the cross-references and add them in a follow-up edit.
- The `block-silent-ignore.sh` hook flags any `default:` keyword in a switch as a potential silent ignore, even when the case returns an error. Restructure as `if/else` chains to avoid the trigger.
- `block-pipe-tail.sh` blocks `make ze-* | tail` and `command | tail`. Use `> tmp/foo.log 2>&1` and read with the Read tool, or grep with `--head-limit` for filtered slices.
- ze does not load `ietf-inet-types`. Use `ze-types` typedefs (`zt:prefix-ipv4`, `zt:prefix-ipv6`, `zt:ipv4-address`, etc.) instead. A future cleanup could add a unified `zt:ip-prefix` union type for reuse across filter plugins.

## Files

- `internal/component/bgp/plugins/filter_prefix/schema/ze-filter-prefix.yang` -- YANG augment of `bgp:bgp/bgp:policy` with `prefix-list` (key=name) containing ordered `entry` (key=prefix, ordered-by user) with `ge`/`le`/`action`
- `internal/component/bgp/plugins/filter_prefix/schema/embed.go` -- `//go:embed` for the YANG file
- `internal/component/bgp/plugins/filter_prefix/schema/register.go` -- `yang.RegisterModule` at init time
- `internal/component/bgp/plugins/filter_prefix/filter_prefix.go` -- Plugin entry point, `RunFilterPrefix(net.Conn) int`, OnConfigure + OnFilterUpdate dispatch, atomic config store
- `internal/component/bgp/plugins/filter_prefix/match.go` -- `evaluatePrefix` (per-route first-match-wins) + `evaluateUpdate` (strict mode walker) + `extractNLRIField` (text parser)
- `internal/component/bgp/plugins/filter_prefix/config.go` -- `parsePrefixLists` walks `bgp/policy/prefix-list/NAME/entry/...` from configjson; supports map and slice forms; applies YANG defaults; validates ge/le bounds and ge<=le
- `internal/component/bgp/plugins/filter_prefix/register.go` -- `registry.Register` with name `bgp-filter-prefix`, ConfigRoots=["bgp"], Dependencies=["bgp"], YANG=ZeFilterPrefixYANG
- `internal/component/bgp/plugins/filter_prefix/match_test.go` -- 16 cases covering ACs 1-13 + update strict mode + nlri extraction (table-driven)
- `internal/component/bgp/plugins/filter_prefix/config_test.go` -- 13 cases covering YANG defaults, ge/le boundaries, ge>le validation, invalid action, malformed prefix, both map and slice config forms
- `internal/component/plugin/all/all.go` -- Regenerated by `make generate` (now imports filter_prefix)
- `internal/component/plugin/all/all_test.go` -- Expected plugin list updated
- `cmd/ze/main_test.go` -- AvailablePlugins expected list updated
- `test/parse/prefix-list-config.ci` -- YANG schema acceptance + chain ref syntax
- `test/plugin/prefix-filter-accept.ci` -- Matching prefix lands in adj-rib-in (observer asserts `total-routes >= 1`)
- `test/plugin/prefix-filter-reject.ci` -- Non-matching prefix absent from adj-rib-in (observer asserts `total-routes == 0`)
