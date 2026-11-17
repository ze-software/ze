// Design: docs/architecture/container/image-store.md -- resolve: the image configuration in the ready answer
// Related: resolve.go -- the resolve that hands it to the runtime
// Related: publish.go -- the image folder that holds config.json byte for byte

package imagestore

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/plugins/container"
	"github.com/ze-software/ze/internal/plugins/container/registry"
)

// The bounds of an image configuration read back from an image folder.
const (
	// imageConfigSizeMax is the bound the registry client applied when it
	// fetched the configuration: a larger file was not written by the fetch.
	imageConfigSizeMax = 4 << 20
	// imageConfigListMax bounds each list of the configuration: the
	// entrypoint, the command, the environment, the exposed ports and the
	// declared volumes.
	imageConfigListMax = 4096
	// imageConfigSystem is the one operating system a container runs.
	imageConfigSystem = "linux"
)

// imageConfiguration is the part of an OCI image configuration the ready
// answer carries. The field names are the image format's, not Ze's.
type imageConfiguration struct {
	System       string               `json:"os"`
	Architecture string               `json:"architecture"`
	Process      processConfiguration `json:"config"`
}

// processConfiguration is the process the image declares. An image that
// declares none leaves every field empty, which the format allows: the
// runtime then takes the process from the container's config.
type processConfiguration struct {
	User         string              `json:"User"`
	ExposedPorts map[string]struct{} `json:"ExposedPorts"`
	Environment  []string            `json:"Env"`
	Entrypoint   []string            `json:"Entrypoint"`
	Command      []string            `json:"Cmd"`
	Volumes      map[string]struct{} `json:"Volumes"`
	WorkingDir   string              `json:"WorkingDir"`
	StopSignal   string              `json:"StopSignal"`
}

// readImageConfiguration reads config.json of the image folder digest names
// into the fields of a ready answer. A configuration over its bound, not
// JSON, of another system or architecture than architecture, or with a value
// no process can carry, is refused: it is never answered as empty fields.
func (r *storeRoot) readImageConfiguration(digest registry.Digest, architecture registry.Architecture) (container.Ready, error) {
	folder := filepath.Join(imagesFolder, digestFolder(digest))
	path := filepath.Join(r.path, folder, configName)
	file, err := r.root.Open(filepath.Join(folder, configName))
	if err != nil {
		return container.Ready{}, configUnusable(path, "the completion mark vouches for it, and it cannot be opened", err)
	}
	content, err := io.ReadAll(io.LimitReader(file, imageConfigSizeMax+1))
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return container.Ready{}, configUnusable(path, "the completion mark vouches for it, and it cannot be read", err)
	}
	ready, err := parseImageConfiguration(content, architecture)
	if err != nil {
		return container.Ready{}, configUnusable(path, "the runtime starts the container's process from it", err)
	}
	return ready, nil
}

// parseImageConfiguration checks content and answers its fields.
func parseImageConfiguration(content []byte, architecture registry.Architecture) (container.Ready, error) {
	if len(content) > imageConfigSizeMax {
		return container.Ready{}, errors.New("the configuration is larger than 4 MiB")
	}
	var configured imageConfiguration
	if err := json.NewDecoder(bytes.NewReader(content)).Decode(&configured); err != nil {
		return container.Ready{}, err
	}
	if configured.System != imageConfigSystem {
		return container.Ready{}, errors.New("the image is for the system " + strconv.Quote(configured.System) + ", and containers run on linux")
	}
	if configured.Architecture != architecture.String() {
		return container.Ready{}, errors.New("the image is for the architecture " + strconv.Quote(configured.Architecture) + ", and this system runs " + architecture.String())
	}
	process := configured.Process
	if err := checkProcessConfiguration(&process); err != nil {
		return container.Ready{}, err
	}
	return container.Ready{
		Entrypoint:       process.Entrypoint,
		Command:          process.Command,
		Environment:      process.Environment,
		WorkingDirectory: process.WorkingDir,
		User:             process.User,
		StopSignal:       process.StopSignal,
		ExposedPorts:     slices.Sorted(maps.Keys(process.ExposedPorts)),
		DeclaredVolumes:  slices.Sorted(maps.Keys(process.Volumes)),
	}, nil
}

// checkProcessConfiguration refuses a list over its bound, a variable that
// is not NAME=value, and a NUL byte, which no argument, variable or path of a
// process can carry.
func checkProcessConfiguration(process *processConfiguration) error {
	lists := []struct {
		name   string
		length int
	}{
		{"Entrypoint", len(process.Entrypoint)},
		{"Cmd", len(process.Command)},
		{"Env", len(process.Environment)},
		{"ExposedPorts", len(process.ExposedPorts)},
		{"Volumes", len(process.Volumes)},
	}
	for _, list := range lists {
		if list.length > imageConfigListMax {
			return errors.New(list.name + " holds " + strconv.Itoa(list.length) + " entries, more than " + strconv.Itoa(imageConfigListMax))
		}
	}
	for _, variable := range process.Environment {
		name, _, found := strings.Cut(variable, "=")
		if !found {
			return errors.New("the variable " + strconv.Quote(variable) + " is not NAME=value")
		}
		if name == "" {
			return errors.New("the variable " + strconv.Quote(variable) + " has no name")
		}
	}
	texts := slices.Concat(process.Entrypoint, process.Command, process.Environment,
		[]string{process.User, process.WorkingDir, process.StopSignal})
	for _, text := range texts {
		if strings.ContainsRune(text, 0) {
			return errors.New("the value " + strconv.Quote(text) + " holds a NUL byte")
		}
	}
	return nil
}

// configUnusable is the refusal of an image configuration the store cannot
// hand to the runtime. The image folder is the store's, so the fix removes
// it, and the next fetch of its reference publishes it again.
func configUnusable(path, why string, cause error) error {
	return opererr.Wrap(cause,
		"the image configuration "+path+" cannot be handed to the container runtime",
		why,
		"rm -rf "+filepath.Dir(path),
	)
}
