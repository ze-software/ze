// Design: docs/architecture/testing/ci-workflows.md -- CodeQL extraction evidence comes from its database.
package codeql

import (
	"bytes"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const extractionAuthoredFixture = "test/codeql/authored_sentinel.go"

// TestExtractionPreparationGuardsExactCacheSentinel drives the registered
// preparation entry. Its explicit cache request must fail before Go loads that
// package, and both real source files must be identified in the saved manifest.
func TestExtractionPreparationGuardsExactCacheSentinel(t *testing.T) {
	root := fixtureRepository(t, map[string]string{
		extractionAuthoredFixture: "package codeqlsentinel\nfunc AuthoredSentinel() int { return 17 }\n",
	})
	manifest := filepath.Join(t.TempDir(), "proof.json")
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	tripwire := filepath.Join(t.TempDir(), "cache-loader-started")
	tools := t.TempDir()
	shim := "#!/bin/sh\nfor argument do\n case \"$argument\" in *gokrazy/modcache/*) : > \"$CODEQL_CACHE_TRIPWIRE\"; exit 91;; esac\ndone\nexec \"$CODEQL_REAL_GO\" \"$@\"\n"
	writeExtractionExecutable(t, filepath.Join(tools, "go"), shim)
	output, code := extractionEntry(t, root, []string{"prepare", "file", manifest, "tags", "ze_setup"},
		"PATH="+tools+string(os.PathListSeparator)+os.Getenv("PATH"), "CODEQL_REAL_GO="+realGo, "CODEQL_CACHE_TRIPWIRE="+tripwire)
	if code != 0 {
		t.Fatalf("registered extraction preparation refused its owned fixture: exit %d\n%s", code, output)
	}
	if _, err := os.Stat(tripwire); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("explicit cache request reached the Go loader before refusal: %v", err)
	}
	data, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatalf("preparation saved no proof manifest: %v", err)
	}
	var proof map[string]string
	if err := json.Unmarshal(data, &proof); err != nil {
		t.Fatalf("proof manifest is not structured data: %v", err)
	}
	if proof["source-root"] != root || proof["authored-file"] != filepath.Join(root, extractionAuthoredFixture) || proof["tags"] != "ze_setup" {
		t.Fatalf("manifest does not identify its exact authored source/context: %s", data)
	}
	cache, err := filepath.Rel(root, proof["cache-file"])
	if err != nil || !strings.HasPrefix(filepath.ToSlash(cache), "gokrazy/modcache/") {
		t.Fatalf("manifest's cache sentinel escaped its cache population: %s (%v)", data, err)
	}
	for _, field := range []string{"authored", "cache"} {
		body, err := os.ReadFile(proof[field+"-file"])
		if err != nil {
			t.Fatalf("%s sentinel is missing: %v", field, err)
		}
		if extractionFixtureHash(body) != proof[field+"-sha256"] {
			t.Fatalf("%s sentinel is not bound to the bytes prepared: %s", field, data)
		}
	}
	if !strings.Contains(proof["cache-refusal"], "CodeQL refuses gokrazy/modcache path") {
		t.Fatalf("manifest claims no actual explicit-cache refusal: %s", data)
	}
}

// TestExtractionPreparationPreservesExistingManifest rejects an occupied
// output before creating any cache input or replacing the earlier evidence.
func TestExtractionPreparationPreservesExistingManifest(t *testing.T) {
	root := fixtureRepository(t, map[string]string{
		extractionAuthoredFixture: "package codeqlsentinel\nfunc AuthoredSentinel() int { return 17 }\n",
	})
	manifest := filepath.Join(t.TempDir(), "original.json")
	original := []byte("retained original evidence\n")
	if err := os.WriteFile(manifest, original, 0o600); err != nil {
		t.Fatal(err)
	}
	output, code := extractionEntry(t, root, []string{"prepare", "file", manifest, "tags", "ze_setup"})
	if code == 0 || !strings.Contains(output, "manifest already exists") {
		t.Fatalf("occupied manifest did not refuse before work: exit %d\n%s", code, output)
	}
	retained, err := os.ReadFile(manifest)
	if err != nil || !bytes.Equal(original, retained) {
		t.Fatalf("preparation changed existing evidence: %q (%v)", retained, err)
	}
	if _, err := os.Stat(filepath.Join(root, "gokrazy")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refused preparation created cache inputs: %v", err)
	}
}

// TestExtractionProofChecksEveryDatabaseRow uses a CodeQL-process stand-in,
// never a selector result. It preserves an authored positive control while
// making any cache row, including an older row and canonical alias, refuse.
func TestExtractionProofChecksEveryDatabaseRow(t *testing.T) {
	for _, test := range []struct{ name, kind, refusal string }{
		{"authored extracted", "valid", ""},
		{"authored missing", "missing", "authored sentinel was not extracted"},
		{"cache alongside authored", "cache", "contains cache source"},
		{"older cache row", "older-cache", "contains cache source"},
		{"canonical cache alias", "alias", "contains cache source"},
		{"empty extraction", "empty", "contains no extracted Go files"},
		{"malformed CSV", "malformed", "read extracted Go files"},
		{"wrong columns", "columns", "two path columns"},
		{"relative absolute column", "relative", "absolute source path"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, manifest, proof := extractionProofFixture(t)
			authored := []string{extractionAuthoredFixture, proof["authored-file"]}
			cacheRelative, err := filepath.Rel(root, proof["cache-file"])
			if err != nil {
				t.Fatal(err)
			}
			cache := []string{filepath.ToSlash(cacheRelative), proof["cache-file"]}
			rows := [][]string{authored}
			switch test.kind {
			case "valid":
			case "missing":
				ordinary := filepath.Join(root, "authored/base.go")
				rows = [][]string{{"authored/base.go", ordinary}}
			case "cache":
				rows = append(rows, cache)
			case "older-cache":
				const olderRelative = "gokrazy/modcache/older-extraction/sentinel.go"
				fixtureWrite(t, root, olderRelative, "package oldercache\nfunc OlderCacheSentinel() int { return 23 }\n")
				rows = [][]string{{olderRelative, filepath.Join(root, filepath.FromSlash(olderRelative))}, authored}
			case "alias":
				alias := filepath.Join(root, "cache-alias.go")
				if err := os.Symlink(proof["cache-file"], alias); err != nil {
					t.Fatal(err)
				}
				rows = append(rows, []string{"cache-alias.go", alias})
			case "empty":
				rows = nil
			case "malformed":
			case "columns":
				rows = [][]string{{extractionAuthoredFixture}}
			case "relative":
				rows = [][]string{{extractionAuthoredFixture, extractionAuthoredFixture}}
			default:
				t.Fatal("unknown extraction fixture")
			}
			var data bytes.Buffer
			writer := csv.NewWriter(&data)
			if err := writer.WriteAll(rows); err != nil {
				t.Fatal(err)
			}
			if test.kind == "malformed" {
				data.Reset()
				data.WriteString("\"unfinished\n")
			}
			tool, environment := extractionCodeQLFixture(t, data.String())
			output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
			if test.refusal == "" {
				if code != 0 || !strings.Contains(output, "authored sentinel extracted") || !strings.Contains(output, "cache sources excluded") {
					t.Fatalf("actual database-row proof did not retain its positive control: exit %d\n%s", code, output)
				}
				return
			}
			if code == 0 || !strings.Contains(output, test.refusal) {
				t.Fatalf("database proof accepted or misreported %s; want %q, exit %d\n%s", test.name, test.refusal, code, output)
			}
		})
	}
}

// TestExtractionProofRejectsChangedSources refuses a changed source identity
// before asking CodeQL for a verdict about another set of input bytes.
func TestExtractionProofRejectsChangedSources(t *testing.T) {
	for _, field := range []string{"authored", "cache"} {
		t.Run(field, func(t *testing.T) {
			root, manifest, proof := extractionProofFixture(t)
			if err := os.WriteFile(proof[field+"-file"], []byte("package changed\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			tool, environment := extractionCodeQLFixture(t, "")
			calls := filepath.Join(t.TempDir(), "codeql-started")
			environment = append(environment, "CODEQL_PROOF_CALLS="+calls)
			output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
			if code == 0 || !strings.Contains(output, field+" sentinel changed") {
				t.Fatalf("changed %s source did not refuse: exit %d\n%s", field, code, output)
			}
			if _, err := os.Stat(calls); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("CodeQL ran before input identity was checked: %v", err)
			}
		})
	}
}

// TestExtractionProofRejectsForeignManifestPaths checks confinement before a
// manifest-controlled path is opened. The foreign files deliberately do not
// exist, so an attempted read produces the wrong refusal rather than proof.
func TestExtractionProofRejectsForeignManifestPaths(t *testing.T) {
	for _, test := range []struct{ field, refusal string }{
		{"source-root", "manifest belongs to another checkout"},
		{"authored-file", "manifest does not name the tracked authored sentinel"},
		{"cache-file", "manifest cache sentinel is outside gokrazy/modcache"},
	} {
		t.Run(test.field, func(t *testing.T) {
			root, manifest, proof := extractionProofFixture(t)
			proof[test.field] = filepath.Join(t.TempDir(), "foreign-source.go")
			data, err := json.Marshal(proof)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifest, data, 0o600); err != nil {
				t.Fatal(err)
			}
			tool, environment := extractionCodeQLFixture(t, "")
			output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
			if code == 0 || !strings.Contains(output, test.refusal) {
				t.Fatalf("manifest crossed its source boundary or read before checking it: exit %d\n%s", code, output)
			}
		})
	}
}

// TestExtractionProofNamesCodeQLFailure prevents a failed query or decoder
// from becoming an empty successful extraction verdict.
func TestExtractionProofNamesCodeQLFailure(t *testing.T) {
	for _, test := range []struct{ operation, refusal string }{
		{"query run", "CodeQL query failed"},
		{"bqrs decode", "CodeQL decode failed"},
	} {
		t.Run(test.operation, func(t *testing.T) {
			root, manifest, _ := extractionProofFixture(t)
			tool, environment := extractionCodeQLFixture(t, "")
			environment = append(environment, "CODEQL_PROOF_FAIL="+test.operation)
			output, code := extractionEntry(t, root, []string{"check", "file", manifest, "database", t.TempDir(), "executable", tool}, environment...)
			if code == 0 || !strings.Contains(output, test.refusal) || !strings.Contains(output, "fixture extraction failure") {
				t.Fatalf("foreign tool failure became success or lost its cause: exit %d\n%s", code, output)
			}
		})
	}
}

func extractionProofFixture(t *testing.T) (string, string, map[string]string) {
	t.Helper()
	root := fixtureRepository(t, map[string]string{
		extractionAuthoredFixture:                         "package codeqlsentinel\nfunc AuthoredSentinel() int { return 17 }\n",
		"gokrazy/modcache/extraction-fixture/sentinel.go": "package cachefixture\nfunc CacheSentinel() int { return 17 }\n",
	})
	proof := map[string]string{
		"source-root": root, "tags": "ze_setup",
		"authored-file": filepath.Join(root, extractionAuthoredFixture),
		"cache-file":    filepath.Join(root, "gokrazy/modcache/extraction-fixture/sentinel.go"),
		"cache-refusal": "CodeQL refuses gokrazy/modcache path",
	}
	for _, field := range []string{"authored", "cache"} {
		body, err := os.ReadFile(proof[field+"-file"])
		if err != nil {
			t.Fatal(err)
		}
		proof[field+"-sha256"] = extractionFixtureHash(body)
	}
	data, err := json.Marshal(proof)
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(t.TempDir(), "proof.json")
	if err := os.WriteFile(manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, manifest, proof
}

func extractionFixtureHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func extractionEntry(t *testing.T, root string, arguments []string, environment ...string) (string, int) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv := append([]string{"-test.run", "^TestFixtureCLI$", "--", "repo", "codeql"}, arguments...)
	command := exec.CommandContext(t.Context(), binary, argv...)
	command.Dir = root
	command.Env = append(fixtureEnvironment(t, root), environment...)
	output, err := command.CombinedOutput()
	if err == nil {
		return string(output), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return string(output), exit.ExitCode()
	}
	t.Fatalf("start native extraction entry: %v", err)
	return "", -1
}

func extractionCodeQLFixture(t *testing.T, rows string) (string, []string) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	data := filepath.Join(directory, "files.csv")
	if err := os.WriteFile(data, []byte(rows), 0o600); err != nil {
		t.Fatal(err)
	}
	fixtureWrite(t, directory, "codeql/placeholder", "owned CodeQL executable directory\n")
	fixtureWrite(t, directory, "codeql/qlpacks/codeql/go-all/fixture/qlpack.yml", "name: codeql/go-all\nversion: 0.0.0\nlibrary: true\n")
	fixtureWrite(t, directory, "codeql/qlpacks/codeql/go-all/fixture/go.qll", "// Synthetic bundled Go library; this process fixture does not compile QL.\n")
	tool := filepath.Join(directory, "codeql", "codeql")
	writeExtractionExecutable(t, tool, "#!/bin/sh\nexec \"$CODEQL_PROOF_TEST_BINARY\" -test.run '^TestExtractionCodeQLProcess$' -- \"$@\"\n")
	return tool, []string{"CODEQL_PROOF_TEST_BINARY=" + binary, "CODEQL_PROOF_CSV=" + data, "CODEQL_PROOF_CHILD=1"}
}

func writeExtractionExecutable(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
}

// TestExtractionCodeQLProcess checks query and decode invocation at the foreign
// executable boundary. Real CodeQL extraction remains a separate CI obligation.
func TestExtractionCodeQLProcess(t *testing.T) {
	if os.Getenv("CODEQL_PROOF_CHILD") != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("CodeQL fixture has no argument boundary")
	}
	arguments := os.Args[separator+1:]
	if path := os.Getenv("CODEQL_PROOF_CALLS"); path != "" {
		if err := os.WriteFile(path, []byte(strings.Join(arguments, "\n")), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output := ""
	for _, argument := range arguments {
		if value, found := strings.CutPrefix(argument, "--output="); found {
			output = value
		}
	}
	if output == "" || len(arguments) < 3 {
		t.Fatalf("CodeQL fixture received no explicit output: %q", arguments)
	}
	if os.Getenv("CODEQL_PROOF_FAIL") == arguments[0]+" "+arguments[1] {
		if _, err := os.Stderr.WriteString("fixture extraction failure\n"); err != nil {
			t.Fatal(err)
		}
		os.Exit(79)
	}
	switch arguments[0] + " " + arguments[1] {
	case "query run":
		query, err := os.ReadFile(arguments[len(arguments)-1])
		if err != nil {
			t.Fatal(err)
		}
		for _, expression := range []string{"from GoFile", ".getRelativePath()", ".getAbsolutePath()"} {
			if !strings.Contains(string(query), expression) {
				t.Fatalf("query does not ask for actual extracted GoFile paths: %s", query)
			}
		}
		if err := os.WriteFile(output, []byte("synthetic BQRS\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "bqrs decode":
		if !slices.Contains(arguments, "--format=csv") || !slices.Contains(arguments, "--no-titles") {
			t.Fatalf("CodeQL decode has no exact CSV contract: %q", arguments)
		}
		rows, err := os.ReadFile(os.Getenv("CODEQL_PROOF_CSV"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, rows, 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unexpected CodeQL operation: %q", arguments)
	}
	os.Exit(0)
}
