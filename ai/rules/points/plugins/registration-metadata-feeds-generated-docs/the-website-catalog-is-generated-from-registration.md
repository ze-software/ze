---
kind: directive
level: MUST
stage:
---
- **`Name`, `Description`, `ConfigRoots`, `Dependencies`, `OptionalDependencies` and `YANG` MUST be treated as public catalog data.** `./le site build` generates the website plugin catalog from them, so a package move or a config-root change changes what is published. The generation path is `docs/architecture/plugin/plugin-system.md`, "Registration metadata feeds the website catalog".
