// Design: docs/guide/netlab.md -- bind a generated lab to its own image content.
package testnetlab

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ze-software/ze/internal/le/interoplab"
)

func toolImageBuild(directory string) interoplab.ImageBuild {
	return interoplab.ImageBuild{
		Name: "ze netlab tools", Tag: "ze-netlab-tools:local",
		Dockerfile: filepath.Join(directory, "Dockerfile.tools"), Context: directory,
		Required: true, Progress: os.Stderr,
	}
}

// buildLabNode returns only after the generated daemon definition names the
// retained tag of this build. Netlab's image validation filters repository/tag
// names; a raw image ID does not satisfy that upstream consumer contract.
func buildLabNode(ctx context.Context, docker *interoplab.Docker, checker *Checker, root, directory string) (image interoplab.ImageResult, err error) {
	work, err := os.MkdirTemp(directory, ".runtime-node-")
	if err != nil {
		return image, err
	}
	// StageRuntimeImage's context belongs only to this build and MUST be
	// removed after Docker has consumed the already staged node binary.
	defer func() {
		cleanupErr := os.RemoveAll(work)
		if cleanupErr == nil {
			return
		}
		if err == nil {
			// The caller registers image cleanup only on success. If local
			// staging cleanup changes that outcome, this function still owns it.
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
			defer cancel()
			cleanupErr = errors.Join(cleanupErr, docker.ReleaseImage(cleanup, image))
		}
		err = errors.Join(err, cleanupErr)
	}()
	build, err := interoplab.StageRuntimeImage(root, filepath.Join(work, "context"), filepath.Join(directory, "ze"), "lab")
	if err != nil {
		return image, err
	}
	build.Name = "ze netlab node"
	build.Tag = "netlab/ze:latest"
	build.Required = true
	build.Progress = os.Stderr
	image, err = docker.Build(ctx, build)
	if err != nil {
		return image, err
	}
	if err := checker.bindNodeImage(directory, image); err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		return image, errors.Join(err, docker.ReleaseImage(cleanup, image))
	}
	return image, nil
}

func (c *Checker) bindNodeImage(directory string, image interoplab.ImageResult) error {
	reference := image.Reference
	const prefix = "sha256:"
	if !strings.HasPrefix(reference, prefix) {
		return errors.New("live node build returned no immutable sha256 image ID")
	}
	var digest [32]byte
	if len(reference) != len(prefix)+hex.EncodedLen(len(digest)) {
		return errors.New("live node build returned an invalid image ID length")
	}
	if _, err := hex.Decode(digest[:], []byte(reference[len(prefix):])); err != nil {
		return fmt.Errorf("decode immutable live node image: %w", err)
	}
	if image.OwnedTag() == "" {
		return errors.New("live node build has no retained run-owned tag")
	}
	path := filepath.Join(directory, "topology-defaults.yml")
	body, err := c.FS.ReadFile(path)
	if err != nil {
		return err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(body, &document); err != nil {
		return err
	}
	if len(document.Content) != 1 {
		return errors.New("live lab defaults have no document mapping")
	}
	node := document.Content[0]
	for _, key := range []string{"daemons", "ze", "clab", "image"} {
		node, err = yamlMappingValue(node, key)
		if err != nil {
			return err
		}
	}
	if node.Kind != yaml.ScalarNode {
		return errors.New("live lab daemon image is not a scalar")
	}
	node.Value = image.OwnedTag()
	node.Tag = "!!str"
	body, err = yaml.Marshal(&document)
	if err != nil {
		return err
	}
	return c.FS.WriteFile(path, body, 0o644)
}

func checkNodeImageBinding(ctx context.Context, docker *interoplab.Docker, image interoplab.ImageResult) error {
	if image.OwnedTag() == "" {
		return errors.New("live node build has no retained run-owned tag")
	}
	inspection, err := docker.InspectImage(ctx, image.OwnedTag())
	if err != nil {
		return fmt.Errorf("inspect retained live node image: %w", err)
	}
	if inspection.ID != image.Reference {
		return fmt.Errorf("retained live node image %s changed: got %s, built %s", image.OwnedTag(), inspection.ID, image.Reference)
	}
	return nil
}

func yamlMappingValue(node *yaml.Node, key string) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("live lab defaults %s parent is not a mapping", key)
	}
	for index := 0; index+1 < len(node.Content); index += 2 {
		if node.Content[index].Value == key {
			return node.Content[index+1], nil
		}
	}
	return nil, fmt.Errorf("live lab defaults are missing %s", key)
}
