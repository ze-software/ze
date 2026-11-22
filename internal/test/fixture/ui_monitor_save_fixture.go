// Design: docs/functional-tests.md -- waiting on atomic stream-save files in .ci tests
// Related: register_ui_monitor_save.go -- registration of these fixtures

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const saveFilePollDelay = 200 * time.Millisecond

const saveTempGlob = ".ze-save-*"

// uiAwaitSaveFile polls atomic `.ze-save-*` temporary files in the test working
// directory until one matches the given regular expression.
//
// Arguments: <pattern>.
func uiAwaitSaveFile(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("usage: ui/await-save-file <pattern>")
	}
	pattern := args[0]
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("ui/await-save-file: compile %q: %w", pattern, err)
	}

	waitCtx, cancel := context.WithTimeout(ctx, WaitBudget(50, 30*time.Second))
	defer cancel()

	attempts := WaitAttempts(50, saveFilePollDelay, 150)
	held := Poll(waitCtx, attempts, saveFilePollDelay, func() bool {
		matches, globErr := filepath.Glob(saveTempGlob)
		if globErr != nil {
			return false
		}
		for _, path := range matches {
			data, readErr := os.ReadFile(path) //nolint:gosec // cliio:allow fixture polls atomic .ze-save-* temp files in test workdir
			if readErr != nil {
				continue
			}
			if re.Match(data) {
				return true
			}
		}
		return false
	})
	if !held {
		return fmt.Errorf("ui/await-save-file: no %s file matched %q inside the wait budget", saveTempGlob, pattern)
	}
	fmt.Println("OK: save in progress holds " + pattern)
	return nil
}

// uiCheckCommittedSave verifies that an atomic stream save committed cleanly:
// <path> exists, no `.ze-save-*` temp file remains, <path> matches
// <must-match-regex>, and <path> does not contain <must-not-contain>.
//
// Arguments: <path> <must-match-regex> <must-not-contain>.
func uiCheckCommittedSave(_ context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: ui/check-committed-save <path> <must-match-regex> <must-not-contain>")
	}
	target, mustMatch, forbidden := args[0], args[1], args[2]
	re, err := regexp.Compile(mustMatch)
	if err != nil {
		return fmt.Errorf("ui/check-committed-save: compile %q: %w", mustMatch, err)
	}

	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("ui/check-committed-save: committed save %s is missing: %w", target, err)
	}
	if info.IsDir() {
		return fmt.Errorf("ui/check-committed-save: committed save %s is a directory", target)
	}

	leftovers, err := filepath.Glob(saveTempGlob)
	if err != nil {
		return fmt.Errorf("ui/check-committed-save: scan %s: %w", saveTempGlob, err)
	}
	if len(leftovers) > 0 {
		return fmt.Errorf("ui/check-committed-save: stopped stream left temporary files behind: %s", strings.Join(leftovers, ", "))
	}

	data, err := os.ReadFile(target) //nolint:gosec // cliio:allow fixture verifies committed save file in test workdir
	if err != nil {
		return fmt.Errorf("ui/check-committed-save: read %s: %w", target, err)
	}
	if !re.Match(data) {
		return fmt.Errorf("ui/check-committed-save: %s did not match %q", target, mustMatch)
	}
	if strings.Contains(string(data), forbidden) {
		return fmt.Errorf("ui/check-committed-save: %s contained forbidden text %q", target, forbidden)
	}
	fmt.Println("OK: committed save " + target + " verified")
	return nil
}
