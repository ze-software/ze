// Design: docs/architecture/l2tp/l2tp-9-observer.md -- real server fixture composition

package l2tp

// The package's cleanup-doctor tests already link the local doctor command.
// Real observer event buses construct pluginserver.Server, which MUST validate
// that command's declared show doctor daemon path against its owning model.
import _ "github.com/ze-software/ze/internal/component/doctor/yang"
