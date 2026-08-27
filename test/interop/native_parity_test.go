package interop_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	interopbgp "github.com/ze-software/ze/internal/le/interoplab/bgp"
)

// TestNativeInteropScenarioParity derives the Python producer population and
// refuses a missing, extra, reordered, nil, or non-fail-closed Go checker.
func TestNativeInteropScenarioParity(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	scenariosRoot := filepath.Join(root, "test", "interop", "scenarios")
	entries, err := os.ReadDir(scenariosRoot)
	if err != nil {
		t.Fatal(err)
	}
	audit := interopbgp.Audit()
	producer := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		checkerSource := filepath.Join(scenariosRoot, entry.Name(), "check.py")
		data, readErr := os.ReadFile(checkerSource)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			t.Fatal(readErr)
		}
		if !strings.Contains(string(data), "def check(") {
			t.Errorf("Python producer %s has no check()", entry.Name())
		}
		if _, statErr := os.Stat(filepath.Join(scenariosRoot, entry.Name(), "ze.conf")); statErr != nil {
			t.Errorf("Python producer %s has no ze.conf: %v", entry.Name(), statErr)
		}
		auditEntry, audited := audit[entry.Name()]
		if !audited {
			t.Errorf("Python producer %s has no native checker audit row", entry.Name())
		} else {
			digest := sha256.Sum256(data)
			if got := hex.EncodeToString(digest[:]); got != auditEntry.SourceSHA256 {
				t.Errorf("Python checker %s changed without a native assertion/control-flow audit: got %s, audited %s", entry.Name(), got, auditEntry.SourceSHA256)
			}
			if auditEntry.SourceName != entry.Name()+"/check.py" {
				t.Errorf("native checker audit %s names source %q", entry.Name(), auditEntry.SourceName)
			}
			if auditEntry.Assertions+auditEntry.ControlOperations == 0 || auditEntry.NativeProducer == "" {
				t.Errorf("native checker audit %s maps no Python obligations: %+v", entry.Name(), auditEntry)
			}
			if got := pythonAssertionCount(data); got != auditEntry.Assertions {
				t.Errorf("Python checker %s has %d assertion/raise branches, audited %d", entry.Name(), got, auditEntry.Assertions)
			}
			if auditEntry.GapStatus != "zero-gap" || len(auditEntry.NativeOperations) == 0 || len(auditEntry.NativeTests) == 0 {
				t.Errorf("native checker audit %s is incomplete: %+v", entry.Name(), auditEntry)
			}
			nativeContract := strings.Join(auditEntry.NativeOperations, "\n")
			for _, class := range pythonControlClasses(data) {
				if !strings.Contains(nativeContract, "class:"+class) {
					t.Errorf("Python checker %s uses unmapped %s control/operation class", entry.Name(), class)
				}
			}
		}
		producer = append(producer, entry.Name())
	}
	sort.Strings(producer)

	if len(audit) != len(producer) {
		t.Errorf("native audit has %d rows, Python producer has %d scenarios", len(audit), len(producer))
	}
	genericWant, specialWant := interopbgp.NativeAuditDigests()
	genericGot := hashNativeFiles(t, root,
		"internal/le/interoplab/bgp/checkers.go",
		"internal/le/interoplab/bgp/check_extras.go",
		"internal/le/interoplab/bgp/check_engine.go",
	)
	specialGot := hashNativeFiles(t, root,
		"internal/le/interoplab/bgp/check_special.go",
		"internal/le/interoplab/bgp/isis_inject.go",
		"internal/le/interoplab/bgp/isis_inject_linux.go",
		"internal/le/interoplab/bgp/isis_inject_other.go",
	)
	if genericGot != genericWant || specialGot != specialWant {
		t.Errorf("native checker operations changed without refreshing the zero-gap audit: generic=%s want=%s special=%s want=%s", genericGot, genericWant, specialGot, specialWant)
	}
	checkers := interopbgp.Checkers()
	native := make([]string, 0, len(checkers))
	for name, checker := range checkers {
		native = append(native, name)
		if checker == nil {
			t.Errorf("native checker %s is nil", name)
			continue
		}
		check := &interoplab.CheckContext{
			Source: interoplab.ScenarioSource{Name: name, Directory: filepath.Join(scenariosRoot, name)},
			Lab:    noOutputLab{},
		}
		checkCtx, cancel := context.WithTimeout(t.Context(), time.Millisecond)
		checkErr := checker(checkCtx, check)
		cancel()
		if checkErr == nil {
			t.Errorf("native checker %s passed without reading peer output", name)
		}
	}
	sort.Strings(native)
	if strings.Join(native, "\n") != strings.Join(producer, "\n") {
		t.Fatalf("scenario names differ\nPython: %v\nGo: %v", producer, native)
	}

	for _, name := range producer {
		selected, selectErr := interoplab.Discover(scenariosRoot, name, checkers)
		if selectErr != nil {
			t.Errorf("select %s: %v", name, selectErr)
			continue
		}
		if len(selected) != 1 || selected[0].Name != name {
			t.Errorf("select %s returned %+v", name, selected)
		}
	}
	if _, err := interoplab.Discover(scenariosRoot, "not-a-scenario", checkers); err == nil {
		t.Fatal("unknown exact selector did not fail")
	}
}

func hashNativeFiles(t *testing.T, root string, paths ...string) string {
	t.Helper()
	digest := sha256.New()
	for _, path := range paths {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := digest.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func pythonAssertionCount(source []byte) int {
	code := string(stripPythonStringsAndComments(source))
	count := 0
	for line := range strings.SplitSeq(code, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "assert ") {
			count++
			continue
		}
		if strings.HasPrefix(trimmed, "raise ") && strings.TrimSpace(strings.TrimPrefix(trimmed, "raise ")) != "" {
			count++
		}
	}
	return count
}

func pythonControlClasses(source []byte) []string {
	code := string(stripPythonStringsAndComments(source))
	classes := make(map[string]struct{})
	add := func(name string) { classes[name] = struct{}{} }
	for line := range strings.SplitSeq(code, "\n") {
		trimmed := strings.TrimSpace(line)
		assertion := strings.HasPrefix(trimmed, "assert ") ||
			strings.HasPrefix(trimmed, "raise AssertionError")
		if assertion {
			add("assertion")
			if strings.Contains(trimmed, " not ") || strings.Contains(trimmed, " is None") {
				add("negative")
			}
		}
		switch {
		case strings.HasPrefix(trimmed, "if "), strings.HasPrefix(trimmed, "elif "),
			trimmed == "else:", trimmed == "try:", strings.HasPrefix(trimmed, "except "),
			trimmed == "finally:":
			add("branch")
		}
		if strings.HasPrefix(trimmed, "for ") || strings.HasPrefix(trimmed, "while ") {
			add("loop")
		}
	}
	for _, item := range []struct {
		class string
		terms []string
	}{
		{"timing", []string{"time.", "poll(", ".wait_", "wait_for("}},
		{"cleanup", []string{"finally:", "_thaw(", "restore_", ".unpause("}},
		{"mutation", []string{"inject_route", "inject_flowspec", "break_link", ".stop(", ".start(", ".signal(", ".pause(", "redistribute(", "clear_route", "restart("}},
		{"negative", []string{"wait_absent", "check_absent", "assert_absent", "not_present", "no_route"}},
		{"concurrency", []string{"threading.", "Thread(", "Lock(", "Event("}},
		{"observer", []string{"raise_if_observer_failed"}},
		{"byte-agreement", []string{"bytes.fromhex", "decode_update", "int.from_bytes", ".hex()"}},
		{"session", []string{"wait_session", "session_established", "peer_state(", "wait_peer_established"}},
		{"route", []string{"wait_route", "check_route", "rib_", "route_", ".route", "routes("}},
		{"capability", []string{"capabil", "addpath", "add_path", "afi_safi", "family"}},
		{"adjacency", []string{"adjacency"}},
		{"query", []string{".cli(", ".cli_json(", "_vtysh", "_birdc", "docker_logs", "docker_exec", "session_established", "adjacency_up"}},
	} {
		for _, term := range item.terms {
			if strings.Contains(code, term) {
				add(item.class)
				break
			}
		}
	}
	result := make([]string, 0, len(classes))
	for class := range classes {
		result = append(result, class)
	}
	sort.Strings(result)
	return result
}

func stripPythonStringsAndComments(source []byte) []byte {
	result := append([]byte(nil), source...)
	var quote byte
	triple := false
	escaped := false
	for index := 0; index < len(result); {
		current := result[index]
		if quote != 0 {
			if triple && index+2 < len(result) &&
				result[index] == quote && result[index+1] == quote && result[index+2] == quote {
				result[index], result[index+1], result[index+2] = ' ', ' ', ' '
				index += 3
				quote = 0
				triple = false
				continue
			}
			if !triple && !escaped && current == quote {
				result[index] = ' '
				index++
				quote = 0
				continue
			}
			if current == '\n' {
				if !triple {
					quote = 0
				}
				escaped = false
				index++
				continue
			}
			escaped = !escaped && current == '\\'
			if current != '\\' {
				escaped = false
			}
			result[index] = ' '
			index++
			continue
		}
		if current == '#' {
			for index < len(result) && result[index] != '\n' {
				result[index] = ' '
				index++
			}
			continue
		}
		if current == '\'' || current == '"' {
			quote = current
			if index+2 < len(result) && result[index+1] == quote && result[index+2] == quote {
				triple = true
				result[index], result[index+1], result[index+2] = ' ', ' ', ' '
				index += 3
				continue
			}
			result[index] = ' '
			index++
			continue
		}
		index++
	}
	return result
}

type noOutputLab struct{}

func (noOutputLab) PeerPID(context.Context, string) (int, error) {
	return 0, errors.New("peer PID was not read")
}
func (noOutputLab) Exec(context.Context, string, []string, []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	return interoplab.CommandResult{}, errors.New("peer produced no output")
}
func (noOutputLab) ExecDetached(context.Context, string, []string, []interoplab.EnvironmentVariable) error {
	return errors.New("peer produced no output")
}
func (noOutputLab) Query(context.Context, string, []string, []interoplab.EnvironmentVariable) (string, error) {
	return "", errors.New("peer produced no output")
}
func (noOutputLab) Logs(context.Context, string, int) (interoplab.LogResult, error) {
	return interoplab.LogResult{}, errors.New("peer logs were not read")
}
func (noOutputLab) Signal(context.Context, string, string) error { return nil }
func (noOutputLab) Pause(context.Context, string) error          { return nil }
func (noOutputLab) Unpause(context.Context, string) error        { return nil }
func (noOutputLab) Start(context.Context, string) error          { return nil }
func (noOutputLab) Stop(context.Context, string, int) error      { return nil }

var _ interoplab.CheckerLab = noOutputLab{}
