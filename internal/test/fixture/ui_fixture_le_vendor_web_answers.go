package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const leVendorWebCorrupted = "internal/component/web/assets/htmx.min.js"

var leVendorWebSubtrees = []string{
	"third_party/web",
	"internal/chaos/web/assets",
	"internal/component/lg/assets",
	"internal/component/web/assets",
	"internal/component/api/rest/assets",
}

func init() {
	Register("ui/le-vendor-web-answers", uiDriver(leVendorWebAnswers))
}

type leVendorWebResult struct {
	code   int
	stdout string
	stderr string
}

func leVendorWebAnswers(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, 300*time.Second)
	defer cancel()

	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return errors.New("FAIL: ZE_REPO_ROOT is not set")
	}
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("FAIL: resolve ZE_REPO_ROOT: %w", err)
	}
	work, _, err := temporaryLEFixtureWorkspace("le-vendor-web-answers-")
	if err != nil {
		return fmt.Errorf("FAIL: create work directory: %w", err)
	}
	defer os.RemoveAll(work)
	binary, err := uiLEBinary(root)
	if err != nil {
		return fmt.Errorf("FAIL: %w", err)
	}

	checkoutBefore, err := leVendorWebDigestSubtrees(root, leVendorWebSubtrees)
	if err != nil {
		return fmt.Errorf("FAIL: snapshot checkout before commands: %w", err)
	}
	productEnv := leVendorWebWithEnv(os.Environ(), "ZE_REPO_ROOT", root)

	// The read-only gate runs over the real checkout and must give a successful,
	// human-readable verdict without writing diagnostics to stderr.
	checked, err := leVendorWebRun(ctx, work, productEnv, binary, "vendor-web", "check")
	if err != nil {
		return err
	}
	if checked.code != 0 {
		return fmt.Errorf("FAIL: `le vendor-web check` exited %d\nstdout:\n%sstderr:\n%s", checked.code, checked.stdout, checked.stderr)
	}
	if checked.stderr != "" {
		return fmt.Errorf("FAIL: `le vendor-web check` wrote stderr: %q", checked.stderr)
	}
	if !strings.Contains(checked.stdout, "consumer copies") {
		return fmt.Errorf("FAIL: `le vendor-web check` reported no verdict:\n%s", checked.stdout)
	}

	// The same answer must be available as data. Unmarshal the complete stdout,
	// so any non-data chatter before or after the payload is also a failure.
	answered, err := leVendorWebRun(ctx, work, productEnv, binary, "vendor-web", "check", "|", "json")
	if err != nil {
		return err
	}
	var report map[string]json.RawMessage
	if err := json.Unmarshal([]byte(answered.stdout), &report); err != nil {
		preview := answered.stdout
		if len(preview) > 400 {
			preview = preview[:400]
		}
		return fmt.Errorf("FAIL: `le vendor-web check | json` did not answer JSON: %v\n%s", err, preview)
	}
	for _, key := range []string{"problems", "compared", "skipped", "drift-checked"} {
		if _, ok := report[key]; !ok {
			keys := make([]string, 0, len(report))
			for present := range report {
				keys = append(keys, present)
			}
			sort.Strings(keys)
			return fmt.Errorf("FAIL: the report answered no %q key: %v", key, keys)
		}
	}
	var problems []json.RawMessage
	if err := json.Unmarshal(report["problems"], &problems); err != nil {
		return fmt.Errorf("FAIL: report problems are not an array: %w", err)
	}
	var compared int
	if err := json.Unmarshal(report["compared"], &compared); err != nil {
		return fmt.Errorf("FAIL: report compared value is not an integer: %w", err)
	}
	if compared <= 0 {
		return fmt.Errorf("FAIL: the report compared %d copies, so it proved nothing", compared)
	}
	if len(problems) != 0 {
		return fmt.Errorf("FAIL: the real checkout has %d vendored-web problems: %s", len(problems), report["problems"])
	}
	if answered.code != checked.code {
		return fmt.Errorf("FAIL: `le vendor-web check | json` exited %d and the bare command exited %d", answered.code, checked.code)
	}
	if answered.stderr != "" {
		return fmt.Errorf("FAIL: `le vendor-web check | json` wrote stderr: %q", answered.stderr)
	}
	counted, err := leVendorWebRun(ctx, work, productEnv, binary, "vendor-web", "check", "|", "count")
	if err != nil {
		return err
	}
	if counted.code != 1 {
		return fmt.Errorf("FAIL: `le vendor-web check | count` exited %d, want the document-shape refusal 1", counted.code)
	}
	if counted.stdout != "" {
		return fmt.Errorf("FAIL: refused `le vendor-web check | count` wrote stdout: %q", counted.stdout)
	}
	if !strings.Contains(counted.stderr, "count") || !strings.Contains(counted.stderr, "rows") {
		return fmt.Errorf("FAIL: count refusal did not identify the row-shape mismatch: %q", counted.stderr)
	}

	// Listing the area names all three actions and exposes their read/write
	// boundary at the point where a developer chooses one.
	listing, err := leVendorWebRun(ctx, work, productEnv, binary, "vendor-web")
	if err != nil {
		return err
	}
	if listing.code != 0 {
		return fmt.Errorf("FAIL: `le vendor-web` exited %d: %s", listing.code, listing.stderr)
	}
	if listing.stderr != "" {
		return fmt.Errorf("FAIL: `le vendor-web` wrote stderr: %q", listing.stderr)
	}
	for _, wanted := range []string{"check", "sync", "update-report", "writes", "checks"} {
		if !strings.Contains(listing.stdout, wanted) {
			return fmt.Errorf("FAIL: the listing does not name %q:\n%s", wanted, listing.stdout)
		}
	}
	foundSync := false
	for _, line := range strings.Split(listing.stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "sync ") {
			foundSync = true
			if !strings.Contains(line, "writes") {
				return fmt.Errorf("FAIL: the sync action is not marked as writing: %q", line)
			}
		}
	}
	if !foundSync {
		return fmt.Errorf("FAIL: the listing has no sync action line:\n%s", listing.stdout)
	}

	// Exercise the writing gate over two independent copies. Both start with
	// exactly one damaged consumer, and neither command is run against the
	// checkout.
	trees := make([]string, 0, 2)
	pristine := make([]map[string][]byte, 0, 2)
	for _, name := range []string{"first-tree", "second-tree"} {
		tree := filepath.Join(work, name)
		if err := os.MkdirAll(tree, 0o755); err != nil {
			return fmt.Errorf("FAIL: create %s: %w", name, err)
		}
		for _, subtree := range leVendorWebSubtrees {
			if err := leVendorWebCopyTree(filepath.Join(root, filepath.FromSlash(subtree)), filepath.Join(tree, filepath.FromSlash(subtree))); err != nil {
				return fmt.Errorf("FAIL: copy %s into %s: %w", subtree, name, err)
			}
		}
		beforeDamage, err := leVendorWebDigest(tree)
		if err != nil {
			return fmt.Errorf("FAIL: digest pristine %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(tree, filepath.FromSlash(leVendorWebCorrupted)), []byte("// an edited consumer copy\n"), 0o644); err != nil {
			return fmt.Errorf("FAIL: corrupt %s in %s: %w", leVendorWebCorrupted, name, err)
		}
		trees = append(trees, tree)
		pristine = append(pristine, beforeDamage)
	}

	synced := make([]leVendorWebResult, 0, len(trees))
	for _, tree := range trees {
		result, err := leVendorWebRun(ctx, work, leVendorWebWithEnv(os.Environ(), "ZE_REPO_ROOT", tree), binary, "vendor-web", "sync")
		if err != nil {
			return err
		}
		if result.code != 0 {
			return fmt.Errorf("FAIL: `le vendor-web sync` for %s exited %d\nstdout:\n%sstderr:\n%s", tree, result.code, result.stdout, result.stderr)
		}
		if result.stderr != "" {
			return fmt.Errorf("FAIL: `le vendor-web sync` for %s wrote stderr: %q", tree, result.stderr)
		}
		wantOutput := "synced: <root>/" + leVendorWebCorrupted + "\n"
		if got := leVendorWebNormalize(result.stdout, tree); got != wantOutput {
			return fmt.Errorf("FAIL: `le vendor-web sync` output differs\ngot:  %q\nwant: %q", got, wantOutput)
		}
		synced = append(synced, result)
	}
	if synced[0].code != synced[1].code {
		return fmt.Errorf("FAIL: the two sync runs exited %d and %d", synced[0].code, synced[1].code)
	}
	if leVendorWebNormalize(synced[0].stdout, trees[0]) != leVendorWebNormalize(synced[1].stdout, trees[1]) {
		return fmt.Errorf("FAIL: sync stdout differs between copied trees\nfirst:\n%ssecond:\n%s", synced[0].stdout, synced[1].stdout)
	}
	if leVendorWebNormalize(synced[0].stderr, trees[0]) != leVendorWebNormalize(synced[1].stderr, trees[1]) {
		return fmt.Errorf("FAIL: sync stderr differs between copied trees\nfirst:\n%ssecond:\n%s", synced[0].stderr, synced[1].stderr)
	}

	resulting := make([]map[string][]byte, 0, len(trees))
	for i, tree := range trees {
		files, err := leVendorWebDigest(tree)
		if err != nil {
			return fmt.Errorf("FAIL: digest resulting tree %s: %w", tree, err)
		}
		if differing := leVendorWebDiffering(pristine[i], files); len(differing) != 0 {
			return fmt.Errorf("FAIL: sync did not restore %s exactly; differing files: %v", tree, leVendorWebFirst(differing, 10))
		}
		resulting = append(resulting, files)
	}
	if differing := leVendorWebDiffering(resulting[0], resulting[1]); len(differing) != 0 {
		return fmt.Errorf("FAIL: the two runs left different trees behind: %v", leVendorWebFirst(differing, 10))
	}

	restored, err := os.ReadFile(filepath.Join(trees[1], filepath.FromSlash(leVendorWebCorrupted)))
	if err != nil {
		return fmt.Errorf("FAIL: read restored consumer: %w", err)
	}
	source, err := os.ReadFile(filepath.Join(root, "third_party", "web", "htmx", "htmx.min.js"))
	if err != nil {
		return fmt.Errorf("FAIL: read vendored source: %w", err)
	}
	if !bytes.Equal(restored, source) {
		return fmt.Errorf("FAIL: %s was not restored from its third_party/web source", leVendorWebCorrupted)
	}

	checkoutAfter, err := leVendorWebDigestSubtrees(root, leVendorWebSubtrees)
	if err != nil {
		return fmt.Errorf("FAIL: snapshot checkout after commands: %w", err)
	}
	if differing := leVendorWebDiffering(checkoutBefore, checkoutAfter); len(differing) != 0 {
		return fmt.Errorf("FAIL: vendored-web commands changed the checkout: %v", leVendorWebFirst(differing, 10))
	}

	fmt.Println("OK")
	return nil
}

func leVendorWebRun(ctx context.Context, dir string, env []string, program string, args ...string) (leVendorWebResult, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := leVendorWebResult{stdout: stdout.String(), stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.code = exitErr.ExitCode()
		return result, nil
	}
	return result, fmt.Errorf("FAIL: execute %s: %w", program, err)
}

func leVendorWebWithEnv(base []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(base)+1)
	for _, item := range base {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return append(out, prefix+value)
}

func leVendorWebNormalize(text, tree string) string {
	text = strings.ReplaceAll(text, tree, "<root>")
	return strings.ReplaceAll(text, "error: ", "")
}

func leVendorWebCopyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}
	if err := os.MkdirAll(dst, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		sourcePath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dst, entry.Name())
		entryInfo, err := os.Stat(sourcePath)
		if err != nil {
			return err
		}
		if entryInfo.IsDir() {
			if err := leVendorWebCopyTree(sourcePath, destPath); err != nil {
				return err
			}
			continue
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("unsupported file in vendored subtree: %s", sourcePath)
		}
		if err := leVendorWebCopyFile(sourcePath, destPath, entryInfo.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func leVendorWebCopyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(dst, mode)
}

func leVendorWebDigestSubtrees(root string, subtrees []string) (map[string][]byte, error) {
	all := make(map[string][]byte)
	for _, subtree := range subtrees {
		files, err := leVendorWebDigest(filepath.Join(root, filepath.FromSlash(subtree)))
		if err != nil {
			return nil, err
		}
		for name, data := range files {
			all[filepath.ToSlash(filepath.Join(subtree, name))] = data
		}
	}
	return all, nil
}

func leVendorWebDigest(root string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = data
		return nil
	})
	return files, err
}

func leVendorWebDiffering(left, right map[string][]byte) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	for name := range left {
		seen[name] = struct{}{}
	}
	for name := range right {
		seen[name] = struct{}{}
	}
	differing := make([]string, 0)
	for name := range seen {
		if !bytes.Equal(left[name], right[name]) {
			differing = append(differing, name)
		}
	}
	sort.Strings(differing)
	return differing
}

func leVendorWebFirst(items []string, limit int) []string {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}
