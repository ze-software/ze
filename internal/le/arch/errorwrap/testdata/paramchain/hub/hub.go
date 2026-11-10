// Package hub stands in for the daemon's reload wiring: the loader is a
// literal that one function builds and returns, handed down three calls as a
// parameter before the reload calls it, the shape diskConfigLoaders feeds
// runReloadContext.
package hub

import (
	"errors"
	"fmt"

	"example.com/fix/loader"
)

// loaders builds the two loaders the reload runs on.
func loaders(path string) (loadMap func() error, loadBoth func() (string, error)) {
	readAndParse := func() error {
		if path == "" {
			return errors.New("no file")
		}
		return loader.Parse()
	}
	loadBoth = func() (string, error) {
		err := readAndParse()
		if err != nil {
			return "", err
		}
		return path, nil
	}
	loadMap = func() error {
		_, err := loadBoth()
		return err
	}
	return loadMap, loadBoth
}

// Run wires the reload the way the daemon's main does.
func Run(path string) error {
	var loadBoth func() (string, error)
	if path != "" {
		_, loadBoth = loaders(path)
	}
	errs := make(chan error, 1)
	go handleSignal(loadBoth, errs)
	return <-errs
}

func handleSignal(load func() (string, error), errs chan<- error) {
	errs <- sighupReload(load)
}

func sighupReload(load func() (string, error)) error {
	if err := runCheck(load); err != nil {
		return err
	}
	return signalReload(load)
}

func signalReload(load func() (string, error)) error {
	return runReload(load)
}

// runReload adds "parse config", which the loader handed down already adds.
func runReload(load func() (string, error)) error {
	_, err := load()
	if err != nil {
		return fmt.Errorf("reload: parse config: %w", err)
	}
	return nil
}

// runCheck adds "check", which nothing below writes: no finding.
func runCheck(load func() (string, error)) error {
	_, err := load()
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	return nil
}
