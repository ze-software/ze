// Design: docs/contributing/spec-workflow.md -- immutable roadmap contract.
// Tests drive registered commands over isolated Git object stores, without staging
// or committing anything in the developer checkout.
package roadmap

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/derived"
	"github.com/ze-software/ze/internal/le/leroot"
)

func fixtureGit(t *testing.T, root, input string, args ...string) string {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, args...)...)
	command.Stdin = strings.NewReader(input)
	command.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture git %v: %v: %s", args, err, output)
	}
	return strings.TrimSpace(string(output))
}

func fixtureRepository(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	fixtureGit(t, root, "", "init", "-q", "--initial-branch=main")
	return root
}

// fixtureCommit replaces the committed tree through fast-import. It never touches
// a working-tree index; arbitrary historical and malformed specs remain test data.
func fixtureCommit(t *testing.T, root string, files map[string]string) string {
	t.Helper()
	var stream strings.Builder
	stream.WriteString("commit refs/heads/main\ncommitter Fixture <fixture@example.test> 1700000000 +0000\ndata 7\nfixture\n")
	// Each import process needs an explicit parent for an existing branch.
	if refs := fixtureGit(t, root, "", "for-each-ref", "--format=%(objectname)", "refs/heads/main"); refs != "" {
		fmt.Fprintf(&stream, "from %s\n", refs)
	}
	stream.WriteString("deleteall\n")
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		body := files[path]
		fmt.Fprintf(&stream, "M 100644 inline %s\ndata %d\n%s\n", strconv.Quote(path), len(body), body)
	}
	stream.WriteString("\ndone\n")
	fixtureGit(t, root, stream.String(), "fast-import", "--quiet")
	return fixtureGit(t, root, "", "rev-parse", "HEAD")
}

func fixtureSpec(title, status, depends string) string {
	return fmt.Sprintf("# Spec: %s\n\n| Field | Value |\n|---|---|\n| Status | %s |\n| Depends | %s |\n| Phase | 1/2 |\n| Updated | 2026-09-18 |\n", title, status, depends)
}

func commandRoot(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

func dispatch(t *testing.T, args ...string) (string, int) {
	t.Helper()
	output, err := os.CreateTemp(t.TempDir(), "stdout-*")
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = output
	defer func() {
		os.Stdout = original
		if err := output.Close(); err != nil {
			t.Error(err)
		}
	}()
	code := leroot.Dispatch("le", append([]string{"spec", "roadmap"}, args...))
	data, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	return string(data), code
}

// Registered list and common pipes must describe the requested tree even when HEAD
// and local files disagree. Invalid history must fail instead of using HEAD.
func TestRoadmapCommandUsesSelectedRevision(t *testing.T) {
	root := fixtureRepository(t)
	old := fixtureCommit(t, root, map[string]string{
		"plan/immediate/spec-wire.md": fixtureSpec("Wire contract", "blocked", "-"),
	})
	fixtureCommit(t, root, map[string]string{"plan/README.md": "empty inventory"})
	if err := os.MkdirAll(filepath.Join(root, "plan"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plan", "spec-untracked.md"), []byte(fixtureSpec("Ghost", "ready", "-")), 0o600); err != nil {
		t.Fatal(err)
	}
	commandRoot(t, root)
	output, code := dispatch(t, "list", "revision", old, "|", "json")
	if code != 0 {
		t.Fatalf("registered list: %d: %s", code, output)
	}
	var snapshot Snapshot
	if err := json.Unmarshal([]byte(output), &snapshot); err != nil {
		t.Fatalf("decode list: %v: %s", err, output)
	}
	if snapshot.Revision != old {
		t.Fatalf("revision = %s, want %s", snapshot.Revision, old)
	}
	if snapshot.Counts.Required != 1 {
		t.Fatalf("required = %d, want 1", snapshot.Counts.Required)
	}
	if len(snapshot.Items) != 1 {
		t.Fatalf("items = %+v", snapshot.Items)
	}
	if snapshot.Items[0].Status != "blocked" {
		t.Fatalf("historical status lost: %+v", snapshot.Items[0])
	}
	for _, format := range []string{"yaml", "table"} {
		output, code := dispatch(t, "list", "revision", old, "|", format)
		if code != 0 {
			t.Fatalf("%s: code %d: %s", format, code, output)
		}
		if !strings.Contains(output, old) {
			t.Fatalf("%s lost selected revision: %s", format, output)
		}
	}
	output, code = dispatch(t, "list", "|", "json")
	if code != 0 {
		t.Fatalf("default HEAD: %d: %s", code, output)
	}
	if err := json.Unmarshal([]byte(output), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Counts.Total != 0 {
		t.Fatalf("untracked spec entered HEAD report: %+v", snapshot)
	}
	if _, code := dispatch(t, "list", "revision", "missing-history"); code == 0 {
		t.Fatal("unknown history answered success")
	}
	if _, err := Collect(t.Context(), root, "missing-history"); !errors.Is(err, ErrRevision) {
		t.Fatalf("unknown revision: %v", err)
	}
	if _, code := dispatch(t, "compare", "from", old); code == 0 {
		t.Fatal("missing comparison endpoint answered success")
	}
}

// Every canonical file remains in the count, including parking and malformed states.
// Missing dates stay absent and duplicate identities must fail with both paths.
func TestRoadmapBucketAccounting(t *testing.T) {
	root := fixtureRepository(t)
	files := map[string]string{
		"plan/immediate/spec-umbrella.md":   fixtureSpec("Umbrella", "blocked", "-"),
		"plan/pre-release/spec-child.md":    fixtureSpec("Child", "deferred", "spec-umbrella.md"),
		"plan/immediate/spec-captured.md":   fixtureSpec("Idea", "skeleton", "-"),
		"plan/pre-release/spec-verify.md":   fixtureSpec("Review", "verification", "-"),
		"plan/spec-optional.md":             fixtureSpec("Optional", "ready", "-"),
		"plan/immediate/spec-missing.md":    "No metadata or title\n",
		"plan/pre-release/spec-unknown.md":  "# Spec: Unknown\n\n| Field | Value |\n|---|---|\n| Status | surprising |\n",
		"plan/immediate/spec-active.md":     fixtureSpec("Active", "in-progress", "-"),
		"plan/immediate/spec-active-two.md": fixtureSpec("Active sibling", "in-progress", "-"),
		"plan/immediate/spec-no-status.md":  "# Spec: Missing status\n\n| Field | Value |\n|---|---|\n| Updated | invalid |\n",
		"plan/pre-release/spec-planning.md": fixtureSpec("Planning", "design", "-"),
		"plan/deep/spec-excluded.md":        fixtureSpec("Outside population", "ready", "-"),
	}
	revision := fixtureCommit(t, root, files)
	snapshot, err := Collect(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Counts.Total != 11 {
		t.Fatalf("total = %d, want 11", snapshot.Counts.Total)
	}
	if snapshot.Counts.Required != 10 {
		t.Fatalf("required = %d, want 10", snapshot.Counts.Required)
	}
	if snapshot.Counts.NiceToHave != 1 {
		t.Fatalf("optional = %d, want 1", snapshot.Counts.NiceToHave)
	}
	if !reflect.DeepEqual(snapshot.Counts.Buckets, map[string]int{"after": 1, "immediate": 6, "pre-release": 4}) {
		t.Fatalf("bucket counts = %v", snapshot.Counts.Buckets)
	}
	wantStatuses := map[string]int{
		"blocked": 1, "deferred": 1, "skeleton": 1, "verification": 1,
		"ready": 1, "unparsed": 1, "surprising": 1, "in-progress": 2, "unknown": 1, "design": 1,
	}
	if !reflect.DeepEqual(snapshot.Counts.Statuses, wantStatuses) {
		t.Fatalf("status counts omitted obligations: %v", snapshot.Counts.Statuses)
	}
	names := make([]string, 0, len(snapshot.Items))
	for _, item := range snapshot.Items {
		names = append(names, item.Name)
	}
	wantOrder := []string{"optional", "missing", "active", "active-two", "captured", "umbrella", "no-status",
		"verify", "planning", "child", "unknown"}
	if !slices.Equal(names, wantOrder) {
		t.Fatalf("bucket/status/identity order = %v, want %v", names, wantOrder)
	}
	for _, item := range snapshot.Items {
		switch item.Name {
		case "missing", "unknown":
			if item.Updated != "" {
				t.Fatalf("date invented for %s: %s", item.Name, item.Updated)
			}
			if len(item.Diagnostics) == 0 {
				t.Fatalf("malformed item %s has no diagnostic", item.Name)
			}
		}
	}
	repeated, err := Collect(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(snapshot, repeated) {
		t.Fatal("same revision changed its report")
	}
	if Markdown(&snapshot, false) != Markdown(&repeated, false) {
		t.Fatal("same revision changed its Markdown")
	}
	files["plan/spec-child.md"] = fixtureSpec("Duplicate", "design", "-")
	duplicate := fixtureCommit(t, root, files)
	_, err = Collect(t.Context(), root, duplicate)
	if !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate identity: %v", err)
	}
	for _, path := range []string{"plan/spec-child.md", "plan/pre-release/spec-child.md"} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("duplicate error omits %s: %v", path, err)
		}
	}
	empty := fixtureCommit(t, root, map[string]string{"plan/README.md": "empty"})
	snapshot, err = Collect(t.Context(), root, empty)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Counts.Total != 0 {
		t.Fatalf("empty population: %+v", snapshot)
	}
	if !strings.Contains(Markdown(&snapshot, true), "No spec work items") {
		t.Fatal("empty population has no explicit presentation")
	}
	missing := fixtureCommit(t, root, map[string]string{"README.md": "no plan tree"})
	if _, err := Collect(t.Context(), root, missing); !errors.Is(err, ErrPlan) {
		t.Fatalf("missing plan answered as empty: %v", err)
	}
}

// Moves and state changes coexist; renamed stems have no guessed identity, and a
// transient item between endpoints cannot be counted as observed throughput.
func TestRoadmapComparisonIsNotCompletion(t *testing.T) {
	root := fixtureRepository(t)
	from := fixtureCommit(t, root, map[string]string{
		"plan/immediate/spec-move.md":      fixtureSpec("Move", "blocked", "-"),
		"plan/pre-release/spec-removed.md": fixtureSpec("Removed", "verification", "-"),
		"plan/spec-old-name.md":            fixtureSpec("Renamed", "ready", "-"),
	})
	fixtureCommit(t, root, map[string]string{
		"plan/spec-transient.md": fixtureSpec("Inside interval", "in-progress", "-"),
	})
	to := fixtureCommit(t, root, map[string]string{
		"plan/spec-move.md":              fixtureSpec("Move", "ready", "-"),
		"plan/spec-new-name.md":          fixtureSpec("Renamed", "ready", "-"),
		"plan/pre-release/spec-added.md": fixtureSpec("Added", "skeleton", "-"),
	})
	commandRoot(t, root)
	output, code := dispatch(t, "compare", "from", from, "to", to, "|", "json")
	if code != 0 {
		t.Fatalf("registered compare: %d: %s", code, output)
	}
	var report Comparison
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Changes) != 5 {
		t.Fatalf("changes = %+v", report.Changes)
	}
	for _, change := range report.Changes {
		switch change.Name {
		case "move":
			if !change.Moved {
				t.Fatal("bucket relocation disappeared")
			}
			if !change.StatusChanged {
				t.Fatal("relocation swallowed status change")
			}
			if change.Before.Bucket != "immediate" {
				t.Fatalf("old bucket = %s", change.Before.Bucket)
			}
			if change.After.Status != "ready" {
				t.Fatalf("new status = %s", change.After.Status)
			}
		case "removed", "old-name":
			if change.Kind != "removed" {
				t.Fatalf("removal inferred completion: %+v", change)
			}
			if !strings.Contains(change.Before.SourceURL, "/"+from+"/") {
				t.Fatalf("removed link is not pinned: %s", change.Before.SourceURL)
			}
		case "added", "new-name":
			if change.Kind != "added" {
				t.Fatalf("addition = %+v", change)
			}
		default:
			t.Fatalf("unobserved identity: %+v", change)
		}
	}
	if report.From.Counts.Required != 2 {
		t.Fatalf("old required count = %d", report.From.Counts.Required)
	}
	if report.To.Counts.Required != 1 {
		t.Fatalf("new required count = %d", report.To.Counts.Required)
	}
	if report.Limitation != EndpointLimit {
		t.Fatalf("endpoint limit lost: %s", report.Limitation)
	}
}

// Registered update uses the same snapshot, escapes hostile metadata, links only
// exact references, and becomes stale after HEAD changes even if the file remains.
func TestRoadmapIndexMatchesSnapshot(t *testing.T) {
	root := fixtureRepository(t)
	revision := fixtureCommit(t, root, map[string]string{
		"plan/immediate/spec-safe.md": fixtureSpec("<script>alert(1)</script> [bad](javascript:evil) {{ze:unknown}}", "ready", "spec-target.md; prose <img src=x>"),
		"plan/spec-target.md":         fixtureSpec("Target", "design", "unknown/spec-safe.md spec-missing.md"),
	})
	commandRoot(t, root)
	output, code := dispatch(t, "update", "|", "json")
	if code != 0 {
		t.Fatalf("registered update: %d: %s", code, output)
	}
	var report UpdateReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatal(err)
	}
	if report.Revision != revision {
		t.Fatalf("update revision = %s", report.Revision)
	}
	snapshot, err := Collect(t.Context(), root, revision)
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(filepath.Join(root, OutputRel))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != Markdown(&snapshot, true) {
		t.Fatal("index differs from its reported inventory")
	}
	for _, unsafe := range []string{"<script>", "<img", "](javascript:", "{{ze:", "[spec-missing.md](", "[spec-safe.md]("} {
		if strings.Contains(string(content), unsafe) {
			t.Fatalf("unsafe metadata or unresolved link %q survived: %s", unsafe, content)
		}
	}
	if !strings.Contains(string(content), "[spec-target.md](spec-target.md)") {
		t.Fatal("exact selected dependency has no local link")
	}
	if !strings.Contains(Markdown(&snapshot, false), "/"+revision+"/plan/spec-target.md") {
		t.Fatal("site dependency has no pinned link")
	}
	var artifact *derived.Artifact
	for _, candidate := range derived.All() {
		if candidate.Path == OutputRel {
			artifact = &candidate
			break
		}
	}
	if artifact == nil {
		t.Fatal("roadmap output has no derived registration")
	}
	if !artifact.Complete(root) {
		t.Fatal("fresh index reported stale")
	}
	fixtureCommit(t, root, map[string]string{"plan/README.md": "new tree"})
	if artifact.Complete(root) {
		t.Fatal("HEAD changed but old index reported current")
	}
	output, code = dispatch(t, "update", "revision", revision, "|", "json")
	if code != 0 {
		t.Fatalf("historical update: %d: %s", code, output)
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatal(err)
	}
	if !report.Historical {
		t.Fatal("historical update is not labeled")
	}
	content, err = os.ReadFile(filepath.Join(root, OutputRel))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "Historical snapshot") {
		t.Fatal("historical index has no visible qualification")
	}
	if artifact.Complete(root) {
		t.Fatal("historical index claimed HEAD freshness")
	}
}
