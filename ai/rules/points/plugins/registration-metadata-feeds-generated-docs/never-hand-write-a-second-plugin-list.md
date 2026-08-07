---
kind: note
level:
stage:
---
Do not create a second hand-written plugin list in docs or website content.
If the catalog needs local prose or display metadata, add a `PLUGIN.md` next
to that plugin's `register.go` with front matter (`area`, `summary`, `tags`)
and body Markdown. If it needs a new machine fact, add structured metadata to
the registry or extractor, then render the website from that data. Catalog
grouping is derived from `ConfigRoots` and source path layout, so package moves
and config-root changes affect the generated site.
