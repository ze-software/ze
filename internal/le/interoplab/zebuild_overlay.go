// Design: docs/architecture/testing/interop.md -- source inputs belong to one staged lab binary.
// Related: zebuild.go -- stageBinary owns preparation, compilation and release.
// Upstream: https://github.com/golang/go/blob/go1.27.4/src/cmd/go/internal/fsys/fsys.go -- Replace paths resolve against the compiler's working directory; an empty backing path deletes a file.
// Upstream: https://github.com/anmitsu/go-shlex -- v0.0.0-20200514113438-38f4b401e2be, Split reads quoted GOFLAGS without executing them.
package interoplab

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/anmitsu/go-shlex"

	"github.com/ze-software/ze/internal/core/opererr"
)

const (
	// The overlay contains paths, not source bodies. This bounds an input
	// supplied through GOFLAGS before it is decoded into a replacement map.
	labOverlayBytesMax    = 4 << 20
	labBuildFlagsBytesMax = 64 << 10
	labBuildInputsFix     = "./le --name session test unit pkg path ./internal/le/interoplab/ run '^TestLabBinary'"
)

// prepareLabBinaryOverlay preserves inherited compiler flags and prepares only
// this binary's optional source substitution. It changes no process environment
// and is safe for concurrent use when the declaring callback is too.
//
// On success, stageBinary MUST call a non-nil cleanup after compilation. On
// failure, this function releases any inputs already prepared, and returns no
// cleanup for the caller to run a second time.
func prepareLabBinaryOverlay(root string, environment []string, binary LabBinary) (path string, cleanup func() error, resultErr error) {
	if binary.PrepareOverlay == nil {
		// No instrumentation is requested; the compiler keeps its original
		// GOFLAGS, including any recorder-provided overlay.
		return "", nil, nil
	}
	path, cleanup, resultErr = binary.PrepareOverlay(root)
	defer func() {
		if resultErr == nil {
			return
		}
		if cleanup != nil {
			resultErr = errors.Join(resultErr, cleanup())
			cleanup = nil
		}
	}()
	if resultErr != nil {
		return "", cleanup, resultErr
	}
	if path == "" {
		return "", cleanup, errors.New("the source overlay declaration returned no file path")
	}
	if cleanup == nil {
		return "", nil, errors.New("the source overlay declaration returned no owner cleanup")
	}
	path, resultErr = labOverlayAbsolute(root, path)
	if resultErr != nil {
		return "", cleanup, resultErr
	}
	declared, err := readLabOverlay(root, path)
	if err != nil {
		return "", cleanup, err
	}
	inheritedPath, err := labOverlayFromFlags(root, environment)
	if err != nil {
		return "", cleanup, err
	}
	if inheritedPath == "" {
		return path, cleanup, nil
	}
	if inheritedPath == path {
		return path, cleanup, nil
	}
	inherited, err := readLabOverlay(root, inheritedPath)
	if err != nil {
		return "", cleanup, err
	}
	merged, removeMerged, err := writeMergedLabOverlay(path, inherited, declared)
	if removeMerged != nil {
		releaseOwner := cleanup
		cleanup = func() error { return errors.Join(removeMerged(), releaseOwner()) }
	}
	if err != nil {
		return "", cleanup, err
	}
	return merged, cleanup, nil
}

// labOverlayFromFlags reads only the effective GOFLAGS entry, with the same
// last-entry rule as a child process. Go reads the final repeated -overlay flag;
// retain that choice while leaving the original environment untouched.
func labOverlayFromFlags(root string, environment []string) (string, error) {
	flags := ""
	for _, entry := range environment {
		if value, held := strings.CutPrefix(entry, "GOFLAGS="); held {
			flags = value
		}
	}
	if len(flags) > labBuildFlagsBytesMax {
		return "", fmt.Errorf("GOFLAGS exceeds the %d-byte lab build input limit", labBuildFlagsBytesMax)
	}
	arguments, err := shlex.Split(flags, true)
	if err != nil {
		return "", fmt.Errorf("read the inherited GOFLAGS: %w", err)
	}
	path := ""
	for _, argument := range arguments {
		if argument == "-overlay" {
			return "", errors.New("GOFLAGS names -overlay without its =path value")
		}
		if value, held := strings.CutPrefix(argument, "-overlay="); held {
			path = value
		}
	}
	if path == "" {
		return "", nil
	}
	return labOverlayAbsolute(root, path)
}

// readLabOverlay canonicalizes source and backing paths as Go's overlay reader
// does, so relative and absolute aliases cannot hide a conflicting edit.
func readLabOverlay(root, path string) (replacements map[string]string, resultErr error) {
	file, err := os.Open(path) //nolint:gosec // the lab or inherited GOFLAGS declares this build input
	if err != nil {
		return nil, fmt.Errorf("open source overlay %s: %w", path, err)
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	body, err := io.ReadAll(io.LimitReader(file, labOverlayBytesMax+1))
	if err != nil {
		return nil, fmt.Errorf("read source overlay %s: %w", path, err)
	}
	if len(body) > labOverlayBytesMax {
		return nil, fmt.Errorf("source overlay %s exceeds the %d-byte lab input limit", path, labOverlayBytesMax)
	}
	var decoded struct {
		Replace map[string]string
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("decode source overlay %s: %w", path, err)
	}
	replacements = make(map[string]string, len(decoded.Replace))
	for source, backing := range decoded.Replace {
		if source == "" {
			return nil, fmt.Errorf("source overlay %s contains an empty source path", path)
		}
		absoluteSource, err := labOverlayAbsolute(root, source)
		if err != nil {
			return nil, err
		}
		if _, duplicate := replacements[absoluteSource]; duplicate {
			return nil, fmt.Errorf("source overlay %s contains multiple paths for %s", path, absoluteSource)
		}
		absoluteBacking, err := labOverlayAbsolute(root, backing)
		if err != nil {
			return nil, err
		}
		replacements[absoluteSource] = absoluteBacking
	}
	return replacements, nil
}

// labOverlayAbsolute preserves Go's empty-backing deletion marker.
func labOverlayAbsolute(root, path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path), nil
	}
	absolute, err := filepath.Abs(filepath.Join(root, path))
	if err != nil {
		return "", fmt.Errorf("resolve source overlay path %s: %w", path, err)
	}
	return absolute, nil
}

// writeMergedLabOverlay creates a per-build input beside the declaring lab's
// own overlay. The caller MUST run the returned cleanup, including after an
// error that occurs once the directory has been created.
func writeMergedLabOverlay(declaredPath string, inherited, declared map[string]string) (string, func() error, error) {
	replacements := make(map[string]string, len(inherited)+len(declared))
	maps.Copy(replacements, inherited)
	for source, backing := range declared {
		if previous, held := replacements[source]; held {
			if previous != backing {
				return "", nil, fmt.Errorf("source overlay conflicts with the inherited replacement for %s", source)
			}
		}
		replacements[source] = backing
	}
	body, err := json.Marshal(struct {
		Replace map[string]string
	}{Replace: replacements})
	if err != nil {
		return "", nil, fmt.Errorf("encode the combined source overlay: %w", err)
	}
	directory, err := os.MkdirTemp(filepath.Dir(declaredPath), "combined-")
	if err != nil {
		return "", nil, fmt.Errorf("create the combined overlay directory: %w", err)
	}
	cleanup := func() error {
		if err := os.RemoveAll(directory); err != nil {
			return fmt.Errorf("remove the combined source overlay %s: %w", directory, err)
		}
		return nil
	}
	path := filepath.Join(directory, "overlay.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		return "", cleanup, fmt.Errorf("write the combined source overlay %s: %w", path, err)
	}
	return path, cleanup, nil
}

func labBinaryOverlayError(problem string, cause error) error {
	return opererr.Wrap(cause, problem,
		"The requested lab binary cannot use its declared source inputs. Correct the declaration or conflicting source replacements before retrying the build.",
		labBuildInputsFix)
}
