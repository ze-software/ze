package fixture

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
)

func init() {
	Register("ui/le-discovery-answers", uiDriver(leDiscoveryAnswers))
}

type uiLeDiscoveryAnswersCommandResult struct {
	stdout []byte
	stderr []byte
	code   int
	err    error
}

func temporaryLEFixtureWorkspace(prefix string) (string, string, error) {
	directory, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", "", err
	}
	return directory, filepath.Join(directory, "le"), nil
}

func leDiscoveryAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return uiLeDiscoveryAnswersFailf("ZE_REPO_ROOT is not set")
	}
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return uiLeDiscoveryAnswersFailf("resolving ZE_REPO_ROOT: %v", err)
	}

	here, _, err := temporaryLEFixtureWorkspace("le-discovery-answers-")
	if err != nil {
		return uiLeDiscoveryAnswersFailf("creating fixture directory: %v", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup

	binary, err := uiLEBinary(root)
	if err != nil {
		return uiLeDiscoveryAnswersFailf("%v", err)
	}

	runLE := func(tree string, args ...string) uiLeDiscoveryAnswersCommandResult {
		overrides := map[string]string{}
		if tree != "" {
			overrides["ZE_REPO_ROOT"] = tree
		}
		return uiLeDiscoveryAnswersRunCommand(ctx, here, overrides, binary, args...)
	}

	// Exercise each generator in an immutable export of HEAD. The tracked copy
	// may legitimately be stale while another change is in flight, so the
	// contract is that update repairs it, touches only its output, and is
	// byte-stable on the next update.
	for _, tc := range []struct {
		command string
		output  string
		unit    string
	}{
		{command: "discovery-index", output: "ai/PACKAGE-MAP.md", unit: fieldPackages},
		{command: "docs-to-code", output: "ai/DOCS-TO-CODE.md", unit: "design docs"},
	} {
		tree := filepath.Join(here, tc.command+"-command")
		if err := exportHEAD(ctx, root, tree); err != nil {
			return uiLeDiscoveryAnswersFailf("exporting HEAD into %s-command: %v", tc.command, err)
		}

		before, err := treeManifest(tree, tc.output)
		if err != nil {
			return uiLeDiscoveryAnswersFailf("recording %s export before generation: %v", tc.command, err)
		}
		wrote := runLE(tree, tc.command, "update")
		if wrote.code != 0 || wrote.err != nil {
			return uiLeDiscoveryAnswersFailf("le %s update exited %d: %s%s", tc.command, wrote.code, wrote.stdout, wrote.stderr)
		}
		outputPath := filepath.Join(tree, filepath.FromSlash(tc.output))
		generated, err := os.ReadFile(outputPath) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return uiLeDiscoveryAnswersFailf("reading generated %s: %v", tc.output, err)
		}
		if len(generated) <= 10000 {
			return uiLeDiscoveryAnswersFailf("%s is %d bytes, so the generation is vacuous", tc.output, len(generated))
		}
		if !bytes.Contains(wrote.stdout, []byte("("+tc.unit)) && !bytes.Contains(wrote.stdout, []byte(tc.unit)) {
			return uiLeDiscoveryAnswersFailf("le %s update did not say what it wrote: %s", tc.command, wrote.stdout)
		}
		after, err := treeManifest(tree, tc.output)
		if err != nil {
			return uiLeDiscoveryAnswersFailf("recording %s export after generation: %v", tc.command, err)
		}
		if before != after {
			return uiLeDiscoveryAnswersFailf("le %s update changed files other than %s", tc.command, tc.output)
		}

		checked := runLE(tree, tc.command, "check")
		if checked.code != 0 || checked.err != nil {
			return uiLeDiscoveryAnswersFailf("le %s check rejected the bytes update just wrote: %s%s", tc.command, checked.stdout, checked.stderr)
		}
		wroteAgain := runLE(tree, tc.command, "update")
		if wroteAgain.code != 0 || wroteAgain.err != nil {
			return uiLeDiscoveryAnswersFailf("second le %s update exited %d: %s%s", tc.command, wroteAgain.code, wroteAgain.stdout, wroteAgain.stderr)
		}
		repeated, err := os.ReadFile(outputPath) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return uiLeDiscoveryAnswersFailf("reading repeated %s generation: %v", tc.output, err)
		}
		if !bytes.Equal(generated, repeated) {
			return uiLeDiscoveryAnswersFailf("two le %s updates over one tree wrote different bytes", tc.command)
		}
	}

	// The router must discover exactly the working-tree paths reported by Git.
	// Running from the fixture directory also verifies that checkout discovery is
	// independent of the process working directory.
	changed, err := gitChangedFiles(ctx, root)
	if err != nil {
		return uiLeDiscoveryAnswersFailf("discovering changed files: %v", err)
	}
	plainWiring := runLE("", "doc wiring", "dry-run")
	if plainWiring.code != 0 || plainWiring.err != nil {
		return uiLeDiscoveryAnswersFailf("doc-wiring dry-run exited %d: %s%s", plainWiring.code, plainWiring.stdout, plainWiring.stderr)
	}
	if len(plainWiring.stderr) != 0 {
		return uiLeDiscoveryAnswersFailf("the dry run wrote to stderr: %s", plainWiring.stderr)
	}

	wiringJSON := runLE("", "doc wiring", "dry-run", "|", "json")
	if wiringJSON.code != 0 || wiringJSON.err != nil {
		return uiLeDiscoveryAnswersFailf("doc-wiring JSON dry-run exited %d: %s%s", wiringJSON.code, wiringJSON.stdout, wiringJSON.stderr)
	}
	var wiringFields map[string]json.RawMessage
	if err := json.Unmarshal(wiringJSON.stdout, &wiringFields); err != nil {
		return uiLeDiscoveryAnswersFailf("doc-wiring JSON is invalid: %v\n%s", err, wiringJSON.stdout)
	}
	for _, key := range []string{"actions", "advisory", "changed", fieldChecks, "declared-groups", "dry-run", fieldError, outcomeFailed, "failure-groups"} {
		if _, ok := wiringFields[key]; !ok {
			return uiLeDiscoveryAnswersFailf("the router answered no %q key: %v", key, uiLeDiscoveryAnswersSortedKeys(wiringFields))
		}
	}
	var routedChanged []string
	if err := json.Unmarshal(wiringFields["changed"], &routedChanged); err != nil {
		return uiLeDiscoveryAnswersFailf("the router's changed field is not a string list: %v", err)
	}
	if routedChanged == nil {
		routedChanged = []string{}
	}
	if changed == nil {
		changed = []string{}
	}
	if !reflect.DeepEqual(routedChanged, changed) {
		return uiLeDiscoveryAnswersFailf("doc-wiring discovered the wrong changed files\nGit: %q\nrouter: %q", changed, routedChanged)
	}

	// Supplying Git's paths explicitly must produce the same ordered, rendered
	// route as automatic checkout discovery.
	if len(changed) != 0 {
		args := []string{checkDocWiring, "dry-run"}
		for _, name := range changed {
			args = append(args, "changed-file", name)
		}
		explicit := runLE("", args...)
		if explicit.code != plainWiring.code || explicit.err != nil || !bytes.Equal(explicit.stdout, plainWiring.stdout) || !bytes.Equal(explicit.stderr, plainWiring.stderr) {
			return uiLeDiscoveryAnswersFailf("doc-wiring automatic and explicit routes differ\nautomatic (exit %d):\n%s%sexplicit (exit %d):\n%s%s", plainWiring.code, plainWiring.stdout, plainWiring.stderr, explicit.code, explicit.stdout, explicit.stderr)
		}
	}

	wiringYAML := runLE("", "doc wiring", "dry-run", "|", "yaml")
	if wiringYAML.code != 0 || wiringYAML.err != nil {
		return uiLeDiscoveryAnswersFailf("le doc wiring dry-run | yaml was refused: %s%s", wiringYAML.stdout, wiringYAML.stderr)
	}

	// Over the shared working tree either a current or stale verdict is valid,
	// but the gate/result boundary and output streams are fixed.
	verdict := runLE("", "discovery-index", "check")
	if verdict.code != 0 && verdict.code != 3 {
		return uiLeDiscoveryAnswersFailf("discovery-index check exited %d, want 0 or 3: %s%s", verdict.code, verdict.stdout, verdict.stderr)
	}
	if verdict.err != nil && verdict.code < 0 {
		return uiLeDiscoveryAnswersFailf("discovery-index check could not run: %v", verdict.err)
	}
	if len(verdict.stderr) != 0 {
		return uiLeDiscoveryAnswersFailf("discovery-index check wrote its verdict to stderr: %s", verdict.stderr)
	}
	if len(verdict.stdout) == 0 {
		return uiLeDiscoveryAnswersFailf("discovery-index check returned an empty verdict")
	}

	// One discovery payload supports all three row renderings.
	report := runLE("", "discovery-index", "check", "|", "json")
	if report.code != 0 && report.code != 3 {
		return uiLeDiscoveryAnswersFailf("discovery-index JSON check exited %d: %s%s", report.code, report.stdout, report.stderr)
	}
	if len(report.stderr) != 0 {
		return uiLeDiscoveryAnswersFailf("discovery-index JSON check wrote to stderr: %s", report.stderr)
	}
	var pageFields map[string]json.RawMessage
	if err := json.Unmarshal(report.stdout, &pageFields); err != nil {
		return uiLeDiscoveryAnswersFailf("discovery-index JSON is invalid: %v\n%s", err, report.stdout)
	}
	for _, key := range []string{fieldFile, fieldPackages, "todo", statusStale, fieldWritten} {
		if _, ok := pageFields[key]; !ok {
			return uiLeDiscoveryAnswersFailf("the gate answered no %q key: %v", key, uiLeDiscoveryAnswersSortedKeys(pageFields))
		}
	}
	var packages []struct {
		Path           string `json:"path"`
		Responsibility string `json:"responsibility"`
	}
	if err := json.Unmarshal(pageFields["packages"], &packages); err != nil {
		return uiLeDiscoveryAnswersFailf("the packages field is invalid: %v", err)
	}
	if len(packages) <= 100 {
		return uiLeDiscoveryAnswersFailf("the map describes %d packages", len(packages))
	}
	for _, row := range packages {
		if row.Path == "" || row.Responsibility == "" {
			return uiLeDiscoveryAnswersFailf("a package row is missing its path or its responsibility")
		}
	}
	var stale, written bool
	if err := json.Unmarshal(pageFields["stale"], &stale); err != nil {
		return uiLeDiscoveryAnswersFailf("the stale field is not boolean: %v", err)
	}
	if err := json.Unmarshal(pageFields["written"], &written); err != nil {
		return uiLeDiscoveryAnswersFailf("the written field is not boolean: %v", err)
	}
	if written {
		return uiLeDiscoveryAnswersFailf("check mode reported a write")
	}
	wantReportCode := 0
	if stale {
		wantReportCode = 3
	}
	if report.code != wantReportCode {
		return uiLeDiscoveryAnswersFailf("the JSON payload says stale=%v but the gate exited %d", stale, report.code)
	}

	counted := runLE("", "discovery-index", "check", "|", "count")
	if strings.TrimSpace(string(counted.stdout)) != fmt.Sprint(len(packages)) {
		return uiLeDiscoveryAnswersFailf("le discovery-index check | count answered %q for %d packages", counted.stdout, len(packages))
	}
	for _, operator := range []string{renderYAML, renderTable} {
		rendered := runLE("", "discovery-index", "check", "|", operator)
		if len(rendered.stderr) != 0 {
			return uiLeDiscoveryAnswersFailf("le discovery-index check | %s was refused: %s", operator, rendered.stderr)
		}
		if len(rendered.stdout) <= 1000 {
			return uiLeDiscoveryAnswersFailf("le discovery-index check | %s rendered %d bytes", operator, len(rendered.stdout))
		}
	}

	// Area listing and parser refusals retain their distinct exit boundaries.
	listing := runLE("", "discovery-index")
	if listing.code != 0 || listing.err != nil {
		return uiLeDiscoveryAnswersFailf("le discovery-index exited %d: %s%s", listing.code, listing.stdout, listing.stderr)
	}
	for _, word := range []string{actionCheck, actionUpdate, wordWrites, fieldChecks} {
		if !bytes.Contains(listing.stdout, []byte(word)) {
			return uiLeDiscoveryAnswersFailf("the listing does not carry %q:\n%s", word, listing.stdout)
		}
	}
	if got := runLE("", "discovery-index", "nonesuch").code; got != 2 {
		return uiLeDiscoveryAnswersFailf("an unknown discovery-index action answered %d, want 2", got)
	}
	if got := runLE("", "docs-to-code", "nonesuch").code; got != 2 {
		return uiLeDiscoveryAnswersFailf("an unknown docs-to-code action answered %d, want 2", got)
	}
	if got := runLE("", "doc wiring", "somefile.go").code; got != 1 {
		return uiLeDiscoveryAnswersFailf("a bare value was accepted with exit %d", got)
	}
	if got := runLE("", "doc wiring", "changed-file").code; got != 1 {
		return uiLeDiscoveryAnswersFailf("a keyword with nothing after it was accepted with exit %d", got)
	}

	// A controlled stale tree fixes the drift verdict at 3 and verifies repair.
	staleTree := filepath.Join(here, "stale")
	if err := os.RemoveAll(staleTree); err != nil {
		return uiLeDiscoveryAnswersFailf("clearing stale tree: %v", err)
	}
	files := map[string]string{
		fileGoMod:                      "module example.com/stale\n",
		fileFeatureGates:               "ze_core\n",
		"internal/core/thing/thing.go": "// Package thing does a thing.\npackage thing\n",
		"ai/PACKAGE-MAP.md":            "stale\n",
	}
	for name, body := range files {
		full := filepath.Join(staleTree, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			return uiLeDiscoveryAnswersFailf("creating %s: %v", name, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
			return uiLeDiscoveryAnswersFailf("writing %s: %v", name, err)
		}
	}

	drifted := runLE(staleTree, "discovery-index", "check")
	if drifted.code != 3 {
		return uiLeDiscoveryAnswersFailf("a stale index answered %d, want 3: %s%s", drifted.code, drifted.stdout, drifted.stderr)
	}
	updated := runLE(staleTree, "discovery-index", "update")
	if updated.code != 0 || updated.err != nil {
		return uiLeDiscoveryAnswersFailf("update did not repair the stale index: %s%s", updated.stdout, updated.stderr)
	}
	current := runLE(staleTree, "discovery-index", "check")
	if current.code != 0 || current.err != nil {
		return uiLeDiscoveryAnswersFailf("the index update just wrote still reads as stale: %s%s", current.stdout, current.stderr)
	}
	for _, operator := range []string{renderJSON, renderYAML, renderTable, pipeCount} {
		rendered := runLE(staleTree, "discovery-index", "check", "|", operator)
		if rendered.code != 0 || rendered.err != nil {
			return uiLeDiscoveryAnswersFailf("le discovery-index check | %s over a current tree exited %d: %s%s", operator, rendered.code, rendered.stdout, rendered.stderr)
		}
	}

	fmt.Println("PASS: le generates the discovery indexes byte for byte and routes the same gates")
	return nil
}

func uiLeDiscoveryAnswersRunCommand(ctx context.Context, cwd string, overrides map[string]string, name string, args ...string) uiLeDiscoveryAnswersCommandResult {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = cwd
	cmd.Env = uiLeDiscoveryAnswersMergedEnvironment(overrides)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return uiLeDiscoveryAnswersCommandResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), code: code, err: err}
}

func uiLeDiscoveryAnswersMergedEnvironment(overrides map[string]string) []string {
	if len(overrides) == 0 {
		return os.Environ()
	}
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, item := range os.Environ() {
		key := item
		if before, _, found := strings.Cut(item, "="); found {
			key = before
		}
		if _, replaced := overrides[key]; !replaced {
			env = append(env, item)
		}
	}
	keys := make([]string, 0, len(overrides))
	for key := range overrides {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		env = append(env, key+"="+overrides[key])
	}
	return env
}

func exportHEAD(ctx context.Context, repo, dest string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "git", "archive", "--format=tar", "HEAD")
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	unpackErr := extractTar(stdout, dest)
	if unpackErr != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if unpackErr != nil {
		return unpackErr
	}
	if waitErr != nil {
		return fmt.Errorf("git archive failed: %w: %s", waitErr, stderr.Bytes())
	}
	return nil
}

func extractTar(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := path.Clean(hdr.Name)
		if name == "." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
			return fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		full, err := containedPath(dest, filepath.FromSlash(name))
		if err != nil {
			return err
		}
		mode := fs.FileMode(hdr.Mode) & fs.ModePerm

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(full, mode); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				return err
			}
			f, err := os.OpenFile(full, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode) //nolint:gosec // the path is the fixture's own scratch file
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(f, tr) //nolint:gosec // the archive is the fixture's own git export
			closeErr := f.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if filepath.IsAbs(hdr.Linkname) {
				return fmt.Errorf("unsafe absolute symlink %q", hdr.Linkname)
			}
			if _, err := containedPath(dest, filepath.Join(filepath.Dir(name), filepath.FromSlash(hdr.Linkname))); err != nil {
				return fmt.Errorf("unsafe symlink %q: %w", hdr.Name, err)
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				return err
			}
			if err := os.Symlink(filepath.FromSlash(hdr.Linkname), full); err != nil {
				return err
			}
		case tar.TypeLink:
			target, err := containedPath(dest, filepath.FromSlash(path.Clean(hdr.Linkname)))
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
				return err
			}
			if err := os.Link(target, full); err != nil {
				return err
			}
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			continue
		default:
			return fmt.Errorf("unsupported archive entry %q of type %d", hdr.Name, hdr.Typeflag)
		}
	}
}

func containedPath(root, name string) (string, error) {
	full := filepath.Clean(filepath.Join(root, name))
	rel, err := filepath.Rel(root, full)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes %q", name, root)
	}
	return full, nil
}

func treeManifest(root, excluded string) ([32]byte, error) {
	h := sha256.New()
	excluded = filepath.ToSlash(filepath.Clean(filepath.FromSlash(excluded)))
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == excluded {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(h, "%s\x00%s\x00%o\x00", rel, info.Mode().Type(), info.Mode().Perm()); err != nil {
			return err
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(name)
			if err != nil {
				return err
			}
			_, err = io.WriteString(h, target)
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		f, err := os.Open(name) //nolint:gosec // the path is the fixture's own scratch file
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(h, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	})
	if err != nil {
		return [32]byte{}, err
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum, nil
}

func gitChangedFiles(ctx context.Context, root string) ([]string, error) {
	result := uiLeDiscoveryAnswersRunCommand(ctx, root, nil, "git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	if result.code != 0 || result.err != nil {
		return nil, fmt.Errorf("git status exited %d: %s%s", result.code, result.stdout, result.stderr)
	}

	parts := bytes.Split(result.stdout, []byte{0})
	files := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		record := parts[i]
		if len(record) == 0 {
			continue
		}
		if len(record) < 4 || record[2] != ' ' {
			return nil, fmt.Errorf("unrecognized git status record %q", record)
		}
		status := record[:2]
		files = append(files, filepath.ToSlash(string(record[3:])))
		if bytes.IndexByte(status, 'R') >= 0 || bytes.IndexByte(status, 'C') >= 0 {
			i++ // The NUL form carries the old path as the following field.
			if i >= len(parts) || len(parts[i]) == 0 {
				return nil, fmt.Errorf("rename status has no source path")
			}
		}
	}
	sort.Strings(files)
	unique := files[:0]
	for _, name := range files {
		if len(unique) == 0 || unique[len(unique)-1] != name {
			unique = append(unique, name)
		}
	}
	return unique, nil
}

func uiLeDiscoveryAnswersSortedKeys(values map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uiLeDiscoveryAnswersFailf(format string, args ...any) error {
	return errors.New("FAIL: " + fmt.Sprintf(format, args...))
}
