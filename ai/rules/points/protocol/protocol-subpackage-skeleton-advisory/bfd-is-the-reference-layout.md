---
kind: directive
level: SHOULD
stage:
---
- **A new protocol SHOULD follow the subpackage skeleton, and BFD SHOULD be treated as the reference layout:** `packet` / `engine` / `session` / `transport` / `auth` / `cmd` / `api` / `yang`.
- **A protocol at root-package-plus-`yang` size needs none of it.** The skeleton applies once a protocol grows subpackages.
- **Existing code MAY adopt it opportunistically. No moves, no renames, and no gate.** `./le protocol-skeleton report` is a lens and always exits 0.
- The modules, the five classes, and how each existing protocol maps are in `docs/architecture/protocol-skeleton.md`.
<!-- source: internal/component/bfd -- subpackage layout -->
