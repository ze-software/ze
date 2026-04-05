// Design: docs/architecture/core-design.md -- BGP codec RPC handler registration

package server

import "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"

func init() {
	registry.AddRPCHandlers(CodecRPCHandlers())
}
