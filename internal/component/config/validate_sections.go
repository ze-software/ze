// Design: docs/architecture/config/yang-config-design.md -- ze:validate custom validators
// Related: loader.go -- LoadConfig refuses a config this walk rejects
// Related: cli/cmd_validate.go -- `ze config validate` runs the same walk

package config

import (
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"

	gyang "github.com/openconfig/goyang/pkg/yang"

	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// ErrCustomValidation marks a LoadConfig failure a registered ze:validate
// custom validator produced. It separates "the operator wrote a value the rules
// refuse" from every other load failure, and one caller needs that separation:
// runYANGConfig (cmd/ze/hub/main.go) answers a load failure with
// RecoverConfig, which walks rollback history and REWRITES the config file on
// disk. Rewriting an operator's file because they typed a bad hostname is the
// wrong answer, so that caller declines recovery for this error.
var ErrCustomValidation = errors.New("config validation failed")

// validatedSections lists the top-level config sections the custom-validator
// walk covers. BGP is excluded because it has its own deeper path:
// PeersFromConfigTree (internal/component/bgp/config/peers.go) re-checks every
// bgp-bound validator name, and startup reaches it directly, from
// CreateReactorFromTree (internal/component/bgp/config/loader_create.go).
// infra.ValidateBGPPeers is the offline door onto the same walk, through
// validatePeersFromTree (internal/component/bgp/config/register.go); its two
// non-test callers are `ze doctor` (checkBGPPeerConfig) and `ze config
// validate` (runValidation), and neither of those is the startup path.
//
// `service` was added on 2026-09-02, and it is the one widening that carries no
// blast radius. The three defects below each gate a validator NAME, and none of
// those names appears under `service`: the only two there are `ipv4-address`
// and `ipv4-prefix`, both pure form checks over the value in hand, reading no
// registry and depending on no startup order. Before it was added, a
// `ze:validate` under `service` loaded, passed CheckAllValidatorsRegistered and
// never ran, so `ze config validate` accepted a DHCP `default-router` of
// 2001:db8::1 against `ze:validate "ipv4-address"`. The parse, install and ui
// suites carry the 22 service-bearing configs in the tree and stay green.
//
// `static` is the same shape and is NOT added here, because nothing has walked
// its configs to measure what its two prefix validators would newly refuse.
//
// Two further properties of the list are known and recorded rather than fixed
// here (plan/journal/gate-excludes-part-of-its-population.md):
//
//   - Six names -- web, ssh, dns, looking-glass, mcp, managed -- are not
//     top-level sections. They sit under `environment`, which the list omits, so
//     GetContainer returns nil for them and those six iterations do nothing.
//   - Twenty-five real top-level sections of the thirty-six the schema derives
//     are absent.
//
// Widening it is gated on three separate defects, each with its own blast
// radius. AddressFamilyValidator reads registry.FamilyMap(), which excludes the
// builtin families, so adding `bgp` and `redistribute` would refuse 742 + 33
// family sites across 687 shipped configs. `send-message-type` and
// `receive-event-type` read registries that RegisterPluginSendTypes populates
// from PeersFromTree and NewServer, both strictly later than LoadConfig, so
// they would fail closed on valid input at exactly the point this walk runs.
var validatedSections = []string{
	sectionInterface, "sysctl", "fib", sectionPlugin, sectionWeb, "ssh", "dns",
	sectionTelemetry, sectionLookingGlass, "mcp", "managed", "vpp",
	"vpn", "pki", "l2tp", "isis", "ospf", "service",
}

// SectionValidationError is one failure the walk found, paired with the
// top-level section it was found under and with whether its leaf holds a
// sensitive value.
type SectionValidationError struct {
	Section   string
	Sensitive bool
	Err       yang.ValidationError
}

// Blocking reports whether this failure refuses the config.
//
// A missing mandatory field does not refuse. `ze config validate` has always
// graded it a warning, and LoadConfig must grade it the same way: an operator
// who checks a config before an upgrade and a daemon that loads it after must
// reach the same verdict on the same bytes.
func (e SectionValidationError) Blocking() bool {
	return e.Err.Type != yang.ErrTypeMissing
}

// Message renders the failure as "<section>: <path>: <reason>". The reason is
// replaced by the error type when the leaf is sensitive, so a refusal never
// prints a password or a key into a startup log.
func (e SectionValidationError) Message() string {
	var tb textbuf.Buffer
	msg := e.Err.Message
	if e.Sensitive {
		msg = tb.Str(e.Err.Type.String()).Str(" validation failed (sensitive value redacted)").String()
	}
	if e.Err.Path != "" {
		msg = tb.Reset().Str(e.Err.Path).Str(": ").Str(msg).String()
	}
	return tb.Reset().Str(e.Section).Str(": ").Str(msg).String()
}

// ValidateCustomSections applies the registered ze:validate custom validators,
// and the YANG cardinality, range and pattern checks, to every section in
// validatedSections.
//
// This is the one walk. LoadConfig calls it so a daemon start and a SIGHUP
// reload apply the rules, and cli.runValidation calls it so `ze config validate`
// reports them. A second walk would drift, and the drift is invisible until a
// validator runs on one path only, which is the defect this function closes.
//
// The validator is built with no plugin YANG, matching `ze config validate`
// rather than LoadConfig's own parse. That is deliberate: the guide tells an
// operator to run `ze config validate` to learn whether the daemon will refuse
// the config, and a validator built over a different module set could answer
// differently. Unknown fields are not the loss they would be elsewhere here --
// ValidateTreeAllModules never emits ErrTypeUnknown, so a node the module set
// does not describe is skipped, not rejected.
//
// An error return means the walk could not run. It is never reported as "no
// failures found": a validator that cannot be built must refuse the config, not
// wave it through.
func ValidateCustomSections(tree *Tree) ([]SectionValidationError, error) {
	validator, err := YANGValidatorWithPlugins(nil)
	if err != nil {
		return nil, fmt.Errorf("build YANG validator: %w", err)
	}

	var found []SectionValidationError
	for _, section := range validatedSections {
		container := tree.GetContainer(section)
		if container == nil {
			continue
		}
		for _, ve := range validator.ValidateTreeAllModules(section, container.ToMap()) {
			found = append(found, SectionValidationError{Section: section, Err: ve})
		}
	}
	if len(found) == 0 {
		return nil, nil
	}

	// Sensitivity is resolved only once something has failed, so a valid config
	// never pays for a second schema build. When the schema cannot be built the
	// answer is unknown, and unknown redacts: printing a secret is the failure
	// that cannot be undone.
	redactAll := true
	var sensitiveKeys map[string]bool
	if schema, schemaErr := YANGSchema(); schemaErr == nil {
		sensitiveKeys = SensitiveKeys(schema)
		redactAll = false
	}
	for i := range found {
		found[i].Sensitive = redactAll || isSensitiveLeaf(found[i].Err.Path, sensitiveKeys)
	}
	return found, nil
}

// refuseInvalidCustomSections turns the walk into LoadConfig's verdict: an error
// naming every blocking failure, or nil.
func refuseInvalidCustomSections(tree *Tree) error {
	found, err := ValidateCustomSections(tree)
	if err != nil {
		return err
	}
	var blocking []string
	for _, f := range found {
		if f.Blocking() {
			blocking = append(blocking, f.Message())
		}
	}
	if len(blocking) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrCustomValidation, textbuf.Join(blocking, "; "))
}

// isSensitiveLeaf reports whether the last element of a validation error's path
// names a leaf the schema marks sensitive. Moved here from cli/cmd_validate.go,
// which had the only copy while it had the only walk.
//
// The separator is "/" because that is what the producer writes: walkTree
// (internal/component/config/yang/validator.go) joins each child onto its
// parent with Byte('/'), and SensitiveKeys (schema.go) keys the map on the bare
// leaf name. The copy this function replaces split on ".", so it matched no
// path the walk emits and every sensitive leaf printed its value.
func isSensitiveLeaf(path string, sensitiveKeys map[string]bool) bool {
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		return sensitiveKeys[path[idx+1:]]
	}
	return sensitiveKeys[path]
}

// knownUnwalkedValidatorSections names every top-level section that declares a
// ze:validate and is deliberately absent from validatedSections, against what
// that absence costs and what it owes. A section in this map is a DECISION. A
// section in neither this map nor validatedSections is a DEFECT, and it is the
// defect nobody could see: an annotation that never runs is spelled exactly
// like one that does, and CheckAllValidatorsRegistered is satisfied either way,
// because it asks whether the validator function exists rather than whether the
// walk reaches it. Three of these five were found by measurement on 2026-09-02
// and had never been recorded.
var knownUnwalkedValidatorSections = map[string]string{
	"bgp": "AddressFamilyValidator reads registry.FamilyMap(), which omits the " +
		"builtin families, so walking bgp refuses 742 family sites across 687 " +
		"shipped configs. The validator is the defect, not the section.",
	"redistribute": "The same AddressFamilyValidator defect, over 33 further " +
		"family sites.",
	"policy": "The one exclusion with a user-visible cost: all four annotated " +
		"`policy route ... rule ... from` leaves accept a value their annotation " +
		"forbids, because the policyroute parser checks next-hop alone, so a " +
		"typo in a match prefix yields a rule that matches nothing and no " +
		"surface says so. Widening is not a measurement run: the population is " +
		"71 configs, the section holds the BGP routing-policy tree as well as " +
		"policy routing, and ze-types.yang carries ze:validate " +
		"\"registered-address-family\" in a grouping, so walking policy can arm " +
		"the AddressFamilyValidator defect above. Settle that reachability first.",
	"static": "Inert, and nothing is user-visible: the static parser refuses a " +
		"malformed prefix itself. Owes the measurement run service had.",
	"control-plane-protection": "Inert, and nothing is user-visible: the copp " +
		"parser refuses a malformed prefix itself. Owes the measurement run " +
		"service had.",
}

// ValidatorCoverage is what the resolved model says about ze:validate: every
// top-level section that declares one, and the subset of those that no rule
// accounts for.
type ValidatorCoverage struct {
	// Declaring is every top-level section carrying a ze:validate anywhere in
	// its subtree, in the model this binary compiled.
	Declaring map[string]bool
	// Unaccounted is the sections in Declaring that ValidateCustomSections does
	// not walk and knownUnwalkedValidatorSections does not excuse. Each one is
	// an annotation that silently does nothing.
	Unaccounted []string
}

// ValidatorSectionCoverage derives ValidatorCoverage from the resolved model.
//
// It reads the RESOLVED tree rather than the YANG source, because a ze:validate
// written inside a grouping lands wherever that grouping is used, so the source
// cannot say which section owns it. It builds the model the way
// ValidateCustomSections does, so the population it reports is the population
// that walk would cover.
//
// The answer is only as complete as the modules the calling binary LINKED. A
// build with a feature compiled out sees neither that plugin's YANG nor the
// section it declares, so a caller MUST check Declaring against what it expects
// before reading an empty Unaccounted as good news.
func ValidatorSectionCoverage() (ValidatorCoverage, error) {
	loader, err := loadYANGModules(nil)
	if err != nil {
		return ValidatorCoverage{}, fmt.Errorf("build YANG model: %w", err)
	}

	walked := make(map[string]bool, len(validatedSections))
	for _, s := range validatedSections {
		walked[s] = true
	}

	cov := ValidatorCoverage{Declaring: make(map[string]bool)}
	unaccounted := make(map[string]bool)
	for _, modName := range loader.ConfModuleNames() {
		entry := loader.GetEntry(modName)
		if entry == nil || entry.Dir == nil {
			continue
		}
		for section, sectionEntry := range entry.Dir {
			if !subtreeDeclaresValidator(sectionEntry) {
				continue
			}
			cov.Declaring[section] = true
			if walked[section] {
				continue
			}
			if _, known := knownUnwalkedValidatorSections[section]; known {
				continue
			}
			unaccounted[section] = true
		}
	}

	cov.Unaccounted = make([]string, 0, len(unaccounted))
	for s := range unaccounted {
		cov.Unaccounted = append(cov.Unaccounted, s)
	}
	sort.Strings(cov.Unaccounted)
	return cov, nil
}

// KnownUnwalkedValidatorSections answers the recorded exclusions, so a test can
// check the model it read actually contains them rather than reporting a clean
// sheet over a tree it never saw.
func KnownUnwalkedValidatorSections() map[string]string {
	out := make(map[string]string, len(knownUnwalkedValidatorSections))
	maps.Copy(out, knownUnwalkedValidatorSections)
	return out
}

// subtreeDeclaresValidator reports whether entry or anything under it carries a
// ze:validate. The recursion is over a resolved YANG tree, which is finite and
// built from modules this binary compiled in, never from operator input.
func subtreeDeclaresValidator(entry *gyang.Entry) bool {
	if yang.GetValidateExtension(entry) != "" {
		return true
	}
	for _, child := range entry.Dir {
		if subtreeDeclaresValidator(child) {
			return true
		}
	}
	return false
}
