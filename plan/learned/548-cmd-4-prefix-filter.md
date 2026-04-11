# 548 -- cmd-4 Prefix-List Filter

## Context

The cmd-0 vendor-parity umbrella identified prefix-list filtering as a missing capability in ze: Junos/EOS/IOS-XR/VyOS all support named prefix-lists with ge/le ranges and accept/reject actions, referenced from peer filter chains. Phase 4 of the umbrella was the first child filter spec to land after the policy framework (541) and apply-mods framework (434) unblocked phases 4-7. Goal: a `bgp-filter-prefix` plugin that loads named prefix-lists from `bgp/policy/prefix-list/NAME` and rejects whole UPDATEs whose NLRI prefixes do not match.

The initial ship (2026-04-11) passed `make ze-verify` but a post-commit `/ze-review` found that the integration was broken end-to-end: the filter was never invoked on the wire path, the `.ci` tests verified nothing, and the observer-plugin pattern used by every existing `.ci` filter test is a false-positive (the test runner ignores the observer's exit code). This summary covers both the initial implementation AND the resolve-pass that followed the review.

## Decisions

### Initial implementation

- **Named filter chain dispatch via `OnFilterUpdate` SDK callback**, over the older in-process `IngressFilter` callback (filter_community style). The chain syntax `filter import [ bgp-filter-prefix:CUSTOMERS ]` requires PolicyFilterChain dispatch so future cmd-5/6/7 filters compose with cmd-4 in user-controlled order. cmd-4 is the first production plugin in-tree to use this pattern; previously only Python mock plugins (`test/plugin/redistribution-*.ci`) exercised it.
- **Zero filter declarations at Stage 1**, over pre-declaring filter names. Filter names come from config (Stage 2), so they cannot be known at Stage 1 registration. Verified that `CallFilterUpdate` does not gate on declared filters; `FilterInfo` lookup miss returns `nil, false` (safe defaults: no AC-13 modify validation, raw=false); `FilterOnError` defaults to fail-closed reject. The plugin handles dynamic dispatch on `input.Filter` by looking up its config-loaded `map[string]*prefixList`.
- **Strict whole-update mode** (any denied prefix rejects whole UPDATE) over per-prefix nlri rewriting. Per-prefix splitting requires the `modify` action plus nlri rewriting in the text protocol and is deferred to a follow-up spec.
- **YANG `union { zt:prefix-ipv4 ; zt:prefix-ipv6 }`** over importing `ietf-inet-types`. ze does not load `ietf-inet-types`; the union of ze's existing typedefs avoids adding a third-party YANG dependency.
- **Plugin folder under `internal/component/bgp/plugins/filter_prefix/`** with standard 4-file scaffolding (`filter_prefix.go` entry + dispatch, `match.go` algorithm, `config.go` parser, `register.go` registration) plus `schema/` subdir. Matches the filter_community layout.
- **`atomic.Pointer[map[string]*prefixList]`** for runtime config storage, over RWMutex + map. Lock-free hot-path reads; `OnConfigure` swaps the entire map atomically.

### Resolve pass (post-review)

- **BLOCKER: the ingress filter chain never fired during `.ci` tests** because bgp-rpki crashes during `OnStarted` when it dispatches `adj-rib-in enable-validation` before adj-rib-in's commands are reachable. The crashed plugin blocks `validate-open` indefinitely, the session sits in OpenConfirm, and the UPDATE is never delivered to `notifyMessageReceiver`. Root cause is a startup-ordering race in the framework, not cmd-4. **Fix:** bgp-rpki's `OnStarted` now logs a warning and returns `nil` when `enable-validation` dispatch fails; RPKI still provides RTR cache + validation when adj-rib-in is present but does not kill the session when it isn't. This is a minimal workaround; the proper fix (ensuring all plugin commands are registered with the dispatcher before `OnStarted` fires) is out of scope for cmd-4.
- **BLOCKER: `FormatAttrsForFilter` emitted only attributes, no NLRI.** The filter received update text like `"origin igp as-path 65001 next-hop 1.1.1.1"` with no `nlri ...` section; `extractNLRIField` returned `""` and `evaluateUpdate("")` always returned accept. **Fix:** added `FormatUpdateForFilter(attrs, wireUpdate, declared)` in `filter_format.go` that formats both attributes and NLRI. It handles legacy IPv4 NLRI + Withdrawn-Routes (RFC 4271) and MP_REACH/MP_UNREACH (RFC 4760) for IPv4/IPv6. EVPN, flowspec, VPN, BGP-LS etc. are intentionally not emitted here -- plugins that need those should declare `raw=true` and parse the wire payload themselves. Updated `reactor_notify.go:381` and `reactor_api_forward.go:419` to use the new function.
- **BLOCKER: the `.ci` tests were false positives.** The Python observer plugin's `sys.exit(1)` runs AFTER it dispatches `daemon shutdown`, so ze has already exited cleanly (code 0) by the time the observer's failure exit is reached. The test runner checks only ze's exit code; the observer is decorative. **Fix:** added real `expect=stderr:pattern=` assertions on production-level Info logs (`prefix-list accept` / `prefix-list reject`) emitted by the plugin itself, with `option=env:var=ze.log.bgp.filter.prefix:value=info` to enable the log level. Both forms are asserted AND negated (reject side asserts `reject=stderr:pattern=prefix-list accept` and vice-versa) so a flipped decision flips a test. This shared pattern of "observer-dispatch-then-exit-1" false positive in `redistribution-*.ci` and `community-*.ci` is out of scope for cmd-4, logged as a separate concern.
- **FEATURE REQUEST: optional filter-type prefix.** The umbrella spec example was `filter import [ prefix-list:CUSTOMERS ]`. The initial ship required the full plugin process name `bgp-filter-prefix:CUSTOMERS`. User pushback surfaced that since `BuildFilterRegistry` already enforces filter-name uniqueness across types, the type prefix is structurally redundant. **Fix:** added `FilterTypes []string` to `registry.Registration` so a plugin can declare its owning YANG filter type names. `filter_prefix` declares `FilterTypes: []string{"prefix-list"}`. Added `registry.PluginForFilterType()` and `FilterTypesMap()` lookups. Added `canonicalizeFilterRefs()` in `redistribution.go` that rewrites chain refs at config-parse time. The user can now write any of three forms -- all canonicalize to `bgp-filter-prefix:CUSTOMERS` before runtime dispatch:

  | User writes | Resolution |
  |-------------|------------|
  | `bgp-filter-prefix:CUSTOMERS` | Pass-through (already the plugin process name). |
  | `prefix-list:CUSTOMERS` | Type `prefix-list` looked up in `FilterTypesMap` → plugin `bgp-filter-prefix`. |
  | `CUSTOMERS` | Plain name looked up in `FilterRegistry` → type `prefix-list` → plugin `bgp-filter-prefix`. |

- **BLOCKER: the `ge`/`le` leaves accepted values up to 128 on IPv4 entries**, silently matching nothing at runtime. **Fix:** `parseOneEntry` now computes `familyMax` per entry (32 for IPv4, 128 for IPv6) and rejects ge/le that exceed it with a clear error.
- **BLOCKER: the map-form entry parser was non-deterministic** for multi-entry prefix-lists because Go map iteration is randomized, breaking first-match-wins. **Fix:** `parsePrefixListEntries` now rejects the map form when `len(map) > 1`. Single-entry map form still works (order is irrelevant) for round-trip configs. Multi-entry lists MUST use the slice form (which is what `ordered-by user` YANG produces).
- **ISSUE: test helper `mk` used 0 as a "use default" sentinel, conflicting with the legitimate `ge=0` value for default-route matching.** **Fix:** `mk` no longer substitutes defaults; callers always pass explicit ge/le.
- **ISSUE: `config_test.go` reinvented `strings.Contains`.** **Fix:** replaced with stdlib.
- **ISSUE: unknown-leaf rejection was untested.** **Fix:** added `test/parse/prefix-list-unknown-leaf.ci` that verifies YANG strict mode rejects `typo-leaf` inside a `prefix-list/entry` block.
- **ISSUE: AC-11 composability evidence was weak** -- only single-filter chains tested. **Fix:** added `test/plugin/prefix-filter-chain-order.ci` that uses two prefix-list filters `[ bgp-filter-prefix:ALLOW bgp-filter-prefix:DENY ]` and asserts both fire with the right decisions in order, proving chain dispatch and first-match semantics.

## Consequences

- **The named-filter-chain dispatch path is now production-validated with real assertions.** cmd-5 (AS-path filter), cmd-6 (community match), and cmd-7 (route modify) inherit both the cmd-4 scaffolding AND the new `FormatUpdateForFilter` path AND the optional-prefix chain ref syntax. They differ only in their match algorithm and (for cmd-7) returning `action=modify` with a delta.
- **Users can use any of three chain ref forms interchangeably.** The umbrella spec's intended `prefix-list:CUSTOMERS` shorthand works, plain names work, and the explicit plugin name works. All three canonicalize at parse time; runtime dispatch sees only the canonical form.
- **Per-prefix filtering remains future work.** Until then, an UPDATE with mixed-match prefixes is rejected wholesale. Matches Junos's "strict" prefix-list mode but is stricter than Cisco's "permit some, drop others" default.
- **Non-unicast families (EVPN, flowspec, VPN, BGP-LS, MVPN) flow through the prefix filter unchanged** because `FormatUpdateForFilter` does not emit `nlri ...` blocks for them. Users who attach `bgp-filter-prefix` to an EVPN session will see EVPN routes pass without matching -- which is correct: prefix-list filtering semantically applies only to CIDR-prefix families. Filter plugins that need non-CIDR families should declare `raw=true` and parse wire payloads themselves.
- **`registry.Registration` has a new `FilterTypes []string` field.** Existing plugins (bgp-filter-community, etc.) continue to work unchanged (empty `FilterTypes` means "no short-form resolution offered"). The canonicalizer is a no-op for refs whose prefix is a known plugin process name or for plain names not in the filter registry.
- **Plugin count is 33 internal plugins** (was 32). Tests in `cmd/ze/main_test.go` and `internal/component/plugin/all/all_test.go` updated to include `bgp-filter-prefix`.
- **bgp-rpki `OnStarted` no longer returns error on `enable-validation` dispatch failure.** It logs a warning and continues. A future spec should fix the startup-ordering race so the command is reliably available by the time `OnStarted` fires.

## Gotchas

- The resolver assumes filter names are globally unique. `BuildFilterRegistry` enforces this at config parse (errors on duplicate across types); adding a new filter type that reuses an existing name is a config error, not a silent conflict.
- The `inactive:` prefix is preserved around canonicalization: `inactive:prefix-list:CUSTOMERS` becomes `inactive:bgp-filter-prefix:CUSTOMERS` so the chain's `inactive:` skip logic (in `filter_chain.go:61`) still works.
- `FormatUpdateForFilter` emits families in this order: legacy IPv4 NLRI add, legacy IPv4 NLRI withdrawn, MP_REACH family add, MP_UNREACH family del. Every non-empty block is prefixed with `nlri <family> <op>` so `extractNLRIField` (which cuts on first `nlri `) grabs from the first block to end-of-string. Filters that want per-family handling should parse each `nlri <family> <op>` header; the text protocol allows multiple blocks in one update.
- `FilterInfo`/`FilterOnError` lookup miss is by design SAFE (returns `nil, false` and `"reject"` respectively), not an error. The plugin author does not need to defensively pre-declare every possible runtime filter name -- the engine just forwards the RPC.
- The `pre-write-spec.sh` and `require-related-refs.sh` hooks enforce that cross-references in `// Detail:` and `// Related:` comments point to existing files. Create leaf files BEFORE the hub file when writing a new package.
- The `block-silent-ignore.sh` hook flags any `default:` keyword in a switch. Restructure as `if/else` chains.
- `block-pipe-tail.sh` blocks `make ze-* | tail`. Use `> tmp/foo.log 2>&1` and Read with the Read tool.
- ze does not load `ietf-inet-types`. Use `ze-types` typedefs (`zt:prefix-ipv4`, `zt:prefix-ipv6`, `zt:ipv4-address`, etc.). A future cleanup could add a unified `zt:ip-prefix` union type for reuse across filter plugins.
- **Pre-existing test-infra bug (not fixed here):** the `.ci` test runner treats the Python observer's `sys.exit(1)` after `daemon shutdown` as a no-op because ze has already exited 0. Every `test/plugin/*.ci` that relies on an observer assertion is vulnerable to the same false-positive as cmd-4's initial tests. Logged as a separate concern; workaround is to use `expect=stderr:pattern=` on production log messages.

## Files

- `internal/component/bgp/plugins/filter_prefix/schema/ze-filter-prefix.yang` -- YANG augment of `bgp:bgp/bgp:policy` with `prefix-list` (key=name) containing ordered `entry` (key=prefix, ordered-by user) with `ge`/`le`/`action`
- `internal/component/bgp/plugins/filter_prefix/schema/embed.go` -- `//go:embed` for the YANG file
- `internal/component/bgp/plugins/filter_prefix/schema/register.go` -- `yang.RegisterModule` at init time
- `internal/component/bgp/plugins/filter_prefix/filter_prefix.go` -- Plugin entry point, `RunFilterPrefix(net.Conn) int`, OnConfigure + OnFilterUpdate dispatch, atomic config store; Info-level decision logs
- `internal/component/bgp/plugins/filter_prefix/match.go` -- `evaluatePrefix` (per-route first-match-wins) + `evaluateUpdate` (strict mode walker) + `extractNLRIField` (text parser)
- `internal/component/bgp/plugins/filter_prefix/config.go` -- `parsePrefixLists`, `parseOneEntry` with per-family ge/le validation, multi-entry map-form rejection
- `internal/component/bgp/plugins/filter_prefix/register.go` -- `registry.Register` with `FilterTypes: []string{"prefix-list"}`
- `internal/component/bgp/plugins/filter_prefix/match_test.go` -- 16 cases covering ACs 1-13 + update strict mode + nlri extraction (table-driven)
- `internal/component/bgp/plugins/filter_prefix/config_test.go` -- cases covering YANG defaults, ge/le per-family bounds, ge>le validation, invalid action, malformed prefix, both map and slice config forms, multi-entry map rejection
- `internal/component/plugin/registry/registry.go` -- new `FilterTypes` field on `Registration`, `filterTypes` map, `PluginForFilterType`, `FilterTypesMap`, Reset/Restore/Snapshot handling
- `internal/component/bgp/config/redistribution.go` -- new `canonicalizeFilterRefs`/`canonicalizeOne` for parse-time chain ref rewriting
- `internal/component/bgp/config/peers.go` -- Step 3b2a canonicalization pass on peer Import/Export filters
- `internal/component/bgp/reactor/filter_format.go` -- new `FormatUpdateForFilter` that includes NLRI (legacy IPv4 + MP_REACH/MP_UNREACH)
- `internal/component/bgp/reactor/reactor_notify.go` -- ingress chain now calls `FormatUpdateForFilter`
- `internal/component/bgp/reactor/reactor_api_forward.go` -- egress chain now calls `FormatUpdateForFilter`
- `internal/component/bgp/plugins/rpki/rpki.go` -- `OnStarted` is tolerant of `adj-rib-in enable-validation` dispatch failure
- `internal/component/plugin/all/all.go` -- Regenerated by `make generate` (imports filter_prefix)
- `internal/component/plugin/all/all_test.go` -- Expected plugin list includes `bgp-filter-prefix`
- `cmd/ze/main_test.go` -- `AvailablePlugins` expected list includes `bgp-filter-prefix`
- `test/parse/prefix-list-config.ci` -- YANG schema acceptance
- `test/parse/prefix-list-unknown-leaf.ci` -- YANG strict mode rejects typos in entry leaves
- `test/plugin/prefix-filter-accept.ci` -- Matching prefix triggers `prefix-list accept` log (real stderr assertion)
- `test/plugin/prefix-filter-reject.ci` -- Non-matching prefix triggers `prefix-list reject` log (real stderr assertion)
- `test/plugin/prefix-filter-chain-order.ci` -- Two filters in chain, both fire in order (AC-11 composability)
- `test/plugin/prefix-filter-shortform.ci` -- `prefix-list:NAME` form resolved via FilterTypes map
- `test/plugin/prefix-filter-plain.ci` -- plain `NAME` form resolved via FilterRegistry + FilterTypes map
