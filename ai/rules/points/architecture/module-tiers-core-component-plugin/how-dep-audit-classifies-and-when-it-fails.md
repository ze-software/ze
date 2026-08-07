---
kind: directive
level:
stage:
---
- parses `pluginDirs` from `scripts/codegen/plugin_imports.go` to exclude nested sub-plugin namespaces (so `bgp/plugins/*` are never flagged);
- treats generated `all.go` files, gated `all_<tag>.go` files, `cmd/ze` dispatch/import companions, and `cmd/ze/setup_features_*.go` as registration importers, not functional dependencies;
- fails (exit 2) on any **new** misplaced engine, naming the dir and its required tier, pointing here;
- fails on a **stale** engine baseline entry (one no longer misplaced), forcing cleanup;
- fails (exit 2) on any illegal, stale, or missing row in `scripts/dev/tier_non_engine_categories.txt`;
- fails (exit 2) if a `DISABLEABLE` feature is imported by always-on (untagged, non-test) code, naming the file and the build tag it needs.
