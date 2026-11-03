// Design: docs/contributing/running-commands.md -- descriptor-bound session init.
// Package seedinit admits only the two session-seeding child invocations.
package seedinit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/env"
)

// DirectoryKey carries a durable label, never authority to open that path.
// Descriptor 4 is the authority; descriptor 3 belongs to the executable.
const DirectoryKey = "ze.session.seed.directory"

var _ = env.MustRegister(env.EnvEntry{Key: DirectoryKey, Type: "string", Description: "Internal session init directory label; requires inherited directory descriptor 4"})

var handler func(*os.Root, *os.File, bool) int

// Register installs the init command owner's in-process startup entry.
// Registration MUST happen before main; the callback borrows both handles.
func Register(fn func(*os.Root, *os.File, bool) int) {
	if handler != nil {
		panic("BUG: duplicate session init owner")
	}
	handler = fn
}

// Environment preserves caller controls, refusing conflicts rather than
// removing them. The returned label authorizes no pathname I/O.
func Environment(environ []string, label string) ([]string, error) {
	if err := controls(environ, true); err != nil {
		return nil, err
	}
	return append(append([]string(nil), environ...), DirectoryKey+"="+label), nil
}

func controls(environ []string, parent bool) error {
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToLower(strings.ReplaceAll(key, "_", "."))
		if parent && key == DirectoryKey {
			return fmt.Errorf("%s is already set; unset it before seeding", key)
		}
		for _, conflict := range []string{"ze.config.dir", "ze.crash.dir", "ze.storage.blob"} {
			if key == conflict {
				return fmt.Errorf("%s conflicts with descriptor-bound session init; unset it before seeding", key)
			}
		}
	}
	return nil
}

// Dispatch runs before ordinary crash initialization. An absent marker leaves
// ordinary startup alone; an invalid contract refuses before writes.
func Dispatch(args []string) (bool, int) {
	label := env.Get(DirectoryKey)
	if label == "" {
		for _, entry := range os.Environ() {
			key, _, _ := strings.Cut(entry, "=")
			if strings.EqualFold(strings.ReplaceAll(key, "_", "."), DirectoryKey) {
				fmt.Fprintln(os.Stderr, "session init: empty directory label; launch through le session seed-store") //nolint:errcheck // Startup refusal.
				return true, 1
			}
		}
		return false, 0 // No inherited session invocation was requested.
	}
	code, err := dispatch(args, label)
	if err != nil {
		fmt.Fprintln(os.Stderr, "session init:", err) //nolint:errcheck // Startup refusal.
		return true, 1
	}
	return true, code
}

func dispatch(args []string, label string) (int, error) {
	if !filepath.IsAbs(label) {
		return 1, fmt.Errorf("session directory label is not absolute; launch through le session seed-store")
	}
	if filepath.Clean(label) != label {
		return 1, fmt.Errorf("session directory label is not canonical; launch through le session seed-store")
	}
	expected, err := filepath.Abs(filepath.Join(filepath.Dir(os.Args[0]), "..", "etc", "ze"))
	if err != nil {
		return 1, err
	}
	if label != expected {
		return 1, fmt.Errorf("session directory label differs from the launched binary's config label; launch through le session seed-store")
	}
	if len(args) < 2 {
		return 1, fmt.Errorf("inherited session directory requires init; launch through le session seed-store")
	}
	if args[0] != "init" {
		return 1, fmt.Errorf("inherited session directory cannot run %s; launch through le session seed-store", args[0])
	}
	importing := false
	if len(args) == 2 && args[1] == "--seed" {
		// The first child creates only the canonical seed.
	} else if len(args) == 3 && args[1] == "--from" {
		from, err := filepath.Abs(args[2])
		if err != nil {
			return 1, err
		}
		if from != filepath.Join(label, "database.zefs") {
			return 1, fmt.Errorf("inherited init source is outside the accepted directory; launch through le session seed-store")
		}
		importing = true
	} else {
		return 1, fmt.Errorf("inherited session directory accepts only seed creation or its import; launch through le session seed-store")
	}
	if err := controls(os.Environ(), false); err != nil {
		return 1, err
	}
	if handler == nil {
		return 1, fmt.Errorf("this binary has no init owner; build a Ze binary with init support")
	}
	root, folder, err := inheritedDirectory(label)
	if err != nil {
		return 1, err
	}
	defer root.Close()   //nolint:errcheck // Command directory operations have finished.
	defer folder.Close() //nolint:errcheck // The child owns its inherited descriptor.
	capture, err := crashlog.InitSeed(root)
	if err != nil {
		return 1, err
	}
	code := handler(root, folder, importing)
	// Not deferred: a panic keeps capture armed for main's recovery and the
	// runtime. Process exit releases the handles on that path.
	if err := capture.Close(); err != nil {
		return 1, err
	}
	return code, nil
}
