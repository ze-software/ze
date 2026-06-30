# iPXE Binaries

Stock iPXE bootloader binaries for PXE provisioning. `ze install remote`
copies these to `build/pxe/tftp/` when not already present.

## Files

| File | Architecture | Purpose |
|------|-------------|---------|
| `ipxe.pxe` | x86 BIOS | Legacy BIOS PXE chainloader |
| `ipxe.efi` | x86_64 UEFI | UEFI PXE chainloader |

## Provenance

These are stock iPXE builds from https://ipxe.org with no embedded script.
The DHCP server directs iPXE to fetch `boot.ipxe` over HTTP after chainloading.

## Building from Source

To build fresh binaries from the iPXE repository:

    git clone https://github.com/ipxe/ipxe.git
    cd ipxe/src
    make bin/ipxe.pxe
    make bin-x86_64-efi/ipxe.efi

Copy the resulting files here. No custom embed script is needed because ze's
image server generates `boot.ipxe` dynamically with the correct kernel
command line.

## Updating

Replace the binaries and note the iPXE commit hash in this section:

- iPXE commit: (fill in after replacing placeholders with real builds)
- Build date: (fill in)

## ARM64

ARM64 EFI (`snponly.efi` or `ipxe.efi` for aarch64) is not bundled.
Operators targeting ARM64 PXE boot must provide their own binary and
place it in `build/pxe/tftp/`.
