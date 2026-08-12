// Design: docs/architecture/config/yang-config-design.md -- ze:validate custom validators
// Related: loader.go -- LoadConfig refuses a config this walk rejects
// Related: cli/cmd_validate.go -- `ze config validate` runs the same walk

package config

import (
	"errors"
	"fmt"
	"strings"

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
// The list is deliberately the one `ze config validate` has always walked, and
// this change does not widen it. Two properties of it are known and recorded
// rather than fixed here (plan/journal/gate-excludes-part-of-its-population.md):
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
	"interface", "sysctl", "fib", "plugin", "web", "ssh", "dns",
	"telemetry", "looking-glass", "mcp", "managed", "vpp",
	"vpn", "pki", "l2tp", "isis", "ospf",
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
