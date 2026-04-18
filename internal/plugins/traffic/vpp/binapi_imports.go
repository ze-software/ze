// Design: plan/spec-fw-7-traffic-vpp.md -- Phase 0 vendor pinning

// Anchor file that keeps the GoVPP binapi packages in the module's import
// graph on EVERY platform, so `go mod vendor` retains them under
// `vendor/` regardless of how the rest of the package evolves.
//
// Current scope: policer and policer_types (HTB/TBF policers + the enums
// they reference). An earlier iteration of the backend also pinned `qos`
// and `classify`; those paths were retired after review found the
// surrounding VPP pipelines were incomplete (see verify.go and
// `plan/deferrals.md` for the destination specs). The next `go mod
// vendor` run dropped qos and classify from vendor/.
//
// Why a separate anchor file (rather than relying on other files'
// imports): `backend_linux.go` carries `//go:build linux`, so on
// non-Linux platforms its imports are invisible to `go mod vendor`.
// `translate.go` currently has no build tag and also imports both
// packages, which would keep them vendored today -- but that coupling
// is fragile. A future refactor that moves `policerFromClass` into
// `backend_linux.go` (for example as part of spec-fw-7b-backend-
// hardening) would silently drop these packages from vendor on the
// next re-vendor from a non-Linux developer machine. The
// unconditional blank imports here are belt-and-suspenders protection
// that the scaffolding cannot be undone by an unrelated change
// elsewhere.

package trafficvpp

import (
	_ "go.fd.io/govpp/binapi/policer"
	_ "go.fd.io/govpp/binapi/policer_types"
)
