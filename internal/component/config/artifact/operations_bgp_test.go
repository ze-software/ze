//go:build ze_bgp

// Design: docs/architecture/web-interface.md -- native resolved artifact comparisons.
package artifact_test

// Match the shipped composition's BGP resolver, not only its command models.
import _ "github.com/ze-software/ze/internal/component/bgp/config"
