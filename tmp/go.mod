// Sentinel module: marks a REAL tmp/ as a nested module so `go list ./...` and
// `go test ./...` skip the Go/QEMU caches under it (they hold foreign go.mod files
// that would otherwise fail with "directory ... outside main module").
//
// Committed so it is present on a fresh checkout. scripts/dev/ensure-links.py recreates
// it whenever tmp/ is a real directory; after the opt-in `make ze-migrate-scratch`, tmp/
// is a symlink that `go list` skips without any sentinel, so this file is not needed there.
// Keep this content in sync with SENTINEL in scripts/dev/ensure-links.py.
// See plan/spec-relocate-scratch-and-cache.md.
module ze-tmp-scratch

go 1.25
