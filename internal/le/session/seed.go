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
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/sessiondir"
	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
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
	configRoot *os.Root
	executable *os.File
	environ    []string
	random     io.Reader
	sleep      func(time.Duration)
	run        func([]string, job.ProcessIO) (int, error)
	waits      int
}

// seedStore seeds the store used by one session-local ze binary. The binary
// path MUST be relative to root and have the session bin-directory shape.
func seedStore(root, binary string, streams streams) (seedReport, int, error) {
	return seedStoreAfterEntry(root, binary, streams, nil)
}

// seedStoreAfterEntry exposes the acquisition/use boundary to deterministic
// replacement tests. The callback belongs to this call, never to process state.
func seedStoreAfterEntry(root, binary string, streams streams, afterEntry func()) (seedReport, int, error) {
	if err := validateSeedEntry(root, binary, os.Environ()); err != nil {
		return seedReport{}, 1, err
	}
	return seedRetainedEntry(root, binary, streams, afterEntry, job.RunProcess)
}

// seedRetainedEntry is the seeder entry beneath its outer guard: it acquires
// the owned session's retained roots and seeds through them. It MUST confine
// every write to the caller's own session store on its own, because a proof
// that the outer guard works runs this entry without that guard.
func seedRetainedEntry(root, binary string, streams streams, afterEntry func(), run func([]string, job.ProcessIO) (int, error)) (seedReport, int, error) {
	resources, err := openSeedResources(root, binary)
	if err != nil {
		return seedReport{}, 1, err
	}
	defer resources.config.Close()     //nolint:errcheck // No directory writes remain at return.
	defer resources.executable.Close() //nolint:errcheck // Children have returned before close.
	if afterEntry != nil {
		afterEntry()
	}
	return seedStoreWithOps(root, binary, streams, seedOps{
		configRoot: resources.config,
		executable: resources.executable,
		environ:    os.Environ(),
		random:     rand.Reader,
		sleep:      time.Sleep,
		run:        run,
		waits:      seedWaitLimit,
	})
}

// validateSeedEntry checks the native entry before even an existing store can
// be reused. The binary-derived config root MUST belong to this session.
// The shared tmp/session prefix can be a managed scratch mount; beneath it,
// symlinks would let init or password handling reach a different store.
func validateSeedEntry(root, binary string, environ []string) error {
	dir, err := validateSessionBinary(binary)
	if err != nil {
		return err
	}
	if name, found := configOverride(environ); found {
		return fmt.Errorf("%s overrides the config directory; unset it to seed %s", name, dir)
	}
	base := filepath.Join(root, dir)
	for _, relative := range []string{".", sessiondir.IDFile, "bin", filepath.Join("bin", filepath.Base(binary)), "etc", filepath.Join("etc", "ze"), filepath.Join("etc", "ze", "database"), filepath.Join("etc", "ze", "database.zefs"), filepath.Join("etc", "ze", ".dev-password"), filepath.Join("etc", "ze", ".seed-lock")} {
		path := filepath.Join(base, relative)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect session seed path %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlink in session seed path %s; use this session's own binary and config directory", path)
		}
	}
	id, err := lepath.ResolveSessionID(root)
	if err != nil {
		return err
	}
	owner, err := sessiondir.Owner(base)
	if err != nil {
		return fmt.Errorf("read seed session owner %s: %w", base, err)
	}
	if owner != id {
		return fmt.Errorf("refusing to seed session %s owned by %s, caller %s; use the caller's session binary", dir, owner, id)
	}
	info, err := os.Stat(filepath.Join(root, binary))
	if err != nil {
		return fmt.Errorf("open session binary %s: %w", binary, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("session binary %s is not a regular file", binary)
	}
	if info.Mode()&0o111 == 0 {
		return fmt.Errorf("session binary %s is not executable", binary)
	}
	return nil
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
	configRoot := ops.configRoot
	if configRoot == nil {
		// Injected workflow callers own their fixture directory. Native callers
		// supply the root acquired with session ownership before the swap seam.
		if err := os.MkdirAll(filepath.Join(root, etcRel), 0o700); err != nil {
			return seedReport{}, 1, err
		}
		configRoot, err = os.OpenRoot(filepath.Join(root, etcRel))
		if err != nil {
			return seedReport{}, 1, err
		}
		defer configRoot.Close() //nolint:errcheck // No directory writes remain at return.
	}
	store := sessionStore{
		config:  configRoot,
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

	if ops.executable == nil {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(binary)))
		if err != nil {
			return report, 1, fmt.Errorf("no binary at %s", binary)
		}
		if !info.Mode().IsRegular() {
			return report, 1, fmt.Errorf("no binary at %s", binary)
		}
		if info.Mode()&0o111 == 0 {
			return report, 1, fmt.Errorf("no binary at %s", binary)
		}
	}
	if name, found := configOverride(ops.environ); found {
		return report, 1, fmt.Errorf("%s overrides the config directory; unset it to seed %s", name, filepath.ToSlash(etcRel))
	}
	lockRel := filepath.Join(etcRel, ".seed-lock")
	if err := configRoot.Mkdir(".seed-lock", 0o700); err != nil {
		if !errors.Is(err, os.ErrExist) {
			return report, 1, fmt.Errorf("cannot create seed lock %s: %w", filepath.ToSlash(lockRel), err)
		}
		lock, err := openSeedDirectory(configRoot, ".seed-lock", false)
		if err != nil {
			return report, 1, err
		}
		if err := lock.Close(); err != nil {
			return report, 1, err
		}
		report.Waited = true
		// The holder's own blob sits in the directory between its two ze
		// runs, so only the published tree with no blob ends the wait.
		for range ops.waits {
			published, err := store.published()
			if err != nil {
				return report, 1, err
			}
			if published {
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
		_ = configRoot.Remove(".seed-lock") //nolint:errcheck // A stale empty lock is reported by the next bounded waiter.
	}()
	exists, err = store.exists()
	if err != nil {
		return report, 1, err
	}
	if exists {
		report.Existing = true
		return report, 0, nil
	}

	password, err := ensurePassword(configRoot, ops.random)
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
		folder, err := configRoot.Open(".")
		if err != nil {
			return report, 1, err
		}
		code, startErr := ops.run(step.argv, job.ProcessIO{
			Executable:    ops.executable,
			SeedDirectory: folder,
			Dir:           root, Environ: ops.environ, Stdin: step.stdin, Stdout: streams.Out, Stderr: streams.Err,
		})
		startErr = errors.Join(startErr, folder.Close())
		report.ChildCode = code
		if startErr != nil {
			return report, 1, fmt.Errorf("ze %s failed for %s: %w", command, report.Database, startErr)
		}
		if code != 0 {
			return report, 1, fmt.Errorf("ze %s failed for %s", command, report.Database)
		}
		present, err := store.present(filepath.Base(step.madeRel))
		if err != nil {
			return report, 1, err
		}
		if !present {
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
// config directory. The names are for reporting; I/O uses the retained root.
type sessionStore struct {
	config  *os.Root
	binary  string
	treeRel string
	blobRel string
}

// exists reports whether the live tree is present. A blob in the config
// directory is an error whether or not the tree is present: ze refuses to open
// the store while it sits there, so neither state is a store a session daemon
// can start from.
func (s sessionStore) exists() (bool, error) {
	blobPresent, err := s.present("database.zefs")
	if err != nil {
		return false, err
	}
	if blobPresent {
		blob := filepath.ToSlash(s.blobRel)
		return false, fmt.Errorf("blob artifact %s is not a live store and ze refuses to open %s beside it; run %s init --from %s", blob, filepath.ToSlash(s.treeRel), s.binary, blob)
	}
	return s.present("database")
}

// published reports whether the live tree is present with no blob beside it.
func (s sessionStore) published() (bool, error) {
	blob, err := s.present("database.zefs")
	if err != nil {
		return false, err
	}
	if blob {
		return false, nil
	}
	return s.present("database")
}

// present observes one entry without following a link or reading its contents.
// This is the observation itself, not validation for a later pathname open.
func (s sessionStore) present(name string) (bool, error) {
	info, err := s.config.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("session seed entry %s is a symlink; use this session's own config directory", name)
	}
	if name == "database" {
		if !info.IsDir() {
			return false, fmt.Errorf("session database is not a directory; use this session's own config directory")
		}
	} else if !info.Mode().IsRegular() {
		return false, fmt.Errorf("session seed artifact %s is not a regular file; use this session's own config directory", name)
	}
	return true, nil
}

func validateSessionBinary(binary string) (string, error) {
	if filepath.IsAbs(binary) {
		return "", fmt.Errorf("refusing %s: not a binary in a session bin directory", binary)
	}
	parts := strings.Split(filepath.ToSlash(binary), "/")
	if len(parts) != 5 {
		if len(parts) > 5 {
			return "", fmt.Errorf("refusing %s: deeper than tmp/session/<YYYY-MM-DD>-<HHMMSS>/bin/<name>", binary)
		}
		return "", fmt.Errorf("refusing %s: not a binary in a session bin directory", binary)
	}
	if parts[0] != "tmp" || parts[1] != "session" || parts[3] != "bin" || parts[4] == "" {
		return "", fmt.Errorf("refusing %s: not a binary in a session bin directory", binary)
	}
	// Both directory shapes on disk pass: <date>-<HHMMSS> and the earlier
	// <date>-<session-id>.
	if !sessiondir.Dated(parts[2]) {
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

// ensurePassword reads and changes permissions through the same descriptor.
// The caller MUST keep root open until this function returns.
func ensurePassword(root *os.Root, random io.Reader) (string, error) {
	const name = ".dev-password"
	file, err := root.OpenFile(name, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if errors.Is(err, os.ErrNotExist) {
		return createSeedPassword(root, random)
	}
	if err != nil {
		return "", fmt.Errorf("cannot open session password: %w; use this session's own config directory", err)
	}
	defer file.Close() //nolint:errcheck // Writes are synced explicitly below.
	info, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("session password is not a regular file; use this session's own config directory")
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &stat); err != nil {
		return "", fmt.Errorf("cannot inspect session password ownership: %w", err)
	}
	if int(stat.Uid) != os.Geteuid() {
		return "", fmt.Errorf("session password belongs to user %d; run le session seed-store as that owner", stat.Uid)
	}
	if stat.Nlink != 1 {
		return "", fmt.Errorf("session password has %d links; remove the session .dev-password alias before running le session seed-store", stat.Nlink)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("cannot read session password: %w", err)
	}
	if len(content) == 0 {
		return createSeedPassword(root, random)
	}
	if err := file.Chmod(0o600); err != nil {
		return "", fmt.Errorf("cannot restrict session password: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	password, _, _ := strings.Cut(string(content), "\n")
	if password == "" {
		return "", fmt.Errorf("session .dev-password holds no password")
	}
	return password, nil
}

// createSeedPassword publishes a complete password through the retained root.
// The caller MUST keep root open until publication and cleanup finish.
func createSeedPassword(root *os.Root, random io.Reader) (string, error) {
	bytes := make([]byte, 24)
	if _, err := io.ReadFull(random, bytes); err != nil {
		return "", fmt.Errorf("cannot read 24 random bytes: %w", err)
	}
	password := hex.EncodeToString(bytes)
	stage := ".dev-password-" + rand.Text()
	file, err := root.OpenFile(stage, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("cannot stage session password: %w", err)
	}
	defer root.Remove(stage) //nolint:errcheck // Publication removes the private staging name.
	defer file.Close()       //nolint:errcheck // Publication checks file sync below.
	if _, err := file.WriteString(password + "\n"); err != nil {
		return "", err
	}
	if err := file.Chmod(0o600); err != nil {
		return "", err
	}
	if err := file.Sync(); err != nil {
		return "", err
	}
	if err := root.Rename(stage, ".dev-password"); err != nil {
		return "", fmt.Errorf("cannot publish session password: %w", err)
	}
	return password, nil
}
