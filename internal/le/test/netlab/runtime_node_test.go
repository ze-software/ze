// Design: docs/guide/netlab.md -- reuse the node binary staged for this lab.
package testnetlab

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// TestLiveNodeImageUsesOnlyItsStagedBinary reads Docker.Build's actual input
// stream, so a full-checkout build or a second source build cannot pass.
func TestLiveNodeImageUsesOnlyItsStagedBinary(t *testing.T) {
	fixture := newCheckerFixture(t, fixtureConfig)
	writeFixtureFile(t, filepath.Join(fixture.root, "contrib", "netlab", "ze.yml"), "daemon_config:\n  ze: /etc/ze/ze.conf\nclab:\n  image: netlab/ze:latest\n", 0o644)
	writeFixtureFile(t, filepath.Join(fixture.root, "docker", "Dockerfile.runtime"), "FROM scratch AS lab\nCOPY ze /usr/local/bin/ze\n", 0o644)
	writeFixtureFile(t, filepath.Join(fixture.root, "docker", "Dockerfile.lab"), "FROM scratch\nCOPY . .\n", 0o644)
	writeFixtureFile(t, filepath.Join(fixture.root, "test", "interop-stale", "ze-linux"), "another lab's binary", 0o755)
	directory := t.TempDir()
	if err := fixture.checker.buildLab(directory); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(directory, "ze"), "this lab's staged node", 0o755)
	writeFixtureFile(t, filepath.Join(directory, "le"), "tools binary excluded from node", 0o755)
	runner := &imageContractRunner{}
	if _, err := buildLabNode(t.Context(), interoplab.NewFixtureDocker(runner), fixture.checker, fixture.root, directory); err != nil {
		t.Fatal(err)
	}
	if len(runner.archives) != 1 {
		t.Fatalf("node image received %d contexts", len(runner.archives))
	}
	entries := make(map[string]string)
	reader := tar.NewReader(bytes.NewReader(runner.archives[0]))
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		entries[header.Name] = string(body)
	}
	if len(entries) != 2 || entries["ze"] != "this lab's staged node" || entries["Dockerfile"] != "FROM scratch AS lab\nCOPY ze /usr/local/bin/ze\n" {
		t.Fatalf("node image did not reuse its minimal staged input: %v", entries)
	}
}
