package fixture

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func uiDriver(driver any) Driver {
	switch run := driver.(type) {
	case func(context.Context, []string) error:
		return run
	case func(context.Context) error:
		return func(ctx context.Context, _ []string) error { return run(ctx) }
	case func([]string) error:
		return func(_ context.Context, args []string) error { return run(args) }
	case func() error:
		return func(context.Context, []string) error { return run() }
	default:
		panic(fmt.Sprintf("unsupported UI fixture driver %T", driver))
	}
}

func registerFixture(name string, driver any) {
	Register(name, uiDriver(driver))
}

func uiLEBinary(root string) (string, error) {
	path := filepath.Join(root, "bin", "le")
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("locate native le binary: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return "", fmt.Errorf("native le binary is not executable: %s", path)
	}
	return path, nil
}

func uiLEFeatureTags(root string, extra ...string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(root, "feature-gates.txt")) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return nil, fmt.Errorf("read feature-gates.txt: %w", err)
	}
	tags := []string{buildTagLE}
	seen := map[string]struct{}{buildTagLE: {}}
	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if _, exists := seen[fields[0]]; !exists {
			seen[fields[0]] = struct{}{}
			tags = append(tags, fields[0])
		}
	}
	for _, tag := range extra {
		if _, exists := seen[tag]; !exists {
			seen[tag] = struct{}{}
			tags = append(tags, tag)
		}
	}
	return tags, nil
}

// uiZEBinary answers the ze binary THIS run built. The functional runner
// symlinks its own binaries under bare names into one directory, and prepends
// that directory to every child's PATH. So a PATH lookup names the subject
// under test and nothing else (Runner.setupBinShims and Runner.childPathEnv,
// internal/test/runner/runner.go).
//
// $ZE_REPO_ROOT/bin/ze, which this used to read, is not that binary. In a
// clean checkout it is not a binary at all, because .gitignore excludes bin/
// and no verification job writes ze there. The fixture therefore failed on the
// CI runner with "locate native ze binary: no such file or directory". It
// passed only on a machine whose developer had built one. That case is worse.
// The build there is whatever they compiled last, so the fixture reported on a
// binary the run never made.
func uiZEBinary() (string, error) {
	path, err := exec.LookPath("ze")
	if err != nil {
		return "", fmt.Errorf("locate the ze binary this run built, on PATH: %w", err)
	}
	return path, nil
}

func uiFreeTCPPort() (int, error) {
	// The listener closes on the next line, so there is nothing to cancel.
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("reserve ephemeral TCP port: %w", err)
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("a tcp listener answered %T, want *net.TCPAddr", listener.Addr())
	}
	port := address.Port
	if err := listener.Close(); err != nil {
		return 0, fmt.Errorf("release ephemeral TCP port: %w", err)
	}
	return port, nil
}
