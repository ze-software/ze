// Design: docs/contributing/committing.md -- advertised read-only checker forms dispatch their real authority consumer.
package commit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestAdvertisedApprovalCheckerRouteQueriesActualParent tokenizes the public
// help command as a caller does, rather than quoting a private multiword verb.
// Its real Answer subprocess must query exact ordinary verdict identities and
// leave both Git and session files untouched.
func TestAdvertisedApprovalCheckerRouteQueriesActualParent(t *testing.T) {
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "advertised-approval-checker-route")
	const spent = "RFC-approved: pkg.TestRoute: judge reviewed the spent change"
	const fresh = "RFC-approved: pkg.TestRoute: judge reviewed a different fresh change"
	commitFixture(t, root, "spend an exact ordinary verdict\n\n"+spent, map[string]string{"route.txt": "retained route fixture\n"}, nil)
	parent := headOf(t, root)
	writeCommitFixture(t, root, "tmp/route-pending.txt", "unrelated pending bytes must not change\n")
	installNativeReviewChecker(t, root)

	page, held := Actions().ActionPage("./le", actionApprovalCheck)
	if !held {
		t.Fatal("public help exposes no approval checker command")
	}
	words := strings.Fields(page.Command)
	if len(words) < 3 || words[0] != "./le" || words[1] != "commit" {
		t.Fatalf("public command is not a runnable commit form: %q", page.Command)
	}
	for _, test := range []struct {
		name    string
		verdict string
		want    int
	}{
		{"fresh exact verdict", fresh, 0},
		{"spent exact verdict", spent, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := approvalRouteFiles(t, root)
			arguments := append([]string(nil), words[1:]...)
			arguments = append(arguments, "parent", parent, "verdict", test.verdict)
			command := exec.CommandContext(t.Context(), filepath.Join(root, words[0]), arguments...)
			command.Dir = root
			output, err := command.CombinedOutput()
			code := 0
			if err != nil {
				exit, ok := errors.AsType[*exec.ExitError](err)
				if !ok {
					t.Fatalf("public command could not start: %v\n%s", err, output)
				}
				code = exit.ExitCode()
			}
			if code != test.want {
				t.Errorf("space-tokenized public command %q answered %d, want authority result %d:\n%s", page.Command, code, test.want, output)
			}
			if strings.Contains(string(output), "unknown keyword") || strings.Contains(string(output), "unknown action") {
				t.Errorf("public help command failed dispatch instead of querying its actual parent:\n%s", output)
			}
			if test.want == 1 && !strings.Contains(string(output), "pkg.TestRoute") {
				t.Errorf("spent verdict refusal did not name its actual authority identity:\n%s", output)
			}
			if got := headOf(t, root); got != parent {
				t.Errorf("read-only checker moved HEAD: %s -> %s", parent, got)
			}
			if after := approvalRouteFiles(t, root); !reflect.DeepEqual(after, before) {
				t.Error("read-only checker changed fixture Git, checkout or session files")
			}
		})
	}
}

func approvalRouteFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := make(map[string]string)
	if err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var body []byte
		if !entry.IsDir() {
			body, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		files[path] = fmt.Sprintf("%v\x00%s", info.Mode(), body)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
