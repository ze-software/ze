// Design: docs/guide/pppoe.md -- configuration follows installed session providers

package pppoe

// The unit package must link the same native provider as the ze_l2tp
// composition root. Registration opens no socket; offline validation is
// about the target build's provider, not this host's kernel capabilities.
import _ "github.com/ze-software/ze/internal/component/l2tp/plugins/pppoe-kernel"
