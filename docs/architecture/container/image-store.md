# The container image store

The image store is the part of the container plugin that puts an image on
disk: it fetches an OCI image from a registry, keeps it in one folder named
by its digest, and gives each container a writable layer and its volumes
above it. The runtime ([runtime.md](runtime.md)) starts a container from what
the store answers. The design is `plan/post-release/features/spec-container-2-image-store.md`;
this page describes what the tree holds today.

## Where it is today

The store lands in phases. Two are in the tree. The wiring: the seam, the
commands, the doctor codes, the state keys and the data directory. The
layout: the store root and its folders, staging emptied at start,
publication of an image folder by one rename, and the image listing. Every
other operation of the store answers that it is not implemented in this
build. No image is fetched yet, so the listing answers an empty store, and
no container starts on the real store: its resolve answers an error, which
the supervisor turns into the `failed` state with the reason.

| Piece | Where | Today |
|-------|-------|-------|
| The store package | `internal/plugins/container/imagestore`, gated by `ze_container` | `Store` answers `show container image list` from the store's folders. `show container store` waits for the quota probe, because its `caps-enforced` field cannot be answered without it. Every other operation answers "not implemented in this build" |
| The seam the runtime asks | `storeseam.go`: `Store`, `SetStoreFactory` | The store package installs its factory from its own `init()`; the runtime package never imports it |
| The seam the commands ask | `storeseam_operations.go`: `ImageOperations` | A store MAY also be an `ImageOperations`; an image command refuses, naming why, when it is not |
| The store a daemon builds | `engineStoreFactory`, `storeseam_engine.go` and `storeseam_zetest.go` | A shipped build builds the store package's store. A `zetest` daemon builds the runtime's fixture store when `ze.test.container.store` is set, and the real store otherwise |
| The commands | `image_cmd.go`, `yang/ze-container-cmd.yang` | Ten command nodes, each proxied to the plugin's process |
| The doctor codes | `doctor_store.go` | Eleven codes, registered from the plugin's package, six of them as report-bus event codes |
| The state keys | `pkg/zefs/keys.go` | Three patterns under `meta/container/`, granted to the plugin |
| The data directory | `internal/core/paths/paths.go` | `ze.data.dir` wins. Without it the directory follows the binary's location, as the config directory does (the table below) |

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
<!-- source: internal/plugins/container/imagestore/digest.go -- imageDigest, parseDigest, parseDigestFolder -->
<!-- source: internal/plugins/container/imagestore/sync_linux.go -- syncFilesystem -->
<!-- test: internal/plugins/container/imagestore/publish_test.go TestPublishIsOneRenameAndRefusesExisting, TestListIgnoresFolderWithoutMark, TestStartEmptiesStagingAndKeepsCheckedBlobs -->
<!-- test: internal/plugins/container/imagestore/layout_test.go TestStoreRootOpenToOthersRefused -->
<!-- test: internal/core/paths/paths_test.go TestDataDirFromBinary, TestDefaultDataDirHonorsOverride -->

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
one.

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
