// Design: docs/contributing/spec-workflow.md -- bounded native source-proof authority.
package aihooks

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/arch/tier"
	docconsistency "github.com/ze-software/ze/internal/le/doc/consistency"
	docindex "github.com/ze-software/ze/internal/le/doc/index"
	docste "github.com/ze-software/ze/internal/le/doc/ste"
	"github.com/ze-software/ze/internal/le/heavytool"
	"github.com/ze-software/ze/internal/le/hookruntime"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/session"
	"github.com/ze-software/ze/internal/le/spec"
	speccitation "github.com/ze-software/ze/internal/le/spec/citation"
	specjournal "github.com/ze-software/ze/internal/le/spec/journal"
	specmechanisms "github.com/ze-software/ze/internal/le/spec/mechanisms"
	specpath "github.com/ze-software/ze/internal/le/spec/path"
	specstatus "github.com/ze-software/ze/internal/le/spec/status"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// hookAuthorityPackages selects direct package scopes from the actual producer
// locations and existing fixture mappings, independently of the watched inputs.
// Each new package boundary needs a named producer here; new helpers in these
// packages require no filename bookkeeping. All platform variants are included.
func hookAuthorityPackages(t *testing.T, checkout string) []string {
	t.Helper()
	producers := []any{
		hookruntime.Probe,
		session.EndSummary,
		testweakened.Proposed,
		rfc.TrailerLanded,
		spec.CheckWIPTransition,
		specpath.IsSpec,
		specmechanisms.Problem,
		specstatus.MetadataStatus,
		speccitation.AuditAnchors,
		docindex.DocumentsByPath,
		docconsistency.Check,
		lepath.SessionCommitSession,
		// specproof consumes this package's DisableableNonProdPrefixes.
		archtier.Check,
		// writeYangDescription consumes WordCount and MaxDescriptiveWords.
		docste.WordCount,
		// postJournal consumes the native journal verdict.
		specjournal.ValidateFile,
		// bashSubagentHeavy and the raw-job guard consume the heavy-tool
		// definition (Program, Split, LeWords, Linter).
		heavytool.Program,
	}
	packages := make(map[string]bool)
	for _, producer := range producers {
		packages[filepath.ToSlash(filepath.Dir(hookProducerSource(t, checkout, producer)))] = true
	}
	for _, category := range fixtureCategories {
		packages[filepath.ToSlash(filepath.Dir(category.owner))] = true
	}
	for _, boundary := range fixtureProducerBoundaries {
		packages[filepath.ToSlash(filepath.Dir(boundary.actionOwner))] = true
		packages[filepath.ToSlash(filepath.Dir(boundary.nativeOwner))] = true
	}
	paths := make([]string, 0, len(packages))
	for path := range packages {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	return paths
}

// hookAuthorityFiles independently discovers the direct non-test Go population.
// It does not inspect hookSourcePaths or any production scope declaration.
func hookAuthorityFiles(t *testing.T, checkout string, packages []string) []string {
	t.Helper()
	var files []string
	for _, directory := range packages {
		entries, err := os.ReadDir(filepath.Join(checkout, filepath.FromSlash(directory)))
		if err != nil {
			t.Fatalf("read authority package %s: %v", directory, err)
		}
		found := false
		for _, entry := range entries {
			if !strings.HasSuffix(entry.Name(), ".go") {
				continue
			}
			if strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			files = append(files, directory+"/"+entry.Name())
			found = true
		}
		if !found {
			t.Fatalf("authority package %s has no production Go source", directory)
		}
	}
	slices.Sort(files)
	return files
}

type hookAuthorityFile struct {
	path string
	body []byte
	mode os.FileMode
}

// hookAuthorityFixture copies the bounded source population once per test, not
// once per mutation. Each subtest restores that same root before the next case.
func hookAuthorityFixture(t *testing.T) (string, []string, []hookAuthorityFile) {
	t.Helper()
	checkout := hookCheckout(t)
	packages := hookAuthorityPackages(t, checkout)
	paths := hookAuthorityFiles(t, checkout, packages)
	root := t.TempDir()
	files := make([]hookAuthorityFile, 0, len(paths))
	for _, relative := range paths {
		path := filepath.Join(checkout, filepath.FromSlash(relative))
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("authority source %s is not regular", relative)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file := hookAuthorityFile{path: relative, body: body, mode: info.Mode().Perm()}
		hookAuthorityRestore(t, root, file)
		files = append(files, file)
	}
	return root, packages, files
}

func hookAuthorityRestore(t *testing.T, root string, file hookAuthorityFile) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(file.path))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, file.body, file.mode); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, file.mode); err != nil {
		t.Fatal(err)
	}
}

func hookAuthorityBaseline(t *testing.T, root string) [sha256.Size]byte {
	t.Helper()
	baseline, err := hookSourceContentDigest(root)
	if err != nil {
		t.Fatalf("compute unchanged source baseline: %v", err)
	}
	hookAuthorityAccept(t, root, baseline)
	return baseline
}

func hookAuthorityAccept(t *testing.T, root string, baseline [sha256.Size]byte) {
	t.Helper()
	result := checkHookSourcesAgainst(root, baseline)
	if !result.Passed || result.Code != 0 {
		t.Fatalf("unchanged/restored sources refused by actual comparison: %+v", result)
	}
}

func hookAuthorityRefuse(t *testing.T, root string, baseline [sha256.Size]byte) {
	t.Helper()
	result := checkHookSourcesAgainst(root, baseline)
	if result.Passed || result.Code != 2 {
		t.Errorf("changed authority accepted by actual comparison: %+v", result)
	}
}

// TestHookAuthorityContentInvalidatesComputedProof mutates every independently
// discovered authority file, checks the actual comparison, and restores one root.
// VALIDATES: unlisted policy helpers cannot retain the computed positive proof.
// PREVENTS: a stale stored seal or a self-selected file list proving rejection.
func TestHookAuthorityContentInvalidatesComputedProof(t *testing.T) {
	root, _, files := hookAuthorityFixture(t)
	baseline := hookAuthorityBaseline(t, root)
	for _, file := range files {
		t.Run(file.path, func(t *testing.T) {
			defer hookAuthorityRestore(t, root, file)
			path := filepath.Join(root, filepath.FromSlash(file.path))
			if err := os.WriteFile(path, append(slices.Clone(file.body), '\n'), file.mode); err != nil {
				t.Fatal(err)
			}
			hookAuthorityRefuse(t, root, baseline)
		})
		hookAuthorityAccept(t, root, baseline)
	}
}

// TestHookAuthorityPopulationInvalidatesComputedProof changes identity and future
// membership without changing existing bytes, then checks the real comparison.
// VALIDATES: package membership, pathname and mode are inputs to the proof.
// PREVENTS: a central filename list silently excluding future authority helpers.
func TestHookAuthorityPopulationInvalidatesComputedProof(t *testing.T) {
	root, _, files := hookAuthorityFixture(t)
	baseline := hookAuthorityBaseline(t, root)
	file := files[0]
	path := filepath.Join(root, filepath.FromSlash(file.path))
	t.Run("rename-without-byte-change", func(t *testing.T) {
		renamed := path + ".renamed.go"
		if err := os.Rename(path, renamed); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Rename(renamed, path); err != nil {
				t.Fatal(err)
			}
		}()
		hookAuthorityRefuse(t, root, baseline)
	})
	hookAuthorityAccept(t, root, baseline)
	for _, target := range []struct {
		name     string
		producer any
	}{{"runtime", hookruntime.Probe}, {"external", rfc.TrailerLanded}} {
		t.Run("future-helper-"+target.name, func(t *testing.T) {
			directory := filepath.Dir(hookProducerSource(t, hookCheckout(t), target.producer))
			helper := filepath.Join(root, directory, "future_source_proof_helper.go")
			if err := os.WriteFile(helper, []byte("package future\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Remove(helper); err != nil {
					t.Fatal(err)
				}
			}()
			hookAuthorityRefuse(t, root, baseline)
		})
		hookAuthorityAccept(t, root, baseline)
	}
	t.Run("delete-member", func(t *testing.T) {
		defer hookAuthorityRestore(t, root, file)
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		// Either discovery failure or a changed population MUST refuse here.
		hookAuthorityRefuse(t, root, baseline)
	})
	hookAuthorityAccept(t, root, baseline)
	t.Run("executable-mode", func(t *testing.T) {
		defer hookAuthorityRestore(t, root, file)
		if err := os.Chmod(path, file.mode^0o111); err != nil {
			t.Fatal(err)
		}
		hookAuthorityRefuse(t, root, baseline)
	})
	hookAuthorityAccept(t, root, baseline)
}

// TestHookAuthorityUnavailableFailsClosed substitutes unusable files and package
// boundaries while retaining the computed baseline's unchanged positive control.
// VALIDATES: missing, empty, unreadable and ambiguous authorities grant no proof.
// PREVENTS: symlink following or a partial population becoming a clean verdict.
func TestHookAuthorityUnavailableFailsClosed(t *testing.T) {
	root, packages, files := hookAuthorityFixture(t)
	baseline := hookAuthorityBaseline(t, root)
	file := files[0]
	path := filepath.Join(root, filepath.FromSlash(file.path))
	for _, failure := range []string{"truncate", "unreadable", "nonregular", "symlink"} {
		t.Run("source-"+failure, func(t *testing.T) {
			defer hookAuthorityRestore(t, root, file)
			switch failure {
			case "truncate":
				if err := os.WriteFile(path, nil, file.mode); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.Chmod(path, file.mode); err != nil {
						t.Fatal(err)
					}
				}()
			case "nonregular", "symlink":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if failure == "nonregular" {
					if err := os.Mkdir(path, 0o750); err != nil {
						t.Fatal(err)
					}
				} else {
					target := filepath.Join(root, "same-content-source.go")
					if err := os.WriteFile(target, file.body, file.mode); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
				defer func() {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}()
			}
			if failure == "truncate" {
				hookAuthorityRefuse(t, root, baseline)
			} else {
				hookAuthorityRequireReadError(t, root, baseline)
			}
		})
		hookAuthorityAccept(t, root, baseline)
	}
	// The final selected package has no selected child package.
	directory := filepath.Join(root, filepath.FromSlash(packages[len(packages)-1]))
	for _, failure := range []string{"missing", "empty", "unreadable", "nonregular", "symlink"} {
		t.Run("scope-"+failure, func(t *testing.T) {
			if failure == "unreadable" {
				if err := os.Chmod(directory, 0); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := os.Chmod(directory, 0o750); err != nil {
						t.Fatal(err)
					}
				}()
				hookAuthorityRequireReadError(t, root, baseline)
				return
			}
			backup := filepath.Join(root, "source-proof-scope-backup")
			if err := os.Rename(directory, backup); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.Rename(backup, directory); err != nil {
					t.Fatal(err)
				}
			}()
			switch failure {
			case "empty":
				if err := os.Mkdir(directory, 0o750); err != nil {
					t.Fatal(err)
				}
			case "nonregular":
				if err := os.WriteFile(directory, []byte("not a scope\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				if err := os.Symlink(backup, directory); err != nil {
					t.Fatal(err)
				}
			}
			if failure != "missing" {
				defer func() {
					if err := os.Remove(directory); err != nil {
						t.Fatal(err)
					}
				}()
			}
			hookAuthorityRequireReadError(t, root, baseline)
		})
		hookAuthorityAccept(t, root, baseline)
	}
	t.Run("symlink-scope-ancestor", func(t *testing.T) {
		ancestor := filepath.Join(root, "internal")
		backup := filepath.Join(root, "source-proof-internal-backup")
		if err := os.Rename(ancestor, backup); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := os.Remove(ancestor); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(backup, ancestor); err != nil {
				t.Fatal(err)
			}
		}()
		if err := os.Symlink(backup, ancestor); err != nil {
			t.Fatal(err)
		}
		hookAuthorityRequireReadError(t, root, baseline)
	})
	hookAuthorityAccept(t, root, baseline)
	t.Run("symlink-root", func(t *testing.T) {
		alias := filepath.Join(t.TempDir(), "root-alias")
		if err := os.Symlink(root, alias); err != nil {
			t.Fatal(err)
		}
		hookAuthorityRequireReadError(t, alias, baseline)
	})
	for _, unknown := range []string{"", t.TempDir(), filepath.Join(t.TempDir(), "missing")} {
		hookAuthorityRequireReadError(t, unknown, baseline)
	}
}

func hookAuthorityRequireReadError(t *testing.T, root string, baseline [sha256.Size]byte) {
	t.Helper()
	if _, err := hookSourceContentDigest(root); err == nil {
		t.Error("unusable authority produced a source digest")
	}
	hookAuthorityRefuse(t, root, baseline)
}

// TestHookAuthorityOrderAndExclusions preserves proof across directory insertion
// order and changes deliberately outside the direct production package scopes.
// VALIDATES: deterministic supplied-root identity, not host or traversal order.
// PREVENTS: test files, nested packages and unrelated sources entering the claim.
func TestHookAuthorityOrderAndExclusions(t *testing.T) {
	root, _, files := hookAuthorityFixture(t)
	baseline := hookAuthorityBaseline(t, root)
	for _, file := range files {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(file.path))); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range slices.Backward(files) {
		hookAuthorityRestore(t, root, file)
	}
	hookAuthorityAccept(t, root, baseline)
	directory := filepath.Dir(files[0].path)
	for _, relative := range []string{
		filepath.Join(directory, "excluded_source_proof_test.go"),
		filepath.Join(directory, "child", "outside.go"),
		"outside/source_proof.go",
	} {
		file := hookAuthorityFile{path: relative, body: []byte("package outside\n"), mode: 0o600}
		hookAuthorityRestore(t, root, file)
		hookAuthorityAccept(t, root, baseline)
		file.body = []byte("package changed\n")
		hookAuthorityRestore(t, root, file)
		hookAuthorityAccept(t, root, baseline)
	}
	// An independent physical root has the same relative identities and bytes.
	other := t.TempDir()
	for _, file := range files {
		hookAuthorityRestore(t, other, file)
	}
	hookAuthorityAccept(t, other, baseline)
}

// TestHookAuthorityContentFraming moves delimiter-shaped bytes between two
// consecutive sorted sources with identical delimiter-concatenated bytes.
// VALIDATES: content boundaries are length-framed, not inferred from source bytes.
// PREVENTS: distinct source trees retaining an identical ambiguous digest input.
func TestHookAuthorityContentFraming(t *testing.T) {
	root, _, files := hookAuthorityFixture(t)
	if len(files) < 2 {
		t.Fatal("content framing needs two independently selected sources")
	}
	first, second := files[0], files[1]
	defer hookAuthorityRestore(t, root, first)
	defer hookAuthorityRestore(t, root, second)
	boundary := "\x00" + second.path + "\x00"
	first.body = []byte("left")
	second.body = []byte("middle" + boundary + "right")
	hookAuthorityRestore(t, root, first)
	hookAuthorityRestore(t, root, second)
	baseline := hookAuthorityBaseline(t, root)
	first.body = []byte("left" + boundary + "middle")
	second.body = []byte("right")
	hookAuthorityRestore(t, root, first)
	hookAuthorityRestore(t, root, second)
	hookAuthorityRefuse(t, root, baseline)
}
