package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() {
	Register("runner/verify-scope-freshness-scoped", verifyScopeFreshnessDriver)
	Register("runner/verify-scope-selector", verifyScopeSelectorDriver)
}

func nativeLEBinary() (string, error) {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return "", errors.New("ZE_REPO_ROOT is not set")
	}
	path := filepath.Join(root, "bin", "le")
	info, err := os.Stat(path)
	if err != nil || info.Mode()&0o111 == 0 {
		return "", fmt.Errorf("native le binary is not executable: %s", path)
	}
	return path, nil
}

func verifyScopeFreshnessDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("freshness fixture takes no arguments")
	}
	repo, err := os.MkdirTemp("", "ze-verify-scope-freshness-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup
	if err := gitFixture(ctx, repo, map[string]string{fileGoMod: "module fixture/freshness\n\ngo 1.24\n", fileFeatureGates: contentFeatureGate, ".gitignore": "tmp/\n", "mine.txt": "mine\n", "theirs.txt": "theirs\n"}); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(repo, "mine.txt"), []byte("mine\nmy edit\n"), 0o600); err != nil {
		return err
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	env := append(os.Environ(), "ZE_REPO_ROOT="+repo)
	run := func(arguments ...string) (string, int, error) { return rawCommand(ctx, repo, env, le, arguments...) }
	if out, code, err := run("verify status", "write", "exit-code", "0", "mode", "full"); err != nil || code != 0 {
		return fmt.Errorf("write status exit=%d: %w %s", code, err, out)
	}
	fmt.Fprintln(os.Stdout, "scratch-repo-ready") //nolint:errcheck // progress output
	if err := os.WriteFile(filepath.Join(repo, "theirs.txt"), []byte("theirs\ntheir edit\n"), 0o600); err != nil {
		return err
	}
	if out, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code != 0 || !strings.Contains(out, "FRESH") {
		return fmt.Errorf("scoped mine check exit=%d: %w %s", code, err, out)
	}
	fmt.Fprintln(os.Stdout, "scoped-fresh-for-my-path") //nolint:errcheck // progress output
	if _, code, err := run("verify status", "check"); err != nil || code == 0 {
		return fmt.Errorf("unscoped check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "unscoped-still-whole-tree") //nolint:errcheck // progress output
	if _, code, err := run("verify status", "check", "path", "theirs.txt"); err != nil || code == 0 {
		return fmt.Errorf("theirs scoped check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "scoped-stale-for-their-path") //nolint:errcheck // progress output
	if err := os.WriteFile(filepath.Join(repo, "mine.txt"), []byte("mine\nmy second edit\n"), 0o600); err != nil {
		return err
	}
	if _, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code == 0 {
		return fmt.Errorf("own edit check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "scoped-stale-for-my-own-edit") //nolint:errcheck // progress output
	if err := os.WriteFile(filepath.Join(repo, "mine.txt"), []byte("mine\nmy edit\n"), 0o600); err != nil {
		return err
	}
	if _, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code != 0 {
		return fmt.Errorf("reverted edit check exit=%d: %w", code, err)
	}
	manifest := filepath.Join(repo, "tmp", "ze-verify-manifest.txt")
	body, err := os.ReadFile(manifest) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	lines := strings.Split(string(body), "\n")
	found := false
	for index, line := range lines {
		if strings.HasSuffix(line, " mine.txt") {
			lines[index] = "MOVED-DURING-RUN mine.txt"
			found = true
		}
	}
	if !found {
		return errors.New("mine.txt missing from verify manifest")
	}
	if err := os.WriteFile(manifest, []byte(strings.Join(lines, "\n")), 0o600); err != nil {
		return err
	}
	out, code, err := run("verify status", "check", "path", "mine.txt")
	if err != nil || code == 0 || !strings.Contains(out, "moved while the run was in flight") {
		return fmt.Errorf("moved check exit=%d: %w %s", code, err, out)
	}
	fmt.Fprintln(os.Stdout, "moved-path-stays-stale") //nolint:errcheck // progress output
	if out, code, err := run("verify status", "write", "exit-code", "1", "mode", "full"); err != nil || code != 0 {
		return fmt.Errorf("write red status exit=%d: %w %s", code, err, out)
	}
	if _, code, err := run("verify status", "check", "path", "mine.txt"); err != nil || code == 0 {
		return fmt.Errorf("red run scoped check exit=%d: %w", code, err)
	}
	fmt.Fprintln(os.Stdout, "red-run-stale-for-every-scope") //nolint:errcheck // progress output
	return nil
}

func verifyScopeSelectorDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("selector fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	root := os.Getenv("ZE_REPO_ROOT")
	work, err := os.MkdirTemp("", "ze-verify-scope-selector-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup
	env := os.Environ()
	run := func(path, printing string) (string, int, error) {
		input := filepath.Join(work, "scope.paths")
		if err := os.WriteFile(input, []byte(path+"\n"), 0o600); err != nil {
			return "", -1, err
		}
		stdout, _, code, err := rawCommandStreams(ctx, root, env, le, "changed", "scope", "print", printing, "paths-from", input)
		return stdout, code, err
	}
	out, code, err := run("internal/component/ssh/ssh.go", "both")
	if err != nil || code != 0 {
		return fmt.Errorf("SSH selector exit=%d: %w %s", code, err, out)
	}
	sections := strings.Split(out, "# tags\n")
	if len(sections) != 2 {
		return fmt.Errorf("bad selector output: %s", out)
	}
	packages := strings.TrimSpace(strings.TrimPrefix(sections[0], "# packages\n"))
	if packages != "./cmd/ze\n./cmd/ze/hub\n./internal/component/ssh" {
		return fmt.Errorf("SSH packages changed: %s", packages)
	}
	if strings.TrimSpace(sections[1]) != "ze_ssh" {
		return fmt.Errorf("SSH tags changed: %s", sections[1])
	}
	fmt.Fprintln(os.Stdout, "ssh-selects-its-gated-importer") //nolint:errcheck // progress output
	fmt.Fprintln(os.Stdout, "ssh-reaches-one-feature")        //nolint:errcheck // progress output
	out, code, err = run("demos/terminal/rpki/demo.cast", "packages")
	if err != nil || code != 0 {
		return fmt.Errorf("unclassified selector exit=%d: %w %s", code, err, out)
	}
	if strings.TrimSpace(out) == "./..." {
		return errors.New("unclassified path selected every package")
	}
	fmt.Fprintln(os.Stdout, "unclassified-path-is-named")               //nolint:errcheck // progress output
	fmt.Fprintln(os.Stdout, "unclassified-path-narrows-to-its-readers") //nolint:errcheck // progress output
	for _, path := range []string{fileGoMod, "go.sum", "vendor/example.com/dep/dep.go"} {
		out, code, err = run(path, "packages")
		if err != nil || code != 0 || strings.TrimSpace(out) != "./..." {
			return fmt.Errorf("dependency %s did not widen: exit=%d %w %s", path, code, err, out)
		}
	}
	fmt.Fprintln(os.Stdout, "dependency-move-widens-and-names-the-path") //nolint:errcheck // progress output
	return nil
}
