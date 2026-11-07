// Design: docs/architecture/testing/interop.md -- images registered by name.
// Related: lab.go -- Suite.Images, which a suite derives from this registry.
package interoplab

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// ImageContext is what an image declaration needs to name its build: the
// checkout its Dockerfile and context live under.
type ImageContext struct {
	Root string
}

// RegisteredImage is one image a suite declares by a name unique across every
// suite, so a scenario of another suite (the topology suite) names it and the
// image is built from the one declaration.
type RegisteredImage struct {
	// Name is the image's logical name: what a PeerConfig.Image names and
	// what ImageBuild.Name carries.
	Name string
	// Owner is the suite that declares it, named in a duplicate refusal.
	Owner string
	// Build answers the image's build declaration for a checkout. Its Name
	// MUST be Name.
	Build func(ImageContext) ImageBuild
	// Binaries are the lab binaries the build context copies, which the
	// suite that runs the image stages before any build (StageBinaries).
	Binaries []LabBinary
}

var (
	imageRegistryMu sync.Mutex
	imageRegistry   []RegisteredImage
)

// RegisterImage adds image to the registry. It is called from init() in a
// suite's register file, so a declaration that cannot be served is a Ze
// defect and panics: an empty name, owner or build, or a name another suite
// already registered, naming both owners.
func RegisterImage(image RegisteredImage) {
	if image.Name == "" {
		panic("BUG: interoplab.RegisterImage without a name, from " + image.Owner)
	}
	if image.Owner == "" {
		panic("BUG: interoplab.RegisterImage " + image.Name + " without an owner")
	}
	if image.Build == nil {
		panic("BUG: interoplab.RegisterImage " + image.Name + " from " + image.Owner + " without a build")
	}
	imageRegistryMu.Lock()
	defer imageRegistryMu.Unlock()
	for index := range imageRegistry {
		if imageRegistry[index].Name == image.Name {
			panic("BUG: interoplab.RegisterImage " + image.Name + " is registered by both " +
				imageRegistry[index].Owner + " and " + image.Owner + "; give one of them another name")
		}
	}
	imageRegistry = append(imageRegistry, image)
}

// SuiteImages answers the builds of every image owner registered, in
// registration order: the list that suite's own runner prepares.
func SuiteImages(owner string, context ImageContext) []ImageBuild {
	imageRegistryMu.Lock()
	defer imageRegistryMu.Unlock()
	var builds []ImageBuild
	for index := range imageRegistry {
		if imageRegistry[index].Owner == owner {
			builds = append(builds, imageBuild(&imageRegistry[index], context))
		}
	}
	return builds
}

// ResolveImages answers the builds of the images names name, once each, and
// the lab binaries those builds copy, for a suite that runs images other
// suites declare. A name nobody registered is refused, naming every
// registered image, before any container or build runs.
func ResolveImages(context ImageContext, names []string) ([]ImageBuild, []LabBinary, error) {
	imageRegistryMu.Lock()
	defer imageRegistryMu.Unlock()
	var builds []ImageBuild
	var binaries []LabBinary
	for _, name := range names {
		if slices.ContainsFunc(builds, func(build ImageBuild) bool { return build.Name == name }) {
			continue
		}
		index := slices.IndexFunc(imageRegistry, func(image RegisteredImage) bool { return image.Name == name })
		if index < 0 {
			return nil, nil, unknownImage(name)
		}
		builds = append(builds, imageBuild(&imageRegistry[index], context))
		for _, binary := range imageRegistry[index].Binaries {
			if !slices.ContainsFunc(binaries, func(staged LabBinary) bool { return staged.Output == binary.Output }) {
				binaries = append(binaries, binary)
			}
		}
	}
	return builds, binaries, nil
}

// imageBuild answers image's build and refuses a declaration whose build
// carries another name, because a peer naming the image would then resolve
// to nothing.
func imageBuild(image *RegisteredImage, context ImageContext) ImageBuild {
	build := image.Build(context)
	if build.Name != image.Name {
		panic("BUG: registered image " + image.Name + " from " + image.Owner + " builds as " + build.Name)
	}
	return build
}

// unknownImage names a missing image and every registered one. The caller
// holds imageRegistryMu.
func unknownImage(name string) error {
	names := make([]string, 0, len(imageRegistry))
	for index := range imageRegistry {
		names = append(names, imageRegistry[index].Name)
	}
	slices.Sort(names)
	var reason textbuf.Buffer
	reason.Str("no suite registers an image named ").Str(name).Str("; name one of the registered images (").
		Str(strings.Join(names, ", ")).Str("), or register it with interoplab.RegisterImage in its suite's register file")
	return errors.New(reason.String())
}
