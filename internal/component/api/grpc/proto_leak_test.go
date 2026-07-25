package grpc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoProtoLeakOutsideGRPC(t *testing.T) {
	apiRoot := ".."
	entries, err := os.ReadDir(apiRoot)
	if err != nil {
		t.Fatalf("read api dir: %v", err)
	}

	for _, entry := range entries {
		if entry.Name() == "grpc" || !entry.IsDir() {
			continue
		}
		subdir := filepath.Join(apiRoot, entry.Name())
		checkDirForProtoImport(t, subdir, entry.Name())
	}

	// Also check top-level api package files.
	topFiles, _ := filepath.Glob(filepath.Join(apiRoot, "*.go"))
	for _, f := range topFiles {
		checkFileForProtoImport(t, f)
	}
}

func checkDirForProtoImport(t *testing.T, dir, _ string) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return
	}
	for _, f := range files {
		checkFileForProtoImport(t, f)
	}
}

func checkFileForProtoImport(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if strings.Contains(string(data), `"github.com/ze-software/ze/api/proto"`) {
		t.Errorf("proto import leaked outside grpc/: %s", path)
	}
	if strings.Contains(string(data), `zepb "`) {
		t.Errorf("zepb alias found outside grpc/: %s", path)
	}
}
