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
	"strconv"
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

// semverRE matches the first semver version on a MANIFEST.md row, prerelease
// suffix included.
//
// The suffix is part of the version. Without it a vendored 4.0.0-beta6 reads as
// 4.0.0 and compares equal to the released 4.0.0. The report then prints "up to
// date" over the one upgrade it exists to announce.
var semverRE = regexp.MustCompile(`\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?`)

// extractVersionFromManifest scans the MANIFEST.md row that mentions the given
// file name and returns the first semver version it finds.
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

// releaseTriple reads the X.Y.Z of a version and says whether it read one.
func releaseTriple(version string) ([3]int, bool) {
	core, _, _ := strings.Cut(version, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return [3]int{}, false
	}
	var triple [3]int
	for index, part := range parts {
		number, err := strconv.Atoi(part)
		if err != nil {
			return [3]int{}, false
		}
		triple[index] = number
	}
	return triple, true
}

// isPrerelease says whether a version carries a prerelease suffix.
func isPrerelease(version string) bool {
	return strings.Contains(version, "-")
}

// laterRelease says whether release is a later version than version. The first
// argument MUST be a release: two prereleases of one triple have no order this
// function can read, so it refuses rather than guess.
//
// A prerelease sorts BEFORE the release that shares its triple, which is what
// makes 4.0.0 later than 4.0.0-beta6 and keeps 4.0.0 level with itself.
func laterRelease(release, version string) bool {
	if isPrerelease(release) {
		return false
	}
	later, ok := releaseTriple(release)
	if !ok {
		return false
	}
	earlier, ok := releaseTriple(version)
	if !ok {
		return false
	}
	for index := range later {
		if later[index] != earlier[index] {
			return later[index] > earlier[index]
		}
	}
	return isPrerelease(version)
}

// newestPublishedRelease asks the npm registry for a package's dist-tags and
// answers the newest RELEASE among them.
//
// Every tag is read rather than "latest" alone, because a release does not have
// to be tagged latest. htmx published 4.0.0 under "next" and left "latest" on
// 2.0.10. A query for "latest" then answers a version OLDER than the one the
// tree vendors, and the report reads as a recommendation to downgrade.
//
// A prerelease is skipped. The tree vendors one deliberately or not at all, and
// a maintainer parking an alpha on a tag is not an upgrade to offer.
func newestPublishedRelease(pkg string) (string, error) {
	var url textbuf.Buffer
	url.Str("https://registry.npmjs.org/-/package/").Str(pkg).Str("/dist-tags")

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
	var tags map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return "", err
	}

	newest := ""
	for _, version := range tags {
		if isPrerelease(version) {
			continue
		}
		if newest == "" || laterRelease(version, newest) {
			newest = version
		}
	}
	return newest, nil
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
	newest, err := newestPublishedRelease(pkg)
	if err != nil || newest == "" {
		return PackageVersion{Package: pkg, Current: current, Err: errorText(err)}
	}
	return PackageVersion{Package: pkg, Current: current, Latest: newest}
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
		return report, fmt.Errorf("%d consumer asset copy problem(s); run `./le vendor-web sync` and commit the result", len(report.Problems))
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
