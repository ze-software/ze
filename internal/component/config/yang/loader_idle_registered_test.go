package yang_test

import (
	"testing"

	"github.com/ze-software/ze/internal/component/config/yang"
	_ "github.com/ze-software/ze/internal/component/plugin/all" // registers every compiled-in YANG module
)

// TestNoShippedModuleAddsNothing: every module compiled into ze adds config,
// an augment, a command-tree node (a ze:command, or the ze:help of a verb-root
// anchor), an rpc, or is imported by another module, which is the rule the
// join refuses an external module by (refuseIdleModules). Method: the
// external test package imports the generated composition root and judges the
// set DefaultLoader answers with IdleModuleNames and EmptyModuleNames, the
// same functions the join's rule is.
//
// The two findings are reported apart, and the join refuses both from an
// external plugin. A module of IdleModuleNames holds definitions ze drops in
// silence. A module of EmptyModuleNames declares no statement at all: in the
// compiled-in set it is a leftover its yang package still registers, and the
// test names it so it is deleted with every line that embeds, registers or
// returns it. Both lists are expected empty.
//
// VALIDATES: one rule for a compiled-in module and an external one; only the
// time differs, the build for one and the start for the other.
// PREVENTS: a compiled-in module whose nodes nothing reads (a config module
// named without -conf, a command module with no ze:command) or that declares
// nothing at all shipped in silence, while the same module from an external
// plugin is refused.
func TestNoShippedModuleAddsNothing(t *testing.T) {
	if len(yang.Modules()) == 0 {
		t.Fatal("no registered YANG module: the composition root registered nothing")
	}
	compiled, err := yang.DefaultLoader()
	if err != nil {
		t.Fatalf("DefaultLoader: %v", err)
	}
	if idle := yang.IdleModuleNames(compiled); len(idle) > 0 {
		t.Errorf("modules compiled into ze declare definitions no reader of ze takes, and no module imports them: %v", idle)
	}
	if empty := yang.EmptyModuleNames(compiled); len(empty) > 0 {
		t.Errorf("modules compiled into ze declare no statement at all (a header, imports and revisions only); "+
			"each is a leftover its yang package still registers, to delete with every line that embeds, registers or returns it: %v", empty)
	}
}
