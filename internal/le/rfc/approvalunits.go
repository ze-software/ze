// Design: docs/contributing/rfc-implementation-guide.md -- canonical approval identity.
// Related: changedunits.go, goscope.go -- shared carrier and function boundaries.
package rfc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/le/gitenv"
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
	content, err := ResolveApprovalUnitContent(root, target, unit)
	return content.Path, err
}

// ResolveApprovalUnitIn resolves unit as ResolveApprovalUnit does, among the
// carriers at carrierPath alone when it is not empty. A unit whose name two
// files of one directory declare at once (a new file whose helper collides
// with a committed one, so the package does not compile until it is renamed)
// is ambiguous by name, and the file is what names the one the judge read.
func ResolveApprovalUnitIn(root, target, unit, carrierPath string) (string, error) {
	content, err := ResolveApprovalUnitContentIn(root, target, unit, carrierPath)
	return content.Path, err
}

// ApprovalUnitContent is one resolved unit and the content an approval of it
// judged: the digest of the unit's text before the change (HEAD, or the
// target's first parent) and after it (the working tree, or the target).
// approvalAbsent stands for a side that holds no carrier.
type ApprovalUnitContent struct {
	Path    string
	Base    string
	Current string
}

// approvalAbsent is the digest of a side with no carrier: a new or a deleted unit.
const approvalAbsent = "none"

// ApprovalContentCell renders the Content cell of one approval row:
// `<base digest>..<current digest>`.
func ApprovalContentCell(content ApprovalUnitContent) string {
	return content.Base + ".." + content.Current
}

// approvalCarrierMark separates a Content cell's digests from the carrier path
// an approval was qualified with. A digest never holds it.
const approvalCarrierMark = "@"

// QualifiedApprovalContentCell renders the Content cell of a row qualified
// with the one file it judged: `<base digest>..<current digest>@<path>`. An
// empty carrierPath renders the unqualified cell.
func QualifiedApprovalContentCell(content ApprovalUnitContent, carrierPath string) string {
	if carrierPath == "" {
		return ApprovalContentCell(content)
	}
	return ApprovalContentCell(content) + approvalCarrierMark + carrierPath
}

// SplitApprovalCarrier separates a Content cell into its digest pair and the
// carrier path it was qualified with, empty for an unqualified cell.
func SplitApprovalCarrier(cell string) (digests, carrierPath string) {
	digests, carrierPath, _ = strings.Cut(cell, approvalCarrierMark)
	return digests, carrierPath
}

// ValidApprovalCarrierPath reports whether carrierPath can qualify an approval
// row: a clean repository-relative slash path that stays inside the checkout
// and fits one table cell.
func ValidApprovalCarrierPath(carrierPath string) bool {
	if carrierPath == "" {
		return false
	}
	if strings.ContainsAny(carrierPath, "|\r\n\x00\\") {
		return false
	}
	if strings.HasPrefix(carrierPath, "/") {
		return false
	}
	if carrierPath != strings.TrimSpace(carrierPath) {
		return false
	}
	if filepath.ToSlash(filepath.Clean(carrierPath)) != carrierPath {
		return false
	}
	if carrierPath == ".." {
		return false
	}
	return !strings.HasPrefix(carrierPath, "../")
}

// ParseApprovalContentCell reads a Content cell ApprovalContentCell wrote.
func ParseApprovalContentCell(cell string) (base, current string, valid bool) {
	base, current, split := strings.Cut(cell, "..")
	if !split {
		return "", "", false
	}
	if !validApprovalDigest(base) {
		return "", "", false
	}
	if !validApprovalDigest(current) {
		return "", "", false
	}
	return base, current, true
}

func validApprovalDigest(digest string) bool {
	if digest == approvalAbsent {
		return true
	}
	if len(digest) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil
}

// ApprovalContentAdmits answers whether an ordinary approval recorded with the
// Content cell (recordedBase, recordedCurrent) still judges the unit as it
// stands now. The change MUST start from the base the judge saw. An approval
// recorded before the edit (its two digests equal: the judge read the
// proposed change against an untouched unit) admits any result from that
// base; one recorded after the edit admits only the very text it judged. A
// later different change of the same unit, or a change from a moved base,
// is not the change the judge approved.
func ApprovalContentAdmits(recordedBase, recordedCurrent string, now ApprovalUnitContent) bool {
	if recordedBase != now.Base {
		return false
	}
	if recordedCurrent == recordedBase {
		return true
	}
	return recordedCurrent == now.Current
}

func approvalDigest(carriers []approvalCarrier) string {
	if len(carriers) == 0 {
		return approvalAbsent
	}
	sum := sha256.Sum256([]byte(carriers[0].text))
	return hex.EncodeToString(sum[:])
}

// ResolveApprovalUnitContent resolves unit as ResolveApprovalUnit does and
// answers the digests of its text on both sides of the change.
func ResolveApprovalUnitContent(root, target, unit string) (ApprovalUnitContent, error) {
	return ResolveApprovalUnitContentIn(root, target, unit, "")
}

// ResolveApprovalUnitContentIn resolves unit as ResolveApprovalUnitIn does and
// answers the digests of its text on both sides of the change.
func ResolveApprovalUnitContentIn(root, target, unit, carrierPath string) (ApprovalUnitContent, error) {
	if carrierPath != "" && !ValidApprovalCarrierPath(carrierPath) {
		return ApprovalUnitContent{}, errors.New("approval file qualifier must be a clean repository-relative path, got " + carrierPath)
	}
	path, base, current, err := resolveApprovalSides(root, target, unit, carrierPath)
	if err != nil {
		return ApprovalUnitContent{}, err
	}
	return ApprovalUnitContent{Path: path, Base: approvalDigest(base), Current: approvalDigest(current)}, nil
}

func resolveApprovalSides(root, target, unit, carrierPath string) (string, []approvalCarrier, []approvalCarrier, error) {
	if !ValidApprovalUnit(unit) {
		return "", nil, nil, errors.New("approval unit must be <package>.<TestName>, got " + unit)
	}
	before := "HEAD"
	if target != "" {
		if !ValidApprovalTarget(target) {
			return "", nil, nil, errors.New("approval target must be a canonical full commit SHA, got " + target)
		}
		resolved, err := approvalGit(root, "rev-parse", "--verify", target+"^{commit}")
		if err != nil {
			return "", nil, nil, err
		}
		if strings.TrimSpace(resolved) != target {
			return "", nil, nil, errors.New("approval target is not a canonical full commit SHA: " + target)
		}
		parents, err := approvalGit(root, "rev-list", "--parents", "-n", "1", target)
		if err != nil {
			return "", nil, nil, err
		}
		fields := strings.Fields(parents)
		if len(fields) < 2 {
			return "", nil, nil, errors.New("approval target has no readable parent: " + target)
		}
		before = fields[1]
	}
	oldPaths := ""
	var newPaths string
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
			return "", nil, nil, err
		}
		newPaths = currentPaths.String()
	} else {
		var err error
		oldPaths, err = approvalGit(root, "ls-tree", "-r", "--name-only", "-z", before)
		if err != nil {
			return "", nil, nil, err
		}
		if target == "" {
			newPaths, err = approvalGit(root, "ls-files", "--cached", "--others", "--exclude-standard", "-z")
		} else {
			newPaths, err = approvalGit(root, "ls-tree", "-r", "--name-only", "-z", target)
		}
		if err != nil {
			return "", nil, nil, err
		}
	}
	oldSet := approvalPathSet(oldPaths)
	newSet := approvalPathSet(newPaths)
	paths := approvalPathSet(oldPaths + newPaths)
	packageName, name := SplitApprovalUnit(unit)
	oldCarriers := make([]approvalCarrier, 0, 1)
	newCarriers := make([]approvalCarrier, 0, 1)
	unchanged := make(map[string]bool)
	for path := range paths {
		if !IsTagCarrier(path) && !strings.HasSuffix(path, ".py") {
			continue
		}
		if filepath.Base(filepath.Dir(path)) != packageName {
			continue
		}
		if carrierPath != "" && path != carrierPath {
			continue
		}
		oldText, err := approvalSide(root, before, path, oldSet[path])
		if err != nil {
			return "", nil, nil, err
		}
		newText, err := approvalSide(root, target, path, newSet[path])
		if err != nil {
			return "", nil, nil, err
		}
		oldUnits, err := approvalUnitTexts(path, oldText, name)
		if err != nil {
			return "", nil, nil, err
		}
		newUnits, err := approvalUnitTexts(path, newText, name)
		if err != nil {
			return "", nil, nil, err
		}
		if len(oldUnits) != 0 && slices.Equal(oldUnits, newUnits) {
			unchanged[path] = true
		}
		for _, text := range oldUnits {
			oldCarriers = append(oldCarriers, approvalCarrier{path: path, text: text})
		}
		for _, text := range newUnits {
			newCarriers = append(newCarriers, approvalCarrier{path: path, text: text})
		}
	}
	oldCarriers = changedApprovalCarriers(oldCarriers, unchanged)
	newCarriers = changedApprovalCarriers(newCarriers, unchanged)
	if len(oldCarriers) > 1 {
		return "", nil, nil, ambiguousApprovalCarrier(unit, oldCarriers)
	}
	if len(newCarriers) > 1 {
		return "", nil, nil, ambiguousApprovalCarrier(unit, newCarriers)
	}
	if len(oldCarriers) == 0 && len(newCarriers) == 0 {
		if carrierPath != "" {
			return "", nil, nil, errors.New("approval unit " + unit + " has no current/parent carrier in " + carrierPath)
		}
		return "", nil, nil, noApprovalCarrier(root, target, unit, newPaths)
	}
	if target != "" {
		changed := len(oldCarriers) != len(newCarriers)
		if len(oldCarriers) == 1 && len(newCarriers) == 1 {
			changed = oldCarriers[0].text != newCarriers[0].text
		}
		if !changed {
			return "", nil, nil, errors.New("approval target " + target + " does not change " + unit)
		}
	}
	if len(newCarriers) == 1 {
		return newCarriers[0].path, oldCarriers, newCarriers, nil
	}
	return oldCarriers[0].path, oldCarriers, newCarriers, nil
}

type approvalCarrier struct {
	path string
	text string
}

// changedApprovalCarriers narrows a side that holds more than one carrier to
// the carriers the change touches. A copy whose unit text is identical on both
// sides is not part of the change when it sits in another directory than every
// changed copy: the move of a test across directories leaves its old path in
// the working tree until the commit removes it, and a sibling directory's
// untouched copy is a test of another package, not what the judge approved.
//
// An unchanged copy in the same directory as a changed one is never dropped.
// Two functions of one name in one directory are one Go package declaring the
// test twice, so the side really holds both at once: that is a simultaneous
// duplicate, and it stays ambiguous however the copies changed. The path, not
// the content, is what tells the two apart, because the resolver runs before
// the commit names the paths it removes. A side whose every carrier is
// unchanged keeps them all, so a real duplicate stays ambiguous, and so does
// a duplicate where both copies change.
func changedApprovalCarriers(carriers []approvalCarrier, unchanged map[string]bool) []approvalCarrier {
	if len(carriers) < 2 {
		return carriers
	}
	directories := make(map[string]bool, len(carriers))
	for _, carrier := range carriers {
		if unchanged[carrier.path] {
			continue
		}
		directories[filepath.Dir(carrier.path)] = true
	}
	if len(directories) == 0 {
		return carriers
	}
	kept := make([]approvalCarrier, 0, len(carriers))
	for _, carrier := range carriers {
		if unchanged[carrier.path] && !directories[filepath.Dir(carrier.path)] {
			continue
		}
		kept = append(kept, carrier)
	}
	return kept
}

// AmbiguousCarrierError refuses a unit whose name more than one carrier
// declares on one side of the change. Paths are the carriers, sorted, so a
// caller can print the file-qualified form that names each one.
type AmbiguousCarrierError struct {
	Unit  string
	Paths []string
}

func (e *AmbiguousCarrierError) Error() string {
	return "approval unit " + e.Unit + " is ambiguous across carriers: " + strings.Join(e.Paths, ", ")
}

func ambiguousApprovalCarrier(unit string, carriers []approvalCarrier) error {
	paths := make([]string, len(carriers))
	for index, carrier := range carriers {
		paths[index] = carrier.path
	}
	slices.Sort(paths)
	return &AmbiguousCarrierError{Unit: unit, Paths: paths}
}

// noApprovalCarrier refuses a unit no directory carries. A unit is keyed by
// its directory's base name, the key every consumer reads (the weakening
// gate, the edit hook, the RFC gate), so a Go package clause that differs
// from its directory (`airules` in internal/le/ai/rules) names nothing they
// would ever match. When a Go package clause equal to the given package
// carries the test, the refusal names the directory key to record instead.
// listed is the current (or target) side's path listing the resolver read.
func noApprovalCarrier(root, target, unit, listed string) error {
	packageName, name := SplitApprovalUnit(unit)
	refusal := "approval unit " + unit + " has no current/parent carrier"
	keys := make([]string, 0, 1)
	for path := range approvalPathSet(listed) {
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		if !IsTagCarrier(path) {
			continue
		}
		text, readErr := approvalSide(root, target, path, true)
		if readErr != nil {
			continue
		}
		if strings.TrimSuffix(goPackageClause(text), "_test") != packageName {
			continue
		}
		units, unitErr := approvalUnitTexts(path, text, name)
		if unitErr != nil {
			continue
		}
		if len(units) == 0 {
			continue
		}
		key := filepath.Base(filepath.Dir(path)) + "." + name
		if !slices.Contains(keys, key) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return errors.New(refusal)
	}
	slices.Sort(keys)
	return errors.New(refusal + ": " + packageName + " is a Go package clause, and approvals are keyed by the directory that holds the test; record " + strings.Join(keys, " or ") + " instead")
}

// goPackageClause answers the name a Go file's package clause declares, or
// empty when the file has none that parses.
func goPackageClause(content string) string {
	file, err := parser.ParseFile(token.NewFileSet(), "", content, parser.PackageClauseOnly)
	// refuse-by-default: guards noApprovalCarrier -- a file with no parsable clause matches no package, so it suggests no key
	if err != nil {
		return ""
	}
	return file.Name.Name
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
	// The weakening scope names evidence that sits outside every function
	// by the file stem (weakenedUnits, pkg.ignore_test), so a unit named for
	// the stem is the whole file, as in a file-scoped carrier.
	if name == fileStem(path) {
		return []string{content}, nil
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
	// Every query here is a read, and the edit hook reaches it on every
	// session's edit over one shared index, so it carries gitenv.ReadOnlyQuery.
	command := exec.CommandContext(context.Background(), "git", args...) // #nosec G204 -- Fixed Git executable with separately passed query operands.
	command.Dir = root
	command.Env = gitenv.ReadOnlyQuery(os.Environ())
	var complaint bytes.Buffer
	command.Stderr = &complaint
	content, err := command.Output()
	if err != nil {
		return "", fmt.Errorf("approval identity: git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(complaint.String()))
	}
	return string(content), nil
}
