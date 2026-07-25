// Design: docs/features/ai-first.md -- doctor check registration
// Related: doctor_provider.go -- provider bridge for show doctor

package diagnostic

import (
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// DoctorCheckPhase determines when a doctor check runs relative to config loading.
type DoctorCheckPhase string

const (
	DoctorPhasePreConfig     DoctorCheckPhase = "pre-config"
	DoctorPhaseMissingConfig DoctorCheckPhase = "missing-config"
	DoctorPhasePostConfig    DoctorCheckPhase = "post-config"
)

// DoctorPlatformAny matches all platforms.
const DoctorPlatformAny = "any"

// DoctorCheckContext is the runtime context passed to each doctor check.
// Tree is typed as any to avoid a cycle between diagnostic and config;
// check functions that need *config.Tree should type-assert.
type DoctorCheckContext struct {
	Tree      any
	ConfigDir string
	Plugins   []zeplugin.PluginConfig
	Store     storage.Storage
	Platform  *host.PlatformInfo
}

// DoctorCheckFunc is the signature of a doctor check function.
type DoctorCheckFunc func(DoctorCheckContext) []Diagnostic

// DoctorCheck is a registered doctor readiness check.
type DoctorCheck struct {
	Name         string
	Phase        DoctorCheckPhase
	Order        int
	Component    string
	Dependencies []string
	Platforms    []string
	Codes        []string
	Check        DoctorCheckFunc
}

var doctorCheckRegistry = struct {
	sync.Mutex
	entries []DoctorCheck
	names   map[string]struct{}
}{names: make(map[string]struct{})}

// RegisterDoctorCheck adds a doctor check to the global registry.
// Returns an error on validation failure or duplicate name.
// Owner packages call this from init() to register their runtime dependency checks.
func RegisterDoctorCheck(check DoctorCheck) error {
	doctorCheckRegistry.Lock()
	defer doctorCheckRegistry.Unlock()
	if err := validateDoctorCheckReg(check); err != nil {
		return err
	}
	if _, exists := doctorCheckRegistry.names[check.Name]; exists {
		var tb textbuf.Buffer
		return errors.New(tb.Str("diagnostic: duplicate doctor check ").Str(check.Name).String())
	}
	doctorCheckRegistry.names[check.Name] = struct{}{}
	doctorCheckRegistry.entries = append(doctorCheckRegistry.entries, cloneDoctorCheckReg(check))
	return nil
}

// DoctorChecksForPhase returns all registered checks for the given phase,
// sorted by order then name.
func DoctorChecksForPhase(phase DoctorCheckPhase) []DoctorCheck {
	doctorCheckRegistry.Lock()
	defer doctorCheckRegistry.Unlock()
	checks := make([]DoctorCheck, 0, len(doctorCheckRegistry.entries))
	for i := range doctorCheckRegistry.entries {
		if doctorCheckRegistry.entries[i].Phase == phase {
			checks = append(checks, cloneDoctorCheckReg(doctorCheckRegistry.entries[i]))
		}
	}
	sort.Slice(checks, func(i, j int) bool {
		return doctorCheckLessReg(checks[i], checks[j])
	})
	return checks
}

// DoctorCheckNames returns the names of all registered checks, sorted.
func DoctorCheckNames() []string {
	doctorCheckRegistry.Lock()
	defer doctorCheckRegistry.Unlock()
	names := make([]string, 0, len(doctorCheckRegistry.entries))
	for i := range doctorCheckRegistry.entries {
		names = append(names, doctorCheckRegistry.entries[i].Name)
	}
	sort.Strings(names)
	return names
}

// DoctorCheckSupportsPlatform reports whether a check should run on the given platform.
func DoctorCheckSupportsPlatform(check DoctorCheck, platform *host.PlatformInfo) bool {
	for _, allowed := range check.Platforms {
		if allowed == DoctorPlatformAny {
			return true
		}
		if platform != nil && allowed == platform.Type.String() {
			return true
		}
	}
	return false
}

// ResetDoctorChecksForTest clears the doctor check registry. Test use only.
func ResetDoctorChecksForTest() {
	doctorCheckRegistry.Lock()
	defer doctorCheckRegistry.Unlock()
	doctorCheckRegistry.entries = nil
	doctorCheckRegistry.names = make(map[string]struct{})
}

func validateDoctorCheckReg(check DoctorCheck) error {
	const p = "diagnostic doctor check: "
	var tb textbuf.Buffer
	if check.Name == "" || !isLowerKebabDiag(check.Name) {
		return errors.New(tb.Str(p).Str("invalid name ").Str(check.Name).String())
	}
	if !check.Phase.Valid() {
		return errors.New(tb.Reset().Str(p).Str("unknown phase ").Str(string(check.Phase)).String())
	}
	if check.Component == "" || !isLowerKebabDiag(check.Component) {
		return errors.New(tb.Reset().Str(p).Str("invalid component ").Str(check.Component).String())
	}
	if check.Check == nil {
		return errors.New(tb.Reset().Str(p).Str("missing check function for ").Str(check.Name).String())
	}
	if err := validateDoctorCheckKebabListReg("dependency", check.Dependencies); err != nil {
		return err
	}
	if err := validateDoctorCheckPlatformsReg(check.Platforms); err != nil {
		return err
	}
	return validateDoctorCheckCodesReg(check.Codes)
}

func validateDoctorCheckKebabListReg(field string, values []string) error {
	const p = "diagnostic doctor check: "
	var tb textbuf.Buffer
	if len(values) == 0 {
		return errors.New(tb.Str(p).Str("missing ").Str(field).String())
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !isLowerKebabDiag(value) {
			return errors.New(tb.Reset().Str(p).Str("invalid ").Str(field).Byte(' ').Str(value).String())
		}
		if _, exists := seen[value]; exists {
			return errors.New(tb.Reset().Str(p).Str("duplicate ").Str(field).Byte(' ').Str(value).String())
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateDoctorCheckPlatformsReg(platforms []string) error {
	const p = "diagnostic doctor check: "
	var tb textbuf.Buffer
	if len(platforms) == 0 {
		return errors.New(tb.Str(p).Str("missing platform").String())
	}
	seen := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		if !validDoctorCheckPlatformReg(platform) {
			return errors.New(tb.Reset().Str(p).Str("invalid platform ").Str(platform).String())
		}
		if _, exists := seen[platform]; exists {
			return errors.New(tb.Reset().Str(p).Str("duplicate platform ").Str(platform).String())
		}
		seen[platform] = struct{}{}
	}
	return nil
}

func validDoctorCheckPlatformReg(platform string) bool {
	// "any" is the doctor-specific wildcard; everything else must be a name the
	// host package owns. host.ValidPlatformName is the single source of truth so
	// this validator cannot drift from the platform set.
	return platform == DoctorPlatformAny || host.ValidPlatformName(platform)
}

func validateDoctorCheckCodesReg(codes []string) error {
	const p = "diagnostic doctor check: "
	var tb textbuf.Buffer
	if len(codes) == 0 {
		return errors.New(tb.Str(p).Str("missing diagnostic code").String())
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if !strings.HasPrefix(code, "doctor-") {
			return errors.New(tb.Reset().Str(p).Str("invalid diagnostic code ").Str(code).String())
		}
		if _, exists := seen[code]; exists {
			return errors.New(tb.Reset().Str(p).Str("duplicate diagnostic code ").Str(code).String())
		}
		seen[code] = struct{}{}
	}
	return nil
}

// Valid reports whether the phase is a known doctor check phase.
func (phase DoctorCheckPhase) Valid() bool {
	switch phase {
	case DoctorPhasePreConfig, DoctorPhaseMissingConfig, DoctorPhasePostConfig:
		return true
	default:
		return false
	}
}

func doctorCheckLessReg(a, b DoctorCheck) bool {
	ar, br := doctorPhaseRank(a.Phase), doctorPhaseRank(b.Phase)
	if ar != br {
		return ar < br
	}
	if a.Order != b.Order {
		return a.Order < b.Order
	}
	return a.Name < b.Name
}

func doctorPhaseRank(phase DoctorCheckPhase) int {
	switch phase {
	case DoctorPhasePreConfig:
		return 0
	case DoctorPhaseMissingConfig:
		return 1
	case DoctorPhasePostConfig:
		return 2
	default:
		return 3
	}
}

func isLowerKebabDiag(value string) bool {
	if value == "" {
		return false
	}
	prevHyphen := true
	for i := range len(value) {
		c := value[i]
		if c == '-' {
			if prevHyphen {
				return false
			}
			prevHyphen = true
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			prevHyphen = false
			continue
		}
		return false
	}
	return !prevHyphen
}

func cloneDoctorCheckReg(check DoctorCheck) DoctorCheck {
	check.Dependencies = append([]string(nil), check.Dependencies...)
	check.Platforms = append([]string(nil), check.Platforms...)
	check.Codes = append([]string(nil), check.Codes...)
	return check
}
