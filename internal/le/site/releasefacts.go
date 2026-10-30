// Design: docs/architecture/site-facts.md -- release data is a network fact, committed by a command
// Related: facts.go -- the snapshot this section belongs to
//
// releasefacts.go derives the snapshot's release section from the committed
// website/data/github-releases.json, which `./le site release update` writes.
// The build reads that file and nothing else for release data, so it never
// reaches the network for it (AC-13 of spec-site-values-from-one-file).

package site

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	siterelease "github.com/ze-software/ze/internal/le/site/release"
)

// factsRelease is the newest published release, as the site shows it.
//
// The snapshot holds it as a pointer, null while the committed file states no
// release, so a page's marked fallback text stays (site.js leaves an element
// whose key holds no value, and so does the build: substituteHTML).
type factsRelease struct {
	// ISO is null while the newest release carries no .iso asset.
	ISO       *factsReleaseISO `json:"iso"`
	Published string           `json:"published"`
	URL       string           `json:"url"`
	Version   string           `json:"version"`
}

// factsReleaseISO is the installation image the newest release carries.
type factsReleaseISO struct {
	Bytes       int64  `json:"bytes"`
	DownloadURL string `json:"download_url"`
	Name        string `json:"name"`
	// SHA256 is null for an asset the forge computed no digest for.
	SHA256      *string `json:"sha256"`
	SizeDisplay string  `json:"size_display"`
}

// isoSuffix names the installation image among a release's assets: the
// appliance build names it the image stem plus .iso (resolveISOOutput,
// internal/appliance/cmd_iso.go).
const isoSuffix = ".iso"

// factsFromReleases reads the release section from the committed file, and
// says in _sources which release it published, or that there is none yet.
func factsFromReleases(source string, facts *siteFacts) error {
	file, err := siterelease.Read(source)
	if err != nil {
		return err
	}
	latest, published := file.Latest()
	if !published {
		facts.Sources["release"] = "none yet: " + siterelease.Path + " states no release (read from " + file.Endpoint + ")"
		return nil
	}
	publishedAt, err := time.Parse(time.RFC3339, latest.PublishedAt)
	if err != nil {
		return fmt.Errorf("%s: release %q published_at: %w", siterelease.Path, latest.TagName, err)
	}
	release := &factsRelease{
		Published: publishedAt.UTC().Format(time.DateOnly),
		URL:       latest.HTMLURL,
		Version:   latest.TagName,
	}
	facts.Sources["release"] = siterelease.Path + ": release " + latest.TagName + ", published " + latest.PublishedAt +
		" (read from " + file.Endpoint + ")"

	iso, carried, err := releaseISO(&latest)
	if err != nil {
		return err
	}
	if carried {
		release.ISO = &iso
	} else {
		facts.Sources["release.iso"] = "none yet: release " + latest.TagName + " carries no " + isoSuffix + " asset"
	}
	facts.Release = release
	return nil
}

// releaseISO answers the one .iso asset of a release, and false when it
// carries none. A release carrying two is refused: the lab page states one
// image, and picking one of two would publish a size nobody chose.
func releaseISO(latest *siterelease.Release) (factsReleaseISO, bool, error) {
	var found *siterelease.Asset
	for index := range latest.Assets {
		if !strings.HasSuffix(latest.Assets[index].Name, isoSuffix) {
			continue
		}
		if found != nil {
			return factsReleaseISO{}, false, fmt.Errorf("%s: release %q carries two %s assets, %q and %q, and the site shows one installation image; "+
				"say which in internal/le/site/releasefacts.go, releaseISO", siterelease.Path, latest.TagName, isoSuffix,
				found.Name, latest.Assets[index].Name)
		}
		found = &latest.Assets[index]
	}
	if found == nil {
		return factsReleaseISO{}, false, nil
	}
	iso := factsReleaseISO{
		Bytes:       found.Size,
		DownloadURL: found.BrowserDownloadURL,
		Name:        found.Name,
		SizeDisplay: megabytes(found.Size),
	}
	if sum, stated := found.SHA256(); stated {
		iso.SHA256 = &sum
	}
	return iso, true, nil
}

// megabytes answers a size in decimal megabytes with one decimal, the unit the
// lab page states ("under 150 MB"). The tenth rounds half up: 128,933,888
// bytes is 128.9 MB.
func megabytes(size int64) string {
	tenths := int((size + 50_000) / 100_000)
	return strconv.Itoa(tenths/10) + "." + strconv.Itoa(tenths%10) + " MB"
}
