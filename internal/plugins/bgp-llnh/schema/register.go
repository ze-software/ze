package schema

import (
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang/registry"
)

func init() {
	registry.RegisterModule("ze-link-local-nexthop.yang", ZeLinkLocalNexthopYANG)
}
