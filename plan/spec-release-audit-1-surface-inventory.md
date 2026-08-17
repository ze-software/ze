# Spec: release-audit-1-surface-inventory

| Field | Value |
|-------|-------|
| Status | design |
| Depends | spec-release-audit-0-umbrella.md |
| Phase | - |
| Updated | 2026-05-24 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file
2. `plan/spec-release-audit-0-umbrella.md` - release audit blocker policy and child audit map
3. `ai/patterns/registration.md` - canonical registry inventory mechanisms
4. `ai/patterns/cli-command.md` - CLI command surface patterns
5. `ai/patterns/web-endpoint.md` - web and LG route surface patterns
6. `ai/patterns/functional-test.md` - functional test suite mapping
7. `cmd/ze/main.go` - root command dispatch and fallback
8. `internal/component/plugin/all/all.go` - generated plugin and schema aggregate imports
9. `mk/test-functional.mk` and `mk/test-release.mk` - release test evidence surfaces

## Task

Build the first release-audit inventory: every user-visible surface must be identified,
grouped, assigned to a child audit, and mapped to available evidence. This inventory is the
index used by the later audit passes. It must derive from the codebase, registries, schemas,
routes, plugins, and tests rather than from memory or documentation alone.

This spec does not fix product bugs and must not change product code. It records release
surfaces, evidence gaps, verified inventory findings, and direction for the future fix owner.

## Audit Scope Boundary

This spec documents issues only. It may run read-only or verification commands to prove a
finding, but it does not edit source, tests, schemas, docs, Makefiles, or generated files.

Each finding should include:
- The observed issue.
- The user or release impact.
- Evidence proving the issue exists.
- The likely owning child audit or subsystem.
- Suggested direction for a future fix.
- Verification the future fix should provide.

## Required Reading

### Architecture Docs and Rules

- [ ] `plan/spec-release-audit-0-umbrella.md` - release audit structure and blocker policy
  -> Decision: inventory is the first child audit and owns the release surface matrix
  -> Constraint: every later child audit must map back to an inventory row
- [ ] `ai/patterns/registration.md` - registration architecture and startup flow
  -> Decision: derive plugins, schemas, env vars, RPC commands, and CLI roots from registries
  -> Constraint: web routes and route metadata have no central registry and need source inspection
- [ ] `ai/patterns/cli-command.md` - offline and online command model
  -> Decision: inventory must split static/offline CLI commands from online YANG/RPC commands
  -> Constraint: command output surfaces later require pipe completeness review
- [ ] `ai/patterns/web-endpoint.md` - web, LG, and chaos web route model
  -> Decision: web inventory must include auth, HTMX mutation routes, SSE, and JSON negotiation
  -> Constraint: web endpoints need functional tests in `test/web/` or justified coverage elsewhere
- [ ] `ai/patterns/functional-test.md` - test directories and runners
  -> Decision: release matrix must distinguish gated and release-evidence-only suites
  -> Constraint: user-facing behavior cannot close with unit tests alone
- [ ] `ai/rules/config.md` - config unknown-key, environment, listener rules
  -> Constraint: YANG config surfaces must later verify exact-or-reject behavior and env registration
- [ ] `ai/rules/cli.md` - command output pipe obligations
  -> Constraint: command inventory must identify output-producing commands for later pipe audit

### Source Files

- [ ] `cmd/ze/main.go` - root dispatch, YANG verb dispatch, static command switch, registry fallback, unknown-command suggestions
  -> Decision: root command inventory must include both static switch commands and `cmdregistry` local commands
  -> Constraint: help, dispatch, and suggestions can drift because they are not one single derived source
- [ ] `cmd/ze/internal/cmdregistry/registry.go` - registered root and local command registry
  -> Decision: registry-backed local commands are release-facing even when absent from the static switch
- [ ] `cmd/ze/*/register.go` - offline command metadata and local command aliases
  -> Decision: root help inventory derives from register packages, not only `main.go`
- [ ] `internal/component/plugin/all/all.go` - generated aggregate import list for schemas, plugins, event namespaces, RPC commands
  -> Decision: plugin and schema surfaces derive from aggregate imports plus registration tests
  -> Constraint: generated import drift is release-impacting when a registered plugin is omitted or a test expectation is stale
- [ ] `internal/component/plugin/all/all_test.go` - aggregate registration expectations
  -> Decision: `TestAllPluginsRegistered` is a release inventory gate
- [ ] `internal/component/plugin/server/rpc_register.go` - RPC registration mechanism
  -> Decision: online command inventory must diff YANG `ze:command` methods against registered wire methods
- [ ] `mk/test-functional.mk` - gated functional test suites
  -> Decision: `ze-functional-test` gates 12 suites, while extra suites remain release evidence only
- [ ] `mk/test-release.mk` - full release evidence matrix
  -> Decision: release inventory must include evidence categories from `ze-evidence-release-verify`

**Key insights:**
- Ze has several registration layers, but no single source covers all user-visible surfaces.
- Root CLI dispatch currently combines YANG verbs, a static switch, file/config startup, and registry fallback.
- Online command wiring has two sources that can drift: YANG command/API schemas and `pluginserver.RegisterRPCs` handlers.
- Web and LG routes have no central registry, so audit inventory must inspect route wiring.
- The release evidence matrix already exists in `mk/test-release.mk` in this worktree, while `ze-precommit-verify` remains the fast gate.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `cmd/ze/main.go` - dispatches YANG verbs `show`, `set`, `del`, `update`, `validate`, `monitor`, `clear`; static roots include `bgp`, `plugin`, `cli`, `config`, `init`, `passwd`, `data`, `schema`, `yang`, `interface`, `firewall`, `traffic-control`, `remote`, `resolve`, `exabgp`, `signal`, `status`, `env`, `sysctl`, `tacacs`, `l2tp`, `appliance`, `service`, `install`, `uninstall`, `completion`, `start`, `version`, `help`, `update-serve`, `--plugins`
- [ ] `cmd/ze/*/register.go` - many roots and local aliases register metadata, including `service`, `install`, `uninstall`, `skills`, `plugin`, `completion`, `bgp`, `explain`, `resolve`, `diag`, `crashes`, `appliance`, `host`, `env`, `signal`, `doctor`, `init`, `tacacs`, `l2tp`, `sysctl`, `traffic-control`, `firewall`, `interface`, `passwd`, `exabgp`, `data`, `yang`, `schema`, `config`, `cli`
- [ ] `cmd/ze/remote/register.go` - not present, while `cmd/ze/main.go` statically dispatches `remote`
- [ ] `internal/component/plugin/all/all.go` - generated aggregate imports schemas, plugin packages, event namespaces, and RPC command packages
- [ ] `internal/component/plugin/all/all_test.go` - expected plugin list includes `ike` and excludes `bgp-filter-remove-private-as`
- [ ] `internal/component/ike/engine/register.go` - registers plugin name `ike`; current aggregate test did not fail on `ike`, likely because another imported command path pulls it in indirectly
- [ ] `internal/component/plugin/server/rpc_register.go` - `RegisterRPCs` appends online command handlers for later wiring
- [ ] `mk/test-functional.mk` - `ze-functional-test` runs encode, plugin, parse, decode, reload, ui, editor, managed, l2tp, firewall, policy, web
- [ ] `mk/test-release.mk` - `ze-evidence-release-verify` composes verify, chaos, fuzz, interop, ipsec-interop, l2tp-interop, functional-extra, perf, qemu, vpp-deployment, live

**Behavior to preserve:**
- `make ze-precommit-verify` remains fast and stays distinct from heavyweight release evidence.
- `cmdregistry` remains the registry for root metadata and local/offline commands.
- Generated `internal/component/plugin/all/all.go` remains the aggregate import point for production registrations.
- YANG remains the source of truth for online command tree shape.
- Web routes remain directly wired until a separate design changes that pattern.

**Audit documentation goal:**
- Release inventory must become explicit and tracked in this spec.
- Each surface row must map to a later audit owner and evidence category.
- Confirmed inventory failures must be recorded as findings with reproduction, owner, and suggested direction.
- Later child audits must not rediscover the surface from scratch; they should start from this inventory.

## Data Flow (MANDATORY)

### Entry Point

Inventory data enters from code and tests:
- `cmd/ze/main.go` and `cmd/ze/*/register.go` for CLI roots and local commands.
- YANG command/API schemas and `pluginserver.RegisterRPCs` for online commands.
- `internal/component/plugin/all/all.go`, plugin `register.go` files, and schema packages for plugin/config surfaces.
- Web, LG, REST, gRPC, MCP, and SSH route/listener setup for network surfaces.
- `mk/test*.mk`, `cmd/ze-test`, and `test/` directories for evidence surfaces.

### Transformation Path

1. Read root dispatch and command registry sources.
2. Group CLI commands into file/config startup, YANG verbs, static roots, and registry fallback locals.
3. Group online RPC commands by YANG command schema and handler registration package.
4. Group plugins by aggregate import and registration name.
5. Group config schemas by YANG module and config root.
6. Group network surfaces by listener and route family.
7. Group tests by gated, release-evidence-only, interop, deployment, fuzz, chaos, perf, and docs/onboarding evidence.
8. Emit a matrix row per surface with audit owner, evidence source, and inventory risk.

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| CLI argv -> dispatch | YANG verb path, static switch, registry fallback | `cmd/ze/main.go`, `cmd/ze/*/register.go` |
| YANG command -> RPC handler | `ze:command` method names mapped to `RegisterRPCs` | `internal/component/plugin/server/rpc_register.go`, command schemas |
| Aggregate import -> plugin registry | generated blank imports run `init()` registration | `internal/component/plugin/all/all.go`, `all_test.go` |
| Config schema -> runtime config | YANG modules loaded by schema registrations | `internal/**/yang/register.go`, config tests |
| HTTP route -> handler | direct route registration in hub/server code | web/API/LG/MCP route files |
| Test target -> test directory | Make target invokes `bin/ze-test` or scripts | `mk/test-functional.mk`, `mk/test-release.mk` |

### Integration Points

- `spec-release-audit-2-bgp-protocol.md` consumes BGP protocol, NLRI, command, interop, and ExaBGP rows.
- `spec-release-audit-3-config-cli.md` consumes root CLI, online command, config schema, editor, completion, and pipe rows.
- `spec-release-audit-4-web-lg-api.md` consumes web, LG, REST, gRPC, MCP, SSH, auth, and streaming rows.
- `spec-release-audit-5-plugins-rib.md` consumes plugin registry, startup, RIB, route-selection, and process protocol rows.
- `spec-release-audit-6-system-linux.md` consumes iface, FIB, firewall, VPP, traffic, PPPoE, L2TP, IPsec, kernel, and QEMU rows.
- `spec-release-audit-7-resilience-security.md` consumes authz, no-auth defaults, shutdown, reload, race, fuzz, chaos, resource, and secret rows.
- `spec-release-audit-8-docs-onboarding.md` consumes docs, install, examples, help, OpenAPI, and first-run rows.

### Architectural Verification

- [ ] No bypassed layers: each row names the source mechanism used to expose the feature.
- [ ] No unintended coupling: inventory records current wiring without proposing new architecture.
- [ ] No duplicated functionality: later automation should derive rows from registries where possible.
- [ ] Zero-copy preserved where applicable: BGP protocol rows route to protocol audit, not this inventory spec.

## Release Surface Matrix

### Binaries

| Surface | Source | Evidence | Owner Audit | Inventory Status |
|---------|--------|----------|-------------|------------------|
| `ze` | `cmd/ze/` | `make ze-precommit-verify`, `test/ui`, `test/plugin`, `test/web`, `test/install` | config/CLI, web/API, plugins | Inventory started |
| `ze-test` | `cmd/ze-test/` | all functional and evidence targets | surface inventory, test evidence | Inventory started |
| `ze-perf` | `cmd/ze-perf/` | `make ze-evidence-perf-record`, `test/perf` | resilience/security, release evidence | Inventory started |
| `ze-analyse` | `cmd/ze-analyse/` | unit/tests to be verified | docs/onboarding or protocol | Needs child audit row |
| `ze-chaos` | `cmd/ze-chaos/` | `make ze-chaos-test`, `test/chaos-web` | resilience/security | Inventory started |

### CLI Root Surfaces

| Surface Group | Source | Representative Commands | Evidence | Owner Audit | Risk |
|---------------|--------|-------------------------|----------|-------------|------|
| Daemon startup | `cmd/ze/main.go` | `ze <config>`, `ze -f <file>`, `ze start` | parse/plugin/web tests, install/docs tests | config/CLI, docs | Needs first-run walkthrough |
| YANG verbs | `cmd/ze/main.go`, YANG command tree | `show`, `set`, `del`, `update`, `validate`, `monitor`, `clear` | `test/ui`, `test/editor`, `test/plugin` | config/CLI | Needs complete command vs handler diff |
| Static offline roots | `cmd/ze/main.go` | `bgp`, `plugin`, `cli`, `config`, `init`, `passwd`, `data`, `schema`, `yang`, `interface`, `firewall`, `traffic-control`, `remote`, `resolve`, `exabgp`, `signal`, `status`, `env`, `sysctl`, `tacacs`, `l2tp`, `appliance`, `service`, `install`, `uninstall`, `completion` | mostly `test/ui`, `test/install`, package tests | config/CLI, system-linux, docs | Help/dispatch drift candidates |
| Registry local commands | `cmd/ze/*/register.go`, `cmdregistry` | `doctor`, `explain`, `skills`, `crashes`, `host`, `ping`, `generate wireguard keypair`, `show env`, `show data`, `show yang`, `show schema`, `show config` | `test/ui` | config/CLI, docs | Must verify help and completion visibility |
| Global metadata | `cmd/ze/main.go`, `cmdregistry` | `help`, `help --ai`, `version`, `--plugins`, `completion` | `test/ui/help-ai-json.ci`, command contract tests | docs/onboarding | Unknown-command suggestions are manually listed |

### Online RPC and YANG Command Surfaces

| Surface Group | Source | Representative Methods | Evidence | Owner Audit | Risk |
|---------------|--------|------------------------|----------|-------------|------|
| Core show | `internal/component/cmd/show/schema`, `internal/component/cmd/show` | version, uptime, system-memory, ping, traceroute, dns-lookup, interface, ip-route, kernel-routes, bmp, rr, vpn-ipsec | unit tests, `test/plugin/show-system-*`, `test/ui` | config/CLI, web/API | Needs full YANG command vs handler diff |
| Core clear | `internal/component/cmd/clear/schema`, `internal/component/cmd/clear` | interface-counters, dns-cache, vpn-ipsec-sa | unit tests to verify | config/CLI, system-linux | Mutation/authz coverage needed |
| BGP peer/update/raw | `internal/component/bgp/plugins/cmd/{peer,update,raw}` | peer list/detail/teardown/pause/resume/flush/history, peer-update, peer-raw | `test/plugin/api-peer-*`, handler tests | BGP protocol, config/CLI | Write operations need authz and selector coverage |
| RIB commands | `internal/component/bgp/plugins/cmd/rib` | status, routes, best, clear-in, inject, withdraw | `test/plugin/api-rib-*`, rib tests | plugins/RIB | Route correctness and mutation coverage needed |
| Monitor commands | `internal/component/bgp/plugins/cmd/monitor` | BGP events, ping, traceroute, interface-rate, netlink, vpn-ipsec | `test/plugin/monitor-*` | resilience/security, config/CLI | Known pipe-completeness risks for monitor log paths |
| Support commands | `internal/component/cmd/{meta,log,metrics,subscribe,bfd,l2tp,pppoe,archive,update}` | command list/help/complete, event list, log, metrics, subscribe, BFD, L2TP, PPPoE | mixed unit and functional tests | config/CLI, system-linux | Needs route-by-route inventory |

### Plugin and Schema Surfaces

| Surface Group | Source | Examples | Evidence | Owner Audit | Risk |
|---------------|--------|----------|----------|-------------|------|
| BGP core/plugin services | `internal/component/bgp/plugin`, `internal/component/bgp/plugins` | bgp, rib, adj-rib-in, persist, watchdog, BMP, RPKI, healthcheck, GR, route-refresh, role, hostname, softver, LLNH, AIGP | BGP tests, plugin tests, interop | BGP protocol, plugins/RIB | Needs per-plugin feature matrix |
| BGP filters | `internal/component/bgp/plugins/filter_*` | prefix, aspath, community, community-match, modify, remove-private-as | unit tests, plugin tests | BGP protocol, plugins/RIB | `bgp-filter-remove-private-as` registration test failure verified |
| BGP NLRI | `internal/component/bgp/plugins/nlri/*` | EVPN, FlowSpec, labeled, LS, MUP, MVPN, RTC, VPLS, VPN | encode/decode, ExaBGP, interop | BGP protocol | Needs family-by-family interop coverage |
| Component-backed plugins | `internal/component/{iface,traffic,vpp,firewall,ike}` | interface, traffic, vpp, firewall, ike | system tests, QEMU/deployment | system-linux | Generated aggregate and dependency ordering need audit |
| Generic plugins | `internal/plugins/` | bfd, connected, dhcpserver, fib, firewall backends, iface backends, imageserver, kernel, L2TP helpers, ntp, policy-routes, routing-table, static, sysctl, sysrib, tftpserver, traffic backends | plugin/unit/functional/QEMU | plugins/RIB, system-linux | Backend and platform coverage varies |
| Config schemas | `internal/**/yang/*.yang` | bgp, interface, firewall, traffic-control, vpp, system, environment, plugin, telemetry, pki, vpn, pppoe, l2tp, redistribute, services | parse/config tests, functional tests | config/CLI, system-linux | Env/listener/schema validation audit needed |

### Network and UI Surfaces

| Surface Group | Source | Representative Routes or Services | Evidence | Owner Audit | Risk |
|---------------|--------|-----------------------------------|----------|-------------|------|
| Web UI | `internal/component/web`, `cmd/ze/hub/main_servers.go` | login, assets, events SSE, admin, CLI, config set/add/rename/delete/commit/discard/diff, tools, L2TP pages, portal, health, show/monitor | `test/web`, `test/plugin/web-*` | web/LG/API, resilience/security | Broad route/auth/CSRF coverage needs matrix |
| Looking Glass | `internal/component/lg` | Birdwatcher API, peers, search, route detail, download, events SSE, graph, help | BMP/LG plugin tests, web tests to verify | web/LG/API | Birdwatcher route coverage appears incomplete |
| REST API | `internal/component/api/rest` | commands, execute, stream, peers, RIB, system, config sessions, OpenAPI, docs | `test/plugin/rest-*` | web/LG/API, resilience/security | Config sessions, stream, docs, read-only/write auth need coverage |
| gRPC API | `internal/component/api/grpc`, `api/proto/ze.proto` | Execute, Stream, ListCommands, DescribeCommand, Complete, config session RPCs | `test/plugin/grpc-execute.ci`, unit tests | web/LG/API | Functional coverage appears execute-focused |
| MCP | `internal/component/mcp` | `/mcp` POST/GET/DELETE/OPTIONS, OAuth metadata, streamable sessions, tools/tasks/elicitation | `test/plugin/mcp-*`, `task-*`, `elicitation-*` | web/LG/API, resilience/security | Raw HTTP/OAuth/CORS route coverage needs audit |
| SSH | `internal/component/ssh`, hub infra | interactive CLI, exec commands, streaming, plugin protocol, lifecycle commands | `test/plugin/ssh-*`, authz/tacacs tests | web/LG/API, resilience/security | Multi-listener and lifecycle auth need audit |

### Test and Evidence Surfaces

| Surface Group | Source | Contents | Owner Audit | Inventory Status |
|---------------|--------|----------|-------------|------------------|
| Fast gate | `make ze-precommit-verify` | lint, vet evidence, cached unit, race-on-changed, functional, ExaBGP | all | Needs current run before release decision |
| Functional gate | `mk/test-functional.mk` | encode, plugin, parse, decode, reload, ui, editor, managed, l2tp, firewall, policy, web | all | Inventory started |
| Extra functional | `mk/test-functional.mk`, `mk/test-release.mk` | static, traffic, vpp, l2tp-wire | system-linux, plugins/RIB | Release evidence only |
| Interop/deployment | `mk/test-integration.mk`, `test/interop`, `test/ipsec-interop`, `test/l2tp-interop` | BGP interop, IPsec interop, L2TP PPP Docker, VPP deployment, QEMU | BGP protocol, system-linux | Release evidence only |
| Fuzz | `mk/test-fuzz.mk` | fuzz targets for parsers/wire paths | resilience/security | Release evidence only |
| Chaos | `mk/test-chaos.mk`, `test/chaos-web` | chaos unit, functional, web | resilience/security | Release evidence only |
| Perf | `mk/perf.mk`, `test/perf` | benchmark and regression history | resilience/security | Release evidence only |
| Install/onboarding | `test/install`, docs | install/uninstall and generated config tests | docs/onboarding | Not in default functional gate according to research |

## Initial Findings

| ID | Severity | Surface | File/line | User Impact | Reproduction | Expected | Actual | Suggested Direction | Owner |
|----|----------|---------|-----------|-------------|--------------|----------|--------|---------------------|-------|
| ~~RA-INV-001~~ (struck 2026-07-10: no longer reproducible, see Post-wave corrections) | ~~Major~~ | ~~plugin registry~~ | ~~`internal/component/plugin/all/all_test.go`~~ | ~~Plugin aggregate registration gate fails, reducing confidence that generated imports and expected registry are synchronized~~ | ~~`go test ./internal/component/plugin/all -run TestAllPluginsRegistered -count=1`~~ | ~~Test passes with every registered production plugin in the expected list~~ | ~~Fails with unexpected plugin `bgp-filter-remove-private-as`~~ (name check moved to TestRegisteredPluginNames, `all_test.go`; snapshot includes the plugin, `testdata/plugins.snapshot:18`; both tests pass 2026-07-10) | ~~Future fix should reconcile the expected plugin inventory with generated registrations and keep the aggregate test as the guard~~ | ~~`spec-release-audit-5-plugins-rib.md`~~ |
| RA-CLI-001 | Major | CLI root UX | `cmd/ze/main.go` | Unknown-command hints can omit valid commands and mislead users | Inspect static suggestion list against dispatch/register roots | Suggestions are derived from the same registry/dispatch inventory as actual commands | Manual suggestion list omits several discovered roots | Future fix should derive suggestions from the same command inventory used by dispatch/help and add UI coverage | `spec-release-audit-3-config-cli.md` |
| RA-CLI-002 | Minor | CLI root help | `cmd/ze/main.go`, missing `cmd/ze/remote/register.go` | `remote` may dispatch but be absent from root help metadata | Compare static switch with `cmd/ze/*/register.go` | Every dispatched root appears in help/AI contract metadata or is intentionally hidden | `remote` has static dispatch and no discovered register file | Future fix should either register `remote` metadata or explicitly document it as hidden/internal, with command-contract evidence | `spec-release-audit-3-config-cli.md` |
| RA-RPC-001 | Major | online commands | YANG command/API schemas and RPC registrations | Some CLI/API command methods may appear in schema but lack matching registered handler, or vice versa | Generate diff of YANG `ze:command`/RPC methods vs `RegisterRPCs` wire methods | Every YANG command maps to a registered handler and every handler maps to a YANG path | Research found drift candidates in update, log, metrics, and peer API method names | Future fix should add a generated or test-backed diff between YANG methods and registered RPC methods | `spec-release-audit-3-config-cli.md` |
| RA-WEB-001 | Major | REST/gRPC/MCP/LG coverage | `internal/component/api`, `internal/component/mcp`, `internal/component/lg` | Network API users may hit untested routes, auth modes, or streaming behavior | Map all routes/RPCs to functional tests | Every public route/RPC has happy-path, auth, and error coverage | Coverage appears concentrated on execute/basic paths, not full route matrix | Future fix should produce a route/RPC coverage matrix and add missing route-level evidence | `spec-release-audit-4-web-lg-api.md` |
| RA-TEST-001 | Major | test gate inventory | `test/install`, `test/ipsec`, `test/pppoe`, Makefiles | Existing release-relevant tests may not run in any release gate | Compare `test/` dirs and `cmd/ze-test` subcommands with Make targets | Every shipped test suite has a documented runner and release disposition | Research found install tests not in Make gate and possible `ipsec`/`pppoe` runner gaps | Future fix should document or add release disposition for every `test/` directory, including Make/runner coverage | `spec-release-audit-1-surface-inventory.md`, then relevant child audit |
| RA-DOC-002 | Minor | docs/test evidence | `docs/functional-tests.md`, `mk/test-fuzz.mk`, `mk/test-functional.mk` | Developer/release operator may run stale or incomplete evidence | Compare docs against Make targets | Docs match current gate composition and fuzz target count/time | Research found docs omit `ze-evidence-vet` and stale fuzz details | Future fix should update docs after source-backed verification of current targets | `spec-release-audit-8-docs-onboarding.md` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Inventory starts | Binaries, CLI roots, online commands, plugins, schemas, routes, and tests have matrix rows |
| AC-2 | Static CLI root exists | It maps to help metadata, local handler, or explicit hidden/internal status |
| AC-3 | Local command is registered | It maps to a user entry point and functional/UI coverage or a finding |
| AC-4 | YANG command/API method exists | It maps to a registered RPC handler or a finding |
| AC-5 | Plugin is registered | It appears in aggregate imports, registry expectations, config roots, and tests or a finding |
| AC-6 | YANG config module exists | It maps to parser/validator coverage, env/listener checks, and owner audit |
| AC-7 | Web/API route exists | It maps to happy-path, auth, and error coverage or a finding |
| AC-8 | Test directory exists | It maps to a runner, Make target, release gate, or explicit release disposition |
| AC-9 | Finding recorded | It includes severity, reproduction, expected, actual, suggested direction, and owner |

## 🧪 TDD Test Plan

This inventory spec records audit work. It documents evidence expected from future fix work but does not add or change tests itself.

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestAllPluginsRegistered` | `internal/component/plugin/all/all_test.go` | Aggregate plugin registry expectation | Existing, currently fails on RA-INV-001 |
| Future command inventory test | TBD | Static/YANG/registry command drift | Suggested verification for RA-CLI-001 and RA-RPC-001 |
| Future test suite inventory test | TBD | `test/` directories have runners and release disposition | Suggested verification for RA-TEST-001 |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N/A | N/A | N/A | N/A | N/A |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| Unknown-command suggestion coverage | `test/ui/*.ci` | Invalid root suggests only valid root commands | Suggested verification for RA-CLI-001 |
| Command contract snapshot | `test/ui/command-contract-snapshot.ci` | Help/AI command contract reflects registered roots | Existing, coverage to verify |
| REST/LG/MCP route matrix tests | `test/plugin`, `test/web` | Each route/RPC has route-level behavior coverage | Suggested verification for web/API child audit |

### Interop Tests

| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| BGP interop inventory | `test/interop/scenarios/` | FRR, BIRD, GoBGP | Every protocol feature has release evidence disposition | Routed to protocol audit |

### Future

- Document candidate inventory checks for command drift and test-suite runner drift after the manual audit stabilizes.

## Files to Modify

- `plan/spec-release-audit-1-surface-inventory.md` - this inventory audit spec
- No production files in this inventory phase
- Future fix directions are recorded in Initial Findings; product edits happen outside this audit spec

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema | [ ] | N/A for inventory spec |
| CLI commands/flags | [ ] | N/A for inventory spec |
| CLI grammar | [ ] | Document-only in this audit; future fix specs decide |
| Editor autocomplete | [ ] | Document-only in this audit; future fix specs decide |
| Functional test for new RPC/API | [ ] | Document expected evidence only |
| Doctor check for runtime dependencies | [ ] | Document if future fix direction implies one |

### Documentation Update Checklist

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | N/A |
| 2 | Config syntax changed? | [ ] | N/A |
| 3 | CLI command added/changed? | [ ] | N/A |
| 4 | API/RPC added/changed? | [ ] | N/A |
| 5 | Plugin added/changed? | [ ] | N/A |
| 6 | Has a user guide page? | [ ] | Docs findings route to `spec-release-audit-8-docs-onboarding.md` |
| 7 | Wire format changed? | [ ] | N/A |
| 8 | Plugin SDK/protocol changed? | [ ] | N/A |
| 9 | RFC behavior implemented? | [ ] | N/A |
| 10 | Test infrastructure changed? | [ ] | N/A |
| 11 | Affects daemon comparison? | [ ] | N/A |
| 12 | Internal architecture changed? | [ ] | N/A |

## Files to Create

- None beyond this spec.

## Implementation Steps

Despite the template heading, these are audit documentation steps only. They do not authorize product code changes.

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Release Surface Matrix and Initial Findings |
| 3. Wiring phase | Data Flow and Boundaries Crossed |
| 4. Document findings | No production implementation in this spec |
| 5. Review gate | Verify matrix rows and findings for accuracy |
| 6. Full verification | `make ze-spec-status`, targeted tests for confirmed findings |
| 7. Critical review | Critical Review Checklist below |
| 8. Route issues | Record suggested direction and owner for future fix work |
| 9. Re-verify audit evidence | Re-check source references and reproductions, not product fixes |
| 10. Repeat | Until inventory has no unassigned surfaces |
| 11. Deliverables review | Surface matrix and finding list complete |
| 12. Security review | Ensure security-relevant surfaces route to resilience/security and RBAC specs |
| 13. Re-verify | Final spec visibility and status checks |
| 14. Present summary | Release inventory report |

### Implementation Phases

1. **Phase: Root CLI inventory** - diff `cmd/ze/main.go` static roots, YANG verbs, registry roots, local commands, help metadata, and UI tests.
2. **Phase: Online command inventory** - diff YANG `ze:command` and API RPC methods against `pluginserver.RegisterRPCs` registrations.
3. **Phase: Plugin and schema inventory** - diff aggregate imports, registry names, expected plugin tests, config roots, and YANG modules.
4. **Phase: Network route inventory** - enumerate web, LG, REST, gRPC, MCP, SSH routes/listeners and map tests.
5. **Phase: Evidence inventory** - map every `test/` directory to `ze-test`, Make targets, `ze-precommit-verify`, `ze-evidence-release-verify`, or explicit non-release disposition.
6. **Phase: Finding triage** - classify findings and route to child audits.

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every release surface group has a matrix row and owner audit |
| Correctness | Findings distinguish verified failures from coverage-risk observations |
| Reproducibility | Every verified finding has a command or source comparison |
| Test mapping | Every row has evidence or a follow-up finding |
| Routing | Every finding has an owner and suggested future fix direction |
| Scope | Inventory does not implement product fixes |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Inventory spec exists | `plan/spec-release-audit-1-surface-inventory.md` |
| Surface matrix exists | Release Surface Matrix section |
| Initial findings exist | Initial Findings section |
| Plugin failure reproduced | `go test ./internal/component/plugin/all -run TestAllPluginsRegistered -count=1` output recorded in session |
| Spec visible to status tool | `make ze-spec-status` shows `release-audit-1-surface-inventory` |

### Security Review Checklist

| Check | What to look for |
|-------|------------------|
| Mutating commands | Every mutation route/command routes to authz review |
| No-auth defaults | REST, gRPC, MCP, web insecure mode, LG public mode route to security review |
| Lifecycle operations | stop, restart, reboot, reload, config commit/discard route to audit/security review |
| Secret-bearing commands | passwd, PKI, SSH keys, tokens, VPN/IPsec, TACACS route to security review |
| Public routes | LG, docs/OpenAPI, health, OAuth metadata route to exposure review |

### Failure Routing

| Failure | Route To |
|---------|----------|
| CLI help/dispatch drift | `spec-release-audit-3-config-cli.md` |
| YANG/RPC drift | `spec-release-audit-3-config-cli.md` |
| Plugin aggregate drift | `spec-release-audit-5-plugins-rib.md` |
| Route/test coverage gaps | `spec-release-audit-4-web-lg-api.md` |
| Linux/system suite gaps | `spec-release-audit-6-system-linux.md` |
| Fuzz/chaos/race gaps | `spec-release-audit-7-resilience-security.md` |
| Docs/onboarding drift | `spec-release-audit-8-docs-onboarding.md` |
| Test-suite runner gap | This spec first, then relevant child audit |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| Missing `ike` import in `all.go` would be the aggregate test failure | The targeted test failed first on unexpected `bgp-filter-remove-private-as`; `ike` did not fail, likely due indirect import from another aggregate package | `go test ./internal/component/plugin/all -run TestAllPluginsRegistered -count=1` | Finding corrected to verified failure only |

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Treat all research observations as confirmed bugs | Some observations need generated diffs or route coverage checks | Record verified failures as findings and coverage-risk observations as audit tasks |

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|
| No single generated command/test inventory exists | First release audit occurrence | Consider generated inventory check after manual audit stabilizes | Track as future inventory test |

## Design Insights

- The inventory itself should become mechanically derivable over time, because manual command and test lists already show drift risks.
- The plugin aggregate test is valuable release evidence because it caught stale registry expectations immediately.
- Route inventory should not rely on docs because web/LG/API/MCP surfaces are wired in multiple packages.
- Test-suite inventory needs a disposition for every `test/` directory, including install, IPsec, PPPoE, internet/live, and stress-like suites.

## RFC Documentation

No RFC behavior is implemented in this inventory spec. BGP and IPsec protocol rows route to protocol-specific audits.

## Audit Summary

### What Was Documented

- Created the first release audit child spec and seeded the release surface matrix.
- Recorded verified and candidate inventory findings.

### Findings Recorded

- Found and verified RA-INV-001: `TestAllPluginsRegistered` fails because `bgp-filter-remove-private-as` is registered but absent from expected list.
- No bugs fixed in this inventory spec. No product code changes are in scope.

### Documentation Updates

- None yet. Documentation drift findings route to `spec-release-audit-8-docs-onboarding.md`.

### Deviations from Plan

- None.

## Implementation Audit

For this audit spec, "implementation" means audit documentation only. It does not mean product code changes.

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|
| Build release surface inventory | Partial | Release Surface Matrix | Matrix is seeded, later phases must make it exhaustive |
| Map surfaces to owner audits | Partial | Integration Points, matrix owner columns | All current groups have owner audit |
| Record findings with evidence and direction | Partial | Initial Findings | One verified failure, several coverage/drift findings needing child validation |

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|
| AC-1 | Partial | Release Surface Matrix | Needs exhaustive generated checks later |
| AC-2 | Partial | RA-CLI-002 | Static root/help diff started |
| AC-3 | Partial | CLI Root Surfaces | Local commands grouped, per-command coverage pending |
| AC-4 | Partial | RA-RPC-001 | Diff task identified, not generated yet |
| AC-5 | Partial | RA-INV-001 | Verified plugin aggregate failure |
| AC-6 | Partial | Plugin and Schema Surfaces | Schema groups listed, env/listener audit pending |
| AC-7 | Partial | Network and UI Surfaces | Route groups listed, route coverage matrix pending |
| AC-8 | Partial | Test and Evidence Surfaces, RA-TEST-001 | Runner/gate mapping started |
| AC-9 | Partial | Initial Findings | Findings include required fields |

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|
| `TestAllPluginsRegistered` | Failing | `internal/component/plugin/all/all_test.go` | Fails on unexpected `bgp-filter-remove-private-as` |
| Future command inventory test | Not started | TBD | Suggested verification only |
| Future test suite inventory test | Not started | TBD | Suggested verification only |

### Files from Plan

| File | Status | Notes |
|------|--------|-------|
| `plan/spec-release-audit-1-surface-inventory.md` | Created | This file |

### Audit Summary

- **Total items:** 9 ACs, 7 initial findings
- **Done:** 0 ACs fully complete
- **Partial:** 9 ACs seeded by current inventory research
- **Skipped:** None
- **Changed:** None

## Goal Validation

| Goal (from Task section) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Identify all release surfaces | Inventory evidence | Release Surface Matrix seeded from dispatch, registrations, routes, and tests |
| Assign every surface to an owner audit | Spec evidence | Owner Audit columns and Integration Points section |
| Capture actionable inventory findings | Test and source evidence | RA-INV-001 verified by targeted Go test; other findings route to child audits |

## Review Gate

### Run 1 (initial)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|
| 1 | NOTE | Inventory is seeded but not mechanically exhaustive yet | This file | Track generated inventory checks as future work |

### Spec Edits Applied

- None.

### Run 2+ (re-runs until clean)

| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status

- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above

## Wiring Test

<!-- Added 2026-07-10 to satisfy the spec validator (.claude/hooks/validate-spec.sh);
     this audit spec has no runtime code. Rows reference the existing Go test suite
     that already guards the inventory surfaces this spec audits. -->

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| generated aggregate imports (`internal/component/plugin/all/all.go`) | -> | plugin registry names snapshot | TestRegisteredPluginNames (`all_test.go`) |
| production aggregate purity | -> | registry excludes test-only plugins | TestAllPluginsRegistered (`all_test.go`) |

## Checklist

<!-- Added 2026-07-10: this audit spec predates the validator's required-section list.
     Audit specs produce documentation, not code; the TDD items bind the future fix
     work routed from findings, not this spec. -->

### Goal Gates (MUST pass)

- [ ] Every surface group has a matrix row with owner audit (AC-1)
- [ ] Findings include reproduction, owner, and suggested direction (AC-9)
- [ ] `make ze-standard-test` evidence requested from future fix work where findings demand it

### TDD (applies to future fix specs routed from findings)

- [ ] Tests written (in the owning fix spec)
- [ ] Tests FAIL (paste output) (in the owning fix spec)
- [ ] Tests PASS (paste output) (in the owning fix spec)

### Post-wave corrections (2026-07-10)

Factual corrections only, re-verified in the current tree after the followup implementation wave; the spec's disposition remains a pending user decision.

- **RA-INV-001 is no longer reproducible** (row struck in Initial Findings above). `TestAllPluginsRegistered` (`internal/component/plugin/all/all_test.go`) no longer compares an expected plugin-name list: it asserts registration is non-empty and rejects the test-only plugins `fakel2tp`/`fakeredist`. The name check moved to `TestRegisteredPluginNames` (`all_test.go`), which snapshots `registry.Names()` against `internal/component/plugin/all/testdata/plugins.snapshot`, and that snapshot INCLUDES `bgp-filter-remove-private-as` (`plugins.snapshot:18`). Both tests pass on 2026-07-10 with the Makefile tag set (`go test -tags 'ze_core <feature-gates.txt tags>' ./internal/component/plugin/all`: ok). Reproducer caveat: a bare untagged `go test` run instead fails with missing `isis`/`ldp`/`ospf`/`rsvp-te`, because those aggregate imports are feature-gated (`internal/component/plugin/all/all_ze_isis.go` and siblings); use the `GO_TEST` tag set (`Makefile:65`).
- Statements superseded by the passing run above (kept for history, do not act on them): the Current Behavior bullet "expected plugin list includes `ike` and excludes `bgp-filter-remove-private-as`"; the TDD row "`TestAllPluginsRegistered` ... Existing, currently fails on RA-INV-001"; the Implementation Audit rows "AC-5 Partial ... RA-INV-001" and "`TestAllPluginsRegistered` Failing".
- **Inventory gaps** (new rows needed when this audit resumes):
  - Plugin: `exabgp-bridge` (`internal/plugins/exabgp/bridgeplugin/register.go`; snapshot entry `plugins.snapshot:53`), registering YANG module `ze-exabgp-bridge-conf` (`internal/plugins/exabgp/bridgeplugin/yang/ze-exabgp-bridge-conf.yang`).
  - Network surface: `internal/core/dnsserver` listener core with DoT (RFC 7858, `internal/core/dnsserver/secure.go`) and DoH (RFC 8484, `secure.go`) listeners, consumed by as112 and geodns.
  - YANG config surfaces: DoT/DoH containers in `ze-as112-conf.yang` (tls container `:147`, DoT enable `:160`, DoH enable `:197`) and `ze-geodns-conf.yang` (tls container `:232`, DoT enable `:243`, DoH enable `:280`); DNSSEC leaf `dnssec-validation` in `internal/component/config/system/yang/ze-system-conf.yang`.
  - Verification gates: the live `ze-precommit-verify` stage list (`scripts/status/verify_run.go`, consumed at `:104`) gained `ze-port-defaults-check` (`scripts/checks/port_defaults.go`) and `ze-platform-vet` (`Makefile:337-341`) in both branches (`verify_run.go`, `:140-141`), alongside the existing `ze-tier-check`, `ze-iface-resolution-check` (`scripts/checks/iface_resolution.go`), and `ze-plugin-boundary-check` (`scripts/checks/plugin_process_boundary.go`).
- **MCP row superseded** (Network and UI Surfaces): the legacy `internal/component/mcp/handler.go` was deleted; Streamable HTTP is the only transport (`internal/component/mcp/streamable.go` `handlePOST:404`, `handleGET:618`, `handleDELETE:681`). The row's "raw HTTP/OAuth/CORS route coverage" risk should be re-scoped to the streamable endpoints.
- Housekeeping from this correction pass: the `## TDD Test Plan` heading was renamed to `## 🧪 TDD Test Plan` and the `## Wiring Test` and `## Checklist` sections above were added to satisfy the blocking spec validator; no audit content was changed by those edits.
