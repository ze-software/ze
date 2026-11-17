# The container image store

The image store is the part of the container plugin that puts an image on
disk: it fetches an OCI image from a registry, keeps it in one folder named
by its digest, and gives each container a writable layer and its volumes
above it. The runtime ([runtime.md](runtime.md)) starts a container from what
the store answers. The design is `plan/post-release/features/spec-container-2-image-store.md`;
this page describes what the tree holds today.

## Where it is today

The store lands in phases. Five are in the tree. The wiring: the seam, the
commands, the doctor codes, the state keys and the data directory. The
layout: the store root and its folders, staging emptied at start,
publication of an image folder by one rename, and the image listing. The
registry client: references, manifests, platform selection and blobs
checked as they arrive. Layer application: the archives of an image
applied into a staged root by the rules of its section below. The fetch:
`create container image reference` runs on the store's one worker, keeps the
blobs it checked, refuses a fetch that would cross the free-space reserve,
and publishes the image; the listing shows the fetches in flight and the
failed ones (the section "The fetch"). Every other operation of the store
answers that it is not implemented in this build. No binding records yet
which image a reference names, so no container starts on the real store:
its resolve answers an error, which the supervisor turns into the `failed`
state with the reason.

| Piece | Where | Today |
|-------|-------|-------|
| The store package | `internal/plugins/container/imagestore`, gated by `ze_container` | `Store` answers `create container image reference` and `show container image list`. `show container store` waits for the quota probe, because its `caps-enforced` field cannot be answered without it. Every other operation answers "not implemented in this build" |
| The seam the runtime asks | `storeseam.go`: `Store`, `SetStoreFactory` | The store package installs its factory from its own `init()`; the runtime package never imports it |
| The seam the commands ask | `storeseam_operations.go`: `ImageOperations` | A store MAY also be an `ImageOperations`; an image command refuses, naming why, when it is not |
| The store a daemon builds | `engineStoreFactory`, `storeseam_engine.go` and `storeseam_zetest.go` | A shipped build builds the store package's store. A `zetest` daemon builds the runtime's fixture store when `ze.test.container.store` is set, and the real store otherwise |
| The commands | `image_cmd.go`, `yang/ze-container-cmd.yang` | Ten command nodes, each proxied to the plugin's process |
| The doctor codes | `doctor_store.go` | Eleven codes, registered from the plugin's package, six of them as report-bus event codes |
| The state keys | `pkg/zefs/keys.go` | Three patterns under `meta/container/`, granted to the plugin |
| The data directory | `internal/core/paths/paths.go` | `ze.data.dir` wins. Without it the directory follows the binary's location, as the config directory does (the table below) |
| The registry client | `internal/plugins/container/registry`, gated by `ze_container` | Resolves a reference and fetches checked blobs (the section below), for the fetch worker |
| The test registry | `internal/test/ociregistry` | An in-process registry the client's and the store's tests fetch from, with scripted faults, a held transfer and a fixed address |
| The registry fixture | `internal/test/fixture/register_container_registry.go` | `le test fixture container/registry <port>` serves one image under `library/redis:latest` to a functional test's daemon |

<!-- source: internal/plugins/container/imagestore/store.go -- Store, newStore, openStore, storePath, withRoot, openRoot, ListImages, notImplemented -->
<!-- source: internal/plugins/container/imagestore/register.go -- init -->
<!-- source: internal/plugins/container/storeseam_operations.go -- ImageOperations, ImageState, VolumeState -->
<!-- source: internal/plugins/container/storeseam_engine.go -- engineStoreFactory -->
<!-- source: internal/plugins/container/storeseam_zetest.go -- engineStoreFactory -->
<!-- source: internal/core/paths/paths.go -- DataDirFromBinary, DefaultDataDir -->

## The data directory and the store layout

The data directory is a folder of its own, never one under the config
directory: the config directory holds the live store and its credentials,
and an image tree holds arbitrary owners, modes and set-user files.
`classifyBinary` recognizes where a binary sits, once, and both
`ConfigDirFromBinary` and `DataDirFromBinary` answer from it, so the two
directories of one binary cannot disagree.

| Binary location | Config directory | Data directory |
|-----------------|------------------|----------------|
| The appliance, `/user/ze` | `/perm/ze` | `/perm/var/lib/ze` |
| A system prefix, `/usr/bin/ze` and the like | `/etc/ze` | `/var/lib/ze` |
| Another prefix, `<prefix>/bin/ze` | `<prefix>/etc/ze` | `<prefix>/var/lib/ze` |
| A relative binary directory, `bin/ze` | `etc/ze` | `var/lib/ze`, made absolute by the store |
| No known layout | none | none: the store refuses every operation and names `ze.data.dir` |

Everything of the feature is under `container/` in the data directory. The
folder names are the field contract: an appliance in the field holds them,
so a name never changes once shipped.

| Path under the data directory | Holds |
|-------------------------------|-------|
| `container/` | The store root. Owned by the user Ze runs as, with no access for group or others, so no local user reaches a set-user file of an image |
| `container/images/sha256-<64 hex digits>/` | One published image: `rootfs/`, `manifest.json`, `config.json`, and the completion mark `complete` |
| `container/layers/` | The writable layers, by container and digest (a later phase) |
| `container/volumes/` | The volumes, by container and volume name (a later phase) |
| `container/blobs/sha256-<64 hex digits>` | The checked blobs of the one fetch that stopped before it published |
| `container/staging/` | What is not checked, not applied or not removed yet |

Each operation opens the store root, checks it and closes it, so a root that
changed owner or mode while Ze ran is refused at the next operation, and no
handle outlives the call. The root is opened through `os.Root`, so no name
the store builds leaves it. A root that is a symbolic link, is not a folder,
belongs to another user, or is open to group or others is not used: the
operation answers an operator error that names the folder and the command
that fixes it, and nothing is created in it.

The first open of a run that succeeds prepares the root: it creates the
folders that are absent, then empties staging. Staging is emptied once per
run and never again in it, because a later empty would remove the work of a
fetch in flight. The plugin's start tries this preparation, and when it
fails it logs the refusal at WARN under `doctor-container-store-unwritable`;
each operation tries again and answers the refusal it meets. The blobs stay:
a blob takes its digest as its name only after it was checked, so its name
is the proof it is whole.

### Publication and the completion mark

An image is applied under `staging/`, then published by `publish`:

1. The staged folder must hold `rootfs/`, `manifest.json` and `config.json`.
2. The filesystem is synced, so the applied root is on the disk before the
   mark says it is whole. On Linux this is one sync of the store's
   filesystem, cheaper than a sync of each file of the root. The sync also
   reports, once, any write that filesystem lost since the last sync,
   whichever file it belonged to. Publication then fails on purpose: the
   report cannot say whether the lost write was one of this image's files,
   so the store does not vouch for the root. The refusal says the
   filesystem reported a write it could not complete and points at the
   kernel log, and a fetch run again syncs cleanly unless the disk loses
   another write.
3. The completion mark is written into the staged folder, with `O_EXCL`, so
   it is never written through a name that holds anything, and synced.
4. The staged folder is renamed into `images/` by `zefs.RenameNoReplace`,
   which fails when the target exists. A reader sees no folder or a whole
   image, and a second publication of one digest never replaces the first.

The mark is a JSON document, read-only: `digest`, `reference` (the whole
reference the publishing fetch named), `platform`, `registry` (the host that
served the bytes), `fetched`, `expanded-bytes`, `entries` and
`devices-skipped`.

The listing reads `images/`. A folder whose name is no digest, and a digest
folder without a mark, are not images and are passed over: publication
renames a folder only after its mark is written, so neither came from a
whole fetch. A mark that cannot be read, is larger than 64 KiB, or names
another digest than its folder is refused with the folder's path, because
that folder claims to be whole. Each image is one row, in digest order,
state `present`, its size the expanded bytes. `used-by` is empty: a
container holds an image only once resolve answered it ready, which a later
phase brings.

<!-- source: internal/core/paths/paths.go -- classifyBinary, ConfigDirFromBinary, DataDirFromBinary -->
<!-- source: internal/plugins/container/imagestore/layout.go -- storeFolder, storeSubfolders, storeRoot, prepareStoreRoot, openStoreRoot, checkStoreFolder, makeSubfolders, emptyStaging -->
<!-- source: internal/plugins/container/imagestore/publish.go -- imageFacts, completionMark, publish, checkStagedImage, writeMark, syncLostWrite, images, readMark -->
<!-- source: internal/plugins/container/imagestore/digest.go -- parseDigestFolder, digestFolder -->
<!-- source: internal/plugins/container/imagestore/sync_linux.go -- syncFilesystem -->
<!-- test: internal/plugins/container/imagestore/publish_test.go TestPublishIsOneRenameAndRefusesExisting, TestListIgnoresFolderWithoutMark, TestStartEmptiesStagingAndKeepsCheckedBlobs -->
<!-- test: internal/plugins/container/imagestore/layout_test.go TestStoreRootOpenToOthersRefused -->
<!-- test: internal/core/paths/paths_test.go TestDataDirFromBinary, TestDefaultDataDirHonorsOverride -->

## The registry client

The client is pull only, written over the standard library's HTTP client, in
the gated package `internal/plugins/container/registry`. It stores nothing:
it hands the store a resolved image and writes a blob into the file the
store gives it.

**A reference is completed once.** `ParseReference` applies the host rule: the
first component is a registry host when it holds a dot or a colon or is
`localhost`, otherwise the name is a repository of the public hub,
`docker.io`, where a name of one component goes under `library/`. No tag and
no digest means the tag `latest`. `index.docker.io` is written `docker.io`,
and the hub is dialled at `registry-1.docker.io`. The short and the whole
spelling of one image are one value. A part that does not match its grammar
refuses the reference, naming the part: nothing is corrected, an upper-case
repository or registry host included. A digest is SHA-256 in lower-case hex
only.

**Resolve** asks for the manifest by its digest when the reference pins one,
by its tag otherwise. An index is resolved to the entry for Linux on the
running architecture (arm64 variant `v8` equals no variant; an amd64 entry
above the baseline feature level is passed over), and the refusal of an index
without one lists the platforms it offers. Then the image configuration is
fetched, and its platform must be the running one too.

| Object | Checked |
|--------|---------|
| A manifest or an index | At most 4 MiB. Its bytes hash to the digest asked for, to the digest the answer declares in `Docker-Content-Digest`, and, for an index entry, have the size the index declares. The type is the body's `mediaType`, or the answer's content type when the body names none |
| Its type | An OCI index or manifest, a Docker manifest list or manifest. Schema 1, a nested index and any other type are refused by name |
| The configuration | An OCI or Docker image configuration type, otherwise the manifest is not a container image. At most 4 MiB, its digest and size checked as it arrives |
| The layers | From 1 to 128, each an uncompressed, gzip or zstd tar. A foreign or non-distributable layer, a layer with foreign addresses, and an encrypted layer are refused |
| A blob | Each byte counted against the declared size as it arrives, the copy cut before the first byte past it, and the whole hashed against the digest. A refused blob leaves the store's file empty |

**Requests.** The standard client follows no redirect: `follow` does, at
most 10, refusing a redirect from HTTPS to another scheme. The
authorization header goes only to the exact host and port it was obtained
for, ports written out from the scheme, so a blob redirect to another host
carries none. A transport error and a server error are retried, three
attempts with a delay that doubles from one second. A mismatch is never
retried, because another attempt fetches another file. A body that sends
nothing for 60 seconds ends with the core fetch package's `StallReader`
error, and a context that ends stops the request at once. No proxy is used.

**Authentication.** A 401 is answered once per request. A bearer challenge
sends the credentials, when the configuration holds them, to the realm it
names, and only there; the realm's answer is bounded at 64 KiB and its
token must be token68. A realm over plain HTTP is refused for a registry
reached over HTTPS. A basic challenge sends the credentials to the registry
itself, and is refused when there are none.

**The mirror** is asked first, once, with no credentials. When it fails in
any way, a mismatch included, the registry is asked, with its retries, and
the mirror's failure is kept (`LastMirrorFailure`) for the fetch worker to
report.

**Trust.** The HTTPS transport verifies the certificate against the system
roots, or against the trust anchor alone when the registry names one. There
is no transport without certificate checks.

**Errors.** Each failure is an `Error` of one class, `Failure`: unreachable,
auth-refused, not-found, digest-mismatch or refused. The classes are the
fetch rows of the doctor table below; the container plugin, which owns the
codes and the commands, adds the code and the fix. No error names a
credential, a token or an authorization header: every URL is built without
userinfo, a registry's error body is reduced to its error code, and
`Credentials` print as a placeholder under every format verb. The client
writes no log line.

<!-- source: internal/plugins/container/registry/reference.go -- ParseReference, ParseDigest, Digest, Reference, splitRegistry, endpointHost -->
<!-- source: internal/plugins/container/registry/client.go -- New, Config, Credentials, Transport, Architecture, RunningArchitecture, fromMirrorThenRegistry, withRetry, get, follow, canonicalHost, transportFailure, statusFailure -->
<!-- source: internal/plugins/container/registry/auth.go -- answerChallenge, token, tokenOnce, parseChallenge -->
<!-- source: internal/plugins/container/registry/manifest.go -- Resolve, manifestOnce, parseManifest, selectPlatform, imageParts, checkLayer, checkConfigPlatform -->
<!-- source: internal/plugins/container/registry/blob.go -- FetchBlob, BlobFile, copyVerified -->
<!-- source: internal/plugins/container/registry/errors.go -- Failure, Error -->
<!-- source: internal/test/ociregistry/registry.go -- Start, Options, Fault, PutImage, PutIndex -->
<!-- test: internal/plugins/container/registry/reference_test.go TestParseReferenceForms, TestParseReferenceHostRule, TestParseReferenceRefusesMalformed -->
<!-- test: internal/plugins/container/registry/pull_test.go TestPullAnonymousManifestAndBlobs, TestRetryOnServerErrorThenSuccess, TestBlobStallTimesOut, TestFetchCancelStopsTransfer -->
<!-- test: internal/plugins/container/registry/auth_test.go TestPullBearerTokenExchangeSendsCredentialsToRealmOnly, TestPullBasicChallenge, TestTokenRealmOverPlainHTTPRefused, TestTokenAnswerSizeBound -->
<!-- test: internal/plugins/container/registry/redirect_test.go TestRedirectToOtherHostDropsAuthorization, TestRedirectSameHostKeepsAuthorization, TestRedirectDowngradeRefused, TestRedirectLimit -->
<!-- test: internal/plugins/container/registry/verify_test.go TestManifestDigestMismatchRefused, TestBlobDigestMismatchRefused, TestBlobSizeMismatchRefused, TestDigestMismatchNotRetried, TestManifestSizeBound, TestNonSHA256DigestRefused -->
<!-- test: internal/plugins/container/registry/platform_test.go TestIndexSelectsRunningPlatform, TestIndexWithoutPlatformNamesOffered, TestUnacceptedManifestTypesRefused -->
<!-- test: internal/plugins/container/registry/mirror_test.go TestMirrorTriedFirstThenRegistry, TestMirrorGetsNoRegistryCredentials -->
<!-- test: internal/plugins/container/registry/tls_test.go TestHTTPTransportFetchesOverPlainHTTP, TestHTTPSTransportRefusesPlainHTTP, TestTrustAnchorReplacesSystemRoots -->
<!-- test: internal/plugins/container/registry/secret_test.go TestCredentialsNeverInErrorOrLog -->

## Layer application

An image is hostile input: its layers are archives a registry chose. The
applier turns them, in order, into the root of one staged folder, and
nothing it does can reach a name outside that root. Nothing in the store
calls it yet: the fetch worker that will is the next phase.

| Step | What the applier does |
|------|-----------------------|
| Open | `newLayerApplier` creates `staging/<staged>/` with an empty `rootfs/` and a `removed/` folder beside it. `close` removes `removed/` whole and MUST run before the staged folder is published |
| Decompress | Uncompressed tar, gzip and zstd only. The zstd decoder runs in the caller's goroutine with a window of at most 32 MiB |
| Count | Every decompressed byte counts against the largest image size, over all layers, and so does every byte written into a file, which a sparse entry can make larger |
| Check the stream | The whole decompressed stream, the end of the archive included, MUST hash to the digest the image configuration declares for the layer |
| Resolve | Every entry's folder is resolved under a descriptor for the root, through the path-safety library runc vendors (`pathrs-lite`, `MkdirAllHandle` and `OpenatInRoot`). A symbolic link on the way is resolved inside the root, as a process in the container resolves it: an absolute target starts again at the root and a parent step stops at it |
| Change a name | A descriptor-relative system call on that folder (`openat`, `mkdirat`, `symlinkat`, `linkat`, `mknodat`, `renameat`), so no name of the image reaches a path-based call |
| Replace | What a name held is renamed into `removed/` before the entry takes the name, so a new file is a new inode and nothing is written through a link or a shared inode |

The rules, entry by entry:

| Entry or case | What the store does |
|---------------|---------------------|
| Regular file | Created with `O_EXCL` and `O_NOFOLLOW` after the name is moved away, then the owner, the mode, the attributes and the times |
| Folder | Created, or an existing folder is kept and its metadata updated. Its times are set when the layer ends, the last folder first, so they come after its children |
| Symbolic link | Created with its target as written, 1 to 4096 bytes with no NUL |
| Hard link | The target is resolved under the root and MUST be a regular file that exists, otherwise the image is refused. The link shares the target's owner, mode and attributes |
| Whiteout `.wh.<name>` | The name and everything under it move to `removed/`. A whiteout of a name no layer applied changes nothing |
| Opaque marker `.wh..wh..opq` | Each child the folder holds that this layer did not apply moves to `removed/`. A child the layer applied before the marker is held, and one after it is created after the removal, so the archive's order does not matter |
| Other `.wh..wh.` markers | Not created |
| Device node under `dev/` | Not created, and counted as `devices-skipped` in the completion mark |
| Device node elsewhere, or as `dev` itself | The image is refused, naming the entry |
| Named pipe | Created |
| Owner | The numeric user and group, 0 to 4294967294. Names are ignored |
| Mode | The twelve bits, set after the owner, because a change of owner clears the set-user and set-group bits |
| Extended attribute | Kept on a file or a folder for `security.capability`, `user.*` and the two POSIX access-list names, set after the owner, which clears a file capability. `trusted.overlay.*` and `user.overlay.*`, any other name, the libarchive form, an attribute on a link or a pipe, and more than 64 KiB on one entry refuse the image |
| Path | An absolute path, a `..` component, an empty component and a NUL byte refuse the image. A `.` component names the folder it is in and is passed over. A folder's name may end in one slash |
| Lengths | A name over 255 bytes or a path over 4096 bytes refuses the image |
| Entries | At most 500,000 per image. A folder on the way to an entry that its layer names for the first time counts as an entry, because the applier may create it |
| Layers | At most 128 per image |
| Archive | A global PAX header names no file and is passed over. Any other type, a malformed archive and a stream that does not hash to its digest refuse the image |

A refusal is a `layerRefusal`, which names the entry, quoted in ASCII and
cut at 256 bytes, and the rule; the fetch worker reports it under
`doctor-container-image-refused`. A system call that fails, such as an
owner the user Ze runs as cannot give, is an `entryFailure` that carries the
system's error. Either way the staged folder holds part of the image, and
the caller removes it and publishes nothing. The applier and its rules are
built on Linux only. On another system the fetch's `applyLayers`
(`fetch_apply_other.go`) refuses: containers run on Linux only.

<!-- source: internal/plugins/container/imagestore/apply_rules.go -- applyLimits, defaultApplyLimits, applyCounts, refusalRule, layerRefusal, entryFailure, parseEntryPath, entryAttributes, checkAttributeName, entryOwner, checkLinkTarget, openLayerStream, expandedStream -->
<!-- source: internal/plugins/container/imagestore/apply_linux.go -- newLayerApplier, apply, applyEntry, applyFile, applyFolder, applyHardLink, applyWhiteout, applyOpaque, moveAway, parentFolder, existingFolder, setMetadata -->
<!-- test: internal/plugins/container/imagestore/apply_test.go TestApplyRegularTreeKeepsOwnerModeTime, TestApplyCompressionsProduceSameTree, TestApplyDiffDigestMismatchRefused -->
<!-- test: internal/plugins/container/imagestore/apply_whiteout_test.go TestApplyWhiteoutRemovesLowerEntry, TestApplyOpaqueDirectoryKeepsSameLayerEntries -->
<!-- test: internal/plugins/container/imagestore/apply_link_test.go TestApplyHardLinkInsideRoot, TestApplyHardLinkOutsideRootRefused, TestApplySymlinkParentResolvedInsideRoot -->
<!-- test: internal/plugins/container/imagestore/apply_refuse_test.go TestApplyParentDirectoryPathRefused, TestApplyReplacesEntryNeverWritesThrough, TestApplyDeviceNodeUnderDevCounted, TestApplyDeviceNodeElsewhereRefused -->
<!-- test: internal/plugins/container/imagestore/apply_xattr_test.go TestApplyOverlayControlAttributeRefused -->
<!-- test: internal/plugins/container/imagestore/apply_xattr_linux_test.go TestApplyXattrFileCapabilityKept -->
<!-- test: internal/plugins/container/imagestore/apply_owner_linux_test.go TestApplyOtherOwnersKept -->
<!-- test: internal/plugins/container/imagestore/apply_bound_test.go TestApplyExpandedSizeBound, TestApplyEntryCountBound, TestApplyLayerCountBound, TestApplyNameAndPathLengthBound -->
<!-- test: internal/plugins/container/imagestore/apply_fuzz_test.go FuzzApplyLayerNeverEscapesRoot -->

## The commands

Each command is a YANG command node of the plugin's command module, bound to
a wire method the plugin's package registers as a proxy to the plugin's
process, internal or forked (`imageCommandRPCs`, `forwardToPlugin`). As for
the restart, the plugin command stops before the keyword of its first leaf.
The words and the answer shapes are table C-1 of
`plan/post-release/features/spec-container-4-operator-surface.md`.

| Command | Wire method | Answer |
|---------|-------------|--------|
| `show container image list` | `ze-container:show-image-list` | Rows under `images`: `reference`, `digest`, `size`, `fetched`, `state` (`present`, `fetching`, `failed`, `previous`), `received`, `used-by`, and `error` and `code` for a failed fetch |
| `show container store` | `ze-container:show-store` | One document: `path`, `total`, `free`, `reserve`, `caps-enforced`, and the rows `layers` |
| `show container volume list` | `ze-container:show-volume-list` | Rows under `volumes`: `instance`, `name`, `size`, `used`, `state` (`in-use`, `orphan`) |
| `create container image reference <image>` | `ze-container:create-image` | `reference`, `digest`, `size`, `created`, `started` |
| `update container image name <container>` | `ze-container:update-image` | `name`, `previous-digest`, `digest`, `changed`, `restarted` |
| `update container image rollback name <container>` | `ze-container:update-image-rollback` | `name`, `previous-digest`, `digest`, `restarted` |
| `delete container image digest <digest>` | `ze-container:delete-image` | `deleted` and `kept` rows |
| `delete container image all` | `ze-container:delete-image-all` | The same |
| `delete container volume name <volume> instance <container>` | `ze-container:delete-volume` | `instance`, `name`, `size` |
| `clear container layer name <container>` | `ze-container:clear-layer` | `name`, `cleared`, `restarted` |

In the plugin's process, `handleCommand` looks the command up in
`imageCommandHandlers`. A handler reads its leaves by their keywords; a
command with one leaf also takes the bare word a programmatic sender passes.
The volume delete reads both of its leaves by keyword only. A missing leaf is
refused with the command that supplies it, and the store's operation is not
called.

A fetch and an update run for minutes, and a plugin command is cut at 30
seconds unless it declares its own bound. The Stage 1 command declaration has
no field for a bound, so the two commands declare none today:
`TestFetchCommandDeclaresItsBound` is red until the plugin protocol carries
one. Until then the fetch command answers within 20 seconds and the fetch
goes on in the store's worker (the section "The fetch").

## The fetch

`FetchImage` completes the reference and hands the fetch to the store's one
worker. One fetch runs and at most eight wait; a ninth is refused, naming the
queue. A fetch of a reference already queued or running is joined. The
worker is started when a fetch arrives and none runs, takes the fetches one
at a time, and ends when none waits.

| Step | What the worker does |
|------|----------------------|
| Client | `engineClientConfig`: the registry the reference names, verified TLS through the system roots, anonymous, no mirror, the running architecture. The registry list of the operator surface will replace it. A `zetest` daemon reaches every registry through the loopback mirror `ze.test.container.registry.port` names, over plain HTTP |
| Resolve | The client resolves the manifest and reads the image configuration. A digest already in the store downloads no layer: the answer says it was not new |
| Leftover blobs | Every blob in `blobs/` that is no layer of this image is removed: the blobs of a stopped fetch of another image |
| Free space | The bytes to download (manifest, configuration and each layer not kept) must leave the reserve free: the larger of 64 MiB and 5% of the filesystem. Otherwise the fetch is refused before any layer is asked for, naming the three numbers, under `doctor-container-fetch-no-space`. A filesystem whose space cannot be read refuses the fetch too |
| Download | Each missing layer is written to `staging/blob-sha256-<hex>`, counted into the bytes received, and renamed into `blobs/` once the client checked its digest and size |
| Apply | The layers are applied in order into `staging/image-sha256-<hex>`, each checked against the configuration's `diff_ids`; the manifest and the configuration are written beside the root, byte for byte |
| Publish | `publish`, then the image's blobs are removed, and the runtime is told the reference is present; the answer names the containers it started |

The fetch command waits at most 20 seconds for its fetch, because a plugin
command is cut at 30 seconds. A fetch that runs longer is answered with an
operator error saying it is still running, whose fix is the listing; the
fetch goes on to its end, so the cut costs the caller the wait and never the
image. The fetch runs under the command's context, which for a plugin
command is the plugin's run context: it stops when the plugin stops, and a
caller that goes away does not stop it.

The listing adds to the published images one row for each fetch in flight,
`fetching` with the bytes it received, and one row for the last failed fetch
of each reference, `failed` with the error's problem and its doctor code. At
most 16 failed rows stay, the oldest leaving first, and a fetch of the
reference that ends removes its row.

| Failure | Code | Fix |
|---------|------|-----|
| The registry or its realm could not be reached | `doctor-container-registry-unreachable` | The fetch again |
| The registry refused the credentials or the access | `doctor-container-registry-auth-refused` | The registry's credentials statement |
| No such repository, tag or digest | `doctor-container-image-not-found` | The fetch with the right reference |
| A digest or size mismatch | `doctor-container-image-digest-mismatch` | The fetch again |
| A type, a platform or an application rule refused | `doctor-container-image-refused` | Another image, or `store largest-image` for the expanded-bytes bound |
| Too little space | `doctor-container-fetch-no-space` | `delete container image all` |
| A write the system refused, a store folder that failed | `doctor-container-store-unwritable` | `ze show doctor` |

No binding is written yet, and no failure is raised on the report bus: the
store holds no handle on the plugin's SDK until the generations phase. Both
wait for it.

<!-- source: internal/plugins/container/imagestore/fetch.go -- FetchImage, enqueueFetch, runFetches, fetchQueue, fetchError, failedRow, fetchingRow -->
<!-- source: internal/plugins/container/imagestore/fetch_steps.go -- fetchInto, download, checkSpace, storeReserve, downloadBlob, countedBlobFile, diffIDs, defaultClientConfig, registryFailed -->
<!-- source: internal/plugins/container/imagestore/fetch_apply_linux.go -- applyLayers, applyFailed -->
<!-- source: internal/plugins/container/imagestore/registry_config_zetest.go -- engineClientConfig -->
<!-- test: internal/plugins/container/imagestore/fetch_test.go TestFetchCommandAnswersResultAtEnd, TestFetchOfPresentDigestDownloadsNothing, TestFetchReusesCheckedBlob, TestFetchOfAnotherImageRemovesLeftoverBlobs, TestFreeSpaceRefusalBeforeAnyBlob, TestListingShowsFetchingWithBytesReceived, TestFetchQueueBound, TestFetchFailureClassesNameTheirCode, TestFetchShortNameRecordsResolvedDigest -->

<!-- source: internal/plugins/container/image_cmd.go -- imageCommandDecls, imageCommandRPCs, forwardToPlugin, imageCommandHandlers, runImageCommand, keywordValue, singleValue -->
<!-- source: internal/plugins/container/container.go -- commandDecls, handleCommand -->
<!-- test: internal/plugins/container/image_cmd_test.go TestEveryImageCommandIsDeclaredAndHandled, TestImageCommandsReachTheStore, TestImageCommandRefusesWhatItCannotRun, TestFetchCommandDeclaresItsBound -->

## Doctor codes

The plugin registers the store's codes beside the runtime's, from its own
package (`registerContainerDiagnosticCodes`), so a build without `ze_container`
holds none of them. The six codes of a fetch's failures are also declared as
event codes (`registerStoreEventCodes`), after their explanations.

| Code | Problem | Reported by |
|------|---------|-------------|
| `doctor-container-store-unwritable` | The store folder is absent, cannot be written, or is open to group or others | The runtime check `container-store`, from the store's `CheckStore`: no data directory, an absent root, a symbolic link, not a folder, another owner, or access for group or others. A read-only filesystem is not probed yet. Also the WARN line of a start whose preparation failed |
| `doctor-container-store-full` | The data filesystem is below the store's reserve | The same check, once the store's free-space rule lands: not reported yet |
| `doctor-container-kernel-quota-unavailable`, `doctor-container-kernel-quota-unknown` | The data directory's filesystem cannot cap a layer or a volume, or Ze could not tell | The kernel capability `container-quota`, not enrolled yet |
| `doctor-container-storage-full` | A writable layer or a volume is at its cap | The runtime check `container-storage-usage`, not declared yet |
| `doctor-container-registry-unreachable` | A fetch could not reach the registry | An event of the fetch |
| `doctor-container-registry-auth-refused` | The registry refused the credentials or the access | An event of the fetch |
| `doctor-container-image-not-found` | The registry holds no such image or tag | An event of the fetch |
| `doctor-container-image-digest-mismatch` | A manifest, a configuration or a layer did not match its digest or size | An event of the fetch |
| `doctor-container-image-refused` | An image was refused: an unsafe entry, a bound, a type, no entry for this platform | An event of the fetch |
| `doctor-container-fetch-no-space` | A fetch was refused for lack of space | An event of the fetch |

<!-- source: internal/plugins/container/doctor_store.go -- storeDiagnosticCodes, registerStoreEventCodes, storeDoctorChecks, answerStoreUsable -->
<!-- source: internal/plugins/container/imagestore/store.go -- CheckStore -->
<!-- test: internal/plugins/container/doctor_store_check_test.go TestStoreDoctorCheckAnswersTheStoresRefusal -->
<!-- source: internal/plugins/container/register.go -- registerContainerDiagnosticCodes -->
<!-- test: internal/plugins/container/doctor_store_test.go TestStoreDiagnosticCodesRegisterFromThePlugin -->

## State keys

The store keeps small records in the daemon's state store, through the SDK
state calls, under three keys the store package grants the plugin
(`statestore.RegisterPluginKeys`). Images, layers and volumes are folders on
the data directory, never keys.

| Key | Holds |
|-----|-------|
| `meta/container/binding/{reference-digest}` | The digest one image reference is bound to. The key carries the SHA-256 of the reference, because a reference holds a slash and a key parameter matches one path segment |
| `meta/container/generation/{container}` | A container's current and previous digests and its last 20 events |
| `meta/container/project-id` | The counter the next storage project id comes from |

<!-- source: pkg/zefs/keys.go -- KeyContainerBinding, KeyContainerGeneration, KeyContainerProjectID -->
