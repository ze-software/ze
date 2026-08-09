# PXE staging and the iPXE chainload

`ze install remote --kernel --initrd` stages everything a PXE install needs and
serves the boot chain. Before it existed, an operator cloned iPXE, wrote an
embed script, compiled iPXE, placed kernel, initrd, and bootloader files by
hand, and wrote a boot script.

<!-- source: internal/plugins/provision/staging.go -- stageArtifacts, validateStaging, pxeDirs -->
<!-- source: internal/plugins/dhcpserver/handler.go -- isIPXE, appendPXEOptions, the chainload branch -->
<!-- source: internal/plugins/imageserver/handler.go -- serveBootIPXE, latestImage -->

## The two-stage path

The firmware ROM and iPXE are different clients and get different answers:

| Client | Answer |
|--------|--------|
| PXE firmware ROM | the TFTP boot file, which is the iPXE binary |
| iPXE | the HTTP boot-script URL |

iPXE is detected from the DHCP user-class option by **prefix**, not by an exact
match, because iPXE builds append a version string. dnsmasq and ISC DHCP use the
same prefix test.

## Decisions

- **The boot script is a dynamic HTTP endpoint**, not a file written at staging
  time. It is always right, because it reads the live address and port. A static
  file goes stale the moment the server address changes.
- **A static `boot.ipxe` in the boot directory overrides the dynamic one**, so
  an operator can customize the script without changing Ze.
- **iPXE binaries are bundled in the repository with update instructions**,
  rather than downloaded at run time. That keeps offline installs working and
  removes a network trust question.
- **An empty boot-script URL stays backward compatible.** A deployment with a
  custom-embedded iPXE keeps working, so the field is not made mandatory when
  PXE is enabled.
- **Staging directories travel in a config struct, not in package-level
  overrides.** Package-level overrides race when tests run in parallel.
- The boot script picks the lexicographically last image file, which is the
  latest by the timestamp naming convention.

## Trap

**The QEMU test booted the kernel directly and skipped the iPXE chain
entirely.** The missing kernel command line in the real PXE path was therefore
never exercised, and physical machines failed at install time with the server
address unset. A provisioning test that bypasses the bootloader proves the
kernel boots, and proves nothing about the chain that is supposed to launch it.

The mux constructor takes the bind address, so any future caller has to supply
it. That signature is what makes the dynamic script correct.

## Related

- `dhcp-server.md` for the option handling
- `image-server.md` for the endpoint that serves the script
- `tftp-server.md` for the first stage
- `../appliance/iso-installer.md` for the offline alternative to PXE
