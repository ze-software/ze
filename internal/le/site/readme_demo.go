// Design: website/AI.md -- the README animation derives from the website's verified recording.
// Related: terminaldemo/readme_svg.go renders the retained terminal events.
package site

import (
	"fmt"
	"os"

	siteterminaldemo "github.com/ze-software/ze/internal/le/site/terminaldemo"
)

const (
	readmeDemoProducerName = "readme-demo"
	readmeDemoID           = "cli-dashboard"
	readmeDemoFile         = "docs/demos/cli-dashboard.svg"
)

// The README is a source-checkout artifact rather than a public website route.
// Every full or partial build refreshes it from the same media the site serves.
func init() {
	registerDerivedProducer(Producer{
		Name: readmeDemoProducerName, Render: renderReadmeDemo, AlwaysOnPartial: true,
	})
}

func renderReadmeDemo(paths Paths) ([]string, error) {
	catalog := newDemoCatalog(paths)
	if err := catalog.load(); err != nil {
		return nil, err
	}
	entry, recorded := catalog.built[readmeDemoID]
	if !recorded {
		return nil, fmt.Errorf("README demo %s has no published recording; run ./le site terminal-demo render name %s", readmeDemoID, readmeDemoID)
	}
	media, err := catalog.openMedia()
	if err != nil {
		return nil, err
	}
	defer media.Close() //nolint:errcheck // The retained payload owns the acquisition verdict.
	cast, err := catalog.verifyAsset(media, readmeDemoID, demoCast, entry)
	if err != nil {
		return nil, err
	}
	animation, err := cast.ReadmeSVG()
	if err != nil {
		return nil, fmt.Errorf("render README demo %s: %w", readmeDemoID, err)
	}
	root, err := os.OpenRoot(paths.Repository)
	if err != nil {
		return nil, err
	}
	defer root.Close() //nolint:errcheck // Publication owns the write verdict.
	if err := publishReadmeDemo(root, animation); err != nil {
		return nil, fmt.Errorf("write README demo %s: %w", readmeDemoFile, err)
	}
	return nil, nil
}

// publishReadmeDemo keeps unchanged bytes untouched and replaces changed bytes
// atomically. The caller MUST serialize publication to this destination, as
// Asset.Publish requires. The open repository root confines every path lookup.
func publishReadmeDemo(root *os.Root, animation siteterminaldemo.Asset) error {
	info, err := root.Lstat(readmeDemoFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		return animation.Publish(root, readmeDemoFile)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("destination is not a regular file; move %s aside before rebuilding the website", readmeDemoFile)
	}
	current, err := root.ReadFile(readmeDemoFile)
	if err != nil {
		return err
	}
	content, err := animation.Text()
	if err != nil {
		return err
	}
	if string(current) == content {
		return nil
	}
	return animation.Publish(root, readmeDemoFile)
}
