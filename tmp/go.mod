// Sentinel module: marks tmp/ as a separate (nested) module so that
// `go list ./...` and `go test ./...` skip everything under tmp/.
//
// QEMU and Docker test runs store Go build/module caches under tmp/
// (e.g. tmp/qemu/gomodcache, tmp/linux-gomodcache). Without this sentinel,
// `go list ./...` descends into those caches and fails with
// "directory ... outside main module or its selected dependencies", which
// breaks `make ze-verify`. tmp/ is .gitignored, so this file is local-only;
// scripts/evidence/qemu-run.py recreates it on each run.
module ze-tmp-scratch

go 1.25
