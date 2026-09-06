# Spec: image-server-listen-interface-drops-entries

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | plugin |
| Depends | - |
| Phase | 1/2 |
| Handoff | - |
| Updated | 2026-09-06 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

**What the operator is promised.**
`internal/plugins/imageserver/yang/ze-image-server-conf.yang` declares
`leaf-list listen-interface` with the description "Interfaces to serve on". The
node is a `leaf-list`, and the description is plural. A reader takes both to
mean the image server serves on every interface named, which is what a
`leaf-list` means everywhere else in the schema. The sibling `listen-port` leaf
describes itself as bound "on the IPv4 address of the first listen-interface",
so the schema contradicts itself on the same page.

**What Ze does instead.** Every entry after the first is dropped with no log
line. `parseConfig` (`internal/plugins/imageserver/config.go`) collects all the
entries into `imageConfig.ListenInterfaces`. The `startServer` closure in
`runImageServerPlugin` (`internal/plugins/imageserver/register.go`) then tests
`len(cfg.ListenInterfaces) > 0` and calls `resolveInterfaceIPv4` on
`cfg.ListenInterfaces[0]` alone, assigns the result to `bindIP`, and builds one
`http.Server` on `bindIP` and `cfg.ListenPort`. Nothing reads index 1 or above,
and nothing warns that they exist. An operator naming two interfaces gets a
server on one of them and no indication which entries were ignored.

The sibling service disagrees. `internal/plugins/tftpserver/register.go` loops
`for _, ifName := range cfg.ListenInterfaces`, binds a listener per interface,
logs a failure per interface and continues, and reports
`tftpserver: no interfaces bound; server not serving` when none succeeded. The
two plugins parse the same shape of leaf-list into the same field name and
disagree about what a leaf-list means. An operator who configures both for a PXE
install gets TFTP on every interface and HTTP on one.

**What closing it means.** The implementer chooses between building the behavior
and refusing the leaf at commit the way `unimplementedVRFValidator`
(`internal/component/config/validators.go`) refuses `vrf`. Building it is
obviously right here, and the refusal is not available in its usual form,
because the leaf is not unimplemented: one entry works. The real choice is
between binding a listener per named interface, which is the shape
`internal/plugins/tftpserver/register.go` already carries and which makes the
`leaf-list` mean what it says, and narrowing the schema to a single `leaf`,
which makes the code honest and takes a capability away from the operator. The
first is the answer the sibling plugin argues for, since two services in one
daemon disagreeing about a leaf-list is a defect on its own. The design owes one
answer that covers both plugins, and if it narrows the schema it also owes the
migration for an operator who wrote two entries and committed them.

## Progress (2026-09-06)

The behavior is built and proven. The image server binds one listener for each
`listen-interface` entry, which is what `internal/plugins/tftpserver/register.go`
already does with the same leaf.

**Discrimination evidence.** The unit tests were run with `listenTargets` and
`startTargets` cut back to the first entry, which is the pre-fix behavior:

```
--- FAIL: TestServeTargetAdvertisesItsOwnAddress   startTargets bound 1 servers, want 2
--- FAIL: TestListenTargetsKeepsEveryEntry         listenTargets returned 1 targets, want 2
--- FAIL: TestStartTargetsBindsEveryTarget         startTargets bound 1 servers, want 2
--- FAIL: TestStartTargetsSkipsUnbindableTarget    startTargets bound 0 servers, want 1
--- PASS: TestListenTargetsWithNoNameBindsEveryAddress
```

The one test that stays green is the wildcard case, which the cut does not
reach. With the cut removed the whole package passes
(`ok github.com/ze-software/ze/internal/plugins/imageserver`).

## Required Reading

<!-- NEVER tick [ ] to [x] -- these checkboxes are template markers, not progress.
     Capture what you learned as -> Decision: / -> Constraint: annotations, which
     survive compaction; track reading progress in the session state file. -->

### Architecture Docs
- [ ] `docs/architecture/provisioning/image-server.md` - the page `register.go`
  declares in its `// Design:` header
  → Decision: the image server owns its own HTTP listener and imports no other
  provisioning plugin, so the listener set is decided inside this plugin alone
  → Constraint: the page is the surface that states how many listeners the leaf
  produces, so the behavior change carries the page edit in the same work
- [ ] `docs/functional-tests.md` - the `.ci` draft-then-promote workflow
  → Constraint: a `.ci` is iterated in `test/draft/<suite>/` and promoted by a
  plain `mv`, because `test/<suite>/` runs on every peer session's verify

### RFC Summaries (Scope: protocol)
- [ ] N-A - Scope is plugin. HTTP is served by `net/http`, and this spec changes
  how many listeners are opened, not what any of them puts on the wire.

**Key insights:** (minimal context to resume after compaction)
- The sibling services settle the design: `internal/plugins/tftpserver/register.go`
  (`startServer`) and the DHCP server read the same `listen-interface` leaf-list and
  bind one listener for each entry.
- `newMux` takes the bind address and `serveBootIPXE` writes it into the boot
  script, so a shared mux would send every client to one interface's address.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/plugins/imageserver/config.go` - `parseConfig` collects every
  `listen-interface` entry into `imageConfig.ListenInterfaces`, accepting both
  the JSON array and the single-string shape
- [ ] `internal/plugins/imageserver/register.go` - the `startServer` closure read
  `cfg.ListenInterfaces[0]` alone, resolved it, and built one `http.Server`
- [ ] `internal/plugins/imageserver/handler.go` - `newMux(cfg, zefsPath, serverAddr)`
  stores the bind address as `imageHandler.serverAddr`, and `serveBootIPXE`
  writes it into `/install/boot/boot.ipxe` as `ze.server=`
- [ ] `internal/plugins/tftpserver/register.go` - loops over
  `cfg.ListenInterfaces`, logs a per-interface failure and continues, and
  reports `no interfaces bound; server not serving` when none succeeded

**Behavior to preserve:** (unless the user explicitly said to change it)
- With no `listen-interface` entry, the port is bound on every address of the
  host and no boot script is built (`newMux` registers `/install/boot/boot.ipxe`
  only when `serverAddr` is not empty).
- A bind is synchronous, so a failure is reported instead of being logged as
  `started` from inside the serve goroutine.
- Every existing `.ci` in `test/install/`, including `image-server-config` and
  `image-resolve-failure`.

**Behavior to change:** (only what the user asked for)
- Every `listen-interface` entry gets its own listener, not the first alone.
- An entry that does not resolve, or that does not bind, is named in the log and
  skipped; the entries that work still serve.
- When nothing binds, the plugin logs `no interfaces bound; server not serving`
  instead of `started`.
- Each listener builds its own mux, so its boot script names its own address.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The operator writes `service { image-server { listen-interface <name>; ... } }`
  in the configuration and commits it.
- The hub sends the `service` section to the plugin as JSON, and the plugin
  receives it in `p.OnConfigure` in `runImageServerPlugin`
  (`internal/plugins/imageserver/register.go`).

### Transformation Path
1. `parseConfig` (`internal/plugins/imageserver/config.go`) turns the JSON into
   `imageConfig`, with every `listen-interface` entry in `ListenInterfaces`.
2. The `startServer` closure calls `listenTargets(cfg.ListenInterfaces, log)`,
   which resolves each entry through `resolveInterfaceIPv4` and returns one
   `listenTarget` for each entry that resolves.
3. `startTargets` calls `serveTarget` for each target, which builds that
   target's mux with `newMux(cfg, zefsPath, target.ip)`, binds the address
   synchronously, and serves it in its own goroutine.
4. `stopServer` closes every server in `httpServers` on reconfigure and on stop.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Hub ↔ plugin | `sdk.ConfigSection` JSON carrying the `service` subtree | Yes - `test/install/image-multi-interface.ci` drives a real daemon from a config |
| Plugin ↔ kernel | `net.ListenConfig.Listen` on each resolved address | Yes - `TestStartTargetsBindsEveryTarget` |
| Plugin ↔ iface component | `resolveInterfaceIPv4`, which falls back to a direct kernel lookup when no iface backend is loaded | Yes - `TestResolveInterfaceIPv4FallsBackWithoutBackend` (pre-existing) |

### Integration Points
- `resolveInterfaceIPv4` (`register.go`) - unchanged, called once per entry
  instead of once per configuration.
- `newMux` (`handler.go`) - unchanged, called once per listener so each one
  carries its own bind address.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | The config still reaches the listener through `OnConfigure` → `parseConfig` → `startServer`; only the fan-out inside `startServer` changed |
| No unintended coupling (components stay isolated) | Yes | No new import: the plugin still imports no other provisioning plugin, and it copies the TFTP server's shape rather than calling into it |
| No duplicated functionality (extends existing, does not recreate) | Yes | `resolveInterfaceIPv4` and `newMux` are reused; the new code is the loop around them |
| Zero-copy preserved where applicable (refs, not copies) | N-A | No wire encoding on this path; the listener set is one slice sized to the entry count |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | Yes | Everything added is unexported and lives in the plugin's own package; no core or shared file names the image server |

## Risks & Assumptions

<!-- LIVE: written during RESEARCH/DESIGN, statuses updated during implementation.
     Gate answers from /ze-spec (assumption challenge, Failure Mode Analysis)
     land HERE, not only in conversation. -->

### Assumptions
<!-- Every row needs a validation method. `unvalidated` is not a valid final
     status: closure re-checks each one. A broken assumption also gets a
     Mistake Log row and a Deviations entry. -->
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `leaf-list` means "serve on each", not "the first wins" | `startServer` in `internal/plugins/tftpserver/register.go` binds one listener per entry of the same leaf | The schema would have to narrow to a single `leaf`, with a migration for an operator who wrote two entries | Reading the sibling producer | confirmed |
| A-2 | Binding one port on several interface addresses does not collide | Each listener binds a distinct IPv4 address, and only the wildcard case binds every address | Two listeners on one address would fail the second bind | `TestServeTargetAdvertisesItsOwnAddress` binds 127.0.0.1 and 127.0.0.2 | confirmed |
| A-3 | Each listener must advertise its own address in the boot script | `newMux` stores `serverAddr`, and `serveBootIPXE` writes it as `ze.server=` | A client on the second network would be told to fetch the kernel from the first | `TestServeTargetAdvertisesItsOwnAddress` | confirmed |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | One unusable interface stops the whole image server, breaking an install that worked before | The plugin logs `listen failed` and serves nothing | A failed target is logged and skipped; only an empty listener set is an error (`TestStartTargetsSkipsUnbindableTarget`) |
| R-2 | A listener leaks across a reconfigure, holding the port | The next commit reports `address already in use` | `stopServer` closes every server in `httpServers` and clears the slice |
| R-3 | An operator reads `started` while nothing is bound | Install clients time out with no server-side error | The zero-listener guard logs `no interfaces bound; server not serving` and returns before the `started` line (`test/install/image-multi-interface.ci` rejects `imageserver: started`) |

## Blast Radius

<!-- What a wrong landing costs, and how to get out. A reviewer reads this first. -->
| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A bare-metal install stalls: PXE clients reach an HTTP server that is not listening on their network, or fetch a boot script naming an address they cannot route to. No running traffic is affected, because the image server is a provisioning service with its own listener. |
| How is it reverted? | A single commit revert. No config migration: a configuration with one entry behaves exactly as before, and the schema gained no leaf. |
| Who else touches this path? | `internal/plugins/tftpserver` and `internal/plugins/dhcpserver` read the same leaf name in their own packages, and are not edited here. |

## Wiring Test (MANDATORY -- NOT deferrable)

<!-- BLOCKING: proves the feature is reachable from its intended entry point.
     Without it the feature exists in isolation: unit tests pass, nothing calls it.
     Every row needs a concrete test name. "Deferred"/"TODO"/empty is rejected
     by `internal/le/hookruntime/lifecycle.go`, which is the point: an unedited row fails. -->
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A committed config naming two `listen-interface` entries, read by a running `ze` daemon | → | `startServer` → `listenTargets` → `startTargets` in `internal/plugins/imageserver/register.go` | `test/install/image-multi-interface.ci` (both entries named in the log, and `imageserver: started` rejected) |
| `imageConfig.ListenInterfaces` holding several entries | → | `listenTargets` | `TestListenTargetsKeepsEveryEntry` |
| A resolved target set | → | `startTargets` → `serveTarget` | `TestStartTargetsBindsEveryTarget` |

## Acceptance Criteria

<!-- Define BEFORE implementation. Each row is a testable assertion, stated as
     observable behavior, never as the mechanism used to reach it. -->
| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Two `listen-interface` entries, both resolving | The image server answers HTTP on both addresses, on the configured port |
| AC-2 | Two entries, the second naming a device that does not exist | The log names the second entry and the server still serves on the first |
| AC-3 | Two entries, neither of which binds | The log carries `no interfaces bound; server not serving` and never `imageserver: started` |
| AC-4 | No `listen-interface` entry | One listener on every address of the host, and no boot script endpoint, exactly as before |
| AC-5 | A client fetches `/install/boot/boot.ipxe` from the second interface | The script carries `ze.server=` set to that interface's address, not the first one's |

## End-to-End User Stories

<!-- One row per user-facing operation the feature enables. ACs verify that
     components work; stories verify the chain is connected. A broken link in a
     path is a spec gap: add the missing component to ACs, Files, and Test Plan
     before proceeding. Delete this section when Scope is tooling or docs. -->
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Names two install networks under `image-server` and commits | config -> `OnConfigure` -> `parseConfig` -> `startServer` -> `listenTargets` -> `startTargets` | `test/install/image-multi-interface.ci`, `TestStartTargetsBindsEveryTarget` |
| 2 | Boots a machine on the second install network | the second listener's mux -> `serveBootIPXE` -> `ze.server=` naming that listener's address | `TestServeTargetAdvertisesItsOwnAddress` |
| 3 | Mistypes one interface name and commits | `listenTargets` reports the entry and keeps the rest | `TestListenTargetsKeepsEveryEntry`, `TestStartTargetsSkipsUnbindableTarget` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestListenTargetsKeepsEveryEntry` | `internal/plugins/imageserver/listen_test.go` | AC-1, AC-2: every entry yields a target, and an unresolvable entry is logged and skipped | PASS |
| `TestListenTargetsWithNoNameBindsEveryAddress` | `internal/plugins/imageserver/listen_test.go` | AC-4: no entry yields one wildcard target | PASS |
| `TestStartTargetsBindsEveryTarget` | `internal/plugins/imageserver/listen_test.go` | AC-1: two targets yield two listeners, both serving | PASS |
| `TestStartTargetsSkipsUnbindableTarget` | `internal/plugins/imageserver/listen_test.go` | AC-2: a target that cannot bind is logged and skipped | PASS |
| `TestServeTargetAdvertisesItsOwnAddress` | `internal/plugins/imageserver/listen_linux_test.go` | AC-5: each listener's boot script names its own address | PASS |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `listen-port` | 1-65535 | 65535 | 0 | 65536 |

The range is unchanged by this spec and is already covered by
`TestParseImageConfigInvalid` (`port_below_min`, `port_above_max`). The entry
count of `listen-interface` has no numeric bound: zero entries is the wildcard
case and is covered by `TestListenTargetsWithNoNameBindsEveryAddress`.

### Functional Tests
<!-- REQUIRED: a unit test proves the algorithm, a .ci proves the user can reach
     the feature. New RPCs/APIs are never covered by unit tests alone.
     Structure: ai/patterns/functional-test.md -->
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `image-multi-interface` | `test/install/image-multi-interface.ci` | An operator names two interfaces; both are named in the log, and a configuration whose interfaces all fail reports `no interfaces bound` rather than `started` (AC-2, AC-3) | PASS |

### Interop Tests (Scope: protocol)
<!-- REQUIRED when wire-visible behavior changes. See
     ai/rules/interop-and-goal-validation.md, including the vacuity traps: prove
     the test FAILS when the behavior under test is reverted. -->
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | Scope is plugin. The change is how many `net/http` listeners are opened and which address each one advertises. Nothing on the wire changes format, so there is no peer implementation to disagree with. The end-to-end path is covered by `test/install/image-multi-interface.ci`. | N-A |

## Files to Modify
<!-- MUST include feature code (internal/*, cmd/*), not only test files.
     Check each file's // Design: annotation: if the change alters behavior the
     referenced architecture doc describes, list that doc here too. -->
- `internal/plugins/imageserver/register.go` - `startServer` binds one listener
  per entry through the new `listenTargets`, `startTargets`, `serveTarget` and
  `listenAddr`; `stopServer` closes every listener
- `internal/plugins/imageserver/yang/ze-image-server-conf.yang` - the
  `listen-interface` and `listen-port` help text now describes one listener per
  entry instead of the first entry
- `docs/architecture/provisioning/image-server.md` - two Decisions rows: one
  listener per entry, and each listener advertising its own address

## Files to Create
- `internal/plugins/imageserver/listen_test.go` - unit tests for the target
  resolution and the bind loop
- `internal/plugins/imageserver/listen_linux_test.go` - the two-address test,
  which needs the 127.0.0.0/8 range Linux carries without configuration
- `test/install/image-multi-interface.ci` - functional test for end-user behavior

### Integration Checklist
<!-- Answer every row Yes / No / N-A. Never leave a bare marker: an unanswered
     row is indistinguishable from a forgotten one. N-A needs a reason. -->
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/imageserver/yang/ze-image-server-conf.yang`: no leaf added or removed, and the `ze:help` on `listen-interface` and `listen-port` now states one listener per entry |
| YANG validation constraints | No | No leaf changed type or bound. `listen-port` keeps `range "1..65535"`, and `listen-interface` stays a string leaf-list, as the TFTP and DHCP servers declare it |
| YANG custom validators | No | An interface that does not exist is an operational fact, not a config error: the TFTP and DHCP servers accept the same name and report the failure at bind time. A commit-time validator would refuse a configuration that is correct for a machine whose interface is not up yet |
| CLI commands/flags | No | No command added; the surface is a config leaf that already exists |
| CLI grammar (keyword before value) | N-A | No command added |
| Editor autocomplete | No | Unchanged: `listen-interface` is a free string leaf-list, exactly as before |
| Functional test for new RPC/API | Yes | `test/install/image-multi-interface.ci` |
| Pipe completeness | N-A | No command output added |
| Env var registration | No | No leaf under `environment/` |
| Doctor check for runtime dependencies | No | `extractImageListeners` (`internal/component/doctor/checks_listener.go`) checks `0.0.0.0` and the port, ignoring `listen-interface`, as `extractTFTPListeners` does for a service that already binds per interface. This spec adds no dependency of a new kind |
| Prometheus counters/metrics | No | No counter added. The listener count is reported in the `imageserver: started` log line |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
<!-- Answer every row Yes / No / N-A. A No must be backed by a source-aware
     check, not a guess: at minimum grep docs/ for source anchors pointing at the
     files you changed. Any factual doc change carries a source anchor. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | No new feature: a leaf-list that dropped entries now serves them all. `docs/features.md` names no listener count |
| 2 | Config syntax changed? | No | No leaf added, removed, or retyped. The `ze:help` text on the two affected leaves is updated in the YANG module, which is where the editor reads it |
| 3 | CLI command added/changed? | No | No command touched |
| 4 | API/RPC added/changed? | No | No RPC touched |
| 5 | Plugin added/changed? | Yes | `docs/architecture/provisioning/image-server.md`, the page `register.go` declares. `docs/guide/plugins.md` describes the plugin mechanism, not this behavior |
| 6 | Has a user guide page? | No | `docs/guide/ze-install.md` covers the install flow and never mentions `listen-interface`; verified by grep |
| 7 | Wire format changed? | No | HTTP served by `net/http`, unchanged |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface touched |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC governs how many interfaces a local HTTP service binds |
| 10 | Test infrastructure changed? | No | The new `.ci` uses the existing `await=`/`expect=`/`reject=` grammar |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares routing daemons |
| 12 | Internal architecture changed? | Yes | `docs/architecture/provisioning/image-server.md` gains the listener-per-entry decision and the per-listener boot script decision |
| 13 | Route metadata keys added/changed? | N-A | No route metadata on this path |
| 14 | Prometheus counters added/changed? | No | No counter added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | The plugin's registration, name, and command surface are unchanged |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | `docs/architecture/provisioning/image-server.md` anchors `register.go` for "plugin registration and listener lifecycle" and is updated here. `docs/DESIGN.md` anchors the same file only to list the plugin in the registry inventory, which this change leaves correct |
| 17 | Existing docs show config/CLI/API examples for this area? | No | The `image-server` examples in `test/install/*.ci` and in the page are single-interface or interface-free, and both still describe what Ze does |

## Implementation Steps

<!-- Concrete phases of work, not a restatement of the /ze-implement stages
     (those live in the skill). Phase 1 is ALWAYS wiring. Order by dependency:
     schema before resolution, resolution before CLI. Each phase follows TDD
     (write test -> fail -> implement -> pass) and ends with a self-critical
     review; fix what it finds before starting the next phase. -->

1. **Phase: Wiring (MANDATORY FIRST)** -- prove the config entries reach the
   listener code
   - Tests: `test/install/image-multi-interface.ci`, `TestListenTargetsKeepsEveryEntry`
   - Files: `internal/plugins/imageserver/register.go`,
     `test/draft/install/image-multi-interface.ci`
   - Verify: the `.ci` names both entries; against the old code the second entry
     never appears in the log
2. **Phase: One listener per entry** -- resolve each entry, bind each target,
   skip and report the ones that fail
   - Tests: `TestStartTargetsBindsEveryTarget`, `TestStartTargetsSkipsUnbindableTarget`,
     `TestListenTargetsWithNoNameBindsEveryAddress`
   - Files: `internal/plugins/imageserver/register.go` (`listenTargets`,
     `startTargets`, `serveTarget`, `listenAddr`, `stopServer`)
   - Verify: tests fail with the first-entry-only code, pass with the loop
3. **Phase: The boot script names its own listener** -- one mux per target
   - Tests: `TestServeTargetAdvertisesItsOwnAddress`
   - Files: `internal/plugins/imageserver/register.go` (`serveTarget` calls
     `newMux` per target)
   - Verify: each listener's `/install/boot/boot.ipxe` carries its own
     `ze.server=`
4. **Phase: Schema and page** -- the help text and the architecture page state
   the new behavior
   - Files: `internal/plugins/imageserver/yang/ze-image-server-conf.yang`,
     `docs/architecture/provisioning/image-server.md`
   - Verify: neither surface still says the first entry wins

### Critical Review Checklist

<!-- Feature-SPECIFIC checks. The generic ones in ai/rules/quality.md always
     apply and are not repeated here. A row that would read the same on any spec
     is not worth a row. -->
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation in `startServer`, `listenTargets`, `startTargets`, or `serveTarget` |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | A failed resolve and a failed bind are both skipped rather than fatal, and an empty listener set is fatal. No listener is left running after `stopServer` |
| Naming | The log keys match the TFTP server's for the same events: `interface`, `interfaces`, `listeners`, and the `no interfaces bound; server not serving` sentence |
| Data flow | Each mux carries its own bind address; no listener reads another target's address |
| Rule: `ai/rules/principles.md` | The zero-listener case says so instead of logging `started` over a server that serves nothing |
| Rule: `ai/rules/goroutine-lifecycle.md` | Each serve goroutine ends when its own server is closed, and `stopServer` closes every server in the slice |

### Deliverables Checklist

<!-- Every deliverable with a command that proves it. "Looks done" is not a
     verification method. -->
| Deliverable | Verification method |
|-------------|---------------------|
| One listener per `listen-interface` entry | `go test ./internal/plugins/imageserver/ -run TestStartTargetsBindsEveryTarget` |
| Every entry reaches the code from a real config | `ze-test install --pattern image-multi-interface` |
| The schema no longer promises first-entry-only | `grep -n "FIRST interface" internal/plugins/imageserver/yang/ze-image-server-conf.yang` returns nothing |
| The page states the new behavior | `grep -n "One listener for each" docs/architecture/provisioning/image-server.md` |

### Security Review Checklist

<!-- Feature-specific: untrusted input, injection, resource exhaustion, error
     leakage, authorization that could fail open. -->
| Check | What to look for |
|-------|-----------------|
| Input validation | `listen-interface` entries come from the operator's own committed config, not from the network. An entry that names no device is refused by `resolveInterfaceIPv4` and never reaches `net.Listen` |
| Exposure surface | Each entry adds one bound address. An operator who names two interfaces asks for two, which is the point of the leaf-list; nothing binds an address the operator did not name, and the wildcard case is unchanged |
| Resource exhaustion | The listener count is bounded by the number of entries in the committed config, and each listener keeps the existing read, write, header and body limits from `serveTarget` |
| Error leakage | A resolve or bind failure is logged with the interface name and the OS error, which the operator already owns; nothing reaches an HTTP response |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights
<!-- LIVE: write immediately when you learn something. At closure these route to
     a subsystem arch doc, a rule, or the learned summary. -->
- Two services in one daemon reading the same leaf name and disagreeing about
  what it means is a defect on its own. The sibling that already loops is the
  design answer, and copying its log vocabulary is part of the fix: an operator
  reads one message for one event across the provisioning services.
- The bind address is not only a socket parameter here. It is content: the boot
  script tells the client where to fetch the kernel, so a listener that
  advertises another listener's address serves a script that cannot work.
- A `.ci` whose interfaces all fail to resolve needs no platform gating and no
  real interface. `resolveInterfaceIPv4` falls back to a direct kernel lookup,
  which fails for a device that does not exist on every platform.

## Key Design Decisions
<!-- "Chose X over Y because Z." The rejected alternative is the valuable half. -->
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bind one listener per entry | Narrow the schema to a single `leaf` | The sibling services settle it: the TFTP server and the DHCP server bind one listener per entry of the same leaf name. Narrowing would take a capability away from an operator provisioning two install networks, and would owe a migration for a committed configuration with two entries |
| Skip a failed entry and keep serving | Refuse the whole configuration when one entry fails | One mistyped name would stop an install that the other interface can serve. This is the TFTP server's behavior for the same failure, and the empty result is still an error |
| One mux per listener | One shared mux built from the first address | `newMux` stores the bind address, and `serveBootIPXE` writes it into the boot script as `ze.server=`. A shared mux would send a client on the second network to the first network's address |
| Report the empty set as `no interfaces bound; server not serving` | Log `started` with no listeners | A daemon that logs `started` while serving nothing is the failure this plugin already had at bind time. The sentence is the TFTP server's, word for word, so an operator reading both services' logs reads one message |

## Known Limitations
<!-- Deliberate scope boundaries. Anything here that is actually outstanding work
     is not a limitation: write it as its own spec, in the bucket that item
     belongs to, and name that spec here (ai/rules/planning.md). -->
- IPv6 is not served. `resolveInterfaceIPv4` returns the interface's first IPv4
  address, which is what the image server bound before this spec and what the
  boot script needs. Serving an install over IPv6 is a separate capability and
  is not narrowed by this change.
- An interface that gains its address after the commit is not re-resolved. The
  resolve happens once per commit, as it did before, and as the TFTP server
  does. Watching for a late address would change both plugins.

## RFC Documentation (Scope: protocol)

N-A. Scope is plugin. No RFC governs how many local addresses an HTTP service
binds, and the HTTP surface itself is `net/http` and is unchanged.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
