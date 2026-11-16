# The container image store

The image store is the part of the container plugin that puts an image on
disk: it fetches an OCI image from a registry, keeps it in one folder named
by its digest, and gives each container a writable layer and its volumes
above it. The runtime ([runtime.md](runtime.md)) starts a container from what
the store answers. The design is `plan/post-release/features/spec-container-2-image-store.md`;
this page describes what the tree holds today.

## Where it is today

The store lands in phases. The first phase, the wiring, is in the tree: the
seam, the commands, the doctor codes, the state keys and the data directory
exist, and every operation of the store answers that it is not implemented in
this build. No image is fetched yet, and no container starts on the real
store: its resolve answers an error, which the supervisor turns into the
`failed` state with the reason.

| Piece | Where | Today |
|-------|-------|-------|
| The store package | `internal/plugins/container/imagestore`, gated by `ze_container` | `Store` answers every operation with "not implemented in this build" |
| The seam the runtime asks | `storeseam.go`: `Store`, `SetStoreFactory` | The store package installs its factory from its own `init()`; the runtime package never imports it |
| The seam the commands ask | `storeseam_operations.go`: `ImageOperations` | A store MAY also be an `ImageOperations`; an image command refuses, naming why, when it is not |
| The store a daemon builds | `engineStoreFactory`, `storeseam_engine.go` and `storeseam_zetest.go` | A shipped build builds the store package's store. A `zetest` daemon builds the runtime's fixture store when `ze.test.container.store` is set, and the real store otherwise |
| The commands | `image_cmd.go`, `yang/ze-container-cmd.yang` | Ten command nodes, each proxied to the plugin's process |
| The doctor codes | `doctor_store.go` | Eleven codes, registered from the plugin's package, six of them as report-bus event codes |
| The state keys | `pkg/zefs/keys.go` | Three patterns under `meta/container/`, granted to the plugin |
| The data directory | `internal/core/paths/paths.go` | `ze.data.dir` is registered and honored. The mapping from the binary's location is a stub that answers no directory |

<!-- source: internal/plugins/container/imagestore/store.go -- Store, newStore, notImplemented -->
<!-- source: internal/plugins/container/imagestore/register.go -- init -->
<!-- source: internal/plugins/container/storeseam_operations.go -- ImageOperations, ImageState, VolumeState -->
<!-- source: internal/plugins/container/storeseam_engine.go -- engineStoreFactory -->
<!-- source: internal/plugins/container/storeseam_zetest.go -- engineStoreFactory -->
<!-- source: internal/core/paths/paths.go -- DataDirFromBinary, DefaultDataDir -->

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
| `doctor-container-store-unwritable` | The store folder is absent, cannot be written, or is open to group or others | The runtime check `container-store`, not declared yet |
| `doctor-container-store-full` | The data filesystem is below the store's reserve | The same |
| `doctor-container-kernel-quota-unavailable`, `doctor-container-kernel-quota-unknown` | The data directory's filesystem cannot cap a layer or a volume, or Ze could not tell | The kernel capability `container-quota`, not enrolled yet |
| `doctor-container-storage-full` | A writable layer or a volume is at its cap | The runtime check `container-storage-usage`, not declared yet |
| `doctor-container-registry-unreachable` | A fetch could not reach the registry | An event of the fetch |
| `doctor-container-registry-auth-refused` | The registry refused the credentials or the access | An event of the fetch |
| `doctor-container-image-not-found` | The registry holds no such image or tag | An event of the fetch |
| `doctor-container-image-digest-mismatch` | A manifest, a configuration or a layer did not match its digest or size | An event of the fetch |
| `doctor-container-image-refused` | An image was refused: an unsafe entry, a bound, a type, no entry for this platform | An event of the fetch |
| `doctor-container-fetch-no-space` | A fetch was refused for lack of space | An event of the fetch |

<!-- source: internal/plugins/container/doctor_store.go -- storeDiagnosticCodes, registerStoreEventCodes -->
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
