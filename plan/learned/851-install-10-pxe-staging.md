# 851 -- install-10-pxe-staging

## Context

PXE bare-metal installation required operators to manually clone iPXE from GitHub, write a custom embed script with ze.server/ze.image/ip=dhcp, compile iPXE from source, stage kernel/initrd/bootloader files in the correct directories, and write their own iPXE boot script. This was a 110-line shell script for what should be a single command. The QEMU test used direct kernel boot and bypassed the iPXE chain entirely, so the missing kernel cmdline in the real PXE boot path was never caught. Physical machines failed with "ze.server not set on kernel cmdline" because iPXE loaded the kernel without passing the required parameters.

## Decisions

- Dynamic HTTP endpoint for boot.ipxe over static file generation at staging time, because dynamic is always correct (IP/port from live config) and static can go stale if the server IP changes
- iPXE detection via option 77 user-class prefix "iPXE" over exact match, because iPXE builds may append version strings; dnsmasq and ISC DHCP use the same prefix approach
- Static boot.ipxe in boot dir overrides dynamic generation, to allow operators to customize the boot script without changing ze code
- Bundle iPXE binaries in repo with update instructions over auto-download from ipxe.org, for offline availability and no network trust question
- Backward compatible when boot-script-url is empty (existing deployments with custom-embedded iPXE keep working) over requiring boot-script-url when PXE enabled
- Staging directories passed via stagingConfig struct fields over package-level global overrides, because global overrides race in parallel tests
- boot.ipxe selects lexicographically last .img file as ze.image (latest by timestamp convention) over requiring explicit configuration

## Consequences

- `ze install remote --kernel --initrd` is the complete PXE provisioning command; no manual staging or iPXE compilation needed
- DHCP handler now has a two-stage PXE path: firmware ROM gets TFTP bootfile, iPXE gets HTTP boot script URL
- Image server has its first dynamic endpoint; all previous endpoints were static file serving
- newMux signature changed from (cfg, zefsPath) to (cfg, zefsPath, serverAddr); any future callers must provide the bind IP
- iPXE binaries in tools/ipxe-binaries/ are placeholders; must be replaced with real builds before first production PXE install

## Gotchas

- The linter hooks block any edit that leaves the package in an incomplete compilation state; when changing a function signature, all callers (including test files) must be fixed in rapid succession or the auto-linter rejects every intermediate edit
- `fmt.Sprintf` is banned in non-test Go files; use textbuf.Buffer for string building in production code
- Package-level var overrides for test isolation are not safe when tests run in parallel; pass configuration through struct fields instead

## Files

- `internal/plugins/dhcpserver/handler.go` -- isIPXE, optUserClass, appendPXEOptions chainload branch
- `internal/plugins/dhcpserver/config.go` -- BootScriptURL field + parsing
- `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` -- boot-script-url leaf
- `internal/plugins/imageserver/handler.go` -- serveBootIPXE, latestImage, serverAddr field
- `internal/plugins/imageserver/register.go` -- pass bindIP to newMux
- `internal/plugins/provision/main.go` -- --kernel, --initrd flags, staging, boot-script-url config
- `internal/plugins/provision/staging.go` -- stageArtifacts, validateStaging, copyFileIfRegular (new)
- `internal/plugins/provision/staging_test.go` -- 5 staging tests (new)
- `tools/ipxe-binaries/` -- stock iPXE binaries + README (new)
- `test/install/pxe-chainload.ci` -- functional test (new)
- `docs/guide/ze-install.md` -- PXE workflow rewrite
