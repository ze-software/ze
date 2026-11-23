// Design: third_party/vpp-patches/README.md -- one source pin for both recipes.
// Related: api.go -- generation lifecycle and api_publish.go -- output ownership.
package buildvpp

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// apiPins has no useful zero value; apiSourcePins constructs it only after the
// canonical Dockerfile supplies exactly one nonempty definition of each pin.
type apiPins struct{ commit, sha256 string }

func apiPatchesDir(root string) string {
	return filepath.Join(root, "third_party", "vpp-patches")
}

func apiSourcePins(root string) (apiPins, error) {
	file, err := os.Open(filepath.Join(apiPatchesDir(root), "Dockerfile")) //nolint:gosec // G304: path joins checkout root with fixed canonical Dockerfile path.
	if err != nil {
		return apiPins{}, err
	}
	defer file.Close() //nolint:errcheck // Read-only source input.
	values := make(map[string]string, 2)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		words := strings.Fields(scanner.Text())
		if len(words) == 0 {
			continue
		}
		if words[0] != "ARG" {
			continue
		}
		if len(words) < 2 {
			return apiPins{}, errors.New("the canonical VPP Dockerfile has an empty ARG")
		}
		name, value, defined := strings.Cut(words[1], "=")
		if name != "VPP_COMMIT" {
			if name != "VPP_SHA256" {
				continue
			}
		}
		if !defined {
			return apiPins{}, fmt.Errorf("the canonical VPP Dockerfile does not define %s", name)
		}
		if value == "" {
			return apiPins{}, fmt.Errorf("the canonical VPP Dockerfile defines an empty %s", name)
		}
		if len(words) > 2 {
			return apiPins{}, fmt.Errorf("the canonical VPP Dockerfile has extra tokens in %s", name)
		}
		if _, exists := values[name]; exists {
			return apiPins{}, fmt.Errorf("the canonical VPP Dockerfile defines %s more than once", name)
		}
		values[name] = value
	}
	if err := scanner.Err(); err != nil {
		return apiPins{}, err
	}
	for _, name := range []string{"VPP_COMMIT", "VPP_SHA256"} {
		if values[name] == "" {
			return apiPins{}, fmt.Errorf("the canonical VPP Dockerfile has no %s pin", name)
		}
	}
	return apiPins{commit: values["VPP_COMMIT"], sha256: values["VPP_SHA256"]}, nil
}

func apiBuildRecipe(root, work string, pins apiPins) (interoplab.ImageBuild, error) {
	patches, err := apiBaselinePatchNames(root)
	if err != nil {
		return interoplab.ImageBuild{}, err
	}
	directory := filepath.Join(work, "context")
	if err := os.Mkdir(directory, 0o750); err != nil {
		return interoplab.ImageBuild{}, err
	}
	for _, name := range append([]string{"Dockerfile.api"}, patches...) {
		data, err := os.ReadFile(filepath.Join(apiPatchesDir(root), name)) //nolint:gosec // G304: path joins checkout root's patch directory with Dockerfile.api or regular .patch entries vetted by apiBaselinePatchNames.
		if err != nil {
			return interoplab.ImageBuild{}, err
		}
		path := filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return interoplab.ImageBuild{}, err
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			return interoplab.ImageBuild{}, err
		}
	}
	return interoplab.ImageBuild{
		Name: apiImageName, Tag: apiImageName,
		Dockerfile: filepath.Join(directory, "Dockerfile.api"), Context: directory,
		BuildArgs: []string{"VPP_COMMIT=" + pins.commit, "VPP_SHA256=" + pins.sha256},
		Timeout:   10 * time.Minute, Required: true, Progress: os.Stderr,
	}, nil
}

// apiBaselinePatchNames follows the runtime Dockerfile's two lexical patch
// layers. Proof mutations live outside those layers and are never API inputs.
func apiBaselinePatchNames(root string) ([]string, error) {
	var names []string
	for _, layer := range []string{"", "ze"} {
		directory := filepath.Join(apiPatchesDir(root), layer)
		info, err := os.Lstat(directory)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("the VPP patch layer is not a directory: %s", directory)
		}
		entries, err := os.ReadDir(directory)
		if err != nil {
			return nil, err
		}
		first := len(names)
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".patch") {
				continue
			}
			if !entry.Type().IsRegular() {
				return nil, fmt.Errorf("the VPP patch is not a regular file: %s", filepath.Join(directory, entry.Name()))
			}
			names = append(names, filepath.Join(layer, entry.Name()))
		}
		if len(names) == first {
			return nil, fmt.Errorf("the VPP patch layer has no patches: %s", directory)
		}
	}
	return names, nil
}
