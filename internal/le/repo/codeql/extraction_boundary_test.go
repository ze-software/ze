// Design: docs/architecture/testing/ci-workflows.md -- bundle and source identities bound the database proof.
package codeql

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestExtractionProofUsesSelectedBundle drives the native check through a
// bundle-shaped executable boundary. Libraries are in codeql/qlpacks, beside
// the codeql/codeql executable, as the real bundle archive lays them out; no
// package-install fallback is accepted by the process.
func TestExtractionProofUsesSelectedBundle(t *testing.T) {
	for _, library := range []bool{true, false} {
		name := "bundled Go library present"
		if !library {
			name = "bundled Go library absent"
		}
		t.Run(name, func(t *testing.T) {
			root, manifest, proof := extractionProofFixture(t)
			tool, environment := extractionBundleFixture(t, extractionBoundaryCSV(t,
				[]string{extractionAuthoredFixture, proof["authored-file"]}), library)
			environment = append(environment, "CODEQL_BOUNDARY_ENFORCE_BUNDLE=1")
			output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
			if library {
				if code != 0 || !strings.Contains(output, "authored sentinel extracted") {
					t.Fatalf("selected bundle was unusable: exit %d\n%s", code, output)
				}
				return
			}
			if code == 0 || !strings.Contains(output, "CodeQL query failed") || !strings.Contains(output, "bundled Go library is unavailable") {
				t.Fatalf("missing bundled library did not refuse with its cause: exit %d\n%s", code, output)
			}
			extractionBoundaryNoReport(t, manifest, output)
		})
	}
}

// TestExtractionProofRechecksSourcesAfterCodeQL changes one prepared source
// only after a successful query or CSV decode at the process boundary. A later
// source identity must refuse without writing a successful evidence report.
func TestExtractionProofRechecksSourcesAfterCodeQL(t *testing.T) {
	for _, operation := range []string{"query run", "bqrs decode"} {
		for _, field := range []string{"authored", "cache"} {
			t.Run(operation+"/"+field, func(t *testing.T) {
				root, manifest, proof := extractionProofFixture(t)
				tool, environment := extractionBundleFixture(t, extractionBoundaryCSV(t,
					[]string{extractionAuthoredFixture, proof["authored-file"]}), true)
				environment = append(environment, "CODEQL_BOUNDARY_MUTATE_AFTER="+operation, "CODEQL_BOUNDARY_MUTATE_FILE="+proof[field+"-file"])
				output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
				if code == 0 || !strings.Contains(output, field+" sentinel changed since preparation") {
					t.Fatalf("source changed after %s but certified successfully: exit %d\n%s", operation, code, output)
				}
				extractionBoundaryNoReport(t, manifest, output)
			})
		}
	}
}

// TestExtractionProofPathColumnsRetainEverySource pairs allowed outside-source
// rows with cache rows lacking a relative path, and requires both columns to
// identify the same file before the authored positive control can certify it.
func TestExtractionProofPathColumnsRetainEverySource(t *testing.T) {
	for _, kind := range []string{"outside-source", "cache-without-relative", "mismatched-columns"} {
		t.Run(kind, func(t *testing.T) {
			root, manifest, proof := extractionProofFixture(t)
			rows := [][]string{{extractionAuthoredFixture, proof["authored-file"]}}
			want := ""
			switch kind {
			case "outside-source":
				outside := filepath.Join(t.TempDir(), "dependency.go")
				if err := os.WriteFile(outside, []byte("package dependency\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				rows = append(rows, []string{"", outside})
			case "cache-without-relative":
				rows = append(rows, []string{"", proof["cache-file"]})
				want = "contains cache source"
			case "mismatched-columns":
				rows = append(rows, []string{"authored/base.go", proof["authored-file"]})
				want = "path columns name different sources"
			default:
				t.Fatal("unknown source-column fixture")
			}
			tool, environment := extractionCodeQLFixture(t, extractionBoundaryCSV(t, rows...))
			output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
			if want == "" {
				if code != 0 || !strings.Contains(output, "authored sentinel extracted") {
					t.Fatalf("valid outside-source row was lost or refused: exit %d\n%s", code, output)
				}
				return
			}
			if code == 0 || !strings.Contains(output, want) {
				t.Fatalf("unsafe source columns passed or refused for the wrong reason; want %q: exit %d\n%s", want, code, output)
			}
			extractionBoundaryNoReport(t, manifest, output)
		})
	}
}

// TestExtractionProofRefusesManifestCacheDirectoryAlias supplies a matching
// hash through a cache-looking directory alias. Only canonical confinement
// prevents this manifest from referring to a file outside the cache boundary.
func TestExtractionProofRefusesManifestCacheDirectoryAlias(t *testing.T) {
	root, manifest, proof := extractionProofFixture(t)
	outside := t.TempDir()
	body := []byte("package outside\nfunc ForeignSentinel() int { return 29 }\n")
	if err := os.WriteFile(filepath.Join(outside, "sentinel.go"), body, 0o600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(proof["cache-file"]), "outside")
	if err := os.Symlink(outside, alias); err != nil {
		t.Fatal(err)
	}
	proof["cache-file"] = filepath.Join(alias, "sentinel.go")
	proof["cache-sha256"] = extractionFixtureHash(body)
	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	tool, environment := extractionCodeQLFixture(t, extractionBoundaryCSV(t,
		[]string{extractionAuthoredFixture, proof["authored-file"]}))
	calls := filepath.Join(t.TempDir(), "codeql-started")
	environment = append(environment, "CODEQL_PROOF_CALLS="+calls)
	output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
	if code == 0 || !strings.Contains(output, "manifest cache sentinel is outside gokrazy/modcache after resolving symlinks") {
		t.Fatalf("foreign cache alias was accepted or lost its cause: exit %d\n%s", code, output)
	}
	if _, err := os.Stat(calls); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("CodeQL ran before manifest cache confinement was established: %v", err)
	}
	extractionBoundaryNoReport(t, manifest, output)
}

func extractionBoundaryNoReport(t *testing.T, manifest, output string) {
	t.Helper()
	if strings.Contains(output, "authored sentinel extracted") {
		t.Fatalf("refusal printed a successful extraction verdict: %s", output)
	}
	reports, err := filepath.Glob(filepath.Join(filepath.Dir(manifest), "codeql-proof-*", "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 0 {
		t.Fatalf("refusal retained a successful evidence report: %v", reports)
	}
}

func extractionBoundaryCSV(t *testing.T, rows ...[]string) string {
	t.Helper()
	var data bytes.Buffer
	writer := csv.NewWriter(&data)
	if err := writer.WriteAll(rows); err != nil {
		t.Fatal(err)
	}
	return data.String()
}

func extractionBundleFixture(t *testing.T, rows string, library bool) (string, []string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	toolcache := t.TempDir()
	fixtureWrite(t, toolcache, "files.csv", rows)
	fixtureWrite(t, toolcache, "codeql/placeholder", "owned bundle fixture\n")
	if library {
		fixtureWrite(t, toolcache, "codeql/qlpacks/codeql/go-all/fixture/qlpack.yml", "name: codeql/go-all\nversion: 0.0.0\nlibrary: true\n")
		fixtureWrite(t, toolcache, "codeql/qlpacks/codeql/go-all/fixture/go.qll", "// Synthetic bundled Go library; this process fixture does not compile QL.\n")
	}
	toolcache, err = filepath.EvalSymlinks(toolcache)
	if err != nil {
		t.Fatal(err)
	}
	// The unpacked archive is one codeql/ directory: the executable and its
	// qlpacks sit side by side in it, as in .../CodeQL/<version>/x64/codeql.
	bundle := filepath.Join(toolcache, "codeql")
	tool := filepath.Join(bundle, "codeql")
	writeExtractionExecutable(t, tool, "#!/bin/sh\nexec \"$CODEQL_BOUNDARY_TEST_BINARY\" -test.run '^TestExtractionBundleProcess$' -- \"$@\"\n")
	return tool, []string{"CODEQL_BOUNDARY_CHILD=1", "CODEQL_BOUNDARY_TEST_BINARY=" + binary,
		"CODEQL_BOUNDARY_BUNDLE=" + bundle, "CODEQL_BOUNDARY_CSV=" + filepath.Join(toolcache, "files.csv")}
}

// TestExtractionBundleProcess checks the selected bundle and query-pack
// contract, and can change synthetic sources after producing successful output.
// It substitutes only the foreign CodeQL process, never the native checker.
func TestExtractionBundleProcess(t *testing.T) {
	if os.Getenv("CODEQL_BOUNDARY_CHILD") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("CodeQL bundle fixture has no argument boundary")
	}
	arguments := os.Args[separator+1:]
	if len(arguments) < 3 {
		t.Fatalf("incomplete CodeQL bundle invocation: %q", arguments)
	}
	operation := arguments[0] + " " + arguments[1]
	output := ""
	for _, argument := range arguments {
		if value, found := strings.CutPrefix(argument, "--output="); found {
			output = value
		}
	}
	if output == "" {
		t.Fatalf("CodeQL bundle fixture has no explicit output: %q", arguments)
	}
	switch operation {
	case "query run":
		if os.Getenv("CODEQL_BOUNDARY_ENFORCE_BUNDLE") == "1" {
			packs := filepath.Join(os.Getenv("CODEQL_BOUNDARY_BUNDLE"), "qlpacks")
			if !slices.Contains(arguments, "--search-path="+packs) {
				t.Fatalf("query does not use the selected bundle's library directory %q: %q", packs, arguments)
			}
			if _, err := os.Stat(filepath.Join(packs, "codeql", "go-all", "fixture", "qlpack.yml")); err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				if _, err := os.Stderr.WriteString("bundled Go library is unavailable\n"); err != nil {
					t.Fatal(err)
				}
				os.Exit(78)
			}
		}
		query := arguments[len(arguments)-1]
		data, err := os.ReadFile(query)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(strings.TrimSpace(string(data)), "import go\n") {
			t.Fatalf("query has no Go library import: %s", data)
		}
		data, err = os.ReadFile(filepath.Join(filepath.Dir(query), "qlpack.yml"))
		if err != nil {
			t.Fatal(err)
		}
		var pack struct {
			Name         string            `yaml:"name"`
			Dependencies map[string]string `yaml:"dependencies"`
		}
		if err := yaml.Unmarshal(data, &pack); err != nil {
			t.Fatal(err)
		}
		if pack.Name != "ze/extraction-proof" || pack.Dependencies["codeql/go-all"] != "*" {
			t.Fatalf("query pack does not select its bundled Go library: %s", data)
		}
		if err := os.WriteFile(output, []byte("synthetic bundle BQRS\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "bqrs decode":
		if !slices.Contains(arguments, "--format=csv") || !slices.Contains(arguments, "--no-titles") {
			t.Fatalf("CodeQL bundle decoder has no exact CSV contract: %q", arguments)
		}
		data, err := os.ReadFile(os.Getenv("CODEQL_BOUNDARY_CSV"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, data, 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unexpected CodeQL bundle operation, including dependency installation: %q", arguments)
	}
	if os.Getenv("CODEQL_BOUNDARY_MUTATE_AFTER") == operation {
		if err := os.WriteFile(os.Getenv("CODEQL_BOUNDARY_MUTATE_FILE"), []byte("package changedaftercodeql\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	os.Exit(0)
}

// TestExtractionBundlePacksFollowsTheArchiveLayout pins where the bundle's
// query packs are looked for. The tool cache holds .../x64/codeql/codeql beside
// .../x64/codeql/qlpacks, and nothing at .../x64/qlpacks (the CI refusal
// "lstat .../x64/qlpacks: no such file or directory"). Each case builds that
// tool cache: the real layout must answer its own codeql/qlpacks even when a
// complete decoy library sits one level up, and a library found only one level
// up must refuse, because it is outside the selected bundle.
func TestExtractionBundlePacksFollowsTheArchiveLayout(t *testing.T) {
	library := func(t *testing.T, root, packs string) {
		t.Helper()
		fixtureWrite(t, root, packs+"/codeql/go-all/fixture/qlpack.yml", "name: codeql/go-all\nversion: 0.0.0\nlibrary: true\n")
		fixtureWrite(t, root, packs+"/codeql/go-all/fixture/go.qll", "// Synthetic bundled Go library.\n")
	}
	toolcache := func(t *testing.T) string {
		t.Helper()
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		fixtureWrite(t, root, "codeql/placeholder", "owned bundle fixture\n")
		writeExtractionExecutable(t, filepath.Join(root, "codeql", "codeql"), "#!/bin/sh\nexit 0\n")
		return root
	}
	t.Run("archive layout beside a decoy one level up", func(t *testing.T) {
		root := toolcache(t)
		library(t, root, "codeql/qlpacks")
		library(t, root, "qlpacks")
		packs, err := extractionBundlePacks(filepath.Join(root, "codeql", "codeql"))
		if err != nil {
			t.Fatalf("real bundle layout refused: %v", err)
		}
		if want := filepath.Join(root, "codeql", "qlpacks"); packs != want {
			t.Fatalf("query packs resolved to %q, want the bundle's own %q", packs, want)
		}
	})
	t.Run("library only one level up", func(t *testing.T) {
		root := toolcache(t)
		library(t, root, "qlpacks")
		_, err := extractionBundlePacks(filepath.Join(root, "codeql", "codeql"))
		if err == nil || !strings.Contains(err.Error(), "bundled Go library is unavailable") {
			t.Fatalf("library outside the bundle was accepted: %v", err)
		}
	})
}
