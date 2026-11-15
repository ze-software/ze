// Design: docs/contributing/gh-pages.md -- raw committed bytes are the publication identity.
// Related: publication_actions.go -- receipt changes and confirmation read this manifest.
package site

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

type publicationTreeFile struct {
	file   publicationFile
	object string
}

// publicationCommittedFiles reads raw blobs from one fixed commit, independent
// of index flags, clean/smudge filters, line-ending conversion and replace refs.
func publicationCommittedFiles(ctx context.Context, root, revision string) ([]publicationFile, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	entries, err := publicationTreeEntries(ctx, root, revision)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	var input strings.Builder
	for _, entry := range entries {
		input.WriteString(entry.object)
		input.WriteByte('\n')
	}
	command := exec.CommandContext(ctx, "git", "--no-replace-objects", "cat-file", "--batch") //nolint:gosec // fixed raw reader; validated object IDs go through stdin
	command.Dir = root
	command.Stdin = strings.NewReader(input.String())
	var diagnostic bytes.Buffer
	command.Stderr = &publicationBuffer{buffer: &diagnostic, remaining: 4096}
	output, err := command.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, err
	}
	waited := false
	defer func() {
		if !waited {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	reader := bufio.NewReaderSize(output, 256)
	files := make([]publicationFile, 0, len(entries))
	for _, entry := range entries {
		header, err := reader.ReadSlice('\n')
		if err != nil {
			return nil, fmt.Errorf("read committed blob header for %s: %w", entry.file.Path, err)
		}
		fields := strings.Fields(string(header))
		if len(fields) != 3 {
			return nil, errors.New("committed blob reader returned a malformed header")
		}
		if fields[0] != entry.object || fields[1] != "blob" {
			return nil, errors.New("committed blob reader returned another object identity or type")
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || size != entry.file.Size {
			return nil, errors.New("committed blob reader returned another object size")
		}
		hash := sha256.New()
		if _, err := io.CopyN(hash, reader, entry.file.Size); err != nil {
			return nil, fmt.Errorf("read committed bytes for %s: %w", entry.file.Path, err)
		}
		ending, err := reader.ReadByte()
		if err != nil || ending != '\n' {
			return nil, errors.New("committed blob reader returned an invalid record terminator")
		}
		file := entry.file
		file.Digest = hex.EncodeToString(hash.Sum(nil))
		files = append(files, file)
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		return nil, errors.New("committed blob reader returned trailing data or failed to finish")
	}
	waitErr := command.Wait()
	waited = true
	if waitErr != nil {
		return nil, fmt.Errorf("read committed website blobs: %w: %s", waitErr, strings.TrimSpace(diagnostic.String()))
	}
	return files, nil
}

func publicationTreeEntries(ctx context.Context, root, revision string) ([]publicationTreeFile, error) {
	if err := publicationRevision(revision); err != nil {
		return nil, err
	}
	listed, err := publicationGit(ctx, root, "--no-replace-objects", "ls-tree", "-r", "-l", "-z", revision)
	if err != nil {
		return nil, err
	}
	entries := make([]publicationTreeFile, 0)
	var total int64
	for raw := range bytes.SplitSeq(listed, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		if len(entries) >= publicationFilesMax {
			return nil, errors.New("committed artifact exceeds the file limit")
		}
		header, name, found := bytes.Cut(raw, []byte{'\t'})
		if !found {
			return nil, errors.New("committed artifact tree entry has no path")
		}
		path := string(name)
		if err := publicationPath(path); err != nil {
			return nil, err
		}
		fields := strings.Fields(string(header))
		if len(fields) != 4 {
			return nil, errors.New("committed artifact tree entry is malformed")
		}
		if fields[0] != "100644" && fields[0] != "100755" {
			return nil, fmt.Errorf("committed artifact path %s has nonregular mode %s", path, fields[0])
		}
		if fields[1] != "blob" {
			return nil, fmt.Errorf("committed artifact path %s is not a blob", path)
		}
		if err := publicationRevision(fields[2]); err != nil {
			return nil, err
		}
		size, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil || size < 0 || size > publicationFileBytesMax {
			return nil, fmt.Errorf("committed artifact path %s has an unsupported blob size", path)
		}
		total += size
		if total > publicationBytesMax {
			return nil, errors.New("committed artifact exceeds the byte limit")
		}
		entries = append(entries, publicationTreeFile{file: publicationFile{Path: path, Size: size}, object: fields[2]})
	}
	slices.SortFunc(entries, func(first, second publicationTreeFile) int {
		return strings.Compare(first.file.Path, second.file.Path)
	})
	for index := 1; index < len(entries); index++ {
		if entries[index-1].file.Path == entries[index].file.Path {
			return nil, errors.New("committed artifact tree repeats a path")
		}
	}
	return entries, nil
}
