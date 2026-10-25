// Design: docs/contributing/running-commands.md -- session identity, and what each agent owns
//
// The eight-hex namespace names every artifact one session prepares: its commit
// script, its commit message and its owner approval files. It lives here,
// beside ResolveSession, because the commit gate and the pre-write hook both
// derive a path from it and neither may import the other.
//
// A subagent shares its parent's harness session id, so the namespace is keyed
// on the AGENT as well: tmp/commit-session-id-<session>+<agent>, where a main
// thread keeps tmp/commit-session-id-<session>. An owner approval is keyed on
// the main thread's namespace (SessionCommitSession), because Thomas answers the
// main thread.
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

	"github.com/ze-software/ze/internal/core/env"
)

var commitSessionPattern = regexp.MustCompile(`^[0-9a-f]{8}$`)

// ErrCommitSessionSpelling is the refusal of a requested commit session that
// is not eight lowercase hexadecimal characters: the caller's keyword was
// wrong, which a caller answers apart from a directory it could not write.
var ErrCommitSessionSpelling = errors.New("session must be 8 lowercase hexadecimal characters")

// ValidCommitSession reports whether namespace has the shape of a commit
// namespace, for a caller handed one from outside (a hook payload on stdin)
// before it names a file after it.
func ValidCommitSession(namespace string) bool {
	return commitSessionPattern.MatchString(namespace)
}

// AgentKey is the dot-notation spelling of ZE_AGENT_ID, the subagent identity
// the pretool Bash hook exports from its payload's agent_id
// (internal/le/hookruntime/session.go, writeParentSessionPrefix). A main
// thread has none.
const AgentKey = "ze.agent.id"

var agentEntry = env.MustRegister(env.EnvEntry{
	Key:         AgentKey,
	Type:        "string",
	Default:     "",
	Description: "the harness subagent this le process runs for; empty in a main thread",
	// Private for the reason rootEntry is: it is le's variable, not an
	// operator's.
	Private: true,
})

// agentSeparator joins a harness session id to one of its agents in every
// per-agent file name. safeSessionID admits neither it nor a glob
// metacharacter in a session id or an agent id, so the joined name splits one
// way only: no session can pose as another session's subagent, and the glob
// <session>+* matches that session's agents and no other session's files.
const agentSeparator = "+"

// commitSessionPrefix starts every commit-namespace file under tmp/.
const commitSessionPrefix = "commit-session-id-"

// Agent answers the subagent identity of this process, or "" for a main
// thread. An unsafe value is an error rather than "": falling back to the
// parent's identity is the sharing this exists to end.
func Agent() (string, error) {
	agent := env.Get(agentEntry.Key)
	if agent == "" {
		return "", nil
	}
	if !ValidAgentID(agent) {
		return "", fmt.Errorf("agent identity: unsafe agent id %q", agent)
	}
	return agent, nil
}

// ValidAgentID reports whether agent can name a per-agent file. It is the one
// test every consumer of an agent id applies (the identity files here and the
// hook's call budget), so an id the identity accepts is also an id the budget
// counts: an agent id obeys the same character set as a session id.
func ValidAgentID(agent string) bool {
	return safeSessionID(agent)
}

// AgentFingerprint is the identity one harness session and one of its agents
// key their per-agent files on. An empty agent is the session itself.
func AgentFingerprint(session, agent string) (string, error) {
	if !safeSessionID(session) {
		return "", fmt.Errorf("agent identity: unsafe session id %q", session)
	}
	if agent == "" {
		return session, nil
	}
	if !ValidAgentID(agent) {
		return "", fmt.Errorf("agent identity: unsafe agent id %q", agent)
	}
	return session + agentSeparator + agent, nil
}

// CommitSession returns the reusable eight-hex commit namespace for the
// canonical native harness session and, in a subagent, for that agent. A
// requested value replaces the stored value.
func CommitSession(root, requested string) (string, error) {
	session, err := ResolveSession(root, false)
	if err != nil {
		return "", err
	}
	agent, err := Agent()
	if err != nil {
		return "", err
	}
	fingerprint, err := AgentFingerprint(session.ID, agent)
	if err != nil {
		return "", err
	}
	return commitSessionFor(root, requested, fingerprint)
}

// SessionCommitSession answers the commit namespace of the harness session
// itself, the main thread's, whichever agent this process runs for. An
// owner's RFC approval is keyed on it rather than on an agent's: the owner
// speaks to the main thread, and the approval holds for each agent it spawns
// to make the edit and for the main thread's commit that carries it.
func SessionCommitSession(root string) (string, error) {
	session, err := ResolveSession(root, false)
	if err != nil {
		return "", err
	}
	return CommitSessionFor(root, session.ID, "")
}

// CommitSessionFor maps one KNOWN harness identity, and the agent inside it
// ("" for the main thread), to its commit namespace, for a caller that was
// handed the identity rather than running under it. A hook payload names the
// session and agent whose edit it judges, and the namespace it resolves has
// to be the one that agent's commit artifacts, or its main thread's owner
// approvals with agent "", are named after.
func CommitSessionFor(root, harness, agent string) (string, error) {
	fingerprint, err := AgentFingerprint(harness, agent)
	if err != nil {
		return "", err
	}
	return commitSessionFor(root, "", fingerprint)
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
	path := filepath.Join(tmp, commitSessionPrefix+fingerprint)
	if requested != "" {
		requested = strings.ToLower(requested)
		if !commitSessionPattern.MatchString(requested) {
			return "", ErrCommitSessionSpelling
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
