# Device-side configuration: canonical source and legacy boot inbox

Stored startup reads the canonical active version through `ReadConfigSource`.
`ReadActiveConfig` follows the active pointer and uses the legacy active entry
only when no pointer exists. An unresolved pointer is an error. Live SSH uploads
use the source-aware commit transaction described in `remote-operations.md`.
<!-- source: internal/component/config/storage/source.go -- ReadConfigSource -->
<!-- source: internal/component/config/storage/pointer.go -- ReadActiveConfig -->
<!-- source: cmd/ze/hub/config_upload.go -- configMutationOwner.Upload -->

## Legacy boot inbox

Before stored startup enters the hub, `checkPushedConfig` reads the external
inbox `/perm/ze/config-pushed.conf`. A valid file replaces the legacy active
entry. Invalid input is logged and the inbox file is deleted. This path does
not publish a canonical version or provide live SSH upload.
<!-- source: cmd/ze/ze_core_start.go -- cmdStart -->
<!-- source: cmd/ze/pushed_config.go -- checkPushedConfig -->

## Decisions

| Decision | Reason |
|----------|--------|
| The inbox lives at `/perm/ze/config-pushed.conf` | An external operator or provisioning system supplies the file |
| Validation goes through the normal config load | the same parse and the same YANG validation as any other config. There is no weaker path for a pushed file |
| The pushed-config check runs on every boot, not only on an appliance | the `ENOENT` fast path costs less than a guard that asks whether this is an appliance |
| The startup hash is stored at `meta/config/active-hash` | It hashes the legacy active entry, rather than following the canonical pointer |
| The monitor writes the previous legacy entry, then the seed fallback | It supplies legacy recovery bytes without publishing a canonical version |
| The boot monitor lasts 30 seconds | A BGP peer-close callback during that interval triggers its recovery write |

<!-- source: cmd/ze/pushed_config.go -- checkPushedConfig, writeConfigActiveHash -->
<!-- source: cmd/ze/health_revert.go -- HealthRevert.Start, HealthRevert.OnPeerClosed, HealthRevert.revert -->

## Constraint the code does not state

`HealthRevert` writes through the shared store. Its recovery write changes only
the legacy active entry and does not reload the running daemon. If a canonical
active pointer exists, that pointer still selects the previous accepted version.
<!-- source: cmd/ze/health_revert.go -- HealthRevert.revert -->
<!-- source: internal/component/config/storage/pointer.go -- ReadActiveConfig -->

The revert timer is `time.AfterFunc` under a mutex. `OnPeerClosed` stops the
timer and writes recovery bytes. Timer expiry records the legacy entry's hash
at `meta/config/last-known-good-pushed`. A live SSH upload starts no such timer.
<!-- source: cmd/ze/health_revert.go -- HealthRevert.OnPeerClosed, HealthRevert.onHealthy -->
<!-- source: cmd/ze/hub/config_upload.go -- configMutationOwner.Upload -->

## Related

- `remote-operations.md` for the bastion side of a config push
- `../core-design.md` section 21 for configuration source selection
