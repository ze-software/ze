# Spec: fixit-ca-root-location

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | - |
| Phase | research complete, design complete, nothing implemented |
| Deferral shard | `-` |
| Handoff | - |
| Updated | 2026-09-04 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The daemon's certificate authority root private key is written to a path that
depends on the process working directory.

`LoadOrGenerateRootFor` (`internal/component/pki/ca.go`) passes the two
registered ZeFS key names, `meta/ca/cert` and `meta/ca/key`, straight to
`store.WriteFile`. Those names are RELATIVE keys. `filesystemStorage.WriteFile`
(`internal/component/config/storage/storage.go`) hands the name verbatim to
`atomicWriteFile`, which calls `os.MkdirAll` on its directory part. The package
doc of `storage` states the contract this breaks: "All callers use absolute
filesystem paths as names". So on a filesystem-backed store the root private key
lands in `<cwd>/meta/ca/key`.

That store is reachable in production. `resolve.Storage`
(`internal/core/resolve/resolve.go`) returns a filesystem store when
`ze.storage.blob` is `false`, when `paths.DefaultConfigDir()` is empty, and when
the blob cannot be opened. `ze start <config-file>` replaces the blob store with
a filesystem store when the blob does not hold the named config
(`cmd/ze/ze_core_start.go`). The consequence is worse than an odd file location:
the daemon's IDENTITY follows its working directory, so the same daemon started
from two directories presents two different certificate authorities, and every
client that trusted the first one is refused by the second.

`TestAPIBootWarnsExactlyWhenNoUsersAndNoToken`
(`cmd/ze/hub/main_servers_test.go`) recreates both files under `cmd/ze/hub/` on
its own. `plan/journal/test-artifacts-land-in-the-repository-root.md` carries
the 2026-09-03 row for that artifact, records the `.gitignore` that hides it,
and names the producer as unfixed.

The goal is the owner's decision of 2026-09-03: the operator chooses where the
root lives, as a blob in the configuration file or as a file path on the
filesystem, and Ze chooses a safe location when the operator writes neither. The
working directory is never one of those locations.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/pki/pki-store.md` - the local certificate authority, its store contract and its doctor check
  → Decision: `RootStore` is three methods (`ReadFile`, `WriteFile`, `Exists`) so that a certificate authority reaches no config path, no write lock and no version history. Any new location arm implements those three and nothing more.
  → Decision: "The root lives in ZeFS, not in config" and "a private key cannot live in config either" are the two sentences this spec overturns on the owner's instruction. They are edited in the same work as the code.
  → Constraint: the doctor check reads the STORE rather than the loaded root, because `ze doctor` is a different process, and it runs pre-config so a daemon with a broken config still reports its authority. A config-driven location contradicts the second half.
  → Constraint: there is no accessor for the root private key and there will be none. It leaves the package only into a signing operation, so no arm may expose it through a show command or a JSON payload.
- [ ] `docs/architecture/pki/tls-listeners.md` - the consumers of an issued leaf, and the doctor precedent for config-driven material
  → Decision: "doctor reads the parsed config, not the live store" is already how `CheckCertReference` works, so a check that reads a configured location follows an established shape rather than inventing one.
  → Constraint: a configured name that does not resolve returns an error and no material, and the daemon refuses to start. The new location arms fail the same way.
- [ ] `docs/architecture/zefs-format.md` - the key registry, the Private flag and the mode that protects stored keys
  → Constraint: ZeFS has no per-key mode. `BlobStore.WriteFile` accepts `perm` and ignores it, and `storeFileInfo.Mode()` is a constant `0o444`. The protection is the blob file's own 0600, created by `os.CreateTemp` inside `atomicWrite`. A filesystem arm therefore owes its own mode enforcement, because nothing under it supplies one.
  → Constraint: the page states that `meta/ca/cert` and `meta/ca/key` are "written once, on the first daemon start that finds no root". That sentence is true only for a blob-backed store and needs the qualifier this spec adds.
- [ ] `docs/architecture/fleet-config.md` - the managed config server, the second caller of the root loader
  → Constraint: the managed server calls `LoadOrGenerateRoot` a second time and the comment calls it "a load and never a second root". That holds only while both calls resolve the same store, so the second call takes the loaded root instead.
- [ ] `docs/architecture/hub-architecture.md` - where the store handle, the parsed tree and the plugin manager meet
  → Decision: `runYANGConfig` is where the root is loaded and the issuer injected, so it is where the resolved location is chosen. `preparePKIConfig` already parses the `pki` block earlier in the same function, so the parsed location is in hand before the root is loaded.
- [ ] `docs/architecture/appliance/builder.md` - the appliance certificate authority, which already stores a root in two files
  → Decision: `applianceRootStore` maps the two registered key names onto `ca-cert.pem` and `ca-key.pem` and refuses any other name. It is the file-backed `RootStore` this spec needs, minus the passphrase, so the file arm promotes it rather than writing a second one.
  → Constraint: the appliance encrypts the private half with the appliance passphrase. A promoted implementation keeps that behavior for the appliance and must not force it on the daemon.
- [ ] `ai/patterns/config-option.md` - the structural template for a YANG leaf
  → Constraint: container names are singular nouns with no `-config` suffix, leaf names are kebab-case with no abbreviation, and every added node carries both a `description` under 96 characters and a distinct `ze:help`.
  → Decision: the decision table answers YANG rather than env var on three rows: an operator sets it, it belongs in `show configuration` and in a backup, and it is not a debug or bootstrap knob.
- [ ] `ai/rules/evidence.md` - guards fail closed
  → Constraint: a guard that cannot answer says so. A permission test that cannot read the owner of a file refuses rather than passing, and a location that does not resolve refuses rather than inventing one.
- [ ] `ai/rules/simplicity.md` - the simplest fully correct answer
  → Constraint: correctness is not what simplicity cuts. The refusal arms and the permission guards stay whatever they cost in lines.

### RFC Summaries (Scope: protocol)
- N-A. No wire protocol changes. The root is Ze's own trust anchor, distributed by hand, and the certificates it issues are unchanged.

**Key insights:** (minimal context to resume after compaction)
- The defect is a relative ZeFS key handed to a store that resolves names as filesystem paths. Both halves are correct on their own; the pairing is the defect.
- `pointerPath` (`internal/component/config/storage/pointer.go`) already solves the same problem for config pointers, by branching on `IsBlobStorage` and building an explicit path for the filesystem arm. Its filesystem arm is rooted at the config file's directory, which is what puts `meta/` in the source tree.
- `ResolveSSHStorage` (`internal/component/config/infra/ssh.go`) and `NewServer` (`internal/component/ssh/ssh.go`) are the closest precedent: an operator leaf, a derived default, and a refusal when neither resolves.
- `resolveKey` (`internal/component/config/storage/blob.go`) passes a `meta/`-prefixed name through unchanged, which is what makes the blob arm safe today.
- Ze is pre-release, so no deployment holds a root under a working-directory path. Only blob-held roots need to keep their identity.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/pki/ca.go` - `LoadOrGenerateRootFor` reads `zefs.KeyCACert.Pattern` and `zefs.KeyCAKey.Pattern` through the `RootStore` interface, generates and writes the pair when either half is absent, and publishes the result in `currentRoot`. Generation is serialized in-process by `rootGenerationMu`.
- [ ] `internal/component/pki/doctor.go` - `checkCARoot` reads the same two key names from `ctx.Store`, in the pre-config phase, and reports missing, half-written, unloadable and expiring roots.
- [ ] `internal/component/pki/config.go` - `ParseConfig` builds `PKIConfig` from the `pki` block: a `ca` list and a `certificate` list, each value read as PEM or as base64 DER. There is no `local-ca` node.
- [ ] `internal/component/pki/yang/ze-pki-conf.yang` - the `pki` container holds `ca` and `certificate` lists only. Certificate material is inline, and no leaf anywhere in the module names a filesystem path.
- [ ] `internal/component/pki/types.go` - `PKIConfig` holds `CACerts` and `Certificates` maps and nothing about the authority.
- [ ] `internal/component/config/storage/storage.go` - `filesystemStorage.WriteFile` calls `atomicWriteFile(name, ...)`, which runs `os.MkdirAll(filepath.Dir(path), 0o700)` and writes the file with the caller's perm. The package doc says every caller passes an absolute path.
- [ ] `internal/component/config/storage/blob.go` - `resolveKey` returns a `meta/` or `file/` prefixed name unchanged, and maps anything else to `file/active/<basename>`. This is why the blob arm addresses a blob key and never a file.
- [ ] `internal/component/config/storage/pointer.go` - `pointerPath` branches on `IsBlobStorage` and joins the config file's directory with `meta/config/<pointer>` for the filesystem arm. The existing two-arm shape for the same problem.
- [ ] `internal/core/resolve/resolve.go` - `Storage` returns the blob at `<DefaultConfigDir>/database.zefs`, and a filesystem store when `ze.storage.blob` is false, when `DefaultConfigDir` is empty, or when the blob will not open.
- [ ] `cmd/ze/ze_core_start.go` - `resolveStorage` prints a warning and returns the filesystem store on a blob error. The `ze start <file>` branch replaces the blob store with a filesystem store when the blob does not hold the named config and the file exists on disk.
- [ ] `cmd/ze/hub/main.go` - `runYANGConfig` parses the `pki` block through `preparePKIConfig`, then loads the authority with `LoadOrGenerateRoot(store)` and injects it into the plugin manager. A failure is a startup failure.
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` calls `LoadOrGenerateRoot(store)` a second time, on the same store handle.
- [ ] `internal/component/config/infra/ssh.go` - `ResolveSSHStorage` returns the main store when it is blob-backed, otherwise opens `database.zefs` under the config directory and then under `DefaultConfigDir`, and falls back to the main store when neither opens.
- [ ] `internal/component/ssh/ssh.go` - `NewServer` defaults `HostKeyPath` to `<ConfigDir or DefaultConfigDir>/ssh_host_ed25519_key` and returns `errHostKeyPathCannotBeResolved` when neither directory resolves.
- [ ] `internal/component/config/loader.go` - `LoadConfig` sets `ConfigDir` to the config file's directory, and to the working directory when the config arrives on stdin.
- [ ] `internal/core/paths/paths.go` - `DefaultConfigDir` honors `ze.config.dir`, then maps the running binary's location to `/etc/ze`, `/perm/ze` or `<prefix>/etc/ze`, and returns an empty string for an unknown layout.
- [ ] `internal/appliance/ca.go` - `applianceRootStore` satisfies `RootStore` over two files, refuses any name that is not one of the two registered keys, and puts the private half through the appliance passphrase.
- [ ] `pkg/zefs/keys.go` - `KeyCACert` is `meta/ca/cert`, `KeyCAKey` is `meta/ca/key` and is registered `Private: true`.
- [ ] `pkg/zefs/store.go` - `atomicWrite` creates the blob with `os.CreateTemp`, which is 0600, and installs it by rename.
- [ ] `internal/core/diagnostic/codes.go` - `doctor-pki-ca-root-missing` and `doctor-pki-ca-root-expiry` carry the operator text for the two existing findings.
- [ ] `test/managed/managed-hub-ca-trust.ci` - the two-daemon test that proves a client validates a chain against the exported root. It is the functional shape a location test follows.

**Behavior to preserve:** (unless the user explicitly said to change it)
- A deployment whose store is blob-backed keeps its root, at `meta/ca/cert` and `meta/ca/key`, with the same bytes and the same issuer. No restart may rotate it.
- `RootStore` stays three methods. Nothing gains a config path, a lock or a version history.
- `LoadOrGenerateRootFor` keeps its signature and its read-before-write order, and the appliance keeps calling it with its own store and its own validity.
- Root generation stays serialized in-process, and stays unserialized across processes (`plan/journal/store-serializes-in-process-only.md`).
- The root private key stays unreachable outside a signing operation. No arm adds an accessor, a show command or a JSON field for it.
- `show pki local-ca pem` keeps answering from the root this process loaded.
- The existing `pki ca` and `pki certificate` lists are untouched.

**Behavior to change:**
- The location of the root becomes an operator choice with three arms: inline in the config, a directory on the filesystem, or the blob store Ze resolves.
- A filesystem-backed config store is no longer a location for the root. The working directory is never a location.
- A daemon that can resolve no location refuses to start, where today it writes into the working directory.
- The doctor check reads the configured location, so it moves from the pre-config phase to the post-config phase.
- `startManagedServer` takes the loaded root rather than resolving one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The `pki { local-ca { ... } }` block in the operator's configuration file, parsed by the YANG-driven config parser into a `*config.Tree`.
- The daemon's storage handle, chosen by `resolve.Storage` and possibly replaced by the `ze start <file>` branch in `cmd/ze/ze_core_start.go`.
- `paths.DefaultConfigDir()`, which reads `ze.config.dir` or the running binary's location.

### Transformation Path
1. `LoadConfig` (`internal/component/config/loader.go`) parses the file into a tree.
2. `preparePKIConfig` (`cmd/ze/hub/main_pki.go`) calls `pki.ParseConfig`, which now also reads the `local-ca` container into a `LocalCASource` value on `PKIConfig`: the inline certificate and key bytes, or the configured directory, or neither.
3. `runYANGConfig` (`cmd/ze/hub/main.go`) calls `pki.ResolveRootStore(source, store)`, which returns the `RootStore` for the chosen arm or an error.
4. `pki.LoadOrGenerateRoot(rootStore)` reads the pair, or generates and writes it, exactly as it does today.
5. The loaded `*Root` is injected into the plugin manager and passed to `startManagedServer`.
6. `ze doctor` runs `checkCARoot` in the post-config phase, resolving the same location from the parsed tree and the store it holds.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ pki | `ParseConfig` reads the `local-ca` container and returns plain bytes and one path string | No |
| pki ↔ storage | The blob arm passes the daemon's `storage.Storage`, which satisfies `RootStore` with no adapter | No |
| pki ↔ filesystem | The file arm owns its own `os` calls, its directory creation at 0700 and its mode and ownership guards | No |
| Hub ↔ managed server | The loaded `*Root` is passed as an argument instead of being resolved a second time | No |
| Daemon ↔ doctor | `ze doctor` is a separate process and resolves the location from the parsed config plus its own store handle | No |

### Integration Points
- `pki.ParseConfig` and `pki.PKIConfig` - gain the parsed location, so every existing caller of `ParseConfig` keeps working unchanged.
- `pki.LoadOrGenerateRootFor` - unchanged, and still the only constructor. The resolution happens before it.
- `internal/appliance/ca.go` - constructs the promoted file store with its passphrase hooks and deletes `applianceRootStore`.
- `internal/core/diagnostic/codes.go` - one new code for a location Ze refuses.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | A `meta/`-prefixed name reaches a blob key and never a filesystem path | `resolveKey` and `isNamespaced` (`internal/component/config/storage/blob.go`) | The blob arm carries the same defect as the filesystem arm | `TestBlobArmWritesNoFileBesideTheBlob` | unvalidated |
| A-2 | The blob file is 0600 by construction, so the blob arm needs no mode enforcement of its own | `atomicWrite` uses `os.CreateTemp` (`pkg/zefs/store.go`), stated in `docs/architecture/zefs-format.md` | The blob arm needs the same mode guard as the file arm | existing `TestRootKeyIsWrittenPrivate` | unvalidated |
| A-3 | `LoadConfigResult.ConfigDir` is the working directory in stdin mode, so it is never a safe candidate | `LoadConfig` (`internal/component/config/loader.go`) | A candidate list that includes it reintroduces the defect for `ze start -` | `TestWorkingDirectoryIsNeverACandidate` | unvalidated |
| A-4 | Ze's YANG loader supports `choice` and `case`, and the config text names neither | `ze-static-conf.yang`, `ze-iface-conf.yang`, and the `dataKeywords` map in `internal/component/config/yang_schema.go` | The two arms need a custom validator instead of native mutual exclusion | `test/parse/pki-local-ca-arms-are-exclusive.ci` | unvalidated |
| A-5 | No deployment holds a root under a working-directory path, because Ze is pre-release | `CLAUDE.md`, owner directive 2026-08-30 | A first start after this change rotates a root that peers already trust, which is the outcome this spec exists to prevent | Thomas confirms | unvalidated |
| A-6 | The three hub boot tests already set `ze.config.dir`, so the default arm resolves for them and none needs an edit | `cmd/ze/hub/main_servers_test.go` and `cmd/ze/hub/service_grpc_test.go` | Those tests fail closed and each needs a location | running them | unvalidated |
| A-7 | File ownership is readable through `os.Stat` on every platform Ze builds for | `internal/appliance/crypto.go` writes secrets with no ownership test today | The ownership guard needs a platform split, or it refuses everywhere | `TestFileArmRefusesAForeignOwner` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The daemon refuses to start where it used to start, because no location resolves | A `ze start` that exits 1 with the new error | The error names both leaves and `ze.config.dir`, and the change note says so. Refusing is the decision: a location Ze invents is what caused the defect |
| R-2 | The doctor's move to the post-config phase loses the authority report for a daemon whose config is broken | `ze doctor` prints nothing about the root on a config that fails to parse | Accepted and stated in `pki-store.md`. A daemon with a broken config does not start, so it has no authority to report on |
| R-3 | The appliance root changes its encryption behavior when `applianceRootStore` is promoted | An appliance key file that opens without the passphrase | The promoted store takes read and write hooks, and the appliance passes its own. `test/appliance` covers the round trip |
| R-4 | A second caller resolves its own store and generates a second root | Two roots in one daemon, or a managed client refused by a hub it trusts | `startManagedServer` takes the loaded root as an argument, and `LoadOrGenerateRoot` keeps exactly one caller |
| R-5 | An operator writes a directory on a shared or world-readable volume | None at runtime, which is why the guard is at startup | Ze refuses a directory with any group or other permission bit, and refuses a key file that is not a regular file owned by the process user |
| R-6 | The inline arm puts a root private key into `show configuration` and into every backup | Nothing fails; the exposure is silent | The leaf is `ze:sensitive`, so the config file holds `$9$` and the display masks it. The daemon logs one line at load naming the exposure, in the shape `warnPlaintextOnDisk` already uses |
| R-7 | A reload changes the `local-ca` block and the running root no longer matches the config | A config the operator believes is in force | The reload path logs that the change takes effect at the next start and leaves the root in force, which is what SSH already does for its host key |
| R-8 | Two daemons share one blob and each replaces the other's keys | State that was written and is no longer there | Out of scope and already recorded in `plan/journal/store-serializes-in-process-only.md`. This spec neither widens nor narrows it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every internal TLS listener that has no operator-named certificate: the plugin hub acceptor, the managed config server, and any client that trusts an exported root. A wrong location either rotates the authority, which refuses every client that trusted the old one, or writes a private key somewhere unsafe |
| How is it reverted? | A single commit revert. The blob arm keeps the same keys and the same bytes, so a reverted daemon reads the root the fixed daemon wrote. A root generated into a configured directory is not visible to a reverted daemon, which would generate a new one |
| Who else touches this path? | `internal/appliance` (a second `RootStore` implementation), `cmd/ze/hub` (two call sites), `internal/component/plugin` and `internal/component/managed` (consumers of the issued leaf). `spec-local-ca` closed on 2026-09-04 and is the spec that built this surface |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pki { local-ca { directory "/abs/dir" } }` in a config file | → | `pki.ResolveRootStore` returns the file store, `fileRootStore.WriteFile` writes `ca-key.pem` | `TestConfiguredDirectoryHoldsTheRoot` |
| `pki { local-ca { certificate ...; private { key ... } } }` | → | `pki.ResolveRootStore` returns the inline store, `LoadOrGenerateRoot` reads it and writes nothing | `TestInlineRootIsUsedAndNothingIsWritten` |
| No `local-ca` block, blob-backed daemon | → | `pki.ResolveRootStore` returns the daemon's blob store, keys unchanged | `TestDefaultArmKeepsTheBlobKeys` |
| No `local-ca` block, no blob store reachable | → | `pki.ResolveRootStore` returns an error and `runYANGConfig` exits 1 | `TestRefusesWhenNoLocationResolves` |
| `ze start <config>` run from a scratch working directory | → | `runYANGConfig` → `ResolveRootStore` → `LoadOrGenerateRoot` | `test/plugin/pki-root-never-lands-in-the-working-directory.ci` |
| `ze doctor` on a config that names a directory | → | `checkCARoot` in the post-config phase reads that directory | `test/ui/doctor-pki-ca-root-configured-directory.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A daemon starts from an arbitrary working directory, with a filesystem-backed config store and no `local-ca` block | Nothing is created in that working directory. No `meta` directory, no key, no certificate |
| AC-2 | A daemon with a blob-backed store and no `local-ca` block starts, stops and starts again | The root is read from `meta/ca/cert` and `meta/ca/key` of that blob, and the second start presents the same certificate, with the same serial |
| AC-3 | `local-ca directory` names an absolute directory that does not exist | Ze creates it with mode 0700, writes `ca-cert.pem` and `ca-key.pem` with mode 0600, and a restart presents the same root |
| AC-4 | `local-ca directory` names a relative path | The daemon refuses to start. The error names the leaf, the value it was given, and that the path must be absolute |
| AC-5 | `local-ca directory` names an existing directory whose mode carries any group or other bit | The daemon refuses to start. The error names the directory and its mode |
| AC-6 | The configured directory holds a `ca-key.pem` that is not a regular file, carries a group or other permission bit, or is owned by another user | The daemon refuses to start, naming which of the three it found. A store that cannot read the owner refuses rather than passing |
| AC-7 | `local-ca` carries an inline certificate and private key | The daemon issues from that root: an issued leaf's issuer common name and authority key identifier match the configured certificate. No file and no blob key is written |
| AC-8 | `local-ca` carries one inline half only | The config is refused, naming the missing half |
| AC-9 | `local-ca` carries both an inline half and a directory | The config is refused at parse time, naming the two arms as exclusive |
| AC-10 | No `local-ca` block, no blob store, and `ze.config.dir` unset with the binary in an unknown layout | The daemon refuses to start. The error names both config arms and `ze.config.dir` |
| AC-11 | `ze doctor` runs against a config that names each arm in turn | The report describes the root at the configured location: absent, unloadable, or expiring, with the same codes as today plus the location in the message |
| AC-12 | A reload changes the `local-ca` block | The root in force does not change, and one log line says the change takes effect at the next start |
| AC-13 | A daemon starts the managed config server | The server uses the root the hub loaded. `LoadOrGenerateRoot` has exactly one caller in `cmd/ze` |
| AC-14 | `show pki local-ca pem` runs on a daemon using each arm in turn | The printed certificate is the one at the configured location |
| AC-15 | The inline arm is in force | One log line at startup names the exposure: the root private key is in the config file and in every backup of it |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Keeps the root on an encrypted volume, away from the config | config `local-ca directory` → `ParseConfig` → `ResolveRootStore` → file store → `LoadOrGenerateRoot` | `test/plugin/pki-local-ca-directory.ci` |
| 2 | Holds one root centrally and pastes it into each router's config | config `local-ca certificate` plus `private key` → inline store → `LoadOrGenerateRoot` | `test/plugin/pki-local-ca-inline.ci` |
| 3 | Writes nothing and keeps the root Ze already generated | no block → blob store → same two keys | `TestDefaultArmKeepsTheBlobKeys`, `test/managed/managed-hub-ca-trust.ci` unchanged and still green |
| 4 | Runs `ze start` from a checkout or a scratch directory | no block → resolved blob → nothing in the working directory | `test/plugin/pki-root-never-lands-in-the-working-directory.ci` |
| 5 | Runs `ze doctor` before starting the daemon | parsed config → configured location → diagnostics | `test/ui/doctor-pki-ca-root-configured-directory.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDefaultArmKeepsTheBlobKeys` | `internal/component/pki/rootstore_test.go` | With no config block and a blob store, the resolved store reads and writes `meta/ca/cert` and `meta/ca/key` (AC-2) | |
| `TestRefusesWhenNoLocationResolves` | `internal/component/pki/rootstore_test.go` | No block, no blob, no config dir: an error, and no file created anywhere (AC-10) | |
| `TestWorkingDirectoryIsNeverACandidate` | `internal/component/pki/rootstore_test.go` | The resolver is driven from a temporary working directory and creates nothing in it, on every arm (AC-1, A-3) | |
| `TestConfiguredDirectoryHoldsTheRoot` | `internal/component/pki/rootstore_test.go` | Generation into a configured directory: 0700 directory, 0600 files, second load reads the same root (AC-3) | |
| `TestFileArmRefusesARelativeDirectory` | `internal/component/pki/rootstore_test.go` | A relative value is an error naming the leaf and the value (AC-4) | |
| `TestFileArmRefusesAGroupReadableDirectory` | `internal/component/pki/rootstore_test.go` | Modes 0750, 0705 and 0777 are each refused, 0700 is accepted (AC-5) | |
| `TestFileArmRefusesAnUnsafeKeyFile` | `internal/component/pki/rootstore_test.go` | A key file that is a symlink, a directory, or carries a group or other bit is refused (AC-6) | |
| `TestFileArmRefusesAForeignOwner` | `internal/component/pki/rootstore_test.go` | A key file owned by another user is refused, and a store that cannot read the owner refuses too (AC-6, A-7) | |
| `TestInlineRootIsUsedAndNothingIsWritten` | `internal/component/pki/rootstore_test.go` | The inline arm loads the configured pair, and its `WriteFile` returns an error rather than persisting (AC-7) | |
| `TestInlineArmRefusesOneHalf` | `internal/component/pki/config_test.go` | A certificate with no key, and a key with no certificate, are each refused by name (AC-8) | |
| `TestBothArmsAreRefused` | `internal/component/pki/config_test.go` | A block carrying a directory and an inline half is refused (AC-9) | |
| `TestBlobArmWritesNoFileBesideTheBlob` | `internal/component/pki/rootstore_test.go` | After a blob-backed generation, the blob's own directory holds only the blob file (A-1) | |
| `TestDoctorReadsTheConfiguredLocation` | `internal/component/pki/doctor_test.go` | The check reports a missing, unloadable and expiring root at a configured directory, and the message names the location (AC-11) | |
| `TestReloadDoesNotRotateTheRoot` | `cmd/ze/hub/main_reload_test.go` | A reload that changes the block leaves the loaded root in place and logs the restart requirement (AC-12) | |
| `TestManagedServerTakesTheLoadedRoot` | `cmd/ze/hub/managed_server_test.go` | The server is constructed with the hub's root, and no second resolution happens (AC-13) | |
| `TestApplianceRootRoundTripsThroughThePromotedStore` | `internal/appliance/ca_test.go` | The appliance still encrypts the private half and reads it back (R-3) | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Directory mode | 0700 only | 0700 | N/A | 0701, 0710, 0750, 0777 each refused |
| Key file mode | 0600 and 0400 | 0600 | N/A | 0601, 0640, 0644 each refused |
| Certificate file mode | any mode readable by the process | 0644 | N/A | N/A, the certificate is public material |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `pki-root-never-lands-in-the-working-directory` | `test/plugin/*.ci` | A daemon started from a scratch working directory leaves it empty (AC-1) | |
| `pki-local-ca-directory` | `test/plugin/*.ci` | The operator points the root at a directory, restarts, and gets the same root (AC-3, story 1) | |
| `pki-local-ca-inline` | `test/plugin/*.ci` | The operator pastes a root into the config and the daemon issues from it (AC-7, story 2) | |
| `pki-local-ca-relative-directory-refused` | `test/parse/*.ci` | A relative directory is refused with a message that names the leaf (AC-4) | |
| `pki-local-ca-arms-are-exclusive` | `test/parse/*.ci` | Both arms together are refused (AC-9, A-4) | |
| `doctor-pki-ca-root-configured-directory` | `test/ui/*.ci` | `ze doctor` reports the root at the configured directory (AC-11) | |
| `managed-hub-ca-trust` | `test/managed/*.ci` | The existing two-daemon trust test still passes, unchanged (AC-2, story 3) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire-visible change. The certificates Ze issues, and the chain a peer validates, are byte-identical whatever arm holds the root. The two-daemon `managed-hub-ca-trust` test is the closest thing to an interop proof this surface has, and it stays green | |

## Files to Modify
- `internal/component/pki/ca.go` - `LoadOrGenerateRootFor` keeps its shape; the file gains nothing but a doc pointer to the resolver
- `internal/component/pki/config.go` - `ParseConfig` reads the `local-ca` container, refuses a half-written inline pair, and returns the parsed source
- `internal/component/pki/types.go` - `PKIConfig` gains the parsed location as a value type
- `internal/component/pki/doctor.go` - the check moves to the post-config phase and resolves the configured location
- `internal/component/pki/yang/ze-pki-conf.yang` - the `local-ca` container, its `choice`, its leaves, and their `description` and `ze:help` texts
- `cmd/ze/hub/main.go` - resolve the location before loading the root, and pass the loaded root to the managed server
- `cmd/ze/hub/managed_server.go` - take the root as an argument
- `cmd/ze/hub/main_reload.go` - warn when a reload changes the block
- `internal/appliance/ca.go` - construct the promoted file store and delete `applianceRootStore`
- `internal/core/diagnostic/codes.go` - the new refusal code and its operator text
- `docs/architecture/pki/pki-store.md` - the two decisions this spec overturns, the three arms, the guards, and the doctor phase
- `docs/architecture/zefs-format.md` - qualify "written once on the first daemon start" with the arm it describes
- `docs/architecture/appliance/builder.md` - the appliance now shares the daemon's file store
- `docs/architecture/hub-architecture.md` - the resolution step between the config parse and the root load
- `docs/architecture/fleet-config.md` - the managed server takes the loaded root
- `docs/guide/configuration.md` - the `local-ca` block in the PKI Certificate Store section
- `docs/config-reference.md` - the `local-ca` syntax
- `docs/features.md` - the operator can choose where the root lives

## Files to Create
- `internal/component/pki/rootstore.go` - `ResolveRootStore`, the inline store, the file store and the permission guards
- `internal/component/pki/rootstore_test.go` - the unit tests above
- `test/plugin/pki-root-never-lands-in-the-working-directory.ci` - the defect's functional proof
- `test/plugin/pki-local-ca-directory.ci` - story 1
- `test/plugin/pki-local-ca-inline.ci` - story 2
- `test/parse/pki-local-ca-relative-directory-refused.ci` - AC-4
- `test/parse/pki-local-ca-arms-are-exclusive.ci` - AC-9
- `test/ui/doctor-pki-ca-root-configured-directory.ci` - AC-11

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/pki/yang/ze-pki-conf.yang`, a `local-ca` container inside the existing `pki` container, with a `choice` over the inline case and the file case |
| YANG validation constraints | Yes | The `choice` gives native mutual exclusion. The directory leaf takes `type string` with a `pattern` that requires a leading slash, so a relative value is refused by the schema before any code runs |
| YANG custom validators | No | Neither arm needs a runtime set. Absoluteness is a pattern, exclusivity is a choice, and the permission tests are startup guards over the live filesystem rather than config validation |
| CLI commands/flags | No | No new command. `show pki local-ca pem` already prints the loaded root and keeps working on every arm |
| CLI grammar (keyword before value) | N-A | No command is added |
| Editor autocomplete | Yes | Automatic for the new leaves from their YANG types. The directory leaf offers no dynamic completion, because a path on the router's filesystem is not enumerable from the editor's process |
| Functional test for new RPC/API | Yes | `test/plugin/pki-local-ca-directory.ci` and `test/plugin/pki-local-ca-inline.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | Nothing lands under `environment/`. The decision table in `ai/patterns/config-option.md` answers YANG on three rows: an operator sets it, it belongs in `show configuration` and in a backup, and it is not a debug or bootstrap knob |
| Doctor check for runtime dependencies | Yes | The existing `pki-ca-root` check moves phase and gains the configured location. One new code, `doctor-pki-ca-root-location`, in `internal/core/diagnostic/codes.go`, for a location Ze refuses. Unit test in `internal/component/pki/doctor_test.go`, functional test in `test/ui/doctor-pki-ca-root-configured-directory.ci` |
| Prometheus counters/metrics | No | The root's state is a doctor and health concern, and both surfaces already report it. A counter of a value that changes once per decade earns nothing |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, the line that names where the local certificate authority root lives |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md` (PKI Certificate Store section), `docs/config-reference.md` |
| 3 | CLI command added/changed? | No | No command is added or changed |
| 4 | API/RPC added/changed? | No | No RPC is added or changed |
| 5 | Plugin added/changed? | No | The plugin hub is a consumer of the issued leaf and is unaffected |
| 6 | Has a user guide page? | Yes | `docs/guide/configuration.md` |
| 7 | Wire format changed? | No | No wire format is touched |
| 8 | Plugin SDK/protocol changed? | No | The SDK reads the root certificate from its environment slot, unchanged |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC requirement changes |
| 10 | Test infrastructure changed? | No | The new tests use the existing `.ci` runner and fixture shapes |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` names no certificate authority behavior |
| 12 | Internal architecture changed? | Yes | `docs/architecture/pki/pki-store.md`, `docs/architecture/zefs-format.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/fleet-config.md`, `docs/architecture/appliance/builder.md` |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counter is added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The new diagnostic code is inventory. `docs/guide/status.md` lists the pki doctor codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-fixit-ca-root-location.md`. The `// Design:` headers of the files named above declare `docs/architecture/pki/pki-store.md`, `docs/architecture/pki/tls-listeners.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/fleet-config.md`, `docs/architecture/appliance/builder.md` and `docs/architecture/zefs-format.md`. Two are named as UNAFFECTED. `docs/architecture/pki/tls-listeners.md`: `internal/component/pki/tls.go` is not modified, and the named-certificate path it describes outranks issuance on every arm, unchanged. `docs/features/ai-first.md`: `internal/core/resolve/resolve.go` was read for how the daemon's store is chosen and is not modified, because the CA resolution sits above it and reads its result |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` shows a `pki` block. The example gains the new container, and the pki-store page's claim that the root cannot live in config is corrected rather than left beside the new leaf |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the location reaches the loader
   - Tests: `TestDefaultArmKeepsTheBlobKeys`, `TestRefusesWhenNoLocationResolves`, `TestWorkingDirectoryIsNeverACandidate`
   - Files: `internal/component/pki/rootstore.go` with the resolver and the blob and refusal arms only, `internal/component/pki/types.go`, `cmd/ze/hub/main.go`
   - Verify: a blob-backed daemon behaves exactly as before, a filesystem-backed daemon refuses instead of writing into the working directory, and the tests fail before the resolver exists
2. **Phase: The file arm** - a directory the operator names
   - Tests: `TestConfiguredDirectoryHoldsTheRoot`, `TestFileArmRefusesARelativeDirectory`, `TestFileArmRefusesAGroupReadableDirectory`, `TestFileArmRefusesAnUnsafeKeyFile`, `TestFileArmRefusesAForeignOwner`
   - Files: `internal/component/pki/rootstore.go`, `internal/appliance/ca.go`, `internal/appliance/ca_test.go`
   - Verify: each guard is proven by a test that constructs the unsafe state and reads the refusal, and the appliance round trip still encrypts
3. **Phase: The YANG and the parser** - the operator can write the block
   - Tests: `TestInlineArmRefusesOneHalf`, `TestBothArmsAreRefused`, `test/parse/pki-local-ca-relative-directory-refused.ci`, `test/parse/pki-local-ca-arms-are-exclusive.ci`
   - Files: `internal/component/pki/yang/ze-pki-conf.yang`, `internal/component/pki/config.go`
   - Verify: `ze config validate` accepts each arm alone and refuses both together and each half alone
4. **Phase: The inline arm** - a root in the config file
   - Tests: `TestInlineRootIsUsedAndNothingIsWritten`, `test/plugin/pki-local-ca-inline.ci`
   - Files: `internal/component/pki/rootstore.go`, `cmd/ze/hub/main.go`
   - Verify: the daemon issues from the configured root, writes nothing, and logs the exposure line once
5. **Phase: The second caller and the reload** - one resolution per daemon
   - Tests: `TestManagedServerTakesTheLoadedRoot`, `TestReloadDoesNotRotateTheRoot`
   - Files: `cmd/ze/hub/managed_server.go`, `cmd/ze/hub/main.go`, `cmd/ze/hub/main_reload.go`
   - Verify: `LoadOrGenerateRoot` has one caller under `cmd/ze`, and a reload leaves the root in force
6. **Phase: The doctor** - the check reads the configured location
   - Tests: `TestDoctorReadsTheConfiguredLocation`, `test/ui/doctor-pki-ca-root-configured-directory.ci`
   - Files: `internal/component/pki/doctor.go`, `internal/core/diagnostic/codes.go`
   - Verify: each arm produces a report that names the location it read
7. **Phase: The functional proof and the pages** - the defect is closed and the documentation matches
   - Tests: `test/plugin/pki-root-never-lands-in-the-working-directory.ci`, `test/plugin/pki-local-ca-directory.ci`
   - Files: every page in Files to Modify
   - Verify: the functional test goes red against the current binary and green against the fixed one, and every page that described the old behavior is corrected

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-1 has a functional test rather than a unit test |
| Feature completeness | Each of the three arms reaches `LoadOrGenerateRoot` from a real config file, not from a test constructor alone |
| Correctness | The working directory appears in no candidate list, on any arm, in any order. `LoadConfigResult.ConfigDir` is not read by the resolver |
| Correctness | A blob-backed deployment reads the same two keys and presents the same certificate before and after the change |
| Naming | The YANG leaf names, the Go field names and the documentation use one spelling for the container, and the file names match the appliance's `ca-cert.pem` and `ca-key.pem` |
| Data flow | The location is resolved exactly once per daemon, and the resolved store is not rebuilt by any second caller |
| Rule: `ai/rules/evidence.md` | Every guard fails closed. A permission or ownership test that cannot read what it needs refuses rather than passing |
| Rule: `ai/rules/no-layering.md` | `applianceRootStore` is deleted, not kept beside the promoted store |
| Rule: `ai/rules/stale-comments.md` | The comment in `managed_server.go` calling the second load "a load and never a second root" is corrected or removed with the call it describes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No key is written relative to the working directory | `test/plugin/pki-root-never-lands-in-the-working-directory.ci`, and `go test ./cmd/ze/hub/...` followed by `git status --porcelain cmd/ze/hub` reporting nothing |
| One caller of the root loader in the daemon | `grep -rn 'LoadOrGenerateRoot(' cmd/ | grep -v _test` returns one line |
| One file-backed root store in the tree | `grep -rln 'ca-key.pem' internal/ | grep -v _test` returns one implementation file |
| The blob arm is unchanged for an existing store | `test/managed/managed-hub-ca-trust.ci` passes with no edit |
| Every new node carries both texts | `./le docvalid help-shape` |
| The pages match the code | `./le spec citation anchors spec plan/spec-fixit-ca-root-location.md` reports no unnamed owner |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The directory value is operator input that becomes a filesystem path. It must be absolute, must not be resolved through a symlink into another user's directory, and must never be joined with anything derived from a peer |
| Permissions | The directory is created 0700 and refused when it exists with any group or other bit. The key file is written 0600 and refused when it carries a group or other bit, is not a regular file, or is owned by another user |
| Fail closed | Every guard that cannot answer refuses. A stat that fails, an owner that cannot be read, a location that does not resolve: each is an error and never a permissive default |
| Error leakage | A refusal names the path and the mode, which the operator needs, and never the key material |
| Exposure | The inline arm puts a private key in the config file, in `show configuration` output and in every backup. The leaf is `ze:sensitive` and the daemon says so once at startup |
| Authorization that could fail open | The root private key gains no accessor. `show pki local-ca pem` prints the certificate only, on every arm |

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

- The defect is not a bug in either half. `filesystemStorage` documents that its names are absolute paths, and the ZeFS key registry documents that its patterns are relative keys. The defect is one caller pairing them, and the same pairing exists wherever a registered key reaches a store that may be filesystem-backed. `pointerPath` is the one place that already branches on `IsBlobStorage` to avoid it.
- The SSH host key is the same problem solved a year earlier and solved differently: it carries an absolute PATH from config or from the config directory, so both backends can serve it, and it refuses when neither resolves. Its refusal, `errHostKeyPathCannotBeResolved`, is the precedent for AC-10.
- `ResolveSSHStorage` ends with `return mainStore`, a fallback to the store that may be filesystem-backed. For an SSH host key that is survivable because the path is absolute. For a registered ZeFS key it is the defect, so the CA resolver refuses where the SSH one falls back.
- The doctor's pre-config phase was a deliberate decision, and a config-driven location contradicts it. The contradiction is the price of the operator's choice, and `tls-listeners.md` already documents a check that reads the parsed config for the same reason.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Two named arms, chosen by the leaf name | One leaf whose value is sniffed as PEM, base64 DER, or a path | A guard that decides between MATERIAL and LOCATION by looking at the string is the ambiguity `ai/rules/principles.md` names: a truncated paste reads as a path, and a bare relative path reads as base64. The leaf name is unambiguous, and the CLI can describe each arm |
| A `choice` in YANG rather than a custom validator | A `ze:validate` validator comparing the two arms | Exclusivity is what `choice` means. Native validation reaches the parser, the editor, the diff and the completion with no Go code, and two existing modules already use it |
| The file arm names a DIRECTORY holding `ca-cert.pem` and `ca-key.pem` | Two leaves each naming a file | The operator choice is where the root lives, not what its files are called. A directory Ze creates is the only shape where Ze can guarantee 0700 on the container as well as 0600 on the key. The file names are the appliance's, so one concept keeps one spelling. An operator with existing files uses the inline arm |
| The default arm is the blob store, resolved without the working directory | Deriving a path from `LoadConfigResult.ConfigDir`, the way `pointerPath` does | `ConfigDir` is the working directory in stdin mode, so it reintroduces the defect for `ze start -`. It also ties the authority's identity to where the config file sits, so moving a config file rotates the CA |
| No location resolves means the daemon refuses to start | Generating an in-memory root and warning; writing under the config file's directory | A root that changes at every restart refuses every client that trusted the last one, and a warning does not stop that. `pki-store.md` already refuses a nil issuer for the same reason: a certificate nothing issued is one no peer can validate |
| The file store is promoted from `internal/appliance` into `pki`, with read and write hooks | A second file-backed `RootStore` inside pki | Two implementations of one concept drift, and the appliance's is already the tested one. The hooks keep the appliance's passphrase behavior without forcing it on the daemon |
| The managed server takes the loaded root | Leaving its own `LoadOrGenerateRoot` call | Once the location is config-driven, a second resolution can disagree with the first and generate a second root. Passing the value removes the possibility rather than documenting it |
| The doctor check moves to the post-config phase | Keeping it pre-config and reporting only the blob arm; registering two checks | A check that reads the wrong location reports a missing root for a deployment that configured one, which is worse than reporting nothing. Two checks spend registry surface on a case that cannot arise: a daemon with a broken config does not start |
| No migration path for a root under a working-directory path | Detecting `<cwd>/meta/ca` at startup and refusing until the operator moves it | Ze is pre-release, so no deployment holds such a root (`CLAUDE.md`, owner directive 2026-08-30). A guard for a population of zero is machinery that also blesses the location it guards against. Blob-held roots, which do exist in test fixtures and on development machines, keep their identity by construction |

## Known Limitations

- A change to the `local-ca` block takes effect at the next start. The root in force is not swapped by a reload, which is what SSH already does for its host key.
- The inline arm stores a private key in the configuration file. The `$9$` encoding is obfuscation and decodes back, so the key is recoverable from the config and from every backup of it. This is the owner's decision of 2026-09-03, and the daemon states it once at startup rather than hiding it.
- The root key is protected by file permissions and nothing else on every arm: no passphrase, no hardware key store, no encryption at rest beyond what the filesystem gives. That is the posture every TLS key in Ze already has.
- Two daemons sharing one blob still replace each other's state. That exposure predates this spec and is recorded in `plan/journal/store-serializes-in-process-only.md`.
- The ownership guard reads the file's owner through the operating system. A filesystem that reports no owner is refused rather than trusted, so a store on such a filesystem needs the inline arm.

## RFC Documentation (Scope: protocol)

N-A. No protocol requirement is implemented or changed. The certificates the authority issues are unchanged in content and in encoding.

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
- [ ] AC-1..AC-15 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

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

## Review Gate

<!-- Filled at implementation time by /ze-review, per ai/rules/planning.md.
     Round 1 covers the whole diff with at least two lenses; round N+1 covers
     only the fixes round N made plus the sibling call sites they touched. -->

### Round 1
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|

### Round 2
| # | Severity | Finding | Location | Fixed by |
|---|----------|---------|----------|----------|
