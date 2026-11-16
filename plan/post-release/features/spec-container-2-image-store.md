# Spec: container-2-image-store

| Field | Value |
|-------|-------|
| Status | design |
| Scope | plugin |
| Priority | P3 |
| Release reason | A post-release feature by owner ruling (umbrella O-3): Ze fetches and stores OCI images itself. No shipped behavior is wrong without it. |
| RFC gaps | - |
| Depends | spec-container-1-runtime |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

This is the "Image store" child of `plan/post-release/features/spec-container-0-umbrella.md`. The umbrella's design and the owner's rulings are fixed and are not repeated as open questions here.

The runtime child (`spec-container-1-runtime.md`) starts a container from a root folder and a description. It has nothing to start until an image is on disk. This child puts it there, keeps it immutable, and gives each container a writable layer and its volumes above it.

The goal, in what an operator sees:

- An explicit command fetches an image from a registry. Every byte Ze stores was checked against its SHA-256 digest and its declared size before Ze parsed it.
- Each image is one folder named by its digest, never changed after it is published. A fetch that stops halfway leaves nothing that reads as an image.
- A container that writes in its root writes into a layer of its own, on disk, keyed by its name and its image's digest. The layer survives a restart and a reboot, has a size cap the kernel enforces, and a command clears it.
- A volume the operator declared is a folder in the same store, with its own cap. It survives an image update, a rollback and a cleared layer.
- An update fetches the new digest, swaps, restarts the container on a fresh layer and keeps exactly one previous generation. A rollback returns the previous image with the writes it had.
- A container whose image is absent stays stopped, says which image it waits for, and starts with no further command once that image is fetched.
- Doctor reports every problem class this child can produce, each with the command that fixes it.

### Scope of this child

| # | Part | In this child |
|---|------|---------------|
| 1 | Registry client, pull only | Yes |
| 2 | Registries as configuration, and TLS trust | The behavior. The schema is `spec-container-4-operator-surface.md`, table S-3 |
| 3 | Fetch, update, rollback, delete and clear commands, and their progress | The behavior and the handlers. The command words are `spec-container-4-operator-surface.md`, table C-1 |
| 4 | Layer application into one folder per digest | Yes |
| 5 | The data directory and the store layout | Yes |
| 6 | The writable layer and its size cap | Yes |
| 7 | Retention, rollback, update end to end | Yes |
| 8 | Free-space checks, accounting, doctor checks, state keys | Yes |
| 9 | The seam to the runtime child | Yes, stated in plain words below |
| 10 | Volumes: where they are stored, their cap, their listing and deletion | Yes. The operator-surface child marks the volume node and its two commands as this child's. The brief for this child did not list them: see the report |
| - | Mounting the overlay and the volumes, starting the process, merging the image's configuration with the operator's | No: `spec-container-1-runtime.md` |
| - | Building an image, pushing an image, image signatures | No: see Known Limitations |

### Umbrella items this child owns

| Umbrella item | This child's rows |
|---------------|-------------------|
| AC-2 (image absent at commit) | AC-9 |
| AC-3 (explicit fetch, digests checked, folder per digest) | AC-1 to AC-8 |
| AC-4 (writes survive restart and reboot) | AC-10 |
| AC-5 (update and rollback) | AC-11 to AC-14 |
| R-6 (a layer writes outside its tree or expands without bound) | AC-6, AC-7, and the `TestApply...` tests |
| R-8 (file capabilities lost in the image format) | AC-6, `TestApplyXattrFileCapabilityKept`, the `container-store` QEMU action |
| R-14 (the persistent partition fills) | AC-15, AC-17, AC-11 retention |
| A-6 (the persistent partition can hold images) | Settled by reading, see assumption A-4 |

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md` - the design every subsystem shares
  → Constraint: every plugin also runs as a separate process. The store reads and writes its state through the SDK state calls, answers doctor through its own callback, and never calls into a component's Go state
  → Constraint: a compile tag is a boundary. The registry client and the store live inside the gated container package, because a core leaf cannot be compiled out
  → Constraint: a backend that cannot apply a config exactly refuses it at verify. A writable layer or a volume whose cap the kernel cannot enforce is refused, never approximated by a measured cap
- [ ] `ai/doors/system-plugins.md` - the edge plugin tier
  → Decision: the plugin owns its commands, YANG, doctor checks and help in its own folder. No central verb package spells them
  → Constraint: a removed config root arrives as an empty body. Reading it as "nothing to apply" leaves containers and layers of a deleted config running
- [ ] `ai/doors/core.md` - shared libraries and runtime state
  → Constraint: runtime state goes through the one store under keys registered in `pkg/zefs/keys.go`, granted to the plugin with `statestore.RegisterPluginKeys`. A raw file write under the scanned roots needs an allowlist entry with its reason
  → Constraint: the core tier cannot be compiled out and imports nothing upward. Only two small exports land in core: the data directory resolver and the stall reader
- [ ] `ai/doors/appliance.md` - the device
  → Constraint: an image in the field is the contract for a path under the persistent partition and for a state key. The store path, its folder names and the state key patterns are permanent once shipped
  → Constraint: `ze appliance build` is build-host tooling. The format of the persistent partition is decided there, in `injectZeFS`, and a device already installed keeps the format it was built with
- [ ] `ai/doors/config-yang.md` - configuration
  → Constraint: `ze config validate` never calls a live verify. A refusal that must hold offline is an in-process verifier or a kernel capability enrolment, not only a verify callback
  → Constraint: a plugin reads another root only by declaring it. The store reads `pki` to find a registry's trust anchor, as a declared read that never auto-loads it
- [ ] `ai/doors/plugin-infra.md` - the plugin protocol
  → Constraint: a plugin command is bounded, 30 seconds unless it declares its own bound (`DefaultCommandTimeout`, `CommandDef.Timeout`). A fetch takes minutes, so the fetch and update commands declare theirs
  → Constraint: state calls are refused until the startup handshake ends, so the store reads its generation records after startup and never from its config stage
- [ ] `ai/doors/le-tooling.md` - tests and tooling
  → Constraint: a `.ci` that needs a Linux kernel feature carries `option=needs-linux`. A new integration package is added to `integrationPackages`, or it never runs
- [ ] `ai/doors/access-security.md` - trust anchors
  → Decision: `pki ca <name>` is the one place a trust anchor is configured. A registry names an entry there. No second certificate store is added
- [ ] `ai/doors/hub-engine.md` - daemon startup
  → Constraint: what `ze config validate` accepts is what boots. The quota refusal has to give one verdict at validate, start and reload
- [ ] `docs/architecture/doctor-and-health-checks.md` - doctor
  → Constraint: a check over state the plugin's process holds is a `runtime` check declared at Stage 1 and answered by the plugin's doctor callback. A `registry.DoctorCheckDef` carries a config phase only
  → Constraint: a transient problem is raised on the report bus under a registered event code. A state is reported by a check. One fault gives one row
  → Decision: the kernel capability enrolment gives one verdict to doctor, daemon start, reload and `ze config validate`. The storage quota uses it
- [ ] `docs/architecture/zefs-format.md` - runtime state
  → Constraint: a key parameter matches one path segment, and a value is bounded at 1 MiB unless its key declares more. An image reference holds a slash and a colon, so a key carries its SHA-256, and the reference is in the value
- [ ] `docs/architecture/appliance/self-update.md`, `docs/architecture/appliance/on-device-installer.md` - the fetch helper and its redirect rule
  → Constraint: `fetch.SameOrigin` refuses a redirect that leaves the first host, by design. A registry sends blobs to another host, so the registry client has its own redirect rule and does not weaken that one
  → Decision: the self-update keeps one previous binary and its history in the state store. The same keep-one-previous step and the same history shape carry over per container
- [ ] `ai/rules/platform-linux.md` - Linux-only code
  → Constraint: Linux-only code ships with integration tests that run in the QEMU guest on Ze's runtime kernel, and each kernel option the feature needs is in `gokrazy/kernel/runtime.config` and `gokrazy/kernel/runtime.require`
  → Constraint: a kernel behavior that forces the design is an entry in `docs/architecture/linux-limitations.md`, and a new QEMU action has a caller in the same change
- [ ] `ai/rules/go-standards.md` - Go rules
  → Constraint: a built artifact is reused only when a completion mark written after a successful run proves it complete. The image folder carries that mark, written last, and a downloaded blob takes its digest as its name only after it was checked
  → Constraint: shipped code runs no external program. The partition's format cannot be changed on the device, because that needs a tool the appliance does not carry
- [ ] `ai/META-DECISIONS.md`, "Code and dependencies" - modules
  → Decision: a module already in `go.mod` may be used in full, and a new module lands in a commit of its own. The zstd decoder is already vendored. The registry client adds no module

### Upstream source (reference, not a dependency)
- [ ] The upstream container runtime at tag v1.5.3, its `go.mod` and its vendored path-safety library - what the runtime child brings
  → Decision: layer application resolves every entry through that library's descriptor-based in-root open and directory creation. This child adds no path-safety code of its own

### Sibling spec
- [ ] `plan/post-release/features/spec-container-4-operator-surface.md`, tables S-1, S-3, C-1 and D-1 - the nodes and commands this child implements
  → Decision: an image reference is whole: a registry host, a repository, and a tag or a digest. There is no default registry and no implied tag
  → Decision: the fetch and update commands answer once, at the end, as the firmware download does. Progress is a row of the image listing
  → Constraint: a registry's transport is HTTPS or HTTP. Unverified TLS does not exist. A registry has at most one mirror

**Key insights:** (minimal context to resume after compaction)
- No data directory exists today. `ConfigDirFromBinary` (`internal/core/paths/paths.go`) answers the config directory only. This child adds a data directory beside it.
- The persistent partition is formatted once, on the build host, by `injectZeFS` (`internal/appliance/cmd_build.go`), with no quota feature. The installer writes the image to the disk as it is and never changes partition 4. gokrazy mounts it with no option. Nothing grows it.
- The kernel has quota support today only by upstream default. It is in neither kernel file.
- The core fetch package gives the digest helpers and the stall reader. Its file fetch has a 30 second total timeout, its stream fetch creates no file and cannot be cancelled, and it sends no credential. The registry client is new code that reuses the helpers.
- The firmware commands block inside the daemon under the request's context (`internal/plugins/update-cmd/cmd/firmware.go`). A plugin command has a 30 second bound unless it declares one, so the fetch and update commands declare theirs.
- The zstd decoder is vendored and in `go.mod`. The archive reader and gzip are standard library.
- The appliance trusts the certificate bundle the build host had on build day: the packer copies it into the image, or takes a bundled fallback when the host has none.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/core/fetch/fetch.go` - `ToFile` writes a file of at most 1 GiB through a client with a 30 second total timeout. `ToDisk` streams to a path it does not create, under `stallReader`, with no size cap. Both retry three times from byte zero, take no context from their caller, and follow redirects only through `SameOrigin`. `ValidSHA256` and `CheckSHA256` check a hex digest of a file. `stallReader` is unexported
- [ ] `internal/core/fetch/scheme.go` - the scheme table of `ze init --from`: http, https and tftp to a file. A registry reference is not a URL of this table
- [ ] `internal/core/diskspace/diskspace.go` - `Free` answers the bytes available on the filesystem that holds a path, walking up to the first path that exists. `Stat` answers the block counts
- [ ] `internal/core/paths/paths.go` - `ConfigDirFromBinary` maps the binary's location to the system config folder, to the persistent partition on gokrazy, or to the config folder of the binary's own prefix. `DefaultConfigDir` applies the `ze.config.dir` override. No data directory is defined
- [ ] `internal/appliance/cmd_build.go` - `injectZeFS` formats the last GPT partition as ext4 with 4096-byte blocks and metadata checksums off, then writes the seed database with debugfs. No quota or project feature is asked for. The image size comes from `image.size`
- [ ] `internal/appliance/config.go` - the default image is 2 GiB, and the accepted range is 512 MiB to 64 GiB
- [ ] `internal/appliance/diskverify.go` - `verifyInject` reads the partition image back, runs a structural check, and mounts it read-only where the build runs as root
- [ ] `internal/install/disk/run.go` - `runHTTP` streams the image to the disk, re-reads the partition table, waits for partition 4 and calls `mountInjectDB`. `runISO` writes the image and powers off. Neither changes the partition table or the filesystem size
- [ ] `internal/install/disk/system.go` - `mountInjectDB` mounts partition 4 read-write as ext4 and keeps or writes the seed. It does not resize
- [ ] `gokrazy/modcache/github.com/gokrazy/gokrazy@v0.0.0-20260703061218-a4a45a20149d/mount.go` - init mounts the persistent partition as ext4 with no flag and no option, then creates the folder its var link resolves to
- [ ] `vendor/github.com/gokrazy/tools/internal/packer/cacerts.go` - `findSystemCertsPEM` copies the build host's certificate bundle into the image, or a bundled fallback when the host has none
- [ ] `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require`, `gokrazy/kernel/kernel.config`, `gokrazy/kernel/kernel.require` - no quota option is requested or required in any of the four
- [ ] `internal/plugins/update-cmd/cmd/firmware.go`, `internal/plugins/update-cmd/cmd/register.go` - the firmware handlers are builtin RPCs that call the update backend under the request's context and answer when it returns. Progress is the backend's status, read by a separate show command
- [ ] `internal/component/config/system/selfupdate.go` - `downloadBinary` writes a temporary file beside the target with a 500 MB bound. `checkDiskSpace` needs twice the binary size and skips the check when the filesystem cannot be read
- [ ] `internal/component/plugin/server/command_registry.go` - a plugin command has a per-command timeout, 30 seconds when it declares none
- [ ] `internal/component/plugin/server/handler.go` - a streaming command declares what it answers on a one-shot surface: one sample, or nothing
- [ ] `internal/core/statestore/statestore.go`, `internal/core/statestore/permissions.go`, `pkg/plugin/sdk/sdk_state.go`, `pkg/zefs/keys.go` - a plugin persists through `StateGet`, `StatePut`, `StateRemove`, `StateList` and `StateIncrement` under registered keys its package was granted. No key for containers exists
- [ ] `internal/le/arch/fspersistence/fspersistence.go` - every raw write in shipped plugin, component, daemon and core code is refused unless its file is in `fileAllowlist` with a reason
- [ ] `internal/component/kernelcap/kernelcap.go` - `MustRegister` enrols one capability with `InUse` and `Probe`, and registers its doctor check
- [ ] `internal/component/doctor/checks_storage.go` - `checkDiskSpace` reports the config filesystem below 5% available
- [ ] `internal/component/pki/yang/ze-pki-conf.yang` - `pki ca <name>` holds a trust anchor other blocks name
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - the `sensitive` extension marks a leaf whose value is obfuscated in display output

**Behavior to preserve:**
- `fetch.SameOrigin` and every caller of it: the installer, `ze init --from` and the self-update keep refusing a redirect to another host.
- `ze appliance build` still produces an image whose persistent partition mounts, holds the seed with root ownership and strict modes, and passes `verifyInject`.
- A device installed before this work keeps booting and keeps its config. Only the container feature is refused on it.
- The config directory, its modes and the live store under it are not touched. No image byte is written under the config directory.
- A build without the container gate carries no code of this child, apart from the two core exports.

**Behavior to change:**
- `ze appliance build` formats the persistent partition with the quota and project features.
- The runtime kernel requests and requires quota support.
- The core fetch package exports its stall reader.
- The core paths package gains a data directory.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| URL fetch, digest check, stall timeout | `internal/core/fetch/fetch.go` `ToFile`, `ToDisk`, `SameOrigin`, `ValidSHA256`, `CheckSHA256`, `stallReader` | Fetches a URL to a file or a device with an optional SHA-256 and a same-origin redirect rule. No credential, no cancellation | Extends in part: `ValidSHA256` and the stall reader are reused, the stall reader becomes exported. `SameOrigin` does not apply and is not changed: a blob redirect to another host is correct for a registry. The registry client is new code in the gated package |
| URL and credential redaction | `internal/core/redact/redact.go` `URL` | Removes the whole userinfo from a URL before it is printed | Extends by use: every URL the client logs or returns goes through it. Tokens and passwords are never formatted at all |
| Config directory resolver | `internal/core/paths/paths.go` `ConfigDirFromBinary`, `DefaultConfigDir` | Maps the binary's location to the config directory, with an env override | Extends: a data directory resolver of the same shape, beside it, with its own override |
| Runtime state store and plugin grants | `pkg/zefs/keys.go`; `internal/core/statestore/permissions.go` `RegisterPluginKeys`; `pkg/plugin/sdk/sdk_state.go` `StatePut` | Small runtime state under registered keys, reached by a plugin through the daemon | Extends: three key patterns for bindings, generations and the project-id counter. Does not apply to image bytes, layers and volumes: they are artifacts on the data directory |
| Raw-write gate | `internal/le/arch/fspersistence/fspersistence.go` `fileAllowlist` | Refuses a raw file write in shipped code unless its file is listed with a reason | Extends: one entry for each new file that holds a write primitive, as the self-update binary has |
| Free-space helpers | `internal/core/diskspace/diskspace.go` `Free`, `Stat` | Answers free bytes and block counts of a filesystem | Extends by use, before a fetch and in the store's doctor check. Unlike `checkDiskSpace` of the self-update, a filesystem that cannot be read is a refusal, not a skipped check |
| Config-partition doctor check | `internal/component/doctor/checks_storage.go` `checkDiskSpace` | Warns below 5% available on the config filesystem | Does not replace it. The store keeps its own reserve so a fetch cannot take the config filesystem below that threshold, and adds its own check for the data directory |
| Firmware update commands | `internal/plugins/update-cmd/cmd/firmware.go` `handleFirmwareDownload`; `docs/architecture/appliance/self-update.md` | A builtin RPC blocks under the request's context while the backend downloads, answers once, and a show command reads the status meanwhile | Extends: the fetch and update commands answer once at their end, and the image listing shows the progress meanwhile. Added: the plugin command declares its own bound, because the default is 30 seconds, and a fetch that stops keeps its checked blobs for the next attempt |
| Streaming commands and their one-shot answer | `internal/component/plugin/server/handler.go` `RegisterPluginStreamingHandler`, `OneShotSample` | A stream on SSH and `ze cli`, and one sample of the same producer elsewhere | Does not apply: progress is a row of the image listing, which every surface reads the same way. No stream is added by this child |
| Self-update keep-one-previous and history | `internal/component/config/system/selfupdate.go` `stageBinary`, `Rollback`, `recordEvent` | Keeps one previous binary, rolls back by hand, keeps 20 history events in the state store | Model, not extended: its registry is a system singleton. The same step and the same bound are applied per container |
| Kernel capability enrolment | `internal/component/kernelcap/kernelcap.go` `MustRegister`, `Capability` | One declaration judged by doctor, start, reload and validate | Extends: one enrolment for the storage quota, whose probe reads the data directory's filesystem |
| Doctor checks and codes | `internal/core/diagnostic/doctor_registry.go` `RegisterDoctorCheck`; `internal/core/diagnostic/registry.go` `Register`; `internal/core/diagnostic/event_codes.go` `RegisterEventCode`; `internal/plugins/ldp/register.go` `OnDoctorCheck` | A check registers from its owner. A plugin's live state is answered by its own callback. A transient code is registered as an event code | Extends: the checks and codes in the table under Data Flow |
| Trust anchors | `internal/component/pki/yang/ze-pki-conf.yang` `list ca` | Named trust anchors other blocks reference | Extends by reference: a registry names one. No new certificate store |
| Sensitive values | `internal/component/config/yang/modules/ze-extensions.yang` `extension sensitive` | Obfuscates a leaf in display output | Extends by use: the registry password leaf carries it |
| Persistent partition format | `internal/appliance/cmd_build.go` `injectZeFS`; `internal/appliance/diskverify.go` `verifyInject` | Formats ext4 on the build host and verifies the result | Extends: two more filesystem features. The verification is unchanged and still has to pass |
| Exclusive publication by rename | `pkg/zefs/check_tree_linux.go` `RenameNoReplace` | Renames beneath a retained directory and fails when the target exists | Extends by use: a finished image folder is published with it |
| Appliance trust roots | `vendor/github.com/gokrazy/tools/internal/packer/cacerts.go` `findSystemCertsPEM` | Writes the build host's certificate bundle into the image | Does not change. The standard library reads that bundle. A registry with a private authority names a `pki ca` entry |
| A server for a package test | `internal/le/interoplab/labtest/labtest.go` `Start`; `docs/architecture/testing/interop.md`, "A package test's own server container (labtest)" | Starts one real server container for a `_test.go` file | Extends by use: the interop test starts a real registry with it |
| A local server inside a functional fixture | `internal/test/fixture/storage_consumer_restart.go` `storageHistoryRestart` | A compiled fixture serves a local HTTP manifest to the daemon under test | Model: a compiled fixture serves a local registry to the daemon in the `.ci` tests |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Commands, each a registered command of the container plugin with a schema node, reached from every surface: fetch an image, list images, update a container's image, roll a container back, delete an image, clear a container's writable layer, list volumes, delete a volume.
- Config: the registry list, and each container's image reference, writable layer size and volumes, delivered through the config transaction.
- The registry: manifests, image configurations and layer blobs over HTTPS.
- The runtime child: a request to resolve a container to its folders.

### Transformation Path
1. The operator names an image reference: a registry host, a repository, and a tag or a digest. The store refuses any other form.
2. The fetch command hands the fetch to the one worker that owns every change to the store, and waits for it under the request's context. Fetches run one at a time, and at most 8 wait.
3. The client asks the registry for the manifest. On an authentication challenge it gets a token from the realm the challenge names, with the registry's credentials when the config holds them. A mirror, when configured, is asked first and without those credentials.
4. An index is resolved to the manifest of the running platform. The manifest's digest names the image.
5. An image already in the store under that digest is bound to the reference and nothing is downloaded.
6. Otherwise the store checks free space and downloads the image configuration and each layer blob. A blob is checked against its digest and size before any byte of it is decompressed or parsed, and takes its digest as its name only then. A blob already present under its digest is not downloaded again.
7. Each layer is decompressed and applied, in order, into a staging root. Every entry is resolved under a descriptor for that root. The decompressed stream is checked against the digest the image configuration declares for it.
8. The store writes the completion mark, syncs, and renames the staging folder into the image folder, refusing an existing target. It removes the blobs and binds the reference to the digest in the state store.
9. The store tells the runtime that the reference is now present. Each container that waited for it starts, and the command's answer names them.
10. When the runtime starts a container, it asks the store to resolve it. The store answers the image folder, the image's configuration, the writable layer's folders and the folder of each declared volume, after it created them and set their caps.

### Reference forms

| Operator writes | Registry | Repository | Pin |
|-----------------|----------|------------|-----|
| A registry host, a path and a tag | The host | The path | The tag, resolved to a digest at fetch and at update only |
| A registry host, a path and a digest | The host | The path | The digest, never resolved again |
| Anything else: no host, no tag and no digest | - | - | Refused by the schema of the operator-surface child, and by the store for a reference a command carries |

A tag is never resolved at commit, at start or at boot. The binding made by the last fetch or update decides which digest a container runs.

### Registry requests

| Step | Request | Checks | Bound |
|------|---------|--------|-------|
| Manifest or index | By tag, or by digest | Media type is one of the four accepted. Bytes hash to the digest asked for, or to the digest the answer declares. Size matches the descriptor when one exists | 4 MiB |
| Token | To the realm of the challenge, over HTTPS unless the registry's transport is HTTP | The answer holds a token | 64 KiB |
| Image configuration | By digest | Digest and size. Operating system is Linux and the architecture is the running one | 4 MiB |
| Layer blob | By digest | Digest and size, before parsing | The manifest's declared size, and the free-space rule |
| Redirect | Followed up to 10 times | The authorization header is sent only to the exact host and port it was obtained for. A redirect from HTTPS to HTTP is refused | 10 |
| Retry | A transport error or a server error is retried, 3 attempts with a doubling delay | A digest or size mismatch is not retried: it is another file | 3 |
| Stall | No byte for 60 seconds ends the transfer | The stall reader of the core fetch package | 60 s |
| Mirror | Asked first, for manifests and blobs, with no credentials | The same digest and size checks. When the mirror fails, the registry itself is asked | One mirror for each registry |
| Trust | The system roots, or the one `pki ca` entry the registry names | A registry that names an anchor is trusted through that anchor only | - |

Accepted manifest types: the OCI image index, the OCI image manifest, the Docker manifest list and the Docker image manifest, both of schema 2. Accepted layer types: uncompressed tar, gzip and zstd. Refused, naming the type: schema 1, a nested index, a layer distributed from a foreign URL, an encrypted layer, a manifest that is not a container image, and any digest algorithm other than SHA-256.

Platform selection takes the entry whose operating system is Linux and whose architecture is the running one, amd64 or arm64. An arm64 entry of variant v8 equals one with no variant. An amd64 entry that names a feature level above the baseline is not selected. With no matching entry the fetch fails and names the platforms the index offers.

### Layer application rules

| Entry or case | What the store does |
|---------------|---------------------|
| Regular file | Removes what the name held, creates a new file by descriptor, writes it, sets owner, mode, times and attributes. It never writes through an existing inode |
| Directory | Creates it, or keeps an existing directory and updates its metadata. Its times are set after its children |
| Symbolic link | Creates it with its target as written. The target is not resolved at this time |
| Hard link | Resolves the target under the root. The target must exist and be a regular file inside the root, otherwise the image is refused |
| A parent component that is a symbolic link | Resolved inside the root, as a process in the container would resolve it |
| Whiteout of a name | Removes that name and everything under it |
| Opaque directory marker | Removes every child the directory had before this layer. Children this layer adds stay, whatever their order in the archive |
| Any other marker of the whiteout family | Not created |
| Device node under the image's device folder | Not created, and counted in the image's record: the runtime mounts its own device folder there, so the node could never be seen |
| Device node anywhere else | The image is refused, naming the entry |
| Named pipe | Created |
| Owner | The numeric user and group of the entry. Names are ignored |
| Mode | As written, the set-user, set-group and sticky bits included, applied after the owner |
| Extended attribute | Kept for the file-capability attribute, the user namespace and the two POSIX access-list names. An overlay control attribute, in the trusted or the user namespace, refuses the image. Any other name refuses the image, naming it |
| A path with a parent-directory component, an empty component or a NUL | The image is refused. Nothing is corrected |
| A name over 255 bytes, a path over 4096 bytes, attributes over 64 KiB on one entry | The image is refused |
| Expanded bytes over the largest image size, entries over 500,000, layers over 128 | The image is refused, naming the bound |
| Decompressed stream that does not hash to its declared digest | The image is refused |

### Store layout

All paths are under the data directory. The folder names are the field contract.

| Path under the data directory | Holds | Access |
|-------------------------------|-------|--------|
| `container/` | Everything of this feature | Root-owned, no access for group or others, so no local user reaches a set-user file of an image |
| `container/images/sha256-<64 hex digits>/rootfs/` | The applied root of one image | As the image declares |
| `container/images/sha256-<64 hex digits>/manifest.json` and `config.json` beside it | The manifest and the image configuration, byte for byte as fetched | Read-only |
| `container/images/sha256-<64 hex digits>/complete` | The completion mark: digest, platform, source registry, fetch time, expanded bytes, entry count, device nodes not created | Read-only |
| `container/layers/<container name>/sha256-<64 hex digits>/upper/` and `work/` beside it | One writable layer | Root-owned |
| `container/volumes/<container name>/<volume name>/` | One volume | Root-owned at creation. The container owns what it writes |
| `container/blobs/sha256-<64 hex digits>` | Checked blobs of the one fetch that stopped before it published. Removed when that image publishes, when a fetch of another image starts, and by the delete-everything command | Root-owned |
| `container/staging/` | Downloads not yet checked, roots being applied, folders being removed. Emptied at every start | Root-owned |

The data directory:

| Binary location | Config directory today | Data directory |
|-----------------|------------------------|----------------|
| The gokrazy appliance | `/perm/ze` | `/perm/var/lib/ze` |
| A system prefix | `/etc/ze` | `/var/lib/ze` |
| Another prefix | `<prefix>/etc/ze` | `<prefix>/var/lib/ze` |
| A relative binary directory | `etc/ze` | `var/lib/ze` |
| The override is set | - | The value of `ze.data.dir` |

### Capped storage: the writable layer and the volumes

| Topic | Writable layer | Volume |
|-------|----------------|--------|
| Key | Container name and image digest. A new digest starts from an empty layer | Container name and volume name. The digest is not part of it, so it survives an update, a rollback and a cleared layer |
| Size | The container's storage node. Default 256 MiB, from 1 MiB to 1 TiB | The volume's size node. Default 64 MiB, from 1 MiB to 1 TiB |
| Created | At the first start on that digest | At the first start of a container that declares it |
| Removed | By retention, and by the clear command, which leaves an empty one | Only by the volume delete command, refused while a running container holds it |
| A size the config lowers below what is used | The commit is accepted. Further writes fail until the usage is under the cap | The same |

Both use one mechanism:

| Topic | Design |
|-------|--------|
| Cap mechanism | ext4 project quota. The store gives each layer and each volume a project id, marks its folder so new files inherit it, and sets a hard block limit equal to the configured size |
| Enforcement | The store turns project quota enforcement on for the filesystem when it opens, through the quota system call. A write past the cap fails inside the container with "disk quota exceeded" |
| Project ids | Allocated from a counter in the state store, inside a range the store reserves. An id that already has usage or a limit on that filesystem is skipped. The id of a folder is read back from the folder, never from a second record |
| Usage | Read from the kernel's quota record, exact |
| Where the filesystem cannot enforce it | The kernel capability `container-storage-quota` is absent. Validate, start and reload refuse a config with a container, and doctor names the fix. No measured cap is used instead |
| Clearing a layer | Stops the container, moves the layer to staging, creates an empty one, starts the container again when it was running |

### Generations

| Event | Current | Previous | Layers kept | Removed |
|-------|---------|----------|-------------|---------|
| First start on digest A | A | none | A | nothing |
| Update resolves to B | B, empty layer | A | A, B | nothing |
| Update resolves to C | C, empty layer | B | B, C | The layer of A. The image A when no container holds it as current or previous and no binding names it |
| Rollback | B with its writes | C | B, C | nothing |
| Rollback again | C with its writes | B | B, C | nothing |
| Update resolves to the current digest | unchanged | unchanged | unchanged | The answer says nothing changed. No restart |
| Update resolves to the previous digest | unchanged | unchanged | unchanged | The answer says it is the retained generation and names the rollback command |
| The config names another reference that is bound and present | That digest | The old current | Both | As for an update |
| Rollback with no previous generation | unchanged | none | unchanged | Refused, saying no previous generation exists |

The generation record is written to the state store before the container restarts, so a crash between the two restarts into the new generation. Each record keeps its last 20 events. Volumes are the same folders in every generation.

### The seam to the runtime child

Both children are packages of one plugin and run in one process, so the seam is a Go boundary inside the plugin.

| Direction | Question or notice | Answer |
|-----------|--------------------|--------|
| Runtime to store | Resolve this container: its name, the image reference, the storage size and the volumes its config holds | Ready: the image digest, the image root folder to mount read-only below, the image's configuration, the writable layer's upper and work folders, and one folder for each declared volume. Or absent: the reference and the reason (never fetched, fetch running, last fetch failed and why). Never an empty value |
| Runtime to store | I hold this resolution, and later I release it | The store refuses to delete or clear what is held |
| Store to runtime | This reference is now present | The runtime starts each container that waited for it, and answers their names |
| Store to runtime | Restart this container on its new generation | The runtime stops and starts it, and answers when it runs or why it does not |
| Store to runtime | Stop this container, then start it again | Used by the clear command |

The image's configuration is handed as fetched and checked, with its bounds: entrypoint, command, environment, working directory, user as written, exposed ports, declared volumes, stop signal and labels. The store does not merge it with the operator's config, does not resolve a user name, and does not mount anything. A volume the image declares and the operator's config does not is handed as a name only: what the runtime does with it is the runtime child's rule.

### Commands and nodes this child implements

The words, the leaves and the answer shapes are fixed in `spec-container-4-operator-surface.md`, tables C-1, S-1 and S-3. This table says what each does here.

| Command or node of the operator-surface child | What this child does |
|-----------------------------------------------|----------------------|
| Fetch an image by reference | Runs the fetch under the request's context and answers once: the reference, the digest, the size, whether the image was new, and the containers it started. The command declares a bound of one hour. A caller that goes away, or the bound, stops the fetch, and the checked blobs stay for the next attempt |
| List images | One row for each image and each fetch in flight: reference, digest, size, fetch time, state (present, fetching, previous), bytes received, and the containers that hold it |
| Update a container's image | Resolves the tag again, fetches, swaps, restarts, under the same bound, and answers the old digest, the new digest, whether it changed and whether the container restarted |
| Roll a container back | Swaps current and previous, restarts |
| Delete an image by digest | Refused for an image a running container holds, naming the container in the kept rows |
| Delete every image | Removes every image no configured container holds, and the leftover blobs |
| Clear a container's writable layer | For a container by name, configured or not |
| List volumes | Container, volume, size, used, and whether it is in use or an orphan |
| Delete a volume | Refused while a running container holds it |
| The registry list | Credentials, transport, mirror |
| A container's image, storage and volume nodes | The reference, the layer's cap, the volumes and their caps |

Needs the operator-surface child does not carry yet. Each is in this child's report:

| Need | Why |
|------|-----|
| A trust anchor leaf on a registry, naming a `pki ca` entry | A registry signed by a private authority cannot be reached otherwise, since unverified TLS does not exist |
| The largest image size, one leaf for the store. Default 1 GiB, from 1 MiB to 64 GiB | The cap on the expanded bytes of one image has to be raisable by the operator |
| A failed state in the image listing, with the error and its code | A fetch whose caller went away fails with nobody reading the answer |
| The store's own figures: path, total and free bytes, the reserve, whether caps are enforced, and the used bytes of each layer | Per-consumer accounting has no command today. The image and volume listings carry sizes, the layers do not |

### Doctor checks and codes

| Problem class | Code | Reported by | Fix the finding names |
|---------------|------|-------------|-----------------------|
| The store folder is absent, not writable, or open to group or others | `doctor-container-store-unwritable` | Plugin runtime check `container-store` | On a distribution, the ownership and mode change for that folder. On the appliance, the kernel log command, because the partition did not mount read-write |
| The data filesystem is below the store's reserve | `doctor-container-store-full` | Plugin runtime check `container-store` | The delete-everything command for images |
| The filesystem cannot enforce a cap | `doctor-container-storage-quota-absent`, `doctor-container-storage-quota-unknown` | Kernel capability `container-storage-quota` | On the appliance, the install of an image built by this version. On a distribution, the export of `ze.data.dir` naming an ext4 filesystem with project quota |
| A container names an image that is not in the store | `doctor-container-image-absent`, declared in table D-1 of the operator-surface child | That child's check, from the absent answer of this child, with the last fetch failure for that reference when one exists | The fetch command with that reference |
| A layer or a volume belongs to a container or a volume the config no longer holds | `doctor-container-store-orphan`, declared in table D-1 of the operator-surface child | That child's check, from this child's layer and volume listings | The clear command or the volume delete command |
| A writable layer or a volume is at its cap | `doctor-container-storage-full` | Plugin runtime check `container-storage-usage` | The clear command for a layer, or the config statement that raises the size |
| A fetch could not reach the registry: name resolution, connection, TLS, server error, stall | `doctor-container-registry-unreachable` | Event on the report bus | The fetch command again. For an unknown authority, the config statement that sets the registry's trust anchor |
| The registry refused the credentials or the access | `doctor-container-registry-auth-refused` | Event | The config statement that sets the registry's credentials |
| The registry holds no such image or tag | `doctor-container-image-not-found` | Event | The config statement that corrects the container's image, or the fetch command with the right reference |
| A manifest, a configuration or a layer did not match its digest or size | `doctor-container-image-digest-mismatch` | Event | The fetch command again. When a mirror served it, the config statement that removes the mirror |
| An image was refused: an unsafe entry, a bound passed, a type Ze does not accept, no entry for this platform | `doctor-container-image-refused` | Event | For a bound, the config statement that raises it. Otherwise the config statement that names another image |
| A fetch was refused for lack of space | `doctor-container-fetch-no-space` | Event | The delete-everything command for images |

### State keys

| Key pattern | Value | Written when |
|-------------|-------|--------------|
| `meta/container/binding/{reference-digest}`, the last segment being the SHA-256 of the reference | The reference as written, and the image digest it is bound to | A fetch or an update completes. Removed when its image is deleted |
| `meta/container/generation/{container}` | Current digest, previous digest, and the last 20 events with their time | A start on a new digest, an update, a rollback |
| `meta/container/project-id` | The counter the next project id comes from | A layer or a volume is created |

What an image is, how large it is and when it was fetched is in its folder's completion mark, not in a key, so a lost state store loses bindings and generations only. A fetch then finds the folder present and binds it again without a download.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Engine to plugin | Registered commands, the config transaction, the doctor callback. Works with the plugin in its own process | No |
| Plugin to state store | The SDK state calls, under granted keys | No |
| Plugin to registry | The registry pull protocol over HTTPS | No |
| Plugin to filesystem | Descriptor-relative calls under the store root | No |
| Plugin to kernel | The quota system call, and the project attribute of a folder | No |
| Store to runtime | The seam above, inside one process | No |
| Build host to device | The format of the persistent partition | No |

### Changed-Wire Census

**Search evidence:** searched the shipped Go trees for a registry reader (terms: manifests, blobs, the authentication challenge header, Bearer) and found none outside the vendored tree. The positive control is `internal/core/fetch/fetch.go` `doToFile`, which the same search finds as an HTTP reader. Searched `pkg/zefs/keys.go` for a container key and found none. Searched every config schema for a container root or a registry list and found none, with `internal/component/pki/yang/ze-pki-conf.yang` `list ca` as the control. Searched the appliance builder, the installer and the installer's init for a formatter of the persistent partition (terms: mkfs, ext4) and found one, `injectZeFS`. Its readers were found by following the partition: `verifyInject`, `mountInjectDB`, and gokrazy's mount of it. Searched the same trees and the gokrazy module cache for a resize of that partition (terms: resize, grow, the partition ioctl) and found none in shipped code.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Registry pull protocol | writer | A container registry, outside the repository | No code in the tree speaks it | New reader only. The registry's behavior is fixed by its published specification, and by the source of the registry the interop test runs |
| Registry pull protocol | reader | The registry client package of the container plugin, new | No existing reader | New |
| Persistent partition format | writer | `internal/appliance/cmd_build.go` `injectZeFS` | The one formatter. It asks for no quota feature today | Changed: two features added. A device formatted before keeps its format, and the capability probe refuses the container feature there |
| Persistent partition format | reader | gokrazy init's mount of the persistent partition | Mounts ext4 with no option | Preserved. Needs a kernel with quota support, which this spec pins. Proven by the `container-store` QEMU action |
| Persistent partition format | reader | `internal/install/disk/system.go` `mountInjectDB` | Mounts partition 4 read-write in the installer | Preserved. Needs the installer's kernel to have quota support. Proven by `./le test qemu installer` |
| Persistent partition format | reader | `internal/appliance/diskverify.go` `verifyInject`, `verifyE2fsck`, `tryLoopbackVerify` | Reads the image back on the build host | Preserved. Proven by `./le test functional appliance` |
| Persistent partition format | reader | The store's quota probe, new | No existing reader | New |
| Store layout on the data directory | writer | The image store package of the container plugin, new | No existing writer: no data directory exists | New. Permanent once shipped |
| Store layout on the data directory | reader | The same package, and the runtime child through the seam | No existing reader | New |
| Container state keys | writer | The container plugin through `StatePut` | No container key in `pkg/zefs/keys.go` | New. Permanent once shipped |
| Container state keys | reader | The container plugin through `StateGet` and `StateList` | No existing reader | New |
| Registry, storage and volume config nodes | writer | The operator's config file and the editor | No such node in any schema | New, owned by `spec-container-4-operator-surface.md` |
| Registry, storage and volume config nodes | reader | The container plugin's config callbacks | No existing reader | New |
| Trust anchor reference | writer | `pki ca <name>` in the operator's config | `internal/component/pki/yang/ze-pki-conf.yang` `list ca` | Unchanged |
| Trust anchor reference | reader | The container plugin, as a declared config read | Existing readers are the IPsec peers and the managed client, unchanged | New reader |

### Integration Points
- The core fetch package: the stall reader is exported and `ValidSHA256` is called. Nothing else changes.
- The core paths package: the data directory resolver.
- `pkg/zefs/keys.go` and `statestore.RegisterPluginKeys`: three key patterns, granted to the container plugin.
- `kernelcap.MustRegister`: the storage quota capability.
- The plugin's Stage 1 registration, created by the runtime child: this child adds its commands with their bounds, its doctor check declarations, its declared read of `pki`, and its event codes.
- `internal/appliance/cmd_build.go` `injectZeFS`: the format.
- `gokrazy/kernel/runtime.config` and `gokrazy/kernel/runtime.require`: quota support.
- `fileAllowlist`: the new files that write.
- `integrationPackages` and the QEMU action table: the integration tests and the appliance proof.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | On an ext4 filesystem formatted with the quota and project features and mounted with no option, Ze can turn project quota enforcement on and set a hard limit through the quota system call | Knowledge of the kernel's quota interface. The kernel source was not read | The store remounts the data filesystem with the project quota option, or the cap needs another design | `TestQuotaRefusesWritePastCap` in the QEMU guest on the runtime kernel, and reading the kernel's ext4 quota code for the linux-limitations entry | unvalidated |
| A-2 | The runtime kernel on amd64 and arm64, and the kernel the installer boots, have quota support, and ext4 refuses a read-write mount of a quota filesystem on a kernel without it | The amd64 resolved config holds quota support from upstream defaults (research report). No arm64 config was read. The installer's kernel profile was not found | A new image's persistent partition does not mount on that kernel, and the device has no config | The two kernel files request and require the options, so a build without them fails. `./le test qemu installer` and the `container-store` action boot the result | unvalidated |
| A-3 | The format tool on a Linux and a macOS build host accepts the two features, and the debugfs injection and the structural check still pass | Not run. The seed is written with debugfs, which may leave the quota usage records behind what the files hold, and `verifyE2fsck` fails the build on any inconsistency | The two features are added after the injection, by the e2fsprogs tool that enables them on an unmounted filesystem and computes the usage then. `ze appliance build` then resolves a third e2fsprogs tool | `TestInjectFormatsPermWithProjectQuota` and `./le test functional appliance` | unvalidated |
| A-4 | Umbrella A-6: the persistent partition is the size the image was built with, about 950 MB on the default 2 GiB image, and nothing grows it | Read: `internal/install/disk/run.go` `runHTTP`, `runISO`; `internal/install/disk/system.go` `mountInjectDB`; gokrazy init's mount; `internal/appliance/config.go`. A search for a resize found none | Nothing in this design depends on growth: every fetch checks the space that exists | The reads above | confirmed |
| A-5 | The path-safety library the runtime child brings builds with cgo off and gives descriptor-based in-root open and directory creation | The upstream runtime's `go.mod` at v1.5.3 requires it, and its vendored tree holds a descriptor-based package | The store resolves through the standard library's rooted directory handle, with the device, attribute and link calls made by descriptor through the system-call package | A type-check after the runtime child's dependency commit, and `FuzzApplyLayerNeverEscapesRoot` | unvalidated |
| A-6 | The registry protocol works as this spec describes it: the challenge, the token answer, the four manifest types, the blob redirect | Written from memory of the published specification. No text of it is in the tree | A request or a check is wrong against a real registry | `TestRegistryInteropDistribution` against a real registry whose source is read first, and `TestRegistryLivePublicPull` | unvalidated |
| A-7 | A plugin command may declare a bound of one hour, and a caller that goes away ends the command in the plugin, internal and forked | `internal/component/plugin/server/command_registry.go` `CommandDef.Timeout` holds a per-command bound. How a caller's departure reaches a plugin command was not read | The fetch keeps running after its caller left, until the bound. The store then needs a way to stop it | Reading the plugin command dispatch, and `image-fetch-caller-leaves` | unvalidated |
| A-8 | The zstd decoder may be used without a new module | `go.mod` lists its module, and the vendored tree holds its zstd package | A dependency commit is owed first | The read | confirmed |
| A-9 | The appliance's var folder exists at boot and is on the persistent partition | gokrazy init creates it after the mount | The data directory needs another parent | The `container-store` QEMU action | unvalidated |
| A-10 | A plugin registers its doctor codes and its kernel capability codes from its own package | Plugin packages call the code registry today (research report). The capability's own comment says its codes are registered in the core code table | The codes need a home that a build without the gate also compiles | Reading the capability's validation at implementation | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A crafted layer writes outside the staging root, through a parent-directory path, a planted symbolic link or a hard link (umbrella R-6) | A file outside the root changes in a test | Every entry is resolved under a descriptor for the root, a name is removed before it is created, and a hard link target is checked. One test per class and a fuzz test |
| R-2 | A layer expands without bound, in bytes or in entries (umbrella R-6) | A fetch fills the filesystem | Bounds on expanded bytes, entries, layers and the decoder's window, counted on the decompressed stream, plus the free-space rule |
| R-3 | An unpacked image puts set-user files and device nodes on the host filesystem, where a local user could reach them | A user other than root lists the image folder | The store root gives no access to group or others and is checked at every start. A device node is never created |
| R-4 | A parser defect in the archive reader or a decompressor is reached by bytes nobody vouched for | A crash in the fetch worker on a registry answer | No blob is parsed before its digest and size were checked |
| R-5 | The registry's credentials reach another host: a redirect target, a mirror, a log line | A test registry on a second host sees an authorization header | The header goes only to the exact host and port it was obtained for, a mirror gets none, and no error or log formats a credential |
| R-6 | The new partition format does not mount on a kernel without quota support, and the device loses its config directory | The appliance boots to the unprovisioned state after an install | The options are pinned in both kernel files before the format changes, and the installer and appliance QEMU proofs gate the change |
| R-7 | The persistent partition fills and Ze can no longer commit its config (umbrella R-14) | The config-partition doctor check warns after a fetch | The reserve, the hard cap on every layer and volume, the retention rule, and the refusal of a fetch before any download |
| R-8 | File capabilities of an image are lost (umbrella R-8) | A program in a container fails for a missing capability | The folder keeps the attribute. A test reads it back, and the QEMU action reads it from a running container |
| R-9 | A device installed before this work cannot run containers at all | The capability reads absent on that device | Stated plainly by doctor with the fix. It is a consequence of exact enforcement and is in the report to the owner |
| R-10 | A distribution's data filesystem is not ext4 with project quota | The capability reads absent | Doctor names the requirement and the override. No other filesystem is supported by this spec |
| R-11 | A layer or a volume left by a container removed from the config holds space for ever | The listings grow | Doctor reports each as an orphan with the command that removes it |
| R-12 | A project id the store picks is already used by something else on a distribution's filesystem | A limit appears on a folder Ze does not own | An id with existing usage or a limit is skipped at allocation |
| R-13 | A fetch is in flight when the daemon stops, or its caller leaves | A staging folder remains | The fetch stops with its context, staging is emptied at every start, and only blobs that were checked stay |
| R-14 | Each image stores every layer again, because nothing is shared between folders | Two images with one base use twice the space | Accepted with the owner's ruling for one folder per digest. The image listing shows each image's bytes |
| R-15 | The sum of the configured caps is larger than the filesystem | A write fails for lack of space before any cap is reached | The caps are limits, not reservations. The reserve protects Ze's config, and the store's doctor check reports the filesystem below it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A defect in layer application lets a registry change a host file as root. A defect in the cap or the reserve lets a container fill the persistent partition, and Ze then cannot commit its config. A format change without the kernel pin leaves a new device with no mountable persistent partition. Routing is not touched by any of these directly |
| How is it reverted? | The code reverts by commit. Images, layers and volumes on a device stay as inert files under the data directory. A device built meanwhile keeps the two filesystem features, which are harmless on a kernel with quota support, so the kernel pin is not reverted with it |
| Who else touches this path? | `spec-container-1-runtime.md` (the same plugin folder and its registration), `spec-container-4-operator-surface.md` (the schema, the command words, two doctor rows), any session editing the appliance builder or the kernel files, and `TestDoctorDependencyInventory` |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| The image fetch command, from the daemon | → | The fetch, the registry client, layer application, publication | `test/container/image-fetch-lists-digest.ci` |
| The image fetch command, with the plugin in its own process | → | The same, over the plugin connection, with state through the SDK | `test/container/image-fetch-external-plugin.ci` |
| The image listing during a fetch | → | The in-flight row with the bytes received | `TestListingShowsFetchingWithBytesReceived` |
| A config with a container whose image is absent | → | The store's resolve answer, the doctor check, the notice to the runtime | `test/container/image-absent-doctor-then-start.ci` |
| The runtime's resolve request | → | The store's answer with the folders and the image's configuration | `TestResolveReadyHandsFoldersAndConfiguration` |
| The update and rollback commands | → | The generation record, the fresh layer, the restart notice | `test/container/image-update-rollback.ci` |
| The delete command | → | The hold check and the folder removal | `test/container/image-delete-in-use-refused.ci` |
| The clear command | → | The layer replacement | `test/container/layer-clear.ci` |
| A config with a volume | → | The volume folder and its cap | `test/container/volume-survives-update.ci` |
| `ze config validate` on a host that cannot enforce a cap | → | The kernel capability refusal | `test/container/store-quota-absent-refused.ci` |
| `ze doctor` with a full layer | → | The plugin's doctor callback | `TestStorageFullCheck` |
| `ze appliance build` | → | `injectZeFS` with the two features | `TestInjectFormatsPermWithProjectQuota` |
| A booted appliance | → | The store on the persistent partition, the quota, the reboot | `./le test qemu container-store` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | The operator runs the fetch command for a reference the registry serves | The command answers the reference, the digest of its manifest, its size and that the image is new. The image listing shows it as present under that digest, and the image folder named by that digest holds the root, the manifest, the configuration and the completion mark | An image already in the store under that digest is bound and not downloaded again, and the answer says it was not new, because the digest proves the folder holds the same content |
| AC-2 | The registry serves a manifest, a configuration or a layer whose bytes do not hash to its digest, or whose size differs | The command fails. Its error names the object, the digest expected and the digest received, under `doctor-container-image-digest-mismatch`. The image listing shows exactly the images it showed before, and staging is empty | Every image already in the store stays, because a failed fetch changes only its own staging folder |
| AC-3 | The registry demands a token, the config holds its credentials, and it redirects a blob to another host | The fetch succeeds. The realm received the credentials, the registry received the token, and the other host received a request with no authorization header | A redirect to the same host and port keeps the header, because it is the server the token was issued for |
| AC-4 | The reference names an index with entries for several platforms | The store holds the image whose platform is the running one, and its completion mark names that platform | An index with no entry for the running platform fails the fetch, naming the platforms it offers, because no other platform's image can run |
| AC-5 | The same file tree is served as an uncompressed, a gzip and a zstd layer | The three fetches produce the same tree: the same names, types, owners, modes, times, attributes and contents | A layer of any other type fails the fetch naming the type, because Ze does not guess a format |
| AC-6 | An image whose layers hold a whiteout, an opaque directory, files of several owners and modes, a set-user file, a hard link, a symbolic link, a named pipe and a file capability | The root holds the result the layers describe: the whited-out name is gone, the opaque directory holds only what its own layer added, and each owner, mode, time, link and capability reads back as written | A device node under the image's device folder is not created and is counted in the image's record, because the runtime mounts its own device folder over it |
| AC-7 | An image with one hostile entry: a path that leaves the root, a write through a planted symbolic link, a hard link to a file outside the root, a device node outside the device folder, an overlay control attribute, or more bytes or entries than the bound | The fetch fails under `doctor-container-image-refused`, naming the entry and the rule. A marker file placed beside the store root is unchanged, and the image listing is unchanged | An entry whose parent is a symbolic link pointing inside the root is applied at the place it resolves to, because a process in the container would see it there |
| AC-8 | The caller of a fetch goes away, or the daemon is killed, while a layer is being applied | The image listing does not show the image as present. At the next start staging is empty. A new fetch of the same reference succeeds and downloads only the blobs that were not checked before | A fetch that had already published its folder stays in the listing, because publication is one rename |
| AC-9 | A config names a container whose image is not in the store, and is committed | The commit succeeds. The store answers the runtime that the image is absent, with the reference, and doctor reports `doctor-container-image-absent` with the fetch command for that reference as its fix. When that fetch succeeds, the container starts with no further command, and the fetch's answer names it | Other containers of the same commit start, because one absent image does not hold the rest |
| AC-10 | A container writes a file in its root. The container is restarted, then the daemon is restarted, then the device reboots | The file is there with its content after each of the three | A file under a memory-backed mount the image or the config declares is lost, because that is what such a mount is |
| AC-11 | The update command runs for a container whose tag now resolves to a new digest, then the rollback command runs | After the update the container runs the new digest from an empty writable layer, and the image listing shows the old digest as previous. After the rollback it runs the old digest and the file it wrote before the update is there | The image and the layer of a generation older than the previous one are removed, by the retention rule, unless another container or a binding still names the image |
| AC-12 | The update command runs and the tag resolves to the digest the container already runs | The answer says nothing changed. The container keeps running with no restart, and its generation record is unchanged | A tag that resolves to the retained previous digest also changes nothing and names the rollback command, because that generation already has its own layer |
| AC-13 | The delete command names an image a running container holds, then an image nothing holds | The first is kept, and the answer names the container as the reason. The second is deleted, and the answer gives its digest and size | Deleting the previous generation of a container is allowed and the answer says rollback is no longer possible, because only the running image is protected |
| AC-14 | The clear command runs for a container that wrote a file in its root and a file in a volume | The container runs again on an empty layer, and the file in its root is gone. The file in the volume is there | The image folder is unchanged, because the command touches the layer only |
| AC-15 | A container writes past its storage size, and another writes past the size of a volume | Each write fails inside the container with "disk quota exceeded". Doctor reports `doctor-container-storage-full` for each, with the fix | Other containers keep writing inside their own caps and Ze commits a config, because each folder has its own limit and the reserve is untouched |
| AC-16 | A config with a container is validated, started or reloaded where the data filesystem cannot enforce a project quota | Each of the three refuses, naming the filesystem requirement, and doctor reports `doctor-container-storage-quota-absent` with the fix for that platform | A config with no container is accepted there, because nothing needs the cap |
| AC-17 | A fetch whose layers need more than the free space above the reserve | The command is refused before any blob is downloaded, under `doctor-container-fetch-no-space`, naming the bytes needed, the bytes free and the reserve | A fetch that fits proceeds, and the reserve stays free for Ze's config |
| AC-18 | A registry is configured with a password, with a trust anchor, or with the HTTP transport | The config shows the password obfuscated. A registry whose certificate chains to the named anchor is fetched from. A registry with the HTTP transport is fetched from over plain HTTP | Without the anchor, a registry signed by a private authority fails under `doctor-container-registry-unreachable` naming the trust anchor statement. With the HTTPS transport, a server that answers plain HTTP is refused. Both because the default is verified TLS |
| AC-19 | A registry has a mirror | The mirror is asked first and receives no registry credentials. When it fails, the registry itself is asked and the fetch succeeds | Content from a mirror is accepted only when it matches its digest, because the digest, not the mirror, vouches for it |
| AC-20 | A fetch is running and another session asks for the image listing, twice | Each answer shows the reference as fetching, and the bytes received are larger in the second. When the fetch ends the row reads present | A row for a fetch that failed leaves the listing, and its error reaches the caller and the report bus, because the listing shows what the store holds |
| AC-21 | Ze starts on an appliance, under a system prefix, under another prefix, and with the override set | The store is under the data directory of that row of the data directory table, with a root folder that gives no access to group or others | A store root that is open to group or others is not used and is reported under `doctor-container-store-unwritable`, because an image holds set-user files |
| AC-22 | An appliance image is built and booted | The storage quota capability reads present, and a write past a cap is refused by the kernel | A device installed from an image built before this work reads absent and keeps every other function, because its partition was formatted without the features |
| AC-23 | The image listing and the volume listing run with two images, and two volumes of which one belongs to a container the config no longer holds | Each image row carries its size and the containers that hold it. Each volume row carries its size and its used bytes, and the second reads orphan | A volume's used bytes are the kernel's quota figure and an image's size is the count taken while it was applied, because each is the exact figure Ze holds |
| AC-24 | A container with a volume writes a file in it. Its image is updated, then rolled back, and the volume delete command runs while it is running, then after it is stopped | The file is there after the update and after the rollback. The first delete is refused, naming the container. The second removes the folder and answers its size | A volume the config no longer declares stays on disk until it is deleted, because only the operator removes stored data |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Fetches an image and sees it listed | Command, fetch worker, registry client, layer application, publication, binding, listing | `test/container/image-fetch-lists-digest.ci` |
| 2 | Commits a container before its image exists, reads doctor, fetches, and the container starts | Config transaction, resolve answers absent, doctor, fetch, notice to the runtime, start | `test/container/image-absent-doctor-then-start.ci` |
| 3 | Updates a container, finds the new image wrong, rolls back and has the old files | Update, generation record, restart, rollback, layer of the old digest | `test/container/image-update-rollback.ci` |
| 4 | Watches a long fetch from a second session, and stops it by leaving | Image listing row, request context, checked blobs kept | `test/container/image-fetch-caller-leaves.ci` |
| 5 | Reboots the appliance and finds the container's files | Store on the persistent partition, layer keyed by name and digest | `./le test qemu container-store` |
| 6 | Frees space | Delete of unused images, clear of a layer, delete of an orphan volume | `test/container/image-delete-in-use-refused.ci`, `test/container/layer-clear.ci`, `test/container/volume-delete.ci` |
| 7 | Fetches from a private registry with a private authority | Registry config, credentials, trust anchor, token exchange | `TestTrustAnchorReplacesSystemRoots`, `TestPullBearerTokenExchangeSendsCredentialsToRealmOnly` |
| 8 | Keeps data across an image update | Volume folder keyed without the digest | `test/container/volume-survives-update.ci` |

## 🧪 TDD Test Plan

A local registry for tests comes from one new package, `internal/test/ociregistry`: it serves the pull endpoints from an image set built in memory, with scripted faults (a wrong digest, a stall, a challenge, a redirect to a second listener). The unit tests call it directly, and the `.ci` tests reach it through a compiled fixture, so no test holds a second copy. Layers in the default image set are owned by the user that runs the test, so application needs no privilege. The tests that need another owner, a file capability or a quota carry the integration tag and run in the QEMU guest.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseReferenceForms` | `internal/plugins/container/registry/reference_test.go` | A host, a path and a tag, and a host, a path and a digest, each give their registry, repository and pin | |
| `TestParseReferenceRefusesPartial` | `internal/plugins/container/registry/reference_test.go` | A reference with no host, or with neither tag nor digest, is refused, naming what is missing | |
| `TestPullAnonymousManifestAndBlobs` | `internal/plugins/container/registry/pull_test.go` | A registry with no challenge serves a manifest and its blobs | |
| `TestPullBearerTokenExchangeSendsCredentialsToRealmOnly` | `internal/plugins/container/registry/auth_test.go` | The credentials reach the realm and only the realm | |
| `TestPullBasicChallenge` | `internal/plugins/container/registry/auth_test.go` | A basic challenge is answered with the credentials to the registry itself | |
| `TestTokenRealmOverPlainHTTPRefused` | `internal/plugins/container/registry/auth_test.go` | A challenge that names a plain HTTP realm is refused for a registry whose transport is HTTPS | |
| `TestRedirectToOtherHostDropsAuthorization` | `internal/plugins/container/registry/redirect_test.go` | A second listener sees no authorization header | |
| `TestRedirectSameHostKeepsAuthorization` | `internal/plugins/container/registry/redirect_test.go` | The same host and port sees the header | |
| `TestRedirectDowngradeRefused` | `internal/plugins/container/registry/redirect_test.go` | A redirect from HTTPS to HTTP is refused | |
| `TestRedirectLimit` | `internal/plugins/container/registry/redirect_test.go` | Ten redirects are followed and the eleventh is refused | |
| `TestManifestDigestMismatchRefused` | `internal/plugins/container/registry/verify_test.go` | A manifest whose bytes hash to another digest is refused, naming both | |
| `TestBlobDigestMismatchRefused` | `internal/plugins/container/registry/verify_test.go` | A blob whose bytes hash to another digest is refused before it is handed on | |
| `TestBlobSizeMismatchRefused` | `internal/plugins/container/registry/verify_test.go` | A blob shorter or longer than its declared size is refused | |
| `TestDigestMismatchNotRetried` | `internal/plugins/container/registry/verify_test.go` | One request is made for a blob with a wrong digest | |
| `TestRetryOnServerErrorThenSuccess` | `internal/plugins/container/registry/pull_test.go` | A server error is retried and the second answer is used | |
| `TestManifestSizeBound` | `internal/plugins/container/registry/verify_test.go` | A manifest of 4 MiB is read and one byte more is refused | |
| `TestNonSHA256DigestRefused` | `internal/plugins/container/registry/verify_test.go` | A descriptor with another algorithm is refused, naming it | |
| `TestIndexSelectsRunningPlatform` | `internal/plugins/container/registry/platform_test.go` | amd64, arm64 and arm64 variant v8 each select their entry | |
| `TestIndexWithoutPlatformNamesOffered` | `internal/plugins/container/registry/platform_test.go` | The refusal lists the platforms the index holds | |
| `TestUnacceptedManifestTypesRefused` | `internal/plugins/container/registry/platform_test.go` | Schema 1, a nested index, a foreign layer, an encrypted layer and an artifact manifest are each refused by name | |
| `TestBlobStallTimesOut` | `internal/plugins/container/registry/pull_test.go` | A transfer that stops sending ends with the stall error | |
| `TestFetchCancelStopsTransfer` | `internal/plugins/container/registry/pull_test.go` | Cancelling the context ends a transfer in flight | |
| `TestMirrorTriedFirstThenRegistry` | `internal/plugins/container/registry/mirror_test.go` | The mirror is asked first, and the registry after the mirror fails | |
| `TestMirrorGetsNoRegistryCredentials` | `internal/plugins/container/registry/mirror_test.go` | The mirror sees no authorization header | |
| `TestHTTPTransportFetchesOverPlainHTTP` | `internal/plugins/container/registry/tls_test.go` | A registry with the HTTP transport is fetched from | |
| `TestHTTPSTransportRefusesPlainHTTP` | `internal/plugins/container/registry/tls_test.go` | The same server under the HTTPS transport is refused | |
| `TestTrustAnchorReplacesSystemRoots` | `internal/plugins/container/registry/tls_test.go` | A registry signed by the named anchor is trusted, and one signed by another is not | |
| `TestCredentialsNeverInErrorOrLog` | `internal/plugins/container/registry/secret_test.go` | No error text and no log line holds the password or the token | |
| `TestStallReaderExported` | `internal/core/fetch/fetch_test.go` | The exported stall reader ends a read after its timeout | |
| `TestApplyRegularTreeKeepsOwnerModeTime` | `internal/plugins/container/imagestore/apply_test.go` | Files, directories and links read back as written | |
| `TestApplyWhiteoutRemovesLowerEntry` | `internal/plugins/container/imagestore/apply_whiteout_test.go` | A whiteout removes a name of an earlier layer and its subtree | |
| `TestApplyOpaqueDirectoryKeepsSameLayerEntries` | `internal/plugins/container/imagestore/apply_whiteout_test.go` | Earlier children are gone and children of the same layer stay, in either archive order | |
| `TestApplyHardLinkInsideRoot` | `internal/plugins/container/imagestore/apply_link_test.go` | Two names share one inode | |
| `TestApplyHardLinkOutsideRootRefused` | `internal/plugins/container/imagestore/apply_link_test.go` | A hard link to a file outside the root refuses the image | |
| `TestApplySymlinkParentResolvedInsideRoot` | `internal/plugins/container/imagestore/apply_link_test.go` | An entry under a linked directory lands inside the root, and a link that points outside never leads a write outside | |
| `TestApplyParentDirectoryPathRefused` | `internal/plugins/container/imagestore/apply_refuse_test.go` | A path with a parent-directory component refuses the image | |
| `TestApplyReplacesEntryNeverWritesThrough` | `internal/plugins/container/imagestore/apply_refuse_test.go` | A file whose name held a symbolic link replaces the link, and the link's target is unchanged | |
| `TestApplyDeviceNodeUnderDevCounted` | `internal/plugins/container/imagestore/apply_refuse_test.go` | The node is not created and the record counts one | |
| `TestApplyDeviceNodeElsewhereRefused` | `internal/plugins/container/imagestore/apply_refuse_test.go` | The image is refused, naming the entry | |
| `TestApplyOverlayControlAttributeRefused` | `internal/plugins/container/imagestore/apply_xattr_test.go` | An overlay control attribute refuses the image | |
| `TestApplyXattrFileCapabilityKept` | `internal/plugins/container/imagestore/apply_xattr_linux_test.go` | A file capability reads back from the applied file (integration tag) | |
| `TestApplyOtherOwnersKept` | `internal/plugins/container/imagestore/apply_owner_linux_test.go` | Entries of several numeric owners read back as written (integration tag) | |
| `TestApplyCompressionsProduceSameTree` | `internal/plugins/container/imagestore/apply_test.go` | Uncompressed, gzip and zstd give one tree | |
| `TestApplyExpandedSizeBound` | `internal/plugins/container/imagestore/apply_bound_test.go` | An image at the bound is applied and one byte more is refused | |
| `TestApplyEntryCountBound` | `internal/plugins/container/imagestore/apply_bound_test.go` | An image at the bound is applied and one entry more is refused | |
| `TestApplyDiffDigestMismatchRefused` | `internal/plugins/container/imagestore/apply_test.go` | A decompressed stream with another digest refuses the image | |
| `FuzzApplyLayerNeverEscapesRoot` | `internal/plugins/container/imagestore/apply_fuzz_test.go` | No archive changes a marker file beside the root | |
| `TestPublishIsOneRenameAndRefusesExisting` | `internal/plugins/container/imagestore/publish_test.go` | A second publication of one digest does not replace the first | |
| `TestListIgnoresFolderWithoutMark` | `internal/plugins/container/imagestore/publish_test.go` | A folder with no completion mark is not an image | |
| `TestStartEmptiesStagingAndKeepsCheckedBlobs` | `internal/plugins/container/imagestore/publish_test.go` | Staging is empty after the store opens, and a blob named by its digest is still there | |
| `TestFetchOfPresentDigestDownloadsNothing` | `internal/plugins/container/imagestore/fetch_test.go` | No blob request is made for a digest in the store | |
| `TestFetchReusesCheckedBlob` | `internal/plugins/container/imagestore/fetch_test.go` | A second attempt requests only the blobs that were not checked before | |
| `TestFetchOfAnotherImageRemovesLeftoverBlobs` | `internal/plugins/container/imagestore/fetch_test.go` | Blobs of a stopped fetch are gone when a fetch of another image starts | |
| `TestFreeSpaceRefusalBeforeAnyBlob` | `internal/plugins/container/imagestore/space_test.go` | A fetch that would cross the reserve makes no blob request | |
| `TestStoreRootOpenToOthersRefused` | `internal/plugins/container/imagestore/layout_test.go` | A root folder readable by others is not used | |
| `TestDeleteRefusedWhileHeld` | `internal/plugins/container/imagestore/delete_test.go` | A held image is not deleted and the holder is named | |
| `TestDeleteUnusedRemovesFolder` | `internal/plugins/container/imagestore/delete_test.go` | The folder and its bindings are gone | |
| `TestGenerationUpdateKeepsOnePrevious` | `internal/plugins/container/imagestore/generation_test.go` | Three updates leave the last two generations | |
| `TestRollbackSwapsGenerations` | `internal/plugins/container/imagestore/generation_test.go` | Two rollbacks return to the start | |
| `TestRollbackWithoutPreviousRefused` | `internal/plugins/container/imagestore/generation_test.go` | The refusal says no previous generation exists | |
| `TestUpdateToRetainedPreviousNamesRollback` | `internal/plugins/container/imagestore/generation_test.go` | Nothing changes and the answer names the rollback | |
| `TestRetentionKeepsImageAnotherContainerHolds` | `internal/plugins/container/imagestore/generation_test.go` | An image dropped by one container stays while another holds it | |
| `TestGenerationRecordWrittenBeforeRestart` | `internal/plugins/container/imagestore/generation_test.go` | The record holds the new digest when the restart notice is sent | |
| `TestLayerKeyedByNameAndDigest` | `internal/plugins/container/imagestore/layer_test.go` | Two digests of one container have two layers | |
| `TestClearLayerStartsEmpty` | `internal/plugins/container/imagestore/layer_test.go` | The layer is empty after the clear | |
| `TestVolumeKeyedWithoutDigest` | `internal/plugins/container/imagestore/volume_test.go` | Two digests of one container resolve to one volume folder | |
| `TestVolumeDeleteRefusedWhileHeld` | `internal/plugins/container/imagestore/volume_test.go` | A held volume is not deleted and the holder is named | |
| `TestOrphanLayerAndVolumeListed` | `internal/plugins/container/imagestore/volume_test.go` | A layer and a volume of a container the config no longer holds read orphan | |
| `TestResolveAbsentNamesReference` | `internal/plugins/container/imagestore/resolve_test.go` | The absent answer holds the reference and the reason | |
| `TestResolveReadyHandsFoldersAndConfiguration` | `internal/plugins/container/imagestore/resolve_test.go` | The ready answer holds the layer folders, a folder for each volume, and each field of the image's configuration | |
| `TestImageConfigurationOverBoundRefused` | `internal/plugins/container/imagestore/resolve_test.go` | A configuration over 4 MiB refuses the image | |
| `TestQuotaProbeStates` | `internal/plugins/container/imagestore/quota_test.go` | Present, absent and cannot-determine each give their capability state | |
| `TestQuotaRefusesWritePastCap` | `internal/plugins/container/imagestore/quota_linux_test.go` | A write past the cap fails with the quota error on the runtime kernel (integration tag) | |
| `TestQuotaAbsentWithoutProjectFeature` | `internal/plugins/container/imagestore/quota_linux_test.go` | A filesystem without the feature reads absent (integration tag) | |
| `TestProjectIDSkipsUsedID` | `internal/plugins/container/imagestore/quota_test.go` | An id with usage is not handed out | |
| `TestDataDirFromBinary` | `internal/core/paths/paths_test.go` | Each row of the data directory table | |
| `TestDefaultDataDirHonorsOverride` | `internal/core/paths/paths_test.go` | The override wins | |
| `TestInjectFormatsPermWithProjectQuota` | `internal/appliance/cmd_build_test.go` | The build asks for the two features | |
| `TestFetchCommandAnswersResultAtEnd` | `internal/plugins/container/image_cmd_test.go` | The answer holds the reference, the digest, the size, whether it was new and the containers started | |
| `TestFetchCommandDeclaresItsBound` | `internal/plugins/container/image_cmd_test.go` | The fetch and update commands register with a one-hour bound | |
| `TestListingShowsFetchingWithBytesReceived` | `internal/plugins/container/image_cmd_test.go` | A held fetch reads fetching, with the bytes received | |
| `TestFetchQueueBound` | `internal/plugins/container/image_cmd_test.go` | Eight fetches wait and the ninth is refused | |
| `TestStoreUnwritableCheck` | `internal/plugins/container/doctor_store_test.go` | The check reports the folder and the fix for the platform | |
| `TestStoreBelowReserveCheck` | `internal/plugins/container/doctor_store_test.go` | The check fires below the reserve and not at it | |
| `TestStorageFullCheck` | `internal/plugins/container/doctor_store_test.go` | A layer and a volume at their cap are each reported with their fix | |
| `TestEveryStoreCodeIsDescribed` | `internal/plugins/container/doctor_store_test.go` | Each code this child owns in the doctor table is registered with a description | |
| `TestBindingKeyUsesReferenceDigest` | `internal/plugins/container/imagestore/state_test.go` | A reference with slashes and a colon gives one key segment | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| Storage size and volume size | Owned by the operator-surface child: 1 to 1048576 mebibytes | Its table | Its table | Its table |
| Largest image size | 1 MiB to 64 GiB | 64 GiB | 1 MiB minus one byte | 64 GiB plus one byte |
| Expanded bytes of one image | 0 to the largest image size | The largest image size | N/A | One byte more |
| Entries of one image | 0 to 500,000 | 500,000 | N/A | 500,001 |
| Layers of one image | 1 to 128 | 128 | 0 | 129 |
| Manifest, index and configuration size | 1 byte to 4 MiB | 4 MiB | 0 | 4 MiB plus one byte |
| Token answer size | 1 byte to 64 KiB | 64 KiB | 0 | 64 KiB plus one byte |
| Redirects followed | 0 to 10 | 10 | N/A | 11 |
| Entry name length | 1 to 255 bytes | 255 | 0 | 256 |
| Entry path length | 1 to 4096 bytes | 4096 | 0 | 4097 |
| Attributes of one entry | 0 to 64 KiB | 64 KiB | N/A | 64 KiB plus one byte |
| Decoder window of a zstd layer | Up to 32 MiB | 32 MiB | N/A | 32 MiB plus one byte |
| Waiting fetches | 0 to 8 | 8 | N/A | 9 |
| Free space after a fetch | The reserve or more: the larger of 64 MiB and 5% of the filesystem | Exactly the reserve | One byte under the reserve | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `image-fetch-lists-digest` | `test/container/image-fetch-lists-digest.ci` | The operator fetches an image from the local registry and the listing shows its digest | |
| `image-fetch-external-plugin` | `test/container/image-fetch-external-plugin.ci` | The same fetch with the plugin run as a separate process | |
| `image-fetch-digest-mismatch` | `test/container/image-fetch-digest-mismatch.ci` | The registry serves a wrong layer, the command fails with the code, and the listing is empty | |
| `image-fetch-caller-leaves` | `test/container/image-fetch-caller-leaves.ci` | A second session sees the fetch in flight. The first caller leaves, the row goes, and a new fetch succeeds | |
| `image-fetch-interrupted-then-clean` | `test/container/image-fetch-interrupted-then-clean.ci` | The daemon is stopped during a fetch, started again, and a new fetch succeeds | |
| `image-absent-doctor-then-start` | `test/container/image-absent-doctor-then-start.ci` | A container waits for its image, doctor names the fetch, and the fetch starts it | |
| `image-update-rollback` | `test/container/image-update-rollback.ci` | The operator updates, sees an empty layer, rolls back and finds the earlier file | |
| `image-update-already-current` | `test/container/image-update-already-current.ci` | An update with nothing new answers so and restarts nothing | |
| `image-delete-in-use-refused` | `test/container/image-delete-in-use-refused.ci` | A delete of a running container's image is refused, and a delete of an unused image removes it | |
| `layer-persists-restart` | `test/container/layer-persists-restart.ci` | A file written in a container is there after a container restart and after a daemon restart | |
| `layer-clear` | `test/container/layer-clear.ci` | The clear command leaves the container on an empty layer and its volume untouched | |
| `volume-survives-update` | `test/container/volume-survives-update.ci` | A file in a volume is there after an image update and after a rollback | |
| `volume-delete` | `test/container/volume-delete.ci` | A volume delete is refused while its container runs, and removes an orphan | |
| `store-quota-absent-refused` | `test/container/store-quota-absent-refused.ci` | With the capability forced absent, validate and start refuse a container config and accept one without | |
| `registry-password-obfuscated` | `test/container/registry-password-obfuscated.ci` | The config display does not show the password as typed | |
| `container-store` | `./le test qemu container-store` | On a booted appliance: the cap is enforced, a write past it fails, a file capability is read from a running container, and a written file is there after a reboot | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `TestRegistryInteropDistribution` | `internal/plugins/container/registry/registry_docker_test.go`, with `test/interop-container/Dockerfile.registry` | The reference open-source registry, started with `labtest.Start`, tag `docker` | Ze fetches a gzip image, a zstd image and an index from a real registry, anonymously and with basic credentials, and each stored tree matches the fixture | |
| `TestRegistryLivePublicPull` | `internal/plugins/container/registry/registry_live_test.go` | A public registry, tag `live` | The token exchange and a blob redirect to another host work against a production registry | |

## Files to Modify
- `internal/core/fetch/fetch.go` - export the stall reader and its constructor. No behavior change
- `internal/core/paths/paths.go` - the data directory resolver and its `ze.data.dir` override
- `pkg/zefs/keys.go` - the three container key patterns
- `internal/appliance/cmd_build.go` - `injectZeFS` formats with the quota and project features
- `gokrazy/kernel/runtime.config`, `gokrazy/kernel/runtime.require` - request and require quota support and its on-disk format
- `internal/le/arch/fspersistence/fspersistence.go` - `fileAllowlist` entries for the new files that write, each with its reason
- `internal/le/test/qemu/alltests.go` - `integrationPackages` gains the store package
- `internal/le/test/qemu/actions.go` - the `container-store` action, with a caller in the scheduled QEMU workflow
- `docs/architecture/appliance/builder.md` - the format of the persistent partition
- `docs/architecture/appliance/on-device-installer.md` - the exported stall reader, and what the installer's kernel must support to mount the partition
- `docs/architecture/zefs-format.md` - the key namespace table
- `docs/architecture/doctor-and-health-checks.md` - the store's checks and codes
- `docs/architecture/testing/interop.md` - the registry test in the labtest section
- `docs/architecture/testing/qemu-integration.md` - the `container-store` action beside the other appliance proofs, and the store package in "Existing Integration Test Packages"
- `docs/architecture/linux-limitations.md` - an entry: the project quota feature cannot be added to a mounted filesystem by Ze, so a device formatted before this work cannot cap a layer
- The container plugin's registration and its Stage 1 declaration, created by the runtime child - commands with their bounds, doctor check declarations, event codes, the declared read of `pki`, the state key grant

## Files to Create
- `internal/plugins/container/registry/` - the pull-only client: reference, authentication, redirect rule, verification, platform selection, mirror, TLS
- `internal/plugins/container/imagestore/` - layout, staging and publication, layer application, the fetch worker, free space, generations, layers, volumes, quota, resolve, state
- `internal/plugins/container/image_cmd.go` - the handlers of the commands table C-1 marks as this child's
- `internal/plugins/container/doctor_store.go` - the doctor checks and the code registrations this child owns
- `internal/plugins/container/kernelcap_quota_linux.go` - the storage quota enrolment
- `internal/test/ociregistry/` - the local test registry and its image builder
- `internal/test/fixture/container_registry.go` - the compiled fixture that serves it to a `.ci` test
- `internal/le/test/qemu/containerstore.go` - the appliance proof
- `test/container/*.ci` - the functional tests of the table above
- `test/interop-container/Dockerfile.registry` - the registry image the interop test starts, with its fixture images
- `docs/architecture/container/image-store.md` - the design page: the store layout, the reference forms, the registry requests, the application rules, capped storage, the generations, the seam

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The plugin's own schema folder, with the nodes and commands `spec-container-4-operator-surface.md` marks as this child's |
| YANG validation constraints | Yes | Stated in that spec's tables: the size ranges, the reference pattern, the registry key |
| YANG custom validators | Yes | The trust anchor leaf is refused at commit when it names no `pki ca` entry, as an IPsec peer's is |
| CLI commands/flags | Yes | `internal/plugins/container/image_cmd.go`. No flag: each is a registered command |
| CLI grammar (keyword before value) | Yes | Judged in `spec-container-4-operator-surface.md`, which owns the words |
| Editor autocomplete | Yes | The digest of the delete command and the container and volume names complete from live state |
| Functional test for new RPC/API | Yes | `test/container/*.ci` |
| Pipe completeness | Yes | Every listing answers structured data through the shared renderer, so the generic pipes apply |
| Env var registration | Yes | `ze.data.dir` in `internal/core/paths/paths.go` |
| Doctor check for runtime dependencies | Yes | `internal/plugins/container/doctor_store.go`: the data directory, the quota. The total `TestDoctorDependencyInventory` holds rises |
| Doctor code per problem class | Yes | The table "Doctor checks and codes" |
| Prometheus counters/metrics | No | The metrics of the feature are table M-1 of the operator-surface child. This child supplies the figures it reads |
| BGP family surface (new SAFI / capability / attribute) | N-A | Not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | The feature file the operator-surface child creates: this child adds its evidence rows |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, through `spec-container-4-operator-surface.md` which owns the schema |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, through `spec-container-4-operator-surface.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`: a plugin command that declares a long bound and shows its progress in a listing |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md`, in the row the runtime child adds |
| 6 | Has a user guide page? | Yes | The guide page `spec-container-4-operator-surface.md` creates: fetching, updating, rolling back, sizes, volumes |
| 7 | Wire format changed? | No | The registry protocol is not Ze's format. The store layout is in the design page |
| 8 | Plugin SDK/protocol changed? | No | The SDK state calls and the doctor callback are used as they are |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | The registry protocol is not an RFC |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: the container suite and its registry fixture. `docs/architecture/testing/interop.md` and `docs/architecture/testing/qemu-integration.md`. The workflow page gains the action's caller |
| 11 | Affects daemon comparison? | No | The umbrella decides whether the comparison page gains a row |
| 12 | Internal architecture changed? | Yes | `docs/architecture/container/image-store.md`, new. `docs/architecture/core-design.md`: the data directory beside the config directory |
| 13 | Route metadata keys added/changed? | N-A | No route is touched |
| 14 | Prometheus counters added/changed? | No | None added by this child |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`: the commands and the kernel capability |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | The pages in Files to Modify. Also, from `./le spec citation anchors`: the two environment pages gain the `ze.data.dir` row, and the appliance guide gains the partition format and the space a container needs. The other pages that mention the changed files (the build pins, the installer pages, the push page, the storage backends page, the runner page and the guides for setup and install) describe behavior this spec does not change |
| 17 | Existing docs show config/CLI/API examples for this area? | No | No page shows a container example today |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the entry points exist and reach stubs
   - Tests: `test/container/image-fetch-lists-digest.ci`, `TestResolveAbsentNamesReference`, `TestFetchCommandDeclaresItsBound`, `TestDataDirFromBinary`
   - Files: the plugin's registration and schema, the command and doctor files, the paths resolver, the key file, the seam with the runtime child
   - Verify: each command is listed on every surface and answers that the store is not implemented. The wiring `.ci` fails because no image is fetched
2. **Phase: Store layout and publication** -- the data directory, modes, staging, the completion mark, the listing
   - Tests: `TestPublishIsOneRenameAndRefusesExisting`, `TestListIgnoresFolderWithoutMark`, `TestStartEmptiesStagingAndKeepsCheckedBlobs`, `TestStoreRootOpenToOthersRefused`
   - Files: the image store package, the raw-write allowlist
   - Verify: tests fail, then pass. The listing command answers an empty store
3. **Phase: Registry client** -- the test registry first, then the client
   - Tests: every registry client test of the plan, and `TestStallReaderExported`
   - Files: the test registry package, the registry client package, the fetch source
   - Verify: tests fail, then pass. No credential in any error
4. **Phase: Layer application** -- one rule of the application table at a time, each with its refusal
   - Tests: every `TestApply...` test and `FuzzApplyLayerNeverEscapesRoot`
   - Files: the image store package
   - Verify: tests fail, then pass. The fuzz test runs clean for its budget
5. **Phase: Fetch command, progress and space** -- the worker, the queue, the in-flight row, the kept blobs, the free-space rule
   - Tests: the `TestFetch...` tests, `TestFreeSpaceRefusalBeforeAnyBlob`, `TestListingShowsFetchingWithBytesReceived`, the fetch `.ci` tests
   - Files: the image store package, the command file, the registry fixture
   - Verify: the wiring `.ci` passes. Assumption A-7 is confirmed or broken here
6. **Phase: Capped storage** -- the kernel options first, then the format, then the probe, the layers and the volumes
   - Tests: `TestInjectFormatsPermWithProjectQuota`, the quota tests, the layer and volume tests, `store-quota-absent-refused`
   - Files: the two kernel files, the appliance builder, the capability enrolment, the image store package
   - Verify: the kernel options land and build before the format changes. Assumptions A-1, A-2 and A-3 are confirmed or broken here
7. **Phase: Generations** -- resolve, update, rollback, retention, delete, clear
   - Tests: the generation, delete and resolve tests, and the `.ci` tests for update, rollback, delete, clear, volumes and persistence
   - Files: the image store package, the command file
   - Verify: tests fail, then pass, with the runtime child's start and stop
8. **Phase: Doctor** -- each check and code this child owns
   - Tests: the doctor tests of the plan
   - Files: the doctor file
   - Verify: `./le arch doctor-coverage check` admits every new problem site
9. **Phase: Interop, appliance proof and pages**
   - Tests: `TestRegistryInteropDistribution`, `TestRegistryLivePublicPull`, `./le test qemu container-store`
   - Files: the registry Dockerfile, the QEMU action, its registration and its workflow caller, the integration package list, the pages of Files to Modify and Files to Create
   - Verify: the registry's source is read at the pinned version before the interop test expects an answer from it, and the function is named in the test

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | No blob byte reaches a decompressor or the archive reader before its digest and size were checked. No write goes through an inode that existed before the entry. The generation record is written before the restart |
| Naming | State keys in the container part of the runtime-state namespace. JSON keys kebab-case. Doctor codes start `doctor-container-`. The folder names of the store layout are exactly those of this spec. Command words and leaf names are exactly those of the operator-surface child |
| Data flow | Every change to the store goes through the one worker. The runtime child reads the store only through the seam. No image byte is written under the config directory |
| Operator errors and doctor codes | Every refusal is an operator error with its fix on the last line. Every warning or error line names a code of the doctor table. Each finding carries its fix |
| Rule: process boundary | The store works with the plugin run as a separate process: state through the SDK, doctor through the callback, no call into a component's Go state. `image-fetch-external-plugin` proves it |
| Rule: lifecycle regressions (`ai/rules/testing.md`) | The plugin's start, supervision and failure scenarios belong to the runtime child. This child reaches one: a stop with a fetch in flight, proven by `image-fetch-interrupted-then-clean`. The other rows of that table are not reachable from this child's code |
| Rule: exact or refuse | No measured cap, no skipped free-space check, no corrected path, no guessed layer type, no completed reference |
| Rule: credentials | The password leaf is sensitive. No credential, token or authorization header appears in a log line, an error, a listing, a state key or the completion mark |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The fetch command stores an image by digest | `./le test functional container` runs `image-fetch-lists-digest` |
| Digest and size are checked on every object | The registry client package tests whose names hold Mismatch or Bound |
| A hostile layer changes nothing outside the root | The image store package tests whose names start `TestApply`, and the fuzz run |
| The store works with the plugin in its own process | `image-fetch-external-plugin` in the container suite |
| The caps are enforced by the kernel on the appliance | `./le test qemu container-store` |
| The appliance still builds, installs and boots | `./le test functional appliance`, `./le test qemu installer` |
| Each problem class has a code and a fix | `TestEveryStoreCodeIsDescribed`, `./le arch doctor-coverage check` |
| No raw write without its allowlist reason | `./le arch fs-persistence check` |
| The gate still compiles out | The runtime child's absent build-tag test, and `./le arch tier check` |
| Ze pulls from a real registry | `TestRegistryInteropDistribution` under the `docker` tag |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Every field of a manifest, an index, a configuration and an archive header is untrusted: sizes, counts, media types, paths, link targets, owners, modes, attribute names. Each has a bound or a closed set, and a value outside it refuses the image. A container name and a volume name become folder names only after they matched their pattern |
| Path traversal | No path string from an archive reaches a path-based system call. Every operation goes through a descriptor for the staging root |
| Resource exhaustion | Bounds on every size and count, the decoder's window and the fetch queue. A stall ends a transfer. A caller that leaves ends a fetch |
| Privilege | The fetch worker runs as root. The store root gives no access to group or others. No device node is created. Set-user files exist only under that root |
| Credential handling | Sent to the realm of a challenge and to the exact host and port only. Never to a mirror or a redirect target. Never logged, never stored outside the config |
| Transport | Verified TLS by default, from the system roots or the named anchor. A downgrade to plain HTTP is refused. The HTTP transport is a choice the operator writes for one registry, and no setting turns certificate checks off |
| Trust in content | The digest is the only thing that vouches for content. A tag, a mirror and a redirect target vouch for nothing. A kept blob is trusted because its name is its digest and it took that name only after the check |
| Error leakage | A registry's error text is bounded and has its control characters removed before it reaches an operator |
| Authorization that could fail open | A quota that cannot be set leaves the folder unused and the container stopped. A free-space reading that fails refuses the fetch. A probe that cannot run reads as cannot-determine, never as present |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| Assumption A-1 or A-2 breaks | DESIGN: the cap mechanism is re-decided with the owner before phase 6 continues |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The firmware download blocks because it runs inside the daemon, where no command bound applies. A plugin command has a bound, 30 seconds by default, so a long plugin command declares its own and shows its progress where every surface can read it.
- The format of the persistent partition is decided on the build host and cannot be changed on the device. Any feature that needs a filesystem property has to arrive with the image build and refuse on a device that predates it.
- Downloading a blob before applying it costs the size of the compressed layers, and buys three things: no parser sees unverified bytes, a retry does not have to undo a half-applied layer, and a fetch that stopped resumes at the first blob it had not checked.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| A hand-written, pull-only registry client inside the gated package | A third-party registry module added to `go.mod` | The pull surface is five request kinds. The standard library and the vendored zstd decoder cover transport, archives and compression, so no module is added. A third-party client still leaves layer application to Ze, links push, credential-helper and layout code Ze never runs, and keeps the redirect, credential and size rules inside its transport, where Ze could state them only through a build-time diff. The cost of writing it is that Ze owns the protocol's corner cases, which the interop and live tests cover |
| The client lives in the container plugin's tree, and core gains only the exported stall reader | A new core package beside the fetch package | A core leaf cannot be compiled out, and the owner ruled that a build without the feature carries none of its code |
| The registry client has its own redirect rule | Relaxing `fetch.SameOrigin` | That rule protects the installer and the self-update, where a redirect to another host is the attack. For a registry it is the normal path, made safe by the digest check and by never forwarding the authorization header |
| Each blob is downloaded and checked, then applied, and checked blobs outlive a stopped fetch | Applying a layer while it streams from the network | No decompressor or archive reader sees bytes nobody vouched for, a retry restarts a download and not an application, and the next attempt continues. The blobs of one stopped fetch are the most that stays |
| The image folder is named by the digest of the platform's manifest | The digest of the index, or of the image configuration | It names exactly the content that was applied. An index digest is a reference that resolves to it |
| The completion mark carries the image's facts | A state key per image | The folder then describes itself. A lost state store costs bindings and generations, not images |
| The fetch command answers once at its end, with a declared bound of one hour | A command that answers at once and a separate list of jobs | It is the firmware download's model and the answer shape the operator-surface child fixed. The bound and the caller's departure both stop it safely, because the checked blobs stay |
| The cap is ext4 project quota, set by Ze through system calls, for layers and volumes alike | (a) A cap Ze measures and enforces by stopping the container. (b) One filesystem image file per layer. (c) A memory-backed layer | (a) is not exact: a container fills the partition between two measurements, and the partition holds Ze's config. (b) needs a filesystem writer Ze does not have and may not run as a tool. (c) contradicts the owner's ruling that the layer is on disk. Project quota is enforced by the kernel at the write, needs no tool on the device, and gives exact usage for accounting |
| Where the cap cannot be enforced, a container config is refused | Falling back to the measured cap | The shared design refuses what a backend cannot apply exactly. The cost is that a device formatted before this work, and a distribution whose data filesystem has no project quota, cannot run containers until reinstalled or pointed at a suitable filesystem |
| A new data directory, separate from the config directory | A folder under the config directory | The config directory holds the live store with strict modes and is the tree the raw-write rule protects. Image trees hold arbitrary owners, modes and set-user files and do not belong beside credentials |
| A volume is keyed by container and volume name, without the digest | A volume per generation | The umbrella says state meant to survive lives in volumes. A volume that changed with the image would not survive an update |
| A mirror is asked first and the registry after it | The mirror in place of the registry | A mirror that is down would otherwise stop every fetch. Content is checked by digest either way |
| A device node under the image's device folder is skipped and counted, any other is refused | Creating device nodes, or refusing every image that holds one | Older base images carry nodes there that the runtime's own device folder hides. Creating a node on the host filesystem is never needed and is the risk |
| An update that resolves to the retained previous digest changes nothing and names rollback | Treating it as an update onto a fresh layer | That generation already has a layer with the operator's writes. Replacing it silently would lose them |
| Rollback is a command | An automatic rollback when the new image fails to start | The restart policy of the runtime child already handles a failing container. The umbrella's criteria ask for a rollback the operator runs |
| The default appliance image stays 2 GiB | A larger default | An appliance without containers must not pay for them. The partition does not grow, so the size is the build's `image.size` setting, and every fetch names the space it lacks |

## Known Limitations
- Layers are not shared between images. Two images with one base store it twice. This follows the owner's ruling for one folder per image digest.
- The persistent partition keeps the size the image was built with. This child settles that fact (assumption A-4) and works at any size. Whether Ze grows the partition at install is the umbrella's question, raised in this child's report, and is not designed here.
- A device installed from an image built before this work cannot run containers, because its persistent partition has no project quota and Ze cannot add it on the device.
- On a distribution, the data directory must be on an ext4 filesystem with project quota. No other filesystem is supported.
- Images are fetched from a registry only. Building an image on the device, loading one from a file and pushing one are not part of the umbrella's scope.
- Image signatures are not verified. The digest proves the content is what the registry named, not who built it.
- A fetch resumes at a whole blob. A blob that was partly downloaded starts again from its first byte.

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the container functional suite, `./le test functional appliance`, and the QEMU actions this spec names. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for the registry protocol

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
