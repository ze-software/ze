// Design: docs/architecture/core-design.md -- one daemon distance declaration.
// Related: route_view_config.go -- complete startup and replacement values.

package sysrib

import (
	"errors"
	"maps"
	"reflect"
	"testing"

	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// An absent RIB root is a complete default declaration at both startup and
// reload. It never means that the old distance or withholding remains active.
func TestRouteViewConfigAbsentRootDeclaresDefaults(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	wantDistances, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	wantPermit, err := parseFIBImportConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	for _, sections := range [][]sdk.ConfigSection{nil, {{Root: "static", Data: "{}"}}} {
		cfg, present, err := parseRouteViewConfiguration(sections)
		if err != nil {
			t.Fatal(err)
		}
		if !present {
			t.Fatal("absent rib root skipped its default declaration")
		}
		if !maps.Equal(cfg.distances, wantDistances) || !maps.Equal(cfg.permitted, wantPermit) {
			t.Fatalf("default declaration = %+v, want distances %v and permission %v", cfg, wantDistances, wantPermit)
		}
		if !cfg.permitted["bgp"] {
			t.Fatal("absent withhold list did not permit the registered BGP control")
		}
	}
}

func routeViewConfigSections(data string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootRIB, Data: data}}
}

func routeViewConfigMustParse(t *testing.T, data string) routeViewConfiguration {
	t.Helper()
	cfg, present, err := parseRouteViewConfiguration(routeViewConfigSections(data))
	if err != nil {
		t.Fatal(err)
	}
	if !present {
		t.Fatal("configured rib root was absent")
	}
	return cfg
}

// Removing the root restores defaults, and rollback restores the replaced
// distance and permission together. The next rollback still targets that
// restored declaration, rather than the candidate the first rollback undid.
func TestRouteViewConfigRootRemovalRollback(t *testing.T) {
	redistevents.RegisterProtocol("bgp")
	initial := routeViewConfigMustParse(t, `{"rib":{"distance":{"static":7},"fib-withhold":["bgp"]}}`)
	current := initial
	var distances []int
	tx := sdk.NewConfigTx(initial, parseRouteViewConfiguration, func(cfg routeViewConfiguration) error {
		current = cfg
		distances = append(distances, cfg.distances["static"])
		return nil
	})
	if err := tx.Verify(nil); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	defaults, err := parseAdminDistanceConfig("{}")
	if err != nil {
		t.Fatal(err)
	}
	if current.distances["static"] != defaults["static"] || !current.permitted["bgp"] {
		t.Fatalf("root removal kept the old declaration: %+v", current)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(current.distances, initial.distances) || !maps.Equal(current.permitted, initial.permitted) {
		t.Fatalf("rollback restored %+v, want %+v", current, initial)
	}
	if err := tx.Verify(routeViewConfigSections(`{"rib":{"distance":{"static":19}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(distances, []int{defaults["static"], 7, 19, 7}) {
		t.Fatalf("declaration sequence = %v", distances)
	}
}

// A refused verification invalidates the superseded preparation. Neither
// verification changes the active declaration, and only a fresh valid one can
// authorize a later apply.
func TestRouteViewConfigRejectedVerifyCannotApplyStaleCandidate(t *testing.T) {
	initial := routeViewConfigMustParse(t, `{"rib":{"distance":{"static":7}}}`)
	current := initial
	installs := 0
	tx := sdk.NewConfigTx(initial, parseRouteViewConfiguration, func(cfg routeViewConfiguration) error {
		installs++
		current = cfg
		return nil
	})
	if err := tx.Verify(routeViewConfigSections(`{"rib":{"distance":{"static":9}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Verify(routeViewConfigSections(`{"rib":{"distance":{"static":"invalid"}}}`)); err == nil {
		t.Fatal("malformed distance declaration was accepted")
	}
	if err := tx.Apply(); !errors.Is(err, sdk.ErrConfigApplyUnverified) {
		t.Fatalf("apply after refusal = %v, want ErrConfigApplyUnverified", err)
	}
	if installs != 0 || current.distances["static"] != 7 {
		t.Fatalf("refused verification changed current declaration: installs %d, config %+v", installs, current)
	}
	if err := tx.Verify(routeViewConfigSections(`{"rib":{"distance":{"static":11}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if installs != 1 || current.distances["static"] != 11 {
		t.Fatalf("fresh valid control was not applied: installs %d, config %+v", installs, current)
	}
}

// Failed rollback retains its exact target. A retry repairs the declaration,
// and the next transaction rolls back to that repaired value too.
func TestRouteViewConfigRollbackRetryRetainsDeclaration(t *testing.T) {
	initial := routeViewConfigMustParse(t, `{"rib":{"distance":{"static":7}}}`)
	current := initial
	refusal := errors.New("daemon rejected this declaration")
	fail := false
	var attempted []int
	tx := sdk.NewConfigTx(initial, parseRouteViewConfiguration, func(cfg routeViewConfiguration) error {
		attempted = append(attempted, cfg.distances["static"])
		if fail {
			fail = false
			return refusal
		}
		current = cfg
		return nil
	})
	if err := tx.Verify(routeViewConfigSections(`{"rib":{"distance":{"static":9}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	fail = true
	if err := tx.Rollback(); !errors.Is(err, refusal) {
		t.Fatalf("failed rollback = %v, want daemon refusal", err)
	}
	if current.distances["static"] != 9 {
		t.Fatal("failed rollback reported an unapplied declaration")
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if current.distances["static"] != 7 {
		t.Fatal("rollback retry lost the original declaration")
	}
	if err := tx.Verify(routeViewConfigSections(`{"rib":{"distance":{"static":11}}}`)); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(attempted, []int{9, 7, 7, 11, 7}) {
		t.Fatalf("declaration attempts = %v", attempted)
	}
}

// Accepting an unchanged declaration closes the preceding undo set. A later
// rollback cannot restore a declaration from a transaction already committed.
func TestRouteViewConfigUnchangedApplyDropsEarlierUndo(t *testing.T) {
	initial := routeViewConfigMustParse(t, `{"rib":{"distance":{"static":7}}}`)
	current := initial
	installs := 0
	tx := sdk.NewConfigTx(initial, parseRouteViewConfiguration, func(cfg routeViewConfiguration) error {
		installs++
		current = cfg
		return nil
	})
	sections := routeViewConfigSections(`{"rib":{"distance":{"static":9}}}`)
	for range 2 {
		if err := tx.Verify(sections); err != nil {
			t.Fatal(err)
		}
		if err := tx.Apply(); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if installs != 1 || current.distances["static"] != 9 {
		t.Fatalf("unchanged apply retained an old undo: installs %d, config %+v", installs, current)
	}
}
