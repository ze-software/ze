---
kind: directive
level: MUST
stage:
---
1. **Extract first.** Search for always-on importers of the feature package (`<module>/internal/component/<x>`). Move every non-lifecycle helper they use to an always-on `internal/core/*` leaf. Re-check until only gated construction remains.
2. **Pick the shape** (see below): construction registry, or a seam.
3. **Add lines** to `feature-gates.txt` for every owned package that MUST vanish: the main package (`ze_<x> internal/component/<x>`) plus sidecars such as command-schema packages under `internal/plugins/<x>-cmd`.
4. **Create the gated files** for your shape (`service_<x>.go` + `register_<x>.go`, or an `*_infra.go` seam + gated registration). All carry `//go:build ze_<x>`. Feature-only helpers live INSIDE a gated file, or a no-feature build flags them U1000-unused.
5. `make generate`. This emits `all_ze_<x>.go` (plugin_imports) AND regenerates the three static tag lists from the manifest (`feature_tags.go`: `.golangci.yml` `build-tags`, `gokrazy/ze/config.json` `GoBuildTags`, `docs/guide/quickstart.md`). Those files' tag lists MUST NOT be hand-edited. Then `make ze-precommit-verify-changed`.
6. Write present/absent build-tag tests: `cmd/ze/hub/build_tag_<x>_present_test.go` (`//go:build ze_<x>`) and `_absent_test.go` (`//go:build !ze_<x>`); an absent test asserts via `go tool nm` that zero feature symbols are linked.
