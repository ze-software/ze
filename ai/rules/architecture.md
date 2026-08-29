# Architecture and Design

**When:** before any design decision, before writing code or a spec, or when deciding where a new package belongs
**Severity:** blocking
**Related:** plugins, performance, protocol, completion, go-standards

## Directives

Rationale and examples: `ai/rationale/design-principles.md`, `ai/rationale/before-writing-code.md`, `ai/rationale/data-flow-tracing.md`, `ai/rationale/architecture-summary.md`.
See also, for the pool/buffer/lazy principles: `ai/rules/performance.md`, `ai/rules/protocol.md`. Full architecture details: `docs/architecture/core-design.md`.

**Before any design decision (communication mechanism, naming, package placement, platform backend, lifecycle), load the relevant context below. Trained instincts about "how software works" are wrong here: ze has opinions.**
**Complete the "Before Writing Code" checklist before writing any code, tests, or documentation.**
**Before any spec: READ source, document current behavior, preserve by default.**
**Before modifying a file, check what else needs to change. Changes to certain file types have predictable ripple effects.**
**Trace full data flow before writing or reviewing specs.**
**Where a Go package lives under `internal/` is decided by dependency direction, not by size or age. Three tiers, two mechanical axes. New code MUST land in the correct tier; an engine in the wrong tier fails `./le verify worktree`.**
**Persist runtime state through the managed zefs store, never as a loose file.**
**Ze differs from typical Go projects in specific, load-bearing ways. An AI trained on standard Go patterns will default to the wrong approach unless it reads the divergence tables below. Each entry names the standard approach, the Ze approach, the rule that governs it, and a one-line reason.**

## Before Writing Code

Complete before writing any code, tests, or documentation.

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

### Memory Lifecycle Tracing

**Before any buffer/pool/allocation, you MUST trace the full lifecycle: where allocated? who holds it? when copied? when released?**
**Ze lifecycle: allocate at receive (Incoming Peer Pool), share read-only through forwarding, copy only on egress modification (Outgoing Peer Pool), release after TCP write.**
**Acquisition point defines the design: "every dispatch" vs "only on modification" are fundamentally different. A pool is not a counter. You MUST look at filter code + `buildModifiedPayload` to see WHERE modification happens before deciding WHERE buffers come from.**
**Red flags:** new file without checking for similar; function that might duplicate; can't name 3 related files.

### Sibling Call-Site Audit

**When you add a guard, fallback, retry, or special case to a call site of a shared function, you MUST grep every other call site in the same commit and apply the same fix where needed.**

| Trigger | Action |
|---------|--------|
| `nil` check on a result | Check every other caller for the same nil case |
| Fallback when external system unavailable | Check every other caller of the dependency |
| Retry / backoff | Check every other caller doing the same I/O |
| New error-wrapping context | Check every other caller wrapping the same error |
| Replace direct call with helper | Check every other caller that should use the helper |
| Change or remove how a binary is invoked | Search EVERY invocation of the bare token, including `.ci` directives, embedded `tmpfs=` bodies, compiled drivers under `internal/test/fixture`, runner launch code, native actions, and docs. Prove the complete affected suite, never a sample (learned 1248) |

```
fn="store.ReadFile"
grep -rn "$fn" internal/ pkg/ cmd/ --include="*.go"
```

**For each match: same guard needed? Yes -> you MUST fix it now. No -> you MUST state in the commit message WHY this caller is exempt. Silence is bug bait.**
**Difference from bulk-edit (item 7): bulk-edit = "same change to N files I already know about" (discipline). Sibling-audit = "change to ONE file; which OTHERS need it?" (discovery).**

## Design Context

Before any design decision (communication mechanism, naming, package placement, platform backend, lifecycle), load the relevant context below.

### Tier 1: Always Read Before Any Design

| What | Where | Prevents |
|------|-------|----------|
| Design principles | "Design Principles" below | "Good enough" backends, translation layers, implicit behavior, missed abstractions (abstract at 2+ use cases) |
| Plugin architecture | `ai/rules/plugins.md` | Wrong package, import violations, wrong comm mechanism |
| Registration pattern | `ai/patterns/registration.md` | Missing init + registry + blank import pattern |
| Existing core packages | `ls internal/core/` | Missing patterns like `internal/core/family/` |

### Tier 2: When Designing a Specific Artifact

| Artifact | Read | Prevents |
|----------|------|----------|
| New plugin | `ai/patterns/plugin.md` | Wrong structure, missing YANG, wrong callback |
| Cross-plugin comm (broadcast) | `pkg/ze/eventbus.go` + `internal/core/events/typed.go` + one consumer (e.g. fibkernel) | EventBus is for async pub/sub notifications, not request/response |
| Cross-plugin comm (request/response) | `pkg/plugin/rpc/bridge.go` (DirectBridge) + `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" (294, retired) | DirectBridge for sync typed calls from core to internal plugins. Do not reinvent this. |
| Shared registry | `internal/core/family/` (read the code) | Registry inside a plugin instead of core |
| Config option | `ai/patterns/config-option.md` + `ai/rules/config.md` + `ai/rules/config.md` + `ai/rules/config.md` | Missing env var, wrong YANG shape, env-only when should be config, wrong leaf name |
| CLI command | `ai/patterns/cli-command.md` | Wrong dispatch structure |
| TUI / terminal colors | `docs/architecture/cli/color-system.md` | Wrong color roles, inconsistent palette across surfaces |
| Platform-specific | Existing splits (`fibkernel/backend_linux.go`, `ifacenetlink/sysctl_linux.go`) | Wrong build tag, wrong abstraction level |
| New feature with dataplane effect | `internal/plugins/iface/netlink/` + `internal/plugins/iface/vpp/` | Netlink-only feature without VPP support |
| Naming | `ai/rules/go-standards.md` + `ai/rules/config.md` (config/env) + grep analogous names | Inventing ze-names when kernel/standard names exist, abbreviated YANG leaves, env var path not mirroring YANG |

### Tier 3: When the Design Touches These Areas

| Area | Read | Prevents |
|------|------|----------|
| Plugin startup timing | `internal/component/plugin/server/startup.go` (`TopologicalTiers`, `runPluginPhase`) | Hand-waving instead of tier ordering |
| Wire encoding | `ai/rules/performance.md` | Allocations in encoding |
| Env vars | `ai/rules/go-standards.md` + `ai/rules/config.md` + `ai/rules/config.md` + `internal/core/env/` | `os.Getenv`, missing `MustRegister`, env-only when should be YANG config, wrong naming convention |
| JSON format | `ai/rules/cli.md` | Wrong key casing |
| Testing | `ai/rules/testing.md` + `ai/patterns/functional-test.md` | Missing .ci tests, wrong structure |
| Daemon lifecycle | `OnStarted`/`OnAllPluginsReady` in a similar plugin | Wrong callback, missing cleanup |

### BGP Domain Facts (Do Not Assume From Training Data)

| Fact | Why it matters |
|------|---------------|
| NEXT_HOP is set at the eBGP border router; all IBGP routes share a small set of next-hops (the L3 device originating the prefix or the eBGP peer) | Attribute byte overlap across IBGP peers is high, not low |
| MED, LOCAL_PREF, communities are policy-set by the sender and tend to be identical across many routes from the same peer | Same-peer attribute reuse is very high |
| AS_PATH is identical for all routes learned from the same external source; IBGP does not prepend | Cross-peer attribute overlap within an AS is significant |
| BGP UPDATE packing groups NLRIs with identical attributes into one message, but convergence events and incremental announcements spread them across multiple UPDATEs | Attribute reuse across UPDATEs from a single peer is common |

### Anti-Patterns

| Anti-pattern | Instead |
|--------------|---------|
| "Docs say X supports Y" | Read the implementation. Docs may be stale or aspirational |
| "Industry standard is X" | Grep ze for how it already does X |
| "Good enough for dev" | "Do it right." Darwin could be prod |
| "Overkill for now" when comparing compile-time vs runtime enforcement | Compile-time enforcement is not overkill, it is correctness. If the compiler can prevent a class of bugs, that is the right option. Implementation cost is not a reason to accept weaker guarantees. |
| "Translation layer for cleaner API" | "Explicit > implicit." Use native names |
| "Put the registry where it's used" | Check `internal/core/` first |
| "DispatchCommand for cross-plugin calls" | EventBus for broadcast; DirectBridge for request/response |
| "New direct-call mechanism for internal plugins" | DirectBridge already exists (`pkg/plugin/rpc/bridge.go`). Read it before proposing. |
| "No cleanup needed on stop" | Ze owns what it touches |
| "VPP support can be added later" | If the feature has a netlink implementation, add the VPP implementation in the same work. Ze targets both dataplanes; netlink-only features create drift. |
| "Defaults are suggestions" | Defaults are requirements; log when overridden |

### Mechanical Check

**You MUST answer these questions before proposing a design:**
- 1. Did I read how ze already handles similar? (grep, not assume)
- 2. Did I check `internal/core/` for an existing shared pattern?
- 3. Did I read the relevant `ai/patterns/` file?
- 4. Does my proposal contradict "Design Principles" below?
- 5. Am I inventing a name when standard/kernel/existing exists?
- 6. Am I proposing a new communication mechanism? Read `pkg/plugin/rpc/bridge.go` first. DirectBridge likely already does it.
- 7. Am I comparing systems or claiming capabilities? Read the implementation for each system being compared. Spawn parallel agents if multiple codepaths need verification. You MUST NOT answer from docs alone.

## Design Principles

| Principle | Rule |
|-----------|------|
| YAGNI | Don't build what's not immediately needed |
| Simplest correct solution | The simplest answer that is FULLY correct, and nothing beyond it. It cuts machinery, never correctness. `ai/rules/simplicity.md` owns this and is BLOCKING |
| Simplicity | Boring code that obviously works > clever code |
| No identity wrappers | Wrapper must transform (type conversion, error wrapping, defaults). A struct holding raw data + accessor methods is an identity wrapper, pass the data, use existing type methods |
| Single responsibility | One thing per function/struct/package. "And" in name = split |
| Explicit > implicit | No hidden magic, convention-based behavior, silent defaults |
| Minimize coupling | Components know the minimum about each other. High->low dependency |
| Interface segregation | Clients depend only on methods they use |
| Abstract when you can | Two concrete use cases justify an abstraction. Abstract at the second use case; don't wait for a third |
| Design for change | Isolate volatility behind stable interfaces |
| Fail-mode awareness | Every external call can fail; every input can be malformed |
| Do it right | Zero-copy, pool dedup, buffer-first. Never trade correctness for implementation speed |
| Exact or reject | Backend/translator cannot apply config EXACTLY -> verify/commit fails with a clear error. No silent approximation. `ai/rules/protocol.md` |
| Durability over velocity | "Never revisit this code" > "get to commit fast". Rework wastes more time than thoroughness |
| Encapsulation onion | Allocate one buffer at the outermost protocol layer; slice inward with specialised types (`WireUpdate`, `PackContext`). Peel by narrowing the window, never by copying |
| Buffer-first encoding | Write side: all wire encoding into pooled, bounded buffers via `WriteTo(buf, off) int`. No `append`, `make`, or `buildFoo() []byte` in helpers. `ai/rules/performance.md` |
| No `make` where pools exist | Variable-N `make([]byte, N)` on a wire-facing path comes from a pre-allocated, bounded pool. `make` is OK for fixed-size headers and one-shot startup allocations |
| Pool strategy by goroutine shape | Single-backing ring (single reactor goroutine, sequential) OR `sync.Pool` seeded for peak (multiple concurrent goroutines). All buffers in a pool are the SAME MAX size |
| Lazy over eager | Read side: raw byte slices + offset iterators (`Next()`), not parsed structs or collected slices. Consumer acts on data directly. N->0-until-needed, not N->1 |
| Zero-copy, copy-on-modify | Allocate at receive (Incoming Peer Pool); share read-only through forwarding; copy only when egress filters modify (Outgoing Peer Pool); release after send. Global Shared Pool handles overflow |

### Scalability Checklist

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

This generalizes `ai/rules/plugins.md` (the "delete the folder" test) into a placement rule that code can audit.

### The Three Tiers

| Tier | Home | What it is | Examples |
|------|------|------------|----------|
| **core / infra** | `internal/core/` | A library you cannot "run as a plugin." Foundational; no config-driven lifecycle. | family, events, metrics, diagnostic, bufpool |
| **component** | `internal/component/` | A platform plugin: other plugins/components depend on it or plug into it. | bgp, iface, firewall, traffic, vpp |
| **plugin (edge)** | `internal/plugins/` | An edge plugin: a config-driven engine that nothing else depends on. | ntp, static, dhcpserver, l2tp-auth-* |

### The Two Axes

| Axis | Mechanical test |
|------|-----------------|
| **A. Is it a config-driven engine?** | does it call `sdk.NewWithConn(`? |
| **B. Does a feature depend on it?** | does any `.go` file under `internal/component/` or `internal/plugins/` (excluding its own subtree, the generated composition root, `cmd/ze` dispatch, `internal/core`, `internal/chaos`, `internal/test`, and `_test.go`) import it? |

Decision:

**Each category of package MUST live in the directory named here:**
- **core library** -> `internal/core/`. It has no config-driven lifecycle, no registry side effect, and no reason to live with a component domain.
- **framework** -> usually `internal/component/`. It provides Ze's wiring substrate rather than a runnable feature: config, plugin, command, cli, doctor, hub, lifecycle, and setup-feature integration.
- **host-service** -> `internal/component/`. It is a daemon or appliance service boundary such as web, ssh, gNMI, MCP, looking-glass, host APIs, or gokrazy support. These packages are not pure core libraries because startup, doctor, listener, or platform registration pins them to composition.
- **domain-library** -> lives with the component domain it serves until that domain is split. In this spec only BNG (`l2tp`, `ppp`, `pppoe`, `pppoeclient`, `subscriber`) and VPN (`ike`, `ipsec`) are clustered. PKI stays top-level because it is shared certificate infrastructure for IPsec and future TLS users. AAA, traffic, firewall, and CoS stay flat unless a later spec proves a clean isolated cluster.
- **engine + a feature depends on it** -> **component** (`internal/component/`). It is a platform other plugins build on. BGP is the archetype: its sub-plugins and other code plug into it.
- **engine + nothing depends on it** -> **edge plugin** (`internal/plugins/`). IS-IS, OSPF, LDP, RSVP-TE are edge protocols: they consume services (iface, the RIB) but nothing consumes them. A *gated* edge engine's blank import in the generated `all_<tag>.go`, a `cmd/ze` dispatch companion, or `cmd/ze/setup_features_*.go` is a registration import, NOT a dependency, so it does not promote the engine to a component.
- The RIB stays **component** because edge protocols install routes through it.

### Non-engine category manifest

The source of truth for intentional non-engine placements outside `internal/core/` is `internal/le/tier/testdata/tier_non_engine_categories.txt`. It is non-code data consumed by `./le tier check`; do not hide new exceptions in Go code.

Each row is:

```text
<repo-relative package dir> <category> <rationale>
```

Allowed categories:

| Category | Meaning | Allowed home |
|----------|---------|--------------|
| `framework` | Wiring substrate or setup feature that exists to register, configure, command, audit, or orchestrate other packages. | `internal/component/` or setup packages under `internal/plugins/` |
| `host-service` | Listener, appliance, host API, or platform service pinned to composition by startup or doctor/platform registration. | `internal/component/` |
| `domain-library` | Non-engine package that belongs to a real domain cluster. In this spec that means BNG and VPN only. | `internal/component/` |
| `planned-violation` | Existing known placement that is scheduled to move or disappear. New rows need a spec reference in the rationale. | `internal/component/` or `internal/plugins/` |

**The manifest is not a general allowlist. It classifies packages whose placement cannot be derived from the engine and registration mechanics alone. A manifest row MUST NOT point at an engine, MUST use the correct home for its category, and MUST NOT go stale; every shared non-engine placement MUST have a row.**

### Authoring rule (read before creating a package)

Decide the tier by the two axes and the non-engine categories BEFORE you pick a directory:

**You MUST pick the directory from these rules:**
- 1. Pure library, no `sdk.NewWithConn`, no plugin lifecycle, no component domain owner -> `internal/core/<x>`.
- 2. Framework or host-service infrastructure -> classify it in `internal/le/tier/testdata/tier_non_engine_categories.txt` and keep it under `internal/component/<x>` unless this rule says setup-package placement belongs under `internal/plugins/<x>`.
- 3. Domain library -> keep it with the owning domain only when the manifest names the domain category. Today that means BNG and VPN; AAA, traffic, firewall, and CoS stay flat.
- 4. Engine that other plugins will depend on -> `internal/component/<x>`.
- 5. Engine that is a self-contained leaf feature -> `internal/plugins/<x>`.

**A sub-plugin of an existing subsystem (e.g. a BGP capability or NLRI codec) MUST go under that subsystem's own plugin namespace (`internal/component/bgp/plugins/<x>`), not at the top level. Those nested namespaces are listed in the generator's `pluginDirs` (`internal/le/plugin/imports/pluginimports.go`).**

### Scope of enforcement

The gate enforces engine placement mechanically and enforces ambiguous non-engine placements through the manifest:

> A config-driven engine (`sdk.NewWithConn`) at a top-level subsystem MUST be in
> `internal/component/` if a feature depends on it, else in `internal/plugins/`.
>
> A non-engine package outside `internal/core/` MUST either be classified by the
> existing registration mechanics or have a manifest row in
> `internal/le/tier/testdata/tier_non_engine_categories.txt`.

The "wired as a plugin" signal is mechanical: the advisory reads composition roots (generated `all.go`, gated `all_<tag>.go`, `cmd/ze` dispatch companions, and `cmd/ze/setup_features_*.go`) to tell registered packages from genuine core candidates. It catches every shape: `registry.Register`, `RegisterRPCs`, `RegisterBackend`, doctor checks, `*-cmd` verb providers, and setup-feature commands. BGP codec/type packages are being split separately; `ike/dataplane` stays under component until its VPP backend is split from the interface package. There is **no permanent allowlist**.

`./le tier check` enforces engine placement, the non-engine manifest, core import direction, disable-ability, and build-tag drift. Grandfathered pairs remain non-code data in `internal/le/tier/testdata/core_import_baseline.txt`; new pairs and stale rows both fail.

**`dep-audit` MUST behave as follows:**
- parses `pluginDirs` from `internal/le/plugin/imports/pluginimports.go` to exclude nested sub-plugin namespaces (so `bgp/plugins/*` are never flagged);
- treats generated `all.go` files, gated `all_<tag>.go` files, `cmd/ze` dispatch/import companions, and `cmd/ze/setup_features_*.go` as registration importers, not functional dependencies;
- fails (exit 2) on any **new** misplaced engine, naming the dir and its required tier, pointing here;
- fails on a **stale** engine baseline entry (one no longer misplaced), forcing cleanup;
- fails (exit 2) on any illegal, stale, or missing row in `internal/le/tier/testdata/tier_non_engine_categories.txt`;
- fails (exit 2) if a `DISABLEABLE` feature is imported by always-on (untagged, non-test) code, naming the file and the build tag it needs.

### Disable-ability (compile-out)

Axis B also decides whether a feature can be **compiled out** of the binary. A feature is compile-out-able exactly when nothing always-compiled depends on it: it is reached ONLY through build-tag-gated registration. A direct functional `import` from always-on (untagged) code pins the package into every binary and defeats the compile-out; only a blank/gated registration import can be dropped by a build tag.

Two construction shapes keep compile-out features out of always-on code. **Listener services** such as looking-glass, web, and MCP register factories in `cmd/ze/hub/service_registry.go`; dedicated seams such as `ssh_infra.go`, `gnmi_infra.go`, `api_infra.go`, and the core metrics hook carry inputs that do not fit that registry. Each gated service keeps its direct package and YANG imports behind the matching `ze_<feature>` build tag. `feature-gates.txt` is the source of truth, `./le feature-tags write` updates static consumers, and `./le feature-tags check` refuses drift.

**Rule:** a compile-out-able feature (gated by `//go:build ze_<feature>`) MUST NOT be directly imported by always-on code. Reach it through the construction registry or a seam (`ssh_infra.go` / `gnmi_infra.go` style) in another gated file. Gates are declared in ONE place as `<tag> <pkg>` rows in `feature-gates.txt`. A feature MAY reuse one tag for sidecar packages that MUST vanish with it. `./le tier check` derives the disable-able package set and refuses every always-on, non-test importer. `./le feature-tags check` refuses drift in the generated static consumers. Full procedure and the two registration shapes: `ai/rules/plugins.md`.

### Migration baseline (transitional, NOT an allowlist)

`internal/le/tier/testdata/tier_migration_baseline.txt` is non-code data listing engines scheduled to move. The native tier gate fails on new violations and stale entries, so the file can only shrink. An empty baseline means zero exceptions.

## Data Flow Tracing

The four subsection names below are required in a spec's Data Flow section (`plan/TEMPLATE.md`, checked by `hookValidateSpec` in `internal/le/hookruntime/lifecycle.go`).

### Entry Point

- [ ] 1. You MUST name the entry points: where does data enter? (wire, API, config, plugin) What format?

### Transformation Path

- [ ] 2. You MUST trace each transformation stage: parse -> validate -> store -> process -> encode

### Boundaries Crossed

- [ ] 3. You MUST name every boundary crossing: Engine <-> Plugin (JSON over pipes), FSM <-> Reactor (event types), WireUpdate <-> RIB (attribute refs), Caps <-> PackContext (encoding context)
- [ ] 4. You MUST check for: violations? Bypassed layers? Unintended coupling? Duplicated functionality? Broken zero-copy?

### Integration Points

- [ ] 5. You MUST check: integration points exist? Signatures match? Unrelated code needs changes?

### Reference Flows

- **Wire -> RIB:** TCP -> message parse -> UPDATE (WireUpdate, lazy iterator) -> attribute extraction -> pool dedup -> RIB entry (NLRI -> attr refs)
- **API -> Wire:** command parse -> attribute building -> WireUpdate -> PackContext -> wire bytes
- **Plugin <-> Engine:** event -> JSON encode -> write stdin -> plugin processes -> write stdout command -> engine parses -> execute

### Must Answer Before Approving Spec

**A spec MUST answer these questions before approval:**
- 1. Where does data come from?
- 2. What happens at each stage?
- 3. Where does it go and in what format?
- 4. Which boundaries does it cross?
- 5. What existing code does it interact with?

## Impact Analysis

### By File Type

#### YANG Schema (`*.yang`)

| What changed | Also update |
|---|---|
| New leaf/container | Config parser that reads the tree (grep `GetContainer`, `GetChild` for the path) |
| New leaf/container | Validator if validation rules apply |
| New leaf/container | CLI completion if the command references the schema |
| Renamed path | `./le yang migration path-refactor` handles slash paths, set commands, brace blocks, GetContainer chains |
| New `environment/` leaf | `env.MustRegister()` in the component's config loader |
| New `ze:listener` | Conflict detection via `FindListenerConflict` |
| New `ze:command` | RPC handler + `./le doc check verify` |

#### Registration (`register.go`, `init()`)

| What changed | Also update |
|---|---|
| New plugin | `./le repository generate` (updates `all.go`), `TestAllPluginsRegistered` count |
| New family | `family.MustRegister()`, NLRI decoder/encoder registration |
| New capability | Capability codec registration |
| New event type | `Registration.EventTypes` field |
| Renamed name | See `ai/rules/plugins.md` "Renaming a Registered Name" for full grep |

#### Go Source (`*.go` under `internal/`)

| What changed | Also check |
|---|---|
| New exported symbol | Wiring: who calls it? (`ai/rules/completion.md`) |
| Modified function signature | All callers (LSP findReferences or grep) |
| New goroutine | `ai/rules/goroutine-lifecycle.md`, cleanup on shutdown |
| New `make([]byte, N)` on wire path | Pool-backed alternative (`ai/rules/performance.md`) |
| New `fmt.Sprintf` | Append-based alternative (`ai/rules/performance.md`) |
| Guard/fallback added | Sibling call-site audit ("Sibling Call-Site Audit" above) |
| Error return ignored | `./le verify lint run` reports the errcheck finding |

#### Functional Test (`*.ci`)

| What changed | Also check |
|---|---|
| New test file | Correct directory (`ai/rules/testing.md` test directories table) |
| Compiled observer | Return an error from the failing `internal/test/fixture` callback (`ai/rules/testing.md` compiled observer section) |
| Config in `tmpfs=` | Parse test validates syntax |

#### Go Source to Documentation

**When changing code, you MUST check `ai/CODE-TO-DOCS.md` for docs that reference the file. You MUST update any claims that are now wrong. Regenerate: `./le docs-to-code index-update`.**

#### Documentation (`docs/`)

| What changed | Also check |
|---|---|
| New factual claim | Source anchor: `<!-- source: path -- symbol -->` |
| Feature count/list | `./le doc check verify` validates against live registry |
| Changed config syntax | `docs/guide/configuration.md` and `docs/architecture/config/syntax.md` |

#### Spec (`plan/spec-*.md`)

| What changed | Also check |
|---|---|
| Status change | per-session marker via `./le spec session` |
| AC added/removed | Wiring test table, audit table |
| Design decision | Annotate with `-> Decision:` for post-compaction recovery |

### Quick Grep Patterns

```bash
# Who calls this function?
grep -rn "FunctionName" internal/ cmd/ --include="*.go" | grep -v "_test.go"

# Who reads this YANG path?
grep -rn "path/to/leaf" internal/ --include="*.go"

# Who references this registered name?
grep -rn "plugin-name" internal/ pkg/ cmd/ test/ docs/ plan/ .claude/

# Who imports this package?
grep -rn "github.com/ze-software/ze/internal/component/foo" internal/ cmd/ --include="*.go"
```

## zefs Persistence (no loose state files)

### The rule

Persist runtime state through the managed zefs store, never as a loose file.

- **Do:** you MUST use `statestore.Put(key, data)` / `statestore.Get(key)` (package `internal/core/statestore`), keyed by a registered `pkg/zefs` key (`meta/<subsystem>/<name>` in `pkg/zefs/keys.go`).
- **Don't:** you MUST NOT use `os.WriteFile` / `os.Create` / `os.OpenFile(..., O_CREATE...)` / `os.Rename` a state blob into a path under the config/state dir.

### Why

On the gokrazy appliance the writable `/perm` partition holds exactly one managed artifact: `database.zefs`. It is integrity-checked (`pkg/zefs` check/repair), seeded at install, and understood by the image build/verify tooling. A loose `state/foo.json` next to it is invisible to all of that: it is not backed up, not verified, and silently gone after a reimage. `resolve.Storage()` already resolves zefs-on-appliance / filesystem-fallback-on-dev; `statestore` is the plugin-facing equivalent: it writes through the config system's OWN blob-store handle (registered once at daemon startup via `statestore.SetStore`), so state and config share one in-memory tree.

### How

```go
// save (best-effort; no-op when no blob store is registered)
data, _ := json.Marshal(snapshot)
_, _ = statestore.Put(zefs.KeyDDoSDetectBaseline.Pattern, data)

// restore
if data, ok := statestore.Get(zefs.KeyDDoSDetectBaseline.Pattern); ok {
    _ = json.Unmarshal(data, &snapshot) // keep version/sanity guards
}
```

- You MUST register the key in `pkg/zefs/keys.go` (`meta/<subsystem>/<name>`; use a `{placeholder}` for per-entity keys and `Key(param)` to fill it).
- `statestore` is **best-effort**: `Put` is a no-op when no blob store is registered (filesystem-fallback mode). Persistence MUST stay non-fatal.
- **One shared instance, not a transient open.** The config system opens `database.zefs` once at startup and holds that single `*zefs.BlobStore` for the process; a flush re-encodes the whole file from its in-memory tree. Writing state through a SEPARATE transient store would let the config store's next flush drop every state key (and a state write could revert a concurrent config commit). So `statestore` MUST write through that same handle (registered with `SetStore` in `cmd/ze/hub`), serialized by the store's own lock: one tree, no lost updates. A write still rewrites the whole store per flush, so cadences MUST stay modest (best-effort caches, not per-packet).

### Legitimate raw filesystem writes (allowlisted)

Not every `os.WriteFile` is state. These stay raw and are allowlisted in the guard with a reason:

**Code in these categories MAY keep raw writes, on the allowlist:**
- **Kernel/device control:** `/proc`, `/sys`, sysfs, `/dev`, cgroup, ethtool.
- **Ephemeral scratch:** `/tmp`, `/run`, pid files, sockets, probe/ready files.
- **External artifacts:** files produced for another consumer: `resolv.conf`, systemd units, PEM exports, MRT dumps, the ze binary during self-update, the externally-written `config-pushed.conf` inbox.
- **The storage layer itself:** `internal/component/config/storage`, `pkg/zefs`, and crash-time writers (`internal/core/crashlog`) that MUST survive a broken zefs. The append-only audit log (`internal/core/audit`) also stays a tailable file (a blob KV store is the wrong shape for an append log).

### Gate

`./le fs-persistence check` (in `./le verify worktree` / `./le verify current mode changed`) runs `internal/le/fspersistence/fspersistence.go`: it flags any non-allowlisted raw filesystem write in the scanned trees. A new legitimate non-state writer needs an allowlist entry (with a reason); genuine state must move to `statestore`.

## Ze Divergences from Standard Go

### Encoding / Wire

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `func (t T) Marshal() ([]byte, error)` | `func (t T) WriteTo(buf []byte, off int) int` | `ai/rules/performance.md` | Zero alloc on hot path; caller owns the buffer |
| `bytes.Buffer` / `append` in helpers | Pre-allocated pooled buffers, slice inward | `ai/rules/performance.md` | Bounded memory, no GC pressure |
| `make([]byte, n)` for variable-length wire data | Pool-backed buffers of fixed MAX size | `ai/rules/performance.md` | Pool strategy by goroutine shape |
| Helper allocates its own scratch | Caller passes buffer down, callee writes into it | `ai/rules/performance.md` | One allocation at outermost scope, not N in sub-functions |
| `sync.Pool` only for "reuse" | `sync.Pool` is the default for multi-goroutine scratch, ring buffer for single-goroutine | `ai/rules/performance.md` | Pool shape must match goroutine shape |
| Parse into structs eagerly | Lazy iterators over raw byte slices (`Next()`) | "Design Principles" above (Lazy over eager) | N->0-until-needed, not N->1 |
| `fmt.Sprintf` for formatting | `textbuf.Buffer` (128B stack-inline) or `strconv.Append*` | `ai/rules/performance.md` | Sprintf allocates 2-3x; textbuf allocates once |
| `strings.Join(parts, " ")` | Single `textbuf.Buffer` with `.Byte(' ')` separators | `ai/rules/performance.md` | Eliminates intermediate `[]string` + final join |

### Architecture / Registration

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Direct imports between packages | `init()` + registry + blank import | `ai/patterns/registration.md` | Small core discovers components; never imports directly |
| Constructor injection | Registry lookup at runtime (`registry.NLRIDecoder(family)`) | `ai/rules/plugins.md` | Plugins are independently removable via blank import |
| `os.Getenv("FOO")` | `env.Get("ze.foo")` via `internal/core/env` | `ai/rules/go-standards.md` | Cache, registration, dot/underscore agnostic, secret clearing |
| `log.Printf` or `logrus` | `slog` via `slogutil.Logger("subsystem")` | `ai/rules/go-standards.md` | Hierarchical per-subsystem levels via env vars |
| Shared types via direct import | Cross-boundary payloads are value types only | `ai/rules/plugins.md` (Cross-Boundary Value Types) | No pointer fields across plugin/component boundaries |

### Config / Schema

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Struct tags + `json.Unmarshal` | YANG schema as sole source of truth | `ai/rules/config.md` | Schema-driven validation, migration, completion, diff |
| Config version field | No version numbers; machine-transformable migration | `ai/rules/config.md` | YANG evolution handles schema changes |
| Silent defaults for missing fields | Fail on unknown keys; suggest closest valid | `ai/rules/config.md` | Explicit > implicit |
| `interface{}` for flexible config | `map[string]any` through canonical pipeline | `ai/rules/repo-maintenance.md` | File -> Tree -> ResolveBGPTree -> map[string]any -> PeersFromTree |

### Communication / IPC

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| gRPC or HTTP between services | JSON events down, text commands up, over pipes or net.Pipe | `ai/rules/plugins.md` | Plugin SDK is language-agnostic (Go/Python/Rust) |
| Direct function calls for sync | DirectBridge for typed in-process calls | `ai/rules/plugins.md` (DirectBridge) | Bypasses JSON serialization for internal plugins |
| Channel-based pub/sub | EventBus with typed handles (`events.Register[T]`) | `ai/rules/plugins.md` (EventBus) | Type-safe, registered event types, no raw `bus.Subscribe` |

### Testing

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `go test ./...` for verification | `./le verify worktree` (two-pass + functional + exabgp) | `ai/rules/testing.md` | 349 packages; cached full + race on changed groups |
| Unit tests prove correctness | Unit tests + `.ci` functional tests (both required) | `ai/rules/completion.md` | Unit proves algorithm; `.ci` proves user can reach the feature |
| `testify/assert` | Standard library `testing` | (convention) | No test framework dependencies |
| `go test -race` once | `go test -race -count=20 ./internal/component/bgp/reactor/...` for reactor code | `ai/rules/testing.md` | Rare schedules need repeated runs to surface |

### CLI / Commands

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| `cobra` or `flag` | YANG-modeled dispatch with RPC handlers | `ai/patterns/cli-command.md` | Unified schema for CLI, web, config, completion |
| `command <identifier> [flags]` | `<verb> <noun> <action> [<identifier>]` | `ai/rules/cli.md` | Identifier-keyword ambiguity elimination |
| Format output as string | Return structured JSON, format via pipe operators | `ai/rules/cli.md` | `\| json`, `\| table`, `\| match`, `\| resolve`, etc. |
| Hardcode help text | Derive from registry/schema | `ai/rules/evidence.md` | Single source of truth; no stale enumerations |

### Native Tooling

| Standard Go | Ze | Rule | Why |
|---|---|---|---|
| Ad-hoc scripts for tooling | Native Go package with a registered `./le` action | `ai/rules/go-standards.md` | One typed implementation serves local and CI callers |
| `/tmp` for scratch files | Per-session directory from `./le session scratch ensure` | `ai/rules/commands.md` | Concurrent sessions do not share names |
| `git add -A && git commit` | `./le commit create`, then the generated script | `ai/rules/git-safety.md` | The declared file population is checked before staging |

## Server-Rendered Markup

- **Markup MUST live in a `.templ` file. A Go string literal MUST NOT build an HTML or SVG tag in `internal/component/web` or `internal/component/lg`.**
- **A templ component MUST take a named struct. A `map[string]any` MUST NOT reach one, and a struct field wrapping one MUST NOT either.**
- **A page MUST NOT carry an inline script, an inline style attribute, or an inline event handler. Both packages answer `'self'` for script, so a browser refuses an inline script and an inline handler and tells the server nothing. The rule covers the style attribute too, so both packages hold one rule and a header CAN be tightened without a hunt.**
- **Behavior a page needs MUST reach it as a data attribute an external asset reads. That asset MUST exist in the embedded filesystem the handler serves.**
- **A new exemption MUST carry its reason and MUST raise the exact count beside it. Each guard fixes the size of its table, so widening one is an edit a reader sees.**
- **A gate that names one package MUST NOT be treated as covering its sibling. Each guard walks its own directory, and `lg` shipped two dead handlers under the web package's green.**

| Guard | Where the rules live | What it refuses |
|---|---|---|
| `TestNoGoFileBuildsMarkup` | `internal/test/markupcheck`, `AssertNoMarkup` | a Go string literal that builds a tag. It reads the FORM of a tag, so `usage: set <leaf>` is not a finding, and it knows HTML's void elements, so a bare `<br>` is one. An exemption that explains nothing is a finding, and so is a table that changed size |
| `TestTemplatesAvoidInlineScriptAndStyle` | `internal/test/markupcheck`, `AssertNoInlineScriptOrStyle` | an inline `<script>` block, an inline `style=`, an `on*` handler, an `hx-on` attribute |
| `TestTemplAssetsResolve` | `internal/test/markupcheck`, `AssertAssetsResolve` | a `src` or `href` the served filesystem does not hold, and one naming an asset tree the package does not serve |
| `TestWebViewDataIsTyped`, `TestLGViewDataIsTyped` | `internal/test/templcheck`, `AssertTyped` | a component parameter that is a map, a named map, a bare `any`, or a struct wrapping any of them |
| `./le doc check templ-output` | `internal/le/doc/check` and `internal/le/doc/wiring` | a `*_templ.go` its `.templ` source no longer produces |
| `go test ./internal/component/web ./internal/component/lg` | `internal/test/golden` and the package capture tests | a rendered byte that moved with no fixture behind it |

## Architecture Summary

### System

```
BGP Subsystem (internal/component/bgp/):
  Peers (FSM) → Wire Layer → Reactor (event loop, BGP cache) → EventDispatcher
   ║ formatted events (down) / commands (up)
Plugin Infrastructure (internal/plugin/):
  Registry · Process Manager · Hub · SDK · DirectBridge
   ║ JSON events + base64 wire bytes (down) / text commands (up)
Plugins: RIB, RR, GR, etc. (Go/Python/Rust)
```

BGP Subsystem handles protocol: FSM manages peers, Wire Layer parses messages into WireUpdate, Reactor processes events, EventDispatcher bridges to Plugin Infrastructure. Plugin Infrastructure manages plugin lifecycle and message routing. Plugins implement RIB, dedup, policy.

### Negotiated Capabilities (per-peer)

| Field | Type | Effect |
|-------|------|--------|
| ASN4 | bool | 4-byte ASN in AS_PATH |
| AddPath | map[Family]Mode | Path-ID prefix in NLRI |
| ExtendedMsg | bool | 65535 byte messages |
| ExtendedNextHop | map[Family]AFI | Per-family NH mapping |
| GracefulRestart | *GR | RFC 4724 state |
| RouteRefresh | bool | RFC 2918 |

**Parsing MUST treat the same wire bytes differently, based on caps. Code MUST use ContextID to identify the encoding context for zero-copy.**

### Wire Writing

**All types MUST implement `BufWriter`: `WriteTo(buf, off) int` or `CheckedWriteTo(buf, off) (int, error)`.**
**Context-dependent types MUST take `*PackContext` for ADD-PATH/ASN4.**

### UPDATE Structure

```
UPDATE = Header (19B) + Withdrawn (IPv4) + Path Attributes
  + MP_REACH_NLRI (non-IPv4 announce) + MP_UNREACH_NLRI (non-IPv4 withdraw)
  + NLRI (IPv4 unicast only)
```

### WireUpdate vs RIB

- WireUpdate MUST transport data only (lazy parse via iterators, keeps wire refs).
- RIB MUST store NLRI -> attribute refs into per-type pools, and MUST NOT store WireUpdate refs.
- Code MUST use per-attribute-type pools with dedup, and per-family NLRI pools.

### Forward Pool

Two-tier model with per-destination-peer workers:

- **Peer Pools** (64 buffers per peer, negotiated size): each peer has an Incoming Peer Pool (inbound) and an Outgoing Peer Pool (outbound modification). Encoding code MUST take a buffer from the peer pool matching the direction. Both pools use the same Peer Pool type, sized at init.
- **Global Shared Pool**: byte-budgeted overflow, mixed 4K/64K blocks. Auto-sized from peer prefix maximums via `overflowPoolBudget()`. Code MUST treat pool exhaustion as the backpressure signal.

### Chaos Simulator

**Unbounded event buffer: events MUST NOT be dropped. Ring buffer rejected because losing route events breaks convergence counts.**

### API Command Syntax

```
Text:   update text origin set igp nhop set 1.1.1.1 nlri ipv4/unicast add 10.0.0.0/24
Binary: update hex attr set 400101... nlri ipv4/unicast add 180a00
```

## Related

**You MAY read more about placement here:**
- `ai/rules/plugins.md`: the delete-the-folder invariant.
- `ai/rules/plugins.md`: registration patterns, Proximity Principle.
- `internal/le/tier/tier.go`: the reverse-dependency report + the placement gate.
- `spec-tiers-0-umbrella` (in git history): the taxonomy, the reorganization plan, and the hardening analysis.

## Rationale

### Incidents behind Design Context

Design recommendations made before the relevant context is loaded are unreliable.

DirectBridge provides typed request/response calls between core and plugins. Use it instead of creating a second direct-call mechanism.

### Incident behind the Sibling Call-Site Audit

Precedent: the blob-store fallback fix (`d029a94d` + `5f66e4f5`) was added in one call site. Three siblings were missed; five plugin tests stayed masked for six days.
