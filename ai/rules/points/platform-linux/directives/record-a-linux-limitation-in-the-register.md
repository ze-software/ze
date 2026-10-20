---
kind: directive
level: MUST
stage:
---
- A Linux kernel behavior, or a library behavior on the path to the kernel, that stops Ze implementing a feature or an RFC requirement as asked MUST be recorded as an entry in `docs/architecture/linux-limitations.md`, with the source file and function read, the quoted line, and the kernel release or module version it was read at. It MUST NOT live only in an `rfc/short/` gap note, a guide, a code comment or an error message: those MUST name the entry id instead of restating it. An entry whose source you could not read MUST be marked `unverified` and MUST say what would settle it.
- A bump of `internal/appliance/kernel.version`, or of a module an entry names, MUST re-read every affected entry at the new version and update its `Checked` line. `TestLinuxLimitsCheckedAtShippedVersions` (`internal/appliance/linux_limits_test.go`) goes red until that is done; the fix is the re-check, never a version edit without it.
