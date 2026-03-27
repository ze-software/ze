package filter

import (
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func init() {
	_ = registry.Register(registry.Registration{
		Name:          "loop",
		Description:   "Route loop detection (RFC 4271 S9, RFC 4456 S8)",
		RFCs:          []string{"4271", "4456"},
		IngressFilter: LoopIngress,
	})
}
