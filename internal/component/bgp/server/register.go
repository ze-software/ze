// Design: docs/architecture/core-design.md -- BGP codec RPC handler registration

package server

import "github.com/ze-software/ze/internal/component/plugin/registry"

func init() {
	registry.AddRPCHandlers(codecRPCHandlers())
}
