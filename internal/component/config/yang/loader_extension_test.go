package yang

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// loadExtensionProbe loads the embedded modules plus the given probe modules
// and returns what Resolve answers.
func loadExtensionProbe(t *testing.T, probes ...Module) error {
	t.Helper()
	loader := NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded: %v", err)
	}
	for _, probe := range probes {
		if err := loader.AddModuleFromText(probe.Name, probe.Content); err != nil {
			t.Fatalf("parse %s: %v", probe.Name, err)
		}
	}
	_, resolveErr := loader.Resolve()
	return resolveErr
}

// assertNames fails the test for every wanted fragment the error text lacks.
func assertNames(t *testing.T, err error, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}

// TestResolveRefusesMisspelledExtension: a misspelled `ze:comand` under an
// imported ze-extensions prefix fails Resolve with ErrUndeclaredExtension,
// naming the module, the file location, the statement and the keyword the
// module behind the prefix does not declare. Method: one probe module beside
// the embedded set; goyang alone accepts it.
//
// VALIDATES: Resolve refuses an extension keyword the prefixed module does
// not declare.
// PREVENTS: a misspelled Ze extension loading and its feature (here a command
// binding) going absent with no error.
func TestResolveRefusesMisspelledExtension(t *testing.T) {
	err := loadExtensionProbe(t, Module{Name: "ze-probe.yang", Content: `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    container probe {
        config false;
        ze:comand "ze-probe:show";
    }
}`})
	if !errors.Is(err, ErrUndeclaredExtension) {
		t.Fatalf("Resolve answered %v, want ErrUndeclaredExtension", err)
	}
	assertNames(t, err, "module ze-probe", "ze-probe.yang:7:", "ze:comand",
		`module ze-extensions declares no extension "comand"`)
}

// TestResolveRefusesUnimportedExtensionPrefix: `zx:help` whose prefix matches
// neither the module's own prefix nor any import fails Resolve, naming the
// prefix. Method: one probe module beside the embedded set.
func TestResolveRefusesUnimportedExtensionPrefix(t *testing.T) {
	err := loadExtensionProbe(t, Module{Name: "ze-probe.yang", Content: `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        zx:help "Probe leaf.";
    }
}`})
	if !errors.Is(err, ErrUndeclaredExtension) {
		t.Fatalf("Resolve answered %v, want ErrUndeclaredExtension", err)
	}
	assertNames(t, err, "module ze-probe", "zx:help", `prefix "zx" resolves to no loaded module`)
}

// TestResolveAcceptsDeclaredExtensions: an imported declared extension and
// the module's own declared extension both resolve. Method: the positive pair
// of the two refusals above, so a checker that refuses everything goes red.
func TestResolveAcceptsDeclaredExtensions(t *testing.T) {
	err := loadExtensionProbe(t, Module{Name: "ze-probe.yang", Content: `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    extension marker { argument name; }
    leaf probe {
        type string;
        ze:help "Probe leaf.";
        probe:marker "local";
    }
}`})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
}

// TestResolveAcceptsParentExtensionInSubmodule: a submodule that uses an
// extension its parent module declares, under the shared prefix, resolves,
// and a keyword neither declares is refused. Method: a module and its
// submodule, loaded as two texts.
func TestResolveAcceptsParentExtensionInSubmodule(t *testing.T) {
	parent := Module{Name: "ze-probe.yang", Content: `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    include ze-probe-sub;
    extension marker { argument name; }
}`}
	submodule := func(keyword string) Module {
		return Module{Name: "ze-probe-sub.yang", Content: `submodule ze-probe-sub {
    belongs-to ze-probe { prefix probe; }
    leaf probe {
        type string;
        probe:` + keyword + ` "sub";
    }
}`}
	}
	if err := loadExtensionProbe(t, parent, submodule("marker")); err != nil {
		t.Fatalf("Resolve with the parent's extension: %v", err)
	}
	err := loadExtensionProbe(t, parent, submodule("makrer"))
	if !errors.Is(err, ErrUndeclaredExtension) {
		t.Fatalf("Resolve answered %v, want ErrUndeclaredExtension", err)
	}
	assertNames(t, err, "module ze-probe-sub", "probe:makrer")
}

// TestDefaultLoaderSurfacesUndeclaredExtension: a registered module carrying
// `ze:hepl` makes DefaultLoader fail with ErrUndeclaredExtension. Method:
// append a probe to the package registry for this test only.
func TestDefaultLoaderSurfacesUndeclaredExtension(t *testing.T) {
	saved := modules
	t.Cleanup(func() { modules = saved })
	modules = append(slices.Clone(saved), Module{Name: "ze-probe.yang", Content: `module ze-probe {
    namespace "urn:ze:probe";
    prefix probe;
    import ze-extensions { prefix ze; }
    leaf probe {
        type string;
        ze:hepl "Probe leaf.";
    }
}`})
	loader, err := DefaultLoader()
	if !errors.Is(err, ErrUndeclaredExtension) {
		t.Fatalf("DefaultLoader answered (%v, %v), want ErrUndeclaredExtension", loader, err)
	}
	assertNames(t, err, "ze:hepl")
}
