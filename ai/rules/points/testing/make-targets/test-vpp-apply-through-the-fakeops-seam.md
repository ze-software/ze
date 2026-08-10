---
kind: directive
level: MUST
stage:
---
**fakeOps pattern:** VPP backends MUST use a `vppOps` interface seam so the Apply
pipeline can be tested with a scripted fake without a running VPP daemon. The
`apply_test.go` files are `//go:build linux` (they import linux-only binapi
types) but do NOT need real VPP. They run in QEMU alongside the integration
tests. See `internal/plugins/traffic/vpp/apply_test.go` for the reference
pattern.
