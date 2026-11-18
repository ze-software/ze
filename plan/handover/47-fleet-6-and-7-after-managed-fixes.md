# Handover: fleet-6 (freeze, disable, reconnect hold) and fleet-7 (divergence resolution)

Written 2026-11-18, at main `022a7bc193`. Nothing in fleet-6 or fleet-7 is
started. This handover exists because the managed (fleet) config code under
both specs changed a lot on 2026-11-18, and both specs were last updated
2026-06-26. Read it before you trust a file name, a line number or an
assumption in either spec.

## The task

| Spec | Path | Status | What it is |
|------|------|--------|------------|
| fleet-6 | `plan/release/fixes/spec-fleet-6-config-freeze.md` | skeleton, P0, release fix | Freeze operator edits on a managed device, `ze fleet disable/enable/status` with immediate sever, persisted baseline and a reconnect hold that marks the device `diverged` instead of overwriting a local emergency edit |
| fleet-7 | `plan/post-release/features/spec-fleet-7-config-reconnect-resolution.md` | skeleton, P3, post-release | Resolve a `diverged` device: `config-push` up-verb, diff at the hub, adopt or revert. Depends on fleet-6 |

Umbrella: `plan/post-release/features/spec-fleet-0-umbrella.md`, "Direction
Update (2026-06-26)".

## Start here

1. Run `/ze-disposition` on fleet-6, then on fleet-7. Both are five months old
   and name files that moved or do not exist (list below). Expect REWORK.
2. Run `/ze-spec` on fleet-6. Its design must be put to Thomas and approved
   before it moves to ready: he is not away. Do not implement from the skeleton.
3. fleet-7 waits for fleet-6, and for the decision on fleet-1 below.

Work in the shared `main` checkout, never a worktree. Commit through
`./le commit create`. A push needs Thomas at a terminal: `./le commit push`
refuses an agent.

## What is true in the code now (2026-11-18)

Design page: `docs/architecture/fleet-config.md`. Operator guide:
`docs/guide/fleet-config.md` (this is the website page `guides/fleet-config/`).
Digest: `ai/digests/hub-engine.md`, steps 9 to 11. Door: `ai/doors/hub-engine.md`.

Commits to read: `951251d339`, `02921e203a`, `1c6a148bbc`.

### Client (`internal/component/managed/`)

- `client.go` `fetchAndProcess` now ends in `ClientConfig.adoptEndpoint`: after
  every applied config the client re-reads the hub-level `client` block. A
  changed block returns `errEndpointChanged` (reconnect at once). No block
  returns `errHubClientRemoved`, and `RunManagedClient` returns for good.
- `ClientConfig` gained `Endpoint`, `HubRoot`, `ConnectTimeout`.
- `CheckManaged` is still read only before each connection attempt, so the
  spec's reason for an immediate sever still holds.
- `tls.go`: trust order is config `ca`, then `HubRoot` (stored root), then
  insecure, then system pool. `FirstUseTLSConfig` exists.
- `doctor.go`: a second check, `doctor-hub-client-missing`, reads
  `meta/instance/managed` and the config tree.

### Start path (`cmd/ze/ze_core_start.go`)

- `extractManagedClientConfig` still sets `Version` to the hash of the cached
  config. The spec's statement that this is not a record of the last
  hub-applied config is still correct, and is still why a baseline is needed.
- `hubClientEndpoint` reads the hub block and applies `--server`, `--name`,
  `--token`. `readManagedBootstrap` and `fetchInitialConfig` do the first start
  from what `ze init --managed` stored.

### Hub (`internal/component/plugin/server/managed_serve.go`, `cmd/ze/hub/managed_server.go`)

- `ManagedServerConfig.Listeners []ManagedListener`: a client is admitted only
  on the listener of the server block that declares it.
- `NotifyConfigChanged` marks a CONNECTED client in a pending set. A client
  that is not connected is not marked. `pushConfigChanged` closes the
  connection of a client that does not accept the push.
- `connectedClients()` is unexported and used by tests only. `Serves(name)` is
  the exported predicate, and it means "declared", not "connected". fleet-6's
  hub-side guard needs a new exported "is connected" predicate.

### Store keys (`pkg/zefs/keys.go`)

New: `meta/managed/hub/address`, `meta/managed/hub/token`,
`meta/managed/hub/ca`, `meta/managed/hub/trust`. `meta/instance/managed` is
unchanged.

### Editor (`internal/component/cli/editor_commit.go`)

- `commitSession` now publishes through `publishCommitted`: a config with a
  recorded active version gets a NEW active version
  (`storage.CommitConfigWithGuard`, which is `restoreConfigLocked`). Before
  this, edits to a hub's `client-<name>.conf` were silently lost.
- The spec's line numbers for this file (161, 319) are stale.

## Where the specs are wrong or incomplete now

| # | Spec says | Reality | What to do |
|---|-----------|---------|------------|
| 1 | fleet-6 edits `internal/component/cli/session_factory.go` to inject the frozen predicate | That file does not exist. The session editor is built in `cmd/ze/hub/session_factory.go` `buildSessionModelFactory` | Correct the file, and read that function: it is also where a session picks which stored config it edits |
| 2 | fleet-6 AC-9: "the hub's per-device config editor refuses edits to a device that is not connected", guard in `plugin/server/managed.go` | Four routes write `client-<name>.conf` on a running hub, and only one is an editor: the SSH session editor (`ze config edit client-<name>.conf`, no reload), `ze data import` / `update data config` (`storage/cli/data_rpc_config.go` `handleDataConfigUpdate`), `request data restore` (`data_rpc.go` `restoreConfigFromSource`), and the rollback RPC (`data_rpc_rollback.go`). The last three already share a `ServesClient` guard | The single-writer guard must cover all four, or the AC is false. The attached console, web editor, REST, gRPC and gNMI edit the hub's own config only |
| 3 | fleet-6 AC-9, same rule | It contradicts two documented behaviors: a client that is not connected gets the change when it next connects (guide, "Change a Client's Configuration"), and a NEW client's first config must be written before it has ever connected (guide, step 2) | Put this to Thomas. A connected-only rule as written makes provisioning a new client impossible |
| 4 | fleet-6 key `meta/instance/managed/base-version` | `meta/instance/managed` is itself a value key. Unverified whether the store accepts a key that is also a prefix of another | Verify in `pkg/zefs` before using it. `meta/managed/hub/` is the namespace the new bootstrap keys use |
| 5 | fleet-6 freeze: the operator cannot commit locally while managed | A managed client that applies a pushed config with no hub `client` block stops following its hub for good (`errHubClientRemoved`). Today the fix is `ze init --managed --force` | Decide how freeze and `ze fleet disable` treat that state: the device is "managed", frozen, and has no hub |
| 6 | fleet-6 assumption A-1: `CommitSession` and `CommitSessionCandidate` are the only operator write paths | Unvalidated, and the daemon-side `update data config` and restore RPCs also write a config on a device | Re-validate against the routes in row 2, on the DEVICE side too |
| 7 | fleet-7 stores the device's pushed config "as a pending candidate" | `storage.CommitConfigWithGuard` and `RestoreConfig` write the candidate pointer of that config and clear it on promote. A pending candidate held in that pointer would be consumed by the next editor commit, import or restore of the same client config | Do not hold fleet-7's pending config in the candidate pointer, or refuse those writes while one is pending |
| 8 | fleet-7 edits `internal/component/plugin/server/registry.go` for the `diverged` state "from fleet-1" | The file does not exist: fleet-1 (device registry) is a skeleton | fleet-7 needs fleet-1, or its own minimal record. The hub today remembers nothing about a client across a restart |
| 9 | Both specs name `plugin/server/managed.go` as the hub handler | That file is `ManagedConfigService` (fetch and hashing). The verb dispatch is `managed_serve.go` `handleRequest` | A new `config-push` verb goes in `handleRequest` |

## Open questions that were read and not settled

- Which SSH users may open another stored config. `buildSessionModelFactory`
  accepts any stored file name the session names; no check on profile was
  found. This matters to fleet-6: the freeze and the hub-side guard both sit on
  this path.
- `ze config edit` when the hub's SSH is unreachable starts a temporary daemon
  on the same store (`cmd_edit.go` `runStoredEditor`). Its behavior against a
  store a running hub holds was not traced.

## Tests and evidence

- `./le --name <session> test managed -a` is 17 of 17 at `022a7bc193`. `--name`
  builds `le` from the working tree; without it the fixtures run stale code.
- Real-daemon fixtures to copy from: `internal/test/fixture/misc_fixture_managed_first_start.go`
  (hub plus three clients, `ze init`, first start, store reads on stdout only),
  `misc_fixture_managed_cached_boot.go` (client with the hub down),
  `misc_fixture_managed_ca_trust.go` (hub root export).
- Go tests against a real `ManagedServer`: `internal/component/managed/client_endpoint_test.go`
  (`startServingHub`, `runClient`) and `cmd/ze/ze_core_start_bootstrap_test.go` (`bootHub`).
- A new `.ci` test is drafted under `test/draft/managed/`, run with
  `--draft`, stressed with `./le test stress-repro`, then moved (`/ze-test`).
- The commit gate refuses a new Warn or Error log line with no doctor code
  (`./le arch doctor-coverage`) and a new operator message that is not an
  `opererr` (`./le arch error-inventory`). Plan a doctor code for `diverged`.

## Not part of this work

fleet-1 to fleet-5 are skeletons. The website and wiki pages on fleet config
are current as of this date and will need a "freeze" and a "leaving the fleet"
section when fleet-6 lands.
