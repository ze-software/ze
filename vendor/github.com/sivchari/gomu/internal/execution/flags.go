package execution

import "strings"

// buildValueFlags are "go build" flags that take a value, either as
// "-flag=value" or as a separate "-flag value" argument.
var buildValueFlags = map[string]bool{
	"tags":          true,
	"mod":           true,
	"modfile":       true,
	"gcflags":       true,
	"ldflags":       true,
	"asmflags":      true,
	"gccgoflags":    true,
	"buildmode":     true,
	"compiler":      true,
	"installsuffix": true,
	"pgo":           true,
	"pkgdir":        true,
	"toolexec":      true,
	"covermode":     true,
	"coverpkg":      true,
	"p":             true,
}

// buildBoolFlags are "go build" flags that take no value.
var buildBoolFlags = map[string]bool{
	"race":       true,
	"msan":       true,
	"asan":       true,
	"cover":      true,
	"trimpath":   true,
	"linkshared": true,
	"modcacherw": true,
	"a":          true,
	"n":          true,
	"v":          true,
	"x":          true,
	"work":       true,
	"buildvcs":   true,
}

// splitBuildFlags returns the subset of testArgs that are also valid "go
// build" flags, since go build rejects test-only flags such as -short or
// -run.
func splitBuildFlags(testArgs []string) []string {
	var buildArgs []string

	for i := 0; i < len(testArgs); i++ {
		arg := testArgs[i]

		name, hasValue := parseFlagName(arg)
		if name == "" {
			continue
		}

		switch {
		case buildValueFlags[name]:
			if hasValue {
				buildArgs = append(buildArgs, arg)

				continue
			}

			buildArgs = append(buildArgs, arg)

			if i+1 < len(testArgs) {
				i++

				buildArgs = append(buildArgs, testArgs[i])
			}
		case buildBoolFlags[name]:
			buildArgs = append(buildArgs, arg)
		}
	}

	return buildArgs
}

// parseFlagName extracts the flag name (without leading dashes) from a
// "-flag", "--flag" or "-flag=value" argument, and reports whether it carries
// an inline value. It returns an empty name for anything that is not a flag.
func parseFlagName(arg string) (name string, hasValue bool) {
	if !strings.HasPrefix(arg, "-") {
		return "", false
	}

	trimmed := strings.TrimPrefix(strings.TrimPrefix(arg, "-"), "-")
	if trimmed == "" || strings.HasPrefix(trimmed, "-") {
		return "", false
	}

	if idx := strings.Index(trimmed, "="); idx >= 0 {
		return trimmed[:idx], true
	}

	return trimmed, false
}
