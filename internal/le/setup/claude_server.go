// Design: docs/guide/developer-setup.md -- the supported Ubuntu server bootstrap
//
// This file keeps the server bootstrap in the le binary. Paths redirect all
// filesystem effects into fixtures; downloads, identity lookup, platform
// values, archive installation, and processes have explicit test seams.
package setup

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/le/leaction"
)

const (
	claudeServerGoVersion     = "1.26.0"
	claudeServerNodeMajor     = "22"
	claudeServerRepo          = "https://github.com/ze-software/ze.git"
	claudeServerDownloadLimit = 256 << 20
	// serverHomeRoot is the parent directory of every account this setup provisions.
	serverHomeRoot = "/home"
)

const (
	goEnvironmentBlock = "\n# Go environment\nexport PATH=\"/usr/local/go/bin:$HOME/go/bin:$PATH\"\nexport GOPATH=\"$HOME/go\"\nexport COLORTERM=truecolor\n"
	claudeAliasBlock   = "\n# Claude alias\nalias cc='claude --dangerously-skip-permissions'\n"
	sshAgentBlock      = "\n# SSH agent — persist across mosh sessions using fixed socket\nexport SSH_AUTH_SOCK=\"$HOME/.ssh/agent.sock\"\nif ! ssh-add -l &>/dev/null; then\n    rm -f \"$SSH_AUTH_SOCK\"\n    eval \"$(ssh-agent -a \"$SSH_AUTH_SOCK\" -s)\" > /dev/null\nfi\n"
	autoCDTemplate     = "\n# Land in ze dev repo on login\ncd ~/Code/github.com/ze-software/ze/main 2>/dev/null\n"
)

// ClaudeServerPaths names every path the bootstrap reads or writes.
type ClaudeServerPaths struct {
	Home       string
	GoRoot     string
	GoArchive  string
	NodeKey    string
	NodeSource string
}

// ClaudeServerEvent is one ordered diagnostic in a server setup report.
type ClaudeServerEvent struct {
	Step   string `json:"step"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// claudeServerReport is both the structured answer and the human transcript.
type claudeServerReport struct {
	Events []ClaudeServerEvent `json:"events"`
}

func (r *claudeServerReport) add(step, status, detail string) {
	r.Events = append(r.Events, ClaudeServerEvent{Step: step, Status: status, Detail: detail})
}

// Text renders the legacy-style ordered diagnostic transcript.
func (r *claudeServerReport) Text() string {
	var text strings.Builder
	for _, event := range r.Events {
		if event.Step != "" {
			_, _ = text.WriteString(event.Step)
			if event.Detail != "" {
				_, _ = text.WriteString(": ")
			}
		}
		_, _ = text.WriteString(event.Detail)
		_ = text.WriteByte('\n')
	}
	return text.String()
}

// MarshalJSON keeps the report usable by le's row operators.
func (r *claudeServerReport) MarshalJSON() ([]byte, error) { return json.Marshal(r.Events) }

// claudeServerSetup provisions one pinned Ubuntu development server.
type claudeServerSetup struct {
	User      string
	SSHKeyDir string
	Paths     ClaudeServerPaths
	Shell     *Shell

	GOOS    string
	GOARCH  string
	Lookup  func(name string) (uid, gid int, err error)
	Fetch   func(ctx context.Context, url string, limit int64) ([]byte, error)
	Extract func(archive, destination string) error
}

func runClaudeServer(args leaction.Arguments) (any, int) {
	name := "thomas"
	if supplied, ok := args["user"]; ok {
		name = supplied
	}
	setup := &claudeServerSetup{User: name, SSHKeyDir: args["ssh-key-dir"]}
	report, code, err := setup.setup()
	if err != nil {
		leaction.ReportError(err)
	}
	return report, code
}

func (s *claudeServerSetup) prepare() {
	if s.Shell == nil {
		s.Shell = &Shell{}
	}
	if s.GOOS == "" {
		s.GOOS = runtime.GOOS
	}
	if s.GOARCH == "" {
		s.GOARCH = runtime.GOARCH
	}
	if s.Paths.Home == "" {
		s.Paths.Home = filepath.Join(serverHomeRoot, s.User)
	}
	if s.Paths.GoRoot == "" {
		s.Paths.GoRoot = "/usr/local/go"
	}
	if s.Paths.GoArchive == "" {
		s.Paths.GoArchive = "/tmp/go.tar.gz"
	}
	if s.Paths.NodeKey == "" {
		s.Paths.NodeKey = "/etc/apt/keyrings/nodesource.asc"
	}
	if s.Paths.NodeSource == "" {
		s.Paths.NodeSource = "/etc/apt/sources.list.d/nodesource.list"
	}
	if s.Lookup == nil {
		s.Lookup = lookupServerUser
	}
	if s.Fetch == nil {
		s.Fetch = fetchServerFile
	}
	if s.Extract == nil {
		s.Extract = extractGoArchive
	}
}

// repositoryPath answers where this setup clones the checkout inside the account.
func (s *claudeServerSetup) repositoryPath() string {
	return filepath.Join(s.Paths.Home, "Code", "github.com", "ze-software", "ze", "main")
}

// goBinary answers the go command inside the installed toolchain.
func (s *claudeServerSetup) goBinary() string {
	return filepath.Join(s.Paths.GoRoot, "bin", "go")
}

// setup provisions the Ubuntu development server without a shell-script runtime.
func (s *claudeServerSetup) setup() (*claudeServerReport, int, error) {
	s.prepare()
	report := &claudeServerReport{}
	if s.GOOS != "linux" || s.GOARCH != "amd64" {
		return report, 1, fmt.Errorf("claude-server setup supports linux/amd64, got %s/%s", s.GOOS, s.GOARCH)
	}
	if s.Shell.Euid != nil && s.Shell.Euid() != 0 {
		return report, 1, fmt.Errorf("run as root (sudo le setup claude-server user %s)", s.User)
	}
	if s.Shell.Euid == nil && os.Geteuid() != 0 {
		return report, 1, fmt.Errorf("run as root (sudo le setup claude-server user %s)", s.User)
	}
	uid, gid, err := s.Lookup(s.User)
	if err != nil {
		return report, 1, fmt.Errorf("user %q does not exist", s.User)
	}
	for _, prerequisite := range []string{aptBin, sudoBin} {
		if !s.Shell.Present(prerequisite) {
			return report, 1, fmt.Errorf("required command %q was not found", prerequisite)
		}
	}

	report.add("", "heading", "=== Setting up ze dev environment for user: "+s.User+" ===")
	if err := s.installSystem(report); err != nil {
		return report, 1, err
	}
	if err := s.configureFirewall(report); err != nil {
		return report, 1, err
	}
	if err := s.installGo(report); err != nil {
		return report, 1, err
	}
	if err := s.installGoTools(report); err != nil {
		return report, 1, err
	}
	if err := s.installNode(report); err != nil {
		return report, 1, err
	}
	if err := s.installClaude(report); err != nil {
		return report, 1, err
	}
	if err := s.configureShell(report); err != nil {
		return report, 1, err
	}
	if err := s.installSSHKey(report, uid, gid); err != nil {
		return report, 1, err
	}
	if err := s.cloneRepository(report); err != nil {
		return report, 1, err
	}
	s.verify(report)
	report.add("", "done", "=== Done ===")
	repo := s.repositoryPath()
	report.add("Next steps", "note", "1. Log in as "+s.User+" (or: su - "+s.User+")")
	report.add("", "note", "2. Run 'claude' to authenticate and start working")
	report.add("", "note", "3. cd "+repo+" && ./le verify current mode full")
	return report, 0, nil
}

func (s *claudeServerSetup) run(report *claudeServerReport, step string, cmd Cmd) error {
	result := s.Shell.Run(cmd)
	if result.OK() {
		for _, line := range outputLines(result.Out) {
			report.add(step, "output", line)
		}
		return nil
	}
	complaint := result.complaint()
	report.add(step, "failed", complaint)
	return fmt.Errorf("%s: %s", step, complaint)
}

func outputLines(output string) []string {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

func (s *claudeServerSetup) installSystem(report *claudeServerReport) error {
	report.add("", "section", "--- Installing system packages ---")
	if err := s.run(report, "apt-get update", Cmd{Argv: []string{aptBin, updateSubcommand, aptQuietFlag}}); err != nil {
		return err
	}
	argv := []string{aptBin, installVerb, "-y", aptQuietFlag, "build-essential", toolGit, "curl", "wget", "jq", "unzip", "mosh"}
	return s.run(report, "apt-get install", Cmd{Argv: argv})
}

func (s *claudeServerSetup) configureFirewall(report *claudeServerReport) error {
	report.add("", "section", "--- Configuring firewall ---")
	if !s.Shell.Present(firewallBin) {
		report.add("Firewall", "skipped", "ufw not found, skipping firewall setup")
		return nil
	}
	for _, argv := range [][]string{{firewallBin, "allow", "OpenSSH"}, {firewallBin, "allow", "60000:61000/udp", "comment", "mosh"}} {
		if err := s.run(report, firewallBin, Cmd{Argv: argv}); err != nil {
			return err
		}
	}
	status := s.Shell.Run(Cmd{Argv: []string{firewallBin, statusSubcommand}})
	if !status.OK() {
		return fmt.Errorf("ufw status: %s", status.complaint())
	}
	if strings.Contains(strings.ToLower(firstLine(status.Out)), "status: active") {
		report.add("Firewall", "present", "ufw already active, rules updated")
	} else {
		if err := s.run(report, "ufw enable", Cmd{Argv: []string{firewallBin, "enable"}, Stdin: []byte("y\n")}); err != nil {
			return err
		}
		report.add("Firewall", "installed", "ufw enabled")
	}
	return s.run(report, "ufw status", Cmd{Argv: []string{firewallBin, statusSubcommand}})
}

func (s *claudeServerSetup) installGo(report *claudeServerReport) error {
	report.add("", "section", "--- Installing Go "+claudeServerGoVersion+" ---")
	goBinary := s.goBinary()
	version := s.Shell.Run(Cmd{Argv: []string{goBinary, versionSubcommand}})
	if version.OK() && serverGoVersion(version.Out) == claudeServerGoVersion {
		report.add("Go", "present", "Go "+claudeServerGoVersion+" already installed, skipping")
		return nil
	}
	archiveURL := "https://go.dev/dl/go" + claudeServerGoVersion + ".linux-amd64.tar.gz"
	archive, err := s.Fetch(s.Shell.context(), archiveURL, claudeServerDownloadLimit)
	if err != nil {
		return fmt.Errorf("download Go %s: %w", claudeServerGoVersion, err)
	}
	checksum, err := s.Fetch(s.Shell.context(), archiveURL+".sha256", 4096)
	if err != nil {
		return fmt.Errorf("download Go %s checksum: %w", claudeServerGoVersion, err)
	}
	if err := verifyServerChecksum(archive, checksum); err != nil {
		return fmt.Errorf("verify Go %s checksum: %w", claudeServerGoVersion, err)
	}
	if err := writeServerFile(s.Paths.GoArchive, archive, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.Paths.GoArchive, err)
	}
	if err := s.Extract(s.Paths.GoArchive, s.Paths.GoRoot); err != nil {
		return fmt.Errorf("install Go %s: %w", claudeServerGoVersion, err)
	}
	if err := os.Remove(s.Paths.GoArchive); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", s.Paths.GoArchive, err)
	}
	installed := s.Shell.Run(Cmd{Argv: []string{goBinary, versionSubcommand}})
	if !installed.OK() {
		return fmt.Errorf("verify installed Go: %s", installed.complaint())
	}
	if got := serverGoVersion(installed.Out); got != claudeServerGoVersion {
		return fmt.Errorf("verify installed Go: got version %q, want %q", got, claudeServerGoVersion)
	}
	report.add("Go", "installed", "Go "+strings.TrimSpace(installed.Out)+" installed")
	return nil
}

func (s *claudeServerSetup) installGoTools(report *claudeServerReport) error {
	report.add("", "section", "--- Installing Go tools ---")
	gopath := filepath.Join(s.Paths.Home, "go")
	if err := s.run(report, "create GOPATH", Cmd{Argv: []string{sudoBin, "-u", s.User, "mkdir", "-p", filepath.Join(gopath, "bin")}}); err != nil {
		return err
	}
	environment := replaceEnvironment(os.Environ(), map[string]string{
		"CGO_ENABLED": "0",
		"GOPATH":      gopath,
		"PATH":        "/usr/local/go/bin:" + filepath.Join(gopath, "bin") + ":" + os.Getenv("PATH"),
	})
	lint := s.Shell.Run(Cmd{Argv: []string{toolGolangCI, versionFlag}, Env: environment})
	if lint.OK() {
		report.add(toolGolangCI, "present", "already installed: "+firstLine(lint.Out))
	} else {
		target := "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + GolangCIVersion
		if err := s.run(report, "install golangci-lint", Cmd{Argv: []string{"go", installVerb, target}, Env: environment}); err != nil {
			return err
		}
		if err := s.run(report, "own GOPATH", Cmd{Argv: []string{"chown", "-R", s.User + ":" + s.User, gopath}}); err != nil {
			return err
		}
		report.add(toolGolangCI, "installed", GolangCIVersion)
	}
	goBinary := s.goBinary()
	staticcheck := []string{sudoBin, "-u", s.User, envBin, "CGO_ENABLED=0", goBinary, installVerb, staticcheckTarget}
	if err := s.run(report, "install staticcheck", Cmd{Argv: staticcheck}); err != nil {
		return err
	}
	result := s.Shell.Run(Cmd{Argv: []string{"staticcheck", versionArgument}, Env: environment})
	if !result.OK() {
		return fmt.Errorf("verify staticcheck: %s", result.complaint())
	}
	report.add("staticcheck", "installed", strings.TrimSpace(result.Out))
	goimports := s.Shell.Run(Cmd{Argv: []string{toolGoimports, "--help"}, Env: environment})
	if goimports.OK() {
		report.add(toolGoimports, "present", "already installed")
		return nil
	}
	argv := []string{sudoBin, "-u", s.User, envBin, "CGO_ENABLED=0", goBinary, installVerb, goimportsTarget}
	if err := s.run(report, "install goimports", Cmd{Argv: argv}); err != nil {
		return err
	}
	report.add(toolGoimports, "installed", "installed")
	return nil
}

func (s *claudeServerSetup) installNode(report *claudeServerReport) error {
	report.add("", "section", "--- Installing Node.js "+claudeServerNodeMajor+".x ---")
	version := s.Shell.Run(Cmd{Argv: []string{toolNode, versionFlag}})
	if version.OK() && strings.HasPrefix(strings.TrimSpace(version.Out), "v"+claudeServerNodeMajor+".") {
		report.add("Node.js", "present", strings.TrimSpace(version.Out)+" already installed, skipping")
		return nil
	}
	keyURL := "https://deb.nodesource.com/gpgkey/nodesource-repo.gpg.key"
	key, err := s.Fetch(s.Shell.context(), keyURL, 1<<20)
	if err != nil {
		return fmt.Errorf("download NodeSource signing key: %w", err)
	}
	if err := writeServerFile(s.Paths.NodeKey, key, 0o644); err != nil {
		return fmt.Errorf("install NodeSource signing key: %w", err)
	}
	source := "deb [arch=amd64 signed-by=" + s.Paths.NodeKey + "] https://deb.nodesource.com/node_" + claudeServerNodeMajor + ".x nodistro main\n"
	if err := writeServerFile(s.Paths.NodeSource, []byte(source), 0o644); err != nil {
		return fmt.Errorf("install NodeSource repository: %w", err)
	}
	if err := s.run(report, "apt-get update for NodeSource", Cmd{Argv: []string{aptBin, updateSubcommand, aptQuietFlag}}); err != nil {
		return err
	}
	if err := s.run(report, "install nodejs", Cmd{Argv: []string{aptBin, installVerb, "-y", aptQuietFlag, "nodejs"}}); err != nil {
		return err
	}
	installed := s.Shell.Run(Cmd{Argv: []string{toolNode, versionFlag}})
	if !installed.OK() {
		return fmt.Errorf("verify Node.js %s.x: %s", claudeServerNodeMajor, installed.complaint())
	}
	got := strings.TrimSpace(installed.Out)
	if !strings.HasPrefix(got, "v"+claudeServerNodeMajor+".") {
		return fmt.Errorf("verify Node.js: got version %q, want v%s.x", got, claudeServerNodeMajor)
	}
	report.add("Node.js", "installed", strings.TrimSpace(installed.Out)+" installed")
	return nil
}

func (s *claudeServerSetup) installClaude(report *claudeServerReport) error {
	report.add("", "section", "--- Installing Claude CLI ---")
	version := s.Shell.Run(Cmd{Argv: []string{toolClaude, versionFlag}})
	if version.OK() {
		report.add("Claude CLI", "present", "already installed, skipping")
		return nil
	}
	if err := s.run(report, "install Claude CLI", Cmd{Argv: []string{toolNpm, installVerb, "-g", "@anthropic-ai/claude-code"}}); err != nil {
		return err
	}
	installed := s.Shell.Run(Cmd{Argv: []string{toolClaude, versionFlag}})
	detail := "ok"
	if installed.OK() && strings.TrimSpace(installed.Out) != "" {
		detail = strings.TrimSpace(installed.Out)
	}
	report.add("Claude CLI", "installed", detail)
	return nil
}

func (s *claudeServerSetup) configureShell(report *claudeServerReport) error {
	report.add("", "section", "--- Configuring shell environment ---")
	bashrc := filepath.Join(s.Paths.Home, ".bashrc")
	steps := []struct {
		needle string
		block  string
		added  string
		found  string
	}{
		{"/usr/local/go/bin", goEnvironmentBlock, "Added Go paths to .bashrc", "Go paths already in .bashrc"},
		{"alias cc=", claudeAliasBlock, "Added cc alias to .bashrc", "cc alias already in .bashrc"},
		{"SSH_AUTH_SOCK", sshAgentBlock, "Added ssh-agent to .bashrc", "ssh-agent already in .bashrc"},
		{"cd.*ze/main", autoCDTemplate, "Added auto-cd to .bashrc", "Auto-cd already in .bashrc"},
	}
	contents, err := os.ReadFile(bashrc) //nolint:gosec // bashrc sits under the home directory this setup provisions
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read %s: %w", bashrc, err)
	}
	for _, step := range steps {
		found := strings.Contains(string(contents), step.needle)
		if step.needle == "cd.*ze/main" {
			found = strings.Contains(string(contents), "cd ~/Code/github.com/ze-software/ze/main")
		}
		if found {
			report.add("Shell", "present", step.found)
			continue
		}
		contents = append(contents, step.block...)
		report.add("Shell", "installed", step.added)
	}
	if err := writeServerFile(bashrc, contents, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", bashrc, err)
	}
	return nil
}

func (s *claudeServerSetup) installSSHKey(report *claudeServerReport, uid, gid int) error {
	report.add("", "section", "--- SSH key setup ---")
	sshDir := filepath.Join(s.Paths.Home, ".ssh")
	key := filepath.Join(sshDir, "id_ed25519")
	if info, err := os.Stat(key); err == nil && !info.IsDir() {
		report.add("SSH key", "present", "already exists at "+key)
		return nil
	}
	if s.SSHKeyDir == "" {
		return fmt.Errorf("no SSH key found and no key directory provided; usage: le setup claude-server user %s ssh-key-dir <path>", s.User)
	}
	private, err := os.ReadFile(filepath.Join(s.SSHKeyDir, "id_ed25519"))
	if err != nil {
		return fmt.Errorf("read SSH private key: %w", err)
	}
	public, err := os.ReadFile(filepath.Join(s.SSHKeyDir, "id_ed25519.pub"))
	if err != nil {
		return fmt.Errorf("read SSH public key: %w", err)
	}
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", sshDir, err)
	}
	if err := os.Chmod(sshDir, 0o700); err != nil { //nolint:gosec // .ssh must carry the owner execute bit or ssh refuses it
		return fmt.Errorf("chmod %s: %w", sshDir, err)
	}
	if err := writeServerFile(key, private, 0o600); err != nil {
		return fmt.Errorf("install SSH private key: %w", err)
	}
	if err := writeServerFile(key+".pub", public, 0o644); err != nil {
		return fmt.Errorf("install SSH public key: %w", err)
	}
	for _, path := range []string{sshDir, key, key + ".pub"} {
		if err := os.Chown(path, uid, gid); err != nil {
			return fmt.Errorf("chown %s: %w", path, err)
		}
	}
	report.add("SSH key", "installed", strings.TrimSpace(string(public)))
	return nil
}

func (s *claudeServerSetup) cloneRepository(report *claudeServerReport) error {
	report.add("", "section", "--- Repository setup ---")
	repo := s.repositoryPath()
	if info, err := os.Stat(filepath.Join(repo, ".git")); err == nil && info.IsDir() {
		report.add("Ze repo", "present", "already cloned at "+repo)
		return nil
	}
	parent := filepath.Dir(repo)
	if err := s.run(report, "create repository directory", Cmd{Argv: []string{sudoBin, "-u", s.User, "mkdir", "-p", parent}}); err != nil {
		return err
	}
	clone := s.Shell.Run(Cmd{Argv: []string{sudoBin, "-u", s.User, toolGit, "clone", claudeServerRepo, repo}})
	if clone.OK() {
		report.add("Ze repo", "installed", "Cloned successfully")
		return nil
	}
	report.add("Ze repo", "failed", "CLONE FAILED — add SSH key to GitHub first, then run:")
	report.add("", "note", "git clone git@github.com:ze-software/ze.git "+repo)
	return nil
}

func (s *claudeServerSetup) verify(report *claudeServerReport) {
	report.add("", "section", "=== Verification ===")
	repo := s.repositoryPath()
	checks := []struct {
		name string
		argv []string
	}{
		{"Go", []string{s.goBinary(), versionSubcommand}},
		{"Node", []string{toolNode, versionFlag}},
		{toolNpm, []string{toolNpm, versionFlag}},
		{"Claude", []string{toolClaude, versionFlag}},
		{"Lint", []string{toolGolangCI, versionFlag}},
		{"Types", []string{"staticcheck", versionArgument}},
		{"Imports", []string{toolGoimports, "--help"}},
		{"Git", []string{toolGit, versionFlag}},
		{"Mosh", []string{"mosh-server", versionFlag}},
		{"Firewall", []string{firewallBin, statusSubcommand}},
	}
	for _, check := range checks {
		result := s.Shell.Run(Cmd{Argv: check.argv})
		detail := "MISSING"
		if result.OK() {
			detail = firstLine(result.Out)
		}
		report.add(check.name, "verification", detail)
	}
	if info, err := os.Stat(filepath.Join(repo, ".git")); err == nil && info.IsDir() {
		report.add("Ze repo", "verification", repo)
	} else {
		report.add("Ze repo", "verification", "NOT CLONED")
	}
}

func firstLine(value string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(value), "\n")
	return line
}

func serverGoVersion(output string) string {
	for field := range strings.FieldsSeq(output) {
		if version, ok := strings.CutPrefix(field, "go"); ok && version != "" {
			return version
		}
	}
	return ""
}

func replaceEnvironment(current []string, values map[string]string) []string {
	environment := make([]string, 0, len(current)+len(values))
	for _, item := range current {
		key, _, _ := strings.Cut(item, "=")
		if _, replaced := values[key]; replaced {
			continue
		}
		environment = append(environment, item)
	}
	for key, value := range values {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func lookupServerUser(name string) (int, int, error) {
	account, err := user.Lookup(name)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("parse uid %q: %w", account.Uid, err)
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("parse gid %q: %w", account.Gid, err)
	}
	return uid, gid, nil
}

func fetchServerFile(ctx context.Context, url string, limit int64) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("GET %s exceeded %d bytes", url, limit)
	}
	return body, nil
}

func verifyServerChecksum(content, checksumFile []byte) error {
	fields := strings.Fields(string(checksumFile))
	if len(fields) == 0 {
		return errors.New("empty checksum file")
	}
	want, err := hex.DecodeString(fields[0])
	if err != nil || len(want) != sha256.Size {
		return fmt.Errorf("invalid SHA-256 %q", fields[0])
	}
	got := sha256.Sum256(content)
	if !equalBytes(got[:], want) {
		return fmt.Errorf("SHA-256 mismatch: got %x, want %x", got, want)
	}
	return nil
}

func equalBytes(left, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	var different byte
	for index := range left {
		different |= left[index] ^ right[index]
	}
	return different == 0
}

func writeServerFile(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode) //nolint:gosec // the caller states the mode, and each call site names it
	if err != nil {
		return err
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Chmod(mode); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func extractGoArchive(archive, destination string) error {
	file, err := os.Open(archive) //nolint:gosec // archive is the toolchain tarball this setup downloaded
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() { _ = compressed.Close() }()
	parent := filepath.Dir(destination)
	stage, err := os.MkdirTemp(parent, ".go-install-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()
	if err := os.Chmod(stage, 0o755); err != nil { //nolint:gosec // every account on the server runs the toolchain, so its tree stays world-traversable
		return err
	}
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if err := extractGoEntry(stage, header, reader); err != nil {
			return err
		}
	}
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return os.Rename(stage, destination)
}

func extractGoEntry(stage string, header *tar.Header, reader io.Reader) error {
	name := filepath.Clean(filepath.FromSlash(header.Name))
	prefix := "go" + string(filepath.Separator)
	if name == "go" {
		if header.Typeflag != tar.TypeDir {
			return fmt.Errorf("archive root %q is not a directory", header.Name)
		}
		return os.Chmod(stage, os.FileMode(header.Mode).Perm())
	}
	if !strings.HasPrefix(name, prefix) {
		return fmt.Errorf("archive entry %q is outside go/", header.Name)
	}
	relative := strings.TrimPrefix(name, prefix)
	target := filepath.Join(stage, relative)
	if target != stage && !strings.HasPrefix(target, stage+string(filepath.Separator)) {
		return fmt.Errorf("archive entry %q escapes the destination", header.Name)
	}
	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(target, os.FileMode(header.Mode).Perm())
	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // every account on the server runs the toolchain, so its tree stays world-traversable
			return err
		}
		file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode).Perm()) //nolint:gosec // target is checked above against escaping the staging directory
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(file, reader)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	case tar.TypeSymlink:
		if filepath.IsAbs(header.Linkname) || strings.Contains(filepath.Clean(header.Linkname), "..") {
			return fmt.Errorf("archive symlink %q has unsafe target %q", header.Name, header.Linkname)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil { //nolint:gosec // every account on the server runs the toolchain, so its tree stays world-traversable
			return err
		}
		return os.Symlink(filepath.FromSlash(header.Linkname), target)
	default:
		return fmt.Errorf("archive entry %q has unsupported type %d", header.Name, header.Typeflag)
	}
}
