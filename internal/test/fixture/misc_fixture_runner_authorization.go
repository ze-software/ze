// Design: docs/contributing/committing.md -- authorization issuance and noninteractive debt queries.
package fixture

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// expectPipedAuthorizationRefusal drives the actual native issuer with the
// runner's pipes and requires refusal before any scratch-checkout write.
func (s *derivedDebtScratch) expectPipedAuthorizationRefusal(ctx context.Context, args ...string) error {
	before, err := runnerAuthorizationSnapshot(s.repo)
	if err != nil {
		return err
	}
	out, code, err := s.le(ctx, args...)
	if err != nil {
		return err
	}
	if code != 2 {
		return fmt.Errorf("piped owner authorization exit %d, want 2: %s", code, out)
	}
	if !strings.Contains(out, "owner authorization requires interactive stdin and stdout") {
		return fmt.Errorf("piped issuer did not name its terminal boundary: %s", out)
	}
	after, err := runnerAuthorizationSnapshot(s.repo)
	if err != nil {
		return err
	}
	if !maps.Equal(before, after) {
		return errors.New("refused piped owner authorization changed the scratch checkout")
	}
	return nil
}

// runnerAuthorizationSnapshot includes ignored session artifacts and existing
// script bytes, so a refused issuer cannot conceal a write from git status.
func runnerAuthorizationSnapshot(root string) (map[string]string, error) {
	entries := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			body, err := os.ReadFile(path) //nolint:gosec // only this fixture's private scratch checkout
			if err != nil {
				return err
			}
			value += string(body)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			value += target
		}
		entries[path] = value
		return nil
	})
	return entries, err
}

// expectDebtStatus checks a nonauthorizing CLI answer over the named commit.
func (s *derivedDebtScratch) expectDebtStatus(ctx context.Context, target string, wantCode int, wants ...string) error {
	args := []string{argCommit, "debt-status"}
	if target != "" {
		args = append(args, "commit", target)
	}
	out, code, err := s.le(ctx, args...)
	if err != nil {
		return err
	}
	if code != wantCode {
		return fmt.Errorf("debt-status exit %d, want %d: %s", code, wantCode, out)
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			return fmt.Errorf("debt-status lacks %q: %s", want, out)
		}
	}
	if strings.Contains(out, "le verify") {
		return fmt.Errorf("debt-status named a verify a push does not owe: %s", out)
	}
	return nil
}
