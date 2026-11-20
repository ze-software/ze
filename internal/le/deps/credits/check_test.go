package credits

import (
	"strings"
	"testing"
)

// TestCheckRefusesALinkedModuleWithNoCredit asks whether a module a shipped
// flavor links, and the declaration does not name, is refused by name with
// the flavors that link it (AC-10). Method: the fixture's data is generated,
// then the linked sets gain gadget on appliance/linux only.
func TestCheckRefusesALinkedModuleWithNoCredit(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	generate(t, root, fixtureInputs())
	links := fixtureInputs(
		flavorLinks{Flavor: "distro/linux", Modules: []string{widget}},
		flavorLinks{Flavor: "appliance/linux", Modules: []string{gadget, widget}},
	)
	requireRefusal(t, check(t, root, links), gadget, "linked into the shipped binary by appliance/linux and has no entry")
}

// TestCheckRefusesAStaleLicenceCopy asks whether a vendored licence file that
// changed after the data was generated is refused, naming the work and the
// file (AC-11). Method: generate, then append a line to widget's LICENSE.
func TestCheckRefusesAStaleLicenceCopy(t *testing.T) {
	root := writeTree(t, fixtureFiles())
	in := fixtureInputs()
	generate(t, root, in)
	writeFile(t, root, widgetLicence, mitText+"An added line.\n")
	requireRefusal(t, check(t, root, in), widget, widgetLicence)
}

// TestCheckRefusesACopyrightNotInTheText asks whether a declared holder that
// the work's licence file does not contain is refused, quoting the declared
// string and naming the file read (AC-12). Method: declare widget's holder
// as a name its LICENSE never spells, and regenerate so only the holder is
// wrong.
func TestCheckRefusesACopyrightNotInTheText(t *testing.T) {
	files := fixtureFiles()
	files[DeclarationRel] = declarationJSON("Acme Gadgets Ltd")
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	report := check(t, root, in)
	requireRefusal(t, report, widget, "\"Acme Gadgets Ltd\", which none of "+widgetLicence+" contains")
}

// TestCheckRefusesADeclaredWorkNobodyCarries asks whether a works entry whose
// carried-by package does not call credits.DeclarePresent for it is refused
// (R-3). Method: the package's source keeps the import and drops the call.
func TestCheckRefusesADeclaredWorkNobodyCarries(t *testing.T) {
	files := fixtureFiles()
	files[webFile] = strings.Replace(webSource, `credits.DeclarePresent("thing")`, `_ = credits.ErrNoData`, 1)
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	requireRefusal(t, check(t, root, in), thing, "lists "+webPackage+" under carried-by, and that package does not declare it present")
}

// TestCheckRefusesABundledLibraryWithNoWork asks whether a library banner in
// a bundle's companion licence file that no works entry claims is refused,
// quoting the banner (R-7). Method: thing ships a companion file with its own
// banner, which its name claims, and a second library's banner nothing
// claims.
func TestCheckRefusesABundledLibraryWithNoWork(t *testing.T) {
	files := fixtureFiles()
	files[thingBundle] = "/*! thing 1.0 | BSD-3-Clause | The Thing Authors */\n\n" +
		"/**\n * @license Lodash <https://lodash.com/>\n * Copyright OpenJS Foundation and other contributors\n * Released under MIT license\n */\n"
	files[DeclarationRel] = strings.Replace(declarationJSON("Acme Widgets"),
		`"files": ["third_party/thing/LICENSE"]`, `"files": ["third_party/thing/LICENSE", "`+thingBundle+`"]`, 1)
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	requireRefusal(t, check(t, root, in), thingBundle+":3", "bundles a library no works entry claims: \"Lodash <https://lodash.com/>\"")
}
