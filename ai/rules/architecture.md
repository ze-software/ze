# Architecture and Design

**When:** before any design decision, before writing code or a spec, or when deciding where a new package belongs
**Severity:** blocking
**Related:** plugins, performance, protocol, completion, go-standards

## Directives

**Before any design decision (communication mechanism, naming, package placement, platform backend, lifecycle), the reading named below for that artifact and that area MUST be loaded. Trained instincts about "how software works" are wrong here: ze has opinions, and `docs/contributing/ze-go-style.md` names each divergence from standard Go.**
**The "Before Writing Code" checklist MUST be completed before writing any code, tests, or documentation.**
**Before any spec: source MUST be read, current behavior MUST be documented, and existing behavior MUST be preserved by default.**
**Before modifying a file, what else the change obliges MUST be checked. A change to a YANG file, a registration file, Go source, a `.ci` test, a docs page, or a spec has a predictable ripple, and the Impact Analysis directives name it.**
**The full data flow MUST be traced before writing or reviewing a spec.**
**Where a Go package lives under `internal/` is decided by dependency direction, not by size or age: three tiers, two mechanical axes (`docs/architecture/module-tiers.md`). New code MUST land in the correct tier; an engine in the wrong tier fails `./le verify worktree`.**
**Runtime state MUST persist through the managed zefs store, never as a loose file.**

**The detail behind these directives lives in the pages below, and the relevant page SHOULD be read before the design work it covers.**

| Page | Covers |
|---|---|
| `docs/architecture/core-design.md` | System architecture, negotiated capabilities, the UPDATE container, RIB storage, plugin API communication, data flow, component boundaries |
| `docs/architecture/module-tiers.md` | The three tiers, the two placement axes, the non-engine category manifest, compile-out, and what `./le tier check` enforces |
| `docs/architecture/buffer-architecture.md` | The buffer path, pool shapes, the wire abstractions, caller-owned buffers |
| `docs/architecture/zefs-format.md` | The zefs store and runtime state through `statestore` |
| `docs/architecture/web-components.md` | Server-rendered markup and the guards that own each property |
| `docs/contributing/ze-go-style.md` | Where Ze diverges from standard Go, in each of seven areas |

## Before Writing Code

**You MUST complete this checklist before writing code.**
- [ ] 1. Read pattern cookbook (touching CLI/web/plugin/config/tests): read `ai/patterns/<domain>.md`. See `ai/INDEX.md` "I Want To..."
- [ ] 2. Grep/Glob for existing implementations and extend one when found. Search `^type Foo ` before writing a new type
- [ ] 3. Know source files: use digests if available; read + write digest if not
- [ ] 4. Verify file paths exist (Glob/Grep)
- [ ] 5. Wiring-first check: for every new feature, name the user entry point (CLI command, web route, config leaf, plugin event) and the function where it will be registered. If the entry point doesn't exist yet, it is Phase 1. If it does, name the function you will modify. "Library code someone will call" is not an answer.
- [ ] 6. Buffer-first check (wire encoding): `ai/rules/performance.md`
- [ ] 7. Lazy-first check: can the consumer use existing wire methods? "Design Principles" below, "Lazy over eager"
- [ ] 8. Bulk-edit check: >2 files with same pattern? Change ONE, test, confirm, THEN `./le source-rewrite replace` (preview before `--apply`). Never assume
- [ ] 9. Sibling call-site audit: adding a guard/fallback/retry to ONE call site? Grep ALL callers; apply same change in the same commit
- [ ] 10. Discovery update check: adding or changing a feature, tool, self-check, verification gate, or test infrastructure? Name the docs/rules/index updates now. See `ai/rules/repo-maintenance.md`

**Before any buffer/pool/allocation, you MUST trace the full lifecycle: where allocated? who holds it? when copied? when released?**
**Ze lifecycle: allocate at receive (Incoming Peer Pool), share read-only through forwarding, copy only on egress modification (Outgoing Peer Pool), release after TCP write.**
**Acquisition point defines the design: "every dispatch" vs "only on modification" are fundamentally different. A pool is not a counter. You MUST look at filter code + `buildModifiedPayload` to see WHERE modification happens before deciding WHERE buffers come from.**
**Red flags:** new file without checking for similar; function that might duplicate; can't name 3 related files.

**When you add a guard, fallback, retry, or special case to a call site of a shared function, you MUST grep every other call site in the same commit and apply the same fix where needed.**

**Each trigger below MUST be answered by the audit named beside it, in the same commit.**

| Trigger | Action |
|---------|--------|
| A `nil` check on a result | Check every other caller for the same nil case |
| A fallback when an external system is unavailable | Check every other caller of that dependency |
| A retry or backoff | Check every other caller doing the same I/O |
| A new error-wrapping context | Check every other caller wrapping the same error |
| Replacing a direct call with a helper | Check every other caller that SHOULD use the helper |
| Changing or removing how a binary is invoked | Search EVERY invocation of the bare token: `.ci` directives, embedded `tmpfs=` bodies, compiled drivers under `internal/test/fixture`, runner launch code, native actions, and docs. The complete affected suite MUST be proven, never a sample |

**For each match: same guard needed? Yes -> you MUST fix it now. No -> you MUST state in the commit message WHY this caller is exempt. Silence is bug bait.**
**Difference from bulk-edit (item 7): bulk-edit = "same change to N files I already know about" (discipline). Sibling-audit = "change to ONE file; which OTHERS need it?" (discovery).**

## Design Context

**Every design decision MUST start from this reading, whatever the artifact.**

| What | Where | Prevents |
|------|-------|----------|
| Design principles | "Design Principles" below | "Good enough" backends, translation layers, implicit behavior, a missed abstraction (abstract at two use cases) |
| Plugin architecture | `ai/rules/plugins.md` | Wrong package, import violations, wrong communication mechanism |
| Registration pattern | `ai/patterns/registration.md` | A missing `init()`, registry, or blank import |
| Existing core packages | `ls internal/core/` | Missing an existing pattern such as `internal/core/family/` |
| Module tiers | `docs/architecture/module-tiers.md` | A package in the wrong tier, which fails `./le tier check` |

**When the design produces one of these artifacts, its row MUST be read before the design is proposed.**

| Artifact | Read | Prevents |
|----------|------|----------|
| New plugin | `ai/patterns/plugin.md` | Wrong structure, missing YANG, wrong callback |
| Cross-plugin broadcast | `pkg/ze/eventbus.go`, `internal/core/events/typed.go`, and one consumer such as fibkernel | Treating EventBus as request and response when it is async pub/sub |
| Cross-plugin request and response | `pkg/plugin/rpc/bridge.go` (DirectBridge) | Reinventing DirectBridge, which already serves synchronous typed calls from core to internal plugins |
| Shared registry | `internal/core/family/` (read the code) | A registry inside a plugin instead of core |
| Config option | `ai/patterns/config-option.md` and `ai/rules/config.md` | Missing env var, wrong YANG shape, env-only where config belongs, wrong leaf name |
| CLI command | `ai/patterns/cli-command.md` | Wrong dispatch structure |
| TUI or terminal colors | `docs/architecture/cli/color-system.md` | Wrong color roles, an inconsistent palette across surfaces |
| Platform-specific code | The existing splits (`fibkernel/backend_linux.go`, `ifacenetlink/sysctl_linux.go`) | Wrong build tag, wrong abstraction level |
| A feature with a dataplane effect | `internal/plugins/iface/netlink/` and `internal/plugins/iface/vpp/` | A netlink-only feature with no VPP support |
| Naming | `ai/rules/go-standards.md`, `ai/rules/config.md`, and a grep for analogous names | Inventing a ze-name where a kernel or standard name exists, an abbreviated YANG leaf, an env var path that does not mirror YANG |

**When the design touches one of these areas, its row MUST be read too.**

| Area | Read | Prevents |
|------|------|----------|
| Plugin startup timing | `internal/component/plugin/server/startup.go` (`TopologicalTiers`, `runPluginPhase`) | Hand-waving instead of tier ordering |
| Wire encoding | `ai/rules/performance.md` | Allocations in encoding |
| Env vars | `ai/rules/go-standards.md`, `ai/rules/config.md`, `internal/core/env/` | `os.Getenv`, a missing `MustRegister`, env-only where YANG config belongs, wrong naming convention |
| JSON format | `ai/rules/cli.md` | Wrong key casing |
| Testing | `ai/rules/testing.md` and `ai/patterns/functional-test.md` | Missing `.ci` tests, wrong structure |
| Daemon lifecycle | `OnStarted` and `OnAllPluginsReady` in a similar plugin | Wrong callback, missing cleanup |

**The reasoning in the left column MUST NOT be acted on. The right column is what the design MUST do instead.**

| Anti-pattern | Instead |
|--------------|---------|
| "Docs say X supports Y" | Read the implementation. A page CAN be stale or aspirational |
| "Industry standard is X" | Grep ze for how it already does X |
| "Good enough for dev" | Do it right. Darwin CAN be production |
| "Overkill for now", comparing compile-time against runtime enforcement | Compile-time enforcement is correctness, not overkill. When the compiler CAN prevent a class of bug, that is the right option, and implementation cost is not a reason to accept a weaker guarantee |
| "A translation layer for a cleaner API" | Explicit beats implicit. Use the native names |
| "Put the registry where it is used" | Check `internal/core/` first |
| "DispatchCommand for cross-plugin calls" | EventBus for broadcast; DirectBridge for request and response |
| "A new direct-call mechanism for internal plugins" | DirectBridge already exists (`pkg/plugin/rpc/bridge.go`). Read it before proposing one |
| "No cleanup needed on stop" | Ze owns what it touches |
| "VPP support CAN be added later" | A feature with a netlink implementation MUST get its VPP implementation in the same work. Ze targets both dataplanes, and a netlink-only feature creates drift |
| "Defaults are suggestions" | Defaults are requirements, and an override MUST be logged |

**You MUST answer these questions before proposing a design:**
- 1. Did I read how ze already handles similar? (grep, not assume)
- 2. Did I check `internal/core/` for an existing shared pattern?
- 3. Did I read the relevant `ai/patterns/` file?
- 4. Does my proposal contradict "Design Principles" below?
- 5. Am I inventing a name when standard/kernel/existing exists?
- 6. Am I proposing a new communication mechanism? Read `pkg/plugin/rpc/bridge.go` first. DirectBridge likely already does it.
- 7. Am I comparing systems or claiming capabilities? Read the implementation for each system being compared. Spawn parallel agents if multiple codepaths need verification. You MUST NOT answer from docs alone.

## Design Principles

**Every design decision MUST satisfy these principles. Where a principle names a rule, that rule governs the detail.**

| Principle | Rule |
|-----------|------|
| YAGNI | Do not build what is not needed now |
| Simplest correct solution | The simplest answer that is FULLY correct, and nothing beyond it. It cuts machinery, never correctness. `ai/rules/simplicity.md` owns this and is BLOCKING |
| Simplicity | Boring code that obviously works beats clever code |
| No identity wrappers | A wrapper MUST transform: a type conversion, error wrapping, a default. A struct holding raw data plus accessors is an identity wrapper, so pass the data and use the existing type's methods |
| Single responsibility | One thing per function, struct and package. "And" in the name means split it |
| Explicit over implicit | No hidden magic, no convention-based behavior, no silent default |
| Minimize coupling | Each component knows the minimum about the others, and dependencies run high to low |
| Interface segregation | A client depends only on the methods it uses |
| Abstract when you can | Two concrete use cases justify an abstraction. Abstract at the second, do not wait for a third |
| Design for change | Isolate volatility behind a stable interface |
| Fail-mode awareness | Every external call CAN fail and every input CAN be malformed |
| Do it right | Zero-copy, pool dedup, buffer-first. Correctness MUST NOT be traded for implementation speed |
| Exact or reject | A backend or translator that cannot apply config EXACTLY MUST fail verify or commit with a clear error. No silent approximation. `ai/rules/protocol.md` |
| Durability over velocity | "Never revisit this code" beats "get to commit fast". Rework costs more than thoroughness |
| Encapsulation onion | Allocate one buffer at the outermost protocol layer and slice inward with specialised types. Peel by narrowing the window, never by copying. `docs/architecture/buffer-architecture.md` |
| Buffer-first encoding | All wire encoding goes into pooled bounded buffers through `WriteTo(buf, off) int`. `ai/rules/performance.md` |
| No `make` where pools exist | A variable-N `make([]byte, N)` on a wire-facing path MUST come from a bounded pool. `make` stays correct for a fixed-size header and a one-shot startup allocation |
| Pool strategy by goroutine shape | A single-backing ring for one sequential goroutine, a pool seeded for peak where several goroutines share. Every buffer in one pool is the SAME maximum size |
| Lazy over eager | Read side: raw byte slices plus offset iterators (`Next()`), never parsed structs or collected slices |
| Zero-copy, copy-on-modify | Allocate at receive, share read-only through forwarding, copy only when an egress filter modifies, release after send. `docs/architecture/buffer-architecture.md` |

**Code MUST pass this checklist:**
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit behavior (no hidden magic?)
- [ ] Minimal coupling
- [ ] Consistent naming
- [ ] Testable in isolation
- [ ] Next-developer test: understood in 30 seconds?

## Module Tiers (core / component / plugin)

- **A config-driven engine (one calling `sdk.NewWithConn`) at a top-level subsystem MUST live in `internal/component/` when a feature depends on it, and in `internal/plugins/` otherwise.**
- **A non-engine package outside `internal/core/` MUST either be classified by the existing registration mechanics, or carry a manifest row in `internal/le/tier/testdata/tier_non_engine_categories.txt`.**

**Each category of package MUST live in the directory named here:**
- **core library** -> `internal/core/`. It has no config-driven lifecycle, no registry side effect, and no reason to live with a component domain.
- **framework** -> usually `internal/component/`. It provides Ze's wiring substrate rather than a runnable feature: config, plugin, command, cli, doctor, hub, lifecycle, and setup-feature integration.
- **host-service** -> `internal/component/`. It is a daemon or appliance service boundary such as web, ssh, gNMI, MCP, looking-glass, host APIs, or gokrazy support. These packages are not pure core libraries because startup, doctor, listener, or platform registration pins them to composition.
- **domain-library** -> lives with the component domain it serves until that domain is split. In this spec only BNG (`l2tp`, `ppp`, `pppoe`, `pppoeclient`, `subscriber`) and VPN (`ike`, `ipsec`) are clustered. PKI stays top-level because it is shared certificate infrastructure for IPsec and future TLS users. AAA, traffic, firewall, and CoS stay flat unless a later spec proves a clean isolated cluster.
- **engine + a feature depends on it** -> **component** (`internal/component/`). It is a platform other plugins build on. BGP is the archetype: its sub-plugins and other code plug into it.
- **engine + nothing depends on it** -> **edge plugin** (`internal/plugins/`). IS-IS, OSPF, LDP, RSVP-TE are edge protocols: they consume services (iface, the RIB) but nothing consumes them. A *gated* edge engine's blank import in the generated `all_<tag>.go`, a `cmd/ze` dispatch companion, or `cmd/ze/setup_features_*.go` is a registration import, NOT a dependency, so it does not promote the engine to a component.
- The RIB stays **component** because edge protocols install routes through it.

**You MUST pick the directory from these rules:**
- 1. Pure library, no `sdk.NewWithConn`, no plugin lifecycle, no component domain owner -> `internal/core/<x>`.
- 2. Framework or host-service infrastructure -> classify it in `internal/le/tier/testdata/tier_non_engine_categories.txt` and keep it under `internal/component/<x>` unless this rule says setup-package placement belongs under `internal/plugins/<x>`.
- 3. Domain library -> keep it with the owning domain only when the manifest names the domain category. Today that means BNG and VPN; AAA, traffic, firewall, and CoS stay flat.
- 4. Engine that other plugins will depend on -> `internal/component/<x>`.
- 5. Engine that is a self-contained leaf feature -> `internal/plugins/<x>`.

**A sub-plugin of an existing subsystem (e.g. a BGP capability or NLRI codec) MUST go under that subsystem's own plugin namespace (`internal/component/bgp/plugins/<x>`), not at the top level. Those nested namespaces are listed in the generator's `pluginDirs` (`internal/le/plugin/imports/pluginimports.go`).**

- **An intentional non-engine placement outside `internal/core/` MUST have a row in `internal/le/tier/testdata/tier_non_engine_categories.txt`, which `./le tier check` reads. A new exception MUST NOT be hidden in Go code. The row format and the categories are `docs/architecture/module-tiers.md`.**

**The manifest is not a general allowlist. It classifies packages whose placement cannot be derived from the engine and registration mechanics alone. A manifest row MUST NOT point at an engine, MUST use the correct home for its category, and MUST NOT go stale; every shared non-engine placement MUST have a row.**

**Rule:** a compile-out-able feature (gated by `//go:build ze_<feature>`) MUST NOT be directly imported by always-on code. Reach it through the construction registry or a seam (`ssh_infra.go` / `gnmi_infra.go` style) in another gated file. Gates are declared in ONE place as `<tag> <pkg>` rows in `feature-gates.txt`. A feature MAY reuse one tag for sidecar packages that MUST vanish with it. `./le tier check` derives the disable-able package set and refuses every always-on, non-test importer. `./le feature-tags check` refuses drift in the generated static consumers. Full procedure and the two registration shapes: `ai/rules/plugins.md`.

## Data Flow Tracing

- **A spec's Data Flow section MUST carry the four subsection names below, spelled as `plan/TEMPLATE.md` spells them. `hookValidateSpec` in `internal/le/hookruntime/lifecycle.go` refuses the spec otherwise.**

- [ ] 1. You MUST name the entry points: where does data enter? (wire, API, config, plugin) What format?

- [ ] 2. You MUST trace each transformation stage: parse -> validate -> store -> process -> encode

- [ ] 3. You MUST name every boundary crossing: Engine <-> Plugin (JSON over pipes), FSM <-> Reactor (event types), WireUpdate <-> RIB (attribute refs), Caps <-> EncodingContext (`internal/core/bgp/context`)
- [ ] 4. You MUST check for: violations? Bypassed layers? Unintended coupling? Duplicated functionality? Broken zero-copy?

- [ ] 5. You MUST check: integration points exist? Signatures match? Unrelated code needs changes?

**A spec MUST answer these questions before approval:**
- 1. Where does data come from?
- 2. What happens at each stage?
- 3. Where does it go and in what format?
- 4. Which boundaries does it cross?
- 5. What existing code does it interact with?

## Impact Analysis

**A change to a `*.yang` file MUST also update everything its row names.**

| What changed | Also update |
|---|---|
| New leaf or container | The config parser that reads the tree (grep `GetContainer` and `GetChild` for the path) |
| New leaf or container | The validator, when validation rules apply |
| New leaf or container | CLI completion, when the command references the schema |
| Renamed path | `./le yang migration path-refactor` handles slash paths, set commands, brace blocks, and GetContainer chains |
| New `environment/` leaf | `env.MustRegister()` in the component's config loader |
| New `ze:listener` | Conflict detection through `FindListenerConflict` |
| New `ze:command` | The RPC handler, then `./le doc check verify` |

**A change to a `register.go` or an `init()` MUST also update everything its row names.**

| What changed | Also update |
|---|---|
| New plugin | `./le repository generate` (updates `all.go`), and the `TestAllPluginsRegistered` count |
| New family | `family.MustRegister()`, plus NLRI decoder and encoder registration |
| New capability | Capability codec registration |
| New event type | The `Registration.EventTypes` field |
| Renamed registered name | `ai/rules/plugins.md`, "Renaming a Registered Name", for the full grep |

**A change to a `*.go` file under `internal/` MUST also check everything its row names.**

| What changed | Also check |
|---|---|
| New exported symbol | Its wiring: who calls it (`ai/rules/completion.md`) |
| Modified function signature | Every caller (`gopls references`, or a grep) |
| New goroutine | `ai/rules/goroutine-lifecycle.md`, and cleanup on shutdown |
| New `make([]byte, N)` on a wire path | The pool-backed alternative (`ai/rules/performance.md`) |
| New `fmt.Sprintf` | The append-based alternative (`ai/rules/performance.md`) |
| A guard or fallback added | The sibling call-site audit above |
| An error return ignored | `./le verify lint run` reports the errcheck finding |

**A change to a `*.ci` functional test MUST also check everything its row names.**

| What changed | Also check |
|---|---|
| New test file | The correct directory (`ai/rules/testing.md`, test directories table) |
| Compiled observer | That the failing `internal/test/fixture` callback returns an error (`ai/rules/testing.md`) |
| Config in `tmpfs=` | That a parse test validates its syntax |

**When changing code, you MUST check `ai/CODE-TO-DOCS.md` for docs that reference the file. You MUST update any claims that are now wrong. Regenerate: `./le docs-to-code index-update`.**

**A change under `docs/` MUST also check everything its row names.**

| What changed | Also check |
|---|---|
| New factual claim | Its source anchor: `<!-- source: path -- symbol -->` |
| A feature count or list | `./le doc check verify` validates it against the live registry |
| Changed config syntax | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` |

**A change to a spec under `plan/` MUST also update everything its row names.**

| What changed | Also update |
|---|---|
| Status change | The per-session marker, through `./le spec session` |
| An AC added or removed | The wiring test table and the audit table |
| A design decision | Annotate it with `-> Decision:` for post-compaction recovery |

## zefs Persistence (no loose state files)

- **Do:** you MUST use `statestore.Put(key, data)` / `statestore.Get(key)` (package `internal/core/statestore`), keyed by a registered `pkg/zefs` key (`meta/<subsystem>/<name>` in `pkg/zefs/keys.go`).
- **Don't:** you MUST NOT use `os.WriteFile` / `os.Create` / `os.OpenFile(..., O_CREATE...)` / `os.Rename` a state blob into a path under the config/state dir.

- You MUST register the key in `pkg/zefs/keys.go` (`meta/<subsystem>/<name>`; use a `{placeholder}` for per-entity keys and `Key(param)` to fill it).
- `statestore` is **best-effort**: `Put` is a no-op when no blob store is registered (filesystem-fallback mode). Persistence MUST stay non-fatal.
- **One shared instance, not a transient open.** The config system opens `database.zefs` once at startup and holds that single `*zefs.BlobStore` for the process; a flush re-encodes the whole file from its in-memory tree. Writing state through a SEPARATE transient store would let the config store's next flush drop every state key (and a state write could revert a concurrent config commit). So `statestore` MUST write through that same handle (registered with `SetStore` in `cmd/ze/hub`), serialized by the store's own lock: one tree, no lost updates. A write still rewrites the whole store per flush, so cadences MUST stay modest (best-effort caches, not per-packet).

**Code in these categories MAY keep raw writes, on the allowlist:**
- **Kernel/device control:** `/proc`, `/sys`, sysfs, `/dev`, cgroup, ethtool.
- **Ephemeral scratch:** `/tmp`, `/run`, pid files, sockets, probe/ready files.
- **External artifacts:** files produced for another consumer: `resolv.conf`, systemd units, PEM exports, MRT dumps, the ze binary during self-update, the externally-written `config-pushed.conf` inbox.
- **The storage layer itself:** `internal/component/config/storage`, `pkg/zefs`, and crash-time writers (`internal/core/crashlog`) that MUST survive a broken zefs. The append-only audit log (`internal/core/audit`) also stays a tailable file (a blob KV store is the wrong shape for an append log).

- **A new legitimate non-state writer MUST carry an allowlist entry with its reason, and genuine state MUST move to `statestore`. `./le fs-persistence check`, inside `./le verify worktree` and `./le verify current mode changed`, refuses a non-allowlisted raw filesystem write.**

## Server-Rendered Markup

- **Markup MUST live in a `.templ` file. A Go string literal MUST NOT build an HTML or SVG tag in `internal/component/web` or `internal/component/lg`.**
- **A templ component MUST take a named struct. A `map[string]any` MUST NOT reach one, and a struct field wrapping one MUST NOT either.**
- **A page MUST NOT carry an inline script, an inline style attribute, or an inline event handler. Both packages answer `'self'` for script, so a browser refuses an inline script and an inline handler and tells the server nothing. The rule covers the style attribute too, so both packages hold one rule and a header CAN be tightened without a hunt.**
- **Behavior a page needs MUST reach it as a data attribute an external asset reads. That asset MUST exist in the embedded filesystem the handler serves.**
- **A new exemption MUST carry its reason and MUST raise the exact count beside it. Each guard fixes the size of its table, so widening one is an edit a reader sees.**
- **A gate that names one package MUST NOT be treated as covering its sibling. Each guard walks its own directory, and `lg` shipped two dead handlers under the web package's green.**

## Architecture Summary

**Parsing MUST treat the same wire bytes differently, based on caps. Code MUST use ContextID to identify the encoding context for zero-copy.**

**Every wire-encodable type MUST implement `wire.BufWriter`: `WriteTo(buf []byte, off int) int`. A type that also validates capacity, or that a caller needs a length from, MUST implement `wire.CheckedBufWriter`, which adds `CheckedWriteTo(buf, off) (int, error)` and `Len() int`.**
**A context-dependent attribute MUST take the source and destination `*bgpctx.EncodingContext`, through `WriteToWithContext(buf, off, srcCtx, dstCtx) int`, so ADD-PATH and ASN4 encode per peer.**

- WireUpdate MUST transport data only (lazy parse via iterators, keeps wire refs).
- RIB MUST store NLRI -> attribute refs into per-type pools, and MUST NOT store WireUpdate refs.
- Code MUST use per-attribute-type pools with dedup, and per-family NLRI pools.

- **Peer Pools** (64 buffers per peer, negotiated size): each peer has an Incoming Peer Pool (inbound) and an Outgoing Peer Pool (outbound modification). Encoding code MUST take a buffer from the peer pool matching the direction. Both pools use the same Peer Pool type, sized at init.
- **Global Shared Pool**: byte-budgeted overflow, mixed 4K/64K blocks. Auto-sized from peer prefix maximums via `overflowPoolBudget()`. Code MUST treat pool exhaustion as the backpressure signal.

**Unbounded event buffer: events MUST NOT be dropped. Ring buffer rejected because losing route events breaks convergence counts.**

## Related

**You MAY read more about placement here:**
- `docs/architecture/module-tiers.md`: the tiers, the two axes, the category manifest, compile-out, and what the gate enforces.
- `ai/rules/plugins.md`: the delete-the-folder invariant, registration patterns, the Proximity Principle.
- `internal/le/tier/tier.go`: the reverse-dependency report and the placement gate.
