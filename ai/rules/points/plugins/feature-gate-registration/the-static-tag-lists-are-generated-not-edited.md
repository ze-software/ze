---
kind: directive
level:
stage:
---
**No consumer is hand-maintained.** The three static files that cannot read the manifest at runtime (`.golangci.yml` `build-tags`, `gokrazy/ze/config.json` `GoBuildTags`, `docs/guide/quickstart.md`'s `go install -tags '...'` command) are GENERATED from it by `scripts/codegen/feature_tags.go` (run by `make generate`, surgical byte-stable edits). Do NOT hand-edit their tag lists: add the gate to `feature-gates.txt` and run `make generate`. Three gates catch drift: the `scripts/codegen` unit test `feature_tags.go --check`, `dep_audit.py --check` (golangci), and `internal/appliance` `TestGokrazyConfigMatchesApplianceBuildTags` (gokrazy).
