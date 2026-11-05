package l2tp

// Blank import triggers the schema package's init(), which registers
// `ze-l2tp-conf.yang` with the config/yang module registry.
import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/health"

	_ "github.com/ze-software/ze/internal/component/l2tp/yang"
)

func init() {
	if err := config.RegisterTreeValidator("l2tp-mru-policy", validateMRUPolicy); err != nil {
		panic("BUG: registering l2tp MRU policy: " + err.Error())
	}
	if err := config.RegisterTreeValidator("l2tp-auth-policy", validateAuthPolicy); err != nil {
		panic("BUG: registering l2tp authentication policy: " + err.Error())
	}
	health.Register("l2tp", checkHealth)
	// Register the redistribute source at init so `import l2tp` resolves during
	// `ze config validate`, which imports plugins but does not start their engines.
	// Subsystem.Start also calls this; sync.Once makes the second call a no-op.
	registerL2TPSources()
}
