---
kind: table
level:
stage:
---
| Consumer | Role | Mechanism |
|----------|------|-----------|
| `Makefile` `ZE_FEATURES` | default-on tags for `ze` / `ze-appliance` | derives: `$(shell awk ...)` |
| `internal/test/runner` `TestBuildTags` | tags for the functional-test `ze` | derives: reads the file |
| `scripts/codegen/plugin_imports.go` `featureTags` | gates `<pkg>` and `<pkg>/yang` into `all_<tag>.go` | derives: `loadFeatureTags` |
| `scripts/dev/dep_audit.py` `DISABLEABLE` | no always-on import of `<pkg>` | derives: `load_feature_gates` |
| `scripts/dev/stress-repro.py` `race_tags` | full-feature race build | derives: `_feature_gate_tags()` |
| `.golangci.yml` `build-tags` | lint the feature-on build | **generated** by `feature_tags.go` |
| `gokrazy/ze/config.json` `GoBuildTags` | appliance image build tags | **generated** by `feature_tags.go` |
| `docs/guide/quickstart.md` `go install` cmd | install without cloning the repo | **generated** by `feature_tags.go` |
