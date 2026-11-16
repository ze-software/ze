// Design: docs/contributing/rfc-conformance-gates.md -- commit-owned tracked relocation.
// Related: rename.go -- source planning, exclusive destinations and publication reports.
package rfc

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
)

// renameSource binds validated source bytes to a regular directory entry and
// its confirmed Git state. The zero value is invalid: identity and Git binding
// must both be present.
type renameSource struct {
	file     renameFile
	identity os.FileInfo
	git      *renameGitBinding
	tracked  bool
}

// renameGitBinding records the source's exact HEAD and index entries. revision
// names the immutable commit used to validate HEAD bytes; unrelated commits
// may advance HEAD later when these source entries remain unchanged.
type renameGitBinding struct {
	revision  string
	head      []byte
	index     []byte
	untracked []byte
}

// readRenameSource MUST follow pair validation. Its caller MUST retain the
// captured identity and bytes until application so later edits are refused.
func readRenameSource(tree, rel string) (renameSource, error) {
	path := treePath(tree, rel)
	info, err := os.Lstat(path)
	if err != nil {
		return renameSource{}, err
	}
	if !info.Mode().IsRegular() {
		return renameSource{}, errors.New(rel + ": source is not a regular file; leave it in place and select the tagged source file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return renameSource{}, err
	}
	binding, err := readRenameGitBinding(tree, rel)
	if err != nil {
		return renameSource{}, err
	}
	state := renameSource{file: renameFile{Rel: rel, Original: body}, identity: info, git: binding}
	state.tracked = len(binding.head) > 0 || len(binding.index) > 0
	if len(binding.head) == 0 {
		if len(binding.index) > 0 || string(binding.untracked) != rel+"\x00" {
			return renameSource{}, errors.New(rel + ": is not tracked at HEAD and is not a new untracked carrier; leave the source in place and check its index and ignore state")
		}
	} else {
		head, ok := gitOutput(tree, "cat-file", "blob", binding.revision+":"+rel)
		if !ok {
			return renameSource{}, errors.New(rel + ": cannot read its HEAD version; leave the source in place, restore repository access and run the move again")
		}
		if !bytes.Equal(head, body) {
			return renameSource{}, errors.New(rel + ": differs from HEAD; commit or leave the edit before moving it")
		}
	}
	if err := renameSourceCurrent(tree, state); err != nil {
		return renameSource{}, err
	}
	return state, nil
}

// readRenameGitBinding MUST fail closed on every Git lookup. HEAD's tree is
// read through an immutable revision, then HEAD is checked again so a commit
// advancing during the observation cannot mix two different source states.
func readRenameGitBinding(tree, rel string) (*renameGitBinding, error) {
	problem := errors.New(rel + ": cannot confirm its Git state; leave the source in place, restore repository access and run the move again")
	revision, ok := gitOutput(tree, "rev-parse", "--verify", "--quiet", headRevision)
	if !ok || len(bytes.TrimSpace(revision)) == 0 {
		return nil, problem
	}
	binding := &renameGitBinding{revision: string(bytes.TrimSpace(revision))}
	binding.head, ok = gitOutput(tree, "--literal-pathspecs", "ls-tree", "-z", binding.revision, "--", rel)
	if !ok {
		return nil, problem
	}
	binding.index, ok = gitOutput(tree, "--literal-pathspecs", "ls-files", "--stage", "-z", "--", rel)
	if !ok {
		return nil, problem
	}
	if len(binding.head) == 0 && len(binding.index) == 0 {
		binding.untracked, ok = gitOutput(tree, "--literal-pathspecs", "ls-files", "--others", "--exclude-standard", "-z", "--", rel)
		if !ok {
			return nil, problem
		}
	}
	current, ok := gitOutput(tree, "rev-parse", "--verify", "--quiet", headRevision)
	if !ok || !bytes.Equal(bytes.TrimSpace(revision), bytes.TrimSpace(current)) {
		return nil, problem
	}
	return binding, nil
}

// renameSourceCurrent MUST run before creating its destination and before any
// untracked-source removal; the plan's captured bytes alone cannot prove that
// another writer left the source unchanged.
func renameSourceCurrent(tree string, source renameSource) error {
	problem := source.file.Rel + ": changed since the rename read it; leave the source in place and run the move again"
	if source.identity == nil || source.git == nil {
		return errors.New(problem)
	}
	path := treePath(tree, source.file.Rel)
	info, err := os.Lstat(path)
	if err != nil {
		return errors.New(problem)
	}
	if !os.SameFile(source.identity, info) {
		return errors.New(problem)
	}
	if info.Mode() != source.identity.Mode() {
		return errors.New(problem)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return errors.New(problem)
	}
	if !bytes.Equal(body, source.file.Original) {
		return errors.New(problem)
	}
	binding, err := readRenameGitBinding(tree, source.file.Rel)
	if err != nil {
		return err
	}
	if !bytes.Equal(source.git.head, binding.head) || !bytes.Equal(source.git.index, binding.index) || !bytes.Equal(source.git.untracked, binding.untracked) {
		return errors.New(problem)
	}
	return nil
}

// copyRenameTarget publishes a complete independent inode without replacing an
// existing destination. Its caller retains every published target on failure.
func copyRenameTarget(target string, source renameSource) error {
	staged, err := os.CreateTemp(filepath.Dir(target), ".rename-source-*.tmp")
	if err != nil {
		return err
	}
	name := staged.Name()
	defer func() { _ = os.Remove(name) }() //nolint:errcheck // only this private staging name is disposable; a published target retains its inode
	if _, err := staged.Write(source.file.Original); err != nil {
		return errors.Join(err, staged.Close())
	}
	if err := staged.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, source.identity.Mode().Perm()); err != nil {
		return err
	}
	return os.Link(name, target)
}
