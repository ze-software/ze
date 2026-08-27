// Design: docs/architecture/core-design.md -- the vendored-web drift gate, as a command
//
// check.go is the read-only half. It compares every consumer copy of a vendored
// web asset against its third_party/web/ source and answers what it found.
//
// The comparison needs NO network, which is what lets it gate a commit in an
// airgapped checkout. The registry query is a separate ACTION over the same
// code (`le vendor-web update-report`), and it is the only part that reaches
// out.

package vendorweb

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/lepath"
)

// registryTimeout bounds one npm registry query. The report is a developer
// waiting at a terminal, so a registry that has stopped answering must say so
// rather than hang.
const registryTimeout = 10 * time.Second

// registryClient is the HTTP client the registry query uses. It is a package
// variable so a test can install a transport and PROVE, rather than assume,
// that the drift comparison makes no request at all -- which is the property
// that lets the check gate a commit in an airgapped checkout.
var registryClient = &http.Client{Timeout: registryTimeout}

// semverRE matches the first semver triple (X.Y.Z) on a MANIFEST.md row.
var semverRE = regexp.MustCompile(`\d+\.\d+\.\d+`)

// extractVersionFromManifest scans the MANIFEST.md row that mentions the given
// file name and returns the first semver triple it finds.
func extractVersionFromManifest(manifest, fileName string) string {
	for line := range strings.SplitSeq(manifest, "\n") {
		if !strings.Contains(line, fileName) {
			continue
		}
		if version := semverRE.FindString(line); version != "" {
			return version
		}
	}
	return ""
}

// fetchLatestNpmVersion queries https://registry.npmjs.org/<pkg>/latest and
// returns the "version" field from the JSON response.
func fetchLatestNpmVersion(pkg string) (string, error) {
	var url textbuf.Buffer
	url.Str("https://registry.npmjs.org/").Str(pkg).Str("/latest")

	// The bound is on the request rather than on the client alone, so a
	// registry that accepts the connection and then stops sending cannot hold
	// a developer at a terminal.
	ctx, cancel := context.WithTimeout(context.Background(), registryTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), http.NoBody)
	if err != nil {
		return "", err
	}

	resp, err := registryClient.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close() //nolint:errcheck // a read-only response body
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("npm registry returned %s", resp.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	return body.Version, nil
}

// errorText is the text an error renders as in a report line, including the
// text a NIL error renders as. The fetch reports a failure when it answers no
// version, and it can answer no version with no error, so the two cases share
// one line and the nil spelling has to be preserved rather than hidden.
func errorText(err error) string {
	if err == nil {
		return "<nil>"
	}
	return err.Error()
}

// checkVersion asks the registry about one package and records the answer.
func checkVersion(pkg, current string) PackageVersion {
	if current == "" {
		return PackageVersion{Package: pkg}
	}
	latest, err := fetchLatestNpmVersion(pkg)
	if err != nil || latest == "" {
		return PackageVersion{Package: pkg, Current: current, Err: errorText(err)}
	}
	return PackageVersion{Package: pkg, Current: current, Latest: latest}
}

// checkUpdates reports newer releases of the vendored packages. This is the one
// part of this package that uses the network, and only the update-report
// command runs it.
func checkUpdates(root string) (UpdateReport, error) {
	manifestPath := filepath.Join(root, vendorDir, "MANIFEST.md")
	manifestBytes, err := os.ReadFile(manifestPath) //nolint:gosec // the path is under the checkout lepath.Root resolved
	if err != nil {
		return UpdateReport{}, fmt.Errorf("read %s: %w", manifestPath, err)
	}
	manifest := string(manifestBytes)

	// One row covers both htmx files. htmx 4 publishes its extensions inside
	// the core npm package, where htmx 2 published htmx-ext-sse beside it, so
	// hx-sse.min.js carries the version htmx.min.js does.
	report := UpdateReport{Packages: []PackageVersion{
		checkVersion("htmx.org", extractVersionFromManifest(manifest, "htmx.min.js")),
	}}

	return report, nil
}

// driftCheck compares each consumer copy of a vendored asset against its
// source, recording one Problem per problem and counting the copies it read.
//
// A consumer subscribes to a vendor directory by holding one file of it, and it
// then owes a matching copy of EVERY file of that directory. A file the sync
// never wrote is therefore a problem, not an absence nobody looks at.
func driftCheck(root string, report *CheckReport) error {
	pkgs, skipped, err := vendorPackages(root)
	report.Skipped = skipped
	if err != nil {
		return err
	}

	consumers, err := consumerDirs(root)
	if err != nil {
		return err
	}

	subscribers := map[string]int{}

	for _, consumer := range consumers {
		for _, pkg := range pkgs {
			if !subscribes(root, consumer, pkg) {
				continue
			}

			subscribers[pkg.dir]++

			for _, name := range pkg.files {
				source := filepath.Join(vendorDir, pkg.dir, name)
				copied := filepath.Join(consumer, name)

				sourceData, sourceErr := os.ReadFile(filepath.Join(root, source)) //nolint:gosec // the path comes from a walk of the checkout
				if sourceErr != nil {
					report.Problems = append(report.Problems, Problem{
						Kind: ProblemUnreadable, File: copied, Source: source, Detail: errorText(sourceErr),
					})
					continue
				}

				copiedData, copiedErr := os.ReadFile(filepath.Join(root, copied)) //nolint:gosec // the path comes from a walk of the checkout
				if copiedErr != nil {
					report.Problems = append(report.Problems, Problem{Kind: ProblemMissing, File: copied, Source: source})
					continue
				}

				report.Compared++

				if !bytes.Equal(sourceData, copiedData) {
					report.Problems = append(report.Problems, Problem{Kind: ProblemDrift, File: copied, Source: source})
				}
			}
		}
	}

	// A vendored package that reaches no consumer is the same defect seen from
	// the source side: the sync was never told to copy it. Without this, the
	// subscription rule would report nothing at all about a new directory.
	for _, pkg := range pkgs {
		if subscribers[pkg.dir] == 0 {
			report.Problems = append(report.Problems, Problem{
				Kind: ProblemUnsynced, File: filepath.Join(vendorDir, pkg.dir),
			})
		}
	}

	return nil
}

// Check compares every consumer copy against its third_party/web/ source and
// answers what it found. updates adds the npm registry query, which is the one
// part of this that uses the network.
//
// The report is answered whether or not the error is nil: a run that failed
// still read part of the tree, and what it read is what a reader needs in order
// to act on the failure.
func Check(root string, updates bool) (CheckReport, error) {
	report := CheckReport{Skipped: map[string]string{}, Problems: []Problem{}}

	if updates {
		update, err := checkUpdates(root)
		if err != nil {
			return report, err
		}
		report.Updates = &update
	}

	report.DriftChecked = true

	if err := driftCheck(root, &report); err != nil {
		return report, err
	}

	// FAIL CLOSED. A run that read nothing has proven nothing, so it must not
	// report what a run that read every copy reports.
	if report.Compared == 0 && len(report.Problems) == 0 {
		return report, fmt.Errorf("no consumer copy of a vendored asset was found under %s/, so this check proved nothing", consumerRoot)
	}

	if len(report.Problems) > 0 {
		return report, fmt.Errorf("%d consumer asset copy problem(s); run `make ze-vendor-web-sync` and commit the result", len(report.Problems))
	}

	return report, nil
}

// runCheck is the `check` and `update-report` actions. updates adds the npm
// registry query, which is the only part of either that uses the network.
func runCheck(updates bool) (any, int) {
	root, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 1
	}

	report, err := Check(root, updates)
	if err != nil {
		reportError(err)
		return report, 1
	}
	return report, 0
}
