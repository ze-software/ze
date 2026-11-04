// Design: docs/contributing/spec-workflow.md -- committed-source readers share Git object I/O.
// Package objects resolves committed revisions and reads immutable blobs without
// exporting, compiling or opening working-tree source files.
package objects

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Resolve resolves revision once to a full commit ID. Empty revision selects HEAD.
// Callers MUST retain this ID for every subsequent tree and metadata selection;
// Read MUST receive object IDs selected from that same resolved commit.
func Resolve(ctx context.Context, root, revision string) (string, error) {
	if revision == "" {
		revision = "HEAD"
	}
	if strings.HasPrefix(revision, "-") {
		return "", fmt.Errorf("invalid committed revision %q; select a Git commit or reference", revision)
	}
	output, err := run(ctx, root, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("revision %q is unavailable; obtain the requested history before reading committed sources: %w", revision, err)
	}
	resolved := strings.TrimSpace(string(output))
	if !objectID(resolved) {
		return "", fmt.Errorf("revision %q returned an invalid commit ID; repair the repository before reading committed sources", revision)
	}
	return resolved, nil
}

// Read reads the selected immutable blob IDs in one Git batch, in request order.
// Callers MUST select IDs from one commit retained from Resolve, never resolve
// HEAD again between selecting membership and reading metadata. Returned slices
// own their retained batch payload and are safe for concurrent read-only use.
func Read(ctx context.Context, root string, ids []string) ([][]byte, error) {
	blobs := make([][]byte, 0, len(ids))
	if len(ids) == 0 {
		return blobs, nil
	}
	var request strings.Builder
	for _, id := range ids {
		if !objectID(id) {
			return nil, fmt.Errorf("invalid source object ID %q; select full blob IDs from the resolved commit", id)
		}
		request.WriteString(id)
		request.WriteByte('\n')
	}
	output, err := run(ctx, root, strings.NewReader(request.String()), "cat-file", "--batch")
	if err != nil {
		return nil, fmt.Errorf("committed source blobs cannot be read; repair the repository before publishing an inventory: %w", err)
	}
	for _, id := range ids {
		blob, rest, err := takeBlob(output, id)
		if err != nil {
			return nil, err
		}
		blobs = append(blobs, blob)
		output = rest
	}
	if len(output) != 0 {
		return nil, fmt.Errorf("Git returned trailing source blob data; repair the repository before publishing an inventory")
	}
	return blobs, nil
}

// takeBlob validates framing before exposing a payload. Length framing, rather
// than line splitting, preserves NUL bytes and embedded newlines in source.
func takeBlob(output []byte, id string) ([]byte, []byte, error) {
	header, rest, found := bytes.Cut(output, []byte{'\n'})
	if !found {
		return nil, nil, fmt.Errorf("missing source blob header for %s; restore the missing Git object", id)
	}
	fields := strings.Fields(string(header))
	if len(fields) != 3 {
		return nil, nil, fmt.Errorf("source blob %s is unavailable; restore the missing Git object", id)
	}
	if fields[0] != id {
		return nil, nil, fmt.Errorf("Git returned a different object for source blob %s; repair the repository", id)
	}
	if fields[1] != "blob" {
		return nil, nil, fmt.Errorf("source object %s is %s, not a blob; select a regular file from the resolved commit", id, fields[1])
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return nil, nil, fmt.Errorf("source blob %s has invalid length; repair the repository: %w", id, err)
	}
	if size < 0 {
		return nil, nil, fmt.Errorf("source blob %s has negative length; repair the repository", id)
	}
	if size >= len(rest) {
		return nil, nil, fmt.Errorf("source blob %s is truncated; restore the complete Git object", id)
	}
	if rest[size] != '\n' {
		return nil, nil, fmt.Errorf("source blob %s has invalid framing; repair the repository", id)
	}
	return rest[:size], rest[size+1:], nil
}

// objectID accepts the full hexadecimal names used by SHA-1 and SHA-256 Git
// repositories. No revision expression or batch-protocol delimiter is an ID.
func objectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for _, digit := range id {
		if digit >= '0' && digit <= '9' {
			continue
		}
		if digit >= 'a' && digit <= 'f' {
			continue
		}
		return false
	}
	return true
}

func run(ctx context.Context, root string, input *strings.Reader, args ...string) ([]byte, error) {
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "git", append([]string{"--no-replace-objects", "-C", root}, args...)...) //nolint:gosec // Internal Git argv; revisions are separated from options and object IDs are validated.
	if input != nil {
		command.Stdin = input
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return output, nil
}
