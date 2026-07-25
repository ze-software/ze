// Design: docs/features/ai-first.md -- plugin doctor check registration
// Related: registry.go -- Registration struct (DoctorChecks field)

package registry

import (
	"fmt"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// DoctorCheckContext is the runtime context for plugin doctor checks.
// Tree is *config.Tree and Platform is *host.PlatformInfo, typed as any
// because this leaf package cannot import those types without cycles.
type DoctorCheckContext struct {
	Tree      any
	ConfigDir string
	Platform  any
}

// DoctorCheckFunc is the function signature for a plugin doctor check.
type DoctorCheckFunc func(DoctorCheckContext) []rpc.DoctorCheckDiagnostic

// DoctorCheckDef declares a doctor readiness check provided by a plugin.
type DoctorCheckDef struct {
	Name         string
	Phase        rpc.DoctorCheckPhase
	Order        int
	Dependencies []string
	Platforms    []string
	Codes        []string
	Check        DoctorCheckFunc
}

// PluginDoctorCheck pairs a plugin name with its doctor check definition.
type PluginDoctorCheck struct {
	PluginName string
	DoctorCheckDef
}

// PluginDoctorChecks returns all doctor checks declared by registered plugins.
func PluginDoctorChecks() []PluginDoctorCheck {
	mu.RLock()
	defer mu.RUnlock()

	var result []PluginDoctorCheck
	for _, reg := range plugins {
		for _, dc := range reg.DoctorChecks {
			result = append(result, PluginDoctorCheck{
				PluginName:     reg.Name,
				DoctorCheckDef: dc,
			})
		}
	}
	return result
}

func validateDoctorCheckDef(pluginName string, idx int, dc DoctorCheckDef) error {
	var tb textbuf.Buffer
	p := tb.Str("plugin ").Quoted(pluginName).Str(" doctor check [").Int(int64(idx)).Byte(']').String()
	if dc.Name == "" || !isKebabCase(dc.Name) {
		return fmt.Errorf("%w: %s: invalid name %q", ErrInvalidDoctorCheck, p, dc.Name)
	}
	if !dc.Phase.Valid() {
		return fmt.Errorf("%w: %s: invalid phase %q", ErrInvalidDoctorCheck, p, dc.Phase)
	}
	if dc.Check == nil {
		return fmt.Errorf("%w: %s: nil check function", ErrInvalidDoctorCheck, p)
	}
	if len(dc.Dependencies) == 0 {
		return fmt.Errorf("%w: %s: missing dependencies", ErrInvalidDoctorCheck, p)
	}
	if len(dc.Platforms) == 0 {
		return fmt.Errorf("%w: %s: missing platforms", ErrInvalidDoctorCheck, p)
	}
	seenPlatform := make(map[string]struct{}, len(dc.Platforms))
	for _, plat := range dc.Platforms {
		if !validDoctorPlatform(plat) {
			return fmt.Errorf("%w: %s: invalid platform %q", ErrInvalidDoctorCheck, p, plat)
		}
		if _, dup := seenPlatform[plat]; dup {
			return fmt.Errorf("%w: %s: duplicate platform %q", ErrInvalidDoctorCheck, p, plat)
		}
		seenPlatform[plat] = struct{}{}
	}
	if len(dc.Codes) == 0 {
		return fmt.Errorf("%w: %s: missing codes", ErrInvalidDoctorCheck, p)
	}
	seenCode := make(map[string]struct{}, len(dc.Codes))
	for _, code := range dc.Codes {
		if !strings.HasPrefix(code, "doctor-") {
			return fmt.Errorf("%w: %s: code %q must have doctor- prefix", ErrInvalidDoctorCheck, p, code)
		}
		if _, dup := seenCode[code]; dup {
			return fmt.Errorf("%w: %s: duplicate code %q", ErrInvalidDoctorCheck, p, code)
		}
		seenCode[code] = struct{}{}
	}
	return nil
}

var validDoctorPlatforms = map[string]struct{}{"any": {}}

// RegisterDoctorPlatforms adds valid platform names for doctor check validation.
// Called from host package init() to register platform type strings.
func RegisterDoctorPlatforms(platforms []string) {
	mu.Lock()
	defer mu.Unlock()
	for _, p := range platforms {
		validDoctorPlatforms[p] = struct{}{}
	}
}

func validDoctorPlatform(platform string) bool {
	_, ok := validDoctorPlatforms[platform]
	return ok
}

func isKebabCase(s string) bool {
	if s == "" {
		return false
	}
	prev := true
	for i := range len(s) {
		c := s[i]
		if c == '-' {
			if prev {
				return false
			}
			prev = true
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			prev = false
			continue
		}
		return false
	}
	return !prev
}
