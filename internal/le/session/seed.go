// Design: docs/architecture/core-design.md -- isolated development session store seeding
// Related: summary.go -- the other session lifecycle action
package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/lejob"
)

const (
	seedWaitLimit = 300
	seedWaitStep  = time.Second
)

// streams are the terminal streams used by the session seeder.
type streams struct {
	Out io.Writer
	Err io.Writer
}

// seedReport records whether this call created or reused the session database.
type seedReport struct {
	Database  string `json:"database"`
	Password  string `json:"password-file"`
	User      string `json:"user"`
	Seeded    bool   `json:"seeded"`
	Existing  bool   `json:"existing"`
	Waited    bool   `json:"waited"`
	ChildCode int    `json:"child-code"`
}

type seedOps struct {
	environ []string
	random  io.Reader
	sleep   func(time.Duration)
	run     func([]string, lejob.ProcessIO) (int, error)
	waits   int
}

// seedStore seeds the store used by one session-local ze binary. The binary
// path MUST be relative to root and have the session bin-directory shape.
func seedStore(root, binary string, streams streams) (seedReport, int, error) {
	return seedStoreWithOps(root, binary, streams, seedOps{
		environ: os.Environ(),
		random:  rand.Reader,
		sleep:   time.Sleep,
		run:     lejob.RunProcess,
		waits:   seedWaitLimit,
	})
}

func seedStoreWithOps(root, binary string, streams streams, ops seedOps) (seedReport, int, error) {
	sessionDir, err := validateSessionBinary(binary)
	if err != nil {
		return seedReport{}, 1, err
	}
	etcRel := filepath.Join(sessionDir, "etc", "ze")
	databaseRel := filepath.Join(etcRel, "database.zefs")
	passwordRel := filepath.Join(etcRel, ".dev-password")
	report := seedReport{Database: filepath.ToSlash(databaseRel), Password: filepath.ToSlash(passwordRel), User: "admin"}
	database := filepath.Join(root, databaseRel)
	if pathExists(database) {
		report.Existing = true
		return report, 0, nil
	}

	binaryPath := filepath.Join(root, filepath.FromSlash(binary))
	info, err := os.Stat(binaryPath)
	if err != nil {
		return report, 1, fmt.Errorf("no binary at %s", binary)
	}
	if !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return report, 1, fmt.Errorf("no binary at %s", binary)
	}
	if name, found := configOverride(ops.environ); found {
		return report, 1, fmt.Errorf("%s overrides the config directory; unset it to seed %s", name, filepath.ToSlash(etcRel))
	}
	if err := os.MkdirAll(filepath.Join(root, etcRel), 0o700); err != nil {
		return report, 1, fmt.Errorf("cannot create %s: %w", filepath.ToSlash(etcRel), err)
	}

	lockRel := filepath.Join(etcRel, ".seed-lock")
	lock := filepath.Join(root, lockRel)
	if err := os.Mkdir(lock, 0o700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return report, 1, fmt.Errorf("cannot create seed lock %s: %w", filepath.ToSlash(lockRel), err)
		}
		report.Waited = true
		for range ops.waits {
			if pathExists(database) {
				report.Existing = true
				return report, 0, nil
			}
			ops.sleep(seedWaitStep)
		}
		if pathExists(database) {
			report.Existing = true
			return report, 0, nil
		}
		return report, 1, fmt.Errorf("another build holds %s and %s never appeared; remove the lock if no build is running", filepath.ToSlash(lockRel), filepath.ToSlash(databaseRel))
	}
	defer func() {
		_ = os.Remove(lock) //nolint:errcheck // a stale empty lock is reported by the next bounded waiter
	}()
	if pathExists(database) {
		report.Existing = true
		return report, 0, nil
	}

	passwordPath := filepath.Join(root, passwordRel)
	password, err := ensurePassword(passwordPath, ops.random)
	if err != nil {
		return report, 1, err
	}
	name := filepath.Base(sessionDir)
	stdin := strings.NewReader(strings.Join([]string{report.User, password, "127.0.0.1", "2222", name, ""}, "\n"))
	code, startErr := ops.run([]string{binary, "init", "--seed"}, lejob.ProcessIO{
		Dir: root, Environ: ops.environ, Stdin: stdin, Stdout: streams.Out, Stderr: streams.Err,
	})
	report.ChildCode = code
	if startErr != nil {
		return report, 1, fmt.Errorf("ze init failed for %s: %w", filepath.ToSlash(databaseRel), startErr)
	}
	if code != 0 {
		return report, 1, fmt.Errorf("ze init failed for %s", filepath.ToSlash(databaseRel))
	}
	if !pathExists(database) {
		return report, 1, fmt.Errorf("ze init reported success and %s does not exist", filepath.ToSlash(databaseRel))
	}
	report.Seeded = true
	if streams.Out != nil {
		fmt.Fprintf(streams.Out, "session store seeded: %s (user %s, password in %s)\n", report.Database, report.User, report.Password) //nolint:errcheck // CLI progress output
	}
	return report, 0, nil
}

func validateSessionBinary(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		return "", fmt.Errorf("refusing %s: not a binary in a session bin directory", binary)
	}
	parts := strings.Split(filepath.ToSlash(binary), "/")
	if len(parts) != 5 {
		if len(parts) > 5 {
			return "", fmt.Errorf("refusing %s: deeper than tmp/session/<dated-id>/bin/<name>", binary)
		}
		return "", fmt.Errorf("refusing %s: not a binary in a session bin directory", binary)
	}
	if parts[0] != "tmp" || parts[1] != "session" || parts[3] != "bin" || parts[4] == "" {
		return "", fmt.Errorf("refusing %s: not a binary in a session bin directory", binary)
	}
	matched, err := filepath.Match("????-??-??-*", parts[2])
	if err != nil || !matched {
		return "", fmt.Errorf("refusing %s: not a binary in a session bin directory", binary)
	}
	return filepath.Join(parts[0], parts[1], parts[2]), nil
}

func configOverride(environ []string) (string, bool) {
	normalizer := strings.NewReplacer(".", "_")
	for _, entry := range environ {
		name, _, found := strings.Cut(entry, "=")
		if !found {
			continue
		}
		normalized := normalizer.Replace(strings.ToLower(name))
		if normalized == "ze_config_dir" {
			return name, true
		}
	}
	return "", false
}

func ensurePassword(path string, random io.Reader) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("cannot read %s: %w", filepath.ToSlash(path), err)
	}
	if len(content) == 0 {
		bytes := make([]byte, 24)
		if _, err := io.ReadFull(random, bytes); err != nil {
			return "", fmt.Errorf("cannot read 24 random bytes: %w", err)
		}
		content = []byte(hex.EncodeToString(bytes) + "\n")
		if err := writeAtomic(path, content, 0o600, ".dev-password-*"); err != nil {
			return "", fmt.Errorf("cannot write %s: %w", filepath.ToSlash(path), err)
		}
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return "", fmt.Errorf("cannot restrict %s: %w", filepath.ToSlash(path), err)
	}
	password := strings.SplitN(string(content), "\n", 2)[0]
	if password == "" {
		return "", fmt.Errorf("%s holds no password", filepath.ToSlash(path))
	}
	return password, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
