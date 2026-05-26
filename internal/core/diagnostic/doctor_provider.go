// Design: docs/features/ai-first.md -- doctor provider for show command

package diagnostic

import "sync"

var doctorProvider struct {
	sync.RWMutex
	fn func(configPath string) []Diagnostic
}

// RegisterDoctorProvider sets the function that runs doctor checks.
// Called by the doctor package during init() so the show command can
// invoke doctor without importing cmd/ze/doctor.
func RegisterDoctorProvider(fn func(configPath string) []Diagnostic) {
	doctorProvider.Lock()
	doctorProvider.fn = fn
	doctorProvider.Unlock()
}

// RunDoctorChecks invokes the registered doctor provider.
// Returns nil if no provider is registered.
func RunDoctorChecks(configPath string) []Diagnostic {
	doctorProvider.RLock()
	fn := doctorProvider.fn
	doctorProvider.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(configPath)
}
