# Appliance architecture

How a Ze appliance image is built, installed, and updated. Code lives in
`internal/appliance/`, `internal/install/disk/`, and `cmd/ze-installer/`.

| Document | Subject |
|----------|---------|
| `builder.md` | `ze appliance` build-host tooling, secrets at rest, the passphrase agent |
| `command-provider.md` | how the whole surface registers and stays removable |
| `build-artifacts.md` | installer kernel and initrd resolution, cache, doctor checks |
| `kernel-profiles.md` | the open kernel-profile registry and the verified guarantee |
| `installer-initrd.md` | the busybox-free PID 1, and the host-versus-target binary rule |
| `on-device-installer.md` | `ze install disk`, build-side verification, the un-brick path |
| `iso-installer.md` | the bootable ISO transport |
| `gokrazy-build-pins.md` | why a derived parent instance must carry the builddir |
| `ota-push.md` | pushing an image to a running device |
| `self-update.md` | a device pulling its own binary |
| `remote-operations.md` | fleet push, config preview, parallel operations |
| `disaster-recovery.md` | encrypted export and import of a bastion |
| `device-config.md` | config priority, last-known-good, auto-revert |

Start with `builder.md` for the build host and `installer-initrd.md` for the
install path.
