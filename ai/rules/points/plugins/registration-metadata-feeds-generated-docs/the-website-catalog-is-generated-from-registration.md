---
kind: note
level:
stage:
---
The website plugin catalog in `../gh-pages/docs/features/plugins/` is
generated from `registry.Registration` fields by `./le site build`.
`internal/le/sitebuild` owns the website catalog producer. When adding or
changing a
plugin, treat `Name`, `Description`, `ConfigRoots`, `Dependencies`,
`OptionalDependencies`, and `YANG` as public catalog data.
