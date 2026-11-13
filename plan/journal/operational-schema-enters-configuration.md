# Operational schema enters configuration

Operational declarations share a module with configuration, and the projection
imports both as editable schema. A module's name is mistaken for the role of
every node it contains.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-11-12 | spec-config-enum-refusal-proposes-valid-values | `schemaFromResolved`, `yangToNode` | Commit 857da9e798c added CoS/shaper `config false` request roots to `-conf` modules. Projection exposes `/show/request/`; handler goldens `post-cli-mode` and `get-show-finder` fail. Mutation acceptance was not executed. | open: separate operational declarations from editable schema; preserve hidden transaction commands and AS112 listener anchors. |
