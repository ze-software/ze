package credits

import (
	"slices"
	"strings"
	"testing"
)

// TestLicenceIdentifiersReadsAnOrExpression asks whether an expression joined
// by OR alone is read as its alternatives, in the order written. Method: the
// expression filippo.io/mlockexe declares.
func TestLicenceIdentifiersReadsAnOrExpression(t *testing.T) {
	identifiers, err := licenceIdentifiers("CC0-1.0 OR 0BSD")
	if err != nil {
		t.Fatalf("licenceIdentifiers refused an OR expression: %v", err)
	}
	want := []string{"CC0-1.0", "0BSD"}
	if !slices.Equal(identifiers, want) {
		t.Fatalf("licenceIdentifiers = %q, want %q", identifiers, want)
	}
}

// TestLicenceIdentifiersRefusesAnExpressionItCannotRead asks whether an
// expression the check would only half read is refused rather than judged:
// OR beside AND, parentheses, a trailing "+", WITH, and an operator with no
// identifier on one side. Method: one expression of each shape.
func TestLicenceIdentifiersRefusesAnExpressionItCannotRead(t *testing.T) {
	for _, expression := range []string{
		"MIT OR Apache-2.0 AND BSD-3-Clause",
		"(MIT OR Apache-2.0)",
		"GPL-2.0+",
		"Apache-2.0 WITH LLVM-exception",
		"MIT OR",
		"OR MIT",
		"",
	} {
		if identifiers, err := licenceIdentifiers(expression); err == nil {
			t.Errorf("licenceIdentifiers(%q) = %q, want a refusal", expression, identifiers)
		}
	}
}

// TestHolderLineRefusesTheWordBeforeAName asks whether a line where
// "Copyright" is followed by a capitalized word, with no year and no
// (c)/© mark, is read as prose rather than a holder. Method: the line
// third_party/iana/README.md writes when it quotes the title of IANA's
// statement.
func TestHolderLineRefusesTheWordBeforeAName(t *testing.T) {
	text := "the page \"Licensing Terms\", section \"Joint Statement of IANA and IETF Concerning\n" +
		"Copyright Rights in the Protocol Registries dated 10 November 2021\". It says\n"
	if line, stated := holderLine(text); stated {
		t.Fatalf("holderLine read %q as a holder line", line)
	}
}

// TestHolderLineAcceptsANoticeWithAYearOrAMark asks whether the two shapes
// of a copyright notice are still read as holder lines: the word followed by
// a year, and the word followed by a (c) mark and a name. Method: one line
// of each shape.
func TestHolderLineAcceptsANoticeWithAYearOrAMark(t *testing.T) {
	for _, text := range []string{
		"Copyright 2019 The Thing Authors. All rights reserved.",
		"Copyright (c) Acme Widgets",
	} {
		if _, stated := holderLine(text); !stated {
			t.Errorf("holderLine did not read %q as a holder line", text)
		}
	}
}

// TestCheckRefusesAnOrAlternativeTheTextDoesNotState asks whether an OR
// expression one of whose alternatives the work's files do not state is
// refused, naming that alternative: an OR records the choice the licence
// offers, so the text states each one. Method: widget's LICENSE states MIT
// only, and the declaration says MIT OR Apache-2.0.
func TestCheckRefusesAnOrAlternativeTheTextDoesNotState(t *testing.T) {
	files := fixtureFiles()
	files[DeclarationRel] = strings.Replace(declarationJSON("Acme Widgets"), `"licence": "MIT",`, `"licence": "MIT OR Apache-2.0",`, 1)
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	requireRefusal(t, check(t, root, in), widget, "none of "+widgetLicence+" states Apache-2.0")
}

// TestBannerIsClaimedByADeclaredPhrase asks whether a banner that does not
// name its library is claimed by the work that declares a phrase the banner
// writes, and by nothing else. Method: deep-extend's banner names no package;
// it is claimed while the work declares the phrase, and left unclaimed once
// the same work declares none.
func TestBannerIsClaimedByADeclaredPhrase(t *testing.T) {
	found := banners("/*! thing 1.0 | BSD-3-Clause */\n" +
		"/*!\n * @description Recursive object extending\n * @license MIT\n */\n")
	claimants := []entry{{name: thing}, {name: "deep-extend", banner: "Recursive object extending"}}
	if left := unclaimedBanners(found, claimants); len(left) != 0 {
		t.Fatalf("banners left unclaimed with the phrase declared: %+v", left)
	}
	claimants[1].banner = ""
	left := unclaimedBanners(found, claimants)
	if len(left) != 1 || left[0].line != 2 {
		t.Fatalf("without the phrase, unclaimed = %+v, want the banner on line 2 alone", left)
	}
}

// TestCheckRefusesABannerPhraseNoBannerWrites asks whether a work that
// declares a banner phrase none of its companion file's banners writes is
// refused, naming the work, the phrase and the file. Method: thing lists a
// companion file whose one banner names thing, and declares a phrase that
// banner does not hold.
func TestCheckRefusesABannerPhraseNoBannerWrites(t *testing.T) {
	files := fixtureFiles()
	files[thingBundle] = "/*! thing 1.0 | BSD-3-Clause | The Thing Authors */\n"
	files[DeclarationRel] = strings.Replace(declarationJSON("Acme Widgets"),
		`"files": ["third_party/thing/LICENSE"]`,
		`"banner": "A phrase nobody wrote", "files": ["third_party/thing/LICENSE", "`+thingBundle+`"]`, 1)
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	requireRefusal(t, check(t, root, in), thing,
		"declares the banner phrase \"A phrase nobody wrote\", which no banner of "+thingBundle+" writes")
}

// TestCheckReadsAModulesLicenceFiles asks whether a module's declared
// licence-files join the files its licence is checked against and copied
// from: filippo.io/mlockexe states CC0 in its README, beside its LICENSE.
// Method: widget's LICENSE states MIT, its README states CC0 1.0, and the
// declaration says MIT OR CC0-1.0 with the README under licence-files; the
// check refuses nothing and the record lists the README after the LICENSE.
func TestCheckReadsAModulesLicenceFiles(t *testing.T) {
	readme := "vendor/" + widget + "/README.md"
	files := fixtureFiles()
	files[readme] = "# widget\n\nThis work is marked CC0 1.0 Universal.\n"
	files[DeclarationRel] = strings.Replace(declarationJSON("Acme Widgets"), `"licence": "MIT",`,
		`"licence": "MIT OR CC0-1.0", "licence-files": ["`+readme+`"],`, 1)
	root := writeTree(t, files)
	in := fixtureInputs()
	generate(t, root, in)
	if report := check(t, root, in); len(report.Refusals) != 0 {
		t.Fatalf("check refused a licence its licence-files state:\n%s", report.Text())
	}
	out, refusals, err := planTree(root, in)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(refusals) != 0 {
		t.Fatalf("plan refused the fixture: %+v", refusals)
	}
	work, held := out.record.work(widget)
	if !held {
		t.Fatalf("no record of %s", widget)
	}
	origins := make([]string, 0, len(work.Files))
	for _, file := range work.Files {
		origins = append(origins, file.Origin)
	}
	want := []string{widgetLicence, readme}
	if !slices.Equal(origins, want) {
		t.Fatalf("%s copies %q, want %q", widget, origins, want)
	}
}
