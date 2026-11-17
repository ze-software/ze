// Design: docs/contributing/gh-pages.md -- stage shared topic sections inside the daemon configuration directory.
package siteterminaldemo

import (
	"errors"
	"os"
	"path/filepath"
)

// stageShowcaseTopicInputs copies only the authored configuration sections.
// The common lab owns these inputs inside its configuration directory; its
// next preparation removes them with the rest of that lab's previous state.
func stageShowcaseTopicInputs() error {
	sources, err := filepath.Glob(filepath.Join(demoTree(), "topics", "*", "*.conf"))
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return errors.New("the showcase has no authored topic configuration sections; restore demos/terminal/topics before preparing the lab")
	}
	sourceRoot, err := os.OpenRoot(demoTree())
	if err != nil {
		return err
	}
	defer sourceRoot.Close() //nolint:errcheck // Acquired assets own their read verdicts.
	if err := os.MkdirAll(scenarioConfigDir(showcaseLabID), 0o750); err != nil {
		return err
	}
	targetRoot, err := os.OpenRoot(scenarioConfigDir(showcaseLabID))
	if err != nil {
		return err
	}
	defer targetRoot.Close() //nolint:errcheck // Each atomic publication owns its write verdict.
	for _, source := range sources {
		relative, err := filepath.Rel(demoTree(), source)
		if err != nil {
			return err
		}
		asset, err := acquireAsset(sourceRoot, relative, nil)
		if err != nil {
			return err
		}
		if err := asset.Publish(targetRoot, relative); err != nil {
			return err
		}
	}
	return nil
}
