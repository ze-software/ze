package testnetlab

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type imageContractRunner struct {
	archives    [][]byte
	sharedImage string
	images      map[string]string
}

func (r *imageContractRunner) Run(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	if command.Arguments[1] == "image" {
		if command.Arguments[2] == "inspect" {
			reference := r.images[command.Arguments[len(command.Arguments)-1]]
			if command.Arguments[4] == "{{.Id}}" {
				return interoplab.ProcessResult{Stdout: reference + "\n"}, nil
			}
			body, err := json.Marshal(map[string]any{"Id": reference})
			return interoplab.ProcessResult{Stdout: string(body)}, err
		}
	}
	if command.Arguments[1] != "build" {
		return interoplab.ProcessResult{}, nil
	}
	body, err := io.ReadAll(command.Stdin)
	if err != nil {
		return interoplab.ProcessResult{}, err
	}
	r.archives = append(r.archives, body)
	r.sharedImage = fmt.Sprintf("sha256:%064d", len(r.archives))
	if r.images == nil {
		r.images = make(map[string]string)
	}
	for index, word := range command.Arguments {
		if word == "-t" {
			r.images[command.Arguments[index+1]] = r.sharedImage
		}
	}
	return interoplab.ProcessResult{Stdout: r.sharedImage + "\n"}, nil
}

// VALIDATES: the live tools recipe is accepted by Docker.Build's actual tar
// context producer when the caller's working directory is outside the lab.
func TestLiveToolsBuildUsesItsPackagedDockerfile(t *testing.T) {
	directory := t.TempDir()
	writeFixtureFile(t, filepath.Join(directory, "Dockerfile.tools"), "FROM scratch\nCOPY le /le\n", 0o644)
	writeFixtureFile(t, filepath.Join(directory, "le"), "staged native le", 0o755)
	runner := &imageContractRunner{}
	docker := interoplab.NewFixtureDocker(runner)
	build := toolImageBuild(directory)
	build.Progress = nil
	image, err := docker.Build(context.Background(), build)
	if err != nil {
		t.Fatalf("packaged tools recipe rejected by real context producer: %v", err)
	}
	if image.Reference == "" {
		t.Fatal("accepted tools build has no immutable image")
	}
	if len(runner.archives) != 1 {
		t.Fatalf("build received %d contexts", len(runner.archives))
	}
	reader := tar.NewReader(bytes.NewReader(runner.archives[0]))
	var found bool
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Name == "Dockerfile.tools" {
			found = true
		}
	}
	if !found {
		t.Fatal("tools Dockerfile absent from actual build archive")
	}
}

// VALIDATES: each lab's generated daemon definition keeps its own immutable
// build result even after another run moves the shared netlab/ze:latest tag.
func TestLiveNodeImageSurvivesAnotherBuildMovingSharedTag(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	writeFixtureFile(t, filepath.Join(fixture.root, "contrib", "netlab", "ze.yml"), "daemon_config:\n  ze: /etc/ze/ze.conf\nclab:\n  image: netlab/ze:latest\n", 0o644)
	writeFixtureFile(t, filepath.Join(fixture.root, "docker", "Dockerfile.runtime"), "FROM scratch AS lab\nCOPY ze /usr/local/bin/ze\n", 0o644)
	runner := &imageContractRunner{}
	docker := interoplab.NewFixtureDocker(runner)
	var references []string
	var ownedTags []string
	var images []interoplab.ImageResult
	var directories []string
	for range 2 {
		directory := t.TempDir()
		if err := fixture.checker.buildLab(directory); err != nil {
			t.Fatal(err)
		}
		writeFixtureFile(t, filepath.Join(directory, "ze"), "already staged node binary", 0o755)
		image, err := buildLabNode(context.Background(), docker, fixture.checker, fixture.root, directory)
		if err != nil {
			t.Fatal(err)
		}
		references = append(references, image.Reference)
		ownedTags = append(ownedTags, image.OwnedTag())
		images = append(images, image)
		directories = append(directories, directory)
	}
	if references[0] == runner.sharedImage {
		t.Fatal("test did not move the shared image tag")
	}
	if err := checkNodeImageBinding(context.Background(), docker, images[0]); err != nil {
		t.Fatalf("shared tag movement disturbed the run-owned image: %v", err)
	}
	for index, directory := range directories {
		body, err := fixture.checker.FS.ReadFile(filepath.Join(directory, "topology-defaults.yml"))
		if err != nil {
			t.Fatal(err)
		}
		var defaults struct {
			Daemons struct {
				Ze struct {
					Clab struct {
						Image string `yaml:"image"`
					} `yaml:"clab"`
				} `yaml:"ze"`
			} `yaml:"daemons"`
		}
		if err := yaml.Unmarshal(body, &defaults); err != nil {
			t.Fatal(err)
		}
		selector := defaults.Daemons.Ze.Clab.Image
		if selector != ownedTags[index] {
			t.Fatalf("lab %d selects %q instead of its run-owned tag %q", index, selector, ownedTags[index])
		}
		if runner.images[selector] != references[index] {
			t.Fatalf("lab %d selector %q resolves to %q instead of its immutable build %q", index, selector, runner.images[selector], references[index])
		}
	}
	// Netlab validates repository/tag selectors with docker image ls. Resolve
	// that selector to content here, preserving the original immutable-ID proof
	// while exercising the actual upstream consumer's accepted selector form.
	runner.images[ownedTags[0]] = references[1]
	if err := checkNodeImageBinding(context.Background(), docker, images[0]); err == nil {
		t.Fatal("changed run-owned tag was accepted before live startup")
	}
}
