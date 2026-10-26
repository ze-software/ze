package fixture

// Related: register_le_retired_spellings.go -- the registration.
// Related: ui_fixture_le_launcher_words.go -- leLauncherWordsRun, shared.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/component/aihelp/reference"
	"github.com/ze-software/ze/internal/core/textbuf"
	leretired "github.com/ze-software/ze/internal/le/le/retired"
)

// leRetiredSpellingsAI is the namespace whose pages `le help ai ...` cannot
// ask: `ai` after `help` selects the AI reference, so its pages are asked with
// a trailing help word.
const leRetiredSpellingsAI = "ai"

// leRetiredSpellings builds the le personality and, for every row of the rename
// map, runs the old spelling (refused, exit 2, naming the successor line) and
// asks the successor's page (exit 0, listing the successor's action). The rows
// are read from leretired.Renames, so a row added to the map is checked here
// with no edit to this file.
func leRetiredSpellings(ctx context.Context) error {
	if os.Getenv("ZE_REPO_ROOT") == "" {
		return errors.New("ZE_REPO_ROOT is not set")
	}
	root, err := filepath.Abs(os.Getenv("ZE_REPO_ROOT"))
	if err != nil {
		return fmt.Errorf("resolve ZE_REPO_ROOT: %w", err)
	}
	work, err := os.MkdirTemp("", "le-retired-spellings-")
	if err != nil {
		return fmt.Errorf("create fixture directory: %w", err)
	}
	defer os.RemoveAll(work) //nolint:errcheck // fixture cleanup

	lePath, _, err := leHelpPagesBuild(ctx, root, work)
	if err != nil {
		return err
	}
	rows := leretired.Renames()
	if len(rows) == 0 {
		return errors.New("the rename map holds no row, so nothing was checked")
	}
	passes, err := leRetiredSpellingsPassThrough(ctx, root, lePath)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err := leRetiredSpellingsRow(ctx, root, lePath, row, passes); err != nil {
			return err
		}
	}
	// One successor run for real, beyond its page: `verify lock` became `job`,
	// whose bare line answers its action listing.
	job, err := leLauncherWordsRun(ctx, root, lePath, "job")
	if err != nil {
		return err
	}
	if job.exitCode != 0 || !strings.Contains(job.stdout, "run") {
		return fmt.Errorf("le job answered %d:\n%s%s", job.exitCode, job.stdout, job.stderr)
	}
	var tb textbuf.Buffer
	tb.Str("refused ").Int(int64(len(rows))).Str(" retired spellings, each naming its successor; every successor page answers\n").
		// The .ci reads this line; an exit code of 0 alone would also pass
		// a driver that returned before its checks ran.
		Str("OK\n").StdOut() //nolint:errcheck // fixture output
	return nil
}

// leRetiredSpellingsPassThrough answers the commands that pass their words
// to another program, read from `le help ai commands | json`.
func leRetiredSpellingsPassThrough(ctx context.Context, root, lePath string) (map[string]bool, error) {
	answer, err := leLauncherWordsRun(ctx, root, lePath, leHelpPagesHelpWord, leRetiredSpellingsAI, "commands", "|", renderJSON)
	if err != nil {
		return nil, err
	}
	if answer.exitCode != 0 {
		return nil, fmt.Errorf("le help ai commands | json answered %d: %s", answer.exitCode, answer.stderr)
	}
	var ref reference.Reference
	if err := json.Unmarshal([]byte(answer.stdout), &ref); err != nil {
		return nil, fmt.Errorf("le help ai commands | json is not the reference: %w", err)
	}
	passes := map[string]bool{}
	for _, command := range ref.Commands {
		if command.PassesTo != "" {
			passes[command.Name] = true
		}
	}
	if len(ref.Commands) == 0 {
		return nil, errors.New("le help ai commands | json lists no command")
	}
	return passes, nil
}

// leRetiredSpellingsRow checks one row: the old words are refused naming the
// new line, and the new command's page answers and lists the new action.
func leRetiredSpellingsRow(ctx context.Context, root, lePath string, row leretired.Rename, passes map[string]bool) error {
	old := strings.Join(row.Old(), " ")
	refused, err := leLauncherWordsRun(ctx, root, lePath, row.Old()...)
	if err != nil {
		return err
	}
	want := "le " + old + " was renamed: run le " + strings.Join(row.New(), " ")
	if refused.exitCode != 2 || !strings.Contains(refused.stderr, want) {
		return fmt.Errorf("le %s answered %d, want 2 and %q; stderr %q", old, refused.exitCode, want, refused.stderr)
	}
	command := strings.Fields(row.Command)
	args := append([]string{leHelpPagesHelpWord}, command...)
	if command[0] == leRetiredSpellingsAI {
		args = append(slices.Clone(command), "--help")
	}
	page, err := leLauncherWordsRun(ctx, root, lePath, args...)
	if err != nil {
		return err
	}
	if page.exitCode != 0 {
		return fmt.Errorf("le %s (the successor of %s) answered %d: %s", strings.Join(args, " "), old, page.exitCode, page.stderr)
	}
	action := strings.Fields(row.Action)
	if len(action) == 0 {
		return nil
	}
	// A pass-through successor hands its words to another program, which
	// owns them, so le's page names the program rather than the word.
	if passes[row.Command] {
		return nil
	}
	for line := range strings.SplitSeq(page.stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == action[0] || strings.HasPrefix(trimmed, action[0]+" ") {
			return nil
		}
	}
	return fmt.Errorf("le %s (the successor of %s) does not list the action %s:\n%s", strings.Join(args, " "), old, action[0], page.stdout)
}
