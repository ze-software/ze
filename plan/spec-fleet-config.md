# Spec: managed-config

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | - |
| Phase | - |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/fleet-config.md` - managed config architecture
4. `internal/component/plugin/ipc/tls.go` - TLS auth (extending for per-client secrets)
5. `internal/component/plugin/server/` - hub handlers (adding config-fetch)
6. `pkg/plugin/rpc/mux.go` - MuxConn wire format

## Task

Implement managed configuration using named `server`/`client` hub blocks. Every ze instance has at least one `server` block (for local plugins/SSH). Managed clients add a hub-level `client` block to connect outbound. Hub servers declare accepted clients under their `server` block. First boot provisioned via `ze init`; subsequent boots read cached config from blob.

Deliverables:
1. Named hub blocks: `plugin { hub { server <name> { host; port; secret } } }` replaces flat listen/secret
2. Per-client secrets: `server <name> { client <name> { secret } }` nested under server
3. Client outbound block: `plugin { hub { client <name> { host; port; secret } } }` at hub level
4. Hub RPCs: `config-fetch`, `config-changed`, `config-ack` handlers
5. Managed client component: connect, fetch, cache in blob, reconnect with backoff, heartbeat
6. `ze start` managed mode: reads blob metadata (from `ze init`) or cached config; CLI flags as overrides
7. Functional tests proving end-to-end fetch, backup resilience, and change notification

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
<!-- Capture insights as -> Decision: / -> Constraint: annotations -- these survive compaction. -->
- [ ] `docs/architecture/fleet-config.md` - managed config architecture (this spec's design doc)
  -> Decision: hub IS the server; no separate server component
  -> Decision: per-client secrets replace shared secret for managed connections
  -> Decision: client name implicit from auth session; config-fetch needs no name field
  -> Decision: named hub blocks: `server <name>` for listeners, `client <name>` for outbound
  -> Decision: every ze instance has at least one `server` block (local plugins/SSH)
  -> Decision: client identity from hub-level `client <name> { host; port; secret }` block
  -> Decision: first boot provisioned via `ze init`; subsequent boots read cached config from blob
  -> Decision: `meta/instance/managed` flag controls whether client connects to hub (implemented in blob-namespaces)
  -> Decision: two-phase config change (notify then fetch); client controls timing
  -> Constraint: version hash = truncated SHA-256 of config bytes
- [ ] `docs/architecture/zefs-format.md` - ZeFS blob store format
  -> Constraint: hierarchical keys via `/` separator; `fs.ValidPath` names
  -> Constraint: single-process ownership; `sync.RWMutex` for concurrency
- [ ] `docs/architecture/hub-architecture.md` - hub design (being extended)
  -> Constraint: `#0 auth` with token + name; constant-time comparison
  -> Constraint: 5-stage protocol for plugins; managed clients skip stages 1-4

**Key insights:**
- Hub already has TLS listener, auth, MuxConn, connection tracking by name
- Named blocks: `server <name> { host; port; secret }` for listeners, `client <name> { host; port; secret }` for outbound
- Per-client secrets nested under server: `server central { client edge-01 { secret } }`
- Client name IS the block name in hub-level `client` block
- Every ze instance needs at least one `server` block (local plugins and SSH need it)
- Multiple `server` blocks allowed (different secrets for different plugin trust levels)
- First boot provisioned via `ze init`; cached config self-describing after first fetch
- `meta/instance/managed` blob flag controls managed mode (already implemented, `ze init --managed`)

### RFC Summaries (MUST for protocol work)
No external RFCs apply. Internal protocol over existing transport.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/ipc/tls.go` - PluginAcceptor, Authenticate(), SendAuth()
  -> Constraint: Authenticate() takes single `expectedToken string`; needs per-client secret lookup
  -> Constraint: PluginAcceptor has flat `secret string` field; needs server-block-aware lookup
  -> Constraint: name validated: regex `^[a-zA-Z0-9][a-zA-Z0-9-]{0,63}$` (1-64 chars)
  -> Constraint: TLS 1.3 minimum; EC P-256 for self-signed; 10s auth timeout
- [ ] `internal/component/plugin/ipc/tls_test.go` - auth flow tests
- [ ] `internal/component/plugin/server/` - hub server (~51 files), startup coordinator, dispatch
  -> Constraint: hub dispatches requests from MuxConn.Requests() channel
  -> Constraint: handler.go routes RPC verbs; managed RPCs will be new verb registrations
- [ ] `internal/component/plugin/types.go` - HubConfig (Listen []string, Secret string), PluginConfig
  -> Constraint: HubConfig struct at line ~284; flat Listen + Secret; needs named server/client blocks
- [ ] `internal/component/bgp/config/plugins.go` - ExtractHubConfig() at line ~91
  -> Constraint: reads flat `plugin { hub { listen ...; secret ...; } }` via tree lookup
  -> Constraint: validates secret >= 32 chars; returns HubConfig{Listen, Secret}
  -> Constraint: replacing with named `server`/`client` block parsing
- [ ] `internal/component/plugin/schema/ze-plugin-conf.yang` - hub YANG schema
  -> Constraint: currently flat: `container hub { leaf-list listen; leaf secret; }`
  -> Constraint: replacing with named `server` list + nested `client` list + hub-level `client` list
- [ ] `pkg/plugin/rpc/mux.go` - MuxConn: CallRPC, Requests channel (no changes needed)
- [ ] `pkg/zefs/store.go` - BlobStore API (no changes needed)
- [ ] `internal/component/config/storage/storage.go` - Storage interface (no changes needed)
- [ ] `internal/component/config/storage/blob.go` - blobStorage with resolveKey() namespaces (no changes needed)
- [ ] `internal/core/env/` - env.Get, env.Set, env.MustRegister (needs `ze.managed.*` registrations)
- [ ] `cmd/ze/main.go` - main dispatch; `ze start` at line 209 calls cmdStart() at line 323
  -> Constraint: cmdStart() requires blob storage, reads meta/instance/name, calls hub.Run()
  -> Constraint: extend cmdStart() for managed mode: check meta/instance/managed, connect to hub
- [ ] `cmd/ze/hub/main.go` - hub.Run() is the actual daemon entry point
  -> Constraint: reads config, creates engine/bus/subsystem/pluginmgr, runs until signal

**Behavior to preserve:**
- All existing standalone `ze start` / `ze config.conf` behavior unchanged
- Hub TLS listener, plugin auth, and 5-stage protocol unchanged for plugin connections
- ZeFS blob format unchanged
- Storage interface unchanged
- MuxConn wire format unchanged
- Existing shared `secret` field continues to work for plugin connections

**Behavior to change:**
- Hub config: flat `listen`/`secret` replaced by named `server <name> { host; port; secret }` blocks
- Hub server blocks gain nested `client <name> { secret }` entries for managed clients
- Hub-level `client <name> { host; port; secret }` blocks for outbound connections
- Hub auth extended: per-client secret lookup under the relevant `server` block
- Hub gains `config-fetch`, `config-changed`, `config-ack` RPC handlers
- `ze start` detects managed mode from `meta/instance/managed` blob flag + cached config or `ze init` metadata
- CLI flags `--server`, `--name`, `--token` as overrides for troubleshooting

## Data Flow (MANDATORY - see `rules/data-flow-tracing.md`)

### Entry Point
- **Hub side:** admin edits config via SSH editor or `ze config edit`; hub's ZeFS blob updated
- **Client first boot:** blob metadata from `ze init` (or CLI flag overrides)
- **Client subsequent boot:** cached config from local blob
- **Client runtime:** TLS connection to hub, RPC messages over MuxConn

### Transformation Path

#### First Boot (After `ze init`)
1. `ze start` -- reads blob: `meta/instance/name`=edge-01, `meta/instance/managed`=true, hub server, hub token
2. Client connects to hub via TLS, sends `#0 auth` with token and name
3. Hub validates token against `client edge-01 { secret }` in its config
4. Client sends `config-fetch {"version":""}` (no cached version)
5. Hub reads edge-01's config from its blob, sends full config
6. Client validates config (contains `plugin { hub { edge-01 { server; secret } } }`)
7. Client writes config to local blob, starts BGP

#### Subsequent Boot (Local Blob -> Hub -> Update)
1. `ze start` (no flags)
2. Client reads cached config from local blob
3. Client parses `plugin { hub { edge-01 { server 10.0.0.1:1790; secret "..."; } } }`
4. Client connects to hub, authenticates, sends `config-fetch` with version hash
5. Hub responds with "current" or updated config
6. Client updates blob if newer, starts or reloads BGP

#### Config Change Flow (Hub -> Client)
1. Admin edits edge-01's config in hub's blob
2. Hub detects change, sends `config-changed` to connected client
3. Client sends `config-fetch` when ready
4. Client validates, writes to blob, reloads BGP

#### Startup with Cached Config (Hub Unreachable)
1. Client reads cached config from blob
2. TLS connect to hub fails (timeout)
3. Client starts BGP from cached config
4. BGP sessions may provide route to hub
5. Background reconnect loop with exponential backoff

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Client -> Hub | TLS + MuxConn RPCs (`config-fetch`, `config-ack`, `ping`) | [ ] |
| Hub -> Client | TLS + MuxConn RPCs (`config-changed`, `ping`) | [ ] |
| Hub blob -> Wire | `ReadFile()` -> base64 -> JSON -> MuxConn | [ ] |
| Wire -> Client blob | MuxConn -> JSON -> base64 decode -> `WriteFile()` | [ ] |
| Config block -> Client identity | Parse `plugin { hub { <name> { } } }` for name, server, token | [ ] |

### Integration Points
- `internal/component/plugin/server/` - hub gains config-fetch handler
- `internal/component/plugin/ipc/tls.go` - auth extended for per-client secrets
- `internal/component/plugin/types.go` - HubConfig gains ClientSecrets
- `internal/component/bgp/config/plugins.go` - parses hub client entries + client-side hub block
- `internal/component/plugin/schema/ze-plugin-conf.yang` - `client` list + client-side named block
- `pkg/zefs/BlobStore` - hub blob and client local blob
- `internal/component/config/storage.NewBlob()` - client opens local blob
- `internal/core/env.MustRegister()` - `ze.managed.*` env vars
- `cmd/ze/main.go` - cmdStart() extended for managed mode detection
- `cmd/ze/hub/main.go` - hub.Run() daemon entry point (unchanged)

### Architectural Verification
- [ ] No bypassed layers (config flows through Storage interface)
- [ ] No unintended coupling (managed client is standalone component; hub extensions are minimal)
- [ ] No duplicated functionality (reuses TLS, auth, MuxConn, ZeFS, Storage)
- [ ] Zero-copy preserved where applicable (ZeFS mmap reads; config copied once for wire)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `ze start` after `ze init` managed=true (first boot) | -> | Client reads blob, connects to hub, fetches config | `test/managed/client-first-boot.ci` |
| `ze start` (cached config) | -> | Client reads config from blob, connects to hub | `test/managed/client-cached-boot.ci` |
| Hub config change | -> | Client receives notification, fetches, applies | `test/managed/config-change-notify.ci` |
| Hub unreachable at startup | -> | Client starts from cached config in blob | `test/managed/client-backup-start.ci` |
| Hub unreachable during run | -> | Client keeps running, reconnects when hub returns | `test/managed/client-reconnect.ci` |
| `plugin { hub { client edge-01 { secret "..."; } } }` | -> | Hub accepts client with per-client token | `test/managed/per-client-auth.ci` |
| Wrong token for name | -> | Hub rejects connection | `test/managed/auth-reject.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze start` after `ze init` with managed=true (first boot, no cached config) | Client reads blob metadata, connects to hub, fetches config, caches, starts BGP |
| AC-2 | `ze start` with cached config containing hub block (subsequent boot) | Client reads config, connects to hub, starts BGP |
| AC-3 | Client running, admin edits edge-01's config on hub | Client receives `config-changed`, fetches, applies |
| AC-4 | Client running, hub process killed | Client continues running on current config |
| AC-5 | `ze start` with hub unreachable, cached config exists | Client starts BGP from cached config |
| AC-6 | `ze start` after `ze init` managed=true, hub unreachable, no cached config | Client exits with clear error |
| AC-7 | Client reconnects after hub comes back | Client sends version hash, fetches if newer |
| AC-8 | Hub sends config that fails validation | Client rejects, sends `config-ack` with error, keeps running |
| AC-9 | Two clients connect with same name | Hub rejects second connection |
| AC-10 | Client connects with wrong token for its name | Hub rejects with auth error |
| AC-11 | Client connects with name that has no `client` entry in any server | Hub rejects with auth error |
| AC-12 | Client reconnect uses exponential backoff | Delays grow: 1s, 2s, 4s, 8s, ... capped at 60s |
| AC-13 | Config unchanged between reconnects | Hub responds `{"status":"current"}`, no reload |
| AC-14 | Config contains `plugin { hub { client edge-01 { host; port; secret } } }` | Client extracts name, address, token from block |
| AC-15 | Config contains `plugin { hub { server local { host; port; secret } } }` | Local hub starts, plugins can connect |
| AC-16 | CLI `--server` flag overrides config and blob values | Flag takes precedence |
| AC-17 | `meta/managed` set to false while connected | Hub connection severed, daemon keeps running locally |

## TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVersionHash` | `pkg/fleet/version_test.go` | SHA-256 truncated hash, deterministic | |
| `TestVersionHashSameContent` | `pkg/fleet/version_test.go` | Identical content = identical hash | |
| `TestVersionHashDifferentContent` | `pkg/fleet/version_test.go` | Different content = different hash | |
| `TestConfigEnvelopeMarshal` | `pkg/fleet/envelope_test.go` | Config envelope -> JSON | |
| `TestConfigEnvelopeRoundTrip` | `pkg/fleet/envelope_test.go` | Marshal -> unmarshal preserves all fields | |
| `TestPerClientSecretLookup` | `internal/component/plugin/ipc/tls_test.go` | Per-client secret found by name | |
| `TestPerClientSecretReject` | `internal/component/plugin/ipc/tls_test.go` | Wrong token for name rejected | |
| `TestPerClientSecretUnknownName` | `internal/component/plugin/ipc/tls_test.go` | Unknown name rejected | |
| `TestHubConfigFetch` | `internal/component/plugin/server/managed_test.go` | Fetch returns config + version hash | |
| `TestHubConfigFetchCurrent` | `internal/component/plugin/server/managed_test.go` | Matching version returns "current" | |
| `TestHubConfigFetchMissing` | `internal/component/plugin/server/managed_test.go` | No config entry for name returns error | |
| `TestHubConfigChanged` | `internal/component/plugin/server/managed_test.go` | Blob write triggers notification to connected client | |
| `TestExtractHubServers` | `internal/component/bgp/config/plugins_test.go` | Named `server` blocks parsed with host/port/secret | |
| `TestExtractHubServerClients` | `internal/component/bgp/config/plugins_test.go` | Nested `client` entries under `server` parsed | |
| `TestExtractHubServerClientSecretTooShort` | `internal/component/bgp/config/plugins_test.go` | Client secret < 32 chars returns error | |
| `TestExtractHubClients` | `internal/component/bgp/config/plugins_test.go` | Hub-level `client` blocks parsed with host/port/secret | |
| `TestExtractHubClientMissing` | `internal/component/bgp/config/plugins_test.go` | No hub-level `client` block returns empty | |
| `TestExtractMultipleServers` | `internal/component/bgp/config/plugins_test.go` | Multiple `server` blocks with different names parsed | |
| `TestReconnectBackoff` | `internal/component/managed/reconnect_test.go` | Backoff doubles: 1s, 2s, 4s, 8s, ... capped at 60s | |
| `TestReconnectBackoffJitter` | `internal/component/managed/reconnect_test.go` | Jitter within 10% | |
| `TestReconnectBackoffCap` | `internal/component/managed/reconnect_test.go` | Never exceeds 60s | |
| `TestClientHandleConfigChanged` | `internal/component/managed/handler_test.go` | Notification triggers fetch | |
| `TestClientValidateConfigOk` | `internal/component/managed/handler_test.go` | Valid config accepted, cached in blob | |
| `TestClientValidateConfigBad` | `internal/component/managed/handler_test.go` | Invalid config rejected, blob unchanged | |
| `TestHeartbeatTimeout` | `internal/component/managed/heartbeat_test.go` | 3 missed pings triggers reconnect | |

### Boundary Tests (MANDATORY for numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Client secret length | 32+ chars | 32 | 31 | N/A |
| Client name length | 1-64 chars | 64 | 0 (empty) | 65 |
| Connect timeout | 1s-300s | 300s | 0s | N/A (capped) |
| Backoff delay | 1s-60s | 60s (cap) | N/A | N/A (capped internally) |
| Version hash length | 16 hex chars | Always 16 | N/A | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `client-first-boot` | `test/managed/client-first-boot.ci` | Client first boot with CLI flags, fetches config | |
| `client-cached-boot` | `test/managed/client-cached-boot.ci` | Client boots from cached config, connects to hub | |
| `per-client-auth` | `test/managed/per-client-auth.ci` | Hub accepts client with per-client token | |
| `auth-reject` | `test/managed/auth-reject.ci` | Hub rejects wrong token | |
| `config-change-notify` | `test/managed/config-change-notify.ci` | Hub notifies client of change, client applies | |
| `client-backup-start` | `test/managed/client-backup-start.ci` | Client starts from cached config when hub unreachable | |
| `client-reconnect` | `test/managed/client-reconnect.ci` | Client reconnects after hub returns | |
| `client-reject-invalid` | `test/managed/client-reject-invalid.ci` | Client rejects invalid config | |

### Future (if deferring any tests)
- Performance tests with many concurrent clients (> 100) -- deferred to scale testing spec
- Config rollback -- deferred to config-archive spec

## Files to Modify
- `internal/component/plugin/ipc/tls.go` - per-client secret lookup in Authenticate()
- `internal/component/plugin/types.go` - HubConfig gains ClientSecrets map[string]string
- `internal/component/bgp/config/plugins.go` - ExtractHubConfig parses `client` entries + client-side hub block
- `internal/component/bgp/config/plugins_test.go` - tests for client secret + managed hub block extraction
- `internal/component/plugin/schema/ze-plugin-conf.yang` - `client` list + client-side named block under `hub`
- `internal/component/plugin/server/` - add managed config handlers
- `cmd/ze/main.go` - cmdStart() extended with managed mode detection + first-boot flags

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (hub client list + client-side block) | [x] | `internal/component/plugin/schema/ze-plugin-conf.yang` |
| CLI flags | [x] | `cmd/ze/main.go` cmdStart() |
| Hub RPC handlers | [x] | `internal/component/plugin/server/managed.go` |
| Functional tests | [x] | `test/managed/*.ci` |
| Architecture doc | [x] | `docs/architecture/fleet-config.md` (already written) |

## Files to Create
- `pkg/fleet/version.go` - version hash computation (SHA-256 truncated)
- `pkg/fleet/version_test.go` - version hash tests
- `pkg/fleet/envelope.go` - config envelope types (RPC payloads)
- `pkg/fleet/envelope_test.go` - envelope tests
- `pkg/fleet/doc.go` - package documentation
- `internal/component/plugin/server/managed.go` - hub-side config-fetch/changed handlers
- `internal/component/plugin/server/managed_test.go` - hub handler tests
- `internal/component/managed/client.go` - managed client: connect, fetch, cache, apply
- `internal/component/managed/reconnect.go` - reconnect loop with exponential backoff
- `internal/component/managed/reconnect_test.go` - backoff tests
- `internal/component/managed/handler.go` - client RPC handlers: config-changed, ping
- `internal/component/managed/handler_test.go` - handler tests
- `internal/component/managed/heartbeat.go` - heartbeat sender and timeout detection
- `internal/component/managed/heartbeat_test.go` - heartbeat tests
- `internal/component/managed/doc.go` - package documentation
- `test/managed/client-first-boot.ci` - functional test: first boot with CLI flags
- `test/managed/client-cached-boot.ci` - functional test: boot from cached config
- `test/managed/per-client-auth.ci` - functional test: per-client secret auth
- `test/managed/auth-reject.ci` - functional test: wrong token rejected
- `test/managed/config-change-notify.ci` - functional test: config change notification
- `test/managed/client-backup-start.ci` - functional test: start from cached config
- `test/managed/client-reconnect.ci` - functional test: client reconnects
- `test/managed/client-reject-invalid.ci` - functional test: client rejects bad config

### Documentation Update Checklist (BLOCKING)
<!-- Every row MUST be answered Yes/No during the Completion Checklist (planning.md step 1). -->
<!-- Every Yes MUST name the file and what to add/change. -->
<!-- See planning.md "Documentation Update Checklist" for the full table with examples. -->
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | [ ] | `docs/features.md` |
| 2 | Config syntax changed? | [ ] | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` |
| 3 | CLI command added/changed? | [ ] | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | [ ] | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | [ ] | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | [ ] | `docs/guide/<topic>.md` |
| 7 | Wire format changed? | [ ] | `docs/architecture/wire/*.md` |
| 8 | Plugin SDK/protocol changed? | [ ] | `.claude/rules/plugin-design.md`, `docs/architecture/api/process-protocol.md` |
| 9 | RFC behavior implemented? | [ ] | `rfc/short/rfcNNNN.md` |
| 10 | Test infrastructure changed? | [ ] | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | [ ] | `docs/comparison.md` |
| 12 | Internal architecture changed? | [ ] | `docs/architecture/core-design.md` or subsystem doc |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists |
| 3. Implement (TDD) | Implementation phases below |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: shared types** -- `pkg/fleet/` version hash, envelope, doc
   - Tests: `TestVersionHash*`, `TestConfigEnvelope*`
   - Files: `pkg/fleet/version.go`, `envelope.go`, `doc.go` + tests
   - Verify: tests fail -> implement -> tests pass

2. **Phase: named hub blocks** -- replace flat listen/secret with `server`/`client` named blocks
   - Tests: `TestExtractHubServers`, `TestExtractHubClients`, `TestExtractMultipleServers`
   - Files: `internal/component/bgp/config/plugins.go`, `types.go`, YANG schema
   - Verify: tests fail -> implement -> tests pass

3. **Phase: per-client secrets** -- nested `client` entries under `server`, auth extension
   - Tests: `TestPerClientSecret*`, `TestExtractHubServerClients*`
   - Files: `internal/component/plugin/ipc/tls.go`, `bgp/config/plugins.go`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: hub config handlers** -- config-fetch, config-changed, config-ack
   - Tests: `TestHubConfigFetch*`, `TestHubConfigChanged`
   - Files: `internal/component/plugin/server/managed.go` + tests
   - Verify: tests fail -> implement -> tests pass

5. **Phase: managed client core** -- connect, fetch, cache, validate
   - Tests: `TestClientHandleConfigChanged`, `TestClientValidateConfig*`
   - Files: `internal/component/managed/client.go`, `handler.go` + tests
   - Verify: tests fail -> implement -> tests pass

6. **Phase: reconnect and heartbeat** -- exponential backoff, liveness
   - Tests: `TestReconnectBackoff*`, `TestHeartbeatTimeout`
   - Files: `internal/component/managed/reconnect.go`, `heartbeat.go` + tests
   - Verify: tests fail -> implement -> tests pass

7. **Phase: ze start managed mode** -- extend cmdStart() for managed config detection, start client
   - Tests: `test/managed/client-first-boot.ci`, `client-cached-boot.ci`, `client-backup-start.ci`
   - Files: `cmd/ze/main.go` (cmdStart), `cmd/ze/hub/main.go` (hub.Run integration)
   - Verify: functional tests

8. **Phase: integration tests** -- auth, notification, reconnect, rejection
   - Tests: remaining `test/managed/*.ci` files
   - Verify: all functional tests pass

9. **Full verification** -> `make ze-verify`
10. **Complete spec** -> Fill audit tables, write learned summary, delete spec.

### Critical Review Checklist (/implement stage 5)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has implementation with file:line |
| Correctness | Version hash deterministic; cached config valid; per-client secrets isolated |
| Naming | JSON keys kebab-case; YANG kebab-case; env vars `ze.managed.*` |
| Data flow | Config: hub blob -> wire -> client blob -> BGP; through Storage interface |
| Auth | Per-client secrets in Authenticate(); shared secret still works for plugins |
| Config as identity | Client name, server, token all from `plugin { hub { <name> { } } }` block |
| Resilience | Client survives hub death; cold-starts from cached config during partition |
| Rule: goroutine-lifecycle | Reconnect loop and heartbeat are long-lived goroutines |
| Rule: env vars | All `ze.managed.*` vars registered via `env.MustRegister()` |

### Deliverables Checklist (/implement stage 9)
| Deliverable | Verification method |
|-------------|---------------------|
| `pkg/fleet/` exists | `ls pkg/fleet/*.go` |
| `internal/component/managed/` exists | `ls internal/component/managed/*.go` |
| Hub managed handler exists | `ls internal/component/plugin/server/managed.go` |
| Named `server` list in YANG | `grep "server" internal/component/plugin/schema/ze-plugin-conf.yang` |
| Named `client` list in YANG | `grep "client" internal/component/plugin/schema/ze-plugin-conf.yang` |
| `host` and `port` leaves in YANG | `grep "host\|port" internal/component/plugin/schema/ze-plugin-conf.yang` |
| First boot works | `test/managed/client-first-boot.ci` passes |
| Cached boot works | `test/managed/client-cached-boot.ci` passes |
| Per-client auth works | `test/managed/per-client-auth.ci` passes |
| Backup start works | `test/managed/client-backup-start.ci` passes |
| Config notification works | `test/managed/config-change-notify.ci` passes |
| Architecture doc exists | `ls docs/architecture/fleet-config.md` |

### Security Review Checklist (/implement stage 10)
| Check | What to look for |
|-------|-----------------|
| Per-client token | Constant-time comparison; minimum 32 chars; no logging of token values |
| Token isolation | Client A's token cannot authenticate as client B |
| Config isolation | Client can only fetch own config (name implicit from auth, no parameter) |
| TLS | TLS 1.3 minimum; strong cipher suites |
| Name validation | Alphanumeric + hyphen; max 64 chars; no path traversal in blob key |
| Envelope bounds | Base64 decode bounded; no unbounded allocations |
| Heartbeat | Timeout prevents hung connections; backoff prevents hub overload |
| Duplicate name | Hub rejects second connection with same name |
| Config token exposure | Token in config file; blob permissions 0600; document env var as preferred |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Separate fleet server component | Hub already has everything needed | Extend hub with per-client auth + config RPCs |
| `fleet { }` config block | No new server; hub is the server | Per-client entries in existing `plugin { hub { } }` |
| Name field in config-fetch | Client can only get its own config | Name implicit from auth session |
| `meta/` blob keys for identity | Identity belongs in config, not metadata | `plugin { hub { <name> { server; secret } } }` in config |
| `ze init` for managed setup | Config is self-describing after first fetch | First boot uses CLI flags; config has everything after |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- Hub-as-server avoids duplicating TLS/auth/MuxConn infrastructure
- Per-client secrets with name binding eliminates need for authorization layer (auth = authz)
- Implicit name from session means the protocol cannot be misused to fetch another client's config
- Config as single source of truth: identity, hub connection, and BGP all in one file
- First boot CLI flags are the only bootstrap; after first fetch, config is self-describing
- No special metadata keys needed; the config block IS the metadata
- `ze start` already exists as "start daemon from blob"; extend for managed mode, no new command needed
- `ze data rm` already handles blob deletion; no new deletion code needed

## RFC Documentation

No external RFCs. Protocol documented in `docs/architecture/fleet-config.md`.

## Implementation Summary

### What Was Implemented
- [List actual changes made]

### Bugs Found/Fixed
- [Any bugs discovered -- add test for each]

### Documentation Updates
- [Docs updated, or "None"]

### Deviations from Plan
- [Differences from original plan and why]

## Implementation Audit

<!-- BLOCKING: Complete BEFORE writing learned summary. See rules/implementation-audit.md -->

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

<!-- BLOCKING: Do NOT trust the audit above. Re-verify everything independently. -->

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-17 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/*`, `cmd/*`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all checks -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all checks documented pass in spec.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled
- [ ] Write learned summary to `plan/learned/NNN-managed-config.md`
- [ ] **Summary included in commit**
