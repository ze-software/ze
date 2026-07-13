// Sentinel module: marks tmp/ as a separate (nested) module so that
// `go list ./...` and `go test ./...` skip everything under tmp/.
//
// QEMU and Docker test runs store Go build/module caches under tmp/
// (e.g. tmp/qemu/gomodcache, tmp/linux-gomodcache). Without this sentinel,
// `go list ./...` descends into those caches and fails with
// "directory ... outside main module or its selected dependencies", which
// breaks `make ze-verify`.
//
// This file is committed (tracked) so it is always present, including on a
// fresh checkout before any QEMU/Docker run; everything else under tmp/ is
// .gitignored. `make clean` wipes tmp/ and then recreates this file
// byte-identically, and qemu-run.py's ensure_tmp_sentinel() recreates it
// too, so clearing tmp/ never shows up as a git change. Keep this content
// in sync with TMP_SENTINEL in scripts/evidence/qemu-run.py.
module ze-tmp-scratch

go 1.25
