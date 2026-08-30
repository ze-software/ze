---
kind: directive
level: MUST
stage:
---
- **Copying a plugin folder in and running `./le repository generate` MUST make its commands live, and deleting the folder and running it again MUST make them vanish.** No manual wiring. This is the "delete the folder" proximity test applied to the whole user-facing surface. The directory layout, the codegen that discovers it, and the two folder tests are `docs/architecture/command-ownership.md`.
