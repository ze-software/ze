// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- commit-time verification of producer-owned firewall tables
// Related: register_producer_verify.go -- registers verifyProducerTables as an original-tree validator
// Related: backend.go -- RunVerifier invokes the selected backend's verifier

package firewall

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	// producerTablesValidatorName is the original-tree validator name registered by
	// this package so boot, reload, commit and `ze config validate` all verify
	// producer-built tables against the candidate firewall backend before any
	// plugin starts or applies.
	producerTablesValidatorName = "firewall-producer-tables"

	firewallSchemaDefaultWhy  = "ze reads the firewall backend default from the YANG schema rather than assuming one"
	firewallSchemaValidateFix = "ze config validate"
)

// ConfigTableBuilder translates a candidate configuration tree into the
// firewall tables a producer will register for that configuration. Builders
// MUST NOT modify tree and MUST be safe for concurrent use. A builder returns
// (nil, nil) when the candidate does not configure that producer.
type ConfigTableBuilder func(tree *config.Tree) ([]Table, error)

type namedConfigTableBuilder struct {
	name  string
	build ConfigTableBuilder
}

var (
	configTableBuildersMu sync.RWMutex
	configTableBuilders   = map[string]ConfigTableBuilder{}
)

// RegisterConfigTableBuilder registers a producer's config-time table builder
// under name. Call from init() in the package that owns the producer. Empty
// names, nil builders and duplicate names are refused.
//
// Safe for concurrent use.
func RegisterConfigTableBuilder(name string, build ConfigTableBuilder) error {
	if name == "" {
		return errors.New("firewall: config table builder registration requires a name")
	}
	if build == nil {
		return fmt.Errorf("firewall: config table builder %q has a nil builder function", name)
	}
	configTableBuildersMu.Lock()
	defer configTableBuildersMu.Unlock()
	if _, exists := configTableBuilders[name]; exists {
		return fmt.Errorf("firewall: config table builder %q is already registered", name)
	}
	configTableBuilders[name] = build
	return nil
}

// ConfigTableBuilderFor returns the registered config table builder for name,
// or (nil, false) when none is registered under that name.
//
// Safe for concurrent use.
func ConfigTableBuilderFor(name string) (ConfigTableBuilder, bool) {
	configTableBuildersMu.RLock()
	defer configTableBuildersMu.RUnlock()
	b, ok := configTableBuilders[name]
	return b, ok
}

// registeredConfigTableBuilders returns a sorted snapshot of every registered
// config table builder so verification runs in deterministic name order without
// holding configTableBuildersMu across builder callbacks.
func registeredConfigTableBuilders() []namedConfigTableBuilder {
	configTableBuildersMu.RLock()
	defer configTableBuildersMu.RUnlock()
	if len(configTableBuilders) == 0 {
		return nil
	}
	names := make([]string, 0, len(configTableBuilders))
	for name := range configTableBuilders {
		names = append(names, name)
	}
	slices.Sort(names)
	entries := make([]namedConfigTableBuilder, len(names))
	for i, name := range names {
		entries[i] = namedConfigTableBuilder{name: name, build: configTableBuilders[name]}
	}
	return entries
}

// verifyProducerTables runs every registered config table builder over tree
// under the candidate's firewall backend and runs that backend's registered
// verifier over the resulting tables, joining every refusal.
//
// When no verifier is registered for the candidate's firewall backend (for
// example the kernel backend, which accepts every table shape), no builder runs
// so malformed producer leaves keep reaching YANG validation and the producer's
// own verifier with their normal diagnostics.
func verifyProducerTables(tree *config.Tree) error {
	// refuse-by-default: guards producerTablesValidatorName -- a nil tree holds no producer configuration
	if tree == nil {
		return nil
	}
	builders := registeredConfigTableBuilders()
	if len(builders) == 0 {
		return nil
	}
	backend, err := resolveFirewallBackend(tree)
	if err != nil {
		return err
	}
	if !hasVerifierForBackend(backend) {
		return nil
	}

	var errs []error
	for _, entry := range builders {
		tables, buildErr := entry.build(tree)
		if buildErr != nil {
			errs = append(errs, buildErr)
			continue
		}
		if len(tables) == 0 {
			continue
		}
		if verifyErr := RunVerifier(backend, tables); verifyErr != nil {
			errs = append(errs, verifyErr)
		}
	}
	return errors.Join(errs...)
}

// hasVerifierForBackend reports whether backendName has a registered firewall
// verifier.
func hasVerifierForBackend(backendName string) bool {
	backendsMu.Lock()
	defer backendsMu.Unlock()
	_, ok := verifiers[backendName]
	return ok
}

// resolveFirewallBackend returns the firewall backend leaf written in tree, or
// the schema default of firewall/backend from config.YANGSchema() when omitted.
// It never hardcodes a fallback backend literal.
func resolveFirewallBackend(tree *config.Tree) (string, error) {
	if fwTree := tree.GetContainer(ConfigRoot); fwTree != nil && !fwTree.IsInactive() {
		if b, ok := fwTree.Get("backend"); ok {
			if !fwTree.IsLeafInactive("backend") && b != "" {
				return b, nil
			}
		}
	}
	schema, err := config.YANGSchema()
	if err != nil {
		return "", opererr.Wrap(err,
			"firewall backend default could not be read: YANG schema unavailable",
			firewallSchemaDefaultWhy,
			firewallSchemaValidateFix,
		)
	}
	node, err := schema.Lookup("firewall/backend")
	if err != nil {
		return "", opererr.Wrap(err,
			"firewall backend default could not be read from the YANG schema",
			firewallSchemaDefaultWhy,
			firewallSchemaValidateFix,
		)
	}
	leaf, ok := node.(*config.LeafNode)
	if !ok {
		return "", opererr.New(
			"firewall/backend in the YANG schema is not a leaf",
			firewallSchemaDefaultWhy,
			firewallSchemaValidateFix,
		)
	}
	if leaf.Default == "" {
		return "", opererr.New(
			"firewall/backend has no default in the YANG schema",
			firewallSchemaDefaultWhy,
			firewallSchemaValidateFix,
		)
	}
	return leaf.Default, nil
}
