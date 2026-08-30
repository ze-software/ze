---
kind: directive
level: SHOULD
stage:
---
**A new protocol SHOULD follow the subpackage skeleton, and BFD SHOULD be treated as the reference layout:** `packet`, `engine`, `session`, `transport`, `auth`, `cmd`, `api`, `yang`. A protocol at root-package-plus-`yang` size needs none of it. The skeleton is ADVISORY for existing code: no moves, no renames, no gate, and `./le protocol-skeleton report` always exits 0. The modules and how each existing protocol maps to them are in `docs/architecture/protocol-skeleton.md`.
<!-- source: internal/component/bfd -- subpackage layout -->
