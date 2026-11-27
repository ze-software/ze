// Design: docs/architecture/aaa-tacacs.md -- per-surface login-service functional fixtures.
// Related: register_login_service.go -- registers these fixture drivers.
// Related: login_service_helpers.go -- shared protocol clients and daemon helpers.
// Related: login_service_modes.go -- remote backend, reload, recovery, and mode fixtures.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc/codes"

	"github.com/ze-software/ze/internal/core/textbuf"
)

const (
	lsUserAdmin   = "admin"
	lsUserAPIOnly = "apiop"
	lsUserWebOnly = "webop"
	lsUserNoWeb   = "noweb"
	lsUserREST    = "restonly"
	lsUserGRPC    = "grpconly"
	lsUserAlice   = "alice"

	lsSSHAuthFailedHint = "ssh: handshake failed"
	lsDeniedCommand     = "clear interface counters all"
)

func lsFixtureSSHPassword(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-ssh-password <ssh-port> <rest-port> <metrics-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	metricsPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	if err := lsWaitSSHAndRESTReady(ctx, sshPort, restPort); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, metricsPort, "Prometheus"); err != nil {
		return err
	}

	// Capture daemon running config before refused attempt (AC-3 zero side effects).
	beforeCfg, err := lsExecSSHPassword(sshPort, lsUserAdmin, lsTestPassword, lsCmdShowConfig)
	if err != nil {
		return fmt.Errorf("admin baseline show config failed: %w", err)
	}

	// Capture wrong-password error on SSH for indistinguishability (AC-2).
	_, wrongErr := lsDialSSHPassword(sshPort, lsUserAPIOnly, lsWrongPassword)
	if wrongErr == nil {
		return errors.New("wrong password unexpectedly authenticated on SSH")
	}
	if !strings.Contains(wrongErr.Error(), lsSSHAuthFailedHint) {
		return fmt.Errorf("wrong password SSH error unexpected: %w", wrongErr)
	}

	// Excluded door: correct password must be refused identically on SSH (AC-2).
	client, excludedErr := lsDialSSHPassword(sshPort, lsUserAPIOnly, lsTestPassword)
	if excludedErr == nil {
		_ = client.Close() //nolint:errcheck // fixture cleanup
		return errors.New("apiop with rest-only login-service unexpectedly authenticated on SSH")
	}
	if !strings.Contains(excludedErr.Error(), lsSSHAuthFailedHint) {
		return fmt.Errorf("SSH refusal mismatch: excluded=%w wrong=%w", excludedErr, wrongErr)
	}
	fmt.Fprintln(os.Stderr, "OK: SSH password login refused for excluded account identically to wrong password")

	// Included door: same account + same password succeeds on REST (AC-2, AC-4).
	statusCode, body, err := lsRESTExecute(ctx, restPort, lsUserAPIOnly+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("REST execute as apiop failed: %w", err)
	}
	if statusCode != http.StatusOK {
		return fmt.Errorf("REST execute as apiop returned HTTP %d, want 200: %s", statusCode, body)
	}
	// AC-4: command denied by apiop's read-only profile is still refused HTTP 403.
	deniedCode, deniedBody, err := lsRESTExecute(ctx, restPort, lsUserAPIOnly+":"+lsTestPassword, lsDeniedCommand)
	if err != nil {
		return fmt.Errorf("REST denied-command probe failed: %w", err)
	}
	if deniedCode != http.StatusForbidden {
		return fmt.Errorf("REST denied command returned HTTP %d, want 403: %s", deniedCode, deniedBody)
	}
	fmt.Fprintln(os.Stderr, "OK: same account succeeded on REST and honored command authorization profile")

	// AC-3: audit log records auth-fail and daemon config is unchanged.
	if err := lsAssertAuditContains(ctx, sshPort, lsUserAdmin, lsTestPassword, lsUserAPIOnly, "ssh"); err != nil {
		return err
	}
	afterCfg, err := lsExecSSHPassword(sshPort, lsUserAdmin, lsTestPassword, lsCmdShowConfig)
	if err != nil {
		return fmt.Errorf("admin post-attempt show config failed: %w", err)
	}
	if beforeCfg != afterCfg {
		return errors.New("refused SSH login changed running configuration")
	}

	// AC-3: Prometheus counter stepped for service="ssh".
	metricsText, err := lsScrapePrometheus(ctx, metricsPort)
	if err != nil {
		return err
	}
	count, found, err := lsRefusedCounterValue(metricsText, "ssh")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("missing %s{service=\"ssh\"} metric series", lsRefusedMetric)
	}
	if count < 1 {
		return fmt.Errorf("expected %s{service=\"ssh\"} >= 1, got %v", lsRefusedMetric, count)
	}
	fmt.Fprintln(os.Stderr, "OK: audit record and refusal metric verified with zero side effects")
	return nil
}

func lsFixtureSSHPublicKey(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/login-service-ssh-publickey <ssh-port> <rest-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	signer, pubKeyData, err := lsGenEd25519KeyPair()
	if err != nil {
		return err
	}
	wrongSigner, _, err := lsGenEd25519KeyPair()
	if err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	var cfg textbuf.Buffer
	cfg.Str("system {\n\tauthentication {\n")
	cfg.Str("\t\tuser deniedkey {\n\t\t\tpassword \"").Str(lsTestPasswordHash).Str("\"\n")
	cfg.Str("\t\t\tlogin-service rest-only\n")
	cfg.Str("\t\t\tpublic-keys k1 { type ssh-ed25519; key ").Str(pubKeyData).Str(" }\n\t\t}\n")
	cfg.Str("\t\tuser allowedkey {\n\t\t\tpassword \"").Str(lsTestPasswordHash).Str("\"\n")
	cfg.Str("\t\t\tlogin-service ssh-only\n")
	cfg.Str("\t\t\tpublic-keys k1 { type ssh-ed25519; key ").Str(pubKeyData).Str(" }\n\t\t}\n")
	cfg.Str("\t}\n\tauthorization {\n")
	cfg.Str("\t\tlogin-service rest-only { service [ rest ] }\n")
	cfg.Str("\t\tlogin-service ssh-only { service [ ssh ] }\n")
	cfg.Str("\t}\n}\n")
	cfg.Str("environment {\n\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
	cfg.Str("\tapi-server { rest { enabled true; server main { ip 127.0.0.1; port ").Str(restPort).Str("; } } }\n}\n")

	cfgPath := filepath.Join(workDir, "ssh-pubkey-ls.conf")
	if err := lsWriteFile(cfgPath, cfg.String()); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	if err := lsWaitSSHAndRESTReady(ctx, sshPort, restPort); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}

	// Wrong key baseline on allowedkey.
	_, wrongErr := lsDialSSHPublicKey(sshPort, "allowedkey", wrongSigner)
	if wrongErr == nil {
		return errors.New("unconfigured wrong key unexpectedly authenticated")
	}

	// Excluded door: valid configured key on deniedkey MUST be refused on SSH.
	deniedClient, deniedErr := lsDialSSHPublicKey(sshPort, "deniedkey", signer)
	if deniedErr == nil {
		_ = deniedClient.Close() //nolint:errcheck // fixture cleanup
		return errors.New("deniedkey with rest-only login-service unexpectedly authenticated by public key on SSH")
	}
	if !strings.Contains(deniedErr.Error(), lsSSHAuthFailedHint) {
		return fmt.Errorf("excluded pubkey error unexpected: %w", deniedErr)
	}
	fmt.Fprintln(os.Stderr, "OK: configured SSH public key refused when ssh is excluded")

	// Included door for deniedkey: password works on REST.
	restStatus, restBody, err := lsRESTExecute(ctx, restPort, "deniedkey:"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("REST execute as deniedkey failed: %w", err)
	}
	if restStatus != http.StatusOK {
		return fmt.Errorf("REST execute as deniedkey returned HTTP %d, want 200: %s", restStatus, restBody)
	}

	// Included door for allowedkey: same Ed25519 key succeeds on SSH.
	client, err := lsDialSSHPublicKey(sshPort, "allowedkey", signer)
	if err != nil {
		return fmt.Errorf("allowedkey with ssh in login-service failed SSH public-key login: %w\n%s", err, daemon.logText())
	}
	defer client.Close() //nolint:errcheck // fixture cleanup
	if _, err := lsRunSSHCommand(client, lsCmdShowVersion); err != nil {
		return fmt.Errorf("allowedkey show version failed: %w", err)
	}
	fmt.Fprintln(os.Stderr, "OK: configured SSH public key admitted when ssh is included")
	return nil
}

func lsFixtureZeCLI(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/login-service-ze-cli <ssh-port> <rest-port>")
	}
	sshPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	if err := lsWaitSSHAndRESTReady(ctx, sshPort, restPort); err != nil {
		return err
	}
	cliDir, err := os.MkdirTemp("", "ls-ze-cli-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cliDir) //nolint:errcheck // fixture cleanup

	// Wrong-password refusal baseline via ze cli.
	wrongOut, wrongErr := lsRunCLI(ctx, cliDir, sshPort, "deniedcli", lsWrongPassword, lsCmdShowVersion)
	if wrongErr == nil {
		return errors.New("ze cli unexpectedly succeeded with wrong password")
	}
	if !strings.Contains(wrongOut, "refused the login") {
		return fmt.Errorf("ze cli wrong-password output unexpected: %q", wrongOut)
	}

	// Excluded account with correct password must be refused by ze cli.
	deniedOut, deniedErr := lsRunCLI(ctx, cliDir, sshPort, "deniedcli", lsTestPassword, lsCmdShowVersion)
	if deniedErr == nil {
		return fmt.Errorf("ze cli unexpectedly succeeded for excluded account: %s", deniedOut)
	}
	if !strings.Contains(deniedOut, "refused the login") {
		return fmt.Errorf("ze cli refusal mismatch: denied=%q wrong=%q", deniedOut, wrongOut)
	}
	fmt.Fprintln(os.Stderr, "OK: ze cli refused excluded account identically to wrong password")

	// Excluded account still succeeds on its included door (REST).
	restStatus, restBody, err := lsRESTExecute(ctx, restPort, "deniedcli:"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("REST execute as deniedcli failed: %w", err)
	}
	if restStatus != http.StatusOK {
		return fmt.Errorf("REST execute as deniedcli returned HTTP %d, want 200: %s", restStatus, restBody)
	}

	// Included account succeeds via ze cli.
	allowedOut, allowedErr := lsRunCLI(ctx, cliDir, sshPort, "allowedcli", lsTestPassword, lsCmdShowVersion)
	if allowedErr != nil {
		return fmt.Errorf("ze cli failed for allowed account: %w\n%s", allowedErr, allowedOut)
	}
	fmt.Fprintln(os.Stderr, "OK: ze cli admitted included account")
	return nil
}

func lsFixtureWebForm(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-web-form <web-port> <ssh-port> <metrics-port>")
	}
	webPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	sshPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	metricsPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, webPort, "web"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, metricsPort, "Prometheus"); err != nil {
		return err
	}
	client, closeIdle := lsNewHTTPSClient()
	defer closeIdle()

	// Wrong-password baseline on web login form.
	wrongResp, err := lsWebFormLogin(ctx, client, webPort, lsUserNoWeb, lsWrongPassword)
	if err != nil {
		return fmt.Errorf("web form wrong-password request failed: %w", err)
	}
	if wrongResp.status != http.StatusUnauthorized {
		return fmt.Errorf("wrong password returned HTTP %d, want 401", wrongResp.status)
	}

	// Excluded account with correct password must get identical 401 + body and no cookie.
	refusedResp, err := lsWebFormLogin(ctx, client, webPort, lsUserNoWeb, lsTestPassword)
	if err != nil {
		return fmt.Errorf("web form excluded-account request failed: %w", err)
	}
	if refusedResp.status != wrongResp.status {
		return fmt.Errorf("excluded web form status %d != wrong-password status %d", refusedResp.status, wrongResp.status)
	}
	if refusedResp.cookie != "" {
		return errors.New("excluded web form login issued a ze-session cookie")
	}
	if refusedResp.body != wrongResp.body {
		return errors.New("excluded web form body differs from wrong-password login page")
	}
	fmt.Fprintln(os.Stderr, "OK: web login form refused excluded account identically to wrong password")

	// Excluded account succeeds on its included door (SSH).
	if _, err := lsExecSSHPassword(sshPort, lsUserNoWeb, lsTestPassword, lsCmdShowVersion); err != nil {
		return fmt.Errorf("noweb failed on its included SSH door: %w", err)
	}

	// Included account receives ze-session cookie and can reach protected route.
	allowedResp, err := lsWebFormLogin(ctx, client, webPort, lsUserWebOnly, lsTestPassword)
	if err != nil {
		return fmt.Errorf("web form allowed-account request failed: %w", err)
	}
	if allowedResp.status != http.StatusSeeOther {
		return fmt.Errorf("allowed web form login returned HTTP %d, want 303", allowedResp.status)
	}
	if allowedResp.cookie == "" {
		return errors.New("allowed web form login issued no ze-session cookie")
	}
	pageResp, err := lsWebCookieGet(ctx, client, webPort, allowedResp.cookie, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("web cookie GET failed: %w", err)
	}
	if pageResp.status != http.StatusOK {
		return fmt.Errorf("allowed account session cookie returned HTTP %d, want 200", pageResp.status)
	}
	fmt.Fprintln(os.Stderr, "OK: web login form issued session cookie for included account")

	// AC-3: audit failure + Prometheus counter for service="web".
	if err := lsAssertAuditContains(ctx, sshPort, lsUserAdmin, lsTestPassword, lsUserNoWeb, "web"); err != nil {
		return err
	}
	metricsText, err := lsScrapePrometheus(ctx, metricsPort)
	if err != nil {
		return err
	}
	count, found, err := lsRefusedCounterValue(metricsText, "web")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("missing %s{service=\"web\"} metric series", lsRefusedMetric)
	}
	if count < 1 {
		return fmt.Errorf("expected %s{service=\"web\"} >= 1, got %v", lsRefusedMetric, count)
	}
	fmt.Fprintln(os.Stderr, "OK: web audit record and refusal metric verified")
	return nil
}

func lsFixtureWebBasic(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/login-service-web-basic <web-port> <ssh-port>")
	}
	webPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	sshPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, webPort, "web"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return err
	}
	client, closeIdle := lsNewHTTPSClient()
	defer closeIdle()

	wrongResp, err := lsWebBasicGet(ctx, client, webPort, lsUserNoWeb, lsWrongPassword, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("web Basic wrong-password failed: %w", err)
	}
	if wrongResp.status != http.StatusUnauthorized {
		return fmt.Errorf("web Basic wrong-password status=%d, want 401", wrongResp.status)
	}

	refusedResp, err := lsWebBasicGet(ctx, client, webPort, lsUserNoWeb, lsTestPassword, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("web Basic excluded-account failed: %w", err)
	}
	if refusedResp.status != http.StatusUnauthorized {
		return fmt.Errorf("web Basic excluded-account status=%d, want 401", refusedResp.status)
	}
	if refusedResp.body != wrongResp.body {
		return errors.New("web Basic excluded-account body differs from wrong-password response")
	}
	fmt.Fprintln(os.Stderr, "OK: web Basic Auth refused excluded account identically to wrong password")

	// Excluded account works on SSH.
	if _, err := lsExecSSHPassword(sshPort, lsUserNoWeb, lsTestPassword, lsCmdShowVersion); err != nil {
		return fmt.Errorf("noweb failed on its included SSH door: %w", err)
	}

	// Included account works on Web Basic Auth.
	allowedResp, err := lsWebBasicGet(ctx, client, webPort, lsUserWebOnly, lsTestPassword, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("web Basic allowed-account failed: %w", err)
	}
	if allowedResp.status != http.StatusOK {
		return fmt.Errorf("web Basic allowed-account status=%d, want 200", allowedResp.status)
	}
	fmt.Fprintln(os.Stderr, "OK: web Basic Auth served included account")
	return nil
}

func lsFixtureWebCookieReload(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("usage: plugin/login-service-web-cookie-reload <web-port> <ssh-port>")
	}
	webPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	sshPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	cfgPath := filepath.Join(workDir, "web-cookie-reload.conf")
	buildCfg := func(opsServices string) string {
		var tb textbuf.Buffer
		tb.Str("system {\n\tauthentication {\n")
		tb.Str("\t\tuser webop { password \"").Str(lsTestPasswordHash).Str("\"; login-service ops-doors; }\n")
		tb.Str("\t\tuser keepop { password \"").Str(lsTestPasswordHash).Str("\"; login-service keep-doors; }\n")
		tb.Str("\t}\n\tauthorization {\n")
		tb.Str("\t\tlogin-service ops-doors { service [ ").Str(opsServices).Str(" ]; }\n")
		tb.Str("\t\tlogin-service keep-doors { service [ ssh web ]; }\n")
		tb.Str("\t}\n}\n")
		tb.Str("environment {\n\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
		tb.Str("\tweb { enabled true; server main { ip 127.0.0.1; port ").Str(webPort).Str("; } }\n}\n")
		return tb.String()
	}

	if err := lsWriteFile(cfgPath, buildCfg("ssh web")); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, workDir, cfgPath, nil)
	if err != nil {
		return err
	}
	defer daemon.stop()

	if err := lsWaitTCPReady(ctx, webPort, "web"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}

	client, closeIdle := lsNewHTTPSClient()
	defer closeIdle()

	// Both accounts log in before reload and hold valid ze-session cookies.
	webopLogin, err := lsWebFormLogin(ctx, client, webPort, lsUserWebOnly, lsTestPassword)
	if err != nil {
		return fmt.Errorf("pre-reload webop login failed: %w", err)
	}
	if webopLogin.cookie == "" {
		return fmt.Errorf("pre-reload webop received no cookie (status=%d)", webopLogin.status)
	}
	keepLogin, err := lsWebFormLogin(ctx, client, webPort, "keepop", lsTestPassword)
	if err != nil {
		return fmt.Errorf("pre-reload keepop login failed: %w", err)
	}
	if keepLogin.cookie == "" {
		return fmt.Errorf("pre-reload keepop received no cookie (status=%d)", keepLogin.status)
	}

	preWebop, err := lsWebCookieGet(ctx, client, webPort, webopLogin.cookie, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("pre-reload webop cookie check failed: %w", err)
	}
	if preWebop.status != http.StatusOK {
		return fmt.Errorf("pre-reload webop cookie returned HTTP %d, want 200", preWebop.status)
	}
	preKeep, err := lsWebCookieGet(ctx, client, webPort, keepLogin.cookie, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("pre-reload keepop cookie check failed: %w", err)
	}
	if preKeep.status != http.StatusOK {
		return fmt.Errorf("pre-reload keepop cookie returned HTTP %d, want 200", preKeep.status)
	}
	fmt.Fprintln(os.Stderr, "OK: pre-reload web session cookies work for both accounts")

	// Reload: edit ONLY the ops-doors set definition (removing web) without touching user webop.
	if err := lsWriteFile(cfgPath, buildCfg("ssh")); err != nil {
		return err
	}
	if err := daemon.reloadAndWait(ctx, 1); err != nil {
		return err
	}

	// webop's pre-reload cookie MUST now be refused HTTP 401 on the very next request.
	afterWebop, err := lsWebCookieGet(ctx, client, webPort, webopLogin.cookie, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("post-reload webop cookie request failed: %w", err)
	}
	if afterWebop.status != http.StatusUnauthorized {
		return fmt.Errorf("post-reload webop cookie returned HTTP %d, want 401", afterWebop.status)
	}
	// webop still reaches SSH.
	if _, err := lsExecSSHPassword(sshPort, lsUserWebOnly, lsTestPassword, lsCmdShowVersion); err != nil {
		return fmt.Errorf("post-reload webop lost SSH access: %w", err)
	}
	// keepop's cookie still works (proves reload did not kill unrelated sessions).
	afterKeep, err := lsWebCookieGet(ctx, client, webPort, keepLogin.cookie, lsWebProbePath)
	if err != nil {
		return fmt.Errorf("post-reload keepop cookie request failed: %w", err)
	}
	if afterKeep.status != http.StatusOK {
		return fmt.Errorf("post-reload keepop cookie returned HTTP %d, want 200", afterKeep.status)
	}
	fmt.Fprintln(os.Stderr, "OK: reload revoked withdrawn web cookie immediately while keeping other sessions and SSH alive")
	return nil
}

func lsFixtureREST(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-rest <rest-port> <grpc-port> <metrics-port>")
	}
	restPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	grpcPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	metricsPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, restPort, "REST"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, grpcPort, "gRPC"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, metricsPort, "Prometheus"); err != nil {
		return err
	}

	// Capture running config before refused attempt (AC-3).
	beforeStatus, beforeCfg, err := lsRESTExecute(ctx, restPort, lsUserREST+":"+lsTestPassword, lsCmdShowConfig)
	if err != nil {
		return fmt.Errorf("REST baseline show config failed: %w", err)
	}
	if beforeStatus != http.StatusOK {
		return fmt.Errorf("REST baseline returned HTTP %d, want 200: %s", beforeStatus, beforeCfg)
	}

	// Wrong password baseline on REST.
	wrongCode, wrongBody, err := lsRESTExecute(ctx, restPort, lsUserGRPC+":"+lsWrongPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("REST wrong-password request failed: %w", err)
	}
	if wrongCode != http.StatusUnauthorized {
		return fmt.Errorf("REST wrong-password returned HTTP %d, want 401", wrongCode)
	}

	// Excluded door: grpconly with CORRECT password on REST must return identical 401.
	refusedCode, refusedBody, err := lsRESTExecute(ctx, restPort, lsUserGRPC+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("REST excluded-account request failed: %w", err)
	}
	if refusedCode != wrongCode {
		return fmt.Errorf("REST refusal status=%d, want %d", refusedCode, wrongCode)
	}
	if refusedBody != wrongBody {
		return fmt.Errorf("REST refusal body mismatch: excluded=%q wrong=%q", refusedBody, wrongBody)
	}
	fmt.Fprintln(os.Stderr, "OK: REST refused grpc-only account identically to wrong password")

	// Included door: grpconly with same credentials succeeds on gRPC (AC-17).
	conn, err := lsNewGRPCClient(grpcPort)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture cleanup
	grpcResp, grpcCode, grpcMsg, err := lsGRPCExecute(ctx, conn, lsUserGRPC+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("grpconly failed on its included gRPC door: code=%s msg=%q err=%w", grpcCode, grpcMsg, err)
	}
	if grpcCode != codes.OK {
		return fmt.Errorf("grpconly gRPC code=%s msg=%q, want OK", grpcCode, grpcMsg)
	}
	if grpcResp.GetStatus() != statusDone {
		return fmt.Errorf("grpconly gRPC status=%q, want done", grpcResp.GetStatus())
	}

	// Reverse control: restonly succeeds on REST.
	okCode, okBody, err := lsRESTExecute(ctx, restPort, lsUserREST+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("restonly failed on REST: %w", err)
	}
	if okCode != http.StatusOK {
		return fmt.Errorf("restonly on REST returned HTTP %d, want 200: %s", okCode, okBody)
	}
	_, afterCfg, err := lsRESTExecute(ctx, restPort, lsUserREST+":"+lsTestPassword, lsCmdShowConfig)
	if err != nil {
		return fmt.Errorf("REST post-attempt show config failed: %w", err)
	}
	if beforeCfg != afterCfg {
		return errors.New("refused REST login modified running config")
	}

	metricsText, err := lsScrapePrometheus(ctx, metricsPort)
	if err != nil {
		return err
	}
	count, found, err := lsRefusedCounterValue(metricsText, "rest")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("missing %s{service=\"rest\"} metric series", lsRefusedMetric)
	}
	if count < 1 {
		return fmt.Errorf("expected %s{service=\"rest\"} >= 1, got %v", lsRefusedMetric, count)
	}
	fmt.Fprintln(os.Stderr, "OK: REST gate refusal, gRPC admission, zero side effects, and metric verified")
	return nil
}

func lsFixtureGRPC(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-grpc <grpc-port> <rest-port> <metrics-port>")
	}
	grpcPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	restPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	metricsPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, grpcPort, "gRPC"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, restPort, "REST"); err != nil {
		return err
	}
	if err := lsWaitTCPReady(ctx, metricsPort, "Prometheus"); err != nil {
		return err
	}

	conn, err := lsNewGRPCClient(grpcPort)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // fixture cleanup

	// Wrong-password baseline on gRPC unary Execute.
	_, wrongCode, wrongMsg, _ := lsGRPCExecute(ctx, conn, lsUserREST+":"+lsWrongPassword, lsCmdShowVersion)
	if wrongCode != codes.Unauthenticated {
		return fmt.Errorf("gRPC wrong password returned code=%s, want Unauthenticated", wrongCode)
	}

	// Excluded door: restonly with CORRECT password on gRPC unary Execute.
	_, unaryCode, unaryMsg, _ := lsGRPCExecute(ctx, conn, lsUserREST+":"+lsTestPassword, lsCmdShowVersion)
	if unaryCode != wrongCode {
		return fmt.Errorf("gRPC unary refusal code=%s, want %s", unaryCode, wrongCode)
	}
	if unaryMsg != wrongMsg {
		return fmt.Errorf("gRPC unary refusal message mismatch: excluded=%q wrong=%q", unaryMsg, wrongMsg)
	}

	// Excluded door: restonly with CORRECT password on gRPC Stream open.
	const streamCmd = "monitor event"
	_, streamCode, streamMsg, _ := lsGRPCStreamOpen(ctx, conn, lsUserREST+":"+lsTestPassword, streamCmd)
	if streamCode != wrongCode {
		return fmt.Errorf("gRPC stream refusal code=%s, want %s", streamCode, wrongCode)
	}
	if streamMsg != wrongMsg {
		return fmt.Errorf("gRPC stream refusal message mismatch: excluded=%q wrong=%q", streamMsg, wrongMsg)
	}
	fmt.Fprintln(os.Stderr, "OK: gRPC unary and stream open refused rest-only account identically to wrong password")

	// Included door for restonly: REST succeeds.
	restCode, restBody, err := lsRESTExecute(ctx, restPort, lsUserREST+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("restonly failed on its included REST door: %w", err)
	}
	if restCode != http.StatusOK {
		return fmt.Errorf("restonly on REST returned HTTP %d, want 200: %s", restCode, restBody)
	}

	// Reverse pair (AC-17): grpconly succeeds on gRPC unary AND stream open, refused on REST.
	execResp, execCode, execMsg, err := lsGRPCExecute(ctx, conn, lsUserGRPC+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("grpconly unary Execute failed: code=%s msg=%q err=%w", execCode, execMsg, err)
	}
	if execCode != codes.OK {
		return fmt.Errorf("grpconly unary code=%s msg=%q, want OK", execCode, execMsg)
	}
	if execResp.GetStatus() != statusDone {
		return fmt.Errorf("grpconly unary status=%q, want done", execResp.GetStatus())
	}
	streamResp, stCode, stMsg, err := lsGRPCStreamOpen(ctx, conn, lsUserGRPC+":"+lsTestPassword, streamCmd)
	if err != nil {
		return fmt.Errorf("grpconly Stream open failed: code=%s msg=%q err=%w", stCode, stMsg, err)
	}
	if stCode != codes.OK {
		return fmt.Errorf("grpconly Stream code=%s msg=%q, want OK", stCode, stMsg)
	}
	if streamResp.GetStatus() != statusDone {
		return fmt.Errorf("grpconly Stream status=%q, want done", streamResp.GetStatus())
	}
	grpcOnRestCode, _, err := lsRESTExecute(ctx, restPort, lsUserGRPC+":"+lsTestPassword, lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("grpconly on REST request failed: %w", err)
	}
	if grpcOnRestCode != http.StatusUnauthorized {
		return fmt.Errorf("grpconly on REST returned HTTP %d, want 401", grpcOnRestCode)
	}

	metricsText, err := lsScrapePrometheus(ctx, metricsPort)
	if err != nil {
		return err
	}
	count, found, err := lsRefusedCounterValue(metricsText, "grpc")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("missing %s{service=\"grpc\"} metric series", lsRefusedMetric)
	}
	if count < 2 {
		return fmt.Errorf("expected %s{service=\"grpc\"} >= 2, got %v", lsRefusedMetric, count)
	}
	fmt.Fprintln(os.Stderr, "OK: gRPC and REST complementary pair, stream open, and metric verified")
	return nil
}

func lsFixtureMCP(ctx context.Context, args []string) error {
	if len(args) != 3 {
		return errors.New("usage: plugin/login-service-mcp <mcp-port> <ssh-port> <metrics-port>")
	}
	mcpPort, err := lsRequirePort(args[0])
	if err != nil {
		return err
	}
	sshPort, err := lsRequirePort(args[1])
	if err != nil {
		return err
	}
	metricsPort, err := lsRequirePort(args[2])
	if err != nil {
		return err
	}
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}
	bearerListDir := filepath.Join(workDir, "mcp-bearer-list")
	if err := os.MkdirAll(bearerListDir, 0o700); err != nil {
		return err
	}
	cfgPath := filepath.Join(bearerListDir, "mcp-ls.conf")
	buildBearerListCfg := func(aliceServices string) string {
		var tb textbuf.Buffer
		tb.Str("system {\n\tauthentication {\n")
		tb.Str("\t\tuser alice { password \"").Str(lsTestPasswordHash).Str("\"; login-service alice-doors; }\n")
		tb.Str("\t\tuser bob { password \"").Str(lsTestPasswordHash).Str("\"; }\n")
		tb.Str("\t}\n\tauthorization {\n")
		tb.Str("\t\tlogin-service alice-doors { service [ ").Str(aliceServices).Str(" ]; }\n")
		tb.Str("\t}\n}\n")
		tb.Str("environment {\n")
		tb.Str("\tssh { enabled true; server main { ip 127.0.0.1; port ").Str(sshPort).Str("; } }\n")
		tb.Str("\tmcp {\n\t\tenabled true;\n\t\tauth-mode bearer-list;\n")
		tb.Str("\t\tidentity alice { token \"alice-mcp-token\"; }\n")
		tb.Str("\t\tidentity bob { token \"bob-mcp-token\"; }\n")
		tb.Str("\t\tserver main { ip 127.0.0.1; port ").Str(mcpPort).Str("; }\n\t}\n}\n")
		tb.Str("telemetry {\n\tprometheus {\n\t\tenabled true;\n")
		tb.Str("\t\tserver main { ip 127.0.0.1; port ").Str(metricsPort).Str("; }\n")
		tb.Str("\t\tnetdata { enabled false; }\n\t}\n}\n")
		return tb.String()
	}

	if err := lsWriteFile(cfgPath, buildBearerListCfg("ssh")); err != nil {
		return err
	}
	daemon, err := lsStartDaemon(ctx, bearerListDir, cfgPath, nil)
	if err != nil {
		return err
	}
	if err := lsVerifyMCPBearerList(ctx, daemon, cfgPath, buildBearerListCfg, mcpPort, sshPort, metricsPort); err != nil {
		daemon.stop()
		return err
	}
	daemon.stop()

	// Part 2 (AC-20 survivor): anonymous single shared `bearer` token stays admitted
	// even when login-service sets exist and exclude mcp.
	sharedDir := filepath.Join(workDir, "mcp-shared-bearer")
	if err := os.MkdirAll(sharedDir, 0o700); err != nil {
		return err
	}
	sharedCfgPath := filepath.Join(sharedDir, "mcp-shared.conf")
	var sharedCfg textbuf.Buffer
	sharedCfg.Str("system {\n\tauthentication {\n")
	sharedCfg.Str("\t\tuser alice { password \"").Str(lsTestPasswordHash).Str("\"; login-service ssh-only; }\n")
	sharedCfg.Str("\t}\n\tauthorization {\n")
	sharedCfg.Str("\t\tlogin-service ssh-only { service [ ssh ]; }\n")
	sharedCfg.Str("\t}\n}\n")
	sharedCfg.Str("environment {\n\tmcp {\n\t\tenabled true;\n\t\tauth-mode bearer;\n")
	sharedCfg.Str("\t\ttoken \"shared-anon-mcp-token\";\n")
	sharedCfg.Str("\t\tserver main { ip 127.0.0.1; port ").Str(mcpPort).Str("; }\n\t}\n}\n")
	if err := lsWriteFile(sharedCfgPath, sharedCfg.String()); err != nil {
		return err
	}
	sharedDaemon, err := lsStartDaemon(ctx, sharedDir, sharedCfgPath, nil)
	if err != nil {
		return err
	}
	defer sharedDaemon.stop()
	if err := lsWaitTCPReady(ctx, mcpPort, "MCP shared-bearer"); err != nil {
		return fmt.Errorf("%w\n%s", err, sharedDaemon.logText())
	}
	anonResp, err := lsMCPToolCall(ctx, mcpPort, "shared-anon-mcp-token", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("anonymous bearer MCP call failed: %w", err)
	}
	if anonResp.status != http.StatusOK {
		return fmt.Errorf("anonymous bearer MCP call returned HTTP %d: %s", anonResp.status, anonResp.body)
	}
	if !strings.Contains(anonResp.body, `"resultType":"complete"`) {
		return fmt.Errorf("anonymous bearer MCP call did not complete: %s", anonResp.body)
	}
	fmt.Fprintln(os.Stderr, "OK: anonymous MCP bearer mode unaffected by login-service sets")
	return nil
}

func lsVerifyMCPBearerList(
	ctx context.Context,
	daemon *lsManagedDaemon,
	cfgPath string,
	buildCfg func(string) string,
	mcpPort, sshPort, metricsPort string,
) error {
	if err := lsWaitTCPReady(ctx, mcpPort, "MCP"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	if err := lsWaitTCPReady(ctx, sshPort, "SSH"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}
	if err := lsWaitTCPReady(ctx, metricsPort, "Prometheus"); err != nil {
		return fmt.Errorf("%w\n%s", err, daemon.logText())
	}

	// Control: unrestricted bob succeeds on MCP.
	bobResp, err := lsMCPToolCall(ctx, mcpPort, "bob-mcp-token", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("bob MCP call failed: %w", err)
	}
	if bobResp.status != http.StatusOK {
		return fmt.Errorf("unrestricted bob MCP call returned HTTP %d: %s", bobResp.status, bobResp.body)
	}
	if !strings.Contains(bobResp.body, `"resultType":"complete"`) {
		return fmt.Errorf("unrestricted bob MCP call did not complete: %s", bobResp.body)
	}

	// Wrong-token baseline on MCP.
	wrongResp, err := lsMCPToolCall(ctx, mcpPort, "wrong-mcp-token", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("wrong-token MCP call failed: %w", err)
	}
	if wrongResp.status != http.StatusUnauthorized {
		return fmt.Errorf("wrong-token MCP call returned HTTP %d, want 401", wrongResp.status)
	}

	// Excluded named principal alice: must match wrong token exactly (401, Bearer error="invalid_token", zero tool execution).
	refusedResp, err := lsMCPToolCall(ctx, mcpPort, "alice-mcp-token", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("alice excluded MCP call failed: %w", err)
	}
	if refusedResp.status != http.StatusUnauthorized {
		return fmt.Errorf("alice excluded MCP call returned HTTP %d, want 401: %s", refusedResp.status, refusedResp.body)
	}
	if refusedResp.wwwAuth != wrongResp.wwwAuth {
		return fmt.Errorf("MCP WWW-Authenticate mismatch: excluded=%q wrong=%q", refusedResp.wwwAuth, wrongResp.wwwAuth)
	}
	if !strings.Contains(refusedResp.wwwAuth, `error="invalid_token"`) {
		return fmt.Errorf("MCP WWW-Authenticate missing invalid_token: %q", refusedResp.wwwAuth)
	}
	if refusedResp.body != wrongResp.body {
		return fmt.Errorf("MCP refusal body mismatch: excluded=%q wrong=%q", refusedResp.body, wrongResp.body)
	}
	if strings.Contains(refusedResp.body, "resultType") {
		return fmt.Errorf("refused MCP POST executed a tool: %s", refusedResp.body)
	}
	fmt.Fprintln(os.Stderr, "OK: MCP refused excluded named bearer-list principal identically to invalid token")

	// Alice still works on her included SSH door.
	if _, err := lsExecSSHPassword(sshPort, lsUserAlice, lsTestPassword, lsCmdShowVersion); err != nil {
		return fmt.Errorf("alice failed on included SSH door: %w", err)
	}

	// Check refusal metric stepped for service="mcp".
	metricsText, err := lsScrapePrometheus(ctx, metricsPort)
	if err != nil {
		return err
	}
	count, found, err := lsRefusedCounterValue(metricsText, "mcp")
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("missing %s{service=\"mcp\"} metric series", lsRefusedMetric)
	}
	if count < 1 {
		return fmt.Errorf("expected %s{service=\"mcp\"} >= 1, got %v", lsRefusedMetric, count)
	}

	// Live reload adds mcp to alice-doors; next POST must be admitted with no restart.
	if err := lsWriteFile(cfgPath, buildCfg("ssh mcp")); err != nil {
		return err
	}
	if err := daemon.reloadAndWait(ctx, 1); err != nil {
		return err
	}
	admittedResp, err := lsMCPToolCall(ctx, mcpPort, "alice-mcp-token", lsCmdShowVersion)
	if err != nil {
		return fmt.Errorf("post-reload alice MCP call failed: %w", err)
	}
	if admittedResp.status != http.StatusOK {
		return fmt.Errorf("post-reload alice MCP call returned HTTP %d: %s", admittedResp.status, admittedResp.body)
	}
	if !strings.Contains(admittedResp.body, `"resultType":"complete"`) {
		return fmt.Errorf("post-reload alice MCP call did not complete: %s", admittedResp.body)
	}
	fmt.Fprintln(os.Stderr, "OK: reload admitted alice on MCP without restart")
	return nil
}
