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

	"github.com/ze-software/ze/internal/le/job"
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
	run     func([]string, job.ProcessIO) (int, error)
	waits   int
}

// seedStore seeds the store used by one session-local ze binary. The binary
// path MUST be relative to root and have the session bin-directory shape.
func seedStore(root, binary string, streams streams) (seedReport, int, error) {
	return seedStoreWithOps(root, binary, streams, seedOps{
		environ: os.Environ(),
		random:  rand.Reader,
		sleep:   time.Sleep,
		run:     job.RunProcess,
		waits:   seedWaitLimit,
	})
}

// seedStoreWithOps seeds the session store in two ze runs. `ze init --seed`
// writes a blob artifact and skips interface discovery, so a session daemon
// never adopts the development host's interfaces. `ze init --from` then
// imports that artifact into the live `database` tree and retires it, which is
// the path an appliance takes at first boot. The blob is never the store: ze
// refuses to open a config directory that holds one, so a blob found here is
// reported with its repair and never counted as an existing store.
func seedStoreWithOps(root, binary string, streams streams, ops seedOps) (seedReport, int, error) {
	sessionDir, err := validateSessionBinary(binary)
	if err != nil {
		return seedReport{}, 1, err
	}
	etcRel := filepath.Join(sessionDir, "etc", "ze")
	store := sessionStore{
		root:    root,
		binary:  binary,
		treeRel: filepath.Join(etcRel, "database"),
		blobRel: filepath.Join(etcRel, "database.zefs"),
	}
	passwordRel := filepath.Join(etcRel, ".dev-password")
	report := seedReport{Database: filepath.ToSlash(store.treeRel), Password: filepath.ToSlash(passwordRel), User: "admin"}
	exists, err := store.exists()
	if err != nil {
		return report, 1, err
	}
	if exists {
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
		// The holder's own blob sits in the directory between its two ze
		// runs, so only the published tree with no blob ends the wait.
		for range ops.waits {
			if store.published() {
				report.Existing = true
				return report, 0, nil
			}
			ops.sleep(seedWaitStep)
		}
		exists, err := store.exists()
		if err != nil {
			return report, 1, err
		}
		if exists {
			report.Existing = true
			return report, 0, nil
		}
		return report, 1, fmt.Errorf("another build holds %s and %s never appeared; remove the lock if no build is running", filepath.ToSlash(lockRel), report.Database)
	}
	defer func() {
		_ = os.Remove(lock) //nolint:errcheck // a stale empty lock is reported by the next bounded waiter
	}()
	exists, err = store.exists()
	if err != nil {
		return report, 1, err
	}
	if exists {
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
	blob := filepath.ToSlash(store.blobRel)
	steps := []struct {
		argv    []string
		stdin   io.Reader
		madeRel string
	}{
		{argv: []string{binary, "init", "--seed"}, stdin: stdin, madeRel: store.blobRel},
		{argv: []string{binary, "init", "--from", blob}, madeRel: store.treeRel},
	}
	for _, step := range steps {
		command := strings.Join(step.argv[1:], " ")
		code, startErr := ops.run(step.argv, job.ProcessIO{
			Dir: root, Environ: ops.environ, Stdin: step.stdin, Stdout: streams.Out, Stderr: streams.Err,
		})
		report.ChildCode = code
		if startErr != nil {
			return report, 1, fmt.Errorf("ze %s failed for %s: %w", command, report.Database, startErr)
		}
		if code != 0 {
			return report, 1, fmt.Errorf("ze %s failed for %s", command, report.Database)
		}
		if !pathExists(filepath.Join(root, step.madeRel)) {
			return report, 1, fmt.Errorf("ze %s reported success and %s does not exist", command, filepath.ToSlash(step.madeRel))
		}
	}
	// The import retires the blob; one still in place would refuse every open.
	if _, err := store.exists(); err != nil {
		return report, 1, err
	}
	report.Seeded = true
	if streams.Out != nil {
		fmt.Fprintf(streams.Out, "session store seeded: %s (user %s, password in %s)\n", report.Database, report.User, report.Password) //nolint:errcheck // CLI progress output
	}
	return report, 0, nil
}

// sessionStore names the live tree and the blob artifact of one session's
// config directory, both relative to the checkout root.
type sessionStore struct {
	root    string
	binary  string
	treeRel string
	blobRel string
}

// exists reports whether the live tree is present. A blob in the config
// directory is an error whether or not the tree is present: ze refuses to open
// the store while it sits there, so neither state is a store a session daemon
// can start from.
func (s sessionStore) exists() (bool, error) {
	if pathExists(filepath.Join(s.root, s.blobRel)) {
		blob := filepath.ToSlash(s.blobRel)
		return false, fmt.Errorf("blob artifact %s is not a live store and ze refuses to open %s beside it; run %s init --from %s", blob, filepath.ToSlash(s.treeRel), s.binary, blob)
	}
	return pathExists(filepath.Join(s.root, s.treeRel)), nil
}

// published reports whether the live tree is present with no blob beside it.
func (s sessionStore) published() bool {
	return pathExists(filepath.Join(s.root, s.treeRel)) && !pathExists(filepath.Join(s.root, s.blobRel))
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
	content, err := os.ReadFile(path) //nolint:gosec // the path is a session state file under the checkout root
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
	password, _, _ := strings.Cut(string(content), "\n")
	if password == "" {
		return "", fmt.Errorf("%s holds no password", filepath.ToSlash(path))
	}
	return password, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
