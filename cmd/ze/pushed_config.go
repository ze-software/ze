// Design: docs/architecture/appliance/device-config.md -- pushed config loading priority at boot

//go:build ze_core

package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/ze-software/ze/cmd/ze/hub"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/zefs"
)

// pushedConfigPath is a loose-file inbox written by an EXTERNAL actor
// (cloud-init / operator), not by ze, so it stays a raw file. Only ze's own
// output state (the active-config hash) is persisted in the shared zefs store.
const pushedConfigPath = "/perm/ze/config-pushed.conf"

var (
	readPushedConfig   = defaultReadPushedConfig
	removePushedConfig = defaultRemovePushedConfig
)

func defaultReadPushedConfig() ([]byte, error) {
	return os.ReadFile(pushedConfigPath) //nolint:gosec // appliance persistent path
}

func defaultRemovePushedConfig() error {
	return os.Remove(pushedConfigPath)
}

// judgeConfigBeforeBoot gives the verdict on candidate that can be given
// before the daemon boots, and says whether the rest of it is boot's.
//
// The full verdict (config.LoadConfig) is given here only when no external
// plugin is declared: not by candidate, not by plugins (the command line), and
// not by fallback, the config that boots when candidate is refused (nil when
// there is none). A config that declares an external plugin is parsed against
// that plugin's own schema, which the daemon holds only once the plugin has
// run and declared it, inside boot; and a full parse here builds values that
// keep the module set, after which boot could no longer publish a declared
// plugin's module (yang.KeepsModuleSet). So with an external plugin declared
// anywhere, this judges the plugin declarations alone (config.FirstPass) and
// answers judgedAtBoot=true: the caller MUST then set hub.ConfigRefused to
// undo what it does with candidate, because boot is what refuses it.
func judgeConfigBeforeBoot(candidate, fallback []byte, plugins []string) (judgedAtBoot bool, err error) {
	firstPass, err := config.FirstPass(string(candidate), "", plugins)
	if err != nil {
		return false, err
	}
	if len(firstPass.External) > 0 {
		return true, nil
	}
	if fallback != nil {
		if mayDeclareExternalPlugin(fallback, plugins) {
			return true, nil
		}
	}
	_, err = config.LoadConfig(string(candidate), "", nil)
	return false, err
}

// mayDeclareExternalPlugin reports whether text, with the command line's
// plugins, declares an external plugin. A text whose plugin declarations
// cannot be read answers true: it cannot be shown to declare none, and its
// own refusal is boot's to report if it ever boots.
func mayDeclareExternalPlugin(text []byte, plugins []string) bool {
	firstPass, err := config.FirstPass(string(text), "", plugins)
	if err != nil {
		return true
	}
	return len(firstPass.External) > 0
}

// revertPushedConfig undoes the application of a pushed config that boot
// refused: it puts back the config that was active before (or removes the
// active copy when there was none) and removes the pushed file, so the next
// start of ze runs the previous config. The daemon cannot fall back to it in
// this process: the plugins a config declares change only when ze restarts.
func revertPushedConfig(store storage.Storage, activeKey string, preChange []byte, refusal error) {
	var tb textbuf.Buffer
	tb.Str("warning: pushed config ").Str(pushedConfigPath).Str(" is invalid and was removed: ").Err(refusal).
		Str("\nze stops here because the plugins a config declares change only when ze restarts; ").
		Str("start ze again to run the previous config: ze start\n")
	tb.StdErr() //nolint:errcheck // stderr of a process that is exiting

	var restoreErr error
	if preChange == nil {
		restoreErr = store.Remove(activeKey)
	} else {
		restoreErr = store.WriteFile(activeKey, preChange, 0)
	}
	if restoreErr != nil {
		var failed textbuf.Buffer
		failed.Str("error: the previous config could not be put back, so the next start reads the refused config again: ").
			Err(restoreErr).Str("\nfix the config, then validate it: ze config validate\n")
		failed.StdErr() //nolint:errcheck // stderr of a process that is exiting
	}
	if rmErr := removePushedConfig(); rmErr != nil {
		var failed textbuf.Buffer
		failed.Str("warning: remove invalid pushed config: ").Err(rmErr).Byte('\n')
		failed.StdErr() //nolint:errcheck // stderr of a process that is exiting
	}
}

// checkPushedConfig applies a pushed config when it is valid, and removes it
// when it is not. plugins is the command line's plugin list, which declares
// plugins as the config does. When an external plugin is declared the full
// verdict is boot's (judgeConfigBeforeBoot), and a refusal there reverts the
// application through hub.ConfigRefused.
func checkPushedConfig(store storage.Storage, configName string, plugins []string) (applied bool, preChange []byte) {
	data, err := readPushedConfig()
	if err != nil {
		return false, nil
	}

	activeKey := zefs.KeyFileActive.Key(configName)
	// A read that fails answers nil: no active copy yet is the "none before"
	// this function answers, and no stored config means nothing boots in the
	// pushed config's place.
	preChange, _ = store.ReadFile(activeKey)
	fallback := preChange
	if fallback == nil {
		fallback, _ = store.ReadFile(configName)
	}

	judgedAtBoot, parseErr := judgeConfigBeforeBoot(data, fallback, plugins)
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "warning: pushed config invalid, ignoring: %v\n", parseErr)
		if rmErr := removePushedConfig(); rmErr != nil {
			fmt.Fprintf(os.Stderr, "warning: remove invalid pushed config: %v\n", rmErr)
		}
		return false, nil
	}

	if writeErr := store.WriteFile(activeKey, data, 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: apply pushed config: %v\n", writeErr)
		return false, nil
	}
	fmt.Fprintf(os.Stderr, "config: using pushed config from %s\n", pushedConfigPath)
	if judgedAtBoot {
		// Applied on the first pass's verdict alone: boot gives the rest of
		// it, and a refusal there undoes the application.
		hub.ConfigRefused = func(refusal error) { revertPushedConfig(store, activeKey, preChange, refusal) }
	}
	return true, preChange
}

func writeConfigActiveHash(store storage.Storage, configName string) {
	data, err := store.ReadFile(zefs.KeyFileActive.Key(configName))
	if err != nil {
		return
	}
	h := sha256.Sum256(data)
	hash := fmt.Sprintf("sha256:%x", h)
	if writeErr := store.WriteFile(zefs.KeyConfigActiveHash.Pattern, []byte(hash), 0); writeErr != nil {
		fmt.Fprintf(os.Stderr, "warning: write config-active-hash: %v\n", writeErr)
	}
}
