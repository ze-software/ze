package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const apiUserRemovedByReloadName = "ui/api-user-removed-by-reload"

func init() {
	Register(apiUserRemovedByReloadName, uiDriver(runAPIUserRemovedByReload))
}

type apiUserReloadFixture struct {
	ctx        context.Context
	port1      int
	port2      int
	baseEnv    []string
	work       string
	tempDirs   []string
	daemon     *exec.Cmd
	daemonDone chan error
}

func runAPIUserRemovedByReload(ctx context.Context) error {
	port1, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	port2, err := uiFreeTCPPort()
	if err != nil {
		return err
	}
	f := &apiUserReloadFixture{
		ctx:   ctx,
		port1: port1,
		port2: port2,
	}
	f.baseEnv = uiApiUserRemovedByReloadReplaceEnv(os.Environ(), map[string]string{
		"ZE_REST_PORT":  strconv.Itoa(f.port1),
		"ZE_REST_PORT2": strconv.Itoa(f.port2),
	})
	workspace, err := f.makeTempDir()
	if err != nil {
		return err
	}
	f.work = workspace
	defer f.cleanup()

	distinctDir, err := f.makeTempDir()
	if err != nil {
		return err
	}
	collisionDir, err := f.makeTempDir()
	if err != nil {
		return err
	}
	reloadDir, err := f.makeTempDir()
	if err != nil {
		return err
	}
	configOnlyDir, err := f.makeTempDir()
	if err != nil {
		return err
	}

	configHash, err := f.passwordHash("configpass")
	if err != nil {
		return err
	}
	collisionHash, err := f.passwordHash("collisionpass")
	if err != nil {
		return err
	}
	bootHash, err := f.passwordHash("bootpass")
	if err != nil {
		return err
	}
	reloadHash, err := f.passwordHash("reloadpass")
	if err != nil {
		return err
	}
	keepHash, err := f.passwordHash("keeppass")
	if err != nil {
		return err
	}
	newHash, err := f.passwordHash("newpass")
	if err != nil {
		return err
	}

	apiOnPort1 := fmt.Sprintf(`api-server {
	rest {
		enabled true
		server main { ip 127.0.0.1; port %d; }
	}
}
`, f.port1)
	apiOnPort2 := fmt.Sprintf(`api-server {
	rest {
		enabled true
		server main { ip 127.0.0.1; port %d; }
	}
}
`, f.port2)
	if err := os.WriteFile(f.path("api-on-port1.conf"), []byte(apiOnPort1), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(f.path("api-on-port2.conf"), []byte(apiOnPort2), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(f.path("api-absent.conf"), nil, 0o600); err != nil {
		return err
	}

	configOnlyUsers := userEntry("configuser", configHash)
	distinctUsers := strings.Join([]string{
		userEntry("configuser", configHash),
		userEntry("bootuser", bootHash),
		userEntry("reloaduser", reloadHash),
		userEntry("keepuser", keepHash),
	}, "")
	collisionUsers := strings.Join([]string{
		userEntry("poweruser", collisionHash),
		userEntry("bootuser", bootHash),
		userEntry("reloaduser", reloadHash),
		userEntry("keepuser", keepHash),
	}, "")
	noBootUsers := strings.Join([]string{
		userEntry("poweruser", collisionHash),
		userEntry("reloaduser", reloadHash),
		userEntryProfile("keepuser", keepHash, "api-denied"),
		userEntry("newuser", newHash),
	}, "")
	keepOnlyUsers := userEntry("poweruser", collisionHash) +
		userEntryProfile("keepuser", keepHash, "api-denied") +
		userEntry("newuser", newHash)

	for name, users := range map[string]string{
		"users-config-only.conf": configOnlyUsers,
		"users-distinct.conf":    distinctUsers,
		"users-collision.conf":   collisionUsers,
		"users-no-boot.conf":     noBootUsers,
		"users-keep-only.conf":   keepOnlyUsers,
	} {
		if err := writeSystem(f.path(name), users); err != nil {
			return err
		}
	}

	// Boot 0: config-only REST and strict AAA, with no zefs database.
	if err := writeCombinedConfig(f.path("api-user-reload.conf"), f.path("users-config-only.conf"), f.path("api-on-port1.conf")); err != nil {
		return err
	}
	if err := f.startDaemon("config-only", "config-only.log", configOnlyDir); err != nil {
		return err
	}
	if err := requireLogText(f.path("config-only.log"), "API auth mode: per-user (1 users)",
		"config-only boot did not publish the API auth profile"); err != nil {
		return err
	}
	if err := f.stopDaemon(); err != nil {
		return err
	}

	// Seed the real zefs producer for the distinct-name boot.
	if err := f.initZE(distinctDir); err != nil {
		return err
	}

	// Boot 1: actual runYANGConfig merge with distinct zefs/config names.
	if err := writeCombinedConfig(f.path("api-user-reload.conf"), f.path("users-distinct.conf"), f.path("api-on-port1.conf")); err != nil {
		return err
	}
	if err := f.startDaemon("distinct", "distinct.log", distinctDir); err != nil {
		return err
	}
	if err := requireLogText(f.path("distinct.log"), "API auth mode: per-user (5 users)",
		"distinct boot did not publish the merged API profile"); err != nil {
		return err
	}
	if err := f.stopDaemon(); err != nil {
		return err
	}

	// Boot 2 uses a fresh zefs database so its active config pointer cannot
	// override the collision fixture written for this boot.
	if err := f.initZE(collisionDir); err != nil {
		return err
	}
	if err := writeCombinedConfig(f.path("api-user-reload.conf"), f.path("users-collision.conf"), f.path("api-on-port1.conf")); err != nil {
		return err
	}
	if err := f.startDaemon("collision", "collision.log", collisionDir); err != nil {
		return err
	}
	if err := requireLogText(f.path("collision.log"), "API auth mode: per-user (4 users)",
		"collision boot did not publish the deduplicated API profile"); err != nil {
		return err
	}
	if err := f.stopDaemon(); err != nil {
		return err
	}

	// Boot 3 uses a fresh filesystem-backed config source for live reloads.
	if err := writeCombinedConfig(f.path("api-user-reload.conf"), f.path("users-distinct.conf"), f.path("api-on-port1.conf")); err != nil {
		return err
	}
	if err := f.startDaemon("reload-boot", "reload.log", reloadDir); err != nil {
		return err
	}

	// Reload 1 removes bootuser, adds newuser, changes keepuser to the denying
	// profile, and removes api-server.
	if err := writeCombinedConfig(f.path("api-user-reload.conf"), f.path("users-no-boot.conf"), f.path("api-absent.conf")); err != nil {
		return err
	}
	if err := f.reloadAndCheck("boot-site"); err != nil {
		return err
	}

	// Reload 2 restores api-server on another port. The move proves UpdateAuth ran.
	if err := writeCombinedConfig(f.path("api-user-reload.conf"), f.path("users-no-boot.conf"), f.path("api-on-port2.conf")); err != nil {
		return err
	}
	if err := f.reloadAndCheck("rebuilt"); err != nil {
		return err
	}

	// Reload 3 removes reloaduser while leaving api-server absent.
	if err := writeCombinedConfig(f.path("api-user-reload.conf"), f.path("users-keep-only.conf"), f.path("api-absent.conf")); err != nil {
		return err
	}
	if err := f.reloadAndCheck("reload-site"); err != nil {
		return err
	}

	if err := f.stopDaemon(); err != nil {
		return err
	}
	return nil
}

func (f *apiUserReloadFixture) makeTempDir() (string, error) {
	dir, err := os.MkdirTemp("", "api-user-reload-")
	if err != nil {
		return "", err
	}
	f.tempDirs = append(f.tempDirs, dir)
	return dir, nil
}

func (f *apiUserReloadFixture) path(name string) string {
	return filepath.Join(f.work, name)
}

func (f *apiUserReloadFixture) cleanup() {
	_ = f.stopDaemon()
	for _, dir := range f.tempDirs {
		_ = os.RemoveAll(dir)
	}
}

func (f *apiUserReloadFixture) passwordHash(password string) (string, error) {
	cmd := exec.CommandContext(f.ctx, "ze", "passwd")
	cmd.Env = f.baseEnv
	cmd.Stdin = strings.NewReader(password + "\n")
	cmd.Stderr = os.Stderr
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ze passwd: %w", err)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

func (f *apiUserReloadFixture) initZE(configDir string) error {
	cmd := exec.CommandContext(f.ctx, "ze", "init")
	cmd.Env = uiApiUserRemovedByReloadReplaceEnv(f.baseEnv, map[string]string{envConfigDir: configDir})
	cmd.Stdin = strings.NewReader("poweruser\npowerpass\n127.0.0.1\n2222\n")
	cmd.Stdout = io.Discard
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ze init: %w", err)
	}
	return nil
}

func userEntry(name, hash string) string {
	return userEntryProfile(name, hash, "api-limited")
}

func userEntryProfile(name, hash, profile string) string {
	return fmt.Sprintf("\t\tuser %s { password %q; profile [ %s ]; }\n", name, hash, profile)
}

func writeSystem(path, users string) error {
	contents := fmt.Sprintf(`system {
	authentication {
%s	}
	authorization {
		profile api-limited {
			run {
				default-action deny
				entry 10 { action allow; match "show version"; }
			}
			edit { default-action deny; }
		}
		profile api-denied {
			run { default-action deny; }
			edit { default-action deny; }
		}
	}
}
`, users)
	return os.WriteFile(path, []byte(contents), 0o600)
}

func writeCombinedConfig(outputPath, systemPath, environmentPath string) error {
	systemConfig, err := os.ReadFile(systemPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	environmentConfig, err := os.ReadFile(environmentPath) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	combined := append([]byte(nil), systemConfig...)
	if len(environmentConfig) != 0 {
		combined = append(combined, []byte("\nenvironment {\n")...)
		combined = append(combined, environmentConfig...)
		combined = append(combined, []byte("}\n")...)
	}
	return os.WriteFile(outputPath, combined, 0o600)
}

func (f *apiUserReloadFixture) startDaemon(stage, logName, configDir string) error {
	readyPath := f.path("daemon.ready")
	_ = os.Remove(readyPath)
	logPath := f.path(logName)
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(f.ctx, "ze", "start", f.path("api-user-reload.conf")) //nolint:gosec // the fixture chooses the program and its arguments
	cmd.Dir = f.work
	cmd.Env = uiApiUserRemovedByReloadReplaceEnv(f.baseEnv, map[string]string{
		envReadyFile: readyPath,
		envConfigDir: configDir,
	})
	cmd.Stdout = os.Stdout
	cmd.Stderr = log
	if err := cmd.Start(); err != nil {
		_ = log.Close()
		return err
	}
	_ = log.Close()
	f.daemon = cmd
	f.daemonDone = make(chan error, 1)
	go func() { f.daemonDone <- cmd.Wait() }()

	if err := f.checkStage(stage); err != nil {
		fmt.Fprintf(os.Stderr, "--- %s ---\n", logPath)
		writeFileToStderr(logPath)
		return err
	}

	ready := false
	for range 300 {
		if uiApiUserRemovedByReloadFileExists(readyPath) {
			ready = true
			break
		}
		if !uiApiUserRemovedByReloadSleepContext(f.ctx, 100*time.Millisecond) {
			return f.ctx.Err()
		}
	}
	if !ready {
		ready = uiApiUserRemovedByReloadFileExists(readyPath)
	}
	if !ready {
		fmt.Fprintln(os.Stderr, "FAIL: daemon never became ready")
		writeFileToStderr(logPath)
		return errors.New("daemon never became ready")
	}
	return nil
}
func (f *apiUserReloadFixture) stopDaemon() error {
	if f.daemon == nil {
		return nil
	}
	_ = f.daemon.Process.Signal(syscall.SIGTERM)
	timer := time.NewTimer(5 * time.Second)
	select {
	case <-f.daemonDone:
		timer.Stop()
	case <-timer.C:
		_ = f.daemon.Process.Kill()
		killTimer := time.NewTimer(5 * time.Second)
		select {
		case <-f.daemonDone:
			killTimer.Stop()
		case <-killTimer.C:
			return errors.New("daemon did not exit after being killed")
		}
	}
	f.daemon = nil
	f.daemonDone = nil
	return nil
}

func (f *apiUserReloadFixture) reloadAndCheck(stage string) error {
	if f.daemon == nil {
		return errors.New("reload requested without a running daemon")
	}
	if err := f.daemon.Process.Signal(syscall.SIGHUP); err != nil {
		return err
	}
	if err := f.checkStage(stage); err != nil {
		fmt.Fprintln(os.Stderr, "--- reload.log ---")
		writeTailToStderr(f.path("reload.log"), 10)
		return err
	}
	return nil
}

func (f *apiUserReloadFixture) checkStage(stage string) error {
	if generation, ok := map[string]int{
		"boot-site":   1,
		"rebuilt":     2,
		"reload-site": 3,
	}[stage]; ok {
		if err := uiApiUserRemovedByReloadRequire(uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return uiApiUserRemovedByReloadReloadGeneration(f.path("reload.log")) >= generation
		}), fmt.Sprintf("reload generation %d completed", generation)); err != nil {
			return err
		}
	}

	switch stage {
	case "config-only":
		if err := uiApiUserRemovedByReloadRequire(uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return f.requestStatus(f.port1, "configuser", "configpass", "") == http.StatusOK
		}), "a config user authenticates with no zefs, BGP, or SSH"); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "configuser", "wrongpass", "") == http.StatusUnauthorized,
			"a wrong config-user password is refused"); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "configuser", "configpass", "show version") == http.StatusOK,
			"the config user can run the command their profile allows"); err != nil {
			return err
		}
		deniedStatus := f.requestStatus(f.port1, "configuser", "configpass", "show system goroutines summary")
		return uiApiUserRemovedByReloadRequire(deniedStatus == http.StatusForbidden,
			fmt.Sprintf("the config user is denied a command outside their profile (status %s)", statusText(deniedStatus)))

	case "distinct":
		if err := uiApiUserRemovedByReloadRequire(uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return f.requestStatus(f.port1, "configuser", "configpass", "") == http.StatusOK
		}), "the distinct config user authenticates beside the zefs user"); err != nil {
			return err
		}
		return uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "poweruser", "powerpass", "show system goroutines summary") == http.StatusOK,
			"the distinct zefs user executes through the recovery profile")

	case "collision":
		collisionReady := uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return f.requestStatus(f.port1, "poweruser", "collisionpass", "") == http.StatusOK
		})
		collisionStatus := f.requestStatus(f.port1, "poweruser", "collisionpass", "")
		if err := uiApiUserRemovedByReloadRequire(collisionReady,
			fmt.Sprintf("the colliding config credential wins at boot (status %s)", statusText(collisionStatus))); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "poweruser", "powerpass", "") == http.StatusUnauthorized,
			"the colliding zefs credential loses at boot"); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "poweruser", "collisionpass", "show version") == http.StatusOK,
			"the colliding config user keeps their assigned allow rule"); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "poweruser", "collisionpass", "show system goroutines summary") == http.StatusForbidden,
			"the colliding config user does not inherit zefs recovery authority"); err != nil {
			return err
		}
		return uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "bootuser", "bootpass", "") == http.StatusOK,
			"the boot-built authenticator admits an ordinary config user")

	case "reload-boot":
		if err := uiApiUserRemovedByReloadRequire(uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return f.requestStatus(f.port1, "bootuser", "bootpass", "") == http.StatusOK
		}), "the reload daemon starts from its config user source"); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "reloaduser", "reloadpass", "") == http.StatusOK,
			"the reload daemon admits the user that later reloads remove"); err != nil {
			return err
		}
		return uiApiUserRemovedByReloadRequire(f.requestStatus(f.port1, "keepuser", "keeppass", "show version") == http.StatusOK,
			"the unchanged user can run the probe command before profile reload")

	case "boot-site":
		if err := uiApiUserRemovedByReloadRequire(uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return f.requestStatus(f.port1, "bootuser", "bootpass", "") == http.StatusUnauthorized
		}), "the boot-built authenticator refuses a removed user"); err != nil {
			return err
		}
		checks := []struct {
			ok  bool
			msg string
		}{
			{f.requestStatus(f.port1, "newuser", "newpass", "") == http.StatusOK, "the boot-built authenticator sees a user added through the live provider"},
			{f.requestStatus(f.port1, "reloaduser", "reloadpass", "") == http.StatusOK, "an unchanged user still authenticates after reload"},
			{f.requestStatus(f.port1, "keepuser", "keeppass", "") == http.StatusOK, "the unchanged user still authenticates after their profile reload"},
			{f.requestStatus(f.port1, "keepuser", "keeppass", "show version") == http.StatusForbidden, "the unchanged user loses the probe command after profile reload"},
			{f.requestStatus(f.port1, "poweruser", "collisionpass", "") == http.StatusOK, "the live provider admits a config user added by reload"},
			{f.requestStatus(f.port1, "poweruser", "powerpass", "") == http.StatusUnauthorized, "the config-only reload does not gain a zefs credential"},
		}
		for _, check := range checks {
			if err := uiApiUserRemovedByReloadRequire(check.ok, check.msg); err != nil {
				return err
			}
		}
		return nil

	case "rebuilt":
		rebuiltReady := uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return f.requestStatus(f.port2, "newuser", "newpass", "") == http.StatusOK
		})
		port2Status := f.requestStatus(f.port2, "newuser", "newpass", "")
		port1Status := f.requestStatus(f.port1, "newuser", "newpass", "")
		if err := uiApiUserRemovedByReloadRequire(rebuiltReady, fmt.Sprintf(
			"the moved listener proves the API authenticator was rebuilt from live users (new status %s, old status %s)",
			statusText(port2Status), statusText(port1Status))); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port2, "reloaduser", "reloadpass", "") == http.StatusOK,
			"the rebuilt authenticator admits the current config user"); err != nil {
			return err
		}
		return uiApiUserRemovedByReloadRequire(f.requestStatus(f.port2, "bootuser", "bootpass", "") == http.StatusUnauthorized,
			"the rebuilt authenticator does not revive the removed user")

	case "reload-site":
		if err := uiApiUserRemovedByReloadRequire(uiApiUserRemovedByReloadPoll(f.ctx, func() bool {
			return f.requestStatus(f.port2, "reloaduser", "reloadpass", "") == http.StatusUnauthorized
		}), "the reload-built authenticator refuses a later removed user"); err != nil {
			return err
		}
		if err := uiApiUserRemovedByReloadRequire(f.requestStatus(f.port2, "keepuser", "keeppass", "") == http.StatusOK,
			"an unchanged user remains admitted by the reload-built authenticator"); err != nil {
			return err
		}
		return uiApiUserRemovedByReloadRequire(f.requestStatus(f.port2, "newuser", "newpass", "") == http.StatusOK,
			"the user added by reload remains admitted")
	default:
		return fmt.Errorf("unknown stage %q", stage)
	}
}

func (f *apiUserReloadFixture) requestStatus(port int, user, password, command string) int {
	path := "/api/v1/commands"
	var body io.Reader
	if command != "" {
		path = "/api/v1/execute"
		payload, err := json.Marshal(map[string]string{fieldCommand: command})
		if err != nil {
			return 0
		}
		body = bytes.NewReader(payload)
	}
	requestCtx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet,
		fmt.Sprintf("http://127.0.0.1:%d%s", port, path), body)
	if err != nil {
		return 0
	}
	req.Header.Set("Authorization", "Bearer "+user+":"+password)
	if command != "" {
		req.Method = http.MethodPost
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return resp.StatusCode
}

// uiApiUserRemovedByReloadAttempts bounds every poll in this fixture.
const uiApiUserRemovedByReloadAttempts = 60

// uiApiUserRemovedByReloadDelay is the pause between two poll attempts.
const uiApiUserRemovedByReloadDelay = 500 * time.Millisecond

func uiApiUserRemovedByReloadPoll(ctx context.Context, condition func() bool) bool {
	for attempt := range uiApiUserRemovedByReloadAttempts {
		if condition() {
			return true
		}
		if attempt+1 < uiApiUserRemovedByReloadAttempts && !uiApiUserRemovedByReloadSleepContext(ctx, uiApiUserRemovedByReloadDelay) {
			return false
		}
	}
	return false
}

func uiApiUserRemovedByReloadSleepContext(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func uiApiUserRemovedByReloadRequire(condition bool, message string) error {
	if !condition {
		fmt.Fprintln(os.Stderr, "FAIL: "+message)
		return errors.New(message)
	}
	fmt.Fprintln(os.Stderr, "OK: "+message)
	return nil
}

func uiApiUserRemovedByReloadReloadGeneration(path string) int {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return 0
	}
	return bytes.Count(contents, []byte("sighup reload complete"))
}

func requireLogText(path, text, failure string) error {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err == nil && bytes.Contains(contents, []byte(text)) {
		return nil
	}
	fmt.Fprintln(os.Stderr, "FAIL: "+failure)
	if err == nil {
		_, _ = os.Stderr.Write(contents)
	}
	return errors.New(failure)
}

func writeFileToStderr(path string) {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err == nil {
		_, _ = os.Stderr.Write(contents)
	}
}

func writeTailToStderr(path string, count int) {
	contents, err := os.ReadFile(path) //nolint:gosec // the path is the fixture's own scratch file
	if err != nil {
		return
	}
	lines := bytes.SplitAfter(contents, []byte("\n"))
	if len(lines) != 0 && len(lines[len(lines)-1]) == 0 {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > count {
		lines = lines[len(lines)-count:]
	}
	for _, line := range lines {
		_, _ = os.Stderr.Write(line)
	}
}

func uiApiUserRemovedByReloadFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func statusText(status int) string {
	if status == 0 {
		return "None"
	}
	return strconv.Itoa(status)
}

func uiApiUserRemovedByReloadReplaceEnv(base []string, values map[string]string) []string {
	out := make([]string, 0, len(base)+len(values))
	for _, entry := range base {
		key := entry
		if before, _, found := strings.Cut(entry, "="); found {
			key = before
		}
		if _, replaced := values[key]; !replaced {
			out = append(out, entry)
		}
	}
	for key, value := range values {
		out = append(out, key+"="+value)
	}
	return out
}
