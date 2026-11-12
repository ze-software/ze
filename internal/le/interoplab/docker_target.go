// Design: docs/architecture/testing/interop.md -- typed Docker build parameters.
package interoplab

import "fmt"

// ImageBuild is a raw Docker request. Validate its optional stage before any
// process runs; a stage is one identifier, never an argument list.
func validateBuildTarget(target string) error {
	if target == "" {
		return nil
	}
	for index, char := range target {
		if char >= 'a' && char <= 'z' {
			continue
		}
		if char >= 'A' && char <= 'Z' {
			continue
		}
		if char >= '0' && char <= '9' {
			continue
		}
		if index > 0 {
			switch char {
			case '_', '.', '-':
				continue
			}
		}
		return fmt.Errorf("Docker build target %q is not a stage identifier; choose one named stage", target)
	}
	return nil
}
