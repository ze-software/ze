// Design: docs/architecture/firewall/table-ownership-and-shutdown-flush.md -- tests for the producer-tables tree validator and config table builder registry
// Related: producer_verify.go -- RegisterConfigTableBuilder, verifyProducerTables
// Related: register_producer_verify.go -- registers firewall-producer-tables

package firewall

import (
	"errors"
	"maps"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

func snapshotConfigTableBuildersAndVerifiers(t *testing.T) {
	t.Helper()

	configTableBuildersMu.Lock()
	savedBuilders := maps.Clone(configTableBuilders)
	configTableBuilders = map[string]ConfigTableBuilder{}
	configTableBuildersMu.Unlock()

	backendsMu.Lock()
	savedVerifiers := maps.Clone(verifiers)
	verifiers = map[string]Verifier{}
	backendsMu.Unlock()

	t.Cleanup(func() {
		configTableBuildersMu.Lock()
		configTableBuilders = savedBuilders
		configTableBuildersMu.Unlock()

		backendsMu.Lock()
		verifiers = savedVerifiers
		backendsMu.Unlock()
	})
}

// VALIDATES: AC-8 -- firewall-producer-tables is registered as an original-tree
// validator, runs every registered ConfigTableBuilder in sorted name order under
// the candidate's firewall backend (or its YANG schema default when omitted),
// runs that backend's verifier over the resulting tables, joins every refusal
// under vpp, and refuses nothing under kernel.
// PREVENTS: producer-owned tables (copp, firewall-irr) passing commit/validate
// under firewall backend vpp only to fail at ApplyAll during daemon startup.
func TestProducerTablesValidatorRefusesVPPShape(t *testing.T) {
	snapshotConfigTableBuildersAndVerifiers(t)

	// Registration invariants: empty name, nil builder, and duplicates are refused.
	if err := RegisterConfigTableBuilder("", func(*config.Tree) ([]Table, error) { return nil, nil }); err == nil {
		t.Fatal("expected error for empty builder name")
	}
	if err := RegisterConfigTableBuilder("nil-builder", nil); err == nil {
		t.Fatal("expected error for nil builder")
	}

	// Duplicate tree-validator registration must fail, proving init() wired it.
	if err := config.RegisterTreeValidator(producerTablesValidatorName, func(*config.Tree) error { return nil }); err == nil {
		t.Fatalf("expected duplicate tree validator error for %q", producerTablesValidatorName)
	}

	var order []string
	if err := RegisterConfigTableBuilder("producer-b", func(*config.Tree) ([]Table, error) {
		order = append(order, "producer-b")
		return []Table{{
			Name:   "ze_b",
			Family: FamilyInet,
			Sets:   []Set{{Name: "s_b", Type: SetTypeIPv4}},
		}}, nil
	}); err != nil {
		t.Fatalf("RegisterConfigTableBuilder(producer-b): %v", err)
	}
	if err := RegisterConfigTableBuilder("producer-a", func(*config.Tree) ([]Table, error) {
		order = append(order, "producer-a")
		return []Table{{
			Name:   "ze_a",
			Family: FamilyInet,
			Sets:   []Set{{Name: "s_a", Type: SetTypeIPv4}},
		}}, nil
	}); err != nil {
		t.Fatalf("RegisterConfigTableBuilder(producer-a): %v", err)
	}
	if err := RegisterConfigTableBuilder("producer-a", func(*config.Tree) ([]Table, error) { return nil, nil }); err == nil {
		t.Fatal("expected duplicate builder registration error")
	}

	if err := RegisterVerifier("vpp", func(tables []Table) error {
		var errs []error
		for _, tbl := range tables {
			if len(tbl.Sets) > 0 {
				errs = append(errs, errors.New("table "+tbl.Name+": named sets not supported by backend vpp"))
			}
		}
		return errors.Join(errs...)
	}); err != nil {
		t.Fatalf("RegisterVerifier(vpp): %v", err)
	}

	t.Run("vpp backend joins refusals from every registered builder via ValidateCustomSections", func(t *testing.T) {
		order = nil
		tree := config.NewTree()
		tree.GetOrCreateContainer(ConfigRoot).Set("backend", "vpp")

		_, err := config.ValidateCustomSections(tree)
		if err == nil {
			t.Fatal("expected ValidateCustomSections to refuse producer tables under firewall backend vpp")
		}
		msg := err.Error()
		if !strings.Contains(msg, "ze_a") || !strings.Contains(msg, "ze_b") {
			t.Errorf("refusal must join errors from both producers, got:\n%s", msg)
		}
		if len(order) != 2 || order[0] != "producer-a" || order[1] != "producer-b" {
			t.Errorf("builder execution order = %v, want [producer-a producer-b]", order)
		}
	})

	t.Run("kernel backend explicit refuses nothing", func(t *testing.T) {
		tree := config.NewTree()
		tree.GetOrCreateContainer(ConfigRoot).Set("backend", "kernel")

		if err := verifyProducerTables(tree); err != nil {
			t.Fatalf("expected nil under kernel backend, got %v", err)
		}
	})

	t.Run("omitted firewall block resolves schema default kernel and refuses nothing", func(t *testing.T) {
		tree := config.NewTree()
		if err := verifyProducerTables(tree); err != nil {
			t.Fatalf("expected nil with omitted firewall block (schema default kernel), got %v", err)
		}
	})

	t.Run("omitted firewall block resolves schema default kernel into verifier lookup", func(t *testing.T) {
		kernelVerifierCalled := false
		if err := RegisterVerifier("kernel", func(tables []Table) error {
			kernelVerifierCalled = true
			return nil
		}); err != nil {
			t.Fatalf("RegisterVerifier(kernel): %v", err)
		}
		tree := config.NewTree()
		if err := verifyProducerTables(tree); err != nil {
			t.Fatalf("verifyProducerTables with accepting kernel verifier: %v", err)
		}
		if !kernelVerifierCalled {
			t.Fatal("expected omitted firewall block to resolve schema default \"kernel\" and invoke its verifier")
		}
	})
}
