// Design: docs/contributing/gh-pages.md -- recording and checking do not authorize publication.
// Related: publication.go -- source observation and exact artifact snapshots.
package site

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/opererr"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

const (
	publicationSourceKeyword     = "source"
	publicationReceiptKeyword    = "receipt"
	publicationDeploymentKeyword = "deployment"
)

type publicationReport struct {
	State            string            `json:"state"`
	Receipt          string            `json:"receipt"`
	Source           publicationSource `json:"source"`
	ArtifactDigest   string            `json:"artifact-digest"`
	ArtifactRevision string            `json:"artifact-revision,omitempty"`
	Files            int               `json:"files"`
	Changed          int               `json:"changed"`
	Deleted          int               `json:"deleted"`
	Probed           int               `json:"probed"`
}

func runPublicationRecord(arguments leaction.Arguments) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		return publicationFailure(err)
	}
	paths, err := ResolvePaths(root, arguments.One(keywordOutput))
	if err != nil {
		return publicationFailure(err)
	}
	checked, err := checkSiteArtifact(paths)
	if err != nil {
		return publicationFailure(err)
	}
	reportReplayRefusals(checked.Replay)
	if code := checked.exit(); code != 0 {
		return checked, code
	}
	receipt, err := recordPublication(context.Background(), paths, arguments.One(publicationSourceKeyword))
	if err != nil {
		return publicationFailure(err)
	}
	return publicationAnswer(paths, receipt, "recorded"), 0
}

func publicationFailure(err error) (any, int) {
	return publicationFailureWithFix(err, "./le site build")
}

func publicationFailureWithFix(err error, fix string) (any, int) {
	leaction.ReportError(opererr.Wrap(err, "Could not verify the website publication",
		"The source or reviewed artifact could not be identified; nothing was published.",
		fix))
	return nil, 1
}

func recordPublication(ctx context.Context, paths Paths, requested string) (publicationReceipt, error) {
	if err := publicationRevision(requested); err != nil {
		return publicationReceipt{}, err
	}
	record, err := publicationProducerRecord(paths)
	if err != nil {
		return publicationReceipt{}, fmt.Errorf("read full-build provenance: %w", err)
	}
	if record.Artifact != paths.Output {
		return publicationReceipt{}, errors.New("the producer record names another artifact")
	}
	if record.Provenance == nil {
		return publicationReceipt{}, errors.New("full-build provenance is absent; run a full ./le site build after reviewing the source")
	}
	provenance := record.Provenance
	if provenance.Format != publicationFormat {
		return publicationReceipt{}, errors.New("the full-build provenance format is unsupported")
	}
	if provenance.Unavailable != "" {
		return publicationReceipt{}, fmt.Errorf("full-build provenance is unavailable: %s", provenance.Unavailable)
	}
	if provenance.Source.Revision != requested {
		return publicationReceipt{}, errors.New("the requested source is not the full build's observed revision")
	}
	if err := publicationCurrentSource(ctx, paths, provenance.Source); err != nil {
		return publicationReceipt{}, err
	}
	files, digest, err := publicationSnapshot(paths.Output)
	if err != nil {
		return publicationReceipt{}, err
	}
	if digest != provenance.ArtifactDigest {
		return publicationReceipt{}, errors.New("artifact bytes changed after the full build; rebuild and review them again")
	}
	if !slices.Equal(files, provenance.Files) {
		return publicationReceipt{}, errors.New("artifact file identities changed after the full build")
	}
	changed, deleted, err := publicationChanges(ctx, paths, files)
	if err != nil {
		return publicationReceipt{}, err
	}
	receipt := publicationReceipt{Format: publicationFormat, Source: provenance.Source,
		Artifact: paths.Output, ArtifactDigest: digest, Files: files, Changed: changed, Deleted: deleted,
		RecordedAt: time.Now().UTC().Format(time.RFC3339)}
	if err := publicationValidateReceipt(receipt); err != nil {
		return publicationReceipt{}, err
	}
	if err := publicationWrite(paths, publicationReceiptPath(paths), receipt); err != nil {
		return publicationReceipt{}, err
	}
	return receipt, nil
}

func checkPublication(ctx context.Context, paths Paths) (publicationReceipt, error) {
	var receipt publicationReceipt
	if err := publicationRead(paths, publicationReceiptPath(paths), &receipt); err != nil {
		return receipt, err
	}
	if err := publicationValidateReceipt(receipt); err != nil {
		return receipt, err
	}
	if receipt.Artifact != paths.Output {
		return receipt, errors.New("the publication receipt names another artifact")
	}
	if err := publicationCurrentSource(ctx, paths, receipt.Source); err != nil {
		return receipt, err
	}
	files, digest, err := publicationSnapshot(paths.Output)
	if err != nil {
		return receipt, err
	}
	if digest != receipt.ArtifactDigest {
		return receipt, errors.New("artifact bytes differ from the recorded review; obtain a fresh review and record")
	}
	if !slices.Equal(files, receipt.Files) {
		return receipt, errors.New("artifact file identities differ from the recorded review")
	}
	return receipt, nil
}

func publicationCurrentSource(ctx context.Context, paths Paths, source publicationSource) error {
	if err := publicationRevision(source.Revision); err != nil {
		return err
	}
	if err := publicationDigest(source.Digest); err != nil {
		return err
	}
	if source.DigestScope != publicationSourceScope {
		return errors.New("the publication source-digest scope is unknown")
	}
	files, err := trackedAndUntrackedSourceFiles(paths)
	if err != nil {
		return err
	}
	digest, err := sourceDigest(paths.Source, files)
	if err != nil {
		return err
	}
	current, err := publicationBuildStart(ctx, paths, digest)
	if err != nil {
		return err
	}
	if current.Revision != source.Revision {
		return errors.New("source HEAD moved after the full build; rebuild from the selected revision")
	}
	if current.Digest != source.Digest {
		return errors.New("website source bytes changed after the full build; rebuild and review them again")
	}
	return nil
}

func publicationValidateReceipt(receipt publicationReceipt) error {
	if receipt.Format != publicationFormat {
		return errors.New("the publication receipt format is unsupported")
	}
	if err := publicationRevision(receipt.Source.Revision); err != nil {
		return err
	}
	if err := publicationDigest(receipt.Source.Digest); err != nil {
		return err
	}
	if err := publicationDigest(receipt.ArtifactDigest); err != nil {
		return err
	}
	if receipt.Source.DigestScope != publicationSourceScope {
		return errors.New("the receipt has no recognized source-digest scope")
	}
	if _, err := time.Parse(time.RFC3339, receipt.RecordedAt); err != nil {
		return errors.New("the receipt has no valid recording time")
	}
	if len(receipt.Files) == 0 {
		return errors.New("the receipt names no artifact files")
	}
	if len(receipt.Files) > publicationFilesMax {
		return errors.New("the receipt exceeds the artifact file limit")
	}
	known := make(map[string]struct{}, len(receipt.Files))
	previous := ""
	var total int64
	for _, file := range receipt.Files {
		if err := publicationPath(file.Path); err != nil {
			return err
		}
		if file.Path <= previous {
			return errors.New("receipt files are repeated or not in canonical order")
		}
		previous = file.Path
		if publicationIgnored(file.Path) {
			return errors.New("the receipt names ignored artifact bookkeeping")
		}
		if err := publicationDigest(file.Digest); err != nil {
			return err
		}
		if file.Size < 0 {
			return errors.New("receipt file size is negative")
		}
		if file.Size > publicationFileBytesMax {
			return errors.New("receipt file exceeds the read limit")
		}
		total += file.Size
		if total > publicationBytesMax {
			return errors.New("receipt artifact exceeds the read limit")
		}
		known[file.Path] = struct{}{}
	}
	if publicationFilesDigest(receipt.Files) != receipt.ArtifactDigest {
		return errors.New("receipt file hashes do not match its artifact digest")
	}
	for _, names := range [][]string{receipt.Changed, receipt.Deleted} {
		if len(names) > publicationFilesMax {
			return errors.New("receipt change set exceeds the file limit")
		}
		previous = ""
		for _, name := range names {
			if err := publicationPath(name); err != nil {
				return err
			}
			if name <= previous {
				return errors.New("receipt changes are repeated or not in canonical order")
			}
			previous = name
		}
	}
	for _, name := range receipt.Changed {
		if _, found := known[name]; !found {
			return errors.New("a changed receipt file has no recorded bytes")
		}
	}
	for _, name := range receipt.Deleted {
		if _, found := known[name]; found {
			return errors.New("a deleted receipt file still has recorded bytes")
		}
	}
	return nil
}

func publicationChanges(ctx context.Context, paths Paths, files []publicationFile) ([]string, []string, error) {
	tracked, err := publicationGit(ctx, paths.Output, "ls-files", "-z")
	if err != nil {
		return nil, nil, err
	}
	for _, name := range publicationNames(tracked) {
		if publicationIgnored(name) {
			return nil, nil, fmt.Errorf("tracked artifact file %s is ignored bookkeeping; remove it from publication through the scoped commit flow", name)
		}
	}
	head, err := publicationGit(ctx, paths.Output, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return nil, nil, err
	}
	baseline, err := publicationCommittedFiles(ctx, paths.Output, strings.TrimSpace(string(head)))
	if err != nil {
		return nil, nil, err
	}
	before := make(map[string]publicationFile, len(baseline))
	for _, file := range baseline {
		if publicationIgnored(file.Path) {
			return nil, nil, fmt.Errorf("baseline commit names ignored publication path %s", file.Path)
		}
		before[file.Path] = file
	}
	known := make(map[string]publicationFile, len(files))
	changed := make([]string, 0)
	for _, file := range files {
		known[file.Path] = file
		prior, found := before[file.Path]
		if !found {
			changed = append(changed, file.Path)
			continue
		}
		if prior != file {
			changed = append(changed, file.Path)
		}
	}
	deleted := make([]string, 0)
	for _, file := range baseline {
		if _, present := known[file.Path]; !present {
			deleted = append(deleted, file.Path)
		}
	}
	return changed, deleted, nil
}

func publicationCommit(ctx context.Context, paths Paths, receipt publicationReceipt, expected string) error {
	if err := publicationRevision(expected); err != nil {
		return err
	}
	head, err := publicationGit(ctx, paths.Output, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != expected {
		return errors.New("artifact HEAD is not the requested deployment commit")
	}
	status, err := publicationGit(ctx, paths.Output, "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(status) != 0 {
		return errors.New("artifact tracked, staged or publishable untracked files differ from HEAD; commit only the reviewed bytes before confirming deployment")
	}
	tracked, err := publicationGit(ctx, paths.Output, "ls-files", "-z")
	if err != nil {
		return err
	}
	names := publicationNames(tracked)
	if len(names) != len(receipt.Files) {
		return errors.New("artifact commit file set differs from the reviewed file set")
	}
	for index, file := range receipt.Files {
		if names[index] != file.Path {
			return errors.New("artifact commit contains missing, extra or ignored tracked files")
		}
	}
	committed, err := publicationCommittedFiles(ctx, paths.Output, expected)
	if err != nil {
		return err
	}
	if !slices.Equal(committed, receipt.Files) {
		return errors.New("raw committed artifact bytes differ from the reviewed receipt; Git filters or index flags cannot substitute for the deployed bytes")
	}
	return nil
}

func publicationAnswer(paths Paths, receipt publicationReceipt, state string) publicationReport {
	return publicationReport{State: state, Receipt: publicationReceiptPath(paths), Source: receipt.Source,
		ArtifactDigest: receipt.ArtifactDigest, Files: len(receipt.Files), Changed: len(receipt.Changed), Deleted: len(receipt.Deleted)}
}
