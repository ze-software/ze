// Design: plan/spec-le-is-a-ze-binary.md -- native le support paths
//
// This file resolves the canonical session identity and its checkout-local
// directories without invoking the shell or Python helpers that native le replaces.
package lepath

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

const sessionRoot = "session"

// procRoot is the Linux process filesystem. A host without it falls back to ps.
const procRoot = "/proc"

// SessionPaths is one session's identity and checkout-root-relative paths.
// Dir owns all session state. Scratch is the subdirectory for temporary work.
type SessionPaths struct {
	ID      string
	Dir     string
	Scratch string
}

// ResolveSession resolves the canonical four-source identity and its
// checkout-local paths in-process. The returned paths are relative to root. An
// existing dated directory wins, so a session does not move at midnight.
// Scratch is created only when ensureScratch is true.
//
// A non-empty raw CLAUDE_CODE_SESSION_ID that is unsafe is an error. It MUST NOT
// fall through to another identity source because that would make one process
// adopt state written for another session.
func ResolveSession(root string, ensureScratch bool) (SessionPaths, error) {
	return resolveSession(root, ensureScratch, nativeProcess{})
}

func resolveSession(root string, ensureScratch bool, process sessionProcess) (SessionPaths, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return SessionPaths{}, fmt.Errorf("resolve session checkout %s: %w", root, err)
	}
	info, err := os.Stat(absoluteRoot)
	if err != nil {
		return SessionPaths{}, fmt.Errorf("open session checkout %s: %w", absoluteRoot, err)
	}
	if !info.IsDir() {
		return SessionPaths{}, fmt.Errorf("open session checkout %s: not a directory", absoluteRoot)
	}

	id, err := resolveSessionID(absoluteRoot, process)
	if err != nil {
		return SessionPaths{}, err
	}
	dir, err := resolveSessionDir(absoluteRoot, id)
	if err != nil {
		return SessionPaths{}, err
	}
	paths := SessionPaths{
		ID:      id,
		Dir:     dir,
		Scratch: filepath.Join(dir, "scratch"),
	}
	if !ensureScratch {
		return paths, nil
	}
	// 0o777 is deliberate: every agent account on this host shares one session
	// scratch tree, and a narrower mode locks the second account out of it.
	if err := os.MkdirAll(filepath.Join(absoluteRoot, paths.Scratch), 0o777); err != nil { //nolint:gosec // shared across agent accounts, see above
		return SessionPaths{}, fmt.Errorf("create session scratch %s: %w", paths.Scratch, err)
	}
	return paths, nil
}

// SessionForID resolves paths for a validated hook payload identity without
// consulting ambient process state. It does not create the directory.
func SessionForID(root, id string) (SessionPaths, error) {
	if !safeSessionID(id) {
		return SessionPaths{}, fmt.Errorf("session paths: unsafe session id %q", id)
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return SessionPaths{}, fmt.Errorf("resolve session checkout %s: %w", root, err)
	}
	dir, err := resolveSessionDir(absoluteRoot, id)
	if err != nil {
		return SessionPaths{}, err
	}
	return SessionPaths{ID: id, Dir: dir, Scratch: filepath.Join(dir, "scratch")}, nil
}

func resolveSessionID(root string, process sessionProcess) (string, error) {
	if raw := os.Getenv("CLAUDE_CODE_SESSION_ID"); raw != "" {
		if !safeSessionID(raw) {
			return "", fmt.Errorf("session scratch: unsafe session id %q", raw)
		}
		return raw, nil
	}
	if id := sessionIDFromProcess(process); id != "" {
		return id, nil
	}
	if id := sessionIDFromJWT(os.Getenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN")); id != "" {
		return id, nil
	}
	return mintSessionID(root, process)
}

func safeSessionID(id string) bool {
	if id == "" || id == "." || id == ".." {
		return false
	}
	for index := range len(id) {
		character := id[index]
		if character >= 'a' && character <= 'z' {
			continue
		}
		if character >= 'A' && character <= 'Z' {
			continue
		}
		if character >= '0' && character <= '9' {
			continue
		}
		if character == '.' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

// ValidSessionID reports whether id is safe as one checkout-local filename
// component. Hook payloads use this before copying an identity into a command.
func ValidSessionID(id string) bool {
	return safeSessionID(id)
}

func sessionIDFromProcess(process sessionProcess) string {
	pid := process.PID()
	for range 256 {
		if pid <= 1 {
			return ""
		}
		argv, _ := process.Argv(pid)
		if id := safeSessionIDFromArgv(argv); id != "" {
			return id
		}
		parent, err := process.parentPID(pid)
		if err != nil || parent <= 0 || parent == pid {
			return ""
		}
		pid = parent
	}
	return ""
}

func safeSessionIDFromArgv(argv []string) string {
	for index, argument := range argv {
		if argument == "--session-id" && index+1 < len(argv) {
			if safeSessionID(argv[index+1]) {
				return argv[index+1]
			}
			return ""
		}
		if value, found := strings.CutPrefix(argument, "--session-id="); found {
			if safeSessionID(value) {
				return value
			}
			return ""
		}
	}
	return ""
}

func sessionIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return ""
	}
	var claims struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || !safeSessionID(claims.SessionID) {
		return ""
	}
	return claims.SessionID
}

func mintSessionID(root string, process sessionProcess) (string, error) {
	pid := sessionCLIAncestor(process)
	key := sessionCacheKey(process, pid)
	cacheDir := filepath.Join(root, "tmp", sessionRoot)
	cache := filepath.Join(cacheDir, ".sid-by-pid-"+key)

	existing, err := readCachedSessionID(cache)
	if err == nil && existing != "" {
		now := time.Now()
		_ = os.Chtimes(cache, now, now)
		return existing, nil
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read session identity cache %s: %w", cache, err)
	}
	// 0o777 is deliberate: the identity cache is shared by every agent account.
	if err := os.MkdirAll(cacheDir, 0o777); err != nil { //nolint:gosec // shared across agent accounts, see above
		return "", fmt.Errorf("create session identity cache directory %s: %w", cacheDir, err)
	}
	minted, err := randomSessionID()
	if err != nil {
		return "", fmt.Errorf("mint session identity: %w", err)
	}
	temporary := cache + "." + strconv.Itoa(os.Getpid()) + ".tmp"
	if err := writeSessionID(temporary, minted); err != nil {
		_ = os.Remove(temporary)
		return "", fmt.Errorf("write session identity cache %s: %w", temporary, err)
	}
	if err := os.Rename(temporary, cache); err != nil {
		_ = os.Remove(temporary)
		return "", fmt.Errorf("publish session identity cache %s: %w", cache, err)
	}
	published, err := readCachedSessionID(cache)
	if err != nil {
		return "", fmt.Errorf("read published session identity cache %s: %w", cache, err)
	}
	if published == "" {
		return "", fmt.Errorf("read published session identity cache %s: unsafe identity", cache)
	}
	return published, nil
}

func readCachedSessionID(path string) (string, error) {
	content, err := os.ReadFile(path) //nolint:gosec // path is the identity cache this package composes
	if err != nil {
		return "", err
	}
	line, _, _ := bytes.Cut(content, []byte{'\n'})
	id := strings.TrimSpace(string(line))
	if !safeSessionID(id) {
		return "", nil
	}
	return id, nil
}

func writeSessionID(path, id string) error {
	// 0o666 is deliberate: the identity cache is rewritten by every agent account.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666) //nolint:gosec // shared across agent accounts, see above
	if err != nil {
		return err
	}
	if _, err := file.WriteString(id + "\n"); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func randomSessionID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = value[6]&0x0f | 0x40
	value[8] = value[8]&0x3f | 0x80
	return fmt.Sprintf(
		"%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16],
	), nil
}

func resolveSessionDir(root, id string) (string, error) {
	relativeRoot := filepath.Join("tmp", sessionRoot)
	absoluteSessionRoot := filepath.Join(root, relativeRoot)
	entries, err := os.ReadDir(absoluteSessionRoot)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read session directory %s: %w", absoluteSessionRoot, err)
	}
	pattern := "????-??-??-" + id
	for _, entry := range entries {
		matched, matchErr := filepath.Match(pattern, entry.Name())
		if matchErr != nil {
			return "", fmt.Errorf("match session directory %s: %w", entry.Name(), matchErr)
		}
		if !matched {
			continue
		}
		candidate := filepath.Join(absoluteSessionRoot, entry.Name())
		info, statErr := os.Stat(candidate)
		if statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return "", fmt.Errorf("inspect session directory %s: %w", candidate, statErr)
		}
		if info.IsDir() {
			return filepath.Join(relativeRoot, entry.Name()), nil
		}
	}
	return filepath.Join(relativeRoot, time.Now().Format("2006-01-02")+"-"+id), nil
}

func sessionCacheKey(process sessionProcess, pid int) string {
	start, err := process.Start(pid)
	if err != nil {
		start = ""
	}
	start = pathToken(start)
	if start == "" {
		start = "unknown"
	}
	return strconv.Itoa(pid) + "-" + start
}

func pathToken(value string) string {
	var token strings.Builder
	separator := false
	for _, character := range strings.TrimSpace(value) {
		safe := character >= 'a' && character <= 'z' ||
			character >= 'A' && character <= 'Z' ||
			character >= '0' && character <= '9' ||
			character == '.' || character == '_' || character == '-'
		if safe {
			if separator {
				token.WriteByte('_')
			}
			token.WriteRune(character)
			separator = false
			continue
		}
		if token.Len() > 0 {
			separator = true
		}
	}
	return strings.Trim(token.String(), "_")
}

func sessionCLIAncestor(process sessionProcess) int {
	pid := process.PID()
	top := pid
	for range 256 {
		if pid <= 1 {
			return top
		}
		if processIsCLI(process, pid) {
			return pid
		}
		top = pid
		parent, err := process.parentPID(pid)
		if err != nil || parent <= 0 || parent == pid {
			return top
		}
		pid = parent
	}
	return top
}

func processIsCLI(process sessionProcess, pid int) bool {
	if command, err := process.comm(pid); err == nil && filepath.Base(command) == "claude" {
		return true
	}
	argv, _ := process.Argv(pid)
	for _, argument := range argv {
		if slices.Contains(strings.Split(argument, "/"), "claude") {
			return true
		}
	}
	return false
}

type sessionProcess interface {
	PID() int
	parentPID(int) (int, error)
	Argv(int) ([]string, error)
	comm(int) (string, error)
	Start(int) (string, error)
}

type nativeProcess struct{}

func (nativeProcess) PID() int { return os.Getpid() }

func (nativeProcess) ps(pid int, field string) (string, error) {
	//nolint:gosec // pid is a numeric process identifier and field is a package constant
	output, err := exec.CommandContext(context.Background(), "ps", "-o", field, "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (nativeProcess) parentPID(pid int) (int, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "status"))
	if err != nil {
		parent, psErr := (nativeProcess{}).ps(pid, "ppid=")
		if psErr != nil {
			return 0, psErr
		}
		return strconv.Atoi(parent)
	}
	for line := range bytes.SplitSeq(content, []byte{'\n'}) {
		if !bytes.HasPrefix(line, []byte("PPid:")) {
			continue
		}
		fields := bytes.Fields(line)
		if len(fields) != 2 {
			return 0, fmt.Errorf("malformed PPid line %q", line)
		}
		return strconv.Atoi(string(fields[1]))
	}
	return 0, errors.New("PPid is absent")
}

func (nativeProcess) Argv(pid int) ([]string, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "cmdline"))
	if err != nil {
		command, psErr := (nativeProcess{}).ps(pid, "command=")
		if psErr != nil {
			return nil, psErr
		}
		return strings.Fields(command), nil
	}
	parts := bytes.Split(content, []byte{0})
	argv := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) != 0 {
			argv = append(argv, string(part))
		}
	}
	return argv, nil
}

func (nativeProcess) comm(pid int) (string, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "comm"))
	if err != nil {
		return (nativeProcess{}).ps(pid, "comm=")
	}
	return strings.TrimSpace(string(content)), nil
}

func (nativeProcess) Start(pid int) (string, error) {
	content, err := os.ReadFile(filepath.Join(procRoot, strconv.Itoa(pid), "stat"))
	if err != nil {
		return (nativeProcess{}).ps(pid, "lstart=")
	}
	closing := bytes.LastIndexByte(content, ')')
	if closing < 0 {
		return "", errors.New("process command has no closing parenthesis")
	}
	fields := bytes.Fields(content[closing+1:])
	if len(fields) <= 19 {
		return "", errors.New("process stat has no start time")
	}
	return string(fields[19]), nil
}
