# Provisioning services

The network services that install a Ze device over the network. Each is a
plugin under `internal/plugins/`, usable from config on any Ze device.

| Document | Subject |
|----------|---------|
| `dhcp-server.md` | leases, pools, and the PXE options |
| `tftp-server.md` | the read-only first stage of a PXE boot |
| `image-server.md` | HTTP images, boot files, and the ZeFS database |
| `pxe-staging.md` | artifact staging and the iPXE chainload |

`pxe-staging.md` describes the chain the other three implement. The offline
alternative is `../appliance/iso-installer.md`.
