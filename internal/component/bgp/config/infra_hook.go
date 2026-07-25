// Design: docs/architecture/config/syntax.md -- infrastructure setup hook
// Related: loader_create.go -- CreateReactorFromTree calls the hook

package bgpconfig

import (
	"github.com/ze-software/ze/internal/component/config/infra"
)

// LoginWarning is the banner-warning shape produced by collectPrefixWarnings.
// The type itself is owned by the always-on infra package (the hub renders it
// without knowing BGP exists); this alias keeps the BGP-side producers reading
// naturally.
type LoginWarning = infra.LoginWarning

// The infrastructure setup contract -- SSHExtractedConfig, HookParams, Hook,
// SetHook -- lives in the always-on internal/component/config/infra package.
// It is not BGP-specific: the hub registers the hook before any engine starts
// and the no-bgp{} startup path uses the same extraction, so it must survive
// //go:build ze_bgp compiling this package out entirely. The engine's only role
// is to call infra.Run once its reactor exists (loader_create.go).
