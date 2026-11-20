package credits

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// generatorPackage is the fixture's package that embeds the data update writes.
const generatorPackage = fixtureModule + "/" + creditsPackage

// listLine is one line as packageTemplate prints it.
func listLine(pkg, module, loadError string) string {
	quoted := ""
	if loadError != "" {
		quoted = strconv.Quote(loadError)
	}
	return pkg + "\t" + module + "\t" + quoted
}

// TestLinkedModulesAcceptsTheDataNotYetWritten asks whether the one load
// error update must survive, the generator package's embed pattern of the
// data directory finding nothing before the first update writes it, is
// accepted with every module still read. Method: a go list answer where the
// credits package carries that error and two packages of widget and one of
// gadget follow it.
func TestLinkedModulesAcceptsTheDataNotYetWritten(t *testing.T) {
	raw := strings.Join([]string{
		listLine(widget+"/a", widget, ""),
		listLine(generatorPackage, "", "pattern data: no matching files found"),
		listLine(gadget, gadget, ""),
		listLine(widget+"/b", widget, ""),
		listLine("fmt", "", ""),
	}, "\n") + "\n"
	modules, err := linkedModules([]byte(raw), generatorPackage)
	if err != nil {
		t.Fatalf("linkedModules refused the data directory not yet written: %v", err)
	}
	want := []string{gadget, widget}
	if !slices.Equal(modules, want) {
		t.Fatalf("linkedModules = %q, want %q", modules, want)
	}
}

// TestLinkedModulesRefusesAnyOtherLoadError asks whether every other load
// error is refused, naming the package and its error: a package that cannot
// be found, a syntax error in the generator package, and the same embed
// error in another package. Method: one go list answer per error.
func TestLinkedModulesRefusesAnyOtherLoadError(t *testing.T) {
	cases := []struct {
		pkg       string
		loadError string
	}{
		{widget + "/missing", "cannot find package \"" + widget + "/missing\""},
		{generatorPackage, "expected 'package', found 'EOF'"},
		{fixtureModule + "/internal/component/web", "pattern data: no matching files found"},
		{generatorPackage, "pattern texts: no matching files found"},
	}
	for _, item := range cases {
		raw := listLine(widget, widget, "") + "\n" + listLine(item.pkg, "", item.loadError) + "\n"
		_, err := linkedModules([]byte(raw), generatorPackage)
		if err == nil {
			t.Errorf("linkedModules accepted %s failing with %q", item.pkg, item.loadError)
			continue
		}
		if !strings.Contains(err.Error(), item.pkg) || !strings.Contains(err.Error(), item.loadError) {
			t.Errorf("refusal %q does not name %s and %q", err, item.pkg, item.loadError)
		}
	}
}
