// Design: docs/architecture/cli/plugin-modes.md -- native installation owners.

package install

// NativeService is the service owner's contribution to native installation.
// Preflight MUST finish before Stop or binary replacement. Stop MUST finish
// before Install; Uninstall MUST stop the daemon before removing its service.
// Callers MUST release storage maintenance locks before Install takes ownership.
type NativeService interface {
	Available() (bool, error)
	Preflight(binary, configDir string) error
	Stop(binary, configDir string) (StoppedService, error)
	Uninstall(binary, configDir string) int
}

// StoppedService retains the prior service state across a binary replacement.
// Install MUST preserve a previously inactive unit. Restore MUST resume only a
// service this stop operation found active, after the prior binary is restored.
type StoppedService interface {
	Install() int
	Restore() error
}

var nativeOwners struct {
	bootstrap func(string) int
	service   NativeService
}

// RegisterBootstrap declares the initializer. Registration MUST precede lookup.
func RegisterBootstrap(handler func(string) int) {
	if handler == nil {
		panic("BUG: nil native bootstrap")
	}
	if nativeOwners.bootstrap != nil {
		panic("BUG: duplicate native bootstrap")
	}
	nativeOwners.bootstrap = handler
}

// Bootstrap answers the registered initializer, or false when it is compiled out.
func Bootstrap() (func(string) int, bool) {
	return nativeOwners.bootstrap, nativeOwners.bootstrap != nil
}

// RegisterService declares the native service owner before command dispatch.
func RegisterService(service NativeService) {
	if service == nil {
		panic("BUG: nil native service")
	}
	if nativeOwners.service != nil {
		panic("BUG: duplicate native service")
	}
	nativeOwners.service = service
}

// Service answers the registered service owner, or false without one linked.
func Service() (NativeService, bool) {
	return nativeOwners.service, nativeOwners.service != nil
}
