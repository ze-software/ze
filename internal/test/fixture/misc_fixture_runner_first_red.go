package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// verifyFirstRedRefusesACutDriver types `le verify worktree part 1 of 2
// first-red` against a throwaway checkout and asserts the refusal: exit 2,
// the reason (a piece is dealt for debt clearing), the two spellings that
// work, and no worktree created.
func verifyFirstRedRefusesACutDriver(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("verify-first-red-refuses-a-cut fixture takes no arguments")
	}
	le, err := nativeLEBinary()
	if err != nil {
		return err
	}
	repo, err := os.MkdirTemp("", "ze-verify-first-red-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(repo) //nolint:errcheck // fixture cleanup

	out, code, err := rawCommand(ctx, repo, envRootedAt(repo), le,
		"verify", "worktree", "part", "1", "of", "2", "first-red")
	if err != nil {
		return fmt.Errorf("run le verify worktree: %w", err)
	}
	if code != 2 {
		return fmt.Errorf("le verify worktree part 1 of 2 first-red exit=%d, want 2: %s", code, out)
	}
	for _, want := range []string{
		"first-red cannot be combined with part",
		"debt clearing",
		"le verify worktree first-red",
		"le verify worktree part <i> of <n>",
	} {
		if !strings.Contains(out, want) {
			return fmt.Errorf("refusal does not say %q: %s", want, out)
		}
	}
	fmt.Fprintln(os.Stdout, "first-red-with-part-refused") //nolint:errcheck // progress output
	if _, err := os.Stat(filepath.Join(repo, "tmp", "verify-worktree")); !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("the refused line created a worktree directory (%v)", err)
	}
	fmt.Fprintln(os.Stdout, "no-worktree-created") //nolint:errcheck // progress output
	return nil
}
