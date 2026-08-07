---
kind: note
level:
stage:
---
The website plugin catalog in `../gh-pages/docs/features/plugins/` is
generated from `registry.Registration` fields via
`../gh-pages/tools/extract-plugin-registry.py` and
`../gh-pages/tools/render-plugin-catalog.py`. When adding or changing a
plugin, treat `Name`, `Description`, `ConfigRoots`, `Dependencies`,
`OptionalDependencies`, and `YANG` as public catalog data.
