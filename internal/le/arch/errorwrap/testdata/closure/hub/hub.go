// Package hub reaches the loader through two local function literals, the
// shape diskConfigLoaders builds.
package hub

import (
	"fmt"

	"example.com/fix/loader"
)

// Reload adds "parse config", which loader.Parse adds two literals down.
func Reload() error {
	readAndParse := func() error {
		return loader.Parse()
	}
	load := func() error {
		return readAndParse()
	}
	if err := load(); err != nil {
		return fmt.Errorf("reload: parse config: %w", err)
	}
	return nil
}
