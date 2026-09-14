package fixture

import (
	"context"
	"fmt"
	"net"
	"os/exec"

	"github.com/ze-software/ze/internal/le/featuretags"
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

// uiLEFeatureTags answers the le personality's build tags for the checkout at
// root: ze_le, every gate the feature manifest declares, then the caller's
// extras. The gates come from featuretags, the one Go reader of the manifest.
// A fixture therefore cannot build a binary with a feature set the tooling
// never selects.
func uiLEFeatureTags(root string, extra ...string) ([]string, error) {
	gates, err := featuretags.DaemonTags(root)
	if err != nil {
		return nil, fmt.Errorf("read the feature manifest: %w", err)
	}

	tags := append([]string{buildTagLE}, gates...)
	seen := map[string]struct{}{buildTagLE: {}}
	for _, tag := range gates {
		seen[tag] = struct{}{}
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
