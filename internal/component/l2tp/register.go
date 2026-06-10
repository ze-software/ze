package l2tp

// Blank import triggers the schema package's init(), which registers
// `ze-l2tp-conf.yang` with the config/yang module registry.
import (
	"codeberg.org/thomas-mangin/ze/internal/core/health"

	_ "codeberg.org/thomas-mangin/ze/internal/component/l2tp/yang"
)

func init() {
	health.Register("l2tp", checkHealth)
}
