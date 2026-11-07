package l2tp

// Blank import triggers the schema package's init(), which registers
// `ze-l2tp-conf.yang` with the config/yang module registry.
import (
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/health"

	_ "github.com/ze-software/ze/internal/component/l2tp/yang"
)

func init() {
	// UDPListener.Start (listener.go) binds every IPv6 listener with
	// IPV6_V6ONLY, so `0.0.0.0` and `::` on one port both bind. Conflict
	// validation must know it, or it refuses the pair as a dual-stack overlap.
	config.RegisterListenerV6Only("l2tp")
	if err := config.RegisterTreeValidator("l2tp-mru-policy", validateMRUPolicy); err != nil {
		panic("BUG: registering l2tp MRU policy: " + err.Error())
	}
	if err := config.RegisterTreeValidator("l2tp-auth-policy", validateAuthPolicy); err != nil {
		panic("BUG: registering l2tp authentication policy: " + err.Error())
	}
	if err := config.RegisterTreeValidator("l2tp-parameters", validateParameters); err != nil {
		panic("BUG: registering l2tp parameters: " + err.Error())
	}
	health.Register("l2tp", checkHealth)
	// Register the redistribute source at init so `import l2tp` resolves during
	// `ze config validate`, which imports plugins but does not start their engines.
	// Subsystem.Start also calls this; sync.Once makes the second call a no-op.
	registerL2TPSources()
}
