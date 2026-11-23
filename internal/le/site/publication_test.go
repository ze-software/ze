// Design: docs/contributing/gh-pages.md -- publication binds checked bytes to their reviewed source.
package site

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestSitePublicationCommandsRegistered proves the public action table reaches
// receipt recording and preflight, rather than exposing test-only helpers.
func TestSitePublicationCommandsRegistered(t *testing.T) {
	var records bool
	var receipt bool
	var deployment bool
	for _, action := range Actions().Actions {
		if action.Verb == "record" {
			for _, parameter := range action.Parameters {
				if parameter.Keyword == "source" {
					records = true
				}
			}
		}
		if action.Verb == "check" {
			for _, parameter := range action.Parameters {
				if parameter.Keyword == "receipt" {
					receipt = true
				}
				if parameter.Keyword == "deployment" {
					deployment = true
				}
			}
		}
	}
	if !records {
		t.Error("site record source <revision> is not registered")
	}
	if !receipt {
		t.Error("site check receipt is not registered")
	}
	if !deployment {
		t.Error("site check deployment <revision> is not registered")
	}
}

// TestSitePublicationRecordAndPreflightReachRegisteredActions proves a full
// build reaches the recording action and the saved receipt reaches check.
func TestSitePublicationRecordAndPreflightReachRegisteredActions(t *testing.T) {
	paths, source := publicationFixture(t)
	payload, code := Answer([]string{"record", "source", source, "output", paths.Output})
	if code != 0 {
		t.Fatalf("record exited %d: %#v", code, payload)
	}
	report, ok := payload.(publicationReport)
	if !ok {
		t.Fatalf("record returned %T", payload)
	}
	if report.State != "recorded" {
		t.Fatalf("record state = %q", report.State)
	}
	if report.Files == 0 {
		t.Fatal("record named no artifact files")
	}
	if report.Source.Revision != source {
		t.Fatalf("record source = %q", report.Source.Revision)
	}
	if strings.HasPrefix(report.Receipt, paths.Output+string(filepath.Separator)) {
		t.Fatal("receipt was written into the served artifact")
	}
	if _, err := os.Stat(report.Receipt); err != nil {
		t.Fatal(err)
	}
	payload, code = Answer([]string{"check", "receipt", "output", paths.Output})
	if code != 0 {
		t.Fatalf("receipt check exited %d: %#v", code, payload)
	}
	checked, ok := payload.(checkReport)
	if !ok {
		t.Fatalf("check returned %T", payload)
	}
	if checked.Publication == nil {
		t.Fatal("registered check returned no publication evidence")
	}
	if checked.Publication.State != "checked" {
		t.Fatalf("preflight state = %q", checked.Publication.State)
	}
}

// TestSitePublicationRefusesChangedBytes checks the same public check that
// accepted the untouched artifact, then changes content without changing size.
func TestSitePublicationRefusesChangedBytes(t *testing.T) {
	paths, source := publicationFixture(t)
	publicationRecordFixture(t, paths, source)
	filename := filepath.Join(paths.Output, "index.html")
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, bytes.Repeat([]byte("x"), len(content)), 0o600); err != nil {
		t.Fatal(err)
	}
	if payload, code := Answer([]string{"check", "receipt", "output", paths.Output}); code != 1 || payload != nil {
		t.Fatalf("registered receipt check did not refuse changed bytes through its publication guard: code %d, payload %#v", code, payload)
	}
	if _, err := checkPublication(t.Context(), paths); err == nil {
		t.Fatal("preflight accepted changed artifact bytes")
	}
}

// TestSitePublicationRefusesMissingPartialAndRacedProvenance proves a receipt
// cannot attribute a mixed or legacy artifact to the latest source revision.
func TestSitePublicationRefusesMissingPartialAndRacedProvenance(t *testing.T) {
	for _, scenario := range []string{"legacy", "partial", "source-race"} {
		t.Run(scenario, func(t *testing.T) {
			paths, source := publicationFixture(t)
			publicationRecordFixture(t, paths, source)
			switch scenario {
			case "legacy":
				record, err := publicationProducerRecord(paths)
				if err != nil {
					t.Fatal(err)
				}
				if err := writeProducerRecord(paths, record.Claims); err != nil {
					t.Fatal(err)
				}
			case "partial":
				if _, err := buildProducers(producerBuildOptions{Repository: paths.Repository, Output: paths.Output, Producers: []string{"publication-fixture"}}); err != nil {
					t.Fatal(err)
				}
			case "source-race":
				record, err := publicationProducerRecord(paths)
				if err != nil {
					t.Fatal(err)
				}
				if record.Provenance == nil {
					t.Fatal("control has no full-build provenance")
				}
				before := record.Provenance.Source
				writeFixtureFile(t, filepath.Join(paths.Source, "CNAME"), "changed.example\n")
				if err := publicationBuildFinish(t.Context(), paths, before); err != nil {
					t.Fatal(err)
				}
			}
			if payload, code := Answer([]string{"record", "source", source, "output", paths.Output}); code != 1 || payload != nil {
				t.Fatalf("registered record action did not refuse %s provenance through its publication guard: code %d, payload %#v", scenario, code, payload)
			}
			if _, err := recordPublication(t.Context(), paths, source); err == nil {
				t.Fatal("record accepted unavailable whole-build provenance")
			}
		})
	}
}

// VALIDATES: a build during which other sessions changed the dirty set outside
// website/ stays recordable, and its receipt names every status entry observed
// at the start or at the end, so no working-tree input goes undisclosed.
// PREVENTS: one edit anywhere in the shared checkout making a whole build
// unrecordable although HEAD and the website bytes never moved.
func TestSitePublicationRecordsBuildWhoseDirtySetChanged(t *testing.T) {
	paths, source := publicationFixture(t)
	record, err := publicationProducerRecord(paths)
	if err != nil {
		t.Fatal(err)
	}
	early := filepath.Join(paths.Repository, "early-session-edit.txt")
	writeFixtureFile(t, early, "dirty when the build started\n")
	before, err := publicationBuildStart(t.Context(), paths, record.Provenance.Source.Digest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(early); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(paths.Repository, "other-session", "late-edit.txt"), "dirty when the build finished\n")
	if err := publicationBuildFinish(t.Context(), paths, before); err != nil {
		t.Fatal(err)
	}
	payload, code := Answer([]string{"record", "source", source, "output", paths.Output})
	if code != 0 {
		t.Fatalf("record refused a build whose HEAD and website bytes did not move: code %d, payload %#v", code, payload)
	}
	report, ok := payload.(publicationReport)
	if !ok {
		t.Fatalf("record returned %T", payload)
	}
	receipt, err := checkPublication(t.Context(), paths)
	if err != nil {
		t.Fatalf("receipt check refused the recorded build: %v", err)
	}
	for _, entry := range []string{"?? early-session-edit.txt", "?? other-session/late-edit.txt"} {
		if !slices.Contains(report.Source.DirtyPaths, entry) {
			t.Errorf("record answer does not disclose %q: %v", entry, report.Source.DirtyPaths)
		}
		if !slices.Contains(receipt.Source.DirtyPaths, entry) {
			t.Errorf("receipt does not disclose %q: %v", entry, receipt.Source.DirtyPaths)
		}
	}
	if !slices.IsSorted(receipt.Source.DirtyPaths) {
		t.Errorf("disclosed entries are not in canonical order: %v", receipt.Source.DirtyPaths)
	}
	if len(slices.Compact(slices.Clone(receipt.Source.DirtyPaths))) != len(receipt.Source.DirtyPaths) {
		t.Errorf("disclosed entries repeat: %v", receipt.Source.DirtyPaths)
	}
	if payload, code := Answer([]string{"check", "receipt", "output", paths.Output}); code != 0 {
		t.Fatalf("registered receipt check refused the recorded build: code %d, payload %#v", code, payload)
	}
}

// VALIDATES: a moved HEAD is named as the reason a build cannot be recorded,
// whatever else changed during the build, and a website byte change that
// happened in the same build is named beside it.
// PREVENTS: a later, lesser observation overwriting the HEAD reason.
func TestSitePublicationNamesEveryRacedSourceReason(t *testing.T) {
	const headReason = "source HEAD changed during the build"
	const bytesReason = "website source bytes changed during the build"
	for _, scenario := range []string{"head-and-dirty-set", "head-and-website-bytes"} {
		t.Run(scenario, func(t *testing.T) {
			paths, source := publicationFixture(t)
			record, err := publicationProducerRecord(paths)
			if err != nil {
				t.Fatal(err)
			}
			before := record.Provenance.Source
			writeFixtureFile(t, filepath.Join(paths.Repository, "other-session", "committed-during-build.txt"), "landed\n")
			roadmapGit(t, paths.Repository, "add", "other-session/committed-during-build.txt")
			roadmapGit(t, paths.Repository, "-c", "commit.gpgsign=false", "commit", "-qm", "Landed during the build")
			writeFixtureFile(t, filepath.Join(paths.Repository, "other-session", "late-edit.txt"), "dirty when the build finished\n")
			if scenario == "head-and-website-bytes" {
				writeFixtureFile(t, filepath.Join(paths.Source, "CNAME"), "changed.example\n")
			}
			if err := publicationBuildFinish(t.Context(), paths, before); err != nil {
				t.Fatal(err)
			}
			if payload, code := Answer([]string{"record", "source", source, "output", paths.Output}); code != 1 || payload != nil {
				t.Fatalf("registered record action accepted a build whose HEAD moved: code %d, payload %#v", code, payload)
			}
			_, err = recordPublication(t.Context(), paths, source)
			if err == nil {
				t.Fatal("record accepted a build whose HEAD moved")
			}
			if !strings.Contains(err.Error(), headReason) {
				t.Errorf("refusal does not name the moved HEAD: %v", err)
			}
			if strings.Contains(err.Error(), bytesReason) != (scenario == "head-and-website-bytes") {
				t.Errorf("refusal names the website bytes wrongly for %s: %v", scenario, err)
			}
			if strings.Contains(err.Error(), "dirty") {
				t.Errorf("refusal blames the dirty set, which is disclosure only: %v", err)
			}
		})
	}
}

// TestSitePublicationRetainsExistingArtifactChecks makes a source-only file
// visible. The registered record action must report that check and save none.
func TestSitePublicationRetainsExistingArtifactChecks(t *testing.T) {
	paths, source := publicationFixture(t)
	writeFixtureFile(t, filepath.Join(paths.Output, "AI.md"), "private source\n")
	payload, code := Answer([]string{"record", "source", source, "output", paths.Output})
	if code == 0 {
		t.Fatal("record accepted a source-only artifact")
	}
	checked, ok := payload.(checkReport)
	if !ok {
		t.Fatalf("record discarded existing check report: %T", payload)
	}
	if len(checked.SourceOnly) == 0 {
		t.Fatal("record lost the source-only finding")
	}
	if _, err := os.Stat(publicationReceiptPath(paths)); !os.IsNotExist(err) {
		t.Fatalf("failed check wrote receipt: %v", err)
	}
}

// TestSitePublicationSnapshotRejectsSymlinks guards the rooted acquisition
// boundary with a control file and a link to an outside file of equal bytes.
func TestSitePublicationSnapshotRejectsSymlinks(t *testing.T) {
	output := t.TempDir()
	writeFixtureFile(t, filepath.Join(output, "index.html"), "reviewed\n")
	if _, _, err := publicationSnapshot(output); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.html")
	writeFixtureFile(t, outside, "reviewed\n")
	if err := os.Symlink(outside, filepath.Join(output, "linked.html")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := publicationSnapshot(output); err == nil {
		t.Fatal("snapshot accepted an artifact symlink")
	}
}

// TestSitePublicationSnapshotBoundsDirectoryDepth reaches the real filesystem
// walker with a control file and then an excessive chain of empty directories.
func TestSitePublicationSnapshotBoundsDirectoryDepth(t *testing.T) {
	output := t.TempDir()
	writeFixtureFile(t, filepath.Join(output, "index.html"), "reviewed\n")
	if _, _, err := publicationSnapshot(output); err != nil {
		t.Fatal(err)
	}
	directory := output
	for range publicationDepthMax + 1 {
		directory = filepath.Join(directory, "nested")
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := publicationSnapshot(output); err == nil {
		t.Fatal("snapshot accepted excessive empty-directory depth")
	}
}

// TestSitePublicationPublicDotPathsAreProbed distinguishes public hidden paths
// from the exact GitHub control files that have no public byte representation.
func TestSitePublicationPublicDotPathsAreProbed(t *testing.T) {
	for _, name := range []string{".well-known/security.txt", ".verification.html", "assets/.proof.json"} {
		address, served, err := publicationAddress(name, strings.Repeat("a", 40))
		if err != nil {
			t.Fatal(err)
		}
		if !served {
			t.Errorf("public path %s was silently excluded", name)
		}
		if !strings.Contains(address, name) {
			t.Errorf("public path lost its identity: %s", address)
		}
	}
	for _, name := range []string{artifactIgnoreFile, ".nojekyll", "CNAME"} {
		_, served, err := publicationAddress(name, strings.Repeat("a", 40))
		if err != nil {
			t.Fatal(err)
		}
		if served {
			t.Errorf("GitHub control file %s was treated as publicly served", name)
		}
	}
}

// TestSitePublicationChecksReviewedBytesAndDeployment reaches the registered
// deployment action using fixed-origin API and live responses, then tests each
// near-miss at that same entry point. No fixture contacts GitHub or the site.
func TestSitePublicationChecksReviewedBytesAndDeployment(t *testing.T) {
	for _, scenario := range []string{"success", "wrong-deployment", "pending", "unknown-status", "wrong-live-bytes", "deleted-still-live", "redirect", "dirty-commit", "tracked-ignored"} {
		t.Run(scenario, func(t *testing.T) {
			paths, source := publicationFixture(t)
			publicationRecordFixture(t, paths, source)
			if scenario == "tracked-ignored" {
				writeFixtureFile(t, filepath.Join(paths.Output, ".DS_Store"), "not reviewed\n")
			}
			if scenario != "dirty-commit" {
				roadmapGit(t, paths.Output, "add", "-f", ".")
				roadmapGit(t, paths.Output, "-c", "commit.gpgsign=false", "commit", "-qm", "Reviewed artifact")
			}
			artifact := roadmapGit(t, paths.Output, "rev-parse", "HEAD")
			previous := publicationTransport
			t.Cleanup(func() { publicationTransport = previous })
			var live int
			publicationTransport = publicationRoundTrip(func(request *http.Request) (*http.Response, error) {
				if request.URL.Host == "api.github.com" {
					switch request.URL.Path {
					case "/repos/ze-software/ze/deployments":
						sha := artifact
						if scenario == "wrong-deployment" {
							sha = strings.Repeat("1", 40)
						}
						return publicationHTTP(t, http.StatusOK, []map[string]any{{"id": 1, "sha": sha, "ref": "gh-pages", "environment": "github-pages"}}), nil
					case "/repos/ze-software/ze/deployments/1/statuses":
						state := "success"
						if scenario == "pending" {
							state = "in_progress"
						}
						if scenario == "unknown-status" {
							state = "invented-status"
						}
						return publicationHTTP(t, http.StatusOK, []map[string]any{{"id": 2, "state": state, "environment_url": publicationSiteURL, "log_url": "https://github.com/ze-software/ze/actions/runs/7"}}), nil
					default:
						t.Errorf("unexpected API path %s", request.URL.Path)
						return publicationHTTP(t, http.StatusNotFound, map[string]any{}), nil
					}
				}
				if request.URL.Host != "ze-software.net" {
					t.Fatalf("probe escaped fixed origins: %s", request.URL)
				}
				live++
				if scenario == "redirect" {
					response := publicationHTTP(t, http.StatusFound, map[string]any{})
					response.Header.Set("Location", "https://other.example/")
					return response, nil
				}
				name := strings.TrimPrefix(request.URL.Path, "/")
				if strings.HasSuffix(request.URL.Path, "/") {
					name += "index.html"
				}
				content, err := os.ReadFile(filepath.Join(paths.Output, filepath.FromSlash(name)))
				if err != nil {
					if !os.IsNotExist(err) {
						t.Fatal(err)
					}
					if scenario == "deleted-still-live" {
						return publicationHTTPBytes(http.StatusOK, []byte("still here")), nil
					}
					return publicationHTTPBytes(http.StatusNotFound, nil), nil
				}
				if scenario == "wrong-live-bytes" {
					content = bytes.Repeat([]byte("x"), len(content))
				}
				return publicationHTTPBytes(http.StatusOK, content), nil
			})
			payload, code := Answer([]string{"check", "deployment", artifact, "output", paths.Output})
			if scenario != "success" {
				if code == 0 {
					t.Fatalf("deployment accepted %s: %#v", scenario, payload)
				}
				return
			}
			if code != 0 {
				t.Fatalf("deployment confirmation exited %d: %#v", code, payload)
			}
			checked, ok := payload.(checkReport)
			if !ok {
				t.Fatalf("confirmation returned %T", payload)
			}
			if checked.Publication == nil {
				t.Fatal("confirmation returned no publication result")
			}
			if checked.Publication.State != "confirmed" {
				t.Fatalf("confirmation state = %q", checked.Publication.State)
			}
			if checked.Publication.ArtifactRevision != artifact {
				t.Fatal("confirmation lost its exact commit")
			}
			if live == 0 {
				t.Fatal("confirmation reached no live-byte probe")
			}
			if checked.Publication.Probed != live {
				t.Fatalf("probed %d, transport observed %d", checked.Publication.Probed, live)
			}
		})
	}
}

// TestSitePublicationCommitUsesRawBlobs proves a clean index view cannot
// substitute for the bytes GitHub publishes from the selected commit.
func TestSitePublicationCommitUsesRawBlobs(t *testing.T) {
	for _, scenario := range []string{"assume-unchanged", "crlf-filter"} {
		t.Run(scenario, func(t *testing.T) {
			paths, source := publicationFixture(t)
			if scenario == "assume-unchanged" {
				roadmapGit(t, paths.Output, "add", "-f", ".")
				roadmapGit(t, paths.Output, "-c", "commit.gpgsign=false", "commit", "-qm", "Original generated artifact")
				roadmapGit(t, paths.Output, "update-index", "--assume-unchanged", "assets/site.css")
				writeSourceAsset(t, paths.Source, filepath.Join("css", "site.css"), "body { color: red; }\n")
			} else {
				writeFixtureFile(t, filepath.Join(paths.Output, ".git", "info", "attributes"), "assets/reviewed.txt text eol=crlf\n")
				writeSourceAsset(t, paths.Source, "reviewed.txt", "reviewed\r\n")
			}
			if _, err := Build(t.Context(), BuildOptions{Repository: paths.Repository, Output: paths.Output}); err != nil {
				t.Fatal(err)
			}
			receipt := publicationRecordFixture(t, paths, source)
			if scenario == "crlf-filter" {
				roadmapGit(t, paths.Output, "add", "-f", ".")
				roadmapGit(t, paths.Output, "-c", "commit.gpgsign=false", "commit", "-qm", "Filtered artifact")
			}
			if status := roadmapGit(t, paths.Output, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
				t.Fatalf("fixture is not clean under Git's index view: %s", status)
			}
			artifact := roadmapGit(t, paths.Output, "rev-parse", "HEAD")
			previous := publicationTransport
			t.Cleanup(func() { publicationTransport = previous })
			requests := 0
			publicationTransport = publicationRoundTrip(func(request *http.Request) (*http.Response, error) {
				requests++
				if request.URL.Path == "/repos/ze-software/ze/deployments" {
					return publicationHTTP(t, http.StatusOK, []map[string]any{{"id": 1, "sha": artifact, "ref": "gh-pages", "environment": "github-pages"}}), nil
				}
				if request.URL.Path == "/repos/ze-software/ze/deployments/1/statuses" {
					return publicationHTTP(t, http.StatusOK, []map[string]any{{"id": 2, "state": "success", "environment_url": publicationSiteURL, "log_url": "https://github.com/ze-software/ze/actions/runs/7"}}), nil
				}
				return publicationHTTPBytes(http.StatusNotFound, nil), nil
			})
			if payload, code := Answer([]string{"check", "deployment", artifact, "output", paths.Output}); code != 1 || payload != nil {
				t.Fatalf("registered deployment check did not refuse %s raw bytes through its publication guard: code %d, payload %#v", scenario, code, payload)
			}
			if requests != 0 {
				t.Errorf("raw commit refusal made %d network requests; expected rejection before API or live probes", requests)
			}
			if err := publicationCommit(t.Context(), paths, receipt, artifact); err == nil {
				t.Fatalf("accepted %s: raw committed bytes differ from the reviewed artifact", scenario)
			}
			if scenario == "assume-unchanged" {
				found := false
				for _, name := range receipt.Changed {
					if name == "assets/site.css" {
						found = true
					}
				}
				if !found {
					t.Error("receipt omitted the asset changed behind an index flag")
				}
			}
		})
	}
}

// TestSitePublicationSnapshotCanonicalOrder checks the flat manifest order
// independently of the directory walk's per-directory order.
func TestSitePublicationSnapshotCanonicalOrder(t *testing.T) {
	output := t.TempDir()
	writeFixtureFile(t, filepath.Join(output, "a", "file"), "nested\n")
	writeFixtureFile(t, filepath.Join(output, "a.txt"), "sibling\n")
	files, _, err := publicationSnapshot(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("snapshot files = %d, want 2", len(files))
	}
	if files[0].Path != "a.txt" {
		t.Errorf("first manifest path = %q, want a.txt", files[0].Path)
	}
	if files[1].Path != "a/file" {
		t.Errorf("second manifest path = %q, want a/file", files[1].Path)
	}
}

func publicationFixture(t *testing.T) (Paths, string) {
	t.Helper()
	stubLiveInputs(t, `[{"path":"show test","short-help":"Show rows","mode":"read-only"}]`)
	stubProducers(t, Producer{Name: "publication-fixture", Render: func(paths Paths) ([]string, error) {
		writeProducerPage(t, paths.Output, "")
		writeFixtureFile(t, filepath.Join(paths.Output, "index.md"), "# Reviewed page\n")
		writeArtifactFile(t, paths.Output, factsFile, plainFigureSnapshot)
		return []string{"/"}, nil
	}})
	previous := namedArtifacts
	t.Cleanup(func() { namedArtifacts = previous })
	namedArtifacts = []string{artifactIgnoreFile, "index.html", "index.md", factsFile}
	root, output := siteFixture(t)
	t.Setenv(lepath.RootKey, root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	// Match the real checkout's runtime exclusion before the first build.
	// Creating its producer record must not masquerade as a source-input race.
	writeFixtureFile(t, filepath.Join(root, ".git", "info", "exclude"), "/tmp/\n")
	writeArtifactFile(t, filepath.Join(root, "website"), plainFigureRatchetFile, noRatchetedPages)
	writeFixtureFile(t, filepath.Join(output, "index.html"), "previous page\n")
	writeFixtureFile(t, filepath.Join(output, "obsolete.html"), "old page\n")
	roadmapGit(t, output, "init")
	roadmapGit(t, output, "add", ".")
	roadmapGit(t, output, "-c", "commit.gpgsign=false", "commit", "-qm", "Previous artifact")
	if _, err := Build(t.Context(), BuildOptions{Repository: root, Output: output}); err != nil {
		t.Fatal(err)
	}
	paths, err := ResolvePaths(root, output)
	if err != nil {
		t.Fatal(err)
	}
	record, err := publicationProducerRecord(paths)
	if err != nil {
		t.Fatal(err)
	}
	if record.Provenance == nil {
		t.Fatal("the first fixture build did not record full-build provenance")
	}
	if record.Provenance.Unavailable != "" {
		t.Fatalf("the first fixture build lost provenance: %s; observed dirty paths: %v", record.Provenance.Unavailable, record.Provenance.Source.DirtyPaths)
	}
	return paths, roadmapGit(t, root, "rev-parse", "HEAD")
}

func publicationRecordFixture(t *testing.T, paths Paths, source string) publicationReceipt {
	t.Helper()
	payload, code := Answer([]string{"record", "source", source, "output", paths.Output})
	if code != 0 {
		t.Fatalf("record fixture exited %d: %#v", code, payload)
	}
	receipt, err := checkPublication(context.Background(), paths)
	if err != nil {
		t.Fatal(err)
	}
	return receipt
}

type publicationRoundTrip func(*http.Request) (*http.Response, error)

func (transport publicationRoundTrip) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func publicationHTTP(t *testing.T, status int, value any) *http.Response {
	t.Helper()
	content, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return publicationHTTPBytes(status, content)
}

func publicationHTTPBytes(status int, content []byte) *http.Response {
	return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(bytes.NewReader(content))}
}
