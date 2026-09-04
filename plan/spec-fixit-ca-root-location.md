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

The goal is the owner's decision of 2026-09-03, whose config shape he settled on
2026-09-04: the operator chooses where the root lives, and says which choice he
made. A `local-ca` block carries three leaves. `type` names the arm, `location`
carries a filesystem directory, and `blob` carries the root material itself. Ze
uses the store it already resolves when the operator writes no block. The
working directory is never one of those locations, and a root already sitting in
one stops the daemon rather than being read or replaced.

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
  → Constraint: `applianceRootStore` maps the two registered key names onto `ca-cert.pem` and `ca-key.pem` (`internal/appliance/ca.go`, `caCertFileName` and `caKeyFileName`) and refuses any other name. Those two names are the established spelling for a root on disk, so the `location` arm writes the same two names inside the configured directory.
  → Constraint: the appliance encrypts the private half with the appliance passphrase and the daemon does not. `internal/appliance/ca.go` is not modified by this spec, so the appliance keeps its own store, its own encryption and its own page.
- [ ] `ai/patterns/config-option.md` - the structural template for a YANG leaf
  → Constraint: container names are singular nouns with no `-config` suffix, leaf names are kebab-case with no abbreviation, and every added node carries both a `description` under 96 characters and a distinct `ze:help`.
  → Decision: the decision table answers YANG rather than env var on three rows: an operator sets it, it belongs in `show configuration` and in a backup, and it is not a debug or bootstrap knob.
  → Decision: a value from a known set is an `enumeration`, so `type` is an enum and the schema refuses an unknown word with the valid list before any Go code runs. A `leaf type` carrying an enumeration is the shape `internal/component/ssh/yang/ze-ssh-conf.yang` already uses for a public key algorithm.
  → Constraint: an optional block is declared with `presence`, and a leaf required once the block is written carries `mandatory true`. `type` takes both halves of that pair, so the block stays optional and the discriminator is never absent inside it.
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
- `resolveKey` (`internal/component/config/storage/blob.go`) passes a `meta/`-prefixed name through unchanged, which is what makes the default arm safe today.
- The owner settled the config shape on 2026-09-04: one `type` leaf naming the arm, one `location` leaf, one `blob` leaf. Nothing is inferred from a value's contents, so no code reads a string to decide what kind of thing it is.
- Terminology, because one word does two jobs in this area. Inside `local-ca`, `blob` is the leaf carrying the root material. The ZeFS store is called the store, and its arm is the default arm. This spec never calls the store's arm the blob arm.
- Ze is pre-release, so a root under a working-directory path is unlikely rather than impossible. The startup guard costs one stat and is the only thing between such a file and a silent rotation, so it is written unconditionally.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/pki/ca.go` - `LoadOrGenerateRootFor` reads `zefs.KeyCACert.Pattern` and `zefs.KeyCAKey.Pattern` through the `RootStore` interface, generates and writes the pair when either half is absent, and publishes the result in `currentRoot`. Generation is serialized in-process by `rootGenerationMu`.
- [ ] `internal/component/pki/doctor.go` - `checkCARoot` reads the same two key names from `ctx.Store`, in the pre-config phase, and reports missing, half-written, unloadable and expiring roots.
- [ ] `internal/component/pki/config.go` - `ParseConfig` builds `PKIConfig` from the `pki` block: a `ca` list and a `certificate` list, each value read as PEM or as base64 DER. There is no `local-ca` node.
- [ ] `internal/component/pki/yang/ze-pki-conf.yang` - the `pki` container holds `ca` and `certificate` lists only. Certificate material is inline, and no leaf anywhere in the module names a filesystem path.
- [ ] `internal/component/pki/types.go` - `PKIConfig` holds `CACerts` and `Certificates` maps and nothing about the authority.
- [ ] `internal/component/config/storage/storage.go` - `filesystemStorage.WriteFile` calls `atomicWriteFile(name, ...)`, which runs `os.MkdirAll(filepath.Dir(path), 0o700)` and writes the file with the caller's perm. The package doc says every caller passes an absolute path.
- [ ] `internal/component/config/storage/blob.go` - `resolveKey` returns a `meta/` or `file/` prefixed name unchanged, and maps anything else to `file/active/<basename>`. This is why the default arm addresses a blob key and never a file.
- [ ] `internal/component/config/storage/pointer.go` - `pointerPath` branches on `IsBlobStorage` and joins the config file's directory with `meta/config/<pointer>` for the filesystem arm. The existing two-arm shape for the same problem.
- [ ] `internal/core/resolve/resolve.go` - `Storage` returns the blob at `<DefaultConfigDir>/database.zefs`, and a filesystem store when `ze.storage.blob` is false, when `DefaultConfigDir` is empty, or when the blob will not open.
- [ ] `cmd/ze/ze_core_start.go` - `resolveStorage` prints a warning and returns the filesystem store on a blob error. The `ze start <file>` branch replaces the blob store with a filesystem store when the blob does not hold the named config and the file exists on disk.
- [ ] `cmd/ze/hub/main.go` - `runYANGConfig` parses the `pki` block through `preparePKIConfig`, then loads the authority with `LoadOrGenerateRoot(store)` and injects it into the plugin manager. A failure is a startup failure.
- [ ] `cmd/ze/hub/managed_server.go` - `startManagedServer` calls `LoadOrGenerateRoot(store)` a second time, on the same store handle.
- [ ] `internal/component/config/infra/ssh.go` - `ResolveSSHStorage` returns the main store when it is blob-backed, otherwise opens `database.zefs` under the config directory and then under `DefaultConfigDir`, and falls back to the main store when neither opens.
- [ ] `internal/component/ssh/ssh.go` - `NewServer` defaults `HostKeyPath` to `<ConfigDir or DefaultConfigDir>/ssh_host_ed25519_key` and returns `errHostKeyPathCannotBeResolved` when neither directory resolves.
- [ ] `internal/component/config/loader.go` - `LoadConfig` sets `ConfigDir` to the config file's directory, and to the working directory when the config arrives on stdin.
- [ ] `internal/core/paths/paths.go` - `DefaultConfigDir` honors `ze.config.dir`, then maps the running binary's location to `/etc/ze`, `/perm/ze` or `<prefix>/etc/ze`, and returns an empty string for an unknown layout.
- [ ] `internal/component/config/yang/validator.go` - `walkTree` reports a missing `mandatory true` child only for a container that is present in the data, and recurses into a child only when the data carries it. A mandatory leaf inside an absent container is therefore never reported, which is what lets `local-ca` stay optional while `type` is required inside it.
- [ ] `internal/component/config/yang_schema.go` - `hasPresenceStatement` reads the YANG `presence` statement into `ContainerNode.Presence`, which is how an optional block is declared to the editor and the schema.
- [ ] `internal/appliance/ca.go` - `applianceRootStore` satisfies `RootStore` over two files named by `caCertFileName` (`ca-cert.pem`) and `caKeyFileName` (`ca-key.pem`), refuses any name that is not one of the two registered keys, and puts the private half through the appliance passphrase. This spec does not modify it.
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
- The location of the root becomes an operator choice, declared by a `type` leaf inside a `local-ca` block: `location` for a directory on the filesystem, `blob` for the root material written into the config. An absent block keeps the store Ze already resolves.
- A `local-ca` block whose `type` and leaves disagree is refused at parse time: an arm named with no leaf set for it, and a leaf set that the named arm does not read.
- A filesystem-backed config store is no longer a location for the root. The working directory is never a location.
- A daemon that can resolve no location refuses to start, where today it writes into the working directory.
- A daemon that finds `cert` or `key` under `<cwd>/meta/ca/` refuses to start on every arm, where today it reads or replaces that pair.
- The doctor check reads the configured location, so it moves from the pre-config phase to the post-config phase.
- `startManagedServer` takes the loaded root rather than resolving one.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The `pki { local-ca { ... } }` block in the operator's configuration file, parsed by the YANG-driven config parser into a `*config.Tree`. The block is a `presence` container holding three leaves and nothing else.

| Node | Type | Required | What it carries |
|------|------|----------|-----------------|
| `local-ca` | `container`, `presence` | No | The whole block. Absent means the default arm, which is the store Ze already resolves |
| `type` | `enumeration` of `location` and `blob` | `mandatory true` inside the block | Which arm is in use. Each value is the name of the leaf that arm reads, so an error can name that leaf verbatim |
| `location` | `string`, `pattern` requiring a leading slash | Only when `type` is `location` | An absolute directory Ze owns, holding `ca-cert.pem` and `ca-key.pem` |
| `blob` | `string`, `ze:sensitive` | Only when `type` is `blob` | One PEM document holding both the CERTIFICATE block and the PRIVATE KEY block of the root |

- The daemon's storage handle, chosen by `resolve.Storage` and possibly replaced by the `ze start <file>` branch in `cmd/ze/ze_core_start.go`.
- `paths.DefaultConfigDir()`, which reads `ze.config.dir` or the running binary's location.

### Transformation Path
1. `LoadConfig` (`internal/component/config/loader.go`) parses the file into a tree.
2. `preparePKIConfig` (`cmd/ze/hub/main_pki.go`) calls `pki.ParseConfig`, which now also reads the `local-ca` container into a `LocalCASource` value on `PKIConfig`: the arm named by `type`, plus the directory or the root material that arm requires. A block whose `type` and leaves disagree is an error here, and the error names the leaf.
3. `runYANGConfig` (`cmd/ze/hub/main.go`) calls `pki.ResolveRootStore(source, store)`. It refuses when `cert` or `key` sits under `<cwd>/meta/ca/`, and otherwise returns the `RootStore` for the named arm or an error. The default arm resolves in two steps: the daemon's own store when it is blob-backed, and otherwise a blob opened at `<paths.DefaultConfigDir()>/database.zefs`. Neither step reads the working directory, and when neither resolves the arm returns an error.
4. `pki.LoadOrGenerateRoot(rootStore)` reads the pair, or generates and writes it, exactly as it does today.
5. The loaded `*Root` is injected into the plugin manager and passed to `startManagedServer`.
6. `ze doctor` runs `checkCARoot` in the post-config phase, resolving the same location from the parsed tree and the store it holds.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config tree ↔ pki | `ParseConfig` reads the `local-ca` container and returns one arm name, one directory string and one PEM bundle | No |
| pki ↔ storage | The default arm passes the daemon's `storage.Storage`, which satisfies `RootStore` with no adapter | No |
| pki ↔ filesystem | The location arm owns its own `os` calls, its directory creation at 0700 and its mode and ownership guards. The stray-root guard owns one `os.Getwd` and two `os.Stat` calls | No |
| Hub ↔ managed server | The loaded `*Root` is passed as an argument instead of being resolved a second time | No |
| Daemon ↔ doctor | `ze doctor` is a separate process and resolves the location from the parsed config plus its own store handle | No |

### Integration Points
- `pki.ParseConfig` and `pki.PKIConfig` - gain the parsed arm, so every existing caller of `ParseConfig` keeps working unchanged.
- `pki.LoadOrGenerateRootFor` - unchanged, and still the only constructor. The resolution happens before it.
- `internal/core/diagnostic/codes.go` - two new codes, both `config-` prefixed because each is a startup refusal rather than a doctor finding: one for a location Ze refuses, one for a root found in the working directory. `config-bgp-peer` and the `config-yang-*` set are the precedent for the prefix.

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
| A-1 | A `meta/`-prefixed name reaches a blob key and never a filesystem path | `resolveKey` and `isNamespaced` (`internal/component/config/storage/blob.go`) | The default arm carries the same defect as a filesystem-backed store | `TestDefaultArmWritesNoFileBesideTheStore` | unvalidated |
| A-2 | The blob file is 0600 by construction, so the default arm needs no mode enforcement of its own | `atomicWrite` uses `os.CreateTemp` (`pkg/zefs/store.go`), stated in `docs/architecture/zefs-format.md` | The default arm needs the same mode guard as the location arm | existing `TestRootKeyIsWrittenPrivate` | unvalidated |
| A-3 | `LoadConfigResult.ConfigDir` is the working directory in stdin mode, so it is never a safe candidate | `LoadConfig` (`internal/component/config/loader.go`) | A candidate list that includes it reintroduces the defect for `ze start -` | `TestWorkingDirectoryIsNeverACandidate` | unvalidated |
| A-4 | `mandatory true` on `type` fires only when the operator writes the `local-ca` block, so the block stays optional | `walkTree` and `validateContainerEntry` (`internal/component/config/yang/validator.go`) report a mandatory child only for a container present in the data | Every existing config without a `local-ca` block fails validation, which breaks every deployment at once | `test/parse/pki-local-ca-absent-block-is-valid.ci` | unvalidated |
| A-5 | Only a root held in the store needs to keep its identity across this change. A root under a working directory is stopped rather than adopted | `CLAUDE.md`, owner directive 2026-08-30, plus the owner's instruction of 2026-09-04 to guard unconditionally | An operator with a working root is stopped and must move two files by hand, which is the intended cost of never rotating in silence | `TestStrayRootErrorNamesTheFileAndBothArms` | unvalidated |
| A-6 | The three hub boot tests already set `ze.config.dir` to a temporary directory, so the default arm's second step opens a blob there and none of them needs an edit | `cmd/ze/hub/main_servers_test.go` and `cmd/ze/hub/service_grpc_test.go` each set `ze.config.dir` and then pass `storage.NewFilesystem()` to `runYANGConfig`, so the first step cannot resolve and the second must | Those tests fail closed at boot and each needs a location written into it | running them | unvalidated |
| A-7 | File ownership is readable through `os.Stat` on every platform Ze builds for | `internal/appliance/crypto.go` writes secrets with no ownership test today | The ownership guard needs a platform split, or it refuses everywhere | `TestLocationArmRefusesAForeignOwner` | unvalidated |
| A-8 | The process working directory at startup is the directory the defect wrote into, so `<cwd>/meta/ca/` is where a stray root sits | `atomicWriteFile` (`internal/component/config/storage/storage.go`) resolves a relative name against the process working directory, and `LoadOrGenerateRootFor` passes the two registered key names unchanged | The guard looks in the wrong place and a stray root is still rotated in silence | `TestStrayRootInTheWorkingDirectoryRefusesStartup` | unvalidated |
| A-9 | A PEM document holding a CERTIFICATE block and a PRIVATE KEY block is what an operator can produce for the `blob` leaf | `ParseConfig` (`internal/component/pki/config.go`) already reads a PEM document for `ca` and `certificate`, and `show pki local-ca pem` prints the certificate half in that encoding | The `blob` arm cannot be filled from what Ze itself prints, so the operator has no route into it | `TestBlobArmReadsABundleZeItselfCanPrint` | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The daemon refuses to start where it used to start, because no location resolves | A `ze start` that exits 1 with the new error | The error names both leaves and `ze.config.dir`, and the change note says so. Refusing is the decision: a location Ze invents is what caused the defect |
| R-2 | The doctor's move to the post-config phase loses the authority report for a daemon whose config is broken | `ze doctor` prints nothing about the root on a config that fails to parse | Accepted and stated in `pki-store.md`. A daemon with a broken config does not start, so it has no authority to report on |
| R-3 | `type` names one arm while the other arm's leaf is the one that is set, and Ze reads the wrong one | A daemon that starts with a `blob` in the config and presents a certificate that is not it | `ParseConfig` refuses the disagreement and names the leaf. The resolver never reads a leaf the named arm does not own, so a value that is present and unused cannot reach a store |
| R-4 | A second caller resolves its own store and generates a second root | Two roots in one daemon, or a managed client refused by a hub it trusts | `startManagedServer` takes the loaded root as an argument, and `LoadOrGenerateRoot` keeps exactly one caller |
| R-5 | An operator writes a directory on a shared or world-readable volume | None at runtime, which is why the guard is at startup | Ze refuses a directory with any group or other permission bit, and refuses a key file that is not a regular file owned by the process user |
| R-6 | The blob arm puts a root private key into `show configuration` and into every backup | Nothing fails; the exposure is silent | The `blob` leaf is `ze:sensitive`, so the config file holds `$9$` and the display masks it. The daemon logs one line at load naming the exposure, in the shape `warnPlaintextOnDisk` already uses |
| R-7 | A reload changes the `local-ca` block and the running root no longer matches the config | A config the operator believes is in force | The reload path logs that the change takes effect at the next start and leaves the root in force, which is what SSH already does for its host key |
| R-8 | Two daemons share one blob and each replaces the other's keys | State that was written and is no longer there | Out of scope and already recorded in `plan/journal/store-serializes-in-process-only.md`. This spec neither widens nor narrows it |
| R-9 | The stray-root guard stops a daemon that started yesterday, on a machine where a previous run left `meta/ca/` behind | A `ze start` that exits 1 naming `<cwd>/meta/ca/key` | The error names the file, says Ze will not read or replace it, and names the two arms that can home it. Ze never deletes, moves or rotates the file it found |
| R-10 | The guard fires inside `go test ./cmd/ze/hub/...` on a developer machine that already holds the artifact `TestAPIBootWarnsExactlyWhenNoUsersAndNoToken` used to create | A hub test that exits with the stray-root refusal rather than its own assertion | That leftover is untracked and hidden by a `.gitignore` line, so it is deleted by hand once. The producing test is fixed in this spec's phase 1 so no run recreates it, which closes the row in `plan/journal/test-artifacts-land-in-the-repository-root.md` |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Every internal TLS listener that has no operator-named certificate: the plugin hub acceptor, the managed config server, and any client that trusts an exported root. A wrong location either rotates the authority, which refuses every client that trusted the old one, or writes a private key somewhere unsafe |
| How is it reverted? | A single commit revert. The default arm keeps the same keys and the same bytes, so a reverted daemon reads the root the fixed daemon wrote. A root generated into a configured directory is not visible to a reverted daemon, which would generate a new one |
| Who else touches this path? | `internal/appliance` (a second `RootStore` implementation, unmodified by this spec), `cmd/ze/hub` (two call sites), `internal/component/plugin` and `internal/component/managed` (consumers of the issued leaf). `spec-local-ca` closed on 2026-09-04 and is the spec that built this surface |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `pki { local-ca { type location; location "/abs/dir"; } }` in a config file | → | `pki.ResolveRootStore` returns the location store, which writes `ca-cert.pem` and `ca-key.pem` inside that directory | `TestConfiguredDirectoryHoldsTheRoot` |
| `pki { local-ca { type blob; blob "<PEM bundle>"; } }` | → | `pki.ResolveRootStore` returns the blob store, `LoadOrGenerateRoot` reads it and writes nothing | `TestBlobArmIsUsedAndNothingIsWritten` |
| A `local-ca` block whose `type` and leaves disagree | → | `pki.ParseConfig` returns an error naming the leaf, and the config load stops | `TestTypeAndLeafMustAgree` |
| No `local-ca` block, blob-backed daemon | → | `pki.ResolveRootStore` returns the daemon's store, keys unchanged | `TestDefaultArmKeepsTheStoreKeys` |
| No `local-ca` block, no store reachable | → | `pki.ResolveRootStore` returns an error and `runYANGConfig` exits 1 | `TestRefusesWhenNoLocationResolves` |
| A root sits at `<cwd>/meta/ca/key` | → | `pki.ResolveRootStore` refuses on every arm and `runYANGConfig` exits 1 | `TestStrayRootInTheWorkingDirectoryRefusesStartup` |
| `ze start <config>` run from a scratch working directory | → | `runYANGConfig` → `ResolveRootStore` → `LoadOrGenerateRoot` | `test/plugin/pki-root-never-lands-in-the-working-directory.ci` |
| `ze doctor` on a config that names a directory | → | `checkCARoot` in the post-config phase reads that directory | `test/ui/doctor-pki-ca-root-configured-directory.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | A daemon starts from an arbitrary working directory, with a filesystem-backed config store and no `local-ca` block | Nothing is created in that working directory. No `meta` directory, no key, no certificate |
| AC-2 | A daemon with a blob-backed store and no `local-ca` block starts, stops and starts again | The root is read from `meta/ca/cert` and `meta/ca/key` of that blob, and the second start presents the same certificate, with the same serial. A config written before this change needs no edit to keep its root |
| AC-3 | `type location` and a `location` naming an absolute directory that does not exist | Ze creates it with mode 0700, writes `ca-cert.pem` and `ca-key.pem` with mode 0600, and a restart presents the same root |
| AC-4 | `location` names a relative path | The daemon refuses to start. The error names the leaf, the value it was given, and that the path must be absolute |
| AC-5 | `location` names an existing directory whose mode carries any group or other bit | The daemon refuses to start. The error names the directory and its mode |
| AC-6 | The configured directory holds a `ca-key.pem` that is not a regular file, carries a group or other permission bit, or is owned by another user | The daemon refuses to start, naming which of the three it found. A guard that cannot read the mode or the owner refuses rather than passing |
| AC-7 | `type blob` and a `blob` holding a CERTIFICATE block and a PRIVATE KEY block | The daemon issues from that root: an issued leaf's issuer common name and authority key identifier match the configured certificate. No file and no store key is written |
| AC-8 | `blob` holds one PEM block only, either the certificate or the private key | The config is refused at parse time, naming the block that is missing |
| AC-9 | `type` names one arm and the leaf that is set belongs to the other, in each direction | The config is refused at parse time. The error names the `type` value, the leaf that arm requires, and the leaf that must not be set |
| AC-10 | No `local-ca` block, no blob-backed store, and `ze.config.dir` unset with the binary in an unknown layout | The daemon refuses to start. The error names `type location`, `type blob` and `ze.config.dir` |
| AC-11 | `ze doctor` runs against a config that names each arm in turn | The report describes the root at the configured location: absent, unloadable, or expiring, with the same codes as today plus the location in the message |
| AC-12 | A reload changes the `local-ca` block | The root in force does not change, and one log line says the change takes effect at the next start |
| AC-13 | A daemon starts the managed config server | The server uses the root the hub loaded. `LoadOrGenerateRoot` has exactly one caller in `cmd/ze` |
| AC-14 | `show pki local-ca pem` runs on a daemon using each arm in turn | The printed certificate is the one at the configured location |
| AC-15 | The blob arm is in force | One log line at startup names the exposure: the root private key is in the config file and in every backup of it |
| AC-16 | A `local-ca` block is written with no `type` leaf | The config is refused by the schema, naming `type` as mandatory and listing `location` and `blob` as its values |
| AC-17 | `type` names an arm and that arm's leaf is absent, in each direction | The config is refused at parse time, naming the `type` value and the leaf it requires |
| AC-18 | A file exists at `<cwd>/meta/ca/cert` or `<cwd>/meta/ca/key` when the daemon starts, on any arm including the default | The daemon refuses to start and names the absolute path of the file it found. It does not read, move, delete or replace that file, and it generates no new root |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Keeps the root on an encrypted volume, away from the config | config `type location` plus `location` → `ParseConfig` → `ResolveRootStore` → location store → `LoadOrGenerateRoot` | `test/plugin/pki-local-ca-location.ci` |
| 2 | Holds one root centrally and pastes it into each router's config | config `type blob` plus `blob` → blob store → `LoadOrGenerateRoot` | `test/plugin/pki-local-ca-blob.ci` |
| 3 | Writes nothing and keeps the root Ze already generated | no block → the store Ze already resolves → same two keys | `TestDefaultArmKeepsTheStoreKeys`, `test/managed/managed-hub-ca-trust.ci` unchanged and still green |
| 4 | Runs `ze start` from a checkout or a scratch directory | no block → resolved store → nothing in the working directory | `test/plugin/pki-root-never-lands-in-the-working-directory.ci` |
| 5 | Runs `ze doctor` before starting the daemon | parsed config → configured location → diagnostics | `test/ui/doctor-pki-ca-root-configured-directory.ci` |
| 6 | Upgrades a daemon that once ran from a directory holding `meta/ca/` | startup → stray-root guard → refusal naming the file | `test/plugin/pki-stray-root-in-working-directory-refused.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDefaultArmKeepsTheStoreKeys` | `internal/component/pki/rootstore_test.go` | With no config block and a blob-backed store, the resolved store reads and writes `meta/ca/cert` and `meta/ca/key` (AC-2) | |
| `TestRefusesWhenNoLocationResolves` | `internal/component/pki/rootstore_test.go` | No block, no blob-backed store, no config dir: an error, and no file created anywhere (AC-10) | |
| `TestWorkingDirectoryIsNeverACandidate` | `internal/component/pki/rootstore_test.go` | The resolver is driven from a temporary working directory and creates nothing in it, on every arm (AC-1, A-3) | |
| `TestStrayRootInTheWorkingDirectoryRefusesStartup` | `internal/component/pki/rootstore_test.go` | A `meta/ca/cert` or `meta/ca/key` planted in the working directory makes every arm refuse, and the planted bytes are unchanged after the call (AC-18, A-8) | |
| `TestStrayRootErrorNamesTheFileAndBothArms` | `internal/component/pki/rootstore_test.go` | The refusal carries the absolute path it found, and names `type location` and `type blob` as the two ways to home it (AC-18, A-5) | |
| `TestConfiguredDirectoryHoldsTheRoot` | `internal/component/pki/rootstore_test.go` | Generation into a configured directory: 0700 directory, `ca-cert.pem` and `ca-key.pem` at 0600, second load reads the same root (AC-3) | |
| `TestLocationArmRefusesARelativeDirectory` | `internal/component/pki/rootstore_test.go` | A relative value is an error naming the leaf and the value (AC-4) | |
| `TestLocationArmRefusesAGroupReadableDirectory` | `internal/component/pki/rootstore_test.go` | Modes 0750, 0705 and 0777 are each refused, 0700 is accepted (AC-5) | |
| `TestLocationArmRefusesAnUnsafeKeyFile` | `internal/component/pki/rootstore_test.go` | A key file that is a symlink, a directory, or carries a group or other bit is refused (AC-6) | |
| `TestLocationArmRefusesAForeignOwner` | `internal/component/pki/rootstore_test.go` | A key file owned by another user is refused, and a guard that cannot read the owner refuses too (AC-6, A-7) | |
| `TestBlobArmIsUsedAndNothingIsWritten` | `internal/component/pki/rootstore_test.go` | The blob arm loads the configured pair, and its `WriteFile` returns an error rather than persisting (AC-7) | |
| `TestBlobArmReadsABundleZeItselfCanPrint` | `internal/component/pki/config_test.go` | A bundle built from what `show pki local-ca pem` prints, plus the matching key, parses into both halves (A-9) | |
| `TestBlobArmRefusesAHalfBundle` | `internal/component/pki/config_test.go` | A certificate with no key block, and a key with no certificate block, are each refused by the name of the missing block (AC-8) | |
| `TestTypeAndLeafMustAgree` | `internal/component/pki/config_test.go` | `type location` with only `blob` set, and `type blob` with only `location` set, are each refused naming both leaves (AC-9) | |
| `TestTypeRequiresItsOwnLeaf` | `internal/component/pki/config_test.go` | `type location` with no `location`, and `type blob` with no `blob`, are each refused naming the leaf the type requires (AC-17) | |
| `TestDefaultArmWritesNoFileBesideTheStore` | `internal/component/pki/rootstore_test.go` | After a generation through the default arm, the blob's own directory holds only the blob file (A-1) | |
| `TestDoctorReadsTheConfiguredLocation` | `internal/component/pki/doctor_test.go` | The check reports a missing, unloadable and expiring root at a configured directory, and the message names the location (AC-11) | |
| `TestReloadDoesNotRotateTheRoot` | `cmd/ze/hub/main_reload_test.go` | A reload that changes the block leaves the loaded root in place and logs the restart requirement (AC-12) | |
| `TestManagedServerTakesTheLoadedRoot` | `cmd/ze/hub/managed_server_test.go` | The server is constructed with the hub's root, and no second resolution happens (AC-13) | |
| `TestAbsentBlockNeedsNoTypeLeaf` | `internal/component/config/yang/validator_test.go` | A config with no `local-ca` block validates, so `mandatory true` on `type` does not make the block mandatory (A-4) | |

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
| `pki-local-ca-location` | `test/plugin/*.ci` | The operator points the root at a directory, restarts, and gets the same root (AC-3, story 1) | |
| `pki-local-ca-blob` | `test/plugin/*.ci` | The operator pastes a root into the config and the daemon issues from it (AC-7, story 2) | |
| `pki-stray-root-in-working-directory-refused` | `test/plugin/*.ci` | A daemon started beside a `meta/ca/` pair refuses and leaves the files untouched (AC-18, story 6) | |
| `pki-local-ca-relative-location-refused` | `test/parse/*.ci` | A relative directory is refused with a message that names the leaf (AC-4) | |
| `pki-local-ca-type-and-leaf-must-agree` | `test/parse/*.ci` | Each disagreement between `type` and the leaves is refused, and a block with no `type` is refused by the schema (AC-9, AC-16, AC-17) | |
| `pki-local-ca-absent-block-is-valid` | `test/parse/*.ci` | A config with no `local-ca` block validates and keeps the default arm (AC-2, A-4) | |
| `doctor-pki-ca-root-configured-directory` | `test/ui/*.ci` | `ze doctor` reports the root at the configured directory (AC-11) | |
| `managed-hub-ca-trust` | `test/managed/*.ci` | The existing two-daemon trust test still passes, unchanged (AC-2, story 3) | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A | N-A | N-A | No wire-visible change. The certificates Ze issues, and the chain a peer validates, are byte-identical whatever arm holds the root. The two-daemon `managed-hub-ca-trust` test is the closest thing to an interop proof this surface has, and it stays green | |

## Files to Modify
- `internal/component/pki/ca.go` - `LoadOrGenerateRootFor` keeps its shape; the file gains nothing but a doc pointer to the resolver
- `internal/component/pki/config.go` - `ParseConfig` reads the `local-ca` container, refuses a `type` that disagrees with the leaves and a `blob` carrying one PEM block, and returns the parsed source
- `internal/component/pki/types.go` - `PKIConfig` gains the parsed arm as a value type
- `internal/component/pki/doctor.go` - the check moves to the post-config phase and resolves the configured location
- `internal/component/pki/yang/ze-pki-conf.yang` - the `local-ca` presence container, its `type`, `location` and `blob` leaves, and their `description` and `ze:help` texts
- `cmd/ze/hub/main.go` - resolve the location before loading the root, and pass the loaded root to the managed server
- `cmd/ze/hub/managed_server.go` - take the root as an argument
- `cmd/ze/hub/main_reload.go` - warn when a reload changes the block
- `internal/core/diagnostic/codes.go` - the two new refusal codes and their operator text
- `docs/architecture/pki/pki-store.md` - the two decisions this spec overturns, the three arms and their discriminator, the guards, the stray-root refusal, and the doctor phase
- `docs/architecture/zefs-format.md` - qualify "written once on the first daemon start" with the arm it describes
- `docs/architecture/hub-architecture.md` - the resolution step between the config parse and the root load
- `docs/architecture/fleet-config.md` - the managed server takes the loaded root
- `docs/guide/configuration.md` - the `local-ca` block in the PKI Certificate Store section
- `docs/config-reference.md` - the `local-ca` syntax
- `docs/features.md` - the operator can choose where the root lives

## Files to Create
- `internal/component/pki/rootstore.go` - `ResolveRootStore`, the blob store, the location store, the stray-root guard and the permission guards
- `internal/component/pki/rootstore_test.go` - the unit tests above
- `test/plugin/pki-root-never-lands-in-the-working-directory.ci` - the defect's functional proof
- `test/plugin/pki-local-ca-location.ci` - story 1
- `test/plugin/pki-local-ca-blob.ci` - story 2
- `test/plugin/pki-stray-root-in-working-directory-refused.ci` - AC-18, story 6
- `test/parse/pki-local-ca-relative-location-refused.ci` - AC-4
- `test/parse/pki-local-ca-type-and-leaf-must-agree.ci` - AC-9, AC-16, AC-17
- `test/parse/pki-local-ca-absent-block-is-valid.ci` - AC-2, A-4
- `test/ui/doctor-pki-ca-root-configured-directory.ci` - AC-11

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/pki/yang/ze-pki-conf.yang`, a `local-ca` presence container inside the existing `pki` container, holding `type`, `location` and `blob` |
| YANG validation constraints | Yes | `type` is an `enumeration` of `location` and `blob`, so an unknown word is refused with the valid list, and it carries `mandatory true` so the block cannot exist without a discriminator. The `location` leaf takes `type string` with a `pattern` that requires a leading slash, so a relative value is refused by the schema before any code runs. The `blob` leaf carries `ze:sensitive` |
| YANG custom validators | No | Neither arm needs a runtime set, and the agreement between `type` and the leaves spans two nodes, which a per-leaf `ze:validate` cannot see. `ParseConfig` refuses it instead, naming the leaf. Absoluteness is a pattern, and the permission tests are startup guards over the live filesystem rather than config validation |
| CLI commands/flags | No | No new command. `show pki local-ca pem` already prints the loaded root and keeps working on every arm |
| CLI grammar (keyword before value) | N-A | No command is added |
| Editor autocomplete | Yes | Automatic for the new leaves from their YANG types, and the `type` enum offers its two values as completion rows. The `location` leaf offers no dynamic completion, because a path on the router's filesystem is not enumerable from the editor's process |
| Functional test for new RPC/API | Yes | `test/plugin/pki-local-ca-location.ci` and `test/plugin/pki-local-ca-blob.ci` |
| Pipe completeness | N-A | No new command output |
| Env var registration | N-A | Nothing lands under `environment/`. The decision table in `ai/patterns/config-option.md` answers YANG on three rows: an operator sets it, it belongs in `show configuration` and in a backup, and it is not a debug or bootstrap knob |
| Doctor check for runtime dependencies | Yes | The existing `pki-ca-root` check moves phase and gains the configured location. Two new codes in `internal/core/diagnostic/codes.go`: `config-pki-ca-root-location` for a location Ze refuses, and `config-pki-ca-root-stray` for a root found in the working directory. Both take the `config-` prefix because each is a startup refusal rather than a doctor finding, which is how `config-bgp-peer` and the `config-yang-*` set are already named. Unit test in `internal/component/pki/doctor_test.go`, functional test in `test/ui/doctor-pki-ca-root-configured-directory.ci` |
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
| 12 | Internal architecture changed? | Yes | `docs/architecture/pki/pki-store.md`, `docs/architecture/zefs-format.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/fleet-config.md`. `docs/architecture/appliance/builder.md` is NOT edited: `internal/appliance/ca.go` is unmodified, so every sentence on that page stays true |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counter is added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The two new diagnostic codes are inventory. `docs/guide/status.md` lists the pki doctor codes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-fixit-ca-root-location.md`. The `// Design:` headers of the files named above declare `docs/architecture/pki/pki-store.md`, `docs/architecture/pki/tls-listeners.md`, `docs/architecture/hub-architecture.md`, `docs/architecture/fleet-config.md` and `docs/architecture/zefs-format.md`. Three are named as UNAFFECTED. `docs/architecture/pki/tls-listeners.md`: `internal/component/pki/tls.go` is not modified, and the named-certificate path it describes outranks issuance on every arm, unchanged. `docs/architecture/appliance/builder.md`: `internal/appliance/ca.go` is not modified, so the appliance keeps its own store, its own two file names and its own passphrase. `docs/features/ai-first.md`: `internal/core/resolve/resolve.go` was read for how the daemon's store is chosen and is not modified, because the CA resolution sits above it and reads its result |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/configuration.md` shows a `pki` block. The example gains the new container, and the pki-store page's claim that the root cannot live in config is corrected rather than left beside the new leaf |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - the location reaches the loader, and the working directory stops being one
   - Tests: `TestDefaultArmKeepsTheStoreKeys`, `TestRefusesWhenNoLocationResolves`, `TestWorkingDirectoryIsNeverACandidate`, `TestStrayRootInTheWorkingDirectoryRefusesStartup`, `TestStrayRootErrorNamesTheFileAndBothArms`
   - Files: `internal/component/pki/rootstore.go` with the resolver, the default arm, the stray-root guard and the refusal only, `internal/component/pki/types.go`, `cmd/ze/hub/main.go`, `internal/core/diagnostic/codes.go`
   - Verify: a blob-backed daemon behaves exactly as before, a filesystem-backed daemon refuses instead of writing into the working directory, a planted `meta/ca/key` stops every arm and is left byte-identical, and the tests fail before the resolver exists
2. **Phase: The location arm** - a directory the operator names
   - Tests: `TestConfiguredDirectoryHoldsTheRoot`, `TestLocationArmRefusesARelativeDirectory`, `TestLocationArmRefusesAGroupReadableDirectory`, `TestLocationArmRefusesAnUnsafeKeyFile`, `TestLocationArmRefusesAForeignOwner`
   - Files: `internal/component/pki/rootstore.go`
   - Verify: each guard is proven by a test that constructs the unsafe state and reads the refusal, and the two file names match `caCertFileName` and `caKeyFileName` in `internal/appliance/ca.go`
3. **Phase: The YANG and the parser** - the operator can write the block
   - Tests: `TestBlobArmRefusesAHalfBundle`, `TestBlobArmReadsABundleZeItselfCanPrint`, `TestTypeAndLeafMustAgree`, `TestTypeRequiresItsOwnLeaf`, `TestAbsentBlockNeedsNoTypeLeaf`, `test/parse/pki-local-ca-relative-location-refused.ci`, `test/parse/pki-local-ca-type-and-leaf-must-agree.ci`, `test/parse/pki-local-ca-absent-block-is-valid.ci`
   - Files: `internal/component/pki/yang/ze-pki-conf.yang`, `internal/component/pki/config.go`
   - Verify: `ze config validate` accepts each arm written whole, refuses every disagreement between `type` and the leaves, refuses a block with no `type`, and still accepts a config with no block at all
4. **Phase: The blob arm** - a root in the config file
   - Tests: `TestBlobArmIsUsedAndNothingIsWritten`, `test/plugin/pki-local-ca-blob.ci`
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
   - Tests: `test/plugin/pki-root-never-lands-in-the-working-directory.ci`, `test/plugin/pki-local-ca-location.ci`, `test/plugin/pki-stray-root-in-working-directory-refused.ci`
   - Files: every page in Files to Modify
   - Verify: the functional test goes red against the current binary and green against the fixed one, and every page that described the old behavior is corrected

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line, and AC-1 has a functional test rather than a unit test |
| Feature completeness | Each of the three arms reaches `LoadOrGenerateRoot` from a real config file, not from a test constructor alone. The default arm's config file is one carrying no `local-ca` block |
| Correctness | The working directory appears in no candidate list, on any arm, in any order. `LoadConfigResult.ConfigDir` is not read by the resolver |
| Correctness | A blob-backed deployment reads the same two keys and presents the same certificate before and after the change |
| Naming | The YANG leaf names, the Go field names and the documentation use one spelling for the container, the file names match the appliance's `ca-cert.pem` and `ca-key.pem`, and each `type` value is spelled exactly like the leaf it activates |
| Naming | `blob` names the config leaf holding the root material and nothing else. No comment, message, test name or page calls the ZeFS store's arm the blob arm |
| Data flow | The location is resolved exactly once per daemon, and the resolved store is not rebuilt by any second caller |
| Rule: `ai/rules/evidence.md` | Every guard fails closed. A permission or ownership test that cannot read what it needs refuses rather than passing, and a `type` that names an arm whose leaf is absent is an error rather than a fall back to another arm |
| Rule: `ai/rules/no-layering.md` | The working directory is deleted as a location, not kept as a last resort behind the new arms. No code path reaches `LoadOrGenerateRootFor` without going through `ResolveRootStore` |
| Rule: `ai/rules/stale-comments.md` | The comment in `managed_server.go` calling the second load "a load and never a second root" is corrected or removed with the call it describes |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No key is written relative to the working directory | `test/plugin/pki-root-never-lands-in-the-working-directory.ci`, and `go test ./cmd/ze/hub/...` followed by `git status --porcelain cmd/ze/hub` reporting nothing |
| One caller of the root loader in the daemon | `grep -rn 'LoadOrGenerateRoot(' cmd/ | grep -v _test` returns one line |
| The two file names have one spelling | `grep -rln 'ca-key.pem' internal/ | grep -v _test` returns exactly `internal/appliance/ca.go` and `internal/component/pki/rootstore.go`, and no third file |
| The default arm is unchanged for an existing store | `test/managed/managed-hub-ca-trust.ci` passes with no edit, and no config file under `test/` gains a `local-ca` block to keep working |
| A root in the working directory stops the daemon | `test/plugin/pki-stray-root-in-working-directory-refused.ci`, and the planted files compare byte-identical after the run |
| Every new node carries both texts | `./le docvalid help-shape` |
| The pages match the code | `./le spec citation anchors spec plan/spec-fixit-ca-root-location.md` reports no unnamed owner |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The `location` value is operator input that becomes a filesystem path. It must be absolute, must not be resolved through a symlink into another user's directory, and must never be joined with anything derived from a peer. Ze appends only the two constant file names to it |
| Input validation | `type` is an enum, so an unknown word is refused by the schema. No code infers an arm from the shape of a value, and no arm is reached by a leaf being set on its own |
| Permissions | The directory is created 0700 and refused when it exists with any group or other bit. The key file is written 0600 and refused when it carries a group or other bit, is not a regular file, or is owned by another user |
| Fail closed | Every guard that cannot answer refuses. A stat that fails, an owner that cannot be read, a location that does not resolve, a `type` whose leaf is absent: each is an error and never a permissive default |
| Fail closed | A root found under the working directory stops startup on every arm. Ze never reads it, never adopts it and never replaces it, because a silent rotation refuses every client that trusted the old authority |
| Error leakage | A refusal names the path and the mode, which the operator needs, and never the key material. The stray-root refusal names the file it found and not its contents |
| Exposure | The blob arm puts a private key in the config file, in `show configuration` output and in every backup. The `blob` leaf is `ze:sensitive` and the daemon says so once at startup |
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
- An explicit discriminator makes two shapes writable that a mutually exclusive schema construct could not express: a `type` naming one arm while the other arm's leaf is the one that is set, and a `type` naming an arm with no leaf set for it. Both are now the spec's to refuse, at parse time and by name, which is what AC-9 and AC-17 exist for. The gain is that no code reads a value to decide what kind of thing it is.
- The stray-root guard is the only place in this change where Ze looks at the working directory, and it looks there to REFUSE. Every other path treats the working directory as absent.

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Three leaves and an explicit discriminator: `type`, `location`, `blob` | - | The owner's decision of 2026-09-04. Nothing is inferred from a value's contents, so no code reads a string to decide what kind of thing it is |
| The `type` values are `location` and `blob`, spelled exactly like the leaves they activate | `file` and `inline`, which describe the arm rather than naming its leaf | The value names the leaf that carries the answer, so a refusal can quote a word the operator finds in his own config, and completion offers the same two words at both nodes. It also removes the second question an operator would otherwise ask, which is which leaf `type file` reads |
| `blob` inside `local-ca` never means the ZeFS store | Naming the store's arm as a third `type` value | The store arm is the ABSENT block, so it carries no name inside this container and the word `blob` there can only mean the material. A third value would give the absent case two spellings, and an operator writing the block to say "use the default" gains nothing over writing nothing |
| `type` is an `enumeration` carrying `mandatory true`, inside a `presence` container | A `string` leaf with a runtime validator | The schema refuses an unknown word with the valid list before any Go code runs. `presence` keeps the block optional, and `walkTree` reports a mandatory child only for a container the data carries, so the two statements together do not make every existing config invalid |
| The `location` arm names a DIRECTORY holding `ca-cert.pem` and `ca-key.pem` | Two leaves, one for each file | A CA root is a certificate AND a private key, and one leaf cannot name two files unless it names their directory. A directory Ze creates is also the only shape where Ze can guarantee 0700 on the container as well as 0600 on the key. The file names are the appliance's (`caCertFileName`, `caKeyFileName`), so one concept keeps one spelling |
| The `blob` leaf carries ONE PEM document holding both blocks | Two leaves, a certificate and a private key | The same reading as the directory, in the other arm: one leaf, both halves. A PEM document concatenating a CERTIFICATE block and a PRIVATE KEY block is the ordinary bundle, and the certificate half is exactly what `show pki local-ca pem` prints, so the operator can build the value from Ze's own output |
| A `type` that disagrees with the leaves is refused at parse time, by name | Reading the leaf the `type` names and ignoring the other | An ignored leaf is a config the operator believes is in force, which is the silently-wrong value `ai/rules/principles.md` bans. The refusal names the `type` value, the leaf that arm requires and the leaf that must not be set, so the operator is told which line to delete |
| The default arm is the store, resolved without the working directory, in two steps: the daemon's store when it is blob-backed, then a blob at `<DefaultConfigDir>/database.zefs` | Deriving a path from `LoadConfigResult.ConfigDir`, the way `pointerPath` does | `ConfigDir` is the working directory in stdin mode, so it reintroduces the defect for `ze start -`. It also ties the authority's identity to where the config file sits, so moving a config file rotates the CA. The two-step shape is `ResolveSSHStorage`'s, minus its final fall back to a store that may be filesystem-backed |
| No location resolves means the daemon refuses to start | Generating an in-memory root and warning; writing under the config file's directory | A root that changes at every restart refuses every client that trusted the last one, and a warning does not stop that. `pki-store.md` already refuses a nil issuer for the same reason: a certificate nothing issued is one no peer can validate |
| The managed server takes the loaded root | Leaving its own `LoadOrGenerateRoot` call | Once the location is config-driven, a second resolution can disagree with the first and generate a second root. Passing the value removes the possibility rather than documenting it |
| The doctor check moves to the post-config phase | Keeping it pre-config and reporting only the default arm; registering two checks | A check that reads the wrong location reports a missing root for a deployment that configured one, which is worse than reporting nothing. Two checks spend registry surface on a case that cannot arise: a daemon with a broken config does not start |
| A root found under `<cwd>/meta/ca/` stops startup, on every arm, and is left untouched | Reading it and adopting it as the root; deleting it; moving it into the resolved location; warning and generating a new one | This is the one path where the change could rotate an authority in silence and refuse every client that trusted it. Adopting the file keeps the defect the spec exists to remove, and moving or deleting a private key without being asked is destructive. Refusing costs one `os.Getwd` and two `os.Stat` calls at startup, names the file, and leaves the operator to place it under `location` or paste it into `blob` |
| The stray-root guard is a startup refusal, not a doctor check | Adding a third doctor code and reporting it from `ze doctor` | `ze doctor` is a separate process with its own working directory, so it would report on a directory the daemon never uses. The guard belongs where the daemon's own working directory is the one that matters |

## Known Limitations

- A change to the `local-ca` block takes effect at the next start. The root in force is not swapped by a reload, which is what SSH already does for its host key.
- The blob arm stores a private key in the configuration file. The `$9$` encoding is obfuscation and decodes back, so the key is recoverable from the config and from every backup of it. This is the owner's decision of 2026-09-03, and the daemon states it once at startup rather than hiding it.
- The stray-root guard looks in the process working directory alone. A root the defect wrote while the daemon ran from a different directory is not found, and the daemon starts normally on the resolved arm. The guard covers the directory the daemon is starting in, which is the one it would have written into again.
- The root key is protected by file permissions and nothing else on every arm: no passphrase, no hardware key store, no encryption at rest beyond what the filesystem gives. That is the posture every TLS key in Ze already has.
- Two daemons sharing one blob still replace each other's state. That exposure predates this spec and is recorded in `plan/journal/store-serializes-in-process-only.md`.
- The ownership guard reads the file's owner through the operating system. A filesystem that reports no owner is refused rather than trusted, so a store on such a filesystem needs the blob arm.

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
- [ ] AC-1..AC-18 all demonstrated
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
