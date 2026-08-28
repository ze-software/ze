# Spec: ze-website-0 -- Browser CLI and Protocol Lab (Umbrella)

| Field | Value |
|-------|-------|
| Status | design |
| Scope | cli, plugin, config, tooling, web |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/ze-website.md` (create on the first deferral) |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| Handoff | - |
| Updated | 2026-08-17 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Build a static browser application from the Ze Go source with a new
`ze_website` binary personality. The application lets a user exercise the same
CLI semantics as SSH, store browser-local ZeFS profiles, and run a two-node lab
with the real BGP and BFD engines.

The website is a control-plane lab. It has no VPP, netlink, netfilter, kernel
FIB, P4, host sockets, SSH listener, child plugin process, or operating-system
signal dependency. It must not simulate command output or protocol state.
Operational output must come from the running browser nodes.

The browser application is not a production router and is not an SSH
replacement. It is another adapter for the shared CLI session and an isolated
in-memory protocol runtime.

### Verified Findings

| ID | Finding | Producer | Consequence | Child |
|----|---------|----------|-------------|-------|
| F-1 | A bare `GOOS=js GOARCH=wasm` build of `cmd/ze` succeeds only when no Ze personality is selected | `cmd/ze/main.go`, `cmd/ze/dispatch.go` | A successful empty-personality build is not a usable Ze artifact | 2 |
| F-2 | The `ze_core` WASM build fails in host, process, Bubble Tea, clipboard, and disk-stat paths before it reaches later platform blockers | `internal/component/host/platform_other.go`, `internal/component/plugin/process/sysproc_other.go`, vendored Bubble Tea and clipboard packages | The website must use a selected composition and browser platform files, not the daemon root | 1, 2 |
| F-3 | VPP already compiles out through `ze_vpp`, but netlink, nftables, kernel FIB, and P4 packages remain in the untagged generated composition | `feature-gates.txt`, `internal/component/plugin/all/all.go` | Add general default-on backend gates. Do not add website-only no-op backends | 2 |
| F-4 | CLI command semantics are split between Bubble Tea's private `Model.dispatchCommand` path and the web terminal's private `executeTerminalCommand` path | `internal/component/cli/model_commands.go`, `internal/component/web/cli_terminal.go` | Extract one transport-independent `cli.Session` and migrate every frontend | 1 |
| F-5 | The editor and commit pipeline already use `storage.Storage`, `storage.WriteGuard`, ZeFS candidate versions, schema validation, and candidate promotion | `internal/component/cli/editor.go`, `internal/component/cli/editor_commit.go`, `internal/component/config/storage/pointer.go` | Preserve this pipeline. Do not create a browser-only config model | 3, 4, 5 |
| F-6 | The ZeFS implementation is concrete around `zefs.BlobStore`, and candidate promotion requires its lock and flush semantics | `internal/component/config/storage/blob.go`, `pkg/zefs/store.go`, `pkg/zefs/lock.go` | Put a persistence seam below `zefs.BlobStore`. A direct `localStorage` implementation of `storage.Storage` would duplicate and break commit semantics | 3 |
| F-7 | BGP already accepts injected dialers and listener factories, but its current mock connection helper opens real loopback TCP | `internal/component/bgp/reactor/reactor.go`, `internal/chaos/mocknet/mocknet.go` | Add a browser-safe buffered in-memory `net.Conn` transport and use the production reactor | 5 |
| F-8 | BFD already has an in-memory paired transport | `internal/component/bfd/transport/loopback.go` | Reuse the real BFD engine and pair. Do not invent a browser protocol model | 5 |
| F-9 | The existing web terminal DOM and response shape are reusable, but its HTTP handler reparses and dispatches commands separately | `internal/component/web/assets/cli.js`, `internal/component/web/cli_terminal.go` | Reuse the visual contract after both web paths delegate to `cli.Session` | 1, 4, 6 |

### Children

| # | Spec | Deliverable | Depends | Coordinates with |
|---|------|-------------|---------|------------------|
| 1 | `plan/spec-ze-website-1-cli-session.md` | Transport-independent `cli.Session`; Bubble Tea moved to `cli/tui`; SSH/TUI and HTTP web migrated with output parity | - | `spec-ssh-optional-composition` for shared CLI and auth surfaces |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| 2 | `plan/spec-ze-website-2-wasm-composition.md` | General backend gates, browser platform support, `ze_website` personality, selected registration composition, WASM build and startup smoke | 1 | `spec-dataplane-seams-0-umbrella` for backend boundary ownership |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| 3 | `plan/spec-ze-website-3-browser-zefs.md` | ZeFS persistence seam, complete-blob `localStorage` backend, Web Locks, import/export/reset, corruption and quota handling | - | Zefs format and config storage contracts |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| 4 | `plan/spec-ze-website-4-browser-cli-bridge.md` | Promise-based JavaScript bridge and browser CLI adapter over `cli.Session`, including profile and node selection | 1, 2, 3 | Existing server-rendered web terminal assets |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| 5 | `plan/spec-ze-website-5-bgp-bfd-lab.md` | Two isolated browser nodes with real BGP, BFD, RIB, config reconciliation, and buffered in-memory transports | 2, 3, 4 | BGP reactor, BFD transport, RIB and config runtime |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| 6 | `plan/spec-ze-website-6-integration-release.md` | Static site bundle, user interface, security controls, browser end-to-end suite, documentation, and public artifact | 1 through 5 | `docs/contributing/gh-pages.md` if publication is selected |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->

Each child is independently reviewable and closable. The umbrella closes only
after all six children close and the final browser goal test passes.

### Decisions Taken at the Design Gate

| # | Decision | Chosen | Rejected |
|---|----------|--------|----------|
| D-1 | Shared CLI boundary | `cli.Session` owns parsing, execution, completion, pipes, history, mode, path, prompt, and structured results | SSH-only API, terminal-emulation API, or another browser parser |
| D-2 | Browser command scope | Real registered commands stay discoverable; commands with missing daemon, plugin, or network capability return an explicit unavailable result | Fixtures or simulated operational output |
| D-3 | Dataplane scope | Exclude every host dataplane backend | No-op backends that accept config but apply nothing |
| D-4 | Protocol transport | In-browser in-memory lab network | WebSocket-to-TCP relay, public socket service, or browser Direct Sockets |
| D-5 | Initial topology | Two real Ze nodes | One simulated peer or a configurable topology editor before the base lab works |
| D-6 | Initial protocols | BGP and BFD | Raw Ethernet fabric or additional protocols before the first lab is proven |
| D-7 | Commit semantics | Same semantic sequence as SSH: validate, persist, promote, reconcile, and report through the same session path | Browser-only save or restart shortcut |
| D-8 | Backend gates | General default-on mechanism gates such as `ze_netlink`, `ze_netfilter`, `ze_kernel`, and `ze_p4`; keep `ze_vpp` | One website-specific exclusion switch |
| D-9 | Build identity | `ze_website` is a distinct `cmd/ze` personality; `./le site build` supplies only the required feature tags | Reusing `ze_core` or `ze_web` as the browser personality |
| D-10 | Storage medium | One Base64-encoded complete ZeFS blob per node profile in `localStorage`, with Web Locks for writes | One key per ZeFS path or IndexedDB in the first implementation |

## Required Reading

### Architecture and Rules

- [ ] `docs/architecture/core-design.md` - component isolation, registration, and control-plane boundaries
  → Decision: browser composition must use existing registries and injected runtime seams. It must not import host backends through a central package.
- [ ] `ai/patterns/registration.md` - generated composition and binary personalities
  → Decision: `ze_website` follows the existing `cmd/ze` personality pattern. Command, YANG, BGP, and BFD inventories remain registry-produced.
- [ ] `ai/rules/plugins.md` - feature-gate registration
  → Constraint: every compile-out-able backend is declared once in `feature-gates.txt`; default native builds keep the gate on; absent builds prove symbol omission.
- [ ] `ai/rules/cli.md` and `ai/patterns/cli-command.md` - CLI grammar, pipes, output, and command ownership
  → Constraint: browser commands use the same grammar, registry, completion, pipe handling, error contract, and structured result as SSH.
- [ ] `docs/architecture/web-interface.md` and `docs/architecture/web-components.md` - existing web terminal and browser asset rules
  → Decision: reuse the existing terminal layout and response fields after command semantics move into `cli.Session`.
- [ ] `docs/architecture/zefs-format.md` - ZeFS encoding and persistence contract
  → Constraint: browser persistence stores and restores a complete encoded ZeFS blob. It does not map logical ZeFS paths to separate browser keys.
- [ ] `docs/architecture/bfd.md` - BFD engine and transport ownership
  → Decision: use the existing paired transport and real BFD state machine.
- [ ] `ai/rules/testing.md` and `ai/rules/interop-and-goal-validation.md` - entry-point proof and non-vacuous protocol tests
  → Constraint: the browser test must fail when the real command, commit, BGP, or BFD path is removed. Startup alone is not evidence.
- [ ] `ai/rules/repo-maintenance.md` - new make target, browser test lane, and discovery updates
  → Constraint: `./le site build`, its test target, architecture docs, and `ai/INDEX.md` must land together.

### Related Specifications

- [x] `spec-ssh-optional-composition` - closed SSH compile-out and shared authentication work
  → Constraint: website work must not restore an SSH-named shared CLI or identity dependency.
- [ ] `plan/spec-dataplane-seams-0-umbrella.md` - backend-neutral dataplane boundaries
  → Constraint: website backend gates remove host mechanisms. They must not redefine dataplane contracts or add no-op implementations.

### Browser Platform References

- [ ] MDN `Window.localStorage` - origin-scoped synchronous persistence
  → Decision: use it only for bounded complete profile blobs and preserve the previous value when a write fails.
- [ ] MDN Web Locks API - same-origin cross-tab exclusion
  → Constraint: persistent mutation requires an acquired profile lock. A browser without Web Locks must refuse persistent writes.
- [ ] MDN storage quotas and eviction criteria - quota and private-mode behavior
  → Constraint: quota failure must be visible and must not destroy the last valid blob.

### Source Files Read

- [ ] `cmd/ze/main.go`, `cmd/ze/dispatch.go`, `cmd/ze/ze_core_dispatch.go` - personality entry and current daemon coupling
  → Decision: add a separate website personality instead of changing the `ze_core` daemon root.
- [ ] `feature-gates.txt`, `internal/component/plugin/all/all.go` - default feature inventory and current untagged backends
  → Constraint: move mechanism-owned imports into generated feature groups. Do not hand-maintain a second backend inventory.
- [ ] `internal/component/cli/model_commands.go`, `internal/component/cli/editor.go`, `internal/component/cli/editor_commit.go` - command and config session producers
  → Decision: extract semantics from the model. Keep editor, validation, candidate, and commit behavior shared.
- [ ] `internal/component/web/cli_terminal.go`, `internal/component/web/assets/cli.js` - duplicate web dispatch and current browser terminal
  → Constraint: delete the duplicate dispatch path after migration. Keep HTTP and WASM as adapters only.
- [ ] `internal/component/config/storage/blob.go`, `internal/component/config/storage/pointer.go`, `pkg/zefs/store.go`, `pkg/zefs/lock.go` - ZeFS storage, candidate promotion, and flush-on-release
  → Decision: add the browser persistence seam below `zefs.BlobStore`, not above `storage.Storage`.
- [ ] `internal/component/bgp/reactor/reactor.go`, `internal/chaos/mocknet/mocknet.go`, `internal/component/bfd/transport/loopback.go` - protocol transport injection
  → Constraint: the website supplies browser-safe transports. Protocol algorithms and wire codecs remain unchanged.

**Key insights:**
- The smallest useful target is not a stripped daemon. It is a new personality
  that composes shared CLI, config, BGP, BFD, and RIB code without host services.
- Browser support exposes two existing architecture debts: duplicated CLI
  dispatch and host backends in the untagged composition root.
- ZeFS commit behavior depends on a whole-store lock and candidate promotion.
  Browser storage must preserve those semantics below the concrete blob store.
- Two real Ze nodes are required. One node with fixture output cannot prove the
  CLI or protocol paths.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - unified entry point and browser-incompatible `os.Exit` call after dispatch.
- [ ] `cmd/ze/dispatch.go` - shared personality registration and dispatch.
- [ ] `cmd/ze/ze_core_dispatch.go` - daemon personality imports and command roots.
- [ ] `feature-gates.txt` - canonical compile-out feature inventory.
- [ ] `internal/component/plugin/all/all.go` - generated untagged registration composition.
- [ ] `internal/component/cli/model_commands.go` - Bubble Tea command dispatcher.
- [ ] `internal/component/cli/editor.go` - editor and storage construction.
- [ ] `internal/component/cli/editor_commit.go` - commit, candidate, and reconcile flow.
- [ ] `internal/component/web/cli_terminal.go` - server web terminal dispatcher.
- [ ] `internal/component/web/assets/cli.js` - current browser terminal adapter.
- [ ] `internal/component/config/storage/blob.go` - concrete ZeFS storage adapter.
- [ ] `internal/component/config/storage/pointer.go` - candidate promotion.
- [ ] `pkg/zefs/store.go` - ZeFS blob encoding and persistence.
- [ ] `pkg/zefs/lock.go` - exclusive write and flush-on-release behavior.
- [ ] `internal/component/bgp/reactor/reactor.go` - BGP runtime injection seams.
- [ ] `internal/chaos/mocknet/mocknet.go` - current loopback-TCP mock transport.
- [ ] `internal/component/bfd/transport/loopback.go` - in-memory BFD pair.


### Build and Composition

`cmd/ze/main.go` has one main function. Build-tagged files register a binary
personality. The `ze_core` personality imports the hub, command providers,
configuration packages, and the generated `plugin/all` root.

`feature-gates.txt` already gates VPP, BGP, BFD, SSH, web, and other optional
features. The untagged generated root still imports kernel FIB, P4 FIB,
nftables, interface netlink, traffic netlink, kernel, static, and related YANG
packages.

The exact scoped probe `GOOS=js GOARCH=wasm CGO_ENABLED=0 go build -tags
ze_core ./cmd/ze` fails before linking. Reported failures include unavailable
resource-limit syscalls, `SysProcAttr.Setpgid`, Bubble Tea platform functions,
and clipboard platform functions. A build without a Ze personality succeeds but
has no CLI, config, or protocol behavior.

### CLI

Bubble Tea `Model` owns editor state and calls a private command dispatcher from
its input loop. The web terminal has a second private dispatcher and returns
structured fields for output, feedback, path, prompt, and mode. YANG verbs and
registered local or operational commands reach different helpers depending on
the frontend.

### Storage

`storage.NewBlob` constructs a `zefs.BlobStore`. The editor reads active content,
creates draft changes, validates the YANG tree, writes a candidate version under
a `storage.WriteGuard`, and promotes the candidate after the runtime accepts it.
The ZeFS write lock flushes the full encoded store once on release.

### Protocol Runtime

BGP reactor construction accepts injected dialers, listener factories, clocks,
and timers. Its default constructor still installs host network functions. The
current chaos mock listener uses real loopback TCP for connection pairs.

BFD provides an in-memory paired transport. Both protocol engines expose real
operational state through registered command handlers when their runtime is
started.

### Behavior to Preserve

- Default native and appliance builds keep every currently default-on backend.
- SSH/TUI and server web CLI grammar, completion, history, pipes, mode, prompt,
  config editing, output text, JSON output, and errors remain unchanged.
- BGP and BFD protocol behavior and wire encoding remain unchanged.
- ZeFS on-disk encoding, candidate promotion, version history, and lock behavior
  remain valid for native storage.
- Registration remains the source of command, YANG, plugin, and capability
  discovery.

### Behavior to Change

- `./le site build` builds a runnable browser artifact with `ze_website`, BGP,
  and BFD, without `ze_core` or host backends.
- All CLI frontends delegate semantic work to one `cli.Session`.
- Host dataplane mechanisms become general default-on feature gates.
- ZeFS can persist its complete encoded blob through a browser backing store.
- The website starts two isolated nodes connected only through in-memory BGP and
  BFD transports.

## Data Flow (MANDATORY)

### Entry Point

- Build entry: `./le site build` compiles `cmd/ze` for `js/wasm` and assembles a
  static bundle.
- Browser entry: the page loads the Go runtime support file and WASM artifact,
  then waits for an explicit ready result.
- User entry: terminal input, completion requests, profile actions, and node
  selection enter through the browser bridge.
- Config entry: editor commands mutate a node's draft and commit through the
  normal editor and storage pipeline.

### Transformation Path

1. The `ze_website` personality imports only the selected registration roots for
   shared config, CLI, BGP, BFD, RIB, and their command/YANG providers.
2. Browser startup creates node A and node B. Each node owns one `cli.Session`,
   one ZeFS profile, one config tree, one BGP reactor, one BFD runtime, and one
   RIB state.
3. The browser adapter sends a command and cursor state to `cli.Session`.
4. `cli.Session` parses, completes, executes, applies pipes, mutates editor state,
   and returns the shared structured result.
5. A commit validates YANG, writes a candidate version, promotes it, and invokes
   the node reconciliation callback. A failed reconcile keeps the prior active
   runtime and reports the failure.
6. Reconciliation creates or updates BGP peers and BFD sessions for the selected
   node. It does not call host dataplane or process APIs.
7. BGP connections use buffered in-memory `net.Conn` pairs. BFD uses paired
   datagram transports. Each endpoint belongs to one node.
8. Registered operational commands query the running reactors, sessions, and
   RIBs. Their output returns through the same `cli.Session` result path.
9. On a successful storage mutation, the node's complete encoded ZeFS blob is
   saved under its profile key while the profile Web Lock is held.

### Boundaries Crossed

| Boundary | How | Required proof |
|----------|-----|----------------|
| Make target to browser artifact | `GOOS=js`, `GOARCH=wasm`, selected tags, static asset assembly | Build gate plus real browser startup |
| Generated registration to website schema and commands | Selected blank imports and package `init()` registration | Inventory test from the linked artifact |
| Browser DOM to Go | Promise-based JavaScript bridge with structured values | Browser command and completion test |
| CLI session to editor | Shared `cli.Session` calls existing editor methods | Native/browser parity tests |
| Editor to ZeFS | Existing `storage.Storage` and `storage.WriteGuard` contracts | Commit, version, rollback, and history tests |
| ZeFS to browser persistence | Complete encoded blob and Web Lock | Refresh, cross-tab, quota, corruption, and import/export tests |
| Node runtime to BGP | Injected in-memory `net.Conn` dial/listen pair | Established session and exchanged route |
| Node runtime to BFD | Existing paired datagram transport | Up state and failure transition |
| Operational command to live state | Registered handler reads node runtime | Mutation test that fails when engine path is removed |

### Integration Points

- `cmd/ze` binary personality registration and the native action tables under `internal/le/` target.
- `feature-gates.txt` and generated plugin imports.
- `internal/component/cli` session, editor, completion, pipes, and rendering
  boundaries.
- `internal/component/web` terminal handler and browser assets.
- `internal/component/config/storage` and `pkg/zefs` persistence.
- BGP reactor dialer/listener injection and BFD paired transport.
- RIB and registered operational command handlers.
- Browser test runner and static artifact packaging.

### Browser Bridge Contract

| Operation | Input | Output | Constraint |
|-----------|-------|--------|------------|
| Execute | Node, command text | Output, feedback, path, prompt, mode, status | Asynchronous; never blocks the browser event loop |
| Complete | Node, input text, cursor | Registry/YANG-derived completion items | No browser-maintained command list |
| State | Node | Current path, prompt, mode, dirty state, protocol summary | Reads live session and runtime state |
| Profile | Create, select, list, or delete request | Profile metadata or actionable error | Mutation requires the profile lock |
| Export | Node profile | Versioned complete ZeFS payload | Label as browser-lab data; never auto-download secrets |
| Import | Node profile and payload | Validated replacement or explicit refusal | Validate before replacing the prior blob |
| Reset | Node profile | New empty profile state | Explicit user action only; preserve other profiles |

### Architectural Verification

| Check | Holds? | Evidence or required action |
|-------|--------|-----------------------------|
| No bypassed CLI layer | Design holds | Every frontend delegates to `cli.Session`; child 1 removes both private semantic dispatch paths |
| No no-op backend | Design holds | Missing backend schema and packages are absent; unsupported command/config returns an explicit error |
| No duplicate command inventory | Design holds | Completion and availability derive from linked registries and YANG |
| Registration over hardcoding | Design holds | Website composition imports owners; owners register themselves |
| Native behavior preserved | Unvalidated | Child 1 parity tests and child 2 default-on build matrix |
| Two-node isolation | Unvalidated | Child 5 ownership and concurrency tests |
| Protocol hot paths unchanged | Design holds | New work supplies transports and lifecycle only; codecs and forwarding algorithms are not modified |
| Browser event loop remains responsive | Unvalidated | Child 4 browser responsiveness and cancellation tests |

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Go goroutines, channels, and timers are sufficient for the existing BGP and BFD runtime under browser WASM | Go `js/wasm` runtime supports the language primitives; current compile fails in platform packages, not protocol algorithms | Protocol lifecycle needs a new scheduler boundary or the target is not viable | Real browser two-node session test under sustained updates | unvalidated |
| A-2 | Two BGP reactors can run in one process with injected transports and separate config | Reactor constructors own peer/listener state and expose injection seams | Global mutable state leaks between nodes | Two-node isolation test with conflicting ASNs, router IDs, and prefixes | unvalidated |
| A-3 | The BFD paired transport can serve two independent node runtimes without host UDP | Existing pair is channel-backed | BFD startup still reaches host socket setup | Browser BFD Up and Down test with host network calls trapped | unvalidated |
| A-4 | Global registries are immutable after startup and safe to share between node runtimes | Registration is init-time; runtime instances hold operational state | Per-node configuration or state contaminates the other node | Race and cross-node state tests | unvalidated |
| A-5 | Representative two-node lab profiles fit browser storage limits | A complete demo config and history are small relative to common browser quotas | Quota failures make normal use unreliable | Size measurement and browser quota failure test | unvalidated |
| A-6 | Supported browsers provide Web Locks in the served context | Web Locks is available in current secure browser contexts | Concurrent tabs can corrupt one profile | Browser capability probe and cross-tab exclusion test | unvalidated |
| A-7 | Every host-only dependency can be excluded or given an honest browser platform implementation without changing protocol algorithms | Current failures are at composition and platform seams | A protocol package has an unavoidable host dependency | Dependency audit plus iterative WASM build and browser startup | unvalidated |
| A-8 | Existing server web and TUI output can migrate to `cli.Session` without byte or semantic drift | Both paths already return similar terminal state and use the same editor types | Migration changes native CLI behavior | Native golden, editor, web, and parity tests | unvalidated |
| A-9 | Browser export can preserve the complete ZeFS payload across desktop and browser readers | ZeFS already has one encoded blob format | Browser persistence needs a divergent format | Cross-format export/import round trip | unvalidated |
| A-10 | The first release needs no external network relay | User selected an in-browser lab network | Users require connection to external peers | Product review after the two-node lab is proven | confirmed by user scope |

### Risks

| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|-----------------------|
| R-1 | A new backend gate drops behavior from default native builds | Default feature matrix omits symbols or rejects existing config | Make every new mechanism gate default-on and add present/absent schema plus symbol tests |
| R-2 | Website composition silently registers a host command whose handler later reaches ENOSYS | A command appears in completion but fails inside a syscall | Derive an availability classification from linked capability; exclude the owner or return explicit unavailable before execution |
| R-3 | A browser-specific CLI path becomes a third parser | Browser output differs from SSH after grammar or completion changes | Child 1 removes duplicate dispatch before child 4 adds the adapter |
| R-4 | Long Go work blocks the browser event loop | Input, repaint, or cancellation stalls during commit or protocol churn | Promise-based bridge, bounded operations, lifecycle cancellation, and responsiveness test |
| R-5 | A failed localStorage write destroys the last valid profile | Refresh opens an empty or corrupt profile after quota failure | Validate and encode first, write one complete replacement, preserve prior value on error, and verify recovery |
| R-6 | Two tabs write one profile concurrently | Lost versions or invalid ZeFS blob | Require Web Lock for mutation; fail closed when unavailable or already held |
| R-7 | Imported data carries passwords, tokens, certificates, or private keys into same-origin storage | Import contains sensitive schema paths | Omit secret-bearing modules where possible and reject sensitive imports with an explicit path list |
| R-8 | Same-origin script compromise reads every profile | Any XSS or third-party script runs in the page origin | Strict CSP, no inline script, no third-party runtime dependency, no secrets, and web security review |
| R-9 | Protocol test passes through a fixture or unchanged path | Removing the reactor or transport still leaves expected text | Mutation-prove each functional test against the producing engine path |
| R-10 | Global runtime state crosses node boundaries | Node A command shows node B peers or routes | One explicit node owner for every mutable object; no package-global operational state |
| R-11 | Browser artifact is too large or starts too slowly for a test page | Bundle or ready time exceeds the recorded budget | Measure each child, remove unused registration roots, and set release budgets from the first working artifact |
| R-12 | Browser storage semantics diverge from native ZeFS | Exported profile cannot reopen or version history differs | Keep one encoder and candidate pipeline; only persistence and locking vary by platform |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Native feature composition, SSH/TUI CLI behavior, ZeFS persistence, or protocol lifecycle can regress; browser profiles can be lost |
| How is it reversed? | Each child is one focused change and must preserve native behavior before the next child begins |
| Who else touches these paths? | Active SSH optionality work touches hub, auth, and config; dataplane seam work owns backend contracts; web work owns server assets |
| What is outside the blast radius? | BGP/BFD wire encoding, external peer transport, appliance dataplane behavior, and public plugin process protocol |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Native SSH/TUI command sequence | → | `cli.Session` | `TestCLISessionNativeParity` |
| Server web terminal request | → | HTTP adapter → `cli.Session` | `TestWebTerminalUsesSharedSession` |
| `./le site build` | → | `ze_website` personality and selected composition | `TestWebsiteWASMBuildAndStart` |
| Website command input | → | JavaScript bridge → `cli.Session` | `TestWebsiteExecuteAndComplete` |
| Website config commit | → | editor → candidate → promote → node reconcile | `TestWebsiteCommitReconcilesNode` |
| Browser profile write and refresh | → | ZeFS encoder → localStorage backing | `TestBrowserZeFSRefreshPersistence` |
| Second tab opens same profile | → | Web Lock acquisition | `TestBrowserZeFSCrossTabExclusion` |
| Node A and node B BGP config | → | in-memory dial/listen → two reactors | `TestWebsiteBGPEstablishedAndRouteExchange` |
| Node A and node B BFD config | → | paired datagram transport → two BFD engines | `TestWebsiteBFDUpAndFailure` |
| Host backend config in website | → | absent schema/registration | `TestWebsiteRejectsHostBackends` |
| Sensitive ZeFS import | → | import validator | `TestWebsiteRejectsSensitiveImport` |
| Export then new profile import | → | complete ZeFS payload | `TestWebsiteExportImportRoundTrip` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Child |
|-------|-------------------|-------------------|-------|
| AC-1 | Run `./le site build` in a supported host environment | It produces a static bundle containing the Go WASM artifact, Go runtime support, HTML, CSS, and JavaScript, and the page reaches an explicit ready state in a real browser | 2, 6 |
| AC-2 | Inspect the website artifact and dependency graph | It contains BGP, BFD, RIB, CLI, config, and ZeFS code and contains no VPP, netlink, netfilter, kernel FIB, P4, SSH server, child-process, host-signal, or host-socket implementation symbols | 2 |
| AC-3 | Run the same shared command sequence through SSH/TUI, server web, and WASM | Parsing, mode and path transitions, completion items, pipe behavior, config mutations, structured status, text output, JSON output, and errors are semantically identical; only frontend rendering differs | 1, 4, 6 |
| AC-4 | Build default native and appliance compositions after adding backend gates | Every existing default-on backend and its config schema remains present, and absent builds reject the omitted backend's config instead of accepting a no-op | 2 |
| AC-5 | Create two browser profiles and configure different router IDs, ASNs, and prefixes | Each node keeps separate CLI, ZeFS, config, reactor, BFD, and RIB state with no cross-node leakage | 3, 5 |
| AC-6 | Edit and commit valid config in the website | The normal editor validates, writes a candidate, promotes it, reconciles the selected node, and reports success through `cli.Session` | 1, 3, 4, 5 |
| AC-7 | Commit invalid config or inject reconcile failure | Validation or reconciliation fails with an actionable error; the prior active runtime and prior valid persisted blob remain usable | 3, 5 |
| AC-8 | Configure matching BGP peers on both browser nodes | The real BGP FSM reaches Established over buffered in-memory connections, an announced route crosses the session, and the remote live RIB and CLI show it | 5, 6 |
| AC-9 | Configure matching BFD sessions on both browser nodes | The real BFD engines reach Up over the paired transport; breaking the pair produces the expected non-Up state and CLI output | 5, 6 |
| AC-10 | Request a command whose capability is not linked in the website | The command is absent from discovery or returns an explicit unavailable result that names the missing capability; no fixture, success-shaped zero value, or silent no-op is returned | 1, 2, 4 |
| AC-11 | Refresh the page after commits and version history changes | Both node profiles reopen with the same active config, versions, drafts, and selected node state | 3, 6 |
| AC-12 | Open a second tab for a profile being mutated | The second writer is refused with an actionable lock error; reads remain consistent and the stored blob is not corrupted | 3, 6 |
| AC-13 | Fill storage, corrupt a stored value, or supply an unknown import version | The website reports the exact failure, preserves the previous valid value when available, and offers explicit export or reset recovery | 3, 6 |
| AC-14 | Import or seed a profile containing sensitive credentials or private key material | The operation fails closed and identifies the sensitive paths; the website never seeds or persists such data | 3, 6 |
| AC-15 | Export a valid browser profile and import it into a new profile | The new profile has the same active config and version history, and the original profile is unchanged | 3, 6 |
| AC-16 | Run the browser test with the BGP reactor, BFD engine, commit promotion, or shared CLI call removed in turn | The corresponding test fails, proving the suite observes the real producer path | 5, 6 |
| AC-17 | Use the website without external network access after static assets load | CLI, storage, BGP, and BFD lab workflows continue to work; no relay, daemon, or remote API is contacted | 2, 5, 6 |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Opens the site, configures two BGP nodes, commits, and announces a prefix | browser UI → shared CLI → editor/ZeFS → node reconcile → two BGP reactors → remote RIB | `TestWebsiteBGPEstablishedAndRouteExchange` |
| 2 | Enables BFD for the lab peers and breaks the link | browser UI → shared CLI → BFD pair → real session state → operational command | `TestWebsiteBFDUpAndFailure` |
| 3 | Uses completion, history, modes, pipes, text output, and JSON output as in SSH | browser adapter → `cli.Session` → registered handlers and renderers | `TestWebsiteExecuteAndComplete` plus `TestCLISessionNativeParity` |
| 4 | Refreshes after editing and later reopens both nodes | editor → ZeFS candidate/promotion → localStorage → browser reload | `TestBrowserZeFSRefreshPersistence` |
| 5 | Opens a second tab while the first tab commits | browser bridge → Web Lock → explicit refusal | `TestBrowserZeFSCrossTabExclusion` |
| 6 | Exports a lab, resets one profile, and imports the export into a new profile | export validator → complete ZeFS payload → import validator → replacement | `TestWebsiteExportImportRoundTrip` |
| 7 | Attempts to configure a host backend or run an unavailable host command | selected schema/registry → explicit rejection | `TestWebsiteRejectsHostBackends` |

## 🧪 TDD Test Plan

### Unit Tests

| Test | Owning child | Validates | Required failure before implementation |
|------|--------------|-----------|----------------------------------------|
| `TestCLISessionNativeParity` | 1 | AC-3 | Shared session absent or native frontend still uses a private dispatcher |
| `TestWebTerminalUsesSharedSession` | 1 | AC-3 | Web handler dispatches directly |
| `TestBackendFeatureTagsDefaultOn` | 2 | AC-4 | Backend imports remain untagged or default build omits them |
| `TestWebsiteCompositionDropsHostSymbols` | 2 | AC-2 | Forbidden backend or process symbol remains linked |
| `TestBrowserZeFSRoundTrip` | 3 | AC-11, AC-15 | Complete blob cannot reopen with history |
| `TestBrowserZeFSPreservesPriorValueOnFailure` | 3 | AC-7, AC-13 | Failed write replaces valid data |
| `TestBrowserZeFSRejectsSensitivePaths` | 3 | AC-14 | Sensitive payload is accepted |
| `TestBrowserBridgeStructuredResult` | 4 | AC-3, AC-10 | Bridge bypasses session or returns ambiguous zero values |
| `TestBrowserNodeIsolation` | 5 | AC-5 | Mutable protocol or config state is shared |
| `TestInMemoryBGPTransportBackpressureAndClose` | 5 | AC-8 | Transport loses bytes, ignores close, or has no bounded backpressure |

### Boundary Tests

| Field | Valid values | Boundary cases |
|-------|--------------|----------------|
| Browser profiles | Distinct non-empty identifiers | Empty, duplicate, unsupported characters, longest accepted identifier |
| Node selector | A or B in the first release | Missing, unknown, stale after reset |
| Stored payload | Supported version and valid Base64 ZeFS blob | Empty, malformed Base64, unknown version, truncated blob, quota failure |
| Command input | Valid shared CLI input | Empty, incomplete, invalid token, pipe-only, large bounded input |
| Transport buffer | Child-defined positive bounded capacity | Full buffer, close while blocked, peer close, cancellation |
| Export/import size | Child-defined supported maximum | Last accepted byte, first rejected byte |

Every numeric maximum introduced by a child must test its last valid value and
first invalid values below and above where applicable.

### Functional Tests

Child 6 implements these cases in `internal/le/deployment/` and  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
binds them to `./le site check`.

| Test | Entry point | What it proves | Status |
|------|-------------|----------------|--------|
| `TestWebsiteWASMBuildAndStart` | `./le site build` then real browser page load | AC-1 and browser runtime startup | planned |
| `TestWebsiteExecuteAndComplete` | Browser terminal | AC-3 and AC-10 through the user surface | planned |
| `TestWebsiteCommitReconcilesNode` | Browser terminal config workflow | AC-6 and AC-7 | planned |
| `TestBrowserZeFSRefreshPersistence` | Browser refresh and reopen | AC-11 | planned |
| `TestBrowserZeFSCrossTabExclusion` | Two real browser tabs | AC-12 | planned |
| `TestWebsiteBGPEstablishedAndRouteExchange` | Browser lab UI and CLI | AC-8 and AC-16 | planned |
| `TestWebsiteBFDUpAndFailure` | Browser lab UI and CLI | AC-9 and AC-16 | planned |
| `TestWebsiteRejectsHostBackends` | Browser CLI/config | AC-2, AC-4, and AC-10 | planned |
| `TestWebsiteRejectsSensitiveImport` | Browser import UI | AC-14 | planned |
| `TestWebsiteExportImportRoundTrip` | Browser profile UI | AC-15 | planned |
| `TestWebsiteOfflineAfterLoad` | Browser network interception | AC-17 | planned |

Child 6 must add one documented make target that builds the bundle, serves it
from this session's scratch directory, drives a real browser, and stops the
server. It must not require an external service.

### Interop Tests

N-A. This umbrella does not change BGP or BFD wire behavior and does not connect
to an external peer. Existing protocol interop tests remain required and must
stay green. The new browser goal test proves the selected transport and runtime
integration with two real Ze protocol engines.

## Files to Modify

- (none directly) - the umbrella coordinates. Each child names exact source,
  test, generated, documentation, and build files after its own source audit.

## Files to Create

- `plan/spec-ze-website-1-cli-session.md` - shared CLI semantic core and frontend migration.  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
- `plan/spec-ze-website-2-wasm-composition.md` - backend gates and WASM personality.  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
- `plan/spec-ze-website-3-browser-zefs.md` - browser ZeFS persistence.  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
- `plan/spec-ze-website-4-browser-cli-bridge.md` - JavaScript and DOM adapter.  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
- `plan/spec-ze-website-5-bgp-bfd-lab.md` - two-node protocol runtime.  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
- `plan/spec-ze-website-6-integration-release.md` - final static application, security, tests, docs, and artifact.  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->

### Cross-Specification Ownership

| Surface | Owning child | Contract |
|---------|--------------|----------|
| `cli.Session` and native frontend migration | 1 | One semantic engine; no browser code in the session core |
| Backend feature manifest and generated import groups | 2 | General default-on gates; no website-specific no-op backend |
| `ze_website` personality and the native action tables under `internal/le/` build | 2 | Selected composition only; no `ze_core` or `ze_web` server root |
| ZeFS persistence seam and browser backing | 3 | One encoder and commit model; browser-only storage implementation behind platform files |
| JavaScript bridge and reusable browser terminal adapter | 4 | Structured asynchronous contract; no command or schema copy in JavaScript |
| Node runtime, transport, reconcile, BGP, BFD, and RIB | 5 | Real engines and isolated mutable state; no external socket relay |
| Final page, browser runner, CSP, packaging, docs, and public artifact | 6 | End-to-end user stories and mutation-proven tests |
| SSH/auth schema optionality | `spec-ssh-optional-composition` | Website children must consume its final shared ownership and must not restore SSH coupling |
| Generic dataplane seam design | `spec-dataplane-seams-0-umbrella` and children | Website gates omit backends but do not redefine shared dataplane payloads |

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | Children 2 and 5 select existing BGP/BFD/config modules; no browser-only config syntax. Absent backend modules must be rejected |
| YANG validation constraints | Yes | Existing constraints remain. Child 3 import validation must run the registered schema before replacement |
| YANG custom validators | Yes | Existing dynamic completion and validators must remain linked when their owner is selected; no browser duplicate |
| CLI commands/flags | No | No new user CLI command is required. `./le site build` is a build target and the browser UI is the entry point |
| CLI grammar (keyword before value) | Yes | Child 1 preserves the existing grammar through `cli.Session`; `./le cli-grammar` remains authoritative |
| Editor autocomplete | Yes | Child 1 and child 4 derive completion from YANG and command registries |
| Functional test for new RPC/API | Yes | Child 4 JavaScript bridge contract and child 6 browser entry-point tests |
| Pipe completeness | Yes | `cli.Session` owns existing text and JSON pipe handling for every frontend |
| Env var registration | N-A | Browser profiles and bridge operations add no `ze.*` environment variable |
| Doctor check for runtime dependencies | N-A | The static browser target adds no host runtime dependency, listener, socket, certificate, external binary, or kernel requirement |
| Prometheus counters/metrics | N-A | The browser lab does not add production metrics. Runtime state is visible through existing operational commands |
| BGP family surface (new SAFI / capability / attribute) | N-A | No new family, capability, attribute, or wire behavior |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|----------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` and a browser-lab feature page in child 6 |
| 2 | Config syntax changed? | No | Existing YANG syntax is reused; verify examples against the selected website schema |
| 3 | CLI command added/changed? | No | Command semantics and availability are preserved or explicitly absent by composition |
| 4 | API/RPC added/changed? | Yes | Document the browser bridge under the owning browser architecture page, not the daemon process protocol |
| 5 | Plugin added/changed? | Yes | Backend compile-out and selected registration change composition; update plugin/build composition documentation |
| 6 | Has a user guide page? | Yes | Create `docs/guide/browser-lab.md` with build, use, persistence, recovery, and security limits |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| 7 | Wire format changed? | No | BGP and BFD encoding are unchanged |
| 8 | Plugin SDK/protocol changed? | No | No external plugin process or SDK contract is used in WASM |
| 9 | RFC behavior implemented, changed, or newly proven? | No | Existing BGP/BFD behavior is reused; no standards ledger change |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` and browser test target documentation |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` must distinguish the browser lab from production daemon behavior if it lists deployment surfaces |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md`, CLI architecture, web architecture, ZeFS architecture, and a browser-lab architecture page |
| 13 | Route metadata keys added/changed? | No | Existing RIB and route metadata are reused |
| 14 | Prometheus counters added/changed? | No | No new metrics |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | Generated composition and feature inventory docs; command inventory remains registry-derived |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | Every child greps and updates anchors for its exact changed files |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | Verify command reference, configuration, web terminal, BGP, BFD, and ZeFS examples |

### Discovery Checklist

| Question | Answer |
|----------|--------|
| Where would an agent look first? | Add `ze_website`, browser lab, WASM, browser ZeFS, and browser CLI rows to `ai/INDEX.md` |
| What rule or gate prevents regression? | Feature gate checks, CLI grammar, generated plugin import checks, native parity tests, WASM build gate, and browser functional target |
| What source of truth prevents drift? | `feature-gates.txt`, command/YANG/plugin registries, generated compositions, and the linked browser artifact inventory |
| What verification proves it? | `./le site build`, child package tests, `./le site check`, feature matrix checks, native CLI/web tests, and goal mutation runs |
| What docs explain usage? | `docs/guide/browser-lab.md` and the browser-lab architecture page created by child 6 |  <!-- doc-links: ignore (file this spec will create; the spec is `design` and the work is not implemented) -->
| What journal record preserves the decision? | None required now. No recurring implementation defect was diagnosed during design |

## Implementation Steps

1. **Phase: Child specification creation** - write children 1 through 6 from this
   umbrella. Each child repeats source verification for its owned surfaces and
   includes failing wiring tests before implementation.
2. **Phase: Shared CLI session** - implement child 1 first.
   - Migrate SSH/TUI and server web before any browser adapter is added.
   - Prove native semantic and output parity.
3. **Phase: Independent foundations** - implement child 2 and child 3 after
   child 1 design is stable. They may run in parallel if their file ownership is
   disjoint.
   - Child 2 owns platform composition and backend gates.
   - Child 3 owns persistence and locking only.
4. **Phase: Browser CLI adapter** - implement child 4 after children 1 through 3.
   - Build the asynchronous bridge over the shared session.
   - Prove command, completion, profile, import, export, and reset contracts in a
     real browser.
5. **Phase: Protocol lab** - implement child 5.
   - Create two isolated runtimes and buffered transports.
   - Wire commit reconciliation, BGP, BFD, RIB, and operational commands.
6. **Phase: Integration and release** - implement child 6 only after all
   foundation children pass their own gates.
   - Assemble the static application, apply CSP and storage restrictions, add
     browser goal tests, document the feature, and measure bundle/startup budgets.
7. **Phase: Umbrella closure** - run the final goal test and independent review.
   Close every child before closing and deleting this umbrella.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Child completeness | Every finding F-1 through F-9 and AC-1 through AC-17 has exactly one owning child and one final proof |
| CLI single path | No frontend retains semantic parsing, dispatch, completion, pipes, or editor logic outside `cli.Session` |
| Composition honesty | The website artifact links only selected capabilities; missing host features cannot accept configuration and pretend success |
| Native preservation | Default native, appliance, SSH/TUI, and server web behavior remain green after each child |
| Node isolation | Every mutable CLI, config, protocol, timer, RIB, and storage object has one node owner |
| Storage correctness | Browser persistence preserves ZeFS candidate, promotion, version, history, and lock semantics |
| Protocol reality | BGP and BFD tests read actual FSM/RIB state and fail when the engine or transport is removed |
| Security | No secrets enter browser storage; CSP and cross-tab locking fail closed |
| Simplicity | No relay, virtual Ethernet fabric, backend no-op, browser schema copy, or second command registry is introduced |
| Performance | New allocations stay at browser/command boundaries; BGP/BFD wire and forwarding paths are unchanged |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Six child specifications | `./le spec-status` plus exact child path checks |
| Shared CLI session | Child 1 package tests, editor tests, web tests, and parity contract |
| General backend gates | Feature matrix, dependency audit, generated import check, and native present/absent artifact tests |
| Browser ZeFS | Unit tests plus real-browser persistence, quota, corruption, lock, and import/export tests |
| WASM bundle | `./le site build` and artifact inventory |
| Two-node BGP/BFD lab | Real-browser Established, route exchange, BFD Up/Down, and node-isolation tests |
| Static website user workflow | `./le site check` in a real browser with network disabled after load |
| Documentation and discovery | Documentation verification, wiring check, and `ai/INDEX.md` entries |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Sensitive data | No passwords, password hashes, tokens, certificates, SSH keys, or private keys are seeded, imported, persisted, or exported |
| Import validation | Malformed, unknown-version, oversized, and sensitive payloads fail before replacing prior data |
| Stored data integrity | One complete validated replacement; no partial multi-key transaction |
| Concurrency | Every mutation holds the profile Web Lock; unavailable locks fail closed |
| Browser execution | Strict CSP, no inline script, no third-party code, no unsafe HTML construction, and no command interpolation into markup |
| Capability honesty | Missing host capability is absent or explicit, never success-shaped zero output |
| Resource exhaustion | Bound command input, transport queues, profile size, history retention, and protocol lifecycle |
| Network isolation | No external socket, relay, fetch, or daemon dependency after assets load |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Native CLI output or behavior changes | Child 1; restore shared semantics before browser work continues |
| `js/wasm` compilation reaches a host package | Child 2; gate the owner or add an honest browser platform file |
| Default native backend disappears | Child 2; fix default-on feature derivation, never special-case the website |
| Browser commit bypasses candidate promotion | Child 3 or 4; use the existing editor/storage contract |
| Profile corruption or lost prior value | Child 3; fix atomic complete-blob replacement and lock behavior |
| Browser bridge blocks the event loop | Child 4; fix asynchronous boundary and cancellation |
| BGP uses loopback TCP or BFD uses host UDP | Child 5; fix transport injection, never add a relay |
| Operational output can pass without a running engine | Child 5 or 6; fix the test and producer path |
| Browser test passes after producer removal | Child 6; mutation-prove the test before closure |
| Security review finds secret persistence | Child 3 or 6; fail closed and keep the umbrella open |

## Alternatives Considered

| Approach | Result | Reason |
|----------|--------|--------|
| Shared `cli.Session`, separate WASM personality, browser ZeFS backing, in-memory lab | Selected | Reuses semantic and protocol producers while excluding host mechanisms |
| Compile `ze_core` and add js stubs until it links | Rejected | Retains daemon composition, host command inventory, and pressure for dishonest no-op backends |
| Reuse the server web terminal through an HTTP or WebSocket relay | Rejected | Adds an external service and does not produce a self-contained browser artifact |
| Port Bubble Tea wholesale through a terminal emulator | Rejected | Keeps semantic work inside the TUI model and adds terminal/browser dependencies without solving duplicate dispatch |
| Implement browser config and protocol fixtures in JavaScript | Rejected | Creates new schema, command, storage, and protocol models that can drift from Ze |
| Use browser Direct Sockets | Rejected | Not available to ordinary websites and would add an untrusted public network boundary |

## Design Insights

- A new personality is smaller and more honest than a daemon with every host call
  replaced by a stub.
- General backend gates improve native composition. The website only selects the
  subset; it does not own a second backend taxonomy.
- The CLI boundary must move above SSH and Bubble Tea. Transport and rendering
  stay in adapters.
- Local storage is safe for this bounded lab only when the profile excludes
  secrets, mutations are locked, and failures preserve the prior blob.
- The protocol lab needs two full nodes because session and RIB behavior cannot
  be proven by one runtime and a fixture.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|-------------------------|-----------|
| Six children with one final integration child | One flat implementation spec; one child per file area | Dependencies are explicit and each child can close without leaving a user-facing half-path |
| `ze_website` personality | Reuse `ze_core`; add a second binary directory | Existing personality registration keeps one `cmd/ze` composition root without importing daemon startup |
| `cli.Session` with frontend adapters | SSH API; terminal API; wrap current model | One semantic engine removes the existing duplicate before browser support adds another frontend |
| Complete ZeFS blob in localStorage | One browser key per path; IndexedDB first | Preserves current encoder, lock, version, and candidate semantics with the least new machinery |
| General default-on backend tags | Website exclusion switch; no-op WASM backends | Native builds remain unchanged and every absent capability is mechanically provable |
| Two-node in-memory BGP/BFD | External relay; mock peer | Real engines and no external network meet the testing goal without a public service boundary |
| Final real-browser test lane | js-only unit tests; server web tests | Only a real browser proves WASM startup, DOM bridge, Web Locks, refresh, storage, and two-tab behavior |

## Known Limitations

- The first release supports two fixed Ze nodes and BGP plus BFD only.
- It has no external peer connectivity, raw Ethernet, host interface, dataplane,
  plugin process, SSH, or appliance behavior.
- It is a browser lab, not a production routing deployment.
- `localStorage` limits profile size and exposes data to same-origin JavaScript.
  The website therefore rejects secret-bearing profiles.
- Browser support is limited to environments that run Go WASM and provide Web
  Locks. Unsupported environments can view an error but cannot mutate profiles.
- The exact bundle-size and startup-time budgets are set from the first working
  child 2 artifact and then become child 6 acceptance criteria.

## Checklist

### Goal Gates

- [ ] AC-1 through AC-17 demonstrated
- [ ] Every user story has a passing real entry-point test
- [ ] All six child specifications closed
- [ ] Shared CLI path has no duplicate semantic dispatcher
- [ ] Website artifact contains no forbidden host backend or process symbols
- [ ] BGP and BFD tests are mutation-proven against the real producers
- [ ] Browser storage failure and security paths fail closed
- [ ] Default native and appliance behavior preserved
- [ ] Documentation, discovery, and generated inventories updated
- [ ] Deferral shard resolved with no live row

### TDD

- [ ] Tests written before each child implementation
- [ ] Tests FAIL for the intended missing behavior
- [ ] Tests PASS after implementation
- [ ] Numeric and queue boundaries tested
- [ ] Real-browser functional tests cover the complete workflow
- [ ] Existing BGP/BFD interop suites remain green; no new wire behavior claimed

### Closure

- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section
- [ ] Independent `/ze-review` gate clean and recorded
- [ ] Goal validation names the browser commands and observed protocol state
- [ ] Learned outcomes routed to architecture docs and discovery surfaces
- [ ] Commit A contains each child's code, tests, docs, and child spec
- [ ] Commit B removes each closed child spec
- [ ] Final umbrella commit records integrated evidence
- [ ] `./le verify current mode full` passes before the final implementation commit
- [ ] Final cleanup commit removes this umbrella only after all children close
