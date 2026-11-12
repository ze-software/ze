// Design: docs/contributing/spec-workflow.md -- committed-source readers share Git object I/O.
// Package objects resolves immutable Git revisions, lists their trees and their
// first-parent history, and reads their blobs without exporting, compiling or
// opening working-tree source files.
package objects

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
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

// ResolveTree resolves one explicit revision to its immutable tree ID.
// Callers MUST retain this ID for all subsequent membership and blob reads.
// Empty input refuses rather than substituting the ambient HEAD.
func ResolveTree(ctx context.Context, root, revision string) (string, error) {
	if revision == "" {
		return "", errors.New("an explicit tree revision is required")
	}
	if strings.HasPrefix(revision, "-") {
		return "", fmt.Errorf("invalid tree revision %q; select an explicit Git tree or commit", revision)
	}
	output, err := run(ctx, root, nil, "rev-parse", "--verify", "--end-of-options", revision+"^{tree}")
	if err != nil {
		return "", fmt.Errorf("tree revision %q is unavailable; restore its history or objects before preparing the commit: %w", revision, err)
	}
	resolved := strings.TrimSpace(string(output))
	if !objectID(resolved) {
		return "", fmt.Errorf("tree revision %q returned an invalid object ID; repair the repository", revision)
	}
	return resolved, nil
}

// ReadPath reads one exact blob member of a retained immutable tree. A missing
// path in a readable tree returns present=false; missing trees or blobs refuse.
// Callers MUST retain the tree returned by ResolveTree, not resolve HEAD again.
func ReadPath(ctx context.Context, root, tree, path string) ([]byte, bool, error) {
	if !objectID(tree) {
		return nil, false, fmt.Errorf("invalid tree object ID %q; resolve the requested immutable tree before reading its paths", tree)
	}
	if path == "" {
		return nil, false, errors.New("an explicit Git path is required")
	}
	for i := range len(path) {
		if path[i] < ' ' || path[i] == 0x7f {
			return nil, false, fmt.Errorf("Git path %q contains a control character", path)
		}
	}
	output, err := run(ctx, root, nil, "--literal-pathspecs", "ls-tree", "-r", "-z", tree, "--", path)
	if err != nil {
		return nil, false, fmt.Errorf("cannot read path %q from immutable tree %s: %w", path, tree, err)
	}
	if len(output) == 0 {
		return nil, false, nil
	}
	entry, rest, terminated := bytes.Cut(output, []byte{0})
	metadata, name, named := bytes.Cut(entry, []byte{'\t'})
	mode, kindAndID, moded := bytes.Cut(metadata, []byte{' '})
	kind, id, typed := bytes.Cut(kindAndID, []byte{' '})
	blobID := string(id)
	if !terminated || len(rest) != 0 || !named || string(name) != path ||
		!moded || len(mode) != 6 || !typed || string(kind) != "blob" {
		return nil, false, fmt.Errorf("path %q does not identify one exact blob member of immutable tree %s", path, tree)
	}
	blobs, err := Read(ctx, root, []string{blobID})
	if err != nil {
		return nil, false, fmt.Errorf("cannot read path %q from immutable tree %s: %w", path, tree, err)
	}
	return blobs[0], true, nil
}

// Read reads the selected immutable blob IDs in one Git batch, in request order.
// Callers MUST select IDs from one immutable revision retained from Resolve or
// ResolveTree, never resolve HEAD again between selecting membership and reading
// metadata. Returned slices own their retained batch payload and are safe for
// concurrent read-only use. The whole batch is held in memory and bounded by
// run's 30 seconds; a read too large for that goes through Stream.
func Read(ctx context.Context, root string, ids []string) ([][]byte, error) {
	blobs := make([][]byte, 0, len(ids))
	if len(ids) == 0 {
		return blobs, nil
	}
	request, err := batchRequest(ids)
	if err != nil {
		return nil, err
	}
	output, err := run(ctx, root, strings.NewReader(request), "cat-file", "--batch")
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
		return nil, fmt.Errorf("git returned trailing source blob data; repair the repository before publishing an inventory")
	}
	return blobs, nil
}

// Stream reads the blob of each ID through one git cat-file --batch process,
// streaming its output, and calls visit once per ID in request order. Callers
// MUST select IDs from one immutable revision retained from Resolve or
// ResolveTree. The blob slice visit receives is valid only until visit
// returns: visit MUST copy any byte it keeps, because the next blob reuses the
// buffer. One blob is held in memory at a time, and ctx alone bounds the run:
// Stream sets no deadline of its own, because a whole history (about 159,000
// blobs, 2.1 GB) is far beyond what Read's buffered 30-second batch carries.
//
// A missing object, a non-blob object, broken framing, output after the last
// blob, a failed git process and an error visit returns each stop the stream
// and refuse it. Visit's error is wrapped, so errors.Is matches it. Trailing
// output and a failed exit are found only after visit saw every blob; the
// error then refuses the whole stream, not only its end. Safe for concurrent
// use: each call runs its own git process and calls its visit sequentially on
// the caller's goroutine.
func Stream(ctx context.Context, root string, ids []string, visit func(id string, blob []byte) error) error {
	if len(ids) == 0 {
		return nil
	}
	request, err := batchRequest(ids)
	if err != nil {
		return err
	}
	streaming, cancel := context.WithCancel(ctx)
	defer cancel()

	command := exec.CommandContext(streaming, "git", "--no-replace-objects", "-C", root, "cat-file", "--batch") //nolint:gosec // Fixed Git argv; validated full object IDs go through stdin.
	// os/exec copies a Stdin that is not a file into git's pipe from a
	// goroutine it owns and Wait joins, so the request needs no goroutine here.
	command.Stdin = strings.NewReader(request)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.StdoutPipe()
	if err != nil {
		return fmt.Errorf("cannot open a pipe to git cat-file; free file descriptors and run again: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("cannot start git cat-file to stream committed blobs; install git or repair PATH: %w", err)
	}
	streamErr := streamBlobs(bufio.NewReader(output), ids, visit)
	if streamErr != nil {
		// A stream stopped early leaves git blocked on a full pipe; the
		// cancel kills it, so Wait returns.
		cancel()
	}
	exitErr := command.Wait()
	if streamErr != nil {
		// exitErr is then the kill above or the failure that ended the stream
		// early; git's own account of that failure is on stderr, and an ended
		// ctx explains a stream that would otherwise read as a missing object.
		if ctxErr := ctx.Err(); ctxErr != nil {
			return fmt.Errorf("streaming committed blobs ended before the last blob; run it again with a longer bound: %w: %w", ctxErr, streamErr)
		}
		return withGitComplaint(streamErr, stderr.String())
	}
	if exitErr != nil {
		return fmt.Errorf("git cat-file failed after streaming every committed blob; repair the repository: %w: %s", exitErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}

// streamBlobs validates one batch frame per requested ID with the header
// checks takeBlob makes, holding one payload at a time in a reused buffer,
// then refuses any output after the last frame. The loop is bounded by
// len(ids). The buffer grows only as payload bytes arrive, so a header that
// lies about its length ends in a truncation refusal, never a huge allocation.
func streamBlobs(reader *bufio.Reader, ids []string, visit func(id string, blob []byte) error) error {
	var payload bytes.Buffer
	for _, id := range ids {
		header, err := reader.ReadSlice('\n')
		if err != nil {
			return fmt.Errorf("%w: %w", missingBlobHeader(id), err)
		}
		size, err := blobSize(header[:len(header)-1], id)
		if err != nil {
			return err
		}
		payload.Reset()
		if _, err := io.CopyN(&payload, reader, int64(size)); err != nil {
			return fmt.Errorf("%w: %w", truncatedBlob(id), err)
		}
		terminator, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("%w: %w", truncatedBlob(id), err)
		}
		if terminator != '\n' {
			return misframedBlob(id)
		}
		if err := visit(id, payload.Bytes()); err != nil {
			return fmt.Errorf("streaming committed blobs stopped at %s: %w", id, err)
		}
	}
	if _, err := reader.ReadByte(); !errors.Is(err, io.EOF) {
		if err != nil {
			return fmt.Errorf("cannot read the end of the committed blob stream; repair the repository: %w", err)
		}
		return errors.New("git returned trailing source blob data after the last requested blob; repair the repository")
	}
	return nil
}

// withGitComplaint adds what git wrote on stderr to a refused stream: a
// stream that ended early because git failed, on an unreadable repository for
// one, otherwise reads only as a missing header.
func withGitComplaint(err error, complaint string) error {
	complaint = strings.TrimSpace(complaint)
	if complaint == "" {
		return err
	}
	return fmt.Errorf("%w; git reported: %s", err, complaint)
}

// Entry is one non-tree member of a recursive tree listing, as git wrote it:
// its mode, its object kind (blob or commit), its object ID and its
// repository-relative path. Callers MUST NOT pass Path back to git as an
// argument: a Git path may hold any byte but NUL.
type Entry struct {
	Mode, Kind, ID, Path string
}

// Regular reports whether the entry is a regular file: a blob of mode 100644
// or 100755. A symbolic link is a blob of mode 120000 and a submodule is a
// commit, so neither is regular.
func (e Entry) Regular() bool {
	if e.Kind != "blob" {
		return false
	}
	return e.Mode == "100644" || e.Mode == "100755"
}

// List answers every entry of the recursive listing of treeish, a full commit
// or tree ID retained from Resolve or ResolveTree, in git's order. Blobs,
// symbolic links (blobs of mode 120000) and submodules (kind commit) are
// listed; trees are not, since their members are. A malformed record refuses
// the whole listing rather than answering part of it.
func List(ctx context.Context, root, treeish string) ([]Entry, error) {
	if !objectID(treeish) {
		return nil, fmt.Errorf("invalid tree object ID %q; list a full commit or tree ID resolved from the requested revision", treeish)
	}
	output, err := run(ctx, root, nil, "ls-tree", "-r", "-z", "--full-tree", treeish)
	if err != nil {
		return nil, fmt.Errorf("cannot list immutable tree %s; restore its history or objects: %w", treeish, err)
	}
	return parseListing(output, treeish)
}

// parseListing splits NUL-terminated ls-tree records, each
// "<mode> <kind> <id>\t<path>". Each pass consumes one terminated record, so
// the loop is bounded by the output length.
func parseListing(output []byte, treeish string) ([]Entry, error) {
	entries := make([]Entry, 0, bytes.Count(output, []byte{0}))
	for len(output) > 0 {
		record, rest, terminated := bytes.Cut(output, []byte{0})
		if !terminated {
			return nil, fmt.Errorf("git returned an unterminated listing record for tree %s; repair the repository", treeish)
		}
		entry, err := parseListingRecord(record)
		if err != nil {
			return nil, fmt.Errorf("tree %s: %w", treeish, err)
		}
		entries = append(entries, entry)
		output = rest
	}
	return entries, nil
}

// parseListingRecord checks each field of one record and pairs its kind with
// its mode: a commit is a submodule, mode 160000, and every other entry of a
// recursive listing is a blob.
func parseListingRecord(record []byte) (Entry, error) {
	metadata, path, named := bytes.Cut(record, []byte{'\t'})
	if !named {
		return Entry{}, malformedListingRecord(record)
	}
	if len(path) == 0 {
		return Entry{}, malformedListingRecord(record)
	}
	mode, kindAndID, moded := bytes.Cut(metadata, []byte{' '})
	if !moded {
		return Entry{}, malformedListingRecord(record)
	}
	kind, id, typed := bytes.Cut(kindAndID, []byte{' '})
	if !typed {
		return Entry{}, malformedListingRecord(record)
	}
	entry := Entry{Mode: string(mode), Kind: string(kind), ID: string(id), Path: string(path)}
	if !octalMode(entry.Mode) {
		return Entry{}, malformedListingRecord(record)
	}
	if !objectID(entry.ID) {
		return Entry{}, malformedListingRecord(record)
	}
	switch entry.Kind {
	case "blob", "commit":
	default:
		return Entry{}, fmt.Errorf("listing record %q names object kind %q, which a recursive listing never holds; repair the repository", record, entry.Kind)
	}
	if (entry.Kind == "commit") != (entry.Mode == "160000") {
		return Entry{}, fmt.Errorf("listing record %q pairs kind %s with mode %s; repair the repository", record, entry.Kind, entry.Mode)
	}
	return entry, nil
}

// octalMode accepts the six octal digits git writes for every tree entry mode.
func octalMode(mode string) bool {
	if len(mode) != 6 {
		return false
	}
	for _, digit := range mode {
		if digit >= '0' && digit <= '7' {
			continue
		}
		return false
	}
	return true
}

func malformedListingRecord(record []byte) error {
	return fmt.Errorf("git returned a malformed tree listing record %q; repair the repository", record)
}

// Commit is one commit of a first-parent history: its ID, its tree's ID, and
// its author and committer times, both in UTC.
type Commit struct {
	ID, Tree               string
	AuthorTime, CommitTime time.Time
}

// FirstParents answers the first-parent ancestry of commit, a full commit ID
// retained from Resolve, oldest first and ending at commit itself. A merge is
// followed through its first parent only. A shallow repository refuses,
// because its history would end early and still read as complete. The
// history comes from one git log within run's 30-second bound, which a whole
// history of about 16,600 commits stays far inside. A malformed line refuses
// the whole history rather than answering part of it.
func FirstParents(ctx context.Context, root, commit string) ([]Commit, error) {
	if !objectID(commit) {
		return nil, fmt.Errorf("invalid commit object ID %q; resolve the requested revision before reading its history", commit)
	}
	if err := wholeHistory(ctx, root, commit); err != nil {
		return nil, err
	}
	output, err := run(ctx, root, nil, "log", "--first-parent", "--reverse", "--no-show-signature", "--format=%H %T %at %ct", commit, "--")
	if err != nil {
		return nil, fmt.Errorf("first-parent history of %s is unavailable; obtain the complete history before reading it: %w", commit, err)
	}
	return parseFirstParents(output, commit)
}

// wholeHistory refuses a shallow repository: git log stops at its boundary
// without an error, so the oldest commit it answers is not the history's
// first.
func wholeHistory(ctx context.Context, root, commit string) error {
	output, err := run(ctx, root, nil, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return fmt.Errorf("cannot tell whether the repository holds the whole history of %s; repair the repository: %w", commit, err)
	}
	switch answer := strings.TrimSpace(string(output)); answer {
	case "false":
		return nil
	case "true":
		return fmt.Errorf("the repository is a shallow clone, so the first-parent history of %s would stop early; run git fetch --unshallow before reading it", commit)
	default:
		return fmt.Errorf("git rev-parse --is-shallow-repository answered %q; repair the repository", answer)
	}
}

// parseFirstParents reads one "<id> <tree> <author seconds> <committer
// seconds>" line per commit and checks that the history ends at commit. Each
// pass consumes one terminated line, so the loop is bounded by the output
// length.
func parseFirstParents(output []byte, commit string) ([]Commit, error) {
	commits := make([]Commit, 0, bytes.Count(output, []byte{'\n'}))
	for len(output) > 0 {
		line, rest, terminated := bytes.Cut(output, []byte{'\n'})
		if !terminated {
			return nil, fmt.Errorf("git returned an unterminated history line for %s; repair the repository", commit)
		}
		parsed, err := parseHistoryLine(string(line))
		if err != nil {
			return nil, fmt.Errorf("first-parent history of %s: %w", commit, err)
		}
		commits = append(commits, parsed)
		output = rest
	}
	if len(commits) == 0 {
		return nil, fmt.Errorf("git returned no history for commit %s; repair the repository", commit)
	}
	if commits[len(commits)-1].ID != commit {
		return nil, fmt.Errorf("git returned a first-parent history that does not end at commit %s; repair the repository", commit)
	}
	return commits, nil
}

func parseHistoryLine(line string) (Commit, error) {
	fields := strings.Split(line, " ")
	if len(fields) != 4 {
		return Commit{}, fmt.Errorf("malformed history line %q; repair the repository", line)
	}
	if !objectID(fields[0]) {
		return Commit{}, fmt.Errorf("history line %q has an invalid commit ID; repair the repository", line)
	}
	if !objectID(fields[1]) {
		return Commit{}, fmt.Errorf("history line %q has an invalid tree ID; repair the repository", line)
	}
	authored, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return Commit{}, fmt.Errorf("history line %q has an invalid author time; repair the repository: %w", line, err)
	}
	committed, err := strconv.ParseInt(fields[3], 10, 64)
	if err != nil {
		return Commit{}, fmt.Errorf("history line %q has an invalid committer time; repair the repository: %w", line, err)
	}
	return Commit{
		ID:         fields[0],
		Tree:       fields[1],
		AuthorTime: time.Unix(authored, 0).UTC(),
		CommitTime: time.Unix(committed, 0).UTC(),
	}, nil
}

// batchRequest writes one validated full object ID per line, the request
// cat-file --batch reads on stdin. A line that is not an ID would be read as a
// revision expression, so it refuses here.
func batchRequest(ids []string) (string, error) {
	var request strings.Builder
	for _, id := range ids {
		if !objectID(id) {
			return "", fmt.Errorf("invalid source object ID %q; select full blob IDs from the resolved commit", id)
		}
		request.WriteString(id)
		request.WriteByte('\n')
	}
	return request.String(), nil
}

// takeBlob validates framing before exposing a payload. Length framing, rather
// than line splitting, preserves NUL bytes and embedded newlines in source.
func takeBlob(output []byte, id string) ([]byte, []byte, error) {
	header, rest, found := bytes.Cut(output, []byte{'\n'})
	if !found {
		return nil, nil, missingBlobHeader(id)
	}
	size, err := blobSize(header, id)
	if err != nil {
		return nil, nil, err
	}
	if size >= len(rest) {
		return nil, nil, truncatedBlob(id)
	}
	if rest[size] != '\n' {
		return nil, nil, misframedBlob(id)
	}
	return rest[:size], rest[size+1:], nil
}

// blobSize checks one cat-file --batch header, without its newline, against
// the requested ID and answers the payload length that follows it. Read and
// Stream judge every header here, so the buffered and the streaming reader
// refuse alike.
func blobSize(header []byte, id string) (int, error) {
	fields := strings.Fields(string(header))
	if len(fields) != 3 {
		return 0, fmt.Errorf("source blob %s is unavailable; restore the missing Git object", id)
	}
	if fields[0] != id {
		return 0, fmt.Errorf("git returned a different object for source blob %s; repair the repository", id)
	}
	if fields[1] != "blob" {
		return 0, fmt.Errorf("source object %s is %s, not a blob; select a regular file from the resolved commit", id, fields[1])
	}
	size, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, fmt.Errorf("source blob %s has invalid length; repair the repository: %w", id, err)
	}
	if size < 0 {
		return 0, fmt.Errorf("source blob %s has negative length; repair the repository", id)
	}
	return size, nil
}

func missingBlobHeader(id string) error {
	return fmt.Errorf("missing source blob header for %s; restore the missing Git object", id)
}

func truncatedBlob(id string) error {
	return fmt.Errorf("source blob %s is truncated; restore the complete Git object", id)
}

func misframedBlob(id string) error {
	return fmt.Errorf("source blob %s has invalid framing; repair the repository", id)
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

// run executes one git command within a 30-second bound and answers its whole
// output; a read that cannot finish in that time streams instead.
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
