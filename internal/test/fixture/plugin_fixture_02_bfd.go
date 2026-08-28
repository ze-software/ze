package fixture

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const bfdRunBudget02 = 12 * time.Second
const bfdShutdownBudget02 = 5 * time.Second

func init() {
	Register("plugin/bfd-auth-meticulous-persist", bfdAuthMeticulousPersist02)
	Register("plugin/bfd-auth-mismatch", bfdAuthMismatch02)
}

type syncBuffer02 struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *syncBuffer02) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *syncBuffer02) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

type bfdDaemon02 struct {
	cmd     *exec.Cmd
	log     syncBuffer02
	started time.Time
	done    chan error
}

func bfdConfig02(stateDir string) string {
	return fmt.Sprintf(`environment {
}

bfd {
	enabled true;
	persist-dir %q;
	profile fast-meticulous {
		detect-multiplier 3
		desired-min-tx-us 50000
		required-min-rx-us 50000
		auth {
			type meticulous-keyed-sha1
			key-id 1
			secret "k-persist"
		}
	}
	single-hop-session 203.0.113.9 {
		profile fast-meticulous
	}
}
`, stateDir)
}

func startBFDDaemon02(ctx context.Context, stateDir string) (*bfdDaemon02, error) {
	d := &bfdDaemon02{started: time.Now(), done: make(chan error, 1)}
	d.cmd = exec.CommandContext(ctx, "ze", "-")
	d.cmd.Stdin = strings.NewReader(bfdConfig02(stateDir))
	d.cmd.Stdout = &d.log
	d.cmd.Stderr = &d.log
	d.cmd.Env = append(os.Environ(), "ze.log.bfd=debug", "ze.bfd.test-parallel=true", "ze.config.dir="+stateDir)
	if err := d.cmd.Start(); err != nil {
		return nil, err
	}
	go func() { d.done <- d.cmd.Wait() }()
	return d, nil
}

func (d *bfdDaemon02) waitFor02(ctx context.Context, what string, interval time.Duration, predicate func() bool) error {
	deadline := d.started.Add(bfdRunBudget02)
	for time.Now().Before(deadline) {
		if predicate() {
			return nil
		}
		select {
		case err := <-d.done:
			d.done <- err
			if err != nil {
				return fmt.Errorf("ze exited (%w) before %s\n%s", err, what, d.log.String())
			}
			return fmt.Errorf("ze exited (<nil>) before %s\n%s", what, d.log.String())
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
	_ = d.abandon02()
	return fmt.Errorf("gave up after %s waiting for %s\n%s", bfdRunBudget02, what, d.log.String())
}

func (d *bfdDaemon02) stop02() (string, error) {
	if err := d.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		return d.log.String(), err
	}
	select {
	case err := <-d.done:
		captured := d.log.String()
		if captured != "" {
			fmt.Fprint(os.Stderr, captured)
		}
		if err != nil {
			return captured, fmt.Errorf("ze exited after SIGTERM: %w", err)
		}
		return captured, nil
	case <-time.After(bfdShutdownBudget02):
		captured := d.log.String()
		_ = d.abandon02()
		return captured, fmt.Errorf("ze did not exit within %s of SIGTERM\n%s", bfdShutdownBudget02, captured)
	}
}

func (d *bfdDaemon02) abandon02() error {
	if d.cmd.ProcessState != nil && d.cmd.ProcessState.Exited() {
		return nil
	}
	if err := d.cmd.Process.Kill(); err != nil {
		return err
	}
	select {
	case <-d.done:
	case <-time.After(2 * time.Second):
	}
	return nil
}

type storeProbe02 struct {
	store string
	key   string
}

func commandOutput02(deadline time.Time, args ...string) ([]byte, error) {
	left := time.Until(deadline)
	if left <= 0 {
		return nil, context.DeadlineExceeded
	}
	if left > 5*time.Second {
		left = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), left)
	defer cancel()
	return exec.CommandContext(ctx, args[0], args[1:]...).Output()
}

func (p *storeProbe02) seq02(deadline time.Time) int {
	if _, err := os.Stat(p.store); err != nil {
		return 0
	}
	if p.key == "" {
		listing, err := commandOutput02(deadline, "ze", "data", "--path", p.store, "ls")
		if err != nil {
			return 0
		}
		for _, key := range strings.Fields(string(listing)) {
			if strings.HasPrefix(key, "meta/bfd/auth/") {
				p.key = key
				break
			}
		}
		if p.key == "" {
			return 0
		}
	}
	got, err := commandOutput02(deadline, "ze", "data", "--path", p.store, "cat", p.key)
	if err != nil {
		return 0
	}
	seq, _ := strconv.Atoi(strings.TrimSpace(string(got)))
	return seq
}

var restoredSequence02 = regexp.MustCompile(`bfd auth sequence restored.*?seq=(\d+)`)

func restoredSeq02(log string) int {
	match := restoredSequence02.FindStringSubmatch(log)
	if len(match) != 2 {
		return 0
	}
	seq, _ := strconv.Atoi(match[1])
	return seq
}

func bfdAuthMeticulousPersist02(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("bfd persistence fixture takes no arguments")
	}
	stateDir, err := os.MkdirTemp(".", "ze-bfd-auth-persist-")
	if err != nil {
		return err
	}
	stateDir, err = filepath.Abs(stateDir)
	if err != nil {
		return err
	}
	probe := &storeProbe02{store: filepath.Join(stateDir, "database.zefs")}
	first, err := startBFDDaemon02(ctx, stateDir)
	if err != nil {
		return err
	}
	if err := first.waitFor02(ctx, "'bfd plugin running'", 50*time.Millisecond, func() bool {
		return strings.Contains(first.log.String(), "bfd plugin running")
	}); err != nil {
		return fmt.Errorf("first run did not reach 'bfd plugin running': %w", err)
	}
	if err := first.waitFor02(ctx, "a sequence reached database.zefs", 250*time.Millisecond, func() bool {
		return probe.seq02(first.started.Add(bfdRunBudget02)) > 0
	}); err != nil {
		return fmt.Errorf("first run never persisted a sequence: %w", err)
	}
	captured1, err := first.stop02()
	if err != nil {
		return err
	}
	if restoredSeq02(captured1) != 0 {
		return fmt.Errorf("first (fresh) run restored a sequence unexpectedly")
	}

	second, err := startBFDDaemon02(ctx, stateDir)
	if err != nil {
		return err
	}
	if err := second.waitFor02(ctx, "'bfd plugin running'", 50*time.Millisecond, func() bool {
		return strings.Contains(second.log.String(), "bfd plugin running")
	}); err != nil {
		return fmt.Errorf("second run did not reach 'bfd plugin running': %w", err)
	}
	if err := second.waitFor02(ctx, "the restored-sequence log line", 50*time.Millisecond, func() bool {
		return restoredSeq02(second.log.String()) > 0
	}); err != nil {
		return fmt.Errorf("second run did not restore a persisted sequence from database.zefs: %w", err)
	}
	captured2, err := second.stop02()
	if err != nil {
		return err
	}
	seq := restoredSeq02(captured2)
	if seq <= 0 {
		return fmt.Errorf("restored sequence vanished from the second run log")
	}
	fmt.Fprintf(os.Stderr, "OK: TX sequence persisted and restored across restart (seq=%d)\n", seq)
	return nil
}

func bfdAuthMismatch02(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("bfd auth mismatch fixture takes no arguments")
	}
	config := `environment {
}

bfd {
	enabled true;
	profile insecure {
		auth {
			type simple-password
			key-id 1
			secret "plaintext"
		}
	}
}
`
	cmd := exec.CommandContext(ctx, "ze", "-")
	cmd.Stdin = strings.NewReader(config)
	var stdout, stderr syncBuffer02
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(), "ze.log.bfd=debug")
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	deadline := time.Now().Add(8 * time.Second)
	for !strings.Contains(stderr.String(), "simple-password rejected") && time.Now().Before(deadline) {
		select {
		case <-done:
			deadline = time.Now()
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
			}
		}
	}
	captured := stderr.String()
	if captured != "" {
		fmt.Fprint(os.Stderr, captured)
	}
	if !strings.Contains(captured, "simple-password rejected") {
		return fmt.Errorf("missing required pattern: simple-password rejected")
	}
	fmt.Fprintln(os.Stderr, "OK: simple-password rejection observed")
	return nil
}
