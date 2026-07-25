// Design: docs/features/ai-first.md — doctor check registration
// Overview: doctor.go — readiness check runner

package doctor

import (
	"errors"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/host"
	zeplugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	doctorCheckPlatformAny = "any"
	doctorCheckErrorPrefix = "doctor check registry: "
)

type doctorCheckPhase string

const (
	doctorCheckPhasePreConfig     doctorCheckPhase = "pre-config"
	doctorCheckPhaseMissingConfig doctorCheckPhase = "missing-config"
	doctorCheckPhasePostConfig    doctorCheckPhase = "post-config"
)

type doctorCheckContext struct {
	Tree      *config.Tree
	ConfigDir string
	Plugins   []zeplugin.PluginConfig
	Store     storage.Storage
	Platform  *host.PlatformInfo
}

type doctorCheckFunc func(doctorCheckContext) []diagnostic.Diagnostic

type doctorCheck struct {
	Name         string
	Phase        doctorCheckPhase
	Order        int
	Component    string
	Dependencies []string
	Platforms    []string
	Codes        []string
	Check        doctorCheckFunc
}

type doctorCheckRegistry struct {
	entries []doctorCheck
	names   map[string]struct{}
}

var defaultDoctorCheckRegistry = newDoctorCheckRegistry()

func newDoctorCheckRegistry() *doctorCheckRegistry {
	return &doctorCheckRegistry{names: make(map[string]struct{})}
}

func runDoctorChecks(phase doctorCheckPhase, ctx doctorCheckContext) []diagnostic.Diagnostic {
	checks := defaultDoctorCheckRegistry.checksForPhase(phase)
	var diags []diagnostic.Diagnostic
	for i := range checks {
		if !checks[i].supportsPlatform(ctx.Platform) {
			continue
		}
		diags = append(diags, checks[i].Check(ctx)...)
	}

	exportedCtx := diagnostic.DoctorCheckContext{
		Tree:      ctx.Tree,
		ConfigDir: ctx.ConfigDir,
		Plugins:   ctx.Plugins,
		Store:     ctx.Store,
		Platform:  ctx.Platform,
	}
	exported := diagnostic.DoctorChecksForPhase(diagnostic.DoctorCheckPhase(phase))
	for i := range exported {
		if !diagnostic.DoctorCheckSupportsPlatform(exported[i], ctx.Platform) {
			continue
		}
		diags = append(diags, exported[i].Check(exportedCtx)...)
	}

	diags = append(diags, runPluginRegistryChecks(phase, ctx)...)
	return diags
}

func (r *doctorCheckRegistry) register(check doctorCheck) error {
	if r.names == nil {
		r.names = make(map[string]struct{})
	}
	if err := validateDoctorCheck(check); err != nil {
		return err
	}
	if _, exists := r.names[check.Name]; exists {
		var tb textbuf.Buffer
		return errors.New(tb.Str(doctorCheckErrorPrefix).Str("duplicate check name ").Str(check.Name).String())
	}
	r.names[check.Name] = struct{}{}
	r.entries = append(r.entries, cloneDoctorCheck(check))
	return nil
}

func (r *doctorCheckRegistry) checks() []doctorCheck {
	checks := make([]doctorCheck, len(r.entries))
	for i := range r.entries {
		checks[i] = cloneDoctorCheck(r.entries[i])
	}
	sort.Slice(checks, func(i, j int) bool {
		return doctorCheckLess(checks[i], checks[j])
	})
	return checks
}

func (r *doctorCheckRegistry) checksForPhase(phase doctorCheckPhase) []doctorCheck {
	checks := r.checks()
	keep := 0
	for i := range checks {
		if checks[i].Phase == phase {
			checks[keep] = checks[i]
			keep++
		}
	}
	return checks[:keep]
}

func (check doctorCheck) supportsPlatform(platform *host.PlatformInfo) bool {
	for _, allowed := range check.Platforms {
		if allowed == doctorCheckPlatformAny {
			return true
		}
		if platform != nil && allowed == platform.Type.String() {
			return true
		}
	}
	return false
}

func validateDoctorCheck(check doctorCheck) error {
	var tb textbuf.Buffer
	if !isLowerKebab(check.Name) {
		return errors.New(tb.Str(doctorCheckErrorPrefix).Str("invalid check name ").Str(check.Name).String())
	}
	if !check.Phase.valid() {
		return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("unknown phase ").Str(string(check.Phase)).String())
	}
	if !isLowerKebab(check.Component) {
		return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("invalid component ").Str(check.Component).String())
	}
	if check.Check == nil {
		return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("missing check function for ").Str(check.Name).String())
	}
	if err := validateDoctorCheckKebabList("dependency", check.Dependencies); err != nil {
		return err
	}
	if err := validateDoctorCheckPlatforms(check.Platforms); err != nil {
		return err
	}
	return validateDoctorCheckCodes(check.Codes)
}

func validateDoctorCheckKebabList(field string, values []string) error {
	var tb textbuf.Buffer
	if len(values) == 0 {
		return errors.New(tb.Str(doctorCheckErrorPrefix).Str("missing ").Str(field).String())
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !isLowerKebab(value) {
			return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("invalid ").Str(field).Byte(' ').Str(value).String())
		}
		if _, exists := seen[value]; exists {
			return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("duplicate ").Str(field).Byte(' ').Str(value).String())
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateDoctorCheckPlatforms(platforms []string) error {
	var tb textbuf.Buffer
	if len(platforms) == 0 {
		return errors.New(tb.Str(doctorCheckErrorPrefix).Str("missing platform").String())
	}
	seen := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		if !validDoctorCheckPlatform(platform) {
			return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("invalid platform ").Str(platform).String())
		}
		if _, exists := seen[platform]; exists {
			return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("duplicate platform ").Str(platform).String())
		}
		seen[platform] = struct{}{}
	}
	return nil
}

func validDoctorCheckPlatform(platform string) bool {
	// "any" is the doctor-specific wildcard; everything else must be a name the
	// host package owns. host.ValidPlatformName is the single source of truth so
	// this validator cannot drift from the platform set.
	return platform == doctorCheckPlatformAny || host.ValidPlatformName(platform)
}

func validateDoctorCheckCodes(codes []string) error {
	var tb textbuf.Buffer
	if len(codes) == 0 {
		return errors.New(tb.Str(doctorCheckErrorPrefix).Str("missing diagnostic code").String())
	}
	seen := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		if !isLowerKebab(code) || !strings.HasPrefix(code, "doctor-") {
			return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("invalid diagnostic code ").Str(code).String())
		}
		if _, exists := seen[code]; exists {
			return errors.New(tb.Reset().Str(doctorCheckErrorPrefix).Str("duplicate diagnostic code ").Str(code).String())
		}
		seen[code] = struct{}{}
	}
	return nil
}

func (phase doctorCheckPhase) valid() bool {
	switch phase {
	case doctorCheckPhasePreConfig, doctorCheckPhaseMissingConfig, doctorCheckPhasePostConfig:
		return true
	default:
		return false
	}
}

func doctorCheckLess(a, b doctorCheck) bool {
	if doctorCheckPhaseRank(a.Phase) != doctorCheckPhaseRank(b.Phase) {
		return doctorCheckPhaseRank(a.Phase) < doctorCheckPhaseRank(b.Phase)
	}
	if a.Order != b.Order {
		return a.Order < b.Order
	}
	return a.Name < b.Name
}

func doctorCheckPhaseRank(phase doctorCheckPhase) int {
	switch phase {
	case doctorCheckPhasePreConfig:
		return 0
	case doctorCheckPhaseMissingConfig:
		return 1
	case doctorCheckPhasePostConfig:
		return 2
	default:
		return 3
	}
}

func isLowerKebab(value string) bool {
	if value == "" {
		return false
	}
	previousHyphen := true
	for i := range len(value) {
		c := value[i]
		if c == '-' {
			if previousHyphen {
				return false
			}
			previousHyphen = true
			continue
		}
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			previousHyphen = false
			continue
		}
		return false
	}
	return !previousHyphen
}

func cloneDoctorCheck(check doctorCheck) doctorCheck {
	check.Dependencies = append([]string(nil), check.Dependencies...)
	check.Platforms = append([]string(nil), check.Platforms...)
	check.Codes = append([]string(nil), check.Codes...)
	return check
}
