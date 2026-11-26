# Spec: yang-enum-vocabulary-agreement-gate

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P1 |
| Release reason | Shipped YANG enumerations and the Go vocabularies that parse them disagree in both directions (`environment/log/backend` omits `kmsg`, `add-path/direction` has dead arm `receive/send`, `diag` capture raw `protocol` omits `bfd`), and the existing enumeration gate misses extra Go values and all 2-value enums. These are defects in shipped config/command surfaces, so they belong before the first release. |
| RFC gaps | - |
| Depends | Split on 2026-11-26 from `plan/release/fixes/spec-yang-single-source-of-truth.md` per owner decision T-1 (2026-11-24: Piece F lands independently of Piece E). Siblings: Piece D `plan/release/fixes/spec-yang-command-arguments-by-leaf-name.md`, Piece E `plan/release/fixes/spec-yang-single-source-of-truth.md`. `software-version/mode` runtime enforcement is owned by `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`. |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**Problem.** Today, `./le arch enumeration check` (`internal/le/arch/enumeration/`)
collects YANG enumerations into a closed corpus (`yangEnumerations` in `corpus.go`),
but has three blind spots that allow Go vocabularies and YANG enums to drift:

1. **One-way detection only:** `Corpus.bestGroup` (`enumeration.go:921`) matches a
   Go syntactic unit only when it contains every value of a YANG enum. It never
   flags an extra string constant in the Go unit that the YANG enum lacks (such
   as `"receive/send"` in `parseAddPathDirection`, `"kmsg"` in `validBackends`,
   or `"bfd"` in `capture_raw.go`).
2. **Minimum size of 3:** `closedGroupKeysMin = 3` (`enumeration.go:885`) makes
   every 2-value YANG enumeration invisible to the gate, so `diag` capture's
   2-value `protocol` enum (`l2tp`, `bgp`) drifted from its Go dispatcher
   (`l2tp`, `bgp`, `bfd`) with zero gate findings.
3. **Unverified `gated by TestX` boilerplate:** `// enumeration: gated by TestX`
   checks only that a function named `func TestX(` exists in a `_test.go` file;
   it cannot verify that `TestX` checks both directions. For example,
   `TestAddPathDirectionsMatchTheYANGModel` checked `YANG -> Go` only and never
   caught the dead `"receive/send"` arm, while `TestLogBackendLeafMatchesSlogutil`
   had to hand-exclude `kmsg`.

**Goal.** Extend `./le arch enumeration check` with bidirectional Go-to-YANG
enumeration agreement (`// enumeration: yang <module>/<leaf-path>`), judge every
mapped Go vocabulary in both directions regardless of enum size (including
2-value enums), migrate the five BGP reactor enum vocabularies onto
`// enumeration: yang` so `internal/component/bgp/reactor/yang_enum_agreement_test.go`
retires its per-package YANG value-set boilerplate (retaining only behavioral
parse/round-trip assertions), and repair the three shipped enum mismatches:

1. `environment/log/backend`: add `enum kmsg` to `ze-hub-conf.yang`, split the
   backend constants out of the mixed log-level `const` block in `slogutil.go`
   and replace the line-136 `gated by` marker with `// enumeration: yang` on
   both the backend `const` block and `validBackends` (`slogutil.go:58` already
   derives `ze.log.backend`'s env description dynamically via `BackendNames()`),
   and remove the `logBackendEnvOnly` exclusion in `log_backend_yang_test.go`.
2. `add-path/direction`: delete the dead `"receive/send"` case arm in
   `parseAddPathDirection` (`internal/component/bgp/reactor/config_capabilities.go`),
   and migrate all five reactor enum tables (`peer_forward_facts.go`,
   `peer_settings.go`, `config.go`, `config_as_set.go`, `config_capabilities.go`)
   onto `// enumeration: yang` with zero dead `gated by` markers remaining.
3. `diag` capture `protocol`: add `enum bfd` to `show diag capture raw`
   `protocol` and include `bfd` in `container raw`'s description in
   `internal/plugins/diag/yang/ze-diag-cmd.yang`, move protocol recognition out
   of the mixed command-token switches in `capture_raw.go` and `capture.go` onto
   dedicated `// enumeration: yang`-annotated protocol lookup maps in
   `capture_common.go`, and keep `show diag capture` `protocol` aligned with
   `HandleShowCapture` (`l2tp`, `bgp`).

Note: `software-version/mode` runtime enforcement (`require` and `refuse`) is
owned solely by `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`;
once it lands, `softver.go` can annotate its mode switch under this gate as well.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` - YANG schema authority and validation
  → Decision: YANG is the single source of truth for config and command enumerations
  → Constraint: Go parsing tables carrying metadata (RFC numbers, handlers, writers) must agree bidirectionally with their YANG enum leaf
- [ ] `docs/architecture/config/environment-block.md` - `environment/log/backend` and `slogutil.ApplyLogConfig`
  → Constraint: config leaves under `environment/log/` map directly to `slogutil` settings; adding `kmsg` to YANG makes config and `ze.log.backend` agree
- [ ] `docs/architecture/core-design.md`, "Command engine and composition roots"
  → Constraint: `internal/le` may link `internal/component/plugin/all` to introspect the full registered YANG model; product code never imports `internal/le`
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md`, `ai/doors/config-yang.md`, `ai/doors/bgp-engine.md`, `ai/doors/hub-engine.md`, `ai/doors/core.md`, `ai/doors/system-plugins.md`
  → Constraint: `internal/core/slogutil` stays under the model and never imports `internal/component/config/yang` (`./le arch tier check`)

**Key insights:**
- `internal/le/arch/enumeration/corpus.go` already links `internal/component/plugin/all` and walks `yang.DefaultLoader()` with `collectEnums`, so resolving `<module>/<leaf-path>` to its enum value set requires no new loader or upward core import.
- A Go table carrying values mapped to handlers or RFC numbers keeps its Go mapping; only the string value set is compared bidirectionally against the YANG enum.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/arch/enumeration/corpus.go` - `yangEnumerations` groups YANG enums by value set with no leaf-path index; `collectEnums` skips nothing with `len(names) > 1`, but `enumeration.go` ignores sets smaller than `closedGroupKeysMin = 3`
- [ ] `internal/le/arch/enumeration/enumeration.go` - `bestGroup` checks only `len(matched) == len(group.Keys)` and `len(group.Keys) >= 3`; `stringConstants` inspects raw `*ast.BasicLit` strings only and does not resolve same-package string `const` identifiers; `// enumeration: gated by TestX` verifies only that `func TestX(` exists in `_test.go`
- [ ] `internal/component/bgp/reactor/config_capabilities.go` - `parseAddPathDirection` (line 499) carries `// enumeration: gated by TestAddPathDirectionsMatchTheYANGModel` and accepts `"send"`, `"receive"`, `"send/receive"`, and `"receive/send"`, while `ze-bgp-conf.yang` declares only `send`, `receive`, `send/receive`
- [ ] `internal/component/bgp/reactor/peer_forward_facts.go` - `sendCommunitySuppression` (line 715) carries `// enumeration: gated by TestSendCommunityKeywordsMatchTheYANGModel` on a 5-case switch matching `ze-bgp-conf` `session/community/send` (`all`, `none`, `standard`, `large`, `extended`)
- [ ] `internal/component/bgp/reactor/peer_settings.go` - `prefixReconnectNames` (line 830) carries `// enumeration: gated by TestPrefixReconnectNamesMatchTheYANGModel`; index 0 holds non-YANG sentinel `PrefixReconnectUnset: "unset"` beside YANG modes `never`, `backoff`, `timer`
- [ ] `internal/component/bgp/reactor/config.go` - lines 61-70 mix `valTrue`, `valFalse`, `valAuto` in the same `const` block as the four `add-path/family/mode` constants (`valEnable`, `valDisable`, `valRequire`, `valRefuse`) under `// enumeration: gated by TestAddPathModeKeywordsMatchTheYANGModel`
- [ ] `internal/component/bgp/reactor/config_as_set.go` - lines 10-13 declare the 2-value `const` block (`asSetPolicyWithdraw`, `asSetPolicyAccept`) for `session/as-set`, unannotated because `closedGroupKeysMin = 3` skipped it
- [ ] `internal/component/bgp/reactor/yang_enum_agreement_test.go` - five hand-written per-package tests with private `yangModel`/`yangEnumValues` helpers; `TestAddPathDirectionsMatchTheYANGModel` tests `YANG -> Go` only and never catches `"receive/send"`
- [ ] `internal/component/hub/yang/ze-hub-conf.yang` - `environment/log/backend` declares `stderr`, `stdout`, `syslog` and omits `kmsg`
- [ ] `internal/core/slogutil/slogutil.go` - line 58 already derives `ze.log.backend`'s env description dynamically via `BackendNames()`; lines 136-147 mix log-level constants (`levelDisabled`..`levelError`) and backend constants (`backendStdout`..`backendKmsg`) under `// enumeration: gated by internal/component/config:TestLogBackendLeafMatchesSlogutil`; `validBackends` (line 362) uses those `const` identifiers as map keys; `writerForBackend` handles `kmsg` via `openKmsg`
- [ ] `internal/component/config/log_backend_yang_test.go` - `TestLogBackendLeafMatchesSlogutil` explicitly deletes `logBackendEnvOnly = "kmsg"` before comparing sets
- [ ] `internal/plugins/diag/yang/ze-diag-cmd.yang` - both `show/capture/protocol` and `show/capture/raw/protocol` declare only `l2tp` and `bgp`, and `container raw` description (lines 95-96) reads `"Protocols: l2tp, bgp."`
- [ ] `internal/plugins/diag/cmd/capture_common.go`, `capture_raw.go`, and `capture.go` - `capture_common.go` declares `capL2TP`, `capBGP`, `capBFD` beside `argCount`, `msgSubsystemNotRunning`, `fmtPcap` under `// enumeration: exempt (...)`; `capture_raw.go:44` switches on `capL2TP`, `capBGP`, `capBFD` inside a mixed command-token switch (`start`/`stop`/`dump`, `pcap`/`json`, `count`); `capture.go:20` switches on `capL2TP`, `capBGP` inside a mixed token switch (`tunnel-id`, `peer`, `count`)

**Behavior to preserve:**
- Existing flat-registry enumeration checks (plugin names, family names, CLI verbs, diagnostic codes) and existing `// enumeration: exempt (...)` / `// enumeration: gated by TestX` annotations continue working without breaking unrelated packages.
- All valid existing configs using `stderr`, `stdout`, `syslog`, `send`, `receive`, `send/receive`, `l2tp`, and `bgp` continue loading identically.

**Behavior to change:**
- `./le arch enumeration check` evaluates every `// enumeration: yang <module>/<leaf-path>` unit in both directions regardless of enum size (2-value enums included), refusing extra Go strings, missing YANG values, and phantom/non-enum leaf paths.
- `environment/log/backend` accepts `kmsg` in config files as well as env vars.
- `parseAddPathDirection` no longer carries the unreachable `"receive/send"` arm.
- `show diag capture raw` accepts `protocol bfd` in the YANG schema and CLI validator/completer, matching `capture_raw.go`.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Enumeration corpus and check | `internal/le/arch/enumeration/corpus.go` `yangEnumerations`; `enumeration.go` `Check`, `bestGroup` | Detects unannotated Go copies of closed YANG enums with >= 3 values and checks `gated by TestX` existence | Extends: adds `// enumeration: yang <module>/<leaf-path>` bidirectional exact-set checking for any enum size, same-package string `const` resolution on mapped units, and phantom-path refusal |
| Per-package reactor enum tables and agreement tests | `internal/component/bgp/reactor/yang_enum_agreement_test.go`; `peer_forward_facts.go` `sendCommunitySuppression`; `peer_settings.go` `prefixReconnectNames`; `config.go` `valEnable..valRefuse`; `config_as_set.go` `asSetPolicy*`; `config_capabilities.go` `parseAddPathDirection` | Five hand-written per-package YANG tests plus four `// enumeration: gated by` markers (and one unannotated 2-value const block) | Replaces (value-set assertions): moves all five reactor enum tables onto `// enumeration: yang <path>` (separating non-YANG sentinel `"unset"` in `peer_settings.go` and non-mode tokens in `config.go`), removes all four reactor `gated by` markers, and retires the five YANG value-set tests while keeping behavioral parse/round-trip assertions |
| Log backend declaration and agreement test | `internal/core/slogutil/slogutil.go` `validBackends`, `BackendNames`, lines 136-147; `internal/component/config/log_backend_yang_test.go` `TestLogBackendLeafMatchesSlogutil` | `slogutil.go:136` gates a mixed level+backend `const` block via `TestLogBackendLeafMatchesSlogutil`, which hand-excludes `kmsg`; `slogutil.go:58` derives `ze.log.backend` help from `BackendNames()` | Extends: splits the backend `const` block from the level constants, replaces the line-136 `gated by` marker with `// enumeration: yang` on the backend `const` block and `validBackends`, and deletes the `kmsg` exclusion in `log_backend_yang_test.go` |
| Diag capture protocol dispatch | `internal/plugins/diag/cmd/capture_common.go`, `capture_raw.go` `handleCaptureRaw`, `capture.go` `HandleShowCapture`; `internal/plugins/diag/yang/ze-diag-cmd.yang` | Mixed token switches match `capL2TP`/`capBGP`/`capBFD` beside `action`/`format`/`count`/`tunnel-id`/`peer`; `ze-diag-cmd.yang` omits `bfd` from `show/capture/raw/protocol` and `container raw` description | Extends: adds `enum bfd` and updates `container raw` description in `ze-diag-cmd.yang`; extracts dedicated `// enumeration: yang`-annotated protocol lookup maps in `capture_common.go` called by `capture_raw.go` and `capture.go` |
| YANG schema loader | `internal/component/config/yang` `DefaultLoader`, `EnumValues` | Loads all registered YANG modules and resolves enumeration value sets | Does not apply: already provides the authoritative resolved enum sets read by `corpus.go` |

## Design documents

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `ai/doors/system.md` | One schema, one command registry, every interface | the door every change reads | "Config is YANG-modeled, and `ConfigProvider` is its single authority" | agrees | Bidirectional enum agreement enforces that Go parsers and YANG enum leaves stay identical in both directions |
| `docs/architecture/core-design.md` | Command engine and composition roots | the // Design: page of `internal/le/arch/enumeration/{corpus,enumeration,report}.go` and `internal/component/bgp/reactor/{config,config_capabilities,peer_forward_facts,peer_settings}.go` | "`le` MAY link the product composition root `internal/component/plugin/all` when a command inventory, CLI-grammar check or YANG-leaf check reads the compiled registry." | agrees | `internal/le/arch/enumeration` already blank-imports `internal/component/plugin/all` to read `yang.DefaultLoader()` for YANG-leaf checks, reports bidirectional findings through the shared renderer in `report.go` (matching the section's shared-renderer rule), and adds no `internal/le` import to product code |
| `ai/doors/le-tooling.md` | Where it sits | the door covering `internal/le/arch/enumeration/` and `test/parse/log-backend-kmsg.ci` | "`le` is allowed to link the product root `internal/component/plugin/all`" | agrees | `corpus.go` reads the live `yang.DefaultLoader()` behind `plugin/all` to resolve every `<module>/<leaf-path>` enum without importing `internal/le` into product code |
| `docs/architecture/config/syntax.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/reactor/config_capabilities.go` and `config.go` | - | silent | Covers config file grammar; ADD-PATH `direction` already spells `send/receive` in YANG, never `receive/send`, and splitting `valEnable`..`valRefuse` changes no config syntax |
| `ai/doors/bgp-engine.md` | Traps | the door covering `internal/component/bgp/reactor/` | "ADD-PATH is per family and per direction, never one bool." | agrees | Deleting the unreachable `"receive/send"` arm in `parseAddPathDirection` keeps all three YANG directions (`send`, `receive`, `send/receive`) intact |
| `docs/architecture/bgp/structural-forwarding.md` | Four costs, and what replaced them | anchored to `internal/component/bgp/reactor/peer_forward_facts.go` | - | silent | Covers route-server forwarding structure and body reuse; annotating `sendCommunitySuppression` with `// enumeration: yang` leaves forwarding behavior and ordering unchanged |
| `docs/architecture/update-building.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/reactor/peer_forward_facts.go` and `peer_settings.go` | - | silent | Covers UPDATE receive, forward, and build paths; annotating `sendCommunitySuppression` and `prefixReconnectNames` leaves UPDATE building unchanged |
| `docs/architecture/api/architecture.md` | Implementation Status | anchored to `internal/component/bgp/reactor/peer_settings.go` | - | silent | Covers plugin process management and bus delivery; separating `PrefixReconnectUnset` from `prefixReconnectNames` leaves `ProcessBinding` and API architecture unchanged |
| `docs/architecture/api/commands.md` | Overview | anchored to `internal/component/bgp/reactor/config.go` | - | silent | Covers API command verbs and transport metadata; splitting `valEnable`..`valRefuse` into their own `const` block in `config.go` leaves command dispatch untouched |
| `docs/architecture/meta/role.md` | Keys Set | anchored to `internal/component/bgp/reactor/config.go` | - | silent | Covers RFC 9234 role metadata keys; splitting the four add-path mode constants in `config.go` leaves peer `remote-ip` and role handling unchanged |
| `docs/architecture/rfc-may-decisions.md` | RFC 4760 Section 7 - Non-Negotiated AFI/SAFI Handling | anchored to `internal/component/bgp/reactor/config.go` | - | silent | Documents optional RFC MAY policy choices; splitting add-path mode constants in `config.go` does not touch `IgnoreFamilyMismatch` or RFC MAY behavior |
| `docs/architecture/wire/capabilities.md` | TL;DR (Read This First) | anchored to `internal/component/bgp/reactor/config.go` | "`enable`/`disable`/`require`/`refuse` — enforcement after negotiation" | agrees | Splitting `valEnable`, `valDisable`, `valRequire`, and `valRefuse` into their own `const` block annotated with `// enumeration: yang` binds Go to those exact four YANG modes |
| `docs/architecture/wire/attributes.md` | Segment Types | the // Design: page of `internal/component/bgp/reactor/config_as_set.go`; anchored to `config_as_set.go` | "The leaf is that operator configuration: an operator may set it on any peer, and `ze exabgp migrate` writes it for an ExaBGP neighbor configured with `as-set accept`." | agrees | Annotating `asSetPolicyWithdraw`/`asSetPolicyAccept` in `config_as_set.go` with `// enumeration: yang` binds Go to the exact two-value `session/as-set` YANG enum without altering RFC 9774 AS_SET behavior |
| `docs/architecture/config/yang-config-design.md` | Claim Completeness Gate | the // Design: page of `yang_enum_agreement_test.go`; anchored to `ze-hub-conf.yang`; a page this spec modifies | "Validation says a config value is well formed." | agrees | Extends this page to document `./le arch enumeration check`'s bidirectional Go-vs-YANG enum agreement beside the schema claim gate |
| `docs/architecture/config/environment-block.md` | TL;DR | anchored to `ze-hub-conf.yang`, `slogutil.go`; the // Design: page of `log_backend_yang_test.go`; a page this spec modifies | "The config loader maps `environment/log/*` to env vars via `slogutil.ApplyLogConfig`" | agrees | Adding `enum kmsg` to `environment/log/backend` makes the config block agree with `slogutil.ApplyLogConfig` and updates this page's `log` section notes |
| `ai/doors/hub-engine.md` | Where it sits | the door covering `internal/component/hub/yang/ze-hub-conf.yang` | "That directory now holds only the `ze-hub-conf.yang` schema" | agrees | Edits `ze-hub-conf.yang` alone to add `enum kmsg` without re-introducing hub orchestration code |
| `docs/architecture/config/environment.md` | Overview | the // Design: page of `internal/core/slogutil/slogutil.go`; anchored to `slogutil.go` | "Each YANG `environment/<section>/<option>` leaf also has a matching env var so the operator can override it at runtime." | agrees | Keeps `ze.log.backend` registered and aligns its description and YANG leaf (`environment/log/backend`) on the same four backend values |
| `docs/architecture/testing/interop.md` | The suites, one per protocol area | anchored to `internal/core/slogutil/slogutil.go` | - | silent | Covers Docker interop suites; untouched by log backend enum alignment |
| `ai/doors/core.md` | Boundaries | the door covering `internal/core/slogutil/slogutil.go` | "Never import upward." | agrees | `internal/core/slogutil` keeps zero imports of `internal/component/config/yang`; the `// enumeration: yang` comment is read from `internal/le/arch/enumeration` |
| `ai/doors/config-yang.md` | Where it sits | the door covering `internal/component/config/log_backend_yang_test.go` | "Each component embeds its `.yang` files and registers them from `init()`" | agrees | `TestLogBackendLeafMatchesSlogutil` reads the live registered `ze-hub-conf` enum via `configyang.EnumValues` with zero hand-coded exclusions |
| `ai/doors/system-plugins.md` | Boundaries | the door covering `internal/plugins/diag/` | "A plugin owns its whole surface." | agrees | `diag` owns both `ze-diag-cmd.yang` and `capture_raw.go`/`capture.go`, and aligns its enum and dispatch switches inside its own package |
| `docs/architecture/diagnostics/packet-capture.md` | Two tiers | the // Design: page of `internal/plugins/diag/cmd/{capture_common,capture_raw,capture}.go`; anchored to all three | "`show capture raw start` with a protocol named (`bgp` or `bfd`) is an error when that capture did not start" | agrees | Adding `enum bfd` and updating `container raw`'s description in `ze-diag-cmd.yang` brings the YANG model into agreement with this page and `capture_raw.go`, while `capture_common.go` holds the dedicated protocol lookup maps |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A commit fails when it configures a feature that programs only the Linux kernel, or that reads only what the Linux kernel sees of the traffic, while the interfaces run on VPP. Two host-side uses stay accepted, so that mixed setups remain possible once VRFs are added: a `fib kernel` block beside a `fib vpp` block, which programs the Linux route table for the router's own traffic, and `sysctl`, which sets the Linux kernel's own tunables. A `fib kernel` block with no `fib vpp` block beside it is refused, for now." | agrees | Does not touch kernel/VPP feature gating |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | Does not touch data-plane mismatch refusal |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "The VPP firewall verifier, not a YANG annotation, decides whether `connection-state` and `mark-set` are accepted on VPP. The `ze:backend` annotation of `flow-offload` is deleted with theirs; `flow-offload` stays refused on VPP because VPP has no flowtable and the `flowtable` list keeps its `ze:backend "kernel"` annotation." | agrees | Unrelated to VPP firewall verification |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every data-plane selector names the plane it programs, `kernel` or `vpp`: `interface backend`, `firewall backend` and `traffic control backend` take `kernel` where they took `netlink`, `nft` and `tc`, and each defaults to `kernel`; the `fib` blocks already name `kernel` and `vpp`. The plane a registration declares and the value a `ze:backend` annotation names are the same word. A second implementation on the same plane, such as eBPF, takes a value of its own when it exists. In Thomas's words (voice-transcribed): "should tc and netfilter be renamed to kernel? so we have vpp and kernel?", then "please do it"." | agrees | Does not touch data-plane selector enums |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "BGP egress next-hop rewriting belongs exclusively in the export filter chain, not under session configuration (`session { next-hop }`). Ze automatically appends `next-hop auto` at the end of the export chain when no earlier export filter has already set a next-hop outcome (`self`, `unchanged`, or an explicit address), so ordinary eBGP rewrites to self while iBGP and route-server clients preserve the received next hop. In Thomas's words (voice-transcribed): "I would think that all the rewrite should be in the prefix export and not under configuration. I believe we should automatically add to the export prefix list the auto at the end of the list if not defined with something else. So if no one, for example, used a self or unchanged, we should automatically change it on eBGP. Because otherwise we have two ways to modify packets on the way out to the peer.", then "Unify into the export chain it is the way I want it."" | agrees | Does not touch egress next-hop policy |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Egress export chains configure next-hop rewriting through one underlying `policychain.Sets` next-hop intent exposed in two authoring forms: `policy { modify <name> { set { next-hop self \| unchanged \| auto \| <ip>; } } }` for matched/conditional rules, and built-in chain references `next-hop:self`, `next-hop:unchanged`, and `next-hop:auto` for peer-wide policy without a wrapper `modify` block. Selected by Thomas on 2026-11-23 ("Option A: Extend policy modify (self\|unchanged\|auto\|<ip>) AND provide built-in next-hop:self / next-hop:unchanged / next-hop:auto chain refs")." | agrees | Does not touch export chain next-hop configuration |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Built-in `next-hop:auto`, `next-hop:self`, and `next-hop:unchanged` are BGP engine steps compiled into `policychain.Chain` (parallel to `loop-detection`), never external plugin RPCs or a `pkg/plugin/rpc.FilterInsertion` tail extension. Config loading appends `next-hop:auto` at the tail of every export chain unless an unconditional `next-hop:self`, `next-hop:unchanged`, or `next-hop:auto` is already present, leaving `Policy.Configured()` false for unconfigured chains; operators override via `next-hop:unchanged`, and `inactive:next-hop:auto` is refused. Selected by Thomas on 2026-11-23 ("Option A: Built-in BGP engine step appended at config compile time (like loop-detection; zero RPC; override via next-hop:unchanged, no inactive:next-hop:auto)")." | agrees | Does not touch `policychain.Chain` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Within an export chain, explicit next-hop sets (`self`, `unchanged`, `auto`, or an IP address) use standard last-write-wins folding while the auto-appended trailing `next-hop:auto` acts only when next-hop is still unset. For a whole-UPDATE raw override (`wireOverride`), Ze compares each emitted next-hop field against the received UPDATE (matching `forward_med.go` provenance): trailing `auto` is skipped only when the raw override changed the next-hop bytes or an earlier chain member already selected `unchanged`/`self`/explicit. Selected by Thomas on 2026-11-23 ("Option A: Provenance check on raw overrides (like MED): skip trailing auto only when the raw override changed the next-hop bytes or an earlier member set unchanged/self")." | agrees | Does not touch export chain folding |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Explicit next-hop addresses in `policy { modify <name> { set { next-hop <ip>; } } }` and `policychain.Sets.NextHop` support both IPv4 (attr 3) and IPv6/MP_REACH (through `NextHopProfile` and `ValidatePeerGlobalNextHop`) on both export and import chains. Egress-only next-hop modes (`self`, `unchanged`, `auto`), whether written as built-in `next-hop:*` refs or inside `policy { modify }`, are refused on import chains at config load. Selected by Thomas on 2026-11-23 ("Option A: Support IPv4 + IPv6/MP_REACH for explicit next-hop on both export and import; refuse self/unchanged/auto on import at config load")." | agrees | Does not touch `policychain.Sets.NextHop` |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Auto-appended trailing `next-hop:auto` applies only to forwarded routes and preserves the originator-chosen next hop on locally originated routes (static config routes, ExaBGP `announce`, API announcements). Explicit export-chain next-hop actions (`next-hop:self`, `modify { set { next-hop ... } }`) apply uniformly to both forwarded and locally originated routes. Selected by Thomas on 2026-11-23 ("Option A: Trailing next-hop:auto applies to forwarded routes only (preserves static/ExaBGP/API explicit next-hop); explicit next-hop:self / modify next-hop applies to both forwarded and originated routes")." | agrees | Does not touch route origination next-hop handling |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-23 | "Every automatic filter-chain member Ze inserts — both built-in BGP engine steps (`next-hop:auto` at the export tail, `loop-detection` at the import head) and plugin-declared `FilterInsertions` — is published through one queryable implied-filter seam carrying direction, position, reference and `implied-by` reason so `\| implied` displays every extra chain member alongside the operator's written filter list. In Thomas's words (2026-11-23): "Can you make sure that \| implied show the extra chain members when it displays the filter ?"." | agrees | Does not touch implied filter publication |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "Ze always starts. The configuration of one service never makes `ze start` exit or refuse to start: a service whose configuration cannot be served is not started, ze runs with its other services, and the service is reported with its fix by an error line in the log under a doctor code and by `ze doctor` under the same code. Examples are a web certificate name that does not resolve, a looking-glass `server` entry with no certificate to present, and a looking-glass address list that does not parse. Where a service has several endpoints, only the endpoint that cannot be served stays down and the others serve. A reload whose config cannot be served stays refused as a whole, and the running services keep what they serve. In Thomas's words (voice-transcribed): "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"; asked about a reload, "reload stay refused"; asked "Should only that endpoint stay down while the others serve", "yes". The reason is that a daemon that does not start brings up no networking, so the operator cannot reach the box to correct it." | agrees | Enum agreement is a build-time `./le arch enumeration check` gate; daemon startup availability is unchanged |
| `docs/architecture/core-design.md` | Design decisions | decision of 2026-11-26 | "The rule of 2026-11-23 above, that a configuration which holds a feature that programs only the Linux kernel while the interfaces run on VPP "is refused at daemon start", is superseded for the start: ze starts, that feature stays off, and `ze doctor` reports the mismatch with its fix. A commit, a reload and `ze config validate` still refuse such a configuration, by the same verdict as before. Thomas was asked whether the rule that ze always starts replaces the refusal at start, "(a) yes, ze starts, that feature stays off and ze doctor reports the mismatch, while commit and reload still refuse it; (b) no", and answered "2a" (voice-transcribed). The reason is the one of the row above: a daemon that does not start brings up no networking, so the operator cannot reach the box to correct the mismatch." | agrees | Does not modify kernel/VPP startup mismatch behavior |
| `docs/architecture/update-building.md` | Design decisions | decision of 2026-11-23 | "Gate draft-simpson 0x0800 FlowSpec MP_REACH next-hop emission behind an explicit per-session opt-in (`session { flowspec-redirect-next-hop true; }`, default `false`), written automatically by `ze exabgp migrate` when a migrated neighbor uses Simpson actions or serves an API process on a FlowSpec family" | agrees | Does not modify FlowSpec redirect next-hop opt-in or emission |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "When `\| implied` modifies an existing config line (such as inserting automatic members into a `filters [ ... ];` list), it keeps the line intact and places an inline `# implied: ...` comment on the same line after the statement naming what was added and why, rather than placing a comment only above the line or splitting the list across multiple lines. In Thomas's words (2026-11-23): "No, it should be on the same line and say what was added"." | agrees | Does not touch `\| implied` config formatting |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "Everything on the bus is a participant named by its configuration path. A participant registers the messages it sends and receives, events and commands alike, and with nothing written that registration is what runs" | agrees | Does not touch bus participant registration or routing |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `send > to` list written on a participant replaces the destinations its messages have by registration; it never adds to them. Present and empty means nobody" | agrees | Does not touch bus `send > to` destination filtering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A `receive > from` list limits who may address a participant. Absent means everybody; present and empty means nobody" | agrees | Does not touch bus `receive > from` filtering |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "A send and a receive allow-list of message names (`send > allow`, `receive > allow`) exist on every participant: absent means everything declared, empty means nothing. The bus enforces them when it receives the message" | agrees | Does not touch bus message allow-lists |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "What is written on the participant and what is written in the `bus` section compile into one filter per participant. A disagreement between the two places, or between an allow-list and a destination on one participant, is refused at commit naming both lines" | agrees | Does not touch bus filter compilation |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "The bus does not arbitrate between participants: no exclusive-role rule, no commit refusal and no silent winner. A plugin that must not duplicate another limits itself" | agrees | Does not touch bus participant arbitration |
| `docs/architecture/api/architecture.md` | Design decisions | decision of 2026-11-23 | "One display shows every registered default beside what the operator wrote, the defaults a written list replaced, and the line that produced each effective entry" | agrees | Does not touch bus default/override display |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-08-21 | "The pipe wins: a command path names the subject and stops; scope, presentation and format are pipe operations after it, selector words included" | agrees | Does not touch command pipe grammar or selector positioning |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-20 | "A command may override any generic pipe operation that acts on its data, as long as the answer keeps the generic's data format: every data operator of the catalog is overridable, each held to its generic's answer shape; a format, paging, `save` and `log` are the data format or its display, and stay the generic's. In Thomas's words: "I believe it should be possible to overwrite more command as long as their data format is not changed."" | agrees | Does not touch generic pipe operation overrides |
| `docs/architecture/api/commands.md` | Design decisions | decision of 2026-11-23 | "A pipe operation works on the answer of the command before it. It may narrow the answer, extend it, transform it, or deliver it somewhere, such as saving it to a file. It never changes what the command does" | agrees | Does not touch pipe operation semantics |
| `docs/architecture/testing/interop.md` | Design decisions | decision of 2026-11-23 | "Each Docker interop suite and scenario checks only the kernel capabilities it actually requires (`Suite.KernelCapabilities` unioned with selected `ScenarioPlan.KernelCapabilities`); suites whose selected scenarios require none run without unrelated kernel features (such as SRv6 or IPsec)" | agrees | Does not modify Docker interop suite capability declarations |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build/CI gate: `./le arch enumeration check` (and unit tests in `internal/le/arch/enumeration/`).
- Operator config: `environment { log { backend kmsg; } }`.
- Operator CLI: `show diag capture raw start protocol bfd` / `stop` / `dump`.

### Transformation Path
1. `corpus.go` resolves the full YANG schema via `yang.DefaultLoader()` and indexes every `gyang.Yenum` leaf by canonical `<module>/<path>` (for every enum of size >= 1, with no 3-value floor).
2. `enumeration.go` builds a per-package string-constant table from `const` declarations across each scanned package's non-test `.go` files, parses Go AST units (`*ast.CompositeLit`, `*ast.SwitchStmt`, `const` `*ast.GenDecl`), and reads `// enumeration: yang <module>/<leaf-path>` directives.
3. For each mapped unit, `enumeration.go` resolves the leaf path (failing on unknown or non-enum paths), extracts the Go unit's string values according to its AST shape — `CaseClause.List` expressions for `*ast.SwitchStmt`, `KeyValueExpr.Key` for `*ast.MapType` composite literals, element expressions or `KeyValueExpr.Value` for `*ast.ArrayType` composite literals (including index-keyed arrays like `prefixReconnectNames`), and `ValueSpec.Values` RHS expressions for `const` `*ast.GenDecl` — resolving both string `*ast.BasicLit` literals and same-package string `const` `*ast.Ident` references (ignoring `""` empty sentinels). It then suppresses any legacy unmapped closed-corpus finding on that unit and compares the two sets bidirectionally: any value in YANG missing from Go or in Go missing from YANG is a finding. Legacy unmapped scans keep their existing literal matching unchanged so unrelated packages do not regress.
4. On the product side: `ze-hub-conf.yang` exposes `kmsg` to `ValidateLeafValue` and `ApplyLogConfig` -> `slogutil` (where `slogutil.go` splits the backend constants from the level constants and annotates both the backend `const` block and `validBackends`, while `slogutil.go:58` already derives `ze.log.backend`'s env description dynamically via `BackendNames()`); `ze-diag-cmd.yang` adds `enum bfd` to `show/capture/raw/protocol` and updates `container raw`'s description, while `capture_common.go` holds dedicated `// enumeration: yang`-annotated protocol lookup maps checked by `capture_raw.go` and `capture.go` outside their mixed command-token switches; and all five BGP reactor enum vocabularies (`peer_forward_facts.go`, `peer_settings.go`, `config.go`, `config_as_set.go`, `config_capabilities.go`) carry `// enumeration: yang` (with dead `"receive/send"` deleted from `parseAddPathDirection`, index-0 `"unset"` handled explicitly in `PrefixReconnectMode.String()` outside `prefixReconnectNames`, and `valEnable`..`valRefuse` split into their own `const` block in `config.go`).

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| `internal/le` → product YANG schema | Blank import `_ "github.com/ze-software/ze/internal/component/plugin/all"` + `yang.DefaultLoader()` (allowed directional le -> ze dependency) | No |
| YANG schema → CLI / config validator | `ValidateLeafValue` (`environment/log/backend`) and `ValidateArgs` (`show diag capture raw protocol`) | No |

### Changed-Wire Census

**Search evidence:** scope `internal/`, `pkg/`, `cmd/`, `test/`; terms `backendKmsg`, `receive/send`, `capBFD`, `ze-diag:show-capture-raw`, `enumeration:`; positive controls: `slogutil.go:332` (`backendKmsg`), `config_capabilities.go:505` (`receive/send`), `capture_raw.go:103` (`capBFD`), `ze-diag-cmd.yang:94` (`ze-diag:show-capture-raw`), `enumeration.go:87` (`enumeration:`). No network, plugin IPC, or persistent storage wire format changes: this spec changes only the development lint gate (`internal/le/arch/enumeration`), YANG enum declarations for existing Go handlers (`kmsg`, `bfd`), dedicated Go protocol/const tables and `// enumeration: yang` annotations, and deletion of an unreachable Go switch arm (`receive/send`).

### Integration Points
- `internal/le/arch/enumeration` (`./le arch enumeration check` and `./le arch enumeration report`) - enforces bidirectional Go-vs-YANG enum agreement across the tree.
- `internal/component/config/yang` `DefaultLoader()` - provides authoritative enum leaf sets.
- `internal/core/slogutil` and `internal/plugins/diag/cmd` - aligned consumers.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | Gate reads `yang.DefaultLoader()` directly; config and CLI validation read YANG directly |
| No unintended coupling (components stay isolated) | No | `internal/core/slogutil` still imports no config/YANG package; `internal/le` introspects product registries in the allowed direction |
| No duplicated functionality (extends existing, does not recreate) | No | Extends `internal/le/arch/enumeration` rather than building a second enum check; retires per-package value-set tests in `yang_enum_agreement_test.go` |
| Zero-copy preserved where applicable (refs, not copies) | No | No hot-path wire code touched |
| Registration over hardcoding, outbound | No | Gate discovers all YANG enums from `yang.DefaultLoader()` dynamically |
| Registration over hardcoding, inbound | No | Searched `yang_enum_agreement_test.go`, `log_backend_yang_test.go`, `enumeration.go`; hand-written `kmsg` exclusion and per-table value-set boilerplate removed |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every YANG enum leaf path in `yang.DefaultLoader()` has a deterministic `<module>/<path>` key | `corpus.go:319-327` already walks sorted modules and sorted `entry.Dir` children | Duplicate or unstable paths across augments | Unit test `TestEveryYANGEnumMappingAgrees` over the full loaded model | unvalidated |
| A-2 | `show diag capture` (structured) handles only `l2tp` and `bgp`, while `show diag capture raw` handles `l2tp`, `bgp`, and `bfd` | Verified by reading `internal/plugins/diag/cmd/capture.go` (lines 21, 47, 49) and `capture_raw.go` (lines 48, 76, 103) | Adding `bfd` to structured `capture` would accept a value with no handler | Unit test + `capture_raw_test.go` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Switch statements that also have string literals inside case bodies (e.g. error messages or log strings) could look like extra enum values if the AST extractor walks the entire switch body | False-positive "extra in Go" findings on mapped switches | For `*ast.SwitchStmt`, extract string constants from `case` clause expressions (`CaseClause.List`) only, never from case bodies; for `*ast.CompositeLit` maps, extract map keys (`KeyValueExpr.Key`) only |
| R-2 | Existing `// enumeration: gated by TestX` rows across 50+ files could break if removed all at once | Massive unrelated diff across 30+ packages | Keep existing `gated by TestX` accepted while adding `// enumeration: yang <module>/<leaf-path>` and migrating the owned reactor/slogutil/diag sites now |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | `./le arch enumeration check` reports false positives or misses drift; `environment { log { backend kmsg; } }` or `show diag capture raw start protocol bfd` fails validation |
| How is it reverted? | Single commit revert |
| Who else touches this path? | `spec-software-version-mode-implements-two-of-four.md` (`softver.go` mode parsing) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A Go switch or map annotated with `// enumeration: yang <path>` that holds an extra value or omits a YANG enum value (including a 2-value enum) | → | `archenumeration.Check` bidirectional enum agreement check | `TestEveryYANGEnumMappingAgrees` |
| Config file setting `environment { log { backend kmsg; } }` | → | YANG validator (`ze-hub-conf.yang`) → `slogutil.ApplyLogConfig` | `test/parse/log-backend-kmsg.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A Go const block, composite literal, or switch carrying `// enumeration: yang <module>/<leaf-path>` omits a YANG enum value or includes an extra string constant the YANG enum lacks, for any enum size (including 2-value enums), whether values are written as string literals or same-package string `const` identifiers | `./le arch enumeration check` resolves same-package string `const` identifiers (e.g., `backendStderr`..`backendKmsg` in `slogutil.go`, `capL2TP`/`capBGP`/`capBFD` in `capture_common.go`) across the package's non-test `.go` files and fails naming the file, line, `<module>/<leaf-path>`, every value missing from Go, and every extra value absent from YANG | String literals inside switch case bodies (log messages, errors) and map values (handlers, RFC numbers, booleans) are not compared; only `CaseClause.List` expressions (switches), `KeyValueExpr.Key` (`*ast.MapType` literals), element expressions or `KeyValueExpr.Value` (`*ast.ArrayType` slice/array literals, including index-keyed arrays), or `ValueSpec.Values` RHS (`const` blocks) are compared; legacy unmapped scans keep existing literal matching unchanged |
| AC-2 | A `// enumeration: yang <module>/<leaf-path>` directive names a path absent from `yang.DefaultLoader()` or pointing to a non-enum node, or sits on no Go unit | `./le arch enumeration check` fails naming the phantom or dead directive and the fix | None |
| AC-3 | `environment { log { backend kmsg; } }` in a config file and `slogutil.go` backend declarations | Config validation and startup accept `kmsg`; `slogutil.go:58` already derives `ze.log.backend`'s description dynamically via `BackendNames()`; `slogutil.go` splits the four backend constants (`backendStdout`..`backendKmsg`) out of the mixed log-level `const` block (lines 136-147), replaces the line-136 `// enumeration: gated by` marker with `// enumeration: yang ze-hub-conf/environment/log/backend` on the backend `const` block, and annotates `validBackends` (line 362) with `// enumeration: yang ze-hub-conf/environment/log/backend`; `log_backend_yang_test.go` deletes `logBackendEnvOnly` and verifies exact equality | Comma-separated multi-backend env overrides (`ze.log.backend=kmsg,stderr`) remain supported on the env-var path as today |
| AC-4 | `parseAddPathDirection` (`internal/component/bgp/reactor/config_capabilities.go`) against `ze-bgp-conf` `add-path/direction` | The dead `"receive/send"` arm is removed; the switch replaces its `gated by` marker with `// enumeration: yang ze-bgp-conf/bgp/group/peer/session/capability/add-path/direction` and agrees bidirectionally (`send`, `receive`, `send/receive`) | None |
| AC-5 | `show diag capture raw start protocol bfd` and `show diag capture` `protocol` against `ze-diag-cmd.yang` | `ze-diag-cmd.yang` `show/capture/raw/protocol` declares `l2tp`, `bgp`, `bfd` and `container raw` description reads `"Protocols: l2tp, bgp, bfd."`; `show/capture/protocol` declares `l2tp`, `bgp`; `capture_common.go` defines dedicated `captureRawProtocols` (`capL2TP`, `capBGP`, `capBFD`) and `captureProtocols` (`capL2TP`, `capBGP`) lookup maps annotated with `// enumeration: yang`, checked by `capture_raw.go` and `capture.go` outside their mixed command-token switches | Structured `show diag capture` keeps `l2tp` and `bgp` only because BFD has raw capture only (`capture_raw.go`), not structured capture (`capture.go`); `capture_common.go`'s `const` block keeps its `// enumeration: exempt (...)` for the flat plugin-names corpus |
| AC-6 | Shipped tree under `./le arch enumeration check`, all five reactor enum tables, and `yang_enum_agreement_test.go` retirement | `./le arch enumeration check` exits 0 with zero dead `// enumeration: gated by` markers; all five reactor enum vocabularies carry `// enumeration: yang`: `peer_forward_facts.go` (`sendCommunitySuppression` -> `session/community/send`), `peer_settings.go` (`prefixReconnectNames` omitting index-0 `PrefixReconnectUnset` while `String()` returns `"unset"` for `PrefixReconnectUnset` -> `session/family/prefix/reconnect`), `config.go` (dedicated 4-const block `valEnable`..`valRefuse` split from `valTrue`/`valFalse`/`valAuto` -> `session/capability/add-path/family/mode`), `config_as_set.go` (`asSetPolicyWithdraw`/`asSetPolicyAccept` -> `session/as-set`), and `config_capabilities.go` (`parseAddPathDirection` -> `session/capability/add-path/direction`); `yang_enum_agreement_test.go` retires all five per-package YANG value-set tests and its private `yangModel`/`yangEnumValues` helpers while retaining behavioral parse/round-trip assertions | Existing `// enumeration: gated by TestX` directives in unrelated packages (including `capture_raw.go:21` action constants) stay valid until migrated |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures `environment { log { backend kmsg; } }` | Config parser -> YANG enum validator (`ze-hub-conf.yang`) -> `ApplyLogConfig` -> `slogutil.writerForBackend` | `test/parse/log-backend-kmsg.ci` |
| 2 | Runs `show diag capture raw start protocol bfd` | CLI -> `ValidateArgs` (`ze-diag-cmd.yang`) -> `captureRawStart` (`capture_raw.go`) | `internal/plugins/diag/cmd/capture_raw_test.go` + `./le arch enumeration check` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestEveryYANGEnumMappingAgrees` | `internal/le/arch/enumeration/yang_mapping_test.go` | AC-1, AC-2 (missing value, extra value, 2-value enum, same-package string `const` resolution, index-keyed array literals, switch-body strings and map values ignored, phantom/non-enum path, clean match) | |
| `TestLogBackendLeafMatchesSlogutil` | `internal/component/config/log_backend_yang_test.go` | AC-3 (exact bidirectional equality including `kmsg`, no exclusion) | |
| `TestAddPathDirectionRejectsReceiveSend` | `internal/component/bgp/reactor/yang_enum_agreement_test.go` | AC-4, AC-6 (`receive/send` and unknown strings return `AddPathNone`; all three valid directions return non-None) | |
| `TestPrefixReconnectModeRoundTripAndRejectsUnset` | `internal/component/bgp/reactor/yang_enum_agreement_test.go` | AC-6 (`PrefixReconnectUnset.String() == "unset"`, `parsePrefixReconnectMode("unset")` returns `false`, every configured mode round-trips) | |
| `TestSendCommunitySuppressionBehavior` | `internal/component/bgp/reactor/yang_enum_agreement_test.go` | AC-6 (`"none"` and unknown keywords fail closed suppressing all three types; `"all"`, `"standard"`, `"large"`, `"extended"` leave the expected types unsuppressed) | |
| `TestDiagCaptureRawProtocolAcceptsBFD` | `internal/plugins/diag/cmd/capture_raw_test.go` | AC-5 (dispatching `show diag capture raw` with `protocol bfd` validates against YANG and routes through `captureRawProtocols`) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric inputs added | N/A | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `log-backend-kmsg` | `test/parse/log-backend-kmsg.ci` | Config with `environment { log { backend kmsg; } }` passes `ze config validate` | |

### Interop Tests (Scope: protocol)
N-A: Scope is config and development lint gate; no protocol wire change.

## Files to Modify
- `internal/le/arch/enumeration/corpus.go` - index YANG enum value sets by `<module>/<leaf-path>` for all enum sizes
- `internal/le/arch/enumeration/enumeration.go` - build per-package string-const table, parse `// enumeration: yang <module>/<leaf-path>`, extract case/map-key/array-element/const value sets (resolving same-package string `const` idents), suppress legacy closed-corpus finding on mapped units, compare bidirectionally, refuse phantom/dead directives
- `internal/le/arch/enumeration/report.go` - report missing, extra, and phantom enum mapping findings with fixes through the shared renderer
- `internal/component/bgp/reactor/config_capabilities.go` - remove `"receive/send"` from `parseAddPathDirection` and replace `gated by` with `// enumeration: yang ze-bgp-conf/bgp/group/peer/session/capability/add-path/direction`
- `internal/component/bgp/reactor/peer_forward_facts.go` - replace `gated by` on `sendCommunitySuppression` switch (line 715) with `// enumeration: yang ze-bgp-conf/bgp/group/peer/session/community/send` and update doc comment
- `internal/component/bgp/reactor/peer_settings.go` - omit index-0 `PrefixReconnectUnset` from `prefixReconnectNames` (handle `PrefixReconnectUnset` returning `"unset"` in `String()`), replace `gated by` on line 830 with `// enumeration: yang ze-bgp-conf/bgp/group/peer/session/family/prefix/reconnect`, and update doc comment
- `internal/component/bgp/reactor/config.go` - split `valEnable`, `valDisable`, `valRequire`, `valRefuse` out of lines 61-70 into a dedicated `const` block separate from `valTrue`, `valFalse`, `valAuto`, replacing `gated by` with `// enumeration: yang ze-bgp-conf/bgp/group/peer/session/capability/add-path/family/mode`
- `internal/component/bgp/reactor/config_as_set.go` - annotate `asSetPolicyWithdraw`/`asSetPolicyAccept` `const` block (line 10) with `// enumeration: yang ze-bgp-conf/bgp/group/peer/session/as-set`
- `internal/component/bgp/reactor/yang_enum_agreement_test.go` - retire all five YANG value-set boilerplate tests and private `yangModel`/`yangEnumValues` helpers; keep behavioral parse/round-trip assertions
- `internal/component/hub/yang/ze-hub-conf.yang` - add `enum kmsg` to `environment/log/backend`
- `internal/core/slogutil/slogutil.go` - split backend constants (`backendStdout`..`backendKmsg`) out of the mixed log-level `const` block (lines 136-147), replace the line-136 `gated by` marker with `// enumeration: yang ze-hub-conf/environment/log/backend` on the backend `const` block and annotate `validBackends` (line 362) with `// enumeration: yang ze-hub-conf/environment/log/backend`
- `internal/component/config/log_backend_yang_test.go` - delete `logBackendEnvOnly = "kmsg"` exclusion
- `internal/plugins/diag/yang/ze-diag-cmd.yang` - add `enum bfd` to `show/capture/raw/protocol` and update `container raw` description (lines 95-96) to `"Protocols: l2tp, bgp, bfd."`
- `internal/plugins/diag/cmd/capture_common.go` - define dedicated `captureRawProtocols` (`capL2TP`, `capBGP`, `capBFD`) and `captureProtocols` (`capL2TP`, `capBGP`) lookup maps annotated with `// enumeration: yang`
- `internal/plugins/diag/cmd/capture_raw.go` - check `captureRawProtocols[a]` outside the mixed token switch in `handleCaptureRaw`
- `internal/plugins/diag/cmd/capture.go` - check `captureProtocols[a]` outside the mixed token switch in `HandleShowCapture`
- `docs/architecture/config/environment-block.md` - document `kmsg` in `environment/log/backend`
- `docs/architecture/config/yang-config-design.md` - document bidirectional Go-vs-YANG enum agreement in `./le arch enumeration check`

## Files to Create
- `internal/le/arch/enumeration/yang_mapping_test.go` - unit tests for bidirectional enum mapping checks
- `test/parse/log-backend-kmsg.ci` - functional parse test for `environment { log { backend kmsg; } }`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `ze-hub-conf.yang` (`kmsg`), `ze-diag-cmd.yang` (`bfd`) |
| YANG validation constraints | Yes | Enumerations in `ze-hub-conf.yang` and `ze-diag-cmd.yang` |
| YANG custom validators | No | None |
| CLI commands/flags | No | Existing `show diag capture raw` gains `bfd` enum value only |
| CLI grammar (keyword before value) | Yes | `./le cli grammar` verifies `ze-diag-cmd.yang` |
| Editor autocomplete | Yes | Automatically offers `kmsg` and `bfd` from YANG enums |
| Functional test for new RPC/API | Yes | `test/parse/log-backend-kmsg.ci` |
| Pipe completeness | No | No command answer shape changes |
| Env var registration | No | `slogutil.go:58` already derives `ze.log.backend`'s env description dynamically via `BackendNames()`, which already includes `kmsg` |
| Doctor check for runtime dependencies | No | No new runtime dependency |
| Doctor code per problem class | No | No new runtime problem class |
| Prometheus counters/metrics | No | None |
| BGP family surface | No | None |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | Aligns shipped YANG enums with existing Go handlers |
| 2 | Config syntax changed? | Yes | `docs/architecture/config/environment-block.md` (`environment/log/backend` adds `kmsg`) |
| 3 | CLI command added/changed? | No | `show diag capture raw` already accepts `protocol`; `bfd` is an enum value |
| 4 | API/RPC added/changed? | No | None |
| 5 | Plugin added/changed? | No | None |
| 6 | Has a user guide page? | No | None |
| 7 | Wire format changed? | No | None |
| 8 | Plugin SDK/protocol changed? | No | None |
| 9 | RFC behavior implemented, changed, or newly proven? | No | None |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/config/yang-config-design.md` (bidirectional enum agreement gate) |
| 11 | Affects daemon comparison? | No | None |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | No | None |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | None |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Derived with `./le spec citation anchors spec plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md`. Declared by changed files: `docs/architecture/config/yang-config-design.md` (updated for bidirectional enum gate), `docs/architecture/config/environment-block.md` (updated for `kmsg`), `docs/architecture/config/environment.md` (unaffected: `ze.log.backend` env key already registered and documented), `docs/architecture/diagnostics/packet-capture.md` (unaffected: already documents `show capture raw start` for `bgp` and `bfd`), `docs/architecture/core-design.md` (unaffected: le-to-ze direction and shared renderer unchanged), `docs/architecture/wire/attributes.md` (unaffected: annotating `asSetPolicyWithdraw`/`asSetPolicyAccept` in `config_as_set.go` leaves RFC 9774 AS_SET behavior unchanged) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/architecture/config/environment-block.md` updated for `kmsg` |

## Design presentation

### What changes

`./le arch enumeration check` gains bidirectional Go-vs-YANG enum agreement via
`// enumeration: yang <module>/<leaf-path>`, refusing both missing YANG values
and extra dead Go arms regardless of enum size (including 2-value enums). Three
shipped enum mismatches are fixed at the same time: `environment/log/backend`
adds `kmsg`, `parseAddPathDirection` drops the dead `receive/send` arm, and
`show diag capture raw protocol` adds `bfd`.

### Before and after

1. **Configuring `environment { log { backend kmsg; } }`.**
   Before: Refused by YANG validation (`invalid enum: "kmsg"`), even though `slogutil` implements `kmsg` via `ze.log.backend=kmsg`.
   After: Accepted by YANG validation, offered by editor autocomplete, and wired to `slogutil`'s `/dev/kmsg` handler.
2. **Running `show diag capture raw start protocol bfd`.**
   Before: Refused by `ValidateArgs` because `ze-diag-cmd.yang` lists only `l2tp` and `bgp`, even though `capture_raw.go` implements and tests `bfd`.
   After: Accepted by `ValidateArgs` and tab-completed in the CLI.
3. **Adding a dead arm (`"receive/send"`) or an extra value to a Go parser mapped to a YANG enum (including 2-value enums).**
   Before: `./le arch enumeration check` exits 0 because it checks only one direction and skips enums with fewer than 3 values.
   After: `./le arch enumeration check` fails naming the leaf path, the extra Go value, and the fix.

### Alternatives set aside

| Alternative | Why set aside |
|-------------|---------------|
| Keep adding one per-package `yang_enum_agreement_test.go` test per enum | Manual hand-list of tests; switches cannot be enumerated from outside the switch, so `TestAddPathDirectionsMatchTheYANGModel` missed `"receive/send"` |
| Lower `closedGroupKeysMin` from 3 to 2 for unmapped scans | Would cause massive false positives across the codebase on common two-word pairs (`true`/`false`, `in`/`out`, `ipv4`/`ipv6`); explicit `// enumeration: yang <path>` checks 2-value enums with zero noise |
| Drop `kmsg` from `slogutil.go` or drop `bfd` from `capture_raw.go` | Both backends are real, tested shipped capabilities (`docs/guide/appliance.md` uses `kmsg`, `capture_raw_test.go` proves `bfd` capture) |

### Documents departed from

none

### Choices for Thomas

none

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- add `TestEveryYANGEnumMappingAgrees` (failing) and `test/parse/log-backend-kmsg.ci` (failing).
2. **Phase: Gate** -- add `<module>/<leaf-path>` index in `corpus.go`, per-package string-const table, `// enumeration: yang <path>` parsing, shape-specific AST extraction (switches, map keys, array/slice elements/values, const blocks), legacy closed-corpus suppression, and bidirectional comparison in `enumeration.go`, and shared-renderer diagnostics in `report.go` (AC-1, AC-2).
3. **Phase: Shipped Mismatches & Reactor Migration** -- fix `ze-hub-conf.yang` / `slogutil.go` (split backend const block + annotate const block and `validBackends`) / `log_backend_yang_test.go` (`kmsg`); migrate all five reactor tables (`config_capabilities.go` dropping `"receive/send"`, `peer_forward_facts.go`, `peer_settings.go` handling `"unset"` in `String()`, `config.go` splitting `valEnable`..`valRefuse`, `config_as_set.go`) and retire superseded YANG value-set tests in `yang_enum_agreement_test.go`; update `ze-diag-cmd.yang` (`enum bfd` + `container raw` description) and extract dedicated annotated protocol maps in `capture_common.go` called from `capture_raw.go` and `capture.go` (AC-3..AC-6).
4. **Phase: Docs** -- update `docs/architecture/config/environment-block.md` and `docs/architecture/config/yang-config-design.md`.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-6 has an implementation at file:line and zero dead `gated by` markers remain across reactor and `slogutil.go` |
| Correctness | Switch extraction reads `CaseClause.List` only (never body strings); map extraction reads `KeyValueExpr.Key` only; array/slice extraction reads elements or index-keyed `KeyValueExpr.Value`; same-package string `const` idents resolve on mapped units |
| Data flow | `internal/core/slogutil` adds zero upward imports |
| Rule: no-layering | Dead `"receive/send"` arm, hand-coded `logBackendEnvOnly` exclusion, and private `yangModel`/`yangEnumValues` boilerplate deleted |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Bidirectional enum gate | `./le test unit pkg path ./internal/le/arch/enumeration/ && ./le arch enumeration check` |
| Log backend `kmsg` | `./le test unit pkg path ./internal/component/config/ && ./le test functional parse` |
| ADD-PATH and diag fixes | `./le test unit pkg path ./internal/component/bgp/reactor/ && ./le test unit pkg path ./internal/plugins/diag/cmd/ && ./le cli grammar` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | YANG enum leaves reject any value outside the declared set before reaching `slogutil` or `diag` handlers |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Gate reports a false positive on a switch body string | Narrow AST extraction to `CaseClause.List` expressions |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows

Moved here on 2026-11-26 from `plan/release/fixes/spec-yang-single-source-of-truth.md` per owner T-1 split (2026-11-24).

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-14 | `environment/log/backend` enum omits `kmsg` |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-11 | ADD-PATH `direction` enum omits `receive/send` |
| `schema-omits-a-leaf-the-backend-supports.md` | 2026-09-14 | `diag` capture `protocol` enum omits `bfd` |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Explicit `// enumeration: yang <module>/<leaf-path>` directive for bidirectional checking of any enum size | Lower global unmapped threshold to 2; keep hand-written `yang_enum_agreement_test.go` tests | Eliminates 2-value blind spot and one-way switch blind spot with zero false positives on unrelated code |
| Add `kmsg` to `ze-hub-conf.yang` and `bfd` to `ze-diag-cmd.yang`, delete `"receive/send"` from `config_capabilities.go` | Remove `kmsg` and `bfd` from Go; add `"receive/send"` alias to YANG | `kmsg` and `bfd` are active shipped capabilities; `"receive/send"` is a redundant dead spelling for `"send/receive"` |

## Known Limitations
- Migrating all ~55 existing `// enumeration: gated by TestX` annotations in unrelated packages to `// enumeration: yang <path>` is incremental; this spec migrates the owned reactor, `slogutil`, and `diag` sites and keeps `gated by TestX` accepted during transition.
- `software-version/mode` runtime enforcement (`require`/`refuse`) is owned by `plan/release/fixes/spec-software-version-mode-implements-two-of-four.md`.

## Review Gate

| Field | Value |
|-------|-------|
| Artifact | - |
| `./le spec review check` | not run |
| Rounds | 0 |
| Reviewer lenses used | - |

### Findings fixed

| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-6 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] Targeted tests pass, run in the main checkout by the main thread: `./le test unit pkg path ./internal/le/arch/enumeration/`, `./le test unit pkg path ./internal/component/config/`, `./le test unit pkg path ./internal/component/bgp/reactor/`, `./le test unit pkg path ./internal/plugins/diag/cmd/`, `./le test functional parse`, `./le arch enumeration check`, `./le cli grammar`, and the suites `./le repo changed scope print both` reaches. No `./le verify` command
- [ ] Every A-N confirmed or broken, none `unvalidated`

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + journal rows marked `fixed:`
- [ ] **Commit B:** `remove plan/release/fixes/spec-yang-enum-vocabulary-agreement-gate.md` only
