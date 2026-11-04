// Design: docs/contributing/rfc-implementation-guide.md -- canonical approval identity.
// Related: changedunits.go, goscope.go -- shared carrier and function boundaries.
package rfc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// ValidApprovalTarget reports whether target has canonical full Git object syntax.
// Existence and repository object format are checked by ResolveApprovalUnit.
func ValidApprovalTarget(target string) bool {
	if len(target) != 40 && len(target) != 64 {
		return false
	}
	for _, char := range target {
		if char >= '0' && char <= '9' {
			continue
		}
		if char >= 'a' && char <= 'f' {
			continue
		}
		return false
	}
	return true
}

// ResolveApprovalUnit resolves one directory-based key to a unique carrier.
// An empty target reads current/HEAD; a target MUST be a canonical full commit
// SHA and MUST change the named unit relative to its first parent. No writes
// occur, so callers MUST resolve before creating a session namespace.
func ResolveApprovalUnit(root, target, unit string) (string, error) {
	if !ValidApprovalUnit(unit) {
		return "", errors.New("approval unit must be <package>.<TestName>, got " + unit)
	}
	before := "HEAD"
	if target != "" {
		if !ValidApprovalTarget(target) {
			return "", errors.New("approval target must be a canonical full commit SHA, got " + target)
		}
		resolved, err := approvalGit(root, "rev-parse", "--verify", target+"^{commit}")
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(resolved) != target {
			return "", errors.New("approval target is not a canonical full commit SHA: " + target)
		}
		parents, err := approvalGit(root, "rev-list", "--parents", "-n", "1", target)
		if err != nil {
			return "", err
		}
		fields := strings.Fields(parents)
		if len(fields) < 2 {
			return "", errors.New("approval target has no readable parent: " + target)
		}
		before = fields[1]
	}
	oldPaths, newPaths := "", ""
	if target == "" && !insideGitCheckout(root) {
		// Edit-time callers also judge real current carriers in fixture trees
		// without history. This is current identity, not invented HEAD data.
		var currentPaths strings.Builder
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			currentPaths.WriteString(filepath.ToSlash(relative))
			currentPaths.WriteByte(0)
			return nil
		})
		if err != nil {
			return "", err
		}
		newPaths = currentPaths.String()
	} else {
		var err error
		oldPaths, err = approvalGit(root, "ls-tree", "-r", "--name-only", "-z", before)
		if err != nil {
			return "", err
		}
		if target == "" {
			newPaths, err = approvalGit(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
		} else {
			newPaths, err = approvalGit(root, "ls-tree", "-r", "--name-only", "-z", target)
		}
		if err != nil {
			return "", err
		}
	}
	oldSet := approvalPathSet(oldPaths)
	newSet := approvalPathSet(newPaths)
	paths := approvalPathSet(oldPaths + newPaths)
	packageName, name, _ := strings.Cut(unit, ".")
	oldCarriers := make([]approvalCarrier, 0, 1)
	newCarriers := make([]approvalCarrier, 0, 1)
	for path := range paths {
		if !IsTagCarrier(path) && !strings.HasSuffix(path, ".py") {
			continue
		}
		if filepath.Base(filepath.Dir(path)) != packageName {
			continue
		}
		oldText, err := approvalSide(root, before, path, oldSet[path])
		if err != nil {
			return "", err
		}
		newText, err := approvalSide(root, target, path, newSet[path])
		if err != nil {
			return "", err
		}
		oldUnits, err := approvalUnitTexts(path, oldText, name)
		if err != nil {
			return "", err
		}
		newUnits, err := approvalUnitTexts(path, newText, name)
		if err != nil {
			return "", err
		}
		for _, text := range oldUnits {
			oldCarriers = append(oldCarriers, approvalCarrier{path: path, text: text})
		}
		for _, text := range newUnits {
			newCarriers = append(newCarriers, approvalCarrier{path: path, text: text})
		}
	}
	if len(oldCarriers) > 1 {
		return "", ambiguousApprovalCarrier(unit, oldCarriers)
	}
	if len(newCarriers) > 1 {
		return "", ambiguousApprovalCarrier(unit, newCarriers)
	}
	if len(oldCarriers) == 0 && len(newCarriers) == 0 {
		return "", errors.New("approval unit " + unit + " has no current/parent carrier")
	}
	if target != "" {
		changed := len(oldCarriers) != len(newCarriers)
		if len(oldCarriers) == 1 && len(newCarriers) == 1 {
			changed = oldCarriers[0].text != newCarriers[0].text
		}
		if !changed {
			return "", errors.New("approval target " + target + " does not change " + unit)
		}
	}
	if len(newCarriers) == 1 {
		return newCarriers[0].path, nil
	}
	return oldCarriers[0].path, nil
}

type approvalCarrier struct {
	path string
	text string
}

func ambiguousApprovalCarrier(unit string, carriers []approvalCarrier) error {
	paths := make([]string, len(carriers))
	for index, carrier := range carriers {
		paths[index] = carrier.path
	}
	slices.Sort(paths)
	return errors.New("approval unit " + unit + " is ambiguous across carriers: " + strings.Join(paths, ", "))
}

func approvalPathSet(raw string) map[string]bool {
	paths := make(map[string]bool)
	for path := range strings.SplitSeq(raw, "\x00") {
		if path != "" {
			paths[path] = true
		}
	}
	return paths
}

func approvalSide(root, revision, path string, present bool) (string, error) {
	if !present {
		return "", nil
	}
	if revision != "" {
		return approvalGit(root, "show", revision+":"+path)
	}
	content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path))) // #nosec G304 -- Git supplies the repository-relative carrier.
	if errors.Is(err, os.ErrNotExist) {
		return "", nil // A tracked deletion is an explicit absent current side.
	}
	return string(content), err
}

func approvalUnitTexts(path, content, name string) ([]string, error) {
	if content == "" {
		return nil, nil
	}
	fileScoped := ScopeReader(path) == ScopeFile
	if !fileScoped {
		fileScoped = tagFallsOutsideFunction(path, content)
	}
	if fileScoped {
		if name == fileStem(path) {
			return []string{content}, nil
		}
		return nil, nil
	}
	var texts []string
	units, exact := readFunctionUnits(content)
	for _, unit := range units {
		if unit.Name == name {
			if !exact {
				return nil, errors.New("approval unit " + name + " has no exact Go declaration boundary in " + path)
			}
			texts = append(texts, unit.Text)
		}
	}
	return texts, nil
}

// approvalGit preserves blob and NUL-delimited path bytes; gitAnswer trims them.
func approvalGit(root string, args ...string) (string, error) {
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- Fixed Git executable with separately passed query operands.
	command.Dir = root
	var complaint bytes.Buffer
	command.Stderr = &complaint
	content, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("approval identity: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(complaint.String()))
	}
	return string(content), nil
}
