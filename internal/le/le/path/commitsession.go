// Design: docs/features/ai-first.md -- per-harness commit-session identity
//
// The eight-hex namespace names every artifact one session prepares: its commit
// script, its commit message, its verification-debt shard and its two test
// ledger shards. It lives here, beside ResolveSession, because the commit gate
// and the pre-write hook both derive a path from it and neither may import the
// other.
package lepath

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var commitSessionPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

// CommitSession returns the reusable eight-hex commit namespace for the
// canonical native harness session. A requested value replaces the stored value.
func CommitSession(root, requested string) (string, error) {
	session, err := ResolveSession(root, false)
	if err != nil {
		return "", err
	}
	return commitSessionFor(root, requested, session.ID)
}

// CommitSessionFor maps one KNOWN harness identity to its commit namespace,
// for a caller that was handed the identity rather than running under it. A
// hook payload names the session whose edit it judges, and the namespace it
// resolves has to be the one that session's commit will use.
func CommitSessionFor(root, harness string) (string, error) {
	return commitSessionFor(root, "", harness)
}

func commitSessionFor(root, requested, fingerprint string) (string, error) {
	if fingerprint == "" || fingerprint == "." || fingerprint == ".." ||
		strings.ContainsAny(fingerprint, `/\`+"\x00\r\n") {
		return "", errors.New("commit session fingerprint is not a safe filename component")
	}
	tmp := filepath.Join(root, "tmp")
	if err := os.MkdirAll(tmp, 0o750); err != nil {
		return "", fmt.Errorf("create commit session directory: %w", err)
	}
	path := filepath.Join(tmp, "commit-session-id-"+fingerprint)
	if requested != "" {
		requested = strings.ToLower(requested)
		if !commitSessionPattern.MatchString(requested) {
			return "", errors.New("session must be 8 lowercase hexadecimal characters")
		}
		if err := writeCommitSession(path, requested); err != nil {
			return "", fmt.Errorf("write commit session: %w", err)
		}
		return requested, nil
	}
	for {
		content, err := os.ReadFile(path) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if err == nil {
			existing := strings.ToLower(strings.TrimSpace(string(content)))
			if !commitSessionPattern.MatchString(existing) {
				return "", errors.New("stored commit session is not 8 lowercase hexadecimal characters")
			}
			return existing, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read commit session: %w", err)
		}
		var token [4]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", fmt.Errorf("generate commit session: %w", err)
		}
		session := hex.EncodeToString(token[:])
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // the path is this session's commit artifact or a tracked file under the checkout root
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("write commit session: %w", err)
		}
		if _, err := file.WriteString(session + "\n"); err != nil {
			_ = file.Close()
			_ = os.Remove(path)
			return "", fmt.Errorf("write commit session: %w", err)
		}
		if err := file.Close(); err != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("write commit session: %w", err)
		}
		return session, nil
	}
}

func writeCommitSession(path, session string) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".commit-session-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name) //nolint:errcheck // a successful rename consumes it
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.WriteString(session + "\n"); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
