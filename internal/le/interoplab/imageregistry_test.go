package interoplab

import (
	"strings"
	"testing"
)

// fixtureImage registers name for owner, built from a Dockerfile under root.
func fixtureImage(name, owner string) RegisteredImage {
	return RegisteredImage{Name: name, Owner: owner,
		Build: func(context ImageContext) ImageBuild {
			return ImageBuild{Name: name, Tag: name + "-tag", Dockerfile: context.Root + "/Dockerfile." + name}
		},
		Binaries: []LabBinary{{Name: "ze", Output: "test/" + owner + "/ze-linux"}}}
}

// VALIDATES: two suites registering one image name panic at init, naming
// both owners (AC-10).
// Method: register a fixture name twice and recover the panic.
// PREVENTS: a scenario resolving a name to whichever suite registered last.
func TestImageRegistryDuplicatePanics(t *testing.T) {
	RegisterImage(fixtureImage("fixture-duplicate", "suite-one"))
	defer func() {
		recovered := recover()
		text, ok := recovered.(string)
		if !ok {
			t.Fatalf("recovered %v, want a BUG panic", recovered)
		}
		if !strings.Contains(text, "suite-one") || !strings.Contains(text, "suite-two") {
			t.Fatalf("panic %q does not name both owners", text)
		}
	}()
	RegisterImage(fixtureImage("fixture-duplicate", "suite-two"))
}

// VALIDATES: a scenario naming an unregistered image is refused at plan time,
// naming the registered images (AC-9); a registered one resolves once to its
// declaration with the binaries its build copies (AC-8); and a suite's list is
// exactly what it registered.
// Method: register two fixture images and resolve known and unknown names.
// PREVENTS: a topology scenario building a private copy of another suite's
// image, or failing late at docker run with an unnamed image.
func TestPlanRefusesUnknownImage(t *testing.T) {
	RegisterImage(fixtureImage("fixture-resolve-a", "suite-resolve"))
	RegisterImage(fixtureImage("fixture-resolve-b", "suite-resolve"))
	context := ImageContext{Root: "/checkout"}
	_, _, err := ResolveImages(context, []string{"fixture-resolve-a", "fixture-missing"})
	if err == nil || !strings.Contains(err.Error(), "no suite registers an image named fixture-missing") ||
		!strings.Contains(err.Error(), "fixture-resolve-b") {
		t.Fatalf("error = %v, want a refusal naming the missing image and the registered ones", err)
	}
	builds, binaries, err := ResolveImages(context, []string{"fixture-resolve-a", "fixture-resolve-a", "fixture-resolve-b"})
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 2 || builds[0].Dockerfile != "/checkout/Dockerfile.fixture-resolve-a" {
		t.Fatalf("builds = %+v, want each image once from its declaration", builds)
	}
	if len(binaries) != 1 {
		t.Fatalf("binaries = %+v, want the shared binary staged once", binaries)
	}
	if listed := SuiteImages("suite-resolve", context); len(listed) != 2 || listed[1].Name != "fixture-resolve-b" {
		t.Fatalf("suite images = %+v, want both in registration order", listed)
	}
}
