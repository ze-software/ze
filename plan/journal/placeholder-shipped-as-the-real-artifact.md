# placeholder-shipped-as-the-real-artifact

A file in the tree stands where a real binary or data blob belongs, and every
consumer treats it as the real thing. The wiring is correct, the path resolves,
and the copy succeeds, so nothing on the code path can tell. Only running the
product against real hardware or a real client reveals it, which is the one test
the placeholder makes it easy never to write.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-28 | - | `tools/ipxe-binaries/ipxe.efi` and `ipxe.pxe`, served by `internal/plugins/provision/staging.go` | Both files are 28 bytes of ASCII: `iPXE UEFI binary placeholder`. `ze install remote` copies them to `build/pxe/tftp/` when that directory holds none, so a PXE client chainloads a text file instead of a bootloader and the boot fails at the firmware, before any Ze code runs. The tree states the gap in two places and neither is a gate: the README's Provenance section calls them stock builds from ipxe.org, and its Updating section then asks for the commit hash to be filled in "after replacing placeholders with real builds". The retired `ze-pxe-build` target cloned github.com/ipxe/ipxe and ran its Makefile, so the tree once had a producer; the recorded retirement is accurate that Ze builds no iPXE binary now. Found while checking what the Make retirement lost, so the placeholders predate it: both files are dated 2026-06-04 | NOT FIXED, and not a regression of the Make retirement. Fixing it needs a decision the owner owns, because the two candidate answers differ in what the repository carries: vendor real stock binaries with their iPXE commit hash recorded, or restore a producer that builds them from the upstream clone. Until then the honest gate is a check that refuses a file under `tools/ipxe-binaries/` too small to be a bootloader, so the placeholder cannot reach an operator silently |
