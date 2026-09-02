package fixture

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

func init() {
	Register("ui/le-ste-answers", uiDriver(leSTEAnswers))
}

type leSTEResult struct {
	stdout []byte
	stderr []byte
	code   int
	err    error
}

type leSTEReview struct {
	DocumentsReviewed int                       `json:"documents-reviewed"`
	Totals            map[string]int            `json:"totals"`
	Counts            map[string]map[string]int `json:"counts"`
	Findings          []json.RawMessage         `json:"findings"`
}

func leSTEAnswers(ctx context.Context) error {
	root := os.Getenv("ZE_REPO_ROOT")
	if root == "" {
		return leSTEFailf("ZE_REPO_ROOT is not set")
	}
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return leSTEFailf("resolving ZE_REPO_ROOT: %v", err)
	}
	here, _, err := temporaryLEFixtureWorkspace("le-ste-answers-")
	if err != nil {
		return leSTEFailf("creating the fixture directory: %v", err)
	}
	defer os.RemoveAll(here) //nolint:errcheck // fixture cleanup
	export := filepath.Join(here, "export")
	if err := os.RemoveAll(export); err != nil {
		return leSTEFailf("removing an old export: %v", err)
	}
	if err := os.MkdirAll(export, 0o750); err != nil {
		return leSTEFailf("creating the export directory: %v", err)
	}

	binary, err := uiLEBinary(root)
	if err != nil {
		return leSTEFailf("%v", err)
	}

	if err := leSTEExport(ctx, root, export); err != nil {
		return err
	}

	gitSteps := [][]string{
		{argInit, "--quiet"},
		{argAdd, "--all"},
		{"-c", "user.email=test@example.invalid", "-c", "user.name=test", "commit", "--quiet", "-m", directionExport},
	}
	for _, args := range gitSteps {
		done := leSTERun(ctx, export, nil, "git", args...)
		if done.code != 0 || done.err != nil {
			return leSTEFailf("git %s in the export failed with %d: %s%s%v", args[0], done.code, done.stdout, done.stderr, done.err)
		}
	}
	status := leSTERun(ctx, export, nil, "git", "status", "--porcelain", "--untracked-files=all")
	if status.code != 0 || status.err != nil {
		return leSTEFailf("reading the export status failed with %d: %s%s%v", status.code, status.stdout, status.stderr, status.err)
	}
	if len(status.stdout) != 0 {
		return leSTEFailf("the newly committed export is not clean: %q", status.stdout)
	}

	productEnv := leSTESetEnv(os.Environ(), "ZE_REPO_ROOT", export)
	le := func(args ...string) leSTEResult {
		return leSTERun(ctx, here, productEnv, binary, args...)
	}

	// The whole-tree page must be successful, stable, ordered, and quiet on stderr.
	page := le("ste", "review")
	if page.code != 0 || page.err != nil {
		return leSTEFailf("the review command failed with %d: %s%s%v", page.code, page.stdout, page.stderr, page.err)
	}
	if len(page.stderr) != 0 {
		return leSTEFailf("the review command wrote to stderr: %s", page.stderr)
	}
	if len(page.stdout) == 0 || page.stdout[len(page.stdout)-1] != '\n' {
		return leSTEFailf("the review page is empty or lacks its final newline")
	}
	pageAgain := le("ste", "review")
	if pageAgain.code != 0 || pageAgain.err != nil || len(pageAgain.stderr) != 0 || !bytes.Equal(page.stdout, pageAgain.stdout) {
		return leSTEFailf("the ordered review page is not stable across identical reads\nfirst tail:\n%s\nsecond tail:\n%s", leSTETail(page.stdout), leSTETail(pageAgain.stdout))
	}

	payloadResult := le("ste", "review", "|", "json")
	if payloadResult.code != 0 || payloadResult.err != nil {
		return leSTEFailf("the review payload failed with %d: %s%s%v", payloadResult.code, payloadResult.stdout, payloadResult.stderr, payloadResult.err)
	}
	if len(payloadResult.stderr) != 0 {
		return leSTEFailf("the review payload wrote to stderr: %s", payloadResult.stderr)
	}
	var payload leSTEReview
	if err := json.Unmarshal(payloadResult.stdout, &payload); err != nil {
		return leSTEFailf("decoding the review payload: %v\npayload tail: %s", err, leSTETail(payloadResult.stdout))
	}
	if payload.DocumentsReviewed < 8000 {
		return leSTEFailf("the review read only %d documents, so the corpus walk is vacuous", payload.DocumentsReviewed)
	}
	if len(payload.Findings) < 40000 {
		return leSTEFailf("the review returned only %d findings, so the detector population is vacuous", len(payload.Findings))
	}
	total := 0
	for _, count := range payload.Totals {
		total += count
	}
	if total != len(payload.Findings) {
		return leSTEFailf("the tally has %d findings but the finding list has %d", total, len(payload.Findings))
	}
	for _, surface := range []string{"markdown", "go-comments", "yang-descriptions"} {
		count := 0
		for _, n := range payload.Counts[surface] {
			count += n
		}
		if count == 0 {
			return leSTEFailf("the %s surface contributed no finding", surface)
		}
	}
	payloadAgain := le("ste", "review", "|", "json")
	if payloadAgain.code != 0 || payloadAgain.err != nil || len(payloadAgain.stderr) != 0 || !bytes.Equal(payloadResult.stdout, payloadAgain.stdout) {
		return leSTEFailf("the ordered review payload is not stable across identical reads")
	}

	// The ratchet over the committed export examines no documents and passes.
	clean := le("ste", "check")
	if clean.code != 0 || clean.err != nil {
		return leSTEFailf("the clean gate failed with %d: %s%s%v", clean.code, clean.stdout, clean.stderr, clean.err)
	}
	if len(clean.stderr) != 0 {
		return leSTEFailf("the clean gate wrote to stderr: %s", clean.stderr)
	}
	if !bytes.Contains(clean.stdout, []byte("in 0 changed document(s)")) {
		return leSTEFailf("the clean gate did not report its zero-document boundary: %s", clean.stdout)
	}
	cleanAgain := le("ste", "check")
	if cleanAgain.code != 0 || cleanAgain.err != nil || len(cleanAgain.stderr) != 0 || !bytes.Equal(clean.stdout, cleanAgain.stdout) {
		return leSTEFailf("the clean gate verdict is not stable across identical reads")
	}

	guide := filepath.Join(export, "docs", "guide", "quickstart.md")
	original, err := os.ReadFile(guide) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return leSTEFailf("reading docs/guide/quickstart.md: %v", err)
	}
	changedText := append(append([]byte(nil), original...), []byte("\nThe daemon may start. It should typically work.\n")...)
	if err := os.WriteFile(guide, changedText, 0o600); err != nil {
		return leSTEFailf("changing docs/guide/quickstart.md: %v", err)
	}
	restore := true
	defer func() {
		if restore {
			_ = os.WriteFile(guide, original, 0o600)
		}
	}()

	status = leSTERun(ctx, export, nil, "git", "status", "--porcelain", "--untracked-files=all")
	if status.code != 0 || status.err != nil {
		return leSTEFailf("reading the changed export status failed with %d: %s%s%v", status.code, status.stdout, status.stderr, status.err)
	}
	if string(status.stdout) != " M docs/guide/quickstart.md\n" {
		return leSTEFailf("the export does not contain exactly the intended changed document: %q", status.stdout)
	}

	grew := le("ste", "check")
	if grew.code != 3 || grew.err == nil {
		return leSTEFailf("the growth gate answered %d, want 3: %s%s", grew.code, grew.stdout, grew.stderr)
	}
	if len(grew.stderr) != 0 {
		return leSTEFailf("the growth gate wrote to stderr: %s", grew.stderr)
	}
	if len(grew.stdout) == 0 || !bytes.Contains(grew.stdout, []byte("docs/guide/quickstart.md")) {
		return leSTEFailf("the growth verdict does not identify the changed document: %s", grew.stdout)
	}
	grewAgain := le("ste", "check")
	if grewAgain.code != 3 || grewAgain.err == nil || len(grewAgain.stderr) != 0 || !bytes.Equal(grew.stdout, grewAgain.stdout) {
		return leSTEFailf("the growth verdict is not stable across identical reads")
	}

	changed := le("ste", "review-changed")
	if changed.code != 0 || changed.err != nil {
		return leSTEFailf("review-changed failed with %d: %s%s%v", changed.code, changed.stdout, changed.stderr, changed.err)
	}
	if len(changed.stderr) != 0 {
		return leSTEFailf("review-changed wrote to stderr: %s", changed.stderr)
	}
	if len(changed.stdout) == 0 || !bytes.Contains(changed.stdout, []byte("docs/guide/quickstart.md")) {
		return leSTEFailf("review-changed omitted the changed document: %s", changed.stdout)
	}
	changedAgain := le("ste", "review-changed")
	if changedAgain.code != 0 || changedAgain.err != nil || len(changedAgain.stderr) != 0 || !bytes.Equal(changed.stdout, changedAgain.stdout) {
		return leSTEFailf("the ordered changed-file report is not stable across identical reads")
	}

	scoped := le("ste", "check", "file", "docs/guide/quickstart.md")
	if scoped.code != 3 || scoped.err == nil {
		return leSTEFailf("the scoped growth gate answered %d, want 3: %s%s", scoped.code, scoped.stdout, scoped.stderr)
	}
	if len(scoped.stderr) != 0 {
		return leSTEFailf("the scoped growth gate wrote to stderr: %s", scoped.stderr)
	}
	if len(scoped.stdout) == 0 || !bytes.Contains(scoped.stdout, []byte("docs/guide/quickstart.md")) {
		return leSTEFailf("the scoped growth verdict omitted its document: %s", scoped.stdout)
	}

	other := le("ste", "check", "file", "README.md")
	if other.code != 0 || other.err != nil {
		return leSTEFailf("an unchanged file failed the gate with %d: %s%s%v", other.code, other.stdout, other.stderr, other.err)
	}
	if len(other.stderr) != 0 {
		return leSTEFailf("the unchanged-file gate wrote to stderr: %s", other.stderr)
	}
	if !bytes.Contains(other.stdout, []byte("in 1 changed document(s)")) {
		return leSTEFailf("the explicit-file scope did not report one selected document: %s", other.stdout)
	}

	if err := os.WriteFile(guide, original, 0o600); err != nil {
		return leSTEFailf("restoring docs/guide/quickstart.md: %v", err)
	}
	restore = false
	status = leSTERun(ctx, export, nil, "git", "status", "--porcelain", "--untracked-files=all")
	if status.code != 0 || status.err != nil || len(status.stdout) != 0 {
		return leSTEFailf("the restored export is not clean: code=%d stdout=%q stderr=%q err=%v", status.code, status.stdout, status.stderr, status.err)
	}

	// Every verdict remains reachable through every generic rendering operator.
	for _, verb := range []string{actionCheck, "review", "review-changed"} {
		for _, operator := range []string{renderJSON, renderYAML, renderTable} {
			piped := le("ste", verb, "|", operator)
			if piped.code != 0 && piped.code != 3 {
				return leSTEFailf("`le ste %s | %s` was refused with %d: %s", verb, operator, piped.code, piped.stderr)
			}
			if piped.err != nil && piped.code != 3 {
				return leSTEFailf("`le ste %s | %s` failed: %v", verb, operator, piped.err)
			}
			if len(piped.stderr) != 0 {
				return leSTEFailf("`le ste %s | %s` wrote to stderr: %s", verb, operator, piped.stderr)
			}
			if len(bytes.TrimSpace(piped.stdout)) == 0 {
				return leSTEFailf("`le ste %s | %s` rendered nothing", verb, operator)
			}
			if operator == renderJSON && !json.Valid(piped.stdout) {
				return leSTEFailf("`le ste %s | json` did not render valid JSON: %s", verb, leSTETail(piped.stdout))
			}
		}
	}

	listing := le("ste")
	if listing.code != 0 || listing.err != nil {
		return leSTEFailf("listing the ste area failed with %d: %s%s%v", listing.code, listing.stdout, listing.stderr, listing.err)
	}
	if len(listing.stderr) != 0 {
		return leSTEFailf("the ste listing wrote to stderr: %s", listing.stderr)
	}
	for _, word := range []string{actionCheck, "review", "review-changed"} {
		if !bytes.Contains(listing.stdout, []byte(word)) {
			return leSTEFailf("the listing does not carry %q:\n%s", word, listing.stdout)
		}
	}
	listingAgain := le("ste")
	if listingAgain.code != 0 || listingAgain.err != nil || len(listingAgain.stderr) != 0 || !bytes.Equal(listing.stdout, listingAgain.stdout) {
		return leSTEFailf("the ordered ste listing is not stable across identical reads")
	}

	actionsResult := le("ste", "|", "json")
	if actionsResult.code != 0 || actionsResult.err != nil {
		return leSTEFailf("the ste action payload failed with %d: %s%s%v", actionsResult.code, actionsResult.stdout, actionsResult.stderr, actionsResult.err)
	}
	if len(actionsResult.stderr) != 0 {
		return leSTEFailf("the ste action payload wrote to stderr: %s", actionsResult.stderr)
	}
	var area struct {
		Actions []struct {
			Writes bool `json:"writes"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(actionsResult.stdout, &area); err != nil {
		return leSTEFailf("decoding the ste action payload: %v", err)
	}
	if len(area.Actions) != 3 {
		return leSTEFailf("the area lists %d actions, want 3", len(area.Actions))
	}
	for i, action := range area.Actions {
		if action.Writes {
			return leSTEFailf("action %d claims to write", i)
		}
	}

	refused := le("ste", "review", "docs")
	if refused.code != 2 || refused.err == nil {
		return leSTEFailf("a path argument answered %d, want 2", refused.code)
	}
	if !bytes.Contains(refused.stderr, []byte("takes no arguments")) {
		return leSTEFailf("the path refusal is silent: %q", refused.stderr)
	}

	unknown := le("ste", "check", "docs/guide/quickstart.md")
	if unknown.code != 1 || unknown.err == nil {
		return leSTEFailf("a bare path answered %d, want 1", unknown.code)
	}
	if !bytes.Contains(unknown.stderr, []byte("unknown keyword")) {
		return leSTEFailf("the unknown-keyword refusal is silent: %q", unknown.stderr)
	}

	missing := le("ste", "nonesuch")
	if missing.code != 2 || missing.err == nil {
		return leSTEFailf("an action the area does not hold answered %d, want 2", missing.code)
	}

	fmt.Fprintln(os.Stdout, "OK") //nolint:errcheck // progress output
	return nil
}

func leSTERun(ctx context.Context, dir string, env []string, name string, args ...string) leSTEResult {
	cmd := exec.CommandContext(ctx, name, args...) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = dir
	if env != nil {
		cmd.Env = env
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := leSTEResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), err: err}
	if err == nil {
		result.code = 0
		return result
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		result.code = exitErr.ExitCode()
		return result
	}
	result.code = -1
	return result
}

func leSTEExport(ctx context.Context, root, export string) error {
	cmd := exec.CommandContext(ctx, "git", "archive", "--format=tar", "HEAD")
	cmd.Dir = root
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return leSTEFailf("opening git archive output: %v", err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return leSTEFailf("starting git archive HEAD: %v", err)
	}
	extractErr := leSTEExtractTar(stdout, export)
	if extractErr != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if extractErr != nil {
		return leSTEFailf("extracting the export failed: %v", extractErr)
	}
	if waitErr != nil {
		return leSTEFailf("git archive HEAD failed: %v: %s", waitErr, stderr.Bytes())
	}
	if stderr.Len() != 0 {
		return leSTEFailf("git archive HEAD wrote to stderr: %s", stderr.Bytes())
	}
	return nil
}

func leSTEExtractTar(r io.Reader, destination string) error {
	type directoryMode struct {
		path string
		mode os.FileMode
	}
	var directories []directoryMode
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(hdr.Name))
		if name == "." {
			continue
		}
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive path %q", hdr.Name)
		}
		path := filepath.Join(destination, name)
		if !leSTEWithin(destination, path) {
			return fmt.Errorf("archive path escapes export: %q", hdr.Name)
		}
		mode := os.FileMode(hdr.Mode) & os.ModePerm
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o750); err != nil {
				return err
			}
			directories = append(directories, directoryMode{path: path, mode: mode})
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				return err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode) //nolint:gosec // the path is the fixture's own scratch file
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, tr) //nolint:gosec // the archive is the fixture's own git export
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			// A link target is a second path into the export, so it is held to
			// the same containment as the entry name above.
			if filepath.IsAbs(hdr.Linkname) {
				return fmt.Errorf("unsafe absolute symlink target %q", hdr.Linkname)
			}
			if !leSTEWithin(destination, filepath.Join(filepath.Dir(path), filepath.FromSlash(hdr.Linkname))) {
				return fmt.Errorf("symlink %q escapes export", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, path); err != nil {
				return err
			}
		case tar.TypeXHeader, tar.TypeXGlobalHeader:
			continue
		default:
			return fmt.Errorf("unsupported archive entry %q with type %d", hdr.Name, hdr.Typeflag)
		}
	}
	for _, directory := range slices.Backward(directories) {
		if err := os.Chmod(directory.path, directory.mode); err != nil {
			return err
		}
	}
	return nil
}

func leSTESetEnv(environment []string, key, value string) []string {
	prefix := key + "="
	out := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			out = append(out, entry)
		}
	}
	return append(out, prefix+value)
}

func leSTEWithin(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// steTailBytes is how much of a failed page the report keeps.
const steTailBytes = 1200

func leSTETail(data []byte) string {
	if len(data) <= steTailBytes {
		return string(data)
	}
	return string(data[len(data)-steTailBytes:])
}

func leSTEFailf(format string, args ...any) error {
	return fmt.Errorf("FAIL: "+format, args...)
}
